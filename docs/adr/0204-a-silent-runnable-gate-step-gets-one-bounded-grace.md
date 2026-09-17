---
id: 0204
status: proposed
date: 2026-09-17
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0204: A silent runnable gate step gets one bounded grace

## Context

Accepted ADR-0167 bounds every silent gate phase to one quiet budget and renews
that budget only on completed output. Under CPU contention, lint can remain
runnable without completing a line, so the first expiry can confuse starvation
with a blocked step. Unlimited CPU renewal would instead make a silent busy loop
unbounded. The live tree follows the bounded rule while this proposal awaits
acceptance.

## Decision

If accepted, this record supersedes only ADR-0167's one-budget silent-phase
bound. At a silent phase's first quiet expiry, a process group with a runnable
member gets one final grace equal to the quiet budget. A completed output line
starts a new phase. A blocked group gets no grace, and the second expiry always
stalls regardless of CPU activity.

## Consequences

Blocked silence remains capped at one budget and runnable silence at two. Busy
loops cannot renew indefinitely. Procfs supplies the optional runnable witness;
an unreadable witness grants nothing. The existing ownership, diagnostics, and
status rules remain unchanged. This proposal is falsified if one silent phase
receives two CPU graces or runnable lint stalls at its first expiry.
