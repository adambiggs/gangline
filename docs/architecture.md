# Architecture

Gangline is a Bash CLI that uses tmux as its substrate: a team is a tmux
session, agents are windows, input is sent as keystrokes, panes provide direct
observation, and tmux options hold live per-window facts. It has no daemon or
database. The core stays in one script so installation and the execution path
are direct; banners make its sections navigable. Claude Code and Codex are
first-class harnesses; a collar supplies the small native interface another
harness needs.

`bin/gang` is ordered by dependency rather than by command flow. Its section
banners mark these layers:

- **State root** keeps local team records, locks, queues, and recovery
  evidence together.
- **Collars** load a harness's commands and evidence readers without putting
  harness branches in the core.
- **Tmux substrate** resolves registered windows, reads panes, and decides when
  a composer is safe to receive input.
- **Delivery and spool** attributes an envelope, serializes a target pane,
  verifies acceptance after submission, and parks a pre-keystroke refusal for a
  later safe boundary. An outcome that cannot be verified is held, not sent
  again.
- **Tick** runs bounded maintenance passes: it drains safe queued work and
  refreshes observable state.
- **Hooks** turn native harness events into recorded facts for the same state
  and delivery paths.
- **Cap** reads provider-window history through its helper and renders the
  resulting capacity evidence.

For a normal message, `gang send` derives or accepts the sender label, builds
an envelope, resolves the recipient and asks its collar whether the composer is
ready. It writes the envelope to that pane, submits it, then captures the pane
to verify that it was accepted. If the check fails before typing, the message is
spooled; a hook or tick later tries it at a safe boundary. If typing may already
have succeeded, Gangline reports the uncertainty and does not create a second
copy.

The local gate runs fast lint and smoke. CI runs integration suites that use
immediate state, event barriers, and fake clocks instead of sleeps or polling;
native harness turns belong to the opt-in end-to-end test.

## Why Bash, and where it stops

Bash needs no extra runtime, works where tmux works, and remains compatible
with macOS Bash 3.2. It also makes terminal commands and shell-level fixtures
plain to inspect. Its costs are weak data modelling, awkward error propagation,
and increasingly fragile concurrent control flow. A compiled rewrite is worth
considering when structured concurrent state or portable terminal/process
control would reduce the integration surface and preserve the same observable
delivery proof; a language change alone is not a reason.
