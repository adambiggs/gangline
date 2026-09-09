---
id: 0027
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0027: A timed send is a spool entry the queue cannot see yet

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

`gang at` parks an ordinary attributed envelope in the target's own spool under a
dot-prefixed name. Every drain, count, age and supersede path matches `[0-9]*`, so no
predicate had to learn about time and no second store exists. The clock promotes by
renaming the entry into that namespace and asking for a drain; from there it is ordinary
waiting mail with the ordinary verification. There is no watcher, scheduler or retry
loop, and the wake is one transient systemd user timer collected after it fires, as a
provider-reset wait is.

## Consequences

Attribution is taken when the message is parked, not when it is delivered. A clock has
no pane, so re-attributing at fire time would turn an identity Gangline observed into
one it was told.

The dot is also the accounting. A spool archives every child and strips leading dots, so
an agent dropped before its time leaves the message readable in the archive rather than
vanishing, and an orphan sweep carries it the same way; its deletion path is the
spool's, unchanged. A timer that never runs leaves the entry exactly where it was, and
status reads the unit as well as the entry so a promise nothing will keep is reported as
one, with "gone" kept apart from "could not be read".

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
