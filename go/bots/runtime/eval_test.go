// Copyright 2026 Sneat.app

package runtime

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEvalScriptRoundTrip is this slice's own acceptance test: "an
// evalScript-style entry point that takes Starlark source plus a JSON
// input and returns a JSON result — enough to prove the round trip end to
// end". The script source is constructed right here at test run time
// rather than read from any build asset or embedded constant — nothing
// about EvalScript requires a script known at compile time, and no symbol
// in this package names a tier or difficulty.
func TestEvalScriptRoundTrip(t *testing.T) {
	rt, err := New()
	if err != nil {
		t.Fatalf("New() = %v, want a runtime that passed the dialect canary", err)
	}

	const source = `
def _run():
    data = json.decode(input)
    data["echoed"] = True
    data["sum"] = data["a"] + data["b"]
    return data

output = json.encode(_run())
`
	outputJSON, err := rt.EvalScript(source, `{"a": 2, "b": 3}`)
	if err != nil {
		t.Fatalf("EvalScript() = %v, want a JSON result", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(outputJSON), &got); err != nil {
		t.Fatalf("EvalScript() returned %q, which does not decode as JSON: %v", outputJSON, err)
	}
	want := map[string]any{"a": 2.0, "b": 3.0, "echoed": true, "sum": 5.0}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("EvalScript() output[%q] = %v, want %v (full output: %s)", k, got[k], v, outputJSON)
		}
	}
}

// TestEvalScriptRejectsMissingOutput proves EvalScript fails loudly rather
// than returning an empty or nil result when a script never binds the
// output global it is contractually required to set.
func TestEvalScriptRejectsMissingOutput(t *testing.T) {
	rt, err := New()
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	const source = `x = 1 + 1` // never sets `output`
	if _, err := rt.EvalScript(source, "null"); err == nil {
		t.Fatal("EvalScript() with no 'output' global succeeded, want an error")
	} else if !strings.Contains(err.Error(), "output") {
		t.Fatalf("EvalScript() error = %q, want it to mention the missing 'output' global", err.Error())
	}
}

// TestEvalScriptRejectsNonStringOutput proves EvalScript does not silently
// coerce a script's forgotten json.encode(...) call — output must be a
// Starlark string.
func TestEvalScriptRejectsNonStringOutput(t *testing.T) {
	rt, err := New()
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	const source = `output = {"a": 1}` // a dict, not json.encode(...)'s string
	if _, err := rt.EvalScript(source, "null"); err == nil {
		t.Fatal("EvalScript() with a non-string 'output' global succeeded, want an error")
	}
}

// TestEvalScriptRejectsScriptError proves a script-level failure (here, a
// syntax/resolve error) surfaces as an error from EvalScript rather than a
// panic or a silently empty result — the same class of failure
// AC:a-script-cannot-name-a-square-the-engine-did-not gestures at for the
// full bot contract, exercised here at the thinner round-trip grain this
// slice actually implements.
func TestEvalScriptRejectsScriptError(t *testing.T) {
	rt, err := New()
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	const source = `this is not valid starlark ///`
	if _, err := rt.EvalScript(source, "null"); err == nil {
		t.Fatal("EvalScript() with invalid source succeeded, want an error")
	}
}

// TestEvalScriptUsesTheOneDialectConstant proves EvalScript's execution
// honours DialectOptions (Set/While) exactly like the canary does — a
// script using either dialect feature must run, since a tier script will.
func TestEvalScriptUsesTheOneDialectConstant(t *testing.T) {
	rt, err := New()
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	const source = `
def _run():
    seen = set()
    i = 0
    while i < 3:
        seen.add(i)
        i += 1
    return len(seen)

output = json.encode(_run())
`
	outputJSON, err := rt.EvalScript(source, "null")
	if err != nil {
		t.Fatalf("EvalScript() = %v, want set()/while to be usable", err)
	}
	if outputJSON != "3" {
		t.Fatalf("EvalScript() = %q, want %q", outputJSON, "3")
	}
}
