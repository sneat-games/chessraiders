// Copyright 2026 Sneat.app

package runtime

import "go.starlark.net/syntax"

// DialectOptions is the ONE *syntax.FileOptions value in this repository
// (REQ:one-dialect-constant). Every call this package makes into
// go.starlark.net — the canary, EvalScript, and every tier script that
// arrives later — parses, resolves and executes under exactly this value.
// Nothing in this package's public surface takes a *syntax.FileOptions
// parameter, so there is no signature through which a caller could ask for
// different options on one target and not the other.
//
// The two fields set true are the two dialect features
// spec/decisions/0007-local-battle-runtime.md's determinism spike measured
// diverging: go.starlark.net's LEGACY dialect API — the package-level
// resolve.Allow* globals, read (not set) by syntax.LegacyFileOptions() via
// a go:linkname pragma — SILENTLY NO-OPS under TinyGo, because of how
// TinyGo's linker handles that pragma (decision 0007's observed "set()
// rejected under the legacy API"). A build using the legacy path does not
// fail to compile — this constant, constructed explicitly rather than read
// off resolve's package globals, is what a build using the legacy path
// anywhere would fail the dialect canary on (canary.go).
//
// ONLY ONE OF THE TWO IS A BROWSER/SERVER SPLIT, though — measured at the
// pinned go.starlark.net version, worth stating precisely because an
// earlier version of this comment claimed both were: syntax.
// LegacyFileOptions() maps While to resolve.AllowGlobalReassign
// (options.go's own go:linkname), which defaults FALSE and is never set
// true anywhere in this repository — so the legacy path rejects `while`
// on STOCK GO too, with the identical "this Starlark dialect does not
// support while loops" error TinyGo would also give. set() is the one
// that actually diverges: resolve.AllowSet defaults true, so a build using
// the legacy path runs set() fine on the server and only fails it in the
// browser. The canary is stronger than the split alone would suggest: it
// still catches the While gap too, since a Go build using the legacy path
// would ALSO fail check 2, on the server, at process-start time — not a
// silent divergence for While, but still a real bug the canary catches
// before it ships.
//
//   - Set: true    — scripts may call the set() builtin.
//   - While: true   — scripts may use `while` inside a function body.
//
// Every other FileOptions field is left at its zero value (false),
// deliberately, matching go.starlark.net's "modern strict" default rather
// than the legacy dialect's defaults:
//
//   - TopLevelControl/GlobalReassign/LoadBindsGlobally: no tier script
//     needs a bare if/for/while or a reassigned global at module top level
//     — a script's own logic lives inside a def, per
//     spec/features/starlark-bot-runtime/builtin-starlark-tiers.
//   - Recursion: left false so the compiler's static recursion check stays
//     ENABLED — self-recursion is REJECTED, which is
//     REQ:dialect-canary's third assertion and one of the six semantic
//     divergences that disqualified starlark-rust in the determinism spike.
var DialectOptions = &syntax.FileOptions{
	Set:   true,
	While: true,
}
