// entity/quest.go — Quest tracking.
//
// THEORY: Quests are simple "kill N of X" objectives for the MVP.
// The quest struct tracks target, progress, and reward. When the player
// defeats a monster in combat (Phase 4), we check all active quests to
// see if that monster type matches a quest target and increment progress.
package entity

// Quest represents an active or completed quest.
type Quest struct {
	Name       string
	Desc       string
	Target     string // monster name to defeat
	Required   int    // how many to defeat
	Progress   int    // how many defeated so far
	RewardXP   int
	RewardCoin int
	Done       bool
}

// IsComplete returns true if the quest objective is fulfilled.
func (q *Quest) IsComplete() bool {
	return q.Progress >= q.Required
}

// AvailableQuests returns the quest pool the Elder can offer.
//
// THEORY — Quest scaling across biomes:
// Each biome chain gets 1-2 quests so the Elder stays relevant throughout
// the game. Rewards scale with biome difficulty: forest quests give modest
// XP/coins, while desert/volcano quests give endgame rewards. This creates
// a natural incentive to revisit the Elder after each new area, and the
// increasing rewards track the player's growing power and expenses.
func AvailableQuests() []Quest {
	return []Quest{
		// --- East chain (Forest → Cave → Lair) ---
		{
			Name:       "Slime Trouble",
			Desc:       "Defeat 3 Slimes in\nthe Enchanted Forest.",
			Target:     "Slime",
			Required:   3,
			RewardXP:   30,
			RewardCoin: 50,
		},
		{
			Name:       "Bat Bane",
			Desc:       "Defeat 5 Bats in\nthe Dark Cave.",
			Target:     "Bat",
			Required:   5,
			RewardXP:   60,
			RewardCoin: 80,
		},
		// --- North chain (Frozen Path → Snow Mountains → Ice Cavern) ---
		{
			Name:       "Frost Wolf Hunt",
			Desc:       "Defeat 4 Frost Wolves\non the Frozen Path.",
			Target:     "Frost Wolf",
			Required:   4,
			RewardXP:   80,
			RewardCoin: 120,
		},
		{
			Name:       "Yeti Problem",
			Desc:       "Defeat 3 Yetis in the\nSnow Mountains.",
			Target:     "Yeti",
			Required:   3,
			RewardXP:   120,
			RewardCoin: 180,
		},
		// --- South chain (Swamp → Volcano) ---
		{
			Name:       "Bog Cleanup",
			Desc:       "Defeat 5 Bog Toads in\nthe Murky Swamp.",
			Target:     "Bog Toad",
			Required:   5,
			RewardXP:   100,
			RewardCoin: 150,
		},
		{
			Name:       "Lava Lizards",
			Desc:       "Defeat 4 Magma Lizards\nin the Volcano.",
			Target:     "Magma Lizard",
			Required:   4,
			RewardXP:   150,
			RewardCoin: 220,
		},
		// --- West chain (Desert → Sand Ruins → Buried Temple) ---
		{
			Name:       "Scorpion Scourge",
			Desc:       "Defeat 5 Scorpions\nin the Arid Desert.",
			Target:     "Scorpion",
			Required:   5,
			RewardXP:   140,
			RewardCoin: 200,
		},
		{
			Name:       "Mummy Hunt",
			Desc:       "Defeat 3 Mummies in\nthe Sand Ruins.",
			Target:     "Mummy",
			Required:   3,
			RewardXP:   200,
			RewardCoin: 300,
		},
	}
}
