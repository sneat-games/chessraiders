// Copyright 2026 Sneat.app

package standardbot

import (
	"fmt"
)

type proposal struct {
	intent Intent
	score  float64
	terms  []Term
	actor  any
	key    string
}

// PARITY-FUNCTION: SBP-F-ADD-TERM
func addTerm(terms *[]Term, term string, value float64, detail string) float64 {
	if value == 0.0 { // PARITY-BRANCH: SBP-B-ADD-TERM-1
		return 0.0
	}
	*terms = append(*terms, Term{
		Term:   term,
		Value:  value,
		Detail: detail,
	})
	return value
}

// PARITY-FUNCTION: SBP-GO-STRUCT-F-CONTAINS-STRING
func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s { // PARITY-BRANCH: SBP-GO-STRUCT-B-CONTAINS-STRING-1
			return true
		}
	}
	return false
}

// PARITY-FUNCTION: SBP-F-OUTCOMES-AT
func outcomesAt(obs *Observation, sourceSquare, destination string) *DestinationOutcomes {
	if byDest, ok := obs.Affordability[sourceSquare]; ok { // PARITY-BRANCH: SBP-B-OUTCOMES-AT-1
		if out, ok2 := byDest[destination]; ok2 { // PARITY-BRANCH: SBP-B-OUTCOMES-AT-2
			return &out
		}
	}
	return nil
}

// PARITY-FUNCTION: SBP-F-CANDIDATE-AT
func candidateAt(obs *Observation, sourceSquare, destination string) *CandidateFact {
	if byDest, ok := obs.Candidates[sourceSquare]; ok { // PARITY-BRANCH: SBP-B-CANDIDATE-AT-1
		if fact, ok2 := byDest[destination]; ok2 { // PARITY-BRANCH: SBP-B-CANDIDATE-AT-2
			return &fact
		}
	}
	return nil
}

// PARITY-FUNCTION: SBP-F-HAS-CANDIDATE
func hasCandidate(obs *Observation, sourceSquare, destination string) bool {
	if byDest, ok := obs.Candidates[sourceSquare]; ok { // PARITY-BRANCH: SBP-B-HAS-CANDIDATE-1
		_, ok2 := byDest[destination]
		return ok2
	}
	return false
}

// PARITY-FUNCTION: SBP-F-CAPTURE-EXPECTED-SUCCESS
func captureExpectedSuccess(outcome *CaptureOutcome) float64 {
	if outcome == nil || !outcome.OddsKnown { // PARITY-BRANCH: SBP-B-CAPTURE-EXPECTED-SUCCESS-1
		return 1.0
	}
	return float64(outcome.Odds.Success) / 100.0
}

// PARITY-FUNCTION: SBP-F-EFFECTIVE-CAPTURE-TARGET
func effectiveCaptureTarget(obs *Observation, b *boardContext, cell *Cell, destination string) *Cell {
	targetCell := b.enemyBySquare[destination]
	if targetCell != nil { // PARITY-BRANCH: SBP-B-EFFECTIVE-CAPTURE-TARGET-1
		return targetCell
	}
	if cell.Convoy || cell.Rank != "pawn" { // PARITY-BRANCH: SBP-B-EFFECTIVE-CAPTURE-TARGET-2
		return nil
	}
	if bySrc, ok := obs.EnPassant[cell.Square]; ok { // PARITY-BRANCH: SBP-B-EFFECTIVE-CAPTURE-TARGET-3
		if victimSquare, ok2 := bySrc[destination]; ok2 && victimSquare != "" { // PARITY-BRANCH: SBP-B-EFFECTIVE-CAPTURE-TARGET-4
			return b.enemyBySquare[victimSquare]
		}
	}
	return nil
}

// PARITY-FUNCTION: SBP-F-CAPTURE-CHOICE-AVAILABLE
func captureChoiceAvailable(obs *Observation, b *boardContext, cell *Cell, destination string) bool {
	targetCell := effectiveCaptureTarget(obs, b, cell, destination)
	if targetCell == nil { // PARITY-BRANCH: SBP-B-CAPTURE-CHOICE-AVAILABLE-1
		return true
	}
	outcomes := outcomesAt(obs, cell.Square, destination)
	if targetCell.Rank == "king" || targetCell.Convoy { // PARITY-BRANCH: SBP-B-CAPTURE-CHOICE-AVAILABLE-2
		return outcomes != nil && outcomes.Capture != nil && outcomes.Capture.Affordable
	}
	if obs.Rules.AllowsKill && outcomes != nil && outcomes.Kill != nil && outcomes.Kill.Affordable { // PARITY-BRANCH: SBP-B-CAPTURE-CHOICE-AVAILABLE-3
		return true
	}
	if obs.Rules.AllowsCapture && outcomes != nil && outcomes.Capture != nil && outcomes.Capture.Affordable { // PARITY-BRANCH: SBP-B-CAPTURE-CHOICE-AVAILABLE-4
		return true
	}
	return false
}

// PARITY-FUNCTION: SBP-F-LEADER-SUPPORT
func leaderSupport(obs *Observation, b *boardContext, leader *Cell, destination string) float64 {
	support := 0.0
	destinationProgress := forwardProgress(b.side, destination)
	leaderUID := leader.UnitID.String()
	for _, ally := range b.own {
		if ally.UnitID.String() == leaderUID { // PARITY-BRANCH: SBP-B-LEADER-SUPPORT-1
			continue
		}
		dist := chebyshevDistance(destination, ally.Square)
		if dist == 0 { // PARITY-BRANCH: SBP-B-LEADER-SUPPORT-2
			continue
		}
		allyProgress := forwardProgress(b.side, ally.Square)
		directionWeight := 0.0
		if allyProgress > destinationProgress { // PARITY-BRANCH: SBP-B-LEADER-SUPPORT-3
			directionWeight = 1.0
		} else if allyProgress == destinationProgress { // PARITY-BRANCH: SBP-B-LEADER-SUPPORT-4
			directionWeight = 0.5
		}
		support += directionWeight / float64(dist)
	}
	res := support / LeaderSupportSaturation
	if res > 1.0 { // PARITY-BRANCH: SBP-B-LEADER-SUPPORT-5
		return 1.0
	}
	return res
}

// PARITY-FUNCTION: SBP-F-CURRENT-MORALE-NEED
func currentMoraleNeed(obs *Observation) int {
	if obs.CaptureMoraleNeed != nil { // PARITY-BRANCH: SBP-B-CURRENT-MORALE-NEED-1
		return *obs.CaptureMoraleNeed
	}
	needed := obs.OwnManaged
	for _, byDest := range obs.Affordability {
		for _, outcomes := range byDest {
			if outcomes.Capture != nil && outcomes.Capture.RequiredMorale > needed { // PARITY-BRANCH: SBP-B-CURRENT-MORALE-NEED-2
				needed = outcomes.Capture.RequiredMorale
			}
		}
	}
	return needed
}

// PARITY-FUNCTION: SBP-F-POST-MOVE-MORALE
func postMoveMorale(obs *Observation, b *boardContext, fromSquare, destination string) int {
	res := forwardProgress(b.side, destination) - obs.OwnMoralePenalty
	if res < 0 { // PARITY-BRANCH: SBP-B-POST-MOVE-MORALE-1
		return 0
	}
	return res
}

// PARITY-FUNCTION: SBP-F-IS-CURRENT-BEACON-BEARER
func isCurrentBeaconBearer(obs *Observation, cell *Cell) bool {
	return obs.Beacon.Lifecycle == "deployed" && obs.Beacon.BearerSquare == cell.Square
}

// PARITY-FUNCTION: SBP-F-CAN-PROMOTE-NEXT-MOVE
func canPromoteNextMove(side string, candidate *CandidateFact) bool {
	if candidate == nil { // PARITY-BRANCH: SBP-B-CAN-PROMOTE-NEXT-MOVE-1
		return false
	}
	for _, nextDestination := range candidate.NextPossibleMoves {
		if isPromotionSquare(side, nextDestination) { // PARITY-BRANCH: SBP-B-CAN-PROMOTE-NEXT-MOVE-2
			return true
		}
	}
	return false
}

// PARITY-FUNCTION: SBP-F-SUPPORT-MATERIAL
func supportMaterial(b *boardContext, squares []string) float64 {
	material := 0.0
	for _, sq := range squares {
		cell := b.ownBySquare[sq]
		if cell != nil { // PARITY-BRANCH: SBP-B-SUPPORT-MATERIAL-1
			rv := rankValue(cell.Rank)
			if rv > RookValue { // PARITY-BRANCH: SBP-B-SUPPORT-MATERIAL-2
				rv = RookValue
			}
			material += rv
		}
	}
	if material > SupportMaterialCap { // PARITY-BRANCH: SBP-B-SUPPORT-MATERIAL-3
		return SupportMaterialCap
	}
	return material
}

// PARITY-FUNCTION: SBP-F-CAPTURE-BACKING-COUNT
func captureBackingCount(targetCell *Cell, moverSquare string) int {
	count := 0
	for _, sq := range targetCell.ThreatenedBy {
		if sq != moverSquare { // PARITY-BRANCH: SBP-B-CAPTURE-BACKING-COUNT-1
			count++
		}
	}
	return count
}

// PARITY-FUNCTION: SBP-F-GUARD-CHANGES
func guardChanges(b *boardContext, cell *Cell, candidate *CandidateFact) ([]string, []string) {
	oldSquare := cell.Square
	before := cell.Guards
	after := candidate.Guards
	var newlyGuarded []string
	var soleGuardLost []string

	for _, square := range after {
		target := b.ownBySquare[square]
		if target != nil && !containsString(target.GuardedBy, oldSquare) { // PARITY-BRANCH: SBP-B-GUARD-CHANGES-1
			newlyGuarded = append(newlyGuarded, square)
		}
	}
	for _, square := range before {
		if containsString(after, square) { // PARITY-BRANCH: SBP-B-GUARD-CHANGES-2
			continue
		}
		target := b.ownBySquare[square]
		if target != nil && len(target.GuardedBy) == 1 && target.GuardedBy[0] == oldSquare { // PARITY-BRANCH: SBP-B-GUARD-CHANGES-3
			soleGuardLost = append(soleGuardLost, square)
		}
	}
	return newlyGuarded, soleGuardLost
}

// PARITY-FUNCTION: SBP-F-INBOUND-SUPPORT-MATERIAL
func inboundSupportMaterial(cell *Cell, candidate *CandidateFact) float64 {
	count := guardedCount(candidate)
	if count > 2 { // PARITY-BRANCH: SBP-B-INBOUND-SUPPORT-MATERIAL-1
		count = 2
	}
	rv := rankValue(cell.Rank)
	if rv > RookValue { // PARITY-BRANCH: SBP-B-INBOUND-SUPPORT-MATERIAL-2
		rv = RookValue
	}
	return float64(count) * rv / RookValue
}

// PARITY-FUNCTION: SBP-F-FORMATION-LEADER-PRIORITY
func formationLeaderPriority(obs *Observation, b *boardContext, params *BotParams, cell *Cell) float64 {
	if cell.Convoy { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-1
		return 0.0
	}
	destinations := obs.Legal[cell.Square]
	if cell.Rank == "king" { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-2
		if params.MoralePush <= 0 { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-3
			return 0.0
		}
		needed := currentMoraleNeed(obs)
		current := postMoveMorale(obs, b, cell.Square, cell.Square)
		if obs.OwnMorale-needed >= LeaderExcessMorale { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-4
			for _, destination := range destinations {
				fact := candidateAt(obs, cell.Square, destination)
				after := postMoveMorale(obs, b, cell.Square, destination)
				if isQuietMove(obs, b, cell, destination) &&
					hasCandidate(obs, cell.Square, destination) && fact != nil && fact.DestinationVisible &&
					supportedAndUnthreatened(fact) &&
					after < current && after >= needed+1 { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-5
					return UnitPriorityFormationLeader
				}
			}
			return 0.0
		}
		if current >= needed+1 { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-6
			return 0.0
		}
		for _, destination := range destinations {
			fact := candidateAt(obs, cell.Square, destination)
			after := postMoveMorale(obs, b, cell.Square, destination)
			if isQuietMove(obs, b, cell, destination) &&
				hasCandidate(obs, cell.Square, destination) && fact != nil && fact.DestinationVisible &&
				supportedAndUnthreatened(fact) &&
				after > current && after <= needed+2 { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-7
				return UnitPriorityFormationLeader
			}
		}
		return 0.0
	}
	if !isCurrentBeaconBearer(obs, cell) || params.BeaconAggression <= 0 { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-8
		return 0.0
	}
	currentSupport := leaderSupport(obs, b, cell, cell.Square)
	for _, destination := range destinations {
		fact := candidateAt(obs, cell.Square, destination)
		if !(cell.Rank == "pawn" && isPromotionSquare(b.side, destination)) && isQuietMove(obs, b, cell, destination) &&
			hasCandidate(obs, cell.Square, destination) && fact != nil && fact.DestinationVisible &&
			supportedAndUnthreatened(fact) &&
			leaderSupport(obs, b, cell, destination) > currentSupport { // PARITY-BRANCH: SBP-B-FORMATION-LEADER-PRIORITY-9
			return UnitPriorityFormationLeader
		}
	}
	return 0.0
}

// PARITY-FUNCTION: SBP-F-UNIT-PRIORITY
func unitPriority(obs *Observation, b *boardContext, params *BotParams, cell *Cell) float64 {
	priority := -float64(distanceToNearestEnemy(cell.Square, b.enemy))
	if cell.Convoy && cell.KingCargo { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-1
		priority += UnitPriorityKingCargo
	} else if cell.Convoy && cell.CargoCount > 0 && params.Prisoner > 0 { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-2
		priority += UnitPriorityPrisonerCargo
	}
	if params.TargetLock > 0 && b.lockedUnits[cell.UnitID.String()] { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-3
		priority += UnitPriorityLockedBonus
	}
	if params.KingSafety > 0 && b.kingThreatened { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-4
		if cell.Rank == "king" { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-5
			priority += UnitPriorityKingItself
		} else if b.kingCell != nil && chebyshevDistance(cell.Square, b.kingCell.Square) <= NearKingRadius { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-6
			priority += UnitPriorityNearKing
		}
	}
	priority += formationLeaderPriority(obs, b, params, cell)
	if !cell.Convoy && cell.Rank != "king" && !isCurrentBeaconBearer(obs, cell) { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-7
		for _, enemy := range b.enemy {
			if enemy.Rank != "king" || enemy.Ghost { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-8
				continue
			}
			if containsString(obs.Legal[cell.Square], enemy.Square) && captureChoiceAvailable(obs, b, cell, enemy.Square) { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-9
				priority += KingValue
				break
			}
			for _, destination := range obs.Legal[cell.Square] {
				fact := candidateAt(obs, cell.Square, destination)
				if isQuietMove(obs, b, cell, destination) &&
					hasCandidate(obs, cell.Square, destination) && fact != nil && fact.DestinationVisible &&
					supportedAndUnthreatened(fact) &&
					containsString(fact.NextPossibleMoves, enemy.Square) { // PARITY-BRANCH: SBP-B-UNIT-PRIORITY-10
					priority += UnitPriorityKingVisibleAttack
					break
				}
			}
		}
	}
	priority += rankValue(cell.Rank) / RankPriorityScale
	return priority
}

// PARITY-FUNCTION: SBP-F-SCORE-MOVE
func scoreMove(obs *Observation, b *boardContext, params *BotParams, memory map[string]int64, cell *Cell, destination string) proposal {
	targetCell := effectiveCaptureTarget(obs, b, cell, destination)
	quietMove := isQuietMove(obs, b, cell, destination)
	candidateKnown := hasCandidate(obs, cell.Square, destination)
	candidate := candidateAt(obs, cell.Square, destination)
	score := 0.0
	capturedValue := 0.0
	var terms []Term

	successChance := 1.0
	captureChoice := ""
	if targetCell != nil && targetCell.Rank != "king" && !targetCell.Convoy { // PARITY-BRANCH: SBP-B-SCORE-MOVE-1
		outcomes := outcomesAt(obs, cell.Square, destination)
		if params.Prisoner > 0 && obs.Rules.AllowsCapture && outcomes != nil && outcomes.Capture != nil && outcomes.Capture.Affordable { // PARITY-BRANCH: SBP-B-SCORE-MOVE-2
			captureChoice = "capture"
		} else if obs.Rules.AllowsKill { // PARITY-BRANCH: SBP-B-SCORE-MOVE-3
			captureChoice = "kill"
		}
		if captureChoice != "" && outcomes != nil { // PARITY-BRANCH: SBP-B-SCORE-MOVE-4
			if captureChoice == "capture" && outcomes.Capture != nil { // PARITY-BRANCH: SBP-B-SCORE-MOVE-5
				successChance = captureExpectedSuccess(outcomes.Capture)
			} else if captureChoice == "kill" && outcomes.Kill != nil { // PARITY-BRANCH: SBP-B-SCORE-MOVE-6
				successChance = captureExpectedSuccess(outcomes.Kill)
			}
		}
	}

	// Material
	if targetCell != nil { // PARITY-BRANCH: SBP-B-SCORE-MOVE-7
		capturedValue = cellValue(targetCell)
		materialGain := capturedValue * successChance * params.Material
		if b.needsFirstMasterEngineer && cell.Rank == "pawn" && cell.Convoy && cell.CargoCount > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-8
			materialGain /= float64(cell.CargoCount + 1)
		}
		score += addTerm(&terms, "material", materialGain, "capture")
		if targetCell.Rank == "king" && !targetCell.Ghost { // PARITY-BRANCH: SBP-B-SCORE-MOVE-9
			score += addTerm(&terms, "kingHunt", params.Advance, "visible")
		}
		if params.Prisoner > 0 && targetCell.Rank != "king" && !targetCell.Convoy { // PARITY-BRANCH: SBP-B-SCORE-MOVE-10
			score += addTerm(&terms, "prisoner", CaptureAliveBonus*params.Prisoner*successChance, "alive")
			outcomes := outcomesAt(obs, cell.Square, destination)
			if b.needsFirstMasterEngineer && cell.Rank == "pawn" && !cell.Convoy &&
				outcomes != nil && outcomes.Capture != nil && outcomes.Capture.Affordable &&
				(captureBackingCount(targetCell, cell.Square) > 0 || cellGuardedCount(targetCell) == 0) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-11
				score += addTerm(&terms, "prisoner", ValueVeteranBootstrap*params.Prisoner, "bootstrap")
			}
		}
	}

	// Safety
	var postThreat, postGuarded int
	if candidateKnown && candidate != nil { // PARITY-BRANCH: SBP-B-SCORE-MOVE-12
		postThreat = threatenedCount(candidate)
		postGuarded = guardedCount(candidate)
	} else if targetCell != nil { // PARITY-BRANCH: SBP-B-SCORE-MOVE-13
		postThreat = cellGuardedCount(targetCell)
		postGuarded = captureBackingCount(targetCell, cell.Square)
	}
	postSafetyKnown := candidateKnown || targetCell != nil

	if postThreat > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-14
		risk := rankValue(cell.Rank)
		if cell.KingCargo { // PARITY-BRANCH: SBP-B-SCORE-MOVE-15
			risk += KingCargoEscortRisk
		}
		if capturedValue >= risk { // PARITY-BRANCH: SBP-B-SCORE-MOVE-16
			risk *= SafeTradeDiscount
		} else if postGuarded > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-17
			risk *= RecaptureDiscount
		}
		score += addTerm(&terms, "safety", -risk*params.Safety, "risk")
	}

	// Tempo
	if params.Tempo > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-18
		chargeMs := obs.Rules.PieceChargeMs[cell.Rank]
		if chargeMs > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-19
			score += addTerm(&terms, "tempo", -(float64(chargeMs)/MillisecondsPerSecond)*params.Tempo, "charge")
		}
	}
	activeCharge := cell.Charging
	if activeCharge != nil && destination != activeCharge.Square { // PARITY-BRANCH: SBP-B-SCORE-MOVE-20
		remainingSeconds := float64(activeCharge.RemainingMs) / MillisecondsPerSecond
		urgency := RouteReplaceUrgencyValue / (1.0 + remainingSeconds)
		score += addTerm(&terms, "tempo", -urgency, "replaceCharge")
	}

	// Win condition / Delivery
	if cell.Convoy && cell.KingCargo { // PARITY-BRANCH: SBP-B-SCORE-MOVE-21
		if containsString(b.deliverySquares, destination) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-22
			score += addTerm(&terms, "delivery", DeliveryBonus, "wins")
		} else {
			progressFrom := cell.Square
			if activeCharge != nil { // PARITY-BRANCH: SBP-B-SCORE-MOVE-23
				progressFrom = activeCharge.Square
			}
			hereCost, okHere := b.convoyHome[progressFrom]
			if !okHere { // PARITY-BRANCH: SBP-B-SCORE-MOVE-24
				hereCost = UnreachablePathCost
			}
			thereCost, okThere := b.convoyHome[destination]
			if !okThere { // PARITY-BRANCH: SBP-B-SCORE-MOVE-25
				thereCost = UnreachablePathCost
			}
			if hereCost < UnreachablePathCost { // PARITY-BRANCH: SBP-B-SCORE-MOVE-26
				if thereCost >= UnreachablePathCost { // PARITY-BRANCH: SBP-B-SCORE-MOVE-27
					score += addTerm(&terms, "delivery", -DeliveryStepValue*params.Delivery, "offRoute")
				} else {
					score += addTerm(&terms, "delivery", float64(hereCost-thereCost)*DeliveryStepValue*params.Delivery, "closer")
				}
			} else {
				progress := distanceToNearestSquare(progressFrom, b.deliverySquares) - distanceToNearestSquare(destination, b.deliverySquares)
				score += addTerm(&terms, "delivery", float64(progress)*DeliveryStepValue*params.Delivery, "drift")
			}
		}
	} else if cell.Convoy && cell.CargoCount > 0 && params.Prisoner > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-28
		prisonerRank := cell.Rank
		if obs.Rules.CargoBasedDelivery { // PARITY-BRANCH: SBP-B-SCORE-MOVE-29
			prisonerRank = "pawn"
		}
		baseSquares := obs.Rules.BaseSquares[prisonerRank]
		progress := distanceToNearestSquare(cell.Square, baseSquares) - distanceToNearestSquare(destination, baseSquares)
		homeward := float64(progress) * PrisonerStepValue * params.Prisoner
		if b.needsFirstMasterEngineer && cell.Rank == "pawn" { // PARITY-BRANCH: SBP-B-SCORE-MOVE-30
			homeward *= float64(cell.CargoCount)
		}
		score += addTerm(&terms, "prisoner", homeward, "escort")
	}

	// Delivery blocker cleanup
	if !cell.Convoy && params.Delivery > 0 && containsString(b.blockingBase, cell.Square) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-31
		rankVal := rankValue(cell.Rank)
		if rankVal > QueenValue { // PARITY-BRANCH: SBP-B-SCORE-MOVE-32
			rankVal = QueenValue
		}
		score += addTerm(&terms, "delivery", (UnblockBaseValue-rankVal*UnblockValueSpread)*params.Delivery, "unblock")
	}

	// Positional pressure
	if params.Advance > 0 && !cell.Convoy { // PARITY-BRANCH: SBP-B-SCORE-MOVE-33
		gain := float64(forwardProgress(b.side, destination) - forwardProgress(b.side, cell.Square))
		positionalKnownAndSupported := !quietMove || (candidateKnown && candidate != nil && candidate.DestinationVisible && guardedCount(candidate) > 0)
		if quietMove && (!candidateKnown || candidate == nil || !candidate.DestinationVisible) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-34
			score += addTerm(&terms, "safety", -UnknownQuietPenalty*params.Safety, "unknownQuiet")
		} else if quietMove && guardedCount(candidate) <= 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-35
			score += addTerm(&terms, "safety", -UnsupportedQuietPenalty*params.Safety, "unsupportedQuiet")
		}
		ordinaryPiece := cell.Rank != "king" && !isCurrentBeaconBearer(obs, cell)
		if cell.Rank == "pawn" && positionalKnownAndSupported && ordinaryPiece { // PARITY-BRANCH: SBP-B-SCORE-MOVE-36
			score += addTerm(&terms, "advance", gain*AdvancePawnMultiplier*params.Advance, "pawn")
			if isPromotionSquare(b.side, destination) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-37
				score += addTerm(&terms, "advance", PromotionBonus*params.Advance, "promotion")
			} else if quietMove && candidateKnown && candidate != nil && candidate.DestinationVisible &&
				supportedAndUnthreatened(candidate) && canPromoteNextMove(b.side, candidate) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-38
				score += addTerm(&terms, "advance", PromotionNextBonus*params.Advance*protectionFactor(candidate), "promotionNext")
			}
		} else if ordinaryPiece && positionalKnownAndSupported { // PARITY-BRANCH: SBP-B-SCORE-MOVE-39
			score += addTerm(&terms, "advance", gain*params.Advance, "piece")
		}
		if quietMove && positionalKnownAndSupported &&
			(cell.Rank == "knight" || cell.Rank == "bishop" || cell.Rank == "rook" || cell.Rank == "queen") &&
			ordinaryPiece && !cell.Moved && gain > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-40
			score += addTerm(&terms, "develop", DevelopFirstForwardValue*params.Advance*protectionFactor(candidate), "firstForward")
		}
		if quietMove && positionalKnownAndSupported && ordinaryPiece && candidate != nil && candidate.PatrolGain > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-41
			patrol := float64(candidate.PatrolGain)
			if patrol > float64(PatrolGainCap) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-42
				patrol = float64(PatrolGainCap)
			}
			score += addTerm(&terms, "coverage", patrol*PatrolGainValue*params.Advance*protectionFactor(candidate), "patrol")
		}
		if quietMove && candidateKnown && candidate != nil && !isPromotionSquare(b.side, destination) &&
			candidate.DestinationVisible && threatenedCount(candidate) == 0 && ordinaryPiece { // PARITY-BRANCH: SBP-B-SCORE-MOVE-43
			newlyGuarded, soleGuardLost := guardChanges(b, cell, candidate)
			netOutboundMaterial := supportMaterial(b, newlyGuarded) - supportMaterial(b, soleGuardLost)
			if netOutboundMaterial > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-44
				score += addTerm(&terms, "coverage", netOutboundMaterial*GuardsValue*params.Advance, "guards")
			} else if netOutboundMaterial < 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-45
				score += addTerm(&terms, "safety", netOutboundMaterial*SoleGuardLostValue*params.Safety, "soleGuardLost")
			}
			inboundMaterial := inboundSupportMaterial(cell, candidate)
			if inboundMaterial > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-46
				score += addTerm(&terms, "safety", inboundMaterial*GuardedByValue*params.Safety, "guardedBy")
			}
		}
	}

	// King hunt
	if !cell.Convoy && quietMove && candidateKnown && candidate != nil &&
		candidate.DestinationVisible && supportedAndUnthreatened(candidate) &&
		cell.Rank != "king" && !isCurrentBeaconBearer(obs, cell) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-47
		for _, enemy := range b.enemy {
			if enemy.Rank == "king" && !enemy.Ghost && containsString(candidate.NextPossibleMoves, enemy.Square) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-48
				score += addTerm(&terms, "kingHunt", KingVisibleAttackBonus, "visible")
				break
			}
		}
	}

	// Target lock dodge
	if params.TargetLock > 0 && b.lockedUnits[cell.UnitID.String()] { // PARITY-BRANCH: SBP-B-SCORE-MOVE-49
		score += addTerm(&terms, "targetLock", TargetLockDodgeValue*params.TargetLock, "dodge")
		if postSafetyKnown && postThreat == 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-50
			score += addTerm(&terms, "targetLock", TargetLockSafeValue*params.TargetLock, "safeDodge")
		}
	}

	// King safety
	if params.KingSafety > 0 && b.kingThreatened { // PARITY-BRANCH: SBP-B-SCORE-MOVE-51
		if cell.Rank == "king" && !cell.Convoy && postSafetyKnown && postThreat == 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-52
			score += addTerm(&terms, "kingSafety", params.KingSafety, "escape")
		}
		if targetCell != nil && b.kingCell != nil && containsString(targetCell.Threatens, b.kingCell.Square) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-53
			score += addTerm(&terms, "kingSafety", params.KingSafety*KingGuardBonus, "guard")
		}
	}

	// Morale push
	if params.MoralePush > 0 && cell.Rank == "king" && !cell.Convoy { // PARITY-BRANCH: SBP-B-SCORE-MOVE-54
		gain := float64(forwardProgress(b.side, destination) - forwardProgress(b.side, cell.Square))
		kingSafeAfter := false
		if candidateKnown && candidate != nil { // PARITY-BRANCH: SBP-B-SCORE-MOVE-55
			kingSafeAfter = candidate.DestinationVisible && supportedAndUnthreatened(candidate)
		} else {
			kingSafeAfter = targetCell != nil && postThreat == 0
		}
		if gain > 0 && kingSafeAfter { // PARITY-BRANCH: SBP-B-SCORE-MOVE-56
			if candidateKnown { // PARITY-BRANCH: SBP-B-SCORE-MOVE-57
				afterMorale := postMoveMorale(obs, b, cell.Square, destination)
				excess := afterMorale - currentMoraleNeed(obs)
				guardStrength := leaderSupport(obs, b, cell, destination)
				if excess >= LeaderExcessMorale { // PARITY-BRANCH: SBP-B-SCORE-MOVE-58
					score += addTerm(&terms, "moralePush", -gain*MoralePushValue*params.MoralePush, "excessAdvance")
				} else {
					score += addTerm(&terms, "moralePush", gain*MoralePushValue*params.MoralePush*guardStrength, "guardedAdvance")
				}
			}
		} else if candidateKnown && gain < 0 && kingSafeAfter { // PARITY-BRANCH: SBP-B-SCORE-MOVE-59
			afterMorale := postMoveMorale(obs, b, cell.Square, destination)
			needed := currentMoraleNeed(obs)
			if obs.OwnMorale-needed >= LeaderExcessMorale && afterMorale >= needed+1 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-60
				score += addTerm(&terms, "moralePush", -gain*MoralePushValue*params.MoralePush*LeaderRetreatValue, "excessRetreat")
			}
		}
		if postThreat > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-61
			score += addTerm(&terms, "safety", -KingValue*params.Safety, "kingIntoStrike")
		}
	}

	// Beacon bearer leadership
	if isCurrentBeaconBearer(obs, cell) && cell.Rank != "king" && !cell.Convoy && quietMove && params.BeaconAggression > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-62
		gain := forwardProgress(b.side, destination) - forwardProgress(b.side, cell.Square)
		if candidateKnown && candidate != nil && candidate.DestinationVisible && supportedAndUnthreatened(candidate) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-63
			supportGain := leaderSupport(obs, b, cell, destination) - leaderSupport(obs, b, cell, cell.Square)
			if supportGain > 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-64
				detail := "guardedAdvance"
				if gain <= 0 { // PARITY-BRANCH: SBP-B-SCORE-MOVE-65
					detail = "regroup"
				}
				score += addTerm(&terms, "beaconAggression", supportGain*params.BeaconAggression*protectionFactor(candidate), detail)
			}
		}
	}

	intent := Intent{
		Kind: "move",
		From: cell.Square,
		To:   destination,
	}
	if cell.Rank == "pawn" && !cell.Convoy && isPromotionSquare(b.side, destination) { // PARITY-BRANCH: SBP-B-SCORE-MOVE-66
		intent.Promotion = "queen"
	}
	if captureChoice != "" { // PARITY-BRANCH: SBP-B-SCORE-MOVE-67
		intent.Choice = captureChoice
	}

	return proposal{
		intent: intent,
		score:  score,
		terms:  terms,
		actor:  cell.UnitID,
		key:    fmt.Sprintf("move|%s|%s", cell.UnitID.String(), destination),
	}
}

// PARITY-FUNCTION: SBP-F-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL
func priorityCaptiveDeliveryProposal(obs *Observation, b *boardContext, params *BotParams) *proposal {
	if !b.needsFirstMasterEngineer { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-1
		return nil
	}
	for _, cell := range b.own {
		if !cell.Convoy || cell.CargoCount == 0 || cell.KingCargo || cell.Rank != "pawn" || b.busyUnits[cell.UnitID.String()] { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-2
			continue
		}
		if cell.NotBriefed { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-3
			continue
		}
		prisonerRank := cell.Rank
		if obs.Rules.CargoBasedDelivery { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-4
			prisonerRank = "pawn"
		}
		destinations := obs.Legal[cell.Square]
		if len(destinations) == 0 { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-5
			continue
		}
		baseSquares := obs.Rules.BaseSquares[prisonerRank]
		to := ""

		if containsString(baseSquares, cell.Square) { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-6
			fallback := ""
			for _, candidate := range destinations {
				fact := candidateAt(obs, cell.Square, candidate)
				if candidate == cell.Square || b.enemyBySquare[candidate] != nil ||
					!hasCandidate(obs, cell.Square, candidate) || fact == nil ||
					!fact.DestinationVisible || !isSafeSubject(fact) { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-7
					continue
				}
				if containsString(baseSquares, candidate) { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-8
					to = candidate
					break
				}
				if fallback == "" { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-9
					fallback = candidate
				}
			}
			if to == "" { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-10
				to = fallback
			}
		} else {
			here := distanceToNearestSquare(cell.Square, baseSquares)
			bestGain := 0
			for _, candidate := range destinations {
				fact := candidateAt(obs, cell.Square, candidate)
				if candidate == cell.Square || b.enemyBySquare[candidate] != nil ||
					!hasCandidate(obs, cell.Square, candidate) || fact == nil ||
					!fact.DestinationVisible || !isSafeSubject(fact) { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-11
					continue
				}
				gain := here - distanceToNearestSquare(candidate, baseSquares)
				if gain > bestGain { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-12
					bestGain = gain
					to = candidate
				}
			}
		}

		if to == "" { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-PROPOSAL-13
			continue
		}
		return &proposal{
			intent: Intent{Kind: "move", From: cell.Square, To: to},
			score:  PriorityCaptiveDeliveryScore,
			terms:  []Term{{Term: "prisoner", Value: PriorityCaptiveDeliveryScore, Detail: "priorityDelivery"}},
			actor:  cell.UnitID,
			key:    fmt.Sprintf("priority-captive-delivery|%s|%s", cell.UnitID.String(), to),
		}
	}
	return nil
}

// PARITY-FUNCTION: SBP-F-PRIORITY-CAPTIVE-DELIVERY-IN-FLIGHT
func priorityCaptiveDeliveryInFlight(obs *Observation, b *boardContext) bool {
	if !b.needsFirstMasterEngineer { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-IN-FLIGHT-1
		return false
	}
	for _, cell := range b.own {
		charging := cell.Charging
		if charging == nil || !cell.Convoy || cell.CargoCount == 0 || cell.KingCargo || cell.Rank != "pawn" { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-IN-FLIGHT-2
			continue
		}
		if b.allBySquare[charging.Square] != nil { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-IN-FLIGHT-3
			continue
		}
		prisonerRank := cell.Rank
		if obs.Rules.CargoBasedDelivery { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-IN-FLIGHT-4
			prisonerRank = "pawn"
		}
		baseSquares := obs.Rules.BaseSquares[prisonerRank]
		if containsString(baseSquares, cell.Square) { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-IN-FLIGHT-5
			return true
		}
		if distanceToNearestSquare(charging.Square, baseSquares) < distanceToNearestSquare(cell.Square, baseSquares) { // PARITY-BRANCH: SBP-B-PRIORITY-CAPTIVE-DELIVERY-IN-FLIGHT-6
			return true
		}
	}
	return false
}

// PARITY-FUNCTION: SBP-F-RANK-OPTIONS
func rankOptions(proposals []proposal, params *BotParams, count int) []Option {
	if count <= 0 { // PARITY-BRANCH: SBP-B-RANK-OPTIONS-1
		return nil
	}
	var chosen []proposal
	seenActors := make(map[string]bool)
	for _, p := range proposals {
		if p.score < params.PassBelow { // PARITY-BRANCH: SBP-B-RANK-OPTIONS-2
			break
		}
		actorKey := p.key
		if p.actor != nil { // PARITY-BRANCH: SBP-B-RANK-OPTIONS-3
			if uid, ok := p.actor.(UnitID); ok && !uid.IsZero() { // PARITY-BRANCH: SBP-B-RANK-OPTIONS-4
				actorKey = uid.String()
			} else if str, ok := p.actor.(string); ok && str != "" { // PARITY-BRANCH: SBP-B-RANK-OPTIONS-5
				actorKey = str
			}
		}
		if seenActors[actorKey] { // PARITY-BRANCH: SBP-B-RANK-OPTIONS-6
			continue
		}
		seenActors[actorKey] = true
		chosen = append(chosen, p)
		if len(chosen) >= count { // PARITY-BRANCH: SBP-B-RANK-OPTIONS-7
			break
		}
	}

	var options []Option
	rank := 0
	leaderScore := 0.0
	for i, p := range chosen {
		if i == 0 || p.score < leaderScore-TieBreakBand { // PARITY-BRANCH: SBP-B-RANK-OPTIONS-8
			rank = i + 1
			leaderScore = p.score
		}
		options = append(options, Option{
			Rank:   rank,
			Intent: p.intent,
			Score:  p.score,
			Terms:  p.terms,
		})
	}
	return options
}
