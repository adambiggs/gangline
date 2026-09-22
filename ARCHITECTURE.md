# Architecture

One Go module, one `gang` binary. Design rules are in
[docs/design.md](docs/design.md).

## Packages

- `core` decides what happens next: given an agent's state and an event, it
  returns the new state and the actions to take. No I/O, no clock.
- `store` keeps each agent's directory: state and inbox.
- `substrate` talks to tmux: create, read, type into, and kill panes.
- `harness` loads collars and knows how to read and drive each harness. It
  doesn't import `core`.
- `cmd/gang` is the CLI that ties them together.

## Sending a message

1. `gang send` writes the message to the recipient's inbox.
2. When the recipient's composer is free, gang types it in.
3. The harness's submit hook reports what it received; if it matches, the
   message is delivered.

## Files

Team state lives under `STATE_ROOT/teams/TEAM/`. `team.json` holds the curfew;
`log.jsonl` is an append-only audit log. Only `gang log` reads it. Name claims
are symlinks in `names/`, pointing to immutable hitch IDs.

Each `agents/ID/` contains `agent.json`, its own `lock`, the submit `witness`,
and `inbox/{tmp,new,cur,failed}/`. State and witnesses are replaced atomically.
Pending messages live in `new/`; terminal directories retain the latest
result, and the audit log retains history. Commands do work proportional to
current pending work and the agents requested, independently of settled history.

Collars live in `harness/collars/` or the directory named by `GANG_COLLARS`.
