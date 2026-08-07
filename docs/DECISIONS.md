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
named with its manual recovery, before pasting and after Enter alike, and an
unreadable verification capture in the queue check is ambiguity that fails
closed. A hard-stuck variant re-queues even typed input and the Up-loaded body
while reporting idle, and nothing drains; its recovery — drop and resume,
re-sending what the queue swallowed — belongs to the operator, never to gang.

The contract is scoped to verified harness renderings: the pin is the
composer hint observed on claude-code 2.1.223, an unobserved version narrows
the guarantee back to box-change verification rather than refusing sends, and
no session-record machinery is built unless a reworded hint supplies the
evidence to reopen that choice. A cleared staged record is evidence the
obstruction is gone, never retroactive proof the recorded body was delivered.

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
useful.

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

## Evidence is selected per predicate

For each fact, prefer the freshest owned event, then owned file state, then pane
scraping; witnesses do not vote. Expired or contradictory evidence is
indeterminate and surfaced, hooks only translate facts, and no background
processor reconciles them.

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
