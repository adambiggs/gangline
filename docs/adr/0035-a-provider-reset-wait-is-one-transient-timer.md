---
id: 0035
status: accepted
date: 2026-08-13
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0035: A provider-reset wait is one transient timer

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Resume an idle agent at a native reset with one systemd user timer that invokes the
ordinary attributed delivery path and is collected after it fires.

## Consequences

Store only the pending declaration in the agent's tmux window, expose it in status and
roster, and provide exact cancellation. Do not run a Gangline watcher, daemon, or retry
loop; a removed or superseded target makes a stale one-shot firing a no-op.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
