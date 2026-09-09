---
id: 0148
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0148: Claude's native Stop-block cap remains enabled

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Claude Code overrides a Stop hook after a finite run of consecutive blocks and ends the
turn without telling it. Gangline once lifted that cap on every Claude launch and
refused a hitch whose shell had not lifted it too, so that a peer reply obligation could
hold the harness at Stop for as long as it stood. Every fault in the reply subsystem — a
proof that never landed, a sender that vanished, a query lost to lock contention — then
became an unbounded run of model turns, and an agent stranded on a record no message
could clear had no exit short of being dropped.

## Consequences

The cap stays in force, and for peer-reply provenance the adapter never depends on
reaching it. It refuses idle once per turn, releases the re-Stop the harness marks with
`stop_hook_active`, and makes the release loud rather than forgiving: the records stay
exactly as they were, `status` and `roster` keep reporting them, the notify target or
the lead is told, and the next delivery's first Stop refuses again. A query that cannot
answer inside the adapter's deadline is its own named state under the same rule. A Stop
boundary that fails or runs out of time is the exception carved out below: it leaves no
record that survives a release, so it refuses on the re-Stop too and may reach the cap.
Provenance still fails closed for a boundary; no provenance state fails forever.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
