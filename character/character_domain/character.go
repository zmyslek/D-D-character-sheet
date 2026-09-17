package characterdomain

import (
	classdomain "mpp-zmyslek/class/class_domain"
	racedomain "mpp-zmyslek/race/race_domain"
)

type Character struct {
	// Character contains the information needed to represent a player character.
	Name               string
	Race               racedomain.Race
	Class              classdomain.Class
	Level              int
	Background         string
	SkillProficiencies []string
	// Height      float64
	// Weight      float64
	// Eyes        string
	// Skin        string
	// Hair        string
	// Personality string
	// Alignment   string
	// Perception  int
	// Money       [5]int
	// Speed       int
	// HitPoints   int
	// Ability scores describe the character's core physical and mental abilities.
	Str int // strength
	Dex int // dexterity
	Con int // constitution
	Int int // intelligence
	Wis int // wisdom
	Cha int // charisma
}

// AbilityModifier returns the D&D 5e modifier for an ability score.
func AbilityModifier(score int) int {
	modifier := (score - 10) / 2
	if score < 10 && (score-10)%2 != 0 {
		modifier--
	}
	return modifier
}

// ProficiencyBonus returns the D&D 5e proficiency bonus for a character level.
func ProficiencyBonus(level int) int {
	if level < 1 {
		return 0
	}
	return 2 + (level-1)/4
}
