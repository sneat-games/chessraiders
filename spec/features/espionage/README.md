---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Espionage

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/espionage?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/espionage?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/espionage?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/espionage?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

In Conquest, an attack can Recruit Informer instead of Kill or Capture: the
target stays exactly where it is, under its own side's control, looking
completely unchanged — but it secretly shares its battlefield vision with
the recruiting side, and secretly slows down its own side's plans. Only the
recruiting team ever knows which enemy piece it turned. The only way to
expose one is to catch it: a king can interrogate an adjacent piece of its
own side to find out.

## Problem

[Fog of War](../fog-of-war/README.md) creates missing information, but
without Espionage there is no way to fight over that missing information —
no way to buy a look behind the fog, and no way to suspect and root out a
leak. Espionage adds bluffing and counterplay around hidden information
using the same [Morale](../morale/README.md) capacity, the same fatigue and
capture-risk model, and the same board you already see — without a separate
currency or a randomised "discovery" roll that would make the information
war itself unreliable.

## Behavior

### Recruiting an informer

#### REQ: recruit-non-displacing

Recruit Informer is a covert, non-displacing outcome of an attack, available
wherever Kill or Capture is offered in Conquest (see
[Capture Outcomes](../capture-outcomes/README.md) for the shared risk-roll
model — Recruit uses the exact same fatigue-adjusted success estimate,
shown as `Recruit Informer - 73%`). On success, the attacker stays on its
own square, and the target stays on its own square, under its own side's
control, unchanged in appearance — nothing about the board looks different.
An enemy king can never be recruited: attacking a king is always Capture in
every mode.

#### REQ: informer-vision

The recruiting side continuously receives the same [Fog of War](../fog-of-war/README.md)
vision that the informer itself generates, following the informer as it
moves. This is sight only: the recruiter cannot move it, attack with it, or
otherwise direct it — it is still entirely the other side's piece. Informer
vision can reveal a piece that has already used up its
[opening deployment reveal](../fog-of-war/README.md), but it never restores
that one-time exception.

An active informer also reveals live charge progress — for itself, and for
any active piece currently inside the patrol-vision footprint the informer
generates — even in a Classic match where the board would otherwise be
fully visible anyway. This never reveals the target square, the route, or
which piece is attacking which; it only shows that a piece is charging and
how far along it is.

#### REQ: informer-secrecy

Only the recruiting side can identify which enemy pieces it has recruited,
and only the recruiting side gets the option to manually release one. The
informer's own side, and any spectator, gets no marker, no different
appearance, and no notification of the recruitment or of a quiet release.
Only a successful interrogation (below) exposes one informer's identity, and
only to the side running that interrogation.

#### REQ: public-command-totals

Both sides always see current morale, current captive-pawn counts, and
current active-informer counts for both White and Black, including a count
of zero — but never which specific enemy pieces are informers.

### Capacity and release

#### REQ: shared-morale-capacity

Captive pawns and active informers share one capacity pool, capped by the
recruiting side's current [morale](../morale/README.md): a side at morale 5
can hold any mix of captives and informers that adds up to at most five. A
new recruitment that would break the cap is rejected, exactly like a capture
that would.

#### REQ: informer-first-overflow

Whenever morale drops below what a side is currently holding, informers are
released before prisoners are executed: the strongest-ranked informer first,
the oldest among equally-ranked informers next, with only remaining overflow
falling through to the prisoner-execution order in
[Morale](../morale/README.md).

#### REQ: manual-release

A side may release any informer it has recruited at any time. Release is
immediate — it consumes only the acting player's ordinary command interval,
never a piece's move charge — and it stops that piece's vision-sharing and
sabotage right away, frees one capacity slot, and reveals nothing to the
informer's own side.

### Sabotage

#### REQ: sabotage-delay

**Universal start-delay rule (founder, 2026-07-31):** every active informer
secretly slows down its own side. The delay is
`recruiting side's current morale × 100 ms` — 100 ms at morale 1, 300 ms at
morale 3, 500 ms at morale 5, 700 ms at morale 7 — fixed at the moment the
action is accepted, so a morale change only affects the informer's *next*
action, never one already under way.

This delay applies the same way to every kind of action an informer takes:
for an ordinary move or attack, it is simply added to that piece's move
charge (after any [Royal Beacon](../royal-beacon/README.md) speed-up), so it
starts charging immediately with no extra hidden wait. For a channelled
action that is not a move at all — starting an interrogation, or, where
[Pawn Specialisation](../pawn-specialisation/README.md) is in play, training
or engineering work — the same delay instead postpones the *start* of that
whole action; once started, its duration and pace run at their normal rate.
Selecting or inspecting a piece is never delayed.

#### REQ: sabotage-has-no-tell

Nothing on the board ever labels a delay as sabotage, shows its length or
cause, or adds a warning icon, suspicion score, or automatic detection. The
only clue available to the informer's own side is what they can observe
directly: that piece is consistently a little slower than it should be.

### Interrogation

#### REQ: king-interrogation

An active king can interrogate one adjacent friendly piece — any of the
eight neighbouring squares. Starting an interrogation never itself reveals
whether the target is an informer. Both sides may run one interrogation at a
time through their own king.

#### REQ: interrogation-channel

Interrogation is a 3-second channel. While it runs, both the king and the
suspect are visibly engaged and cannot complete another action without
cancelling it first — this is public information, so the opposing side can
see the channel and knows exactly how long it has to disrupt it. Only the
interrogating side can cancel it voluntarily.

#### REQ: interrogation-outcome

If the channel completes uninterrupted, a loyal suspect simply produces a
loyal result, known only to the interrogating side. An informer is instead
exposed — its own side and its recruiter both learn the truth, the
recruitment relationship ends immediately (vision and sabotage stop), and
the recruiting side gets its capacity slot back. No other informer's
identity is affected.

#### REQ: interrogation-cancellation

The channel cancels immediately, with no result and no capacity change, the
instant either the king or the suspect completes any other action — moving,
attacking, being captured, or anything else that changes their state. An
illegal or rejected attempt does not cancel it. The opposing side has no
direct way to cancel an interrogation it isn't running; its only lever is to
threaten the immobilised king or suspect and force the interrogating side to
choose between finishing the channel and protecting its pieces.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)
- [Morale](../morale/README.md)
- [Capture Outcomes](../capture-outcomes/README.md)
- [Fog of War](../fog-of-war/README.md)

## Acceptance Criteria

### AC: recruitment-leaves-both-pieces-in-place

**Given** a legal Conquest attack against an enemy piece with Recruit
Informer enabled
**When** the attacker chooses Recruit Informer and it succeeds
**Then** both pieces remain on their original squares, the target stays
under its own side's control and visually unchanged, and the attacker's
charge completes with no extra delay left behind afterward.

### AC: informer-vision-follows-the-piece

**Given** a recruited informer is later moved by its own side
**When** the recruiting side's view is produced
**Then** their vision from that informer updates to its new position, while
the informer's own side and any spectator see no marker identifying it.

### AC: charge-intel-reveals-progress-not-target

**Given** an active informer and another friendly piece inside its patrol
footprint are both charging
**When** the recruiting side views the board
**Then** they see live charge progress for both pieces but no target square,
route, or which-piece-is-attacking-which information.

### AC: capacity-is-shared-and-caps-recruitment

**Given** a side at morale 4 already holds two captive pawns and one active
informer
**When** it attempts to recruit a second informer
**Then** the recruitment is rejected because the shared capacity is already
full.

### AC: overflow-releases-informers-before-executing-prisoners

**Given** a side holds captive pawns and informers together exceeding its
new, lower morale
**When** the overflow resolves
**Then** informers are released first — strongest rank, then oldest — before
any prisoner execution happens for whatever overflow remains.

### AC: manual-release-is-instant-and-silent

**Given** an active informer whose recruiting side chooses to release it
**When** the release happens
**Then** vision-sharing and sabotage stop immediately, one capacity slot is
freed, no move charge is spent, and the informer's own side receives no
identity-revealing signal.

### AC: sabotage-scales-with-current-morale

**Given** the same informer stays active while its recruiting side's morale
changes between separate actions
**When** each action is accepted at morale 1, 3, 5, and 7
**Then** the added delay is respectively 100 ms, 300 ms, 500 ms, and 700 ms,
and changing morale after an action is already accepted never changes that
action's timing.

### AC: sabotage-never-shows-a-tell

**Given** a player selects or inspects a piece that is secretly an informer
**When** the interaction happens
**Then** it responds immediately and shows no sabotage marker, delay length,
or warning of any kind.

### AC: interrogation-reveals-nothing-early

**Given** a king begins interrogating an adjacent friendly piece
**When** the 3-second channel is running
**Then** both pieces are shown publicly as engaged with time remaining, and
no viewer learns whether the suspect is an informer before it completes.

### AC: successful-interrogation-exposes-and-frees-a-slot

**Given** the suspect is genuinely an active informer and neither piece acts
or is removed for the full 3 seconds
**When** the channel completes
**Then** the informer relationship ends, its own side and the recruiter both
learn the truth, and the recruiting side regains one capacity slot.

### AC: any-action-cancels-interrogation

**Given** an interrogation is in progress
**When** either the king or the suspect completes a legal move, attack, or
any other state-changing action
**Then** the interrogation cancels immediately with no result revealed and no
capacity change.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
