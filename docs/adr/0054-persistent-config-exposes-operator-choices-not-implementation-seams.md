---
id: 0054
status: accepted
date: 2026-08-12
supersedes: []
superseded-by: []
tags: [configuration]
---

# ADR-0054: Persistent config exposes operator choices, not implementation seams

## Context

Operator choices need durable configuration without exposing internal implementation
seams as promises.

## Decision

Only variables with a live persistent override consumer join `GANG_CONFIG_KEYS`.

## Consequences

`GANG_ACTIVITY_LIMIT` and `GANG_CLEAR_PRESSES` retain their environment reads but have
no repository, test, environment, or audited operator override; copying their defaults
into the file parser created public promises without users. Config files that named them
now refuse as unknown so the removed surface cannot look accepted while doing something
else.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
