// Copyright 2026 Sneat.app

package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const validManifest = `{
  "schema":"chess-raiders-bot-manifest/v1",
  "display":{"name":"Test bot","description":"A strict fixture."},
  "author":{"name":"Test author"},
  "source":{"repository":"example.test/bot","attribution":"Test author"},
  "license":{"spdx":"Apache-2.0"},
  "compatibility":{"game":"chess-raiders","rules":{"minimum":"chess-raiders-rules/v1","maximum":"chess-raiders-rules/v1"},"runtime":{"minimum":"chess-raiders-starlark-runtime/v1","maximum":"chess-raiders-starlark-runtime/v1"}},
  "runtime":{"kind":"starlark","protocol":"chess-raiders-bot-script/v1","entrypoint":"bot.star"},
  "stateMode":"stateless",
  "parameters":{"path":"parameters.json","declarations":[{"name":"search","type":"integer","default":4,"range":{"minimum":1,"maximum":8}}]},
  "requiredCapabilities":["legal-actions"],
  "sources":["chess-raiders-bot.json","bot.star","parameters.json"]
}`

func validEntries(raw []byte) []TreeEntry {
	return []TreeEntry{
		{Path: ManifestPath, Kind: EntryKindRegular, Content: raw},
		{Path: "bot.star", Kind: EntryKindRegular, Content: []byte("def decide(observation, memory, params, random_draw, options):\n    return None\n")},
		{Path: "parameters.json", Kind: EntryKindRegular, Content: []byte(`{"search":4}`)},
	}
}

var testLimits = ValidationLimits{MaxFiles: 3, MaxFileBytes: 10_000, MaxTotalBytes: 30_000}

func TestParseStrictlyRejectsUnknownFieldsAndInvalidDeclarations(t *testing.T) {
	_, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse(valid manifest) = %v", err)
	}
	for _, raw := range []string{
		`{"schema":"chess-raiders-bot-manifest/v1","unknown":true}`,
		`{"schema":"chess-raiders-bot-manifest/v1","schema":"chess-raiders-bot-manifest/v1"}`,
		`{"schema":"wrong"}`,
	} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatalf("Parse(%s) = nil, want strict validation error", raw)
		}
	}
}

func TestPortableManifestFixtures(t *testing.T) {
	valid, err := os.ReadFile(filepath.Join("testdata", "valid-v1.json"))
	if err != nil {
		t.Fatalf("read valid fixture: %v", err)
	}
	if _, err := Parse(valid); err != nil {
		t.Fatalf("Parse(valid fixture) = %v", err)
	}
	for _, name := range []string{"invalid-unknown-field.json", "invalid-traversal.json"} {
		raw, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err := Parse(raw); err == nil {
			t.Fatalf("Parse(%s) = nil, want failure", name)
		}
	}
}

func TestValidateArtifactRejectsUnsafeAndNonCanonicalClosures(t *testing.T) {
	raw := []byte(validManifest)
	tests := []struct {
		name    string
		entries []TreeEntry
		limits  ValidationLimits
		code    ErrorCode
	}{
		{name: "traversal", entries: append(validEntries(raw), TreeEntry{Path: "../secret", Kind: EntryKindRegular}), limits: ValidationLimits{MaxFiles: 4, MaxFileBytes: 10_000, MaxTotalBytes: 30_000}, code: ErrInvalidPath},
		{name: "symlink", entries: []TreeEntry{{Path: ManifestPath, Kind: EntryKindRegular, Content: raw}, {Path: "bot.star", Kind: EntryKindSymlink}, {Path: "parameters.json", Kind: EntryKindRegular, Content: []byte(`{}`)}}, limits: testLimits, code: ErrNonRegularFile},
		{name: "lfs", entries: []TreeEntry{{Path: ManifestPath, Kind: EntryKindRegular, Content: raw}, {Path: "bot.star", Kind: EntryKindRegular, Content: []byte("version https://git-lfs.github.com/spec/v1\noid sha256:abc\n")}, {Path: "parameters.json", Kind: EntryKindRegular, Content: []byte(`{}`)}}, limits: testLimits, code: ErrLFSPointer},
		{name: "undeclared", entries: append(validEntries(raw), TreeEntry{Path: "README.md", Kind: EntryKindRegular}), limits: ValidationLimits{MaxFiles: 4, MaxFileBytes: 10_000, MaxTotalBytes: 30_000}, code: ErrUndeclaredFile},
		{name: "file limit", entries: validEntries(raw), limits: ValidationLimits{MaxFiles: 2, MaxFileBytes: 10_000, MaxTotalBytes: 30_000}, code: ErrFileLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateArtifact(raw, test.entries, test.limits)
			if err == nil {
				t.Fatal("ValidateArtifact() = nil, want failure")
			}
			var validationError *Error
			if !errors.As(err, &validationError) || validationError.Code != test.code {
				t.Fatalf("ValidateArtifact() error = %T %v, want code %s", err, err, test.code)
			}
		})
	}
}

func TestDigestClosureIsSortedAndLengthDelimited(t *testing.T) {
	first := DigestClosure(map[string][]byte{"a": []byte("bc"), "ab": []byte("c")})
	second := DigestClosure(map[string][]byte{"ab": []byte("c"), "a": []byte("bc")})
	if first != second {
		t.Fatalf("same closure in a different map order produced %s and %s", first, second)
	}
	joinedDifferently := DigestClosure(map[string][]byte{"a": []byte("bca"), "b": []byte("c")})
	if first == joinedDifferently {
		t.Fatal("length-distinct closures produced the same digest")
	}
}
