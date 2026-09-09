---
id: 0104
status: accepted
date: 2026-08-17
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0104: Pane death is ordered through a FIFO held by the pane

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

A death is ordered behind the pane's own file descriptors instead. The pane holds a fifo
open read-write so its own open cannot block, and reading that fifo to EOF ends when the
pane's last descriptor closes, so the process is gone rather than merely typed at. A
tmux round trip with a child of its own then drains any corpse the server had not
reaped, and the settled fact is asserted immediately, so an unsettled death is a loud
red rather than a hang.

## Consequences

The fifo is opened in the pane's own launch command wherever the fixture writes one. A
launch command runs before the shell reads anything, so the descriptor exists as soon as
tmux has spawned the pane, and a pane that is alive but never reads its input cannot
park the fixture waiting to open it. Where the launch belongs to a hitched agent the
line is typed instead, on the same established readiness the exit typed after it already
rests on; that is the suite's ordinary standard for a typed barrier, and it is the
remaining place where a shell that stopped reading would park rather than fail.

Why tmux fails to run its SIGCHLD handler in this window is not established, and the
rule does not depend on it: a fixture that cannot be broken by a late reap does not need
the cause.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
