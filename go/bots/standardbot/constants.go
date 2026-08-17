// Copyright 2026 Sneat.app

package standardbot

// Constants shared by every tier in standardbot, matching chess-raiders-bot.star.
const (
	// ---- material, in pawns -----------------------------------------------------
	PawnValue      = 1.0
	KnightValue    = 3.0
	BishopValue    = 3.25
	RookValue      = 5.0
	QueenValue     = 9.0
	KingValue      = 60.0 // capturing the king only loads it into a convoy that must still be walked home
	KingCargoValue = 50.0 // a convoy carrying a captured king is worth the king riding inside it
	GhostDiscount  = 0.5  // a remembered, possibly-stale fogged occupant is trusted less than something actually visible

	// ---- safety: trading and danger --------------------------------------------
	SafeTradeDiscount        = 0.35 // a trade that already wins material is treated as a fraction of the risk
	RecaptureDiscount        = 0.7  // a square a friendly piece guards is treated as a fraction of the risk
	KingCargoEscortRisk      = KingCargoValue
	RouteReplaceUrgencyValue = 1.0
	RouteReplaceBaseline     = 0.7
	CommitmentDecay          = 0.9

	// ---- the win condition: walking a captured king home -----------------------
	DeliveryBonus       = 500.0 // reaching a delivery square ends the match in our favour
	DeliveryStepValue   = 4.0   // value of one convoy step closer to delivery, before delivery weight
	UnblockBaseValue    = 20.0  // reward for a friendly piece giving way off the convoy's home square
	UnblockValueSpread  = 1.0   // coefficient on the blocker's own value
	UnreachablePathCost = 99    // sentinel for "no convoy route home at all"

	// ---- prisoner logistics: escorting a captive home -------------------------
	CaptureAliveBonus            = 0.25 // extra credit for capturable piece taken alive
	PrisonerStepValue            = 1.5  // value of one step closer to base for a captive convoy
	ValueVeteranBootstrap        = 4.0  // extra push toward a pawn's own capture
	PriorityCaptiveDeliveryScore = 1000.0

	// ---- positional pressure ----------------------------------------------------
	AdvancePawnMultiplier    = 2.0
	PromotionBonus           = 8.0
	PromotionNextBonus       = 4.0
	LastRankIndex            = 7
	DevelopFirstForwardValue = 1.0
	PatrolGainValue          = 0.5
	PatrolGainCap            = 4
	SupportMaterialCap       = 6.0
	GuardsValue              = 0.5
	GuardedByValue           = 0.5
	SoleGuardLostValue       = 0.5
	KingVisibleAttackBonus   = 30.0
	UnknownQuietPenalty      = 1.0
	UnsupportedQuietPenalty  = 0.75

	// ---- king and Beacon leadership --------------------------------------------
	LeaderSupportSaturation = 2.0
	LeaderExcessMorale      = 3
	MoralePushValue         = 14.0
	LeaderRetreatValue      = 0.5

	// ---- reacting to a public threat --------------------------------------------
	TargetLockDodgeValue = 2.0
	TargetLockSafeValue  = 1.0
	KingGuardBonus       = 0.5

	// ---- which unit gets looked at first (unit_priority) ------------------------
	UnitPriorityKingCargo         = 100.0
	UnitPriorityPrisonerCargo     = 20.0
	UnitPriorityLockedBonus       = 5.0
	UnitPriorityKingItself        = 8.0
	UnitPriorityFormationLeader   = 40.0
	UnitPriorityNearKing          = 3.0
	UnitPriorityKingVisibleAttack = 30.0
	NearKingRadius                = 2
	RankPriorityScale             = 100.0

	// ---- system actions: Beacon ------------------------------------------------
	SystemBeaconHandOffValue = 5.0
	SystemBeaconDeployValue  = 4.0
	SystemBeaconRestoreValue = 3.0
	SystemBeaconForgeValue   = 3.2

	// ---- system actions: training -----------------------------------------------
	SystemTrainValue         = 3.0
	SystemAdvancedTrainValue = 3.5

	// ---- system actions: wall REPAIR/DISMANTLE ----------------------------------
	ValueRepairWall    = 1.5
	ValueDismantleWall = 1.0

	// ---- system actions: espionage ----------------------------------------------
	InterrogateValue = 0.6

	// ---- bookkeeping sentinels ---------------------------------------------------
	TieBreakBand            = 1e-9
	UnreachableDistance     = 99
	NoSquareIndex           = -1
	MaxSquareIndex          = 63
	BoardFiles              = 8
	RefusedSetSize          = 4
	QuietVacatedSquares     = 3
	UnlimitedActiveCommands = 1000000
	MillisecondsPerSecond   = 1000.0
	PlacementSquaresPerSlot = 13
	PlacementBoardSlots     = 5

	CommitKindRoute   = 1
	CommitKindKing    = 2
	CommitScoreScale  = 1000.0
	CommitScoreOffset = 2000000
	CommitScoreField  = 4000000
	CommitAgeField    = 100000
	CommitAgeCap      = 64
)
