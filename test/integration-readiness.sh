# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Readiness: staged records against a fresher box, the occupied and busy classifications, turn brackets, context decay, and native compaction.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# A staged record is state; the box is fresher evidence. Staged input can
# flush outside gang's sight — an operator's Enter, a queue draining at a
# turn boundary — and status/roster must not report a paste the empty box
# proves gone. A box still holding content keeps the record and the report.
tmux set-option -w -t "$(window_id 1)" @gl_staged \
  "'MARK_GONE' is staged unsent in this box"
excludes "an empty box retires a staged record at status time" \
  "$("$GANG" status 1)" "undelivered input"
equal "and the retired record is gone" "" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_staged)"
# The wording of a real no-rendering leg: the box could not be read after the
# paste, so there is nothing to match it against later. Calling what turns up in
# that box a human draft would invent the one fact gang is missing, and would
# invent it against gang's own adjacent record of having pasted there.
tmux set-option -w -t "$(window_id 1)" @gl_staged \
  "'MARK_HELD' was pasted into this box and the box could not be read afterwards — it may be sitting there unsent"
tmux send-keys -l -t "$(window_id 1)" 'MARK_HELD draft'
contains "a non-empty box keeps the record and the report" \
  "$("$GANG" status 1)" "undelivered input"
contains "a staged record gang cannot match to the box claims no author" \
  "$("$GANG" status 1)" "box: unattributed:"
# A rendering that exists and does not match is the same missing fact: the box
# has moved since gang recorded it, and who moved it is exactly what is unknown.
tmux set-option -w -t "$(window_id 1)" @gl_staged_box "MARK_HELD something else"
contains "and a rendering that does not match settles nothing either" \
  "$("$GANG" status 1)" "box: unattributed:"
# Same box, now byte-identical to the rendering gang recorded when it staged
# its OWN body — the one comparison that separates gang's text from a person's,
# and the same equality stage_clear retires a record on.
tmux set-option -w -t "$(window_id 1)" @gl_staged_box "$("$GANG" composer 1)"
contains "a box matching gang's recorded rendering is classified as its own" \
  "$("$GANG" status 1)" "box: staged:"
tmux set-option -uw -t "$(window_id 1)" @gl_staged_box
contains "roster carries the same verdict" "$("$GANG" roster)" "undelivered-input"
tmux send-keys -t "$(window_id 1)" C-u
tmux set-option -uw -t "$(window_id 1)" @gl_staged

# Clearing a record is evidence the OBSTRUCTION is gone, never retroactive
# proof the recorded body was delivered. A refused delivery changes nothing;
# the next VERIFIED delivery to the same window retires the record, because
# verified success is only reachable through the provably clear box that is
# itself the gone-obstruction evidence.
tmux set-option -w -t "$(window_id 1)" @gl_staged \
  "'MARK_OLD' is staged unsent in this box"
tmux send-keys -l -t "$(window_id 1)" 'blocking draft'
if printf 'MARK_RETAIN' | "$GANG" send --to 1 --from tester --stdin >/dev/null 2>&1; then
  fail "a refused delivery does not clear another record" "send succeeded over a draft"
else
  pass "a refused delivery does not clear another record"
fi
contains "the record survives the refusal" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_staged)" "MARK_OLD"
tmux send-keys -t "$(window_id 1)" C-u
printf 'MARK_CLEARS' | "$GANG" send --to 1 --from tester --stdin >/dev/null
equal "a verified delivery to the same window retires the stale record" "" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_staged)"

# The drain-between-reads race: a prior obstruction can vanish AFTER
# stage_clear reads it (record retained on nonempty evidence) but BEFORE
# input_clear reads again — a queue draining at a turn boundary mid-send. The
# delivery then verifies cleanly and the stale record must not survive it.
# The fixture replays the race deterministically: eight tickets return a
# parked reading for exactly the box reads through stage_clear's landing
# zone — the occupied probe at cmd_send, inject, and stage_clear reads the
# box once each via input_painted, the settled pairs at inject and
# stage_clear read it twice each, and stage_clear's landing zone is the
# eighth — and every later read sees the real, drained composer. The ticket
# count is coupled to that read order; changing inject's guard sequence must
# update it. Leftover tickets fail loudly as a refused send, but EXHAUSTING
# them early would let stage_clear see the drained box and clear the record
# itself — both cores pass and the world silently stops exercising the race —
# so the fixture logs the caller of every parked reading and the world
# asserts the final ticket was consumed by stage_clear's landing-zone read,
# the coupling's one load-bearing position.
cat > "$RUN_ROOT/collars/drain.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_drain_real="\$(declare -f collar_input)"
eval "drain_real_input \${_gl_drain_real#collar_input}"
collar_input() { # a parked reading per ticket, then the real drained box
  local n i=1
  n="\$(cat "$RUN_ROOT/drain-tickets" 2>/dev/null)"
  if [ "\${n:-0}" -gt 0 ]; then
    printf '%s' "\$((n - 1))" > "$RUN_ROOT/drain-tickets"
    # THE PREDICATE THAT ASKED, not the classifier it asked through. Every
    # composer reading in gang goes through one place that tells a box from an
    # absent one from a pane it could not read, and that funnel is a frame in
    # this stack rather than a reader. The claim below is about which guard
    # took the eighth parked reading, so the funnel is stepped over.
    while [ "\${FUNCNAME[i]}" = input_read ]; do i=\$((i + 1)); done
    printf '%s<%s\n' "\${FUNCNAME[i]}" "\${FUNCNAME[i+1]}" >> "$RUN_ROOT/drain-reads.log"
    printf 'PARKED_OBSTRUCTION'
    return 0
  fi
  drain_real_input "\$1"
}
SH
printf '0' > "$RUN_ROOT/drain-tickets"
"$HITCH" drain -c drain -d /tmp >/dev/null
tmux set-option -w -t "$(window_id drain)" @gl_staged \
  "'MARK_OLD' is staged unsent in this box"
tmux set-option -w -t "$(window_id drain)" @gl_staged_box "BOXMEMO_NOT_MATCHING"
printf '8' > "$RUN_ROOT/drain-tickets"
if drain_out="$(printf 'MARK_DRAIN' | "$GANG" send --to drain --from tester --stdin 2>&1)"; then
  pass "a delivery whose prior obstruction drained mid-send still verifies"
else
  fail "a delivery whose prior obstruction drained mid-send still verifies" \
    "$drain_out"
fi
equal "and the verified delivery retires the drained record" "" \
  "$(tmux show-options -wqv -t "$(window_id drain)" @gl_staged)"
equal "the final parked reading was stage_clear's own landing-zone read" \
  "landing_zone<stage_clear" "$(tail -1 "$RUN_ROOT/drain-reads.log")"
"$GANG" drop drain >/dev/null

# Once queue evidence is declared, an UNREADABLE verification reread is
# ambiguity, not proof of submission. The fixture's composer flips to a
# sentinel after Enter and its collar_input grants exactly one readable look
# at that sentinel: the first reading breaks the change loop as a normal
# non-queue submission would, and the late queue-evidence reread finds the
# box unreadable. Falling through to success here is the hole; the send must
# die naming the uncertainty and record the body as unknown.
cat > "$RUN_ROOT/flicker-rc" <<'RC'
PS1='❯ '
PROMPT_COMMAND='[ -f "$FLICKER_FLAG" ] && PS1="❯ POST_SENTINEL"'
RC
cat > "$RUN_ROOT/collars/flicker.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'FLICKER_FLAG=$RUN_ROOT/flicker-flag ENV=$RUN_ROOT/flicker-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*QUEUE_HINT_NEVER_SHOWN\$'
collar_input() { # one readable look at the post-Enter sentinel, then nothing
  local line
  line="\$(tmux capture-pane -pJ -t "\$1" | grep '❯' | tail -1)" || return 1
  line="\${line#*❯}"
  case "\$line" in
    *POST_SENTINEL*)
      [ ! -e "$RUN_ROOT/flicker-seen" ] || return 1
      : > "$RUN_ROOT/flicker-seen" ;;
  esac
  printf '%s' "\$line" | tr -d '\302\240'
}
SH
"$HITCH" flicker -c flicker -d /tmp >/dev/null
touch "$RUN_ROOT/flicker-flag"
if flicker_out="$(printf 'MARK_FLICKER' | "$GANG" send --to flicker --from tester --stdin 2>&1)"; then
  fail "an unreadable verification reread is not a delivery" "send reported success"
else
  pass "an unreadable verification reread is not a delivery"
fi
contains "the ambiguity is named rather than spent as success" \
  "$flicker_out" "is unknown"
contains "the uncertain body is recorded against the window" \
  "$(tmux show-options -wqv -t "$(window_id flicker)" @gl_staged)" "is unknown"
"$GANG" drop flicker >/dev/null

# An EXPIRED turn bracket is could-not-determine, not busy: stale owned state
# must not veto a delivery that fresh box evidence proves safe. A provably
# empty composer proceeds under the full submission verification; anything
# less refuses naming both the expired witness and the box state. A FRESH
# open bracket keeps refusing mid-turn exactly as before. Delivery leaves
# @gl_turn byte-identical: native hooks write it lock-free, tmux has no
# atomic compare-and-delete, so any reader's unset can erase a fresh hook
# stamp landing between the read and the unset — the invariant pinned here
# and in the malformed world below is that NO reader on delivery's
# transitive path writes the bracket at all.
stale_bracket="open $(( $(date +%s) - 400 ))"
tmux set-option -w -t "$(window_id 1)" @gl_turn "$stale_bracket"
if printf 'MARK_TURNFALL' | "$GANG" send --to 1 --from tester --stdin >/dev/null 2>&1; then
  pass "an expired bracket over a provably empty box does not veto delivery"
else
  pass_rc=$?
  fail "an expired bracket over a provably empty box does not veto delivery" \
    "send refused with rc $pass_rc"
fi
contains "and the delivery actually landed" "$(pane 1)" "MARK_TURNFALL"
equal "delivery leaves the turn bracket to its native owner, byte-identical" \
  "$stale_bracket" "$(tmux show-options -wqv -t "$(window_id 1)" @gl_turn)"
tmux set-option -w -t "$(window_id 1)" @gl_turn "open $(( $(date +%s) - 400 ))"
tmux send-keys -l -t "$(window_id 1)" 'half a draft'
if veto_draft="$(printf 'MARK_NODRAFT' | "$GANG" send --to 1 --from tester --stdin 2>&1)"; then
  fail "an expired bracket over a drafted box still refuses" "send succeeded"
else
  pass "an expired bracket over a drafted box still refuses"
fi
contains "the refusal names the expired witness" \
  "$veto_draft" "no usable busy witness"
contains "with the bracket's own reason" "$veto_draft" "turn-bracket bound reached"
contains "and the box state, not a mid-turn claim" \
  "$veto_draft" "not provably empty"
tmux send-keys -t "$(window_id 1)" C-u
tmux set-option -w -t "$(window_id 1)" @gl_turn "open $(date +%s)"
if fresh_veto="$(printf 'MARK_FRESH' | "$GANG" send --to 1 --from tester --stdin 2>&1)"; then
  fail "a fresh open bracket still refuses mid-turn" "send succeeded"
else
  pass "a fresh open bracket still refuses mid-turn"
fi
contains "with the mid-turn refusal, not the expired one" \
  "$fresh_veto" "not safely reachable mid-turn"
excludes "the refused fresh-bracket body never landed" "$(pane 1)" "MARK_FRESH"
tmux set-option -uw -t "$(window_id 1)" @gl_turn

# The box-vanishes backstop: occupied's read sees a composer, then the box
# disappears before the expired fall-through's own read — one readable look,
# then nothing. The refusal must name the expired witness and the unreadable
# box rather than claim mid-turn work.
cat > "$RUN_ROOT/collars/vanish.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_vanish_real="\$(declare -f collar_input)"
eval "vanish_real_input \${_gl_vanish_real#collar_input}"
collar_input() { # readable per ticket once the vanish flag is set, then not
  local n
  if [ ! -f "$RUN_ROOT/vanish-flag" ]; then vanish_real_input "\$1"; return; fi
  n="\$(cat "$RUN_ROOT/vanish-tickets" 2>/dev/null)"
  if [ "\${n:-0}" -gt 0 ]; then
    printf '%s' "\$((n - 1))" > "$RUN_ROOT/vanish-tickets"
    vanish_real_input "\$1"
    return
  fi
  return 1
}
SH
"$HITCH" vanish -c vanish -d /tmp >/dev/null
tmux set-option -w -t "$(window_id vanish)" @gl_turn "open $(( $(date +%s) - 400 ))"
printf '1' > "$RUN_ROOT/vanish-tickets"
touch "$RUN_ROOT/vanish-flag"
if vanish_out="$(printf 'MARK_VANISH' | "$GANG" send --to vanish --from tester --stdin 2>&1)"; then
  fail "a box that vanishes before the fall-through's read still refuses" \
    "send succeeded"
else
  pass "a box that vanishes before the fall-through's read still refuses"
fi
contains "naming the expired witness" "$vanish_out" "no usable busy witness"
contains "and the unreadable box" "$vanish_out" "cannot be read"
excludes "the refused vanished-box body never landed" \
  "$(pane vanish)" "MARK_VANISH"
"$GANG" drop vanish >/dev/null

# The classification look is taken AFTER the decision it names, so the
# obstruction can leave in the gap between them — an operator's C-u, a queue
# draining at a turn boundary. That look then reads an empty box successfully,
# which is the opposite finding from a box that cannot be read, and only one of
# the two is a harness in trouble. This collar serves the draft for exactly the
# reads a refusal takes to settle and hands the naming look an empty box after
# it; a change in that count fails this check rather than quietly retargeting
# it, because the class named is asserted exactly.
cat > "$RUN_ROOT/collars/emptied.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_emptied_real="\$(declare -f collar_input)"
eval "emptied_real_input \${_gl_emptied_real#collar_input}"
collar_input() { # the draft per ticket once the flag is set, then an empty box
  local n
  if [ ! -f "$RUN_ROOT/emptied-flag" ]; then emptied_real_input "\$1"; return; fi
  n="\$(cat "$RUN_ROOT/emptied-tickets" 2>/dev/null)"
  if [ "\${n:-0}" -gt 0 ]; then
    printf '%s' "\$((n - 1))" > "$RUN_ROOT/emptied-tickets"
    emptied_real_input "\$1"
    return
  fi
  printf ''
}
SH
"$HITCH" emptied -c emptied -d /tmp >/dev/null
tmux send-keys -l -t "$(window_id emptied)" 'MARK_LEAVING'
# Six: the box reads a refused send takes to settle — the busy verdict's
# composer and emptiness pair, the settled-composer pair, the parked-queue
# preflight, and the emptiness read that refuses. The seventh is the naming
# look, and it gets an empty box.
printf '6' > "$RUN_ROOT/emptied-tickets"
touch "$RUN_ROOT/emptied-flag"
if emptied_out="$(printf 'MARK_EMPTIED' |
  "$GANG" send --to emptied --from tester --stdin 2>&1)"; then
  fail "a box emptying under the refusal is still a refusal" "send succeeded"
else
  pass "a box emptying under the refusal is still a refusal"
fi
contains "and the naming look reports the box gone, not a harness that cannot be read" \
  "$emptied_out" "[cleared:"
excludes "the refused body never landed" "$(pane emptied)" "MARK_EMPTIED"
"$GANG" drop emptied >/dev/null

# A malformed bracket is REPORTED, never repaired: the reader-path clear was
# the same erase-fresh-evidence race one call deeper — cmd_send reaches
# turn_witness through busy(), and an unset there can land on top of a fresh
# hook stamp. Status names the unreadable value, delivery falls through on
# box evidence, and the value survives both byte-identical until the hooks
# that own the bracket rewrite it.
tmux set-option -w -t "$(window_id 1)" @gl_turn "gibberish not-a-stamp"
malformed_status="$("$GANG" status 1)"
contains "a malformed bracket reads as unreadable, not busy" \
  "$malformed_status" "turn-bracket value unreadable"
equal "and status leaves the malformed value in place" \
  "gibberish not-a-stamp" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_turn)"
if printf 'MARK_MALFORMED' | "$GANG" send --to 1 --from tester --stdin >/dev/null 2>&1; then
  pass "a malformed bracket over a provably empty box does not veto delivery"
else
  malformed_rc=$?
  fail "a malformed bracket over a provably empty box does not veto delivery" \
    "send refused rc $malformed_rc"
fi
contains "and that delivery landed" "$(pane 1)" "MARK_MALFORMED"
equal "delivery leaves even a malformed bracket untouched, byte-identical" \
  "gibberish not-a-stamp" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_turn)"
tmux set-option -uw -t "$(window_id 1)" @gl_turn

# The issue-#102 shape: an Escape-interrupted turn leaves a fossil busy
# marker in the transcript — "Retrying in Ns" — matching the busy regex
# forever while the process sits idle. Frozen paint witnesses a live turn
# only while something repaints it: over an expired bracket, with no recent
# pty activity and a byte-stable pane, it is could-not-determine. The
# fixture is quiet-at-rest so the activity leg genuinely reads
# #{window_activity} — the deterministic activity inputs are the window
# bounds themselves: an enormous window makes the fresh paint "recent"
# under any load, a zero window makes every stamp old. Roster's immediate
# snapshot keeps the painted verdict by design — it cannot probe stability
# without consuming the churn wait.
cat > "$RUN_ROOT/collars/fossil.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
"$HITCH" fossil -c fossil -d /tmp >/dev/null
tmux send-keys -l -t "$(window_id fossil)" \
  'echo "Retrying in 8s left by an interrupted loop"'
tmux send-keys -t "$(window_id fossil)" Enter
fossil_bracket="open $(( $(date +%s) - 400 ))"
tmux set-option -w -t "$(window_id fossil)" @gl_turn "$fossil_bracket"
# Positive control: recent pty activity preserves painted busy — the
# demotion must not fire while the pty leg credits the fresh paint.
fossil_active="$(GANG_ACTIVITY_WINDOW=100000 "$GANG" status fossil | sed -n '1p')"
equal "recent pty activity keeps the busy verdict itself, not its explanation" \
  "-busy-" "$fossil_active"
if printf 'MARK_ACTIVE' | GANG_ACTIVITY_WINDOW=100000 \
  "$GANG" send --to fossil --from tester --stdin >/dev/null 2>&1; then
  fail "recent pty activity keeps refusing delivery mid-turn" "send succeeded"
else
  pass "recent pty activity keeps refusing delivery mid-turn"
fi
excludes "the refused active-pane body never landed" \
  "$(pane fossil)" "MARK_ACTIVE"
contains "roster's immediate snapshot keeps the painted verdict" \
  "$("$GANG" roster)" "-busy-"
# The fossil verdict: no recent write, byte-stable pane, expired bracket.
fossil_status="$(GANG_ACTIVITY_WINDOW=0 "$GANG" status fossil)"
contains "frozen busy paint over an expired bracket reads expired, not busy" \
  "$fossil_status" "?unknown?"
contains "naming the frozen paint beside the bracket's reason" \
  "$fossil_status" "busy paint frozen"
if printf 'MARK_FOSSIL' | GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to fossil --from tester --stdin >/dev/null 2>&1; then
  pass "a fossil busy marker does not veto delivery to a provably empty box"
else
  fossil_rc=$?
  fail "a fossil busy marker does not veto delivery to a provably empty box" \
    "send refused rc $fossil_rc"
fi
contains "and the delivery landed under the fossil" "$(pane fossil)" "MARK_FOSSIL"
equal "the fossil's bracket is left to its native owner, byte-identical" \
  "$fossil_bracket" \
  "$(tmux show-options -wqv -t "$(window_id fossil)" @gl_turn)"
"$GANG" drop fossil >/dev/null

# The other half of the issue-#102 shape, on the path no harness reports: a turn
# ended by RAW keys in the pane. `gang interrupt` closes the fact it ended, and
# an Escape typed straight into the pane closes nothing — the bracket stays open
# and only ever gets older, so could-not-determine would be that agent's
# permanent verdict while it sits provably ready. The tiers under the expired
# event decide it instead. Time is an input here, never a wait: the bracket's
# age and the activity window are injected.
cat > "$RUN_ROOT/collars/abandoned.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
"$HITCH" abandoned -c abandoned -d /tmp >/dev/null
abandoned_bracket="open $(( $(date +%s) - 400 ))"
tmux set-option -w -t "$(window_id abandoned)" @gl_turn "$abandoned_bracket"
# The guard the decay must not stomp, asserted first: an UNEXPIRED bracket is
# fresh owned state and outranks every quiet tier under it. Nothing about a
# still-bounded turn changes, however ready the pane looks.
equal "an unexpired bracket over the same quiet box is still a live turn" \
  "-busy-" \
  "$(GANG_TURN_LIMIT=100000 GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | sed -n '1p')"
equal "an abandoned turn decays to idle once its bracket expires" \
  "~idle~" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | sed -n '1p')"
contains "roster's snapshot decays with it — reading the box costs no churn wait" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^abandoned ')" "~idle~"
# Not a free pass over anything the box refutes: a draft sitting in the composer
# is the state the busy verdict exists to protect, and it keeps the answer
# could-not-determine on both readings.
tmux send-keys -l -t "$(window_id abandoned)" 'half a thought'
equal "a drafted box refuses the decay" \
  "?unknown? (turn-bracket bound reached)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | sed -n '1p')"
contains "roster's snapshot refuses it on the same evidence" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^abandoned ')" "?unknown?"
tmux send-keys -t "$(window_id abandoned)" C-u
equal "clearing the draft restores the decayed verdict" \
  "~idle~" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | sed -n '1p')"
# Quiet must be MEASURED, never assumed. Hold the activity-only bound open past
# its limit with the pty credited as recent: the activity tier then reports
# could-not-determine, and an unknown tier cannot witness readiness.
tmux set-option -w -t "$(window_id abandoned)" @gl_activity_only_since \
  "$(( $(date +%s) - 400 ))"
equal "an unmeasurable pty keeps the answer could-not-determine" \
  "?unknown? (turn-bracket bound reached)" \
  "$(GANG_ACTIVITY_WINDOW=100000 "$GANG" status abandoned | sed -n '1p')"
tmux set-option -uw -t "$(window_id abandoned)" @gl_activity_only_since
# The decay widens nothing: this send already landed through the
# could-not-determine fall-through, and the bracket is still not a reader's to
# write.
if printf 'MARK_DECAYED' | GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to abandoned --from tester --stdin >/dev/null 2>&1; then
  pass "delivery to a decayed agent lands as an ordinary idle delivery"
else
  abandoned_rc=$?
  fail "delivery to a decayed agent lands as an ordinary idle delivery" \
    "send refused rc $abandoned_rc"
fi
contains "and that delivery landed" "$(pane abandoned)" "MARK_DECAYED"
equal "the decayed verdict leaves the bracket to its native owner, byte-identical" \
  "$abandoned_bracket" \
  "$(tmux show-options -wqv -t "$(window_id abandoned)" @gl_turn)"
"$GANG" drop abandoned >/dev/null

# The quiet leg must be measured, and a collar that does not declare
# quiet-at-rest measures nothing: its harness writes to the pty constantly at
# rest, so the activity tier reports inactive by abstention rather than by
# observation. Spending that as the positive evidence a decay requires would
# decay every abandoned turn on a harness gang cannot hear.
"$HITCH" assumed -c bash -d /tmp >/dev/null
tmux set-option -w -t "$(window_id assumed)" @gl_turn "open $(( $(date +%s) - 400 ))"
equal "a collar that never measures the pty cannot witness the quiet a decay needs" \
  "?unknown? (turn-bracket bound reached)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status assumed | sed -n '1p')"
"$GANG" drop assumed >/dev/null

# Each tier read is a separate moment, and a decay assembled across them can be
# a state no single instant held: the harness resuming between the quiet
# reading and the box reading would have its still-open turn discarded on
# evidence that had already gone stale. This collar writes to its own pty
# whenever gang reads its box — the harness moving underneath the decision,
# made deterministic — and the decay must refuse rather than report idle.
{ printf '. %s\nMOVING_ON=%s\n' "$ROOT/collars/bash.sh" "$RUN_ROOT/moving.on"; cat <<'SH'
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_QUIET_AT_REST=1
eval "$(declare -f collar_input | sed '1s/collar_input/moving_read/')"
collar_input() { # $1 = tmux target; reads the box, then writes below it
  local out rc=0
  out="$(moving_read "$1")" || rc=$?
  [ ! -e "$MOVING_ON" ] \
    || printf '\ntick\n' > "$(tmux display-message -p -t "$1" '#{pane_tty}')" 2>/dev/null
  printf '%s' "$out"
  return $rc
}
SH
} > "$RUN_ROOT/collars/moving.sh"
"$HITCH" moving -c moving -d /tmp >/dev/null
tmux set-option -w -t "$(window_id moving)" @gl_turn "open $(( $(date +%s) - 400 ))"
: > "$RUN_ROOT/moving.on"   # the harness starts moving only now that it is up
equal "a pane that moves while gang is deciding refuses the decay" \
  "?unknown? (the pane was written to while gang was deciding)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status moving | sed -n '1p')"
contains "and roster's snapshot refuses it on the same witness" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^moving ')" "?unknown?"
"$GANG" drop moving >/dev/null

# The earlier seam, and the one the moving fixture cannot reach: the quiet stamp
# is read INSIDE recently_active, so a witness captured after that call leaves
# the reading it rests on outside the guarded interval. A write landing there is
# present in both witnesses, agrees with itself, and passes as if nothing moved —
# the pty accepted as quiet at the instant it was not. Deterministic through a
# tmux shim that writes to the pane exactly when the inactive path clears its
# activity-only bound: after the stamp was read, before any witness could be.
mkdir -p "$RUN_ROOT/bin-seam"
{ printf '#!/usr/bin/env bash\nreal=%s\n' "$(command -v tmux)"; cat <<'SH'
seen=0; target=""; prev=""
for a in "$@"; do
  [ "$a" != @gl_activity_only_since ] || seen=1
  [ "$prev" != -t ] || target="$a"
  prev="$a"
done
if [ "$seen" = 1 ] && [ -n "$target" ]; then
  "$real" "$@"; rc=$?
  tty="$("$real" display-message -p -t "$target" '#{pane_tty}' 2>/dev/null)"
  [ -z "$tty" ] || printf '\ntick-after-quiet-read\n' > "$tty" 2>/dev/null
  exit "$rc"
fi
exec "$real" "$@"
SH
} > "$RUN_ROOT/bin-seam/tmux"
chmod +x "$RUN_ROOT/bin-seam/tmux"
"$HITCH" seam -c abandoned -d /tmp >/dev/null
tmux set-option -w -t "$(window_id seam)" @gl_turn "open $(( $(date +%s) - 400 ))"
equal "a write between the quiet reading and the witness refuses the decay" \
  "?unknown? (the pane was written to while gang was deciding)" \
  "$(PATH="$RUN_ROOT/bin-seam:$PATH" GANG_ACTIVITY_WINDOW=0 "$GANG" status seam | sed -n '1p')"
"$GANG" drop seam >/dev/null

# The delivery half of the same seam. Refusing the decay leaves
# could-not-determine, and send's fall-through delivers into a provably empty
# box on that verdict — correct when the verdict means stale evidence, wrong
# here, where it means gang WATCHED the screen being written to while it
# decided. A harness paints the opening of a turn with its composer still
# empty, so an empty box read out of a moving screen proves nothing and the
# paste lands in live work. The collar's busy marker is what the shim paints,
# so this is a turn starting mid-decision and not merely noise.
cat > "$RUN_ROOT/collars/seamsend.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='tick-after-quiet-read'
GANG_QUIET_AT_REST=1
SH
"$HITCH" seamsend -c seamsend -d /tmp >/dev/null
tmux set-option -w -t "$(window_id seamsend)" @gl_turn "open $(( $(date +%s) - 400 ))"
seamsend_out=""
if seamsend_out="$(printf 'MARK_LIVE_SEND' | PATH="$RUN_ROOT/bin-seam:$PATH" \
  GANG_ACTIVITY_WINDOW=0 "$GANG" send --to seamsend --from tester --stdin 2>&1)"; then
  fail "a turn painted during the decision refuses the delivery" "send succeeded"
else
  pass "a turn painted during the decision refuses the delivery"
fi
contains "naming the moving screen rather than a stale witness" \
  "$seamsend_out" "moving screen"
excludes "and nothing was typed into it" "$(pane seamsend)" "MARK_LIVE_SEND"
equal "so no paste is left staged there either" "" \
  "$(tmux show-options -wqv -t "$(window_id seamsend)" @gl_staged)"
"$GANG" drop seamsend >/dev/null

# A collar that declares no input reader has no box for gang to measure:
# landing_zone falls back to the whole pane, which is never empty, so the
# expired-witness refusal fires on a transcript rather than on a composer.
# Calling that "its input box" blames a draft nobody wrote — the class says
# what gang actually read.
cat > "$RUN_ROOT/collars/noreader.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
unset -f collar_input
SH
"$HITCH" noreader -c noreader -d /tmp >/dev/null
tmux set-option -w -t "$(window_id noreader)" @gl_turn \
  "open $(( $(date +%s) - 400 ))"
if noreader_out="$(printf 'MARK_NOREADER' | GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to noreader --from tester --stdin 2>&1)"; then
  fail "an expired witness over a collar with no input reader refuses" \
    "send succeeded"
else
  pass "an expired witness over a collar with no input reader refuses"
fi
contains "and names the whole pane it actually measured" \
  "$noreader_out" "whole-pane:"
excludes "the refused body never landed" "$(pane noreader)" "MARK_NOREADER"
"$GANG" drop noreader >/dev/null

# Positive control for the stability leg: a churning pane preserves painted
# busy even with the activity credit forced off. The verdict is asserted
# EXACTLY: the broken state reads "?unknown? (busy paint frozen…)", whose
# explanation contains the word busy, so a substring check would false-green
# on the feature's own text. Two determinism traps solved here: the tick
# lines are UNIQUE (a periodic screen scrolls into byte-identical captures
# and genuinely reads stable), and the stability probe's wait runs under a
# fake clock that returns once the pane has actually repainted — the wait
# becomes an event barrier on the change real time would have delivered.
# The ticker paints a ❯ line so its composer reads provably empty — if the
# stability leg were deleted, the demotion would fire and this send would
# DELIVER, turning the send assertions red alongside the verdict.
mkdir -p "$RUN_ROOT/churn-bin"
cat > "$RUN_ROOT/churn-bin/sleep" <<'SH'
#!/bin/sh
# Fake clock for the churn probe: return once the pane has repainted.
[ -n "$CHURN_PANE" ] || exit 0
base="$(tmux capture-pane -pJ -t "$CHURN_PANE" 2>/dev/null)" || exit 0
i=0
while [ "$i" -lt 200 ]; do
  now="$(tmux capture-pane -pJ -t "$CHURN_PANE" 2>/dev/null)" || exit 0
  [ "$now" = "$base" ] || exit 0
  i=$((i + 1))
done
exit 0
SH
chmod +x "$RUN_ROOT/churn-bin/sleep"
cat > "$RUN_ROOT/collars/ticker.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'i=0; while :; do i=\\\$((i + 1)); echo Retrying in 9s tick \\\$i; echo \"❯ \"; tmux wait-for -S \"gltick-\\\$GANG_SESSION\"; done' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
if GANG_BOOT_TIMEOUT=0 "$GANG" hitch ticker -c ticker -d /tmp >/dev/null 2>&1; then
  fail "an always-churning ticker cannot complete a hitch" "hitch reported success"
else
  pass "an always-churning ticker cannot complete a hitch"
fi
tmux set-option -w -t "$(window_id ticker)" @gl_turn "open $(( $(date +%s) - 400 ))"
# Event barrier, not a poll: every ticker iteration signals, so waiting for
# one guarantees at least one full paint is on screen before any verdict.
tmux wait-for "gltick-$GANG_SESSION"
CHURN_PANE="$(tmux list-panes -t "$(window_id ticker)" -F '#{pane_id}')"
equal "a churning pane keeps the busy verdict itself, not its explanation" \
  "-busy-" \
  "$(CHURN_PANE="$CHURN_PANE" PATH="$RUN_ROOT/churn-bin:$PATH" \
     GANG_ACTIVITY_WINDOW=0 "$GANG" status ticker | sed -n '1p')"
if ticker_out="$(printf 'MARK_TICKER' | CHURN_PANE="$CHURN_PANE" \
  PATH="$RUN_ROOT/churn-bin:$PATH" GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to ticker --from tester --stdin 2>&1)"; then
  fail "a churning pane keeps refusing delivery mid-turn" "send succeeded"
else
  pass "a churning pane keeps refusing delivery mid-turn"
fi
contains "with the mid-turn refusal" "$ticker_out" "not safely reachable mid-turn"
excludes "the refused churning-pane body never landed" \
  "$(pane ticker)" "MARK_TICKER"
"$GANG" drop ticker >/dev/null

# An expired bracket over a box that cannot be read at all is refused by the
# occupancy guard upstream of the busy witness — no readable composer on a
# settled screen means a UI of unknown authority owns input, and that refusal
# fires before the bracket is ever weighed. The in-branch unreadable refusal
# stays as a backstop for a box that vanishes between those two reads.
cat > "$RUN_ROOT/collars/blindbox.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_blind_real="\$(declare -f collar_input)"
eval "blind_real_input \${_gl_blind_real#collar_input}"
collar_input() {
  [ ! -f "$RUN_ROOT/blindbox-flag" ] || return 1
  blind_real_input "\$1"
}
SH
"$HITCH" blindbox -c blindbox -d /tmp >/dev/null
tmux set-option -w -t "$(window_id blindbox)" @gl_turn "open $(( $(date +%s) - 400 ))"
touch "$RUN_ROOT/blindbox-flag"
if blind_out="$(printf 'MARK_BLIND' | "$GANG" send --to blindbox --from tester --stdin 2>&1)"; then
  fail "an expired bracket over an unreadable box still refuses" "send succeeded"
else
  pass "an expired bracket over an unreadable box still refuses"
fi
contains "as occupancy of unknown authority" "$blind_out" "authority unknown"
"$GANG" drop blindbox >/dev/null

# A collar-provided native compaction command uses the same verified injection
# primitive. Record execution outside the pane so the typed command cannot
# satisfy its own guard before the shell runs it.
mkdir -p "$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/native.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="printf NATIVE_COMPACT > $RUN_ROOT/native-compact-executed"
SH
export GANG_COLLARS="$RUN_ROOT/collars"
"$HITCH" compactable -c native -d /tmp >/dev/null
"$GANG" compact compactable >/dev/null
equal "compact executes the collar's native command" \
  "NATIVE_COMPACT" "$(cat "$RUN_ROOT/native-compact-executed")"
# A COLLAR THAT DECLARES NO SLOT GETS NOTHING APPENDED. Instructions are typed
# at the harness's own summariser, so a harness that has not been driven to
# accept them must not have text pushed at it on the assumption that it does.
excludes "and appends no instructions to a collar that declares no slot" \
  "$(pane compactable)" "still outstanding in your lane"

# WHERE THE SLOT IS DECLARED, THE INSTRUCTIONS ARE WHAT GETS TYPED. A summary
# chosen without instruction keeps what reads as important, which is not what a
# lane needs to continue.
cat > "$RUN_ROOT/collars/native-slot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="printf SLOTTED_{{instructions}}"
SH
"$HITCH" slotted -c native-slot -d /tmp >/dev/null
"$GANG" compact slotted >/dev/null
slotted_pane="$(pane slotted)"
contains "a declared slot is filled with the continuation instructions" \
  "$slotted_pane" "still outstanding in your lane"
contains "which name the durable state a lane is resumed from" \
  "$slotted_pane" "SLOTTED_Keep the brief you were given, the durable state"
# Orientation, never direction: a default that told a finished agent to carry on
# would make it invent work, which is the worse failure of the two.
excludes "and never tell the agent to keep working" "$slotted_pane" "continue working"
excludes "the placeholder itself is never typed" "$slotted_pane" "{{instructions}}"
"$GANG" drop slotted >/dev/null 2>&1 || :

# A COMPACTION LANDS THE AGENT ON A TURN, NOT AN EMPTY COMPOSER. The continuation
# is typed behind the compaction command and carries Gangline's own attribution.
compactable_pane="$(pane compactable)"
contains "a continuation turn is typed behind the compaction command" \
  "$compactable_pane" "Re-read your brief and the durable state you wrote"
contains "attributed to Gangline itself, not to a peer" \
  "$compactable_pane" "[gang:gangline#"
# Orientation, never direction, for the same reason the keep-instructions are.
excludes "and the default continuation never tells the agent to keep working" \
  "$compactable_pane" "continue working"
"$GANG" compact compactable --resume "RESUME_MARKER_ONLY" >/dev/null
contains "--resume replaces the default continuation" \
  "$(pane compactable)" "RESUME_MARKER_ONLY"
compact_bad_rc=0
"$GANG" compact compactable --resume "   " >/dev/null 2>&1 || compact_bad_rc=$?
equal "a whitespace-only continuation is refused rather than typed" \
  "1" "$compact_bad_rc"
"$GANG" drop compactable >/dev/null 2>&1 || :

# THE CONTINUATION IS MEANT TO PARK, AND THE PARK IS THE LANDING. A harness that
# is compacting queues the turn typed behind the compaction command and submits it
# when the compaction ends. The composer reads clean while the compaction runs and
# carries the hint only once something is queued, which is what claude-code 2.1.227
# was driven doing; a fixture showing the hint earlier would refuse at the
# preflight and prove nothing. Same observed hint as the claude-code collar.
cat > "$RUN_ROOT/compact-queue-rc" <<'RC'
PS1='❯ '
PROMPT_COMMAND='if [ -f "$QUEUE_STRAND" ]; then
                  [ -z "$QUEUE_ARMED" ] || PS1="❯ Press up to edit queued messages"
                  QUEUE_ARMED=1
                fi'
RC
cat > "$RUN_ROOT/collars/compact-queueing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'QUEUE_STRAND=$RUN_ROOT/compact-queue-strand ENV=$RUN_ROOT/compact-queue-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
GANG_COMPACT_CMD="touch $RUN_ROOT/compact-queue-strand"
SH
rm -f "$RUN_ROOT/compact-queue-strand"
"$HITCH" parking -c compact-queueing -d /tmp >/dev/null
if "$GANG" compact parking >/dev/null 2>&1; then
  pass "a continuation the harness parks is accepted, not reported as failed"
else
  fail "a continuation the harness parks is accepted, not reported as failed" \
    "compact reported the park as a failed delivery"
fi
# A DELIBERATE PARK LEAVES NO RECOVERY RECORD. flush exists to rescue a peer
# message the harness swallowed; this one drains itself when the compaction ends,
# and a record here would send an operator after a message already on its way.
equal "and records no parked message for flush to chase" "" \
  "$(tmux show-options -wqv -t "$(window_id parking)" @gl_parked)"
# The exception is scoped to the continuation and does not widen delivery. With
# the fixture's queue still showing, an ordinary peer message is refused on the
# same evidence gang has always refused it on, before anything is typed.
parking_rc=0
parking_out="$(printf 'MARK_PEER_PARKED' \
  | "$GANG" send --to parking --from tester --live-only --stdin 2>&1)" \
  || parking_rc=$?
equal "while an ordinary message into that same queue is still refused" \
  "3" "$parking_rc"
contains "on the parked-queue evidence, before anything is typed" \
  "$parking_out" "parked earlier input"
"$GANG" drop parking >/dev/null 2>&1 || :

# A self-request made inside an agent's own pane must not submit the native
# command during that turn. Stop consumes it once, after which a one-shot worker
# submits the collar command and exits. The worker's unconditional exit barrier
# establishes when its execution artifact can be read without ever waiting on a
# conditional event that a correctly refused submission cannot raise.
self_executed="$RUN_ROOT/self-compact-executed"
self_busy="$RUN_ROOT/self-compact-busy"
cat > "$RUN_ROOT/collars/deferred.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="printf SELF_COMPACT; : > $self_executed"
GANG_SELF_COMPACT=deferred
GANG_BUSY_REGEX='BUSY_DEFERRED'
_gl_self_input="\$(declare -f collar_input)"
eval "self_real_input \${_gl_self_input#collar_input}"
collar_input() {
  [ ! -e "$self_busy" ] || { printf ''; return; }
  self_real_input "\$1"
}
SH
"$HITCH" selfable -c deferred -d /tmp >/dev/null
self_id="$(window_id selfable)"
self_tmux_pane="$(tmux list-panes -t "$self_id" -F '#{pane_id}')"
self_requested="test-self-compact-requested-$$"
self_release="test-self-compact-release-$$"
self_released="test-self-compact-released-$$"
printf -v self_command ': > %q; printf BUSY_DEFERRED; GANG_SESSION=%q GANG_COLLARS=%q %q compact; tmux wait-for -S %q; tmux wait-for %q; rm -f -- %q; tmux wait-for -S %q' \
  "$self_busy" "$GANG_SESSION" "$GANG_COLLARS" "$GANG" "$self_requested" \
  "$self_release" "$self_busy" "$self_released"
tmux send-keys -l -t "$self_id" "$self_command"
tmux send-keys -t "$self_id" Enter
tmux wait-for "$self_requested"
self_request="$(tmux show-options -wqv -t "$self_id" @gl_self_compact_requested)"
contains "the deferred self-compaction request is made while the pane paints busy" \
  "$(pane selfable)" "BUSY_DEFERRED"
equal "deferred self-compaction leaves the busy composer's reading empty" \
  "" "$("$GANG" composer selfable)"
contains "self-compaction records one request inside the running agent" \
  "$(pane selfable)" "self-compaction scheduled for the end of this turn"
excludes "self-compaction does not submit before Stop" \
  "$(pane selfable)" "SELF_COMPACT"
tmux wait-for -S "$self_release"
tmux wait-for "$self_released"

if [ -n "$self_request" ]; then
  tmux wait-for "gang-self-compact-$self_request" &
  self_dispatch_waiter=$!
  printf '%s' '{"hook_event_name":"Stop"}' |
    TMUX_PANE="$self_tmux_pane" "$GANG" hook >/dev/null
  wait "$self_dispatch_waiter"
  if [ -e "$self_executed" ]; then
    pass "native Stop submits the deferred self-compaction command"
  else
    fail "native Stop submits the deferred self-compaction command" \
      "the worker exited without the collar command's execution artifact"
  fi
  equal "the one-shot self-compaction worker exits without an error" "" \
    "$(tmux show-options -wqv -t "$self_id" @gl_self_compact_failed)"
else
  fail "self-compaction records one request inside the running agent" \
    "@gl_self_compact_requested is empty"
fi
"$GANG" drop selfable >/dev/null 2>&1 || :

# TWO STOP EVENTS CAN READ ONE REQUEST BEFORE EITHER CONSUMES IT. tmux has no
# compare-and-set for user options, so the fixture makes that crossing exact:
# both hooks read the standing token through the real tmux, and then both
# detached workers revalidate that same token before either can claim it. The
# dispatching write is the claim the old path performed twice. Its log is
# independent of the option's final value, which two writers can overwrite with
# the same bytes.
compact_race_executed="$RUN_ROOT/compact-race-executed"
compact_race_draft="$RUN_ROOT/compact-race-draft"
cat > "$RUN_ROOT/collars/compact-race.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="printf RACE_COMPACT; printf 'run\\n' >> $compact_race_executed"
GANG_SELF_COMPACT=deferred
GANG_STOP_HOOK=1
_gl_compact_race_input="\$(declare -f collar_input)"
eval "compact_race_real_input \${_gl_compact_race_input#collar_input}"
collar_input() {
  [ ! -e "$compact_race_draft" ] || { printf 'half written operator line'; return; }
  compact_race_real_input "\$1"
}
SH
"$HITCH" compact-race -c compact-race -d /tmp >/dev/null
compact_race_id="$(window_id compact-race)"
compact_race_pane="$(tmux list-panes -t "$compact_race_id" -F '#{pane_id}')"
compact_race_holder="$(tmux display-message -p '#{pid}')"
compact_race_token="test-compact-race-$$"
compact_race_channel="test-compact-race-crossed-$$"
compact_race_bin="$RUN_ROOT/compact-race-bin"
compact_race_arm="$RUN_ROOT/compact-race-arm"
compact_race_pair="$RUN_ROOT/compact-race-pair"
compact_race_reads="$RUN_ROOT/compact-race-reads"
compact_race_claims="$RUN_ROOT/compact-race-claims"
compact_race_dispatches="$RUN_ROOT/compact-race-dispatches"
compact_race_done="$RUN_ROOT/compact-race-done"
compact_race_release="$RUN_ROOT/compact-race-release"
compact_race_confirm="$RUN_ROOT/compact-race-confirm"
mkdir -p "$compact_race_bin" "$compact_race_pair" "$compact_race_done"
{
  printf '#!/usr/bin/env bash\n'
  printf 'REAL=%q\n' "$(command -v tmux)"
  printf 'ARM=%q\n' "$compact_race_arm"
  printf 'PAIR=%q\n' "$compact_race_pair"
  printf 'READS=%q\n' "$compact_race_reads"
  printf 'DISPATCHES=%q\n' "$compact_race_dispatches"
  printf 'DONE=%q\n' "$compact_race_done"
  printf 'CHANNEL=%q\n' "$compact_race_channel"
  cat <<'SH'
request=0 dispatch=0 release=0
for arg in "$@"; do
  case "$arg" in
    @gl_self_compact_requested) request=1 ;;
    @gl_self_compact_dispatching)
      case "${1:-}" in
        set-option) case " $* " in *' -uw '*) release=1 ;; *) dispatch=1 ;; esac ;;
      esac ;;
  esac
done
if [ "$request" -eq 1 ] && [ "${1:-}" = show-options ] && [ -e "$ARM" ]; then
  out="$("$REAL" "$@")"; rc=$?
  printf '%s\n' "$PPID" >> "$READS"
  slot=
  for candidate in 1 2 3 4; do
    if mkdir "$PAIR/$candidate" 2>/dev/null; then
      slot="$candidate"
      break
    fi
  done
  case "$slot" in
    1) "$REAL" wait-for "$CHANNEL-read-outer" ;;
    2) "$REAL" wait-for -S "$CHANNEL-read-outer" ;;
    3) "$REAL" wait-for "$CHANNEL-read-worker" ;;
    4) rm -f -- "$ARM"
       "$REAL" wait-for -S "$CHANNEL-read-worker" ;;
    *) printf 'compact-race fixture observed an unexpected request read\n' >&2
       exit 97 ;;
  esac
  printf '%s\n' "$out"
  exit "$rc"
fi
if [ "$dispatch" -eq 1 ]; then
  "$REAL" "$@"; rc=$?
  [ "$rc" -ne 0 ] || printf '%s\n' "$PPID" >> "$DISPATCHES"
  exit "$rc"
fi
if [ "$release" -eq 1 ]; then
  "$REAL" "$@"; rc=$?
  if mkdir "$DONE/1" 2>/dev/null; then n=1
  elif mkdir "$DONE/2" 2>/dev/null; then n=2
  elif mkdir "$DONE/3" 2>/dev/null; then n=3
  else n=overflow
  fi
  "$REAL" wait-for -S "$CHANNEL-done-$n"
  exit "$rc"
fi
exec "$REAL" "$@"
SH
} > "$compact_race_bin/tmux"
{
  printf '#!/usr/bin/env bash\n'
  printf 'REAL=%q\n' "$(command -v ln)"
  printf 'TMUX=%q\n' "$(command -v tmux)"
  printf 'CLAIMS=%q\n' "$compact_race_claims"
  printf 'CHANNEL=%q\n' "$compact_race_channel"
  printf 'RELEASE=%q\n' "$compact_race_release"
  printf 'CONFIRM=%q\n' "$compact_race_confirm"
  printf 'DRAFT=%q\n' "$compact_race_draft"
  cat <<'SH'
last="${!#}"
case "$last" in
  *self-compact-*.claim)
    "$REAL" "$@"; rc=$?
    printf '%s\n' "$rc" >> "$CLAIMS"
    if [ -e "$RELEASE" ]; then
      if [ "$rc" -eq 0 ]; then
        [ -e "$CONFIRM" ] || "$TMUX" wait-for "$CHANNEL-claim"
      else
        "$TMUX" wait-for -S "$CHANNEL-claim"
        "$TMUX" wait-for "$CHANNEL-winner-released"
        rm -f -- "$DRAFT"
        touch "$CONFIRM"
      fi
    else
      if [ "$rc" -eq 0 ]; then
        "$TMUX" wait-for "$CHANNEL-claim"
      fi
    fi
    exit "$rc" ;;
esac
exec "$REAL" "$@"
SH
} > "$compact_race_bin/ln"
{
  printf '#!/usr/bin/env bash\n'
  printf 'REAL=%q\n' "$(command -v rm)"
  printf 'TMUX=%q\n' "$(command -v tmux)"
  printf 'RELEASE=%q\n' "$compact_race_release"
  printf 'CONFIRM=%q\n' "$compact_race_confirm"
  printf 'CHANNEL=%q\n' "$compact_race_channel"
  cat <<'SH'
last="${!#}"
case "$last" in
  *self-compact-*.claim)
    "$REAL" "$@"; rc=$?
    if [ -e "$RELEASE" ]; then
      if [ -e "$CONFIRM" ]; then
        "$TMUX" wait-for -S "$CHANNEL-loser-done"
      else
        "$TMUX" wait-for -S "$CHANNEL-winner-released"
      fi
    fi
    exit "$rc" ;;
esac
exec "$REAL" "$@"
SH
} > "$compact_race_bin/rm"
{
  printf '#!/usr/bin/env bash\n'
  printf 'REAL=%q\n' "$(command -v readlink)"
  printf 'TMUX=%q\n' "$(command -v tmux)"
  printf 'RELEASE=%q\n' "$compact_race_release"
  printf 'CHANNEL=%q\n' "$compact_race_channel"
  printf 'HOLDER=%q\n' "$compact_race_holder"
  cat <<'SH'
last="${!#}"
case "$last" in
  *self-compact-*.claim)
    out="$("$REAL" "$@")"; rc=$?
    if [ "$rc" -eq 0 ] && [ ! -e "$RELEASE" ]; then
      # Keep the liveness observation true after releasing the winner. The
      # disposable fixture's tmux server outlives every worker in this test.
      printf '%s\n' "$HOLDER"
      "$TMUX" wait-for -S "$CHANNEL-loser-observed"
      "$TMUX" wait-for -S "$CHANNEL-claim"
      exit 0
    fi
    printf '%s\n' "$out"
    exit "$rc" ;;
esac
exec "$REAL" "$@"
SH
} > "$compact_race_bin/readlink"
chmod +x "$compact_race_bin/tmux" "$compact_race_bin/ln" \
  "$compact_race_bin/rm" "$compact_race_bin/readlink"
tmux set-option -w -t "$compact_race_id" @gl_self_compact_requested "$compact_race_token"
: > "$compact_race_arm"
: > "$compact_race_claims"
tmux wait-for "gang-self-compact-$compact_race_token" &
compact_race_waiter=$!
tmux wait-for "$compact_race_channel-loser-observed" &
compact_race_loser_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' \
  | PATH="$compact_race_bin:$PATH" TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null &
compact_race_first=$!
printf '%s' '{"hook_event_name":"Stop"}' \
  | PATH="$compact_race_bin:$PATH" TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null &
compact_race_second=$!
wait "$compact_race_first"
wait "$compact_race_second"
wait "$compact_race_waiter"
wait "$compact_race_loser_waiter"
compact_race_read_count="$(wc -l < "$compact_race_reads" | tr -d ' ')"
compact_race_dispatch_count="$(wc -l < "$compact_race_dispatches" | tr -d ' ')"
equal "both Stops and both workers read the same request before it is claimed" \
  "4" "$compact_race_read_count"
equal "the workers make one winning and one losing atomic claim" \
  $'0\n1' "$(sort "$compact_race_claims")"
equal "one self-compaction token is claimed exactly once across crossed Stops" \
  "1" "$compact_race_dispatch_count"
# A losing claim has no state-changing path after its recorded failed ln. Wait
# for every winning worker before ending its window; this is a counted event
# barrier, not a guess that the compaction process has gone.
compact_race_n=1
while [ "$compact_race_n" -le "$compact_race_dispatch_count" ]; do
  tmux wait-for "$compact_race_channel-done-$compact_race_n"
  compact_race_n=$((compact_race_n + 1))
done
equal "the one claimed request submits one compaction command" \
  "1" "$(wc -l < "$compact_race_executed" | tr -d ' ')"
equal "and no successful peer can restore the consumed request" "" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_requested)"

# A CLAIM FAILURE IS NOT A PEER LOSS. A path that cannot be a lock directory
# makes event_claim fail before any compaction or delivery lock exists. That
# terminal outcome must remain visible rather than look like healthy contention.
# Without a claim it cannot safely clear request state, so the same token stays
# scheduled and becomes runnable when the lock fault is repaired.
compact_claim_bad_root="$RUN_ROOT/compact-claim-not-directory"
compact_claim_bad_token="test-compact-claim-failed-$$"
touch "$compact_claim_bad_root"
tmux set-option -w -t "$compact_race_id" @gl_self_compact_requested \
  "$compact_claim_bad_token"
tmux wait-for "gang-self-compact-$compact_claim_bad_token" &
compact_claim_bad_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_LOCK_DIR="$compact_claim_bad_root" TMUX_PANE="$compact_race_pane" \
    "$GANG" hook >/dev/null
wait "$compact_claim_bad_waiter"
equal "a broken claim root leaves the exact request retryable" \
  "$compact_claim_bad_token" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_requested)"
contains "claim infrastructure failure records the terminal outcome" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_failed)" \
  "could not establish the self-compaction claim"
contains "status exposes the claim failure" "$("$GANG" status compact-race)" \
  "self-compaction NOT submitted after Stop"
contains "status also says the failed request remains pending" \
  "$("$GANG" status compact-race)" \
  "self-compaction requested; waiting for the turn boundary"
equal "claim infrastructure failure submits no second compaction command" \
  "1" "$(wc -l < "$compact_race_executed" | tr -d ' ')"
excludes "an unclaimed failure does not risk a duplicate spool note" \
  "$("$GANG" status compact-race)" "spooled:"

# Repairing the claim root is sufficient: the next boundary takes the same
# token through the ordinary claimed path and retires the failure record.
tmux wait-for "gang-self-compact-$compact_claim_bad_token" &
compact_claim_repair_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null
wait "$compact_claim_repair_waiter"
equal "a later boundary runs the request after claim-root repair" \
  "2" "$(wc -l < "$compact_race_executed" | tr -d ' ')"
equal "the repaired attempt consumes the standing request" "" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_requested)"
equal "the repaired attempt retires the claim failure" "" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_failed)"

# A FAILURE WRITE CAN ARRIVE AFTER A REPAIRED SUCCESS. Hold the old worker at
# its tmux write, let a healthy boundary consume the same token and clear its
# state, then release the stale write. The raw history remains inspectable, but
# token-aware status and roster must not paint it as the current request.
compact_delay_token="test-compact-delayed-failure-$$"
compact_delay_bin="$RUN_ROOT/compact-delay-bin"
compact_delay_channel="test-compact-delay-$$"
mkdir "$compact_delay_bin"
{
  printf '#!/usr/bin/env bash\n'
  printf 'REAL=%q\n' "$(command -v tmux)"
  printf 'CHANNEL=%q\n' "$compact_delay_channel"
  cat <<'SH'
case " $* " in
  *' set-option '*' @gl_self_compact_failed '*)
    "$REAL" wait-for -S "$CHANNEL-ready"
    "$REAL" wait-for "$CHANNEL-release"
    "$REAL" "$@"; rc=$?
    "$REAL" wait-for -S "$CHANNEL-written"
    exit "$rc" ;;
esac
exec "$REAL" "$@"
SH
} > "$compact_delay_bin/tmux"
chmod +x "$compact_delay_bin/tmux"
tmux set-option -w -t "$compact_race_id" @gl_self_compact_requested \
  "$compact_delay_token"
tmux wait-for "$compact_delay_channel-ready" &
compact_delay_ready_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' \
  | PATH="$compact_delay_bin:$PATH" GANG_LOCK_DIR="$compact_claim_bad_root" \
    TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null
wait "$compact_delay_ready_waiter"
tmux wait-for "gang-self-compact-$compact_delay_token" &
compact_delay_success_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null
wait "$compact_delay_success_waiter"
tmux wait-for "$compact_delay_channel-written" &
compact_delay_written_waiter=$!
tmux wait-for -S "$compact_delay_channel-release"
wait "$compact_delay_written_waiter"
equal "a repaired boundary consumes the token before its delayed failure writes" "" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_requested)"
equal "that repaired boundary submits one compact command" \
  "3" "$(wc -l < "$compact_race_executed" | tr -d ' ')"
contains "the delayed failure retains its exact historical token" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_failed)" \
  "[request:$compact_delay_token]"
excludes "status suppresses a delayed failure whose token is no longer current" \
  "$("$GANG" status compact-race)" "self-compaction NOT submitted after Stop"
excludes "roster suppresses the same obsolete failure" \
  "$("$GANG" roster)" "self-compact-failed"
tmux set-option -uw -t "$compact_race_id" @gl_self_compact_failed

# PID zero addresses a process group to kill(2), not a claim holder. Accepting
# it as a live peer would suppress every attempt forever.
compact_zero_token="test-compact-zero-holder-$$"
compact_zero_claim="$GANG_LOCK_DIR/self-compact-$(printf '%s' \
  "$compact_race_id-$compact_zero_token" | tr -c 'A-Za-z0-9' '_').claim"
ln -s 0 "$compact_zero_claim"
tmux set-option -w -t "$compact_race_id" @gl_self_compact_requested \
  "$compact_zero_token"
tmux wait-for "gang-self-compact-$compact_zero_token" &
compact_zero_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null
wait "$compact_zero_waiter"
equal "a zero-holder claim leaves the exact request retryable" \
  "$compact_zero_token" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_requested)"
contains "a zero-holder claim is a visible malformed artifact" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_failed)" \
  "invalid process id '0'"
rm -f -- "$compact_zero_claim"
tmux set-option -uw -t "$compact_race_id" @gl_self_compact_requested
tmux set-option -uw -t "$compact_race_id" @gl_self_compact_failed

# A LOSER CAN INSPECT ONLY AFTER A REFUSING WINNER RELEASES. The winner restores
# token R before dropping its claim. The ln wrapper holds the loser after its
# failed creation until that release, then makes the confirmation claim and rm
# observable. The loser must classify the vanished peer as contention and must
# not clear R; the waiting note then owns the next Stop as usual.
compact_release_token="test-compact-release-race-$$"
rmdir "$compact_race_pair/1" "$compact_race_pair/2" \
  "$compact_race_pair/3" "$compact_race_pair/4"
: > "$compact_race_arm"
touch "$compact_race_release"
tmux set-option -w -t "$compact_race_id" @gl_self_compact_requested \
  "$compact_release_token"
touch "$compact_race_draft"
tmux wait-for "$compact_race_channel-loser-done" &
compact_release_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' \
  | PATH="$compact_race_bin:$PATH" TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null &
compact_release_first=$!
printf '%s' '{"hook_event_name":"Stop"}' \
  | PATH="$compact_race_bin:$PATH" TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null &
compact_release_second=$!
wait "$compact_release_first"
wait "$compact_release_second"
wait "$compact_release_waiter"
rm -f -- "$compact_race_release" "$compact_race_confirm"
equal "a loser inspecting after peer release preserves the restored request" \
  "$compact_release_token" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_requested)"
equal "the refusing winner and crossed loser submit no compact command" \
  "3" "$(wc -l < "$compact_race_executed" | tr -d ' ')"
contains "the refusing winner records that the request still stands" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_failed)" \
  "still scheduled"
contains "the refusing winner leaves one note for a later boundary" \
  "$("$GANG" status compact-race)" "spooled:"

tmux wait-for "gang-spool-drain-$compact_race_id" &
compact_release_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$compact_race_pane" "$GANG" hook >/dev/null
wait "$compact_release_drain_waiter"
equal "the failure note's boundary leaves the restored request standing" \
  "$compact_release_token" \
  "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_requested)"
compact_release_status="$("$GANG" status compact-race)"
case "$compact_release_status" in
  *spooled:*) fail "the refusal note drains before a compaction retry is armed" \
                "status still reports waiting mail" ;;
  *) pass "the refusal note drains before a compaction retry is armed"
     # The first refusing worker already signalled the token-keyed barrier.
     # Observe this worker through dispatch cleanup, so that earlier signal
     # cannot satisfy the witness for a later boundary using the same token.
     compact_release_finish_bin="$RUN_ROOT/compact-release-finish-bin"
     mkdir "$compact_release_finish_bin"
     ln -s "$compact_race_bin/tmux" "$compact_release_finish_bin/tmux"
     tmux wait-for "$compact_race_channel-done-3" &
     compact_release_retry_waiter=$!
     printf '%s' '{"hook_event_name":"Stop"}' \
       | PATH="$compact_release_finish_bin:$PATH" TMUX_PANE="$compact_race_pane" \
         "$GANG" hook >/dev/null
     wait "$compact_release_retry_waiter"
     equal "a later mail-free boundary submits the restored request once" \
       "4" "$(wc -l < "$compact_race_executed" | tr -d ' ')"
     equal "that successful retry consumes the request" "" \
       "$(tmux show-options -wqv -t "$compact_race_id" @gl_self_compact_requested)" ;;
esac
"$GANG" drop compact-race >/dev/null 2>&1 || :

# A BOUNDARY WHOSE COMPOSER BELONGS TO SOMEBODY ELSE DEFERS, IT DOES NOT DROP.
# The delivery guards inside inject refuse with status 3, and the dispatcher
# used to spend its one request on that boundary: the agent went idle believing
# it had compacted, nothing retried, and the only record was a tmux option no
# agent reads. A draft in the box is the ordinary case at a Stop, because a
# Stop is exactly when an operator is mid-reply.
drafted_executed="$RUN_ROOT/self-drafted-executed"
drafted_busy="$RUN_ROOT/self-drafted-busy"
drafted_draft="$RUN_ROOT/self-drafted-draft"
cat > "$RUN_ROOT/collars/drafted.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="printf DRAFTED_COMPACT; : > $drafted_executed"
GANG_SELF_COMPACT=deferred
GANG_STOP_HOOK=1
GANG_BUSY_REGEX='BUSY_DRAFTED'
_gl_drafted_input="\$(declare -f collar_input)"
eval "drafted_real_input \${_gl_drafted_input#collar_input}"
collar_input() {
  [ ! -e "$drafted_busy" ] || { printf ''; return; }
  [ ! -e "$drafted_draft" ] || { printf 'half written operator line'; return; }
  drafted_real_input "\$1"
}
SH
"$HITCH" drafted -c drafted -d /tmp >/dev/null
drafted_id="$(window_id drafted)"
drafted_tmux_pane="$(tmux list-panes -t "$drafted_id" -F '#{pane_id}')"
drafted_requested="test-drafted-requested-$$"
drafted_release="test-drafted-release-$$"
drafted_released="test-drafted-released-$$"
printf -v drafted_command ': > %q; printf BUSY_DRAFTED; GANG_SESSION=%q GANG_COLLARS=%q %q compact; tmux wait-for -S %q; tmux wait-for %q; rm -f -- %q; tmux wait-for -S %q' \
  "$drafted_busy" "$GANG_SESSION" "$GANG_COLLARS" "$GANG" "$drafted_requested" \
  "$drafted_release" "$drafted_busy" "$drafted_released"
tmux send-keys -l -t "$drafted_id" "$drafted_command"
tmux send-keys -t "$drafted_id" Enter
tmux wait-for "$drafted_requested"
drafted_request="$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_requested)"
tmux wait-for -S "$drafted_release"
tmux wait-for "$drafted_released"

# The draft lands before the boundary, so the dispatcher meets an occupied box.
: > "$drafted_draft"
if [ -n "$drafted_request" ]; then
  tmux wait-for "gang-self-compact-$drafted_request" &
  drafted_waiter=$!
  printf '%s' '{"hook_event_name":"Stop"}' |
    TMUX_PANE="$drafted_tmux_pane" "$GANG" hook >/dev/null
  wait "$drafted_waiter"
  excludes "a refused boundary submits no compaction command" \
    "$(pane drafted)" "DRAFTED_COMPACT"
  equal "a refused boundary puts the self-compaction request back" \
    "$drafted_request" \
    "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_requested)"
  contains "the recorded failure says the request still stands" \
    "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_failed)" \
    "still scheduled"
  equal "the deferral note is keyed to this request token" \
    "$drafted_request" \
    "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_noted)"
  # WHAT THIS PROVES IS THAT THE NOTE IS WAITING, not that it arrived. Only a
  # Stop or a PostCompact drains a spool, and the refused boundary raised
  # neither, so an agent that goes idle here is still not told. That gap is
  # real and is not closed by this change.
  contains "the deferral leaves a note waiting in the agent's own spool" \
    "$("$GANG" status drafted)" "spooled: 1"
  contains "status reports the deferred self-compaction as still pending" \
    "$("$GANG" status drafted)" "self-compaction requested"

  # Mail already waiting owns the next boundary. The old path launched its
  # drain beside the compaction worker and let the pane-lock race decide which
  # happened; this boundary drains the note and leaves the request untouched.
  rm -f -- "$drafted_draft"
  drafted_retry="$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_requested)"
  # A request that was NOT put back leaves nothing to wait on, and a barrier
  # keyed to it would hang instead of failing. The claim above already reports
  # that; this one refuses to arm a wait it cannot satisfy.
  if [ -n "$drafted_retry" ]; then
    tmux wait-for "gang-spool-drain-$drafted_id" &
    drafted_drain_waiter=$!
    printf '%s' '{"hook_event_name":"Stop"}' |
      TMUX_PANE="$drafted_tmux_pane" "$GANG" hook >/dev/null
    wait "$drafted_drain_waiter"
    equal "waiting mail leaves the compaction request for another boundary" \
      "$drafted_retry" \
      "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_requested)"
    excludes "and that boundary drains the one deferral note" \
      "$("$GANG" status drafted)" "spooled:"
    if [ -e "$drafted_executed" ]; then
      fail "mail-first dispatch does not execute the compaction beside its drain" \
        "the compact command's execution artifact already exists"
    else
      pass "mail-first dispatch does not execute the compaction beside its drain"
    fi

    # With no mail left, the following boundary has one action and consumes the
    # same request. Its successful episode retires both the failure and note id.
    tmux wait-for "gang-self-compact-$drafted_retry" &
    drafted_retry_waiter=$!
    printf '%s' '{"hook_event_name":"Stop"}' |
      TMUX_PANE="$drafted_tmux_pane" "$GANG" hook >/dev/null
    wait "$drafted_retry_waiter"
    if [ -e "$drafted_executed" ]; then
      equal "a submitted retry consumes the request" "" \
        "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_requested)"
      equal "a submitted retry clears the recorded failure" "" \
        "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_failed)"
      equal "and retires the completed episode's note identity" "" \
        "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_noted)"
    else
      fail "a submitted retry consumes the request" \
        "the mail-free boundary completed without running the compact command"
      fail "a submitted retry clears the recorded failure" \
        "the mail-free boundary completed without running the compact command"
      fail "and retires the completed episode's note identity" \
        "the mail-free boundary completed without running the compact command"
    fi
  else
    fail "a submitted retry consumes the request" \
      "the deferred request was dropped, so no later boundary could run it"
    fail "a submitted retry clears the recorded failure" \
      "the deferred request was dropped, so no later boundary could run it"
  fi
else
  fail "a refused boundary puts the self-compaction request back" \
    "@gl_self_compact_requested was empty before the boundary"
fi

# A COMMAND THAT REPORTS NO EFFECT MUST HAVE HAD NONE. A refused boundary puts
# its request back, so a standing request is an ordinary state and a second
# compact an ordinary thing to try — and the continuation used to be written
# before anything checked whether the request could be made at all. The state
# is set here rather than driven, because what is under test is the order of
# two writes inside one command.
tmux set-option -w -t "$drafted_id" @gl_self_compact_requested standing
tmux set-option -w -t "$drafted_id" @gl_self_compact_resume MARK_STANDING_TURN
if repeat_out="$(TMUX_PANE="$drafted_tmux_pane" "$GANG" compact 2>&1)"; then
  fail "a second self-compaction refuses while one stands" \
    "compact reported success"
else
  pass "a second self-compaction refuses while one stands"
fi
contains "naming the request that already stands" \
  "$repeat_out" "already waiting for this turn to end"
equal "and a bare repeat leaves the standing continuation where it was" \
  "MARK_STANDING_TURN" \
  "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_resume)"
if TMUX_PANE="$drafted_tmux_pane" "$GANG" compact --resume MARK_REPLACEMENT \
  >/dev/null 2>&1; then
  fail "a repeat carrying its own continuation refuses too" \
    "compact reported success"
else
  pass "a repeat carrying its own continuation refuses too"
fi
equal "and does not install the continuation it was refused" \
  "MARK_STANDING_TURN" \
  "$(tmux show-options -wqv -t "$drafted_id" @gl_self_compact_resume)"
tmux set-option -uw -t "$drafted_id" @gl_self_compact_requested
tmux set-option -uw -t "$drafted_id" @gl_self_compact_resume

"$GANG" drop drafted >/dev/null 2>&1 || :

# Without the deferred declaration, the same self-call takes the direct path
# and puts the native command into the tty while the caller's turn is active.
nodeferred_busy="$RUN_ROOT/nodeferred-compact-busy"
cat > "$RUN_ROOT/collars/nodeferred.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="/compact"
GANG_BUSY_REGEX='BUSY_NODEFERRED_COMPACT'
_gl_nodeferred_input="\$(declare -f collar_input)"
eval "nodeferred_real_input \${_gl_nodeferred_input#collar_input}"
collar_input() {
  [ ! -e "$nodeferred_busy" ] || { printf ''; return; }
  nodeferred_real_input "\$1"
}
SH
"$HITCH" nodeferred -c nodeferred -d /tmp >/dev/null
nodeferred_id="$(window_id nodeferred)"
nodeferred_observed="test-nodeferred-compact-observed-$$"
nodeferred_release="test-nodeferred-compact-release-$$"
printf -v nodeferred_command ': > %q; printf BUSY_NODEFERRED_COMPACT; GANG_SESSION=%q GANG_COLLARS=%q %q compact nodeferred >/dev/null 2>&1 || :; tmux wait-for -S %q; tmux wait-for %q; rm -f -- %q' \
  "$nodeferred_busy" "$GANG_SESSION" "$GANG_COLLARS" "$GANG" \
  "$nodeferred_observed" "$nodeferred_release" "$nodeferred_busy"
tmux send-keys -l -t "$nodeferred_id" "$nodeferred_command"
tmux send-keys -t "$nodeferred_id" Enter
tmux wait-for "$nodeferred_observed"
contains "without deferral the same self-call types the compact command mid-turn" \
  "$(pane nodeferred)" "/compact"
equal "the undeferred path stamps no pending self-compaction request" "" \
  "$(tmux show-options -wqv -t "$nodeferred_id" @gl_self_compact_requested)"
tmux wait-for -S "$nodeferred_release"
"$GANG" drop nodeferred >/dev/null

# A TRANSPORT REFUSAL IS NOT A READING. Every predicate below asks the pane a
# question and spends the answer as evidence: no box drawn means nothing owns
# the screen, a box that reads the same twice means nobody is typing, a witness
# that matches itself means nothing moved. A capture that never happened
# answers none of them, and nothing a healthy tmux does produces one — so the
# fixture refuses the transport underneath a live, healthy pane, and checks
# that it did.
mkdir -p "$RUN_ROOT/refuse-bin"
{
  printf '#!/bin/sh\n'
  printf 'REAL=%q\n' "$(command -v tmux)"
  printf 'LOG=%q\n' "$RUN_ROOT/refuse-log"
  printf 'COUNT=%q\n' "$RUN_ROOT/refuse-count"
  cat <<'SH'
if [ "${1:-}" = capture-pane ] && [ -n "${REFUSE_CAPTURE_FROM:-}" ]; then
  n=0
  [ ! -s "$COUNT" ] || n="$(cat "$COUNT")"
  n=$((n + 1))
  printf '%s\n' "$n" > "$COUNT"
  if [ "$n" -ge "$REFUSE_CAPTURE_FROM" ]; then
    printf 'MARK_CAPTURE_REFUSED\n' >&2
    printf '%s\n' "$n" >> "$LOG"
    exit 42
  fi
fi
exec "$REAL" "$@"
SH
} > "$RUN_ROOT/refuse-bin/tmux"
chmod +x "$RUN_ROOT/refuse-bin/tmux"

refuse_capture() { # $1 = first capture-pane call to refuse, rest = the command
  local from="$1"
  shift
  rm -f -- "$RUN_ROOT/refuse-count" "$RUN_ROOT/refuse-log"
  ( export PATH="$RUN_ROOT/refuse-bin:$PATH" REFUSE_CAPTURE_FROM="$from"; "$@" )
}

# WHICH capture a caller takes is a property of its path, not a constant a
# fixture may assume. Each index is refused in turn until the run names the
# unknown; an index that lands past every read ends the sweep, so a caller that
# stopped reading the pane there cannot pass this by pointing at some other
# guard, and a wording that arrives with nothing refused cannot pass it at all.
refuses_a_refused_capture() { # $1 description, $2 expected wording, rest = command
  local description="$1" expected="$2" i=1 out rc
  shift 2
  while [ "$i" -le 24 ]; do
    rc=0
    out="$(refuse_capture "$i" "$@" 2>&1)" || rc=$?
    case "$out" in
      *"$expected"*)
        if [ "$rc" -eq 0 ]; then
          fail "$description" "named the unknown and still reported success"
        elif [ ! -s "$RUN_ROOT/refuse-log" ]; then
          fail "$description" "the wording appeared with no capture refused"
        else
          pass "$description"
        fi
        return ;;
    esac
    [ "$rc" -ne 0 ] || break
    i=$((i + 1))
  done
  fail "$description" "no refused capture in this path reached [$expected]"
}

"$HITCH" refused -c bash -d /tmp >/dev/null
refused_id="$(window_id refused)"

send_to_refused() {
  printf 'MARK_REFUSED_SEND' | "$GANG" send --to refused --from tester --stdin --live-only
}
status_of_refused() { "$GANG" status refused; }

# A refused capture on a live agent used to leave `occupied` with no box drawn
# and no busy marker declared, and the negation of an unread busy tier turned
# that into a UI owning the screen — a state read off a pane nobody looked at.
refused_status_rc=0
refused_status="$(refuse_capture 1 status_of_refused 2>&1)" || refused_status_rc=$?
if [ "$refused_status_rc" -eq 0 ]; then
  fail "a status whose captures are all refused reports no state" \
    "status succeeded: [$refused_status]"
else
  pass "a status whose captures are all refused reports no state"
fi
contains "and names the reading it could not take" \
  "$refused_status" "refusing to guess occupancy"
excludes "rather than inventing an occupancy out of it" \
  "$refused_status" "!occupied!"
equal "the pane it refused about was alive throughout" 0 \
  "$(tmux display-message -p -t "$refused_id" '#{pane_dead}')"

# Settled is a positive finding a caller spends by pasting into that composer.
refuses_a_refused_capture \
  "a send whose composer read is refused does not read the box as settled" \
  "whether somebody is typing into it is unknown" \
  send_to_refused
equal "and nothing was typed into the box it could not read" "" \
  "$("$GANG" composer refused)"

# Two witnesses assembled out of failed reads are equal to each other, and an
# open turn bracket past its bound decays to idle against that self-agreement.
tmux set-option -w -t "$refused_id" @gl_turn "open 1"
refused_stale="$("$GANG" status refused)"
contains "an abandoned turn bracket with every capture answered is undetermined" \
  "$refused_stale" "?unknown?"
refuses_a_refused_capture \
  "a decay witness that could not be taken refuses instead of matching itself" \
  "refusing to witness a decay against a reading that was never taken" \
  status_of_refused
tmux set-option -uw -t "$refused_id" @gl_turn

# THE COLLAR'S OWN ANSWER, asked directly. bin/gang takes a second look at the
# pane for a collar that cannot tell a refused read from an absent box, and
# that backstop would cover for a shipped collar losing the distinction — a
# covered-for collar still reports a pane it never read as a pane with no box.
for refused_collar in bash claude-code codex opencode pi; do
  refused_collar_rc=0
  rm -f -- "$RUN_ROOT/refuse-count"
  (
    # shellcheck source=/dev/null
    . "$ROOT/collars/$refused_collar.sh"
    export PATH="$RUN_ROOT/refuse-bin:$PATH" REFUSE_CAPTURE_FROM=1
    collar_input "$refused_id" >/dev/null 2>&1
  ) || refused_collar_rc=$?
  equal "collar $refused_collar reads a refused capture as unreadable, not as an absent box" \
    3 "$refused_collar_rc"
done

# THE SAME PANE, THE OTHER READING. A collar's context reading spends nothing —
# the command ends whichever way it fails — but the refusal an operator gets
# decides where they look next, and a capture piped into a parser makes every
# refused pane read arrive as a missing context readout. Codex is absent from
# this loop because it reads a rollout file rather than the pane.
#
# BOTH OUTCOMES, OR NEITHER IS EVIDENCE. A collar that refused everything would
# pass a refusal check on its own, so each collar is asked the same question
# twice against the same live pane: once with the transport refusing, and once
# with it answering a pane that carries no readout. The pair is the assertion —
# before this contract both answers were the same one.
#
# `die` and `refuse` belong to bin/gang, and a collar sourced on its own has
# neither. Stood in here at their exit statuses, so what is read below is which
# of the two the collar reached.
cat > "$RUN_ROOT/collar-refusal-stand-ins" <<'SH'
die() { printf 'gang: %s\n' "$*" >&2; exit 1; }
refuse() { printf 'gang: %s\n' "$*" >&2; exit 3; }
SH
for refused_collar in bash claude-code opencode pi; do
  refused_ctx_rc=0
  rm -f -- "$RUN_ROOT/refuse-count" "$RUN_ROOT/refuse-log"
  refused_ctx_err="$(
    # shellcheck source=/dev/null
    . "$RUN_ROOT/collar-refusal-stand-ins"
    # shellcheck source=/dev/null
    . "$ROOT/collars/$refused_collar.sh"
    export PATH="$RUN_ROOT/refuse-bin:$PATH" REFUSE_CAPTURE_FROM=1
    collar_context "$refused_id" 2>&1 >/dev/null
  )" || refused_ctx_rc=$?
  equal "collar $refused_collar refuses a context read it could not take" \
    3 "$refused_ctx_rc"
  contains "and collar $refused_collar names the pane rather than a missing readout" \
    "$refused_ctx_err" "cannot read pane"
  if [ ! -s "$RUN_ROOT/refuse-log" ]; then
    fail "collar $refused_collar was asked about a capture that was actually refused" \
      "no capture was refused"
  else
    pass "collar $refused_collar was asked about a capture that was actually refused"
  fi
  readable_ctx_rc=0
  readable_ctx_err="$(
    # shellcheck source=/dev/null
    . "$RUN_ROOT/collar-refusal-stand-ins"
    # shellcheck source=/dev/null
    . "$ROOT/collars/$refused_collar.sh"
    collar_context "$refused_id" 2>&1 >/dev/null
  )" || readable_ctx_rc=$?
  # Claude's hitch-wired beacon can be absent from a readable frame while the
  # source itself remains healthy: redraws, overlays and transcript output all
  # move it off-screen. Its old status 1 made that transient screen miss
  # indistinguishable from every ordinary source failure one layer up, which is
  # exactly the false unavailable edge this pair is meant to prevent. The
  # other pane collars have not declared that a missing readout is transient.
  readable_ctx_expected=1
  [ "$refused_collar" != claude-code ] || readable_ctx_expected=2
  equal "while a pane collar $refused_collar could read, carrying no readout, classifies that absence" \
    "$readable_ctx_expected" "$readable_ctx_rc"
  excludes "so collar $refused_collar keeps the two apart" \
    "$readable_ctx_err" "cannot read pane"
done

"$GANG" drop refused >/dev/null

# A REFUSAL THAT HEALS, THROUGH A COLLAR THAT CANNOT REPORT ONE. Gang asks the
# pane directly when a collar answers "no box", and that probe can only turn an
# absence back into an unknown while the transport is still refusing. A refusal
# that has healed by then leaves the collar's 1 standing, and the one predicate
# that spends absence as PERMISSION TO TYPE must not rest on it: settled is what
# a caller cashes in by pasting into that composer.
#
# The fixture is a collar from before the refused-read status, which pipes its
# capture into its parser — so a refused capture reaches gang as that parser's
# verdict on empty input, status 1, the same answer it gives for a pane with no
# composer. Its refusal is real rather than simulated: one nominated read is
# aimed at a window that is not there, so tmux itself refuses it, while the raw
# probe gang takes next reads the live pane and succeeds.
cat > "$RUN_ROOT/collars/masking.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_input() {
  local line n i=1 target="\$1"
  while [ "\${FUNCNAME[i]}" = input_read ]; do i=\$((i + 1)); done
  n="\$(cat "$RUN_ROOT/masking-count" 2>/dev/null || printf 0)"
  n=\$((n + 1))
  printf '%s' "\$n" > "$RUN_ROOT/masking-count"
  printf '%s %s\n' "\$n" "\${FUNCNAME[i]}" >> "$RUN_ROOT/masking-reads.log"
  case " \$(cat "$RUN_ROOT/masking-refuse-at" 2>/dev/null) " in
    *" \$n "*) target=@999999 ;;
  esac
  line="\$(tmux capture-pane -pJ -t "\$target" 2>/dev/null |
    awk '{ i = index(\$0, "❯")
           if (i > 0 && (i == 1 || substr(\$0, 1, i - 1) ~ /[^ \t]/)) line = \$0 }
         END { print line }')" || return 1
  case "\$line" in *❯*) ;; *) return 1 ;; esac
  printf '%s' "\${line#*❯}" | tr -d '\302\240'
}
SH
"$HITCH" masking -c masking -d /tmp >/dev/null
masking_id="$(window_id masking)"

# WHICH reads belong to the settled check is discovered, never assumed: the
# fixture records the predicate behind every reading, and the refusals are aimed
# at the looks that check takes. A build that stops reading the box there leaves
# nothing to aim at and fails below rather than passing quietly.
masking_send() { # $1 = the reads to refuse, $2 = the marker to deliver
  : > "$RUN_ROOT/masking-count"
  printf '%s' "$1" > "$RUN_ROOT/masking-refuse-at"
  printf '%s' "$2" | "$GANG" send --to masking --from tester --stdin 2>&1
}

: > "$RUN_ROOT/masking-reads.log"
: > "$RUN_ROOT/masking-count"
rm -f -- "$RUN_ROOT/masking-refuse-at"
printf 'MARK_MASKING_LOCATE' | "$GANG" send --to masking --from tester --stdin >/dev/null
masking_at="$(awk '$2 == "composer_settled" { print $1; exit }' "$RUN_ROOT/masking-reads.log")"
masking_next="$(awk '$2 == "composer_settled" { n++ } n == 2 { print $1; exit }' \
  "$RUN_ROOT/masking-reads.log")"
if [ -n "$masking_at" ] && [ -n "$masking_next" ]; then
  pass "the settled check takes two looks at the box of a collar that cannot report a refusal"
else
  fail "the settled check takes two looks at the box of a collar that cannot report a refusal" \
    "readings served: $(tr '\n' ' ' < "$RUN_ROOT/masking-reads.log")"
fi

# ONE look refused: the pair disagrees, and disagreement is what the first
# version of this guard was built to catch.
masking_one_rc=0
masking_one="$(masking_send "$masking_at" MARK_MASKING_ONE)" || masking_one_rc=$?
if [ "$masking_one_rc" -eq 0 ]; then
  fail "a healed refusal at one settled look does not become permission to type" \
    "the send reported success: [$masking_one]"
else
  pass "a healed refusal at one settled look does not become permission to type"
fi
excludes "so nothing was typed into it" "$(pane_all masking)" "MARK_MASKING_ONE"

# BOTH looks refused, which is the case a status comparison cannot see: a collar
# that masks a refusal answers it with the status it uses for a pane carrying no
# composer, so two refusals arrive as two absences that agree with each other
# perfectly. Settled has to mean two readings that were TAKEN, not two that
# match.
masking_both_rc=0
masking_both="$(masking_send "$masking_at $masking_next" MARK_MASKING_BOTH)" || masking_both_rc=$?
if [ "$masking_both_rc" -eq 0 ]; then
  fail "two healed refusals do not agree their way into permission to type" \
    "the send reported success: [$masking_both]"
else
  pass "two healed refusals do not agree their way into permission to type"
fi
contains "and the refusal names a box gang never got a settled reading of" \
  "$masking_both" "did not both come back with a box it could parse"
excludes "so nothing was typed into it either" "$(pane_all masking)" "MARK_MASKING_BOTH"
equal "the pane it refused about was alive throughout" 0 \
  "$(tmux display-message -p -t "$masking_id" '#{pane_dead}')"
rm -f -- "$RUN_ROOT/masking-refuse-at"
"$GANG" drop masking >/dev/null

# ---------------------------------------------------------------------------
# EVIDENCE OF ACTION, WHICH IS NOT EVIDENCE OF HEALTH. A delivery can be
# verified into a pane and a turn can open and close without the recipient
# running a single command; roster read healthy through three hours of exactly
# that. What gang gained is the age of the last tool call and, where it cannot
# read one, the age of the last write to the pane — two facts and no verdict on
# them, because a long build and a wedge both go quiet.
#
# ANCHORED FAR ENOUGH BACK TO BE STABLE. Every assertion below reads an age gang
# recomputes against its own clock, so each one names a leading unit that a
# second passing between the collar's answer and gang's reading cannot change.
action_now="$(date +%s)"
action_collar() { # $1 = collar name, rest = the body of collar_last_action
  local name="$1"; shift
  cat > "$RUN_ROOT/collars/$name.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_last_action() { $*; }
SH
}
action_collar acted "printf 'at %s' $(( action_now - 7200 )); return 0"
action_collar never "return 1"
action_collar bounded "printf 'before %s' $(( action_now - 7200 )); return 0"
action_collar unreadable "printf 'the rollout this window is bound to was deleted'; return 2"
action_collar ahead "printf 'at %s' $(( action_now + 7200 )); return 0"
action_collar garbled "printf 'sideways 12'; return 0"

"$HITCH" acted -c acted -d /tmp >/dev/null
contains "roster carries the age of the last tool call" \
  "$("$GANG" roster)" "last-tool=2h"
contains "status carries the same age in full" \
  "$("$GANG" status acted)" "last tool call: 2h"
# THE READING IS NOT A HEALTH STATE, and status says so where an operator would
# otherwise infer one from a large number.
contains "status refuses to let the age read as a health verdict" \
  "$("$GANG" status acted)" "not of health"
contains "status reports the pane clock beside it" \
  "$("$GANG" status acted)" "last pane write:"
"$GANG" drop acted >/dev/null

# THE WEDGE ITSELF: a source gang read completely, holding no tool call at all.
"$HITCH" never -c never -d /tmp >/dev/null
contains "an agent that has run nothing is reported as having run nothing" \
  "$("$GANG" roster)" "last-tool=none"
contains "and status says so in words rather than as an age" \
  "$("$GANG" status never)" "has not run a tool"
"$GANG" drop never >/dev/null

# A BOUND IS NOT A COUNT. Past the records the reader walked, the newest tool
# call is older than what it read and gang may not say there was none.
"$HITCH" bounded -c bounded -d /tmp >/dev/null
contains "a bounded answer is reported as a bound" \
  "$("$GANG" roster)" "last-tool>2h"
contains "and status says the reading is a floor" \
  "$("$GANG" status bounded)" "more than 2h"
"$GANG" drop bounded >/dev/null

# UNKNOWN IS NOT NONE. A source gang could not read is the one case where the
# pane clock is the whole of the evidence, so that is where it appears on a row.
"$HITCH" unreadable -c unreadable -d /tmp >/dev/null
action_row="$("$GANG" roster)"
contains "a source gang could not read is an unknown age" "$action_row" "last-tool=?"
contains "and the pane clock stands in as the last evidence left" \
  "$action_row" "last-write="
contains "status names why the reading could not be taken" \
  "$("$GANG" status unreadable)" "the rollout this window is bound to was deleted"
excludes "and an unreadable source is never reported as an idle agent" \
  "$("$GANG" status unreadable)" "has not run a tool"
"$GANG" drop unreadable >/dev/null

# A COLLAR THAT DECLARES NOTHING IS UNKNOWN, not an agent that never acted.
"$HITCH" silent -c bash -d /tmp >/dev/null
contains "a collar with no tool-call source reads unknown" \
  "$("$GANG" roster)" "last-tool=?"
contains "and status names the missing declaration" \
  "$("$GANG" status silent)" "declares no tool-call source"
"$GANG" drop silent >/dev/null

# A CLOCK DISAGREEMENT IS THE FINDING. Spending a future stamp as a tool call a
# moment ago would report the healthiest possible reading from the least
# trustworthy evidence.
"$HITCH" ahead -c ahead -d /tmp >/dev/null
contains "a tool call stamped in the future is unknown, not fresh" \
  "$("$GANG" status ahead)" "in the future"
excludes "and is not reported as an age at all" \
  "$("$GANG" roster)" "last-tool=0s"
"$GANG" drop ahead >/dev/null

# AN ANSWER GANG CANNOT INTERPRET IS LOUD. A collar whose reply names neither
# form is a broken declaration, and consuming it as any verdict would hide it.
"$HITCH" garbled -c garbled -d /tmp >/dev/null
action_garbled_rc=0
action_garbled="$("$GANG" status garbled 2>&1)" || action_garbled_rc=$?
if [ "$action_garbled_rc" -eq 0 ]; then
  fail "an uninterpretable tool-call answer refuses" "status returned 0"
else
  contains "an uninterpretable tool-call answer refuses by name" \
    "$action_garbled" "names neither 'at' nor 'before'"
fi
"$GANG" drop garbled >/dev/null

# ---------------------------------------------------------------------------
# THE WHOLE JOIN, ON A REAL DELIVERY. Every assertion above drives the reading
# through a collar that answers from a literal. This one hitches two agents,
# delivers a verified message to each, and reads the surface afterwards — so a
# reader wired to the wrong window, or a row that never consults the collar at
# all, cannot pass. The two agents differ in exactly one thing: whether their
# own source ever records an action.
#
# NEITHER AGENT IS QUIET BECAUSE IT IS BROKEN. Both accept the delivery and both
# fall silent afterwards, which is the point: the working one must not acquire
# the wedged one's reading merely by going quiet.
mkdir -p "$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/acting.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "\$ROOT/collars/bash.sh"
# Reads the agent's OWN record of what it ran, keyed on the window gang asks
# about, so a reader pointed at the wrong window answers about the wrong agent.
collar_last_action() { # \$1 = tmux target
  local file bare stamp
  bare="\$(tmux show-options -wqv -t "\$1" @gl_agent 2>/dev/null)" || bare=""
  [ -n "\$bare" ] || { printf 'this window registered no agent'; return 2; }
  file="$RUN_ROOT/acts-\$bare"
  [ -f "\$file" ] || { printf 'no action log is bound to this window yet'; return 2; }
  stamp="\$(tail -n 1 "\$file")" || stamp=""
  [ -n "\$stamp" ] || return 1
  printf 'at %s' "\$stamp"
}
SH
action_wedged_log="$RUN_ROOT/acts-wedged"
action_working_log="$RUN_ROOT/acts-working"
: > "$action_wedged_log"
date +%s > "$action_working_log"
"$HITCH" wedged -c acting -d /tmp >/dev/null
"$HITCH" working -c acting -d /tmp >/dev/null

# DELIVERY IS VERIFIED SEPARATELY FROM THE READING, so an agent reported as
# having run nothing is one that was demonstrably spoken to.
action_send_rc=0
printf 'MARK_ACTION_WEDGED reproduce the issue' \
  | "$GANG" send --to wedged --from tester --stdin >"$RUN_ROOT/action-send.out" 2>&1 \
  || action_send_rc=$?
equal "the wedged agent accepted a verified delivery" 0 "$action_send_rc"
contains "and gang reported that delivery verified, not merely typed" \
  "$(<"$RUN_ROOT/action-send.out")" "delivered to wedged"
action_send_rc=0
printf 'MARK_ACTION_WORKING reproduce the issue' \
  | "$GANG" send --to working --from tester --stdin >>"$RUN_ROOT/action-send.out" 2>&1 \
  || action_send_rc=$?
equal "the working agent accepted a verified delivery too" 0 "$action_send_rc"

# THE ROW SAYS WHICH ONE ACTED. Both panes are quiet now; only their own records
# differ.
action_roster="$("$GANG" roster)"
contains "the agent that ran nothing is reported as having run nothing" \
  "$(printf '%s\n' "$action_roster" | grep -- '^wedged ')" "last-tool=none"
excludes "and the one that acted does not inherit that reading" \
  "$(printf '%s\n' "$action_roster" | grep -- '^working ')" "last-tool=none"
contains "the agent that acted carries an age instead" \
  "$(printf '%s\n' "$action_roster" | grep -- '^working ')" "last-tool="
contains "status names the absence of action in words" \
  "$("$GANG" status wedged)" "has not run a tool"
excludes "and says nothing of the kind about the one that acted" \
  "$("$GANG" status working)" "has not run a tool"

# AN EMPTY SOURCE IS NOT A MISSING ONE. Deleting the wedged agent's record makes
# the same quiet pane unknown rather than idle, which is the difference the whole
# reading exists to keep.
rm -f -- "$action_wedged_log"
contains "a source that is gone reads unknown, not inactive" \
  "$("$GANG" status wedged)" "no action log is bound to this window yet"
excludes "and unknown is never spent as an agent that ran nothing" \
  "$("$GANG" status wedged)" "has not run a tool"
"$GANG" drop wedged >/dev/null
"$GANG" drop working >/dev/null

