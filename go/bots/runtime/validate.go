// Copyright 2026 Sneat.app

package runtime

// ValidateSyntax parses source under the ONE DialectOptions value this
// package exports and reports whether it parses — WITHOUT executing it (no
// thread, no predeclared globals, no returned globals; f is discarded).
//
// WHY THIS EXISTS, and why it is safe where EvalScript's own execution is
// not: go.starlark.net's parser reports a syntax error by panicking
// internally (syntax/scan.go) and recovering from that panic ITSELF
// (DialectOptions.Parse's own `defer p.in.recover(&err)`, inside the
// library, invisible to any caller) before ever returning — ordinarily an
// implementation detail nobody outside the library needs to know about.
// Under TinyGo 0.41.1 that internal recover does not work: the panic
// propagates all the way out as an uncatchable wasm trap instead of the
// error this function returns, verified against the real compiled
// artifact — see server-go/cmd/chesswasm/bots_wasm_bridge_test.go, which is
// the Node-driven proof that the SHIPPED with-bots module traps on exactly
// this input.
//
// Native Go has no such limitation — recover works correctly here (proven
// by TestValidateSyntaxRejectsMalformedSource below returning an ordinary
// error, never panicking, for the identical source the wasm test proves
// traps). That is exactly why this function is only useful when called
// from native Go, and only BEFORE a script ever reaches a TinyGo build or
// a TinyGo-wasm runtime:
//
//   - at build time, over every script this repository itself ships (see
//     TestShippedScriptsParseCleanly below) — the MVP instance of the
//     principle, and the only one this slice implements;
//   - later, per the founder's 2026-08-06 ruling on the eventual
//     user-authored-bot path, as a server-side parse gate that runs ahead
//     of the browser on every submitted script (a native-Go process, same
//     reasoning) — never a browser-side recovery mechanism, because a
//     TinyGo-wasm trap carries no diagnostic to recover WITH (see this
//     package's own doc comment and webapp/src/local/wasmLoad.ts's
//     BotsRuntimeTrapError for what a trap looks like from the caller's
//     side once it already happened). That server-side gate is not built
//     here — this function is the primitive it will call.
//
// A syntax error that reaches EvalScript inside a TinyGo-wasm build anyway
// (a bug in whatever validated it, or an as-yet-unbuilt path that skipped
// validation) still traps the module exactly as before; this function
// makes that trap avoidable up front, not survivable after the fact.
func ValidateSyntax(filename, source string) error {
	_, err := DialectOptions.Parse(filename, source, 0)
	return err
}
