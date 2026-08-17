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

func addTerm(terms *[]Term, term string, value float64, detail string) float64 {
	if value == 0.0 {
		return 0.0
	}
	*terms = append(*terms, Term{
		Term:   term,
		Value:  value,
		Detail: detail,
	})
	return value
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func outcomesAt(obs *Observation, sourceSquare, destination string) *DestinationOutcomes {
	if byDest, ok := obs.Affordability[sourceSquare]; ok {
		if out, ok2 := byDest[destination]; ok2 {
			return &out
		}
	}
	return nil
}

func candidateAt(obs *Observation, sourceSquare, destination string) *CandidateFact {
	if byDest, ok := obs.Candidates[sourceSquare]; ok {
		if fact, ok2 := byDest[destination]; ok2 {
			return &fact
		}
	}
	return nil
}

func hasCandidate(obs *Observation, sourceSquare, destination string) bool {
	if byDest, ok := obs.Candidates[sourceSquare]; ok {
		_, ok2 := byDest[destination]
		return ok2
	}
	return false
}

func captureExpectedSuccess(outcome *CaptureOutcome) float64 {
	if outcome == nil || !outcome.OddsKnown {
		return 1.0
	}
	return float64(outcome.Odds.Success) / 100.0
}

func effectiveCaptureTarget(obs *Observation, b *boardContext, cell *Cell, destination string) *Cell {
	targetCell := b.enemyBySquare[destination]
	if targetCell != nil {
		return targetCell
	}
	if cell.Convoy || cell.Rank != "pawn" {
		return nil
	}
	if bySrc, ok := obs.EnPassant[cell.Square]; ok {
		if victimSquare, ok2 := bySrc[destination]; ok2 && victimSquare != "" {
			return b.enemyBySquare[victimSquare]
		}
	}
	return nil
}

func captureChoiceAvailable(obs *Observation, b *boardContext, cell *Cell, destination string) bool {
	targetCell := effectiveCaptureTarget(obs, b, cell, destination)
	if targetCell == nil {
		return true
	}
	outcomes := outcomesAt(obs, cell.Square, destination)
	if targetCell.Rank == "king" || targetCell.Convoy {
		return outcomes != nil && outcomes.Capture != nil && outcomes.Capture.Affordable
	}
	if obs.Rules.AllowsKill && outcomes != nil && outcomes.Kill != nil && outcomes.Kill.Affordable {
		return true
	}
	if obs.Rules.AllowsCapture && outcomes != nil && outcomes.Capture != nil && outcomes.Capture.Affordable {
		return true
	}
	return false
}

func leaderSupport(obs *Observation, b *boardContext, leader *Cell, destination string) float64 {
	support := 0.0
	destinationProgress := forwardProgress(b.side, destination)
	leaderUID := leader.UnitID.String()
	for _, ally := range b.own {
		if ally.UnitID.String() == leaderUID {
			continue
		}
		dist := chebyshevDistance(destination, ally.Square)
		if dist == 0 {
			continue
		}
		allyProgress := forwardProgress(b.side, ally.Square)
		directionWeight := 0.0
		if allyProgress > destinationProgress {
			directionWeight = 1.0
		} else if allyProgress == destinationProgress {
			directionWeight = 0.5
		}
		support += directionWeight / float64(dist)
	}
	res := support / LeaderSupportSaturation
	if res > 1.0 {
		return 1.0
	}
	return res
}

func currentMoraleNeed(obs *Observation) int {
	if obs.CaptureMoraleNeed != nil {
		return *obs.CaptureMoraleNeed
	}
	needed := obs.OwnManaged
	for _, byDest := range obs.Affordability {
		for _, outcomes := range byDest {
			if outcomes.Capture != nil && outcomes.Capture.RequiredMorale > needed {
				needed = outcomes.Capture.RequiredMorale
			}
		}
	}
	return needed
}

func postMoveMorale(obs *Observation, b *boardContext, fromSquare, destination string) int {
	res := forwardProgress(b.side, destination) - obs.OwnMoralePenalty
	if res < 0 {
		return 0
	}
	return res
}

func isCurrentBeaconBearer(obs *Observation, cell *Cell) bool {
	return obs.Beacon.Lifecycle == "deployed" && obs.Beacon.BearerSquare == cell.Square
}

func canPromoteNextMove(side string, candidate *CandidateFact) bool {
	if candidate == nil {
		return false
	}
	for _, nextDestination := range candidate.NextPossibleMoves {
		if isPromotionSquare(side, nextDestination) {
			return true
		}
	}
	return false
}

func supportMaterial(b *boardContext, squares []string) float64 {
	material := 0.0
	for _, sq := range squares {
		cell := b.ownBySquare[sq]
		if cell != nil {
			rv := rankValue(cell.Rank)
			if rv > RookValue {
				rv = RookValue
			}
			material += rv
		}
	}
	if material > SupportMaterialCap {
		return SupportMaterialCap
	}
	return material
}

func captureBackingCount(targetCell *Cell, moverSquare string) int {
	count := 0
	for _, sq := range targetCell.ThreatenedBy {
		if sq != moverSquare {
			count++
		}
	}
	return count
}

func guardChanges(b *boardContext, cell *Cell, candidate *CandidateFact) ([]string, []string) {
	oldSquare := cell.Square
	before := cell.Guards
	after := candidate.Guards
	var newlyGuarded []string
	var soleGuardLost []string

	for _, square := range after {
		target := b.ownBySquare[square]
		if target != nil && !containsString(target.GuardedBy, oldSquare) {
			newlyGuarded = append(newlyGuarded, square)
		}
	}
	for _, square := range before {
		if containsString(after, square) {
			continue
		}
		target := b.ownBySquare[square]
		if target != nil && len(target.GuardedBy) == 1 && target.GuardedBy[0] == oldSquare {
			soleGuardLost = append(soleGuardLost, square)
		}
	}
	return newlyGuarded, soleGuardLost
}

func inboundSupportMaterial(cell *Cell, candidate *CandidateFact) float64 {
	count := guardedCount(candidate)
	if count > 2 {
		count = 2
	}
	rv := rankValue(cell.Rank)
	if rv > RookValue {
		rv = RookValue
	}
	return float64(count) * rv / RookValue
}

func formationLeaderPriority(obs *Observation, b *boardContext, params *BotParams, cell *Cell) float64 {
	if cell.Convoy {
		return 0.0
	}
	destinations := obs.Legal[cell.Square]
	if cell.Rank == "king" {
		if params.MoralePush <= 0 {
			return 0.0
		}
		needed := currentMoraleNeed(obs)
		current := postMoveMorale(obs, b, cell.Square, cell.Square)
		if obs.OwnMorale-needed >= LeaderExcessMorale {
			for _, destination := range destinations {
				fact := candidateAt(obs, cell.Square, destination)
				after := postMoveMorale(obs, b, cell.Square, destination)
				if isQuietMove(obs, b, cell, destination) &&
					hasCandidate(obs, cell.Square, destination) && fact != nil && fact.DestinationVisible &&
					supportedAndUnthreatened(fact) &&
					after < current && after >= needed+1 {
					return UnitPriorityFormationLeader
				}
			}
			return 0.0
		}
		if current >= needed+1 {
			return 0.0
		}
		for _, destination := range destinations {
			fact := candidateAt(obs, cell.Square, destination)
			after := postMoveMorale(obs, b, cell.Square, destination)
			if isQuietMove(obs, b, cell, destination) &&
				hasCandidate(obs, cell.Square, destination) && fact != nil && fact.DestinationVisible &&
				supportedAndUnthreatened(fact) &&
				after > current && after <= needed+2 {
				return UnitPriorityFormationLeader
			}
		}
		return 0.0
	}
	if !isCurrentBeaconBearer(obs, cell) || params.BeaconAggression <= 0 {
		return 0.0
	}
	currentSupport := leaderSupport(obs, b, cell, cell.Square)
	for _, destination := range destinations {
		fact := candidateAt(obs, cell.Square, destination)
		if !(cell.Rank == "pawn" && isPromotionSquare(b.side, destination)) && isQuietMove(obs, b, cell, destination) &&
			hasCandidate(obs, cell.Square, destination) && fact != nil && fact.DestinationVisible &&
			supportedAndUnthreatened(fact) &&
			leaderSupport(obs, b, cell, destination) > currentSupport {
			return UnitPriorityFormationLeader
		}
	}
	return 0.0
}

func unitPriority(obs *Observation, b *boardContext, params *BotParams, cell *Cell) float64 {
	priority := -float64(distanceToNearestEnemy(cell.Square, b.enemy))
	if cell.Convoy && cell.KingCargo {
		priority += UnitPriorityKingCargo
	} else if cell.Convoy && cell.CargoCount > 0 && params.Prisoner > 0 {
		priority += UnitPriorityPrisonerCargo
	}
	if params.TargetLock > 0 && b.lockedUnits[cell.UnitID.String()] {
		priority += UnitPriorityLockedBonus
	}
	if params.KingSafety > 0 && b.kingThreatened {
		if cell.Rank == "king" {
			priority += UnitPriorityKingItself
		} else if b.kingCell != nil && chebyshevDistance(cell.Square, b.kingCell.Square) <= NearKingRadius {
			priority += UnitPriorityNearKing
		}
	}
	priority += formationLeaderPriority(obs, b, params, cell)
	if !cell.Convoy && cell.Rank != "king" && !isCurrentBeaconBearer(obs, cell) {
		for _, enemy := range b.enemy {
			if enemy.Rank != "king" || enemy.Ghost {
				continue
			}
			if containsString(obs.Legal[cell.Square], enemy.Square) && captureChoiceAvailable(obs, b, cell, enemy.Square) {
				priority += KingValue
				break
			}
			for _, destination := range obs.Legal[cell.Square] {
				fact := candidateAt(obs, cell.Square, destination)
				if isQuietMove(obs, b, cell, destination) &&
					hasCandidate(obs, cell.Square, destination) && fact != nil && fact.DestinationVisible &&
					supportedAndUnthreatened(fact) &&
					containsString(fact.NextPossibleMoves, enemy.Square) {
					priority += UnitPriorityKingVisibleAttack
					break
				}
			}
		}
	}
	priority += rankValue(cell.Rank) / RankPriorityScale
	return priority
}

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
	if targetCell != nil && targetCell.Rank != "king" && !targetCell.Convoy {
		outcomes := outcomesAt(obs, cell.Square, destination)
		if params.Prisoner > 0 && obs.Rules.AllowsCapture && outcomes != nil && outcomes.Capture != nil && outcomes.Capture.Affordable {
			captureChoice = "capture"
		} else if obs.Rules.AllowsKill {
			captureChoice = "kill"
		}
		if captureChoice != "" && outcomes != nil {
			if captureChoice == "capture" && outcomes.Capture != nil {
				successChance = captureExpectedSuccess(outcomes.Capture)
			} else if captureChoice == "kill" && outcomes.Kill != nil {
				successChance = captureExpectedSuccess(outcomes.Kill)
			}
		}
	}

	// Material
	if targetCell != nil {
		capturedValue = cellValue(targetCell)
		materialGain := capturedValue * successChance * params.Material
		if b.needsFirstMasterEngineer && cell.Rank == "pawn" && cell.Convoy && cell.CargoCount > 0 {
			materialGain /= float64(cell.CargoCount + 1)
		}
		score += addTerm(&terms, "material", materialGain, "capture")
		if targetCell.Rank == "king" && !targetCell.Ghost {
			score += addTerm(&terms, "kingHunt", params.Advance, "visible")
		}
		if params.Prisoner > 0 && targetCell.Rank != "king" && !targetCell.Convoy {
			score += addTerm(&terms, "prisoner", CaptureAliveBonus*params.Prisoner*successChance, "alive")
			outcomes := outcomesAt(obs, cell.Square, destination)
			if b.needsFirstMasterEngineer && cell.Rank == "pawn" && !cell.Convoy &&
				outcomes != nil && outcomes.Capture != nil && outcomes.Capture.Affordable &&
				(captureBackingCount(targetCell, cell.Square) > 0 || cellGuardedCount(targetCell) == 0) {
				score += addTerm(&terms, "prisoner", ValueVeteranBootstrap*params.Prisoner, "bootstrap")
			}
		}
	}

	// Safety
	var postThreat, postGuarded int
	if candidateKnown && candidate != nil {
		postThreat = threatenedCount(candidate)
		postGuarded = guardedCount(candidate)
	} else if targetCell != nil {
		postThreat = cellGuardedCount(targetCell)
		postGuarded = captureBackingCount(targetCell, cell.Square)
	}
	postSafetyKnown := candidateKnown || targetCell != nil

	if postThreat > 0 {
		risk := rankValue(cell.Rank)
		if cell.KingCargo {
			risk += KingCargoEscortRisk
		}
		if capturedValue >= risk {
			risk *= SafeTradeDiscount
		} else if postGuarded > 0 {
			risk *= RecaptureDiscount
		}
		score += addTerm(&terms, "safety", -risk*params.Safety, "risk")
	}

	// Tempo
	if params.Tempo > 0 {
		chargeMs := obs.Rules.PieceChargeMs[cell.Rank]
		if chargeMs > 0 {
			score += addTerm(&terms, "tempo", -(float64(chargeMs)/MillisecondsPerSecond)*params.Tempo, "charge")
		}
	}
	activeCharge := cell.Charging
	if activeCharge != nil && destination != activeCharge.Square {
		remainingSeconds := float64(activeCharge.RemainingMs) / MillisecondsPerSecond
		urgency := RouteReplaceUrgencyValue / (1.0 + remainingSeconds)
		score += addTerm(&terms, "tempo", -urgency, "replaceCharge")
	}

	// Win condition / Delivery
	if cell.Convoy && cell.KingCargo {
		if containsString(b.deliverySquares, destination) {
			score += addTerm(&terms, "delivery", DeliveryBonus, "wins")
		} else {
			progressFrom := cell.Square
			if activeCharge != nil {
				progressFrom = activeCharge.Square
			}
			hereCost, okHere := b.convoyHome[progressFrom]
			if !okHere {
				hereCost = UnreachablePathCost
			}
			thereCost, okThere := b.convoyHome[destination]
			if !okThere {
				thereCost = UnreachablePathCost
			}
			if hereCost < UnreachablePathCost {
				if thereCost >= UnreachablePathCost {
					score += addTerm(&terms, "delivery", -DeliveryStepValue*params.Delivery, "offRoute")
				} else {
					score += addTerm(&terms, "delivery", float64(hereCost-thereCost)*DeliveryStepValue*params.Delivery, "closer")
				}
			} else {
				progress := distanceToNearestSquare(progressFrom, b.deliverySquares) - distanceToNearestSquare(destination, b.deliverySquares)
				score += addTerm(&terms, "delivery", float64(progress)*DeliveryStepValue*params.Delivery, "drift")
			}
		}
	} else if cell.Convoy && cell.CargoCount > 0 && params.Prisoner > 0 {
		prisonerRank := cell.Rank
		if obs.Rules.CargoBasedDelivery {
			prisonerRank = "pawn"
		}
		baseSquares := obs.Rules.BaseSquares[prisonerRank]
		progress := distanceToNearestSquare(cell.Square, baseSquares) - distanceToNearestSquare(destination, baseSquares)
		homeward := float64(progress) * PrisonerStepValue * params.Prisoner
		if b.needsFirstMasterEngineer && cell.Rank == "pawn" {
			homeward *= float64(cell.CargoCount)
		}
		score += addTerm(&terms, "prisoner", homeward, "escort")
	}

	// Delivery blocker cleanup
	if !cell.Convoy && params.Delivery > 0 && containsString(b.blockingBase, cell.Square) {
		rankVal := rankValue(cell.Rank)
		if rankVal > QueenValue {
			rankVal = QueenValue
		}
		score += addTerm(&terms, "delivery", (UnblockBaseValue-rankVal*UnblockValueSpread)*params.Delivery, "unblock")
	}

	// Positional pressure
	if params.Advance > 0 && !cell.Convoy {
		gain := float64(forwardProgress(b.side, destination) - forwardProgress(b.side, cell.Square))
		positionalKnownAndSupported := !quietMove || (candidateKnown && candidate != nil && candidate.DestinationVisible && guardedCount(candidate) > 0)
		if quietMove && (!candidateKnown || candidate == nil || !candidate.DestinationVisible) {
			score += addTerm(&terms, "safety", -UnknownQuietPenalty*params.Safety, "unknownQuiet")
		} else if quietMove && guardedCount(candidate) <= 0 {
			score += addTerm(&terms, "safety", -UnsupportedQuietPenalty*params.Safety, "unsupportedQuiet")
		}
		ordinaryPiece := cell.Rank != "king" && !isCurrentBeaconBearer(obs, cell)
		if cell.Rank == "pawn" && positionalKnownAndSupported && ordinaryPiece {
			score += addTerm(&terms, "advance", gain*AdvancePawnMultiplier*params.Advance, "pawn")
			if isPromotionSquare(b.side, destination) {
				score += addTerm(&terms, "advance", PromotionBonus*params.Advance, "promotion")
			} else if quietMove && candidateKnown && candidate != nil && candidate.DestinationVisible &&
				supportedAndUnthreatened(candidate) && canPromoteNextMove(b.side, candidate) {
				score += addTerm(&terms, "advance", PromotionNextBonus*params.Advance*protectionFactor(candidate), "promotionNext")
			}
		} else if ordinaryPiece && positionalKnownAndSupported {
			score += addTerm(&terms, "advance", gain*params.Advance, "piece")
		}
		if quietMove && positionalKnownAndSupported &&
			(cell.Rank == "knight" || cell.Rank == "bishop" || cell.Rank == "rook" || cell.Rank == "queen") &&
			ordinaryPiece && !cell.Moved && gain > 0 {
			score += addTerm(&terms, "develop", DevelopFirstForwardValue*params.Advance*protectionFactor(candidate), "firstForward")
		}
		if quietMove && positionalKnownAndSupported && ordinaryPiece && candidate != nil && candidate.PatrolGain > 0 {
			patrol := float64(candidate.PatrolGain)
			if patrol > float64(PatrolGainCap) {
				patrol = float64(PatrolGainCap)
			}
			score += addTerm(&terms, "coverage", patrol*PatrolGainValue*params.Advance*protectionFactor(candidate), "patrol")
		}
		if quietMove && candidateKnown && candidate != nil && !isPromotionSquare(b.side, destination) &&
			candidate.DestinationVisible && threatenedCount(candidate) == 0 && ordinaryPiece {
			newlyGuarded, soleGuardLost := guardChanges(b, cell, candidate)
			netOutboundMaterial := supportMaterial(b, newlyGuarded) - supportMaterial(b, soleGuardLost)
			if netOutboundMaterial > 0 {
				score += addTerm(&terms, "coverage", netOutboundMaterial*GuardsValue*params.Advance, "guards")
			} else if netOutboundMaterial < 0 {
				score += addTerm(&terms, "safety", netOutboundMaterial*SoleGuardLostValue*params.Safety, "soleGuardLost")
			}
			inboundMaterial := inboundSupportMaterial(cell, candidate)
			if inboundMaterial > 0 {
				score += addTerm(&terms, "safety", inboundMaterial*GuardedByValue*params.Safety, "guardedBy")
			}
		}
	}

	// King hunt
	if !cell.Convoy && quietMove && candidateKnown && candidate != nil &&
		candidate.DestinationVisible && supportedAndUnthreatened(candidate) &&
		cell.Rank != "king" && !isCurrentBeaconBearer(obs, cell) {
		for _, enemy := range b.enemy {
			if enemy.Rank == "king" && !enemy.Ghost && containsString(candidate.NextPossibleMoves, enemy.Square) {
				score += addTerm(&terms, "kingHunt", KingVisibleAttackBonus, "visible")
				break
			}
		}
	}

	// Target lock dodge
	if params.TargetLock > 0 && b.lockedUnits[cell.UnitID.String()] {
		score += addTerm(&terms, "targetLock", TargetLockDodgeValue*params.TargetLock, "dodge")
		if postSafetyKnown && postThreat == 0 {
			score += addTerm(&terms, "targetLock", TargetLockSafeValue*params.TargetLock, "safeDodge")
		}
	}

	// King safety
	if params.KingSafety > 0 && b.kingThreatened {
		if cell.Rank == "king" && !cell.Convoy && postSafetyKnown && postThreat == 0 {
			score += addTerm(&terms, "kingSafety", params.KingSafety, "escape")
		}
		if targetCell != nil && b.kingCell != nil && containsString(targetCell.Threatens, b.kingCell.Square) {
			score += addTerm(&terms, "kingSafety", params.KingSafety*KingGuardBonus, "guard")
		}
	}

	// Morale push
	if params.MoralePush > 0 && cell.Rank == "king" && !cell.Convoy {
		gain := float64(forwardProgress(b.side, destination) - forwardProgress(b.side, cell.Square))
		kingSafeAfter := false
		if candidateKnown && candidate != nil {
			kingSafeAfter = candidate.DestinationVisible && supportedAndUnthreatened(candidate)
		} else {
			kingSafeAfter = targetCell != nil && postThreat == 0
		}
		if gain > 0 && kingSafeAfter {
			if candidateKnown {
				afterMorale := postMoveMorale(obs, b, cell.Square, destination)
				excess := afterMorale - currentMoraleNeed(obs)
				guardStrength := leaderSupport(obs, b, cell, destination)
				if excess >= LeaderExcessMorale {
					score += addTerm(&terms, "moralePush", -gain*MoralePushValue*params.MoralePush, "excessAdvance")
				} else {
					score += addTerm(&terms, "moralePush", gain*MoralePushValue*params.MoralePush*guardStrength, "guardedAdvance")
				}
			}
		} else if candidateKnown && gain < 0 && kingSafeAfter {
			afterMorale := postMoveMorale(obs, b, cell.Square, destination)
			needed := currentMoraleNeed(obs)
			if obs.OwnMorale-needed >= LeaderExcessMorale && afterMorale >= needed+1 {
				score += addTerm(&terms, "moralePush", -gain*MoralePushValue*params.MoralePush*LeaderRetreatValue, "excessRetreat")
			}
		}
		if postThreat > 0 {
			score += addTerm(&terms, "safety", -KingValue*params.Safety, "kingIntoStrike")
		}
	}

	// Beacon bearer leadership
	if isCurrentBeaconBearer(obs, cell) && cell.Rank != "king" && !cell.Convoy && quietMove && params.BeaconAggression > 0 {
		gain := forwardProgress(b.side, destination) - forwardProgress(b.side, cell.Square)
		if candidateKnown && candidate != nil && candidate.DestinationVisible && supportedAndUnthreatened(candidate) {
			supportGain := leaderSupport(obs, b, cell, destination) - leaderSupport(obs, b, cell, cell.Square)
			if supportGain > 0 {
				detail := "guardedAdvance"
				if gain <= 0 {
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
	if cell.Rank == "pawn" && !cell.Convoy && isPromotionSquare(b.side, destination) {
		intent.Promotion = "queen"
	}
	if captureChoice != "" {
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

func priorityCaptiveDeliveryProposal(obs *Observation, b *boardContext, params *BotParams) *proposal {
	if !b.needsFirstMasterEngineer {
		return nil
	}
	for _, cell := range b.own {
		if !cell.Convoy || cell.CargoCount == 0 || cell.KingCargo || cell.Rank != "pawn" || b.busyUnits[cell.UnitID.String()] {
			continue
		}
		if cell.NotBriefed {
			continue
		}
		prisonerRank := cell.Rank
		if obs.Rules.CargoBasedDelivery {
			prisonerRank = "pawn"
		}
		destinations := obs.Legal[cell.Square]
		if len(destinations) == 0 {
			continue
		}
		baseSquares := obs.Rules.BaseSquares[prisonerRank]
		to := ""

		if containsString(baseSquares, cell.Square) {
			fallback := ""
			for _, candidate := range destinations {
				fact := candidateAt(obs, cell.Square, candidate)
				if candidate == cell.Square || b.enemyBySquare[candidate] != nil ||
					!hasCandidate(obs, cell.Square, candidate) || fact == nil ||
					!fact.DestinationVisible || !isSafeSubject(fact) {
					continue
				}
				if containsString(baseSquares, candidate) {
					to = candidate
					break
				}
				if fallback == "" {
					fallback = candidate
				}
			}
			if to == "" {
				to = fallback
			}
		} else {
			here := distanceToNearestSquare(cell.Square, baseSquares)
			bestGain := 0
			for _, candidate := range destinations {
				fact := candidateAt(obs, cell.Square, candidate)
				if candidate == cell.Square || b.enemyBySquare[candidate] != nil ||
					!hasCandidate(obs, cell.Square, candidate) || fact == nil ||
					!fact.DestinationVisible || !isSafeSubject(fact) {
					continue
				}
				gain := here - distanceToNearestSquare(candidate, baseSquares)
				if gain > bestGain {
					bestGain = gain
					to = candidate
				}
			}
		}

		if to == "" {
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

func priorityCaptiveDeliveryInFlight(obs *Observation, b *boardContext) bool {
	if !b.needsFirstMasterEngineer {
		return false
	}
	for _, cell := range b.own {
		charging := cell.Charging
		if charging == nil || !cell.Convoy || cell.CargoCount == 0 || cell.KingCargo || cell.Rank != "pawn" {
			continue
		}
		if b.allBySquare[charging.Square] != nil {
			continue
		}
		prisonerRank := cell.Rank
		if obs.Rules.CargoBasedDelivery {
			prisonerRank = "pawn"
		}
		baseSquares := obs.Rules.BaseSquares[prisonerRank]
		if containsString(baseSquares, cell.Square) {
			return true
		}
		if distanceToNearestSquare(charging.Square, baseSquares) < distanceToNearestSquare(cell.Square, baseSquares) {
			return true
		}
	}
	return false
}

func rankOptions(proposals []proposal, params *BotParams, count int) []Option {
	if count <= 0 {
		return nil
	}
	var chosen []proposal
	seenActors := make(map[string]bool)
	for _, p := range proposals {
		if p.score < params.PassBelow {
			break
		}
		actorKey := p.key
		if p.actor != nil {
			if uid, ok := p.actor.(UnitID); ok && !uid.IsZero() {
				actorKey = uid.String()
			} else if str, ok := p.actor.(string); ok && str != "" {
				actorKey = str
			}
		}
		if seenActors[actorKey] {
			continue
		}
		seenActors[actorKey] = true
		chosen = append(chosen, p)
		if len(chosen) >= count {
			break
		}
	}

	var options []Option
	rank := 0
	leaderScore := 0.0
	for i, p := range chosen {
		if i == 0 || p.score < leaderScore-TieBreakBand {
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
