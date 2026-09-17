package characterdomain

type CharacterRepository interface {
	// CharacterRepository defines the persistence operations used by the application.
	// GetAll returns every character stored by the repository.
	GetAll() ([]*Character, error)
	// GetByName finds one character using its unique name.
	GetByName(name string) (*Character, error)
	// Create adds a new character to storage.
	Create(character *Character) error
	// Update replaces the stored data for an existing character.
	Update(character *Character) error
	// Delete removes a character identified by name.
	Delete(name string) error
}
