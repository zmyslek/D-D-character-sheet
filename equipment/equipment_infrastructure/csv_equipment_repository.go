package equipmentinfrastructure

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	equipmentdomain "mpp-zmyslek/equipment/equipment_domain"
)

type CSVEquipmentRepository struct {
	filePath string
}

func NewCSVEquipmentRepository(filePath string) *CSVEquipmentRepository {
	return &CSVEquipmentRepository{filePath: filePath}
}

func (r *CSVEquipmentRepository) GetByName(name string) (*equipmentdomain.Equipment, error) {
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
		if strings.EqualFold(strings.TrimSpace(row[0]), strings.TrimSpace(name)) {
			return &equipmentdomain.Equipment{
				Name: row[0],
				Type: row[1],
			}, nil
		}
	}

	return nil, fmt.Errorf("equipment %q not found", name)
}
