# Review tiers

Every result is reviewed before it lands. How far that review reaches is a
property of what the result touches, not of how large it felt to write, and it
is fixed when the work is assigned rather than argued when the work is done.

## Tier A

Tier A results get a reviewer in a harness other than the owner's, hitched for
the review and dropped when it ends. A second harness over the same model
shares the blind spot the review exists to escape, so the reviewer's model
differs too.

A result is Tier A when it touches:

- a security boundary, or anything that decides what an agent may reach;
- delivery, locking, or anything else that decides whether a message or a write
  happened exactly once;
- a deploy runner, or any path that acts on a machine outside the checkout;
- anything invoked with `sudo`, and anything an operator must finish by hand.

## Tier B

Everything else is Tier B. The owner reviews it with its own subagents and
proves it through the repository's gate. No teammate is hitched for a Tier B
review, and the review is not a separate arc.

## Rounds

A review gets two rounds in either tier. A round is one pass by the reviewer
and the owner's answer to every finding it raised.

If the second round still leaves a finding neither side can settle, the result
does not get a third round. Either it lands with the finding recorded against
it, or the owner hands the disagreement to the lead and the lead decides. An
unbounded review is a stalled arc, not a careful one.

## The tier line

Every assignment carries its tier on a line of its own:

```
tier: A
```

or

```
tier: B
```

The line is written by whoever assigns the work, above or below the task text
and before any evidence pointers. An assignment that arrives without it is
incomplete, but it is not a reason to stop: ask the sender for the tier and
begin the work as Tier B, then say in your report that no tier was given and
which one you applied. A tier chosen by the owner is the judgment the tier
exists to remove, so the sender's answer replaces yours whenever it arrives —
including after the review has already run, when Tier A means running it again
outside your harness.
