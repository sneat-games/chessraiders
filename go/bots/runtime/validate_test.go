// Copyright 2026 Sneat.app

package runtime

import (
	"strings"
	"testing"
)

// TestValidateSyntaxRejectsMalformedSource proves ValidateSyntax returns an
// ORDINARY error for a syntax error under native Go — the same source
// server-go/cmd/chesswasm/bots_wasm_bridge_test.go proves TRAPS the real
// compiled with-bots TinyGo-wasm artifact. That contrast is the whole point
// of this function's own doc comment: native Go's recover works here,
// TinyGo's does not, and this test is the "before" half of that claim (the
// "after" half — a wasm trap — cannot be reproduced in a native test at
// all, which is exactly why bots_wasm_bridge_test.go exists as a separate,
// artifact-level test).
func TestValidateSyntaxRejectsMalformedSource(t *testing.T) {
	err := ValidateSyntax("script.star", "this is not valid starlark ///")
	if err == nil {
		t.Fatal("ValidateSyntax(malformed source) = nil, want a syntax error")
	}
	if strings.Contains(err.Error(), "panic") {
		t.Fatalf("ValidateSyntax(malformed source) error = %q, want an ordinary parse error, not a panic message", err.Error())
	}
}

// TestValidateSyntaxAcceptsWellFormedSource is the positive control: a
// script this package's own tests already execute successfully elsewhere
// (eval_test.go's TestEvalScriptRoundTrip) must also pass syntax validation
// alone, so this function is proven to be a real filter, not one that
// rejects everything.
func TestValidateSyntaxAcceptsWellFormedSource(t *testing.T) {
	const source = `
def _run():
    data = json.decode(input)
    data["echoed"] = True
    return data

output = json.encode(_run())
`
	if err := ValidateSyntax("script.star", source); err != nil {
		t.Fatalf("ValidateSyntax(well-formed source) = %v, want nil", err)
	}
}

// TestShippedScriptsParseCleanly is this slice's own build-time validation:
// "parses every shipped script at build time and fails the build on a
// syntax error." canarySource and recursionProbeSource are the two dialect-
// canary scripts this package itself owns.
//
// server-go/starlarktier/chess-raiders-bot.star — the first REAL tier script this
// repository ships — is NOT in this table: it cannot be, since
// starlarktier imports this package (CompileTier -> starlarkbot.Compile),
// so an internal test file here importing starlarktier.Script back would
// be a compile-time import cycle. tier_syntax_test.go (this directory,
// `package starlarkbot_test`, a separate compilation unit) is where a
// tier's own script joins this same build-time gate instead — see that
// file's own doc comment for the full reasoning and for why compiling
// chess-raiders-bot.star at wasm MODULE LOAD (bots_entry.go) makes this gate matter
// even more there than it does here. A syntax error in ANY shipped script,
// in either file, fails `go test ./...` — this repository's own "build"
// gate for exactly this class of check (dialect_lint_test.go and
// import_boundary_test.go already enforce their own policies the same way,
// over the same package).
//
// Running on native Go is what makes this a real build-time gate rather
// than a runtime trap waiting to happen: see ValidateSyntax's own doc
// comment for why recover works here and does not under TinyGo.
func TestShippedScriptsParseCleanly(t *testing.T) {
	shipped := map[string]string{
		"canary.star":                 canarySource,
		"canary-recursion-probe.star": recursionProbeSource,
	}
	for name, source := range shipped {
		if err := ValidateSyntax(name, source); err != nil {
			t.Errorf("shipped script %s failed build-time syntax validation: %v", name, err)
		}
	}
}
