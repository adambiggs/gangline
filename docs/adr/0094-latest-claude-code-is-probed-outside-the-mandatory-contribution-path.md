---
id: 0094
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [testing]
---

# ADR-0094: Latest claude-code is probed outside the mandatory contribution path

## Context

Mandatory evidence must distinguish the behavior under test from timing and fixture
artifacts.

## Decision

The offline e2e lane runs daily and on dispatch in its own workflow, which installs the
npm `latest` claude-code release.

## Consequences

Harness and collar drift is then probed within a day without adding real-TUI wall time
to pushes, pull requests, hooks, or `test/gate.sh`. The workflow records the harness
build and runner environment before the lane so a failure can distinguish a moved
harness surface from a changed runner substrate.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
