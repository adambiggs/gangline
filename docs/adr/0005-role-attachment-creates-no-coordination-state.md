---
id: 0005
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [roles]
---

# ADR-0005: Role attachment creates no coordination state

## Context

Role prose must guide reusable team behavior without becoming coordination state in
Gangline.

## Decision

Gangline builds no coordination state around attachment: no role or brief identity is
recorded in a window or session option, no later command reads one, no output varies by
it, and nothing checks whether an agent behaved as its brief describes.

## Consequences

Delivery necessarily leaves the brief where delivery put it — in the agent's transcript
or system prompt, in the launched process's arguments, and in the launch string tmux
retains for the pane. Those are artifacts of having delivered it, readable by anyone who
can already read the pane; they are not a record Gangline keeps or consults.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
