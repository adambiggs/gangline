---
id: 0008
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0008: Harness driving is a seam, not a second product

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Keep launch syntax, composer parsing, native state, submission, and native commands
behind the small collar contract.

## Consequences

Gangline consumes that boundary internally so core decisions can consume explicit
observations and remain deterministically unit-testable. Extract a general harness
driver only when a second non-benchmark consumer exists and can define the interface
from real use.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
