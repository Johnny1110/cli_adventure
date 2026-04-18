// data/areas.go — Wild area map definitions and area graph.
//
// THEORY — Location graph:
// Each area is a node in a graph. Edges are connections — specific tile positions
// that, when stepped on, transition to another area. This is how Pokemon routes
// work: walk to the edge of one map, appear at the entry of the next.
//
// Each area also has an encounter rate (steps between random encounter checks)
// and a reference to its monster table in data/monsters.go.
package data

import "cli_adventure/asset"

// Area describes a wild exploration area.
type Area struct {
	Name          string
	MapKey        string // key into the encounter table
	Width, Height int
	Ground        [][]int
	Solid         [][]bool
	EncounterRate int // check for encounter every N steps (0 = no encounters)
	Connections   []AreaConnection
	PlayerStartX  int
	PlayerStartY  int
}

// ExitEdge describes which edge of the map an exit is on.
//
// THEORY — Direction-aware spawning:
// Classic RPGs like Pokemon place you on the *opposite* edge of the new map
// from the direction you came from. Walk off the east edge of Route 1 and
// you appear on the west edge of Route 2. This maintains spatial continuity
// and prevents the disorienting "I walked east but appeared at the top" effect.
// Each connection now carries its ExitEdge so the target area knows where
// to spawn the player.
type ExitEdge int

const (
	EdgeNorth ExitEdge = iota // exit is on the north (top) edge
	EdgeSouth                 // exit is on the south (bottom) edge
	EdgeWest                  // exit is on the west (left) edge
	EdgeEast                  // exit is on the east (right) edge
)

// AreaConnection defines a transition zone between areas.
type AreaConnection struct {
	TargetArea string   // area name to transition to
	Edge       ExitEdge // which edge of the map this exit is on

	// Source tile range on current map.
	// For north/south exits: FromMinX..FromMaxX at FromY.
	// For east/west exits: FromMinY..FromMaxY at FromX.
	FromMinX, FromMaxX int
	FromMinY, FromMaxY int
	FromX              int // column for east/west exits
	FromY              int // row for north/south exits
}

// Short aliases for forest tiles (prefixed W_ to avoid collision with town aliases)
const (
	FG  = asset.WildGrass
	FD  = asset.WildGrassDark
	FT  = asset.WildTree
	FTD = asset.WildTreeDense
	WP  = asset.WildPath     // W_ prefix to avoid colliding with town FP
	BU  = asset.WildBush     // BU for Bush, avoids colliding with town WB (WallBot)
	FR  = asset.WildRock
	FM  = asset.WildMushroom
	// Cave
	CF = asset.CaveFloor
	CW = asset.CaveWall
	CR = asset.CaveRock
	CC = asset.CaveCrystal
	// Lair
	LF = asset.LairFloor
	LL = asset.LairLava
	LB = asset.LairBone
	LS = asset.LairSkull
	// Special
	CH = asset.ChestClosed
	FF = asset.FairyFlower

	// Snow biome
	SG = asset.SnowGround
	SP = asset.SnowPath
	SN = asset.SnowPine
	SR = asset.SnowRock
	IF = asset.IceFloor
	IW = asset.IceWall

	// Swamp biome
	MG = asset.SwampGrass
	MW = asset.SwampWater
	MT = asset.SwampTree
	MM = asset.SwampMud

	// Volcano
	VF = asset.VolcFloor
	VW = asset.VolcWall
	VL = asset.VolcLava

	// Desert biome
	DF = asset.SandFloor
	DD = asset.SandDune
	DC = asset.Cactus
	DK = asset.SandRock

	// Ruins / Temple
	RF = asset.RuinsFloor
	RW = asset.RuinsWall
)

// ForestArea is the Enchanted Forest — first wild area.
func ForestArea() *Area {
	w, h := 20, 18
	ground := [][]int{
		{FTD, FTD, FTD, FTD, FTD, FTD, FTD, FTD, WP, WP, WP, FTD, FTD, FTD, FTD, FTD, FTD, FTD, FTD, FTD},
		{FTD, FT, FG, FG, FG, FG, FT, FG, WP, FG, WP, FG, FG, FM, FG, FG, FT, FG, FG, FTD},
		{FTD, FG, FG, BU, FG, FG, FG, FG, WP, FG, WP, FG, FG, FG, FG, FG, FG, BU, FG, FTD},
		{FTD, FG, FG, FG, FG, FT, FG, FG, WP, WP, WP, FG, FT, FG, FG, FG, FG, FG, FG, FTD},
		{FTD, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FM, FG, FG, FG, FG, FTD},
		{FTD, FT, FG, BU, FG, FG, FG, FR, FG, FG, FG, FR, FG, FG, FG, FG, BU, FG, FT, FTD},
		{FTD, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FG, FTD},
		{FTD, FG, FG, FG, FG, FG, FG, FG, FG, FD, FD, FG, FG, FG, FG, FG, FG, FG, FG, FTD},
		{FTD, FG, FT, FG, FG, FG, FD, FD, FD, FD, FD, FD, FD, FG, FG, FG, FG, FT, FG, FTD},
		{FTD, FG, FG, FG, FG, FG, FD, FG, FG, CH, FG, FG, FD, FG, FG, FG, FG, FG, FG, FTD},
		{FTD, FG, FG, FG, FG, FG, FD, FG, FG, FG, FG, FG, FD, FG, FG, FR, FG, FG, FG, FTD},
		{FTD, FG, FM, FG, FG, FG, FD, FG, FG, FG, FG, FG, FD, FG, FG, FG, FG, FG, FG, FTD},
		{FTD, FG, FG, FG, FG, FG, FD, FD, FD, WP, FD, FD, FD, FG, FG, FG, FG, FG, FG, FTD},
		{FTD, FT, FG, FG, FG, FG, FG, FG, FG, WP, FG, FG, FG, FG, FG, FG, FG, FT, FG, FTD},
		{FTD, FG, FG, BU, FG, FG, FG, FG, FG, WP, FG, FG, FG, FG, BU, FG, FG, FG, FG, FTD},
		{FTD, FG, FF, FG, FG, FG, FG, FG, FG, WP, FG, FG, FG, FG, FG, FG, FG, FG, FG, FTD},
		{FTD, FG, FG, FG, FG, FT, FG, FG, FG, WP, FG, FG, FT, FG, FG, FG, FG, CH, FG, FTD},
		{FTD, FTD, FTD, FTD, FTD, FTD, FTD, FTD, WP, WP, WP, FTD, FTD, FTD, FTD, FTD, FTD, FTD, FTD, FTD},
	}

	solid := makeSolidFromGround(w, h, ground, []int{FTD, FT, FR})

	return &Area{
		Name:          "Enchanted Forest",
		MapKey:        "forest",
		Width:         w,
		Height:        h,
		Ground:        ground,
		Solid:         solid,
		EncounterRate: 12, // encounter check every ~12 steps
		PlayerStartX:  9,
		PlayerStartY:  1,
		Connections: []AreaConnection{
			{TargetArea: "town", Edge: EdgeNorth, FromMinX: 8, FromMaxX: 10, FromY: 0},
			{TargetArea: "cave", Edge: EdgeSouth, FromMinX: 8, FromMaxX: 10, FromY: 17},
		},
	}
}

// CaveArea is the Dark Cave — second wild area.
func CaveArea() *Area {
	w, h := 20, 18
	ground := [][]int{
		{CW, CW, CW, CW, CW, CW, CW, CW, CF, CF, CF, CW, CW, CW, CW, CW, CW, CW, CW, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CR, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CR, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CC, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CR, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CR, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CR, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CR, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CC, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CH, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CR, CF, CF, CF, CF, CH, CF, CF, CF, CF, CF, CF, CR, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CF, CW},
		{CW, CW, CW, CW, CW, CW, CW, CW, CF, CF, CF, CW, CW, CW, CW, CW, CW, CW, CW, CW},
	}

	solid := makeSolidFromGround(w, h, ground, []int{CW, CR})

	return &Area{
		Name:          "Dark Cave",
		MapKey:        "cave",
		Width:         w,
		Height:        h,
		Ground:        ground,
		Solid:         solid,
		EncounterRate: 10,
		PlayerStartX:  9,
		PlayerStartY:  1,
		Connections: []AreaConnection{
			{TargetArea: "forest", Edge: EdgeNorth, FromMinX: 8, FromMaxX: 10, FromY: 0},
			{TargetArea: "lair", Edge: EdgeSouth, FromMinX: 8, FromMaxX: 10, FromY: 17},
		},
	}
}

// LairArea is the Dragon's Lair — boss room.
func LairArea() *Area {
	w, h := 12, 10
	ground := [][]int{
		{CW, CW, CW, CW, CW, CF, CF, CW, CW, CW, CW, CW},
		{CW, LF, LF, LF, LF, LF, LF, LF, LF, LF, LF, CW},
		{CW, LF, LL, LF, LF, LF, LF, LF, LF, LL, LF, CW},
		{CW, LF, LF, LF, LF, LB, LF, LF, LF, LF, LF, CW},
		{CW, LF, LF, LF, LF, LF, LF, LF, LF, LF, LF, CW},
		{CW, LF, LL, LF, LF, LS, LF, LF, LF, LL, LF, CW},
		{CW, LF, LF, LF, LF, LF, LF, LF, LF, LF, LF, CW},
		{CW, LF, LF, LB, LF, LF, LF, LB, LF, LF, LF, CW},
		{CW, LF, LF, LF, LF, LF, LF, LF, LF, LF, LF, CW},
		{CW, CW, CW, CW, CW, CF, CF, CW, CW, CW, CW, CW},
	}

	solid := makeSolidFromGround(w, h, ground, []int{CW, LL})

	return &Area{
		Name:          "Dragon's Lair",
		MapKey:        "lair",
		Width:         w,
		Height:        h,
		Ground:        ground,
		Solid:         solid,
		EncounterRate: 0, // boss area — fixed encounter, not random
		PlayerStartX:  5,
		PlayerStartY:  8,
		Connections: []AreaConnection{
			{TargetArea: "cave", Edge: EdgeSouth, FromMinX: 5, FromMaxX: 6, FromY: 9},
		},
	}
}

// ─── North Chain: Snow biome ───

// FrozenPathArea is the first area in the north chain — icy forest.
func FrozenPathArea() *Area {
	w, h := 20, 18
	ground := [][]int{
		{SN, SN, SN, SN, SN, SN, SN, SN, SP, SP, SP, SN, SN, SN, SN, SN, SN, SN, SN, SN},
		{SN, SG, SG, SG, SG, SG, SN, SG, SP, SG, SP, SG, SG, SG, SG, SG, SN, SG, SG, SN},
		{SN, SG, SG, SR, SG, SG, SG, SG, SP, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SN, SG, SG, SP, SP, SP, SG, SN, SG, SG, SG, SR, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SN, SG, SG, SG, SG, SG, SR, SG, SG, SG, SR, SG, SG, SG, SG, SG, SG, SN, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SN, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, CH, SG, SG, SN, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SR, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SN, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SG, SG, SN, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SN, SG, SG, SG, SP, SG, SG, SN, SG, SG, SG, SG, CH, SG, SN},
		{SN, SN, SN, SN, SN, SN, SN, SN, SP, SP, SP, SN, SN, SN, SN, SN, SN, SN, SN, SN},
	}
	solid := makeSolidFromGround(w, h, ground, []int{SN, SR})
	return &Area{
		Name: "Frozen Path", MapKey: "frozen_path",
		Width: w, Height: h, Ground: ground, Solid: solid,
		EncounterRate: 10, PlayerStartX: 9, PlayerStartY: 16,
		Connections: []AreaConnection{
			{TargetArea: "town", Edge: EdgeSouth, FromMinX: 8, FromMaxX: 10, FromY: 17},
			{TargetArea: "snow_mountains", Edge: EdgeNorth, FromMinX: 8, FromMaxX: 10, FromY: 0},
		},
	}
}

// SnowMountainsArea is the second area in the north chain.
func SnowMountainsArea() *Area {
	w, h := 20, 18
	ground := [][]int{
		{SN, SN, SN, SN, SN, SN, SN, SN, SP, SP, SP, SN, SN, SN, SN, SN, SN, SN, SN, SN},
		{SN, SG, SG, SR, SG, SG, SG, SG, SP, SG, SP, SG, SG, SR, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SP, SG, SP, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SR, SG, SG, SG, SG, SG, SG, SP, SP, SP, SG, SG, SG, SG, SG, SR, SG, SR, SN},
		{SN, SG, SG, SG, SG, SR, SG, SG, SG, SG, SG, SG, SR, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SR, SG, SG, SG, SN},
		{SN, SG, SR, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, CH, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SR, SG, SG, SG, SG, SG, SG, SG, SG, SG, SR, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SR, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SR, SG, SG, SG, SG, SG, SG, CH, SG, SG, SG, SG, SG, SG, SR, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SG, SN},
		{SN, SN, SN, SN, SN, SN, SN, SN, SP, SP, SP, SN, SN, SN, SN, SN, SN, SN, SN, SN},
	}
	solid := makeSolidFromGround(w, h, ground, []int{SN, SR})
	return &Area{
		Name: "Snow Mountains", MapKey: "snow_mountains",
		Width: w, Height: h, Ground: ground, Solid: solid,
		EncounterRate: 8, PlayerStartX: 9, PlayerStartY: 16,
		Connections: []AreaConnection{
			{TargetArea: "frozen_path", Edge: EdgeSouth, FromMinX: 8, FromMaxX: 10, FromY: 17},
			{TargetArea: "ice_cavern", Edge: EdgeNorth, FromMinX: 8, FromMaxX: 10, FromY: 0},
		},
	}
}

// IceCavernArea is the boss room of the north chain.
func IceCavernArea() *Area {
	w, h := 12, 10
	ground := [][]int{
		{IW, IW, IW, IW, IW, IF, IF, IW, IW, IW, IW, IW},
		{IW, IF, IF, IF, IF, IF, IF, IF, IF, IF, IF, IW},
		{IW, IF, IF, IF, IF, IF, IF, IF, IF, IF, IF, IW},
		{IW, IF, IF, IF, IF, IF, IF, IF, IF, IF, IF, IW},
		{IW, IF, IF, IF, IF, IF, IF, IF, IF, IF, IF, IW},
		{IW, IF, IF, IF, IF, IF, IF, IF, IF, IF, IF, IW},
		{IW, IF, IF, IF, IF, IF, IF, IF, IF, IF, IF, IW},
		{IW, IF, IF, IF, IF, IF, IF, IF, IF, IF, IF, IW},
		{IW, IF, IF, IF, IF, IF, IF, IF, IF, IF, IF, IW},
		{IW, IW, IW, IW, IW, IF, IF, IW, IW, IW, IW, IW},
	}
	solid := makeSolidFromGround(w, h, ground, []int{IW})
	return &Area{
		Name: "Ice Cavern", MapKey: "ice_cavern",
		Width: w, Height: h, Ground: ground, Solid: solid,
		EncounterRate: 0, PlayerStartX: 5, PlayerStartY: 8,
		Connections: []AreaConnection{
			{TargetArea: "snow_mountains", Edge: EdgeSouth, FromMinX: 5, FromMaxX: 6, FromY: 9},
		},
	}
}

// ─── South Chain: Swamp + Volcano ───

// SwampArea is the first area in the south chain.
func SwampArea() *Area {
	w, h := 20, 18
	// West/east exits: left edge (col 0) → town, right edge (col 19) → volcano.
	// Rows 8-10 are open on both edges. The mud river runs vertically through center.
	ground := [][]int{
		{MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT},
		{MT, MG, MG, MG, MG, MG, MT, MG, MG, MG, MG, MG, MG, MG, MG, MG, MT, MG, MG, MT},
		{MT, MG, MW, MW, MG, MG, MG, MG, MM, MG, MM, MG, MG, MG, MW, MW, MG, MG, MG, MT},
		{MT, MG, MW, MW, MG, MG, MG, MG, MM, MM, MM, MG, MG, MG, MW, MW, MG, MG, MG, MT},
		{MT, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MT},
		{MT, MT, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG, CH, MG, MG, MG, MG, MT, MT},
		{MT, MG, MG, MG, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MG, MG, MG, MG, MT},
		{MT, MG, MG, MT, MG, MG, MG, MW, MG, MM, MG, MG, MW, MG, MG, MG, MT, MG, MG, MT},
		{MG, MG, MG, MG, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG},
		{MG, MG, MG, MG, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG},
		{MG, MG, MG, MG, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MG, MG, MG, MG, MG},
		{MT, MG, MG, MW, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MW, MG, MG, MG, MT},
		{MT, MG, MG, MG, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MG, MG, MG, MG, MT},
		{MT, MT, MG, MG, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MG, MG, MT, MG, MT},
		{MT, MG, MG, MG, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MG, MG, MG, MG, MT},
		{MT, MG, MG, MG, MG, MG, MG, MG, MG, MM, MG, MG, MG, MG, MG, MG, MG, MG, MG, MT},
		{MT, MG, MG, MG, MG, MT, MG, MG, MG, MM, MG, MG, MT, MG, MG, MG, MG, CH, MG, MT},
		{MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT, MT},
	}
	solid := makeSolidFromGround(w, h, ground, []int{MT, MW})
	return &Area{
		Name: "Murky Swamp", MapKey: "swamp",
		Width: w, Height: h, Ground: ground, Solid: solid,
		EncounterRate: 8, PlayerStartX: 1, PlayerStartY: 9,
		Connections: []AreaConnection{
			{TargetArea: "town", Edge: EdgeWest, FromX: 0, FromMinY: 8, FromMaxY: 10},
			{TargetArea: "volcano", Edge: EdgeEast, FromX: 19, FromMinY: 8, FromMaxY: 10},
		},
	}
}

// VolcanoArea is the boss room of the south chain.
func VolcanoArea() *Area {
	w, h := 12, 10
	// West exit (col 0) → swamp. Player enters from the left.
	ground := [][]int{
		{VW, VW, VW, VW, VW, VW, VW, VW, VW, VW, VW, VW},
		{VW, VF, VF, VF, VF, VF, VF, VF, VF, VF, VF, VW},
		{VW, VF, VL, VF, VF, VF, VF, VF, VF, VL, VF, VW},
		{VW, VF, VF, VF, VF, VF, VF, VF, VF, VF, VF, VW},
		{VF, VF, VF, VF, VF, VF, VF, VF, VF, VF, VF, VW},
		{VF, VF, VF, VF, VF, VF, VF, VF, VF, VF, VF, VW},
		{VW, VF, VF, VF, VF, VF, VF, VF, VF, VF, VF, VW},
		{VW, VF, VL, VF, VF, VF, VF, VF, VF, VL, VF, VW},
		{VW, VF, VF, VF, VF, VF, VF, VF, VF, VF, VF, VW},
		{VW, VW, VW, VW, VW, VW, VW, VW, VW, VW, VW, VW},
	}
	solid := makeSolidFromGround(w, h, ground, []int{VW, VL})
	return &Area{
		Name: "Volcano Core", MapKey: "volcano",
		Width: w, Height: h, Ground: ground, Solid: solid,
		EncounterRate: 0, PlayerStartX: 1, PlayerStartY: 5,
		Connections: []AreaConnection{
			{TargetArea: "swamp", Edge: EdgeWest, FromX: 0, FromMinY: 4, FromMaxY: 5},
		},
	}
}

// ─── West Chain: Desert + Ruins + Temple ───

// DesertArea is the first area in the west chain (post-boss content).
func DesertArea() *Area {
	w, h := 20, 18
	// East/west exits: right edge (col 19) → town, left edge (col 0) → sand ruins.
	// Rows 8-10 are open on both edges for the exit corridors.
	ground := [][]int{
		{DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD},
		{DD, DF, DF, DF, DF, DF, DD, DF, DF, DF, DF, DF, DF, DF, DF, DF, DD, DF, DF, DD},
		{DD, DF, DF, DC, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DC, DF, DF, DF, DF, DD},
		{DD, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DD},
		{DD, DF, DF, DF, DF, DK, DF, DF, DF, DF, DF, DF, DK, DF, DF, DF, DF, DF, DF, DD},
		{DD, DD, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DD, DD},
		{DD, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DD},
		{DD, DF, DF, DD, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DD, DF, DF, DF, DD},
		{DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, CH, DF, DF, DF, DF, DF},
		{DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF},
		{DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF},
		{DD, DF, DF, DC, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DC, DF, DF, DF, DD},
		{DD, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DD},
		{DD, DF, DK, DF, DF, DF, DF, DF, DF, CH, DF, DF, DF, DF, DF, DF, DK, DF, DF, DD},
		{DD, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DD},
		{DD, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DF, DD},
		{DD, DF, DF, DF, DF, DD, DF, DF, DF, DF, DF, DF, DD, DF, DF, DF, DF, DC, DF, DD},
		{DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD, DD},
	}
	solid := makeSolidFromGround(w, h, ground, []int{DD, DK, DC})
	return &Area{
		Name: "Arid Desert", MapKey: "desert",
		Width: w, Height: h, Ground: ground, Solid: solid,
		EncounterRate: 10, PlayerStartX: 18, PlayerStartY: 9,
		Connections: []AreaConnection{
			{TargetArea: "town", Edge: EdgeEast, FromX: 19, FromMinY: 8, FromMaxY: 10},
			{TargetArea: "sand_ruins", Edge: EdgeWest, FromX: 0, FromMinY: 8, FromMaxY: 10},
		},
	}
}

// SandRuinsArea is the second area in the west chain.
func SandRuinsArea() *Area {
	w, h := 20, 18
	// East/west exits: right edge (col 19) → desert, left edge (col 0) → buried temple.
	ground := [][]int{
		{RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW},
		{RW, RF, RF, RF, RF, RF, RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RW, RF, RF, RF, RF, RF, RF, RW, RF, RF, RF, RF, RF, RF, RW},
		{RW, RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, CH, RF, RF, RF, RW, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW, RF, RF, RF, RW},
		{RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF},
		{RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF},
		{RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RW, RF, RF, RF, RF, RF, RF, RF, CH, RF, RF, RF, RF, RF, RF, RF, RW, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RW, RF, RF, RF, RF, RF, RF, RW, RF, RF, RF, RF, RF, RF, RW},
		{RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW},
	}
	solid := makeSolidFromGround(w, h, ground, []int{RW})
	return &Area{
		Name: "Sand Ruins", MapKey: "sand_ruins",
		Width: w, Height: h, Ground: ground, Solid: solid,
		EncounterRate: 8, PlayerStartX: 18, PlayerStartY: 9,
		Connections: []AreaConnection{
			{TargetArea: "desert", Edge: EdgeEast, FromX: 19, FromMinY: 8, FromMaxY: 10},
			{TargetArea: "buried_temple", Edge: EdgeWest, FromX: 0, FromMinY: 8, FromMaxY: 10},
		},
	}
}

// BuriedTempleArea is the boss room of the west chain.
func BuriedTempleArea() *Area {
	w, h := 12, 10
	// East exit (col 11) → sand ruins. Player enters from the right.
	ground := [][]int{
		{RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RF, RF, RF, RF, RF, RF, RF, RF, RF, RF, RW},
		{RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW, RW},
	}
	solid := makeSolidFromGround(w, h, ground, []int{RW})
	return &Area{
		Name: "Buried Temple", MapKey: "buried_temple",
		Width: w, Height: h, Ground: ground, Solid: solid,
		EncounterRate: 0, PlayerStartX: 10, PlayerStartY: 5,
		Connections: []AreaConnection{
			{TargetArea: "sand_ruins", Edge: EdgeEast, FromX: 11, FromMinY: 4, FromMaxY: 5},
		},
	}
}

// AllAreas returns a map of all wild areas keyed by name.
func AllAreas() map[string]*Area {
	return map[string]*Area{
		// East chain (original)
		"forest": ForestArea(),
		"cave":   CaveArea(),
		"lair":   LairArea(),
		// North chain (snow)
		"frozen_path":    FrozenPathArea(),
		"snow_mountains": SnowMountainsArea(),
		"ice_cavern":     IceCavernArea(),
		// South chain (swamp + volcano)
		"swamp":   SwampArea(),
		"volcano": VolcanoArea(),
		// West chain (desert + ruins — post-boss)
		"desert":         DesertArea(),
		"sand_ruins":     SandRuinsArea(),
		"buried_temple":  BuriedTempleArea(),
	}
}

// OppositeEdge returns the edge opposite to the given one.
// If you exited via EdgeNorth, you entered the next map from its south side.
func OppositeEdge(e ExitEdge) ExitEdge {
	switch e {
	case EdgeNorth:
		return EdgeSouth
	case EdgeSouth:
		return EdgeNorth
	case EdgeWest:
		return EdgeEast
	case EdgeEast:
		return EdgeWest
	}
	return EdgeNorth
}

// SpawnForEntry returns the (tileX, tileY) where a player should appear
// when entering the given area from a specific edge.
//
// THEORY — Opposite-edge spawning:
// In Pokemon, walking off Route 1's south edge places you at Route 2's
// north edge. Your brain models these maps as spatially adjacent, so the
// entry point must be on the edge you'd expect if the maps were stitched
// together. "I walked south out of the forest, so I appear at the TOP of
// the cave." This single rule eliminates the disorientation the user was
// experiencing when town exits didn't match area spawn positions.
//
// For east/west entry, we pick the vertical center of the map and
// place the player 1 tile inward from the edge (to avoid instantly
// re-triggering an exit). For north/south, we use the horizontal center.
func SpawnForEntry(area *Area, entryEdge ExitEdge) (int, int) {
	centerX := area.Width / 2
	centerY := area.Height / 2

	var x, y int
	switch entryEdge {
	case EdgeNorth:
		x, y = centerX, 1
	case EdgeSouth:
		x, y = centerX, area.Height-2
	case EdgeWest:
		x, y = 1, centerY
	case EdgeEast:
		x, y = area.Width-2, centerY
	default:
		return area.PlayerStartX, area.PlayerStartY
	}

	// If the computed position is solid, search nearby for a walkable tile.
	// This prevents spawning inside walls/trees on maps with irregular edges.
	if area.Solid != nil && y >= 0 && y < area.Height && x >= 0 && x < area.Width && area.Solid[y][x] {
		// Search in a small radius
		for r := 1; r < 5; r++ {
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 1 && nx < area.Width-1 && ny >= 1 && ny < area.Height-1 {
						if !area.Solid[ny][nx] {
							return nx, ny
						}
					}
				}
			}
		}
	}
	return x, y
}

// makeSolidFromGround generates a collision grid from the ground layer.
func makeSolidFromGround(w, h int, ground [][]int, solidTiles []int) [][]bool {
	solidSet := map[int]bool{}
	for _, t := range solidTiles {
		solidSet[t] = true
	}

	solid := make([][]bool, h)
	for y := 0; y < h; y++ {
		solid[y] = make([]bool, w)
		for x := 0; x < w; x++ {
			if solidSet[ground[y][x]] {
				solid[y][x] = true
			}
		}
	}
	return solid
}
