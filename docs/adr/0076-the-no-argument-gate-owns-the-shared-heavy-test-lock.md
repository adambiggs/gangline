---
id: 0076
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [testing, gates]
---

# ADR-0076: The no-argument gate owns the shared heavy-test lock

## Context

Mandatory evidence must distinguish the behavior under test from timing and fixture
artifacts.

## Decision

The no-argument gate also takes the shared heavy-test lock itself through `flock -o`;
callers run `test/gate.sh` directly.

## Consequences

Closing the descriptor in the command process keeps disposable tmux servers from
inheriting the open file description and retaining the lock after the gate wrapper dies.
Read-only and snapshot helper modes do not take the heavy lock.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
