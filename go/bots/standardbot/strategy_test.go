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
	"pieces":{"e1":{"unitId":"wk","side":"white","rank":"king","convoy":false,"cargoCount":0,"kingCargo":false,"ghost":false,"refitting":false}},
	"legal":{}, "affordability":{}, "enPassant":{}, "candidates":{},
	"deliverySquares":[], "convoyHome":{}, "blockingBase":[],
	"rules":{"veteranProgression":false,"allowsKill":true,"allowsCapture":true,"pieceChargeMs":{},"baseSquares":{},"beaconEnabled":true,"beaconForgeEnabled":false,"beaconKingStartsAsBearer":false,"specialistsEnabled":false,"woodWallsEnabled":false,"stoneWallsEnabled":false},
	"systems":{"training":false,"walls":false,"beacon":false,"prisoners":false,"morale":false,"espionage":false},
	"beacon":{"lifecycle":"undeployed","bearerSquare":"","everHandedOff":false}, "walls":[], "enemyManaged":0
}`

func decodedTierParams(t *testing.T, tier string) map[string]any {
	t.Helper()
	var envelope struct {
		Tiers map[string]map[string]any `json:"tiers"`
	}
	if err := json.Unmarshal(standardbot.Params, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Tiers[tier]
}

func decodedRecruitParams(t *testing.T) map[string]any {
	t.Helper()
	return decodedTierParams(t, "recruit")
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
		replaceSidePieces(observation, "white", strategyCell("wp", "a2", "white", "pawn"))
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
		`"develop": ["firstForward"]`, `"coverage": ["patrol", "guards"]`,
		`"safety": ["risk", "kingIntoStrike", "unknownQuiet", "unsupportedQuiet", "guardedBy", "soleGuardLost"]`,
		`"kingHunt": ["visible"]`, `"repeatPenalty": ["quiet"]`,
		`"beaconAggression": ["handOff", "deploy", "restore", "forge", "guardedAdvance", "regroup"]`,
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
	observation["candidates"] = map[string]any{"wk": map[string]any{"e2": map[string]any{
		"destinationVisible": true, "patrolGain": 0, "guardedBy": relationTestSquares('a', 1),
		"threatenedBy": relationTestSquares('f', 0),
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
	replaceSidePieces(observation, "black", strategyCell("bk", "h8", "black", "king"), strategyCell("bp", "d4", "black", "pawn"))
	observation["legal"] = map[string]any{
		"wk": []any{"e3", "d3"}, "wn": []any{"d4", "e4", "a3"}, "wb": []any{"g2", "h3"}, "wr": []any{"a2", "a3"}, "wp": []any{"d3", "d4"},
	}
	observation["candidates"] = map[string]any{
		"wk": map[string]any{"e3": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}, "d3": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
		"wn": map[string]any{"d4": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 2}, "e4": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 1}, "a3": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
		"wb": map[string]any{"g2": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 1}, "h3": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 1}},
		"wr": map[string]any{"a2": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 1}, "a3": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 1}},
		"wp": map[string]any{"d3": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}, "d4": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
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
		putStrategyPiece(observation, strategyCell(extra.id, extra.from, "white", extra.rank))
		observation["legal"].(map[string]any)[extra.id] = []any{extra.to}
		observation["candidates"].(map[string]any)[extra.id] = map[string]any{extra.to: map[string]any{
			"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 1,
		}}
	}
	if got := countStrategyPieces(observation, "white"); got != 16 {
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
	first["candidates"] = map[string]any{"wk": map[string]any{"e2": map[string]any{"destinationVisible": true, "patrolGain": 0, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0)}}}
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
	moveStrategyPiece(second, "e1", "e2")
	second["legal"] = map[string]any{"wk": []any{"e1"}}
	second["candidates"] = map[string]any{"wk": map[string]any{"e1": map[string]any{"destinationVisible": true, "patrolGain": 0, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0)}}}
	got := decideWithMemoryJSON(t, second, start[1], decodedRecruitParams(t))
	if !strings.HasPrefix(got, `[null,`) {
		t.Fatalf("quiet immediate king reverse was not blocked: %s", got)
	}
	strategyPieceAt(second, "e2")["threatenedBy"] = []any{"h8"}
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
		"refitting": false,
	}
}

func putStrategyPiece(observation map[string]any, cell map[string]any) {
	pieces := observation["pieces"].(map[string]any)
	square := cell["square"].(string)
	wire := make(map[string]any, len(cell)-1)
	for key, value := range cell {
		if key != "square" {
			wire[key] = value
		}
	}
	pieces[square] = wire
}

func replaceSidePieces(observation map[string]any, side string, cells ...map[string]any) {
	pieces := observation["pieces"].(map[string]any)
	for square, value := range pieces {
		if value.(map[string]any)["side"] == side {
			delete(pieces, square)
		}
	}
	for _, cell := range cells {
		putStrategyPiece(observation, cell)
	}
}

func replaceSidePieceValues(observation map[string]any, side string, cells []any) {
	typed := make([]map[string]any, 0, len(cells))
	for _, cell := range cells {
		typed = append(typed, cell.(map[string]any))
	}
	replaceSidePieces(observation, side, typed...)
}

func strategyPieceAt(observation map[string]any, square string) map[string]any {
	return observation["pieces"].(map[string]any)[square].(map[string]any)
}

func moveStrategyPiece(observation map[string]any, from, to string) {
	pieces := observation["pieces"].(map[string]any)
	pieces[to] = pieces[from]
	delete(pieces, from)
}

func countStrategyPieces(observation map[string]any, side string) int {
	count := 0
	for _, value := range observation["pieces"].(map[string]any) {
		if value.(map[string]any)["side"] == side {
			count++
		}
	}
	return count
}

func strategyObservation(t *testing.T, own ...map[string]any) map[string]any {
	t.Helper()
	observation := decodeBeaconScenario(t)
	observation["pieces"] = map[string]any{}
	for _, cell := range own {
		putStrategyPiece(observation, cell)
	}
	observation["rules"].(map[string]any)["beaconEnabled"] = false
	observation["beacon"] = map[string]any{"lifecycle": "", "bearerSquare": "", "everHandedOff": false}
	observation["candidates"] = map[string]any{}
	return observation
}

func relationTestSquares(start byte, count int) []any {
	squares := make([]any, 0, count)
	for i := 0; i < count; i++ {
		squares = append(squares, string([]byte{start + byte(i), '1'}))
	}
	return squares
}

func candidateFact(visible bool, guarded, threatened, patrol int) map[string]any {
	return map[string]any{
		"destinationVisible": visible, "guardedBy": relationTestSquares('a', guarded),
		"threatenedBy": relationTestSquares('f', threatened), "patrolGain": patrol,
	}
}

func candidateFacts(unit, destination string, visible bool, guarded, threatened, patrol int) map[string]any {
	return map[string]any{unit: map[string]any{destination: map[string]any{
		"destinationVisible": visible, "guardedBy": relationTestSquares('a', guarded),
		"threatenedBy": relationTestSquares('f', threatened), "patrolGain": patrol,
	}}}
}

func decision(t *testing.T, observation map[string]any, memory map[string]int64) (map[string]any, map[string]int64, []map[string]any) {
	t.Helper()
	return decisionWithParams(t, observation, memory, decodedRecruitParams(t))
}

func decisionWithParams(t *testing.T, observation map[string]any, memory map[string]int64, parameterRow map[string]any) (map[string]any, map[string]int64, []map[string]any) {
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
	params, _ := json.Marshal(parameterRow)
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
			first["candidates"] = candidateFacts("leader", tc.firstTo, true, 1, 0, 0)
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
			reverse["candidates"] = candidateFacts("leader", tc.firstFrom, true, 1, 0, 0)
			intent, _, _ := decision(t, reverse, memory)
			if intent != nil {
				t.Fatalf("immediate quiet reverse escaped guard: %#v", intent)
			}

			// Any actual placement change releases the one-ply guard.
			putStrategyPiece(reverse, strategyCell("changed", "a8", "black", "pawn"))
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
	first["candidates"] = candidateFacts("wk", "e2", true, 1, 0, 0)
	_, memory, _ := decision(t, first, nil)

	// While the command is charging, the physical board still contains the
	// pre-move placement.  It must retain rather than clear the guard.
	charging := strategyObservation(t, strategyCell("wk", "e1", "white", "king"))
	strategyPieceAt(charging, "e1")["charging"] = map[string]any{"square": "e2", "remainingMs": 500}
	charging["legal"] = map[string]any{"wk": []any{"e2"}}
	charging["candidates"] = candidateFacts("wk", "e2", true, 1, 0, 0)
	_, stillArmed, _ := decision(t, charging, memory)
	if stillArmed["leaderGuardActive"] != 1 {
		t.Fatalf("charging pre-layout cleared guard: %#v", stillArmed)
	}

	reverse := strategyObservation(t, strategyCell("wk", "e2", "white", "king"))
	reverse["legal"] = map[string]any{"wk": []any{"e1"}}
	reverse["candidates"] = candidateFacts("wk", "e1", true, 1, 0, 0)
	strategyPieceAt(reverse, "e2")["threatenedBy"] = []any{"h8"}
	intent, _, _ := decision(t, reverse, memory)
	if intent == nil || intent["to"] != "e1" {
		t.Fatalf("threat escape was incorrectly held by guard: %#v", intent)
	}
}

func TestChargingRouteReplacementIsTheOnlyActiveCommandFront(t *testing.T) {
	affordable := map[string]any{
		"kill":    map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
		"capture": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
	}
	makeObservation := func(targetRank string) map[string]any {
		o := strategyObservation(t,
			strategyCell("wk", "e1", "white", "king"),
			strategyCell("charger", "a2", "white", "pawn"),
			strategyCell("idle", "h1", "white", "rook"),
		)
		putStrategyPiece(o, strategyCell("route-target", "c2", "black", targetRank))
		putStrategyPiece(o, strategyCell("replacement-target", "a3", "black", "rook"))
		putStrategyPiece(o, strategyCell("tempting", "h8", "black", "queen"))
		strategyPieceAt(o, "a2")["charging"] = map[string]any{"square": "c2", "remainingMs": 100}
		o["legal"] = map[string]any{
			"charger": []any{"c2", "a3"},
			"idle":    []any{"h8"},
		}
		o["candidates"] = map[string]any{}
		o["affordability"] = map[string]any{
			"charger": map[string]any{"a3": affordable},
			"idle":    map[string]any{"h8": affordable},
		}
		// Without the active route, this higher-scoring system proposal would
		// also compete. It must not open a second command front.
		o["rules"].(map[string]any)["beaconEnabled"] = true
		o["beacon"] = map[string]any{"lifecycle": "undeployed", "bearerSquare": "", "everHandedOff": false}
		return o
	}

	t.Run("visible king target is retained", func(t *testing.T) {
		o := makeObservation("king")
		intent, _, options := decisionWithParams(t, o, nil, decodedTierParams(t, "commander"))
		if intent != nil || len(options) != 0 {
			t.Fatalf("visible king charge was replaced: intent=%#v options=%#v", intent, options)
		}
	})

	t.Run("pawn target may be replaced by the same actor only", func(t *testing.T) {
		o := makeObservation("pawn") // the only mutation from the protected case
		intent, _, options := decisionWithParams(t, o, nil, decodedTierParams(t, "commander"))
		if intent == nil || intent["kind"] != "move" || intent["from"] != "a2" || intent["to"] != "a3" {
			t.Fatalf("lower-value route did not use its offered replacement: intent=%#v options=%#v", intent, options)
		}
		if !hasTerm(requireOption(t, options, "a2", "a3"), "tempo", "replaceCharge") {
			t.Fatalf("replacement did not explain its remaining-time urgency cost: %#v", options)
		}
		for _, option := range options {
			candidate := option["intent"].(map[string]any)
			if candidate["kind"] != "move" || candidate["from"] != "a2" {
				t.Fatalf("active route leaked an unrelated move or system action: %#v", options)
			}
		}
		if optionFor(options, "h1", "h8") != nil {
			t.Fatalf("higher-value idle capture opened a second command front: %#v", options)
		}
	})
}

func TestRoutedQuietReplacementDoesNotClaimUnknownPostMoveSafety(t *testing.T) {
	t.Run("target lock dodge is not called safe", func(t *testing.T) {
		o := strategyObservation(t, strategyCell("charger", "a2", "white", "pawn"))
		putStrategyPiece(o, strategyCell("route-target", "c2", "black", "pawn"))
		cell := strategyPieceAt(o, "a2")
		cell["charging"] = map[string]any{"square": "c2", "remainingMs": 500}
		cell["targetLocked"] = true
		o["legal"] = map[string]any{"charger": []any{"c2", "a3"}}
		o["candidates"] = map[string]any{}
		params := decodedTierParams(t, "commander")
		params["passBelow"] = -100.0
		_, _, options := decisionWithParams(t, o, nil, params)
		option := requireOption(t, options, "a2", "a3")
		if !hasTerm(option, "targetLock", "dodge") {
			t.Fatalf("routed replacement lost the observable target-lock dodge: %#v", option)
		}
		if hasTerm(option, "targetLock", "safeDodge") {
			t.Fatalf("routed replacement without a candidate fact claimed safeDodge: %#v", option)
		}
	})

	t.Run("threatened king move is not called an escape", func(t *testing.T) {
		o := strategyObservation(t, strategyCell("wk", "e1", "white", "king"))
		putStrategyPiece(o, strategyCell("route-target", "e2", "black", "pawn"))
		cell := strategyPieceAt(o, "e1")
		cell["charging"] = map[string]any{"square": "e2", "remainingMs": 500}
		cell["threatenedBy"] = []any{"a8"}
		cell["targetLocked"] = true // keeps the unknown quiet replacement observable
		o["legal"] = map[string]any{"wk": []any{"e2", "f1"}}
		o["candidates"] = map[string]any{}
		params := decodedTierParams(t, "commander")
		params["passBelow"] = -100.0
		_, _, options := decisionWithParams(t, o, nil, params)
		option := requireOption(t, options, "e1", "f1")
		if hasTerm(option, "kingSafety", "escape") {
			t.Fatalf("routed king replacement without a candidate fact claimed escape: %#v", option)
		}
	})
}

func TestRecruitRouteReplacementValuesRemainingTimeAgainstRetention(t *testing.T) {
	t.Run("near-complete pawn route retains over marginal quiet move", func(t *testing.T) {
		o := strategyObservation(t, strategyCell("charger", "a2", "white", "pawn"))
		putStrategyPiece(o, strategyCell("route-target", "c2", "black", "pawn"))
		strategyPieceAt(o, "a2")["charging"] = map[string]any{"square": "c2", "remainingMs": 1}
		o["legal"] = map[string]any{"charger": []any{"c2", "a3"}}
		o["candidates"] = map[string]any{} // active routes may lack a fog-correct settled projection
		intent, _, options := decision(t, o, nil)
		if intent != nil || len(options) != 0 {
			t.Fatalf("Recruit discarded a nearly settled route for marginal progress: intent=%#v options=%#v", intent, options)
		}
	})

	t.Run("affordable king replacement clears the retention baseline", func(t *testing.T) {
		o := strategyObservation(t, strategyCell("charger", "a2", "white", "pawn"))
		putStrategyPiece(o, strategyCell("route-target", "c2", "black", "pawn"))
		putStrategyPiece(o, strategyCell("king", "b3", "black", "king"))
		strategyPieceAt(o, "a2")["charging"] = map[string]any{"square": "c2", "remainingMs": 1}
		o["legal"] = map[string]any{"charger": []any{"c2", "b3"}}
		o["affordability"] = map[string]any{"charger": map[string]any{"b3": map[string]any{
			"capture": map[string]any{"affordable": true, "requiredMorale": 0},
		}}}
		intent, _, options := decision(t, o, nil)
		if intent == nil || intent["from"] != "a2" || intent["to"] != "b3" || !hasTerm(requireOption(t, options, "a2", "b3"), "tempo", "replaceCharge") {
			t.Fatalf("Recruit did not replace a pawn route with a direct king capture: intent=%#v options=%#v", intent, options)
		}
	})
}

func TestMultipleChargingRoutesOnlyConsiderTheirOwnReplacements(t *testing.T) {
	o := strategyObservation(t,
		strategyCell("first", "a2", "white", "pawn"),
		strategyCell("second", "b2", "white", "pawn"),
		strategyCell("idle", "h1", "white", "queen"),
	)
	putStrategyPiece(o, strategyCell("first-target", "c2", "black", "pawn"))
	putStrategyPiece(o, strategyCell("second-target", "d2", "black", "pawn"))
	putStrategyPiece(o, strategyCell("first-replacement", "a3", "black", "rook"))
	putStrategyPiece(o, strategyCell("second-replacement", "b3", "black", "bishop"))
	putStrategyPiece(o, strategyCell("idle-target", "h8", "black", "rook"))
	strategyPieceAt(o, "a2")["charging"] = map[string]any{"square": "c2", "remainingMs": 500}
	strategyPieceAt(o, "b2")["charging"] = map[string]any{"square": "d2", "remainingMs": 700}
	o["legal"] = map[string]any{
		"first":  []any{"c2", "a3"},
		"second": []any{"d2", "b3"},
		"idle":   []any{"h8"},
	}
	o["candidates"] = map[string]any{}
	affordable := map[string]any{
		"kill":    map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
		"capture": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
	}
	o["affordability"] = map[string]any{
		"first":  map[string]any{"a3": affordable},
		"second": map[string]any{"b3": affordable},
		"idle":   map[string]any{"h8": affordable},
	}
	intent, _, options := decisionWithParams(t, o, nil, decodedTierParams(t, "commander"))
	if intent == nil || intent["kind"] != "move" || intent["from"] != "a2" && intent["from"] != "b2" {
		t.Fatalf("simultaneous routes did not choose an offered same-route replacement: intent=%#v options=%#v", intent, options)
	}
	for _, option := range options {
		candidate := option["intent"].(map[string]any)
		if candidate["kind"] != "move" || candidate["from"] != "a2" && candidate["from"] != "b2" {
			t.Fatalf("simultaneous routes admitted an idle actor: %#v", options)
		}
	}
}

func TestSimultaneousInterrogationsBusyBothTargetsAndOurKing(t *testing.T) {
	makeObservation := func() map[string]any {
		o := strategyObservation(t,
			strategyCell("wk", "e1", "white", "king"),
			strategyCell("own-target", "a2", "white", "queen"),
			strategyCell("idle", "b2", "white", "pawn"),
		)
		putStrategyPiece(o, strategyCell("enemy-victim", "h7", "black", "pawn"))
		putStrategyPiece(o, strategyCell("enemy-king", "h8", "black", "king"))
		putStrategyPiece(o, strategyCell("capture-target", "a3", "black", "rook"))
		strategyPieceAt(o, "a2")["interrogationRemainingMs"] = 400
		strategyPieceAt(o, "h7")["interrogationRemainingMs"] = 600
		o["legal"] = map[string]any{
			"wk":         []any{"e2"},
			"own-target": []any{"a3"},
			"idle":       []any{"b3"},
		}
		o["candidates"] = map[string]any{
			"wk":   map[string]any{"e2": candidateFact(true, 1, 0, 0)},
			"idle": map[string]any{"b3": candidateFact(true, 1, 0, 0)},
		}
		o["affordability"] = map[string]any{"own-target": map[string]any{"a3": map[string]any{
			"kill":    map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
			"capture": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
		}}}
		return o
	}

	both := makeObservation()
	intent, _, options := decision(t, both, nil)
	if intent == nil || intent["from"] != "b2" || optionFor(options, "e1", "e2") != nil || optionFor(options, "a2", "a3") != nil {
		t.Fatalf("simultaneous interrogations did not leave only the idle actor: intent=%#v options=%#v", intent, options)
	}

	ownReleased := makeObservation()
	delete(strategyPieceAt(ownReleased, "a2"), "interrogationRemainingMs")
	_, _, options = decision(t, ownReleased, nil)
	if optionFor(options, "a2", "a3") == nil {
		t.Fatalf("removing own target activity did not restore that actor: %#v", options)
	}

	kingReleased := makeObservation()
	delete(strategyPieceAt(kingReleased, "h7"), "interrogationRemainingMs")
	_, _, options = decision(t, kingReleased, nil)
	if optionFor(options, "e1", "e2") == nil {
		t.Fatalf("removing opposing target activity did not restore our implicit interrogator: %#v", options)
	}
}

func TestKingLeaderSupportUsesNumberDistanceDirectionAndSaturation(t *testing.T) {
	guardedAdvance := func(t *testing.T, supporters ...string) float64 {
		t.Helper()
		own := []map[string]any{strategyCell("wk", "e1", "white", "king")}
		for i, square := range supporters {
			own = append(own, strategyCell("s"+string(rune('a'+i)), square, "white", "pawn"))
		}
		o := strategyObservation(t, own...)
		o["legal"] = map[string]any{"wk": []any{"e2"}}
		o["candidates"] = candidateFacts("wk", "e2", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		value, _ := termValue(requireOption(t, options, "e1", "e2"), "moralePush", "guardedAdvance")
		return value
	}

	one := guardedAdvance(t, "d3")
	two := guardedAdvance(t, "d3", "f3")
	if !(two > one && math.Abs(two-2*one) < 1e-9) {
		t.Fatalf("two close supporters should double one before saturation: one=%v two=%v", one, two)
	}
	near := guardedAdvance(t, "e3")
	far := guardedAdvance(t, "e4")
	if !(near > far && math.Abs(near-2*far) < 1e-9) {
		t.Fatalf("near supporter should count twice distance-two supporter: near=%v far=%v", near, far)
	}
	ahead := guardedAdvance(t, "d3")
	abreast := guardedAdvance(t, "d2")
	behind := guardedAdvance(t, "d1")
	if !(ahead > abreast && math.Abs(ahead-2*abreast) < 1e-9 && behind == 0) {
		t.Fatalf("support direction weights wrong: ahead=%v abreast=%v behind=%v", ahead, abreast, behind)
	}
	saturated := guardedAdvance(t, "d3", "f3")
	overfilled := guardedAdvance(t, "d3", "e3", "f3")
	if math.Abs(saturated-overfilled) > 1e-9 {
		t.Fatalf("support must saturate after two full nearby supporters: saturated=%v overfilled=%v", saturated, overfilled)
	}
}

func TestBeaconBearerRewardsGuardedAdvanceAndRegroup(t *testing.T) {
	for _, tc := range []struct {
		name, from, to, supporter, detail string
	}{
		{"advance", "e2", "e3", "e4", "guardedAdvance"},
		{"regroup", "e4", "d3", "c3", "regroup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bearer := strategyCell("bearer", tc.from, "white", "knight")
			o := strategyObservation(t, bearer, strategyCell("support", tc.supporter, "white", "pawn"))
			o["rules"].(map[string]any)["beaconEnabled"] = true
			o["beacon"] = map[string]any{"lifecycle": "deployed", "bearerSquare": tc.from, "everHandedOff": true}
			o["legal"] = map[string]any{"bearer": []any{tc.to}}
			o["candidates"] = candidateFacts("bearer", tc.to, true, 1, 0, 0)
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
			replaceSidePieceValues(observation, "black", tc.enemy)
			observation["legal"] = map[string]any{tc.cell["unitId"].(string): []any{tc.to}}
			observation["candidates"] = candidateFacts(tc.cell["unitId"].(string), tc.to, true, 1, 0, 0)
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
	observation["candidates"] = candidateFacts("wk", "e3", true, 1, 0, 0)
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
		putStrategyPiece(o, strategyCell("bk", "h8", "black", "king"))
		return o
	}
	base := makeDevelopment()
	base["legal"] = map[string]any{"wn": []any{"d3"}}
	base["candidates"] = map[string]any{"wn": map[string]any{"d3": map[string]any{
		"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 2,
		"nextPossibleMoves": []any{"h8"},
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
			o["candidates"] = map[string]any{"wn": map[string]any{
				"d3": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 2, "nextPossibleMoves": []any{"h8"}},
				"b3": map[string]any{"destinationVisible": tc.known, "guardedBy": relationTestSquares('a', tc.protected), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 2, "nextPossibleMoves": []any{"h8"}},
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
			o["candidates"] = candidateFacts("unit", "d3", true, 1, 0, 1)
			_, _, options := decision(t, o, nil)
			option := requireOption(t, options, "c2", "d3")
			if hasTerm(option, "develop", "firstForward") {
				t.Fatalf("%s received first-development bonus: %#v", tc.name, option)
			}
		})
	}
}

func TestNextPossibleMovesSelectsExactSafeKingHuntWithoutDistanceGuessing(t *testing.T) {
	makeObservation := func(nextPossibleMoves []any) map[string]any {
		hunter := strategyCell("rook", "a1", "white", "rook")
		hunter["moved"] = true
		o := strategyObservation(t, hunter, strategyCell("pawn", "b2", "white", "pawn"))
		replaceSidePieces(o, "black", strategyCell("king", "a8", "black", "king"), strategyCell("victim", "b3", "black", "pawn"))
		o["legal"] = map[string]any{"rook": []any{"a2", "a7"}, "pawn": []any{"b3"}} // non-attacking choice intentionally comes first
		o["candidates"] = map[string]any{"rook": map[string]any{
			"a2": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0},
			"a7": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0, "nextPossibleMoves": nextPossibleMoves},
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
		for _, nextPossibleMoves := range [][]any{nil, {}} {
			intent, _, options := decision(t, makeObservation(nextPossibleMoves), nil)
			if intent == nil || intent["from"] != "b2" || intent["to"] != "b3" || hasTerm(optionFor(options, "a1", "a7"), "kingHunt", "visible") {
				t.Fatalf("empty nextPossibleMoves did not remove +30 hunt preference: intent=%#v options=%#v", intent, options)
			}
		}
	})
	t.Run("exact thirty-point score has the stated decision boundary", func(t *testing.T) {
		for _, tc := range []struct {
			name         string
			material     float64
			wantKingHunt bool
		}{
			{"twenty-nine point competitor loses", 29, true},
			{"thirty-one point competitor wins", 31, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				o := makeObservation([]any{"a8"})
				params := decodedRecruitParams(t)
				params["advance"] = 0.0
				params["safety"] = 0.0
				params["material"] = tc.material
				intent, _, options := decisionWithParams(t, o, nil, params)
				hunt := requireOption(t, options, "a1", "a7")
				value, ok := termValue(hunt, "kingHunt", "visible")
				if !ok || value != 30 {
					t.Fatalf("host-proven king attack score = %v (%v), want exact +30: %#v", value, ok, hunt)
				}
				gotKingHunt := intent != nil && intent["from"] == "a1" && intent["to"] == "a7"
				if gotKingHunt != tc.wantKingHunt {
					t.Fatalf("+30 boundary chose %#v with material=%v, want hunt=%v; options=%#v", intent, tc.material, tc.wantKingHunt, options)
				}
			})
		}
	})
}

func TestPawnPromotionNextUsesHostNextPossibleMoves(t *testing.T) {
	makeObservation := func(side, from, to, promotionSquare string, known bool, protected, threatened int, nextPossibleMoves []any) map[string]any {
		o := strategyObservation(t, strategyCell("pawn", from, side, "pawn"))
		o["side"] = side
		o["legal"] = map[string]any{"pawn": []any{to}}
		o["candidates"] = map[string]any{"pawn": map[string]any{to: map[string]any{
			"destinationVisible": known, "guardedBy": relationTestSquares('a', protected), "threatenedBy": relationTestSquares('f', threatened), "patrolGain": 0, "nextPossibleMoves": nextPossibleMoves,
		}}}
		return o
	}
	for _, tc := range []struct{ name, side, from, to, promotion string }{
		{"white", "white", "a2", "a3", "a8"},
		{"black", "black", "a7", "a6", "a1"},
	} {
		t.Run(tc.name+" safe setup", func(t *testing.T) {
			o := makeObservation(tc.side, tc.from, tc.to, tc.promotion, true, 1, 0, []any{tc.promotion})
			intent, _, options := decision(t, o, nil)
			if intent == nil || intent["from"] != tc.from || intent["to"] != tc.to || !hasTerm(requireOption(t, options, tc.from, tc.to), "advance", "promotionNext") {
				t.Fatalf("safe %s promotion setup lacked promotionNext: intent=%#v options=%#v", tc.name, intent, options)
			}
		})
	}
	for _, tc := range []struct {
		name              string
		known             bool
		protected, threat int
		nextPossibleMoves []any
	}{
		{"empty", true, 1, 0, []any{}}, {"omitted", true, 1, 0, nil},
		{"unknown", false, 1, 0, []any{"a8"}}, {"unprotected", true, 0, 0, []any{"a8"}}, {"unsafe", true, 1, 1, []any{"a8"}},
	} {
		t.Run(tc.name+" has no setup bonus", func(t *testing.T) {
			o := makeObservation("white", "a2", "a3", "a8", tc.known, tc.protected, tc.threat, tc.nextPossibleMoves)
			_, _, options := decision(t, o, nil)
			if hasTerm(requireOption(t, options, "a2", "a3"), "advance", "promotionNext") {
				t.Fatalf("%s pawn received promotionNext: %#v", tc.name, options)
			}
		})
	}
	t.Run("immediate promotion remains senior", func(t *testing.T) {
		o := makeObservation("white", "a7", "a8", "a8", true, 1, 0, []any{"a8"})
		_, _, options := decision(t, o, nil)
		option := requireOption(t, options, "a7", "a8")
		immediate, ok := termValue(option, "advance", "promotion")
		if !ok || immediate <= 4*0.15 || hasTerm(option, "advance", "promotionNext") {
			t.Fatalf("immediate promotion was not senior to setup: %#v", option)
		}
	})
}

func TestGenericSupportFactsRewardNetFormationWithoutOverridingTactics(t *testing.T) {
	makeFact := func(guards, guardedBy []any) map[string]any {
		return map[string]any{
			"destinationVisible": true, "guardedBy": guardedBy, "guards": guards, "patrolGain": 0,
		}
	}
	makeFormation := func(currentGuards []any, first, second map[string]any) map[string]any {
		mover := strategyCell("mover", "c2", "white", "rook")
		mover["guards"] = currentGuards
		p1, p2 := strategyCell("p1", "a2", "white", "pawn"), strategyCell("p2", "b2", "white", "pawn")
		p3, queen := strategyCell("p3", "f2", "white", "pawn"), strategyCell("queen", "e2", "white", "queen")
		for _, target := range []map[string]any{p1, p2, p3, queen} {
			for _, square := range currentGuards {
				if target["square"] == square {
					target["guardedBy"] = []any{"c2"}
				}
			}
		}
		o := strategyObservation(t, mover, p1, p2, p3, queen)
		o["legal"] = map[string]any{"mover": []any{"c3", "d3"}}
		o["candidates"] = map[string]any{"mover": map[string]any{"c3": first, "d3": second}}
		return o
	}
	t.Run("outbound and inbound guard benefits are independent and additive", func(t *testing.T) {
		scoreFact := func(t *testing.T, fact map[string]any) map[string]any {
			t.Helper()
			o := strategyObservation(t,
				strategyCell("mover", "c2", "white", "rook"),
				strategyCell("p1", "a2", "white", "pawn"),
				strategyCell("p2", "b2", "white", "pawn"),
			)
			o["legal"] = map[string]any{"mover": []any{"c3"}}
			o["candidates"] = map[string]any{"mover": map[string]any{"c3": fact}}
			_, _, options := decision(t, o, nil)
			return requireOption(t, options, "c2", "c3")
		}
		outbound := scoreFact(t, makeFact([]any{"a2", "b2"}, nil))
		inbound := scoreFact(t, makeFact(nil, []any{"a2"}))
		both := scoreFact(t, makeFact([]any{"a2", "b2"}, []any{"a2"}))
		outboundValue, outboundOK := termValue(outbound, "coverage", "guards")
		inboundValue, inboundOK := termValue(inbound, "safety", "guardedBy")
		bothOutbound, bothOutboundOK := termValue(both, "coverage", "guards")
		bothInbound, bothInboundOK := termValue(both, "safety", "guardedBy")
		if !outboundOK || outboundValue <= 0 || hasTerm(outbound, "safety", "guardedBy") {
			t.Fatalf("outbound-only relation did not receive only its guards benefit: %#v", outbound)
		}
		if !inboundOK || inboundValue <= 0 || hasTerm(inbound, "coverage", "guards") {
			t.Fatalf("inbound-only relation did not receive only its guardedBy benefit: %#v", inbound)
		}
		if !bothOutboundOK || !bothInboundOK || bothOutbound != outboundValue || bothInbound != inboundValue {
			t.Fatalf("combined relations were not the sum of independent benefits: outbound=%#v inbound=%#v both=%#v", outbound, inbound, both)
		}
	})
	t.Run("net two new guards minus one sole loss beats otherwise equal", func(t *testing.T) {
		plain := makeFact([]any{"f2"}, []any{"a2"})
		positive := makeFact([]any{"a2", "b2"}, []any{"a2"})
		o := makeFormation([]any{"f2"}, plain, positive)
		intent, _, options := decision(t, o, nil)
		if intent == nil || intent["to"] != "d3" || !hasTerm(requireOption(t, options, "c2", "d3"), "coverage", "guards") || !hasTerm(requireOption(t, options, "c2", "d3"), "safety", "guardedBy") {
			t.Fatalf("positive net generic support did not win otherwise-equal move: intent=%#v options=%#v", intent, options)
		}
	})
	t.Run("exposed queen reverses two pawn guards", func(t *testing.T) {
		good := makeFact([]any{"e2"}, []any{"a2"})
		loss := makeFact([]any{"a2", "b2"}, []any{"a2"})
		o := makeFormation([]any{"e2"}, good, loss)
		intent, _, options := decision(t, o, nil)
		if intent == nil || intent["to"] != "c3" {
			t.Fatalf("two pawn guards outweighed exposed queen: intent=%#v options=%#v", intent, options)
		}
		lossOnly := makeFormation([]any{"e2"}, loss, loss)
		lossOnly["legal"] = map[string]any{"mover": []any{"d3"}}
		_, _, lossOptions := decision(t, lossOnly, nil)
		if !hasTerm(requireOption(t, lossOptions, "c2", "d3"), "safety", "soleGuardLost") {
			t.Fatalf("exposed queen did not record abandonment loss: %#v", lossOptions)
		}
	})
	t.Run("redundant guard means no abandonment penalty", func(t *testing.T) {
		plain := makeFact(nil, []any{"a2"})
		o := makeFormation([]any{"f2"}, plain, plain)
		strategyPieceAt(o, "f2")["guardedBy"] = []any{"a2", "c2"}
		o["legal"] = map[string]any{"mover": []any{"d3"}}
		_, _, options := decision(t, o, nil)
		if hasTerm(requireOption(t, options, "c2", "d3"), "safety", "soleGuardLost") {
			t.Fatalf("redundant guard fabricated sole-guard penalty: %#v", options)
		}
	})
	t.Run("capped pawn cluster cannot outweigh queen capture", func(t *testing.T) {
		units := []map[string]any{strategyCell("setup", "c3", "white", "rook"), strategyCell("capture", "h7", "white", "rook")}
		for _, square := range []string{"a1", "b1", "d1", "e1", "f1", "g1", "a2"} {
			units = append(units, strategyCell("pawn"+square, square, "white", "pawn"))
		}
		o := strategyObservation(t, units...)
		putStrategyPiece(o, strategyCell("victim", "h8", "black", "queen"))
		o["legal"] = map[string]any{"setup": []any{"c4"}, "capture": []any{"h8"}}
		o["candidates"] = map[string]any{"setup": map[string]any{"c4": makeFact([]any{"a1", "b1", "d1", "e1", "f1", "g1", "a2"}, []any{"a2"})}}
		o["affordability"] = map[string]any{"capture": map[string]any{"h8": map[string]any{
			"kill": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
		}}}
		intent, _, options := decision(t, o, nil)
		if intent == nil || intent["from"] != "h7" || intent["to"] != "h8" || !hasTerm(requireOption(t, options, "c3", "c4"), "coverage", "guards") {
			t.Fatalf("capped pawn cluster displaced material capture: intent=%#v options=%#v", intent, options)
		}
	})
	t.Run("immediate promotion remains free of support-list terms", func(t *testing.T) {
		o := strategyObservation(t, strategyCell("pawn", "a7", "white", "pawn"), strategyCell("ally", "b2", "white", "pawn"))
		o["legal"] = map[string]any{"pawn": []any{"a8"}}
		o["candidates"] = map[string]any{"pawn": map[string]any{"a8": makeFact([]any{"b2"}, []any{"b2"})}}
		_, _, options := decision(t, o, nil)
		option := requireOption(t, options, "a7", "a8")
		if !hasTerm(option, "advance", "promotion") || hasTerm(option, "coverage", "guards") || hasTerm(option, "safety", "guardedBy") || hasTerm(option, "safety", "soleGuardLost") {
			t.Fatalf("immediate promotion received generic support noise: %#v", option)
		}
	})
}

func TestNextPossibleMovesRejectsUnsafeAndNonExactKingHunts(t *testing.T) {
	makeObservation := func(rank string, enemy map[string]any, fact map[string]any) map[string]any {
		o := strategyObservation(t, strategyCell("unit", "c2", "white", rank))
		putStrategyPiece(o, enemy)
		o["legal"] = map[string]any{"unit": []any{"d3"}}
		o["candidates"] = map[string]any{"unit": map[string]any{"d3": fact}}
		return o
	}
	baseFact := func() map[string]any {
		return map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0, "nextPossibleMoves": []any{"h8"}}
	}
	for _, tc := range []struct {
		name  string
		rank  string
		enemy map[string]any
		fact  map[string]any
	}{
		{"knight adjacent empty", "knight", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0, "nextPossibleMoves": []any{}}},
		{"blocked slider empty", "rook", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
		{"unknown", "rook", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationVisible": false, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0, "nextPossibleMoves": []any{"h8"}}},
		{"unprotected", "rook", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 0), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0, "nextPossibleMoves": []any{"h8"}}},
		{"unsafe", "rook", strategyCell("king", "h8", "black", "king"), map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 1), "patrolGain": 0, "nextPossibleMoves": []any{"h8"}}},
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

func TestNextPossibleMovesEntersBreadthBelowDirectKingCapture(t *testing.T) {
	t.Run("exact post-settlement attack enters Recruit breadth", func(t *testing.T) {
		units := []map[string]any{strategyCell("hunter", "a1", "white", "rook")}
		for _, square := range []string{"g7", "g8", "h7", "f7", "f8"} {
			units = append(units, strategyCell("noise"+square, square, "white", "queen"))
		}
		o := strategyObservation(t, units...)
		putStrategyPiece(o, strategyCell("king", "a8", "black", "king"))
		o["legal"] = map[string]any{"hunter": []any{"a7"}}
		o["candidates"] = map[string]any{"hunter": map[string]any{"a7": map[string]any{
			"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0, "nextPossibleMoves": []any{"a8"},
		}}}
		intent, _, _ := decision(t, o, nil)
		if intent == nil || intent["from"] != "a1" || intent["to"] != "a7" {
			t.Fatalf("exact post-move king attack stayed outside Recruit breadth: %#v", intent)
		}
	})
	t.Run("unknown unprotected or unsafe attack facts stay outside breadth", func(t *testing.T) {
		for _, tc := range []struct {
			name              string
			known             bool
			protected, threat int
		}{
			{"unknown", false, 1, 0}, {"unprotected", true, 0, 0}, {"unsafe", true, 1, 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				units := []map[string]any{strategyCell("hunter", "a1", "white", "rook")}
				for _, square := range []string{"g7", "g8", "h7", "f7", "f8"} {
					units = append(units, strategyCell("noise"+square, square, "white", "queen"))
				}
				o := strategyObservation(t, units...)
				putStrategyPiece(o, strategyCell("king", "a8", "black", "king"))
				o["legal"] = map[string]any{"hunter": []any{"a7"}}
				o["candidates"] = map[string]any{"hunter": map[string]any{"a7": map[string]any{
					"destinationVisible": tc.known, "guardedBy": relationTestSquares('a', tc.protected), "threatenedBy": relationTestSquares('f', tc.threat), "patrolGain": 0, "nextPossibleMoves": []any{"a8"},
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
		putStrategyPiece(o, strategyCell("king", "h8", "black", "king"))
		o["legal"] = map[string]any{"direct": []any{"h8"}, "attack": []any{"g7"}}
		o["candidates"] = map[string]any{"attack": map[string]any{"g7": map[string]any{
			"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0, "nextPossibleMoves": []any{"h8"},
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
	replaceSidePieces(observation, "black",
		strategyCell("noise1", "a3", "black", "pawn"), strategyCell("noise2", "b3", "black", "pawn"),
		strategyCell("noise3", "c3", "black", "pawn"), strategyCell("noise4", "d3", "black", "pawn"),
		strategyCell("king", "h8", "black", "king"),
	)
	if len(units) <= int(decodedRecruitParams(t)["breadth"].(float64)) {
		t.Fatal("fixture must place the hunter beyond Recruit breadth before its king-capture priority bump")
	}
	observation["legal"] = map[string]any{"hunter": []any{"h8"}}
	observation["candidates"] = map[string]any{}
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
		putStrategyPiece(o, strategyCell("bk", "h8", "black", "king"))
		o["legal"] = map[string]any{
			"wk": []any{"e2"}, "near1": []any{"g6"}, "near2": []any{"f8"}, "near3": []any{"h6"}, "near4": []any{"f6"},
		}
		o["candidates"] = map[string]any{
			"wk":    map[string]any{"e2": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
			"near1": map[string]any{"g6": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
			"near2": map[string]any{"f8": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
			"near3": map[string]any{"h6": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
			"near4": map[string]any{"f6": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
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
		o["candidates"].(map[string]any)["wk"] = map[string]any{"e8": map[string]any{
			"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0,
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
		o["candidates"].(map[string]any)["wk"] = map[string]any{"e5": map[string]any{
			"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0,
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
	putStrategyPiece(o, strategyCell("bk", "h8", "black", "king"))
	o["legal"] = map[string]any{
		"wk": []any{"e2"}, "a1": []any{"h8"}, "a2": []any{"h8"}, "a3": []any{"h8"}, "a4": []any{"h8"}, "a5": []any{"h8"},
	}
	o["candidates"] = map[string]any{"wk": map[string]any{"e2": map[string]any{
		"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0,
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
		candidateFactsByUnit := map[string]any{leaderID: map[string]any{leaderTo: map[string]any{
			"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0,
		}}}
		for index := range nearQueens {
			id := nearQueens[index]["unitId"].(string)
			to := []string{"g6", "f8", "h6", "f6", "e8"}[index]
			legal[id] = []any{to}
			candidateFactsByUnit[id] = map[string]any{to: map[string]any{
				"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0,
			}}
		}
		o["legal"], o["candidates"] = legal, candidateFactsByUnit
	}
	t.Run("king capture", func(t *testing.T) {
		units := append([]map[string]any{strategyCell("wk", "e1", "white", "king")}, nearQueens...)
		o := strategyObservation(t, units...)
		putStrategyPiece(o, strategyCell("victim", "e8", "black", "pawn"))
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
	o["candidates"] = map[string]any{
		"qa":    map[string]any{"a8": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
		"qhome": map[string]any{"d2": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
	}
	memory := map[string]int64{"quietVacated0": 6, "quietVacated1": 7, "quietVacated2": 23} // a7, a8, c8
	intent, _, options := decision(t, o, memory)
	if intent == nil || intent["from"] != "d1" || intent["to"] != "d2" || optionFor(options, "b8", "a8") != nil {
		t.Fatalf("multi-piece cycle was not excluded in favour of viable queen move: intent=%#v options=%#v", intent, options)
	}
	t.Run("forced return remains legal", func(t *testing.T) {
		forced := strategyObservation(t, qa)
		forced["legal"] = map[string]any{"qa": []any{"a8"}}
		forced["candidates"] = candidateFacts("qa", "a8", true, 1, 0, 0)
		intent, _, options := decision(t, forced, memory)
		if intent == nil || intent["from"] != "b8" || intent["to"] != "a8" || optionFor(options, "b8", "a8") == nil {
			t.Fatalf("forced cycle return was incorrectly filtered: intent=%#v options=%#v", intent, options)
		}
	})
	t.Run("promotion is not a quiet-cycle return", func(t *testing.T) {
		pawn := strategyCell("pawn", "b7", "white", "pawn")
		promotion := strategyObservation(t, pawn, strategyCell("other", "d1", "white", "queen"))
		promotion["legal"] = map[string]any{"pawn": []any{"b8"}, "other": []any{"d2"}}
		promotion["candidates"] = map[string]any{
			"pawn":  map[string]any{"b8": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
			"other": map[string]any{"d2": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
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
				"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0,
			}}}
			if releaseAlternative {
				factsByUnit["other"] = map[string]any{"b2": map[string]any{
					"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0,
				}}
			}
			o["candidates"] = factsByUnit
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
			candidateFactsByUnit := map[string]any{}
			for id, destinations := range legal {
				to := destinations.([]any)[0].(string)
				candidateFactsByUnit[id] = map[string]any{to: map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}}
			}
			o["candidates"] = candidateFactsByUnit
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
		seedAction := decodeBeaconScenario(t)
		_, actionMemory, _ := decision(t, seedAction, nil)
		first := strategyObservation(t, strategyCell("wk", "e1", "white", "king"), strategyCell("wp", "d2", "white", "pawn"))
		first["revision"] = float64(2)
		first["legal"] = map[string]any{"wk": []any{"e2"}}
		first["candidates"] = candidateFacts("wk", "e2", true, 1, 0, 0)
		_, guarded, _ := decision(t, first, actionMemory)
		second := strategyObservation(t, strategyCell("wk", "e2", "white", "king"), strategyCell("wp", "d2", "white", "pawn"))
		second["revision"] = float64(3)
		second["legal"] = map[string]any{"wp": []any{"d3"}}
		second["candidates"] = candidateFacts("wp", "d3", true, 1, 0, 0)
		intent, next, _ := decision(t, second, guarded)
		if intent == nil || intent["from"] != "d2" || intent["to"] != "d3" {
			t.Fatalf("fixture did not append quiet-cycle state: %#v", intent)
		}
		if len(next) != 31 {
			t.Fatalf("worst-case action + move + leader guard + quiet ring has %d entries, want the measured 31/32 Recruit bound: %#v", len(next), next)
		}
		t.Logf("leader guard plus quiet cycle ring uses %d/32 Recruit memory entries", len(next))
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
		o["candidates"] = map[string]any{
			"repeat": map[string]any{"e2": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
			"other":  map[string]any{"b3": map[string]any{"destinationVisible": true, "guardedBy": relationTestSquares('a', 1), "threatenedBy": relationTestSquares('f', 0), "patrolGain": 0}},
		}
		return o
	}
	seed := strategyObservation(t, strategyCell("repeat", "e2", "white", "knight"))
	seed["legal"] = map[string]any{"repeat": []any{"e3"}}
	seed["candidates"] = candidateFacts("repeat", "e3", true, 1, 0, 0)
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
			putStrategyPiece(o, strategyCell("victim", "e2", "black", "pawn"))
			o["affordability"] = map[string]any{"repeat": map[string]any{"e2": map[string]any{
				"kill":    map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
				"capture": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
			}}}
		}},
		{"en passant", func(o map[string]any) {
			strategyPieceAt(o, "e3")["rank"] = "pawn"
			putStrategyPiece(o, strategyCell("victim", "d3", "black", "pawn"))
			o["enPassant"] = map[string]any{"repeat": map[string]any{"e2": "d3"}}
			o["affordability"] = map[string]any{"repeat": map[string]any{"e2": map[string]any{
				"kill":    map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
				"capture": map[string]any{"affordable": true, "oddsKnown": false, "odds": map[string]any{"success": 1, "defenderKilled": 1, "repelled": 0}},
			}}}
		}},
		{"threat escape", func(o map[string]any) {
			strategyPieceAt(o, "e3")["threatenedBy"] = []any{"h8"}
		}},
		{"leader", func(o map[string]any) { strategyPieceAt(o, "e3")["rank"] = "king" }},
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
	o["candidates"] = map[string]any{}
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
		{"exact plus one reserve", "e4", "guardedAdvance"},
		{"exact plus two reserve", "e5", "guardedAdvance"},
		{"exact plus three excess", "e6", "excessAdvance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := makeKing("e2")
			o["legal"] = map[string]any{"wk": []any{tc.destination}}
			o["candidates"] = candidateFacts("wk", tc.destination, true, 1, 0, 0)
			_, _, options := decision(t, o, nil)
			option := requireOption(t, options, "e2", tc.destination)
			value, ok := termValue(option, "moralePush", tc.detail)
			if !ok {
				t.Fatalf("requiredMorale=2 did not produce %s at %s: %#v", tc.detail, tc.destination, option)
			}
			if tc.detail == "guardedAdvance" && value <= 0 || tc.detail == "excessAdvance" && value >= 0 {
				t.Fatalf("%s has wrong sign %v: %#v", tc.detail, value, option)
			}
		})
	}
	t.Run("three excess morale rewards retreat", func(t *testing.T) {
		o := makeKing("e6")
		o["ownMorale"] = 5
		o["legal"] = map[string]any{"wk": []any{"e5"}}
		o["candidates"] = candidateFacts("wk", "e5", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		value, ok := termValue(requireOption(t, options, "e6", "e5"), "moralePush", "excessRetreat")
		if !ok || value <= 0 {
			t.Fatalf("excess morale did not reward retreat: %#v", options)
		}
	})
	t.Run("retreat preserves one point of reserve", func(t *testing.T) {
		o := makeKing("e6")
		o["ownMorale"] = 5
		// Put the tempting deeper retreat first. Without the post-move reserve
		// floor it receives four times the excessRetreat reward and wins.
		o["legal"] = map[string]any{"wk": []any{"e2", "e5"}}
		o["candidates"] = map[string]any{"wk": map[string]any{
			"e2": candidateFact(true, 1, 0, 0),
			"e5": candidateFact(true, 1, 0, 0),
		}}
		intent, _, options := decision(t, o, nil)
		if intent == nil || intent["from"] != "e6" || intent["to"] != "e5" {
			t.Fatalf("below-reserve retreat beat the bounded regroup: intent=%#v options=%#v", intent, options)
		}
		if !hasTerm(requireOption(t, options, "e6", "e5"), "moralePush", "excessRetreat") {
			t.Fatalf("bounded e6→e5 retreat lost its excessRetreat reward: %#v", options)
		}
		// Explained options are distinct by actor, so isolate the deeper move
		// after proving it lost the real competition and inspect its terms.
		o["legal"] = map[string]any{"wk": []any{"e2"}}
		_, _, deepOptions := decision(t, o, nil)
		if hasTerm(requireOption(t, deepOptions, "e6", "e2"), "moralePush", "excessRetreat") {
			t.Fatalf("e6→e2 retreat spent the required morale reserve: %#v", deepOptions)
		}
	})
	t.Run("rank one incursion penalty prevents false excess", func(t *testing.T) {
		o := makeKing("e3")
		o["ownMoralePenalty"] = 1
		o["legal"] = map[string]any{"wk": []any{"e6"}}
		o["candidates"] = candidateFacts("wk", "e6", true, 1, 0, 0)
		_, _, options := decision(t, o, nil)
		option := requireOption(t, options, "e3", "e6")
		if !hasTerm(option, "moralePush", "guardedAdvance") || hasTerm(option, "moralePush", "excessAdvance") {
			t.Fatalf("ownMoralePenalty was not applied to post-move morale: %#v", option)
		}
	})
}
