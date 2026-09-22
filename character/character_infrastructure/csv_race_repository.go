package characterinfrastructure

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Race struct {
	Name             string
	SkillOptions     []string
	SkillChoiceCount int
}

type CSVRaceRepository struct {
	filePath string
}

func NewCSVRaceRepository(filePath string) *CSVRaceRepository {
	return &CSVRaceRepository{filePath: filePath}
}

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

	normalizedName := strings.ToLower(strings.TrimSpace(name))
	for index, row := range rows {
		if index == 0 {
			continue
		}
		if len(row) < 10 {
			return nil, fmt.Errorf("invalid race row %d", index+1)
		}
		if strings.ToLower(strings.TrimSpace(row[0])) != normalizedName {
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
