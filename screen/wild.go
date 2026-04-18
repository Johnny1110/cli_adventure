// screen/wild.go — Wild exploration screen with movement, encounters, and area transitions.
//
// THEORY — Random encounters:
// Classic RPGs use a step counter for random encounters. Every time the player
// moves one tile, a counter increments. When it hits a threshold, a random
// check is made against the area's encounter rate. If triggered, the screen
// does the classic "battle flash" effect (screen flashes white rapidly) and
// transitions to the combat screen with a randomly selected monster.
//
// The monster selection uses weighted random: each monster in the area's encounter
// table has a weight. We sum all weights, roll a random number, and walk the table
// to find which monster was selected. Higher-weight monsters appear more often.
//
// THEORY — Area transitions:
// Each area has AreaConnection entries that define "exit zones" — specific tile
// rows at the top or bottom edge. When the player steps on an exit tile, we
// load the target area, place the player at that area's designated start position,
// and do a fade transition. The area graph is: Town <-> Forest <-> Cave <-> Lair.
package screen

import (
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/asset"
	"cli_adventure/data"
	"cli_adventure/entity"
	netpkg "cli_adventure/net"
	"cli_adventure/render"
)

type wildState int

const (
	wildWalking wildState = iota
	wildEncounterFlash
	wildTransition
	wildPaused
	wildBossDialogue
)

// WildScreen handles exploration in wild areas.
type WildScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player

	// Current area
	area    *data.Area
	areas   map[string]*data.Area
	tileMap *render.TileMap
	camera  *render.Camera
	tileset *ebiten.Image

	// Player position
	tileX, tileY   int
	pixelX, pixelY float64
	moving         bool
	facing         direction
	moveSpeed      float64
	walkTick       int

	// Sprites
	charSprites    map[int]*render.SpriteSheet
	monsterSprites map[string]*render.SpriteSheet

	// Encounter system
	stepCount      int
	encounterMon   *entity.Monster // the monster about to be fought

	// State
	state     wildState
	flashTick int
	fadeTick  int
	fadeDir   int // 1 = fading out, -1 = fading in
	nextArea  string
	nextEdge  data.ExitEdge // which edge the player enters the next area from

	// Boss
	bossDialogue *render.DialogueBox
	bossTriggered bool

	// Particles
	particles *render.ParticleSystem

	// Multiplayer session (nil in single-player). When set:
	//   - Client never triggers encounters or area changes; it follows the
	//     host via area_change / combat_start session events.
	//   - Host broadcasts area transitions before switching screens so the
	//     client can follow. Encounters switch to CombatMPScreen on every
	//     peer (including the host), which runs a host-authoritative
	//     simultaneous-decision round machine (see net/combat_round.go).
	session *netpkg.Session
}

// areaStart returns the default start tile for an area, or (5,5) as a
// neutral fallback. Used when the host announces an area change so the
// client's snapshot has a sane tile before its own SetMyPosition arrives.
func areaStart(name string) (int, int) {
	if name == "town" {
		return 9, 10
	}
	if a, ok := data.AllAreas()[name]; ok && a != nil {
		return a.PlayerStartX, a.PlayerStartY
	}
	return 5, 5
}

// NewWildScreenMP creates a wild screen wired to a live multiplayer session.
// The session is carried through every further transition (wild→wild,
// wild→combat, wild→town) so peers stay linked.
func NewWildScreenMP(switcher ScreenSwitcher, player *entity.Player, areaName string, sess *netpkg.Session) *WildScreen {
	w := NewWildScreenAt(switcher, player, areaName, -1, -1)
	w.session = sess
	if sess != nil {
		sess.SetMyPosition(areaName, w.tileX, w.tileY, int(w.facing),
			player.Stats.HP, player.Stats.MaxHP)
	}
	return w
}

// NewWildScreenMPAt is the session-aware variant of NewWildScreenAt.
func NewWildScreenMPAt(switcher ScreenSwitcher, player *entity.Player, areaName string, tileX, tileY int, sess *netpkg.Session) *WildScreen {
	w := NewWildScreenAt(switcher, player, areaName, tileX, tileY)
	w.session = sess
	if sess != nil {
		sess.SetMyPosition(areaName, w.tileX, w.tileY, int(w.facing),
			player.Stats.HP, player.Stats.MaxHP)
	}
	return w
}

// NewWildScreen creates a wild exploration screen starting in the given area
// at the area's default start position.
func NewWildScreen(switcher ScreenSwitcher, player *entity.Player, areaName string) *WildScreen {
	return NewWildScreenAt(switcher, player, areaName, -1, -1)
}

// NewWildScreenFromEdge creates a wild screen with direction-aware spawning.
// The player appears on the edge opposite to the direction they came from.
// E.g., if they walked east out of town (entering from the west), they spawn
// on the west edge of the new map.
func NewWildScreenFromEdge(switcher ScreenSwitcher, player *entity.Player, areaName string, entryEdge data.ExitEdge) *WildScreen {
	areas := data.AllAreas()
	tileset := asset.GenerateWildTileset()

	w := &WildScreen{
		switcher:       switcher,
		player:         player,
		areas:          areas,
		tileset:        tileset,
		facing:         dirDown,
		moveSpeed:      2.0,
		charSprites:    asset.GenerateCharSprites(),
		monsterSprites: asset.GenerateMonsterSprites(),
		// Start with a fade-in so the transition from town is smooth.
		state:    wildTransition,
		fadeTick: 15,
		fadeDir:  -1, // fading IN (15 → 0)
	}
	w.loadAreaFromEdge(areaName, entryEdge)

	// Set facing based on entry direction (face inward)
	switch entryEdge {
	case data.EdgeNorth:
		w.facing = dirDown
	case data.EdgeSouth:
		w.facing = dirUp
	case data.EdgeWest:
		w.facing = dirRight
	case data.EdgeEast:
		w.facing = dirLeft
	}
	return w
}

// NewWildScreenAt creates a wild exploration screen at a specific tile position.
// Pass (-1, -1) to use the area's default start position.
// This is used when returning from combat to preserve the player's location.
func NewWildScreenAt(switcher ScreenSwitcher, player *entity.Player, areaName string, tileX, tileY int) *WildScreen {
	areas := data.AllAreas()
	tileset := asset.GenerateWildTileset()

	w := &WildScreen{
		switcher:       switcher,
		player:         player,
		areas:          areas,
		tileset:        tileset,
		facing:         dirDown,
		moveSpeed:      2.0,
		charSprites:    asset.GenerateCharSprites(),
		monsterSprites: asset.GenerateMonsterSprites(),
		state:          wildWalking,
	}
	w.loadArea(areaName)

	// Override start position if specified
	if tileX >= 0 && tileY >= 0 {
		w.tileX = tileX
		w.tileY = tileY
		w.pixelX = float64(tileX * render.TileSize)
		w.pixelY = float64(tileY * render.TileSize)
	}
	return w
}

// loadArea loads a new area with direction-aware spawning.
// entryEdge indicates which edge the player enters from. Pass -1 to use
// the area's default PlayerStartX/Y (for initial load or combat return).
func (w *WildScreen) loadArea(name string) {
	w.loadAreaFromEdge(name, -1)
}

func (w *WildScreen) loadAreaFromEdge(name string, entryEdge data.ExitEdge) {
	area, ok := w.areas[name]
	if !ok {
		return
	}
	w.area = area

	// Build tile map
	ground := make([][]int, area.Height)
	overlay := make([][]int, area.Height)
	solid := make([][]bool, area.Height)
	for y := 0; y < area.Height; y++ {
		ground[y] = make([]int, area.Width)
		overlay[y] = make([]int, area.Width)
		solid[y] = make([]bool, area.Width)
		for x := 0; x < area.Width; x++ {
			ground[y][x] = area.Ground[y][x]
			overlay[y][x] = -1
			solid[y][x] = area.Solid[y][x]
		}
	}

	w.tileMap = render.NewTileMap(area.Width, area.Height, ground, overlay, solid, w.tileset)
	w.camera = render.NewCamera(area.Width, area.Height)

	// Spawn position: use direction-aware spawning if an entry edge is given,
	// otherwise fall back to the area's default start position.
	if entryEdge >= 0 {
		w.tileX, w.tileY = data.SpawnForEntry(area, entryEdge)
	} else {
		w.tileX = area.PlayerStartX
		w.tileY = area.PlayerStartY
	}
	w.pixelX = float64(w.tileX * render.TileSize)
	w.pixelY = float64(w.tileY * render.TileSize)
	w.stepCount = 0
	w.bossTriggered = false
	w.particles = render.NewParticleSystem(area.MapKey, area.Width, area.Height)
}

func (w *WildScreen) OnEnter() {
	// Level-gate guard check: when entering a new area, if the player
	// is underleveled, show a warning dialogue from a guard NPC.
	//
	// THEORY — Soft level gates:
	// We don't block the player from entering — that would feel
	// frustrating. Instead, a guard warns them. If they're strong
	// enough, the guard encourages them. This respects player agency
	// while naturally guiding progression. The warning creates tension:
	// "I was warned... should I push deeper or go back and grind?"
	if w.area != nil {
		// Skip guard warning if this area's boss is already defeated.
		// The area is cleared — no point warning the player about danger
		// that no longer exists. This also prevents the guard dialogue
		// from accidentally feeding into updateBossDialogue's boss-fight
		// trigger (the primary fix for the re-trigger bug is there, but
		// this avoids the unnecessary dialogue entirely).
		bossKey := bossKeyForArea(w.area.MapKey)
		alreadyCleared := bossKey != "" && w.player.BossDefeated != nil && w.player.BossDefeated[bossKey]

		if !alreadyCleared {
			_, warning := data.CanEnterArea(w.area.MapKey, w.player.Level, w.player.BossDefeated)
			if warning != "" {
				w.bossDialogue = render.NewDialogueBox([]string{
					"A guard stands at\nthe entrance...",
					warning,
				})
				w.state = wildBossDialogue
			}
		}
	}
}
func (w *WildScreen) OnExit() {}

func (w *WildScreen) Update() error {
	w.walkTick++

	switch w.state {
	case wildWalking:
		w.updateWalking()
	case wildEncounterFlash:
		w.updateFlash()
	case wildTransition:
		w.updateTransition()
	case wildPaused:
		w.updatePaused()
	case wildBossDialogue:
		w.updateBossDialogue()
	}

	// Update ambient particles
	if w.particles != nil {
		w.particles.Update()
	}

	centerX := int(w.pixelX) + render.TileSize/2
	centerY := int(w.pixelY) + render.TileSize/2
	w.camera.Follow(centerX, centerY)

	// Push our position into the shared session (no-op in single-player)
	// and react to remote events (host-driven area changes, combat start).
	w.syncSession()

	return nil
}

// syncSession is the per-tick multiplayer heartbeat. Mirrors town.go's
// version: push our own tile position/HP into the session so the host
// (or other peers via the host's rebroadcast) sees us, then drain the
// event queue for host-initiated transitions.
func (w *WildScreen) syncSession() {
	if w.session == nil {
		return
	}
	w.session.SetMyPosition(w.area.MapKey, w.tileX, w.tileY, int(w.facing),
		w.player.Stats.HP, w.player.Stats.MaxHP)
	for _, ev := range w.session.PopEvents() {
		switch ev.Kind {
		case "combat_start":
			// Only the client should auto-switch; the host opened combat
			// itself in updateFlash() and has already switched by the time
			// this fires. Clients transition to the shared MP combat view.
			if w.session.Role() == netpkg.RoleClient {
				w.switcher.SwitchScreen(NewCombatMPScreen(
					w.switcher, w.player, w.session,
					w.area.MapKey, w.tileX, w.tileY,
				))
			}
		case "area_change":
			if w.session.Role() != netpkg.RoleClient {
				continue
			}
			if ev.Area == "town" {
				w.switcher.SwitchScreen(NewTownScreenMP(w.switcher, w.player, w.session))
				return
			}
			if ev.Area != "" && ev.Area != w.area.MapKey {
				w.switcher.SwitchScreen(NewWildScreenMP(w.switcher, w.player, ev.Area, w.session))
				return
			}
		}
	}
}

// isRemoteAt returns true when another peer occupies the given tile in
// the current area — prevents peers from walking through each other.
func (w *WildScreen) isRemoteAt(x, y int) bool {
	if w.session == nil {
		return false
	}
	for _, p := range w.session.RemotePlayers(w.area.MapKey) {
		if p.TileX == x && p.TileY == y {
			return true
		}
	}
	return false
}

func (w *WildScreen) updateWalking() {
	// Smooth interpolation
	if w.moving {
		targetX := float64(w.tileX * render.TileSize)
		targetY := float64(w.tileY * render.TileSize)

		if dx := targetX - w.pixelX; dx > 0 {
			w.pixelX += w.moveSpeed
			if w.pixelX >= targetX { w.pixelX = targetX }
		} else if dx < 0 {
			w.pixelX -= w.moveSpeed
			if w.pixelX <= targetX { w.pixelX = targetX }
		}
		if dy := targetY - w.pixelY; dy > 0 {
			w.pixelY += w.moveSpeed
			if w.pixelY >= targetY { w.pixelY = targetY }
		} else if dy < 0 {
			w.pixelY -= w.moveSpeed
			if w.pixelY <= targetY { w.pixelY = targetY }
		}

		if w.pixelX == targetX && w.pixelY == targetY {
			w.moving = false
			w.onStepComplete()
		}
		return
	}

	// Movement input
	newX, newY := w.tileX, w.tileY
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		newY--
		w.facing = dirUp
	} else if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		newY++
		w.facing = dirDown
	} else if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		newX--
		w.facing = dirLeft
	} else if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		newX++
		w.facing = dirRight
	}

	if newX != w.tileX || newY != w.tileY {
		if !w.tileMap.IsSolid(newX, newY) && !w.isRemoteAt(newX, newY) {
			dx := newX - w.tileX
			dy := newY - w.tileY
			w.tileX = newX
			w.tileY = newY
			w.moving = true
			// Forward input to host (client side only). Host applies its
			// own movement locally.
			if w.session != nil && w.session.Role() == netpkg.RoleClient {
				w.session.SubmitInput(netpkg.InputMsg{DX: dx, DY: dy})
			}
		}
	}

	// Pause
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		w.state = wildPaused
	}

	// Map (M key)
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		w.switcher.SwitchScreen(NewMapScreen(w.switcher, w, w.area.MapKey))
	}

	// Bag (B key)
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		w.switcher.SwitchScreen(NewBagScreen(w.switcher, w.player, w))
	}

	// Equipment (E key)
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		w.switcher.SwitchScreen(NewEquipScreen(w.switcher, w.player, w))
	}

	// Skill tree (T key)
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		w.switcher.SwitchScreen(NewSkillScreen(w.switcher, w.player, w))
	}
}

// onStepComplete is called when the player finishes moving to a new tile.
func (w *WildScreen) onStepComplete() {
	// Advance day/night cycle with each step
	if w.player.DayNight != nil {
		w.player.DayNight.Step()
	}

	// Check area transitions. In multiplayer, only the host initiates these;
	// clients follow via the area_change event dispatched from syncSession.
	if w.session != nil && w.session.Role() == netpkg.RoleClient {
		// Still drop treasure/fairy/boss logic on the floor for clients —
		// those are host-authoritative too.
		return
	}
	for _, conn := range w.area.Connections {
		// Check if the player is standing on this exit zone.
		// North/South exits check a horizontal tile range at a specific row.
		// East/West exits check a vertical tile range at a specific column.
		hit := false
		switch conn.Edge {
		case data.EdgeNorth, data.EdgeSouth:
			hit = w.tileY == conn.FromY && w.tileX >= conn.FromMinX && w.tileX <= conn.FromMaxX
		case data.EdgeEast, data.EdgeWest:
			hit = w.tileX == conn.FromX && w.tileY >= conn.FromMinY && w.tileY <= conn.FromMaxY
		}
		if !hit {
			continue
		}

		if conn.TargetArea == "town" {
			// Return to town via fade transition.
			if w.session != nil && w.session.Role() == netpkg.RoleHost {
				w.session.BroadcastAreaChange("town", 9, 10)
				w.switcher.SwitchScreen(NewTownScreenMP(w.switcher, w.player, w.session))
				return
			}
			// Use the same fade system — nextArea="town" is a special case
			// handled in updateTransition.
			w.nextArea = "town"
			w.fadeTick = 0
			w.fadeDir = 1
			w.state = wildTransition
			return
		}
		// Transition to another wild area. In MP, broadcast first so
		// clients enter the same area in their own syncSession pass.
		entryEdge := data.OppositeEdge(conn.Edge)
		if w.session != nil && w.session.Role() == netpkg.RoleHost {
			if a, ok := w.areas[conn.TargetArea]; ok {
				sx, sy := data.SpawnForEntry(a, entryEdge)
				w.session.BroadcastAreaChange(conn.TargetArea, sx, sy)
			}
		}
		w.nextArea = conn.TargetArea
		w.nextEdge = entryEdge
		w.fadeTick = 0
		w.fadeDir = 1
		w.state = wildTransition
		return
	}

	// Check for treasure chest
	if w.area.Ground[w.tileY][w.tileX] == asset.ChestClosed {
		chestKey := w.area.MapKey + ":" + intToStr(w.tileX) + ":" + intToStr(w.tileY)
		if !w.player.OpenedChests[chestKey] {
			w.player.OpenedChests[chestKey] = true
			// Change tile to open chest
			w.area.Ground[w.tileY][w.tileX] = asset.ChestOpen
			w.tileMap.SetGround(w.tileX, w.tileY, asset.ChestOpen)
			// Give random loot
			w.giveChestLoot()
			return
		}
	}

	// Check for fairy flower (secret!)
	if w.area.Ground[w.tileY][w.tileX] == asset.FairyFlower && !w.player.FairyBlessing {
		w.player.FairyBlessing = true
		// Permanent stat boost!
		w.player.Stats.MaxHP += 10
		w.player.Stats.HP += 10
		w.player.Stats.ATK += 2
		w.player.Stats.DEF += 2
		w.player.Stats.SPD += 2
		w.bossDialogue = render.NewDialogueBox([]string{
			"A fairy appears from\nthe glowing flowers!",
			"\"You found my secret\ngarden, brave hero!\"",
			"\"I bless you with\nthe forest's power!\"",
			"All stats increased!",
		})
		w.state = wildBossDialogue
		return
	}

	// Boss area: trigger boss encounter when player moves near center.
	// Each boss area has EncounterRate=0 (no randoms) and a single boss
	// in its encounter table. The boss triggers when the player walks
	// into the top half of the arena.
	if w.area.EncounterRate == 0 && !w.bossTriggered {
		bossName := bossForArea(w.area.MapKey)
		if bossName != "" {
			bossKey := bossKeyForArea(w.area.MapKey)
			// If already defeated, let the player explore freely
			if w.player.BossDefeated != nil && w.player.BossDefeated[bossKey] {
				// No boss — area is cleared
			} else if w.tileY <= w.area.Height/2 {
				w.bossTriggered = true
				w.bossDialogue = render.NewDialogueBox(bossIntroLines(w.area.MapKey, bossName))
				w.state = wildBossDialogue
				return
			}
		}
	}

	// Random encounter check
	if w.area.EncounterRate > 0 {
		w.stepCount++
		if w.stepCount >= w.area.EncounterRate {
			w.stepCount = 0
			// Roll for encounter (50% chance each check)
			if rand.Intn(100) < 50 {
				w.triggerEncounter()
			}
		}
	}
}

func (w *WildScreen) triggerEncounter() {
	// Pick a monster using weighted random
	table := data.EncounterTable[w.area.MapKey]
	if len(table) == 0 {
		return
	}

	totalWeight := 0
	for _, e := range table {
		totalWeight += e.Weight
	}

	roll := rand.Intn(totalWeight)
	cumulative := 0
	for _, e := range table {
		cumulative += e.Weight
		if roll < cumulative {
			template := data.MonsterTemplates[e.Name]
			if template != nil {
				w.encounterMon = template.Clone()
				// Night makes monsters stronger
				if w.player.DayNight != nil {
					mult := w.player.DayNight.MonsterStatMultiplier()
					if mult > 1.0 {
						w.encounterMon.HP = int(float64(w.encounterMon.HP) * mult)
						w.encounterMon.MaxHP = w.encounterMon.HP
						w.encounterMon.ATK = int(float64(w.encounterMon.ATK) * mult)
						w.encounterMon.DEF = int(float64(w.encounterMon.DEF) * mult)
					}
				}
				// 5% chance of rare golden variant!
				if rand.Intn(100) < 5 {
					w.encounterMon.MakeGolden()
				}
				w.flashTick = 0
				w.state = wildEncounterFlash
			}
			return
		}
	}
}

func (w *WildScreen) giveChestLoot() {
	// THEORY — Area-scaled chest loot:
	// The loot table is split into tiers: coins, consumables, old-school
	// weapon/armor drops, and NEW equipment (helmets, boots, shields,
	// accessories) pulled from the area's loot pool. The new gear has a
	// ~25% combined chance, making each chest exciting — there's always
	// a chance of finding gear for one of your empty slots.
	roll := rand.Intn(100)
	var msg string
	switch {
	case roll < 20:
		// Coins (scaled to area level)
		base := 20
		if w.area != nil {
			// Higher-level areas give more coins
			for _, wa := range data.WorldGraph {
				if wa.ID == w.area.MapKey {
					base = 20 + wa.MinLevel*5
					break
				}
			}
		}
		amount := base + rand.Intn(30)
		w.player.Coins += amount
		msg = "Found " + intToStr(amount) + " coins!"
	case roll < 35:
		// Potion
		w.player.AddItem(entity.Item{Name: "Potion", Type: entity.ItemConsumable, StatBoost: 15, Price: 15, ClassRestrict: -1})
		msg = "Found a Potion!"
	case roll < 45:
		// Hi Potion
		w.player.AddItem(entity.Item{Name: "Hi Potion", Type: entity.ItemConsumable, StatBoost: 40, Price: 40, ClassRestrict: -1})
		msg = "Found a Hi Potion!"
	case roll < 55:
		// Crystal Sword (rare weapon!) — universal, any class
		w.player.AddItem(entity.Item{Name: "Crystal Sword", Type: entity.ItemWeapon, StatBoost: 9, Price: 200, ClassRestrict: -1})
		msg = "Found a Crystal Sword!"
	case roll < 65:
		// Dragon Scale armor (rare!) — universal, any class
		w.player.AddItem(entity.Item{Name: "Dragon Scale", Type: entity.ItemArmor, StatBoost: 9, Price: 200, ClassRestrict: -1})
		msg = "Found Dragon Scale\narmor!"
	case roll < 90:
		// NEW EQUIPMENT — pull from area loot table with rarity roll
		areaKey := ""
		areaMinLevel := 0
		if w.area != nil {
			areaKey = w.area.MapKey
			// Look up recommended level for rarity scaling
			if wa, ok := data.WorldGraph[areaKey]; ok {
				areaMinLevel = wa.MinLevel
			}
		}
		item, ok := data.RollLootWithRarity(areaKey, areaMinLevel)
		if ok {
			w.player.AddItem(item)
			rarityTag := ""
			if item.Rarity > entity.RarityCommon {
				rarityTag = "[" + item.Rarity.RarityName() + "] "
			}
			msg = "Found " + rarityTag + "\n" + item.Name + "!"
		} else {
			// Fallback: coins if no loot for this area
			amount := 50 + rand.Intn(50)
			w.player.Coins += amount
			msg = "Found " + intToStr(amount) + " coins!"
		}
	default:
		// Jackpot: lots of coins
		amount := 100 + rand.Intn(50)
		w.player.Coins += amount
		msg = "Jackpot! " + intToStr(amount) + "\ncoins!"
	}

	w.bossDialogue = render.NewDialogueBox([]string{
		"Opened a treasure\nchest!",
		msg,
	})
	w.state = wildBossDialogue
}

func (w *WildScreen) updateFlash() {
	w.flashTick++
	// Flash for ~30 ticks (0.5 seconds), then transition to combat.
	// Multiplayer path (host only — the host is the one who rolled the
	// encounter; clients follow via the combat_start event).
	if w.flashTick >= 30 {
		if w.session != nil && w.session.Role() == netpkg.RoleHost {
			// Open a team-play fight on the host's round machine.
			mon := w.encounterMon
			w.session.StartTeamCombat(netpkg.MonsterInit{
				ID:         mon.SpriteID,
				Name:       mon.Name,
				SpriteID:   mon.SpriteID,
				HP:         mon.HP,
				MaxHP:      mon.MaxHP,
				ATK:        mon.ATK,
				DEF:        mon.DEF,
				SPD:        mon.SPD,
				IsBoss:     mon.IsBoss,
				XPReward:   mon.XPReward,
				CoinReward: mon.CoinReward,
			}, playerCombatStats(w.player))
			w.switcher.SwitchScreen(NewCombatMPScreen(
				w.switcher, w.player, w.session,
				w.area.MapKey, w.tileX, w.tileY,
			))
			return
		}
		if w.session != nil {
			// Client-side: the host will broadcast combat_start. We just
			// switch ourselves to the MP combat screen now — the shared
			// state will populate from the first MsgCombatState.
			w.switcher.SwitchScreen(NewCombatMPScreen(
				w.switcher, w.player, w.session,
				w.area.MapKey, w.tileX, w.tileY,
			))
			return
		}
		w.switcher.SwitchScreen(NewCombatScreenAt(
			w.switcher, w.player, w.encounterMon, w.area.MapKey,
			w.tileX, w.tileY,
		))
	}
}

func (w *WildScreen) updateTransition() {
	// THEORY — Two-phase fade:
	// Phase 1 (fadeDir=1): fade OUT — fadeTick counts up from 0 to 15.
	//   At 15, the screen is fully black. We load the new area.
	// Phase 2 (fadeDir=-1): fade IN — fadeTick counts down from 15 to 0.
	//   At 0, the new area is fully visible and we resume walking.
	//
	// BUG FIX: The increment must only happen during fade-out. If we
	// increment unconditionally, the ++ and -- cancel out during fade-in
	// and fadeTick never reaches 0 (permanent black screen).
	if w.fadeDir == 1 {
		w.fadeTick++
		if w.fadeTick >= 15 {
			// Fully faded out — either load new wild area or switch to town
			if w.nextArea == "town" {
				w.switcher.SwitchScreen(NewTownScreen(w.switcher, w.player))
				return
			}
			w.loadAreaFromEdge(w.nextArea, w.nextEdge)
			w.fadeDir = -1
		}
	} else if w.fadeDir == -1 {
		w.fadeTick--
		if w.fadeTick <= 0 {
			w.state = wildWalking
		}
	}
}

func (w *WildScreen) updatePaused() {
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		w.state = wildWalking
	}
	// Open bag from pause
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		w.state = wildWalking
		w.switcher.SwitchScreen(NewBagScreen(w.switcher, w.player, w))
	}
	// Open map from pause
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		w.state = wildWalking
		w.switcher.SwitchScreen(NewMapScreen(w.switcher, w, w.area.MapKey))
	}
}

func (w *WildScreen) updateBossDialogue() {
	if w.bossDialogue != nil {
		w.bossDialogue.Update()
		if w.bossDialogue.Finished {
			// Trigger boss fight using the area's boss monster
			bossName := bossForArea(w.area.MapKey)
			if bossName == "" {
				w.state = wildWalking
				return
			}
			// Guard: don't re-fight a boss that's already defeated.
			// This dialogue might have come from a level-gate warning
			// in OnEnter rather than a boss intro, so we must check
			// BossDefeated here at the point of action, not just at
			// the point of entry.
			bossKey := bossKeyForArea(w.area.MapKey)
			if bossKey != "" && w.player.BossDefeated != nil && w.player.BossDefeated[bossKey] {
				w.state = wildWalking
				return
			}
			boss := data.MonsterTemplates[bossName]
			if boss != nil {
				w.encounterMon = boss.Clone()
				w.flashTick = 0
				w.state = wildEncounterFlash
			}
		}
	}
}

// bossForArea returns the monster template name for the boss in a given area.
// Returns "" if the area has no boss.
func bossForArea(areaKey string) string {
	switch areaKey {
	case "lair":
		return "Dragon"
	case "ice_cavern":
		return "Ice Wyrm"
	case "volcano":
		return "Hydra"
	case "buried_temple":
		return "Sphinx"
	default:
		return ""
	}
}

// bossKeyForArea returns the BossDefeated map key for an area's boss.
func bossKeyForArea(areaKey string) string {
	switch areaKey {
	case "lair":
		return "dragon"
	case "ice_cavern":
		return "ice_wyrm"
	case "volcano":
		return "hydra"
	case "buried_temple":
		return "sphinx"
	default:
		return ""
	}
}

// bossIntroLines returns the pre-battle dialogue for each boss area.
func bossIntroLines(areaKey, bossName string) []string {
	switch areaKey {
	case "lair":
		return []string{
			"The ground trembles...",
			"A massive Dragon\nappears before you!",
		}
	case "ice_cavern":
		return []string{
			"The air freezes solid\naround you...",
			"An enormous Ice Wyrm\ncoils from the shadows!",
		}
	case "volcano":
		return []string{
			"Lava erupts from\nthe ground!",
			"A terrifying Hydra\nrises from the magma!",
		}
	case "buried_temple":
		return []string{
			"Ancient runes glow\non the temple walls...",
			"The Sphinx awakens\nwith a riddle of death!",
		}
	default:
		return []string{
			"A powerful " + bossName + "\nblocks your path!",
		}
	}
}

// ---- Draw ----

func (w *WildScreen) Draw(screen *ebiten.Image) {
	// Draw ground
	w.tileMap.Draw(screen, w.camera.X, w.camera.Y)

	// Draw remote co-op party members first so the local sprite renders
	// on top if they overlap. No-op in single-player.
	w.drawRemotePlayers(screen)

	// Draw player
	w.drawPlayer(screen)

	// Draw overlay
	w.tileMap.DrawOverlay(screen, w.camera.X, w.camera.Y)

	// Ambient particles (drawn over tiles, under HUD)
	if w.particles != nil {
		w.particles.Draw(screen, w.camera.X, w.camera.Y)
	}

	// Day/night tint overlay
	w.drawDayNightTint(screen)

	// HUD
	render.DrawText(screen, w.area.Name, 4, 2, render.ColorMint)
	// Show time of day in HUD — right-aligned to 320 width
	if w.player.DayNight != nil {
		phaseName := w.player.DayNight.PhaseName()
		phaseClr := render.ColorWhite
		switch w.player.DayNight.Phase {
		case entity.PhaseNight:
			phaseClr = render.ColorSky
		case entity.PhaseDusk:
			phaseClr = render.ColorPeach
		case entity.PhaseDawn:
			phaseClr = render.ColorPink
		}
		render.DrawText(screen, phaseName, 280, 2, phaseClr)
	}
	render.DrawText(screen, "X:Menu B:Bag E:Equip", 32, 276, render.ColorDarkGray)

	// State overlays
	switch w.state {
	case wildEncounterFlash:
		w.drawFlash(screen)
	case wildTransition:
		w.drawFade(screen)
	case wildPaused:
		w.drawPause(screen)
	case wildBossDialogue:
		if w.bossDialogue != nil {
			w.bossDialogue.Draw(screen)
		}
	}
}

func (w *WildScreen) drawPlayer(screen *ebiten.Image) {
	sheet := w.charSprites[int(w.player.Class)]
	frame := 0
	if w.moving && (w.walkTick/8)%2 == 1 {
		frame = 1
	}
	sx := w.pixelX - float64(w.camera.X)
	sy := w.pixelY - float64(w.camera.Y)
	sheet.DrawFrame(screen, frame, sx, sy)
}

// drawRemotePlayers renders every other party member currently in this
// wild area, with a small name tag above their head.
func (w *WildScreen) drawRemotePlayers(screen *ebiten.Image) {
	if w.session == nil {
		return
	}
	for _, rp := range w.session.RemotePlayers(w.area.MapKey) {
		sheet, ok := w.charSprites[rp.Class]
		if !ok {
			continue
		}
		sx := float64(rp.TileX*render.TileSize) - float64(w.camera.X)
		sy := float64(rp.TileY*render.TileSize) - float64(w.camera.Y)
		if sx < -16 || sx > 320 || sy < -16 || sy > 288 {
			continue
		}
		sheet.DrawFrame(screen, 0, sx, sy)
		label := rp.Name
		if len(label) > 10 {
			label = label[:10]
		}
		render.DrawText(screen, label,
			int(sx)+8-render.TextWidth(label)/2,
			int(sy)-8,
			render.ColorMint)
	}
}

func (w *WildScreen) drawFlash(screen *ebiten.Image) {
	// Classic battle encounter flash: screen alternates white/black
	flash := ebiten.NewImage(320, 288)
	if (w.flashTick/3)%2 == 0 {
		flash.Fill(color.RGBA{R: 255, G: 255, B: 255, A: 200})
	} else {
		flash.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 200})
	}
	screen.DrawImage(flash, nil)
}

func (w *WildScreen) drawFade(screen *ebiten.Image) {
	alpha := float64(w.fadeTick) / 15.0
	if alpha > 1 { alpha = 1 }
	if alpha < 0 { alpha = 0 }
	fade := ebiten.NewImage(320, 288)
	a := uint8(alpha * 255)
	fade.Fill(color.RGBA{R: 0, G: 0, B: 0, A: a})
	screen.DrawImage(fade, nil)

	// Show area name during fade-in (when arriving at a new area).
	// Only show when nearly fully black (alpha > 0.7) so the text
	// appears briefly then fades away with the overlay.
	if alpha > 0.7 && w.area != nil {
		nameLen := len(w.area.Name) * 8
		nameX := (320 - nameLen) / 2
		render.DrawText(screen, w.area.Name, nameX, 136, render.ColorGold)
	}
}

func (w *WildScreen) drawPause(screen *ebiten.Image) {
	render.DrawBox(screen, 16, 16, 288, 256, render.ColorBoxBG, render.ColorSky)

	info := entity.ClassTable[w.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(w.player.Level), 28, 24, render.ColorPink)

	s := w.player.Stats
	y := 44
	render.DrawText(screen, "HP: "+intToStr(s.HP)+"/"+intToStr(s.MaxHP), 28, y, render.ColorGreen)
	render.DrawBar(screen, 160, y+1, 120, 7, float64(s.HP)/float64(s.MaxHP), render.ColorGreen, render.ColorDarkGray)
	y += 18
	render.DrawText(screen, "MP: "+intToStr(s.MP)+"/"+intToStr(s.MaxMP), 28, y, render.ColorSky)
	render.DrawBar(screen, 160, y+1, 120, 7, float64(s.MP)/float64(s.MaxMP), render.ColorSky, render.ColorDarkGray)
	y += 22
	render.DrawText(screen, "ATK: "+intToStr(w.player.EffectiveATK()), 28, y, render.ColorPeach)
	render.DrawText(screen, "DEF: "+intToStr(w.player.EffectiveDEF()), 160, y, render.ColorSky)
	y += 18
	render.DrawText(screen, "SPD: "+intToStr(s.SPD), 28, y, render.ColorGold)
	y += 26

	// Area info
	render.DrawText(screen, "Area: "+w.area.Name, 28, y, render.ColorMint)
	y += 22

	// Quest progress
	q := w.player.ActiveQuest()
	if q != nil {
		render.DrawText(screen, "Quest: "+q.Name, 28, y, render.ColorLavender)
		y += 18
		render.DrawText(screen, intToStr(q.Progress)+"/"+intToStr(q.Required)+" "+q.Target, 28, y, render.ColorPeach)
	}

	render.DrawText(screen, "B:Bag M:Map X:Close", 28, 256, render.ColorGray)
}

// drawDayNightTint draws a colored overlay for the current time of day.
func (w *WildScreen) drawDayNightTint(screen *ebiten.Image) {
	if w.player.DayNight == nil {
		return
	}
	tint := w.player.DayNight.TintColor()
	if tint.A == 0 {
		return // daytime, no tint
	}
	overlay := ebiten.NewImage(320, 288)
	overlay.Fill(tint)
	screen.DrawImage(overlay, nil)
}
