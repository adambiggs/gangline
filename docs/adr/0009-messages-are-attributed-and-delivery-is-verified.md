---
id: 0009
status: accepted
date: 2026-08-04
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0009: Messages are attributed and delivery is verified

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

Require a sender, wrap each message in a nonce-bound envelope, and confirm that the
target composer accepted and submitted it.

## Consequences

Gangline is single-tenant and does not claim authentication, but it never reports
unverified delivery.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
