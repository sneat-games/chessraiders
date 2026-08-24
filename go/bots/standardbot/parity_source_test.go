// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

type parityManifest struct {
	Schema string   `json:"schema"`
	IDs    []string `json:"ids"`
}

var parityIDPattern = regexp.MustCompile(`(?m)^\s*(?://|#) PARITY-(?:FUNCTION|BRANCH): (SBP-[A-Z0-9-]+)$`)

// TestDecideParityAnnotationsPairGoAndStarlark keeps the stable cross-language
// decision-pipeline IDs machine checked. The JSON manifest is deliberately
// compact so the TypeScript implementation can consume or mirror it without
// parsing either source language.
func TestDecideParityAnnotationsPairGoAndStarlark(t *testing.T) {
	manifestBytes, err := os.ReadFile("parity-manifest.json")
	if err != nil {
		t.Fatalf("read parity manifest: %v", err)
	}
	var manifest parityManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode parity manifest: %v", err)
	}
	if manifest.Schema != "chess-raiders-standard-bot-parity/v1" {
		t.Fatalf("parity manifest schema = %q", manifest.Schema)
	}
	goIDs := parityIDs(t, "bot.go")
	starlarkIDs := parityIDs(t, "chess-raiders-bot.star")
	sort.Strings(manifest.IDs)
	if !reflect.DeepEqual(goIDs, manifest.IDs) {
		t.Fatalf("Go parity IDs = %v, want manifest %v", goIDs, manifest.IDs)
	}
	if !reflect.DeepEqual(starlarkIDs, manifest.IDs) {
		t.Fatalf("Starlark parity IDs = %v, want manifest %v", starlarkIDs, manifest.IDs)
	}
}

func parityIDs(t *testing.T, path string) []string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	matches := parityIDPattern.FindAllStringSubmatch(string(content), -1)
	ids := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		id := match[1]
		if seen[id] {
			t.Fatalf("%s repeats parity ID %s", path, id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		t.Fatalf("%s has no parity IDs", path)
	}
	sort.Strings(ids)
	return ids
}
