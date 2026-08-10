// Copyright 2026 Sneat.app

// Package runtime is the Starlark bot runtime named by
// spec/features/starlark-bot-runtime/README.md: one implementation
// (go.starlark.net), compiled to two targets — linked natively into the
// backend and built with TinyGo into the browser's engine wasm module
// (server-go/cmd/chesswasm), which itself builds in two variants: slim
// (engine only, `-tags nobots`) and with-bots (engine plus this package,
// the default). There is no longer a separate bots-only wasm artifact —
// see spec/features/starlark-bot-runtime/bots-wasm-module's
// REQ:the-runtime-ships-inside-the-engine-artifact, which superseded the
// REQ:thin-module-boundary this package's own import-boundary rule below
// predates.
//
// THIS SLICE IS DELIBERATELY THIN. It exists to prove the module boundary
// and the dialect mechanics before any bot semantics are built on top of
// them:
//
//   - the ONE syntax.FileOptions constant every call site in this package
//     uses (dialect.go), so the tier scripts that arrive later inherit a
//     single, explicit dialect rather than each construction picking its
//     own;
//   - the dialect canary (canary.go) that a caller runs once, at runtime
//     construction, before trusting this package with a bot script;
//   - EvalScript (eval.go), a source-plus-JSON-in, JSON-out round trip with
//     no bot vocabulary at all — no observation, no legal-move set, no
//     memory, no randomness source, no step budget. Those belong to
//     spec/features/starlark-bot-runtime/bot-script-contract and
//     bot-execution-limits, built on later.
//
// THE DATA BOUNDARY SURVIVES THE ARTIFACT BOUNDARY'S DELETION.
// REQ:thin-module-boundary used to require this package to ship as its own
// TinyGo artifact, separate from the engine, so a bot could never derive
// moves from a stale embedded copy of the rules while the authority judged
// it by a different, newer copy. That requirement is SUPERSEDED
// (spec/features/starlark-bot-runtime/bots-wasm-module's
// REQ:the-runtime-ships-inside-the-engine-artifact, 2026-08-06): one
// TinyGo build of one source tree removes the disagreement hazard rather
// than managing it, since there is now only one binary to disagree with.
// Two of the old requirement's properties survive UNCHANGED, because they
// were never about the artifact boundary in the first place:
//
//   - this package MUST STILL NOT import
//     github.com/sneat-co/chessraiders/server-go/chess,
//     github.com/sneat-co/chessraiders/server-go/chessview, or
//     github.com/sneat-co/chessraiders/server-go/facade4chess. It contains no
//     board representation, no legality code, no projection code and no
//     rules — the COMMAND that builds the with-bots variant
//     (server-go/cmd/chesswasm, specifically bots_entry.go) imports both
//     this package AND the engine packages, but this package itself still
//     imports neither, so the dependency inside that one binary runs only
//     one way: a script can never reach the engine except through data
//     (an observation, a legal-move set) it was handed.
//   - a script is handed its legal moves and has no query surface of its
//     own — same binary or not, this package's own public API (EvalScript)
//     takes and returns plain JSON, never a live handle onto anything the
//     engine holds.
//
// REQ:legacy-dialect-entry-points-are-banned: every call this package makes
// into go.starlark.net uses the "*Options" entry point that takes an
// explicit *syntax.FileOptions — never the legacy standalone form that
// silently substitutes syntax.LegacyFileOptions(), the exact path whose
// resolve.Allow* globals TinyGo's linkname handling zeroes out (decision
// 0007's observed "set() rejected under the legacy API").
// dialect_lint_test.go's name check walks every .go file under this
// module's own root (not just this package's own directory — a bare
// go/parser.ParseFile does not consult build tags, so a js/wasm-only file
// elsewhere in this module would be scanned regardless of the host's own
// GOOS). The complementary half of the same invariant — that no OTHER
// package reaches for go.starlark.net/resolve, go.starlark.net/syntax or
// go.starlark.net/starlark at all — is enforced on the CONSUMING side too:
// sneat-co/chessraiders' own server-go/importboundary package sweeps its
// entire module's `go list ./...` (both the host's native GOOS and
// GOOS=js GOARCH=wasm, for the identical build-tag-blind-spot reason) and
// fails if any first-party package there imports go.starlark.net directly
// instead of going through this package. This package is the one
// implementation either side's check exempts.
//
// THE SANDBOX TODAY, RECORDED HONESTLY — this slice implements no step
// budget, no memory budget and no output gating (bot-execution-limits is
// later work), and four concrete facts about what that means are worth
// stating plainly rather than left implicit, each verified against the
// real compiled TinyGo-wasm artifact:
//
//   - print() is reachable from a script and writes directly to the host
//     console with no gating at all — verified: a script calling
//     print("...") puts that text on stdout/the browser console
//     unconditionally. A first-party script has no reason to call it, but
//     nothing here stops one from using it as an unmonitored side channel.
//   - There is NO memory bound. A script that allocates a huge value (e.g.
//     `len("x" * 1_000_000_000)`) is not rejected — verified: linear
//     memory grew to 1.5 GB for exactly that call before this repository's
//     own review stopped pushing it further, well past what any legitimate
//     tier decision needs.
//   - The engine and the bots share ONE Worker thread today
//     (webapp/src/local/worker.ts) — a script executing `while True: pass`
//     wedges that shared thread, and with it every board surface driven by
//     the SAME worker, not merely the bot's own turn. This is a property
//     of where webapp/src/local/localMatch.ts calls the bot decision from
//     (synchronously, in-line with the match tick), not of this package.
//   - THE ENGINE AND THE BOTS NOW SHARE ONE WASM INSTANCE TOO, in the
//     with-bots build variant (server-go/cmd/chesswasm's
//     REQ:the-runtime-ships-inside-the-engine-artifact) — a step beyond
//     the shared-thread fact above. A Starlark syntax error traps the
//     whole instance (bots_wasm_bridge_test.go's
//     TestEvalScriptTrapsOnSyntaxErrorInTheShippedArtifact), and because
//     it is now the SAME instance the engine's own entry points answer
//     from, every engine call made against that instance would fail too
//     until webapp/src/local/wasmLoad.ts re-instantiates it. Recovery is
//     cheap and correctness-preserving BECAUSE neither role retains state
//     in Go across a call (main.go's own "JSON in, JSON out, no shared
//     mutable state" contract, which EvalScript follows too) — the
//     caller's own chess.State lives in JS, not in the dead instance — but
//     a call racing the same tick as the trap, before the reinstantiate
//     completes, still fails. This is a DIFFERENT condition from a failed
//     dialect canary (REQ:dialect-canary), which never traps at all and
//     leaves the engine fully usable by construction; a trap is the rarer,
//     "should never happen once ValidateSyntax has run" backstop case.
//
// None of this is a defect in THIS slice — EvalScript's own contract never
// promised a bound, and bot-execution-limits is the Feature that adds one.
// It is recorded here so a later reader evaluating "is the runtime safe to
// hand a script to" does not have to rediscover any of the three by
// running the artifact again.
package runtime
