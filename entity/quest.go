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
func AvailableQuests() []Quest {
	return []Quest{
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
	}
}
