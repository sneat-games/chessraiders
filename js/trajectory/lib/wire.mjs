// Copyright 2026 Sneat.app

// wire.mjs converts between two JSON shapes the published with-bots wasm
// artifact (chessbots.wasm) uses on its own globalThis.chesswasm surface:
//
//   - botDecide's own request/response, which names squares as algebraic
//     strings ("e4") and a side as "white"/"black" — the shape the
//     published Starlark runtime's decide() reads and returns, unchanged
//     across the wasm boundary.
//   - newMatch/apply/submit/settleDue/resolveBotAttack's own chess.Command
//     and chess.State, which encode a square as a small integer, a side as
//     1/2, a rank as 1-6 and so on — chess.Square/chess.Rank/chess.Side/
//     chess.Profession have no custom JSON marshalling, so the wasm's own
//     json.Unmarshal expects the plain Go zero-value encoding of each.
//
// EVERY MAPPING HERE IS A WIRE ENCODING, NEVER A RULE: it re-labels a
// decision the published script (running inside the wasm, via the
// published go/bots/runtime module) already made, exactly the way a human
// player's own client turns a tap into a chess.Command. It does not decide
// whether a move is legal, what a capture resolves to, or which candidate a
// script should prefer — every one of those questions still gets asked of
// the wasm itself (apply/resolveBotAttack), never answered by this file.
//
// The square<->index formula and every enum's integer value below were
// confirmed empirically against a real with-bots build (chess.Side/
// chess.Rank/chess.Profession's own constants: NoSide/NoRank/
// NoProfession == 0, then White/Pawn/Engineer == 1, ... in declaration
// order) — see this task's own PR description for the worked derivation
// (every starting unit's own algebraic square cross-checked against its
// state.units[...].square value).

const FILES = 'abcdefgh';

/** @param {string} algebraic e.g. "e4" */
export function squareIndex(algebraic) {
  const file = FILES.indexOf(algebraic[0]);
  const rank = Number(algebraic.slice(1));
  if (file < 0 || !Number.isInteger(rank) || rank < 1 || rank > 8) {
    throw new Error(`wire.squareIndex: "${algebraic}" is not a valid algebraic square`);
  }
  return file * 8 + (rank - 1);
}

/** @param {number} index */
export function algebraicSquare(index) {
  if (!Number.isInteger(index) || index < 0 || index > 63) {
    throw new Error(`wire.algebraicSquare: ${index} is not a valid square index`);
  }
  const file = Math.floor(index / 8);
  const rank = (index % 8) + 1;
  return `${FILES[file]}${rank}`;
}

export const SIDE_WIRE = Object.freeze({ white: 1, black: 2 });
export const RANK_WIRE = Object.freeze({ pawn: 1, knight: 2, bishop: 3, rook: 4, queen: 5, king: 6 });
export const PROFESSION_WIRE = Object.freeze({ engineer: 1, sergeant: 2 });

/**
 * Finds the unit of `sideWire` currently occupying `squareIdx`. This reads
 * a field the engine already computed (state.units[*].square) — it is a
 * lookup, not a legality question, exactly like a human client resolving
 * which piece its own tap landed on before ever building a Command.
 */
export function unitAtSquare(state, sideWire, squareIdx) {
  for (const [id, unit] of Object.entries(state.units)) {
    if (unit.side === sideWire && unit.square === squareIdx) return id;
  }
  return null;
}

/**
 * Turns one botDecide response's own `intent` into the chess.Command
 * chesswasm's apply/submit expect — the same translation
 * facade4chess/bots.go's botCommandFor performs for every bot-driven
 * command in production (turn an intent into the same chess.Command a
 * human tap would produce), reproduced here from its own field-by-field
 * contract since that function is private:
 *
 *   - an "action" intent addresses its own square (From == To) and carries
 *     Action/Target/Profession/Direction/Material verbatim; an unaimed
 *     `beacon_take` (no Target) defaults to the side's own CURRENT Beacon
 *     bearer — the only giver adjacency could ever resolve to when the
 *     script leaves Target empty (this is the one default this function
 *     applies on the script's behalf, and it is a lookup of a field the
 *     engine already computed — state.beacons[side].bearer — not a rule).
 *   - a "move" intent's own Choice/Promotion are used when the script set
 *     them; an unset Choice is resolved by asking the engine itself
 *     (resolveBotAttack, this module's one caller-supplied wasm call) for
 *     its own deterministic default, never derived here.
 *
 * `callResolveBotAttack` is injected (rather than imported) so this module
 * stays a pure translator with no wasm dependency of its own — the caller
 * (trajectory.test.mjs) is the one holding a live wasm host.
 *
 * @param {object} state the chess.State this command will be applied to
 * @param {'white'|'black'} side
 * @param {object} intent botDecide's own non-null intent
 * @param {number} nowMs
 * @param {(req: {state: object, unit: string, to: number, choice: string, atMs: number}) => {target: object|null, choice: string}} callResolveBotAttack
 * @param {string} playerId the wire's own free-text `player` field
 */
export function intentToCommand(state, side, intent, nowMs, callResolveBotAttack, playerId) {
  const sideWire = SIDE_WIRE[side];
  if (!sideWire) throw new Error(`wire.intentToCommand: unknown side "${side}"`);
  const fromIdx = squareIndex(intent.from);
  const unit = unitAtSquare(state, sideWire, fromIdx);
  if (!unit) {
    throw new Error(
      `wire.intentToCommand: no ${side} unit occupies ${intent.from} (index ${fromIdx}) — ` +
        `intent was ${JSON.stringify(intent)}`,
    );
  }
  const command = { player: playerId, side: sideWire, unit, from: fromIdx, at: nowMs };

  if (intent.kind === 'action') {
    command.to = fromIdx;
    command.action = intent.action;
    if (intent.target) command.target = intent.target;
    if (intent.profession) {
      const professionWire = PROFESSION_WIRE[intent.profession];
      if (professionWire === undefined) {
        throw new Error(`wire.intentToCommand: unknown profession "${intent.profession}"`);
      }
      command.profession = professionWire;
    }
    if (intent.direction) command.direction = intent.direction;
    if (intent.material) command.material = intent.material;
    if (intent.action === 'beacon_take' && !command.target) {
      const bearer = state.beacons?.[String(sideWire)]?.bearer;
      if (!bearer) {
        throw new Error(
          `wire.intentToCommand: beacon_take with no target and no recorded bearer for side ${sideWire}`,
        );
      }
      command.target = bearer;
    }
    return command;
  }

  if (intent.kind !== 'move') {
    throw new Error(`wire.intentToCommand: unknown intent kind "${intent.kind}"`);
  }
  const toIdx = squareIndex(intent.to);
  command.to = toIdx;
  if (intent.promotion) {
    const rankWire = RANK_WIRE[intent.promotion];
    if (rankWire === undefined) throw new Error(`wire.intentToCommand: unknown promotion rank "${intent.promotion}"`);
    command.promotion = rankWire;
  }
  // The one engine round trip this translator makes: resolveBotAttack
  // mirrors botCommandFor's own capture-choice fallback
  // (chess.ForcedCaptureChoice) — a script's explicit choice always wins,
  // an unset one gets the engine's own deterministic default, asked of the
  // engine rather than re-derived here.
  const resolved = callResolveBotAttack({ state, unit, to: toIdx, choice: intent.choice || '', atMs: nowMs });
  if (resolved?.choice) command.choice = resolved.choice;
  return command;
}
