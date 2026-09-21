# Design decisions

These decisions record non-obvious tradeoffs that still shape Gangline 1.0.
The [constitution](../CONSTITUTION.md) defines the broader constraints.

## Append intent before external effects

Every command appends its input event and snapshots the resulting state before
it touches tmux or a native harness. The observed outcome is another event.
After interruption, replay therefore distinguishes work that was never
attempted from work whose outcome is unknown.

An effect is retried only while duplication is safe. A delivery whose input
keystrokes landed without a matching native submit witness becomes
`delivery_unverified` and is not sent again automatically.

## Verify submission through native hooks

Pane paint proves only that text appeared in a terminal. Delivery requires the
native `UserPromptSubmit` hook to report the attributed, nonce-bearing envelope.

The collar declares the comparison primitive. Codex uses exact prompt bytes.
Claude Code may wrap bracketed paste bytes in a `pasted_content` element;
Gangline accepts only a well-formed wrapper whose opening and closing IDs match,
then compares the entire inner envelope byte-for-byte. Missing, malformed, or
changed hook data remains unknown rather than success.

## Bind terminal input to the foreground harness

Before it sends any input, Gangline reads tmux's pane process and the host
process table. The expected collar executable must be a descendant in the
terminal's foreground process group. A shell or replacement program may paint
a convincing composer, but it cannot receive Gangline input; the refusal is
recorded as the effect's outcome.

Process identity corroborates the native composer and hook evidence rather
than replacing either. Keeping the process-tree query in `substrate` also keeps
host and tmux details out of harness primitives and the command state machine.

## Put harness differences in CUE collars and Go primitives

The command layer has no branches on harness names. A collar declares launch
arguments, hook payload mappings, primitive selections, actions, and context
bands. Branching logic lives in a named Go primitive so it is testable and a
third-party collar can select the same behavior.

Operator collars pass through the same schema and loader as embedded collars.
Unknown fields and primitive names fail before launch.

## Leave native choices with the native harness

Gangline recognizes trust, authentication, and permission surfaces but never
answers them. A hitch that is not ready exits with status 4 and points the
operator at the pane. This keeps security choices out of collars and preserves
the harness's own interaction model.

## Keep the event log authoritative

The per-team JSONL log is the source of truth. Snapshots carry the event count,
log byte length, and digest needed to validate that they describe the same log;
otherwise state is replayed.

`gang down` is the deletion path: after active hitches stop, it removes the
team directory. Evidence that must outlive teardown is copied before that
command.

## Keep the mandatory gate immediate

Unit tests use supplied times and direct state. Black-box scenarios use private
tmux sockets and event barriers. The local gate has a hard ceiling below two
minutes and never exercises the operator's live team.
