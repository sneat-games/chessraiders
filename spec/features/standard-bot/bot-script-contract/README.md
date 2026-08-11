---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Bot Script Contract

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot/bot-script-contract?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot/bot-script-contract?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot/bot-script-contract?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot/bot-script-contract?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

What a bot script is handed each decision, what it may return, and the one source of randomness it may use.

**Mechanic:** the host supplies one fog-correct piece map, actionable choices,
post-move relation facts, and public activity state; the script may select but
never reconstruct them.
**Real-world analogy:** a commander receives a current reconnaissance report
and an approved-order sheet, not the enemy's hidden map or permission to
invent roads.

## Problem

A script that can quietly work out its own version of a rule is a script
whose author is really re-writing the game's logic and hoping it agrees —
and a mismatch like that has bitten this project before: a shipped script
once tried to re-derive one of the game's own gates and got it slightly
wrong, giving a wrong answer that stood for weeks before anyone noticed.
The fix that has stuck since is to never let a script derive an answer the
game could just hand it directly. Everything below follows from that one
decision, and it's the same contract whether the script is one of the
[three built-in difficulties](../README.md) or one you write yourself.

## Behavior

### What a script is

#### REQ: one-entry-point

A bot script defines exactly one function, called once every time the bot
is due to act. It returns either nothing — a deliberate pass, not a
failure — or a single instruction: which of the bot's own pieces to
command, and what to do with it.

### What it's handed

#### REQ: the-observation-is-your-own-sides-view

A script is handed exactly the view your own side would see if you were
playing by hand — under [Fog of War](../../fog-of-war/README.md), no more
than your own patrol vision and any active informers show you, never the
true board. Nothing about it changes because the caller is a script
rather than a person.

#### REQ: legal-moves-are-already-worked-out

Alongside that view, `legal[unitId]` gives the host-offered actionable
destinations for each own piece, including replacement destinations for a
piece whose route is already charging. A script never computes where a piece
is allowed to go — it can plan around that set (a route home for a captured
king's convoy is exactly this: chaining several one-step offered moves into a
plan), but the one move it issues must be a destination the game offered.

Under Fog of War, an offered destination is a belief-correct hint, not a leak
from the true hidden board. Submission revalidates it atomically and may still
reject it because of an unseen blocker without identifying that blocker. A
script that names a destination the host did not offer is rejected before it
reaches the board.

#### REQ: pieces-map-is-the-only-board-shape

The board is `pieces`, one map keyed by algebraic square. Each value carries
an opaque stable `unitId`, an explicit `side`, rank and physical state; it
does not repeat its square. Side is never inferred from the unit ID because
recruitment can change ownership. Scripts sort the square keys before
iteration, and the published standard bot reconstructs a transient square
field without mutating the observation.

There is no parallel `own`/`enemy` board and no top-level `danger` summary.
Every visible non-ghost piece may instead carry sorted, duplicate-free square
lists: `threatens`, `threatenedBy`, `guards`, and `guardedBy`; omitted means
empty. These relations describe current combat reach, independently of
cooldown and morale affordability. Ghosts carry no live relations.

#### REQ: deterministic-candidates-describe-the-exact-post-state

`candidates[unitId][destination]` exists for deterministic non-capture
relocations. It carries `destinationVisible`, the host's fog-safe `patrolGain`,
`nextPossibleMoves`, and the same four post-move relation lists. Presence of a
candidate does not make its destination visible, and omitted lists mean empty.

Candidate facts cover ordinary quiet moves, deterministic convoy relocation
and unload/refit/delivery, and a chosen-queen quiet promotion. Captures,
en-passant and branching choices have no fabricated single future state:
their affordability and outcomes remain separate, and current target
`guardedBy` relations describe visible recapture risk. A script must not infer
piece geometry, future attacks, or protection from distance.

An already charging unit may conservatively omit candidate facts when its
hidden route state prevents a fog-correct settled projection. `legal` still
authorizes same-actor replacement; a capture replacement remains fully scored
from current target relations and affordability, while an absent quiet fact
earns no speculative future bonus.

#### REQ: activity-fields-preserve-command-privacy

An own active route is `charging: {square, remainingMs}`. A currently visible
enemy route exposes only `isCharging: true` and `chargingRemainingMs`, never
its destination; invisible enemies and ghosts expose neither. Public physical
recovery is `recoveryRemainingMs`; own-only work is
`forgingRemainingMs` or `training: {profession, grade?, remainingMs}`; a piece
being interrogated carries only `interrogationRemainingMs`, from which the
sole adjacent king interrogator is derived. Inactive fields are omitted.

Specialist identity uses `profession`, engineer `grade` (`basic` or `master`),
and an optional `eligibleFor` list containing `masterEngineerTraining`; the
removed boolean summaries are not accepted as a second schema.

#### REQ: parameters-are-just-numbers-you-declare

A script declares the settings it wants — a small table of named values,
each with a type, a default and a range — and is handed exactly those
values, validated before the match even starts. This is the entire
difference between [Recruit, Lieutenant and Commander](../README.md): one
script, and a different row of numbers for each. A configuration that
doesn't validate is refused before anything begins, never partway through
a match. A parameter value is always plain data — never source code, and
never something the game runs or interprets as an instruction of its own.

#### REQ: memory-is-the-only-thing-that-carries-over

A script can hand itself a small bundle of whole numbers, keyed by name,
to read back on its next turn — that bundle is the only thing that
survives between one decision and the next. A script can't remember
anything any other way: nothing it does outside of returning that bundle
sticks around for its next turn. A returned value that doesn't fit — a
number too large, or something that isn't a whole number — is a scripting
error, never silently rounded or truncated.

#### REQ: one-source-of-randomness

When a script needs to break a tie, it draws from exactly one place the
game hands it — never a randomness source it invents itself. Two
candidate moves that score identically are separated this way, never by
which one happens to come first in some list.

### What comes back

#### REQ: an-instruction-names-a-piece-and-a-destination

The instruction a script returns is small and plain: which piece, and
what to do with it — move, or one of the game's other commands (train a
specialist, work on a fortification, and so on) drawn from exactly the
same vocabulary a human player's own click would produce. Anything else —
an unrecognised action, a destination outside the legal set handed to the
script, a value that doesn't belong to the option it's attached to — is
rejected as a scripting error before it can reach the board, and the same
mistake always produces the same rejection, whether or not the
destination happens to be occupied.

### Reading the board the same way, every time

#### REQ: the-same-position-reads-the-same-way

Everything a script can look through — sorted `pieces` keys, relation lists,
candidate lists, legal moves, and its own configuration values — always
comes in the same order for the same position. A script's behaviour never
depends on map iteration or another ordering nobody promised it, so the same
script facing the same position always sees it the same way.

## Dependencies

- [Standard Bot](../README.md) — the three difficulties that use this
  contract, and the one-script mechanism it depends on.
- [Fog of War](../../fog-of-war/README.md) — the projection every
  observation is built from.
- [bot-execution-limits](../bot-execution-limits/README.md) — what happens
  when a script exceeds what this contract allows it.

## Acceptance Criteria

### AC: a-script-cannot-name-a-square-the-game-didnt-offer

**Given** a script that ignores its supplied legal moves and returns a
destination it worked out for itself
**When** that destination is not one the game actually offered
**Then** the instruction is rejected before it reaches the board, and no
illegal move is ever issued

### AC: a-script-sees-only-its-own-sides-view

**Given** a Fog of War match where the enemy holds a unit the bot's own
side cannot see
**When** a script's decision is inspected
**Then** nothing in it mentions that hidden unit, and nothing the script
can call gives it a way to ask about one

### AC: observation-has-one-unambiguous-board-schema

**Given** a current bot observation
**When** its board is decoded
**Then** every projected occupant appears exactly once under its algebraic
key in `pieces`, carries explicit `side` and opaque `unitId`, and no
`own`/`enemy` arrays, cell `square`, top-level `danger`, or removed boolean
specialist summary is present

### AC: candidate-relations-are-exact-and-geometry-free

**Given** one deterministic quiet move and one branching capture
**When** their host facts are inspected
**Then** the quiet move's `candidates` entry carries visibility, patrol,
next-move and post-state relation facts while the capture has no fabricated
post-state candidate, and the script uses supplied relations/outcomes rather
than distance or piece-geometry guesses

### AC: charge-activity-shows-progress-without-leaking-enemy-targets

**Given** one own charging piece, one currently visible charging enemy, and
one fogged enemy ghost
**When** the bot observation is produced
**Then** the own piece exposes destination and remaining time, the visible
enemy exposes active status and remaining time but no destination, and the
ghost exposes no live charge fields

### AC: bad-configuration-fails-before-the-match-not-during-it

**Given** a script configuration that violates one of its own declared
parameter ranges
**When** a match is started with it
**Then** it fails immediately, naming the offending value, and no match
is created — never a failure discovered mid-match

### AC: a-configuration-is-never-run-as-code

**Given** a parameter value that is itself valid script source text
**When** a decision runs with it
**Then** the script receives it as an inert value, and it is never
compiled, executed, or treated as anything but data

### AC: memory-out-of-range-is-a-script-error-not-a-silent-wrap

**Given** a script that returns a memory value too large to be a whole
number in range
**When** the decision runs
**Then** it is rejected as a script error, the bot's previously stored
memory is left untouched, and nothing is silently rounded or wrapped

### AC: the-same-position-always-reads-the-same-way

**Given** the identical position handed to a script one thousand times
**When** every list inside the observation is inspected each time
**Then** every one of them comes back in the identical order, every time

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
