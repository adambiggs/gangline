---
id: 0049
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0049: A team curfew is an optional declaration

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Let the operator declare one wall-clock curfew for the team.

## Consequences

Derive exactly two relative, advisory edges from that span: yellow halfway through and
red after four-fifths. Do not invent a default, enforce the deadline, allocate per-agent
budgets, or run a patrol; the substrate exposes operator intent and each agent decides
how to respond.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
