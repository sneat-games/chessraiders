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
const standardBotClosureDigest = "sha256:7029fbd941db2843c9de6a2b71c01c5cd181e191701b905f8591b0f24e46b628"

const standardBotScriptDigest = "201a96ed9c124840984a1cd7320555e32b0de3e3c33a4b477976171f22747920"
