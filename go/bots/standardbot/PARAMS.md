# The standard bot's parameter data

This bot's difficulty tiers are pure data, resolved through three layers:
**authored** (what a human edits), **resolution** (the one piece of code
allowed to fill in a default), and **generated** (a materialized artifact for
a consumer that cannot run that code). This document is the map of all
three; [`README.md`](README.md) covers what a row *means* and how to change
one number.

## 1. Authored layer

[`params.schema.json`](params.schema.json) is the single authority for every
input's default, bounds and meaning. It is a directly consumable JSON Schema
draft 2020-12 document: seventeen flat properties (eleven score weights,
three scheduling knobs, three doctrine switches — see `README.md`'s tables),
each with a `type`, `description`, `default`, and numeric `minimum`/`maximum`
where relevant. Nothing outside this file may declare or restate a default.

[`params.json`](params.json) is a sparse envelope:

```json
{
  "schema": "chess-raiders-bot-parameter-sets/v1",
  "sets": {
    "adviser": { "...": "..." },
    "commander": {},
    "lieutenant": { "...": "..." },
    "recruit": { "...": "..." }
  }
}
```

Each named set under `sets` carries **only its deltas** from
`params.schema.json`'s defaults — a property present in a set must differ
from that property's schema default. `commander` is the empty object `{}`
precisely because Commander *is* the schema defaults with nothing overridden.
This is delta-enforcement, and it is not a style preference: it is checked
code, every time a set is resolved. `manifest.ResolveParameterSet` (see
layer 2) validates *every* named set in the envelope on every call —
including sets other than the one selected — and rejects any set that
duplicates a schema default with `"duplicates its schema default; named sets
must contain differences only"`
([`go/bots/manifest/parameters.go`](../manifest/parameters.go),
`validatePartialParameterConfig`'s `enforceDelta` path). The behavior is
covered directly by
`TestNamedParameterSetsContainOnlySemanticDeltas`
([`go/bots/manifest/parameters_test.go`](../manifest/parameters_test.go)),
and exercised against this package's own `params.json` by every test and
generator call that resolves any tier, because resolution validates the
whole envelope, not just the requested row.

## 2. Resolution layer

**`ResolveParams(name string) ([]byte, error)`**, exported from this
package ([`github.com/sneat-games/chessraiders/go/bots/standardbot`](script.go)),
is the only code allowed to turn a sparse set plus the schema into a
complete row. It is a thin, cached wrapper over
**`manifest.ResolveParameterSet`**
([`github.com/sneat-games/chessraiders/go/bots/manifest`](../manifest/parameters.go)),
which does the real work: parses and validates `params.schema.json`,
validates every set in `params.json` (delta-enforcement, layer 1), and
expands the selected set into a complete, deterministic JSON object — every
one of the seventeen schema properties present, using the set's own override
where supplied and the schema default everywhere else.

Nothing else may re-derive a default or fill in an omitted property. Go
hosts — this package's own tests, and the private implementation, which
depends on this module at a released tag — call `ResolveParams` directly at
runtime. The generator (layer 3) calls nothing else either.

## 3. Generated layer

[`params.resolved.json`](params.resolved.json) materializes every named
set's complete row ahead of time:

```json
{
  "schema": "chess-raiders-bot-resolved-parameters/v1",
  "rows": {
    "adviser": { "...": "complete 17-key row..." },
    "commander": { "...": "..." },
    "lieutenant": { "...": "..." },
    "recruit": { "...": "..." }
  }
}
```

It is produced by `go generate` — the `//go:generate go run ./gen`
directive in [`resolved_params.go`](resolved_params.go) runs
[`gen/main.go`](gen/main.go), which calls this package's own
`MarshalResolvedParams()` (in turn `ResolveParams`, layer 2, once per named
set in `params.json`) and writes the result. The generator contains no
resolution or default-filling logic of its own — only marshaling. **This
file is generated output and is never hand-edited.**

Staleness is not possible to ship: `resolved_params_test.go`'s
`TestResolvedParamsFileMatchesTheResolver` regenerates the rows in memory on
every `go test ./...` run and fails with the exact regenerate command if the
committed file differs from what `ResolveParams` returns, byte-for-byte. CI
already runs `go test ./...`, so this is a hard gate, not a convention.

## 4. Consumers

- **Go hosts call the resolver directly.** Anything running Go and linking
  this module — this package's own tests, and the private implementation via
  its `go.mod` dependency on a released tag — calls `standardbot.ResolveParams`
  and never touches `params.resolved.json`.
- **Synchronous JSON consumers load `params.resolved.json`.** The motivating
  case is Chess Raiders' private webapp's `wasmBridge.ts`, which needs a
  complete parameter row at load time but cannot run Go, and needs the load
  to be synchronous rather than awaiting a resolver call. It reads
  `params.resolved.json` and indexes into `rows[<tier>]`.

This is the entire reason the generated layer exists: so that **no consumer,
in any language, ever reimplements `ResolveParameterSet`'s default-filling**.
A JSON-Schema resolver written a second time in TypeScript is exactly the
drift risk this artifact avoids.

## 5. Change workflow

1. Edit `params.json` and/or `params.schema.json`.
2. From the `go/` module root: `go generate ./bots/standardbot`.
3. `go test ./...` proves the resolver, the schema, and the regenerated
   `params.resolved.json` all agree — this is the same gate CI runs.
4. Open a PR.
5. Once merged, the module is tagged for release — e.g. `go/v0.0.6` of
   `sneat-games/chessraiders` (fully qualified; never a bare version number)
   — which is what the private implementation's `go.mod` actually pins.

## 6. Why two files instead of one

`params.json` stays sparse so the *authored* diff of a tuning change is
small and reviewable — raising Recruit's `safety` from `0.5` to `0.6` is a
one-line diff, not a seventeen-key rewrite, and every value that isn't named
still comes from exactly one place (`params.schema.json`) instead of being
copy-pasted into every tier and left to drift. `params.resolved.json` exists
for the opposite reason: it is a zero-logic, complete artifact for consumers
that need every value already resolved and cannot or should not run the
resolver themselves. Keeping both means the authored source stays minimal
and reviewable while every consumer — Go or not — still sees the exact same
resolved values, because both paths terminate in the same call to
`ResolveParams`.
