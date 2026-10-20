package application

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"mpp-zmyslek/internal/domain"
)

// CharacterReader is the read-only part of the character repository. The
// queries depend only on it, so they cannot save, change or delete anything.
type CharacterReader interface {
	GetAll() ([]*domain.Character, error)
	GetByName(name string) (*domain.Character, error)
}

// ResourceLookup fetches extra details about a spell or an item from an
// external source. It is declared here, where it is used, and implemented by
// infrastructure.SRDAPIClient.
type ResourceLookup interface {
	Lookup(ctx context.Context, resource, name string) (map[string]any, error)
}

// CharacterQueries contains the read-only use cases (the "Q" in CQRS). The
// commands in CharacterService change characters; these methods only read.
// Both sides share the same storage and model, so a query always sees the
// latest saved command (no eventual consistency).
type CharacterQueries struct {
	characters CharacterReader
	resources  ResourceLookup
}

// NewCharacterQueries creates the query service. resources may be nil when no
// external lookups are needed (the CLI).
func NewCharacterQueries(characters CharacterReader, resources ResourceLookup) *CharacterQueries {
	return &CharacterQueries{characters: characters, resources: resources}
}

// GetAll returns every character.
func (q *CharacterQueries) GetAll() ([]*domain.Character, error) {
	return q.characters.GetAll()
}

// Get returns one character by name.
func (q *CharacterQueries) Get(name string) (*domain.Character, error) {
	return q.characters.GetByName(name)
}

// Lookup returns the external details of one spell ("spells") or item ("equipment").
func (q *CharacterQueries) Lookup(ctx context.Context, resource, name string) (map[string]any, error) {
	return q.resources.Lookup(ctx, resource, name)
}

// Enrichment is the external information about one spell or item of a character.
type Enrichment struct {
	Resource string         `json:"resource"`
	Name     string         `json:"name"`
	Details  *Details       `json:"details,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// Details are the properties of a spell, weapon or armor that the sheet
// shows, taken from the external API document. Fields that do not apply to
// the resource are left empty.
type Details struct {
	// Spells
	School string `json:"school,omitempty"`
	Range  string `json:"range,omitempty"`
	// Weapons
	Category    string `json:"category,omitempty"`
	NormalRange int    `json:"normal_range,omitempty"`
	TwoHanded   bool   `json:"two_handed,omitempty"`
	// Armor (a shield has armor class 2)
	ArmorClass int  `json:"armor_class,omitempty"`
	DexBonus   bool `json:"dex_bonus,omitempty"`
	MaxBonus   int  `json:"max_dex_bonus,omitempty"`
}

// DetailsFrom selects the sheet's properties from an API document:
// spells give school and range; weapons give category, normal range and
// whether they are two-handed; armor gives armor class and Dexterity bonus.
func DetailsFrom(resource string, data map[string]any) Details {
	var details Details
	if resource == "spells" {
		details.School = text(object(data["school"])["name"])
		details.Range = text(data["range"])
		return details
	}
	if category := text(data["weapon_category"]); category != "" {
		details.Category = category
		details.NormalRange = number(object(data["range"])["normal"])
		properties, _ := data["properties"].([]any)
		for _, property := range properties {
			if text(object(property)["index"]) == "two-handed" {
				details.TwoHanded = true
			}
		}
	}
	if armorClass := object(data["armor_class"]); armorClass != nil {
		details.Category = text(data["armor_category"])
		details.ArmorClass = number(armorClass["base"])
		details.DexBonus, _ = armorClass["dex_bonus"].(bool)
		details.MaxBonus = number(armorClass["max_bonus"])
	}
	return details
}

// String formats the details for the CLI, for example
// "school Evocation, range 120 feet".
func (d Details) String() string {
	var parts []string
	add := func(format string, value any) { parts = append(parts, fmt.Sprintf(format, value)) }
	if d.School != "" {
		add("school %s", d.School)
	}
	if d.Range != "" {
		add("range %s", d.Range)
	}
	if d.Category != "" {
		add("category %s", d.Category)
	}
	if d.NormalRange > 0 {
		add("normal range %d ft.", d.NormalRange)
	}
	if d.TwoHanded {
		parts = append(parts, "two-handed")
	}
	if d.ArmorClass > 0 {
		add("armor class %d", d.ArmorClass)
		switch {
		case d.DexBonus && d.MaxBonus > 0:
			add("dex bonus (max %d)", d.MaxBonus)
		case d.DexBonus:
			parts = append(parts, "dex bonus")
		default:
			parts = append(parts, "no dex bonus")
		}
	}
	return strings.Join(parts, ", ")
}

// object, text and number read JSON values decoded into map[string]any;
// a missing or differently typed value gives the zero value.
func object(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func text(value any) string {
	result, _ := value.(string)
	return result
}

func number(value any) int {
	result, _ := value.(float64)
	return int(result)
}

// EnrichCharacter looks up every equipped item and spell of a character at the
// same time and waits for all of them.
//
// Each lookup runs in its own goroutine. Each goroutine writes only its own
// element of the results slice, so no lock is needed, and WaitGroup.Wait
// guarantees all writes are finished before the slice is returned. A failed
// lookup is reported in that entry's Error field instead of being dropped or
// failing the whole request. When ctx is cancelled (the client disconnects or
// the timeout passes) the waiting lookups return early with ctx's error.
func (q *CharacterQueries) EnrichCharacter(ctx context.Context, name string) ([]Enrichment, error) {
	character, err := q.characters.GetByName(name)
	if err != nil {
		return nil, err
	}
	var results []Enrichment
	for _, item := range []string{character.MainHand(), character.OffHand(), character.Armor(), character.Shield()} {
		if item != "" {
			results = append(results, Enrichment{Resource: "equipment", Name: item})
		}
	}
	for _, spell := range append(character.KnownSpells(), character.PreparedSpells()...) {
		results = append(results, Enrichment{Resource: "spells", Name: spell})
	}

	var wg sync.WaitGroup
	for index := range results {
		wg.Add(1)
		go func(entry *Enrichment) {
			defer wg.Done()
			data, err := q.resources.Lookup(ctx, entry.Resource, entry.Name)
			if err != nil {
				entry.Error = err.Error()
				return
			}
			details := DetailsFrom(entry.Resource, data)
			entry.Data, entry.Details = data, &details
		}(&results[index])
	}
	wg.Wait()
	return results, nil
}
