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
  #
  # The box is the LAST NON-EMPTY ROW of the pane, not the last row that begins
  # with the prompt: -J joins an overflowing row onto the row below it, so
  # output that wrapped can carry the composer's prompt off column 0, and an
  # anchored match then lands on a stale empty prompt row further up — saying
  # the box is empty while this one holds text. Within that row, what follows
  # the FIRST marker is the box, because a body ending in the marker would
  # empty out the suffix of the last one. Both choices fail towards
  # over-reporting content: empty is the answer that authorises a keystroke, so
  # a read that cannot vouch for itself must not be able to give it. A last row
  # carrying no marker at all is not this box, and rc 1 says so rather than
  # handing back whatever was sitting there.
  local line
  line="$(tmux capture-pane -pJ -t "$1" | awk 'NF { line = $0 } END { print line }')" || return 1
  case "$line" in *❯*) ;; *) return 1 ;; esac
  printf '%s' "${line#*❯}" | tr -d '\302\240'
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
