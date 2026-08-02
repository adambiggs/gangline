# ADR-0011: A rename before publication is total and retroactive

- **Status:** Accepted
- **Date:** 2026-08-01

## Context

Naming happens by iteration. A term is chosen, used for a while, and replaced by
a better one — because the first was imprecise, or collided with a word already
doing other work, or read wrong to someone who had not been in the conversation.
That is ordinary drafting, and it happens to most pages before they settle.

The instinct at that moment is to be helpful and record the change: a note that
the term used to be something else, a line listing what was deliberately left
alone, a commit subject naming both. The instinct is honest and it is wrong.
What it produces is a project documenting two vocabularies where it has one, and
a reader who cannot tell that the first was abandoned rather than reserved.

The cost is asymmetric, which is what settles it. A breadcrumb helps exactly one
reader — the one who already knew the abandoned term and is looking for its
replacement — and that reader is by construction someone who was already here.
Everyone who arrives afterwards pays for it.

## Decision

**A rename before publication is total and retroactive. The final name was
always the name.**

Every artifact under the project's control reads as though the surviving term is
the only term that ever existed: pages, guides, indexes, and commit messages and
diffs wherever that history has not left the machine. No "formerly", no
renamed-from notes, no exclusion entries naming what was not renamed, no
compatibility breadcrumbs.

### The test

**A search for the abandoned term returns nothing in the sense that was renamed
— and no surviving artifact lets a reader reconstruct that a rename happened at
all.**

The second half is the one that gets missed. The search comes back clean while a
History entry two sections down narrates the whole change, and the page has
failed while looking like it passed. The standard is comparative and it is
strict: a page that passes is indistinguishable from one written under the final
name from birth.

The first half is bounded on purpose. An abandoned term is often an ordinary
word doing unrelated work elsewhere in the tree, and those uses stay — they were
never the term. They are also not evidence of anything once the rename is
unrecorded, which is the part worth noticing: **erasure is what makes surviving
hits legible as ordinary usage.** A reader who cannot reconstruct that a rename
happened reads them as the plain English word, attached to nothing.

### The boundary: retroactivity ends at publication

Past that line the artifacts stop being drafts. Pushed history someone has
pulled, a released flag someone scripts against, a name recorded in an
operator's own notes or an external tracker — rewriting those is either a lie
about what people already saw or a break in something that works, and the
principle yields. The ordinary deprecation machinery takes over there, and it
records the old name *because* the old name has become a real obligation.

Before that line, erasure is just editing a draft, and a draft owes no history.
The line is publication, not merge: work sitting on a branch nobody has pulled
is still a draft in every sense this page cares about.

### Names are drafts; decisions are records

[ADR-0010](0010-a-benchmark-is-a-consumer-not-a-design-input.md) holds that
motivation is recordable and that a register hiding its own history is lying
about it. That page and this one describe different objects and do not collide.

A **decision** is an event. Something was weighed, something was chosen, and the
reasoning binds future work — which is exactly why erasing it would be a lie,
and why the register exists. A superseded **name** is not an event. It is a
draft state of a sentence, the same class of thing as a paragraph rewritten
before anyone read it, and no register records those either.

So a rename before publication is an edit in place, not an event. Nothing was
decided that a future reader needs; the page simply says what it means, as it
should have from the start. Every decision the page carried before the rename it
still carries after, in the final vocabulary.

The question that separates the two: **does it bind future work?** A decision
does — *this is why the bound sits here, so do not move it without this
argument*. A name does not. The term carries no reasoning of its own, and a
reader who only ever knew the final one has lost nothing.

### Why the breadcrumb is expensive

A recorded old name is not inert. It is a live alternative sitting in the
register with an implicit case for itself, and readers do three things with it:

- **Argue with it.** A term that appears on the page is a term that can be
  preferred, and a settled question reopens on the strength of a footnote.
- **Resurrect it.** New code gets written under the recorded name, because it
  was right there and read as legitimate. The project then has both terms, which
  is the state the rename existed to end.
- **"Finish" it.** An exclusion entry listing what was deliberately *not*
  renamed reads, to anyone who did not write it, as an unfinished job. The
  considerate next contributor completes the sweep and renames things that were
  never the term.

The third is why exclusion entries are refused by name. They are written as a
guard, and they are the breadcrumb most likely to cause the harm they guard
against: a list of surviving hits is an invitation to act on them.

ADR-0010's trap applies here in its own vocabulary. The justification is the
part that ages into a design. A note explaining why one term replaced another
teaches a future reader that the distinction is live and load-bearing, and they
extend accordingly — until there is a compatibility argument in the codebase
that nobody ever decided to have.

### What this is not

- **Not a licence to rewrite published history.** The boundary above is the
  whole of it. Once something is out, the old name is a fact about the world and
  gets the ordinary treatment.
- **Not a claim that renames are cheap.** Total is expensive on purpose. The
  cost is the argument for naming carefully the first time, and for doing a
  rename before publication rather than discovering the need after.
- **Not applicable to decisions.** A page may record that it weighed two designs
  and chose one. That is not a name, and hiding it would be the lie ADR-0010
  refuses.
- **Not a ban on discussing a rename while it happens.** A reviewer needs to
  know what changed in order to check it, and that conversation is necessary.
  What must not survive is a durable artifact recording it. Take a fictional
  case: a review comment reading *"this used to be `spool` — confirm every call
  site moved"* does its job and then goes away with the review. The same
  sentence in a History entry does not.

## Consequences

- Review gains a question beside ADR-0010's: *does any artifact record a prior
  name?* It applies to pages, indexes, guides, commit subjects and diffs, not
  only to prose.
- A rename's own commit is written in the final vocabulary and describes the
  resulting state rather than the transition. Where unpublished history already
  carries the abandoned term, rewriting that history is part of the rename, not
  a separate cleanup.
- Exclusion entries are refused. Surviving uses of the abandoned word in an
  unrelated sense are left alone and left unmentioned.
- Naming carries more weight up front, which is the intent. The cheap moment to
  argue about a term is before it spreads; the expensive one is after
  publication, where this page stops applying.
- A proposal to reintroduce an abandoned term arrives with no history to lean
  on, and is argued on its merits like any other new proposal.

## History

- **2026-08-01** — operator direction, recorded as standing the same day it was
  given. The occasion was ordinary: a term settled late in drafting, and the
  question was what the register owed the term that did not survive. The answer
  was nothing, and the page above is that answer generalized. The motivation is
  stated here in general terms only — which is this page passing its own test,
  since a History entry naming the pages or the words involved would be the
  artifact it refuses.
