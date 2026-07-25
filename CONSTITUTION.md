# The Gangline Constitution

These laws bind every change to this repo. They exist because the predecessor harness
reached 70,000 lines in 20 days — larger than the robot it served — and its guard
components caused more defects than they prevented. Violating a law is a defect.

1. **tmux is the substrate.** Agents are tmux windows. Messages are keystrokes.
   Observation is `capture-pane`. Termination is `kill-window`. State lives in tmux
   options. No message buses, no databases, no IPC layers, no daemons.

2. **Identity is a prefix; trust is assumed.** Every injected line carries
   `[gl:<sender>]`, and a sender identity is required — there is no default.
   Single-tenant by design: anyone at the keyboard is the operator. Never build
   authentication, generation fencing, or anti-tamper into this repo.

3. **Delivered means verified.** A send is confirmed by pane capture or it fails
   loudly. No fire-and-forget, no success receipts for messages nobody saw.

4. **Harness integration is a profile, not a plugin.** Per-harness knowledge is a
   few lines of config: launch command, busy marker, compact command. Code inside a
   harness requires an ADR proving the value is real and unachievable any other way.

5. **Nothing lands without a live consumer.** If nothing invokes it the day it
   merges, it does not merge. Speculative generality is the seed of the pile.

6. **Everything has a deletion path.** Any artifact this system produces — logs,
   state, records — must say how and when it dies.

7. **The harness never manages itself.** Gangline may not grow a component whose
   job is watching, policing, or coordinating another Gangline component. This
   exact loop is what ate the predecessor.

8. **Fail loud.** No silent fallbacks, no degraded modes that pretend to be healthy,
   no fabricated status. A regex that stops matching a new TUI version must break
   the command, visibly.

9. **Size is watched, not capped.** Growth must justify itself against the mission:
   drive long-horizon multi-agent sessions with minimal machinery. When in doubt,
   the answer is prose in an agent's prompt, not code in this repo.
