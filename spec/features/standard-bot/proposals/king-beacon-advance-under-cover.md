# Proposal: The king and the Beacon advance under cover

**Status:** Draft
**Type:** change-request
**Targets:** standard-bot
**Date:** 2026-08-11
**Owner:** alex
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we make the bot advance its king and Beacon bearer behind a screen of friendly pieces, so neither ever stands ahead of its own formation?

**Mechanic:** safe leader moves are admitted and scored by formation support,
current morale need, and a one-ply anti-reversal guard.
**Real-world analogy:** a commander and signal officer move with their line,
building a small reserve of command capacity instead of leading a reckless
charge.

## Context

The superseded pre-fix measurement found repeated board positions and passive
opponent stalls across all tiers. Those historical counts are diagnostic
evidence, not the current acceptance result: the completed behavior is gated
by a fresh 75-seed run for each of Recruit, Lieutenant and Commander after the
observation provider and public script are integrated. The root design issue
remains the founder's instruction from 2026-08-11: “King should not be a
hero.”

## Recommended Direction

Scale the leader reward by a saturating measure of friendly formation support
ahead of the destination, so advancing into empty space earns nothing. The
destination must also be visible, guarded and unthreatened in the host's
`candidates` facts. Apply the same discipline to the current Beacon bearer,
including regrouping when a move improves its formation support. Beacon
aggression itself is the bot permission: `0` disables all Beacon actions and
a positive value enables and scores them, always beneath the match's Beacon
rule gates.

The founder's model, in his own four statements:

1. There is no point in the king advancing if no covering pieces are in front of him.
2. The more pieces in front of him, **and** the farther he is behind the front line, the more the forward move should be rewarded.
3. Advancing with fewer pieces should reward less.
4. Pieces farther from the king contribute less cover than near ones.

The same discipline applies to the **Beacon bearer**, not only the king. Both are pieces whose loss is decisive, so both move up behind a screen rather than into space.

The shape this takes in scoring:

```
support       = SUM over friendly p ahead of the destination of  1 / chebyshev(p, destination)
supportFactor = min(1, support / LEADER_SUPPORT_SATURATION)
advance       = gain * MORALE_PUSH_VALUE * moralePush * supportFactor
```

- **"ahead"** means greater `forward_progress` than the king.
- **Chebyshev** distance, not rank difference, so a piece far off to the side decays as cover rather than counting fully.
- **The saturation is load-bearing.** Without `min(1, …)` the term is unbounded and the 0–1 aggression dial stops meaning anything. Formation support only modulates downward; it never multiplies the tier dial above its stated value.

The king is scheduled into shallow breadth while it has less than one point of
reserve over current visible capture/managed-work need, and advances no farther
than two reserve points. At three or more excess points, further advance is
penalised and a safe retreat is scheduled so long as it keeps at least one
reserve point. The Beacon bearer receives breadth priority only for a safe
quiet move that actually improves formation support.

After either leader makes a quiet move, a bounded placement fingerprint blocks
the exact immediate reverse while all other pieces remain unchanged. Threat
escape, captures, promotion and other tactical state changes release it.

## Alternatives Considered

**Make the existing king-advance term symmetric and stop there.** This does
resolve the simplest oscillation, but it answers *“should the king give back
ground?”* without answering *“should the king have advanced at all?”* A
symmetric term still pays a lone king to walk into open space.

**Gate king advance on a hard rule** — for example, forbid it whenever the king would become the most advanced friendly piece. Simple, legible, and enforces the founder's "never in the first line of attack" exactly. It lost because a hard gate cannot express *degree*: it treats one distant covering piece the same as a full screen, and gives the tiers no dial to differ on.

**Scale the penalty by two-move enemy attack pressure.** The best model of the three — a king that backs off *before* it is attacked rather than once it already is. Deferred rather than rejected; see Not Doing.

## MVP Scope

Formation-scaled king and current-Beacon-bearer movement, using the new
`pieces` relation graph and host-projected `candidates`, an exact one-ply
anti-reversal guard, a three-square ordinary quiet-cycle ring, and the
0.3/0.6/0.9 Recruit/Lieutenant/Commander aggression dials (Adviser 1.0 for
Beacon, its existing 0.5 for morale). The old 0.2/0.4/0.8 proposal values are
superseded by this founder decision.

## Not Doing (and Why)

- A two-move enemy attack map — the host supplies only exact current and
  deterministic one-move facts
- Approximate distance-to-king pursuit — a nearby rook or knight does not
  necessarily attack the king; only exact `nextPossibleMoves` membership earns
  the fixed +30 king-hunt value
- A full visited-position history inside the script — the external 75×3 gate
  owns strict repeat detection; the script retains only bounded local cycle
  state

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | Formation scaling still lets every tier create the one-to-two point morale reserve needed to capture | Run the paired provider/script build at exactly 75 seeds × 3 tiers; every match must capture and deliver the king without a repeated placement |
| Must-be-true | The script reads support and safety from the final observation contract | Confirm `pieces` is the sole board shape and `candidates` provides destination visibility plus relation lists; no `own`, `enemy`, `danger` or scalar move-fact fallback remains |
| Should-be-true | Anti-reversal and the quiet ring leave tactical necessities open | Mutation-sensitive decisions cover threat escape, capture, promotion, delivery and forced/no-alternative moves alongside the measured rook and promoted-queen cycles |
| Should-be-true | The strategy stays inside Recruit's public limits | Representative measurement is 34,189/250,000 interpreter steps; worst-case action + move + leader guard + quiet ring is 31/32 integer memory entries |


## SpecScore Integration

- **New Features this would create:** none — this changes how `standard-bot` scores king movement
- **Existing Features affected:** `standard-bot`; `royal-beacon` and `morale` supply the rules this interacts with
- **Dependencies:** the paired private observation producer must emit the
  final `pieces`/`candidates` contract before the public corpus and passive
  acceptance run can be regenerated

## Open Questions

None. Recruit, Lieutenant and Commander use the shared saturation constant and
their approved 0.3/0.6/0.9 morale and Beacon weights; zero remains disabled.
