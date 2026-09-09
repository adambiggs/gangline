---
id: 0163
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0163: A message answers only what its sender has read

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

Outbound mail correlates to a peer's request only when the sender's own native prompt
proof of that request stands.

## Consequences

A request with delivery proof alone is queued in the harness, not in the sender's
context, and a message that crosses it settled a debt the debtor had never seen; the
crossed request now stays audit until it is read and is owed then, and the crossing
message is a request of its own. The contract already asks a crossed message to be
acknowledged in the next reply, which is only possible if the crossing does not answer
it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
