package tests

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"mpp-zmyslek/internal/application"
	"mpp-zmyslek/internal/domain"
	"mpp-zmyslek/internal/infrastructure"
	"mpp-zmyslek/web"
)

// openCharacters opens a characters file in the format its extension names,
// so every service test can run on either storage.
func openCharacters(file string) domain.CharacterRepository {
	if strings.HasSuffix(file, ".csv") {
		return infrastructure.NewCSVCharacterRepository(file)
	}
	return infrastructure.NewJSONCharacterRepository(file)
}

// storages are the character storages that must behave the same.
var storages = []string{"characters.json", "characters.csv"}

// awkward is a character whose values need quoting or escaping in a file:
// commas, quotes, semicolon-free lists, a formula-like name and non-ASCII text.
var awkward = domain.CharacterData{
	Name: `=Zoë "Quick", the Bold`, Race: "half-elf", Class: "bard", Level: 7, Background: "acolyte",
	SkillProficiencies: []string{"sleight of hand", "insight", "religion"},
	MainHand:           "Crossbow, heavy", OffHand: "Dagger", Armor: "Studded Leather Armor", Shield: "Shield",
	KnownSpells: []string{"vicious mockery", "Tasha's Hideous Laughter"},
	Str:         8, Dex: 16, Con: 13, Int: 12, Wis: 10, Cha: 17,
}

// The CSV storage supports the same core operations, with the same results
// and errors, as the JSON storage.
func TestCharacterStoragesBehaveTheSame(t *testing.T) {
	for _, name := range storages {
		t.Run(name, func(t *testing.T) {
			repository := openCharacters(filepath.Join(t.TempDir(), name))
			if all, err := repository.GetAll(); err != nil || len(all) != 0 {
				t.Fatalf("missing file: %v, %v; want no characters", all, err)
			}

			if err := repository.Create(domain.RestoreCharacter(awkward)); err != nil {
				t.Fatal(err)
			}
			mira := domain.CharacterData{Name: "Mira", Race: "elf", Class: "wizard", Level: 1, Str: 8, Dex: 16, Con: 13, Int: 15, Wis: 12, Cha: 10}
			if err := repository.Create(domain.RestoreCharacter(mira)); err != nil {
				t.Fatal(err)
			}
			got, err := repository.GetByName(awkward.Name)
			if err != nil || !reflect.DeepEqual(got.Data(), awkward) {
				t.Fatalf("round trip:\n got %+v, %v\nwant %+v", got.Data(), err, awkward)
			}

			// Create with an existing name replaces it; Update changes it.
			mira.Level = 2
			if err := repository.Create(domain.RestoreCharacter(mira)); err != nil {
				t.Fatal(err)
			}
			mira.Level, mira.PreparedSpells = 3, []string{"fire bolt"}
			if err := repository.Update(domain.RestoreCharacter(mira)); err != nil {
				t.Fatal(err)
			}
			all, err := repository.GetAll()
			if err != nil || len(all) != 2 || all[0].Name() != awkward.Name || !reflect.DeepEqual(all[1].Data(), mira) {
				t.Errorf("GetAll = %d characters, %v; second %+v", len(all), err, all[1].Data())
			}

			nobody := domain.RestoreCharacter(domain.CharacterData{Name: "Nobody"})
			if err := repository.Update(nobody); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("update unknown: %v", err)
			}
			if _, err := repository.GetByName("Nobody"); !errors.Is(err, domain.ErrNotFound) || err.Error() != `character "Nobody" not found` {
				t.Errorf("get unknown: %v", err)
			}
			if err := repository.Delete(awkward.Name); err != nil {
				t.Fatal(err)
			}
			if err := repository.Delete(awkward.Name); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("delete twice: %v", err)
			}
			if all, _ := repository.GetAll(); len(all) != 1 || all[0].Name() != "Mira" {
				t.Errorf("after delete: %d characters", len(all))
			}
		})
	}
}

// The file is plain, readable CSV: a header, one row per character, lists
// separated by "; ", no JSON inside.
func TestCSVFileIsHumanReadable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "characters.csv")
	if err := infrastructure.NewCSVCharacterRepository(file).Create(domain.RestoreCharacter(awkward)); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	want := "Name,Race,Class,Level,Background,STR,DEX,CON,INT,WIS,CHA,Skill Proficiencies,Main Hand,Off Hand,Armor,Shield,Known Spells,Prepared Spells\n" +
		`"'=Zoë ""Quick"", the Bold",half-elf,bard,7,acolyte,8,16,13,12,10,17,sleight of hand; insight; religion,"Crossbow, heavy",Dagger,Studded Leather Armor,Shield,vicious mockery; Tasha's Hideous Laughter,` + "\n"
	if string(content) != want {
		t.Errorf("file:\n%s\nwant:\n%s", content, want)
	}
}

// A file saved by a spreadsheet program may start with a byte order mark,
// have its columns reordered, contain extra columns, leave out empty cells at
// the end of a row, and pad lists with spaces. It is still read correctly,
// and the next save writes the standard layout.
func TestCSVFileEditedInASpreadsheet(t *testing.T) {
	file := filepath.Join(t.TempDir(), "characters.csv")
	edited := "\xef\xbb\xbfNotes, name ,CLASS,Race,Level,Background,STR,DEX,CON,INT,WIS,CHA,Main Hand,Off Hand,Armor,Shield,Known Spells,Prepared Spells,Skill Proficiencies\r\n" +
		"my favourite,Mira,wizard,elf,1,acolyte,8,16,13,15,12,10,Dagger,,,,, fire bolt ;; magic missile ,arcana;history\r\n" +
		"short row,Conan,fighter,human,3,,15,14,13,12,10,8\r\n"
	if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	repository := infrastructure.NewCSVCharacterRepository(file)
	all, err := repository.GetAll()
	if err != nil || len(all) != 2 {
		t.Fatalf("GetAll = %v, %v", all, err)
	}
	wantMira := domain.CharacterData{
		Name: "Mira", Race: "elf", Class: "wizard", Level: 1, Background: "acolyte",
		SkillProficiencies: []string{"arcana", "history"}, MainHand: "Dagger",
		PreparedSpells: []string{"fire bolt", "magic missile"},
		Str:            8, Dex: 16, Con: 13, Int: 15, Wis: 12, Cha: 10,
	}
	if !reflect.DeepEqual(all[0].Data(), wantMira) {
		t.Errorf("Mira = %+v\nwant %+v", all[0].Data(), wantMira)
	}
	if conan := all[1].Data(); conan.Level != 3 || conan.Str != 15 || conan.MainHand != "" || conan.KnownSpells != nil {
		t.Errorf("Conan = %+v", conan)
	}
	if err := repository.Update(all[1]); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(file)
	if !strings.HasPrefix(string(content), "Name,Race,Class,Level,") || strings.Contains(string(content), "Notes") {
		t.Errorf("rewritten file:\n%s", content)
	}
}

// A broken file is reported with the file name, row and column, and nothing
// is overwritten.
func TestCSVFileErrors(t *testing.T) {
	header := "Name,Race,Class,Level,Background,STR,DEX,CON,INT,WIS,CHA,Skill Proficiencies,Main Hand,Off Hand,Armor,Shield,Known Spells,Prepared Spells\n"
	tests := []struct{ name, content, want string }{
		{"missing columns", "Name,Race,Class\nMira,elf,wizard\n", "characters.csv: missing columns: Level, Background, STR"},
		{"not a number", header + "Mira,elf,wizard,one,,8,16,13,15,12,10,,,,,,,\n", `characters.csv row 2: column "Level": "one" is not a whole number`},
		{"empty name", header + "Mira,elf,wizard,1,,8,16,13,15,12,10,,,,,,,\n ,elf,wizard,1,,8,16,13,15,12,10,,,,,,,\n", `characters.csv row 3: column "Name" is empty`},
		{"broken quotes", header + "\"Mira,elf\n", "failed to read CSV"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "characters.csv")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			repository := infrastructure.NewCSVCharacterRepository(file)
			if _, err := repository.GetAll(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("GetAll err = %v, want %q", err, tt.want)
			}
			if err := repository.Create(domain.RestoreCharacter(domain.CharacterData{Name: "New"})); err == nil {
				t.Error("Create on a broken file must fail")
			}
			if content, _ := os.ReadFile(file); string(content) != tt.content {
				t.Error("a broken file must not be overwritten")
			}
		})
	}
}

// A name starting with "=" would run as a formula in a spreadsheet; the file
// stores it with a leading apostrophe and reads it back unchanged. A name
// that really starts with an apostrophe is not touched.
func TestCSVFormulaInjectionIsEscaped(t *testing.T) {
	file := filepath.Join(t.TempDir(), "characters.csv")
	repository := infrastructure.NewCSVCharacterRepository(file)
	for _, name := range []string{"=HYPERLINK(\"http://evil\")", "+1", "-1", "@SUM(A1)", "'Quoted", "O'Brien"} {
		if err := repository.Create(domain.RestoreCharacter(domain.CharacterData{Name: name, Race: "elf", Class: "wizard", Level: 1})); err != nil {
			t.Fatal(err)
		}
		if got, err := repository.GetByName(name); err != nil || got.Name() != name {
			t.Errorf("%q read back as %v, %v", name, got, err)
		}
	}
	content, _ := os.ReadFile(file)
	for _, line := range strings.Split(string(content), "\n")[1:5] {
		if !strings.HasPrefix(line, "'") && !strings.HasPrefix(line, `"'`) {
			t.Errorf("formula not escaped: %s", line)
		}
	}
}

// Concurrent creates in one process lose nothing and leave no temporary files.
func TestCSVConcurrentCreates(t *testing.T) {
	dir := t.TempDir()
	repository := infrastructure.NewCSVCharacterRepository(filepath.Join(dir, "characters.csv"))
	var wg sync.WaitGroup
	for index := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("Hero %d", index)
			if err := repository.Create(domain.RestoreCharacter(domain.CharacterData{Name: name, Race: "elf", Class: "wizard", Level: 1})); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if all, err := repository.GetAll(); err != nil || len(all) != 20 {
		t.Errorf("stored %d characters, %v; want 20", len(all), err)
	}
	if files, _ := os.ReadDir(dir); len(files) != 1 {
		t.Errorf("files left in the folder: %d, want only characters.csv", len(files))
	}
}

// Every command and query of the services works the same on CSV storage.
func TestServicesOnCSVStorage(t *testing.T) {
	file := filepath.Join(t.TempDir(), "characters.csv")
	commands, queries := servicesFor(file, nil)
	if _, _, err := commands.Create(wizardRequest("Mira"), ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := commands.Create(wizardRequest("Mira"), ""); !errors.Is(err, application.ErrAlreadyExists) {
		t.Errorf("duplicate: %v", err)
	}
	steps := []func() error{
		func() error { _, err := commands.EquipWeapon("Mira", "dagger", "main hand"); return err },
		func() error { _, err := commands.EquipArmor("Mira", "leather armor"); return err },
		func() error { _, err := commands.AddSpell("Mira", "fire bolt", true); return err },
		func() error { _, err := commands.ChangeLevel("Mira", 3); return err },
		func() error { _, err := commands.Unequip("Mira", "armor"); return err },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	mira, err := queries.Get("Mira")
	if err != nil || mira.Level() != 3 || mira.MainHand() != "Dagger" || mira.Armor() != "" ||
		!reflect.DeepEqual(mira.PreparedSpells(), []string{"fire bolt"}) || mira.Scores().Dex != 16 {
		t.Errorf("Mira = %+v, %v", mira.Data(), err)
	}
	if err := commands.Delete("Mira"); err != nil {
		t.Fatal(err)
	}
	if all, _ := queries.GetAll(); len(all) != 0 {
		t.Errorf("%d characters left", len(all))
	}
}

// The web API works on CSV storage too (create, edit, read, delete).
func TestWebAPIOnCSVStorage(t *testing.T) {
	file := filepath.Join(t.TempDir(), "characters.csv")
	server, err := web.NewServer(servicesFor(file, &fakeLookup{}))
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler()
	edit := strings.Replace(strings.Replace(miraJSON, `"Level":1`, `"Level":2,"MainHand":"Dagger"`, 1), `"Dex":14`, `"Dex":16`, 1)
	for _, step := range []struct {
		method, target, body string
		want                 int
	}{
		{http.MethodPost, "/api/characters", miraJSON, http.StatusCreated},
		{http.MethodPut, "/api/characters/Mira", edit, http.StatusOK},
		{http.MethodGet, "/api/characters/Mira", "", http.StatusOK},
		{http.MethodGet, "/characters/Mira", "", http.StatusOK},
	} {
		if got := request(handler, step.method, step.target, step.body); got.Code != step.want {
			t.Fatalf("%s %s = %d %s", step.method, step.target, got.Code, got.Body)
		}
	}
	content, _ := os.ReadFile(file)
	if !strings.Contains(string(content), "\nMira,elf,wizard,2,acolyte,8,16,13,15,12,10,arcana; history; insight; religion,Dagger,") {
		t.Errorf("file:\n%s", content)
	}
	if got := request(handler, http.MethodDelete, "/api/characters/Mira", ""); got.Code != http.StatusOK {
		t.Errorf("delete = %d", got.Code)
	}
}

// The export use case copies characters as stored, without applying the
// creation rules (racial bonuses) a second time.
func TestExportCharacters(t *testing.T) {
	commands, queries, _ := newService(t)
	for _, name := range []string{"Mira", "Lyra"} {
		if _, _, err := commands.Create(wizardRequest(name), ""); err != nil {
			t.Fatal(err)
		}
	}
	target := infrastructure.NewCSVCharacterRepository(filepath.Join(t.TempDir(), "export.csv"))
	if count, err := application.ExportCharacters(queries, target); err != nil || count != 2 {
		t.Fatalf("ExportCharacters = %d, %v", count, err)
	}
	original, _ := queries.Get("Mira")
	exported, err := target.GetByName("Mira")
	if err != nil || !reflect.DeepEqual(exported.Data(), original.Data()) {
		t.Errorf("exported %+v, %v\nwant %+v", exported.Data(), err, original.Data())
	}

	// A source that fails to read exports nothing.
	broken := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(broken, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if count, err := application.ExportCharacters(infrastructure.NewJSONCharacterRepository(broken), target); err == nil || count != 0 {
		t.Errorf("broken source = %d, %v", count, err)
	}
}

// MPP_STORAGE=csv makes every CLI command use data/characters.csv.
func TestCLIWithCSVStorage(t *testing.T) {
	dir := workspace(t)
	t.Setenv("MPP_STORAGE", "csv")
	t.Setenv("MPP_SRD_API_URL", "http://127.0.0.1:1")
	for _, step := range []struct {
		args []string
		want string
	}{
		{[]string{"create", "-name", "Mira", "-race", "elf", "-class", "wizard", "-int", "15"}, "saved character Mira"},
		{[]string{"equip", "-name", "Mira", "-weapon", "dagger", "-slot", "main hand"}, "Equipped weapon dagger to main hand"},
		{[]string{"prepare-spell", "-name", "Mira", "-spell", "fire bolt"}, "Prepared spell fire bolt"},
		{[]string{"level", "-name", "Mira", "-level", "2"}, "Mira is now level 2"},
		{[]string{"list"}, "Mira - elf wizard (level 2)"},
		{[]string{"view", "-name", "Mira"}, "Main hand: dagger"},
	} {
		out, _ := cli(t, dir, step.args...)
		expectContains(t, out, step.want)
	}
	content, err := os.ReadFile(filepath.Join(dir, "data", "characters.csv"))
	if err != nil || !strings.Contains(string(content), "Mira,elf,wizard,2,acolyte,") {
		t.Errorf("characters.csv = %q, %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "characters.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("characters.json was written: %v", err)
	}
	out, _ := cli(t, dir, "delete", "-name", "Mira")
	expectContains(t, out, "deleted Mira")

	t.Setenv("MPP_STORAGE", "xml")
	out, code := cli(t, dir, "list")
	if code != 2 {
		t.Errorf("unknown storage exit code = %d", code)
	}
	expectContains(t, out, `unknown MPP_STORAGE "xml": use csv or json`)
}

// "export" writes the saved characters to a new CSV (or JSON) file that the
// other storage can open, and never overwrites an existing file.
func TestCLIExport(t *testing.T) {
	dir := workspace(t)
	t.Setenv("MPP_SRD_API_URL", "http://127.0.0.1:1")
	seedCLI(t, dir)
	out, code := cli(t, dir, "export", "-file", "party.csv")
	if code != 0 {
		t.Fatalf("export exit %d: %s", code, out)
	}
	expectContains(t, out, "exported 3 characters to party.csv")

	// The exported file works as storage.
	t.Setenv("MPP_STORAGE", "csv")
	t.Setenv("MPP_CHARACTERS_FILE", filepath.Join(dir, "party.csv"))
	out, _ = cli(t, dir, "list")
	expectContains(t, out, "Mira - elf wizard (level 1)", "Sora - human sorcerer (level 1)", "Conan - human fighter (level 1)")

	// And back from CSV to JSON.
	out, _ = cli(t, dir, "export", "-file", "party.json", "-format", "json")
	expectContains(t, out, "exported 3 characters to party.json")

	for _, tt := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"export", "-file", "party.csv"}, 1, "party.csv already exists"},
		{[]string{"export"}, 2, "file is required"},
		{[]string{"export", "-file", "x.xml", "-format", "xml"}, 2, `unknown format "xml": use csv or json`},
	} {
		out, code := cli(t, dir, tt.args...)
		if code != tt.code {
			t.Errorf("%v exit code = %d, want %d", tt.args, code, tt.code)
		}
		expectContains(t, out, tt.want)
	}
}
