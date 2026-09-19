---
id: 0016
status: accepted
date: 2026-09-05
---

# ADR-0016: A scoped hitch has an immutable cgroup identity

## Context

`gang rename` changes an agent's registered name without restarting its pane.
Scope units named after that name stayed active after the name was freed, so the
next hitch of the name was refused by a unit the registry said was gone
(b877b49).

## Decision

Each scoped hitch mints an immutable 16-hex-digit identity and launches in
`gangline-<session>-<hitch-name>-<hitch-id>.scope`, recorded in the window's
`@gl_scope`. Rename leaves the unit alone.

## Consequences

A replacement can reuse a registered name. Moving a live process into a newly
named scope was rejected: it turns a metadata rename into a second platform
mutation whose partial failure would leave registration and cgroup
contradicting each other.
