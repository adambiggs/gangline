---
id: 0002
status: accepted
date: 2026-08-07
---

# ADR-0002: Configuration is parsed, never sourced

## Context

Operator choices need a durable file. Sourcing a shell file would execute
operator text on every command and every native hook, and silently ignoring a
typo would claim a setting Gangline did not apply.

## Decision

The config file mirrors the environment names in strict scalar lines. Gangline
parses it, never sources it, refuses unknown or duplicated keys, and lets a set
environment variable win.

## Consequences

A config file cannot run code. A misspelt key fails loudly at the next command
instead of being ignored.
