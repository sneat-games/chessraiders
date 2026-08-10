// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

// A replayer that only ever reports agreement is not proven to detect
// disagreement — it could pass by never actually comparing anything. Every
// test below takes one REAL, unmutated corpus case (which
// TestCorpusReplayAgreesWithThePublishedScript already proves replays
// clean), breaks exactly one field of an in-memory copy, and asserts the
// SAME functions that test relies on — checkCorpusMetadata, replayCase — now
// return an error that names the case and the broken field. None of these
// write to testdata/corpus: every mutation lives on a copy held only in this
// process's memory, so the corpus itself is never touched.

// sampleCase loads the corpus's first case (by sorted path) for these tests
// to copy and break. Fatalf if none exists — same reasoning as
// TestCorpusReplayAgreesWithThePublishedScript's own empty-corpus guard:
// these tests need a real case to perturb, and "no corpus" would otherwise
// make every test below vacuously pass by never entering its own assertion.
func sampleCase(t *testing.T) (string, corpusCase) {
	t.Helper()
	entries := corpusEntries(t)
	if len(entries) == 0 {
		t.Fatal("no corpus case available to perturb — see TestCorpusReplayAgreesWithThePublishedScript")
	}
	return entries[0], readCorpusCase(t, entries[0])
}

// TestReplayCaseDetectsAWrongRecordedIntent perturbs the one field that
// matters most: what the corpus claims decide() returned.
func TestReplayCaseDetectsAWrongRecordedIntent(t *testing.T) {
	path, c := sampleCase(t)
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatalf("runtime.Compile: %v", err)
	}

	// Sanity first: prove the UNMUTATED case agrees, so a later failure to
	// detect the mutation can only be the detector's fault, not a sample
	// that was already broken.
	if err := replayCase(program, path, c); err != nil {
		t.Fatalf("sample case %s does not replay clean before perturbation (fix the sample, not this test): %v", path, err)
	}

	original := string(c.Intent)
	c.Intent = json.RawMessage(`{"kind":"move","from":"a1","to":"h8"}`)
	if string(c.Intent) == original {
		t.Fatal("perturbation produced the same intent as the original — pick a different fixed value")
	}

	err = replayCase(program, path, c)
	if err == nil {
		t.Fatal("replayCase accepted a deliberately wrong recorded intent — the disagreement detector is not detecting disagreement")
	}
	if !strings.Contains(err.Error(), "intent differs") {
		t.Errorf("replayCase's error does not name the mismatching field (\"intent differs\"): %v", err)
	}
	if !strings.Contains(err.Error(), c.Test) {
		t.Errorf("replayCase's error does not name the case (%q): %v", c.Test, err)
	}
	t.Logf("confirmed detection: %v", err)
}

// TestReplayCaseDetectsAPerturbedParameterRow perturbs `parameters` rather
// than the recorded intent directly — proving the detector also catches a
// disagreement that arises from decide() itself scoring differently, not
// only from a hand-edited intent field. TestBotClearsAFriendlyBlockerToDeliverTheCapturedKing.0
// is used by name (not "entries[0]") because the perturbation below —
// zeroing `delivery`, the one weight that gates the whole "escort a captive
// home" category tier.star's own scoring reads (go/bots/standardbot/README.md's
// own weight table) — is verified, empirically, against THIS specific
// case's recorded decision to flip it; an arbitrary case's own decision is
// not guaranteed to be sensitive to any one weight, so asserting this
// against "whichever case sorts first" would be exactly the kind of
// probably-vacuous check this task was warned against.
func TestReplayCaseDetectsAPerturbedParameterRow(t *testing.T) {
	const wantTest = "TestBotClearsAFriendlyBlockerToDeliverTheCapturedKing"
	entries := corpusEntries(t)
	var path string
	var c corpusCase
	found := false
	for _, p := range entries {
		candidate := readCorpusCase(t, p)
		if candidate.Test == wantTest && candidate.Case == 0 {
			path, c, found = p, candidate, true
			break
		}
	}
	if !found {
		t.Fatalf("corpus has no case named %s.0 — this test's perturbation was verified against that specific "+
			"case; if it was renamed or removed, re-verify the perturbation against its replacement and update "+
			"wantTest, do not delete this test", wantTest)
	}

	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatalf("runtime.Compile: %v", err)
	}
	if err := replayCase(program, path, c); err != nil {
		t.Fatalf("sample case %s does not replay clean before perturbation (fix the sample, not this test): %v", path, err)
	}

	var params map[string]json.RawMessage
	if err := json.Unmarshal(c.Parameters, &params); err != nil {
		t.Fatalf("decode sample parameters: %v", err)
	}
	if _, ok := params["delivery"]; !ok {
		t.Fatal(`sample parameters have no "delivery" key to perturb`)
	}
	params["delivery"] = json.RawMessage("0")
	perturbed, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("re-encode perturbed parameters: %v", err)
	}
	c.Parameters = perturbed

	err = replayCase(program, path, c)
	if err == nil {
		t.Fatal("replayCase accepted a decision replayed against a perturbed parameter row that should have changed it — the disagreement detector is not detecting disagreement")
	}
	if !strings.Contains(err.Error(), "intent differs") {
		t.Errorf("replayCase's error does not name the mismatching field (\"intent differs\"): %v", err)
	}
	if !strings.Contains(err.Error(), wantTest) {
		t.Errorf("replayCase's error does not name the case (%q): %v", wantTest, err)
	}
	t.Logf("confirmed detection: %v", err)
}

// TestCheckCorpusMetadataDetectsAWrongScriptModule perturbs script.module —
// proving a case recorded against a different module entirely is rejected
// before replayCase ever runs it, rather than silently accepted because
// decide() happened to still agree.
func TestCheckCorpusMetadataDetectsAWrongScriptModule(t *testing.T) {
	path, c := sampleCase(t)
	modulePath := thisModulePath(t)

	if err := checkCorpusMetadata(path, c, modulePath, c.Script.Version); err != nil {
		t.Fatalf("sample case %s does not pass metadata checks before perturbation (fix the sample, not this test): %v", path, err)
	}

	c.Script.Module = "github.com/example/not-the-real-module"
	err := checkCorpusMetadata(path, c, modulePath, c.Script.Version)
	if err == nil {
		t.Fatal("checkCorpusMetadata accepted a case declaring a different module entirely — the guard is not guarding")
	}
	if !strings.Contains(err.Error(), "script.module") {
		t.Errorf("checkCorpusMetadata's error does not name the mismatching field (\"script.module\"): %v", err)
	}
	if !strings.Contains(err.Error(), c.Test) {
		t.Errorf("checkCorpusMetadata's error does not name the case (%q): %v", c.Test, err)
	}
	t.Logf("confirmed detection: %v", err)
}

// TestCheckCorpusMetadataDetectsAnInconsistentScriptVersion perturbs
// script.version — proving a corpus that quietly mixed two script versions
// (one case recorded against a different release than the rest) is caught,
// not silently averaged into "the corpus" as if it were one thing.
func TestCheckCorpusMetadataDetectsAnInconsistentScriptVersion(t *testing.T) {
	path, c := sampleCase(t)
	modulePath := thisModulePath(t)
	realVersion := c.Script.Version

	if err := checkCorpusMetadata(path, c, modulePath, realVersion); err != nil {
		t.Fatalf("sample case %s does not pass metadata checks before perturbation (fix the sample, not this test): %v", path, err)
	}

	c.Script.Version = "v99.99.99"
	if c.Script.Version == realVersion {
		t.Fatal("perturbation produced the same version as the original — pick a different fixed value")
	}
	err := checkCorpusMetadata(path, c, modulePath, realVersion)
	if err == nil {
		t.Fatal("checkCorpusMetadata accepted a script.version that disagrees with the rest of the corpus — the guard is not guarding")
	}
	if !strings.Contains(err.Error(), "script.version") {
		t.Errorf("checkCorpusMetadata's error does not name the mismatching field (\"script.version\"): %v", err)
	}
	if !strings.Contains(err.Error(), c.Test) {
		t.Errorf("checkCorpusMetadata's error does not name the case (%q): %v", c.Test, err)
	}
	t.Logf("confirmed detection: %v", err)
}
