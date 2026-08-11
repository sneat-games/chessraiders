// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

// beaconScenario is deliberately tiny: no legal moves compete with the one
// Beacon deployment action, so it isolates the contract cutover from tactical
// scoring. Every direct-indexed script field is present just as the private
// host contract promises.
const beaconScenario = `{
	"lifecycle":"playing", "side":"white", "nowMs":0, "revision":1,
	"ownMorale":0, "ownMoralePenalty":0, "ownManaged":0,
	"own":[{"unitId":"wk","square":"e1","side":"white","rank":"king","convoy":false,"cargoCount":0,"kingCargo":false,"ghost":false,"refitting":false,"recoveryUntil":0,"charging":false,"training":false,"forging":false,"profession":"","veteran":false,"advanced":false,"advancedEligible":false}],
	"enemy":[], "legal":{}, "affordability":{}, "enPassant":{}, "danger":{},
	"routes":[], "interrogations":[], "targetLocks":[], "deliverySquares":[], "convoyHome":{}, "blockingBase":[],
	"rules":{"veteranProgression":false,"allowsKill":true,"allowsCapture":true,"pieceChargeMs":{},"baseSquares":{},"beaconEnabled":true,"beaconForgeEnabled":false,"beaconKingStartsAsBearer":false,"specialistsEnabled":false,"woodWallsEnabled":false,"stoneWallsEnabled":false},
	"systems":{"training":false,"walls":false,"beacon":false,"prisoners":false,"morale":false,"espionage":false},
	"beacon":{"lifecycle":"undeployed","bearerSquare":"","everHandedOff":false}, "walls":[], "enemyManaged":0
}`

func decodedRecruitParams(t *testing.T) map[string]any {
	t.Helper()
	var envelope struct {
		Tiers map[string]map[string]any `json:"tiers"`
	}
	if err := json.Unmarshal(standardbot.Params, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Tiers["recruit"]
}

func decideJSON(t *testing.T, observation map[string]any, params map[string]any) (string, error) {
	t.Helper()
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	row, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return program.Call("decide", string(obs), `{}`, string(row), `0`)
}

func decideWithMemoryJSON(t *testing.T, observation map[string]any, memory json.RawMessage, params map[string]any) string {
	t.Helper()
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatal(err)
	}
	obs, _ := json.Marshal(observation)
	row, _ := json.Marshal(params)
	got, err := program.Call("decide", string(obs), string(memory), string(row), `0`, `1`)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func decodeBeaconScenario(t *testing.T) map[string]any {
	t.Helper()
	var observation map[string]any
	if err := json.Unmarshal([]byte(beaconScenario), &observation); err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestCurrentBeaconAggressionIsPermissionNotBotSystemsGate(t *testing.T) {
	observation := decodeBeaconScenario(t)
	got, err := decideJSON(t, observation, decodedRecruitParams(t))
	if err != nil {
		t.Fatalf("current observation decide() = %v", err)
	}
	if !strings.Contains(got, `"action":"beacon_deploy"`) {
		t.Fatalf("current Beacon aggression should deploy despite systems.beacon=false, got %s", got)
	}
}

func TestBeaconAggressionAndMatchRulesAreIndependentGates(t *testing.T) {
	t.Run("zero aggression", func(t *testing.T) {
		observation := decodeBeaconScenario(t)
		params := decodedRecruitParams(t)
		params["beaconAggression"] = 0.0
		got, err := decideJSON(t, observation, params)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, `[null,`) {
			t.Fatalf("zero beaconAggression proposed an action: %s", got)
		}
	})
	t.Run("match Beacon disabled", func(t *testing.T) {
		observation := decodeBeaconScenario(t)
		observation["rules"].(map[string]any)["beaconEnabled"] = false
		got, err := decideJSON(t, observation, decodedRecruitParams(t))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, `[null,`) {
			t.Fatalf("disabled match Beacon proposed an action: %s", got)
		}
	})
	t.Run("forge rule gate", func(t *testing.T) {
		observation := decodeBeaconScenario(t)
		observation["beacon"] = map[string]any{"lifecycle": "lost", "bearerSquare": "", "everHandedOff": false}
		observation["own"] = []any{map[string]any{
			"unitId": "wp", "square": "a2", "side": "white", "rank": "pawn", "convoy": false, "cargoCount": 0, "kingCargo": false, "ghost": false, "refitting": false, "recoveryUntil": 0, "charging": false, "training": false, "forging": false, "profession": "", "veteran": false, "advanced": false, "advancedEligible": false,
		}}
		observation["rules"].(map[string]any)["baseSquares"] = map[string]any{"pawn": []any{"a2"}}
		got, err := decideJSON(t, observation, decodedRecruitParams(t))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, `[null,`) {
			t.Fatalf("forge with beaconForgeEnabled=false proposed an action: %s", got)
		}
		observation["rules"].(map[string]any)["beaconForgeEnabled"] = true
		got, err = decideJSON(t, observation, decodedRecruitParams(t))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, `"action":"beacon_forge"`) {
			t.Fatalf("forge with beaconForgeEnabled=true was not proposed: %s", got)
		}
	})
}

func TestLegacyBeaconFallbackRetainsSystemsGate(t *testing.T) {
	observation := decodeBeaconScenario(t)
	params := decodedRecruitParams(t)
	delete(params, "beaconAggression")
	params["system"] = 1.0
	got, err := decideJSON(t, observation, params)
	if err != nil {
		t.Fatalf("legacy observation decide() = %v", err)
	}
	if !strings.HasPrefix(got, `[null,`) {
		t.Fatalf("legacy systems.beacon=false must still suppress Beacon actions, got %s", got)
	}
	observation["systems"].(map[string]any)["beacon"] = true
	got, err = decideJSON(t, observation, params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"action":"beacon_deploy"`) {
		t.Fatalf("legacy systems.beacon=true and system>0 must still deploy Beacon: %s", got)
	}
}

func TestParamsTableCarriesFounderAggressionValues(t *testing.T) {
	var envelope struct {
		Version string                    `json:"version"`
		Tiers   map[string]map[string]any `json:"tiers"`
	}
	if err := json.Unmarshal(standardbot.Params, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Version != "chess-bot-tier-params/v1" {
		t.Fatalf("params envelope = %q, want additive chess-bot-tier-params/v1", envelope.Version)
	}
	for tier, want := range map[string]float64{"recruit": 0.3, "lieutenant": 0.6, "commander": 0.9, "adviser": 1.0} {
		if got, ok := envelope.Tiers[tier]["beaconAggression"].(float64); !ok || got != want {
			t.Errorf("%s beaconAggression = %v, want %v", tier, envelope.Tiers[tier]["beaconAggression"], want)
		}
	}
	for tier, want := range map[string]float64{"recruit": 0.3, "lieutenant": 0.6, "commander": 0.9, "adviser": 0.5} {
		if got, ok := envelope.Tiers[tier]["moralePush"].(float64); !ok || got != want {
			t.Errorf("%s moralePush = %v, want %v", tier, envelope.Tiers[tier]["moralePush"], want)
		}
	}
}

func TestExplanationDetailsDeclareEveryNewStrategyTerm(t *testing.T) {
	for _, needle := range []string{
		`"develop": ["firstForward"]`, `"coverage": ["patrol"]`,
		`"kingHunt": ["visible"]`, `"repeatPenalty": ["quiet"]`,
		`"beaconAggression": ["handOff", "deploy", "restore", "forge", "coveredAdvance", "regroup"]`,
	} {
		if !strings.Contains(standardbot.Script, needle) {
			t.Errorf("TERM_DETAILS does not declare %s", needle)
		}
	}
}

func TestLeaderGuardMemoryRoundTripsAsBoundedInt64Map(t *testing.T) {
	observation := decodeBeaconScenario(t)
	observation["rules"].(map[string]any)["beaconEnabled"] = false
	observation["beacon"] = map[string]any{"lifecycle": "", "bearerSquare": "", "everHandedOff": false}
	observation["legal"] = map[string]any{"wk": []any{"e2"}}
	observation["moveFacts"] = map[string]any{"wk": map[string]any{"e2": map[string]any{
		"destinationKnown": true, "patrolGain": 0, "protectedAfter": 1,
		"threatenedAfter": 0, "cheapestThreatAfter": 0,
	}}}
	got, err := decideJSON(t, observation, decodedRecruitParams(t))
	if err != nil {
		t.Fatalf("leader quiet move = %v", err)
	}
	var tuple []json.RawMessage
	if err := json.Unmarshal([]byte(got), &tuple); err != nil {
		t.Fatal(err)
	}
	if len(tuple) != 3 {
		t.Fatalf("decision tuple = %s", got)
	}
	var memory map[string]int64
	if err := json.Unmarshal(tuple[1], &memory); err != nil {
		t.Fatalf("persistent memory does not round-trip through map[string]int64: %v (%s)", err, tuple[1])
	}
	if memory["leaderGuardActive"] != 1 {
		t.Fatalf("leader guard active = %d, want 1 (%v)", memory["leaderGuardActive"], memory)
	}
	if len(memory) > 32 {
		t.Fatalf("leader guard memory has %d entries, Recruit allows at most 32", len(memory))
	}
}

func TestCurrentStrategyFitsRecruitStepBudgetOnRepresentativeBoard(t *testing.T) {
	observation := strategyObservation(t,
		strategyCell("wk", "e2", "white", "king"), strategyCell("wn", "c2", "white", "knight"),
		strategyCell("wb", "f1", "white", "bishop"), strategyCell("wr", "a1", "white", "rook"),
		strategyCell("wp", "d2", "white", "pawn"),
	)
	observation["enemy"] = []any{strategyCell("bk", "h8", "black", "king"), strategyCell("bp", "d4", "black", "pawn")}
	observation["legal"] = map[string]any{
		"wk": []any{"e3", "d3"}, "wn": []any{"d4", "e4", "a3"}, "wb": []any{"g2", "h3"}, "wr": []any{"a2", "a3"}, "wp": []any{"d3", "d4"},
	}
	observation["moveFacts"] = map[string]any{
		"wk": map[string]any{"e3": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}, "d3": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
		"wn": map[string]any{"d4": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 2}, "e4": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 1}, "a3": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
		"wb": map[string]any{"g2": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 1}, "h3": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 1}},
		"wr": map[string]any{"a2": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 1}, "a3": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 1}},
		"wp": map[string]any{"d3": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}, "d4": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
	}
	// A full side still has legal/current facts for every active piece. This
	// forces unit_priority to inspect sixteen candidates before Recruit narrows
	// to breadth and each retained unit to exercise candidate spread.
	for _, extra := range []struct{ id, from, to, rank string }{
		{"pa", "a2", "a3", "pawn"}, {"pb", "b2", "b3", "pawn"},
		{"pc", "c3", "c4", "pawn"}, {"pe", "e4", "e5", "pawn"},
		{"pf", "f2", "f3", "pawn"}, {"pg", "g2", "g3", "pawn"},
		{"ph", "h2", "h3", "pawn"}, {"wq", "d1", "d3", "queen"},
		{"rb", "b1", "b3", "rook"}, {"ng", "g1", "f3", "knight"},
		{"bc", "c1", "b2", "bishop"},
	} {
		observation["own"] = append(observation["own"].([]any), strategyCell(extra.id, extra.from, "white", extra.rank))
		observation["legal"].(map[string]any)[extra.id] = []any{extra.to}
		observation["moveFacts"].(map[string]any)[extra.id] = map[string]any{extra.to: map[string]any{
			"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0,
			"cheapestThreatAfter": 0, "patrolGain": 1,
		}}
	}
	if got := len(observation["own"].([]any)); got != 16 {
		t.Fatalf("representative side has %d pieces, want 16", got)
	}
	outcomes := map[string]any{
		"kill":    map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
		"capture": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
	}
	observation["affordability"] = map[string]any{
		"wn": map[string]any{"d4": outcomes},
		"wp": map[string]any{"d4": outcomes},
	}
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatal(err)
	}
	obs, _ := json.Marshal(observation)
	params, _ := json.Marshal(decodedRecruitParams(t))
	if _, steps, err := program.CallWithStepLimit(250_000, "decide", string(obs), `{}`, string(params), `0`); err != nil {
		t.Fatalf("current strategy exceeded Recruit's shipped 250000-step budget after %d: %v", steps, err)
	} else if steps == 0 {
		t.Fatal("current strategy reported zero interpreter steps")
	} else {
		t.Logf("current strategy representative board used %d interpreter steps", steps)
	}
}

func TestKingQuietReverseGuardAndThreatEscape(t *testing.T) {
	first := decodeBeaconScenario(t)
	first["rules"].(map[string]any)["beaconEnabled"] = false
	first["legal"] = map[string]any{"wk": []any{"e2"}}
	first["moveFacts"] = map[string]any{"wk": map[string]any{"e2": map[string]any{"destinationKnown": true, "patrolGain": 0, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0}}}
	initial, err := decideJSON(t, first, decodedRecruitParams(t))
	if err != nil {
		t.Fatal(err)
	}
	var start []json.RawMessage
	if err := json.Unmarshal([]byte(initial), &start); err != nil {
		t.Fatal(err)
	}
	second := decodeBeaconScenario(t)
	second["rules"].(map[string]any)["beaconEnabled"] = false
	second["own"].([]any)[0].(map[string]any)["square"] = "e2"
	second["legal"] = map[string]any{"wk": []any{"e1"}}
	second["moveFacts"] = map[string]any{"wk": map[string]any{"e1": map[string]any{"destinationKnown": true, "patrolGain": 0, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0}}}
	got := decideWithMemoryJSON(t, second, start[1], decodedRecruitParams(t))
	if !strings.HasPrefix(got, `[null,`) {
		t.Fatalf("quiet immediate king reverse was not blocked: %s", got)
	}
	second["danger"] = map[string]any{"e2": map[string]any{"threat": 1, "cheapest": 1, "guarded": 0}}
	got = decideWithMemoryJSON(t, second, start[1], decodedRecruitParams(t))
	if !strings.Contains(got, `"from":"e2"`) || !strings.Contains(got, `"to":"e1"`) {
		t.Fatalf("threat escape reverse was blocked: %s", got)
	}
}

// strategyCell deliberately builds the full public Cell contract.  Keeping
// these scenarios on decide(), rather than calling a private scorer helper,
// means a test protects the same path that picks a bot move in a match.
func strategyCell(id, square, side, rank string) map[string]any {
	return map[string]any{
		"unitId": id, "square": square, "side": side, "rank": rank,
		"convoy": false, "cargoCount": 0, "kingCargo": false, "ghost": false,
		"refitting": false, "recoveryUntil": 0, "charging": false,
		"training": false, "forging": false, "profession": "", "veteran": false,
		"advanced": false, "advancedEligible": false, "moved": false,
	}
}

func strategyObservation(t *testing.T, own ...map[string]any) map[string]any {
	t.Helper()
	observation := decodeBeaconScenario(t)
	pieces := make([]any, 0, len(own))
	for _, cell := range own {
		pieces = append(pieces, cell)
	}
	observation["own"] = pieces
	observation["rules"].(map[string]any)["beaconEnabled"] = false
	observation["beacon"] = map[string]any{"lifecycle": "", "bearerSquare": "", "everHandedOff": false}
	observation["moveFacts"] = map[string]any{}
	return observation
}

func facts(unit, destination string, known bool, protected, threatened, patrol int) map[string]any {
	return map[string]any{unit: map[string]any{destination: map[string]any{
		"destinationKnown": known, "protectedAfter": protected, "threatenedAfter": threatened,
		"cheapestThreatAfter": threatened, "patrolGain": patrol,
	}}}
}

func decision(t *testing.T, observation map[string]any, memory map[string]int64) (map[string]any, map[string]int64, []map[string]any) {
	t.Helper()
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatal(err)
	}
	obs, _ := json.Marshal(observation)
	if memory == nil {
		memory = map[string]int64{}
	}
	mem, _ := json.Marshal(memory)
	params, _ := json.Marshal(decodedRecruitParams(t))
	raw, err := program.Call("decide", string(obs), string(mem), string(params), `0`, `8`)
	if err != nil {
		t.Fatalf("decide() = %v", err)
	}
	var tuple []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &tuple); err != nil || len(tuple) != 3 {
		t.Fatalf("decision tuple %s: %v", raw, err)
	}
	var intent map[string]any
	_ = json.Unmarshal(tuple[0], &intent)
	var next map[string]int64
	if err := json.Unmarshal(tuple[1], &next); err != nil {
		t.Fatalf("memory %s: %v", tuple[1], err)
	}
	var options []map[string]any
	if err := json.Unmarshal(tuple[2], &options); err != nil {
		t.Fatalf("options %s: %v", tuple[2], err)
	}
	return intent, next, options
}

func optionFor(options []map[string]any, from, to string) map[string]any {
	for _, option := range options {
		intent, _ := option["intent"].(map[string]any)
		if intent["from"] == from && intent["to"] == to {
			return option
		}
	}
	return nil
}

func requireOption(t *testing.T, options []map[string]any, from, to string) map[string]any {
	t.Helper()
	option := optionFor(options, from, to)
	if option == nil {
		t.Fatalf("expected proposal %s→%s is absent: %#v", from, to, options)
	}
	return option
}

func hasTerm(option map[string]any, term, detail string) bool {
	if option == nil {
		return false
	}
	terms, _ := option["terms"].([]any)
	for _, raw := range terms {
		entry, _ := raw.(map[string]any)
		if entry["term"] == term && entry["detail"] == detail {
			return true
		}
	}
	return false
}

func termValue(option map[string]any, term, detail string) (float64, bool) {
	if option == nil {
		return 0, false
	}
	terms, _ := option["terms"].([]any)
	for _, raw := range terms {
		entry, _ := raw.(map[string]any)
		if entry["term"] == term && entry["detail"] == detail {
			value, ok := entry["value"].(float64)
			return value, ok
		}
	}
	return 0, false
}

func TestLeaderReverseGuardBothDirectionsAndPlacementRelease(t *testing.T) {
	for _, tc := range []struct {
		name, rank, firstFrom, firstTo string
		beacon                         bool
	}{
		{"king advance", "king", "e1", "e2", false},
		{"king retreat", "king", "e3", "e2", false},
		{"Beacon advance", "knight", "e2", "e3", true},
		{"Beacon retreat", "knight", "e4", "e3", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			leader := strategyCell("leader", tc.firstFrom, "white", tc.rank)
			first := strategyObservation(t, leader)
			if tc.beacon {
				first["rules"].(map[string]any)["beaconEnabled"] = true
				first["beacon"] = map[string]any{"lifecycle": "deployed", "bearerSquare": tc.firstFrom, "everHandedOff": true}
			}
			first["legal"] = map[string]any{"leader": []any{tc.firstTo}}
			first["moveFacts"] = facts("leader", tc.firstTo, true, 1, 0, 0)
			_, memory, _ := decision(t, first, nil)
			if memory["leaderGuardActive"] != 1 {
				t.Fatalf("quiet leader move did not arm: %#v", memory)
			}

			reverse := strategyObservation(t, strategyCell("leader", tc.firstTo, "white", tc.rank))
			if tc.beacon {
				reverse["rules"].(map[string]any)["beaconEnabled"] = true
				reverse["beacon"] = map[string]any{"lifecycle": "deployed", "bearerSquare": tc.firstTo, "everHandedOff": true}
			}
			reverse["legal"] = map[string]any{"leader": []any{tc.firstFrom}}
			reverse["moveFacts"] = facts("leader", tc.firstFrom, true, 1, 0, 0)
			intent, _, _ := decision(t, reverse, memory)
			if intent != nil {
				t.Fatalf("immediate quiet reverse escaped guard: %#v", intent)
			}

			// Any actual placement change releases the one-ply guard.
			reverse["enemy"] = []any{strategyCell("changed", "a8", "black", "pawn")}
			intent, _, _ = decision(t, reverse, memory)
			if intent == nil || intent["to"] != tc.firstFrom {
				t.Fatalf("placement change did not release guard: %#v", intent)
			}
		})
	}
}

func TestLeaderGuardThreatAndChargingExceptions(t *testing.T) {
	first := strategyObservation(t, strategyCell("wk", "e1", "white", "king"))
	first["legal"] = map[string]any{"wk": []any{"e2"}}
	first["moveFacts"] = facts("wk", "e2", true, 1, 0, 0)
	_, memory, _ := decision(t, first, nil)

	// While the command is charging, the physical board still contains the
	// pre-move placement.  It must retain rather than clear the guard.
	charging := strategyObservation(t, strategyCell("wk", "e1", "white", "king"))
	charging["own"].([]any)[0].(map[string]any)["charging"] = true
	charging["legal"] = map[string]any{"wk": []any{"e2"}}
	charging["moveFacts"] = facts("wk", "e2", true, 1, 0, 0)
	_, stillArmed, _ := decision(t, charging, memory)
	if stillArmed["leaderGuardActive"] != 1 {
		t.Fatalf("charging pre-layout cleared guard: %#v", stillArmed)
	}

	reverse := strategyObservation(t, strategyCell("wk", "e2", "white", "king"))
	reverse["legal"] = map[string]any{"wk": []any{"e1"}}
	reverse["moveFacts"] = facts("wk", "e1", true, 1, 0, 0)
	reverse["danger"] = map[string]any{"e2": map[string]any{"threat": 1, "guarded": 0, "cheapest": 1}}
	intent, _, _ := decision(t, reverse, memory)
	if intent == nil || intent["to"] != "e1" {
		t.Fatalf("threat escape was incorrectly held by guard: %#v", intent)
	}
}

func TestKingLeaderSupportUsesNumberDistanceDirectionAndSaturation(t *testing.T) {
	coveredAdvance := func(t *testing.T, supporters ...string) float64 {
		t.Helper()
		own := []map[string]any{strategyCell("wk", "e1", "white", "king")}
		for i, square := range supporters {
			own = append(own, strategyCell("s"+string(rune('a'+i)), square, "white", "pawn"))
		}
		o := strategyObservation(t, own...)
		o["legal"] = map[string]any{"wk": []any{"e2"}}
		o["moveFacts"] = facts("wk", "e2", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		value, _ := termValue(requireOption(t, options, "e1", "e2"), "moralePush", "coveredAdvance")
		return value
	}

	one := coveredAdvance(t, "d3")
	two := coveredAdvance(t, "d3", "f3")
	if !(two > one && math.Abs(two-2*one) < 1e-9) {
		t.Fatalf("two close supporters should double one before saturation: one=%v two=%v", one, two)
	}
	near := coveredAdvance(t, "e3")
	far := coveredAdvance(t, "e4")
	if !(near > far && math.Abs(near-2*far) < 1e-9) {
		t.Fatalf("near supporter should count twice distance-two supporter: near=%v far=%v", near, far)
	}
	ahead := coveredAdvance(t, "d3")
	abreast := coveredAdvance(t, "d2")
	behind := coveredAdvance(t, "d1")
	if !(ahead > abreast && math.Abs(ahead-2*abreast) < 1e-9 && behind == 0) {
		t.Fatalf("support direction weights wrong: ahead=%v abreast=%v behind=%v", ahead, abreast, behind)
	}
	saturated := coveredAdvance(t, "d3", "f3")
	overfilled := coveredAdvance(t, "d3", "e3", "f3")
	if math.Abs(saturated-overfilled) > 1e-9 {
		t.Fatalf("support must saturate after two full nearby supporters: saturated=%v overfilled=%v", saturated, overfilled)
	}
}

func TestBeaconBearerRewardsCoveredAdvanceAndRegroup(t *testing.T) {
	for _, tc := range []struct {
		name, from, to, supporter, detail string
	}{
		{"advance", "e2", "e3", "e4", "coveredAdvance"},
		{"regroup", "e4", "d3", "c3", "regroup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bearer := strategyCell("bearer", tc.from, "white", "knight")
			o := strategyObservation(t, bearer, strategyCell("support", tc.supporter, "white", "pawn"))
			o["rules"].(map[string]any)["beaconEnabled"] = true
			o["beacon"] = map[string]any{"lifecycle": "deployed", "bearerSquare": tc.from, "everHandedOff": true}
			o["legal"] = map[string]any{"bearer": []any{tc.to}}
			o["moveFacts"] = facts("bearer", tc.to, true, 1, 0, 0)
			intent, _, options := decision(t, o, nil)
			if intent == nil || intent["from"] != tc.from || intent["to"] != tc.to {
				t.Fatalf("Beacon %s was not selected: %#v", tc.name, intent)
			}
			value, ok := termValue(requireOption(t, options, tc.from, tc.to), "beaconAggression", tc.detail)
			if !ok || value <= 0 {
				t.Fatalf("Beacon %s lacks positive formation reward: %#v", tc.name, options)
			}
		})
	}
}

func TestOnlyQuietNonPromotionLeaderMovesArmGuard(t *testing.T) {
	cases := []struct {
		name  string
		cell  map[string]any
		enemy []any
		to    string
	}{
		{"capture", strategyCell("wk", "e2", "white", "king"), []any{strategyCell("ep", "e3", "black", "pawn")}, "e3"},
		{"promotion", strategyCell("bp", "e7", "white", "pawn"), []any{}, "e8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observation := strategyObservation(t, tc.cell)
			if tc.name == "promotion" {
				observation["rules"].(map[string]any)["beaconEnabled"] = true
				observation["beacon"] = map[string]any{"lifecycle": "deployed", "bearerSquare": "e7", "everHandedOff": true}
			}
			observation["enemy"] = tc.enemy
			observation["legal"] = map[string]any{tc.cell["unitId"].(string): []any{tc.to}}
			observation["moveFacts"] = facts(tc.cell["unitId"].(string), tc.to, true, 1, 0, 0)
			if tc.name == "capture" {
				observation["affordability"] = map[string]any{"wk": map[string]any{"e3": map[string]any{"capture": map[string]any{"affordable": true, "requiredMorale": 0}}}}
			}
			intent, memory, _ := decision(t, observation, nil)
			if intent == nil || intent["from"] != tc.cell["square"] || intent["to"] != tc.to {
				t.Fatalf("%s scenario did not select the intended move: %#v", tc.name, intent)
			}
			if memory["leaderGuardActive"] != 0 {
				t.Fatalf("%s armed a quiet-only guard: %#v", tc.name, memory)
			}
		})
	}
	convoy := strategyCell("wk", "e2", "white", "king")
	convoy["convoy"] = true
	observation := strategyObservation(t, convoy)
	observation["legal"] = map[string]any{"wk": []any{"e3"}}
	observation["moveFacts"] = facts("wk", "e3", true, 1, 0, 0)
	intent, memory, _ := decision(t, observation, nil)
	if intent == nil || intent["from"] != "e2" || intent["to"] != "e3" {
		t.Fatalf("convoy scenario did not select its move: %#v", intent)
	}
	if memory["leaderGuardActive"] != 0 {
		t.Fatalf("convoy move armed leader guard: %#v", memory)
	}
}

func TestDevelopmentAndKingHuntRequireKnownProtectedQuietMove(t *testing.T) {
	makeDevelopment := func() map[string]any {
		o := strategyObservation(t, strategyCell("wn", "c2", "white", "knight"))
		o["enemy"] = []any{strategyCell("bk", "h8", "black", "king")}
		return o
	}
	base := makeDevelopment()
	base["legal"] = map[string]any{"wn": []any{"d3"}}
	base["moveFacts"] = map[string]any{"wn": map[string]any{"d3": map[string]any{
		"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 2,
		"visibleAttacks": []any{"h8"},
	}}}
	intent, _, options := decision(t, base, nil)
	if intent == nil || intent["to"] != "d3" {
		t.Fatalf("known protected development was not selected: %#v", intent)
	}
	option := requireOption(t, options, "c2", "d3")
	if !hasTerm(option, "develop", "firstForward") || !hasTerm(option, "coverage", "patrol") || !hasTerm(option, "kingHunt", "visible") {
		t.Fatalf("known protected first development did not receive all strategic terms: %#v", option)
	}

	for _, tc := range []struct {
		name, detail string
		known        bool
		protected    int
	}{
		{"unknown", "unknownQuiet", false, 1},
		{"unprotected", "unsupportedQuiet", true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// With both destinations legal, the known protected square must beat
			// the equally-forward unknown/unprotected choice.
			o := makeDevelopment()
			o["legal"] = map[string]any{"wn": []any{"d3", "b3"}}
			o["moveFacts"] = map[string]any{"wn": map[string]any{
				"d3": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 2, "visibleAttacks": []any{"h8"}},
				"b3": map[string]any{"destinationKnown": tc.known, "protectedAfter": tc.protected, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 2, "visibleAttacks": []any{"h8"}},
			}}
			intent, _, _ := decision(t, o, nil)
			if intent == nil || intent["to"] != "d3" {
				t.Fatalf("known protected square did not beat %s choice: %#v", tc.name, intent)
			}

			// Isolate the rejected choice so its executable safety reason remains
			// observable instead of disappearing behind per-unit option dedupe.
			o["legal"] = map[string]any{"wn": []any{"b3"}}
			_, _, options := decision(t, o, nil)
			option := requireOption(t, options, "c2", "b3")
			if !hasTerm(option, "safety", tc.detail) {
				t.Fatalf("%s choice lacks its safety penalty: %#v", tc.name, option)
			}
			if hasTerm(option, "develop", "firstForward") || hasTerm(option, "coverage", "patrol") || hasTerm(option, "kingHunt", "visible") {
				t.Fatalf("%s choice received protected-development rewards: %#v", tc.name, option)
			}
		})
	}

	for _, tc := range []struct {
		name, rank string
		moved      bool
	}{
		{"moved back", "knight", true},
		{"pawn", "pawn", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cell := strategyCell("unit", "c2", "white", tc.rank)
			cell["moved"] = tc.moved
			o := strategyObservation(t, cell)
			o["legal"] = map[string]any{"unit": []any{"d3"}}
			o["moveFacts"] = facts("unit", "d3", true, 1, 0, 1)
			_, _, options := decision(t, o, nil)
			option := requireOption(t, options, "c2", "d3")
			if hasTerm(option, "develop", "firstForward") {
				t.Fatalf("%s received first-development bonus: %#v", tc.name, option)
			}
		})
	}
}

func TestVisibleAttacksSelectsExactSafeKingHuntWithoutDistanceGuessing(t *testing.T) {
	makeObservation := func(visibleAttacks []any) map[string]any {
		hunter := strategyCell("rook", "a1", "white", "rook")
		hunter["moved"] = true
		o := strategyObservation(t, hunter, strategyCell("pawn", "b2", "white", "pawn"))
		o["enemy"] = []any{strategyCell("king", "a8", "black", "king"), strategyCell("victim", "b3", "black", "pawn")}
		o["legal"] = map[string]any{"rook": []any{"a2", "a7"}, "pawn": []any{"b3"}} // non-attacking choice intentionally comes first
		o["moveFacts"] = map[string]any{"rook": map[string]any{
			"a2": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0},
			"a7": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0, "visibleAttacks": visibleAttacks},
		}}
		o["affordability"] = map[string]any{"pawn": map[string]any{"b3": map[string]any{
			"kill": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
		}}}
		return o
	}
	t.Run("rook clear line fact admits and chooses its matching destination", func(t *testing.T) {
		intent, _, options := decision(t, makeObservation([]any{"a8"}), nil)
		if intent == nil || intent["from"] != "a1" || intent["to"] != "a7" {
			t.Fatalf("host-proven rook attack was not selected: intent=%#v options=%#v", intent, options)
		}
		if !hasTerm(requireOption(t, options, "a1", "a7"), "kingHunt", "visible") {
			t.Fatalf("host-proven rook attack lacks kingHunt score: %#v", options)
		}
		reversed := makeObservation([]any{"a8"})
		reversed["legal"].(map[string]any)["rook"] = []any{"a7", "a2"}
		intent, _, _ = decision(t, reversed, nil)
		if intent == nil || intent["from"] != "a1" || intent["to"] != "a7" {
			t.Fatalf("legal ordering changed exact attack selection: %#v", intent)
		}
	})
	t.Run("empty or omitted fact flips to tactical alternative", func(t *testing.T) {
		for _, visibleAttacks := range [][]any{nil, {}} {
			intent, _, options := decision(t, makeObservation(visibleAttacks), nil)
			if intent == nil || intent["from"] != "b2" || intent["to"] != "b3" || hasTerm(optionFor(options, "a1", "a7"), "kingHunt", "visible") {
				t.Fatalf("empty visibleAttacks did not remove +30 hunt preference: intent=%#v options=%#v", intent, options)
			}
		}
	})
}

func TestVisibleAttacksRejectsUnsafeAndNonExactKingHunts(t *testing.T) {
	makeObservation := func(rank string, enemy map[string]any, fact map[string]any) map[string]any {
		o := strategyObservation(t, strategyCell("unit", "c2", "white", rank))
		o["enemy"] = []any{enemy}
		o["legal"] = map[string]any{"unit": []any{"d3"}}
		o["moveFacts"] = map[string]any{"unit": map[string]any{"d3": fact}}
		return o
	}
	baseFact := func() map[string]any {
		return map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0, "visibleAttacks": []any{"h8"}}
	}
	for _, tc := range []struct {
		name  string
		rank  string
		enemy map[string]any
		fact  map[string]any
	}{
		{"knight adjacent empty", "knight", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0, "visibleAttacks": []any{}}},
		{"blocked slider empty", "rook", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
		{"unknown", "rook", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationKnown": false, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0, "visibleAttacks": []any{"h8"}}},
		{"unprotected", "rook", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationKnown": true, "protectedAfter": 0, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0, "visibleAttacks": []any{"h8"}}},
		{"unsafe", "rook", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 1, "cheapestThreatAfter": 1, "patrolGain": 0, "visibleAttacks": []any{"h8"}}},
		{"ghost king", "rook", func() map[string]any {
			king := strategyCell("king", "h8", "black", "king")
			king["ghost"] = true
			return king
		}(), baseFact()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, options := decision(t, makeObservation(tc.rank, tc.enemy, tc.fact), nil)
			if hasTerm(requireOption(t, options, "c2", "d3"), "kingHunt", "visible") {
				t.Fatalf("%s received a visible-attack king hunt bonus: %#v", tc.name, options)
			}
		})
	}
}

func TestVisibleAttacksEntersBreadthBelowDirectKingCapture(t *testing.T) {
	t.Run("exact post-settlement attack enters Recruit breadth", func(t *testing.T) {
		units := []map[string]any{strategyCell("hunter", "a1", "white", "rook")}
		for _, square := range []string{"g7", "g8", "h7", "f7", "f8"} {
			units = append(units, strategyCell("noise"+square, square, "white", "queen"))
		}
		o := strategyObservation(t, units...)
		o["enemy"] = []any{strategyCell("king", "a8", "black", "king")}
		o["legal"] = map[string]any{"hunter": []any{"a7"}}
		o["moveFacts"] = map[string]any{"hunter": map[string]any{"a7": map[string]any{
			"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0, "visibleAttacks": []any{"a8"},
		}}}
		intent, _, _ := decision(t, o, nil)
		if intent == nil || intent["from"] != "a1" || intent["to"] != "a7" {
			t.Fatalf("exact post-move king attack stayed outside Recruit breadth: %#v", intent)
		}
	})
	t.Run("unknown unprotected or unsafe attack facts stay outside breadth", func(t *testing.T) {
		for _, tc := range []struct {
			name                     string
			known, protected, threat any
		}{
			{"unknown", false, 1, 0}, {"unprotected", true, 0, 0}, {"unsafe", true, 1, 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				units := []map[string]any{strategyCell("hunter", "a1", "white", "rook")}
				for _, square := range []string{"g7", "g8", "h7", "f7", "f8"} {
					units = append(units, strategyCell("noise"+square, square, "white", "queen"))
				}
				o := strategyObservation(t, units...)
				o["enemy"] = []any{strategyCell("king", "a8", "black", "king")}
				o["legal"] = map[string]any{"hunter": []any{"a7"}}
				o["moveFacts"] = map[string]any{"hunter": map[string]any{"a7": map[string]any{
					"destinationKnown": tc.known, "protectedAfter": tc.protected, "threatenedAfter": tc.threat,
					"cheapestThreatAfter": tc.threat, "patrolGain": 0, "visibleAttacks": []any{"a8"},
				}}}
				intent, _, options := decision(t, o, nil)
				if intent != nil || optionFor(options, "a1", "a7") != nil {
					t.Fatalf("%s exact attack incorrectly entered Recruit breadth: intent=%#v options=%#v", tc.name, intent, options)
				}
			})
		}
	})
	t.Run("affordable direct capture remains senior", func(t *testing.T) {
		o := strategyObservation(t, strategyCell("direct", "h1", "white", "rook"), strategyCell("attack", "a1", "white", "rook"))
		o["enemy"] = []any{strategyCell("king", "h8", "black", "king")}
		o["legal"] = map[string]any{"direct": []any{"h8"}, "attack": []any{"g7"}}
		o["moveFacts"] = map[string]any{"attack": map[string]any{"g7": map[string]any{
			"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0, "visibleAttacks": []any{"h8"},
		}}}
		o["affordability"] = map[string]any{"direct": map[string]any{"h8": map[string]any{"capture": map[string]any{"affordable": true, "requiredMorale": 0}}}}
		intent, _, _ := decision(t, o, nil)
		if intent == nil || intent["from"] != "h1" || intent["to"] != "h8" {
			t.Fatalf("+30 exact attack outranked affordable direct king capture: %#v", intent)
		}
	})
}

func TestKingCaptureIsPrioritizedWithinBreadth(t *testing.T) {
	// The nearest enemy pieces intentionally consume Recruit's shallow breadth.
	// The hunter is farther away, but its legal king capture must still get the
	// priority bump and become the selected real decision.
	units := []map[string]any{
		strategyCell("near1", "a2", "white", "pawn"), strategyCell("near2", "b2", "white", "pawn"),
		strategyCell("near3", "c2", "white", "pawn"), strategyCell("near4", "d2", "white", "pawn"),
		strategyCell("hunter", "h1", "white", "rook"),
	}
	observation := strategyObservation(t, units...)
	observation["enemy"] = []any{
		strategyCell("noise1", "a3", "black", "pawn"), strategyCell("noise2", "b3", "black", "pawn"),
		strategyCell("noise3", "c3", "black", "pawn"), strategyCell("noise4", "d3", "black", "pawn"),
		strategyCell("king", "h8", "black", "king"),
	}
	if len(units) <= int(decodedRecruitParams(t)["breadth"].(float64)) {
		t.Fatal("fixture must place the hunter beyond Recruit breadth before its king-capture priority bump")
	}
	observation["legal"] = map[string]any{"hunter": []any{"h8"}}
	observation["moveFacts"] = map[string]any{}
	observation["affordability"] = map[string]any{"hunter": map[string]any{"h8": map[string]any{"capture": map[string]any{"affordable": true, "requiredMorale": 0}}}}
	intent, _, options := decision(t, observation, nil)
	if intent == nil || intent["from"] != "h1" || intent["to"] != "h8" {
		t.Fatalf("legal king capture lost to breadth pruning: intent=%#v options=%#v", intent, options)
	}
	if !hasTerm(optionFor(options, "h1", "h8"), "kingHunt", "visible") {
		t.Fatalf("king capture lacks visible king priority: %#v", options)
	}
}

func TestFormationLeaderPriorityLetsRecruitBuildNeededMoraleButNotExcess(t *testing.T) {
	makeObservation := func(kingSquare string, requiredMorale int) map[string]any {
		units := []map[string]any{
			strategyCell("wk", kingSquare, "white", "king"),
			strategyCell("near1", "g7", "white", "queen"), strategyCell("near2", "g8", "white", "queen"),
			strategyCell("near3", "h7", "white", "queen"), strategyCell("near4", "f7", "white", "queen"),
		}
		o := strategyObservation(t, units...)
		o["enemy"] = []any{strategyCell("bk", "h8", "black", "king")}
		o["legal"] = map[string]any{
			"wk": []any{"e2"}, "near1": []any{"g6"}, "near2": []any{"f8"}, "near3": []any{"h6"}, "near4": []any{"f6"},
		}
		o["moveFacts"] = map[string]any{
			"wk":    map[string]any{"e2": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
			"near1": map[string]any{"g6": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
			"near2": map[string]any{"f8": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
			"near3": map[string]any{"h6": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
			"near4": map[string]any{"f6": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
		}
		o["affordability"] = map[string]any{"probe": map[string]any{"x": map[string]any{
			"capture": map[string]any{"affordable": false, "requiredMorale": requiredMorale},
		}}}
		return o
	}
	t.Run("high need still breaks Recruit breadth", func(t *testing.T) {
		o := makeObservation("e1", 5)
		intent, _, options := decision(t, o, nil)
		if intent == nil || intent["from"] != "e1" || intent["to"] != "e2" {
			t.Fatalf("safe king advance stayed outside Recruit breadth at high morale need: intent=%#v options=%#v", intent, options)
		}
	})
	t.Run("zero morale weight disables the scheduler", func(t *testing.T) {
		o := makeObservation("e1", 5)
		params := decodedRecruitParams(t)
		params["moralePush"] = 0.0
		got, err := decideJSON(t, o, params)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, `"from":"e1"`) {
			t.Fatalf("zero moralePush still scheduled king through Recruit breadth: %s", got)
		}
	})
	t.Run("one point reserve stops the scheduling boost", func(t *testing.T) {
		o := makeObservation("e7", 5)
		o["legal"].(map[string]any)["wk"] = []any{"e8"}
		o["moveFacts"].(map[string]any)["wk"] = map[string]any{"e8": map[string]any{
			"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0,
		}}
		intent, _, options := decision(t, o, nil)
		if intent != nil && intent["from"] == "e7" || optionFor(options, "e7", "e8") != nil {
			t.Fatalf("king kept its shallow-breadth privilege after reserve was achieved: intent=%#v options=%#v", intent, options)
		}
	})
	t.Run("three point excess schedules safe retreat", func(t *testing.T) {
		o := makeObservation("e6", 2)
		o["ownMorale"] = 5
		o["legal"].(map[string]any)["wk"] = []any{"e5"}
		o["moveFacts"].(map[string]any)["wk"] = map[string]any{"e5": map[string]any{
			"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0,
		}}
		intent, _, options := decision(t, o, nil)
		if intent == nil || intent["from"] != "e6" || intent["to"] != "e5" || !hasTerm(optionFor(options, "e6", "e5"), "moralePush", "excessRetreat") {
			t.Fatalf("excess morale did not bring safe retreat through Recruit breadth: intent=%#v options=%#v", intent, options)
		}
	})
}

func TestUnaffordableKingContactsDoNotConsumeRecruitBreadth(t *testing.T) {
	// At the passive-opponent stall, promoted queens could geometrically reach
	// the final king but lacked morale to take it. They must not receive the
	// +KING_VALUE scheduling bump for a move move_proposals will later reject.
	attackers := []map[string]any{
		strategyCell("wk", "e1", "white", "king"),
		strategyCell("a1", "g7", "white", "queen"), strategyCell("a2", "g8", "white", "queen"),
		strategyCell("a3", "h7", "white", "queen"), strategyCell("a4", "f7", "white", "queen"),
		strategyCell("a5", "f8", "white", "queen"),
	}
	o := strategyObservation(t, attackers...)
	o["enemy"] = []any{strategyCell("bk", "h8", "black", "king")}
	o["legal"] = map[string]any{
		"wk": []any{"e2"}, "a1": []any{"h8"}, "a2": []any{"h8"}, "a3": []any{"h8"}, "a4": []any{"h8"}, "a5": []any{"h8"},
	}
	o["moveFacts"] = map[string]any{"wk": map[string]any{"e2": map[string]any{
		"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0,
	}}}
	unaffordable := map[string]any{"capture": map[string]any{"affordable": false, "requiredMorale": 1}}
	o["affordability"] = map[string]any{
		"a1": map[string]any{"h8": unaffordable}, "a2": map[string]any{"h8": unaffordable}, "a3": map[string]any{"h8": unaffordable},
		"a4": map[string]any{"h8": unaffordable}, "a5": map[string]any{"h8": unaffordable},
	}
	if got := len(attackers) - 1; got <= int(decodedRecruitParams(t)["breadth"].(float64)) {
		t.Fatalf("fixture needs more unaffordable contacts than Recruit breadth, got %d", got)
	}
	intent, _, _ := decision(t, o, nil)
	if intent == nil || intent["from"] != "e1" || intent["to"] != "e2" {
		t.Fatalf("unaffordable king contacts crowded out safe morale advance: %#v", intent)
	}
	for _, byDestination := range o["affordability"].(map[string]any) {
		byDestination.(map[string]any)["h8"].(map[string]any)["capture"].(map[string]any)["affordable"] = true
	}
	intent, _, _ = decision(t, o, nil)
	if intent == nil || intent["from"] == "e1" || intent["to"] != "h8" {
		t.Fatalf("affordable king contact did not take priority after morale transition: %#v", intent)
	}
}

func TestFormationLeaderPriorityNeverSchedulesTacticalMoves(t *testing.T) {
	nearQueens := []map[string]any{
		strategyCell("near1", "g7", "white", "queen"), strategyCell("near2", "g8", "white", "queen"),
		strategyCell("near3", "h7", "white", "queen"), strategyCell("near4", "f7", "white", "queen"),
		strategyCell("near5", "f8", "white", "queen"),
	}
	legalAndFacts := func(o map[string]any, leaderID, leaderTo string) {
		legal := map[string]any{leaderID: []any{leaderTo}}
		moveFacts := map[string]any{leaderID: map[string]any{leaderTo: map[string]any{
			"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0,
		}}}
		for index := range nearQueens {
			id := nearQueens[index]["unitId"].(string)
			to := []string{"g6", "f8", "h6", "f6", "e8"}[index]
			legal[id] = []any{to}
			moveFacts[id] = map[string]any{to: map[string]any{
				"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0,
			}}
		}
		o["legal"], o["moveFacts"] = legal, moveFacts
	}
	t.Run("king capture", func(t *testing.T) {
		units := append([]map[string]any{strategyCell("wk", "e1", "white", "king")}, nearQueens...)
		o := strategyObservation(t, units...)
		o["enemy"] = []any{strategyCell("victim", "e8", "black", "pawn")}
		legalAndFacts(o, "wk", "e8")
		intent, _, options := decision(t, o, nil)
		if intent != nil && intent["from"] == "e1" || optionFor(options, "e1", "e8") != nil {
			t.Fatalf("king capture received formation breadth boost: intent=%#v options=%#v", intent, options)
		}
	})
	t.Run("Beacon promotion", func(t *testing.T) {
		bearer := strategyCell("bearer", "b7", "white", "pawn")
		units := append([]map[string]any{bearer}, nearQueens...)
		o := strategyObservation(t, units...)
		o["rules"].(map[string]any)["beaconEnabled"] = true
		o["beacon"] = map[string]any{"lifecycle": "deployed", "bearerSquare": "b7", "everHandedOff": true}
		legalAndFacts(o, "bearer", "b8")
		intent, _, options := decision(t, o, nil)
		if intent != nil && intent["from"] == "b7" || optionFor(options, "b7", "b8") != nil {
			t.Fatalf("Beacon promotion received formation breadth boost: intent=%#v options=%#v", intent, options)
		}
	})
}

func TestRecentVacatedSquaresBreakObservedMultiPieceCycle(t *testing.T) {
	// This is Recruit's observed loose-layout failure at tick 61: qa returning
	// b8→a8 recreates tick 55 even though qa and qb exchanged the queen role.
	// The ring records a8 as vacated two quiet moves ago, rather than treating
	// unit identity as board identity.
	qa := strategyCell("qa", "b8", "white", "queen")
	o := strategyObservation(t,
		qa, strategyCell("qb", "g7", "white", "queen"), strategyCell("qc", "g8", "white", "queen"),
		strategyCell("qd", "h8", "white", "queen"), strategyCell("qhome", "d1", "white", "queen"),
	)
	o["legal"] = map[string]any{"qa": []any{"a8"}, "qhome": []any{"d2"}}
	o["moveFacts"] = map[string]any{
		"qa":    map[string]any{"a8": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
		"qhome": map[string]any{"d2": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
	}
	memory := map[string]int64{"quietVacated0": 6, "quietVacated1": 7, "quietVacated2": 23} // a7, a8, c8
	intent, _, options := decision(t, o, memory)
	if intent == nil || intent["from"] != "d1" || intent["to"] != "d2" || optionFor(options, "b8", "a8") != nil {
		t.Fatalf("multi-piece cycle was not excluded in favour of viable queen move: intent=%#v options=%#v", intent, options)
	}
	t.Run("forced return remains legal", func(t *testing.T) {
		forced := strategyObservation(t, qa)
		forced["legal"] = map[string]any{"qa": []any{"a8"}}
		forced["moveFacts"] = facts("qa", "a8", true, 1, 0, 0)
		intent, _, options := decision(t, forced, memory)
		if intent == nil || intent["from"] != "b8" || intent["to"] != "a8" || optionFor(options, "b8", "a8") == nil {
			t.Fatalf("forced cycle return was incorrectly filtered: intent=%#v options=%#v", intent, options)
		}
	})
	t.Run("promotion is not a quiet-cycle return", func(t *testing.T) {
		pawn := strategyCell("pawn", "b7", "white", "pawn")
		promotion := strategyObservation(t, pawn, strategyCell("other", "d1", "white", "queen"))
		promotion["legal"] = map[string]any{"pawn": []any{"b8"}, "other": []any{"d2"}}
		promotion["moveFacts"] = map[string]any{
			"pawn":  map[string]any{"b8": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
			"other": map[string]any{"d2": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
		}
		promotionMemory := map[string]int64{"quietVacated0": 15} // b8
		_, _, options := decision(t, promotion, promotionMemory)
		if option := requireOption(t, options, "b7", "b8"); option["intent"].(map[string]any)["promotion"] != "queen" {
			t.Fatalf("promotion was filtered as an ordinary cycle move: %#v", option)
		}
	})
}

func TestRecentVacatedSquaresBreakMeasuredRookAndQueenCycles(t *testing.T) {
	t.Run("Lieutenant h7-g7-h7 does not close", func(t *testing.T) {
		makeState := func(rookSquare string, releaseAlternative bool) map[string]any {
			o := strategyObservation(t, strategyCell("rook", rookSquare, "white", "rook"), strategyCell("other", "a1", "white", "bishop"))
			legal := map[string]any{"rook": []any{"h7"}}
			if rookSquare == "h7" {
				legal["rook"] = []any{"g7"}
			}
			if rookSquare == "g7" {
				legal["rook"] = []any{"h7"}
			}
			if releaseAlternative {
				legal["other"] = []any{"b2"}
			}
			o["legal"] = legal
			factsByUnit := map[string]any{"rook": map[string]any{legal["rook"].([]any)[0].(string): map[string]any{
				"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0,
			}}}
			if releaseAlternative {
				factsByUnit["other"] = map[string]any{"b2": map[string]any{
					"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0,
				}}
			}
			o["moveFacts"] = factsByUnit
			return o
		}
		intent, memory, _ := decision(t, makeState("h4", false), nil)
		if intent == nil || intent["from"] != "h4" || intent["to"] != "h7" {
			t.Fatalf("first rook leg = %#v", intent)
		}
		intent, memory, _ = decision(t, makeState("h7", false), memory)
		if intent == nil || intent["from"] != "h7" || intent["to"] != "g7" {
			t.Fatalf("second rook leg = %#v", intent)
		}
		intent, _, options := decision(t, makeState("g7", true), memory)
		if intent == nil || intent["from"] != "a1" || intent["to"] != "b2" || optionFor(options, "g7", "h7") != nil {
			t.Fatalf("rook reverse closed measured h7-g7-h7 cycle: intent=%#v options=%#v", intent, options)
		}
	})
	t.Run("Recruit promoted queens cannot restore earlier rank layout", func(t *testing.T) {
		makeState := func(qa, qb, actor string, final bool) map[string]any {
			o := strategyObservation(t,
				strategyCell("qa", qa, "white", "queen"), strategyCell("qb", qb, "white", "queen"),
				strategyCell("qc", "g8", "white", "queen"), strategyCell("qd", "h8", "white", "queen"),
				strategyCell("qhome", "d1", "white", "queen"),
			)
			sequence := map[string]string{"g7": "b7", "b7": "c8", "c8": "b8", "b8": "a8", "a8": "a7", "a7": "g7"}
			from := qa
			if actor == "qb" {
				from = qb
			}
			legal := map[string]any{actor: []any{sequence[from]}}
			if final {
				legal = map[string]any{"qa": []any{"a8"}, "qhome": []any{"d2"}}
			}
			o["legal"] = legal
			moveFacts := map[string]any{}
			for id, destinations := range legal {
				to := destinations.([]any)[0].(string)
				moveFacts[id] = map[string]any{to: map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}}
			}
			o["moveFacts"] = moveFacts
			return o
		}
		states := []struct{ qa, qb, actor, from, to string }{
			{"g7", "b8", "qa", "g7", "b7"}, {"b7", "b8", "qa", "b7", "c8"}, {"c8", "b8", "qb", "b8", "a8"},
			{"c8", "a8", "qa", "c8", "b8"}, {"b8", "a8", "qb", "a8", "a7"}, {"b8", "a7", "qb", "a7", "g7"},
		}
		var memory map[string]int64
		for _, state := range states {
			intent, next, _ := decision(t, makeState(state.qa, state.qb, state.actor, false), memory)
			if intent == nil || intent["from"] != state.from || intent["to"] != state.to {
				t.Fatalf("queen setup leg %s→%s chose %#v", state.from, state.to, intent)
			}
			memory = next
		}
		intent, _, options := decision(t, makeState("b8", "g7", "qa", true), memory)
		if intent == nil || intent["from"] != "d1" || intent["to"] != "d2" || optionFor(options, "b8", "a8") != nil {
			t.Fatalf("promoted queens restored prior loose layout: intent=%#v options=%#v memory=%#v", intent, options, memory)
		}
	})
}

func TestQuietCycleRingPreservesPlacementOnlyActionsAndFitsLeaderMemory(t *testing.T) {
	t.Run("action preserves placement history", func(t *testing.T) {
		o := decodeBeaconScenario(t)
		intent, next, _ := decision(t, o, map[string]int64{"quietVacated0": 7, "quietVacated1": 6, "quietVacated2": 23})
		if intent == nil || intent["kind"] != "action" {
			t.Fatalf("fixture did not take a system action: %#v", intent)
		}
		for key, want := range map[string]int64{"quietVacated0": 7, "quietVacated1": 6, "quietVacated2": 23} {
			if next[key] != want {
				t.Fatalf("%s changed although action leaves piece placement intact: %#v", key, next)
			}
		}
	})
	t.Run("leader guard plus quiet ring stays within Recruit limit", func(t *testing.T) {
		first := strategyObservation(t, strategyCell("wk", "e1", "white", "king"), strategyCell("wp", "d2", "white", "pawn"))
		first["legal"] = map[string]any{"wk": []any{"e2"}}
		first["moveFacts"] = facts("wk", "e2", true, 1, 0, 0)
		_, guarded, _ := decision(t, first, nil)
		second := strategyObservation(t, strategyCell("wk", "e2", "white", "king"), strategyCell("wp", "d2", "white", "pawn"))
		second["legal"] = map[string]any{"wp": []any{"d3"}}
		second["moveFacts"] = facts("wp", "d3", true, 1, 0, 0)
		intent, next, _ := decision(t, second, guarded)
		if intent == nil || intent["from"] != "d2" || intent["to"] != "d3" {
			t.Fatalf("fixture did not append quiet-cycle state: %#v", intent)
		}
		if len(next) > 32 {
			t.Fatalf("leader guard plus quiet cycle ring has %d entries, Recruit allows at most 32: %#v", len(next), next)
		}
		if next["quietVacated0"] == -1 {
			t.Fatalf("ordinary quiet move did not record vacated source: %#v", next)
		}
	})
}

func TestRepeatPenaltyNeedsViableAlternativeAndExemptsExceptionalMoves(t *testing.T) {
	makeRepeat := func(t *testing.T) map[string]any {
		t.Helper()
		o := strategyObservation(t, strategyCell("repeat", "e3", "white", "knight"), strategyCell("other", "a2", "white", "bishop"))
		o["legal"] = map[string]any{"repeat": []any{"e2"}, "other": []any{"b3"}}
		o["moveFacts"] = map[string]any{
			"repeat": map[string]any{"e2": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
			"other":  map[string]any{"b3": map[string]any{"destinationKnown": true, "protectedAfter": 1, "threatenedAfter": 0, "cheapestThreatAfter": 0, "patrolGain": 0}},
		}
		return o
	}
	seed := strategyObservation(t, strategyCell("repeat", "e2", "white", "knight"))
	seed["legal"] = map[string]any{"repeat": []any{"e3"}}
	seed["moveFacts"] = facts("repeat", "e3", true, 1, 0, 0)
	_, memory, _ := decision(t, seed, nil) // real persisted lastQuietTo for e3
	t.Run("viable other ordinary move", func(t *testing.T) {
		o := makeRepeat(t)
		intent, _, options := decision(t, o, memory)
		if intent == nil || intent["from"] != "a2" || intent["to"] != "b3" || optionFor(options, "e3", "e2") != nil {
			t.Fatalf("recently-vacated return was not excluded in favour of viable alternative: intent=%#v options=%#v", intent, options)
		}
	})
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"forced", func(o map[string]any) { o["legal"].(map[string]any)["other"] = []any{} }},
		{"capture", func(o map[string]any) {
			o["enemy"] = []any{strategyCell("victim", "e2", "black", "pawn")}
			o["affordability"] = map[string]any{"repeat": map[string]any{"e2": map[string]any{
				"kill":    map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
				"capture": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
			}}}
		}},
		{"en passant", func(o map[string]any) {
			o["own"].([]any)[0].(map[string]any)["rank"] = "pawn"
			o["enemy"] = []any{strategyCell("victim", "d3", "black", "pawn")}
			o["enPassant"] = map[string]any{"repeat": map[string]any{"e2": "d3"}}
			o["affordability"] = map[string]any{"repeat": map[string]any{"e2": map[string]any{
				"kill":    map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
				"capture": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
			}}}
		}},
		{"threat escape", func(o map[string]any) {
			o["danger"] = map[string]any{"e3": map[string]any{"threat": 1, "guarded": 0, "cheapest": 1}}
		}},
		{"leader", func(o map[string]any) { o["own"].([]any)[0].(map[string]any)["rank"] = "king" }},
		{"Beacon bearer", func(o map[string]any) {
			o["rules"].(map[string]any)["beaconEnabled"] = true
			o["beacon"] = map[string]any{"lifecycle": "deployed", "bearerSquare": "e3", "everHandedOff": true}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := makeRepeat(t)
			tc.mutate(o)
			_, _, options := decision(t, o, memory)
			option := requireOption(t, options, "e3", "e2")
			if hasTerm(option, "repeatPenalty", "quiet") {
				t.Fatalf("%s incorrectly received repeat penalty: %#v", tc.name, options)
			}
		})
	}
	// A king-cargo convoy is a delivery exception even when the old memory
	// happens to name its square; it is never an ordinary quiet repeat.
	convoy := strategyCell("convoy", "e3", "white", "rook")
	convoy["convoy"], convoy["kingCargo"] = true, true
	o := strategyObservation(t, convoy, strategyCell("other", "a2", "white", "bishop"))
	o["legal"] = map[string]any{"convoy": []any{"e2"}, "other": []any{"b3"}}
	o["moveFacts"] = map[string]any{}
	o["deliverySquares"] = []any{"e2"}
	_, _, options := decision(t, o, memory)
	if hasTerm(requireOption(t, options, "e3", "e2"), "repeatPenalty", "quiet") {
		t.Fatalf("delivery convoy received repeat penalty: %#v", options)
	}
}

func TestMoraleReserveExcessRetreatAndIncursionPenalty(t *testing.T) {
	makeKing := func(from string) map[string]any {
		o := strategyObservation(t, strategyCell("wk", from, "white", "king"), strategyCell("support", "e7", "white", "pawn"))
		// This visible capture threshold is deliberately unrelated to the king's
		// legal move. It makes current_morale_need, rather than a zero default,
		// determine the desired reserve band.
		o["affordability"] = map[string]any{"probe": map[string]any{"x": map[string]any{
			"capture": map[string]any{"affordable": false, "requiredMorale": 2},
		}}}
		return o
	}
	for _, tc := range []struct {
		name, destination, detail string
	}{
		{"exact plus one reserve", "e4", "coveredAdvance"},
		{"exact plus two reserve", "e5", "coveredAdvance"},
		{"exact plus three excess", "e6", "excessAdvance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := makeKing("e2")
			o["legal"] = map[string]any{"wk": []any{tc.destination}}
			o["moveFacts"] = facts("wk", tc.destination, true, 1, 0, 0)
			_, _, options := decision(t, o, nil)
			option := requireOption(t, options, "e2", tc.destination)
			value, ok := termValue(option, "moralePush", tc.detail)
			if !ok {
				t.Fatalf("requiredMorale=2 did not produce %s at %s: %#v", tc.detail, tc.destination, option)
			}
			if tc.detail == "coveredAdvance" && value <= 0 || tc.detail == "excessAdvance" && value >= 0 {
				t.Fatalf("%s has wrong sign %v: %#v", tc.detail, value, option)
			}
		})
	}
	t.Run("three excess morale rewards retreat", func(t *testing.T) {
		o := makeKing("e6")
		o["ownMorale"] = 5
		o["legal"] = map[string]any{"wk": []any{"e5"}}
		o["moveFacts"] = facts("wk", "e5", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		value, ok := termValue(requireOption(t, options, "e6", "e5"), "moralePush", "excessRetreat")
		if !ok || value <= 0 {
			t.Fatalf("excess morale did not reward retreat: %#v", options)
		}
	})
	t.Run("rank one incursion penalty prevents false excess", func(t *testing.T) {
		o := makeKing("e3")
		o["ownMoralePenalty"] = 1
		o["legal"] = map[string]any{"wk": []any{"e6"}}
		o["moveFacts"] = facts("wk", "e6", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		option := requireOption(t, options, "e3", "e6")
		if !hasTerm(option, "moralePush", "coveredAdvance") || hasTerm(option, "moralePush", "excessAdvance") {
			t.Fatalf("ownMoralePenalty was not applied to post-move morale: %#v", option)
		}
	})
}
