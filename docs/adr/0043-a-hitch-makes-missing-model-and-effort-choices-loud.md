---
id: 0043
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [lifecycle, models]
---

# ADR-0043: A hitch makes missing model and effort choices loud

## Context

Agent lifecycle actions span tmux and host processes, so partial success can leave live
or durable state.

## Decision

Warn at hitch for each omitted choice that the collar exposes, before a harness silently
supplies its default.

## Consequences

Gangline requires the choice, never its content: model names, effort levels, and policy
remain the collar's and operator's words.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
