// Package asset provides sprite sheet loading and placeholder sprite generation.
//
// THEORY — Why generate sprites in code?
// During development, we don't want to be blocked on art assets. By defining
// sprites as color grids in code, we can iterate on gameplay immediately.
// The sprites here are "programmer art" — functional placeholders that clearly
// show what each entity is. When real pixel-art PNGs are ready, we swap them
// in through the SpriteSheet loader without changing any game logic.
//
// Each sprite is defined as a 16x16 grid of palette color indices.
// A value of 0 means transparent. The Generate* functions bake these grids
// into *ebiten.Image objects that the rest of the engine can draw.
package asset

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"cli_adventure/render"
)

// Palette indices used in sprite definitions.
// 0 is always transparent.
var spritePalette = []color.RGBA{
	{0, 0, 0, 0},           // 0: transparent
	{190, 195, 210, 255},    // 1: silver (armor light)
	{140, 145, 160, 255},    // 2: silver dark (armor shadow)
	{100, 105, 120, 255},    // 3: steel dark
	{80, 140, 220, 255},     // 4: blue (cape/accent)
	{50, 100, 180, 255},     // 5: blue dark
	{255, 220, 180, 255},    // 6: skin
	{220, 185, 145, 255},    // 7: skin shadow
	{255, 220, 100, 255},    // 8: gold
	{230, 235, 245, 255},    // 9: white (blade/highlight)
	{40, 40, 55, 255},       // 10: near-black (outline/eyes)
	{150, 110, 70, 255},     // 11: brown light
	{110, 80, 50, 255},      // 12: brown dark
	{160, 100, 200, 255},    // 13: purple light
	{120, 70, 170, 255},     // 14: purple dark
	{100, 200, 130, 255},    // 15: green
	{70, 160, 100, 255},     // 16: green dark
	{255, 150, 180, 255},    // 17: pink
	{200, 60, 80, 255},      // 18: red
}

// Knight sprite — 16x16 side-view chibi: armored figure with sword & shield
// Big head (chibi proportions), stocky body, blue cape, silver armor
var knightGrid = [16][16]byte{
	{0, 0, 0, 0, 0, 0, 10, 10, 10, 10, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 10, 2, 1, 1, 2, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 1, 1, 1, 1, 1, 1, 10, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 1, 1, 1, 1, 1, 1, 10, 4, 0, 0, 0},
	{0, 0, 0, 0, 10, 2, 6, 6, 6, 6, 2, 10, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 10, 10, 6, 6, 10, 10, 10, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 10, 7, 7, 7, 10, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 5, 10, 2, 1, 1, 2, 10, 9, 0, 0, 0, 0},
	{0, 0, 0, 5, 4, 10, 1, 1, 1, 1, 10, 9, 0, 0, 0, 0},
	{0, 0, 0, 5, 4, 10, 1, 4, 4, 1, 10, 9, 0, 0, 0, 0},
	{0, 0, 0, 0, 5, 10, 1, 4, 8, 4, 1, 10, 9, 0, 0, 0},
	{0, 0, 0, 0, 0, 10, 1, 4, 4, 1, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 10, 2, 8, 8, 2, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 10, 2, 2, 2, 2, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 12, 10, 0, 10, 12, 10, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 12, 10, 0, 10, 12, 10, 0, 0, 0, 0},
}

// Mage sprite — 16x16 side-view chibi: robed figure with pointy hat & staff
var mageGrid = [16][16]byte{
	{0, 0, 0, 0, 0, 0, 0, 14, 0, 0, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 0, 14, 13, 14, 0, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 14, 13, 13, 13, 14, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 14, 13, 13, 8, 13, 13, 14, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 14, 14, 14, 14, 14, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 6, 6, 6, 6, 6, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 10, 10, 6, 6, 10, 10, 0, 0, 0, 0, 0},
	{0, 0, 11, 0, 0, 10, 7, 7, 7, 10, 0, 0, 0, 0, 0, 0},
	{0, 0, 11, 0, 10, 14, 13, 13, 13, 14, 10, 0, 0, 0, 0, 0},
	{0, 0, 11, 0, 10, 13, 13, 13, 13, 13, 10, 0, 0, 0, 0, 0},
	{0, 0, 11, 0, 10, 13, 14, 14, 14, 13, 10, 0, 0, 0, 0, 0},
	{0, 0, 11, 0, 0, 10, 13, 13, 13, 10, 0, 0, 0, 0, 0, 0},
	{0, 0, 13, 0, 0, 10, 14, 14, 14, 10, 0, 0, 0, 0, 0, 0},
	{0, 0, 13, 0, 0, 0, 10, 10, 10, 0, 0, 0, 0, 0, 0, 0},
	{0, 0, 8, 0, 0, 10, 12, 10, 10, 12, 10, 0, 0, 0, 0, 0},
	{0, 8, 8, 8, 0, 10, 12, 10, 10, 12, 10, 0, 0, 0, 0, 0},
}

// Archer sprite — 16x16 side-view chibi: slim figure with hood & bow
var archerGrid = [16][16]byte{
	{0, 0, 0, 0, 0, 0, 10, 16, 10, 0, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 10, 15, 15, 15, 10, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 15, 15, 15, 15, 15, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 16, 15, 15, 15, 16, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 6, 6, 6, 6, 6, 10, 0, 0, 0, 0, 0, },
	{0, 0, 0, 0, 10, 10, 10, 6, 6, 10, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 10, 7, 7, 7, 10, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 16, 15, 15, 15, 16, 10, 0, 11, 0, 0, 0},
	{0, 0, 0, 0, 10, 15, 15, 15, 15, 15, 10, 10, 0, 10, 0, 0},
	{0, 0, 0, 0, 10, 15, 16, 16, 16, 15, 10, 0, 0, 0, 10, 0},
	{0, 0, 0, 0, 0, 10, 15, 15, 15, 10, 0, 0, 0, 0, 10, 0},
	{0, 0, 0, 0, 0, 10, 16, 16, 16, 10, 0, 0, 0, 10, 0, 0},
	{0, 0, 0, 0, 0, 0, 10, 10, 10, 0, 0, 0, 10, 0, 0, 0},
	{0, 0, 0, 0, 0, 10, 15, 0, 15, 10, 0, 11, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 11, 10, 0, 10, 11, 10, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 10, 12, 10, 0, 10, 12, 10, 0, 0, 0, 0, 0},
}

// GenerateCharSprites creates sprite sheet images for all 3 classes.
// Returns a map of class -> SpriteSheet with 2 frames (idle animation).
func GenerateCharSprites() map[int]*render.SpriteSheet {
	grids := map[int][16][16]byte{
		0: knightGrid,
		1: mageGrid,
		2: archerGrid,
	}

	result := map[int]*render.SpriteSheet{}
	for classID, grid := range grids {
		// Create a 2-frame sheet (32x16): frame 0 = normal, frame 1 = shifted 1px up (idle bob)
		img := ebiten.NewImage(32, 16)
		// Frame 0: normal
		drawGrid(img, grid, 0, 0)
		// Frame 1: shifted 1px up (simple idle animation)
		drawGrid(img, grid, 16, -1)
		result[classID] = render.NewSpriteSheet(img, 16, 16)
	}
	return result
}

// drawGrid renders a 16x16 color grid onto an image at the given offset.
func drawGrid(dst *ebiten.Image, grid [16][16]byte, offsetX, offsetY int) {
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			idx := grid[y][x]
			if idx == 0 {
				continue // transparent
			}
			if int(idx) >= len(spritePalette) {
				continue
			}
			px := offsetX + x
			py := offsetY + y
			if px >= 0 && py >= 0 {
				dst.Set(px, py, spritePalette[idx])
			}
		}
	}
}
