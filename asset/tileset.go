// asset/tileset.go — Programmatic tileset generation for placeholder art.
//
// THEORY: Each tile is 16x16 pixels. We pack them into a tileset image
// arranged in columns. Tile ID 0 is the top-left tile, ID 1 is next, etc.
// When real art is ready, this entire file gets replaced by a PNG loader.
//
// Tile IDs for the town tileset:
//   0 = grass           4 = wall_top       8 = door          12 = flowers_pink
//   1 = path_stone      5 = wall_mid       9 = water         13 = flowers_blue
//   2 = path_dirt       6 = roof_left     10 = bridge        14 = sign
//   3 = wall_bottom     7 = roof_right    11 = tree          15 = well
//  16 = wormhole (animated portal — multiplayer entrypoint)
package asset

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	TileGrass      = 0
	TilePath       = 1
	TileDirt       = 2
	TileWallBot    = 3
	TileWallTop    = 4
	TileWallMid    = 5
	TileRoofL      = 6
	TileRoofR      = 7
	TileDoor       = 8
	TileWater      = 9
	TileBridge     = 10
	TileTree       = 11
	TileFlowerPink = 12
	TileFlowerBlue = 13
	TileSign       = 14
	TileWell       = 15
	TileWormhole   = 16 // swirling portal — interactable
)

// townTileColors defines the dominant color for each placeholder tile.
// Real tiles would be detailed pixel art; these are solid/patterned blocks.
var townTileColors = map[int]tiledef{
	TileGrass:      {bg: c(80, 160, 80), pattern: "grass"},
	TilePath:       {bg: c(180, 170, 150), pattern: "stone"},
	TileDirt:       {bg: c(160, 130, 90), pattern: "plain"},
	TileWallBot:    {bg: c(140, 120, 100), pattern: "brick"},
	TileWallTop:    {bg: c(160, 140, 120), pattern: "brick"},
	TileWallMid:    {bg: c(150, 130, 110), pattern: "brick"},
	TileRoofL:      {bg: c(180, 80, 80), pattern: "roof"},
	TileRoofR:      {bg: c(160, 70, 70), pattern: "roof"},
	TileDoor:       {bg: c(120, 80, 50), pattern: "door"},
	TileWater:      {bg: c(80, 140, 210), pattern: "water"},
	TileBridge:     {bg: c(150, 120, 80), pattern: "bridge"},
	TileTree:       {bg: c(80, 160, 80), pattern: "tree"},
	TileFlowerPink: {bg: c(80, 160, 80), pattern: "flower_pink"},
	TileFlowerBlue: {bg: c(80, 160, 80), pattern: "flower_blue"},
	TileSign:       {bg: c(80, 160, 80), pattern: "sign"},
	TileWell:       {bg: c(130, 130, 140), pattern: "well"},
	TileWormhole:   {bg: c(80, 160, 80), pattern: "wormhole"},
}

type tiledef struct {
	bg      color.RGBA
	pattern string
}

func c(r, g, b uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// GenerateTownTileset creates a placeholder tileset image.
// Layout: 8 columns x 3 rows (24 cells) — only 17 IDs are used, the rest
// stay transparent and are never referenced by a map cell.
func GenerateTownTileset() *ebiten.Image {
	cols := 8
	rows := 3
	img := ebiten.NewImage(cols*16, rows*16)

	for id := 0; id < cols*rows; id++ {
		def, ok := townTileColors[id]
		if !ok {
			continue
		}
		ox := (id % cols) * 16
		oy := (id / cols) * 16
		drawTile(img, ox, oy, def)
	}
	return img
}

func drawTile(img *ebiten.Image, ox, oy int, def tiledef) {
	// Fill background
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy+y, def.bg)
		}
	}

	// Add pattern detail
	switch def.pattern {
	case "grass":
		// Scattered darker grass blades
		grassDark := darken(def.bg, 30)
		grassLight := lighten(def.bg, 20)
		spots := [][2]int{{3, 2}, {10, 5}, {6, 10}, {13, 3}, {1, 13}, {8, 14}, {14, 11}}
		for _, s := range spots {
			img.Set(ox+s[0], oy+s[1], grassDark)
			img.Set(ox+s[0], oy+s[1]-1, grassLight)
		}

	case "stone":
		// Grid pattern for stone path
		line := darken(def.bg, 25)
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy+7, line)
			img.Set(ox+x, oy+15, line)
		}
		for y := 0; y < 16; y++ {
			img.Set(ox+7, oy+y, line)
			img.Set(ox+15, oy+y, line)
		}

	case "brick":
		// Brick pattern
		line := darken(def.bg, 30)
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy+3, line)
			img.Set(ox+x, oy+7, line)
			img.Set(ox+x, oy+11, line)
			img.Set(ox+x, oy+15, line)
		}
		for y := 0; y < 4; y++ {
			img.Set(ox+7, oy+y, line)
			img.Set(ox+15, oy+y+4, line)
			img.Set(ox+7, oy+y+8, line)
			img.Set(ox+15, oy+y+12, line)
		}

	case "roof":
		// Diagonal shingle pattern
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if (x+y)%4 == 0 {
					img.Set(ox+x, oy+y, darken(def.bg, 20))
				}
			}
		}

	case "door":
		// Door with handle
		frame := darken(def.bg, 30)
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy, frame)
			img.Set(ox+x, oy+15, frame)
		}
		for y := 0; y < 16; y++ {
			img.Set(ox, oy+y, frame)
			img.Set(ox+15, oy+y, frame)
		}
		// Handle
		img.Set(ox+11, oy+8, c(220, 200, 80))
		img.Set(ox+11, oy+9, c(220, 200, 80))

	case "water":
		// Wavy highlights
		light := lighten(def.bg, 30)
		for x := 0; x < 16; x++ {
			wy := (x / 3) % 2
			img.Set(ox+x, oy+4+wy, light)
			img.Set(ox+x, oy+11+wy, light)
		}

	case "bridge":
		// Horizontal planks
		line := darken(def.bg, 30)
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy, line)
			img.Set(ox+x, oy+5, line)
			img.Set(ox+x, oy+10, line)
			img.Set(ox+x, oy+15, line)
		}
		// Railings
		for y := 0; y < 16; y++ {
			img.Set(ox, oy+y, c(100, 80, 60))
			img.Set(ox+15, oy+y, c(100, 80, 60))
		}

	case "tree":
		// Green canopy on grass background
		trunk := c(120, 80, 50)
		canopy := c(50, 130, 60)
		canopyL := c(70, 160, 80)
		// Trunk
		for y := 10; y < 16; y++ {
			img.Set(ox+7, oy+y, trunk)
			img.Set(ox+8, oy+y, trunk)
		}
		// Canopy (round blob)
		for y := 1; y < 11; y++ {
			for x := 2; x < 14; x++ {
				dx := x - 8
				dy := y - 5
				if dx*dx+dy*dy < 28 {
					if (x+y)%3 == 0 {
						img.Set(ox+x, oy+y, canopyL)
					} else {
						img.Set(ox+x, oy+y, canopy)
					}
				}
			}
		}

	case "flower_pink":
		// Grass with pink flowers
		drawTile(img, ox, oy, tiledef{bg: def.bg, pattern: "grass"})
		pink := c(255, 150, 180)
		pinkD := c(230, 120, 150)
		spots := [][2]int{{4, 6}, {11, 3}, {7, 12}}
		for _, s := range spots {
			img.Set(ox+s[0], oy+s[1], pink)
			img.Set(ox+s[0]+1, oy+s[1], pinkD)
			img.Set(ox+s[0], oy+s[1]+1, pinkD)
		}

	case "flower_blue":
		drawTile(img, ox, oy, tiledef{bg: def.bg, pattern: "grass"})
		blue := c(130, 180, 255)
		blueD := c(100, 150, 230)
		spots := [][2]int{{3, 4}, {10, 8}, {6, 13}}
		for _, s := range spots {
			img.Set(ox+s[0], oy+s[1], blue)
			img.Set(ox+s[0]+1, oy+s[1], blueD)
			img.Set(ox+s[0], oy+s[1]+1, blueD)
		}

	case "sign":
		// Grass background + small signpost
		drawTile(img, ox, oy, tiledef{bg: def.bg, pattern: "grass"})
		wood := c(140, 100, 60)
		board := c(200, 180, 130)
		// Post
		img.Set(ox+8, oy+10, wood)
		img.Set(ox+8, oy+11, wood)
		img.Set(ox+8, oy+12, wood)
		img.Set(ox+8, oy+13, wood)
		// Board
		for x := 5; x < 12; x++ {
			for y := 6; y < 10; y++ {
				img.Set(ox+x, oy+y, board)
			}
		}
		// Board border
		for x := 5; x < 12; x++ {
			img.Set(ox+x, oy+6, wood)
			img.Set(ox+x, oy+9, wood)
		}

	case "wormhole":
		// Grass background + a swirling portal.
		// We draw the portal as concentric rings in shifting pastel hues so it
		// reads as "magical / interactive" even at 16x16.
		drawTile(img, ox, oy, tiledef{bg: def.bg, pattern: "grass"})
		outer := c(120, 80, 200) // deep violet
		mid := c(180, 130, 255)  // lavender
		inner := c(230, 200, 255) // near-white sparkle
		core := c(255, 240, 255)
		// Elliptical portal footprint (cx,cy) = tile center.
		cx, cy := 8, 8
		for y := 2; y < 14; y++ {
			for x := 2; x < 14; x++ {
				dx := x - cx
				dy := y - cy
				// r2 = scaled squared distance; y squeezed so the portal
				// reads as a tilted oval rather than a perfect circle.
				r2 := dx*dx*3 + dy*dy*4
				switch {
				case r2 < 18:
					img.Set(ox+x, oy+y, core)
				case r2 < 40:
					img.Set(ox+x, oy+y, inner)
				case r2 < 80:
					img.Set(ox+x, oy+y, mid)
				case r2 < 130:
					img.Set(ox+x, oy+y, outer)
				}
			}
		}

	case "well":
		// Stone well
		stone := c(150, 150, 160)
		stoneD := c(120, 120, 130)
		water := c(60, 120, 200)
		// Base circle (square approximation)
		for x := 3; x < 13; x++ {
			for y := 4; y < 14; y++ {
				img.Set(ox+x, oy+y, stone)
			}
		}
		// Inner water
		for x := 5; x < 11; x++ {
			for y := 6; y < 12; y++ {
				img.Set(ox+x, oy+y, water)
			}
		}
		// Border
		for x := 3; x < 13; x++ {
			img.Set(ox+x, oy+4, stoneD)
			img.Set(ox+x, oy+13, stoneD)
		}
		for y := 4; y < 14; y++ {
			img.Set(ox+3, oy+y, stoneD)
			img.Set(ox+12, oy+y, stoneD)
		}
	}
}

func darken(c color.RGBA, amount uint8) color.RGBA {
	r, g, b := c.R, c.G, c.B
	if r > amount { r -= amount } else { r = 0 }
	if g > amount { g -= amount } else { g = 0 }
	if b > amount { b -= amount } else { b = 0 }
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func lighten(c color.RGBA, amount uint8) color.RGBA {
	r, g, b := c.R, c.G, c.B
	if r+amount < 255 { r += amount } else { r = 255 }
	if g+amount < 255 { g += amount } else { g = 255 }
	if b+amount < 255 { b += amount } else { b = 255 }
	return color.RGBA{R: r, G: g, B: b, A: 255}
}
