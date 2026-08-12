// Copyright 2026 Sneat.app

package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/runtime"
)

var testParameterLimits = ParameterLimits{
	MaxDocumentBytes: 1_000_000,
	MaxJSONDepth:     32,
	MaxProperties:    10_000,
	MaxSets:          10_000,
	MaxResolvedBytes: 1_000_000,
}

const scalarParameterSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "description":"profile fixture",
  "properties":{
    "count":{"type":"integer","default":4,"minimum":0,"maximum":8},
    "weight":{"type":"number","default":1.5,"minimum":-10,"maximum":10},
    "mode":{"type":"string","default":"steady","enum":["steady","bold"]},
    "enabled":{"type":"boolean","default":true},
    "lanes":{"type":"array","default":[1,2],"items":{"type":"integer","minimum":0,"maximum":8}}
  },
  "additionalProperties":false
}`

func TestResolveParameterConfigFillsDefaultsAndNormalizesIntegers(t *testing.T) {
	resolved, err := ResolveParameterConfig([]byte(scalarParameterSchema), []byte(`{"count":4.0,"mode":"bold"}`), testParameterLimits)
	if err != nil {
		t.Fatalf("ResolveParameterConfig() = %v", err)
	}
	const want = `{"count":4,"enabled":true,"lanes":[1,2],"mode":"bold","weight":1.5}`
	if string(resolved) != want {
		t.Fatalf("resolved config = %s, want %s", resolved, want)
	}
}

func TestResolveParameterConfigRejectsUnknownWrongAndOutOfRangeValues(t *testing.T) {
	tests := []string{
		`{"unknown":1}`,
		`{"count":"4"}`,
		`{"count":4.5}`,
		`{"count":9}`,
		`{"weight":true}`,
		`{"mode":"unknown"}`,
		`{"enabled":0}`,
		`{"lanes":[1,"2"]}`,
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			_, err := ResolveParameterConfig([]byte(scalarParameterSchema), []byte(raw), testParameterLimits)
			assertParameterError(t, err, ErrParameterConfig)
		})
	}
}

func TestParameterValuesRejectNullForEveryScalarAndArrayItem(t *testing.T) {
	for _, raw := range []string{
		`{"count":null}`,
		`{"weight":null}`,
		`{"mode":null}`,
		`{"enabled":null}`,
		`{"lanes":null}`,
		`{"lanes":[null]}`,
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := ResolveParameterConfig([]byte(scalarParameterSchema), []byte(raw), testParameterLimits)
			assertParameterError(t, err, ErrParameterConfig)
		})
	}
}

func TestParameterSchemaRejectsNullOrWrongDefaultsForEveryType(t *testing.T) {
	replacements := [][2]string{
		{`"default":4`, `"default":null`},
		{`"default":1.5`, `"default":null`},
		{`"default":"steady"`, `"default":null`},
		{`"default":true`, `"default":null`},
		{`"default":[1,2]`, `"default":[null]`},
		{`"default":4`, `"default":"4"`},
		{`"default":1.5`, `"default":false`},
		{`"default":"steady"`, `"default":1`},
		{`"default":true`, `"default":"true"`},
		{`"default":[1,2]`, `"default":{}`},
	}
	for _, replacement := range replacements {
		raw := strings.Replace(scalarParameterSchema, replacement[0], replacement[1], 1)
		if _, err := ParseParameterSchema([]byte(raw), testParameterLimits); err == nil {
			t.Fatalf("ParseParameterSchema(%s -> %s) = nil, want failure", replacement[0], replacement[1])
		}
	}
}

func TestParameterRangeKeepsIntegersExactPastFloat64Precision(t *testing.T) {
	// 2^60+1 is distinct from 2^60. A float64 comparison rounds both values
	// to the same number and would accept the second configuration.
	const maximum uint64 = 1 << 60
	if float64(maximum) != float64(maximum+1) {
		t.Fatal("test vector no longer demonstrates the float64 boundary collapse")
	}
	schema := strings.Replace(scalarParameterSchema,
		`"default":4,"minimum":0,"maximum":8`,
		`"default":1152921504606846976,"minimum":0,"maximum":1152921504606846976`,
		1)
	if _, err := ResolveParameterConfig([]byte(schema), []byte(`{"count":1152921504606846976}`), testParameterLimits); err != nil {
		t.Fatalf("ResolveParameterConfig(2^60) = %v, want exact acceptance", err)
	}
	_, err := ResolveParameterConfig([]byte(schema), []byte(`{"count":1152921504606846977}`), testParameterLimits)
	assertParameterError(t, err, ErrParameterConfig)
}

func TestParameterSchemaRejectsUnsupportedOrInexactShapes(t *testing.T) {
	tests := []struct {
		raw  string
		code ErrorCode
	}{
		{strings.Replace(scalarParameterSchema, `"minimum":0,"maximum":8`, `"minimum":0.5,"maximum":8`, 1), ErrParameterSchema},
		{strings.Replace(scalarParameterSchema, `"additionalProperties":false`, `"additionalProperties":true`, 1), ErrParameterSchema},
		{strings.Replace(scalarParameterSchema, `"type":"boolean","default":true`, `"type":"object","default":{}`, 1), ErrParameterSchema},
		{strings.Replace(scalarParameterSchema, `"type":"boolean","default":true`, `"type":"boolean","default":true,"$ref":"#/$defs/x"`, 1), ErrParameterSchema},
		{strings.Replace(scalarParameterSchema, `"type":"integer","default":4`, `"Type":"integer","default":4`, 1), ErrParameterSchema},
		{strings.Replace(scalarParameterSchema, `"$schema"`, `"$ſchema"`, 1), ErrParameterSchema},
		{strings.Replace(scalarParameterSchema, `"$schema":"https://json-schema.org/draft/2020-12/schema"`, `"$schema":"https://json-schema.org/draft/2020-12/schema","$ſchema":"https://json-schema.org/draft/2020-12/schema"`, 1), ErrInvalidJSON},
	}
	for _, test := range tests {
		_, err := ParseParameterSchema([]byte(test.raw), testParameterLimits)
		assertParameterError(t, err, test.code)
	}
}

func TestDeveloperNamedParameterKeysRejectFoldEquivalentDuplicates(t *testing.T) {
	duplicateProperty := strings.Replace(
		scalarParameterSchema,
		`"count":{"type":"integer","default":4,"minimum":0,"maximum":8}`,
		`"count":{"type":"integer","default":4,"minimum":0,"maximum":8},"Count":{"type":"integer","default":5,"minimum":0,"maximum":8}`,
		1,
	)
	_, err := ParseParameterSchema([]byte(duplicateProperty), testParameterLimits)
	assertParameterError(t, err, ErrInvalidJSON)

	duplicateSet := []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{},"Default":{}}}`)
	_, err = ResolveParameterSet([]byte(scalarParameterSchema), duplicateSet, "default", testParameterLimits)
	assertParameterError(t, err, ErrInvalidJSON)
}

func TestResolveParameterSetValidatesEveryPartialRowButExpandsOnlySelected(t *testing.T) {
	valid := []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{},"bold":{"mode":"bold","count":8}}}`)
	resolved, err := ResolveParameterSet([]byte(scalarParameterSchema), valid, "default", testParameterLimits)
	if err != nil {
		t.Fatalf("ResolveParameterSet(valid) = %v", err)
	}
	if string(resolved) == "{}" || strings.Contains(string(resolved), `"mode":"bold"`) {
		t.Fatalf("resolved selected set = %s, want complete default only", resolved)
	}

	invalid := []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{},"broken":{"count":9}}}`)
	_, err = ResolveParameterSet([]byte(scalarParameterSchema), invalid, "default", testParameterLimits)
	assertParameterError(t, err, ErrParameterConfig)
}

func TestNamedParameterSetsContainOnlySemanticDeltas(t *testing.T) {
	for _, sameAsDefault := range []string{
		`{"count":4.0}`,
		`{"weight":1.50}`,
		`{"mode":"steady"}`,
		`{"enabled":true}`,
		`{"lanes":[1.0,2]}`,
	} {
		raw := []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":` + sameAsDefault + `}}`)
		_, err := ResolveParameterSet([]byte(scalarParameterSchema), raw, "default", testParameterLimits)
		assertParameterError(t, err, ErrParameterConfig)
	}
}

func TestSelectedParameterResolutionIsBoundedWithoutPropertiesTimesSetsExpansion(t *testing.T) {
	const count = 128
	var schema strings.Builder
	schema.WriteString(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{`)
	for index := 0; index < count; index++ {
		if index != 0 {
			schema.WriteByte(',')
		}
		fmt.Fprintf(&schema, `"p%03d":{"type":"integer","default":0,"minimum":0,"maximum":1}`, index)
	}
	schema.WriteString(`},"additionalProperties":false}`)
	var sets strings.Builder
	sets.WriteString(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{`)
	for index := 0; index < count; index++ {
		if index != 0 {
			sets.WriteByte(',')
		}
		fmt.Fprintf(&sets, `"set%03d":{}`, index)
	}
	sets.WriteString(`}}`)

	limits := testParameterLimits
	limits.MaxProperties = count
	limits.MaxSets = count
	resolved, err := ResolveParameterSet([]byte(schema.String()), []byte(sets.String()), "set127", limits)
	if err != nil {
		t.Fatalf("ResolveParameterSet(128 properties, 128 sets) = %v", err)
	}
	var complete map[string]json.RawMessage
	if err := json.Unmarshal(resolved, &complete); err != nil || len(complete) != count {
		t.Fatalf("selected result has %d properties (%v), want %d", len(complete), err, count)
	}

	for _, test := range []struct {
		name   string
		mutate func(*ParameterLimits)
	}{
		{"property count", func(limits *ParameterLimits) { limits.MaxProperties = count - 1 }},
		{"set count", func(limits *ParameterLimits) { limits.MaxSets = count - 1 }},
		{"resolved bytes", func(limits *ParameterLimits) { limits.MaxResolvedBytes = int64(len(resolved) - 1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			bounded := limits
			test.mutate(&bounded)
			_, err := ResolveParameterSet([]byte(schema.String()), []byte(sets.String()), "set127", bounded)
			assertParameterError(t, err, ErrParameterLimit)
		})
	}
}

func TestExportedParameterSetResolverRequiresEveryPositiveLimit(t *testing.T) {
	valid := []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{}}}`)
	for _, test := range []struct {
		name   string
		mutate func(*ParameterLimits)
	}{
		{"document bytes", func(limits *ParameterLimits) { limits.MaxDocumentBytes = 0 }},
		{"JSON depth", func(limits *ParameterLimits) { limits.MaxJSONDepth = 0 }},
		{"properties", func(limits *ParameterLimits) { limits.MaxProperties = 0 }},
		{"sets", func(limits *ParameterLimits) { limits.MaxSets = 0 }},
		{"resolved bytes", func(limits *ParameterLimits) { limits.MaxResolvedBytes = 0 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			limits := testParameterLimits
			test.mutate(&limits)
			_, err := ResolveParameterSet([]byte(scalarParameterSchema), valid, "default", limits)
			assertParameterError(t, err, ErrInvalidLimit)
		})
	}
}

func TestNumberParametersRejectRuntimeFloatOverflowAndUnderflowToZero(t *testing.T) {
	for _, literal := range []string{"1e400", "-1e400", "1e-4000", "-1e-4000"} {
		raw := []byte(`{"weight":` + literal + `}`)
		_, err := ResolveParameterConfig([]byte(scalarParameterSchema), raw, testParameterLimits)
		assertParameterError(t, err, ErrParameterConfig)
	}
}

func TestResolvedIntegersRoundTripExactlyThroughTheRuntime(t *testing.T) {
	program, err := runtime.Compile(`def inspect(params):
    return params["count"]
`)
	if err != nil {
		t.Fatalf("runtime.Compile() = %v", err)
	}
	for _, literal := range []string{
		"1152921504606846977",
		"123456789012345678901234567890123456789012345678901234567890",
	} {
		schema := strings.Replace(scalarParameterSchema,
			`"default":4,"minimum":0,"maximum":8`,
			`"default":0,"minimum":0,"maximum":`+literal,
			1)
		resolved, err := ResolveParameterConfig([]byte(schema), []byte(`{"count":`+literal+`}`), testParameterLimits)
		if err != nil {
			t.Fatalf("ResolveParameterConfig(%s) = %v", literal, err)
		}
		got, err := program.Call("inspect", string(resolved))
		if err != nil {
			t.Fatalf("Program.Call(%s) = %v", literal, err)
		}
		if got != literal {
			t.Fatalf("runtime integer = %s, want exact %s", got, literal)
		}
	}
}

func TestJSONDuplicateTrackingRemainsLinearInObjectCount(t *testing.T) {
	// This bounded many-key vector guards the map-based folded-key tracker. The
	// previous slice scan performed every prior-key comparison for each key.
	var raw strings.Builder
	raw.WriteByte('{')
	for index := 0; index < 5000; index++ {
		if index != 0 {
			raw.WriteByte(',')
		}
		fmt.Fprintf(&raw, "%q:%d", fmt.Sprintf("field%04d", index), index)
	}
	raw.WriteByte('}')
	if err := scanJSON([]byte(raw.String()), 2, false); err != nil {
		t.Fatalf("scanJSON(5000 unique keys) = %v", err)
	}
}

func assertParameterError(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", code)
	}
	var validationError *Error
	if !errors.As(err, &validationError) || validationError.Code != code {
		t.Fatalf("error = %T %v, want code %s", err, err, code)
	}
}
