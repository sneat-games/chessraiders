"""Chess Raiders bot tier — the ONE script every difficulty shares.

A tier is a row of numbers (see params.go's Params/RecruitParams), not a
separate file: Recruit, Lieutenant and Commander all call the SAME decide()
below, differing only in the weights they pass in. Turning Recruit into
Lieutenant is editing a row of numbers, never this file.

WHAT A TIER IS DECIDING: given the board you can see (one square-keyed map
of projected pieces and their host-computed guard/threat relations), and the
actionable destinations the host offers for each own piece, score every
(piece, destination) pair and play the best one — unless nothing scores well
enough, in which case pass this turn. A "system action" (train a specialist,
work a wall, deploy the Beacon) competes on the same scale whenever its match
rules and tier permission enable it.

WHAT THIS SCRIPT NEVER DOES: it never invents a square. Every destination
it ever considers comes from `observation.legal`, which the host computed
from the real game engine. It never computes whether a piece can physically
reach a square, whether a step is blocked by a wall, or which squares
attack which — all of that ("board geometry") is computed by the host and
handed over as plain facts (the per-piece relations, `convoyHome`, `deliverySquares`
and `blockingBase` fields on `observation`). This script only ever weighs
facts it is given; it does not derive new ones about how pieces move.

ONE DELIBERATE DEPARTURE from the target design, forced by how this
particular pass had to be built rather than chosen for its own sake — full
reasoning in the accompanying report, not repeated here: decide() below
takes a FOURTH argument, host_random_draw, that the target contract does
not have. It stands in for a live `rand.intn(n)` host builtin this pass
could not wire up; see intn()'s own comment.

EXPLAIN MODE (spec/features/starlark-bot-runtime/adviser-tier's
REQ:adviser-is-an-explain-mode-not-a-third-implementation). decide() takes a
FIFTH argument, `options`, and returns a THIRD value alongside the chosen
intent and the memory: up to `options` ranked, explained candidates. This is
not a second scorer and not a branch inside score_move — score_move builds
its per-term breakdown ALWAYS, on every candidate, for every caller, and a
bot decision (options = 0) simply throws it away. The Adviser the player
reads and the bot that plays are therefore the SAME evaluation by
construction, which is the whole point: a hint that disagreed with the
engine would be the exact drift this repository has spent its lessons
deleting. The script produces NUMBERS AND TERM NAMES ONLY — no English, no
Russian, no user-facing sentence appears anywhere in this file. The client
turns fifteen term names plus a small closed set of details into prose, which
keeps the translation surface a closed vocabulary rather than a prose
catalogue.

Compile-once, decide() called directly: server-go/starlarkbot.Compile
parses and runs this whole module's top level exactly ONCE per session
(server-go/starlarkbot/program.go), and server-go/starlarktier's own
CompiledTier.Decide (runtime.go) calls decide() below directly, per
decision, through Program.Call — there is no exec-the-whole-file-and-read-
a-global adapter left in this file; that shape existed only while compile-
once did not.

READING THE `bot4chess/...` CITATIONS BELOW. This file cites
`bot4chess/score.go`, `bot4chess/systems.go`, `bot4chess/observe.go` and
`bot4chess.go` about forty times, in the form "mirrors X's own Y". **That
package no longer exists** — the Go bot logic was deleted on 2026-08-09
(commit 27707e9, spec/plans/retire-the-go-bot-tiers.md Task 20), leaving
this script as the ONE bot implementation on both runtimes. Grep will not
find those files; `git show add61e5:server-go/bot4chess/score.go` will,
`add61e5` being the last commit that had them.

The citations are kept deliberately rather than deleted. Each one records
WHY a specific number or ordering is what it is — a weight, a discount, the
sequence score_move walks its terms in — and that reasoning survives the
file it was ported from. Deleting the citation would delete the only record
of where a magic number came from, which is strictly worse than a reference
a reader has to resolve through history. The same choice was made across the
~20 other files that cite the package (Task 20's own report).

What is NOT kept is a citation a reader would act on and be misled by: the
three that named tests have been re-pointed to `server-go/tests4bot`, where
those tests actually still run.
"""

# =============================================================================
# Constants shared by every tier. A tier's OWN numbers — the eleven weights,
# candidateSpread, passBelow, breadth — arrive as `params`, never here. What
# follows is everything bot4chess/score.go and observe.go hold FIXED across
# Recruit/Lieutenant/Commander: material values, fixed bonuses and discounts,
# and the priority bumps that order which unit gets looked at first.
# =============================================================================

# ---- material, in pawns -----------------------------------------------------
PAWN_VALUE = 1.0
KNIGHT_VALUE = 3.0
BISHOP_VALUE = 3.25
ROOK_VALUE = 5.0
QUEEN_VALUE = 9.0
KING_VALUE = 60.0  # huge but not infinite: capturing the king only loads it into a convoy that must still be walked home
KING_CARGO_VALUE = 50.0  # a convoy carrying a captured king is worth the king riding inside it
GHOST_DISCOUNT = 0.5  # a remembered, possibly-stale fogged occupant is trusted less than something actually visible

# ---- safety: trading and danger --------------------------------------------
SAFE_TRADE_DISCOUNT = 0.35  # a trade that already wins material is treated as a fraction of the risk
RECAPTURE_DISCOUNT = 0.7  # a square a friendly piece guards is treated as a fraction of the risk (a recapture is available)
KING_CARGO_ESCORT_RISK = KING_CARGO_VALUE  # losing the escort loses the captured king it carries
ROUTE_REPLACE_URGENCY_VALUE = 1.0  # tier-independent bounded cost of cancelling a route that is close to settling
# ROUTE_REPLACE_BASELINE (sneat-co/chessraiders#84, DEFECT B): retaining the
# current route is the zero-score alternative, and a replacement candidate
# must beat it — but ROUTE_REPLACE_URGENCY_VALUE's own deterrent is bounded
# by REMAINING charge time (near-settlement is expensive to cancel, a
# freshly-started charge is nearly free to cancel), and replacing a route
# resets its remaining time to full, so that deterrent never accumulates:
# the SAME actor can be re-targeted at a fresh destination every single
# decision, forever, without ever crossing "close to settling". Measured
# directly (pinned-legacy Lieutenant-vs-Lieutenant, beaconAggression=0.3,
# 5 seeds x 500 ticks): 26-36 routes replaced per side out of 39-52 accepted,
# only 33-79% of started charges ever completing, and the SAME actor
# re-targeted at a fresh square almost every decision — a low-value quiet
# reposition (Lieutenant's own advance weight is small: measured ~0.03-0.09
# for an opening pawn push) was enough to unseat an already-committed route
# every single time. A flat positive margin makes replacement require
# CLEARLY outscoring retention, not merely beating it by an epsilon,
# while staying BELOW TestFirstEngineerRouteRetentionExcludesNonPriorityRoutes'
# own asserted floor (go/bots/standardbot/strategy_test.go) for a genuinely
# attractive Commander-tier replacement (an affordable Kill/Capture nets
# ~0.75-1.0 there before this margin) — 0.7 is the largest value that keeps
# that whole suite green (bisected empirically; 0.75 already fails it),
# confirming the fix stays inside the tier's own already-tested "a real
# capture must still win" contract. NOTE: this margin fixes the LOW-value
# churn class measured above. It does NOT fully suppress a SEPARATE,
# HIGHER-value oscillation found during the same investigation (a
# Commander-tier kill vs. targetLock-dodge swing measured at 9.6 vs. 1.6-1.8
# — see the report for sneat-co/chessraiders#84): a margin large enough to
# beat that swing would exceed 0.75 and break the test above, so that
# remaining case is reported as open, not silently forced through here.
ROUTE_REPLACE_BASELINE = 0.7
# COMMITMENT_DECAY (sneat-co/chessraiders#84, route-commitment mechanism —
# the remaining open case ROUTE_REPLACE_BASELINE's own comment just
# described): that margin is a FLAT wall — retention scores 0 plus the
# margin, regardless of what the in-flight route is actually worth — which
# is exactly why it cannot fix a swing the flat margin cannot see coming: a
# Commander-tier kill charge measured at ~9.6 was replaced by a
# targetLock-dodge measured at ~1.6-1.8, then re-proposed, forever, because
# ANY candidate scoring above 0.7 unseats ANY committed route no matter how
# valuable that route itself is. retention_score (below, alongside
# COMMIT_KIND_ROUTE/_KING) instead remembers the WINNING proposal's own
# score at the moment it was submitted (build_memory's "committed" entry,
# written by next_committed) and prices retention as committed_score *
# COMMITMENT_DECAY^age + ROUTE_REPLACE_BASELINE, age counted in decisions
# since that commitment (the same currency this file's other memory counters
# already use). A fresh 9.6 needs a replacement scoring above
# 9.6*0.9 + 0.7 = 9.34, so the 1.6-1.8 dodge can never unseat it; a
# genuinely better capture still wins immediately (it outscores the
# committed value, not merely the flat margin); and a STALE commitment
# decays toward the flat baseline as the position changes, so v0.0.7's own
# flat-margin behaviour is the LIMIT case as age grows, never regressed —
# TestFirstEngineerRouteRetentionExcludesNonPriorityRoutes stays green
# because a fresh commitment's floor can only rise above ROUTE_REPLACE_
# BASELINE, never fall below it. A SCRIPT CONSTANT, not a param (founder
# call, 2026-08-13, route-commitment design doc: constant over a per-tier
# tuning knob — "smallest surface, delta-enforcement untouched"). 0.9 is the
# measured choice among {0.85, 0.9, 0.95} against the founder's own
# validation matrix (churn, engagement, winner-horizon, the 12k-tick soak);
# see the route-commitment PR description for the full table each value
# produced.
COMMITMENT_DECAY = 0.9

# ---- the win condition: walking a captured king home -----------------------
DELIVERY_BONUS = 500.0  # reaching a delivery square ends the match in our favour
DELIVERY_STEP_VALUE = 4.0  # value of one convoy step closer to delivery, before the tier's own delivery weight
UNBLOCK_BASE_VALUE = 20.0  # reward for a friendly piece giving way off the convoy's only remaining home square
UNBLOCK_VALUE_SPREAD = 1.0  # coefficient on the blocker's own value, so the cheapest blocker gives way first
UNREACHABLE_PATH_COST = 99  # the host's own sentinel for "no convoy route home at all" (geometry.go)

# ---- prisoner logistics: escorting a captive home instead of parading it ---
CAPTURE_ALIVE_BONUS = 0.25  # extra credit for a capturable piece a Prisoner-minded tier could take alive instead of killing it
PRISONER_STEP_VALUE = 1.5  # value of one step closer to base for a captive convoy, before the tier's own prisoner weight
# VALUE_VETERAN_BOOTSTRAP is the extra push toward a PAWN's own capture while
# this side still needs its first Master Engineer (board.
# needs_first_master_engineer — spec: veteran-progression; founder,
# 2026-08-07: "first of all bot should try to capture and bring prisoner ...
# pawn guarded by other pieces"). Only the escort that
# PERSONALLY makes a capture ever earns the ladder's credit
# (chess.CaptiveRef.CapturedBy), so a pawn's own capture has to outscore the
# identical capture made by any other rank — otherwise the usual biggest-
# material-wins ordering would happily let a knight or a rook take the same
# piece instead, and the pawn (or the Engineer it later becomes; Profession
# changes but Rank never does, so this same bonus reaches the ladder's
# second personal delivery too) would never personally deliver anything.
# Scaled by params["prisoner"], like every other prisoner-logistics term
# here — mirrors bot4chess/score.go's own valueVeteranBootstrap.
VALUE_VETERAN_BOOTSTRAP = 4.0
# PRIORITY_CAPTIVE_DELIVERY_SCORE is a MARKER, not a competing magnitude —
# see priority_captive_delivery_proposal's own doc comment. The proposal it
# labels is never mixed into move_proposals/system_proposals' shared pool
# (decide() returns it before either ever runs), so this value only has to
# clear rank_options' own passBelow floor for the Adviser to have something
# to show; bot4chess/score.go has NO number here at all, only an early
# return (this whole rule is a short-circuit, not a score — see decide()'s
# own call site).
PRIORITY_CAPTIVE_DELIVERY_SCORE = 1000.0

# ---- positional pressure ----------------------------------------------------
ADVANCE_PAWN_MULTIPLIER = 2.0  # a pawn's forward progress counts double
PROMOTION_BONUS = 8.0  # extra value for a pawn stepping onto its promotion rank
PROMOTION_NEXT_BONUS = 4.0  # a safe pawn setup that leaves an exact promotion destination next turn, deliberately below PROMOTION_BONUS
LAST_RANK_INDEX = 7  # squares are 0-indexed 0..7; this is the board's far edge
DEVELOP_FIRST_FORWARD_VALUE = 1.0  # first forward officer move puts a dormant unit into play
PATROL_GAIN_VALUE = 0.5  # one host-reported patrol square is useful, but never material-sized
PATROL_GAIN_CAP = 4  # broad vision is useful, not an unbounded material substitute
SUPPORT_MATERIAL_CAP = 6.0  # generic support facts are formation preferences, never a pawn-cluster multiplier
GUARDS_VALUE = 0.5  # value per capped friendly material newly guarded, before the advance dial
GUARDED_BY_VALUE = 0.5  # value of capped backing for the mover itself, before the safety dial
SOLE_GUARD_LOST_VALUE = 0.5  # penalty per capped friendly material whose only guard moved away
KING_VISIBLE_ATTACK_BONUS = 30.0  # host-proven post-settlement attack on a visible king; the script never guesses rank geometry
UNKNOWN_QUIET_PENALTY = 1.0  # a positional guess cannot tie a known, supported move
UNSUPPORTED_QUIET_PENALTY = 0.75  # forward pressure needs a friendly guard after the move

# ---- king and Beacon leadership --------------------------------------------
LEADER_SUPPORT_SATURATION = 2.0  # two weighted nearby supporters saturate the guard preference
LEADER_EXCESS_MORALE = 3  # enough spare morale: stop turning the king into a hero
MORALE_PUSH_VALUE = 14.0  # a guarded leader move must outscore ordinary development while morale is needed; the .3/.6/.9 rows remain the permission and dial
LEADER_RETREAT_VALUE = 0.5  # excessive morale makes a measured regroup half as valuable as an advance

# ---- reacting to a public threat --------------------------------------------
TARGET_LOCK_DODGE_VALUE = 2.0  # base reward for moving a publicly-locked piece at all
TARGET_LOCK_SAFE_VALUE = 1.0  # extra reward if the dodge also lands somewhere safe
KING_GUARD_BONUS = 0.5  # reward for capturing a piece that was itself attacking our king

# ---- which unit gets looked at first (unit_priority) ------------------------
UNIT_PRIORITY_KING_CARGO = 100.0  # a king-carrying convoy is always considered first: delivery wins the match
UNIT_PRIORITY_PRISONER_CARGO = 20.0  # a captive-carrying convoy is considered early when the tier values prisoners
UNIT_PRIORITY_LOCKED_BONUS = 5.0  # a publicly target-locked piece is considered early so it can dodge
UNIT_PRIORITY_KING_ITSELF = 8.0  # the king itself, when it is under threat
UNIT_PRIORITY_FORMATION_LEADER = 40.0  # admission-only: above ordinary positioning, below an exact affordable king capture (+60); weights remain the scoring dial
UNIT_PRIORITY_NEAR_KING = 3.0  # a piece standing near a threatened king
UNIT_PRIORITY_KING_VISIBLE_ATTACK = 30.0  # exact safe post-move king attack enters shallow breadth, below an affordable capture (+60)
NEAR_KING_RADIUS = 2  # how close counts as "near" the king for the bump above
RANK_PRIORITY_SCALE = 100.0  # a tiny, stable preference for a more valuable piece, once every bump above is applied

# ---- system actions: Beacon deploy/restore/forge/hand-off -------------------
# SYSTEM_BEACON_HAND_OFF_VALUE mirrors bot4chess/systems.go's own
# valueBeaconHandOff (5.0) exactly, deliberately kept ABOVE
# SYSTEM_BEACON_DEPLOY_VALUE/SYSTEM_BEACON_RESTORE_VALUE just below (4.0/3.0
# there too) — so whoever reads them sees the ranking without re-deriving
# why. Freeing a king that started the match as the Beacon's own bearer
# (BeaconRules.KingStartsAsBearer) is a PREREQUISITE for the Beacon — and
# morale, which is derived from the king's own board rank, and every
# morale-gated capture — being worth anything at all: a king that never
# moves keeps morale at 0 forever. See beacon_hand_off_proposal's own
# comment for the full accounting.
SYSTEM_BEACON_HAND_OFF_VALUE = 5.0
# SYSTEM_BEACON_DEPLOY_VALUE/SYSTEM_BEACON_RESTORE_VALUE mirror bot4chess/
# systems.go's own valueBeaconDeploy (4.0) and valueBeaconRestore (3.0)
# exactly — see beacon_deploy_or_restore_proposal's own comment.
SYSTEM_BEACON_DEPLOY_VALUE = 4.0
SYSTEM_BEACON_RESTORE_VALUE = 3.0
# SYSTEM_BEACON_FORGE_VALUE mirrors bot4chess/systems.go's own
# valueBeaconForge (3.2) exactly: it sits close to SYSTEM_BEACON_RESTORE_VALUE
# — both recreate a Lost Beacon — but very slightly above it, because
# Forging spends a pawn rather than tying up the king's own turn, so it is
# the marginally cheaper recovery, not because the founder ranked it above
# Restore. See beacon_forge_proposals' own comment.
SYSTEM_BEACON_FORGE_VALUE = 3.2

# ---- system actions: training ------------------------------------------------
# SYSTEM_TRAIN_VALUE / SYSTEM_ADVANCED_TRAIN_VALUE mirror bot4chess/systems.go's
# own valueTrain (3.0) and valueAdvancedTrain (3.5) exactly — see
# training_proposals' own comment.
SYSTEM_TRAIN_VALUE = 3.0
SYSTEM_ADVANCED_TRAIN_VALUE = 3.5

# ---- bookkeeping sentinels ---------------------------------------------------
TIE_BREAK_BAND = 1e-9  # candidates within this band of the best score are "equally good" and go to the tie-break draw
UNREACHABLE_DISTANCE = 99  # larger than any real board distance; a plain sentinel, not a scoring weight
NO_SQUARE_INDEX = -1  # mirrors the host's own chess.NoSquare: "nothing here"
MAX_SQUARE_INDEX = 63  # the highest valid board index (an 8x8 board, 0-indexed)
BOARD_FILES = 8  # files per rank, for packing (file, rank) into one 0..63 index
REFUSED_SET_SIZE = 4  # drop_refused's own small ring buffer size — see that function's own doc comment for why ONE remembered square was not enough
QUIET_VACATED_SQUARES = 3  # enough to break the observed queen shuffle without turning Recruit's tiny memory into a board history
MILLISECONDS_PER_SECOND = 1000.0  # unit conversion for the Tempo term's charge duration
BITBOARD_SIGN_BIT = 1 << 63
BITBOARD_MODULUS = 1 << 64
PLACEMENT_SQUARES_PER_SLOT = 13  # 13 nibbles (52 bits) per packed placement memory entry — see pack_placement_boards' own doc comment
PLACEMENT_BOARD_SLOTS = 5  # ceil(64 / PLACEMENT_SQUARES_PER_SLOT) packed entries cover every one of the 64 squares
JS_SAFE_INTEGER_MAX = 9007199254740991  # 2**53 - 1 — the largest integer magnitude a browser's float64 JSON numbers still round-trip exactly (Number.MAX_SAFE_INTEGER)


# =============================================================================
# Small, geometry-free helpers. Chebyshev distance and forward-progress are
# arithmetic on squares the host already gave us — never a question of
# whether a piece could physically get there, which is exactly the kind of
# geometry this script must not compute (see the host's own geometry.go).
# =============================================================================

def square_file(square):
    """The 0..7 file (column) of an algebraic square like "e4"."""
    return ord(square[0]) - ord("a")

def square_rank_number(square):
    """The 0..7 rank (row) of an algebraic square like "e4"."""
    return ord(square[1]) - ord("1")

def square_index(square):
    """The host's own 0..63 board index for an algebraic square, so a
    memory entry can be compared against facade4chess.BotMemory's raw
    square-index encoding."""
    return square_file(square) * BOARD_FILES + square_rank_number(square)

def square_name(index):
    """The inverse of square_index."""
    file_number = index // BOARD_FILES
    rank_number = index % BOARD_FILES
    return chr(ord("a") + file_number) + chr(ord("1") + rank_number)

def chebyshev_distance(first_square, second_square):
    """King-move distance between two GIVEN squares — measuring, never
    asking whether either square is reachable."""
    file_gap = abs(square_file(first_square) - square_file(second_square))
    rank_gap = abs(square_rank_number(first_square) - square_rank_number(second_square))
    return file_gap if file_gap > rank_gap else rank_gap

def forward_progress(side, square):
    """How far a square is toward the enemy back rank, 0..7."""
    rank_number = square_rank_number(square)
    return rank_number if side == "white" else LAST_RANK_INDEX - rank_number

def is_promotion_square(side, square):
    return forward_progress(side, square) == LAST_RANK_INDEX

def rank_value(rank):
    return {
        "pawn": PAWN_VALUE,
        "knight": KNIGHT_VALUE,
        "bishop": BISHOP_VALUE,
        "rook": ROOK_VALUE,
        "queen": QUEEN_VALUE,
        "king": KING_VALUE,
    }.get(rank, 0.0)

def cell_value(cell):
    """What capturing this projected occupant is worth."""
    value = rank_value(cell["rank"])
    if cell["kingCargo"]:
        value += KING_CARGO_VALUE
    if cell["ghost"]:
        value *= GHOST_DISCOUNT
    return value

def distance_to_nearest_enemy(square, enemy_cells):
    closest = UNREACHABLE_DISTANCE
    for enemy_cell in enemy_cells:
        gap = chebyshev_distance(square, enemy_cell["square"])
        if gap < closest:
            closest = gap
    return closest

def distance_to_nearest_square(square, candidate_squares):
    closest = UNREACHABLE_DISTANCE
    for candidate in candidate_squares:
        gap = chebyshev_distance(square, candidate)
        if gap < closest:
            closest = gap
    return closest

EMPTY_CANDIDATE = {}

def candidate_at(observation, source_square, destination):
    return observation.get("candidates", {}).get(source_square, {}).get(destination, EMPTY_CANDIDATE)

def has_candidate(observation, source_square, destination):
    """Whether the host supplied a deterministic post-state candidate."""
    return observation.get("candidates", {}).get(source_square, {}).get(destination) != None

def relation_squares(subject, relation):
    """Relationship arrays are sorted/deduplicated by the host and omitted
    when empty. The script reads that compact wire without inventing geometry."""
    return subject.get(relation, []) or []

def guarded_count(subject):
    return len(relation_squares(subject, "guardedBy"))

def threatened_count(subject):
    return len(relation_squares(subject, "threatenedBy"))

def supported_and_unthreatened(subject):
    return guarded_count(subject) > 0 and threatened_count(subject) == 0

def is_safe_subject(subject):
    """The common safety gate: a friendly guard, or no visible threat."""
    return guarded_count(subject) > 0 or threatened_count(subject) == 0

def projected_cell(observation, square):
    """Returns a transient square-bearing copy of one projected piece.
    The wire keys pieces by square and intentionally omits Cell.square."""
    raw = observation["pieces"].get(square)
    if raw == None:
        return None
    cell = dict(raw)
    cell["square"] = square
    return cell

def piece_squares(observation):
    """Projected-piece object order is non-semantic. Always recover the
    engine's file-major square order explicitly before a decision depends on
    iteration order."""
    return sorted(observation["pieces"].keys(), key = square_index)

def beacon_aggression(observation, params):
    """The positive weight is both Beacon permission and preference."""
    return params["beaconAggression"]

def is_quiet_move(observation, board, cell, destination):
    """A quiet destination has neither a direct target nor host-confirmed
    en-passant victim. Captures retain their tactical scoring unchanged."""
    if board["enemy_by_square"].get(destination):
        return False
    return not observation.get("enPassant", {}).get(cell["square"], {}).get(destination)

def protection_factor(candidate):
    return min(1.0, guarded_count(candidate) / 2.0)


# =============================================================================
# Board bookkeeping: which projected pieces are mine, which may be commanded
# now, and where the kings stand. `pieces` is keyed by algebraic square and a
# Cell deliberately omits that redundant field, so this builds transient
# copies in sorted-square order. It never mutates the decoded observation.
# =============================================================================

def build_board(observation):
    own_cells = []
    enemy_cells = []
    own_by_square = {}
    enemy_by_square = {}
    all_by_square = {}
    for square in piece_squares(observation):
        cell = projected_cell(observation, square)
        all_by_square[square] = cell
        if cell["side"] == observation["side"]:
            own_cells.append(cell)
            own_by_square[square] = cell
        else:
            enemy_cells.append(cell)
            enemy_by_square[square] = cell

    locked_units = {}
    for cell in own_cells:
        if cell.get("targetLocked"):
            locked_units[cell["unitId"]] = True

    busy_units = {}
    charging_units = {}
    protected_charging_units = {}
    replaceable_charging_units = {}
    nonroute_busy_units = {}
    # has_master_engineer mirrors bot4chess/observe.go's board.
    # needsFirstMasterEngineer exactly (spec: retire-the-go-tiers#A4(f)):
    # whether this side has fielded an Engineer who has ALREADY reached
    # Master grade is public specialist identity. It is not inferred from a
    # unit ID: IDs remain opaque when recruitment changes ownership.
    has_master_engineer = False
    king_cell = None
    own_interrogation_targets = {}
    enemy_interrogation_active = False
    for cell in own_cells + enemy_cells:
        if cell.get("interrogationRemainingMs") != None:
            if cell["side"] == observation["side"]:
                own_interrogation_targets[cell["unitId"]] = True
            else:
                enemy_interrogation_active = True
    for cell in own_cells:
        unit_id = cell["unitId"]
        if cell.get("profession", "") == "engineer" and cell.get("grade") == "master":
            has_master_engineer = True
        if cell["rank"] == "king" and not cell["convoy"]:
            king_cell = cell
        charging = cell.get("charging")
        if charging:
            charging_units[unit_id] = True
            busy_units[unit_id] = True
            target = enemy_by_square.get(charging["square"])
            # A charge already committed to the visible enemy king is the
            # win attempt: retain it. Lower-value or now-empty destinations
            # remain replaceable through the ordinary legal proposal path.
            if target and target["rank"] == "king" and not target["ghost"]:
                protected_charging_units[unit_id] = True
            else:
                replaceable_charging_units[unit_id] = True
        # "forging" joins "training" here, never charging_units: a Beacon
        # Forge channel locks a pawn in place exactly like a training
        # channel does (observation.go's forgingRemainingMs contract), so a
        # unit mid-Forge must read busy the identical way — omitting it let
        # this bot propose an ordinary quiet move on its OWN forging pawn,
        # which apply's shared locked-channel cancellation then reads as
        # "cancel the Forge", destroying work already in flight.
        if cell.get("training") or cell.get("forgingRemainingMs", 0) > 0 or cell.get("recoveryRemainingMs", 0) > 0:
            busy_units[unit_id] = True
            nonroute_busy_units[unit_id] = True
        if own_interrogation_targets.get(unit_id):
            busy_units[unit_id] = True
            nonroute_busy_units[unit_id] = True

    # Only a king interrogates. If the target is opposing, our own king is
    # the implicit source and is just as busy as the target.
    if enemy_interrogation_active and king_cell:
        busy_units[king_cell["unitId"]] = True
        nonroute_busy_units[king_cell["unitId"]] = True

    actionable_units = [cell for cell in own_cells if not busy_units.get(cell["unitId"])]
    if charging_units:
        # One command front at a time. While any own route is active, the
        # only permitted exception is replacing that same lower-value route;
        # idle pieces cannot start a second move alongside it.
        move_actionable_units = [cell for cell in own_cells if (replaceable_charging_units.get(cell["unitId"]) and
                                                                not nonroute_busy_units.get(cell["unitId"]))]
    else:
        move_actionable_units = actionable_units
    king_threatened = king_cell != None and threatened_count(king_cell) > 0

    needs_first_master_engineer = observation["rules"]["veteranProgression"] and not has_master_engineer

    return {
        "side": observation["side"],
        "own": own_cells,
        "enemy": enemy_cells,
        "all_by_square": all_by_square,
        "own_by_square": own_by_square,
        "enemy_by_square": enemy_by_square,
        "locked_units": locked_units,
        "busy_units": busy_units,
        "charging_units": charging_units,
        "protected_charging_units": protected_charging_units,
        "actionable_units": actionable_units,
        "move_actionable_units": move_actionable_units,
        "needs_first_master_engineer": needs_first_master_engineer,
        "king_cell": king_cell,
        "king_threatened": king_threatened,
        "delivery_squares": observation.get("deliverySquares", []),
        "convoy_home": observation.get("convoyHome", {}),
        "blocking_base": observation.get("blockingBase", []),
    }


# =============================================================================
# Scoring. One term per line, in the order bot4chess/score.go's scoreMove
# accumulates them, so a reader can check this against the Go it replaced
# term for term. Every weight comes from `params`; every fixed value above
# is a named constant; nothing here is a bare number.
# =============================================================================

# TERM_DETAILS is the CLOSED SET this script declares (adviser-tier's
# REQ:explanation-is-structured-terms-and-the-client-renders). Its keys are
# the parameter-backed score categories plus the named positional/cycle terms
# this script owns. The values discriminate BRANCHES a magnitude alone cannot
# explain: a `delivery` term that ENDS the match, one that steps closer, one
# that steps off the only route home, and one that clears a blocked base square
# are four different things a player should hear differently.
#
# This dict is documentation the script owns rather than a lookup anything
# reads at runtime — add_term takes the detail as a literal at each call site,
# where the branch actually fires. A client that meets a detail it does not
# recognise MUST fall back to rendering the bare term name, which is what
# makes adding a discriminator here a non-breaking change forever.
TERM_DETAILS = {
    "material": ["capture"],
    "safety": ["risk", "kingIntoStrike", "unknownQuiet", "unsupportedQuiet", "guardedBy", "soleGuardLost"],
    "tempo": ["charge", "replaceCharge"],
    "advance": ["pawn", "promotion", "promotionNext", "piece"],
    "targetLock": ["dodge", "safeDodge"],
    "kingSafety": ["escape", "guard"],
    "moralePush": ["guardedAdvance", "excessAdvance", "excessRetreat"],
    "beaconAggression": ["handOff", "deploy", "restore", "forge", "guardedAdvance", "regroup"],
    "develop": ["firstForward"],
    "coverage": ["patrol", "guards"],
    "kingHunt": ["visible"],
    "repeatPenalty": ["quiet"],
    "delivery": ["wins", "closer", "offRoute", "drift", "unblock"],
    "prisoner": ["alive", "escort", "bootstrap", "priorityDelivery"],
    "system": [
        "train", "advancedTrain",
        "repairWall", "dismantleWall", "sergeantSupported",
    ],
}

def add_term(terms, term, value, detail):
    """Records ONE signed contribution and returns the identical number, so
    every call site reads `score += add_term(terms, ...)` — the breakdown and
    the total are written by one statement and cannot disagree. That property
    is asserted rather than assumed (AC:one-scoring-path-two-outputs: the sum
    of an option's terms equals its score within floating-point tolerance);
    writing it this way is what makes the assertion cheap to keep true.

    A contribution of exactly zero is NOT recorded: a term whose weight is
    zero, or whose branch computed nothing, is ABSENT from the breakdown
    rather than present as a zero — a reason worth nothing is not a reason,
    and showing one would pad an explanation with noise."""
    if value == 0.0:
        return 0.0
    entry = {"term": term, "value": value}
    if detail:
        entry["detail"] = detail
    terms.append(entry)
    return value

def unit_priority(observation, board, params, cell):
    """Orders units before any move is scored: pieces near the action
    first, a king-carrying convoy always (delivery wins the match), and a
    publicly-locked piece early so it can dodge."""
    priority = -distance_to_nearest_enemy(cell["square"], board["enemy"])
    if cell["convoy"] and cell["kingCargo"]:
        priority += UNIT_PRIORITY_KING_CARGO
    elif cell["convoy"] and cell["cargoCount"] > 0 and params["prisoner"] > 0:
        priority += UNIT_PRIORITY_PRISONER_CARGO
    if params["targetLock"] > 0 and board["locked_units"].get(cell["unitId"]):
        priority += UNIT_PRIORITY_LOCKED_BONUS
    if params["kingSafety"] > 0 and board["king_threatened"]:
        if cell["rank"] == "king":
            priority += UNIT_PRIORITY_KING_ITSELF
        elif board["king_cell"] and chebyshev_distance(cell["square"], board["king_cell"]["square"]) <= NEAR_KING_RADIUS:
            priority += UNIT_PRIORITY_NEAR_KING
    priority += formation_leader_priority(observation, board, params, cell)
    if not cell["convoy"] and cell["rank"] != "king" and not is_current_beacon_bearer(observation, cell):
        for enemy in board["enemy"]:
            if enemy["rank"] != "king" or enemy["ghost"]:
                continue
            if (enemy["square"] in observation["legal"].get(cell["square"], []) and
                    capture_choice_available(observation, board, cell, enemy["square"])):
                priority += KING_VALUE
                break
            for destination in observation["legal"].get(cell["square"], []):
                fact = candidate_at(observation, cell["square"], destination)
                if (is_quiet_move(observation, board, cell, destination) and
                        has_candidate(observation, cell["square"], destination) and fact.get("destinationVisible", False) and
                        supported_and_unthreatened(fact) and
                        enemy["square"] in (fact.get("nextPossibleMoves", []) or [])):
                    priority += UNIT_PRIORITY_KING_VISIBLE_ATTACK
                    break
    priority += rank_value(cell["rank"]) / RANK_PRIORITY_SCALE  # stable, tiny rank preference
    return priority

EMPTY_OUTCOMES = {}
EMPTY_OUTCOME = {
    "affordable": False, "requiredMorale": 0, "availableMorale": 0,
    "oddsKnown": False, "odds": {"success": 0, "defenderKilled": 0, "repelled": 0},
}

def outcomes_at(observation, source_square, destination):
    """The host's own per-choice affordability AND odds facts for one (unit,
    destination) pair (observation.affordability — observation.go's
    DestinationOutcomes), keyed exactly like observation.legal itself.
    Absent only for a quiet-move destination (nothing to afford); this
    script never computes the morale rule OR the odds that fill it in, only
    reads the engine's own answer (observation.go's CaptureOutcome, sourced
    from chess.BeliefCaptureBands)."""
    return observation["affordability"].get(source_square, EMPTY_OUTCOMES).get(destination, EMPTY_OUTCOMES)

def capture_expected_success(outcome):
    """The fraction (0.0-1.0) of the time `outcome` (one CaptureOutcome —
    Kill or Capture) actually lands, for weighing EXPECTED material rather
    than the mere fact that a capture is affordable (REQ:bot-sees-capture-
    odds — a capture the bot can afford but will probably lose is not the
    same value as one it will probably win).

    1.0 (certain) whenever the host reports no forecast (outcome["oddsKnown"]
    is False): a King/convoy contact (never rolled), a Kill under today's
    default deterministic KillSuccessPercent, and capture risk disabled
    outright ALL fall through to this default — every one of them IS
    certain, and 1.0 is exactly this script's own behaviour before odds
    existed at all, so a ruleset that never enables capture risk sees no
    scoring change from this function's existence."""
    if not outcome["oddsKnown"]:
        return 1.0
    return outcome["odds"]["success"] / 100.0

def effective_capture_target(observation, board, cell, destination):
    """Resolves what `cell` would actually capture by moving to
    `destination`: the piece standing there for an ordinary capture, or —
    for an unescorted pawn whose destination is empty — the en-passant
    victim beside it (mirrors bot4chess/score.go's own
    effectiveCaptureTarget, spec: retire-the-go-tiers#A4(e)). An en-passant
    capture lands the attacking pawn on an EMPTY square — the victim never
    stands on `destination` — which is exactly why
    board["enemy_by_square"].get(destination) alone always misses it. The
    victim's identity is HOST-computed (observation.enPassant, itself
    sourced from chess.State.EnPassantVictim) and never re-derived here:
    this script has no authority over the en-passant window or geometry,
    only the host's own answer for this exact (unit, destination) pair. A
    convoy escort can never be the attacker (the host oracle's own
    IsConvoy() guard), so cell["convoy"] short-circuits before the lookup,
    mirroring Go's own guard."""
    target_cell = board["enemy_by_square"].get(destination)
    if target_cell:
        return target_cell
    if cell["convoy"] or cell["rank"] != "pawn":
        return None
    victim_square = observation["enPassant"].get(cell["square"], {}).get(destination)
    if not victim_square:
        return None
    return board["enemy_by_square"].get(victim_square)

def capture_choice_available(observation, board, cell, destination):
    """Is this (unit, destination) pair one the bot may actually submit?
    True for an ordinary quiet-move destination (nothing to afford). For a
    destination holding an enemy: a king or convoy target is ALWAYS
    resolved through the same morale-gated capture path regardless of
    CaptureMode or requested choice (chess/apply.go's resolveCaptureOutcome
    routes both around the kill switch entirely — see
    destinationOutcomes' own doc comment, observation.go), so only its
    Capture affordability matters, never rules.allowsKill/allowsCapture.
    For an ordinary piece, either an allowed-and-affordable Kill or an
    allowed-and-affordable Capture makes the pair usable — Kill is always
    affordable there (kills are never morale-gated), so this is really
    asking "does the ruleset even allow ONE of the two outcomes" for a
    plain undefended piece, and asking the real morale question only when
    Capture is the one being relied on.

    A destination whose only available outcome is unaffordable is not
    proposed at all: submitting it would only be refused by the engine,
    wasting the bot's turn on a command it never had a chance of winning —
    the whole point of this guard (see the accompanying report's
    rejection-rate measurement)."""
    target_cell = effective_capture_target(observation, board, cell, destination)
    if not target_cell:
        return True
    outcomes = outcomes_at(observation, cell["square"], destination)
    if target_cell["rank"] == "king" or target_cell["convoy"]:
        return outcomes.get("capture", EMPTY_OUTCOME)["affordable"]
    if observation["rules"]["allowsKill"] and outcomes.get("kill", EMPTY_OUTCOME)["affordable"]:
        return True
    if observation["rules"]["allowsCapture"] and outcomes.get("capture", EMPTY_OUTCOME)["affordable"]:
        return True
    return False

def leader_support(observation, board, leader, destination):
    """Friendly guarding at a candidate leader square. A supporter ahead of
    the leader counts fully, one abreast counts half, and one behind cannot
    make an advance safer. Nearer supporters count more; two weighted units
    saturate rather than turning this 0..1 preference into a material score."""
    support = 0.0
    destination_progress = forward_progress(board["side"], destination)
    for ally in board["own"]:
        if ally["unitId"] == leader["unitId"]:
            continue
        distance = chebyshev_distance(destination, ally["square"])
        if distance == 0:
            continue
        ally_progress = forward_progress(board["side"], ally["square"])
        direction_weight = 1.0 if ally_progress > destination_progress else (0.5 if ally_progress == destination_progress else 0.0)
        support += direction_weight / distance
    return min(1.0, support / LEADER_SUPPORT_SATURATION)

def current_morale_need(observation):
    """The highest currently visible capture threshold, also bounded by the
    number already managed. The host owns affordability; this only asks how
    much current work more morale would unlock."""
    needed = observation.get("ownManaged", 0)
    for by_destination in observation.get("affordability", {}).values():
        for outcomes in by_destination.values():
            needed = max(needed, outcomes.get("capture", EMPTY_OUTCOME).get("requiredMorale", 0))
    return needed

def post_move_morale(observation, board, from_square, destination):
    """Do not score the king as current morale plus forward gain: an
    incursion penalty already paid at the current square follows the king.
    The host projects that exact applied 0/1 penalty because reconstructing
    it from a clamped morale value is ambiguous at zero."""
    current_penalty = observation.get("ownMoralePenalty", 0)
    return max(0, forward_progress(board["side"], destination) - current_penalty)

def is_current_beacon_bearer(observation, cell):
    beacon = observation.get("beacon", {})
    return beacon.get("lifecycle") == "deployed" and beacon.get("bearerSquare") == cell["square"]

def can_promote_next_move(side, candidate):
    """The host's own post-settlement moves, not a pawn-geometry guess. An
    omitted/empty list means no known next promotion setup."""
    for next_destination in (candidate.get("nextPossibleMoves", []) or []):
        if is_promotion_square(side, next_destination):
            return True
    return False

def support_material(board, squares):
    """Material on named friendly squares, capped before a term applies.
    The host determines which squares are guarded or abandoned; the script
    only prices their existing occupants."""
    material = 0.0
    for square in (squares or []):
        cell = board["own_by_square"].get(square)
        if cell:
            material += min(rank_value(cell["rank"]), ROOK_VALUE)
    return min(material, SUPPORT_MATERIAL_CAP)

def capture_backing_count(target_cell, mover_square):
    """Other friendly attackers of a capture target can recapture after the
    mover settles there; the mover's own source is not backing itself."""
    return len([square for square in relation_squares(target_cell, "threatenedBy") if square != mover_square])

def guard_changes(board, cell, candidate):
    """Returns genuinely new mover->ally guard edges and allies losing their
    sole guard. Moving one of several redundant guards is deliberately free:
    it does not leave the ally unguarded."""
    old_square = cell["square"]
    before = relation_squares(cell, "guards")
    after = relation_squares(candidate, "guards")
    newly_guarded = []
    sole_guard_lost = []
    for square in after:
        target = board["own_by_square"].get(square)
        if target and old_square not in relation_squares(target, "guardedBy"):
            newly_guarded.append(square)
    for square in before:
        if square in after:
            continue
        target = board["own_by_square"].get(square)
        if target and relation_squares(target, "guardedBy") == [old_square]:
            sole_guard_lost.append(square)
    return newly_guarded, sole_guard_lost

def inbound_support_material(cell, candidate):
    """Two real protectors are enough for this soft preference. The mover's
    material is what is protected, not the protector's rank."""
    count = guarded_count(candidate)
    return min(count, 2) * min(rank_value(cell["rank"]), ROOK_VALUE) / ROOK_VALUE

def formation_leader_priority(observation, board, params, cell):
    """Lets a leader whose current weight permits formation work enter a
    shallow tier's breadth only while it has a known, guarded, safe move that
    improves the formation. This is scheduling, not an extra move score: the
    usual morale/Beacon terms still choose the destination. A king stops
    receiving this consideration once it has the one-to-two morale reserve;
    a Beacon bearer needs real support gain, so neither becomes a hero."""
    if cell["convoy"]:
        return 0.0
    destinations = observation["legal"].get(cell["square"], [])
    if cell["rank"] == "king":
        if params["moralePush"] <= 0:
            return 0.0
        needed = current_morale_need(observation)
        current = post_move_morale(observation, board, cell["square"], cell["square"])
        # An excessive reserve is the one case where a leader should get
        # breadth preference to step BACK: score_move already awards this
        # same regroup, but shallow tiers must first get to consider it.
        if observation.get("ownMorale", 0) - needed >= LEADER_EXCESS_MORALE:
            for destination in destinations:
                fact = candidate_at(observation, cell["square"], destination)
                after = post_move_morale(observation, board, cell["square"], destination)
                if (is_quiet_move(observation, board, cell, destination) and
                        has_candidate(observation, cell["square"], destination) and fact.get("destinationVisible", False) and
                        supported_and_unthreatened(fact) and
                        after < current and after >= needed + 1):
                    return UNIT_PRIORITY_FORMATION_LEADER
            return 0.0
        # Once the king already holds the first reserve point, it no longer
        # gets breadth preference merely for being a king.
        if current >= needed + 1:
            return 0.0
        for destination in destinations:
            fact = candidate_at(observation, cell["square"], destination)
            after = post_move_morale(observation, board, cell["square"], destination)
            if (is_quiet_move(observation, board, cell, destination) and
                    has_candidate(observation, cell["square"], destination) and fact.get("destinationVisible", False) and
                    supported_and_unthreatened(fact) and
                    after > current and after <= needed + 2):
                return UNIT_PRIORITY_FORMATION_LEADER
        return 0.0
    if not is_current_beacon_bearer(observation, cell) or params["beaconAggression"] <= 0:
        return 0.0
    current_support = leader_support(observation, board, cell, cell["square"])
    for destination in destinations:
        fact = candidate_at(observation, cell["square"], destination)
        if (not (cell["rank"] == "pawn" and is_promotion_square(board["side"], destination)) and is_quiet_move(observation, board, cell, destination) and
                has_candidate(observation, cell["square"], destination) and fact.get("destinationVisible", False) and
                supported_and_unthreatened(fact) and
                leader_support(observation, board, cell, destination) > current_support):
            return UNIT_PRIORITY_FORMATION_LEADER
    return 0.0

def has_other_ordinary_choice(observation, board, current_cell):
    for cell in board["actionable_units"]:
        if cell["unitId"] == current_cell["unitId"] or cell["convoy"] or cell["rank"] == "king" or is_current_beacon_bearer(observation, cell):
            continue
        for destination in observation["legal"].get(cell["square"], []):
            if destination != cell["square"]:
                return True
    return False

def score_move(observation, board, params, memory, cell, destination):
    """Scores one (own unit, legal destination) pair, AND builds the named
    breakdown of that score. `destination` always comes from
    observation.legal — this function only ever weighs a move the host
    already said is legal; it never checks whether one is.

    THE BREAKDOWN IS NOT OPTIONAL AND NOT CONDITIONAL. There is no `explain`
    parameter and no `if explain:` branch here, deliberately
    (adviser-tier's REQ:adviser-is-an-explain-mode-not-a-third-implementation
    states it as a prohibition, because a branch that changed WHICH terms are
    evaluated would recreate the bot/adviser divergence this whole design
    exists to delete). Every caller gets `terms`; a bot decision discards it.
    The cost is a handful of dict writes per candidate, measured rather than
    assumed — see the accompanying report."""
    target_cell = effective_capture_target(observation, board, cell, destination)
    quiet_move = is_quiet_move(observation, board, cell, destination)
    candidate_known = has_candidate(observation, cell["square"], destination)
    candidate = candidate_at(observation, cell["square"], destination)
    score = 0.0
    captured_value = 0.0
    terms = []

    # capture_choice/success_chance decide, up front, which verb this
    # candidate would actually submit — the SAME preference the closing
    # intent-choice block below applies — so the material term just below
    # can weigh EXPECTED value (REQ:bot-sees-capture-odds) rather than mere
    # affordability: a capture the bot can equally afford but will probably
    # lose is not worth the same as one it will probably win. King/convoy
    # targets and a target with neither an allowed nor affordable verb keep
    # success_chance at 1.0 — exactly this script's own pre-odds behaviour
    # (see capture_expected_success's own doc comment for why that is
    # correct, not merely a fallback).
    success_chance = 1.0
    capture_choice = None
    if target_cell and target_cell["rank"] != "king" and not target_cell["convoy"]:
        outcomes = outcomes_at(observation, cell["square"], destination)
        if (params["prisoner"] > 0 and observation["rules"]["allowsCapture"] and
                outcomes.get("capture", EMPTY_OUTCOME)["affordable"]):
            capture_choice = "capture"
        elif observation["rules"]["allowsKill"]:
            capture_choice = "kill"
        if capture_choice:
            success_chance = capture_expected_success(outcomes.get(capture_choice, EMPTY_OUTCOME))

    # Material: take what is there — weighted by how often the verb this
    # candidate would submit actually lands.
    if target_cell:
        captured_value = cell_value(target_cell)
        # success_chance (capture-odds, main) weights the expected value the
        # SAME way for both the bot's own score and the adviser's explained
        # breakdown — an option is never explained by a number the bot itself
        # did not actually use to decide.
        material_gain = captured_value * success_chance * params["material"]
        # Founder formula (2026-08-07, verbatim: "decrease value of sideways
        # captures with every captured piece ... divide by (number of
        # prisoners+1)" — bot4chess/score.go's own comment: what this fights
        # is a SIDEWAYS capture (no baseDistance progress at all) outscoring
        # a homeward quiet step, since a loaded convoy can never move
        # FURTHER from home (chess/legal.go's convoyGeometry rejects that
        # outright). cell["cargoCount"] is always >=1 here — gated on
        # cargoCount>0 explicitly — so this only ever shrinks the term
        # toward zero, never inverts its sign or divides by zero.
        if (board["needs_first_master_engineer"] and cell["rank"] == "pawn" and
                cell["convoy"] and cell["cargoCount"] > 0):
            material_gain /= (cell["cargoCount"] + 1)
        score += add_term(terms, "material", material_gain, "capture")
        if target_cell["rank"] == "king" and not target_cell["ghost"]:
            score += add_term(terms, "kingHunt", params["advance"], "visible")
        if params["prisoner"] > 0 and target_cell["rank"] != "king" and not target_cell["convoy"]:
            score += add_term(terms, "prisoner", CAPTURE_ALIVE_BONUS * params["prisoner"] * success_chance, "alive")
            # Veteran-progression bootstrap (bot4chess/score.go's own
            # valueVeteranBootstrap): while this side still needs its first
            # Master Engineer, an ORDINARY (not yet loaded) pawn's own
            # capture must outscore the identical capture by any other rank
            # — only the escort that PERSONALLY makes a capture ever earns
            # the ladder's credit, so the usual biggest-material-wins
            # ordering would otherwise happily let a knight or rook take it
            # instead. Bounded to a capture this pawn will not simply be
            # recaptured out of (the target's relation graph — the board's
            # guard graph, no new machinery) and to a
            # target the morale gate actually lets THIS attacker take alive
            # (outcomes["capture"]["affordable"] — the exact fact
            # bot4chess's own prisonerAllowed answers, proven to agree with
            # this host computation on every input reachable here), so the
            # bonus never inflates a capture that settles as a Kill anyway.
            if (board["needs_first_master_engineer"] and cell["rank"] == "pawn" and not cell["convoy"] and
                    outcomes.get("capture", EMPTY_OUTCOME)["affordable"] and
                    (capture_backing_count(target_cell, cell["square"]) > 0 or guarded_count(target_cell) == 0)):
                score += add_term(terms, "prisoner", VALUE_VETERAN_BOOTSTRAP * params["prisoner"], "bootstrap")

    # Safety: a trade that already wins material is worth taking; walking a
    # queen in front of a pawn for nothing is not.
    if candidate_known:
        post_threat = threatened_count(candidate)
        post_guarded = guarded_count(candidate)
    elif target_cell:
        # Captures have branching outcomes, so the host deliberately does not
        # fabricate one post-state candidate. Enemy guards of the target are
        # the visible recapture threats. Other friendly attackers of that
        # target are the mover's potential backing (exclude the mover itself).
        post_threat = guarded_count(target_cell)
        post_guarded = capture_backing_count(target_cell, cell["square"])
    else:
        post_threat = 0
        post_guarded = 0
    # Quiet active-route replacements deliberately lack candidate facts.
    # Zero above is an arithmetic fallback, never evidence that the
    # destination is safe. Captures retain their visible-target relation
    # fallback because their outcome branches cannot have one settled fact.
    post_safety_known = candidate_known or target_cell != None
    if post_threat > 0:
        risk = rank_value(cell["rank"])
        if cell["kingCargo"]:
            risk += KING_CARGO_ESCORT_RISK
        if captured_value >= risk:
            risk *= SAFE_TRADE_DISCOUNT
        elif post_guarded > 0:
            risk *= RECAPTURE_DISCOUNT
        score += add_term(terms, "safety", -risk * params["safety"], "risk")

    # Tempo: a slower piece spends more of the match charging.
    if params["tempo"] > 0:
        charge_ms = observation["rules"]["pieceChargeMs"].get(cell["rank"], 0)
        if charge_ms > 0:
            score += add_term(terms, "tempo", -(charge_ms / MILLISECONDS_PER_SECOND) * params["tempo"], "charge")
    active_charge = cell.get("charging")
    if active_charge and destination != active_charge["square"]:
        # The wire intentionally exposes remaining time, not a fabricated
        # total duration. Near-settlement work is more urgent to retain;
        # the bounded inverse never pretends to know elapsed progress. This
        # cost is tier-independent: Recruit's ordinary tempo weight is zero,
        # but sunk route progress still exists for Recruit.
        remaining_seconds = active_charge["remainingMs"] / MILLISECONDS_PER_SECOND
        urgency = ROUTE_REPLACE_URGENCY_VALUE / (1.0 + remaining_seconds)
        score += add_term(terms, "tempo", -urgency, "replaceCharge")

    # The win condition: walk a captured enemy king home. Progress is
    # measured along the host's own real path cost over board occupancy
    # (convoyHome), so a sideways shuffle in front of a blocked line scores
    # nothing and a step through a gap two files away scores what it is
    # worth. Chebyshev distance survives only as the fallback for a
    # position with no path home at all, where homeward drift still beats
    # standing still.
    if cell["convoy"] and cell["kingCargo"]:
        if destination in board["delivery_squares"]:
            score += add_term(terms, "delivery", DELIVERY_BONUS, "wins")
        else:
            here_cost = board["convoy_home"].get(cell["square"], UNREACHABLE_PATH_COST)
            there_cost = board["convoy_home"].get(destination, UNREACHABLE_PATH_COST)
            if here_cost < UNREACHABLE_PATH_COST:
                if there_cost >= UNREACHABLE_PATH_COST:
                    # stepping off the only route home
                    score += add_term(terms, "delivery", -DELIVERY_STEP_VALUE * params["delivery"], "offRoute")
                else:
                    score += add_term(terms, "delivery",
                                      (here_cost - there_cost) * DELIVERY_STEP_VALUE * params["delivery"], "closer")
            else:
                progress = (distance_to_nearest_square(cell["square"], board["delivery_squares"]) -
                            distance_to_nearest_square(destination, board["delivery_squares"]))
                score += add_term(terms, "delivery", progress * DELIVERY_STEP_VALUE * params["delivery"], "drift")

    # Captive logistics: escort a prisoner home rather than parading it.
    elif cell["convoy"] and cell["cargoCount"] > 0 and params["prisoner"] > 0:
        prisoner_rank = "pawn" if observation["rules"]["cargoBasedDelivery"] else cell["rank"]
        base_squares = observation["rules"]["baseSquares"].get(prisoner_rank, [])
        progress = (distance_to_nearest_square(cell["square"], base_squares) -
                    distance_to_nearest_square(destination, base_squares))
        homeward = progress * PRISONER_STEP_VALUE * params["prisoner"]
        # Founder formula (2026-08-07, verbatim): "increase value of going
        # backwards - multiply by number of prisoners" — bot4chess/score.go's
        # own comment: cell["cargoCount"] is always >=1 here (this whole
        # branch is gated on cargoCount>0 above), so this only ever SCALES
        # UP a positive homeward gain and is a no-op multiply-by-zero-gain
        # for a sideways/no-progress candidate; it can never zero out or
        # invert homeward's own sign. At exactly one prisoner the multiplier
        # is x1 (no boost yet); the pull to get home grows with every
        # additional one this escort is holding. Gated to a Pawn-rank
        # escort, same as the capture-divide above: only that rank can ever
        # earn the ladder credit this whole feature exists for.
        if board["needs_first_master_engineer"] and cell["rank"] == "pawn":
            homeward *= cell["cargoCount"]
        score += add_term(terms, "prisoner", homeward, "escort")

    # Blocker clean-up: our own king-carrying convoy cannot land anywhere
    # because every base square for its rank is held by one of OUR OWN
    # pieces. This is not the convoy's own move — `cell` is whichever piece
    # happens to sit on one of those squares — so moving it ANYWHERE is
    # rewarded, independent of `destination`.
    if not cell["convoy"] and params["delivery"] > 0 and cell["square"] in board["blocking_base"]:
        score += add_term(terms, "delivery",
                          (UNBLOCK_BASE_VALUE - min(rank_value(cell["rank"]), QUEEN_VALUE) * UNBLOCK_VALUE_SPREAD) * params["delivery"],
                          "unblock")

    # Positional pressure. A deterministic relocation receives one post-state
    # fact; destinationVisible remains a separate fog gate because fact
    # presence alone does not make a square visible.
    if params["advance"] > 0 and not cell["convoy"]:
        gain = forward_progress(board["side"], destination) - forward_progress(board["side"], cell["square"])
        positional_known_and_supported = (not quiet_move or
                                          (candidate_known and candidate.get("destinationVisible", False) and guarded_count(candidate) > 0))
        if quiet_move and (not candidate_known or not candidate.get("destinationVisible", False)):
            score += add_term(terms, "safety", -UNKNOWN_QUIET_PENALTY * params["safety"], "unknownQuiet")
        elif quiet_move and guarded_count(candidate) <= 0:
            score += add_term(terms, "safety", -UNSUPPORTED_QUIET_PENALTY * params["safety"], "unsupportedQuiet")
        ordinary_piece = cell["rank"] != "king" and not is_current_beacon_bearer(observation, cell)
        if cell["rank"] == "pawn" and positional_known_and_supported and ordinary_piece:
            score += add_term(terms, "advance", gain * ADVANCE_PAWN_MULTIPLIER * params["advance"], "pawn")
            if is_promotion_square(board["side"], destination):
                score += add_term(terms, "advance", PROMOTION_BONUS * params["advance"], "promotion")
            elif (quiet_move and candidate_known and candidate.get("destinationVisible", False) and
                    supported_and_unthreatened(candidate) and can_promote_next_move(board["side"], candidate)):
                score += add_term(terms, "advance", PROMOTION_NEXT_BONUS * params["advance"] * protection_factor(candidate), "promotionNext")
        elif ordinary_piece and positional_known_and_supported:
            score += add_term(terms, "advance", gain * params["advance"], "piece")
        if (quiet_move and positional_known_and_supported and
                cell["rank"] in ["knight", "bishop", "rook", "queen"] and ordinary_piece and not cell.get("moved", False) and gain > 0):
            score += add_term(terms, "develop", DEVELOP_FIRST_FORWARD_VALUE * params["advance"] * protection_factor(candidate), "firstForward")
        if quiet_move and positional_known_and_supported and ordinary_piece and candidate.get("patrolGain", 0) > 0:
            score += add_term(terms, "coverage", min(candidate.get("patrolGain", 0), PATROL_GAIN_CAP) * PATROL_GAIN_VALUE * params["advance"] * protection_factor(candidate), "patrol")
        # The before/after `guards` graph yields generic support changes.
        # Outbound support is NET material:
        # guarding two pawns while exposing a queen is a loss, while two equal
        # guards minus one loss is positive. guardedBy rewards real backing
        # only when unsupportedQuiet did NOT fire; captures and immediate
        # promotions retain their senior tactical scoring without list noise.
        #
        # ordinary_piece-gated (sneat-co/chessraiders#84, DEFECT A): every
        # sibling positional term above (pawn/piece advance, develop,
        # coverage/patrol) already excludes the king and the current Beacon
        # bearer, because their OWN movement is priced by the dedicated
        # kingSafety/moralePush/beaconAggression terms instead. This
        # guards/guardedBy block was the one term in this whole section that
        # did NOT check ordinary_piece, so a king (or Beacon bearer) shuffling
        # between two already-mutually-guarded squares — a purely sideways
        # move with zero forward_progress gain, so moralePush never fires,
        # and no active threat, so kingSafety never fires either — farmed a
        # positive "coverage"/"safety" score every single time, forever,
        # because guard_changes recomputes a fresh newly-guarded/lost delta
        # each way. Measured: a Commander-vs-Commander standard-rules match
        # (bare CreateLobby, KillOrCapture) reaches a position where BOTH
        # kings' only positive-scoring move is exactly this shuffle
        # (g3<->f3 / f6<->g6 in the recorded case) — it outscores a real,
        # available capture (a knight for a bishop, net +1.050 after risk)
        # because the shuffle scores 1.288-1.550 from guards/guardedBy alone
        # — and the match runs out the full probed horizon (2000 ticks)
        # without a single further kill, capture, delivery or winner in 4 of
        # 5 seeds. No repetition guard catches it: the one-ply leader-
        # reversal guard and the three-square quiet-vacated ring both
        # deliberately exempt leader/Beacon-bearer moves (this file's own
        # leader_reverse_forbidden/apply_repeat_penalty), so nothing else in
        # the model ever recognises this as a repeat.
        if (quiet_move and candidate_known and not is_promotion_square(board["side"], destination) and
                candidate.get("destinationVisible", False) and threatened_count(candidate) == 0 and ordinary_piece):
            newly_guarded, sole_guard_lost = guard_changes(board, cell, candidate)
            net_outbound_material = (support_material(board, newly_guarded) -
                                     support_material(board, sole_guard_lost))
            if net_outbound_material > 0:
                score += add_term(terms, "coverage", net_outbound_material * GUARDS_VALUE * params["advance"], "guards")
            elif net_outbound_material < 0:
                score += add_term(terms, "safety", net_outbound_material * SOLE_GUARD_LOST_VALUE * params["safety"], "soleGuardLost")
            inbound_material = inbound_support_material(cell, candidate)
            if inbound_material > 0:
                score += add_term(terms, "safety", inbound_material * GUARDED_BY_VALUE * params["safety"], "guardedBy")
    # Exact host-proven king pressure is a named +30, not an approximation
    # scaled by a tier's generic advance preference. Admission priority gets
    # the same number independently; this term selects the matching
    # destination after breadth admits the actor.
    if (not cell["convoy"] and quiet_move and candidate_known and
            candidate.get("destinationVisible", False) and supported_and_unthreatened(candidate) and
            cell["rank"] != "king" and not is_current_beacon_bearer(observation, cell)):
        for enemy in board["enemy"]:
            if enemy["rank"] == "king" and not enemy["ghost"] and enemy["square"] in (candidate.get("nextPossibleMoves", []) or []):
                score += add_term(terms, "kingHunt", KING_VISIBLE_ATTACK_BONUS, "visible")
                break

    # Dodge a target lock: the enemy has publicly committed to striking
    # this piece, so moving it is worth real value.
    if params["targetLock"] > 0 and board["locked_units"].get(cell["unitId"]):
        score += add_term(terms, "targetLock", TARGET_LOCK_DODGE_VALUE * params["targetLock"], "dodge")
        if post_safety_known and post_threat == 0:
            score += add_term(terms, "targetLock", TARGET_LOCK_SAFE_VALUE * params["targetLock"], "safeDodge")

    # King safety: get the king out of danger, or take the piece
    # threatening it.
    if params["kingSafety"] > 0 and board["king_threatened"]:
        if cell["rank"] == "king" and not cell["convoy"] and post_safety_known and post_threat == 0:
            score += add_term(terms, "kingSafety", params["kingSafety"], "escape")
        if (target_cell and board["king_cell"] and
                board["king_cell"]["square"] in relation_squares(target_cell, "threatens")):
            score += add_term(terms, "kingSafety", params["kingSafety"] * KING_GUARD_BONUS, "guard")

    # Morale is the king's own rank: pushing the king forward strengthens
    # every command system, and only a tier that values it plays that way.
    if params["moralePush"] > 0 and cell["rank"] == "king" and not cell["convoy"]:
        gain = forward_progress(board["side"], destination) - forward_progress(board["side"], cell["square"])
        king_safe_after = ((candidate.get("destinationVisible", False) and supported_and_unthreatened(candidate)) if candidate_known else
                           (target_cell != None and post_threat == 0))
        if gain > 0 and king_safe_after:
            if candidate_known:
                after_morale = post_move_morale(observation, board, cell["square"], destination)
                excess = after_morale - current_morale_need(observation)
                guard_strength = leader_support(observation, board, cell, destination)
                if excess >= LEADER_EXCESS_MORALE:
                    score += add_term(terms, "moralePush", -gain * MORALE_PUSH_VALUE * params["moralePush"], "excessAdvance")
                else:
                    score += add_term(terms, "moralePush", gain * MORALE_PUSH_VALUE * params["moralePush"] * guard_strength, "guardedAdvance")
        elif candidate_known and gain < 0 and king_safe_after:
            after_morale = post_move_morale(observation, board, cell["square"], destination)
            needed = current_morale_need(observation)
            if (observation.get("ownMorale", 0) - needed >= LEADER_EXCESS_MORALE and
                    after_morale >= needed + 1):
                score += add_term(terms, "moralePush", -gain * MORALE_PUSH_VALUE * params["moralePush"] * LEADER_RETREAT_VALUE, "excessRetreat")
        if post_threat > 0:
            # never walk the king into a strike — a SAFETY cost, priced by the
            # safety weight, so it reads to a player as the danger it is
            score += add_term(terms, "safety", -KING_VALUE * params["safety"], "kingIntoStrike")

    # A Beacon bearer motivates the formation but is not a hero either. The
    # field is a permission as well as a weight on current observations.
    if (is_current_beacon_bearer(observation, cell) and cell["rank"] != "king" and
            not cell["convoy"] and quiet_move and params["beaconAggression"] > 0):
        gain = forward_progress(board["side"], destination) - forward_progress(board["side"], cell["square"])
        if candidate_known and candidate.get("destinationVisible", False) and supported_and_unthreatened(candidate):
            support_gain = leader_support(observation, board, cell, destination) - leader_support(observation, board, cell, cell["square"])
            if support_gain > 0:
                detail = "guardedAdvance" if gain > 0 else "regroup"
                score += add_term(terms, "beaconAggression", support_gain * params["beaconAggression"] * protection_factor(candidate), detail)

    intent = {"kind": "move", "from": cell["square"], "to": destination}
    if cell["rank"] == "pawn" and not cell["convoy"] and is_promotion_square(board["side"], destination):
        intent["promotion"] = "queen"
    # capture_choice was already decided above, before scoring, precisely so
    # the material term could weigh THIS verb's own success odds — move_
    # proposals' own capture_choice_available already proved at least one of
    # Kill/Capture is allowed AND affordable before this candidate ever
    # reached scoring, and the preference order here (Capture when params
    # prefers it and morale affords it, else Kill) is that same "choose an
    # affordable outcome" instruction, computed once rather than twice.
    if capture_choice:
        intent["choice"] = capture_choice

    # `actor` is what REQ:options-are-distinct-by-unit excludes on — the UNIT,
    # never the move. Walking the sorted list and taking the first N entries
    # routinely yields three destinations of one knight: three versions of one
    # idea presented as three options, which teaches nothing. Excluding the
    # chosen MOVE fixes almost nothing for the same reason. A system action
    # (system_proposals' lane) has no unit and carries its own action identity
    # here instead.
    return {
        "intent": intent, "score": score, "terms": terms,
        "actor": cell["unitId"],
        "key": "move|" + str(cell["unitId"]) + "|" + destination,
    }


def rank_options(proposals, params, count):
    """Turns the ALREADY-SORTED proposal list into up to `count` ranked,
    explained options — ONE walk down a list that already exists, never a
    re-scoring pass with the previous winner disabled (REQ:options-are-
    distinct-by-unit). Re-scoring N times computes the same answer N times and
    charges the decision budget for each pass; the founder's "find best,
    disable it, find next best" is implemented as its INTENT, not its
    mechanism.

    Three properties this function owns:
      * DISTINCT BY ACTOR — the first candidate for a unit wins and every
        remaining candidate for that same unit is skipped.
      * HONEST FLOOR — `passBelow` is the adviser's own row value and nothing
        under it is ever offered. `proposals` is sorted descending, so the
        first candidate below the floor ends the walk.
      * TIES SHARE A RANK — two candidates inside the script's own tie band
        carry the SAME rank, because ordering them would be asserting an
        ordering the score does not support: the only thing separating them is
        the alphabetical `key` tie-break, and presenting that as a judgement
        about the game would be a lie dressed as advice. The host random draw
        that picks a BOT's move among ties is deliberately not used here — a
        bot has to pick one, an adviser does not."""
    if count <= 0:
        return []
    chosen = []
    seen_actors = {}
    for proposal in proposals:
        if proposal["score"] < params["passBelow"]:
            break
        # `.get`, not `[...]`: system_proposals' lane is an empty generator
        # today, and a future Veteran/Commander proposal added there without
        # an `actor` would otherwise fail the whole decision rather than
        # degrade. Falling back to the proposal's own `key` gives a system
        # action its own action identity as its exclusion key, which is
        # exactly what REQ:options-are-distinct-by-unit asks for in that case.
        actor = proposal.get("actor", proposal["key"])
        if seen_actors.get(actor):
            continue
        seen_actors[actor] = True
        chosen.append(proposal)
        if len(chosen) >= count:
            break

    options = []
    rank = 0
    leader_score = 0.0
    for index in range(len(chosen)):
        proposal = chosen[index]
        if index == 0 or proposal["score"] < leader_score - TIE_BREAK_BAND:
            rank = index + 1
            leader_score = proposal["score"]
        options.append({
            "rank": rank,
            "intent": proposal["intent"],
            "score": proposal["score"],
            "terms": proposal["terms"],
        })
    return options


# =============================================================================
# Turning scored moves into a ranked list of proposals, and the (currently
# empty) system-action lane they compete against.
# =============================================================================

def _unit_priority_key(entry):
    return (-entry["priority"], square_index(entry["cell"]["square"]))

def _proposal_score_key(proposal):
    return (-proposal["score"], proposal["key"])

def apply_repeat_penalty(observation, board, params, memory, proposals):
    """Only penalise a second quiet ordinary move when a different ordinary
    proposal has already survived legality, affordability and breadth/spread
    filtering. Raw legal squares are not enough: a morale-refused capture is
    not a real alternative. lastQuietTo is written only for an ordinary quiet
    move, so actions, captures, threatened escapes, delivery and leaders
    remain exempt; pawns still avoid needless repetition."""
    # A quiet move that returns to a recently-vacated square closes a real
    # board-layout cycle even if a *different* promoted queen now occupies
    # the source. Keep only three squares: it catches the observed multi-queen
    # loop and the rook backtrack while preserving Recruit's 32-entry ceiling.
    recently_vacated = {}
    for slot in range(QUIET_VACATED_SQUARES):
        square = memory.get("quietVacated" + str(slot), NO_SQUARE_INDEX)
        if square >= 0:
            recently_vacated[square] = True
    cycle_keys = {}
    for proposal in proposals:
        cell = board["own_by_square"].get(proposal["intent"]["from"])
        destination = proposal["intent"].get("to")
        if (proposal["intent"]["kind"] == "move" and not proposal["intent"].get("promotion") and cell and destination and not cell["convoy"] and cell["rank"] != "king" and
                not is_current_beacon_bearer(observation, cell) and
                is_quiet_move(observation, board, cell, destination) and
                threatened_count(cell) == 0 and
                square_index(destination) in recently_vacated):
            cycle_keys[proposal["key"]] = True
    if cycle_keys:
        other_viable = False
        for proposal in proposals:
            candidate = board["own_by_square"].get(proposal["intent"]["from"])
            if (proposal["key"] not in cycle_keys and candidate and not candidate["convoy"] and candidate["rank"] != "king" and
                    not is_current_beacon_bearer(observation, candidate) and proposal["score"] >= params["passBelow"]):
                other_viable = True
                break
        if other_viable:
            proposals = [proposal for proposal in proposals if proposal["key"] not in cycle_keys]
    last_to = memory.get("lastQuietTo", NO_SQUARE_INDEX)
    if last_to < 0:
        return proposals
    repeated_cell = board["own_by_square"].get(square_name(last_to))
    if not repeated_cell or repeated_cell["convoy"] or repeated_cell["rank"] == "king" or is_current_beacon_bearer(observation, repeated_cell):
        return proposals
    other_viable = False
    for proposal in proposals:
        candidate = board["own_by_square"].get(proposal["intent"]["from"])
        if (candidate and candidate["unitId"] != repeated_cell["unitId"] and not candidate["convoy"] and candidate["rank"] != "king" and
                not is_current_beacon_bearer(observation, candidate) and proposal["score"] >= params["passBelow"]):
            other_viable = True
            break
    if not other_viable:
        return proposals
    for proposal in proposals:
        if proposal["actor"] != repeated_cell["unitId"]:
            continue
        destination = proposal["intent"].get("to")
        if is_quiet_move(observation, board, repeated_cell, destination) and threatened_count(repeated_cell) == 0:
            penalty = -params["advance"]
            proposal["score"] += add_term(proposal["terms"], "repeatPenalty", penalty, "quiet")
    return proposals

def move_proposals(observation, board, params, memory):
    """Expands the tier's breadth of own units into scored moves. Legality
    always comes from observation.legal, computed by the host — never from
    this function's own judgement, which only ever picks among what is
    already there."""
    if not board["move_actionable_units"]:
        return []

    ranked_units = []
    for cell in board["move_actionable_units"]:
        ranked_units.append({"cell": cell, "priority": unit_priority(observation, board, params, cell)})
    ranked_units = sorted(ranked_units, key=_unit_priority_key)

    breadth = params["breadth"]
    if breadth <= 0 or breadth > len(ranked_units):
        breadth = len(ranked_units)

    proposals = []
    leader_guard = active_leader_guard(observation, memory)
    for entry in ranked_units[:breadth]:
        cell = entry["cell"]
        destinations = observation["legal"].get(cell["square"], [])
        candidates = []
        for destination in destinations:
            if destination == cell["square"]:
                continue
            if cell.get("charging") and destination == cell["charging"]["square"]:
                continue  # retaining the current route is represented by pass
            if not capture_choice_available(observation, board, cell, destination):
                continue
            if leader_reverse_forbidden(observation, leader_guard, cell, destination):
                continue
            candidates.append(score_move(observation, board, params, memory, cell, destination))
        # A shallow tier keeps only its narrow spread of that unit's best
        # destinations — that, plus breadth, is what "shallow evaluation"
        # means.
        candidates = sorted(candidates, key=_proposal_score_key)
        candidate_spread = params["candidateSpread"]
        if candidate_spread <= 0 or candidate_spread > len(candidates):
            candidate_spread = len(candidates)
        proposals.extend(candidates[:candidate_spread])
    return apply_repeat_penalty(observation, board, params, memory, proposals)


# =============================================================================
# The founder's own hard rule (bot4chess/score.go's own
# priorityCaptiveDelivery, spec: retire-the-go-tiers#A4(a)) — verbatim,
# 2026-08-07: "It should always prioritize unload over anything else." This
# is HARD CONTROL FLOW, not a scored preference: decide() calls this BEFORE
# move_proposals/system_proposals ever run and returns immediately the
# moment it finds a move, so nothing this file's own scoring could ever
# compute is even offered the chance to outscore it. See
# priority_captive_delivery_proposal's own doc comment for the full
# mechanism and decide()'s own call site for the gate.
# =============================================================================

def priority_captive_delivery_proposal(observation, board, params):
    """A loaded, ordinary (non-king) Pawn-rank convoy takes the safe move
    that gets its captive home RIGHT NOW, ahead of anything else this
    decision could do, while this side still needs its first Master
    Engineer (board["needs_first_master_engineer"]) — mirrors bot4chess/
    score.go's own priorityCaptiveDelivery exactly. A score, however large,
    still competes with move_proposals/system_proposals and can lose to a
    bigger number somewhere else on the board; this cannot, because
    decide() never even generates those before checking this first.

    Two cases, mirroring score.go's own convoy-scoring split:
      * the convoy already stands on its own delivery rank: take any safe
        QUIET destination — chess/apply.go's applyConvoyMove unloads a
        captive on DEPARTURE from the pawn base rank, never on arrival, so
        this is the one move that actually delivers the captive. Prefer a
        destination that ALSO lands on the base rank (keeps the freshly-
        unloaded escort immediately eligible for its next training step —
        a future trainingProposals port only ever looks at a Pawn standing
        on ITS OWN base rank), falling back to any other safe quiet
        departure only once every same-rank option is unsafe.
      * the convoy is still en route: take the safe quiet move that most
        reduces its remaining distance to a base square for its (possibly
        CargoBasedDelivery-remapped) prisoner rank.

    "Safe" is is_safe_square — the identical guarded/threat gate every
    other term in this file uses, no new machinery. Either case falls
    through to None (ordinary scoring) the moment no safe candidate
    exists, so a convoy boxed in with nothing safe to do is never forced
    into danger.

    Deliberately excludes a king-cargo convoy (cell["kingCargo"]) and is
    gated by decide()'s own caller on `not board["convoy_home"]`: the
    captured-enemy-king delivery is a SEPARATE, senior mechanic (score_
    move's own `cell["convoy"] and cell["kingCargo"]` branch, the +500
    terminal DELIVERY_BONUS) that must stay supreme over this — an
    ordinary prisoner's delivery must never pre-empt the move that could
    win the whole match outright.

    Returns a FULL proposal dict (intent/score/terms/actor/key), not a bare
    intent, so decide() can run it through rank_options exactly like any
    other candidate — the Adviser, which rides this same decide() (adviser-
    tier#REQ:one-scoring-path-two-outputs), needs something to explain
    instead of an empty option list even though a bot decision never lets
    this proposal compete with anything else. Returns None when no
    qualifying, safe convoy exists at all, so decide() falls through to
    move_proposals/system_proposals unchanged."""
    if not board["needs_first_master_engineer"]:
        return None
    for cell in board["own"]:
        if (not cell["convoy"] or cell["cargoCount"] == 0 or cell["kingCargo"] or
                cell["rank"] != "pawn" or board["busy_units"].get(cell["unitId"])):
            continue
        prisoner_rank = "pawn" if observation["rules"]["cargoBasedDelivery"] else cell["rank"]
        destinations = observation["legal"].get(cell["square"], [])
        if not destinations:
            continue
        base_squares = observation["rules"]["baseSquares"].get(prisoner_rank, [])
        to = None

        if cell["square"] in base_squares:
            fallback = None
            for candidate in destinations:
                fact = candidate_at(observation, cell["square"], candidate)
                if (candidate == cell["square"] or board["enemy_by_square"].get(candidate) or
                        not has_candidate(observation, cell["square"], candidate) or
                        not fact.get("destinationVisible", False) or not is_safe_subject(fact)):
                    continue  # an attack (not the quiet departure that unloads), or unsafe
                if candidate in base_squares:
                    to = candidate
                    break
                if fallback == None:
                    fallback = candidate
            if to == None:
                to = fallback
        else:
            here = distance_to_nearest_square(cell["square"], base_squares)
            best_gain = 0
            for candidate in destinations:
                fact = candidate_at(observation, cell["square"], candidate)
                if (candidate == cell["square"] or board["enemy_by_square"].get(candidate) or
                        not has_candidate(observation, cell["square"], candidate) or
                        not fact.get("destinationVisible", False) or not is_safe_subject(fact)):
                    continue
                gain = here - distance_to_nearest_square(candidate, base_squares)
                if gain > best_gain:
                    best_gain, to = gain, candidate

        if to == None:
            continue
        return {
            "intent": {"kind": "move", "from": cell["square"], "to": to},
            "score": PRIORITY_CAPTIVE_DELIVERY_SCORE,
            "terms": [{"term": "prisoner", "value": PRIORITY_CAPTIVE_DELIVERY_SCORE, "detail": "priorityDelivery"}],
            "actor": cell["unitId"],
            "key": "priority-captive-delivery|" + str(cell["unitId"]) + "|" + to,
        }
    return None


def priority_captive_delivery_in_flight(observation, board):
    """Whether the one active route is the first Master Engineer's own
    pawn-convoy delivery. Such a route is a hard commitment, not a candidate
    to re-score at the Commander's next (one-second) cadence: StandardRules
    takes two seconds to settle a convoy step, so letting another equally
    homeward square replace it restarts the charge forever.

    This is intentionally narrower than retaining every route. It excludes
    king cargo, non-Pawn escorts, empty convoys, and routes that do not make
    homeward delivery progress. A threatened retreat, an ordinary tactical
    route, or a captured-king delivery therefore keeps the existing
    replacement behaviour. Once the route settles, there are no charging
    units and priority_captive_delivery_proposal above resumes its normal
    safe, one-step-at-a-time control flow.

    A convoy already on its delivery rank is also retained: its pending quiet
    departure is the only command that unloads the captive (apply.go unloads
    on departure), even though its destination need not reduce distance."""
    if not board["needs_first_master_engineer"]:
        return False
    for cell in board["own"]:
        charging = cell.get("charging")
        if (not charging or not cell["convoy"] or cell["cargoCount"] == 0 or
                cell["kingCargo"] or cell["rank"] != "pawn"):
            continue
        # The priority proposal below only chooses a QUIET departure. An
        # occupied route target is an attack, which must retain ordinary
        # route-replacement freedom even if it happens to point homeward.
        if board["all_by_square"].get(charging["square"]):
            continue
        prisoner_rank = "pawn" if observation["rules"]["cargoBasedDelivery"] else cell["rank"]
        base_squares = observation["rules"]["baseSquares"].get(prisoner_rank, [])
        if cell["square"] in base_squares:
            return True
        if distance_to_nearest_square(charging["square"], base_squares) < distance_to_nearest_square(cell["square"], base_squares):
            return True
    return False


def most_advanced_adjacent_ally(board):
    """Picks the own, non-king, non-convoy, not-busy unit standing exactly
    one king-step from the king (Chebyshev distance 1) with the greatest
    forward_progress — mirrors bot4chess/systems.go's
    mostAdvancedAdjacentAlly exactly, over this script's own actionable-unit
    set (already excludes convoy/refitting/charging/training/recovering/
    routed/interrogated pieces — build_board's own busy_units) rather than a
    *board Go value. BeaconInfluence radiates OUTWARD from the bearer's own
    square (chess/beacon.go), so the most advanced candidate puts that
    radius over the most board, not merely over whichever ally happens to
    be listed first. Ties keep the FIRST candidate found — board["own"]'s
    own board-index order (piece_squares), so this is
    deterministic across the native-Go and TinyGo-wasm runtimes for the
    identical observation. Returns None when the king has no eligible
    neighbour at all."""
    king_cell = board["king_cell"]
    if king_cell == None:
        return None
    best_square = None
    best_progress = -1
    for cell in board["actionable_units"]:
        if cell["rank"] == "king" or cell["convoy"]:
            continue
        if chebyshev_distance(king_cell["square"], cell["square"]) != 1:
            continue
        progress = forward_progress(board["side"], cell["square"])
        if progress > best_progress:
            best_progress = progress
            best_square = cell["square"]
    return best_square

def beacon_hand_off_proposal(observation, board, params):
    """Hands the Beacon from a never-handed-off king-bearer to the most
    advanced adjacent ally — mirrors bot4chess/systems.go's beaconProposals'
    "deployed" case (Deploy/Restore are the sibling "undeployed"/"lost"
    cases, ported alongside this one in beacon_deploy_or_restore_proposal
    below; see system_proposals' own comment for why training/walls/
    dismantle are not ported here).

    Not merely cosmetic: the founder's own instruction is "the bot should
    definitely prioritize passing the beacon from king to forward pieces."
    Under BeaconRules.KingStartsAsBearer, a king holding a Beacon that has
    NEVER left him is ROOTED (immobile) by design — a human discovers this
    the moment their king's move is rejected, and then hands the Beacon off.
    A bot that never proposes the hand-off has no such moment: its king
    simply never moves again for the rest of the match, and since team
    morale is derived from the king's own board rank, morale never leaves 0
    — and since morale gates every capture, the bot could never take a
    single prisoner, all game, silently. Freeing the king is therefore a
    PREREQUISITE for the Beacon (and captures) being worth anything at all,
    which is why SYSTEM_BEACON_HAND_OFF_VALUE outranks
    SYSTEM_BEACON_DEPLOY_VALUE/SYSTEM_BEACON_RESTORE_VALUE.

    Gated on Rules.BeaconKingStartsAsBearer EXPLICITLY, never merely on
    Beacon.Lifecycle == "deployed" — a bot-deployed Beacon under LEGACY
    rules (the flag off) also reaches "deployed", where rooting is
    unconditional and always was; this hand-off exists only for the ruling
    that made a king-bearer's rooting conditional in the first place. A
    disabled Beacon (Rules.Beacon.Enabled false) is handled for free:
    observation["beacon"]["lifecycle"] is then always "" (BeaconFact's own
    doc comment, observation.go), which the lifecycle check below already
    rejects. Stops proposing itself the instant everHandedOff is true — a
    freed king does not need to keep giving the Beacon away.

    Carries a "terms" breakdown (one "system" entry, detail "handOff") for
    the identical reason priority_captive_delivery_proposal's own doc
    comment gives: this is a FULL proposal dict, not a bare intent, and
    rank_options reads `proposal["terms"]` by direct indexing (never
    `.get`) for every candidate that reaches it — a system-action proposal
    missing the key would KeyError the whole decision the moment an
    Adviser call (options > 0, DecideOptionsFromState) ranked it into the
    returned slice, silently working today only because every existing
    caller of THIS function decides with options = 0.

    Also carries "actor" — the RECEIVING ally's own unitId, resolved from
    `to` below, never the king's — because the intent addresses itself at
    the ally's own square (validateAndConvert's own ownUnitAt resolves the
    unit the identical way), so that is the physical unit
    REQ:options-are-distinct-by-unit must dedupe THIS proposal against: an
    ordinary move of that same ally proposed elsewhere in the same decide()
    call is the other candidate for the SAME idea, not a different one."""
    if not observation["rules"].get("beaconEnabled", False):
        return None
    king_cell = board["king_cell"]
    if king_cell == None or board["busy_units"].get(king_cell["unitId"]):
        return None
    if not observation["rules"]["beaconKingStartsAsBearer"]:
        return None
    beacon = observation["beacon"]
    if beacon["lifecycle"] != "deployed" or beacon["everHandedOff"]:
        return None
    if beacon["bearerSquare"] != king_cell["square"]:
        return None
    to = most_advanced_adjacent_ally(board)
    if to == None:
        return None
    actor = None
    for cell in board["actionable_units"]:
        if cell["square"] == to:
            actor = cell["unitId"]
            break
    score = SYSTEM_BEACON_HAND_OFF_VALUE * beacon_aggression(observation, params)
    return {
        "intent": {"kind": "action", "from": to, "to": to, "action": "beacon_take"},
        "score": score,
        "terms": [{"term": "beaconAggression", "value": score, "detail": "handOff"}],
        "actor": actor,
        "key": "beacon-hand-off",
    }

def training_proposals(observation, board, params):
    """Trains an idle, safe pawn standing on its own base rank into a
    specialist, and pushes an eligible Engineer on to Master Engineer
    Training — mirrors bot4chess/systems.go's own trainingProposals exactly,
    over the observation's own host-computed facts (rules.specialistCeiling,
    ownSpecialists, rules.permittedProfessions, rules.baseSquares,
    rules.veteranProgression, and per-cell profession/veteran/grade/
    eligibleFor fields — bot-script-contract#
    AC:a-system-generator-can-be-written-from-the-observation-alone) rather
    than a *board Go value.

    params["advancedTraining"] REPLACES bot4chess's own
    isCommanderTier(t.profile.difficulty) branch (spec: retire-the-go-tiers#
    REQ:no-decision-branches-on-a-tier-name) — a Lieutenant row leaves it
    false, a Commander/Custom row sets it true; the behaviour this gate
    reaches is otherwise identical to Go's `isCommander &&` guard
    (TestCommanderAdvancedTrainsLieutenantDoesNot,
    server-go/tests4bot/tiers_test.go).

    have_engineer scans EVERY own cell in `pieces`, never only
    board["actionable_units"]: a busy or convoying Engineer still counts as
    "we already have one" for the Engineer-first ordering below, exactly as
    bot4chess's own loop over the unfiltered b.own does. The per-candidate
    loop below then excludes Convoy/Refitting by hand rather than folding
    them into board["busy_units"]: Go's own b.busy map never contains
    Convoy/Refitting (bot4chess/observe.go's own readBoard), so mirroring it
    here means checking them separately too, exactly like Go's own
    `c.Convoy || c.Refitting || b.busy[c.UnitID]`.

    The safe/idle/base-rank/ceiling/Veteran-badge gates below are the same
    "founder bug" fix bot4chess/systems.go's own doc comment describes: a
    pawn training rejected under VeteranProgression un-suppresses itself the
    very next cycle unless training is never PROPOSED for it at all, so this
    reads cell["veteran"] before ever proposing ActionTrain,
    not after the engine has already refused it once
    (TestCommanderTrainsAnEngineerButNeverBuildsAWall,
    server-go/tests4bot/tiers_test.go)."""
    have_engineer = False
    for cell in board["own"]:
        if cell.get("profession", "") == "engineer":
            have_engineer = True
            break

    ceiling = observation["rules"]["specialistCeiling"]
    permitted = observation["rules"]["permittedProfessions"]
    base_squares = observation["rules"]["baseSquares"].get("pawn", [])
    proposals = []
    for cell in board["own"]:
        if (cell["convoy"] or cell["refitting"] or cell["rank"] != "pawn" or
                board["busy_units"].get(cell["unitId"])):
            continue
        if cell["square"] not in base_squares:
            continue
        # A training channel locks the pawn in place until it settles, with
        # no way to dodge in between — never start one on a pawn the enemy
        # can already take next turn ("safe pawns", founder 2026-07-31).
        if threatened_count(cell) > 0:
            continue
        if cell.get("profession", "") == "":
            # Gate A (spec: veteran-progression#REQ:training-requires-
            # veteran): ActionTrain rejects any pawn that has not itself
            # delivered a prisoner it personally captured — c.Veteran is the
            # same public badge a human client renders.
            if observation["rules"]["veteranProgression"] and not cell.get("veteran", False):
                continue
            if observation["ownSpecialists"] >= ceiling:
                continue
            # An Engineer first (it unlocks fortifications), then Sergeants.
            profession = "sergeant"
            if not have_engineer and "engineer" in permitted:
                profession = "engineer"
            if profession not in permitted:
                continue
            score = SYSTEM_TRAIN_VALUE * params["system"]
            proposals.append({
                "intent": {
                    "kind": "action", "from": cell["square"], "to": cell["square"],
                    "action": "train", "profession": profession,
                },
                "score": score,
                "terms": [{"term": "system", "value": score, "detail": "train"}],
                "actor": cell["unitId"],
                "key": "train|" + str(cell["unitId"]) + "|" + profession,
            })
            continue
        if (params["advancedTraining"] and cell.get("profession", "") == "engineer" and
                "masterEngineerTraining" in (cell.get("eligibleFor", []) or []) and cell.get("grade") != "master"):
            score = SYSTEM_ADVANCED_TRAIN_VALUE * params["system"]
            proposals.append({
                "intent": {
                    "kind": "action", "from": cell["square"], "to": cell["square"],
                    "action": "advanced_train",
                },
                "score": score,
                "terms": [{"term": "system", "value": score, "detail": "advancedTrain"}],
                "actor": cell["unitId"],
                "key": "advanced|" + str(cell["unitId"]),
            })
    return proposals

# ---- system actions: wall REPAIR/DISMANTLE ----------------------------------
# VALUE_REPAIR_WALL/VALUE_DISMANTLE_WALL mirror bot4chess/systems.go's own
# valueRepairWall (1.5) and valueDismantleWall (1.0) exactly. The Sergeant-
# support bonus is deliberately NOT a constant here the way those two are:
# Go's own valueSergeantSupported (0.5) was gated at its two call sites by
# isCommanderTier, which params.go's own SergeantPreference doc comment
# replaces with a per-row NUMBER (0.5 for Commander/Custom/Adviser, 0 for
# Recruit/Lieutenant) rather than a second doctrine bool — see wall_
# proposals' own doc comment for the coupling that number carries.
VALUE_REPAIR_WALL = 1.5
VALUE_DISMANTLE_WALL = 1.0

def sergeant_supported(board, square):
    """Whether an own, active, non-convoy Sergeant stands exactly one
    king-step from `square` — mirrors bot4chess/systems.go's
    sergeantSupported exactly, over the projected own pieces rather than a *board
    Go value (Profession is public physical identity, projected to both
    sides like a uniform, per that function's own doc comment, so this is a
    plain read rather than a privileged one). Chebyshev distance 1 is
    precisely the 8 king-step neighbours most_advanced_adjacent_ally already
    uses the same way — no new geometry. Deliberately does not check
    charging/training/recovering: an adjacent Sergeant who is itself
    mid-channel still speeds up this work, exactly like Go's own check,
    which excludes only Convoy and Refitting."""
    for cell in board["own"]:
        if (cell.get("profession", "") == "sergeant" and not cell["convoy"] and not cell["refitting"] and
                chebyshev_distance(square, cell["square"]) == 1):
            return True
    return False

def wall_proposals(observation, board, params):
    """Repairs an own wall below 60% of its maximum integrity, and dismantles
    a blocking enemy wall when this tier's doctrine contests enemy work —
    mirrors bot4chess/systems.go's wallProposals exactly, minus the
    isCommanderTier branch (REQ:no-decision-branches-on-a-tier-name) that
    function gated DISMANTLE (and its own Sergeant bonus) behind: this port
    reads params["contestEnemyWork"] instead, a value rather than a tier-name
    comparison. No BUILD case — the founder removed it outright (2026-08-08:
    "I don't think bot should be building walls at all ... too sophisticated
    ... hard to specify when it should be actioned") and this port does not
    reintroduce it; REPAIR and DISMANTLE are the only two wall actions this
    function ever proposes.

    THE COUPLING THIS PRESERVES (params.go's own SergeantPreference doc
    comment): Go's DISMANTLE case applies valueSergeantSupported with NO gate
    of its own ("if sergeantSupported(...) { score += ... }"), safe only
    because that whole case is already reachable exclusively when isCommander
    is true. This mirrors that shape exactly — the bonus below is added
    inside the dismantle branch UNCONDITIONALLY once the branch itself has
    already fired on params["contestEnemyWork"], never re-gated a second
    time. It stays correct only as long as params["sergeantPreference"] is
    zero on every row where params["contestEnemyWork"] is false (params.go's
    own invariant, cross-checked by
    server-go/tests4bot/doctrine_params_test.go); a row that broke that
    invariant would
    make an enemy-wall dismantle carry a Sergeant bonus a Lieutenant-doctrine
    row was never meant to score at all.

    Skips a wall already worked by one of this side's own sessions
    (wall["ownWorkSession"], host-computed exactly like Go's own
    workedByOwnSide) and any unit that is a convoy, refitting, or otherwise
    busy (board["actionable_units"] already excludes charging/training/
    forging/recovering/routed/interrogated units — build_board's own
    busy_units), matching wallProposals' own `c.Convoy || c.Refitting ||
    b.busy[c.UnitID]` guard exactly."""
    proposals = []
    for cell in board["actionable_units"]:
        if cell["convoy"] or cell["refitting"]:
            continue
        for wall in observation["walls"]:
            if wall["a"] == cell["square"]:
                direction = wall["directionFromA"]
            elif wall["b"] == cell["square"]:
                direction = wall["directionFromB"]
            else:
                continue  # this wall's edge does not touch this unit's square
            if wall["ownWorkSession"]:
                continue  # a session of ours is already running on this wall
            terms = []
            if (wall["side"] == board["side"] and cell.get("profession", "") == "engineer" and
                    wall["maximumIntegrity"] > 0 and wall["integrity"] * 10 < wall["maximumIntegrity"] * 6):
                score = add_term(terms, "system", VALUE_REPAIR_WALL * params["system"], "repairWall")
                if sergeant_supported(board, cell["square"]):
                    score += add_term(terms, "system", params["sergeantPreference"] * params["system"], "sergeantSupported")
                proposals.append({
                    "intent": {"kind": "action", "from": cell["square"], "to": cell["square"],
                               "action": "repair_wall", "direction": direction},
                    "score": score, "terms": terms, "actor": cell["unitId"],
                    "key": "repair|" + str(cell["unitId"]) + "|" + wall["edge"],
                })
            elif wall["side"] != board["side"] and params["contestEnemyWork"]:
                # Contesting enemy work is offensive play a Lieutenant leaves
                # alone (params["contestEnemyWork"] is false on that row) —
                # only a Commander-doctrine row reaches for it.
                score = add_term(terms, "system", VALUE_DISMANTLE_WALL * params["system"], "dismantleWall")
                if sergeant_supported(board, cell["square"]):
                    score += add_term(terms, "system", params["sergeantPreference"] * params["system"], "sergeantSupported")
                proposals.append({
                    "intent": {"kind": "action", "from": cell["square"], "to": cell["square"],
                               "action": "dismantle_wall", "direction": direction},
                    "score": score, "terms": terms, "actor": cell["unitId"],
                    "key": "dismantle|" + str(cell["unitId"]) + "|" + wall["edge"],
                })
    return proposals

def beacon_deploy_or_restore_proposal(observation, board, params):
    """Deploys an undeployed Beacon, or restores one that is Lost — mirrors
    bot4chess/systems.go's beaconProposals' own "undeployed"/"lost" switch
    cases exactly (the "deployed" case is beacon_hand_off_proposal above).
    Gated on the SAME king-nil/busy check beaconProposals itself opens with,
    in the same position relative to everything else: a busy king proposes
    neither action.

    observation["beacon"] is already narrowed to the OBSERVING SIDE's own
    Beacon (BeaconFact's own doc comment in observation.go, mirroring
    bot4chess's own `beacon.Side != b.side` filter), so there is nothing
    left to filter by side here. A disabled Beacon (Rules.Beacon.Enabled
    false) is handled for free, exactly like beacon_hand_off_proposal's own
    doc comment describes: lifecycle is then always "", matching neither
    branch below.

    lifecycle can also read "restoring" — chessview's own beaconLifecycle
    reports that, distinct from "lost", for the whole duration of an
    in-flight Restore channel (chess.Beacon.RestoreDueAt > 0) — and this
    function's exact-string checks fall through it exactly like Go's switch
    does, so a Restore already under way is never re-proposed (the engine
    would reject a second one: chess/beacon.go's own ActionBeaconRestore
    case rejects when RestoreDueAt is already set).

    Carries a "terms" breakdown on both branches (one "system" entry,
    detail "deploy"/"restore") — see beacon_hand_off_proposal's own doc
    comment for why this is required, not optional, on every proposal dict
    system_proposals' lane returns. Also carries "actor": king_cell's own
    unitId on both branches — the king is the unit this action addresses
    itself at (REQ:options-are-distinct-by-unit), so any ordinary king move
    proposed elsewhere in the same decide() call is the SAME idea's other
    candidate, not a different one."""
    if not observation["rules"].get("beaconEnabled", False):
        return None
    king_cell = board["king_cell"]
    if king_cell == None or board["busy_units"].get(king_cell["unitId"]):
        return None
    beacon = observation["beacon"]
    if beacon["lifecycle"] == "undeployed":
        score = SYSTEM_BEACON_DEPLOY_VALUE * beacon_aggression(observation, params)
        return {
            "intent": {"kind": "action", "from": king_cell["square"], "to": king_cell["square"], "action": "beacon_deploy"},
            "score": score,
            "terms": [{"term": "beaconAggression", "value": score, "detail": "deploy"}],
            "actor": king_cell["unitId"],
            "key": "beacon-deploy",
        }
    if beacon["lifecycle"] == "lost":
        score = SYSTEM_BEACON_RESTORE_VALUE * beacon_aggression(observation, params)
        return {
            "intent": {"kind": "action", "from": king_cell["square"], "to": king_cell["square"], "action": "beacon_restore"},
            "score": score,
            "terms": [{"term": "beaconAggression", "value": score, "detail": "restore"}],
            "actor": king_cell["unitId"],
            "key": "beacon-restore",
        }
    return None

def beacon_forge_proposals(observation, board, params):
    """Proposes Beacon Forging (BeaconRules.ForgeEnabled) for every own,
    idle, safe pawn standing on its own pawn base rank while the side's
    Beacon is Lost — mirrors bot4chess/systems.go's forgeProposals exactly,
    including its independence from the king-nil/busy gate the other two
    Beacon cases open with: an idle pawn may forge while the king is
    mid-charge, mid-Restore, or anywhere else, exactly as a human player
    could. This is the founder's own "not optional... the failure mode I
    care most about" half of the Beacon story: a Beacon that is Lost and
    never forged is lost PERMANENTLY for the rest of the match, with no
    other recovery besides the king's own (busy-gated, one-at-a-time)
    Restore.

    One proposal per eligible pawn — TWO pawns forging at once is legal
    (chess.applyBeaconForge places no side-wide lock), so nothing here
    artificially picks just one; rank_options' own truncation is what
    ultimately commands at most one action this tick.

    board["actionable_units"] already excludes a pawn mid-Forge itself —
    build_board's own busy_units reads cell["forgingRemainingMs"], so the
    fix a unit mid-Forge being free in Starlark remains enforced —
    so this never re-proposes an already-channelling pawn as its own
    replacement.

    Each proposal carries a "terms" breakdown (one "system" entry, detail
    "forge") — see beacon_hand_off_proposal's own doc comment for why this
    is required, not optional, on every proposal dict system_proposals'
    lane returns. Also carries "actor": the forging pawn's own unitId
    (REQ:options-are-distinct-by-unit) — already unique per proposal here
    since the loop below is keyed on cell["unitId"] already, but every
    other system generator in this file names it explicitly rather than
    leaving rank_options to fall back to `key`, so this does too."""
    if not observation["rules"].get("beaconEnabled", False) or not observation["rules"].get("beaconForgeEnabled", False):
        return []
    if observation["beacon"]["lifecycle"] != "lost":
        return []
    own_pawn_base_squares = observation["rules"]["baseSquares"].get("pawn", [])
    proposals = []
    for cell in board["actionable_units"]:
        if cell["convoy"] or cell["refitting"] or cell["rank"] != "pawn":
            continue
        if cell["square"] not in own_pawn_base_squares:
            continue
        # Never start a Forge on a pawn the enemy can already take next turn
        # — same "safe pawns" gate trainingProposals uses for the identical
        # reason (a channel locks progress in with no way to dodge).
        if threatened_count(cell) > 0:
            continue
        score = SYSTEM_BEACON_FORGE_VALUE * beacon_aggression(observation, params)
        proposals.append({
            "intent": {"kind": "action", "from": cell["square"], "to": cell["square"], "action": "beacon_forge"},
            "score": score,
            "terms": [{"term": "beaconAggression", "value": score, "detail": "forge"}],
            "actor": cell["unitId"],
            "key": "beacon-forge|" + str(cell["unitId"]),
        })
    return proposals

# ---- system actions: espionage ----------------------------------------------
# INTERROGATE_VALUE mirrors bot4chess/systems.go's own valueInterrogate (0.6)
# exactly — deliberately the cheapest system-action value in this file:
# insurance against an already-turned informer, never a reason to skip a real
# move or a costlier system action.
INTERROGATE_VALUE = 0.6

def espionage_proposals(observation, board, params):
    """Interrogates every own, adjacent, non-king, non-convoy, idle piece
    while the enemy holds something of ours — mirrors bot4chess/systems.go's
    espionageProposals over board["actionable_units"] filtered to Chebyshev
    distance 1 from the king (the same equivalence most_advanced_adjacent_
    ally's own doc comment already establishes for that function: the 8
    king-step squares Go loops over ARE the Chebyshev-1 set, so busy_units
    already excludes exactly what Go's own suspect.Convoy-adjacent b.busy
    check excludes).

    ONE DELIBERATE DIFFERENCE from Go's own trigger: Go reads only the
    enemy's own Captives count (obs.View.BlackCaptives/WhiteCaptives); this
    reads observation["enemyManaged"] (Captives+Informers) instead — the
    field Task 5 added specifically for this generator (its own doc comment,
    observation.go: "the only public signal that they may have turned one of
    OUR pieces into an informer"). A widened trigger, not an oversight: an
    enemy that already holds an INFORMER of ours with zero live Captives is
    exactly the case Go's own doc comment describes and Go's own narrower
    read could never catch.

    Every eligible neighbour becomes its OWN candidate — not the single best
    one, unlike most_advanced_adjacent_ally — because every candidate scores
    identically (INTERROGATE_VALUE * params["system"], no per-suspect term),
    so which one is finally CHOSEN among ties is left entirely to the shared
    key/seeded-draw machinery every other proposal already goes through.

    Carries "actor": king_cell's own unitId — the king performs the
    interrogation (the intent addresses itself at the king's own square);
    `suspect` is only the TARGET, a different unit. REQ:options-are-distinct-
    by-unit dedupes this against any ordinary king move proposed elsewhere in
    the same decide() call, the same reasoning beacon_deploy_or_restore_
    proposal's own doc comment gives."""
    king_cell = board["king_cell"]
    if king_cell == None or board["busy_units"].get(king_cell["unitId"]):
        return []
    if observation["enemyManaged"] <= 0:
        return []
    proposals = []
    for suspect in board["actionable_units"]:
        if suspect["rank"] == "king" or suspect["convoy"]:
            continue
        if chebyshev_distance(king_cell["square"], suspect["square"]) != 1:
            continue
        terms = []
        score = add_term(terms, "system", INTERROGATE_VALUE * params["system"], None)
        proposals.append({
            "intent": {
                "kind": "action", "from": king_cell["square"], "to": king_cell["square"],
                "action": "interrogate", "target": suspect["unitId"],
            },
            "score": score,
            "terms": terms,
            "actor": king_cell["unitId"],
            "key": "interrogate|" + str(suspect["unitId"]),
        })
    return proposals

def system_proposals(observation, board, params):
    """Training, wall-work (repair and dismantle), the full Beacon surface
    (deploy/restore/forge/hand-off) and espionage — the channelled/system
    actions the tier's rules gate allows, competing for the same choice as
    a move on the same scale.

    All five are now ported: training_proposals, wall_proposals (REPAIR and
    DISMANTLE — sergeant_supported's own bonus applies to both branches),
    beacon_hand_off_proposal, beacon_deploy_or_restore_proposal,
    beacon_forge_proposals, and espionage_proposals. This is no longer the
    "real gate wired to an empty generator" state the pre-port version of
    this comment described — every case the doctrine gates below can now
    actually reach has a real generator behind it.

    Training is gated exactly like bot4chess's own `systems.Training &&
    obs.View.SpecialistsEnabled` double gate (spec: retire-the-go-tiers Task
    8). Wall-work is gated on systems["walls"] and either
    rules["woodWallsEnabled"] or rules["stoneWallsEnabled"]; DISMANTLE
    within it additionally requires params["contestEnemyWork"] — a
    Lieutenant-doctrine row never reaches it, only Commander/Custom do (see
    wall_proposals' own doc comment for the Sergeant-bonus coupling that
    carries). None of the three Beacon cases is optional the way
    training/walls are: skipping the hand-off under
    BeaconRules.KingStartsAsBearer leaves a Lieutenant/Commander bot's king
    rooted for the rest of the match, and skipping Forge leaves a Lost
    Beacon lost PERMANENTLY once the king's own Restore is also unavailable
    (see those two functions' own doc comments for the full accounting).
    Current rows use beaconAggression itself as the Beacon permission: a
    positive value enables Beacon proposals subject to the match rules,
    independently of systems["beacon"]. Forge additionally needs
    rules["beaconForgeEnabled"].
    espionage_proposals is gated on systems["espionage"] alone (see its own
    doc comment for the enemyManaged trigger it reads instead of Go's
    narrower Captives-only read) — so Recruit's empty systems gate, and
    params["system"] == 0, leaves every one of them exactly as inert as any
    other system action."""
    systems = observation["systems"]
    beacon_allowed = observation["rules"].get("beaconEnabled", False) and beacon_aggression(observation, params) > 0
    any_other_system_enabled = systems["training"] or systems["walls"] or systems["prisoners"] or systems["morale"] or systems["espionage"]
    if (not beacon_allowed and (not any_other_system_enabled or params["system"] <= 0)):
        return []
    proposals = []
    if params["system"] > 0:
        if systems["training"] and observation["rules"]["specialistsEnabled"]:
            proposals.extend(training_proposals(observation, board, params))
        if systems["walls"] and (observation["rules"]["woodWallsEnabled"] or observation["rules"]["stoneWallsEnabled"]):
            proposals.extend(wall_proposals(observation, board, params))
        if systems["espionage"]:
            proposals.extend(espionage_proposals(observation, board, params))
    if beacon_allowed:
        hand_off = beacon_hand_off_proposal(observation, board, params)
        if hand_off != None:
            proposals.append(hand_off)
        deploy_or_restore = beacon_deploy_or_restore_proposal(observation, board, params)
        if deploy_or_restore != None:
            proposals.append(deploy_or_restore)
        if observation["rules"]["beaconForgeEnabled"]:
            proposals.extend(beacon_forge_proposals(observation, board, params))
    return proposals


# =============================================================================
# Memory: a tiny, bounded decision record for refused-command suppression,
# leader anti-reversal and short quiet-cycle avoidance. Live route activity
# itself comes from each current Cell.charging fact, never stale memory.
# =============================================================================

# =============================================================================
# Route/channel commitment (sneat-co/chessraiders#84, route-commitment —
# COMMITMENT_DECAY's own comment has the full accounting of WHY). WHICH kind
# of in-flight channel this bot is currently defending — a unit's own
# move/route charge, or the king's own Beacon Restore, both of which
# chess/apply.go's shared trial-cancel path (`interrogating || restoring ||
# ...`) protects from a competing command the identical way — plus the
# WINNING proposal's own score at the moment it was submitted and how many
# decisions have passed since, packed into ONE int64 memory entry
# ("committed") rather than two or three bare ones: leader_guard_keys' own
# comment measures the full worst-case shape at 25 of Recruit's 32-entry
# MemoryBudget (a move having happened AND a leader guard active
# simultaneously) — packing age/kind/score into ONE entry keeps that margin
# comfortable rather than spending it down key by key. Arithmetic packing,
# not bitwise (unlike the placement bitboards
# above): age and score both need ordinary decimal magnitudes, not flag
# bits, and BotMemory.Values is int64-only (server-go/botcontract/memory.go)
# — score is fixed-point, not float, for exactly that reason. Interrogation
# shares the same engine-level trial-cancel path but is deliberately NOT a
# third kind here — is_king_channel_start's own doc comment has the reason
# (a pinned test's own model of the king staying free during its own
# suspect's Interrogation).
# =============================================================================
COMMIT_KIND_ROUTE = 1  # a unit's own move/route charge (board["charging_units"])
COMMIT_KIND_KING = 2  # the king's own Beacon Restore channel
COMMIT_SCORE_SCALE = 1000.0  # three decimal digits of score precision
COMMIT_SCORE_OFFSET = 2000000  # keeps the packed score field non-negative
COMMIT_SCORE_FIELD = 4000000  # score field width: +/-2000.0, comfortably above every reachable committed score (PRIORITY_CAPTIVE_DELIVERY_SCORE's 1000 included)
COMMIT_AGE_FIELD = 100000  # width of the age field within the packed value
COMMIT_AGE_CAP = 64  # COMMITMENT_DECAY^64 is negligible for every candidate decay value tried; bounds both the decay_power loop and the packed value's own growth

def round_half_up(value):
    """Starlark's Universe has no round() builtin (go.starlark.net's own
    predeclared set stops at int()/float()), so this rounds a float to the
    nearest integer by hand, ties away from zero — adequate for packing a
    committed score, which is never itself a tie-break input."""
    if value >= 0:
        return int(value + 0.5)
    return -int(-value + 0.5)

def pack_committed(kind, age, score):
    offset_score = round_half_up(score * COMMIT_SCORE_SCALE) + COMMIT_SCORE_OFFSET
    if offset_score < 0:
        offset_score = 0
    elif offset_score >= COMMIT_SCORE_FIELD:
        offset_score = COMMIT_SCORE_FIELD - 1
    return kind * (COMMIT_AGE_FIELD * COMMIT_SCORE_FIELD) + age * COMMIT_SCORE_FIELD + offset_score

def unpack_committed(packed):
    """Returns (kind, age, score). kind is 0 — neither COMMIT_KIND_ROUTE nor
    COMMIT_KIND_KING is ever 0 — for an absent entry: memory.get("committed",
    0)'s own default reads back as exactly this sentinel, the same
    "0/NO_SQUARE_INDEX means absent" convention this file's other memory
    fields already use."""
    field = COMMIT_AGE_FIELD * COMMIT_SCORE_FIELD
    kind = packed // field
    remainder = packed % field
    age = remainder // COMMIT_SCORE_FIELD
    offset_score = remainder % COMMIT_SCORE_FIELD
    return kind, age, (offset_score - COMMIT_SCORE_OFFSET) / COMMIT_SCORE_SCALE

def decay_power(age):
    """COMMITMENT_DECAY raised to `age` by repeated multiplication —
    Starlark has no exponent operator, and age is capped small
    (COMMIT_AGE_CAP) so the loop costs nothing against the tier's own
    StepBudget."""
    value = 1.0
    for _ in range(min(age, COMMIT_AGE_CAP)):
        value *= COMMITMENT_DECAY
    return value

def retention_score(memory, kind):
    """The value a replacement candidate must beat to cancel the in-flight
    channel of the given KIND — ROUTE_REPLACE_BASELINE alone (today's flat
    wall, unregressed) when memory holds no commitment for THIS kind (never
    committed, a stale entry left by a DIFFERENT kind, or the tracked
    channel already settled and was cleared by next_committed), otherwise
    the decayed committed value plus that same baseline (COMMITMENT_DECAY's
    own comment has the full accounting)."""
    stored_kind, age, score = unpack_committed(memory.get("committed", 0))
    if stored_kind != kind:
        return ROUTE_REPLACE_BASELINE
    return score * decay_power(age) + ROUTE_REPLACE_BASELINE

def is_king_channel_start(intent):
    """Whether `intent` opens the one king-actor multi-decision channel this
    file protects with commitment: Beacon Restore. chess/apply.go's own
    shared trial-cancel path (`interrogating || restoring || ...`) treats
    Interrogate identically at the ENGINE level, but THIS script's own
    busy_units already models "our king interrogating our own suspect" as
    NOT locking the king (build_board's own comment: only the suspect is
    marked busy) — a deliberate, pre-existing choice
    TestSimultaneousInterrogationsBusyBothTargetsAndOurKing pins (the king
    stays free to act while its own suspect's Interrogation continues), so
    treating Interrogate as a protectable channel here would fight that
    pinned behaviour rather than extend it: gating king proposals against a
    decayed Interrogate commitment leaves the king "channel active" in a
    position that test asserts it is NOT. Extending protection to
    Interrogate is therefore left OUT — see this task's own report for the
    full reasoning — and every OTHER king action (an ordinary move, Beacon
    Deploy/Hand-off, a wall dismantle) is a single-decision act with
    nothing to protect either way."""
    return intent != None and intent["kind"] == "action" and intent.get("action") == "beacon_restore"

def king_channel_active(observation, board):
    """Whether the king currently has an in-flight Beacon Restore —
    chess/apply.go's own restoring() fact, read from the wire the script
    already has (observation["beacon"]["lifecycle"]) rather than new host
    plumbing. See is_king_channel_start's own doc comment for why
    Interrogation is deliberately not read here too."""
    if board["king_cell"] == None:
        return False
    return observation.get("beacon", {}).get("lifecycle") == "restoring"

def next_committed(observation, board, memory, chosen):
    """The next "committed" memory value: a fresh commitment (age 0) the
    moment a route-worthy move or a Beacon Restore is actually chosen; one
    more decision of age on the SAME tracked channel while it survives this
    decision untouched; or cleared (0, popped by finish_decision) the
    instant neither a route charge nor the king's Restore is in flight — "a
    stale entry protecting a dead route is a bug" (route-commitment design,
    sneat-co/chessraiders#84), so channel liveness is read fresh from
    board/observation every single decision, never from a memory-side
    ticker of its own.

    `chosen` is the winning proposal dict (intent/score/actor) exactly as
    every other call site in this file already shapes one, or None for a
    pass — the same shape decide() already threads through rank_options."""
    intent = chosen["intent"] if chosen != None else None
    if intent != None and intent["kind"] == "move":
        mover = board["own_by_square"].get(intent["from"])
        charge_ms = observation["rules"]["pieceChargeMs"].get(mover["rank"], 0) if mover else 0
        if charge_ms > 0:
            # The only "move" intents reachable while a route is ALREADY
            # charging are move_proposals' own replaceable-charging-unit
            # destinations (that function's own doc comment) — a genuine
            # replacement. Reached before any route is charging yet, this
            # is instead a route's FIRST commit, recorded before the charge
            # itself is even visible on the wire (it only appears on the
            # NEXT decision's observation) — recording it here means
            # retention_score never has to special-case "brand new" versus
            # "replaced". Gated on the mover's own charge duration (rather
            # than recording on every move unconditionally) so an instant,
            # zero-duration move never spends a memory entry it can never
            # need — TestQuietCycleRingPreservesPlacementOnlyActionsAndFits
            # LeaderMemory's own worst-case count pins exactly this.
            return pack_committed(COMMIT_KIND_ROUTE, 0, chosen["score"])
    if is_king_channel_start(intent):
        return pack_committed(COMMIT_KIND_KING, 0, chosen["score"])

    stored_kind, age, score = unpack_committed(memory.get("committed", 0))
    if board["charging_units"] and stored_kind == COMMIT_KIND_ROUTE:
        return pack_committed(COMMIT_KIND_ROUTE, min(age + 1, COMMIT_AGE_CAP), score)
    king_id = board["king_cell"]["unitId"] if board["king_cell"] != None else NO_SQUARE_INDEX
    actor = chosen.get("actor") if chosen != None else None
    if king_channel_active(observation, board) and stored_kind == COMMIT_KIND_KING and actor != king_id:
        # The channel survives untouched only when THIS decision's own
        # action, if any, was not the king's own — ANY king action cancels
        # an in-flight Restore (chess/apply.go's shared trial-cancel path),
        # and a same-kind continuation (a fresh Restore) already returned
        # its own new commitment above.
        return pack_committed(COMMIT_KIND_KING, min(age + 1, COMMIT_AGE_CAP), score)
    return 0

def finish_decision(observation, board, memory, chosen, ranked):
    """The one place every decide() return path funnels through: builds the
    ordinary memory shape (build_memory, untouched by this mechanism) and
    layers this decision's own "committed" entry on top of it — present
    only while it is tracking something (next_committed's own 0 sentinel
    pops it, matching every other NO_SQUARE_INDEX-style absent-entry
    convention this file already uses rather than writing a meaningless
    zero into the budget)."""
    intent = chosen["intent"] if chosen != None else None
    updated_memory = build_memory(observation, memory, intent)
    committed = next_committed(observation, board, memory, chosen)
    if committed == 0:
        updated_memory.pop("committed", None)
    else:
        updated_memory["committed"] = committed
    return intent, updated_memory, ranked

def holds_focus(board, memory):
    """Retain only a route visibly committed to the enemy king. Other own
    charges stay system-busy but may be replaced by a host-offered legal move."""
    focus_marker = memory.get("focusFrom", 0)
    if focus_marker <= 0:
        return False
    focus_square = square_name(focus_marker - 1)
    focus_cell = board["own_by_square"].get(focus_square)
    return focus_cell != None and board["protected_charging_units"].get(focus_cell["unitId"], False)

def drop_refused(proposals, observation, memory):
    """If our last command(s) at THIS SAME revision were refused, leave
    those pieces alone this turn rather than proposing them again — a
    SMALL SET of up to REFUSED_SET_SIZE most-recently-refused squares
    (build_memory's own refused0..refused{N-1} ring buffer), not just the
    single most recent one. One remembered square is not enough: two
    pieces that keep alternately tying for the same gated target (say, two
    attackers on a morale-gated king, each scoring far above every other
    candidate) can defeat it forever — excluding whichever was refused
    LAST always lets the OTHER one back into contention, which gets
    refused in turn and excludes the first one right back. The fix that
    actually resolves that livelock is a SEEDED host_random_draw
    (runtime.go's own doc comment): at a frozen revision the tie-break
    stops re-rolling, so a single exclusion is enough once the draw itself
    is deterministic. This wider set is defence in depth on top of that,
    for however many pieces turn out to tie, and costs nothing when only
    one ever does."""
    if not memory or memory.get("revision") != observation["revision"]:
        return proposals
    refused_squares = []
    for slot in range(REFUSED_SET_SIZE):
        index = memory.get("refused" + str(slot), NO_SQUARE_INDEX)
        if index >= 0 and index <= MAX_SQUARE_INDEX:
            refused_squares.append(square_name(index))
    if not refused_squares:
        return proposals
    return [proposal for proposal in proposals if proposal["intent"]["from"] not in refused_squares]

# One-ply leader reversal guard. It deliberately keeps placement as twelve
# signed int64 side×rank bitboards, not a hash: each occupied square sets one
# unique bit in exactly one category, so any intervening relocation, capture
# or promotion changes at least one stored value. That twelve-board shape is
# still how every comparison below reasons about placement — only how it is
# PERSISTED to memory changed (sneat-games/chessraiders bitboard-safe
# incident): bot memory round-trips as JSON int64 through the host, exact on
# the server, but the browser (wasm) path relays it through JavaScript's
# float64 numbers, safe only up to JS_SAFE_INTEGER_MAX. A raw 64-bit bitboard
# routinely sets bit 53 or higher (any occupied square with index >= 53) and
# bit 63 flips the sign; both are silently-or-loudly unsafe in the browser —
# measured production failure: a black rook on h8 (square index 63, bit 63 of
# the black-rook category board) round-tripped through JS as a corrupted
# magnitude that Go's own int64 JSON unmarshal then rejected outright, and
# every later leader-guard decision for that bot failed identically from
# then on. Since every square belongs to at most one of the twelve boards
# (the same invariant this comment already leans on), pack_placement_boards
# and unpack_placement_boards below store the whole 64-square layout as ONE
# base-16 digit per square (0 = empty, 1..12 = the occupying board's slot +
# 1) instead of twelve raw 64-bit values: PLACEMENT_SQUARES_PER_SLOT (13)
# nibbles — 52 bits, comfortably under JS_SAFE_INTEGER_MAX's 53 — pack into
# each of PLACEMENT_BOARD_SLOTS (5) int64 memory entries. Lossless, and
# CHEAPER than the naive two-JS-safe-halves-per-board split (12 boards × 2 =
# 24 entries): five slots plus active/from/to/kind add nine entries to the
# existing eleven-entry memory shape. The three quiet-vacated slots make 23;
# a historical action counter brings the measured worst case (a move having
# happened AND a leader guard active simultaneously) to 24/32 Recruit
# entries, 25/32 with a route/Beacon-Restore channel also committed — eight
# or seven spares, not zero.
RANK_BIT_SLOTS = {"pawn": 0, "knight": 1, "bishop": 2, "rook": 3, "queen": 4, "king": 5}

def signed_bitboard(value):
    return value - BITBOARD_MODULUS if value >= BITBOARD_SIGN_BIT else value

def placement_bitboards(observation, moved_unit_id = 0, moved_to = ""):
    bitboards = [0] * 12
    for occupied_square in piece_squares(observation):
        cell = observation["pieces"][occupied_square]
        rank_slot = RANK_BIT_SLOTS.get(cell["rank"])
        if rank_slot == None:
            continue
        side_slot = 0 if cell["side"] == "white" else 6
        square = moved_to if cell["unitId"] == moved_unit_id else occupied_square
        bitboards[side_slot + rank_slot] |= 1 << square_index(square)
    return [signed_bitboard(value) for value in bitboards]

def pack_placement_boards(bitboards):
    """The JS-float64-safe memory encoding for placement_bitboards' own
    twelve-board result — see this section's own top comment for why. Every
    square sets at most one bit across the twelve boards, so this walks all
    64 squares once, looks up which (if any) board claims each one, and
    packs the per-square board-slot-plus-one code (0..12, fits in a nibble)
    PLACEMENT_SQUARES_PER_SLOT at a time into PLACEMENT_BOARD_SLOTS plain
    (never negative, never large) int64 entries."""
    packed = {}
    for slot_index in range(PLACEMENT_BOARD_SLOTS):
        base_square = slot_index * PLACEMENT_SQUARES_PER_SLOT
        squares_in_slot = min(PLACEMENT_SQUARES_PER_SLOT, 64 - base_square)
        value = 0
        for local in range(squares_in_slot):
            square = base_square + local
            code = 0
            for board_slot in range(12):
                raw = bitboards[board_slot]
                if raw < 0:
                    raw += BITBOARD_MODULUS
                if (raw >> square) & 1:
                    code = board_slot + 1
                    break
            value |= code << (local * 4)
        packed["leaderBoard" + str(slot_index)] = value
    return packed

def unpack_placement_boards(memory):
    """The exact inverse of pack_placement_boards: reconstructs
    placement_bitboards' own twelve-signed-int64-board shape so every
    comparison below stays unchanged — only the memory boundary moved."""
    bitboards = [0] * 12
    for slot_index in range(PLACEMENT_BOARD_SLOTS):
        base_square = slot_index * PLACEMENT_SQUARES_PER_SLOT
        squares_in_slot = min(PLACEMENT_SQUARES_PER_SLOT, 64 - base_square)
        value = memory.get("leaderBoard" + str(slot_index), 0)
        for local in range(squares_in_slot):
            code = (value >> (local * 4)) & 0xF
            if code != 0:
                bitboards[code - 1] |= 1 << (base_square + local)
    return [signed_bitboard(value) for value in bitboards]

def leader_kind(observation, cell):
    if cell["rank"] == "king":
        return 6  # king's rank bitboard slot + 1
    if is_current_beacon_bearer(observation, cell):
        return RANK_BIT_SLOTS.get(cell["rank"], -1) + 1
    return 0

def leader_guard_keys():
    return ["leaderGuardActive", "leaderGuardFrom", "leaderGuardTo", "leaderGuardKind"] + ["leaderBoard" + str(slot) for slot in range(PLACEMENT_BOARD_SLOTS)]

def clear_leader_guard(memory):
    for key in leader_guard_keys():
        memory.pop(key, None)

def leader_guard_matches(observation, memory):
    if memory.get("leaderGuardActive", 0) != 1:
        return False
    from_square = square_name(memory["leaderGuardFrom"])
    to_square = square_name(memory["leaderGuardTo"])
    # During charging/pre-layout the source placement is still visible. Keep
    # the guard rather than treating that expected transient as a board edit.
    source_cell = projected_cell(observation, from_square)
    if source_cell and source_cell["side"] != observation["side"]:
        source_cell = None
    if source_cell and leader_kind(observation, source_cell) == memory["leaderGuardKind"]:
        expected_pre = unpack_placement_boards(memory)
        side_slot = 0 if source_cell["side"] == "white" else 6
        board_slot = side_slot + memory["leaderGuardKind"] - 1
        value = expected_pre[board_slot]
        if value < 0:
            value += BITBOARD_MODULUS
        value ^= (1 << memory["leaderGuardFrom"]) | (1 << memory["leaderGuardTo"])
        expected_pre[board_slot] = signed_bitboard(value)
        return placement_bitboards(observation) == expected_pre
    destination_cell = projected_cell(observation, to_square)
    if destination_cell and destination_cell["side"] != observation["side"]:
        destination_cell = None
    if not destination_cell or leader_kind(observation, destination_cell) != memory["leaderGuardKind"]:
        return False
    return placement_bitboards(observation) == unpack_placement_boards(memory)

def active_leader_guard(observation, memory):
    if not leader_guard_matches(observation, memory):
        return None
    return {"from": memory["leaderGuardFrom"], "to": memory["leaderGuardTo"], "kind": memory["leaderGuardKind"]}

def leader_reverse_forbidden(observation, guard, cell, destination):
    if guard == None:
        return False
    if cell["square"] != square_name(guard["to"]) or destination != square_name(guard["from"]):
        return False
    if leader_kind(observation, cell) != guard["kind"]:
        return False
    # A threatened leader may always take the exact reverse as an escape.
    return threatened_count(cell) <= 0

def memory_last_to_is(memory, square):
    return memory.get("lastTo", NO_SQUARE_INDEX) == square_index(square)

def quiet_leader_intent(observation, intent):
    if not intent or intent["kind"] != "move" or intent.get("promotion"):
        return None
    cell = projected_cell(observation, intent["from"])
    if cell and cell["side"] != observation["side"]:
        cell = None
    if not cell or cell["convoy"] or not leader_kind(observation, cell):
        return None
    board = {"enemy_by_square": {square: projected_cell(observation, square) for square in piece_squares(observation) if observation["pieces"][square]["side"] != observation["side"]}}
    if not is_quiet_move(observation, board, cell, intent["to"]):
        return None
    return cell

def quiet_ordinary_intent(observation, intent):
    if not intent or intent["kind"] != "move" or intent.get("promotion"):
        return None
    cell = projected_cell(observation, intent["from"])
    if cell and cell["side"] != observation["side"]:
        cell = None
    if not cell or cell["convoy"] or leader_kind(observation, cell):
        return None
    board = {"enemy_by_square": {square: projected_cell(observation, square) for square in piece_squares(observation) if observation["pieces"][square]["side"] != observation["side"]}}
    return cell if is_quiet_move(observation, board, cell, intent["to"]) else None

def build_memory(observation, memory, intent):
    """The next memory to persist: always records the revision and the
    intent just chosen (or NO_SQUARE_INDEX for a pass), and additionally
    the focus square when the intent is a move — mirrors bot4chess's own
    remember() field for field (revision, lastFrom, lastTo, actions, moves,
    focusFrom).

    Also advances drop_refused's own small ring buffer
    (refused0..refused{REFUSED_SET_SIZE-1}, refusedCursor): RESET to hold
    only THIS decision's own from-square the moment the revision moves on
    (a settled command means every earlier refusal at the OLD revision no
    longer applies — drop_refused's own revision gate would ignore them
    anyway, but starting fresh here keeps a future frozen spell from ever
    excluding a square for a reason that stopped being true), and
    otherwise APPENDED to — round-robin, oldest entry evicted first — so a
    run of refusals at the SAME frozen revision keeps growing the excluded
    set instead of only ever remembering the latest one."""
    updated_memory = dict(memory)
    if updated_memory.get("leaderGuardActive", 0) == 1 and not leader_guard_matches(observation, updated_memory):
        clear_leader_guard(updated_memory)
    from_index = square_index(intent["from"]) if intent else NO_SQUARE_INDEX
    to_index = square_index(intent["to"]) if intent else NO_SQUARE_INDEX

    still_frozen = memory.get("revision") == observation["revision"]
    cursor = memory.get("refusedCursor", 0) if still_frozen else 0
    if not still_frozen:
        for slot in range(REFUSED_SET_SIZE):
            updated_memory["refused" + str(slot)] = NO_SQUARE_INDEX
    updated_memory["refused" + str(cursor)] = from_index
    updated_memory["refusedCursor"] = (cursor + 1) % REFUSED_SET_SIZE

    updated_memory["revision"] = observation["revision"]
    updated_memory["lastFrom"] = from_index
    updated_memory["lastTo"] = to_index
    if intent and intent["kind"] == "action":
        updated_memory["actions"] = memory.get("actions", 0) + 1
    elif intent and intent["kind"] == "move":
        updated_memory["moves"] = memory.get("moves", 0) + 1
        updated_memory["focusFrom"] = from_index + 1  # the piece this bot is now committed to; holds_focus reads it back
    leader = quiet_leader_intent(observation, intent)
    if leader:
        updated_memory["leaderGuardActive"] = 1
        updated_memory["leaderGuardFrom"] = from_index
        updated_memory["leaderGuardTo"] = to_index
        updated_memory["leaderGuardKind"] = leader_kind(observation, leader)
        bitboards = placement_bitboards(observation, leader["unitId"], intent["to"])
        for key, value in pack_placement_boards(bitboards).items():
            updated_memory[key] = value
    quiet_ordinary = quiet_ordinary_intent(observation, intent)
    updated_memory["lastQuietTo"] = to_index if quiet_ordinary else NO_SQUARE_INDEX
    if quiet_ordinary:
        for slot in range(QUIET_VACATED_SQUARES - 1, 0, -1):
            updated_memory["quietVacated" + str(slot)] = memory.get("quietVacated" + str(slot - 1), NO_SQUARE_INDEX)
        updated_memory["quietVacated0"] = from_index
    elif intent and intent["kind"] == "move":
        # Captures, promotions, convoys and leaders change piece placement,
        # rank or cargo; do not let a stale quiet shuffle constrain the new
        # layout. A None/charging pass and every system action preserve it:
        # neither changes the acceptance rule's piece placement.
        for slot in range(QUIET_VACATED_SQUARES):
            updated_memory["quietVacated" + str(slot)] = NO_SQUARE_INDEX
    return updated_memory

def intn(random_draw, count):
    """Stands in for the target host builtin rand.intn(count) — see this
    file's own top comment and the report's Host Interface section for why
    this pass draws ONE host value up front instead of calling a live
    builtin. count <= 1 always returns 0 WITHOUT using the draw, so a
    single candidate never depends on randomness — mirrors bot4chess/
    rng.go's own intn exactly, including that comment's reasoning."""
    if count <= 1:
        return 0
    return random_draw % count


# =============================================================================
# The entry point. Observe (already done, by the host), evaluate, choose —
# read top to bottom, this is the whole decision in five named steps.
# =============================================================================

def decide(observation, memory, params, host_random_draw, options = 0):
    """Returns a THREE-tuple: (chosen_intent, updated_memory, explained_options).

    `options` is how many explained candidates the caller wants — 0 for a bot
    decision, which is every caller that plays. The third return value is
    ALWAYS present and is simply `[]` at 0, so there is one return shape, one
    scoring pass and one candidate walk regardless of who is asking. The
    observable property adviser-tier's AC:one-scoring-path-two-outputs holds
    this to: for any position and any parameter row, the intent returned at
    options = 0 is the same intent as option 1 at options >= 1, tie-break
    randomness excluded.

    ONE EXCEPTION to "one scoring pass, one candidate walk": the founder's
    own hard captive-delivery rule (priority_captive_delivery_proposal,
    spec: retire-the-go-tiers#A4(a)) is a SHORT-CIRCUIT, not a score, so it
    is checked and — when it fires — returned BEFORE move_proposals/
    system_proposals ever run, exactly mirroring bot4chess.go's own Decide.
    It still goes through rank_options as the sole candidate, so an adviser
    asking about this exact position sees the one move that will actually
    be played, with a "priorityDelivery" term explaining why, rather than
    an empty list.

    ROUTE/CHANNEL COMMITMENT (sneat-co/chessraiders#84, route-commitment):
    every return path below funnels through finish_decision rather than
    build_memory directly — see that function's own doc comment, and
    COMMITMENT_DECAY's, for why and how the ONE new "committed" memory
    entry is written, decayed and cleared."""
    if observation["lifecycle"] != "playing":
        return None, memory, []

    board = build_board(observation)
    if not board["own"]:
        return None, memory, []

    # A visible enemy-king charge is already the highest-value objective.
    # Retain it even when it originated outside this bot's memory.
    if board["protected_charging_units"]:
        return finish_decision(observation, board, memory, None, [])

    # StandardRules' convoy charge (2s) is longer than the Commander
    # cadence (1s). Keep this precise first-Engineer delivery route until it
    # settles instead of letting an equally-homeward alternative reset it.
    if priority_captive_delivery_in_flight(observation, board):
        return finish_decision(observation, board, memory, None, [])

    # See this function's own doc comment above: gated identically to
    # bot4chess.go's own Decide (not holds_focus, no king-carrying convoy
    # already in flight — board["convoy_home"] empty).
    if not board["charging_units"] and not holds_focus(board, memory) and not board["convoy_home"]:
        priority = priority_captive_delivery_proposal(observation, board, params)
        if priority != None:
            ranked = rank_options([priority], params, options)
            return finish_decision(observation, board, memory, priority, ranked)

    proposals = []
    if not holds_focus(board, memory):
        proposals = move_proposals(observation, board, params, memory)
    if not board["charging_units"]:
        proposals = proposals + system_proposals(observation, board, params)
    proposals = drop_refused(proposals, observation, memory)

    # King-channel commitment (sneat-co/chessraiders#84, route-commitment):
    # a Beacon Restore is a multi-decision channel exactly like a route
    # charge, but the king is never marked busy for it (build_board's own
    # busy_units computation has no Restore case), so every OTHER king-actor
    # candidate this decision generated — an ordinary move, Beacon
    # Deploy/Hand-off, a wall dismantle, an Interrogate — must beat the SAME
    # decayed-commitment retention_score protects a route with, or it is
    # dropped here before it can even compete. Left unguarded, an in-flight
    # Restore measured a start-cancel loop against exactly these competing
    # king actions (route-commitment design doc, sneat-co/chessraiders#84: a
    # Restore/Interrogate ping-pong, 1446/1467 cycles across a 12,000-tick
    # soak) — is_king_channel_start's own doc comment explains why
    # Interrogation itself is not ALSO a protected kind here, only one of
    # the actions this gate weighs against a protected Restore. Gated on
    # `not board["charging_units"]` because system_proposals itself already
    # is (this function's own call above) — a route charge and the king's
    # Restore are never BOTH freshly competing on the same decision.
    if king_channel_active(observation, board) and not board["charging_units"] and board["king_cell"] != None:
        king_threshold = retention_score(memory, COMMIT_KIND_KING)
        king_id = board["king_cell"]["unitId"]
        proposals = [proposal for proposal in proposals if proposal.get("actor") != king_id or proposal["score"] > king_threshold]

    # Retaining an active route is a real zero-score choice. Replacement is
    # the one-command-at-a-time exception, not an invitation to cancel sunk
    # progress for the tier's ordinary negative pass floor. The urgency term
    # above makes a nearly settled route harder to beat for every tier,
    # including Recruit whose ordinary move-duration tempo weight is zero.
    # The threshold itself is VALUE-AWARE, not the flat ROUTE_REPLACE_
    # BASELINE alone (COMMITMENT_DECAY's own comment has the full
    # accounting) — retention_score falls back to that same flat baseline
    # whenever memory holds no route commitment at all.
    if board["charging_units"]:
        threshold = retention_score(memory, COMMIT_KIND_ROUTE)
        proposals = [proposal for proposal in proposals if proposal["score"] > threshold]

    if not proposals:
        return finish_decision(observation, board, memory, None, [])

    proposals = sorted(proposals, key=_proposal_score_key)
    # Ranked BEFORE the pass check below, from the same sorted list, so an
    # adviser asking about a position whose best move is below its own floor
    # gets an honest empty list rather than a padded one — rank_options
    # applies passBelow itself.
    ranked = rank_options(proposals, params, options)
    best_proposal = proposals[0]
    if best_proposal["score"] < params["passBelow"]:
        return finish_decision(observation, board, memory, None, ranked)

    # Tie-break among equally good choices — the only randomness this bot
    # has, and it never affects a decision with a single good answer. It is
    # deliberately NOT used to order `ranked` (rank_options' own doc comment).
    tied_count = 1
    while tied_count < len(proposals) and proposals[tied_count]["score"] >= best_proposal["score"] - TIE_BREAK_BAND:
        tied_count += 1
    chosen = proposals[intn(host_random_draw, tied_count)]
    return finish_decision(observation, board, memory, chosen, ranked)
