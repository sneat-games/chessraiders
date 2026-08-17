// Copyright 2026 Sneat.app

package standardbot

import (
	"fmt"
)

func mostAdvancedAdjacentAlly(b *boardContext) string {
	kingCell := b.kingCell
	if kingCell == nil {
		return ""
	}
	bestSquare := ""
	bestProgress := -1
	for _, cell := range b.actionableUnits {
		if cell.Rank == "king" || cell.Convoy {
			continue
		}
		if chebyshevDistance(kingCell.Square, cell.Square) != 1 {
			continue
		}
		progress := forwardProgress(b.side, cell.Square)
		if progress > bestProgress {
			bestProgress = progress
			bestSquare = cell.Square
		}
	}
	return bestSquare
}

func beaconHandOffProposal(obs *Observation, b *boardContext, params *BotParams) *proposal {
	if !obs.Rules.BeaconEnabled {
		return nil
	}
	kingCell := b.kingCell
	if kingCell == nil || b.busyUnits[kingCell.UnitID.String()] {
		return nil
	}
	if !obs.Rules.BeaconKingStartsAsBearer {
		return nil
	}
	beacon := obs.Beacon
	if beacon.Lifecycle != "deployed" || beacon.EverHandedOff {
		return nil
	}
	if beacon.BearerSquare != kingCell.Square {
		return nil
	}
	to := mostAdvancedAdjacentAlly(b)
	if to == "" {
		return nil
	}
	var actor any
	for _, cell := range b.actionableUnits {
		if cell.Square == to {
			actor = cell.UnitID
			break
		}
	}
	score := SystemBeaconHandOffValue * beaconAggression(obs, params)
	return &proposal{
		intent: Intent{Kind: "action", From: to, To: to, Action: "beacon_take"},
		score:  score,
		terms:  []Term{{Term: "beaconAggression", Value: score, Detail: "handOff"}},
		actor:  actor,
		key:    "beacon-hand-off",
	}
}

func trainingProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	haveEngineer := false
	for _, cell := range b.own {
		if cell.Profession == "engineer" {
			haveEngineer = true
			break
		}
	}

	ceiling := obs.Rules.SpecialistCeiling
	permitted := obs.Rules.PermittedProfessions
	baseSquares := obs.Rules.BaseSquares["pawn"]
	var proposals []proposal

	for _, cell := range b.own {
		if cell.Convoy || cell.Refitting || cell.Rank != "pawn" || b.busyUnits[cell.UnitID.String()] {
			continue
		}
		if !containsString(baseSquares, cell.Square) {
			continue
		}
		if cellThreatenedCount(cell) > 0 {
			continue
		}
		if cell.Profession == "" {
			if obs.Rules.VeteranProgression && !cell.Veteran {
				continue
			}
			if obs.OwnSpecialists >= ceiling {
				continue
			}
			profession := "sergeant"
			if !haveEngineer && containsString(permitted, "engineer") {
				profession = "engineer"
			}
			if !containsString(permitted, profession) {
				continue
			}
			score := SystemTrainValue * params.System
			proposals = append(proposals, proposal{
				intent: Intent{
					Kind:       "action",
					From:       cell.Square,
					To:         cell.Square,
					Action:     "train",
					Profession: profession,
				},
				score: score,
				terms: []Term{{Term: "system", Value: score, Detail: "train"}},
				actor: cell.UnitID,
				key:   fmt.Sprintf("train|%s|%s", cell.UnitID.String(), profession),
			})
			continue
		}
		if params.AdvancedTraining && cell.Profession == "engineer" &&
			containsString(cell.EligibleFor, "masterEngineerTraining") && cell.Grade != "master" {
			score := SystemAdvancedTrainValue * params.System
			proposals = append(proposals, proposal{
				intent: Intent{
					Kind:   "action",
					From:   cell.Square,
					To:     cell.Square,
					Action: "advanced_train",
				},
				score: score,
				terms: []Term{{Term: "system", Value: score, Detail: "advancedTrain"}},
				actor: cell.UnitID,
				key:   fmt.Sprintf("advanced|%s", cell.UnitID.String()),
			})
		}
	}
	return proposals
}

func sergeantSupported(b *boardContext, square string) bool {
	for _, cell := range b.own {
		if cell.Profession == "sergeant" && !cell.Convoy && !cell.Refitting &&
			chebyshevDistance(square, cell.Square) == 1 {
			return true
		}
	}
	return false
}

func wallProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	var proposals []proposal
	for _, cell := range b.actionableUnits {
		if cell.Convoy || cell.Refitting {
			continue
		}
		for _, wall := range obs.Walls {
			direction := ""
			if wall.A == cell.Square {
				direction = wall.DirectionFromA
			} else if wall.B == cell.Square {
				direction = wall.DirectionFromB
			} else {
				continue
			}
			if wall.OwnWorkSession {
				continue
			}
			var terms []Term
			if wall.Side == b.side && cell.Profession == "engineer" &&
				wall.MaximumIntegrity > 0 && wall.Integrity*10 < wall.MaximumIntegrity*6 {
				score := addTerm(&terms, "system", ValueRepairWall*params.System, "repairWall")
				if sergeantSupported(b, cell.Square) {
					score += addTerm(&terms, "system", params.SergeantPreference*params.System, "sergeantSupported")
				}
				proposals = append(proposals, proposal{
					intent: Intent{
						Kind:      "action",
						From:      cell.Square,
						To:        cell.Square,
						Action:    "repair_wall",
						Direction: direction,
					},
					score: score,
					terms: terms,
					actor: cell.UnitID,
					key:   fmt.Sprintf("repair|%s|%s", cell.UnitID.String(), wall.Edge),
				})
			} else if wall.Side != b.side && params.ContestEnemyWork {
				score := addTerm(&terms, "system", ValueDismantleWall*params.System, "dismantleWall")
				if sergeantSupported(b, cell.Square) {
					score += addTerm(&terms, "system", params.SergeantPreference*params.System, "sergeantSupported")
				}
				proposals = append(proposals, proposal{
					intent: Intent{
						Kind:      "action",
						From:      cell.Square,
						To:        cell.Square,
						Action:    "dismantle_wall",
						Direction: direction,
					},
					score: score,
					terms: terms,
					actor: cell.UnitID,
					key:   fmt.Sprintf("dismantle|%s|%s", cell.UnitID.String(), wall.Edge),
				})
			}
		}
	}
	return proposals
}

func beaconDeployOrRestoreProposal(obs *Observation, b *boardContext, params *BotParams) *proposal {
	if !obs.Rules.BeaconEnabled {
		return nil
	}
	kingCell := b.kingCell
	if kingCell == nil || b.busyUnits[kingCell.UnitID.String()] {
		return nil
	}
	beacon := obs.Beacon
	if beacon.Lifecycle == "undeployed" {
		score := SystemBeaconDeployValue * beaconAggression(obs, params)
		return &proposal{
			intent: Intent{Kind: "action", From: kingCell.Square, To: kingCell.Square, Action: "beacon_deploy"},
			score:  score,
			terms:  []Term{{Term: "beaconAggression", Value: score, Detail: "deploy"}},
			actor:  kingCell.UnitID,
			key:    "beacon-deploy",
		}
	}
	if beacon.Lifecycle == "lost" {
		score := SystemBeaconRestoreValue * beaconAggression(obs, params)
		return &proposal{
			intent: Intent{Kind: "action", From: kingCell.Square, To: kingCell.Square, Action: "beacon_restore"},
			score:  score,
			terms:  []Term{{Term: "beaconAggression", Value: score, Detail: "restore"}},
			actor:  kingCell.UnitID,
			key:    "beacon-restore",
		}
	}
	return nil
}

func beaconForgeProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	if !obs.Rules.BeaconEnabled || !obs.Rules.BeaconForgeEnabled {
		return nil
	}
	if obs.Beacon.Lifecycle != "lost" {
		return nil
	}
	ownPawnBaseSquares := obs.Rules.BaseSquares["pawn"]
	var proposals []proposal
	for _, cell := range b.actionableUnits {
		if cell.Convoy || cell.Refitting || cell.Rank != "pawn" {
			continue
		}
		if !containsString(ownPawnBaseSquares, cell.Square) {
			continue
		}
		if cellThreatenedCount(cell) > 0 {
			continue
		}
		score := SystemBeaconForgeValue * beaconAggression(obs, params)
		proposals = append(proposals, proposal{
			intent: Intent{Kind: "action", From: cell.Square, To: cell.Square, Action: "beacon_forge"},
			score:  score,
			terms:  []Term{{Term: "beaconAggression", Value: score, Detail: "forge"}},
			actor:  cell.UnitID,
			key:    fmt.Sprintf("beacon-forge|%s", cell.UnitID.String()),
		})
	}
	return proposals
}

func espionageProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	kingCell := b.kingCell
	if kingCell == nil || b.busyUnits[kingCell.UnitID.String()] {
		return nil
	}
	if obs.EnemyManaged <= 0 {
		return nil
	}
	var proposals []proposal
	for _, suspect := range b.actionableUnits {
		if suspect.Rank == "king" || suspect.Convoy {
			continue
		}
		if chebyshevDistance(kingCell.Square, suspect.Square) != 1 {
			continue
		}
		var terms []Term
		score := addTerm(&terms, "system", InterrogateValue*params.System, "")
		proposals = append(proposals, proposal{
			intent: Intent{
				Kind:   "action",
				From:   kingCell.Square,
				To:     kingCell.Square,
				Action: "interrogate",
				Target: suspect.UnitID,
			},
			score: score,
			terms: terms,
			actor: kingCell.UnitID,
			key:   fmt.Sprintf("interrogate|%s", suspect.UnitID.String()),
		})
	}
	return proposals
}

func systemProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	systems := obs.Systems
	beaconAllowed := obs.Rules.BeaconEnabled && beaconAggression(obs, params) > 0
	anyOtherSystemEnabled := systems.Training || systems.Walls || systems.Prisoners || systems.Morale || systems.Espionage
	if !beaconAllowed && (!anyOtherSystemEnabled || params.System <= 0) {
		return nil
	}
	var proposals []proposal
	if params.System > 0 {
		if systems.Training && obs.Rules.SpecialistsEnabled {
			proposals = append(proposals, trainingProposals(obs, b, params)...)
		}
		if systems.Walls && (obs.Rules.WoodWallsEnabled || obs.Rules.StoneWallsEnabled) {
			proposals = append(proposals, wallProposals(obs, b, params)...)
		}
		if systems.Espionage {
			proposals = append(proposals, espionageProposals(obs, b, params)...)
		}
	}
	if beaconAllowed {
		if handOff := beaconHandOffProposal(obs, b, params); handOff != nil {
			proposals = append(proposals, *handOff)
		}
		if deployOrRestore := beaconDeployOrRestoreProposal(obs, b, params); deployOrRestore != nil {
			proposals = append(proposals, *deployOrRestore)
		}
		if obs.Rules.BeaconForgeEnabled {
			proposals = append(proposals, beaconForgeProposals(obs, b, params)...)
		}
	}
	return proposals
}
