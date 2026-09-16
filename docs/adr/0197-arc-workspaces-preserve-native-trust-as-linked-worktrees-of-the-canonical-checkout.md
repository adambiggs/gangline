---
id: 0197
status: proposed
date: 2026-09-16
supersedes: []
superseded-by: []
tags: [worktrees, trust, security]
---

# ADR-0197: Arc workspaces preserve native trust as linked worktrees of the canonical checkout

## Context

Native workspace trust belongs to a repository identity, not a directory
prefix. Fresh clones under a trusted parent remain new projects and can stop an
unattended hitch at security prompts. Linked worktrees isolate files while
retaining the canonical checkout's repository identity and native approvals.

## Decision

If accepted, an arc workspace is a linked Git worktree at
`<canonical-checkout>/.worktrees/<arc>/`. The repository root ignores only
`/.worktrees/`. Hitches use the physical worktree path, and the agent can write
the parent Git metadata that owns its HEAD, index, refs, and logs. Gangline adds
no dialog answer, trust configuration, permission override, or application
state edit.

## Consequences

The canonical checkout must first be trusted through each native harness.
Repositories adopting the convention add the same root-anchored ignore. A
clone remains a separate trust decision, and a symlink is not a trust boundary.
Cleanup identifies and removes one exact registered worktree rather than
pruning globally. The live tree follows the operator directive, not this
proposal. This decision is falsified if a fresh linked worktree of a trusted
checkout prompts anew, gains unrelated permissions, cannot write its own Git
metadata, or appears in the parent's status.
