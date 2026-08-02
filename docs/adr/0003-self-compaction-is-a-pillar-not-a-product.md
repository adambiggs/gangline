# ADR-0003: Self-compaction is a pillar, not a product

- **Status:** Accepted
- **Date:** 2026-07-29

## Context

Self-compaction is the part of Gangline that people who see it first ask to have
on its own: measure an agent's context, warn it as it crosses a band, and let it
compact itself and pick the thread back up. It is useful without a second agent
in the session, which is what makes "ship it as a standalone tool" recur. ADR-0002
answered "why not MCP" once so the question would stop being re-litigated; this
does the same job for extraction.

The proposal is really three, and they have three answers.

What the loop is made of matters to all three, so it is worth stating
structurally, as measured on this page's date. The policy layer is the band
ladder (`ladder`), the band state and the note it paints (`band_state_read`,
`band_state_write`, `band_note`), the compact command (`cmd_compact`), the
resume waiter (`resume_after_compaction` with `compaction_mark`,
`compaction_clear`, `compaction_pending`), and the sweep that applies them
(`cmd_patrol`, `patrol_one`).

Under it, and not separable from it, is the delivery substrate: `inject`,
`gated`, the pane locks (`lock_base`, `lock_pane`, `lock_release`),
`busy_painted`, and `compacting` — plus the context-drop proof family
(`context_*`), which is what makes a claim about a compaction having happened
evidence rather than assertion.

So the policy layer is not a small thing sitting on a large one. It measured
slightly larger than the substrate it stands on, and every line of it reaches
down.

## Decision

Self-compaction stays a pillar of Gangline. It is not extracted, not reduced to a
hook, and not given a second name.

**Full extraction into a standalone tool: no.** The policy layer cannot act
without the substrate. Warning an agent is a verified send; compacting one is a
send of the harness's own command; resuming it is a send that has to wait for
compaction to settle and must not land in a pane that a modal owns. Each of those
is `inject`, `gated`, and a pane lock. A standalone tool therefore either
**vendors** that substrate — two copies of the injection path, two busy-marker
tables, two rot treadmills drifting apart on their own schedules, which is the
defect ADR-0001 exists to prevent — or **depends** on Gangline, in which case it is
an alias for a command Gangline already ships. Neither is a tool.

**A no-tmux, hook-only version: structurally lesser, and not built.** Per
ADR-0002 the only universal way to synthesize input into a live session is the
tty; no harness turns an external event into a new turn in an agent's own loop. A
hook can therefore measure and it can warn, but it cannot act — the human still
types `/compact`. That drops the third verb from measure/warn/act, and the third
verb is what makes the loop a loop rather than a meter. It is a different thing
from this thing. Law 5 settles whether to build it anyway: no live consumer.

**A second brand for Gangline-minus-teams: no.** That artifact already exists,
and it is `gang up` with nobody else hitched. A second name for the same binary
buys a landing page and costs a concept: two vocabularies, two sets of docs to
hold current, and a reader who has to be told the two are the same program.

## Consequences

- Extraction proposals get this ADR rather than a redesign cycle.
- The solo path is documented as a *mode* of Gangline rather than a product —
  README's "A team of one" — so nothing is renamed or aliased to serve it.
- Watch item, not work item (law 5): if a harness ships a native programmatic
  channel that can both trigger compaction and queue what follows it without a
  tty, that becomes a profile transport under law 4, the same way ADR-0002 leaves
  the door for an external-control channel. Gangline's verbs would not move; only the
  delivery mechanism behind them. Still one tree.
- Size is watched, not capped (law 9): the loop and the evidence it produces are the
  largest single concern in the file, outweighing every other `cmd_*` verb put
  together. That is a weight to re-measure at each addition to it, and it argues for
  keeping one copy of the substrate rather than for splitting the loop out of it.
