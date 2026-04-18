package asset

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// Wild tileset tile IDs (separate namespace from town tiles)
const (
	WildGrass     = 0
	WildGrassDark = 1
	WildTree      = 2
	WildTreeDense = 3
	WildPath      = 4
	WildBush      = 5
	WildRock      = 6
	WildMushroom  = 7
	// Cave tiles
	CaveFloor     = 8
	CaveWall      = 9
	CaveRock      = 10
	CaveCrystal   = 11
	// Lair tiles
	LairFloor     = 12
	LairLava      = 13
	LairBone      = 14
	LairSkull     = 15
	// Special tiles
	ChestClosed   = 16
	ChestOpen     = 17
	FairyFlower   = 18 // secret fairy fountain tile
)

// GenerateWildTileset creates tilesets for forest, cave, and lair areas.
func GenerateWildTileset() *ebiten.Image {
	cols := 8
	rows := 3
	img := ebiten.NewImage(cols*16, rows*16)

	defs := map[int]tiledef{
		WildGrass:     {bg: c(60, 140, 60), pattern: "grass"},
		WildGrassDark: {bg: c(45, 110, 45), pattern: "grass"},
		WildTree:      {bg: c(45, 110, 45), pattern: "tree"},
		WildTreeDense: {bg: c(35, 90, 35), pattern: "tree_dense"},
		WildPath:      {bg: c(140, 120, 80), pattern: "plain"},
		WildBush:      {bg: c(60, 140, 60), pattern: "bush"},
		WildRock:      {bg: c(60, 140, 60), pattern: "rock"},
		WildMushroom:  {bg: c(60, 140, 60), pattern: "wild_mush"},
		CaveFloor:     {bg: c(70, 65, 75), pattern: "cave_floor"},
		CaveWall:      {bg: c(50, 45, 55), pattern: "cave_wall"},
		CaveRock:      {bg: c(70, 65, 75), pattern: "cave_rock"},
		CaveCrystal:   {bg: c(50, 45, 55), pattern: "crystal"},
		LairFloor:     {bg: c(60, 40, 35), pattern: "lair_floor"},
		LairLava:      {bg: c(200, 80, 30), pattern: "lava"},
		LairBone:      {bg: c(60, 40, 35), pattern: "bone"},
		LairSkull:     {bg: c(60, 40, 35), pattern: "skull"},
		ChestClosed:   {bg: c(60, 140, 60), pattern: "chest_closed"},
		ChestOpen:     {bg: c(60, 140, 60), pattern: "chest_open"},
		FairyFlower:   {bg: c(60, 140, 60), pattern: "fairy_flower"},
	}

	for id := 0; id < cols*rows; id++ {
		def, ok := defs[id]
		if !ok {
			continue
		}
		ox := (id % cols) * 16
		oy := (id / cols) * 16
		drawWildTile(img, ox, oy, def)
	}
	return img
}

func drawWildTile(img *ebiten.Image, ox, oy int, def tiledef) {
	// Base fill
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy+y, def.bg)
		}
	}

	switch def.pattern {
	case "grass":
		d := darken(def.bg, 20)
		l := lighten(def.bg, 15)
		spots := [][2]int{{2, 3}, {9, 6}, {5, 11}, {12, 2}, {1, 14}, {14, 10}, {7, 8}}
		for _, s := range spots {
			img.Set(ox+s[0], oy+s[1], d)
			img.Set(ox+s[0], oy+s[1]-1, l)
		}

	case "tree":
		trunk := c(90, 65, 40)
		canopy := c(35, 100, 40)
		canopyL := c(50, 130, 55)
		for y := 10; y < 16; y++ {
			img.Set(ox+7, oy+y, trunk)
			img.Set(ox+8, oy+y, trunk)
		}
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

	case "tree_dense":
		canopy := c(25, 80, 30)
		canopyL := c(40, 105, 40)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				dx := x - 8
				dy := y - 8
				if dx*dx+dy*dy < 60 {
					if (x+y)%3 == 0 {
						img.Set(ox+x, oy+y, canopyL)
					} else {
						img.Set(ox+x, oy+y, canopy)
					}
				}
			}
		}

	case "bush":
		bush := c(40, 120, 50)
		bushL := c(60, 150, 70)
		for y := 6; y < 14; y++ {
			for x := 3; x < 13; x++ {
				if (x+y)%4 == 0 {
					img.Set(ox+x, oy+y, bushL)
				} else {
					img.Set(ox+x, oy+y, bush)
				}
			}
		}

	case "rock":
		rock := c(130, 130, 140)
		rockD := c(100, 100, 110)
		for y := 5; y < 13; y++ {
			for x := 4; x < 12; x++ {
				dx := x - 8
				dy := y - 9
				if dx*dx+dy*dy < 18 {
					if y < 8 {
						img.Set(ox+x, oy+y, rock)
					} else {
						img.Set(ox+x, oy+y, rockD)
					}
				}
			}
		}

	case "wild_mush":
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "grass"})
		cap := c(200, 70, 70)
		stem := c(220, 210, 190)
		img.Set(ox+7, oy+10, stem)
		img.Set(ox+7, oy+11, stem)
		for x := 5; x < 10; x++ {
			img.Set(ox+x, oy+8, cap)
			img.Set(ox+x, oy+9, cap)
		}
		img.Set(ox+6, oy+8, c(255, 200, 200)) // spots
		img.Set(ox+8, oy+9, c(255, 200, 200))

	case "cave_floor":
		d := darken(def.bg, 10)
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x*7+y*13)%11 < 3 {
					img.Set(ox+x, oy+y, d)
				}
			}
		}

	case "cave_wall":
		l := lighten(def.bg, 15)
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy+14, l)
			img.Set(ox+x, oy+15, l)
			if (x*3)%5 < 2 {
				img.Set(ox+x, oy+13, l)
			}
		}

	case "cave_rock":
		rock := lighten(def.bg, 25)
		for y := 4; y < 13; y++ {
			for x := 4; x < 12; x++ {
				dx := x - 8
				dy := y - 8
				if dx*dx+dy*dy < 16 {
					img.Set(ox+x, oy+y, rock)
				}
			}
		}

	case "crystal":
		cry := color.RGBA{R: 120, G: 180, B: 255, A: 255}
		cryL := color.RGBA{R: 180, G: 220, B: 255, A: 255}
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "cave_wall"})
		// Simple crystal shape
		img.Set(ox+8, oy+4, cryL)
		img.Set(ox+7, oy+5, cry)
		img.Set(ox+8, oy+5, cryL)
		img.Set(ox+9, oy+5, cry)
		img.Set(ox+7, oy+6, cry)
		img.Set(ox+8, oy+6, cry)
		img.Set(ox+9, oy+6, cry)
		img.Set(ox+7, oy+7, cry)
		img.Set(ox+8, oy+7, cryL)
		img.Set(ox+9, oy+7, cry)
		img.Set(ox+8, oy+8, cry)

	case "lair_floor":
		d := darken(def.bg, 10)
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x*11+y*7)%13 < 3 {
					img.Set(ox+x, oy+y, d)
				}
			}
		}

	case "lava":
		bright := c(240, 120, 30)
		hot := c(255, 200, 60)
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x+y*2)%5 < 2 {
					img.Set(ox+x, oy+y, bright)
				}
				if (x*3+y)%7 == 0 {
					img.Set(ox+x, oy+y, hot)
				}
			}
		}

	case "bone":
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "lair_floor"})
		bone := c(220, 210, 190)
		img.Set(ox+6, oy+10, bone)
		img.Set(ox+7, oy+10, bone)
		img.Set(ox+8, oy+10, bone)
		img.Set(ox+9, oy+10, bone)
		img.Set(ox+10, oy+10, bone)

	case "skull":
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "lair_floor"})
		skull := c(220, 210, 190)
		skullD := c(180, 170, 150)
		for x := 5; x < 11; x++ {
			for y := 6; y < 11; y++ {
				img.Set(ox+x, oy+y, skull)
			}
		}
		img.Set(ox+6, oy+7, c(40, 30, 30))  // left eye
		img.Set(ox+9, oy+7, c(40, 30, 30))  // right eye
		img.Set(ox+7, oy+9, skullD)          // nose
		img.Set(ox+8, oy+9, skullD)

	case "chest_closed":
		// Draw grass base then a cute treasure chest
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "grass"})
		wood := c(160, 100, 40)
		woodD := c(120, 75, 30)
		gold := c(255, 220, 80)
		// Chest body
		for y := 7; y < 14; y++ {
			for x := 4; x < 12; x++ {
				if y < 10 {
					img.Set(ox+x, oy+y, wood)
				} else {
					img.Set(ox+x, oy+y, woodD)
				}
			}
		}
		// Gold clasp
		img.Set(ox+7, oy+10, gold)
		img.Set(ox+8, oy+10, gold)
		// Gold trim on lid
		for x := 4; x < 12; x++ {
			img.Set(ox+x, oy+7, gold)
		}

	case "chest_open":
		// Open chest (empty)
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "grass"})
		woodD := c(120, 75, 30)
		gold := c(255, 220, 80)
		// Chest body (lower half, lid is open)
		for y := 10; y < 14; y++ {
			for x := 4; x < 12; x++ {
				img.Set(ox+x, oy+y, woodD)
			}
		}
		// Open lid above (tilted back)
		for x := 4; x < 12; x++ {
			img.Set(ox+x, oy+7, gold)
			img.Set(ox+x, oy+8, c(160, 100, 40))
		}

	case "fairy_flower":
		// Magical glowing flowers on grass
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "grass"})
		pink := c(255, 150, 200)
		glow := c(255, 200, 255)
		// Cluster of glowing flowers
		flowers := [][2]int{{4, 8}, {8, 6}, {11, 10}, {6, 12}, {10, 7}}
		for _, f := range flowers {
			img.Set(ox+f[0], oy+f[1], pink)
			img.Set(ox+f[0]+1, oy+f[1], glow)
			img.Set(ox+f[0], oy+f[1]+1, glow)
			img.Set(ox+f[0], oy+f[1]-1, c(100, 200, 100)) // stem
		}
	}
}
