---
id: 0012
status: accepted
date: 2026-09-03
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0012: A vanished stable sender retires its reply obligation

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

Retire a reply obligation, complete or partial, when a complete stable-identity
inventory proves its sender token no longer exists.

## Consequences

Resolve that token for every prompt-witnessed request record, whether or not its
delivery proof landed: the missing proof asks the same question about who could still be
answered, and a record whose sender is gone names an action no debtor can take. Write a
separate monotonic retirement proof beside the immutable message evidence, then report
the record as retired without resolving that dead token again; retained history
therefore preserves the reply query's linear pass. The audit verdict and status name
sender retirement. This is lifecycle settlement, not fabricated reply proof: a fresh
agent with the same name does not inherit the old token, and an unreadable identity
remains unknown and blocks Stop.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
