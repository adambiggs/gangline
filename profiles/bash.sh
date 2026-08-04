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
  # The box starts at the LAST ROW CARRYING THE PROMPT MARKER — anywhere in the
  # row, not anchored at column 0 — and includes every row after it. -J joins an
  # overflowing row onto the row below it, while -S - keeps the start of a large
  # multiline draft after that prompt has scrolled out of the visible pane.
  # Reading only the marker row loses the rest of that box; reading only the
  # visible pane makes the same live box disappear once it is tall enough.
  #
  # The marker is what starts the box, and the last non-empty row is not: a body
  # pasted into the box occupies rows BELOW the prompt, and the last of those
  # carries no marker at all. Every later row belongs to that rendering until a
  # newer prompt or an indented marker replaces it.
  #
  # Not every marker on the screen is a prompt, and the column-0 anchor this
  # replaces was the only thing saying so. A first-run dialog draws a marker of
  # its own as a menu cursor, INDENTED — and a brief pasted into a security
  # prompt answers it. What separates the two is what precedes the marker: a
  # prompt is drawn at the start of a row and is displaced only by output that
  # ran off the row above, which leaves non-whitespace in front of it. Nothing
  # but whitespace in front of a marker means something drew it there
  # deliberately, and that is not this box. It also CANCELS an older prompt
  # found in history; otherwise widening the capture would resurrect a stale
  # composer underneath the dialog now owning the visible pane.
  #
  # Within the start row, what follows the FIRST marker begins the box, because
  # a body ending in the marker would empty out the suffix of the last one.
  # Every choice fails towards over-reporting content or refusing: empty is the
  # answer that authorises a keystroke, so a read that cannot vouch for itself
  # must not be able to give it.
  local box
  box="$(tmux capture-pane -pJS - -t "$1" |
    awk 'BEGIN { marker = "❯" }
         {
           i = index($0, marker)
           if (i > 0) {
             before = substr($0, 1, i - 1)
             if (i == 1 || before ~ /[^ \t]/) {
               active = 1
               box = substr($0, i + length(marker))
               next
             }
             active = 0
             box = ""
             next
           }
           if (active) box = box ORS $0
         }
         END { if (active) printf "%s", box; else exit 1 }')" || return 1
  printf '%s' "$box" | tr -d '\302\240'
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
