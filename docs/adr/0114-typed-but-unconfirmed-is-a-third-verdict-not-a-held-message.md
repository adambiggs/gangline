---
id: 0114
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0114: Typed-but-unconfirmed is a third verdict, not a held message

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

`refuse` means nothing was typed, so the body is still the sender's and can be parked.
`die` means Gangline could not do what it was asked. A send whose Enter was pressed and
whose screen then stopped answering is neither: parked it becomes a second copy of a
message that may already have arrived, and reported as a failure it invites the sender
to make that copy by hand.

## Consequences

It exits 5, says `delivered but UNVERIFIED`, and its spool record is named `unverified-`
rather than `failed-` so every later reader can tell "Gangline watched and saw nothing
enter" from "Gangline watched the keys go in and lost the screen". Neither is ever sent
again.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
