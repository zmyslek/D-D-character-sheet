package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	characterdomain "mpp-zmyslek/character/character_domain"
	characterinfrastructure "mpp-zmyslek/character/character_infrastructure"
)

// usage prints the commands currently supported by the command-line interface.
func usage() {
	fmt.Printf(`Usage:
	%s create -name CHARACTER_NAME -race RACE -class CLASS -background BACKGROUND -skills SKILLS -level N -str N -dex N -con N -int N -wis N -cha N
  %s view -name CHARACTER_NAME
  %s list
  %s delete -name CHARACTER_NAME
  %s equip -name CHARACTER_NAME -weapon WEAPON_NAME -slot SLOT
  %s equip -name CHARACTER_NAME -armor ARMOR_NAME
  %s equip -name CHARACTER_NAME -shield SHIELD_NAME
  %s learn-spell -name CHARACTER_NAME -spell SPELL_NAME
  %s prepare-spell -name CHARACTER_NAME -spell SPELL_NAME 
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

// main reads the selected command and runs the matching application workflow.
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]

	switch cmd {
	case "create":
		createCmd := flag.NewFlagSet("create", flag.ContinueOnError)
		// Named flags make character creation readable and allow names with spaces.
		name := createCmd.String("name", "", "character name")
		race := createCmd.String("race", "", "character race")
		className := createCmd.String("class", "", "character class")
		background := createCmd.String("background", "", "character background")
		skills := createCmd.String("skills", "", "comma-separated skill proficiencies")
		level := createCmd.Int("level", 0, "character level")
		strength := createCmd.Int("str", 0, "strength")
		dexterity := createCmd.Int("dex", 0, "dexterity")
		constitution := createCmd.Int("con", 0, "constitution")
		intelligence := createCmd.Int("int", 0, "intelligence")
		wisdom := createCmd.Int("wis", 0, "wisdom")
		charisma := createCmd.Int("cha", 0, "charisma")
		if err := createCmd.Parse(os.Args[2:]); err != nil {
			return
		}
		// Check the name separately because it is the character's required identifier.
		if *name == "" {
			fmt.Println("name is required")
			os.Exit(2)
		}
		if *race == "" || *className == "" {
			fmt.Println("race and class are required")
			return
		}

		// Convert the parsed values into the domain object used by the repository.
		character := &characterdomain.Character{
			Name:               *name,
			Race:               racedomain.Race{RaceName: *race},
			Class:              classdomain.Class{ClassName: *className},
			Level:              *level,
			Background:         *background,
			SkillProficiencies: splitValues(*skills),
			Str:                *strength,
			Dex:                *dexterity,
			Con:                *constitution,
			Int:                *intelligence,
			Wis:                *wisdom,
			Cha:                *charisma,
		}

		repository := characterinfrastructure.NewJSONCharacterRepository("characters.json")
		// Keep character data in one JSON file so it survives between CLI runs.
		if err := repository.Create(character); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("saved character %s\n", character.Name)

	case "view":
		viewCmd := flag.NewFlagSet("view", flag.ContinueOnError)
		name := viewCmd.String("name", "", "character name")
		if err := viewCmd.Parse(os.Args[2:]); err != nil {
			return
		}

		if *name == "" {
			fmt.Println("name is required")
			return
		}

		repository := characterinfrastructure.NewJSONCharacterRepository("characters.json")
		character, err := repository.GetByName(*name)
		if err != nil {
			fmt.Println(err)
			return
		}
		printCharacter(character)

	case "list":
		// Reserved for listing all saved characters.

	case "delete":
		deleteCmd := flag.NewFlagSet("delete", flag.ContinueOnError)
		name := deleteCmd.String("name", "", "character name")
		if err := deleteCmd.Parse(os.Args[2:]); err != nil {
			return
		}
		if *name == "" {
			fmt.Println("name is required")
			return
		}

		repository := characterinfrastructure.NewJSONCharacterRepository("characters.json")
		if err := repository.Delete(*name); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("deleted %s\n", *name)

	case "equip":
		// Reserved for adding equipment to a character.

	case "learn-spell":
		// Reserved for adding a known spell to a character.

	case "prepare-spell":
		// Reserved for preparing a spell for use.

	default:
		usage()
		os.Exit(2)
	}
}

func splitValues(values string) []string {
	if values == "" {
		return nil
	}

	parts := strings.Split(values, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, strings.ToLower(trimmed))
		}
	}
	return result
}

func printCharacter(character *characterdomain.Character) {
	fmt.Printf("Name: %s\n", character.Name)
	fmt.Printf("Class: %s\n", strings.ToLower(character.Class.ClassName))
	fmt.Printf("Race: %s\n", strings.ToLower(character.Race.RaceName))
	fmt.Printf("Background: %s\n", strings.ToLower(character.Background))
	fmt.Printf("Level: %d\n", character.Level)
	fmt.Println("Ability scores:")
	fmt.Printf("  STR: %d (%s)\n", character.Str, formatModifier(characterdomain.AbilityModifier(character.Str)))
	fmt.Printf("  DEX: %d (%s)\n", character.Dex, formatModifier(characterdomain.AbilityModifier(character.Dex)))
	fmt.Printf("  CON: %d (%s)\n", character.Con, formatModifier(characterdomain.AbilityModifier(character.Con)))
	fmt.Printf("  INT: %d (%s)\n", character.Int, formatModifier(characterdomain.AbilityModifier(character.Int)))
	fmt.Printf("  WIS: %d (%s)\n", character.Wis, formatModifier(characterdomain.AbilityModifier(character.Wis)))
	fmt.Printf("  CHA: %d (%s)\n", character.Cha, formatModifier(characterdomain.AbilityModifier(character.Cha)))
	fmt.Printf("Proficiency bonus: %s\n", formatModifier(characterdomain.ProficiencyBonus(character.Level)))
	fmt.Printf("Skill proficiencies: %s\n", strings.Join(character.SkillProficiencies, ", "))
}

func formatModifier(modifier int) string {
	return fmt.Sprintf("%+d", modifier)
}
