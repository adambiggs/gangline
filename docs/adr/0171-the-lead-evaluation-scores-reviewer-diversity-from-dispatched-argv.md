---
id: 0171
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0171: The lead evaluation scores reviewer diversity from dispatched argv

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

`test/leadeval.sh` closes that gap for one line — that every owner has an available
reviewer whose error modes differ from its own.

## Consequences

It stages a lead in a disposable session, gives it independent results to dispatch, and
scores the collar and model in the `gang` invocations it actually made. The verdict is
computed from argv and never from prose, so a lead that dispatches a monoculture while
explaining diversity fails. That line is scored because it is the only decision in the
brief visible in argv alone; the rest need a manifest the lane would supply, or a model
judging prose, and a model judge is not a test.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
