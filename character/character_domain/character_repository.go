package characterdomain

type CharacterRepository interface {
	GetAll() ([]*Character, error)
	GetByName(name string) (*Character, error)
	Create(character *Character) error
	Update(character *Character) error
	Delete(name string) error
}
