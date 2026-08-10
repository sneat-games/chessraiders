---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Pre-Planned Attacks

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/real-time-command/pre-planned-attacks?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/real-time-command/pre-planned-attacks?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/real-time-command/pre-planned-attacks?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/real-time-command/pre-planned-attacks?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

Chaining a further step onto a piece that is already charging works the same
way whether that step is an ordinary move or an attack: appending onto a
live enemy square asks for the Kill/Capture/Recruit choice right away, at
the moment you plan it, and the chosen outcome travels with that step to the
back of the queue rather than starting a brand-new one-step route.

## Problem

Queuing several moves ahead — tap a reachable square while a piece is
already charging, and it joins the end of the route — is one of the more
natural things to do in [Real-Time Command](../README.md). It should work
exactly the same when the appended square happens to hold an enemy: pick
Kill, Capture, or Recruit for it now, and let the rest of the plan stand.
Without this, planning an attack several steps ahead is a trap rather than a
convenience.

## Behavior

### Planning an attack while chaining moves

#### REQ: append-attack-asks-choice-immediately

Appending a further step onto a piece that is already charging, where the
new step's destination holds a live enemy piece the match's rules require a
Kill/Capture/Recruit decision for, opens that same choice immediately — at
the moment of the append, not deferred until the step becomes current. A
queued step that still needs a decision when it is finally reached has
nowhere left to ask it, since nothing else in the loop pauses for a player's
answer at that instant.

#### REQ: queued-choice-preserves-existing-route

Making that choice adds the attack to the **end** of the existing route
rather than replacing it: every step already queued stays queued, the
piece's live charge is untouched, and the new step carries the outcome you
chose. Choosing Kill, Capture, or Recruit for a planned attack never
discards the steps queued ahead of it.

#### REQ: chain-continues-past-planned-attack

Once the planned attack is queued, further taps keep appending from the end
of the route exactly as they would for any other pre-move — another
ordinary move, or another attack with its own choice, up to the match's
route-length limit.

### When the planned attack becomes current

#### REQ: same-target-resolves-planned-choice

When the route reaches a step that was queued with a chosen outcome, and the
exact enemy piece the append targeted is still standing on that square, the
step charges as an attack and resolves with the outcome chosen when it was
planned — the player is not asked again.

#### REQ: mismatched-target-cancels-planned-attack

If the destination no longer holds that exact enemy piece — it escaped, was
captured, or a different piece now stands there, ally or enemy — the queued
step and the complete remaining route are cancelled, per
[Real-Time Command](../README.md)'s `changed-target-cancels-route`. A
pre-chosen outcome is never silently applied against a piece the player did
not choose it for, and the step never quietly resolves as an ordinary
advance into the square.

### Promotion shares the same fix

#### REQ: promotion-append-shares-the-fix

An appended step that reaches the promotion rank works the same way: the
rank chosen in the promotion prompt is added as an appended step, not a
replacement, so it no longer discards a queued route.

### Plan-time odds stay advisory

#### REQ: plan-time-odds-stay-advisory

Any success percentage shown while planning an appended attack is local and
advisory only. It is never stored against the queued step and never treated
as a guarantee: when the step actually charges and resolves, the real odds
are recalculated fresh from both pieces' state at that moment, per
[Capture Outcomes](../../capture-outcomes/README.md), and may differ from
what was shown while the step was only being planned.

## Dependencies

- [Real-Time Command](../README.md)
- [Capture Outcomes](../../capture-outcomes/README.md)

## Acceptance Criteria

### AC: append-attack-asks-choice-immediately

**Given** a piece is already charging toward one square with nothing queued
behind it, and a further reachable square holds an enemy piece
**When** the player taps that further square
**Then** the Kill/Capture/Recruit choice opens immediately, before the
piece's current charge resolves, and the route is not yet extended.

### AC: queued-choice-preserves-existing-route

**Given** a piece already has one step queued and the player has just chosen
an outcome for a newly appended attack
**When** the choice is submitted
**Then** the route holds both steps in order — the first step unaffected,
the attack queued behind it carrying the chosen outcome — and neither step
is discarded or restarted.

### AC: chain-continues-past-planned-attack

**Given** a route now ends with a planned attack and its chosen outcome
**When** the player taps a square legally reachable from that planned
attack's destination
**Then** it is appended as the next route step, exactly like any other
pre-move.

### AC: same-target-resolves-planned-choice

**Given** a queued attack step was chosen as Capture and the exact same
enemy piece is still on its destination square when the route reaches that
step
**When** the step begins charging
**Then** it charges as a Capture attempt against that piece and resolves
without asking the player again.

### AC: mismatched-target-cancels-planned-attack

**Given** a queued attack step targeted a specific enemy piece that has
since moved off that square, been captured, or been replaced by a different
piece by the time the route reaches that step
**When** the step is reached
**Then** the pending step and the complete remaining route are cancelled,
the piece stays on its last completed square, and the stored choice is
discarded rather than applied to whatever now occupies — or no longer
occupies — the destination.

### AC: promotion-append-shares-the-fix

**Given** a pawn is charging with a route already queued and the next
reachable square is on the promotion rank
**When** the player appends that square and chooses a promotion
**Then** the promotion is submitted as an appended step and the pawn's
existing queued route is preserved.

### AC: plan-time-odds-stay-advisory

**Given** a player sees a locally computed success percentage while planning
an appended attack
**When** the queued step later resolves at its actual charge completion
**Then** the real odds are recalculated fresh from both pieces' state at
that moment, the previewed percentage is not stored or replayed anywhere,
and the resolved outcome may differ from what was shown while planning.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
