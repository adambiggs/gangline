---
id: 0143
status: accepted
date: 2026-08-31
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0143: An orphaned spool earns a roster warning by its contents, not by its state

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

The warning exists because a spool whose window is gone is unreachable by every command
Gangline has: each of them resolves a spool through `@gl_spool` on a live window, so
only the roster line stands between held mail and nobody ever reading it. That argument
is entirely about mail. An orphan holding nothing has none, nothing to lose, and no
repair to ask for — it is a reservation the next session opening sweeps — so its line
can only ever say to ignore it, and a warning class an operator learns to skim takes the
lines that name real mail down with it. The roster therefore reports an orphan only when
something is in it, and always says how much.

## Consequences

Reporting is not removing. The token an empty orphan names is a reservation, and
`spool_orphans` reads one tmux server, so a window on another may still hold it;
removing on a read would hand that identity to a later mint and enrol two windows on one
spool. Removal stays with the sweep at a session opening, where the server being read is
the one gang has just proved it can talk to.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
