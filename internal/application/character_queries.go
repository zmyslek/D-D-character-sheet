package application

import (
	"context"
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
	Data     map[string]any `json:"data,omitempty"`
	Error    string         `json:"error,omitempty"`
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
			entry.Data = data
		}(&results[index])
	}
	wg.Wait()
	return results, nil
}
