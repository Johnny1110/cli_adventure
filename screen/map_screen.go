// screen/map_screen.go — World map overlay (press M to open).
//
// THEORY — Minimap as area graph:
// Rather than showing a pixel-level minimap (which would be too small at 160x144),
// we show a high-level area graph: boxes representing each zone, connected by
// lines showing how they link together. The current area is highlighted with a
// pulsing border. This gives the player spatial awareness of the game world
// without needing a detailed zoomed-out view.
//
// This is similar to how Metroid shows its map — abstracted room shapes rather
// than pixel-perfect recreations. At Game Boy resolution, abstraction is key.
//
// The map also shows whether the player has visited each area (visited = colored,
// unvisited = dimmed). For our simple linear graph (Town → Forest → Cave → Lair),
// all areas are always visible since the path is straightforward.
package screen

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/render"
)

// MapScreen shows the world map overlay.
type MapScreen struct {
	switcher ScreenSwitcher
	returnTo Screen
	areaKey  string // current area the player is in
	tick     int
}

// NewMapScreen creates the map overlay. areaKey is "town", "forest", "cave", or "lair".
func NewMapScreen(switcher ScreenSwitcher, returnTo Screen, areaKey string) *MapScreen {
	return &MapScreen{
		switcher: switcher,
		returnTo: returnTo,
		areaKey:  areaKey,
	}
}

func (m *MapScreen) OnEnter() {}
func (m *MapScreen) OnExit()  {}

func (m *MapScreen) Update() error {
	m.tick++

	// Close map
	if inpututil.IsKeyJustPressed(ebiten.KeyM) ||
		inpututil.IsKeyJustPressed(ebiten.KeyX) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		m.switcher.SwitchScreen(m.returnTo)
	}

	return nil
}

func (m *MapScreen) Draw(screen *ebiten.Image) {
	screen.Fill(render.ColorBG)

	// Title
	render.DrawText(screen, "World Map", 48, 4, render.ColorGold)

	// Area definitions for the map display
	type areaBox struct {
		key    string
		name   string
		x, y   int
		w, h   int
		clr    color.Color
	}

	areas := []areaBox{
		{"town", "Town", 55, 24, 50, 18, render.ColorMint},
		{"forest", "Forest", 50, 54, 60, 18, render.ColorGreen},
		{"cave", "Cave", 52, 84, 56, 18, render.ColorLavender},
		{"lair", "Lair", 55, 114, 50, 18, render.ColorRed},
	}

	// Draw connections (lines between areas)
	connColor := render.ColorDarkGray
	// Town → Forest
	drawVertLine(screen, 80, 42, 54, connColor)
	// Forest → Cave
	drawVertLine(screen, 80, 72, 84, connColor)
	// Cave → Lair
	drawVertLine(screen, 80, 102, 114, connColor)

	// Draw area boxes
	for _, a := range areas {
		borderClr := a.clr
		bgClr := color.Color(render.ColorBoxBG)

		// Current area: pulsing bright border
		if a.key == m.areaKey {
			// Pulse effect
			if (m.tick/15)%2 == 0 {
				borderClr = render.ColorGold
			}
			bgClr = color.RGBA{R: 30, G: 30, B: 50, A: 240}
		}

		render.DrawBox(screen, a.x, a.y, a.w, a.h, bgClr, borderClr)

		// Area name centered in box
		textX := a.x + (a.w-len(a.name)*6)/2
		textY := a.y + (a.h-7)/2
		textClr := a.clr
		if a.key == m.areaKey {
			textClr = render.ColorGold
		}
		render.DrawText(screen, a.name, textX, textY, textClr)

		// "You are here" marker
		if a.key == m.areaKey {
			render.DrawText(screen, "<", a.x+a.w+2, textY, render.ColorGold)
		}
	}

	// Arrow indicators between areas
	render.DrawText(screen, "v", 78, 46, render.ColorGray)
	render.DrawText(screen, "v", 78, 76, render.ColorGray)
	render.DrawText(screen, "v", 78, 106, render.ColorGray)

	// Controls
	render.DrawText(screen, "M:Close", 58, 136, render.ColorGray)
}

// drawVertLine draws a vertical line from y1 to y2 at column x.
func drawVertLine(dst *ebiten.Image, x, y1, y2 int, clr color.Color) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	line := ebiten.NewImage(1, y2-y1)
	line.Fill(clr)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y1))
	dst.DrawImage(line, op)
}
