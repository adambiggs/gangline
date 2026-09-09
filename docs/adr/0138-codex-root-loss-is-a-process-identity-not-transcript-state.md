---
id: 0138
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0138: Codex root loss is a process identity, not transcript state

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

A Codex process can die inside a live pane without writing a terminal rollout record.

## Consequences

An unclosed native turn is also the ordinary shape of live work, so a transcript reader
would alert healthy agents. Tmux's pane root is the only universal process boundary that
does not confuse a sandboxed tool child for the harness. The Codex collar therefore
accepts only that root when Linux reports it as a non-zombie `codex` process, paired
with its kernel start stamp to defeat PID reuse. A missing or changed recorded witness
is harness loss; an unrecorded or unreadable witness remains visibly unobserved, not
healthy.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
