// data/items.go — Static item definitions and shop inventory.
//
// THEORY — Data-driven items:
// All items are defined here as plain data. The shop screen reads from this
// table. Adding a new item = adding one entry. No logic changes needed.
//
// THEORY — Class-specific equipment:
// Each class has its own weapon and armor progression. This is a staple of
// JRPGs like Final Fantasy where a Fighter uses swords, a Black Mage uses
// rods, and a Thief uses daggers. It makes each class playthrough feel
// different and gives the shop meaningful variety.
//
// Equipment tiers (by StatBoost):
//   Tier 1 (cheap):   +3  — starter upgrade
//   Tier 2 (mid):     +6  — mid-game
//   Tier 3 (premium): +10 — late-game, expensive
//
// ClassRestrict: -1 = any class, 0 = Knight, 1 = Mage, 2 = Archer
package data

import (
	"math/rand"

	"cli_adventure/entity"
)

// ShopInventory is the list of items available at the merchant's shop.
// The shop shows all items but grays out ones the player's class can't equip.
var ShopInventory = []entity.Item{
	// === Knight weapons (swords) ===
	{Name: "Iron Sword", Type: entity.ItemWeapon, StatBoost: 3, Price: 40, ClassRestrict: 0},
	{Name: "Steel Sword", Type: entity.ItemWeapon, StatBoost: 6, Price: 100, ClassRestrict: 0},
	{Name: "Holy Blade", Type: entity.ItemWeapon, StatBoost: 10, Price: 250, ClassRestrict: 0},

	// === Mage weapons (staffs) ===
	{Name: "Oak Staff", Type: entity.ItemWeapon, StatBoost: 3, Price: 40, ClassRestrict: 1},
	{Name: "Rune Staff", Type: entity.ItemWeapon, StatBoost: 6, Price: 100, ClassRestrict: 1},
	{Name: "Arcane Rod", Type: entity.ItemWeapon, StatBoost: 10, Price: 250, ClassRestrict: 1},

	// === Archer weapons (bows) ===
	{Name: "Short Bow", Type: entity.ItemWeapon, StatBoost: 3, Price: 40, ClassRestrict: 2},
	{Name: "Long Bow", Type: entity.ItemWeapon, StatBoost: 6, Price: 100, ClassRestrict: 2},
	{Name: "Gale Bow", Type: entity.ItemWeapon, StatBoost: 10, Price: 250, ClassRestrict: 2},

	// === Knight armor (heavy) ===
	{Name: "Chain Mail", Type: entity.ItemArmor, StatBoost: 4, Price: 45, ClassRestrict: 0},
	{Name: "Plate Armor", Type: entity.ItemArmor, StatBoost: 8, Price: 120, ClassRestrict: 0},
	{Name: "Holy Armor", Type: entity.ItemArmor, StatBoost: 12, Price: 280, ClassRestrict: 0},

	// === Mage armor (robes) ===
	{Name: "Linen Robe", Type: entity.ItemArmor, StatBoost: 2, Price: 35, ClassRestrict: 1},
	{Name: "Silk Robe", Type: entity.ItemArmor, StatBoost: 5, Price: 90, ClassRestrict: 1},
	{Name: "Arcane Robe", Type: entity.ItemArmor, StatBoost: 9, Price: 240, ClassRestrict: 1},

	// === Archer armor (leather) ===
	{Name: "Leather Vest", Type: entity.ItemArmor, StatBoost: 3, Price: 35, ClassRestrict: 2},
	{Name: "Hide Armor", Type: entity.ItemArmor, StatBoost: 6, Price: 100, ClassRestrict: 2},
	{Name: "Wind Cloak", Type: entity.ItemArmor, StatBoost: 10, Price: 260, ClassRestrict: 2},

	// === HP Consumables (any class) ===
	{Name: "Potion", Type: entity.ItemConsumable, Consumable: entity.ConsumeHP, StatBoost: 15, Price: 15, ClassRestrict: -1},
	{Name: "Hi Potion", Type: entity.ItemConsumable, Consumable: entity.ConsumeHP, StatBoost: 40, Price: 40, ClassRestrict: -1},
	{Name: "Elixir", Type: entity.ItemConsumable, Consumable: entity.ConsumeHP, StatBoost: 99, Price: 100, ClassRestrict: -1},

	// === MP Consumables (any class) ===
	// Named after Final Fantasy's "Ether" tradition — the classic MP restore item.
	{Name: "Ether", Type: entity.ItemConsumable, Consumable: entity.ConsumeMP, StatBoost: 10, Price: 20, ClassRestrict: -1},
	{Name: "Hi Ether", Type: entity.ItemConsumable, Consumable: entity.ConsumeMP, StatBoost: 30, Price: 50, ClassRestrict: -1},

	// === Status effect consumables ===
	// THEORY — Tactical consumables create meaningful item choices:
	// Antidotes cure DoTs (essential against poison-happy enemies), Smoke Bombs
	// guarantee escape (a safety valve for when things go wrong), and buff potions
	// trade gold for power (useful pre-boss). These give the player more strategic
	// options in combat beyond "heal HP" and "restore MP".
	{Name: "Antidote", Type: entity.ItemConsumable, Consumable: entity.ConsumeAntidote, Price: 12, ClassRestrict: -1},
	{Name: "Smoke Bomb", Type: entity.ItemConsumable, Consumable: entity.ConsumeSmoke, Price: 30, ClassRestrict: -1},
	{Name: "Power Seed", Type: entity.ItemConsumable, Consumable: entity.ConsumeATKBuff, StatBoost: 5, Price: 40, ClassRestrict: -1},
	{Name: "Iron Pill", Type: entity.ItemConsumable, Consumable: entity.ConsumeDEFBuff, StatBoost: 5, Price: 40, ClassRestrict: -1},
}

// ShopForClass returns items the given class can buy (own gear + consumables).
// This filters the full inventory so the shop isn't overwhelming.
func ShopForClass(class entity.Class) []entity.Item {
	var items []entity.Item
	for _, item := range ShopInventory {
		if item.ClassRestrict < 0 || item.ClassRestrict == int(class) {
			items = append(items, item)
		}
	}
	return items
}

// BlacksmithInventory holds tier-2 biome-specific gear.
//
// THEORY — Boss-gated progression shops:
// In classic JRPGs (Chrono Trigger, FF6), each town near a new area has
// a shop with stronger gear. We compress that into one Blacksmith who
// unlocks stock as you defeat bosses. This creates a "the world reacts
// to your victories" feeling: beat the Dragon → the Blacksmith forges
// gear from dragon scales. Beat the Ice Wyrm → he crafts frost weapons.
// Each boss kill is rewarded not just with XP, but with access to a new
// tier of equipment.
var BlacksmithInventory = []struct {
	Item    entity.Item
	BossReq string // boss key required in BossDefeated map ("" = always available)
}{
	// --- Always available (starter upgrade) ---
	{entity.Item{Name: "Tempered Blade", Type: entity.ItemWeapon, StatBoost: 8, Price: 180, ClassRestrict: 0}, ""},
	{entity.Item{Name: "Crystal Staff", Type: entity.ItemWeapon, StatBoost: 8, Price: 180, ClassRestrict: 1}, ""},
	{entity.Item{Name: "Composite Bow", Type: entity.ItemWeapon, StatBoost: 8, Price: 180, ClassRestrict: 2}, ""},
	{entity.Item{Name: "Reinforced Mail", Type: entity.ItemArmor, StatBoost: 7, Price: 160, ClassRestrict: 0}, ""},
	{entity.Item{Name: "Enchanted Robe", Type: entity.ItemArmor, StatBoost: 6, Price: 150, ClassRestrict: 1}, ""},
	{entity.Item{Name: "Ranger Vest", Type: entity.ItemArmor, StatBoost: 7, Price: 160, ClassRestrict: 2}, ""},

	// --- Post-Dragon (east chain boss) ---
	{entity.Item{Name: "Dragon Fang", Type: entity.ItemWeapon, StatBoost: 14, Price: 400, ClassRestrict: 0}, "dragon"},
	{entity.Item{Name: "Ember Rod", Type: entity.ItemWeapon, StatBoost: 14, Price: 400, ClassRestrict: 1}, "dragon"},
	{entity.Item{Name: "Flame Bow", Type: entity.ItemWeapon, StatBoost: 14, Price: 400, ClassRestrict: 2}, "dragon"},
	{entity.Item{Name: "Dragonscale", Type: entity.ItemArmor, StatBoost: 15, Price: 450, ClassRestrict: 0}, "dragon"},
	{entity.Item{Name: "Fireweave Robe", Type: entity.ItemArmor, StatBoost: 12, Price: 380, ClassRestrict: 1}, "dragon"},
	{entity.Item{Name: "Wyrmhide Cloak", Type: entity.ItemArmor, StatBoost: 13, Price: 400, ClassRestrict: 2}, "dragon"},

	// --- Post-Ice Wyrm (north chain boss) ---
	{entity.Item{Name: "Frostbrand", Type: entity.ItemWeapon, StatBoost: 18, Price: 600, ClassRestrict: 0}, "ice_wyrm"},
	{entity.Item{Name: "Blizzard Staff", Type: entity.ItemWeapon, StatBoost: 18, Price: 600, ClassRestrict: 1}, "ice_wyrm"},
	{entity.Item{Name: "Icicle Bow", Type: entity.ItemWeapon, StatBoost: 18, Price: 600, ClassRestrict: 2}, "ice_wyrm"},

	// --- Post-Hydra (swamp/volcano boss) ---
	{entity.Item{Name: "Hydra Slayer", Type: entity.ItemWeapon, StatBoost: 22, Price: 800, ClassRestrict: 0}, "hydra"},
	{entity.Item{Name: "Venom Wand", Type: entity.ItemWeapon, StatBoost: 22, Price: 800, ClassRestrict: 1}, "hydra"},
	{entity.Item{Name: "Serpent Bow", Type: entity.ItemWeapon, StatBoost: 22, Price: 800, ClassRestrict: 2}, "hydra"},

	// --- Post-Sphinx (desert boss) --- Endgame tier
	{entity.Item{Name: "Pharaoh Blade", Type: entity.ItemWeapon, StatBoost: 28, Price: 1200, ClassRestrict: 0}, "sphinx"},
	{entity.Item{Name: "Ankh Staff", Type: entity.ItemWeapon, StatBoost: 28, Price: 1200, ClassRestrict: 1}, "sphinx"},
	{entity.Item{Name: "Sandstorm Bow", Type: entity.ItemWeapon, StatBoost: 28, Price: 1200, ClassRestrict: 2}, "sphinx"},
	{entity.Item{Name: "Sphinx Guard", Type: entity.ItemArmor, StatBoost: 24, Price: 1000, ClassRestrict: 0}, "sphinx"},
	{entity.Item{Name: "Pharaoh Robe", Type: entity.ItemArmor, StatBoost: 20, Price: 900, ClassRestrict: 1}, "sphinx"},
	{entity.Item{Name: "Desert Shroud", Type: entity.ItemArmor, StatBoost: 22, Price: 950, ClassRestrict: 2}, "sphinx"},
}

// BlacksmithForClass returns the blacksmith items the player can see,
// filtered by class and boss progression.
func BlacksmithForClass(class entity.Class, bossDefeated map[string]bool) []entity.Item {
	var items []entity.Item
	for _, entry := range BlacksmithInventory {
		// Boss gate check
		if entry.BossReq != "" {
			if bossDefeated == nil || !bossDefeated[entry.BossReq] {
				continue
			}
		}
		// Class filter
		if entry.Item.ClassRestrict >= 0 && entry.Item.ClassRestrict != int(class) {
			continue
		}
		items = append(items, entry.Item)
	}
	return items
}

// ---------- Loot-only equipment (Helmets, Boots, Shields, Accessories) ----------
//
// THEORY — Loot-only as exploration reward:
// New equipment slots (helmet, boots, shield, accessory) are found ONLY in
// chests and boss drops — never sold in shops. This makes exploration feel
// rewarding: every chest might contain a gear upgrade for one of 4 new slots.
// It also creates a scarcity dynamic where you can't just buy your way to
// full gear — you have to adventure for it. This is the Zelda model: key
// items come from dungeons, not shops.
//
// Each area has a loot pool appropriate to its difficulty. Items have level
// requirements that roughly match the area's recommended level, preventing
// a lucky low-level player from immediately equipping endgame gear.
//
// THEORY — Stat budget by slot:
//   Helmet:    Moderate DEF (50-70% of body armor for same tier)
//   Boots:     Low DEF + optional SPD bonus (mobility theme)
//   Shield:    High DEF but class-restricted (Knight gets best shields)
//   Accessory: Diverse bonuses (HP, MP, ATK, SPD) — the "wildcard" slot
//
// The accessory slot is intentionally the most varied. It's the slot where
// build diversity lives: a Knight might want an ATK ring for offense, or an
// HP amulet for survivability. This creates meaningful loot decisions even
// between items of similar power level.

// LootEntry pairs an item with the areas it can drop in.
type LootEntry struct {
	Item  entity.Item
	Areas []string // which area keys this can appear in ("forest", "cave", etc.)
}

// AreaLootTable contains all loot-only equipment that can drop from chests.
// Each entry lists which areas it can appear in.
var AreaLootTable = []LootEntry{
	// ═══════════════════════════════════════════════
	// EAST CHAIN — Forest / Cave / Lair (Lv 1-6)
	// ═══════════════════════════════════════════════

	// Helmets
	{entity.Item{Name: "Leather Cap", Type: entity.ItemHelmet, StatBoost: 2, Price: 20, ClassRestrict: -1},
		[]string{"forest", "cave"}},
	{entity.Item{Name: "Iron Helm", Type: entity.ItemHelmet, StatBoost: 4, Price: 60, LevelReq: 3, ClassRestrict: 0},
		[]string{"cave", "lair"}},
	{entity.Item{Name: "Wizard Hat", Type: entity.ItemHelmet, StatBoost: 2, Price: 50, LevelReq: 3, ClassRestrict: 1, BonusStat: 3, BonusType: entity.BonusMP},
		[]string{"cave", "lair"}},

	// Boots
	{entity.Item{Name: "Travel Boots", Type: entity.ItemBoots, StatBoost: 1, Price: 15, ClassRestrict: -1, BonusStat: 1, BonusType: entity.BonusSPD},
		[]string{"forest"}},
	{entity.Item{Name: "Iron Greaves", Type: entity.ItemBoots, StatBoost: 3, Price: 50, LevelReq: 3, ClassRestrict: 0},
		[]string{"cave", "lair"}},

	// Shields
	{entity.Item{Name: "Wooden Shield", Type: entity.ItemShield, StatBoost: 3, Price: 25, ClassRestrict: 0},
		[]string{"forest", "cave"}},
	{entity.Item{Name: "Buckler", Type: entity.ItemShield, StatBoost: 2, Price: 20, ClassRestrict: -1},
		[]string{"forest"}},

	// Accessories
	{entity.Item{Name: "HP Ring", Type: entity.ItemAccessory, StatBoost: 0, Price: 30, ClassRestrict: -1, BonusStat: 5, BonusType: entity.BonusHP},
		[]string{"forest", "cave"}},
	{entity.Item{Name: "Power Charm", Type: entity.ItemAccessory, StatBoost: 0, Price: 60, LevelReq: 4, ClassRestrict: -1, BonusStat: 2, BonusType: entity.BonusATK},
		[]string{"cave", "lair"}},

	// ═══════════════════════════════════════════════
	// NORTH CHAIN — Frozen Path / Snow Mountains / Ice Cavern (Lv 8-12)
	// ═══════════════════════════════════════════════

	// Helmets
	{entity.Item{Name: "Fur Hood", Type: entity.ItemHelmet, StatBoost: 5, Price: 100, LevelReq: 8, ClassRestrict: -1},
		[]string{"frozen_path", "snow_mountains"}},
	{entity.Item{Name: "Glacial Helm", Type: entity.ItemHelmet, StatBoost: 8, Price: 200, LevelReq: 10, ClassRestrict: 0},
		[]string{"snow_mountains", "ice_cavern"}},
	{entity.Item{Name: "Frost Crown", Type: entity.ItemHelmet, StatBoost: 6, Price: 180, LevelReq: 10, ClassRestrict: 1, BonusStat: 5, BonusType: entity.BonusMP},
		[]string{"ice_cavern"}},

	// Boots
	{entity.Item{Name: "Snow Treads", Type: entity.ItemBoots, StatBoost: 4, Price: 90, LevelReq: 8, ClassRestrict: -1, BonusStat: 2, BonusType: entity.BonusSPD},
		[]string{"frozen_path", "snow_mountains"}},
	{entity.Item{Name: "Blizzard Boots", Type: entity.ItemBoots, StatBoost: 6, Price: 160, LevelReq: 10, ClassRestrict: -1, BonusStat: 3, BonusType: entity.BonusSPD},
		[]string{"ice_cavern"}},

	// Shields
	{entity.Item{Name: "Ice Barrier", Type: entity.ItemShield, StatBoost: 7, Price: 150, LevelReq: 8, ClassRestrict: 0},
		[]string{"frozen_path", "snow_mountains"}},
	{entity.Item{Name: "Tome of Frost", Type: entity.ItemShield, StatBoost: 3, Price: 120, LevelReq: 8, ClassRestrict: 1, BonusStat: 4, BonusType: entity.BonusMP},
		[]string{"frozen_path", "snow_mountains"}},
	{entity.Item{Name: "Frost Quiver", Type: entity.ItemShield, StatBoost: 2, Price: 110, LevelReq: 8, ClassRestrict: 2, BonusStat: 3, BonusType: entity.BonusATK},
		[]string{"frozen_path", "snow_mountains"}},

	// Accessories
	{entity.Item{Name: "Yeti Fang", Type: entity.ItemAccessory, StatBoost: 0, Price: 130, LevelReq: 8, ClassRestrict: -1, BonusStat: 4, BonusType: entity.BonusATK},
		[]string{"snow_mountains", "ice_cavern"}},
	{entity.Item{Name: "Frost Amulet", Type: entity.ItemAccessory, StatBoost: 0, Price: 140, LevelReq: 9, ClassRestrict: -1, BonusStat: 10, BonusType: entity.BonusHP},
		[]string{"ice_cavern"}},

	// ═══════════════════════════════════════════════
	// SOUTH CHAIN — Swamp / Volcano (Lv 12-15)
	// ═══════════════════════════════════════════════

	// Helmets
	{entity.Item{Name: "Marsh Visor", Type: entity.ItemHelmet, StatBoost: 7, Price: 160, LevelReq: 12, ClassRestrict: -1},
		[]string{"swamp"}},
	{entity.Item{Name: "Magma Crown", Type: entity.ItemHelmet, StatBoost: 10, Price: 300, LevelReq: 14, ClassRestrict: -1, BonusStat: 3, BonusType: entity.BonusATK},
		[]string{"volcano"}},

	// Boots
	{entity.Item{Name: "Bog Walkers", Type: entity.ItemBoots, StatBoost: 5, Price: 140, LevelReq: 12, ClassRestrict: -1, BonusStat: 3, BonusType: entity.BonusSPD},
		[]string{"swamp"}},
	{entity.Item{Name: "Lava Treads", Type: entity.ItemBoots, StatBoost: 8, Price: 250, LevelReq: 14, ClassRestrict: -1, BonusStat: 4, BonusType: entity.BonusSPD},
		[]string{"volcano"}},

	// Shields
	{entity.Item{Name: "Swamp Bark", Type: entity.ItemShield, StatBoost: 9, Price: 200, LevelReq: 12, ClassRestrict: 0},
		[]string{"swamp"}},
	{entity.Item{Name: "Inferno Guard", Type: entity.ItemShield, StatBoost: 12, Price: 350, LevelReq: 14, ClassRestrict: 0},
		[]string{"volcano"}},
	{entity.Item{Name: "Ember Tome", Type: entity.ItemShield, StatBoost: 4, Price: 180, LevelReq: 12, ClassRestrict: 1, BonusStat: 6, BonusType: entity.BonusMP},
		[]string{"swamp", "volcano"}},

	// Accessories
	{entity.Item{Name: "Venom Ring", Type: entity.ItemAccessory, StatBoost: 0, Price: 200, LevelReq: 12, ClassRestrict: -1, BonusStat: 5, BonusType: entity.BonusATK},
		[]string{"swamp"}},
	{entity.Item{Name: "Flame Heart", Type: entity.ItemAccessory, StatBoost: 0, Price: 300, LevelReq: 14, ClassRestrict: -1, BonusStat: 15, BonusType: entity.BonusHP},
		[]string{"volcano"}},

	// ═══════════════════════════════════════════════
	// WEST CHAIN — Desert / Sand Ruins / Buried Temple (Lv 10-18)
	// ═══════════════════════════════════════════════

	// Helmets
	{entity.Item{Name: "Desert Turban", Type: entity.ItemHelmet, StatBoost: 6, Price: 120, LevelReq: 10, ClassRestrict: -1, BonusStat: 1, BonusType: entity.BonusSPD},
		[]string{"desert"}},
	{entity.Item{Name: "Sandstone Helm", Type: entity.ItemHelmet, StatBoost: 9, Price: 250, LevelReq: 14, ClassRestrict: 0},
		[]string{"sand_ruins"}},
	{entity.Item{Name: "Pharaoh Mask", Type: entity.ItemHelmet, StatBoost: 12, Price: 450, LevelReq: 17, ClassRestrict: -1, BonusStat: 5, BonusType: entity.BonusATK},
		[]string{"buried_temple"}},

	// Boots
	{entity.Item{Name: "Sand Striders", Type: entity.ItemBoots, StatBoost: 5, Price: 110, LevelReq: 10, ClassRestrict: -1, BonusStat: 3, BonusType: entity.BonusSPD},
		[]string{"desert"}},
	{entity.Item{Name: "Sphinx Boots", Type: entity.ItemBoots, StatBoost: 9, Price: 350, LevelReq: 16, ClassRestrict: -1, BonusStat: 5, BonusType: entity.BonusSPD},
		[]string{"buried_temple"}},

	// Shields
	{entity.Item{Name: "Scarab Shield", Type: entity.ItemShield, StatBoost: 10, Price: 250, LevelReq: 12, ClassRestrict: 0},
		[]string{"desert", "sand_ruins"}},
	{entity.Item{Name: "Ankh Ward", Type: entity.ItemShield, StatBoost: 14, Price: 500, LevelReq: 16, ClassRestrict: -1},
		[]string{"buried_temple"}},

	// Accessories
	{entity.Item{Name: "Scarab Charm", Type: entity.ItemAccessory, StatBoost: 0, Price: 200, LevelReq: 10, ClassRestrict: -1, BonusStat: 4, BonusType: entity.BonusDEF},
		[]string{"desert", "sand_ruins"}},
	{entity.Item{Name: "Eye of Ra", Type: entity.ItemAccessory, StatBoost: 0, Price: 500, LevelReq: 17, ClassRestrict: -1, BonusStat: 7, BonusType: entity.BonusATK},
		[]string{"buried_temple"}},
	{entity.Item{Name: "Ankh Pendant", Type: entity.ItemAccessory, StatBoost: 0, Price: 450, LevelReq: 16, ClassRestrict: -1, BonusStat: 20, BonusType: entity.BonusHP},
		[]string{"buried_temple"}},
}

// LootForArea returns the loot-only items that can drop in a specific area.
// Used by the chest loot system to pick area-appropriate gear.
func LootForArea(areaKey string) []entity.Item {
	var items []entity.Item
	for _, entry := range AreaLootTable {
		for _, a := range entry.Areas {
			if a == areaKey {
				items = append(items, entry.Item)
				break
			}
		}
	}
	return items
}

// ---------- Rarity roll system ----------
//
// THEORY — Hybrid rarity model (area floor + random roll):
// The rarity of a dropped item is determined by two factors:
//   1. Area minimum level → sets the rarity FLOOR (no Common drops in endgame)
//   2. Random roll → can push rarity above the floor
//
// This prevents frustration (hard content never gives garbage) while preserving
// excitement (lucky rolls can happen anywhere). The area-level breakpoints:
//
//   MinLevel 0-5:   floor = Common     (early game: mostly white, some green)
//   MinLevel 6-9:   floor = Uncommon   (mid game: green minimum, blue possible)
//   MinLevel 10-13: floor = Rare       (late game: blue minimum, purple possible)
//   MinLevel 14-17: floor = Rare       (endgame: blue/purple, legendary chance)
//   MinLevel 18+:   floor = Epic       (post-game: purple minimum, gold possible)
//
// Within each floor, the roll percentages shift upward. The result is that
// early areas feel exciting when you roll a Rare, while endgame areas feel
// rewarding because even the floor drop is good.

// RollRarity returns a random rarity appropriate for the given area minimum level.
func RollRarity(areaMinLevel int) entity.Rarity {
	roll := rand.Intn(100)

	switch {
	case areaMinLevel >= 18:
		// Post-game: Epic floor
		// 40% Epic, 35% Rare, 25% Legendary
		if roll < 25 {
			return entity.RarityLegendary
		} else if roll < 65 {
			return entity.RarityEpic
		}
		return entity.RarityRare

	case areaMinLevel >= 14:
		// Endgame: Rare floor
		// 35% Rare, 30% Epic, 20% Uncommon, 15% Legendary
		if roll < 15 {
			return entity.RarityLegendary
		} else if roll < 45 {
			return entity.RarityEpic
		} else if roll < 80 {
			return entity.RarityRare
		}
		return entity.RarityUncommon

	case areaMinLevel >= 10:
		// Late game: Uncommon floor
		// 30% Uncommon, 35% Rare, 20% Epic, 10% Common, 5% Legendary
		if roll < 5 {
			return entity.RarityLegendary
		} else if roll < 25 {
			return entity.RarityEpic
		} else if roll < 60 {
			return entity.RarityRare
		} else if roll < 90 {
			return entity.RarityUncommon
		}
		return entity.RarityCommon

	case areaMinLevel >= 6:
		// Mid game: Common floor but better odds
		// 30% Common, 35% Uncommon, 20% Rare, 10% Epic, 5% Legendary
		if roll < 5 {
			return entity.RarityLegendary
		} else if roll < 15 {
			return entity.RarityEpic
		} else if roll < 35 {
			return entity.RarityRare
		} else if roll < 70 {
			return entity.RarityUncommon
		}
		return entity.RarityCommon

	default:
		// Early game: mostly Common
		// 55% Common, 25% Uncommon, 13% Rare, 5% Epic, 2% Legendary
		if roll < 2 {
			return entity.RarityLegendary
		} else if roll < 7 {
			return entity.RarityEpic
		} else if roll < 20 {
			return entity.RarityRare
		} else if roll < 45 {
			return entity.RarityUncommon
		}
		return entity.RarityCommon
	}
}

// RollLootWithRarity picks a random item from the area's loot table and
// applies a rarity roll. Returns the item with Rarity set.
// If the rarity is Uncommon+ and the item has no BonusStat, a random
// bonus is injected to make higher rarities feel more special.
func RollLootWithRarity(areaKey string, areaMinLevel int) (entity.Item, bool) {
	loot := LootForArea(areaKey)
	if len(loot) == 0 {
		return entity.Item{}, false
	}

	item := loot[rand.Intn(len(loot))]
	item.Rarity = RollRarity(areaMinLevel)

	// THEORY — Rarity bonus injection:
	// If an item has no secondary stat (BonusStat == 0) and it rolls
	// Uncommon or better, we inject a small random bonus. This ensures
	// higher-rarity items always feel meaningfully different from Common.
	// The bonus type is chosen to match the item's role:
	//   Weapon → ATK bonus, Armor/Helm/Shield → DEF bonus,
	//   Boots → SPD bonus, Accessory → random
	if item.Rarity > entity.RarityCommon && item.BonusStat == 0 {
		bonusBase := 1 + int(item.Rarity) // 2 for Uncommon, 5 for Legendary
		switch item.Type {
		case entity.ItemWeapon:
			item.BonusType = entity.BonusATK
			item.BonusStat = bonusBase
		case entity.ItemArmor, entity.ItemHelmet, entity.ItemShield:
			item.BonusType = entity.BonusDEF
			item.BonusStat = bonusBase
		case entity.ItemBoots:
			item.BonusType = entity.BonusSPD
			item.BonusStat = bonusBase
		case entity.ItemAccessory:
			// Random bonus type for accessories
			types := []entity.BonusStatType{entity.BonusHP, entity.BonusATK, entity.BonusDEF, entity.BonusSPD}
			item.BonusType = types[rand.Intn(len(types))]
			item.BonusStat = bonusBase * 2 // accessories get bigger bonuses
		}
	}

	return item, true
}

// RollShopRarity returns a rarity for shop-purchased items.
// Shop items are always Common — you buy reliability, not excitement.
// The excitement of rare drops is reserved for exploration.
func RollShopRarity() entity.Rarity {
	return entity.RarityCommon
}
