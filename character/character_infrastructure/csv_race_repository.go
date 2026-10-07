package characterinfrastructure

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Race describes race skill choices loaded from the CSV data source.
type Race struct {
	Name             string
	SkillOptions     []string
	SkillChoiceCount int
}

// CSVRaceRepository reads race definitions from a CSV file.
type CSVRaceRepository struct {
	filePath string
}

// NewCSVRaceRepository configures race lookups to read from filePath.
func NewCSVRaceRepository(filePath string) *CSVRaceRepository {
	return &CSVRaceRepository{filePath: filePath}
}

// GetByName finds a race by name and parses its available skill choices.
func (repository *CSVRaceRepository) GetByName(name string) (*Race, error) {
	file, err := os.Open(repository.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open race file: %w", err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read race file: %w", err)
	}

	normalizedName := normalizeRaceName(name)
	for index, row := range rows {
		if index == 0 {
			continue
		}
		if len(row) < 10 {
			return nil, fmt.Errorf("invalid race row %d", index+1)
		}
		if normalizeRaceName(row[0]) != normalizedName {
			continue
		}

		choiceCount, err := strconv.Atoi(strings.TrimSpace(row[9]))
		if err != nil {
			return nil, fmt.Errorf("invalid skill choice count on race row %d: %w", index+1, err)
		}
		return &Race{
			Name:             strings.TrimSpace(row[0]),
			SkillOptions:     splitOptions(row[8]),
			SkillChoiceCount: choiceCount,
		}, nil
	}

	return nil, fmt.Errorf("race %q not found", name)
}

// normalizeRaceName makes race names comparable across spaces and hyphens.
func normalizeRaceName(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "-")
}
