// Package bots is the root of Chess Raiders' publishable bot tree.
//
// Two things live under here, and the split is deliberate:
//
//   - runtime/     the Starlark interpreter that executes a bot script. It is
//     bot-agnostic and game-agnostic — it knows nothing about
//     chess, convoys or morale, only how to compile a script,
//     call decide(), and stop it when it exceeds its budget.
//   - standardbot/ the standard Chess Raiders bot: the script itself and the
//     parameter table whose rows are its difficulty tiers.
//
// The runtime is a SIBLING of the bot rather than a child of it, on the
// founder's 2026-08-09 direction, because a runtime executes any bot —
// standard, experimental, or one a player wrote. Nesting it under standardbot
// would assert an ownership that does not exist.
//
// # The one invariant, and why it is a test rather than a comment
//
// Nothing under this tree may import anything beyond the Go standard library
// and go.starlark.net. dependencies_test.go enforces that by walking the
// module's own source, and it is the reason this package exists at all before
// its children do.
//
// The invariant is not tidiness. This module is published so that a person who
// has never seen the private implementation can read the bot, fork it, and run
// it. Every dependency added here is a thing that person must also obtain — and
// if one of them were ever private, they could not obtain it at all. That is
// not hypothetical: sneat-co/chess spent a day of CI breakage in August 2026 on
// exactly that shape, a browser build reaching a private competitions module
// through a facade it did not need, and the fix was to make the contract
// public rather than to add credentials.
//
// It also keeps the exit open. The runtime is published here because Chess
// Raiders is the first game to need it, not because it belongs to Chess
// Raiders. If a second game wants scriptable bots, moving runtime/ to a home
// of its own should be a `git mv` and a go.mod — which stays true only while
// nothing in it has reached back into anything chess-shaped.
package bots
