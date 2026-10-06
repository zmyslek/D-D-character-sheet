package web

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	characterdomain "mpp-zmyslek/character/character_domain"
)

// abilityRow is one ability score box on the view page.
type abilityRow struct {
	Name  string
	Score int
	Mod   string
}

// saveRow is one saving throw line on the view page.
type saveRow struct {
	Name string
	Mod  string
}

// skillRow is one skill line on the view page.
type skillRow struct {
	Name       string
	Ability    string
	Mod        string
	Proficient bool
}

// sheetView holds everything view.html needs, already formatted for display.
type sheetView struct {
	Name            string
	ClassLevel      string
	RaceLabel       string
	BackgroundLabel string
	Abilities       []abilityRow
	Saves           []saveRow
	Skills          []skillRow
	ProficiencyText string
	InitiativeText  string
	ArmorClass      int
	PassiveWisdom   int
	OtherProfs      string
	Attacks         [3]string
	SpellText       string
	EquipmentText   string
	FeaturesText    string
}

var abilityTable = []struct{ Name, Short string }{
	{"Strength", "Str"}, {"Dexterity", "Dex"}, {"Constitution", "Con"},
	{"Intelligence", "Int"}, {"Wisdom", "Wis"}, {"Charisma", "Cha"},
}

var skillTable = []struct{ Name, Ability string }{
	{"Acrobatics", "Dex"}, {"Animal Handling", "Wis"}, {"Arcana", "Int"},
	{"Athletics", "Str"}, {"Deception", "Cha"}, {"History", "Int"},
	{"Insight", "Wis"}, {"Intimidation", "Cha"}, {"Investigation", "Int"},
	{"Medicine", "Wis"}, {"Nature", "Int"}, {"Perception", "Wis"},
	{"Performance", "Cha"}, {"Persuasion", "Cha"}, {"Religion", "Int"},
	{"Sleight of Hand", "Dex"}, {"Stealth", "Dex"}, {"Survival", "Wis"},
}

// makeSheetView converts a character into the display model for view.html.
func makeSheetView(character *characterdomain.Character) sheetView {
	base := makeCharacterView(character)
	proficiency := base.ProficiencyBonus

	view := sheetView{
		Name:            character.Name,
		ClassLevel:      fmt.Sprintf("%s %d", titleCase(character.Class), character.Level),
		RaceLabel:       titleCase(character.Race),
		BackgroundLabel: titleCase(character.Background),
		ProficiencyText: signed(proficiency),
		InitiativeText:  signed(base.Initiative),
		ArmorClass:      base.ArmorClass,
		PassiveWisdom:   base.PassiveWisdom,
	}

	for _, ability := range abilityTable {
		score := abilityScore(character, ability.Short)
		modifier := signed(characterdomain.AbilityModifier(score))
		view.Abilities = append(view.Abilities, abilityRow{Name: ability.Name, Score: score, Mod: modifier})
		view.Saves = append(view.Saves, saveRow{Name: ability.Name, Mod: modifier})
	}

	proficient := make(map[string]bool)
	profNames := make([]string, 0, len(character.SkillProficiencies))
	for _, skill := range character.SkillProficiencies {
		normalized := strings.ToLower(strings.TrimSpace(skill))
		if normalized == "" {
			continue
		}
		proficient[normalized] = true
		profNames = append(profNames, titleCase(normalized))
	}
	view.OtherProfs = strings.Join(profNames, ", ")

	for _, skill := range skillTable {
		modifier := characterdomain.AbilityModifier(abilityScore(character, skill.Ability))
		isProficient := proficient[strings.ToLower(skill.Name)]
		if isProficient {
			modifier += proficiency
		}
		view.Skills = append(view.Skills, skillRow{
			Name:       skill.Name,
			Ability:    skill.Ability,
			Mod:        signed(modifier),
			Proficient: isProficient,
		})
	}

	view.Attacks[0] = titleCase(character.MainHand)
	view.Attacks[1] = titleCase(character.OffHand)

	var equipment []string
	if character.Armor != "" {
		equipment = append(equipment, "Armor: "+titleCase(character.Armor))
	}
	if character.Shield != "" {
		equipment = append(equipment, "Shield: "+titleCase(character.Shield))
	}
	if character.MainHand != "" {
		equipment = append(equipment, "Weapon: "+titleCase(character.MainHand))
	}
	if character.OffHand != "" {
		equipment = append(equipment, "Off-hand: "+titleCase(character.OffHand))
	}
	view.EquipmentText = strings.Join(equipment, "\n")

	var spellLines, featureLines []string
	if character.CanCastSpells() {
		spellLines = append(spellLines, fmt.Sprintf("Spellcasting: %s | Save DC %d | Attack %s",
			titleCase(base.SpellAbility), base.SpellSaveDC, signed(base.SpellAttack)))
		if len(character.KnownSpells) > 0 {
			spellLines = append(spellLines, "Known spells: "+titleCaseList(character.KnownSpells))
		}
		if len(character.PreparedSpells) > 0 {
			spellLines = append(spellLines, "Prepared spells: "+titleCaseList(character.PreparedSpells))
		}

		slots := character.SpellSlots()
		levels := make([]int, 0, len(slots))
		for level := range slots {
			levels = append(levels, level)
		}
		sort.Ints(levels)
		featureLines = append(featureLines, "Spell slots:")
		for _, level := range levels {
			label := fmt.Sprintf("Level %d", level)
			if level == 0 {
				label = "Cantrips"
			}
			featureLines = append(featureLines, fmt.Sprintf("  %s: %d", label, slots[level]))
		}
	}
	view.SpellText = strings.Join(spellLines, "\n")
	view.FeaturesText = strings.Join(featureLines, "\n")

	return view
}

// abilityScore returns the raw score for a three-letter ability code such as "Str".
func abilityScore(character *characterdomain.Character, short string) int {
	switch short {
	case "Str":
		return character.Str
	case "Dex":
		return character.Dex
	case "Con":
		return character.Con
	case "Int":
		return character.Int
	case "Wis":
		return character.Wis
	case "Cha":
		return character.Cha
	}
	return 0
}

// signed renders a number with an explicit plus or minus sign.
func signed(value int) string {
	return fmt.Sprintf("%+d", value)
}

// titleCase capitalizes each word of a stored (usually lowercase) value.
func titleCase(value string) string {
	words := strings.Fields(strings.ToLower(value))
	for index, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[index] = string(runes)
	}
	return strings.Join(words, " ")
}

func titleCaseList(values []string) string {
	titled := make([]string, 0, len(values))
	for _, value := range values {
		titled = append(titled, titleCase(value))
	}
	return strings.Join(titled, ", ")
}
