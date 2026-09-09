---
id: 0165
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0165: Every witness of a correlated reply settles it

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

The creditor's native prompt proof of a correlated reply writes the same settlement on
the debtor's request that spool acceptance and delivery verification write.

## Consequences

A sending process killed between typing and verification, at a tick deadline or after
losing the screen, left the reply record without delivery proof and no later writer, so
the debtor was refused idle at every Stop for a reply it had given and sent it again.
The settlement is one immutable digest per record, so late and repeated witnesses
rewrite the same bytes rather than conflicting.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
