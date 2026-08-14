// Copyright 2026 Sneat.app

package botvalue_test

// safety_test.go proves the three properties the native-value protocol must
// hold against a HOSTILE script, not merely a cooperative one. Bot scripts
// will be user-contributed for tournaments, so "our script does not do that"
// is not evidence of anything here — each property is proved by writing the
// attack and observing it fail.

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/botvalue"
	"github.com/sneat-games/chessraiders/go/bots/runtime"
)

// runAttack compiles and runs a decide() body against a live native
// observation and returns the error text (empty when the script succeeded).
func runAttack(t *testing.T, body string) string {
	t.Helper()
	h := newNativeHost(newFixture())
	source := "def decide(observation, memory, params, host_random_draw, options):\n" + body + "\n"
	p, err := runtime.Compile(source)
	if err != nil {
		return "compile: " + err.Error()
	}
	gen := h.sess.Begin()
	h.array.Rebind(gen)
	defer h.sess.End()
	if _, err := p.CallValues("decide", h.obs, botvalue.None, botvalue.None, botvalue.None, botvalue.None); err != nil {
		return err.Error()
	}
	return ""
}

// TestMutationIsUnrepresentable is the founder's own requirement: mutation
// must not be "rejected", it must have no method to dispatch to. Every one
// of these is a real Starlark mutation form, and each must fail.
func TestMutationIsUnrepresentable(t *testing.T) {
	attacks := map[string]struct{ body, want string }{
		"set a row field": {
			"    observation.units[0].square = \"e4\"\n    return None, {}, []",
			"can't assign to .square field of unit",
		},
		"set a row key": {
			"    observation.units[0][\"square\"] = \"e4\"\n    return None, {}, []",
			"unit value does not support item assignment",
		},
		"set an array element": {
			"    observation.units[0] = None\n    return None, {}, []",
			"rows value does not support item assignment",
		},
		"append to a square list": {
			"    observation.units[0].guarded_by.append(\"e4\")\n    return None, {}, []",
			"has no .append field or method",
		},
		"set a square-list element": {
			"    observation.units[0].guarded_by[0] = \"e4\"\n    return None, {}, []",
			"squares value does not support item assignment",
		},
		"set an observation field": {
			"    observation.units = None\n    return None, {}, []",
			"can't assign to .units field of observation",
		},
		"use a row as a dict key": {
			"    d = {}\n    d[observation.units[0]] = 1\n    return None, {}, []",
			"unhashable type: unit",
		},
	}
	for name, attack := range attacks {
		t.Run(name, func(t *testing.T) {
			got := runAttack(t, attack.body)
			if got == "" {
				t.Fatalf("attack %q SUCCEEDED — mutation reached a native value", name)
			}
			if !strings.Contains(got, attack.want) {
				t.Fatalf("attack %q failed with %q, want a message containing %q", name, got, attack.want)
			}
		})
	}
}

// TestUnknownAttributeIsRejectedWithTheClosedSurface proves the schema is the
// exposure boundary: an attribute the schema does not declare does not exist,
// no matter what the engine happens to hold in memory.
func TestUnknownAttributeIsRejectedWithTheClosedSurface(t *testing.T) {
	got := runAttack(t, "    return observation.units[0].capture_seed, {}, []")
	if !strings.Contains(got, "has no .capture_seed field or method") {
		t.Fatalf("undeclared attribute produced %q, want a no-such-field error", got)
	}
}

// TestStaleReferenceFailsLoudly is the lazy-backing hazard. A Go-backed row
// reads live host memory; a reference read after its decision ended must
// diagnose itself rather than silently report whatever the engine now holds.
func TestStaleReferenceFailsLoudly(t *testing.T) {
	h := newNativeHost(newFixture())
	gen := h.sess.Begin()
	h.array.Rebind(gen)

	// A host (or a script's return value) holding a row past the decision.
	row, err := h.obs.Attr("units")
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	units, ok := row.(*botvalue.RowArray)
	if !ok {
		t.Fatalf("units is %T", row)
	}
	first, _ := units.Index(0).(*botvalue.Row)
	if _, err := first.Attr("square"); err != nil {
		t.Fatalf("live read failed: %v", err)
	}

	h.sess.End() // the decision ends

	_, err = first.Attr("square")
	if err == nil {
		t.Fatal("a row read after its decision ended SUCCEEDED — a stale reference silently reads live engine state")
	}
	stale, ok := err.(*botvalue.ErrStaleObservation)
	if !ok {
		t.Fatalf("stale read returned %T (%v), want *ErrStaleObservation", err, err)
	}
	if !strings.Contains(stale.Error(), "was read after its decision ended") {
		t.Fatalf("stale error is not self-diagnosing: %q", stale.Error())
	}
}

// TestReferenceCannotSurviveIntoTheNextDecision runs the concrete attack the
// founder named: a script that squirrels a reference away and reads it on a
// later decision. Module globals are frozen by Compile, so the script cannot
// even store one — and if a host ever hands a row back across the boundary,
// the generation check above catches it. Both halves are asserted here.
func TestReferenceCannotSurviveIntoTheNextDecision(t *testing.T) {
	got := runAttack(t, "    STASH.append(observation.units[0])\n    return None, {}, []")
	if got == "" {
		t.Fatal("a script stashed an observation row in a module global")
	}
	// The global does not resolve at all under this dialect, which is the
	// first line of defence; the freeze is the second.
	if !strings.Contains(got, "STASH") {
		t.Fatalf("stash attempt failed for an unrelated reason: %q", got)
	}

	h := newNativeHost(newFixture())
	source := `
def decide(observation, memory, params, host_random_draw, options):
    return observation.units[0], {}, []
`
	p, err := runtime.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	gen := h.sess.Begin()
	h.array.Rebind(gen)
	result, err := p.CallValues("decide", h.obs, botvalue.None, botvalue.None, botvalue.None, botvalue.None)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	h.sess.End()

	returned, _ := botvalue.TupleAt(result, 0)
	escaped, ok := returned.(*botvalue.Row)
	if !ok {
		t.Fatalf("returned value is %T, want *Row", returned)
	}
	if _, err := escaped.Attr("rank"); err == nil {
		t.Fatal("a row RETURNED out of decide() still reads live engine state after the decision ended")
	}
}

// TestV1ScriptFailsImmediatelyAgainstV2Values is the loud-mismatch
// requirement. A script written for the dict-shaped protocol indexes the
// observation; a native observation implements no Mapping, so it fails on
// its very first read rather than behaving subtly differently.
func TestV1ScriptFailsImmediatelyAgainstV2Values(t *testing.T) {
	got := runAttack(t, "    return observation[\"pieces\"], {}, []")
	if got == "" {
		t.Fatal("a v1 dict-shaped script ran against a v2 native observation")
	}
	if !strings.Contains(got, "unhandled index operation") {
		t.Fatalf("v1-against-v2 failed with %q, want an index-operation error", got)
	}
}

// TestV2ScriptFailsImmediatelyAgainstV1Values is the reverse mismatch.
func TestV2ScriptFailsImmediatelyAgainstV1Values(t *testing.T) {
	p, err := runtime.Compile(`
def decide(observation, memory, params, host_random_draw, options):
    return len(observation.units), {}, []
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_, err = p.Call("decide", `{"pieces":{}}`, "{}", "{}", "0", "0")
	if err == nil {
		t.Fatal("a v2 attribute-shaped script ran against a v1 JSON observation")
	}
	if !strings.Contains(err.Error(), "has no .units field or method") {
		t.Fatalf("v2-against-v1 failed with %q, want a no-such-field error", err)
	}
}

// TestParallelSessionsShareInternedValuesWithoutRacing is the tournament
// case: many matches decide at once against the SAME package-level interned
// tables and the SAME compiled program, each with its own Session. Run under
// -race this proves the sharing model, not merely that it happens to work.
func TestParallelSessionsShareInternedValuesWithoutRacing(t *testing.T) {
	p, err := runtime.Compile(scriptC)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	reference := newNativeHost(newFixture())
	gen := reference.sess.Begin()
	reference.array.Rebind(gen)
	refResult, err := p.CallValues("decide", reference.obs, botvalue.None, botvalue.None, botvalue.None, botvalue.None)
	if err != nil {
		t.Fatalf("reference call: %v", err)
	}
	reference.sess.End()
	refFirst, _ := botvalue.TupleAt(refResult, 0)
	want, _ := botvalue.AsFloat(refFirst)

	const goroutines, calls = 32, 40
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each match owns its own Session, rows and source; only the
			// interned tables and the compiled Program are shared.
			h := newNativeHost(newFixture())
			for i := 0; i < calls; i++ {
				gen := h.sess.Begin()
				h.array.Rebind(gen)
				result, err := p.CallValues("decide", h.obs, botvalue.None, botvalue.None, botvalue.None, botvalue.None)
				h.sess.End()
				if err != nil {
					errs <- err
					return
				}
				first, _ := botvalue.TupleAt(result, 0)
				got, _ := botvalue.AsFloat(first)
				if got != want {
					errs <- fmt.Errorf("score %v, want bit-identical %v", got, want)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("parallel decision: %v", err)
	}
}
