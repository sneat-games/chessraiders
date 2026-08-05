---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Capture Outcomes

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/capture-outcomes?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/capture-outcomes?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/capture-outcomes?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/capture-outcomes?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

Every match is set to one battle mode, and the mode decides what your
options are when a piece lands on an enemy. Killing is certain by default
whenever it is offered — a match can opt into the **Probabilistic Kill**
setting to make it roll instead. Taking a prisoner or recruiting an
informer, on the other hand, is always a deliberate gamble: the attacker
names the risk before committing, sees an estimated chance of success, and
the server resolves it as success, a dead defender, or a repelled attack.

## Problem

Chess Raiders wanted killing to remain ordinary chess — reliable, readable,
no surprises — while giving players who *want* prisoners, informers, and the
extra strategic weight that comes with them a genuine reason to accept risk
for a bigger payoff. Making every capture unconditionally certain rewards
neither a defender who has fought hard to survive nor an attacker who
strikes fresh instead of exhausted; making killing itself uncertain would
have just made ordinary chess unreliable for no reason.

The founder's framing: **kills are deterministic chess by default; the risk
is entirely opt-in.** Nobody is ever forced to gamble to capture an enemy
piece — the gamble is the price of *keeping the defender alive*. A lobby
that wants Kill itself to carry risk can opt into Probabilistic Kill, but
that is a deliberate match-setting choice, never the default.

## Behavior

### Battle modes

#### REQ: battle-modes

A match uses exactly one of three battle modes, chosen before it starts:

| Mode | What capturing does |
|---|---|
| **Skirmish** (`kill_only`) | Every non-king capture removes the defender from the board; the attacker takes its square. The closest thing to classic chess, and almost entirely morale-free. |
| **Raid** (`capture_only`) | There is no Kill option. Every non-king attack deterministically creates a [convoy](../prisoner-convoys/README.md) with the defender as a captive, or transfers an existing convoy. This is the signature Chess Raiders mode. |
| **Conquest** (`kill_or_capture`) | The attacker explicitly chooses Kill, Capture, or Recruit Informer for each individual attack. Kill is certain by default; Capture and Recruit Informer always carry risk. |

Two things are true regardless of mode: capturing the enemy king is always a
capture, never a kill (see [Prisoner Convoys](../prisoner-convoys/README.md)),
and landing on a loaded enemy convoy is always an interception, never a
choice.

### Choosing an outcome

#### REQ: outcome-choice

Where Conquest offers a choice, the attacker's command carries the chosen
outcome — Kill, Capture, or Recruit Informer — as part of committing the
attack. There is never an intermediate "deciding" state visible on the
board. Recruit Informer is only ever offered against an ordinary enemy piece
and behaves as covert vision-sharing rather than a capture; its rules are
owned by [Espionage](../espionage/README.md). When the target is the enemy
king, only Capture is ever offered.

### Committing an attack

#### REQ: kill-waits-for-rest

As of the **balance-v7** profile, every attack — Kill included — requires
the acting piece to be fully rested before its charge can begin; see
[Fatigue](../fatigue/README.md)'s rest-then-fight rule. A Kill command
issued while the piece is still fatigued does not fail and does not change
its odds: the piece visibly rests in place, and the moment its fatigue
reaches zero the Kill's attack charge starts on its own. This is a timing
precondition only — it never touches Kill's determinism, the
Capture/Recruit success formula, or the Probabilistic Kill setting's
success rate. Capturing the enemy king and intercepting a loaded enemy
convoy are exempt and always begin immediately, regardless of fatigue.

### The risk roll

#### REQ: risky-outcome-buckets

An explicit Capture or Recruit attempt splits 100 percentage points across
three outcomes: **success** (the defender becomes a prisoner, or an
informer), **defender killed** (no prisoner or informer is created and the
attacker simply advances, as if it had killed outright), and **repelled**
(neither piece moves, both survive, and both suffer a brief recovery
penalty before either can act again). Kill itself never rolls by default —
it is certain whenever it is offered, unless the match has turned on
Probabilistic Kill (see below), in which case Kill rolls its own two-outcome
version of this same mechanism.

The default success formula compares the two pieces' material value and
current [fatigue](../fatigue/README.md):

```text
difference     = attacker's material value − defender's material value
baseSuccess    = clamp(65 + 3 × difference, 35, 85)
finalSuccess   = clamp(baseSuccess + defenderFatigue − attackerFatigue, 10, 90)
```

A fresh, equal-material attempt starts at 65% success; each point of
material advantage adds 3 percentage points, each point of disadvantage
removes 3, and fatigue then shifts the number further — a tired defender is
easier to take, a tired attacker is less capable. The remaining probability
splits between defender-killed and repelled, with both of those outcomes
always keeping at least a 5% chance:

| Contest | Success | Defender killed | Repelled |
|---|---:|---:|---:|
| Equal material, both fresh | 65% | 12% | 23% |
| Attacker +1 material | 68% | 13% | 19% |
| Attacker −1 material | 62% | 11% | 27% |

These are the default tuning values recorded on the match; a rules preset
may adjust them.

### Probabilistic Kill (opt-in)

#### REQ: probabilistic-kill-setting

Kill is deterministic by default in every battle mode: **off** is the
default value of the Probabilistic Kill match setting, and an off match
behaves exactly as described above — Kill never rolls. A lobby may instead
turn Probabilistic Kill **on** for the match. When it is on, an explicit
Kill command uses the same roll-and-repel machinery as Capture and Recruit,
but with only two outcomes instead of three: **success** (the defender is
eliminated and the attacker advances, exactly like today's deterministic
Kill) and **repelled** (neither piece moves, both survive, and both take
the match's configured recovery penalty). There is no "defender killed"
bucket for Kill — that outcome already *is* Kill's success case. The
setting carries its own configured success rate, commonly 85–90% when
enabled, independent of the Capture/Recruit success formula. Kill's outcome
is never influenced by either piece's fatigue, on or off.

#### REQ: capture-estimate-display

Whenever an explicit Capture or Recruit attempt is currently available, the
action shows its estimated success percentage in this exact form:
`Capture - 73%` (or `Recruit Informer - 73%`). The estimate is calculated
for the moment the attack's charge would finish, using both pieces'
projected fatigue at that time — it updates as fatigue, timing, or material
change, and it is an estimate, not a guarantee: the actual roll happens at
the moment of contact, using the real state at that instant. Deterministic
actions (a default-mode Kill, an automatic Raid-mode capture, king capture,
and convoy interception) never show a percentage, because they never roll.
When a match has Probabilistic Kill turned on, Kill shows its own estimate
in the same `Kill - 90%` form, using that setting's configured rate rather
than the Capture/Recruit formula.

### Outcomes in play

#### REQ: outcome-effects

- **Success** — Capture behaves exactly like a deterministic capture: the
  defender becomes a prisoner and the attacker becomes its convoy escort.
  Recruit success instead turns the defender into a hidden informer for the
  attacking side, with neither piece moving; see
  [Espionage](../espionage/README.md).
- **Defender killed** — behaves like an ordinary kill: no prisoner or
  informer is created, and the attacker advances onto the now-empty square.
- **Repelled** — neither piece moves, both survive, and both receive the
  match's configured brief recovery penalty before either can begin
  charging its next action. This is combat exhaustion, distinct from
  ordinary move charging.

Every resolved risky attempt — whichever way it lands — counts as a physical
clash for [fatigue](../fatigue/README.md) purposes for any piece that
survives it. If the target manages to escape before the attack ever makes
contact, no roll happens at all and neither piece gains fatigue from it.

## Dependencies

- [Real-Time Command](../real-time-command/README.md)
- [Fatigue](../fatigue/README.md)

## Acceptance Criteria

### AC: kill-mode-is-certain

**Given** a match set to Skirmish with Probabilistic Kill left at its off
default
**When** a non-king piece captures an enemy piece
**Then** the defender is removed from the board and the attacker occupies
its square, with no roll and no estimate shown.

### AC: kill-queues-behind-fatigue

**Given** a piece carrying fatigue is commanded to Kill an enemy piece
**When** the command is accepted
**Then** the Kill does not begin charging immediately; the piece visibly
rests until its fatigue reaches zero, and only then does the Kill's attack
charge start — with the eventual outcome exactly as certain as an
unfatigued Kill.

### AC: king-capture-and-interception-ignore-the-rest-gate

**Given** an attacker carries fatigue, including the match's maximum
**When** it captures the enemy king, or intercepts a loaded enemy convoy
**Then** the action proceeds immediately, without waiting for the attacker
to rest.

### AC: probabilistic-kill-can-be-repelled

**Given** a match has turned the Probabilistic Kill setting on
**When** a piece commits an explicit Kill against an enemy piece
**Then** the action shows a `Kill - NN%` estimate beforehand, and the server
resolves it as either success (the defender is eliminated exactly like a
deterministic Kill) or repelled (neither piece moves, both survive, and
both take the configured recovery penalty) — never as a third "defender
killed" outcome.

### AC: probabilistic-kill-ignores-fatigue

**Given** a match has Probabilistic Kill on, and an attacker and defender
carry different fatigue levels
**When** the Kill's success rate is calculated
**Then** it matches the setting's configured rate exactly, unaffected by
either piece's fatigue.

### AC: raid-mode-always-creates-a-convoy

**Given** a match set to Raid
**When** a non-king piece captures an enemy piece
**Then** a convoy is deterministically created or an existing convoy gains
the captive, with no Kill option ever offered.

### AC: conquest-offers-a-real-choice

**Given** a match set to Conquest and a legal attack against an ordinary
enemy piece
**When** the attacking player commits the attack
**Then** they choose exactly one of Kill, Capture, or Recruit Informer as
part of the same command, and the board never shows an intermediate
"deciding" state.

### AC: estimate-matches-the-formula

**Given** a fresh, equal-material attacker attempts Capture against a
defender that will have decayed to 8 fatigue points by the time the attack's
charge completes
**When** the action button is shown
**Then** it reads `Capture - 73%`, matching `65 + 8 − 0`.

### AC: fatigue-shifts-the-odds-both-ways

**Given** two otherwise identical Capture attempts, one by a fresh attacker
and one by a fatigued attacker, both against an equally fatigued defender
**When** each is attempted
**Then** the fresh attacker's estimated and actual success is higher than
the fatigued attacker's.

### AC: three-outcomes-always-sum-to-whole

**Given** any risky Capture or Recruit attempt
**When** its three outcome buckets are calculated
**Then** success, defender-killed, and repelled always sum to 100%, success
stays between 10% and 90%, and both failure outcomes retain at least 5%.

### AC: repelled-leaves-both-pieces-alive

**Given** a risky attempt resolves as Repelled
**When** the outcome is applied
**Then** neither piece moves, both survive, and both begin the configured
recovery period before either can start its next action.

### AC: king-and-convoy-attacks-never-roll

**Given** an attack against the enemy king, and separately an attack landing
on a loaded enemy convoy
**When** each is committed
**Then** neither ever shows a percentage or uses the risk roll — the king
attack is always Capture and the convoy attack is always an interception.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
