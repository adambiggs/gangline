---
id: 0097
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0097: An API key in the environment is a first-run gate

## Context

A mandatory gate must remain reproducible, bounded, and honest about evidence it cannot
obtain.

## Decision

A cold `CLAUDE_CONFIG_DIR` draws onboarding rather than a composer, which the
claude-code collar already enumerates.

## Consequences

A seeded one still stops: given `ANTHROPIC_API_KEY`, the harness draws a two-choice
approval box defaulting to No, hitch correctly reports a native first-run prompt, and an
unattended lane waits for an operator who is never coming. The harness identifies a key
by its last twenty characters, so the lane records the answer the same way alongside the
onboarding and trust flags.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
