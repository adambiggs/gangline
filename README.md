# Gangline

A harness harness. Gangline unifies any CLI coding agent it can get its hands on —
Claude Code, Pi, whatever ships next — into a tmux-powered team, using each for its
strengths, with minimal harness-specific integration points.

The name: a gangline is the single rope that hitches many dogs, each in its own
harness, to one sled and one musher. The integration point is the attachment clip —
never surgery inside the dog.

Read `CONSTITUTION.md` before touching anything. Law 1: tmux is not a hack, it IS
the substrate.

## Install

```sh
ln -sf "$(pwd)/bin/gang" ~/.local/bin/gang
```

Requires tmux ≥ 3.2 (bracketed paste via `paste-buffer -p`).

## Quickstart

```sh
gang up -p claude-code -d ~/my/repo          # fresh start: session + your own window
                                             # ("manager", attached) — run from outside tmux
gang spawn ada -p claude-code -d ~/my/repo   # ada is now a tmux window running claude
gang send ada --from manager "read the failing test in ci and fix it"
gang status ada                              # busy | idle
gang capture ada                             # what's on ada's screen
gang roster                                  # everyone, with state
gang attach                                  # watch the whole team live
gang kill ada
```

Every message an agent receives is prefixed `[gang:<sender>]` — agents always know
who is speaking. Sends into a busy agent are refused (or `--wait`). Sends are
verified in the pane before submit; an unverified send is a loud failure.

## Layout

```
bin/gang      the whole tool
profiles/     one small file per harness: launch cmd, busy marker
docs/adr/     decisions
```

## Components

1. **Communication substrate** — `bin/gang` (this is it, in full). Proven live:
   Claude Code and Pi agents in one team, manager→worker tasking on both, and a
   cross-harness worker→worker relay (`ada` → `gang send` → `sol`), every hop
   identity-prefixed and pane-verified.
2. **Self-compaction** — models cannot feel their own token count, so the
   substrate measures it and each harness compacts itself. Measurement:
   `gang context <name>` (and a roster column) reads context-window usage
   through the profile's own introspection — Pi's status bar renders usage
   natively; Claude Code gets the shipped statusline beacon
   (`statusline/claude-code-context.sh`, wired via `settings.json`
   `statusLine`; the payload carries `context_window` figures and the beacon
   paints them into the pane where gang can read them). Warning, two legs:
   `gang context-hook` is a Claude Code hook (UserPromptSubmit + PostToolUse)
   injecting one in-context note per crossed `GANG_CONTEXT_BANDS` threshold
   (bare numbers = tokens, `%` suffix = percent of window; default
   `120000,180000,250000,90%`), re-armed when compaction drops usage; and
   `gang patrol` is the harness-agnostic leg — a one-shot roster sweep (run
   by hand or from cron) that injects the same band note as `[gang:patrol]`
   into any agent that crossed a threshold since the last sweep, for
   harnesses with no hook system (Pi's model never sees its own status bar).
   Patrol skips busy agents without burning state (next sweep retries) and
   re-arms on a usage drop. Action:
   `gang compact <name> [--resume <msg>]` triggers the harness's own compaction
   command through the verified-injection path; the resume message is injected
   while compaction runs, so the harness's own input queue holds it and fires
   it when the compact turn ends — the agent resumes work with no idle gap
   and no process babysitting the wait (proven live on Claude Code and Pi;
   a failed compact still releases the queue, so the agent is never
   stranded). An agent past a band self-queues
   its own compact (plus a resume directive) at the next arc seam — no
   permission ask. Durable-state and handoff conventions ride in agent
   prompts, not code.
