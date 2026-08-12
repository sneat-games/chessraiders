// Copyright 2026 Sneat.app

// wasmHost.mjs instantiates the published with-bots wasm artifact
// (chessbots.wasm) under Node and waits for it to register
// globalThis.chesswasm. This is loading mechanics only — no game logic
// lives here, and every JSON shape this module's caller sends or receives
// crosses the wasm boundary exactly as chesswasm's own JS entry points
// define it (see main.go/bots_entry.go in the private repository for their
// canonical definitions; this file never reads either).
//
// TinyGo's own wasm_exec.js is a classic (non-ESM) script that assigns
// `globalThis.Go = class {...}` as a side effect rather than exporting
// anything, and it expects `require`/`fs`/`crypto`/`TextEncoder` to be
// reachable the way a CommonJS module or a <script> tag would provide them.
// vm.runInThisContext runs it against THIS module's own realm (a fresh
// vm.Context, or a Worker, would get an unreachable `global` of its own —
// see Node's vm docs: "does not have access to local scope, but does have
// access to the current global object"), so the assignment lands where the
// rest of this file can see it.
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

/**
 * @param {string} wasmExecSource the wasm_exec.js text shipped BESIDE the
 *   wasm binary in the same release (never a copy vendored in this tree —
 *   the two are paired by the TinyGo version that produced them, and a
 *   mismatched pair is exactly the "second copy that can drift" this
 *   suite's own README warns against).
 * @param {Buffer|Uint8Array} wasmBytes the compiled chessbots.wasm.
 * @param {number} [readyTimeoutMs]
 * @returns {Promise<{ instance: WebAssembly.Instance }>}
 */
export async function loadChessWasm(wasmExecSource, wasmBytes, readyTimeoutMs = 5000) {
  // Every `require()` call inside wasm_exec.js that this Node target
  // actually reaches is already guarded with `if (!global.X)` — Node 18+
  // provides global.crypto/TextEncoder/TextDecoder natively, so `fs` is the
  // one binding this harness has to supply itself. `global.require` is
  // deliberately left unset: wasm_exec.js's own Node-CLI-runner tail block
  // references a bare `module` identifier that only exists inside a real
  // CommonJS wrapper, and vm.runInThisContext gives it none — a truthy
  // global.require here would reach that block and throw before its own
  // guard could short-circuit it.
  if (!globalThis.fs) {
    globalThis.fs = await import('node:fs');
  }
  vm.runInThisContext(wasmExecSource, { filename: 'wasm_exec.js' });
  if (typeof globalThis.Go !== 'function') {
    throw new Error('wasmHost.loadChessWasm: wasm_exec.js did not define globalThis.Go');
  }

  const go = new globalThis.Go();
  const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
  // chesswasm's own main() never returns (it parks on `select {}` once its
  // globals are registered, so every js.Func it built stays alive) — this
  // promise is intentionally never awaited to completion in normal
  // operation.
  void go.run(instance);
  await waitForChessWasmNamespace(readyTimeoutMs);
  return { instance };
}

/**
 * Convenience wrapper over loadChessWasm for local development: reads both
 * files from disk first. Used only behind the CHESS_WASM_DIR override this
 * suite's README documents — the real suite otherwise always calls
 * loadChessWasm directly against bytes engineRelease.mjs already verified
 * against SHA256SUMS.txt, never a file this repository shipped itself
 * (nothing under js/trajectory/ commits a wasm binary).
 * @param {string} wasmExecPath
 * @param {string} wasmPath
 */
export async function loadChessWasmFromFiles(wasmExecPath, wasmPath) {
  return loadChessWasm(readFileSync(wasmExecPath, 'utf8'), readFileSync(wasmPath));
}

/**
 * main() registers `globalThis.chesswasm` synchronously before parking on
 * `select {}`, but `go.run(instance)` only returns control to this event
 * loop turn once the Go scheduler itself yields — polled with a timeout
 * rather than assumed, so a slow CI runner fails loudly and namelessly
 * rather than racing a namespace that was never going to appear on time.
 * @param {number} timeoutMs
 */
function waitForChessWasmNamespace(timeoutMs) {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    const poll = () => {
      if (globalThis.chesswasm) {
        resolve(undefined);
        return;
      }
      if (Date.now() - start > timeoutMs) {
        reject(new Error(`wasmHost: globalThis.chesswasm never appeared within ${timeoutMs}ms`));
        return;
      }
      setImmediate(poll);
    };
    poll();
  });
}

/**
 * call() wraps one globalThis.chesswasm[fnName] round trip: JSON.stringify
 * the request, invoke the export, JSON.parse the {ok,result}|{ok,error}
 * envelope every chesswasm entry point returns, throw or return the plain
 * result. This is the entire client-side contract those entry points
 * define — nothing about which fields a request needs, or what a result
 * means, is decided here.
 * @param {string} fnName
 * @param {unknown} [request]
 */
export function call(fnName, request) {
  const fn = globalThis.chesswasm?.[fnName];
  if (typeof fn !== 'function') {
    throw new Error(`wasmHost.call: globalThis.chesswasm.${fnName} is missing or not a function`);
  }
  const raw = fn(JSON.stringify(request ?? {}));
  /** @type {{ok: true, result: unknown} | {ok: false, error: string}} */
  let envelope;
  try {
    envelope = JSON.parse(raw);
  } catch {
    throw new Error(`wasmHost.call: chesswasm.${fnName} returned non-JSON: ${raw}`);
  }
  if (!envelope.ok) {
    throw new Error(`wasmHost.call: chesswasm.${fnName}: ${envelope.error}`);
  }
  return envelope.result;
}
