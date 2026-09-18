---
id: 0206
status: proposed
date: 2026-09-18
supersedes: []
superseded-by: []
tags: [messaging, roles]
---

# ADR-0206: Declared assignments carry one review tier before launch

## Context

Review depth is the assigner's choice before work starts. Prose alone let an
assignment launch without it, leaving the owner to infer the tier afterwards.
Gangline already distinguishes its stdin and task-only assignment forms from
ordinary messages whose intent it cannot know.

## Decision

Before its first tmux mutation, `hitch --stdin` refuses an assignment unless its
body contains exactly one own-line `tier: A` or `tier: B`. A task-only assignment
requires `--tier A|B`, which renders the same line into the startup assignment.
Tier B acceptance states that its review remains inside the owner's harness and
a teammate reviewer violates the tier. Gangline validates the declaration but
does not retain it or classify ordinary message prose.

## Consequences

This remains compatible with ADR-0001 and ADR-0005: one declared delivery
boundary is checked, but no work is allocated, no tier becomes coordination
state, and no agent behavior is inspected. Existing senders must declare the
tier before launch.

Falsifier: a declared assignment without exactly one supported tier launches,
or a later command reads retained tier state.
