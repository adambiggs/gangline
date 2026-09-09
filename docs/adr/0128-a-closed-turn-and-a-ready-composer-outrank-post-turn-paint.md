---
id: 0128
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0128: A closed turn and a ready composer outrank post-turn paint

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Harnesses keep drawing recap, update, and spinner chrome after their native turn-close
event.

## Consequences

Pane activity cannot reopen that event. Once the current composer is readable and empty,
it agrees with the closed bracket and the state is idle; a draft, absent box, or
unreadable box preserves unknown. This spends positive current input evidence, not a
quiet-time guess.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
