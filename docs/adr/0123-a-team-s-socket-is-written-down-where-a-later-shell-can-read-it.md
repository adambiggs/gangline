---
id: 0123
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0123: A team's socket is written down where a later shell can read it

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

tmux clients discover the default socket, so a team on a private `TMUX_TMPDIR` is
unreachable from a shell that lost that environment and looks exactly like a team that
ended.

## Consequences

`hitch` records the socket under the lock root; `teams` reads the records back and asks
each server rather than believing the file; `attach` crosses to a recorded socket only
when this shell's own does not have the team. The record is a fact about reachability
and never authority.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
