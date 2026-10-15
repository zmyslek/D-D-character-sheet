package domain

// SpellRepository provides lookup access to spell data.
type SpellRepository interface {
	// GetByName returns the spell whose name matches the requested value.
	GetByName(name string) (*Spell, error)
}
