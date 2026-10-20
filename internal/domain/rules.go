package domain

import "strings"

// This file holds the SRD 5.1 rules as pure functions: the result depends only
// on the arguments, nothing is read from or written to anywhere else.

// AbilityModifier returns the D&D 5e modifier for an ability score.
func AbilityModifier(score int) int {
	// Go integer division truncates toward zero, so negative odd values need flooring.
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

// normalize makes names comparable regardless of case and surrounding spaces.
func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// fullCasterSlots is the SRD "spell slots per spell level" table used by
// bards, clerics, druids, sorcerers and wizards (half casters use it at half
// their level). Row n lists the slots for spell levels 1-9 at caster level n.
var fullCasterSlots = [21][]int{
	{}, {2}, {3}, {4, 2}, {4, 3}, {4, 3, 2}, {4, 3, 3}, {4, 3, 3, 1}, {4, 3, 3, 2},
	{4, 3, 3, 3, 1}, {4, 3, 3, 3, 2}, {4, 3, 3, 3, 2, 1}, {4, 3, 3, 3, 2, 1},
	{4, 3, 3, 3, 2, 1, 1}, {4, 3, 3, 3, 2, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 3, 1, 1, 1, 1},
	{4, 3, 3, 3, 3, 2, 1, 1, 1}, {4, 3, 3, 3, 3, 2, 2, 1, 1},
}

// spellSlots returns spell slots by spell level for a class, using level zero
// for the number of cantrips known.
func spellSlots(class string, level int) map[int]int {
	if !canCastSpells(class, level) {
		return nil
	}
	level = min(level, 20)
	slots := map[int]int{}
	if cantrips := cantripsKnown(class, level); cantrips > 0 {
		slots[0] = cantrips
	}
	switch normalize(class) {
	case "warlock":
		count, slotLevel := pactMagicSlots(level)
		slots[slotLevel] = count
		return slots
	case "paladin", "ranger":
		level = (level + 1) / 2 // half casters
	}
	for index, count := range fullCasterSlots[level] {
		slots[index+1] = count
	}
	return slots
}

// pactMagicSlots returns the warlock's slot count and slot level.
func pactMagicSlots(level int) (count, slotLevel int) {
	switch {
	case level >= 17:
		return 4, 5
	case level >= 11:
		return 3, 5
	case level >= 9:
		return 2, 5
	case level >= 7:
		return 2, 4
	case level >= 5:
		return 2, 3
	case level >= 3:
		return 2, 2
	case level >= 2:
		return 2, 1
	default:
		return 1, 1
	}
}

// cantripsKnown returns the cantrips known from the SRD class tables.
func cantripsKnown(class string, level int) int {
	// Cantrips known at levels 1, 4 and 10.
	var table [3]int
	switch normalize(class) {
	case "bard", "druid", "warlock":
		table = [3]int{2, 3, 4}
	case "cleric", "wizard":
		table = [3]int{3, 4, 5}
	case "sorcerer":
		table = [3]int{4, 5, 6}
	default:
		return 0
	}
	switch {
	case level >= 10:
		return table[2]
	case level >= 4:
		return table[1]
	case level >= 1:
		return table[0]
	default:
		return 0
	}
}

// canCastSpells reports whether a class has spellcasting at a level.
func canCastSpells(class string, level int) bool {
	switch normalize(class) {
	case "bard", "cleric", "druid", "sorcerer", "warlock", "wizard":
		return level >= 1
	case "paladin", "ranger":
		return level >= 2
	default:
		return false
	}
}

// spellcastingAbility returns the ability a class casts spells with, or "".
func spellcastingAbility(class string) string {
	switch normalize(class) {
	case "bard", "paladin", "sorcerer", "warlock":
		return "charisma"
	case "cleric", "druid", "ranger":
		return "wisdom"
	case "wizard":
		return "intelligence"
	default:
		return ""
	}
}

// Spellcasting is what a class can do with spells at one level, straight from
// the SRD class tables. Screens use it to show the rules before a character
// exists (the creation form) or while a level is being edited, so they never
// keep their own copy of the tables.
type Spellcasting struct {
	Ability  string      // "" for classes without spellcasting
	Prepares bool        // true for classes that prepare instead of learn spells
	Slots    map[int]int // slots by spell level, 0 = cantrips known; nil = can't cast
}

// SpellcastingFor returns the spellcasting of a class at a level.
func SpellcastingFor(class string, level int) Spellcasting {
	return Spellcasting{
		Ability:  spellcastingAbility(class),
		Prepares: isPreparedCaster(class),
		Slots:    spellSlots(class, level),
	}
}

// isPreparedCaster reports whether a class prepares spells rather than learning them.
func isPreparedCaster(class string) bool {
	switch normalize(class) {
	case "cleric", "druid", "paladin", "wizard":
		return true
	default:
		return false
	}
}

// highestSpellLevel returns the highest spell level the class can cast. A
// higher slot can always cast a lower-level spell (warlock Pact Magic).
func highestSpellLevel(class string, level int) int {
	highest := 0
	for spellLevel := range spellSlots(class, level) {
		highest = max(highest, spellLevel)
	}
	return highest
}

// armorClass applies the SRD armor table, Unarmored Defense and a shield.
func armorClass(class, armor string, hasShield bool, dexterity, constitution, wisdom int) int {
	dex := AbilityModifier(dexterity)
	result := 10 + dex
	switch normalize(armor) {
	case "padded armor", "leather armor":
		result = 11 + dex
	case "studded leather armor":
		result = 12 + dex
	case "hide armor":
		result = 12 + min(dex, 2)
	case "chain shirt":
		result = 13 + min(dex, 2)
	case "scale mail", "breastplate":
		result = 14 + min(dex, 2)
	case "half plate", "half plate armor":
		result = 15 + min(dex, 2)
	case "ring mail":
		result = 14
	case "chain mail":
		result = 16
	case "splint armor":
		result = 17
	case "plate armor":
		result = 18
	case "":
		// Unarmored Defense: barbarians may use a shield, monks may not.
		switch normalize(class) {
		case "barbarian":
			result += AbilityModifier(constitution)
		case "monk":
			if !hasShield {
				result += AbilityModifier(wisdom)
			}
		}
	}
	if hasShield {
		result += 2
	}
	return result
}

// backgroundSkills returns the skills granted by a background. The SRD only
// contains the Acolyte background.
func backgroundSkills(background string) []string {
	if normalize(background) == "acolyte" {
		return []string{"insight", "religion"}
	}
	return nil
}

// Skills lists the eighteen SRD skills in alphabetical order with the ability
// each one uses.
var Skills = []struct{ Name, Ability string }{
	{"acrobatics", "dexterity"}, {"animal handling", "wisdom"}, {"arcana", "intelligence"},
	{"athletics", "strength"}, {"deception", "charisma"}, {"history", "intelligence"},
	{"insight", "wisdom"}, {"intimidation", "charisma"}, {"investigation", "intelligence"},
	{"medicine", "wisdom"}, {"nature", "intelligence"}, {"perception", "wisdom"},
	{"performance", "charisma"}, {"persuasion", "charisma"}, {"religion", "intelligence"},
	{"sleight of hand", "dexterity"}, {"stealth", "dexterity"}, {"survival", "wisdom"},
}

// skillAbility returns the ability a skill uses, or "" for an unknown skill.
func skillAbility(skill string) string {
	for _, entry := range Skills {
		if entry.Name == normalize(skill) {
			return entry.Ability
		}
	}
	return ""
}
