# ADR-0017: A declared fact that never arrives is not a missing tier

- **Status:** Accepted
- **Date:** 2026-08-03

## Context

Every predicate Gangline answers is tiered
([ADR-0008](0008-evidence-is-tiered-per-predicate.md)). The strongest tier is
facts the harness itself writes through its own events; below it sit marks
Gangline wrote, and below those the pane scrapes. A reader takes the best tier
that can answer and falls through when one cannot.

A profile already declares which of those facts its harness supplies. One
harness declares a turn bracket, a context readout, occupancy and compaction;
another declares none at all, deliberately, and says so where a reader will find
it. The declaration exists because only the profile can know it — core holding a
belief about a harness is what laws 1 and 4 refuse.

Absence of a record is currently read as absence of the tier. Where the harness
supplies nothing, that is exactly right: there is no event tier, the reader falls
to the one below, and nothing has gone wrong. Where a harness *declares* a fact
and that fact never arrives, the same branch fires and the same fall-through
happens — silently. Gangline then answers from weaker evidence and reports a
state that looks healthy, with nothing anywhere saying that stronger evidence was
promised and did not come.

The two ways a fact can be missing are one branch apart and only one of them is
loud. A record that arrived and then could not be read back sets a
could-not-determine verdict, which surfaces under its own name. A record that was
never written at all returns the answer meaning *this harness has no such tier*.
Same profile, same declaration, two different silences.

None of this is unknown to the codebase, and that is the part worth stating
plainly. The function that checks declared facts against what actually arrived
already explains this failure in prose: a declared fact absent after a driven
turn means every reader of that predicate is silently falling to the tier below
with nothing saying so. That function is reachable only from the probe, and the
probe drives a throwaway harness on its own socket. It has never looked at a
running agent. Whether that is still so is a question for `grep`, not for this
page.

So the diagnosis is written down, correct, and living in the one place that
cannot observe the thing it describes. That is worse than an absent check,
because an absent check looks absent: someone reading the probe finds the failure
explained and reasonably concludes it is handled. A declaration nothing reads is
decoration; a correct diagnosis that cannot reach its subject is camouflage.

The same shape appears again, and in this register's own words. The decision that
established tiering routed this loudness deliberately —
[ADR-0008](0008-evidence-is-tiered-per-predicate.md) says that law 8's loudness
lives one level up, where it can be loud without breaking the work, that an
expected fact which *stops arriving* surfaces as expiry, and that the probe
asserts the pipeline end to end. Every clause of that is true. It describes the
fact that arrived and then aged, and it does not describe the fact that never
arrived at all, which cannot expire because there is nothing to age. A reader
arrives at that sentence with the general question and leaves believing the
general answer.

That makes this an addition to ADR-0008 rather than a revision of it. Nothing it
decided is disturbed: the routing it chose is the routing adopted below, and what
follows fills the silence its wording covers over. It is worth noticing that the
shape reached the page that named the class — an explanation is most convincing
exactly where the reasoning is best, and that is where it is least likely to be
checked.

## Decision

**A profile's declaration of the facts it supplies is read by the witnesses it
describes, not only by the probe.** The declaration is unchanged and no new one
is introduced. What changes is its reach.

**A declared fact still absent where a lower tier witnessed the very activity it
exists to record is a disagreement, and it is reported as a finding.** It does
not change a verdict. The reader continues to take the highest tier that can
answer, exactly as it does today, and the absence still falls through — what is
added is that the fall-through stops being silent. The disagreement joins the
surface that already carries tier disagreements, beside the one running the other
way.

**A finding rather than a verdict is not the tidy choice; it is the only sound
one.** The forward direction — an owned event vetting a pane scrape — is
available because the event tier is authoritative over the scrape tier. Reversed,
the accusation runs from the less trusted witness to the more trusted one, and a
verdict change on that footing would be the lower tier outvoting the higher, which
this register has already refused: tiers pick witnesses, they do not vote. A
finding carries no such requirement. It does not ask the accusing tier to be
authoritative, only the disagreement to be real — and the disagreement here is
between a fact that was promised and activity that demonstrably occurred without
it. The disposition is what makes the reverse direction available at all.

**It follows that a fact which has not arrived yet is not a fact that never
will.** A window whose agent has done nothing has no activity for anything to
disagree with, so it produces no finding and keeps today's behaviour exactly. The
precondition is established from evidence rather than assumed, and no clock is
introduced to estimate it — a bound would answer *probably* to a question the
window can answer.

**A declaration names facts that arrive by independent routes, and does not say
which.** One is written by the hook the harness fires; another by the statusline
script, wired separately and running on its own occasions. Their failures are
therefore independent, and a single question about whether *the declared facts
arrived* is several questions wearing one name. Nothing here may treat one
arriving as evidence for another: that would be a cross-check between witnesses
answering different questions, which is not a cross-check.

**This adds nothing that watches.** No component is introduced, nothing is
scheduled, and no state is reconciled against any other state — law 7 is not
approached. An existing declaration becomes visible to readers that already run,
at the moment they already run, on the window they are already reading, and the
surface that receives the finding is one this tool already owns and an operator
already reads.

**A profile declaration is a defeasible claim, and this file already treats one
that way.** The precedent is shipped: a harness declares whether it is quiet at
rest, each profile says why, one profile deliberately declares nothing, and core
consults that declaration inside the witness it informs. Its own reasoning
records that finite observations cannot prove such a declaration forever, and
bounds how long that signal is trusted alone. This decision is the second
instance of a mechanism already in service, not a new one.

**The residual on the closed-bracket path is not reopened.** A record that exists
and reads closed ages under a bound that was chosen deliberately and argued in
place, and its remaining exposure — a working agent whose events have died still
reading idle for the length of that bound — is documented where it lives. That is
a different branch from the one this decision changes: it concerns a record that
is present, where this concerns one that is absent. Nothing here touches it, and
it is named so the next reader does not have to re-derive that it was considered.

**Two neighbouring findings look like this same shape and are deliberately not
decided here.** One is that the set of hook events Gangline may write on is
per-harness knowledge held in core rather than declared by the profile; the other
is that the probe reports no row when the harness it launched exits underneath
it. Both were reported by other agents and neither has been measured by this
author, and an ADR has no way to mark a sentence as relayed — every line in one
reads as decided, at equal weight, from then on. A claim that enters on someone
else's evidence therefore leaves as this register's own, which is a laundering
this page declines to perform on its own behalf. The general statement above is
meant to be sufficient for both: whichever of them is measured can be attached to
it by argument from evidence rather than by reopening what is decided here.

## Consequences

A harness may now be given a declaration and have its breakage reported, which is
what makes a second harness trustworthy on the same terms as the first. Until a
profile declares a fact, nothing about its behaviour changes — the undeclared
path is the path it already takes.

A profile that declares a fact it does not in practice supply becomes visibly
broken where it was quietly degraded. What changes for it is what an operator can
see, not what any predicate answers: no verdict moves, and an agent reported busy
or idle today is reported the same way after. The remedy is to declare what is
supplied, which is what the declaration is for.

Because the report is a finding rather than a verdict, it can be wrong without
costing anything but attention, and that asymmetry is deliberate. A false finding
is read by someone who can check it against the window; a false verdict is
consumed by a send. The disposition puts the cost of being wrong where a human
sees it rather than where a message lands.

The probe's rows keep their present meaning and are not made redundant. The probe
answers whether a harness *can* supply its declared facts under controlled
conditions; this answers whether a running agent's facts *did* arrive. Those are
different questions and neither substitutes for the other.

A declaration now carries consequences at runtime, so writing one becomes a
claim a profile author is held to rather than documentation. That raises the cost
of declaring and is the correct direction: the alternative on offer is a
declaration that costs nothing and means nothing.
