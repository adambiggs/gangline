---
id: 0015
status: accepted
date: 2026-08-10
supersedes: []
superseded-by: []
tags: [compaction]
---

# ADR-0015: A turn event settles an open compaction bracket

## Context

Compaction crosses a native turn boundary where duplicated or lost input would corrupt
the agent's work.

## Decision

PreCompact is not reliably paired: a harness that REFUSES the compaction raises it and
never raises PostCompact. An unpaired opening held the agent busy until the bracket aged
out, and reported busy over an idle harness for that whole window.

## Consequences

A turn event closes it, because taking a turn and compacting are mutually exclusive.
Input typed into a compaction is parked, and the park raises nothing until it drains, so
a turn witnesses a harness that is not compacting whether or not the compaction it
followed ever finished. Settling only ever closes a bracket that is already open; a turn
on a window that never had one writes nothing.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
