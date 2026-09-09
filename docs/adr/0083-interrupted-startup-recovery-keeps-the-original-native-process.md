---
id: 0083
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0083: Interrupted startup recovery keeps the original native process

## Context

An interrupted foreground observer must not turn a viable native session into a
duplicate launch or duplicate contract.

## Decision

After positive pane evidence of a cleared operator-owned startup prompt commits the
entry, keep it under the foreground hitch without a post-gate deadline; after
interruption let `gang tick` retry, reserve drop-and-re-hitch for native-process
replacement, and resume only a stamped native session.

## Consequences

Once that positive evidence commits the entry, the hitch remains its foreground owner
without a post-gate deadline; Gangline starts no resident watcher. If the foreground
hitch is interrupted, a later `gang tick` becomes the retry owner once the prompt
clears. Drop and re-hitch is reserved for replacing the native process, and resume
applies only where a native session identity was stamped before interruption. A second
hand-sent contract is never the recovery.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
