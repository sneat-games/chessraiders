---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Fog of War

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fog-of-war?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fog-of-war?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fog-of-war?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/fog-of-war?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

Classic matches show the whole board to everyone and remain the default.
Shared Fog of War is an optional per-match variant where each team shares one
view built from its own pieces' vision — every enemy unit is visible only
where a friendly piece could currently see or reach it. Two exceptions keep
early play readable and keep command itself honest: the enemy king and queen
start visible until they first move, and both teams' morale is always public.

**Mechanic:** the server projects one team-shared visible/unknown/ghost board
and exposes only public physical state for currently visible enemies.
**Real-world analogy:** scouts can report that a visible enemy is preparing
something and how soon it will happen, but not the sealed destination on its
orders; an old sighting cannot radio live updates.

## Problem

Full information turns Chess Raiders into a puzzle where every plan is
already known to the opponent, which undercuts the anonymous target-lock
warning and the whole point of covert informers. Fog needed to add real
uncertainty about where the enemy actually is without becoming unreadable —
players must always be able to tell "visible," "never seen," and "seen once,
maybe gone" apart at a glance.

## Behavior

### Variants

#### REQ: fog-variants

Visibility is a per-match setting chosen before the match starts. **Classic**
gives both teams full visibility of the board and is the default. **Shared
Fog of War** replaces that with team-shared, vision-limited viewing as
described below. Both variants use the same underlying rules engine — fog
only changes what a team is shown, never what is actually legal.

### Team-shared vision

#### REQ: shared-team-vision

Under Shared Fog, vision belongs to the team, not to the individual player:
every player on a side sees exactly the same board, built from the combined
vision of every one of that side's own pieces.

#### REQ: patrol-vision

Each of your pieces sees: its own square; all eight squares immediately
around it (including squares holding convoys or refitting pieces); and every
square it could currently move to. A sliding piece (bishop, rook, queen)
sees along its open lines up to and including the first piece blocking that
line — so parking a piece across a corridor blinds your own view past it as
much as it blinds the enemy's. A knight sees its jump squares regardless of
what stands between. An unmoved pawn sees its double-step square only when
the single step ahead of it is empty.

Attempted moves are always checked against the true board, not against what
you can see — probing into the fog with a move attempt is legitimate
reconnaissance, though a move rejected because of something unseen names no
more than that something is in the way.

### Opening deployment reveal

#### REQ: opening-deployment-reveal

Each enemy king and queen starts visible on its initial square, even outside
your patrol vision, until that piece completes its first successful move.
Charging a move, or having a move fail or get cancelled, does not end this —
only an actual completed relocation does, including via castling or an
attack. Once that first move resolves, the exception is permanently spent
for that king or queen: returning to its original square later never brings
it back. From then on it is visible only through your ordinary team vision
or an active informer's vision (see [Espionage](../espionage/README.md)).
Every other enemy piece is hidden from the start unless your vision actually
observes it.

### What stays public regardless of fog

#### REQ: fog-public-information

Some information is never hidden by any fog variant:

- **Morale** — both teams' current morale is always visible; see
  [Morale](../morale/README.md).
- **King incursion** — the moment an enemy king crosses onto your half of
  the board, or leaves it, is announced to both teams even though the king's
  exact square stays hidden as usual; see [Morale](../morale/README.md).
- **Command totals** — both teams' captive-pawn counts and active-informer
  counts are always visible, without revealing which enemy pieces are
  informers; see [Espionage](../espionage/README.md).
- **Under-attack warnings** — an anonymous target-lock on a threatened piece
  is visible to both teams; see [Real-Time Command](../real-time-command/README.md).
- **Interrogation** — while a king is interrogating an adjacent piece, both
  engaged squares and the remaining time are public; see
  [Espionage](../espionage/README.md).

### Reading the fogged board

#### REQ: fog-square-states

The board always shows three clearly different states for a square, never
confusable with one another: **visible** (the live truth right now);
**unknown** (never seen — must never be mistaken for a visible empty
square); and a **ghost** (fogged now, but you saw an occupant there before —
shown as stale and possibly wrong). A ghost is always based on what you
actually saw; it never reveals an enemy piece's live charge state, its
ordinary route plan, or any cargo change that happened while it was out of
your sight.

#### REQ: visible-enemy-charge-progress-without-target

A currently visible enemy piece reveals whether it is actively charging and,
when it is, the remaining milliseconds. It never reveals the destination,
queued route or attack target. Your own charging piece shows both its current
destination and remaining time. An invisible enemy and a ghost expose no live
charge fields at all; becoming visible again resumes the current status rather
than replaying what happened while hidden.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)

## Acceptance Criteria

### AC: classic-shows-everything

**Given** a match is started with the Classic variant
**When** either team views the board
**Then** every piece, on both sides, is fully visible at all times.

### AC: shared-fog-limits-view-to-team-vision

**Given** a match uses Shared Fog of War and a White player checks the board
in the starting position
**When** the view is produced
**Then** the Black king and queen are visible on their initial squares even
outside White's patrol vision, every other Black piece outside White's
patrol or informer vision is shown as unknown, and both morale values are
present.

### AC: opening-reveal-is-spent-after-first-move

**Given** a queen sits on its initial square outside the enemy's ordinary
vision in Shared Fog
**When** it completes its first move and later returns to that same initial
square, still outside ordinary vision
**Then** the enemy sees it on its initial square only up until the first move
resolves, never learns its hidden destination, and does not see it again
when it returns home.

### AC: blocking-a-line-blinds-your-own-view

**Given** a sliding piece has an open line of sight down a file
**When** a piece — friendly or enemy — moves into that file between the
slider and the far end
**Then** the slider's vision along that file stops at the blocking piece,
exactly as it would for movement.

### AC: unknown-versus-ghost-versus-visible

**Given** a square you have never seen, a square you can currently see, and a
square you saw an enemy occupy before it left your vision
**When** the board is rendered
**Then** the three squares are shown in three visually distinct states, and
the ghost square never displays live charge, route, or cargo information.

### AC: probing-is-legal-and-stays-vague

**Given** a player commands a move toward a square hidden from their team
**When** the move turns out to be illegal because of what is actually there
**Then** the rejection does not reveal what occupies the square or who owns
it.

### AC: visible-charge-progress-does-not-reveal-the-route

**Given** an own charging piece, a currently visible charging enemy piece,
and a ghost of another enemy that is charging out of sight
**When** the team's fogged view is produced
**Then** the own piece shows destination and remaining time, the visible enemy
shows active status and remaining time but no destination, and the ghost shows
no live charge status or timing

### AC: morale-and-incursion-stay-public-under-fog

**Given** a Shared Fog match where an enemy king has just crossed onto a
team's home half
**When** either team views the board
**Then** both teams' morale values are shown and both receive the incursion
announcement, even though the king's exact square remains hidden as usual.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
