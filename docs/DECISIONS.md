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

Attachment is a launch choice like a model or an effort level. `gang up`
chooses the shipped `lead` role because it creates the team's lead; an explicit
role replaces that default. `gang hitch` attaches only the role its caller
names and never infers one from the agent name.

Gangline builds no coordination state around attachment: no role or brief
identity is recorded in a window or session option, no later command reads one,
no output varies by it, and nothing checks whether an agent behaved as its brief
describes. Delivery necessarily leaves the brief where delivery put it — in the
agent's transcript or system prompt, in the launched process's arguments, and
in the launch string tmux retains for the pane. Those are artifacts of having
delivered it, readable by anyone who can already read the pane; they are not a
record Gangline keeps or consults.

## tmux is the transport

Represent a team as one tmux session and each agent as a named window. Use the
tty for input, pane capture for observation, window options for ephemeral state,
and collars for harness-specific knowledge; this keeps agents observable and
controllable without a daemon, database, or private protocol.

## UTF-8 is a host prerequisite

Refuse startup when neither the environment nor the host locale inventory can
establish UTF-8. Gangline writes Unicode protocol glyphs into tmux state and
reads them back; continuing under an unverified character contract would report
a degraded transport as healthy rather than provide a supported fallback.

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

## A turn event settles an open compaction bracket

PreCompact is not reliably paired: a harness that REFUSES the compaction raises it
and never raises PostCompact. An unpaired opening held the agent busy until the
bracket aged out, and reported busy over an idle harness for that whole window.

A turn event closes it, because taking a turn and compacting are mutually
exclusive. Input typed into a compaction is parked, and the park raises nothing
until it drains, so a turn witnesses a harness that is not compacting whether or
not the compaction it followed ever finished. Settling only ever closes a bracket
that is already open; a turn on a window that never had one writes nothing.

## A compaction is followed by a continuation turn

Gang types a continuation behind every compaction command, so an agent lands with
a turn to take instead of an empty composer. It rides the composer, not the
PreCompact/PostCompact pair: a refused compaction raises PreCompact and never
PostCompact, so a continuation keyed on the bracket is lost exactly when the agent
still holds its full context. A compacting harness parks the continuation and
submits it when the compaction ends; a harness that refused takes it immediately.
Both land, on every collar declaring a compact command, which is why this does not
split a team into collars that resume and collars that sit.

The agent supplies the turn with `--resume`, or a default fires. The default is
orientation, never direction — "continue working" would make a finished agent
invent work.

Parking here is a landing rather than a failed delivery, which is the one
exception to *Queued is not delivered*: gang is using the harness's own queue as
the delivery timer, and claims entry only for a message it told a peer was sent.

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

## Overlay occupancy needs a structural second question

The auto-mode environment rendering is pinned to the live claude-code 2.1.239
frame in `test/fixtures/claude-auto-mode-environment.txt`. Its pure top band
must touch the exact title and its exact guide must be the last nonblank row
before the composer opens, so the same prose inside a message cannot hide that
message's box. A reworded or rearranged dialog narrows classification back to
ordinary composer evidence; either change reopens the choice between updating
the exact pin and adding a version gate.

## A refused delivery is parked, a failed one is not

A refusal happens before any keystroke, so the body is still the sender's and
parking it loses nothing; a failure after a paste has an unknown fate, and a
second copy of a message that may have landed is worse than one loud failure.
Parking is the default and `--live-only` is the explicit probe. Every collar
drains on the target's native Stop event; a `steer` collar may also drain at
PostToolUse when its composer is free, after attribution has committed the
entry. There is no poller, scheduler, or watcher. A collar whose harness
announces no delivery boundary degrades to live-only and names the missing
declaration rather than holding a message nothing would drain. An entry is claimed out of
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

A drain never gets a weaker delivery predicate. A readable obstruction after a
boundary is an ordinary refusal and remains queued; a boundary that still exposes
no readable composer records a drain failure while leaving every entry unclaimed.
Silently spending that unreadability as another healthy retry would strand the
same queue at every later boundary while status claimed only that it was waiting.

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

Refuse ordinary input whenever a harness-owned UI occupies the composer, and do
not infer who may clear it. Occupancy recognition belongs in collars, as
`GANG_OCCUPIED_REGEX`, and unknown authority fails closed.

Gangline answered a collar-enumerated whole-block fingerprint through 1.x, with
authority language mechanically forbidden and directory trust as the one narrow
exception. 2.0 removes that: the registry, its per-dialog fingerprints, the key
driving, and both collars' records are gone. It bought dismissing one Codex wait
screen and repeating a directory-trust choice `hitch -d` had already made, and
cost the most version-fragile and security-sensitive TUI machinery in core —
per-build strings that rot into a silent fallback while we believe we have
coverage, which is the same argument that already refused a name-only registry.
What remains is the simpler product that was always underneath: occupied means
occupied, whoever drew the screen, and answering one is `gang attach`.

At hitch, positive evidence of a prompt without a composer is an operator
outcome before it is a launch failure: report `gang attach` once and spend the
remaining original boot bound waiting. A blank pane is only startup, not prompt
evidence. Unknown prompts remain manual, but clearing one leaves a healthy
agent whose startup contract can be delivered with `gang send`; only a composer
that never appears calls for drop-and-re-hitch.

A native permission-request witness has no timed decay. The readable composer
that proves the dialog ended is its only closer; malformed evidence refuses
instead of being erased. A synthetic `remain-on-exit` survivor read identically
before and after the former bound, so the timer prevented nothing, while under
tmux defaults a dead harness removes its window and leaves no orphan to report.

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

## Context lights are minimal, and their default is the collar's per model

Expose exactly yellow and red at intentionally high thresholds, notify once per
context epoch, and leave the decision to compact with the agent. Place both
thresholds below the observed native automatic-compaction boundary while
preserving most of the effective window. Expose the same native reading as an
on-demand query whether or not lights are enabled, because signalling and asking
are different acts.

One team-wide absolute pair cannot fit a mixed team: a red sized for the widest
native window cannot fire in the narrowest at all. So the thresholds an agent
gets default to the collar's own answer for the hitched model, and the collar
answers per model because one harness runs models whose windows differ
several-fold and the same fraction leaves very different absolute runway in
each. Collars express those defaults as fractions, since the window a model
reports is the provider's to change while the fraction stays correct. An
explicit spec overrides that answer for one agent or for the team, and absolute
tokens remain available there for a single observed window.

A default never arms a light its own collar cannot take a reading for; an
explicitly configured threshold still does, because that was an ask. Where a
collar wires its native context source at launch, the collar is sourced already
knowing the agent's request, so the wiring and the armed thresholds cannot
disagree.

## Provider usage is a collar-native observation

Keep provider-usage signaling optional and edge-triggered at operator-declared
used percentages. Read only non-interactive native evidence: a headless harness
query or the target session's native rate-limit event. Record its observation
clock and reset alongside the percentage so staleness stays visible, and let a
collar cap the age of evidence that may drive a warning. A still-future absolute
reset may arm a wake without spending quota merely to refresh its percentage. Never feed
decisions from the interactive usage page: driving a composer cannot observe a
busy agent and a pane rendering is not a stable data contract. Collars without
a correctness source say unavailable; status and roster report the last
ephemeral reading without turning observation into a poll.

## A provider-reset wait is one transient timer

Resume an idle agent at a native reset with one systemd user timer that invokes
the ordinary attributed delivery path and is collected after it fires. Store
only the pending declaration in the agent's tmux window, expose it in status and
roster, and provide exact cancellation. Do not run a Gangline watcher, daemon,
or retry loop; a removed or superseded target makes a stale one-shot firing a
no-op.

## An automatic resume is an arming threshold, not a cap detector

Let an operator declare one provider-used percentage that arms the existing
reset wake from the agent's own turn hook, separately from the warning
thresholds. Gangline cannot witness the provider's refusal: the only in-band
evidence is pane prose, already refused as a data contract, and a refused agent
takes no further turns, so no hook fires after the cap. Arming therefore happens
at the last percentage observable while the agent still runs, and the resulting
continuation may reach an agent that never capped — accepted, because it rides
the ordinary spool and the alternatives are a threshold that can never fire and
a scrape. Arm once per provider window, keyed on the reset that was decided for,
so a cleared wake is not re-armed over the operator and a refusal is not
retried. One shared transaction serves the manual command and the hook: it
accepts the already-validated sample instead of taking a second native reading
that could cross a provider reset, and overlapping hooks serialize the marker
check with the arm.
An existing manual wake is the operator's decision and wins over that automatic
transaction: mark the sampled window handled without cancelling its timer or
replacing its optional continuation. An overdue declaration, or a future one
whose timer is provably dead, is residue instead: replace it, and disclose when
that replacement discards a custom continuation.

## A promised wake is read from the unit, not only the declaration

A pending declaration on a tmux window and the transient timer that keeps it can
outlive each other, and the declaration alone then reports a wake nothing will
deliver. Status reads the unit whenever the reset is still ahead, and separates
gone from unreadable rather than merging them into an answer.

## A dead Claude stream resumes from two native witnesses, for one hop

Claude Code emits no Stop when a provider stream dies, but its later
`idle_prompt` notification binds the transcript path and the newest top-level
assistant record carries `error`, `isApiErrorMessage`, and a UUID. Require both:
the notification proves the interactive harness is waiting, and the structural
record distinguishes a dead turn from ordinary idleness without matching its
prose. Under `GANG_AUTO_RESUME`, close that missing turn boundary and submit one
ordinary attributed continuation per error UUID. The continuation's own
Gangline envelope is recorded before submission and compared byte-for-byte with
the native prompt event; a failure of that owned turn gets no second hop. When
ownership cannot be proved, fail closed and record the refusal for status and
roster. An ordinary prompt opens a new episode but does not erase an unseen
refusal; positive ownership of a later automatic turn does. This is immediate
collar-native discrimination, not a watcher or a general retry policy; collars
with no structural record declare no equivalent.

## Claude transcript predicates stop at the newest relevant record

Fatal-state and auto-resume readers walk the append-only JSONL backward and
stop once the record that decides their predicate is found. Earlier bytes cannot
change that answer and are not reread at every idle event. Every complete record
that could still outrank the answer must parse; a final line without its newline
is an append in flight and does not yet count as a record. This bounds ordinary
reads to the relevant tail without incremental state, rotation, or cleanup.

The continuation marker is visible to the agent in its own transcript. An agent
that reads it can in principle reproduce it, so it is an ownership witness under
Gangline's single-tenant trust model, not an authentication boundary. Hiding or
authenticating it would require a different native witness and must not be
smuggled in as anti-tamper machinery.

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

## A hitch makes missing model and effort choices loud

Warn at hitch for each omitted choice that the collar exposes, before a harness
silently supplies its default. Gangline requires the choice, never its content:
model names, effort levels, and policy remain the collar's and operator's words.

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

A turn fact nobody will ever edit — a bracket abandoned by an interruption
typed straight into the pane, which no harness reports — decays instead of
standing unknown for the life of the window: once it passes its bound, the
tiers beneath it answer. Decay requires every leg, measured rather than
assumed — a collar that does not declare quiet-at-rest reports inactive by
abstention, and an abstention is not a witness — and applies only to a readable
open bracket past its bound, since an unreadable or future-stamped one is
unknown, not abandoned. The pty clock and the screen are read before the first
tier and after the last, and a decay assembled while either moved is refused.
Inside its bound the bracket still outranks the tiers beneath it.

Movement seen during a decision is presence, not indeterminacy: a harness
paints the opening of a turn with its composer still empty, so an empty box
read out of a moving screen is one frame of something in motion rather than a
settled reading. That verdict carries its reason so delivery can refuse on it
alone.

## A refusal names what was read, not just that it refused

An obstruction gang can classify is classified: draft, staged, unattributed,
parked, whole-pane, cleared, unreadable, each with the look or recovery that
settles it. The naming look is taken after the decision and decides nothing, so
it must report what it actually saw: a box that emptied in that gap read
successfully and is cleared, never a harness that could not be read.
Human authorship is never the leftover case — the delivery legs that record a
paste whose fate gang never saw record no rendering to match, so a box beside
one of those records is gang's own text as plausibly as anybody's and is named
unattributed rather than blamed on a person. Classification uses only
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

## No name-only dialog registry

A registry naming stall screens without declaring a safe keystroke was decided
against. It could only name screens somebody had already met, while the generic
pointer gang already prints — inspect it with `gang attach` — covers every
screen including the unmet ones. Each per-dialog fingerprint is one more
per-build string that rots: when the wording moves it stops matching and falls
back to exactly that generic path, except that by then we believe we have
coverage. A guard that degrades to correct-but-silent is fine; one that
degrades while we think it holds is not.

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

## Persistent config exposes operator choices, not implementation seams

Only variables with a live persistent override consumer join
`GANG_CONFIG_KEYS`. `GANG_ACTIVITY_LIMIT` and `GANG_CLEAR_PRESSES` retain their
environment reads but have no repository, test, environment, or audited
operator override; copying their defaults into the file parser created public
promises without users. Config files that named them now refuse as unknown so
the removed surface cannot look accepted while doing something else.

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

## Native hooks share an opportunistic refresh pass

Native hook traffic is Gangline's cooperative event loop: no patrol, watcher,
timer, daemon, background processor, job queue, or scheduler duplicates it.
Every selected native hook invokes the one Gangline hook handler; collars own
event selection and mapping. A recognized event first applies its specific fact,
then one cheap, bounded, idempotent common pass may refresh derived state and do
work proved safe and useful on every invocation. A verified frequent hook may
carry that pass only when the refresh is its concrete live consumer. Unknown
event semantics stay loud, though safe event-independent maintenance may still
run. Hook silence leaves last-witnessed state honestly stale: refresh is
opportunistic, not continuous. Expensive or unsafe work stays throttled or
event-specific.

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

The outer hook writes to the terminal, and Gangline does not capture it to
replay the verdict last. What replaying it bought is given up knowingly: the
global gate's verdict printed in place sits above every line this hook emits
afterwards, and holding it back made it the closing lines of a successful push,
where a person reads the result. It is given up because a host-global gate that
spends minutes in inference reports its progress while it runs, and withholding
that until Gangline's own lint and suite finish makes a live push
indistinguishable from a hang. Fresher observation outranks closing position.

## Mandatory tests are immediate

Mandatory tests do not sleep, poll, or test timeout behaviour; use immediate
state, event barriers, or fake clocks, and test real harnesses only in separate
disposable tmux sessions.

## The irreversible verb is the one that demands an argument

`gang down` requires the session it ends and refuses from inside it. Every other
argument-taking command answers a bare invocation with its usage; `down` did not,
so the reflex that reads the manual — run it bare and see what it wants —
executed the teardown instead.

## Teardown archives mail before deleting its spool

A composed message is not teardown state. `gang drop` and `gang down` move every
waiting or held entry into a human-readable archive before deleting its window's
spool, and refuse to end anything if that archive cannot be written. The archive
is created only when mail exists and its printed path is the handoff to a person.

An addressee's own `gang mail` read still consumes what it prints, so the next
turn boundary cannot deliver the same message twice. It moves each claimed entry
into the same archive surface before writing that entry to stdout, and prints the
archive and deletion paths on stderr. A shell filter may hide rendered output;
it cannot erase the only copy or the route back to it.

Read archives are durable recovery state, not a cache: Gangline never guesses
when their human purpose is over. The read prints the exact deletion command,
and operations guidance makes the operator responsible for running it after
recovery or audit ends.

## A queue drains as one message

When a turn boundary drains a spool, every waiting entry is delivered as one
chronological bundle under one pane lock, envelopes intact. Delivering them
one at a time submits the first, which starts a turn, which refuses the second —
so a target that is never idle for long accumulates exactly the messages that
would have corrected it.

## A stop carries its reason, and never through the queue

`gang interrupt -m` stops a turn and delivers its reason at the boundary that
stop creates, under one continuous pane lock. It is never spooled. `--supersede`
retires the sender's own waiting entries and stamps the replacement now, which
sorts it behind every other sender's — so a stop sent that way arrives after the
work it was meant to stop.

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
tested has either happened or never will.

## The contract rides the system prompt where a collar has one

The standing terms live in `CONTRACT.md`, resolved operator-first and validated
before a window opens; a missing or non-prose contract refuses the hitch,
because an agent sent to read a contract that is not there would find that out
alone, in a pane, with nobody to tell. Where a collar declares
`GANG_ROLE_PROMPT_OPT`, the contract passes through it: the harness resends
that prompt every turn, so the terms are unconditional and survive a compaction
without a re-read. Collars without the option point the startup contract at the
file instead, which is still better than a paste bounded by what a composer
renders and by pane geometry. A role brief joins the contract in that prompt
wherever the collar has the option, and is pasted where it does not; doctrine is
pasted always. Neither can be replaced by a pointer the way the contract can:
they are per-hitch, and the pointer buys them nothing.

Prose validation proves file shape, readability, NUL and control-byte absence,
and UTF-8. It does not cap bytes: the former threshold admitted a measured body
the composer could not render while refusing differently shaped prose a system
prompt could carry. Pane-bound prose fails at verified delivery; system-prompt
prose meets the harness, operating-system, and model-context limits that
actually consume it. A mis-pointed binary still reaches the content checks
rather than being inferred from size.

A hitch may be role-less, but the contract is always present, so a role-less
hitch still carries a system prompt. Both share one option rather than passing
it twice, because a collar declares a single spelling and repeating it would
guess at whether the harness concatenates or keeps the last.

The startup contract does not repeat what the system prompt says. An agent that
reads the same assurance twice can still only act on it once, and Gangline
cannot verify that a launch option reached the model, so that line is spent on
the recovery instead: where the contract lives, and an instruction to report a
missing attachment rather than improvise.

## Harness prompt guidance stays in collars

Let a collar contribute harness-specific prose to the single system-prompt
addition when a live native feature creates a trap agents cannot infer. Refuse
that prose when the collar declares no system-prompt option; do not pass the
native option twice and guess how repeated values compose. Claude Code uses
this surface to state that its task list is session-scoped and unreadable from
other Gangline windows.

## A message is charged to the receiver; doctrine owns brevity

Message text stays in the recipient's context across later turns, so the sender
writes it once and the recipient pays repeatedly. Observed directly: two agents
on single lanes reached 500k and 616k tokens, the larger share inbound brief
rather than work.

The cost remains a team operating rule, not a substrate message schema. Doctrine
owns brevity and peer routing; the contract keeps the shared-state reachability
rule. Gangline does not inspect or ration message bodies.

## The gate runs against a tree the run owns

Run the mandatory gate through `test/gate.sh`. It copies the working tree —
tracked, staged and untracked alike — into a private snapshot, commits it there,
and runs lint and the suite from that copy.

Two properties follow and both are load-bearing. The complete gate becomes
runnable before a commit, because the executable is clean against the snapshot's
own HEAD and the dirty-execution warning that one mandatory stderr assertion
reads as failure never fires. No assertion was relaxed to reach that, and no
suite-only environment switch exists to relax one later. And the run owns its
tree: bash reads a script incrementally and `gang` re-reads collars and roles at
hitch time, so an edit landing mid-run otherwise changes what executes.

`test/lint.sh` and `test/integration.sh` refuse a tree they would not own and
name the command that does. The suite reads the tree's identity again at the
end, so a run whose source moved underneath it reports no verdict rather than a
count about a tree that no longer exists. What counts as the tree resolves the
operator's own git configuration and never the caller's environment: two reads
in one run must be answering the same question. A reading that cannot be taken
refuses; an unknown is not ownership, and neither is an index instructed not to
look at a file.

`test/gate.sh` is the one file the snapshot cannot protect, because it is the
file running from the live tree while the copy is judged. Its whole executable
body is one function, called on the last line with the exit, so bash has read
the file before it blocks and a save landing mid-run reaches nothing. And a
relative symlink is judged by where it points, not by whether the source end of
it exists: a dangling one resolves against the destination's parent and can read
bytes the source never held.

The no-argument gate also takes the shared heavy-test lock itself through
`flock -o`; callers run `test/gate.sh` directly. Closing the descriptor in the
command process keeps disposable tmux servers from inheriting the open file
description and retaining the lock after the gate wrapper dies. Read-only and
snapshot helper modes do not take the heavy lock.

## Commit gates require a destination boundary

Check Conventional Commits over an exact event range and refuse an unusable or
all-zero base. Post-push ref advertisement cannot determine a new ref's pushed
range: two refs created in one push are already visible to both runs, and two
equal new refs are indistinguishable from a routine new branch at a pre-existing
head. Default-branch merge-base therefore admits a false refusal by including
commits already on another branch, while subtracting other advertised refs
admits a false clean when concurrent new refs overlap. Neither is a measured
replacement for the missing pre-push boundary; pull requests retain their real
base and exact range.
The first push of a branch carries an all-zero `before` and therefore goes red;
that is the accepted cost of refusing to invent its missing event boundary.

## Native continuation owns compaction recovery

Native continuation now returns every supported compact command to a turn that
re-reads the brief and saved state. Repository checkpoint safety remains in
`AGENTS.md` and operations rather than the standing team contract.

## A fixture shell is hermetic

Gangline's suite compresses gang's production waits, so the pane's own reaction
to a keystroke is the whole margin. A bash launched with `--rcfile` or
`--init-file` still reads `/etc/bash.bashrc`, where Debian installs a
`command_not_found_handle` that runs a Python program against a multi-megabyte
apt database — and every envelope this suite delivers is an unrunnable command,
so that handler lands on the Enter path of every submission gang verifies. It
cost most of the compressed budget on an otherwise idle box, and it made the
suite's verdict depend on the operator's system configuration and page cache.

Fixture rc files therefore reach their shell through `ENV` in posix mode, where
bash reads no system rc at all, or the shell takes none with `--norc`.
`test/lint.sh` checks it. Enforcement rather than convention is the decision:
the property was already known and written into most fixture rc files as a line
authors copied, and the fixtures that starved were the ones that had not.

## A missing name is a self target

Where a command's only required argument is an agent name, omitting that name
targets the calling window. An agent reading or stopping its own state does not
have to know its own name, and the pane already answers who it is. This holds
for `usage`, `interrupt` and `flush` as much as for `status` and `capture`: what
the resolved target's state then allows is a separate answer, and `usage`
refusing its own mid-turn caller is that second answer rather than a reason to
withhold the first.

What is omitted is the name, not every argument. A leading flag is not a name,
so `gang interrupt -m "reason"` is a self-targeted stop carrying its reason;
reading the flag as a bad name made that the one self target an agent could not
spell. The reason is delivered to its own author on purpose — it is written to
be read after the turn it ended — where the same self-delivery on `send` stays
refused as the accident it is there. Gangline does not promise that delivery:
the caller runs inside the turn it is stopping, and a harness that ends that
turn by killing the tool call takes the sending process with it.

## Arity Gangline cannot consume is refused

Every command names the argument it will not accept and exits non-zero. An
argument taken and discarded reports success for a request nobody made: the
caller reads the answer as the answer to what they typed. The guard is a table
of one-argument-too-many probes whose expected text is each command's own
refusal, compared for completeness against the dispatcher's case arms, so a
command cannot be added without one and a probe cannot pass on an unrelated
failure. A probe's containment must hold under the regression it exists to
catch, and must not be read by the thing whose regression it contains: a
nonexistent agent name contains `drop`, but it is what `hitch` and `up` create,
and a working directory passed in argv is dropped by the same parser fault it
guards against. Those two are contained from the environment, which no argument
parser can discard, with the argv containment kept as the nearer of two. A
containment is asserted to be absent by reading an inventory, never by driving
the lifecycle command whose safety is the thing in question — a check that acts
in order to discover whether acting was safe has already done the damage in the
one case it exists to catch.

`gang hook` is the one command that records rather than dies. Its event is the
payload on standard input, so argv is an invocation it cannot read; its caller
is a harness configuration and law 7 keeps hooks non-fatal. It therefore names
the argument on stderr, stamps the window where `status` and `roster` surface
it, and declines the event. Declining is the refusal — processing the event
while discarding the argument is the silent acceptance the rule forbids.

## Startup contracts park behind native first-run gates

A first-run gate can outlive any boot bound before an agent turn exists, so a
Stop-only spool would strand the contract and a longer wait would only move the
failure. On positive pane evidence of an operator-owned startup prompt, hitch
immediately commits the attributed startup envelope to the ordinary window
spool and owns it until the universal tty surface exposes a composer. Direct
`hitch` keeps that observer in the foreground; `up` exposes the gated window in
its tmux client while the same invocation observes beside it. The verified
drain retries a pre-keystroke refusal and accounts for an exact entry retired by
a crossed native drain. This remains correct if an operator declines configured
hooks at a native security gate, adds no harness event or persistent fact, and
leaves the envelope inspectable if hitch is interrupted.
Once that positive evidence commits the entry, the hitch remains its foreground
owner without a post-gate deadline; Gangline starts no background watcher. An
interrupted gated hitch therefore requires drop and re-hitch/resume recovery
rather than a second hand-sent contract; resume applies only where a native
session identity was stamped before interruption.
Unknown stable screens still fail loudly instead of being called startup
prompts. Spool drains take the pane delivery lock before the first claim, so
crossed native workers cannot split or reorder the oldest-first bundle. A
mid-turn collar may declare `steer`: the envelope commits to the attributed
spool before any composer keystroke, then a free composer may accept its claim
as native steering. If the harness parks that Enter in its native queue,
Gangline retains the exact composer record for status and verified flush
recovery after retiring the attributed claim. PostToolUse is a delivery
opportunity even while the native turn record remains open. A live compaction
is excluded: its queue is the sanctioned landing only for the attributed
continuation that owns the compaction, while peer mail waits for PostCompact.
`park`, an occupied composer, and tmux copy-mode leave the entry live; Gangline
never cancels operator-owned mode state. Post-paste verification tolerates
bounded unreadable or unchanged redraw frames, then opens the staged-unknown
record and fails closed if no changed composer appears.

## State explanations instrument the classified read

`gang explain` records match/miss results only while the ordinary live state
reader evaluates collar-owned busy and occupancy rules, and prints the first
matching line from that exact capture. It does not take a later diagnostic
snapshot that could describe a different TUI frame. Rules bypassed by stronger
native evidence are named as not evaluated rather than fabricated as misses;
the diagnostic adds no stored state.

## Model discovery and validation stay collar-native

A complete native model catalog is a collar reader, normalized to exact model
ids with optional per-model efforts. `gang models` prints it and hitch requires
an exact match before opening a window. A harness with no complete catalog may
publish only documented aliases and a native recognition check; discovery says
the list is incomplete, and recognition never claims provider or account
availability. Failed, empty, duplicate, or malformed evidence is unknown and
refused. Core carries no harness model vocabulary.

## Fatal turns are a live collar state

An optional collar reader classifies native fatal-turn evidence as matched,
absent, or unreadable. A match becomes `!bricked!` before ordinary busy paint;
unreadable becomes `?unknown?`, and transient errors remain absent unless the
collar proves otherwise. The state is recomputed by observation and stores no
watcher record. Claude seeks backward to the newest complete top-level semantic
transcript record; an in-flight unterminated append is not a record, while
complete malformed data stays unknown. Meta notices and tool-result-only records
cannot clear the state, a newer real user turn outranks an old failure, and a
retryable API error cannot be borrowed as selected-model evidence or auto-resumed
as if replay could repair the selected model.

## Caller barriers use temporary native events

`gang wait` is an opt-in barrier, never a patrol or supervisor. Each caller owns
a unique temporary tmux `wait-for` channel and sparse hook key pinned to the
target window and active pane; ownership is re-read before cleanup because tmux
reuses the lowest free ordinary hook-array slot. Native Stop closes the turn
before signalling it, while natural pane exit and Gangline teardown release it
as a loud vanished-target failure. Direct tmux kill commands bypass that release
and are documented as unsupported teardown for a waited target. A foreground
deadline, defaulting to the existing native turn-fact bound, is a Python
one-shot alarm around one fresh Bash/tmux process group. Python is already a
required dependency; this avoids GNU `timeout`, while deadline and foreground
signals kill and reap that exact group before the caller cleans its hook. No
daemon, option, or file records the wait. A successful waiter consumes its
signal once; cleanup does not signal and wait again, because that second wait
can race the returning client and deadlock. Tmux offers no deletion for a latch
stranded by `SIGKILL` or a boundary race, so that nonce-named memory can last
until server exit. `?unknown?` and a Stop declaration with no native turn
evidence are refused rather than waited through, and `done` deliberately
promises only the next Stop — it does not claim to identify or own a logical
turn. Tmux channel locks are excluded because a dead holder can leak one
indefinitely.

## The mandatory gate fits under a memory ceiling

`test/lint.sh` runs one `shellcheck` per file, and `test/integration.sh` is
split into sourced parts for that reason alone. shellcheck holds an
invocation's whole input at once and its cost grows faster than that input
does, so the set costs far more than the sum of its files. Handed this repo as
a single invocation it reached 6.1 GB; on 2026-08-12 the kernel OOM killer took
it twice on an 11.6 GB host, and that host was power-cycled hours later after a
starvation livelock. A mandatory gate that is the largest single allocation on
the machine punishes the agent who runs it, which is the opposite of what a
gate everybody must run should do.

The rule is the ceiling, not a file count. The gate must pass under

```sh
systemd-run --user --scope -p MemoryMax=2G -p MemorySwapMax=0 -- test/gate.sh
```

and a file that grows until it alone will not fit is split. Parts are sourced
rather than executed, so the suite stays one program with one set of fixtures,
counters and ordering and the split moves no assertion. A part is a fragment,
so shellcheck cannot see across the boundary: a variable that crosses one
carries a directive naming the file at the other end, and that directive is the
record that the crossing was deliberate.

Splitting re-keys `test/source-guards.allow`, whose fingerprints bind a
statement to its path on purpose. A move is settled by showing that the
multiset of reviewed statements is unchanged and carrying each review note onto
its new key — never by regenerating the ledger, which would launder an
unreviewed guard through a mechanical step.

## Release installs are explicit and tag-bound

`install.sh` and `gang upgrade` resolve the greatest stable semantic
`gangline-vMAJOR.MINOR.PATCH` tag and install that commit, never the moving
`main` branch. `gang upgrade --check` is the sole availability probe; ordinary
commands stay offline. A named operator action keeps network failure visible
and avoids adding a cache, timer, startup tax, or self-watching component.

## Pre-push proves the fast boundary

The repository pre-push hook runs the outer contribution gate, production and
hook lint, a command smoke, and commit-message checks against the pushed tree.
It names the test lint, checker self-tests, and integration suite it skipped;
CI runs full lint and integration on pushes to `main`. Release Please is a job
in that same workflow and needs both verdicts before it can publish. The local
boundary stays quick enough to use on every push without overlapping the
memory-heavy shell linters, while the complete gate also runs the smoke.

## Real harness proof is one opt-in lane against a local model server

Every mandatory test drives a fixture: a shell pretending to be a harness, an
immediate clock, a pane whose transitions are synchronous. That is the right
trade for a gate everyone runs, and it leaves the collar unproven — the pane
regexes, the native hook wiring, the turn bracket and the transcript readers all
describe a harness no mandatory test ever starts. `test/e2e.sh` boots the real
one against `test/e2e/stub.py` instead of a provider, so the proof costs no
network, no account and no money.

It stays out of `test/gate.sh`. A real boot costs seconds and the lane holds a
turn open deliberately, which is exactly what the mandatory suite forbids;
`test/lint.sh` grants this one file the timing exemption and refuses if the file
ever appears in the gate, so the exemption cannot outlive its reason.

The stub answers on markers the lane puts in the prompt, hardcoded, rather than
through a scenario language. Its request log is the instrument: a claim that an
envelope was delivered is settled by finding it in what the harness actually
sent, not by reading it off a pane that shows only what was typed.

## Latest claude-code is probed outside the mandatory contribution path

The offline e2e lane runs daily and on dispatch in its own workflow, which
installs the npm `latest` claude-code release. Harness and collar drift is then
probed within a day without adding real-TUI wall time to pushes, pull requests,
hooks, or `test/gate.sh`. The workflow records the harness build and runner
environment before the lane so a failure can distinguish a moved harness
surface from a changed runner substrate.

## A held response is the only honest way to freeze a turn

Scenarios that need a live turn — mid-turn steering, a wait that must block —
cannot get one from a sleep, because the thing being measured is whether
Gangline observes a turn that is genuinely in flight. The stub holds its
response open on a FIFO pair instead. Opening a FIFO for writing blocks until a
reader arrives and opening one for reading blocks until a writer does, so the
lane learns the turn is live from the turn itself and ends it when the
assertions are done. Neither side polls, and the turn is live for exactly the
window under test.

## Not every request carrying the prompt is the agent's turn

Observed on claude-code 2.1.233: each submitted prompt also triggers a small
auxiliary session-title completion whose body quotes the user's message, and it
arrives BEFORE the real turn. A stub keying on prompt text alone holds that one,
releases the lane, and lets the turn it meant to freeze run unheld — with every
assertion still passing, because a request log records arrival rather than
completion. The lane therefore identifies the agent's own turn by the standing
contract the collar passes through `--append-system-prompt`, read from
`CONTRACT.md` at run time so a reworded contract breaks loudly instead of
quietly reclassifying every request as a side errand.

## An API key in the environment is a first-run gate

A cold `CLAUDE_CONFIG_DIR` draws onboarding rather than a composer, which the
claude-code collar already enumerates. A seeded one still stops: given
`ANTHROPIC_API_KEY`, the harness draws a two-choice approval box defaulting to
No, hitch correctly reports a native first-run prompt, and an unattended lane
waits for an operator who is never coming. The harness identifies a key by its
last twenty characters, so the lane records the answer the same way alongside
the onboarding and trust flags.

## A request log is only evidence once it separates whose request it was

An aggregated log answers "did these words reach the server", which is not the
question any assertion means to ask. The harness counts tokens and titles
sessions with bodies that quote the same prompt, so a search over every record
can be satisfied by a request the agent's turn never made — and a log records
arrival, so it can also be satisfied by a request the harness is still blocked
on. The stub therefore marks each record with whether it was the agent's own
turn and writes a second record when the answer is fully sent, and the lane
reads only completed agent turns. Two facts that must hold together are asserted
against one record, not against the log twice.

## An answer that names its request is what a pane check needs

Every completion this stub writes looks alike, and booting already put one on
the pane, so a check for the answer's prefix passes whether or not the turn
under test was ever drawn. Each answer carries the sequence number of the
request it answers and the stub tells the lane which request it froze, so a
scenario can name one turn's reply instead of accepting any.

## A budget that cannot be spent is not a bound

The lane's bounded waits discarded exhaustion and let the following read decide,
so a wait that used its entire allowance still passed on the extra moment the
last nap gave it. Exhaustion is now a failure in its own right, and the
assertion after it stays only to report what was true when the wait gave up.
The same rule closes the run: cleanup that fails, a signal handler that returns
into a torn-down world, and a scenario list that selects nothing are all ways of
finishing without reporting, and each one now ends the run with a reason.

## One agent per killable cgroup, or one kill ends the team

A tmux server inherits the cgroup of whatever started it, so every agent on a
team is a process in the login session's scope. `systemd-oomd` selects the
descendant *leaf* cgroup holding the most swap, and a long-lived session full of
dormant agents is by construction the largest holder of swapped-out anon memory:
idleness is the qualification for being chosen, not a defence. One kill ends
every agent at once.

`GANG_SCOPE=on` wraps each hitched launch in a transient systemd user scope, so
each agent is its own leaf, holds its own swap, and is named in the kill message.
This is a platform-specific launch prefix rather than a branch on any harness:
the collar still declares the whole launch line, and the scope is composed
around it. It is off unless the operator declares it, and where it cannot be
honoured the hitch is refused rather than quietly run unscoped.

The trade is deliberate and is not free. A scope lives under the systemd user
manager, so scoped agents also fall under its memory-pressure policy — a
separate policy with its own trigger, which can take one agent for pressure that
no agent caused, and can act again after its own delay. And a swap kill selects
only a candidate holding more than 5% of total swap, so a team split finely
enough can leave every agent under that bar, where the swap policy selects
nobody rather than the team. Losing one named agent is still a better outcome
than losing every agent at once, but the thresholds are the operator's and this
changes only the victim pool.

## A launch that died is read as a death, not waited out

Every reading in the boot wait asks what a pane is showing, and none of them
asks whether anything is still running to show it. So a harness that failed at
launch spent the whole boot budget and was then refused as an agent that is up
but showing something other than its input box — a live-agent recovery offered
for a process that is not running — or, where the window went with it, as a raw
tmux error naming nothing Gangline had tried to run.

The wait ends on the death instead, and the refusal carries the launch command,
plus the exit status and the pane's last line wherever a corpse was held. The
same reading answers every window registration a hitch performs, because a
launch can die under any of them.

Absence is established from the answer rather than from a command status: tmux
expands a target it cannot resolve to nothing and still exits 0, so the reading
asks for the window's own id alongside the fact it wants, and a reading that is
neither of those refuses rather than passing for a healthy launch.

## A pane's death is proven by its descriptors, not by tmux's pane-died hook

No fixture may wait on `pane-died`. tmux settles a pane's death from two
independent events — the pty reaching EOF, which is what makes `#{pane_dead}`
read 1, and the reap of the child, which fills in `#{pane_dead_status}` and
draws the held corpse's banner. The hook is dispatched only from the second,
and only where the first has already landed, so a death whose EOF is processed
before its reap dispatches no hook at all: not then, and not when the reap
arrives afterwards. Forcing the reap fills the status in and leaves the channel
blocked, so the signal is lost rather than late.

`tmux wait-for` has no bound, so a fixture holding that channel cannot go red —
it can only stop the suite and exhaust the CI cap. `gang` itself reads
`#{pane_dead}`, which the EOF alone settles, so the fact under test is true well
before the hook that was being waited on.

A death is ordered behind the pane's own file descriptors instead. The pane
holds a fifo open read-write so its own open cannot block, and reading that fifo
to EOF ends when the pane's last descriptor closes, so the process is gone
rather than merely typed at. A tmux round trip with a child of its own then
drains any corpse the server had not reaped, and the settled fact is asserted
immediately, so an unsettled death is a loud red rather than a hang.

The fifo is opened in the pane's own launch command wherever the fixture writes
one. A launch command runs before the shell reads anything, so the descriptor
exists as soon as tmux has spawned the pane, and a pane that is alive but never
reads its input cannot park the fixture waiting to open it. Where the launch
belongs to a hitched agent the line is typed instead, on the same established
readiness the exit typed after it already rests on; that is the suite's ordinary
standard for a typed barrier, and it is the remaining place where a shell that
stopped reading would park rather than fail.

Why tmux fails to run its SIGCHLD handler in this window is not established, and
the rule does not depend on it: a fixture that cannot be broken by a late reap
does not need the cause.

## A read that was refused is unknown, never an absent box or a settled one

A pane reading is transport, and the transport can refuse while the agent it
describes is alive and healthy. Every predicate that consumes such a reading
spends it as evidence — no box drawn means nothing owns the screen, a box
identical twice means nobody is typing, a witness equal to the one before it
means nothing moved — so a refusal folded into any of those becomes a positive
finding about a pane nobody looked at, and a caller then acts on it.

So a refused read carries its own status the whole way. Collars answer `3`,
distinct from the `1` that means the harness drew no composer and from the `2`
that means a composer outgrew its pane. `input_read` is the single place that
classification is made in `bin/gang`. Predicates that cannot express unknown
refuse loudly instead: occupancy, busy/idle and decay all name the reading they
could not take.

Producing status `3` is the collar's obligation and cannot be performed on its
behalf. Where a collar answers `1`, Gangline asks the pane directly, and that
probe is worth exactly one thing: a transport still refusing turns the absence
back into an unknown. The converse is not available. A pane that answers proves
the transport is up at that moment, and the collar never parsed that reading, so
it is no evidence about the read that already happened — a refusal that healed
in between remains an absent box to a predicate that looks once. Claiming
otherwise would be the same fabrication one layer up.

What closes the gap is not trusting any single absence where absence is spent as
permission. The settled check takes two looks and compares their statuses as
well as their contents: a box drawn for one and absent for the other is refused
whichever way it moved, because a harness painting or dropping its composer and
a collar reporting a refused read as an absence are the same reading from here,
and neither is a box nobody is typing into. That holds whoever wrote the collar.

The rule binds every pane reading a collar takes, not only the composer.
`collar_context` spends nothing — the command ends either way — but a refused
capture reaching its parser makes it report a missing context readout on a pane
nobody read, which points an operator at the harness when the fault is the
transport. It reads into a variable and refuses with a status of its own.

Two shapes hide such a refusal by construction and are banned wherever a
reading is assembled. A capture piped straight into a parser arrives as the
parser's verdict on empty input, which is indistinguishable from its verdict on
a pane with nothing to find. And a reading assembled inside the arguments of a
`printf` — or handed to another substitution as an argument — leaves the outer
command succeeding on a substitution that failed, so `die` inside it exits a
subshell nobody is watching. Read into a variable, check the status, then use
it.

## A row Gangline could not read is that agent's fact, not the roster's silence

`gang roster` is the check run before `gang down` or `gang drop`. It ran each
row in a subshell under `set -e`, so the first agent whose pane refused a read
ended the whole listing: the rows before it printed, the rows after it did not,
and from outside that is indistinguishable from a smaller team. Loud, but the
loudness was about the wrong scope.

The refusal belongs to one agent. Its row carries `?unknown?` and the marker
`state-unreadable`, the refusal naming the reading it could not take reaches
stderr immediately above that row, every other agent is still read and printed,
and the command exits nonzero so nothing spends the listing as an all-clear. The
predicates are unchanged: occupancy and busy still refuse out loud rather than
express an unknown they cannot express, because callers spend their answers as
permission. What changed is where that refusal stops.

The porcelain word is `unknown` for both a state Gangline determined it could
not settle and one it could not read at all. The human row separates them and
the machine row does not; the two are the same answer to the question porcelain
is asked, which is whether this agent's state is known.

The exit status does not carry that answer, and saying it did was an overclaim.
It reports whether every row's reading could be TAKEN. A reading that WAS taken
and did not settle prints `?unknown?` with its own witness and exits 0, which is
what `gang status` has always done for the same agent, and making roster alone
disagree would put the same fact in two channels that contradict each other. The
rows are where an unknown is reported; the status says only whether gang could
look.

## Whether an agent can be resumed is answered where the harness is chosen

Resuming needs a launch line with a session slot and a collar that witnesses the
native id to put in it, and those are separate declarations that are missing
differently. `gang drop` was the first place that said anything about either —
after the agent, and its session, were already gone. `hitch` warns instead, and
`gang collars` marks the capability per collar, because `-c` is where the
operator is choosing and would choose differently for work that must survive a
restart.

Treating the two as one requirement produced a false statement for each single
half. A collar with the launch and no witness still relaunches, onto an id the
operator holds; calling that `no-resume` said the session was lost for good.
A collar with the witness and no launch still stamps its id through the hook
path, so a warning saying Gangline would never learn that id was disproved by
the core path minutes later — and `gang drop` then printed a relaunch command
`hitch` refuses, which is the worst of the three: an operator's one recorded way
back, quoted at the moment the agent ends, not runnable.

So each half is answered where it fails. The capability word has three values.
The hitch warning has a branch per half. And a parting line quotes a relaunch
only for a collar that declares the launch, and otherwise prints the id as what
it is — a record of the harness session rather than a way back into it.

## A native first-run prompt is answered by a person, and the wait for one is bounded

Gangline answers no native dialog, so a positively identified first-run gate
leaves exactly one thing to do: a person answers it. Holding the caller's
terminal until that happens made an unanswered prompt stall the caller too —
and the caller is often another agent, which cannot answer a native prompt at
all, so one gated boot stopped two agents and neither could report why.

`GANG_GATE_LOOKS` bounds the observations `hitch` spends on a prompt that is
still unanswered, and reaching it exits 4: distinct from a hitch that failed and
from one that delivered, so a caller can tell the three apart. Nothing else
changes — the window is alive, the harness is running behind its prompt, and the
attributed contract is committed to the ordinary spool. What ends is Gangline's
claim on the terminal.

Observations, not seconds. Every readiness pass that sees the gate returns on
its first reading, and a duration is spent instantly under a stopped clock,
where the give-up path could then never be driven at all.

`gang up` is exempt. The budget releases a caller who cannot answer, and `up`
has already attached its caller to a tmux client showing the prompt: there is no
stalled third party there, and the give-up would detach the one client that can
answer — reporting that nobody answered by removing the means to.

The budget cannot be recovered by waiting longer, because nothing claims a spool
before its agent takes a turn and an agent behind its first-run prompt has taken
none. So the recovery named is the one that works: answer the prompt, drop, and
hitch again. Answering is a native persisted choice, and codex's hook trust in
particular is keyed on the hook definition rather than on the bytes of the
script the hook command names — so ordinary development on `bin/gang` leaves an
answered gate answered, while a `gang` at a different path raises a new one.

## A readable frame without a native beacon is a miss before it is an outage

Claude can briefly redraw, cover or scroll its context beacon off-screen while
its pane and the beacon source remain healthy. The collar returns status 2 for
that readable-frame miss. Context lights report the first miss in a source
failure epoch without changing the last real light; alternating good and missed
frames do not repeat it. A consecutive miss latches unavailable, while an
unreadable or malformed source still fails on its first observation.

The threshold is carried by a window option and advances only on native context
checks. It adds no pane capture, retry loop, clock, parser or roster read.

## A spool nobody claims is swept where a team starts, not watched

A spool directory is resolved through `@gl_spool` on a live window, so one left
by a window that did not die through `drop` or `down` is unreachable by every
command Gangline has. The answer is not a component that watches for them —
that is the loop law 7 forbids — but the one moment the question is both cheap
and safe to ask: the hitch that opens a session, where the server has just
proved it answers and the only spool Gangline owns is the one it just minted.

The sweep archives; it never deletes a body. A directory it cannot archive, and
one Gangline did not mint, are named and left exactly where they are. A window
list that cannot be read is reported rather than answered as "nobody holds
anything", which would archive every live agent's mail.

This makes one server per `GANG_LOCK_DIR` explicit. It was already implicit:
`spool_mint` draws an identity no live window holds, and reads that list from
one server.

## Typed-but-unconfirmed is a third verdict, not a held message

`refuse` means nothing was typed, so the body is still the sender's and can be
parked. `die` means Gangline could not do what it was asked. A send whose Enter
was pressed and whose screen then stopped answering is neither: parked it
becomes a second copy of a message that may already have arrived, and reported
as a failure it invites the sender to make that copy by hand.

It exits 5, says `delivered but UNVERIFIED`, and its spool record is named
`unverified-` rather than `failed-` so every later reader can tell "Gangline
watched and saw nothing enter" from "Gangline watched the keys go in and lost
the screen". Neither is ever sent again.

## An abandoned staging fragment is asked about its writer, not a clock

`spool_stage` writes a body under a name no drain reads; `spool_commit` renames
it into the deliverable namespace. A sender that died between the two left a
message no surface named. The filename already carries the writing pid, so the
question "is this a casualty or a send from a moment ago" is answered by asking
whether that process still answers — the same evidence the delivery lock already
trusts about its own holder.

## A harness with no hook command still has a session identity

Gangline learns a native session id only from a hook payload, so a collar whose
harness ships no hook command can never stamp one. `drop` then reports UNSTAMPED
after the conversation is already gone, and every agent on that harness is
unresumable — a fact the operator needed at hitch time, when the harness was
chosen.

Where the harness says its id somewhere else, the collar carries it out from
there. OpenCode's plugin bus is such a place: the collar composes a plugin into
its launch the way its siblings compose native hooks, and the plugin turns
session events into the one payload that stamps identity.

What a collar carries out this way claims nothing further. The payload is shaped
as an event the collar gives no other meaning, so identity is recorded and no
turn bracket opens that no event would close; a collar that cannot witness turn
boundaries still declares no Stop hook. A stamp is an identity, not a way back,
and the two halves stay separately earned.

## A prompt the harness draws into an empty box is not a human's line

A composer carrying the harness's own suggestion is empty. Returning that
suggestion as composer contents makes Gangline see a half-written line, refuse
to paste into it, and leave a freshly hitched agent registered, running, and
unreachable.

Recognising one is pinned copy paired with the position it must hold, not a
colour: a theme chooses colours, the harness chooses words. Pinned copy fails
safe — a reworded prompt stops matching and the collar reports a draft again,
which refuses delivery. Nothing here may turn a line somebody is typing into an
empty box.

## A collar refuses a launch it can see will stop on a native gate

Gangline does not answer native dialogs, and `hitch` behaves correctly when one
appears: the screen is occupied, delivery parks, and the prompt is named for a
person. What that costs is a boot spent to learn something knowable beforehand.

Where a harness will answer, before it starts, whether it is about to gate its
own boot, the collar asks and refuses the launch instead — naming what is
ungated and the exact native command that clears it. Codex answers this about
its hook trust through its app-server, under the same overrides the launch will
use, so nothing reproduces its hash algorithm or reads its trust records behind
its back.

The refusal grants nothing. Trust decides what may run outside the sandbox and
stays the operator's; the remediation is the operator answering the native menu
once. A state the collar cannot read is refused as well: launching blind would
restore the stall the check exists to remove, and a gate that quietly degrades
while it is believed to hold is the worse failure.
## Evidence of action is two ages and no verdict

A verified delivery proves text reached a pane; a turn bracket proves a turn
opened and closed. Neither proves the recipient did anything, and an agent that
answers in words and runs nothing satisfies both. So `roster` and `status` carry
the age of the last tool call the harness recorded, and the age of the last write
to the pane where that is the only evidence left.

They are not collapsed into a health state. A long build and a wedge both go
quiet, so any threshold gang picked would be gang's judgement wearing the
harness's authority. The four tool-call answers are kept apart instead — an age,
a bound when a scan limit was reached first, `none` for a source read whole with
no tool call in it, and unknown with a reason — because `none` is a claim about
the session and unknown is a claim about the instrument.

## A tool call is a family of record types, not one name

The rollout that started this was read as having no tool calls because only
`function_call` was counted; every call in it was a `custom_tool_call`. A closed
list that silently misses a family reports a working agent as one that has never
acted. Each collar therefore recognizes the families it has seen, and reports a
call-shaped record outside them by name as unknown — loud, in the direction that
cannot fabricate either verdict.

## A dead response stream is told apart by its missing status, not its sentence

Claude Code ends a turn killed mid-stream with a synthetic assistant record
carrying `error=server_error` and no `apiErrorStatus` key. Specimens of it say
the response stopped arriving, that the server errored mid-response, and that the
connection was lost; the structure is the same in all three. Matching the absent
key rather than the prose keeps the reader on the harness's data and off its
wording, and leaves every status-bearing `server_error` nonfatal.

## The teardown guard compares sockets, never command shapes

Inside a pane `$TMUX` outranks `TMUX_TMPDIR`, so the command that ended a live
team read as aimed at a sandbox. A guard that matched shapes would have passed
it. The shim gang puts on an agent's PATH resolves the socket the invocation
would actually reach — `-S`, then `-L` under `TMUX_TMPDIR`, then `$TMUX`, then
the default — and compares it with the socket gang recorded for that agent's own
team. Every reading it cannot take ends with the real tmux running unchanged.

It is a guardrail rather than a boundary, and says so: one variable runs the
command anyway. Refusals and overrides are both written under the lock root,
because the 2026-08-24 reconstruction had to be assembled from journald for want
of any record gangline owned.

## A team's socket is written down where a later shell can read it

tmux clients discover the default socket, so a team on a private `TMUX_TMPDIR`
is unreachable from a shell that lost that environment and looks exactly like a
team that ended. `hitch` records the socket under the lock root; `teams` reads
the records back and asks each server rather than believing the file; `attach`
crosses to a recorded socket only when this shell's own does not have the team.
The record is a fact about reachability and never authority.

## A named composer band says which conversation owns the box, not that there is none

claude-code draws the composer between two full-width rules and burns the
ACTIVE conversation's name into the opening one as soon as that conversation
has a name. A backgrounded parent session carries its own title; a selected
in-process subagent carries its task. The two frames are drawn identically and
the name inside is free text, so the band alone cannot say whose box it is.

Reading the band as "no composer drawn" answered both cases wrongly in
different directions: a titled parent went unreachable — status called a
healthy agent occupied by something it could not name, and delivery refused —
while the child's box was refused for a reason that named nothing an operator
could act on.

The footer settles it. The permission-mode control belongs to the parent
conversation and is drawn under its composer in every mode; a selected child
carries that child's own controls, and the switcher marks the conversation in
use with a filled ring rather than with the keyboard caret. A named frame
proven to be the parent's is read as the parent's composer. Anything else is
status 4 — a composer is drawn and it is not this agent's — which Gangline
carries through instead of flattening into absence, because driven on 2.1.241
typing into the selected child's box resumed the CHILD in the child's own
transcript. Refusal is the safe direction and every uncertainty takes it.

## A turn bracket that reached its bound is a boundary nobody raised

Every spool drain hangs off an event the harness announces. Measured on
claude-code 2.1.241, a turn a person ends by declining a permission dialog
announces nothing at all: no `Stop`, no `StopFailure`, no `PermissionDenied`,
no `PostToolUseFailure`, and no late `Notification` — three denials by two
routes, silent for up to 160s against a pane visibly at rest, with an entry
spooled before the denial still queued 98s later. The harness's own code says
why: `PermissionDenied` fires only for an auto-mode classifier denial,
`StopFailure` requires an error result, and `Stop` runs at the normal end of a
query loop that a denial aborts past. Registering an event, which is what the
report proposed, has nothing to register.

The bracket's expiry is Gangline's own fact and already licenses an idle
verdict, so it also offers the window one delivery opportunity. It does NOT
rewrite the bracket: stamping it closed would convert `turn_witness`'s
could-not-determine verdict into a confident idle one, and a tool call longer
than `GANG_TURN_LIMIT` is exactly the turn that would then be typed into. What
expiry buys is the attempt; the busy witness, the collar's mid-turn
declaration and the composer guards decide it, the same way they decide an
ordinary send against the same window.

The full measurement, its raw hook log and the probe that produced it are at
`~/Documents/agent-artifacts/2026-08-24-stopsmith/issue123-measurement.md`.
