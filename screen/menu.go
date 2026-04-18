package screen

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/asset"
	"cli_adventure/entity"
	"cli_adventure/render"
	"cli_adventure/save"
)

// menuPhase tracks which sub-screen we're on within the menu.
//
// THEORY — Two-phase menu:
// The title screen has two entry paths: New Game (pick a class and start fresh)
// and Continue (load a save slot). This is the standard RPG title screen layout
// (Final Fantasy, Dragon Quest, Pokemon). The "Continue" option only appears if
// at least one save slot has data — no point showing it to a brand-new player.
type menuPhase int

const (
	menuMain     menuPhase = iota // "New Game" / "Continue" choice
	menuClassSel                  // class selection (existing flow)
	menuLoadSlot                  // save slot picker for Continue
)

// MenuScreen is the title/class-selection screen.
type MenuScreen struct {
	switcher  ScreenSwitcher
	selected  int // 0=Knight, 1=Mage, 2=Archer (in class-select phase)
	sprites   map[int]*render.SpriteSheet
	anims     map[int]*render.Animation
	titleBob  int // tick counter for title animation
	confirmed bool
	fadeAlpha float64 // for transition effect

	// Menu phase
	phase       menuPhase
	mainChoice  int // 0=New Game, 1=Continue
	hasSaves    bool
	loadSlot    int                             // selected slot in load screen
	loadSlots   [save.MaxSlots]save.SlotSummary // cached slot info
	loadedSave  *save.SaveData                  // non-nil when we're fading into a loaded game
}

// NewMenuScreen creates the main menu.
func NewMenuScreen(switcher ScreenSwitcher) *MenuScreen {
	sprites := asset.GenerateCharSprites()

	anims := map[int]*render.Animation{}
	for i := 0; i < 3; i++ {
		anims[i] = render.NewAnimation([]int{0, 1}, 20)
	}

	m := &MenuScreen{
		switcher: switcher,
		sprites:  sprites,
		anims:    anims,
		phase:    menuMain,
	}

	// Check if any save slots exist
	m.loadSlots = save.ListSlots()
	for _, s := range m.loadSlots {
		if s.Used {
			m.hasSaves = true
			break
		}
	}

	return m
}

func (m *MenuScreen) OnEnter() {
	m.confirmed = false
	m.fadeAlpha = 0
	m.loadedSave = nil
	// Refresh save slots
	m.loadSlots = save.ListSlots()
	m.hasSaves = false
	for _, s := range m.loadSlots {
		if s.Used {
			m.hasSaves = true
			break
		}
	}
}

func (m *MenuScreen) OnExit() {}

func (m *MenuScreen) Update() error {
	m.titleBob++

	for _, a := range m.anims {
		a.Update()
	}

	if m.confirmed {
		m.fadeAlpha += 0.05
		if m.fadeAlpha >= 1.0 {
			if m.loadedSave != nil {
				// Load game: restore player and transition to saved area
				player := save.RestorePlayer(*m.loadedSave)
				if m.loadedSave.CurrentArea == "town" || m.loadedSave.CurrentArea == "" {
					m.switcher.SwitchScreen(NewTownScreen(m.switcher, player))
				} else {
					m.switcher.SwitchScreen(NewWildScreen(m.switcher, player, m.loadedSave.CurrentArea))
				}
			} else {
				// New game
				player := entity.NewPlayer(entity.Class(m.selected))
				m.switcher.SwitchScreen(NewTownScreen(m.switcher, player))
			}
		}
		return nil
	}

	switch m.phase {
	case menuMain:
		m.updateMain()
	case menuClassSel:
		m.updateClassSelect()
	case menuLoadSlot:
		m.updateLoadSlot()
	}

	return nil
}

func (m *MenuScreen) updateMain() {
	maxChoice := 0
	if m.hasSaves {
		maxChoice = 1
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		m.mainChoice--
		if m.mainChoice < 0 {
			m.mainChoice = maxChoice
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		m.mainChoice++
		if m.mainChoice > maxChoice {
			m.mainChoice = 0
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		if m.mainChoice == 0 {
			m.phase = menuClassSel
			m.selected = 0
		} else {
			m.phase = menuLoadSlot
			m.loadSlot = 0
			m.loadSlots = save.ListSlots()
		}
	}
}

func (m *MenuScreen) updateClassSelect() {
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
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		m.confirmed = true
	}
	// Back to main menu
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		m.phase = menuMain
	}
}

func (m *MenuScreen) updateLoadSlot() {
	// Navigate 3 slots + Back option
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		m.loadSlot--
		if m.loadSlot < 0 {
			m.loadSlot = save.MaxSlots // Back option
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		m.loadSlot++
		if m.loadSlot > save.MaxSlots {
			m.loadSlot = 0
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		if m.loadSlot >= save.MaxSlots {
			// Back
			m.phase = menuMain
			return
		}
		if !m.loadSlots[m.loadSlot].Used {
			return // can't load empty slot
		}
		// Load the save data
		sd, err := save.Load(m.loadSlot)
		if err != nil {
			return // corrupt or missing — ignore
		}
		m.loadedSave = &sd
		m.confirmed = true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		m.phase = menuMain
	}
}

func (m *MenuScreen) Draw(screen *ebiten.Image) {
	// Title (always visible) — centered on 320-wide screen
	title := "CLI Adventure"
	titleX := (320 - render.TextWidth(title)) / 2
	titleY := 16
	if (m.titleBob/30)%2 == 0 {
		titleY += 2
	}
	render.DrawText(screen, title, titleX, titleY, render.ColorPink)

	switch m.phase {
	case menuMain:
		m.drawMain(screen)
	case menuClassSel:
		m.drawClassSelect(screen)
	case menuLoadSlot:
		m.drawLoadSlots(screen)
	}

	// Fade overlay for transition — 320x288 canvas
	if m.fadeAlpha > 0 {
		fade := ebiten.NewImage(320, 288)
		a := uint8(m.fadeAlpha * 255)
		if a > 255 {
			a = 255
		}
		fade.Fill(color.RGBA{R: 0, G: 0, B: 0, A: a})
		screen.DrawImage(fade, nil)
	}
}

func (m *MenuScreen) drawMain(screen *ebiten.Image) {
	// Draw character sprites as decoration — spread across 320px width
	// Old: x=12,64,116 for 160px. New: x=40,136,232 for 320px (more breathing room)
	classXPositions := [3]int{40, 136, 232}
	for i := 0; i < 3; i++ {
		cx := classXPositions[i]
		cy := 68
		frame := m.anims[i].CurrentFrame()
		m.sprites[i].DrawFrame(screen, frame, float64(cx+12), float64(cy))
	}

	// Menu options — centered on 320px screen
	optY := 148
	options := []string{"New Game"}
	if m.hasSaves {
		options = append(options, "Continue")
	}

	for i, opt := range options {
		y := optY + i*28
		clr := render.ColorWhite
		if i == m.mainChoice {
			render.DrawCursor(screen, 110, y, render.ColorGold)
			clr = render.ColorGold
		}
		render.DrawText(screen, opt, 122, y, clr)
	}

	hint := "Up/Down  Z:Select"
	hintX := (320 - render.TextWidth(hint)) / 2
	render.DrawText(screen, hint, hintX, 276, render.ColorGray)
}

func (m *MenuScreen) drawClassSelect(screen *ebiten.Image) {
	sub := "Choose your class"
	subX := (320 - render.TextWidth(sub)) / 2
	render.DrawText(screen, sub, subX, 44, render.ColorSky)

	classNames := []string{"Knight", "Mage", "Archer"}
	classColors := []color.Color{render.ColorSky, render.ColorLavender, render.ColorMint}

	// Old: cx = 12 + i*52 (boxes of 44px wide). New: spread across 320px with wider boxes.
	// 3 boxes of 72px wide, spaced evenly: start at 28, gap ~38px between boxes
	// Positions: 28, 124, 220
	for i := 0; i < 3; i++ {
		cx := 28 + i*96
		cy := 72

		if i == m.selected {
			render.DrawBox(screen, cx-2, cy-2, 72, 80, render.ColorDarkGray, classColors[i])
		}

		spriteX := float64(cx + 24)
		spriteY := float64(cy + 4)
		frame := m.anims[i].CurrentFrame()
		if i != m.selected {
			frame = 0
		}
		m.sprites[i].DrawFrame(screen, frame, spriteX, spriteY)

		nameX := cx + (68-render.TextWidth(classNames[i]))/2
		render.DrawText(screen, classNames[i], nameX, cy+50, classColors[i])
	}

	drawStatsPanel(screen, m.selected)

	hint := "Left/Right Z:Select X:Back"
	hintX := (320 - render.TextWidth(hint)) / 2
	render.DrawText(screen, hint, hintX, 276, render.ColorGray)
}

func (m *MenuScreen) drawLoadSlots(screen *ebiten.Image) {
	sub := "Load Game"
	subX := (320 - render.TextWidth(sub)) / 2
	render.DrawText(screen, sub, subX, 44, render.ColorSky)

	classNames := []string{"Knight", "Mage", "Archer"}

	for i := 0; i < save.MaxSlots; i++ {
		y := 80 + i*52
		render.DrawBox(screen, 36, y-2, 248, 42, render.ColorBoxBG, render.ColorBoxBorder)

		clr := render.ColorWhite
		if i == m.loadSlot {
			render.DrawBox(screen, 36, y-2, 248, 42, render.ColorDarkGray, render.ColorGold)
			clr = render.ColorGold
		}

		slotLabel := "Slot " + intToStr(i+1) + ": "
		if m.loadSlots[i].Used {
			cn := "???"
			if m.loadSlots[i].Class >= 0 && m.loadSlots[i].Class < len(classNames) {
				cn = classNames[m.loadSlots[i].Class]
			}
			render.DrawText(screen, slotLabel+cn, 46, y+6, clr)
			render.DrawText(screen, "Lv."+intToStr(m.loadSlots[i].Level)+" "+m.loadSlots[i].Area, 46, y+22, render.ColorGray)
		} else {
			render.DrawText(screen, slotLabel+"--- Empty ---", 46, y+6, render.ColorGray)
		}
	}

	// Back option
	backY := 80 + save.MaxSlots*52
	backClr := render.ColorWhite
	if m.loadSlot >= save.MaxSlots {
		render.DrawCursor(screen, 110, backY, render.ColorGold)
		backClr = render.ColorGold
	}
	render.DrawText(screen, "Back", 122, backY, backClr)

	hint := "Up/Down Z:Load X:Back"
	hintX := (320 - render.TextWidth(hint)) / 2
	render.DrawText(screen, hint, hintX, 276, render.ColorGray)
}

func drawStatsPanel(screen *ebiten.Image, classIdx int) {
	info := entity.ClassTable[entity.Class(classIdx)]
	s := info.Base

	bx, by := 20, 200
	render.DrawBox(screen, bx, by, 280, 56, render.ColorBoxBG, render.ColorBoxBorder)

	render.DrawText(screen, info.Desc, bx+8, by+6, render.ColorWhite)

	statsY := by + 34
	render.DrawText(screen, "HP:", bx+8, statsY, render.ColorGreen)
	render.DrawText(screen, intToStr(s.MaxHP), bx+30, statsY, render.ColorWhite)

	render.DrawText(screen, "ATK:", bx+68, statsY, render.ColorRed)
	render.DrawText(screen, intToStr(s.ATK), bx+98, statsY, render.ColorWhite)

	render.DrawText(screen, "DEF:", bx+140, statsY, render.ColorSky)
	render.DrawText(screen, intToStr(s.DEF), bx+170, statsY, render.ColorWhite)

	render.DrawText(screen, "SPD:", bx+210, statsY, render.ColorGold)
	render.DrawText(screen, intToStr(s.SPD), bx+240, statsY, render.ColorWhite)
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
