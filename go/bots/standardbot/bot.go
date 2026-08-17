// Copyright 2026 Sneat.app

package standardbot

import (
	"encoding/json"
	"sort"
)

func intn(randomDraw int64, count int) int {
	if count <= 1 {
		return 0
	}
	res := randomDraw % int64(count)
	if res < 0 {
		res += int64(count)
	}
	return int(res)
}

// Decide computes the bot decision directly on typed Go structures.
func Decide(obs *Observation, memory map[string]int64, params *BotParams, hostRandomDraw int64, options int) (*Intent, map[string]int64, []Option) {
	if obs.Lifecycle != "playing" {
		return nil, memory, nil
	}

	b := buildBoard(obs)
	if len(b.own) == 0 {
		return nil, memory, nil
	}

	if len(b.protectedChargingUnits) > 0 {
		return finishDecision(obs, b, memory, nil, nil)
	}

	if priorityCaptiveDeliveryInFlight(obs, b) {
		return finishDecision(obs, b, memory, nil, nil)
	}

	if len(b.chargingUnits) == 0 && !holdsFocus(b, memory) && len(b.convoyHome) == 0 {
		priority := priorityCaptiveDeliveryProposal(obs, b, params)
		if priority != nil {
			ranked := rankOptions([]proposal{*priority}, params, options)
			return finishDecision(obs, b, memory, priority, ranked)
		}
	}

	var proposals []proposal
	if !holdsFocus(b, memory) {
		proposals = moveProposals(obs, b, params, memory)
	}
	if len(b.chargingUnits) == 0 {
		proposals = append(proposals, systemProposals(obs, b, params)...)
	}
	proposals = dropRefused(proposals, obs, memory)

	if kingChannelActive(obs, b) && len(b.chargingUnits) == 0 && b.kingCell != nil {
		kingThreshold := retentionScore(memory, CommitKindKing)
		kingID := b.kingCell.UnitID.String()
		var filtered []proposal
		for _, p := range proposals {
			actorID := ""
			if uid, ok := p.actor.(UnitID); ok {
				actorID = uid.String()
			} else if str, ok := p.actor.(string); ok {
				actorID = str
			}
			if actorID != kingID || p.score > kingThreshold {
				filtered = append(filtered, p)
			}
		}
		proposals = filtered
	}

	if len(b.chargingUnits) > 0 {
		threshold := retentionScore(memory, CommitKindRoute)
		var filtered []proposal
		for _, p := range proposals {
			actorID := ""
			if uid, ok := p.actor.(UnitID); ok {
				actorID = uid.String()
			} else if str, ok := p.actor.(string); ok {
				actorID = str
			}
			if !b.chargingUnits[actorID] || p.score > threshold {
				filtered = append(filtered, p)
			}
		}
		proposals = filtered
	}

	if len(proposals) == 0 {
		return finishDecision(obs, b, memory, nil, nil)
	}

	sort.Slice(proposals, func(i, j int) bool {
		if proposals[i].score != proposals[j].score {
			return proposals[i].score > proposals[j].score
		}
		return proposals[i].key < proposals[j].key
	})

	ranked := rankOptions(proposals, params, options)
	bestProposal := proposals[0]
	if bestProposal.score < params.PassBelow {
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
func DecideJSON(observationJSON, memoryJSON, paramsJSON []byte, hostRandomDraw int64, options int) ([]byte, []byte, []byte, error) {
	var obs Observation
	if err := json.Unmarshal(observationJSON, &obs); err != nil {
		return nil, nil, nil, err
	}
	var mem map[string]int64
	if len(memoryJSON) > 0 && string(memoryJSON) != "null" {
		if err := json.Unmarshal(memoryJSON, &mem); err != nil {
			return nil, nil, nil, err
		}
	}
	if mem == nil {
		mem = make(map[string]int64)
	}
	var params BotParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, nil, nil, err
	}

	intent, updatedMem, ranked := Decide(&obs, mem, &params, hostRandomDraw, options)

	var intentBytes, memBytes, rankedBytes []byte
	var err error
	if intent != nil {
		intentBytes, err = json.Marshal(intent)
		if err != nil {
			return nil, nil, nil, err
		}
	} else {
		intentBytes = []byte("null")
	}

	memBytes, err = json.Marshal(updatedMem)
	if err != nil {
		return nil, nil, nil, err
	}

	if ranked != nil {
		rankedBytes, err = json.Marshal(ranked)
		if err != nil {
			return nil, nil, nil, err
		}
	} else {
		rankedBytes = []byte("[]")
	}

	return intentBytes, memBytes, rankedBytes, nil
}
