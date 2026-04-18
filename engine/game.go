// Package engine contains the root game struct that implements ebiten.Game.
//
// THEORY: Ebitengine's game loop runs two methods in a cycle:
//   - Update() is called at a fixed 60 TPS (ticks per second) for game logic.
//     This is where input handling, state changes, and game math happen.
//   - Draw() is called each frame for rendering. It receives an *ebiten.Image
//     (the screen buffer) and should only draw — never mutate game state.
//   - Layout() returns the logical resolution. Ebitengine scales this up to the
//     OS window using nearest-neighbor filtering, giving us crisp pixel art.
//
// The Game struct acts as a state machine hub: it holds a "current screen" and
// delegates Update/Draw to it. Switching screens (menu → town → combat) is just
// swapping the currentScreen field.
package engine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"cli_adventure/screen"
)

const (
	// ScreenWidth is the logical horizontal resolution (2x Game Boy).
	ScreenWidth = 320
	// ScreenHeight is the logical vertical resolution (2x Game Boy).
	ScreenHeight = 288
)

// Game is the root struct implementing ebiten.Game.
// It owns the state machine and delegates to the active Screen.
type Game struct {
	currentScreen screen.Screen
	nextScreen    screen.Screen // set by screens to request a transition
}

// NewGame creates a new Game starting at the main menu.
func NewGame() *Game {
	g := &Game{}
	g.currentScreen = screen.NewMenuScreen(g)
	g.currentScreen.OnEnter()
	return g
}

// Update is called 60 times per second. It delegates to the current screen
// and handles screen transitions when a screen requests one.
func (g *Game) Update() error {
	// If a screen transition was requested, perform it
	if g.nextScreen != nil {
		g.currentScreen.OnExit()
		g.currentScreen = g.nextScreen
		g.nextScreen = nil
		g.currentScreen.OnEnter()
	}

	return g.currentScreen.Update()
}

// Draw renders the current screen onto the provided image.
// The image is 160x144 (our logical resolution); Ebitengine scales it up.
func (g *Game) Draw(img *ebiten.Image) {
	// Clear to a dark background (near-black, like a Game Boy screen)
	img.Fill(color.RGBA{R: 16, G: 16, B: 24, A: 255})
	g.currentScreen.Draw(img)
}

// Layout returns the logical screen dimensions. Ebitengine uses this to
// determine the internal rendering resolution, then scales it to the window.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}

// SwitchScreen requests a transition to a new screen.
// The transition happens at the start of the next Update() tick, ensuring
// we never switch mid-frame (which could cause rendering glitches).
func (g *Game) SwitchScreen(s screen.Screen) {
	g.nextScreen = s
}
