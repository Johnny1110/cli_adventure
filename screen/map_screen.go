// screen/map_screen.go — World map overlay (press M to open).
//
// THEORY — Minimap as area graph:
// Rather than showing a pixel-level minimap (which would be too small at 320x288),
// we show a high-level area graph: boxes representing each zone, connected by
// lines showing how they link together. The current area is highlighted with a
// pulsing border. This gives the player spatial awareness of the game world
// without needing a detailed zoomed-out view.
//
// This is similar to how Metroid shows its map — abstracted room shapes rather
// than pixel-perfect recreations. At our resolution, abstraction is key.
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
	render.DrawText(screen, "World Map", 112, 8, render.ColorGold)

	// THEORY — Hub-and-spoke map layout:
	// The town sits at the center with four chains radiating outward.
	// We arrange them like a compass rose:
	//   North chain goes up, South(east) chain goes down-right,
	//   West chain goes left, East(swamp) chain goes down-left.
	// Actually, to fit the 320x288 screen we use:
	//   Town in center, East chain below (original), North above,
	//   West to the left, South/East to the right side.

	type areaBox struct {
		key  string
		name string
		x, y int
		w, h int
		clr  color.Color
	}

	// Town hub (center)
	townBox := areaBox{"town", "Town", 120, 126, 80, 24, render.ColorMint}

	// East chain (below — original path)
	eastChain := []areaBox{
		{"forest", "Forest", 116, 162, 88, 22, render.ColorGreen},
		{"cave", "Cave", 120, 194, 80, 22, render.ColorLavender},
		{"lair", "Lair", 124, 226, 72, 22, render.ColorRed},
	}

	// North chain (above)
	northChain := []areaBox{
		{"frozen_path", "Frozen Path", 104, 90, 112, 22, render.ColorSky},
		{"snow_mountains", "Snow Mts", 112, 58, 96, 22, render.ColorSky},
		{"ice_cavern", "Ice Cavern", 116, 26, 88, 22, render.ColorSky},
	}

	// West chain (left side)
	westChain := []areaBox{
		{"desert", "Desert", 8, 126, 80, 24, render.ColorGold},
		{"sand_ruins", "Sand Ruins", 4, 162, 88, 22, render.ColorGold},
		{"buried_temple", "Temple", 12, 194, 72, 22, render.ColorGold},
	}

	// South chain (right side — swamp+volcano)
	southChain := []areaBox{
		{"swamp", "Swamp", 232, 126, 80, 24, render.ColorGreen},
		{"volcano", "Volcano", 236, 162, 72, 22, render.ColorRed},
	}

	connColor := render.ColorDarkGray

	// Draw connections first (behind boxes)
	// Town → East chain
	drawVertLine(screen, 160, 150, 162, connColor)
	drawVertLine(screen, 160, 184, 194, connColor)
	drawVertLine(screen, 160, 216, 226, connColor)

	// Town → North chain
	drawVertLine(screen, 160, 90+22, 126, connColor)
	drawVertLine(screen, 160, 58+22, 90, connColor)
	drawVertLine(screen, 160, 26+22, 58, connColor)

	// Town → West chain
	drawHorizLine(screen, 88, 120, 138, connColor)
	drawVertLine(screen, 48, 150, 162, connColor)
	drawVertLine(screen, 48, 184, 194, connColor)

	// Town → South/East chain (swamp side)
	drawHorizLine(screen, 200, 232, 138, connColor)
	drawVertLine(screen, 272, 150, 162, connColor)

	// Collect all boxes for drawing
	allBoxes := []areaBox{townBox}
	allBoxes = append(allBoxes, eastChain...)
	allBoxes = append(allBoxes, northChain...)
	allBoxes = append(allBoxes, westChain...)
	allBoxes = append(allBoxes, southChain...)

	for _, a := range allBoxes {
		borderClr := a.clr
		bgClr := color.Color(render.ColorBoxBG)

		if a.key == m.areaKey {
			if (m.tick/15)%2 == 0 {
				borderClr = render.ColorGold
			}
			bgClr = color.RGBA{R: 30, G: 30, B: 50, A: 240}
		}

		render.DrawBox(screen, a.x, a.y, a.w, a.h, bgClr, borderClr)

		textX := a.x + (a.w-len(a.name)*6)/2
		textY := a.y + (a.h-7)/2
		textClr := a.clr
		if a.key == m.areaKey {
			textClr = render.ColorGold
		}
		render.DrawText(screen, a.name, textX, textY, textClr)
	}

	// Direction labels
	render.DrawText(screen, "N", 156, 18, render.ColorGray)
	render.DrawText(screen, "S", 156, 254, render.ColorGray)
	render.DrawText(screen, "W", 16, 116, render.ColorGray)
	render.DrawText(screen, "E", 296, 116, render.ColorGray)

	// Controls
	render.DrawText(screen, "M:Close", 132, 276, render.ColorGray)
}

// drawVertLine draws a vertical line from y1 to y2 at column x.
func drawVertLine(dst *ebiten.Image, x, y1, y2 int, clr color.Color) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if y2-y1 <= 0 {
		return
	}
	line := ebiten.NewImage(1, y2-y1)
	line.Fill(clr)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y1))
	dst.DrawImage(line, op)
}

// drawHorizLine draws a horizontal line from x1 to x2 at row y.
func drawHorizLine(dst *ebiten.Image, x1, x2, y int, clr color.Color) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if x2-x1 <= 0 {
		return
	}
	line := ebiten.NewImage(x2-x1, 1)
	line.Fill(clr)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x1), float64(y))
	dst.DrawImage(line, op)
}
