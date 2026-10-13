package domain

// Spell contains the spell data required for class and slot validation.
type Spell struct {
	// Name is the spell's display and lookup name.
	Name string
	// Level is zero for a cantrip and one through nine for leveled spells.
	Level int
	// Classes lists the classes allowed to use the spell.
	Classes []string
}
