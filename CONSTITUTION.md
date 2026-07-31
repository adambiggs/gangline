# The Gangline Constitution

These laws bind every change to this repo. Harness machinery accretes faster than the
work it serves, and components built to guard a system are a defect source in their
own right. Violating a law is a defect.

1. **Minimize bespoke integration surface.** Integrate only through universal
   surfaces: the tty (tmux), the shell (`gang` as a CLI), and open standards a
   harness speaks natively (e.g. MCP). tmux is the default transport — agents are
   tmux windows, messages are keystrokes, observation is `capture-pane`,
   termination is `kill-window`, state lives in tmux options. A harness-specific
   code path requires an ADR proving no universal surface can carry the value.
   No bespoke message buses, no databases, no daemons.

2. **Every message is attributed; trust is assumed.** A sender identity is required —
   there is no default. Where Gangline can see the sending window it reads the name off
   that window and refuses a mismatch, so an agent cannot casually sign as a peer.
   Where it cannot see one — the operator's own shell, cron — the name stands as
   claimed. This is attribution, not authentication, and it holds because the system
   is single-tenant by design: anyone at the keyboard is the operator. Never build
   authentication, generation fencing, or anti-tamper into this repo.

3. **Delivered means verified.** A send is confirmed by pane capture or it fails
   loudly. No fire-and-forget, no success receipts for messages nobody saw.

4. **Harness integration is a profile, not a plugin.** Per-harness knowledge lives in
   a profile, never as a branch in `bin/gang`; the profile contract itself is
   documented in `docs/reference.md`. Code inside a harness requires an ADR proving
   the value is real and unachievable any other way.

5. **Nothing lands without a live consumer.** If nothing invokes it the day it
   merges, it does not merge. Speculative generality is the seed of the pile.

6. **Everything has a deletion path.** Any artifact this system produces — logs,
   state, records — must say how and when it dies.

7. **The harness never manages itself.** Gangline may not grow a component whose
   job is watching, policing, or coordinating another Gangline component. That
   loop is how a harness ends up spending most of its code on itself.

8. **Fail loud.** No silent fallbacks, no degraded modes that pretend to be healthy,
   no fabricated status. A regex that stops matching a new TUI version must break
   the command, visibly.

9. **Size is watched, not capped.** Growth must justify itself against the mission:
   drive long-horizon multi-agent sessions with minimal machinery. When in doubt,
   the answer is prose in an agent's prompt, not code in this repo.
