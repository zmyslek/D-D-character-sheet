package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"mpp-zmyslek/web"
)

// newWebHandler builds the web server on a temporary characters file.
func newWebHandler(t *testing.T) http.Handler {
	t.Helper()
	commands, queries := servicesFor(t.TempDir()+"/characters.json", &fakeLookup{})
	server, err := web.NewServer(commands, queries)
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler()
}

func request(handler http.Handler, method, target, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

const miraJSON = `{"Name":"Mira","Race":"elf","Class":"wizard","Level":1,"Background":"acolyte",
	"Str":8,"Dex":14,"Con":13,"Int":15,"Wis":12,"Cha":10}`

func TestCharactersAPI(t *testing.T) {
	handler := newWebHandler(t)

	created := request(handler, http.MethodPost, "/api/characters", miraJSON)
	if created.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", created.Code, created.Body.String())
	}
	var body struct {
		Name string
		Dex  int
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil || body.Name != "Mira" || body.Dex != 16 {
		t.Errorf("created body = %s", created.Body.String())
	}

	tests := []struct {
		name, method, target, body string
		want                       int
	}{
		{"list", http.MethodGet, "/api/characters", "", http.StatusOK},
		{"duplicate name", http.MethodPost, "/api/characters", miraJSON, http.StatusConflict},
		{"invalid JSON", http.MethodPost, "/api/characters", "{", http.StatusBadRequest},
		{"not the standard array", http.MethodPost, "/api/characters", strings.Replace(strings.Replace(miraJSON, `"Str":8`, `"Str":18`, 1), "Mira", "Strong", 1), http.StatusBadRequest},
		{"unknown class", http.MethodPost, "/api/characters", strings.Replace(strings.Replace(miraJSON, "wizard", "jester", 1), "Mira", "Jest", 1), http.StatusBadRequest},
		{"wrong method", http.MethodPatch, "/api/characters", "", http.StatusMethodNotAllowed},
		{"empty spell name", http.MethodGet, "/api/spells/", "", http.StatusBadRequest},
	}
	for _, tt := range tests {
		if got := request(handler, tt.method, tt.target, tt.body); got.Code != tt.want {
			t.Errorf("%s: status = %d, want %d (%s)", tt.name, got.Code, tt.want, got.Body.String())
		}
	}
}

func TestCreateWithIdempotencyKey(t *testing.T) {
	handler := newWebHandler(t)
	first := request(handler, http.MethodPost, "/api/characters", miraJSON, "Idempotency-Key", "k1")
	retry := request(handler, http.MethodPost, "/api/characters", miraJSON, "Idempotency-Key", "k1")
	if first.Code != http.StatusCreated || retry.Code != http.StatusCreated || retry.Header().Get("Idempotent-Replayed") != "true" {
		t.Errorf("first %d, retry %d %v", first.Code, retry.Code, retry.Header())
	}
	other := strings.Replace(miraJSON, "Mira", "Other", 1)
	if got := request(handler, http.MethodPost, "/api/characters", other, "Idempotency-Key", "k1"); got.Code != http.StatusConflict {
		t.Errorf("reused key with a different body = %d", got.Code)
	}

	// Simultaneous retries of one request create one character.
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.Replace(miraJSON, "Mira", "Storm", 1)
			if got := request(handler, http.MethodPost, "/api/characters", body, "Idempotency-Key", "k2"); got.Code != http.StatusCreated {
				t.Errorf("concurrent retry = %d", got.Code)
			}
		}()
	}
	wg.Wait()
	var list []struct{ Name string }
	_ = json.Unmarshal(request(handler, http.MethodGet, "/api/characters", "").Body.Bytes(), &list)
	if len(list) != 2 {
		t.Errorf("stored %d characters, want 2", len(list))
	}
}

func TestCharacterAPIPutAndDelete(t *testing.T) {
	handler := newWebHandler(t)
	request(handler, http.MethodPost, "/api/characters", miraJSON)
	edit := strings.Replace(miraJSON, `"Level":1`, `"Level":3,"MainHand":"Dagger","PreparedSpells":["fire bolt"]`, 1)
	edit = strings.Replace(edit, `"Dex":14`, `"Dex":16`, 1) // the stored score includes the elf bonus

	tests := []struct {
		name, method, target, body string
		want                       int
	}{
		{"valid edit", http.MethodPut, "/api/characters/Mira", edit, http.StatusOK},
		{"name mismatch", http.MethodPut, "/api/characters/Mira", strings.Replace(edit, `"Mira"`, `"Other"`, 1), http.StatusBadRequest},
		{"race change", http.MethodPut, "/api/characters/Mira", strings.Replace(edit, "elf", "dwarf", 1), http.StatusBadRequest},
		{"level decrease", http.MethodPut, "/api/characters/Mira", strings.Replace(edit, `"Level":3`, `"Level":2`, 1), http.StatusBadRequest},
		{"unknown item", http.MethodPut, "/api/characters/Mira", strings.Replace(edit, "Dagger", "Banana", 1), http.StatusBadRequest},
		{"unknown character", http.MethodPut, "/api/characters/Nobody", strings.Replace(edit, "Mira", "Nobody", 1), http.StatusNotFound},
		{"invalid JSON", http.MethodPut, "/api/characters/Mira", "{", http.StatusBadRequest},
		{"read one", http.MethodGet, "/api/characters/Mira", "", http.StatusOK},
		{"read unknown", http.MethodGet, "/api/characters/Nobody", "", http.StatusNotFound},
		{"delete", http.MethodDelete, "/api/characters/Mira", "", http.StatusOK},
		{"delete again", http.MethodDelete, "/api/characters/Mira", "", http.StatusNotFound},
		{"wrong method", http.MethodPost, "/api/characters/Mira", "", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		if got := request(handler, tt.method, tt.target, tt.body); got.Code != tt.want {
			t.Errorf("%s: status = %d, want %d (%s)", tt.name, got.Code, tt.want, got.Body.String())
		}
	}
}

func TestPages(t *testing.T) {
	handler := newWebHandler(t)
	form := url.Values{
		"name": {"Form Hero"}, "race": {"dwarf"}, "class": {"cleric"}, "background": {"acolyte"},
		"level": {"1"}, "str": {"15"}, "dex": {"14"}, "con": {"13"}, "int": {"12"}, "wis": {"10"}, "cha": {"8"},
	}
	created := request(handler, http.MethodPost, "/characters/new", form.Encode(), "Content-Type", "application/x-www-form-urlencoded")
	if created.Code != http.StatusSeeOther || created.Header().Get("Location") != "/characters/Form%20Hero" {
		t.Fatalf("form create = %d %v", created.Code, created.Header())
	}
	pages := map[string]string{
		"/": "Form Hero", "/characters/Form%20Hero": "Form Hero",
		"/characters/Form%20Hero/edit": "Save changes", "/characters/new": "Create Character",
	}
	for path, fragment := range pages {
		got := request(handler, http.MethodGet, path, "")
		if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), fragment) {
			t.Errorf("GET %s = %d, missing %q", path, got.Code, fragment)
		}
	}
	for path, want := range map[string]int{"/characters/Nobody": 404, "/nowhere": 404, "/style.css": 200, "/static/normalize.css": 200} {
		if got := request(handler, http.MethodGet, path, ""); got.Code != want {
			t.Errorf("GET %s = %d, want %d", path, got.Code, want)
		}
	}
	form.Set("level", "one")
	if got := request(handler, http.MethodPost, "/characters/new", form.Encode(), "Content-Type", "application/x-www-form-urlencoded"); got.Code != http.StatusBadRequest {
		t.Errorf("bad form = %d", got.Code)
	}
	if got := request(handler, http.MethodPost, "/characters/Form%20Hero/delete", ""); got.Code != http.StatusSeeOther {
		t.Errorf("delete form = %d", got.Code)
	}
	if got := request(handler, http.MethodPost, "/characters/Form%20Hero/delete", ""); got.Code != http.StatusNotFound {
		t.Errorf("second delete = %d", got.Code)
	}
}

func TestHealthAndInstanceHeader(t *testing.T) {
	handler := newWebHandler(t)
	got := request(handler, http.MethodGet, "/healthz", "")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"status":"ok"`) {
		t.Errorf("healthz = %d %s", got.Code, got.Body.String())
	}
	// The load balancer demo uses this header to show which instance answered.
	if got.Header().Get("X-Instance") == "" {
		t.Error("X-Instance header missing")
	}
}

func TestEnrichmentEndpoints(t *testing.T) {
	handler := newWebHandler(t)
	withGear := strings.Replace(miraJSON, `"Level":1`, `"Level":1`, 1)
	request(handler, http.MethodPost, "/api/characters", withGear)
	request(handler, http.MethodPut, "/api/characters/Mira",
		strings.Replace(strings.Replace(miraJSON, `"Dex":14`, `"Dex":16`, 1), `"Level":1`, `"Level":1,"MainHand":"Dagger","PreparedSpells":["fire bolt"]`, 1))

	got := request(handler, http.MethodGet, "/api/characters/Mira/enrichment", "")
	var results []struct{ Resource, Name, Error string }
	if err := json.Unmarshal(got.Body.Bytes(), &results); err != nil || got.Code != http.StatusOK {
		t.Fatalf("enrichment = %d %s", got.Code, got.Body.String())
	}
	if len(results) != 2 || results[0].Name != "Dagger" || results[1].Name != "fire bolt" || results[0].Error != "" {
		t.Errorf("results = %+v", results)
	}
	if got := request(handler, http.MethodGet, "/api/characters/Nobody/enrichment", ""); got.Code != http.StatusNotFound {
		t.Errorf("unknown character = %d", got.Code)
	}
	if got := request(handler, http.MethodGet, "/api/spells/fire%20bolt", ""); got.Code != http.StatusOK {
		t.Errorf("spell lookup = %d %s", got.Code, got.Body.String())
	}
	if got := request(handler, http.MethodGet, "/api/equipment/broken", ""); got.Code != http.StatusBadGateway {
		t.Errorf("failing upstream = %d", got.Code)
	}
}

// The web create path follows the same SRD rules as the CLI commands: a
// fighter can't start with spells, and skill proficiencies come from the class
// and background, not from whatever the request lists.
func TestCreateFollowsSRDRules(t *testing.T) {
	handler := newWebHandler(t)
	fighter := strings.Replace(strings.Replace(miraJSON, "wizard", "fighter", 1), "Mira", "Conan", 1)

	withSpells := strings.Replace(fighter, `"Level":1`, `"Level":1,"KnownSpells":["fire bolt"]`, 1)
	if got := request(handler, http.MethodPost, "/api/characters", withSpells); got.Code != http.StatusBadRequest ||
		!strings.Contains(got.Body.String(), "can't cast spells") {
		t.Errorf("fighter with spells = %d %s", got.Code, got.Body.String())
	}

	withSkills := strings.Replace(fighter, `"Level":1`, `"Level":1,"SkillProficiencies":["arcana","stealth","sleight of hand"]`, 1)
	created := request(handler, http.MethodPost, "/api/characters", withSkills)
	var body struct{ SkillProficiencies []string }
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil || created.Code != http.StatusCreated {
		t.Fatalf("fighter = %d %s", created.Code, created.Body.String())
	}
	want := []string{"acrobatics", "animal handling", "insight", "religion"}
	if strings.Join(body.SkillProficiencies, ",") != strings.Join(want, ",") {
		t.Errorf("skills = %v, want the first two fighter skills + acolyte skills %v", body.SkillProficiencies, want)
	}

	wizard := strings.Replace(miraJSON, `"Level":1`, `"Level":1,"PreparedSpells":["Fire Bolt"]`, 1)
	if got := request(handler, http.MethodPost, "/api/characters", wizard); got.Code != http.StatusCreated ||
		!strings.Contains(got.Body.String(), `"PreparedSpells":["fire bolt"]`) {
		t.Errorf("wizard with a cantrip = %d %s", got.Code, got.Body.String())
	}
}

// The sheet gets its spellcasting from the domain: nothing for a fighter, and
// the class table the script uses has no slots for fighters at any level.
func TestSheetSpellcastingComesFromTheDomain(t *testing.T) {
	handler := newWebHandler(t)
	request(handler, http.MethodPost, "/api/characters", strings.Replace(strings.Replace(miraJSON, "wizard", "fighter", 1), "Mira", "Conan", 1))
	request(handler, http.MethodPost, "/api/characters", miraJSON)

	sheetData := func(path string) map[string]any {
		page := request(handler, http.MethodGet, path, "").Body.String()
		start := strings.Index(page, `<script id="character-data" type="application/json">`)
		if start < 0 {
			t.Fatalf("%s has no character data", path)
		}
		page = page[start:]
		page = page[strings.Index(page, ">")+1 : strings.Index(page, "</script>")]
		var data map[string]any
		if err := json.Unmarshal([]byte(page), &data); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return data
	}

	conan := sheetData("/characters/Conan")
	if conan["CanCastSpells"] != false || conan["SpellAbility"] != "" || conan["SpellSaveDC"] != 0.0 || conan["SpellAttack"] != 0.0 {
		t.Errorf("fighter sheet spellcasting = %v %v %v %v", conan["CanCastSpells"], conan["SpellAbility"], conan["SpellSaveDC"], conan["SpellAttack"])
	}
	table := conan["SpellcastingByClass"].(map[string]any)
	for level, rules := range table["fighter"].([]any) {
		if slots := rules.(map[string]any)["Slots"]; slots != nil {
			t.Errorf("fighter level %d has slots %v", level, slots)
		}
	}
	mira := sheetData("/characters/Mira")
	if mira["CanCastSpells"] != true || mira["SpellAbility"] != "intelligence" {
		t.Errorf("wizard sheet = %v %v", mira["CanCastSpells"], mira["SpellAbility"])
	}
}

// A saved sheet is read-only: only the edit page and the creation page have a
// save button. A save button on the read-only sheet would post the character
// again as a new one, with scores that already include racial bonuses.
func TestSheetSaveButtonOnlyWhenCreatingOrEditing(t *testing.T) {
	handler := newWebHandler(t)
	if got := request(handler, http.MethodPost, "/api/characters", miraJSON); got.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", got.Code, got.Body)
	}
	for target, want := range map[string]bool{"/characters/new": true, "/characters/Mira": false, "/characters/Mira/edit": true} {
		page := request(handler, http.MethodGet, target, "")
		if got := strings.Contains(page.Body.String(), `<button type="button" id="save-character">`); page.Code != http.StatusOK || got != want {
			t.Errorf("%s: status %d, save button %v, want %v", target, page.Code, got, want)
		}
	}
}
