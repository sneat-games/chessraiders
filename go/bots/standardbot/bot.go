// Copyright 2026 Sneat.app

package standardbot

import (
	"encoding/json"
	"sort"
)

// PARITY-FUNCTION: SBP-F-INTN
func intn(randomDraw int64, count int) int {
	if count <= 1 { // PARITY-BRANCH: SBP-B-INTN-1
		return 0
	}
	res := randomDraw % int64(count)
	if res < 0 { // PARITY-BRANCH: SBP-B-INTN-2
		res += int64(count)
	}
	return int(res)
}

// Decide computes the bot decision directly on typed Go structures.
// PARITY-FUNCTION: SBP-F-DECIDE
func Decide(obs *Observation, memory map[string]int64, params *BotParams, hostRandomDraw int64, options int) (*Intent, map[string]int64, []Option) {
	if obs.Lifecycle != "playing" { // PARITY-BRANCH: SBP-B-DECIDE-1
		return nil, memory, nil
	}

	b := buildBoard(obs)
	if len(b.own) == 0 { // PARITY-BRANCH: SBP-B-DECIDE-2
		return nil, memory, nil
	}

	if len(b.protectedChargingUnits) > 0 { // PARITY-BRANCH: SBP-B-DECIDE-3
		return finishDecision(obs, b, memory, nil, nil)
	}

	if priorityCaptiveDeliveryInFlight(obs, b) { // PARITY-BRANCH: SBP-B-DECIDE-4
		return finishDecision(obs, b, memory, nil, nil)
	}

	if len(b.chargingUnits) == 0 && !holdsFocus(b, memory) && len(b.convoyHome) == 0 { // PARITY-BRANCH: SBP-B-DECIDE-5
		priority := priorityCaptiveDeliveryProposal(obs, b, params)
		if priority != nil { // PARITY-BRANCH: SBP-B-DECIDE-6
			ranked := rankOptions([]proposal{*priority}, params, options)
			return finishDecision(obs, b, memory, priority, ranked)
		}
	}

	var proposals []proposal
	if !holdsFocus(b, memory) { // PARITY-BRANCH: SBP-B-DECIDE-7
		proposals = moveProposals(obs, b, params, memory)
	}
	if len(b.chargingUnits) == 0 { // PARITY-BRANCH: SBP-B-DECIDE-8
		proposals = append(proposals, systemProposals(obs, b, params)...)
	}
	proposals = dropRefused(proposals, obs, memory)

	if kingChannelActive(obs, b) && len(b.chargingUnits) == 0 && b.kingCell != nil { // PARITY-BRANCH: SBP-B-DECIDE-9
		kingThreshold := retentionScore(memory, CommitKindKing)
		kingID := b.kingCell.UnitID.String()
		var filtered []proposal
		for _, p := range proposals {
			actorID := ""
			if uid, ok := p.actor.(UnitID); ok { // PARITY-BRANCH: SBP-B-DECIDE-10
				actorID = uid.String()
			} else if str, ok := p.actor.(string); ok { // PARITY-BRANCH: SBP-B-DECIDE-11
				actorID = str
			}
			if actorID != kingID || p.score > kingThreshold { // PARITY-BRANCH: SBP-B-DECIDE-12
				filtered = append(filtered, p)
			}
		}
		proposals = filtered
	}

	if len(b.chargingUnits) > 0 { // PARITY-BRANCH: SBP-B-DECIDE-13
		threshold := retentionScore(memory, CommitKindRoute)
		var filtered []proposal
		for _, p := range proposals {
			actorID := ""
			if uid, ok := p.actor.(UnitID); ok { // PARITY-BRANCH: SBP-B-DECIDE-14
				actorID = uid.String()
			} else if str, ok := p.actor.(string); ok { // PARITY-BRANCH: SBP-B-DECIDE-15
				actorID = str
			}
			if !b.chargingUnits[actorID] || p.score > threshold { // PARITY-BRANCH: SBP-B-DECIDE-16
				filtered = append(filtered, p)
			}
		}
		proposals = filtered
	}

	if len(proposals) == 0 { // PARITY-BRANCH: SBP-B-DECIDE-17
		return finishDecision(obs, b, memory, nil, nil)
	}

	sort.Slice(proposals, func(i, j int) bool {
		if proposals[i].score != proposals[j].score { // PARITY-BRANCH: SBP-B-DECIDE-18
			return proposals[i].score > proposals[j].score
		}
		return proposals[i].key < proposals[j].key
	})

	ranked := rankOptions(proposals, params, options)
	bestProposal := proposals[0]
	if bestProposal.score < params.PassBelow { // PARITY-BRANCH: SBP-B-DECIDE-19
		return finishDecision(obs, b, memory, nil, ranked)
	}

	tiedCount := 1
	for tiedCount < len(proposals) && proposals[tiedCount].score >= bestProposal.score-TieBreakBand {
		tiedCount++
	}
	chosen := proposals[intn(hostRandomDraw, tiedCount)]
	return finishDecision(obs, b, memory, &chosen, ranked)
}

// DecideJSON accepts JSON bytes and returns JSON bytes.
// PARITY-FUNCTION: SBP-GO-STRUCT-F-DECIDE-JSON
func DecideJSON(observationJSON, memoryJSON, paramsJSON []byte, hostRandomDraw int64, options int) ([]byte, []byte, []byte, error) {
	var obs Observation
	if err := json.Unmarshal(observationJSON, &obs); err != nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-1
		return nil, nil, nil, err
	}
	var mem map[string]int64
	if len(memoryJSON) > 0 && string(memoryJSON) != "null" { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-2
		if err := json.Unmarshal(memoryJSON, &mem); err != nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-3
			return nil, nil, nil, err
		}
	}
	if mem == nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-4
		mem = make(map[string]int64)
	}
	var params BotParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-5
		return nil, nil, nil, err
	}

	intent, updatedMem, ranked := Decide(&obs, mem, &params, hostRandomDraw, options)

	var intentBytes, memBytes, rankedBytes []byte
	var err error
	if intent != nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-6
		intentBytes, err = json.Marshal(intent)
		if err != nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-7
			return nil, nil, nil, err
		}
	} else {
		intentBytes = []byte("null")
	}

	memBytes, err = json.Marshal(updatedMem)
	if err != nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-8
		return nil, nil, nil, err
	}

	if ranked != nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-9
		rankedBytes, err = json.Marshal(ranked)
		if err != nil { // PARITY-BRANCH: SBP-GO-STRUCT-B-DECIDE-JSON-10
			return nil, nil, nil, err
		}
	} else {
		rankedBytes = []byte("[]")
	}

	return intentBytes, memBytes, rankedBytes, nil
}
