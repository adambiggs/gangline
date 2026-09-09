---
id: 0167
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0167: The mandatory gate treats output as a renewable lease

## Context

A stalled gate must fail without imposing a total wall-clock limit.

## Decision

Run the snapshot copy, lint, smoke, and integration in separate process groups, and
refuse a step that completes no output line inside the configured quiet budget. Lint
retains the overlap that keeps it off the integration-bound critical path; smoke and
integration run in order on the other branch, and an ordinary failure in either branch
does not omit the remaining evidence. This overlap avoids adding lint's full runtime; it
does not reinstate the withdrawn five-minute local wall-clock rule. On a stall, print
that group's process tree and trailing output before ending exactly that group and
cancelling its concurrent sibling. The gate shell closes both lock descriptions at the
process-group boundary, so its exit releases the shared heavy-test lock and no
descendant can inherit either lock.

## Consequences

The default lease is 300 seconds, over twice the 104-second trailing quiet gap measured
in a healthy lint run and far above the 6.430-second maximum adjacent-line gap in a
successful 759-second integration run. It bounds one silent phase to five minutes
without imposing a total-duration limit on a healthy run whose completed lines keep
renewing the lease. `GANG_GATE_QUIET_SECONDS` carries the operator's slower-host choice.
A waiter first takes a nonblocking reading and reports the PID, working directory, and
age the owner wrote into the locked inode before it joins the queue. Gate and end-to-end
owners clear that record before unlocking. A second permanent kernel lock corroborates a
stable read; an acquisition gap, a legacy holder, or bytes left by a dead predecessor
are reported as unknown instead of attributed. An unknown report includes a best-effort
`fuser` command for rollout periods where legacy owners cannot provide a corroborated
record. A stalled step's 124 or ownership-refusal 125 takes precedence over the
concurrent branch's internal cancellation; 123 is reserved for a branch that vanishes
without either cause. The 124 or 125 is published atomically in the stall marker before
diagnostics; the main shell reads that marker and never cancels a branch that owns one.
Thus a sibling exit during reporting cannot erase the stall or truncate its evidence.

Revisit if a supported-host gate stays bounded and honest without renewable output
leases.
