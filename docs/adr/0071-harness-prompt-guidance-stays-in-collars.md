---
id: 0071
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0071: Harness prompt guidance stays in collars

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Let a collar contribute harness-specific prose to the single system-prompt addition when
a live native feature creates a trap agents cannot infer.

## Consequences

Refuse that prose when the collar declares no system-prompt option; do not pass the
native option twice and guess how repeated values compose. Claude Code uses this surface
to state that its task list is session-scoped and unreadable from other Gangline
windows.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
