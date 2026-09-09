---
id: 0158
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [lifecycle]
---

# ADR-0158: A scoped hitch has an immutable cgroup identity

## Context

Agent lifecycle actions span tmux and host processes, so partial success can leave live
or durable state.

## Decision

Mint one immutable per-hitch scope identity, record the exact unit on the window, and
leave that identity unchanged across agent rename.

## Consequences

An agent's registered name is mutable: `gang rename` changes it without restarting the
pane. Naming its transient systemd scope from that registration therefore left the old
unit active after the old name was free, and the next `hitch` of that name was refused
by a resource the registry said belonged to someone else.

Each scoped hitch now mints an immutable 16-hex-digit identity and launches in
`gangline-<session>-<hitch-name>-<hitch-id>.scope`, recording the exact unit in the
window's `@gl_scope` and exposing it through `gang explain`. The hitch-time name remains
only as a human label: rename leaves that label, the record, and the running cgroup
alone, while the nonce lets a replacement reuse the old registered name. Moving a live
process to a newly named scope was rejected because it would turn a metadata rename into
a second platform mutation with a separately failing boundary; a partial move could
leave the registration and actual cgroup contradicting each other.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
