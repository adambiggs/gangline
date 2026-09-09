---
id: 0002
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [lifecycle, roles]
---

# ADR-0002: Role briefs are validated prose delivered at hitch

## Context

Agent lifecycle actions span tmux and host processes, so partial success can leave live
or durable state.

## Decision

Role briefs are that prose, shipped.

## Consequences

`roles/<name>.md` is text attached to the one hitch that asked for it, replaceable
file-for-file by an operator file of the same name, on the same footing as the harness
knowledge a collar carries. Gangline validates a brief as prose and delivers it — at
system-prompt level where a collar declares the option, at message level otherwise — and
never parses it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
