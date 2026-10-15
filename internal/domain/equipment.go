package domain

import "strings"

// Equipment is an item from the SRD equipment list.
type Equipment struct {
	name          string
	equipmentType string
}

// NewEquipment creates an item with its CSV type ("Weapon", "Armor", ...).
func NewEquipment(name, equipmentType string) *Equipment {
	return &Equipment{name: name, equipmentType: equipmentType}
}

// Name returns the item name.
func (equipment *Equipment) Name() string { return equipment.name }

// Type returns the item type from the data file.
func (equipment *Equipment) Type() string { return equipment.equipmentType }

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
