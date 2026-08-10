# Decisions

This file records the durable choices that constrain Gangline. Each entry states
the rule and the reason it exists; implementation history belongs in git.

## Gangline is substrate, not coordination

Provide local harness lifecycle, transport, observation, and compaction
primitives. Do not manage roles, work allocation, or agent behaviour;
those policies belong to the operator and the native harnesses.
Coordination is declarative: express goals, roles, status, handoffs, and
lead heuristics through prose or native harness features. Gangline
defines no coordination schema, reporting protocol, or lead state
machine.

Role briefs are that prose, shipped. `roles/<name>.md` is text attached
to the one hitch that asked for it, replaceable file-for-file by an
operator file of the same name, on the same footing as the harness
knowledge a collar carries. Gangline validates a brief as prose and
delivers it — at system-prompt level where a collar declares the option,
at message level otherwise — and never parses it.

Attachment is a launch choice like a model or an effort level, and
Gangline builds no coordination state around it: no role or brief
identity is recorded in a window or session option, no later command
reads one, no output varies by it, and nothing checks whether an agent
behaved as its brief describes. Delivery necessarily leaves the brief
where delivery put it — in the agent's transcript or system prompt, in
the launched process's arguments, and in the launch string tmux retains
for the pane. Those are artifacts of having delivered it, readable by
anyone who can already read the pane; they are not a record Gangline
keeps or consults. A role is a thing an agent was told, not a thing
Gangline knows.

## tmux is the transport

Represent a team as one tmux session and each agent as a named window. Use the
tty for input, pane capture for observation, window options for ephemeral state,
and collars for harness-specific knowledge; this keeps agents observable and
controllable without a daemon, database, or private protocol.

## Harness driving is a seam, not a second product

Keep launch syntax, composer parsing, native state, submission, and native
commands behind the small collar contract. Gangline consumes that boundary
internally so core decisions can consume explicit observations and remain
deterministically unit-testable. Extract a general harness driver only when a
second non-benchmark consumer exists and can define the interface from real use.

## Messages are attributed and delivery is verified

Require a sender, wrap each message in a nonce-bound envelope, and confirm that
the target composer accepted and submitted it. Gangline is single-tenant and
does not claim authentication, but it never reports unverified delivery.

## MCP may be a face, not the transport

Add an MCP wrapper only for a real consumer that cannot use the CLI. MCP does not
universally start a turn in an idle native harness, while tty input does; agents
remain free to use MCP tools without Gangline mediating them.

## Native self-compaction stays in Gangline

Agents request their harness's native compaction at natural checkpoints. Keep
this beside the verified tty substrate it needs rather than creating a second
product or a duplicate injection path; defer the command to Stop when a harness
cannot submit it during its own turn.

## Queued is not delivered

A harness may accept the Enter and park the message in its own input queue —
claude's queue strand renders the parked body exactly like a submitted prompt
and empties the composer, so "the box changed" cannot prove entry into the
session. The one place the states differ is the composer itself, which reads
as the harness's queue hint; the collar declares that evidence
(`GANG_QUEUED_REGEX`), matched against the box reading only so a delivered
body quoting the hint can never trip it. Parked input is a failed delivery
named with the recovery `gang flush` performs, before pasting and after Enter
alike, and an unreadable verification capture in the queue check is ambiguity
that fails closed. A hard-stuck variant re-queues even typed input and the
recalled body while reporting idle, and nothing drains; its recovery — drop and
resume, re-sending what the queue swallowed — belongs to the operator, never to
gang.

The contract is scoped to verified harness renderings: the pin is the
composer hint observed on claude-code 2.1.223, an unobserved version narrows
the guarantee back to box-change verification rather than refusing sends, and
no session-record machinery is built unless a reworded hint supplies the
evidence to reopen that choice. A cleared staged record is evidence the
obstruction is gone, never retroactive proof the recorded body was delivered.

## A refused delivery is parked, a failed one is not

A refusal happens before any keystroke, so the body is still the sender's and
parking it loses nothing; a failure after a paste has an unknown fate, and a
second copy of a message that may have landed is worse than one loud failure.
Parking is the default and `--live-only` is the explicit probe. It drains only
on the target's own native Stop event and delivers through the ordinary verified
path — no poller, scheduler, or watcher. A collar whose harness announces no
turn boundary degrades to live-only and names the missing declaration rather
than holding a message nothing would drain. An entry is claimed out of
the spool before it is delivered, because ownership has to span the submission
AND the retirement: the pane lock is released inside the delivery, so anything
still live afterwards could be sent again by the next drain or the next
boundary. An entry whose delivery could not be verified, or whose drain died
mid-flight, is held and counted rather than re-sent; Gangline never sends a
message a second time on the chance the first did not arrive, and never holds
one without naming it. Supersession is the sender's explicit flag and reaches
only that sender's own earlier messages. A window's spool identity is minted at
hitch and adopt, where nothing can race it — minting it when a message needs
parking would let two senders mint two and strand one of their messages in a
directory nothing points at.

## A parked queue is recovered, not narrated

Gangline already owns every piece of evidence the manual recovery uses, so it
performs the recovery instead of printing the keystrokes. The collar declares
the key that loads the parked body, the record written when Gangline watched the
harness park it says which body must come back, and the loaded composer is read
back against that record, byte for byte and whole, before any Enter. Nothing is
normalized first: both sides are readings the same parser took from the same
composer, which is the comparison the delivery path already trusts exactly, and
every normalization discards content that some body means — trailing-space
trimming is line-oriented, so it cannot tell a Markdown hard line break from its
absence. Missing evidence, a key that loads
nothing, and a readback that does not match are all refusals with nothing
pressed. The post-Enter proof is one shared implementation, because two copies
of it would drift and the drifted one would report a submission nobody saw.

## An interrupt is a collar keystroke and a fact Gangline owns

The key that stops a turn is harness knowledge, declared per collar; an
undeclared collar refuses. Gangline drops the turn bracket rather than closing
it. Leaving it open strands a busy the harness will never end, but writing a
closed one is worse: a fresh closed bracket reads as definitive idle before any
evidence from the pane is consulted, so a harness that ignored the key would be
declared reachable and the next send would enter mid-turn. Gang saw a keystroke
leave; it did not see a turn end, and removing the fact is the only edit that
says so. How an interrupt is recorded belongs with whatever redesigns the turn
state, not here. Occupancy refuses the command: that key is often what a native
dialog reads as an answer.

## Occupancy is not authority

Refuse ordinary input whenever a harness-owned UI occupies the composer, but do
not infer who may clear it. UI recognition belongs in collars, unknown authority
fails closed, and Gangline answers only a collar-enumerated, whole-block
fingerprint whose safe row carries no authority. Re-read the selected row before
every key and the cleared composer after confirmation. Permission, approval,
authorization, access, elevation, grant, administration, denial, bypass,
credential, token, secret, privilege, and sandbox language is never an
auto-answer surface. Directory trust is the narrow exception when a collar
marks that one record as the directory already selected by `hitch -d`; the
prompt asks Gangline to repeat a choice the operator or lead already made. The
live Codex `safety-buffering-prompt` capture is the record for every other
dialog: installed-binary string extraction undercounted its options and omitted
`Learn more`.

At hitch, positive evidence of a prompt without a composer is an operator
outcome before it is a launch failure: report `gang attach` once and spend the
remaining original boot bound waiting. A blank pane is only startup, not prompt
evidence. Unknown prompts remain manual, but clearing one leaves a healthy
agent whose startup contract can be delivered with `gang send`; only a composer
that never appears calls for drop-and-re-hitch.

## Binary identity is a window witness

Stamp hitch and adopt windows with the checksum and size of the invoked script,
including in checkouts: executable bytes, not repository state, determine live
skew. Compute and compare that witness only when stamping, status, or roster
needs it; unavailability is visible but never blocks lifecycle commands. Skew
does not justify a patrol or an attempt to retrofit launch-time context,
collars, or hooks.
When that script lives in a git checkout and differs from HEAD, warn on every
operator command dispatch with the exact path and HEAD; the native hook endpoint
stays silent except for a crossed light. Keep live-by-path installation, because
a merge can intentionally upgrade a running team.

## Context lights are optional and minimal

Keep context signaling off by default. When enabled, expose exactly yellow and
red at intentionally high absolute token thresholds, notify once per context
epoch, and leave the decision to compact with the agent. Place both thresholds
below the observed native automatic-compaction boundary while preserving most
of that effective window; larger windows do not make degraded context more
useful. Expose the same computation as an on-demand query that reads whether or
not lights are enabled, because signalling and asking are different acts.

## Effort is the collar's word

A reasoning-effort choice rides hitch beside the model choice, but the collar
owns both the spelling and the vocabulary: the option is declared whole,
including its separator, and joined to the level with no space; the levels come
from a collar command that prints them. Printing keeps "not a level" distinct
from "could not determine" — an exit status merges them and blames the operator
for a harness that is merely absent. A bad level is refused at hitch, the last
cheap place: harnesses either warn and run at a default nobody chose or open a
window whose first turn the provider refuses. The append sits below the resume
swap so both launch forms carry the effort by construction.

## Configuration is parsed, never sourced

Mirror the existing environment names in one strict scalar file, keep a set
environment variable authoritative, and refuse unknown or duplicated keys.
Sourcing would execute operator text on every command and native hook; silently
ignoring a typo would claim a setting Gangline did not apply.

## Doctrine is the operator's, and every hitch carries it

Core owns one optional doctrine slot and ships no content. Deliver its validated
prose in every hitch contract because Gangline cannot observe which caller is an
operator; scope belongs in the operator's words, not an inferred role. Adoption
still injects no startup text.

## A hitch states its model and its effort

Require the startup contract to tell agents to choose both deliberately when
hitching teammates. Gangline requires the choice, never its content: model names,
effort levels, and policy remain the collar's and operator's words.

## Evidence is selected per predicate

For each fact, prefer the freshest owned event, then owned file state, then pane
scraping; witnesses do not vote. Expired or contradictory evidence is
unknown and surfaced, hooks only translate facts, and no background
processor reconciles them. Unknown never vetoes an action that fresher
direct evidence proves safe — the action's own verification carries the
residual risk — and state the new evidence refutes is retired at that moment,
never by a patrol. Retirement applies only to state gang alone writes: the
turn bracket belongs to lock-free native hooks and tmux offers no atomic
compare-and-delete, so no reader — delivery or status — writes it at all; a
malformed value is reported as unreadable, never repaired, and eligibility
is re-derived per action. A hookless window without mid-turn input whose
pane keeps a frozen busy marker therefore stays refused until the marker
scrolls off or the agent is renewed — fail-closed by intent.

A turn fact nobody will ever edit decays instead of standing unknown for
the life of the window. An interruption typed straight into the pane is reported
by no harness, and `gang interrupt`, which drops the bracket, is the only
command that touches it; the bracket a raw keystroke abandons stays open
forever. Once it passes its bound the tiers
beneath it answer: a quiet pty, a stable pane, and the harness's own input box
on screen and provably empty are the positive readiness evidence idle is defined
by, so the state is idle rather than a permanent could-not-determine. Decay
requires every leg, measured rather than assumed — a collar that does not
declare quiet-at-rest reports inactive by abstention, and an abstention is not
a witness — and applies only to a readable open bracket past its bound, since
an unreadable or future-stamped one is unknown, not abandoned. Those legs are
read one after another, so a decay could otherwise describe a state no instant
held; the pty clock and the screen are read before the first tier and after the
last, and a decay assembled while either moved is refused. That pair is also
what lets the snapshot reader decay at all, being immediate where a stability
check costs a churn wait. It widens no guard: inside its bound the bracket still
outranks the tiers beneath it, and delivery already reached a provably empty box
through the unknown fall-through.

Movement seen during a decision is not indeterminacy, and delivery must not
consume it as such. Every other road to could-not-determine is an absence — a
witness too old, a tier that never answered — and the fall-through exists
precisely so an absence cannot veto what a fresh reading proves safe. A pane
observed being written to is presence: a harness paints the opening of a turn
with its composer still empty, so an empty box read out of a moving screen is
one frame of something in motion rather than a settled reading, and typing into
it lands in work that began while gang was deciding. That verdict carries its
reason so delivery can refuse on it alone.

## A refusal names what was read, not just that it refused

An obstruction gang can classify is classified: draft, staged, unattributed,
parked, whole-pane, cleared, unreadable, each with the look or recovery that
settles it. The naming look is taken after the decision and decides nothing, so
it must report what it actually saw: a box that emptied in that gap read
successfully and is cleared, never a harness that could not be read.
Human authorship is never the leftover case — the delivery legs that record a
paste whose fate gang never saw record no rendering to match, so a box beside
one of those records is gang's own text as plausibly as anybody's and is named
unattributed rather than blamed on a person. The
alternative is what happened — operators diagnosing a blocked box by eye from a
raw capture, where a dim suggested-prompt placeholder is indistinguishable from
a half-written line, and getting it wrong publicly. Classification uses only
evidence gang already owns: the collar's styled reading, its declared queue
evidence, and the box rendering gang recorded when it staged its own body. A
placeholder is deliberately not a class — the styled reading strips it, so it
never blocks a delivery and cannot be what a refusal saw. Inspection pointers
name the styled reading rather than the raw pane, because the raw pane is where
the question was got wrong.

## Server loss is a relaunch, not restoration

Do not persist a Gangline roster. `--resume` asks a collar's verified,
explicit-id native command for the session stamped in `@gl_session_id`, read
from a surviving registered window or quoted from `gang drop`'s parting output,
and refuses without one; the operator supplies which agents and ids to relaunch.

## A team curfew is an optional declaration

Let the operator declare one wall-clock curfew for the team. Derive exactly two
relative, advisory edges from that span: yellow halfway through and red after
four-fifths. Do not invent a default, enforce the deadline, allocate per-agent
budgets, or run a patrol; the substrate exposes operator intent and each agent
decides how to respond.

## A stall light is a harness's own witness, forwarded

Where a harness itself reports that it is waiting on a person, deliver that
fact as an ordinary attributed message to one optional operator-declared
target. Nothing polls, nothing infers a stall from a quiet pane, and nothing
infers a lead: the target is a declaration in the shape of the team curfew,
and with none declared there are no stall lights. A repeated report of the
same kind inside one stall is one note, cleared by the harness's own next move.
A harness that reports nothing gets no substitute, and a delivery that fails
is recorded on the window for status to surface rather than killing the hook —
a record retired only by a later note accepted live or parked, because a light
that is still broken has to keep saying so.

## Benchmarks consume Gangline but do not shape it

Every core change must have a general operator or agent consumer and a rationale
that survives removing the benchmark's name. Benchmark-specific adaptation stays
outside the core and hidden tests or reference solutions are never read.

## Unpublished renames are complete

Before publication, replace an abandoned name everywhere without compatibility
breadcrumbs or rename history. After publication, preserve compatibility and use
normal deprecation because the old name has become an external fact.

## A published name deprecates for one major cycle

A name the 1.0 release renamed keeps its old spelling as an accepted alias
through 1.x and is removed in 2.0. Every alias announces itself on stderr,
naming the new spelling and the removal release, so a working setup keeps
working and says what to change. The promise is that it is announced, not that
it is announced exactly once: a setting read once per process says so once,
while a collar declaration read per window says so per window, and deduplicating
that would need durable state worth less than the line it would suppress. Old
and new spellings of one setting are never both honored: two names for one
setting is a refusal naming both origins, because preferring either silently
would claim a configuration the operator did not write. Announcement is for
names the operator wrote and still holds. Gangline's own window and session
state is migrated in place and silently, by ordinary commands and by the native
hook endpoint alike: there is nothing there for an operator to change, and
carrying the news across the process that erased the evidence would take durable
marker state no consumer wants. Two values that genuinely disagree still
refuse.

## A window name carries last-witnessed state

A gang-managed window wraps its agent name in the glyph of the state Gangline
last witnessed, so a tmux status bar shows the team at a glance without asking
anything. It is written at the observation points and hook events that already
determine state, never by a patrol, so between observations it can be stale and
`gang roster` remains the live-computed truth. Addressing is always the bare
name: every lookup strips the glyph, and two windows that strip to one name are
a refusal rather than a guess, because a silently wrong target is worse than no
target. tmux appends its own flags after the name; that rendering is documented,
not fought.

## Instale data is refused from documentation

A data point that is stale the instant it is recorded does not belong in standing
documentation. Do not record changing counts, versions, sizes, timings, or
tallies; point to the command that measures them, and retain a measurement only
when it is dated evidence without which a decision's rationale would fail.

## PII prevention belongs to Snubline

Gangline carries no scanner, no scanning CI, and no scanning tests. Snubline owns
the patterns, the fixtures, and the gate. A second copy is a second thing to keep
correct, and the copy that lags is the one that reports clean. Scanning stays
prospective — it reads what a push would add and never rewrites history, and any
history rewrite remains a separate explicit decision. Keep operator-specific
denylist values untracked; `.gitignore` covers Snubline's `.pii-scan-denylist`,
which Snubline itself refuses to let anyone commit.

The cost is accepted deliberately. Snubline is not published, so Gangline's
public CI cannot run it, and a contributor without a Snubline installation pushes
unscanned. Restoring that coverage means publishing Snubline, not re-vendoring a
scanner here.

## Host-global contribution safety belongs to Snubline

Snubline owns the host-global dispatcher, deterministic PII scanner, installer,
and their behavioral suite. Gangline remains a consumer: its repository-local
pre-push hook delegates to an executable global hook before running its own lint
and commit gates, so opting into `.githooks` does not shadow the operator's
machine-wide gate. The dispatcher suppresses re-entry only when resolved hook
identities agree; the local hook never trusts an ambient recursion variable. An
absent global hook is a silent no-op. Gangline tracks no scanner of its own;
outward delegation is the whole of its participation.

## Mandatory tests are immediate

The mandatory suite normally completes in seconds and must remain under five
minutes. Tests do not sleep, poll, or test timeout behaviour; use immediate state,
event barriers, or fake clocks, and test real harnesses only in separate disposable
tmux sessions.

## The irreversible verb is the one that demands an argument

`gang down` requires the session it ends and refuses from inside it. Every other
argument-taking command answers a bare invocation with its usage; `down` did not,
so the reflex that reads the manual — run it bare and see what it wants —
executed the teardown instead. A gesture that asks what a command does must never
be the gesture that performs it, and the command with no undo is the one that
must cost an argument rather than the one that costs none.

## Teardown archives mail before deleting its spool

A composed message is not teardown state. `gang drop` and `gang down` move every
waiting or held entry into a human-readable archive before deleting its window's
spool, and refuse to end anything if that archive cannot be written. The archive
is created only when mail exists and its printed path is the handoff to a person.

## A queue drains as one message

When a turn boundary drains a spool, every waiting entry is delivered as one
chronological bundle under one pane lock, envelopes intact. Delivering them
one at a time submits the first, which starts a turn, which refuses the second —
so a target that is never idle for long accumulates exactly the messages that
would have corrected it. One paste and one Enter cannot race the turn they create.

## A stop carries its reason, and never through the queue

`gang interrupt -m` stops a turn and delivers its reason at the boundary that
stop creates, under one continuous pane lock. It is never spooled. `--supersede`
retires the sender's own waiting entries and stamps the replacement now, which
sorts it behind every other sender's — so a stop sent that way arrives after the
work it was meant to stop. Priority is not a property a queue position can carry;
it is a property of not being in the queue.

## A guard witnesses the artifact, and witnesses it in order

Assert the thing a defect actually produces — the text left on the pane, the
body recorded in the window option — and not a status that merely travels with
it. A refusal and a delivery that failed after typing both exit non-zero, so an
exit-status assertion is green on the very defect it was written to catch, and
a passing count then reports ground nobody covered. The same trap catches the
fixture that cannot produce the artifact at all: a pane with no busy marker
cannot paint a turn, so a probe built on one proves nothing about typing into
live work, however carefully it is run.

Witnessing the right artifact is half of it. A guard is proven by reverting its
fix and watching the evidence come back on screen — but red once is not proof;
red in a defined order is. A keystroke sent is not a keystroke observed: the
send returns when the key is enqueued, so a capture taken afterwards reads
whichever moment it happens to catch, and a run where the harness had not yet
acted goes green for no reason anyone chose. Order the observation behind the
same input path the work travels, so that when the probe reports, what is being
tested has either happened or never will. A guard that is right about what to
look at and undefined about when is still luck.

## The contract is a file agents read, not a paste they receive

The standing terms live in `CONTRACT.md` and the startup contract points at it.
Prose that is pasted is bounded by what a composer renders and by pane geometry,
it is unaddressable inside one long line, and it is gone once a compaction
summarises it. A path has none of those properties: the file can be structured,
it can define its own vocabulary, and re-reading it is the recovery. Gangline
resolves and validates it before opening a window, operator-first like a role
brief, and refuses the hitch when it is missing or is not prose — an agent sent
to read a contract that is not there would find that out alone, in a pane, with
nobody to tell.

The cost is real and accepted: a pasted rule is unconditional, and a pointer is
not. What the contract requires is unchanged, so the guards that recorded those
requirements now prove them against the file, and every hitch is held to naming
it. Doctrine and role briefs stay pasted; they are per-hitch, and the pointer
buys them nothing.

## The contract rides the system prompt where a collar has one

A pointer is conditional on the agent following it, and the contract binds
every agent whether or not it does. Where a collar declares
`GANG_ROLE_PROMPT_OPT`, pass the contract through it: the harness resends that
prompt every turn, so the terms are unconditional and survive a compaction
without a re-read. The startup contract then names the file instead of ordering
it read — an agent already holding the contract spends a tool call and gains
nothing. Collars without the option keep the pointer, which is still better
than a paste bounded by pane geometry.

A role is opt-in and the contract is not, so a role-less hitch still carries a
system prompt. Both share one option rather than passing it twice, because a
collar declares a single spelling and repeating it would guess at whether the
harness concatenates or keeps the last.

The startup contract does not repeat what the system prompt says. An agent that
reads the same assurance twice can still only act on it once, and Gangline
cannot verify that a launch option reached the model, so that line is spent on
the recovery instead: where the contract lives, and an instruction to report a
missing attachment rather than improvise.
