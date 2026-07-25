# ADR-0001: tmux is the substrate

- **Status:** Accepted
- **Date:** 2026-07-25

## Context

Gangline's predecessor (the tombo repo's agent-harness stack) coordinated agents
through harness-native extensions: message facades, SQLite buses, spawn fences,
stop guardians, status UIs. In 20 days it grew to ~70,000 lines — larger than the
robot it served — while five independent audits found: a telemetry tool returning
hardcoded fake health strings for live hardware, a "foundation verifier" that was a
regex returning VERIFIED, a coordination bus whose 140 green tests never exercised
the two principals actually deployed, and guard components that generated the
defects they guarded against (its BUG-0165/0166/0167). Every major component was,
at bottom, machinery replacing something tmux already does: messaging replaced
typing into a pane, status UIs replaced looking at one, stop-guardians replaced
Ctrl+C, hot-reload replaced restarting a process.

The predecessor also banned keystroke relays (its BUG-0146) after injected
keystrokes were mistaken for the operator. The substance of that defect was
**impersonation** — messages arriving with no sender identity — not the transport.

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

The required sender prefix carries the predecessor's impersonation lesson into the
substrate mechanically: an agent can always tell operator input from peer traffic.
This supersedes the predecessor's transport ban while keeping its safety property.

## Consequences

- A new harness costs a profile (~10 lines), not an integration.
- The operator can attach to any window and watch, steer, or kill — the transcript
  is the pane.
- Text conventions are version-fragile: a TUI update can change a busy marker.
  Accepted: per law 8 the break is loud, and the fix is one line in a profile.
- Offline delivery (messaging an agent not yet spawned) is a file included in the
  spawn prompt, not a store-and-forward bus.
- Single-tenant trust is assumed (law 2); Gangline is not multi-user software.
