---
id: 0004
status: accepted
date: 2026-08-10
supersedes: []
superseded-by: []
tags: [roles]
---

# ADR-0004: Role attachment is an explicit launch choice

## Context

Role prose must guide reusable team behavior without becoming coordination state in
Gangline.

## Decision

Attachment is a launch choice like a model or an effort level.

## Consequences

`gang up` chooses the shipped `lead` role because it creates the team's lead; an
explicit role replaces that default. `gang hitch` attaches only the role its caller
names and never infers one from the agent name.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
