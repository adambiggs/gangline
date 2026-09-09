---
id: 0014
status: accepted
date: 2026-08-04
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0014: MCP may be a face but not the transport

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Add an MCP wrapper only for a real consumer that cannot use the CLI.

## Consequences

MCP does not universally start a turn in an idle native harness, while tty input does;
agents remain free to use MCP tools without Gangline mediating them.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
