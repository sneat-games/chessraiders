// Copyright 2026 Sneat.app

package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
	"unicode"
)

// MaximumJSONDepth is the parser's hard safety ceiling. Callers still choose a
// positive limit at or below it for each untrusted artifact.
const MaximumJSONDepth = 256

// Parse parses a structurally strict manifest: unknown fields and trailing JSON
// are rejected so a field a newer producer relies on cannot be silently
// ignored by an older validator. Parse does not establish execution or
// competition eligibility for untrusted input; callers must use
// ValidateArtifact with their own CompatibilityProfile and ValidationLimits.
func Parse(raw []byte) (Manifest, error) {
	return parseWithDepth(raw, MaximumJSONDepth)
}

func parseWithDepth(raw []byte, maxDepth int) (Manifest, error) {
	if err := scanJSON(raw, maxDepth, true); err != nil {
		return Manifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, jsonError(err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Manifest{}, err
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return &Error{Code: ErrInvalidJSON, Detail: "contains more than one JSON value"}
	}
	return jsonError(err)
}

func jsonError(err error) error {
	code := ErrInvalidJSON
	if strings.Contains(err.Error(), "unknown field") {
		code = ErrUnknownField
	}
	return &Error{Code: code, Detail: err.Error()}
}

// scanStrictJSON runs before ordinary decoding because encoding/json otherwise
// accepts case-insensitive struct fields and keeps only a final duplicate key.
// The depth ceiling makes this preflight safe on untrusted raw input.
func scanJSON(raw []byte, maxDepth int, enforceManifestFieldCase bool) error {
	if maxDepth <= 0 || maxDepth > MaximumJSONDepth {
		return &Error{Code: ErrInvalidLimit, Field: "maxJSONDepth", Detail: fmt.Sprintf("must be between 1 and %d", MaximumJSONDepth)}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, "", 1, maxDepth, enforceManifestFieldCase); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func scanJSONValue(decoder *json.Decoder, objectPath string, depth, maxDepth int, enforceManifestFieldCase bool) error {
	if depth > maxDepth {
		return &Error{Code: ErrJSONDepth, Field: objectPath, Detail: "exceeds the caller's JSON nesting limit"}
	}
	token, err := decoder.Token()
	if err != nil {
		return jsonError(err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		seenFolded := map[string]string{}
		expected := map[string]string(nil)
		if enforceManifestFieldCase {
			expected = expectedFields(objectPath)
		}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return jsonError(err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return &Error{Code: ErrInvalidJSON, Detail: "object key is not a string"}
			}
			if _, exists := seen[key]; exists {
				return &Error{Code: ErrInvalidJSON, Detail: "duplicate object key " + key}
			}
			foldedKey := foldJSONKey(key)
			if prior, exists := seenFolded[foldedKey]; exists {
				return &Error{Code: ErrInvalidJSON, Field: objectPath, Detail: "case-fold-equivalent duplicate object keys " + prior + " and " + key}
			}
			if canonical, found := exactCaseField(expected, key); found && canonical != key {
				return &Error{Code: ErrUnknownField, Field: objectPath, Detail: "field " + key + " must use exact case " + canonical}
			}
			seen[key] = struct{}{}
			seenFolded[foldedKey] = key
			if err := scanJSONValue(decoder, childObjectPath(objectPath, key), depth+1, maxDepth, enforceManifestFieldCase); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return jsonError(err)
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, objectPath, depth+1, maxDepth, enforceManifestFieldCase); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return jsonError(err)
		}
	}
	return nil
}

func exactCaseField(expected map[string]string, key string) (string, bool) {
	canonical, found := expected[foldJSONKey(key)]
	return canonical, found
}

// foldJSONKey returns the canonical representative of each Unicode simple-
// fold equivalence class. It matches strings.EqualFold semantics without an
// O(n²) prior-key scan, so aliases such as the long-s in "ſchema" cannot
// evade exact fixed-field checks or duplicate detection.
func foldJSONKey(value string) string {
	return strings.Map(func(character rune) rune {
		canonical := character
		for folded := unicode.SimpleFold(character); folded != character; folded = unicode.SimpleFold(folded) {
			if folded < canonical {
				canonical = folded
			}
		}
		return canonical
	}, value)
}

func childObjectPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func expectedFields(path string) map[string]string {
	fields := func(values ...string) map[string]string {
		result := make(map[string]string, len(values))
		for _, value := range values {
			result[foldJSONKey(value)] = value
		}
		return result
	}
	switch path {
	case "":
		return fields("schema", "display", "author", "source", "license", "compatibility", "runtime", "stateMode", "parameters", "requiredCapabilities", "sources")
	case "display":
		return fields("name", "description")
	case "author":
		return fields("name", "url")
	case "source":
		return fields("repository", "attribution")
	case "license":
		return fields("spdx")
	case "compatibility":
		return fields("game", "rules", "runtime")
	case "compatibility.rules", "compatibility.runtime":
		return fields("minimum", "maximum")
	case "runtime":
		return fields("kind", "protocol", "entrypoint")
	case "parameters":
		return fields("schemaPath", "setsPath", "defaultSet")
	default:
		return nil
	}
}

// Validate checks only declarative facts. It does not open a repository,
// launch a runtime, or select a competition/provider policy.
func Validate(m Manifest) error {
	if m.Schema != SchemaVersion {
		return invalid("schema", "want "+SchemaVersion)
	}
	if blank(m.Display.Name) || blank(m.Display.Description) {
		return invalid("display", "name and description are required")
	}
	if blank(m.Author.Name) || blank(m.Source.Repository) || blank(m.Source.Attribution) || blank(m.License.SPDX) {
		return invalid("author/source/license", "author name, source repository, source attribution and SPDX licence are required")
	}
	if m.Compatibility.Game != "chess-raiders" || !qualified(m.Compatibility.Rules.Minimum) || !qualified(m.Compatibility.Rules.Maximum) || !qualified(m.Compatibility.Runtime.Minimum) || !qualified(m.Compatibility.Runtime.Maximum) {
		return invalid("compatibility", "game must be chess-raiders and rules/runtime ranges must use qualified protocols")
	}
	if m.Runtime.Kind != "starlark" || !qualified(m.Runtime.Protocol) {
		return invalid("runtime", "kind must be starlark and protocol must be qualified")
	}
	if _, err := canonicalPath(m.Runtime.Entrypoint); err != nil {
		return pathError("runtime.entrypoint", err)
	}
	if m.StateMode != StateModeStateless && m.StateMode != StateModeBattleStateful {
		return invalid("stateMode", "must be stateless or battle-stateful")
	}
	if _, err := canonicalPath(m.Parameters.SchemaPath); err != nil {
		return pathError("parameters.schemaPath", err)
	}
	if _, err := canonicalPath(m.Parameters.SetsPath); err != nil {
		return pathError("parameters.setsPath", err)
	}
	if blank(m.Parameters.DefaultSet) {
		return invalid("parameters.defaultSet", "is required")
	}
	if m.Parameters.SchemaPath == m.Parameters.SetsPath {
		return invalid("parameters", "schemaPath and setsPath must name different files")
	}
	if len(m.RequiredCapabilities) != len(uniqueStrings(m.RequiredCapabilities)) {
		return invalid("requiredCapabilities", "contains a duplicate capability")
	}
	for _, capability := range m.RequiredCapabilities {
		if blank(capability) {
			return invalid("requiredCapabilities", "contains an empty capability")
		}
	}
	if len(m.Sources) == 0 {
		return invalid("sources", "must declare the complete executable closure")
	}
	seen := make(map[string]struct{}, len(m.Sources))
	for _, source := range m.Sources {
		canonical, err := canonicalPath(source)
		if err != nil {
			return pathError("sources", err)
		}
		if _, exists := seen[canonical]; exists {
			return &Error{Code: ErrDuplicatePath, Path: canonical, Detail: "is declared more than once"}
		}
		seen[canonical] = struct{}{}
	}
	for _, required := range []string{ManifestPath, m.Runtime.Entrypoint, m.Parameters.SchemaPath, m.Parameters.SetsPath} {
		if _, exists := seen[required]; !exists {
			return &Error{Code: ErrMissingFile, Path: required, Detail: "is required by the manifest but not declared in sources"}
		}
	}
	return nil
}

// exactDecimal accepts a JSON number and preserves its mathematical value as
// a rational. It intentionally avoids float64: 2^60 and 2^60+1 must not
// collapse into one value while deciding whether a manifest default is inside
// its declared range.
func exactDecimal(value string) (*big.Rat, error) {
	if value == "" {
		return nil, fmt.Errorf("is required")
	}
	index := 0
	negative := false
	if value[index] == '-' {
		negative = true
		index++
		if index == len(value) {
			return nil, fmt.Errorf("is not a JSON number")
		}
	}
	integerStart := index
	if value[index] == '0' {
		index++
	} else if value[index] >= '1' && value[index] <= '9' {
		for index < len(value) && value[index] >= '0' && value[index] <= '9' {
			index++
		}
	} else {
		return nil, fmt.Errorf("is not a JSON number")
	}
	integer := value[integerStart:index]
	fraction := ""
	if index < len(value) && value[index] == '.' {
		index++
		fractionStart := index
		for index < len(value) && value[index] >= '0' && value[index] <= '9' {
			index++
		}
		if fractionStart == index {
			return nil, fmt.Errorf("is not a JSON number")
		}
		fraction = value[fractionStart:index]
	}
	exponent := 0
	if index < len(value) && (value[index] == 'e' || value[index] == 'E') {
		index++
		sign := 1
		if index < len(value) && (value[index] == '+' || value[index] == '-') {
			if value[index] == '-' {
				sign = -1
			}
			index++
		}
		exponentStart := index
		for index < len(value) && value[index] >= '0' && value[index] <= '9' {
			index++
		}
		if exponentStart == index {
			return nil, fmt.Errorf("is not a JSON number")
		}
		exponentValue, ok := new(big.Int).SetString(value[exponentStart:index], 10)
		if !ok || !exponentValue.IsInt64() || exponentValue.Int64() > 10_000 {
			return nil, fmt.Errorf("has an unsupported exponent")
		}
		exponent = sign * int(exponentValue.Int64())
	}
	if index != len(value) {
		return nil, fmt.Errorf("is not a JSON number")
	}
	digits := integer + fraction
	numerator, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, fmt.Errorf("is not a JSON number")
	}
	if negative {
		numerator.Neg(numerator)
	}
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(fraction))), nil)
	if exponent >= 0 {
		numerator.Mul(numerator, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil))
	} else {
		denominator.Mul(denominator, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-exponent)), nil))
	}
	return new(big.Rat).SetFrac(numerator, denominator), nil
}

func blank(value string) bool { return strings.TrimSpace(value) == "" }

func qualified(protocol string) bool {
	return strings.Contains(protocol, "/") && !strings.HasPrefix(protocol, "/") && !strings.HasSuffix(protocol, "/")
}

func invalid(field, detail string) error {
	return &Error{Code: ErrInvalidManifest, Field: field, Detail: detail}
}

func pathError(field string, err error) error {
	return &Error{Code: ErrInvalidPath, Field: field, Detail: err.Error()}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			unique = append(unique, value)
		}
	}
	return unique
}

func canonicalPath(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return "", fmt.Errorf("must be a relative POSIX path")
	}
	for _, character := range value {
		if character <= 0x1f || character == 0x7f {
			return "", fmt.Errorf("must not contain ASCII control characters")
		}
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("must be canonical without empty, dot or dot-dot segments")
		}
	}
	return value, nil
}
