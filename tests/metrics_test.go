package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A small program whose numbers can be counted by hand (34 lines in total:
// 27 code, 3 comment, 4 blank). Branchy has cyclomatic complexity
// 1 + if + && + for + case + case = 6; "default" adds nothing.
const metricsSample = `package sample

// Simple has no branches.
func Simple() int {
	return 1
}

/* block
comment */
func Branchy(a, b int) int {
	total := 0 // trailing comment
	if a > 0 && b > 0 {
		total++
	}
	for i := 0; i < a; i++ {
		switch i {
		case 1, 2:
			total++
		case 3:
			total--
		default:
			total += 2
		}
	}
	return total
}

type T struct{}

func (t *T) Method() string {
	s := ` + "`raw\nstring`" + `
	return s
}
`

func writeSource(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMetricsLinesUnitSizeAndComplexity(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "sample.go", metricsSample)

	out, code := run(t, metricsBinary, t.TempDir(), "-dir", dir)
	if code != 0 {
		t.Fatalf("exit code %d: %s", code, out)
	}
	expectContains(t, out,
		"total lines: 34", "code lines: 27", "comment lines: 3", "blank lines: 4",
		"files: 1")
	// Unit size counts code lines only; the raw string spans two code lines.
	for _, want := range []string{"Simple", "lines=3 ", "cc=1 ", "Branchy", "lines=17", "cc=6 ", "T.Method", "lines=5 "} {
		expectContains(t, out, want)
	}
}

func TestMetricsDuplication(t *testing.T) {
	dir := t.TempDir()
	block := "\ta := 1\n\tb := 2\n\tc := 3\n\td := 4\n\te := 5\n\tf := 6\n"
	writeSource(t, dir, "a.go", "package p\n\nfunc One() {\n"+block+"}\n")
	writeSource(t, dir, "b.go", "package p\n\n// other comment\nfunc Two() {\n"+block+"\tg := 7\n}\n")
	writeSource(t, dir, "c.go", "package p\n\nfunc Three() {\n\tz := 99\n}\n")

	out, _ := run(t, metricsBinary, t.TempDir(), "-dir", dir)
	section := out[strings.Index(out, "DUPLICATION"):]
	expectContains(t, section, "a.go", "b.go")
	if strings.Contains(section, "c.go") {
		t.Errorf("the unique file must not be reported:\n%s", section)
	}
	if strings.Contains(section, "duplicated lines: 0 of") {
		t.Errorf("the repeated block should be detected:\n%s", section)
	}
}

func TestMetricsFlags(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "main.go", metricsSample)
	writeSource(t, dir, "main_test.go", "package sample\nfunc TestX() {}\n")
	writeSource(t, dir, "tools/skipped.go", "package skipped\n")

	out, _ := run(t, metricsBinary, t.TempDir(), "-dir", dir)
	expectContains(t, out, "files: 1")

	out, _ = run(t, metricsBinary, t.TempDir(), "-dir", dir, "-tests")
	expectContains(t, out, "files: 2")

	out, _ = run(t, metricsBinary, t.TempDir(), "-dir", dir, "-skip", "none")
	expectContains(t, out, "files: 2") // main.go plus tools/skipped.go
}

func TestMetricsSyntaxErrorFails(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "broken.go", "package x\nfunc (")
	if out, code := run(t, metricsBinary, t.TempDir(), "-dir", dir); code != 1 {
		t.Errorf("exit code = %d, want 1; output: %s", code, out)
	}
}
