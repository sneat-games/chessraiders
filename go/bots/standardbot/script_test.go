// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

// expectedTiers is the exact set of rows the private implementation's own
// params.go produces params.json from — RecruitParams/LieutenantParams/
// CommanderParams, the three PLAYING difficulties, plus AdviserParams, the
// Adviser's own row (params.go's own doc comment: deliberately not derived
// from, or shared with, any playing tier's row). A row added or removed
// there without a matching change to this list is exactly the "one list
// beside the thing it mirrors" drift CLAUDE.md warns about, just on this
// side of the publish boundary instead of that one.
var expectedTiers = []string{"adviser", "commander", "lieutenant", "recruit"}

// paramsFile is params.json's own envelope shape: {"version": ...,
// "tiers": {name: {...fields...}}}. Tier rows decode as json.RawMessage,
// not a typed struct — this package deliberately ships no Params struct of
// its own (script.go's own doc comment on the Params variable), so a test
// that needs one row's raw JSON to hand to decide() reaches for the bytes
// directly rather than round-tripping through a type only this file needs.
type paramsFile struct {
	Version string                     `json:"version"`
	Tiers   map[string]json.RawMessage `json:"tiers"`
}

func decodeParams(t *testing.T) paramsFile {
	t.Helper()
	var decoded paramsFile
	if err := json.Unmarshal(standardbot.Params, &decoded); err != nil {
		t.Fatalf("params.json does not parse as JSON: %v", err)
	}
	return decoded
}

// TestParamsJSONHasEveryTierTheFixtureExpects proves params.json parses and
// carries exactly the rows the private fixture generates it from — no tier
// silently dropped in the copy, and none here this package wasn't told to
// expect — plus that its declared envelope version matches the Go constant
// script.go exports for it.
func TestParamsJSONHasEveryTierTheFixtureExpects(t *testing.T) {
	decoded := decodeParams(t)
	if decoded.Version != standardbot.ParamsVersion {
		t.Errorf("params.json version %q does not match standardbot.ParamsVersion %q", decoded.Version, standardbot.ParamsVersion)
	}

	var got []string
	for name := range decoded.Tiers {
		got = append(got, name)
	}
	sort.Strings(got)

	want := append([]string(nil), expectedTiers...)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("params.json tiers = %v, want %v", got, want)
	}
	for i, name := range got {
		if name != want[i] {
			t.Fatalf("params.json tiers = %v, want %v", got, want)
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

// emptyBoardObservation is the smallest observation build_board() (chess-raiders-bot.star)
// accepts: a "playing" match with no pieces on either side and no legal
// moves. It deliberately never reaches move_proposals/score_move — proving
// those needs the full board-and-legal-move schema bot-script-contract
// describes, which is the private engine's job to emit and the private
// implementation's job to test against, not this package's. What this DOES
// prove is that params.json's own shape survives a real decide() call, for
// every tier, without decide() ever erroring on a field it expected and
// couldn't find.
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
	decoded := decodeParams(t)

	for _, tier := range expectedTiers {
		t.Run(tier, func(t *testing.T) {
			row, ok := decoded.Tiers[tier]
			if !ok {
				t.Fatalf("params.json has no %q row", tier)
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
