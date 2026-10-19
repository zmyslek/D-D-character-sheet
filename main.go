package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"mpp-zmyslek/internal/application"
	characterdomain "mpp-zmyslek/internal/domain"
	infrastructure "mpp-zmyslek/internal/infrastructure"
	"mpp-zmyslek/web"
)

// envOr returns the environment variable key, or fallback when it is unset.
// The defaults reproduce the original behaviour (files in the working directory);
// Docker sets the variables to use a data volume.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// app holds the wired services of one program run.
type app struct {
	commands *application.CharacterService
	queries  *application.CharacterQueries
}

// newApp reads the configuration and wires the repositories into the
// services. One repository instance is shared by commands and queries, so
// both use the same mutex.
func newApp() app {
	dataDir := envOr("MPP_DATA_DIR", "data")
	characters := infrastructure.NewJSONCharacterRepository(envOr("MPP_CHARACTERS_FILE", filepath.Join(dataDir, "characters.json")))
	return app{
		commands: application.NewCharacterService(
			characters,
			infrastructure.NewCSVClassRepository(filepath.Join(dataDir, "Classes.csv")),
			infrastructure.NewCSVRaceAbilityRepository(filepath.Join(dataDir, "RaceAbilityBonuses.csv")),
			infrastructure.NewCSVEquipmentRepository(filepath.Join(dataDir, "5e-SRD-Equipment.csv")),
			infrastructure.NewCSVSpellsRepository(filepath.Join(dataDir, "5e-SRD-Spells.csv")),
		),
		queries: application.NewCharacterQueries(characters,
			infrastructure.NewSRDAPIClient(envOr("MPP_SRD_API_URL", infrastructure.DefaultSRDAPIURL))),
	}
}

// usage prints the commands currently supported by the command-line interface.
func usage() {
	fmt.Printf(`Usage:
	%s serve [-addr ADDRESS]
	%s create -name CHARACTER_NAME -race RACE -class CLASS -background BACKGROUND -skills SKILLS -level N -str N -dex N -con N -int N -wis N -cha N
  %s view -name CHARACTER_NAME
  %s list
  %s delete -name CHARACTER_NAME
  %s equip -name CHARACTER_NAME -weapon WEAPON_NAME -slot SLOT
  %s equip -name CHARACTER_NAME -armor ARMOR_NAME
  %s equip -name CHARACTER_NAME -shield SHIELD_NAME
  %s learn-spell -name CHARACTER_NAME -spell SPELL_NAME
  %s prepare-spell -name CHARACTER_NAME -spell SPELL_NAME 
  %s level -name CHARACTER_NAME -level N
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

// Texts used by several commands.
const (
	nameFlagUsage   = "character name"
	msgNameRequired = "name is required"
)

// main reads the selected command and runs the matching function. Each
// command returns the process exit code.
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	commands := map[string]func([]string) int{
		"serve":         runServe,
		"create":        runCreate,
		"view":          runView,
		"list":          runList,
		"delete":        runDelete,
		"equip":         runEquip,
		"learn-spell":   func(args []string) int { return runSpell("learn-spell", args, false) },
		"prepare-spell": func(args []string) int { return runSpell("prepare-spell", args, true) },
		"level":         runLevel,
	}
	run, ok := commands[os.Args[1]]
	if !ok {
		usage()
		os.Exit(2)
	}
	os.Exit(run(os.Args[2:]))
}

// runServe starts the HTTP application with the same repositories as the CLI.
func runServe(args []string) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	address := flags.String("addr", envOr("MPP_ADDR", ":8080"), "HTTP listen address")
	if flags.Parse(args) != nil {
		return 0
	}
	if err := serve(*address); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runCreate applies race bonuses and records class/background skills. The CLI
// accepts any ability scores and replaces an existing character with the same
// name, exactly as the original program did.
func runCreate(args []string) int {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	// Named flags make character creation readable and allow names with spaces.
	name := flags.String("name", "", nameFlagUsage)
	race := flags.String("race", "", "character race")
	className := flags.String("class", "", "character class")
	background := flags.String("background", "acolyte", "character background")
	skills := flags.String("skills", "", "comma-separated skill proficiencies")
	level := flags.Int("level", 1, "character level")
	strength := flags.Int("str", 10, "strength")
	dexterity := flags.Int("dex", 0, "dexterity")
	constitution := flags.Int("con", 0, "constitution")
	intelligence := flags.Int("int", 0, "intelligence")
	wisdom := flags.Int("wis", 0, "wisdom")
	charisma := flags.Int("cha", 0, "charisma")
	if flags.Parse(args) != nil {
		return 0
	}
	// The name is the character's identifier, so a missing name is a usage error.
	if *name == "" {
		fmt.Println(msgNameRequired)
		return 2
	}
	if *race == "" || *className == "" {
		fmt.Println("race and class are required")
		return 0
	}
	character, _, err := newApp().commands.Create(application.CreateRequest{
		Name: *name, Race: *race, Class: *className, Background: *background, Level: *level,
		Scores: characterdomain.AbilityScores{
			Str: *strength, Dex: *dexterity, Con: *constitution,
			Int: *intelligence, Wis: *wisdom, Cha: *charisma,
		},
		Skills: splitValues(*skills),
	}, "")
	if err != nil {
		fmt.Println(err)
		return 0
	}
	fmt.Printf("saved character %s\n", character.Name())
	return 0
}

// runView prints one saved character's calculated sheet.
func runView(args []string) int {
	name, ok := parseName("view", args)
	if !ok {
		return 0
	}
	character, err := newApp().queries.Get(name)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	printCharacter(character)
	return 0
}

// runList prints the saved characters' names and their core identity fields.
func runList([]string) int {
	characters, err := newApp().queries.GetAll()
	if err != nil {
		fmt.Println(err)
		return 0
	}
	for _, character := range characters {
		fmt.Printf("%s - %s %s (level %d)\n", character.Name(), character.Race(), character.Class(), character.Level())
	}
	return 0
}

// runDelete removes one character by its unique name.
func runDelete(args []string) int {
	name, ok := parseName("delete", args)
	if !ok {
		return 0
	}
	if err := newApp().commands.Delete(name); err != nil {
		fmt.Println(err)
		return 0
	}
	fmt.Printf("deleted %s\n", name)
	return 0
}

// parseName parses a command that only takes -name. It prints the problem and
// returns false when the flags are invalid or the name is missing.
func parseName(command string, args []string) (string, bool) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	name := flags.String("name", "", nameFlagUsage)
	if flags.Parse(args) != nil {
		return "", false
	}
	if *name == "" {
		fmt.Println(msgNameRequired)
		return "", false
	}
	return *name, true
}

// runEquip puts exactly one weapon, armor or shield on a character.
func runEquip(args []string) int {
	flags := flag.NewFlagSet("equip", flag.ContinueOnError)
	name := flags.String("name", "", nameFlagUsage)
	weapon := flags.String("weapon", "", "weapon name")
	slot := flags.String("slot", "", "weapon slot")
	armor := flags.String("armor", "", "armor name")
	shield := flags.String("shield", "", "shield name")
	if flags.Parse(args) != nil {
		return 0
	}
	if *name == "" {
		fmt.Println(msgNameRequired)
		return 0
	}
	selected := 0
	for _, value := range []string{*weapon, *armor, *shield} {
		if value != "" {
			selected++
		}
	}
	if selected != 1 {
		fmt.Println("provide exactly one of weapon, armor, or shield")
		return 0
	}

	commands := newApp().commands
	switch {
	case *weapon != "":
		equipment, err := commands.EquipWeapon(*name, *weapon, *slot)
		if !reportEquipError(err, characterdomain.ErrNotAWeapon, "equipment %q is not a weapon\n", *weapon) {
			fmt.Printf("Equipped weapon %s to %s\n", strings.ToLower(equipment.Name()), strings.ToLower(strings.TrimSpace(*slot)))
		}
	case *armor != "":
		equipment, err := commands.EquipArmor(*name, *armor)
		if !reportEquipError(err, characterdomain.ErrNotArmor, "equipment %q is not armor\n", *armor) {
			fmt.Printf("Equipped armor %s\n", strings.ToLower(equipment.Name()))
		}
	default:
		equipment, err := commands.EquipShield(*name, *shield)
		if !reportEquipError(err, characterdomain.ErrNotAShield, "equipment %q is not a shield\n", *shield) {
			fmt.Printf("Equipped shield %s\n", strings.ToLower(equipment.Name()))
		}
	}
	return 0
}

// reportEquipError prints an equip error and reports whether there was one.
// A wrong item kind is reported with the name exactly as the user typed it.
func reportEquipError(err, wrongKind error, format, typed string) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, wrongKind):
		fmt.Printf(format, typed)
	default:
		fmt.Println(err)
	}
	return true
}

// runSpell handles learn-spell and prepare-spell; the class, level and spell
// list rules are checked by the domain.
func runSpell(command string, args []string, prepare bool) int {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	name := flags.String("name", "", nameFlagUsage)
	spellName := flags.String("spell", "", "spell name")
	if flags.Parse(args) != nil {
		return 0
	}
	learnOrPrepareSpell(*name, *spellName, prepare)
	return 0
}

// runLevel changes a character's level; the proficiency bonus follows from it.
func runLevel(args []string) int {
	flags := flag.NewFlagSet("level", flag.ContinueOnError)
	name := flags.String("name", "", nameFlagUsage)
	level := flags.Int("level", 0, "new level")
	if flags.Parse(args) != nil {
		return 0
	}
	if *name == "" {
		fmt.Println(msgNameRequired)
		return 0
	}
	character, err := newApp().commands.ChangeLevel(*name, *level)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	fmt.Printf("%s is now level %d (proficiency bonus %+d)\n", character.Name(), character.Level(), character.ProficiencyBonus())
	return 0
}

// splitValues turns a comma-separated CLI flag into normalized non-empty entries.
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

// printCharacter formats a character and its calculated combat information for the CLI.
func printCharacter(character *characterdomain.Character) {
	scores := character.Scores()
	fmt.Printf("Name: %s\n", character.Name())
	fmt.Printf("Class: %s\n", strings.ToLower(character.Class()))
	fmt.Printf("Race: %s\n", strings.ToLower(character.Race()))
	fmt.Printf("Background: %s\n", strings.ToLower(character.Background()))
	fmt.Printf("Level: %d\n", character.Level())
	fmt.Println("Ability scores:")
	for _, ability := range []struct {
		label string
		score int
	}{{"STR", scores.Str}, {"DEX", scores.Dex}, {"CON", scores.Con}, {"INT", scores.Int}, {"WIS", scores.Wis}, {"CHA", scores.Cha}} {
		fmt.Printf("  %s: %d (%s)\n", ability.label, ability.score, formatModifier(characterdomain.AbilityModifier(ability.score)))
	}
	fmt.Printf("Proficiency bonus: %s\n", formatModifier(character.ProficiencyBonus()))
	fmt.Printf("Skill proficiencies: %s\n", strings.Join(character.SkillProficiencies(), ", "))
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
	for _, item := range []struct{ label, name string }{
		{"Main hand", character.MainHand()}, {"Off hand", character.OffHand()},
		{"Armor", character.Armor()}, {"Shield", character.Shield()},
	} {
		if item.name != "" {
			fmt.Printf("%s: %s\n", item.label, strings.ToLower(item.name))
		}
	}
	if character.CanCastSpells() {
		ability, _ := character.SpellcastingAbility()
		fmt.Printf("Spellcasting ability: %s\n", ability)
		fmt.Printf("Spell save DC: %d\n", character.SpellSaveDC())
		fmt.Printf("Spell attack bonus: %s\n", formatModifier(character.SpellAttackBonus()))
	}
	fmt.Printf("Armor class: %d\n", character.ArmorClass())
	fmt.Printf("Initiative bonus: %d\n", character.Initiative())
	fmt.Printf("Passive perception: %d\n", character.PassivePerception())
}

// learnOrPrepareSpell validates spellcasting rules and stores the selected spell.
func learnOrPrepareSpell(name, spellName string, prepare bool) {
	if name == "" || spellName == "" {
		fmt.Println("name and spell are required")
		return
	}
	spell, err := newApp().commands.AddSpell(name, spellName, prepare)
	if err != nil {
		fmt.Println(err)
		return
	}
	if prepare {
		fmt.Printf("Prepared spell %s\n", strings.ToLower(spell.Name()))
		return
	}
	fmt.Printf("Learned spell %s\n", strings.ToLower(spell.Name()))
}

// formatModifier renders a numeric modifier with an explicit plus or minus sign.
func formatModifier(modifier int) string {
	return fmt.Sprintf("%+d", modifier)
}

// serve runs the web server until it receives SIGINT or SIGTERM (Ctrl+C or
// "docker stop"). It then stops accepting connections and gives running
// requests up to 10 seconds to finish, so no write is cut off halfway.
func serve(address string) error {
	services := newApp()
	server, err := web.NewServer(services.commands, services.queries)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	httpServer := &http.Server{Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The server runs in its own goroutine; main waits for it to fail or for a signal.
	served := make(chan error, 1)
	go func() { served <- httpServer.Serve(listener) }()
	fmt.Printf("Character manager listening on %s\n", address)

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
