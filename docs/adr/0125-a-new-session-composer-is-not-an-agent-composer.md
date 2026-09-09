---
id: 0125
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0125: A new-session composer is not an agent composer

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

claude-code's background-sessions view draws a framed composer, but text typed there
creates a new session rather than reaching the hitched conversation.

## Consequences

Generic pane and tmux state cannot distinguish two native composers, so the claude-code
collar owns the distinction. The paired view notice and new-session placeholder observed
on 2.1.251 produce status 6 before clipped-box handling; core delivery carries that
status through as a named refusal.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
