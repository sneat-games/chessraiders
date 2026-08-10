// Copyright 2026 Sneat.app

package runtime

import (
	"fmt"

	starlarkjson "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
)

// Runtime is a Starlark runtime that has passed REQ:dialect-canary. New is
// the only way to obtain one, and New itself runs the canary — a Runtime
// value existing at all is the "canary passed" proof, so no method on it
// carries a redundant readiness check.
//
// This type deliberately knows nothing about bots: no observation, no
// legal-move set, no memory, no randomness source, no step budget. Those
// arrive with spec/features/starlark-bot-runtime/bot-script-contract and
// bot-execution-limits; this slice proves the module boundary and the
// dialect mechanics the tiers will stand on, nothing more.
type Runtime struct{}

// New runs the dialect canary and returns a Runtime only if it passed.
// REQ:dialect-canary: "the canary runs during runtime construction" — on
// the server, a caller treats a non-nil error here as a fatal startup
// error (this repository ships no server binary to wire that into,
// server-go/cmd/ holds only chesswasm, in two build variants — see that
// package's own top-of-file doc comment); in the browser,
// server-go/cmd/chesswasm's bots_entry.go (the with-bots variant's own
// half of that package) treats a canary failure as a reason to mark the
// BOT unavailable, never to withhold or abort the shared "chesswasm"
// namespace the engine's own entry points answer from too.
func New() (*Runtime, error) {
	if err := RunCanary(); err != nil {
		return nil, err
	}
	return &Runtime{}, nil
}

// EvalScript compiles and runs source under the ONE DialectOptions value
// this package exports, predeclaring:
//
//   - input, the raw inputJSON text, as a Starlark string. A script decodes
//     it itself — via the predeclared json module below — rather than the
//     host pre-parsing it into Starlark values, which is the same seam
//     spec/decisions/0007-local-battle-runtime.md's determinism spike
//     validated: "observations and intents cross the wasm boundary as JSON
//     via the predeclared go.starlark.net/lib/json module (no bespoke value
//     bridge)".
//   - json, go.starlark.net/lib/json's own module (encode/decode/indent),
//     predeclared per call rather than installed into starlark.Universe —
//     this keeps every execution's predeclared surface an explicit,
//     inspectable value instead of mutable global state.
//
// The script must bind a global named output to a Starlark string — the
// result of calling json.encode(...) itself — which EvalScript returns
// verbatim as outputJSON. There is no other contract: no observation
// shape, no legal-move set, no intent shape, no tier vocabulary. This is
// deliberately just enough to prove the source-in/JSON-in, JSON-out round
// trip end to end on both targets; see this package's doc comment for what
// is NOT here yet.
//
// EvalScript names no tier, no difficulty and no script ID — the parent
// Feature's REQ:script-provenance-is-runtime-data note that "no symbol in
// the runtime package names any tier or difficulty" already holds simply
// because nothing tier-shaped has been added yet.
func (r *Runtime) EvalScript(source, inputJSON string) (outputJSON string, err error) {
	thread := &starlark.Thread{Name: "starlarkbot-eval"}
	predeclared := starlark.StringDict{
		"input": starlark.String(inputJSON),
		"json":  starlarkjson.Module,
	}
	globals, err := starlark.ExecFileOptions(DialectOptions, thread, "script.star", source, predeclared)
	if err != nil {
		return "", fmt.Errorf("starlarkbot: script error: %w", err)
	}
	out, ok := globals["output"]
	if !ok {
		return "", fmt.Errorf("starlarkbot: script did not set a global named 'output'")
	}
	outStr, ok := out.(starlark.String)
	if !ok {
		return "", fmt.Errorf("starlarkbot: script's 'output' global is a %s, want string (call json.encode(...) to produce one)", out.Type())
	}
	return string(outStr), nil
}
