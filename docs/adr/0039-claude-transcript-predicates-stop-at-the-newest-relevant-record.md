---
id: 0039
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0039: Claude transcript predicates stop at the newest relevant record

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Fatal-state and auto-resume readers walk the append-only JSONL backward and stop once
the record that decides their predicate is found. Earlier bytes cannot change that
answer and are not reread at every idle event. Every complete record that could still
outrank the answer must parse; a final line without its newline is an append in flight
and does not yet count as a record. This bounds ordinary reads to the relevant tail
without incremental state, rotation, or cleanup.

## Consequences

The continuation marker is visible to the agent in its own transcript. An agent that
reads it can in principle reproduce it, so it is an ownership witness under Gangline's
single-tenant trust model, not an authentication boundary. Hiding or authenticating it
would require a different native witness and must not be smuggled in as anti-tamper
machinery.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
