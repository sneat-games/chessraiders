// Copyright 2026 Sneat.app

package runtime

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// concurrencySource is deliberately busy across every axis Program's own
// CONCURRENCY doc comment (program.go) measures a shared thread against: a
// `while` loop (thread.stack depth plus the step counter), `set()` (a
// dialect-sensitive builtin — decision 0007's own determinism spike), and a
// bounded chain of FIVE DISTINCT, non-self-recursive functions (thread.stack
// depth again, from a different direction than the loop). It deliberately
// does NOT use genuine self-recursion: this package's own DialectOptions
// (dialect.go) sets Recursion:false, so go.starlark.net rejects a function
// that calls itself — directly OR by calling back into any function whose
// code already appears anywhere in the current thread's call stack — and
// the canary (canary.go's check 7) already pins that rejection as CORRECT
// behaviour. A test asserting "recursion works" would therefore either be
// wrong about this dialect or would have to assert every call fails, which
// contradicts proving "zero errors" below. step1..step5 give the same
// stack-depth exercise a bounded, LEGAL recursion would, without tripping
// that rejection. Every Program.Call also performs a JSON round trip on its
// own (decodeJSON for the one argument below, encodeJSON for the result),
// so nothing extra is needed to exercise that axis.
const concurrencySource = `
def step5(n):
    return n
def step4(n):
    return step5(n + 1)
def step3(n):
    return step4(n + 1)
def step2(n):
    return step3(n + 1)
def step1(n):
    return step2(n + 1)

def decide(seed):
    s = set()
    total = 0
    i = 0
    while i < 50:
        s.add(i % 7)
        total += i
        i += 1
    return {"total": total, "distinct": len(s), "chained": step1(seed), "seed": seed}
`

// decideResult is concurrencySource's own decide() output, decoded through
// encoding/json rather than compared as a raw string — Starlark dict
// iteration order is insertion order today, but asserting on FIELD VALUES
// here is the right level of coupling regardless of json.encode's own key
// ordering.
type decideResult struct {
	Total    int `json:"total"`
	Distinct int `json:"distinct"`
	Chained  int `json:"chained"`
	Seed     int `json:"seed"`
}

// TestProgramCallConcurrentUseIsRaceFree is
// AC:one-fresh-thread-serves-a-whole-call's own proof, run against THIS
// package's real, shipped Program: one COMPILED Program (compiled exactly
// ONCE, exactly like server-go/starlarkbot4chess's own package-level
// sync.Once compiles the real chess-raiders-bot.star exactly once and shares it across
// every match), called from 64 goroutines making 200 calls each — 12,800
// calls total — must produce zero errors and zero result mismatches when
// run under `go test -race`.
//
// This is meaningless without -race: two goroutines racing on
// *starlark.Thread's own unexported fields (thread.stack, in particular —
// program.go's own doc comment on why) usually still produces a plausible-
// looking (wrong or right) result rather than a visible failure on a normal
// `go test` run; only the race detector (or the SIGSEGV this package's own
// commit history records — 19 data races on thread.stack, then a crash in
// starlark.(*frame).asCallFrame, probed against the shared-thread shape this
// file's sibling change replaced) makes the defect observable at all. The
// repo's own Go loop (AGENTS.md) runs `go test ./starlarkbot/...` for a fast
// inner loop and a full `-race` pass before merge; this test is exactly the
// reason that full pass matters for this package.
func TestProgramCallConcurrentUseIsRaceFree(t *testing.T) {
	prog, err := Compile(concurrencySource)
	if err != nil {
		t.Fatalf("Compile() = %v, want a compiled program", err)
	}

	const goroutines = 64
	const callsPerGoroutine = 200

	type failure struct {
		goroutine, call int
		err             error
		got             string
	}
	failures := make(chan failure, goroutines*callsPerGoroutine)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for c := 0; c < callsPerGoroutine; c++ {
				out, err := prog.Call("decide", strconv.Itoa(seed))
				if err != nil {
					failures <- failure{seed, c, fmt.Errorf("Call() = %w, want a JSON result", err), ""}
					continue
				}
				var got decideResult
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					failures <- failure{seed, c, fmt.Errorf("Call() output %q did not decode: %w", out, err), out}
					continue
				}
				want := decideResult{Total: 1225, Distinct: 7, Chained: seed + 4, Seed: seed}
				if got != want {
					failures <- failure{seed, c, nil, out}
				}
			}
		}(g)
	}
	wg.Wait()
	close(failures)

	var n int
	for f := range failures {
		n++
		if f.err != nil {
			t.Errorf("goroutine %d call %d: %v", f.goroutine, f.call, f.err)
		} else {
			t.Errorf("goroutine %d call %d: Call() = %q, mismatched result", f.goroutine, f.call, f.got)
		}
	}
	if n > 0 {
		t.Fatalf("%d of %d concurrent calls failed or mismatched", n, goroutines*callsPerGoroutine)
	}
}

// TestConcurrentProgramsWithDifferentStepBudgetsCancelAtTheirOwn is
// AC:one-fresh-thread-serves-a-whole-call's "ten concurrent calls with
// different SetMaxExecutionSteps budgets each cancel at precisely their
// own", exercised through CompileWithStepLimit/Call rather than
// CallWithStepLimit (program.go's own later addition, for a caller — the
// server bot resolver — whose ceiling varies call to call against one
// SHARED Program; that method has its own equivalent coverage in
// program_test.go). CompileWithStepLimit bakes maxSteps into a *Program at
// compile time, so "different budgets" here means ten DIFFERENT Programs,
// each compiled with its own ceiling, called from ten goroutines at once.
// What this proves is exactly the property newThread (program.go) exists
// for: maxSteps lives on the *Program, each Program's own field is read
// only by that Program's own calls, and building a fresh thread per Call
// means no goroutine can ever observe — let alone spend down — a ceiling
// that belongs to a different Program's concurrent call.
func TestConcurrentProgramsWithDifferentStepBudgetsCancelAtTheirOwn(t *testing.T) {
	const spinSource = `
def spin():
    x = 0
    while True:
        x = x + 1
    return x
`
	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		budget := uint64(1_000 * (i + 1))
		prog, err := CompileWithStepLimit(spinSource, budget)
		if err != nil {
			t.Fatalf("CompileWithStepLimit(%d) = %v", budget, err)
		}
		wg.Add(1)
		go func(i int, prog *Program) {
			defer wg.Done()
			_, err := prog.Call("spin")
			errs[i] = err
		}(i, prog)
	}
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Errorf("program %d: Call() succeeded, want its own step-limit error", i)
			continue
		}
		if !strings.Contains(err.Error(), "too many steps") {
			t.Errorf("program %d: Call() error = %v, want it to name the step-limit cancellation", i, err)
		}
	}
}
