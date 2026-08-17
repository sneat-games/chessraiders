// Copyright 2026 Sneat.app

package standardbot

import (
	"sort"
)

func squareFile(square string) int {
	if len(square) < 2 {
		return 0
	}
	return int(square[0] - 'a')
}

func squareRankNumber(square string) int {
	if len(square) < 2 {
		return 0
	}
	return int(square[1] - '1')
}

func squareIndex(square string) int {
	if len(square) < 2 {
		return 0
	}
	return squareFile(square)*BoardFiles + squareRankNumber(square)
}

func squareName(index int) string {
	if index < 0 || index > MaxSquareIndex {
		return ""
	}
	fileNumber := index / BoardFiles
	rankNumber := index % BoardFiles
	return string([]byte{byte('a' + fileNumber), byte('1' + rankNumber)})
}

func chebyshevDistance(firstSquare, secondSquare string) int {
	f1, r1 := squareFile(firstSquare), squareRankNumber(firstSquare)
	f2, r2 := squareFile(secondSquare), squareRankNumber(secondSquare)
	df := f1 - f2
	if df < 0 {
		df = -df
	}
	dr := r1 - r2
	if dr < 0 {
		dr = -dr
	}
	if df > dr {
		return df
	}
	return dr
}

func forwardProgress(side, square string) int {
	r := squareRankNumber(square)
	if side == "white" {
		return r
	}
	return LastRankIndex - r
}

func isPromotionSquare(side, square string) bool {
	return forwardProgress(side, square) == LastRankIndex
}

func rankValue(rank string) float64 {
	switch rank {
	case "pawn":
		return PawnValue
	case "knight":
		return KnightValue
	case "bishop":
		return BishopValue
	case "rook":
		return RookValue
	case "queen":
		return QueenValue
	case "king":
		return KingValue
	default:
		return 0.0
	}
}

func cellValue(cell *Cell) float64 {
	if cell == nil {
		return 0.0
	}
	value := rankValue(cell.Rank)
	if cell.KingCargo {
		value += KingCargoValue
	}
	if cell.Ghost {
		value *= GhostDiscount
	}
	return value
}

func distanceToNearestEnemy(square string, enemyCells []*Cell) int {
	closest := UnreachableDistance
	for _, enemy := range enemyCells {
		gap := chebyshevDistance(square, enemy.Square)
		if gap < closest {
			closest = gap
		}
	}
	return closest
}

func distanceToNearestSquare(square string, candidateSquares []string) int {
	closest := UnreachableDistance
	for _, cand := range candidateSquares {
		gap := chebyshevDistance(square, cand)
		if gap < closest {
			closest = gap
		}
	}
	return closest
}

func guardedCount(fact *CandidateFact) int {
	if fact == nil {
		return 0
	}
	return len(fact.GuardedBy)
}

func threatenedCount(fact *CandidateFact) int {
	if fact == nil {
		return 0
	}
	return len(fact.ThreatenedBy)
}

func cellGuardedCount(cell *Cell) int {
	if cell == nil {
		return 0
	}
	return len(cell.GuardedBy)
}

func cellThreatenedCount(cell *Cell) int {
	if cell == nil {
		return 0
	}
	return len(cell.ThreatenedBy)
}

func supportedAndUnthreatened(fact *CandidateFact) bool {
	return guardedCount(fact) > 0 && threatenedCount(fact) == 0
}

func isSafeSubject(fact *CandidateFact) bool {
	return guardedCount(fact) > 0 || threatenedCount(fact) == 0
}

func pieceSquares(obs *Observation) []string {
	squares := make([]string, 0, len(obs.Pieces))
	for sq := range obs.Pieces {
		squares = append(squares, sq)
	}
	sort.Slice(squares, func(i, j int) bool {
		return squareIndex(squares[i]) < squareIndex(squares[j])
	})
	return squares
}

func beaconAggression(obs *Observation, params *BotParams) float64 {
	return params.BeaconAggression
}

func isQuietMove(obs *Observation, b *boardContext, cell *Cell, destination string) bool {
	if b.enemyBySquare[destination] != nil {
		return false
	}
	if ep, ok := obs.EnPassant[cell.Square]; ok && ep[destination] != "" {
		return false
	}
	return true
}

func protectionFactor(fact *CandidateFact) float64 {
	gc := guardedCount(fact)
	res := float64(gc) / 2.0
	if res > 1.0 {
		return 1.0
	}
	return res
}

func deliverySquareNeighbors(square string) []string {
	f := squareFile(square)
	r := squareRankNumber(square)
	var neighbors []string
	for df := -1; df <= 1; df++ {
		for dr := -1; dr <= 1; dr++ {
			if df == 0 && dr == 0 {
				continue
			}
			nf, nr := f+df, r+dr
			if nf < 0 || nf > 7 || nr < 0 || nr > 7 {
				continue
			}
			neighbors = append(neighbors, string([]byte{byte('a' + nf), byte('1' + nr)}))
		}
	}
	return neighbors
}

func deliveryLaneBlockers(deliverySquares []string, ownBySquare map[string]*Cell, enemyBySquare map[string]*Cell) []string {
	blockers := make(map[string]bool)
	for _, target := range deliverySquares {
		if enemyBySquare[target] != nil {
			return nil
		}
		if ownBySquare[target] != nil {
			blockers[target] = true
			continue
		}
		neighbors := deliverySquareNeighbors(target)
		if len(neighbors) == 0 {
			return nil
		}
		for _, neighbor := range neighbors {
			if enemyBySquare[neighbor] != nil {
				return nil
			}
			if ownBySquare[neighbor] == nil {
				return nil // a real, open approach exists
			}
			blockers[neighbor] = true
		}
	}
	var res []string
	for k := range blockers {
		res = append(res, k)
	}
	sort.Slice(res, func(i, j int) bool {
		return squareIndex(res[i]) < squareIndex(res[j])
	})
	return res
}

type boardContext struct {
	side                     string
	own                      []*Cell
	enemy                    []*Cell
	allBySquare              map[string]*Cell
	ownBySquare              map[string]*Cell
	enemyBySquare            map[string]*Cell
	lockedUnits              map[string]bool
	busyUnits                map[string]bool
	chargingUnits            map[string]bool
	protectedChargingUnits   map[string]bool
	actionableUnits          []*Cell
	moveActionableUnits      []*Cell
	needsFirstMasterEngineer bool
	kingCell                 *Cell
	kingThreatened           bool
	deliverySquares          []string
	convoyHome               map[string]int
	blockingBase             []string
}

func buildBoard(obs *Observation) *boardContext {
	b := &boardContext{
		side:                   obs.Side,
		allBySquare:            make(map[string]*Cell),
		ownBySquare:            make(map[string]*Cell),
		enemyBySquare:          make(map[string]*Cell),
		lockedUnits:            make(map[string]bool),
		busyUnits:              make(map[string]bool),
		chargingUnits:          make(map[string]bool),
		protectedChargingUnits: make(map[string]bool),
		convoyHome:             obs.ConvoyHome,
		deliverySquares:        obs.DeliverySquares,
	}

	replaceableChargingUnits := make(map[string]bool)
	nonrouteBusyUnits := make(map[string]bool)
	ownInterrogationTargets := make(map[string]bool)
	enemyInterrogationActive := false
	hasMasterEngineer := false

	orderedSquares := pieceSquares(obs)
	for _, square := range orderedSquares {
		raw := obs.Pieces[square]
		cellCopy := raw
		cellCopy.Square = square
		b.allBySquare[square] = &cellCopy
		if cellCopy.Side == obs.Side {
			b.own = append(b.own, &cellCopy)
			b.ownBySquare[square] = &cellCopy
		} else {
			b.enemy = append(b.enemy, &cellCopy)
			b.enemyBySquare[square] = &cellCopy
		}
	}

	for _, cell := range b.own {
		if cell.TargetLocked {
			b.lockedUnits[cell.UnitID.String()] = true
		}
	}

	allCells := append(append([]*Cell(nil), b.own...), b.enemy...)
	for _, cell := range allCells {
		if cell.InterrogationRemainingMs != nil {
			if cell.Side == obs.Side {
				ownInterrogationTargets[cell.UnitID.String()] = true
			} else {
				enemyInterrogationActive = true
			}
		}
	}

	for _, cell := range b.own {
		uid := cell.UnitID.String()
		if cell.Profession == "engineer" && cell.Grade == "master" {
			hasMasterEngineer = true
		}
		if cell.Rank == "king" && !cell.Convoy {
			b.kingCell = cell
		}
		if cell.Charging != nil {
			b.chargingUnits[uid] = true
			b.busyUnits[uid] = true
			target := b.enemyBySquare[cell.Charging.Square]
			if target != nil && target.Rank == "king" && !target.Ghost {
				b.protectedChargingUnits[uid] = true
			} else {
				replaceableChargingUnits[uid] = true
			}
		}
		if cell.Training != nil || cell.ForgingRemainingMs > 0 || cell.RecoveryRemainingMs > 0 {
			b.busyUnits[uid] = true
			nonrouteBusyUnits[uid] = true
		}
		if ownInterrogationTargets[uid] {
			b.busyUnits[uid] = true
			nonrouteBusyUnits[uid] = true
		}
	}

	if enemyInterrogationActive && b.kingCell != nil {
		kingUID := b.kingCell.UnitID.String()
		b.busyUnits[kingUID] = true
		nonrouteBusyUnits[kingUID] = true
	}

	for _, cell := range b.own {
		if !b.busyUnits[cell.UnitID.String()] {
			b.actionableUnits = append(b.actionableUnits, cell)
		}
	}

	maxActiveCommands := obs.Rules.MaxActiveCommands
	if maxActiveCommands <= 0 {
		maxActiveCommands = UnlimitedActiveCommands
	}

	var replaceableNow []*Cell
	for _, cell := range b.own {
		uid := cell.UnitID.String()
		if replaceableChargingUnits[uid] && !nonrouteBusyUnits[uid] {
			replaceableNow = append(replaceableNow, cell)
		}
	}

	if len(b.chargingUnits) >= maxActiveCommands {
		b.moveActionableUnits = replaceableNow
	} else {
		b.moveActionableUnits = append(append([]*Cell(nil), b.actionableUnits...), replaceableNow...)
	}

	b.kingThreatened = b.kingCell != nil && cellThreatenedCount(b.kingCell) > 0
	b.needsFirstMasterEngineer = obs.Rules.VeteranProgression && !hasMasterEngineer
	b.blockingBase = deliveryLaneBlockers(obs.DeliverySquares, b.ownBySquare, b.enemyBySquare)

	return b
}
