---
id: 0175
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0175: Reviewer-diversity evaluation requires Gangline dispatch

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The scored line is unsatisfiable through a harness's own sub-agents, which inherit the
lead's harness and model and therefore its error modes.

## Consequences

A scenario measuring the line must name Gangline dispatch, or it scores a lead that had
no choice to make.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
