// Copyright 2026 Sneat.app

package standardbot

import (
	"encoding/json"
	"fmt"
)

// UnitID is a flexible unit identifier that can be unmarshaled from JSON number or string.
type UnitID struct {
	Num int64
	Str string
}

func (u UnitID) IsZero() bool {
	return u.Num == 0 && u.Str == ""
}

func (u UnitID) String() string {
	if u.Str != "" {
		return u.Str
	}
	return fmt.Sprintf("%d", u.Num)
}

func (u UnitID) MarshalJSON() ([]byte, error) {
	if u.Str != "" {
		return json.Marshal(u.Str)
	}
	return json.Marshal(u.Num)
}

func (u *UnitID) UnmarshalJSON(b []byte) error {
	var num int64
	if err := json.Unmarshal(b, &num); err == nil {
		u.Num = num
		u.Str = ""
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		u.Str = str
		u.Num = 0
		return nil
	}
	return nil
}

// ChargingFact describes in-flight movement charge for a unit.
type ChargingFact struct {
	Square      string `json:"square"`
	RemainingMs int    `json:"remainingMs"`
}

// Cell is a projected piece on the board.
type Cell struct {
	Square                   string        `json:"square,omitempty"`
	UnitID                   UnitID        `json:"unitId"`
	Side                     string        `json:"side"`
	Rank                     string        `json:"rank"`
	Convoy                   bool          `json:"convoy"`
	CargoCount               int           `json:"cargoCount"`
	KingCargo                bool          `json:"kingCargo"`
	Ghost                    bool          `json:"ghost"`
	Refitting                bool          `json:"refitting"`
	Profession               string        `json:"profession,omitempty"`
	Grade                    string        `json:"grade,omitempty"`
	EligibleFor              []string      `json:"eligibleFor,omitempty"`
	Veteran                  bool          `json:"veteran,omitempty"`
	TargetLocked             bool          `json:"targetLocked,omitempty"`
	Charging                 *ChargingFact `json:"charging,omitempty"`
	Training                 any           `json:"training,omitempty"`
	ForgingRemainingMs       int           `json:"forgingRemainingMs,omitempty"`
	RecoveryRemainingMs      int           `json:"recoveryRemainingMs,omitempty"`
	InterrogationRemainingMs *int          `json:"interrogationRemainingMs,omitempty"`
	NotBriefed               bool          `json:"notBriefed,omitempty"`
	Moved                    bool          `json:"moved,omitempty"`
	Guards                   []string      `json:"guards,omitempty"`
	GuardedBy                []string      `json:"guardedBy,omitempty"`
	Threatens                []string      `json:"threatens,omitempty"`
	ThreatenedBy             []string      `json:"threatenedBy,omitempty"`
}

// CandidateFact is the host-provided deterministic post-state fact.
type CandidateFact struct {
	DestinationVisible bool     `json:"destinationVisible"`
	PatrolGain         int      `json:"patrolGain"`
	NextPossibleMoves  []string `json:"nextPossibleMoves,omitempty"`
	Guards             []string `json:"guards,omitempty"`
	GuardedBy          []string `json:"guardedBy,omitempty"`
	Threatens          []string `json:"threatens,omitempty"`
	ThreatenedBy       []string `json:"threatenedBy,omitempty"`
}

// CaptureOdds carries forecast odds.
type CaptureOdds struct {
	Success        int `json:"success"`
	DefenderKilled int `json:"defenderKilled"`
	Repelled       int `json:"repelled"`
}

// CaptureOutcome is the affordability and odds for Kill or Capture.
type CaptureOutcome struct {
	Affordable      bool        `json:"affordable"`
	RequiredMorale  int         `json:"requiredMorale"`
	AvailableMorale int         `json:"availableMorale"`
	OddsKnown       bool        `json:"oddsKnown"`
	Odds            CaptureOdds `json:"odds"`
}

// DestinationOutcomes maps "kill" / "capture" to CaptureOutcome.
type DestinationOutcomes struct {
	Kill    *CaptureOutcome `json:"kill,omitempty"`
	Capture *CaptureOutcome `json:"capture,omitempty"`
}

// Rules are match-level public rules.
type Rules struct {
	AllowsKill               bool                `json:"allowsKill"`
	AllowsCapture            bool                `json:"allowsCapture"`
	CargoBasedDelivery       bool                `json:"cargoBasedDelivery"`
	MoraleEnabled            bool                `json:"moraleEnabled"`
	SpecialistsEnabled       bool                `json:"specialistsEnabled"`
	WoodWallsEnabled         bool                `json:"woodWallsEnabled"`
	StoneWallsEnabled        bool                `json:"stoneWallsEnabled"`
	VeteranProgression       bool                `json:"veteranProgression"`
	BeaconEnabled            bool                `json:"beaconEnabled"`
	BeaconForgeEnabled       bool                `json:"beaconForgeEnabled"`
	BeaconKingStartsAsBearer bool                `json:"beaconKingStartsAsBearer"`
	SpecialistCeiling        int                 `json:"specialistCeiling"`
	PermittedProfessions     []string            `json:"permittedProfessions"`
	MaxActiveCommands        int                 `json:"maxActiveCommands"`
	PieceChargeMs            map[string]int      `json:"pieceChargeMs"`
	BaseSquares              map[string][]string `json:"baseSquares"`
}

// Systems are subsystem enable flags.
type Systems struct {
	Training  bool `json:"training"`
	Walls     bool `json:"walls"`
	Beacon    bool `json:"beacon"`
	Prisoners bool `json:"prisoners"`
	Morale    bool `json:"morale"`
	Espionage bool `json:"espionage"`
}

// BeaconFact carries information about the team Beacon.
type BeaconFact struct {
	Lifecycle     string `json:"lifecycle"`
	BearerSquare  string `json:"bearerSquare"`
	EverHandedOff bool   `json:"everHandedOff"`
}

// WallFact describes one wall edge.
type WallFact struct {
	Edge             string `json:"edge"`
	A                string `json:"a"`
	B                string `json:"b"`
	Side             string `json:"side"`
	DirectionFromA   string `json:"directionFromA"`
	DirectionFromB   string `json:"directionFromB"`
	Integrity        int    `json:"integrity"`
	MaximumIntegrity int    `json:"maximumIntegrity"`
	OwnWorkSession   bool   `json:"ownWorkSession"`
}

// Observation is the fog-correct input to decide().
type Observation struct {
	Lifecycle         string                                    `json:"lifecycle"`
	Side              string                                    `json:"side"`
	NowMs             int64                                     `json:"nowMs"`
	Revision          int64                                     `json:"revision"`
	Turn              int                                       `json:"turn"`
	Pieces            map[string]Cell                           `json:"pieces"`
	Legal             map[string][]string                       `json:"legal"`
	Affordability     map[string]map[string]DestinationOutcomes `json:"affordability"`
	EnPassant         map[string]map[string]string              `json:"enPassant"`
	Candidates        map[string]map[string]CandidateFact       `json:"candidates"`
	DeliverySquares   []string                                  `json:"deliverySquares"`
	ConvoyHome        map[string]int                            `json:"convoyHome"`
	BlockingBase      []string                                  `json:"blockingBase"`
	Rules             Rules                                     `json:"rules"`
	Systems           Systems                                   `json:"systems"`
	Beacon            BeaconFact                                `json:"beacon"`
	Walls             []WallFact                                `json:"walls"`
	OwnMorale         int                                       `json:"ownMorale"`
	OwnMoralePenalty  int                                       `json:"ownMoralePenalty"`
	OwnManaged        int                                       `json:"ownManaged"`
	OwnSpecialists    int                                       `json:"ownSpecialists"`
	EnemyManaged      int                                       `json:"enemyManaged"`
	CaptureMoraleNeed *int                                      `json:"captureMoraleNeed,omitempty"`
}

// BotParams is a resolved tier parameter row.
type BotParams struct {
	Material           float64 `json:"material"`
	Safety             float64 `json:"safety"`
	Tempo              float64 `json:"tempo"`
	Advance            float64 `json:"advance"`
	TargetLock         float64 `json:"targetLock"`
	KingSafety         float64 `json:"kingSafety"`
	MoralePush         float64 `json:"moralePush"`
	BeaconAggression   float64 `json:"beaconAggression"`
	Delivery           float64 `json:"delivery"`
	Prisoner           float64 `json:"prisoner"`
	System             float64 `json:"system"`
	Breadth            int     `json:"breadth"`
	CandidateSpread    int     `json:"candidateSpread"`
	PassBelow          float64 `json:"passBelow"`
	AdvancedTraining   bool    `json:"advancedTraining"`
	ContestEnemyWork   bool    `json:"contestEnemyWork"`
	SergeantPreference float64 `json:"sergeantPreference"`
}

// Intent is a chosen bot command.
type Intent struct {
	Kind       string `json:"kind"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Choice     string `json:"choice,omitempty"`
	Promotion  string `json:"promotion,omitempty"`
	Action     string `json:"action,omitempty"`
	Target     any    `json:"target,omitempty"`
	Profession string `json:"profession,omitempty"`
	Direction  string `json:"direction,omitempty"`
}

// Term is one signed contribution to an Option's score.
type Term struct {
	Term   string  `json:"term"`
	Value  float64 `json:"value"`
	Detail string  `json:"detail,omitempty"`
}

// Option is a ranked candidate with its score explanation.
type Option struct {
	Rank   int     `json:"rank"`
	Intent Intent  `json:"intent"`
	Score  float64 `json:"score"`
	Terms  []Term  `json:"terms"`
}
