# ADR-0001: tmux is the substrate

- **Status:** Accepted
- **Date:** 2026-07-25
- **Amended:** 2026-07-27 — impersonation paragraph softened from a mechanical
  guarantee to a trust-scoped convention; a test suite typing into the manager's
  pane demonstrated the prefix is spoofable by any writer to the pane.
- **Amended:** 2026-07-28 — three decisions restated to match what the transport
  does: messages are enveloped rather than prefixed, delivery is verified against
  the input box rather than by finding the text, and mid-turn delivery is a
  per-profile declaration rather than a blanket refusal.
- **Amended:** 2026-07-29 — the profile size estimate dropped from the decision and
  its consequence. What was load-bearing is the boundary — harness knowledge lives in
  a profile, never a branch in `bin/gang` — and law 9 owns size.
- **Amended:** 2026-07-30 — the attribution and neutralisation claims restated as
  what they are. The sender is read off the sending window where gang can see one
  and stands as claimed where it cannot: a send from a shell with no readable pane
  delivered under a borrowed name. Tag neutralisation is defence in depth over the
  nonce rather than proof that a body cannot emit an unattributed line: a fullwidth
  bracket, a capital, and whitespace inside a tag each survived it. Both measured;
  the neutraliser now matches the shape of a tag, which narrows the gap and does
  not close it.

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
- Sending = paste into the pane inside a `[gang:<sender>#<nonce>] … [/gang:…]`
  envelope, then submit. No default sender exists; an unattributed send is an
  error, and where gang can see the sending window the sender is read off that
  window rather than from what it claims.
- Delivery is verified by reading the harness's input box before and after the
  paste and requiring it to change, then requiring submission to empty it.
  Unverified sends fail loudly.
- Mid-turn delivery is the profile's declaration, not a blanket rule: a harness
  that takes input during a turn is sent to, one that does not is refused rather
  than pasted into whatever that turn is running.
- Observation = `capture-pane`. Termination = `kill-window`. Per-agent state
  (profile binding) = tmux window options.
- Per-harness knowledge lives in a profile (launch command, busy regex, compact
  command), never as a branch in `bin/gang`. Harness-native extensions require their
  own ADR proving the value is unachievable from outside.

The envelope answers impersonation as a convention, not a security property:
attributed traffic is distinguishable, but nothing stops any writer to the pane
from typing an envelope itself. What the envelope does buy mechanically is that
a body cannot close its own envelope: the nonce is minted per message, from a
body that already exists, so whoever wrote that body cannot know the value.
Neutralising tag-shaped text is defence in depth over that nonce, aimed at a
reader that is a model rather than a parser — it matches the shape of a tag
rather than one spelling of it, and homoglyphs are unbounded, so it narrows the
gap without closing it. The convention holds because every writer is already
trusted — single-tenant, one operator, one host (law 2). Keystroke relay is safe
here because attribution is mandatory, unattributed sends are an error, and no
untrusted principal can reach a pane.

## Consequences

- A new harness costs a profile, not an integration.
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
