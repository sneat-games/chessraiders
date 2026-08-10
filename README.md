# Chess Raiders

**Chess Raiders** is a real-time war strategy game played on a chessboard:
no turns, no check, whole teams sharing one army, prisoners and convoys
instead of a simple kill, and a captured-king delivery as the only way to
win. Play it at [chessraiders.com](https://chessraiders.com).

This repository is the public, authoritative reference for Chess Raiders'
**gameplay rules** — what a player can rely on when planning a raid, not how
the game is built. It is maintained with [SpecScore](https://specscore.md),
the spec-first workflow behind this project, and can be explored or
questioned live at [SpecScore Studio](https://specscore.studio).

## What's here

Every rule feature lives under [`spec/features/`](spec/features/README.md)
as a lint-clean SpecScore Feature: a summary, the problem it solves, its
behavior as `MUST`-level requirements, and Given/When/Then acceptance
criteria a player could verify by playing. Start with the
[features index](spec/features/README.md) for the full list, or read the
[live rules on the site](https://chessraiders.com/rules/) for the
narrated version of the same rules.

## The bot

[`go/`](go/) holds Chess Raiders' standard bot as source you can read, fork and
run: the Starlark script it decides by, the parameter table whose rows are its
difficulty tiers, and the interpreter that executes them. It is a Go module,
`github.com/sneat-games/chessraiders/go`, and it depends on nothing but the Go
standard library and `go.starlark.net` — an invariant enforced in CI, so that
anyone can build it without access to anything of ours.

## What's not here

Chess Raiders is built by the Sneat platform. Its implementation — the game
engine, server architecture, client internals — and its product and business
decisions are maintained privately in `sneat-co/chess`.

So this repository is the rules and the bot: what happens on the board, and how
the built-in opponent decides. It stops short of how any of it is made to
happen.

## Rights

Two licences, and the boundary is the `go/` directory.

**Rules text** — everything outside `go/` — © Sneat Co., licensed under
[CC BY-NC-ND 4.0](https://creativecommons.org/licenses/by-nc-nd/4.0/): share
with attribution; no commercial use, no derivatives. Game mechanics described
here are, as mechanics, not the licensed material. See [LICENSE](LICENSE).

**Everything under [`go/`](go/)** is [Apache-2.0](go/LICENSE) instead. The
rules text is published to be read; the bot is published to be *forked*, and a
NoDerivatives licence would forbid exactly that. Apache-2.0 carries a patent
grant and asks that you state your changes.
