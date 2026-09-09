---
id: 0121
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0121: A dead response stream is told apart by its missing status, not its sentence

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Claude Code ends a turn killed mid-stream with a synthetic assistant record carrying
`error=server_error` and no `apiErrorStatus` key.

## Consequences

Specimens of it say the response stopped arriving, that the server errored mid-response,
and that the connection was lost; the structure is the same in all three. Matching the
absent key rather than the prose keeps the reader on the harness's data and off its
wording, and leaves every status-bearing `server_error` nonfatal.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
