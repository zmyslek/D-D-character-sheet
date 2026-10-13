package infrastructure

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// RaceAbilityBonus represents one ability-score increase granted by a race.
type RaceAbilityBonus struct {
	Ability string
	Points  int
}

// CSVRaceAbilityRepository reads racial ability bonuses from a CSV file.
type CSVRaceAbilityRepository struct {
	filePath string
}

// NewCSVRaceAbilityRepository configures racial bonus lookups for filePath.
func NewCSVRaceAbilityRepository(filePath string) *CSVRaceAbilityRepository {
	return &CSVRaceAbilityRepository{filePath: filePath}
}

// GetByRace returns every ability bonus configured for the requested race.
func (repository *CSVRaceAbilityRepository) GetByRace(race string) ([]RaceAbilityBonus, error) {
	file, err := os.Open(repository.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open race ability file: %w", err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read race ability file: %w", err)
	}

	normalizedRace := normalizeRaceName(race)
	bonuses := make([]RaceAbilityBonus, 0)
	for index, row := range rows {
		if index == 0 {
			continue
		}
		if len(row) < 3 {
			return nil, fmt.Errorf("invalid race ability row %d", index+1)
		}
		if normalizeRaceName(row[0]) != normalizedRace {
			continue
		}

		points, err := strconv.Atoi(strings.TrimSpace(row[2]))
		if err != nil {
			return nil, fmt.Errorf("invalid bonus on race ability row %d: %w", index+1, err)
		}
		bonuses = append(bonuses, RaceAbilityBonus{
			Ability: strings.ToLower(strings.TrimSpace(row[1])),
			Points:  points,
		})
	}

	if len(bonuses) == 0 {
		return nil, fmt.Errorf("race %q not found", race)
	}
	return bonuses, nil
}
