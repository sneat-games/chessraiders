// Copyright 2026 Sneat.app

// Package standardbot is Chess Raiders' standard bot: Script, the one
// Starlark decide() every difficulty shares (spec/features/standard-bot's
// REQ:one-script-three-difficulties), and Params, the parameter table whose
// three rows — recruit, lieutenant, commander — are the entire declared
// difference between them. Turning Recruit into Lieutenant is editing a row
// of numbers in params.json, never touching tier.star; see this package's
// own README.md for what a row means and how to change one.
//
// Both files are embedded verbatim by the two go:embed directives below,
// never copied into a Go string literal by hand, so `git diff` against
// tier.star or params.json IS the diff against what a compiled bot actually
// reads — there is no intermediate copy for the two to drift apart from.
// Load Script into go/bots/runtime.Compile to get a callable Program; the
// README walks the shortest path from there to a decision.
//
// Neither file originates here. tier.star is ported byte-for-byte from the
// private implementation's server-go/starlarktier/tier.star, and params.json
// is that repository's own contracts/chess-bot-tier-params-v1.json — itself
// generated FROM Go structs there, never hand-edited (see that file's own
// generating test, bot_tier_params_fixture_test.go, for the mechanism this
// package's copy has no way to re-run). A change to either lands upstream
// first and is re-published here; this package is a read path, not a source
// of truth for either file's content.
package standardbot

import _ "embed"

// Script is tier.star's own content, byte-for-byte: the one decide()
// function every difficulty calls, differing only in which row of Params it
// is handed. It declares no dependency of its own beyond what
// go/bots/runtime's dialect predeclares (json.encode/json.decode) — nothing
// in it reaches outside the observation, legal-move set and params a caller
// hands it.
//
//go:embed tier.star
var Script string

// Params is params.json's own content, byte-for-byte: an envelope naming its
// own format (ParamsVersion below) around one row per difficulty
// ("recruit", "lieutenant", "commander"), each row exactly the fields Script
// reads off its `params` argument by name — the ten score weights, the three
// scheduling knobs (candidateSpread, passBelow, breadth) and the three
// doctrine switches (advancedTraining, contestEnemyWork,
// sergeantPreference). A caller that wants a typed row rather than raw JSON
// decodes Params itself; this package declares no such type, so that
// decoding it is never blocked on go/bots/standardbot shipping a struct
// field the caller's own use case didn't need.
//
//go:embed params.json
var Params []byte

// ParamsVersion is params.json's own top-level "version" field, exported so
// a caller decoding Params can assert it is reading the envelope format it
// was written against rather than silently misreading a future,
// incompatible one. It is not this package's version, nor tier.star's — it
// names params.json's shape alone, and matches the private implementation's
// own fixture envelope constant 1:1 so a regenerated params.json copied here
// verbatim needs no translation.
const ParamsVersion = "chess-bot-tier-params/v1"
