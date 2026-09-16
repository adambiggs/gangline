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
prefix, and both native harnesses document only exact-path trust. Fresh clones
under a trusted parent remain new projects and stop an unattended hitch at
security prompts. Linked worktrees isolate files while retaining the canonical
checkout's identity and native approvals. Some canonical checkouts live in a
tree another security domain can write, such as a directory shared into a VM,
and cannot host arc workspaces beneath themselves.

## Decision

If accepted, an arc workspace is a linked Git worktree of the canonical
checkout, never a clone, at `<canonical-checkout>/.worktrees/<arc>/` or outside
the checkout. A checkout hosting worktrees ignores only `/.worktrees/`. A
checkout in a tree another domain can write hosts none: its worktrees live
outside that tree, are registered locked, and land only by reviewed commit id.
Hitches use the physical worktree path, and the agent can write the parent Git
metadata that owns its HEAD, index, refs, and logs. Gangline adds no dialog
answer, trust configuration, permission override, or application state edit,
and predicts no prompt before a hitch.

## Consequences

The canonical checkout must first be trusted through each native harness. A
clone remains a separate trust decision, and a symlink is not a trust boundary.
A shared checkout's other domain can rewrite an arc's branch, as it can already
rewrite the checkout and code the host runs from it; the reviewed commit id is
what keeps a rewrite from landing. A prompt Gangline does not predict is still
observed at boot and parks the contract. Cleanup removes one exact registered
worktree rather than pruning globally. This decision is falsified if a fresh
linked worktree of a trusted checkout prompts anew, gains unrelated
permissions, cannot write its own Git metadata, appears in the parent's status,
or loses a locked registration to routine maintenance.
