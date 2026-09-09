---
id: 0030
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0030: An interrupt is a collar keystroke and a fact Gangline owns

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

The key that stops a turn is harness knowledge, declared per collar; an undeclared
collar refuses.

## Consequences

Gangline drops the turn bracket rather than closing it. Leaving it open strands a busy
the harness will never end, but writing a closed one is worse: a fresh closed bracket
reads as definitive idle before any evidence from the pane is consulted, so a harness
that ignored the key would be declared reachable and the next send would enter mid-turn.
Gang saw a keystroke leave; it did not see a turn end, and removing the fact is the only
edit that says so. How an interrupt is recorded belongs with whatever redesigns the turn
state, not here. Occupancy refuses the command: that key is often what a native dialog
reads as an answer.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
