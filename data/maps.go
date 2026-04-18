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
	WH = asset.TileWormhole
	SC = asset.TileSaveCrystal
)

// TownWidth and TownHeight in tiles.
const (
	TownWidth  = 20
	TownHeight = 18
)

// TownGround is the base terrain layer.
//
// THEORY — Town layout as narrative space:
// Building placement tells a story. The Merchant and Elder are near the
// top (the "important" part of town, visible first when entering from the
// south). The Home is tucked in the bottom-left near water (peaceful,
// residential). The Blacksmith is in the bottom-right (industrial, away
// from homes). The Inn is mid-right near the main road (traveler-friendly,
// easy to find). Each building's position reinforces its role.
var TownGround = [TownHeight][TownWidth]int{
	//  0   1   2   3   4   5   6   7   8   9  10  11  12  13  14  15  16  17  18  19
	{TR, TR, TR, TR, TR, TR, TR, TR, TR, D,  D,  D,  TR, TR, TR, TR, TR, TR, TR, TR}, // 0  north exit
	{TR, G,  G,  G,  G,  G,  G,  G,  G,  P,  P,  P,  G,  G,  G,  G,  G,  G,  G,  TR}, // 1
	{TR, G,  RL, RR, RL, RR, G,  G,  G,  P,  G,  P,  G,  G,  RL, RR, RL, RR, G,  TR}, // 2
	{TR, G,  WT, WT, WT, WT, G,  G,  FP, P,  G,  P,  FB, G,  WT, WT, WT, WT, G,  TR}, // 3
	{TR, G,  WM, WM, WM, WM, G,  G,  G,  P,  P,  P,  G,  G,  WM, WM, WM, WM, G,  TR}, // 4
	{TR, G,  WB, DR, WB, WB, G,  G,  G,  P,  G,  P,  G,  G,  WB, WB, DR, WB, G,  TR}, // 5
	{TR, G,  G,  P,  G,  G,  G,  G,  G,  P,  WE, P,  G,  G,  G,  G,  P,  G,  G,  TR}, // 6
	{TR, G,  G,  P,  G,  FP, G,  G,  G,  P,  P,  P,  G,  G,  FB, G,  P,  G,  G,  TR}, // 7
	{D,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  P,  D},  // 8  main road + west/east exits
	{TR, G,  G,  G,  G,  G,  G,  G,  G,  P,  SI, P,  SC, G,  G,  G,  G,  G,  G,  TR}, // 9  save crystal at (12,9)
	{TR, G,  FP, G,  G,  G,  G,  FB, G,  P,  G,  P,  G,  WH, G,  RL, RR, G,  G,  TR}, // 10 — wormhole(13,10), inn roof
	{TR, G,  G,  G,  G,  G,  G,  G,  G,  P,  G,  P,  G,  RL, RR, WB, DR, G,  G,  TR}, // 11 smith roof, inn door(16,11)
	{TR, WA, WA, WA, WA, RL, RR, G,  G,  P,  G,  P,  G,  WT, WT, G,  P,  G,  G,  TR}, // 12 smith walls
	{TR, WA, WA, WA, WA, WT, WT, G,  G,  P,  G,  P,  G,  WB, DR, G,  P,  G,  G,  TR}, // 13 smith door(14,13)
	{TR, G,  BR, BR, G,  WB, DR, G,  G,  P,  G,  P,  G,  G,  P,  G,  G,  G,  G,  TR}, // 14 home(6,14), smith path
	{TR, G,  G,  G,  G,  G,  P,  G,  G,  P,  G,  P,  G,  G,  G,  G,  G,  G,  G,  TR}, // 15
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
			case TR, WA, WT, WM, WB, RL, RR, WE, SC:
				solid[y][x] = true
			}
		}
	}
	// Doors are walkable (they trigger NPC interaction)
	solid[5][3] = false   // merchant door
	solid[5][16] = false  // elder door
	solid[14][6] = false  // home door
	solid[11][16] = false // inn door
	solid[13][14] = false // blacksmith door
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
		{Name: "Bed", TileX: 6, TileY: 13, Role: "home"},
		{Name: "Innkeeper", TileX: 16, TileY: 12, Role: "innkeeper"},
		{Name: "Blacksmith", TileX: 14, TileY: 14, Role: "blacksmith"},
	}
}

// Town exit positions. The town is a hub with four exits:
//   North (row 0, cols 9-11)  → Frozen Path (snow chain)
//   South (row 17, cols 9-11) → Enchanted Forest (east chain — original path)
//   West  (col 0, row 8)      → Arid Desert (west chain — post-boss)
//   East  (col 19, row 8)     → Murky Swamp (south chain)
//
// THEORY — Why use row/column edges:
// The existing AreaConnection system uses horizontal strip matching
// (FromY + FromMinX..FromMaxX). For east/west exits we match column
// edges similarly but check tileX instead of tileY. The town.go
// updateWalking() function handles each exit direction explicitly.
var TownExitY     = 17 // south exit row
var TownExitNorthY = 0  // north exit row
var TownExitWestX  = 0  // west exit column
var TownExitEastX  = 19 // east exit column

// Save crystal position in the town.
const (
	TownSaveCrystalX = 12
	TownSaveCrystalY = 9
)

// Wormhole tile position in the town. Players can face it and press Z to
// open the multiplayer menu. The tile itself is walkable (stepping onto it
// also triggers the wormhole screen) to make discovery easier for new players.
const (
	TownWormholeX = 13
	TownWormholeY = 10
)
