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
	"go.starlark.net/starlark"
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
			"    observation.units[0].defenders.append(observation.units[1])\n    return None, {}, []",
			"has no .append field or method",
		},
		"set a relation-list element": {
			"    observation.units[0].defenders[0] = 1\n    return None, {}, []",
			"units value does not support item assignment",
		},
		"set a relation list": {
			"    observation.units[0].defenders = 0\n    return None, {}, []",
			"can't assign to .defenders field of unit",
		},
		"set an int-list element": {
			"    observation.units[0].destination_files[0] = 1\n    return None, {}, []",
			"ints value does not support item assignment",
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

// TestMembershipWorksOnRelationArrays probes the `in` operator, which the
// production script uses against relation sets ("is the enemy king among
// the squares this unit threatens"). eval.go's Binary(IN) dispatches on
// Container, NOT on Sequence, so an indexable value that omits Has silently
// fails this — a gap worth a test rather than an assumption.
func TestMembershipWorksOnRelationArrays(t *testing.T) {
	got := runAttack(t, "    u = observation.units[0]\n    if u.destination_files[0] in u.destination_files:\n        return None, {}, []\n    fail(\"not found\")")
	if got != "" {
		t.Fatalf("`in` against an indexable array failed: %s", got)
	}
}

// TestBitShiftAllocationTrap probes whether a script walking a 32-bit mask
// with shifts stays inside the interpreter's small-int representation.
// go.starlark.net stores int32-range values as a pointer-sized union
// (int_posix64.go) and anything wider as a *big.Int, so 1 << 31 crosses out
// of the cheap representation on the LAST bit of a uint32 mask.
func TestBitShiftAllocationTrap(t *testing.T) {
	p, err := runtime.Compile(`
def decide(observation, memory, params, host_random_draw, options):
    total = 0
    for i in range(32):
        total += (1 << i) & 0
    return total, {}, []
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	allocs := testing.AllocsPerRun(200, func() {
		if _, err := p.Call("decide", "0", "0", "0", "0", "0"); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("a 32-iteration shift walk costs %.0f allocs/op", allocs)
	if allocs < 1 {
		t.Fatal("expected the shift walk to allocate; the trap this documents may have gone away")
	}
}

// TestRelationListAnswersCountAndMembersWithoutAShiftWalk is the fix for the
// trap above, and the apples-to-apples comparison against the custom set type
// this replaced: the SAME four operations over the SAME 32 units, so the cost
// of choosing a legible array over a clever set value is a number rather than
// an assertion. The set value measured 76 allocs/op here.
func TestRelationListAnswersCountAndMembersWithoutAShiftWalk(t *testing.T) {
	h := newNativeHost(newFixture())
	p, err := runtime.Compile(`
def decide(observation, memory, params, host_random_draw, options):
    total = 0
    for unit in observation.units:
        total += unit.defenders.count
        for other in unit.defenders:
            total += other.cargo_count
        if unit in unit.defenders:
            total += 1
        total += len([u for u in unit.defenders if u not in unit.attackers])
    return total, {}, []
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var last starlark.Value
	allocs := testing.AllocsPerRun(200, func() {
		gen := h.sess.Begin()
		h.array.Rebind(gen)
		v, err := p.CallValues("decide", h.obs, botvalue.None, botvalue.None, botvalue.None, botvalue.None)
		h.sess.End()
		if err != nil {
			t.Fatal(err)
		}
		last = v
	})
	first, _ := botvalue.TupleAt(last, 0)
	sum, _ := botvalue.AsInt(first)
	t.Logf("count + full member walk + membership + difference over %d units: %.0f allocs/op "+
		"(sum %d) — the custom set type measured 76 for the identical four operations",
		unitCount, allocs, sum)
	if sum == 0 {
		t.Fatal("the mask walk produced nothing, so this measured an empty loop")
	}
}
