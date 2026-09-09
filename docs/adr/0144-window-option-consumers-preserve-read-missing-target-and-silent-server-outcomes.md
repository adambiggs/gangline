---
id: 0144
status: accepted
date: 2026-08-31
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0144: Window-option consumers preserve read, missing-target, and silent-server outcomes

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

The guard can now separate a live window's unset option from a window that is gone, and
a consumer that discards that status puts the distinction straight back: an empty value
is reported as an absence nobody observed, and a bare strict-mode read ends its command
on a shell status that says nothing about what the command did or did not do. `gang
status` printed part of a report and exited on the guard's line with no account of its
own; `gang flush` exited before the recall key without saying the key was never pressed.

## Consequences

Reads that make a surface lie or die therefore go through one reader that keeps all
three answers — read, target gone, server silent — and each caller supplies the clause
only it can write: what this costs, in its own terms. Status stops at the first
unreadable read, because every field below it is unknown rather than absent and a
cascade of unreadable lines is not more information. `stage_clear` does the opposite and
clears nothing: a record that cannot be read is not a record that is gone.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
