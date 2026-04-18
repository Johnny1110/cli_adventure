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
	render.DrawText(screen, "Equipment", 112, 4, render.ColorGold)

	// Class and level
	info := entity.ClassTable[e.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(e.player.Level), 8, 28, render.ColorPink)

	// Divider
	for x := 8; x < 312; x += 2 {
		screen.Set(x, 46, render.ColorDarkGray)
	}

	// === Equipped items (all 6 slots) ===
	y := 52
	e.drawEquipSlot(screen, "Weapon:", e.player.Weapon, "ATK", render.ColorPeach, y)
	y += 16
	e.drawEquipSlot(screen, "Armor:", e.player.Armor, "DEF", render.ColorSky, y)
	y += 16
	e.drawEquipSlot(screen, "Helmet:", e.player.Helmet, "DEF", render.ColorSky, y)
	y += 16
	e.drawEquipSlot(screen, "Boots:", e.player.Boots, "DEF", render.ColorMint, y)
	y += 16
	e.drawEquipSlot(screen, "Shield:", e.player.Shield, "DEF", render.ColorSky, y)
	y += 16
	e.drawEquipSlotAcc(screen, "Accsry:", e.player.Accessory, render.ColorLavender, y)

	// Divider
	y += 20
	for x := 8; x < 312; x += 2 {
		screen.Set(x, y, render.ColorDarkGray)
	}
	y += 6

	// === Stat summary ===
	s := e.player.Stats
	bonusATK := e.player.EffectiveATK() - s.ATK
	bonusDEF := e.player.EffectiveDEF() - s.DEF
	bonusSPD := e.player.EffectiveSPD() - s.SPD

	// HP bar
	render.DrawText(screen, "HP", 8, y, render.ColorGreen)
	render.DrawText(screen, intToStr(s.HP)+"/"+intToStr(s.MaxHP), 40, y, render.ColorWhite)
	render.DrawBar(screen, 160, y+1, 140, 8, float64(s.HP)/float64(s.MaxHP), render.ColorGreen, render.ColorDarkGray)
	y += 14

	// MP bar
	render.DrawText(screen, "MP", 8, y, render.ColorSky)
	render.DrawText(screen, intToStr(s.MP)+"/"+intToStr(s.MaxMP), 40, y, render.ColorWhite)
	mpFrac := 0.0
	if s.MaxMP > 0 {
		mpFrac = float64(s.MP) / float64(s.MaxMP)
	}
	render.DrawBar(screen, 160, y+1, 140, 8, mpFrac, render.ColorSky, render.ColorDarkGray)
	y += 14

	// ATK: base + bonus (from weapon + accessories)
	e.drawStatLine(screen, "ATK", s.ATK, bonusATK, render.ColorPeach, y)
	y += 14

	// DEF: base + bonus (from armor + helmet + boots + shield + accessories)
	e.drawStatLine(screen, "DEF", s.DEF, bonusDEF, render.ColorSky, y)
	y += 14

	// SPD: base + bonus (from boots + accessories)
	e.drawStatLine(screen, "SPD", s.SPD, bonusSPD, render.ColorGold, y)
	y += 14

	// XP progress
	render.DrawText(screen, "XP", 8, y, render.ColorLavender)
	if e.player.IsMaxLevel() {
		render.DrawText(screen, "MAX", 40, y, render.ColorGold)
	} else {
		render.DrawText(screen, intToStr(e.player.XP)+"/"+intToStr(e.player.XPToNextLevel()), 40, y, render.ColorWhite)
		xpFrac := float64(e.player.XP) / float64(e.player.XPToNextLevel())
		render.DrawBar(screen, 160, y+1, 140, 8, xpFrac, render.ColorLavender, render.ColorDarkGray)
	}
	y += 14

	// Coins
	render.DrawText(screen, "Coins: "+intToStr(e.player.Coins)+"G", 8, y, render.ColorGold)

	// Controls
	render.DrawText(screen, "X:Close", 132, 276, render.ColorGray)
}

// drawStatLine renders "LABEL: base (+bonus) = total" with colored bonus.
func (e *EquipScreen) drawStatLine(screen *ebiten.Image, label string, base, bonus int, clr color.Color, y int) {
	render.DrawText(screen, label, 8, y, clr)
	if bonus > 0 {
		render.DrawText(screen, intToStr(base), 52, y, render.ColorWhite)
		render.DrawText(screen, "(+"+intToStr(bonus)+")", 92, y, render.ColorGold)
		render.DrawText(screen, "= "+intToStr(base+bonus), 160, y, clr)
	} else {
		render.DrawText(screen, intToStr(base), 52, y, render.ColorWhite)
	}
}

// drawEquipSlot renders one equipment slot line: "Label: ItemName  STAT+N"
// Item name is colored by rarity.
func (e *EquipScreen) drawEquipSlot(screen *ebiten.Image, label string, slot *entity.Item, statLabel string, _ color.Color, y int) {
	render.DrawText(screen, label, 8, y, render.ColorGray)
	if slot != nil {
		name := slot.DisplayName()
		if len(name) > 14 {
			name = name[:14]
		}
		nameClr := render.RarityColor(int(slot.Rarity))
		render.DrawText(screen, name, 76, y, nameClr)
		render.DrawText(screen, statLabel+"+"+intToStr(slot.EffectiveStatBoost()), 240, y, render.ColorGold)
	} else {
		render.DrawText(screen, "(none)", 76, y, render.ColorDarkGray)
	}
}

// drawEquipSlotAcc renders the accessory slot with its bonus stat type.
// Item name colored by rarity.
func (e *EquipScreen) drawEquipSlotAcc(screen *ebiten.Image, label string, slot *entity.Item, _ color.Color, y int) {
	render.DrawText(screen, label, 8, y, render.ColorGray)
	if slot != nil {
		name := slot.DisplayName()
		if len(name) > 14 {
			name = name[:14]
		}
		nameClr := render.RarityColor(int(slot.Rarity))
		render.DrawText(screen, name, 76, y, nameClr)
		bl := bonusTypeLabel(slot.BonusType)
		if bl != "" && slot.BonusStat > 0 {
			render.DrawText(screen, bl+"+"+intToStr(slot.EffectiveBonusStat()), 240, y, render.ColorGold)
		}
	} else {
		render.DrawText(screen, "(none)", 76, y, render.ColorDarkGray)
	}
}
