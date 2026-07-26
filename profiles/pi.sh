# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# Pi coding agent. Busy marker observed live against the installed TUI: a running
# turn shows a braille spinner + "Working..."; the spinner char cycles, so match
# the stable token. Compact command from dist/core/slash-commands.js: "compact".
GANG_LAUNCH="pi"
GANG_BUSY_REGEX="Working\\.\\.\\."
GANG_COMPACT_CMD="/compact"

profile_context() { # $1 = tmux target; Pi's status bar renders "30.7%/272k" natively
  local m pct win
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo '[0-9]+(\.[0-9]+)?%/[0-9]+k' | tail -1)" \
    || die "no context readout visible in Pi's status bar"
  pct="${m%%\%*}"; win="${m##*/}"; win="${win%k}"
  awk -v p="$pct" -v w="$win" 'BEGIN{printf "%.0fk/%sk (%s%%)\n", p*w/100, w, p}'
}
