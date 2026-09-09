---
id: 0108
status: accepted
date: 2026-08-18
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0108: Pane reads are checked before their output is parsed

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

Two shapes hide such a refusal by construction and are banned wherever a reading is
assembled.

## Consequences

A capture piped straight into a parser arrives as the parser's verdict on empty input,
which is indistinguishable from its verdict on a pane with nothing to find. And a
reading assembled inside the arguments of a `printf` — or handed to another substitution
as an argument — leaves the outer command succeeding on a substitution that failed, so
`die` inside it exits a subshell nobody is watching. Read into a variable, check the
status, then use it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
