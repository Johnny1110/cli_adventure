// data/maps.go — Static map definitions for town and other areas.
//
// THEORY — Data-driven maps:
// The map is defined as a 2D array of tile IDs. This is the simplest possible
// map format and it's trivially editable. Professional games use tools like
// Tiled (.tmx format), but for an MVP, hand-coded arrays are clearer and have
// zero external dependencies.
//
// The town is 20x18 tiles (320x288 pixels), roughly 2 screens of scrolling.
// It has:
//   - A central path leading to buildings
//   - A merchant shop (top-left area)
//   - An elder's house (top-right area)
//   - A well in the center square
//   - Flowers and trees for decoration
//   - An exit to the south (leads to Wild in Phase 3)
package data

import "cli_adventure/asset"

// Shorthand aliases for readability
const (
	G  = asset.TileGrass
	P  = asset.TilePath
	D  = asset.TileDirt
	WB = asset.TileWallBot
	WT = asset.TileWallTop
	WM = asset.TileWallMid
	RL = asset.TileRoofL
	RR = asset.TileRoofR
	DR = asset.TileDoor
	WA = asset.TileWater
	BR = asset.TileBridge
	TR = asset.TileTree
	FP = asset.TileFlowerPink
	FB = asset.TileFlowerBlue
	SI = asset.TileSign
	WE = asset.TileWell
)

// TownWidth and TownHeight in tiles.
const (
	TownWidth  = 20
	TownHeight = 18
)

// TownGround is the base terrain layer.
var TownGround = [TownHeight][TownWidth]int{
	//  0   1   2   3   4   5   6   7   8   9  10  11  12  13  14  15  16  17  18  19
	{TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR, TR}, // 0
	{TR, G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  G,  TR}, // 1
	{TR, G,  RL, RR, RL, RR, G,  G,  G,  G,  G,  G,  G,  G,  RL, RR, RL, RR, G,  TR}, // 2
	{TR, G,  WT, WT, WT, WT, G,  G,  FP, G,  G,  G,  FB, G,  WT, WT, WT, WT, G,  TR}, // 3
	{TR, G,  WM, WM, WM, WM, G,  G,  G,  P,  P,  P,  G,  G,  WM, WM, WM, WM, G,  TR}, // 4
	{TR, G,  WB, DR, WB, WB, G,  G,  G,  P,  G,  P,  G,  G,  WB, WB, DR, WB, G,  TR}, // 5
	{TR, G,  G,  P,  G,  G,  G,  G,  G,  P,  WE, P,  G,  G,  G,  G,  P,  G,  G,  TR}, // 6
	{TR, G,  G,  P,  G,  FP, G,  G,  G,  P,  P,  P,  G,  G,  FB, G,  P,  G,  G,  TR}, // 7
	{TR, G,  G,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  G,  G,  TR}, // 8  main road
	{TR, G,  G,  G,  G,  G,  G,  G,  G,  P,  SI, P,  G,  G,  G,  G,  G,  G,  G,  TR}, // 9
	{TR, G,  FP, G,  G,  G,  G,  FB, G,  P,  G,  P,  G,  FP, G,  G,  G,  FB, G,  TR}, // 10
	{TR, G,  G,  G,  G,  G,  G,  G,  G,  P,  G,  P,  G,  G,  G,  G,  G,  G,  G,  TR}, // 11
	{TR, WA, WA, WA, WA, G,  G,  G,  G,  P,  G,  P,  G,  G,  G,  G,  G,  G,  G,  TR}, // 12
	{TR, WA, WA, WA, WA, G,  G,  G,  G,  P,  G,  P,  G,  G,  G,  G,  G,  G,  G,  TR}, // 13
	{TR, G,  BR, BR, G,  G,  G,  G,  G,  P,  G,  P,  G,  G,  G,  TR, G,  G,  G,  TR}, // 14
	{TR, G,  G,  G,  G,  G,  G,  G,  G,  P,  G,  P,  G,  G,  G,  G,  G,  G,  G,  TR}, // 15
	{TR, G,  G,  G,  G,  G,  G,  G,  G,  P,  P,  P,  G,  G,  G,  G,  G,  G,  G,  TR}, // 16
	{TR, TR, TR, TR, TR, TR, TR, TR, TR, D,  D,  D,  TR, TR, TR, TR, TR, TR, TR, TR}, // 17  south exit
}

// TownOverlay is the overlay layer (drawn on top of entities). -1 = empty.
// Used for tree tops and roof peaks that should render over the player.
var TownOverlay = [TownHeight][TownWidth]int{
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
	{ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
}

// TownSolid defines which tiles are impassable.
// true = player can NOT walk here.
func TownSolid() [TownHeight][TownWidth]bool {
	var solid [TownHeight][TownWidth]bool
	for y := 0; y < TownHeight; y++ {
		for x := 0; x < TownWidth; x++ {
			t := TownGround[y][x]
			switch t {
			case TR, WA, WT, WM, WB, RL, RR, WE:
				solid[y][x] = true
			}
		}
	}
	// Doors are walkable (they trigger NPC interaction)
	solid[5][3] = false   // merchant door
	solid[5][16] = false  // elder door
	return solid
}

// NPCSpawn defines where NPCs are placed on the town map.
type NPCSpawn struct {
	Name string
	TileX, TileY int
	Role string // "merchant" or "elder"
}

// TownNPCs returns the NPC spawn points for the town.
func TownNPCs() []NPCSpawn {
	return []NPCSpawn{
		{Name: "Merchant", TileX: 3, TileY: 6, Role: "merchant"},
		{Name: "Elder", TileX: 16, TileY: 6, Role: "elder"},
	}
}

// TownExitTile is the tile position of the south exit (leads to Wild).
var TownExitY = 17
