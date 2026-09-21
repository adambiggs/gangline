# Architecture

Gangline is a single Go binary with a functional core and a thin imperative
shell. The shell observes the outside world, turns observations into events,
and executes effects chosen by the core. This keeps terminal and process
quirks away from state transitions and makes recorded sessions replayable.

## Packages

`core` owns Gangline's vocabulary and state machine. Its sealed `Event` and
`Effect` types make every case explicit. `Step(State, Event)` is deterministic:
it performs no I/O, reads no clock, and creates no identifiers. Times,
identifiers, and external outcomes arrive in events.

`substrate` defines the only implementation boundary. A substrate can spawn and
stop panes, send input, inspect composer state, and capture a parsed screen.
Screens are grids of attributed cells plus a cursor; backend escape sequences
do not cross the package boundary. The production implementation uses tmux; the
acceptance suite drives a disposable private tmux session.

`store` owns the versioned state root, per-team lock, append-only event log, and
snapshots. The event log is the source of truth. A snapshot is only a shortcut
for folding the log through `core.Step`.

`harness` contains reusable operations over parsed screens and input: startup
prompt handling, composer reads, submission, hook installation, turn-boundary
detection, model identification, and wedge detection. It also loads CUE
collars. Neither `harness` nor `substrate` imports `core`; both remain useful
without Gangline's team and message concepts.

`cmd/gang` parses commands and connects the packages. Commands and hooks share
one execution loop:

1. lock one team's store;
2. load its state and turn the input into an event;
3. append the event, then fold it with `core.Step`;
4. persist the resulting state and release the lock;
5. execute returned effects; and
6. feed each external outcome back through the same loop as another event.

Appending before effects means a crash leaves an inspectable intent. A later
invocation can retry a delivery or record that its pane disappeared without
inventing partial state.

## Schemas and collars

Events have a CUE schema in `core/schema/events.cue` and an exported JSON Schema
in `core/schema/events.schema.json`. Event decoding validates input before it
constructs a core value.

Collars are CUE data validated against `harness/schema/collar.cue`. A collar
declares a launch command, native hook templates, and named harness primitives;
conditional behavior belongs in a Go primitive. Shipped collars use the same
loader as third-party collars.

## Failure model

Invalid state transitions do not panic or return errors. `Step` emits a
`RecordEvent` effect containing a rejection event and leaves state unchanged.
I/O failures are returned with context by the package that observed them and
become outcome events at the command boundary. Waits use explicit deadlines;
timeouts and wedges are durable events with evidence, not inferred success.

## Verification

The contribution gate formats and vets Go, checks exhaustive switches over sum
types, enforces the package import boundary, runs unit tests, and exercises a
real tmux session with a fake harness. The shell implementation remains in the
tree as the behavior reference until the Go command reaches cutover.
