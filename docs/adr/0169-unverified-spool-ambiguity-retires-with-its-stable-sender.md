---
id: 0169
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0169: Unverified spool ambiguity retires with its stable sender

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

Keep an unverified submission in the live target spool only while the exact stable
sender token still belongs to a window.

## Consequences

Once that identity is gone, archive the record and omit it from mail and roster; a fresh
window reusing the name has a different token and inherits nothing. Legacy or unreadable
provenance remains kept because absence was not proved. Retirement preserves the
unresolved body in a fresh archive and reports its source, destination, and deletion
command on stderr, so the ambiguity stays recoverable and cannot disappear as an
unexplained count change. Waiting delivery entries and kept ambiguity stay separate in
operator output: `spooled` is mail Gangline will deliver, while `spool-held` is a record
it will not deliver.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
