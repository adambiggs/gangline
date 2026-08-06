# Benchmark selection

This page records which external evaluations can credibly test Gangline and why.
It is selection guidance, not a source of product requirements. Benchmarks
consume Gangline; they do not shape the core.

## Portfolio

No single score proves Gangline's value. Use a small portfolio chosen for
evaluator integrity, genuine horizon length, reproducibility, and direct
relevance to persistent native-harness work.

| Benchmark | Use | Why it belongs | Limitation |
|---|---|---|---|
| [SWE-Milestone](https://swe-milestone.com/) | Primary controlled test | Its [ICML paper](https://arxiv.org/abs/2603.13428) evaluates one persistent codebase through dependent milestones. Mistakes, regressions, context pressure, and technical debt carry forward. Evaluation is separated from the agent and its [quarantine](https://github.com/DeepCommit-ai/SWE-Milestone/blob/main/docs/quarantine.md) fails closed. Native Claude Code and Codex adapters can resume subscription-backed sessions. | Release histories may resemble pretrained material; quarantine must cover every native web surface. |
| [MirrorCode](https://epoch.ai/MirrorCode) | Marquee ultra-long-horizon demonstration | Epoch AI and METR evaluate autonomous reimplementation of whole programs with hidden end-to-end tests, public trajectories, red-team work, and a private test set. Its coherent tasks can run far beyond one context window. | The released harness uses an Inspect agent and scaffold-managed compaction. Native controls and Gangline need equivalent adapters before comparison. Target-source memorization remains possible. |
| [RE-Bench](https://metr.org/blog/2024-11-22-evaluating-r-d-capabilities-of-llms/) | Independent generality check | METR created original research-engineering environments, collected matched human-expert baselines, and published trajectories. Continuous objective scores reward sustained experimentation rather than a single lucky patch. | GPU-heavy, ML-specific, and not natively adapted to subscription harnesses. |
| [SlopCodeBench](https://www.scbench.ai/) | Conditional code-quality diagnostic | The academic benchmark measures correctness, structural erosion, and verbosity as agents repeatedly extend their own code. It publishes native Claude Code and Codex results. | It resets the conversation at each checkpoint, so it does not test context continuity or self-compaction. Public evaluator material also needs a native-web leakage audit. |

GitHub stars are not a validity criterion. Venue or institutional credibility is
useful, but evaluator boundaries, task construction, disclosed limitations, and
reproducible receipts matter more.

## First comparison

Start with one SWE-Milestone repository, not a full suite. `navidrome` is the
first candidate because its backend, UI, scanner, plugin, and protocol work can
support independent value streams without beginning with the largest task.

Use these arms:

- bare native Claude Code;
- bare native Codex;
- one Claude worker and one Codex worker under Gangline, with a thin, mostly
  idle lead.

Pin the benchmark, harnesses, models, effort, starting state, milestone
information, quarantine, and elapsed-time opportunity. Report accepted progress
and regressions together with wall time, aggregate active agent time, tokens,
compactions, resumes, worker changes, duplicated work, and accepted contribution
from each harness. The extra worker is not free speedup. The target is useful
value over the controls, not a perfect score.

Escalate to a larger SWE-Milestone repository only after the first comparison
is valid and informative. Adapt one unsaturated MirrorCode task only after the
native control boundary is proven. RE-Bench and SlopCodeBench are later checks,
not prerequisites for publishing an initial result.

## Validity gates

Before spending model time, verify that:

- no agent can read grader code, official solutions, hidden tests, verifier
  output, or prior trajectories;
- isolation covers server-side web and browser tools as well as shell network
  access;
- an isolation failure aborts the run instead of degrading silently;
- credentials are mounted read-only and never placed in process arguments;
- controls and treatment receive the same benchmark-visible information;
- native sessions resume after interruption;
- optional context lights activate only near, but before, the observed native
  automatic-compaction point; and
- the pinned configuration, arm definitions, limits, and acceptance criteria
  are committed before launch.

A fixture smoke test can prove an adapter starts and grades, but it is not
benchmark evidence. Model runs require explicit operator approval after the
preflight is reviewable.

## Exclusions

- LHTB is retired. The previous campaign was contaminated by grader, solution,
  test, and verifier-output access; its results are not evidence.
- [SWE-bench Verified](https://openai.com/index/why-we-no-longer-evaluate-swe-bench-verified/)
  is saturated and contaminated at the frontier.
- SWE-bench Pro is excluded because [OpenAI's audit](https://openai.com/index/separating-signal-from-noise-coding-evaluations/)
  found widespread broken tasks and retracted its recommendation.
- [SWE-Marathon](https://www.swe-marathon.org/) and
  [FrontierSWE](https://www.frontierswe.com/) contain attractive long tasks but
  are costly and lack the external validation needed to displace MirrorCode.
- [DeepSWE](https://github.com/datacurve-ai/deep-swe) uses strong original-task
  construction but resets between tasks, making it a coding-capability control
  rather than a Gangline showcase.
- [PaperBench](https://openai.com/index/paperbench/) is reputable, but GPU cost
  and rubric-judge dependence make RE-Bench the cleaner first cross-domain
  check.
