---
id: 0057
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [ticks]
---

# ADR-0057: Cooperative ticks replace dependence on recipient hook frequency

## Context

Accepted work must remain retryable when a recipient emits no later native hook.

## Decision

Use `gang tick` as the direct retry edge for accepted work; do not depend on the
recipient producing another hook.

## Consequences

This supersedes hook-frequency selection. Copy-mode, an idle pane that raises no later
boundary, a hook disabled at launch, and a falsely occupied screen can no longer leave
accepted work dependent on that same recipient producing another event. A later `gang
tick` supplies the retry directly.

Here “supersedes” refers to the pre-record hook-frequency behavior; no earlier ADR
occupies the empty relationship list.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
