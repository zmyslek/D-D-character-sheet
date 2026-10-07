package spellsinfrastructure

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	spellsdomain "mpp-zmyslek/spells/spells_domain"
)

// CSVSpellsRepository reads spell definitions from a CSV file.
type CSVSpellsRepository struct{ filePath string }

// NewCSVSpellsRepository configures spell lookups to read from filePath.
func NewCSVSpellsRepository(filePath string) *CSVSpellsRepository {
	return &CSVSpellsRepository{filePath: filePath}
}

// GetByName finds a spell and parses its level and allowed classes.
func (repository *CSVSpellsRepository) GetByName(name string) (*spellsdomain.Spell, error) {
	file, err := os.Open(repository.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open spell file: %w", err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read spell file: %w", err)
	}
	for index, row := range rows {
		if index == 0 {
			continue
		}
		if len(row) < 3 {
			return nil, fmt.Errorf("invalid spell row %d", index+1)
		}
		if !strings.EqualFold(strings.TrimSpace(row[0]), strings.TrimSpace(name)) {
			continue
		}
		level, err := strconv.Atoi(strings.TrimSpace(row[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid spell level on row %d: %w", index+1, err)
		}
		classes := make([]string, 0)
		for _, className := range strings.Split(row[2], ",") {
			classes = append(classes, strings.ToLower(strings.TrimSpace(className)))
		}
		return &spellsdomain.Spell{Name: strings.TrimSpace(row[0]), Level: level, Classes: classes}, nil
	}
	return nil, fmt.Errorf("spell %q not found", name)
}
