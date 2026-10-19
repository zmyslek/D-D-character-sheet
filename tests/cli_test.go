package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The command-line program and the metrics tool are "package main", which Go
// cannot import, so these tests build them once and run them as real programs.
var cliBinary, metricsBinary string

// coverDir collects coverage data from the built programs when set.
var coverDir = os.Getenv("MPP_CLI_COVERDIR")

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mpp-bin")
	if err != nil {
		panic(err)
	}
	cliBinary = filepath.Join(dir, "mpp")
	metricsBinary = filepath.Join(dir, "metrics")
	for target, output := range map[string]string{".": cliBinary, "./tools/metrics": metricsBinary} {
		args := []string{"build", "-o", output}
		// With MPP_CLI_COVERDIR set, the programs are built with coverage
		// instrumentation and write their coverage data to that directory
		// (see tests/TESTING.md), so main.go is measured too.
		if coverDir != "" {
			args = append(args, "-cover", "-coverpkg=./...")
		}
		build := exec.Command("go", append(args, target)...)
		build.Dir = ".." // project root
		if out, err := build.CombinedOutput(); err != nil {
			panic("build failed: " + string(out))
		}
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// workspace creates a temp dir with the project's CSV files linked into its
// own data folder, because the CLI reads "data/*.csv" and writes
// "data/characters.json" relative to its working directory. Only the CSVs are
// linked so the tests never touch the real characters.json.
func workspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	csvs, err := filepath.Glob(filepath.Join("..", "data", "*.csv"))
	if err != nil || len(csvs) == 0 {
		t.Fatalf("no CSV files found in ../data: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, csv := range csvs {
		source, err := filepath.Abs(csv)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(source, filepath.Join(dir, "data", filepath.Base(csv))); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// run starts a binary in dir and returns its combined output and exit code.
func run(t *testing.T, binary, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	if coverDir != "" {
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func cli(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	return run(t, cliBinary, dir, args...)
}

func expectContains(t *testing.T, output string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(output, fragment) {
			t.Errorf("output does not contain %q:\n%s", fragment, output)
		}
	}
}

// seedCLI stores a wizard (prepared caster), a sorcerer (learner) and a fighter.
func seedCLI(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"create", "-name", "Mira", "-race", "elf", "-class", "wizard"},
		{"create", "-name", "Sora", "-race", "human", "-class", "sorcerer"},
		{"create", "-name", "Conan", "-race", "human", "-class", "fighter"},
	} {
		if out, code := cli(t, dir, args...); code != 0 || !strings.Contains(out, "saved character") {
			t.Fatalf("seeding %v failed (exit %d): %s", args, code, out)
		}
	}
}

func TestCLIUnknownCommandAndNoArguments(t *testing.T) {
	out, code := cli(t, t.TempDir(), "dance")
	if code != 2 {
		t.Errorf("unknown command exit code = %d, want 2", code)
	}
	expectContains(t, out, "Usage:", "learn-spell", "prepare-spell")

	out, code = cli(t, t.TempDir())
	if code != 1 {
		t.Errorf("no arguments exit code = %d, want 1", code)
	}
	expectContains(t, out, "Usage:")
}

func TestCLICreateViewListDelete(t *testing.T) {
	dir := workspace(t)

	out, code := cli(t, dir, "create", "-name", "Aria Stone", "-race", "half-elf", "-class", "bard", "-skills", "stealth",
		"-str", "15", "-dex", "14", "-con", "13", "-int", "12", "-wis", "10", "-cha", "8")
	if code != 0 {
		t.Fatalf("create failed with %d: %s", code, out)
	}
	expectContains(t, out, "saved character Aria Stone")

	out, _ = cli(t, dir, "list")
	expectContains(t, out, "Aria Stone - half-elf bard (level 1)")

	out, _ = cli(t, dir, "view", "-name", "Aria Stone")
	// half-elf: +2 cha, +1 con, +1 dex on the default array (cha 8, con 13, dex 14).
	expectContains(t, out, "CHA: 10 (+0)", "CON: 14 (+2)", "DEX: 15 (+2)", "Skill proficiencies: stealth",
		"insight", "religion", // acolyte background skills are added automatically
		"Spell slots:", "Level 0: 2", "Level 1: 2", "Spellcasting ability: charisma")

	out, _ = cli(t, dir, "delete", "-name", "Aria Stone")
	expectContains(t, out, "deleted Aria Stone")
	out, _ = cli(t, dir, "list")
	if strings.TrimSpace(out) != "" {
		t.Errorf("list should be empty after delete, got %q", out)
	}
}

func TestCLIValidation(t *testing.T) {
	dir := workspace(t)
	tests := []struct {
		name     string
		args     []string
		wantOut  string
		wantCode int
	}{
		{"create: missing name", []string{"create", "-race", "elf", "-class", "wizard"}, "name is required", 2},
		{"create: missing race", []string{"create", "-name", "A", "-class", "wizard"}, "race and class are required", 0},
		// The CLI accepts any scores (manual entry), but never outside the SRD range 0-30.
		{"create: score out of range", []string{"create", "-name", "A", "-race", "elf", "-class", "wizard", "-str", "31"}, "ability scores must be between 0 and 30", 0},
		{"create: level out of range", []string{"create", "-name", "A", "-race", "elf", "-class", "wizard", "-level", "21"}, "level must be between 1 and 20", 0},
		{"create: unknown race", []string{"create", "-name", "A", "-race", "martian", "-class", "wizard"}, "not found", 0},
		{"create: unknown class", []string{"create", "-name", "A", "-race", "elf", "-class", "jester"}, "not found", 0},
		{"view: missing name", []string{"view"}, "name is required", 0},
		{"view: unknown", []string{"view", "-name", "Nobody"}, "not found", 0},
		{"delete: missing name", []string{"delete"}, "name is required", 0},
		{"delete: unknown", []string{"delete", "-name", "Nobody"}, "character not found", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := cli(t, dir, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			expectContains(t, out, tt.wantOut)
		})
	}
}

func TestCLIEquip(t *testing.T) {
	dir := workspace(t)
	seedCLI(t, dir)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing name", []string{"equip", "-weapon", "club"}, "name is required"},
		{"nothing selected", []string{"equip", "-name", "Conan"}, "exactly one of"},
		{"two selected", []string{"equip", "-name", "Conan", "-weapon", "club", "-armor", "chain mail"}, "exactly one of"},
		{"unknown character", []string{"equip", "-name", "Nobody", "-weapon", "club", "-slot", "main hand"}, "not found"},
		{"unknown weapon", []string{"equip", "-name", "Conan", "-weapon", "banana", "-slot", "main hand"}, "not found"},
		{"not a weapon", []string{"equip", "-name", "Conan", "-weapon", "chain mail", "-slot", "main hand"}, "is not a weapon"},
		{"bad slot", []string{"equip", "-name", "Conan", "-weapon", "club", "-slot", "head"}, "slot must be"},
		{"weapon ok", []string{"equip", "-name", "Conan", "-weapon", "Longsword", "-slot", "main hand"}, "Equipped weapon longsword to main hand"},
		{"slot taken", []string{"equip", "-name", "Conan", "-weapon", "club", "-slot", "main hand"}, "already occupied"},
		{"armor ok", []string{"equip", "-name", "Conan", "-armor", "chain mail"}, "Equipped armor chain mail"},
		{"weapon as armor", []string{"equip", "-name", "Conan", "-armor", "club"}, "is not armor"},
		{"shield as armor", []string{"equip", "-name", "Conan", "-armor", "shield"}, "is not armor"},
		{"unknown armor", []string{"equip", "-name", "Conan", "-armor", "banana"}, "not found"},
		{"shield ok", []string{"equip", "-name", "Conan", "-shield", "shield"}, "Equipped shield shield"},
		{"armor as shield", []string{"equip", "-name", "Conan", "-shield", "chain mail"}, "is not a shield"},
		{"unknown shield", []string{"equip", "-name", "Conan", "-shield", "banana"}, "not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _ := cli(t, dir, tt.args...)
			expectContains(t, out, tt.want)
		})
	}

	out, _ := cli(t, dir, "view", "-name", "Conan")
	// Chain mail (16) + shield (+2) = 18.
	expectContains(t, out, "Main hand: longsword", "Armor: chain mail", "Shield: shield", "Armor class: 18", "Class: fighter")
	if strings.Contains(out, "Spell") {
		t.Errorf("a fighter must not print spell information:\n%s", out)
	}
}

func TestCLISpells(t *testing.T) {
	dir := workspace(t)
	seedCLI(t, dir)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing arguments", []string{"prepare-spell", "-spell", "fire bolt"}, "name and spell are required"},
		{"unknown character", []string{"learn-spell", "-name", "Nobody", "-spell", "fire bolt"}, "not found"},
		{"non caster", []string{"learn-spell", "-name", "Conan", "-spell", "fire bolt"}, "can't cast spells"},
		{"prepared caster cannot learn", []string{"learn-spell", "-name", "Mira", "-spell", "fire bolt"}, "prepares spells and can't learn"},
		{"learner cannot prepare", []string{"prepare-spell", "-name", "Sora", "-spell", "fire bolt"}, "learns spells and can't prepare"},
		{"unknown spell", []string{"prepare-spell", "-name", "Mira", "-spell", "wish upon a star"}, "not found"},
		{"wrong class list", []string{"prepare-spell", "-name", "Mira", "-spell", "cure wounds"}, "not available to this class"},
		{"too high level", []string{"prepare-spell", "-name", "Mira", "-spell", "fireball"}, "higher level than the available spell slots"},
		{"prepare ok", []string{"prepare-spell", "-name", "Mira", "-spell", "Magic Missile"}, "Prepared spell magic missile"},
		{"cantrip ok", []string{"prepare-spell", "-name", "Mira", "-spell", "Fire Bolt"}, "Prepared spell fire bolt"},
		{"learn ok", []string{"learn-spell", "-name", "Sora", "-spell", "Fire Bolt"}, "Learned spell fire bolt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _ := cli(t, dir, tt.args...)
			expectContains(t, out, tt.want)
		})
	}
}

func TestCLIFlagParseErrorsReturnQuietly(t *testing.T) {
	dir := workspace(t)
	for _, command := range []string{"create", "view", "delete", "equip", "learn-spell", "prepare-spell", "serve"} {
		if _, code := cli(t, dir, command, "-unknown-flag"); code != 0 {
			t.Errorf("%s with a bad flag exited with %d, want 0", command, code)
		}
	}
}

func TestCLIServeStartupFailure(t *testing.T) {
	out, code := cli(t, workspace(t), "serve", "-addr", "not-a-valid-address")
	if code != 1 {
		t.Errorf("exit code = %d, want 1; output: %s", code, out)
	}
}
