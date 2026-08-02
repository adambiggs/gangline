# ADR-0012: Instale data is refused from documentation

- **Status:** Accepted
- **Date:** 2026-08-01

## Context

Documentation carries two kinds of statement that look identical on the page and
behave nothing alike. One is a claim about the design: *this is why the bound
sits here*. It stays true until somebody changes the decision, and changing the
decision is the event that rewrites it. The other is a measurement of the
project at a moment: how many of a thing there are, what version something was
at, how long a file runs. It stops being true as soon as anyone commits, and
nothing about that commit knows the sentence exists.

The operator's name for the second kind is **instale** — instantly stale. Not
data that goes stale on some horizon worth planning around, but data that is
stale the moment it is written, because the thing it measures moves and the
sentence does not.

It is worth naming who writes it. Prose produced by an agent reaches for
concrete-sounding specifics — counts, sizes, versions, tallies — because they
read as evidence of having actually looked. They are evidence of having looked
*once*. This page's most frequent reader is also its most frequent violator,
and the rule exists because good intentions produce the failure reliably.

### Why it is worse than absent

A missing number costs a reader one lookup. A wrong one costs them the lookup
plus whatever they built on it — and they never know to do the lookup, because
the number is sitting in a document that is right about everything else and
carries that document's authority.

**Instale data is worse than useless, in subtle and sinister ways: it is
crystallized context rot.** ADR-0006 defines rot as degradation carried by the
accumulating artifact, and that rot lives in a context window where compaction
eventually clears it. Written into a document, the same decay survives every
clearing. It is re-read as current by everyone who arrives afterwards, and it
propagates: the next reader repeats it, cites it, and builds on it.

## Decision

**A data point that is stale the instant it is recorded does not go in
documentation.** Counts of things, library and tool versions, file sizes and
line counts, tallies of tests or checks, measured durations, and any other
momentary reading of a moving system.

The scope includes historical documents. ADR History sections, changelogs and
postmortems are inside it, not outside. **"It is in a dated entry" is not an
exemption** — a date tells a reader when the sentence was written, not whether
it is still true, and readers do not perform that arithmetic. They read the
number.

### The test

**Is this still true after the next ordinary commit? If not, does it carry its
own justification on its face?**

The first question separates a measurement from a claim. The second is the only
exit, and it is narrow by construction.

### The carve-out: measurement that a decision rests on

A measured observation may stay when it is the evidence a decision was made on.
Then it must be dated, attributed to the act of measuring, and phrased as an
observation rather than as a fact about the present — *sampled on this date,
this was measured, which is why the bound sits here*. A reader can tell at a
glance that they are reading history rather than status, which is the whole
requirement.

The sorting question is load-bearing-ness: **delete the number and see whether
the paragraph's argument survives.** If the argument is intact, the number was
decoration and decoration is what rots. If the argument collapses, the number is
evidence and it stays, dated.

### The discriminators

Most of the work is telling similar-looking things apart.

- **A design constant is not a measurement.** Rung fractions, thresholds, the
  arithmetic that derives one bound from another — these *are* the decision. They
  did not read the project's state; they set it, and they change only when the
  decision changes. Recording them is recording the design, which is what a
  register is for.
- **An immutable reference is not a measurement.** A commit hash, an issue
  number, a released version being cited, the title of a published report. These
  are pointers, and a pointer cannot go stale: it keeps naming the thing it
  named. Note the trap in the middle — *"the version we depend on"* is a
  measurement, while *"fixed in that release"* is a pointer, and the two can be
  one word apart.
- **What is refused is neither of those.** It is the number that reads as
  current fact about a moving thing and is falsified by the next ordinary
  commit: how many checks a suite holds, how long a file runs, how many of some
  artifact ship today, the dimensions of somebody else's system.

The last of those deserves its own sentence, because it is the easiest one to
write in good faith. **Another project's numbers are not ours to keep current.**
Describing an external system by its dimensions imports a maintenance obligation
we cannot discharge — they change it, we do not find out, and our page is
confidently wrong about somebody else's work. Name the system and name what
about it mattered to the decision.

### Motivation, names, measurements

These neighbouring rules divide by object, and one question sorts them: **does
the thing bind future work?**

- [ADR-0010](0010-a-benchmark-is-a-consumer-not-a-design-input.md) — *motivation*
  is recordable. Why work got scheduled explains why the design is the shape it
  is, so it binds, and a register hiding it lies about its own history.
- [ADR-0011](0011-a-rename-before-publication-is-total-and-retroactive.md) —
  superseded *names* are not recordable. A name carries no reasoning of its own,
  binds nothing, and is a draft state of a sentence. A recorded prior name is
  instale residue: accurate for the length of one draft, misleading forever
  after. That page is one limb of this one.
- **This page** — *measurements* are recordable only as dated evidence. A
  measurement binds exactly when a decision rests on it, which is why the
  carve-out is narrow rather than generous, and why what it demands is a date
  rather than a promise to update.

Same discriminator, different answers. A rule that derives them from one
question is easier to apply at the moment of writing than separate rules held
side by side.

The compass over all three is **think declaratively**: state the design in
sentences only a decision can falsify — narrating process is the register every
failure here shares, and the dated historical record is where the compass
deliberately yields.

### What this is not

- **Not a ban on numbers.** Design constants, thresholds and the arithmetic
  between them are the substance of a decision page. This rule protects them by
  clearing out the numbers that make a reader distrust the page.
- **Not a ban on measuring.** Measure constantly. Report it in the conversation,
  the review, the commit message — places timestamped by construction, where a
  reader already knows they are seeing a moment. What is refused is promoting a
  measurement into standing documentation.
- **Not a licence to be vague where precision is the point.** The replacement
  for an instale number is usually a *more* precise sentence, not a hedge: name
  the mechanism instead of counting its instances. Vagueness is a different
  failure and this page does not ask for it.
- **Not bounded by publication.** Unlike ADR-0011, nothing here becomes a
  compatibility obligation once it ships. A wrong number in a published document
  is simply wrong; it gets corrected in place, whenever it is found.

## Consequences

- Review gains a third question beside the neighbouring pages': *does this state
  a measurement that the next ordinary commit falsifies?*
- A number that survives review is a design constant, an immutable reference, or
  a dated observation carrying the decision it justifies. There is no fourth
  category.
- A page describing an external system names the system and what about it
  mattered, not its dimensions.
- Writing a document costs one extra pass — a sweep for numbers that read as
  current status. It is cheap, and it is cheapest before the document is read by
  anyone.
- Instale data found in an existing document is corrected in place. There is no
  boundary past which the error is preserved for compatibility.

## History

- **2026-08-01** — operator direction, made standing the same day it was given.
  *Instale* — instantly stale — is the operator's coinage for data points in
  documentation that are stale the instant they are recorded, given with the
  observation that agent-written prose produces them constantly and unprompted.
  The instruction was that such data is forbidden from documentation except for
  the most extreme cases of justification, and that this reaches ADRs and other
  historical documents rather than stopping at them. The coinage is recorded
  here, where ADR-0011 would have erased a superseded name, because it is not a
  superseded name: it is the term the rule is stated in, and every later refusal
  cites it. That is ADR-0011's own discriminator returning the opposite answer on
  a different object, which is the point of having asked the question rather than
  memorised the outcome.
