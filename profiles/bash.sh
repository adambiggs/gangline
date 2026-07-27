# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# Plain bash — for testing gangline's own mechanics. Never busy. No scraped
# TUI markers, so no version pin ("any" = version-independent to gang doctor).
GANG_LAUNCH="bash"
GANG_BUSY_REGEX=""
GANG_VERIFIED_VERSIONS="any"

profile_input_clear() { # $1 = tmux target; same shape as a real TUI's input box
  # Mirrors the claude-code guard so gang's own input-box handling is exercised
  # without a harness installed: the box is the last "❯"-prefixed line, and no
  # such line at all means hold, conservatively. A bash pane has none until the
  # test prints one, which is exactly the state a booting TUI is in.
  local line
  line="$(tmux capture-pane -pJ -t "$1" | grep '^❯' | tail -1)" || return 1
  ! printf '%s' "${line#❯}" | tr -d '\302\240' | grep -q '[^[:space:]]'
}

profile_context() { # $1 = tmux target; reads a beacon the pane was told to print
  # Same "ctx <used>k/<win>k <pct>%" shape the claude-code statusline paints, so
  # echoing one line into a bash pane exercises the whole band path — ladder,
  # patrol guards, context-hook — without a harness installed. Not a scraped
  # marker: nothing but the test writes it, so there is no TUI here to rot.
  local m
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || die "no ctx beacon in pane"
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}
