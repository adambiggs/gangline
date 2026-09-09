---
id: 0053
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0053: Rename compatibility depends on whether a name is published

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Before publication, replace an abandoned name everywhere without compatibility
breadcrumbs or rename history. After publication, preserve compatibility and use normal
deprecation because the old name has become an external fact.

## Consequences

A name the 1.0 release renamed keeps its old spelling as an accepted alias through 1.x
and is removed in 2.0. Every alias announces itself on stderr, naming the new spelling
and the removal release, so a working setup keeps working and says what to change. The
promise is that it is announced, not that it is announced exactly once: a setting read
once per process says so once, while a collar declaration read per window says so per
window, and deduplicating that would need durable state worth less than the line it
would suppress. Old and new spellings of one setting are never both honored: two names
for one setting is a refusal naming both origins, because preferring either silently
would claim a configuration the operator did not write. Announcement is for names the
operator wrote and still holds. Gangline's own window and session state is migrated in
place and silently, by ordinary commands and by the native hook endpoint alike: there is
nothing there for an operator to change, and carrying the news across the process that
erased the evidence would take durable marker state no consumer wants. Two values that
genuinely disagree still refuse.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
