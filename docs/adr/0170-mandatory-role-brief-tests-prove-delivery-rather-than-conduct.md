---
id: 0170
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [testing, roles]
---

# ADR-0170: Mandatory role-brief tests prove delivery rather than conduct

## Context

Mandatory evidence must distinguish the behavior under test from timing and fixture
artifacts.

## Decision

Every mandatory assertion about `roles/lead.md` proves the brief is delivered: its bytes
validate, reach a system prompt intact, and are still in the shipped file.

## Consequences

None reads what a lead did after receiving one, so none separates a line that changes
behaviour from a line that only reads as though it would.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
