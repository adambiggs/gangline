---
id: 0034
status: accepted
date: 2026-08-13
supersedes: []
superseded-by: []
tags: [collars, usage]
---

# ADR-0034: Provider usage is a collar-native observation

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Keep provider-usage signaling optional and edge-triggered at operator-declared used
percentages.

## Consequences

Read only non-interactive native evidence: a headless harness query or the target
session's native rate-limit event. Record its observation clock and reset alongside the
percentage so staleness stays visible, and let a collar cap the age of evidence that may
drive a warning. A still-future absolute reset may arm a wake without spending quota
merely to refresh its percentage. Never feed decisions from the interactive usage page:
driving a composer cannot observe a busy agent and a pane rendering is not a stable data
contract. Collars without a correctness source say unavailable; status and roster report
the last ephemeral reading without turning observation into a poll.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
