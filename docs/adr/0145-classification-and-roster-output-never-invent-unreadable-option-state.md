---
id: 0145
status: accepted
date: 2026-09-01
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0145: Classification and roster output never invent unreadable option state

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The leftover branch of the box classification is that rule seen from the other side. It
names a human as the author, so it may only be reached from reads that answered; let an
unreadable record arrive there as an absence and the line invents exactly the provenance
the classification exists to refuse to invent.

## Consequences

The roster row is the same rule at summary length. It cannot spend a line on what it
could not read, so it names the record and stops there — `staged- unreadable`, beside
the `usage-unreadable` it already prints — and commits the note in neither direction,
neither as waiting nor as absent.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
