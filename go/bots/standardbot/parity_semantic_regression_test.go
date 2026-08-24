// Copyright 2026 Sneat.app

package standardbot_test

import (
	"strings"
	"testing"
)

func TestCorrectedSemanticGroupsNameTheSameDecisionInBothImplementations(t *testing.T) {
	ledger := readSemanticLedger(t)
	tests := []struct {
		id       string
		goSource string
		star     string
	}{
		{id: "SBP-B-SCORE-MOVE-5", goSource: "targetCell != nil", star: "target_cell"},
		{id: "SBP-B-SCORE-MOVE-6", goSource: "needsFirstMasterEngineer", star: "needs_first_master_engineer"},
		{id: "SBP-B-SCORE-MOVE-7", goSource: `targetCell.Rank == "king"`, star: `target_cell["rank"] == "king"`},
		{id: "SBP-B-SCORE-MOVE-9", goSource: "needsFirstMasterEngineer", star: "needs_first_master_engineer"},
		{id: "SBP-B-SCORE-MOVE-10", goSource: "candidateKnown", star: "candidate_known"},
		{id: "SBP-B-SCORE-MOVE-20", goSource: "deliverySquares", star: "delivery_squares"},
		{id: "SBP-B-SCORE-MOVE-21", goSource: "hereCost", star: "here_cost"},
		{id: "SBP-B-SCORE-MOVE-34", goSource: "PatrolGain", star: "patrolGain"},
		{id: "SBP-B-SCORE-MOVE-35", goSource: "DestinationVisible", star: "destinationVisible"},
		{id: "SBP-B-SCORE-MOVE-55", goSource: "supportGain", star: "support_gain"},
		{id: "SBP-B-SCORE-MOVE-56", goSource: "isPromotionSquare", star: "is_promotion_square"},
		{id: "SBP-B-SCORE-MOVE-57", goSource: `captureChoice != ""`, star: "capture_choice"},
		{id: "SBP-B-BUILD-MEMORY-2", goSource: "stillFrozen", star: "still_frozen"},
		{id: "SBP-B-DECIDE-10", goSource: "chargingUnits", star: "charging_units"},
		{id: "SBP-B-MOVE-PROPOSALS-2", goSource: "breadth", star: "breadth"},
		{id: "SBP-B-NEXT-COMMITTED-1", goSource: `intent.Kind == "move"`, star: `intent["kind"] == "move"`},
		{id: "SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-4", goSource: "destinations", star: "destinations"},
		{id: "SBP-B-UNPACK-PLACEMENT-BOARDS-1", goSource: "code != 0", star: "code != 0"},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			binding := semanticBindingByID(t, ledger, test.id)
			if len(binding.Go) != 1 || !strings.Contains(binding.Go[0].Source, test.goSource) {
				t.Errorf("Go source = %#v, want shared decision containing %q", binding.Go, test.goSource)
			}
			if len(binding.Starlark) != 1 || !strings.Contains(binding.Starlark[0].Source, test.star) {
				t.Errorf("Starlark source = %#v, want shared decision containing %q", binding.Starlark, test.star)
			}
		})
	}
}

func semanticBindingByID(t *testing.T, ledger semanticLedger, id string) semanticBinding {
	t.Helper()
	for _, binding := range ledger.Bindings {
		if binding.ID == id {
			return binding
		}
	}
	t.Fatalf("semantic ledger has no binding %s", id)
	return semanticBinding{}
}
