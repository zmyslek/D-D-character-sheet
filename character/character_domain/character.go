package characterdomain

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	MainHandSlot = "main hand"
	OffHandSlot  = "off hand"
)

type Character struct {
	// Character contains the information needed to represent a player character.
	Name               string
	Race               string
	Class              string
	Level              int
	Background         string
	SkillProficiencies []string
	MainHand           string
	OffHand            string
	Armor              string
	Shield             string
	KnownSpells        []string `json:",omitempty"`
	PreparedSpells     []string `json:",omitempty"`
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

func (character *Character) CanCastSpells() bool {
	switch strings.ToLower(strings.TrimSpace(character.Class)) {
	case "bard", "cleric", "druid", "paladin", "ranger", "sorcerer", "warlock", "wizard":
		return true
	default:
		return false
	}
}

func (character *Character) IsPreparedCaster() bool {
	switch strings.ToLower(strings.TrimSpace(character.Class)) {
	case "cleric", "druid", "paladin", "wizard":
		return true
	default:
		return false
	}
}

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
		return map[int]int{slotLevel: slots}
	}

	casterLevel := level
	if className == "paladin" || className == "ranger" {
		casterLevel = (level + 1) / 2
	}
	fullCasterSlots := [][]int{
		{}, {2}, {3}, {4}, {4, 2}, {4, 3}, {4, 3, 2}, {4, 3, 3, 1}, {4, 3, 3, 2},
		{4, 3, 3, 3, 1}, {4, 3, 3, 3, 2}, {4, 3, 3, 3, 2, 1}, {4, 3, 3, 3, 2, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1, 1}, {4, 3, 3, 3, 2, 1, 1, 1, 1},
	}
	if casterLevel > 20 {
		casterLevel = 20
	}
	slots := make(map[int]int)
	for slotLevel, count := range fullCasterSlots[casterLevel] {
		slots[slotLevel+1] = count
	}
	return slots
}

// ApplyAbilityBonus applies a racial ability-score increase to the character.
func (character *Character) ApplyAbilityBonus(ability string, points int) error {
	if points == 0 {
		return nil
	}

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
