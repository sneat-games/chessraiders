// Copyright 2026 Sneat.app

package standardbot

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestTypesUnitID(t *testing.T) {
	var zero UnitID
	if !zero.IsZero() {
		t.Error("expected zero.IsZero() to be true")
	}
	zero.Num = 42
	if zero.IsZero() {
		t.Error("expected non-zero Num to not be IsZero()")
	}
	if zero.String() != "42" {
		t.Errorf("String() = %q, want 42", zero.String())
	}
	b, err := zero.MarshalJSON()
	if err != nil || string(b) != "42" {
		t.Errorf("MarshalJSON() = %s, %v", b, err)
	}

	strID := UnitID{Str: "u123"}
	if strID.IsZero() {
		t.Error("expected non-zero Str to not be IsZero()")
	}
	if strID.String() != "u123" {
		t.Errorf("String() = %q, want u123", strID.String())
	}
	b, err = strID.MarshalJSON()
	if err != nil || string(b) != `"u123"` {
		t.Errorf("MarshalJSON() = %s, %v", b, err)
	}

	var parsed UnitID
	if err := json.Unmarshal([]byte("12345"), &parsed); err != nil || parsed.Num != 12345 || parsed.Str != "" {
		t.Errorf("UnmarshalJSON number = %+v, %v", parsed, err)
	}
	if err := json.Unmarshal([]byte(`"hero"`), &parsed); err != nil || parsed.Str != "hero" || parsed.Num != 0 {
		t.Errorf("UnmarshalJSON string = %+v, %v", parsed, err)
	}
	if err := json.Unmarshal([]byte(`[1, 2, 3]`), &parsed); err != nil {
		t.Errorf("UnmarshalJSON invalid type err = %v, want nil", err)
	}
}

func TestIntn(t *testing.T) {
	if got := intn(10, 0); got != 0 {
		t.Errorf("intn(10, 0) = %d, want 0", got)
	}
	if got := intn(10, 1); got != 0 {
		t.Errorf("intn(10, 1) = %d, want 0", got)
	}
	if got := intn(-5, 4); got != 3 {
		t.Errorf("intn(-5, 4) = %d, want 3", got)
	}
	if got := intn(7, 4); got != 3 {
		t.Errorf("intn(7, 4) = %d, want 3", got)
	}
}

func TestBoardHelpers(t *testing.T) {
	if squareFile("") != 0 || squareRankNumber("") != 0 || squareIndex("") != 0 {
		t.Error("empty square didn't return 0")
	}
	if squareName(-1) != "" || squareName(64) != "" {
		t.Error("out of bounds square index didn't return empty string")
	}
	if squareName(0) != "a1" || squareName(63) != "h8" {
		t.Errorf("squareName(0)=%s, squareName(63)=%s", squareName(0), squareName(63))
	}
	if rankValue("wizard") != 0.0 {
		t.Errorf("rankValue(wizard) = %f, want 0", rankValue("wizard"))
	}
	if cellValue(nil) != 0.0 {
		t.Error("cellValue(nil) != 0")
	}
	c := &Cell{Rank: "pawn", KingCargo: true, Ghost: true}
	expectedVal := (PawnValue + KingCargoValue) * GhostDiscount
	if cellValue(c) != expectedVal {
		t.Errorf("cellValue = %f, want %f", cellValue(c), expectedVal)
	}

	if guardedCount(nil) != 0 || threatenedCount(nil) != 0 {
		t.Error("guardedCount or threatenedCount on nil != 0")
	}
	if cellGuardedCount(nil) != 0 || cellThreatenedCount(nil) != 0 {
		t.Error("cellGuardedCount or cellThreatenedCount on nil != 0")
	}

	fact := &CandidateFact{GuardedBy: []string{"a1"}, ThreatenedBy: []string{"b2"}}
	if !isSafeSubject(fact) {
		t.Error("isSafeSubject with guarded > 0 should be true")
	}
	factNoThreat := &CandidateFact{GuardedBy: nil, ThreatenedBy: nil}
	if !isSafeSubject(factNoThreat) {
		t.Error("isSafeSubject with threatened == 0 should be true")
	}
	factUnsafe := &CandidateFact{GuardedBy: nil, ThreatenedBy: []string{"b2"}}
	if isSafeSubject(factUnsafe) {
		t.Error("isSafeSubject with guarded == 0 and threatened > 0 should be false")
	}

	// deliveryLaneBlockers
	ownCells := map[string]*Cell{"c3": {Square: "c3"}}
	enemyCells := map[string]*Cell{"b2": {Square: "b2"}}
	// target with enemy
	if deliveryLaneBlockers([]string{"b2"}, ownCells, enemyCells) != nil {
		t.Error("deliveryLaneBlockers enemy on target should return nil")
	}
	// target with own
	blockers := deliveryLaneBlockers([]string{"c3"}, ownCells, nil)
	if len(blockers) != 1 || blockers[0] != "c3" {
		t.Errorf("deliveryLaneBlockers own on target = %v", blockers)
	}
	// target with open neighbor (ownBySquare[neighbor] == nil)
	if deliveryLaneBlockers([]string{"d4"}, ownCells, nil) != nil {
		t.Error("deliveryLaneBlockers with open neighbor approach should return nil")
	}
	// target with neighbor having enemy
	if deliveryLaneBlockers([]string{"c2"}, ownCells, enemyCells) != nil {
		t.Error("deliveryLaneBlockers with neighbor enemy should return nil")
	}
	// target with no neighbors (out of board)
	if deliveryLaneBlockers([]string{"z9"}, ownCells, nil) != nil {
		t.Error("deliveryLaneBlockers with out of board target should return nil")
	}
	// neighbor enemy directly
	if deliveryLaneBlockers([]string{"a1"}, ownCells, map[string]*Cell{"a2": {Square: "a2"}}) != nil {
		t.Error("deliveryLaneBlockers with neighbor enemy should return nil")
	}

	// rankBitSlot all branches
	for _, r := range []string{"pawn", "knight", "bishop", "rook", "queen", "king"} {
		if rankBitSlot(r) < 0 {
			t.Errorf("rankBitSlot(%s) returned < 0", r)
		}
	}
}

func TestBuildBoardEdgeCases(t *testing.T) {
	rem := 1000
	obs := &Observation{
		Side:      "white",
		Lifecycle: "playing",
		Rules: Rules{
			MaxActiveCommands:  0,
			VeteranProgression: true,
			PieceChargeMs:      map[string]int{"pawn": 5000},
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
		},
		Pieces: map[string]Cell{
			"a1": {
				UnitID:                   UnitID{Str: "u1"},
				Side:                     "white",
				Rank:                     "king",
				TargetLocked:             true,
				InterrogationRemainingMs: &rem,
			},
			"a2": {
				UnitID:                   UnitID{Str: "u2"},
				Side:                     "white",
				Rank:                     "pawn",
				Profession:               "engineer",
				Grade:                    "master",
				Training:                 "something",
				ForgingRemainingMs:       100,
				RecoveryRemainingMs:      100,
				InterrogationRemainingMs: &rem,
			},
			"b2": {
				UnitID:                   UnitID{Str: "e1"},
				Side:                     "black",
				Rank:                     "king",
				InterrogationRemainingMs: &rem,
			},
			"c3": {
				UnitID:   UnitID{Str: "u3"},
				Side:     "white",
				Rank:     "pawn",
				Charging: &ChargingFact{Square: "b2", RemainingMs: 200},
			},
			"c4": {
				UnitID:   UnitID{Str: "u4"},
				Side:     "white",
				Rank:     "pawn",
				Charging: &ChargingFact{Square: "e8", RemainingMs: 200},
			},
		},
	}
	b := buildBoard(obs)
	if !b.lockedUnits["u1"] {
		t.Error("u1 should be locked")
	}
	if !b.protectedChargingUnits["u3"] {
		t.Error("u3 charging enemy king should be protectedChargingUnit")
	}
	if b.chargingUnits["u4"] && b.protectedChargingUnits["u4"] {
		t.Error("u4 charging empty square should not be protected")
	}
	if b.needsFirstMasterEngineer {
		t.Error("master engineer present, needsFirstMasterEngineer should be false")
	}

	// buildBoard with maxActiveCommands reached
	obs.Rules.MaxActiveCommands = 1
	b2 := buildBoard(obs)
	if len(b2.chargingUnits) < b2.maxActiveCommands(obs) {
		t.Errorf("expected charging units >= maxActiveCommands")
	}
}

func (b *boardContext) maxActiveCommands(obs *Observation) int {
	m := obs.Rules.MaxActiveCommands
	if m <= 0 {
		return UnlimitedActiveCommands
	}
	return m
}

func TestMemoryEdgeCases(t *testing.T) {
	// roundHalfUp
	if roundHalfUp(2.5) != 3 || roundHalfUp(2.4) != 2 || roundHalfUp(-2.5) != -3 || roundHalfUp(-2.4) != -2 {
		t.Error("roundHalfUp failed on test values")
	}

	// packCommitted limits
	pLow := packCommitted(1, 0, -3000.0)
	_, _, sLow := unpackCommitted(pLow)
	if sLow > -50.0 {
		t.Errorf("sLow = %f", sLow)
	}
	pHigh := packCommitted(1, 0, 3000.0)
	_, _, sHigh := unpackCommitted(pHigh)
	if sHigh < 50.0 {
		t.Errorf("sHigh = %f", sHigh)
	}

	// decayPower
	if decayPower(1) >= 1.0 {
		t.Error("decayPower(1) should decay")
	}
	if decayPower(100) != decayPower(int64(CommitAgeCap)) {
		t.Error("decayPower(100) should be capped at CommitAgeCap")
	}

	// retentionScore
	mem := map[string]int64{"committed": packCommitted(CommitKindRoute, 2, 10.0)}
	if retentionScore(mem, CommitKindKing) != RouteReplaceBaseline {
		t.Error("retentionScore for wrong kind did not return RouteReplaceBaseline")
	}
	if retentionScore(mem, CommitKindRoute) <= RouteReplaceBaseline {
		t.Error("retentionScore for matching kind should be > RouteReplaceBaseline")
	}

	// isKingChannelStart
	if isKingChannelStart(nil) || isKingChannelStart(&Intent{Kind: "move"}) || !isKingChannelStart(&Intent{Kind: "action", Action: "beacon_restore"}) {
		t.Error("isKingChannelStart failed")
	}

	// kingChannelActive
	bNoKing := &boardContext{}
	if kingChannelActive(&Observation{}, bNoKing) {
		t.Error("kingChannelActive with nil kingCell should be false")
	}
	bWithKing := &boardContext{kingCell: &Cell{Rank: "king"}}
	if !kingChannelActive(&Observation{Beacon: BeaconFact{Lifecycle: "restoring"}}, bWithKing) {
		t.Error("kingChannelActive restoring should be true")
	}

	// nextCommitted
	obs := &Observation{
		Rules:  Rules{PieceChargeMs: map[string]int{"pawn": 5000}},
		Beacon: BeaconFact{Lifecycle: "restoring"},
	}
	b := &boardContext{
		ownBySquare:   map[string]*Cell{"a2": {Rank: "pawn", UnitID: UnitID{Str: "p1"}}},
		kingCell:      &Cell{UnitID: UnitID{Str: "k1"}},
		chargingUnits: map[string]bool{"p1": true},
	}
	// move with charge
	chosenMove := &proposal{intent: Intent{Kind: "move", From: "a2", To: "a3"}, score: 5.0}
	if nextCommitted(obs, b, nil, chosenMove) == 0 {
		t.Error("nextCommitted on charged move returned 0")
	}
	// king restore action
	chosenRestore := &proposal{intent: Intent{Kind: "action", Action: "beacon_restore"}, score: 8.0}
	if nextCommitted(obs, b, nil, chosenRestore) == 0 {
		t.Error("nextCommitted on restore returned 0")
	}
	// Route aging past cap
	memRoute := map[string]int64{"committed": packCommitted(CommitKindRoute, int64(CommitAgeCap), 5.0)}
	agedRoute := nextCommitted(obs, b, memRoute, nil)
	_, ageR, _ := unpackCommitted(agedRoute)
	if ageR != int64(CommitAgeCap) {
		t.Errorf("agedRoute age = %d, want CommitAgeCap", ageR)
	}
	// King channel aging past cap
	memKing := map[string]int64{"committed": packCommitted(CommitKindKing, int64(CommitAgeCap), 5.0)}
	chosenOther := &proposal{actor: "other_pawn"}
	agedKing := nextCommitted(obs, b, memKing, chosenOther)
	_, ageK, _ := unpackCommitted(agedKing)
	if ageK != int64(CommitAgeCap) {
		t.Errorf("agedKing age = %d, want CommitAgeCap", ageK)
	}

	// holdsFocus
	if holdsFocus(b, map[string]int64{"focusFrom": 0}) {
		t.Error("holdsFocus <= 0 should be false")
	}
	bFocus := &boardContext{
		ownBySquare:            map[string]*Cell{"a1": {UnitID: UnitID{Str: "u1"}}},
		protectedChargingUnits: map[string]bool{"u1": true},
	}
	if !holdsFocus(bFocus, map[string]int64{"focusFrom": int64(squareIndex("a1") + 1)}) {
		t.Error("holdsFocus should be true when protected unit is at focus square")
	}

	// dropRefused
	proposals := []proposal{{intent: Intent{From: "a1"}}, {intent: Intent{From: "a2"}}}
	if len(dropRefused(proposals, &Observation{Revision: 2}, map[string]int64{"revision": 1})) != 2 {
		t.Error("dropRefused with revision mismatch should not filter")
	}
	refusedMem := map[string]int64{
		"revision": 2,
		"refused0": int64(squareIndex("a1")),
	}
	filtered := dropRefused(proposals, &Observation{Revision: 2}, refusedMem)
	if len(filtered) != 1 || filtered[0].intent.From != "a2" {
		t.Errorf("dropRefused filtered = %+v", filtered)
	}

	// rankBitSlot
	if rankBitSlot("superpawn") != -1 {
		t.Error("rankBitSlot unknown rank should return -1")
	}

	// packPlacementBoards & unpackPlacementBoards
	boards := [12]int64{}
	boards[0] = 1 << 0  // white pawn on a1
	boards[6] = 1 << 60 // black pawn on e8
	packed := packPlacementBoards(boards)
	unpacked := unpackPlacementBoards(packed)
	if unpacked[0] != boards[0] || unpacked[6] != boards[6] {
		t.Errorf("pack/unpack placement boards mismatch: %v vs %v", unpacked, boards)
	}

	// leaderKind
	obsBeacon := &Observation{
		Rules:  Rules{BeaconEnabled: true},
		Beacon: BeaconFact{Lifecycle: "deployed", BearerSquare: "d4"},
	}
	bearerCell := &Cell{Square: "d4", Rank: "pawn"}
	if leaderKind(obsBeacon, bearerCell) != 1 {
		t.Errorf("leaderKind bearer pawn = %d, want 1", leaderKind(obsBeacon, bearerCell))
	}
	plainCell := &Cell{Square: "e4", Rank: "pawn"}
	if leaderKind(obsBeacon, plainCell) != 0 {
		t.Errorf("leaderKind plain pawn = %d, want 0", leaderKind(obsBeacon, plainCell))
	}

	// leaderGuardKeys & clearLeaderGuard
	mGuard := map[string]int64{"leaderGuardActive": 1, "leaderBoard0": 123}
	clearLeaderGuard(mGuard)
	if len(mGuard) != 0 {
		t.Errorf("clearLeaderGuard did not clear: %v", mGuard)
	}

	// projectedCell
	obsProj := &Observation{Pieces: map[string]Cell{"a1": {Rank: "pawn"}}}
	if projectedCell(obsProj, "a1") == nil || projectedCell(obsProj, "b2") != nil {
		t.Error("projectedCell failed")
	}

	// leaderGuardMatches
	if leaderGuardMatches(obsProj, map[string]int64{"leaderGuardActive": 0}) {
		t.Error("leaderGuardMatches inactive should be false")
	}

	// activeLeaderGuard
	if activeLeaderGuard(obsProj, map[string]int64{"leaderGuardActive": 0}) != nil {
		t.Error("activeLeaderGuard inactive should be nil")
	}

	// leaderReverseForbidden
	guard := &leaderGuard{from: int64(squareIndex("a1")), to: int64(squareIndex("a2")), kind: 6}
	kingAtA2 := &Cell{Square: "a2", Rank: "king"}
	if !leaderReverseForbidden(&Observation{}, guard, kingAtA2, "a1") {
		t.Error("leaderReverseForbidden should be true for reverse king move")
	}
	if leaderReverseForbidden(&Observation{}, nil, kingAtA2, "a1") {
		t.Error("leaderReverseForbidden with nil guard should be false")
	}
	if leaderReverseForbidden(&Observation{}, guard, kingAtA2, "a3") {
		t.Error("leaderReverseForbidden with wrong destination should be false")
	}
	nonKingAtA2 := &Cell{Square: "a2", Rank: "pawn"}
	if leaderReverseForbidden(&Observation{}, guard, nonKingAtA2, "a1") {
		t.Error("leaderReverseForbidden with wrong kind should be false")
	}

	// quietLeaderIntent & quietOrdinaryIntent
	obsQuiet := &Observation{
		Side:   "white",
		Pieces: map[string]Cell{"a1": {Side: "white", Rank: "king"}, "a2": {Side: "white", Rank: "pawn"}},
	}
	if quietLeaderIntent(obsQuiet, nil) != nil || quietLeaderIntent(obsQuiet, &Intent{Kind: "action"}) != nil {
		t.Error("quietLeaderIntent nil/action should be nil")
	}
	if quietLeaderIntent(obsQuiet, &Intent{Kind: "move", From: "a1", To: "b1"}) == nil {
		t.Error("quietLeaderIntent king quiet move should return cell")
	}
	if quietOrdinaryIntent(obsQuiet, nil) != nil || quietOrdinaryIntent(obsQuiet, &Intent{Kind: "action"}) != nil {
		t.Error("quietOrdinaryIntent nil/action should be nil")
	}
	if quietOrdinaryIntent(obsQuiet, &Intent{Kind: "move", From: "a2", To: "a3"}) == nil {
		t.Error("quietOrdinaryIntent pawn quiet move should return cell")
	}

	// applyRepeatPenalty
	bPenalty := &boardContext{
		ownBySquare: map[string]*Cell{
			"a2": {UnitID: UnitID{Str: "p1"}, Square: "a2", Rank: "pawn"},
			"b2": {UnitID: UnitID{Str: "p2"}, Square: "b2", Rank: "pawn"},
		},
	}
	params := &BotParams{Advance: 1.0, PassBelow: 0.0}
	propList := []proposal{
		{intent: Intent{Kind: "move", From: "a2", To: "a3"}, score: 2.0, key: "p1-a3", actor: UnitID{Str: "p1"}},
		{intent: Intent{Kind: "move", From: "b2", To: "b3"}, score: 3.0, key: "p2-b3", actor: "p2"},
	}
	memPenalty := map[string]int64{
		"quietVacated0": int64(squareIndex("a3")),
		"lastQuietTo":   int64(squareIndex("a2")),
	}
	penalized := applyRepeatPenalty(obsQuiet, bPenalty, params, memPenalty, propList)
	if len(penalized) == 0 {
		t.Error("applyRepeatPenalty returned empty")
	}
}

func TestScoreAndSystemsEdgeCases(t *testing.T) {
	// leaderSupport
	bLeader := &boardContext{
		side: "white",
		own: []*Cell{
			{UnitID: UnitID{Str: "k"}, Square: "d1", Rank: "king"},
			{UnitID: UnitID{Str: "a1"}, Square: "d3", Rank: "pawn"},
			{UnitID: UnitID{Str: "a2"}, Square: "e2", Rank: "pawn"},
			{UnitID: UnitID{Str: "a3"}, Square: "d1", Rank: "pawn"},
		},
	}
	sup := leaderSupport(&Observation{}, bLeader, bLeader.own[0], "d2")
	if sup <= 0 {
		t.Errorf("leaderSupport = %f, want > 0", sup)
	}

	// priorityCaptiveDeliveryProposal & in flight
	obsPrisoner := &Observation{
		Side: "white",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
			CargoBasedDelivery: true,
		},
		Legal: map[string][]string{"e2": {"e1"}},
		Candidates: map[string]map[string]CandidateFact{
			"e2": {"e1": {DestinationVisible: true}},
		},
		Pieces: map[string]Cell{
			"e2": {
				UnitID:     UnitID{Str: "c1"},
				Side:       "white",
				Rank:       "pawn",
				Convoy:     true,
				CargoCount: 1,
			},
		},
	}
	bPrisoner := buildBoard(obsPrisoner)
	prioProp := priorityCaptiveDeliveryProposal(obsPrisoner, bPrisoner, &BotParams{})
	if prioProp == nil {
		t.Error("expected priority captive delivery proposal")
	}

	// captive in flight
	obsInFlight := &Observation{
		Side: "white",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
			CargoBasedDelivery: true,
		},
		Pieces: map[string]Cell{
			"e2": {
				UnitID:     UnitID{Str: "c1"},
				Side:       "white",
				Rank:       "pawn",
				Convoy:     true,
				CargoCount: 1,
				Charging:   &ChargingFact{Square: "e1", RemainingMs: 100},
			},
		},
	}
	bInFlight := buildBoard(obsInFlight)
	if !priorityCaptiveDeliveryInFlight(obsInFlight, bInFlight) {
		t.Error("expected priorityCaptiveDeliveryInFlight to be true")
	}

	// rankOptions
	params := &BotParams{PassBelow: 0.0}
	if rankOptions([]proposal{{score: 1.0}}, params, 0) != nil {
		t.Error("rankOptions with count 0 should return nil")
	}
	props := []proposal{
		{score: 5.0, actor: UnitID{Str: "u1"}, key: "k1"},
		{score: 4.0, actor: UnitID{Str: "u1"}, key: "k2"}, // duplicate actor
		{score: 3.0, actor: "string_actor", key: "k3"},
		{score: 2.0, actor: nil, key: "k4"},
		{score: -1.0, actor: "low", key: "k5"}, // below PassBelow
	}
	opts := rankOptions(props, params, 3)
	if len(opts) != 3 {
		t.Errorf("rankOptions len = %d, want 3", len(opts))
	}

	// systems: beaconDeployOrRestoreProposal
	obsBeaconSys := &Observation{
		Rules:  Rules{BeaconEnabled: true},
		Beacon: BeaconFact{Lifecycle: "lost"},
	}
	bKing := &boardContext{kingCell: &Cell{UnitID: UnitID{Str: "k1"}, Square: "e1"}}
	restoreProp := beaconDeployOrRestoreProposal(obsBeaconSys, bKing, &BotParams{BeaconAggression: 1.0})
	if restoreProp == nil || restoreProp.intent.Action != "beacon_restore" {
		t.Errorf("expected restore proposal, got %+v", restoreProp)
	}
	obsBeaconSys.Beacon.Lifecycle = "undeployed"
	deployProp := beaconDeployOrRestoreProposal(obsBeaconSys, bKing, &BotParams{BeaconAggression: 1.0})
	if deployProp == nil || deployProp.intent.Action != "beacon_deploy" {
		t.Errorf("expected deploy proposal, got %+v", deployProp)
	}

	// beaconForgeProposals
	obsForge := &Observation{
		Rules: Rules{
			BeaconEnabled:      true,
			BeaconForgeEnabled: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
		},
		Beacon: BeaconFact{Lifecycle: "lost"},
	}
	bForge := &boardContext{
		actionableUnits: []*Cell{{UnitID: UnitID{Str: "p1"}, Square: "e1", Rank: "pawn"}},
	}
	forgeProps := beaconForgeProposals(obsForge, bForge, &BotParams{BeaconAggression: 1.0})
	if len(forgeProps) != 1 {
		t.Errorf("expected 1 forge proposal, got %d", len(forgeProps))
	}

	// espionageProposals
	obsEsp := &Observation{EnemyManaged: 1}
	bEsp := &boardContext{
		kingCell:        &Cell{UnitID: UnitID{Str: "k1"}, Square: "e1"},
		actionableUnits: []*Cell{{UnitID: UnitID{Str: "suspect"}, Square: "e2", Rank: "pawn"}},
	}
	espProps := espionageProposals(obsEsp, bEsp, &BotParams{System: 1.0})
	if len(espProps) != 1 || espProps[0].intent.Action != "interrogate" {
		t.Errorf("expected interrogate proposal, got %+v", espProps)
	}

	// beaconHandOffProposal
	obsHandOff := &Observation{
		Side: "white",
		Rules: Rules{
			BeaconEnabled:            true,
			BeaconKingStartsAsBearer: true,
		},
		Beacon: BeaconFact{
			Lifecycle:    "deployed",
			BearerSquare: "e1",
		},
	}
	bHandOff := &boardContext{
		side:     "white",
		kingCell: &Cell{UnitID: UnitID{Str: "k1"}, Square: "e1"},
		allBySquare: map[string]*Cell{
			"e2": {UnitID: UnitID{Str: "p1"}, Square: "e2", Side: "white", Rank: "pawn"},
		},
		actionableUnits: []*Cell{
			{UnitID: UnitID{Str: "p1"}, Square: "e2", Side: "white", Rank: "pawn"},
		},
	}
	handOffProp := beaconHandOffProposal(obsHandOff, bHandOff, &BotParams{BeaconAggression: 1.0})
	if handOffProp == nil || handOffProp.intent.Action != "beacon_take" {
		t.Errorf("expected beacon_take proposal, got %+v", handOffProp)
	}
}

func TestDecideAndDecideJSONEdgeCases(t *testing.T) {
	// Decide with lifecycle != "playing"
	obsNotPlaying := &Observation{Lifecycle: "ended"}
	intent, _, ranked := Decide(obsNotPlaying, nil, &BotParams{}, 0, 1)
	if intent != nil || ranked != nil {
		t.Error("Decide on ended lifecycle should return nil")
	}

	// Decide with no own units
	obsNoUnits := &Observation{Lifecycle: "playing", Side: "white"}
	intent, _, ranked = Decide(obsNoUnits, nil, &BotParams{}, 0, 1)
	if intent != nil || ranked != nil {
		t.Error("Decide with no own units should return nil")
	}

	// Decide with protected charging units
	obsCharging := &Observation{
		Lifecycle: "playing",
		Side:      "white",
		Pieces: map[string]Cell{
			"a1": {UnitID: UnitID{Str: "k1"}, Side: "white", Rank: "king"},
			"a2": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn", Charging: &ChargingFact{Square: "e8", RemainingMs: 100}},
			"e8": {UnitID: UnitID{Str: "ek"}, Side: "black", Rank: "king"},
		},
	}
	intent, _, _ = Decide(obsCharging, nil, &BotParams{}, 0, 1)
	if intent != nil {
		t.Error("Decide with protected charging unit should return nil intent")
	}

	// Decide with pass below
	obsPass := &Observation{
		Lifecycle: "playing",
		Side:      "white",
		Pieces: map[string]Cell{
			"a1": {UnitID: UnitID{Str: "k1"}, Side: "white", Rank: "king"},
		},
	}
	intent, _, _ = Decide(obsPass, nil, &BotParams{PassBelow: 999.0}, 0, 1)
	if intent != nil {
		t.Error("Decide with PassBelow 999 should return nil intent")
	}

	// DecideJSON validation errors
	if _, _, _, err := DecideJSON([]byte("{bad"), nil, nil, 0, 0); err == nil {
		t.Error("DecideJSON with bad obs JSON succeeded, want error")
	}
	if _, _, _, err := DecideJSON([]byte("{}"), []byte("{bad"), nil, 0, 0); err == nil {
		t.Error("DecideJSON with bad mem JSON succeeded, want error")
	}
	if _, _, _, err := DecideJSON([]byte("{}"), []byte("null"), []byte("{bad"), 0, 0); err == nil {
		t.Error("DecideJSON with bad params JSON succeeded, want error")
	}

	// DecideJSON Marshal errors
	origMarshal := jsonMarshal
	defer func() { jsonMarshal = origMarshal }()

	validObs, _ := json.Marshal(&Observation{Lifecycle: "ended"})
	validParams, _ := json.Marshal(&BotParams{})

	// Error on mem marshal
	jsonMarshal = func(v any) ([]byte, error) {
		if _, ok := v.(map[string]int64); ok {
			return nil, errors.New("mem marshal error")
		}
		return json.Marshal(v)
	}
	if _, _, _, err := DecideJSON(validObs, nil, validParams, 0, 0); err == nil {
		t.Error("DecideJSON with mem marshal error succeeded, want error")
	}

	obsPlaying := &Observation{
		Lifecycle: "playing",
		Side:      "white",
		Rules: Rules{
			BeaconEnabled: true,
		},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "k1"}, Side: "white", Rank: "king"},
		},
		Beacon: BeaconFact{Lifecycle: "undeployed"},
	}
	validObsPlaying, _ := json.Marshal(obsPlaying)
	paramsBeacon, _ := json.Marshal(&BotParams{BeaconAggression: 1.0, PassBelow: -100.0})

	// Error on ranked marshal
	jsonMarshal = func(v any) ([]byte, error) {
		if _, ok := v.([]Option); ok {
			return nil, errors.New("ranked marshal error")
		}
		return json.Marshal(v)
	}
	if _, _, _, err := DecideJSON(validObsPlaying, nil, paramsBeacon, 0, 1); err == nil {
		t.Error("DecideJSON with ranked marshal error succeeded, want error")
	}

	// Error on intent marshal
	jsonMarshal = func(v any) ([]byte, error) {
		if _, ok := v.(*Intent); ok {
			return nil, errors.New("intent marshal error")
		}
		return json.Marshal(v)
	}
	if _, _, _, err := DecideJSON(validObsPlaying, nil, paramsBeacon, 0, 1); err == nil {
		t.Error("DecideJSON with intent marshal error succeeded, want error")
	}
}

func TestResolvedParamsFailures(t *testing.T) {
	// Bad Params
	origParams := Params
	defer func() { Params = origParams }()

	Params = []byte("{invalid")
	if _, err := paramsSetNames(); err == nil {
		t.Error("paramsSetNames with invalid Params succeeded, want error")
	}
	if _, err := MarshalResolvedParams(); err == nil {
		t.Error("MarshalResolvedParams with invalid Params succeeded, want error")
	}

	// ResolveParams unknown name
	Params = origParams
	if _, err := ResolveParams("nonexistent_tier_xyz"); err == nil {
		t.Error("ResolveParams with nonexistent tier succeeded, want error")
	}

	// MarshalResolvedParams with unknown set
	Params = []byte(`{"sets": {"unknown_xyz_set": {}}}`)
	if _, err := MarshalResolvedParams(); err == nil {
		t.Error("MarshalResolvedParams with unknown set should return error")
	}
}

func TestLeaderGuardMatchesFull(t *testing.T) {
	fromIdx := int64(squareIndex("a1"))
	toIdx := int64(squareIndex("a2"))
	kind := int64(6)

	obsWhite := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"a1": {Square: "a1", Side: "white", Rank: "king"},
		},
	}
	pre := placementBitboards(obsWhite, "", "")
	pre[5] ^= (1 << fromIdx) | (1 << toIdx)
	mem := packPlacementBoards(pre)
	mem["leaderGuardActive"] = 1
	mem["leaderGuardFrom"] = fromIdx
	mem["leaderGuardTo"] = toIdx
	mem["leaderGuardKind"] = kind

	if !leaderGuardMatches(obsWhite, mem) {
		t.Error("leaderGuardMatches white sourceCell should match")
	}

	// Black sourceCell
	obsBlack := &Observation{
		Side: "black",
		Pieces: map[string]Cell{
			"a1": {Square: "a1", Side: "black", Rank: "king"},
		},
	}
	preB := placementBitboards(obsBlack, "", "")
	preB[11] ^= (1 << fromIdx) | (1 << toIdx)
	memB := packPlacementBoards(preB)
	memB["leaderGuardActive"] = 1
	memB["leaderGuardFrom"] = fromIdx
	memB["leaderGuardTo"] = toIdx
	memB["leaderGuardKind"] = kind
	if !leaderGuardMatches(obsBlack, memB) {
		t.Error("leaderGuardMatches black sourceCell should match")
	}

	// sourceCell.Side != obs.Side -> becomes nil, checks destinationCell
	obsSourceEnemy := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"a1": {Square: "a1", Side: "black", Rank: "king"},
			"a2": {Square: "a2", Side: "white", Rank: "king"},
		},
	}
	memDest := packPlacementBoards(placementBitboards(obsSourceEnemy, "", ""))
	memDest["leaderGuardActive"] = 1
	memDest["leaderGuardFrom"] = fromIdx
	memDest["leaderGuardTo"] = toIdx
	memDest["leaderGuardKind"] = kind
	if !leaderGuardMatches(obsSourceEnemy, memDest) {
		t.Error("leaderGuardMatches destinationCell should match")
	}

	// destinationCell.Side != obs.Side
	obsDestEnemy := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"a1": {Square: "a1", Side: "black", Rank: "king"},
			"a2": {Square: "a2", Side: "black", Rank: "king"},
		},
	}
	if leaderGuardMatches(obsDestEnemy, memDest) {
		t.Error("leaderGuardMatches destinationCell enemy should not match")
	}

	// destinationCell == nil
	obsDestNil := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"a1": {Square: "a1", Side: "black", Rank: "king"},
		},
	}
	if leaderGuardMatches(obsDestNil, memDest) {
		t.Error("leaderGuardMatches destinationCell nil should not match")
	}

	// activeLeaderGuard success
	if guard := activeLeaderGuard(obsWhite, mem); guard == nil {
		t.Error("expected non-nil guard")
	}

	// quietLeaderIntent and quietOrdinaryIntent edges
	obsEnemyFrom := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"a1": {Side: "black", Rank: "king"},
			"a2": {Side: "black", Rank: "pawn"},
			"b1": {Side: "black", Rank: "pawn"},
		},
	}
	if quietLeaderIntent(obsEnemyFrom, &Intent{Kind: "move", From: "a1", To: "b1"}) != nil {
		t.Error("quietLeaderIntent with enemy piece should return nil")
	}
	if quietOrdinaryIntent(obsEnemyFrom, &Intent{Kind: "move", From: "a2", To: "b1"}) != nil {
		t.Error("quietOrdinaryIntent with enemy piece should return nil")
	}
	obsNotQuiet := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"a1": {Side: "white", Rank: "king"},
			"a2": {Side: "white", Rank: "pawn"},
			"b1": {Side: "black", Rank: "rook"},
		},
	}
	if quietLeaderIntent(obsNotQuiet, &Intent{Kind: "move", From: "a1", To: "b1"}) != nil {
		t.Error("quietLeaderIntent into enemy piece should return nil")
	}
	if quietOrdinaryIntent(obsNotQuiet, &Intent{Kind: "move", From: "a2", To: "b1"}) != nil {
		t.Error("quietOrdinaryIntent into enemy piece should return nil")
	}
}

func TestDecideFilteringAndTies(t *testing.T) {
	// kingChannelActive with proposals
	obsKingChannel := &Observation{
		Side:      "white",
		Lifecycle: "playing",
		Rules: Rules{
			MaxActiveCommands: 5,
		},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "k1"}, Side: "white", Rank: "king"},
			"e2": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn"},
		},
		Beacon: BeaconFact{Lifecycle: "restoring"},
		Legal: map[string][]string{
			"e1": {"f1"},
			"e2": {"e3"},
		},
	}
	memKing := map[string]int64{
		"committed": packCommitted(CommitKindKing, 0, 50.0),
	}
	Decide(obsKingChannel, memKing, &BotParams{PassBelow: -100.0}, 0, 2)

	// Charging units filtering in Decide
	obsCharging := &Observation{
		Side:      "white",
		Lifecycle: "playing",
		Rules: Rules{
			MaxActiveCommands: 5,
		},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "k1"}, Side: "white", Rank: "king"},
			"e2": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn", Charging: &ChargingFact{Square: "e3", RemainingMs: 1000}},
		},
		Legal: map[string][]string{
			"e1": {"f1"},
			"e2": {"e4"},
		},
	}
	memRoute := map[string]int64{
		"committed": packCommitted(CommitKindRoute, 0, 50.0),
	}
	Decide(obsCharging, memRoute, &BotParams{PassBelow: -100.0}, 0, 2)

	// Tied proposals
	p1 := proposal{score: 10.0, key: "b_key", actor: UnitID{Str: "u1"}}
	p2 := proposal{score: 10.0, key: "a_key", actor: UnitID{Str: "u2"}}
	proposals := []proposal{p1, p2}
	ranked := rankOptions(proposals, &BotParams{PassBelow: 0.0}, 2)
	if len(ranked) != 2 {
		t.Errorf("expected 2 ranked options, got %d", len(ranked))
	}
}

func TestScoreCompleteRemaining(t *testing.T) {
	// outcomesAt nil
	if outcomesAt(&Observation{}, "a1", "a2") != nil {
		t.Error("outcomesAt empty observation should be nil")
	}

	// captureChoiceAvailable branches
	obsCapture := &Observation{
		Rules: Rules{AllowsKill: false, AllowsCapture: true},
		Affordability: map[string]map[string]DestinationOutcomes{
			"a1": {
				"b2": {Capture: &CaptureOutcome{Affordable: true}},
				"c3": {Capture: &CaptureOutcome{Affordable: false}},
			},
		},
	}
	bCapture := &boardContext{
		enemyBySquare: map[string]*Cell{
			"b2": {Square: "b2", Rank: "king"},
			"c3": {Square: "c3", Rank: "king"},
			"d4": {Square: "d4", Rank: "pawn"},
		},
	}
	pawnA1 := &Cell{Square: "a1", Rank: "pawn"}
	if !captureChoiceAvailable(obsCapture, bCapture, pawnA1, "b2") {
		t.Error("captureChoiceAvailable affordable capture king should be true")
	}
	if captureChoiceAvailable(obsCapture, bCapture, pawnA1, "c3") {
		t.Error("captureChoiceAvailable unaffordable capture king should be false")
	}
	if captureChoiceAvailable(obsCapture, bCapture, pawnA1, "d4") {
		t.Error("captureChoiceAvailable no outcomes pawn should be false")
	}

	// currentMoraleNeed with CaptureMoraleNeed != nil
	need := 7
	if currentMoraleNeed(&Observation{CaptureMoraleNeed: &need}) != 7 {
		t.Error("currentMoraleNeed should return *CaptureMoraleNeed")
	}

	// postMoveMorale < 0
	obsPenalty := &Observation{Side: "white", OwnMoralePenalty: 10}
	if postMoveMorale(obsPenalty, &boardContext{side: "white"}, "a1", "a2") != 0 {
		t.Error("postMoveMorale with large penalty should return 0")
	}

	// canPromoteNextMove
	if canPromoteNextMove("white", nil) {
		t.Error("canPromoteNextMove(nil) should be false")
	}
	candProm := &CandidateFact{NextPossibleMoves: []string{"a8"}}
	if !canPromoteNextMove("white", candProm) {
		t.Error("canPromoteNextMove with promotion destination should be true")
	}

	// leaderSupport res > 1.0 and dist == 0
	bManyAllies := &boardContext{
		side: "white",
		own: []*Cell{
			{UnitID: UnitID{Str: "k"}, Square: "d1", Rank: "king"},
			{UnitID: UnitID{Str: "a1"}, Square: "d2", Rank: "pawn"},
			{UnitID: UnitID{Str: "a2"}, Square: "d2", Rank: "pawn"},
			{UnitID: UnitID{Str: "a3"}, Square: "e3", Rank: "pawn"},
			{UnitID: UnitID{Str: "a4"}, Square: "c3", Rank: "pawn"},
			{UnitID: UnitID{Str: "a5"}, Square: "e2", Rank: "pawn"},
		},
	}
	_ = leaderSupport(&Observation{}, bManyAllies, bManyAllies.own[0], "d2")

	// formationLeaderPriority
	obsLeader := &Observation{
		Side:      "white",
		OwnMorale: 10,
		Legal: map[string][]string{
			"d1": {"d2"},
			"d4": {"d5"},
		},
		Candidates: map[string]map[string]CandidateFact{
			"d1": {"d2": {DestinationVisible: true, GuardedBy: []string{"c1"}}},
			"d4": {"d5": {DestinationVisible: true, GuardedBy: []string{"c4"}}},
		},
		Beacon: BeaconFact{Lifecycle: "deployed", BearerSquare: "d4"},
	}
	bLeaderCtx := &boardContext{
		side: "white",
		own: []*Cell{
			{UnitID: UnitID{Str: "k"}, Square: "d1", Rank: "king"},
			{UnitID: UnitID{Str: "b"}, Square: "d4", Rank: "pawn"},
			{UnitID: UnitID{Str: "g"}, Square: "c4", Rank: "pawn"},
			{UnitID: UnitID{Str: "g2"}, Square: "c1", Rank: "pawn"},
		},
		ownBySquare: map[string]*Cell{
			"d1": {UnitID: UnitID{Str: "k"}, Square: "d1", Rank: "king"},
			"d4": {UnitID: UnitID{Str: "b"}, Square: "d4", Rank: "pawn"},
			"c4": {UnitID: UnitID{Str: "g"}, Square: "c4", Rank: "pawn"},
			"c1": {UnitID: UnitID{Str: "g2"}, Square: "c1", Rank: "pawn"},
		},
	}
	kingCell := bLeaderCtx.own[0]
	bearerCell := bLeaderCtx.own[1]
	pLeader := &BotParams{MoralePush: 1.0, BeaconAggression: 1.0}
	formationLeaderPriority(obsLeader, bLeaderCtx, pLeader, kingCell)
	formationLeaderPriority(obsLeader, bLeaderCtx, pLeader, bearerCell)

	// unitPriority direct king capture
	obsUnitPrio := &Observation{
		Legal: map[string][]string{
			"e2": {"e8"},
		},
		Rules: Rules{AllowsKill: true},
		Affordability: map[string]map[string]DestinationOutcomes{
			"e2": {"e8": {Kill: &CaptureOutcome{Affordable: true}}},
		},
	}
	bUnitPrio := &boardContext{
		enemy:         []*Cell{{Square: "e8", Rank: "king"}},
		enemyBySquare: map[string]*Cell{"e8": {Square: "e8", Rank: "king"}},
	}
	unitPriority(obsUnitPrio, bUnitPrio, &BotParams{}, &Cell{Square: "e2", Rank: "pawn"})

	// scoreMove rich scenarios
	obsRich := &Observation{
		Side: "white",
		Rules: Rules{
			PieceChargeMs:      map[string]int{"pawn": 5000, "queen": 5000},
			CargoBasedDelivery: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
			AllowsKill:         true,
			AllowsCapture:      true,
		},
		Legal: map[string][]string{
			"e2": {"e3", "e8"},
		},
		Candidates: map[string]map[string]CandidateFact{
			"e2": {
				"e3": {
					DestinationVisible: true,
					GuardedBy:          []string{"d2"},
					NextPossibleMoves:  []string{"e8"},
					PatrolGain:         100, // tests PatrolGainCap
				},
			},
		},
		Affordability: map[string]map[string]DestinationOutcomes{
			"e2": {
				"e8": {Kill: &CaptureOutcome{Affordable: true}},
			},
		},
		Pieces: map[string]Cell{
			"e8": {Square: "e8", Side: "black", Rank: "king"},
		},
	}
	bRich := buildBoard(obsRich)
	bRich.needsFirstMasterEngineer = true
	bRich.blockingBase = []string{"e2"}
	bRich.deliverySquares = []string{"e8"}
	bRich.convoyHome = map[string]int{"e2": 1, "e3": 2} // offRoute

	paramsRich := &BotParams{
		Material:         1.0,
		Safety:           1.0,
		Tempo:            1.0,
		Advance:          1.0,
		Delivery:         1.0,
		Prisoner:         1.0,
		TargetLock:       1.0,
		KingSafety:       1.0,
		MoralePush:       1.0,
		BeaconAggression: 1.0,
	}

	pawnCell := &Cell{
		UnitID:     UnitID{Str: "p"},
		Square:     "e2",
		Rank:       "pawn",
		Convoy:     true,
		KingCargo:  true,
		CargoCount: 1,
		Charging:   &ChargingFact{Square: "e1", RemainingMs: 500},
	}
	// Delivery offRoute & closer
	scoreMove(obsRich, bRich, paramsRich, nil, pawnCell, "e3")
	bRich.convoyHome["e3"] = 0 // closer
	scoreMove(obsRich, bRich, paramsRich, nil, pawnCell, "e3")
	// Delivery wins
	scoreMove(obsRich, bRich, paramsRich, nil, pawnCell, "e8")

	// Pawn promotion move
	pawnProm := &Cell{UnitID: UnitID{Str: "p2"}, Square: "e7", Rank: "pawn"}
	scoreMove(obsRich, bRich, paramsRich, nil, pawnProm, "e8")

	// King escape & guard & morale push
	bRich.kingThreatened = true
	bRich.kingCell = &Cell{Square: "e1", Rank: "king", UnitID: UnitID{Str: "k"}}
	bRich.lockedUnits["k"] = true
	scoreMove(obsRich, bRich, paramsRich, nil, bRich.kingCell, "e2")

	// priorityCaptiveDeliveryProposal remaining edge cases
	obsBasePrio := &Observation{
		Side: "white",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1", "e2"}},
		},
		Legal: map[string][]string{
			"e1": {"e2"},
		},
		Candidates: map[string]map[string]CandidateFact{
			"e1": {"e2": {DestinationVisible: true}},
		},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "c1"}, Side: "white", Rank: "pawn", Convoy: true, CargoCount: 1},
		},
	}
	bBasePrio := buildBoard(obsBasePrio)
	priorityCaptiveDeliveryProposal(obsBasePrio, bBasePrio, &BotParams{})
}

func TestSystemsCompleteRemaining(t *testing.T) {
	// mostAdvancedAdjacentAlly with kingCell nil
	if mostAdvancedAdjacentAlly(&boardContext{}) != "" {
		t.Error("mostAdvancedAdjacentAlly should return empty string when kingCell is nil")
	}

	// beaconHandOffProposal edge cases
	obsNoBeacon := &Observation{Rules: Rules{BeaconEnabled: false}}
	if beaconHandOffProposal(obsNoBeacon, &boardContext{}, &BotParams{}) != nil {
		t.Error("beaconHandOffProposal with beacon disabled should return nil")
	}

	// trainingProposals with specialist ceiling reached and profession sergeant
	obsTrain := &Observation{
		Side: "white",
		Rules: Rules{
			SpecialistCeiling:    1,
			PermittedProfessions: []string{"sergeant", "engineer"},
			BaseSquares:          map[string][]string{"pawn": {"e1"}},
		},
		OwnSpecialists: 1, // ceiling reached
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn"},
		},
	}
	bTrain := buildBoard(obsTrain)
	trainingProposals(obsTrain, bTrain, &BotParams{System: 1.0})

	// trainingProposals advanced training
	obsAdvTrain := &Observation{
		Side: "white",
		Rules: Rules{
			BaseSquares: map[string][]string{"pawn": {"e1"}},
		},
		Pieces: map[string]Cell{
			"e1": {
				UnitID:      UnitID{Str: "e"},
				Side:        "white",
				Rank:        "pawn",
				Profession:  "engineer",
				Grade:       "apprentice",
				EligibleFor: []string{"masterEngineerTraining"},
			},
		},
	}
	bAdvTrain := buildBoard(obsAdvTrain)
	advProps := trainingProposals(obsAdvTrain, bAdvTrain, &BotParams{System: 1.0, AdvancedTraining: true})
	if len(advProps) == 0 {
		t.Error("expected advanced training proposal")
	}

	// wallProposals directionFromB, sergeantSupported
	obsWall := &Observation{
		Side: "white",
		Walls: []WallFact{
			{
				Edge:             "e2_e3",
				A:                "e2",
				B:                "e3",
				DirectionFromB:   "south",
				Side:             "white",
				MaximumIntegrity: 100,
				Integrity:        10,
			},
			{
				Edge:             "e3_e4",
				A:                "e3",
				B:                "e4",
				DirectionFromA:   "north",
				Side:             "black",
				MaximumIntegrity: 100,
				Integrity:        10,
			},
		},
		Pieces: map[string]Cell{
			"e3": {UnitID: UnitID{Str: "eng"}, Side: "white", Rank: "pawn", Profession: "engineer"},
			"d3": {UnitID: UnitID{Str: "sgt"}, Side: "white", Rank: "pawn", Profession: "sergeant"},
		},
	}
	bWall := buildBoard(obsWall)
	wallProps := wallProposals(obsWall, bWall, &BotParams{System: 1.0, ContestEnemyWork: true, SergeantPreference: 1.0})
	if len(wallProps) < 2 {
		t.Errorf("expected at least 2 wall proposals, got %d", len(wallProps))
	}

	// systemProposals with system <= 0 and beacon disabled
	if systemProposals(&Observation{Rules: Rules{BeaconEnabled: false}}, &boardContext{}, &BotParams{System: 0}) != nil {
		t.Error("systemProposals should return nil when no systems active")
	}
}

func TestMoveProposalsComplete(t *testing.T) {
	// moveProposals when moveActionableUnits is empty
	bEmpty := &boardContext{}
	if moveProposals(&Observation{}, bEmpty, &BotParams{}, nil) != nil {
		t.Error("moveProposals on empty actionable units should return nil")
	}

	// moveProposals filtering: self destination, charging destination, etc.
	obsMove := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"e2": {
				UnitID:   UnitID{Str: "p1"},
				Side:     "white",
				Rank:     "pawn",
				Charging: &ChargingFact{Square: "e3", RemainingMs: 1000},
			},
		},
		Legal: map[string][]string{
			"e2": {"e2", "e3", "e4"},
		},
	}
	bMove := buildBoard(obsMove)
	bMove.moveActionableUnits = []*Cell{bMove.ownBySquare["e2"]}
	props := moveProposals(obsMove, bMove, &BotParams{Breadth: 1, CandidateSpread: 1}, nil)
	if len(props) == 0 {
		t.Error("expected proposals after filtering self and charging destinations")
	}
}

func TestRemainingEdges100Percent(t *testing.T) {
	// bot.go lines 41, 47: Decide with priorityCaptiveDeliveryInFlight and priority captive delivery proposal
	obsInFlight := &Observation{
		Side:      "white",
		Lifecycle: "playing",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
			CargoBasedDelivery: true,
		},
		Pieces: map[string]Cell{
			"e2": {
				UnitID:     UnitID{Str: "c1"},
				Side:       "white",
				Rank:       "pawn",
				Convoy:     true,
				CargoCount: 1,
				Charging:   &ChargingFact{Square: "e1", RemainingMs: 100},
			},
		},
	}
	intent, _, _ := Decide(obsInFlight, nil, &BotParams{}, 0, 1)
	if intent != nil {
		t.Error("Decide with priority in flight should return nil intent")
	}

	obsPrisoner := &Observation{
		Side:      "white",
		Lifecycle: "playing",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
			CargoBasedDelivery: true,
		},
		Legal: map[string][]string{"e2": {"e1"}},
		Candidates: map[string]map[string]CandidateFact{
			"e2": {"e1": {DestinationVisible: true}},
		},
		Pieces: map[string]Cell{
			"e2": {
				UnitID:     UnitID{Str: "c1"},
				Side:       "white",
				Rank:       "pawn",
				Convoy:     true,
				CargoCount: 1,
			},
		},
	}
	intent, _, ranked := Decide(obsPrisoner, nil, &BotParams{}, 0, 1)
	if intent == nil || len(ranked) == 0 {
		t.Error("Decide with priority captive delivery should return chosen intent and ranked")
	}

	// bot.go string actor in kingChannel filtering and chargingUnits filtering
	decideExtraProposals = []proposal{
		{actor: "k1", score: -100.0, intent: Intent{From: "e1"}},
		{actor: "chargingStr", score: -100.0, intent: Intent{From: "e1"}},
	}
	defer func() { decideExtraProposals = nil }()

	obsKingDecide := &Observation{
		Side:      "white",
		Lifecycle: "playing",
		Rules:     Rules{MaxActiveCommands: 5},
		Beacon:    BeaconFact{Lifecycle: "restoring"},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "k1"}, Side: "white", Rank: "king"},
		},
		Legal: map[string][]string{"e1": {"e2"}},
	}
	Decide(obsKingDecide, map[string]int64{"committed": packCommitted(CommitKindKing, 0, 50.0)}, &BotParams{PassBelow: -50.0}, 0, 1)

	obsChargingDecide := &Observation{
		Side:      "white",
		Lifecycle: "playing",
		Rules:     Rules{MaxActiveCommands: 5},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "k1"}, Side: "white", Rank: "king"},
			"e2": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn", Charging: &ChargingFact{Square: "e3", RemainingMs: 1000}},
		},
		Legal: map[string][]string{"e1": {"f1"}},
	}
	Decide(obsChargingDecide, map[string]int64{"committed": packCommitted(CommitKindRoute, 0, 50.0)}, &BotParams{PassBelow: -50.0}, 0, 1)

	// bot.go: Decide bestProposal.score < params.PassBelow with options > 0
	obsPassBelow := &Observation{
		Side:      "white",
		Lifecycle: "playing",
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "k1"}, Side: "white", Rank: "king"},
		},
		Legal: map[string][]string{"e1": {"e2"}},
	}
	Decide(obsPassBelow, nil, &BotParams{PassBelow: 999.0}, 0, 1)

	// memory.go: placementBitboards with invalid rank
	obsInvalidRank := &Observation{
		Side:   "white",
		Pieces: map[string]Cell{"a1": {Rank: "alien"}},
	}
	placementBitboards(obsInvalidRank, "", "")

	// memory.go: buildMemory with leaderGuardActive == 1 and non-matching guard
	memActiveGuard := map[string]int64{
		"leaderGuardActive": 1,
		"leaderGuardFrom":   0,
		"leaderGuardTo":     1,
		"leaderGuardKind":   6,
	}
	bm := buildMemory(&Observation{}, memActiveGuard, nil)
	if bm["leaderGuardActive"] == 1 {
		t.Error("buildMemory should clear non-matching leader guard")
	}

	// memory.go: buildMemory with revision == obs.Revision
	memFrozen := map[string]int64{
		"revision":      5,
		"refusedCursor": 2,
	}
	bmFrozen := buildMemory(&Observation{Revision: 5}, memFrozen, nil)
	if bmFrozen["refusedCursor"] != 3 {
		t.Errorf("refusedCursor = %d, want 3", bmFrozen["refusedCursor"])
	}

	// memory.go: applyRepeatPenalty edge cases
	bRep := &boardContext{
		ownBySquare: map[string]*Cell{
			"e1": {UnitID: UnitID{Str: "k"}, Square: "e1", Rank: "king"},
			"e2": {UnitID: UnitID{Str: "p"}, Square: "e2", Rank: "pawn"},
			"e3": {UnitID: UnitID{Str: "p2"}, Square: "e3", Rank: "pawn"},
		},
	}
	// repeatedCell is king
	memRepKing := map[string]int64{"lastQuietTo": int64(squareIndex("e1"))}
	applyRepeatPenalty(&Observation{}, bRep, &BotParams{}, memRepKing, nil)

	// not otherViable (only repeatedCell has proposal)
	memRepPawn := map[string]int64{"lastQuietTo": int64(squareIndex("e2"))}
	propsSingle := []proposal{{actor: UnitID{Str: "p"}, intent: Intent{Kind: "move", From: "e2", To: "e3"}}}
	applyRepeatPenalty(&Observation{}, bRep, &BotParams{PassBelow: 0}, memRepPawn, propsSingle)

	// string actor and repeat penalty applied
	propsRep := []proposal{
		{actor: "p", intent: Intent{Kind: "move", From: "e2", To: "d2"}, score: 5.0},
		{actor: UnitID{Str: "p2"}, intent: Intent{Kind: "move", From: "e3", To: "e4"}, score: 5.0},
	}
	pen := applyRepeatPenalty(&Observation{Pieces: map[string]Cell{"e2": {Side: "white", Rank: "pawn"}}}, bRep, &BotParams{Advance: 1.0, PassBelow: 0}, memRepPawn, propsRep)
	if len(pen) != 2 {
		t.Errorf("len(pen) = %d, want 2", len(pen))
	}

	// memory.go: moveProposals filtering captureChoiceAvailable and leaderReverseForbidden
	fromIdx := int64(squareIndex("a1"))
	toIdx := int64(squareIndex("a2"))
	obsGuardRev := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"a2": {Square: "a2", Side: "white", Rank: "king", UnitID: UnitID{Str: "k"}},
			"b3": {Square: "b3", Side: "black", Rank: "king"},
		},
		Legal: map[string][]string{
			"a2": {"b3", "a1"},
		},
		Affordability: map[string]map[string]DestinationOutcomes{
			"a2": {"b3": {}}, // capture not affordable
		},
	}
	memGuardRev := packPlacementBoards(placementBitboards(obsGuardRev, "", ""))
	memGuardRev["leaderGuardActive"] = 1
	memGuardRev["leaderGuardFrom"] = fromIdx
	memGuardRev["leaderGuardTo"] = toIdx
	memGuardRev["leaderGuardKind"] = 6
	bGuardRev := buildBoard(obsGuardRev)
	moveProposals(obsGuardRev, bGuardRev, &BotParams{}, memGuardRev)

	// score.go: captureChoiceAvailable AllowsCapture true branch 4
	obsCapOnly := &Observation{
		Rules: Rules{AllowsKill: false, AllowsCapture: true},
		Affordability: map[string]map[string]DestinationOutcomes{
			"a2": {"b3": {Capture: &CaptureOutcome{Affordable: true}}},
		},
	}
	bCapOnly := &boardContext{
		enemyBySquare: map[string]*Cell{"b3": {Square: "b3", Rank: "pawn"}},
	}
	if !captureChoiceAvailable(obsCapOnly, bCapOnly, &Cell{Square: "a2", Rank: "pawn"}, "b3") {
		t.Error("captureChoiceAvailable AllowsCapture true should be true")
	}

	// score.go: formationLeaderPriority Branch 1: king with excess morale and after < current && after >= needed+1
	obsKingExcess := &Observation{
		Side:      "white",
		OwnMorale: 10,
		Legal:     map[string][]string{"e4": {"e3"}},
		Candidates: map[string]map[string]CandidateFact{
			"e4": {"e3": {DestinationVisible: true, GuardedBy: []string{"d3"}}},
		},
	}
	needZero := 0
	obsKingExcess.CaptureMoraleNeed = &needZero
	bKingExcess := &boardContext{
		side: "white",
		own:  []*Cell{{UnitID: UnitID{Str: "k"}, Square: "e4", Rank: "king"}, {UnitID: UnitID{Str: "g"}, Square: "d3", Rank: "pawn"}},
		ownBySquare: map[string]*Cell{
			"e4": {UnitID: UnitID{Str: "k"}, Square: "e4", Rank: "king"},
			"d3": {UnitID: UnitID{Str: "g"}, Square: "d3", Rank: "pawn"},
		},
	}
	formationLeaderPriority(obsKingExcess, bKingExcess, &BotParams{MoralePush: 1.0}, bKingExcess.own[0])

	// score.go: unitPriority king direct capture
	obsUnitKingCap := &Observation{
		Legal: map[string][]string{
			"e2": {"e8"},
		},
		Affordability: map[string]map[string]DestinationOutcomes{
			"e2": {"e8": {Capture: &CaptureOutcome{Affordable: true}}},
		},
	}
	bUnitKingCap := &boardContext{
		enemy:         []*Cell{{Square: "e8", Rank: "king"}},
		enemyBySquare: map[string]*Cell{"e8": {Square: "e8", Rank: "king"}},
	}
	unitPriority(obsUnitKingCap, bUnitKingCap, &BotParams{}, &Cell{Square: "e2", Rank: "pawn"})

	// scoreMove: escort risk with KingCargo, safe trade discount, prisoner cargo escort, king safety escape
	obsEscortRisk := &Observation{
		Side: "white",
		Pieces: map[string]Cell{
			"e3": {Square: "e3", Side: "black", Rank: "queen"},
		},
		Candidates: map[string]map[string]CandidateFact{
			"e2": {"e3": {DestinationVisible: true, ThreatenedBy: []string{"d4"}}},
		},
	}
	bEscortRisk := buildBoard(obsEscortRisk)
	cellKingCargo := &Cell{Square: "e2", Rank: "pawn", Convoy: true, KingCargo: true, UnitID: UnitID{Str: "p"}}
	scoreMove(obsEscortRisk, bEscortRisk, &BotParams{Safety: 1.0}, nil, cellKingCargo, "e3")
	cellPawnTrade := &Cell{Square: "e2", Rank: "pawn", UnitID: UnitID{Str: "p"}}
	scoreMove(obsEscortRisk, bEscortRisk, &BotParams{Safety: 1.0, Material: 1.0}, nil, cellPawnTrade, "e3")

	// scoreMove: cargo escort with needsFirstMasterEngineer
	obsPrisonerCargo := &Observation{
		Side: "white",
		Rules: Rules{
			BaseSquares: map[string][]string{"pawn": {"e1"}},
		},
	}
	bPrisonerCargo := buildBoard(obsPrisonerCargo)
	bPrisonerCargo.needsFirstMasterEngineer = true
	cellPrisonerCargo := &Cell{Square: "e3", Rank: "pawn", Convoy: true, CargoCount: 3, UnitID: UnitID{Str: "p"}}
	scoreMove(obsPrisonerCargo, bPrisonerCargo, &BotParams{Prisoner: 1.0}, nil, cellPrisonerCargo, "e2")

	// scoreMove: delivery offRoute (thereCost >= UnreachablePathCost)
	obsOffRoute := &Observation{
		Side: "white",
		Rules: Rules{
			BaseSquares: map[string][]string{"pawn": {"e1"}},
		},
	}
	bOffRoute := buildBoard(obsOffRoute)
	bOffRoute.convoyHome = map[string]int{"e2": 1} // e3 not in convoyHome
	cellConvoyHome := &Cell{Square: "e2", Rank: "pawn", Convoy: true, KingCargo: true, UnitID: UnitID{Str: "p"}}
	scoreMove(obsOffRoute, bOffRoute, &BotParams{Delivery: 1.0}, nil, cellConvoyHome, "e3")

	// scoreMove: king escape
	obsKingEscape := &Observation{
		Side: "white",
		Candidates: map[string]map[string]CandidateFact{
			"e1": {"e2": {DestinationVisible: true}},
		},
	}
	bKingEscape := buildBoard(obsKingEscape)
	bKingEscape.kingThreatened = true
	bKingEscape.kingCell = &Cell{Square: "e1", Rank: "king", UnitID: UnitID{Str: "k"}}
	scoreMove(obsKingEscape, bKingEscape, &BotParams{KingSafety: 1.0}, nil, bKingEscape.kingCell, "e2")

	// scoreMove: moralePush excessAdvance, excessRetreat
	obsKingMoralePush := &Observation{
		Side:      "white",
		OwnMorale: 10,
		Candidates: map[string]map[string]CandidateFact{
			"e2": {"e6": {DestinationVisible: true, GuardedBy: []string{"d5"}}},
			"e6": {"e5": {DestinationVisible: true, GuardedBy: []string{"d5"}}},
		},
	}
	bKingMoralePush := buildBoard(obsKingMoralePush)
	kE2 := &Cell{Square: "e2", Rank: "king", UnitID: UnitID{Str: "k"}}
	scoreMove(obsKingMoralePush, bKingMoralePush, &BotParams{MoralePush: 1.0}, nil, kE2, "e6")
	kE6 := &Cell{Square: "e6", Rank: "king", UnitID: UnitID{Str: "k"}}
	scoreMove(obsKingMoralePush, bKingMoralePush, &BotParams{MoralePush: 1.0}, nil, kE6, "e5")

	// scoreMove: beacon bearer regroup (gain <= 0 with supportGain > 0)
	obsBearerRegroup := &Observation{
		Side:   "white",
		Beacon: BeaconFact{Lifecycle: "deployed", BearerSquare: "d4"},
		Candidates: map[string]map[string]CandidateFact{
			"d4": {"d3": {DestinationVisible: true, GuardedBy: []string{"c2"}}},
		},
	}
	bBearerRegroup := &boardContext{
		side: "white",
		own: []*Cell{
			{Square: "d4", Rank: "pawn", UnitID: UnitID{Str: "b"}},
			{Square: "d2", Rank: "pawn", UnitID: UnitID{Str: "ally"}},
		},
		ownBySquare: map[string]*Cell{
			"d4": {Square: "d4", Rank: "pawn", UnitID: UnitID{Str: "b"}},
			"d2": {Square: "d2", Rank: "pawn", UnitID: UnitID{Str: "ally"}},
		},
	}
	scoreMove(obsBearerRegroup, bBearerRegroup, &BotParams{BeaconAggression: 1.0}, nil, bBearerRegroup.own[0], "d3")

	// priorityCaptiveDeliveryProposal fallback and not-on-base gain<=0
	obsPrioFallback := &Observation{
		Side: "white",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
		},
		Legal: map[string][]string{
			"e1": {"e2"},
			"e3": {"e4"},
		},
		Candidates: map[string]map[string]CandidateFact{
			"e1": {"e2": {DestinationVisible: true}},
			"e3": {"e4": {DestinationVisible: true}},
		},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "c1"}, Side: "white", Rank: "pawn", Convoy: true, CargoCount: 1},
			"e3": {UnitID: UnitID{Str: "c2"}, Side: "white", Rank: "pawn", Convoy: true, CargoCount: 1},
		},
	}
	bPrioFallback := buildBoard(obsPrioFallback)
	priorityCaptiveDeliveryProposal(obsPrioFallback, bPrioFallback, &BotParams{})

	// priorityCaptiveDeliveryInFlight on base square
	obsInFlightBase := &Observation{
		Side: "white",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
		},
		Pieces: map[string]Cell{
			"e1": {
				UnitID:     UnitID{Str: "c1"},
				Side:       "white",
				Rank:       "pawn",
				Convoy:     true,
				CargoCount: 1,
				Charging:   &ChargingFact{Square: "e2", RemainingMs: 100},
			},
		},
	}
	bInFlightBase := buildBoard(obsInFlightBase)
	priorityCaptiveDeliveryInFlight(obsInFlightBase, bInFlightBase)

	// rankOptions passBelow break
	rankOptions([]proposal{{score: -10.0}}, &BotParams{PassBelow: 0.0}, 2)

	// systems.go: trainingProposals, beaconDeploy, beaconForge, espionage
	obsTrainNoPermit := &Observation{
		Side: "white",
		Rules: Rules{
			BaseSquares:          map[string][]string{"pawn": {"e1"}},
			PermittedProfessions: []string{"none"},
		},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn"},
		},
	}
	bTrainNoPermit := buildBoard(obsTrainNoPermit)
	trainingProposals(obsTrainNoPermit, bTrainNoPermit, &BotParams{System: 1.0})

	beaconDeployOrRestoreProposal(&Observation{Rules: Rules{BeaconEnabled: false}}, &boardContext{}, &BotParams{})

	obsForgeFilters := &Observation{
		Rules: Rules{
			BeaconEnabled:      true,
			BeaconForgeEnabled: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
		},
		Beacon: BeaconFact{Lifecycle: "lost"},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn", Convoy: true},
			"e2": {UnitID: UnitID{Str: "p2"}, Side: "white", Rank: "pawn"},
			"e3": {UnitID: UnitID{Str: "p3"}, Side: "white", Rank: "pawn", ThreatenedBy: []string{"e8"}},
		},
	}
	bForgeFilters := buildBoard(obsForgeFilters)
	beaconForgeProposals(obsForgeFilters, bForgeFilters, &BotParams{})

	obsEspFilters := &Observation{
		EnemyManaged: 1,
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "k"}, Side: "white", Rank: "king"},
			"e2": {UnitID: UnitID{Str: "k2"}, Side: "white", Rank: "king"},
			"e8": {UnitID: UnitID{Str: "far"}, Side: "white", Rank: "pawn"},
		},
	}
	bEspFilters := buildBoard(obsEspFilters)
	espionageProposals(obsEspFilters, bEspFilters, &BotParams{System: 1.0})
}

func TestFinalEdgesTo100Percent(t *testing.T) {
	// 1. score.go:289 formationLeaderPriority deficit king
	obsDeficit := &Observation{
		Side:      "white",
		OwnMorale: 1,
		Legal:     map[string][]string{"e1": {"e2"}},
		Candidates: map[string]map[string]CandidateFact{
			"e1": {"e2": {DestinationVisible: true, GuardedBy: []string{"d1"}}},
		},
	}
	needThree := 3
	obsDeficit.CaptureMoraleNeed = &needThree
	bDeficit := &boardContext{
		side: "white",
		own:  []*Cell{{Square: "e1", Rank: "king", UnitID: UnitID{Str: "k"}}, {Square: "d1", Rank: "pawn", UnitID: UnitID{Str: "g"}}},
		ownBySquare: map[string]*Cell{
			"e1": {Square: "e1", Rank: "king", UnitID: UnitID{Str: "k"}},
			"d1": {Square: "d1", Rank: "pawn", UnitID: UnitID{Str: "g"}},
		},
		enemyBySquare: map[string]*Cell{},
	}
	if p := formationLeaderPriority(obsDeficit, bDeficit, &BotParams{MoralePush: 1.0}, bDeficit.own[0]); p != UnitPriorityFormationLeader {
		t.Errorf("formationLeaderPriority deficit = %f, want %f", p, UnitPriorityFormationLeader)
	}

	// 2. score.go:304 formationLeaderPriority beacon bearer
	obsBearerPrio := &Observation{
		Side:   "white",
		Beacon: BeaconFact{Lifecycle: "deployed", BearerSquare: "d4"},
		Legal:  map[string][]string{"d4": {"d5"}},
		Candidates: map[string]map[string]CandidateFact{
			"d4": {"d5": {DestinationVisible: true, GuardedBy: []string{"c4"}}},
		},
	}
	bBearerPrio := &boardContext{
		side: "white",
		own: []*Cell{
			{UnitID: UnitID{Str: "b"}, Square: "d4", Rank: "knight"},
			{UnitID: UnitID{Str: "g"}, Square: "d6", Rank: "pawn"},
		},
		ownBySquare: map[string]*Cell{
			"d4": {UnitID: UnitID{Str: "b"}, Square: "d4", Rank: "knight"},
			"d6": {UnitID: UnitID{Str: "g"}, Square: "d6", Rank: "pawn"},
		},
		enemyBySquare: map[string]*Cell{},
	}
	if p := formationLeaderPriority(obsBearerPrio, bBearerPrio, &BotParams{BeaconAggression: 1.0}, bBearerPrio.own[0]); p != UnitPriorityFormationLeader {
		t.Errorf("formationLeaderPriority bearer = %f, want %f", p, UnitPriorityFormationLeader)
	}

	// 3. score.go:344 unitPriority king visible attack next move
	obsKingVis := &Observation{
		Legal: map[string][]string{"e2": {"e3"}},
		Candidates: map[string]map[string]CandidateFact{
			"e2": {"e3": {DestinationVisible: true, GuardedBy: []string{"d2"}, NextPossibleMoves: []string{"e8"}}},
		},
	}
	bKingVis := &boardContext{
		enemy: []*Cell{{Square: "e8", Rank: "king"}},
		ownBySquare: map[string]*Cell{
			"d2": {Square: "d2", Rank: "pawn"},
		},
		enemyBySquare: map[string]*Cell{},
	}
	pawnPrio := &Cell{Square: "e2", Rank: "pawn"}
	unitPriority(obsKingVis, bKingVis, &BotParams{}, pawnPrio)

	// 4. score.go:399 bootstrap bonus, 548 king visible attack bonus, 509 promotion next
	obsBoot := &Observation{
		Side: "white",
		Rules: Rules{
			AllowsCapture: true,
			BaseSquares:   map[string][]string{"pawn": {"e1"}},
		},
		Affordability: map[string]map[string]DestinationOutcomes{
			"e2": {"e3": {Capture: &CaptureOutcome{Affordable: true}}},
		},
		Candidates: map[string]map[string]CandidateFact{
			"e6": {"e7": {DestinationVisible: true, GuardedBy: []string{"d6"}, NextPossibleMoves: []string{"e8"}}},
		},
	}
	bBoot := &boardContext{
		side:                     "white",
		needsFirstMasterEngineer: true,
		enemyBySquare: map[string]*Cell{
			"e3": {Square: "e3", Rank: "pawn"},
			"e8": {Square: "e8", Rank: "king"},
		},
		enemy: []*Cell{
			{Square: "e8", Rank: "king"},
		},
		ownBySquare: map[string]*Cell{
			"d6": {Square: "d6", Rank: "pawn"},
		},
		blockingBase: []string{"e1"},
	}
	pawnBoot := &Cell{Square: "e2", Rank: "pawn", UnitID: UnitID{Str: "p"}}
	scoreMove(obsBoot, bBoot, &BotParams{Prisoner: 1.0}, nil, pawnBoot, "e3")

	// score.go:488 king in blocking base (rankVal > QueenValue)
	kingBlock := &Cell{Square: "e1", Rank: "king", UnitID: UnitID{Str: "k"}}
	scoreMove(obsBoot, bBoot, &BotParams{Delivery: 1.0}, nil, kingBlock, "e2")

	// score.go:509 and 548 on pawnPromNext
	pawnPromNext := &Cell{Square: "e6", Rank: "pawn", UnitID: UnitID{Str: "p"}}
	scoreMove(obsBoot, bBoot, &BotParams{Advance: 1.0}, nil, pawnPromNext, "e7")

	// 5. score.go:589 guardedAdvance, 600 kingIntoStrike
	obsKingGuarded := &Observation{
		Side:      "white",
		OwnMorale: 1, // deficit -> excess < LeaderExcessMorale
		Candidates: map[string]map[string]CandidateFact{
			"e1": {
				"e2": {DestinationVisible: true, GuardedBy: []string{"d1"}},
				"e3": {DestinationVisible: true, ThreatenedBy: []string{"d4"}},
			},
		},
	}
	bKingGuarded := &boardContext{
		side: "white",
		own: []*Cell{
			{Square: "e1", Rank: "king", UnitID: UnitID{Str: "k"}},
			{Square: "d1", Rank: "pawn", UnitID: UnitID{Str: "g"}},
		},
		ownBySquare: map[string]*Cell{
			"e1": {Square: "e1", Rank: "king", UnitID: UnitID{Str: "k"}},
			"d1": {Square: "d1", Rank: "pawn", UnitID: UnitID{Str: "g"}},
		},
		enemyBySquare: map[string]*Cell{},
	}
	kGuarded := bKingGuarded.own[0]
	// guardedAdvance
	scoreMove(obsKingGuarded, bKingGuarded, &BotParams{MoralePush: 1.0}, nil, kGuarded, "e2")
	// kingIntoStrike
	scoreMove(obsKingGuarded, bKingGuarded, &BotParams{MoralePush: 1.0, Safety: 1.0}, nil, kGuarded, "e3")

	// 6. score.go:610-614 beacon bearer supportGain > 0 with gain > 0 and gain <= 0
	obsBearerAdvance := &Observation{
		Side:   "white",
		Beacon: BeaconFact{Lifecycle: "deployed", BearerSquare: "d3"},
		Candidates: map[string]map[string]CandidateFact{
			"d3": {
				"d4": {DestinationVisible: true, GuardedBy: []string{"c4"}}, // gain > 0
			},
		},
	}
	bBearerAdvance := &boardContext{
		side: "white",
		own: []*Cell{
			{Square: "d3", Rank: "pawn", UnitID: UnitID{Str: "b"}},
			{Square: "d5", Rank: "pawn", UnitID: UnitID{Str: "ally"}},
		},
		ownBySquare: map[string]*Cell{
			"d3": {Square: "d3", Rank: "pawn", UnitID: UnitID{Str: "b"}},
			"d5": {Square: "d5", Rank: "pawn", UnitID: UnitID{Str: "ally"}},
		},
		enemyBySquare: map[string]*Cell{},
	}
	scoreMove(obsBearerAdvance, bBearerAdvance, &BotParams{BeaconAggression: 1.0}, nil, bBearerAdvance.own[0], "d4")

	obsBearerRegroup := &Observation{
		Side:   "white",
		Beacon: BeaconFact{Lifecycle: "deployed", BearerSquare: "d5"},
		Candidates: map[string]map[string]CandidateFact{
			"d5": {
				"d4": {DestinationVisible: true, GuardedBy: []string{"c4"}}, // gain <= 0
			},
		},
	}
	bBearerRegroup := &boardContext{
		side: "white",
		own: []*Cell{
			{Square: "d5", Rank: "pawn", UnitID: UnitID{Str: "b"}},
			{Square: "c5", Rank: "pawn", UnitID: UnitID{Str: "ally"}},
		},
		ownBySquare: map[string]*Cell{
			"d5": {Square: "d5", Rank: "pawn", UnitID: UnitID{Str: "b"}},
			"c5": {Square: "c5", Rank: "pawn", UnitID: UnitID{Str: "ally"}},
		},
		enemyBySquare: map[string]*Cell{},
	}
	scoreMove(obsBearerRegroup, bBearerRegroup, &BotParams{BeaconAggression: 1.0}, nil, bBearerRegroup.own[0], "d4")

	// 7. score.go:650, 658, 670, 691, 702 priority delivery edge cases
	obsPrioMore := &Observation{
		Side: "white",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
		},
		Legal: map[string][]string{
			"e1": {"e2"},
			"e3": {"e4"},
			"e5": {},
			"e6": {"e7"},
		},
		Candidates: map[string]map[string]CandidateFact{
			"e1": {"e2": {DestinationVisible: false}},                           // 670 !visible
			"e3": {"e4": {DestinationVisible: true, GuardedBy: []string{"d4"}}}, // 691 gain <= bestGain
			"e6": {"e7": {DestinationVisible: false}},                           // 702 to == ""
		},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "c1"}, Side: "white", Rank: "pawn", Convoy: true, CargoCount: 1},
			"e3": {UnitID: UnitID{Str: "c2"}, Side: "white", Rank: "pawn", Convoy: true, CargoCount: 1},
			"e5": {UnitID: UnitID{Str: "c3"}, Side: "white", Rank: "pawn", Convoy: true, CargoCount: 1}, // 658 len == 0
			"e6": {UnitID: UnitID{Str: "c4"}, Side: "white", Rank: "pawn", Convoy: true, CargoCount: 1},
			"e7": {UnitID: UnitID{Str: "c5"}, Side: "white", Rank: "pawn", Convoy: true, CargoCount: 1, NotBriefed: true}, // 650 NotBriefed
		},
	}
	bPrioMore := buildBoard(obsPrioMore)
	priorityCaptiveDeliveryProposal(obsPrioMore, bPrioMore, &BotParams{})

	// 8. score.go:726 priorityCaptiveDeliveryInFlight with charging.Square occupied
	obsFlightOcc := &Observation{
		Side: "white",
		Rules: Rules{
			VeteranProgression: true,
			BaseSquares:        map[string][]string{"pawn": {"e1"}},
		},
		Pieces: map[string]Cell{
			"e2": {
				UnitID:     UnitID{Str: "c1"},
				Side:       "white",
				Rank:       "pawn",
				Convoy:     true,
				CargoCount: 1,
				Charging:   &ChargingFact{Square: "e1", RemainingMs: 100},
			},
			"e1": {
				UnitID: UnitID{Str: "blocker"},
				Side:   "white",
				Rank:   "rook",
			},
		},
	}
	bFlightOcc := buildBoard(obsFlightOcc)
	priorityCaptiveDeliveryInFlight(obsFlightOcc, bFlightOcc)

	// 9. systems.go:47, 50, 93, 96, 176
	obsHandoff1 := &Observation{
		Rules:  Rules{BeaconEnabled: true, BeaconKingStartsAsBearer: true},
		Beacon: BeaconFact{Lifecycle: "lost"},
	}
	bHandoff1 := &boardContext{kingCell: &Cell{Square: "e1"}}
	beaconHandOffProposal(obsHandoff1, bHandoff1, &BotParams{})

	obsHandoff2 := &Observation{
		Rules:  Rules{BeaconEnabled: true, BeaconKingStartsAsBearer: true},
		Beacon: BeaconFact{Lifecycle: "deployed", BearerSquare: "d4"},
	}
	beaconHandOffProposal(obsHandoff2, bHandoff1, &BotParams{})

	obsTrainMore := &Observation{
		Side: "white",
		Rules: Rules{
			SpecialistCeiling:    10,
			BaseSquares:          map[string][]string{"pawn": {"e1", "f1", "g1"}},
			PermittedProfessions: []string{"engineer"},
		},
		Pieces: map[string]Cell{
			"e2": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn"},                               // 93 not on base
			"e1": {UnitID: UnitID{Str: "p2"}, Side: "white", Rank: "pawn", ThreatenedBy: []string{"e8"}}, // 96 threatened
			"f1": {UnitID: UnitID{Str: "p3"}, Side: "white", Rank: "pawn"},                               // 110 trains sergeant, but permitted is engineer
			"a3": {UnitID: UnitID{Str: "eng"}, Side: "white", Rank: "pawn", Profession: "engineer"},
		},
	}
	bTrainMore := buildBoard(obsTrainMore)
	trainingProposals(obsTrainMore, bTrainMore, &BotParams{System: 1.0}) // 110 !contains permitted ("sergeant")

	obsWallSession := &Observation{
		Side: "white",
		Walls: []WallFact{
			{A: "e1", B: "e2", OwnWorkSession: true},
		},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "eng"}, Side: "white", Rank: "pawn", Profession: "engineer"},
		},
	}
	bWallSession := buildBoard(obsWallSession)
	wallProposals(obsWallSession, bWallSession, &BotParams{System: 1.0})

	// beaconForgeProposals 258, 267, 270, 273
	beaconForgeProposals(&Observation{Rules: Rules{BeaconEnabled: false}}, &boardContext{}, &BotParams{})
	obsForgeMore := &Observation{
		Side: "white",
		Rules: Rules{
			BeaconEnabled:      true,
			BeaconForgeEnabled: true,
			BaseSquares:        map[string][]string{"pawn": {"e1", "f1", "g1"}},
		},
		Beacon: BeaconFact{Lifecycle: "lost"},
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "p1"}, Side: "white", Rank: "pawn", Convoy: true},                 // 267
			"e2": {UnitID: UnitID{Str: "p2"}, Side: "white", Rank: "pawn"},                               // 270
			"f1": {UnitID: UnitID{Str: "p3"}, Side: "white", Rank: "pawn", ThreatenedBy: []string{"e8"}}, // 273
		},
	}
	bForgeMore := buildBoard(obsForgeMore)
	beaconForgeProposals(obsForgeMore, bForgeMore, &BotParams{})

	// espionageProposals 299, 302
	obsEspMore := &Observation{
		Side:         "white",
		EnemyManaged: 1,
		Pieces: map[string]Cell{
			"e1": {UnitID: UnitID{Str: "k"}, Side: "white", Rank: "king"},
			"e2": {UnitID: UnitID{Str: "k2"}, Side: "white", Rank: "king"},  // 299
			"e8": {UnitID: UnitID{Str: "far"}, Side: "white", Rank: "pawn"}, // 302
		},
	}
	bEspMore := buildBoard(obsEspMore)
	espionageProposals(obsEspMore, bEspMore, &BotParams{System: 1.0})
}
