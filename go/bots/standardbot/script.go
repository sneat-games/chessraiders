// Copyright 2026 Sneat.app

// Package standardbot is Chess Raiders' standard bot: Script, the one
// Starlark decide() every difficulty shares (spec/features/standard-bot's
// REQ:one-script-three-difficulties), and Params, the parameter table whose
// three rows — recruit, lieutenant, commander — are the entire declared
// difference between them. Turning Recruit into Lieutenant is editing a row
// of numbers in params.json, never touching chess-raiders-bot.star; see this package's
// own README.md for what a row means and how to change one.
//
// Params carries a FOURTH row, adviser, that the same decide() scores
// candidate moves against for explain-mode advice rather than play. It is
// deliberately its own row, not derived from and not shared with any
// playing difficulty's row — see the private implementation's own
// AdviserParams for why inheriting a playing tier's numbers cannot produce
// honest advice in either direction. It is not a fourth difficulty: there
// is no such thing as "the adviser's own difficulty" in this bot's
// vocabulary, only its own row of the same table.
//
// Both files are embedded verbatim by the two go:embed directives below,
// never copied into a Go string literal by hand, so `git diff` against
// chess-raiders-bot.star or params.json IS the diff against what a compiled bot actually
// reads — there is no intermediate copy for the two to drift apart from.
// Load Script into go/bots/runtime.Compile to get a callable Program; the
// README walks the shortest path from there to a decision.
//
// BOTH FILES ORIGINATE HERE — this package is the one and only source of
// truth for chess-raiders-bot.star and params.json; a change to either is made HERE
// first, never in the private implementation, and the private repository
// republishes what this package already carries, never the other way
// around.
//
// chess-raiders-bot.star made that crossing first, in the private implementation's plan
// publish-the-standard-bot (task-5, 2026-08-10): server-go/starlarktier/
// recruit.go no longer `//go:embed`s a local chess-raiders-bot.star at all — there is no
// local chess-raiders-bot.star left there to embed — and re-exports THIS package's own
// Script symbol instead (that file's own doc comment has the full account).
//
// params.json made the SAME crossing one task later, in task-6: this file —
// not anything hand-written in that other repository — is now the table's
// one and only author. Before task-6, the causality ran the other way:
// server-go/starlarktier/params.go hand-wrote RecruitParams/
// LieutenantParams/CommanderParams/AdviserParams as Go struct literals, and
// a generator there (bot_tier_params_fixture_test.go, since deleted) copied
// their three PLAYING rows out to a private fixture that this package's
// copy of params.json was, in turn, republished from — the Adviser row
// never made that trip, which is the historical reason the two files
// briefly were not byte-for-byte identical even though both traced back to
// params.go. Task-6 collapsed that whole chain: params.go now DECODES this
// very file's own bytes (fetched by module version, the same way
// server-go/starlarktier/recruit.go fetches chess-raiders-bot.star FROM this package)
// instead of hand-writing the numbers, so there is exactly one copy of the
// table again, and it is HERE.
package standardbot

import _ "embed"

// Script is chess-raiders-bot.star's own content, byte-for-byte: the one decide()
// function every difficulty calls, differing only in which row of Params it
// is handed. It declares no dependency of its own beyond what
// go/bots/runtime's dialect predeclares (json.encode/json.decode) — nothing
// in it reaches outside the observation, legal-move set and params a caller
// hands it.
//
//go:embed chess-raiders-bot.star
var Script string

// Params is params.json's own content, byte-for-byte: an envelope naming its
// own format (ParamsVersion below) around one row per difficulty
// ("recruit", "lieutenant", "commander") plus the Adviser's own non-playing
// row ("adviser" — this file's own package doc above), each row exactly the
// fields Script reads off its `params` argument by name — the ten score
// weights, the three scheduling knobs (candidateSpread, passBelow, breadth)
// and the three doctrine switches (advancedTraining, contestEnemyWork,
// sergeantPreference). A caller that wants a typed row rather than raw JSON
// decodes Params itself; this package declares no such type, so that
// decoding it is never blocked on go/bots/standardbot shipping a struct
// field the caller's own use case didn't need.
//
//go:embed params.json
var Params []byte

// Manifest is the strict, portable source-artifact declaration for this exact
// public standard-bot closure. It is an artifact beside Script and Params, not
// a second source of either: standardbot_manifest_test.go binds all three raw
// files into one canonical closure.
//
//go:embed chess-raiders-bot.json
var Manifest []byte

// ParamsVersion is params.json's own top-level "version" field, exported so
// a caller decoding Params can assert it is reading the envelope format it
// was written against rather than silently misreading a future,
// incompatible one. It is not this package's version, nor chess-raiders-bot.star's — it
// names params.json's shape alone. The private implementation's own
// server-go/starlarktier/params.go imports THIS constant directly and
// panics at init if a decoded params.json disagrees with it, rather than
// declaring a second copy of the string to keep in sync by hand.
const ParamsVersion = "chess-bot-tier-params/v1"
