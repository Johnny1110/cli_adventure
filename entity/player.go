// Package entity defines the core domain types: Player, Monster, NPC, Item, Quest.
//
// THEORY: These are pure data types with game-logic methods — no rendering code.
// This separation matters because it lets us unit-test game mechanics (damage,
// leveling, inventory) without any graphics dependency. The rendering layer reads
// from these structs but never modifies them; only Update() logic does that.
package entity

// Class represents the player's chosen class.
type Class int

const (
	ClassKnight Class = iota
	ClassMage
	ClassArcher
)

// ClassInfo holds display metadata and base stats for a class.
type ClassInfo struct {
	Name string
	Desc string
	Base Stats
}

// Stats holds the numeric attributes of a character.
type Stats struct {
	MaxHP int
	HP    int
	MaxMP int
	MP    int
	ATK   int
	DEF   int
	SPD   int
}

// ClassTable maps each class to its info. This is the data-driven approach:
// adding a new class means adding one entry here, not changing logic.
var ClassTable = map[Class]ClassInfo{
	ClassKnight: {
		Name: "Knight",
		Desc: "A brave warrior.\nStrong and dependable.",
		Base: Stats{MaxHP: 30, HP: 30, MaxMP: 5, MP: 5, ATK: 8, DEF: 12, SPD: 4},
	},
	ClassMage: {
		Name: "Mage",
		Desc: "A master of magic.\nCasts powerful spells.",
		Base: Stats{MaxHP: 20, HP: 20, MaxMP: 20, MP: 20, ATK: 12, DEF: 4, SPD: 6},
	},
	ClassArcher: {
		Name: "Archer",
		Desc: "A swift ranger.\nStrikes before the foe.",
		Base: Stats{MaxHP: 22, HP: 22, MaxMP: 10, MP: 10, ATK: 10, DEF: 6, SPD: 12},
	},
}

// Player represents the player character with stats, inventory, and progression.
type Player struct {
	Class  Class
	Stats  Stats
	Level  int
	XP     int
	Coins  int
	Items  []Item
	Weapon *Item
	Armor  *Item
	Quests []Quest // active quests

	// Skills
	SkillPoints int            // unspent SP (earn 1 per level up)
	Skills      []PlayerSkill  // learned skills

	// Exploration tracking
	OpenedChests  map[string]bool // "area:x:y" -> true for opened chests
	FairyBlessing bool            // true if player received the fairy's blessing

	// Combat buffs (reset each combat)
	ATKBuff     int // bonus ATK from War Cry, wears off
	ATKBuffTurns int
	AegisActive bool // halves next hit taken
	PoisonDmg   int  // poison damage per turn on enemy (set by combat engine)
	PoisonTurns int
}

// AddItem adds an item to the player's inventory.
func (p *Player) AddItem(item Item) {
	p.Items = append(p.Items, item)
}

// UsePotion consumes the first potion in inventory and heals HP.
// Returns the item used, or nil if no potions available.
func (p *Player) UsePotion() *Item {
	for i, item := range p.Items {
		if item.Type == ItemConsumable {
			p.Stats.HP += item.StatBoost
			if p.Stats.HP > p.Stats.MaxHP {
				p.Stats.HP = p.Stats.MaxHP
			}
			// Remove from inventory
			p.Items = append(p.Items[:i], p.Items[i+1:]...)
			return &item
		}
	}
	return nil
}

// Equip sets a weapon or armor, returning the previously equipped item (if any).
func (p *Player) Equip(item Item) *Item {
	var old *Item
	switch item.Type {
	case ItemWeapon:
		old = p.Weapon
		cp := item
		p.Weapon = &cp
	case ItemArmor:
		old = p.Armor
		cp := item
		p.Armor = &cp
	}
	return old
}

// ActiveQuest returns the first incomplete quest, or nil.
func (p *Player) ActiveQuest() *Quest {
	for i := range p.Quests {
		if !p.Quests[i].Done {
			return &p.Quests[i]
		}
	}
	return nil
}

// CompleteQuest marks the active quest as done and grants rewards.
func (p *Player) CompleteQuest(q *Quest) {
	q.Done = true
	p.GainXP(q.RewardXP)
	p.Coins += q.RewardCoin
}

// NewPlayer creates a player with the given class's base stats.
func NewPlayer(class Class) *Player {
	info := ClassTable[class]
	return &Player{
		Class:        class,
		Stats:        info.Base,
		Level:        1,
		Coins:        50, // starting gold
		OpenedChests: map[string]bool{},
	}
}

// XPToNextLevel returns how much XP is needed to reach the next level.
// Formula: 10 * level^1.5 (rounded). This gives a gentle early curve
// that steepens later — classic RPG progression.
func (p *Player) XPToNextLevel() int {
	// Simple formula: each level needs more XP
	return p.Level * p.Level * 10
}

// GainXP adds experience and triggers level-ups.
func (p *Player) GainXP(amount int) bool {
	p.XP += amount
	leveledUp := false
	for p.XP >= p.XPToNextLevel() {
		p.XP -= p.XPToNextLevel()
		p.LevelUp()
		leveledUp = true
	}
	return leveledUp
}

// LevelUp increases the player's level and stats based on class growth rates.
func (p *Player) LevelUp() {
	p.Level++
	p.SkillPoints++ // earn 1 SP per level for the skill tree
	// Growth rates differ by class — this is where class identity shines
	switch p.Class {
	case ClassKnight:
		p.Stats.MaxHP += 5
		p.Stats.ATK += 2
		p.Stats.DEF += 3
		p.Stats.SPD += 1
	case ClassMage:
		p.Stats.MaxHP += 2
		p.Stats.MaxMP += 5
		p.Stats.ATK += 3
		p.Stats.DEF += 1
		p.Stats.SPD += 2
	case ClassArcher:
		p.Stats.MaxHP += 3
		p.Stats.ATK += 2
		p.Stats.DEF += 2
		p.Stats.SPD += 3
	}
	// Full heal on level up (a kind gift to the player)
	p.Stats.HP = p.Stats.MaxHP
	p.Stats.MP = p.Stats.MaxMP
}

// SkillLevel returns the current level of a skill (0 = not learned).
func (p *Player) SkillLevel(id SkillID) int {
	for _, s := range p.Skills {
		if s.ID == id {
			return s.Level
		}
	}
	return 0
}

// LearnSkill spends SP to learn or upgrade a skill. Returns true on success.
func (p *Player) LearnSkill(def SkillDef) bool {
	currentLvl := p.SkillLevel(def.ID)
	if currentLvl >= def.MaxLevel {
		return false // already maxed
	}
	cost := def.SPCost[currentLvl] // cost for next level
	if p.SkillPoints < cost {
		return false // not enough SP
	}
	p.SkillPoints -= cost

	// Update or add skill
	for i, s := range p.Skills {
		if s.ID == def.ID {
			p.Skills[i].Level++
			return true
		}
	}
	p.Skills = append(p.Skills, PlayerSkill{ID: def.ID, Level: 1})
	return true
}

// LearnedSkills returns skill defs with level > 0, for use in combat menus.
func (p *Player) LearnedSkills() []SkillDef {
	tree := ClassSkillTree(p.Class)
	var learned []SkillDef
	for _, def := range tree {
		if p.SkillLevel(def.ID) > 0 {
			learned = append(learned, def)
		}
	}
	return learned
}

// ResetCombatBuffs clears temporary buffs at the start of combat.
func (p *Player) ResetCombatBuffs() {
	p.ATKBuff = 0
	p.ATKBuffTurns = 0
	p.AegisActive = false
	p.PoisonDmg = 0
	p.PoisonTurns = 0
}

// EffectiveATK returns ATK including weapon bonus and temporary buffs.
func (p *Player) EffectiveATK() int {
	atk := p.Stats.ATK
	if p.Weapon != nil {
		atk += p.Weapon.StatBoost
	}
	atk += p.ATKBuff
	return atk
}

// EffectiveDEF returns DEF including armor bonus.
func (p *Player) EffectiveDEF() int {
	def := p.Stats.DEF
	if p.Armor != nil {
		def += p.Armor.StatBoost
	}
	return def
}
