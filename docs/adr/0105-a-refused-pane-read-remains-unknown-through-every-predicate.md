---
id: 0105
status: accepted
date: 2026-08-18
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0105: A refused pane read remains unknown through every predicate

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

A pane reading is transport, and the transport can refuse while the agent it describes
is alive and healthy. Every predicate that consumes such a reading spends it as evidence
— no box drawn means nothing owns the screen, a box identical twice means nobody is
typing, a witness equal to the one before it means nothing moved — so a refusal folded
into any of those becomes a positive finding about a pane nobody looked at, and a caller
then acts on it.

## Consequences

So a refused read carries its own status the whole way. Collars answer `3`, distinct
from the `1` that means the harness drew no composer and from the `2` that means a
composer outgrew its pane. `input_read` is the single place that classification is made
in `bin/gang`. Predicates that cannot express unknown refuse loudly instead: occupancy,
busy/idle and decay all name the reading they could not take.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
