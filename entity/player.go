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
//
// THEORY — Base stats preview the growth curve:
// Starting stats should let the player immediately *feel* their class
// identity in the first few fights. The Knight can tank 3-4 hits where
// others die in 2. The Mage starts with a deep MP pool for multiple
// spells. The Archer goes first against every early monster.
var ClassTable = map[Class]ClassInfo{
	ClassKnight: {
		Name: "Knight",
		Desc: "A brave warrior.\nStrong and dependable.",
		Base: Stats{MaxHP: 35, HP: 35, MaxMP: 5, MP: 5, ATK: 9, DEF: 12, SPD: 3},
	},
	ClassMage: {
		Name: "Mage",
		Desc: "A master of magic.\nCasts powerful spells.",
		Base: Stats{MaxHP: 16, HP: 16, MaxMP: 24, MP: 24, ATK: 11, DEF: 3, SPD: 5},
	},
	ClassArcher: {
		Name: "Archer",
		Desc: "A swift ranger.\nStrikes before the foe.",
		Base: Stats{MaxHP: 20, HP: 20, MaxMP: 8, MP: 8, ATK: 10, DEF: 5, SPD: 14},
	},
}

// Player represents the player character with stats, inventory, and progression.
type Player struct {
	Class  Class
	Stats  Stats
	Level  int
	XP     int
	Coins  int
	Items     []Item
	Weapon    *Item
	Armor     *Item
	Helmet    *Item
	Boots     *Item
	Shield    *Item
	Accessory *Item
	Quests []Quest // active quests

	// Skills
	SkillPoints int            // unspent SP (earn 1 per level up)
	Skills      []PlayerSkill  // learned skills

	// Day/night cycle
	DayNight *DayNight

	// Exploration tracking
	OpenedChests  map[string]bool // "area:x:y" -> true for opened chests
	FairyBlessing bool            // true if player received the fairy's blessing

	// World progression
	BossDefeated map[string]bool // "dragon" -> true, etc. Tracks which bosses are down.

	// Combat buffs are now managed by combat.Engine's StatusEffect system.
	// The old ad-hoc fields (ATKBuff, AegisActive, PoisonDmg, EnemyStunned)
	// have been replaced by unified []StatusEffect slices on the Engine.
	// Only the Inn buff remains here because it persists across combats.

	// Inn buff — persists across combats, counts down each fight.
	// THEORY: The Inn buff gives a reason to spend gold before a boss attempt.
	// It lasts for a fixed number of combats, not turns, so you can't waste it
	// on weak encounters and save it for the boss. This encourages the player
	// to head straight for tough fights after buying the buff.
	InnATKBuff    int // bonus ATK from inn meal
	InnDEFBuff    int // bonus DEF from inn meal
	InnBuffFights int // fights remaining (decremented at combat start)
}

// AddItem adds an item to the player's inventory.
func (p *Player) AddItem(item Item) {
	p.Items = append(p.Items, item)
}

// Consumables returns a list of (inventory index, item) pairs for all
// consumable items the player is carrying. The combat item menu displays
// this list and the player picks by position.
//
// THEORY — Index indirection:
// We return the *original inventory index* alongside each item so that
// UseItem can remove the correct entry. The item sub-menu shows a
// filtered view (only consumables), but deletion must target the
// unfiltered Items slice. This is the same pattern RPGs use internally:
// the "bag" is one flat list, but screens show filtered views (weapons,
// armor, consumables) while keeping stable references back to the source.
func (p *Player) Consumables() []struct {
	Idx  int
	Item Item
} {
	var result []struct {
		Idx  int
		Item Item
	}
	for i, item := range p.Items {
		if item.Type == ItemConsumable {
			result = append(result, struct {
				Idx  int
				Item Item
			}{i, item})
		}
	}
	return result
}

// UseItem consumes the item at the given inventory index.
// Handles HP potions, MP ethers, and self-contained consumables.
// Returns the item used, or nil if the index is invalid.
// Note: Combat-specific consumables (Antidote, Smoke Bomb, buff potions)
// are consumed here but their effects are applied by the combat engine,
// which checks the returned item's ConsumableType.
func (p *Player) UseItem(invIdx int) *Item {
	if invIdx < 0 || invIdx >= len(p.Items) {
		return nil
	}
	item := p.Items[invIdx]
	if item.Type != ItemConsumable {
		return nil
	}

	// Apply the restore based on consumable type
	switch item.Consumable {
	case ConsumeHP:
		p.Stats.HP += item.StatBoost
		if p.Stats.HP > p.Stats.MaxHP {
			p.Stats.HP = p.Stats.MaxHP
		}
	case ConsumeMP:
		p.Stats.MP += item.StatBoost
		if p.Stats.MP > p.Stats.MaxMP {
			p.Stats.MP = p.Stats.MaxMP
		}
	// Antidote, Smoke Bomb, buff potions: the item is consumed here but the
	// combat engine applies the actual effect (clearing DoTs, granting flee,
	// adding buffs to the StatusEffect system).
	case ConsumeAntidote, ConsumeSmoke, ConsumeATKBuff, ConsumeDEFBuff:
		// No stat change on player — combat engine handles these
	}

	// Remove from inventory
	p.Items = append(p.Items[:invIdx], p.Items[invIdx+1:]...)
	return &item
}

// UsePotion consumes the first potion in inventory and heals HP.
// Kept for backward compatibility (e.g., NPC healing events).
func (p *Player) UsePotion() *Item {
	for i, item := range p.Items {
		if item.Type == ItemConsumable && item.Consumable == ConsumeHP {
			return p.UseItem(i)
		}
		_ = item // silence
	}
	return nil
}

// Equip sets gear in the appropriate slot, returning the previously equipped
// item (if any). Handles all 6 equipment types.
//
// THEORY — Slot-based equip as a strategy pattern:
// Each ItemType maps to exactly one player field. The switch statement acts
// like a dispatch table. Adding a new slot = adding one case here, one field
// on Player, and one entry in the UI. The rest of the game (combat, save,
// stats) reads from the fields generically.
func (p *Player) Equip(item Item) *Item {
	var old *Item
	cp := item
	switch item.Type {
	case ItemWeapon:
		old = p.Weapon
		p.Weapon = &cp
	case ItemArmor:
		old = p.Armor
		p.Armor = &cp
	case ItemHelmet:
		old = p.Helmet
		p.Helmet = &cp
	case ItemBoots:
		old = p.Boots
		p.Boots = &cp
	case ItemShield:
		old = p.Shield
		p.Shield = &cp
	case ItemAccessory:
		old = p.Accessory
		p.Accessory = &cp
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
		DayNight:      NewDayNight(),
		OpenedChests:  map[string]bool{},
		BossDefeated:  map[string]bool{},
	}
}

// MaxLevel is the absolute level cap. At 99 the XP bar shows "MAX" and
// all XP gains are discarded. This prevents stat overflow and gives the
// endgame a sense of finality — a common RPG convention (Final Fantasy,
// Pokemon, Dragon Quest all cap at ~100).
const MaxLevel = 99

// IsMaxLevel returns whether the player has reached the level cap.
func (p *Player) IsMaxLevel() bool {
	return p.Level >= MaxLevel
}

// XPToNextLevel returns how much XP is needed to reach the next level.
// Formula: 10 * level^2. This gives a gentle early curve
// that steepens later — classic RPG progression. Returns 0 at max level.
func (p *Player) XPToNextLevel() int {
	if p.Level >= MaxLevel {
		return 0
	}
	return p.Level * p.Level * 10
}

// GainXP adds experience and triggers level-ups.
// At max level, XP is discarded — the player has reached godhood.
func (p *Player) GainXP(amount int) bool {
	if p.Level >= MaxLevel {
		p.XP = 0
		return false
	}
	p.XP += amount
	leveledUp := false
	for p.Level < MaxLevel && p.XP >= p.XPToNextLevel() {
		p.XP -= p.XPToNextLevel()
		p.LevelUp()
		leveledUp = true
	}
	if p.Level >= MaxLevel {
		p.XP = 0 // discard overflow at cap
	}
	return leveledUp
}

// LevelUp increases the player's level and stats based on class growth rates.
//
// THEORY — Asymmetric stat budgets per class:
// Each class gets a fixed "budget" of stat points per level, but the
// distribution is wildly different. This is what makes replaying as a
// different class feel like a different game, not just a reskin.
//
//   Knight: HP+6, MP+1, ATK+3, DEF+3, SPD+1  (tank — absorbs hits, low magic)
//   Mage:   HP+1, MP+6, ATK+3, DEF+1, SPD+2  (glass cannon — huge MP, fragile)
//   Archer: HP+2, MP+1, ATK+3, DEF+1, SPD+4  (speedster — always acts first)
//
// Note that ATK serves double duty: physical damage for Knight/Archer,
// spell power for Mage. So giving Mage +3 ATK means their spells scale
// with levels, not just their MP pool.
//
// The Knight's low MP growth (+1/level) means they'll run dry fast in
// long fights — they need HP potions to survive and Ethers to keep using
// Shield Bash. The Mage's +1 HP means a single boss hit can kill them —
// they *must* use Defend and Aegis strategically. The Archer's +4 SPD
// compounds over levels: by endgame they'll act first in every single
// fight, which is an enormous tactical advantage despite lower bulk.
func (p *Player) LevelUp() {
	if p.Level >= MaxLevel {
		return
	}
	p.Level++
	p.SkillPoints++ // earn 1 SP per level for the skill tree

	switch p.Class {
	case ClassKnight:
		// The fortress: massive HP and DEF, almost no MP or speed.
		// Plays like a war of attrition — outlast the enemy.
		p.Stats.MaxHP += 6
		p.Stats.MaxMP += 1
		p.Stats.ATK += 3
		p.Stats.DEF += 3
		p.Stats.SPD += 1
	case ClassMage:
		// The glass cannon: enormous MP pool, devastating spells,
		// but crumbles if anything touches them. High risk, high reward.
		p.Stats.MaxHP += 1
		p.Stats.MaxMP += 6
		p.Stats.ATK += 3
		p.Stats.DEF += 1
		p.Stats.SPD += 2
	case ClassArcher:
		// The speedster: always strikes first, hits hard, but can't
		// take a punch. Relies on killing before being killed.
		p.Stats.MaxHP += 2
		p.Stats.MaxMP += 1
		p.Stats.ATK += 3
		p.Stats.DEF += 1
		p.Stats.SPD += 4
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

// ResetCombatBuffs is a no-op now that combat buffs are managed by the Engine's
// StatusEffect system. The Engine creates fresh effect slices per combat, so
// there's nothing to reset on the Player. Kept for API compatibility.
func (p *Player) ResetCombatBuffs() {
	// Combat effects now live on combat.Engine, not on Player.
	// Inn buffs (InnATKBuff, InnDEFBuff, InnBuffFights) persist across combats
	// and are managed separately by TickInnBuff().
}

// EffectiveATK returns ATK including weapon bonus, equipment bonuses, and inn buff.
// NOTE: Combat-specific buffs (War Cry, etc.) are now applied by the combat Engine
// via StatusEffect — they're added on top of this value during combat calculations.
// Uses EffectiveStatBoost() to account for reinforcement levels.
func (p *Player) EffectiveATK() int {
	atk := p.Stats.ATK
	if p.Weapon != nil {
		atk += p.Weapon.EffectiveStatBoost()
	}
	// Accessory may grant bonus ATK
	atk += p.equipBonusFor(BonusATK)
	atk += p.InnATKBuff
	return atk
}

// EffectiveDEF returns DEF including all defensive equipment, bonuses, and inn buff.
//
// THEORY — DEF stacking across 4 slots:
// Armor, Helmet, Boots, and Shield all contribute DEF. This is the classic
// "paper doll" model from Diablo/WoW: each slot adds a chunk of defense.
// The sum creates a satisfying "tankiness" curve where each new piece is
// a visible improvement. Accessories can also grant bonus DEF.
func (p *Player) EffectiveDEF() int {
	def := p.Stats.DEF
	if p.Armor != nil {
		def += p.Armor.EffectiveStatBoost()
	}
	if p.Helmet != nil {
		def += p.Helmet.EffectiveStatBoost()
	}
	if p.Boots != nil {
		def += p.Boots.EffectiveStatBoost()
	}
	if p.Shield != nil {
		def += p.Shield.EffectiveStatBoost()
	}
	// Accessory/boots may grant bonus DEF
	def += p.equipBonusFor(BonusDEF)
	def += p.InnDEFBuff
	return def
}

// EffectiveSPD returns SPD including equipment bonuses.
func (p *Player) EffectiveSPD() int {
	return p.Stats.SPD + p.equipBonusFor(BonusSPD)
}

// EquipBonusHP returns total MaxHP bonus from equipment secondary stats.
func (p *Player) EquipBonusHP() int {
	return p.equipBonusFor(BonusHP)
}

// EquipBonusMP returns total MaxMP bonus from equipment secondary stats.
func (p *Player) EquipBonusMP() int {
	return p.equipBonusFor(BonusMP)
}

// equipBonusFor sums BonusStat from all equipment slots for a given BonusStatType.
// Uses EffectiveBonusStat() which includes rarity scaling.
func (p *Player) equipBonusFor(bt BonusStatType) int {
	total := 0
	for _, slot := range []*Item{p.Weapon, p.Armor, p.Helmet, p.Boots, p.Shield, p.Accessory} {
		if slot != nil && slot.BonusType == bt && slot.BonusStat > 0 {
			total += slot.EffectiveBonusStat()
		}
	}
	return total
}

// TickInnBuff decrements the inn buff fight counter. Called at combat start.
func (p *Player) TickInnBuff() {
	if p.InnBuffFights > 0 {
		p.InnBuffFights--
		if p.InnBuffFights <= 0 {
			p.InnATKBuff = 0
			p.InnDEFBuff = 0
		}
	}
}

// HasInnBuff returns whether the inn buff is active.
func (p *Player) HasInnBuff() bool {
	return p.InnBuffFights > 0
}

// InnHealCost returns the cost to heal at the inn, scaling with level.
// Scales gently: 10 gold at level 1, ~50 at level 10, ~200 at level 50.
func (p *Player) InnHealCost() int {
	return 5 + p.Level*4
}

// InnBuffCost returns the cost for the ATK+DEF buff meal.
func (p *Player) InnBuffCost() int {
	return 15 + p.Level*6
}
