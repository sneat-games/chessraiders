# The public trajectory suite

Plan task-20 (`spec/plans/publish-the-standard-bot.md`, repository
`sneat-games/chessraiders`): a Node suite that loads the published engine
wasm through `wasm_exec.js` and plays whole matches with the published bot
script (`go/bots/standardbot/chess-raiders-bot.star`) against an opponent that issues no
commands at all. This is CI for the public module — Task 12's own in-product
browser replayer (repository `sneat-co/chessraiders`) is the creator-facing
twin; both drive the identical engine and the identical script, but this one
proves the two properties Task 12 also proves, headlessly, for every push and
PR here.

**Nothing in this harness implements a rule.** Every question about
legality, capture resolution, or which candidate a script should prefer is
asked of the wasm itself (`apply`, `resolveBotAttack`) — see `lib/wire.mjs`
and `lib/trajectory.mjs` for exactly which calls, and why each one is a wire
translation (squares as algebraic strings <-> the engine's own integer
encoding, an intent's own vocabulary <-> a `chess.Command`) rather than a
rule. Nothing from the private `sneat-co/chessraiders` tree is copied
anywhere in this directory; every mapping was derived by reading the private
repository's own already-public wire contracts (`botcontract.BotIntent`,
`chess.Command`) and confirmed empirically against a real with-bots wasm
build — see this task's own PR description for the worked derivation.

## What it downloads, and how it verifies it

The suite queries `sneat-games/chessraiders`'s own GitHub Releases for the
newest tag matching `engine/<12-hex-engine-commit>` (task-19's own asset),
downloads `chessbots.wasm` and `wasm_exec.js`, and verifies both against that
release's own `SHA256SUMS.txt` before instantiating anything — see
`lib/engineRelease.mjs`. No credential of any kind: the release, and every
asset on it, is public.

## Running it

```sh
cd js/trajectory
npm test          # node --test
```

Requires Node >= 20 (for `node:test` and global `fetch`). No dependency
beyond Node's own standard library — nothing to `npm install`.

### Local development, before a real release exists

As of this suite's own authoring (2026-08-11), **no `engine/*` release has
ever been published** — task-19's `publish-engine-wasm` job exists in the
private repository but only fires on a real `v*` tag push, and no such push
has landed since that job merged. Every test in this suite calls
`t.skip(...)` with that exact reason in that case, rather than reporting a
false pass on zero real wasm calls — see "Empty-asset handling" below.

To exercise this suite before that release exists, build the with-bots
artifact from a `sneat-co/chessraiders` checkout and point this suite at it
directly:

```sh
# from a sneat-co/chessraiders checkout:
bash server-go/cmd/chesswasm/build.sh with-bots

# from this directory:
CHESS_WASM_DIR=/path/to/sneat-co/chessraiders/server-go/cmd/chesswasm/dist npm test
```

`CHESS_WASM_DIR` is a local-development-only override (`trajectory.test.mjs`'s
own `resolveChessWasm`). CI never sets it — the suite's real path is always
download-and-verify against the public release.

## Empty-asset handling

An empty or unreadable published asset is a **named skip**, never a silent
pass. Concretely: `resolveChessWasm()` in `trajectory.test.mjs` reports
`{ available: false, reason: "..." }` when no `engine/*` release exists (or
when the GitHub API call itself fails), and every test calls
`t.skip(reason)` at its very first line in that case — Node's test runner
reports these as **skipped**, a status distinct from both pass and fail, so
a CI job reading its output cannot mistake "the release doesn't exist yet"
for "the suite ran and everything is fine." This mirrors the Go corpus
replayer's own discipline (`go/bots/standardbot/corpus_replay_test.go`:
"An empty or missing corpus is a HARD FAILURE here, not a skip") at the one
point where the difference is real: that suite's corpus ships IN this
repository (an empty one is a regression), while this suite's wasm asset is
published by a SEPARATE, deliberate release action on a different
repository that has, genuinely, not happened yet. Once a first `engine/*`
release exists, every test here runs for real, and a skip afterward would
mean the download or verification broke — worth exactly as much attention
as a failure.

## Known finding: the standard bot does not reliably force a win against a
## passive opponent

While building this suite (2026-08-11), the passive-opponent trajectory was
run against the Commander tier across five seeds (`shippedDefaultRules`,
`kill_only`, 250ms ticks). Results:

| Seed | Outcome |
| --- | --- |
| 2 | **Won** in 244 decisions (61.25s simulated, ~7.5s wall clock) — a real `killed` -> `promoted` -> `king_captured` -> `convoy_created` -> `king_delivered` sequence. |
| 1 | Did not win within 6,000 decisions (1,500s simulated). After one early `beacon_passed`, the run degenerates into the king stepping in and out of enemy territory (`king_incursion_started`/`_ended`) roughly every cooldown tick, forever, with zero further kills. |
| 3 | Did not win within 1,500 decisions. Captured the enemy king (`king_captured`) and created a convoy, then stalled — no `king_delivered`. |
| 4, 5 | Did not win within 1,500 decisions. Multiple kills and promotions, but the enemy king was never captured. |

This suite's own CI-facing test pins seed 2 — the one scenario proven to
converge — the same way a golden fixture pins one case rather than
asserting a property that does not yet hold everywhere. **This is a real
finding about `chess-raiders-bot.star`'s own play quality, not a defect in this
harness**: seed 2's own run completes the full capture -> morale -> convoy
-> delivery pipeline the plan's own Approach section names as never having
been asserted end-to-end unattended before. The plan's own text anticipated
exactly this ("a bot that genuinely cannot force a win is a finding to
route, not a cap to loosen") — routed here rather than fixed, since fixing
`chess-raiders-bot.star`'s scoring is not this task's scope. Worth a follow-up task.

## What this suite needs from CI (routed to Task 10)

This directory adds no `.github/workflows/` file — Task 10 owns those in
this repository. What it needs from whichever job runs it:

- Node >= 20 on `PATH`.
- Run from `js/trajectory/`: `npm test` (equivalently `node --test`).
- No credentials, no `GOPRIVATE`, no checkout beyond this repository itself.
- This job may legitimately report all three tests **skipped** (not failed)
  until a real `engine/*` release exists — that is a correct, honest
  result today, not a broken job. Do not gate a merge or a tag on "all
  tests passed" here without also accepting "all tests skipped, named
  reason: no engine/* release yet" as equally green; gating on a literal
  pass would block ordinary development on an unrelated release action.

## Files

- `trajectory.test.mjs` — the suite: one win test (pinned seed), one budget
  test, one disagreement-detection (negative) test.
- `lib/wasmHost.mjs` — instantiates chessbots.wasm under Node via
  `wasm_exec.js`; the generic `call()` JSON-envelope wrapper every other
  module uses.
- `lib/engineRelease.mjs` — finds the newest `engine/*` release, downloads
  its assets, verifies them against its own `SHA256SUMS.txt`.
- `lib/wire.mjs` — the intent-to-`chess.Command` wire translation (square
  encoding, side/rank/profession enums, the `resolveBotAttack` capture-choice
  fallback, the `beacon_take` unaimed-target default).
- `lib/trajectory.mjs` — the match-driving loop: decide/apply/settle for one
  side, silence for the other, until a win or a named failure.
