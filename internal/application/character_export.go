package application

import "mpp-zmyslek/internal/domain"

// CharacterSource lists every character. Both a CharacterRepository and
// CharacterQueries are one.
type CharacterSource interface {
	GetAll() ([]*domain.Character, error)
}

// ExportCharacters copies every character from one storage into another, for
// example from the JSON file into a CSV file for a spreadsheet. Characters
// are copied as stored (Create replaces a character with the same name), so
// no creation rule is applied twice. It returns how many were copied.
func ExportCharacters(from CharacterSource, to domain.CharacterRepository) (int, error) {
	characters, err := from.GetAll()
	if err != nil {
		return 0, err
	}
	for copied, character := range characters {
		if err := to.Create(character); err != nil {
			return copied, err
		}
	}
	return len(characters), nil
}
