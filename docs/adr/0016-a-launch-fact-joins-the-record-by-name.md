# ADR-0016: A launch fact joins the record by name

- **Status:** Accepted
- **Date:** 2026-08-03

## Context

[ADR-0015](0015-a-context-is-renewed-by-cycling-not-by-summary.md) made renewal
replay the facts a window was launched with. The facts asked for next are new
ones — a model or a profile chosen at the moment of renewal, a reasoning effort,
context bands set per agent rather than per install — and nothing says how a
fact joins the record at all.

The record already answers part of it. Each fact is its own window option, and a
separate option vouches for them by carrying the format they are written in.
Packing them into one value was considered and refused on the record, because a
working directory may contain whichever separator is picked, and a dir that
truncates comes back as a live agent running somewhere other than where it was.

What remains is not obvious, and the mistakes below were reached for during this
design rather than imagined for it. Bumping the format for a fact that merely
joins refuses every currently-live agent, mid-task, for facts sitting on their
windows intact and readable. And giving a new fact its own writer standing
beside the existing one puts that write *after* the vouch.

## Decision

**A new fact takes its own option name, and joins without bumping the format if
and only if its absence is its default.** A window recorded before the fact
existed then replays truthfully — absent is what that window actually had, not a
value invented for it. A fact that changes how an existing option is *read*
fails that test and must bump, because there the old bytes mean something else.

**Nothing the vouch covers is written after it.** The vouch goes on last, after
every fact it stands for, so a hitch that dies partway leaves facts with no
record — and an absent record is refused. Written any earlier, that same partial
write reads as a complete one. This is what bounds the rule above: absence is
truthful *inside* the vouch, and outside it the same rule launders a failed
write into a legitimate default, which is the fabricated status law 8 forbids.

**Each fact is named where it is written, not implied by position.** The facts
are strings of the same shape, so an ordered list of them cannot tell a
transposition from an intention: swap two and the record says a model is a role
and reports nothing wrong. A name carried beside each value moves the two
together; a name that is not a fact is refused, which is what makes a
transposition loud, because the name slot then holds a value. A refusal there
leaves facts without a vouch — the safe direction, and the reason the guard may
sit inline rather than ahead of the writes.

**A value whose spelling a profile owns does not cross profiles.** A recorded
model is spelled for the harness it was hitched on, so replaying it under
another one produces a model name that harness never had. Renewing onto a
different profile is refused while such a value is recorded, unless the same
command supplies one for the profile being moved to. Where the new profile
declares no option to carry it, the existing refusal already covers it and is
routed through rather than restated.

**Refused (laws 1, 4, 5 and 8):** no packed record; no table translating a
harness-owned value into another harness's spelling, which would be harness
knowledge in core and wrong the day a harness renames anything; no general
record framework — the shape above is the whole of it; no format bump for a
fact that only joins; and no silent default where a value was asked for and did
not take.

## Consequences

- The profile mark is not free to reshape the way the other facts are. It is
  both a launch fact and the mark that makes a window an agent — read across the
  roster, resolution, status and the hook dispatch, written by adoption as well
  as by hitch, and removed by retirement. The rest of the facts are touched only
  by the record's own writer and reader.
- Replaying a fact is not the same as honouring it, and a harness that accepts
  a value it does not know establishes nothing by starting. One runs on at its
  own default; another opens a window and is refused a turn later, by the
  provider rather than by the harness. Either way the record claims what was
  asked for while the agent is something else. So a fact of that kind has its
  validity established before the window opens, where the cutoff and the
  directory are already checked — a refusal raised after that point leaves a
  live agent behind for a word Gangline never accepted, and how to check it is
  the profile's to declare, because the vocabulary can be the harness's own and
  can narrow with the model chosen.
- The day-one consumer (law 5) is a model named at renewal: the mechanism lands
  with it rather than bare.
- ADR-0015's replayed set grows with whatever facts land; `--resume` and
  `--cutoff` remain refused replay for the reasons given there.
