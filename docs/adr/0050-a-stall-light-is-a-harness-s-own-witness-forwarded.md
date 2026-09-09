---
id: 0050
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0050: A stall light is a harness's own witness, forwarded

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Where a harness itself reports that it is waiting on a person, deliver that fact as an
ordinary attributed message to one optional operator-declared target.

## Consequences

Nothing polls, nothing infers a stall from a quiet pane, and nothing infers a lead: the
target is a declaration in the shape of the team curfew, and with none declared there
are no stall lights. A repeated report of the same kind inside one stall is one note,
cleared by the harness's own next move. A harness that reports nothing gets no
substitute, and a delivery that fails is recorded on the window for status to surface
rather than killing the hook — a record retired only by a later note accepted live or
parked, because a light that is still broken has to keep saying so.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
