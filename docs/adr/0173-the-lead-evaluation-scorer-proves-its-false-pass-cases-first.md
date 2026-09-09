---
id: 0173
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0173: The lead-evaluation scorer proves its false-pass cases first

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The scorer answers for itself before any budget is spent.

## Consequences

`test/leadeval/score-selftest.sh` is deterministic, needs no harness or clock, and the
lane runs it first. It pins the false-pass shapes in particular: a hitch that launches
nothing still parses as a hitch and carries defaults, so counting one adds a phantom
agent differing from every real one, which supplies the missing reviewer and turns a
monoculture into a PASS. A scorer that fails toward PASS retires the question it exists
to ask.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
