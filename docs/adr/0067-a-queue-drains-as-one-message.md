---
id: 0067
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0067: A queue drains as one message

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

When a turn boundary drains a spool, every waiting entry is delivered as one
chronological bundle under one pane lock, envelopes intact.

## Consequences

Delivering them one at a time submits the first, which starts a turn, which refuses the
second — so a target that is never idle for long accumulates exactly the messages that
would have corrected it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
