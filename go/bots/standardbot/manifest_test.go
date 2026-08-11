// Copyright 2026 Sneat.app

package standardbot_test

import (
	"errors"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/manifest"
	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

var standardBotLimits = manifest.ValidationLimits{
	MaxFiles:         3,
	MaxFileBytes:     200_000,
	MaxTotalBytes:    400_000,
	MaxManifestBytes: 20_000,
	MaxJSONDepth:     32,
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
	if artifact.Digest.String() != standardBotClosureDigest {
		t.Fatalf("closure digest = %s, want %s", artifact.Digest, standardBotClosureDigest)
	}
	if err := runtime.ValidateEntrypoint(standardbot.Script, "decide", 5); err != nil {
		t.Fatalf("ValidateEntrypoint(standard bot) = %v, want exact five-argument decide", err)
	}
}

func TestStandardBotManifestRejectsAByteThatDoesNotMatchTheEmbeddedArtifact(t *testing.T) {
	entries := standardBotEntries()
	entries[1].Content = append(entries[1].Content, '\n')
	got := manifest.DigestClosure(map[string][]byte{
		manifest.ManifestPath:    standardbot.Manifest,
		"chess-raiders-bot.star": entries[1].Content,
		"params.json":            standardbot.Params,
	})
	if got.String() == standardBotClosureDigest {
		t.Fatal("a changed script produced the published closure digest")
	}
}

func TestStandardBotManifestClosureRejectsAnUndeclaredFile(t *testing.T) {
	entries := append(standardBotEntries(), manifest.TreeEntry{Path: "notes.txt", Kind: manifest.EntryKindRegular, Content: []byte("not source")})
	_, err := manifest.ValidateArtifact(standardbot.Manifest, entries, manifest.ValidationLimits{MaxFiles: 4, MaxFileBytes: 200_000, MaxTotalBytes: 400_000, MaxManifestBytes: 20_000, MaxJSONDepth: 32}, standardBotProfile)
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
const standardBotClosureDigest = "sha256:48068f256b8c7fef4f4230c339939b6afb42de43ec9ea488fe6f97108a497c42"
