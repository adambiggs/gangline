---
id: 0041
status: accepted
date: 2026-08-07
supersedes: []
superseded-by: []
tags: [configuration]
---

# ADR-0041: Configuration is parsed, never sourced

## Context

Operator choices need durable configuration without exposing internal implementation
seams as promises.

## Decision

Mirror the existing environment names in one strict scalar file, keep a set environment
variable authoritative, and refuse unknown or duplicated keys.

## Consequences

Sourcing would execute operator text on every command and native hook; silently ignoring
a typo would claim a setting Gangline did not apply.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
