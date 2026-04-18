// entity/skill.go — Skill tree data types and class skill definitions.
//
// THEORY — Skill trees with XP-spend progression:
// Each class has a set of 4 skills arranged in a linear tree. Skills unlock
// sequentially: you must learn Skill 1 before Skill 2, etc. Each skill has
// 3 levels (Lv.1 → Lv.3), and each level costs increasing skill points (SP).
//
// SP are earned on level-up (1 SP per level), creating a genuine resource
// allocation decision: do you max out your first skill early, or spread
// points across multiple skills? This is the same tension as Path of Exile's
// or Final Fantasy Tactics' skill systems.
//
// Skills in combat replace the single "Magic" action with a choice of learned
// skills. Each skill has an MP cost, a damage multiplier, and a special effect.
// Higher skill levels reduce MP cost or increase damage, rewarding investment.
//
// Skill design per class:
//   Knight: Shield Bash (stun), War Cry (ATK buff), Holy Strike (heal+damage), Aegis (DEF buff all)
//   Mage:   Fireball (AoE damage), Ice Shard (slow), Thunder (high damage), Meteor (nuke)
//   Archer: Snipe (crit), Poison Arrow (DoT), Multi Shot (hits), Deadeye (guaranteed crit+)
package entity

// SkillID uniquely identifies a skill across all classes.
type SkillID int

const (
	// Knight skills
	SkillShieldBash SkillID = iota
	SkillWarCry
	SkillHolyStrike
	SkillAegis

	// Mage skills
	SkillFireball
	SkillIceShard
	SkillThunder
	SkillMeteor

	// Archer skills
	SkillSnipe
	SkillPoisonArrow
	SkillMultiShot
	SkillDeadeye
)

// SkillDef defines a skill's properties at a given level.
type SkillDef struct {
	ID          SkillID
	Name        string
	Desc        string
	MaxLevel    int
	MPCost      [3]int     // MP cost at level 1, 2, 3
	Multiplier  [3]float64 // damage multiplier at each level
	SPCost      [3]int     // SP cost to reach level 1, 2, 3
	SpecialDesc string     // short description of special effect
}

// PlayerSkill tracks a learned skill and its current level.
type PlayerSkill struct {
	ID    SkillID
	Level int // 0 = not learned, 1-3 = current level
}

// ClassSkillTree returns the skill definitions for a class, in tree order.
func ClassSkillTree(class Class) []SkillDef {
	switch class {
	case ClassKnight:
		return knightSkills
	case ClassMage:
		return mageSkills
	case ClassArcher:
		return archerSkills
	}
	return nil
}

var knightSkills = []SkillDef{
	{
		ID: SkillShieldBash, Name: "Shield Bash", MaxLevel: 3,
		Desc:        "Bash with shield.\nStuns enemy 1 turn.",
		MPCost:      [3]int{3, 3, 2},
		Multiplier:  [3]float64{0.8, 1.0, 1.2},
		SPCost:      [3]int{1, 2, 3},
		SpecialDesc: "Stun 1 turn",
	},
	{
		ID: SkillWarCry, Name: "War Cry", MaxLevel: 3,
		Desc:        "Boost ATK for\nnext 2 attacks.",
		MPCost:      [3]int{4, 4, 3},
		Multiplier:  [3]float64{1.3, 1.5, 1.8},
		SPCost:      [3]int{1, 2, 3},
		SpecialDesc: "ATK buff 2 turns",
	},
	{
		ID: SkillHolyStrike, Name: "Holy Strike", MaxLevel: 3,
		Desc:        "Deal damage and\nheal HP equal to\nhalf damage dealt.",
		MPCost:      [3]int{5, 5, 4},
		Multiplier:  [3]float64{1.0, 1.3, 1.6},
		SPCost:      [3]int{2, 3, 4},
		SpecialDesc: "Lifesteal 50%",
	},
	{
		ID: SkillAegis, Name: "Aegis", MaxLevel: 3,
		Desc:        "Raise a divine shield.\nHalves next hit.",
		MPCost:      [3]int{4, 3, 2},
		Multiplier:  [3]float64{0, 0, 0}, // no damage, pure defense
		SPCost:      [3]int{2, 3, 4},
		SpecialDesc: "Block 50% next hit",
	},
}

var mageSkills = []SkillDef{
	{
		ID: SkillFireball, Name: "Fireball", MaxLevel: 3,
		Desc:        "Launch a ball of fire.\nHigh magic damage.",
		MPCost:      [3]int{5, 5, 4},
		Multiplier:  [3]float64{2.0, 2.5, 3.0},
		SPCost:      [3]int{1, 2, 3},
		SpecialDesc: "Pure magic damage",
	},
	{
		ID: SkillIceShard, Name: "Ice Shard", MaxLevel: 3,
		Desc:        "Freeze the enemy.\nReduces enemy SPD.",
		MPCost:      [3]int{4, 4, 3},
		Multiplier:  [3]float64{1.5, 1.8, 2.2},
		SPCost:      [3]int{1, 2, 3},
		SpecialDesc: "Enemy SPD -3",
	},
	{
		ID: SkillThunder, Name: "Thunder", MaxLevel: 3,
		Desc:        "Call down lightning.\nIgnores defense.",
		MPCost:      [3]int{7, 6, 5},
		Multiplier:  [3]float64{2.5, 3.0, 3.5},
		SPCost:      [3]int{2, 3, 4},
		SpecialDesc: "Ignores DEF",
	},
	{
		ID: SkillMeteor, Name: "Meteor", MaxLevel: 3,
		Desc:        "Ultimate destruction.\nMassive magic damage.",
		MPCost:      [3]int{12, 10, 8},
		Multiplier:  [3]float64{4.0, 5.0, 6.0},
		SPCost:      [3]int{3, 4, 5},
		SpecialDesc: "Devastating power",
	},
}

var archerSkills = []SkillDef{
	{
		ID: SkillSnipe, Name: "Snipe", MaxLevel: 3,
		Desc:        "Aimed shot.\nGuaranteed critical!",
		MPCost:      [3]int{4, 4, 3},
		Multiplier:  [3]float64{1.5, 1.8, 2.2},
		SPCost:      [3]int{1, 2, 3},
		SpecialDesc: "Always crits",
	},
	{
		ID: SkillPoisonArrow, Name: "Poison Arrow", MaxLevel: 3,
		Desc:        "Venomous arrow.\nPoison for 3 turns.",
		MPCost:      [3]int{3, 3, 2},
		Multiplier:  [3]float64{0.8, 1.0, 1.2},
		SPCost:      [3]int{1, 2, 3},
		SpecialDesc: "Poison 3 turns",
	},
	{
		ID: SkillMultiShot, Name: "Multi Shot", MaxLevel: 3,
		Desc:        "Fire multiple arrows.\nHits 2-3 times.",
		MPCost:      [3]int{5, 5, 4},
		Multiplier:  [3]float64{0.6, 0.7, 0.8},
		SPCost:      [3]int{2, 3, 4},
		SpecialDesc: "Hits 2-3 times",
	},
	{
		ID: SkillDeadeye, Name: "Deadeye", MaxLevel: 3,
		Desc:        "Perfect aim.\nTriple damage crit.",
		MPCost:      [3]int{8, 7, 6},
		Multiplier:  [3]float64{2.5, 3.0, 3.5},
		SPCost:      [3]int{3, 4, 5},
		SpecialDesc: "3x crit damage",
	},
}
