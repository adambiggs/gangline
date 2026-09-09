---
id: 0095
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0095: A held response is the only honest way to freeze a turn

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Scenarios that need a live turn — mid-turn steering, a wait that must block — cannot get
one from a sleep, because the thing being measured is whether Gangline observes a turn
that is genuinely in flight.

## Consequences

The stub holds its response open on a FIFO pair instead. Opening a FIFO for writing
blocks until a reader arrives and opening one for reading blocks until a writer does, so
the lane learns the turn is live from the turn itself and ends it when the assertions
are done. Neither side polls, and the turn is live for exactly the window under test.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
