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
		ATK: 5, DEF: 2, SPD: 3,
		XPReward: 8, CoinReward: 5,
		SpriteID: "slime",
		CRITRATE: 10,
	},
	"Bat": {
		Name: "Bat", HP: 10, MaxHP: 10,
		ATK: 7, DEF: 1, SPD: 8,
		XPReward: 10, CoinReward: 7,
		SpriteID: "bat",
		CRITRATE: 10,
	},
	"Mushroom": {
		Name: "Mushroom", HP: 18, MaxHP: 18,
		ATK: 7, DEF: 8, SPD: 2,
		XPReward: 12, CoinReward: 10,
		SpriteID: "mushroom",
		CRITRATE: 10,
	},
	"Dragon": {
		Name: "Dragon", HP: 100, MaxHP: 100,
		ATK: 16, DEF: 13, SPD: 8,
		XPReward: 200, CoinReward: 300,
		IsBoss:   true,
		SpriteID: "dragon",
		CRITRATE: 25,
	},

	// --- Snow biome (level 8-12) ---
	"Ice Wolf": {
		Name: "Ice Wolf", HP: 35, MaxHP: 35,
		ATK: 14, DEF: 8, SPD: 10,
		XPReward: 45, CoinReward: 30,
		SpriteID: "ice_wolf",
		CRITRATE: 10,
	},
	"Frost Golem": {
		Name: "Frost Golem", HP: 60, MaxHP: 60,
		ATK: 10, DEF: 18, SPD: 3,
		XPReward: 55, CoinReward: 40,
		SpriteID: "frost_golem",
		CRITRATE: 10,
	},
	"Snow Harpy": {
		Name: "Snow Harpy", HP: 25, MaxHP: 25,
		ATK: 12, DEF: 5, SPD: 14,
		XPReward: 40, CoinReward: 25,
		SpriteID: "snow_harpy",
		CRITRATE: 10,
	},
	"Ice Wyrm": {
		Name: "Ice Wyrm", HP: 120, MaxHP: 120,
		ATK: 25, DEF: 18, SPD: 11,
		XPReward: 350, CoinReward: 500,
		IsBoss:   true,
		SpriteID: "ice_wyrm",
		CRITRATE: 30,
	},

	// --- Swamp biome (level 12-15) ---
	"Toxic Frog": {
		Name: "Toxic Frog", HP: 40, MaxHP: 40,
		ATK: 13, DEF: 10, SPD: 7,
		XPReward: 50, CoinReward: 35,
		SpriteID: "toxic_frog",
		CRITRATE: 10,
	},
	"Bog Lurker": {
		Name: "Bog Lurker", HP: 70, MaxHP: 70,
		ATK: 15, DEF: 12, SPD: 2,
		XPReward: 60, CoinReward: 45,
		SpriteID: "bog_lurker",
		CRITRATE: 10,
	},
	"Will-o-Wisp": {
		Name: "Will-o-Wisp", HP: 20, MaxHP: 20,
		ATK: 16, DEF: 3, SPD: 15,
		XPReward: 45, CoinReward: 30,
		SpriteID: "wisp",
		CRITRATE: 20,
	},
	"Hydra": {
		Name: "Hydra", HP: 150, MaxHP: 150,
		ATK: 30, DEF: 23, SPD: 20,
		XPReward: 500, CoinReward: 700,
		IsBoss:   true,
		SpriteID: "hydra",
		CRITRATE: 50,
	},

	// --- Desert biome (level 10-18) ---
	"Sand Scorpion": {
		Name: "Sand Scorpion", HP: 45, MaxHP: 45,
		ATK: 20, DEF: 10, SPD: 8,
		XPReward: 55, CoinReward: 40,
		SpriteID: "scorpion",
		CRITRATE: 10,
	},
	"Dust Devil": {
		Name: "Dust Devil", HP: 30, MaxHP: 30,
		ATK: 26, DEF: 20, SPD: 16,
		XPReward: 50, CoinReward: 35,
		SpriteID: "dust_devil",
		CRITRATE: 20,
	},
	"Mummy": {
		Name: "Mummy", HP: 55, MaxHP: 55,
		ATK: 30, DEF: 16, SPD: 4,
		XPReward: 60, CoinReward: 45,
		SpriteID: "mummy",
		CRITRATE: 20,
	},
	"Sphinx": {
		Name: "Sphinx", HP: 180, MaxHP: 180,
		ATK: 45, DEF: 30, SPD: 30,
		XPReward: 600, CoinReward: 900,
		IsBoss:   true,
		SpriteID: "sphinx",
		CRITRATE: 50,
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

	// --- Snow biome ---
	"frozen_path": {
		{Name: "Ice Wolf", Weight: 50},
		{Name: "Snow Harpy", Weight: 30},
		{Name: "Frost Golem", Weight: 20},
	},
	"snow_mountains": {
		{Name: "Frost Golem", Weight: 40},
		{Name: "Ice Wolf", Weight: 35},
		{Name: "Snow Harpy", Weight: 25},
	},
	"ice_cavern": {
		{Name: "Ice Wyrm", Weight: 100},
	},

	// --- Swamp biome ---
	"swamp": {
		{Name: "Toxic Frog", Weight: 45},
		{Name: "Bog Lurker", Weight: 30},
		{Name: "Will-o-Wisp", Weight: 25},
	},
	"volcano": {
		{Name: "Hydra", Weight: 100},
	},

	// --- Desert biome ---
	"desert": {
		{Name: "Sand Scorpion", Weight: 40},
		{Name: "Dust Devil", Weight: 35},
		{Name: "Mummy", Weight: 25},
	},
	"sand_ruins": {
		{Name: "Mummy", Weight: 40},
		{Name: "Sand Scorpion", Weight: 35},
		{Name: "Dust Devil", Weight: 25},
	},
	"buried_temple": {
		{Name: "Sphinx", Weight: 100},
	},
}
