# ADR-0010: A benchmark is a consumer, not a design input

- **Status:** Accepted
- **Date:** 2026-08-01

## Context

An external evaluation is now a project goal, and that changes the pressure on
every decision taken while it is in view. The hazard is not the temptation to
cheat — that one is easy to see and easy to refuse. It is drift. A scoring
target supplies numbers, a shape, and a schedule of its own, and those become
the most available inputs in the room. A default gets picked because it matches
what one caller happens to send. A bound gets tuned until one caller's runs come
out right. A justification gets written naming that caller, because it is the
true reason and honesty appears to demand it. None of those is cheating. All of
them leave the tool fitted to a caller that will eventually leave.

Two laws already point this way and neither one closes it. Law 5 asks for a live
consumer, and a benchmark is a *superb* consumer — live, measurable, paying out
in a number — so it can satisfy law 5's letter for very nearly anything. Law 9
asks growth to justify itself against the mission, and a scoring target is
easily mistaken for the mission, being the only part of the mission with a
number attached. The laws are not wrong here; they are underdetermined against a
consumer this attractive, and the gap wants naming.

There is a second reason, independent of design quality. A tool fitted to the
thing measuring it is no longer the thing that got measured. Whatever the score
then says, it is not a claim anyone can carry outside the benchmark — us
included. Generality here is not modesty. It is the only condition under which
the number means anything.

## Decision

**A benchmark may consume Gangline. It may never shape it.** Every change is
justified by the general case, and reaches the tool through the same surfaces
any operator uses.

### The test

**Write the justification without naming the benchmark. If it does not stand up
on its own, it is gaming** — however correct the code turns out to be.

It is cheap, it is mechanical, and it runs before the code does. It is also the
whole rule: everything below is that one sentence applied to cases that have
already come up.

### Motivation is recordable; justification must stand alone

These are different objects and the rule binds only one of them. *Why the work
got scheduled* is a fact, and a register that hides it is lying about its own
history — this page names its motivating benchmark in History, and so does
ADR-0009. *Why the design is what it is* must hold with that name deleted. Work
may be motivated by anything at all; it is justified generally or it is not
justified.

The practical form: strike the benchmark's name out of the commit message or the
ADR. If the reasoning survives intact, it was general. If striking it leaves a
hole, the design was aimed and the aim was concealed.

### What fails the test

- **Numbers taken from a benchmark's shape** — budgets, sizes, timeouts,
  thresholds — hardcoded anywhere, *including as a default*. A default is the
  most durable hiding place a borrowed number has, because nobody re-derives
  one. ADR-0009's "there is no default; an undeclared budget produces silence,
  not a guess" is this rule already applied to one axis.
- **Bounds tuned until one caller's runs come out right**, rather than derived
  from the argument that says where they belong. A rung that cannot be defended
  from the general case is a rung fitted to a caller.
- **Detection of any kind.** An environment variable, a task layout on disk, a
  container fingerprint, the presence of a particular runner — anything that
  lets the tool notice it is being measured and behave accordingly. This line
  takes no exceptions, because a tool that behaves differently when observed is
  reporting a status true only while observed (law 8), and the measurement then
  measures the detector rather than the tool.
- **Core knowledge of a benchmark's artifacts** — what a verifier is, what a
  task manifest is, what a runner's config looks like. An adapter may hold that
  knowledge. `bin/gang` and the profiles may not.
- **Reading, parsing, or reacting to hidden tests or reference solutions. At any
  tier, for any reason.** They are the answer key. A run that touched them
  measured nothing — and, the reason this is a rule rather than a caution, a
  tool that *can* touch them cannot afterwards prove that it did not. The only
  defensible position is code that never learned the path.

### Every gap names a consumer that is not the benchmark

Law 5 pointed at scope rather than at dead code. Before a gap is closed, name
who else wants it: an operator, a lead, a teammate, a documented workflow. Most
gaps worth closing have several — a wall-clock budget belongs to a sprint, a
demo cut and a paid hour long before it belongs to any scored run.

A gap whose only consumer is the benchmark is **a finding to report, not a thing
to quietly build.** It may still be worth building; that is a decision somebody
makes deliberately, in the open, and records here — which is the outcome this
rule exists to protect.

### The trap

The subtle failure is not a benchmark-shaped change. Those are visible and they
get caught. It is **a correct, general change carrying a benchmark-shaped
justification.**

The code is fine the day it lands. What the next reader inherits is the
reasoning, and reasoning is load-bearing: a reader who finds "we need this
because the benchmark does X" has learned that X is what this code serves, and
extends it toward X. Three such extensions later the machinery is benchmark
machinery, and no single commit did it. The justification is the part that ages
into a design, which is why the test is aimed there and not at the diff.

### What this is not

- **Not a refusal to be measured.** Entering an evaluation is how a claim like
  "drives long-horizon multi-agent sessions with minimal machinery" gets tested
  instead of asserted. The rule exists so that the result means something.
- **Not a ban on an adapter.** An adapter is a legitimate consumer living
  outside the core: it translates a caller's facts into surfaces an operator
  already uses, and holds no machinery of its own. ADR-0009's worked example — a
  task budget passed through `--cutoff` exactly as a person would type it — is
  the shape. An adapter that needs new core behaviour in order to work is not an
  adapter.
- **Not suspicion of benchmark-motivated work.** The motivation is fine, and
  frequently productive; scoring targets are good at finding real gaps. The
  justification is the part that gets audited.
- **Not about one benchmark.** Any scoring target is the same hazard — a
  leaderboard, a demo that has to land, a customer's acceptance script, a number
  someone will screenshot. This page is written to outlive the entry that
  prompted it.

## Consequences

- A proposal states the consumer it serves, and that consumer is not the
  benchmark. One with no other consumer is reported rather than built.
- Review gains a question: *does this justification survive deleting the
  benchmark's name?* It applies to commit messages and ADR pages, not only to
  code.
- No core code learns a benchmark's shape. An adapter, should one be built,
  lives outside `bin/gang` and passes values through documented surfaces.
- ADR-0009's "not benchmark machinery" bullet stays where it is. It is this page
  applied to the time ladder, sitting on the page a time-ladder proposal reads.
- A proposal refused on these grounds is refused by citation, so the refusal
  stays reachable without knowing which review it happened in.

## History

- **2026-08-01** — operator direction, given the day the long-horizon benchmark
  entry (Long-Horizon Terminal-Bench: 46 tasks, wall-clock budgets,
  artifact-graded) became a goal: nothing built here may be benchmark-specific
  in a way that could be misconstrued as gaming the benchmark. The test, the
  named failure modes and the named-consumer discipline above are that direction
  recorded as standing rules rather than as a briefing that expires with the
  session it was given in.
- **2026-08-01** — ADR-0009 carried the constraint first, as one bullet on a page
  about the time ladder. An implementation-state assessment of that page observed
  that the constraint was repo-wide in force but reachable only by inheritance
  from a page about something else, and flagged it as wanting a home of its own.
  ADR-0005 is the precedent for what happens otherwise: a decision reachable only
  by knowing where to look is a decision that gets silently reopened by whoever
  did not know. The register is what a proposal searches, so this is where the
  constraint lives.
