// Copyright 2026 Sneat.app

package runtime

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// bannedEntryPoints is REQ:legacy-dialect-entry-points-are-banned's list,
// verified against the pinned go.starlark.net module version. Every one of
// these silently substitutes syntax.LegacyFileOptions() internally — the
// exact path decision 0007 found to no-op under TinyGo — and the "*Options"
// twin of each is the required form. Deliberately NOT a search for the
// string "resolve.Allow": that string already occurs in this repository's
// own doc comments (this file's, and canary.go's, and dialect.go's) in
// order to forbid it, and a textual search would flag the forbidding
// prose along with any real violation.
var bannedEntryPoints = map[string]map[string]bool{
	"go.starlark.net/starlark": {
		"ExecFile":      true,
		"SourceProgram": true,
		"Eval":          true,
		"EvalExpr":      true,
		"ExprFunc":      true,
	},
	"go.starlark.net/syntax": {
		"Parse":             true,
		"ParseCompoundStmt": true,
		"ParseExpr":         true,
		"LegacyFileOptions": true,
	},
	"go.starlark.net/resolve": {
		"File":      true,
		"REPLChunk": true,
		"Expr":      true,
	},
}

// dotImportedPaths returns the full import paths of every `import . "..."`
// dot import in file. A dot import merges every EXPORTED name from that
// package into this file's own identifier scope, so a call written as bare
// `ExecFile(...)` rather than `starlark.ExecFile(...)` carries no
// SelectorExpr at all — invisible to a selector-based scan entirely, which
// is exactly the gap this function exists to let callers close.
func dotImportedPaths(file *ast.File) []string {
	var paths []string
	for _, imp := range file.Imports {
		if imp.Name != nil && imp.Name.Name == "." {
			paths = append(paths, strings.Trim(imp.Path.Value, `"`))
		}
	}
	return paths
}

// bannedIdentifierViolations walks file's AST and returns one message per:
//   - a reference to a banned entry point, either qualified
//     (`pkg.ExecFile`, a SelectorExpr on an import alias bound to one of
//     bannedEntryPoints' three import paths) or bare (`ExecFile`, an Ident
//     reachable only because the file dot-imports that same path);
//   - a reference to a resolve.Allow* package-level global in ANY
//     expression position — not only as an assignment's LHS (`resolve.
//     AllowSet = true`), which is the one shape an earlier version of this
//     function checked: `p := &resolve.AllowSet; *p = true` mutates the
//     SAME global through an expression position (`&resolve.AllowSet`)
//     that check never looked at, and a bare read (`if resolve.AllowSet
//     {...}`) already means the legacy globals are being consulted at all,
//     which this package's own code has no legitimate reason to do EITHER
//     way — qualified or bare (dot-imported) alike;
//   - a `syntax.FileOptions{...}` (or bare `FileOptions{...}`, dot-
//     imported) composite literal constructed OUTSIDE dialect.go, in a
//     NON-test file. REQ:one-dialect-constant's whole point is that
//     dialect.go's DialectOptions is the only production construction —
//     test files are deliberately exempted (canary_test.go's own
//     TestRunCanaryFailsWithoutSet/Without While/WhenRecursionIsAllowed
//     construct alternate options ON PURPOSE, to prove the canary reacts
//     to a WRONG dialect; that is what those tests are for, not a
//     violation of "one value in production").
//
// importAliases maps a package's local identifier (usually its default
// name, or an explicit `import x "path"` alias) to its full import path.
// filename is the base name the caller parsed file from (a real path's
// base, or a synthetic name like "fake_test.go" for the synthetic-source
// tests below) — passed explicitly rather than recovered from file itself,
// since go/ast's *File does not carry its own filename as a plain string
// (only as a position resolvable through the *token.FileSet that produced
// it), and threading that FileSet through just for this one lookup would
// be more machinery than the two checks below (isTestFile, isDialectGo)
// are worth.
func bannedIdentifierViolations(file *ast.File, importAliases map[string]string, filename string) []string {
	var violations []string

	isTestFile := strings.HasSuffix(filename, "_test.go")
	isDialectGo := filename == "dialect.go"

	dotImports := dotImportedPaths(file)
	bannedBareNames := map[string]string{} // bare identifier -> its dot-imported path, for the message
	dotImportsResolve := false
	dotImportsSyntax := false
	for _, importPath := range dotImports {
		for name := range bannedEntryPoints[importPath] {
			bannedBareNames[name] = importPath
		}
		if importPath == "go.starlark.net/resolve" {
			dotImportsResolve = true
		}
		if importPath == "go.starlark.net/syntax" {
			dotImportsSyntax = true
		}
	}

	reportSelector := func(sel *ast.SelectorExpr) {
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return
		}
		importPath, ok := importAliases[ident.Name]
		if !ok {
			return
		}
		if importPath == "go.starlark.net/resolve" && strings.HasPrefix(sel.Sel.Name, "Allow") {
			violations = append(violations, fmt.Sprintf(
				"reference to %s.%s: resolve.Allow* globals are banned in any expression position, not only as an assignment target — they are what TinyGo's linkname handling silently no-ops",
				ident.Name, sel.Sel.Name))
			return
		}
		if names, ok := bannedEntryPoints[importPath]; ok && names[sel.Sel.Name] {
			violations = append(violations, fmt.Sprintf(
				"%s.%s is a banned entry point (substitutes syntax.LegacyFileOptions() internally) — use its *Options twin instead",
				ident.Name, sel.Sel.Name))
		}
	}

	reportBareIdent := func(ident *ast.Ident) {
		if importPath, ok := bannedBareNames[ident.Name]; ok {
			violations = append(violations, fmt.Sprintf(
				"%s is a banned entry point dot-imported from %s (substitutes syntax.LegacyFileOptions() internally) — use its *Options twin instead",
				ident.Name, importPath))
			return
		}
		if dotImportsResolve && strings.HasPrefix(ident.Name, "Allow") {
			violations = append(violations, fmt.Sprintf(
				"reference to %s: resolve.Allow* globals (dot-imported from go.starlark.net/resolve) are banned in any expression position — they are what TinyGo's linkname handling silently no-ops",
				ident.Name))
		}
	}

	// isFileOptionsType reports whether expr names syntax.FileOptions,
	// either qualified (via importAliases) or bare (dot-imported). A
	// composite literal's own Type field is never a pointer expression
	// even for `&syntax.FileOptions{...}` — the `&` is the SURROUNDING
	// ast.UnaryExpr, not part of the literal's Type — so this only ever
	// needs to recognise the two forms below, no pointer-dereferencing
	// case included.
	isFileOptionsType := func(expr ast.Expr) bool {
		switch t := expr.(type) {
		case *ast.SelectorExpr:
			ident, ok := t.X.(*ast.Ident)
			if !ok {
				return false
			}
			return importAliases[ident.Name] == "go.starlark.net/syntax" && t.Sel.Name == "FileOptions"
		case *ast.Ident:
			return dotImportsSyntax && t.Name == "FileOptions"
		}
		return false
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			reportSelector(node)
		case *ast.Ident:
			reportBareIdent(node)
		case *ast.CompositeLit:
			if node.Type != nil && isFileOptionsType(node.Type) && !isDialectGo && !isTestFile {
				violations = append(violations, "syntax.FileOptions{...} constructed outside dialect.go — REQ:one-dialect-constant requires exactly one production value, dialect.go's own DialectOptions")
			}
		}
		return true
	})

	return violations
}

// fileImportAliases builds the local-identifier -> import-path map
// bannedIdentifierViolations needs, from one parsed file's own import
// spec list.
func fileImportAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		name := path.Base(importPath)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		aliases[name] = importPath
	}
	return aliases
}

// scanTreeForBannedIdentifiers walks every .go file under root (recursing
// into subdirectories) and returns one message per violation, prefixed
// with the file's path relative to root. A bare go/parser.ParseFile does
// not consult build tags at all, which is a FEATURE here, not a gap: a
// file gated by `//go:build js && wasm` (server-go/cmd/chesswasm/main.go,
// bots_entry.go, bots_entry_nobots.go) is parsed and scanned exactly like
// any other .go file — the blind spot this walk exists to close is a
// `go build`/`go list`-based check silently skipping such a file under the
// host's own GOOS, never something a plain AST parse was ever subject to.
func scanTreeForBannedIdentifiers(root string) ([]string, error) {
	fset := token.NewFileSet()
	var violations []string
	err := filepath.WalkDir(root, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(fullPath, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, fullPath, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parser.ParseFile(%s): %w", fullPath, err)
		}
		rel, relErr := filepath.Rel(root, fullPath)
		if relErr != nil {
			rel = fullPath
		}
		for _, v := range bannedIdentifierViolations(file, fileImportAliases(file), filepath.Base(fullPath)) {
			violations = append(violations, rel+": "+v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(violations)
	return violations, nil
}

// moduleRoot returns the directory holding this module's own go.mod, found
// by walking up from this test file's own path — the same technique
// pinned_version_test.go and dependencies_test.go use, kept local to this
// file (rather than a shared helper) because the private repository's own
// import_boundary_test.go, which used to provide the equivalent repoRoot
// helper for this package before the move, does not travel with it.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not resolve this test file's path")
	}
	// this file: <moduleRoot>/bots/runtime/dialect_lint_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// TestNoLegacyDialectEntryPoints is REQ:legacy-dialect-entry-points-are-
// banned's own-source half: every .go file under this module's own root
// (production and test alike, EVERY package, not just this one) is parsed
// and walked for a banned identifier. This is the check AC:the-legacy-
// entry-points-cannot-be-called requires to fail a scratch commit that
// rewrites starlark.ExecFileOptions to starlark.ExecFile —
// TestBannedIdentifierDetectorCatchesTheScratchCommit below proves the
// detector itself does that, without needing an actual scratch commit.
//
// WALKS THE WHOLE MODULE, NOT os.Getwd() — mirroring the same widening this
// check went through in its original, private-repository home (where
// scoping the walk to only this package's own directory once missed a
// legacy entry point that had landed in a different package entirely — see
// TestBannedIdentifierDetectorCatchesAScratchCommitOutsideThisPackage below
// for the proof that a nested package is still reached). The exemption for
// calling go.starlark.net at all remains this one package; the private
// module that now consumes this one keeps its own equivalent sweep,
// server-go/importboundary, over ITS OWN tree — see this package's own
// doc.go for how the two checks divide the work.
func TestNoLegacyDialectEntryPoints(t *testing.T) {
	root := moduleRoot(t)
	violations, err := scanTreeForBannedIdentifiers(root)
	if err != nil {
		t.Fatalf("scanTreeForBannedIdentifiers(%s): %v", root, err)
	}
	if len(violations) > 0 {
		t.Fatalf("banned go.starlark.net entry point(s) found under this module:\n%s",
			strings.Join(violations, "\n"))
	}
}

// TestBannedIdentifierDetectorCatchesAScratchCommitOutsideThisPackage is
// the walk-reaches-outside-the-package proof TestNoLegacyDialectEntryPoints
// itself now depends on: a banned identifier in a file OUTSIDE
// server-go/starlarkbot (in a nested subdirectory, mirroring a
// server-go/cmd/<command> command's own real depth — "botswasm" below is
// the historical name of the command this actually happened in, since
// folded into server-go/cmd/chesswasm's own bots_entry.go) must still be
// caught. Written against a scratch temp directory tree rather than the
// real repository — dropping a real file under server-go/cmd/chesswasm to
// prove this would leave the tree dirty for every OTHER test run in the
// same process.
func TestBannedIdentifierDetectorCatchesAScratchCommitOutsideThisPackage(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "cmd", "botswasm")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", nested, err)
	}
	const violatingSource = `package main

import "go.starlark.net/starlark"

func run() {
	starlark.ExecFile(nil, "f.star", "", nil)
}
`
	if err := os.WriteFile(filepath.Join(nested, "main.go"), []byte(violatingSource), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// A clean sibling package, proving the walk does not flag files that
	// have nothing to do with the violation just because they share a
	// root.
	cleanNested := filepath.Join(root, "cmd", "chesswasm")
	if err := os.MkdirAll(cleanNested, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", cleanNested, err)
	}
	const cleanSource = `package main

func run() {}
`
	if err := os.WriteFile(filepath.Join(cleanNested, "main.go"), []byte(cleanSource), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	violations, err := scanTreeForBannedIdentifiers(root)
	if err != nil {
		t.Fatalf("scanTreeForBannedIdentifiers(%s): %v", root, err)
	}
	if len(violations) != 1 {
		t.Fatalf("scanTreeForBannedIdentifiers found %d violation(s), want exactly 1: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], filepath.Join("cmd", "botswasm", "main.go")) {
		t.Errorf("violation %q does not name the nested file it was found in", violations[0])
	}
}

// TestBannedIdentifierDetectorCatchesTheScratchCommit is
// AC:the-legacy-entry-points-cannot-be-called's proof that the detector
// above actually catches the scratch commit the AC names — "rewrites one
// runtime call site from starlark.ExecFileOptions to starlark.ExecFile" —
// and does not flag the unmodified, compliant form. Exercised against
// synthetic source rather than a real scratch commit, since a real one
// would have to live somewhere in this repository to be compiled at all.
func TestBannedIdentifierDetectorCatchesTheScratchCommit(t *testing.T) {
	const violatingSource = `package fake

import "go.starlark.net/starlark"

func run() {
	starlark.ExecFile(nil, "f.star", "", nil)
}
`
	const compliantSource = `package fake

import "go.starlark.net/starlark"

func run() {
	starlark.ExecFileOptions(nil, nil, "f.star", "", nil)
}
`
	fset := token.NewFileSet()

	violatingFile, err := parser.ParseFile(fset, "violating.go", violatingSource, 0)
	if err != nil {
		t.Fatalf("parse violating source: %v", err)
	}
	if got := bannedIdentifierViolations(violatingFile, fileImportAliases(violatingFile), "violating.go"); len(got) == 0 {
		t.Fatalf("bannedIdentifierViolations did not flag starlark.ExecFile")
	}

	compliantFile, err := parser.ParseFile(fset, "compliant.go", compliantSource, 0)
	if err != nil {
		t.Fatalf("parse compliant source: %v", err)
	}
	if got := bannedIdentifierViolations(compliantFile, fileImportAliases(compliantFile), "compliant.go"); len(got) != 0 {
		t.Fatalf("bannedIdentifierViolations flagged the compliant starlark.ExecFileOptions call: %v", got)
	}
}

// TestBannedIdentifierDetectorCatchesAllowAssignment is the second half of
// the ban: "any assignment to a resolve.Allow* package global" — never
// reachable from this package's own real source (which never imports
// go.starlark.net/resolve at all), so it is proven against synthetic
// source too.
func TestBannedIdentifierDetectorCatchesAllowAssignment(t *testing.T) {
	const source = `package fake

import "go.starlark.net/resolve"

func bad() {
	resolve.AllowSet = true
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bad.go", source, 0)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	if got := bannedIdentifierViolations(file, fileImportAliases(file), "bad.go"); len(got) == 0 {
		t.Fatalf("bannedIdentifierViolations did not flag assignment to resolve.AllowSet")
	}
}

// TestBannedIdentifierDetectorCatchesAllowInAnyExpressionPosition is
// A3's proven hole: "resolve.Allow* in any expression position other than
// an assignment LHS" — here, taking its address and mutating it through a
// pointer, which never appears as an AssignStmt.Lhs selector at all, and a
// bare READ in an if-condition, which is not an assignment in any form.
func TestBannedIdentifierDetectorCatchesAllowInAnyExpressionPosition(t *testing.T) {
	const viaPointer = `package fake

import "go.starlark.net/resolve"

func bad() {
	p := &resolve.AllowSet
	*p = true
}
`
	const viaRead = `package fake

import "go.starlark.net/resolve"

func bad() bool {
	if resolve.AllowSet {
		return true
	}
	return false
}
`
	fset := token.NewFileSet()
	for name, source := range map[string]string{"viaPointer.go": viaPointer, "viaRead.go": viaRead} {
		file, err := parser.ParseFile(fset, name, source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if got := bannedIdentifierViolations(file, fileImportAliases(file), name); len(got) == 0 {
			t.Errorf("bannedIdentifierViolations(%s) did not flag a resolve.AllowSet reference outside an assignment LHS", name)
		}
	}
}

// TestBannedIdentifierDetectorCatchesDotImportedEntryPoint is A3's other
// proven hole: "a dot-import of the three packages" makes a banned call
// unqualified (`ExecFile(...)`, no `starlark.` prefix at all), invisible to
// a selector-based scan. Also covers the resolve.Allow* half of the same
// bypass — a dot-imported bare `AllowSet` reference.
func TestBannedIdentifierDetectorCatchesDotImportedEntryPoint(t *testing.T) {
	const viaDotImportedEntryPoint = `package fake

import . "go.starlark.net/starlark"

func run() {
	ExecFile(nil, "f.star", "", nil)
}
`
	const viaDotImportedAllow = `package fake

import . "go.starlark.net/resolve"

func bad() {
	AllowSet = true
}
`
	const compliantDotImport = `package fake

import . "go.starlark.net/starlark"

func run() {
	ExecFileOptions(nil, nil, "f.star", "", nil)
}
`
	fset := token.NewFileSet()
	for name, source := range map[string]string{
		"viaDotImportedEntryPoint.go": viaDotImportedEntryPoint,
		"viaDotImportedAllow.go":      viaDotImportedAllow,
	} {
		file, err := parser.ParseFile(fset, name, source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if got := bannedIdentifierViolations(file, fileImportAliases(file), name); len(got) == 0 {
			t.Errorf("bannedIdentifierViolations(%s) did not flag the dot-imported bare reference", name)
		}
	}

	compliantFile, err := parser.ParseFile(fset, "compliantDotImport.go", compliantDotImport, 0)
	if err != nil {
		t.Fatalf("parse compliantDotImport.go: %v", err)
	}
	if got := bannedIdentifierViolations(compliantFile, fileImportAliases(compliantFile), "compliantDotImport.go"); len(got) != 0 {
		t.Fatalf("bannedIdentifierViolations flagged the compliant dot-imported ExecFileOptions call: %v", got)
	}
}

// TestBannedIdentifierDetectorCatchesFileOptionsOutsideDialectGo is A3's
// third proven hole: "a syntax.FileOptions composite literal outside
// dialect.go" — AC:one-options-value-in-the-tree ("exactly one
// syntax.FileOptions value exists ... it is exported from the runtime
// package") is, as the task brief notes, "currently enforced by nothing".
// A second production construction (in a file other than dialect.go) must
// be caught; a TEST file's own alternate-options construction (exactly
// what canary_test.go's TestRunCanaryFailsWithoutSet and its siblings do,
// on purpose, to prove the canary reacts to a WRONG dialect) must NOT be —
// proven here against a synthetic non-test file and a synthetic test file
// with otherwise-identical content, so the exemption is shown to be about
// the file's role, not a loophole that swallows every violation.
func TestBannedIdentifierDetectorCatchesFileOptionsOutsideDialectGo(t *testing.T) {
	const source = `package fake

import "go.starlark.net/syntax"

var secondOptions = &syntax.FileOptions{Set: true}
`
	fset := token.NewFileSet()

	productionFile, err := parser.ParseFile(fset, "notdialect.go", source, 0)
	if err != nil {
		t.Fatalf("parse notdialect.go: %v", err)
	}
	if got := bannedIdentifierViolations(productionFile, fileImportAliases(productionFile), "notdialect.go"); len(got) == 0 {
		t.Fatal("bannedIdentifierViolations did not flag a second syntax.FileOptions{...} construction in a non-test, non-dialect.go file")
	}

	dialectFile, err := parser.ParseFile(fset, "dialect.go", source, 0)
	if err != nil {
		t.Fatalf("parse dialect.go: %v", err)
	}
	if got := bannedIdentifierViolations(dialectFile, fileImportAliases(dialectFile), "dialect.go"); len(got) != 0 {
		t.Fatalf("bannedIdentifierViolations flagged dialect.go's own construction: %v", got)
	}

	testFile, err := parser.ParseFile(fset, "probe_test.go", source, 0)
	if err != nil {
		t.Fatalf("parse probe_test.go: %v", err)
	}
	if got := bannedIdentifierViolations(testFile, fileImportAliases(testFile), "probe_test.go"); len(got) != 0 {
		t.Fatalf("bannedIdentifierViolations flagged a _test.go file's own alternate-options construction, exactly what canary_test.go's own tests legitimately do: %v", got)
	}
}
