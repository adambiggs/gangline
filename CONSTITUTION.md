# Gangline design principles

Use these principles to decide whether a change belongs in Gangline. Read them
before extending the command surface, runtime, or collar interface. If a change
conflicts with a principle, redesign it before implementation.

Harness machinery grows faster than the work it serves, and components built
to guard a system create defects of their own.

1. **Minimize bespoke integration surface.** Integrate only through universal
   surfaces: the tty (tmux), the shell (`gang` as a CLI), and open standards a
   harness speaks natively (e.g. MCP). tmux is the default transport — agents are
   tmux windows, messages are keystrokes, observation is `capture-pane`,
   termination is `kill-window`, and state lives in a per-team append-only
   event log. A harness-specific
   code path requires a section of `docs/design.md` proving no universal surface
   can carry the value.
   No bespoke message buses, no databases, no daemons.

2. **Every message is attributed; trust is assumed.** A sender identity is required.
   Where Gangline can see the sending window it reads the name off that window and
   refuses a claimed identity, so an agent cannot casually sign as a peer.
   Where it cannot see one — the operator's own shell — the name stands as
   claimed. This is attribution, not authentication, and it holds because the system
   is single-tenant by design: anyone at the keyboard is the operator. Never build
   authentication, generation fencing, or anti-tamper into this repo.

3. **Delivered means verified.** A send is confirmed by the harness's native
   submit hook or it fails loudly. No fire-and-forget, no success receipts for
   messages nobody saw.

4. **Harness integration is a collar, not a plugin.** Per-harness knowledge lives in
   a collar, never as a harness-name branch in `cmd/gang`; the collar contract
   itself is documented in `docs/reference.md`. Code inside a harness requires a
   section of `docs/design.md` proving the value is real and unachievable any
   other way.

5. **Nothing lands without a live consumer.** If nothing invokes it the day it
   merges, it does not merge.

6. **Everything has a deletion path.** Any artifact this system produces — logs,
   state, records — must say how and when it dies.

7. **The harness never manages itself.** Gangline may not grow a component whose
   job is watching, policing, or coordinating another Gangline component. That
   loop is how a harness ends up spending most of its code on itself.

8. **Fail loud.** No silent fallbacks, no degraded modes that pretend to be healthy,
   no fabricated status. A regex that stops matching a new TUI version must break
   the command, visibly.

9. **Size is watched, not capped.** Growth must justify itself against the mission:
   support long-running multi-agent sessions with minimal machinery. When in doubt,
   the answer is prose in an agent's prompt, not code in this repo.
