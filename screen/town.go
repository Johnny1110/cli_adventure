// screen/town.go — The town overworld screen with player movement, NPCs, and dialogue.
//
// THEORY — Grid-based movement with smooth interpolation:
// The player moves on a tile grid (like Pokemon). When you press a direction key,
// the player's logical tile position changes instantly (if the target tile isn't
// solid), but the visual position interpolates smoothly over several frames.
// This gives the satisfying "slide into place" feel of classic Game Boy RPGs.
//
// Implementation: the player has (tileX, tileY) as their logical position and
// (pixelX, pixelY) as their visual position. Each Update() tick, pixelX/Y lerp
// toward the target (tileX*16, tileY*16). While lerping, input is locked to
// prevent diagonal movement or skipping tiles.
//
// Sub-states: The town screen has its own internal state machine:
//   - Walking: player moves around, can interact with NPCs
//   - Talking: dialogue box is active, movement is locked
//   - Shopping: shop UI is open
//   - Paused: inventory/stats overlay
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
	"cli_adventure/save"
)

// reinforceEntry pairs an inventory index with the item for the reinforce UI.
type reinforceEntry struct {
	InvIdx int         // index in player.Items (or -1 for equipped weapon, -2 for equipped armor)
	Item   *entity.Item // pointer to the actual item so we can modify EnhanceLevel
	Source string       // "weapon", "armor", or "bag"
}

type townState int

const (
	townWalking    townState = iota
	townTalking
	townShopping              // merchant buy/sell
	townBlacksmith            // blacksmith shop
	townReinforce             // blacksmith reinforce sub-menu
	townPaused
	townSaving
	townFadeOut               // fading to black before area transition
)

// direction constants
type direction int

const (
	dirDown direction = iota
	dirLeft
	dirRight
	dirUp
)

// TownScreen is the main town overworld.
type TownScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player

	// Map
	tileMap *render.TileMap
	camera  *render.Camera

	// Player position
	tileX, tileY   int     // logical grid position
	pixelX, pixelY float64 // visual position (smooth interpolation)
	moving         bool    // currently interpolating?
	facing         direction
	moveSpeed      float64 // pixels per tick during movement

	// Player sprite
	charSprites map[int]*render.SpriteSheet
	walkTick    int

	// NPCs
	npcs       []*entity.NPC
	npcSprites map[string]*render.SpriteSheet
	npcAnims   map[string]*render.Animation

	// Sub-state
	state    townState
	dialogue *render.DialogueBox

	// Shop state (merchant)
	shopItems    []entity.Item
	shopSelected int
	shopMode     int  // 0=buy list, 1=confirm
	shopSelling  bool // true = sell mode, false = buy mode

	// Blacksmith state
	smithItems    []entity.Item
	smithSelected int

	// Reinforce state
	reinforceItems []reinforceEntry // equippable items from inventory
	reinforceIdx   int

	// Interaction tracking
	interactNPC *entity.NPC

	// Save state
	saveSlotSelected int
	saveSlots        [save.MaxSlots]save.SlotSummary

	// Fade transition for area exits
	fadeTick   int
	fadeExitArea string
	fadeEntryEdge data.ExitEdge

	// Multiplayer session (nil in single-player).
	session *netpkg.Session
	// Last input we forwarded to the host (client side) — avoids spamming
	// duplicate messages every tick.
	lastDX, lastDY int
}

// NewTownScreenMP creates a multiplayer-aware town screen wired to a live
// Session. All remote players are drawn alongside the local player, and
// client-side inputs are forwarded to the host.
func NewTownScreenMP(switcher ScreenSwitcher, player *entity.Player, sess *netpkg.Session) *TownScreen {
	t := NewTownScreen(switcher, player)
	t.session = sess
	// Seed the session with our own position so the host/room sees us.
	sess.SetMyPosition("town", t.tileX, t.tileY, int(t.facing), player.Stats.HP, player.Stats.MaxHP)
	return t
}

// NewTownScreen creates the town screen.
func NewTownScreen(switcher ScreenSwitcher, player *entity.Player) *TownScreen {
	// Build the tile map
	tileset := asset.GenerateTownTileset()
	solid := data.TownSolid()

	ground := make([][]int, data.TownHeight)
	overlay := make([][]int, data.TownHeight)
	solidSlice := make([][]bool, data.TownHeight)
	for y := 0; y < data.TownHeight; y++ {
		ground[y] = make([]int, data.TownWidth)
		overlay[y] = make([]int, data.TownWidth)
		solidSlice[y] = make([]bool, data.TownWidth)
		for x := 0; x < data.TownWidth; x++ {
			ground[y][x] = data.TownGround[y][x]
			overlay[y][x] = data.TownOverlay[y][x]
			solidSlice[y][x] = solid[y][x]
		}
	}

	tm := render.NewTileMap(data.TownWidth, data.TownHeight, ground, overlay, solidSlice, tileset)
	cam := render.NewCamera(data.TownWidth, data.TownHeight)

	// Create NPCs
	npcSpawns := data.TownNPCs()
	npcs := make([]*entity.NPC, len(npcSpawns))
	for i, spawn := range npcSpawns {
		npcs[i] = &entity.NPC{
			Name:  spawn.Name,
			TileX: spawn.TileX,
			TileY: spawn.TileY,
			Role:  spawn.Role,
		}
	}

	// Starting position: center of town, on the main road
	startX, startY := 9, 10

	return &TownScreen{
		switcher:    switcher,
		player:      player,
		tileMap:     tm,
		camera:      cam,
		tileX:       startX,
		tileY:       startY,
		pixelX:      float64(startX * render.TileSize),
		pixelY:      float64(startY * render.TileSize),
		facing:      dirDown,
		moveSpeed:   2.0, // 2 pixels per tick = 8 ticks to cross a tile (smooth)
		charSprites: asset.GenerateCharSprites(),
		npcs:        npcs,
		npcSprites:  asset.GenerateNPCSprites(),
		npcAnims:    makeNPCAnims(npcs),
		shopItems:   data.ShopForClass(player.Class),
		state:       townWalking,
	}
}

func makeNPCAnims(npcs []*entity.NPC) map[string]*render.Animation {
	anims := map[string]*render.Animation{}
	for _, npc := range npcs {
		anims[npc.Role] = render.NewAnimation([]int{0, 1}, 30)
	}
	return anims
}

func (t *TownScreen) OnEnter() {
	// THEORY — Town is a safe zone but no longer auto-heals:
	// With the day/night survival system, the player must actively sleep at
	// home to heal. This adds resource management tension — you can't just
	// walk into town and be fully healed. You need to find your house and
	// rest. This makes the home building feel meaningful and the night
	// cycle consequential.
}
func (t *TownScreen) OnExit() {}

func (t *TownScreen) Update() error {
	// Update NPC animations
	for _, a := range t.npcAnims {
		a.Update()
	}
	t.walkTick++

	switch t.state {
	case townWalking:
		t.updateWalking()
	case townTalking:
		t.updateTalking()
	case townShopping:
		t.updateShopping()
	case townBlacksmith:
		t.updateBlacksmith()
	case townReinforce:
		t.updateReinforce()
	case townPaused:
		t.updatePaused()
	case townSaving:
		t.updateSaving()
	case townFadeOut:
		t.updateFadeOut()
	}

	// Update camera
	centerX := int(t.pixelX) + render.TileSize/2
	centerY := int(t.pixelY) + render.TileSize/2
	t.camera.Follow(centerX, centerY)

	// Push our position into the shared session (no-op in single-player).
	t.syncSession()

	return nil
}

func (t *TownScreen) updateWalking() {
	// Smooth movement interpolation
	if t.moving {
		targetX := float64(t.tileX * render.TileSize)
		targetY := float64(t.tileY * render.TileSize)

		dx := targetX - t.pixelX
		dy := targetY - t.pixelY

		if dx > 0 {
			t.pixelX += t.moveSpeed
			if t.pixelX >= targetX { t.pixelX = targetX }
		} else if dx < 0 {
			t.pixelX -= t.moveSpeed
			if t.pixelX <= targetX { t.pixelX = targetX }
		}
		if dy > 0 {
			t.pixelY += t.moveSpeed
			if t.pixelY >= targetY { t.pixelY = targetY }
		} else if dy < 0 {
			t.pixelY -= t.moveSpeed
			if t.pixelY <= targetY { t.pixelY = targetY }
		}

		if t.pixelX == targetX && t.pixelY == targetY {
			t.moving = false
			// Advance day/night with each step
			if t.player.DayNight != nil {
				t.player.DayNight.Step()
			}
		}
		return // no input while moving
	}

	// Check for exits — the town is a hub with four cardinal exits.
	// South → Forest (vertical chain: forest → cave → lair)
	// North → Frozen Path (vertical chain: frozen_path → snow → ice_cavern)
	// West → Desert (horizontal chain: desert ← sand_ruins ← buried_temple)
	// East → Swamp (horizontal chain: swamp → volcano)
	//
	// THEORY — Hub-and-spoke world design:
	// The town sits at the center of the world graph. Each exit leads to a
	// different biome chain. This gives the player meaningful choice about
	// which direction to explore, while the level-gating of each chain
	// creates a natural progression order. The west chain (desert) is
	// gated behind the Dragon boss to provide post-game content.

	// Determine exit area and which edge the player enters from.
	// The entry edge is the OPPOSITE of the direction the player walked:
	// walk south out of town → enter forest from the north edge.
	exitArea := ""
	var entryEdge data.ExitEdge
	switch {
	case t.tileY >= data.TownExitY:
		exitArea = "forest"      // south → east chain
		entryEdge = data.EdgeNorth // player enters forest from its north side
	case t.tileY <= data.TownExitNorthY:
		exitArea = "frozen_path"
		entryEdge = data.EdgeSouth // player enters from south
	case t.tileX <= data.TownExitWestX:
		exitArea = "desert"
		entryEdge = data.EdgeEast // walked west → enters desert from east
	case t.tileX >= data.TownExitEastX:
		exitArea = "swamp"
		entryEdge = data.EdgeWest // walked east → enters swamp from west
	}

	if exitArea != "" {
		// Boss gate check for west (desert) — requires Dragon defeated
		if exitArea == "desert" {
			if t.player.BossDefeated == nil || !t.player.BossDefeated["dragon"] {
				// Block entry — show warning dialogue
				t.dialogue = render.NewDialogueBox([]string{
					"A dark barrier blocks\nthe path west...",
					"Perhaps defeating the\nDragon will open the way.",
				})
				t.state = townTalking
				// Push player back
				t.tileX = data.TownExitWestX + 1
				t.pixelX = float64(t.tileX * render.TileSize)
				return
			}
		}

		// In multiplayer, only the host initiates area changes.
		if t.session != nil && t.session.Role() == netpkg.RoleClient {
			// Roll back — clients don't initiate transitions.
			switch exitArea {
			case "forest":
				t.tileY = data.TownExitY - 1
				t.pixelY = float64(t.tileY * render.TileSize)
			case "frozen_path":
				t.tileY = data.TownExitNorthY + 1
				t.pixelY = float64(t.tileY * render.TileSize)
			case "desert":
				t.tileX = data.TownExitWestX + 1
				t.pixelX = float64(t.tileX * render.TileSize)
			case "swamp":
				t.tileX = data.TownExitEastX - 1
				t.pixelX = float64(t.tileX * render.TileSize)
			}
			return
		}
		if t.session != nil && t.session.Role() == netpkg.RoleHost {
			sx, sy := areaStart(exitArea)
			t.session.BroadcastAreaChange(exitArea, sx, sy)
			t.switcher.SwitchScreen(NewWildScreenMP(t.switcher, t.player, exitArea, t.session))
			return
		}
		// Start fade-out transition instead of instant switch.
		// THEORY — Fade transitions mask spatial discontinuity:
		// When you walk east out of town but appear on the west edge of a
		// vertically-oriented map, the visual jump is jarring. A short fade
		// to black with the area name displayed (like Pokemon's route signs)
		// gives the player's brain a "loading moment" to reset spatial
		// expectations. The area name reinforces where they are now.
		t.fadeExitArea = exitArea
		t.fadeEntryEdge = entryEdge
		t.fadeTick = 0
		t.state = townFadeOut
		return
	}

	// Movement input
	newX, newY := t.tileX, t.tileY
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		newY--
		t.facing = dirUp
	} else if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		newY++
		t.facing = dirDown
	} else if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		newX--
		t.facing = dirLeft
	} else if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		newX++
		t.facing = dirRight
	}

	if newX != t.tileX || newY != t.tileY {
		// Check collision with map (and, in multiplayer, other players)
		if !t.tileMap.IsSolid(newX, newY) && !t.isNPCAt(newX, newY) && !t.isRemoteAt(newX, newY) {
			dx := newX - t.tileX
			dy := newY - t.tileY
			t.tileX = newX
			t.tileY = newY
			t.moving = true
			// Forward input to the host (client side only — the host
			// applies its movement locally).
			if t.session != nil && t.session.Role() == netpkg.RoleClient {
				t.session.SubmitInput(netpkg.InputMsg{DX: dx, DY: dy})
			}
			// Auto-trigger wormhole when stepping onto the tile.
			if t.tileX == data.TownWormholeX && t.tileY == data.TownWormholeY {
				t.openWormhole()
				return
			}
		}
	}

	// Interact with NPC (Z key)
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		t.tryInteract()
	}

	// Pause menu (X key or Escape)
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.state = townPaused
	}

	// Map (M key)
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		t.switcher.SwitchScreen(NewMapScreen(t.switcher, t, "town"))
	}

	// Bag (B key)
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		t.switcher.SwitchScreen(NewBagScreen(t.switcher, t.player, t))
	}

	// Equipment (E key)
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		t.switcher.SwitchScreen(NewEquipScreen(t.switcher, t.player, t))
	}

	// Skill tree (T key)
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		t.switcher.SwitchScreen(NewSkillScreen(t.switcher, t.player, t))
	}
}

func (t *TownScreen) tryInteract() {
	// Check the tile the player is facing
	fx, fy := t.tileX, t.tileY
	switch t.facing {
	case dirUp:
		fy--
	case dirDown:
		fy++
	case dirLeft:
		fx--
	case dirRight:
		fx++
	}

	for _, npc := range t.npcs {
		if npc.TileX == fx && npc.TileY == fy {
			t.interactNPC = npc
			switch npc.Role {
			case "merchant":
				t.startMerchantDialogue()
			case "elder":
				t.startElderDialogue()
			case "home":
				t.startHomeDialogue()
			case "blacksmith":
				t.startBlacksmithDialogue()
			case "innkeeper":
				t.startInnDialogue()
			}
			return
		}
	}

	// Check special tiles
	if fy >= 0 && fy < data.TownHeight && fx >= 0 && fx < data.TownWidth {
		// Save crystal
		if data.TownGround[fy][fx] == asset.TileSaveCrystal {
			t.startSaveDialogue()
			return
		}
		if data.TownGround[fy][fx] == asset.TileSign {
			t.dialogue = render.NewDialogueBox([]string{
				"Peaceful Village",
				"A quiet town at the\nedge of the forest.",
			})
			t.state = townTalking
			return
		}
		// Wormhole: opens the multiplayer lobby. Works both facing the
		// tile and standing on it (the player naturally walks onto it).
		if data.TownGround[fy][fx] == asset.TileWormhole {
			t.openWormhole()
			return
		}
	}
	// Also trigger wormhole if we're standing on it.
	if t.tileX == data.TownWormholeX && t.tileY == data.TownWormholeY {
		t.openWormhole()
	}
}

// openWormhole transitions to the multiplayer menu. If we already have a
// session, we stay in town — the wormhole tile becomes inert (a visual
// reminder of the active room) after you've entered.
func (t *TownScreen) openWormhole() {
	if t.session != nil {
		// Already in a session — show a brief hint instead of reopening.
		t.dialogue = render.NewDialogueBox([]string{
			"The wormhole hums.",
			"You are already\nlinked to a room.",
		})
		t.state = townTalking
		return
	}
	t.switcher.SwitchScreen(NewWormholeScreen(t.switcher, t.player, t))
}

func (t *TownScreen) startMerchantDialogue() {
	// Shop closed at night — survival mechanic!
	if t.player.DayNight != nil && t.player.DayNight.IsNight() {
		t.dialogue = render.NewDialogueBox([]string{
			"Sorry, shop is closed\nat night.",
			"Come back during\nthe day!",
		})
		t.state = townTalking
		return
	}
	t.dialogue = render.NewChoiceBox(
		"Welcome! Want to\nbuy or sell?",
		[]string{"Buy", "Sell", "No thanks"},
	)
	t.state = townTalking
}

func (t *TownScreen) startBlacksmithDialogue() {
	t.dialogue = render.NewChoiceBox(
		"I forge mighty weapons.\nWhat do you need?",
		[]string{"Buy", "Reinforce", "Leave"},
	)
	t.state = townTalking
}

func (t *TownScreen) startInnDialogue() {
	healCost := t.player.InnHealCost()
	buffCost := t.player.InnBuffCost()
	// THEORY — Inn as preparation ritual:
	// The Inn gives two services: a basic heal (cheaper than potions at high
	// levels) and a combat buff meal. The buff costs more but gives a tangible
	// edge for the next few fights. This creates a "preparation loop" before
	// boss attempts: save at the crystal, eat at the inn, then go fight.
	t.dialogue = render.NewChoiceBox(
		"Welcome to the inn!\nHow can I help?",
		[]string{
			"Heal (" + intToStr(healCost) + "G)",
			"Buff (" + intToStr(buffCost) + "G)",
			"Leave",
		},
	)
	t.state = townTalking
}

func (t *TownScreen) startElderDialogue() {
	q := t.player.ActiveQuest()
	if q != nil && q.IsComplete() {
		// Turn in quest
		t.player.CompleteQuest(q)
		t.dialogue = render.NewDialogueBox([]string{
			"Well done, hero!",
			"Here is your reward.",
			"Come back when you\nare ready for more.",
		})
		t.state = townTalking
		return
	}
	if q != nil {
		// Quest in progress
		t.dialogue = render.NewDialogueBox([]string{
			q.Desc,
			"Progress: " + intToStr(q.Progress) + "/" + intToStr(q.Required),
			"Keep going, hero!",
		})
		t.state = townTalking
		return
	}
	// Offer new quest
	available := entity.AvailableQuests()
	// Find next unfinished quest
	for _, aq := range available {
		alreadyDone := false
		for _, pq := range t.player.Quests {
			if pq.Name == aq.Name && pq.Done {
				alreadyDone = true
				break
			}
		}
		if !alreadyDone {
			t.dialogue = render.NewChoiceBox(
				aq.Desc,
				[]string{"Accept", "Decline"},
			)
			t.state = townTalking
			return
		}
	}
	// All quests done
	t.dialogue = render.NewDialogueBox([]string{
		"You have completed\nall my requests.",
		"The village is safe\nthanks to you!",
	})
	t.state = townTalking
}

func (t *TownScreen) startSaveDialogue() {
	// Scan existing save slots for the UI
	t.saveSlots = save.ListSlots()
	t.saveSlotSelected = 0
	t.state = townSaving
}

func (t *TownScreen) updateSaving() {
	// Navigate save slots (3 slots + Cancel)
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		t.saveSlotSelected--
		if t.saveSlotSelected < 0 {
			t.saveSlotSelected = save.MaxSlots // Cancel option
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		t.saveSlotSelected++
		if t.saveSlotSelected > save.MaxSlots {
			t.saveSlotSelected = 0
		}
	}

	// Confirm
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if t.saveSlotSelected >= save.MaxSlots {
			// Cancel
			t.state = townWalking
			return
		}
		// Save to selected slot
		sd := save.SnapshotPlayer(t.player, "town")
		err := save.Save(t.saveSlotSelected, sd)
		if err != nil {
			t.dialogue = render.NewDialogueBox([]string{
				"Save failed!",
				"Could not write\nsave data.",
			})
		} else {
			t.dialogue = render.NewDialogueBox([]string{
				"Game saved to\nSlot " + intToStr(t.saveSlotSelected+1) + "!",
				"Your progress is\nsafe now.",
			})
			// Refresh slot summaries
			t.saveSlots = save.ListSlots()
		}
		t.state = townTalking
	}

	// Cancel with X
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.state = townWalking
	}
}

// updateFadeOut handles the fade-to-black transition when exiting town.
// THEORY — Two-phase fade with area name banner:
// Phase 1 (ticks 0–15): screen fades to black, player movement locked.
// Phase 2 (ticks 16–45): fully black with area name displayed, like
// Pokemon's "Route X" popup. This gives the player 0.5s to read where
// they're going, which resets spatial expectations before the new map loads.
// At the end, we switch to the wild screen (which does its own fade-in).
func (t *TownScreen) updateFadeOut() {
	t.fadeTick++
	if t.fadeTick >= 45 {
		// Transition complete — switch to wild screen
		t.switcher.SwitchScreen(NewWildScreenFromEdge(t.switcher, t.player, t.fadeExitArea, t.fadeEntryEdge))
	}
}

// drawFadeOut renders the fade-to-black overlay and area name banner.
func (t *TownScreen) drawFadeOut(screen *ebiten.Image) {
	// Fade alpha: ramp from 0 to 255 over the first 15 ticks, then hold.
	alpha := t.fadeTick * 17 // ~255 at tick 15
	if alpha > 255 {
		alpha = 255
	}
	// Draw a black overlay with increasing opacity
	for y := 0; y < 288; y++ {
		for x := 0; x < 320; x++ {
			screen.Set(x, y, color.RGBA{0, 0, 0, uint8(alpha)})
		}
	}

	// Show area name once fully faded (ticks 16+)
	if t.fadeTick >= 16 {
		// Look up the display name from the world graph
		areaName := t.fadeExitArea
		if wa, ok := data.WorldGraph[t.fadeExitArea]; ok {
			areaName = wa.Name
		}
		// Center the name on screen
		nameLen := len(areaName) * 8 // approximate pixel width (8px per char)
		nameX := (320 - nameLen) / 2
		render.DrawText(screen, areaName, nameX, 136, render.ColorGold)
	}
}

func (t *TownScreen) startHomeDialogue() {
	prompt := "Your cozy home.\nRest in bed?"
	if t.player.DayNight != nil && t.player.DayNight.IsNight() {
		prompt = "It's dangerous at\nnight. Sleep till dawn?"
	}
	t.dialogue = render.NewChoiceBox(
		prompt,
		[]string{"Sleep", "Not now"},
	)
	t.state = townTalking
}

func (t *TownScreen) updateTalking() {
	if t.dialogue == nil {
		t.state = townWalking
		return
	}
	t.dialogue.Update()

	if t.dialogue.Finished {
		// Handle choice results
		if t.dialogue.ChoiceMade {
			if t.interactNPC != nil && t.interactNPC.Role == "home" {
				if t.dialogue.SelectedChoice == 0 { // "Sleep"
					t.player.Stats.HP = t.player.Stats.MaxHP
					t.player.Stats.MP = t.player.Stats.MaxMP
					// Advance night → dawn if it's nighttime
					wasNight := false
					if t.player.DayNight != nil && t.player.DayNight.IsNight() {
						t.player.DayNight.Sleep()
						wasNight = true
					}
					msgs := []string{
						"You rest in your bed.",
						"HP and MP fully\nrestored!",
					}
					if wasNight {
						msgs = append(msgs, "Dawn breaks...\nA new day begins!")
					} else {
						msgs = append(msgs, "You feel refreshed!")
					}
					t.dialogue = render.NewDialogueBox(msgs)
					t.state = townTalking
					t.interactNPC = nil
					return
				}
			}
			if t.interactNPC != nil && t.interactNPC.Role == "merchant" {
				if t.dialogue.SelectedChoice == 0 { // "Buy"
					t.shopSelected = 0
					t.shopSelling = false
					t.state = townShopping
					t.dialogue = nil
					return
				}
				if t.dialogue.SelectedChoice == 1 { // "Sell"
					if len(t.player.Items) == 0 {
						t.dialogue = render.NewDialogueBox([]string{
							"You have nothing\nto sell!",
						})
						t.state = townTalking
						t.interactNPC = nil
						return
					}
					t.shopSelected = 0
					t.shopSelling = true
					t.state = townShopping
					t.dialogue = nil
					return
				}
			}
			if t.interactNPC != nil && t.interactNPC.Role == "blacksmith" {
				if t.dialogue.SelectedChoice == 0 { // "Buy"
					t.smithItems = data.BlacksmithForClass(t.player.Class, t.player.BossDefeated)
					t.smithSelected = 0
					t.state = townBlacksmith
					t.dialogue = nil
					return
				}
				if t.dialogue.SelectedChoice == 1 { // "Reinforce"
					t.buildReinforceList()
					if len(t.reinforceItems) == 0 {
						t.dialogue = render.NewDialogueBox([]string{
							"You have no weapons\nor armor to reinforce!",
						})
						t.state = townTalking
						t.interactNPC = nil
						return
					}
					t.reinforceIdx = 0
					t.state = townReinforce
					t.dialogue = nil
					return
				}
			}
			if t.interactNPC != nil && t.interactNPC.Role == "innkeeper" {
				if t.dialogue.SelectedChoice == 0 { // "Heal"
					cost := t.player.InnHealCost()
					if t.player.Coins >= cost {
						t.player.Coins -= cost
						t.player.Stats.HP = t.player.Stats.MaxHP
						t.player.Stats.MP = t.player.Stats.MaxMP
						t.dialogue = render.NewDialogueBox([]string{
							"You rest at the inn.",
							"HP and MP fully\nrestored!",
						})
					} else {
						t.dialogue = render.NewDialogueBox([]string{
							"Not enough coins!",
						})
					}
					t.state = townTalking
					t.interactNPC = nil
					return
				}
				if t.dialogue.SelectedChoice == 1 { // "Buff"
					cost := t.player.InnBuffCost()
					if t.player.Coins >= cost {
						t.player.Coins -= cost
						t.player.InnATKBuff = 3 + t.player.Level/5
						t.player.InnDEFBuff = 3 + t.player.Level/5
						t.player.InnBuffFights = 5
						t.dialogue = render.NewDialogueBox([]string{
							"A hearty meal!",
							"ATK+" + intToStr(t.player.InnATKBuff) +
								" DEF+" + intToStr(t.player.InnDEFBuff) +
								"\nfor 5 fights!",
						})
					} else {
						t.dialogue = render.NewDialogueBox([]string{
							"Not enough coins!",
						})
					}
					t.state = townTalking
					t.interactNPC = nil
					return
				}
			}
			if t.interactNPC != nil && t.interactNPC.Role == "elder" {
				if t.dialogue.SelectedChoice == 0 { // "Accept"
					// Add quest
					available := entity.AvailableQuests()
					for _, aq := range available {
						alreadyHave := false
						for _, pq := range t.player.Quests {
							if pq.Name == aq.Name {
								alreadyHave = true
								break
							}
						}
						if !alreadyHave {
							t.player.Quests = append(t.player.Quests, aq)
							t.dialogue = render.NewDialogueBox([]string{
								"Quest accepted!",
								"Good luck, hero!",
							})
							t.state = townTalking
							t.interactNPC = nil
							return
						}
					}
				}
			}
		}
		t.dialogue = nil
		t.interactNPC = nil
		t.state = townWalking
	}
}

func (t *TownScreen) updateShopping() {
	if t.shopSelling {
		t.updateSellMode()
		return
	}

	// Navigate shop items (buy mode)
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		t.shopSelected--
		if t.shopSelected < 0 {
			t.shopSelected = len(t.shopItems)  // last entry = "Exit"
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		t.shopSelected++
		if t.shopSelected > len(t.shopItems) {
			t.shopSelected = 0
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if t.shopSelected >= len(t.shopItems) {
			// Exit shop
			t.state = townWalking
			return
		}
		item := t.shopItems[t.shopSelected]
		if t.player.Coins >= item.Price {
			t.player.Coins -= item.Price
			t.player.AddItem(item)
			// Auto-equip if better (compare effective stats so reinforced gear isn't replaced)
			if item.Type == entity.ItemWeapon {
				if t.player.Weapon == nil || item.EffectiveStatBoost() > t.player.Weapon.EffectiveStatBoost() {
					t.player.Equip(item)
				}
			} else if item.Type == entity.ItemArmor {
				if t.player.Armor == nil || item.EffectiveStatBoost() > t.player.Armor.EffectiveStatBoost() {
					t.player.Equip(item)
				}
			}
			t.dialogue = render.NewDialogueBox([]string{
				"Bought " + item.Name + "!",
			})
			t.state = townTalking
		} else {
			t.dialogue = render.NewDialogueBox([]string{
				"Not enough coins!",
			})
			t.state = townTalking
		}
	}

	// X to exit shop
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.state = townWalking
	}
}

// updateSellMode handles the sell sub-mode of the merchant shop.
//
// THEORY — Sell price at 50%:
// Half-price selling is the universal RPG convention (Dragon Quest, Pokemon,
// Final Fantasy all use it). It prevents infinite money exploits (buy for X,
// sell for X, repeat), forces forward progression ("grind for money, don't
// arbitrage"), and makes buying decisions feel weighty since you'll lose
// half the value if you change your mind.
func (t *TownScreen) updateSellMode() {
	items := t.player.Items
	maxIdx := len(items) // last = "Exit"

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		t.shopSelected--
		if t.shopSelected < 0 {
			t.shopSelected = maxIdx
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		t.shopSelected++
		if t.shopSelected > maxIdx {
			t.shopSelected = 0
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if t.shopSelected >= len(items) {
			t.state = townWalking
			return
		}
		item := items[t.shopSelected]
		sellPrice := item.Price / 2
		if sellPrice < 1 {
			sellPrice = 1
		}
		t.player.Coins += sellPrice
		t.player.Items = append(t.player.Items[:t.shopSelected], t.player.Items[t.shopSelected+1:]...)
		// Adjust cursor if at end
		if t.shopSelected >= len(t.player.Items) && t.shopSelected > 0 {
			t.shopSelected--
		}
		t.dialogue = render.NewDialogueBox([]string{
			"Sold " + item.Name + "\nfor " + intToStr(sellPrice) + "G!",
		})
		t.state = townTalking
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.state = townWalking
	}
}

// updateBlacksmith handles the blacksmith shop navigation.
func (t *TownScreen) updateBlacksmith() {
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		t.smithSelected--
		if t.smithSelected < 0 {
			t.smithSelected = len(t.smithItems) // Exit
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		t.smithSelected++
		if t.smithSelected > len(t.smithItems) {
			t.smithSelected = 0
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if t.smithSelected >= len(t.smithItems) {
			t.state = townWalking
			return
		}
		item := t.smithItems[t.smithSelected]
		if t.player.Coins >= item.Price {
			t.player.Coins -= item.Price
			t.player.AddItem(item)
			// Auto-equip if better (compare effective stats so reinforced gear isn't replaced)
			if item.Type == entity.ItemWeapon {
				if t.player.Weapon == nil || item.EffectiveStatBoost() > t.player.Weapon.EffectiveStatBoost() {
					t.player.Equip(item)
				}
			} else if item.Type == entity.ItemArmor {
				if t.player.Armor == nil || item.EffectiveStatBoost() > t.player.Armor.EffectiveStatBoost() {
					t.player.Equip(item)
				}
			}
			t.dialogue = render.NewDialogueBox([]string{
				"Forged " + item.Name + "!",
			})
			t.state = townTalking
		} else {
			t.dialogue = render.NewDialogueBox([]string{
				"Not enough coins!",
			})
			t.state = townTalking
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.state = townWalking
	}
}

// buildReinforceList collects all equippable items (equipped + in bag) for
// the reinforcement UI. Each entry carries a pointer to the actual Item so
// we can modify EnhanceLevel in place.
func (t *TownScreen) buildReinforceList() {
	t.reinforceItems = nil
	// Currently equipped items (all 6 slots)
	equippedSlots := []*entity.Item{
		t.player.Weapon, t.player.Armor, t.player.Helmet,
		t.player.Boots, t.player.Shield, t.player.Accessory,
	}
	for idx, slot := range equippedSlots {
		if slot != nil {
			t.reinforceItems = append(t.reinforceItems, reinforceEntry{
				InvIdx: -(idx + 1), // -1=weapon, -2=armor, -3=helmet, ...
				Item:   slot,
				Source: "equipped",
			})
		}
	}
	// Equipment in bag (all types except consumables)
	for i := range t.player.Items {
		item := &t.player.Items[i]
		if item.Type != entity.ItemConsumable {
			t.reinforceItems = append(t.reinforceItems, reinforceEntry{
				InvIdx: i,
				Item:   item,
				Source: "bag",
			})
		}
	}
}

// updateReinforce handles the reinforcement sub-menu.
//
// THEORY — The reinforce loop:
// Select item → see cost + success rate → press Z to attempt → roll dice.
// Success: level goes up, stats improve, "Success!" message.
// Failure: gold is lost but item is kept, "Failed..." message.
// This creates a high-stakes gambling loop that's deeply addictive in games
// like MapleStory and Black Desert Online. The "one more try" urge is
// powered by sunk cost fallacy and variable reinforcement (psychology term
// for random rewards, the most addictive reward schedule known).
func (t *TownScreen) updateReinforce() {
	maxIdx := len(t.reinforceItems) // last = "Exit"

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		t.reinforceIdx--
		if t.reinforceIdx < 0 {
			t.reinforceIdx = maxIdx
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		t.reinforceIdx++
		if t.reinforceIdx > maxIdx {
			t.reinforceIdx = 0
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if t.reinforceIdx >= len(t.reinforceItems) {
			// Exit
			t.state = townWalking
			return
		}

		entry := t.reinforceItems[t.reinforceIdx]
		item := entry.Item
		cost := item.ReinforceCost()

		if t.player.Coins < cost {
			t.dialogue = render.NewDialogueBox([]string{
				"Not enough coins!",
				"You need " + intToStr(cost) + "G.",
			})
			t.state = townTalking
			return
		}

		// Deduct cost
		t.player.Coins -= cost

		// Roll for success
		successRate := item.ReinforceSuccessRate()
		roll := rand.Float64()

		if roll < successRate {
			// Success!
			item.EnhanceLevel++
			newStat := item.EffectiveStatBoost()
			statType := "ATK"
			if item.Type != entity.ItemWeapon {
				statType = "DEF"
			}
			t.dialogue = render.NewDialogueBox([]string{
				"Reinforcement success!",
				item.DisplayName() + "!",
				statType + " is now " + intToStr(newStat) + "!",
			})
		} else {
			// Failure — lost gold, item unchanged
			t.dialogue = render.NewDialogueBox([]string{
				"Reinforcement failed...",
				"The materials\nwere wasted.",
				"Your " + item.DisplayName() +
					"\nis unharmed.",
			})
		}
		t.state = townTalking
		t.interactNPC = nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.state = townWalking
	}
}

func (t *TownScreen) updatePaused() {
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.state = townWalking
	}
	// Open bag from pause
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		t.state = townWalking // reset state before switching
		t.switcher.SwitchScreen(NewBagScreen(t.switcher, t.player, t))
	}
	// Open map from pause
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		t.state = townWalking
		t.switcher.SwitchScreen(NewMapScreen(t.switcher, t, "town"))
	}
}

func (t *TownScreen) isNPCAt(x, y int) bool {
	for _, npc := range t.npcs {
		if npc.TileX == x && npc.TileY == y {
			return true
		}
	}
	return false
}

// isRemoteAt returns true when another multiplayer peer occupies the tile.
// Prevents the local player from walking "through" a remote co-op player.
func (t *TownScreen) isRemoteAt(x, y int) bool {
	if t.session == nil {
		return false
	}
	for _, p := range t.session.RemotePlayers("town") {
		if p.TileX == x && p.TileY == y {
			return true
		}
	}
	return false
}

// syncSession pushes the local player's position into the shared Session
// so the host/broadcaster can include us in the authoritative snapshot.
// Called once per tick.
func (t *TownScreen) syncSession() {
	if t.session == nil {
		return
	}
	t.session.SetMyPosition("town", t.tileX, t.tileY, int(t.facing),
		t.player.Stats.HP, t.player.Stats.MaxHP)
	// Drain peer-level events so screens can react (e.g. combat_start).
	for _, ev := range t.session.PopEvents() {
		switch ev.Kind {
		case "combat_start":
			// Host opened combat — all clients follow them into the
			// shared team-play battle screen. The host already switched
			// its own screen when the fight began.
			if t.session.Role() == netpkg.RoleClient {
				t.switcher.SwitchScreen(NewCombatMPScreen(
					t.switcher, t.player, t.session,
					"town", t.tileX, t.tileY,
				))
			}
		case "area_change":
			if t.session.Role() == netpkg.RoleClient && ev.Area != "" && ev.Area != "town" {
				t.switcher.SwitchScreen(NewWildScreenMP(t.switcher, t.player, ev.Area, t.session))
			}
		}
	}
}

// ---- Draw ----

func (t *TownScreen) Draw(screen *ebiten.Image) {
	// Draw ground layer
	t.tileMap.Draw(screen, t.camera.X, t.camera.Y)

	// Draw NPCs
	t.drawNPCs(screen)

	// Draw remote players (co-op party members) before the local player
	// so the local sprite renders on top when they overlap (purely visual).
	t.drawRemotePlayers(screen)

	// Draw player
	t.drawPlayer(screen)

	// Draw overlay layer (on top of entities)
	t.tileMap.DrawOverlay(screen, t.camera.X, t.camera.Y)

	// Day/night tint overlay
	t.drawDayNightTint(screen)

	// Draw HUD
	t.drawHUD(screen)

	// Draw sub-state overlays
	switch t.state {
	case townTalking:
		if t.dialogue != nil {
			t.dialogue.Draw(screen)
		}
	case townShopping:
		t.drawShop(screen)
	case townBlacksmith:
		t.drawBlacksmith(screen)
	case townReinforce:
		t.drawReinforce(screen)
	case townPaused:
		t.drawPause(screen)
	case townSaving:
		t.drawSaveSlots(screen)
	case townFadeOut:
		t.drawFadeOut(screen)
	}
}

func (t *TownScreen) drawPlayer(screen *ebiten.Image) {
	sheet := t.charSprites[int(t.player.Class)]
	frame := 0
	if t.moving && (t.walkTick/8)%2 == 1 {
		frame = 1 // alternate walk frame
	}

	// Screen position = world position - camera
	sx := t.pixelX - float64(t.camera.X)
	sy := t.pixelY - float64(t.camera.Y)
	sheet.DrawFrame(screen, frame, sx, sy)
}

// drawRemotePlayers paints every other party member on the town map.
// We reuse the same character sprite sheets (Knight/Mage/Archer) so a
// remote Knight looks like the local Knight. A small name tag floats
// above their head so you can tell who's who at a glance.
func (t *TownScreen) drawRemotePlayers(screen *ebiten.Image) {
	if t.session == nil {
		return
	}
	for _, rp := range t.session.RemotePlayers("town") {
		sheet, ok := t.charSprites[rp.Class]
		if !ok {
			continue
		}
		sx := float64(rp.TileX*render.TileSize) - float64(t.camera.X)
		sy := float64(rp.TileY*render.TileSize) - float64(t.camera.Y)
		// Skip off-screen draws — screen is 320x288.
		if sx < -16 || sx > 320 || sy < -16 || sy > 288 {
			continue
		}
		sheet.DrawFrame(screen, 0, sx, sy)
		// Name tag
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

func (t *TownScreen) drawNPCs(screen *ebiten.Image) {
	for _, npc := range t.npcs {
		sprite, ok := t.npcSprites[npc.Role]
		if !ok {
			continue
		}
		anim, ok := t.npcAnims[npc.Role]
		frame := 0
		if ok {
			frame = anim.CurrentFrame()
		}

		sx := float64(npc.TileX*render.TileSize) - float64(t.camera.X)
		sy := float64(npc.TileY*render.TileSize) - float64(t.camera.Y)
		sprite.DrawFrame(screen, frame, sx, sy)
	}
}

func (t *TownScreen) drawHUD(screen *ebiten.Image) {
	// Small top-left HUD: location name
	render.DrawText(screen, "Peaceful Village", 8, 4, render.ColorMint)

	// Time of day (top-center)
	if t.player.DayNight != nil {
		phaseName := t.player.DayNight.PhaseName()
		phaseClr := render.ColorWhite
		switch t.player.DayNight.Phase {
		case entity.PhaseNight:
			phaseClr = render.ColorSky
		case entity.PhaseDusk:
			phaseClr = render.ColorPeach
		case entity.PhaseDawn:
			phaseClr = render.ColorPink
		}
		render.DrawText(screen, phaseName, 152, 4, phaseClr)
	}

	// Coins display (top-right)
	coinText := intToStr(t.player.Coins) + "G"
	render.DrawText(screen, coinText, 320-render.TextWidth(coinText)-8, 4, render.ColorGold)

	// Inn buff indicator
	if t.player.HasInnBuff() {
		buffText := "BUFF:" + intToStr(t.player.InnBuffFights)
		render.DrawText(screen, buffText, 320-render.TextWidth(buffText)-8, 14, render.ColorMint)
	}

	// Bottom-center: key hints
	hintText := "Z:Talk X:Menu E:Equip"
	render.DrawText(screen, hintText, 160-render.TextWidth(hintText)/2, 276, render.ColorDarkGray)

	// "Linked" indicator when a multiplayer session is live.
	if t.session != nil {
		peers := t.session.RemotePlayers("")
		render.DrawText(screen, "LINK "+intToStr(len(peers)+1), 8, 18, render.ColorLavender)
	}
}

func (t *TownScreen) drawShop(screen *ebiten.Image) {
	if t.shopSelling {
		t.drawSellShop(screen)
		return
	}

	// Buy mode — shop overlay
	render.DrawBox(screen, 30, 20, 260, 248, render.ColorBoxBG, render.ColorGold)
	render.DrawText(screen, "Shop - Buy", 124, 28, render.ColorGold)

	// Coins
	render.DrawText(screen, "Coins: "+intToStr(t.player.Coins)+"G", 42, 46, render.ColorWhite)

	// Item list (scroll if too many)
	maxShow := 8
	startIdx := 0
	if t.shopSelected >= maxShow && t.shopSelected < len(t.shopItems) {
		startIdx = t.shopSelected - maxShow + 1
	}

	for i := startIdx; i < len(t.shopItems) && i < startIdx+maxShow; i++ {
		item := t.shopItems[i]
		y := 68 + (i-startIdx)*18
		clr := render.ColorWhite
		if i == t.shopSelected {
			render.DrawCursor(screen, 38, y, render.ColorGold)
			clr = render.ColorGold
		}

		typeName := "WPN"
		if item.Type == entity.ItemArmor {
			typeName = "ARM"
		} else if item.Type == entity.ItemConsumable {
			typeName = "USE"
		}

		render.DrawText(screen, item.Name, 52, y, clr)
		render.DrawText(screen, typeName, 192, y, render.ColorGray)
		render.DrawText(screen, intToStr(item.Price)+"G", 230, y, render.ColorPeach)
	}

	// Exit option
	exitY := 68 + min(len(t.shopItems)-startIdx, maxShow)*18
	exitClr := render.ColorWhite
	if t.shopSelected >= len(t.shopItems) {
		render.DrawCursor(screen, 38, exitY, render.ColorGold)
		exitClr = render.ColorGold
	}
	render.DrawText(screen, "Exit", 52, exitY, exitClr)

	// Selected item stats
	if t.shopSelected < len(t.shopItems) {
		item := t.shopItems[t.shopSelected]
		sy := 228
		statLabel := "ATK+"
		if item.Type == entity.ItemArmor {
			statLabel = "DEF+"
		} else if item.Type == entity.ItemConsumable {
			if item.Consumable == entity.ConsumeMP {
				statLabel = "MP +"
			} else {
				statLabel = "HP +"
			}
		}
		render.DrawText(screen, statLabel+intToStr(item.StatBoost), 42, sy, render.ColorSky)
	}

	render.DrawText(screen, "Z:Buy  X:Exit", 110, 248, render.ColorGray)
}

func (t *TownScreen) drawSellShop(screen *ebiten.Image) {
	render.DrawBox(screen, 30, 20, 260, 248, render.ColorBoxBG, render.ColorPeach)
	render.DrawText(screen, "Sell Items", 124, 28, render.ColorPeach)
	render.DrawText(screen, "Coins: "+intToStr(t.player.Coins)+"G", 42, 46, render.ColorWhite)

	items := t.player.Items
	maxShow := 8
	startIdx := 0
	if t.shopSelected >= maxShow && t.shopSelected < len(items) {
		startIdx = t.shopSelected - maxShow + 1
	}

	for i := startIdx; i < len(items) && i < startIdx+maxShow; i++ {
		item := items[i]
		y := 68 + (i-startIdx)*18
		clr := render.ColorWhite
		if i == t.shopSelected {
			render.DrawCursor(screen, 38, y, render.ColorPeach)
			clr = render.ColorGold
		}
		sellPrice := item.Price / 2
		if sellPrice < 1 {
			sellPrice = 1
		}
		render.DrawText(screen, item.DisplayName(), 52, y, clr)
		render.DrawText(screen, intToStr(sellPrice)+"G", 230, y, render.ColorGold)
	}

	// Exit option
	exitY := 68 + min(len(items)-startIdx, maxShow)*18
	exitClr := render.ColorWhite
	if t.shopSelected >= len(items) {
		render.DrawCursor(screen, 38, exitY, render.ColorPeach)
		exitClr = render.ColorGold
	}
	render.DrawText(screen, "Exit", 52, exitY, exitClr)

	render.DrawText(screen, "Z:Sell  X:Exit", 110, 248, render.ColorGray)
}

func (t *TownScreen) drawBlacksmith(screen *ebiten.Image) {
	render.DrawBox(screen, 30, 20, 260, 248, render.ColorBoxBG, render.ColorRed)
	render.DrawText(screen, "Blacksmith", 120, 28, render.ColorRed)
	render.DrawText(screen, "Coins: "+intToStr(t.player.Coins)+"G", 42, 46, render.ColorWhite)

	if len(t.smithItems) == 0 {
		render.DrawText(screen, "No items available.", 42, 80, render.ColorGray)
		render.DrawText(screen, "Defeat bosses to\nunlock new gear!", 42, 100, render.ColorGray)
	}

	maxShow := 8
	startIdx := 0
	if t.smithSelected >= maxShow && t.smithSelected < len(t.smithItems) {
		startIdx = t.smithSelected - maxShow + 1
	}

	for i := startIdx; i < len(t.smithItems) && i < startIdx+maxShow; i++ {
		item := t.smithItems[i]
		y := 68 + (i-startIdx)*18
		clr := render.ColorWhite
		if i == t.smithSelected {
			render.DrawCursor(screen, 38, y, render.ColorRed)
			clr = render.ColorGold
		}

		typeName := "WPN"
		if item.Type == entity.ItemArmor {
			typeName = "ARM"
		}

		render.DrawText(screen, item.Name, 52, y, clr)
		render.DrawText(screen, typeName, 192, y, render.ColorGray)
		render.DrawText(screen, intToStr(item.Price)+"G", 230, y, render.ColorPeach)
	}

	exitY := 68 + min(len(t.smithItems)-startIdx, maxShow)*18
	exitClr := render.ColorWhite
	if t.smithSelected >= len(t.smithItems) {
		render.DrawCursor(screen, 38, exitY, render.ColorRed)
		exitClr = render.ColorGold
	}
	render.DrawText(screen, "Exit", 52, exitY, exitClr)

	// Selected item stats
	if t.smithSelected < len(t.smithItems) {
		item := t.smithItems[t.smithSelected]
		sy := 228
		statLabel := "ATK+"
		if item.Type == entity.ItemArmor {
			statLabel = "DEF+"
		}
		render.DrawText(screen, statLabel+intToStr(item.StatBoost), 42, sy, render.ColorSky)
	}

	render.DrawText(screen, "Z:Buy  X:Exit", 110, 248, render.ColorGray)
}

func (t *TownScreen) drawReinforce(screen *ebiten.Image) {
	render.DrawBox(screen, 20, 16, 280, 256, render.ColorBoxBG, render.ColorRed)
	render.DrawText(screen, "Reinforce", 124, 22, render.ColorRed)
	render.DrawText(screen, "Coins: "+intToStr(t.player.Coins)+"G", 32, 40, render.ColorWhite)

	// Item list
	maxShow := 5
	startIdx := 0
	if t.reinforceIdx >= maxShow && t.reinforceIdx < len(t.reinforceItems) {
		startIdx = t.reinforceIdx - maxShow + 1
	}

	for i := startIdx; i < len(t.reinforceItems) && i < startIdx+maxShow; i++ {
		entry := t.reinforceItems[i]
		item := entry.Item
		y := 60 + (i-startIdx)*20

		// Use rarity color for item name; gold when cursor is on it
		clr := render.RarityColor(int(item.Rarity))
		if i == t.reinforceIdx {
			render.DrawCursor(screen, 28, y, render.ColorRed)
			clr = render.ColorGold
		}

		// Show name with level and source tag
		tag := ""
		if entry.Source == "equipped" {
			tag = "[E]"
		}
		render.DrawText(screen, item.DisplayName(), 42, y, clr)
		if tag != "" {
			render.DrawText(screen, tag, 200, y, render.ColorMint)
		}

		// Show stat type
		statType := "ATK"
		if item.Type != entity.ItemWeapon {
			statType = "DEF"
		}
		render.DrawText(screen, statType+":"+intToStr(item.EffectiveStatBoost()), 232, y, render.ColorSky)
	}

	// Exit option
	exitY := 60 + min(len(t.reinforceItems)-startIdx, maxShow)*20
	exitClr := render.ColorWhite
	if t.reinforceIdx >= len(t.reinforceItems) {
		render.DrawCursor(screen, 28, exitY, render.ColorRed)
		exitClr = render.ColorGold
	}
	render.DrawText(screen, "Exit", 42, exitY, exitClr)

	// Selected item details — show cost and success rate
	if t.reinforceIdx < len(t.reinforceItems) {
		item := t.reinforceItems[t.reinforceIdx].Item
		detailY := 195
		// Divider
		for x := 28; x < 290; x += 2 {
			screen.Set(x, detailY-4, render.ColorDarkGray)
		}

		// Current → Next preview
		curStat := item.EffectiveStatBoost()
		// Peek at next level stat
		nextStat := int(float64(item.StatBoost) * reinforceMult(item.EnhanceLevel+1))
		statType := "ATK"
		if item.Type != entity.ItemWeapon {
			statType = "DEF"
		}
		render.DrawText(screen, statType+": "+intToStr(curStat)+" -> "+intToStr(nextStat), 32, detailY, render.ColorWhite)

		// Cost
		cost := item.ReinforceCost()
		costClr := render.ColorGold
		if t.player.Coins < cost {
			costClr = render.ColorRed
		}
		render.DrawText(screen, "Cost: "+intToStr(cost)+"G", 32, detailY+16, costClr)

		// Success rate
		pct := item.ReinforceSuccessPct()
		rateClr := render.ColorGreen
		if pct < 50 {
			rateClr = render.ColorGold
		}
		if pct < 25 {
			rateClr = render.ColorRed
		}
		render.DrawText(screen, "Rate: "+intToStr(pct)+"%", 180, detailY+16, rateClr)
	}

	render.DrawText(screen, "Z:Reinforce X:Exit", 88, 254, render.ColorGray)
}

// reinforceMult returns 1.05^level.
func reinforceMult(level int) float64 {
	m := 1.0
	for i := 0; i < level; i++ {
		m *= 1.05
	}
	return m
}

func (t *TownScreen) drawPause(screen *ebiten.Image) {
	// Pause/inventory overlay — spacious for 320x288
	render.DrawBox(screen, 20, 16, 280, 256, render.ColorBoxBG, render.ColorSky)

	info := entity.ClassTable[t.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(t.player.Level), 32, 26, render.ColorPink)

	s := t.player.Stats
	y := 46
	render.DrawText(screen, "HP: "+intToStr(s.HP)+"/"+intToStr(s.MaxHP), 32, y, render.ColorGreen)
	render.DrawBar(screen, 140, y+1, 120, 7, float64(s.HP)/float64(s.MaxHP), render.ColorGreen, render.ColorDarkGray)
	y += 16
	render.DrawText(screen, "MP: "+intToStr(s.MP)+"/"+intToStr(s.MaxMP), 32, y, render.ColorSky)
	render.DrawBar(screen, 140, y+1, 120, 7, float64(s.MP)/float64(s.MaxMP), render.ColorSky, render.ColorDarkGray)
	y += 18
	render.DrawText(screen, "ATK: "+intToStr(t.player.EffectiveATK()), 32, y, render.ColorPeach)
	render.DrawText(screen, "DEF: "+intToStr(t.player.EffectiveDEF()), 170, y, render.ColorSky)
	y += 14
	render.DrawText(screen, "SPD: "+intToStr(s.SPD), 32, y, render.ColorGold)
	if t.player.IsMaxLevel() {
		render.DrawText(screen, "XP: MAX", 170, y, render.ColorGold)
	} else {
		render.DrawText(screen, "XP: "+intToStr(t.player.XP)+"/"+intToStr(t.player.XPToNextLevel()), 170, y, render.ColorWhite)
	}
	y += 14
	render.DrawText(screen, "Coins: "+intToStr(t.player.Coins)+"G", 32, y, render.ColorGold)

	// Equipment (compact 6-slot display)
	y += 18
	render.DrawText(screen, "Equipment", 32, y, render.ColorLavender)
	y += 14
	t.drawPauseSlot(screen, "Wpn", t.player.Weapon, 32, y, render.ColorPeach)
	t.drawPauseSlot(screen, "Arm", t.player.Armor, 168, y, render.ColorSky)
	y += 12
	t.drawPauseSlot(screen, "Hlm", t.player.Helmet, 32, y, render.ColorSky)
	t.drawPauseSlot(screen, "Bts", t.player.Boots, 168, y, render.ColorMint)
	y += 12
	t.drawPauseSlot(screen, "Shd", t.player.Shield, 32, y, render.ColorSky)
	t.drawPauseSlot(screen, "Acc", t.player.Accessory, 168, y, render.ColorLavender)

	// Quests
	y += 20
	render.DrawText(screen, "Quests", 32, y, render.ColorMint)
	y += 16
	q := t.player.ActiveQuest()
	if q != nil {
		render.DrawText(screen, q.Name, 32, y, render.ColorWhite)
		y += 14
		render.DrawText(screen, intToStr(q.Progress)+"/"+intToStr(q.Required), 32, y, render.ColorPeach)
	} else {
		render.DrawText(screen, "No active quests", 32, y, render.ColorGray)
	}

	render.DrawText(screen, "B:Bag M:Map X:Close", 32, 258, render.ColorGray)
}

func (t *TownScreen) drawSaveSlots(screen *ebiten.Image) {
	render.DrawBox(screen, 40, 40, 240, 208, render.ColorBoxBG, render.ColorSky)
	render.DrawText(screen, "Save Game", 128, 50, render.ColorSky)

	classNames := []string{"Knight", "Mage", "Archer"}

	for i := 0; i < save.MaxSlots; i++ {
		y := 74 + i*36
		clr := render.ColorWhite
		if i == t.saveSlotSelected {
			render.DrawCursor(screen, 50, y, render.ColorSky)
			clr = render.ColorGold
		}

		slotLabel := "Slot " + intToStr(i+1) + ": "
		if t.saveSlots[i].Used {
			cn := "???"
			if t.saveSlots[i].Class >= 0 && t.saveSlots[i].Class < len(classNames) {
				cn = classNames[t.saveSlots[i].Class]
			}
			render.DrawText(screen, slotLabel+cn+" Lv."+intToStr(t.saveSlots[i].Level), 64, y, clr)
			render.DrawText(screen, t.saveSlots[i].Area, 64, y+14, render.ColorGray)
		} else {
			render.DrawText(screen, slotLabel+"--- Empty ---", 64, y, render.ColorGray)
		}
	}

	// Cancel option
	cancelY := 74 + save.MaxSlots*36
	cancelClr := render.ColorWhite
	if t.saveSlotSelected >= save.MaxSlots {
		render.DrawCursor(screen, 50, cancelY, render.ColorSky)
		cancelClr = render.ColorGold
	}
	render.DrawText(screen, "Cancel", 64, cancelY, cancelClr)

	render.DrawText(screen, "Z:Save  X:Cancel", 100, 232, render.ColorGray)
}

// drawDayNightTint draws a colored overlay for the current time of day.
func (t *TownScreen) drawDayNightTint(screen *ebiten.Image) {
	if t.player.DayNight == nil {
		return
	}
	tint := t.player.DayNight.TintColor()
	if tint.A == 0 {
		return
	}
	overlay := ebiten.NewImage(320, 288)
	overlay.Fill(tint)
	screen.DrawImage(overlay, nil)
}

// drawPauseSlot draws a compact "Label:ItemName" in the pause menu.
// Uses rarity color for the item name.
func (t *TownScreen) drawPauseSlot(screen *ebiten.Image, label string, slot *entity.Item, x, y int, _ color.Color) {
	if slot != nil {
		name := slot.DisplayName()
		if len(name) > 11 {
			name = name[:11]
		}
		nameClr := render.RarityColor(int(slot.Rarity))
		render.DrawText(screen, label+":"+name, x, y, nameClr)
	} else {
		render.DrawText(screen, label+":-", x, y, render.ColorDarkGray)
	}
}
