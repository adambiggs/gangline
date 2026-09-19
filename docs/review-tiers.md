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
proves it through the repository's gate, and hitches nobody.

## Rounds

A review gets two rounds in either tier: one pass by the reviewer and the
owner's answer to every finding, twice. If a finding still stands after the
second, land with it recorded or hand it to the lead to decide; there is no
third.

## The tier line

Every assignment carries its tier on a line of its own:

```
tier: A
```

or

```
tier: B
```

`gang hitch --stdin` refuses an assignment without it; a task-only hitch
supplies `--tier A|B`, and Gangline renders the same line into the assignment.
