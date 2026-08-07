---
format: https://specscore.md/feature-specification
status: Implementing
---

# Feature: Fortifications

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fortifications?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fortifications?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fortifications?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fortifications?op=request-change) |
**Status:** Implementing
**Source Ideas:** —

## Summary

Engineers build on the edges between squares, not on the squares themselves.
Wood blocks passage and nothing else; stone blocks passage and sight alike.
Every wall belongs to the whole team, carries real Integrity that can be
chipped away and later repaired, and falls only to sustained adjacent
work — never to a ranged blow.

## Problem

The battlefield was pure open ground: every rule about movement and vision
was purely a function of which squares held which pieces, so position could
only ever be shaped by standing somewhere. There was no way to prepare
ground, protect a corridor, deny a line, or force a breach —
[Pawn Specialisation](../pawn-specialisation/README.md) gave the army a
professional soldier capable of that kind of work, and Fortifications gives
that soldier something real to build.

## Behavior

### The wall model

#### REQ: edge-walls

A wall occupies the edge between two orthogonally adjacent squares — a cell
boundary, never a square itself. It separates its two neighbouring squares;
both squares stay independently free or occupied exactly as they would
without it. At most one wall can ever exist on a given edge, and the edge
between two squares is the same edge no matter which side you approach it
from.

#### REQ: wall-team-ownership

Walls belong to a side, never to whichever Engineer built them. Any friendly
Engineer may repair or efficiently dismantle any of its side's walls, and
any other friendly piece may dismantle its own side's wall at its ordinary
rate — nothing about a wall ever requires its original builder specifically.
Enemy walls can be dismantled, but never repaired.

#### REQ: wall-materials

There are exactly two materials, and no further tiers:

- **Wooden** — blocks movement across its edge, but does not block vision at
  all. Lower maximum Integrity, faster to build, easier to dismantle. It
  stays useful even after stone is unlocked, because it lets its own builder
  keep seeing across the line it defends.
- **Stone** — requires a Qualification II Engineer (see
  [Pawn Specialisation](../pawn-specialisation/README.md)) to build; blocks
  both movement and vision across its edge, symmetrically for both sides.
  Higher maximum Integrity, harder to dismantle, and a longer base
  construction time.

*Amended by
[Veteran Progression](../pawn-specialisation/veteran-progression/README.md)
(2026-08-07): stone requires a **Veteran Engineer** — an Engineer that has
brought a prisoner home since it qualified — rather than a numbered
qualification. Nothing else about either material changes, and a Veteran
Engineer keeps the faster construction and repair the old grade carried.*

### Building a wall

#### REQ: construction

Only an Engineer builds a wall, one edge at a time, choosing a single
orthogonal direction — north, south, east, or west — per construction
action; never several walls at once. Construction is a channelled action
with a default duration of 3 seconds for wooden and 6 seconds for stone,
faster for a more qualified Engineer or with
[Sergeant](../pawn-specialisation/README.md) support. An edge that already
carries a wall, or is already under construction, cannot be started again.

#### REQ: construction-cancellation

If the building Engineer moves, dies, is captured, or is otherwise removed
before construction finishes, the attempt is cancelled and its progress is
discarded completely — unlike dismantling damage (below), incomplete
construction leaves nothing behind. A wall only exists from the instant its
construction finishes, and it exists at full Integrity from that moment.

### Moving around walls

#### REQ: walls-block-crossing

No piece may directly cross a walled edge — this applies uniformly to a
pawn's single or double step (a double step crosses two edges and is
blocked if either one is walled), a king's step, a convoy's step, and a
rook's or queen's slide, which stops at a walled edge exactly as it would at
a blocking piece. Castling is blocked the same way if a walled edge lies
between the squares the king or rook needs to cross.

#### REQ: corner-rule

Diagonal movement is governed by one simple, symmetric rule. A diagonal step
passes through the corner point shared by the moving square, the
destination square, and the two squares beside them. That corner is sealed
only when *both* edges of one full side through it are walled — either both
vertical edges, or both horizontal edges. A single wall alone never blocks a
diagonal step through its corner; a continuous two-wall line does. The same
rule governs a bishop's or queen's diagonal slide, a pawn's diagonal
capture, a king's diagonal step, an en-passant capture, a stone wall's
vision, and the [Royal Beacon](../royal-beacon/README.md) chain's diagonal
links — one geometric rule, applied everywhere diagonals matter.

#### REQ: knights-jump-walls

A knight jumps over a wall exactly as it jumps over a piece: a wall blocks
ground movement across its edge, not a knight's leap over it. A knight is
otherwise an ordinary piece around walls — like everyone else, it still has
to stand adjacent to a wall to dismantle it.

### Vision

#### REQ: stone-blocks-vision

A wooden wall never blocks vision. A stone wall blocks vision across its
edge for both sides equally: it stops a sliding line of sight the same way
it stops a sliding move, adjacency vision does not cross it, and diagonal
sight lines obey the same corner rule as diagonal movement. A recruited
informer (see [Espionage](../espionage/README.md)) sees through no stone
wall it does not otherwise have a clear line past. A move rejected because
of a wall you cannot see gives no more information than any other rejection
caused by something hidden — walls introduce no new way to learn what the
enemy has built or where its formation stands.

### Integrity

#### REQ: wall-integrity

A wall's durability, called **Integrity**, is measured in the time a single
ordinary pawn working alone would need to dismantle it from full: 5 seconds'
worth for a wooden wall, 15 seconds' worth for a stone wall, by default.
Integrity belongs to fortifications only — pieces never gain health of their
own and remain governed entirely by the normal capture rules. Damage to a
wall persists: three seconds of work followed by an interruption leaves the
wall at two seconds' worth of remaining Integrity for whoever picks the work
back up later — nothing resets just because the work stopped.

#### REQ: dismantling-by-class

Dismantling is adjacent, hands-on work — never a ranged attack. A piece must
stand on one of the two squares the wall's edge separates to work on it, at
a rate set by how suited that piece is to physical labour, not by its chess
value:

| Class | Pieces | Rate (of pawn baseline) |
|---|---|---:|
| Engineering | Engineer | 2.0× |
| Heavy | Rook, Bishop | 1.5× |
| Infantry | Pawn | 1.0× |
| Light | Knight | 0.5× |
| Command | Queen, King | 0.25× |

These are the default tuning multiples; the ordering between classes is the
important, settled part, and any two figures within a class may be
rebalanced. No piece can ever damage a wall through a ranged attack.

#### REQ: repair

Only a friendly Engineer repairs a wall — any Engineer, on any of its side's
walls, stone included — and a more advanced Engineer repairs faster.
Repairing is adjacent, channelled work, exactly like dismantling: it raises
Integrity over time, never above maximum, and stops the instant the
repairing Engineer moves, dies, or is captured.

#### REQ: cooperative-work

Several adjacent pieces can work the same wall at once — cooperative
dismantling and cooperative repair are both natural team tactics, and both
are allowed by default. All the active work on one wall combines into a
single net rate: total repair minus total dismantling, applied continuously
and never pushed outside the wall's own Integrity range. Starting or
stopping one contributor simply changes that net rate from that moment on.
A wall that reaches zero Integrity is destroyed and every active session on
it ends at once; a wall at maximum Integrity simply idles under repair
without overflowing.

### The command chain

#### REQ: walls-sever-the-chain

A wall — wooden or stone, it makes no difference — severs the
[Royal Beacon](../royal-beacon/README.md) chain of command across its edge:
an orthogonal link between two pieces does not exist across a walled edge,
and a diagonal link obeys the same corner rule described above. This lets a
wall protect a friendly command corridor, cut an enemy one, or force a relay
to reposition.

## Dependencies

- [Pawn Specialisation](../pawn-specialisation/README.md)
- [Real-Time Command](../real-time-command/README.md)
- [Royal Beacon](../royal-beacon/README.md)
- [Fog of War](../fog-of-war/README.md)

## Acceptance Criteria

### AC: build-each-edge-independently

**Given** a Qualification I Engineer with wooden walls enabled
**When** it completes four separate construction actions choosing north,
south, east, and west in turn
**Then** four independent wooden walls exist, one on each edge, each built
by its own channelled action.

### AC: duplicate-construction-is-rejected

**Given** a wall already exists on an edge
**When** any Engineer attempts to build on that same edge from either side
**Then** the attempt is rejected, and a second attempt on an edge already
under construction is likewise rejected.

### AC: cancelled-construction-leaves-nothing

**Given** an Engineer partway through building a wooden wall
**When** that Engineer is captured before construction completes
**Then** no wall exists on that edge, and a later attempt there starts from
zero.

### AC: wooden-blocks-movement-but-not-sight

**Given** a wooden wall on one edge
**When** movement and vision across that edge are each checked
**Then** movement across the edge is blocked, while vision continues past it
exactly as if no wall were there.

### AC: stone-blocks-both

**Given** a stone wall on one edge in a Fog of War match
**When** movement and vision across that edge are each checked
**Then** both are blocked for both sides, including a recruited informer's
vision unless it has its own separate clear line.

### AC: knight-jumps-a-fully-walled-square

**Given** a knight surrounded by walls on every edge of its own square
**When** it is commanded to a legal knight-move square
**Then** the move succeeds normally, because a knight jumps ground
obstacles rather than crossing edges.

### AC: single-wall-never-blocks-a-diagonal

**Given** a single wall on one edge of a corner
**When** a diagonal move or sight line passes through that corner
**Then** it is unaffected; **given** a second, collinear wall on the other
edge through the same corner
**Then** the same diagonal move or sight line is blocked.

### AC: dismantling-damage-persists-across-interruptions

**Given** a fresh wooden wall and a pawn that dismantles it for 3 of its 5
seconds' worth of Integrity before stopping
**When** a different piece later resumes work on the same wall
**Then** it starts from 2 seconds' worth of remaining Integrity, not 5.

### AC: dismantling-rate-follows-class-order

**Given** identical fresh walls, each worked alone by an Engineer, a Rook, a
Pawn, a Knight, and a Queen
**When** each works uninterrupted
**Then** the walls fall in that exact order — Engineer first, then Rook,
Pawn, Knight, and Queen last — at their configured rate multiples.

### AC: no-ranged-wall-damage

**Given** a piece with a clear line to a wall but not standing adjacent to
it
**When** it attempts to affect that wall in any way
**Then** the attempt is rejected — only a piece standing on one of the
edge's two squares can work on it.

### AC: cooperative-work-nets-deterministically

**Given** one enemy piece dismantling a wall while one friendly Engineer
repairs it at the same time
**When** both act concurrently
**Then** the wall's Integrity changes by exactly the net of the two rates,
and adding a second dismantler shortens the time to destruction
accordingly.

### AC: wall-destroyed-at-zero-integrity

**Given** a wall under sustained dismantling with no repair
**When** its Integrity reaches zero
**Then** the wall is removed, every active work session on it ends, and
movement across that edge is legal again.

### AC: repair-never-exceeds-maximum

**Given** a damaged friendly wall under repair by two Engineers
**When** its Integrity reaches maximum
**Then** it stops rising there, and an enemy Engineer's attempt to repair it
is rejected outright.

### AC: any-friendly-engineer-can-repair-or-dismantle

**Given** a stone wall built by one Engineer
**When** a different friendly Engineer repairs it and a friendly ordinary
pawn dismantles it
**Then** both actions are accepted at their own rates, with the original
builder uninvolved.

### AC: wall-severs-a-command-chain-link

**Given** a live Royal Beacon chain running through two friendly pieces on
adjacent squares
**When** a wall completes on the edge between them with no alternate path
available
**Then** the chain breaks until the wall is removed or another link forms.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
