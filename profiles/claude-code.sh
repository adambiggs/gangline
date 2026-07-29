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
# Launched as the operator has it configured — no permission flag here, on
# purpose. A gangline agent is unattended by construction (nobody is watching
# its pane, a dialog stops the whole team, and gang will not answer one), so a
# harness on its interactive defaults stalls a team at the first prompt. The
# setting that fixes it belongs in the operator's own settings.json, where they
# can see it and change it back: "permissions": { "defaultMode": "auto" }.
# Choosing it here would put gang's thumb on a security decision that is not
# gang's to make. Want it in the launch line anyway? Shadow this profile via
# GANG_PROFILES and patch this one line.
GANG_LAUNCH="claude"
# From `claude --help`: --model takes an alias ("sonnet", "opus") or a full
# model id ("claude-fable-5").
GANG_MODEL_OPT="--model"
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
# re-verify + append (gang vet watches the pin).
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
# Modal chrome, not dialog sentences. Claude Code frames every modal the same
# two ways, and both were watched live: a selection cursor "❯" that is INDENTED,
# and an "Esc to <verb>" footer. Indentation is the whole tell — the composer's
# own "❯" sits at column zero (profile_input below keys on exactly that), so a
# "❯" with space in front of it is a menu row, which means a menu owns the
# screen. The footer covers the modals that carry no menu at all.
#
# Watched live, in the state described, with the composer gone in every one:
# the shell approval (" ❯ 1. Yes" · "Esc to cancel · Tab to amend"), the plan
# approval ("   ❯ 1. Yes, and use auto mode", NO Esc footer), the first-run
# trust modal (" ❯ 1. Yes, I trust this folder" · "Esc to cancel"), /model,
# /theme and /config (indented "❯" plus a footer), and /status (footer only —
# an info screen with no menu). Neither alternate covers all seven; the union
# does, which is why both are here.
#
# Enumerating the sentences is what this replaces, and the plan approval is the
# receipt: it asks "Would you like to proceed?", so the old
# "Do you want to proceed\?" never matched it and a pane stopped dead on a
# permission dialog reported idle.
GANG_GATED_REGEX='^ +❯|Esc to'

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
  # The input box is the "❯" line inside the composer's own frame: two rules of
  # "─" drawn at column zero, one directly above the box and one directly below
  # it. The frame is what identifies the box, because the prompt character alone
  # does not — Claude Code echoes every SUBMITTED message into the transcript
  # behind the same column-zero "❯". Taking the last "❯" line therefore found a
  # message sent minutes ago and reported a box that was no longer on screen.
  # Lived it: a pane stopped at a Bash approval dialog read idle, because the
  # user message above the dialog still matched — so the gate never fired, and a
  # send would have landed in the dialog and answered it with its own Enter.
  #
  # A modal takes the frame with it. The approval dialogs draw a single rule as
  # their top edge and none below; the plan approval and /theme indent theirs and
  # draw them narrower; /model, /config and /status use a different glyph
  # entirely. None of them leaves a pair, so none of them is mistaken for a box —
  # which is what lets hitch's no-input-box refusal catch a dialog before a brief
  # can answer it.
  #
  # Failing when there is no frame carries real information both ways: before the
  # TUI has painted it the harness is not yet taking input, and once it is gone
  # the screen belongs to something else — either way, hold. A human draft, a
  # dimmed ghost-text suggestion, and the "Press up to edit queued messages" hint
  # all render as text after the prompt char, and a draft can run to several rows
  # inside the frame, so every row of it is printed.
  #
  # Multibyte care, twice over. The rule test is a gsub of a literal "─" with the
  # remainder checked for emptiness — a literal matches as a byte sequence in
  # cron's C locale, where "─+" would quantify only the last byte and a bracket
  # class would match single bytes. And the idle cursor cell captures as U+00A0
  # (no-break space), whose [:space:] membership is locale-dependent, so its
  # bytes are stripped rather than trusted to a class.
  local box
  box="$(tmux capture-pane -pJ -t "$1" | awk '
    { line[NR] = $0; if (NF) last = NR
      t = $0; n = gsub(/─/, "", t)
      # All rule glyphs and nothing else. Leading spaces survive the gsub, so
      # this pins the rule to column zero too, and an indented dialog edge is
      # rejected by the same test. Width is carried along: the two rules of one
      # frame always match each other.
      if (n && t == "") { prev = rule; prevw = rulew; rule = NR; rulew = n }
    }
    END {
      if (!prev || !rule || rulew != prevw) exit 1
      # A stale rule higher up the transcript must not pair with the top edge of
      # a dialog and bracket the transcript between the two. The real frame
      # closes at the bottom of the screen, with only the ctx beacon and the
      # permission-mode hint under it.
      if (last - rule > 5) exit 1
      seen = 0
      for (i = prev + 1; i < rule; i++) {
        s = line[i]
        if (!seen) {
          if (s ~ /^[[:space:]]*$/) continue
          if (substr(s, 1, 3) != "❯") exit 1   # framed, but not the composer
          sub(/^❯/, "", s); seen = 1
        }
        print s
      }
      if (!seen) print ""
    }')" || return 1
  printf '%s' "$box" | tr -d '\302\240'
}
