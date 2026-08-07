---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Veteran Progression

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation/veteran-progression?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation/veteran-progression?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation/veteran-progression?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/pawn-specialisation/veteran-progression?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

A pawn that captures an enemy, escorts the prisoner home and unloads it becomes a Veteran for life — a pawn-only status that promotion ends. Only Veterans can be trained as Engineers or Sergeants, a second mission as an Engineer earns Veteran Engineer (faster wooden walls), and a second training earns Master Engineer (stone walls).

No other rank can ever hold Veteran status, and a pawn that promotes away
from the infantry loses it in the same move.

The ladder is five rungs, not four, because the founder reversed an earlier
version of this correction that removed the top one:

**Pawn → Veteran Pawn → Engineer → Veteran Engineer → Master Engineer**

An Engineer that brings a *second* prisoner home since qualifying becomes a
**Veteran Engineer** and builds wooden walls faster. An Engineer that trains a
*second* time at the base becomes a **Master Engineer** and can build in
stone. Both gates — the training requirement and the stone requirement — are
switched on from day one; there is no wait-and-see rollout.

## Problem

Capturing is the bravest thing you can do in Chess Raiders. You have to reach
the enemy, take them alive, then walk a slow, loaded convoy all the way home
while everyone on the board knows exactly where it is going. Until now that
paid you in material and morale, and nothing else.

Training paid better and cost nothing. The pawn that became an Engineer was
usually the pawn that had never left rank 2 — the safest soldier in the army,
promoted for standing still, while the one who fought its way to the enemy
line and dragged a prisoner back had no career at all.

Tying a career to that act fixes it — but only for pawns. An officer that
pulls off the exact same capture, escort, and delivery earns nothing from it,
the same way an officer could never train in the first place. Nothing about
that changes here.

## Behavior

### Becoming a Veteran

#### REQ: veteran-earned-by-bringing-a-prisoner-home

A **pawn** becomes a **Veteran** the moment it unloads, at its own side's pawn
base rank, a prisoner it captured itself — and it must still be a pawn at
that exact moment. There is one way to earn it and no other: you must make
the capture, escort the prisoner home yourself, and unload it yourself, all
without ever leaving the pawn rank in between.

Escorting a prisoner someone else captured does not count. Standing nearby
does not count. A teammate finishing the job for you does not count. Killing
an enemy never counts — a kill leaves no prisoner to bring home.

**Only a pawn can be a Veteran.** An earlier draft of this Feature said the
opposite — "any piece can be a Veteran, not only a pawn" — and the founder
has ruled directly against it: *"Pawn that promotes to another kind is not
veteran anymore. We do not have veteran system for non-pawn pieces."* A rook
that takes a prisoner and walks it home has done something genuinely brave,
and earns nothing for it — not a smaller reward, no reward, because there is
no such thing as a Veteran rook. The same is true of a pawn that promotes in
the very move that delivers its own prisoner (a pawn intercepting an enemy
convoy on the back rank promotes and takes custody of the demoted escort in
the same instant): whatever it unloads later, it unloads as an officer, and
an officer earns nothing.

Delivering the captured **king** does not make you a Veteran, because
delivering the king wins the match.

#### REQ: veteran-credit-follows-the-capture

Credit for a capture is tracked the same way regardless of rank — whoever
demoted a piece into a prisoner is who it's credited to, including when that
happens by taking a *loaded* enemy convoy prisoner and inheriting everything
it was carrying. Only the escort you personally beat credits you; captives
you merely inherited credit nobody.

Taking a loaded convoy prisoner is something any rank can do, officers most of
all — it is one of the more dangerous things an officer can attempt. So most
of that credit is earned by pieces that were never going to become Veterans
no matter what they unloaded. Only when the piece doing the taking is itself
still a pawn at the moment it unloads its prize does that credit turn into
Veteran status.

#### REQ: veteran-is-for-life-except-promotion-and-capture

Once earned, Veteran status is never taken away while that piece is on the
board **and remains a pawn**. It survives escort duty, refitting, a morale
collapse, and training. It does not survive the two things that end a piece's
time as a pawn:

- **Capture.** A captured piece becomes an ordinary prisoner like any other,
  and if it is ever unloaded back into play it comes home an ordinary,
  untrained, non-Veteran pawn — on its captor's side. Nothing about who it
  used to be survives being taken.
- **Promotion.** A Veteran pawn that promotes to a queen is **not** a Veteran
  queen. An earlier draft of this Feature claimed the opposite — that Veteran
  status was "a service record, not a trade," and survived promotion the way
  a profession does not. The founder corrected that directly: there is no
  Veteran system for non-pawn pieces at all, so the moment a Veteran pawn
  becomes something else, there is nothing left for the status to attach to.
  This is now a stated rule at the exact moment of promotion, not something
  left to happen by accident — promotion is where the piece already gets its
  professional trade taken away (see [Pawn Specialisation](../README.md)),
  and Veteran status goes with it in the same move.

#### REQ: a-veteran-is-visible

A Veteran is visibly marked on the board, distinguishable at a glance from an
ordinary pawn, and an Engineer, a Veteran Engineer and a Master Engineer are
all distinguishable from one another. Because only a pawn can ever be a
Veteran, the board never needs to show that mark on any other rank — one
fewer combination to design for, not one more.

The mark is **public** — both sides see it. Your opponent watched the convoy
come home; there is nothing to hide, and a reward nobody can see is not a
reward.

### The Engineer's two upper rungs

This correction also reverses something else this Feature said the first
time: that Qualification II left the game entirely. It does not. The founder
brought it back as a named rung — **Master Engineer** — and then said exactly
how each of the two upper rungs is earned. Both turn out to already exist
under the old names.

| Rung | How you get it | What it's worth |
|---|---|---|
| Veteran Pawn | 1 prisoner mission, as an ordinary pawn | may train (Engineer or Sergeant) |
| Engineer | 1 training session at base, 10s | builds wooden walls |
| Veteran Engineer | a 2nd prisoner mission, as an Engineer | builds wooden walls **faster** |
| Master Engineer | a 2nd training session at base | can build **stone** walls |

Spelled out plainly, so nobody has to work it out: getting to stone costs
**two** prisoner missions and **two** trips to the training ground — one
mission and one training to become an Engineer, a second mission to become a
Veteran Engineer, and a second training on top of that to become a Master
Engineer. There is no third mission anywhere on this ladder. (An earlier
version of this correction, written before the founder said Master Engineer
is trained rather than earned in the field, said three missions. That number
is wrong; this table replaces it.)

#### REQ: veteran-engineer-is-a-second-mission

A Veteran Engineer is exactly what this Feature used to call "advanced
eligibility": an Engineer that has personally captured a second enemy,
escorted it home, and unloaded it, while already holding the Engineer trade.
Nothing about how you earn it changes. What changes is that it is no longer a
hidden step toward something else — it is a rung in its own right, with its
own name and its own reward: your wooden walls go up faster from that point
on.

An earlier draft of this Feature also claimed this status would apply to
every profession, so that a "Veteran Sergeant" was a meaningful state even
though nothing used it yet. That claim was wrong — the underlying mechanism
has only ever existed for the Engineer trade, and this Feature does not
invent a Sergeant equivalent. There is no Veteran Sergeant and no Master
Sergeant. Sergeant stays exactly the single rank it already was.

#### REQ: master-engineer-is-a-second-training

A Master Engineer is exactly what this Feature used to call "Qualification
II, earned by Advanced Engineer Training": a Veteran Engineer that goes back
to its own pawn base rank and completes a second, separate training channel.
It is not Morale-gated — training further does not add another specialist to
the count. Nothing about how you earn it changes; what changes is that it
stays in the game under a better name instead of leaving it.

**One number is not settled.** The founder said this training defaults to 10
seconds. The version of this training already running in the game defaults to
15. Ten seconds is what *ordinary* Engineer and Sergeant training already
takes — so either the shipped 15-second value is about to become 10, or "10
seconds" was describing the general shape of training rather than this
specific one. This Feature does not guess which; see Open Questions.

#### REQ: veteran-engineer-builds-wood-faster

A Veteran Engineer's wooden walls go up faster than an ordinary Engineer's.
This is specifically about **building** — the founder's own word. It is not
about repairing (which already has its own separate speed table, gated at
Master Engineer, and stays that way) and it is not about dismantling (which
has never depended on qualification at all — it depends on what kind of piece
is doing the work). A Master Engineer keeps the faster wood build a Veteran
Engineer already had; nothing about advancing further takes it away.

#### REQ: stone-is-a-master-engineers-work

Building a **stone** wall requires a Master Engineer — the trained-twice
rung, not the field-earned one. An earlier draft of this correction placed
the stone gate one rung lower, on Veteran Engineer, following the founder's
very first instruction that stone needed "an Engineer who brought at least
one prisoner back to base since becoming an Engineer." The founder's later,
more specific word — a Master Engineer can build stone — moves that gate up
one rung. A Veteran Engineer that has not yet trained a second time builds
faster wood, same as before, but still cannot touch stone.

### What only a Veteran can do

#### REQ: only-veterans-can-train

Only a Veteran may be trained as a specialist. Every other condition on
training stays exactly as
[Pawn Specialisation](../README.md) already states it — an active, ordinary,
untrained pawn, standing on its own side's pawn base rank, within its side's
morale limit, choosing its profession explicitly, channelled for the usual 10
seconds. This adds one requirement to that list and removes none.

A pawn that has never brought a prisoner home is refused, and the refusal says
what to go and do about it rather than simply saying no.

### Naming

#### REQ: veteran-names-the-progression-not-the-bot

"Veteran" names this progression. The single-player bot difficulty tier that
used to carry the name is renamed **Lieutenant**, and now reads Recruit →
Lieutenant → Commander. The tier itself — how it plays, how hard it is — does
not change, and links or saved matches that already chose it keep working.

#### REQ: sapper-is-reserved

**"Sapper" is reserved** for a future mine-laying specialist and is not spent
on anything in this ladder — Engineer keeps its own name throughout. There is
no mine-laying specialist yet; this rule exists only so nobody else uses the
name first.

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

### AC: an-officer-earns-nothing-for-the-same-mission

**Given** a rook that captures an enemy piece, becomes its escort, and
personally walks it home and unloads it at its own pawn base rank
**When** the unload completes
**Then** the rook is not a Veteran and never becomes one, even though the
exact same sequence performed by a pawn would have earned it.

### AC: escorting-someone-elses-prisoner-earns-nothing

**Given** a convoy carrying only a prisoner that a different piece captured
**When** it unloads that prisoner at its own pawn base rank
**Then** the escort is not a Veteran.

### AC: taking-a-loaded-convoy-credits-only-the-escort-you-beat

**Given** a pawn that attacks an enemy convoy already carrying two prisoners,
taking its escort prisoner and inheriting its cargo
**When** it brings all three home and unloads them
**Then** it becomes a Veteran for the escort it beat, and the two inherited
prisoners earn it nothing.

### AC: an-officer-taking-a-loaded-convoy-is-credited-but-earns-nothing

**Given** a knight that attacks an enemy convoy, taking its escort prisoner
**When** the knight brings the demoted escort home and unloads it
**Then** the capture is still credited to the knight, but no Veteran status
is granted, because the knight was never a pawn.

### AC: promotion-ends-veteran-status-the-same-way-capture-does

**Given** a Veteran pawn one square from the last rank, and a different
Veteran pawn adjacent to an enemy
**When** the first promotes
**Then** the new queen is **not** a Veteran and cannot become one, being a
queen; **and given** the second is captured and later unloaded by its captor
**Then** it comes back an ordinary, untrained, non-Veteran pawn on its
captor's side.

### AC: an-untried-pawn-cannot-train

**Given** an ordinary pawn on its own pawn base rank, with its side well inside
its morale limit
**When** its player tries to train it as an Engineer
**Then** the attempt is refused and the refusal says a prisoner must be brought
home first; **and when** that same pawn later captures an enemy, escorts it
home and unloads it
**Then** the same training command succeeds.

### AC: a-second-mission-makes-a-veteran-engineer-that-builds-wood-faster

**Given** a trained Engineer that has never brought a prisoner home since
qualifying, and a second, otherwise-identical Engineer
**When** the second one captures an enemy, escorts it home and unloads it,
and both Engineers then build a wooden wall
**Then** the second Engineer is a Veteran Engineer, its wall goes up faster
than the first Engineer's, and neither Engineer's repair or dismantle speed
differs from the other's.

### AC: stone-needs-a-second-training-not-just-a-second-mission

**Given** a Veteran Engineer that has never trained a second time
**When** it tries to build a stone wall, then completes a second training
session and tries again
**Then** the first attempt is refused, and the second succeeds — it is now a
Master Engineer.

### AC: master-engineer-training-is-the-old-advanced-training-renamed

**Given** an Engineer that is not yet a Veteran Engineer, and a Veteran
Engineer, both on their own pawn base rank
**When** each tries to start the second training session
**Then** the first is refused for not having brought a second prisoner home
yet, and the second's training starts and, on completion, makes it a Master
Engineer.

### AC: advanced-engineer-training-is-renamed-not-gone

**Given** an Engineer standing on its own pawn base rank
**When** its player looks for a way to advance it
**Then** the training exists under the name "Master Engineer training," not
"Advanced Engineer Training" or "Qualification II," anywhere the rules or the
client describe it.

## Open Questions

- **Is Master Engineer training 10 seconds or 15?** The founder said 10; the
  version of this training already in the game defaults to 15, and 10 is what
  *ordinary* specialist training already takes. Needs a founder call on
  whether the shipped duration changes or the 10-second figure was describing
  the pattern rather than this specific channel.
- **Does "builds... faster" ever extend to repairing?** This Feature reads it
  as building only, because that was the founder's specific word. Repair
  speed already has its own table, already gated at Master Engineer, and this
  correction leaves it there unless told otherwise.
- **Does the morale limit on training still earn its keep?** A side is now
  limited both by its morale and by how many prisoners it has actually brought
  home. Two limits on the same action may make specialists rarer than intended.
  The founder has ruled balance measurement out as a reason to delay shipping
  this — both gates are on regardless — so this is a tuning question for the
  next balance review, not a shipping question.

---
*This document follows the https://specscore.md/feature-specification*
