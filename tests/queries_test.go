package tests

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mpp-zmyslek/internal/application"
	"mpp-zmyslek/internal/domain"
	"mpp-zmyslek/internal/infrastructure"
)

// fakeLookup answers lookups locally and records how many run at once.
type fakeLookup struct {
	delay        time.Duration
	active, peak atomic.Int32
}

func (f *fakeLookup) Lookup(ctx context.Context, resource, name string) (map[string]any, error) {
	now := f.active.Add(1)
	defer f.active.Add(-1)
	// Raise the recorded peak if this lookup pushed it higher.
	for {
		peak := f.peak.Load()
		if now <= peak || f.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if name == "broken" {
		return nil, errors.New("upstream failed")
	}
	return map[string]any{"name": name, "resource": resource}, nil
}

// storeCharacter writes a character with equipment and spells straight to a
// repository, so query tests do not depend on the commands.
func storeCharacter(t *testing.T, file string, data domain.CharacterData) {
	t.Helper()
	if err := infrastructure.NewJSONCharacterRepository(file).Create(domain.RestoreCharacter(data)); err != nil {
		t.Fatal(err)
	}
}

func TestQueriesDoNotChangeStoredData(t *testing.T) {
	commands, queries, _ := newService(t)
	if _, _, err := commands.Create(wizardRequest("Mira"), ""); err != nil {
		t.Fatal(err)
	}
	character, err := queries.Get("Mira")
	if err != nil {
		t.Fatal(err)
	}
	// A query returns a fresh copy: changing it without a command saves nothing.
	if err := character.ChangeLevel(10); err != nil {
		t.Fatal(err)
	}
	stored, _ := queries.Get("Mira")
	if stored.Level() != 1 {
		t.Errorf("stored level = %d; changing a query result must not be saved", stored.Level())
	}

}

func TestEnrichCharacterRunsLookupsConcurrently(t *testing.T) {
	file := t.TempDir() + "/characters.json"
	lookup := &fakeLookup{delay: 50 * time.Millisecond}
	_, queries := servicesFor(file, lookup)
	storeCharacter(t, file, domain.CharacterData{
		Name: "Mira", Race: "elf", Class: "wizard", Level: 5, MainHand: "Dagger", Armor: "Leather Armor",
		PreparedSpells: []string{"fire bolt", "broken", "fireball"},
	})

	started := time.Now()
	results, err := queries.EnrichCharacter(context.Background(), "Mira")
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)

	var names []string
	for _, result := range results {
		names = append(names, result.Name)
		switch {
		case result.Name == "broken" && (result.Error == "" || result.Data != nil):
			t.Errorf("a failed lookup must keep its error: %+v", result)
		case result.Name != "broken" && (result.Error != "" || result.Data["name"] != result.Name):
			t.Errorf("result %+v has no data", result)
		}
	}
	if strings.Join(names, ",") != "Dagger,Leather Armor,fire bolt,broken,fireball" {
		t.Errorf("results are not in sheet order: %v", names)
	}
	// Five 50 ms lookups one after another would take 250 ms.
	if lookup.peak.Load() < 2 || elapsed > 200*time.Millisecond {
		t.Errorf("peak concurrency %d, took %v; lookups did not run in parallel", lookup.peak.Load(), elapsed)
	}
	if _, err := queries.EnrichCharacter(context.Background(), "Nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown character err = %v", err)
	}
}

func TestEnrichCharacterStopsWhenCancelled(t *testing.T) {
	file := t.TempDir() + "/characters.json"
	_, queries := servicesFor(file, &fakeLookup{delay: 5 * time.Second})
	storeCharacter(t, file, domain.CharacterData{Name: "Mira", Class: "wizard", Level: 1, KnownSpells: []string{"a", "b", "c"}})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	results, err := queries.EnrichCharacter(ctx, "Mira")
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > time.Second {
		t.Error("cancellation did not stop the lookups")
	}
	for _, result := range results {
		if !strings.Contains(result.Error, "deadline exceeded") {
			t.Errorf("%s: error = %q, want the cancellation", result.Name, result.Error)
		}
	}
}

// fakeSRDAPI is a stand-in for dnd5eapi.co that counts requests.
func fakeSRDAPI(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		hits.Add(1)
		switch request.URL.Path {
		case "/spells/fire-bolt":
			_, _ = response.Write([]byte(`{"name":"Fire Bolt","range":"120 feet","school":{"name":"Evocation"}}`))
		case "/spells/garbage":
			_, _ = response.Write([]byte(`{not json`))
		case "/spells/crash":
			http.Error(response, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(response, request)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestSRDAPIClient(t *testing.T) {
	var hits atomic.Int32
	client := infrastructure.NewSRDAPIClient(fakeSRDAPI(t, &hits).URL + "/")
	ctx := context.Background()

	data, err := client.Lookup(ctx, "spells", "Fire Bolt")
	if err != nil || data["name"] != "Fire Bolt" || data["range"] != "120 feet" {
		t.Errorf("Lookup = %v, %v", data, err)
	}
	if _, err := client.Lookup(ctx, "spells", "Wish Upon"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown spell err = %v, want ErrNotFound", err)
	}
	for _, name := range []string{"garbage", "crash"} {
		if _, err := client.Lookup(ctx, "spells", name); err == nil || errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: err = %v, want an upstream error", name, err)
		}
	}
	down := infrastructure.NewSRDAPIClient("http://127.0.0.1:1")
	if _, err := down.Lookup(ctx, "spells", "light"); err == nil {
		t.Error("an unreachable API must return an error")
	}
}

// The client must space requests at least 200 ms apart (5 per second), even
// when many goroutines ask at the same moment.
func TestSRDAPIClientRateLimit(t *testing.T) {
	var hits atomic.Int32
	client := infrastructure.NewSRDAPIClient(fakeSRDAPI(t, &hits).URL)
	started := time.Now()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := client.Lookup(context.Background(), "spells", "fire bolt"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	// 4 requests need at least 3 intervals of 200 ms. Only a lower bound is
	// checked, so the test cannot fail on a slow machine.
	if elapsed := time.Since(started); elapsed < 600*time.Millisecond || hits.Load() != 4 {
		t.Errorf("4 requests took %v with %d hits; the rate limit is not applied", elapsed, hits.Load())
	}

	// A caller that gives up while waiting for its turn gets ctx's error.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.Lookup(ctx, "spells", "fire bolt"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want deadline exceeded", err)
	}
}

func TestQueriesWithoutCharacters(t *testing.T) {
	queries := application.NewCharacterQueries(infrastructure.NewJSONCharacterRepository(t.TempDir()+"/none.json"), nil)
	if all, err := queries.GetAll(); err != nil || len(all) != 0 {
		t.Errorf("GetAll = %v, %v", all, err)
	}
	if _, err := queries.Get("Nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}
