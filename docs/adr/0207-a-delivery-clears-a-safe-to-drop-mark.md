---
id: 0207
status: proposed
date: 2026-09-18
supersedes: []
superseded-by: []
tags: [lifecycle, messaging]
---

# ADR-0207: A delivery clears a safe-to-drop mark

## Context

This proposal is aimed at accepted ADR-0198, under which a registration marked
safe to drop accepts no further actionable delivery. That refusal strands every
message addressed to a marked agent, replies it is owed included, and forces a
replacement hitch for work the marked agent could simply take. The mark exists
to authorize teardown, not to close the agent's input.

## Decision

A delivery to a marked registration clears the mark under the pane lock that
delivery already holds, records the clearing as an `agent.unmarked` event, and
proceeds as it would to any live agent. The agent marks itself again when it is
done. A mark that cannot be read refuses the delivery, since it cannot be
cleared. `drop` is unchanged.

## Consequences

A drop already in flight can still tear down an agent that was just delivered
to. The decision is falsified by a delivery that reaches a marked registration
and leaves the mark standing, or by `status` or `roster` naming the mark after
a delivery cleared it.
