---
id: 0140
status: accepted
date: 2026-09-07
supersedes: []
superseded-by: []
tags: [messaging, collars]
---

# ADR-0140: A collar may answer for a queue its composer never shows

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

Gangline matched its parked-queue evidence against the input box only, so that a
delivered body quoting a harness's hint could never trip it.

## Consequences

Codex parks a follow-up by returning its composer to the placeholder and drawing the
queued bodies above it, which that reading cannot see at all: every parked delivery was
reported to its sender as submitted. A collar may therefore declare `collar_queued` and
answer from wherever its own harness draws the queue. Its unknown is carried into the
delivery outcome rather than flattened, because "gang could not tell" reaching a sender
as "submitted" is the failure this exists to remove. Asked with exact evidence from the
body gang composed, the same function settles what the hint alone cannot — a queue drawn
after Enter can describe the message just typed or a turn that raced it — and only a
positive answer is taken. Core normally asks with the whole body. When the harness
truncates previews, the unique leading Gangline attribution prefix through its nonce is
enough: it survives in the first row even when a long reply-to clause wraps, and a
pre-existing message has a different nonce.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
