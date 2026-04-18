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
type bagTab int

const (
	bagTabWeapons bagTab = iota
	bagTabArmor
	bagTabItems
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
		if b.tab < bagTabItems {
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
	case entity.ItemWeapon:
		if !item.CanEquip(b.player.Class) {
			b.msgText = "Can't equip that!"
			b.msgTick = 40
			return
		}
		old := b.player.Equip(item)
		b.removeItem(item)
		if old != nil {
			b.player.AddItem(*old)
		}
		b.msgText = "Equipped " + item.Name + "!"
		b.msgTick = 60
	case entity.ItemArmor:
		if !item.CanEquip(b.player.Class) {
			b.msgText = "Can't equip that!"
			b.msgTick = 40
			return
		}
		old := b.player.Equip(item)
		b.removeItem(item)
		if old != nil {
			b.player.AddItem(*old)
		}
		b.msgText = "Equipped " + item.Name + "!"
		b.msgTick = 60
	case entity.ItemConsumable:
		if b.player.Stats.HP >= b.player.Stats.MaxHP {
			b.msgText = "HP already full!"
			b.msgTick = 40
			return
		}
		b.player.Stats.HP += item.StatBoost
		if b.player.Stats.HP > b.player.Stats.MaxHP {
			b.player.Stats.HP = b.player.Stats.MaxHP
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
	render.DrawText(screen, "Bag", 68, 2, render.ColorGold)

	// Tab bar
	tabNames := []string{"Weapons", "Armor", "Items"}
	tabPositions := []int{4, 56, 100} // x positions
	tabWidths := []int{42, 30, 30}

	for i, name := range tabNames {
		clr := color.Color(render.ColorGray)
		if bagTab(i) == b.tab {
			clr = render.ColorGold
		}
		render.DrawText(screen, name, tabPositions[i], 12, clr)
	}

	// Underline active tab
	ulX := tabPositions[b.tab]
	ulW := tabWidths[b.tab]
	underline := ebiten.NewImage(ulW, 1)
	underline.Fill(render.ColorGold)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(ulX), 19)
	screen.DrawImage(underline, op)

	// Item list
	items := b.currentItems()
	listY := 24
	maxVisible := 6
	startIdx := 0
	if b.cursor >= maxVisible {
		startIdx = b.cursor - maxVisible + 1
	}

	if len(items) == 0 {
		render.DrawText(screen, "(empty)", 30, listY+20, render.ColorGray)
	}

	for i := startIdx; i < len(items) && i < startIdx+maxVisible; i++ {
		item := items[i]
		y := listY + (i-startIdx)*14

		clr := color.Color(render.ColorWhite)
		if i == b.cursor {
			clr = render.ColorGold
			render.DrawCursor(screen, 6, y, render.ColorGold)
		}

		render.DrawText(screen, item.Name, 14, y, clr)

		// Stat boost on the right
		boostStr := "+" + intToStr(item.StatBoost)
		switch item.Type {
		case entity.ItemWeapon:
			render.DrawText(screen, "ATK"+boostStr, 110, y, render.ColorPeach)
		case entity.ItemArmor:
			render.DrawText(screen, "DEF"+boostStr, 110, y, render.ColorSky)
		case entity.ItemConsumable:
			render.DrawText(screen, "HP"+boostStr, 112, y, render.ColorGreen)
		}
	}

	// Currently equipped section
	eqY := 110
	render.DrawText(screen, "Equipped:", 4, eqY, render.ColorGray)
	if b.player.Weapon != nil {
		render.DrawText(screen, "W:"+b.player.Weapon.Name, 4, eqY+10, render.ColorPeach)
	} else {
		render.DrawText(screen, "W:(none)", 4, eqY+10, render.ColorDarkGray)
	}
	if b.player.Armor != nil {
		render.DrawText(screen, "A:"+b.player.Armor.Name, 80, eqY+10, render.ColorSky)
	} else {
		render.DrawText(screen, "A:(none)", 80, eqY+10, render.ColorDarkGray)
	}

	// Message overlay
	if b.msgTick > 0 {
		render.DrawBox(screen, 20, 56, 120, 20, render.ColorBoxBG, render.ColorGold)
		render.DrawText(screen, b.msgText, 26, 60, render.ColorGold)
	}

	// Controls hint
	render.DrawText(screen, "Z:Use X:Close", 34, 136, render.ColorGray)
}
