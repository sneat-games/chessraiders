---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Veteran Progression

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation/veteran-progression?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation/veteran-progression?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation/veteran-progression?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation/veteran-progression?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

A piece that captures an enemy, escorts the prisoner home and unloads it becomes a Veteran for life. Only Veterans can be trained as Engineers or Sergeants, and only a Veteran Engineer can build in stone.

The ladder is four rungs, and every step up it is bought with the same act —
bringing an enemy home alive:

**Pawn → Veteran Pawn → Engineer → Veteran Engineer**

Engineer Qualification II and Advanced Engineer Training no longer exist. The
second prisoner mission that used to earn Qualification II now makes an
Engineer a Veteran Engineer, and stone is what a Veteran Engineer builds.

## Problem

Capturing is the bravest thing you can do in Chess Raiders. You have to reach
the enemy, take them alive, then walk a slow, loaded convoy all the way home
while everyone on the board knows exactly where it is going. Until now that
paid you in material and morale, and nothing else.

Training paid better and cost nothing. The pawn that became an Engineer was
usually the pawn that had never left rank 2 — the safest soldier in the army,
promoted for standing still, while the one who fought its way to the enemy line
and dragged a prisoner back had no career at all.

Meanwhile the Engineer's own ladder had a rung with a clumsy name and a second
trip to the training ground attached. The thing that rung was really trying to
say — *this one has been out there and brought someone back* — is worth
keeping. "Qualification II" was not.

## Behavior

### Becoming a Veteran

#### REQ: veteran-earned-by-bringing-a-prisoner-home

A piece becomes a **Veteran** the moment it unloads, at its own side's pawn
base rank, a prisoner it captured itself. There is one way to earn it and no
other: you must make the capture, escort the prisoner home yourself, and
unload it yourself.

Escorting a prisoner someone else captured does not count. Standing nearby
does not count. A teammate finishing the job for you does not count. Killing
an enemy never counts — a kill leaves no prisoner to bring home.

Any piece can be a Veteran, not only a pawn: a rook that takes a prisoner and
walks it home has earned it exactly as a pawn would.

Delivering the captured **king** does not make you a Veteran, because
delivering the king wins the match.

#### REQ: veteran-is-for-life

Once earned, Veteran status is never taken away while that piece is on the
board. It survives escort duty, refitting, a morale collapse, training, and
even promotion — a Veteran pawn that promotes to a queen is a Veteran queen.
It is a service record, not a trade, so unlike a profession it is not traded
away for the new rank.

Only capture ends it, and it ends it completely: a captured piece becomes an
ordinary prisoner like any other, and if it is ever unloaded back into play it
comes home an ordinary, untrained, non-Veteran pawn — on its captor's side.
Nothing about who it used to be survives being taken.

#### REQ: veteran-in-your-trade

A specialist who brings a prisoner home *after* qualifying becomes a **Veteran
Engineer** or a **Veteran Sergeant** — the same mission again, done with a
trade in hand.

This is a second, separate standing from being a Veteran pawn. Completing the
mission before you trained does not count towards it: the deliveries that made
you eligible to train are spent on that, and the trade starts your record over.
Like Veteran status itself, it is earned once and kept for life.

#### REQ: a-veteran-is-visible

A Veteran is visibly marked on the board, distinguishable at a glance from an
ordinary piece of the same rank, and a Veteran Engineer or Veteran Sergeant is
distinguishable from an ordinary one of the same profession.

The mark is **public** — both sides see it. Your opponent watched the convoy
come home; there is nothing to hide, and a reward nobody can see is not a
reward.

### What a Veteran can do

#### REQ: only-veterans-can-train

Only a Veteran may be trained as a specialist. Every other condition on
training stays exactly as
[Pawn Specialisation](../README.md) already states it — an active, ordinary,
untrained pawn, standing on its own side's pawn base rank, within its side's
morale limit, choosing its profession explicitly, channelled for the usual 10
seconds. This adds one requirement to that list and removes none.

A pawn that has never brought a prisoner home is refused, and the refusal says
what to go and do about it rather than simply saying no.

#### REQ: stone-is-a-veteran-engineers-work

Building a **stone** wall requires a Veteran Engineer: an Engineer that has
brought at least one prisoner home *since* it qualified as an Engineer. A
newly trained Engineer builds in wood, and earns stone the same way it earned
the right to train in the first place.

A Veteran Engineer also works faster than an ordinary one — construction,
dismantling and repair — exactly the speed advantage the old Qualification II
carried. See [Fortifications](../fortifications/README.md).

#### REQ: no-more-qualifications

There are no numbered Engineer qualifications and no Advanced Engineer
Training. An Engineer is either a Veteran Engineer or it is not, and that is
decided in the field rather than at the training ground. There is no third
rung above it and no further wall material.

Matches that were already under way when this rule arrived keep the rules they
started with, and finish under them.

### Naming

#### REQ: veteran-names-the-progression-not-the-bot

"Veteran" names this progression. The single-player bot difficulty tier that
used to carry the name is renamed **Lieutenant**, and now reads Recruit →
Lieutenant → Commander. The tier itself — how it plays, how hard it is — does
not change, and links or saved matches that already chose it keep working.

## Dependencies

- [Pawn Specialisation](../README.md)
- [Prisoner Convoys](../prisoner-convoys/README.md)
- [Fortifications](../fortifications/README.md)
- [Capture Outcomes](../capture-outcomes/README.md)
- [Morale](../morale/README.md)

## Acceptance Criteria

### AC: bringing-a-prisoner-home-earns-veteran

**Given** an ordinary pawn that captures an enemy piece and becomes its escort
**When** it walks the convoy back to its own pawn base rank and unloads the
prisoner there
**Then** that pawn is a Veteran, both players can see it, and a pawn that spent
the match moving without ever taking a prisoner is not.

### AC: escorting-someone-elses-prisoner-earns-nothing

**Given** a convoy carrying only a prisoner that a different piece captured
**When** it unloads that prisoner at its own pawn base rank
**Then** the escort is not a Veteran.

### AC: taking-a-loaded-convoy-credits-only-the-escort-you-beat

**Given** a piece that attacks an enemy convoy already carrying two prisoners,
taking its escort prisoner and inheriting its cargo
**When** it brings all three home and unloads them
**Then** it becomes a Veteran for the escort it beat, and the two inherited
prisoners earn it nothing.

### AC: a-veteran-queen-is-still-a-veteran

**Given** a Veteran pawn one square from the last rank
**When** it promotes
**Then** the new queen is still a Veteran; **and given** a different Veteran is
captured and later unloaded by its captor
**Then** it comes back an ordinary, untrained, non-Veteran pawn on its captor's
side.

### AC: an-untried-pawn-cannot-train

**Given** an ordinary pawn on its own pawn base rank, with its side well inside
its morale limit
**When** its player tries to train it as an Engineer
**Then** the attempt is refused and the refusal says a prisoner must be brought
home first; **and when** that same pawn later captures an enemy, escorts it
home and unloads it
**Then** the same training command succeeds.

### AC: a-new-engineer-builds-in-wood-not-stone

**Given** a Veteran pawn that has just completed Engineer training
**When** it tries to build a stone wall
**Then** the attempt is refused; **and when** it captures a second enemy,
escorts it home and unloads it
**Then** it is a Veteran Engineer and the same stone wall can be built.

### AC: advanced-engineer-training-is-gone

**Given** an Engineer standing on its own pawn base rank
**When** its player looks for a way to advance it
**Then** no advanced training exists on any surface, and the rules describe no
Engineer qualification levels at all.

## Open Questions

- **Does the morale limit on training still earn its keep?** A side is now
  limited both by its morale and by how many prisoners it has actually brought
  home. Two limits on the same action may make specialists rarer than intended.
  Unchanged by this Feature; flagged for the next balance review.

---
*This document follows the https://specscore.md/feature-specification*
