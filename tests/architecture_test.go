package tests

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// importsOf returns the imports of every non-test Go file in dir.
func importsOf(t *testing.T, dir string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no Go files in %s: %v", dir, err)
	}
	imports := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			imports[path] = true
		}
	}
	return imports
}

// The dependency rule: domain <- application <- infrastructure / web / main.
// The domain knows nothing about storage, HTTP or the CLI, and the
// application layer depends on interfaces, not on infrastructure.
func TestDependencyDirection(t *testing.T) {
	rules := []struct {
		dir       string
		forbidden []string
	}{
		{"internal/domain", []string{
			"mpp-zmyslek/internal/application", "mpp-zmyslek/internal/infrastructure", "mpp-zmyslek/web",
			"net/http", "encoding/json", "encoding/csv", "os",
		}},
		{"internal/application", []string{
			"mpp-zmyslek/internal/infrastructure", "mpp-zmyslek/web", "net/http", "encoding/csv", "os",
		}},
		{"internal/infrastructure", []string{"mpp-zmyslek/internal/application", "mpp-zmyslek/web"}},
		{"web", []string{"mpp-zmyslek/internal/infrastructure"}},
	}
	for _, rule := range rules {
		imports := importsOf(t, rule.dir)
		for _, forbidden := range rule.forbidden {
			if imports[forbidden] {
				t.Errorf("%s must not import %s", rule.dir, forbidden)
			}
		}
	}
}

// All tests live in the tests folder (the project's convention).
func TestTestsLiveInTheTestsFolder(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "tests") {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, "_test.go") {
			t.Errorf("test file outside tests/: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
