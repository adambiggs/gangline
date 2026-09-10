---
id: 0186
status: accepted
date: 2026-09-09
supersedes: []
superseded-by: []
tags: [delivery, replies]
---

# ADR-0186: No-reply envelopes join the next ordinary delivery

## Context

A four-day trace contained 122 correlated-reply turns and 14 stop-alert turns that
woke the lead only to establish that no acknowledgement was owed. The trace did not
preserve whether each correlated reply answered a request or acknowledged a reply,
and its sampled prose shows that action-bearing rulings can look like short
acknowledgements. Delivery therefore has to use the immutable thread modes rather
than infer intent from message text. Session and native events still cannot hide a
debt that already stands.

## Decision

Gangline defers only a correlated peer message whose matched threads are all replies,
plus its own reply-release stop-alert envelopes. This is a pure acknowledgement chain.
A message that answers any matched request remains immediate because its recipient is
waiting for that answer, even though the answer creates no reciprocal acknowledgement
debt. State and input-stall alerts remain immediate because they report conditions that
may require intervention. Native Stop feedback that refuses idle remains immediate
because it exposes an obligation already owed; every uncorrelated verified peer message
also remains an immediate wake. Supersession can inherit correlation, but any inherited
thread keeps the replacement on the waking path because the fresh classifier cannot
prove that an appended request thread carries no awaited answer.

A deferred envelope is accepted only after a transient timer is armed and its full
attributed body is renamed into the recipient's spool. Thirty minutes is the delivery
deadline, not a claim that a live competing process can be interrupted. Every callback
makes one promotion attempt even when suspend delayed its first invocation; after lock
contention or another pre-promotion failure, the deadline service retries every five
seconds for at most five additional minutes. It then stops and records an exhausted
handoff that `status` distinguishes from a service gone for an unknown reason. A missing or refused
initial timer preserves immediate delivery. If the spool cannot record the detailed
exhaustion handoff, the callback still stops and `status` reports the marker-less
service as gone for an unknown reason. Status, roster, mail, teardown, and orphan
archival expose the hidden entry and its timer health, including an overdue entry whose
retry authority is gone.

The next ordinary Gangline delivery that reaches a composer-verified drain promotes
the deferred entries under the drain's pane lock. Their original acceptance stamps
place them beside older ordinary mail before one claim loop builds one verified
delivery. A native prompt alone does not promote them: advisory hook output is not
delivery proof, and exposing held mail there would make the prompt's following Stop
create a second turn solely for acknowledgements. Each deferred body is visibly
marked. If no earlier ordinary delivery arrives, the timer promotes the whole
accumulated set and attempts an ordinary drain. Explicit live-only sends and immediate
state or input-stall alerts retain their priority semantics rather than joining the
spool transaction.

## Consequences

Durable spool acceptance still settles the request answered by a correlated reply, but
holding, promotion, and delivery do not alter any other reply record on the recipient.
An early wake leaves stale timer callbacks harmless because their exact hidden entry no
longer exists. Promotion, delivery ambiguity, teardown, and timer loss remain named
states rather than disappearance.

A collar without native hooks cannot contribute a Stop delivery opportunity; its
acknowledgement waits for another ordinary Gangline delivery, self-mail, or the
deadline service. A prompt with no ordinary queued delivery does not mint a standalone
acknowledgement turn.
The historical trace cannot quantify the newly safe subtype split. Future command
verdicts distinguish `held` pure acknowledgements from immediate answers, so production
reduction is measurable prospectively rather than claimed from the old 122-turn total.

This decision is falsified by a replay in which an uncorrelated peer message fails to
wake an idle recipient immediately, an answer to a matched request is held, the
recipient's unrelated reply-obligation query changes across a hold, accumulated
envelopes overtake older ordinary mail or arrive after the ordinary waking message
whose drain promoted them or more than once, hook stdout consumes an entry without
verified harness acceptance, retry
processes continue beyond their five-minute recovery window, or a hidden entry with no
live retry authority lacks a loud overdue handoff.
