# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Teardown authority: the safe-to-drop mark, its preconditions, delivery refusal, and who may drop whom.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
#
# Agents are fixture shells; an agent caller is TMUX_PANE naming its pane, the
# way a native harness process inherits it. Every hitch below that runs from an
# agent pane records that agent as the hitcher, so the tree is:
#
#   td-lead (operator)       td-peer (operator)
#     td-worker                td-other
#       td-child
#     td-wedged
#     td-orphan
#     td-orphan2
cat > "$RUN_ROOT/collars/droppable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_SELF_COMPACT=deferred
GANG_COMPACT_CMD='printf ignored'
SH

td_pane() { tmux list-panes -t "$(window_id "$1")" -F '#{pane_id}'; }
td_as() { # $1 agent name, rest = gang arguments; stdout+stderr, status in td_rc
  local pane
  pane="$(td_pane "$1")"
  shift
  td_rc=0
  td_out="$(TMUX_PANE="$pane" "$GANG" "$@" 2>&1)" || td_rc=$?
}
td_send() { # $1 from agent, $2 to agent, $3 body
  local pane
  pane="$(td_pane "$1")"
  td_rc=0
  td_out="$(printf '%s' "$3" | TMUX_PANE="$pane" "$GANG" send --to "$2" --stdin 2>&1)" || td_rc=$?
}
td_mark() { tmux show-options -wqv -t "$(window_id "$1")" @gl_safe_to_drop; }
td_teardown_word() { "$GANG" roster --porcelain | awk -F '\t' -v n="$1" '$1 == n { print $9 }'; }

"$HITCH" td-lead -c droppable -d /tmp >/dev/null
"$HITCH" td-peer -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-lead)" "$HITCH" td-worker -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-lead)" "$HITCH" td-wedged -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-lead)" "$HITCH" td-orphan -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-lead)" "$HITCH" td-orphan2 -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-worker)" "$HITCH" td-child -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-peer)" "$HITCH" td-other -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-lead)" "$HITCH" td-bare -c droppable -d /tmp >/dev/null
# Readiness: every provenance stamp this part reasons from is already written.
equal "the fixture tree records its hitchers" \
  "$(tmux show-options -wqv -t "$(window_id td-lead)" @gl_spool) $(tmux show-options -wqv -t "$(window_id td-worker)" @gl_spool) operator" \
  "$(tmux show-options -wqv -t "$(window_id td-worker)" @gl_hitched_by) $(tmux show-options -wqv -t "$(window_id td-child)" @gl_hitched_by) $(tmux show-options -wqv -t "$(window_id td-lead)" @gl_hitched_by)"

# ONLY A REGISTERED AGENT MARKS ITSELF, AND ONLY AFTER A PROVED DELIVERY.
td_rc=0
td_out="$("$GANG" safe-to-drop --report-to td-lead 2>&1)" || td_rc=$?
equal "an operator shell cannot mark anything safe to drop" 3 "$td_rc"
contains "the operator-shell refusal says why" "$td_out" "only a registered agent marks itself"
td_as td-child safe-to-drop --report-to td-worker
equal "a mark with no delivered report is refused" 3 "$td_rc"
contains "the missing-report refusal names the recipient" "$td_out" \
  "no message from this registration is proved delivered to td-worker"
equal "a refused mark records nothing" "" "$(td_mark td-child)"
td_as td-child safe-to-drop --report-to td-child
equal "a report to yourself is refused" 3 "$td_rc"

# THE FIRST FALSIFIER: A LIVE CHILD. td-worker reports to td-lead, but td-child
# is still live and unmarked.
td_send td-worker td-lead "TD_WORKER_REPORT"
equal "the worker's report is delivered" 0 "$td_rc"
td_as td-worker safe-to-drop --report-to td-lead
equal "a mark over a live unmarked child is refused" 3 "$td_rc"
contains "the live-child refusal names the child" "$td_out" "td-child"
equal "the live-child refusal records nothing" "" "$(td_mark td-worker)"
equal "porcelain names an unmarked agent with a dash" "-" "$(td_teardown_word td-worker)"

# MAIL STILL WAITING IS NOT STRANDED BY A MARK.
td_send td-child td-worker "TD_CHILD_REPORT"
equal "the child's report is delivered" 0 "$td_rc"
td_child_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$(window_id td-child)" @gl_spool)"
printf '%s\n%s\n%s\n' tester TD_WAITING \
  '[gang:tester#abcd1234] TD_WAITING [/gang:tester#abcd1234]' \
  > "$td_child_spool/00000000100000000000-abcd1234"
td_as td-child safe-to-drop --report-to td-worker
equal "a mark with mail still waiting is refused" 3 "$td_rc"
contains "the waiting-mail refusal names the queue" "$td_out" "1 waiting"
equal "the waiting-mail refusal records nothing" "" "$(td_mark td-child)"
td_as td-child mail
contains "the waiting mail is taken by its addressee" "$td_out" "TD_WAITING"
printf 'v2\ttester\t-\tcontrol\t-\tabcd1235\n%s\n%s\n' TD_TIMED \
  '[gang:tester#abcd1235] TD_TIMED [/gang:tester#abcd1235]' \
  > "$td_child_spool/.timed-00000000100000000000-abcd1235"
td_as td-child safe-to-drop --report-to td-worker
equal "a mark with a timed message pending is refused" 3 "$td_rc"
contains "the timed-mail refusal names it" "$td_out" "1 timed"
rm -f -- "$td_child_spool/.timed-00000000100000000000-abcd1235"
printf 'v2\ttester\t-\tcontrol\t-\tabcd1236\n%s\n%s\n' TD_HELD \
  '[gang:tester#abcd1236] TD_HELD [/gang:tester#abcd1236]' \
  > "$td_child_spool/unverified-00000000100000000000-abcd1236"
td_as td-child safe-to-drop --report-to td-worker
equal "a mark over a held, unsettled delivery is refused" 3 "$td_rc"
contains "the held refusal names it" "$td_out" "1 held"
rm -f -- "$td_child_spool/unverified-00000000100000000000-abcd1236"

# THE MARK TAKES THE DELIVERY LOCK. A live holder of td-child's pane lock
# stands for a delivery in progress; the mark must contend, not overtake it.
td_lock="$GANG_LOCK_DIR/$(printf '%s' "$(window_id td-child)" | tr -c 'A-Za-z0-9' '_').lock"
ln -s "$$" "$td_lock"
td_as td-child safe-to-drop --report-to td-worker
rm -f -- "$td_lock"
equal "a mark contending with a delivery in flight is refused" 3 "$td_rc"
contains "the contended mark names the delivery" "$td_out" "another Gangline process is delivering to td-child"
equal "the contended mark records nothing" "" "$(td_mark td-child)"

td_as td-child safe-to-drop --report-to td-worker
equal "a child with a delivered report and an empty spool marks itself" 0 "$td_rc"
contains "the mark says what it means" "$td_out" "td-child marked safe to drop"
equal "the mark is the registration's spool identity" \
  "$(tmux show-options -wqv -t "$(window_id td-child)" @gl_spool)" "$(td_mark td-child)"
equal "porcelain names the marked child" marked "$(td_teardown_word td-child)"
contains "status names the mark and who may act on it" "$("$GANG" status td-child)" \
  "safe to drop: its hitcher or the operator may drop it"
contains "roster names the mark" "$("$GANG" roster)" "safe-to-drop"
td_as td-child safe-to-drop --report-to td-worker
equal "marking again is idempotent" 0 "$td_rc"
contains "marking again says so" "$td_out" "already marked"

# A MARKED REGISTRATION ACCEPTS NO FURTHER DELIVERY, AND SAYS SO LOUDLY.
td_send td-worker td-child "TD_LATE_WORK"
equal "a send to a marked agent is refused" 3 "$td_rc"
contains "the refusal names the mark" "$td_out" "has marked itself safe to drop"
td_rc=0
td_out="$(printf 'TD_LATE_LIVE' | "$GANG" send --to td-child --from tester --live-only --stdin 2>&1)" || td_rc=$?
equal "a live-only send to a marked agent is refused" 3 "$td_rc"
contains "the live-only refusal names the mark" "$td_out" "has marked itself safe to drop"
td_rc=0
td_out="$("$GANG" interrupt td-child -m TD_LATE_REASON --from tester 2>&1)" || td_rc=$?
equal "an interrupt carrying a reason to a marked agent is refused" 3 "$td_rc"
contains "the interrupt refusal names the mark" "$td_out" "has marked itself safe to drop"
td_rc=0
td_out="$(printf 'TD_LATE_TIMED' | "$GANG" at 1h --to td-child --from tester --stdin 2>&1)" || td_rc=$?
if [ "$td_rc" -ne 0 ]; then
  pass "a timed send to a marked agent is refused"
else
  fail "a timed send to a marked agent is refused" "$td_out"
fi
contains "the timed refusal names the mark" "$td_out" "has marked itself safe to drop"
td_rc=0
td_out="$("$GANG" compact td-child 2>&1)" || td_rc=$?
equal "a compaction of a marked agent is refused" 3 "$td_rc"
contains "the compaction refusal names the mark" "$td_out" "has marked itself safe to drop"
td_as td-child compact --resume TD_LATE_RESUME
equal "a marked agent's own compaction is refused" 3 "$td_rc"
equal "the refused self-compaction records no request" "" \
  "$(tmux show-options -wqv -t "$(window_id td-child)" @gl_self_compact_requested)"
td_child_left=0
for td_entry in "$td_child_spool"/* "$td_child_spool"/.[!.]*; do
  [ -e "$td_entry" ] || [ -L "$td_entry" ] || continue
  td_child_left=$((td_child_left + 1))
done
equal "nothing refused was parked for the marked agent" 0 "$td_child_left"
excludes "nothing refused reached the marked pane" "$(pane_all td-child)" "TD_LATE"
contains "the refused body is still the sender's to act on" "$td_out" "the body is still the sender's"

# A RE-ADOPTION KEEPS THE REGISTRATION, SO IT CANNOT RESUME A MARKED AGENT.
td_rc=0
td_out="$("$GANG" adopt td-child -c droppable 2>&1)" || td_rc=$?
equal "re-adopting a marked agent is refused" 3 "$td_rc"
equal "the refused re-adoption keeps the mark" \
  "$(tmux show-options -wqv -t "$(window_id td-child)" @gl_spool)" "$(td_mark td-child)"

# THE CHILD IS CLOSED OUT, SO ITS HITCHER MAY NOW MARK ITSELF — but not over
# a self-compaction state it cannot read.
mkdir -p "$RUN_ROOT/td-norequest"
cat > "$RUN_ROOT/td-norequest/tmux" <<SH
#!/bin/sh
REAL="$(command -v tmux)"
GANG_TEST_PATH_SHIM_GUARD="$GANG_TEST_PATH_SHIM_GUARD"
SH
cat >> "$RUN_ROOT/td-norequest/tmux" <<'SH'
. "$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard "$REAL" "$0" tmux || exit $?
if [ "$1" = show-options ]; then
  for a in "$@"; do
    [ "$a" = @gl_self_compact_requested ] && exit 1
  done
fi
exec "$REAL" "$@"
SH
chmod +x "$RUN_ROOT/td-norequest/tmux"
td_worker_id="$(window_id td-worker)"
tmux set-option -w -t "$td_worker_id" @gl_self_compact_requested 0123456789abcdef
tmux set-option -w -t "$td_worker_id" @gl_self_compact_resume TD_WITHDRAWN_RESUME
tmux set-option -w -t "$td_worker_id" @gl_self_compact_failed \
  '[request:0123456789abcdef] another Gangline process is delivering to td-worker (still scheduled; gang retries at the next turn boundary)'
td_as td-worker safe-to-drop --report-to td-lead
equal "a standing self-compaction request refuses the mark" 3 "$td_rc"
contains "that refusal names the request" "$td_out" "still standing"
# A REQUEST WHOSE BOUNDARY KEEPS REFUSING IS WITHDRAWN BY ITS AGENT. Without
# this the only ways out were a retry that could fail again, a hand-edit of the
# window option, or a drop without the mark.
contains "that refusal says how to withdraw it" "$td_out" "gang compact --cancel"
td_as td-peer compact td-worker --cancel
equal "an agent cannot withdraw another agent's self-compaction" 3 "$td_rc"
equal "the refused withdrawal leaves the request standing" 0123456789abcdef \
  "$(tmux show-options -wqv -t "$td_worker_id" @gl_self_compact_requested)"
td_as td-worker compact --cancel --resume TD_BOTH
equal "a withdrawal takes no continuation" 1 "$td_rc"
td_as td-worker compact --cancel
equal "an agent withdraws its own standing self-compaction" 0 "$td_rc"
contains "the withdrawal says what it withdrew" "$td_out" "withdrawn"
for td_option in @gl_self_compact_requested @gl_self_compact_resume \
    @gl_self_compact_failed @gl_self_compact_witness @gl_self_compact_noted; do
  equal "the withdrawal clears $td_option" "" \
    "$(tmux show-options -wqv -t "$td_worker_id" "$td_option")"
done
td_as td-worker compact --cancel
equal "withdrawing when nothing stands is not an error" 0 "$td_rc"
contains "and says nothing was standing" "$td_out" "no self-compaction"
tmux set-option -w -t "$td_worker_id" @gl_self_compact_dispatching 0123456789abcdef
td_as td-worker safe-to-drop --report-to td-lead
equal "a dispatching self-compaction refuses the mark" 3 "$td_rc"
td_as td-worker compact --cancel
equal "a compaction already being dispatched cannot be withdrawn" 3 "$td_rc"
equal "the refused withdrawal leaves the dispatch standing" 0123456789abcdef \
  "$(tmux show-options -wqv -t "$td_worker_id" @gl_self_compact_dispatching)"
tmux set-option -uw -t "$td_worker_id" @gl_self_compact_dispatching
equal "neither refusal records a mark" "" "$(td_mark td-worker)"
td_rc=0
td_out="$(PATH="$RUN_ROOT/td-norequest:$PATH" TMUX_PANE="$(td_pane td-worker)" \
  "$GANG" safe-to-drop --report-to td-lead 2>&1)" || td_rc=$?
equal "an unreadable self-compaction state refuses the mark" 3 "$td_rc"
contains "that refusal says it cannot be read" "$td_out" "cannot be read"
equal "the unreadable-state refusal records nothing" "" "$(td_mark td-worker)"
td_as td-worker safe-to-drop --report-to td-lead
equal "a worker whose only child is marked marks itself" 0 "$td_rc"

# WHO MAY DROP WHOM.
td_as td-peer drop td-child
equal "an agent cannot drop a marked window another live agent hitched" 3 "$td_rc"
contains "the refusal names the live hitcher" "$td_out" "hitched by live agent 'td-worker'"
td_as td-lead drop td-child
equal "a root agent is not the hitcher of a live agent's child" 3 "$td_rc"
td_as td-child drop td-child
equal "an agent cannot drop itself" 3 "$td_rc"
td_as td-worker drop td-peer
equal "an agent cannot drop an operator-hitched window" 3 "$td_rc"
contains "that refusal names the operator" "$td_out" "the operator hitched 'td-peer'"
if window_id td-child >/dev/null; then
  pass "every refused drop left its target live"
else
  fail "every refused drop left its target live" "td-child is gone"
fi
td_wedged_lock="$GANG_LOCK_DIR/$(printf '%s' "$(window_id td-wedged)" | tr -c 'A-Za-z0-9' '_').lock"
ln -s "$$" "$td_wedged_lock"
td_as td-wedged compact
rm -f -- "$td_wedged_lock"
equal "a self-compaction contends for the lock the mark is set under" 3 "$td_rc"
contains "the contended self-compaction names the delivery" "$td_out" "another Gangline process is delivering to td-wedged"
equal "the contended self-compaction records no request" "" \
  "$(tmux show-options -wqv -t "$(window_id td-wedged)" @gl_self_compact_requested)"
td_lead_token="$(tmux show-options -wqv -t "$(window_id td-lead)" @gl_spool)"
td_as td-peer adopt td-wedged -c droppable
equal "a peer may repair another agent's registration" 0 "$td_rc"
equal "re-adoption keeps the window's hitcher" "$td_lead_token" \
  "$(tmux show-options -wqv -t "$(window_id td-wedged)" @gl_hitched_by)"
td_as td-peer drop td-wedged
equal "re-adopting a window gives the re-adopter no drop authority" 3 "$td_rc"
td_socket="$(tmux display-message -p -t "=$GANG_SESSION" '#{socket_path}')"
td_rc=0
td_out="$(env -u TMUX_PANE TMUX="$td_socket,1,0" "$GANG" drop td-wedged 2>&1)" || td_rc=$?
equal "a caller inside the team's server that names no pane cannot drop" 3 "$td_rc"
contains "that refusal says why" "$td_out" "does not name a pane"
td_rc=0
td_out="$(TMUX="$td_socket,1,0" TMUX_PANE=%999999 "$GANG" drop td-wedged 2>&1)" || td_rc=$?
equal "a caller naming a pane the team's server cannot find cannot drop" 3 "$td_rc"
contains "that refusal names the pane" "$td_out" "which tmux cannot find"
# An agent of another session on the same server is still an agent. Its
# registration is written by hand: this part owns no second team.
tmux new-session -d -s td-elsewhere
td_elsewhere_id="$(tmux list-windows -t "=td-elsewhere" -F '#{window_id}')"
td_elsewhere_pane="$(tmux list-panes -t "$td_elsewhere_id" -F '#{pane_id}')"
tmux set-option -w -t "$td_elsewhere_id" @gl_agent td-elsewhere
tmux set-option -w -t "$td_elsewhere_id" @gl_spool fedcba9876543210
td_rc=0
td_out="$(TMUX="$td_socket,1,0" TMUX_PANE="$td_elsewhere_pane" "$GANG" drop td-wedged 2>&1)" || td_rc=$?
equal "an agent registered in another session cannot drop here" 3 "$td_rc"
contains "that refusal names the outside registration" "$td_out" "not in the readable pane list"
# The environment gang launches an agent with: TMUX removed, TMUX_PANE kept.
td_rc=0
td_out="$(TMUX_PANE="$td_elsewhere_pane" "$GANG" drop td-wedged 2>&1)" || td_rc=$?
equal "an agent of another session launched without TMUX cannot drop here" 3 "$td_rc"
contains "that refusal names the outside registration too" "$td_out" "not in the readable pane list"
tmux set-option -uw -t "$td_elsewhere_id" @gl_agent
tmux set-option -uw -t "$td_elsewhere_id" @gl_spool
# The control: the same pane with no registration is the operator's, and
# drops a window nobody else would be allowed to.
TMUX_PANE="$(td_pane td-lead)" "$HITCH" td-victim -c droppable -d /tmp >/dev/null
td_rc=0
td_out="$(TMUX_PANE="$td_elsewhere_pane" "$GANG" drop td-victim 2>&1)" || td_rc=$?
equal "an unregistered pane of another session drops as the operator" 0 "$td_rc"
tmux kill-session -t "=td-elsewhere"
# PANE IDS ARE SERVER-LOCAL. A caller from another tmux server carries that
# server as GANG_TMUX_SOCKET, the way hitch launches an agent.
td_remote="$RUN_ROOT/td-remote.sock"
tmux -S "$td_remote" new-session -d -s td-remote
td_remote_win="$(tmux -S "$td_remote" list-windows -t "=td-remote" -F '#{window_id}')"
td_remote_pane="$(tmux -S "$td_remote" list-panes -t "$td_remote_win" -F '#{pane_id}')"
tmux -S "$td_remote" set-option -w -t "$td_remote_win" @gl_agent td-remote
tmux -S "$td_remote" set-option -w -t "$td_remote_win" @gl_spool 00112233aabbccdd
td_rc=0
td_out="$(GANG_TMUX_SOCKET="$td_remote" TMUX_PANE="$td_remote_pane" "$GANG" drop td-wedged 2>&1)" || td_rc=$?
equal "an agent of another tmux server cannot drop here" 3 "$td_rc"
contains "that refusal names the other server" "$td_out" "on another tmux server"
td_rc=0
td_out="$(TMUX="$td_remote,1,0" TMUX_PANE="$td_remote_pane" "$GANG" drop td-wedged 2>&1)" || td_rc=$?
equal "a shell attached to another server's agent pane cannot drop here" 3 "$td_rc"
contains "that refusal names the other server too" "$td_out" "on another tmux server"
# A REAL COLLISION: a registered remote pane whose id td-peer's pane also has.
# Read on the team server, that id is td-peer, which may drop td-other.
td_peer_pane="$(td_pane td-peer)"
td_collide_pane="$td_remote_pane"
while [ "${td_collide_pane#%}" -lt "${td_peer_pane#%}" ]; do
  td_collide_pane="$(tmux -S "$td_remote" new-window -d -P -F '#{pane_id}' -t "=td-remote:")"
done
equal "the remote server holds a pane with td-peer's id" "$td_peer_pane" "$td_collide_pane"
td_collide_win="$(tmux -S "$td_remote" display-message -p -t "$td_collide_pane" '#{window_id}')"
tmux -S "$td_remote" set-option -w -t "$td_collide_win" @gl_agent td-collide
tmux -S "$td_remote" set-option -w -t "$td_collide_win" @gl_spool 44556677aabbccdd
td_rc=0
td_out="$(GANG_TMUX_SOCKET="$td_remote" TMUX_PANE="$td_collide_pane" "$GANG" drop td-other 2>&1)" || td_rc=$?
equal "a colliding pane id from another server is not the team window" 3 "$td_rc"
td_rc=0
td_out="$(TMUX="$td_remote,1,0" TMUX_PANE="$td_collide_pane" "$GANG" drop td-other 2>&1)" || td_rc=$?
equal "nor is it from a shell attached to that server" 3 "$td_rc"
tmux -S "$td_remote" set-option -uw -t "$td_remote_win" @gl_agent
tmux -S "$td_remote" set-option -uw -t "$td_remote_win" @gl_spool
TMUX_PANE="$(td_pane td-lead)" "$HITCH" td-victim2 -c droppable -d /tmp >/dev/null
td_rc=0
td_out="$(GANG_TMUX_SOCKET="$td_remote" TMUX_PANE="$td_remote_pane" "$GANG" drop td-victim2 2>&1)" || td_rc=$?
equal "an unregistered pane of another server drops as the operator" 0 "$td_rc"
tmux -S "$td_remote" kill-server
# A target-session pane scan that cannot be read does not make a registered
# agent the operator.
mkdir -p "$RUN_ROOT/td-noscan"
cat > "$RUN_ROOT/td-noscan/tmux" <<SH
#!/bin/sh
REAL="$(command -v tmux)"
GANG_TEST_PATH_SHIM_GUARD="$GANG_TEST_PATH_SHIM_GUARD"
SH
cat >> "$RUN_ROOT/td-noscan/tmux" <<'SH'
. "$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard "$REAL" "$0" tmux || exit $?
[ "$1" = list-panes ] && [ "$2" = -s ] && exit 1
exec "$REAL" "$@"
SH
chmod +x "$RUN_ROOT/td-noscan/tmux"
td_rc=0
td_out="$(PATH="$RUN_ROOT/td-noscan:$PATH" TMUX_PANE="$(td_pane td-peer)" \
  "$GANG" drop td-wedged 2>&1)" || td_rc=$?
equal "an unreadable team pane scan does not make an agent the operator" 3 "$td_rc"
td_bare_id="$(window_id td-bare)"
tmux set-option -uw -t "$td_bare_id" @gl_hitched_by
tmux set-option -uw -t "$td_bare_id" @gl_hitched_by_name
td_as td-peer adopt td-bare -c droppable
equal "a peer may repair a window with no recorded hitcher" 0 "$td_rc"
equal "that repair records no hitcher" "" \
  "$(tmux show-options -wqv -t "$td_bare_id" @gl_hitched_by)"
td_as td-peer drop td-bare
equal "repairing a window without provenance grants no drop authority" 3 "$td_rc"
contains "that refusal names the missing provenance" "$td_out" "records no hitch provenance"
td_as td-lead drop td-wedged
equal "a hitcher drops its own unmarked (wedged) child" 0 "$td_rc"
td_as td-worker drop td-child
equal "a hitcher drops its marked child" 0 "$td_rc"

# THE THIRD FALSIFIER: A WITNESSED-GONE HITCHER. td-orphan loses td-lead only
# when the operator drops td-lead; td-orphan's hitcher is then a name alone.
td_send td-orphan td-peer "TD_ORPHAN_REPORT"
equal "the orphan-to-be delivers its report to a live peer" 0 "$td_rc"
"$GANG" drop td-worker >/dev/null
"$GANG" drop td-lead >/dev/null
equal "the orphan's hitcher is witnessed, not live" "td-lead" \
  "$(tmux show-options -wqv -t "$(window_id td-orphan)" @gl_hitched_by_name)"
td_as td-peer drop td-orphan
equal "a root agent cannot drop an unmarked orphan" 3 "$td_rc"
# The refusal used to say only the operator may, which --orphan below makes
# false for a root agent; the operator is still named first.
contains "the unmarked-orphan refusal names the operator and the flag" "$td_out" \
  "only the operator, or a root agent passing --orphan, may drop an unmarked orphan"
td_as td-other drop td-orphan2 --orphan
equal "a non-root agent cannot take an unmarked orphan with --orphan" 3 "$td_rc"
contains "and the refusal names who may" "$td_out" "only a root agent"
td_as td-peer drop td-other --orphan
equal "--orphan refuses a drop it does not authorize" 3 "$td_rc"
contains "and says to retry without it" "$td_out" "Retry without --orphan"
equal "so a hitcher's own child survives a misused flag" 1 \
  "$(window_names | grep -cx td-other || :)"
td_as td-peer drop td-orphan2 --orphan
equal "a root agent takes an unmarked orphan with --orphan" 0 "$td_rc"
equal "and the orphan's window is gone" "" "$(window_id td-orphan2 || :)"
td_as td-orphan safe-to-drop --report-to td-peer
equal "an orphan with a delivered report marks itself" 0 "$td_rc"
equal "porcelain names a marked orphan" marked-orphan "$(td_teardown_word td-orphan)"
td_as td-other drop td-orphan
equal "a non-root agent cannot drop a marked orphan" 3 "$td_rc"
contains "the non-root refusal names who may" "$td_out" "only a root agent"

# THE SECOND FALSIFIER: A MARK SURVIVING ITS REGISTRATION. A mark whose token
# is not the window's current spool identity is not honoured anywhere.
td_orphan_id="$(window_id td-orphan)"
td_real_mark="$(td_mark td-orphan)"
tmux set-option -w -t "$td_orphan_id" @gl_safe_to_drop 0123456789abcdef
equal "porcelain names a mark from another registration stale" stale "$(td_teardown_word td-orphan)"
td_as td-peer drop td-orphan
equal "a stale mark does not authorize a root agent's drop" 3 "$td_rc"
td_send td-peer td-orphan "TD_STALE_DELIVERY"
equal "a stale mark does not refuse delivery to the current registration" 0 "$td_rc"
tmux set-option -w -t "$td_orphan_id" @gl_safe_to_drop "$td_real_mark"
mkdir -p "$RUN_ROOT/td-noregistry"
cat > "$RUN_ROOT/td-noregistry/tmux" <<SH
#!/bin/sh
REAL="$(command -v tmux)"
GANG_TEST_PATH_SHIM_GUARD="$GANG_TEST_PATH_SHIM_GUARD"
SH
cat >> "$RUN_ROOT/td-noregistry/tmux" <<'SH'
. "$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard "$REAL" "$0" tmux || exit $?
# The whole-server register, and nothing else, cannot be read.
[ "$1" = list-windows ] && [ "$2" = -a ] && exit 1
exec "$REAL" "$@"
SH
chmod +x "$RUN_ROOT/td-noregistry/tmux"
td_rc=0
td_out="$(PATH="$RUN_ROOT/td-noregistry:$PATH" TMUX_PANE="$(td_pane td-peer)" \
  "$GANG" drop td-orphan 2>&1)" || td_rc=$?
equal "an unreadable register does not prove a hitcher gone" 3 "$td_rc"
contains "that refusal names the unchecked register" "$td_out" "cannot be checked against the live window register"
td_unread_row="$(PATH="$RUN_ROOT/td-noregistry:$PATH" "$GANG" roster --porcelain | awk -F '\t' '$1 == "td-orphan"')"
# The row has to be an ordinary one, or unknown would be the row's fallback.
equal "under an unreadable register the orphan's hitcher state reads unknown" unknown \
  "$(printf '%s' "$td_unread_row" | cut -f7)"
td_unread_state="$(printf '%s' "$td_unread_row" | cut -f3)"
case "$td_unread_state" in
  ''|unknown) fail "and its state column is still a reading" "state was [${td_unread_state}]" ;;
  *) pass "and its state column is still a reading" ;;
esac
equal "and a mark whose hitcher cannot be checked reads unknown" unknown \
  "$(printf '%s' "$td_unread_row" | cut -f9)"
if window_id td-orphan >/dev/null; then
  pass "the unproved orphan is still live"
else
  fail "the unproved orphan is still live" "td-orphan is gone"
fi
td_as td-peer drop td-orphan
equal "a root agent drops a marked orphan by name" 0 "$td_rc"

# THE OPERATOR SHELL DROPS ANYTHING, AS IT ALWAYS HAS.
td_rc=0
td_out="$("$GANG" drop td-other 2>&1)" || td_rc=$?
equal "the operator drops an unmarked agent another agent hitched" 0 "$td_rc"
# A MARKED ROOT: its hitcher is the operator, and that is an ordinary mark.
"$HITCH" td-root -c droppable -d /tmp >/dev/null
td_send td-root td-bare "TD_ROOT_REPORT"
equal "the root agent's report is delivered" 0 "$td_rc"
td_as td-root safe-to-drop --report-to td-bare
equal "a root agent marks itself" 0 "$td_rc"
equal "porcelain names an operator-hitched mark as marked" marked "$(td_teardown_word td-root)"
contains "status says who may drop it" "$("$GANG" status td-root)" \
  "safe to drop: its hitcher or the operator may drop it"
"$GANG" drop td-root >/dev/null
"$GANG" drop td-bare >/dev/null
"$GANG" drop td-peer >/dev/null
