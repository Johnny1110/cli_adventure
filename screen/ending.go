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
	"image/color"

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
	switcher  ScreenSwitcher
	player    *entity.Player
	phase     endingPhase
	tick      int
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
	y := 60
	if e.phase == endPhaseMessage {
		y = 20 // move up when message appears
	}

	// Stars around the title (twinkling)
	if (e.tick/10)%3 != 0 {
		render.DrawText(screen, "*", 56, y-8, render.ColorGold)
		render.DrawText(screen, "*", 244, y+4, render.ColorGold)
	}
	if (e.tick/10)%3 != 1 {
		render.DrawText(screen, "*", 84, y+20, render.ColorPeach)
		render.DrawText(screen, "*", 220, y-4, render.ColorPeach)
	}

	render.DrawText(screen, "Victory!", 120, y, render.ColorGold)

	// Subtitle
	render.DrawText(screen, "The Dragon is slain!", 60, y+28, render.ColorPink)
}

func (e *EndingScreen) drawMessage(screen *ebiten.Image) {
	msgs := []string{
		"Peace returns to the",
		"village. The people",
		"celebrate their hero!",
	}

	y := 88
	for i, msg := range msgs {
		// Typewriter reveal based on phaseTick
		charsToShow := (e.phaseTick - i*20) * 2
		if charsToShow <= 0 {
			continue
		}
		if charsToShow > len(msg) {
			charsToShow = len(msg)
		}
		render.DrawText(screen, msg[:charsToShow], 20, y+i*18, render.ColorWhite)
	}

	// Prompt
	if e.phaseTick >= 80 && (e.tick/20)%2 == 0 {
		render.DrawText(screen, "Z:Continue", 112, 264, render.ColorGray)
	}
}

func (e *EndingScreen) drawStats(screen *ebiten.Image) {
	// Final hero stats
	render.DrawText(screen, "- Hero's Journey -", 80, 12, render.ColorGold)

	info := entity.ClassTable[e.player.Class]
	y := 48

	render.DrawText(screen, "Class: "+info.Name, 40, y, render.ColorPink)
	y += 18
	render.DrawText(screen, "Level: "+intToStr(e.player.Level), 40, y, render.ColorMint)
	y += 18

	s := e.player.Stats
	render.DrawText(screen, "HP:"+intToStr(s.MaxHP), 40, y, render.ColorGreen)
	render.DrawText(screen, "MP:"+intToStr(s.MaxMP), 160, y, render.ColorSky)
	y += 18
	render.DrawText(screen, "ATK:"+intToStr(e.player.EffectiveATK()), 40, y, render.ColorPeach)
	render.DrawText(screen, "DEF:"+intToStr(e.player.EffectiveDEF()), 160, y, render.ColorSky)
	y += 18
	render.DrawText(screen, "SPD:"+intToStr(s.SPD), 40, y, render.ColorGold)
	y += 24

	// Equipment (all 6 slots)
	e.drawGearLine(screen, "Wpn", e.player.Weapon, render.ColorPeach, 28, y)
	e.drawGearLine(screen, "Arm", e.player.Armor, render.ColorSky, 168, y)
	y += 14
	e.drawGearLine(screen, "Hlm", e.player.Helmet, render.ColorSky, 28, y)
	e.drawGearLine(screen, "Bts", e.player.Boots, render.ColorMint, 168, y)
	y += 14
	e.drawGearLine(screen, "Shd", e.player.Shield, render.ColorSky, 28, y)
	e.drawGearLine(screen, "Acc", e.player.Accessory, render.ColorLavender, 168, y)
	y += 20

	// Coins
	render.DrawText(screen, "Coins: "+intToStr(e.player.Coins)+"G", 28, y, render.ColorGold)

	// Quests completed
	done := 0
	for _, q := range e.player.Quests {
		if q.Done {
			done++
		}
	}
	render.DrawText(screen, "Quests: "+intToStr(done)+"/"+intToStr(len(e.player.Quests)), 160, y, render.ColorLavender)

	if (e.tick/20)%2 == 0 {
		render.DrawText(screen, "Z:Continue", 112, 264, render.ColorGray)
	}
}

func (e *EndingScreen) drawEnd(screen *ebiten.Image) {
	// "THE END" with decorative border
	render.DrawBox(screen, 40, 60, 240, 120, render.ColorBoxBG, render.ColorGold)

	// Twinkling stars
	stars := [][2]int{{60, 80}, {240, 90}, {70, 150}, {230, 76}, {160, 70}}
	for i, s := range stars {
		if (e.tick/8+i*3)%4 != 0 {
			render.DrawText(screen, "*", s[0], s[1], render.ColorGold)
		}
	}

	render.DrawText(screen, "THE END", 120, 104, render.ColorGold)

	render.DrawText(screen, "Thanks for playing!", 68, 136, render.ColorPink)

	// Prompt to return
	if e.phaseTick >= 30 && (e.tick/20)%2 == 0 {
		render.DrawText(screen, "Z:Title Screen", 88, 220, render.ColorWhite)
	}
}

func (e *EndingScreen) drawGearLine(screen *ebiten.Image, label string, slot *entity.Item, _ color.Color, x, y int) {
	if slot != nil {
		name := slot.DisplayName()
		if len(name) > 11 {
			name = name[:11]
		}
		nameClr := render.RarityColor(int(slot.Rarity))
		render.DrawText(screen, label+":"+name, x, y, nameClr)
	} else {
		render.DrawText(screen, label+":-", x, y, render.ColorDarkGray)
	}
}
