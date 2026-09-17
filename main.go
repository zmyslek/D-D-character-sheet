package main

import (
	"flag"
	"fmt"
	"os"

	characterdomain "mpp-zmyslek/character/character_domain"
	characterinfrastructure "mpp-zmyslek/character/character_infrastructure"
	classdomain "mpp-zmyslek/class/class_domain"
	racedomain "mpp-zmyslek/race/race_domain"
)

func usage() {
	fmt.Printf(`Usage:
	%s create -name CHARACTER_NAME -race RACE -class CLASS -level N -str N -dex N -con N -int N -wis N -cha N
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

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]

	switch cmd {
	case "create":
		createCmd := flag.NewFlagSet("create", flag.ContinueOnError)
		name := createCmd.String("name", "", "character name")
		race := createCmd.String("race", "", "character race")
		className := createCmd.String("class", "", "character class")
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
		if *name == "" || *race == "" || *className == "" {
			fmt.Println("name, race, and class are required")
			return
		}

		character := &characterdomain.Character{
			Name:  *name,
			Race:  racedomain.Race{RaceName: *race},
			Class: classdomain.Class{ClassName: *className},
			Level: *level,
			Str:   *strength,
			Dex:   *dexterity,
			Con:   *constitution,
			Int:   *intelligence,
			Wis:   *wisdom,
			Cha:   *charisma,
		}

		repository := characterinfrastructure.NewJSONCharacterRepository("characters.json")
		if err := repository.Create(character); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("saved character %s\n", character.Name)

	case "view":

	case "list":

	case "delete":

	case "equip":

	case "learn-spell":

	case "prepare-spell":

	default:
		usage()
		os.Exit(2)
	}
}
