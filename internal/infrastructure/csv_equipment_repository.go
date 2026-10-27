package infrastructure

import (
	"encoding/csv"
	"fmt"
	"math"
	domain "mpp-zmyslek/internal/domain"
	"os"
	"strconv"
	"strings"
)

// halfPlate is the name the armor rules use for "Half Plate Armor".
const halfPlate = "half plate"

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
			if normalizeEquipmentName(name) == halfPlate {
				equipmentName = halfPlate
			}
			weight, err := parseWeight(row)
			if err != nil {
				return nil, fmt.Errorf("invalid weight on equipment row %d: %w", index+1, err)
			}
			return domain.NewEquipment(equipmentName, row[1]).WithWeight(weight), nil
		}
	}

	return nil, fmt.Errorf("equipment %q not found", name)
}

// parseWeight reads the optional third column, the weight in pounds. A
// missing or empty value means the SRD lists no weight (packs, mounts,
// vehicles), which counts as 0.
func parseWeight(row []string) (float64, error) {
	if len(row) < 3 || strings.TrimSpace(row[2]) == "" {
		return 0, nil
	}
	weight, err := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
	if err != nil || math.IsNaN(weight) || math.IsInf(weight, 0) {
		return 0, fmt.Errorf("%q is not a number", row[2])
	}
	if weight < 0 {
		return 0, fmt.Errorf("%q is negative", row[2])
	}
	return weight, nil
}

// normalizeEquipmentName handles aliases used by the equipment CSV and rules.
func normalizeEquipmentName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "half plate armor" {
		return halfPlate
	}
	return normalized
}
