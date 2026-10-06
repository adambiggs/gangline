# Design

See [internals](internals.md#design-principles) for the design principles and
the decisions that follow from them.

## Native usage windows and scheduled wakes

Provider cap notices use native usage readings already collected for
`gang limits`. The account-wide window is keyed by collar and native reset,
so concurrent agents do not each page the lead. Unknown windows do not
trigger a notice. A wake is an agent-authored durable message intent, submitted
through the ordinary delivery path at or after its due time. This keeps
delivery and retry semantics aligned with other Gangline messages. The team
state root is the schedule's source of truth; a host scheduler does not
restart a stopped team. On a later startup, an overdue wake goes to its
caller if active, otherwise to the lead.
An exact native submit witness records that the harness received the wake.
The wake completes only after a matching successful native turn boundary.
An attributable cap failure creates a replacement wake at
the next native reset while preserving the note.
Curfew deadlines notify each active agent without dropping registrations or
refusing startup or attach. Unknown turn outcomes remain
pending. A missing recipient after restart moves an unconfirmed wake back to
the due queue for the caller or lead.
Usage-band notices carry measurements only by default. An attributable native
cap failure creates an automatic wake even when no manual wake was scheduled.
If the reset is not yet observed, the parked wake waits for a native reading.
