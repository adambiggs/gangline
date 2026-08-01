# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# Plain bash — for testing gangline's own mechanics. Never busy. No scraped
# TUI markers, so no version pin ("any" = version-independent to gang vet).
# Prompted with "❯ " so this stand-in has a real input box, painted when the
# shell is actually ready for input — the same signal a TUI gives, from a
# process gang has no special knowledge of.
GANG_LAUNCH="PS1='❯ ' bash --norc"
GANG_BUSY_REGEX=""
GANG_VERIFIED_VERSIONS="any"
# No GANG_MODEL_OPT: a shell has no model, so `hitch -m` on this profile refuses.

profile_input() { # $1 = tmux target; same shape as a real TUI's input box
  # The simplest thing that answers the question, so gang's input-box handling
  # is exercised without a harness installed. A prompt string is all a shell
  # paints — there is no transcript echoing the prompt character back and no
  # modal to hide it, so the frame-finding a real TUI needs would have nothing
  # here to find.
  local line
  line="$(tmux capture-pane -pJ -t "$1" | grep '^❯' | tail -1)" || return 1
  printf '%s' "${line#❯}" | tr -d '\302\240'
}

profile_context() { # $1 = tmux target; reads a beacon the pane was told to print
  # Same "ctx <used>k/<win>k <pct>%" shape the claude-code statusline paints, so
  # echoing one line into a bash pane exercises the whole band path — ladder,
  # patrol guards, `gang hook`'s band note — without a harness installed. Not a scraped
  # marker: nothing but the test writes it, so there is no TUI here to rot.
  local m
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || die "no ctx beacon in pane"
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}
