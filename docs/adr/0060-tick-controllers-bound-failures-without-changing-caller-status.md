---
id: 0060
status: accepted
date: 2026-08-31
supersedes: []
superseded-by: []
tags: [ticks]
---

# ADR-0060: Tick controllers bound failures without changing caller status

## Context

Cooperative retries must make progress without creating a resident coordinator or
unbounded owner.

## Decision

Ambiguous identity always retains the lock loudly.

## Consequences

The worker accepts only the controller's fixed production budget, so noncanonical shell
arithmetic cannot move a fresh lock onto the termination path. The deadline controller
separately bounds the whole worker process group. Tick failure never changes the
spawning command's status. Catchable controller death first kills and reaps that owned
group, then re-raises the controller signal, so the new session cannot turn controller
loss into an unbounded worker. Failure is instead written to per-team health and log
state, repeated by the next invocation and status/roster, flashed to the attached
client, and raised in a dedicated tmux alerts window. This retains the
no-resident-daemon decision while keeping verified delivery and loud failure as the
non-negotiable result.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
