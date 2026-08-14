// Copyright 2026 Sneat.app

package standardbot_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/manifest"
	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

var standardBotLimits = manifest.ValidationLimits{
	MaxFiles:                  4,
	MaxFileBytes:              200_000,
	MaxTotalBytes:             400_000,
	MaxManifestBytes:          20_000,
	MaxJSONDepth:              32,
	MaxParameterProperties:    32,
	MaxParameterSets:          8,
	MaxResolvedParameterBytes: 20_000,
}

var standardBotProfile = manifest.CompatibilityProfile{
	Game:              "chess-raiders",
	Rules:             manifest.Range{Minimum: "chess-raiders-rules/v1", Maximum: "chess-raiders-rules/v1"},
	Runtime:           manifest.Range{Minimum: "chess-raiders-starlark-runtime/v1", Maximum: "chess-raiders-starlark-runtime/v1"},
	ScriptProtocol:    "chess-raiders-bot-script/v1",
	AllowedStateModes: []manifest.StateMode{manifest.StateModeStateless, manifest.StateModeBattleStateful},
}

func standardBotEntries() []manifest.TreeEntry {
	return []manifest.TreeEntry{
		{Path: manifest.ManifestPath, Kind: manifest.EntryKindRegular, Content: standardbot.Manifest},
		{Path: "chess-raiders-bot.star", Kind: manifest.EntryKindRegular, Content: []byte(standardbot.Script)},
		{Path: "params.schema.json", Kind: manifest.EntryKindRegular, Content: standardbot.ParamsSchema},
		{Path: "params.json", Kind: manifest.EntryKindRegular, Content: standardbot.Params},
	}
}

func TestStandardBotManifestBindsTheExactPublishedClosure(t *testing.T) {
	artifact, err := manifest.ValidateArtifact(standardbot.Manifest, standardBotEntries(), standardBotLimits, standardBotProfile)
	if err != nil {
		t.Fatalf("ValidateArtifact(standard bot) = %v", err)
	}
	if got, want := artifact.Manifest.Runtime.Entrypoint, "chess-raiders-bot.star"; got != want {
		t.Fatalf("entrypoint = %q, want %q", got, want)
	}
	if artifact.Manifest.StateMode != manifest.StateModeBattleStateful {
		t.Fatalf("stateMode = %q, want battle-stateful for the script's persisted focus/refusal memory", artifact.Manifest.StateMode)
	}
	if artifact.Digest.String() != standardBotClosureDigest {
		t.Fatalf("closure digest = %s, want %s", artifact.Digest, standardBotClosureDigest)
	}
	commander, err := standardbot.ResolveParams("commander")
	if err != nil {
		t.Fatalf("ResolveParams(commander) = %v", err)
	}
	if !bytes.Equal(artifact.ResolvedParameters, commander) {
		t.Fatalf("artifact default parameters = %s, want resolved Commander %s", artifact.ResolvedParameters, commander)
	}
	if err := runtime.ValidateEntrypoint(standardbot.Script, "decide", 5); err != nil {
		t.Fatalf("ValidateEntrypoint(standard bot) = %v, want exact five-argument decide", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(standardbot.Script))); got != standardBotScriptDigest {
		t.Fatalf("standard strategy source digest = %s, want preserved %s", got, standardBotScriptDigest)
	}
}

func TestStandardBotManifestRejectsAByteThatDoesNotMatchTheEmbeddedArtifact(t *testing.T) {
	entries := standardBotEntries()
	entries[1].Content = append(entries[1].Content, '\n')
	got := manifest.DigestClosure(map[string][]byte{
		manifest.ManifestPath:    standardbot.Manifest,
		"chess-raiders-bot.star": entries[1].Content,
		"params.schema.json":     standardbot.ParamsSchema,
		"params.json":            standardbot.Params,
	})
	if got.String() == standardBotClosureDigest {
		t.Fatal("a changed script produced the published closure digest")
	}
}

func TestStandardBotManifestClosureRejectsAnUndeclaredFile(t *testing.T) {
	entries := append(standardBotEntries(), manifest.TreeEntry{Path: "notes.txt", Kind: manifest.EntryKindRegular, Content: []byte("not source")})
	limits := standardBotLimits
	limits.MaxFiles = 5
	_, err := manifest.ValidateArtifact(standardbot.Manifest, entries, limits, standardBotProfile)
	if err == nil {
		t.Fatal("ValidateArtifact() = nil, want undeclared-file rejection")
	}
	var validationError *manifest.Error
	if !errors.As(err, &validationError) || validationError.Code != manifest.ErrUndeclaredFile {
		t.Fatalf("ValidateArtifact() error = %T %v, want typed undeclared-file", err, err)
	}
}

// Updated only when an intentional standard-bot artifact change is reviewed.
// This is a closure digest, not the Go module version.
//
// Updated 2026-08-14 (native immutable observation): first-party decisions
// receive the engine's immutable native mapping view directly, with no JSON
// observation decode or temporary conversion dictionaries. Historical corpus
// inputs retain their exact JSON carrier; no parameter or manifest schema
// changed.
//
// Updated 2026-08-13 (bitboard-safe): chess-raiders-bot.star changed — the
// leader guard's twelve placement bitboards are no longer persisted as raw
// 64-bit ints (leaderBoard0..leaderBoard11). Bot memory round-trips as JSON
// int64 through the host, exact on the server, but the browser (wasm) path
// relays it through JavaScript's float64 numbers, safe only up to
// Number.MAX_SAFE_INTEGER (2**53-1): a bitboard with a set bit at square
// index >= 53 silently loses precision, and bit 63 (the sign bit) corrupts
// outright — measured production failure, Practice Bot (Recruit, seed
// 31337): leaderBoard9 carried bit 63 on decision 6, and every later
// leader-guard decision for that bot failed identically from then on
// (json: cannot unmarshal number -9223372036854776000 into Go struct field
// botDecideRequest.memory of type int64). pack_placement_boards /
// unpack_placement_boards now store the same twelve-board placement
// losslessly as five JS-safe leaderBoard0..leaderBoard4 entries (a packed
// nibble-per-square code) instead of twelve raw ones — see that function's
// own doc comment. No params changed; params.resolved.json is untouched.
//
// 2026-08-14 (task convoy-stall, founder production report): a king-cargo
// convoy's own delivery square can be FREE yet walled in by the side's own
// army standing on every one of its king-step approaches — geometry.go's
// blockingOwnBaseSquares (private engine) only ever flags a delivery square
// OCCUPIED by us, never one merely surrounded, so nothing ever scored
// clearing a lane and the convoy sat put (sneat-co/chessraiders' own
// prod-diagnostics.json: id-19 convoy parked on f2, e8 empty but
// d7/d8/e7/f7/f8 all friendly). build_board's own "blocking_base" now
// supersedes the host's blockingBase with delivery_lane_blockers, computed
// from data the observation already carries (deliverySquares/own_by_square/
// enemy_by_square) — see that function's own doc comment. Separately, once
// unblocked, the convoy still ping-ponged between two equal-cost approach
// squares forever (g5<->f6/g6 in the same reproduction): scoring a
// REPLACEMENT candidate's delivery progress against the convoy's CURRENT
// square let two king-step-equidistant destinations tie exactly, and
// COMMITMENT_DECAY's own erosion let the tie "win" back and forth every one
// to two decisions — score_move's delivery branch now measures progress
// against the charge ALREADY in flight (active_charge, its own doc comment)
// instead, so an equal-cost swap scores zero and the convoy commits. No
// params changed; params.resolved.json is untouched.
//
// 2026-08-14 (obs-briefing-limit, founder proposal "stop briefing units the
// bot cannot act on"): the host may now skip an idle (non-charging) own
// unit's legal/candidate/affordability/enPassant hints once the side already
// has rules.maxActiveCommands routes in flight (chess.Rules.MaxActiveCommands,
// default 1 — the bot's own long-standing single-command-front policy, now
// engine-generic and host-configurable) — see spec/features/starlark-bot-
// runtime/bot-script-contract#REQ:not-briefed-is-not-no-legal-moves. Three
// changes to chess-raiders-bot.star: (1) build_board's own move_actionable_units
// narrowing reads rules.maxActiveCommands as a NUMBER (len(charging_units) >=
// max_active_commands) instead of a hard-coded "any charge blocks everything"
// boolean, so it generalizes correctly when the limit is raised above 1; (2)
// has_other_ordinary_choice, the master-engineer prisoner-delivery proposal,
// and the route-retention threshold in decide() itself all now check/skip a
// cell's notBriefed flag (or, for the retention threshold, scope to the
// charging unit's own proposals only) rather than risk conflating "not
// evaluated this decision" with "evaluated, no legal moves"; (3)
// current_morale_need's doc comment now states its scan is scoped to
// currently-briefed units, a deliberate, verified-neutral narrowing, not a
// bug. Follow-up same day: max_active_commands now reads
// observation["rules"].get("maxActiveCommands", 0) or
// UNLIMITED_ACTIVE_COMMANDS instead of a bare direct index — a hand-built
// RulesView that never sets the new field (starlarkbot's own
// task15_regressions_test.go, private repo) sends a bare zero, and
// `len(charging_units) >= 0` is ALWAYS true, i.e. every decision reads as
// already at the limit — the opposite of unlimited, and total paralysis for
// any caller that has not been updated for this field. The safe fallback now
// lives in the script itself, the one place this value is consumed, so
// every present and future producer gets it for free. No params changed;
// params.resolved.json is untouched.
const standardBotClosureDigest = "sha256:8d5e43f769723310c34a78fba1c5b369af821303079da281c8fccb61c16052cd"

const standardBotScriptDigest = "385201fe61a3b9d1b2cc2d7a1318a01f2b85112f06104dff9918f6eccf27bca1"
