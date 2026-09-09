---
id: 0164
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0164: A reply is answerable in the turn that read it

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

A message sent to a reply's sender in the turn that read the reply is correlated to it
and opens no debt, so a thread closes on any acknowledgement.

## Consequences

Under the former rule every acknowledgement of a reply was a request, and a thread could
not end without one message left unanswered, at one paid turn per acknowledgement. The
reply record takes the same settlement proof a request does, written by the
acknowledgement that answered it or by the end of the turn which read it: its native
Stop, `gang interrupt`, or the prompt that begins the next turn when no Stop closed the
last one. Left open, every later message to that peer would be a correlated reply and a
follow-up request would open no debt. The Stop writes the close after every other fact
of the boundary and refuses the boundary when the close cannot be written, so a refused
Stop has closed nothing and an allowed one has closed everything it read. A reply not
yet read is left open across a boundary, since its turn has not begun; a prompt inside a
live turn is steering and closes nothing.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
