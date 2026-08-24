// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

type parityManifest struct {
	Schema    string          `json:"schema"`
	Inventory parityInventory `json:"inventory"`
}

type parityInventory struct {
	Paired             []string             `json:"paired"`
	FanOut             []parityMultiplicity `json:"fanOut"`
	GoStructural       []string             `json:"goStructural"`
	StarlarkStructural []string             `json:"starlarkStructural"`
}

type parityMultiplicity struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Go       int    `json:"go"`
	Starlark int    `json:"starlark"`
	Reason   string `json:"reason"`
}

type parityItem struct {
	ID             string
	Kind           string
	AnnotationKind string
	Line           int
}

type parityKey struct {
	ID   string
	Kind string
}

type parityCounts struct {
	Go       int
	Starlark int
}

var parityIDPattern = regexp.MustCompile(`PARITY-(FUNCTION|BRANCH): (SBP-[A-Z0-9-]+)`)
var starlarkFunctionPattern = regexp.MustCompile(`^def\s+[A-Za-z0-9_]+\(`)
var starlarkBranchPattern = regexp.MustCompile(`^\s*(?:if|elif)\b`)

var strategyGoFiles = []string{"bot.go", "board.go", "score.go", "memory.go", "systems.go"}

// TestEveryStrategyConstructHasCategorizedParityID parses the source rather
// than merely comparing today's comments. A new Go function/if or Starlark
// def/if/elif without an annotation fails. An annotated construct also fails
// until its ID is explicitly classified as paired or implementation-structural
// in parity-manifest.json.
func TestEveryStrategyConstructHasCategorizedParityID(t *testing.T) {
	manifest := readParityManifest(t)
	goItems := goParityItems(t)
	starlarkItems := starlarkParityItems(t)

	actual := make(map[parityKey]parityCounts)
	countItems(t, actual, goItems, true)
	countItems(t, actual, starlarkItems, false)
	expected := expectedParityCounts(t, manifest.Inventory)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("source parity multiplicity differs from manifest:\nactual: %#v\nexpected: %#v", actual, expected)
	}

	t.Logf("Go: %d functions, %d if statements; Starlark: %d functions, %d if/elif branches; paired IDs: %d; structural IDs: Go %d, Starlark %d",
		countKind(goItems, "function"), countKind(goItems, "branch"),
		countKind(starlarkItems, "function"), countKind(starlarkItems, "branch"),
		len(manifest.Inventory.Paired), len(manifest.Inventory.GoStructural), len(manifest.Inventory.StarlarkStructural))
}

func countItems(t *testing.T, counts map[parityKey]parityCounts, items []parityItem, isGo bool) {
	t.Helper()
	for _, item := range items {
		if item.AnnotationKind != item.Kind {
			t.Fatalf("%s annotation at line %d has kind %q on a %s", item.ID, item.Line, item.AnnotationKind, item.Kind)
		}
		key := parityKey{ID: item.ID, Kind: item.Kind}
		count := counts[key]
		if isGo {
			count.Go++
		} else {
			count.Starlark++
		}
		counts[key] = count
	}
}

func expectedParityCounts(t *testing.T, inventory parityInventory) map[parityKey]parityCounts {
	t.Helper()
	expected := make(map[parityKey]parityCounts)
	for _, id := range inventory.Paired {
		key := parityKey{ID: id, Kind: parityKind(t, id)}
		if _, exists := expected[key]; exists {
			t.Fatalf("manifest repeats paired ID %s", id)
		}
		expected[key] = parityCounts{Go: 1, Starlark: 1}
	}
	for _, fanOut := range inventory.FanOut {
		key := parityKey{ID: fanOut.ID, Kind: fanOut.Kind}
		if _, paired := expected[key]; !paired {
			t.Fatalf("fan-out %s/%s is not listed as paired", fanOut.ID, fanOut.Kind)
		}
		if fanOut.Reason == "" || fanOut.Go < 1 || fanOut.Starlark < 1 || fanOut.Go == fanOut.Starlark {
			t.Fatalf("invalid fan-out declaration: %#v", fanOut)
		}
		expected[key] = parityCounts{Go: fanOut.Go, Starlark: fanOut.Starlark}
	}
	for _, id := range inventory.GoStructural {
		key := parityKey{ID: id, Kind: parityKind(t, id)}
		if _, exists := expected[key]; exists {
			t.Fatalf("Go-structural ID %s is already categorized", id)
		}
		expected[key] = parityCounts{Go: 1}
	}
	for _, id := range inventory.StarlarkStructural {
		key := parityKey{ID: id, Kind: parityKind(t, id)}
		if _, exists := expected[key]; exists {
			t.Fatalf("Starlark-structural ID %s is already categorized", id)
		}
		expected[key] = parityCounts{Starlark: 1}
	}
	return expected
}

func parityKind(t *testing.T, id string) string {
	t.Helper()
	switch {
	case strings.Contains(id, "-F-"):
		return "function"
	case strings.Contains(id, "-B-"):
		return "branch"
	default:
		t.Fatalf("parity ID %s does not encode function/branch kind", id)
		return ""
	}
}

func readParityManifest(t *testing.T) parityManifest {
	t.Helper()
	data, err := os.ReadFile("parity-manifest.json")
	if err != nil {
		t.Fatalf("read parity manifest: %v", err)
	}
	var manifest parityManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode parity manifest: %v", err)
	}
	if manifest.Schema != "chess-raiders-standard-bot-parity/v1" {
		t.Fatalf("parity manifest schema = %q", manifest.Schema)
	}
	return manifest
}

func goParityItems(t *testing.T) []parityItem {
	t.Helper()
	fset := token.NewFileSet()
	var items []parityItem
	for _, name := range strategyGoFiles {
		path := filepath.Join(".", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		lines := strings.Split(string(data), "\n")
		file, err := parser.ParseFile(fset, path, data, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			line := fset.Position(fn.Pos()).Line
			items = append(items, requiredParityItem(t, path, lines, line-1, line-1, "function", line))
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				branch, ok := node.(*ast.IfStmt)
				if !ok {
					return true
				}
				start := fset.Position(branch.Pos()).Line
				end := fset.Position(branch.Body.Lbrace).Line
				items = append(items, requiredParityItem(t, path, lines, start, end, "branch", start))
				return true
			})
		}
	}
	return items
}

func starlarkParityItems(t *testing.T) []parityItem {
	t.Helper()
	const path = "chess-raiders-bot.star"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	var items []parityItem
	for index, line := range lines {
		lineNumber := index + 1
		if starlarkFunctionPattern.MatchString(line) {
			items = append(items, requiredParityItem(t, path, lines, lineNumber-1, lineNumber-1, "function", lineNumber))
		}
		if starlarkBranchPattern.MatchString(line) {
			items = append(items, requiredParityItem(t, path, lines, lineNumber, lineNumber, "branch", lineNumber))
		}
	}
	return items
}

func requiredParityItem(t *testing.T, path string, lines []string, first, last int, kind string, sourceLine int) parityItem {
	t.Helper()
	if first < 1 || last > len(lines) || first > last {
		t.Fatalf("invalid annotation range %s:%d-%d", path, first, last)
	}
	for line := first; line <= last; line++ {
		if match := parityIDPattern.FindStringSubmatch(lines[line-1]); match != nil {
			return parityItem{ID: match[2], Kind: kind, AnnotationKind: strings.ToLower(match[1]), Line: sourceLine}
		}
	}
	t.Fatalf("%s:%d has no parity annotation", path, first)
	return parityItem{}
}

func countKind(items []parityItem, kind string) int {
	count := 0
	for _, item := range items {
		if item.Kind == kind {
			count++
		}
	}
	return count
}
