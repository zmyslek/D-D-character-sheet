package equipmentdomain

type EquipmentRepository interface {
	GetByName(name string) (*Equipment, error)
}
