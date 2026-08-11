# Handover — Standard bot cannot beat a passive opponent

**Date:** 2026-08-11
**Repo:** `sneat-games/chessraiders` (public). Some work is in `sneat-co/chessraiders` (private) — each item below names its repo.
**Status at handover:** design settled with the founder, measurement substantially done, **one founder decision blocking the shape of the fix**.

---

## 0. Read this first

Three things outrank everything else in this document:

1. **A bot cannot win without `Systems.Beacon` — proven from the code, not measured.** King delivery is the only terminal condition in the engine. The founder's constraint *"even without it a bot should be able to win"* is unsatisfiable by weights and needs a game-design decision from him. **Put this to him before writing code.** (§2a)
2. **The single next measurement is one matrix row** — the cover model has never been swept, and it can only reduce the king-advance term the founder's 0.2 / 0.4 were chosen against. (§3e)
3. **Recruit has a second, unrelated bug** — its repeat cycle is a promoted-queen lateral shuffle that no king-advance change touches. (§2c)

---

## 1. What you are picking up

Two spec requirements were merged today that the bot **does not currently satisfy**:

- `REQ: every-difficulty-defeats-a-passive-opponent` (merged `913fd5d`)
- `REQ: no-repeated-position-against-a-passive-opponent` (merged `9afcabb`)

A passive opponent is one that never initiates. Every playing tier must beat it, and no board position may repeat while doing so. **Board state means which piece stands on which square, and nothing else** — not fatigue, charge, morale, Beacon standing, or the clock. That definition is the founder's and was corrected into the spec explicitly; do not widen it.

No ordering between difficulties is implied. A Recruit may finish *first* — against no resistance, how quickly a tier wins measures how directly it plays, not how well.

---

## 2. The measured problem

A 225-match sweep (75 seeds × 3 tiers, 400-decision budget) produced:

| tier | won | repeated a position | first repeat (min/med/max) | distinct movers |
|---|---|---|---|---|
| recruit | **0/75** | 75/75 | 27 / 29 / 46 | 4.0 |
| lieutenant | **0/75** | 75/75 | 61 / 71 / 81 | 8.0 |
| commander | 13/75 | 62/75 | 36 / 42 / 293 | 16.3 |

**These are two different bugs, not one.**

### 2a. Recruit and Lieutenant — a tier-capability defect, not scoring

Three gates compose into a dead end:

1. `DefaultBotTier` (`server-go/chess/bots.go:217`) gives Recruit a **zero-value `BotSystems`** — every system false — and gives Lieutenant only `Training` and `Walls`. Neither gets `Beacon`.
2. `beaconRootsBearer` (`server-go/chess/beacon.go:301`) roots the **king** while `Rules.Beacon.KingStartsAsBearer` and `!EverPassed`. `EverPassed` is set **only** by `ActionBeaconTake` (line 461), which requires `Systems.Beacon`. No Beacon system → the king can never unroot.
3. `moralePush` is `0` for both tiers, so nothing rewards a king move even if one were legal.

Result: **0 king moves in 23,973 Recruit decisions and 16,169 Lieutenant decisions.** `TeamMorale = kingRank − 1` stays 0, and `CaptureAllowed` requires morale ≥ 2 — so these tiers can never capture, and therefore can never win.

Removing all three gates: Recruit 0/10 → 4/10, Lieutenant 0/10 → 2/10. **Removing any one alone changes nothing.** That is the important part — a partial fix measures as no fix.

> ### ANSWERED — and it is a game-design decision, not a tuning problem
>
> **A win is impossible without `Systems.Beacon`.** This is a proof from the code, not an inference:
>
> - `deliverKingIfEligible` (`chess/apply.go:1132-1133`) is the **only** function in the engine that sets `s.Winner`, and `EvMatchEnded` is emitted in exactly one place. Every facade-level `m.Winner` is just `= m.State.Winner`. **King delivery is the sole terminal condition.**
> - Delivering needs a captured king; capturing one needs `CaptureAllowed` at morale ≥ 2; `TeamMorale` = king rank − 1; the king is rooted while it holds a never-passed Beacon; `EverPassed` flips only via `ActionBeaconTake`, gated on `systems["beacon"]`.
>
> Corroborated empirically: with the Beacon grant withheld, Recruit and Lieutenant stay **0/75 with `kingMoveShare = 0.00`** under *every* weight combination tried (matrix rows 8, 9, 14 below).
>
> So the founder's constraint — *"even without it a bot should be able to win"* — **cannot be satisfied by weights at all.** It needs one of: grant the capability, change the rooting rule, decouple morale from king rank, or add a second terminal condition. **That is the founder's call and it blocks the shape of the fix. Put it to him first.**
>
> Related trap: `system_proposals` is a **double gate** — `any_system_enabled` (capability) AND `params["system"] > 0`. A new Beacon weight must be checked **outside** the `params["system"]` lump, or Recruit's `system: 0` keeps it inert no matter what the Beacon weight says.

### 2b. Commander — a genuine scoring defect

In `tier.star`, the `moralePush` block rewards forward king movement only:

```python
if params["moralePush"] > 0 and cell["rank"] == "king" and not cell["convoy"]:
    gain = forward_progress(board["side"], destination) - forward_progress(board["side"], cell["square"])
    if gain > 0 and danger["threat"] == 0:
        score += add_term(terms, "moralePush", gain * params["moralePush"], "push")
```

The `if gain > 0` guard means retreating adds **no term at all** — not a penalty, a zero. Scores do not accumulate; each decision scores candidates by its own delta, so nothing ever charges the king for giving back ground it was paid to take. **The bot is paid to oscillate.** That is the repeat-position bug for Commander.

> **Correction to an earlier write-up — the margin is small, and this matters.** It was previously reasoned that "retreat scores exactly 0, a tie with any neutral move, and re-advancing pays +1.2." Instrumentation contradicts that. At Commander's actual flip point the retreat `c5→b4` scored **−0.35** (tempo only) against a best alternative of **−0.45** — it wins by a **0.10** margin, not by tying at zero. Recruit's equivalent margin is **0.05**.
>
> The conclusion survives (any penalty above ~0.10 breaks the cycle, and the proposed −0.60 clears it comfortably), but **size the penalty against 0.10, not against 1.2.** Anyone re-deriving this from the earlier reasoning will pick a number an order of magnitude larger than needed.

A shared plateau affects all tiers: `score_move` uses per-move deltas, and both `advance` and `moralePush` are zero for lateral moves.

### 2c. A third bug, newly found — Recruit's cycle is not the king at all

Even under the full fix stack, Recruit reaches only **6/75**. The reason is that **Recruit's repeat cycle is a promoted-queen lateral shuffle, not a king oscillation.** No `moralePush` change touches it, because both `advance` and `moralePush` score zero for lateral moves.

This is unfixed and undiagnosed. Do not expect the king work to close it.

---

## 3. The agreed fix — settled with the founder, not yet fully built

### 3a. `moralePush` becomes a normalised 0–1 dial

The founder's complaint was that `1.2` is unreadable. The root cause is real: **`moralePush` is the only weight in the table that skips the table's own idiom.** Every other weight multiplies a named base constant; this one multiplies a bare rank count.

```
safety     -> -KING_VALUE * safety              (60   x w)
delivery   -> DELIVERY_STEP_VALUE * delivery    (4.0  x w)
prisoner   -> PRISONER_STEP_VALUE * prisoner    (1.5  x w)
moralePush -> gain * moralePush                 (bare count x w)   <-- the odd one out
```

Fix: add the missing constant.

```
MORALE_PUSH_VALUE = 1.5   # value of advancing the king one rank at full aggression
```

| tier | dial | points | vs today |
|---|---|---|---|
| recruit | 0.2 | 0.30 | was 0 |
| lieutenant | 0.4 | 0.60 | was 0 |
| **commander** | **0.8** | **1.20** | **unchanged** |
| **adviser** | **0.33** | **0.50** | **unchanged** |

Commander and Adviser are specified as **bit-identical to today**. That makes the 53-case corpus a sharp acceptance test: any diff there means a constant is wrong.

`1.0` is the documented sane maximum, and must be documented as *"outbids any pawn-level gain"* — **not** as "always advances". An additive term cannot dominate delivery (16 for Commander) or the −60 safety wall; the founder asked about "always" and accepted this.

Scale anchors, for judging any future value: `PAWN_VALUE 1.0`, `KNIGHT_VALUE 3.0`, `ROOK_VALUE 5.0`, `QUEEN_VALUE 9.0`, `KING_VALUE 60.0`, `DELIVERY_STEP_VALUE 4.0`, `DELIVERY_BONUS 500.0`.

### 3b. The cover model — the king advances behind its own screen

Founder: *"There is no point for king to advance if no covering pieces in front of him. King should not be a hero."*

```
cover       = SUM over friendly p ahead of the king of  1 / (1 + chebyshev(p, king))
coverFactor = min(1, cover / COVER_SATURATION)          # ~2.0 start
advance     = gain * MORALE_PUSH_VALUE * moralePush * coverFactor
```

- **"ahead"** = greater `forward_progress` than the king.
- **Chebyshev** distance, not rank difference, so a piece far off to the side decays as cover rather than counting fully.
- **The `min(1, ...)` saturation is load-bearing.** Without it the term is unbounded and the 0–1 dial stops meaning anything. Cover must only ever modulate downward.

**Applies to the Beacon bearer as well as the king.** Both are pieces whose loss is decisive; both move up behind a screen.

**The founder's fourth statement — "the farther from the front line, the more reward" — is a SECOND factor and must be measured separately, not multiplied in.** It correlates strongly with cover (being behind the front line largely implies pieces ahead), so multiplying double-counts, and two factors each averaging ~0.5 give ~0.25 — which would shrink 0.2 and 0.4 to an effective 0.05 and 0.1 and restore the original bug. Build cover first; add the gap factor only if it measurably beats cover alone.

### 3c. Retreat penalty at 50% of the advance rate, scaled by the same `coverFactor`

Founder: *"retreating penalty should be about 50% of advance bonus"* and *"bot should be penalized for moving king back unless king is under attack."*

Symmetry is not required, because scores do not accumulate — the live comparison is retreat-vs-neutral-at-0, and any non-zero penalty breaks today's tie.

A useful property falls out of scaling by `coverFactor`: a well-screened king is penalised for dropping back (stay with the formation), while an **exposed** king regroups for free. Correct in both cases, with no special case.

**Keep the `board["king_threatened"]` suppression** until data proves it unreachable. Reason: `kingSafety` is **0 for Recruit**, so Recruit never receives the `+3/+4` escape bonus, and an unopposed retreat penalty would make its king refuse to flee — a new bug in the tier being fixed. It is redundant for Lieutenant and above and load-bearing for Recruit; comment it so nobody deletes it as unreachable.

Note the two signals are distinct: `danger["threat"]` = destination is attacked; `board["king_threatened"]` = the king is currently under attack.

### 3d. A separate Beacon aggression weight

Founder: *"we should have a setting/weight on how aggressive king and beacon should be (separate weights)."*

King aggression already exists as `moralePush`. **Beacon aggression is genuinely new** — Beacon play today is a boolean `Systems.Beacon` capability with no weight anywhere.

Same idiom: a 0–1 multiplier on the `SYSTEM_BEACON_*` constants that already exist (`HAND_OFF 5.0`, `DEPLOY 4.0`, `RESTORE 3.0`).

**It sits alongside `Systems.Beacon`, it does not replace it.** The flag is a rules capability — what the match permits — and the weight is a bot preference. A weight must never authorise an action the rules forbid.

This is also what makes granting `Beacon: true` to Recruit safe, which was the standing objection: Recruit can hold the capability with a near-zero Beacon weight, so it makes the opening pass that frees its king and nothing cleverer. Its published description ("moves and captures only… no command-chain action") stays true, and difficulty lives in numbers rather than in which code paths exist.

---

## 3e. What has actually been measured

All figures below were run. **Anything not listed here was not run** — do not infer it.

Branch `diagnose-bot-stall` @ `2ebc0ed`, pushed to `origin` on **private `sneat-co/chessraiders`**, not merged, no PR. All harness code is throwaway diagnostics under `server-go/tests4bot/` (`zz_stall_diagnosis_test.go`, `zz_variant_matrix_test.go`, `zz_blast_radius_test.go`). **No published artifact was edited** — `tier.star`, `params.json`, `params.go`, `chess/bots.go` and the 53-case corpus are untouched on disk; every variant is compiled from an in-memory patched copy.

### Corpus replay — 53 cases, baseline 53/53 reproduced

| patch | intents changed | errored |
|---|---|---|
| normalised dial, script only (corpus keeps old scale) | 1/53 | 0 |
| **normalised dial + params rescaled together** | **0/53** | 0 |
| normalised dial + cover model + params rescaled | 2/53 (commander) | 0 |
| retreat penalty, un-normalised | 0/53 | 0 |
| rotating breadth slot | 0/53 | 0 |
| Beacon weight, bare `.get(…, 0)` gate | 5/53 (commander) | 0 |
| **Beacon weight, `.get(…, params["system"])` fallback** | **0/53** | 0 |
| Beacon weight via `params[…]` bracket | 0 | **19/53 error** |
| **rename `moralePush` → `kingAggression`** | 0 | **50/53 error** |

Two conclusions to carry forward:

- **The rescale is exact.** `0.8 × 1.5 == 1.2` and `(1/3) × 1.5 == 0.5` hold bit-exactly through the JSON round trip, and the 0/53 result confirms Commander and Adviser are unchanged. `MORALE_PUSH_VALUE = 1.5` with dial 0.8 / 0.33 is verified, not assumed.
- **Do not rename `moralePush`.** The corpus records parameter names inline, so a rename errors 50 of 53 cases. The name is effectively frozen by the published corpus.

### Sweep matrix — 75 seeds × 3 tiers, 400-decision budget (`won/75`)

Legend: **A** drop the `gain > 0` guard · **B** `Beacon: true` capability · **C** rotating breadth slot · **Dk** dial 0.2/0.4 · **Db** Beacon weight

| row | recruit | lieutenant | commander |
|---|---|---|---|
| baseline | 0 | 0 | 13 |
| A | 0 | 0 | **53** |
| B | 0 | 0 | 13 |
| C | 0 | 0 | 13 |
| Dk | 0 | 0 | 13 |
| Db | 0 | 0 | 13 |
| A+Dk | 0 | 0 | 53 |
| Dk+Db *(B withheld)* | 0 | 0 | 13 |
| A+C+Dk+Db *(B withheld)* | 0 | 0 | 56 |
| B+Db | 0 | 0 | 13 |
| A+B+Dk+Db | 0 | 0 | 53 |
| A+B+C+Dk+Db | 3 | 35 | 56 |
| **A+B+C+Dk+Db (normalised)** | **6** | **42** | **55** |

**Reading it:** A alone takes Commander 13 → 53 and is the single highest-value change. Recruit and Lieutenant move only when **A, B and C are all present** — consistent with §2a's finding that removing any one gate alone changes nothing. Nothing reaches all-win, and Recruit stays low for the separate reason in §2c.

### Not run — the gap in the evidence

- **No sweep row contains the cover model.** It is built (`COVER_SATURATION = 2.0`, computed once per decision in `build_board`) and corpus-measured at 2/53, but never swept.
- **The gap factor was never executed at all** — written, never run, not corpus-tested.
- The founder's 0.2 / 0.4 are sweep-measured **only unscaled**. Their behaviour under cover-scaling is **unknown**, and cover can only reduce the term.
- CI cost of the cover model: **not measured.**

**This is the single next action:** run `A(normalised) + cover + B + C + Dk + Db` at 75 seeds and compare Recruit / Lieutenant against 6 and 42. That one row decides whether 0.2 / 0.4 survive cover-scaling or must be escalated to the founder. The harness already supports it — add a `cover` flag to `variant` that appends `coverPairs` after `normalisedPairs`.

### Feasibility — answered, and favourable

**The cover model is a `tier.star` change only; no host-side change.** `observation["own"]` carries `square` and `rank` per cell, and `forward_progress` and `chebyshev_distance` already exist in the script. Cover is computed **once per decision** in `build_board` — it depends only on the king's current square — so it is O(own pieces) per *decision*, not per candidate.

---

## 4. Where the parameter table is authored — read this before editing anything

**`server-go/starlarktier/params.go` in the private repo is the single author.**

```
params.go  ->  bot_tier_params_fixture_test.go (generator)
           ->  contracts/chess-bot-tier-params-v1.json  (private)
           ->  go/bots/standardbot/params.json          (public, this repo)
```

The public package's own doc says it plainly: *"this package is a read path, not a source of truth."* **Never hand-edit the public copy.**

Consequences:

- All four rows live in `params.go`, so a new field gets an **Adviser** value whether or not anyone chooses one. Choose it deliberately — that row's doc comment says inheriting a playing tier's numbers cannot produce honest advice in either direction.
- `ParamsVersion` (`chess-bot-tier-params/v1`) names the **envelope shape**, not row contents, so an additive field probably needs no bump — but confirm.
- `script.go`'s package doc enumerates the row as *"the ten score weights, the three scheduling knobs and the three doctrine switches."* An eleventh weight makes that prose wrong.
- The 53 corpus cases record `parameters` **inline per case**, so they decode a new field as its zero value. Gated on `> 0`, they should replay identically — **verify, do not argue it.**

The bot script is `go/bots/standardbot/tier.star`, renamed to `chess-raiders-bot.star` in flight (founder: *"tier says nothing"*). Check which name is on `main` before you edit.

---

## 5. Acceptance — what "done" means here

1. **All 53 corpus cases still pass.** Commander and Adviser are specified unchanged, so a single diff means a constant is wrong. This is the sharpest available check.
2. **The 75-seed × 3-tier matrix**, with each change measured **alone and in combination**, comparable to the baseline in §2 (recruit 0/75, lieutenant 0/75, commander 13/75). Rows to keep separate: the dial values, the cover model, the retreat penalty, the `Beacon: true` grant, and the breadth policy. At least one run must **withhold** the Beacon grant, to answer §2a's open question.
3. **Both spec requirements asserted**, not just the win. A pure pass-stall never trips the repeat check, so a win-only assertion is not sufficient.

### CI gate facts already established

- Budget: **400 decisions**.
- 1 seed × 1 tier ≈ **1.0s**; 30 matches ≈ **15.5s** today — but **60–110s once the bot actually plays**, because winning takes more decisions than stalling. Budget the CI slot for the fixed bot, not the broken one.
- **No green-preserving seed exists** — seed 1 fails all three tiers.
- Determinism verified over 45 runs.
- Founder's shape: **1 stable seed + 9 random per tier**, so it doubles as fuzzing. *"If we can do 10 runs per level under few secs that would be nice."*
- Placement: the passive-opponent test runs **before** the wasm build and before the journey tests. *"Before we test bots in WASM we should test in Go — that should be much faster turn around."*

---

## 6. Explicitly parked — do not build

**Dynamic retreat penalty scaled by 2-move attack pressure.** The founder proposed scaling the penalty by how many enemy pieces could attack the square within two moves — a king that backs off *before* it is attacked rather than once it already is. Better model, deliberately deferred:

- likely needs a 2-ply attack map the observation does not carry, i.e. a host-side change in `server-go`, not a `tier.star` edit;
- costs a per-candidate sweep against a decision budget already being timed for CI;
- fog-of-war means a 2-move map over unseen squares is speculative and would need `GHOST_DISCOUNT` (0.5) applied.

**Revisit trigger:** the sweep report says whether the flat 50% leaves a gap this would close. Not before.

---

---

## 8. Working rules that bind this task

- **Never work in a canonical clone** under `/Users/alex/projects/` — hooks reject changes and other sessions share it. Use `wb worktree create <name> <org>/<repo> --original-prompt-file <path>`.
- **`isolation: "worktree"` isolates the filesystem tree and nothing else.** Fixed ports collide across worktrees (`wrangler dev` pins 18788; an agent lost time to another agent's server today). Check `lsof -i :<port>`; **never kill a process you did not start**. Same for shared scratchpad paths — give every agent its own subdirectory.
- **Never hand-edit a `**Status:**` line, frontmatter, or an index row.** Status changes go through `specscore <kind> change-status`.
- **A subagent may not reduce approved scope to satisfy a mechanical guard** — size budget, lint cap, coverage floor, timeout. If the guard cannot be met within scope, stop and report the trade-off.
- **Do not raise a size-budget cap to make a change fit.**
- **The engine source stays private.** No part of `server-go/chess` may appear in the public tree; only the compiled artifact crosses.
- **Finished work lands the same session** — validated-but-unmerged is not done. `gh run watch <run-id> --exit-status` in the foreground is the blocking call; the Bash cap is 600000ms against a ~19-minute CI, so loop it.
- Before `gh pr merge --delete-branch`, run `gh pr list --base <branch>` — a stacked PR gets **closed** by the delete, not retargeted.

---

## 9. State of the three parked agents

All three were stopped mid-task when session budget ran out.

| work | repo | state |
|---|---|---|
| **Bot diagnosis + variant matrix** | private `sneat-co/chessraiders` | **Reported.** Branch `diagnose-bot-stall` @ `2ebc0ed`, pushed, **not merged, no PR**. Throwaway diagnostics only; no published artifact touched. Everything in §3e came from it. |
| Nightly red, 6 nights, Webapp unit tests | `sneat-co/chessraiders` | **Stalled and died** before reporting. Its last output was installing dependencies and preparing to run the failing step with **TinyGo on `PATH`** — a hint about where to look, **not a diagnosis**. Nothing was fixed and no cause was established. Start from the logs, not from that hint. |
| The cover change-request in this repo | `sneat-games/chessraiders` | **Stalled and died** before writing anything. It did independently confirm the feature spec lives in `sneat-games/chessraiders` (this repo). Its intended content was written directly into [`../features/standard-bot/proposals/king-beacon-advance-under-cover.md`](../features/standard-bot/proposals/king-beacon-advance-under-cover.md) instead. |

**First action for whoever picks this up:** `gh pr list` in both repos and `wb worktree list`, to find any branch pushed but unmerged. Do not assume a branch is absent because this document does not name it.

The `diagnose-bot-stall` branch is worth keeping — its harness is what makes the next measurement cheap. It is diagnostics, not product code, so decide deliberately whether it lands or stays a branch.

---

## 10. Outstanding founder actions

1. **Branch-protection decision on public `main`** — there is none today.
2. Delete the throwaway fork `trakhimenok/chessraiders`.
3. Cut a tag to unblock ~369 deferred rename prose mentions.

### A blocked item whose cause is NOT established — re-diagnose, do not act on it

Plan **Task 10** (the public module's two CI jobs, PR #13) is blocked on a **404**. It was previously written up as *"`SNEAT_GAMES_RELEASE_TOKEN` needs read access to `sneat-co/chessraiders`"*. **That framing is unsupported, and the founder challenged it correctly.**

`SNEAT_GAMES_RELEASE_TOKEN` already carries read/write on `sneat-games/chessraiders`, and the cross-org release flow runs **from** the private repo **writing to** the public one — a direction needing write on `sneat-games` and nothing at all on `sneat-co`, since the workflow runs in `sneat-co` under its own `GITHUB_TOKEN`.

The recorded ask therefore does not follow from the recorded architecture. **Reproduce the 404 and read what it is actually refusing before asking the founder for any grant.** Check first: the token is SSO-unauthorised for the org; the run is a fork PR with secrets withheld; or the 404 is a missing ref or artifact rather than a permission failure at all — GitHub returns 404 rather than 403 for unauthorised private resources, which is precisely what makes this easy to misread as a permissions problem.
