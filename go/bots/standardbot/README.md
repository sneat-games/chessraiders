# standardbot

The standard Chess Raiders bot, published as one portable four-file closure:

- **[`chess-raiders-bot.star`](chess-raiders-bot.star)** — one Starlark script, defining one function,
  `decide()`. This is the entire brain of every built-in opponent.
- **[`params.schema.json`](params.schema.json)** — the directly consumable JSON
  Schema declaration for all sixteen flat inputs, with Commander as the
  runnable default.
- **[`params.json`](params.json)** — four named partial sets: `recruit`,
  `lieutenant` and `commander`, the three playing difficulties, plus
  `adviser`, a fourth set the same script scores candidate moves against to
  produce explain-mode advice rather than a move to play. `ResolveParams`
  validates each set and fills omitted schema defaults before `decide()` sees
  it. There is no other difference between the three difficulties: same
  script, same rules, different parameter values. The Adviser's effective row
  is deliberately its own — not inherited from, or shared with, any playing
  difficulty's effective row (see `params.json`'s own `adviser` entry and the
  private implementation's `AdviserParams` for why).
- **[`chess-raiders-bot.json`](chess-raiders-bot.json)** — the portable
  manifest that binds the script, schema and partial sets into one exact
  executable closure.

[`script.go`](script.go) embeds the script, schema, parameter sets and manifest
verbatim (`Script`, `ParamsSchema`, `Params` and `Manifest`)
so a Go program can load them without touching the filesystem; see
[go/bots/runtime](../runtime) for the interpreter that actually runs `Script`.
Everything below assumes no access to Chess Raiders' private implementation
and no goal beyond reading this bot, changing it, and running `go test`.

## What the bot is

Every command a bot issues — move, capture, charge, train a specialist, work
a wall — goes through the exact same legality and pacing rules a human
player's click would, under the same Fog of War: a bot never sees the true
board, only its own side's projection, and it never acts faster than its own
difficulty's reaction pace allows. See
[`spec/features/standard-bot`](../../../spec/features/standard-bot/README.md)
for the full behavioral contract (what a bot may do, how the three
difficulties differ, how convoy escort and advice work) and
[`bot-script-contract`](../../../spec/features/standard-bot/bot-script-contract/README.md)
for exactly what `decide()` is handed each turn and what it may return.

`decide()` is called once per decision with five arguments — the bot's own
view of the board (`observation`), whatever it chose to remember last turn
(`memory`), its difficulty's complete row from `ResolveParams` (`params`), one random
draw for breaking ties (`host_random_draw`), and how many ranked alternatives
to explain (`options`, 0 when just playing) — and it scores every legal
`(piece, destination)` pair the host has already worked out, plays the best
one, or passes if nothing clears the bar. It never invents a square, never
computes whether a move is legal, and never sees anything Fog of War would
hide from a human on the same side: every fact it weighs (danger, routes to
deliver a captured king, which squares are safe) arrives as plain data on
`observation`, computed by the host.

## What a single tier row means

A row is sixteen named values, in three groups:

**Ten score weights** — each scales one whole category of `decide()`'s
scoring, and each is a *gate*, not just a multiplier: `chess-raiders-bot.star` checks
`if params["X"] > 0` before scoring that category at all, so a weight of `0`
does not merely make that term worth nothing — it turns the whole category
off. This is what makes Recruit "moves and captures only": every weight
below except `material`, `safety`, `advance` and `delivery` is zero on its
row, so `chess-raiders-bot.star` never even considers a tempo, prisoner, or system
(training/wall/beacon) action for it.

| Key | What it weighs |
|---|---|
| `material` | The value of a piece captured or put at risk. |
| `safety` | Danger to the piece making the move — a weight of 0 means the bot never avoids a risky square for its own sake. |
| `tempo` | The cost of a move that costs real charge time relative to one that doesn't. |
| `advance` | Progress toward the enemy end of the board, and pawn promotion. |
| `targetLock` | Dodging a piece the enemy has target-locked. |
| `kingSafety` | Escaping or guarding a threatened king. |
| `moralePush` | The morale gain from advancing the king (Chess Raiders' king can safely advance further than in traditional chess — see the [Standard Bot spec](../../../spec/features/standard-bot/README.md)). |
| `delivery` | Escorting a captured enemy king home, and clearing a blocked delivery lane. |
| `prisoner` | Taking and holding prisoners, independent of delivering the king itself. |
| `system` | Every non-move action: training, wall repair/dismantle, Beacon deploy/restore/forge/hand-off, interrogation. |

**Three scheduling knobs** — bound how much of the board one decision looks
at, not what it values:

| Key | What it bounds |
|---|---|
| `breadth` | How many of the bot's own units get a proposal considered at all, out of every actionable unit, ranked by priority. |
| `candidateSpread` | How many destinations of *one* chosen unit get scored, out of its legal moves. |
| `passBelow` | The score floor: a proposal scoring below this is never played — the bot passes instead. |

**Three doctrine switches** — the only place a difficulty's *behavior*
branches, rather than its scoring:

| Key | What it switches |
|---|---|
| `advancedTraining` | Whether this tier ever pursues Advanced Engineer Training (Commander only, today). |
| `contestEnemyWork` | Whether this tier ever proposes dismantling an *enemy* wall — repairing its own is always open regardless. |
| `sergeantPreference` | The score bonus for wall work an adjacent Sergeant is speeding up. Must be `0` on any row where `contestEnemyWork` is `false` — `chess-raiders-bot.star`'s dismantle branch applies this bonus unconditionally *inside* code that only runs when `contestEnemyWork` already fired, so a nonzero value on a row that doesn't contest enemy work is a live inconsistency, not a no-op. |

No difficulty ever raises a *new* wall — that decision was removed outright
(see `chess-raiders-bot.star`'s own `wall_proposals` comment) and there is no parameter
that brings it back.

## Changing one number

Say you want a braver Recruit — one that stops flinching from every risky
square. Recruit's partial set has `"safety": 0.5`. Raise it toward `1.0` (the
Commander schema default and Lieutenant's own override) and Recruit will
start declining more of the
captures and advances it currently takes, specifically the ones that leave
the moving piece somewhere dangerous — nothing else about it changes,
because `safety` only ever scales the risk term `score_move` already
computes. Drop it to `0` instead and Recruit stops weighing danger to itself
at all: it will walk pieces into obvious losses whenever doing so scores well
on material or advance alone.

Widening `breadth` (say, Recruit's `4` toward Lieutenant's `8`) makes Recruit
consider more of its own pieces per decision — a stronger, slower bot, not a
differently-behaved one, because the scoring itself hasn't changed. Widening
`candidateSpread` does the same for how many destinations of *each*
considered piece get scored.

Every other effective row follows the same rule: a score weight changes *how much a
category matters* without touching what triggers it; a scheduling knob
changes *how much of the board gets looked at*; a doctrine switch changes
*whether a whole category of action is available at all*. Change one number,
re-run the conformance suite below, and read the diff in `decide()`'s
behavior against the empty-board smoke test or your own richer observation —
there is nothing else to regenerate: `params.schema.json` is the one source of
defaults, `params.json` contains only named differences, and `ResolveParams`
produces the inert data `chess-raiders-bot.star` reads at call time.

## Running the conformance suite

From this module's root (`go/`):

```sh
go test ./bots/standardbot/...
```

This proves, in order:

1. `params.schema.json` conforms to the supported JSON-Schema profile;
   `params.json`'s `schema` field matches the `ParameterSetsVersion` constant
   `script.go` exports; and it carries exactly the four partial sets
   (`recruit`, `lieutenant`, `commander`, `adviser`) that resolve into the
   complete rows consumed by the private implementation. The public schema
   and partial sets remain the only source of those values, with no row
   silently dropped in either direction.
2. `chess-raiders-bot.star` compiles under [go/bots/runtime](../runtime)'s dialect and
   binds a callable `decide`.
3. `decide()` actually runs, once per difficulty, each with that
   difficulty's fully resolved row, on the smallest observation
   `chess-raiders-bot.star`'s own `build_board()` accepts (an empty board, no legal
   moves) — proving every field a row declares is one `decide()` can
   actually consume, not just one that happens to parse.
4. `decide()`'s lifecycle gate still passes cleanly outside an active match.

What this suite does **not** prove is that the bot plays *well*, or that a
richer position scores the way the design intends — that needs the full
board-and-legal-move observation [`bot-script-contract`](../../../spec/features/standard-bot/bot-script-contract/README.md)
describes, which only the real game engine emits, so that end-to-end
behavior is verified against the private implementation instead (its own
`server-go/tests4bot` suite, replayed here byte-for-byte by the conformance
corpus below). **All closure files are edited HERE now, not there:** the script,
schema, named partial sets and manifest are this module's own source — the private implementation
requires this module at a released tag (`go.mod`) and reads them through it,
never the other way around. A `git diff` in this package IS the diff a
released tag ships; the private repository's own `go.work`-based local
iteration (its `CLAUDE.md`) points a checkout at an unreleased edit here
for testing, but publishing that edit still means landing it in this
package first.

## The conformance corpus

[`testdata/corpus`](testdata/corpus) closes most of that gap without needing
the private implementation at all: 53 real `decide()` calls, each with a
genuine board-and-legal-move observation, recorded from the private engine's
own behavioural suite (`server-go/tests4bot`, `sneat-co/chessraiders`, plan
task-8) and published here byte-for-byte — see
[`testdata/corpus/README.md`](testdata/corpus/README.md) for exactly what
each case is and why the capture contains 53 decisions from 30 of the private
suite's then-41 tests, not all 41.

[`corpus_replay_test.go`](corpus_replay_test.go)'s
`TestCorpusReplayAgreesWithThePublishedScript` is the replayer plan task-9
asks for: it feeds every one of those 53 cases' own recorded `{observation,
memory, parameters, randomDraw, options}` into THIS checkout's own
`runtime.Compile(Script)` and reports every decision that disagrees with what
was recorded, naming the case and the field. It resolves which script
version it is checking against from each case's own declared `script.version`
— never from this module's `go.mod`, which has no version for a module that
does not require itself — so a corpus that quietly mixed two recordings, or
declared the wrong module, is caught before a single `decide()` call runs.
[`corpus_replay_detects_disagreement_test.go`](corpus_replay_detects_disagreement_test.go)
proves the detector itself: it perturbs a recorded intent, a parameter row
and the declared script version, one at a time, and asserts each one is
caught with a legible, case-naming error — a replayer that can only report
agreement is not proven to detect disagreement.

This still does not prove the bot plays well against a *live, moving*
opponent, or that a change to `chess-raiders-bot.star` was intentional rather than a
regression the corpus happens not to cover — only that, for these 53
recorded decisions, this checkout's script still decides exactly what the
private engine's own tests once observed it decide.

## Running the corpus replayer

```sh
go test ./bots/standardbot/... -run TestCorpusReplay
```

reports `N/N recorded decisions agree` on success, or one line per
disagreeing case naming the file, the originating test/case and the
mismatching field on failure. An empty or missing `testdata/corpus` is a
hard failure here, not a skip — the corpus is supposed to ship 53 cases, so
finding none is a regression in what shipped, not a reason to pass.

To exercise the whole module, including the dependency-boundary check every
package here is held to:

```sh
go build ./...
go vet ./...
go test ./...
```

## License

Everything in this directory is [Apache-2.0](../../LICENSE), like the rest of
`go/` — see the repository's [top-level README](../../../README.md) for why
the license boundary runs at that directory rather than covering the whole
repository.
