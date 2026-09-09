---
id: 0017
status: accepted
date: 2026-08-04
supersedes: []
superseded-by: []
tags: [compaction]
---

# ADR-0017: Native self-compaction stays beside verified tty delivery

## Context

Compaction crosses a native turn boundary where duplicated or lost input would corrupt
the agent's work.

## Decision

Agents request their harness's native compaction at natural checkpoints.

## Consequences

Keep this beside the verified tty substrate it needs rather than creating a second
product or a duplicate injection path; defer the command to Stop when a harness cannot
submit it during its own turn.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
