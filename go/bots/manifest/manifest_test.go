// Copyright 2026 Sneat.app

package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
  "parameters":{"schemaPath":"parameters.schema.json","setsPath":"parameters.json","defaultSet":"default"},
  "requiredCapabilities":["legal-actions"],
  "sources":["chess-raiders-bot.json","bot.star","parameters.schema.json","parameters.json"]
}`

const validParameterSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "properties":{"search":{"type":"integer","default":4,"minimum":1,"maximum":8}},
  "additionalProperties":false
}`

const validParameterSets = `{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"search":4}}}`

func validEntries(raw []byte) []TreeEntry {
	return []TreeEntry{
		{Path: ManifestPath, Kind: EntryKindRegular, Content: raw},
		{Path: "bot.star", Kind: EntryKindRegular, Content: []byte("def decide(observation, memory, params, random_draw, options):\n    return None\n")},
		{Path: "parameters.schema.json", Kind: EntryKindRegular, Content: []byte(validParameterSchema)},
		{Path: "parameters.json", Kind: EntryKindRegular, Content: []byte(validParameterSets)},
	}
}

var testLimits = ValidationLimits{MaxFiles: 4, MaxFileBytes: 10_000, MaxTotalBytes: 40_000, MaxManifestBytes: 10_000, MaxJSONDepth: 32}

var testProfile = CompatibilityProfile{
	Game:                "chess-raiders",
	Rules:               Range{Minimum: "chess-raiders-rules/v1", Maximum: "chess-raiders-rules/v1"},
	Runtime:             Range{Minimum: "chess-raiders-starlark-runtime/v1", Maximum: "chess-raiders-starlark-runtime/v1"},
	ScriptProtocol:      "chess-raiders-bot-script/v1",
	AllowedStateModes:   []StateMode{StateModeStateless},
	AllowedCapabilities: []string{"legal-actions"},
}

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

func TestPortablePathsRejectASCIIControlCharacters(t *testing.T) {
	for _, escapedPath := range []string{`bot\r.star`, `bot\n.star`, `bot\u007f.star`} {
		raw := strings.ReplaceAll(validManifest, "bot.star", escapedPath)
		_, err := Parse([]byte(raw))
		if err == nil {
			t.Fatalf("Parse(entrypoint %q) = nil, want control-character path rejection", escapedPath)
		}
		var validationError *Error
		if !errors.As(err, &validationError) || validationError.Code != ErrInvalidPath {
			t.Fatalf("Parse(entrypoint %q) error = %T %v, want invalid-path", escapedPath, err, err)
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
		{name: "traversal", entries: append(validEntries(raw), TreeEntry{Path: "../secret", Kind: EntryKindRegular}), limits: ValidationLimits{MaxFiles: 5, MaxFileBytes: 10_000, MaxTotalBytes: 50_000, MaxManifestBytes: 10_000, MaxJSONDepth: 32}, code: ErrInvalidPath},
		{name: "symlink", entries: []TreeEntry{{Path: ManifestPath, Kind: EntryKindRegular, Content: raw}, {Path: "bot.star", Kind: EntryKindSymlink}, {Path: "parameters.json", Kind: EntryKindRegular, Content: []byte(`{}`)}}, limits: testLimits, code: ErrNonRegularFile},
		{name: "submodule", entries: []TreeEntry{{Path: ManifestPath, Kind: EntryKindRegular, Content: raw}, {Path: "bot.star", Kind: EntryKindSubmodule}}, limits: testLimits, code: ErrNonRegularFile},
		{name: "lfs", entries: []TreeEntry{{Path: ManifestPath, Kind: EntryKindRegular, Content: raw}, {Path: "bot.star", Kind: EntryKindRegular, Content: []byte("version https://git-lfs.github.com/spec/v1\noid sha256:abc\n")}, {Path: "parameters.json", Kind: EntryKindRegular, Content: []byte(`{}`)}}, limits: testLimits, code: ErrLFSPointer},
		{name: "undeclared", entries: append(validEntries(raw), TreeEntry{Path: "README.md", Kind: EntryKindRegular}), limits: ValidationLimits{MaxFiles: 5, MaxFileBytes: 10_000, MaxTotalBytes: 50_000, MaxManifestBytes: 10_000, MaxJSONDepth: 32}, code: ErrUndeclaredFile},
		{name: "file limit", entries: validEntries(raw), limits: ValidationLimits{MaxFiles: 3, MaxFileBytes: 10_000, MaxTotalBytes: 40_000, MaxManifestBytes: 10_000, MaxJSONDepth: 32}, code: ErrFileLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateArtifact(raw, test.entries, test.limits, testProfile)
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

func TestValidateArtifactRequiresAnExactCallerProfile(t *testing.T) {
	raw := []byte(validManifest)
	tests := []struct {
		name    string
		profile CompatibilityProfile
	}{
		{name: "game", profile: func() CompatibilityProfile {
			profile := testProfile
			profile.Game = "another-game"
			return profile
		}()},
		{name: "rules", profile: func() CompatibilityProfile {
			profile := testProfile
			profile.Rules.Maximum = "chess-raiders-rules/v2"
			return profile
		}()},
		{name: "runtime", profile: func() CompatibilityProfile {
			profile := testProfile
			profile.Runtime.Minimum = "chess-raiders-starlark-runtime/v2"
			return profile
		}()},
		{name: "script protocol", profile: func() CompatibilityProfile {
			profile := testProfile
			profile.ScriptProtocol = "another-protocol/v1"
			return profile
		}()},
		{name: "state mode", profile: func() CompatibilityProfile {
			profile := testProfile
			profile.AllowedStateModes = []StateMode{StateModeBattleStateful}
			return profile
		}()},
		{name: "capability", profile: func() CompatibilityProfile { profile := testProfile; profile.AllowedCapabilities = nil; return profile }()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateArtifact(raw, validEntries(raw), testLimits, test.profile)
			if err == nil {
				t.Fatal("ValidateArtifact() = nil, want unsupported-profile")
			}
			var validationError *Error
			if !errors.As(err, &validationError) || validationError.Code != ErrUnsupportedProfile {
				t.Fatalf("ValidateArtifact() error = %T %v, want unsupported-profile", err, err)
			}
		})
	}
}

func TestValidateManifestBuildsABoundedCanonicalFetchPlan(t *testing.T) {
	plan, err := ValidateManifest([]byte(validManifest), testLimits, testProfile)
	if err != nil {
		t.Fatalf("ValidateManifest() = %v", err)
	}
	wantPaths := []string{"bot.star", "chess-raiders-bot.json", "parameters.json", "parameters.schema.json"}
	if strings.Join(plan.Paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("plan paths = %v, want sorted %v", plan.Paths, wantPaths)
	}
	if plan.Limits != testLimits {
		t.Fatalf("plan limits = %+v, want %+v", plan.Limits, testLimits)
	}

	tooFewFiles := testLimits
	tooFewFiles.MaxFiles = 3
	_, err = ValidateManifest([]byte(validManifest), tooFewFiles, testProfile)
	if err == nil {
		t.Fatal("ValidateManifest() = nil, want declared file-limit rejection before any fetch")
	}
}

func TestValidateArtifactReturnsOnlyTheResolvedSelectedParameters(t *testing.T) {
	artifact, err := ValidateArtifact([]byte(validManifest), validEntries([]byte(validManifest)), testLimits, testProfile)
	if err != nil {
		t.Fatalf("ValidateArtifact() = %v", err)
	}
	if got, want := string(artifact.ResolvedParameters), `{"search":4}`; got != want {
		t.Fatalf("ResolvedParameters = %s, want %s", got, want)
	}
}

func TestValidateArtifactRejectsMalformedDeclaredParameterBytes(t *testing.T) {
	raw := []byte(validManifest)
	for _, content := range [][]byte{
		[]byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"search":`),
		[]byte(strings.Repeat(`{"nested":`, 17) + `0` + strings.Repeat(`}`, 17)),
		[]byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"Search":4,"search":4}}}`),
	} {
		entries := validEntries(raw)
		entries[3].Content = content
		limits := testLimits
		limits.MaxJSONDepth = 16
		_, err := ValidateArtifact(raw, entries, limits, testProfile)
		if err == nil {
			t.Fatal("ValidateArtifact() = nil, want malformed/deep/duplicate parameter JSON rejection")
		}
		var validationError *Error
		if !errors.As(err, &validationError) || validationError.Path != "parameters.json" {
			t.Fatalf("ValidateArtifact() error = %T %v, want parameter-path validation error", err, err)
		}
	}
}

func TestValidateArtifactBoundsRawManifestAndJSONDepthBeforeDecode(t *testing.T) {
	tooSmall := testLimits
	tooSmall.MaxManifestBytes = int64(len(validManifest) - 1)
	_, err := ValidateArtifact([]byte(validManifest), nil, tooSmall, testProfile)
	if err == nil {
		t.Fatal("ValidateArtifact() = nil, want raw manifest byte limit rejection")
	}
	var validationError *Error
	if !errors.As(err, &validationError) || validationError.Code != ErrByteLimit {
		t.Fatalf("raw manifest limit error = %T %v, want byte-limit", err, err)
	}

	deep := []byte(strings.Repeat("[", 33) + strings.Repeat("]", 33))
	depthLimited := testLimits
	depthLimited.MaxJSONDepth = 16
	_, err = ValidateArtifact(deep, nil, depthLimited, testProfile)
	if err == nil {
		t.Fatal("ValidateArtifact() = nil, want JSON depth rejection")
	}
	if !errors.As(err, &validationError) || validationError.Code != ErrJSONDepth {
		t.Fatalf("deep JSON error = %T %v, want json-depth", err, err)
	}
}

func TestParseRejectsCaseFoldedSchemaAndNestedFieldAliases(t *testing.T) {
	for _, raw := range []string{
		strings.Replace(validManifest, `"schema"`, `"Schema"`, 1),
		strings.Replace(validManifest, `"name":"Test bot"`, `"Name":"Test bot"`, 1),
		`{"schema":"chess-raiders-bot-manifest/v1","Schema":"chess-raiders-bot-manifest/v1"}`,
	} {
		_, err := Parse([]byte(raw))
		if err == nil {
			t.Fatalf("Parse(%s) = nil, want exact-case rejection", raw)
		}
		var validationError *Error
		if !errors.As(err, &validationError) || (validationError.Code != ErrUnknownField && validationError.Code != ErrInvalidJSON) {
			t.Fatalf("Parse() error = %T %v, want exact-case typed error", err, err)
		}
	}
}

func TestValidateClosureAppliesRawManifestLimitsForDirectCallers(t *testing.T) {
	raw := []byte(validManifest)
	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(valid manifest) = %v", err)
	}
	for _, test := range []struct {
		name   string
		limits ValidationLimits
	}{
		{
			name: "manifest ceiling",
			limits: func() ValidationLimits {
				limits := testLimits
				limits.MaxManifestBytes = int64(len(raw) - 1)
				return limits
			}(),
		},
		{
			name: "per-file ceiling",
			limits: func() ValidationLimits {
				limits := testLimits
				limits.MaxFileBytes = int64(len(raw) - 1)
				return limits
			}(),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateClosure(parsed, raw, validEntries(raw), test.limits)
			if err == nil {
				t.Fatal("ValidateClosure() = nil, want raw manifest byte limit rejection")
			}
			var validationError *Error
			if !errors.As(err, &validationError) || validationError.Code != ErrByteLimit {
				t.Fatalf("ValidateClosure() error = %T %v, want byte-limit", err, err)
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
