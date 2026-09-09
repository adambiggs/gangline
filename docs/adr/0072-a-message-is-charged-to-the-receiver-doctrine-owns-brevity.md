---
id: 0072
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0072: A message is charged to the receiver; doctrine owns brevity

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

Keep message bodies unrestricted in the substrate; doctrine requires senders to route
concise text because recipients retain and repay its context cost.

## Consequences

Message text stays in the recipient's context across later turns, so the sender writes
it once and the recipient pays repeatedly. Observed directly: two agents on single lanes
reached 500k and 616k tokens, the larger share inbound brief rather than work.

The cost remains a team operating rule, not a substrate message schema. Doctrine owns
brevity and peer routing; the contract keeps the shared-state reachability rule.
Gangline does not inspect or ration message bodies.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
