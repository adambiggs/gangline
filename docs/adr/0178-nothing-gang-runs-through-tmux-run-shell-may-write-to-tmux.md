---
id: 0178
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0178: Nothing gang runs through tmux run-shell may write to tmux

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

A run-shell child's output does not return to the caller. tmux prints every line of it,
plus a line of its own for a nonzero exit, into a pane it selects — the caller's pane
where the invocation has one, and otherwise whichever pane `cmd_find_from_nothing` lands
on, which belongs to some other agent — and the pane is forced into view-mode to hold
it. A pane in view-mode does not take a paste at its shell, so the next message
delivered to that agent never arrives and nothing reports it.

## Consequences

So a command handed to run-shell ends with its own output and status guard, and what the
caller needs to read comes back in a named tmux buffer instead. The teardown usage
append is on this path and prints on success as well as on failure, so every `gang drop`
and `gang down` reached it.

That buffer is a reply, and a reply has to be answerable. One 128-bit token per call
names both the staged event and its report and is the report's first word, so a report
without this call's token is another teardown's, or the remains of one, and is refused
rather than read — and no two teardowns stage an event under the same name. The caller
deletes the report as soon as it has read it; a caller interrupted before that leaves
the buffer in the server until the server exits, which is what the diagnostics say when
a delete fails.

The report also says where the event ended up — in the record, in a recovery file, or
nowhere yet — because the caller, not the run-shell child, owns the staging buffer: it
discards that copy once a report accounts for the event and keeps it when nothing else
holds one. A report that never arrives therefore proves nothing about the append, and
the caller says so instead of guessing.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
