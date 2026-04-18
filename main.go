package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"cli_adventure/engine"
)

func main() {
	game := engine.NewGame()

	// 320x288 logical resolution (2x Game Boy) scaled 3x to a visible window.
	// This gives us a 960x864 window — comfortable on modern monitors.
	ebiten.SetWindowSize(320*3, 288*3)
	ebiten.SetWindowTitle("CLI Adventure")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
