// Copyright 2026 Sneat.app

package standardbot

import (
	"fmt"
)

// PARITY-FUNCTION: SBP-F-MOST-ADVANCED-ADJACENT-ALLY
func mostAdvancedAdjacentAlly(b *boardContext) string {
	kingCell := b.kingCell
	if kingCell == nil { // PARITY-BRANCH: SBP-B-MOST-ADVANCED-ADJACENT-ALLY-1
		return ""
	}
	bestSquare := ""
	bestProgress := -1
	for _, cell := range b.actionableUnits {
		if cell.Rank == "king" || cell.Convoy { // PARITY-BRANCH: SBP-B-MOST-ADVANCED-ADJACENT-ALLY-2
			continue
		}
		if chebyshevDistance(kingCell.Square, cell.Square) != 1 { // PARITY-BRANCH: SBP-B-MOST-ADVANCED-ADJACENT-ALLY-3
			continue
		}
		progress := forwardProgress(b.side, cell.Square)
		if progress > bestProgress { // PARITY-BRANCH: SBP-B-MOST-ADVANCED-ADJACENT-ALLY-4
			bestProgress = progress
			bestSquare = cell.Square
		}
	}
	return bestSquare
}

// PARITY-FUNCTION: SBP-F-BEACON-HAND-OFF-PROPOSAL
func beaconHandOffProposal(obs *Observation, b *boardContext, params *BotParams) *proposal {
	if !obs.Rules.BeaconEnabled { // PARITY-BRANCH: SBP-B-BEACON-HAND-OFF-PROPOSAL-1
		return nil
	}
	kingCell := b.kingCell
	if kingCell == nil || b.busyUnits[kingCell.UnitID.String()] { // PARITY-BRANCH: SBP-B-BEACON-HAND-OFF-PROPOSAL-2
		return nil
	}
	if !obs.Rules.BeaconKingStartsAsBearer { // PARITY-BRANCH: SBP-B-BEACON-HAND-OFF-PROPOSAL-3
		return nil
	}
	beacon := obs.Beacon
	if beacon.Lifecycle != "deployed" || beacon.EverHandedOff { // PARITY-BRANCH: SBP-B-BEACON-HAND-OFF-PROPOSAL-4
		return nil
	}
	if beacon.BearerSquare != kingCell.Square { // PARITY-BRANCH: SBP-B-BEACON-HAND-OFF-PROPOSAL-5
		return nil
	}
	to := mostAdvancedAdjacentAlly(b)
	if to == "" { // PARITY-BRANCH: SBP-B-BEACON-HAND-OFF-PROPOSAL-6
		return nil
	}
	var actor any
	for _, cell := range b.actionableUnits {
		if cell.Square == to { // PARITY-BRANCH: SBP-B-BEACON-HAND-OFF-PROPOSAL-7
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

// PARITY-FUNCTION: SBP-F-TRAINING-PROPOSALS
func trainingProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	haveEngineer := false
	for _, cell := range b.own {
		if cell.Profession == "engineer" { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-1
			haveEngineer = true
			break
		}
	}

	ceiling := obs.Rules.SpecialistCeiling
	permitted := obs.Rules.PermittedProfessions
	baseSquares := obs.Rules.BaseSquares["pawn"]
	var proposals []proposal

	for _, cell := range b.own {
		if cell.Convoy || cell.Refitting || cell.Rank != "pawn" || b.busyUnits[cell.UnitID.String()] { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-2
			continue
		}
		if !containsString(baseSquares, cell.Square) { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-3
			continue
		}
		if cellThreatenedCount(cell) > 0 { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-4
			continue
		}
		if cell.Profession == "" { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-5
			if obs.Rules.VeteranProgression && !cell.Veteran { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-6
				continue
			}
			if obs.OwnSpecialists >= ceiling { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-7
				continue
			}
			profession := "sergeant"
			if !haveEngineer && containsString(permitted, "engineer") { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-8
				profession = "engineer"
			}
			if !containsString(permitted, profession) { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-9
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
			containsString(cell.EligibleFor, "masterEngineerTraining") && cell.Grade != "master" { // PARITY-BRANCH: SBP-B-TRAINING-PROPOSALS-10
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

// PARITY-FUNCTION: SBP-F-SERGEANT-SUPPORTED
func sergeantSupported(b *boardContext, square string) bool {
	for _, cell := range b.own {
		if cell.Profession == "sergeant" && !cell.Convoy && !cell.Refitting &&
			chebyshevDistance(square, cell.Square) == 1 { // PARITY-BRANCH: SBP-B-SERGEANT-SUPPORTED-1
			return true
		}
	}
	return false
}

// PARITY-FUNCTION: SBP-F-WALL-PROPOSALS
func wallProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	var proposals []proposal
	for _, cell := range b.actionableUnits {
		if cell.Convoy || cell.Refitting { // PARITY-BRANCH: SBP-B-WALL-PROPOSALS-1
			continue
		}
		for _, wall := range obs.Walls {
			direction := ""
			if wall.A == cell.Square { // PARITY-BRANCH: SBP-B-WALL-PROPOSALS-2
				direction = wall.DirectionFromA
			} else if wall.B == cell.Square { // PARITY-BRANCH: SBP-B-WALL-PROPOSALS-3
				direction = wall.DirectionFromB
			} else {
				continue
			}
			if wall.OwnWorkSession { // PARITY-BRANCH: SBP-B-WALL-PROPOSALS-4
				continue
			}
			var terms []Term
			if wall.Side == b.side && cell.Profession == "engineer" &&
				wall.MaximumIntegrity > 0 && wall.Integrity*10 < wall.MaximumIntegrity*6 { // PARITY-BRANCH: SBP-B-WALL-PROPOSALS-5
				score := addTerm(&terms, "system", ValueRepairWall*params.System, "repairWall")
				if sergeantSupported(b, cell.Square) { // PARITY-BRANCH: SBP-B-WALL-PROPOSALS-6
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
			} else if wall.Side != b.side && params.ContestEnemyWork { // PARITY-BRANCH: SBP-B-WALL-PROPOSALS-7
				score := addTerm(&terms, "system", ValueDismantleWall*params.System, "dismantleWall")
				if sergeantSupported(b, cell.Square) { // PARITY-BRANCH: SBP-B-WALL-PROPOSALS-8
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

// PARITY-FUNCTION: SBP-F-BEACON-DEPLOY-OR-RESTORE-PROPOSAL
func beaconDeployOrRestoreProposal(obs *Observation, b *boardContext, params *BotParams) *proposal {
	if !obs.Rules.BeaconEnabled { // PARITY-BRANCH: SBP-B-BEACON-DEPLOY-OR-RESTORE-PROPOSAL-1
		return nil
	}
	kingCell := b.kingCell
	if kingCell == nil || b.busyUnits[kingCell.UnitID.String()] { // PARITY-BRANCH: SBP-B-BEACON-DEPLOY-OR-RESTORE-PROPOSAL-2
		return nil
	}
	beacon := obs.Beacon
	if beacon.Lifecycle == "undeployed" { // PARITY-BRANCH: SBP-B-BEACON-DEPLOY-OR-RESTORE-PROPOSAL-3
		score := SystemBeaconDeployValue * beaconAggression(obs, params)
		return &proposal{
			intent: Intent{Kind: "action", From: kingCell.Square, To: kingCell.Square, Action: "beacon_deploy"},
			score:  score,
			terms:  []Term{{Term: "beaconAggression", Value: score, Detail: "deploy"}},
			actor:  kingCell.UnitID,
			key:    "beacon-deploy",
		}
	}
	if beacon.Lifecycle == "lost" { // PARITY-BRANCH: SBP-B-BEACON-DEPLOY-OR-RESTORE-PROPOSAL-4
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

// PARITY-FUNCTION: SBP-F-BEACON-FORGE-PROPOSALS
func beaconForgeProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	if !obs.Rules.BeaconEnabled || !obs.Rules.BeaconForgeEnabled { // PARITY-BRANCH: SBP-B-BEACON-FORGE-PROPOSALS-1
		return nil
	}
	if obs.Beacon.Lifecycle != "lost" { // PARITY-BRANCH: SBP-B-BEACON-FORGE-PROPOSALS-2
		return nil
	}
	ownPawnBaseSquares := obs.Rules.BaseSquares["pawn"]
	var proposals []proposal
	for _, cell := range b.actionableUnits {
		if cell.Convoy || cell.Refitting || cell.Rank != "pawn" { // PARITY-BRANCH: SBP-B-BEACON-FORGE-PROPOSALS-3
			continue
		}
		if !containsString(ownPawnBaseSquares, cell.Square) { // PARITY-BRANCH: SBP-B-BEACON-FORGE-PROPOSALS-4
			continue
		}
		if cellThreatenedCount(cell) > 0 { // PARITY-BRANCH: SBP-B-BEACON-FORGE-PROPOSALS-5
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

// PARITY-FUNCTION: SBP-F-ESPIONAGE-PROPOSALS
func espionageProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	kingCell := b.kingCell
	if kingCell == nil || b.busyUnits[kingCell.UnitID.String()] { // PARITY-BRANCH: SBP-B-ESPIONAGE-PROPOSALS-1
		return nil
	}
	if obs.EnemyManaged <= 0 { // PARITY-BRANCH: SBP-B-ESPIONAGE-PROPOSALS-2
		return nil
	}
	var proposals []proposal
	for _, suspect := range b.actionableUnits {
		if suspect.Rank == "king" || suspect.Convoy { // PARITY-BRANCH: SBP-B-ESPIONAGE-PROPOSALS-3
			continue
		}
		if chebyshevDistance(kingCell.Square, suspect.Square) != 1 { // PARITY-BRANCH: SBP-B-ESPIONAGE-PROPOSALS-4
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

// PARITY-FUNCTION: SBP-F-SYSTEM-PROPOSALS
func systemProposals(obs *Observation, b *boardContext, params *BotParams) []proposal {
	systems := obs.Systems
	beaconAllowed := obs.Rules.BeaconEnabled && beaconAggression(obs, params) > 0
	anyOtherSystemEnabled := systems.Training || systems.Walls || systems.Prisoners || systems.Morale || systems.Espionage
	if !beaconAllowed && (!anyOtherSystemEnabled || params.System <= 0) { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-1
		return nil
	}
	var proposals []proposal
	if params.System > 0 { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-2
		if systems.Training && obs.Rules.SpecialistsEnabled { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-3
			proposals = append(proposals, trainingProposals(obs, b, params)...)
		}
		if systems.Walls && (obs.Rules.WoodWallsEnabled || obs.Rules.StoneWallsEnabled) { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-4
			proposals = append(proposals, wallProposals(obs, b, params)...)
		}
		if systems.Espionage { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-5
			proposals = append(proposals, espionageProposals(obs, b, params)...)
		}
	}
	if beaconAllowed { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-6
		if handOff := beaconHandOffProposal(obs, b, params); handOff != nil { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-7
			proposals = append(proposals, *handOff)
		}
		if deployOrRestore := beaconDeployOrRestoreProposal(obs, b, params); deployOrRestore != nil { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-8
			proposals = append(proposals, *deployOrRestore)
		}
		if obs.Rules.BeaconForgeEnabled { // PARITY-BRANCH: SBP-B-SYSTEM-PROPOSALS-9
			proposals = append(proposals, beaconForgeProposals(obs, b, params)...)
		}
	}
	return proposals
}
