// Copyright 2026 Sneat.app

package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
)

// Parse parses a strict manifest: unknown fields and trailing JSON are
// rejected so a field a newer producer relies on cannot be silently ignored by
// an older validator.
func Parse(raw []byte) (Manifest, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return Manifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
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

// rejectDuplicateJSONKeys runs before ordinary decoding because encoding/json
// otherwise keeps only the final duplicate key. Silent overwrite is not a
// strict source-artifact contract.
func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func scanJSONValue(decoder *json.Decoder) error {
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
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return jsonError(err)
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return jsonError(err)
		}
	}
	return nil
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
	if _, err := canonicalPath(m.Parameters.Path); err != nil {
		return pathError("parameters.path", err)
	}
	if err := validateParameters(m.Parameters); err != nil {
		return err
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
	for _, required := range []string{ManifestPath, m.Runtime.Entrypoint, m.Parameters.Path} {
		if _, exists := seen[required]; !exists {
			return &Error{Code: ErrMissingFile, Path: required, Detail: "is required by the manifest but not declared in sources"}
		}
	}
	return nil
}

func validateParameters(parameters Parameters) error {
	if len(parameters.Declarations) == 0 {
		return invalid("parameters.declarations", "must declare at least one parameter")
	}
	seen := map[string]struct{}{}
	for _, declaration := range parameters.Declarations {
		if blank(declaration.Name) {
			return invalid("parameters.declarations", "parameter name is required")
		}
		if _, exists := seen[declaration.Name]; exists {
			return invalid("parameters.declarations", "contains a duplicate parameter name "+declaration.Name)
		}
		seen[declaration.Name] = struct{}{}
		if err := validateParameter(declaration); err != nil {
			return err
		}
	}
	return nil
}

func validateParameter(parameter ParameterDeclaration) error {
	if len(parameter.Default) == 0 || !json.Valid(parameter.Default) {
		return invalid("parameters."+parameter.Name+".default", "must be one valid JSON value")
	}
	if parameter.Range != nil && (math.IsNaN(parameter.Range.Minimum) || math.IsNaN(parameter.Range.Maximum) || math.IsInf(parameter.Range.Minimum, 0) || math.IsInf(parameter.Range.Maximum, 0) || parameter.Range.Minimum > parameter.Range.Maximum) {
		return invalid("parameters."+parameter.Name+".range", "must be finite with minimum no greater than maximum")
	}
	decoder := json.NewDecoder(bytes.NewReader(parameter.Default))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return invalid("parameters."+parameter.Name+".default", err.Error())
	}
	switch parameter.Type {
	case ParameterTypeNumber, ParameterTypeInteger:
		number, ok := value.(json.Number)
		if !ok {
			return invalid("parameters."+parameter.Name+".default", "must match its numeric type")
		}
		value, err := number.Float64()
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return invalid("parameters."+parameter.Name+".default", "must be finite")
		}
		if parameter.Type == ParameterTypeInteger && math.Trunc(value) != value {
			return invalid("parameters."+parameter.Name+".default", "must be an integer")
		}
		if parameter.Range == nil {
			return invalid("parameters."+parameter.Name+".range", "is required for a numeric parameter")
		}
		if value < parameter.Range.Minimum || value > parameter.Range.Maximum {
			return invalid("parameters."+parameter.Name+".default", "must be inside its declared range")
		}
	case ParameterTypeBoolean:
		if _, ok := value.(bool); !ok || parameter.Range != nil {
			return invalid("parameters."+parameter.Name, "boolean parameters require a boolean default and no numeric range")
		}
	case ParameterTypeString:
		if _, ok := value.(string); !ok || parameter.Range != nil {
			return invalid("parameters."+parameter.Name, "string parameters require a string default and no numeric range")
		}
	case ParameterTypeJSON:
		if parameter.Range != nil {
			return invalid("parameters."+parameter.Name+".range", "is only valid for numeric parameters")
		}
	default:
		return invalid("parameters."+parameter.Name+".type", "must be number, integer, boolean, string or json")
	}
	return nil
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
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("must be a relative POSIX path")
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("must be canonical without empty, dot or dot-dot segments")
		}
	}
	return value, nil
}
