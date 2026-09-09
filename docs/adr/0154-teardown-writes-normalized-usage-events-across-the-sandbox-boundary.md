---
id: 0154
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [usage, lifecycle]
---

# ADR-0154: Teardown writes normalized usage events across the sandbox boundary

## Context

Usage evidence crosses process and sandbox boundaries while accounting policy remains
operator-owned.

## Decision

`drop` and `down` prepare one normalized line per agent for
`${XDG_DATA_HOME:-~/.local/share}/gangline/usage/events.jsonl`, carrying the launch
record, the team's identity (its name and the epoch its session was created, since a
name is reused), the end time, and ccusage's reading at that moment, read once for every
window a `down` ends, with a status that says whether it was matched, unmatched,
unstamped, absent, failed, or malformed.

## Consequences

Preparation stays in the caller, preserving its PATH and transcript roots. A sandboxed
caller may be able to read those transcripts while seeing the operator's usage data
directory as read-only, so the normalized JSON crosses the sandbox boundary in a named
tmux buffer and a synchronous `run-shell` child appends it from the server's host mount
namespace. This preserves the record without weakening a collar's sandbox or adding a
daemon. The ccusage call stays bounded, and the buffer is deleted after a successful
append.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
