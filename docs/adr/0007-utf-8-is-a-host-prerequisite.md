---
id: 0007
status: accepted
date: 2026-08-12
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0007: UTF-8 is a host prerequisite

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Refuse startup when neither the environment nor the host locale inventory can establish
UTF-8.

## Consequences

Gangline writes Unicode protocol glyphs into tmux state and reads them back; continuing
under an unverified character contract would report a degraded transport as healthy
rather than provide a supported fallback.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
