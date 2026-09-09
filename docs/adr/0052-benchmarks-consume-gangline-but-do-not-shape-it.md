---
id: 0052
status: accepted
date: 2026-08-04
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0052: Benchmarks consume Gangline but do not shape it

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Every core change must have a general operator or agent consumer and a rationale that
survives removing the benchmark's name.

## Consequences

Benchmark-specific adaptation stays outside the core and hidden tests or reference
solutions are never read.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
