---
id: 0113
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0113: A spool nobody claims is swept where a team starts, not watched

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

A spool directory is resolved through `@gl_spool` on a live window, so one left by a
window that did not die through `drop` or `down` is unreachable by every command
Gangline has. The answer is not a component that watches for them — that is the loop law
7 forbids — but the one moment the question is both cheap and safe to ask: the hitch
that opens a session, where the server has just proved it answers and the only spool
Gangline owns is the one it just minted.

## Consequences

The sweep archives; it never deletes a body. A directory it cannot archive, and one
Gangline did not mint, are named and left exactly where they are. A window list that
cannot be read is reported rather than answered as "nobody holds anything", which would
archive every live agent's mail.

This makes one server per `GANG_LOCK_DIR` explicit. It was already implicit:
`spool_mint` draws an identity no live window holds, and reads that list from one
server.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
