# ADR-0001: tmux is the substrate

- **Status:** Accepted
- **Date:** 2026-07-25

## Context

Coordinating agents through harness-native extensions — message facades, SQLite
buses, spawn fences, stop guardians, status UIs — builds machinery that replaces
something tmux already does: messaging replaces typing into a pane, status UIs
replace looking at one, stop-guardians replace Ctrl+C, hot-reload replaces
restarting a process. Such stacks outgrow the work they serve, and the components
added to guard them generate the defects they guard against: health strings no
live system produced, a "verifier" that is a regex returning VERIFIED, a green
test suite that never exercises the principals actually deployed.

Keystroke relay carries a real defect of its own — injected keystrokes mistaken
for the operator. The substance of that is **impersonation**: messages arriving
with no sender identity. It is a property of unattributed messages, not of the
transport.

## Decision

tmux is the communication substrate, not a fallback and not a hack.

- One tmux session per team; one window per agent; window name = agent identity.
- Sending = paste into the pane with a mandatory `[gang:<sender>]` prefix, then
  submit. No default sender exists; an unattributed send is an error.
- Delivery is verified by capturing the pane and finding the injected text before
  submitting. Unverified sends fail loudly.
- Never inject mid-turn: each harness profile declares a busy marker; sends refuse
  or wait while it matches.
- Observation = `capture-pane`. Termination = `kill-window`. Per-agent state
  (profile binding) = tmux window options.
- Per-harness knowledge lives in a profile of a few lines (launch command, busy
  regex, compact command). Harness-native extensions require their own ADR proving
  the value is unachievable from outside.

The required sender prefix answers impersonation mechanically: an agent can always
tell operator input from peer traffic. Keystroke relay is safe here precisely
because attribution is mandatory and unattributed sends are an error.

## Consequences

- A new harness costs a profile (~10 lines), not an integration.
- The operator can attach to any window and watch, steer, or kill — the transcript
  is the pane.
- Text conventions are version-fragile: a TUI update can change a busy marker.
  Accepted: per law 8 the break is loud, and the fix is one line in a profile.
- Offline delivery (messaging an agent not yet spawned) is a file included in the
  spawn prompt, not a store-and-forward bus.
- Single-tenant trust is assumed (law 2); Gangline is not multi-user software.
- tmux is the default, not a dogma: any open standard a harness speaks — natively
  or through its own supported package system — qualifies as a universal surface
  under law 1. MCP qualifies pair-wide here: Claude Code speaks it natively, and
  Pi core omits it by design but supports it via installed packages (its
  docs/usage.md: "does not include built-in MCP… build or install those workflows
  as extensions or packages"), a one-line settings install. An MCP face for gang
  (typed send/roster/wait/capture tools) lands when a consumer wants it.
