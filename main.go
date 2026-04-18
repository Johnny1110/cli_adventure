package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"cli_adventure/engine"
)

func main() {
	game := engine.NewGame()

	// Game Boy resolution (160x144) scaled up 4x to a visible window
	ebiten.SetWindowSize(160*4, 144*4)
	ebiten.SetWindowTitle("CLI Adventure")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
