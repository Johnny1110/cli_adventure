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

import "cli_adventure/entity"

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

	// === Consumables (any class) ===
	{Name: "Potion", Type: entity.ItemConsumable, StatBoost: 15, Price: 15, ClassRestrict: -1},
	{Name: "Hi Potion", Type: entity.ItemConsumable, StatBoost: 40, Price: 40, ClassRestrict: -1},
	{Name: "Elixir", Type: entity.ItemConsumable, StatBoost: 99, Price: 100, ClassRestrict: -1},
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
