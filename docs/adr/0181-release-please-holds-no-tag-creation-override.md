---
id: 0181
status: accepted
date: 2026-09-08
supersedes: []
superseded-by: []
tags: [release]
---

# ADR-0181: Release Please holds no tag-creation override

## Context

Release automation must preserve the exact commit and tag selected by the release
process.

## Decision

Release Please issues no separate tag-creation call. A release whose tag does not
already exist is tagged by its Create a release call, which is how every release through
2.11.0 was tagged.

## Consequences

Create a Reference refuses an Actions `GITHUB_TOKEN` for a tag whose target differs from
the default branch under `.github/workflows/`, and it refuses before reading whether the
ref already exists. With `refs/tags/gangline-v2.11.1` present at the exact commit
Release Please selects for it, that call answers `Resource not accessible by
integration` rather than the already-exists status a tag-first ordering needs. An
ordering that creates the tag first therefore never reaches its existing-tag branch, and
it displaces the release call, which has an existing-tag path of its own: the release
API ignores `target_commitish` when its tag already exists.

A release commit whose workflow files have fallen behind `main` is publishable only
through that existing-tag path, and only once its tag exists. Nothing available to
`GITHUB_TOKEN` can create that tag, because workflows permission is not among the
permissions a workflow may grant it, so such a tag reaches the remote from outside
Actions: `gangline-v2.11.1` carries the numbered version its own release commit
declares, pushed under a credential holding the workflow scope.

Tag name and target commit stay Release Please's own, chosen from its merged release
pull request, and the same main-push verification jobs gate the release job.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
