---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Real-Time Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/real-time-command?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/real-time-command?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/real-time-command?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/real-time-command?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

Chess Raiders has no turns. Both armies act at the same time. A commanded
piece does not move the instant you command it — it charges on its starting
square for a duration set by its rank and by whether it is moving or
attacking, then the order resolves. Charging is readable to your own side as
a route plan; an attack is readable to the threatened side as an anonymous
warning. Classical movement, castling, en passant, and promotion continue to
work exactly as in ordinary chess.

## Contents

| Child | Description |
|---|---|
| [pre-planned-attacks](pre-planned-attacks/README.md) | Chaining a further move onto a piece that is already charging can target an enemy square too: the Kill/Capture/Recruit choice is asked at planning time and travels with the queued step instead of being lost. |

## Problem

Alternating turns do not fit a group commanding one army together, and simply
moving a piece at once and locking it afterward gives a defender no way to
see a blow coming and gives teammates no way to see that someone is already
routing a piece toward a square. Chess Raiders needed preparation *before* movement
rather than recovery *after* it — readable to your own side without leaking
your plans to the enemy, except for the one signal the enemy is owed: that
something is about to hit them.

## Behavior

### Command and army

#### REQ: shared-command

A match has exactly two teams, White and Black. Each team has one or more
players, and team sizes need not match — 1v1, 2v3, 5v10, and every other
combination are supported, never flagged as invalid. Every player on a team
may command any of that team's units; there is no per-player ownership of a
piece and no turn order. Any teammate may continue a plan another teammate
started.

#### REQ: charge-before-move

When you command a legal move or attack, the piece stays on its starting
square and begins charging. It has at most one active order at a time. While
it charges, an upward-filling indicator is bound to the piece itself, not to
the square — it disappears the instant the move resolves or the order is
cancelled. The piece leaves its square only at the moment the charge
completes and the move resolves.

Charge duration depends on the piece's rank, and moving charges faster than
attacking: repositioning is quick, but violence always takes longer to
prepare, so an alert defender can generally answer an attack before it
lands. The king moves faster than anything on the board but fights slower
than most of it — it leads, escapes, and defends rather than assassinating.

| Piece | Move charge | Attack charge |
|---|---:|---:|
| King | 1s | 3s |
| Pawn | 1s | 2s |
| Knight | 1.5s | 2.5s |
| Bishop | 1.5s | 2.5s |
| Rook | 2s | 3s |
| Queen | 2.5s | 3.5s |
| Loaded convoy | 2s | — (cannot attack) |

These are the default tuning values of the **balance-v7** profile, recorded
on every match; a match may be started with a different rules preset.

#### REQ: player-command-interval

Separately from any piece's charge time, every player has a personal
1-second command interval that applies across every unit they command. It
paces how quickly one person can issue new orders; it is not a property of
the piece and is never reduced or extended by anything that shortens or
lengthens a piece's charge.

### Route planning

#### REQ: route-planning-basics

A route is your ordered list of destinations for one piece: step 1, step 2,
step 3, and so on. Only your own team sees it. Each planned destination shows
as a small, dim ring on its target square carrying the route-step number; the
ring reflects cumulative progress through the whole route, not just the
current step. If several teammates plan moves to the same square, the team
sees one ring that grows bolder with each additional plan and carries a small
badge with the total overlap count.

Selecting a piece that already has a queued route shows its full planned path
as numbered rings joined by dashed lines, together with the moves currently
legal from where the piece actually stands — this is read-only and changes
nothing. If you then commit one of those currently legal moves, the old route
and its live charge are dropped, all its markers disappear, and a new route
begins at step 1. This works regardless of which teammate queued the
original route, because the army is shared.

None of this — destinations, order, progress, overlaps, or which piece is
moving — is ever visible to the opposing team.

#### REQ: attack-target-lock

An accepted attack creates a second, different signal: an anonymous
under-attack marker on the threatened piece, visible to both teams. It says
only that the piece is threatened. It never reveals who is attacking, where
the attack is coming from, its charge progress, or its route. Your own team
still gets its ordinary route marker on the attacker in addition to this.

#### REQ: changed-target-cancels-route

An attack is committed against the exact piece it targeted at the moment it
was accepted — never against whichever piece later happens to occupy the
destination square. The instant that exact piece moves away, is captured,
or is replaced by any other piece on that square — even one of the same
rank and side — the warning ends and the attacker's entire route is
cancelled immediately, at that same instant. This is never deferred to the
attacker's own charge completion: the attacker does **not** advance onto the
vacated square once its charge finishes, and every step queued behind the
cancelled attack is cancelled with it. A step already completed before the
cancellation, and any step ordered before it, are unaffected.

#### REQ: occupied-destination-cancels-route

An ordinary planned move — one with no attack target of its own — fails at
the moment it would resolve if its destination has since become occupied,
by an ally, a different enemy, or another unit whose own simultaneous move
settled there first. The charging piece stays on its current square, its
live charge indicator ends, and its complete remaining route is cancelled
along with it, exactly as `changed-target-cancels-route` cancels an
attacker's route when its specific target changes.

#### REQ: alerted-flee-discount

A piece currently carrying an active attack target-lock reacts faster than
usual: while the warning is live, a move command for that piece — fleeing,
not attacking — charges at a reduced rate, a configurable percentage taken
off that piece's ordinary move-charge time. The default discount is **10%**;
a match may configure it anywhere from 0% (no discount — off) to 100% (an
alerted piece moves instantly). The discount is applied to the base
move-charge time before any [fatigue](../fatigue/README.md) charge-slowing
is added on top. It never applies to an attack command, only to moving away,
and it stops the instant the warning ends — whether because the piece
escaped, the attack resolved, or the attack was cancelled.

#### REQ: rest-before-attacking

As of the **balance-v7** profile, committing an attack additionally
requires the acting piece to be fully rested: its
[fatigue](../fatigue/README.md) must be at zero before an attack charge may
start. A command that is otherwise legal but issued while the piece is
still fatigued does not fail — it queues, the piece visibly rests on its
square, and its attack charge begins automatically the instant fatigue
reaches zero, at an instant already fixed the moment the order was
accepted. This applies to every attack, including a plain Kill. Two attacks
are exempt and always begin immediately regardless of fatigue: capturing
the enemy king and intercepting a loaded enemy convoy.

### Classical movement baseline

#### REQ: classical-movement

Ordinary piece movement, pawn direction, the pawn's initial double step, path
blocking, and capture geometry all follow classical chess. Check, checkmate,
and stalemate do not exist: a king may move into or remain on an attacked
square, and the match never ends merely because a king is under attack. A
king fights, moves, and gets captured like any other piece — see
[Prisoner Convoys](../prisoner-convoys/README.md) for what capturing a king
actually does.

#### REQ: castling

Castling is a single command: select the king with destination g1/c1 (White)
or g8/c8 (Black). It is legal when neither the king nor the chosen rook has
ever moved, both are on the board and able to act, the squares between them
are free, and neither is busy with something else. There is no check concept,
so attacked squares along the way are irrelevant. King and rook both charge
on their starting squares and then move together the instant the charge
completes.

#### REQ: en-passant

A pawn that completes a double-step move becomes capturable en passant for a
2-second real-time window. Within that window, an enemy pawn on an adjacent
file may capture it onto the square it passed over, with ordinary en-passant
geometry. The window closes early if the double-stepped pawn moves, is
captured, or is otherwise transformed before the window expires.

#### REQ: promotion

A pawn move onto the last rank must carry its promotion choice — queen, rook,
bishop, or knight — as part of the same command; there is no separate
"awaiting promotion" state. The promoted piece keeps its history, and from
that point on it moves, charges, and counts for every rank-based rule as its
new rank.

#### REQ: declining-promotion

A pawn reaching the last rank by taking a prisoner may instead choose to
decline promotion and stay a pawn. This choice exists only for a
prisoner-taking capture: that capture already turns the pawn into a
[convoy](../prisoner-convoys/README.md) escort, and a loaded convoy can
travel sideways and back toward its own home rank — so a pawn that declines
still has somewhere to go, can unload its cargo, and remains a trainable
pawn afterward with any [Pawn Specialisation](../pawn-specialisation/README.md)
trade and veteran status intact. An ordinary quiet push onto the last rank,
an explicit Kill, or a Recruit Informer — none of which creates a convoy —
offer no such choice: a pawn declining there would be left standing on the
last rank with no legal move ever again, so promotion stays mandatory for
every case but a prisoner-taking capture.

## Acceptance Criteria

### AC: charge-then-resolve

**Given** a legal move is commanded for a piece
**When** the server accepts it
**Then** the piece remains on its starting square for its full charge
duration, showing an upward-filling indicator bound to the piece, and only
moves once that charge completes.

### AC: attack-charges-slower-than-move

**Given** a piece has both a move charge and an attack charge configured in
the active rules preset
**When** the same piece is commanded to move to an empty square, and
separately commanded to attack
**Then** the attack takes at least as long to charge as the move, matching
the two distinct values recorded for that piece and preset.

### AC: alerted-piece-flees-faster

**Given** a piece is carrying an active attack target-lock and the match's
flee-discount setting is at its 10% default
**When** a teammate commands that piece to move away
**Then** the move's charge time is 10% shorter than the piece's ordinary
move charge, and an attack command from that same piece is unaffected by
the discount.

### AC: flee-discount-is-configurable

**Given** a match configures the flee-discount setting to 0%, and a separate
match configures it to 100%
**When** an alerted piece is commanded to move away in each match
**Then** the 0% match charges the move at the ordinary rate with no
discount, and the 100% match resolves the move instantly.

### AC: fatigued-attack-queues-then-charges

**Given** a piece carrying fatigue is ordered to attack
**When** the command is accepted
**Then** no attack charge starts immediately; the piece rests visibly until
its fatigue reaches zero, and its attack charge then begins on its own at
that deterministic instant.

### AC: player-interval-is-separate

**Given** a player has just issued one command
**When** they attempt to issue another command for a different unit before
their 1-second interval has elapsed
**Then** the second command is paced by the player interval regardless of
either unit's own charge time.

### AC: route-markers-are-team-private

**Given** a player plans an ordered route with three destinations
**When** a teammate and an opposing player each view the board
**Then** the teammate sees the numbered rings and cumulative progress while
the opponent sees no destination, order, progress, or identity of the moving
piece.

### AC: overlapping-plans-stand-out

**Given** one teammate already has a route targeting a square
**When** a second teammate plans a different route to the same square
**Then** the team's shared marker on that square becomes visibly bolder and
shows an overlap count, without exposing either plan to the opponent.

### AC: replacing-a-queued-route

**Given** a piece has an existing queued route and live charge
**When** a teammate selects it and commits a different, currently legal move
**Then** the old route and charge are cancelled, all of its markers vanish,
and the new move becomes step 1 of a fresh route.

### AC: attack-warning-is-anonymous

**Given** an attack begins charging against a piece
**When** both teams view the board
**Then** both see that the piece is under attack, but neither the identity of
the attacker, its origin, nor its route is ever revealed by that warning.

### AC: changed-target-cancels-the-route-immediately

**Given** an attack is charging toward a square
**When** the exact targeted piece legally moves away, is captured, or is
replaced by a different piece before the charge completes — even a piece of
the same rank and side
**Then** the warning ends and the attacker's entire route is cancelled at
that same instant; the attacker remains on its current square and never
advances onto the square, even once its charge would otherwise have
completed and even if the square is empty by then.

### AC: occupied-destination-cancels-an-ordinary-move

**Given** a piece is charging toward a square with no attack target of its
own, and later route steps are queued behind it
**When** an ally, a different enemy, or a simultaneous arrival occupies that
square before the charge resolves
**Then** the move fails, the piece remains on its current square, and its
complete remaining route is cancelled.

### AC: king-can-stand-under-attack

**Given** a king is on a square that an enemy piece threatens
**When** a player considers moving the king there or leaving it there
**Then** the move is legal — there is no check, checkmate, or stalemate rule
that forbids it.

### AC: castling-is-one-command

**Given** neither the king nor the chosen rook has moved, both are able to
act, and the squares between them are free
**When** the player commands castling
**Then** both pieces charge on their own squares and then move together the
moment the charge completes, with no check-related restriction applied.

### AC: en-passant-window-closes

**Given** a pawn has just completed a double-step move
**When** an adjacent enemy pawn captures it en passant within 2 seconds
**Then** the capture is legal; **when** instead 2 seconds pass without a
capture, or the pawn moves or is captured first, the window is closed and the
en-passant capture is no longer available.

### AC: promotion-choice-travels-with-the-move

**Given** a pawn moves onto the last rank
**When** the move is committed
**Then** it carries its promotion choice in the same command, and the
resulting piece immediately moves, charges, and counts as its new rank.

### AC: declining-promotion-only-works-on-a-capture

**Given** a pawn takes a prisoner in a move that lands it on the last rank
**When** its player declines promotion
**Then** the pawn stays a pawn, becomes a convoy escort carrying that
prisoner, and can move sideways or back toward its own home rank; **given**
instead the same pawn reaches the last rank by a quiet push, a Kill, or a
Recruit Informer
**Then** no decline option is offered and a promotion choice is required.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
