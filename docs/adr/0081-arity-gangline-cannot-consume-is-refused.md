---
id: 0081
status: accepted
date: 2026-08-12
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0081: Arity Gangline cannot consume is refused

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Every command names the argument it will not accept and exits non-zero. An argument
taken and discarded reports success for a request nobody made: the caller reads the
answer as the answer to what they typed. The guard is a table of one-argument-too-many
probes whose expected text is each command's own refusal, compared for completeness
against the dispatcher's case arms, so a command cannot be added without one and a probe
cannot pass on an unrelated failure. A probe's containment must hold under the
regression it exists to catch, and must not be read by the thing whose regression it
contains: a nonexistent agent name contains `drop`, but it is what `hitch` and `up`
create, and a working directory passed in argv is dropped by the same parser fault it
guards against. Those two are contained from the environment, which no argument parser
can discard, with the argv containment kept as the nearer of two. A containment is
asserted to be absent by reading an inventory, never by driving the lifecycle command
whose safety is the thing in question — a check that acts in order to discover whether
acting was safe has already done the damage in the one case it exists to catch.

## Consequences

`gang hook` is the one command that records rather than dies. Its event is the payload
on standard input, so argv is an invocation it cannot read; its caller is a harness
configuration and law 7 keeps hooks non-fatal. It therefore names the argument on
stderr, stamps the window where `status` and `roster` surface it, and declines the
event. Declining is the refusal — processing the event while discarding the argument is
the silent acceptance the rule forbids.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
