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
// THEORY — Pokemon-style layout at 160x144:
// The Game Boy screen is tiny, so every pixel counts. Classic layout:
//   Top area:    Monster sprite + name + HP bar
//   Middle:      Divider line
//   Bottom-left: Player stats (name, HP bar, MP bar)
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
	"cli_adventure/render"
)

// combatUIState tracks what the combat screen is currently showing.
type combatUIState int

const (
	cuiIntro        combatUIState = iota // "A wild X appeared!" text
	cuiPlayerMenu                        // action menu visible, awaiting input
	cuiSkillMenu                         // skill selection sub-menu
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

	// Screen shake
	shakeIntensity float64
	shakeX, shakeY float64
	shakeBuf       *ebiten.Image // offscreen buffer for shake effect
}

func NewCombatScreen(switcher ScreenSwitcher, player *entity.Player, monster *entity.Monster, areaKey string) *CombatScreen {
	return NewCombatScreenAt(switcher, player, monster, areaKey, -1, -1)
}

// NewCombatScreenAt creates a combat screen that remembers where the player was.
func NewCombatScreenAt(switcher ScreenSwitcher, player *entity.Player, monster *entity.Monster, areaKey string, retX, retY int) *CombatScreen {
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

	switch c.uiState {
	case cuiIntro:
		c.updateIntro()
	case cuiPlayerMenu:
		c.updatePlayerMenu()
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
	// 2x2 menu navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		if c.menuIdx%2 == 1 {
			c.menuIdx--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		if c.menuIdx%2 == 0 {
			c.menuIdx++
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		if c.menuIdx >= 2 {
			c.menuIdx -= 2
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if c.menuIdx < 2 {
			c.menuIdx += 2
		}
	}

	// Confirm action
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		actions := []combat.Action{
			combat.ActionAttack,
			combat.ActionMagic,
			combat.ActionDefend,
			combat.ActionFlee,
		}
		c.doPlayerAction(actions[c.menuIdx])
	}
}

func (c *CombatScreen) doPlayerAction(action combat.Action) {
	result := c.engine.PlayerAction(action)
	c.msgText = result.Message
	c.animTick = 0
	c.showDamage = false
	c.showHeal = false

	switch c.engine.Phase {
	case combat.PhaseVictory:
		// Player killed the monster
		if result.Damage > 0 {
			c.damageNum = result.Damage
			c.showDamage = true
			c.monsterShakeX = 4
		}
		c.msgQueue = []string{result.Message}
		if result.IsCrit {
			c.msgQueue = append(c.msgQueue, "Critical hit!")
		}
		c.uiState = cuiPlayerAnim
	case combat.PhaseFlee:
		c.uiState = cuiMessage
	case combat.PhaseFleeFaild:
		c.msgQueue = []string{result.Message}
		c.uiState = cuiMessage
	default:
		// Normal action
		c.msgQueue = []string{result.Message}
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

	c.msgQueue = []string{result.Message}
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

// returnAfterVictory goes to the ending screen if boss was defeated,
// otherwise returns to the wild area at the player's pre-combat position.
func (c *CombatScreen) returnAfterVictory() {
	if c.monster.IsBoss {
		// Boss defeated — show the ending!
		c.switcher.SwitchScreen(NewEndingScreen(c.switcher, c.player))
	} else {
		c.switcher.SwitchScreen(NewWildScreenAt(c.switcher, c.player, c.areaKey, c.returnX, c.returnY))
	}
}

// returnToWild creates a WildScreen at the player's pre-combat position.
func (c *CombatScreen) returnToWild() {
	c.switcher.SwitchScreen(NewWildScreenAt(c.switcher, c.player, c.areaKey, c.returnX, c.returnY))
}

func (c *CombatScreen) updateDefeat() {
	c.animTick++
	// On Z, return to menu (restart)
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if c.animTick >= 30 {
			// Heal player and send back to town (mercy mechanic)
			c.player.Stats.HP = c.player.Stats.MaxHP
			c.player.Stats.MP = c.player.Stats.MaxMP
			// Lose some coins as penalty
			c.player.Coins = c.player.Coins * 3 / 4
			c.switcher.SwitchScreen(NewTownScreen(c.switcher, c.player))
		}
	}
}

// ---- Draw ----

func (c *CombatScreen) Draw(screen *ebiten.Image) {
	// THEORY — Screen shake via offscreen buffer:
	// We render the entire combat scene to an offscreen image at native
	// resolution (160x144), then draw that image onto the real screen
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
		y := 40.0 - c.damageY
		if c.uiState == cuiEnemyAnim {
			// Damage on player
			y = 75.0 - c.damageY
			render.DrawText(buf, intToStr(c.damageNum), 120, int(y), render.ColorRed)
		} else {
			// Damage on monster
			render.DrawText(buf, intToStr(c.damageNum), 90, int(y), render.ColorGold)
		}
	}
	if c.showHeal {
		render.DrawText(buf, "+"+intToStr(c.healNum), 100, 70, render.ColorGreen)
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
	render.DrawText(screen, c.monster.Name, 4, 2, nameClr)

	// Monster HP bar
	monFrac := float64(c.displayMonHP) / float64(c.monster.MaxHP)
	hpColor := hpBarColor(monFrac)
	render.DrawText(screen, "HP", 4, 11, render.ColorWhite)
	render.DrawBar(screen, 18, 12, 50, 4, monFrac, hpColor, render.ColorDarkGray)

	// Monster sprite
	sprite, ok := c.monsterSprites[c.monster.SpriteID]
	if ok {
		frame := c.monsterAnim.CurrentFrame()
		sx := 100.0 + c.monsterShakeX
		sy := 4.0
		if c.monster.IsBoss {
			sx = 90.0 + c.monsterShakeX
		} else {
			sy = 10.0
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
	for x := 4; x < 156; x += 2 {
		screen.Set(x, 55, render.ColorDarkGray)
	}
}

func (c *CombatScreen) drawPlayerArea(screen *ebiten.Image) {
	// Player sprite (small, bottom-left)
	sheet := c.charSprites[int(c.player.Class)]
	sx := 8.0 + c.playerShakeX
	sheet.DrawFrame(screen, 0, sx, 60.0)

	// Player info
	info := entity.ClassTable[c.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(c.player.Level), 30, 58, render.ColorSky)

	// HP bar
	plrHPFrac := float64(c.displayPlrHP) / float64(c.player.Stats.MaxHP)
	hpColor := hpBarColor(plrHPFrac)
	render.DrawText(screen, "HP", 30, 68, render.ColorWhite)
	render.DrawBar(screen, 44, 69, 60, 4, plrHPFrac, hpColor, render.ColorDarkGray)
	render.DrawText(screen, intToStr(int(c.displayPlrHP))+"/"+intToStr(c.player.Stats.MaxHP), 108, 68, render.ColorWhite)

	// MP bar
	mpFrac := 0.0
	if c.player.Stats.MaxMP > 0 {
		mpFrac = float64(c.player.Stats.MP) / float64(c.player.Stats.MaxMP)
	}
	render.DrawText(screen, "MP", 30, 78, render.ColorWhite)
	render.DrawBar(screen, 44, 79, 60, 4, mpFrac, render.ColorSky, render.ColorDarkGray)
	render.DrawText(screen, intToStr(c.player.Stats.MP)+"/"+intToStr(c.player.Stats.MaxMP), 108, 78, render.ColorWhite)
}

func (c *CombatScreen) drawBottomUI(screen *ebiten.Image) {
	boxY := 90
	boxH := 50

	switch c.uiState {
	case cuiPlayerMenu:
		c.drawActionMenu(screen, boxY, boxH)
	case cuiVictory:
		c.drawVictoryBox(screen, boxY, boxH)
	case cuiDefeat:
		c.drawDefeatBox(screen, boxY, boxH)
	case cuiLevelUp:
		c.drawLevelUpBox(screen, boxY, boxH)
	default:
		// Message box
		render.DrawBox(screen, 4, boxY, 152, boxH, render.ColorBoxBG, render.ColorBoxBorder)
		render.DrawText(screen, c.msgText, 10, boxY+6, render.ColorWhite)

		// Show "Z" prompt if waiting for input
		if c.uiState == cuiMessage || c.uiState == cuiIntro {
			if (c.tick/20)%2 == 0 {
				render.DrawText(screen, "Z", 144, boxY+boxH-10, render.ColorGold)
			}
		}
	}
}

func (c *CombatScreen) drawActionMenu(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 4, boxY, 152, boxH, render.ColorBoxBG, render.ColorBoxBorder)

	// Class-specific skill name
	skillName := "Magic"
	switch c.player.Class {
	case entity.ClassKnight:
		skillName = "Bash"
	case entity.ClassMage:
		skillName = "Fire"
	case entity.ClassArcher:
		skillName = "Snipe"
	}
	labels := []string{"Attack", skillName, "Defend", "Flee"}
	positions := [][2]int{
		{14, boxY + 8},
		{84, boxY + 8},
		{14, boxY + 24},
		{84, boxY + 24},
	}

	for i, label := range labels {
		clr := color.Color(render.ColorWhite)
		if i == c.menuIdx {
			clr = render.ColorGold
			// Draw cursor
			render.DrawCursor(screen, positions[i][0]-8, positions[i][1], render.ColorGold)
		}
		render.DrawText(screen, label, positions[i][0], positions[i][1], clr)
	}

	// Subtle hint
	render.DrawText(screen, "Z:Select", 52, boxY+boxH-10, render.ColorGray)
}

func (c *CombatScreen) drawVictoryBox(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 4, boxY, 152, boxH, render.ColorBoxBG, render.ColorBoxBorder)

	render.DrawText(screen, "Victory!", 54, boxY+4, render.ColorGold)

	if c.animTick >= 30 {
		render.DrawText(screen, "XP +"+intToStr(c.xpGained), 14, boxY+16, render.ColorMint)
		render.DrawText(screen, "Coins +"+intToStr(c.coinsGained), 14, boxY+26, render.ColorGold)
		if (c.tick/20)%2 == 0 {
			render.DrawText(screen, "Z:Continue", 46, boxY+boxH-10, render.ColorWhite)
		}
	}
}

func (c *CombatScreen) drawDefeatBox(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 4, boxY, 152, boxH, render.ColorBoxBG, render.ColorBoxBorder)

	render.DrawText(screen, "Defeated...", 44, boxY+4, render.ColorRed)

	if c.animTick >= 30 {
		render.DrawText(screen, "You wake up in town.", 10, boxY+18, render.ColorWhite)
		render.DrawText(screen, "Lost some coins...", 14, boxY+28, render.ColorPeach)
		if (c.tick/20)%2 == 0 {
			render.DrawText(screen, "Z:Continue", 46, boxY+boxH-10, render.ColorWhite)
		}
	}
}

func (c *CombatScreen) drawLevelUpBox(screen *ebiten.Image, boxY, boxH int) {
	render.DrawBox(screen, 4, boxY, 152, boxH, render.ColorBoxBG, render.ColorBoxBorder)

	render.DrawText(screen, "Level Up!", 48, boxY+4, render.ColorGold)
	render.DrawText(screen, "Lv."+intToStr(c.prevLevel)+" -> Lv."+intToStr(c.player.Level), 30, boxY+16, render.ColorMint)
	render.DrawText(screen, "HP and MP restored!", 14, boxY+28, render.ColorGreen)

	if (c.tick/20)%2 == 0 {
		render.DrawText(screen, "Z:Continue", 46, boxY+boxH-10, render.ColorWhite)
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
		c.shakeBuf = ebiten.NewImage(160, 144)
	}
	return c.shakeBuf
}

// Ensure rand is seeded (Go 1.20+ auto-seeds, but just in case)
var _ = rand.Int
