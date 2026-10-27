package tests

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mpp-zmyslek/internal/application"
	"mpp-zmyslek/internal/domain"
	"mpp-zmyslek/internal/infrastructure"
	"mpp-zmyslek/web"
)

// Capacity is 20 + 2 × the Strength modifier, including negative modifiers.
func TestCarryingCapacity(t *testing.T) {
	for strength, want := range map[int]int{0: 10, 1: 10, 8: 18, 9: 18, 10: 20, 11: 20, 16: 26, 30: 40} {
		if got := domain.CarryingCapacity(strength); got != want {
			t.Errorf("CarryingCapacity(%d) = %d, want %d", strength, got, want)
		}
	}
}

// Strength 8 gives a capacity of 18 lb, so the limits are 18 and 36 lb.
// Exactly the capacity is not "more than" it, so it gives the lighter condition.
func TestEncumbranceBoundaries(t *testing.T) {
	tests := []struct {
		carried float64
		want    domain.Condition
	}{
		{0, domain.NotEncumbered},
		{17.75, domain.NotEncumbered},
		{18, domain.NotEncumbered},
		{18.25, domain.Encumbered},
		{36, domain.Encumbered},
		{36.25, domain.HeavilyEncumbered},
		{500, domain.HeavilyEncumbered},
	}
	for _, tt := range tests {
		got := domain.EncumbranceFor(8, tt.carried)
		if got.Condition != tt.want || got.Capacity != 18 || got.CarriedWeight != tt.carried {
			t.Errorf("EncumbranceFor(8, %g) = %+v, want %q with capacity 18", tt.carried, got, tt.want)
		}
	}
}

// The domain adds up the weights it is given and uses the character's Strength.
func TestCharacterEncumbranceAddsWeights(t *testing.T) {
	character, err := domain.NewCharacter("Pip", "halfling", "rogue", "acolyte", 1, domain.AbilityScores{Str: 8, Dex: 15})
	if err != nil {
		t.Fatal(err)
	}
	dagger := domain.NewEquipment("Dagger", "Weapon").WithWeight(1)
	for _, slot := range []string{domain.MainHandSlot, domain.OffHandSlot} {
		if err := character.EquipWeapon(dagger, slot); err != nil {
			t.Fatal(err)
		}
	}
	// A dagger in each hand is two daggers.
	if got := character.EquippedItems(); !reflect.DeepEqual(got, []string{"Dagger", "Dagger"}) {
		t.Errorf("EquippedItems = %v", got)
	}
	got := character.Encumbrance([]*domain.Equipment{dagger, dagger})
	if got != (domain.Encumbrance{CarriedWeight: 2, Capacity: 18, Condition: domain.NotEncumbered}) {
		t.Errorf("Encumbrance = %+v", got)
	}
	if got := character.Encumbrance(nil); got.CarriedWeight != 0 || got.Condition != domain.NotEncumbered {
		t.Errorf("nothing carried = %+v", got)
	}
}

func TestWithWeightReturnsACopy(t *testing.T) {
	plain := domain.NewEquipment("Shield", "Armor")
	weighed := plain.WithWeight(6)
	if plain.Weight() != 0 || weighed.Weight() != 6 || weighed.Name() != "Shield" || !weighed.IsShield() {
		t.Errorf("plain weighs %g, weighed weighs %g", plain.Weight(), weighed.Weight())
	}
}

// The weights come from the SRD (third column of the equipment CSV).
func TestEquipmentWeightsFromCSV(t *testing.T) {
	repository := infrastructure.NewCSVEquipmentRepository("../data/5e-SRD-Equipment.csv")
	for name, want := range map[string]float64{
		"shield": 6, "Studded Leather Armor": 13, "crossbow, heavy": 18, "dart": 0.25,
		"half plate armor": 40,
		"Explorer's Pack":  0, // the SRD lists no weight for packs
	} {
		item, err := repository.GetByName(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if item.Weight() != want {
			t.Errorf("%s weighs %g, want %g", name, item.Weight(), want)
		}
	}
}

// A hand-edited equipment file with a bad weight is reported with its row
// instead of silently counting as 0 lb.
func TestEquipmentWeightColumnIsValidated(t *testing.T) {
	tests := []struct {
		name, rows string
		want       float64
		wantErr    string
	}{
		{"old two-column file", "name,type\nClub,Weapon\n", 0, ""},
		{"empty weight", "name,type,weight\nClub,Weapon,\n", 0, ""},
		{"fraction with spaces", "name,type,weight\nClub,Weapon, 2.5 \n", 2.5, ""},
		{"text", "name,type,weight\nClub,Weapon,heavy\n", 0, `row 2: "heavy" is not a number`},
		{"unit", "name,type,weight\nClub,Weapon,2 lb\n", 0, "is not a number"},
		{"not a number", "name,type,weight\nClub,Weapon,NaN\n", 0, "is not a number"},
		{"infinite", "name,type,weight\nClub,Weapon,Inf\n", 0, "is not a number"},
		{"negative", "name,type,weight\nClub,Weapon,-2\n", 0, `"-2" is negative`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "equipment.csv")
			if err := os.WriteFile(file, []byte(tt.rows), 0o644); err != nil {
				t.Fatal(err)
			}
			item, err := infrastructure.NewCSVEquipmentRepository(file).GetByName("club")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || item.Weight() != tt.want {
				t.Errorf("GetByName = %v, %v; want weight %g", item, err, tt.want)
			}
		})
	}
}

// halfling creates the exam's level 1 halfling rogue with Strength 8 and
// Dexterity 15 and equips the given items in order.
func halfling(t *testing.T, commands *application.CharacterService, name string, equip func(name string) error) {
	t.Helper()
	request := application.CreateRequest{
		Name: name, Race: "halfling", Class: "rogue", Background: "acolyte", Level: 1,
		Scores: domain.AbilityScores{Str: 8, Dex: 15, Con: 14, Int: 12, Wis: 13, Cha: 10},
	}
	if _, _, err := commands.Create(request, ""); err != nil {
		t.Fatal(err)
	}
	if err := equip(name); err != nil {
		t.Fatal(err)
	}
}

// examEquipment are the four test cases of the exam, from no items to the
// shield, studded leather armor and a heavy crossbow in the main hand.
func examEquipment(commands *application.CharacterService) []struct {
	name  string
	equip func(name string) error
	want  domain.Encumbrance
} {
	shield := func(name string) error { _, err := commands.EquipShield(name, "shield"); return err }
	armor := func(name string) error { _, err := commands.EquipArmor(name, "studded leather armor"); return err }
	crossbow := func(name string) error {
		_, err := commands.EquipWeapon(name, "crossbow, heavy", domain.MainHandSlot)
		return err
	}
	all := func(steps ...func(string) error) func(string) error {
		return func(name string) error {
			for _, step := range steps {
				if err := step(name); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return []struct {
		name  string
		equip func(name string) error
		want  domain.Encumbrance
	}{
		{"nothing", all(), domain.Encumbrance{CarriedWeight: 0, Capacity: 18, Condition: domain.NotEncumbered}},
		{"shield", all(shield), domain.Encumbrance{CarriedWeight: 6, Capacity: 18, Condition: domain.NotEncumbered}},
		{"shield and armor", all(shield, armor), domain.Encumbrance{CarriedWeight: 19, Capacity: 18, Condition: domain.Encumbered}},
		{"shield, armor and crossbow", all(shield, crossbow, armor), domain.Encumbrance{CarriedWeight: 37, Capacity: 18, Condition: domain.HeavilyEncumbered}},
	}
}

// The exam's examples, through the real services, CSV data and JSON storage.
func TestEncumbranceExamExamples(t *testing.T) {
	commands, queries, _ := newService(t)
	for _, tt := range examEquipment(commands) {
		t.Run(tt.name, func(t *testing.T) {
			halfling(t, commands, tt.name, tt.equip)
			character, err := queries.Get(tt.name)
			if err != nil {
				t.Fatal(err)
			}
			if character.Scores().Str != 8 {
				t.Fatalf("Str = %d; a halfling gets no Strength bonus", character.Scores().Str)
			}
			got, err := queries.Encumbrance(character)
			if err != nil || got != tt.want {
				t.Errorf("Encumbrance = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

// An item that is no longer in the equipment list cannot be weighed; that is
// an error, not 0 lb.
func TestEncumbranceOfAnUnknownItem(t *testing.T) {
	_, queries, _ := newService(t)
	character := domain.RestoreCharacter(domain.CharacterData{Name: "Old", Race: "human", Class: "fighter", Level: 1, MainHand: "Banana", Str: 10})
	if _, err := queries.Encumbrance(character); err == nil || !strings.Contains(err.Error(), "Banana") {
		t.Errorf("err = %v, want the unknown item named", err)
	}
}

// sheetJSON returns the character data the sheet page hands to its script.
func sheetJSON(t *testing.T, handler http.Handler, path string) (int, map[string]any) {
	t.Helper()
	got := request(handler, http.MethodGet, path, "")
	page := got.Body.String()
	start := strings.Index(page, `<script id="character-data" type="application/json">`)
	if start < 0 {
		return got.Code, nil
	}
	page = page[start:]
	page = page[strings.Index(page, ">")+1 : strings.Index(page, "</script>")]
	var data map[string]any
	if err := json.Unmarshal([]byte(page), &data); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return got.Code, data
}

// The sheet page (view and edit) shows the condition in Features & Traits.
func TestSheetShowsEncumbrance(t *testing.T) {
	file := filepath.Join(t.TempDir(), "characters.json")
	commands, queries := servicesFor(file, &fakeLookup{})
	server, err := web.NewServer(commands, queries)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler()
	cases := examEquipment(commands)
	for _, tt := range cases {
		halfling(t, commands, tt.name, tt.equip)
	}
	for _, tt := range cases {
		for _, path := range []string{"/characters/" + strings.ReplaceAll(tt.name, " ", "%20"), "/characters/" + strings.ReplaceAll(tt.name, " ", "%20") + "/edit"} {
			code, data := sheetJSON(t, handler, path)
			encumbrance, _ := data["Encumbrance"].(map[string]any)
			if code != http.StatusOK || encumbrance["Condition"] != string(tt.want.Condition) ||
				encumbrance["CarriedWeight"] != tt.want.CarriedWeight || encumbrance["Capacity"] != float64(tt.want.Capacity) {
				t.Errorf("%s: %d %v, want %+v", path, code, encumbrance, tt.want)
			}
		}
	}
	// The script writes the condition into the Features & Traits box.
	page := request(handler, http.MethodGet, "/characters/nothing", "").Body.String()
	if !strings.Contains(page, "encumbranceSummary(character.Encumbrance)") || !strings.Contains(page, `id="features"`) {
		t.Error("the sheet does not put the encumbrance in Features & Traits")
	}
	// Nothing is carried on the creation form, so no condition is shown there.
	if _, data := sheetJSON(t, handler, "/characters/new"); data["Encumbrance"].(map[string]any)["Condition"] != "" {
		t.Errorf("creation form condition = %v", data["Encumbrance"])
	}

	// A stored item that is no longer in the equipment list is a server error.
	storeCharacter(t, file, domain.CharacterData{Name: "Old", Race: "human", Class: "fighter", Level: 1, MainHand: "Banana", Str: 10})
	if got := request(handler, http.MethodGet, "/characters/Old", ""); got.Code != http.StatusInternalServerError {
		t.Errorf("unknown item = %d", got.Code)
	}
}

// "view -full" prints the condition; the plain view stays exactly as before.
func TestCLIViewShowsEncumbrance(t *testing.T) {
	dir := workspace(t)
	t.Setenv("MPP_SRD_API_URL", "http://127.0.0.1:1")
	cli(t, dir, "create", "-name", "Pip", "-race", "halfling", "-class", "rogue", "-str", "8", "-dex", "15")
	steps := []struct {
		args []string
		want string
	}{
		{nil, "Condition: none (0 lb carried, capacity 18 lb)"},
		{[]string{"equip", "-name", "Pip", "-shield", "shield"}, "Condition: none (6 lb carried, capacity 18 lb)"},
		{[]string{"equip", "-name", "Pip", "-armor", "studded leather armor"}, "Condition: encumbered (19 lb carried, capacity 18 lb)"},
		{[]string{"equip", "-name", "Pip", "-weapon", "crossbow, heavy", "-slot", "main hand"}, "Condition: heavily encumbered (37 lb carried, capacity 18 lb)"},
		{[]string{"unequip", "-name", "Pip", "-slot", "armor"}, "Condition: encumbered (24 lb carried, capacity 18 lb)"},
	}
	for _, step := range steps {
		if step.args != nil {
			cli(t, dir, step.args...)
		}
		out, _ := cli(t, dir, "view", "-name", "Pip", "-full")
		expectContains(t, out, step.want)
	}
	if out, _ := cli(t, dir, "view", "-name", "Pip"); strings.Contains(out, "Condition") {
		t.Errorf("the plain view must not change:\n%s", out)
	}
}
