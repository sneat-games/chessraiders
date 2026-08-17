// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

// This file is plan task-9 (spec/plans/publish-the-standard-bot.md, repository
// sneat-games/chessraiders): "Add the recorded testdata/ and a Go replayer
// under go/bots/standardbot, running every case through go/bots/runtime
// against the published script and reporting which decisions differ."
//
// testdata/corpus holds the recorded cases themselves — copied byte-for-byte
// from server-go/tests4bot/testdata/corpus (plan task-8, sneat-co/chessraiders,
// private) — and testdata/corpus/README.md's own Format table is this file's
// spec for every field below. This file is the replayer: it reads those exact
// bytes, feeds {observation, memory, parameters, randomDraw, options} into
// THIS checkout's own compiled Script via go/bots/runtime, and compares the
// returned intent against what was recorded. Nothing here imports, or could
// import, anything outside the standard library and this module —
// dependencies_test.go enforces that over the whole module, corpus replay
// included — so this proves the isolation the plan's own task-9 text asks
// for ("it must pass in a checkout with no access to sneat-co/chessraiders
// and no credentials for any private module") rather than merely asserting
// it: there is no import path by which this test COULD reach either.
const (
	corpusDir                     = "testdata/corpus"
	corpusFormat                  = "chess-bot-conformance-case/v1"
	expectedCorpusCaseCount       = 53
	expectedCorpusSourceTestCount = 30
	// recordedCorpusScriptVersion is the exact Chess Raiders Go module release
	// whose decisions this immutable corpus records.
	recordedCorpusScriptVersion = "v0.0.4"
)

// expectedCorpusFilenames is the reviewed identity allowlist. Each filename is
// also checked against its file's own exact (test, case) fields, so replacing a
// source test while preserving the aggregate 53/30 counts cannot silently
// redefine what the corpus covers.
const expectedCorpusFilenames = `
TestAdvanceRewardsThePawnDoubleStepAndThePromotionBonus.0.json
TestAdvanceRewardsThePawnDoubleStepAndThePromotionBonus.1.json
TestBotClearsAFriendlyBlockerToDeliverTheCapturedKing.0.json
TestBotClearsAFriendlyBlockerToDeliverTheCapturedKing.1.json
TestBotClearsAFriendlyBlockerToDeliverTheCapturedKing.2.json
TestBotDeliversOnceTheBlockingPieceHasVacated.0.json
TestBotDoesNotFabricateEnPassantOutsideWindow.0.json
TestBotPassesWhileItsOwnPieceCharges.0.json
TestBotPlaysEnPassantCapture.0.json
TestCandidateSpreadTruncatesAfterScoringNotBeforeIt.0.json
TestCommanderAdvancedTrainsLieutenantDoesNot.0.json
TestCommanderAdvancedTrainsLieutenantDoesNot.1.json
TestCommanderDismantlesEnemyWallLieutenantLeavesItAlone.0.json
TestCommanderDismantlesEnemyWallLieutenantLeavesItAlone.1.json
TestCommanderDoesNotForgeWithAnActiveBeacon.0.json
TestCommanderEscortsCaptivesHome.0.json
TestCommanderForgesALostBeaconWhenForgeEnabled.0.json
TestCommanderNeverForgesWhenTheFlagIsOff.0.json
TestCommanderPrefersTheSergeantSupportedWallWork.0.json
TestCommanderTrainsAnEngineerButNeverBuildsAWall.0.json
TestCommanderTrainsAnEngineerButNeverBuildsAWall.1.json
TestCommanderTrainsAnEngineerButNeverBuildsAWall.2.json
TestCommanderTrainsAnEngineerButNeverBuildsAWall.3.json
TestConvoyPathCostIgnoresAnOccupiedHomeSquare.0.json
TestConvoyPathCostReachesDeliverySquareWhenConvoyReachesBackRankIsFalse.0.json
TestConvoyPathFallsBackToDriftWhenThereIsNoRouteHome.0.json
TestCustomBotHonoursItsOwnBreadth.0.json
TestCustomBotHonoursItsOwnBreadth.1.json
TestDecisionsAreDeterministic.0.json
TestDecisionsAreDeterministic.1.json
TestDecisionsAreDeterministic.2.json
TestEveryTierRoutesAPinnedConvoyAroundTheBlockedLine.0.json
TestEveryTierRoutesAPinnedConvoyAroundTheBlockedLine.1.json
TestEveryTierRoutesAPinnedConvoyAroundTheBlockedLine.2.json
TestEveryTierWalksACapturedKingHome.0.json
TestEveryTierWalksACapturedKingHome.1.json
TestEveryTierWalksACapturedKingHome.2.json
TestKingSafetyEscapeRewardsMovingAThreatenedKingToSafety.0.json
TestMoralePushRewardsAdvancingTheKingWhileSafe.0.json
TestPawnPromotionSetsTheQueenChoiceOnTheIntent.0.json
TestTargetLockRewardsDodgingALockedPieceAndTheExtraSafeBonus.0.json
TestTargetLockRewardsDodgingALockedPieceAndTheExtraSafeBonus.1.json
TestTargetLockRewardsDodgingALockedPieceAndTheExtraSafeBonus.2.json
TestTempoDiscountPenalisesASlowerPieceEnoughToChangeWhichUnitMoves.0.json
TestTiersDifferOnPrisonerLogistics.0.json
TestTiersDifferOnPrisonerLogistics.1.json
TestTiersDifferOnPrisonerLogistics.2.json
TestTiersDifferOnTheSameQuietPosition.0.json
TestTiersDifferOnTheSameQuietPosition.1.json
TestTiersDifferOnTheSameQuietPosition.2.json
TestTiersStayWithinTheirBudgetInNormalPlay.0.json
TestTiersStayWithinTheirBudgetInNormalPlay.1.json
TestTiersStayWithinTheirBudgetInNormalPlay.2.json
`

// scriptRef mirrors a corpus case's own "script" field —
// testdata/corpus/README.md's Format table: "{module, version} — the exact
// published Go module and version ... this case's intent was decided
// against."
type scriptRef struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

// corpusCase mirrors one case file's own top-level shape, field for field,
// against testdata/corpus/README.md's own Format table — nothing renamed,
// nothing reshaped. Parameters, Observation, Memory and Intent stay raw JSON
// deliberately: they cross into decide() (Parameters/Observation/Memory) or
// get compared against decide()'s own raw JSON output (Intent) exactly as
// recorded, never round-tripped through a Go struct of this package's own
// invention that could silently drop a field chess-raiders-bot.star gains later — the
// same reasoning script.go's own doc comment gives for shipping no Params
// struct.
type corpusCase struct {
	Format        string          `json:"format"`
	ParamsVersion string          `json:"paramsVersion"`
	Script        scriptRef       `json:"script"`
	Test          string          `json:"test"`
	Case          int             `json:"case"`
	Difficulty    string          `json:"difficulty"`
	Parameters    json.RawMessage `json:"parameters"`
	Observation   json.RawMessage `json:"observation"`
	Memory        json.RawMessage `json:"memory"`
	RandomDraw    string          `json:"randomDraw"`
	Options       int             `json:"options"`
	Intent        json.RawMessage `json:"intent"`
}

// TestCorpusReplayAgreesWithThePublishedScript is the replayer itself: every
// case under testdata/corpus, run through THIS checkout's own
// runtime.Compile(standardbot.Script), reported against its own recorded
// intent.
//
// An empty or missing corpus is a HARD FAILURE here, not a skip and not a
// silent pass on zero cases — testdata/corpus/README.md documents 53
// recorded decisions; a corpus directory that glob returns nothing from is a
// regression (deleted or emptied testdata), and a test that quietly reports
// "0/0 agree" on that is the exact vacuous-pass shape this task was warned
// against. Likewise a file that fails to read or parse FAILS the run
// (t.Fatalf) rather than being counted as "not replayable" and skipped —
// this package makes no claim that any case is unreplayable; testdata/corpus/
// README.md's own "What replayable excludes" section is a filter server-go/
// tests4bot already applied before recording, not a filter this test applies
// again.
func TestCorpusReplayAgreesWithThePublishedScript(t *testing.T) {
	entries := corpusEntries(t)
	if len(entries) == 0 {
		t.Fatalf("no corpus case files found under %s — testdata/corpus/README.md documents 53 recorded "+
			"decisions; an empty corpus is a regression in what shipped, not a reason for this test to pass",
			corpusDir)
	}
	cases := make([]corpusCase, len(entries))
	for index, path := range entries {
		cases[index] = readCorpusCase(t, path)
	}
	if err := checkCorpusInventory(entries, cases); err != nil {
		t.Fatal(err)
	}

	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatalf("runtime.Compile(standardbot.Script) = %v, want a compiled program", err)
	}
	modulePath := thisModulePath(t)

	agree := 0
	for index, path := range entries {
		c := cases[index]
		if err := checkCorpusMetadata(path, c, modulePath); err != nil {
			t.Error(err)
			continue
		}
		if err := replayCase(program, path, c); err != nil {
			t.Error(err)
			continue
		}
		agree++
	}

	if agree != len(entries) {
		t.Errorf("%d of %d recorded decisions agree with a replay against %s@%s — see the errors above for which "+
			"case(s) differ and on what field", agree, len(entries), modulePath, recordedCorpusScriptVersion)
		return
	}
	t.Logf("%d/%d recorded decisions agree with a replay against %s@%s", agree, len(entries), modulePath, recordedCorpusScriptVersion)
}

func TestGoStandardBotReplayAgreesWithThePublishedCorpus(t *testing.T) {
	entries := corpusEntries(t)
	if len(entries) == 0 {
		t.Fatalf("no corpus case files found under %s", corpusDir)
	}
	cases := make([]corpusCase, len(entries))
	for index, path := range entries {
		cases[index] = readCorpusCase(t, path)
	}
	if err := checkCorpusInventory(entries, cases); err != nil {
		t.Fatal(err)
	}
	modulePath := thisModulePath(t)

	agree := 0
	for index, path := range entries {
		c := cases[index]
		if err := checkCorpusMetadata(path, c, modulePath); err != nil {
			t.Error(err)
			continue
		}
		if err := replayCaseGo(path, c); err != nil {
			t.Error(err)
			continue
		}
		agree++
	}

	if agree != len(entries) {
		t.Errorf("%d of %d recorded decisions agree with Go standard bot replay against %s@%s — see errors above",
			agree, len(entries), modulePath, recordedCorpusScriptVersion)
		return
	}
	t.Logf("%d/%d recorded decisions agree with Go standard bot replay", agree, len(entries))
}

func checkCorpusInventory(entries []string, cases []corpusCase) error {
	if len(entries) != len(cases) {
		return fmt.Errorf("corpus has %d filenames for %d decoded cases", len(entries), len(cases))
	}
	type identity struct {
		test       string
		caseNumber int
	}
	seen := make(map[identity]struct{}, len(cases))
	sourceTests := make(map[string]struct{}, expectedCorpusSourceTestCount)
	for _, corpusCase := range cases {
		key := identity{test: corpusCase.Test, caseNumber: corpusCase.Case}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("corpus contains duplicate identity (%s, %d)", corpusCase.Test, corpusCase.Case)
		}
		seen[key] = struct{}{}
		sourceTests[corpusCase.Test] = struct{}{}
	}
	if len(cases) != expectedCorpusCaseCount {
		return fmt.Errorf("corpus contains %d cases, want exactly %d", len(cases), expectedCorpusCaseCount)
	}
	if len(sourceTests) != expectedCorpusSourceTestCount {
		return fmt.Errorf("corpus names %d source tests, want exactly %d as documented at capture", len(sourceTests), expectedCorpusSourceTestCount)
	}
	expectedFilenames := strings.Fields(expectedCorpusFilenames)
	if len(expectedFilenames) != expectedCorpusCaseCount {
		return fmt.Errorf("reviewed corpus filename allowlist contains %d entries, want %d", len(expectedFilenames), expectedCorpusCaseCount)
	}
	actualFilenames := make([]string, len(entries))
	for index, path := range entries {
		actual := filepath.Base(path)
		derived := fmt.Sprintf("%s.%d.json", cases[index].Test, cases[index].Case)
		if actual != derived {
			return fmt.Errorf("corpus filename %q does not match its exact (test, case) identity %q", actual, derived)
		}
		actualFilenames[index] = actual
	}
	sort.Strings(actualFilenames)
	for index := range expectedFilenames {
		if actualFilenames[index] != expectedFilenames[index] {
			return fmt.Errorf("corpus filename identities differ from the reviewed recording: got %q at index %d, want %q", actualFilenames[index], index, expectedFilenames[index])
		}
	}
	return nil
}

// corpusEntries returns every *.json path under corpusDir, sorted, so a
// re-run's own t.Log output lists cases in a stable order. Any glob failure
// is a Fatalf: a broken glob pattern is this test's own bug, not a corpus
// finding.
func corpusEntries(t *testing.T) []string {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join(corpusDir, "*.json"))
	if err != nil {
		t.Fatalf("glob %s/*.json: %v", corpusDir, err)
	}
	sort.Strings(entries)
	return entries
}

// readCorpusCase reads and decodes one case file. Fatalf on any read or
// decode error — a malformed corpus file is not "skipped" here (that would
// silently shrink coverage the same way an empty corpus would); it stops the
// whole run so the broken file gets fixed, not quietly excluded.
func readCorpusCase(t *testing.T, path string) corpusCase {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var c corpusCase
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return c
}

// thisModulePath reads go/go.mod's own "module" directive directly, the same
// "read the durable manifest, not build-time reflection" discipline
// server-go/tests4bot's own mustStandardBotModuleVersion applies one
// repository over — see that function's doc comment for the empirical
// reason (an empty runtime/debug.ReadBuildInfo().Deps in a library package's
// own test binary, confirmed on go1.26.4): a test binary's build metadata is
// not a reliable source for "which module/version is this", here any more
// than it was there.
//
// This module never requires itself, so — unlike mustStandardBotModuleVersion,
// which reads a REQUIRE line naming a dependency's pinned version — there is
// no version line here to read; go.mod's OWN "module" directive names this
// module's own path, which is the fact checkCorpusMetadata actually needs:
// proof that a case was recorded against THIS module, not a different fork
// entirely, before ever trusting its script.version field for anything.
func thisModulePath(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("read go/go.mod for this module's own path: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1]
		}
	}
	t.Fatal("go/go.mod has no module directive")
	return ""
}

// checkCorpusMetadata validates the fields that identify WHICH script a case
// was decided against — as distinct from replayCase below, which validates
// WHAT it decided. A case whose script.module names a different module
// entirely, or whose script.version does not name the exact reviewed release,
// is wrong regardless of whether decide() happens to still agree with its
// recorded intent: this corpus's whole point is "this is what Chess Raiders Go
// module v0.0.2 decided". Resolving that claim from the corpus's own declared
// field — never by asking go.mod what version IT thinks is current, which has
// no meaning for a module that does not require itself — is exactly what plan
// task-9 asks this replayer to get right.
//
// recordedCorpusScriptVersion is intentionally hardcoded: a future re-record
// against another Chess Raiders Go module release must review and update the
// corpus, its filename identity allowlist and this release fact together.
func checkCorpusMetadata(path string, c corpusCase, expectedModule string) error {
	if c.Format != corpusFormat {
		return fmt.Errorf("%s (%s case %d): format %q, want %q", path, c.Test, c.Case, c.Format, corpusFormat)
	}
	if c.ParamsVersion != standardbot.ParamsVersion {
		return fmt.Errorf("%s (%s case %d): paramsVersion %q, want standardbot.ParamsVersion %q for the resolved-row contract",
			path, c.Test, c.Case, c.ParamsVersion, standardbot.ParamsVersion)
	}
	if c.Script.Module != expectedModule {
		return fmt.Errorf("%s (%s case %d): script.module %q, want %q (this module's own go.mod path) — "+
			"a case recorded against a different module proves nothing about this one",
			path, c.Test, c.Case, c.Script.Module, expectedModule)
	}
	if c.Script.Version != recordedCorpusScriptVersion {
		return fmt.Errorf("%s (%s case %d): script.version %q, want the exact recorded Chess Raiders Go module release %q",
			path, c.Test, c.Case, c.Script.Version, recordedCorpusScriptVersion)
	}
	return nil
}

// replayCase feeds case c's own {observation, memory, parameters,
// randomDraw, options} into program's decide() — exactly the call sequence
// testdata/corpus/README.md's own Format section documents, and exactly
// server-go/starlartier's own internal recipe — and compares decide()'s own
// result[0] against c's own recorded intent. Every failure names the file,
// the test/case provenance and the field: a decode error, a decide() error,
// or an intent mismatch are three DIFFERENT things a caller needs to
// distinguish, not one generic "case N failed".
func replayCaseGo(path string, c corpusCase) error {
	draw, err := strconv.ParseInt(c.RandomDraw, 10, 64)
	if err != nil {
		return fmt.Errorf("%s (%s case %d): randomDraw %q does not parse as a decimal int64: %w",
			path, c.Test, c.Case, c.RandomDraw, err)
	}
	memory := c.Memory
	if len(memory) == 0 {
		memory = json.RawMessage("{}")
	}

	intentBytes, _, _, err := standardbot.DecideJSON(c.Observation, memory, c.Parameters, draw, c.Options)
	if err != nil {
		return fmt.Errorf("%s (%s case %d): DecideJSON returned an error: %w", path, c.Test, c.Case, err)
	}

	got, err := canonicalJSON(intentBytes)
	if err != nil {
		return fmt.Errorf("%s (%s case %d): decode replayed intent %s: %w", path, c.Test, c.Case, intentBytes, err)
	}
	want, err := canonicalJSON(c.Intent)
	if err != nil {
		return fmt.Errorf("%s (%s case %d): decode recorded intent %s: %w", path, c.Test, c.Case, c.Intent, err)
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("%s (%s case %d): intent differs — replayed %s, recorded %s",
			path, c.Test, c.Case, trimJSON(intentBytes), trimJSON(c.Intent))
	}
	return nil
}

func replayCase(program *runtime.Program, path string, c corpusCase) error {
	draw, err := strconv.ParseInt(c.RandomDraw, 10, 64)
	if err != nil {
		return fmt.Errorf("%s (%s case %d): randomDraw %q does not parse as a decimal int64: %w",
			path, c.Test, c.Case, c.RandomDraw, err)
	}
	drawJSON, err := json.Marshal(draw)
	if err != nil {
		return fmt.Errorf("%s (%s case %d): encode randomDraw: %w", path, c.Test, c.Case, err)
	}
	optionsJSON, err := json.Marshal(c.Options)
	if err != nil {
		return fmt.Errorf("%s (%s case %d): encode options: %w", path, c.Test, c.Case, err)
	}
	memory := c.Memory
	if len(memory) == 0 {
		memory = json.RawMessage("{}")
	}

	result, err := program.Call("decide",
		string(c.Observation), string(memory), string(c.Parameters), string(drawJSON), string(optionsJSON))
	if err != nil {
		return fmt.Errorf("%s (%s case %d): decide() returned an error: %w", path, c.Test, c.Case, err)
	}

	var tuple [3]json.RawMessage
	if err := json.Unmarshal([]byte(result), &tuple); err != nil {
		return fmt.Errorf("%s (%s case %d): decide()'s own result %s does not decode as a 3-tuple: %w",
			path, c.Test, c.Case, result, err)
	}

	got, err := canonicalJSON(tuple[0])
	if err != nil {
		return fmt.Errorf("%s (%s case %d): decode replayed intent %s: %w", path, c.Test, c.Case, tuple[0], err)
	}
	want, err := canonicalJSON(c.Intent)
	if err != nil {
		return fmt.Errorf("%s (%s case %d): decode recorded intent %s: %w", path, c.Test, c.Case, c.Intent, err)
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("%s (%s case %d): intent differs — replayed %s, recorded %s",
			path, c.Test, c.Case, trimJSON(tuple[0]), trimJSON(c.Intent))
	}
	return nil
}

// canonicalJSON decodes raw into a plain interface{} tree (map[string]any /
// []any / string / float64 / bool / nil) rather than a hand-declared intent
// struct, so a field chess-raiders-bot.star's decide() starts returning tomorrow — one
// this package's own author never enumerated in a struct — cannot be
// silently dropped on either side of the comparison in replayCase. An empty
// RawMessage (an absent JSON field, never actually produced by json.Marshal
// but defended against here since corpusCase.Intent is a pointer-free
// json.RawMessage) is treated as JSON null, matching decide()'s own
// documented "pass" shape (testdata/corpus/README.md, "Why intent is
// sometimes null").
func canonicalJSON(raw json.RawMessage) (interface{}, error) {
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// trimJSON renders raw compactly for an error message; falls back to the raw
// bytes verbatim if they somehow do not round-trip (never expected, since
// canonicalJSON already decoded them above every call site that uses this).
func trimJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "null"
	}
	return string(raw)
}
