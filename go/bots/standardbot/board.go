// Copyright 2026 Sneat.app

package standardbot

import (
	"sort"
)

// PARITY-FUNCTION: SBP-F-SQUARE-FILE
func squareFile(square string) int {
	if len(square) < 2 { // PARITY-BRANCH: SBP-B-SQUARE-FILE-1
		return 0
	}
	return int(square[0] - 'a')
}

// PARITY-FUNCTION: SBP-F-SQUARE-RANK-NUMBER
func squareRankNumber(square string) int {
	if len(square) < 2 { // PARITY-BRANCH: SBP-B-SQUARE-RANK-NUMBER-1
		return 0
	}
	return int(square[1] - '1')
}

// PARITY-FUNCTION: SBP-F-SQUARE-INDEX
func squareIndex(square string) int {
	if len(square) < 2 { // PARITY-BRANCH: SBP-B-SQUARE-INDEX-1
		return 0
	}
	return squareFile(square)*BoardFiles + squareRankNumber(square)
}

// PARITY-FUNCTION: SBP-F-SQUARE-NAME
func squareName(index int) string {
	if index < 0 || index > MaxSquareIndex { // PARITY-BRANCH: SBP-B-SQUARE-NAME-1
		return ""
	}
	fileNumber := index / BoardFiles
	rankNumber := index % BoardFiles
	return string([]byte{byte('a' + fileNumber), byte('1' + rankNumber)})
}

// PARITY-FUNCTION: SBP-F-CHEBYSHEV-DISTANCE
func chebyshevDistance(firstSquare, secondSquare string) int {
	f1, r1 := squareFile(firstSquare), squareRankNumber(firstSquare)
	f2, r2 := squareFile(secondSquare), squareRankNumber(secondSquare)
	df := f1 - f2
	if df < 0 { // PARITY-BRANCH: SBP-B-CHEBYSHEV-DISTANCE-1
		df = -df
	}
	dr := r1 - r2
	if dr < 0 { // PARITY-BRANCH: SBP-B-CHEBYSHEV-DISTANCE-2
		dr = -dr
	}
	if df > dr { // PARITY-BRANCH: SBP-B-CHEBYSHEV-DISTANCE-3
		return df
	}
	return dr
}

// PARITY-FUNCTION: SBP-F-FORWARD-PROGRESS
func forwardProgress(side, square string) int {
	r := squareRankNumber(square)
	if side == "white" { // PARITY-BRANCH: SBP-B-FORWARD-PROGRESS-1
		return r
	}
	return LastRankIndex - r
}

// PARITY-FUNCTION: SBP-F-IS-PROMOTION-SQUARE
func isPromotionSquare(side, square string) bool {
	return forwardProgress(side, square) == LastRankIndex
}

// PARITY-FUNCTION: SBP-F-RANK-VALUE
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

// PARITY-FUNCTION: SBP-F-CELL-VALUE
func cellValue(cell *Cell) float64 {
	if cell == nil { // PARITY-BRANCH: SBP-B-CELL-VALUE-1
		return 0.0
	}
	value := rankValue(cell.Rank)
	if cell.KingCargo { // PARITY-BRANCH: SBP-B-CELL-VALUE-2
		value += KingCargoValue
	}
	if cell.Ghost { // PARITY-BRANCH: SBP-B-CELL-VALUE-3
		value *= GhostDiscount
	}
	return value
}

// PARITY-FUNCTION: SBP-F-DISTANCE-TO-NEAREST-ENEMY
func distanceToNearestEnemy(square string, enemyCells []*Cell) int {
	closest := UnreachableDistance
	for _, enemy := range enemyCells {
		gap := chebyshevDistance(square, enemy.Square)
		if gap < closest { // PARITY-BRANCH: SBP-B-DISTANCE-TO-NEAREST-ENEMY-1
			closest = gap
		}
	}
	return closest
}

// PARITY-FUNCTION: SBP-F-DISTANCE-TO-NEAREST-SQUARE
func distanceToNearestSquare(square string, candidateSquares []string) int {
	closest := UnreachableDistance
	for _, cand := range candidateSquares {
		gap := chebyshevDistance(square, cand)
		if gap < closest { // PARITY-BRANCH: SBP-B-DISTANCE-TO-NEAREST-SQUARE-1
			closest = gap
		}
	}
	return closest
}

// PARITY-FUNCTION: SBP-F-GUARDED-COUNT
func guardedCount(fact *CandidateFact) int {
	if fact == nil { // PARITY-BRANCH: SBP-B-GUARDED-COUNT-1
		return 0
	}
	return len(fact.GuardedBy)
}

// PARITY-FUNCTION: SBP-F-THREATENED-COUNT
func threatenedCount(fact *CandidateFact) int {
	if fact == nil { // PARITY-BRANCH: SBP-B-THREATENED-COUNT-1
		return 0
	}
	return len(fact.ThreatenedBy)
}

// PARITY-FUNCTION: SBP-F-GUARDED-COUNT
func cellGuardedCount(cell *Cell) int {
	if cell == nil { // PARITY-BRANCH: SBP-B-GUARDED-COUNT-2
		return 0
	}
	return len(cell.GuardedBy)
}

// PARITY-FUNCTION: SBP-F-THREATENED-COUNT
func cellThreatenedCount(cell *Cell) int {
	if cell == nil { // PARITY-BRANCH: SBP-B-THREATENED-COUNT-2
		return 0
	}
	return len(cell.ThreatenedBy)
}

// PARITY-FUNCTION: SBP-F-SUPPORTED-AND-UNTHREATENED
func supportedAndUnthreatened(fact *CandidateFact) bool {
	return guardedCount(fact) > 0 && threatenedCount(fact) == 0
}

// PARITY-FUNCTION: SBP-F-IS-SAFE-SUBJECT
func isSafeSubject(fact *CandidateFact) bool {
	return guardedCount(fact) > 0 || threatenedCount(fact) == 0
}

// PARITY-FUNCTION: SBP-F-PIECE-SQUARES
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

// PARITY-FUNCTION: SBP-F-BEACON-AGGRESSION
func beaconAggression(obs *Observation, params *BotParams) float64 {
	return params.BeaconAggression
}

// PARITY-FUNCTION: SBP-F-IS-QUIET-MOVE
func isQuietMove(obs *Observation, b *boardContext, cell *Cell, destination string) bool {
	if b.enemyBySquare[destination] != nil { // PARITY-BRANCH: SBP-B-IS-QUIET-MOVE-1
		return false
	}
	if ep, ok := obs.EnPassant[cell.Square]; ok && ep[destination] != "" { // PARITY-BRANCH: SBP-B-IS-QUIET-MOVE-2
		return false
	}
	return true
}

// PARITY-FUNCTION: SBP-F-PROTECTION-FACTOR
func protectionFactor(fact *CandidateFact) float64 {
	gc := guardedCount(fact)
	res := float64(gc) / 2.0
	if res > 1.0 { // PARITY-BRANCH: SBP-B-PROTECTION-FACTOR-1
		return 1.0
	}
	return res
}

// PARITY-FUNCTION: SBP-F-DELIVERY-SQUARE-NEIGHBORS
func deliverySquareNeighbors(square string) []string {
	f := squareFile(square)
	r := squareRankNumber(square)
	var neighbors []string
	for df := -1; df <= 1; df++ {
		for dr := -1; dr <= 1; dr++ {
			if df == 0 && dr == 0 { // PARITY-BRANCH: SBP-B-DELIVERY-SQUARE-NEIGHBORS-1
				continue
			}
			nf, nr := f+df, r+dr
			if nf < 0 || nf > 7 || nr < 0 || nr > 7 { // PARITY-BRANCH: SBP-B-DELIVERY-SQUARE-NEIGHBORS-2
				continue
			}
			neighbors = append(neighbors, string([]byte{byte('a' + nf), byte('1' + nr)}))
		}
	}
	return neighbors
}

// PARITY-FUNCTION: SBP-F-DELIVERY-LANE-BLOCKERS
func deliveryLaneBlockers(deliverySquares []string, ownBySquare map[string]*Cell, enemyBySquare map[string]*Cell) []string {
	blockers := make(map[string]bool)
	for _, target := range deliverySquares {
		if enemyBySquare[target] != nil { // PARITY-BRANCH: SBP-B-DELIVERY-LANE-BLOCKERS-1
			return nil
		}
		if ownBySquare[target] != nil { // PARITY-BRANCH: SBP-B-DELIVERY-LANE-BLOCKERS-2
			blockers[target] = true
			continue
		}
		neighbors := deliverySquareNeighbors(target)
		if len(neighbors) == 0 { // PARITY-BRANCH: SBP-B-DELIVERY-LANE-BLOCKERS-3
			return nil
		}
		for _, neighbor := range neighbors {
			if enemyBySquare[neighbor] != nil { // PARITY-BRANCH: SBP-B-DELIVERY-LANE-BLOCKERS-4
				return nil
			}
			if ownBySquare[neighbor] == nil { // PARITY-BRANCH: SBP-B-DELIVERY-LANE-BLOCKERS-5
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

// PARITY-FUNCTION: SBP-F-BUILD-BOARD
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
		if cellCopy.Side == obs.Side { // PARITY-BRANCH: SBP-B-BUILD-BOARD-1
			b.own = append(b.own, &cellCopy)
			b.ownBySquare[square] = &cellCopy
		} else {
			b.enemy = append(b.enemy, &cellCopy)
			b.enemyBySquare[square] = &cellCopy
		}
	}

	for _, cell := range b.own {
		if cell.TargetLocked { // PARITY-BRANCH: SBP-B-BUILD-BOARD-2
			b.lockedUnits[cell.UnitID.String()] = true
		}
	}

	allCells := append(append([]*Cell(nil), b.own...), b.enemy...)
	for _, cell := range allCells {
		if cell.InterrogationRemainingMs != nil { // PARITY-BRANCH: SBP-B-BUILD-BOARD-3
			if cell.Side == obs.Side { // PARITY-BRANCH: SBP-B-BUILD-BOARD-4
				ownInterrogationTargets[cell.UnitID.String()] = true
			} else {
				enemyInterrogationActive = true
			}
		}
	}

	for _, cell := range b.own {
		uid := cell.UnitID.String()
		if cell.Profession == "engineer" && cell.Grade == "master" { // PARITY-BRANCH: SBP-B-BUILD-BOARD-5
			hasMasterEngineer = true
		}
		if cell.Rank == "king" && !cell.Convoy { // PARITY-BRANCH: SBP-B-BUILD-BOARD-6
			b.kingCell = cell
		}
		if cell.Charging != nil { // PARITY-BRANCH: SBP-B-BUILD-BOARD-7
			b.chargingUnits[uid] = true
			b.busyUnits[uid] = true
			target := b.enemyBySquare[cell.Charging.Square]
			if target != nil && target.Rank == "king" && !target.Ghost { // PARITY-BRANCH: SBP-B-BUILD-BOARD-8
				b.protectedChargingUnits[uid] = true
			} else {
				replaceableChargingUnits[uid] = true
			}
		}
		if cell.Training != nil || cell.ForgingRemainingMs > 0 || cell.RecoveryRemainingMs > 0 { // PARITY-BRANCH: SBP-B-BUILD-BOARD-9
			b.busyUnits[uid] = true
			nonrouteBusyUnits[uid] = true
		}
		if ownInterrogationTargets[uid] { // PARITY-BRANCH: SBP-B-BUILD-BOARD-10
			b.busyUnits[uid] = true
			nonrouteBusyUnits[uid] = true
		}
	}

	if enemyInterrogationActive && b.kingCell != nil { // PARITY-BRANCH: SBP-B-BUILD-BOARD-11
		kingUID := b.kingCell.UnitID.String()
		b.busyUnits[kingUID] = true
		nonrouteBusyUnits[kingUID] = true
	}

	for _, cell := range b.own {
		if !b.busyUnits[cell.UnitID.String()] { // PARITY-BRANCH: SBP-B-BUILD-BOARD-12
			b.actionableUnits = append(b.actionableUnits, cell)
		}
	}

	maxActiveCommands := obs.Rules.MaxActiveCommands
	if maxActiveCommands <= 0 { // PARITY-BRANCH: SBP-B-BUILD-BOARD-13
		maxActiveCommands = UnlimitedActiveCommands
	}

	var replaceableNow []*Cell
	for _, cell := range b.own {
		uid := cell.UnitID.String()
		if replaceableChargingUnits[uid] && !nonrouteBusyUnits[uid] { // PARITY-BRANCH: SBP-B-BUILD-BOARD-14
			replaceableNow = append(replaceableNow, cell)
		}
	}

	if len(b.chargingUnits) >= maxActiveCommands { // PARITY-BRANCH: SBP-B-BUILD-BOARD-15
		b.moveActionableUnits = replaceableNow
	} else {
		b.moveActionableUnits = append(append([]*Cell(nil), b.actionableUnits...), replaceableNow...)
	}

	b.kingThreatened = b.kingCell != nil && cellThreatenedCount(b.kingCell) > 0
	b.needsFirstMasterEngineer = obs.Rules.VeteranProgression && !hasMasterEngineer
	b.blockingBase = deliveryLaneBlockers(obs.DeliverySquares, b.ownBySquare, b.enemyBySquare)

	return b
}
