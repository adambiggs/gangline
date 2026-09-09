---
id: 0146
status: accepted
date: 2026-09-01
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0146: Every option reader reports failure instead of clearing or omitting state

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Propagate every window-option read failure to the caller and preserve the unread state;
never clear it, omit it, or render it as absent.

## Consequences

The other two records the row reads were the same defect wearing a different face: their
readers collapsed the failure inside themselves, so no status reached a caller to keep.
Closing those meant changing what each reader returns rather than how one caller reads
it, and it is worth naming what that reached. A self-compaction failure is only current
while its request still stands, so an unreadable request was retiring a failure that was
still standing — a record cleared on the strength of a read that never happened. And a
caller reading such a function through a pipe keeps grep's status, not the reader's, so
an unreadable record left a tick pass silent and green.

Closing one of the three would have made that one look like a special case. The rule is
the record, not the caller: a read that failed is reported as a read that failed,
wherever it is made, and the note it would have carried is invented in neither
direction.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
