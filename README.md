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

## What's not here

Chess Raiders is built by the Sneat platform. Its implementation specs —
server architecture, the game engine, client internals, and product and
business decisions — are maintained privately in `sneat-co/chess`. This
repository intentionally stops at the rules: what happens on the board,
never how it is made to happen.

## Rights

Rules text © Sneat Co., licensed under
[CC BY-NC-ND 4.0](https://creativecommons.org/licenses/by-nc-nd/4.0/) — share
with attribution; no commercial use, no derivatives. Game mechanics
described here are, as mechanics, not the licensed material. See
[LICENSE](LICENSE).
