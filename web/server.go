package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"mpp-zmyslek/internal/application"
	characterdomain "mpp-zmyslek/internal/domain"
)

// Route prefixes and header values used in several handlers.
const (
	apiCharactersPrefix = "/api/characters/"
	pagesPrefix         = "/characters/"
	contentType         = "Content-Type"
	htmlContentType     = "text/html; charset=utf-8"
	methodNotAllowed    = "method not allowed"
)

var errMethodNotAllowed = errors.New(methodNotAllowed)

// assets contains the templates and styles shipped inside the server binary.
//
//go:embed templates/*.html static/*.css
var assets embed.FS

// Server coordinates HTTP routes, template rendering, and API calls. It keeps
// no per-user state; all character data goes through the CharacterService.
type Server struct {
	commands  *application.CharacterService
	queries   *application.CharacterQueries
	templates *template.Template
	instance  string
}

// characterView combines a character's data with values calculated for its web sheet.
type characterView struct {
	characterdomain.CharacterData
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
	CanCastSpells        bool
	SpellAbility         string
	SpellSaveDC          int
	SpellAttack          int
	Editing              bool
	// SpellcastingByClass holds, per class and level 0-20, the domain's
	// spellcasting rules, so the sheet's script can update slots when the
	// class or level is edited without keeping its own copy of the SRD tables.
	SpellcastingByClass map[string][]characterdomain.Spellcasting
}

// srdClasses are the twelve SRD classes shown in the sheet's spellcasting table.
var srdClasses = []string{
	"barbarian", "bard", "cleric", "druid", "fighter", "monk",
	"paladin", "ranger", "rogue", "sorcerer", "warlock", "wizard",
}

// spellcastingByClass is computed once from the domain rules.
var spellcastingByClass = func() map[string][]characterdomain.Spellcasting {
	table := make(map[string][]characterdomain.Spellcasting, len(srdClasses))
	for _, class := range srdClasses {
		levels := make([]characterdomain.Spellcasting, characterdomain.MaxLevel+1)
		for level := range levels {
			levels[level] = characterdomain.SpellcastingFor(class, level)
		}
		table[class] = levels
	}
	return table
}()

// NewServer parses embedded templates and creates an HTTP server. Commands
// change characters; queries read them.
func NewServer(commands *application.CharacterService, queries *application.CharacterQueries) (*Server, error) {
	templates, err := template.New("pages").Funcs(template.FuncMap{
		"abilityModifier": characterdomain.AbilityModifier,
		"pathEscape":      url.PathEscape,
		"upper":           strings.ToUpper,
		"title":           titleCase,
		"slice":           func(values ...string) []string { return values },
		"json": func(value any) template.JS {
			encoded, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return template.JS("null")
			}
			return template.JS(encoded)
		},
	}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse web templates: %w", err)
	}
	// The host name identifies this instance when several run behind a load balancer.
	instance, _ := os.Hostname()
	return &Server{commands: commands, queries: queries, templates: templates, instance: instance}, nil
}

// Handler returns the router containing the web pages and JSON endpoints.
func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/static/", server.static)
	mux.HandleFunc("/normalize.css", server.rootAsset)
	mux.HandleFunc("/style.css", server.rootAsset)
	mux.HandleFunc("/api/characters", server.charactersAPI)
	mux.HandleFunc(apiCharactersPrefix, server.characterAPI)
	mux.HandleFunc("/api/spells/", server.spellAPI)
	mux.HandleFunc("/api/equipment/", server.equipmentAPI)
	mux.HandleFunc("/characters/new", server.newCharacter)
	mux.HandleFunc(pagesPrefix, server.characterPage)
	mux.HandleFunc("/healthz", server.health)
	mux.HandleFunc("/", server.index)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Instance", server.instance)
		mux.ServeHTTP(response, request)
	})
}

// health answers the liveness checks of Docker and the load balancer.
func (server *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, map[string]string{"status": "ok", "instance": server.instance})
}

// index renders the list of saved characters and rejects non-root paths.
func (server *Server) index(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	characters, err := server.queries.GetAll()
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	server.render(response, "index.html", characterData(characters))
}

// characterPage loads one character from the URL and renders its calculated sheet.
func (server *Server) characterPage(response http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.EscapedPath(), pagesPrefix)
	if strings.HasSuffix(path, "/delete") {
		server.deleteCharacter(response, request, strings.TrimSuffix(path, "/delete"))
		return
	}
	edit := strings.HasSuffix(path, "/edit")
	if edit {
		path = strings.TrimSuffix(path, "/edit")
	}
	name, err := url.PathUnescape(path)
	if err != nil || name == "" {
		http.NotFound(response, request)
		return
	}
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, methodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	character, err := server.queries.Get(name)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	if edit {
		view := makeCharacterView(character)
		view.Editing = true
		server.renderSheet(response, view)
		return
	}
	server.renderSheet(response, makeCharacterView(character))
}

// deleteCharacter removes the URL-selected record after a form submits a POST.
func (server *Server) deleteCharacter(response http.ResponseWriter, request *http.Request, escapedName string) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		http.Error(response, methodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	name, err := url.PathUnescape(escapedName)
	if err != nil || name == "" {
		http.NotFound(response, request)
		return
	}
	if err := server.commands.Delete(name); err != nil {
		http.NotFound(response, request)
		return
	}
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

// characterAPI reads a named record with GET, updates it with PUT or removes it with DELETE.
func (server *Server) characterAPI(response http.ResponseWriter, request *http.Request) {
	if strings.HasSuffix(request.URL.Path, "/enrichment") && request.Method == http.MethodGet {
		server.characterEnrichment(response, request)
		return
	}
	name, err := url.PathUnescape(strings.TrimPrefix(request.URL.EscapedPath(), apiCharactersPrefix))
	if err != nil || strings.TrimSpace(name) == "" {
		writeJSONError(response, fmt.Errorf("character name is required"), http.StatusBadRequest)
		return
	}
	switch request.Method {
	case http.MethodGet:
		character, err := server.queries.Get(name)
		if err != nil {
			writeJSONError(response, err, statusFor(err))
			return
		}
		writeJSON(response, character.Data())
	case http.MethodPut:
		server.updateCharacter(response, request, name)
	case http.MethodDelete:
		if err := server.commands.Delete(name); err != nil {
			writeJSONError(response, err, statusFor(err))
			return
		}
		writeJSONStatus(response, map[string]string{"deleted": name}, http.StatusOK)
	default:
		response.Header().Set("Allow", "GET, PUT, DELETE")
		writeJSONError(response, errMethodNotAllowed, http.StatusMethodNotAllowed)
	}
}

// updateCharacter saves an edited sheet. The name comes from the URL, so an
// edit cannot silently create a renamed record.
func (server *Server) updateCharacter(response http.ResponseWriter, request *http.Request, name string) {
	var data characterdomain.CharacterData
	if err := json.NewDecoder(request.Body).Decode(&data); err != nil {
		writeJSONError(response, fmt.Errorf("invalid character JSON: %w", err), http.StatusBadRequest)
		return
	}
	if data.Name != name {
		writeJSONError(response, fmt.Errorf("name must match the URL"), http.StatusBadRequest)
		return
	}
	character, err := server.commands.Update(name, application.CharacterEdit{
		Race: data.Race, Class: data.Class, Level: data.Level, Scores: scoresOf(data),
		MainHand: data.MainHand, OffHand: data.OffHand, Armor: data.Armor, Shield: data.Shield,
		KnownSpells: data.KnownSpells, PreparedSpells: data.PreparedSpells,
	})
	if err != nil {
		writeJSONError(response, err, statusFor(err))
		return
	}
	writeJSON(response, character.Data())
}

// newCharacter renders the creation form or persists a submitted character.
func (server *Server) newCharacter(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		server.renderSheet(response, makeCharacterView(characterdomain.RestoreCharacter(characterdomain.CharacterData{
			Level: 1, Str: 15, Dex: 14, Con: 13, Int: 12, Wis: 10, Cha: 8,
		})))
		return
	}
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "GET, POST")
		http.Error(response, methodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(response, "invalid form", http.StatusBadRequest)
		return
	}
	createRequest, err := createRequestFromForm(request)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	character, _, err := server.commands.Create(createRequest, "")
	if err != nil {
		http.Error(response, err.Error(), statusFor(err))
		return
	}
	http.Redirect(response, request, pagesPrefix+url.PathEscape(character.Name()), http.StatusSeeOther)
}

// charactersAPI serves character reads and validates character creation requests.
func (server *Server) charactersAPI(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		characters, err := server.queries.GetAll()
		if err != nil {
			writeJSONError(response, err, http.StatusInternalServerError)
			return
		}
		writeJSON(response, characterData(characters))
		return
	}
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "GET, POST")
		writeJSONError(response, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	var data characterdomain.CharacterData
	if err := json.NewDecoder(request.Body).Decode(&data); err != nil {
		writeJSONError(response, fmt.Errorf("invalid character JSON: %w", err), http.StatusBadRequest)
		return
	}
	// A client that may retry (for example after a timeout) sends the same
	// Idempotency-Key each time; the character is then created only once.
	// Skill proficiencies are not taken from the request: the class grants its
	// first N options and the background its own skills (SRD + assignment rule).
	character, replayed, err := server.commands.Create(application.CreateRequest{
		Name: data.Name, Race: data.Race, Class: data.Class, Background: data.Background, Level: data.Level,
		Scores:      scoresOf(data),
		KnownSpells: data.KnownSpells, PreparedSpells: data.PreparedSpells,
		RequireStandardArray: true, RejectDuplicate: true,
	}, request.Header.Get("Idempotency-Key"))
	if err != nil {
		writeJSONError(response, err, statusFor(err))
		return
	}
	if replayed {
		response.Header().Set("Idempotent-Replayed", "true")
	}
	writeJSONStatus(response, character.Data(), http.StatusCreated)
}

// spellAPI enriches a spell name through the shared external-resource handler.
func (server *Server) spellAPI(response http.ResponseWriter, request *http.Request) {
	server.enrich(response, request, "spells", strings.TrimPrefix(request.URL.Path, "/api/spells/"))
}

// equipmentAPI enriches an equipment name through the shared external-resource handler.
func (server *Server) equipmentAPI(response http.ResponseWriter, request *http.Request) {
	server.enrich(response, request, "equipment", strings.TrimPrefix(request.URL.Path, "/api/equipment/"))
}

// enrich returns the external details of one spell or item.
func (server *Server) enrich(response http.ResponseWriter, request *http.Request, resource, name string) {
	if name == "" {
		writeJSONError(response, fmt.Errorf("resource name is required"), http.StatusBadRequest)
		return
	}
	data, err := server.queries.Lookup(request.Context(), resource, name)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, characterdomain.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSONError(response, err, status)
		return
	}
	writeJSON(response, map[string]any{"name": data["name"], "resource": resource, "data": data})
}

// characterEnrichment looks up all spells and items of one character at once.
// The lookups stop after 30 seconds or when the client disconnects.
func (server *Server) characterEnrichment(response http.ResponseWriter, request *http.Request) {
	name, err := url.PathUnescape(strings.TrimSuffix(strings.TrimPrefix(request.URL.EscapedPath(), apiCharactersPrefix), "/enrichment"))
	if err != nil {
		writeJSONError(response, err, http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	defer cancel()
	results, err := server.queries.EnrichCharacter(ctx, name)
	if err != nil {
		writeJSONError(response, err, statusFor(err))
		return
	}
	writeJSON(response, results)
}

// static serves CSS files from the embedded filesystem.
func (server *Server) static(response http.ResponseWriter, request *http.Request) {
	staticAssets, err := fs.Sub(assets, "static")
	if err != nil {
		http.Error(response, "static assets unavailable", http.StatusInternalServerError)
		return
	}
	http.StripPrefix("/static/", http.FileServer(http.FS(staticAssets))).ServeHTTP(response, request)
}

func (server *Server) rootAsset(response http.ResponseWriter, request *http.Request) {
	name := request.URL.Path[1:]
	content, err := assets.ReadFile("static/" + name)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	if name == "normalize.css" || name == "style.css" {
		response.Header().Set(contentType, "text/css; charset=utf-8")
	}
	_, _ = response.Write(content)
}

// render executes a named embedded template and reports rendering failures as HTTP errors.
func (server *Server) render(response http.ResponseWriter, name string, data any) {
	response.Header().Set(contentType, htmlContentType)
	if err := server.templates.ExecuteTemplate(response, name, data); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) renderSheet(response http.ResponseWriter, data any) {
	response.Header().Set(contentType, htmlContentType)
	if err := server.templates.ExecuteTemplate(response, "charactersheet.html", data); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

// makeCharacterView calculates the derived statistics displayed by the character page.
func makeCharacterView(character *characterdomain.Character) characterView {
	ability, _ := character.SpellcastingAbility()
	if !character.CanCastSpells() {
		ability = ""
	}
	scores := character.Scores()
	return characterView{
		CharacterData:        character.Data(),
		ProficiencyBonus:     character.ProficiencyBonus(),
		ArmorClass:           character.ArmorClass(),
		Initiative:           character.Initiative(),
		PassiveWisdom:        character.PassivePerception(),
		StrengthModifier:     characterdomain.AbilityModifier(scores.Str),
		DexterityModifier:    characterdomain.AbilityModifier(scores.Dex),
		ConstitutionModifier: characterdomain.AbilityModifier(scores.Con),
		IntelligenceModifier: characterdomain.AbilityModifier(scores.Int),
		WisdomModifier:       characterdomain.AbilityModifier(scores.Wis),
		CharismaModifier:     characterdomain.AbilityModifier(scores.Cha),
		CanCastSpells:        character.CanCastSpells(),
		SpellAbility:         ability,
		SpellSaveDC:          character.SpellSaveDC(),
		SpellAttack:          character.SpellAttackBonus(),
		SpellcastingByClass:  spellcastingByClass,
	}
}

// characterData converts characters into plain data for templates and JSON.
func characterData(characters []*characterdomain.Character) []characterdomain.CharacterData {
	data := make([]characterdomain.CharacterData, 0, len(characters))
	for _, character := range characters {
		data = append(data, character.Data())
	}
	return data
}

func scoresOf(data characterdomain.CharacterData) characterdomain.AbilityScores {
	return characterdomain.AbilityScores{Str: data.Str, Dex: data.Dex, Con: data.Con, Int: data.Int, Wis: data.Wis, Cha: data.Cha}
}

// statusFor maps service errors to HTTP status codes; anything else is a
// rule violation in the request (400).
func statusFor(err error) int {
	switch {
	case errors.Is(err, characterdomain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, application.ErrAlreadyExists), errors.Is(err, application.ErrIdempotencyKeyReused):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

// writeJSON writes a successful JSON response with HTTP 200 status.
func writeJSON(response http.ResponseWriter, value any) {
	writeJSONStatus(response, value, http.StatusOK)
}

// writeJSONStatus writes a JSON response with an explicit HTTP status code.
func writeJSONStatus(response http.ResponseWriter, value any, status int) {
	response.Header().Set(contentType, "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

// writeJSONError formats an error as a JSON object for API clients.
func writeJSONError(response http.ResponseWriter, err error, status int) {
	writeJSONStatus(response, map[string]string{"error": err.Error()}, status)
}

// createRequestFromForm validates form fields and converts them into a create request.
func createRequestFromForm(request *http.Request) (application.CreateRequest, error) {
	numbers := map[string]int{}
	for _, field := range []string{"level", "str", "dex", "con", "int", "wis", "cha"} {
		value, err := strconv.Atoi(strings.TrimSpace(request.FormValue(field)))
		if err != nil {
			return application.CreateRequest{}, fmt.Errorf("%s must be a number", field)
		}
		numbers[field] = value
	}
	return application.CreateRequest{
		Name:       strings.TrimSpace(request.FormValue("name")),
		Race:       strings.TrimSpace(request.FormValue("race")),
		Class:      strings.TrimSpace(request.FormValue("class")),
		Background: strings.TrimSpace(request.FormValue("background")),
		Level:      numbers["level"],
		Scores: characterdomain.AbilityScores{
			Str: numbers["str"], Dex: numbers["dex"], Con: numbers["con"],
			Int: numbers["int"], Wis: numbers["wis"], Cha: numbers["cha"],
		},
		RequireStandardArray: true,
		RejectDuplicate:      true,
	}, nil
}

// titleCase capitalizes each word of a stored (usually lowercase) value.
func titleCase(value string) string {
	words := strings.Fields(strings.ToLower(value))
	for index, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[index] = string(runes)
	}
	return strings.Join(words, " ")
}
