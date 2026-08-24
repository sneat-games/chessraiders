// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"fmt"
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

const semanticLedgerPath = "parity-semantic-ledger.json"

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
	Anchor         paritySourceAnchor
}

type paritySourceAnchor struct {
	File     string `json:"file"`
	Function string `json:"function"`
	Source   string `json:"source"`
}

type semanticBinding struct {
	ID       string               `json:"id"`
	Kind     string               `json:"kind"`
	Go       []paritySourceAnchor `json:"go"`
	Starlark []paritySourceAnchor `json:"starlark"`
}

type semanticLedger struct {
	Schema   string            `json:"schema"`
	Bindings []semanticBinding `json:"bindings"`
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
var starlarkFunctionPattern = regexp.MustCompile(`^def\s+([A-Za-z0-9_]+)\(`)
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
		t.Fatal(parityCountDiff(actual, expected))
	}

	bindings := semanticBindingsFromItems(t, manifest.Inventory, goItems, starlarkItems)
	if os.Getenv("UPDATE_PARITY_SEMANTIC_LEDGER") == "1" {
		writeSemanticLedger(t, semanticLedger{
			Schema:   "chess-raiders-standard-bot-semantic-parity/v1",
			Bindings: bindings,
		})
		return
	}
	if err := validateSemanticLedger(manifest.Inventory, readSemanticLedger(t), bindings); err != nil {
		t.Fatal(err)
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
	if err := validateFanOutDeclarations(inventory.FanOut); err != nil {
		t.Fatal(err)
	}
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

func validateFanOutDeclarations(fanOuts []parityMultiplicity) error {
	seen := make(map[parityKey]bool, len(fanOuts))
	for _, fanOut := range fanOuts {
		key := parityKey{ID: fanOut.ID, Kind: fanOut.Kind}
		if seen[key] {
			return fmt.Errorf("duplicate fan-out declaration for %s/%s", fanOut.ID, fanOut.Kind)
		}
		seen[key] = true
	}
	return nil
}

func TestDuplicateFanOutDeclarationsAreRejected(t *testing.T) {
	declaration := parityMultiplicity{
		ID:       "SBP-F-DUPLICATE",
		Kind:     "function",
		Go:       2,
		Starlark: 1,
		Reason:   "fixture",
	}
	err := validateFanOutDeclarations([]parityMultiplicity{declaration, declaration})
	if err == nil {
		t.Fatal("duplicate fan-out declarations were accepted")
	}
	if !strings.Contains(err.Error(), "duplicate fan-out declaration") {
		t.Fatalf("duplicate fan-out error = %v", err)
	}
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
			item := requiredParityItem(t, path, lines, line-1, line-1, "function", line)
			item.Anchor = paritySourceAnchor{File: name, Function: fn.Name.Name, Source: sourceBetween(data, fset, fn.Pos(), fn.Body.Lbrace)}
			items = append(items, item)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				branch, ok := node.(*ast.IfStmt)
				if !ok {
					return true
				}
				start := fset.Position(branch.Pos()).Line
				end := fset.Position(branch.Body.Lbrace).Line
				item := requiredParityItem(t, path, lines, start, end, "branch", start)
				item.Anchor = paritySourceAnchor{File: name, Function: fn.Name.Name, Source: sourceBetween(data, fset, branch.Cond.Pos(), branch.Cond.End())}
				items = append(items, item)
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
	function := ""
	for index, line := range lines {
		lineNumber := index + 1
		if match := starlarkFunctionPattern.FindStringSubmatch(line); match != nil {
			function = match[1]
			item := requiredParityItem(t, path, lines, lineNumber-1, lineNumber-1, "function", lineNumber)
			item.Anchor = paritySourceAnchor{File: path, Function: function, Source: normalizeSource(line)}
			items = append(items, item)
		}
		if starlarkBranchPattern.MatchString(line) {
			end, source := starlarkBranchSource(lines, index)
			item := requiredParityItem(t, path, lines, lineNumber, end+1, "branch", lineNumber)
			item.Anchor = paritySourceAnchor{File: path, Function: function, Source: source}
			items = append(items, item)
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

func parityCountDiff(actual, expected map[parityKey]parityCounts) string {
	keys := make(map[parityKey]bool, len(actual)+len(expected))
	for key := range actual {
		keys[key] = true
	}
	for key := range expected {
		keys[key] = true
	}
	ordered := make([]parityKey, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ID != ordered[j].ID {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Kind < ordered[j].Kind
	})
	var lines []string
	for _, key := range ordered {
		if actual[key] != expected[key] {
			lines = append(lines, fmt.Sprintf("%s/%s: actual=%+v expected=%+v", key.ID, key.Kind, actual[key], expected[key]))
		}
	}
	return "source parity multiplicity differs from manifest:\n" + strings.Join(lines, "\n")
}

func sourceBetween(data []byte, fset *token.FileSet, start, end token.Pos) string {
	from := fset.PositionFor(start, false).Offset
	to := fset.PositionFor(end, false).Offset
	return normalizeSource(string(data[from:to]))
}

func starlarkBranchSource(lines []string, start int) (int, string) {
	parts := make([]string, 0, 2)
	for end := start; end < len(lines); end++ {
		line := strings.TrimSpace(strings.SplitN(lines[end], "#", 2)[0])
		parts = append(parts, line)
		if strings.HasSuffix(line, ":") {
			source := strings.TrimSpace(strings.TrimSuffix(normalizeSource(strings.Join(parts, " ")), ":"))
			source = strings.TrimPrefix(strings.TrimPrefix(source, "if "), "elif ")
			return end, source
		}
	}
	return len(lines) - 1, normalizeSource(strings.Join(parts, " "))
}

func normalizeSource(source string) string {
	return strings.Join(strings.Fields(source), " ")
}

func semanticBindingsFromItems(t *testing.T, inventory parityInventory, goItems, starlarkItems []parityItem) []semanticBinding {
	t.Helper()
	bindings := make(map[string]*semanticBinding, len(inventory.Paired))
	for _, id := range inventory.Paired {
		bindings[id] = &semanticBinding{ID: id, Kind: parityKind(t, id)}
	}
	for _, item := range goItems {
		if binding := bindings[item.ID]; binding != nil {
			binding.Go = append(binding.Go, item.Anchor)
		}
	}
	for _, item := range starlarkItems {
		if binding := bindings[item.ID]; binding != nil {
			binding.Starlark = append(binding.Starlark, item.Anchor)
		}
	}
	result := make([]semanticBinding, 0, len(bindings))
	for _, binding := range bindings {
		sortAnchors(binding.Go)
		sortAnchors(binding.Starlark)
		result = append(result, *binding)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func sortAnchors(anchors []paritySourceAnchor) {
	sort.Slice(anchors, func(i, j int) bool {
		if anchors[i].File != anchors[j].File {
			return anchors[i].File < anchors[j].File
		}
		if anchors[i].Function != anchors[j].Function {
			return anchors[i].Function < anchors[j].Function
		}
		return anchors[i].Source < anchors[j].Source
	})
}

func readSemanticLedger(t *testing.T) semanticLedger {
	t.Helper()
	data, err := os.ReadFile(semanticLedgerPath)
	if err != nil {
		t.Fatalf("read semantic ledger: %v; regenerate with UPDATE_PARITY_SEMANTIC_LEDGER=1", err)
	}
	var ledger semanticLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("decode semantic ledger: %v", err)
	}
	return ledger
}

func writeSemanticLedger(t *testing.T, ledger semanticLedger) {
	t.Helper()
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		t.Fatalf("encode semantic ledger: %v", err)
	}
	if err := os.WriteFile(semanticLedgerPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write semantic ledger: %v", err)
	}
}

func validateSemanticLedger(inventory parityInventory, ledger semanticLedger, actual []semanticBinding) error {
	if ledger.Schema != "chess-raiders-standard-bot-semantic-parity/v1" {
		return fmt.Errorf("semantic ledger schema = %q", ledger.Schema)
	}
	if len(ledger.Bindings) != len(inventory.Paired) || len(actual) != len(inventory.Paired) {
		return fmt.Errorf("semantic binding count: ledger=%d actual=%d paired=%d", len(ledger.Bindings), len(actual), len(inventory.Paired))
	}
	for index, id := range inventory.Paired {
		if ledger.Bindings[index].ID != id || actual[index].ID != id {
			return fmt.Errorf("semantic binding order at %d: ledger=%s actual=%s expected=%s", index, ledger.Bindings[index].ID, actual[index].ID, id)
		}
		if !reflect.DeepEqual(ledger.Bindings[index], actual[index]) {
			return fmt.Errorf("semantic binding %s differs:\nledger: %#v\nactual: %#v", id, ledger.Bindings[index], actual[index])
		}
	}
	return nil
}

func TestSemanticLedgerRejectsSelfConsistentDisplacement(t *testing.T) {
	inventory := parityInventory{Paired: []string{"SBP-B-FIRST", "SBP-B-SECOND"}}
	ledger := semanticLedger{Schema: "chess-raiders-standard-bot-semantic-parity/v1", Bindings: []semanticBinding{
		{ID: "SBP-B-FIRST", Kind: "branch", Go: []paritySourceAnchor{{Source: "first"}}},
		{ID: "SBP-B-SECOND", Kind: "branch", Go: []paritySourceAnchor{{Source: "second"}}},
	}}
	displaced := []semanticBinding{
		{ID: "SBP-B-FIRST", Kind: "branch", Go: []paritySourceAnchor{{Source: "second"}}},
		{ID: "SBP-B-SECOND", Kind: "branch", Go: []paritySourceAnchor{{Source: "first"}}},
	}
	if err := validateSemanticLedger(inventory, ledger, displaced); err == nil || !strings.Contains(err.Error(), "semantic binding") {
		t.Fatalf("self-consistent displacement error = %v", err)
	}
}

func TestSemanticLedgerRejectsMissingAndReorderedBindings(t *testing.T) {
	inventory := parityInventory{Paired: []string{"SBP-B-FIRST", "SBP-B-SECOND"}}
	ledger := semanticLedger{Schema: "chess-raiders-standard-bot-semantic-parity/v1", Bindings: []semanticBinding{
		{ID: "SBP-B-FIRST", Kind: "branch"},
		{ID: "SBP-B-SECOND", Kind: "branch"},
	}}
	if err := validateSemanticLedger(inventory, ledger, ledger.Bindings[:1]); err == nil || !strings.Contains(err.Error(), "count") {
		t.Fatalf("missing binding error = %v", err)
	}
	if err := validateSemanticLedger(inventory, ledger, []semanticBinding{ledger.Bindings[1], ledger.Bindings[0]}); err == nil || !strings.Contains(err.Error(), "order") {
		t.Fatalf("reordered binding error = %v", err)
	}
}
