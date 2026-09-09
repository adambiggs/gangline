---
id: 0078
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [compaction]
---

# ADR-0078: Native continuation owns compaction recovery

## Context

Compaction crosses a native turn boundary where duplicated or lost input would corrupt
the agent's work.

## Decision

Native continuation now returns every supported compact command to a turn that re-reads
the brief and saved state.

## Consequences

Repository checkpoint safety remains in `AGENTS.md` and operations rather than the
standing team contract.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
