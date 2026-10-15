// Command metrics measures a Go code base using only the standard library:
// total lines, unit size, cyclomatic complexity and duplication.
//
//	go run ./tools/metrics            # analyse the project in the current directory
//	go run ./tools/metrics -top 15    # list more of the worst units
//	go run ./tools/metrics -tests     # include *_test.go files
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// band is one risk category with an inclusive upper limit (the last has none).
type band struct {
	Label string
	Max   int
}

// Risk bands follow the SIG/Software Improvement Group maintainability model.
var (
	sizeBands       = []band{{"1-15 lines (low risk)", 15}, {"16-30 lines (moderate)", 30}, {"31-60 lines (high)", 60}, {">60 lines (very high)", 1 << 30}}
	complexityBands = []band{{"CC 1-5 (low risk)", 5}, {"CC 6-10 (moderate)", 10}, {"CC 11-25 (high)", 25}, {"CC >25 (very high)", 1 << 30}}
)

func main() {
	root := flag.String("dir", ".", "directory to analyse")
	includeTests := flag.Bool("tests", false, "also analyse *_test.go files")
	skip := flag.String("skip", "tools,.git,vendor", "comma-separated directory names to skip")
	top := flag.Int("top", 10, "how many of the largest/most complex units to list")
	flag.Parse()

	files, err := collect(*root, *includeTests, strings.Split(*skip, ","))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report(os.Stdout, files, *top)
}

// collect parses every Go file below root.
func collect(root string, includeTests bool, skip []string) ([]*fileMetric, error) {
	skipped := make(map[string]bool)
	for _, name := range skip {
		skipped[strings.TrimSpace(name)] = true
	}
	var files []*fileMetric
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skipped[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || (!includeTests && strings.HasSuffix(path, "_test.go")) {
			return nil
		}
		metric, err := analyzeFile(root, path)
		if err != nil {
			return err
		}
		files = append(files, metric)
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, err
}

// analyzeFile reads and measures one file, naming it relative to root.
func analyzeFile(root, path string) (*fileMetric, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	relative, _ := filepath.Rel(root, path)
	metric, err := analyzeSource(filepath.ToSlash(relative), source)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return metric, nil
}

// percentages distributes the weighted values over the risk bands.
func percentages(bands []band, units []unitMetric, value func(unitMetric) int) []float64 {
	totals := make([]int, len(bands))
	sum := 0
	for _, unit := range units {
		for index, b := range bands {
			if value(unit) <= b.Max {
				totals[index] += unit.CodeLines
				sum += unit.CodeLines
				break
			}
		}
	}
	result := make([]float64, len(bands))
	for index, total := range totals {
		if sum > 0 {
			result[index] = 100 * float64(total) / float64(sum)
		}
	}
	return result
}

// report prints the four metrics.
func report(out io.Writer, files []*fileMetric, top int) {
	var total, code, comment, blank int
	var units []unitMetric
	for _, file := range files {
		total += file.TotalLines
		code += file.CodeLines
		comment += file.CommentLines
		blank += file.BlankLines
		units = append(units, file.Units...)
	}

	fmt.Fprintln(out, "== 1. VOLUME (total lines) ==")
	fmt.Fprintf(out, "files: %d\ntotal lines: %d\ncode lines: %d\ncomment lines: %d\nblank lines: %d\n", len(files), total, code, comment, blank)
	fmt.Fprintf(out, "%-45s %6s %6s %6s %6s\n", "file", "total", "code", "cmnt", "blank")
	for _, file := range files {
		fmt.Fprintf(out, "%-45s %6d %6d %6d %6d\n", file.Name, file.TotalLines, file.CodeLines, file.CommentLines, file.BlankLines)
	}

	fmt.Fprintf(out, "\n== 2. UNIT SIZE (code lines per function, %d units) ==\n", len(units))
	for index, share := range percentages(sizeBands, units, func(u unitMetric) int { return u.CodeLines }) {
		fmt.Fprintf(out, "%-26s %5.1f%% of unit code\n", sizeBands[index].Label, share)
	}
	printTop(out, units, top, func(a, b unitMetric) bool { return a.CodeLines > b.CodeLines }, "largest units")

	fmt.Fprintln(out, "\n== 3. CYCLOMATIC COMPLEXITY (per function) ==")
	for index, share := range percentages(complexityBands, units, func(u unitMetric) int { return u.Complexity }) {
		fmt.Fprintf(out, "%-26s %5.1f%% of unit code\n", complexityBands[index].Label, share)
	}
	printTop(out, units, top, func(a, b unitMetric) bool { return a.Complexity > b.Complexity }, "most complex units")

	duplicated := findDuplicates(files)
	duplicatedLines := 0
	for _, lines := range duplicated {
		duplicatedLines += len(lines)
	}
	fmt.Fprintf(out, "\n== 4. DUPLICATION (blocks of %d+ identical code lines) ==\n", duplicationWindow)
	if code > 0 {
		fmt.Fprintf(out, "duplicated lines: %d of %d code lines = %.1f%%\n", duplicatedLines, code, 100*float64(duplicatedLines)/float64(code))
	}
	names := make([]string, 0, len(duplicated))
	for name := range duplicated {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(out, "  %-43s %4d duplicated lines\n", name, len(duplicated[name]))
	}
}

func printTop(out io.Writer, units []unitMetric, count int, before func(a, b unitMetric) bool, title string) {
	sorted := append([]unitMetric(nil), units...)
	sort.SliceStable(sorted, func(i, j int) bool { return before(sorted[i], sorted[j]) })
	if count > len(sorted) {
		count = len(sorted)
	}
	fmt.Fprintf(out, "top %d %s:\n", count, title)
	for _, unit := range sorted[:count] {
		fmt.Fprintf(out, "  %-40s lines=%-4d cc=%-3d (%s:%d)\n", unit.Name, unit.CodeLines, unit.Complexity, unit.File, unit.StartLine)
	}
}
