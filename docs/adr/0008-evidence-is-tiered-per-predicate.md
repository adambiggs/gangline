# ADR-0008: Evidence is tiered per predicate

- **Status:** Accepted
- **Date:** 2026-08-01
- **Erratum (2026-08-01):** this page was written holding that compaction paints
  nothing on the pane. That was refuted for manual `/compact`, which paints
  "Compacting conversation" over a bar the busy regex already matches;
  auto-compaction remains unobserved (gh #57). Context now states the corrected
  mechanism, and the current account lives in profiles/claude-code.sh, landed
  with the compaction bracket (3698e20). The decision is unaffected — the event
  tier exists for what the pane cannot witness, and the unobserved auto case is
  exactly that.

## Context

Every verdict Gangline issues about a harness — busy, idle, occupied, compacting,
how full its context is — is read off the pane. The pane is the one witness every
harness has (law 1), and it stays the reason a new harness costs a profile rather
than an integration. It is also a weak witness in ways this repo has measured and
named: on claude-code the busy marker is absent from most frames of a
demonstrably live turn, so its silence is no evidence that the turn has ended;
and what a manual compaction paints is a bar the busy regex already matches, so
the pane cannot separate compacting from working at all. ADR-0001
accepts the version fragility of text conventions as a cost, loud per law 8, one
line to fix.

Meanwhile the harnesses have started volunteering the very facts the scrapers
reconstruct. Claude Code fires named hook events — turn start, turn end,
permission request, compaction start, compaction end — from the operator's
settings; codex takes a `notify` program that fires at turn completion. These
surfaces were verified against the vendor's own docs on 2026-08-01, including the
edges: `Stop` does not fire on an Esc interrupt (a separate `StopFailure` covers
API errors), `PreCompact` distinguishes `manual` from `auto`, `PostCompact`
exists and carries the summary, and no hook payload carries a token count — the
statusline JSON is the only context source.

The question this ADR answers is the standing of those events relative to the
pane. The wrong answer is "replace." Three things only the pane can witness: boot
(hooks exist only after the harness has loaded its config — the first-run trust
dialog appears before any hook is wired), death (a crashed harness emits no
I-died event), and some closures (nothing fires when a permission dialog is
dismissed). And ADR-0002 is untouched on the other side of the wire: no event
channel starts a turn, so delivery stays the tty. Hooks are the harness's voice
out. They are never Gangline's hand in.

## Decision

**Per predicate, evidence is tiered: owned event > owned file > pane scrape.**
The profile declares which tiers exist for each predicate, and degradation is
per-predicate, never per-profile. A harness with event-witnessed turns and
scrape-witnessed dialogs is not "partially migrated"; it is a portfolio, and
every shipped harness will be one.

- **Facts carry an epoch and a bound.** A hook-fed fact is a window option
  holding value and write-time, trusted until its freshness bound and then
  neither believed nor inverted: expiry is the could-not-determine third answer,
  never last-known-good. The shapes already exist in the tree and are reused,
  not reinvented: `@gl_activity_only_since` (time-bounded trust a stronger
  signal clears), `@gl_band` (two writers, one ledger, rebuild-not-refuse on a
  malformed value), `@gl_compacting` (a mark with a grace bound).

- **One ingestion verb.** `context-hook` grows into `gang hook`: the harness
  fires the same command for every event, the verb branches on the native event
  name, and the event→fact mapping is Gangline's vocabulary, not the settings
  file's. Binding is unchanged from context-hook today: `$TMUX_PANE` to
  `@gl_profile` to the window's own options.

- **Wired at hitch, nothing on disk.** The profile injects the hook wiring into
  the launch line — `--settings` with an inline JSON string for claude-code,
  `-c notify=` for codex — so the operator's own config is untouched, the same
  principle the opencode profile established with `OPENCODE_CONFIG_CONTENT`.
  Inline means there is no generated file to owe law 6 a deletion path.

- **Translators are dumb and always step aside.** A hook invocation writes its
  fact and exits 0. It never uses a decision channel — no exit-2, no
  `"decision":"block"` — because a translator that can fail a harness's own turn
  is the self-management loop law 7 forbids, wearing a new coat. Law 8's
  loudness lives one level up, where it can be loud without breaking the work:
  an expected fact that stops arriving surfaces as expiry, and `vet --probe`
  asserts the pipeline end to end.

- **The write path is dumb; the read path is smart.** Ingestion distills a
  payload to the fact vocabulary — event name, epoch, and the field or two a
  predicate reads (a trigger, a notification type) — and stores nothing else.
  Interpretation happens at read time, where the verdict functions weigh tier
  and freshness at the moment a question is asked. Nothing processes a stream,
  which is why nothing needs a daemon.

- **Wired per predicate, not per firehose.** The harness offers dozens of
  events; the wiring names exactly those that feed a declared predicate with a
  live consumer (law 5). One more event later costs a settings line and a
  mapping case, which is why none are wired in case.

- **Tiers pick witnesses; they don't vote.** The reader takes the highest tier
  that is still fresh — it never blends witnesses into a score. When two tiers
  are both fresh and disagree, that is a finding, not an averaging problem: an
  event saying idle under a pane still painting a spinner means the scrape
  regex has almost certainly rotted, and the disagreement feeds the ROT RISK
  surface `vet` already owns. The event tier thereby vets the scrape tier
  passively, during ordinary operation, for free.

- **The paper trail is a log, not a store.** Fact transitions — a bracket
  opening or closing, a compaction starting or ending, an occupied raise or
  clear — append a row to the rotated log family the patrol log established;
  heartbeats do not. A postmortem reads that log beside the harness's own
  transcript. Window options remain the only live store.

- **A fact lands with its consumer** (law 5). Turn brackets land in the same
  change that teaches `busy()` to prefer a fresh bracket over paint and pty;
  compaction events land with the `compaction_pending()` branches that read
  them; the occupied raise lands with the scrape-confirmed clear. A fact nobody
  reads the day it merges does not merge.

- **Claude-code first, codex second.** The claude-code portfolio: turns
  bracketed by `UserPromptSubmit`/`Stop` with `PostToolUse` as heartbeat — and
  because `Stop` stays silent on interrupt and crash, an open bracket expires
  like any fact; compaction bracketed by `PreCompact`/`PostCompact`, which
  covers the harness's own auto-compaction — the case `@gl_compacting`
  deliberately does not; occupied raised by the permission-request event and
  cleared by scrape; context written directly by the statusline script Gangline
  already ships — it keeps painting the beacon for the human and the fallback,
  and stops being a paint-then-scrape round trip, which is the artifact the
  zero-token guards exist for. The codex portfolio is mixed by design: `notify`
  closes a turn, the open is Gangline's own send-mark where Gangline did the
  sending and scrape where the operator did, and the rollout JSONL already is
  the owned-file context tier. opencode and pi stay whole-portfolio scrape —
  standing, not legacy.

### What this is not

- Not a plugin API, a daemon, or a bus. A "plugin" here is two lines of config
  naming a `gang` invocation, fired by the harness, dead with the window.
- Not scrape retirement. The composer-parsing awk programs survive, demoted:
  still load-bearing for interleaving-avoidance and for every predicate's
  bottom tier, no longer the sole witness of state.
- Not a capability mode. There is no "hook-native harness"; the unit of
  capability is the predicate.
- Not a transport change. ADR-0002 stands: inbound is the tty, and nothing here
  wakes an idle agent.
- Not a warehouse. The last write per predicate and tier is the entire evidence
  store, and a fact past its bound is refused, not archived — forgetting is the
  feature. History-questions read the harness's own records where they live
  (codex's rollout JSONL, claude-code's transcripts); copying them gang-side is
  two truths drifting on separate lifecycles, ADR-0003's vendoring argument in
  a new costume. ADR-0007 already rejected a one-line durable file; a database
  is that objection multiplied, and law 1 says the word outright.
- Not authentication (law 2). A fact is a trusted claim from a trusted harness
  in a single-tenant system, bound to its window the way context-hook already
  is.

## Consequences

- `vet --probe` stops watching for a spinner glyph at 4Hz and starts asserting
  that the expected facts arrive and expire across a driven turn — and can
  finally probe surfaces it currently declares `not probed`, occupancy and
  compaction first among them.
- The measured blind spots close by name: the unmarked streaming turn is held
  by bracket-plus-heartbeat, the invisible compaction by its event bracket.
  What remains of them is the scrape tier's problem only when the tiers above
  are absent or expired.
- The zero-token beacon guards in bin/gang's context read path become
  fallback-path guards rather than the main line.
- Hooks supplied via `--settings` face no trust gate. A window gang hitched with
  its own inline payload fired them with no prompt in the way, on a project whose
  trust dialog had never been accepted. Two caveats travel with that, and neither
  is decoration: it was established by watching a live launch, not by proving no
  gate exists inside the harness; and this host had already completed onboarding,
  so a fresh install, or an image that has never run the harness at all, is
  untested. Either way the pre-hook trust dialog itself stays scrape-tier
  forever, because it exists before hooks do.
- The context beacon rides the same payload the hooks ride, so the context tier
  no longer waits on an operator having wired a statusline by hand. The merge is
  BY KEY: a key the payload does not name survives from the operator's settings
  file, and a key it names is overridden for that session. Measured in both
  directions at one launch with a statusLine wired on each side — the inline one
  painted, and the operator's did not paint in any form. Intended and actual
  agree, so nothing here leans on a precedence the harness was not already going
  to give it. Scope stated plainly: one host, by observation, not proven from the
  binary.
- Exec form deletes the fire-time shell for the hooks and NOT for the beacon.
  Hook commands are a command plus an argv, so nothing re-splits a path when they
  fire; `statusLine` has no exec form at all, and its command is one string handed
  to a shell every time the line paints. The third quoting layer comes back for
  that one key, which is why the payload escapes what a double-quoted path still
  leaves live rather than trusting the launch-line guard alone.
- A window gang ADOPTED gets none of that. It was launched by somebody else, so
  no inline payload ever reached it, and the operator's settings file is that
  agent's only beacon source. That is what the remaining settings-file findings
  are about, and why they now name what they cost — one agent's context tier —
  instead of reporting the harness blind.
- codex's turn predicate stays scrape-tier because there is no owned-event
  channel to translate, not because nobody has got to it. Wiring one would mean
  inventing the channel and then claiming the tier it implies, which is a
  fabricated witness; an empty row is the honest reading and is how it is meant
  to be read.
- The tiers may disagree without rot when something accounts for it. A compaction
  paints the busy marker while opening no turn, so a fresh compaction bracket
  under a painted marker and a closed turn is explained rather than filed — and
  both readers apply the exemption, the patrol log that witnesses when it happened
  and the vet row that answers what is true now. A rule that held on only one of
  them would let a postmortem and a report describe different systems.
- The bottom tier is a surface gang also writes to, which makes gang's own output
  part of the scrape contract. Nothing gang re-emits into a pane may read as a
  marker or tag: the rows that report a busy marker name the file it is declared
  in, and the pattern travels in the issues vet files, which are read somewhere
  other than a pane. Escaping the pattern instead does not generalise — a busy
  regex is allowed to be a bare single character, and any text quoting one matches
  it. The one deliberate exception is the pane dump behind a failing probe
  verdict, which stays verbatim because a capture with the marker edited out is an
  assertion rather than evidence; the row naming it says so.
- Residual, named rather than papered over: the suite's generic "nothing gang
  paints while probing reads as a harness busy marker" sweep reads the pane of a
  probe that PASSES, so any row only a FAILING probe prints is outside it. Those
  rows carry checks of their own instead. Closing the general case needs a second
  windowed probe driven to a failure, which is not free.
- The profile contract in docs/reference.md grows the per-predicate tier
  declaration when the first portfolio lands, not before (law 5).

## History

- **2026-08-01** — operator direction: claude-code rock-solid first, then
  codex; profiles are thin native plugins as well as scrapers. Hook surface
  verified against code.claude.com the same day; the summarizing fetch used
  first hallucinated a PreCompact token field, which is its own small argument
  for this ADR's epistemology — the raw source is the witness, summaries are a
  tier below.
- The measured numbers are recorded where they were measured, in
  profiles/claude-code.sh's comments; this page cites rather than restates
  them.
- **2026-08-01, review** — a per-harness evidence database was proposed and
  closed the same day: runtime questions are about now, and the fact layer with
  expiry already answers them; correlation was sharpened from fusion to
  conflict-surfacing (tiers pick witnesses), and the postmortem trail was fixed
  as log-shaped. The bullets above are that review made standing.
- **2026-08-01, build** — the wiring above was built against the live harness
  rather than reasoned from its docs, and three of these bullets are things the
  plan did not have until something was measured: the by-key merge, statusLine's
  missing exec form, and the trust gate's absence. Each is recorded with the
  scope of its evidence, because "observed once here" and "cannot happen" are
  different claims and this page's whole subject is which witness said what.
