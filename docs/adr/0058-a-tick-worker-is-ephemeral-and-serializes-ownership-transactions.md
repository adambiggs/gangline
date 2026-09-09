---
id: 0058
status: accepted
date: 2026-08-31
supersedes: []
superseded-by: []
tags: [ticks]
---

# ADR-0058: A tick worker is ephemeral and serializes ownership transactions

## Context

Cooperative retries must make progress without creating a resident coordinator or
unbounded owner.

## Decision

The worker is ephemeral, not resident: no process outlives the pass it was born to
finish.

## Consequences

A per-team kernel flock serializes lock-metadata transactions; the generation symlink
remains the worker ownership record between them. The guard descriptor is opened only
for a transaction and closed before the cooperative pass, so a subprocess cannot inherit
exclusion beyond the worker's lifetime. Every read-decide-unlink sequence and final
owner release holds that guard. The empty guard inode remains under `GANG_LOCK_DIR`
until that operator-owned lock root is removed. A contender marks the owner dirty and
exits, and the owner consumes that edge with one more pass before release. Dead and
replaced generations are reclaimed. A live owner beyond the

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
