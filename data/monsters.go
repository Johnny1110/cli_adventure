// data/monsters.go — Monster stat tables.
//
// THEORY — Encounter tables:
// Each wild area has a weighted monster table. When a random encounter triggers,
// we roll against the weights to pick which monster appears. Higher-weight
// monsters appear more often. This is the same system Pokemon uses —
// common Pokemon have high encounter rates, rare ones are low.
package data

import "cli_adventure/entity"

// MonsterTemplates is the global monster registry, keyed by name.
var MonsterTemplates = map[string]*entity.Monster{
	"Slime": {
		Name: "Slime", HP: 12, MaxHP: 12,
		ATK: 4, DEF: 2, SPD: 3,
		XPReward: 8, CoinReward: 5,
		SpriteID: "slime",
	},
	"Bat": {
		Name: "Bat", HP: 10, MaxHP: 10,
		ATK: 6, DEF: 1, SPD: 8,
		XPReward: 10, CoinReward: 7,
		SpriteID: "bat",
	},
	"Mushroom": {
		Name: "Mushroom", HP: 18, MaxHP: 18,
		ATK: 5, DEF: 6, SPD: 2,
		XPReward: 12, CoinReward: 10,
		SpriteID: "mushroom",
	},
	"Dragon": {
		Name: "Dragon", HP: 80, MaxHP: 80,
		ATK: 16, DEF: 10, SPD: 7,
		XPReward: 200, CoinReward: 300,
		IsBoss:   true,
		SpriteID: "dragon",
	},
}

// EncounterEntry pairs a monster name with a spawn weight.
type EncounterEntry struct {
	Name   string
	Weight int // higher = more common
}

// EncounterTable maps area names to their monster lists.
var EncounterTable = map[string][]EncounterEntry{
	"forest": {
		{Name: "Slime", Weight: 60},
		{Name: "Mushroom", Weight: 30},
		{Name: "Bat", Weight: 10},
	},
	"cave": {
		{Name: "Bat", Weight: 50},
		{Name: "Mushroom", Weight: 30},
		{Name: "Slime", Weight: 20},
	},
	"lair": {
		// Boss area — always the dragon
		{Name: "Dragon", Weight: 100},
	},
}
