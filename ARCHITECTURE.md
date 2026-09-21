# Architecture

Gangline is one Go module and one `gang` binary. Its functional core decides
state transitions; a thin command layer observes tmux and native harnesses,
turns those observations into events, and executes the resulting effects.
The principles and tradeoffs behind this structure live in
[docs/design.md](docs/design.md).

## Packages

`core` owns teams, hitches, envelopes, deliveries, compactions, events, and
effects. `Step(State, Event) (State, []Effect)` is deterministic: it performs no
I/O, reads no clock, and creates no identifiers. Event and effect families are
sealed sum types checked for exhaustive switches.

`store` owns the versioned state root, one advisory lock per team, append-only
JSONL event logs, and snapshots. The event log is authoritative; a snapshot is
only a validated shortcut for replaying it.

`substrate` defines terminal values and the common backend interface: spawn,
capture, send keys, kill, and attach. The tmux backend also owns session and
window lifecycle. It returns parsed screens made of attributed cells and a
cursor, so tmux escape sequences never reach harness logic.

`harness` loads and validates CUE collars and implements reusable native
primitives: startup recognition, composer reading, submission, submit-witness
normalization, hook decoding, context and provider-limit readings, model
discovery, and wedge detection. It does not import `core`.

`cmd/gang` parses the CLI and connects those packages. It contains no
harness-name branches; shipped and operator collars use the same CUE schema and
loader.

## Command path

Commands and native hooks use the same state loop:

1. lock one team's store;
2. load its snapshot and replay any later log entries;
3. append the input event before doing external work;
4. fold the event with `core.Step`, save the snapshot, and unlock;
5. execute the returned effects against tmux or the harness; and
6. feed each observed outcome back through the loop as another event.

## Verified delivery

A send renders a nonce-bearing attributed envelope, verifies that the native
composer is empty, pastes the envelope, waits for the TUI to settle, and submits
it. Delivery succeeds only when `UserPromptSubmit` reports the same prompt under
the collar's declared normalization.

## State and schemas

The default root is `${XDG_STATE_HOME:-~/.local/state}/gangline/v1/TEAM/`.
`events.jsonl` is the source of truth and `snapshot.json` accelerates loading.
The event schema lives in `core/schema/events.cue`.

Collars live in `harness/collars/*.cue` or an operator directory selected by
`GANG_COLLARS`. `harness/schema/collar.cue` validates launch arguments, hook
wiring, primitive selection, actions, and context bands before a harness is
started.

## Verification

`test/gate.sh` runs formatting, vet, exhaustive-sum checks, the package-boundary
check, unit tests, and black-box acceptance scenarios against a separately named
private tmux socket. The gate has a hard wall-clock ceiling below two minutes.
