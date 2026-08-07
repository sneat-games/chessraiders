---
format: https://specscore.md/feature-specification
status: Implementing
---

# Feature: Pawn Specialisation

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation?op=request-change) |
**Status:** Implementing
**Source Ideas:** —

## Summary

A pawn can earn a trade at your own base rank: Engineer or Sergeant. The
profession is permanent — earned once, worn for life, lost only to capture
or death — and morale limits how many pawns you can train at once, never how
many you keep once trained.

## Contents

| Child | Description |
|---|---|
| [veteran-progression](veteran-progression/README.md) | A pawn that captures an enemy, escorts the prisoner home and unloads it becomes a Veteran for life — a pawn-only status that promotion ends. Only Veterans can be trained as Engineers or Sergeants, a second mission as an Engineer earns Veteran Engineer (faster wooden walls), and a second training earns Master Engineer (stone walls). |

## Problem

Every pawn in Chess Raiders was interchangeable: no way to invest in one,
and no career available to it short of promotion, which removes it from the
infantry entirely. Chess Raiders is becoming a game about commanding an
organised army — leadership, intelligence, logistics, and now training — and
specialisation gives an individual pawn a durable identity, gives morale a
new meaning as a *limit* rather than something you spend, and creates the
professional soldier that battlefield engineering (see
[Fortifications](../fortifications/README.md)) needs to exist.

## Behavior

### Professions in general

#### REQ: one-permanent-profession

A pawn holds no profession or exactly one — never more, and never a
retrain into a different one. The profession is permanent: nothing short of
capture or death takes it away, and it is never lost to a morale drop, the
passage of time, or any lesser event. The two initial professions are
**Engineer** and **Sergeant**; the framework is built to support future
professions without redesign, though none beyond these two exist yet.

#### REQ: capture-strips-the-profession

A captured specialist immediately loses its profession in the same instant
it becomes a neutral captive pawn (see
[Prisoner Convoys](../prisoner-convoys/README.md)) — captives are always
uniform, untrained pawns, so nothing about overflow order, execution order,
or unloading order changes because of who they used to be. A prisoner that
is later unloaded back into play returns as an ordinary, untrained pawn.
Specialist training never transfers to the capturing side in any form.

#### REQ: promotion-strips-the-profession

A profession is pawn-class identity. A specialist that promotes stops being
a pawn and loses its profession in that same move — promotion remains
freely available to a specialist, but choosing it trades the trade for the
new rank.

#### REQ: profession-survives-escort-duty

A specialist that captures an enemy piece becomes a convoy escort exactly
like any other piece, and keeps its profession through the whole escort and
[refitting](../prisoner-convoys/README.md) cycle back to being an active
pawn again — this is the specialist's own continuing service, not a loss
event. The only ways to lose the profession during that cycle are the
ordinary ones: being intercepted (which captures the escort) or dying.
While serving as an escort or refitting, a specialist's profession-specific
actions are unavailable — a Sergeant's aura, however, still applies to
others whenever the Sergeant itself is an ordinary active piece.

#### REQ: informer-orthogonal

Being secretly recruited as an informer (see [Espionage](../espionage/README.md))
never changes which side a piece fights for, so a recruited specialist keeps
its profession and every professional ability for its own original side.
Informer status and profession are completely independent of each other.
The informer sabotage delay is universal across every kind of action a
sabotaged piece takes — see [Espionage](../espionage/README.md) — including
a specialist's own training, wall construction, dismantling, or repair: the
delay always postpones the *start* of the action, never its duration or
pace once it is under way.

### Training

#### REQ: training-at-the-base

Training is available only to an ordinary, active pawn standing on its own
side's pawn base rank — the same rank used for unloading captives (White
rank 2, Black rank 7). The player must explicitly choose the profession
being trained; there is no default. A convoy, a refitting piece, or any
non-pawn cannot train.

*Amended by [Veteran Progression](veteran-progression/README.md) (2026-08-07):
the pawn must also already be a **Veteran** — it must have captured an enemy,
escorted the prisoner home and unloaded it. Everything above still applies
unchanged; that is one more requirement, not a replacement.*

#### REQ: morale-limits-training

The number of trained specialists a side may have is capped by its current
[morale](../morale/README.md) at the moment training starts — this is a
training-time limit, never an ongoing cost, and morale itself is never
spent by it: specialists use none of the shared captive/informer capacity
described in [Morale](../morale/README.md). If morale later drops below the
number of specialists already trained, nothing happens to them — every one
keeps its profession and abilities for as long as it lives — the side
simply cannot begin training a new specialist while already at or above its
current morale.

#### REQ: training-channel

Training is a channelled action: it uses the acting player's ordinary
command interval to start, does not begin a piece move charge, and is not
sped up by the [Royal Beacon](../royal-beacon/README.md). The default
duration is 10 seconds for either profession, though a match's rules may set
different durations per profession. Training is cancelled without effect if
the trainee moves, is captured, dies, or is otherwise transformed before it
completes — a cancelled training grants no partial profession, consumes no
training slot, and must be restarted from zero.

### The Engineer

#### REQ: engineer-qualifications

*Renamed, not superseded, by [Veteran Progression](veteran-progression/README.md)
(2026-08-07): the numbered qualifications are not gone — an earlier version
of that correction said they would be, and the founder reversed it. The
ordinary grade keeps its rule below exactly as written; the higher grade is
renamed **Master Engineer**, still earned as described below, and still
gating stone. A new rung sits between the two, **Veteran Engineer** — earned
by a second prisoner mission, not a qualification at all, and worth a faster
wooden-wall build rather than a new material.*

The Engineer is a pawn-class specialist focused on battlefield fortification
and logistics — it moves, fights, and gets captured exactly like an ordinary
pawn; its value is entirely in what it can build (see
[Fortifications](../fortifications/README.md)). There are exactly two
qualifications:

- **Qualification I**, granted the moment Engineer training completes: build
  wooden walls, and dismantle or repair friendly walls efficiently.
- **Qualification II**, granted by completing Advanced Engineer Training
  (below): build stone walls, plus faster dismantling and repair where the
  fortifications rules grant a speed-up.

There is no Qualification III and no further wall tier.

#### REQ: advanced-eligibility-through-service

*Renamed, not superseded, by [Veteran Progression](veteran-progression/README.md)
(2026-08-07): the mission described below is unchanged, still live, and is
exactly what makes an Engineer a Veteran Engineer. It still unlocks a trip
back to the training ground, same as always — that trip is now called Master
Engineer training rather than Advanced Engineer Training, and completing it
is a second promotion, not a replacement for this one.*

An Engineer only becomes eligible for Advanced Engineer Training by
personally completing the full prisoner mission: it must itself capture an
enemy piece, personally escort that convoy home, and personally unload a
captive it captured at its own pawn base rank. Helping out, escorting a
convoy someone else captured, or having a teammate finish the mission does
not qualify — the same Engineer has to do the whole thing. Eligibility, once
earned, is kept until Advanced Engineer Training uses it up or the Engineer
is captured or dies.

#### REQ: advanced-engineer-training

*Renamed, not superseded, by [Veteran Progression](veteran-progression/README.md)
(2026-08-07): this training still exists, under the name **Master Engineer
training**. An earlier version of that correction said field service alone
would make the top rung and this training would disappear; the founder
specified the opposite — Veteran Engineer is the field-service rung, and this
training is still the second visit to the base that turns a Veteran Engineer
into a Master Engineer. Its shipped duration (15 seconds, below) and the
founder's stated 10 seconds do not agree; see that Feature's Open
Questions.*

Advanced Engineer Training is a second channelled action, available only to
an eligible Engineer and only at its own pawn base rank, with a default
duration of 15 seconds. It is not limited by morale — the Engineer is
already a trained specialist, and advancing it does not add to the trained
count. It is cancelled without effect by movement, death, capture, or
transformation, exactly like ordinary training. Completing it grants
Qualification II permanently, subject to the same capture-or-death loss
rule as the profession itself.

### The Sergeant

#### REQ: sergeant-aura

The Sergeant is a low-level infantry commander: it makes nearby infantry
better and gains nothing for itself. Its support reaches exactly the eight
squares immediately around it — never farther — and affects only pawn-class
units: ordinary pawns, Engineers, other Sergeants, and any future
pawn-derived profession. It never affects a Knight, Bishop, Rook, Queen, or
King. A Sergeant never benefits from its own aura, though two adjacent
Sergeants can support each other.

#### REQ: sergeant-no-stacking

A pawn-class piece receives at most one Sergeant bonus at a time — when more
than one Sergeant is adjacent, only the strongest applicable bonus applies,
once, never added together.

#### REQ: sergeant-benefits

Sergeant support speeds up execution, not raw strength: it reduces the move
and attack charge time of adjacent pawn-class pieces, and speeds up
adjacent Engineers' (and any other adjacent pawn-class worker's) wall
construction, dismantling, and repair rates. The default multipliers are
conservative — ×0.85 on charge time, ×1.25 on engineering work rates — and
are tuning values, not fixed rules. The Sergeant composes with, and never
replaces, the [Royal Beacon](../royal-beacon/README.md)'s own charge
reduction, and never pushes a charge time below that system's existing
floor.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)
- [Morale](../morale/README.md)
- [Prisoner Convoys](../prisoner-convoys/README.md)
- [Royal Beacon](../royal-beacon/README.md)
- [Espionage](../espionage/README.md)

## Acceptance Criteria

### AC: train-an-engineer-at-the-base

**Given** an active pawn standing on its own pawn base rank
**When** its player starts training and selects Engineer, and the channel
completes
**Then** the pawn becomes an Engineer holding Qualification I, while a pawn
on any other rank attempting the same command is rejected.

### AC: moving-cancels-training

**Given** a pawn partway through a training channel
**When** its player moves it before the channel completes
**Then** training is cancelled, the pawn remains an ordinary pawn, no
training slot is consumed, and a later attempt starts from zero.

### AC: morale-limits-new-training-not-existing-specialists

**Given** a side at morale 2 with two already-trained specialists
**When** a third pawn attempts to start training
**Then** the attempt is rejected until morale rises to 3, at which point the
same command succeeds; **and given** that side's morale later falls to 1
**Then** all three specialists keep their professions and abilities
regardless.

### AC: no-retraining

**Given** a trained Sergeant standing on its pawn base rank
**When** its player attempts to train it as an Engineer
**Then** the command is rejected and the pawn remains a Sergeant.

### AC: capture-erases-the-profession

**Given** an enemy piece captures a trained Engineer
**When** the capture resolves
**Then** the resulting captive is an ordinary neutral pawn with no
profession, and if it is later unloaded it enters play as an untrained
pawn.

### AC: promotion-trades-profession-for-rank

**Given** a Sergeant pawn moves onto the last rank with a promotion choice
**When** the move resolves
**Then** the resulting piece has the chosen rank and no profession.

### AC: advanced-eligibility-requires-personal-service

**Given** an Engineer personally captures an enemy piece, escorts the
convoy home, and unloads that captive at its own pawn base rank
**When** the unload completes
**Then** that Engineer becomes eligible for Advanced Engineer Training,
while a different Engineer that merely escorted someone else's captive does
not.

### AC: advanced-training-unlocks-stone

**Given** an eligible Engineer standing on its pawn base rank
**When** it completes Advanced Engineer Training
**Then** it gains Qualification II and can begin stone-wall construction,
which a Qualification I Engineer cannot.

### AC: sergeant-supports-only-adjacent-pawn-class-units

**Given** a Sergeant with a friendly pawn adjacent to it and a friendly
Knight two squares away
**When** both begin a move
**Then** the adjacent pawn's charge time carries the Sergeant's bonus and
the distant Knight's does not — Knights are never affected regardless of
distance.

### AC: sergeant-bonuses-never-stack

**Given** a pawn-class piece adjacent to two Sergeants
**When** it begins a move
**Then** exactly one bonus — the strongest applicable — applies, never both
added together.

### AC: informer-specialist-actions-are-delayed-not-slowed

**Given** a recruited-informer Engineer, whose recruiting side is at morale
3, and an otherwise identical non-informer Engineer both start wall
construction at the same instant
**When** construction proceeds
**Then** the informer's construction begins after the configured delay and
finishes later by exactly that amount, while its build duration itself
matches the non-informer's exactly.

## Open Questions

- **Does the morale limit on training still earn its keep** now that a pawn
  must also have brought a prisoner home before it can train at all? Two
  limits on the same action may make specialists rarer than intended.
  Unchanged by [Veteran Progression](veteran-progression/README.md); flagged
  for the next balance review.

---
*This document follows the https://specscore.md/feature-specification*
