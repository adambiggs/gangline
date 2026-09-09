---
id: 0061
status: accepted
date: 2026-08-04
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0061: Stale data is refused from standing documentation

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

A data point that is stale the instant it is recorded does not belong in standing
documentation.

## Consequences

Do not record changing counts, versions, sizes, timings, or tallies; point to the
command that measures them, and retain a measurement only when it is dated evidence
without which a decision's rationale would fail.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
