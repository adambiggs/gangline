# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# Claude Code CLI. Busy markers verified live against the installed TUI — two
# forms: some phases show "esc to interrupt", but tool execution and long turns
# render only a spinner status line (glyph + gerund + ellipsis), e.g.
# "✽ Generating…" or "* Hashing… (3m 56s · ↓ 12.1k tokens)". The completed form
# ("✻ Baked for 1s") persists in scrollback and must NOT match — it reads
# " for Ns", never an ellipsis. Observed false-negative: patrol nudged a
# mid-turn manager because only "esc to interrupt" was matched.
GANG_LAUNCH="claude"
# Third busy form observed live: API-retry backoff paints
# "✻ 529 Overloaded · Retrying in 2s · attempt 4/10" — a turn is in flight
# but the line starts with a digit, so the gerund pattern misses it.
# Fourth form, compaction: /compact paints a progress bar ("▰▰▰▱▱… 42%") with
# NO gerund spinner and no "esc to interrupt" — none of the other forms match
# (lived it: patrol read a compacting pane as idle and its nudge jumped the
# input queue). The bar glyphs are erased on completion, never in scrollback.
# Literal alternation, not [▰▱]: in cron's C locale a multibyte bracket class
# matches single BYTES, and the welcome-logo glyphs share the UTF-8 prefix.
GANG_BUSY_REGEX='esc to interrupt|^[^ ] [A-Z][a-zé]+(…|\.\.\.) *(\(|$)|Retrying in [0-9]+s|▰|▱'
# Every scraped marker in this file (busy regex, ctx beacon shape, input-box
# shape) was live-verified against these harness versions. New release =
# re-verify + append (gang doctor watches the pin).
GANG_VERSION_CMD="claude --version"
GANG_VERIFIED_VERSIONS="2.1.220"
# Compact command verified live: /compact is the built-in context compaction
# slash command in the installed Claude Code TUI.
GANG_COMPACT_CMD="/compact"
# The progress bar again, on its own, because it answers a second question: not
# "is a turn in flight" but "is THIS turn the compaction" — the one a resume can
# be handed to early. Same glyphs and the same literal-alternation reason as the
# busy regex above; a compacting pane is necessarily busy, so this stays a subset
# of it. Watched frame by frame through a live /compact: the bar paints ~1s after
# submission and holds unbroken to the end (385 consecutive captures, no gap), and
# the input box is drawn and EMPTY in every one of them — so a resume delivered
# mid-compaction has somewhere to land and nothing to interleave with.
GANG_COMPACTING_REGEX='▰|▱'
# Input pasted during a turn is taken, not dropped and not fed to whatever the
# turn is running: the box accepts the paste and Enter replaces it with "Press up
# to edit queued messages". Verified live, which is what lets a send reach a
# working agent instead of bouncing off it. What the flag does NOT promise is
# when: text is often handed straight to the turn already running, while a queued
# slash command waits for that turn to end — so text submitted after a command
# can still arrive before it (see resume_after_compaction).
GANG_MIDTURN_INPUT=1
# The permission dialog, watched live through a full ask→deny→erase cycle in a
# gating permission mode: the question line holds still while the command, the
# menu, and the footer vary around it. While the dialog is up no busy form
# matches and the "❯" input box is gone — so before this marker a gated pane
# read IDLE and a send died "not taking input" with nothing saying why. The
# frame is erased the moment the prompt is answered, never left in scrollback,
# so a match means the dialog owns the screen right now. Only the shell-command
# dialog has been watched; a variant worded differently falls back to that old
# behavior. The first-run trust modal words its question differently and stays
# the spawn-time exit-2 refusal, deliberately.
GANG_GATED_REGEX='Do you want to proceed\?'

profile_context() { # $1 = tmux target; reads the gangline statusline beacon
  # Claude Code shows no context numbers natively — the shipped statusline
  # (statusline/claude-code-context.sh) paints "ctx <used>k/<win>k <pct>%"
  # into the pane from the statusline payload's own context_window figures.
  local m
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || die "no ctx beacon in pane — wire statusline/claude-code-context.sh into settings.json statusLine (see README)"
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}

profile_input() { # $1 = tmux target; prints the input box's contents, fails if no box
  # The input box is the last "❯"-prefixed line in the pane. Failing when no
  # such line exists carries real information both ways: before the TUI has
  # painted it the harness is not yet taking input, and once it is gone the
  # screen belongs to something else (a dialog, a pager) — either way, hold.
  # A human draft, a dimmed ghost-text suggestion, and the "Press up to edit
  # queued messages" hint all render as text after the prompt char.
  # The idle cursor cell captures as U+00A0 (no-break space), whose [:space:]
  # membership is locale-dependent — strip its bytes so a cron locale cannot
  # read every empty box as occupied.
  local line
  line="$(tmux capture-pane -pJ -t "$1" | grep '^❯' | tail -1)" || return 1
  printf '%s' "${line#❯}" | tr -d '\302\240'
}
