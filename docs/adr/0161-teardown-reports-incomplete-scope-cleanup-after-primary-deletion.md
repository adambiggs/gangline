---
id: 0161
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [lifecycle]
---

# ADR-0161: Teardown reports incomplete scope cleanup after primary deletion

## Context

Agent lifecycle actions span tmux and host processes, so partial success can leave live
or durable state.

## Decision

Drop and down keep their historical success across pre-nonce windows whose recorded
scope is already gone.

## Consequences

After the window or session is deleted, they return nonzero only when a recorded scope
is still active or its state cannot be read and Gangline lacks the issuance proof or
membership evidence required to stop it. That exit says the primary teardown happened
but promised cleanup did not; it never grants authority to stop on a name alone. This
keeps cleanup on commands already responsible for deletion and avoids a resident scope
watcher.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
