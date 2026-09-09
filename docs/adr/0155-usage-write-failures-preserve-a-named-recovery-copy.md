---
id: 0155
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [usage]
---

# ADR-0155: Usage write failures preserve a named recovery copy

## Context

Usage evidence crosses process and sandbox boundaries while accounting policy remains
operator-owned.

## Decision

A preparation failure is reported as a lost event.

## Consequences

If the primary append fails, the host-side worker saves the prepared JSON under the
effective archive root's `usage-unrecorded/` directory and prints the exact recovery
path; if that write also fails, it keeps and names the tmux buffer until the server
exits. None of these failures holds a teardown that has already archived the agent's
mail. The file is the operator's; `gang usage` prints its path, `--all` reads all of it,
and removing it is `rm`.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
