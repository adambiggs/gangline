---
id: 0021
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [collars, compaction]
---

# ADR-0021: A collar without an idle witness refuses self-compaction

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

A collar that declares the witness `unavailable` keeps the earlier rule: a self-request
is refused as unsupported rather than deferred, no request is recorded, no dispatcher
starts, no recovery continuation is created, and the refusal names the peer form that
does work, since a peer types into a composer it can see is idle.

## Consequences

The refusal stays on the window for `status` and `roster` until a peer compaction lands
from a genuinely idle boundary. A request recorded before this rule is retired, with a
note to the agent, by the next boundary or by the next refused self-call, whichever
comes first. Preserving such a request as "pending" read as a compaction that would
still happen, and an agent that ended its turn on that reading waited on a boundary that
could only refuse it while the roster showed it scheduled. A peer compaction that lands
clears whatever self-request the same window holds, whatever its collar: leaving it
would compact the fresh context a second time at the next boundary. The retirement and
the peer's clearing take one per-window claim, since tmux has no compare-and-set and a
retirement that read its request before the peer cleared it would otherwise write the
refusal back over a stall that had just ended; a retirement that cannot take the claim
yields to the holder, and a peer that cannot is refused before it types.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
