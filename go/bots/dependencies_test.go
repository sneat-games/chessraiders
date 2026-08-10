package bots_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// modulePath is this module's own import prefix. An import starting with it is
// intra-module and always allowed.
const modulePath = "github.com/sneat-games/chessraiders/go"

// allowedExternal is the complete set of third-party module prefixes anything
// under go/ may import. It is deliberately one entry long.
//
// go.starlark.net is the Starlark interpreter the runtime wraps, and it is the
// ONE implementation permitted anywhere in Chess Raiders — a second interpreter
// would mean a script could decide differently depending on which surface ran
// it, which is the drift this whole tree exists to remove.
//
// Adding an entry here is a real decision with a cost, spelled out in
// doc.go: every dependency is something a forker must also obtain. Do not add
// one to make a test compile.
var allowedExternal = []string{
	"go.starlark.net/",
}

// TestModuleDependsOnNothingButStdlibAndStarlark walks every Go file in the
// module — including test files, which are equally capable of dragging in a
// dependency — and fails on any import that is neither standard library, nor
// intra-module, nor an explicitly allowed external prefix.
//
// This runs before runtime/ and standardbot/ exist. That is intentional: an
// invariant added after the code it constrains has to be argued for against
// working code, while one that is already there is simply the rule.
func TestModuleDependsOnNothingButStdlibAndStarlark(t *testing.T) {
	root := moduleRoot(t)
	fileSet := token.NewFileSet()
	checked := 0

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		checked++
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relative = path
		}
		for _, spec := range parsed.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				t.Errorf("%s: unparsable import %s", relative, spec.Path.Value)
				continue
			}
			if permitted(importPath) {
				continue
			}
			t.Errorf("%s imports %q.\n"+
				"Nothing under go/ may import outside the standard library, this module, "+
				"and go.starlark.net — see bots/doc.go for why. If this dependency is "+
				"genuinely required, that is a decision to take deliberately and record, "+
				"not a line to add to allowedExternal so a build goes green.",
				relative, importPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if checked == 0 {
		t.Fatal("no Go files were examined — the walk is not reaching the module, " +
			"so this test would pass no matter what the module imported")
	}
	t.Logf("checked imports of %d Go file(s)", checked)
}

// permitted reports whether one import path is allowed.
func permitted(importPath string) bool {
	if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
		return true
	}
	for _, prefix := range allowedExternal {
		if strings.HasPrefix(importPath, prefix) {
			return true
		}
	}
	return isStandardLibrary(importPath)
}

// isStandardLibrary uses the same rule the go command does: a standard-library
// path's first segment contains no dot, because every module path outside it
// begins with a domain.
func isStandardLibrary(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

// TestTheDetectorActuallyRejects proves the check above can fail. Without it,
// a walk that silently examined nothing, or a permitted() that returned true
// for everything, would look exactly like a clean module.
func TestTheDetectorActuallyRejects(t *testing.T) {
	allowed := []string{
		"fmt", "os/exec", "encoding/json",
		"go.starlark.net/starlark",
		modulePath + "/bots/runtime",
		modulePath,
	}
	for _, importPath := range allowed {
		if !permitted(importPath) {
			t.Errorf("permitted(%q) = false, want true", importPath)
		}
	}

	rejected := []string{
		"github.com/sneat-co/chess/server-go/chess",
		"github.com/sneat-co/competios/backend/competioscontract",
		"github.com/sneat-games/chessraiders-other/thing",
		"cloud.google.com/go/firestore",
		"gopkg.in/yaml.v3",
	}
	for _, importPath := range rejected {
		if permitted(importPath) {
			t.Errorf("permitted(%q) = true, want false", importPath)
		}
	}
}

// moduleRoot returns the directory holding go.mod, found by walking up from
// this test file's own package directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolving working directory: %v", err)
	}
	for {
		if _, statErr := filepath.Glob(filepath.Join(dir, "go.mod")); statErr == nil {
			matches, _ := filepath.Glob(filepath.Join(dir, "go.mod"))
			if len(matches) == 1 {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the test's working directory")
		}
		dir = parent
	}
}
