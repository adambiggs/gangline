# ADR-0002: MCP is a face, not a transport

- **Status:** Accepted
- **Date:** 2026-07-27

## Context

ADR-0001 already places MCP: it qualifies as a universal surface under law 1,
and a typed MCP face for Gangline's verbs lands when a consumer wants it (law 5).
What that bullet does not record is the rest of the fork — whether MCP could
carry Gangline's core function instead of merely fronting it. The question
recurs ("should we be leveraging MCP?"), so the answer is written down once.

"Leverage MCP" is four different proposals, and they have four different
answers.

## Decision

**As the message transport: never — structural, not taste.** MCP requests are
client-initiated; the agent is the client. Whatever server→client affordances
the protocol carries, no harness turns an inbound MCP interaction into a new
turn in the agent's own conversation loop. The only thing that starts a real
turn with the agent's full context is user input, and the only universal way to
synthesize user input is the tty. Waking an idle agent is Gangline's core function;
an idle agent runs no loop and can poll nothing. This fork is closed on
structure, not preference, and does not reopen with a better proposal — it
reopens only if harnesses grow a native inbound channel (see watch item).

**As a face over Gangline's verbs: as ADR-0001 left it.** Typed send/roster/wait/
capture tools land when a consumer wants them. Until then, every harness
already shells out to `gang` — the CLI is the universal surface (law 1), and a
second door to the same room needs a consumer that cannot use the first door.
Registering a server in three harness config formats, holding schemas current,
and paying per-agent context for idle tool definitions is integration surface
with no live consumer (law 5).

**As the agent substrate (`codex mcp-server`, `claude mcp serve`): not for the
peer tier.** Those modes expose a harness as request/response task execution —
a fine shape for fire-and-forget work, which each harness's own subagents
already provide. Gangline exists for the other tier: long-lived, steerable,
observable teammates. Headless engines lose the glass — attach to any window,
watch mid-flight, type over an agent, take control — which is the property the
whole system is built to keep.

**Inside agents: not Gangline's business.** Gangline agents use MCP servers as
tools freely. That is the capability layer; Gangline is the coordination
layer, and it does not mediate what tools a teammate loads.

## Consequences

- Proposals to "switch Gangline to MCP" get this ADR, not a redesign cycle.
- Watch item, not work item (law 5): vendors are separating UI from engine —
  Codex ships experimental `app-server`, `remote-control`, and a TUI `--remote
  <ADDR>` flag. If a native external-control channel stabilizes and can start
  a turn in a live session, a profile may declare it as that harness's
  transport under law 4. Gangline's verbs — spawn, send, capture, compact, roster
  — do not move; only the delivery mechanism behind them would.
- The MCP face, if a consumer ever wants it, is additive: a thin typed wrapper
  over the same CLI, not a parallel implementation.
