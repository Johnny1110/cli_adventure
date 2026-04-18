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

// AreaConnection defines a transition zone between areas.
type AreaConnection struct {
	TargetArea string // area name to transition to
	// Source tile range on current map
	FromMinX, FromMaxX int
	FromY              int // the row that triggers transition
	IsExitSouth        bool // true = bottom edge, false = top edge
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
			{TargetArea: "town", FromMinX: 8, FromMaxX: 10, FromY: 0, IsExitSouth: false},
			{TargetArea: "cave", FromMinX: 8, FromMaxX: 10, FromY: 17, IsExitSouth: true},
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
			{TargetArea: "forest", FromMinX: 8, FromMaxX: 10, FromY: 0, IsExitSouth: false},
			{TargetArea: "lair", FromMinX: 8, FromMaxX: 10, FromY: 17, IsExitSouth: true},
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
			{TargetArea: "cave", FromMinX: 5, FromMaxX: 6, FromY: 9, IsExitSouth: true},
		},
	}
}

// AllAreas returns a map of all wild areas keyed by name.
func AllAreas() map[string]*Area {
	return map[string]*Area{
		"forest": ForestArea(),
		"cave":   CaveArea(),
		"lair":   LairArea(),
	}
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
