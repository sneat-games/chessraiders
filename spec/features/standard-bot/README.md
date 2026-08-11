---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Standard Bot

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/chessraiders/spec/features/standard-bot?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

The built-in Recruit, Lieutenant and Commander opponents: what each difficulty does differently and what stays the same for all three.

## Contents

| Child | Description |
|---|---|
| [bot-script-contract](bot-script-contract/README.md) | What a bot script is handed each decision, what it may return, and the one source of randomness it may use. |
| [bot-execution-limits](bot-execution-limits/README.md) | The step budget and memory bound a bot script must live within, and what happens when it does not. |

## Problem

A real-time game with no turns has no natural pause for a lesson: a new
player can only learn Chess Raiders by losing to a stranger, and an
experienced one can't rehearse a plan alone. Chess Raiders needed an
opponent that's always available — and one that's honest about how it plays,
so that beating it (or losing to it) teaches something about the game
itself rather than about a black box.

An opponent that plays well but can't be read teaches nothing about *why*
it played that way, and one that can't be forked can't be improved on by
anyone but its own authors. So the three built-in commanders are not three
separate programs: they are one script, read the same way for all three,
with a short table of numbers beside it that is the entire difference
between Recruit, Lieutenant and Commander. Change one number and you have a
fourth opponent; read the script once and you know how all three think.

## Behavior

### Playing against a bot

#### REQ: bot-roster-configuration

Before a battle starts, add any number of bots to either team, each at its
own difficulty. A bot needs no account and is ready the moment it's added —
mixed rosters are normal: a bot ally beside you, a harder bot opposing, or
a whole team of bots while you command the other side of a bot-augmented
army. A battle with any bot defaults to private and never appears in the
open-battles list. The lobby and the board always show which units are a
bot's and at what difficulty, and the result is recorded as a bot match,
kept apart from human-versus-human outcomes.

#### REQ: bot-plays-like-a-player

A bot is an ordinary participant. It acts through the exact same options a
human has — moves, captures, charging, Beacon actions, everything — under
the same legality, charge and command-pacing rules, with no privileged
information: under [Fog of War](../fog-of-war/README.md), a bot sees only
its own side's projection, never the true board. Nothing a bot does is
possible only because it is a bot.

#### REQ: bot-timing-honesty

A bot acts through real time, like a human does: each difficulty has its
own reaction pace, and a bot never bursts several commands faster than
that pace allows.

### One script, three difficulties

#### REQ: one-script-three-difficulties

Recruit, Lieutenant and Commander are not three separate programs. They
are one script, played three times over with a different small table of
numbers beside it — that table is the entire difference between the
three. Read the script once and you know how all three commanders think;
change one number in one difficulty's table and you have a new opponent
without touching the others.

#### REQ: forking-needs-only-a-browser

The published script and its parameter table are ordinary text — nothing
about either needs a separate program to read or run. Copy one, change a
line or a number, and load it back into the game in your own browser to
see exactly which decisions changed: no toolchain to install, nothing to
compile, no repository to check out.

### The three difficulties

#### REQ: three-difficulties

Exactly three difficulties, and each is honest about what it's doing:

- **Recruit** reacts slowly and sticks to the basics: moves and captures
  only — no specialist training, no fortification work, no command-chain
  action — and goes for the obvious material gain.
- **Lieutenant** reacts at a medium pace and looks one step further: it
  weighs material, tempo, and a threat against its own units, trains an
  idle and undefended base-rank pawn into a specialist when it's safe to,
  and keeps its own command chain moving rather than letting it stall.
- **Commander** reacts fastest and plays the full board: prisoner
  logistics, incursion positioning, advanced specialist training,
  dismantling an enemy fortification, and everything Lieutenant already
  does.

A higher difficulty plays better because it looks further ahead and reacts
faster — never because it can see something a human on the same side
could not.

#### REQ: every-difficulty-defeats-a-passive-opponent

Every difficulty, Recruit included, wins against an opponent who never
issues a single command.

This is a floor underneath the three difficulties above, not a fourth
difficulty. It is not a walkover: a side that does nothing still has its
whole army standing in the way, so the bot has to advance into it, take
material, promote a pawn, capture the enemy king and escort it home —
all without a single move from the other side to react to. Recruit is
expected to take far longer than Commander and to win untidily.

What no difficulty may do is fail to finish. Wandering, stalling, or
repeating the same two moves until the clock runs out is a defect, not a
weaker setting — a commander who cannot beat someone standing still has
not been given a lower difficulty, they have been given a broken one.

#### REQ: no-bot-ever-builds-a-new-wall

None of the three difficulties ever raises a brand-new fortification.
Judging where and when to build one takes a kind of long-range planning
none of the built-in commanders attempts — a wall put in the wrong place
is worse than no wall at all. Repairing a wall that already stands, and
knocking down an enemy's, stay open to Lieutenant and Commander; only the
decision to start a new one is off the table for every difficulty.

#### REQ: no-bot-trains-or-builds-under-threat

No difficulty ever starts training a pawn, or starts any fortification
work, on a piece it can see is currently under threat — training and
building both hold a piece in place for their duration, so starting either
one there would just hand away material for nothing.

### Escorting a captured king home

#### REQ: convoy-finds-a-real-path

A bot escorting a captured enemy king home finds an actual path around
whatever is in the way, never a straight line through it — so it will
route two files over to reach an open lane rather than sit in front of a
blocked one waiting for nothing to change.

#### REQ: a-blocked-path-gets-opened

When every way home is blocked by the bot's own army, it moves one of its
own pieces aside rather than holding still — preferring its cheapest
available piece and a square the enemy can't immediately punish it on.

#### REQ: the-king-convoy-comes-first

A convoy carrying the captured enemy king outranks everything else a bot
could be doing: delivering that king is the only way to win, so the bot
will pass up an unrelated free capture to keep escorting it, and once no
further step is available it uses its turn to guard the convoy rather than
moving something else at random.

#### REQ: one-command-at-a-time

A bot commands one piece at a time. While any of its own pieces is
mid-action, it issues no further command — not even for a free capture or
to keep the king's convoy moving — so a bot never gets to act on more
fronts at once than a human player commanding the same army could.

### Getting advice

#### REQ: advice-is-the-bots-own-scoring-explained

When you ask for advice, you get the same assessment the standard bot
itself would use to choose a move — never a second opinion from a
separate system. Advice is that scoring made visible: the move it
actually favours, together with the reasons that produced its score. This
is deliberate: two independent systems that both claim to know "what's a
good move here" inevitably drift apart from each other over time, so
there is exactly one place that decides it, used both to play and to
explain.

#### REQ: advice-always-has-something-to-say

When nothing the bot's own scoring finds clears its bar for "clearly
better," advice does not go silent. It says so plainly — that nothing
scored clearly better right now — and then offers one or more of a small
set of general principles instead: sound, general ideas about this game
rather than a calculated pick. A general-principle suggestion is never
dressed up as a calculated one: it is always visibly a different kind of
answer, so you're never left wondering whether the game found something
clever or just isn't saying.

The principles offered today:

| Principle | What it notices |
|---|---|
| Develop your pieces | An officer still sitting on its starting square, or a base-rank pawn that's eligible to train, is contributing nothing yet. |
| Advance the king in moderation | Advancing your king raises your own team's Morale — an early king walk is a real asset here, unlike in traditional chess — but an unescorted king past the midline is one capture from becoming the enemy's prisoner. |
| Expand your visibility | Under Fog of War, your side's vision is only what your pieces' own footprints currently cover; a move that opens a new line of sight usually beats standing still and guessing. |
| Protect the command chain | A [Royal Beacon](../royal-beacon/README.md) whose chain back to the king has broken, or whose bearer stands undefended, is about to lose every benefit it's giving your formation. |
| Escort the captured king home | A convoy carrying a captured enemy king wins the match on delivery and loses everything on interception — nothing else on the board matters more while one exists. |
| Mind your morale before attacking | An attack you can see is sometimes one your own Morale doesn't allow — advice says so, in the same words the game uses when it rejects the attempt directly. |

Three further principles are recorded but not yet offered: resting a
fatigued unit before committing it again, fortifying an exposed flank, and
keeping every unit employed. Each is a real idea; each also risks nagging
or misleading in a way the six above don't — a fortification suggestion in
a match that never turned fortifications on, for instance — and that's a
judgement still being worked through rather than a gap nobody noticed.

## Dependencies

- [Real-Time Command](../real-time-command/README.md) — the command surface and cadence a bot plays through.
- [Fog of War](../fog-of-war/README.md) — the projection a bot's decisions are limited to.
- [Prisoner Convoys](../prisoner-convoys/README.md) — the captured-king delivery a bot's convoy priority serves.
- [Pawn Specialisation](../pawn-specialisation/README.md) — the training Lieutenant and Commander use.
- [Fortifications](../fortifications/README.md) — the wall work Lieutenant and Commander use.
- [Royal Beacon](../royal-beacon/README.md) — the command chain Lieutenant and Commander keep moving.

## Acceptance Criteria

### AC: solo-vs-bot-playable

**Given** a player adds a Recruit bot to the enemy team and a Lieutenant
bot to their own team before starting a battle
**When** they start the battle
**Then** both bots are ready with no second account needed, each issues
commands for its own team at its own pace, and the match ends with a
result recorded as a bot match

### AC: bot-plays-by-the-rules

**Given** a running bot match under Fog of War
**When** the bot acts
**Then** every command it issues is one the same rules would accept from a
human, it never moves faster than its own pace allows, and it never acts
on information Fog of War would have hidden from a human on its side

### AC: one-script-covers-all-three-difficulties

**Given** the three published difficulties
**When** their scripts are compared
**Then** all three are the identical script, and the only difference
between any two is their table of parameter values

### AC: a-changed-script-runs-with-nothing-but-a-browser

**Given** a copy of the standard bot's script with one number changed
**When** it is loaded into the game
**Then** it plays using a browser alone, with no separate program
installed, nothing compiled, and no checkout of any repository

### AC: difficulty-tiers-differ

**Given** the three difficulties playing identical starting positions
**When** each plays out
**Then** Recruit only moves and captures, Lieutenant additionally trains a
pawn when it's safe to and keeps its command chain moving, and Commander
additionally works fortifications, prisoner logistics and incursion
positioning

### AC: every-tier-beats-a-passive-opponent

**Given** each of the three difficulties in turn, against an opponent that
issues no commands at all, across a range of starting seeds rather than one
**When** each match plays out under a time budget generous enough for the
slowest difficulty
**Then** every difficulty delivers the enemy king home and wins, in every
seed — a single seed that stalls, wanders, or repeats one pair of moves
until the budget expires fails this criterion, and pinning the check to a
seed that happens to pass does not satisfy it

### AC: no-tier-builds-a-wall

**Given** any of the three difficulties with fortifications enabled and an
opportunity to raise a new wall
**When** it decides
**Then** it never does, though repairing an existing wall or dismantling
an enemy's remains open to Lieutenant and Commander

### AC: no-training-under-threat

**Given** a bot with an eligible pawn that is currently under threat
**When** it decides
**Then** it does not start training or fortification work on that pawn

### AC: convoy-routes-around-a-blocked-line

**Given** a bot's convoy carrying the captured king pinned in front of its
own pawn line, with a free way home through a gap two files over
**When** the bot decides on successive turns
**Then** it steps the convoy along a real path through that gap and
delivers the king, rather than sitting still in front of the blocked line

### AC: convoy-opens-its-own-blocked-home

**Given** a bot convoy whose every route home runs through its own pieces
**When** the bot decides
**Then** it moves one of those pieces clear rather than holding the
convoy still, and the convoy proceeds through the opened square on a
later turn

### AC: king-convoy-outranks-everything

**Given** a bot holding the captured enemy king as convoy cargo, with an
unrelated free capture available elsewhere
**When** the bot decides
**Then** it commands the convoy rather than taking the unrelated capture

### AC: one-command-at-a-time

**Given** a bot match left to run
**When** any of its own pieces is mid-action
**Then** the bot issues no further command until that action resolves,
even when a capture or its own king's convoy would otherwise call for one

### AC: advice-matches-the-bots-own-pick

**Given** a position and a request for advice
**When** the advice is compared with what the bot itself would choose in
the identical position
**Then** the top-scoring suggestion is the same move, and every reason
given for it is one the bot's own scoring actually used — never a second,
independently authored judgement

### AC: advice-never-goes-silent

**Given** a position where no move clears the bar for "clearly better"
**When** advice is requested
**Then** it says so plainly before offering anything else, offers at
least one general-principle suggestion from the table above, and that
suggestion is never presented as though it were a calculated pick

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
