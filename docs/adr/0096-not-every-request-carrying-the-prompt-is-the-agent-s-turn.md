---
id: 0096
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0096: Not every request carrying the prompt is the agent's turn

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Identify the agent's turn from the standing contract supplied through the collar's
system-prompt option, not from prompt text that auxiliary requests may quote.

## Consequences

Observed on claude-code 2.1.233: each submitted prompt also triggers a small auxiliary
session-title completion whose body quotes the user's message, and it arrives BEFORE the
real turn. A stub keying on prompt text alone holds that one, releases the lane, and
lets the turn it meant to freeze run unheld — with every assertion still passing,
because a request log records arrival rather than completion. The lane therefore
identifies the agent's own turn by the standing contract the collar passes through
`--append-system-prompt`, read from `CONTRACT.md` at run time so a reworded contract
breaks loudly instead of quietly reclassifying every request as a side errand.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
