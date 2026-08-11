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
	t.Run("forge rule disabled", func(t *testing.T) {
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

func TestCurrentStrategyReportsBoundedInterpreterSteps(t *testing.T) {
	observation := decodeBeaconScenario(t)
	observation["moveFacts"] = map[string]any{}
	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		t.Fatal(err)
	}
	obs, _ := json.Marshal(observation)
	params, _ := json.Marshal(decodedRecruitParams(t))
	if _, steps, err := program.CallWithStepLimit(100_000, "decide", string(obs), `{}`, string(params), `0`); err != nil {
		t.Fatalf("current strategy exceeded 100000 interpreter steps after %d: %v", steps, err)
	} else if steps == 0 {
		t.Fatal("current strategy reported zero interpreter steps")
	} else {
		t.Logf("current strategy beacon scenario used %d interpreter steps", steps)
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
