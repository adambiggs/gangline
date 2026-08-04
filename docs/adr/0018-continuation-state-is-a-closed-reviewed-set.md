# ADR-0018: Continuation state is a closed, reviewed set

- **Status:** Accepted
- **Date:** 2026-08-03
- **Amends:** [ADR-0015](0015-a-context-is-renewed-by-cycling-not-by-summary.md)

## Operator impact

Renewal input is no longer arbitrary prose. The lead owns the live task ledger;
each dog owns a structured package that references only its assigned live tasks.
The practical workflow is in
[Operating a team](../operations.md#renewing-context), and the command-facing
boundary is in the
[reference](../reference.md#structured-continuation-input).

A legacy window's first structured attempt establishes a review floor and
refuses before destructive action. A later reviewed package may proceed, but
validation is still not delivery: only pane verification completes acceptance.
A leftover `pending` lineage requires explicit drop and ordinary hitch or adopt;
direct send and another renewal do not repair it.

Navigate to the [decision](#decision),
[task-ledger grammar](#the-task-ledger-wire-format),
[per-dog grammar](#the-per-dog-continuation-wire-format),
[review and transition rules](#review-freshness-and-transition-state),
[validation boundary](#what-validation-establishes), or
[acceptance criteria](#acceptance-criteria).

## Context

ADR-0015 made an authored handoff the trustworthy renewal channel and put an
outer byte budget at the verb. That guard prevents one failure: a payload too
large for the policy in force cannot consume the context that was emptied for
it. It does not establish that the bytes are continuation state.

### Measured observations

On 2026-08-03, `wc -c` measured the lead renewal package before and after a
current-state rewrite and reported different byte lengths. A heading inventory
showed that the predecessor carried mostly settled arguments, completed work,
prior team status and copied bounds, while the rewrite retained the work the
successor still owed. Those working files had deletion paths and were not a
durable public archive, so this dated observation records the result without
presenting either file as a retained receipt. The decision below rests on the
content discriminator, not on either byte-length reading alone.

Source inspection found that `cmd_cycle` and `cmd_compact` treat a resume as an
opaque non-empty body, measure it with `wc -c`, compare it with
`GANG_RESUME_MAX`, and record only the latest size and its growth from the prior
delivery. The integration suite guards the outer refusal, the operator override,
the folded-body success path and the growth report. No current branch parses a
continuation record, rejects history, establishes review freshness, or resolves
a shared task.

The role contract already says current state only, supersede in place, point at
durable sources, and keep epistemic labels. A package violating those rules is
accepted today when it is non-empty and within the outer guard. The rule is
documented at one site, satisfied by convention and enforced nowhere.

The narrow native freshness candidate was also measured and refuted on
2026-08-03. On the supported host,

```text
tmux display-message -p -t <window> '#{window_created}'
tmux display-message -p -t <window> '#{pane_created}'
```

returned empty values. `#{session_created}` exists at the wrong scope and
`#{window_activity}` changes while a seat runs. Neither can witness when the
current agent seat began. Gangline can record when it establishes a review
floor, but it must not rename that observation into a historical seat start.

The operator also requires one team-level current-task ledger. A task outcome,
owner and team state copied into every dog's package would create several stale
authorities immediately. Conversely, a built-in path would teach Gangline a
per-repository convention it has not needed: the existing delivery boundary can
read an explicitly named ordinary file and validate the referenced task there.

### Research limit

An independent research review completed on 2026-08-03 limits what a strict
format may claim. Structure can make authorship, provenance, refutation, expiry
and open obligations inspectable. It cannot prove that an authored claim is
true, that the package is complete, that files, processes or credentials
survived, or that accepted bytes reached the intended reader. Those are
separate guarantees with separate evidence.

The operator's strict-schema direction governs despite the report's
recommendation to evaluate the semantic contract first. The limitation still
binds the schema: validation may establish syntax, cross-reference integrity,
review ordering and transition accounting, never the truth or completeness of
the state being accounted for.

### Required discriminator

Acceptance must distinguish these worlds:

- the predecessor package, below the outer ceiling but structurally bloated
  with settled or copied team state;
- a concise package containing only current shared references and private live
  state; and
- a package whose legitimate current private state is unusually large.

The first must fail the closed schema. The other two may pass if they satisfy
the same contract and the existing outer guard. A smaller unexplained byte limit
would collapse the last two worlds and is not this decision.

## Decision

Continuation state is split into two strict authored artifacts:

1. a lead-owned task ledger containing the team's live tasks and note-pressure
   policy; and
2. one per-dog continuation package containing only that dog's live private
   notes and references to tasks in the ledger.

Both are line-oriented ordinary UTF-8 text without a byte-order mark. Line
endings are one LF byte; NUL, CR and invalid UTF-8 are refused. ASCII keys,
section names and delimiters are fixed. A scalar value is a non-empty single
line; fields described as identifier or locator lists use their stated comma
grammar instead. The first line is the magic line, immediately followed by the
complete ordered header block: no empty line appears after the magic line or
between header fields. Exactly one empty line appears between the final header
field and the first section heading. Exactly one empty line appears immediately
after every section heading. Exactly one empty line appears after every
heading-only task reference, before the next task reference or section. Exactly
one empty line appears after the final field of every ledger task or private-note
record, before the next record, section or end marker. No empty line appears
between a record heading and its first field or between record fields. No other
empty line is admitted. Thus an empty section is its heading, its one required
empty line, and the next section heading or end marker. Each file ends with its
exact end marker followed by one newline, not another empty line. The Task
References section is the exception to otherwise permitted empty sections: it
contains at least one live task heading. Comments, extra fields, duplicate
fields, duplicate identifiers, reordered fields, unknown sections, unknown
record types and trailing content are refused. There is no extension bucket: a
new field requires a new schema decision rather than becoming a place where
history can accumulate.

Raw capture is part of the boundary. Renewal input is first consumed into a
private byte-preserving staging file, and a ledger attempt is captured into a
separate byte-preserving staging file. Neither unvalidated source enters command
substitution or a shell variable: those representations cannot carry NUL and
would silently normalize the bytes before the parser could refuse them. The
outer byte guard measures the staged renewal bytes; structural preflight then
checks the staged files for the exact encoding and line rules above. Only a
validated text file may be imported into the existing envelope path. If the
attribution envelope's reserved-tag handling would alter a validated body, the
renewal refuses before destructive action instead of silently delivering
different bytes. A staging read error, byte-identity comparison failure or
encoding check that cannot run is could-not-determine and refuses, never a clean
absence result. Staging files are removed on every exit.

Task identifiers have the authored grammar
`task.<label>.<opaque-uuid>`. Private-note identifiers have the authored grammar
`note.<label>.<opaque-uuid>`. A label is a lowercase ASCII slug made from letters,
digits and internal dashes. The opaque component is a newly minted lowercase
UUID in the exact text shape
`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`; the validator
does not assign meaning to its version or variant bits. The full identifiers
remain within Gangline's existing letter/digit/dot/dash name vocabulary.

The validator checks the grammar and live uniqueness it can observe. The opaque
component makes reuse after removal practically avoidable; it does not let a
history-free validator prove that an identifier has never appeared before.
Non-reuse remains an authored obligation, not a fabricated all-time guarantee.

A locator is a non-empty sequence of visible ASCII bytes other than comma. A
literal percent byte appears only as an uppercase `%HH` escape. Comma, ASCII
whitespace, literal percent and every non-ASCII UTF-8 byte in the underlying
value are percent-encoded byte by byte. Thus locator lists split exactly on
literal commas without erasing paths or message references that contain spaces.
A byte that is permitted literally is not percent-encoded. A malformed or
non-canonical escape, raw whitespace or empty list member is refused. The exact
value `none` is reserved for fields that explicitly admit that sentinel and is
invalid wherever a locator is required.

### The task-ledger wire format

This is an illustrative valid first revision; the field and section order is the
contract, while the authored values are not defaults:

```text
GANGLINE-TASK-LEDGER 1
Revision: 1
Reviews: first
Writer: lead
Note-Pressure: 6

## Tasks

### task.continuation-contract.8d3b0d62-1a6f-4f8c-b2d7-3a5e9c7f1042
Reviewed-In: 1
Remove: acceptance-met-and-receipted
Outcome: Make continuation state closed, fresh, and authored.
Owner: adr9
State: active
Depends-On: none
Acceptance: command:test/integration.sh
Receipt: adr:0018
Blocker: none
Source: operator:2026-08-03

END GANGLINE-TASK-LEDGER
```

`Revision` and `Note-Pressure` are positive ASCII decimal whole numbers with no
leading zero. `Writer` is a claimed identity in Gangline's existing agent-name
grammar. The first valid ledger observed in a session pins that identity, and
every later revision uses the same bytes. The operating contract makes the
current lead the sole author; validation establishes only a stable attributed
identity and does not prove that its holder has the lead role. Revision one
alone says `Reviews: first`; every later revision says
`Reviews: <immediately-preceding-revision>`, where the value is the candidate
`Revision` minus one in the same canonical decimal grammar. Every retained or
new task carries the current revision in `Reviewed-In`, so a new ledger revision
makes its writer's review assertion visible on every live task. This is an
authored assertion, not proof that the review was careful or complete.

A task block has exactly the fields shown, in that order. `Reviewed-In` equals
the ledger revision, `Remove` is exactly `acceptance-met-and-receipted`, and
`Owner` is one claimed identity in Gangline's agent-name grammar. The closed
task states are `queued`, `active`, `blocked` and `review`; all are live states.
`blocked` requires a non-`none` `Blocker`, while the other states require
`Blocker: none`. `review` requires a non-`none` candidate `Receipt`. There is
deliberately no `done`, `completed` or archive state.

`Depends-On` is `none` or a comma-separated set of live task identifiers in the
same ledger, with no whitespace or duplicates. `Acceptance` and `Source` are
locators. `Receipt` and `Blocker` are `none` or one locator. These fields are
authored data, never commands Gangline executes. A task whose acceptance check
has passed and whose durable receipt has been recorded may be removed. Within an
observed session transition, removal requires a non-`none` receipt in that
task's last observed copy and the candidate revision must remove that identifier
from every dependency; the removal remains the writer's assertion that
acceptance passed. If revisions between the last observed bytes and the
candidate added a receipt and then removed the task, the candidate still refuses:
the receipt-bearing task must be observed before its removal can be admitted.
Completion is absence from the live ledger, not a permanent record in it. The
durable receipt, commit, session archive or ADR holds the history.

`Note-Pressure` is the current team policy applied separately to each dog's
private-note set. It is not a task limit and does not count task-reference
entries. Changing it is a ledger change: the ledger revision advances and every
live task is reviewed under that revision.

### Session-scoped ledger continuity

The first valid ledger observed in a tmux session becomes that session's
baseline, whatever its valid revision. A baseline at revision one must say
`Reviews: first`; a higher baseline revision must name its numeric predecessor.
Gangline pins the exact ledger path, declared writer, observed revision and exact
byte identity as one encoded compound value in a session-owned tmux option. The
representation must preserve those fields and their relationships; this decision
does not prescribe its implementation encoding. “No baseline” means that entire
compound option is absent. A present value that is partial, malformed or
relationally inconsistent is could-not-determine and refuses rather than being
treated as absent or re-baselined.

After the baseline, the same revision is accepted only when every captured byte
is identical. A changed ledger carries a revision numerically greater than the
session's last observed revision. Its `Reviews` field still names its authored
immediate numeric predecessor, whether or not Gangline observed that predecessor.
A lower revision is rollback, and different bytes under the last observed
revision are mutation; both refuse. The unchanged current revision may be read
by every dog without advancing merely because another dog referenced it.

A numeric gap records only monotonic observed progress. Gangline does not
validate, reconstruct or claim to have witnessed any intermediate ledger bytes.
It compares the later ledger directly with the last bytes it observed, including
the receipt rule for a task that disappeared and resolution of every package
reference at the current delivery event. A task introduced and removed entirely
inside an unseen gap was never observed and cannot be transition-validated.
Ordinary ledger edits therefore do not impose a hidden renewal after every
revision, and no watcher is added to manufacture intermediate observations.

Ledger observation is serialized in a short session-scoped critical section.
The invocation captures the file once, then holds that section from reading the
session's pinned facts through validating and, when appropriate, storing the
captured baseline or later revision. It releases the section before package
delivery. Two dogs racing different baselines, different bytes for one revision
or later revisions therefore cannot both validate against absent or stale
session facts: the first accepted observation changes the facts the second must
read. The second may succeed only if its captured ledger is still identical to
or a valid monotonic transition from the state that won. Failure to acquire the
section or read its state is could-not-determine and refuses. A failed compound
state write for a baseline or advance also refuses and does not report that the
ledger was observed. This synchronous invocation lock does not watch or
reconcile anything.

This continuity claim ends with the tmux session. A fresh session may baseline
any currently valid durable ledger, including a higher revision whose earlier
bytes Gangline never observed. Gangline stores no task history and cannot detect
a rollback that happened entirely between sessions. It reports only continuity
it witnessed inside the current session.

Byte identity is an equality instrument, not authentication or anti-tamper.
Senders and the ledger writer remain trusted under law 2. Gangline does not
infer who edited the file, whether the claimed writer holds the lead role, or
whether its claims are true.

### The per-dog continuation wire format

This package references the illustrative task above. Task references are
heading-only live references; private-note blocks carry the fixed fields shown:

```text
GANGLINE-CONTINUATION 1
Writer: adr9
Reviewed-At: 1785744000
Ledger: /workspace/gangline/.gang-tasks

## Task References

### task.continuation-contract.8d3b0d62-1a6f-4f8c-b2d7-3a5e9c7f1042

## Active Work

### note.resume-boundary.2b6a89f1-c8dc-46c2-90bd-95d1d6f8e4a7
Task: task.continuation-contract.8d3b0d62-1a6f-4f8c-b2d7-3a5e9c7f1042
Supersedes: none
Remove: checkpointed-or-no-longer-live
Evidence: verified
Provenance: file:bin/gang#cmd_cycle
Text: The resume is still opaque after the outer byte check.

## Next Actions

### note.draft-review.36c77717-3d7b-4a70-a998-b0bc5d23b42c
Task: task.continuation-contract.8d3b0d62-1a6f-4f8c-b2d7-3a5e9c7f1042
Supersedes: none
Remove: done-or-superseded
Evidence: claimed
Provenance: message:lead/design-direction-2026-08-03
Text: Obtain lead review of ADR-0018 before implementation.

## Local Blockers

## Binding Bounds

### note.no-coordinator.a3da3582-25ae-43e7-915d-04b98629f096
Task: all
Supersedes: none
Remove: reference-superseded
Evidence: verified
Provenance: file:CONSTITUTION.md#law-7
Text: ref:CONSTITUTION.md#law-7

## Dangerous Refutations

### note.native-window-created.01544ae3-f290-43e2-bda4-63db55654b91
Task: task.continuation-contract.8d3b0d62-1a6f-4f8c-b2d7-3a5e9c7f1042
Supersedes: none
Remove: contrary-evidence-accepted
Evidence: refuted
Provenance: command:tmux-display-message/window-created-2026-08-03
Text: tmux exposes a usable creation epoch for this supported window.

END GANGLINE-CONTINUATION
```

The header fields are exactly `Writer`, `Reviewed-At` and `Ledger`, in that
order. `Writer` is in Gangline's agent-name grammar and must equal the target dog
and the attributed sender of the renewal. `Reviewed-At` is a positive ASCII
decimal epoch with no leading zero. `Ledger` is an absolute path. The fixed
sections are `Task References`, `Active Work`, `Next Actions`, `Local Blockers`,
`Binding Bounds` and `Dangerous Refutations`, in that order.

A task-reference block consists only of its `### <task-id>` heading. It carries
no `Reviewed-At`, `Remove`, `Supersedes`, note-pressure field, or copied task
data. Outcome, owner, state, dependencies, acceptance, receipt and blocker remain
live authority in the ledger and are invalid fields in a package. Every reference
is checked against the captured ledger at the delivery event. At least one task
reference is required; a dog with no live assigned task has no valid continuation
package to deliver.

Every working, action and blocker note names one referenced task. A bound or
refutation may name one referenced task or `all`. Every referenced task must
exist in the ledger, name `Writer` as owner, and remain in a live state. Each
referenced task has at least one Next Actions or Local Blockers note; syntax can
make that obligation inspectable but cannot establish that the authored action
or blocker is complete.

Private-note identifiers are unique across all private-note sections, not merely
within one section. Task references are excluded from that identifier set and
from pressure accounting. All private notes are reviewed at the package header's
`Reviewed-At`; they do not carry a second per-record timestamp.

Every private-note block has exactly `Task`, `Supersedes`, `Remove`, `Evidence`,
`Provenance` and `Text`, in that order. `Remove` is the record's inspectable
expiry instruction and is fixed by section:

| Section | `Remove` value |
|---|---|
| Active Work | `checkpointed-or-no-longer-live` |
| Next Actions | `done-or-superseded` |
| Local Blockers | `resolved-or-task-removed` |
| Binding Bounds | `reference-superseded` |
| Dangerous Refutations | `contrary-evidence-accepted` |

The evidence vocabulary is `verified`, `claimed`, `inferred`, `unverified` and
`refuted`. `verified` says the writer directly checked the proposition by the
listed provenance; `claimed` says another named agent asserted it; `inferred`
says the writer derived it from the listed provenance rather than directly
observing the proposition; `unverified` claims neither check nor derivation; and
`refuted` says the proposition was disproved by the listed provenance. These are
authored categorical assertions, independent of provenance and review freshness.
Dangerous Refutations requires `Evidence: refuted`, and `refuted` is invalid in
every other section. A `claimed` note has at least one
`message:<claimed-agent>/<receipt>` locator, where the claimed agent is in
Gangline's name grammar and the receipt portion follows the common locator
encoding. The claimed agent differs from the package `Writer`, and the encoded
receipt after the separating slash is non-empty. No confidence field exists.

`Provenance` is a comma-separated set of locators with no duplicates. `Text` is
opaque UTF-8 scalar text except that a Binding Bounds value is exactly one
`ref:` locator rather than copied governance prose. The encoded locator after
the literal `ref:` prefix is non-empty and follows the locator grammar; `ref:`
alone is invalid. The validator establishes that required locators are present
and preserved through a declared synthesis. It does not establish that a locator
exists, supports the text, or has the authority the author attributes to it.

### Note identity, review and synthesis

A note identifier names one immutable point-in-time record. A note retained from
the immediately prior accepted package keeps the same section and fields. It is
reviewed under the candidate package's new header timestamp, but its record body
does not change. Changing its task, expiry, evidence, provenance or text creates
a new note identifier and removes or supersedes the old one.

`Supersedes` is `none` or a comma-separated set of prior private-note identifiers
with no whitespace or duplicates. A first package in an empty lineage says
`none` for every note. For a newly introduced synthesis, every listed identifier
must exist in the immediately prior accepted package, be absent from the
candidate, and be claimed by no other new note. A retained synthesis keeps its
already validated `Supersedes` value and otherwise remains immutable; Gangline
does not require its retired predecessors to remain in the next manifest.

A synthesis's `Provenance` contains every provenance locator carried by every
note it supersedes; it may add locators for the synthesis itself. If any
predecessor is a Dangerous Refutation or carries `Evidence: refuted`, the
synthesis is also a Dangerous Refutation with `Evidence: refuted`. These checks
preserve inspectable provenance and refutation status for the replacement the
author declared. Gangline cannot tell whether a plainly removed note's expiry
condition truly fired or whether an omitted fact should have been synthesized;
validation therefore does not prove completeness.

### Review freshness and transition state

When an ordinary hitch or adopt makes a new window an agent, Gangline writes a
review floor with `date +%s` and initializes a known-empty continuation
transition state. The floor is named `@gl_resume_review_floor`; the transition
state is named `@gl_resume_transition`. Both writes are required, and hitch or
adopt is not reported successful without both. The floor records the earliest
review Gangline can accept for that window, not when the underlying process or
historical seat began, and it dies with the window.

A legacy window with no review floor is not silently treated as a first
delivery. When both the floor and transition state are absent, its first
structured-package validation writes the current epoch as the floor, initializes
the transition state as known empty, refuses before any compaction, retirement
or package delivery, and tells the author to review after that floor and retry.
The refusal preserves the live context. No migration-only wire spelling is
introduced. A window carrying only one fact, or a floor beside invalid transition
state, is inconsistent rather than legacy and refuses without being initialized
again.

Before either renewal verb can type or retire anything, `Reviewed-At` is a whole
epoch that is:

- strictly later than the target window's review floor;
- no later than the host clock at validation; and
- strictly later than the accepted review in `@gl_resume_transition` when the
  agent lineage has a prior successfully delivered package.

This is an ordering check, not a maximum-age policy. There is no arbitrary
freshness duration. A package inherited unchanged into a newly hitched window is
older than that window's floor and is stale by construction. A package reused in
one window does not advance past the last successful review and is stale by
construction. Trust is still assumed: the boundary orders the author's explicit
review claim against facts Gangline wrote; it does not prove that the author
thought carefully or checked the environment.

Whole seconds create an explicit edge. Two successful reviews cannot carry the
same epoch. The later one is refused before destructive action and told that the
clock and authored `Reviewed-At` must advance. Gangline does not weaken `>` to
`>=`, accept a future stamp, invent non-portable precision, or sleep inside the
delivery on the author's behalf.

The transition state has exactly three statuses. `empty` carries no accepted
review or manifest. `accepted` carries the immediately prior accepted review and
private-note manifest. `pending` preserves that entire prior `empty` or
`accepted` state and also records the candidate review and manifest identity.
Missing, malformed or relationally inconsistent state refuses. A leftover
`pending` state also refuses; it is never interpreted as empty or accepted. Only
the synchronous acceptance that wrote a matching pending candidate while still
holding the target critical section may continue from it.

An explicit drop followed by an ordinary hitch or adopt is the only operation
that retires one lineage and initializes a genuinely new one. A `cycle` is a
replacement inside the existing lineage even though it internally hitches a new
window. A `cycle` without a candidate reads and validates the old window's
transition state under the target critical section. It carries `empty` or
`accepted` exactly onto the replacement; `accepted` retains its accepted review
and manifest. A `pending` state encountered by that later invocation is leftover
pending and refuses before retirement with the old window intact. The replacement
receives a new review floor, but the floor does not reset carried `empty` or
`accepted` state. Missing, malformed or inconsistent state likewise refuses
before retirement rather than becoming empty on a replacement.

A `compact` invocation without `--resume-stdin` remains the existing native
compaction on the same window. It neither creates a new lineage nor initializes,
resets or advances continuation transition state.

Final acceptance is serialized by a synchronous command-local critical section
for the target agent lineage. While holding it, an invocation repeats every
event-wise check against the current state, changes the state to `pending` before
any resume byte can land, performs and verifies delivery, then replaces it with
the candidate `accepted` state. A failure to acquire the section is
could-not-determine. Any failure after `pending` was written leaves it pending
and reports a loud resume failure, including a failure to store `accepted` after
bytes were verified. A later invocation therefore cannot mistake partial or
unrecorded delivery for first delivery. This lock exists only for the duration
of the accepting invocation; it is not a watcher or coordinator.

`cycle` with a candidate validates against the old window's floor and carried
transition state and marks that lineage `pending` before retirement. An admitted
candidate retires the old window, hitches the replacement with a new review
floor, and carries the same pending prior state and candidate identity onto it.
The replacement never passes through an externally usable `empty` state merely
because its window is new; a failure to install the carried state leaves missing
or malformed state that later validation refuses. Immediately before delivery,
the owning invocation recaptures the ledger and repeats its continuity,
task-reference and note-policy checks while still holding the target critical
section. Verified delivery changes the replacement to the candidate `accepted`
state. A later validation or delivery failure is loud and leaves the old window
retired rather than pretending the renewal did not begin. A successful candidate
cannot be reused there: the next review must be later than both the replacement's
new floor and the accepted review.

A leftover `pending` state has one operator recovery path: run `gang drop <dog>`
to retire that lineage, establish a genuinely new lineage through an ordinary
`gang hitch` or `gang adopt`, then author a review strictly later than the new
review floor. Cycling again, with or without a candidate, cannot perform that
recovery because every later invocation refuses leftover pending before
retirement. Direct `gang send` is ordinary attributed message delivery, not
continuation validation or acceptance, and cannot advance or cure transition
state. The implementation removes the current cycle-failure advice to retry a
handoff through direct send; failure output names the pending state and the
explicit new-lineage recovery instead. No automatic reset verb is added, and
recovery never interprets `pending` as `accepted` or `empty`.

`compact` performs preflight under the target critical section, releases it
before issuing compaction, then reacquires it and repeats every event-wise check
in its detached delivery path. The event also requires that the captured window
is still the active target in the same agent lineage; retirement, replacement or
loss of that identity refuses instead of delivering into a spent or different
window. It preserves the prior accepted review and manifest inside `pending`
until verified `inject` changes them to the candidate. Concurrent queued resumes
cannot spend one prior review: after serialization, the later acceptance sees
either the first candidate's advanced accepted state or its fail-closed pending
state and refuses through the existing resume-failure surface.

### Private-note pressure

Pressure counts private-note identifiers across Active Work, Next Actions, Local
Blockers, Binding Bounds and Dangerous Refutations. It never counts Task
References. The applicable `Note-Pressure` value comes from the captured ledger
at the delivery event, never from a field a dog can raise in its package.

A candidate presented to an initialized `empty` transition state is the first
package in that agent lineage. It establishes the prior-note manifest and is
admitted subject to structure, freshness, ledger references and the outer guard.
On a later transition whose prior accepted private-note set is below the current
ledger policy, the candidate may cross the policy without pressure refusal.
Crossing changes the rule for the following transition; the policy is a
watermark, not an admission cap.

Once the immediately prior accepted private-note set has reached or crossed the
current ledger policy, additions require at least as many removals. An addition
is a candidate note identifier absent from the prior set. A removal is a prior
note identifier absent from the candidate. Retained identifiers have no pressure
effect. This is set arithmetic, not FIFO. Gangline does not rank records, infer
importance from age, select a victim, summarize text or silently trim.

There is no second byte-growth rule inside `GANG_RESUME_MAX`. An immutable note
changed through one-for-one replacement may carry longer text or additional
provenance if the outer guard admits it. The strict transition makes that authored
replacement inspectable without pretending byte length measures importance.

The carried transition fact is the immediately prior package's accepted review
and private-note manifest, including each identifier, section, a byte-preserving
encoding of every immutable record, evidence status and provenance set. This is
window-scoped comparison state, not another authored source or retained task
history. It replaces only after verified delivery and dies with the window.
`cycle` without a candidate carries old `empty` or `accepted` lineage state and
refuses leftover `pending`; a candidate cycle carries only the matching pending
state owned by that accepting invocation and installs its accepted manifest on
the replacement only after delivery. `compact` advances state only after its
delayed delivery verifies. Neither verb can re-baseline an `accepted` or
`pending` lineage as empty. Gangline keeps no retired-note history, so UUID
non-reuse outside the observable prior manifest remains an authored obligation.

### Ledger reference and validation boundary

`Ledger` is an absolute path supplied by the authored package. Gangline reads
that file once into the byte-preserving capture for each validation attempt,
validates exactly those captured bytes, and checks task references immediately
before the existing delivery action. It does not know a built-in repository
path and does not learn the per-dog package's path; the package still arrives
only through `--resume-stdin`.

The first valid reference pins the ledger path for the tmux session. A different
spelling later is refused rather than starting a second task authority. A
missing, unreadable, partial or malformed ledger; an unknown task; an owner
mismatch; or a non-live state refuses before `cycle` retires a window or
`compact` issues its command. The package copies no ledger revision or task data,
so it cannot make a stale copy authoritative.

For delayed `compact` delivery, preflight is not certification. The detached path
captures and validates the ledger again at the event that would deliver the
package. A task removal, reassignment, policy change, or ledger continuity
failure that appeared while compaction ran refuses that delivery visibly.

The validator runs only when an existing renewal delivery invokes it. It does
not watch the ledger, schedule work, assign owners, transition states, notify
dogs, reconcile a running agent, or write either authored file. The current lead
is contractually the sole ledger writer; workers request changes through
attributed messages and read the file directly. The pinned `Writer` identity
makes that authorship claim stable and attributed but does not prove the role.
The ledger is a dumb authored artifact, not a law-7 coordinator.

### Day-one implementation and live consumer

The current team's first structured renewal is this decision's day-one consumer.
Before the validator-enabled `bin/gang` replaces the live script, the current
lead authors the live ledger at its intended durable path and each current dog
that will renew migrates its continuation file to the exact package grammar. The
staged structural parser must accept captures of those authored files before the
live boundary is enabled. A current legacy window's first live validation still
establishes its review floor and known-empty state and refuses without destructive
action; its author then reviews again after that floor and retries with the
migrated package. This ordered rollout is part of implementation acceptance, so
the parser cannot be enabled first and leave the running team without valid
authored ledger and package inputs.

Migration does not fabricate acceptance. Staged parsing, floor establishment and
the initial refusal deliver no package and install no accepted manifest. Only
the later renewal event may perform the pending-to-accepted transition after the
existing delivery verification.

### What validation establishes

A successful structural validation establishes only that:

- the captured ledger and package match the closed grammar;
- the package's `Writer`, task references, evidence categories, provenance
  locators, expiry instructions and open action/blocker obligations are
  inspectable in fixed fields;
- the review claim is ordered after Gangline's observable floor and prior
  accepted review;
- live task references resolve in the captured ledger; and
- observable ledger and private-note transitions satisfy their revision,
  immutability, supersession, refutation and pressure accounting.

It does not establish truth, completeness, environment continuity or delivery.
`Evidence: verified` is an authored category; the validator does not execute its
provenance locator. No grammar can prove an omitted fact was not required. No
ledger check proves that uncommitted files, processes, credentials or tool state
survived. Structural validation happens before the existing delivery mechanism;
only that mechanism's pane verification can establish delivery, and a delivery
failure remains a delivery failure even when both files were valid.

### The outer guard remains

`GANG_RESUME_MAX` remains the outer defense in depth decided by ADR-0015. The
byte check is cheap and precedes structural parsing. A structurally valid package
may fill any amount the outer policy admits. `Note-Pressure` is a transition
watermark, not a package-size or task-count cap, and there is no undocumented
inner byte threshold. Gangline carries every authored byte inside the existing
attributed envelope or refuses the body. It never summarizes, trims, rewrites or
auto-folds it.

### Artifact deletion paths

Completed tasks leave the live ledger after their durable receipt is recorded.
At team wrap, lead or the operator deletes the authored ledger after no live task
needs it. A dog deletes its own continuation file when its referenced tasks are
removed or the agent is released. Gangline does not delete files it did not
author.

The pinned ledger facts and its critical section die with the tmux session. The
review floor, transition state and target critical section die with their window
or lineage except for the explicit carried state from an old window to its
`cycle` replacement. No sweeper, archive job or reconciliation loop is
introduced.

### Refused alternatives

- automatic summary, trimming, folding or other mutation of authored bytes;
- append-only task history, completed-task sections, permanent `done` records or
  a catch-all notes section;
- permissive unknown fields, sections or record types;
- a built-in per-repository ledger path when an explicit path at the live
  delivery boundary is sufficient;
- a smaller byte ceiling or an inner byte-growth rule as the primary bloat
  control;
- FIFO eviction, age-as-importance, an automatically selected victim or a
  Gangline-authored synthesis;
- wall-clock maximum age, confidence scores, retrieval infrastructure, automatic
  reconciliation, polling, a supervisor, a database, a task scheduler or a
  watchdog;
- environment restoration or a claim that file validation restored it;
- copying task outcomes or team status into per-dog packages; and
- treating a missing or malformed review or transition fact as first delivery.

### Relationship to ADR-0015 and the Constitution

This amends ADR-0015's singular phrase "the authored handoff is the only
channel." The no-summary decision stands. Authored current state now has two
parts: shared task state in the ledger and private working state in the delivered
package. The fresh context still receives authored bytes and durable pointers,
never a statistical summary.

Law 1 is met by the existing shell delivery boundary, an explicitly supplied
ordinary path and tmux options; there is no harness branch, bus, daemon or
database. Law 3 remains separate and explicit: validation never reports
delivery, and only the existing verified delivery mechanism may do so. Law 5's
day-one consumer is the current team's first structured renewal, preceded by the
live ledger and package migration above; there is no claim that a ledger already
exists. Law 6 is met by the file and tmux-option deletion paths above. Law 7 is
not approached because no component runs between invocations or acts on task
state. Law 8 requires every malformed, stale, missing or unresolved case to
refuse rather than fall back.

The parser, transition manifest and pressure check are justified under law 9 by
the operator-directed requirement and the measured failure at the existing
renewal boundary. They run only when an operator or dog invokes that boundary.
The pressure rule constrains growth of the authored private-note identifier set
after the ledger watermark is reached; it neither caps package bytes nor lets
Gangline choose, summarize or author content. The existing outer byte guard
remains defense in depth, not a content-authoring mechanism.

The review floor, transition state, ledger revision and private-note manifest are
lifecycle facts, not authentication or generation fencing under law 2. They do
not distinguish trusted from untrusted senders, prove a ledger writer's role,
prevent a predecessor from acting, or secure a file against tampering. They make
a trusted author's reuse or mutation of observable continuation state visible at
the boundary that spends it.

## Consequences

- A current-state package can be large without being mistaken for history; its
  schema and authored review, not an unexplained smaller number, decide structural
  admission.
- A settled section, copied task field, stale review, missing task, owner
  mismatch or completed-state spelling fails before the renewal act.
- Reusing an unchanged ledger revision across dogs is normal; mutating it without
  advancing its review is not. A later observed revision may skip unseen authored
  revisions; the guarantee is monotonic observation, not reconstruction, and is
  honestly session-scoped.
- A legacy window keeps its live context on first validation, gains a review
  floor, and requires a later authored review rather than a fabricated historical
  seat-start fact.
- After note pressure begins, age still has no semantics. Authors remove state
  whose condition fired and synthesize state that remains load-bearing;
  Gangline verifies only observable transition accounting.
- The successor reads one shared task outcome and only its own local execution
  state. Removing a completed task removes live authority rather than moving it
  into an in-place archive.
- Implementation belongs at the common `cycle`/`compact` resume boundary. No
  profile changes, retrieval layer, environment restorer or background component
  are required.
- A valid structure can still contain a false or incomplete claim, point into an
  environment that no longer exists, or fail delivery. Those outcomes remain
  separately visible rather than being promoted to healthy continuation.

## Acceptance criteria

- The acceptance discriminator exercises the below-outer-guard bloated
  predecessor, a concise current package, and legitimately large current state.
  The first fails structure; both current-state packages pass the same schema and
  outer policy.
- Exact-wire fixtures cover valid ledger and package specimens plus reordered,
  duplicate, unknown, partial and trailing-field forms. They refuse BOM, CR,
  NUL, invalid UTF-8 and wrong blank-line placement, including a blank after the
  magic line, a blank inside the header block, a missing required boundary blank
  and an extra blank before the end marker. Every absence/negative assertion has
  a positive specimen proving the parser reached the field and a
  could-not-determine specimen proving an unreadable predicate cannot pass as
  absence.
- Raw-capture fixtures prove a staged NUL and staged invalid UTF-8 are refused
  before command substitution or a shell variable can normalize them. A valid
  UTF-8 control with planted multibyte text reaches parsing and verified delivery
  byte-for-byte; planted staging-read and encoding-check failures report
  could-not-determine, never valid absence. A reserved tag-shaped body is refused
  rather than rewritten, a nearby non-reserved spelling is delivered unchanged,
  and an unavailable byte comparison is could-not-determine. Every staging path
  is removed after success and each refusal.
- Ledger fixtures accept revision-one `Reviews: first`, accept a higher valid
  baseline in a fresh tmux session, accept unchanged bytes at the current
  revision, the next revision, and a harmless later revision across an unseen
  numeric gap. Every later revision names its own immediate numeric predecessor
  in `Reviews` even when that predecessor was not observed. They refuse a reused
  revision with changed content and rollback inside one session; no check claims
  to validate or reconstruct unseen intermediate bytes. Removal-across-gap
  fixtures accept a task removal only when its last observed copy has a durable
  receipt and refuse the same later ledger when that last observed receipt is
  `none`, even if an unseen revision could have added one. A concurrent control
  races conflicting candidates for the absent baseline, different bytes for one
  revision and monotonic later revisions. At most one candidate is admitted
  against each original state; the other invocation evaluates the state that won
  and succeeds only if it independently satisfies continuity from there. Planted
  partial, malformed and relationally inconsistent compound pins refuse rather
  than re-baseline. Planted baseline-write and advance-write failures refuse and
  produce no ledger-observed receipt.
- Task fixtures require the opaque UUID component, live uniqueness, exact task
  fields, a valid claimed `Writer` and `Owner`, stable baseline writer identity,
  live dependencies, a prior durable receipt before observed task removal, and
  removal from dependencies. Removal/recreation tests assert grammar and
  current-ledger uniqueness only; they do not present a history-free validator
  as proof of all-time non-reuse or the claimed writer's role.
- Package fixtures prove task references are heading-only, copy no task fields,
  carry no supersession or pressure state, contain at least one live reference,
  and resolve against the ledger at the delivery event. Locator fixtures accept
  encoded whitespace and commas and refuse their raw or malformed spellings.
  Binding Bounds fixtures require a non-empty canonical locator after `ref:`.
  Evidence fixtures distinguish claimed, inferred and verified provenance; a
  claimed control names a non-writer agent and non-empty receipt, while empty
  receipts and a claimed agent equal to package `Writer` refuse. Focused positive
  and could-not-determine controls accompany those predicates wherever the
  instrument can be unavailable.
- Review-floor fixtures prove new hitch/adopt writes the floor, a legacy window's
  first validation writes it plus known-empty transition state and refuses
  without destructive action, and a review after that floor succeeds. They
  refuse same-epoch reuse, future review claims, missing or malformed state, and
  leftover pending state as could-not-determine rather than first delivery. A
  planted post-delivery state-write failure remains pending and cannot be
  replayed as empty. Recovery controls prove only explicit lineage retirement
  followed by ordinary hitch/adopt establishes a new empty state and a new floor;
  no automatic reset interprets pending as accepted or empty.
- Pressure fixtures admit the first package in an initialized empty lineage and
  the first crossing, then refuse more additions than removals after the prior
  set reaches policy. They accept a genuine removal and a one-for-one authored
  replacement, show that task-reference changes do not affect pressure, and
  prove a `cycle` replacement carries prior pressure state instead of gaining a
  new baseline.
- Synthesis fixtures accept prior identifiers exactly once, require their
  absence, preserve every predecessor provenance locator and refutation status,
  and refuse unknown, duplicated, retained or multiply claimed predecessors.
  Retained-note controls prove an unchanged body is admitted and a changed body
  under the same note identifier is refused.
- Renewal controls prove `cycle` leaves the old window intact on every preflight
  refusal, retires it before attempting delivery for an admitted candidate, and
  reports a planted later delivery failure loudly with pending state on the
  replacement. A planted carried-state initialization failure leaves no usable
  empty baseline on that replacement. No-resume cycle controls prove `empty`
  remains empty and accepted review and manifest state survive on the
  replacement; leftover pending refuses before retirement with the old window
  intact. Cycle-delivery failure output never advertises direct `gang send` as a
  continuation retry and names the explicit new-lineage recovery. `compact`
  controls prove no native command is issued on
  refusal, delayed delivery rechecks the ledger, review and pressure state, and
  pending compact delivery preserves the prior accepted manifest. A compact
  without a candidate retains its existing same-window behavior and does not
  reset transition state. Concurrent same-target controls prove the command-local
  critical section lets only one candidate spend a prior accepted review, makes
  lock failure could-not-determine, and refuses a delayed compact resume after
  the captured window was cycled.
- Day-one rollout evidence shows the live ledger and current continuation files
  in exact package grammar pass the staged structural parser before the validator
  is enabled in live `bin/gang`. The current team's first structured renewal
  then exercises the legacy-floor refusal, later authored review, structural
  validation and verified acceptance in that order; no pre-delivery step is
  reported as delivery.
- Delivery instrumentation includes known-delivered and known-undelivered bodies;
  structural validation alone never prints a delivery receipt. Environment
  instrumentation includes a deliberately present and deliberately absent item;
  package validation never reports environment continuity for either.
- `GANG_RESUME_MAX` refusal, override, valid near-policy package and growth report
  remain meaningful. No test or prose treats note pressure as a second byte cap.
