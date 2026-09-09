---
id: 0183
status: accepted
date: 2026-09-09
supersedes: []
superseded-by: []
tags: [ticks]
---

# ADR-0183: Actionable advisory collars receive empty-spool ticks

## Context

An actionable advisory can appear after the native boundary that starts a harness
turn. A cooperative pass limited to windows with attributed mail cannot reach the
collar action while that window's spool is empty.

Gangline remains cooperative: team commands and harness-native boundaries supply
ticks, without a resident coordinator or terminal-output watcher.

## Decision

Every cooperative tick visits a window whose collar declares
`collar_dismiss_advisory`, even when that window has no attributed mail. An ordinary
team command or a native hook boundary from any agent can therefore offer the existing
locked advisory action to every actionable collar.

The collar remains the only authority for a key. Core still proves occupancy before
calling it, and the collar still requires the selected row and complete
bottom-anchored menu before sending the dismissal shortcut. Delivery still waits for
the composer to return empty and stable.

## Consequences

A team with no other activity leaves the menu to the provider's own answer. Gangline
does not poll pane output, create another tmux session, or install a standing watcher.

An actionable collar adds one guarded pane visit to each cooperative pass even when
its spool is empty. The pass owns delivery sequencing during that visit, so a
concurrent native Stop leaves its duplicate spool or self-compaction dispatcher to
the pass or a later cooperative boundary. Provider text left in scrollback, the
no-retry provider form, unrelated numbered choosers, and the approaching-rate-limit
menu remain unable to authorize a key. A menu that closes between the collar read and
its shortcut still fails the post-key composer proof and cannot release or consume
attributed mail.
