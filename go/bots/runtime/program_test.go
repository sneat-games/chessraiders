// Copyright 2026 Sneat.app

package runtime

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestProgramCallRoundTrip is Program's own acceptance test: compile once,
// call a global function with JSON arguments, get a JSON result back —
// the direct-call shape tier.star's own bottom adapter block existed only
// to simulate.
func TestProgramCallRoundTrip(t *testing.T) {
	const source = `
def add(a, b):
    return a["x"] + b["y"]
`
	prog, err := Compile(source)
	if err != nil {
		t.Fatalf("Compile() = %v, want a compiled program", err)
	}
	out, err := prog.Call("add", `{"x": 2}`, `{"y": 3}`)
	if err != nil {
		t.Fatalf("Call() = %v, want a JSON result", err)
	}
	if out != "5" {
		t.Fatalf("Call() = %q, want %q", out, "5")
	}
}

// TestProgramCallReturnsATupleAsAJSONArray proves the multi-return-value
// shape decide() itself uses (`return chosen_intent, updated_memory`) comes
// back as a two-element JSON array, in order — never flattened, never
// re-ordered.
func TestProgramCallReturnsATupleAsAJSONArray(t *testing.T) {
	const source = `
def pair():
    return {"a": 1}, {"b": 2}
`
	prog, err := Compile(source)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	out, err := prog.Call("pair")
	if err != nil {
		t.Fatalf("Call() = %v", err)
	}
	var got []map[string]int
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("Call() output %q did not decode as a JSON array: %v", out, err)
	}
	if len(got) != 2 || got[0]["a"] != 1 || got[1]["b"] != 2 {
		t.Fatalf("Call() = %q, want a 2-element array [{\"a\":1},{\"b\":2}]", out)
	}
}

// TestProgramCallInvokesTheAlreadyCompiledModuleRepeatedly proves the whole
// point of compile-once: calling the SAME already-compiled Program many
// times never re-runs the module's own top-level statements. A top-level
// counter that only runs once (module exec) but is READ many times (each
// Call) would report the same value on every call if — and only if — the
// module body genuinely executed exactly once.
func TestProgramCallInvokesTheAlreadyCompiledModuleRepeatedly(t *testing.T) {
	const source = `
_compile_count = 1

def compile_count():
    return _compile_count
`
	prog, err := Compile(source)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	for i := 0; i < 5; i++ {
		out, err := prog.Call("compile_count")
		if err != nil {
			t.Fatalf("Call() #%d = %v", i, err)
		}
		if out != "1" {
			t.Fatalf("Call() #%d = %q, want %q (module top level must run exactly once, at Compile)", i, out, "1")
		}
	}
}

// TestProgramCallRejectsUnknownGlobal proves Call fails loudly (not with a
// nil/empty result) when asked to invoke a name the module never bound.
func TestProgramCallRejectsUnknownGlobal(t *testing.T) {
	prog, err := Compile(`x = 1`)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	if _, err := prog.Call("does_not_exist"); err == nil {
		t.Fatal("Call() on an unbound global succeeded, want an error")
	} else if !strings.Contains(err.Error(), "does_not_exist") {
		t.Fatalf("Call() error = %q, want it to name the missing global", err.Error())
	}
}

// TestProgramCallRejectsANonCallableGlobal proves Call distinguishes "no
// such global" from "that global exists but is not a function".
func TestProgramCallRejectsANonCallableGlobal(t *testing.T) {
	prog, err := Compile(`decide = 42`)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	if _, err := prog.Call("decide"); err == nil {
		t.Fatal("Call() on a non-callable global succeeded, want an error")
	}
}

// TestCompileRejectsScriptError proves Compile itself fails on a syntax
// error rather than deferring the failure to the first Call.
func TestCompileRejectsScriptError(t *testing.T) {
	if _, err := Compile(`this is not valid starlark ///`); err == nil {
		t.Fatal("Compile() with invalid source succeeded, want an error")
	}
}

// TestCompileWithStepLimitBoundsAnInfiniteLoop proves the whole point of
// CompileWithStepLimit: a script whose Call would otherwise never return
// (`while True: pass` — no budget check inside a plain interpreter loop is
// ever reached from a hand-placed Charge, unlike bot4chess's own Go tiers)
// fails FAST, with a clear message, instead of hanging the calling
// goroutine forever.
func TestCompileWithStepLimitBoundsAnInfiniteLoop(t *testing.T) {
	const source = `
def spin():
    x = 0
    while True:
        x = x + 1
    return x
`
	prog, err := CompileWithStepLimit(source, 10_000)
	if err != nil {
		t.Fatalf("CompileWithStepLimit() = %v", err)
	}
	_, err = prog.Call("spin")
	if err == nil {
		t.Fatal("Call() on an infinite loop succeeded, want a step-limit error")
	}
	if !strings.Contains(err.Error(), "too many steps") {
		t.Fatalf("Call() error = %q, want it to name the step-limit cancellation", err.Error())
	}
}

// TestCompileWithStepLimitIsPerCallNotPerProgram proves the budget resets
// on every Call rather than being spent down, once, across a whole
// session's worth of decisions — a script within budget on decision one
// must still be within budget on decision one thousand.
func TestCompileWithStepLimitIsPerCallNotPerProgram(t *testing.T) {
	const source = `
def modest():
    x = 0
    i = 0
    while i < 100:
        x = x + i
        i = i + 1
    return x
`
	prog, err := CompileWithStepLimit(source, 5_000)
	if err != nil {
		t.Fatalf("CompileWithStepLimit() = %v", err)
	}
	for i := 0; i < 50; i++ {
		out, err := prog.Call("modest")
		if err != nil {
			t.Fatalf("Call() #%d = %v, want the budget to be fresh on every call", i, err)
		}
		if out != "4950" {
			t.Fatalf("Call() #%d = %q, want %q", i, out, "4950")
		}
	}
}

// TestCompileWithStepLimitLeavesAnOrdinaryProgramUnbounded proves
// CompileWithStepLimit is additive: Compile's own Programs are completely
// unaffected by this field existing at all.
func TestCompileWithStepLimitLeavesAnOrdinaryProgramUnbounded(t *testing.T) {
	const source = `
def modest():
    x = 0
    i = 0
    while i < 200000:
        x = x + i
        i = i + 1
    return x
`
	prog, err := Compile(source)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	if _, err := prog.Call("modest"); err != nil {
		t.Fatalf("Call() = %v, want an ordinary Compile()'d Program to stay unbounded", err)
	}
}

// TestCallWithStepLimitBoundsOneCallAndReportsItsCost proves
// CallWithStepLimit's whole reason for existing over CompileWithStepLimit:
// ONE ordinary, unbounded Compile()'d Program can be called twice with two
// DIFFERENT ceilings, and each call is bounded (or not) by the ceiling THAT
// call passed — never by anything baked in at Compile — while also reporting
// how many steps it actually spent, on both the successful and the
// cancelled call.
func TestCallWithStepLimitBoundsOneCallAndReportsItsCost(t *testing.T) {
	const modest = `
def modest():
    x = 0
    i = 0
    while i < 100:
        x = x + i
        i = i + 1
    return x
`
	prog, err := Compile(modest)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}

	out, steps, err := prog.CallWithStepLimit(5_000, "modest")
	if err != nil {
		t.Fatalf("CallWithStepLimit(5000) = %v, want it to complete inside its own generous ceiling", err)
	}
	if out != "4950" {
		t.Fatalf("CallWithStepLimit(5000) = %q, want %q", out, "4950")
	}
	if steps == 0 {
		t.Fatal("CallWithStepLimit(5000) reported 0 steps, want the real cost of the call")
	}
	if steps >= 5_000 {
		t.Fatalf("CallWithStepLimit(5000) reported %d steps, want strictly under its own ceiling", steps)
	}

	_, tinySteps, err := prog.CallWithStepLimit(3, "modest")
	if err == nil {
		t.Fatal("CallWithStepLimit(3) succeeded, want a step-limit error against a 3-step ceiling")
	}
	if !strings.Contains(err.Error(), "too many steps") {
		t.Fatalf("CallWithStepLimit(3) error = %q, want it to name the step-limit cancellation", err.Error())
	}
	if tinySteps < 3 {
		t.Fatalf("CallWithStepLimit(3) reported %d steps on a cancelled call, want at least its own ceiling", tinySteps)
	}

	// The SAME Program, called with maxSteps == 0, stays unbounded — proving
	// this method never mutates p.maxSteps (which is 0 on this Program
	// regardless) and never leaks one call's ceiling into the next.
	if _, _, err := prog.CallWithStepLimit(0, "modest"); err != nil {
		t.Fatalf("CallWithStepLimit(0) = %v, want maxSteps == 0 to mean unbounded", err)
	}
}

// TestCallWithStepLimitConcurrentUseIsRaceFree is
// AC:one-fresh-thread-serves-a-whole-call's own proof for THIS method
// specifically: many goroutines calling CallWithStepLimit against the SAME
// compiled Program with DIFFERENT per-call ceilings, under `go test -race` —
// the exact shape server-go/starlarkbot4chess's production resolver uses
// (one compiled tier.star, one budget per decision, many matches deciding
// concurrently). concurrency_test.go's own
// TestConcurrentProgramsWithDifferentStepBudgetsCancelAtTheirOwn proves the
// equivalent property for CompileWithStepLimit/Call (ten SEPARATE Programs);
// this proves it for ONE Program shared across every goroutine instead,
// which is the shape this method exists for.
func TestCallWithStepLimitConcurrentUseIsRaceFree(t *testing.T) {
	prog, err := Compile(concurrencySource)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}

	const goroutines = 32
	type result struct {
		goroutine int
		err       error
		steps     uint64
		ceiling   uint64
	}
	results := make(chan result, goroutines)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			// Every goroutine gets its own ceiling, deliberately mixing
			// "comfortably wide" (never cancels) and "tiny" (always cancels)
			// so a race that smears one goroutine's ceiling onto another's
			// thread would show up as an unexpected success or failure.
			ceiling := uint64(20 + (g % 3)) // 20/21/22 — the loop alone needs far more, so these always cancel
			if g%4 == 0 {
				ceiling = 100_000 // comfortably wide — never cancels
			}
			_, steps, err := prog.CallWithStepLimit(ceiling, "decide", strconv.Itoa(g))
			results <- result{g, err, steps, ceiling}
		}(g)
	}
	wg.Wait()
	close(results)

	for r := range results {
		wantCancel := r.ceiling < 1000
		gotCancel := r.err != nil
		if wantCancel != gotCancel {
			t.Errorf("goroutine %d: ceiling %d, err = %v (wantCancel=%v)", r.goroutine, r.ceiling, r.err, wantCancel)
			continue
		}
		if wantCancel && !strings.Contains(r.err.Error(), "too many steps") {
			t.Errorf("goroutine %d: error = %q, want it to name the step-limit cancellation", r.goroutine, r.err.Error())
		}
		if r.steps == 0 {
			t.Errorf("goroutine %d: reported 0 steps, want a real cost either way", r.goroutine)
		}
	}
}

// TestHasFunctionDistinguishesPresentMissingAndNonCallable proves the
// load-time existence check bot lab's own CompileCustomTier relies on.
func TestHasFunctionDistinguishesPresentMissingAndNonCallable(t *testing.T) {
	prog, err := Compile("def decide():\n    return None\nnotAFunction = 42\n")
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	if !prog.HasFunction("decide") {
		t.Error(`HasFunction("decide") = false, want true`)
	}
	if prog.HasFunction("does_not_exist") {
		t.Error(`HasFunction("does_not_exist") = true, want false`)
	}
	if prog.HasFunction("notAFunction") {
		t.Error(`HasFunction("notAFunction") = true, want false (it is an int, not callable)`)
	}
}

// TestProgramCallUsesTheOneDialectConstant proves Compile honours
// DialectOptions (Set/While) exactly like EvalScript and the canary do.
func TestProgramCallUsesTheOneDialectConstant(t *testing.T) {
	const source = `
def run():
    seen = set()
    i = 0
    while i < 3:
        seen.add(i)
        i += 1
    return len(seen)
`
	prog, err := Compile(source)
	if err != nil {
		t.Fatalf("Compile() = %v, want set()/while to be usable", err)
	}
	out, err := prog.Call("run")
	if err != nil {
		t.Fatalf("Call() = %v", err)
	}
	if out != "3" {
		t.Fatalf("Call() = %q, want %q", out, "3")
	}
}
