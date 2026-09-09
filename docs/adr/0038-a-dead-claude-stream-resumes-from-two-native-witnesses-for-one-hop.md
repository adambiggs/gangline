---
id: 0038
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0038: A dead Claude stream resumes from two native witnesses, for one hop

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Claude Code emits no Stop when a provider stream dies, but its later `idle_prompt`
notification binds the transcript path and the newest top-level assistant record carries
`error`, `isApiErrorMessage`, and a UUID.

## Consequences

Require both: the notification proves the interactive harness is waiting, and the
structural record distinguishes a dead turn from ordinary idleness without matching its
prose. Under `GANG_AUTO_RESUME`, close that missing turn boundary and submit one
ordinary attributed continuation per error UUID. The continuation's own Gangline
envelope is recorded before submission and compared byte-for-byte with the native prompt
event; a failure of that owned turn gets no second hop. When ownership cannot be proved,
fail closed and record the refusal for status and roster. An ordinary prompt opens a new
episode but does not erase an unseen refusal; positive ownership of a later automatic
turn does. This is immediate collar-native discrimination, not a watcher or a general
retry policy; collars with no structural record declare no equivalent.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
