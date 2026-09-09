---
id: 0032
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0032: Binary identity is a window witness

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

Stamp hitch and adopt windows with the checksum and size of the invoked script,
including in checkouts: executable bytes, not repository state, determine live skew.

## Consequences

Compute and compare that witness only when stamping, status, or roster needs it;
unavailability is visible but never blocks lifecycle commands. Skew does not justify a
patrol or an attempt to retrofit launch-time context, collars, or hooks. When that
script lives in a git checkout and differs from HEAD, warn on every operator command
dispatch with the exact path and HEAD; the native hook endpoint stays silent except for
a crossed light. Keep live-by-path installation, because a merge can intentionally
upgrade a running team.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
