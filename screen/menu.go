package screen

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/asset"
	"cli_adventure/entity"
	"cli_adventure/render"
)

// MenuScreen is the title/class-selection screen.
//
// THEORY: This screen demonstrates the full rendering pipeline:
// - Pixel-art sprites (character previews)
// - Bitmap text (title, class names, stats)
// - UI elements (selection cursor, stat boxes)
// - Animation (idle bob on selected character)
// - Input handling (arrow keys + confirm)
// - State transitions (menu → town via ScreenSwitcher)
type MenuScreen struct {
	switcher  ScreenSwitcher
	selected  int // 0=Knight, 1=Mage, 2=Archer
	sprites   map[int]*render.SpriteSheet
	anims     map[int]*render.Animation
	titleBob  int // tick counter for title animation
	confirmed bool
	fadeAlpha float64 // for transition effect
}

// NewMenuScreen creates the main menu.
func NewMenuScreen(switcher ScreenSwitcher) *MenuScreen {
	sprites := asset.GenerateCharSprites()

	anims := map[int]*render.Animation{}
	for i := 0; i < 3; i++ {
		anims[i] = render.NewAnimation([]int{0, 1}, 20) // bob every 20 ticks
	}

	return &MenuScreen{
		switcher: switcher,
		sprites:  sprites,
		anims:    anims,
	}
}

func (m *MenuScreen) OnEnter() {
	m.confirmed = false
	m.fadeAlpha = 0
}

func (m *MenuScreen) OnExit() {}

func (m *MenuScreen) Update() error {
	m.titleBob++

	// Update all animations
	for _, a := range m.anims {
		a.Update()
	}

	if m.confirmed {
		// Fade out transition
		m.fadeAlpha += 0.05
		if m.fadeAlpha >= 1.0 {
			// Transition to town (stub for now)
			player := entity.NewPlayer(entity.Class(m.selected))
			m.switcher.SwitchScreen(NewTownScreen(m.switcher, player))
		}
		return nil
	}

	// Input: navigate classes
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		m.selected--
		if m.selected < 0 {
			m.selected = 2
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		m.selected++
		if m.selected > 2 {
			m.selected = 0
		}
	}
	// Confirm selection
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		m.confirmed = true
	}

	return nil
}

func (m *MenuScreen) Draw(screen *ebiten.Image) {
	// Title
	title := "CLI Adventure"
	titleX := (160 - render.TextWidth(title)) / 2
	titleY := 8
	// Gentle bob effect on title
	if (m.titleBob/30)%2 == 0 {
		titleY += 1
	}
	render.DrawText(screen, title, titleX, titleY, render.ColorPink)

	// Subtitle
	sub := "Choose your class"
	subX := (160 - render.TextWidth(sub)) / 2
	render.DrawText(screen, sub, subX, 22, render.ColorSky)

	// Draw 3 class sprites side by side
	classNames := []string{"Knight", "Mage", "Archer"}
	classColors := []color.Color{render.ColorSky, render.ColorLavender, render.ColorMint}

	for i := 0; i < 3; i++ {
		// Position: spread across the 160px width
		cx := 12 + i*52 // 12, 64, 116
		cy := 38

		// Highlight box for selected class
		if i == m.selected {
			render.DrawBox(screen, cx-2, cy-2, 44, 60, render.ColorDarkGray, classColors[i])
		}

		// Draw sprite (centered in the 40px column)
		spriteX := float64(cx + 12)
		spriteY := float64(cy + 2)
		frame := m.anims[i].CurrentFrame()
		// Only animate the selected character
		if i != m.selected {
			frame = 0
		}
		m.sprites[i].DrawFrame(screen, frame, spriteX, spriteY)

		// Class name below sprite
		nameX := cx + (40-render.TextWidth(classNames[i]))/2
		render.DrawText(screen, classNames[i], nameX, cy+22, classColors[i])
	}

	// Stats panel for selected class
	drawStatsPanel(screen, m.selected)

	// Hint text
	hint := "Left/Right  Z:Select"
	hintX := (160 - render.TextWidth(hint)) / 2
	render.DrawText(screen, hint, hintX, 134, render.ColorGray)

	// Fade overlay for transition
	if m.fadeAlpha > 0 {
		fade := ebiten.NewImage(160, 144)
		a := uint8(m.fadeAlpha * 255)
		if a > 255 {
			a = 255
		}
		fade.Fill(color.RGBA{R: 0, G: 0, B: 0, A: a})
		screen.DrawImage(fade, nil)
	}
}

func drawStatsPanel(screen *ebiten.Image, classIdx int) {
	info := entity.ClassTable[entity.Class(classIdx)]
	s := info.Base

	// Draw a box at the bottom
	bx, by := 8, 100
	render.DrawBox(screen, bx, by, 144, 30, render.ColorBoxBG, render.ColorBoxBorder)

	// Class description
	render.DrawText(screen, info.Desc, bx+4, by+3, render.ColorWhite)

	// Stats row
	statsY := by + 20
	render.DrawText(screen, "HP:", bx+4, statsY, render.ColorGreen)
	render.DrawText(screen, intToStr(s.MaxHP), bx+22, statsY, render.ColorWhite)

	render.DrawText(screen, "ATK:", bx+40, statsY, render.ColorRed)
	render.DrawText(screen, intToStr(s.ATK), bx+64, statsY, render.ColorWhite)

	render.DrawText(screen, "DEF:", bx+80, statsY, render.ColorSky)
	render.DrawText(screen, intToStr(s.DEF), bx+104, statsY, render.ColorWhite)

	render.DrawText(screen, "SPD:", bx+116, statsY, render.ColorGold)
	render.DrawText(screen, intToStr(s.SPD), bx+140, statsY, render.ColorWhite)
}

// intToStr converts an int to a string without importing strconv.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
