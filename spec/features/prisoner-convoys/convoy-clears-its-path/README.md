---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Convoy Clears Its Path

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/prisoner-convoys/convoy-clears-its-path?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/prisoner-convoys/convoy-clears-its-path?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/prisoner-convoys/convoy-clears-its-path?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/prisoner-convoys/convoy-clears-its-path?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

A loaded convoy is not defenceless. When its one legal step — sideways or
toward its own home — would land on an enemy piece, the escort may attack
it instead of being blocked outright, choosing Kill, Capture, or Recruit
Informer exactly like any other piece. Out in the open field a convoy still
cannot fight its way sideways through a whole enemy rank; that sideways
option only opens up once the convoy has actually reached its own home
ground.

## Problem

A loaded convoy's own movement is already the most restricted on the board —
one square at a time, never toward the enemy. Without this feature, an enemy
piece parked on that one remaining legal square denied the convoy any way
forward at all, including the one square that would have delivered a
captured king and won the match. A single stalling piece could make delivery
permanently unreachable, with no counterplay for the side that did the
capturing. At the same time, an unconditional convoy attack would let a
loaded convoy walk sideways down an entire enemy rank, taking one piece per
step with no risk of ever advancing into danger itself — so the sideways
half of this rule only opens up once the convoy is already home.

## Behavior

### Which square a convoy may attack

#### REQ: convoy-attack-targets-its-one-legal-destination

A loaded convoy may submit a move whose destination is occupied by an enemy
piece, provided that destination is already a legal step for it — one
square, sideways or toward its own home, with no wall in the way. A
destination toward the enemy side stays illegal regardless of whether it is
empty or occupied; attacking a blocker is never a new direction, only the
convoy's one existing legal step, now allowed to land on an enemy instead of
being rejected outright.

#### REQ: sideways-attacks-wait-for-home-ground

A sideways attack — as opposed to a sideways quiet move onto a free square,
which is always legal — is only offered once the convoy has reached its own
home ground: its own pawn-starting rank, or further back on its own back
rank. Strictly ahead of that, in the open field, a convoy may still step
sideways onto a free square but may not attack sideways; an enemy blocking a
field square sideways of the convoy simply has to be gone around or
defeated some other way. A homeward attack — one square closer to the
convoy's own side — is always legal regardless of where the convoy stands,
because advancing toward home is never the scoop this restriction exists to
prevent.

### Choosing how the attack resolves

#### REQ: convoy-attack-offers-kill-or-capture

When a convoy's one legal destination holds an enemy piece that is not
itself a convoy and not the enemy king (kings are always captured, never
killed — unchanged), the attack offers the same Kill/Capture choice, gated
by the same match rules, that any other attacker's would. Kill is
deterministic. Capture is deterministic or risky exactly as
[Capture Outcomes](../../capture-outcomes/README.md) already governs for any
other attacker of the escort's own rank — there is no separate formula for a
convoy.

#### REQ: convoy-attack-offers-recruit-too

Recruit Informer is offered on the same terms as any other attacker's,
whenever the match allows it. It is the one choice that does not clear the
square — recruitment never displaces the target — so a convoy that recruits
its blocker is still blocked by it afterward and may simply attack again
later with Kill or Capture. Choosing to recruit the blocker rather than
clear it is the player's call: an informer's vision, and the drag it puts on
its own side, may be worth more right now than one open square.

#### REQ: every-match-allows-a-way-through

Every match enables at least one of Kill or Capture. A ruleset offering only
Recruit Informer would leave a blocked convoy with no outcome that could
ever actually clear its own path — reintroducing the exact dead end this
feature exists to remove.

### What happens to the convoy and its cargo

#### REQ: repelled-attack-leaves-everything-in-place

A repelled attack changes nothing but the recovery lock: neither piece
moves, the blocker is not removed, and the convoy's entire cargo — the
captured king and every captive it already carries — is untouched. Both the
convoy and the blocker take the ordinary capture-risk recovery penalty, and
either side may try again once it ends.

#### REQ: successful-attack-clears-and-relocates

A Kill, or a Capture that resolves as success or as the defender killed,
removes the blocker and moves the convoy onto the now-cleared square — the
same relocation any other successful attacker gets. The convoy stays a
convoy throughout; nothing about winning this fight turns it back into an
ordinary piece.

#### REQ: existing-cargo-survives-a-new-capture

A convoy that already carries a captured king, captives, or both keeps
everything it was carrying when a new attack succeeds: the newly captured
piece joins the cargo it already has, rather than replacing it. No captive
or king already aboard is ever dropped by a convoy winning a fight to clear
its own path.

### Attacking another convoy, and rescuing your own king

#### REQ: convoy-vs-convoy-attack-stays-deterministic-interception

When the blocking enemy piece is itself a loaded convoy, the attack is
always interception — the same unconditional rule that applies whenever any
piece captures onto a loaded convoy's square (see
[Prisoner Convoys](../README.md)'s `convoy-interception`). No Kill/Capture/
Recruit choice is offered or consulted; the attacker's own existing cargo
merges with the demoted enemy escort and everything it was carrying.

#### REQ: rescuing-your-own-king

If the piece a convoy attacks — or any other piece captures — turns out to
be an enemy convoy escorting *your own* previously captured king, that king
is released immediately onto the intercepted square as an ordinary active
piece of its own side, rather than being taken into the rescuer's cargo. A
king can be on the board or riding as exactly one convoy's cargo, never
both and never queued behind another, so the rescuer itself does not
relocate onto that square — the freed king is standing there instead — and
any cargo the rescuer already carried is untouched. Only an *enemy* king
taken by an ordinary capture becomes cargo; a friendly king freed this way
simply rejoins play.

### Winning in the same move

#### REQ: clearing-a-delivery-square-can-win-immediately

If a convoy's cleared destination is also an eligible delivery square for
whatever it is carrying — a royal square for a captured king, its own
pawn-starting rank for an ordinary captive — the same command both clears
the blocker and completes the delivery, per
[Prisoner Convoys](../README.md)'s `cargo-based-delivery`. Clearing the one
square that was standing between a convoy and victory wins the match in
that same move, with no separate follow-up required.

## Dependencies

- [Prisoner Convoys](../README.md)
- [Capture Outcomes](../../capture-outcomes/README.md)
- [Espionage](../../espionage/README.md)

## Acceptance Criteria

### AC: sideways-step-onto-an-enemy-becomes-an-attack

**Given** a loaded convoy stands with an enemy piece directly sideways of it,
on its own home ground
**When** the escort's player commands a move onto that square
**Then** the engine offers the attack choice instead of rejecting the
command outright.

### AC: homeward-step-onto-an-enemy-becomes-an-attack

**Given** a loaded convoy stands one homeward step from an eligible delivery
square, currently held by an enemy piece
**When** the escort's player commands a move onto that square
**Then** the engine offers the attack choice instead of rejecting the
command outright, regardless of how far from home the convoy currently
stands.

### AC: forward-square-still-blocks-regardless-of-occupant

**Given** a loaded convoy stands with an enemy piece directly ahead of it,
toward the enemy side
**When** the escort's player commands a move onto that square
**Then** the command is rejected exactly as it would be against an empty
square, and no attack is offered.

### AC: sideways-attack-in-the-field-is-not-offered

**Given** a loaded convoy stands strictly ahead of its own pawn-starting
rank, with an enemy piece directly sideways of it
**When** the escort's player commands a move onto that square
**Then** the command is rejected — a convoy still travelling through the
field may step sideways onto a free square but may not attack sideways
there.

### AC: kill-or-capture-resolves-like-any-other-attacker

**Given** a match allowing Kill, and a loaded convoy attacking a blocker with
Choice Kill
**When** the command resolves
**Then** the blocker is removed, the convoy relocates onto its square, and
its existing cargo is unchanged; **given** instead the match enables only
Capture with risk on
**Then** the same attack draws from the same success/defender-killed/
repelled odds any other attacker of the escort's rank would get.

### AC: recruit-leaves-the-blocker-standing

**Given** a match enabling Recruit Informer and a loaded convoy attacking a
blocker with Choice Recruit
**When** the command resolves
**Then** the recruitment succeeds exactly as it would for any other
attacker, the blocker stays on its square, and the convoy remains blocked
until it attacks again.

### AC: repelled-attack-changes-nothing-but-the-recovery-lock

**Given** a loaded convoy's attack on a blocker resolves as repelled
**When** the outcome applies
**Then** the convoy stays on its own square with its cargo untouched, the
blocker stays on its own square, and both take the ordinary recovery
penalty.

### AC: existing-cargo-survives-a-new-capture

**Given** a convoy already carrying a captured king and one captive attacks
and successfully captures its blocker
**When** the command resolves
**Then** the convoy's cargo afterward holds the original king, the original
captive, and the newly captured piece — none of the earlier cargo is
replaced or dropped.

### AC: attacking-a-blocking-convoy-is-always-interception

**Given** a loaded convoy's one legal destination holds an enemy loaded
convoy
**When** the escort's player commands a move onto that square with any
choice at all
**Then** the interaction resolves as ordinary interception, exactly as it
would for a fresh attacker, and the choice submitted is not consulted.

### AC: rescuing-your-own-king-releases-it-onto-the-board

**Given** an enemy loaded convoy is escorting a piece's own side's
previously captured king, and a friendly piece — convoy or otherwise —
captures onto that convoy's square
**When** the capture resolves
**Then** the rescued king is released onto that square as an ordinary active
piece of its own side, the attacker does not relocate onto the square, and
any cargo the attacker already carried is unaffected.

### AC: clearing-the-last-blocker-can-win-in-the-same-move

**Given** a loaded convoy carrying the captured enemy king stands one legal
step from a royal square currently held by an enemy piece
**When** the escort's player attacks and clears that piece
**Then** the convoy lands on the royal square in that same command and the
match ends immediately in its side's favour.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
