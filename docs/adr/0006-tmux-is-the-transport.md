---
id: 0006
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0006: tmux is the transport

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

Represent a team as one tmux session and each agent as a named window.

## Consequences

Use the tty for input, pane capture for observation, window options for ephemeral state,
and collars for harness-specific knowledge; this keeps agents observable and
controllable without a daemon, database, or private protocol.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
