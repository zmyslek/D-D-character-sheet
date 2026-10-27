package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"mpp-zmyslek/internal/domain"
)

// JSONCharacterRepository stores characters in a JSON file on disk.
//
// It is safe for concurrent use within one process: a mutex lets only one
// goroutine read-modify-write the file at a time, so two simultaneous saves
// cannot overwrite each other's changes. Writes go to a temporary file that is
// then renamed over the real one, so a reader or a crash never sees a
// half-written file.
type JSONCharacterRepository struct {
	filePath string
	mu       sync.Mutex
}

// NewJSONCharacterRepository creates a repository that uses filePath for storage.
func NewJSONCharacterRepository(filePath string) *JSONCharacterRepository {
	return &JSONCharacterRepository{filePath: filePath}
}

// characterRecord is the JSON shape of one character in the file. It is kept
// separate from the domain type so file details stay in this package.
type characterRecord struct {
	Name               string
	Race               legacyName
	Class              legacyName
	Level              int
	Background         string   `json:",omitempty"`
	SkillProficiencies []string `json:",omitempty"`
	MainHand           string   `json:",omitempty"`
	OffHand            string   `json:",omitempty"`
	Armor              string   `json:",omitempty"`
	Shield             string   `json:",omitempty"`
	KnownSpells        []string `json:",omitempty"`
	PreparedSpells     []string `json:",omitempty"`
	Str                int
	Dex                int
	Con                int
	Int                int
	Wis                int
	Cha                int
}

// legacyName also accepts the object-shaped Race and Class fields written by
// the previous version of the program, so old files remain readable.
type legacyName string

// UnmarshalJSON implements json.Unmarshaler.
func (name *legacyName) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*name = legacyName(text)
		return nil
	}
	var legacy struct{ Name, RaceName, ClassName string }
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	for _, value := range []string{legacy.RaceName, legacy.ClassName, legacy.Name} {
		if value != "" {
			*name = legacyName(value)
			return nil
		}
	}
	*name = ""
	return nil
}

func toRecord(character *domain.Character) characterRecord {
	data := character.Data()
	return characterRecord{
		Name: data.Name, Race: legacyName(data.Race), Class: legacyName(data.Class), Level: data.Level,
		Background: data.Background, SkillProficiencies: data.SkillProficiencies,
		MainHand: data.MainHand, OffHand: data.OffHand, Armor: data.Armor, Shield: data.Shield,
		KnownSpells: data.KnownSpells, PreparedSpells: data.PreparedSpells,
		Str: data.Str, Dex: data.Dex, Con: data.Con, Int: data.Int, Wis: data.Wis, Cha: data.Cha,
	}
}

func (record characterRecord) toCharacter() *domain.Character {
	return domain.RestoreCharacter(domain.CharacterData{
		Name: record.Name, Race: string(record.Race), Class: string(record.Class), Level: record.Level,
		Background: record.Background, SkillProficiencies: record.SkillProficiencies,
		MainHand: record.MainHand, OffHand: record.OffHand, Armor: record.Armor, Shield: record.Shield,
		KnownSpells: record.KnownSpells, PreparedSpells: record.PreparedSpells,
		Str: record.Str, Dex: record.Dex, Con: record.Con, Int: record.Int, Wis: record.Wis, Cha: record.Cha,
	})
}

// GetAll reads and decodes the complete character collection from the JSON file.
func (r *JSONCharacterRepository) GetAll() ([]*domain.Character, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readAll()
}

// GetByName returns the character with exactly this name.
func (r *JSONCharacterRepository) GetByName(name string) (*domain.Character, error) {
	characters, err := r.GetAll()
	if err != nil {
		return nil, err
	}
	for _, character := range characters {
		if character.Name() == name {
			return character, nil
		}
	}
	// Prints as: character "Name" not found
	return nil, fmt.Errorf("character %q %w", name, domain.ErrNotFound)
}

// Create inserts a new character or replaces the record with the same name.
func (r *JSONCharacterRepository) Create(character *domain.Character) error {
	return r.change(func(characters []*domain.Character) ([]*domain.Character, error) {
		// Names identify characters, so creating an existing name updates that record
		// instead of leaving duplicate records that could make lookup ambiguous.
		for index, existing := range characters {
			if existing.Name() == character.Name() {
				characters[index] = character
				return characters, nil
			}
		}
		return append(characters, character), nil
	})
}

// Update replaces an existing record and returns an error if its name is unknown.
func (r *JSONCharacterRepository) Update(character *domain.Character) error {
	return r.change(func(characters []*domain.Character) ([]*domain.Character, error) {
		for index, existing := range characters {
			if existing.Name() == character.Name() {
				characters[index] = character
				return characters, nil
			}
		}
		return nil, fmt.Errorf("character %w", domain.ErrNotFound)
	})
}

// Delete removes the named record.
func (r *JSONCharacterRepository) Delete(name string) error {
	return r.change(func(characters []*domain.Character) ([]*domain.Character, error) {
		for index, character := range characters {
			if character.Name() == name {
				return append(characters[:index], characters[index+1:]...), nil
			}
		}
		return nil, fmt.Errorf("character %w", domain.ErrNotFound)
	})
}

// change runs one read-modify-write cycle while holding the mutex.
func (r *JSONCharacterRepository) change(modify func([]*domain.Character) ([]*domain.Character, error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	characters, err := r.readAll()
	if err != nil {
		return err
	}
	characters, err = modify(characters)
	if err != nil {
		return err
	}
	return r.writeAll(characters)
}

func (r *JSONCharacterRepository) readAll() ([]*domain.Character, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Character{}, nil
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var records []characterRecord
	if err := json.NewDecoder(file).Decode(&records); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}
	characters := make([]*domain.Character, 0, len(records))
	for _, record := range records {
		characters = append(characters, record.toCharacter())
	}
	return characters, nil
}

// writeAll writes to a temporary file and renames it over the real file, so
// the file is always either the old or the new version, never half-written.
func (r *JSONCharacterRepository) writeAll(characters []*domain.Character) error {
	records := make([]characterRecord, 0, len(characters))
	for _, character := range characters {
		records = append(records, toRecord(character))
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return replaceFile(r.filePath, data)
}
