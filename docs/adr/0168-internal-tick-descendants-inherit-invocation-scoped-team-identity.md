---
id: 0168
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [lifecycle, ticks]
---

# ADR-0168: Internal tick descendants inherit invocation-scoped team identity

## Context

Agent lifecycle actions span tmux and host processes, so partial success can leave live
or durable state.

## Decision

A public `gang` invocation records Bash's own effective user id and, once its configured
team is live, derives the tick key from that team's socket and session. It exports those
values only as internal carriers for the deadline worker and any successor it launches.
Internal descendants accept only a numeric uid and a 24-character lowercase hexadecimal
key; every public boundary discards a supplied key before it can create or address a
team.

## Consequences

A successful derivation keeps the worker tree on one team identity without repeating
invariant lookups in every guarded tmux client or command substitution. A failed
derivation remains retryable at later call sites. The lock ownership check still uses
the process's real effective uid, and its refusal reports that real value rather than
the cached carrier.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
