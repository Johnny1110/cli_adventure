// screen/bag.go — Inventory/Bag screen for managing gear and items.
//
// THEORY — Inventory as categorized lists:
// RPG inventories get unwieldy fast. The classic solution is tabs/categories:
// Weapons, Armor, Consumables. The player scrolls within a category and
// performs context-sensitive actions: Equip for gear, Use for consumables.
//
// THEORY — Equip preview:
// When the player highlights a weapon or armor piece, the UI shows a stat
// comparison: "ATK: 8 → 11 (+3)". This is crucial for player decision-making —
// without it, players have to memorize numbers. Pokemon didn't have this
// (items weren't equippable), but RPGs like Final Fantasy and Dragon Quest do.
//
// THEORY — Screen overlay vs separate screen:
// The bag opens as a full-screen overlay (like a pause menu). It captures all
// input and renders on top of whatever was behind it. When closed, we return
// to the previous screen by calling SwitchScreen back to the caller.
package screen

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/entity"
	"cli_adventure/render"
)

// bagTab represents which category is selected.
//
// THEORY — Tab categories mirror equipment slots:
// With 6 equipment slots, we need more bag tabs. We group defensives
// (helmet, boots, shield) under "Gear" to avoid having 7 tiny tabs.
// The player scrolls through: Weapons | Armor | Gear | Accessory | Items
// "Gear" is the catch-all for head/feet/off-hand — similar to how many
// RPGs (Diablo, Path of Exile) group secondary equipment together.
type bagTab int

const (
	bagTabWeapons   bagTab = iota
	bagTabArmor            // body armor
	bagTabGear             // helmets, boots, shields
	bagTabAccessory        // rings, amulets
	bagTabItems            // consumables
)

// BagScreen is the inventory management overlay.
type BagScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
	returnTo Screen // screen to return to when closing

	tab     bagTab
	cursor  int
	msgText string
	msgTick int
}

// NewBagScreen creates the bag screen. returnTo is the screen we go back to.
func NewBagScreen(switcher ScreenSwitcher, player *entity.Player, returnTo Screen) *BagScreen {
	return &BagScreen{
		switcher: switcher,
		player:   player,
		returnTo: returnTo,
	}
}

func (b *BagScreen) OnEnter() {}
func (b *BagScreen) OnExit()  {}

func (b *BagScreen) Update() error {
	// Message display countdown
	if b.msgTick > 0 {
		b.msgTick--
	}

	// Tab switching with left/right
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		if b.tab > 0 {
			b.tab--
			b.cursor = 0
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		if b.tab < bagTabItems { // bagTabItems is the last tab
			b.tab++
			b.cursor = 0
		}
	}

	// Scroll within category
	items := b.currentItems()
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		b.cursor--
		if b.cursor < 0 {
			if len(items) > 0 {
				b.cursor = len(items) - 1
			} else {
				b.cursor = 0
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		b.cursor++
		if b.cursor >= len(items) {
			b.cursor = 0
		}
	}

	// Action: Z to equip/use
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if len(items) > 0 && b.cursor < len(items) {
			b.doAction(items[b.cursor])
		}
	}

	// Close: X or Escape
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		b.switcher.SwitchScreen(b.returnTo)
	}

	return nil
}

func (b *BagScreen) currentItems() []entity.Item {
	var filtered []entity.Item
	for _, item := range b.player.Items {
		switch b.tab {
		case bagTabWeapons:
			if item.Type == entity.ItemWeapon {
				filtered = append(filtered, item)
			}
		case bagTabArmor:
			if item.Type == entity.ItemArmor {
				filtered = append(filtered, item)
			}
		case bagTabGear:
			if item.Type == entity.ItemHelmet || item.Type == entity.ItemBoots || item.Type == entity.ItemShield {
				filtered = append(filtered, item)
			}
		case bagTabAccessory:
			if item.Type == entity.ItemAccessory {
				filtered = append(filtered, item)
			}
		case bagTabItems:
			if item.Type == entity.ItemConsumable {
				filtered = append(filtered, item)
			}
		}
	}
	return filtered
}

func (b *BagScreen) doAction(item entity.Item) {
	switch item.Type {
	case entity.ItemWeapon, entity.ItemArmor, entity.ItemHelmet, entity.ItemBoots, entity.ItemShield, entity.ItemAccessory:
		// Unified equip logic for all 6 equipment types
		if !item.CanEquip(b.player.Class) {
			b.msgText = "Wrong class!"
			b.msgTick = 40
			return
		}
		if !item.MeetsLevelReq(b.player.Level) {
			b.msgText = "Need Lv." + intToStr(item.LevelReq) + "!"
			b.msgTick = 40
			return
		}
		old := b.player.Equip(item)
		b.removeItem(item)
		if old != nil {
			b.player.AddItem(*old)
		}
		b.msgText = "Equipped " + item.DisplayName() + "!"
		b.msgTick = 60
	case entity.ItemConsumable:
		switch item.Consumable {
		case entity.ConsumeHP:
			if b.player.Stats.HP >= b.player.Stats.MaxHP {
				b.msgText = "HP already full!"
				b.msgTick = 40
				return
			}
			b.player.Stats.HP += item.StatBoost
			if b.player.Stats.HP > b.player.Stats.MaxHP {
				b.player.Stats.HP = b.player.Stats.MaxHP
			}
		case entity.ConsumeMP:
			if b.player.Stats.MP >= b.player.Stats.MaxMP {
				b.msgText = "MP already full!"
				b.msgTick = 40
				return
			}
			b.player.Stats.MP += item.StatBoost
			if b.player.Stats.MP > b.player.Stats.MaxMP {
				b.player.Stats.MP = b.player.Stats.MaxMP
			}
		default:
			// Antidote, Smoke Bomb, buff items: only usable in combat
			b.msgText = "Use in combat!"
			b.msgTick = 40
			return
		}
		b.removeItem(item)
		b.msgText = "Used " + item.Name + "!"
		b.msgTick = 60
	}
	// Adjust cursor if list got shorter
	items := b.currentItems()
	if b.cursor >= len(items) {
		b.cursor = len(items) - 1
		if b.cursor < 0 {
			b.cursor = 0
		}
	}
}

func (b *BagScreen) removeItem(item entity.Item) {
	for i, it := range b.player.Items {
		if it.Name == item.Name && it.Type == item.Type {
			b.player.Items = append(b.player.Items[:i], b.player.Items[i+1:]...)
			return
		}
	}
}

func (b *BagScreen) Draw(screen *ebiten.Image) {
	screen.Fill(render.ColorBG)

	// Title
	render.DrawText(screen, "Bag", 144, 4, render.ColorGold)

	// Tab bar
	tabNames := []string{"Wpn", "Armor", "Gear", "Acc", "Items"}
	tabPositions := []int{4, 52, 112, 168, 220}
	tabWidths := []int{36, 48, 44, 36, 48}

	for i, name := range tabNames {
		clr := color.Color(render.ColorGray)
		if bagTab(i) == b.tab {
			clr = render.ColorGold
		}
		render.DrawText(screen, name, tabPositions[i], 24, clr)
	}

	// Underline active tab
	ulX := tabPositions[b.tab]
	ulW := tabWidths[b.tab]
	underline := ebiten.NewImage(ulW, 1)
	underline.Fill(render.ColorGold)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(ulX), 38)
	screen.DrawImage(underline, op)

	// Item list
	items := b.currentItems()
	listY := 48
	maxVisible := 8
	startIdx := 0
	if b.cursor >= maxVisible {
		startIdx = b.cursor - maxVisible + 1
	}

	if len(items) == 0 {
		render.DrawText(screen, "(empty)", 60, listY+40, render.ColorGray)
	}

	for i := startIdx; i < len(items) && i < startIdx+maxVisible; i++ {
		item := items[i]
		y := listY + (i-startIdx)*18

		// Item name colored by rarity (gold cursor overrides)
		clr := render.RarityColor(int(item.Rarity))
		if i == b.cursor {
			clr = render.ColorGold
			render.DrawCursor(screen, 12, y, render.ColorGold)
		}

		render.DrawText(screen, item.DisplayName(), 28, y, clr)

		// Stat boost on the right (use EffectiveStatBoost for equipment)
		switch item.Type {
		case entity.ItemWeapon:
			render.DrawText(screen, "ATK+"+intToStr(item.EffectiveStatBoost()), 200, y, render.ColorPeach)
		case entity.ItemArmor, entity.ItemHelmet, entity.ItemBoots, entity.ItemShield:
			render.DrawText(screen, "DEF+"+intToStr(item.EffectiveStatBoost()), 200, y, render.ColorSky)
		case entity.ItemAccessory:
			// Show the bonus stat type (rarity-scaled)
			bonusLabel := bonusTypeLabel(item.BonusType)
			if bonusLabel != "" {
				render.DrawText(screen, bonusLabel+"+"+intToStr(item.EffectiveBonusStat()), 200, y, render.ColorMint)
			}
		case entity.ItemConsumable:
			switch item.Consumable {
			case entity.ConsumeHP:
				render.DrawText(screen, "HP+"+intToStr(item.StatBoost), 200, y, render.ColorGreen)
			case entity.ConsumeMP:
				render.DrawText(screen, "MP+"+intToStr(item.StatBoost), 200, y, render.ColorSky)
			case entity.ConsumeAntidote:
				render.DrawText(screen, "Cure", 224, y, render.ColorMint)
			case entity.ConsumeSmoke:
				render.DrawText(screen, "Flee", 224, y, render.ColorGray)
			case entity.ConsumeATKBuff:
				render.DrawText(screen, "ATK+"+intToStr(item.StatBoost), 200, y, render.ColorPeach)
			case entity.ConsumeDEFBuff:
				render.DrawText(screen, "DEF+"+intToStr(item.StatBoost), 200, y, render.ColorSky)
			}
		}
		// Show level requirement if player is underleveled
		if item.LevelReq > 0 && item.LevelReq > b.player.Level {
			render.DrawText(screen, "Lv"+intToStr(item.LevelReq), 276, y, render.ColorDarkGray)
		}
	}

	// Currently equipped section — compact 6-slot display
	eqY := 200
	render.DrawText(screen, "Equipped:", 8, eqY, render.ColorGray)
	eqY += 14
	b.drawSlotLine(screen, "Wpn", b.player.Weapon, 8, eqY, render.ColorPeach)
	b.drawSlotLine(screen, "Arm", b.player.Armor, 160, eqY, render.ColorSky)
	eqY += 12
	b.drawSlotLine(screen, "Hlm", b.player.Helmet, 8, eqY, render.ColorSky)
	b.drawSlotLine(screen, "Bts", b.player.Boots, 160, eqY, render.ColorMint)
	eqY += 12
	b.drawSlotLine(screen, "Shd", b.player.Shield, 8, eqY, render.ColorSky)
	b.drawSlotLine(screen, "Acc", b.player.Accessory, 160, eqY, render.ColorLavender)

	// Message overlay
	if b.msgTick > 0 {
		render.DrawBox(screen, 40, 112, 240, 40, render.ColorBoxBG, render.ColorGold)
		render.DrawText(screen, b.msgText, 52, 120, render.ColorGold)
	}

	// Controls hint
	render.DrawText(screen, "Z:Use X:Close", 100, 276, render.ColorGray)
}

// drawSlotLine draws a compact "Label:ItemName" or "Label:(none)" for the equipped section.
// Uses rarity color for the item name.
func (b *BagScreen) drawSlotLine(screen *ebiten.Image, label string, slot *entity.Item, x, y int, _ color.Color) {
	if slot != nil {
		name := slot.DisplayName()
		if len(name) > 12 {
			name = name[:12] // truncate for space
		}
		nameClr := render.RarityColor(int(slot.Rarity))
		render.DrawText(screen, label+":"+name, x, y, nameClr)
	} else {
		render.DrawText(screen, label+":-", x, y, render.ColorDarkGray)
	}
}

// bonusTypeLabel returns a short display label for a BonusStatType.
func bonusTypeLabel(bt entity.BonusStatType) string {
	switch bt {
	case entity.BonusHP:
		return "HP"
	case entity.BonusMP:
		return "MP"
	case entity.BonusATK:
		return "ATK"
	case entity.BonusDEF:
		return "DEF"
	case entity.BonusSPD:
		return "SPD"
	}
	return ""
}
