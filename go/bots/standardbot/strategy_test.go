// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
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
		`"beaconAggression": ["handOff", "deploy", "restore", "forge", "coveredAdvance"]`,
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
	base := strategyObservation(t, strategyCell("wn", "c2", "white", "knight"))
	base["enemy"] = []any{strategyCell("bk", "h8", "black", "king")}
	base["legal"] = map[string]any{"wn": []any{"d3"}}
	base["moveFacts"] = facts("wn", "d3", true, 1, 0, 2)
	_, _, options := decision(t, base, nil)
	option := optionFor(options, "c2", "d3")
	if !hasTerm(option, "develop", "firstForward") || !hasTerm(option, "coverage", "patrol") || !hasTerm(option, "kingHunt", "visible") {
		t.Fatalf("known protected first development did not receive all strategic terms: %#v", option)
	}

	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		forbid []string
	}{
		{"moved back", func(o map[string]any) { o["own"].([]any)[0].(map[string]any)["moved"] = true }, []string{"develop"}},
		{"pawn", func(o map[string]any) { o["own"].([]any)[0].(map[string]any)["rank"] = "pawn" }, []string{"develop"}},
		{"unknown", func(o map[string]any) { o["moveFacts"] = facts("wn", "d3", false, 1, 0, 2) }, []string{"develop", "coverage", "kingHunt"}},
		{"unprotected", func(o map[string]any) { o["moveFacts"] = facts("wn", "d3", true, 0, 0, 2) }, []string{"develop", "coverage", "kingHunt"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := strategyObservation(t, strategyCell("wn", "c2", "white", "knight"))
			o["enemy"] = []any{strategyCell("bk", "h8", "black", "king")}
			o["legal"] = map[string]any{"wn": []any{"d3"}}
			o["moveFacts"] = facts("wn", "d3", true, 1, 0, 2)
			tc.mutate(o)
			_, _, options := decision(t, o, nil)
			option := optionFor(options, "c2", "d3")
			details := map[string]string{"develop": "firstForward", "coverage": "patrol", "kingHunt": "visible"}
			bad := false
			for _, term := range tc.forbid {
				bad = bad || hasTerm(option, term, details[term])
			}
			if bad {
				t.Fatalf("%s received protected-development terms: %#v", tc.name, option)
			}
		})
	}
}

func TestKingCaptureIsPrioritizedWithinBreadth(t *testing.T) {
	// The nearest enemy pieces intentionally consume Recruit's shallow breadth.
	// The hunter is farther away, but its legal king capture must still get the
	// priority bump and become the selected real decision.
	units := []map[string]any{
		strategyCell("near1", "a2", "white", "pawn"), strategyCell("near2", "b2", "white", "pawn"),
		strategyCell("near3", "c2", "white", "pawn"), strategyCell("hunter", "h1", "white", "rook"),
	}
	observation := strategyObservation(t, units...)
	observation["enemy"] = []any{
		strategyCell("noise1", "a3", "black", "pawn"), strategyCell("noise2", "b3", "black", "pawn"),
		strategyCell("noise3", "c3", "black", "pawn"), strategyCell("king", "h8", "black", "king"),
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
		_, _, options := decision(t, o, memory)
		if !hasTerm(optionFor(options, "e3", "e2"), "repeatPenalty", "quiet") {
			t.Fatalf("repeat was not penalized despite viable alternative: %#v", options)
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
		o := strategyObservation(t, strategyCell("wk", from, "white", "king"), strategyCell("support", "e5", "white", "pawn"))
		return o
	}
	t.Run("one to two morale reserve advances", func(t *testing.T) {
		o := makeKing("e1")
		o["legal"] = map[string]any{"wk": []any{"e2"}}
		o["moveFacts"] = facts("wk", "e2", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		if !hasTerm(optionFor(options, "e1", "e2"), "moralePush", "coveredAdvance") {
			t.Fatalf("safe covered reserve advance was not rewarded: %#v", options)
		}
	})
	t.Run("three excess morale penalizes advance", func(t *testing.T) {
		o := makeKing("e3")
		o["legal"] = map[string]any{"wk": []any{"e4"}}
		o["moveFacts"] = facts("wk", "e4", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		if !hasTerm(optionFor(options, "e3", "e4"), "moralePush", "excessAdvance") {
			t.Fatalf("three excess morale did not penalize advance: %#v", options)
		}
	})
	t.Run("three excess morale rewards retreat", func(t *testing.T) {
		o := makeKing("e5")
		o["ownMorale"] = 4
		o["legal"] = map[string]any{"wk": []any{"e4"}}
		o["moveFacts"] = facts("wk", "e4", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		if !hasTerm(optionFor(options, "e5", "e4"), "moralePush", "excessRetreat") {
			t.Fatalf("excess morale did not reward retreat: %#v", options)
		}
	})
	t.Run("rank one incursion penalty prevents false excess", func(t *testing.T) {
		o := makeKing("e3")
		o["ownMoralePenalty"] = 1
		o["legal"] = map[string]any{"wk": []any{"e4"}}
		o["moveFacts"] = facts("wk", "e4", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		option := optionFor(options, "e3", "e4")
		if !hasTerm(option, "moralePush", "coveredAdvance") || hasTerm(option, "moralePush", "excessAdvance") {
			t.Fatalf("ownMoralePenalty was not applied to post-move morale: %#v", option)
		}
	})
}
