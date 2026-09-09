---
id: 0102
status: accepted
date: 2026-08-17
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0102: A launch that died is read as a death, not waited out

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Every reading in the boot wait asks what a pane is showing, and none of them asks
whether anything is still running to show it. So a harness that failed at launch spent
the whole boot budget and was then refused as an agent that is up but showing something
other than its input box — a live-agent recovery offered for a process that is not
running — or, where the window went with it, as a raw tmux error naming nothing Gangline
had tried to run.

## Consequences

The wait ends on the death instead, and the refusal carries the launch command, plus the
exit status and the pane's last line wherever a corpse was held. The same reading
answers every window registration a hitch performs, because a launch can die under any
of them.

Absence is established from the answer rather than from a command status: tmux expands a
target it cannot resolve to nothing and still exits 0, so the reading asks for the
window's own id alongside the fact it wants, and a reading that is neither of those
refuses rather than passing for a healthy launch.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
