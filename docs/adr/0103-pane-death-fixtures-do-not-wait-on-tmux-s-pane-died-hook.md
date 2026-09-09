---
id: 0103
status: accepted
date: 2026-08-17
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0103: Pane-death fixtures do not wait on tmux's pane-died hook

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

No fixture may wait on `pane-died`. tmux settles a pane's death from two independent
events — the pty reaching EOF, which is what makes `#{pane_dead}` read 1, and the reap
of the child, which fills in `#{pane_dead_status}` and draws the held corpse's banner.
The hook is dispatched only from the second, and only where the first has already
landed, so a death whose EOF is processed before its reap dispatches no hook at all: not
then, and not when the reap arrives afterwards. Forcing the reap fills the status in and
leaves the channel blocked, so the signal is lost rather than late.

## Consequences

`tmux wait-for` has no bound, so a fixture holding that channel cannot go red — it can
only stop the suite and exhaust the CI cap. `gang` itself reads `#{pane_dead}`, which
the EOF alone settles, so the fact under test is true well before the hook that was
being waited on.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
