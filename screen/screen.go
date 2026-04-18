// Package screen defines the Screen interface and game states.
//
// THEORY: Each screen (menu, town, combat, etc.) is a self-contained module
// that handles its own input, logic, and rendering. The root Game struct holds
// a reference to the "current" screen and delegates Update/Draw to it.
//
// This is the "parent delegates to child" pattern. The parent (Game) doesn't
// know what a MenuScreen or CombatScreen does internally — it just calls the
// interface methods. This means adding a new screen is just implementing the
// Screen interface and wiring up the transition.
//
// OnEnter/OnExit are lifecycle hooks: OnEnter is called when the screen becomes
// active (good for resetting state, starting music), OnExit when it's about to
// be replaced (good for cleanup).
package screen

import "github.com/hajimehoshi/ebiten/v2"

// GameState represents which screen the game is currently showing.
type GameState int

const (
	StateMenu GameState = iota
	StateTown
	StateWild
	StateCombat
)

// ScreenSwitcher is implemented by the root Game to allow screens
// to request transitions without importing the engine package (avoiding cycles).
type ScreenSwitcher interface {
	SwitchScreen(s Screen)
}

// Screen is the interface every game screen must implement.
type Screen interface {
	// Update handles input and game logic. Called 60 times per second.
	Update() error

	// Draw renders the screen onto the provided image (160x144).
	Draw(screen *ebiten.Image)

	// OnEnter is called when this screen becomes the active screen.
	OnEnter()

	// OnExit is called when this screen is about to be replaced.
	OnExit()
}
