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

## Context

The standard bot cannot beat a passive opponent: Recruit and Lieutenant won 0 of 75 matches, Commander 13 of 75, and every tier repeated board positions. Commander's cause is that the king-advance term rewards forward movement only, so retreating costs nothing and re-advancing always pays. Making that term symmetric fixes the oscillation but leaves the deeper question unanswered: when should a king advance at all? Founder, 2026-08-11: 'King should not be a hero.'

## Recommended Direction

Scale the king-advance reward by a saturating measure of friendly cover ahead of the king, so advancing into empty space earns nothing. Apply the same discipline to the current Beacon bearer, including regrouping when a move increases its formation support. Beacon aggression itself is the bot permission: `0` disables all Beacon actions and a positive value enables and scores them, always beneath the match's Beacon rule gates.

The founder's model, in his own four statements:

1. There is no point in the king advancing if no covering pieces are in front of him.
2. The more pieces in front of him, **and** the farther he is behind the front line, the more the forward move should be rewarded.
3. Advancing with fewer pieces should reward less.
4. Pieces farther from the king contribute less cover than near ones.

The same discipline applies to the **Beacon bearer**, not only the king. Both are pieces whose loss is decisive, so both move up behind a screen rather than into space.

The shape this takes in scoring:

```
cover       = SUM over friendly p ahead of the king of  1 / (1 + chebyshev(p, king))
coverFactor = min(1, cover / COVER_SATURATION)
advance     = gain * MORALE_PUSH_VALUE * moralePush * coverFactor
```

- **"ahead"** means greater `forward_progress` than the king.
- **Chebyshev** distance, not rank difference, so a piece far off to the side decays as cover rather than counting fully.
- **The saturation is load-bearing.** Without `min(1, …)` the term is unbounded and the 0–1 aggression dial stops meaning anything. Cover must only ever modulate downward, never rescale.

A property worth keeping: if the retreat penalty is scaled by the same `coverFactor`, a well-screened king is penalised for dropping back — stay with the formation — while an **exposed** king regroups for free. Both behaviours are correct, and neither needs a special case.

## Alternatives Considered

**Make the existing king-advance term symmetric and stop there.** Cheapest fix, and it does resolve the oscillation — dropping the one-directional guard alone takes Commander from 13/75 to 53/75 against a passive opponent. It lost because it answers *"should the king give back ground?"* without answering *"should the king have advanced at all?"* A symmetric term still pays a lone king to walk into open space.

**Gate king advance on a hard rule** — for example, forbid it whenever the king would become the most advanced friendly piece. Simple, legible, and enforces the founder's "never in the first line of attack" exactly. It lost because a hard gate cannot express *degree*: it treats one distant covering piece the same as a full screen, and gives the tiers no dial to differ on.

**Scale the penalty by two-move enemy attack pressure.** The best model of the three — a king that backs off *before* it is attacked rather than once it already is. Deferred rather than rejected; see Not Doing.

## MVP Scope

Cover-scaled king and current-Beacon-bearer movement, using host-projected
post-move support/threat facts, an exact one-ply anti-reversal guard, and the
0.3/0.6/0.9 Recruit/Lieutenant/Commander aggression dials (Adviser 1.0 for
Beacon, its existing 0.5 for morale). The old 0.2/0.4/0.8 proposal values are
superseded by this founder decision.

## Not Doing (and Why)

- A two-move enemy attack map — needs data the observation does not carry, making it a host-side change rather than a bot-script edit
- The distance-behind-the-front-line factor — correlates strongly with cover, so multiplying both double-counts and would shrink the tier weights fourfold

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | Cover-scaling does not suppress the king-advance term below the point where the tier weights still move the king | Run the full fix stack with and without cover at 75 seeds × 3 tiers and compare wins; unscaled it reaches recruit 6 / lieutenant 42 / commander 55 |
| Must-be-true | The bot script can compute cover from the observation alone, with no host-side change | Confirmed: `observation["own"]` carries `square` and `rank` per cell, and `forward_progress` and `chebyshev_distance` already exist in the script |
| Should-be-true | Cover-scaling leaves already-correct decisions alone | Replay the 53-case decision corpus; the change measured 2/53 intents changed, both Commander |
| Should-be-true | Computing cover is cheap enough for the CI passive-opponent gate | Cover is computed once per decision rather than per candidate, so it is O(own pieces) per decision — but the CI cost has not been measured |
| Might-be-true | Distance-behind-the-front-line adds signal that cover does not already carry | Add it as its own sweep row and compare against cover alone; it has never been executed |


## SpecScore Integration

- **New Features this would create:** none — this changes how `standard-bot` scores king movement
- **Existing Features affected:** `standard-bot`; `royal-beacon` and `morale` supply the rules this interacts with
- **Dependencies:** none in code. Blocked in practice by an open founder decision — see Open Questions

## Open Questions

- **Do Recruit and Lieutenant keep their intended character once their king advances?** Resolved for this change: their positive Beacon aggression is permission to use the command chain when match rules allow it; it is no longer delegated to the BotSystems Beacon gate.
- **Should `COVER_SATURATION` differ per tier?** It is currently one constant for every difficulty. A tier that reads as cautious might warrant a higher bar for what counts as a full screen — but that is a third knob, and the parameter table is published for outside creators, so it needs to earn its place.
