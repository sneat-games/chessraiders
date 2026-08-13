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
const standardBotClosureDigest = "sha256:8b68a178792eb9bb250b4a112989fe49a121bf860e5df2f2429f1250d1f947f1"

const standardBotScriptDigest = "2f372be0cdaf9cb1310d968c846121774064d1ec54b381d75d013bc5fb47329a"
