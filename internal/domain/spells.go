package domain

import "slices"

// Spell is a spell from the SRD spell list.
type Spell struct {
	name    string
	level   int
	classes []string
}

// NewSpell creates a spell. Level 0 is a cantrip. Class names are stored in lowercase.
func NewSpell(name string, level int, classes []string) *Spell {
	normalized := make([]string, 0, len(classes))
	for _, class := range classes {
		normalized = append(normalized, normalize(class))
	}
	return &Spell{name: name, level: level, classes: normalized}
}

// Name returns the spell name.
func (spell *Spell) Name() string { return spell.name }

// Level returns 0 for a cantrip and 1-9 for other spells.
func (spell *Spell) Level() int { return spell.level }

// AvailableTo reports whether the spell is on the class's spell list.
func (spell *Spell) AvailableTo(class string) bool {
	return slices.Contains(spell.classes, normalize(class))
}
