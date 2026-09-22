# Design

These principles and decisions define Gangline's design. Use them to decide
whether a change belongs here; if a change conflicts with a principle,
redesign it before implementation.

## Principles

Harness machinery grows faster than the work it serves, and components built
to guard a system create defects of their own.

1. **Minimize bespoke integration surface.** Integrate only through universal
   surfaces: the tty (tmux), the shell (`gang` as a CLI), and open standards a
   harness speaks natively (e.g. MCP). tmux is the default transport — agents
   are tmux windows, messages are keystrokes, observation is `capture-pane`,
   termination is `kill-window`, and state lives in a per-team append-only
   event log. A harness-specific code path requires a section below proving no
   universal surface can carry the value. No bespoke message buses, databases,
   or daemons.
2. **Every message is attributed; trust is assumed.** A sender identity is
   required. Where Gangline can see the sending window it reads the name off
   that window and refuses a claimed identity, so an agent cannot casually sign
   as a peer. Where it cannot see one — the operator's own shell — the name
   stands as claimed. This is attribution, not authentication, and it holds
   because the system is single-tenant by design: anyone at the keyboard is the
   operator. Never build authentication, generation fencing, or anti-tamper
   into this repo.
3. **Delivered means verified.** A send is confirmed by the harness's native
   submit hook or it fails loudly. No fire-and-forget, no success receipts for
   messages nobody saw.
4. **Harness integration is a collar, not a plugin.** Per-harness knowledge
   lives in a collar, never as a harness-name branch in `cmd/gang`; the collar
   contract itself is documented in `docs/reference.md`. Code inside a harness
   requires a section below proving the value is real and unachievable any
   other way.
5. **Nothing lands without a live consumer.** If nothing invokes it the day it
   merges, it does not merge.
6. **Everything has a deletion path.** Any artifact this system produces —
   logs, state, records — must say how and when it dies.
7. **The harness never manages itself.** Gangline may not grow a component
   whose job is watching, policing, or coordinating another Gangline component.
   That loop is how a harness ends up spending most of its code on itself.
8. **Fail loud.** No silent fallbacks, no degraded modes that pretend to be
   healthy, no fabricated status. A regex that stops matching a new TUI version
   must break the command, visibly.
9. **Size is watched, not capped.** Growth must justify itself against the
   mission: support long-running multi-agent sessions with minimal machinery.
   When in doubt, the answer is prose in an agent's prompt, not code in this
   repo.

## Decisions

These decisions record non-obvious tradeoffs that still shape Gangline 1.0.

### Append intent before external effects

Every command appends its input event and snapshots the resulting state before
it touches tmux or a native harness. The observed outcome is another event.
After interruption, replay therefore distinguishes work that was never
attempted from work whose outcome is unknown.

An effect is retried only while duplication is safe. A delivery whose input
keystrokes landed without a matching native submit witness becomes
`delivery_unverified` and is not sent again automatically.

### Verify submission through native hooks

Pane paint proves only that text appeared in a terminal. Delivery requires the
native `UserPromptSubmit` hook to report the attributed, nonce-bearing envelope.

The collar declares the comparison primitive. Codex uses exact prompt bytes.
Claude Code may wrap bracketed paste bytes in a `pasted_content` element;
Gangline accepts only a well-formed wrapper whose opening and closing IDs match,
then compares the entire inner envelope byte-for-byte. Missing, malformed, or
changed hook data remains unknown rather than success.

### Bind terminal input to the foreground harness

Before it sends any input, Gangline reads tmux's pane process and the host
process table. The expected collar executable must be a descendant in the
terminal's foreground process group. A shell or replacement program may paint
a convincing composer, but it cannot receive Gangline input; the refusal is
recorded as the effect's outcome.

Process identity corroborates the native composer and hook evidence rather
than replacing either. Keeping the process-tree query in `substrate` also keeps
host and tmux details out of harness primitives and the command state machine.

### Reap only descendants recorded before pane termination

Before tmux removes a pane, Gangline records every descendant using its PID and
start time. After pane termination it signals only matching survivors, first
with `SIGTERM` and then, when direct observation still finds them, `SIGKILL`.
A final process-table observation must find none of those identities.

The snapshot catches descendants that changed session or process group with
`setsid`, while the start time prevents a reused PID from inheriting ownership.
Executable names are deliberately excluded: they neither prove ownership nor
remain stable across wrapper scripts and re-exec.

### Put harness differences in CUE collars and Go primitives

The command layer has no branches on harness names. A collar declares launch
arguments, hook payload mappings, primitive selections, actions, and context
bands. Branching logic lives in a named Go primitive so it is testable and a
third-party collar can select the same behavior.

Operator collars pass through the same schema and loader as embedded collars.
Unknown fields and primitive names fail before launch.

### Leave native choices with the native harness

Gangline recognizes trust, authentication, and permission surfaces but never
answers them. A hitch that is not ready exits with status 4 and points the
operator at the pane. This keeps security choices out of collars and preserves
the harness's own interaction model.

Runtime permission and approval surfaces are recorded as `blocked`. The collar
selects a screen primitive with separate prompt and choice rules, while a
native permission hook supplies an earlier signal when available. Delivery
stays queued until direct observation clears the surface; Gangline never types
through it or chooses an answer.

### Keep deferred delivery with its command or native boundary hook

A harness can draw a permission dialog after its composer first looked ready.
Startup delivery stays with `gang hitch`; queued sends and compaction
continuations stay with the native asynchronous Stop or PostCompact hook that
releases them. A synchronous boundary hook cannot wait for submission: the
native dispatcher must first return from that hook to accept the next prompt.

All three paths share exponential backoff and a configurable finite deadline.
The deadline bounds the lifetime of the owning command or hook; no resident
watcher is introduced. Attempts require an empty composer without the collar's
native busy marker. After input lands, a separate short witness budget limits
uncertainty, and unverified input is never retried automatically.

Compaction completion releases the continuation without requiring a later Stop.
Interruption records idle only after direct native observation confirms it,
then releases queued work. Sending Escape alone does not prove interruption.

### Keep the event log authoritative

The per-team JSONL log is the source of truth. Snapshots carry the event count,
log byte length, and digest needed to validate that they describe the same log;
changed event bytes are refused. State is replayed with the current binary even
when the snapshot's prefix matches: an upgrade can change reducer semantics
without changing event bytes. This costs a full replay per load, but prevents a
prior binary's cached interpretation from stranding the next operation after
its intent has been appended.

`gang down` is the deletion path: after active hitches stop, it removes the
team directory. Evidence that must outlive teardown is copied before that
command.

`gang wait` registers a native file notification on that log, then rechecks its
length to close the registration race. It folds each append until the target is
idle or the caller's deadline expires. A timeout is itself appended as an event;
waiting never polls or changes the target hitch's activity.

### Project recorded state into tmux window names

Managed tmux window names carry a compact projection of the event-log state:
question marks mean a lifecycle transition, tildes idle, hyphens working, and
exclamation marks blocked, wedged, or failed. The projection changes after the
event is recorded and `gang tick` reconciles drift, so a tmux rename never
becomes a second source of truth.

A failed hitch continues to occupy its agent name until an explicit drop. This
prevents an old generation and its pane from becoming indistinguishable from a
replacement, while preserving `gang drop` as the deletion path.

### Keep the mandatory gate immediate

Unit tests use supplied times and direct state. Black-box scenarios use private
tmux sockets and event barriers. The local gate has a hard ceiling below two
minutes and never exercises the operator's live team.

`gang wait` observes complete newline-terminated log records without acquiring
the snapshot writer's lock. An append notification can precede lock release;
that contention is not a failed wait. An unfinished record cannot establish
idle, and malformed completed records still fail loudly.
