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
