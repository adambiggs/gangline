# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# Pi coding agent. Busy marker observed live against the installed TUI: a running
# turn shows a braille spinner + "Working..."; the spinner char cycles, so match
# the stable token. Compact command from dist/core/slash-commands.js: "compact".
GANG_LAUNCH="pi"
# From `pi --help`: --model, spelled provider/id with an optional :<thinking>
# suffix ("openai-codex/gpt-5.6-sol:high").
GANG_MODEL_OPT="--model"
GANG_BUSY_REGEX="Working\\.\\.\\."
# The working half is NOT measured here — only on claude-code. It costs nothing
# either way: if this harness writes nothing while it works, the arm never fires
# and churn answers as it always did. It is declared because the rest half, which
# is the one with a dangerous direction, IS measured.
# Measured quiet at rest: 30 samples at 1s with the composer empty and the agent
# finished, #{window_activity} frozen on one value, 0 ticks. That is what this
# declares and the ONLY thing gang needs from it — a harness that repaints
# anything at rest ticks forever and could then never read idle, so this stays
# unset until somebody has watched a finished agent sit still.
GANG_QUIET_AT_REST=1
GANG_COMPACT_CMD="/compact"
# GANG_MIDTURN_INPUT is deliberately unset: whether Pi takes input typed during a
# turn, or hands the keystrokes to whatever that turn is running, has not been
# watched live — and that is the difference between a delivered message and a
# paste landing somewhere unintended. Unset, a mid-turn send is refused. Set it to
# 1 once somebody types into a working Pi and reads the result out of its input
# area.
# GANG_COMPACTING_REGEX is unset for the same reason: what Pi paints while it is
# compacting, and whether it keeps an input box up while it does, has not been
# watched. Unset costs a resume nothing but time — it waits for the pane to go
# quiet instead of queueing behind the compaction. Set it once somebody samples a
# live Pi /compact and finds a marker that is gone the moment compaction ends.
# Modal chrome: Pi draws the selected row of a modal with a "→ " at column zero.
# Watched live, in the state described: /settings ("→ Auto-compact  true", with
# a "Type to search · Enter/Space to change · Esc to cancel" footer) and the
# /model selector ("→ gpt-5.6-sol [openai-codex] ✓"). Both take the input area
# over completely — see profile_input below, which is where the other half of
# this lives.
#
# The autocomplete popup carries the same cursor and is deliberately NOT a gate:
# typing "/" lists commands BELOW the input area while the input area keeps the
# keyboard, so a send lands in the composer and must not be refused. The marker
# alone cannot tell the two apart; position does, and profile_input reads the
# position — a live input area is what stops this match from being called a gate.
#
# Core Pi ships no tool-approval system (verified in the installed dist source),
# so a vanilla Pi paints no permission dialog and none is declared here. Gates
# appear only when a project loads a permission extension, and that dialog is the
# EXTENSION's TUI — it rots on the extension's version, which `pi --version`
# cannot see. A project that wires one should shadow this file via GANG_PROFILES,
# declare the dialog's shape there after watching a full ask→answer→erase cycle
# live, and point GANG_VERSION_CMD at something that includes the extension
# version so gang vet watches the pin that can actually rot.
GANG_GATED_REGEX='^→ '
# Every scraped marker in this file was live-verified against these harness
# versions. New release = re-verify + append (gang vet watches the pin).
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
  #
  # A modal opens INSIDE that rule pair rather than replacing it: /settings and
  # the /model selector both push the closing rule down the screen and fill the
  # space with their own search field and list. The frame alone therefore still
  # "finds an input area" while a picker owns the keyboard — so hitch would paste
  # a brief into the picker's search field, and the gated check, which needs the
  # box to be missing, would never fire. The selection cursor at column zero is
  # what tells them apart, and it is the same marker GANG_GATED_REGEX declares
  # above. A literal multibyte string compared by bytes: in cron's C locale
  # substr counts bytes and "→ " is four of them, which is exactly what is wanted.
  tmux capture-pane -pJ -t "$1" | awk '
    /^──────────/ { r1 = r2; r2 = NR }
    { line[NR] = $0 }
    END {
      if (!r1) exit 1
      for (i = r1 + 1; i < r2; i++)
        if (substr(line[i], 1, 4) == "→ ") exit 1
      for (i = r1 + 1; i < r2; i++) print line[i]
    }'
}
