// Copyright 2026 Sneat.app

// engineRelease.mjs finds and downloads the engine wasm release asset
// task-19 (spec/plans/publish-the-standard-bot.md) publishes on THIS
// repository: a GitHub Release tagged `engine/<12-hex-engine-commit>`
// carrying chessbots.wasm, chess.wasm, wasm_exec.js, SHA256SUMS.txt and
// PROVENANCE.json (scripts/release/lib/engineRelease.mjs, private
// repository — this module only ever reads the PUBLISHED result of that
// script, never its source).
//
// Every download is verified against the release's own SHA256SUMS.txt
// before this suite trusts it as "the published wasm asset" — the plan's
// own bar for this task ("verify it against SHA256SUMS.txt rather than
// trusting the download"). No dependency beyond Node's own global fetch
// and node:crypto: the public go/bots module's own dependency floor
// (stdlib + go.starlark.net only) has no bearing on this js/ tree, but the
// same "justify every dependency" discipline still applies, and neither
// fetch nor sha256 needs one.
import { createHash } from 'node:crypto';

const ENGINE_TAG_PATTERN = /^engine\/[0-9a-f]{12}$/;

/**
 * @param {string} repoSlug e.g. "sneat-games/chessraiders"
 * @returns {Promise<{tag: string, assets: Array<{name: string, url: string}>} | null>}
 *   null when no `engine/*` release exists yet — see this file's own README
 *   neighbour for why that is a real, current possibility rather than a bug.
 */
export async function findLatestEngineRelease(repoSlug) {
  // GitHub's own releases list is already newest-first; no client-side sort
  // is needed as long as that ordering is what "latest" means here — the
  // same assumption the private repo's own `gh release list` usage makes
  // elsewhere in this plan.
  const response = await fetch(`https://api.github.com/repos/${repoSlug}/releases?per_page=100`, {
    headers: { Accept: 'application/vnd.github+json' },
  });
  if (!response.ok) {
    throw new Error(`engineRelease: GET /repos/${repoSlug}/releases -> ${response.status} ${response.statusText}`);
  }
  /** @type {Array<{tag_name: string, assets: Array<{name: string, browser_download_url: string}>}>} */
  const releases = await response.json();
  const match = releases.find((r) => ENGINE_TAG_PATTERN.test(r.tag_name));
  if (!match) return null;
  return {
    tag: match.tag_name,
    assets: match.assets.map((a) => ({ name: a.name, url: a.browser_download_url })),
  };
}

/**
 * Downloads every required asset from `release.assets` into memory,
 * verifies chessbots.wasm and wasm_exec.js against SHA256SUMS.txt (the
 * conventional `sha256sum -c` format: `<hex>␠␠<name>` per line — the same
 * format the private repo's publisher writes and `sha256sum -c` reads), and
 * returns their raw bytes. Throws, naming the exact file and digest
 * mismatch, on anything that fails to verify — this suite never runs
 * against an unverified download.
 * @param {{tag: string, assets: Array<{name: string, url: string}>}} release
 * @returns {Promise<{wasm: Buffer, wasmExec: string}>}
 */
export async function downloadVerifiedEngineAssets(release) {
  const required = ['chessbots.wasm', 'wasm_exec.js', 'SHA256SUMS.txt'];
  const byName = new Map(release.assets.map((a) => [a.name, a]));
  for (const name of required) {
    if (!byName.has(name)) {
      throw new Error(
        `engineRelease: release ${release.tag} is missing required asset "${name}" ` +
          `(has: ${[...byName.keys()].join(', ')})`,
      );
    }
  }

  const sumsText = await fetchText(byName.get('SHA256SUMS.txt').url);
  const sums = parseSha256Sums(sumsText);

  const wasm = await fetchBuffer(byName.get('chessbots.wasm').url);
  verifyDigest('chessbots.wasm', wasm, sums);

  const wasmExecBuffer = await fetchBuffer(byName.get('wasm_exec.js').url);
  verifyDigest('wasm_exec.js', wasmExecBuffer, sums);

  return { wasm, wasmExec: wasmExecBuffer.toString('utf8') };
}

/** @param {string} text sha256sum -c compatible: "<hex>  <name>" or "<hex> *<name>" per line */
export function parseSha256Sums(text) {
  const out = new Map();
  for (const line of text.split('\n')) {
    const m = /^([0-9a-f]{64})[ ][ *](.+)$/.exec(line.trim());
    if (m) out.set(m[2], m[1]);
  }
  return out;
}

function verifyDigest(name, buffer, sums) {
  const want = sums.get(name);
  if (!want) throw new Error(`engineRelease: SHA256SUMS.txt names no digest for "${name}"`);
  const got = createHash('sha256').update(buffer).digest('hex');
  if (got !== want) {
    throw new Error(`engineRelease: ${name} digest mismatch — SHA256SUMS.txt says ${want}, downloaded file hashes to ${got}`);
  }
}

async function fetchBuffer(url) {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`engineRelease: GET ${url} -> ${response.status} ${response.statusText}`);
  return Buffer.from(await response.arrayBuffer());
}

async function fetchText(url) {
  return (await fetchBuffer(url)).toString('utf8');
}
