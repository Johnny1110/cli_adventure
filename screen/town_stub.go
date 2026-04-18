package screen

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/entity"
	"cli_adventure/render"
)

// TownStubScreen is a placeholder for Phase 2's full town implementation.
// It confirms the state machine works by showing the selected class info.
type TownStubScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
}

func NewTownStubScreen(switcher ScreenSwitcher, player *entity.Player) *TownStubScreen {
	return &TownStubScreen{
		switcher: switcher,
		player:   player,
	}
}

func (t *TownStubScreen) OnEnter() {}
func (t *TownStubScreen) OnExit()  {}

func (t *TownStubScreen) Update() error {
	// Press Escape to go back to menu
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.switcher.SwitchScreen(NewMenuScreen(t.switcher))
	}
	return nil
}

func (t *TownStubScreen) Draw(screen *ebiten.Image) {
	// Title
	title := "Peaceful Village"
	titleX := (160 - render.TextWidth(title)) / 2
	render.DrawText(screen, title, titleX, 10, render.ColorMint)

	// Player info
	info := entity.ClassTable[t.player.Class]
	render.DrawText(screen, "Welcome, "+info.Name+"!", 20, 40, render.ColorWhite)

	// Stats display
	render.DrawBox(screen, 20, 55, 120, 50, render.ColorBoxBG, render.ColorBoxBorder)

	s := t.player.Stats
	render.DrawText(screen, "Level: "+intToStr(t.player.Level), 26, 60, render.ColorGold)
	render.DrawText(screen, "HP: "+intToStr(s.HP)+"/"+intToStr(s.MaxHP), 26, 72, render.ColorGreen)
	render.DrawText(screen, "ATK: "+intToStr(s.ATK)+"  DEF: "+intToStr(s.DEF), 26, 82, render.ColorWhite)
	render.DrawText(screen, "Coins: "+intToStr(t.player.Coins), 26, 92, render.ColorGold)

	// Phase 2 coming soon
	render.DrawText(screen, "Town coming in Phase 2!", 16, 116, render.ColorGray)
	render.DrawText(screen, "Esc: back to menu", 30, 130, render.ColorGray)
}
