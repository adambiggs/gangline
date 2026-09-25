# Concepts

Gangline names, hitches, connects, and observes agents, delivers their messages
with sender attribution, and coordinates context compaction. Its startup
contract explains delivery and reporting. Your instructions define units of
work, decisions, review, and acceptance; Gangline does not track tasks or
claims.

## Agents, registrations, and sessions

An **agent** is a Claude Code or Codex process running in a named tmux window.
A **team** is the collection selected by `GANG_SESSION`. `gang up` starts its
first agent with the `lead` role and attaches your terminal. The lead's brief
asks it to follow operator policy when assigning work to named agents.
Your instructions decide the work and staffing.

A registration records an agent with a fixed ID and a name you can change with
`gang rename`. Dropping and hitching the same name creates a new registration.
A failed agent keeps its name until it is dropped.

A **native session** is the conversation history owned by Claude Code or
Codex. It is separate from the tmux session that holds the team. A new
registration can resume a native session with `--resume SESSION`.

## Messages and envelopes

Agents send messages by name. Gangline wraps each message in an **envelope**
that identifies the sender and delivery attempt. A registered pane supplies
an observed agent name. A sender supplied with `--from` is marked
`self-declared:` because Gangline did not observe that identity.

A `gangline:` sender identifies a message emitted by Gangline. Text supplied
by the caller, such as an interrupt reason, still has an unverified author.
Text typed directly into a window is operator input without a teammate's
envelope.

The receipt tells you who holds the input:

| Receipt | Meaning |
| --- | --- |
| `queued` | Gangline holds the message until the recipient can take it. |
| `accepted` | The native input queue owns it. Do not resend. |
| `delivered` | The native submit hook confirmed the exact message. |
| `unverified` | Input may have been typed, but its receipt could not be confirmed. Inspect the pane before acting. |

None of these receipts means the agent read or acted on the message. A reply
is separate evidence. Queued messages wait while a recipient is alive;
dropping it fails pending messages.

## Roles and startup instructions

A **role brief** describes how an agent should work. The startup **contract**
explains messaging and reporting. Optional operator **doctrine** supplies your
own policy. Gangline combines these with the task supplied when you hitch the
agent.
An `assignment` envelope contains work to begin; a `startup` envelope without
a task supplies context only.

These instructions are read when the agent is hitched. Editing them does not
change an agent that is already running. The [reference](reference.md#startup-instructions)
lists their override files.

## Context and compaction

**Context** is the native harness's reported conversation usage. Gangline
reports missing readings as unknown. A collar can define context bands;
crossing one upward queues a notice telling the agent to save its work and
compact at a suitable stopping point.

Compaction requests wait for a native idle boundary. Gangline holds the resume
note until it sees native completion, then delivers it before other queued
messages. Submitting the compact command alone does not establish completion.

## Collars and hooks

A **collar** is a CUE file that tells Gangline how to launch and communicate
with a particular harness. It is a declarative, schema-validated integration
contract: the collar describes the connection, and Gangline implements the
behavior. The bundled collars support Claude Code and Codex.

A **hook** is a native callback used to
report an event such as message submission or turn completion. Hooks and
native readings let Gangline distinguish what happened from what merely
appeared on screen.

For package boundaries, state files, and delivery mechanics, see
[internals](internals.md).
