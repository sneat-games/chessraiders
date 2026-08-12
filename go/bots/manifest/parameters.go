// Copyright 2026 Sneat.app

package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	// JSONSchemaDraft202012 is the dialect used by the deliberately small
	// parameter-schema profile below.
	JSONSchemaDraft202012 = "https://json-schema.org/draft/2020-12/schema"

	// ParameterSetsSchemaVersion identifies the portable named-partial-set
	// envelope, not a Chess Raiders release or a Go module release.
	ParameterSetsSchemaVersion = "chess-raiders-bot-parameter-sets/v1"
)

// ParameterLimits bound every public parameter parser and resolver. They are
// caller policy: this portable package owns no Cup- or Season-specific values.
// MaxDocumentBytes applies independently to the schema and sets documents;
// MaxResolvedBytes caps the one complete runtime-facing object returned.
type ParameterLimits struct {
	MaxDocumentBytes int64
	MaxJSONDepth     int
	MaxProperties    int
	MaxSets          int
	MaxResolvedBytes int64
}

type ParameterType string

const (
	ParameterTypeNumber  ParameterType = "number"
	ParameterTypeInteger ParameterType = "integer"
	ParameterTypeBoolean ParameterType = "boolean"
	ParameterTypeString  ParameterType = "string"
	ParameterTypeArray   ParameterType = "array"
)

// ParameterSchema is the supported JSON Schema draft 2020-12 profile. It is
// intentionally flat: nested objects and schema-composition features are not
// part of the bot parameter contract. Every JSON object key is exact, including
// developer-chosen property and set names, and one object may not contain two
// names that are Unicode simple-fold equivalents.
type ParameterSchema struct {
	Schema               string                       `json:"$schema"`
	Type                 ParameterType                `json:"type"`
	Description          string                       `json:"description,omitempty"`
	Properties           map[string]ParameterProperty `json:"properties"`
	AdditionalProperties *bool                        `json:"additionalProperties"`
}

type ParameterProperty struct {
	Type        ParameterType   `json:"type"`
	Description string          `json:"description,omitempty"`
	Default     json.RawMessage `json:"default"`
	Minimum     json.Number     `json:"minimum,omitempty"`
	Maximum     json.Number     `json:"maximum,omitempty"`
	Enum        []string        `json:"enum,omitempty"`
	Items       *ParameterItem  `json:"items,omitempty"`
}

type ParameterItem struct {
	Type        ParameterType `json:"type"`
	Description string        `json:"description,omitempty"`
	Minimum     json.Number   `json:"minimum,omitempty"`
	Maximum     json.Number   `json:"maximum,omitempty"`
	Enum        []string      `json:"enum,omitempty"`
}

// ParameterSets is a versioned collection of named partial parameter objects.
// Each set is resolved through the same schema; omitted properties receive the
// schema default.
type ParameterSets struct {
	Schema string                     `json:"schema"`
	Sets   map[string]json.RawMessage `json:"sets"`
}

// ParseParameterSchema parses and validates the supported JSON Schema profile.
// It applies caller-supplied raw byte and nesting limits before decoding.
func ParseParameterSchema(raw []byte, limits ParameterLimits) (ParameterSchema, error) {
	if err := validateParameterLimits(limits, false); err != nil {
		return ParameterSchema{}, err
	}
	if err := validateJSONInput(raw, limits.MaxDocumentBytes, limits.MaxJSONDepth); err != nil {
		return ParameterSchema{}, err
	}
	root, err := rawObject(raw)
	if err != nil {
		return ParameterSchema{}, parameterError(ErrParameterSchema, "", err.Error())
	}
	if err := rejectUnknownFields(root, "$schema", "type", "description", "properties", "additionalProperties"); err != nil {
		return ParameterSchema{}, parameterError(ErrParameterSchema, "schema", err.Error())
	}

	var schema ParameterSchema
	if err := decodeRaw(raw, &schema); err != nil {
		return ParameterSchema{}, parameterError(ErrParameterSchema, "schema", err.Error())
	}
	if err := requireOptionalString(root, "description", "description"); err != nil {
		return ParameterSchema{}, err
	}
	propertiesRaw, ok := root["properties"]
	if !ok {
		return ParameterSchema{}, parameterError(ErrParameterSchema, "properties", "is required")
	}
	propertyObjects, err := rawObject(propertiesRaw)
	if err != nil {
		return ParameterSchema{}, parameterError(ErrParameterSchema, "properties", "must be an object")
	}
	if len(propertyObjects) > limits.MaxProperties {
		return ParameterSchema{}, parameterError(ErrParameterLimit, "properties", "schema declares more properties than the caller permits")
	}
	properties := make(map[string]ParameterProperty, len(propertyObjects))
	propertyNames := make([]string, 0, len(propertyObjects))
	for name := range propertyObjects {
		propertyNames = append(propertyNames, name)
	}
	sort.Strings(propertyNames)
	for _, name := range propertyNames {
		if strings.TrimSpace(name) == "" {
			return ParameterSchema{}, parameterError(ErrParameterSchema, "properties", "contains an empty property name")
		}
		property, err := parseParameterProperty(propertyObjects[name], "properties."+name)
		if err != nil {
			return ParameterSchema{}, err
		}
		properties[name] = property
	}
	schema.Properties = properties
	if err := validateParameterSchema(schema); err != nil {
		return ParameterSchema{}, err
	}
	return schema, nil
}

// ResolveParameterConfig validates one partial configuration against rawSchema
// and returns a deterministic complete JSON object with every omitted default
// filled. Both documents are independently bounded before decoding.
func ResolveParameterConfig(rawSchema, raw []byte, limits ParameterLimits) (json.RawMessage, error) {
	schema, err := ParseParameterSchema(rawSchema, limits)
	if err != nil {
		return nil, err
	}
	return validatePartialParameterConfig(schema, raw, limits, true, false)
}

// ResolveParameterSet validates every named partial set, then expands only
// selected. Work is O(schema properties + supplied override bytes), never the
// Cartesian product of properties and sets. Iteration is sorted so the first
// reported invalid set is stable.
func ResolveParameterSet(rawSchema, raw []byte, selected string, limits ParameterLimits) (json.RawMessage, error) {
	if err := validateParameterLimits(limits, true); err != nil {
		return nil, err
	}
	schema, err := ParseParameterSchema(rawSchema, limits)
	if err != nil {
		return nil, err
	}
	return resolveParameterSet(schema, raw, selected, limits)
}

func resolveParameterSet(schema ParameterSchema, raw []byte, selected string, limits ParameterLimits) (json.RawMessage, error) {
	if err := validateJSONInput(raw, limits.MaxDocumentBytes, limits.MaxJSONDepth); err != nil {
		return nil, err
	}
	root, err := rawObject(raw)
	if err != nil {
		return nil, parameterError(ErrParameterConfig, "sets", "must be an object")
	}
	if err := rejectUnknownFields(root, "schema", "sets"); err != nil {
		return nil, parameterError(ErrParameterConfig, "sets", err.Error())
	}
	var envelope ParameterSets
	if err := decodeRaw(raw, &envelope); err != nil {
		return nil, parameterError(ErrParameterConfig, "sets", err.Error())
	}
	if envelope.Schema != ParameterSetsSchemaVersion {
		return nil, parameterError(ErrParameterConfig, "sets.schema", "want "+ParameterSetsSchemaVersion)
	}
	if len(envelope.Sets) == 0 {
		return nil, parameterError(ErrParameterConfig, "sets", "must contain at least one named configuration")
	}
	if len(envelope.Sets) > limits.MaxSets {
		return nil, parameterError(ErrParameterLimit, "sets", "document declares more named sets than the caller permits")
	}
	if strings.TrimSpace(selected) == "" {
		return nil, parameterError(ErrParameterConfig, "selected", "set name is required")
	}

	names := make([]string, 0, len(envelope.Sets))
	for name := range envelope.Sets {
		names = append(names, name)
	}
	sort.Strings(names)
	var resolved json.RawMessage
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return nil, parameterError(ErrParameterConfig, "sets", "contains an empty set name")
		}
		row, err := validatePartialParameterConfig(schema, envelope.Sets[name], limits, name == selected, true)
		if err != nil {
			return nil, parameterSetError(name, err)
		}
		if name == selected {
			resolved = row
		}
	}
	if resolved == nil {
		return nil, parameterError(ErrParameterConfig, "selected", "named set is absent")
	}
	return resolved, nil
}

func parameterSetError(name string, err error) error {
	var validationError *Error
	if !errors.As(err, &validationError) {
		return parameterError(ErrParameterConfig, "sets."+name, err.Error())
	}
	copy := *validationError
	if copy.Field == "" {
		copy.Field = "sets." + name
	} else {
		copy.Field = "sets." + name + "." + copy.Field
	}
	return &copy
}

func validatePartialParameterConfig(schema ParameterSchema, raw []byte, limits ParameterLimits, expand, enforceDelta bool) (json.RawMessage, error) {
	if err := validateJSONInput(raw, limits.MaxDocumentBytes, limits.MaxJSONDepth); err != nil {
		return nil, err
	}
	supplied, err := rawObject(raw)
	if err != nil {
		return nil, parameterError(ErrParameterConfig, "configuration", "must be an object")
	}
	if len(supplied) > limits.MaxProperties {
		return nil, parameterError(ErrParameterLimit, "configuration", "supplies more properties than the caller permits")
	}
	names := make([]string, 0, len(supplied))
	for name := range supplied {
		names = append(names, name)
	}
	sort.Strings(names)
	normalizedOverrides := make(map[string]json.RawMessage, len(supplied))
	for _, name := range names {
		property, ok := schema.Properties[name]
		if !ok {
			return nil, parameterError(ErrParameterConfig, name, "is not declared; additionalProperties is false")
		}
		normalized, err := validateParameterValue(property, supplied[name], name)
		if err != nil {
			return nil, err
		}
		if enforceDelta {
			equal, err := parameterValuesEqual(property, supplied[name], property.Default)
			if err != nil {
				return nil, err
			}
			if equal {
				return nil, parameterError(ErrParameterConfig, name, "duplicates its schema default; named sets must contain differences only")
			}
		}
		normalizedOverrides[name] = normalized
	}
	if !expand {
		return json.RawMessage{}, nil
	}
	resolved := make(map[string]json.RawMessage, len(schema.Properties))
	propertyNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propertyNames = append(propertyNames, name)
	}
	sort.Strings(propertyNames)
	resolvedSize := int64(2) // opening and closing braces
	for index, name := range propertyNames {
		property := schema.Properties[name]
		value, ok := normalizedOverrides[name]
		if !ok {
			value, err = validateParameterValue(property, property.Default, name)
			if err != nil {
				return nil, err
			}
		}
		encodedName, _ := json.Marshal(name)
		componentSize := int64(len(encodedName) + 1 + len(value)) // key, colon, value
		if index != 0 {
			componentSize++ // comma
		}
		if componentSize > limits.MaxResolvedBytes-resolvedSize {
			return nil, parameterError(ErrParameterLimit, "configuration", "resolved object exceeds the caller's byte limit")
		}
		resolvedSize += componentSize
		resolved[name] = value
	}
	encoded, err := json.Marshal(resolved)
	if err != nil {
		return nil, parameterError(ErrParameterConfig, "configuration", err.Error())
	}
	if int64(len(encoded)) != resolvedSize || int64(len(encoded)) > limits.MaxResolvedBytes {
		return nil, parameterError(ErrParameterLimit, "configuration", "resolved object exceeds the caller's byte limit")
	}
	return encoded, nil
}

func parseParameterProperty(raw json.RawMessage, field string) (ParameterProperty, error) {
	object, err := rawObject(raw)
	if err != nil {
		return ParameterProperty{}, parameterError(ErrParameterSchema, field, "must be an object")
	}
	if err := rejectUnknownFields(object, "type", "description", "default", "minimum", "maximum", "enum", "items"); err != nil {
		return ParameterProperty{}, parameterError(ErrParameterSchema, field, err.Error())
	}
	var property ParameterProperty
	if err := decodeRaw(raw, &property); err != nil {
		return ParameterProperty{}, parameterError(ErrParameterSchema, field, err.Error())
	}
	if err := requireOptionalString(object, "description", field+".description"); err != nil {
		return ParameterProperty{}, err
	}
	if err := validateOptionalEnum(object, field+".enum"); err != nil {
		return ParameterProperty{}, err
	}
	if property.Type != ParameterTypeInteger && property.Type != ParameterTypeNumber {
		if _, present := object["minimum"]; present {
			return ParameterProperty{}, parameterError(ErrParameterSchema, field+".minimum", "is allowed only for integer or number")
		}
		if _, present := object["maximum"]; present {
			return ParameterProperty{}, parameterError(ErrParameterSchema, field+".maximum", "is allowed only for integer or number")
		}
	}
	if itemsRaw, present := object["items"]; present {
		itemsObject, err := rawObject(itemsRaw)
		if err != nil {
			return ParameterProperty{}, parameterError(ErrParameterSchema, field+".items", "must be an object")
		}
		if err := rejectUnknownFields(itemsObject, "type", "description", "minimum", "maximum", "enum"); err != nil {
			return ParameterProperty{}, parameterError(ErrParameterSchema, field+".items", err.Error())
		}
		if err := requireOptionalString(itemsObject, "description", field+".items.description"); err != nil {
			return ParameterProperty{}, err
		}
		if err := validateOptionalEnum(itemsObject, field+".items.enum"); err != nil {
			return ParameterProperty{}, err
		}
		if property.Items == nil {
			return ParameterProperty{}, parameterError(ErrParameterSchema, field+".items", "must not be null")
		}
		if property.Items.Type != ParameterTypeInteger && property.Items.Type != ParameterTypeNumber {
			if _, present := itemsObject["minimum"]; present {
				return ParameterProperty{}, parameterError(ErrParameterSchema, field+".items.minimum", "is allowed only for integer or number")
			}
			if _, present := itemsObject["maximum"]; present {
				return ParameterProperty{}, parameterError(ErrParameterSchema, field+".items.maximum", "is allowed only for integer or number")
			}
		}
	}
	return property, nil
}

func validateParameterSchema(schema ParameterSchema) error {
	if schema.Schema != JSONSchemaDraft202012 {
		return parameterError(ErrParameterSchema, "$schema", "want "+JSONSchemaDraft202012)
	}
	if schema.Type != "object" {
		return parameterError(ErrParameterSchema, "type", "must be object")
	}
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		return parameterError(ErrParameterSchema, "additionalProperties", "must be present and false")
	}
	if len(schema.Properties) == 0 {
		return parameterError(ErrParameterSchema, "properties", "must declare at least one property")
	}
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		property := schema.Properties[name]
		if len(property.Default) == 0 {
			return parameterError(ErrParameterSchema, "properties."+name+".default", "is required")
		}
		if err := validatePropertyShape(property, "properties."+name); err != nil {
			return err
		}
		if _, err := validateParameterValue(property, property.Default, name); err != nil {
			return parameterError(ErrParameterSchema, "properties."+name+".default", err.Error())
		}
	}
	return nil
}

func validatePropertyShape(property ParameterProperty, field string) error {
	switch property.Type {
	case ParameterTypeInteger, ParameterTypeNumber:
		if property.Items != nil || len(property.Enum) != 0 {
			return parameterError(ErrParameterSchema, field, "numeric properties allow only minimum and maximum constraints")
		}
		return validateNumericBounds(property.Type, property.Minimum, property.Maximum, field)
	case ParameterTypeString:
		if property.Items != nil || property.Minimum != "" || property.Maximum != "" {
			return parameterError(ErrParameterSchema, field, "string properties do not allow numeric bounds or items")
		}
		return validateStringEnum(property.Enum, field+".enum")
	case ParameterTypeBoolean:
		if property.Items != nil || property.Minimum != "" || property.Maximum != "" || len(property.Enum) != 0 {
			return parameterError(ErrParameterSchema, field, "boolean properties do not allow numeric bounds, enum or items")
		}
		return nil
	case ParameterTypeArray:
		if property.Items == nil {
			return parameterError(ErrParameterSchema, field+".items", "is required for an array")
		}
		if property.Minimum != "" || property.Maximum != "" || len(property.Enum) != 0 {
			return parameterError(ErrParameterSchema, field, "array constraints belong to its scalar items")
		}
		return validateItemShape(*property.Items, field+".items")
	default:
		return parameterError(ErrParameterSchema, field+".type", "must be integer, number, string, boolean or array")
	}
}

func validateItemShape(item ParameterItem, field string) error {
	switch item.Type {
	case ParameterTypeInteger, ParameterTypeNumber:
		if len(item.Enum) != 0 {
			return parameterError(ErrParameterSchema, field, "numeric items do not allow enum")
		}
		return validateNumericBounds(item.Type, item.Minimum, item.Maximum, field)
	case ParameterTypeString:
		if item.Minimum != "" || item.Maximum != "" {
			return parameterError(ErrParameterSchema, field, "string items do not allow numeric bounds")
		}
		return validateStringEnum(item.Enum, field+".enum")
	case ParameterTypeBoolean:
		if item.Minimum != "" || item.Maximum != "" || len(item.Enum) != 0 {
			return parameterError(ErrParameterSchema, field, "boolean items do not allow numeric bounds or enum")
		}
		return nil
	default:
		return parameterError(ErrParameterSchema, field+".type", "must be integer, number, string or boolean")
	}
}

func validateNumericBounds(kind ParameterType, minimumNumber, maximumNumber json.Number, field string) error {
	minimum, err := exactDecimal(minimumNumber.String())
	if err != nil {
		return parameterError(ErrParameterSchema, field+".minimum", err.Error())
	}
	maximum, err := exactDecimal(maximumNumber.String())
	if err != nil {
		return parameterError(ErrParameterSchema, field+".maximum", err.Error())
	}
	if minimum.Cmp(maximum) > 0 {
		return parameterError(ErrParameterSchema, field, "minimum must not exceed maximum")
	}
	if kind == ParameterTypeInteger && (!minimum.IsInt() || !maximum.IsInt()) {
		return parameterError(ErrParameterSchema, field, "integer minimum and maximum must be integers")
	}
	return nil
}

func validateStringEnum(values []string, field string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return parameterError(ErrParameterSchema, field, "contains duplicate value "+value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func requireOptionalString(object map[string]json.RawMessage, name, field string) error {
	raw, present := object[name]
	if !present {
		return nil
	}
	var decoded any
	if err := decodeRaw(raw, &decoded); err != nil {
		return parameterError(ErrParameterSchema, field, "must be a string")
	}
	if _, ok := decoded.(string); !ok {
		return parameterError(ErrParameterSchema, field, "must be a string")
	}
	return nil
}

func validateOptionalEnum(object map[string]json.RawMessage, field string) error {
	raw, present := object["enum"]
	if !present {
		return nil
	}
	var values []any
	if err := decodeRaw(raw, &values); err != nil || values == nil || len(values) == 0 {
		return parameterError(ErrParameterSchema, field, "must be a non-empty array of strings")
	}
	for _, value := range values {
		if _, ok := value.(string); !ok {
			return parameterError(ErrParameterSchema, field, "must contain only strings")
		}
	}
	return nil
}

func validateParameterValue(property ParameterProperty, raw json.RawMessage, field string) (json.RawMessage, error) {
	switch property.Type {
	case ParameterTypeInteger, ParameterTypeNumber:
		return validateNumericValue(property.Type, property.Minimum, property.Maximum, raw, field)
	case ParameterTypeString:
		var decoded any
		if err := decodeRaw(raw, &decoded); err != nil {
			return nil, parameterError(ErrParameterConfig, field, "must be a string")
		}
		value, ok := decoded.(string)
		if !ok {
			return nil, parameterError(ErrParameterConfig, field, "must be a string")
		}
		if len(property.Enum) != 0 && !containsString(property.Enum, value) {
			return nil, parameterError(ErrParameterConfig, field, "is not one of the declared enum values")
		}
		encoded, _ := json.Marshal(value)
		return encoded, nil
	case ParameterTypeBoolean:
		var decoded any
		if err := decodeRaw(raw, &decoded); err != nil {
			return nil, parameterError(ErrParameterConfig, field, "must be a boolean")
		}
		value, ok := decoded.(bool)
		if !ok {
			return nil, parameterError(ErrParameterConfig, field, "must be a boolean")
		}
		encoded, _ := json.Marshal(value)
		return encoded, nil
	case ParameterTypeArray:
		var values []json.RawMessage
		if err := decodeRaw(raw, &values); err != nil || values == nil {
			return nil, parameterError(ErrParameterConfig, field, "must be an array")
		}
		normalized := make([]json.RawMessage, len(values))
		for index, value := range values {
			itemProperty := ParameterProperty{
				Type:    property.Items.Type,
				Minimum: property.Items.Minimum,
				Maximum: property.Items.Maximum,
				Enum:    property.Items.Enum,
			}
			item, err := validateParameterValue(itemProperty, value, fmt.Sprintf("%s[%d]", field, index))
			if err != nil {
				return nil, err
			}
			normalized[index] = item
		}
		encoded, err := json.Marshal(normalized)
		if err != nil {
			return nil, parameterError(ErrParameterConfig, field, err.Error())
		}
		return encoded, nil
	default:
		return nil, parameterError(ErrParameterSchema, field, "has unsupported type")
	}
}

func parameterValuesEqual(property ParameterProperty, left, right json.RawMessage) (bool, error) {
	switch property.Type {
	case ParameterTypeInteger, ParameterTypeNumber:
		var leftNumber, rightNumber json.Number
		if err := decodeRaw(left, &leftNumber); err != nil {
			return false, parameterError(ErrParameterConfig, "configuration", err.Error())
		}
		if err := decodeRaw(right, &rightNumber); err != nil {
			return false, parameterError(ErrParameterSchema, "default", err.Error())
		}
		leftValue, err := exactDecimal(leftNumber.String())
		if err != nil {
			return false, err
		}
		rightValue, err := exactDecimal(rightNumber.String())
		if err != nil {
			return false, err
		}
		return leftValue.Cmp(rightValue) == 0, nil
	case ParameterTypeArray:
		var leftValues, rightValues []json.RawMessage
		if err := decodeRaw(left, &leftValues); err != nil {
			return false, err
		}
		if err := decodeRaw(right, &rightValues); err != nil {
			return false, err
		}
		if len(leftValues) != len(rightValues) {
			return false, nil
		}
		itemProperty := ParameterProperty{
			Type:    property.Items.Type,
			Minimum: property.Items.Minimum,
			Maximum: property.Items.Maximum,
			Enum:    property.Items.Enum,
		}
		for index := range leftValues {
			equal, err := parameterValuesEqual(itemProperty, leftValues[index], rightValues[index])
			if err != nil || !equal {
				return equal, err
			}
		}
		return true, nil
	default:
		leftNormalized, err := validateParameterValue(property, left, "configuration")
		if err != nil {
			return false, err
		}
		rightNormalized, err := validateParameterValue(property, right, "default")
		if err != nil {
			return false, err
		}
		return bytes.Equal(leftNormalized, rightNormalized), nil
	}
}

func validateNumericValue(kind ParameterType, minimumNumber, maximumNumber json.Number, raw json.RawMessage, field string) (json.RawMessage, error) {
	var decoded any
	if err := decodeRaw(raw, &decoded); err != nil {
		return nil, parameterError(ErrParameterConfig, field, "must be a number")
	}
	number, ok := decoded.(json.Number)
	if !ok {
		return nil, parameterError(ErrParameterConfig, field, "must be a number")
	}
	value, err := exactDecimal(number.String())
	if err != nil {
		return nil, parameterError(ErrParameterConfig, field, err.Error())
	}
	minimum, _ := exactDecimal(minimumNumber.String())
	maximum, _ := exactDecimal(maximumNumber.String())
	if kind == ParameterTypeInteger && !value.IsInt() {
		return nil, parameterError(ErrParameterConfig, field, "must be integral")
	}
	if value.Cmp(minimum) < 0 || value.Cmp(maximum) > 0 {
		return nil, parameterError(ErrParameterConfig, field, "is outside its declared minimum/maximum")
	}
	if kind == ParameterTypeInteger {
		return json.RawMessage(value.Num().String()), nil
	}
	if strings.ContainsAny(number.String(), ".eE") {
		floatValue, err := strconv.ParseFloat(number.String(), 64)
		if err != nil || math.IsInf(floatValue, 0) || math.IsNaN(floatValue) {
			return nil, parameterError(ErrParameterConfig, field, "is not a finite runtime float")
		}
		if value.Sign() != 0 && floatValue == 0 {
			return nil, parameterError(ErrParameterConfig, field, "nonzero value collapses to zero in the runtime float representation")
		}
	}
	return json.RawMessage(number.String()), nil
}

func validateParameterLimits(limits ParameterLimits, requireSets bool) error {
	if limits.MaxDocumentBytes <= 0 || limits.MaxJSONDepth <= 0 || limits.MaxProperties <= 0 || limits.MaxResolvedBytes <= 0 || (requireSets && limits.MaxSets <= 0) {
		return &Error{Code: ErrInvalidLimit, Detail: "caller must supply positive parameter document, JSON depth, property, set and resolved-byte limits"}
	}
	return nil
}

func validateJSONInput(raw []byte, maxBytes int64, maxDepth int) error {
	if maxBytes <= 0 || maxDepth <= 0 {
		return &Error{Code: ErrInvalidLimit, Detail: "caller must supply positive JSON byte and depth limits"}
	}
	if int64(len(raw)) > maxBytes {
		return &Error{Code: ErrByteLimit, Detail: "JSON exceeds the caller's raw byte limit"}
	}
	return scanJSON(raw, maxDepth, false)
}

func rawObject(raw []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := decodeRaw(raw, &object); err != nil || object == nil {
		if err == nil {
			err = fmt.Errorf("must be an object")
		}
		return nil, err
	}
	return object, nil
}

func decodeRaw(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func rejectUnknownFields(object map[string]json.RawMessage, allowed ...string) error {
	known := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		known[field] = struct{}{}
	}
	for field := range object {
		if _, ok := known[field]; !ok {
			return fmt.Errorf("unknown or incorrectly cased field %s", field)
		}
	}
	return nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func parameterError(code ErrorCode, field, detail string) error {
	return &Error{Code: code, Field: field, Detail: detail}
}
