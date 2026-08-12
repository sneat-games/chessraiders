# Handovers

Point-in-time handover documents. Each one exists so a fresh agent — or a person returning after a gap — can resume a piece of work without the originating session's context.

A handover is a **snapshot, not a specification.** It records what was measured, what was decided, what is still unknown, and what the next action is. Requirements belong in [`features/`](../features/README.md); pre-spec directions belong in [`ideas/`](../ideas/README.md) or a feature's `proposals/`. When a handover's content becomes durable, promote it there and let the handover go stale.

Treat a handover as accurate only as of its date. Verify anything actionable against the repository before acting on it.

## Index

| Handover | Date | Subject | Blocking decision |
|----------|------|---------|-------------------|
| [Standard bot vs a passive opponent](2026-08-11-standard-bot-passive-opponent.md) | 2026-08-11 | Why no difficulty beats a passive opponent, the agreed king-aggression and cover design, and the measurement still outstanding | Yes — a bot cannot win without the Beacon system, which needs a game-design decision |

## Open Questions

None at this time. Questions raised by an individual handover belong with the work they block — in the owning Feature's `## Open Questions`, or in the proposal that carries the decision — not here.

## Conventions

- One file per handover, named `YYYY-MM-DD-<topic>.md`.
- Lead with what outranks everything else in the document, so a reader who stops after the first screen still has the decisive facts.
- Separate **measured** from **inferred**, and say plainly what was not run. An honest "not measured" is worth more to the next reader than a confident number nobody observed.
