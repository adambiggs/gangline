---
id: 0077
status: accepted
date: 2026-08-12
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0077: Commit gates require a destination boundary

## Context

A mandatory gate must remain reproducible, bounded, and honest about evidence it cannot
obtain.

## Decision

Check Conventional Commits over an exact event range and refuse an unusable or all-zero
base.

## Consequences

Post-push ref advertisement cannot determine a new ref's pushed range: two refs created
in one push are already visible to both runs, and two equal new refs are
indistinguishable from a routine new branch at a pre-existing head. Default-branch
merge-base therefore admits a false refusal by including commits already on another
branch, while subtracting other advertised refs admits a false clean when concurrent new
refs overlap. Neither is a measured replacement for the missing pre-push boundary; pull
requests retain their real base and exact range. The first push of a branch carries an
all-zero `before` and therefore goes red; that is the accepted cost of refusing to
invent its missing event boundary.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
