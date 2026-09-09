---
id: 0107
status: accepted
date: 2026-08-18
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0107: Permission from pane absence requires two settled reads

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

What closes the gap is not trusting any single absence where absence is spent as
permission. The settled check takes two looks and compares their statuses as well as
their contents: a box drawn for one and absent for the other is refused whichever way it
moved, because a harness painting or dropping its composer and a collar reporting a
refused read as an absence are the same reading from here, and neither is a box nobody
is typing into. That holds whoever wrote the collar.

## Consequences

The rule binds every pane reading a collar takes, not only the composer.
`collar_context` spends nothing — the command ends either way — but a refused capture
reaching its parser makes it report a missing context readout on a pane nobody read,
which points an operator at the harness when the fault is the transport. It reads into a
variable and refuses with a status of its own.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
