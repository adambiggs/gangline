---
id: 0120
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0120: A tool call is a family of record types, not one name

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The rollout that started this was read as having no tool calls because only
`function_call` was counted; every call in it was a `custom_tool_call`.

## Consequences

A closed list that silently misses a family reports a working agent as one that has
never acted. Each collar therefore recognizes the families it has seen, and reports a
call-shaped record outside them by name as unknown — loud, in the direction that cannot
fabricate either verdict.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
