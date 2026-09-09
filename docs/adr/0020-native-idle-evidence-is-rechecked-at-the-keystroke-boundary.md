---
id: 0020
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0020: Native-idle evidence is rechecked at the keystroke boundary

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The record speaks for a turn, not for the composer at the keystroke, and the harness can
open its next turn on its own the instant the last one ends (queued input, a
continuation of its own).

## Consequences

So the witness is read a second time under the pane lock, together with the pane's busy
verdict, immediately before the command is typed; a rollout whose newest later turn is
still open, or a pane that no longer reads idle, refuses the boundary with nothing
typed. A later turn whose terminal record has also landed is idle, so a missed hook
cannot turn an older payload into a permanent busy veto. The binding travels with the
request: one recorded against a native-idle witness is never released by a collar that
later declares less, because without the reader the read cannot happen, so that boundary
refuses and the request waits for a collar that defines it. The helper reads past
nothing it cannot correlate: only `turn_aborted` may go unnamed (it is attributed to the
turn it interrupted); a `task_complete` naming no turn, or a turn id that is not a
string, is no answer for a named turn.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
