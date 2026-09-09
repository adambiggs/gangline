---
id: 0026
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0026: A refused delivery is parked, a failed one is not

## Context

Delivery needs distinct outcomes before and after keystrokes reach a harness.

## Decision

A refusal happens before any keystroke, so the body is still the sender's and parking it
loses nothing; a failure after a paste has an unknown fate, and a second copy of a
message that may have landed is worse than one loud failure. Parking is the default and
`--live-only` is the explicit probe. A collar with a native Stop event drains there
immediately; a `steer` collar may also drain at PostToolUse when its composer is free,
after attribution has committed the entry. `gang tick` supplies the bounded cooperative
pass, and other operational commands launch it after preserving their own results, so
hookless collars can park pre-keystroke refusals without a resident poller, scheduler,
or watcher. Missing hooks still mean missing native facts, not missing retry. An entry
is claimed out of the spool before it is delivered, because ownership has to span the
submission AND the retirement: the pane lock is released inside the delivery, so
anything still live afterwards could be sent again by the next drain or the next
boundary. An entry whose delivery could not be verified, or whose drain died mid-flight,
is held and counted rather than re-sent; Gangline never sends a message a second time on
the chance the first did not arrive, and never holds one without naming it. Supersession
is the sender's explicit flag and reaches only that sender's own earlier messages. A
window's spool identity is minted at hitch and adopt, where nothing can race it —
minting it when a message needs parking would let two senders mint two and strand one of
their messages in a directory nothing points at.

## Consequences

A drain never gets a weaker delivery predicate. A readable obstruction after a boundary
is an ordinary refusal and remains queued; a boundary that still exposes no readable
composer records a drain failure while leaving every entry unclaimed. Silently spending
that unreadability as another healthy retry would strand the same queue at every later
boundary while status claimed only that it was waiting.

Revisit if a supported-host test safely retries a post-keystroke failure.
