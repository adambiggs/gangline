# Decisions

This file records the durable choices that constrain Gangline. Each entry states
the rule and the reason it exists; implementation history belongs in git.

## Gangline is substrate, not coordination

Provide local harness lifecycle, transport, observation, and compaction
primitives. Do not manage roles, work allocation, or agent behaviour;
those policies belong to the operator and the native harnesses.
Coordination is declarative: express goals, roles, status, handoffs, and lead
heuristics through prose or native harness features. Gangline defines no
coordination schema, reporting protocol, or lead state machine.

## tmux is the transport

Represent a team as one tmux session and each agent as a named window. Use the
tty for input, pane capture for observation, window options for ephemeral state,
and profiles for harness-specific knowledge; this keeps agents observable and
controllable without a daemon, database, or private protocol.

## Harness driving is a seam, not a second product

Keep launch syntax, composer parsing, native state, submission, and native
commands behind the small profile contract. Gangline consumes that boundary
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
as the harness's queue hint; the profile declares that evidence
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

## A refused delivery may be spooled, a failed one may not

A refusal happens before any keystroke, so the body is still the sender's and
parking it loses nothing; a failure after a paste has an unknown fate, and a
second copy of a message that may have landed is worse than one loud failure.
Spooling is opt-in per send, drains only on the target's own native Stop event,
and delivers through the ordinary verified path — no poller, scheduler, or
watcher, and a profile whose harness announces no turn boundary refuses the flag
rather than holding a message nothing would drain. An entry is claimed out of
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
performs the recovery instead of printing the keystrokes. The profile declares
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

## An interrupt is a profile keystroke and a fact Gangline owns

The key that stops a turn is harness knowledge, declared per profile; an
undeclared profile refuses. Gangline drops the turn bracket rather than closing
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
not infer who may clear it. UI recognition belongs in profiles, unknown authority
fails closed, and Gangline does not autonomously answer native dialogs.

At hitch, positive evidence of a prompt without a composer is an operator
outcome before it is a launch failure: report `gang attach` once and spend the
remaining original boot bound waiting. A blank pane is only startup, not prompt
evidence. Never answer the native prompt; timeout retains the manual
drop-and-re-hitch recovery.

## Binary identity is a window witness

Stamp hitch and adopt windows with the checksum and size of the invoked script,
including in checkouts: executable bytes, not repository state, determine live
skew. Compute and compare that witness only when stamping, status, or roster
needs it; unavailability is visible but never blocks lifecycle commands. Skew
does not justify a patrol or an attempt to retrofit launch-time context,
profiles, or hooks.

## Context lights are optional and minimal

Keep context signaling off by default. When enabled, expose exactly yellow and
red at intentionally high absolute token thresholds, notify once per context
epoch, and leave the decision to compact with the agent. Place both thresholds
below the observed native automatic-compaction boundary while preserving most
of that effective window; larger windows do not make degraded context more
useful. Expose the same computation as an on-demand query that reads whether or
not lights are enabled, because signalling and asking are different acts.

## Effort is the profile's word

A reasoning-effort choice rides hitch beside the model choice, but the profile
owns both the spelling and the vocabulary: the option is declared whole,
including its separator, and joined to the level with no space; the levels come
from a profile command that prints them. Printing keeps "not a level" distinct
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
effort levels, and policy remain the profile's and operator's words.

## Evidence is selected per predicate

For each fact, prefer the freshest owned event, then owned file state, then pane
scraping; witnesses do not vote. Expired or contradictory evidence is
indeterminate and surfaced, hooks only translate facts, and no background
processor reconciles them. Indeterminate never vetoes an action that fresher
direct evidence proves safe — the action's own verification carries the
residual risk — and state the new evidence refutes is retired at that moment,
never by a patrol. Retirement applies only to state gang alone writes: the
turn bracket belongs to lock-free native hooks and tmux offers no atomic
compare-and-delete, so no reader — delivery or status — writes it at all; a
malformed value is reported as unreadable, never repaired, and eligibility
is re-derived per action. A hookless window without mid-turn input whose
pane keeps a frozen busy marker therefore stays refused until the marker
scrolls off or the agent is renewed — fail-closed by intent.

A turn fact nobody will ever edit decays instead of standing indeterminate for
the life of the window. An interruption typed straight into the pane is reported
by no harness, and `gang interrupt`, which drops the bracket, is the only
command that touches it; the bracket a raw keystroke abandons stays open
forever. Once it passes its bound the tiers
beneath it answer: a quiet pty, a stable pane, and the harness's own input box
on screen and provably empty are the positive readiness evidence idle is defined
by, so the state is idle rather than a permanent could-not-determine. Decay
requires every leg, measured rather than assumed — a profile that does not
declare quiet-at-rest reports inactive by abstention, and an abstention is not
a witness — and applies only to a readable open bracket past its bound, since
an unreadable or future-stamped one is unknown, not abandoned. Those legs are
read one after another, so a decay could otherwise describe a state no instant
held; the pty clock and the screen are read before the first tier and after the
last, and a decay assembled while either moved is refused. That pair is also
what lets the snapshot reader decay at all, being immediate where a stability
check costs a churn wait. It widens no guard: inside its bound the bracket still
outranks the tiers beneath it, and delivery already reached a provably empty box
through the indeterminate fall-through.

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
evidence gang already owns: the profile's styled reading, its declared queue
evidence, and the box rendering gang recorded when it staged its own body. A
placeholder is deliberately not a class — the styled reading strips it, so it
never blocks a delivery and cannot be what a refusal saw. Inspection pointers
name the styled reading rather than the raw pane, because the raw pane is where
the question was got wrong.

## Server loss is a relaunch, not restoration

Do not persist a Gangline roster. `--resume` asks a profile's verified,
directory-scoped native command for the latest conversation and fails when the
profile cannot make that request safely; the operator supplies which agents to
relaunch.

## A team cutoff is an optional declaration

Let the operator declare one wall-clock cutoff for the team. Derive exactly two
relative, advisory edges from that span: yellow halfway through and red after
four-fifths. Do not invent a default, enforce the deadline, allocate per-agent
budgets, or run a patrol; the substrate exposes operator intent and each agent
decides how to respond.

## Benchmarks consume Gangline but do not shape it

Every core change must have a general operator or agent consumer and a rationale
that survives removing the benchmark's name. Benchmark-specific adaptation stays
outside the core and hidden tests or reference solutions are never read.

## Unpublished renames are complete

Before publication, replace an abandoned name everywhere without compatibility
breadcrumbs or rename history. After publication, preserve compatibility and use
normal deprecation because the old name has become an external fact.

## Instale data is refused from documentation

A data point that is stale the instant it is recorded does not belong in standing
documentation. Do not record changing counts, versions, sizes, timings, or
tallies; point to the command that measures them, and retain a measurement only
when it is dated evidence without which a decision's rationale would fail.

## PII prevention is prospective

Use one scanner for repository content at local and CI gates, and scan issue or
pull-request prose before sending it. Keep operator-specific denylist values
untracked, prove scanner patterns against fixtures, and treat any history rewrite
as a separate explicit decision.

## Mandatory tests are immediate

The mandatory suite normally completes in seconds and must remain under five
minutes. Tests do not sleep, poll, or test timeout behaviour; use immediate state,
event barriers, or fake clocks, and test real harnesses only in separate disposable
tmux sessions.

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
