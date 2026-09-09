---
id: 0066
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [messaging, lifecycle]
---

# ADR-0066: Teardown archives mail before deleting its spool

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

A composed message is not teardown state. `gang drop` and `gang down` move every waiting
or held entry into a human-readable archive before deleting its window's spool, and
refuse to end anything if that archive cannot be written. The archive is created only
when mail exists and its printed path is the handoff to a person.

## Consequences

An addressee's own `gang mail` read still consumes what it prints, so the next turn
boundary cannot deliver the same message twice. It moves each claimed entry into the
same archive surface before writing that entry to stdout, and prints the archive and
deletion paths on stderr. A shell filter may hide rendered output; it cannot erase the
only copy or the route back to it.

Read archives are durable recovery state, not a cache: Gangline never guesses when their
human purpose is over. The read prints the exact deletion command, and operations
guidance makes the operator responsible for running it after recovery or audit ends.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
