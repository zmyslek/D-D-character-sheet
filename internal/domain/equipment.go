package domain

import "strings"

// Equipment is an item from the SRD equipment list.
type Equipment struct {
	name          string
	equipmentType string
	weight        float64
}

// NewEquipment creates an item with its CSV type ("Weapon", "Armor", ...).
// It weighs nothing until WithWeight gives it a weight.
func NewEquipment(name, equipmentType string) *Equipment {
	return &Equipment{name: name, equipmentType: equipmentType}
}

// WithWeight returns a copy of the item that weighs pounds. The item itself
// is not changed.
func (equipment *Equipment) WithWeight(pounds float64) *Equipment {
	weighed := *equipment
	weighed.weight = pounds
	return &weighed
}

// Name returns the item name.
func (equipment *Equipment) Name() string { return equipment.name }

// Type returns the item type from the data file.
func (equipment *Equipment) Type() string { return equipment.equipmentType }

// Weight returns the item's weight in pounds; 0 when the SRD lists none.
func (equipment *Equipment) Weight() float64 { return equipment.weight }

// IsWeapon reports whether the item can be held as a weapon.
func (equipment *Equipment) IsWeapon() bool {
	return strings.EqualFold(equipment.equipmentType, "Weapon")
}

// IsShield reports whether the item is a shield.
func (equipment *Equipment) IsShield() bool {
	return strings.EqualFold(equipment.name, "Shield")
}

// IsArmor reports whether the item is body armor (shields are listed as armor
// in the data file but go in their own slot).
func (equipment *Equipment) IsArmor() bool {
	return strings.EqualFold(equipment.equipmentType, "Armor") && !equipment.IsShield()
}
