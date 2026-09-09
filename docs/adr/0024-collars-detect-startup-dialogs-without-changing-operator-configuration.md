---
id: 0024
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [collars, configuration]
---

# ADR-0024: Collars detect startup dialogs without changing operator configuration

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Suppressing the dialog at launch is not available and is not to be reattempted.

## Consequences

On claude-code 2.1.241 no flag, environment variable or settings key turns the
onboarding prompts off, and the gating state lives in the operator's global
`~/.claude.json` beside their permission mode. A collar does not write operator
configuration, so the remedy is detection rather than prevention.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
