// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"path/filepath"
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

func allCorpusCases(t *testing.T) []corpusCase {
	t.Helper()
	entries := corpusEntries(t)
	cases := make([]corpusCase, len(entries))
	for index, path := range entries {
		cases[index] = readCorpusCase(t, path)
	}
	return cases
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
// home" category chess-raiders-bot.star's own scoring reads (go/bots/standardbot/README.md's
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

	if err := checkCorpusMetadata(path, c, modulePath); err != nil {
		t.Fatalf("sample case %s does not pass metadata checks before perturbation (fix the sample, not this test): %v", path, err)
	}

	c.Script.Module = "github.com/example/not-the-real-module"
	err := checkCorpusMetadata(path, c, modulePath)
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

// TestCheckCorpusMetadataDetectsAWrongParamsVersion proves that a case whose
// resolved parameter row uses a different protocol cannot quietly enter the
// corpus merely because its current JSON object happens to look compatible.
func TestCheckCorpusMetadataDetectsAWrongParamsVersion(t *testing.T) {
	path, c := sampleCase(t)
	modulePath := thisModulePath(t)

	if err := checkCorpusMetadata(path, c, modulePath); err != nil {
		t.Fatalf("sample case %s does not pass metadata checks before perturbation (fix the sample, not this test): %v", path, err)
	}

	c.ParamsVersion = "not-the-resolved-row-contract/v1"
	err := checkCorpusMetadata(path, c, modulePath)
	if err == nil {
		t.Fatal("checkCorpusMetadata accepted a case declaring the wrong resolved-parameter-row protocol")
	}
	if !strings.Contains(err.Error(), "paramsVersion") {
		t.Errorf("checkCorpusMetadata's error does not name the mismatching field (\"paramsVersion\"): %v", err)
	}
}

// TestCheckCorpusInventoryDetectsCountIdentityAndSourceCoverageDrift proves
// that the public corpus cannot silently shrink, duplicate a decision, or
// stop representing one of the 30 source tests recorded at capture time.
func TestCheckCorpusInventoryDetectsCountIdentityAndSourceCoverageDrift(t *testing.T) {
	entries := corpusEntries(t)
	cases := allCorpusCases(t)
	if err := checkCorpusInventory(entries, cases); err != nil {
		t.Fatalf("unmodified corpus inventory is invalid: %v", err)
	}

	t.Run("missing case", func(t *testing.T) {
		err := checkCorpusInventory(entries[:len(entries)-1], append([]corpusCase(nil), cases[:len(cases)-1]...))
		if err == nil || !strings.Contains(err.Error(), "want exactly 53") {
			t.Fatalf("missing case error = %v, want exact-count failure", err)
		}
	})

	t.Run("duplicate identity", func(t *testing.T) {
		mutated := append([]corpusCase(nil), cases...)
		mutated[len(mutated)-1] = mutated[0]
		err := checkCorpusInventory(entries, mutated)
		if err == nil || !strings.Contains(err.Error(), "duplicate identity") {
			t.Fatalf("duplicate identity error = %v, want duplicate-identity failure", err)
		}
	})

	t.Run("missing source test", func(t *testing.T) {
		mutated := append([]corpusCase(nil), cases...)
		removedSource := mutated[len(mutated)-1].Test
		replacementSource := mutated[0].Test
		for index := range mutated {
			if mutated[index].Test == removedSource {
				mutated[index].Test = replacementSource
				mutated[index].Case = 10000 + index
			}
		}
		err := checkCorpusInventory(entries, mutated)
		if err == nil || !strings.Contains(err.Error(), "source tests") {
			t.Fatalf("missing source-test error = %v, want source-coverage failure", err)
		}
	})

	t.Run("substituted source identity", func(t *testing.T) {
		counts := make(map[string]int, expectedCorpusSourceTestCount)
		for _, c := range cases {
			counts[c.Test]++
		}
		replaceIndex := -1
		for index, c := range cases {
			if counts[c.Test] == 1 {
				replaceIndex = index
				break
			}
		}
		if replaceIndex < 0 {
			t.Fatal("corpus has no single-case source test to substitute while preserving the 30-source aggregate")
		}
		mutatedCases := append([]corpusCase(nil), cases...)
		mutatedEntries := append([]string(nil), entries...)
		mutatedCases[replaceIndex].Test = "TestSubstitutedSource"
		mutatedCases[replaceIndex].Case = 0
		mutatedEntries[replaceIndex] = filepath.Join(corpusDir, "TestSubstitutedSource.0.json")
		err := checkCorpusInventory(mutatedEntries, mutatedCases)
		if err == nil || !strings.Contains(err.Error(), "reviewed recording") {
			t.Fatalf("substituted source error = %v, want exact reviewed-identity failure", err)
		}
	})
}

// TestCheckCorpusMetadataDetectsAnIncorrectRecordedScriptVersion proves one
// case cannot claim a different Chess Raiders Go module release.
func TestCheckCorpusMetadataDetectsAnIncorrectRecordedScriptVersion(t *testing.T) {
	path, c := sampleCase(t)
	modulePath := thisModulePath(t)

	if err := checkCorpusMetadata(path, c, modulePath); err != nil {
		t.Fatalf("sample case %s does not pass metadata checks before perturbation (fix the sample, not this test): %v", path, err)
	}

	c.Script.Version = "v99.99.99"
	if c.Script.Version == recordedCorpusScriptVersion {
		t.Fatal("perturbation produced the same version as the original — pick a different fixed value")
	}
	err := checkCorpusMetadata(path, c, modulePath)
	if err == nil {
		t.Fatal("checkCorpusMetadata accepted a script.version other than the exact recorded Chess Raiders Go module release")
	}
	if !strings.Contains(err.Error(), "script.version") {
		t.Errorf("checkCorpusMetadata's error does not name the mismatching field (\"script.version\"): %v", err)
	}
	if !strings.Contains(err.Error(), c.Test) {
		t.Errorf("checkCorpusMetadata's error does not name the case (%q): %v", c.Test, err)
	}
	t.Logf("confirmed detection: %v", err)
}

func TestCheckCorpusMetadataRejectsAWholeCorpusRetag(t *testing.T) {
	entries := corpusEntries(t)
	cases := allCorpusCases(t)
	modulePath := thisModulePath(t)
	for index := range cases {
		cases[index].Script.Version = "v0.0.3"
	}
	for index, c := range cases {
		if err := checkCorpusMetadata(entries[index], c, modulePath); err == nil {
			t.Fatalf("case %s.%d accepted after every case was retagged away from Chess Raiders Go module %s", c.Test, c.Case, recordedCorpusScriptVersion)
		}
	}
}
