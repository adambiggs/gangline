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
   harness's own submit hook reports it. Otherwise the send fails visibly.
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

Choices whose reasons aren't obvious from the code. Where the code still uses
the old team lock and log replay, it's being moved over to what's described
here.

### Record intent before acting

Gang records what it's about to do before touching tmux or a harness, then
records what happened. If it dies in between, the unfinished step is marked
unknown and never retried, so a message is never typed twice.

### Check what was submitted, not the screen

Text on screen only proves it was pasted. Gang checks that the harness's submit
hook reports the exact message it sent, including its one-time ID. Anything
missing or different counts as unknown, not delivered.

### Only type into the harness

Before typing, gang checks that the harness process is in the pane's
foreground. A shell that just looks like a composer gets refused.

### Kill only what the agent started

Dropping an agent also kills the processes it started, including ones that
detached. Gang records them before closing the pane and signals them through a
kernel handle, never a bare PID, so a reused PID can't hit an unrelated
process.

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

An agent's state, inbox, and events live in its own directory, and gang locks
only the agent it's acting on.

### Show state in window names

Window names show each agent's state: `?name?` changing, `~name~` idle,
`-name-` working, `!name!` blocked or failed. A failed agent keeps its name
until it's dropped, so it can't be mixed up with a replacement.

### Hooks append and exit

A hook appends one line to its agent's event file and exits, without taking a
lock.
