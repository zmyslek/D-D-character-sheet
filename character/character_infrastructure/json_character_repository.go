package characterinfrastructure

import (
	"encoding/json"
	"errors"
	"fmt"

	// JSONCharacterRepository stores characters in a JSON file on disk.
	"os"

	characterdomain "mpp-zmyslek/character/character_domain"
	// NewJSONCharacterRepository creates a repository that uses filePath for storage.
)

type JSONCharacterRepository struct {
	filePath string
	// GetAll loads every character from the JSON file.
}

func NewJSONCharacterRepository(filePath string) *JSONCharacterRepository {
	return &JSONCharacterRepository{filePath: filePath}
}

func (r *JSONCharacterRepository) GetAll() ([]*characterdomain.Character, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*characterdomain.Character{}, nil
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var characters []*characterdomain.Character
	if err := json.NewDecoder(file).Decode(&characters); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
		// GetByName searches the stored characters and returns the matching character.
	}

	return characters, nil
}

func (r *JSONCharacterRepository) GetByName(name string) (*characterdomain.Character, error) {
	characters, err := r.GetAll()
	if err != nil {
		return nil, err
	}

	for _, character := range characters {
		if character.Name == name {
			return character, nil
		}
		// Create appends a character and writes the complete collection back to disk.
	}

	return nil, fmt.Errorf("character %q not found", name)
}

func (r *JSONCharacterRepository) Create(character *characterdomain.Character) error {
	characters, err := r.GetAll()
	if err != nil {
		return err
	}

	// Names identify characters, so creating an existing name updates that record
	// instead of leaving duplicate records that could make lookup ambiguous.
	for index, existingCharacter := range characters {
		if existingCharacter.Name == character.Name {
			characters[index] = character
			return r.writeAll(characters)
		}
	}

	return r.writeAll(append(characters, character))
}

func (r *JSONCharacterRepository) Update(character *characterdomain.Character) error {
	characters, err := r.GetAll()
	if err != nil {
		return err
	}

	for index, existingCharacter := range characters {
		if existingCharacter.Name == character.Name {
			characters[index] = character
			return r.writeAll(characters)
		}
		// Delete removes the character with the requested name from the collection.
	}

	return errors.New("character not found")
}

func (r *JSONCharacterRepository) Delete(name string) error {
	characters, err := r.GetAll()
	if err != nil {
		return err
	}

	for index, character := range characters {
		if character.Name == name {
			characters = append(characters[:index], characters[index+1:]...)
			return r.writeAll(characters)
		}
		// writeAll centralizes JSON serialization for update and delete operations.
	}

	return errors.New("character not found")
}

func (r *JSONCharacterRepository) writeAll(characters []*characterdomain.Character) error {
	file, err := os.Create(r.filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(characters); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
