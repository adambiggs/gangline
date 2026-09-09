---
id: 0160
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [lifecycle]
---

# ADR-0160: Scope cleanup requires issuance and membership evidence

## Context

Agent lifecycle actions span tmux and host processes, so partial success can leave live
or durable state.

## Decision

That residue is now part of lifecycle rather than a systemd scavenging job.

## Consequences

Status and roster compare active nonce-bearing team scopes with the complete live
`@gl_scope` register and report the difference without mutation. Drop captures its unit
before deleting the window; down captures every live unit and every already-orphaned
unit before deleting the session. Only after that tmux identity is gone does teardown
read the surviving unit's ControlGroup and task membership and explicitly stop it,
printing what it stopped. A prefix match is not ownership, and neither is a 16-hex
suffix by itself: before launch, hitch reserves each issued nonce and exact unit under
the team's shared lock root. A look-alike without that corroborating record is reported
and left alone, and unreadable membership also leaves a unit alone. The registry
survives loss of the window or tmux server, because either loss can be the event that
leaves the scope behind; a lifecycle read removes records for collected units, and a
successful stop removes its record. Exact unit discovery reads systemd's `Id` property
rather than its human unit table, whose launch-derived Description can contain newlines
and cannot be parsed as one row per unit.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
