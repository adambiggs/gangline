# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# Pi coding agent. Busy marker observed live against the installed TUI: a running
# turn shows a braille spinner + "Working..."; the spinner char cycles, so match
# the stable token. Compact command from dist/core/slash-commands.js: "compact".
GANG_LAUNCH="pi"
GANG_BUSY_REGEX="Working\\.\\.\\."
GANG_COMPACT_CMD="/compact"
# GANG_MIDTURN_INPUT is deliberately unset: whether Pi takes input typed during a
# turn, or hands the keystrokes to whatever that turn is running, has not been
# watched live — and that is the difference between a delivered message and a
# paste landing somewhere unintended. Unset, a mid-turn send is refused. Set it to
# 1 once somebody types into a working Pi and reads the result out of its input
# area.
# Every scraped marker in this file was live-verified against these harness
# versions. New release = re-verify + append (gang doctor watches the pin).
GANG_VERSION_CMD="pi --version"
GANG_VERIFIED_VERSIONS="0.82.0"

profile_context() { # $1 = tmux target; Pi's status bar renders "30.7%/272k" natively
  local m pct win
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo '[0-9]+(\.[0-9]+)?%/[0-9]+k' | tail -1)" \
    || die "no context readout visible in Pi's status bar"
  pct="${m%%\%*}"; win="${m##*/}"; win="${win%k}"
  awk -v p="$pct" -v w="$win" 'BEGIN{printf "%.0fk/%sk (%s%%)\n", p*w/100, w, p}'
}

profile_input() { # $1 = tmux target; prints Pi's input area, fails if it has none
  # The input area sits between the last two full-width horizontal rules;
  # anything non-blank there is a draft an injection would interleave with.
  # No rule pair found means Pi has not drawn its input area yet (still booting)
  # or something else owns the screen — either way, not ready and not clear.
  tmux capture-pane -pJ -t "$1" | awk '
    /^──────────/ { r1 = r2; r2 = NR }
    { line[NR] = $0 }
    END {
      if (!r1) exit 1
      for (i = r1 + 1; i < r2; i++) print line[i]
    }'
}
