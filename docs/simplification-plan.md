# Gangline simplification plan

## Status

Complete. The mandatory acceptance is guarded by `test/lint.sh` and
`test/integration.sh`; native-harness behavior is verified only in separately
named disposable teams as documented in `docs/operations.md`.

## Target

Gangline is the local substrate for long-running, multi-harness coding sessions.
It gives native harnesses tmux transport, attributed delivery, observation,
lifecycle control, and native compaction. It does not coordinate the work,
supervise agents, prescribe team strategy, or spend agent context explaining its
own machinery.

The agent-facing surface should be obvious from a short startup message. An agent
needs to know its name, its optional role brief, who sent a message, how to reply,
and how to request native compaction. Everything else belongs in operator-facing
reference material or nowhere.

## Keep

- Lifecycle: `up`, `hitch`, `adopt`, `drop`, `down`, and `attach`.
- Attributed, verified `send`.
- Direct observation: `status`, `roster`, `capture`, and `wait`.
- Harness profiles and silent native hooks that establish observable state.
- Native compaction, including self-compaction at natural checkpoints.
- Optional context lights with exactly two thresholds: yellow and red.

## Remove or collapse

- Agent startup bookkeeping, marker turns, state-machine tutorials, and Gangline
  operating doctrine.
- Coordination policy: patrol, deadlines, cutoffs, automatic nudging, role
  management, and any component that watches another Gangline component.
- Context ladders, repeated warnings, and context output when lights are disabled.
- Agent-facing diagnostics and issue-filing machinery that belong to operators.
- Historical arguments, forensic commentary, duplicated explanations, and dead
  compatibility surface in code, profiles, roles, and documentation.
- The numbered decision archive; durable constraints live as terse entries in
  `DECISIONS.md`.

Delete policy while preserving transport guarantees: sender attribution, verified
delivery, exact addressing, truthful observation, and loud failure.

## Completed sequence

1. Made native Codex self-compaction reliable at the end of the requesting turn.
2. Collapsed standing decisions into `DECISIONS.md` and deleted the numbered archive.
3. Reduced hitch output and role briefs to the minimum agent contract.
4. Made context lights disabled by default; when enabled, they expose only yellow and
   red and notify once per context epoch.
5. Deleted coordination and supervision commands, state, hooks, tests, and docs.
6. Slimmed profiles, diagnostics, and operator documentation around the remaining
   substrate.

Each step lands as a small green checkpoint. Deletion does not wait for a grand
rewrite.

## Acceptance

- A newly hitched agent can identify itself, read its optional role, attribute and
  answer messages, and request native compaction without a bookkeeping turn.
- Disabled context lights add no prompts, warnings, markers, or roster noise.
- Enabled context lights have only yellow and red states and remain advisory.
- Self-compaction happens at a natural checkpoint through the harness's native
  mechanism and reports failure explicitly.
- The mandatory suite contains no sleeps, polling, or timeout tests and normally
  completes in seconds, with a hard ceiling under five minutes.
- Real-harness probes run only in separate, disposable tmux sessions; the
  development agent is never the test subject.
- Gangline has no daemon, supervisor, watchdog, or agent-coordination loop.
