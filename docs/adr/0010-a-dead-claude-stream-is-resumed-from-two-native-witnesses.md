---
id: 0010
status: accepted
date: 2026-08-15
---

# ADR-0010: A dead Claude stream is resumed from two native witnesses

## Context

Claude Code emits no Stop when a provider stream dies. The turn just ends, and
an agent waiting on it looks idle. The error prose varies between releases.

## Decision

Gangline requires two native witnesses: the later `idle_prompt` notification,
which proves the harness is waiting and binds the transcript path, and the
newest top-level assistant record carrying `error`, `isApiErrorMessage` and a
UUID. Under `GANG_AUTO_RESUME` it closes the turn and submits one attributed
continuation per error UUID; a failure of that continuation gets no second
hop.

## Consequences

The dead turn is told from ordinary idleness by record structure, not by its
sentence. When ownership of the turn cannot be proved Gangline refuses and
records the refusal for status and roster. Collars without such a record
declare no equivalent.
