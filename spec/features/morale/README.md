---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Morale

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/morale?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/morale?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/morale?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/morale?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

Your morale is never a resource you spend or bank — it is simply how far
your king has advanced, read straight off the board. It decides which of
your pieces are allowed to take prisoners, which enemy pieces can be taken
prisoner, and how many captives and informers your side can manage at once.
Push your king forward and your whole army fights with more authority;
retreat it, or let the enemy king stand on your own ground, and your
authority shrinks — immediately, and at a cost.

## Problem

Chess Raiders wanted a "command" resource that could not be hoarded,
gambled, or forgotten about between fights, and that made the king's own
position matter without introducing check or checkmate. Deriving morale
directly from the king's rank does that: it cannot be spent to zero by
accident, it cannot be stockpiled by turtling, and leading from the front is
rewarded exactly as directly as hiding is punished.

## Behavior

### What morale is

#### REQ: morale-derivation

Each team's morale is derived entirely from the current board — never
stored, never spendable — primarily from that team's own king's current
rank: White rank 1 gives morale 0 up to rank 8 giving morale 7, mirrored for
Black. A king that is escorting a convoy still stands on the board and still
counts normally. A king held as cargo inside any convoy gives its team
morale 0.

**King incursion (amended 2026-07-31):** while an active enemy king stands
on your own half of the board — across the rank 4–5 border — your morale is
additionally reduced by 1 (never below 0). An enemy king riding as cargo
inside a convoy does not trigger this; it only applies to an active king
that is genuinely standing (or escorting) on your side of the line. The
moment an enemy king crosses into your half, or leaves it, that crossing is
announced to every player on both teams — the crossing itself is public even
under [Fog of War](../fog-of-war/README.md); only the king's exact square
stays hidden as usual.

### What morale gates

#### REQ: morale-capture-gates

Taking a prisoner requires your morale to reach two thresholds from the same
table: the attacking piece's own rank, and the target piece's rank.

| Piece | Morale required |
|---|---:|
| Pawn | 1 |
| King | 2 |
| Knight | 3 |
| Bishop | 3 |
| Rook | 4 |
| Queen | 5 |

Kills are never gated by morale. Where Kill is not offered as an outcome, a
capture that fails this gate is simply rejected — never accepted and then
punished. Because the enemy king is always taken prisoner and never killed
(see [Prisoner Convoys](../prisoner-convoys/README.md)), capturing it always
requires morale at least 2, using the attacker's rank before any promotion.

#### REQ: morale-capacity

The number of captive pawns plus active informers your side is holding
across your whole army must never exceed your current morale — captives and
informers share one capacity pool. See
[Prisoner Convoys](../prisoner-convoys/README.md) for how captives are
held and [Espionage](../espionage/README.md) for informers. The captured
enemy king never counts against this cap, and a captive pawn that has
already been unloaded back into the ranks never counts either. A capture or
recruitment that would break the cap is rejected up front, with a clear
"your army cannot manage more prisoners" explanation.

#### REQ: morale-overflow

Whenever your morale drops below what you are currently holding — because
your king retreated, was captured, or an enemy king just crossed into your
territory — the overflow is resolved immediately and deterministically, with
no randomness involved:

1. Active informers are released first (see
   [Espionage](../espionage/README.md) for their own release order).
2. Once informers can no longer help, prisoners are executed: the largest
   convoy first, then the convoy farthest from your own back rank, then the
   oldest convoy, with a fixed tie-break as a last resort. Within a convoy,
   the newest prisoner is executed first — the opposite order from
   unloading, which is first-in-first-out.
3. A convoy that loses its very last prisoner this way instantly becomes a
   plain escort piece in refitting state on its square — see
   [Prisoner Convoys](../prisoner-convoys/README.md).

#### REQ: morale-interception-exempt

Intercepting an enemy convoy is never gated by morale, even at morale 0 —
this is deliberately how a team with no morale at all can still rescue its
own captured king by hitting the convoy carrying it. The cargo gained still
counts against your capacity immediately afterward, so a desperate rescue at
low morale often costs you a prisoner the instant it lands, through the
overflow rule above.

### Where morale shows up elsewhere

Morale is the shared foundation under three other systems: it limits how
many pawns can be trained as specialists (see
[Pawn Specialisation](../pawn-specialisation/README.md)), it scales how much
the [Royal Beacon](../royal-beacon/README.md) speeds up nearby orders, and it
sets the informer sabotage delay described in
[Espionage](../espionage/README.md). Every one of those consumers reads the
same morale value computed here — there is no separate morale for each
system.

#### REQ: morale-is-public

Both teams' morale values are always visible to both teams, in every
[Fog of War](../fog-of-war/README.md) variant. The enemy king itself may be
hidden, but his army's command posture never is.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)

## Acceptance Criteria

### AC: king-rank-sets-morale

**Given** White's king stands on rank 4
**When** morale is read for White
**Then** it is exactly 3, and if the king instead stood on rank 1 it would be
0, or on rank 8 it would be 7.

### AC: king-incursion-drops-morale-and-announces

**Given** the incursion rule is enabled, White is at morale 3 holding two
captives, and the Black king stands one square short of the rank 4–5 border
**When** the Black king crosses onto White's half of the board
**Then** White's morale drops by 1 with overflow resolved per the
deterministic order, both teams receive a public announcement of the
crossing, and when the Black king later leaves White's half a second
announcement fires and the reduction ends.

### AC: morale-gates-who-can-capture

**Given** a team's morale is 2
**When** a knight on that team (needing morale 3) attempts to capture, and
separately a pawn on that team (needing morale 1) attempts to capture
**Then** the knight's capture is rejected with an explanation naming the
morale requirement, while the pawn's capture succeeds.

### AC: capacity-caps-captives-and-informers-together

**Given** a team at morale 4 already holds three captive pawns and one active
informer
**When** that team attempts one more capture
**Then** the capture is rejected before it happens, because captives and
informers already fill the team's capacity.

### AC: retreat-executes-overflow-deterministically

**Given** a team holds three captive pawns across two convoys at morale 3
**When** that team's king retreats one rank, dropping morale to 2
**Then** exactly one prisoner is executed — the newest prisoner in the
largest convoy — and if that execution empties a convoy, it instantly
becomes a refitting escort.

### AC: zero-morale-can-still-rescue-the-king

**Given** a team's own king is held captive in an enemy convoy, leaving that
team at morale 0
**When** a piece from that team intercepts the convoy carrying their king
**Then** the interception succeeds regardless of morale, and any overflow
created by the newly gained cargo is resolved immediately afterward.

### AC: morale-is-always-visible

**Given** a match is running under Shared Fog of War
**When** either player checks the board
**Then** both teams' current morale values are shown, even though enemy
piece positions may be hidden.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
