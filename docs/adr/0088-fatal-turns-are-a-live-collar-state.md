---
id: 0088
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0088: Fatal turns are a live collar state

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

An optional collar reader classifies native fatal-turn evidence as matched, absent, or
unreadable.

## Consequences

A match becomes `!bricked!` before ordinary busy paint; unreadable becomes `?unknown?`,
and transient errors remain absent unless the collar proves otherwise. The state is
recomputed by observation and stores no watcher record. Claude seeks backward to the
newest complete top-level semantic transcript record; an in-flight unterminated append
is not a record, while complete malformed data stays unknown. Meta notices and
tool-result-only records cannot clear the state, a newer real user turn outranks an old
failure, and a retryable API error cannot be borrowed as selected-model evidence or
auto-resumed as if replay could repair the selected model.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
