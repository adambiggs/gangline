---
id: 0015
status: accepted
date: 2026-09-01
---

# ADR-0015: Every option reader reports failure instead of clearing or omitting state

## Context

Window-option readers collapsed a failed read into an empty value. An unreadable
option was reported as unset (b6b2950), an unreadable self-compaction request
retired a failure that was still standing, and a caller reading through a pipe
kept grep's status, so an unreadable record left a tick pass silent and green.

## Decision

Every window-option read failure propagates to its caller and the unread state
is preserved. No reader clears it, omits it, or renders it as absent.

## Consequences

Status and roster show an unreadable value as unknown rather than inventing one.
A reader that grows a failure status means sweeping its call sites for pipes and
command substitutions, since neither trips `set -e`.
