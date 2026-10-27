package infrastructure

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"mpp-zmyslek/internal/domain"
)

// CSVCharacterRepository stores characters in a CSV file, one character per
// row, so the file can be opened, read and edited in a spreadsheet program:
//
//	Name,Race,Class,Level,Background,STR,DEX,CON,INT,WIS,CHA,Skill Proficiencies,Main Hand,...
//	Mira,elf,wizard,1,acolyte,8,16,13,15,12,10,arcana; history; insight; religion,Dagger,...
//
// Lists (skills and spells) are written in one cell, separated by "; ", like
// the skill lists in Classes.csv. Columns are found by their header, so a
// spreadsheet may reorder them; unknown columns are ignored.
//
// Like JSONCharacterRepository, it is safe for concurrent use within one
// process (a mutex around every read-modify-write) and replaces the file
// atomically (write to a temporary file, then rename).
type CSVCharacterRepository struct {
	filePath string
	mu       sync.Mutex
}

// NewCSVCharacterRepository creates a repository that uses filePath for storage.
func NewCSVCharacterRepository(filePath string) *CSVCharacterRepository {
	return &CSVCharacterRepository{filePath: filePath}
}

// listSeparator separates the entries of a list cell.
const listSeparator = "; "

// characterColumn is one column of the file: its header, and how a cell is
// written from and read into the character data.
type characterColumn struct {
	header string
	format func(domain.CharacterData) string
	parse  func(data *domain.CharacterData, cell string) error
}

// textColumn, numberColumn and listColumn build a column from a pointer to
// the field it stores, so each column is declared in one line below.
func textColumn(header string, field func(*domain.CharacterData) *string) characterColumn {
	return characterColumn{
		header: header,
		format: func(data domain.CharacterData) string { return escapeFormula(*field(&data)) },
		parse: func(data *domain.CharacterData, cell string) error {
			*field(data) = unescapeFormula(cell)
			return nil
		},
	}
}

func numberColumn(header string, field func(*domain.CharacterData) *int) characterColumn {
	return characterColumn{
		header: header,
		format: func(data domain.CharacterData) string { return strconv.Itoa(*field(&data)) },
		parse: func(data *domain.CharacterData, cell string) error {
			value, err := strconv.Atoi(strings.TrimSpace(cell))
			if err != nil {
				return fmt.Errorf("%q is not a whole number", cell)
			}
			*field(data) = value
			return nil
		},
	}
}

func listColumn(header string, field func(*domain.CharacterData) *[]string) characterColumn {
	return characterColumn{
		header: header,
		format: func(data domain.CharacterData) string {
			return escapeFormula(strings.Join(*field(&data), listSeparator))
		},
		parse: func(data *domain.CharacterData, cell string) error {
			var values []string
			for _, value := range strings.Split(unescapeFormula(cell), ";") {
				if value = strings.TrimSpace(value); value != "" {
					values = append(values, value)
				}
			}
			*field(data) = values
			return nil
		},
	}
}

// characterColumns is the layout of the file, in the order it is written.
var characterColumns = []characterColumn{
	textColumn("Name", func(data *domain.CharacterData) *string { return &data.Name }),
	textColumn("Race", func(data *domain.CharacterData) *string { return &data.Race }),
	textColumn("Class", func(data *domain.CharacterData) *string { return &data.Class }),
	numberColumn("Level", func(data *domain.CharacterData) *int { return &data.Level }),
	textColumn("Background", func(data *domain.CharacterData) *string { return &data.Background }),
	numberColumn("STR", func(data *domain.CharacterData) *int { return &data.Str }),
	numberColumn("DEX", func(data *domain.CharacterData) *int { return &data.Dex }),
	numberColumn("CON", func(data *domain.CharacterData) *int { return &data.Con }),
	numberColumn("INT", func(data *domain.CharacterData) *int { return &data.Int }),
	numberColumn("WIS", func(data *domain.CharacterData) *int { return &data.Wis }),
	numberColumn("CHA", func(data *domain.CharacterData) *int { return &data.Cha }),
	listColumn("Skill Proficiencies", func(data *domain.CharacterData) *[]string { return &data.SkillProficiencies }),
	textColumn("Main Hand", func(data *domain.CharacterData) *string { return &data.MainHand }),
	textColumn("Off Hand", func(data *domain.CharacterData) *string { return &data.OffHand }),
	textColumn("Armor", func(data *domain.CharacterData) *string { return &data.Armor }),
	textColumn("Shield", func(data *domain.CharacterData) *string { return &data.Shield }),
	listColumn("Known Spells", func(data *domain.CharacterData) *[]string { return &data.KnownSpells }),
	listColumn("Prepared Spells", func(data *domain.CharacterData) *[]string { return &data.PreparedSpells }),
}

// escapeFormula protects a text cell from formula injection: a spreadsheet
// runs a cell that starts with =, +, - or @ as a formula, so such a cell is
// written with a leading apostrophe, which the spreadsheet shows as text
// (OWASP "CSV Injection"). unescapeFormula removes it again when reading.
func escapeFormula(cell string) string {
	if cell != "" && strings.ContainsRune("=+-@\t\r", rune(cell[0])) {
		return "'" + cell
	}
	return cell
}

func unescapeFormula(cell string) string {
	if len(cell) > 1 && cell[0] == '\'' && strings.ContainsRune("=+-@\t\r", rune(cell[1])) {
		return cell[1:]
	}
	return cell
}

// GetAll reads every character from the CSV file.
func (r *CSVCharacterRepository) GetAll() ([]*domain.Character, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readAll()
}

// GetByName returns the character with exactly this name.
func (r *CSVCharacterRepository) GetByName(name string) (*domain.Character, error) {
	characters, err := r.GetAll()
	if err != nil {
		return nil, err
	}
	if index := indexOfCharacter(characters, name); index >= 0 {
		return characters[index], nil
	}
	return nil, fmt.Errorf("character %q %w", name, domain.ErrNotFound)
}

// Create adds a new row, or replaces the row with the same name.
func (r *CSVCharacterRepository) Create(character *domain.Character) error {
	return r.change(func(characters []*domain.Character) ([]*domain.Character, error) {
		if index := indexOfCharacter(characters, character.Name()); index >= 0 {
			characters[index] = character
			return characters, nil
		}
		return append(characters, character), nil
	})
}

// Update replaces an existing row and returns an error if its name is unknown.
func (r *CSVCharacterRepository) Update(character *domain.Character) error {
	return r.change(func(characters []*domain.Character) ([]*domain.Character, error) {
		index := indexOfCharacter(characters, character.Name())
		if index < 0 {
			return nil, fmt.Errorf("character %w", domain.ErrNotFound)
		}
		characters[index] = character
		return characters, nil
	})
}

// Delete removes the named row.
func (r *CSVCharacterRepository) Delete(name string) error {
	return r.change(func(characters []*domain.Character) ([]*domain.Character, error) {
		index := indexOfCharacter(characters, name)
		if index < 0 {
			return nil, fmt.Errorf("character %w", domain.ErrNotFound)
		}
		return slices.Delete(characters, index, index+1), nil
	})
}

// indexOfCharacter returns the position of the named character, or -1.
func indexOfCharacter(characters []*domain.Character, name string) int {
	return slices.IndexFunc(characters, func(character *domain.Character) bool { return character.Name() == name })
}

// change runs one read-modify-write cycle while holding the mutex.
func (r *CSVCharacterRepository) change(modify func([]*domain.Character) ([]*domain.Character, error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	characters, err := r.readAll()
	if err != nil {
		return err
	}
	if characters, err = modify(characters); err != nil {
		return err
	}
	return r.writeAll(characters)
}

// readAll parses the file. A missing or empty file holds no characters.
func (r *CSVCharacterRepository) readAll() ([]*domain.Character, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Character{}, nil
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // a spreadsheet may leave out empty cells at the end of a row
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}
	characters := []*domain.Character{}
	if len(rows) == 0 {
		return characters, nil
	}
	positions, err := columnPositions(rows[0])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(r.filePath), err)
	}
	for index, row := range rows[1:] {
		data, err := parseCharacterRow(row, positions)
		if err != nil {
			// index 0 is the second line of the file, after the header.
			return nil, fmt.Errorf("%s row %d: %w", filepath.Base(r.filePath), index+2, err)
		}
		characters = append(characters, domain.RestoreCharacter(data))
	}
	return characters, nil
}

// columnPositions finds every column in the header row by name, ignoring
// case, surrounding spaces and the byte order mark spreadsheet programs put
// at the start of a UTF-8 file. positions[i] is the cell of characterColumns[i].
func columnPositions(header []string) ([]int, error) {
	found := make(map[string]int, len(header))
	for position, name := range header {
		found[normalizeHeader(name)] = position
	}
	positions := make([]int, len(characterColumns))
	var missing []string
	for index, column := range characterColumns {
		position, ok := found[normalizeHeader(column.header)]
		if !ok {
			missing = append(missing, column.header)
		}
		positions[index] = position
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing columns: %s", strings.Join(missing, ", "))
	}
	return positions, nil
}

// byteOrderMark is U+FEFF in UTF-8, which Excel writes at the start of a
// "CSV UTF-8" file.
const byteOrderMark = "\xef\xbb\xbf"

func normalizeHeader(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, byteOrderMark)))
}

// parseCharacterRow reads one row. A cell missing at the end of a short row
// is empty. Text is kept exactly as written (the name is the identifier);
// numbers and list entries are trimmed.
func parseCharacterRow(row []string, positions []int) (domain.CharacterData, error) {
	var data domain.CharacterData
	for index, column := range characterColumns {
		cell := ""
		if positions[index] < len(row) {
			cell = row[positions[index]]
		}
		if err := column.parse(&data, cell); err != nil {
			return data, fmt.Errorf("column %q: %w", column.header, err)
		}
	}
	if strings.TrimSpace(data.Name) == "" {
		return data, errors.New(`column "Name" is empty`)
	}
	return data, nil
}

// writeAll writes the header and one row per character, then replaces the
// file atomically, so it is always either the old or the new version.
func (r *CSVCharacterRepository) writeAll(characters []*domain.Character) error {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	row := make([]string, len(characterColumns))
	for index, column := range characterColumns {
		row[index] = column.header
	}
	_ = writer.Write(row) // a bytes.Buffer never fails; writer.Error reports any problem
	for _, character := range characters {
		data := character.Data()
		for index, column := range characterColumns {
			row[index] = column.format(data)
		}
		_ = writer.Write(row)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("failed to encode CSV: %w", err)
	}
	return replaceFile(r.filePath, buffer.Bytes())
}

// replaceFile writes data to a temporary file in the same folder and renames
// it over path. A rename within one folder is atomic.
func replaceFile(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer os.Remove(temp.Name()) // no-op after a successful rename
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("failed to write file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	// CreateTemp makes owner-only files; keep the file readable like the JSON file.
	if err := os.Chmod(temp.Name(), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return os.Rename(temp.Name(), path)
}
