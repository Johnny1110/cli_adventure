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

	// Snow biome tiles
	SnowGround    = 19
	SnowPath      = 20
	SnowPine      = 21
	SnowRock      = 22
	IceFloor      = 23
	IceWall       = 24

	// Swamp biome tiles
	SwampGrass    = 25
	SwampWater    = 26
	SwampTree     = 27
	SwampMud      = 28

	// Volcano tiles
	VolcFloor     = 29
	VolcWall      = 30
	VolcLava      = 31

	// Desert biome tiles
	SandFloor     = 32
	SandDune      = 33
	Cactus        = 34
	SandRock      = 35

	// Ruins / Temple tiles
	RuinsFloor    = 36
	RuinsWall     = 37
)

// GenerateWildTileset creates tilesets for forest, cave, and lair areas.
func GenerateWildTileset() *ebiten.Image {
	cols := 8
	rows := 5 // expanded from 3 to fit snow/swamp/desert/ruins biomes
	img := ebiten.NewImage(cols*16, rows*16)

	defs := map[int]tiledef{
		// Forest
		WildGrass:     {bg: c(60, 140, 60), pattern: "grass"},
		WildGrassDark: {bg: c(45, 110, 45), pattern: "grass"},
		WildTree:      {bg: c(45, 110, 45), pattern: "tree"},
		WildTreeDense: {bg: c(35, 90, 35), pattern: "tree_dense"},
		WildPath:      {bg: c(140, 120, 80), pattern: "plain"},
		WildBush:      {bg: c(60, 140, 60), pattern: "bush"},
		WildRock:      {bg: c(60, 140, 60), pattern: "rock"},
		WildMushroom:  {bg: c(60, 140, 60), pattern: "wild_mush"},
		// Cave
		CaveFloor:     {bg: c(70, 65, 75), pattern: "cave_floor"},
		CaveWall:      {bg: c(50, 45, 55), pattern: "cave_wall"},
		CaveRock:      {bg: c(70, 65, 75), pattern: "cave_rock"},
		CaveCrystal:   {bg: c(50, 45, 55), pattern: "crystal"},
		// Lair
		LairFloor:     {bg: c(60, 40, 35), pattern: "lair_floor"},
		LairLava:      {bg: c(200, 80, 30), pattern: "lava"},
		LairBone:      {bg: c(60, 40, 35), pattern: "bone"},
		LairSkull:     {bg: c(60, 40, 35), pattern: "skull"},
		// Special
		ChestClosed:   {bg: c(60, 140, 60), pattern: "chest_closed"},
		ChestOpen:     {bg: c(60, 140, 60), pattern: "chest_open"},
		FairyFlower:   {bg: c(60, 140, 60), pattern: "fairy_flower"},
		// Snow biome
		SnowGround:    {bg: c(220, 225, 235), pattern: "snow_ground"},
		SnowPath:      {bg: c(190, 195, 210), pattern: "snow_path"},
		SnowPine:      {bg: c(220, 225, 235), pattern: "snow_pine"},
		SnowRock:      {bg: c(220, 225, 235), pattern: "snow_rock"},
		IceFloor:      {bg: c(180, 210, 240), pattern: "ice_floor"},
		IceWall:       {bg: c(140, 170, 200), pattern: "ice_wall"},
		// Swamp biome
		SwampGrass:    {bg: c(70, 100, 55), pattern: "swamp_grass"},
		SwampWater:    {bg: c(50, 80, 60), pattern: "swamp_water"},
		SwampTree:     {bg: c(70, 100, 55), pattern: "swamp_tree"},
		SwampMud:      {bg: c(90, 75, 50), pattern: "swamp_mud"},
		// Volcano
		VolcFloor:     {bg: c(80, 50, 40), pattern: "volc_floor"},
		VolcWall:      {bg: c(55, 35, 30), pattern: "volc_wall"},
		VolcLava:      {bg: c(220, 90, 20), pattern: "lava"},
		// Desert biome
		SandFloor:     {bg: c(220, 195, 140), pattern: "sand_floor"},
		SandDune:      {bg: c(200, 175, 120), pattern: "sand_dune"},
		Cactus:        {bg: c(220, 195, 140), pattern: "cactus"},
		SandRock:      {bg: c(220, 195, 140), pattern: "sand_rock"},
		// Ruins / Temple
		RuinsFloor:    {bg: c(170, 155, 130), pattern: "ruins_floor"},
		RuinsWall:     {bg: c(130, 120, 100), pattern: "ruins_wall"},
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
		flowers := [][2]int{{4, 8}, {8, 6}, {11, 10}, {6, 12}, {10, 7}}
		for _, f := range flowers {
			img.Set(ox+f[0], oy+f[1], pink)
			img.Set(ox+f[0]+1, oy+f[1], glow)
			img.Set(ox+f[0], oy+f[1]+1, glow)
			img.Set(ox+f[0], oy+f[1]-1, c(100, 200, 100))
		}

	// ─── Snow biome ───
	case "snow_ground":
		l := lighten(def.bg, 10)
		d := darken(def.bg, 15)
		// Scattered snow texture
		spots := [][2]int{{3, 2}, {10, 5}, {6, 10}, {13, 3}, {1, 13}, {14, 11}, {8, 7}}
		for _, s := range spots {
			img.Set(ox+s[0], oy+s[1], l)
			img.Set(ox+s[0]+1, oy+s[1], d)
		}

	case "snow_path":
		d := darken(def.bg, 12)
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x*5+y*7)%9 < 2 {
					img.Set(ox+x, oy+y, d)
				}
			}
		}

	case "snow_pine":
		// Pine tree with snow on branches
		trunk := c(90, 65, 40)
		pine := c(30, 80, 45)
		pineL := c(40, 100, 55)
		snow := c(240, 245, 250)
		for y := 11; y < 16; y++ {
			img.Set(ox+7, oy+y, trunk)
			img.Set(ox+8, oy+y, trunk)
		}
		// Triangular canopy
		for layer := 0; layer < 3; layer++ {
			baseY := 2 + layer*3
			width := 2 + layer*2
			for y := baseY; y < baseY+3; y++ {
				spread := width - (y - baseY)
				for x := 8 - spread; x <= 7 + spread; x++ {
					if x >= 0 && x < 16 {
						if (x+y)%3 == 0 {
							img.Set(ox+x, oy+y, pineL)
						} else {
							img.Set(ox+x, oy+y, pine)
						}
					}
				}
			}
			// Snow on top edge of each layer
			img.Set(ox+7, oy+baseY, snow)
			img.Set(ox+8, oy+baseY, snow)
		}

	case "snow_rock":
		rock := c(170, 175, 185)
		rockD := c(140, 145, 155)
		snowCap := c(240, 245, 250)
		for y := 6; y < 13; y++ {
			for x := 4; x < 12; x++ {
				dx := x - 8
				dy := y - 9
				if dx*dx+dy*dy < 18 {
					if y < 9 {
						img.Set(ox+x, oy+y, rock)
					} else {
						img.Set(ox+x, oy+y, rockD)
					}
				}
			}
		}
		// Snow cap
		for x := 5; x < 11; x++ {
			img.Set(ox+x, oy+5, snowCap)
			img.Set(ox+x, oy+6, snowCap)
		}

	case "ice_floor":
		l := lighten(def.bg, 15)
		// Icy reflection streaks
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x*3+y*11)%13 < 2 {
					img.Set(ox+x, oy+y, l)
				}
			}
		}

	case "ice_wall":
		l := lighten(def.bg, 20)
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy+14, l)
			img.Set(ox+x, oy+15, l)
			if (x*3)%5 < 2 {
				img.Set(ox+x, oy+13, l)
			}
		}
		// Ice crystals along the wall
		img.Set(ox+4, oy+12, c(200, 230, 255))
		img.Set(ox+11, oy+11, c(200, 230, 255))

	// ─── Swamp biome ───
	case "swamp_grass":
		d := darken(def.bg, 15)
		l := lighten(def.bg, 10)
		// Dark, soggy grass
		spots := [][2]int{{2, 4}, {9, 7}, {5, 12}, {13, 2}, {1, 9}, {14, 13}, {7, 1}}
		for _, s := range spots {
			img.Set(ox+s[0], oy+s[1], d)
			img.Set(ox+s[0], oy+s[1]-1, l)
		}

	case "swamp_water":
		l := lighten(def.bg, 20)
		d := darken(def.bg, 15)
		// Murky water with ripples
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x*7+y*3)%11 < 2 {
					img.Set(ox+x, oy+y, l)
				} else if (x*3+y*5)%9 < 2 {
					img.Set(ox+x, oy+y, d)
				}
			}
		}

	case "swamp_tree":
		// Dead/gnarled tree with hanging moss
		trunk := c(80, 60, 40)
		trunkD := c(60, 45, 30)
		moss := c(90, 110, 50)
		for y := 4; y < 16; y++ {
			img.Set(ox+7, oy+y, trunk)
			img.Set(ox+8, oy+y, trunkD)
		}
		// Bare branches
		img.Set(ox+5, oy+4, trunk)
		img.Set(ox+6, oy+3, trunk)
		img.Set(ox+10, oy+4, trunk)
		img.Set(ox+11, oy+3, trunk)
		// Hanging moss
		img.Set(ox+5, oy+5, moss)
		img.Set(ox+5, oy+6, moss)
		img.Set(ox+11, oy+4, moss)
		img.Set(ox+11, oy+5, moss)

	case "swamp_mud":
		d := darken(def.bg, 15)
		l := lighten(def.bg, 10)
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x*9+y*5)%7 < 2 {
					img.Set(ox+x, oy+y, d)
				} else if (x*3+y*11)%13 < 2 {
					img.Set(ox+x, oy+y, l)
				}
			}
		}

	// ─── Volcano ───
	case "volc_floor":
		d := darken(def.bg, 12)
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x*11+y*7)%13 < 3 {
					img.Set(ox+x, oy+y, d)
				}
			}
		}

	case "volc_wall":
		l := lighten(def.bg, 15)
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy+14, l)
			img.Set(ox+x, oy+15, l)
			if (x*3)%5 < 2 {
				img.Set(ox+x, oy+13, l)
			}
		}
		// Glow from heat
		img.Set(ox+6, oy+12, c(180, 60, 20))
		img.Set(ox+12, oy+11, c(180, 60, 20))

	// ─── Desert biome ───
	case "sand_floor":
		l := lighten(def.bg, 10)
		d := darken(def.bg, 10)
		// Wind-swept sand texture
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x+y*2)%7 < 2 {
					img.Set(ox+x, oy+y, l)
				} else if (x*3+y)%11 < 2 {
					img.Set(ox+x, oy+y, d)
				}
			}
		}

	case "sand_dune":
		// Tall dune (solid obstacle)
		d := darken(def.bg, 20)
		l := lighten(def.bg, 15)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if y < 8 {
					img.Set(ox+x, oy+y, l) // sun-lit side
				} else {
					img.Set(ox+x, oy+y, d) // shadow side
				}
			}
		}

	case "cactus":
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "sand_floor"})
		green := c(50, 130, 50)
		greenD := c(40, 100, 40)
		// Main trunk
		for y := 5; y < 14; y++ {
			img.Set(ox+7, oy+y, green)
			img.Set(ox+8, oy+y, greenD)
		}
		// Left arm
		img.Set(ox+5, oy+7, green)
		img.Set(ox+6, oy+7, green)
		img.Set(ox+5, oy+8, green)
		img.Set(ox+5, oy+9, green)
		// Right arm
		img.Set(ox+9, oy+9, green)
		img.Set(ox+10, oy+9, green)
		img.Set(ox+10, oy+10, green)
		img.Set(ox+10, oy+11, green)

	case "sand_rock":
		drawWildTile(img, ox, oy, tiledef{bg: def.bg, pattern: "sand_floor"})
		rock := c(180, 160, 120)
		rockD := c(150, 135, 100)
		for y := 6; y < 13; y++ {
			for x := 4; x < 12; x++ {
				dx := x - 8
				dy := y - 9
				if dx*dx+dy*dy < 18 {
					if y < 9 {
						img.Set(ox+x, oy+y, rock)
					} else {
						img.Set(ox+x, oy+y, rockD)
					}
				}
			}
		}

	// ─── Ruins / Temple ───
	case "ruins_floor":
		d := darken(def.bg, 12)
		l := lighten(def.bg, 8)
		// Cracked stone floor
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if (x*7+y*3)%11 < 2 {
					img.Set(ox+x, oy+y, d)
				}
			}
		}
		// Grid lines suggesting tiled floor
		for x := 0; x < 16; x++ {
			img.Set(ox+x, oy+0, l)
			img.Set(ox+x, oy+8, l)
		}
		for y := 0; y < 16; y++ {
			img.Set(ox+0, oy+y, l)
			img.Set(ox+8, oy+y, l)
		}

	case "ruins_wall":
		l := lighten(def.bg, 20)
		d := darken(def.bg, 15)
		// Brick pattern
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				row := y / 4
				shift := 0
				if row%2 == 1 {
					shift = 4
				}
				bx := (x + shift) % 8
				if bx == 0 || y%4 == 0 {
					img.Set(ox+x, oy+y, d) // mortar
				} else {
					img.Set(ox+x, oy+y, l) // brick face
				}
			}
		}
	}
}
