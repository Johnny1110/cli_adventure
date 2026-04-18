// screen/equip.go — Equipment panel screen.
//
// THEORY — Equipment review as feedback loop:
// In RPGs, the equipment screen is where players "feel" their progression.
// Seeing your ATK go from 8 to 14 because you equipped a Steel Sword is deeply
// satisfying — it's the numeric proof that grinding paid off. This screen shows:
//   1. Currently equipped weapon and armor with their stat bonuses
//   2. Total stats (base + equipment) with color-coded additions
//   3. Class identity and level
//
// The layout is inspired by classic Game Boy RPG menus: compact, text-heavy,
// using color to distinguish base stats from equipment bonuses. The "E" key
// opens this from walking screens, and X closes it.
package screen

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/entity"
	"cli_adventure/render"
)

// EquipScreen shows the player's equipment and full stat breakdown.
type EquipScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
	returnTo Screen
	tick     int
}

// NewEquipScreen creates the equipment review panel.
func NewEquipScreen(switcher ScreenSwitcher, player *entity.Player, returnTo Screen) *EquipScreen {
	return &EquipScreen{
		switcher: switcher,
		player:   player,
		returnTo: returnTo,
	}
}

func (e *EquipScreen) OnEnter() {}
func (e *EquipScreen) OnExit()  {}

func (e *EquipScreen) Update() error {
	e.tick++

	// Close
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsKeyJustPressed(ebiten.KeyE) {
		e.switcher.SwitchScreen(e.returnTo)
	}
	return nil
}

func (e *EquipScreen) Draw(screen *ebiten.Image) {
	screen.Fill(render.ColorBG)

	// Title with pulsing gold
	render.DrawText(screen, "Equipment", 48, 2, render.ColorGold)

	// Class and level
	info := entity.ClassTable[e.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(e.player.Level), 4, 14, render.ColorPink)

	// Divider
	for x := 4; x < 156; x += 2 {
		screen.Set(x, 23, render.ColorDarkGray)
	}

	// === Equipped items ===
	y := 26
	render.DrawText(screen, "Weapon:", 4, y, render.ColorGray)
	if e.player.Weapon != nil {
		render.DrawText(screen, e.player.Weapon.Name, 52, y, render.ColorPeach)
		render.DrawText(screen, "ATK+"+intToStr(e.player.Weapon.StatBoost), 120, y, render.ColorGold)
	} else {
		render.DrawText(screen, "(none)", 52, y, render.ColorDarkGray)
	}

	y += 12
	render.DrawText(screen, "Armor:", 4, y, render.ColorGray)
	if e.player.Armor != nil {
		render.DrawText(screen, e.player.Armor.Name, 52, y, render.ColorSky)
		render.DrawText(screen, "DEF+"+intToStr(e.player.Armor.StatBoost), 120, y, render.ColorGold)
	} else {
		render.DrawText(screen, "(none)", 52, y, render.ColorDarkGray)
	}

	// Divider
	y += 14
	for x := 4; x < 156; x += 2 {
		screen.Set(x, y, render.ColorDarkGray)
	}
	y += 4

	// === Full stat breakdown ===
	render.DrawText(screen, "Stats", 4, y, render.ColorMint)
	y += 12

	s := e.player.Stats
	baseATK := s.ATK
	bonusATK := 0
	if e.player.Weapon != nil {
		bonusATK = e.player.Weapon.StatBoost
	}
	baseDEF := s.DEF
	bonusDEF := 0
	if e.player.Armor != nil {
		bonusDEF = e.player.Armor.StatBoost
	}

	// HP bar
	render.DrawText(screen, "HP", 4, y, render.ColorGreen)
	render.DrawText(screen, intToStr(s.HP)+"/"+intToStr(s.MaxHP), 24, y, render.ColorWhite)
	render.DrawBar(screen, 80, y+1, 70, 4, float64(s.HP)/float64(s.MaxHP), render.ColorGreen, render.ColorDarkGray)
	y += 10

	// MP bar
	render.DrawText(screen, "MP", 4, y, render.ColorSky)
	render.DrawText(screen, intToStr(s.MP)+"/"+intToStr(s.MaxMP), 24, y, render.ColorWhite)
	mpFrac := 0.0
	if s.MaxMP > 0 {
		mpFrac = float64(s.MP) / float64(s.MaxMP)
	}
	render.DrawBar(screen, 80, y+1, 70, 4, mpFrac, render.ColorSky, render.ColorDarkGray)
	y += 12

	// ATK: base + bonus
	e.drawStatLine(screen, "ATK", baseATK, bonusATK, render.ColorPeach, y)
	y += 10

	// DEF: base + bonus
	e.drawStatLine(screen, "DEF", baseDEF, bonusDEF, render.ColorSky, y)
	y += 10

	// SPD (no equipment bonus for now)
	render.DrawText(screen, "SPD", 4, y, render.ColorGold)
	render.DrawText(screen, intToStr(s.SPD), 30, y, render.ColorWhite)
	y += 12

	// Divider
	for x := 4; x < 156; x += 2 {
		screen.Set(x, y, render.ColorDarkGray)
	}
	y += 4

	// XP progress
	render.DrawText(screen, "XP", 4, y, render.ColorLavender)
	render.DrawText(screen, intToStr(e.player.XP)+"/"+intToStr(e.player.XPToNextLevel()), 24, y, render.ColorWhite)
	xpFrac := float64(e.player.XP) / float64(e.player.XPToNextLevel())
	render.DrawBar(screen, 80, y+1, 70, 4, xpFrac, render.ColorLavender, render.ColorDarkGray)
	y += 10

	// Coins
	render.DrawText(screen, "Coins: "+intToStr(e.player.Coins)+"G", 4, y, render.ColorGold)

	// Controls
	render.DrawText(screen, "X:Close", 56, 136, render.ColorGray)
}

// drawStatLine renders "LABEL: base (+bonus) = total" with colored bonus.
func (e *EquipScreen) drawStatLine(screen *ebiten.Image, label string, base, bonus int, clr color.Color, y int) {
	render.DrawText(screen, label, 4, y, clr)
	if bonus > 0 {
		render.DrawText(screen, intToStr(base), 30, y, render.ColorWhite)
		render.DrawText(screen, "(+"+intToStr(bonus)+")", 52, y, render.ColorGold)
		render.DrawText(screen, "= "+intToStr(base+bonus), 86, y, clr)
	} else {
		render.DrawText(screen, intToStr(base), 30, y, render.ColorWhite)
	}
}
