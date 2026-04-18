package entity

import "fmt"

// ItemType categorizes items for equip logic and UI display.
//
// THEORY — Six-slot equipment model (Final Fantasy / Diablo):
// Classic JRPGs give each body part its own slot. With 6 slots the player
// makes 6 independent gear choices, and each loot drop can potentially
// upgrade any one of them. This dramatically increases the percentage of
// "interesting" loot — in a 2-slot system most drops are irrelevant, but
// with 6 slots there's almost always *something* worth comparing.
type ItemType int

const (
	ItemWeapon     ItemType = iota // ATK bonus
	ItemArmor                      // DEF bonus (body)
	ItemHelmet                     // DEF bonus (head)
	ItemBoots                      // DEF bonus (feet), some give SPD
	ItemShield                     // DEF bonus (off-hand)
	ItemAccessory                  // misc bonus (ring/amulet — HP, ATK, DEF, or SPD)
	ItemConsumable                 // single-use (potions, ethers)
)

// ConsumableType distinguishes what stat a consumable restores.
//
// THEORY — HP vs MP potions as resource tension:
// In RPGs with an MP system (Final Fantasy, Dragon Quest), having separate
// HP potions and MP ethers creates a resource management mini-game during
// combat. Do you heal HP to survive the next hit, or restore MP to cast
// the spell that ends the fight? This tension makes the "Item" command a
// meaningful strategic choice rather than a simple panic button.
type ConsumableType int

const (
	ConsumeHP       ConsumableType = iota // restores HP (Potion, Hi Potion, Elixir)
	ConsumeMP                             // restores MP (Ether, Hi Ether)
	ConsumeAntidote                       // cures Poison/Burn/Bleed DoTs
	ConsumeSmoke                          // guaranteed flee from non-boss combat
	ConsumeATKBuff                        // grants temporary ATK boost in combat
	ConsumeDEFBuff                        // grants temporary DEF boost in combat
)

// Rarity represents the quality tier of an item. Higher rarity means better
// stats (via multiplier) and additional bonus effects.
//
// THEORY — The 5-tier color-coded rarity system (Diablo / WoW / Destiny):
// Rarity is an orthogonal power axis to item tier. A "Leather Cap" has base
// stats, but a *Rare* Leather Cap gets 1.3× those stats plus an extra bonus.
// This creates excitement at every loot drop — even familiar items can roll
// high rarity. The 5-color spectrum is universally recognized by RPG players:
//
//	Common (white):     1.0× stats, no bonus  — baseline
//	Uncommon (green):   1.15× stats, +small bonus  — "nice find"
//	Rare (blue):        1.3× stats, +medium bonus  — "this is good!"
//	Epic (purple):      1.5× stats, +large bonus  — "whoa, keep this!"
//	Legendary (gold):   1.8× stats, +huge bonus  — "once in a lifetime"
//
// The multipliers compound with reinforcement (+5% per level), so a
// Legendary +3 item is dramatically stronger than a Common +3.
type Rarity int

const (
	RarityCommon    Rarity = iota // white — no bonus
	RarityUncommon                // green — slight edge
	RarityRare                    // blue — noticeably strong
	RarityEpic                    // purple — elite gear
	RarityLegendary               // gold — best in slot
)

// RarityName returns a display string for the rarity tier.
func (r Rarity) RarityName() string {
	switch r {
	case RarityUncommon:
		return "Uncommon"
	case RarityRare:
		return "Rare"
	case RarityEpic:
		return "Epic"
	case RarityLegendary:
		return "Legendary"
	default:
		return "Common"
	}
}

// RarityMultiplier returns the stat scaling factor for a rarity tier.
// Applied to StatBoost in EffectiveStatBoost().
func (r Rarity) RarityMultiplier() float64 {
	switch r {
	case RarityUncommon:
		return 1.15
	case RarityRare:
		return 1.3
	case RarityEpic:
		return 1.5
	case RarityLegendary:
		return 1.8
	default:
		return 1.0
	}
}

// RarityBonusMultiplier returns a multiplier for the BonusStat granted by
// rarity. Common items get no rarity bonus. Higher rarities add or amplify
// the item's secondary stat.
//
// THEORY — Additive rarity bonus:
// If an item already has a BonusStat (e.g., boots with +3 SPD), rarity
// scales it up. If it has no BonusStat, rarity can grant one — but we
// handle that at drop time by injecting a random bonus, not here.
// This function just scales existing bonuses.
func (r Rarity) RarityBonusMultiplier() float64 {
	switch r {
	case RarityUncommon:
		return 1.2
	case RarityRare:
		return 1.5
	case RarityEpic:
		return 1.8
	case RarityLegendary:
		return 2.5
	default:
		return 1.0
	}
}

// Item represents an equippable or consumable game item.
//
// THEORY — Class restrictions:
// In many RPGs (Final Fantasy, Dragon Quest), equipment is class-locked: a Mage
// can't equip a greatsword, and a Knight can't wear robes. This creates meaningful
// loot differentiation — finding a rare staff is exciting for a Mage but vendor
// trash for a Knight. We use ClassRestrict: -1 means "any class can equip",
// 0/1/2 means "only that class". This encourages the player to replay with
// different classes to experience different gear sets.
//
// THEORY — Level requirements (hard lock):
// Items can have a minimum level to equip (LevelReq). The player literally
// cannot equip the item until they reach that level. This is the WoW/Diablo
// model: finding an amazing helm you can't wear yet creates a "carrot on a
// stick" that motivates grinding. The binary unlock moment (finally hitting
// the level and equipping it) is deeply satisfying. Contrast with soft
// penalties where there's no clear unlock — just a gradual improvement.
//
// THEORY — Secondary stat (BonusStat):
// Accessories and boots often grant a secondary effect: SPD on boots,
// MaxHP on an amulet, etc. BonusStat holds this value, and BonusType
// says which stat it applies to. For simple items (weapons, basic armor),
// BonusStat is 0 and ignored. This lets one Item struct serve all roles
// without needing an effects system.
type Item struct {
	Name          string
	Type          ItemType
	Rarity        Rarity         // quality tier: Common through Legendary
	Consumable    ConsumableType // only meaningful when Type == ItemConsumable
	StatBoost     int            // ATK for weapons, DEF for armor/helm/boots/shield, restore for consumables
	BonusStat     int            // secondary stat value (e.g., SPD on boots, MaxHP on accessory)
	BonusType     BonusStatType  // which stat BonusStat applies to
	Price         int
	LevelReq      int // minimum player level to equip (0 = no requirement)
	ClassRestrict int // -1 = any class, 0 = Knight, 1 = Mage, 2 = Archer
	EnhanceLevel  int // reinforcement level (+0, +1, +2, ...)
}

func (i *Item) String() string {
	return fmt.Sprintf("%s(%s)", i.DisplayName(), i.Rarity)
}

// BonusStatType identifies which stat an item's BonusStat applies to.
type BonusStatType int

const (
	BonusNone BonusStatType = iota // no secondary stat
	BonusHP                        // +MaxHP
	BonusMP                        // +MaxMP
	BonusATK                       // +ATK
	BonusDEF                       // +DEF
	BonusSPD                       // +SPD
)

// CanEquip checks if the given class and level can equip this item.
// Class restriction: -1 = any class. Level restriction: hard lock.
func (it *Item) CanEquip(class Class) bool {
	if it.ClassRestrict < 0 {
		return true // universal item
	}
	return it.ClassRestrict == int(class)
}

// MeetsLevelReq checks if the player level satisfies this item's level gate.
// Separated from CanEquip so the UI can show "Lv.X required" distinctly
// from "Wrong class" — different error messages for different problems.
func (it *Item) MeetsLevelReq(playerLevel int) bool {
	return playerLevel >= it.LevelReq
}

// EffectiveStatBoost returns the stat boost after rarity scaling and reinforcement.
//
// THEORY — Multiplicative stacking: rarity × reinforcement:
// The final stat is: base × rarityMult × (1.05^enhanceLevel).
// Rarity and reinforcement multiply together, creating a power curve where
// a Legendary +3 item is dramatically stronger than a Common +3:
//
//	Common  base 10 +3 → 10 × 1.0 × 1.157 = 11
//	Legend. base 10 +3 → 10 × 1.8 × 1.157 = 20
//
// This multiplicative relationship makes both rarity AND reinforcement
// feel impactful. Neither one alone defines the item's power — you want
// both a high rarity AND high enhancement for the best gear.
func (it *Item) EffectiveStatBoost() int {
	stat := float64(it.StatBoost)
	// Apply rarity multiplier
	stat *= it.Rarity.RarityMultiplier()
	// Apply reinforcement compounding
	for i := 0; i < it.EnhanceLevel; i++ {
		stat *= 1.05
	}
	return int(stat)
}

// EffectiveBonusStat returns the secondary stat after rarity scaling.
func (it *Item) EffectiveBonusStat() int {
	if it.BonusStat <= 0 {
		return 0
	}
	return int(float64(it.BonusStat) * it.Rarity.RarityBonusMultiplier())
}

// DisplayName returns the item name with enhancement suffix.
// A +3 Iron Sword shows as "Iron Sword +3".
// Rarity is NOT included in the name — it's conveyed via text color in the UI.
// This keeps names short enough to fit on our 320px-wide screen.
func (it *Item) DisplayName() string {
	name := it.Name
	if it.EnhanceLevel <= 0 {
		return name
	}
	// Manual int-to-string to avoid importing strconv
	n := it.EnhanceLevel
	digits := make([]byte, 0, 4)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return name + " +" + string(digits)
}

// ReinforceCost returns the gold cost to reinforce from the current level
// to the next level. Formula: base_price × 5^(currentLevel + 1).
//
// THEORY — Rolling price model:
// Each reinforcement costs 5× the item's current "effective price", and
// the cost becomes the new effective price. For a 40G sword:
//
//	+0→+1:  40×5   =   200G  (effective price is now 200G)
//	+1→+2: 200×5   =  1000G  (effective price is now 1000G)
//	+2→+3: 1000×5  =  5000G
//	+3→+4: 5000×5  = 25000G
//
// Compared to the old price^(level+1) formula, this is far gentler:
// the old model charged 64000G for +3 on a 40G sword, the new model
// charges 5000G. The 5× multiplier still creates an exponential curve
// that acts as a soft cap, but the base is small enough that the first
// 2-3 levels feel affordable and exciting, while +5 and beyond become
// a serious gold sink for dedicated grinders.
func (it *Item) ReinforceCost() int {
	cost := it.Price
	for i := 0; i <= it.EnhanceLevel; i++ {
		cost *= 5
		if cost > 9999999 {
			return 9999999 // overflow protection
		}
	}
	if cost < 1 {
		return 1
	}
	return cost
}

// ReinforceSuccessRate returns the probability (0.0–1.0) of a successful
// reinforcement at the current level. Starts at 100% for +0→+1, then
// multiplies by 0.85 for each subsequent level.
//
// THEORY — Decreasing success rate as risk/reward:
//
//	+0→+1: 100%  (guaranteed first upgrade — hooks the player)
//	+1→+2: 85%   (slight risk — "probably fine")
//	+2→+3: 72%   (noticeable risk — "should I save first?")
//	+5→+6: 44%   (coin flip territory — genuine tension)
//	+9→+10: 20%  (heroic attempt — massive dopamine if it lands)
//
// This curve is borrowed from Korean MMOs where the decreasing odds
// make each successful high-level reinforce a genuine achievement.
func (it *Item) ReinforceSuccessRate() float64 {
	rate := 1.0
	for i := 0; i < it.EnhanceLevel; i++ {
		rate *= 0.85
	}
	return rate
}

// ReinforceSuccessPct returns the success rate as an integer percentage.
func (it *Item) ReinforceSuccessPct() int {
	return int(it.ReinforceSuccessRate() * 100)
}
