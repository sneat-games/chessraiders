---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Bot Execution Limits

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot/bot-execution-limits?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot/bot-execution-limits?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot/bot-execution-limits?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot/bot-execution-limits?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

The step budget and memory bound a bot script must live within, and what happens when it does not.

## Problem

A budget only protects a match if crossing it is actually detectable — and
a script that spins forever inside a single operation may never check
anything at all, no matter how generous or how tight the number is. So the
real question isn't just "what's a safe budget" — it's "what happens when a
script doesn't play along with the budget at all," and this Feature
answers both.

## Behavior

### The budget

#### REQ: the-budget-is-counted-in-work-not-time

A decision's budget is counted in a number of execution steps, never in
milliseconds. A script's own work — and anything it asks a game-provided
helper to do on its behalf, tie-breaking included — all counts against the
same one number. This is deliberate: a slow device, a busy server and a
fast laptop all reach the identical decision for identical inputs, because
nothing about the outcome depends on how fast anything actually ran.

#### REQ: memory-is-bounded-too

The bundle a script hands itself between turns
([bot-script-contract](../bot-script-contract/README.md)'s memory) is
capped at a fixed number of entries. A script that returns more forfeits
that turn exactly as a step-budget overrun does.

#### REQ: a-forfeit-costs-one-turn-never-the-match

Exceeding either budget, or a script raising an error, forfeits that ONE
turn: the previous memory is kept untouched, so a bad turn can't poison
the next one, and the match keeps going — nobody is declared the winner
because a bot script had a bad turn, and no bot loses the match outright
for it.

#### REQ: the-built-in-tiers-never-forfeit

None of the three standard difficulties is ever expected to exceed its own
budget or raise an error. If one ever does, that's a defect being tracked
down, never a tuning choice you're meant to notice as part of ordinary
play.

### When a bot appears to have stopped

#### REQ: a-stopped-opponent-is-announced

If the same bot fails to respond for three turns in a row, the game tells
you plainly that your opponent has stopped, rather than leaving you to
guess whether it's thinking or broken. Nothing else about the match
changes: no winner is declared, the bot goes on being asked for its next
turn on schedule, and if it starts responding again the message clears
and play continues exactly as before.

#### REQ: a-script-error-names-only-the-script

When a script raises an error, what you're told is the script's own
message and — for an error inside the script itself — roughly where in
the script it happened. Nothing else about the game or the device it ran
on is ever included, and the same mistake always produces the exact same
wording, whether it happened over an empty square or one hiding something
[Fog of War](../../fog-of-war/README.md) wouldn't otherwise show you — so
an error can never be used to learn something the fog itself keeps
hidden.

### A script that will not stop

#### REQ: a-frozen-script-does-not-freeze-the-match

A script that never returns at all — stuck in a loop no budget check is
ever reached from — does not freeze the game around it. The bot's own
runtime is rebuilt, the interrupted turn is recorded as an ordinary
forfeit, and the match keeps advancing throughout: the board keeps
responding, nothing about the game's own state is affected, and the
recovered bot resumes on its very next scheduled turn from the memory it
held before it froze.

## Dependencies

- [Standard Bot](../README.md) — the tiers this budget bounds.
- [bot-script-contract](../bot-script-contract/README.md) — what a script
  is handed each decision, and the memory this Feature bounds.

## Acceptance Criteria

### AC: identical-inputs-decide-identically-regardless-of-device

**Given** the same script, observation and memory
**When** it is decided on a fast device and on a deliberately slowed-down
one
**Then** both produce the identical instruction and the identical
returned memory

### AC: memory-overflow-forfeits-and-keeps-the-old-memory

**Given** a bot with existing stored memory, and a script returning one
entry more than its budget allows
**When** the decision runs
**Then** that turn is forfeited, the stored memory afterward is unchanged
from before the decision, and the bot is asked for its next turn normally

### AC: a-forfeit-never-ends-the-match

**Given** a bot turn that overruns its budget, and separately one whose
script raises an error
**When** each forfeits
**Then** in both cases the match continues with no winner declared, and
the bot is asked for its next turn on its usual schedule

### AC: three-forfeits-in-a-row-tells-the-player

**Given** a bot slot that forfeits three turns in a row, and later
responds normally
**When** the third forfeit occurs, and again when it resumes
**Then** the player is told the opponent has stopped after the third
forfeit, no winner is assigned, and the message clears once the bot
responds again

### AC: an-error-reveals-nothing-about-the-fog

**Given** the identical script error triggered against a square that's
empty and, separately, one that hides an enemy piece under Fog of War
**When** the two errors are compared
**Then** they read identically, and neither names nor implies what, if
anything, occupies the square

### AC: a-frozen-script-is-recovered-without-freezing-the-match

**Given** a bot script that never returns
**When** the match runs past that bot's next scheduled turn
**Then** the match keeps advancing and stays responsive throughout, the
interrupted turn is recorded as an ordinary forfeit, and the bot resumes
on its next scheduled turn from the memory it held before

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
