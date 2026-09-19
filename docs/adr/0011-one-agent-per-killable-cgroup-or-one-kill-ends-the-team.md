---
id: 0011
status: accepted
date: 2026-08-17
---

# ADR-0011: One agent per killable cgroup, or one kill ends the team

## Context

A tmux server inherits the cgroup of whatever started it, so every agent on a
team lived in the login session's scope. `systemd-oomd` kills the leaf cgroup
holding the most swap, and a session full of dormant agents is by construction
that leaf: on 2026-08-16 one kill ended a whole team at once.

## Decision

With `GANG_SCOPE=on`, each hitched launch runs in its own transient systemd user
scope, so each agent is its own leaf and is named in the kill message. Where a
scope cannot be created the hitch is refused rather than run unscoped.

## Consequences

Losing one named agent replaces losing the team. Scoped agents also fall under
the user manager's memory-pressure policy, which can take one agent for pressure
it did not cause; the thresholds remain the operator's.
