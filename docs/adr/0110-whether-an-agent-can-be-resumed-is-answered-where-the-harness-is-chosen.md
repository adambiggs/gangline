---
id: 0110
status: accepted
date: 2026-08-18
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0110: Whether an agent can be resumed is answered where the harness is chosen

## Context

Resume support depends separately on a native launch form and a witnessed session id.

## Decision

Declare resume launch and native-session witnessing separately in the collar, expose
both at hitch and in `gang collars`, and print a relaunch command only when the launch
form exists.

## Consequences

Resuming needs a launch line with a session slot and a collar that witnesses the native
id to put in it, and those are separate declarations that are missing differently. `gang
drop` was the first place that said anything about either — after the agent, and its
session, were already gone. `hitch` warns instead, and `gang collars` marks the
capability per collar, because `-c` is where the operator is choosing and would choose
differently for work that must survive a restart.

Treating the two as one requirement produced a false statement for each single half. A
collar with the launch and no witness still relaunches, onto an id the operator holds;
calling that `no-resume` said the session was lost for good. A collar with the witness
and no launch still stamps its id through the hook path, so a warning saying Gangline
would never learn that id was disproved by the core path minutes later — and `gang drop`
then printed a relaunch command `hitch` refuses, which is the worst of the three: an
operator's one recorded way back, quoted at the moment the agent ends, not runnable.

So each half is answered where it fails. The capability word has three values. The hitch
warning has a branch per half. And a parting line quotes a relaunch only for a collar
that declares the launch, and otherwise prints the id as what it is — a record of the
harness session rather than a way back into it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
