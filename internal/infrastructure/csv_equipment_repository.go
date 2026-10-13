package infrastructure

import (
	"encoding/csv"
	"fmt"
	domain "mpp-zmyslek/internal/domain"
	"os"
	"strings"
)

// CSVEquipmentRepository reads equipment records from a CSV file.
type CSVEquipmentRepository struct {
	filePath string
}

// NewCSVEquipmentRepository configures equipment lookups to read from filePath.
func NewCSVEquipmentRepository(filePath string) *CSVEquipmentRepository {
	return &CSVEquipmentRepository{filePath: filePath}
}

// GetByName finds equipment using case-insensitive, normalized names.
func (r *CSVEquipmentRepository) GetByName(name string) (*domain.Equipment, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open equipment file: %w", err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read equipment file: %w", err)
	}

	for index, row := range rows {
		if index == 0 || len(row) < 2 {
			continue
		}
		if normalizeEquipmentName(row[0]) == normalizeEquipmentName(name) {
			equipmentName := strings.TrimSpace(row[0])
			if normalizeEquipmentName(name) == "half plate" {
				equipmentName = "half plate"
			}
			return &domain.Equipment{
				Name: equipmentName,
				Type: row[1],
			}, nil
		}
	}

	return nil, fmt.Errorf("equipment %q not found", name)
}

// normalizeEquipmentName handles aliases used by the equipment CSV and rules.
func normalizeEquipmentName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "half plate armor" {
		return "half plate"
	}
	return normalized
}
