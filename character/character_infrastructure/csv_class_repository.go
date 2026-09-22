package characterinfrastructure

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Class struct {
	Name             string
	SkillOptions     []string
	SkillChoiceCount int
}

type CSVClassRepository struct {
	filePath string
}

func NewCSVClassRepository(filePath string) *CSVClassRepository {
	return &CSVClassRepository{filePath: filePath}
}

func (repository *CSVClassRepository) GetByName(name string) (*Class, error) {
	file, err := os.Open(repository.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open class file: %w", err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read class file: %w", err)
	}

	normalizedName := strings.ToLower(strings.TrimSpace(name))
	for index, row := range rows {
		if index == 0 {
			continue
		}
		if len(row) < 6 {
			return nil, fmt.Errorf("invalid class row %d", index+1)
		}
		if strings.ToLower(strings.TrimSpace(row[0])) != normalizedName {
			continue
		}

		choiceCount, err := strconv.Atoi(strings.TrimSpace(row[5]))
		if err != nil {
			return nil, fmt.Errorf("invalid skill choice count on class row %d: %w", index+1, err)
		}
		return &Class{
			Name:             strings.TrimSpace(row[0]),
			SkillOptions:     splitOptions(row[4]),
			SkillChoiceCount: choiceCount,
		}, nil
	}

	return nil, fmt.Errorf("class %q not found", name)
}

func splitOptions(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ";")
	options := make([]string, 0, len(parts))
	for _, part := range parts {
		if option := strings.ToLower(strings.TrimSpace(part)); option != "" {
			options = append(options, option)
		}
	}
	return options
}
