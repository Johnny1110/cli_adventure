package screen

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/entity"
	"cli_adventure/render"
)

// WildStubScreen is a placeholder for Phase 3's full exploration implementation.
type WildStubScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
}

func NewWildStubScreen(switcher ScreenSwitcher, player *entity.Player) *WildStubScreen {
	return &WildStubScreen{
		switcher: switcher,
		player:   player,
	}
}

func (w *WildStubScreen) OnEnter() {}
func (w *WildStubScreen) OnExit()  {}

func (w *WildStubScreen) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		w.switcher.SwitchScreen(NewTownScreen(w.switcher, w.player))
	}
	return nil
}

func (w *WildStubScreen) Draw(screen *ebiten.Image) {
	render.DrawText(screen, "Enchanted Forest", 30, 10, render.ColorMint)
	render.DrawText(screen, "Wild exploration", 30, 40, render.ColorWhite)
	render.DrawText(screen, "coming in Phase 3!", 28, 52, render.ColorWhite)
	render.DrawText(screen, "Esc: back to town", 28, 80, render.ColorGray)
}
