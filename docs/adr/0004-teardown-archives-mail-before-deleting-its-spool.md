---
id: 0004
status: accepted
date: 2026-08-11
---

# ADR-0004: Teardown archives mail before deleting its spool

## Context

A message parked for an agent is the only copy of what its sender said. Teardown
used to unlink the spool with it, and a `gang mail | tail` destroyed the head of
a lead's directive with nothing to recover it from.

## Decision

`gang drop` and `gang down` move every waiting or held entry into a
human-readable archive before deleting the spool, and refuse to end anything if
that archive cannot be written. `gang mail` moves each entry into the same
archive before printing it.

## Consequences

A shell filter can hide what `gang mail` printed but cannot erase the only copy.
Archives are recovery state: Gangline never deletes them on its own, and the
read prints the command that does.
