// Copyright 2026 Sneat.app

package standardbot

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:generate go run ./gen

// ResolvedParamsVersion identifies params.resolved.json's own envelope: one
// materialized row per named set in Params, keyed the same way Params' own
// "sets" object is. It is distinct from ParamsVersion (the shape of one
// resolved row) and ParameterSetsVersion (Params' own sparse-delta
// envelope).
const ResolvedParamsVersion = "chess-raiders-bot-resolved-parameters/v1"

// ResolvedParams is params.resolved.json's own content, byte-for-byte: every
// named set in Params (adviser, commander, lieutenant, recruit today) after
// ResolveParams has validated it and filled every omitted schema default,
// wrapped as {"schema": ResolvedParamsVersion, "rows": {name: completeRow}}.
//
// It exists for a consumer that cannot run this package's JSON-Schema
// resolver at load time: Chess Raiders' private webapp's wasmBridge.ts reads
// this file with one synchronous JSON load per tier instead of
// reimplementing manifest.ResolveParameterSet's delta-enforcement and
// default-filling in TypeScript. Params and ParamsSchema remain the single
// AUTHORED source of every value — this file is generated output, produced
// by gen/main.go via the //go:generate directive above, and is never
// hand-edited. resolved_params_test.go fails the build with instructions to
// regenerate it if it ever drifts from what ResolveParams actually returns.
//
// See PARAMS.md for the full picture: the authored layer (params.json,
// ParamsSchema), the resolution layer (ResolveParams), the generated layer
// (this file), who consumes each, and the go generate workflow.
//
//go:embed params.resolved.json
var ResolvedParams []byte

// resolvedParamsFile mirrors params.resolved.json's on-disk shape.
type resolvedParamsFile struct {
	Schema string                     `json:"schema"`
	Rows   map[string]json.RawMessage `json:"rows"`
}

// MarshalResolvedParams derives params.resolved.json's exact bytes by
// calling ResolveParams — this package's existing resolver — once per named
// set declared in Params, then marshaling the results. It performs no
// resolution or default-filling of its own: every value in the returned
// bytes came from ResolveParams. Both gen/main.go (the generator) and
// resolved_params_test.go (the drift test) call this, so the committed
// artifact and the bytes it is checked against can only ever be produced one
// way.
//
// Marshaling is deterministic: json.MarshalIndent on a fixed struct (schema,
// then rows) combined with Go's own sorted map-key encoding (rows' keys, and
// each row's own keys, since ResolveParameterSet returns its resolved object
// as a map) makes two calls with the same inputs byte-identical.
func MarshalResolvedParams() ([]byte, error) {
	names, err := paramsSetNames()
	if err != nil {
		return nil, err
	}
	rows := make(map[string]json.RawMessage, len(names))
	for _, name := range names {
		row, err := ResolveParams(name)
		if err != nil {
			return nil, fmt.Errorf("standardbot: resolve params for marshaling: %w", err)
		}
		rows[name] = json.RawMessage(row)
	}
	encoded, err := json.MarshalIndent(resolvedParamsFile{
		Schema: ResolvedParamsVersion,
		Rows:   rows,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("standardbot: marshal resolved params: %w", err)
	}
	return append(encoded, '\n'), nil
}

// paramsSetNames reads the named-set keys straight out of Params
// (params.json's own "sets" object) instead of hardcoding them a second
// time, so a fifth tier or a rename needs no change here.
func paramsSetNames() ([]string, error) {
	var envelope struct {
		Sets map[string]json.RawMessage `json:"sets"`
	}
	if err := json.Unmarshal(Params, &envelope); err != nil {
		return nil, fmt.Errorf("standardbot: parse params.json: %w", err)
	}
	names := make([]string, 0, len(envelope.Sets))
	for name := range envelope.Sets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
