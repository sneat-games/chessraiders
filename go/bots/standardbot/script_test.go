// Copyright 2026 Sneat.app

package standardbot_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/manifest"
	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

// expectedTiers is the exact public set of named partial configurations:
// three playing difficulties plus the Adviser's non-playing set. The private
// implementation consumes these through ResolveParams; it does not generate
// or own this list.
var expectedTiers = []string{"adviser", "commander", "lieutenant", "recruit"}

// paramsFile is params.json's raw named-partial-set envelope. Runtime-facing
// complete rows come only from ResolveParams after schema validation/default
// filling; this type is used solely to assert the raw public artifact shape.
type paramsFile struct {
	Schema string                     `json:"schema"`
	Sets   map[string]json.RawMessage `json:"sets"`
}

func decodeParams(t *testing.T) paramsFile {
	t.Helper()
	var decoded paramsFile
	if err := json.Unmarshal(standardbot.Params, &decoded); err != nil {
		t.Fatalf("params.json does not parse as JSON: %v", err)
	}
	return decoded
}

// TestParamsJSONHasEveryTierTheFixtureExpects proves params.json carries every
// public named set exactly once and declares the matching envelope version.
func TestParamsJSONHasEveryTierTheFixtureExpects(t *testing.T) {
	decoded := decodeParams(t)
	if decoded.Schema != standardbot.ParameterSetsVersion {
		t.Errorf("params.json schema %q does not match standardbot.ParameterSetsVersion %q", decoded.Schema, standardbot.ParameterSetsVersion)
	}

	var got []string
	for name := range decoded.Sets {
		got = append(got, name)
	}
	sort.Strings(got)

	want := append([]string(nil), expectedTiers...)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("params.json sets = %v, want %v", got, want)
	}
	for i, name := range got {
		if name != want[i] {
			t.Fatalf("params.json sets = %v, want %v", got, want)
		}
	}
}

func TestResolvedRowsPreserveThePublishedStandardBotSemantics(t *testing.T) {
	// These hashes were captured from canonical JSON encodings of the four
	// complete rows before Commander defaults moved into params.schema.json.
	// Hashes pin behavior without introducing a second table of parameter
	// values that could become another source of truth.
	want := map[string]string{
		"adviser":    "103cfefa578bbf4600042e4c4946c6bbd5dc5974cec5a27720d69724db0c1a6d",
		"commander":  "6f3f7079612af0792254b09ac5f08639aa8b45c02ebff56fcef5c6bba8073ed2",
		"lieutenant": "aeb9cceb8070b2f1ebaf9a0bad064f27cf0cf1f6eae1aa9153db78c39199ce16",
		"recruit":    "7e377204f819a65d9886e4a1861871c6520d7415a9d388e5dafee4fbb479ce67",
	}
	for name, wantDigest := range want {
		resolved, err := standardbot.ResolveParams(name)
		if err != nil {
			t.Fatalf("ResolveParams(%q) = %v", name, err)
		}
		got := fmt.Sprintf("%x", sha256.Sum256(resolved))
		if got != wantDigest {
			t.Errorf("ResolveParams(%q) digest = %s, want %s", name, got, wantDigest)
		}
	}

	decoded := decodeParams(t)
	if got := string(decoded.Sets["commander"]); got != "{}" {
		t.Fatalf("raw Commander partial set = %s, want {} so schema defaults remain the one source", got)
	}
}

func TestParameterSchemaDeclaresExactlyTheInputsTheScriptReads(t *testing.T) {
	schema, err := manifest.ParseParameterSchema(standardbot.ParamsSchema, manifest.ParameterLimits{
		MaxDocumentBytes: int64(len(standardbot.ParamsSchema)),
		MaxJSONDepth:     manifest.MaximumJSONDepth,
		MaxProperties:    17,
		MaxResolvedBytes: int64(len(standardbot.ParamsSchema)),
	})
	if err != nil {
		t.Fatalf("ParseParameterSchema() = %v", err)
	}
	reads := map[string]struct{}{}
	for _, match := range regexp.MustCompile(`params\["([^"]+)"\]`).FindAllStringSubmatch(standardbot.Script, -1) {
		reads[match[1]] = struct{}{}
	}
	declared := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		declared = append(declared, name)
	}
	read := make([]string, 0, len(reads))
	for name := range reads {
		read = append(read, name)
	}
	sort.Strings(declared)
	sort.Strings(read)
	if len(declared) != len(read) {
		t.Fatalf("schema properties = %v, script parameter reads = %v", declared, read)
	}
	for index := range declared {
		if declared[index] != read[index] {
			t.Fatalf("schema properties = %v, script parameter reads = %v", declared, read)
		}
	}
}

// TestScriptCompiles proves Script embeds a syntactically valid module under
// go/bots/runtime's own dialect and binds a callable decide — the one
// precondition every other test in this file, and every real caller, relies
// on without re-checking.
func TestScriptCompiles(t *testing.T) {
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatalf("runtime.Compile(standardbot.Script) = %v, want a compiled program", err)
	}
	if !program.HasFunction("decide") {
		t.Fatal(`Script does not bind a callable global named "decide"`)
	}
}

func TestScriptUsesTargetOnlyInterrogationActivity(t *testing.T) {
	if !strings.Contains(standardbot.Script, `"interrogationRemainingMs"`) {
		t.Fatal("script no longer consumes the target-side interrogation timer")
	}
	for _, forbidden := range []string{`"interrogating"`, `"interrogatedBy"`, `"beingInterrogated"`} {
		if strings.Contains(standardbot.Script, forbidden) {
			t.Fatalf("script consumes forbidden king/source-side interrogation field %s", forbidden)
		}
	}
}

func TestScriptTreatsPiecesObjectOrderAsNonSemantic(t *testing.T) {
	if !strings.Contains(standardbot.Script, `sorted(observation["pieces"].keys(), key = square_index)`) {
		t.Fatal("script does not explicitly recover engine square order from the unordered pieces object")
	}
	for _, forbidden := range []string{
		`for square in observation["pieces"]`,
		`for occupied_square in observation["pieces"]`,
	} {
		if strings.Contains(standardbot.Script, forbidden) {
			t.Fatalf("script behavior depends on pieces object member order: %s", forbidden)
		}
	}
}

func TestScriptReadsNumericActorsFromSourceSquareFacts(t *testing.T) {
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatalf("runtime.Compile(standardbot.Script) = %v", err)
	}
	params, err := standardbot.ResolveParams("recruit")
	if err != nil {
		t.Fatalf(`ResolveParams("recruit") = %v`, err)
	}
	const observation = `{
		"lifecycle":"playing", "side":"white", "nowMs":0, "revision":1,
		"ownMorale":0, "ownMoralePenalty":0, "ownManaged":0,
		"pieces":{"a2":{"unitId":9,"side":"white","rank":"pawn","convoy":false,"cargoCount":0,"kingCargo":false,"ghost":false,"refitting":false}},
		"legal":{"a2":["a3"]}, "affordability":{}, "enPassant":{},
		"candidates":{"a2":{"a3":{"destinationVisible":true,"patrolGain":0,"nextPossibleMoves":[],"threatens":[],"threatenedBy":[],"guards":[],"guardedBy":["a2"]}}},
		"deliverySquares":[], "convoyHome":{}, "blockingBase":[],
		"rules":{"veteranProgression":false,"allowsKill":true,"allowsCapture":true,"pieceChargeMs":{},"baseSquares":{},"beaconEnabled":false,"beaconForgeEnabled":false,"beaconKingStartsAsBearer":false,"specialistsEnabled":false,"woodWallsEnabled":false,"stoneWallsEnabled":false},
		"systems":{"training":false,"walls":false,"beacon":false,"prisoners":false,"morale":false,"espionage":false},
		"beacon":{"lifecycle":"undeployed","bearerSquare":"","everHandedOff":false}, "walls":[], "enemyManaged":0
	}`

	result, err := program.Call("decide", observation, `{}`, string(params), `0`)
	if err != nil {
		t.Fatalf("decide() with numeric actor and source-square facts = %v", err)
	}
	decision := decideThreeTuple(t, result)
	if !strings.Contains(string(decision[0]), `"from":"a2"`) {
		t.Fatalf("decide() = %s, want the numeric actor's move from source square a2", decision[0])
	}
}

// emptyBoardObservation is the smallest observation build_board() (chess-raiders-bot.star)
// accepts: a "playing" match with no pieces on either side and no legal
// moves. It deliberately never reaches move_proposals/score_move — proving
// those needs the full board-and-legal-move schema bot-script-contract
// describes, which is the private engine's job to emit and the private
// implementation's job to test against, not this package's. What this DOES
// prove is that every schema-resolved set survives a real decide() call
// without decide() ever erroring on a field it expected and couldn't find.
const emptyBoardObservation = `{
	"lifecycle": "playing",
	"side": "white",
	"nowMs": 0,
	"pieces": {},
	"legal": {},
	"rules": {"veteranProgression": false, "allowsCapture": true}
}`

// decideThreeTuple decodes decide()'s own JSON return shape: a 3-element
// array of (intent, memory, explainedOptions) — see go/bots/runtime's
// Program.Call doc comment on why a multi-value Starlark return arrives as a
// JSON array.
func decideThreeTuple(t *testing.T, result string) []json.RawMessage {
	t.Helper()
	var decision []json.RawMessage
	if err := json.Unmarshal([]byte(result), &decision); err != nil {
		t.Fatalf("decide() returned %s, which does not decode as a 3-tuple: %v", result, err)
	}
	if len(decision) != 3 {
		t.Fatalf("decide() returned %d values (%s), want 3 (intent, memory, explained options)", len(decision), result)
	}
	return decision
}

// TestDecideAcceptsEveryTiersOwnRow calls decide() once per difficulty with
// that difficulty's own params.json row, on the empty board above. Every
// argument — including the params row — is JSON-decoded by go.starlark.net
// before decide() ever runs (Program.Call's own doc comment), so a
// malformed or missing field in params.json fails here as a decode or
// runtime error, not silently.
func TestDecideAcceptsEveryTiersOwnRow(t *testing.T) {
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatalf("runtime.Compile(standardbot.Script) = %v", err)
	}
	for _, tier := range expectedTiers {
		t.Run(tier, func(t *testing.T) {
			row, err := standardbot.ResolveParams(tier)
			if err != nil {
				t.Fatalf("ResolveParams(%q) = %v", tier, err)
			}
			result, err := program.Call("decide", emptyBoardObservation, `{}`, string(row), `0`)
			if err != nil {
				t.Fatalf("decide() with the %s row = %v, want a clean pass on an empty board", tier, err)
			}
			decision := decideThreeTuple(t, result)
			if string(decision[0]) != "null" {
				t.Errorf("decide() with the %s row on an empty board returned intent %s, want null (nothing to command)", tier, decision[0])
			}
		})
	}
}

// TestScriptPassesOutsideThePlayingLifecycle is the cheapest possible proof
// that decide()'s own lifecycle gate still exists and still short-circuits
// before touching params at all: a caller passing a non-"playing" lifecycle
// (a lobby, a finished match) always gets a pass, with an empty params row
// that would fail immediately if the gate below it were ever reached.
func TestScriptPassesOutsideThePlayingLifecycle(t *testing.T) {
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatalf("runtime.Compile(standardbot.Script) = %v", err)
	}
	result, err := program.Call("decide", `{"lifecycle": "lobby"}`, `{}`, `{}`, `0`)
	if err != nil {
		t.Fatalf("decide() outside the playing lifecycle = %v, want a clean pass", err)
	}
	decision := decideThreeTuple(t, result)
	if string(decision[0]) != "null" {
		t.Errorf("decide() outside the playing lifecycle returned intent %s, want null", decision[0])
	}
}

func TestStandardBotCarriesBattleStateThroughItsMemoryContract(t *testing.T) {
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatalf("runtime.Compile(standardbot.Script) = %v", err)
	}
	first, err := program.Call("build_memory", `{"revision":7}`, `{}`, `null`)
	if err != nil {
		t.Fatalf("build_memory(first pass) = %v", err)
	}
	var memory map[string]int64
	if err := json.Unmarshal([]byte(first), &memory); err != nil {
		t.Fatalf("build_memory result %s: %v", first, err)
	}
	if memory["revision"] != 7 || memory["refusedCursor"] != 1 {
		t.Fatalf("first persisted memory = %v, want revision and refusal cursor", memory)
	}
	second, err := program.Call("build_memory", `{"revision":7}`, first, `null`)
	if err != nil {
		t.Fatalf("build_memory(second pass) = %v", err)
	}
	if err := json.Unmarshal([]byte(second), &memory); err != nil {
		t.Fatalf("second build_memory result %s: %v", second, err)
	}
	if memory["refusedCursor"] != 2 {
		t.Fatalf("second persisted memory = %v, want prior battle state to advance", memory)
	}
}
