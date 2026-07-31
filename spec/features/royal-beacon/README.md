---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Royal Beacon

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/royal-beacon?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/royal-beacon?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/royal-beacon?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/royal-beacon?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

The Royal Beacon carries the king's authority to the front. It starts on the
king, and any adjacent friendly piece can ask to carry it forward, forming a
living chain of neighbouring friendly units back to the king. While that
chain holds, every friendly piece near the Beacon's carrier charges its
moves a little faster, scaled by the army's own morale and by how close a
piece stands to the carrier. Break the chain and the benefit stops at once;
reconnect it and the benefit returns immediately — there is no timer, no
stored path, and no separate resource spent to use it.

## Problem

Classical chess rewards where you stand. Chess Raiders wanted to also reward
whether your army can still act *as* an army — whether the king's orders can
actually reach the fighting. Without something like the Beacon, a formation
that gets cut off from its king suffers no consequence beyond the pieces
already lost; with it, keeping a supply line of adjacent friendly pieces
back to the king becomes a real, visible, fightable objective in its own
right.

## Vision

Chess Raiders already gives an army leadership through
[Morale](../morale/README.md), logistics through
[Prisoner Convoys](../prisoner-convoys/README.md), and intelligence through
[Espionage](../espionage/README.md) and [Fog of War](../fog-of-war/README.md).
The Royal Beacon is the fourth pillar: communication and command. It asks
one more strategic question of every position — *can the king's will still
reach the front?* — on top of the classical question of where each piece
stands.

Without the Beacon, pieces still move, fight, and hold their ground; they
simply act on their own initiative rather than as a coordinated force. A
lowly pawn holding the one square that keeps the chain intact can matter
more, for a moment, than a queen standing just outside the network. A king
may need to step forward — not to attack, but to shorten the distance his
own orders have to travel.

The Royal Beacon is deliberately never described or shown as a generic
"buff" — it is a position on the board that has to be built, carried,
defended, and sometimes sacrificed.

## Behavior

### The Beacon and its bearer

#### REQ: one-beacon-per-side

Each side has exactly one Royal Beacon, owned by its king. At any moment it
is in one of five states: **Dormant** (not yet deployed), **Deployed** (on a
bearer, possibly connected), **Recalling**, **Lost**, or **Restoring**. It is
not a combat unit, has no square of its own separate from its bearer, cannot
be captured on its own, and spends no morale or other resource to use.

#### REQ: deploy-and-abandon

Only the active king may deploy a Dormant Beacon — this places it on the
king itself, who becomes its first bearer. A side may also explicitly
abandon a Deployed Beacon from its current bearer at any time, which
immediately removes all of its benefit and sends it to Lost. Both actions
use only the acting player's ordinary command interval; neither starts a
move charge.

#### REQ: bearer-cannot-relocate

While a piece or convoy is carrying the Beacon, it cannot move, attack,
capture, escort, intercept, or castle — it must first have the Beacon taken
by an adjacent ally, recalled by the king, or abandoned. This applies
equally to an ordinary piece, a king, or a convoy currently serving as
bearer.

### Passing the Beacon

#### REQ: receiver-initiated-take

Passing the Beacon is always initiated by the receiver, never pushed by the
current bearer: select a friendly piece adjacent to the bearer — any of the
eight neighbouring squares — and choose to take the Beacon. The king can
receive it the exact same way, by an adjacent piece choosing to hand it to
him. Taking the Beacon uses only the acting player's ordinary command
interval; it starts no move charge on either the giver or the receiver, and
does not disturb either piece's existing charge or plans.

### The chain of command

#### REQ: eight-neighbor-chain

While Deployed, the chain is recalculated live from the current board: two
friendly active pieces on any of the eight neighbouring squares to each
other are linked. A live Chain of Command exists exactly when the active
king and the current bearer sit in the same connected chain of such links —
diagonal neighbours count exactly like orthogonal ones. There is no chosen
path and nothing is remembered between moments: if the board can connect
them, they are connected, however many alternate routes exist.

An on-board convoy can relay or carry the Beacon under the same rules as any
other friendly piece; its cargo cannot. A captured enemy king riding as
cargo is off the board for this purpose and cannot relay or bear anything. A
recruited informer still relays for its own original side only —
recruitment never changes who it fights for.

#### REQ: chain-breaks-and-restores-live

The instant any move, capture, or other change breaks every remaining path
between king and bearer, the Beacon's benefit stops immediately, with no
grace period — the Beacon stays Deployed, it simply becomes inactive. The
instant a friendly piece restores a path, the benefit resumes immediately.

### The effect

#### REQ: effect-radius

While the chain is active, every non-king friendly piece within Chebyshev
distance 3 of the bearer (the bearer itself is distance 0) is close enough
to benefit — whether or not that piece is itself part of the chain. Pieces
farther than distance 3 get nothing, and the king himself never benefits
from his own Beacon, even standing right next to it.

#### REQ: charge-reduction

For an eligible piece beginning a move, the full-strength reduction is the
side's current [morale](../morale/README.md) times 100 milliseconds. That
reduction falls off with distance from the bearer: 100% of it applies at
distance 0, 75% at distance 1, 50% at distance 2, and 25% at distance 3. The
resulting charge time is never allowed to drop below half of the piece's
normal base charge time, however high morale climbs. This is evaluated the
moment a piece's move is accepted — a relay piece that then breaks the chain
by moving still keeps the reduction it already locked in for that move.
Neither the player's command interval nor any channelled action (training,
interrogation, and the like) is ever reduced by the Beacon.

### Recall, loss, and restoration

#### REQ: king-recall

Only the active king may recall a Deployed Beacon, even through a currently
broken chain — this is the recovery action. Recall uses only the player's
ordinary command interval and does not stop the king from acting normally.
After a 2-second delay, the Beacon redeploys on the king's current square.

#### REQ: bearer-loss

If the current bearer is captured, killed, intercepted, or otherwise removed
from the board, the Beacon becomes Lost immediately and every benefit it was
providing ends in the same instant. Recruiting the bearer as a secret
informer does not remove it from the board, so it does not lose the Beacon
that way.

#### REQ: king-restore

Only the active king can restore a Lost Beacon, by channelling for 7
uninterrupted seconds. The king is visibly engaged for that whole time and
cannot act without cancelling the restoration; completing another action
cancels it and leaves the Beacon Lost, while an illegal attempted action
does not. Once restoration completes, the Beacon redeploys on the king.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)
- [Morale](../morale/README.md)

## Acceptance Criteria

### AC: king-deploys-first

**Given** a side's Beacon is Dormant
**When** the active king deploys it
**Then** the king becomes its bearer and the Beacon becomes Deployed, using
only the king's ordinary command interval.

### AC: receiver-takes-without-a-move-charge

**Given** a Deployed Beacon is borne on one square and an eligible friendly
piece stands adjacent to it
**When** that piece's player chooses to take the Beacon
**Then** it becomes the new bearer, no move charge begins on either piece,
and only the acting player's command interval is spent.

### AC: bearer-must-transfer-recall-or-abandon-before-moving

**Given** a piece currently bears the Beacon
**When** its player attempts to move, attack, capture, escort, intercept, or
castle with it
**Then** the action is rejected until the Beacon has been taken by an
adjacent ally, recalled by the king, or abandoned.

### AC: chain-breaks-and-restores-immediately

**Given** an active chain links the king and the bearer through one or more
friendly pieces
**When** the last connecting piece leaves the chain
**Then** the Beacon's benefit stops immediately; **when** a friendly piece
later restores any path between king and bearer
**Then** the benefit resumes immediately, with no timer involved either way.

### AC: benefit-follows-radius-not-chain-membership

**Given** an active Beacon, one non-relay friendly piece inside its radius,
and one friendly relay piece outside its radius
**When** each begins an otherwise identical move
**Then** only the in-radius piece receives the charge reduction, regardless
of whether it is part of the connecting chain.

### AC: reduction-scales-with-distance-and-floors-at-half

**Given** eligible friendly pieces standing at distances 0, 1, 2, and 3 from
an active bearer, all at the same current morale
**When** each begins a move
**Then** they receive 100%, 75%, 50%, and 25% of the morale-based reduction
respectively, and none of their charge times drops below half of their
normal base charge.

### AC: recall-redeploys-on-the-king

**Given** a Deployed Beacon with a currently broken chain
**When** the king recalls it
**Then** the Beacon provides no benefit for 2 seconds and then redeploys on
the king's current square.

### AC: removed-bearer-loses-the-beacon

**Given** a piece currently bears the Beacon
**When** that piece is captured, killed, or otherwise removed from the board
**Then** the Beacon becomes Lost immediately and every benefit it was
providing ends in the same instant.

### AC: king-restores-over-seven-seconds

**Given** a Lost Beacon
**When** the king channels an uninterrupted 7-second restoration
**Then** the Beacon redeploys on the king; **when** the king instead
completes another action during that channel
**Then** the restoration cancels and the Beacon remains Lost.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
