// entity/monster.go — Monster definitions.
//
// THEORY — Monsters as data:
// Each monster is a template: stats, name, rewards. When an encounter triggers,
// the game clones a template into a live combat instance with its own HP.
// This is the prototype pattern — one definition, many instances.
// Adding a new monster = adding one entry to data/monsters.go.
package entity

// Monster represents a monster in combat.
type Monster struct {
	Name       string
	HP         int
	MaxHP      int
	ATK        int
	DEF        int
	SPD        int
	XPReward   int
	CoinReward int
	IsBoss     bool
	IsGolden   bool   // rare golden variant — 3x rewards, boosted stats
	BaseName   string // original name before golden prefix (for quest tracking)
	SpriteID   string // key into the monster sprite map
}

// Clone creates a fresh copy of a monster template for combat.
func (m *Monster) Clone() *Monster {
	copy := *m
	copy.HP = copy.MaxHP
	return &copy
}

// MakeGolden transforms a monster into a rare golden variant.
// Golden monsters are tougher but give 3x XP and coins.
func (m *Monster) MakeGolden() {
	m.IsGolden = true
	m.BaseName = m.Name // preserve original name for quest tracking
	m.Name = "Golden " + m.Name
	m.MaxHP = m.MaxHP * 3 / 2
	m.HP = m.MaxHP
	m.ATK = m.ATK + 2
	m.DEF = m.DEF + 2
	m.XPReward *= 3
	m.CoinReward *= 3
}
