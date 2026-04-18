// screen/ending.go — Victory ending screen shown after defeating the Dragon.
//
// THEORY — The payoff:
// Every RPG needs a satisfying ending. After the climactic boss fight, the player
// deserves a celebration. Classic Game Boy RPGs used simple but effective techniques:
// a congratulations message, the player's final stats, a "THE END" title, and
// sometimes a slow credit scroll. We keep it simple but make it feel earned.
//
// The ending screen uses a multi-phase reveal:
//   1. "Victory!" title fades in
//   2. A congratulatory message types out
//   3. Final stats are displayed (class, level, play summary)
//   4. "THE END" with option to return to title screen
//
// This pacing gives each moment room to breathe — rushing straight to "THE END"
// would feel anticlimactic after a tough boss fight.
package screen

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/entity"
	"cli_adventure/render"
)

// endingPhase tracks the visual sequence of the ending.
type endingPhase int

const (
	endPhaseTitle   endingPhase = iota // "Victory!" fading in
	endPhaseMessage                    // congratulations message
	endPhaseStats                      // final stats display
	endPhaseEnd                        // "THE END" + return to menu
)

// EndingScreen shows the game ending after the boss is defeated.
type EndingScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
	phase    endingPhase
	tick     int
	phaseTick int
}

func NewEndingScreen(switcher ScreenSwitcher, player *entity.Player) *EndingScreen {
	return &EndingScreen{
		switcher: switcher,
		player:   player,
		phase:    endPhaseTitle,
	}
}

func (e *EndingScreen) OnEnter() {}
func (e *EndingScreen) OnExit()  {}

func (e *EndingScreen) Update() error {
	e.tick++
	e.phaseTick++

	switch e.phase {
	case endPhaseTitle:
		if e.phaseTick >= 90 || inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			e.phase = endPhaseMessage
			e.phaseTick = 0
		}
	case endPhaseMessage:
		if e.phaseTick >= 120 || inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			e.phase = endPhaseStats
			e.phaseTick = 0
		}
	case endPhaseStats:
		if e.phaseTick >= 60 && (inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter)) {
			e.phase = endPhaseEnd
			e.phaseTick = 0
		}
	case endPhaseEnd:
		if e.phaseTick >= 30 && (inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter)) {
			// Return to title screen (new game)
			e.switcher.SwitchScreen(NewMenuScreen(e.switcher))
		}
	}

	return nil
}

func (e *EndingScreen) Draw(screen *ebiten.Image) {
	screen.Fill(render.ColorBG)

	switch e.phase {
	case endPhaseTitle:
		e.drawTitle(screen)
	case endPhaseMessage:
		e.drawTitle(screen)
		e.drawMessage(screen)
	case endPhaseStats:
		e.drawStats(screen)
	case endPhaseEnd:
		e.drawEnd(screen)
	}
}

func (e *EndingScreen) drawTitle(screen *ebiten.Image) {
	// "Victory!" title with star decorations
	y := 30
	if e.phase == endPhaseMessage {
		y = 10 // move up when message appears
	}

	// Stars around the title (twinkling)
	if (e.tick/10)%3 != 0 {
		render.DrawText(screen, "*", 28, y-4, render.ColorGold)
		render.DrawText(screen, "*", 122, y+2, render.ColorGold)
	}
	if (e.tick/10)%3 != 1 {
		render.DrawText(screen, "*", 42, y+10, render.ColorPeach)
		render.DrawText(screen, "*", 110, y-2, render.ColorPeach)
	}

	render.DrawText(screen, "Victory!", 52, y, render.ColorGold)

	// Subtitle
	render.DrawText(screen, "The Dragon is slain!", 20, y+14, render.ColorPink)
}

func (e *EndingScreen) drawMessage(screen *ebiten.Image) {
	msgs := []string{
		"Peace returns to the",
		"village. The people",
		"celebrate their hero!",
	}

	y := 44
	for i, msg := range msgs {
		// Typewriter reveal based on phaseTick
		charsToShow := (e.phaseTick - i*20) * 2
		if charsToShow <= 0 {
			continue
		}
		if charsToShow > len(msg) {
			charsToShow = len(msg)
		}
		render.DrawText(screen, msg[:charsToShow], 10, y+i*12, render.ColorWhite)
	}

	// Prompt
	if e.phaseTick >= 80 && (e.tick/20)%2 == 0 {
		render.DrawText(screen, "Z:Continue", 46, 130, render.ColorGray)
	}
}

func (e *EndingScreen) drawStats(screen *ebiten.Image) {
	// Final hero stats
	render.DrawText(screen, "- Hero's Journey -", 22, 6, render.ColorGold)

	info := entity.ClassTable[e.player.Class]
	y := 24

	render.DrawText(screen, "Class: "+info.Name, 20, y, render.ColorPink)
	y += 12
	render.DrawText(screen, "Level: "+intToStr(e.player.Level), 20, y, render.ColorMint)
	y += 12

	s := e.player.Stats
	render.DrawText(screen, "HP:"+intToStr(s.MaxHP), 20, y, render.ColorGreen)
	render.DrawText(screen, "MP:"+intToStr(s.MaxMP), 80, y, render.ColorSky)
	y += 12
	render.DrawText(screen, "ATK:"+intToStr(e.player.EffectiveATK()), 20, y, render.ColorPeach)
	render.DrawText(screen, "DEF:"+intToStr(e.player.EffectiveDEF()), 80, y, render.ColorSky)
	y += 12
	render.DrawText(screen, "SPD:"+intToStr(s.SPD), 20, y, render.ColorGold)
	y += 16

	// Equipment
	wpn := "None"
	if e.player.Weapon != nil {
		wpn = e.player.Weapon.Name
	}
	arm := "None"
	if e.player.Armor != nil {
		arm = e.player.Armor.Name
	}
	render.DrawText(screen, "Weapon: "+wpn, 14, y, render.ColorPeach)
	y += 10
	render.DrawText(screen, "Armor:  "+arm, 14, y, render.ColorSky)
	y += 14

	// Coins
	render.DrawText(screen, "Coins: "+intToStr(e.player.Coins)+"G", 14, y, render.ColorGold)

	// Quests completed
	done := 0
	for _, q := range e.player.Quests {
		if q.Done {
			done++
		}
	}
	render.DrawText(screen, "Quests: "+intToStr(done)+"/"+intToStr(len(e.player.Quests)), 80, y, render.ColorLavender)

	if (e.tick/20)%2 == 0 {
		render.DrawText(screen, "Z:Continue", 46, 130, render.ColorGray)
	}
}

func (e *EndingScreen) drawEnd(screen *ebiten.Image) {
	// "THE END" with decorative border
	render.DrawBox(screen, 20, 30, 120, 60, render.ColorBoxBG, render.ColorGold)

	// Twinkling stars
	stars := [][2]int{{30, 40}, {120, 45}, {35, 75}, {115, 38}, {80, 35}}
	for i, s := range stars {
		if (e.tick/8+i*3)%4 != 0 {
			render.DrawText(screen, "*", s[0], s[1], render.ColorGold)
		}
	}

	render.DrawText(screen, "THE END", 52, 52, render.ColorGold)

	render.DrawText(screen, "Thanks for playing!", 18, 68, render.ColorPink)

	// Prompt to return
	if e.phaseTick >= 30 && (e.tick/20)%2 == 0 {
		render.DrawText(screen, "Z:Title Screen", 32, 110, render.ColorWhite)
	}
}
