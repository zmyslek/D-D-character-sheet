package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// MainHandSlot identifies the character's primary weapon slot.
	MainHandSlot = "main hand"
	// OffHandSlot identifies the character's secondary weapon slot.
	OffHandSlot = "off hand"
)

// Character contains the persistent data and calculated inputs for one player character.
type Character struct {
	// Name is the unique display and lookup identifier for the character.
	Name string
	// Race and Class describe the character's selected D&D options.
	Race  string
	Class string
	// Level controls proficiency, spell slots, and other level-dependent rules.
	Level int
	// Background and SkillProficiencies describe the character's non-combat training.
	Background         string
	SkillProficiencies []string
	// Equipment fields store only the currently equipped items.
	MainHand       string
	OffHand        string
	Armor          string
	Shield         string
	KnownSpells    []string `json:",omitempty"`
	PreparedSpells []string `json:",omitempty"`
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

// CanCastSpells reports whether the character's class and level grant spellcasting.
func (character *Character) CanCastSpells() bool {
	className := strings.ToLower(strings.TrimSpace(character.Class))
	switch className {
	case "bard", "cleric", "druid", "sorcerer", "warlock", "wizard":
		return true
	case "paladin", "ranger":
		return character.Level >= 2
	default:
		return false
	}
}

// IsPreparedCaster reports whether the class prepares spells rather than learning them.
func (character *Character) IsPreparedCaster() bool {
	switch strings.ToLower(strings.TrimSpace(character.Class)) {
	case "cleric", "druid", "paladin", "wizard":
		return true
	default:
		return false
	}
}

// SpellSlots returns spell slots by spell level, using level zero for cantrips.
func (character *Character) SpellSlots() map[int]int {
	level := character.Level
	if level < 1 {
		return nil
	}
	className := strings.ToLower(strings.TrimSpace(character.Class))
	if className == "warlock" {
		slotLevel := 1
		slots := 1
		switch {
		case level >= 17:
			slots, slotLevel = 4, 5
		case level >= 11:
			slots, slotLevel = 3, 5
		case level >= 9:
			slots, slotLevel = 2, 5
		case level >= 7:
			slots, slotLevel = 2, 4
		case level >= 5:
			slots, slotLevel = 2, 3
		case level >= 3:
			slots, slotLevel = 2, 2
		case level >= 2:
			slots = 2
		}
		return map[int]int{0: character.CantripCount(), slotLevel: slots}
	}

	casterLevel := level
	if className == "paladin" || className == "ranger" {
		casterLevel = (level + 1) / 2
	}
	// Each inner slice contains the number of slots for spell levels 1 through 9.
	fullCasterSlots := [][]int{
		{}, {2}, {3}, {4}, {4, 2}, {4, 3}, {4, 3, 2}, {4, 3, 3, 1}, {4, 3, 3, 2},
		{4, 3, 3, 3, 1}, {4, 3, 3, 3, 2}, {4, 3, 3, 3, 2, 1}, {4, 3, 3, 3, 2, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 3, 2, 2, 1, 1},
	}
	if casterLevel > 20 {
		casterLevel = 20
	}
	slots := make(map[int]int)
	if cantrips := character.CantripCount(); cantrips > 0 {
		slots[0] = cantrips
	}
	for slotLevel, count := range fullCasterSlots[casterLevel] {
		slots[slotLevel+1] = count
	}
	return slots
}

// CantripCount returns the number of cantrips available to the character's class and level.
func (character *Character) CantripCount() int {
	level := character.Level
	className := strings.ToLower(strings.TrimSpace(character.Class))
	if className == "paladin" || className == "ranger" || level < 1 {
		return 0
	}
	if className == "warlock" {
		if level >= 10 {
			return 4
		}
		if level >= 4 {
			return 3
		}
		return 2
	}
	if className == "bard" {
		if level >= 10 {
			return 4
		}
		if level >= 4 {
			return 3
		}
		return 2
	}
	if level >= 10 {
		return 5
	}
	if level >= 4 {
		return 4
	}
	return 3
}

// ArmorClass calculates armor class from armor, Dexterity, class features, and a shield.
func (character *Character) ArmorClass() int {
	dexterity := AbilityModifier(character.Dex)
	armor := strings.ToLower(strings.TrimSpace(character.Armor))
	armorClass := 10 + dexterity

	switch armor {
	case "leather armor":
		armorClass = 11 + dexterity
	case "studded leather armor":
		armorClass = 12 + dexterity
	case "hide armor":
		armorClass = 12 + min(dexterity, 2)
	case "chain shirt":
		armorClass = 13 + min(dexterity, 2)
	case "scale mail", "breastplate":
		armorClass = 14 + min(dexterity, 2)
	case "half plate", "half plate armor":
		armorClass = 15 + min(dexterity, 2)
	case "ring mail":
		armorClass = 14
	case "chain mail":
		armorClass = 16
	case "splint armor":
		armorClass = 17
	case "plate armor":
		armorClass = 18
	case "":
		className := strings.ToLower(strings.TrimSpace(character.Class))
		if className == "barbarian" {
			armorClass = 10 + dexterity + AbilityModifier(character.Con)
		} else if className == "monk" {
			armorClass = 10 + dexterity + AbilityModifier(character.Wis)
		}
	}

	if character.Shield != "" {
		armorClass += 2
	}
	return armorClass
}

// SpellcastingAbility returns the governing ability name and score for the character's class.
func (character *Character) SpellcastingAbility() (string, int) {
	abilities := map[string]struct {
		name  string
		score int
	}{
		"bard": {"charisma", character.Cha}, "cleric": {"wisdom", character.Wis},
		"druid": {"wisdom", character.Wis}, "paladin": {"charisma", character.Cha},
		"ranger": {"wisdom", character.Wis}, "sorcerer": {"charisma", character.Cha},
		"warlock": {"charisma", character.Cha}, "wizard": {"intelligence", character.Int},
	}
	ability, ok := abilities[strings.ToLower(strings.TrimSpace(character.Class))]
	if !ok {
		return "", 0
	}
	return ability.name, ability.score
}

// min returns the smaller of two ability-derived values.
func min(first, second int) int {
	if first < second {
		return first
	}
	return second
}

// ApplyAbilityBonus applies a racial ability-score increase to the character.
func (character *Character) ApplyAbilityBonus(ability string, points int) error {
	if points == 0 {
		return nil
	}

	// Keep the source spelling tolerant of capitalization and surrounding spaces.
	switch strings.ToLower(strings.TrimSpace(ability)) {
	case "strength":
		character.Str += points
	case "dexterity":
		character.Dex += points
	case "constitution":
		character.Con += points
	case "intelligence":
		character.Int += points
	case "wisdom":
		character.Wis += points
	case "charisma":
		character.Cha += points
	default:
		return fmt.Errorf("unknown ability %q", ability)
	}

	return nil
}

// UnmarshalJSON also accepts the object-shaped Race and Class fields written by
// the previous domain model, so existing character files remain readable.
func (character *Character) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	// Older saved records stored race and class as nested objects instead of strings.
	decodeName := func(field string) (string, error) {
		value, ok := fields[field]
		if !ok {
			return "", nil
		}

		var name string
		if err := json.Unmarshal(value, &name); err == nil {
			return name, nil
		}

		var legacy struct {
			Name      string `json:"Name"`
			RaceName  string `json:"RaceName"`
			ClassName string `json:"ClassName"`
		}
		if err := json.Unmarshal(value, &legacy); err != nil {
			return "", err
		}
		if field == "Race" {
			return legacy.RaceName, nil
		}
		return legacy.ClassName, nil
	}

	race, err := decodeName("Race")
	if err != nil {
		return fmt.Errorf("invalid race: %w", err)
	}
	class, err := decodeName("Class")
	if err != nil {
		return fmt.Errorf("invalid class: %w", err)
	}

	// Decode the remaining properties through an alias to avoid recursive unmarshalling.
	delete(fields, "Race")
	delete(fields, "Class")
	normalized, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	type characterAlias Character
	var decoded characterAlias
	if err := json.Unmarshal(normalized, &decoded); err != nil {
		return err
	}
	*character = Character(decoded)
	character.Race = race
	character.Class = class
	return nil
}

// EquipWeapon stores a weapon in an available main-hand or off-hand slot.
func (character *Character) EquipWeapon(weaponName string, slot string) error {
	switch strings.ToLower(strings.TrimSpace(slot)) {
	case MainHandSlot:
		if character.MainHand != "" {
			return fmt.Errorf("main hand already occupied")
		}
		character.MainHand = weaponName
	case OffHandSlot:
		if character.OffHand != "" {
			return fmt.Errorf("off hand already occupied")
		}
		character.OffHand = weaponName
	default:
		return fmt.Errorf("slot must be %q or %q", MainHandSlot, OffHandSlot)
	}

	return nil
}

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

// HasStandardArray reports whether the six base scores use each Standard Array value once.
func (character *Character) HasStandardArray() bool {
	// Decrement a count for each score; duplicates and values outside the array fail.
	counts := map[int]int{8: 1, 10: 1, 12: 1, 13: 1, 14: 1, 15: 1}
	for _, score := range []int{character.Str, character.Dex, character.Con, character.Int, character.Wis, character.Cha} {
		if counts[score] == 0 {
			return false
		}
		counts[score]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
