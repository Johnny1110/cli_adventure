// screen/combat.go — Full turn-based combat screen.
//
// THEORY — Combat screen as a rendering layer over the engine:
// The combat engine (combat/engine.go) is pure logic — it computes damage,
// turn order, and victory conditions. This screen takes those results and
// turns them into visual events: sprite shakes, HP bar animations, damage
// numbers, and text messages. The screen has its own mini state machine
// for sequencing animations (e.g., "show attack text → shake sprite →
// show damage number → update HP bar → pause → next turn").
//
// THEORY — Pokemon-style layout at 320x288:
// The doubled resolution gives us room to breathe. Classic layout:
//   Top area:    Monster sprite + name + HP bar (spacious)
//   Middle:      Divider line
//   Mid-lower:   Player sprite + stats (name, HP bar, MP bar)
//   Bottom:      Action menu OR battle text
//
// The action menu shows 4 options in a 2x2 grid: Attack, Magic, Defend, Flee.
// During enemy turns or result display, the menu is replaced by a text box
// showing what happened ("Slime attacks! 5 damage!").
//
// THEORY — Animation timing:
// We use tick-based timing (at 60 TPS). For example:
//   - Attack text shows for 30 ticks (0.5s)
//   - Sprite shake lasts 12 ticks (0.2s)
//   - Damage number floats up over 20 ticks
//   - HP bar drains smoothly over 15 ticks
// These timings create a snappy but readable battle flow.
package screen

import (
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/asset"
	"cli_adventure/combat"
	"cli_adventure/entity"
	netpkg "cli_adventure/net"
	"cli_adventure/render"
)

// combatUIState tracks what the combat screen is currently showing.
type combatUIState int

const (
	cuiIntro        combatUIState = iota // "A wild X appeared!" text
	cuiPlayerMenu                        // action menu visible, awaiting input
	cuiSkillMenu                         // skill selection sub-menu
	cuiItemMenu                          // item selection sub-menu
	cuiPlayerAnim                        // animating player's action
	cuiEnemyAnim                         // animating enemy's action
	cuiMessage                           // showing a result message
	cuiVictory                           // victory sequence
	cuiDefeat                            // defeat sequence
	cuiLevelUp                           // level-up notification
)

// CombatScreen handles the full turn-based combat UI.
type CombatScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
	monster  *entity.Monster
	areaKey  string
	engine   *combat.Engine

	// UI state
	uiState   combatUIState
	menuIdx   int // 0=Attack, 1=Magic, 2=Defend, 3=Flee
	tick      int
	animTick  int
	msgText   string
	msgQueue  []string // queue of messages to show sequentially

	// Animation
	monsterShakeX float64
	playerShakeX  float64
	damageNum     int
	damageY       float64
	showDamage    bool
	healNum       int
	showHeal      bool

	// HP bar smooth animation
	displayMonHP  float64 // smoothly animated display value
	displayPlrHP  float64

	// Sprites
	monsterSprites map[string]*render.SpriteSheet
	monsterAnim    *render.Animation
	charSprites    map[int]*render.SpriteSheet

	// Victory state
	xpGained    int
	coinsGained int
	didLevelUp  bool
	prevLevel   int

	// Return position — where the player was standing before combat
	returnX, returnY int

	// Skill menu
	skillIdx int // cursor in skill sub-menu

	// Item menu
	itemIdx int // cursor in item sub-menu

	// Screen shake
	shakeIntensity float64
	shakeX, shakeY float64
	shakeBuf       *ebiten.Image // offscreen buffer for shake effect

	// Multiplayer hooks (nil in single-player).
	// When a session is attached AND the local player is the host, we
	// mirror authoritative state changes (monster HP, end result) to
	// every connected client. The host still drives its own combat
	// engine — clients are spectators submitting actions via MsgCombatAction,
	// which the host harvests between turns.
	session *netpkg.Session
	announcedStart bool
	lastBroadcastMonHP int
}

// NewCombatScreenCoop creates a combat screen wired to a live session.
// It's a thin wrapper so single-player tests never touch the net package.
func NewCombatScreenCoop(switcher ScreenSwitcher, player *entity.Player, monster *entity.Monster, areaKey string, retX, retY int, sess *netpkg.Session) *CombatScreen {
	c := NewCombatScreenAt(switcher, player, monster, areaKey, retX, retY)
	c.session = sess
	c.lastBroadcastMonHP = monster.HP
	return c
}

func NewCombatScreen(switcher ScreenSwitcher, player *entity.Player, monster *entity.Monster, areaKey string) *CombatScreen {
	return NewCombatScreenAt(switcher, player, monster, areaKey, -1, -1)
}

// NewCombatScreenAt creates a combat screen that remembers where the player was.
func NewCombatScreenAt(switcher ScreenSwitcher, player *entity.Player, monster *entity.Monster, areaKey string, retX, retY int) *CombatScreen {
	player.ResetCombatBuffs() // clear temporary buffs from previous combat
	player.TickInnBuff()     // decrement inn buff counter (persists across combats)
	eng := combat.NewEngine(player, monster)
	return &CombatScreen{
		switcher:       switcher,
		player:         player,
		monster:        monster,
		areaKey:        areaKey,
		engine:         eng,
		returnX:        retX,
		returnY:        retY,
		uiState:        cuiIntro,
		monsterSprites: asset.GenerateMonsterSprites(),
		monsterAnim:    render.NewAnimation([]int{0, 1}, 20),
		charSprites:    asset.GenerateCharSprites(),
		displayMonHP:   float64(monster.HP),
		displayPlrHP:   float64(player.Stats.HP),
	}
}

func (c *CombatScreen) OnEnter() {}
func (c *CombatScreen) OnExit()  {}

func (c *CombatScreen) Update() error {
	c.tick++
	c.monsterAnim.Update()

	// Update screen shake
	if c.shakeIntensity > 0.5 {
		c.shakeX = (rand.Float64() - 0.5) * c.shakeIntensity
		c.shakeY = (rand.Float64() - 0.5) * c.shakeIntensity
		c.shakeIntensity *= 0.85 // decay
	} else {
		c.shakeX = 0
		c.shakeY = 0
		c.shakeIntensity = 0
	}

	// Smooth HP bar animation
	c.animateHPBars()

	// Multiplayer mirroring: announce start once and push HP diffs.
	c.syncCombatToSession()

	switch c.uiState {
	case cuiIntro:
		c.updateIntro()
	case cuiPlayerMenu:
		c.updatePlayerMenu()
	case cuiSkillMenu:
		c.updateSkillMenu()
	case cuiItemMenu:
		c.updateItemMenu()
	case cuiPlayerAnim:
		c.updatePlayerAnim()
	case cuiEnemyAnim:
		c.updateEnemyAnim()
	case cuiMessage:
		c.updateMessage()
	case cuiVictory:
		c.updateVictory()
	case cuiDefeat:
		c.updateDefeat()
	case cuiLevelUp:
		c.updateLevelUp()
	}

	return nil
}

func (c *CombatScreen) animateHPBars() {
	// Smoothly interpolate displayed HP toward actual HP
	targetMon := float64(c.monster.HP)
	if c.displayMonHP > targetMon {
		c.displayMonHP -= 0.5
		if c.displayMonHP < targetMon {
			c.displayMonHP = targetMon
		}
	}
	targetPlr := float64(c.player.Stats.HP)
	if c.displayPlrHP > targetPlr {
		c.displayPlrHP -= 0.5
		if c.displayPlrHP < targetPlr {
			c.displayPlrHP = targetPlr
		}
	} else if c.displayPlrHP < targetPlr {
		c.displayPlrHP += 0.5
		if c.displayPlrHP > targetPlr {
			c.displayPlrHP = targetPlr
		}
	}
}

func (c *CombatScreen) updateIntro() {
	if c.tick == 1 {
		if c.monster.IsBoss {
			c.msgText = "The Dragon attacks!"
		} else if c.monster.IsGolden {
			c.msgText = "A rare " + c.monster.Name + "\nappeared! It shines!"
		} else {
			c.msgText = "A wild " + c.monster.Name + "\nappeared!"
		}
	}
	if c.tick >= 60 || inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		c.engine.Start()
		if c.engine.Phase == combat.PhasePlayerTurn {
			c.uiState = cuiPlayerMenu
		} else {
			c.doEnemyTurn()
		}
	}
}

func (c *CombatScreen) updatePlayerMenu() {
	// THEORY — 3×2 menu grid with 5 options:
	// The classic 2×2 grid (Attack/Skill/Defend/Flee) had no room for "Item".
	// Rather than cramming it into a tiny space, we expand to 3 rows:
	//   Row 0: Attack | Skill
	//   Row 1: Item   | Defend
	//   Row 2: Flee   |
	// Navigation wraps naturally: left/right toggles column (unless at
	// index 4 which has no right neighbor), up/down moves by 2.
	// Index 4 (Flee) sits alone in the bottom-left — a common RPG pattern
	// that subtly de-emphasizes fleeing by putting it last and isolated.

	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		if c.menuIdx%2 == 1 {
			c.menuIdx--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		if c.menuIdx%2 == 0 && c.menuIdx+1 <= 4 {
			// Don't move right from Flee (index 4) — no neighbor
			if c.menuIdx < 4 {
				c.menuIdx++
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		if c.menuIdx >= 2 {
			c.menuIdx -= 2
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if c.menuIdx+2 <= 4 {
			c.menuIdx += 2
		}
	}

	// Confirm action
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		switch c.menuIdx {
		case 1: // Skill
			learned := c.player.LearnedSkills()
			if len(learned) > 0 {
				c.skillIdx = 0
				c.uiState = cuiSkillMenu
				return
			}
			// No skills learned — fallback to old class magic
			c.doPlayerAction(combat.ActionMagic)
		case 2: // Item
			consumables := c.player.Consumables()
			if len(consumables) > 0 {
				c.itemIdx = 0
				c.uiState = cuiItemMenu
				return
			}
			// No items — show a message
			c.msgText = "No items!"
			c.uiState = cuiMessage
		default:
			// 0=Attack, 3=Defend, 4=Flee
			actions := map[int]combat.Action{
				0: combat.ActionAttack,
				3: combat.ActionDefend,
				4: combat.ActionFlee,
			}
			c.doPlayerAction(actions[c.menuIdx])
		}
	}
}

// updateSkillMenu handles input in the skill selection sub-menu.
func (c *CombatScreen) updateSkillMenu() {
	learned := c.player.LearnedSkills()

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		c.skillIdx--
		if c.skillIdx < 0 {
			c.skillIdx = len(learned) - 1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		c.skillIdx++
		if c.skillIdx >= len(learned) {
			c.skillIdx = 0
		}
	}

	// Confirm skill
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		c.engine.SelectedSkillIdx = c.skillIdx
		c.doPlayerAction(combat.ActionSkill)
	}

	// Cancel — back to main menu
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.uiState = cuiPlayerMenu
	}
}

// updateItemMenu handles input in the item selection sub-menu.
//
// THEORY — Item sub-menu mirrors skill sub-menu:
// We reuse the same UX pattern as the skill menu: a scrollable vertical
// list with cursor, Z to confirm, X to cancel. This consistency means the
// player only needs to learn one sub-menu pattern. Each entry shows the
// item name, what it restores (HP/MP), and the restore amount — giving
// the player enough info to make a tactical choice without opening a
// separate info screen.
func (c *CombatScreen) updateItemMenu() {
	consumables := c.player.Consumables()
	if len(consumables) == 0 {
		// Items ran out (shouldn't happen, but safety)
		c.uiState = cuiPlayerMenu
		return
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		c.itemIdx--
		if c.itemIdx < 0 {
			c.itemIdx = len(consumables) - 1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		c.itemIdx++
		if c.itemIdx >= len(consumables) {
			c.itemIdx = 0
		}
	}

	// Confirm item
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		selected := consumables[c.itemIdx]
		c.engine.SelectedItemIdx = selected.Idx // pass the real inventory index to the engine
		c.doPlayerAction(combat.ActionItem)
	}

	// Cancel — back to main menu
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.uiState = cuiPlayerMenu
	}
}

func (c *CombatScreen) doPlayerAction(action combat.Action) {
	result := c.engine.PlayerAction(action)
	c.msgText = result.Message
	c.animTick = 0
	c.showDamage = false
	c.showHeal = false

	// Prepend effect tick messages (DoT damage, buff expiry) to the message queue
	effectMsgs := c.engine.EffectMessages

	switch c.engine.Phase {
	case combat.PhaseVictory:
		// Player killed the monster
		if result.Damage > 0 {
			c.damageNum = result.Damage
			c.showDamage = true
			c.monsterShakeX = 4
		}
		c.msgQueue = append(effectMsgs, result.Message)
		if result.IsCrit {
			c.msgQueue = append(c.msgQueue, "Critical hit!")
		}
		c.uiState = cuiPlayerAnim
	case combat.PhaseFlee:
		c.uiState = cuiMessage
	case combat.PhaseFleeFaild:
		c.msgQueue = append(effectMsgs, result.Message)
		c.uiState = cuiMessage
	default:
		// Normal action — effect messages first, then the action result
		c.msgQueue = append(effectMsgs, result.Message)
		if result.IsCrit {
			c.msgQueue = append(c.msgQueue, "Critical hit!")
			c.shakeIntensity = 6 // big screen shake on crit!
		}
		if result.Damage > 0 {
			c.damageNum = result.Damage
			c.showDamage = true
			c.monsterShakeX = 4
		}
		if result.Healed > 0 {
			c.healNum = result.Healed
			c.showHeal = true
		}
		c.uiState = cuiPlayerAnim
	}
}

func (c *CombatScreen) updatePlayerAnim() {
	c.animTick++

	// Shake animation
	if c.monsterShakeX > 0 {
		c.monsterShakeX *= -0.8
		if absF(c.monsterShakeX) < 0.5 {
			c.monsterShakeX = 0
		}
	}

	// Damage number float up
	if c.showDamage {
		c.damageY += 0.3
	}

	// After animation time, transition
	if c.animTick >= 40 {
		c.showDamage = false
		c.showHeal = false
		c.damageY = 0
		c.monsterShakeX = 0

		// Check if monster died
		if c.engine.Phase == combat.PhaseVictory {
			c.startVictory()
			return
		}

		// Move to enemy turn
		c.engine.AdvancePhase()
		c.doEnemyTurn()
	}
}

func (c *CombatScreen) doEnemyTurn() {
	result := c.engine.EnemyAction()
	c.msgText = result.Message
	c.animTick = 0
	c.showDamage = false
	c.showHeal = false

	// Check if enemy was killed by DoT before acting
	if c.engine.Phase == combat.PhaseVictory {
		c.msgQueue = append(c.engine.EffectMessages, result.Message)
		c.startVictory()
		return
	}

	if result.Damage > 0 {
		c.damageNum = result.Damage
		c.showDamage = true
		c.playerShakeX = 4
		// Screen shake scales with damage: enemy hits shake the screen,
		// boss specials (ActionMagic from enemy) shake harder
		if result.Action == combat.ActionMagic {
			c.shakeIntensity = 8 // boss fire breath — BIG shake
		} else if result.Damage >= 10 {
			c.shakeIntensity = 4 // heavy hit
		} else {
			c.shakeIntensity = 2 // light hit
		}
	}

	// Effect messages (DoT on enemy, buff expiry) shown before the attack result
	c.msgQueue = append(c.engine.EffectMessages, result.Message)
	c.uiState = cuiEnemyAnim
}

func (c *CombatScreen) updateEnemyAnim() {
	c.animTick++

	// Player shake
	if c.playerShakeX > 0 {
		c.playerShakeX *= -0.8
		if absF(c.playerShakeX) < 0.5 {
			c.playerShakeX = 0
		}
	}

	if c.showDamage {
		c.damageY += 0.3
	}

	if c.animTick >= 40 {
		c.showDamage = false
		c.showHeal = false
		c.damageY = 0
		c.playerShakeX = 0

		if c.engine.Phase == combat.PhaseDefeat {
			c.uiState = cuiDefeat
			c.animTick = 0
			c.msgText = "You were defeated..."
			return
		}

		// Back to player turn
		c.engine.AdvancePhase()
		c.uiState = cuiPlayerMenu
	}
}

func (c *CombatScreen) updateMessage() {
	// Show message, advance on Z
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || c.tick%90 == 0 {
		if c.engine.Phase == combat.PhaseFlee {
			c.returnToWild()
			return
		}
		if c.engine.Phase == combat.PhaseFleeFaild {
			c.engine.AdvancePhase()
			c.doEnemyTurn()
			return
		}
		c.uiState = cuiPlayerMenu
	}
}

func (c *CombatScreen) startVictory() {
	c.xpGained = c.monster.XPReward
	c.coinsGained = c.monster.CoinReward
	c.prevLevel = c.player.Level

	// Update quest progress (golden monsters count for their base type)
	q := c.player.ActiveQuest()
	if q != nil && !q.Done {
		monName := c.monster.Name
		if c.monster.BaseName != "" {
			monName = c.monster.BaseName
		}
		if q.Target == monName {
			q.Progress++
		}
	}

	c.uiState = cuiVictory
	c.animTick = 0
	c.msgText = "Victory!"
}

func (c *CombatScreen) updateVictory() {
	c.animTick++

	if c.animTick == 30 {
		// Grant rewards
		c.didLevelUp = c.player.GainXP(c.xpGained)
		c.player.Coins += c.coinsGained
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if c.animTick >= 30 {
			if c.didLevelUp {
				c.uiState = cuiLevelUp
				c.animTick = 0
				return
			}
			c.returnAfterVictory()
		}
	}
}

func (c *CombatScreen) updateLevelUp() {
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		c.returnAfterVictory()
	}
}

// returnAfterVictory handles post-combat transitions.
//
// THEORY — Post-boss continuation:
// In the original design, defeating the Dragon ended the game. Now bosses
// are milestones, not endings. Defeating a boss sets a flag in the player's
// BossDefeated map, which unlocks new content (e.g., defeating the Dragon
// opens the western desert chain). The game continues — the player returns
// to the wild area where they fought the boss.
//
// The ending screen is now only shown when ALL bosses are defeated (or
// could be triggered by a final superboss in a future phase). For now,
// each boss victory returns the player to the world with a congratulatory
// dialogue shown via the wild screen's boss dialogue system.
func (c *CombatScreen) returnAfterVictory() {
	// Broadcast the win to every connected peer.
	if c.session != nil {
		c.session.EndCombat(true, c.xpGained, c.coinsGained)
	}

	if c.monster.IsBoss {
		// Record boss defeat
		if c.player.BossDefeated == nil {
			c.player.BossDefeated = map[string]bool{}
		}
		bossKey := bossKeyFromArea(c.areaKey)
		c.player.BossDefeated[bossKey] = true
	}

	// Return to the wild area at the player's pre-combat position.
	if c.session != nil {
		c.switcher.SwitchScreen(NewWildScreenMPAt(c.switcher, c.player, c.areaKey, c.returnX, c.returnY, c.session))
		return
	}
	c.switcher.SwitchScreen(NewWildScreenAt(c.switcher, c.player, c.areaKey, c.returnX, c.returnY))
}

// bossKeyFromArea maps an area key to its boss name for the BossDefeated map.
func bossKeyFromArea(areaKey string) string {
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
		return areaKey
	}
}

// syncCombatToSession is the per-tick multiplayer mirror. It's a no-op
// in single-player (session == nil). On the host side it:
//
//   - Opens the shared combat on the first tick (StartCombat).
//   - Broadcasts monster HP whenever it changes.
//   - Broadcasts short log text for big events (hit, heal, miss) so
//     clients have something to read.
//
// This file keeps combat authority inside combat.Engine. All we're doing
// here is mirroring visible state.
func (c *CombatScreen) syncCombatToSession() {
	if c.session == nil {
		return
	}
	if !c.announcedStart {
		c.session.StartCombat(c.monster.Name, c.monster.Name, c.monster.HP, c.monster.MaxHP)
		c.announcedStart = true
	}
	if c.monster.HP != c.lastBroadcastMonHP {
		c.session.SetMonsterHP(c.monster.HP)
		c.lastBroadcastMonHP = c.monster.HP
	}
}

// returnToWild creates a WildScreen at the player's pre-combat position.
func (c *CombatScreen) returnToWild() {
	if c.session != nil {
		c.switcher.SwitchScreen(NewWildScreenMPAt(c.switcher, c.player, c.areaKey, c.returnX, c.returnY, c.session))
		return
	}
	c.switcher.SwitchScreen(NewWildScreenAt(c.switcher, c.player, c.areaKey, c.returnX, c.returnY))
}

func (c *CombatScreen) updateDefeat() {
	c.animTick++
	// On Z, return to menu (restart)
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if c.animTick >= 30 {
			// Announce the loss to every peer before tearing down.
			if c.session != nil {
				c.session.EndCombat(false, 0, 0)
			}
			// Heal player and send back to town (mercy mechanic)
			c.player.Stats.HP = c.player.Stats.MaxHP
			c.player.Stats.MP = c.player.Stats.MaxMP
			// Lose some coins as penalty
			c.player.Coins = c.player.Coins * 3 / 4
			if c.session != nil {
				c.switcher.SwitchScreen(NewTownScreenMP(c.switcher, c.player, c.session))
			} else {
				c.switcher.SwitchScreen(NewTownScreen(c.switcher, c.player))
			}
		}
	}
}

// ---- Draw ----

func (c *CombatScreen) Draw(screen *ebiten.Image) {
	// THEORY — Screen shake via offscreen buffer:
	// We render the entire combat scene to an offscreen image at native
	// resolution (320x288), then draw that image onto the real screen
	// with a random offset (shakeX, shakeY). This means EVERY pixel
	// on screen shifts together — a clean, juicy shake effect like
	// Vlambeer-style "game feel". The background fill on the real
	// screen ensures no garbage pixels show at the edges during shake.
	buf := c.getShakeBuffer()
	buf.Fill(render.ColorBG)

	c.drawMonsterArea(buf)
	c.drawDivider(buf)
	c.drawPlayerArea(buf)
	c.drawBottomUI(buf)

	// Floating damage/heal numbers
	if c.showDamage {
		y := 70.0 - c.damageY
		if c.uiState == cuiEnemyAnim {
			// Damage on player
			y = 150.0 - c.damageY
			render.DrawText(buf, intToStr(c.damageNum), 240, int(y), render.ColorRed)
		} else {
			// Damage on monster
			render.DrawText(buf, intToStr(c.damageNum), 180, int(y), render.ColorGold)
		}
	}
	if c.showHeal {
		render.DrawText(buf, "+"+intToStr(c.healNum), 200, 140, render.ColorGreen)
	}

	// Blit the buffer onto screen with shake offset
	screen.Fill(render.ColorBG)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(c.shakeX, c.shakeY)
	screen.DrawImage(buf, op)
}

func (c *CombatScreen) drawMonsterArea(screen *ebiten.Image) {
	// Monster name (golden monsters get gold text)
	nameClr := render.ColorPink
	if c.monster.IsGolden {
		nameClr = render.ColorGold
	}
	render.DrawText(screen, c.monster.Name, 8, 4, nameClr)

	// Monster HP bar — wider bar with more room
	monFrac := float64(c.displayMonHP) / float64(c.monster.MaxHP)
	hpColor := hpBarColor(monFrac)
	render.DrawText(screen, "HP", 8, 18, render.ColorWhite)
	render.DrawBar(screen, 30, 19, 120, 6, monFrac, hpColor, render.ColorDarkGray)

	// Active effects on monster (debuffs from player)
	c.drawEffectIcons(screen, c.engine.EnemyEffects, 8, 30)

	// Monster sprite — positioned in the right portion of the top area
	sprite, ok := c.monsterSprites[c.monster.SpriteID]
	if ok {
		frame := c.monsterAnim.CurrentFrame()
		sx := 220.0 + c.monsterShakeX
		sy := 8.0
		if c.monster.IsBoss {
			sx = 200.0 + c.monsterShakeX
		} else {
			sy = 20.0
		}

		if c.monster.IsGolden {
			// Golden tint: draw with a gold color scale (shimmer effect)
			shimmer := 0.9 + 0.1*absF(float64((c.tick/4)%10-5))/5.0
			sprite.DrawFrameTinted(screen, frame, sx, sy, shimmer, 0.85, 0.3)
		} else {
			sprite.DrawFrame(screen, frame, sx, sy)
		}
	}
}

func (c *CombatScreen) drawDivider(screen *ebiten.Image) {
	for x := 8; x < 312; x += 2 {
		screen.Set(x, 110, render.ColorDarkGray)
	}
}

func (c *CombatScreen) drawPlayerArea(screen *ebiten.Image) {
	// Player sprite (bottom-left, more room now)
	sheet := c.charSprites[int(c.player.Class)]
	sx := 16.0 + c.playerShakeX
	sheet.DrawFrame(screen, 0, sx, 120.0)

	// Player info — spread out with generous spacing
	info := entity.ClassTable[c.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(c.player.Level), 60, 118, render.ColorSky)

	// HP bar — wider with more spacing
	plrHPFrac := float64(c.displayPlrHP) / float64(c.player.Stats.MaxHP)
	hpColor := hpBarColor(plrHPFrac)
	render.DrawText(screen, "HP", 60, 134, render.ColorWhite)
	render.DrawBar(screen, 84, 135, 130, 6, plrHPFrac, hpColor, render.ColorDarkGray)
	render.DrawText(screen, intToStr(int(c.displayPlrHP))+"/"+intToStr(c.player.Stats.MaxHP), 220, 134, render.ColorWhite)

	// MP bar — wider with more spacing
	mpFrac := 0.0
	if c.player.Stats.MaxMP > 0 {
		mpFrac = float64(c.player.Stats.MP) / float64(c.player.Stats.MaxMP)
	}
	render.DrawText(screen, "MP", 60, 150, render.ColorWhite)
	render.DrawBar(screen, 84, 151, 130, 6, mpFrac, render.ColorSky, render.ColorDarkGray)
	render.DrawText(screen, intToStr(c.player.Stats.MP)+"/"+intToStr(c.player.Stats.MaxMP), 220, 150, render.ColorWhite)

	// Active effects on player (buffs + enemy debuffs)
	c.drawEffectIcons(screen, c.engine.PlayerEffects, 60, 164)
}

// drawEffectIcons renders compact status effect indicators.
//
// THEORY — Compact effect display:
// Classic Game Boy RPGs showed status with tiny icons or 3-letter abbreviations
// next to the character name: "PSN", "SLP", "PAR". We do the same: each active
// effect gets a 3-4 char label color-coded by category (green=buff, red=debuff,
// purple=DoT, orange=status). The turn count is shown as a superscript number.
// This gives the player at-a-glance tactical awareness without cluttering the UI.
func (c *CombatScreen) drawEffectIcons(screen *ebiten.Image, effects []combat.StatusEffect, startX, y int) {
	icons := combat.EffectSummary(effects)
	x := startX
	for _, icon := range icons {
		// Pick color based on category
		var clr color.Color
		switch icon.Color {
		case combat.IconBuff:
			clr = render.ColorGreen
		case combat.IconDebuff:
			clr = render.ColorRed
		case combat.IconDoT:
			clr = render.ColorLavender
		case combat.IconStatus:
			clr = render.ColorGold
		default:
			clr = render.ColorGray
		}

		// Draw abbreviated label + turn count
		label := icon.Short
		if icon.Turns > 0 {
			label += intToStr(icon.Turns)
		}
		render.DrawText(screen, label, x, y, clr)
		x += len(label)*8 + 4 // approximate spacing

		// Don't overflow the screen
		if x > 280 {
			break
		}
	}
}

func (c *CombatScreen) drawBottomUI(screen *ebiten.Image) {
	boxY := 180
	boxH := 100

	switch c.uiState {
	case cuiPlayerMenu:
		c.drawActionMenu(screen, boxY, boxH)
	case cuiSkillMenu:
		c.drawSkillSelect(screen, boxY, boxH)
	case cuiItemMenu:
		c.drawItemSelect(screen, boxY, boxH)
	case cuiVictory:
		c.drawVictoryBox(screen, boxY, boxH)
	case cuiDefeat:
		c.drawDefeatBox(screen, boxY, boxH)
	case cuiLevelUp:
		c.drawLevelUpBox(screen, boxY, boxH)
	default:
		// Message box
		render.DrawBox(screen, 8, boxY, 304, boxH, render.ColorBoxBG, render.ColorBoxBorder)
		render.DrawText(screen, c.msgText, 20, boxY+12, render.ColorWhite)

		// Show "Z" prompt if waiting for input
		if c.uiState == cuiMessage || c.uiState == cuiIntro {
			if (c.tick/20)%2 == 0 {
				render.DrawText(screen, "Z", 292, boxY+boxH-16, render.ColorGold)
			}
		}
	}
}

func (c *CombatScreen) drawActionMenu(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 8, boxY, 304, boxH, render.ColorBoxBG, render.ColorBoxBorder)

	// Skill slot label: shows "Skill" if player has learned skills, else class default
	skillName := "Magic"
	if len(c.player.LearnedSkills()) > 0 {
		skillName = "Skill"
	} else {
		switch c.player.Class {
		case entity.ClassKnight:
			skillName = "Bash"
		case entity.ClassMage:
			skillName = "Fire"
		case entity.ClassArcher:
			skillName = "Snipe"
		}
	}

	// 3×2 grid layout:
	//   0: Attack   1: Skill
	//   2: Item     3: Defend
	//   4: Flee
	labels := []string{"Attack", skillName, "Item", "Defend", "Flee"}
	positions := [][2]int{
		{32, boxY + 10},
		{180, boxY + 10},
		{32, boxY + 34},
		{180, boxY + 34},
		{32, boxY + 58},
	}

	for i, label := range labels {
		clr := color.Color(render.ColorWhite)
		if i == c.menuIdx {
			clr = render.ColorGold
			render.DrawCursor(screen, positions[i][0]-10, positions[i][1], render.ColorGold)
		}
		render.DrawText(screen, label, positions[i][0], positions[i][1], clr)
	}

	// Show consumable count next to "Item" as a convenience hint
	nConsumables := len(c.player.Consumables())
	render.DrawText(screen, "("+intToStr(nConsumables)+")", 80, boxY+34, render.ColorGray)

	// Subtle hint
	render.DrawText(screen, "Z:Select", 120, boxY+boxH-12, render.ColorGray)
}

func (c *CombatScreen) drawSkillSelect(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 8, boxY, 304, boxH, render.ColorBoxBG, render.ColorLavender)
	render.DrawText(screen, "Skills", 12, boxY+4, render.ColorLavender)

	learned := c.player.LearnedSkills()
	maxShow := 4
	startIdx := 0
	if c.skillIdx >= maxShow {
		startIdx = c.skillIdx - maxShow + 1
	}

	for i := startIdx; i < len(learned) && i < startIdx+maxShow; i++ {
		def := learned[i]
		lvl := c.player.SkillLevel(def.ID)
		y := boxY + 22 + (i-startIdx)*18

		clr := color.Color(render.ColorWhite)
		if i == c.skillIdx {
			clr = render.ColorGold
			render.DrawCursor(screen, 14, y, render.ColorGold)
		}

		render.DrawText(screen, def.Name, 28, y, clr)
		render.DrawText(screen, "Lv"+intToStr(lvl), 180, y, render.ColorMint)
		mpCost := def.MPCost[lvl-1]
		mpClr := color.Color(render.ColorSky)
		if c.player.Stats.MP < mpCost {
			mpClr = render.ColorRed // can't afford
		}
		render.DrawText(screen, intToStr(mpCost)+"MP", 240, y, mpClr)
	}

	render.DrawText(screen, "Z:Use X:Back", 100, boxY+boxH-16, render.ColorGray)
}

// drawItemSelect renders the item selection sub-menu.
//
// THEORY — Showing restore type + amount:
// Unlike the skill menu (which shows MP cost), the item menu shows what
// each item restores: "HP+15" or "MP+10". This lets the player make an
// informed choice. Color coding reinforces the distinction: green for HP
// items, blue/sky for MP items — matching the HP and MP bar colors the
// player already associates with those stats.
func (c *CombatScreen) drawItemSelect(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 8, boxY, 304, boxH, render.ColorBoxBG, render.ColorMint)
	render.DrawText(screen, "Items", 12, boxY+4, render.ColorMint)

	consumables := c.player.Consumables()
	maxShow := 4
	startIdx := 0
	if c.itemIdx >= maxShow {
		startIdx = c.itemIdx - maxShow + 1
	}

	for i := startIdx; i < len(consumables) && i < startIdx+maxShow; i++ {
		item := consumables[i].Item
		y := boxY + 22 + (i-startIdx)*18

		clr := color.Color(render.ColorWhite)
		if i == c.itemIdx {
			clr = render.ColorGold
			render.DrawCursor(screen, 14, y, render.ColorGold)
		}

		render.DrawText(screen, item.Name, 28, y, clr)

		// Show restore type and amount with appropriate color
		switch item.Consumable {
		case entity.ConsumeHP:
			render.DrawText(screen, "HP+"+intToStr(item.StatBoost), 200, y, render.ColorGreen)
		case entity.ConsumeMP:
			render.DrawText(screen, "MP+"+intToStr(item.StatBoost), 200, y, render.ColorSky)
		case entity.ConsumeAntidote:
			render.DrawText(screen, "Cure", 220, y, render.ColorMint)
		case entity.ConsumeSmoke:
			render.DrawText(screen, "Flee", 220, y, render.ColorGray)
		case entity.ConsumeATKBuff:
			render.DrawText(screen, "ATK+"+intToStr(item.StatBoost), 200, y, render.ColorPeach)
		case entity.ConsumeDEFBuff:
			render.DrawText(screen, "DEF+"+intToStr(item.StatBoost), 200, y, render.ColorSky)
		}
	}

	render.DrawText(screen, "Z:Use X:Back", 100, boxY+boxH-16, render.ColorGray)
}

func (c *CombatScreen) drawVictoryBox(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 8, boxY, 304, boxH, render.ColorBoxBG, render.ColorBoxBorder)

	render.DrawText(screen, "Victory!", 120, boxY+10, render.ColorGold)

	if c.animTick >= 30 {
		render.DrawText(screen, "XP +"+intToStr(c.xpGained), 28, boxY+34, render.ColorMint)
		render.DrawText(screen, "Coins +"+intToStr(c.coinsGained), 28, boxY+54, render.ColorGold)
		if (c.tick/20)%2 == 0 {
			render.DrawText(screen, "Z:Continue", 110, boxY+boxH-16, render.ColorWhite)
		}
	}
}

func (c *CombatScreen) drawDefeatBox(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 8, boxY, 304, boxH, render.ColorBoxBG, render.ColorBoxBorder)

	render.DrawText(screen, "Defeated...", 110, boxY+10, render.ColorRed)

	if c.animTick >= 30 {
		render.DrawText(screen, "You wake up in town.", 20, boxY+36, render.ColorWhite)
		render.DrawText(screen, "Lost some coins...", 28, boxY+56, render.ColorPeach)
		if (c.tick/20)%2 == 0 {
			render.DrawText(screen, "Z:Continue", 110, boxY+boxH-16, render.ColorWhite)
		}
	}
}

func (c *CombatScreen) drawLevelUpBox(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 8, boxY, 304, boxH, render.ColorBoxBG, render.ColorBoxBorder)

	render.DrawText(screen, "Level Up!", 120, boxY+10, render.ColorGold)
	render.DrawText(screen, "Lv."+intToStr(c.prevLevel)+" -> Lv."+intToStr(c.player.Level), 80, boxY+34, render.ColorMint)
	render.DrawText(screen, "HP and MP restored!", 28, boxY+56, render.ColorGreen)

	if (c.tick/20)%2 == 0 {
		render.DrawText(screen, "Z:Continue", 110, boxY+boxH-16, render.ColorWhite)
	}
}

// hpBarColor returns green/yellow/red based on HP fraction.
func hpBarColor(frac float64) color.Color {
	if frac > 0.5 {
		return render.ColorGreen
	}
	if frac > 0.25 {
		return render.ColorGold
	}
	return render.ColorRed
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// getShakeBuffer lazily creates the offscreen buffer for screen shake.
func (c *CombatScreen) getShakeBuffer() *ebiten.Image {
	if c.shakeBuf == nil {
		c.shakeBuf = ebiten.NewImage(320, 288)
	}
	return c.shakeBuf
}

// Ensure rand is seeded (Go 1.20+ auto-seeds, but just in case)
var _ = rand.Int
