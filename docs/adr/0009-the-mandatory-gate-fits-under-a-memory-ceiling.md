---
id: 0009
status: accepted
date: 2026-08-13
---

# ADR-0009: The mandatory gate fits under a memory ceiling

## Context

One `shellcheck` over the whole repository reached 6.1 GB. On 2026-08-12 the OOM
killer took it twice on an 11.6 GB host, which was power-cycled hours later
after a starvation livelock. A gate every agent must run cannot be the largest
allocation on the machine.

## Decision

The gate must pass under

```sh
systemd-run --user --scope -p MemoryMax=2G -p MemorySwapMax=0 -- test/gate.sh
```

`test/lint.sh` runs one `shellcheck` per file, and a test file that grows until
it alone will not fit is split into sourced parts.

## Consequences

Sourced parts keep the suite one program with one set of fixtures and counters.
shellcheck cannot see across a part boundary, so a variable that crosses one
carries a directive naming the file at the other end.
