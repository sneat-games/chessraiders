// Copyright 2026 Sneat.app

package runtime

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// pinnedGoStarlarkNetVersion is the ONE go.starlark.net version this
// repository builds against — recorded here so that bumping it is a
// deliberate, reviewed change to THIS constant, never a transitive
// surprise from an unrelated `go get -u` (parent Feature's
// AC:single-interpreter-in-the-tree: "a test asserts that pinned version
// string so a bump is a deliberate reviewed change"). Update this string
// in the SAME commit that updates the root go.mod's go.starlark.net line.
const pinnedGoStarlarkNetVersion = "v0.0.0-20260708150628-5395d018f003"

// TestGoStarlarkNetVersionIsPinned reads THIS module's own go.mod
// (github.com/sneat-games/chessraiders/go, rooted at go/ — there is no
// separate go/bots/go.mod; bots/* is part of that single module) and fails
// if go.starlark.net does not appear exactly once, at exactly the version
// above. This is the module that OWNS the pin after the interpreter's move
// out of sneat-co/chessraiders — that repository's own go.mod may still
// name go.starlark.net too (indirectly, through this module), which is a
// property AC:single-interpreter-in-the-tree's own amendment accepts by
// design: one pinned version reachable from the tree, asserted here, in the
// module that owns it, not "appears in exactly one go.mod file".
func TestGoStarlarkNetVersionIsPinned(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not resolve this test file's path")
	}
	// this file: <moduleRoot>/bots/runtime/pinned_version_test.go
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	// Matches both go.mod require shapes: a `require (...)` block, where the
	// module sits on its own indented line (the private repository's larger
	// go.mod, with many requirements), and a single-line `require
	// go.starlark.net vX` statement (this module's own go.mod, `go mod tidy`'s
	// normal output for a module with only one or two requirements) — the
	// optional `require\s+` prefix is the one difference between the two.
	re := regexp.MustCompile(`(?m)^\s*(?:require\s+)?go\.starlark\.net\s+(\S+)`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	if len(matches) != 1 {
		t.Fatalf("go.mod names go.starlark.net %d time(s), want exactly 1 (matches: %v)", len(matches), matches)
	}
	if got := matches[0][1]; got != pinnedGoStarlarkNetVersion {
		t.Fatalf("go.mod pins go.starlark.net at %s, want %s — update pinnedGoStarlarkNetVersion in the same commit as a deliberate version bump", got, pinnedGoStarlarkNetVersion)
	}
}
