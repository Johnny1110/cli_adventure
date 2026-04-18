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

func (w *WildScreen) loadArea(name string) {
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

	w.tileX = area.PlayerStartX
	w.tileY = area.PlayerStartY
	w.pixelX = float64(w.tileX * render.TileSize)
	w.pixelY = float64(w.tileY * render.TileSize)
	w.stepCount = 0
	w.bossTriggered = false
	w.particles = render.NewParticleSystem(area.MapKey, area.Width, area.Height)
}

func (w *WildScreen) OnEnter() {}
func (w *WildScreen) OnExit()  {}

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
	// Check area transitions. In multiplayer, only the host initiates these;
	// clients follow via the area_change event dispatched from syncSession.
	if w.session != nil && w.session.Role() == netpkg.RoleClient {
		// Still drop treasure/fairy/boss logic on the floor for clients —
		// those are host-authoritative too.
		return
	}
	for _, conn := range w.area.Connections {
		if w.tileY == conn.FromY && w.tileX >= conn.FromMinX && w.tileX <= conn.FromMaxX {
			if conn.TargetArea == "town" {
				// Return to town. In MP, announce the move so clients follow.
				if w.session != nil && w.session.Role() == netpkg.RoleHost {
					w.session.BroadcastAreaChange("town", 9, 10)
					w.switcher.SwitchScreen(NewTownScreenMP(w.switcher, w.player, w.session))
					return
				}
				w.switcher.SwitchScreen(NewTownScreen(w.switcher, w.player))
				return
			}
			// Transition to another wild area. In MP, broadcast first so
			// clients enter the same area in their own syncSession pass.
			if w.session != nil && w.session.Role() == netpkg.RoleHost {
				sx, sy := areaStart(conn.TargetArea)
				w.session.BroadcastAreaChange(conn.TargetArea, sx, sy)
			}
			w.nextArea = conn.TargetArea
			w.fadeTick = 0
			w.fadeDir = 1
			w.state = wildTransition
			return
		}
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

	// Boss area: trigger boss encounter when player moves near center
	if w.area.MapKey == "lair" && !w.bossTriggered {
		if w.tileY <= 5 {
			w.bossTriggered = true
			w.bossDialogue = render.NewDialogueBox([]string{
				"The ground trembles...",
				"A massive Dragon\nappears before you!",
			})
			w.state = wildBossDialogue
			return
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
	// Random chest loot table
	roll := rand.Intn(100)
	var msg string
	switch {
	case roll < 30:
		// Coins
		amount := 20 + rand.Intn(30)
		w.player.Coins += amount
		msg = "Found " + intToStr(amount) + " coins!"
	case roll < 55:
		// Potion
		w.player.AddItem(entity.Item{Name: "Potion", Type: entity.ItemConsumable, StatBoost: 15, Price: 15, ClassRestrict: -1})
		msg = "Found a Potion!"
	case roll < 70:
		// Hi Potion
		w.player.AddItem(entity.Item{Name: "Hi Potion", Type: entity.ItemConsumable, StatBoost: 40, Price: 40, ClassRestrict: -1})
		msg = "Found a Hi Potion!"
	case roll < 85:
		// Crystal Sword (rare weapon!) — universal, any class
		w.player.AddItem(entity.Item{Name: "Crystal Sword", Type: entity.ItemWeapon, StatBoost: 9, Price: 200, ClassRestrict: -1})
		msg = "Found a Crystal Sword!"
	case roll < 95:
		// Dragon Scale armor (rare!) — universal, any class
		w.player.AddItem(entity.Item{Name: "Dragon Scale", Type: entity.ItemArmor, StatBoost: 9, Price: 200, ClassRestrict: -1})
		msg = "Found Dragon Scale\narmor!"
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
			// Fully faded out — load new area
			w.loadArea(w.nextArea)
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
			// Trigger boss fight
			dragon := data.MonsterTemplates["Dragon"]
			if dragon != nil {
				w.encounterMon = dragon.Clone()
				w.flashTick = 0
				w.state = wildEncounterFlash
			}
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

	// HUD
	render.DrawText(screen, w.area.Name, 4, 2, render.ColorMint)
	render.DrawText(screen, "X:Menu B:Bag E:Equip", 16, 136, render.ColorDarkGray)

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
		if sx < -16 || sx > 160 || sy < -16 || sy > 144 {
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
	flash := ebiten.NewImage(160, 144)
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
	fade := ebiten.NewImage(160, 144)
	a := uint8(alpha * 255)
	fade.Fill(color.RGBA{R: 0, G: 0, B: 0, A: a})
	screen.DrawImage(fade, nil)
}

func (w *WildScreen) drawPause(screen *ebiten.Image) {
	render.DrawBox(screen, 8, 8, 144, 128, render.ColorBoxBG, render.ColorSky)

	info := entity.ClassTable[w.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(w.player.Level), 14, 12, render.ColorPink)

	s := w.player.Stats
	y := 24
	render.DrawText(screen, "HP: "+intToStr(s.HP)+"/"+intToStr(s.MaxHP), 14, y, render.ColorGreen)
	render.DrawBar(screen, 80, y+1, 60, 5, float64(s.HP)/float64(s.MaxHP), render.ColorGreen, render.ColorDarkGray)
	y += 10
	render.DrawText(screen, "MP: "+intToStr(s.MP)+"/"+intToStr(s.MaxMP), 14, y, render.ColorSky)
	render.DrawBar(screen, 80, y+1, 60, 5, float64(s.MP)/float64(s.MaxMP), render.ColorSky, render.ColorDarkGray)
	y += 12
	render.DrawText(screen, "ATK: "+intToStr(w.player.EffectiveATK()), 14, y, render.ColorPeach)
	render.DrawText(screen, "DEF: "+intToStr(w.player.EffectiveDEF()), 80, y, render.ColorSky)
	y += 10
	render.DrawText(screen, "SPD: "+intToStr(s.SPD), 14, y, render.ColorGold)
	y += 14

	// Area info
	render.DrawText(screen, "Area: "+w.area.Name, 14, y, render.ColorMint)
	y += 12

	// Quest progress
	q := w.player.ActiveQuest()
	if q != nil {
		render.DrawText(screen, "Quest: "+q.Name, 14, y, render.ColorLavender)
		y += 10
		render.DrawText(screen, intToStr(q.Progress)+"/"+intToStr(q.Required)+" "+q.Target, 14, y, render.ColorPeach)
	}

	render.DrawText(screen, "B:Bag M:Map X:Close", 14, 128, render.ColorGray)
}
