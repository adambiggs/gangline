---
id: 0135
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0135: Blocked state alone gates delivery

## Context

A mandatory gate must remain reproducible, bounded, and honest about evidence it cannot
obtain.

## Decision

Delivery consults the state here and nowhere else, because this is the only state whose
composer answers for itself.

## Consequences

An unknown reading is deliberately not refused: the state surface already reports it,
and one corrupt transcript line must not strand delivery to a window whose composer read
back clean.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
