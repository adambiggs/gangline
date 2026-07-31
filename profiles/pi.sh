# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# Pi coding agent. Busy marker observed live against the installed TUI: a running
# turn shows a braille spinner + "Working..."; the spinner char cycles, so match
# the stable token. Compact command from dist/core/slash-commands.js: "compact".
GANG_LAUNCH="pi"
# From `pi --help`: --model, spelled provider/id with an optional :<thinking>
# suffix ("openai-codex/gpt-5.6-sol:high").
# NO GANG_RESUME_LAUNCH, deliberately (ADR-0007). pi has --continue, documented
# only as "Continue previous session", with no statement that the selection is
# scoped to a working directory and no such scoping established here. Every other
# marker in this file was watched live before it was written down, and a resume
# declaration is held to the same bar: an unscoped one would hand a rebuilt agent
# somebody else's conversation. `gang hitch --resume` refuses this profile until
# somebody measures it.
GANG_MODEL_OPT="--model"
GANG_BUSY_REGEX="Working\\.\\.\\."
# GANG_QUIET_AT_REST is deliberately unset. Two controlled rests really were
# quiet: the original finished-turn probe, and a fresh probe immediately after a
# successful manual compaction, each sampled 30 times at 1s with an empty
# composer. Both held #{window_activity} and the capture hash constant; the
# post-compaction probe captured zero raw pty bytes too. But a live 0.82.0 Pi
# falsified the general claim: with no Working marker, an empty composer, and a
# byte-identical capture over 8s, its activity timestamp kept advancing for about
# forty minutes and gang kept the finished agent busy. The pane resumed work
# before the repeated idle writes could be captured, so their exact source and
# the state that starts them remain unknown; compaction alone is ruled out.
#
# Removing the pty-activity arm does not leave one sampled frame standing alone.
# Three complete tool turns were read every ~0.17s through clear/redraw output and
# timed sleeps: Working was present in 153/155 pre-completion frames, absent only
# in the first frame of two turns before the marker first painted, never absent
# again after that, and present in all 106 samples where the sleep process proved
# the tool was live. Pane churn covers those initial paint transitions. A stable,
# markerless live state outside those turns is still unknown, so this stays unset
# rather than turning a disproved rest assertion into fabricated busy status.
GANG_COMPACT_CMD="/compact"
# Watched end to end on 0.82.0 while Pi's bash tool was running `sleep 15` and
# the pane painted Working: typed text appeared inside the two-rule composer,
# never in the tool. Enter cleared the composer and painted `Steering: <text>`;
# once the running turn ended, the text became the next user message and Pi
# answered it. The actual gang path was exercised too: send verified the paste
# and the post-Enter transition, reported it accepted mid-turn, and the complete
# gang envelope became the next user message rather than input to the tool.
GANG_MIDTURN_INPUT=1
# Watched on 0.82.0 across a complete manual /compact. Within 135ms the status
# row painted a cycling spinner plus "Compacting context... (escape to cancel)";
# it stayed present through the 4.1s summarization call and disappeared when the
# input box returned. The transcript then retained "[compaction]", which is why
# the regex names only the live status text. The composer remained drawn and
# empty throughout, so compacting()'s input_clear check accepts the same frames.
# The actual gang path then saw this marker in 113/123 samples, injected its
# resume as Steering while compaction was live, and Pi accepted that envelope as
# the next user message; @gl_resume_failed stayed empty.
GANG_COMPACTING_REGEX='Compacting context\.\.\.'
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
GANG_OCCUPIED_REGEX='^→ '
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
  # what tells them apart, and it is the same marker GANG_OCCUPIED_REGEX declares
  # above. Matched as an anchored regex rather than substr(line, 1, 4): counting
  # to 4 assumes "→ " is four BYTES, which is true only where awk counts bytes.
  # A character-oriented awk takes four CHARACTERS, so the test never matches,
  # the exit below never fires, and the picker's search field is handed back as
  # an input box — the exact paste-into-the-picker this check exists to stop,
  # turned back on by nothing but which awk is installed. A literal in a regex
  # matches the same way under both.
  tmux capture-pane -pJ -t "$1" | awk '
    /^──────────/ { r1 = r2; r2 = NR }
    { line[NR] = $0 }
    END {
      if (!r1) exit 1
      for (i = r1 + 1; i < r2; i++)
        if (line[i] ~ /^→ /) exit 1
      for (i = r1 + 1; i < r2; i++) print line[i]
    }'
}
