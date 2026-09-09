---
id: 0059
status: accepted
date: 2026-09-03
supersedes: []
superseded-by: []
tags: [ticks]
---

# ADR-0059: Stale tick ownership is retired only from proven process identity

## Context

Cooperative retries must make progress without creating a resident coordinator or
unbounded owner.

## Decision

published worker deadline fails health; generation locks measure that budget in the
deadline controller's monotonic clock domain, so suspend and wall-clock steps cannot
spend it.

## Consequences

After one more deadline interval, Linux may SIGKILL only the pidfd-bound leader
generation, confirm its death, and retire the lock. Legacy pid-only locks migrate only
where the live PID is positively not a tick worker for this team and never authorize
termination because they carry no generation or monotonic acquisition stamp. A pid names
a process only inside one pid namespace, and a harness sandbox with its own pid table
shares the lock directory with the host, so the record also carries the owner's
namespace. A contender in the initial namespace resolves that owner through `/proc` and
reclaims one whose namespace holds no such process, but only where that `/proc` is
procfs for its own table with no hidepid mode that filters this user's entries: absence
read through any narrower view proves nothing. A contender that cannot see the namespace
retains the lock and names it. A record that predates the namespace field is resolved by
pid and start token among the visible namespaces, since its writer may have been in a
sandbox. A recorded pid whose process belongs to another user is retired as a reused
number: every tick worker runs as this user and proved its own identity readable at
acquisition, so unreadable means not ours, never alive.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
