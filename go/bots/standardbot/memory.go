// Copyright 2026 Sneat.app

package standardbot

import (
	"fmt"
	"sort"
)

func roundHalfUp(value float64) int64 {
	if value >= 0 {
		return int64(value + 0.5)
	}
	return -int64(-value + 0.5)
}

func packCommitted(kind int64, age int64, score float64) int64 {
	offsetScore := roundHalfUp(score*CommitScoreScale) + CommitScoreOffset
	if offsetScore < 0 {
		offsetScore = 0
	} else if offsetScore >= CommitScoreField {
		offsetScore = CommitScoreField - 1
	}
	return kind*(int64(CommitAgeField)*int64(CommitScoreField)) + age*int64(CommitScoreField) + offsetScore
}

func unpackCommitted(packed int64) (kind int64, age int64, score float64) {
	field := int64(CommitAgeField) * int64(CommitScoreField)
	kind = packed / field
	remainder := packed % field
	age = remainder / int64(CommitScoreField)
	offsetScore := remainder % int64(CommitScoreField)
	score = float64(offsetScore-CommitScoreOffset) / CommitScoreScale
	return
}

func decayPower(age int64) float64 {
	value := 1.0
	cap := int(age)
	if cap > CommitAgeCap {
		cap = CommitAgeCap
	}
	for i := 0; i < cap; i++ {
		value *= CommitmentDecay
	}
	return value
}

func retentionScore(memory map[string]int64, kind int64) float64 {
	storedKind, age, score := unpackCommitted(memory["committed"])
	if storedKind != kind {
		return RouteReplaceBaseline
	}
	return score*decayPower(age) + RouteReplaceBaseline
}

func isKingChannelStart(intent *Intent) bool {
	return intent != nil && intent.Kind == "action" && intent.Action == "beacon_restore"
}

func kingChannelActive(obs *Observation, b *boardContext) bool {
	if b.kingCell == nil {
		return false
	}
	return obs.Beacon.Lifecycle == "restoring"
}

func nextCommitted(obs *Observation, b *boardContext, memory map[string]int64, chosen *proposal) int64 {
	var intent *Intent
	if chosen != nil {
		intent = &chosen.intent
	}
	if intent != nil && intent.Kind == "move" {
		mover := b.ownBySquare[intent.From]
		chargeMs := 0
		if mover != nil {
			chargeMs = obs.Rules.PieceChargeMs[mover.Rank]
		}
		if chargeMs > 0 {
			return packCommitted(CommitKindRoute, 0, chosen.score)
		}
	}
	if isKingChannelStart(intent) {
		return packCommitted(CommitKindKing, 0, chosen.score)
	}

	storedKind, age, score := unpackCommitted(memory["committed"])
	if len(b.chargingUnits) > 0 && storedKind == CommitKindRoute {
		ageNext := age + 1
		if ageNext > CommitAgeCap {
			ageNext = CommitAgeCap
		}
		return packCommitted(CommitKindRoute, ageNext, score)
	}
	kingID := ""
	if b.kingCell != nil {
		kingID = b.kingCell.UnitID.String()
	}
	actorID := ""
	if chosen != nil && chosen.actor != nil {
		if uid, ok := chosen.actor.(UnitID); ok {
			actorID = uid.String()
		} else if str, ok := chosen.actor.(string); ok {
			actorID = str
		}
	}
	if kingChannelActive(obs, b) && storedKind == CommitKindKing && actorID != kingID {
		ageNext := age + 1
		if ageNext > CommitAgeCap {
			ageNext = CommitAgeCap
		}
		return packCommitted(CommitKindKing, ageNext, score)
	}
	return 0
}

func finishDecision(obs *Observation, b *boardContext, memory map[string]int64, chosen *proposal, ranked []Option) (*Intent, map[string]int64, []Option) {
	var intent *Intent
	if chosen != nil {
		intent = &chosen.intent
	}
	updatedMemory := buildMemory(obs, memory, intent)
	committed := nextCommitted(obs, b, memory, chosen)
	if committed == 0 {
		delete(updatedMemory, "committed")
	} else {
		updatedMemory["committed"] = committed
	}
	return intent, updatedMemory, ranked
}

func holdsFocus(b *boardContext, memory map[string]int64) bool {
	focusMarker := memory["focusFrom"]
	if focusMarker <= 0 {
		return false
	}
	focusSquare := squareName(int(focusMarker - 1))
	focusCell := b.ownBySquare[focusSquare]
	return focusCell != nil && b.protectedChargingUnits[focusCell.UnitID.String()]
}

func dropRefused(proposals []proposal, obs *Observation, memory map[string]int64) []proposal {
	if memory == nil || memory["revision"] != obs.Revision {
		return proposals
	}
	var refusedSquares []string
	for slot := 0; slot < RefusedSetSize; slot++ {
		key := fmt.Sprintf("refused%d", slot)
		if idx, ok := memory[key]; ok && idx >= 0 && idx <= MaxSquareIndex {
			refusedSquares = append(refusedSquares, squareName(int(idx)))
		}
	}
	if len(refusedSquares) == 0 {
		return proposals
	}
	var filtered []proposal
	for _, p := range proposals {
		if !containsString(refusedSquares, p.intent.From) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func rankBitSlot(rank string) int {
	switch rank {
	case "pawn":
		return 0
	case "knight":
		return 1
	case "bishop":
		return 2
	case "rook":
		return 3
	case "queen":
		return 4
	case "king":
		return 5
	default:
		return -1
	}
}

func placementBitboards(obs *Observation, movedUnitID string, movedTo string) [12]int64 {
	var bitboards [12]uint64
	for _, sq := range pieceSquares(obs) {
		cell := obs.Pieces[sq]
		rankSlot := rankBitSlot(cell.Rank)
		if rankSlot < 0 {
			continue
		}
		sideSlot := 0
		if cell.Side != "white" {
			sideSlot = 6
		}
		sqTarget := sq
		if cell.UnitID.String() == movedUnitID {
			sqTarget = movedTo
		}
		idx := squareIndex(sqTarget)
		if idx >= 0 && idx < 64 {
			bitboards[sideSlot+rankSlot] |= 1 << idx
		}
	}
	var res [12]int64
	for i := 0; i < 12; i++ {
		res[i] = int64(bitboards[i])
	}
	return res
}

func packPlacementBoards(bitboards [12]int64) map[string]int64 {
	packed := make(map[string]int64)
	for slotIndex := 0; slotIndex < PlacementBoardSlots; slotIndex++ {
		baseSquare := slotIndex * PlacementSquaresPerSlot
		squaresInSlot := PlacementSquaresPerSlot
		if 64-baseSquare < squaresInSlot {
			squaresInSlot = 64 - baseSquare
		}
		var value int64
		for local := 0; local < squaresInSlot; local++ {
			square := baseSquare + local
			code := 0
			for boardSlot := 0; boardSlot < 12; boardSlot++ {
				raw := uint64(bitboards[boardSlot])
				if (raw>>square)&1 == 1 {
					code = boardSlot + 1
					break
				}
			}
			value |= int64(code) << (local * 4)
		}
		packed[fmt.Sprintf("leaderBoard%d", slotIndex)] = value
	}
	return packed
}

func unpackPlacementBoards(memory map[string]int64) [12]int64 {
	var bitboards [12]uint64
	for slotIndex := 0; slotIndex < PlacementBoardSlots; slotIndex++ {
		baseSquare := slotIndex * PlacementSquaresPerSlot
		squaresInSlot := PlacementSquaresPerSlot
		if 64-baseSquare < squaresInSlot {
			squaresInSlot = 64 - baseSquare
		}
		value := memory[fmt.Sprintf("leaderBoard%d", slotIndex)]
		for local := 0; local < squaresInSlot; local++ {
			code := (value >> (local * 4)) & 0xF
			if code != 0 {
				bitboards[code-1] |= 1 << (baseSquare + local)
			}
		}
	}
	var res [12]int64
	for i := 0; i < 12; i++ {
		res[i] = int64(bitboards[i])
	}
	return res
}

func leaderKind(obs *Observation, cell *Cell) int64 {
	if cell.Rank == "king" {
		return 6
	}
	if isCurrentBeaconBearer(obs, cell) {
		return int64(rankBitSlot(cell.Rank) + 1)
	}
	return 0
}

func leaderGuardKeys() []string {
	keys := []string{"leaderGuardActive", "leaderGuardFrom", "leaderGuardTo", "leaderGuardKind"}
	for slot := 0; slot < PlacementBoardSlots; slot++ {
		keys = append(keys, fmt.Sprintf("leaderBoard%d", slot))
	}
	return keys
}

func clearLeaderGuard(memory map[string]int64) {
	for _, key := range leaderGuardKeys() {
		delete(memory, key)
	}
}

func projectedCell(obs *Observation, square string) *Cell {
	if raw, ok := obs.Pieces[square]; ok {
		c := raw
		c.Square = square
		return &c
	}
	return nil
}

func leaderGuardMatches(obs *Observation, memory map[string]int64) bool {
	if memory["leaderGuardActive"] != 1 {
		return false
	}
	fromSquare := squareName(int(memory["leaderGuardFrom"]))
	toSquare := squareName(int(memory["leaderGuardTo"]))

	sourceCell := projectedCell(obs, fromSquare)
	if sourceCell != nil && sourceCell.Side != obs.Side {
		sourceCell = nil
	}
	if sourceCell != nil && leaderKind(obs, sourceCell) == memory["leaderGuardKind"] {
		expectedPre := unpackPlacementBoards(memory)
		sideSlot := 0
		if sourceCell.Side != "white" {
			sideSlot = 6
		}
		boardSlot := sideSlot + int(memory["leaderGuardKind"]) - 1
		raw := uint64(expectedPre[boardSlot])
		raw ^= (uint64(1) << memory["leaderGuardFrom"]) | (uint64(1) << memory["leaderGuardTo"])
		expectedPre[boardSlot] = int64(raw)
		return placementBitboards(obs, "", "") == expectedPre
	}

	destinationCell := projectedCell(obs, toSquare)
	if destinationCell != nil && destinationCell.Side != obs.Side {
		destinationCell = nil
	}
	if destinationCell == nil || leaderKind(obs, destinationCell) != memory["leaderGuardKind"] {
		return false
	}
	return placementBitboards(obs, "", "") == unpackPlacementBoards(memory)
}

type leaderGuard struct {
	from int64
	to   int64
	kind int64
}

func activeLeaderGuard(obs *Observation, memory map[string]int64) *leaderGuard {
	if !leaderGuardMatches(obs, memory) {
		return nil
	}
	return &leaderGuard{
		from: memory["leaderGuardFrom"],
		to:   memory["leaderGuardTo"],
		kind: memory["leaderGuardKind"],
	}
}

func leaderReverseForbidden(obs *Observation, guard *leaderGuard, cell *Cell, destination string) bool {
	if guard == nil {
		return false
	}
	if cell.Square != squareName(int(guard.to)) || destination != squareName(int(guard.from)) {
		return false
	}
	if leaderKind(obs, cell) != guard.kind {
		return false
	}
	return cellThreatenedCount(cell) <= 0
}

func quietLeaderIntent(obs *Observation, intent *Intent) *Cell {
	if intent == nil || intent.Kind != "move" || intent.Promotion != "" {
		return nil
	}
	cell := projectedCell(obs, intent.From)
	if cell != nil && cell.Side != obs.Side {
		cell = nil
	}
	if cell == nil || cell.Convoy || leaderKind(obs, cell) == 0 {
		return nil
	}
	enemyBySquare := make(map[string]*Cell)
	for sq, piece := range obs.Pieces {
		if piece.Side != obs.Side {
			p := piece
			p.Square = sq
			enemyBySquare[sq] = &p
		}
	}
	boardMock := &boardContext{enemyBySquare: enemyBySquare}
	if !isQuietMove(obs, boardMock, cell, intent.To) {
		return nil
	}
	return cell
}

func quietOrdinaryIntent(obs *Observation, intent *Intent) *Cell {
	if intent == nil || intent.Kind != "move" || intent.Promotion != "" {
		return nil
	}
	cell := projectedCell(obs, intent.From)
	if cell != nil && cell.Side != obs.Side {
		cell = nil
	}
	if cell == nil || cell.Convoy || leaderKind(obs, cell) != 0 {
		return nil
	}
	enemyBySquare := make(map[string]*Cell)
	for sq, piece := range obs.Pieces {
		if piece.Side != obs.Side {
			p := piece
			p.Square = sq
			enemyBySquare[sq] = &p
		}
	}
	boardMock := &boardContext{enemyBySquare: enemyBySquare}
	if !isQuietMove(obs, boardMock, cell, intent.To) {
		return nil
	}
	return cell
}

func buildMemory(obs *Observation, memory map[string]int64, intent *Intent) map[string]int64 {
	updatedMemory := make(map[string]int64)
	for k, v := range memory {
		updatedMemory[k] = v
	}
	if updatedMemory["leaderGuardActive"] == 1 && !leaderGuardMatches(obs, updatedMemory) {
		clearLeaderGuard(updatedMemory)
	}
	fromIndex := int64(NoSquareIndex)
	toIndex := int64(NoSquareIndex)
	if intent != nil {
		fromIndex = int64(squareIndex(intent.From))
		toIndex = int64(squareIndex(intent.To))
	}

	stillFrozen := memory != nil && memory["revision"] == obs.Revision
	cursor := int64(0)
	if stillFrozen {
		cursor = memory["refusedCursor"]
	} else {
		for slot := 0; slot < RefusedSetSize; slot++ {
			updatedMemory[fmt.Sprintf("refused%d", slot)] = int64(NoSquareIndex)
		}
	}
	updatedMemory[fmt.Sprintf("refused%d", cursor)] = fromIndex
	updatedMemory["refusedCursor"] = (cursor + 1) % int64(RefusedSetSize)

	updatedMemory["revision"] = obs.Revision
	updatedMemory["lastFrom"] = fromIndex
	updatedMemory["lastTo"] = toIndex
	if intent != nil && intent.Kind == "action" {
		updatedMemory["actions"] = memory["actions"] + 1
	} else if intent != nil && intent.Kind == "move" {
		updatedMemory["moves"] = memory["moves"] + 1
		updatedMemory["focusFrom"] = fromIndex + 1
	}

	leader := quietLeaderIntent(obs, intent)
	if leader != nil {
		updatedMemory["leaderGuardActive"] = 1
		updatedMemory["leaderGuardFrom"] = fromIndex
		updatedMemory["leaderGuardTo"] = toIndex
		updatedMemory["leaderGuardKind"] = leaderKind(obs, leader)
		bitboards := placementBitboards(obs, leader.UnitID.String(), intent.To)
		for k, v := range packPlacementBoards(bitboards) {
			updatedMemory[k] = v
		}
	}

	quietOrdinary := quietOrdinaryIntent(obs, intent)
	if quietOrdinary != nil {
		updatedMemory["lastQuietTo"] = toIndex
		for slot := QuietVacatedSquares - 1; slot > 0; slot-- {
			val, ok := memory[fmt.Sprintf("quietVacated%d", slot-1)]
			if !ok {
				val = int64(NoSquareIndex)
			}
			updatedMemory[fmt.Sprintf("quietVacated%d", slot)] = val
		}
		updatedMemory["quietVacated0"] = fromIndex
	} else if intent != nil && intent.Kind == "move" {
		updatedMemory["lastQuietTo"] = int64(NoSquareIndex)
		for slot := 0; slot < QuietVacatedSquares; slot++ {
			updatedMemory[fmt.Sprintf("quietVacated%d", slot)] = int64(NoSquareIndex)
		}
	} else {
		updatedMemory["lastQuietTo"] = int64(NoSquareIndex)
	}

	return updatedMemory
}

func applyRepeatPenalty(obs *Observation, b *boardContext, params *BotParams, memory map[string]int64, proposals []proposal) []proposal {
	recentlyVacated := make(map[int]bool)
	for slot := 0; slot < QuietVacatedSquares; slot++ {
		sq, ok := memory[fmt.Sprintf("quietVacated%d", slot)]
		if !ok {
			sq = int64(NoSquareIndex)
		}
		if sq >= 0 {
			recentlyVacated[int(sq)] = true
		}
	}
	cycleKeys := make(map[string]bool)
	for _, p := range proposals {
		cell := b.ownBySquare[p.intent.From]
		destination := p.intent.To
		if p.intent.Kind == "move" && p.intent.Promotion == "" && cell != nil && destination != "" &&
			!cell.Convoy && cell.Rank != "king" && !isCurrentBeaconBearer(obs, cell) &&
			isQuietMove(obs, b, cell, destination) &&
			cellThreatenedCount(cell) == 0 &&
			recentlyVacated[squareIndex(destination)] {
			cycleKeys[p.key] = true
		}
	}
	if len(cycleKeys) > 0 {
		otherViable := false
		for _, p := range proposals {
			candidate := b.ownBySquare[p.intent.From]
			if !cycleKeys[p.key] && candidate != nil && !candidate.Convoy && candidate.Rank != "king" &&
				!isCurrentBeaconBearer(obs, candidate) && p.score >= params.PassBelow {
				otherViable = true
				break
			}
		}
		if otherViable {
			var nonCycle []proposal
			for _, p := range proposals {
				if !cycleKeys[p.key] {
					nonCycle = append(nonCycle, p)
				}
			}
			proposals = nonCycle
		}
	}

	lastToVal, ok := memory["lastQuietTo"]
	if !ok {
		lastToVal = int64(NoSquareIndex)
	}
	if lastToVal < 0 {
		return proposals
	}
	repeatedCell := b.ownBySquare[squareName(int(lastToVal))]
	if repeatedCell == nil || repeatedCell.Convoy || repeatedCell.Rank == "king" || isCurrentBeaconBearer(obs, repeatedCell) {
		return proposals
	}
	otherViable := false
	for _, p := range proposals {
		candidate := b.ownBySquare[p.intent.From]
		if candidate != nil && candidate.UnitID.String() != repeatedCell.UnitID.String() && !candidate.Convoy &&
			candidate.Rank != "king" && !isCurrentBeaconBearer(obs, candidate) && p.score >= params.PassBelow {
			otherViable = true
			break
		}
	}
	if !otherViable {
		return proposals
	}
	for i := range proposals {
		p := &proposals[i]
		actorID := ""
		if uid, ok := p.actor.(UnitID); ok {
			actorID = uid.String()
		} else if str, ok := p.actor.(string); ok {
			actorID = str
		}
		if actorID != repeatedCell.UnitID.String() {
			continue
		}
		destination := p.intent.To
		if isQuietMove(obs, b, repeatedCell, destination) && cellThreatenedCount(repeatedCell) == 0 {
			penalty := -params.Advance
			p.score += addTerm(&p.terms, "repeatPenalty", penalty, "quiet")
		}
	}
	return proposals
}

type unitPriorityEntry struct {
	cell     *Cell
	priority float64
}

func moveProposals(obs *Observation, b *boardContext, params *BotParams, memory map[string]int64) []proposal {
	if len(b.moveActionableUnits) == 0 {
		return nil
	}

	var rankedUnits []unitPriorityEntry
	for _, cell := range b.moveActionableUnits {
		rankedUnits = append(rankedUnits, unitPriorityEntry{
			cell:     cell,
			priority: unitPriority(obs, b, params, cell),
		})
	}
	sort.Slice(rankedUnits, func(i, j int) bool {
		if rankedUnits[i].priority != rankedUnits[j].priority {
			return rankedUnits[i].priority > rankedUnits[j].priority
		}
		return squareIndex(rankedUnits[i].cell.Square) < squareIndex(rankedUnits[j].cell.Square)
	})

	breadth := params.Breadth
	if breadth <= 0 || breadth > len(rankedUnits) {
		breadth = len(rankedUnits)
	}

	var proposals []proposal
	leaderGuard := activeLeaderGuard(obs, memory)
	for _, entry := range rankedUnits[:breadth] {
		cell := entry.cell
		destinations := obs.Legal[cell.Square]
		var candidates []proposal
		for _, destination := range destinations {
			if destination == cell.Square {
				continue
			}
			if cell.Charging != nil && destination == cell.Charging.Square {
				continue
			}
			if !captureChoiceAvailable(obs, b, cell, destination) {
				continue
			}
			if leaderReverseForbidden(obs, leaderGuard, cell, destination) {
				continue
			}
			candidates = append(candidates, scoreMove(obs, b, params, memory, cell, destination))
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].score != candidates[j].score {
				return candidates[i].score > candidates[j].score
			}
			return candidates[i].key < candidates[j].key
		})
		spread := params.CandidateSpread
		if spread <= 0 || spread > len(candidates) {
			spread = len(candidates)
		}
		proposals = append(proposals, candidates[:spread]...)
	}
	return applyRepeatPenalty(obs, b, params, memory, proposals)
}
