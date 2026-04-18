package entity

// ItemType categorizes items for equip logic and UI display.
type ItemType int

const (
	ItemWeapon ItemType = iota
	ItemArmor
	ItemConsumable
)

// Item represents an equippable or consumable game item.
//
// THEORY — Class restrictions:
// In many RPGs (Final Fantasy, Dragon Quest), equipment is class-locked: a Mage
// can't equip a greatsword, and a Knight can't wear robes. This creates meaningful
// loot differentiation — finding a rare staff is exciting for a Mage but vendor
// trash for a Knight. We use ClassRestrict: -1 means "any class can equip",
// 0/1/2 means "only that class". This encourages the player to replay with
// different classes to experience different gear sets.
type Item struct {
	Name          string
	Type          ItemType
	StatBoost     int // ATK for weapons, DEF for armor, HP restore for consumables
	Price         int
	ClassRestrict int // -1 = any class, 0 = Knight, 1 = Mage, 2 = Archer
}

// CanEquip checks if the given class can equip this item.
func (it *Item) CanEquip(class Class) bool {
	if it.ClassRestrict < 0 {
		return true // universal item
	}
	return it.ClassRestrict == int(class)
}
