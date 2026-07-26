# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# Claude Code CLI. Busy markers verified live against the installed TUI — two
# forms: some phases show "esc to interrupt", but tool execution and long turns
# render only a spinner status line (glyph + gerund + ellipsis), e.g.
# "✽ Generating…" or "* Hashing… (3m 56s · ↓ 12.1k tokens)". The completed form
# ("✻ Baked for 1s") persists in scrollback and must NOT match — it reads
# " for Ns", never an ellipsis. Observed false-negative: patrol nudged a
# mid-turn manager because only "esc to interrupt" was matched.
GANG_LAUNCH="claude"
GANG_BUSY_REGEX='esc to interrupt|^[^ ] [A-Z][a-zé]+(…|\.\.\.) *(\(|$)'
# Every scraped marker in this file (busy regex, ctx beacon shape, input-box
# shape) was live-verified against these harness versions. New release =
# re-verify + append (gang doctor watches the pin).
GANG_VERSION_CMD="claude --version"
GANG_VERIFIED_VERSIONS="2.1.220"
# Compact command verified live: /compact is the built-in context compaction
# slash command in the installed Claude Code TUI.
GANG_COMPACT_CMD="/compact"

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

profile_input_clear() { # $1 = tmux target; input box holds no draft/ghost/queued text
  # The input box is the last "❯"-prefixed line in the pane. A human draft,
  # a dimmed ghost-text suggestion, and the "Press up to edit queued messages"
  # hint all render as text after the prompt char — any of them means an
  # injection would interleave. No "❯" visible at all = hold, conservatively.
  # The idle cursor cell captures as U+00A0 (no-break space), whose [:space:]
  # membership is locale-dependent — strip its bytes so a cron locale can't
  # turn every empty box into a false hold.
  local line
  line="$(tmux capture-pane -pJ -t "$1" | grep '^❯' | tail -1)" || return 1
  ! printf '%s' "${line#❯}" | tr -d '\302\240' | grep -q '[^[:space:]]'
}
