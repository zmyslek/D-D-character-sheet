package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	characterdomain "mpp-zmyslek/character/character_domain"
	characterinfrastructure "mpp-zmyslek/character/character_infrastructure"
	equipmentinfrastructure "mpp-zmyslek/equipment/equipment_infrastructure"
	spellsinfrastructure "mpp-zmyslek/spells/spells_infrastructure"
	web "mpp-zmyslek/web"
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
	%s serve -addr ADDRESS
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
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
		background := createCmd.String("background", "acolyte", "character background")
		skills := createCmd.String("skills", "", "comma-separated skill proficiencies")
		level := createCmd.Int("level", 1, "character level")
		strength := createCmd.Int("str", 10, "strength")
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
			Race:               *race,
			Class:              *className,
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

		raceAbilityRepository := characterinfrastructure.NewCSVRaceAbilityRepository("csvs/RaceAbilityBonuses.csv")
		raceBonuses, err := raceAbilityRepository.GetByRace(character.Race)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, bonus := range raceBonuses {
			if err := character.ApplyAbilityBonus(bonus.Ability, bonus.Points); err != nil {
				fmt.Println(err)
				return
			}
		}

		classRepository := characterinfrastructure.NewCSVClassRepository("csvs/Classes.csv")
		class, err := classRepository.GetByName(character.Class)
		if err != nil {
			fmt.Println(err)
			return
		}
		character.SkillProficiencies = addSkillProficiencies(
			character.SkillProficiencies,
			class.SkillOptions,
			class.SkillChoiceCount,
		)
		character.SkillProficiencies = addBackgroundSkills(character.SkillProficiencies, character.Background)

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
		repository := characterinfrastructure.NewJSONCharacterRepository("characters.json")
		characters, err := repository.GetAll()
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, character := range characters {
			fmt.Printf("%s - %s %s (level %d)\n", character.Name, character.Race, character.Class, character.Level)
		}

	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ContinueOnError)
		address := serveCmd.String("addr", ":8080", "HTTP listen address")
		if err := serveCmd.Parse(os.Args[2:]); err != nil {
			return
		}
		repository := characterinfrastructure.NewJSONCharacterRepository("characters.json")
		server, err := web.NewServer(repository)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("web server listening on http://%s\n", *address)
		if err := http.ListenAndServe(*address, server.Handler()); err != nil {
			fmt.Println(err)
		}

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
		equipCmd := flag.NewFlagSet("equip", flag.ContinueOnError)
		name := equipCmd.String("name", "", "character name")
		weapon := equipCmd.String("weapon", "", "weapon name")
		slot := equipCmd.String("slot", "", "weapon slot")
		armor := equipCmd.String("armor", "", "armor name")
		shield := equipCmd.String("shield", "", "shield name")
		if err := equipCmd.Parse(os.Args[2:]); err != nil {
			return
		}
		if *name == "" {
			fmt.Println("name is required")
			return
		}

		selected := 0
		if *weapon != "" {
			selected++
		}
		if *armor != "" {
			selected++
		}
		if *shield != "" {
			selected++
		}
		if selected != 1 {
			fmt.Println("provide exactly one of weapon, armor, or shield")
			return
		}

		characterRepository := characterinfrastructure.NewJSONCharacterRepository("characters.json")
		character, err := characterRepository.GetByName(*name)
		if err != nil {
			fmt.Println(err)
			return
		}
		equipmentRepository := equipmentinfrastructure.NewCSVEquipmentRepository("csvs/5e-SRD-Equipment.csv")

		if *weapon != "" {
			equipment, err := equipmentRepository.GetByName(*weapon)
			if err != nil {
				fmt.Println(err)
				return
			}
			if !strings.EqualFold(equipment.Type, "Weapon") {
				fmt.Printf("equipment %q is not a weapon\n", *weapon)
				return
			}
			if err := character.EquipWeapon(equipment.Name, *slot); err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("Equipped weapon %s to %s\n", strings.ToLower(equipment.Name), strings.ToLower(strings.TrimSpace(*slot)))
		}
		if *armor != "" {
			equipment, err := equipmentRepository.GetByName(*armor)
			if err != nil {
				fmt.Println(err)
				return
			}
			if !strings.EqualFold(equipment.Type, "Armor") || strings.EqualFold(equipment.Name, "Shield") {
				fmt.Printf("equipment %q is not armor\n", *armor)
				return
			}
			character.Armor = equipment.Name
			fmt.Printf("Equipped armor %s\n", strings.ToLower(equipment.Name))
		}
		if *shield != "" {
			equipment, err := equipmentRepository.GetByName(*shield)
			if err != nil {
				fmt.Println(err)
				return
			}
			if !strings.EqualFold(equipment.Name, "Shield") {
				fmt.Printf("equipment %q is not a shield\n", *shield)
				return
			}
			character.Shield = equipment.Name
			fmt.Printf("Equipped shield %s\n", strings.ToLower(equipment.Name))
		}

		if err := characterRepository.Update(character); err != nil {
			fmt.Println(err)
			return
		}

	case "learn-spell":
		spellCmd := flag.NewFlagSet("learn-spell", flag.ContinueOnError)
		name := spellCmd.String("name", "", "character name")
		spellName := spellCmd.String("spell", "", "spell name")
		if err := spellCmd.Parse(os.Args[2:]); err != nil {
			return
		}
		learnOrPrepareSpell(*name, *spellName, false)

	case "prepare-spell":
		spellCmd := flag.NewFlagSet("prepare-spell", flag.ContinueOnError)
		name := spellCmd.String("name", "", "character name")
		spellName := spellCmd.String("spell", "", "spell name")
		if err := spellCmd.Parse(os.Args[2:]); err != nil {
			return
		}
		learnOrPrepareSpell(*name, *spellName, true)

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

func addSkillProficiencies(existing, options []string, choiceCount int) []string {
	result := make([]string, 0, len(existing)+choiceCount)
	for _, skill := range existing {
		normalized := strings.ToLower(strings.TrimSpace(skill))
		if normalized != "" {
			result = append(result, normalized)
		}
	}
	selected := 0
	for _, option := range options {
		normalized := strings.ToLower(strings.TrimSpace(option))
		if selected >= choiceCount {
			break
		}
		if normalized == "" {
			continue
		}
		result = append(result, normalized)
		selected++
	}
	return result
}

func addBackgroundSkills(existing []string, background string) []string {
	backgroundSkills := map[string][]string{
		"acolyte": {"insight", "religion"},
	}
	return addSkillProficiencies(existing, backgroundSkills[strings.ToLower(strings.TrimSpace(background))], 2)
}

func printCharacter(character *characterdomain.Character) {
	fmt.Printf("Name: %s\n", character.Name)
	fmt.Printf("Class: %s\n", strings.ToLower(character.Class))
	fmt.Printf("Race: %s\n", strings.ToLower(character.Race))
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
	if character.CanCastSpells() {
		slots := character.SpellSlots()
		levels := make([]int, 0, len(slots))
		for level := range slots {
			levels = append(levels, level)
		}
		sort.Ints(levels)
		fmt.Println("Spell slots:")
		for _, level := range levels {
			fmt.Printf("  Level %d: %d\n", level, slots[level])
		}
	}
	if character.MainHand != "" {
		fmt.Printf("Main hand: %s\n", strings.ToLower(character.MainHand))
	}
	if character.OffHand != "" {
		fmt.Printf("Off hand: %s\n", strings.ToLower(character.OffHand))
	}
	if character.Armor != "" {
		fmt.Printf("Armor: %s\n", strings.ToLower(character.Armor))
	}
	if character.Shield != "" {
		fmt.Printf("Shield: %s\n", strings.ToLower(character.Shield))
	}
	if character.CanCastSpells() {
		ability, score := character.SpellcastingAbility()
		fmt.Printf("Spellcasting ability: %s\n", ability)
		fmt.Printf("Spell save DC: %d\n", 8+characterdomain.ProficiencyBonus(character.Level)+characterdomain.AbilityModifier(score))
		fmt.Printf("Spell attack bonus: %s\n", formatModifier(characterdomain.ProficiencyBonus(character.Level)+characterdomain.AbilityModifier(score)))
	}
	fmt.Printf("Armor class: %d\n", character.ArmorClass())
	fmt.Printf("Initiative bonus: %d\n", characterdomain.AbilityModifier(character.Dex))
	fmt.Printf("Passive perception: %d\n", 10+characterdomain.AbilityModifier(character.Wis))
}

func learnOrPrepareSpell(name, spellName string, prepare bool) {
	if name == "" || spellName == "" {
		fmt.Println("name and spell are required")
		return
	}
	characterRepository := characterinfrastructure.NewJSONCharacterRepository("characters.json")
	character, err := characterRepository.GetByName(name)
	if err != nil {
		fmt.Println(err)
		return
	}
	if !character.CanCastSpells() {
		fmt.Println("this class can't cast spells")
		return
	}
	if prepare && !character.IsPreparedCaster() {
		fmt.Println("this class learns spells and can't prepare them")
		return
	}
	if !prepare && character.IsPreparedCaster() {
		fmt.Println("this class prepares spells and can't learn them")
		return
	}

	spellRepository := spellsinfrastructure.NewCSVSpellsRepository("csvs/5e-SRD-Spells.csv")
	spell, err := spellRepository.GetByName(spellName)
	if err != nil {
		fmt.Println(err)
		return
	}
	className := strings.ToLower(strings.TrimSpace(character.Class))
	classAvailable := false
	for _, availableClass := range spell.Classes {
		if availableClass == className {
			classAvailable = true
			break
		}
	}
	if !classAvailable {
		fmt.Printf("spell %s is not available to this class\n", strings.ToLower(spell.Name))
		return
	}
	if spell.Level > 0 {
		available := character.SpellSlots()
		if available[spell.Level] == 0 {
			fmt.Println("the spell has higher level than the available spell slots")
			return
		}
	}
	if prepare {
		character.PreparedSpells = append(character.PreparedSpells, strings.ToLower(spell.Name))
		if err := characterRepository.Update(character); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Prepared spell %s\n", strings.ToLower(spell.Name))
		return
	}
	character.KnownSpells = append(character.KnownSpells, strings.ToLower(spell.Name))
	if err := characterRepository.Update(character); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Learned spell %s\n", strings.ToLower(spell.Name))
}

func formatModifier(modifier int) string {
	return fmt.Sprintf("%+d", modifier)
}
