// Copyright 2026 Sneat.app

package runtime

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// TestRunCanaryPasses is AC:the-canary-catches-a-wrong-dialect's positive
// half over DialectOptions itself: "the same canary passes on the
// correctly-configured builds of both targets". This test runs on native
// Go; server-go/cmd/chesswasm's with-bots build (TinyGo) is where the SAME
// canarySource/recursionProbeSource constants run on the other target —
// there is exactly one canary source, shared by both, per REQ:dialect-
// canary.
func TestRunCanaryPasses(t *testing.T) {
	if err := RunCanary(); err != nil {
		t.Fatalf("RunCanary() under DialectOptions = %v, want nil", err)
	}
}

// TestRunCanaryFailsWithoutSet is AC:the-canary-catches-a-wrong-dialect's
// negative half: "a build of each target deliberately configured with the
// legacy dialect path so its options silently no-op ... the canary fails
// ... naming the first assertion that failed". The measured fact this
// tree is built on (this module's README) is that
// syntax.LegacyFileOptions() reads resolve.AllowSet as true under stock
// Go and silently as false under TinyGo — so a Go-only test cannot
// reproduce the TinyGo-specific no-op directly. What it CAN, and does,
// prove is that the canary is a real mechanism, not a tautology: pointed
// at an options value with Set false (exactly what the TinyGo no-op
// yields), it fails, names check 1, and does not silently pass.
func TestRunCanaryFailsWithoutSet(t *testing.T) {
	thread := &starlark.Thread{Name: "canary-without-set"}
	badOptions := &syntax.FileOptions{While: true} // Set omitted (false).
	_, err := starlark.ExecFileOptions(badOptions, thread, "canary.star", canarySource, nil)
	if err == nil {
		t.Fatal("ExecFileOptions(canarySource) with Set:false succeeded, want a resolve error naming 'set'")
	}
	if !strings.Contains(err.Error(), "set") {
		t.Fatalf("ExecFileOptions(canarySource) with Set:false failed with %q, want it to name 'set'", err.Error())
	}
}

// TestRunCanaryFailsWithoutWhile is TestRunCanaryFailsWithoutSet's twin for
// the While option.
func TestRunCanaryFailsWithoutWhile(t *testing.T) {
	thread := &starlark.Thread{Name: "canary-without-while"}
	badOptions := &syntax.FileOptions{Set: true} // While omitted (false).
	_, err := starlark.ExecFileOptions(badOptions, thread, "canary.star", canarySource, nil)
	if err == nil {
		t.Fatal("ExecFileOptions(canarySource) with While:false succeeded, want a resolve error rejecting `while`")
	}
}

// TestRunCanaryFailsWhenRecursionIsAllowed proves the recursion probe
// really is load-bearing: under Recursion:true (the option that DISABLES
// the compiler's recursion check, i.e. "self-recursion allowed" —
// starlark-rust's disqualifying default in decision 0007's determinism
// spike), the recursion probe must compile and run without error, which is
// exactly what RunCanary treats as a failure.
func TestRunCanaryFailsWhenRecursionIsAllowed(t *testing.T) {
	thread := &starlark.Thread{Name: "recursion-allowed-probe"}
	permissive := &syntax.FileOptions{Set: true, While: true, Recursion: true}
	if _, err := starlark.ExecFileOptions(permissive, thread, "canary-recursion-probe.star", recursionProbeSource, nil); err != nil {
		t.Fatalf("recursion probe under Recursion:true failed to even compile: %v — the probe itself may be broken", err)
	}
	// RunCanary always builds its recursion probe under the real
	// DialectOptions (Recursion left false), so this test does not call
	// RunCanary — it establishes that were DialectOptions ever loosened to
	// allow recursion, the probe would stop erroring and RunCanary's check 7
	// would correctly start failing.
}

// TestRunCanaryCheck7IsPresentAndEnforced is A4's own proof: "deleting ...
// the whole check-7 block in canary.go:130-135 leaves the suite green."
// Every test above it either calls RunCanary() (always under the real
// DialectOptions, which never allows recursion, so check 7's own code path
// inside RunCanary is never actually exercised by any of them) or tests
// recursionProbeSource in isolation via a direct starlark.ExecFileOptions
// call (proving the PROBE reacts to Recursion:true, never that RunCanary
// itself still runs it). Neither would notice check 7's block being
// deleted from runCanary's body.
//
// This test calls runCanary(permissive) DIRECTLY — the exact function
// RunCanary() itself calls, per canary.go's own doc comment on why that
// split exists — with Recursion:true injected. If check 7's block is ever
// removed from runCanary, this goes from failing-as-expected to a silent
// pass, exactly the mutation this test exists to catch.
func TestRunCanaryCheck7IsPresentAndEnforced(t *testing.T) {
	permissive := &syntax.FileOptions{Set: true, While: true, Recursion: true}
	err := runCanary(permissive)
	if err == nil {
		t.Fatal("runCanary(permissive) with Recursion:true succeeded — check 7 (self-recursion rejected) either was not run or was removed from runCanary's own body")
	}
	if !strings.Contains(err.Error(), "check 7") {
		t.Fatalf("runCanary(permissive) failed with %q, want it to name check 7", err.Error())
	}
}

// TestDialectOptionsRejectsTopLevelControl, TestDialectOptionsRejects
// GlobalReassign and TestDialectOptionsRejectsLoadBindsGlobally are A4's
// second proof: "a probe asserting the deliberately-false options
// (TopLevelControl, GlobalReassign, LoadBindsGlobally) really are
// rejected — the canary asserts none of them today." dialect.go's own doc
// comment claims these three are "left at their zero value, deliberately"
// because no tier script needs them — a claim about intent, not behaviour,
// until something actually tries the disallowed construct under the REAL
// DialectOptions (not a synthetic alternate, unlike the Set/While probes
// above) and confirms it is rejected.
func TestDialectOptionsRejectsTopLevelControl(t *testing.T) {
	thread := &starlark.Thread{Name: "toplevelcontrol-probe"}
	const source = `
if True:
    x = 1
`
	_, err := starlark.ExecFileOptions(DialectOptions, thread, "toplevelcontrol-probe.star", source, nil)
	if err == nil {
		t.Fatal("a top-level `if` compiled under DialectOptions — TopLevelControl is meant to stay false (dialect.go's own comment: \"no tier script needs a bare if/for/while ... at module top level\")")
	}
	if !strings.Contains(err.Error(), "not within a function") {
		t.Fatalf("top-level `if` failed with %q, want an error naming that it is not within a function", err.Error())
	}
}

func TestDialectOptionsRejectsGlobalReassign(t *testing.T) {
	thread := &starlark.Thread{Name: "globalreassign-probe"}
	const source = `
x = 1
x = 2
`
	_, err := starlark.ExecFileOptions(DialectOptions, thread, "globalreassign-probe.star", source, nil)
	if err == nil {
		t.Fatal("reassigning a top-level global compiled under DialectOptions — GlobalReassign is meant to stay false")
	}
	if !strings.Contains(err.Error(), "reassign") {
		t.Fatalf("global reassignment failed with %q, want an error naming the reassignment", err.Error())
	}
}

// TestDialectOptionsRejectsLoadBindsGlobally probes LoadBindsGlobally
// specifically, not merely "load() is unusable" (which would also be true
// under LoadBindsGlobally:true, since no loader function is wired into
// EvalScript's own starlark.Thread either way — that absence is a SEPARATE
// fact from this option). The distinguishing, resolve-time-observable
// behaviour: with LoadBindsGlobally false, a name bound by `load(...)` is
// FILE-LOCAL, so reassigning it hits the SAME "cannot reassign local ..."
// resolve error a local variable would (GlobalReassign governs globals
// only) — never "cannot reassign global ...". Verified once by hand against
// the pinned go.starlark.net version before writing this assertion: the
// error text says "local", confirming the binding really is local under
// this dialect, which is exactly what LoadBindsGlobally:false claims.
func TestDialectOptionsRejectsLoadBindsGlobally(t *testing.T) {
	thread := &starlark.Thread{Name: "loadbindsglobally-probe"}
	const source = `
load("mod.star", "y")
y = 2
`
	_, err := starlark.ExecFileOptions(DialectOptions, thread, "loadbindsglobally-probe.star", source, nil)
	if err == nil {
		t.Fatal("reassigning a load()-bound name compiled under DialectOptions — LoadBindsGlobally is meant to stay false, which should make that name file-local and therefore unreassignable under GlobalReassign:false")
	}
	if !strings.Contains(err.Error(), "reassign local") {
		t.Fatalf("load()-bound reassignment failed with %q, want an error naming a LOCAL reassignment (proving the binding is file-local, not global) — LoadBindsGlobally may have started behaving differently, or this probe is stale against the pinned go.starlark.net version", err.Error())
	}
}
