---
id: 0056
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [ticks]
---

# ADR-0056: Operational commands supply a cooperative tick

## Context

Cooperative retries must make progress without creating a resident coordinator or
unbounded owner.

## Decision

`gang tick` runs one synchronous pass over every hitched window.

## Consequences

Other operational commands launch the pass detached after preserving their own result;
alert inspection and the tick's internal workers do not recursively launch one. The pass
retries all waiting spool entries through the existing verified-delivery gates, retries
safe deferred self-compaction, and verifies a collar's live native-session identity
where one can be read. Native hooks still own their event facts and event-specific work;
a tick may prefer a closed native turn witness over contradictory stale pane paint, but
it grants no new permission to interrupt or type.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
