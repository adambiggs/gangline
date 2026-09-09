---
id: 0159
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [lifecycle]
---

# ADR-0159: Per-hitch scope identity permits registered-name reuse

## Context

Agent lifecycle actions span tmux and host processes, so partial success can leave live
or durable state.

## Decision

The old deterministic name made a leaked scope discoverable on the next hitch of that
agent name.

## Consequences

Per-hitch identity deliberately gives up that collision as a discovery path: normal
teardown relies on systemd-run's `--collect`, while a detached child may keep its unit
alive after the window and its exact `@gl_scope` record are gone. Keeping the hitch-time
label in the unit preserves human attribution in systemd's unit list; it does not
reserve the registration or prevent another hitch of that name.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
