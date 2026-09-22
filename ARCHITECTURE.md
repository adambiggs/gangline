# Architecture

One Go module, one `gang` binary. Design rules are in
[docs/design.md](docs/design.md).

## Packages

- `core` decides what happens next: given an agent's state and an event, it
  returns the new state and the actions to take. No I/O, no clock.
- `store` keeps each agent's directory: state, inbox, and event file.
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

Team state lives under `${XDG_STATE_HOME:-~/.local/state}/gangline/v1/TEAM/`,
one directory per agent. Collars live in `harness/collars/` or the directory
named by `GANG_COLLARS`.
