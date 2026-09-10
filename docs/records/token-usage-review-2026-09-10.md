# Token usage review — 2026-09-10

> Status: Completed on 2026-09-10. Tests the claims an external report on
> coding-agent token efficiency makes about Gangline, against the repository
> and thirty days of measured token use. The figures are a dated measurement,
> not a standing claim.

## The report

An external research report ("Token efficiency for frontier-model coding
agents", 10 September 2026) recommends bounded context, pruning, event-driven
coordination, selective review, and effort routing for Gangline. Its
literature claims are not re-derived here. Its claims about Gangline, and its
estimates for this fleet, are tested against the repository at `28563b8` and
against measured per-call token use.

## Verdict

The report's direction is right and most of its Gangline-specific premises are
wrong or already settled. Its largest recommended mechanism has no Gangline
surface, its cost model does not apply to this fleet, and the one leak it could
not identify is identifiable and is the single largest measured lever.

## Corrections to the report

1. **The 100k / 200k lights are not a proposal.** The fleet's operator
   configuration already sets
   `GANG_CONTEXT_LIGHTS=claude-code/*=10%,20% codex/*=collar`. On a 1M window
   that is 100k yellow and 200k red. Codex keeps its collar default of
   75%,90% of its 258,400 window (194k / 233k). Lights are advisory hook
   messages emitted once per context epoch (ADR-0033); nothing acts on them
   but the agent.

2. **The GPT-5.6 272k surcharge cannot occur here.** Every Codex call in the
   window (86,279 calls) reports a model context window of 258,400 tokens.
   No call exceeded 272k. This fleet's Codex usage is metered against a
   subscription's provider window, not per-token dollars, so the report's
   dollar tables are hypothetical here; the binding constraint is the
   provider window that `gang cap` records.

3. **The "30-second Codex poll" is real, is inference-backed, and is the
   biggest lever.** It is not a Gangline primitive: `gang wait` is a tmux
   `wait-for` barrier and is refused from inside an agent window. It is
   Codex's own `exec` tool yielding after `yield_time_ms` and the model
   re-polling the yielded command through `write_stdin` with empty input and
   a 30-second yield.
   Each re-poll is a full model call at full context. Over thirty days,
   about 28,000 of 86,000 Codex calls were such re-polls, and the calls
   that read their result carried 34.8% of all Codex context tokens.
   The commands being waited on are dominated by `test/gate.sh` and
   `test/integration.sh`; the median yielded command needed 2 re-polls, the
   p90 needed 17, the worst 204.

4. **Coordination is not 20-25% of tokens.** Classifying sessions by the name
   in their hitch message, lead plus reviewer sessions carried 18.5% of Claude
   context tokens (lead 12.5%, reviewer 6.0%) and 3.9% of Codex context tokens
   (reviewer 2.8%, lead 1.1%): 13.8% combined. Name-based classification can
   miss a reviewer with an unrelated name, so treat this as a floor near 14%,
   not a ceiling. Reviewer cost is small; the report's "blanket review is
   probably overused" is not supported on cost grounds.

5. **The lead's cost is the lead working, not the lead polling.** Across 26
   Claude lead sessions: 7,073 Bash calls, 329 file edits or writes, 1,926
   `gang send` calls (median 1,160 chars, p90 3,192). Roster and status polls
   were 8.2% of lead context tokens and 1% of the fleet's. The shipped lead
   brief already forbids both polling and producing deliverables; the
   measurement shows the prose is not holding.

6. **The 600k trajectories are real but are not mostly inbound briefs.**
   42 of 503 Claude sessions exceeded 400k context and 219 exceeded 200k; the
   largest reached 638k. ADR-0072 recorded two lanes where the larger share was
   inbound brief. In aggregate owner sessions are 56.8% tool results, 34.9%
   tool inputs (commands, file writes), 4.4% Gangline envelopes, 2.4% assistant
   text by characters. The report's premise that observations dominate holds
   for owners; ADR-0072's diagnosis does not generalise.

7. **Caching is not the problem.** Cache reads are 98.2% of tokens in the
   `gang usage` event records, 98% of Claude context tokens, and 97.8% of
   Codex input tokens. The report is right to move past this.

8. **The Stop adapter is not a poll.** It refuses idle once per turn while a
   peer reply is owed and then releases (ADR-0148 to ADR-0151). About ninety
   refusals or timeouts occurred in thirty days of Claude sessions.
   Negligible.

9. **Observation pruning has no Gangline surface.** Neither Claude Code nor
   Codex exposes tool-result masking to a user; both offer only native
   compaction, which Gangline already drives (`gang compact`, ADR-0078).
   Building a pruner, a scheduler, a risk scorer, a gate-result cache, a
   status index, or a per-call telemetry ledger would breach Constitution
   laws 1 and 7. The report's "prune continuously" lever is a doctrine line
   about bounding tool output at the shell (`| tail`, `tee` to an evidence
   file) and nothing more.

## Measurements

All figures cover the thirty days ending 2026-09-10 for one fleet and include
sessions that were not Gangline agents (about 1.4% of Claude tokens, 2% of
Codex). "Context tokens" is the prompt size of one call: input plus cache
creation plus cache read.

### Claude Code (526 sessions, 121,529 calls, 21.19B context tokens)

| Context band | Calls | Share of context tokens |
|---|---:|---:|
| 0-50k | 2,682 | 0.6% |
| 50-100k | 28,528 | 10.3% |
| 100-150k | 29,655 | 17.4% |
| 150-200k | 23,475 | 19.3% |
| 200-272k | 17,802 | 19.4% |
| 272-400k | 14,478 | 22.3% |
| 400k and above | 4,886 | 10.8% |

Calls at or above 200k carry 52.5% of Claude context tokens. Log-log slope of
session context tokens against call count is 1.34 (445 sessions with at least
20 calls): a session's context tokens grow about as `calls^1.34`.

Tokens above a hard per-call cap, holding call count fixed, are the ceiling on
what rotation alone can save before any reconstruction cost:

| Cap | Excess share of Claude context tokens |
|---:|---:|
| 100k | 46.5% |
| 150k | 28.8% |
| 200k | 17.4% |
| 272k | 8.2% |

Compaction: 186 compactions; context at the last call before one was median
215k (p10 143k, p90 407k). The first call after compaction was median 54.6k
(p10 47k, p90 60k), which is the per-call floor this configuration pays on
every call: system prompt, tool definitions, imported instructions, memory,
and the summary. Regrowth to 100k took median 60 calls.

### Codex (533 sessions, 86,279 calls, 10.11B context tokens)

| Context band | Calls | Share of context tokens |
|---|---:|---:|
| 0-50k | 12,871 | 4.0% |
| 50-100k | 21,161 | 16.0% |
| 100-150k | 24,897 | 30.8% |
| 150-200k | 21,956 | 37.7% |
| 200-272k | 5,389 | 11.4% |
| 272k and above | 0 | 0% |

Compaction: 434 compactions; first call after one was median 21.8k.

### Role class

| Class | Claude sessions | Claude share | Codex sessions | Codex share |
|---|---:|---:|---:|---:|
| owner | 372 | 80.1% | 440 | 94.1% |
| lead | 26 | 12.5% | 5 | 1.1% |
| reviewer | 84 | 6.0% | 47 | 2.8% |
| not a Gangline agent | 44 | 1.4% | 41 | 2.0% |

### Polling

| Pattern | Calls | Context tokens on the issuing and reading calls |
|---|---:|---:|
| Claude `gang roster/status/explain/mail` | 1,750 | 3.0% of Claude |
| Codex `write_stdin` re-poll of a yielded command | ~28,000 | 34.8% of Codex |

Claude agents waiting on long jobs used `until ...; do sleep N; done` loops
inside one Bash call, which block without inference up to the tool timeout.
That pattern is cheap. The Codex pattern is not.

## Levers, ranked by measured size

1. **Stop Codex re-polling the gate.** Up to about a third of Codex context
   tokens. The agent's choice, not the harness's default, is to spin
   `write_stdin` every 30 seconds; the gate takes minutes. Candidate
   substrate-free remedies: run the long command detached to an evidence log,
   end the turn, and arm one `gang at <duration> --to <self>` wake; or have
   the detached job `gang send` its exit status to the agent. Neither has been
   smoke-tested from inside an agent window; test before writing it into
   doctrine or the Codex collar's prompt guidance.
2. **Hold the Claude red light as rotate-not-resume.** Ceiling 17.4% of Claude
   context tokens at 200k, 28.8% at 150k, before reconstruction cost. The
   configuration is in place; what is unmeasured is whether agents act on the
   light. Measure by re-running the band histogram in two weeks.
3. **Lead conduct.** 12.5% of Claude tokens sit in lead sessions that edit
   files and run thousands of shell commands. The shipped brief already
   forbids this; the lever is the brief's wording and the lead's model and
   effort choice, both launch-time prose.
4. **The ~55k per-call floor on Claude.** Roughly a quarter of Claude context
   tokens is fixed prefix repaid on every call. It is cached and cheap per
   token, but on a subscription the meter is the provider's, not the API's.
   The operator controls the imported instruction files and enabled tool
   surface; Gangline controls only `CONTRACT.md` and the role brief.
5. **Review policy.** Reviewer share is 6% or less. Leave it.

## What remains unproven

- Whether cache reads count toward a subscription's usage window at all, and
  at what weight. Nothing measured here answers it, and it decides whether
  lever 2 or lever 4 matters on a subscription.
- Any denominator. No accepted-change count exists, so nothing here is a cost
  per accepted change.
- Whether a Codex agent can end its turn and be woken by its own detached job
  through Gangline without a second agent involved.
- The reviewer share, which name-based classification can undercount.
