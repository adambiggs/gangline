---
id: 0018
status: accepted
date: 2026-09-08
---

# ADR-0018: Release Please holds no tag-creation override

## Context

GitHub's Create a Reference call refuses an Actions `GITHUB_TOKEN` for a tag
whose target's `.github/workflows/` differs from the default branch, and refuses
before checking whether the ref exists. A tag-first ordering therefore failed
with `Resource not accessible by integration` on 2.11.1.

## Decision

Release Please issues no separate tag-creation call. A release is tagged by its
Create a release call, as every release through 2.11.0 was.

## Consequences

A release commit whose workflow files have fallen behind `main` publishes only
once its tag exists, and that tag is pushed from outside Actions under a
credential holding the workflow scope. Tag name and target stay Release
Please's.
