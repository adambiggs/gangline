---
id: 0139
status: accepted
date: 2026-08-31
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0139: Quiet window-option reads carry a window witness

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

Tmux's quiet window-option lookup reports the same successful empty result for an unset
option on a readable window and for a window record that is no longer available.

## Consequences

Gangline's tmux PATH shim therefore precedes an explicitly targeted quiet window-option
read with `list-windows -t` against the same socket. A live target retains tmux's
ordinary option result; an unavailable target exposes the nonzero observation instead of
being reported as an unset option. A responding server with no target exits 1; a server
that no longer answers exits 2, so a consumer never has to infer which record was
unavailable from one overloaded status. Teardown classification remains on its separate
fail-closed path.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
