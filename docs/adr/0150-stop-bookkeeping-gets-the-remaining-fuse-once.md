---
id: 0150
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0150: Stop bookkeeping gets the remaining fuse once

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Give ordinary Stop bookkeeping the native fuse's remaining time exactly once, and refuse
the boundary when that single attempt cannot prove completion.

## Consequences

The ordinary Stop bookkeeping then takes everything the fuse has left, once. A fixed
three-second cap on it was under twice the idle cost of one Gangline call, so ordinary
CPU contention timed it out, and the refusal named a broken hook path that answered well
inside the cap the moment load fell. A refusal that names the wrong cause is worse than
a slow boundary: it spends a model turn and one of the native cap's blocks sending the
agent to rewrite wiring that works. The refusal therefore says the boundary ran out of
time and asks for the wait that can clear it.

That boundary is never retried and never begun below its floor. `gang hook` closes the
turn, records the boundary's facts, dispatches delivery or self-compaction, and closes
reply threads last; a killed attempt leaves some prefix of that done, and a second
attempt would repeat the prefix beside a child the kill did not reach. One attempt or
none is the only shape that keeps a mutating boundary honest under a deadline.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
