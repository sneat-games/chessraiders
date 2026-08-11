// Copyright 2026 Sneat.app

package manifest

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

var testJSONLimits = JSONLimits{MaxBytes: 1_000_000, MaxDepth: 32}

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
	resolved, err := ResolveParameterConfig([]byte(scalarParameterSchema), []byte(`{"count":4.0,"mode":"bold"}`), testJSONLimits)
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
			_, err := ResolveParameterConfig([]byte(scalarParameterSchema), []byte(raw), testJSONLimits)
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
			_, err := ResolveParameterConfig([]byte(scalarParameterSchema), []byte(raw), testJSONLimits)
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
		if _, err := ParseParameterSchema([]byte(raw), testJSONLimits); err == nil {
			t.Fatalf("ParseParameterSchema(%s -> %s) = nil, want failure", replacement[0], replacement[1])
		}
	}
}

func TestParameterRangeKeepsIntegersExactPastFloat64Precision(t *testing.T) {
	// 2^60+1 is distinct from 2^60. A float64 comparison rounds both values
	// to the same number and would accept the second configuration.
	schema := strings.Replace(scalarParameterSchema,
		`"default":4,"minimum":0,"maximum":8`,
		`"default":1152921504606846976,"minimum":0,"maximum":1152921504606846976`,
		1)
	if _, err := ResolveParameterConfig([]byte(schema), []byte(`{"count":1152921504606846976}`), testJSONLimits); err != nil {
		t.Fatalf("ResolveParameterConfig(2^60) = %v, want exact acceptance", err)
	}
	_, err := ResolveParameterConfig([]byte(schema), []byte(`{"count":1152921504606846977}`), testJSONLimits)
	assertParameterError(t, err, ErrParameterConfig)
}

func TestParameterSchemaRejectsUnsupportedOrInexactShapes(t *testing.T) {
	tests := []string{
		strings.Replace(scalarParameterSchema, `"minimum":0,"maximum":8`, `"minimum":0.5,"maximum":8`, 1),
		strings.Replace(scalarParameterSchema, `"additionalProperties":false`, `"additionalProperties":true`, 1),
		strings.Replace(scalarParameterSchema, `"type":"boolean","default":true`, `"type":"object","default":{}`, 1),
		strings.Replace(scalarParameterSchema, `"type":"boolean","default":true`, `"type":"boolean","default":true,"$ref":"#/$defs/x"`, 1),
		strings.Replace(scalarParameterSchema, `"type":"integer","default":4`, `"Type":"integer","default":4`, 1),
	}
	for _, raw := range tests {
		_, err := ParseParameterSchema([]byte(raw), testJSONLimits)
		assertParameterError(t, err, ErrParameterSchema)
	}
}

func TestResolveParameterSetsValidatesEveryNamedPartialRow(t *testing.T) {
	valid := []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{},"bold":{"mode":"bold","count":8}}}`)
	sets, err := ResolveParameterSets([]byte(scalarParameterSchema), valid, testJSONLimits)
	if err != nil {
		t.Fatalf("ResolveParameterSets(valid) = %v", err)
	}
	if len(sets) != 2 || string(sets["default"]) == "{}" {
		t.Fatalf("resolved sets = %#v, want two complete rows", sets)
	}

	invalid := []byte(`{"schema":"chess-raiders-bot-parameter-sets/v1","sets":{"default":{},"broken":{"count":9}}}`)
	_, err = ResolveParameterSets([]byte(scalarParameterSchema), invalid, testJSONLimits)
	assertParameterError(t, err, ErrParameterConfig)
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
