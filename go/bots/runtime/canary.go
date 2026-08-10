// Copyright 2026 Sneat.app

package runtime

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// canarySource is REQ:dialect-canary's shared Starlark source: the exact
// same text runs, unmodified, on both targets (native Go and the
// TinyGo-wasm build) under the ONE DialectOptions value dialect.go exports.
// Every assertion here reproduces a divergence
// spec/decisions/0007-local-battle-runtime.md's determinism spike or the
// 2026-08-06 TinyGo session actually observed — this is not a general
// Starlark conformance suite, it is the specific footgun list.
//
// Each check calls fail() with a message naming what it checked — but that
// message is what runCanary sees ONLY when the dialect option the check
// depends on is actually ENABLED and the runtime assertion itself proves
// false (a bug in this source, or a real semantic divergence). When the
// option is DISABLED instead (Set:false, say — the shape
// TestRunCanaryFailsWithoutSet exercises), go.starlark.net's own resolver
// rejects the disallowed construct (`set(...)`) before the function
// containing the fail() call ever runs at all, with an error naming the
// missing feature directly ("this Starlark dialect does not support
// sets") rather than "check 1" — fail()'s own message is never reached in
// that case. Both are real failure shapes this canary must catch; only one
// of them speaks in this source's own vocabulary.
const canarySource = `
def _run_canary():
    # 1. set() is available and behaves as a set — the exact symptom the
    #    legacy dialect API's silent no-op produced (decision 0007: "set()
    #    rejected under the legacy API").
    s = set([1, 2, 2, 3])
    if len(s) != 3:
        fail("check 1 (set availability): len(set([1,2,2,3])) ==", len(s), "want 3")
    if 2 not in s:
        fail("check 1 (set availability): 2 not in set([1,2,2,3])")

    # 2. while loops are permitted.
    total = 0
    i = 0
    while i < 5:
        total += i
        i += 1
    if total != 10:
        fail("check 2 (while loops): sum was", total, "want 10")

    # 3. String indexing is by UTF-8 BYTE, not codepoint. The two bytes of
    #    "é" (U+00E9, UTF-8 0xC3 0xA9) must be visible and distinct — a
    #    codepoint-indexed dialect (starlark-rust's disqualifying divergence
    #    in the determinism spike) would report length 1 and could not
    #    slice them apart.
    two_byte = "é"
    if len(two_byte) != 2:
        fail("check 3 (byte indexing): len(\"é\") ==", len(two_byte), "want 2")
    if two_byte[0:1] == two_byte[1:2]:
        fail("check 3 (byte indexing): the two UTF-8 bytes of \"é\" compared equal")

    # 4. Integers are arbitrary-precision: 2**64 (written as 1 << 64 —
    #    Starlark has no ** operator; ** is call syntax for keyword-dict
    #    expansion, not exponentiation) is exact, and an integer past
    #    2**53 compares exactly against a float rather than being silently
    #    rounded to one before the comparison.
    if (1 << 64) != 18446744073709551616:
        fail("check 4 (arbitrary precision): 1<<64 ==", 1 << 64, "want 18446744073709551616")
    if ((1 << 53) + 1) == 9007199254740992.0:
        fail("check 4 (exact int/float compare): (1<<53)+1 compared equal to 1<<53 as a float")

    # 5. Starlark's NaN total order holds under sorted(): NaN sorts as the
    #    GREATEST value ("totally ordered with NaN > +Inf").
    ordered = sorted([float("nan"), 1.0, -1.0])
    if ordered[0] != -1.0 or ordered[1] != 1.0:
        fail("check 5 (NaN total order): finite values misordered:", ordered)
    if str(ordered[2]) != "nan":
        fail("check 5 (NaN total order): NaN did not sort last:", ordered)

    # 6. sorted() is stable: sorting a list of equal-key pairs preserves
    #    input order.
    pairs = [(1, "a"), (1, "b"), (1, "c"), (0, "d")]
    stable = sorted(pairs, key=lambda p: p[0])
    if stable != [(0, "d"), (1, "a"), (1, "b"), (1, "c")]:
        fail("check 6 (stable sort): got", stable)

    return True

canary_passed = _run_canary()
`

// recursionProbeSource is checked SEPARATELY from canarySource, and that
// split is structural, not stylistic. go.starlark.net rejects self-
// recursion with a RUNTIME check (starlark/interp.go's CallInternal scans
// the thread's own call stack for the same function code before making the
// call) rather than a static one — so the probe must actually CALL the
// recursive function, once, for the check to run at all; merely DEFINING
// a self-recursive def and never calling it compiles and executes fine
// under either dialect and would prove nothing. The call is bounded (n
// counts down from 3), so it terminates in three frames if recursion is
// (wrongly) allowed rather than hanging.
//
// A self-recursive def sharing canarySource's own file would still be a
// problem even with the check being dynamic, not static: reaching this
// def's call at all requires _run_canary() to have already returned
// normally past every earlier check, so a recursion failure here could
// never be "the first assertion that failed" if it shared the file with
// checks 1-6 — it would always be the LAST thing that could possibly run.
// Keeping it in its own file makes that ordering explicit rather than
// accidental.
//
// PASSING the canary requires this call to FAIL: self-recursion REJECTED
// is go.starlark.net's default, and was one of the six semantic
// divergences that disqualified starlark-rust (whose default is the
// opposite) in decision 0007's determinism spike.
const recursionProbeSource = `
def _f(n):
    if n <= 0:
        return 0
    return _f(n - 1)

_f(3)
`

// RunCanary executes REQ:dialect-canary's checks under DialectOptions and
// returns nil only if every one of them held. Call it once, at runtime
// construction (New), before any tier script runs — never per decision.
//
// A THIN WRAPPER OVER runCanary, deliberately: RunCanary itself takes no
// parameter and is exactly what New calls in production, but the real
// checking logic lives in runCanary(opts), which DOES take one — see that
// function's own doc comment for why the split exists (canary_test.go's
// own TestRunCanaryCheck7IsPresentAndEnforced is the test this split makes
// possible).
func RunCanary() error {
	return runCanary(DialectOptions)
}

// runCanary is RunCanary's own implementation, parameterized on opts so a
// test can inject alternate options and observe check 7's behaviour
// through THIS EXACT function — never a hand-copied duplicate of its
// logic. Without this split, check 7 (the block below testing self-
// recursion) was reachable ONLY through RunCanary()'s own hardcoded
// DialectOptions, which never allows recursion — so check 7 could never
// actually FIRE via the public API, and deleting the whole block would
// leave every existing test (which only ever calls RunCanary(), or tests
// the recursion probe's SOURCE in isolation via a direct
// starlark.ExecFileOptions call — never runCanary/RunCanary itself)
// green. TestRunCanaryCheck7IsPresentAndEnforced calls this function
// directly with Recursion:true injected, which only reaches check 7's own
// code path if it is still there.
func runCanary(opts *syntax.FileOptions) error {
	thread := &starlark.Thread{Name: "starlarkbot-canary"}
	globals, err := starlark.ExecFileOptions(opts, thread, "canary.star", canarySource, nil)
	if err != nil {
		return fmt.Errorf("starlarkbot: dialect canary failed: %w", err)
	}
	if passed, ok := globals["canary_passed"].(starlark.Bool); !ok || !bool(passed) {
		return fmt.Errorf("starlarkbot: dialect canary did not report success (canary_passed = %v)", globals["canary_passed"])
	}

	// Check 7: self-recursion must be REJECTED. A nil error here means it
	// was silently ALLOWED, which is itself the canary failure. Rejected
	// at CALL time, not resolve time — go.starlark.net's recursion check is
	// dynamic (recursionProbeSource's own doc comment explains why the
	// probe must actually invoke the recursive function for this to fire
	// at all), so a permissive opts value (Recursion: true) lets this
	// ExecFileOptions call both compile AND run to completion without
	// error, which is exactly the condition this treats as a failure.
	recursionThread := &starlark.Thread{Name: "starlarkbot-canary-recursion-probe"}
	if _, err := starlark.ExecFileOptions(opts, recursionThread, "canary-recursion-probe.star", recursionProbeSource, nil); err == nil {
		return fmt.Errorf("starlarkbot: dialect canary failed: check 7 (self-recursion rejected): a self-recursive def compiled and ran without error, want rejected when called")
	}

	return nil
}
