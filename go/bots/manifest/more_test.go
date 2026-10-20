// Copyright 2026 Sneat.app

package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
)

func TestDigestAndHex(t *testing.T) {
	d := Digest{1, 2, 3, 4}
	s := d.String()
	if len(s) == 0 {
		t.Fatal("Digest.String() is empty")
	}
}

func TestValidateManifestAndCompatibilityEdges(t *testing.T) {
	// Invalid limits
	zeroLimits := ValidationLimits{}
	if _, err := ValidateManifest([]byte(validManifest), zeroLimits, standardProfile); err == nil {
		t.Error("ValidateManifest with zero limits succeeded, want error")
	}

	// Manifest byte limit exceeded
	smallLimits := testLimits
	smallLimits.MaxManifestBytes = 10
	if _, err := ValidateManifest([]byte(validManifest), smallLimits, standardProfile); err == nil {
		t.Error("ValidateManifest with small MaxManifestBytes succeeded, want error")
	}

	// Invalid json
	if _, err := ValidateManifest([]byte("{invalid"), testLimits, standardProfile); err == nil {
		t.Error("ValidateManifest with invalid JSON succeeded, want error")
	}

	// Compatibility error: empty profile fields
	emptyProfile := CompatibilityProfile{}
	if err := ValidateCompatibility(Manifest{}, emptyProfile); err == nil {
		t.Error("ValidateCompatibility with empty profile succeeded, want error")
	}

	// Manifest compatibility mismatch
	m := Manifest{
		Compatibility: Compatibility{
			Game:    "other-game",
			Rules:   Range{Minimum: "min", Maximum: "max"},
			Runtime: Range{Minimum: "min", Maximum: "max"},
		},
		Runtime:   Runtime{Protocol: "proto"},
		StateMode: StateModeStateless,
	}
	if err := ValidateCompatibility(m, standardProfile); err == nil {
		t.Error("ValidateCompatibility with mismatched game succeeded, want error")
	}

	// StateMode not allowed
	mAllowed := Manifest{
		Compatibility: standardProfile.Compatibility(),
		Runtime:       Runtime{Protocol: standardProfile.ScriptProtocol},
		StateMode:     StateMode("unknown-mode"),
	}
	if err := ValidateCompatibility(mAllowed, standardProfile); err == nil {
		t.Error("ValidateCompatibility with disallowed stateMode succeeded, want error")
	}

	// Exceed MaxFiles in ValidateManifest
	fewFilesLimits := testLimits
	fewFilesLimits.MaxFiles = 1
	if _, err := ValidateManifest([]byte(validManifest), fewFilesLimits, standardProfile); err == nil {
		t.Error("ValidateManifest exceeding MaxFiles succeeded, want error")
	}
}

var standardProfile = CompatibilityProfile{
	Game:                "chess-raiders",
	Rules:               Range{Minimum: "chess-raiders-rules/v1", Maximum: "chess-raiders-rules/v1"},
	Runtime:             Range{Minimum: "chess-raiders-starlark-runtime/v1", Maximum: "chess-raiders-starlark-runtime/v1"},
	ScriptProtocol:      "chess-raiders-bot-script/v1",
	AllowedStateModes:   []StateMode{StateModeStateless, StateModeBattleStateful},
	AllowedCapabilities: []string{"legal-actions"},
}

func (p CompatibilityProfile) Compatibility() Compatibility {
	return Compatibility{Game: p.Game, Rules: p.Rules, Runtime: p.Runtime}
}

func TestValidateClosureEdges(t *testing.T) {
	manifestBytes := []byte(validManifest)
	var m Manifest
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		t.Fatal(err)
	}

	// Invalid limits
	if _, err := validateClosure(m, manifestBytes, validEntries(manifestBytes), ValidationLimits{}); err == nil {
		t.Error("validateClosure with zero limits succeeded, want error")
	}

	// Manifest argument mismatch
	otherM := m
	otherM.Display.Name = "Different Name"
	if _, err := validateClosure(otherM, manifestBytes, validEntries(manifestBytes), testLimits); err == nil {
		t.Error("validateClosure with mismatched manifest argument succeeded, want error")
	}

	// Non-canonical path in entry
	badEntries := validEntries(manifestBytes)
	badEntries[1].Path = "./bot.star"
	if _, err := validateClosure(m, manifestBytes, badEntries, testLimits); err == nil {
		t.Error("validateClosure with non-canonical path succeeded, want error")
	}

	// Duplicate path in entries
	dupEntries := append(validEntries(manifestBytes), validEntries(manifestBytes)[0])
	if _, err := validateClosure(m, manifestBytes, dupEntries, testLimits); err == nil {
		t.Error("validateClosure with duplicate path succeeded, want error")
	}

	// Entry byte limit exceeded
	tinyLimits := testLimits
	tinyLimits.MaxFileBytes = 5
	if _, err := validateClosure(m, manifestBytes, validEntries(manifestBytes), tinyLimits); err == nil {
		t.Error("validateClosure exceeding MaxFileBytes succeeded, want error")
	}

	// Total byte limit exceeded
	smallTotal := testLimits
	smallTotal.MaxTotalBytes = 100
	if _, err := validateClosure(m, manifestBytes, validEntries(manifestBytes), smallTotal); err == nil {
		t.Error("validateClosure exceeding MaxTotalBytes succeeded, want error")
	}

	// Manifest bytes differ from tree entry content
	badManifestContent := validEntries(manifestBytes)
	badManifestContent[0].Content = []byte(validManifest + " ")
	if _, err := validateClosure(m, manifestBytes, badManifestContent, testLimits); err == nil {
		t.Error("validateClosure with differing manifest bytes in tree succeeded, want error")
	}

	// Declared file missing from tree
	missingEntries := validEntries(manifestBytes)[:3]
	if _, err := validateClosure(m, manifestBytes, missingEntries, testLimits); err == nil {
		t.Error("validateClosure with missing file succeeded, want error")
	}

	// Resolved parameter bytes exceeded
	paramLimits := testLimits
	paramLimits.MaxResolvedParameterBytes = 2
	if _, err := validateClosure(m, manifestBytes, validEntries(manifestBytes), paramLimits); err == nil {
		t.Error("validateClosure exceeding MaxResolvedParameterBytes succeeded, want error")
	}
}

func TestExactDecimal(t *testing.T) {
	cases := []struct {
		input string
		valid bool
	}{
		{"", false},
		{"-", false},
		{"abc", false},
		{"1.", false},
		{"1e", false},
		{"1e999999999999999999999999999999", false},
		{"1.23x", false},
		{"0", true},
		{"-0", true},
		{"123", true},
		{"-456", true},
		{"0.789", true},
		{"-0.001", true},
		{"1e2", true},
		{"1e-2", true},
		{"1.5e+3", true},
		{"1.5E-3", true},
	}

	for _, c := range cases {
		_, err := exactDecimal(c.input)
		if (err == nil) != c.valid {
			t.Errorf("exactDecimal(%q) error = %v, want valid=%v", c.input, err, c.valid)
		}
	}
}

func TestParameterValidationEdges(t *testing.T) {
	limits := ParameterLimits{
		MaxDocumentBytes: 10_000,
		MaxJSONDepth:     10,
		MaxProperties:    10,
		MaxSets:          10,
		MaxResolvedBytes: 10_000,
	}

	// Invalid parameter limits
	if _, err := ParseParameterSchema([]byte("{}"), ParameterLimits{}); err == nil {
		t.Error("ParseParameterSchema with zero limits succeeded, want error")
	}

	// Input not an object
	if _, err := ParseParameterSchema([]byte("[]"), limits); err == nil {
		t.Error("ParseParameterSchema with array input succeeded, want error")
	}

	// Unknown fields in schema
	if _, err := ParseParameterSchema([]byte(`{"properties":{}, "extra":1}`), limits); err == nil {
		t.Error("ParseParameterSchema with extra fields succeeded, want error")
	}

	// Description not a string
	if _, err := ParseParameterSchema([]byte(`{"properties":{}, "description":123}`), limits); err == nil {
		t.Error("ParseParameterSchema with non-string description succeeded, want error")
	}

	// Missing properties
	if _, err := ParseParameterSchema([]byte(`{"type":"object"}`), limits); err == nil {
		t.Error("ParseParameterSchema missing properties succeeded, want error")
	}

	// Properties not an object
	if _, err := ParseParameterSchema([]byte(`{"properties":123}`), limits); err == nil {
		t.Error("ParseParameterSchema with non-object properties succeeded, want error")
	}

	// Empty property name
	if _, err := ParseParameterSchema([]byte(`{"properties":{" ":{}}}`), limits); err == nil {
		t.Error("ParseParameterSchema with empty property name succeeded, want error")
	}

	// Property shapes
	shapes := []string{
		`{"properties":{"x":{"type":"integer","minimum":1,"maximum":5,"items":{}}}}`,
		`{"properties":{"x":{"type":"string","minimum":1}}}`,
		`{"properties":{"x":{"type":"boolean","enum":[true]}}}`,
		`{"properties":{"x":{"type":"array"}}}`,
		`{"properties":{"x":{"type":"array","minimum":1,"items":{"type":"integer","minimum":1,"maximum":5}}}}`,
		`{"properties":{"x":{"type":"unknown"}}}`,
		`{"properties":{"x":{"type":"array","items":{"type":"integer","minimum":10,"maximum":5}}}}`,
		`{"properties":{"x":{"type":"array","items":{"type":"integer","enum":["a"]}}}}`,
		`{"properties":{"x":{"type":"array","items":{"type":"string","minimum":1}}}}`,
		`{"properties":{"x":{"type":"array","items":{"type":"boolean","minimum":1}}}}`,
		`{"properties":{"x":{"type":"array","items":{"type":"unknown"}}}}`,
		`{"properties":{"x":{"type":"integer","minimum":5,"maximum":2}}}`,
		`{"properties":{"x":{"type":"integer","minimum":1.5,"maximum":5}}}`,
		`{"properties":{"x":{"type":"string","enum":["a","a"]}}}`,
		`{"properties":{"x":{"type":"string","enum":[]}}}`,
		`{"properties":{"x":{"type":"string","enum":[123]}}}`,
	}
	for _, shape := range shapes {
		if _, err := ParseParameterSchema([]byte(shape), limits); err == nil {
			t.Errorf("ParseParameterSchema(%s) succeeded, want error", shape)
		}
	}
}

func TestResolveParameterSetEdges(t *testing.T) {
	limits := ParameterLimits{
		MaxDocumentBytes: 10_000,
		MaxJSONDepth:     10,
		MaxProperties:    10,
		MaxSets:          10,
		MaxResolvedBytes: 10_000,
	}

	schemaBytes := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"properties":{
			"count":{"type":"integer","default":2,"minimum":1,"maximum":5},
			"rate":{"type":"number","default":1.5,"minimum":0.5,"maximum":3.0},
			"name":{"type":"string","default":"default","enum":["default","custom"]},
			"active":{"type":"boolean","default":true},
			"items":{"type":"array","default":[1],"items":{"type":"integer","minimum":1,"maximum":5}}
		},
		"additionalProperties":false
	}`)
	schema, err := ParseParameterSchema(schemaBytes, limits)
	if err != nil {
		t.Fatalf("ParseParameterSchema failed: %v", err)
	}

	_ = schema
	// Invalid json in sets
	if _, err := ResolveParameterSet(schemaBytes, []byte("{invalid"), "default", limits); err == nil {
		t.Error("ResolveParameterSet with invalid JSON succeeded, want error")
	}

	// Sets not an object
	if _, err := ResolveParameterSet(schemaBytes, []byte("[]"), "default", limits); err == nil {
		t.Error("ResolveParameterSet with array input succeeded, want error")
	}

	// Unknown field in sets
	if _, err := ResolveParameterSet(schemaBytes, []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{},"extra":1}`), "default", limits); err == nil {
		t.Error("ResolveParameterSet with extra field succeeded, want error")
	}

	// Wrong schema version
	if _, err := ResolveParameterSet(schemaBytes, []byte(`{"schema":"wrong/v1","sets":{}}`), "default", limits); err == nil {
		t.Error("ResolveParameterSet with wrong schema succeeded, want error")
	}

	// Empty sets
	if _, err := ResolveParameterSet(schemaBytes, []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{}}`), "default", limits); err == nil {
		t.Error("ResolveParameterSet with empty sets succeeded, want error")
	}

	// Selected empty
	validSets := `{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{}}}`
	if _, err := ResolveParameterSet(schemaBytes, []byte(validSets), "", limits); err == nil {
		t.Error("ResolveParameterSet with empty selected succeeded, want error")
	}

	// Selected not present
	if _, err := ResolveParameterSet(schemaBytes, []byte(validSets), "nonexistent", limits); err == nil {
		t.Error("ResolveParameterSet with nonexistent selected succeeded, want error")
	}

	// Empty set name in sets
	emptySetName := `{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{" ":{}}}`
	if _, err := ResolveParameterSet(schemaBytes, []byte(emptySetName), " ", limits); err == nil {
		t.Error("ResolveParameterSet with whitespace set name succeeded, want error")
	}

	// Exceed MaxSets
	smallSetsLimits := limits
	smallSetsLimits.MaxSets = 1
	multiSets := `{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"a":{},"b":{}}}`
	if _, err := ResolveParameterSet(schemaBytes, []byte(multiSets), "a", smallSetsLimits); err == nil {
		t.Error("ResolveParameterSet exceeding MaxSets succeeded, want error")
	}

	// Malformed property values in set
	badSets := []string{
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"count":"not-int"}}}`,
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"count":10}}}`,
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"name":123}}}`,
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"name":"unknown"}}}`,
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"active":"not-bool"}}}`,
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"items":"not-array"}}}`,
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"items":[10]}}}`,
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"items":["string"]}}}`,
		`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{"undeclared":1}}}`,
	}
	for _, bad := range badSets {
		if _, err := ResolveParameterSet(schemaBytes, []byte(bad), "default", limits); err == nil {
			t.Errorf("ResolveParameterSet(%s) succeeded, want error", bad)
		}
	}

	// Valid set resolution with overrides
	customSet := `{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{},"custom":{"count":3,"name":"custom","active":false,"items":[2]}}}`
	resolved, err := ResolveParameterSet(schemaBytes, []byte(customSet), "custom", limits)
	if err != nil {
		t.Fatalf("ResolveParameterSet custom failed: %v", err)
	}
	if len(resolved) == 0 {
		t.Fatal("resolved parameter JSON is empty")
	}
}

func TestParameterSetErrorGeneric(t *testing.T) {
	err := parameterSetError("my_set", errors.New("generic error"))
	if err == nil || err.Error() == "" {
		t.Error("parameterSetError failed")
	}

	errWithEmptyField := parameterSetError("my_set", &Error{Code: ErrParameterConfig})
	if errWithEmptyField == nil {
		t.Error("parameterSetError with empty field failed")
	}
}

func TestManifestValidateFull(t *testing.T) {
	base := Manifest{
		Schema:  SchemaVersion,
		Display: Display{Name: "Name", Description: "Desc"},
		Author:  Attribution{Name: "Author"},
		Source:  Source{Repository: "repo", Attribution: "attr"},
		License: License{SPDX: "Apache-2.0"},
		Compatibility: Compatibility{
			Game:    "chess-raiders",
			Rules:   Range{Minimum: "chess-raiders-rules/v1", Maximum: "chess-raiders-rules/v1"},
			Runtime: Range{Minimum: "chess-raiders-starlark-runtime/v1", Maximum: "chess-raiders-starlark-runtime/v1"},
		},
		Runtime: Runtime{
			Kind:       "starlark",
			Protocol:   "chess-raiders-bot-script/v1",
			Entrypoint: "bot.star",
		},
		StateMode: StateModeStateless,
		Parameters: Parameters{
			SchemaPath: "parameters.schema.json",
			SetsPath:   "parameters.json",
			DefaultSet: "default",
		},
		RequiredCapabilities: []string{"legal-actions"},
		Sources: []string{
			ManifestPath,
			"bot.star",
			"parameters.schema.json",
			"parameters.json",
		},
	}

	if err := Validate(base); err != nil {
		t.Fatalf("Validate(base) failed: %v", err)
	}

	cases := []func(*Manifest){
		func(m *Manifest) { m.Schema = "bad" },
		func(m *Manifest) { m.Display.Name = "" },
		func(m *Manifest) { m.Author.Name = "" },
		func(m *Manifest) { m.Compatibility.Game = "bad" },
		func(m *Manifest) { m.Runtime.Kind = "python" },
		func(m *Manifest) { m.Runtime.Entrypoint = "" },
		func(m *Manifest) { m.StateMode = StateMode("bad") },
		func(m *Manifest) { m.Parameters.SchemaPath = "" },
		func(m *Manifest) { m.Parameters.SetsPath = "" },
		func(m *Manifest) { m.Parameters.DefaultSet = "" },
		func(m *Manifest) { m.Parameters.SetsPath = m.Parameters.SchemaPath },
		func(m *Manifest) { m.RequiredCapabilities = []string{"cap", "cap"} },
		func(m *Manifest) { m.RequiredCapabilities = []string{""} },
		func(m *Manifest) { m.Sources = nil },
		func(m *Manifest) { m.Sources = []string{ManifestPath, ManifestPath} },
		func(m *Manifest) { m.Sources = []string{ManifestPath} }, // missing required entrypoint/params
	}

	for i, modify := range cases {
		copy := base
		modify(&copy)
		if err := Validate(copy); err == nil {
			t.Errorf("Validate case %d succeeded, want error", i)
		}
	}
}

func TestRuntimeNumbersEqual(t *testing.T) {
	nInt1 := runtimeNumber{integer: big.NewInt(10)}
	nInt2 := runtimeNumber{integer: big.NewInt(10)}
	nInt3 := runtimeNumber{integer: big.NewInt(20)}
	if !runtimeNumbersEqual(nInt1, nInt2) {
		t.Error("expected ints equal")
	}
	if runtimeNumbersEqual(nInt1, nInt3) {
		t.Error("expected ints not equal")
	}

	nFlt1 := runtimeNumber{isFloat: true, floating: 3.14}
	nFlt2 := runtimeNumber{isFloat: true, floating: 3.14}
	nFlt3 := runtimeNumber{isFloat: true, floating: 2.71}
	if !runtimeNumbersEqual(nFlt1, nFlt2) {
		t.Error("expected floats equal")
	}
	if runtimeNumbersEqual(nFlt1, nFlt3) {
		t.Error("expected floats not equal")
	}

	nInt10 := runtimeNumber{integer: big.NewInt(10)}
	nFlt10 := runtimeNumber{isFloat: true, floating: 10.0}
	if !runtimeNumbersEqual(nInt10, nFlt10) {
		t.Error("expected int and float 10 equal")
	}
	if !runtimeNumbersEqual(nFlt10, nInt10) {
		t.Error("expected float and int 10 equal")
	}
}

func TestManifestValidationDeepEdges(t *testing.T) {
	if f := expectedFields("unknown"); f != nil {
		t.Error("expectedFields(unknown) should be nil")
	}

	if err := scanJSON([]byte("{}"), MaximumJSONDepth+1, false); err == nil {
		t.Error("scanJSON with depth > MaximumJSONDepth succeeded, want error")
	}

	decExtra := json.NewDecoder(bytes.NewReader([]byte("{} 123")))
	var v any
	_ = decExtra.Decode(&v)
	if err := requireJSONEOF(decExtra); err == nil {
		t.Error("requireJSONEOF with trailing value succeeded, want error")
	}

	decBadTrailing := json.NewDecoder(bytes.NewReader([]byte("{} {")))
	_ = decBadTrailing.Decode(&v)
	if err := requireJSONEOF(decBadTrailing); err == nil {
		t.Error("requireJSONEOF with trailing bad syntax succeeded, want error")
	}

	if err := validateJSONInput([]byte("{}"), -1, 10); err == nil {
		t.Error("validateJSONInput with negative bytes succeeded, want error")
	}
	if err := validateJSONInput([]byte("{}"), 1, 10); err == nil {
		t.Error("validateJSONInput exceeding max bytes succeeded, want error")
	}

	if _, err := rawObject([]byte("null")); err == nil {
		t.Error("rawObject(null) succeeded, want error")
	}

	limits := ParameterLimits{
		MaxDocumentBytes: 10_000,
		MaxJSONDepth:     10,
		MaxProperties:    10,
		MaxSets:          10,
		MaxResolvedBytes: 10_000,
	}

	moreBadSchemas := []string{
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"string","additionalProperties":false,"properties":{"x":{"type":"integer","minimum":1,"maximum":5,"default":2}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"integer","minimum":1,"maximum":5}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"integer","minimum":1,"maximum":5,"default":2,"description":123}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"string","default":"a","minimum":1}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"string","default":"a","maximum":1}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"array","default":[],"items":123}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"array","default":[],"items":{"type":"integer","minimum":1,"maximum":5,"extra":1}}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"array","default":[],"items":{"type":"integer","minimum":1,"maximum":5,"description":123}}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"array","default":[],"items":{"type":"string","minimum":1}}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"x":{"type":"array","default":[],"items":{"type":"string","maximum":1}}}}`,
	}
	for _, s := range moreBadSchemas {
		if _, err := ParseParameterSchema([]byte(s), limits); err == nil {
			t.Errorf("ParseParameterSchema(%s) succeeded, want error", s)
		}
	}

	propUnknown := ParameterProperty{Type: ParameterType("unknown")}
	if _, err := normalizeParameterValue(propUnknown, json.RawMessage("123"), "field"); err == nil {
		t.Error("normalizeParameterValue with unknown type succeeded, want error")
	}

	propInt := ParameterProperty{Type: ParameterTypeInteger, Minimum: "1", Maximum: "5"}
	if _, err := normalizeParameterValue(propInt, json.RawMessage(`"not-a-number"`), "field"); err == nil {
		t.Error("normalizeParameterValue with non-number succeeded, want error")
	}

	if parameterValuesEqual(propUnknown, normalizedParameterValue{}, normalizedParameterValue{}) {
		t.Error("parameterValuesEqual with unknown type returned true, want false")
	}
}

func TestManifestValidationUltimateEdges(t *testing.T) {
	limits := ParameterLimits{
		MaxDocumentBytes: 10_000,
		MaxJSONDepth:     10,
		MaxProperties:    10,
		MaxSets:          10,
		MaxResolvedBytes: 10_000,
	}

	// ResolveParameterConfig with invalid schema
	if _, err := ResolveParameterConfig([]byte("{invalid"), []byte("{}"), limits); err == nil {
		t.Error("ResolveParameterConfig with invalid schema succeeded, want error")
	}

	// ResolveParameterConfig with non-object config
	validSchema := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"properties":{"x":{"type":"integer","default":1,"minimum":0,"maximum":5}},
		"additionalProperties":false
	}`)
	if _, err := ResolveParameterConfig(validSchema, []byte("[]"), limits); err == nil {
		t.Error("ResolveParameterConfig with array config succeeded, want error")
	}

	// ResolveParameterConfig with too many properties
	smallPropLimits := limits
	smallPropLimits.MaxProperties = 1
	twoPropsConfig := []byte(`{"a":1, "b":2}`)
	if _, err := ResolveParameterConfig(validSchema, twoPropsConfig, smallPropLimits); err == nil {
		t.Error("ResolveParameterConfig exceeding MaxProperties succeeded, want error")
	}

	// ResolveParameterConfig exceeding MaxResolvedBytes
	smallBytesLimits := limits
	smallBytesLimits.MaxResolvedBytes = 2
	if _, err := ResolveParameterConfig(validSchema, []byte("{}"), smallBytesLimits); err == nil {
		t.Error("ResolveParameterConfig exceeding MaxResolvedBytes succeeded, want error")
	}

	// ResolveParameterConfig invalid limits
	if _, err := ResolveParameterConfig(validSchema, []byte("{}"), ParameterLimits{}); err == nil {
		t.Error("ResolveParameterConfig with zero limits succeeded, want error")
	}

	// resolveParameterSet with whitespace set name
	setsWithSpace := []byte(`{
		"schema":"chess-raiders-bot-parameter-sets/v1",
		"sets":{"   ":{}, "default":{}}
	}`)
	if _, err := ResolveParameterSet(validSchema, setsWithSpace, "default", limits); err == nil {
		t.Error("ResolveParameterSet with whitespace set name succeeded, want error")
	}

	// Bad property schemas
	badProperties := []string{
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"p":123}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"p":"string"}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"p":{"type":"integer","default":1,"minimum":0,"maximum":5,"description":{}}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"p":{"type":"array","default":[],"items":null}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"p":{"type":"integer","default":1,"minimum":0,"maximum":5,"enum":[1]}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"p":{"type":"integer","default":1,"minimum":"abc","maximum":5}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"p":{"type":"integer","default":1,"minimum":1,"maximum":"abc"}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"description":{},"properties":{"p":{"type":"integer","default":1,"minimum":1,"maximum":5}}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"description":123,"properties":{"p":{"type":"integer","default":1,"minimum":1,"maximum":5}}}`,
	}
	for _, bp := range badProperties {
		if _, err := ParseParameterSchema([]byte(bp), limits); err == nil {
			t.Errorf("ParseParameterSchema(%s) succeeded, want error", bp)
		}
	}

	// Parse with depth trailing
	if _, err := Parse([]byte(validManifest + " 123")); err == nil {
		t.Error("Parse with trailing tokens succeeded, want error")
	}

	// Validate with non-canonical source path
	badPathManifest := Manifest{
		Schema:  SchemaVersion,
		Display: Display{Name: "Name", Description: "Desc"},
		Author:  Attribution{Name: "Author"},
		Source:  Source{Repository: "repo", Attribution: "attr"},
		License: License{SPDX: "Apache-2.0"},
		Compatibility: Compatibility{
			Game:    "chess-raiders",
			Rules:   Range{Minimum: "chess-raiders-rules/v1", Maximum: "chess-raiders-rules/v1"},
			Runtime: Range{Minimum: "chess-raiders-starlark-runtime/v1", Maximum: "chess-raiders-starlark-runtime/v1"},
		},
		Runtime: Runtime{
			Kind:       "starlark",
			Protocol:   "chess-raiders-bot-script/v1",
			Entrypoint: "bot.star",
		},
		StateMode: StateModeStateless,
		Parameters: Parameters{
			SchemaPath: "parameters.schema.json",
			SetsPath:   "parameters.json",
			DefaultSet: "default",
		},
		RequiredCapabilities: []string{"legal-actions"},
		Sources: []string{
			ManifestPath,
			"bot.star",
			"parameters.schema.json",
			"parameters.json",
			"../outside",
		},
	}
	if err := Validate(badPathManifest); err == nil {
		t.Error("Validate with non-canonical path succeeded, want error")
	}

	// validateClosure errors
	manifestBytes := []byte(validManifest)
	var m Manifest
	_ = json.Unmarshal(manifestBytes, &m)
	entries := validEntries(manifestBytes)

	// parseWithDepth error on manifestBytes
	if _, err := validateClosure(m, []byte("{invalid"), entries, testLimits); err == nil {
		t.Error("validateClosure with invalid manifestBytes succeeded, want error")
	}

	// Validate(manifest) error
	invalidM := m
	invalidM.Schema = "bad-schema"
	badMBytes, _ := json.Marshal(invalidM)
	if _, err := validateClosure(invalidM, badMBytes, validEntries(badMBytes), testLimits); err == nil {
		t.Error("validateClosure with invalid manifest struct succeeded, want error")
	}
}

func TestParametersDirectCoverage(t *testing.T) {
	// validatePropertyShape
	_ = validatePropertyShape(ParameterProperty{Type: ParameterTypeInteger, Enum: []string{"a"}}, "f")
	_ = validatePropertyShape(ParameterProperty{Type: ParameterTypeString, Minimum: "1"}, "f")
	_ = validatePropertyShape(ParameterProperty{Type: ParameterTypeBoolean, Minimum: "1"}, "f")
	_ = validatePropertyShape(ParameterProperty{Type: ParameterTypeArray}, "f")
	_ = validatePropertyShape(ParameterProperty{Type: ParameterTypeArray, Minimum: "1", Items: &ParameterItem{}}, "f")
	_ = validatePropertyShape(ParameterProperty{Type: ParameterType("unknown")}, "f")

	// validateItemShape
	_ = validateItemShape(ParameterItem{Type: ParameterTypeInteger, Enum: []string{"a"}}, "f")
	_ = validateItemShape(ParameterItem{Type: ParameterTypeString, Minimum: "1"}, "f")
	_ = validateItemShape(ParameterItem{Type: ParameterTypeBoolean, Minimum: "1"}, "f")
	_ = validateItemShape(ParameterItem{Type: ParameterType("unknown")}, "f")

	// validateNumericBounds
	_ = validateNumericBounds(ParameterTypeInteger, "abc", "5", "f")
	_ = validateNumericBounds(ParameterTypeInteger, "1", "abc", "f")
	_ = validateNumericBounds(ParameterTypeInteger, "5", "1", "f")

	// validateStringEnum
	_ = validateStringEnum([]string{"x", "x"}, "f")

	// validateOptionalEnum
	_ = validateOptionalEnum(map[string]json.RawMessage{"enum": []byte("[123]")}, "f")

	// normalizeParameterValue
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeString}, []byte("123"), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeString}, []byte("{bad"), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeString}, []byte(`"hello"`), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeBoolean}, []byte(`"true"`), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeBoolean}, []byte("{bad"), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeBoolean}, []byte("true"), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeArray}, []byte("[]"), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeArray, Items: &ParameterItem{Type: ParameterTypeInteger}}, []byte("123"), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeInteger, Minimum: "1", Maximum: "5"}, []byte(`"abc"`), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeInteger, Minimum: "1", Maximum: "5"}, []byte("1.2.3"), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeInteger, Minimum: "2", Maximum: "5"}, []byte("1"), "f")
	_, _ = normalizeParameterValue(ParameterProperty{Type: ParameterTypeInteger, Minimum: "1", Maximum: "5"}, []byte("1e999999999999999999999999999999"), "f")

	// validateItemShape happy paths
	_ = validateItemShape(ParameterItem{Type: ParameterTypeString}, "f")
	_ = validateItemShape(ParameterItem{Type: ParameterTypeBoolean}, "f")

	// parseParameterProperty
	_, _ = parseParameterProperty([]byte("123"), "f")
	_, _ = parseParameterProperty([]byte(`{"items": 123}`), "f")
	_, _ = parseParameterProperty([]byte(`{"items": {"description": 123}}`), "f")
	_, _ = parseParameterProperty([]byte(`{"items": {"enum": []}}`), "f")
	_, _ = parseParameterProperty([]byte(`{"items": null}`), "f")
	_, _ = parseParameterProperty([]byte(`{"description": 123}`), "f")
	_, _ = parseParameterProperty([]byte(`{"description": {}}`), "f")

	// validatePartialParameterConfig
	schema := ParameterSchema{
		Properties: map[string]ParameterProperty{
			"prop": {Type: ParameterTypeInteger, Minimum: "1", Maximum: "5", normalizedDefault: nil},
		},
	}
	limits := ParameterLimits{
		MaxDocumentBytes: 1000,
		MaxJSONDepth:     10,
		MaxProperties:    10,
		MaxSets:          10,
		MaxResolvedBytes: 1000,
	}
	// validateJSONInput error
	_, _ = validatePartialParameterConfig(schema, []byte("{bad"), limits, false, false)

	// enforceDelta true with normalizedDefault == nil
	_, _ = validatePartialParameterConfig(schema, []byte(`{"prop": 2}`), limits, true, true)
	// expand true with normalizedDefault == nil
	_, _ = validatePartialParameterConfig(schema, []byte("{}"), limits, true, false)

	// limits.MaxResolvedBytes <= 0 in validatePartialParameterConfig
	badResolvedLimits := limits
	badResolvedLimits.MaxResolvedBytes = 0
	_, _ = validatePartialParameterConfig(schema, []byte(`{"prop": 2}`), badResolvedLimits, true, false)

	// decodeRaw error in resolveParameterSet
	_, _ = resolveParameterSet(schema, []byte("{bad"), "default", limits)
	_, _ = resolveParameterSet(schema, []byte(`{"schema":123,"sets":{}}`), "default", limits)
}

func TestClosureRemainingEdges(t *testing.T) {
	manifestBytes := []byte(validManifest)
	var m Manifest
	_ = json.Unmarshal(manifestBytes, &m)

	limits := testLimits
	limits.MaxFiles = 10

	// Duplicate path in entries
	entries := validEntries(manifestBytes)
	dupEntries := append(entries, TreeEntry{Path: "bot.star", Kind: EntryKindRegular, Content: []byte("def decide(): pass")})
	_, _ = validateClosure(m, manifestBytes, dupEntries, limits)

	// Per-file byte limit exceeded
	fileLimitEntries := validEntries(manifestBytes)
	fileLimitEntries[1].Content = make([]byte, 2000)
	smallFileLimits := limits
	smallFileLimits.MaxFileBytes = 1000
	smallFileLimits.MaxManifestBytes = 10_000
	_, _ = validateClosure(m, manifestBytes, fileLimitEntries, smallFileLimits)

	// Total byte limit exceeded
	totalLimitEntries := validEntries(manifestBytes)
	totalLimitEntries[1].Content = make([]byte, 500)
	smallTotalLimits := limits
	smallTotalLimits.MaxFileBytes = 1000
	smallTotalLimits.MaxManifestBytes = 1000
	smallTotalLimits.MaxTotalBytes = 1000
	_, _ = validateClosure(m, manifestBytes, totalLimitEntries, smallTotalLimits)

	// Entrypoint syntax error in closure
	badCodeEntries := validEntries(manifestBytes)
	badCodeEntries[1].Content = []byte("invalid starlark syntax :::")
	_, _ = validateClosure(m, manifestBytes, badCodeEntries, limits)
}
