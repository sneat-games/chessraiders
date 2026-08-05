---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Fatigue

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fatigue?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fatigue?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fatigue?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fatigue?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

Fighting takes a toll and long marches are exhausting. As of the
**balance-v7** profile, every piece carries a single fatigue meter, measured
in seconds and capped at 10 — the fatigue bar always reads as a percentage
of that cap. The meter rises when a piece moves or fights and only drains
back down while that piece stands completely idle. Fatigue never removes a
piece outright. It shifts the odds the moment that piece is voluntarily
involved in a risky [Capture or Recruit](../capture-outcomes/README.md)
attempt, it slows down that piece's own next charge, and it holds every
attack back until the piece is fully rested: tired soldiers are slow, none
of them — not even the king — fight until they have caught their breath, and
none of them catch their breath while still moving.

## Problem

A raiding army that fights and marches without consequence has no reason to
pace itself. Fatigue gives every push forward a cost that is visible on the
piece itself, rewards a defender who has already survived a fight, and
rewards an attacker who strikes with fresh troops rather than a piece that
has been marching and fighting nonstop.

## Behavior

### Where fatigue comes from

#### REQ: movement-fatigue

Every completed move adds a flat, per-rank configurable cost to the piece's
fatigue meter — **1 second by default for every rank** in the balance-v7
profile, 10% of the meter's cap. The cost is charged once per accepted
command (one route step), never per square crossed: a rook sliding five
squares in a single order costs the same 1 second as a king's single-square
step. A move that is rejected, cancelled, or otherwise fails to relocate the
piece adds none.

#### REQ: battle-fatigue

Every resolved physical clash — a Kill, a deterministic Capture, an explicit
Capture or Recruit attempt, a forced king capture, an en-passant capture, or
a convoy interception in which attacker and defender actually meet — adds a
flat **2 seconds** (20% of the meter's cap) to the fatigue meter of every
participant that survives the clash as an active piece. A contact that never
actually happens (the target escaped first, the command was illegal, or the
move was cancelled) adds no fatigue to anyone.

Movement fatigue and battle fatigue are not separate pools — both add to the
same single per-piece meter, and a piece that fights and then successfully
advances in the same action simply adds both amounts onto it.

### How fatigue behaves over time

#### REQ: fatigue-decay

Fatigue is a single meter per piece, not a set of independently expiring
stacks. It rises by the amounts above, up to a hard cap of **10 seconds** —
the fatigue bar always reads as a percentage of that cap, so a single move
fills 10% of it and a single fight fills 20%. The meter drains at 1 second
per second, but only while the piece has no live charge: a piece that is
currently moving or attacking recovers nothing at all, however long that
charge takes. "You don't catch your breath while sprinting" — ten move
commands issued back to back with no pause fill the meter straight to its
10-second cap, and working that back down again costs a full ten seconds of
standing completely still, exactly the seconds actually spent moving.

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
**balance-v7** profile: every second currently on a piece's fatigue meter
adds 0.25 seconds to that piece's next charge — move or attack — up to a cap
of +2 seconds of added charge time regardless of how high the meter climbs.
This applies to every piece without exception, including the king: tired
soldiers are slow. The penalty is computed from the piece's fatigue at the
moment the new charge starts and is fixed for that charge; later fatigue
changes affect only the charge after it.

#### REQ: rest-then-fight

Also new in the **balance-v7** profile: a piece may begin charging any
attack — Kill included — only while fully rested, at zero fatigue. A piece
ordered to attack while it still carries any fatigue does not refuse the
order: it visibly rests in place, and the instant its meter drains to zero
the attack charge starts on its own, with no further command needed. The
wait is deterministic — the meter only drains while the piece is idle, at a
fixed rate, so the exact instant a queued attack will start charging is
already knowable the moment the order is accepted. Because a piece that has
just finished a move is still carrying its own fresh movement fatigue and
has had no idle time to shed it, this also means no piece — the king
included — can strike in the same breath it arrives beside a target: it
pauses first, and that pause is the defender's reaction window. Two actions
ignore this gate entirely and are always available regardless of fatigue:
capturing the enemy king and intercepting a loaded enemy convoy.

#### REQ: fatigue-visibility

Fatigue is shown as a slim vertical bar bound to the piece, on the side
opposite its charge indicator, using a darker red/amber palette that drains
downward only while the piece is resting. It stays visually secondary to the
brighter charge indicator — this is a status light, not a second alert. Your
own team always sees your pieces' fatigue. The opposing team sees a piece's
fatigue bar only while that piece is currently visible to them; a stale
"ghost" memory of a piece under [Fog of War](../fog-of-war/README.md) never
shows fatigue or its decay.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)

## Acceptance Criteria

### AC: move-cost-is-per-command-not-per-square

**Given** a rook slides five squares to complete a single move command, and
a king separately completes an ordinary single-square move command
**When** each move resolves
**Then** both pieces' fatigue meters rise by exactly the same 1 second,
because the cost is charged once per command, never per square crossed.

### AC: battle-adds-two-seconds

**Given** two pieces resolve any kind of physical clash and both survive as
active pieces
**When** the clash resolves
**Then** each survivor's fatigue meter rises by exactly 2 seconds.

### AC: no-clash-no-fatigue

**Given** an attacker's charge was aimed at a piece
**When** that piece escapes before the charge completes and the attacker
simply advances into an empty square
**Then** neither piece gains any fatigue from the encounter.

### AC: fatigue-only-drains-while-idle

**Given** a piece is carrying fatigue and is immediately given a new move or
attack command with no pause
**When** that new charge is in progress
**Then** the piece's fatigue meter does not decrease at all while the charge
runs, however long the charge takes.

### AC: sprint-costs-real-rest-time

**Given** a piece with no starting fatigue completes ten consecutive move
commands back to back with no pause between them, each adding 1 second
**When** the tenth move resolves
**Then** the piece's meter sits at its 10-second cap, and working it back
down to zero costs a full ten seconds of the piece standing completely
idle — not the single longest-step interval a set of independently expiring
stacks would have allowed.

### AC: fatigue-caps-at-ten-seconds

**Given** a piece's fatigue meter is already at its 10-second cap
**When** it would gain more fatigue from a further move or fight
**Then** the meter stays at 10 seconds; it never rises above the cap.

### AC: fatigue-shifts-only-risky-actions

**Given** one attacker is fresh and an otherwise identical attacker carries 2
seconds of fatigue from a recent fight
**When** each attempts an explicit Capture against an identical, equally
fatigued defender
**Then** the fresh attacker's estimated success is higher than the fatigued
attacker's, while a Kill attempt by either attacker is unaffected because
Kill never uses fatigue.

### AC: fatigue-adds-quarter-second-per-second

**Given** a piece carries 4 seconds of fatigue on its meter when it begins a
new charge
**When** that charge starts
**Then** the charge takes 1 additional second to complete (4 × 0.25s), on
top of the piece's ordinary move or attack charge time for that order.

### AC: fatigue-charge-penalty-is-capped

**Given** a piece carries its full 10-second meter, which would compute to a
2.5 second penalty
**When** it begins a new charge
**Then** the added charge time is capped at 2 seconds, not 2.5, and this cap
applies to every piece including the king.

### AC: attack-waits-for-full-rest

**Given** a piece carries 3 seconds of fatigue
**When** it is ordered to attack an enemy piece, including a plain Kill
**Then** the attack does not begin charging immediately; the piece visibly
rests in place until its meter drains to zero, and only then does the
attack charge start.

### AC: king-capture-and-interception-ignore-the-rest-gate

**Given** an attacker carries fatigue, including the meter's 10-second cap
**When** that attacker captures the enemy king, or intercepts a loaded
enemy convoy
**Then** the action proceeds immediately, without waiting for the attacker
to rest.

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
