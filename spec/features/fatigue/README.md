---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Fatigue

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fatigue?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fatigue?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fatigue?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fatigue?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

Fighting takes a toll and long marches are exhausting. Every piece carries a
fatigue level that rises when it fights or marches and drains back down when
it rests. Fatigue never removes a piece or blocks a move by itself. It shifts
the odds the moment that piece is voluntarily involved in a risky
[Capture or Recruit](../capture-outcomes/README.md) attempt, and — as of the
**balance-v7** profile — it also slows down that piece's own next charge:
tired soldiers are slow, and that now includes the king.

## Problem

A raiding army that fights and marches without consequence has no reason to
pace itself. Fatigue gives every push forward a cost that is visible on the
piece itself, rewards a defender who has already survived a fight, and
rewards an attacker who strikes with fresh troops rather than a piece that
has been marching and fighting nonstop.

## Behavior

### Where fatigue comes from

#### REQ: battle-fatigue

Every resolved physical clash — a Kill, a deterministic Capture, an explicit
Capture or Recruit attempt, a forced king capture, an en-passant capture, or
a convoy interception in which attacker and defender actually meet — adds a
10-second fatigue stack to every participant that survives the clash as an
active piece. A contact that never actually happens (the target escaped
first, the command was illegal, or the move was cancelled) adds no fatigue to
anyone.

#### REQ: movement-fatigue

Every successful charged relocation adds its own fatigue stack lasting that
move's charge duration plus one second. A move that is rejected, cancelled,
or otherwise fails to relocate the piece adds none. Because an older stack
keeps draining while the next step charges, an uninterrupted queued route
with unchanged step timing adds only one net second of fatigue per additional
completed step — a steady march costs far less than the raw numbers suggest.

Battle fatigue and movement fatigue are independent and both cumulative: a
piece that fights and then successfully advances in the same action can
collect both.

### How fatigue behaves over time

#### REQ: fatigue-decay

Each fatigue stack counts for one point per second of remaining duration and
contributes nothing once it expires: a fresh battle stack is worth 10 points
and drains to zero exactly 10 seconds after the clash; a movement stack
drains to zero exactly one second after the charge duration that created it
would have ended. A piece's total fatigue is simply the sum of every stack
that has not yet expired. Stacks never overwrite or refresh one another —
they simply pile up and drain independently.

### What fatigue affects

#### REQ: fatigue-shapes-capture-risk

Fatigue shifts the odds of an explicit
[Capture or Recruit](../capture-outcomes/README.md) attempt. A tired
defender is easier to take alive; a tired attacker is less able to secure a
prisoner. Equal fatigue on both sides cancels out. Kill, king capture, and
convoy interception are never affected by fatigue as an input to a roll —
Kill stays certain by default regardless of either side's fatigue (see
[Capture Outcomes](../capture-outcomes/README.md) for the opt-in
Probabilistic Kill setting, which rolls but still ignores fatigue).

#### REQ: fatigue-slows-charging

Fatigue also has a second, universal effect, introduced in the
**balance-v7** profile: every point of a piece's current fatigue adds 0.25
seconds to that piece's next charge — move or attack — up to a cap of +2
seconds of added charge time regardless of how much fatigue the piece is
carrying. This applies to every piece without exception, including the
king: tired soldiers are slow. The penalty is computed from the piece's
fatigue at the moment the new charge starts and is fixed for that charge;
later fatigue changes affect only the charge after it.

#### REQ: fatigue-visibility

Fatigue is shown as a slim vertical bar bound to the piece, on the side
opposite its charge indicator, using a darker red/amber palette that drains
downward as fatigue expires. It stays visually secondary to the brighter
charge indicator — this is a status light, not a second alert. Your own team
always sees your pieces' fatigue. The opposing team sees a piece's fatigue
bar only while that piece is currently visible to them; a stale "ghost"
memory of a piece under [Fog of War](../fog-of-war/README.md) never shows
fatigue or its decay.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)

## Acceptance Criteria

### AC: battle-adds-a-ten-second-stack

**Given** two pieces resolve any kind of physical clash and both survive as
active pieces
**When** the clash resolves
**Then** each survivor gains a 10-second fatigue stack that is worth 10
points immediately and reaches zero exactly 10 seconds later.

### AC: no-clash-no-fatigue

**Given** an attacker's charge was aimed at a piece
**When** that piece escapes before the charge completes and the attacker
simply advances into an empty square
**Then** neither piece gains any fatigue from the encounter.

### AC: successive-moves-add-one-net-second

**Given** a piece with no other fatigue makes two consecutive moves, each
with an unchanged 2.9-second charge and no gap between them
**When** both moves resolve successfully
**Then** the piece ends up with exactly one more fatigue point after the
second move than it had immediately after the first, because the first
stack keeps draining while the second charges.

### AC: fatigue-shifts-only-risky-actions

**Given** one attacker is fresh and an otherwise identical attacker carries a
fresh 10-second fatigue stack
**When** each attempts an explicit Capture against an identical, equally
fatigued defender
**Then** the fresh attacker's estimated success is higher than the fatigued
attacker's, while a Kill attempt by either attacker is unaffected because
Kill never uses fatigue.

### AC: fatigue-adds-quarter-second-per-point

**Given** a piece carries 4 fatigue points when it begins a new charge
**When** that charge starts
**Then** the charge takes 1 additional second to complete (4 × 0.25s), on
top of the piece's ordinary move or attack charge time for that order.

### AC: fatigue-charge-penalty-is-capped

**Given** a piece carries 12 fatigue points, which would compute to a 3
second penalty
**When** it begins a new charge
**Then** the added charge time is capped at 2 seconds, not 3, and this cap
applies to every piece including the king.

### AC: fatigue-bar-follows-visibility

**Given** a team's piece carries visible fatigue and the opposing team cannot
currently see that piece
**When** the board is shown to both teams
**Then** the owning team sees the darker fatigue bar draining, the opponent
sees none of it, and the opponent gains the bar only once that piece becomes
visible to them again — never as a stale "ghost" value.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
