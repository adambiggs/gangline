---
id: 0115
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0115: An abandoned staging fragment is asked about its writer, not a clock

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

`spool_stage` writes a body under a name no drain reads; `spool_commit` renames it into
the deliverable namespace.

## Consequences

A sender that died between the two left a message no surface named. The filename already
carries the writing pid, so the question "is this a casualty or a send from a moment
ago" is answered by asking whether that process still answers — the same evidence the
delivery lock already trusts about its own holder.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
