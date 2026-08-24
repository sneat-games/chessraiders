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
	"sort"
	"strings"
	"testing"
)

type parityManifest struct {
	Schema    string          `json:"schema"`
	Inventory parityInventory `json:"inventory"`
}

type parityInventory struct {
	Paired             []string `json:"paired"`
	GoStructural       []string `json:"goStructural"`
	StarlarkStructural []string `json:"starlarkStructural"`
}

type parityItem struct {
	ID   string
	Kind string
	Line int
}

var parityIDPattern = regexp.MustCompile(`PARITY-(?:FUNCTION|BRANCH): (SBP-[A-Z0-9-]+)`)
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

	goIDs := itemIDSet(goItems)
	starlarkIDs := itemIDSet(starlarkItems)
	paired := stringSet(manifest.Inventory.Paired)
	goStructural := stringSet(manifest.Inventory.GoStructural)
	starlarkStructural := stringSet(manifest.Inventory.StarlarkStructural)

	if overlap := intersect(goStructural, starlarkStructural); len(overlap) != 0 {
		t.Fatalf("structural IDs cannot belong to both implementations: %v", overlap)
	}
	if overlap := intersect(paired, goStructural); len(overlap) != 0 {
		t.Fatalf("paired and Go-structural IDs overlap: %v", overlap)
	}
	if overlap := intersect(paired, starlarkStructural); len(overlap) != 0 {
		t.Fatalf("paired and Starlark-structural IDs overlap: %v", overlap)
	}

	if got, want := difference(goIDs, goStructural), paired; !reflect.DeepEqual(got, want) {
		t.Fatalf("Go paired IDs differ from manifest; got %v, want %v", got, want)
	}
	if got, want := difference(starlarkIDs, starlarkStructural), paired; !reflect.DeepEqual(got, want) {
		t.Fatalf("Starlark paired IDs differ from manifest; got %v, want %v", got, want)
	}
	if got := intersect(goIDs, goStructural); !reflect.DeepEqual(got, goStructural) {
		t.Fatalf("manifest names absent Go-structural IDs: %v", difference(goStructural, goIDs))
	}
	if got := intersect(starlarkIDs, starlarkStructural); !reflect.DeepEqual(got, starlarkStructural) {
		t.Fatalf("manifest names absent Starlark-structural IDs: %v", difference(starlarkStructural, starlarkIDs))
	}

	t.Logf("Go: %d functions, %d if statements; Starlark: %d functions, %d if/elif branches; paired IDs: %d; structural IDs: Go %d, Starlark %d",
		countKind(goItems, "function"), countKind(goItems, "branch"),
		countKind(starlarkItems, "function"), countKind(starlarkItems, "branch"),
		len(paired), len(goStructural), len(starlarkStructural))
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
			items = append(items, parityItem{ID: requiredParityID(t, path, lines, line-1, line-1), Kind: "function", Line: line})
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				branch, ok := node.(*ast.IfStmt)
				if !ok {
					return true
				}
				start := fset.Position(branch.Pos()).Line
				end := fset.Position(branch.Body.Lbrace).Line
				items = append(items, parityItem{ID: requiredParityID(t, path, lines, start, end), Kind: "branch", Line: start})
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
			items = append(items, parityItem{ID: requiredParityID(t, path, lines, lineNumber-1, lineNumber-1), Kind: "function", Line: lineNumber})
		}
		if starlarkBranchPattern.MatchString(line) {
			items = append(items, parityItem{ID: requiredParityID(t, path, lines, lineNumber, lineNumber), Kind: "branch", Line: lineNumber})
		}
	}
	return items
}

func requiredParityID(t *testing.T, path string, lines []string, first, last int) string {
	t.Helper()
	if first < 1 || last > len(lines) || first > last {
		t.Fatalf("invalid annotation range %s:%d-%d", path, first, last)
	}
	for line := first; line <= last; line++ {
		if match := parityIDPattern.FindStringSubmatch(lines[line-1]); match != nil {
			return match[1]
		}
	}
	t.Fatalf("%s:%d has no parity annotation", path, first)
	return ""
}

func itemIDSet(items []parityItem) []string {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item.ID] = struct{}{}
	}
	return sortedSet(set)
}

func stringSet(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return sortedSet(set)
}

func intersect(left, right []string) []string {
	seen := make(map[string]bool, len(right))
	for _, value := range right {
		seen[value] = true
	}
	var result []string
	for _, value := range left {
		if seen[value] {
			result = append(result, value)
		}
	}
	return result
}

func difference(left, right []string) []string {
	seen := make(map[string]bool, len(right))
	for _, value := range right {
		seen[value] = true
	}
	var result []string
	for _, value := range left {
		if !seen[value] {
			result = append(result, value)
		}
	}
	return result
}

func sortedSet(set map[string]struct{}) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
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
