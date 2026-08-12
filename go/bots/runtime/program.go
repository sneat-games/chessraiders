// Copyright 2026 Sneat.app

package runtime

import (
	"fmt"

	starlarkjson "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
)

// Program is a Starlark module compiled and executed exactly ONCE: Compile
// parses and runs source's top level (every constant, every def) a single
// time, and Call re-enters an already-bound global function per decision —
// never re-parsing or re-executing the module's own top level again.
//
// This is the compile-once API EvalScript never had room for. EvalScript's
// whole contract is "parse, run the WHOLE module, read one output global" —
// exactly once, per call — which is exactly right for a one-shot round trip
// but untenable for a tier script invoked once per bot decision: a 590-line
// script would pay the full lex/parse/resolve/exec cost of every constant
// and every def statement on EVERY decision, not just the first. Program
// pays that cost once per Compile and amortises it across every later Call.
//
// The dialect canary, DialectOptions and ValidateSyntax are all UNCHANGED by
// this: Compile runs under the exact same DialectOptions constant EvalScript
// and the canary already use, and a caller still runs the canary once (via
// New/RunCanary) before ever trusting this package with a real script —
// Program adds a second, complementary entry point, it does not replace the
// first one or the readiness check that gates it.
//
// CONCURRENCY: a *Program is called from more than one goroutine at once in
// production — server-go/starlarkbot4chess compiles ONE Program (wrapped in
// one *starlarktier.CompiledTier, behind a package-level sync.OnceValues)
// and every server-hosted match's own goroutine (each
// decides inside its own facade4chess.Store.Update transaction) calls the
// SAME Program's Call concurrently. Program therefore holds NO per-call
// mutable state of its own: globals is frozen by starlark.ExecFileOptions
// before Compile/CompileWithStepLimit ever return (Call's own doc comment
// below), and Call constructs a BRAND-NEW *starlark.Thread — never a shared
// one — for every single invocation. That is deliberate, not merely
// convenient, and the plan behind it deserves stating once, here, since
// "just add a mutex" or "keep a pool of threads" are the two fixes a future
// reader is likely to reach for instead:
//
//   - A MUTEX would make two concurrent matches serialize on ONE Starlark
//     thread — correct, but it turns a server with N cores into a server
//     that runs bot decisions one at a time, and it reintroduces exactly the
//     cross-decision thread state (thread.Local, thread.Steps,
//     thread.cancelReason) this design avoids needing to reason about at
//     all.
//   - A POOL (free list of reusable *starlark.Thread values) avoids the
//     serialization but is rejected on SIZE, not just complexity, and that
//     has to survive in code because a pool is exactly what a future
//     "optimisation" would reach for: this package also compiles into the
//     BROWSER's with-bots wasm artifact (server-go/cmd/chesswasm), where
//     there is exactly one Worker and exactly one decision in flight at a
//     time ever — a free list there buys nothing but costs every player
//     bytes of a size-budgeted artifact (webapp's size-budget.mjs) to solve
//     a problem that artifact does not have. The server-only concurrency
//     problem gets a server-only-cost fix: a thread is cheap to allocate
//     (go.starlark.net's own Thread is a small struct with no I/O), so
//     "construct a fresh one, use it once, let the GC reclaim it" costs
//     nothing extra on the one-decision-at-a-time browser path and removes
//     every data race on the many-decisions-at-once server path.
//
// Measured under `go test -race` on the pinned go.starlark.net version: the
// shared-thread shape this replaced produced 19 data races on
// thread.stack and then a SIGSEGV in starlark.(*frame).asCallFrame within
// seconds of two matches deciding concurrently; the fresh-thread-per-Call
// shape in newThread/Call below measured 0 races, 0 result mismatches and 0
// errors across 64 goroutines making 200 calls each (concurrency_test.go).
type Program struct {
	globals starlark.StringDict
	// maxSteps is 0 for every Program Compile returns (unbounded, exactly
	// today's behaviour) and non-zero only for one CompileWithStepLimit
	// returns — see that function's own doc comment. newThread below branches
	// on this being non-zero, so a first-party tier's own Program (Compile,
	// never touched by this field) takes the exact same path it always did.
	maxSteps uint64
}

// Compile parses and executes source's top level exactly once, under this
// package's own DialectOptions, predeclaring go.starlark.net/lib/json's
// module under the same "json" name EvalScript predeclares — a script may
// still call json.encode/json.decode from inside a function body. UNLIKE
// EvalScript, Compile does NOT predeclare `input`: a Program's own
// per-decision data travels through Call's own arguments below, never a
// fresh global rebound on every call the way EvalScript's single-shot
// contract required (that per-call rebinding is exactly the "exec-and-
// read-a-global" adapter shape a compile-once caller no longer needs).
//
// The thread this uses to run the module's own top level is LOCAL to this
// call and discarded the moment ExecFileOptions returns — it is never kept
// on the returned Program and never reused by Call. That is safe precisely
// because nothing about a go.starlark.net *starlark.Function remembers the
// thread that defined it: Call below hands every invocation its own fresh
// thread (newThread), and this compile-time one is no different in kind,
// just shorter-lived.
func Compile(source string) (*Program, error) {
	thread := &starlark.Thread{Name: "starlarkbot-program-compile"}
	predeclared := programPredeclared()
	globals, err := starlark.ExecFileOptions(DialectOptions, thread, "script.star", source, predeclared)
	if err != nil {
		return nil, fmt.Errorf("starlarkbot: compile: %w", err)
	}
	return &Program{globals: globals}, nil
}

// CompileWithStepLimit is Compile's sibling for a script this codebase did
// not author — the "bot lab" feature's whole reason for existing (spec/
// features/bot-lab). Byte-for-byte Compile except for one addition: every
// thread newThread constructs for a Call against the returned Program
// carries a per-decision execution-step ceiling (go.starlark.net's own
// Thread.SetMaxExecutionSteps — decision 0007's own "Thread.
// SetMaxExecutionSteps carries the tier step budget", applied here to the
// one caller that actually needs it today) — a FRESH ceiling every Call,
// never a cumulative one spent down across a whole session, because
// newThread's thread is itself fresh every Call. A session that calls
// decide() a thousand times must not spend a thousand-decision-wide budget
// down to nothing by decision fifty.
//
// THIS IS NOT bot-execution-limits' OWN STEP BUDGET. That Feature (spec/
// features/starlark-bot-runtime/bot-execution-limits) is still Draft: its
// own numbers are calibrated per tier against a measured 250ms whole-cycle
// ceiling, and it ships a memory bound and a two-level Worker-termination
// recovery model alongside the step count, none of which exists yet. This
// function borrows ONLY the mechanism — go.starlark.net's own
// Thread.SetMaxExecutionSteps — for the one property a script this
// codebase did not write cannot safely run without: a bound that turns "an
// infinite loop wedges the calling thread forever" into "the call fails
// fast with a clear message", nothing more. maxSteps is a caller-supplied
// constant (starlarktier.CustomScriptMaxSteps), never tier-specific,
// because there is no tier here — one script, one budget.
//
// A first-party tier's own Program (Compile, above) is completely
// unaffected: maxSteps stays 0 on that Program forever, and newThread's own
// branch on it is a no-op whenever maxSteps is 0 — go.starlark.net's own
// Thread.Steps/maxSteps are otherwise left at their normal, effectively-
// unbounded default (eval.go's own one-time initialisation) on every thread
// newThread builds.
func CompileWithStepLimit(source string, maxSteps uint64) (*Program, error) {
	thread := &starlark.Thread{Name: "starlarkbot-program-compile-bounded"}
	predeclared := programPredeclared()
	globals, err := starlark.ExecFileOptions(DialectOptions, thread, "script.star", source, predeclared)
	if err != nil {
		return nil, fmt.Errorf("starlarkbot: compile: %w", err)
	}
	return &Program{globals: globals, maxSteps: maxSteps}, nil
}

// programPredeclared is the single source for names visible to a compiled bot
// module. Static entrypoint validation uses this same dictionary's Has method,
// so a name cannot resolve during admission and then disappear at execution.
func programPredeclared() starlark.StringDict {
	return starlark.StringDict{"json": starlarkjson.Module}
}

// newThread builds the fresh *starlark.Thread every single Call gets — see
// Program's own doc comment (the CONCURRENCY section) for why this exists
// instead of a shared thread or a pool of them, and Call's own doc comment
// for why every one of its three go.starlark.net entry points
// (decodeJSON/the decide call itself/encodeJSON) must be handed THIS SAME
// thread value and never a mix of this one and the old shared one — a
// partial conversion narrows the data race rather than removing it.
//
// This is also where CompileWithStepLimit's own per-Call ceiling actually
// applies: a fresh thread starts at Steps == 0 with no cancellation reason
// set, so honouring maxSteps here needs no Uncancel()/Steps = 0 reset the
// way a shared, reused thread would have — there is nothing left over from
// a previous call to reset in the first place.
func (p *Program) newThread() *starlark.Thread {
	thread := &starlark.Thread{Name: "starlarkbot-program-call"}
	if p.maxSteps > 0 {
		thread.SetMaxExecutionSteps(p.maxSteps)
	}
	return thread
}

// Call invokes the global function named `name` — bound once, by Compile's
// own module execution, never re-executed here — with each element of
// argsJSON decoded from JSON into a Starlark value first (go.starlark.net/
// lib/json's own decode, the exact module Compile predeclares), and returns
// its result re-encoded as JSON (json.encode) verbatim: a single JSON value
// for a single Starlark return value, or a JSON array when the function
// returns more than one — Starlark's `return a, b` is a 2-tuple, and
// json.encode's own doc says "any other Starlark Iterable (e.g. list,
// tuple) is encoded as a JSON array", so a two-value return comes back as a
// two-element JSON array, in order.
//
// This is the direct call a tier script's own bottom adapter block existed
// only to simulate: no per-call `json.decode(input) / decide(...) /
// output = json.encode(...)` glue living IN the script at all — that
// plumbing now lives here, once, in Go, and the script's own top-level
// function is invoked exactly as if a Go caller already held it as a real
// function value (which, after Compile, it does — p.globals[name]).
//
// Every Starlark global Compile bound is shared AND FROZEN across every
// Call this Program ever makes — isolation here is actually STRONGER than
// an earlier version of this comment claimed ("shared, unfrozen"):
// starlark.ExecFileOptions calls globals.Freeze() immediately after
// mod.Init succeeds (go.starlark.net's own eval.go), before Compile ever
// returns, so even a MUTABLE top-level binding (a script-level list or
// dict) is permanently frozen the moment compilation finishes. chess-raiders-bot.star
// has none (every top-level binding is a numeric constant or a def), so
// this is inert for it today, but it is worth stating precisely for a
// future script that tries to keep mutable state at module level: it
// would not "see it persist, and leak, across decisions" the way ordinary
// shared-and-mutable module state would — Starlark raises an immediate
// runtime error on the FIRST attempted mutation (e.g. "cannot append to
// frozen list"), inside that one Call, rather than silently leaking state
// between decisions. Only a Go-side value threaded through Call's own
// arguments crosses a decision's own boundary uncontrolled; nothing at
// Starlark module scope can.
//
// CONCURRENCY: exactly ONE *starlark.Thread is constructed per Call
// (newThread above) and that SAME thread — never the globals dict's own
// long-gone compile-time thread, never a second one built partway through —
// serves ALL THREE of this call's own go.starlark.net entry points: the
// argument decode loop below (decodeJSON, once per element of argsJSON),
// the decide/entry-point invocation itself (starlark.Call), and the result
// encode (encodeJSON). Converting only one or two of those three and leaving
// the others on a shared thread would not remove the data race Program's
// own doc comment measures — it would only narrow it to whichever call
// still shares state, and a narrowed race is exactly what a `-race` test
// written against the FULLY-fixed shape can miss. That claim was CHECKED,
// not assumed, while writing concurrency_test.go: a scratch build that
// moved only the decide() call to a fresh thread and left decodeJSON/
// encodeJSON on the old shared p.thread field still failed
// TestProgramCallConcurrentUseIsRaceFree under `-race` (races on
// thread.stack, same signature as the fully-shared shape) — proof that
// this test criterion catches the narrowed race and not merely the obvious
// one, kept here as a record rather than as a permanently-failing test in
// this package's own suite.
func (p *Program) Call(name string, argsJSON ...string) (string, error) {
	return p.call(p.newThread(), name, argsJSON...)
}

// CallWithStepLimit is Call's sibling for a caller whose step ceiling varies
// CALL TO CALL rather than being fixed once at Compile time — the server bot
// resolver (server-go/starlarkbot4chess, via server-go/starlarktier), which
// shares ONE compiled chess-raiders-bot.star Program across Recruit/Lieutenant/Commander/
// Custom yet must bound each decision by THAT match's own
// chess.BotTierRules.StepBudget. CompileWithStepLimit cannot express this on
// its own: maxSteps lives on the *Program (this file's own CONCURRENCY
// comment), so one Program has exactly one ceiling for its whole process
// lifetime — right for bot lab's one-script-one-budget shape
// (starlarktier.CompileCustomTier), wrong for a shared first-party tier whose
// budget differs by difficulty. This method takes maxSteps as an ordinary
// argument instead, applied to the ONE fresh thread this call constructs
// (never p.maxSteps, which stays whatever Compile/CompileWithStepLimit set —
// irrelevant here since the resolver's own Program is an ordinary,
// unbounded Compile result).
//
// maxSteps == 0 means unbounded, exactly like an ordinary Program's default
// (newThread's own branch on p.maxSteps > 0) — a caller must pass a real
// ceiling to get one.
//
// Also returns the thread's own ExecutionSteps(), UNCONDITIONALLY — on
// success and on error alike, so a caller can record the real cost of a
// call that hit its own ceiling (a cancelled call has no valid JSON result
// to decode, but it still spent real, chargeable steps) as well as an
// ordinary one that returned cleanly. This is the ONLY way a caller outside
// this package ever learns a call's step cost: Call above deliberately
// stays two-return-values, byte-for-byte what it always was, so every
// existing caller of it (this package's own tests included) is completely
// unaffected by this method's existence.
func (p *Program) CallWithStepLimit(maxSteps uint64, name string, argsJSON ...string) (result string, steps uint64, err error) {
	thread := &starlark.Thread{Name: "starlarkbot-program-call"}
	if maxSteps > 0 {
		thread.SetMaxExecutionSteps(maxSteps)
	}
	result, err = p.call(thread, name, argsJSON...)
	return result, thread.ExecutionSteps(), err
}

// call is Call's and CallWithStepLimit's shared core: given a thread already
// constructed by whichever caller needs its own ceiling (or none), invoke
// the named global once, decoding argsJSON and encoding the result through
// THAT SAME thread for all three go.starlark.net entry points — see this
// file's own doc comment (the CONCURRENCY section) for why all three must
// share one thread and never a mix of two.
func (p *Program) call(thread *starlark.Thread, name string, argsJSON ...string) (string, error) {
	fn, ok := p.globals[name]
	if !ok {
		return "", fmt.Errorf("starlarkbot: program has no global function named %q", name)
	}
	callable, ok := fn.(starlark.Callable)
	if !ok {
		return "", fmt.Errorf("starlarkbot: global %q is a %s, not callable", name, fn.Type())
	}

	args := make(starlark.Tuple, len(argsJSON))
	for i, raw := range argsJSON {
		v, err := p.decodeJSON(thread, raw)
		if err != nil {
			return "", fmt.Errorf("starlarkbot: decode argument %d to %q: %w", i, name, err)
		}
		args[i] = v
	}
	result, err := starlark.Call(thread, callable, args, nil)
	if err != nil {
		return "", fmt.Errorf("starlarkbot: call %q: %w", name, err)
	}
	out, err := p.encodeJSON(thread, result)
	if err != nil {
		return "", fmt.Errorf("starlarkbot: encode result of %q: %w", name, err)
	}
	return out, nil
}

// HasFunction reports whether Compile bound a CALLABLE global named `name` —
// a cheap, non-executing check (no thread, no arguments, no step charged)
// for a caller that wants to reject a script at LOAD time rather than wait
// for its first Call to discover a missing or misspelled entry point. It
// does not check arity: a `decide` with the wrong number of parameters
// still passes this check and fails, with an equally clear message, on its
// first real Call (bot-script-contract's own REQ:decide-entrypoint asks for
// arity checking too; this is the cheap half of that requirement, not the
// whole of it — see starlarktier.CompileCustomTier's own doc comment).
func (p *Program) HasFunction(name string) bool {
	fn, ok := p.globals[name]
	if !ok {
		return false
	}
	_, ok = fn.(starlark.Callable)
	return ok
}

// decodeJSON/encodeJSON call go.starlark.net/lib/json's own decode/encode
// builtins directly (via starlark.Call, never a fresh starlark.ExecFile* —
// these are ordinary builtin-function calls, not module re-execution) so
// Program never has to duplicate that library's own JSON<->Starlark
// conversion rules. Both are unexported: Program's own public surface stays
// JSON-in/JSON-out, exactly like EvalScript's, so a caller outside this
// package never has to import go.starlark.net/starlark to hold or build a
// starlark.Value itself — which matters beyond convenience, since
// import_boundary_test.go enforces that only this package may import
// go.starlark.net/{starlark,resolve,syntax} at all.
//
// Both take `thread` as a parameter rather than reading a Program-owned
// field — the SAME *starlark.Thread Call's own newThread built for this one
// invocation, and never any other — which is what makes these two safe to
// call from many goroutines at once against the same *Program: there is no
// shared mutable field here for two concurrent Calls to collide on. See
// Call's own doc comment for why passing a mix (one call site converted,
// another still reading a shared field) would defeat the point.
func (p *Program) decodeJSON(thread *starlark.Thread, raw string) (starlark.Value, error) {
	decodeFn, _ := starlarkjson.Module.Attr("decode")
	if decodeFn == nil {
		return nil, fmt.Errorf("starlarkbot: json.decode is not available on the predeclared json module")
	}
	return starlark.Call(thread, decodeFn, starlark.Tuple{starlark.String(raw)}, nil)
}

func (p *Program) encodeJSON(thread *starlark.Thread, v starlark.Value) (string, error) {
	encodeFn, _ := starlarkjson.Module.Attr("encode")
	if encodeFn == nil {
		return "", fmt.Errorf("starlarkbot: json.encode is not available on the predeclared json module")
	}
	result, err := starlark.Call(thread, encodeFn, starlark.Tuple{v}, nil)
	if err != nil {
		return "", err
	}
	s, ok := result.(starlark.String)
	if !ok {
		return "", fmt.Errorf("starlarkbot: json.encode returned a %s, want string", result.Type())
	}
	return string(s), nil
}
