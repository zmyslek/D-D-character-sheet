package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	characterdomain "mpp-zmyslek/character/character_domain"
)

// assets contains the templates and styles shipped inside the server binary.
//
//go:embed templates/*.html static/*.css
var assets embed.FS

// Server coordinates HTTP routes, template rendering, persistence, and API calls.
type Server struct {
	repository characterdomain.CharacterRepository
	templates  *template.Template
	client     *http.Client
	sheet      *template.Template
	rateMu     sync.Mutex
	lastAPIHit time.Time
}

// characterView combines a character with values calculated for its web sheet.
type characterView struct {
	*characterdomain.Character
	ProficiencyBonus     int
	ArmorClass           int
	Initiative           int
	PassiveWisdom        int
	StrengthModifier     int
	DexterityModifier    int
	ConstitutionModifier int
	IntelligenceModifier int
	WisdomModifier       int
	CharismaModifier     int
	SpellAbility         string
	SpellSaveDC          int
	SpellAttack          int
}

// NewServer parses embedded templates and creates an HTTP server backed by repository.
func NewServer(repository characterdomain.CharacterRepository) (*Server, error) {
	templates, err := template.New("pages").Funcs(template.FuncMap{
		"abilityModifier": characterdomain.AbilityModifier,
		"pathEscape":      url.PathEscape,
		"upper":           strings.ToUpper,
		"slice":           func(values ...string) []string { return values },
	}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse web templates: %w", err)
	}
	sheet, err := template.New("charactersheet.html").Funcs(template.FuncMap{
		"json": func(value any) template.JS {
			encoded, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return template.JS("null")
			}
			return template.JS(encoded)
		},
	}).ParseFiles("charactersheet.html")
	if err != nil {
		return nil, fmt.Errorf("parse character sheet: %w", err)
	}
	return &Server{
		repository: repository,
		templates:  templates,
		sheet:      sheet,
		client:     &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Handler returns the router containing the web pages and JSON endpoints.
// Handler returns the router containing the web pages and JSON endpoints.
func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/static/", server.static)
	mux.HandleFunc("/normalize.css", server.rootAsset)
	mux.HandleFunc("/style.css", server.rootAsset)
	mux.HandleFunc("/api/characters", server.charactersAPI)
	mux.HandleFunc("/api/spells/", server.spellAPI)
	mux.HandleFunc("/api/equipment/", server.equipmentAPI)
	mux.HandleFunc("/characters/new", server.newCharacter)
	mux.HandleFunc("/characters/", server.characterPage)
	mux.HandleFunc("/", server.index)
	return mux
}

// index renders the list of saved characters and rejects non-root paths.
// index renders the list of saved characters and rejects non-root paths.
func (server *Server) index(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	characters, err := server.repository.GetAll()
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	server.render(response, "index.html", characters)
}

// characterPage loads one character from the URL and renders its calculated sheet.
// characterPage loads one character from the URL and renders its calculated sheet.
func (server *Server) characterPage(response http.ResponseWriter, request *http.Request) {
	name, err := url.PathUnescape(strings.TrimPrefix(request.URL.Path, "/characters/"))
	if err != nil || name == "" {
		http.NotFound(response, request)
		return
	}
	character, err := server.repository.GetByName(name)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	server.renderSheet(response, makeCharacterView(character))
}

// newCharacter renders the creation form or persists a submitted character.
// newCharacter renders the creation form or persists a submitted character.
func (server *Server) newCharacter(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		server.render(response, "create.html", nil)
		return
	}
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "GET, POST")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(response, "invalid form", http.StatusBadRequest)
		return
	}
	character, err := characterFromForm(request)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	if err := server.repository.Create(character); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(response, request, "/characters/"+url.PathEscape(character.Name), http.StatusSeeOther)
}

// charactersAPI serves character reads and validates character creation requests.
// charactersAPI serves character reads and validates character creation requests.
func (server *Server) charactersAPI(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		characters, err := server.repository.GetAll()
		if err != nil {
			writeJSONError(response, err, http.StatusInternalServerError)
			return
		}
		writeJSON(response, characters)
		return
	}
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "GET, POST")
		writeJSONError(response, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	var character characterdomain.Character
	if err := json.NewDecoder(request.Body).Decode(&character); err != nil {
		writeJSONError(response, fmt.Errorf("invalid character JSON: %w", err), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(character.Name) == "" || strings.TrimSpace(character.Race) == "" || strings.TrimSpace(character.Class) == "" {
		writeJSONError(response, fmt.Errorf("name, race, and class are required"), http.StatusBadRequest)
		return
	}
	if err := server.repository.Create(&character); err != nil {
		writeJSONError(response, err, http.StatusInternalServerError)
		return
	}
	writeJSONStatus(response, character, http.StatusCreated)
}

// spellAPI enriches a spell name through the shared external-resource handler.
// spellAPI enriches a spell name through the shared external-resource handler.
func (server *Server) spellAPI(response http.ResponseWriter, request *http.Request) {
	server.enrich(response, "spells", strings.TrimPrefix(request.URL.Path, "/api/spells/"))
}

// equipmentAPI enriches an equipment name through the shared external-resource handler.
// equipmentAPI enriches an equipment name through the shared external-resource handler.
func (server *Server) equipmentAPI(response http.ResponseWriter, request *http.Request) {
	server.enrich(response, "equipment", strings.TrimPrefix(request.URL.Path, "/api/equipment/"))
}

// enrich queries the external D&D API while enforcing a minimum request interval.
// enrich queries the external D&D API while enforcing a minimum request interval.
func (server *Server) enrich(response http.ResponseWriter, resource, name string) {
	if name == "" {
		writeJSONError(response, fmt.Errorf("resource name is required"), http.StatusBadRequest)
		return
	}
	server.rateMu.Lock()
	if wait := 200*time.Millisecond - time.Since(server.lastAPIHit); wait > 0 {
		timer := time.NewTimer(wait)
		<-timer.C
	}
	server.lastAPIHit = time.Now()
	server.rateMu.Unlock()

	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	request, err := http.NewRequest(http.MethodGet, "https://www.dnd5eapi.co/api/2014/"+resource+"/"+url.PathEscape(slug), nil)
	if err != nil {
		writeJSONError(response, err, http.StatusBadRequest)
		return
	}
	result, err := server.client.Do(request)
	if err != nil {
		writeJSONError(response, err, http.StatusBadGateway)
		return
	}
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK {
		writeJSONError(response, fmt.Errorf("external API returned %s", result.Status), result.StatusCode)
		return
	}
	var raw map[string]any
	if err := json.NewDecoder(result.Body).Decode(&raw); err != nil {
		writeJSONError(response, err, http.StatusBadGateway)
		return
	}
	writeJSON(response, map[string]any{"name": raw["name"], "resource": resource, "data": raw})
}

// static serves CSS files from the embedded filesystem.
// static serves CSS files from the embedded filesystem.
func (server *Server) static(response http.ResponseWriter, request *http.Request) {
	http.FileServer(http.FS(assets)).ServeHTTP(response, request)
}

func (server *Server) rootAsset(response http.ResponseWriter, request *http.Request) {
	name := request.URL.Path[1:]
	content, err := os.ReadFile(name)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	if name == "normalize.css" || name == "style.css" {
		response.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	_, _ = response.Write(content)
}

// render executes a named embedded template and reports rendering failures as HTTP errors.
// render executes a named embedded template and reports rendering failures as HTTP errors.
func (server *Server) render(response http.ResponseWriter, name string, data any) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := server.templates.ExecuteTemplate(response, name, data); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) renderSheet(response http.ResponseWriter, data any) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := server.sheet.ExecuteTemplate(response, "charactersheet.html", data); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

// makeCharacterView calculates the derived statistics displayed by the character page.
// makeCharacterView calculates the derived statistics displayed by the character page.
func makeCharacterView(character *characterdomain.Character) characterView {
	ability, score := character.SpellcastingAbility()
	return characterView{
		Character:            character,
		ProficiencyBonus:     characterdomain.ProficiencyBonus(character.Level),
		ArmorClass:           character.ArmorClass(),
		Initiative:           characterdomain.AbilityModifier(character.Dex),
		PassiveWisdom:        10 + characterdomain.AbilityModifier(character.Wis),
		StrengthModifier:     characterdomain.AbilityModifier(character.Str),
		DexterityModifier:    characterdomain.AbilityModifier(character.Dex),
		ConstitutionModifier: characterdomain.AbilityModifier(character.Con),
		IntelligenceModifier: characterdomain.AbilityModifier(character.Int),
		WisdomModifier:       characterdomain.AbilityModifier(character.Wis),
		CharismaModifier:     characterdomain.AbilityModifier(character.Cha),
		SpellAbility:         ability,
		SpellSaveDC:          8 + characterdomain.ProficiencyBonus(character.Level) + characterdomain.AbilityModifier(score),
		SpellAttack:          characterdomain.ProficiencyBonus(character.Level) + characterdomain.AbilityModifier(score),
	}
}

// writeJSON writes a successful JSON response with HTTP 200 status.
// writeJSON writes a successful JSON response with HTTP 200 status.
func writeJSON(response http.ResponseWriter, value any) {
	writeJSONStatus(response, value, http.StatusOK)
}

// writeJSONStatus writes a JSON response with an explicit HTTP status code.
// writeJSONStatus writes a JSON response with an explicit HTTP status code.
func writeJSONStatus(response http.ResponseWriter, value any, status int) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

// writeJSONError formats an error as a JSON object for API clients.
// writeJSONError formats an error as a JSON object for API clients.
func writeJSONError(response http.ResponseWriter, err error, status int) {
	writeJSONStatus(response, map[string]string{"error": err.Error()}, status)
}

// characterFromForm validates form fields and converts them into a domain character.
// characterFromForm validates form fields and converts them into a domain character.
func characterFromForm(request *http.Request) (*characterdomain.Character, error) {
	parse := func(field string) (int, error) {
		var value int
		_, err := fmt.Sscan(request.FormValue(field), &value)
		return value, err
	}
	character := &characterdomain.Character{Name: strings.TrimSpace(request.FormValue("name")), Race: strings.TrimSpace(request.FormValue("race")), Class: strings.TrimSpace(request.FormValue("class")), Background: strings.TrimSpace(request.FormValue("background"))}
	if character.Name == "" || character.Race == "" || character.Class == "" {
		return nil, fmt.Errorf("name, race, and class are required")
	}
	var err error
	if character.Level, err = parse("level"); err != nil {
		return nil, fmt.Errorf("level must be a number")
	}
	for field, target := range map[string]*int{"str": &character.Str, "dex": &character.Dex, "con": &character.Con, "int": &character.Int, "wis": &character.Wis, "cha": &character.Cha} {
		if *target, err = parse(field); err != nil {
			return nil, fmt.Errorf("%s must be a number", field)
		}
	}
	return character, nil
}
