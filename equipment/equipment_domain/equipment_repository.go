package equipmentdomain

// EquipmentRepository provides lookup access to equipment data.
type EquipmentRepository interface {
	// GetByName returns the equipment whose name matches the requested value.
	GetByName(name string) (*Equipment, error)
}
