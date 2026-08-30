# Lead-brief revision

> Status: Current. Records why `roles/lead.md` gained the decisions between arcs
> it now carries, and what its acceptance instrument can and cannot prove. The
> body is historical; the shipped brief is authoritative.

## What the previous brief did not decide

The statements it carried — delegate whole results, do not produce, do not
prescribe, do not review, stay idle, choose harness and model, judge the report
on evidence — are all still true, and every one of them is about a single arc.
A lead can obey all of them and still have no rule for anything that holds
between two arcs, which is where the decisions below live. Each is stated
as a claim about the role: if it needed a particular team on a particular day to
be persuasive, it was a fact about that team and not about the brief.

**Disjoint tasks do not make arcs concurrent; disjoint files do.** Splitting
work by issue, feature, or defect partitions the *reasons* for editing and says
nothing about the *lines* edited. Two owners handed unrelated defects in one
component will both open the file that implements it. Nothing surfaces the
overlap while the arcs run — each owner sees only its own tree — so the
collision appears at landing, after both have finished, and the cost is paid
twice: reconciling the edits, and re-proving that neither owner's work was lost
in the reconciliation. The dispatch is the only moment the lead holds both
answers, so the comparison belongs there and has to be over files written
rather than tasks assigned.

**Silence is not evidence of progress.** With reporting push-based, a lead's
only positive signal from an arc is the owner's report, and a lead that is
correctly not polling has silence as its normal reading of a working arc. An
agent that has died produces exactly the same reading, and it produces it
forever, because the report that would end it is the one thing a dead agent
cannot send. So the two states are indistinguishable from the outside and one of
them never resolves. Phrasing the answer as a watch does not work: silence is an
arc's normal state, so a watch over it resolves to an elapsed-time expectation
or to repeated inspection, which are tracking and polling under other names.
What makes it actionable instead is that the ambiguity only costs anything when
the lead asserts something. A lead may report an arc's state only from what it
observed, and a lost arc is unfinished work needing a new owner rather than a
completed one.

**A step only the lead can perform becomes the team's throughput ceiling.**
Where some step of the work is available to the lead and not to owners — a
credential, a gate, an approval, a shell the owners' harnesses cannot reach —
each owner reaching it stops and hands the step back. Performed once this is
help and reads like it. Performed for every owner, it serialises the whole team
through one agent's turns, and each pass costs a full round trip of report, run,
read, brief. The threshold that distinguishes the two is the second arc: at that
point the obstacle is no longer an interruption but a piece of work with its own
result, and it should be dispatched as one. A decision only the lead can make is
not this, because deciding is the lead's own work and does not become an arc by
recurring.

**A lead's ruling is made with less evidence than the owner ends up holding.**
The lead rules at dispatch or on a decision request, from a summary; the owner
then spends an arc in the material and can measure what the lead assumed. A
brief that tells the lead to judge an owner's evidence but is silent on the
reverse leaves the lead defending a ruling against the only party better placed
to test it. Worse, rulings propagate: the lead relays them to other owners and
to the operator, and a relayed ruling keeps being acted on until it is withdrawn.
So the obligation is two-part — re-decide on the new evidence rather than defend,
and send the outcome as far as the ruling itself travelled.

**A failing substrate cannot be delegated, because delegation runs on it.**
Every other rule here assumes the lead can hitch an owner and that the owner can
work. When what fails is the thing all arcs stand on — a host that cannot spawn
a process, a lock every agent blocks on, a filesystem that has filled — the
assignment itself is unavailable, and an owner's harness may already be dead of
the same cause. This is the one case where a lead that does the work is not
usurping an owner but is the only agent able to act at all. It stays bounded by
two conditions rather than by judgment: what fails must be common to every arc,
and no owner must be reachable in time. The repair afterwards is ordinary work
and goes back to an owner as an arc.

**Harness spread was stated as evenness, which was never the property.** The old
line asked for arcs spread "evenly across available harnesses rather than
defaulting to one." Evenness is not a benefit; the benefit it was standing in
for is that a reviewer sharing the author's model reproduces the author's error
instead of catching it. Harness count does not deliver that either — two
harnesses can run one model, and one harness can host several — and stating the
count also lets a lead optimise topology while every owner still reviews itself
into the same blind spot. What the launch choices must buy is the property
directly: every owner has an available reviewer whose error modes differ from
its own.

## The executive framing, and one argument with it

The framing this revision was given is that a Gangline lead is an executive over
orchestrators rather than an orchestrator: it delegates whole results rather
than tasks, sequences work it will not perform, judges evidence rather than
effort, and spends its own context on decisions that cannot be delegated.

Three of those four were already in the brief. Delegating whole results is its
first line; judging evidence is its last; not performing the work is the second
half of its first. Only *sequencing work it will not perform* was absent, and
the fourth — spending context on undelegatable decisions — is a claim about a
resource rather than a behaviour: it does not tell a lead which decisions those
are, so it cannot be obeyed or disobeyed.

The decisions above answer that question, and they answer it the same way
every time. Every one of them is a property of the seams between arcs: which arcs may
run at once, what each may touch, which agents still exist, what the whole team
is queued behind, which models are represented, and what all arcs stand on. An
orchestrator sees inside one result. An executive sees between results, and
that is the only vantage from which any of these is visible at all — no owner,
however good, can see outside its own arc. So the undelegatable decisions are
not a category the lead has to feel out; they are exactly the cross-arc ones.
The brief names them one by one and does not state the principle above them,
for the reason given below.

## What the acceptance instrument proves

`test/role-briefs.sh` is a delivery instrument. It proves that a brief's bytes
survive validation, reach the system prompt or the pane intact and in order,
lose to an operator file of the same name, refuse when malformed, and are
attached by `gang up` without being inferred from a window name.

For the *content* of a brief it can do one thing: assert that a sentence is
present. AC16 now locks each decision the lead brief carries as its own named
check, so deleting one or rewording it away is a visible red rather than a
silent edit. What a lock cannot catch is a sentence added later that negates
one still present — appending "ignore the above where delivery would be faster"
leaves every lock green while gutting all of them. That limit is inherent to
matching text, and the comment above the locks says so rather than leaving a
reader to assume otherwise.

AC1 previously asserted that one marker sentence of the shipped lead brief
reached an agent's system prompt, and AC16 reads the file on disk. Between them
a delivery truncated after the first decision would have satisfied both, so the
pair could not support the claim that every locked decision is delivered. AC1
now compares the whole shipped body against the launch argument. Driven against
a `bin/gang` mutated to deliver only the brief's first lines, the marker
assertion still passes and the whole-body assertion fails, which is the gap it
was added to close.

One guard fails on content nobody has written yet: no sentence past a length
floor may appear in both `CONTRACT.md` and a shipped brief, which are edited
separately and drift toward restating each other. It is an exact-duplicate
guard and nothing more, and what defeats a naive one is Markdown rather than
rewriting. It therefore strips three things before comparing. Headings, which
terminate no sentence and so glue themselves to the sentence below, hiding a
duplicate outright. Terminal punctuation, without which a verbatim copy escapes
on a changed full stop. Emphasis and code markers, without which a copy escapes
on a pair of asterisks that both stop the splitter ending the sentence and stay
inside the compared text. Underscore is deliberately left alone, spelling
identifiers here far more often than emphasis. A paraphrase, a copy with an added clause, or a rule split across
two sentences all still pass, and no general matcher would catch those. Its
green is evidence only because fixtures in the same check feed it those two
disguised copies and require it to go red on both.

## What is not tested, and what would test it

Every line of the brief is behavioural, and no line of it is behaviourally
tested. The locks are claims about a file, not about a lead.

The principle above them is deliberately absent from the brief: a sentence
saying the decisions between arcs are the ones only the lead can make is the
reasoning behind the specific paragraphs, and it makes them cohere, but a lead
could obey it and do nothing differently. Locking a sentence like that would
give rationale the same permanence as a decision. The specific paragraphs carry
the behaviour; this record carries the reason.

Testing conduct needs a different lane, and it is further from the current
substrate than it first appears. Gangline transports prose and observes tmux. It
holds no arc, no file ownership and no task state, and by `CONSTITUTION.md` it
must not grow any: a dispatch is free text, so nothing in the substrate knows
which files an arc was told to write or whether an obstacle became an arc.

That splits the brief's decisions in two. A few turn on a fact that is a
window: an agent killed while its roster line still reads idle, and whether a
replacement is hitched. A scenario harness on a private socket can stage that
and read the outcome off tmux with no judge — but only the positive outcome. A
lead that never looked produces no event to observe, only an absence before a
deadline the eval declares, and the scenario's own declaration is likewise the
only thing that makes the killed window mid-arc rather than merely dead. So even
the cheap half is an opt-in eval with a substrate-scored positive, not a test.
It is still the half worth building first.

The rest are not observable without a manifest the eval itself supplies —
declaring, outside Gangline, which files each staged arc was told to write and
which step was blocked — and scoring them means reading the lead's dispatches,
which is a model judging prose. A model judge is not a test. The same
repository's push scanner measured two runs of one ruleset over identical bytes
agreeing on 17 of 39 findings. Such a lane must sample repeatedly and take a
union or majority across runs, state its measured spread beside every number,
compare two candidate briefs rather than pass judgment on one, and never gate a
commit. It is a separate opt-in lane for that reason as much as for the model
calls and wall clock the mandatory gate is allowed neither of.
