// Copyright 2026 Sneat.app

// trajectory.mjs is the reusable match-driving loop both trajectory.test.mjs
// tests below call: load a script into the with-bots wasm's Starlark
// runtime, then decide/apply/settle in a loop for ONE side while the other
// side issues no commands at all — "an opponent that issues no commands" is
// literal here: this module never calls botDecide, and never builds or
// applies a command, for the passive side.
//
// NOTHING HERE DECIDES WHETHER A MOVE IS LEGAL, WHAT A CAPTURE RESOLVES TO,
// OR WHICH CANDIDATE A SCRIPT SHOULD PREFER. Every one of those questions is
// asked of the wasm (botDecide for the decision, resolveBotAttack for the
// one fallback a human tap gets for free, apply for legality) — this module
// only sequences those calls and reports what happened.
import { call } from './wasmHost.mjs';
import { intentToCommand, SIDE_WIRE } from './wire.mjs';

/**
 * @typedef {object} TrajectoryResult
 * @property {boolean} won
 * @property {number} winnerSide the numeric side (1|2) chesswasm reports
 * @property {number} decisions how many botDecide calls the active side made
 * @property {number} applied how many of those were accepted
 * @property {number} rejected how many were rejected by the engine
 * @property {number} simulatedMs
 * @property {number} wallMs
 * @property {number} maxDecisionWallMs the slowest single botDecide call
 * @property {Record<string, number>} eventCounts
 */

/**
 * Plays a whole match: `activeSide` is driven by `script` at `difficulty`
 * every `tickMs`; the OTHER side never acts. Runs until the match ends
 * (state.status becomes non-zero) or `maxDecisions` is reached — reaching
 * the cap is a FAILURE (thrown), never a silent partial result, because a
 * cap that can be satisfied by "ran out of budget and stopped" would make
 * this indistinguishable from an actual win to any caller that only checks
 * "did it throw".
 *
 * @param {object} opts
 * @param {string} opts.script Starlark source for the published bot's decide()
 * @param {'white'|'black'} [opts.activeSide]
 * @param {'recruit'|'lieutenant'|'commander'} [opts.difficulty]
 * @param {number} [opts.seed]
 * @param {number} [opts.tickMs]
 * @param {number} [opts.maxDecisions]
 * @param {string} [opts.rulesMode] one of chess.CaptureMode's own wire strings
 * @returns {TrajectoryResult}
 */
export function runPassiveOpponentTrajectory({
  script,
  activeSide = 'white',
  difficulty = 'commander',
  seed = 1,
  tickMs = 250,
  maxDecisions = 1000,
  rulesMode = 'kill_only',
}) {
  call('loadTier', { script });
  const rules = call('shippedDefaultRules', { mode: rulesMode });
  let state = call('newMatch', { rules, seed });

  const sideWire = SIDE_WIRE[activeSide];
  if (!sideWire) throw new Error(`runPassiveOpponentTrajectory: unknown activeSide "${activeSide}"`);

  let nowMs = 0;
  let memory = {};
  let applied = 0;
  let rejected = 0;
  let maxDecisionWallMs = 0;
  /** @type {Record<string, number>} */
  const eventCounts = {};
  const startWall = Date.now();
  const tally = (events) => {
    for (const ev of events ?? []) eventCounts[ev.type] = (eventCounts[ev.type] ?? 0) + 1;
  };

  let decisions = 0;
  for (; decisions < maxDecisions; decisions++) {
    const decideStart = Date.now();
    const decided = call('botDecide', { state, side: activeSide, nowMs, difficulty, row: difficulty, memory });
    maxDecisionWallMs = Math.max(maxDecisionWallMs, Date.now() - decideStart);
    memory = decided.memory ?? {};

    if (decided.intent) {
      const command = intentToCommand(
        state,
        activeSide,
        decided.intent,
        nowMs,
        (req) => call('resolveBotAttack', req),
        `trajectory-suite:${activeSide}`,
      );
      const result = call('apply', { state, command });
      if (result.rejection) {
        rejected++;
      } else {
        applied++;
        state = result.state;
        tally(result.events);
      }
    }

    nowMs += tickMs;
    const settled = call('settleDue', { state, atMs: nowMs });
    state = settled.state;
    tally(settled.events);

    if (state.status !== 0) {
      return {
        won: state.winner === sideWire,
        winnerSide: state.winner,
        decisions: decisions + 1,
        applied,
        rejected,
        simulatedMs: nowMs,
        wallMs: Date.now() - startWall,
        maxDecisionWallMs,
        eventCounts,
      };
    }
  }

  throw new Error(
    `passive-opponent check failed: ${activeSide} (${difficulty}) did not defeat a passive opponent within ` +
      `${maxDecisions} decisions (${nowMs}ms simulated, ${Date.now() - startWall}ms wall clock) — the script ` +
      `under test never forced a win. ${applied} command(s) applied, ${rejected} rejected, ` +
      `events observed: ${JSON.stringify(eventCounts)}`,
  );
}
