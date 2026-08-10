---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Prisoner Convoys

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/prisoner-convoys?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/prisoner-convoys?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/prisoner-convoys?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/prisoner-convoys?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

A successful capture does not just clear a square — it hands you a logistics
problem. The capturing piece becomes an escort, the captured piece becomes an
anonymous prisoner riding along as cargo, and getting that cargo home is a
second objective layered on top of ordinary movement. Hit an enemy convoy
and everything it carries — cargo and escort alike — becomes yours instead.
Capturing the enemy king is the ultimate version of this: he becomes a
prisoner too, and delivering him to one of your two royal squares — for
king cargo specifically, regardless of which piece is escorting him — is
how the match is won.

## Contents

| Child | Description |
|---|---|
| [convoy-clears-its-path](convoy-clears-its-path/README.md) | A loaded convoy is not defenceless: it may attack whatever blocks its one legal step home, choosing Kill, Capture, or Recruit Informer exactly like any other piece. |

## Problem

A capture that simply removes a piece from the board treats every capture as
identical and gives raiding armies no reason to protect what they take.
Chess Raiders turns capture into logistics: a prisoner is a liability that
has to be escorted, can be intercepted and lost, and only pays off once it is
actually brought home — the same is true, at higher stakes, of the enemy
king himself.

## Behavior

### Becoming a convoy

#### REQ: capture-creates-a-convoy

When a non-king piece is captured (see
[Capture Outcomes](../capture-outcomes/README.md) for how a capture is
decided), the capturing piece becomes the convoy's escort, keeping its own
rank as visible information, and the captured piece is demoted to one
neutral captive pawn — whatever it used to be survives only as history.
Captives are neutral: they belong to no side while they ride as cargo and
only take the capturing side's colour once they are unloaded.

#### REQ: convoy-is-a-unit

A loaded convoy — one carrying at least one captive or the captured enemy
king — is a first-class piece on the board: it occupies exactly one square,
blocks movement through that square, can be moved by any teammate, and can
be attacked by the enemy. Whether — and how — it can strike back is covered
in [Convoy Clears Its Path](convoy-clears-its-path/README.md).

### Moving a convoy

#### REQ: convoy-movement

A loaded convoy moves exactly one square per move, and only sideways or
toward its own side's home — straight or diagonally, never toward the
enemy. Dead ends are legal and intended: nothing rescues a trapped convoy by
movement alone. There is no escape action, no teleport, and no relief for
congestion. A loaded convoy charges its one-square move in 2 seconds — quick
for logistics, but slow enough to give a defender a real chance to catch it.

A loaded convoy may not enter its own back rank — rank 1 for White, rank 8
for Black — unless that exact step delivers the captured enemy king onto
one of the two royal squares there and wins the match outright (see
`cargo-based-delivery` below); every other loaded convoy is held to its own
pawn-starting rank as a floor. A loaded convoy may also dismantle — but
never repair — a wall on an edge touching its own square, at its escort's
own rate, exactly like any other piece; see
[Fortifications](../fortifications/README.md).

### Interception

#### REQ: convoy-interception

Any enemy piece that captures onto a loaded convoy's square always
intercepts it — Kill is never offered here, and no morale threshold applies.
The attacker becomes the new escort with its own rank as metadata; the
convoy keeps its square and all of its existing cargo, including any
captured king; and the previous escort is demoted into the convoy as one
more neutral captive. Interception chains are unlimited — a convoy carrying
a captured king can change hands again and again.

### Unloading prisoners

#### REQ: pawn-unloading

Captive pawns come off only at your own pawn-starting rank — rank 2 for
White, rank 7 for Black — and only on the way out. Simply entering a free
square on that rank changes nothing by itself: the convoy stays whole,
loaded, and interceptable. One captive is unloaded only when the convoy then
legally leaves that square: the square it departs receives a brand-new
active pawn in the convoy's owner's colour, the convoy's cargo drops by one,
and the convoy continues on with what remains. Captives always unload
first-in, first-out. The freshly unloaded pawn counts as unmoved, so it may
still double-step. Repeating the enter-and-leave pattern across free
pawn-rank squares can unload several prisoners over a match — a side can end
up with more than eight pawns.

### Refitting

#### REQ: refit-lifecycle

The moment a departure unloads a convoy's very last captive, the unit on the
departure square stops being a convoy and becomes its escort piece again, in
a refitting state: its original rank and movement rules, moveable by any
teammate, unable to attack or capture, and still capturable by the enemy.
There is no timer — refitting completes the instant the piece is standing on
a free base square for its rank, and it becomes an ordinary active piece
again. Because the final unload always happens on the pawn-starting rank, a
pawn escort finishing its unload refits instantly and never has to travel.
Base squares are:

| Piece | Base square (White) |
|---|---|
| Rook | a1 / h1 |
| Knight | b1 / g1 |
| Bishop | c1 / f1 |
| Queen | d1 |
| King | e1 |
| Pawn | any square on rank 2 |

Black mirrors these on ranks 8 and 7. A refitting piece must physically reach
its base square under its own movement rules — nothing teleports it there. A
convoy that loses its very last prisoner to a
[morale overflow execution](../morale/README.md) also becomes a refitting
escort on the spot, and refits immediately if it happens to already be
standing on a base square.

These base squares govern only an *emptied* escort's own return to active
play. A *loaded* convoy — one still carrying cargo — delivers that cargo
somewhere else entirely; see `cargo-based-delivery` below.

### The captured king

#### REQ: king-is-always-captured

Capturing the enemy king is always a capture, in every battle mode — never a
kill. He becomes a unique piece of cargo, distinct from ordinary captive
pawns, can coexist with any number of captive pawns in the same convoy, and
transfers intact through every interception a convoy carrying him goes
through. A convoy can carry at most one captured king at a time. Capturing
the enemy king does not end the match by itself.

#### REQ: cargo-based-delivery

Where a loaded convoy may deliver depends on what it is carrying, never on
its escort's own rank. A captured king is delivered onto either of the
delivering side's two **royal squares** — the King's own base square or the
Queen's base square (e1 or d1 for White, e8 or d8 for Black) — no matter
which piece is currently escorting him. A captive pawn is still delivered
(unloaded) onto the delivering side's pawn-starting rank exactly as
`pawn-unloading` describes above, likewise regardless of escort rank. An
escort's own rank continues to decide only its own refit once it is
carrying no cargo at all — it never decides where cargo already aboard
comes home.

Two royal squares exist, rather than one, because a Bishop's *capture* is
colour-bound: a Bishop's attack geometry is diagonal-only, which preserves
square colour, so a Bishop can only ever capture a king standing on a
square of the Bishop's own colour. White's e1 is a dark square and d1 is
light (the mirror image for Black), so exactly one Bishop colour can
capture-and-deliver on a given royal square in the same motion as the
capture. Once a capture has formed a convoy, ordinary convoy movement (see
`convoy-movement` above) is unrestricted by rank or square colour, so this
colour bind affects only that one instant — a Bishop escort already under
way can still walk to either royal square.

#### REQ: king-delivery-victory

The match is won the instant a convoy carrying the captured enemy king
stands on either of the delivering side's royal squares (per
`cargo-based-delivery`) — whether it walked there or was formed there
directly by an interception landing on one. This is the highest-priority
objective in the game: any captive pawns still riding along do not delay or
prevent the win, and are simply recorded as undelivered. Only the *enemy's*
captured king counts this way — if a team recaptures a convoy carrying its
own king, that king is just protected cargo; reaching a royal square with it
triggers nothing. The moment victory is won, the match ends immediately: the
winning side, the final board, and the full match history are preserved,
and nothing further can be commanded.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)
- [Morale](../morale/README.md)
- [Fortifications](../fortifications/README.md)

## Acceptance Criteria

### AC: capture-forms-a-convoy

**Given** a piece captures an enemy non-king piece
**When** the capture resolves
**Then** the capturing piece becomes a convoy escort with one neutral
captive pawn as cargo, standing on the capture square.

### AC: convoy-never-moves-toward-the-enemy

**Given** a loaded convoy of either side stands in open space
**When** it attempts a straight or diagonal move toward the enemy side
**Then** the move is rejected, while sideways and homeward moves succeed.

### AC: back-rank-is-closed-except-for-king-delivery

**Given** a loaded convoy stands one homeward step from its own back rank
**When** entering that square would not deliver the captured enemy king onto
a royal square
**Then** the move is rejected; **when** instead that same step delivers the
captured enemy king onto a royal square
**Then** the move succeeds and wins the match immediately.

### AC: convoy-dismantles-but-never-repairs-a-wall

**Given** a loaded convoy stands adjacent to a wall on either side's team
**When** its player commands it to dismantle that wall
**Then** the action is accepted at the escort's own work rate; **when**
instead the player commands it to repair a wall
**Then** the action is rejected regardless of which side built the wall.

### AC: unloading-only-happens-on-departure

**Given** a convoy with two captive pawns enters a free square on its owner's
pawn-starting rank
**When** it is intercepted before moving again, no pawn appears on that
square and the interceptor gains all of the cargo; **and when** instead the
same convoy legally departs that square
**Then** the departure square receives a new active pawn and the convoy
continues carrying one fewer captive.

### AC: interception-transfers-everything

**Given** a convoy carrying one captive pawn is escorted by a piece of one
side
**When** an enemy piece captures onto the convoy's square
**Then** the convoy changes ownership, keeps its square and all its cargo,
gains one more captive from the demoted former escort, and no Kill option
was ever offered.

### AC: final-unload-starts-a-refit

**Given** a convoy's departure unloads its last captive pawn
**When** the departure completes
**Then** the unit becomes a refitting piece with its original movement rules,
unable to attack, and becomes fully active the instant it reaches a free
base square for its rank.

### AC: king-is-captured-never-killed

**Given** any battle mode, including one where non-king captures are killed
outright
**When** a piece captures the enemy king
**Then** the king becomes captured-king cargo rather than being removed from
the game, and the match continues.

### AC: delivering-the-captured-king-wins

**Given** a convoy carrying the captured enemy king, plus some captive
pawns, is positioned to enter a free royal square (e1 or d1 for White, e8 or
d8 for Black)
**When** the convoy enters that square
**Then** the delivering side wins immediately, the match locks against
further commands, and the final board and match history are preserved.

### AC: delivery-square-follows-cargo-not-escort

**Given** a Rook-escorted convoy carries the captured enemy king and stands
one homeward step from a royal square, with the Rook's own base squares
(a1/h1) irrelevant to this cargo
**When** the convoy enters that royal square
**Then** the delivering side wins immediately, exactly as it would for any
other escort rank, because the king cargo decides the delivery square, not
the Rook's own rank

### AC: bishop-capture-and-deliver-is-colour-bound

**Given** a Bishop captures the enemy king while the king already stands on
a royal square of the Bishop's own colour, forming the convoy directly on
that square
**When** the capture is applied
**Then** the delivering side wins immediately in the same motion as the
capture — the second royal square exists for exactly this reason, so a
Bishop of either colour can capture-and-deliver the king in one move, even
though a single Bishop can never capture a king standing on the opposite
colour

### AC: own-king-recapture-is-just-protected-cargo

**Given** a side recaptures a convoy carrying its own captured king
**When** that convoy later reaches a royal square
**Then** nothing special happens — the king remains protected cargo, and the
match does not end.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
