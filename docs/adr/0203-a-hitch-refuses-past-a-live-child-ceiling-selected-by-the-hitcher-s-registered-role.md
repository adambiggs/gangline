---
id: 0203
status: rejected
date: 2026-09-17
supersedes: []
superseded-by: []
tags: [roles, hitch]
---

# ADR-0203: A hitch refuses past a live-child ceiling selected by the hitcher's registered role

Rejected: no incident of a child outliving its work was ever found, and 13 of
36 recorded hitches overrode the ceiling.

## Context

An agent is a whole harness with its own subagents, so a second window is the
expensive way to get parallelism and the easiest to leave behind: children
outlive the work they were opened for and nothing in the substrate notices.
Bounding them needs one fact about the hitcher that Gangline can prove, and the
window name is not it — an agent called lead is not the lead. This proposal is
aimed at accepted ADR-0005, which forbids recording a role in a window option,
reading one later, and varying output by it. It does all three.

## Decision

Gangline records the role a hitch was launched with on the agent's window, and
`hitch` refuses when the hitcher already holds that role's number of live
registered children. The operator is unbounded, as is a role the configuration
names by neither its own entry nor `*`. A child that has marked itself safe to
drop still counts, and an unreadable window registry refuses rather than reading
as zero. An override carries the hitcher's reason into the event stream. Nothing
still checks whether an agent behaved as its brief describes.

## Consequences

The recorded role is a launch fact of the same class as the model and effort
already kept on the window, not an account of how the agent behaved. ADR-0001's
ban on managing work allocation therefore stands: the ceiling bounds windows and
never what is done inside them. ADR-0005 decides four things and this breaks
three, so its fourth is restated above rather than left to a tombstone that
would drop it.

Agents hitched before this lands carry no role and are bounded as unnamed ones.
A resume without the role re-registers it empty, as the model and effort already
do, which tightens a guard where those only dropped a brief. `adopt` stamps the
same provenance without consulting the ceiling, so an agent at its limit can
still hold children it did not hitch.

The falsifier: hitch from a window whose recorded role the configuration bounds,
already holding that many live children, and watch a window open.
