package domain

// Class describes the skill choices a class offers.
type Class struct {
	name             string
	skillOptions     []string
	skillChoiceCount int
}

// NewClass creates a class with its skill options and how many may be chosen.
func NewClass(name string, skillOptions []string, skillChoiceCount int) *Class {
	return &Class{name: name, skillOptions: skillOptions, skillChoiceCount: skillChoiceCount}
}

// Name returns the class name.
func (class *Class) Name() string { return class.name }

// SkillOptions returns the skills the class may choose from.
func (class *Class) SkillOptions() []string { return append([]string(nil), class.skillOptions...) }

// SkillChoiceCount returns how many skills the class chooses.
func (class *Class) SkillChoiceCount() int { return class.skillChoiceCount }

// ClassRepository provides lookup access to class data.
type ClassRepository interface {
	GetByName(name string) (*Class, error)
}

// RaceAbilityBonus is one ability-score increase granted by a race.
type RaceAbilityBonus struct {
	Ability string
	Points  int
}

// RaceAbilityRepository provides the ability bonuses of each race.
type RaceAbilityRepository interface {
	GetByRace(race string) ([]RaceAbilityBonus, error)
}
