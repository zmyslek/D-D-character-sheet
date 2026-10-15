package main

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"
)

// lineKind says what a physical source line contains.
type lineKind int

const (
	blankLine lineKind = iota
	commentLine
	codeLine
)

// codeText is one line of code with its line number, used for duplication detection.
type codeText struct {
	Line int
	Text string // trimmed, trailing comment removed
}

// unitMetric describes one function or method (a "unit").
type unitMetric struct {
	File       string
	Name       string
	StartLine  int
	EndLine    int
	CodeLines  int // unit size: lines of code, comments and blank lines excluded
	Complexity int // cyclomatic complexity
}

// fileMetric holds everything measured for one source file.
type fileMetric struct {
	Name         string
	TotalLines   int
	CodeLines    int
	CommentLines int
	BlankLines   int
	Units        []unitMetric
	Code         []codeText
}

// analyzeSource measures one Go source file. Only the standard library is used:
// go/scanner classifies lines and go/parser + go/ast give us the functions.
func analyzeSource(name string, src []byte) (*fileMetric, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	kinds, code := classifyLines(fset, src)
	metric := &fileMetric{Name: name, TotalLines: len(kinds), Code: withoutBoilerplate(fset, file, code)}
	countKinds(metric, kinds)
	metric.Units = functionUnits(fset, file, name, kinds)
	return metric, nil
}

// countKinds adds up the code, comment and blank lines of a file.
func countKinds(metric *fileMetric, kinds []lineKind) {
	for _, kind := range kinds {
		switch kind {
		case codeLine:
			metric.CodeLines++
		case commentLine:
			metric.CommentLines++
		default:
			metric.BlankLines++
		}
	}
}

// withoutBoilerplate drops package and import lines: they repeat in every
// file, so they must not count as duplication (they still count as code lines).
func withoutBoilerplate(fset *token.FileSet, file *ast.File, code []codeText) []codeText {
	boilerplate := map[int]bool{fset.Position(file.Package).Line: true}
	for _, declaration := range file.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok && general.Tok == token.IMPORT {
			for line := fset.Position(general.Pos()).Line; line <= fset.Position(general.End()).Line; line++ {
				boilerplate[line] = true
			}
		}
	}
	filtered := code[:0]
	for _, line := range code {
		if !boilerplate[line.Line] {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

// functionUnits measures every function and method that has a body.
func functionUnits(fset *token.FileSet, file *ast.File, name string, kinds []lineKind) []unitMetric {
	var units []unitMetric
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		start := fset.Position(function.Pos()).Line
		end := fset.Position(function.End()).Line
		units = append(units, unitMetric{
			File:       name,
			Name:       functionName(function),
			StartLine:  start,
			EndLine:    end,
			CodeLines:  countCode(kinds, start, end),
			Complexity: complexity(function),
		})
	}
	return units
}

// classifyLines marks every physical line as blank, comment-only or code.
// A line that holds code and a trailing comment counts as code.
func classifyLines(fset *token.FileSet, src []byte) ([]lineKind, []codeText) {
	lines := strings.Split(string(src), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1] // a final newline does not start a new line
	}
	columns := lineColumns{
		kinds:        make([]lineKind, len(lines)),
		firstCodeCol: make([]int, len(lines)), // 1-based column, 0 = none
		commentCol:   make([]int, len(lines)),
	}
	columns.scan(fset, src)
	return columns.kinds, columns.codeTexts(lines)
}

// lineColumns records, per line, its kind and where code and comments start.
type lineColumns struct {
	kinds        []lineKind
	firstCodeCol []int
	commentCol   []int
}

// scan walks the tokens of src and marks the lines they cover.
func (c *lineColumns) scan(fset *token.FileSet, src []byte) {
	file := fset.AddFile("scan", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	for {
		position, tok, literal := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON && literal == "\n" {
			continue // automatically inserted, not real source text
		}
		p := fset.Position(position)
		extra := strings.Count(literal, "\n") // multi-line comments and raw strings
		for line := p.Line; line <= p.Line+extra && line <= len(c.kinds); line++ {
			c.mark(line, p, tok == token.COMMENT)
		}
	}
}

// mark records that a token starting at p covers the given line.
func (c *lineColumns) mark(line int, p token.Position, comment bool) {
	if comment {
		if c.kinds[line-1] == blankLine {
			c.kinds[line-1] = commentLine
		}
		if line == p.Line && c.commentCol[line-1] == 0 {
			c.commentCol[line-1] = p.Column
		}
		return
	}
	c.kinds[line-1] = codeLine
	if c.firstCodeCol[line-1] == 0 {
		c.firstCodeCol[line-1] = p.Column
	}
}

// codeTexts returns the trimmed text of every code line.
func (c *lineColumns) codeTexts(lines []string) []codeText {
	var code []codeText
	for index, kind := range c.kinds {
		if kind != codeLine {
			continue
		}
		text := lines[index]
		// Drop a trailing comment so "x := 1 // note" equals "x := 1".
		if c.commentCol[index] > c.firstCodeCol[index] && c.firstCodeCol[index] > 0 && c.commentCol[index]-1 <= len(text) {
			text = text[:c.commentCol[index]-1]
		}
		code = append(code, codeText{Line: index + 1, Text: strings.TrimSpace(text)})
	}
	return code
}

// countCode counts code lines between start and end (1-based, inclusive).
func countCode(kinds []lineKind, start, end int) int {
	total := 0
	for line := start; line <= end && line <= len(kinds); line++ {
		if kinds[line-1] == codeLine {
			total++
		}
	}
	return total
}

// functionName returns "Name" for functions and "Type.Name" for methods.
func functionName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	receiver := function.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		receiver = star.X
	}
	if identifier, ok := receiver.(*ast.Ident); ok {
		return identifier.Name + "." + function.Name.Name
	}
	return function.Name.Name
}

// complexity returns McCabe cyclomatic complexity: 1 plus the number of decision
// points (if, for, range, non-default case, select case, && and ||).
// Function literals inside the function are counted as part of it.
func complexity(function *ast.FuncDecl) int {
	result := 1
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			result++
		case *ast.CaseClause:
			if n.List != nil { // default has no expressions
				result++
			}
		case *ast.CommClause:
			if n.Comm != nil {
				result++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				result++
			}
		}
		return true
	})
	return result
}

// duplicationWindow is the number of consecutive code lines that must repeat
// before they count as a duplicate (the common 6-line rule).
const duplicationWindow = 6

// findDuplicates returns, per file name, the set of line numbers that belong to
// a block of at least duplicationWindow identical consecutive code lines that
// appears more than once (in the same or in another file).
func findDuplicates(files []*fileMetric) map[string]map[int]bool {
	duplicated := make(map[string]map[int]bool)
	for _, locations := range codeWindows(files) {
		if len(locations) < 2 {
			continue
		}
		for _, place := range locations {
			if duplicated[place.file.Name] == nil {
				duplicated[place.file.Name] = make(map[int]bool)
			}
			for offset := 0; offset < duplicationWindow; offset++ {
				duplicated[place.file.Name][place.file.Code[place.index+offset].Line] = true
			}
		}
	}
	return duplicated
}

// location is the start of a code window inside a file.
type location struct {
	file  *fileMetric
	index int
}

// codeWindows groups every run of duplicationWindow consecutive code lines by
// its text, so equal runs end up under the same key.
func codeWindows(files []*fileMetric) map[string][]location {
	seen := make(map[string][]location)
	for _, file := range files {
		for index := 0; index+duplicationWindow <= len(file.Code); index++ {
			texts := make([]string, duplicationWindow)
			for offset := range texts {
				texts[offset] = file.Code[index+offset].Text
			}
			key := strings.Join(texts, "\n")
			seen[key] = append(seen[key], location{file, index})
		}
	}
	return seen
}
