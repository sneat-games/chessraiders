// Copyright 2026 Sneat.app

// Package standardbot is Chess Raiders' standard bot: Script, the one
// Starlark decide() every difficulty shares (spec/features/standard-bot's
// REQ:one-script-three-difficulties), ParamsSchema, the flat input declaration
// and Commander defaults, and Params, the named partial differences for
// recruit, lieutenant, commander and adviser. Turning Recruit into Lieutenant
// changes parameter data, never chess-raiders-bot.star; see this package's own
// README.md for what one resolved row means and how to change it.
//
// Params carries a fourth named set, adviser, that the same decide() scores
// candidate moves against for explain-mode advice rather than play. It is
// deliberately resolved from the common schema defaults plus its own partial
// set, never inherited from a playing set. It is not a fourth difficulty:
// there is no such thing as "the adviser's own difficulty" in this bot's
// vocabulary, only its own configuration of the same inputs.
//
// The source artifacts are embedded verbatim by the go:embed directives below,
// never copied into Go string literals by hand, so a diff of the four source
// files is the diff against what a compiled bot actually reads. There is no
// intermediate copy for the artifacts to drift apart from.
// Load Script into go/bots/runtime.Compile to get a callable Program; the
// README walks the shortest path from there to a decision.
//
// All four source files originate here. A consumer imports Script and calls
// ResolveParams from a tagged version of this module; it does not republish or
// regenerate a private copy of the strategy or its parameter data.
package standardbot

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/sneat-games/chessraiders/go/bots/manifest"
)

// Script is chess-raiders-bot.star's own content, byte-for-byte: the one decide()
// function every difficulty calls, differing only in which row of Params it
// is handed. It declares no dependency of its own beyond what
// go/bots/runtime's dialect predeclares (json.encode/json.decode) — nothing
// in it reaches outside the observation, legal-move set and params a caller
// hands it.
//
//go:embed chess-raiders-bot.star
var Script string

// Params is params.json's own content, byte-for-byte: four named PARTIAL
// configurations. Commander is the empty object because ParamsSchema owns the
// runnable defaults; every other set contains only its differences. Call
// ResolveParams to validate a set and fill all defaults before handing it to
// Script.
//
//go:embed params.json
var Params []byte

// ParamsSchema is the directly consumable JSON Schema draft 2020-12 document
// declaring every flat input Script reads. It is the sole source of Commander
// defaults; Params does not duplicate them.
//
//go:embed params.schema.json
var ParamsSchema []byte

// Manifest is the strict, portable source-artifact declaration for this exact
// public standard-bot closure. It is an artifact beside Script, ParamsSchema
// and Params, not a second source of any of them: manifest_test.go binds all
// four raw files into one canonical closure.
//
//go:embed chess-raiders-bot.json
var Manifest []byte

// ParamsVersion identifies the COMPLETE resolved row shape handed to Script.
// Moving Commander defaults from full rows into ParamsSchema does not change
// that runtime-facing shape, so the recorded conformance corpus remains on
// this same qualified contract version.
const ParamsVersion = "chess-bot-tier-params/v1"

// ParameterSetsVersion identifies Params' named-partial-set envelope.
const ParameterSetsVersion = manifest.ParameterSetsSchemaVersion

var resolvedParamsCache sync.Map

func parameterLimits() manifest.ParameterLimits {
	maxBytes := len(Params)
	if len(ParamsSchema) > maxBytes {
		maxBytes = len(ParamsSchema)
	}
	return manifest.ParameterLimits{
		MaxDocumentBytes: int64(maxBytes),
		MaxJSONDepth:     manifest.MaximumJSONDepth,
		MaxProperties:    len(ParamsSchema),
		MaxSets:          len(Params),
		MaxResolvedBytes: int64(len(ParamsSchema) + len(Params)),
	}
}

// ResolveParams returns one validated, complete, deterministic parameter row.
// The returned bytes are a copy, so callers cannot mutate the package cache.
func ResolveParams(name string) ([]byte, error) {
	if cached, ok := resolvedParamsCache.Load(name); ok {
		return append([]byte(nil), cached.([]byte)...), nil
	}
	row, err := manifest.ResolveParameterSet(ParamsSchema, Params, name, parameterLimits())
	if err != nil {
		return nil, fmt.Errorf("standardbot: resolve parameter set %q: %w", name, err)
	}
	stored, _ := resolvedParamsCache.LoadOrStore(name, append([]byte(nil), row...))
	return append([]byte(nil), stored.([]byte)...), nil
}
