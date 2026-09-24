# Design

Gangline's principles and the decisions that follow from them. If a change
conflicts with one, change the design or change the principle first.

## Principles

1. **Use universal surfaces.** Agents are tmux windows, messages are
   keystrokes, observation is `capture-pane`, and `gang` is a CLI. No message
   bus, database, or daemon.
2. **Every message names its sender.** Gang reads the name from the sending
   window. From the operator's shell, it takes the name given.
3. **Delivered means verified.** A message is delivered only when the
   harness's own submit hook reports it. Native queue ownership is a separate
   successful `accepted` result; neither result means read or acted on.
4. **Harness differences are data.** Each harness, such as Claude Code or
   Codex, has one CUE file (its collar) that says how to launch it, hook it up,
   and read its screen. `cmd/gang` has no harness-specific code.
5. **Build only what's used.** Don't merge code that nothing calls yet.
6. **Everything Gangline writes gets cleaned up.** Every file it creates has a
   command that removes it.
7. **Fail loud.** When something breaks, the command errors. No fallbacks, no
   reporting a state Gangline didn't observe, and no code that works around
   Gangline's own bugs.
8. **Prefer prose to code.** If an agent's prompt can handle it, don't write
   code for it.
9. **Gang adds no waiting.** Gang's work for one agent never waits on its work
   for another, a hook never holds up its harness, and nothing gets slower as
   the team runs longer.
10. **Smallest fix, with its cost.** Choose the least machinery that fixes the
    root cause, and say what it costs per operation and how that grows.
11. **Security choices stay with the operator.** Gang never answers a
    harness's trust, login, or permission prompt, and a collar can't loosen a
    sandbox or approval setting.

## Decisions

Choices whose reasons aren't obvious from the code.

### Record intent before acting

Gang records what it's about to do before touching tmux or a harness, then
records what happened. If it dies in between, the unfinished step is marked
unknown and never automatically retried, so a message is never typed twice.
Explicit startup recovery can submit an original envelope still identified
exactly in the composer; it never pastes the message again.

### Check what was submitted, not the screen

Text on screen only proves it was pasted. Gang checks that the harness's submit
hook reports the exact message it sent, including its one-time ID when present. Anything
missing or different cannot count as delivered. A freshly observed native
queue entry may instead prove `accepted` when its declared layout shows the
full sender and one-time-ID opener and the composer is empty. Queue previews
can truncate bodies, so this receipt never substitutes for exact hook proof.
Accepted input leaves the spool and must not be resent. Without either proof,
the result remains unverified and the command fails visibly.

### Only type into the harness

Before typing, gang checks that the harness process is in the pane's
foreground. A shell that just looks like a composer gets refused.

### Kill only what the agent started

Dropping an agent also kills its observable descendants, including those in
separate process groups. Gang records native identity at spawn and validates it when acquiring kernel
handles for drop. It records the processes selected for teardown before
signalling them, so a reused PID cannot hit an unrelated process and a partial
drop can finish safely. Descendants already reparented away from the recorded
process tree cannot be discovered this way.

### Wait out permission prompts

An agent showing a permission prompt is marked blocked, and messages queue
until the prompt is gone.

### Deliver as soon as it's sent

Messages queue in the recipient's inbox and are typed as soon as the composer
is free, mid-turn included. They wait as long as the recipient is alive;
dropping it fails them.

### Resume after provider errors

A provider error can end a turn without a Stop hook. `gang tick` spots the
error on screen and sends one continuation per error, backing off, until an
operator-set budget runs out.

### Each agent has its own state

An agent's state and inbox live in its own directory. Gang locks only the
agent it is acting on and appends audit events to the team's log.

### Show state in window names

Window names show each agent's state: `?name?` changing, `~name~` idle,
`-name-` working, `!name!` blocked or failed. A failed agent keeps its name
until it's dropped, so it can't be mixed up with a replacement.

### Hooks append and exit

A hook appends one line to the team's audit log without taking a lock. Submit
hooks publish a witness and, when an uncertain receipt remains, start a
detached tick to reconcile it.
Turn-end and compaction-end hooks also start a detached tick for their agent
and exit without waiting.

### Keep idle teams moving

A whole-team tick replaces a transient user watchdog timer. Its next tick
re-arms before observing agents, so an idle team still drains messages and
refreshes state. Agent-scoped hook ticks leave an existing deadline alone;
one busy agent must not postpone work for idle peers. Hitch and adopt arm an
initial timer. The fixed interval is documented in [reference.md](reference.md).

The Linux scheduler uses systemd user timers, with no resident Gangline daemon.
Other platforms and hosts without a user scheduler keep ordinary ticks and log
that no watchdog is armed once per team. Timer failures are errors and audit
events; they are not silently retried. A failed arm may require another tick.

Whole-team and watchdog ticks skip occupied agent locks, including curfew
teardown. Detached agent-scoped ticks retain their existing lock wait so native
boundary notices survive contention without delaying the original hook. Timer transactions
use a separate nonblocking lock released before agent work. Concurrent ticks
leave timer replacement to its owner; cleanup reports contention rather than
claiming a timer was removed. Generation tokens reject superseded timers.
Dropping the last registration or removing the team disarms the timer; elapsed
transient units are collected. Failed agents remain registered until dropped.

Each replacement costs a bounded scheduler stop and start, independent of team
size; each whole-team tick still visits the registered agents. Scoped ticks
with an armed timer only read its marker. Scheduler calls have bounded budgets.

### Notify on context crossings

An observed upward crossing of a collar's context threshold queues a band note
through ordinary delivery. Its `[gang:context-band]` tag omits the internal ID;
only an exact submit hook can prove delivery, not a queue preview of the shared
tag. If the hook is not available immediately after submission, record the note
as unverified and release the agent lock; a later exact hook can confirm it.
The deciding reading is recorded in a
`context_band_crossed` lifecycle event. Persist publication intents beside the
native cursor so interrupted publication can resume without repeating input.
Unknown readings do not reset crossings. A lower observed reading, a model
change, or completed compaction permits later crossings again. A note states
the reading and tells the agent to save its state and compact itself with a
resume note at its next checkpoint. Once a note has been sent, the first
reading after compaction is the new baseline, so later notes do not ask the
agent to compact the context it just compacted to.

### Confirm compaction before resuming

Compaction waits for a freshly observed native idle seam, even when ordinary
messages support mid-turn input. Submitting the compact command is not proof
that it ran: withhold the resume until a newer native completion event for the
same session is observed. Refusal is failure; missing completion is unconfirmed.

### Observation failures are unknown

A probe timeout or missing submit witness describes Gangline's uncertainty,
not a wedged native turn. Report unknown with the probe or input failure reason.
A failed activity probe breaks the continuous screen observation window; a
fresh successful observation can restore busy or idle. A locked status read
also reports its observation as unavailable instead of presenting stale activity.

A nonempty composer without a native busy indicator is blocked on unsubmitted
input. The presence of a draft alone cannot establish that a turn is working;
observation never submits or discards that draft.
