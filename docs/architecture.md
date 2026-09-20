# Architecture

Use this page to understand where Gangline keeps state, how a message reaches
an agent, and where a harness-specific change belongs. If you are adding a
harness, read the [collar contract](reference.md#collar-contract) next.

Gangline is a Bash CLI over tmux. A team is a tmux session. Each agent is a
window running its native CLI harness. Gangline sends input as keystrokes,
observes panes directly, and records live per-window facts in tmux options. It
has no daemon or database.

## Runtime model

```text
operator or agent
       |
       v
    gang CLI -------- durable state
       |              queues, locks, events, archives
       v
  tmux session
       |
       +-- window: lead   -- collar --> native harness
       +-- window: worker -- collar --> native harness
       +-- window: ...    -- collar --> native harness
```

The core stays in `bin/gang`. Its section banners follow dependency order:

1. **State root** resolves team records, locks, queues, and recovery evidence.
2. **Collars** load launch commands and native evidence readers.
3. **tmux substrate** resolves registered windows, reads panes, and decides
   whether a composer can receive input.
4. **Delivery and spool** attributes messages, verifies submission, and holds
   bodies that have not reached a safe composer.
5. **Hooks and tick** turn native events into recorded facts and retry work at
   safe boundaries.
6. **Capacity** reads provider-window evidence and local usage data.

## Message path

For `gang send worker`, Gangline follows one path:

1. It derives the sender from the calling window, or accepts an explicit
   outside sender.
2. It resolves `worker` to a registered tmux window and loads that window's
   collar.
3. It asks the collar and tmux whether the native composer is safe.
4. It writes the attributed envelope and submits it.
5. It captures the pane and reports delivery only after the native UI shows
   acceptance.

If Gangline refuses before typing, it still owns the body and can put it in the
recipient's spool. A later native hook or `gang tick` retries it. If keystrokes
may already have landed but verification fails, Gangline holds the record and
does not send another copy automatically.

This distinction is the central delivery guarantee: a known refusal is safe to
retry; an unknown outcome is not.

## State ownership

Gangline keeps each fact with the layer that can prove it:

| State | Owner | Lifetime |
| --- | --- | --- |
| Window identity, collar, live status | tmux window options | Until the window is removed |
| Pending and held messages | Durable team state | Until delivery, explicit reading, or teardown archive |
| Native turn and session evidence | Harness hooks through a collar | Until replaced by newer evidence or the window is removed |
| Operator configuration | Config file and environment | Read by each command; launch choices are fixed at hitch time |
| Message archives | Durable state directory | Until the operator removes them |
| Event logs | Durable state directory | Rotating bounded generations |

A collar interprets native state. It does not weaken a harness sandbox, answer
permission prompts, or move product-specific branches into `bin/gang`.

## Process boundaries

Most commands are short-lived. `gang tick` is a bounded, one-shot maintenance
pass, not a resident watcher. Native hooks invoke Gangline when the harness
reaches an event boundary. Optional provider sampling uses the host's service
manager; it does not add a Gangline daemon.

With `GANG_SCOPE=on`, each hitched agent runs in its own transient systemd user
scope. Without it, agents inherit the tmux server's cgroup. The setting changes
process isolation, not delivery or identity.

## Implementation boundary

Bash keeps the installed path direct: the same script exposes commands, reads
tmux, and runs on macOS Bash 3.2. Keep structured harness knowledge in collars
and keep operator policy in prose or configuration. A core change is justified
when every harness needs it and tmux, the shell, or a native open interface can
carry it without adding a coordinator.

Read the [design decisions](design.md) before changing delivery, lifecycle, or
test behavior. They record the non-obvious constraints behind those paths.
