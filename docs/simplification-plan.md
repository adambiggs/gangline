# Gangline simplification plan

## Target

Gangline is the local substrate for long-running, multi-harness coding sessions.
It gives native harnesses tmux transport, attributed delivery, observation,
lifecycle control, and native compaction. It does not coordinate the work,
supervise agents, prescribe team strategy, or spend agent context explaining its
own machinery.

The agent-facing surface should fit in one short startup message: its name, how
to message a peer, and how to request native compaction. Everything
else belongs in operator reference material or nowhere.

## Keep

- Lifecycle: `up`, `hitch`, `adopt`, `drop`, `down`, and `attach`.
- Attributed, verified `send`.
- Direct observation: `status`, `roster`, and `capture`.
- Harness profiles and silent native hooks that establish observable state.
- Native compaction, including self-compaction at natural checkpoints.
- Optional context lights with exactly two intentionally high thresholds:
  yellow and red.
- One optional team cutoff with exactly two relative advisory edges.

## Refuse or remove

- Agent startup bookkeeping, marker turns, state-machine tutorials, and Gangline
  operating doctrine.
- Coordination policy: patrol, deadline enforcement, per-agent budgets,
  automatic nudging, role management, and any component that watches another
  Gangline component.
- Context ladders, repeated warnings, and context output when lights are disabled.
- Agent-facing diagnostics and issue-filing machinery that belong to operators.
- Historical arguments, forensic commentary, duplicated explanations, and dead
  compatibility surface in code, profiles, and documentation.
- The numbered decision archive; durable constraints live as terse entries in
  `DECISIONS.md`.

Delete policy while preserving transport guarantees: sender attribution, verified
delivery, exact addressing, truthful observation, and loud failure.

## Acceptance

- A newly hitched agent can identify itself, receive attributed messages, send
  to peers, and request native compaction without a bookkeeping turn.
- Disabled context lights add no prompts, warnings, markers, or roster noise.
- Enabled context lights have only intentionally high yellow and red states,
  preserve most of the native context window, and remain advisory.
- A declared team cutoff has only yellow and red states and remains advisory.
- Self-compaction happens at a natural checkpoint through the harness's native
  mechanism and reports failure explicitly.
- The mandatory suite contains no sleeps, polling, or timeout tests and normally
  completes in seconds, with a hard ceiling under five minutes.
- Real-harness probes run only in separate, disposable tmux sessions; the
  development agent is never the test subject.
- Gangline has no daemon, supervisor, watchdog, or agent-coordination loop.
