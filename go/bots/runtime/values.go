// Copyright 2026 Sneat.app

package runtime

import (
	"fmt"

	"github.com/sneat-games/chessraiders/go/bots/botvalue"
	"go.starlark.net/starlark"
)

// CallValues is Call's native-value twin: it invokes the same already-bound
// global function with arguments that are ALREADY Starlark values, skipping
// json.decode entirely, and returns the raw result without json.encode.
//
// This is the entry point the no-dictionary observation protocol uses. Call
// (JSON in, JSON out) stays byte-for-byte what it always was for the v1
// protocol; the two coexist so a host can run both during a migration and so
// a script written for one can never be silently fed the other — a v1 script
// reaching for observation["pieces"] against a botvalue row finds no Mapping
// to index and fails on its first read, and a v2 script reaching for
// observation.units against a decoded dict is told a dict has no such field.
//
// Arguments are typed botvalue.Value, which is an ALIAS for starlark.Value.
// That alias is what lets the private engine module build and pass these
// without importing go.starlark.net at all — server-go/importboundary sweeps
// import paths, and a host that imports only botvalue names no banned path.
//
// CONCURRENCY is unchanged from Call: one fresh *starlark.Thread per
// invocation, no per-Program mutable state, so a tournament may run this
// against one shared *Program from as many goroutines as it has matches.
func (p *Program) CallValues(name string, args ...botvalue.Value) (botvalue.Value, error) {
	return p.callValues(p.newThread(), name, args...)
}

// CallValueWithJSONArgs invokes name with one native value followed by JSON
// arguments. It is the production bot-protocol boundary: observation is an
// immutable, Go-backed value, while the mutable memory map, parameter map and
// scalar control arguments retain their established JSON representation.
//
// Keeping those two concerns separate is deliberate. The host must never
// construct a Starlark dictionary itself (only botvalue may mint native
// values), but bot memory is script-owned, dynamically keyed state and its
// JSON contract is independent of the observation protocol. The result stays
// JSON too, so callers can retain their existing intent validation path.
func (p *Program) CallValueWithJSONArgs(name string, first botvalue.Value, argsJSON ...string) (string, error) {
	return p.callValueWithJSONArgs(p.newThread(), name, first, argsJSON...)
}

// CallValueWithJSONArgsWithStepLimit is CallValueWithJSONArgs with the same
// per-decision limit and accounting contract as CallWithStepLimit. The
// returned step count includes JSON decoding for the non-observation
// arguments, the Starlark call, and encoding the result.
func (p *Program) CallValueWithJSONArgsWithStepLimit(
	maxSteps uint64, name string, first botvalue.Value, argsJSON ...string,
) (result string, steps uint64, err error) {
	thread := &starlark.Thread{Name: "starlarkbot-program-call"}
	if maxSteps > 0 {
		thread.SetMaxExecutionSteps(maxSteps)
	}
	result, err = p.callValueWithJSONArgs(thread, name, first, argsJSON...)
	return result, thread.ExecutionSteps(), err
}

// CallValuesWithStepLimit is CallValues under a per-decision execution-step
// ceiling, the exact mechanism CallWithStepLimit already applies to the JSON
// path: a FRESH ceiling per call, never a session-wide one, and the consumed
// step count returned unconditionally so a caller can charge a budget on the
// error path too.
func (p *Program) CallValuesWithStepLimit(maxSteps uint64, name string, args ...botvalue.Value) (botvalue.Value, uint64, error) {
	thread := &starlark.Thread{Name: "starlarkbot-program-call"}
	if maxSteps > 0 {
		thread.SetMaxExecutionSteps(maxSteps)
	}
	result, err := p.callValues(thread, name, args...)
	return result, thread.ExecutionSteps(), err
}

func (p *Program) callValues(thread *starlark.Thread, name string, args ...botvalue.Value) (botvalue.Value, error) {
	fn, ok := p.globals[name]
	if !ok {
		return nil, fmt.Errorf("starlarkbot: program has no global function named %q", name)
	}
	callable, ok := fn.(starlark.Callable)
	if !ok {
		return nil, fmt.Errorf("starlarkbot: global %q is a %s, not callable", name, fn.Type())
	}
	tuple := make(starlark.Tuple, len(args))
	for i, arg := range args {
		if arg == nil {
			tuple[i] = starlark.None
			continue
		}
		tuple[i] = arg
	}
	result, err := starlark.Call(thread, callable, tuple, nil)
	if err != nil {
		return nil, fmt.Errorf("starlarkbot: call %q: %w", name, err)
	}
	return result, nil
}

func (p *Program) callValueWithJSONArgs(
	thread *starlark.Thread, name string, first botvalue.Value, argsJSON ...string,
) (string, error) {
	fn, ok := p.globals[name]
	if !ok {
		return "", fmt.Errorf("starlarkbot: program has no global function named %q", name)
	}
	callable, ok := fn.(starlark.Callable)
	if !ok {
		return "", fmt.Errorf("starlarkbot: global %q is a %s, not callable", name, fn.Type())
	}

	args := make(starlark.Tuple, 1+len(argsJSON))
	if first == nil {
		args[0] = starlark.None
	} else {
		args[0] = first
	}
	for i, raw := range argsJSON {
		value, err := p.decodeJSON(thread, raw)
		if err != nil {
			return "", fmt.Errorf("starlarkbot: decode argument %d to %q: %w", i+1, name, err)
		}
		args[i+1] = value
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
