---
id: 0201
status: proposed
date: 2026-09-16
supersedes: []
superseded-by: []
tags: [messaging, hitch]
---

# ADR-0201: A hitch assignment is answered by its completion report

## Context

Reply debt makes every verified request block idle until it is answered, and
the native Stop adapter says any acknowledgement will do. The message hitch
sends after the startup contract is the agent's whole assignment, so hitched
agents cleared its debt by acknowledging it before starting, even when the
brief told them not to.

## Decision

The message `gang hitch --stdin` sends is an assignment. Its envelope reads
`assignment`, and it owes nothing back: like a `--no-reply` send, a fresh
assignment leaves a waived record, and one that answers a message its
recipient sent stays a correlated reply. Both the startup contract and
`CONTRACT.md` say it is owed no acknowledgement because the completion report
is its reply.

## Consequences

This is compatible with ADR-0001 and ADR-0005. The marker is a word on the
wire, like `no-reply`, and Gangline keeps no record of which message was the
assignment beyond the reply record any `--no-reply` send leaves. No
later command reads it. A report sent in the turn that read the assignment
reaches its reader as a correlated reply that owes nothing back; one sent in
a later turn is a fresh request, as any later message is.

An agent that acknowledges and stops anyway looks idle with nothing owed.
Detecting that is rejected: it needs the assignment's identity kept on the
window and a check of the agent's behaviour against it, both of which
ADR-0005 forbids, so it waits on a record superseding ADR-0005.

Falsifier: a hitch assignment leaves reply debt, or its envelope lacks the
`assignment` marker.
