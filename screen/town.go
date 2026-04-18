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
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/asset"
	"cli_adventure/data"
	"cli_adventure/entity"
	netpkg "cli_adventure/net"
	"cli_adventure/render"
)

type townState int

const (
	townWalking townState = iota
	townTalking
	townShopping
	townPaused
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

	// Shop state
	shopItems    []entity.Item
	shopSelected int
	shopMode     int // 0=buy list, 1=confirm

	// Interaction tracking
	interactNPC *entity.NPC

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
	// THEORY — Town as safe haven:
	// Returning to town fully heals the player, like resting at a Pokemon Center.
	// This is a core RPG quality-of-life feature that encourages exploration:
	// players aren't punished for retreating to restock, making the game feel
	// generous rather than punishing.
	t.player.Stats.HP = t.player.Stats.MaxHP
	t.player.Stats.MP = t.player.Stats.MaxMP
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
	case townPaused:
		t.updatePaused()
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
		}
		return // no input while moving
	}

	// Check for exit (south edge)
	if t.tileY >= data.TownExitY {
		// In multiplayer, the host is the one who changes area. Remote
		// clients will automatically be pulled along by the host's
		// area_change broadcast (see net/client.go + wild/town syncSession).
		if t.session != nil && t.session.Role() == netpkg.RoleClient {
			// Clients don't initiate — roll back onto the exit tile.
			t.tileY = data.TownExitY - 1
			t.pixelY = float64(t.tileY * render.TileSize)
			return
		}
		if t.session != nil && t.session.Role() == netpkg.RoleHost {
			sx, sy := areaStart("forest")
			t.session.BroadcastAreaChange("forest", sx, sy)
			t.switcher.SwitchScreen(NewWildScreenMP(t.switcher, t.player, "forest", t.session))
			return
		}
		t.switcher.SwitchScreen(NewWildScreen(t.switcher, t.player, "forest"))
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
			}
			return
		}
	}

	// Check sign tile
	if fy >= 0 && fy < data.TownHeight && fx >= 0 && fx < data.TownWidth {
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
	t.dialogue = render.NewChoiceBox(
		"Welcome! Want to\nbrowse my wares?",
		[]string{"Buy", "No thanks"},
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

func (t *TownScreen) updateTalking() {
	if t.dialogue == nil {
		t.state = townWalking
		return
	}
	t.dialogue.Update()

	if t.dialogue.Finished {
		// Handle choice results
		if t.dialogue.ChoiceMade {
			if t.interactNPC != nil && t.interactNPC.Role == "merchant" {
				if t.dialogue.SelectedChoice == 0 { // "Buy"
					t.shopSelected = 0
					t.state = townShopping
					t.dialogue = nil
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
	// Navigate shop items
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
			// Auto-equip if better
			if item.Type == entity.ItemWeapon {
				if t.player.Weapon == nil || item.StatBoost > t.player.Weapon.StatBoost {
					t.player.Equip(item)
				}
			} else if item.Type == entity.ItemArmor {
				if t.player.Armor == nil || item.StatBoost > t.player.Armor.StatBoost {
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
	case townPaused:
		t.drawPause(screen)
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
		// Skip off-screen draws — screen is 160x144.
		if sx < -16 || sx > 160 || sy < -16 || sy > 144 {
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
	render.DrawText(screen, "Peaceful Village", 4, 2, render.ColorMint)

	// Coins display (top-right)
	coinText := intToStr(t.player.Coins) + "G"
	render.DrawText(screen, coinText, 160-render.TextWidth(coinText)-4, 2, render.ColorGold)

	// Bottom-right: key hints
	render.DrawText(screen, "Z:Talk X:Menu E:Equip", 22, 136, render.ColorDarkGray)

	// "Linked" indicator when a multiplayer session is live.
	if t.session != nil {
		peers := t.session.RemotePlayers("")
		render.DrawText(screen, "LINK "+intToStr(len(peers)+1), 2, 10, render.ColorLavender)
	}
}

func (t *TownScreen) drawShop(screen *ebiten.Image) {
	// Shop overlay
	render.DrawBox(screen, 10, 10, 140, 120, render.ColorBoxBG, render.ColorGold)
	render.DrawText(screen, "Shop", 62, 14, render.ColorGold)

	// Coins
	render.DrawText(screen, "Coins: "+intToStr(t.player.Coins)+"G", 16, 24, render.ColorWhite)

	// Item list
	for i, item := range t.shopItems {
		y := 36 + i*12
		clr := render.ColorWhite
		if i == t.shopSelected {
			render.DrawCursor(screen, 14, y, render.ColorGold)
			clr = render.ColorGold
		}

		typeName := "WPN"
		if item.Type == entity.ItemArmor {
			typeName = "ARM"
		} else if item.Type == entity.ItemConsumable {
			typeName = "USE"
		}

		render.DrawText(screen, item.Name, 24, y, clr)
		render.DrawText(screen, typeName, 100, y, render.ColorGray)
		render.DrawText(screen, intToStr(item.Price)+"G", 122, y, render.ColorPeach)
	}

	// Exit option
	exitY := 36 + len(t.shopItems)*12
	exitClr := render.ColorWhite
	if t.shopSelected >= len(t.shopItems) {
		render.DrawCursor(screen, 14, exitY, render.ColorGold)
		exitClr = render.ColorGold
	}
	render.DrawText(screen, "Exit", 24, exitY, exitClr)

	// Selected item stats
	if t.shopSelected < len(t.shopItems) {
		item := t.shopItems[t.shopSelected]
		sy := 108
		statLabel := "ATK+"
		if item.Type == entity.ItemArmor {
			statLabel = "DEF+"
		} else if item.Type == entity.ItemConsumable {
			statLabel = "HP +"
		}
		render.DrawText(screen, statLabel+intToStr(item.StatBoost), 16, sy, render.ColorSky)
	}

	render.DrawText(screen, "Z:Buy  X:Exit", 32, 120, render.ColorGray)
}

func (t *TownScreen) drawPause(screen *ebiten.Image) {
	// Pause/inventory overlay
	render.DrawBox(screen, 8, 8, 144, 128, render.ColorBoxBG, render.ColorSky)

	info := entity.ClassTable[t.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(t.player.Level), 14, 12, render.ColorPink)

	s := t.player.Stats
	y := 24
	render.DrawText(screen, "HP: "+intToStr(s.HP)+"/"+intToStr(s.MaxHP), 14, y, render.ColorGreen)
	render.DrawBar(screen, 80, y+1, 60, 5, float64(s.HP)/float64(s.MaxHP), render.ColorGreen, render.ColorDarkGray)
	y += 10
	render.DrawText(screen, "MP: "+intToStr(s.MP)+"/"+intToStr(s.MaxMP), 14, y, render.ColorSky)
	render.DrawBar(screen, 80, y+1, 60, 5, float64(s.MP)/float64(s.MaxMP), render.ColorSky, render.ColorDarkGray)
	y += 12
	render.DrawText(screen, "ATK: "+intToStr(t.player.EffectiveATK()), 14, y, render.ColorPeach)
	render.DrawText(screen, "DEF: "+intToStr(t.player.EffectiveDEF()), 80, y, render.ColorSky)
	y += 10
	render.DrawText(screen, "SPD: "+intToStr(s.SPD), 14, y, render.ColorGold)
	render.DrawText(screen, "XP: "+intToStr(t.player.XP)+"/"+intToStr(t.player.XPToNextLevel()), 80, y, render.ColorWhite)
	y += 10
	render.DrawText(screen, "Coins: "+intToStr(t.player.Coins)+"G", 14, y, render.ColorGold)

	// Equipment
	y += 14
	render.DrawText(screen, "Equipment", 14, y, render.ColorLavender)
	y += 10
	wpn := "None"
	if t.player.Weapon != nil {
		wpn = t.player.Weapon.Name
	}
	render.DrawText(screen, "Weapon: "+wpn, 14, y, render.ColorWhite)
	y += 10
	arm := "None"
	if t.player.Armor != nil {
		arm = t.player.Armor.Name
	}
	render.DrawText(screen, "Armor:  "+arm, 14, y, render.ColorWhite)

	// Quests
	y += 14
	render.DrawText(screen, "Quests", 14, y, render.ColorMint)
	y += 10
	q := t.player.ActiveQuest()
	if q != nil {
		render.DrawText(screen, q.Name, 14, y, render.ColorWhite)
		y += 10
		render.DrawText(screen, intToStr(q.Progress)+"/"+intToStr(q.Required), 14, y, render.ColorPeach)
	} else {
		render.DrawText(screen, "No active quests", 14, y, render.ColorGray)
	}

	render.DrawText(screen, "B:Bag M:Map X:Close", 14, 128, render.ColorGray)
}
