package tests

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"mpp-zmyslek/internal/application"
	"mpp-zmyslek/internal/domain"
	"mpp-zmyslek/internal/infrastructure"
)

// newService builds the real commands and queries on a temporary characters
// file and the project's CSV data, wired like main.go, so these tests exercise
// the real repositories too.
func newService(t *testing.T) (*application.CharacterService, *application.CharacterQueries, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "characters.json")
	commands, queries := servicesFor(file, nil)
	return commands, queries, file
}

func servicesFor(file string, resources application.ResourceLookup) (*application.CharacterService, *application.CharacterQueries) {
	characters := infrastructure.NewJSONCharacterRepository(file)
	return application.NewCharacterService(
			characters,
			infrastructure.NewCSVClassRepository("../data/Classes.csv"),
			infrastructure.NewCSVRaceAbilityRepository("../data/RaceAbilityBonuses.csv"),
			infrastructure.NewCSVEquipmentRepository("../data/5e-SRD-Equipment.csv"),
			infrastructure.NewCSVSpellsRepository("../data/5e-SRD-Spells.csv"),
		),
		application.NewCharacterQueries(characters, resources)
}

func wizardRequest(name string) application.CreateRequest {
	return application.CreateRequest{
		Name: name, Race: "elf", Class: "wizard", Background: "acolyte", Level: 1,
		Scores:               domain.AbilityScores{Str: 8, Dex: 14, Con: 13, Int: 15, Wis: 12, Cha: 10},
		RequireStandardArray: true, RejectDuplicate: true,
	}
}

func TestServiceCreate(t *testing.T) {
	service, _, _ := newService(t)
	character, replayed, err := service.Create(wizardRequest("Mira"), "")
	if err != nil || replayed {
		t.Fatalf("Create = %v, %v", replayed, err)
	}
	if character.Scores().Dex != 16 {
		t.Errorf("elf Dex = %d, want 14 + 2", character.Scores().Dex)
	}
	want := []string{"arcana", "history", "insight", "religion"}
	if !reflect.DeepEqual(character.SkillProficiencies(), want) {
		t.Errorf("skills = %v, want %v", character.SkillProficiencies(), want)
	}

	if _, _, err := service.Create(wizardRequest("Mira"), ""); !errors.Is(err, application.ErrAlreadyExists) {
		t.Errorf("duplicate err = %v", err)
	}
	bad := wizardRequest("Bad")
	bad.Scores.Str = 18
	if _, _, err := service.Create(bad, ""); !errors.Is(err, application.ErrNotStandardArray) {
		t.Errorf("standard array err = %v", err)
	}
	unknown := wizardRequest("X")
	unknown.Race = "martian"
	if _, _, err := service.Create(unknown, ""); err == nil || err.Error() != `race "martian" not found` {
		t.Errorf("unknown race err = %v", err)
	}
}

func TestServiceIdempotencyKey(t *testing.T) {
	service, _, _ := newService(t)
	first, _, err := service.Create(wizardRequest("Mira"), "key-1")
	if err != nil {
		t.Fatal(err)
	}
	again, replayed, err := service.Create(wizardRequest("Mira"), "key-1")
	if err != nil || !replayed || again.Name() != first.Name() {
		t.Errorf("retry = %v, %v; want the first character replayed", replayed, err)
	}
	if _, _, err := service.Create(wizardRequest("Other"), "key-1"); !errors.Is(err, application.ErrIdempotencyKeyReused) {
		t.Errorf("same key, different request err = %v", err)
	}
}

// Many simultaneous retries with the same key must create one character.
func TestConcurrentRetriesCreateOnce(t *testing.T) {
	service, queries, _ := newService(t)
	const clients = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	created, replays := 0, 0
	for range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, replayed, err := service.Create(wizardRequest("Mira"), "same-key")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				t.Error(err)
			case replayed:
				replays++
			default:
				created++
			}
		}()
	}
	wg.Wait()
	all, _ := queries.GetAll()
	if created != 1 || replays != clients-1 || len(all) != 1 {
		t.Errorf("created %d, replayed %d, stored %d; want 1, %d, 1", created, replays, len(all), clients-1)
	}
}

// Each goroutine reads the character, adds a spell and saves it. Without the
// service mutex two of them could read the same version and one spell would
// be lost; with it every spell must be stored, once.
func TestConcurrentChangesAreNotLost(t *testing.T) {
	service, queries, _ := newService(t)
	if _, _, err := service.Create(wizardRequest("Mira"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeLevel("Mira", 5); err != nil {
		t.Fatal(err)
	}
	spells := []string{"Fire Bolt", "Light", "Magic Missile", "Shield", "Burning Hands", "Fireball", "Fly", "Haste"}
	var wg sync.WaitGroup
	for _, spell := range spells {
		for range 3 { // repeats must not create duplicates
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := service.AddSpell("Mira", spell, true); err != nil {
					t.Error(err)
				}
			}()
		}
	}
	wg.Wait()
	character, _ := queries.Get("Mira")
	if got := len(character.PreparedSpells()); got != len(spells) {
		t.Errorf("prepared %d spells %v, want %d", got, character.PreparedSpells(), len(spells))
	}
}

// newFighter creates the fighter "Conan" and returns the services.
func newFighter(t *testing.T) (*application.CharacterService, *application.CharacterQueries) {
	t.Helper()
	service, queries, _ := newService(t)
	fighter := wizardRequest("Conan")
	fighter.Class = "fighter"
	if _, _, err := service.Create(fighter, ""); err != nil {
		t.Fatal(err)
	}
	return service, queries
}

func TestServiceEquipment(t *testing.T) {
	service, queries := newFighter(t)
	if item, err := service.EquipWeapon("Conan", "longsword", "main hand"); err != nil || item.Name() != "Longsword" {
		t.Errorf("EquipWeapon = %v, %v", item, err)
	}
	if _, err := service.EquipWeapon("Conan", "chain mail", "off hand"); !errors.Is(err, domain.ErrNotAWeapon) {
		t.Errorf("armor as weapon err = %v", err)
	}
	if _, err := service.EquipArmor("Conan", "Half Plate Armor"); err != nil {
		t.Error(err)
	}
	if _, err := service.EquipShield("Conan", "shield"); err != nil {
		t.Error(err)
	}
	if _, err := service.EquipArmor("Conan", "banana"); err == nil || err.Error() != `equipment "banana" not found` {
		t.Errorf("unknown armor err = %v", err)
	}
	conan, _ := queries.Get("Conan")
	if conan.MainHand() != "Longsword" || conan.Armor() != "half plate" || conan.Shield() != "Shield" {
		t.Errorf("Conan = %+v", conan.Data())
	}
}

func TestServiceCreateWithSpells(t *testing.T) {
	service, queries, _ := newService(t)
	mira := wizardRequest("Mira")
	mira.PreparedSpells = []string{"Fire Bolt", "Magic Missile"}
	character, _, err := service.Create(mira, "")
	if err != nil || !reflect.DeepEqual(character.PreparedSpells(), []string{"fire bolt", "magic missile"}) {
		t.Fatalf("wizard with spells = %v, %v", character, err)
	}

	fighter := wizardRequest("Conan")
	fighter.Class = "fighter"
	invalid := map[string]application.CreateRequest{
		"fighter learns":   func() application.CreateRequest { r := fighter; r.KnownSpells = []string{"fire bolt"}; return r }(),
		"fighter prepares": func() application.CreateRequest { r := fighter; r.PreparedSpells = []string{"fire bolt"}; return r }(),
		"wizard learns": func() application.CreateRequest {
			r := wizardRequest("W1")
			r.KnownSpells = []string{"fire bolt"}
			return r
		}(),
		"too high for slots": func() application.CreateRequest {
			r := wizardRequest("W2")
			r.PreparedSpells = []string{"fireball"}
			return r
		}(),
		"other class's spell": func() application.CreateRequest {
			r := wizardRequest("W3")
			r.PreparedSpells = []string{"cure wounds"}
			return r
		}(),
	}
	for name, request := range invalid {
		if _, _, err := service.Create(request, ""); err == nil {
			t.Errorf("%s: was accepted", name)
		}
	}
	if all, _ := queries.GetAll(); len(all) != 1 {
		t.Errorf("stored %d characters, want only Mira", len(all))
	}
}

func TestServiceSpellsLevelAndDelete(t *testing.T) {
	service, _ := newFighter(t)
	if _, err := service.AddSpell("Conan", "unknown spell", false); err == nil || err.Error() != "this class can't cast spells" {
		t.Errorf("non caster err = %v (class check comes before the spell lookup)", err)
	}
	if _, err := service.ChangeLevel("Conan", 1); err != nil {
		t.Error(err)
	}
	if _, err := service.EquipWeapon("Nobody", "club", "main hand"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown character err = %v", err)
	}
	if err := service.Delete("Conan"); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete("Conan"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second delete err = %v", err)
	}
}

func TestServiceUpdate(t *testing.T) {
	service, queries, _ := newService(t)
	mira, _, err := service.Create(wizardRequest("Mira"), "")
	if err != nil {
		t.Fatal(err)
	}
	edit := application.CharacterEdit{
		Race: mira.Race(), Class: mira.Class(), Scores: mira.Scores(), Level: 3,
		MainHand: "dagger", Armor: "leather armor",
		PreparedSpells: []string{"Fire Bolt", "magic missile"},
	}
	updated, err := service.Update("Mira", edit)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Level() != 3 || updated.MainHand() != "Dagger" || updated.Armor() != "Leather Armor" ||
		!reflect.DeepEqual(updated.PreparedSpells(), []string{"fire bolt", "magic missile"}) {
		t.Errorf("updated = %+v", updated.Data())
	}

	invalid := []func(*application.CharacterEdit){
		func(e *application.CharacterEdit) { e.Race = "dwarf" },
		func(e *application.CharacterEdit) { e.Scores.Str = 20 },
		func(e *application.CharacterEdit) { e.Level = 2 },
		func(e *application.CharacterEdit) { e.MainHand = "chain mail" },
		func(e *application.CharacterEdit) { e.PreparedSpells = []string{"Fireball"} },
		func(e *application.CharacterEdit) { e.KnownSpells = []string{"light"} },
	}
	for index, change := range invalid {
		bad := edit
		change(&bad)
		if _, err := service.Update("Mira", bad); err == nil {
			t.Errorf("invalid edit %d was accepted", index)
		}
	}
	stored, _ := queries.Get("Mira")
	if !reflect.DeepEqual(stored.Data(), updated.Data()) {
		t.Error("rejected edits must not change the stored character")
	}
}

// The JSON repository writes through a temporary file and a rename, so a
// failed or finished write never leaves a broken or extra file behind.
func TestJSONRepositoryFiles(t *testing.T) {
	service, queries, file := newService(t)
	if _, _, err := service.Create(wizardRequest("Mira"), ""); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(file))
	if len(entries) != 1 {
		t.Errorf("directory contains %d files, want only characters.json", len(entries))
	}

	// Files written by the first version stored race and class as objects.
	legacy := `[{"Name":"Old","Race":{"RaceName":"dwarf"},"Class":{"ClassName":"cleric"},"Level":2,"Str":10,"Dex":10,"Con":12,"Int":8,"Wis":15,"Cha":13}]`
	if err := os.WriteFile(file, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := queries.Get("Old")
	if err != nil || old.Race() != "dwarf" || old.Class() != "cleric" {
		t.Errorf("legacy record = %+v, %v", old, err)
	}

	if err := os.WriteFile(file, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.GetAll(); err == nil || !strings.Contains(err.Error(), "failed to decode JSON") {
		t.Errorf("corrupt file err = %v", err)
	}
	if _, err := emptyQueries(t).GetAll(); err != nil {
		t.Errorf("a missing file should read as empty, got %v", err)
	}
}

func TestCSVRepositoriesWithRealData(t *testing.T) {
	classes := infrastructure.NewCSVClassRepository("../data/Classes.csv")
	bard, err := classes.GetByName("Bard")
	if err != nil || bard.SkillChoiceCount() != 3 || len(bard.SkillOptions()) != 18 {
		t.Errorf("bard = %v, %v", bard, err)
	}
	races := infrastructure.NewCSVRaceAbilityRepository("../data/RaceAbilityBonuses.csv")
	if bonuses, err := races.GetByRace("half orc"); err != nil || len(bonuses) != 2 {
		t.Errorf("half-orc bonuses = %v, %v", bonuses, err)
	}
	spells := infrastructure.NewCSVSpellsRepository("../data/5e-SRD-Spells.csv")
	if spell, err := spells.GetByName("FIREBALL"); err != nil || spell.Level() != 3 || !spell.AvailableTo("wizard") {
		t.Errorf("fireball = %v, %v", spell, err)
	}
	equipment := infrastructure.NewCSVEquipmentRepository("../data/5e-SRD-Equipment.csv")
	if shield, err := equipment.GetByName("shield"); err != nil || !shield.IsShield() || shield.IsArmor() {
		t.Errorf("shield = %v, %v", shield, err)
	}
	for _, missing := range []error{
		func() error { _, err := classes.GetByName("jester"); return err }(),
		func() error { _, err := spells.GetByName("wish upon a star"); return err }(),
		func() error { _, err := infrastructure.NewCSVClassRepository("nope.csv").GetByName("bard"); return err }(),
	} {
		if missing == nil {
			t.Error("expected an error")
		}
	}
}

func emptyQueries(t *testing.T) *application.CharacterQueries {
	_, queries := servicesFor(filepath.Join(t.TempDir(), "missing.json"), nil)
	return queries
}
