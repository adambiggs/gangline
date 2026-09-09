---
id: 0001
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0001: Gangline provides substrate without coordinating work

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Provide local harness lifecycle, transport, observation, and compaction primitives.

## Consequences

Do not manage roles, work allocation, or agent behaviour; those policies belong to the
operator and the native harnesses. Coordination is declarative: express goals, roles,
status, handoffs, and lead heuristics through prose or native harness features. Gangline
defines no coordination schema, reporting protocol, or lead state machine.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
