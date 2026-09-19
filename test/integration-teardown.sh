# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Teardown authority: who may drop whom, what a drop does with the dropped
# agent's children, and an agent withdrawing its own self-compaction.
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
td_hitcher() { # $1 agent -> its recorded hitcher token and name
  printf '%s %s' "$(tmux show-options -wqv -t "$(window_id "$1")" @gl_hitched_by)" \
    "$(tmux show-options -wqv -t "$(window_id "$1")" @gl_hitched_by_name)"
}

"$HITCH" td-lead -c droppable -d /tmp >/dev/null
"$HITCH" td-peer -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-lead)" "$HITCH" td-worker -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-worker)" "$HITCH" td-child -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-peer)" "$HITCH" td-other -c droppable -d /tmp >/dev/null
# Readiness: every provenance stamp this part reasons from is already written.
td_lead_token="$(tmux show-options -wqv -t "$(window_id td-lead)" @gl_spool)"
equal "the fixture tree records its hitchers" "$td_lead_token td-lead" "$(td_hitcher td-worker)"

# A REQUEST WHOSE BOUNDARY KEEPS REFUSING IS WITHDRAWN BY ITS AGENT. Without
# this the only ways out were a retry that could fail again or a hand-edit of
# the window option.
td_worker_id="$(window_id td-worker)"
tmux set-option -w -t "$td_worker_id" @gl_self_compact_requested 0123456789abcdef
tmux set-option -w -t "$td_worker_id" @gl_self_compact_resume TD_WITHDRAWN_RESUME
tmux set-option -w -t "$td_worker_id" @gl_self_compact_failed \
  '[request:0123456789abcdef] another Gangline process is delivering to td-worker (still scheduled; gang retries at the next turn boundary)'
td_as td-peer compact td-worker --cancel
equal "an agent cannot withdraw another agent's self-compaction" 3 "$td_rc"
equal "the refused withdrawal leaves the request standing" 0123456789abcdef \
  "$(tmux show-options -wqv -t "$td_worker_id" @gl_self_compact_requested)"
td_as td-worker compact --cancel --resume TD_BOTH
equal "a withdrawal takes no continuation" 3 "$td_rc"
td_as td-worker compact --cancel --recover
equal "withdrawal and recovery together are refused" 3 "$td_rc"
equal "and refused before the request is withdrawn" 0123456789abcdef \
  "$(tmux show-options -wqv -t "$td_worker_id" @gl_self_compact_requested)"
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
td_as td-worker compact --cancel
equal "a compaction already being dispatched cannot be withdrawn" 3 "$td_rc"
equal "the refused withdrawal leaves the dispatch standing" 0123456789abcdef \
  "$(tmux show-options -wqv -t "$td_worker_id" @gl_self_compact_dispatching)"
tmux set-option -uw -t "$td_worker_id" @gl_self_compact_dispatching

# WHO MAY DROP WHOM. Any agent drops any other agent in its team; a window
# never drops itself, since the teardown would end the caller before it printed
# the relaunch line.
td_as td-worker drop td-worker
equal "an agent cannot drop itself" 3 "$td_rc"
contains "that refusal says why" "$td_out" "does not drop itself"
equal "and the agent survives" 1 "$(window_names | grep -cx td-worker || :)"
td_as td-peer drop td-child
equal "an agent drops a window another live agent hitched" 0 "$td_rc"
equal "and that window is gone" "" "$(window_id td-child || :)"
"$GANG" drop td-other >/dev/null
equal "the operator shell drops an agent another agent hitched" "" "$(window_id td-other || :)"

# A DROP HANDS ITS CHILDREN UP (#289) and never takes them with it. The tree:
#
#   td-gp (operator)
#     td-mid
#       td-live
#         td-late
"$HITCH" td-gp -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-gp)" "$HITCH" td-mid -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-mid)" "$HITCH" td-live -c droppable -d /tmp >/dev/null
TMUX_PANE="$(td_pane td-live)" "$HITCH" td-late -c droppable -d /tmp >/dev/null
td_gp_token="$(tmux show-options -wqv -t "$(window_id td-gp)" @gl_spool)"
td_live_token="$(tmux show-options -wqv -t "$(window_id td-live)" @gl_spool)"
td_as td-peer drop td-mid
equal "an agent drops a window that has children of its own" 0 "$td_rc"
equal "the child survives its hitcher" 1 "$(window_names | grep -cx td-live || :)"
equal "and answers to the dropped hitcher's own hitcher" "$td_gp_token td-gp" "$(td_hitcher td-live)"
contains "the drop says who took it over" "$td_out" "td-live now answers to td-gp"
equal "a grandchild keeps its own live hitcher" "$td_live_token td-live" "$(td_hitcher td-late)"
"$GANG" drop td-gp >/dev/null
equal "a child whose hitcher the operator hitched survives the drop" 1 \
  "$(window_names | grep -cx td-live || :)"
td_as td-peer drop td-live
equal "and an orphan is any teammate's to drop" 0 "$td_rc"
contains "its own child is left an orphan" "$td_out" "td-late is left an orphan"
"$GANG" drop td-late >/dev/null

# PROVENANCE IS A WINDOW OPTION ANY PANE CAN WRITE, so two windows can name
# each other as hitcher. The drop ends, and never hands a child to itself.
"$HITCH" td-x -c droppable -d /tmp >/dev/null
"$HITCH" td-y -c droppable -d /tmp >/dev/null
td_x_token="$(tmux show-options -wqv -t "$(window_id td-x)" @gl_spool)"
td_y_token="$(tmux show-options -wqv -t "$(window_id td-y)" @gl_spool)"
tmux set-option -w -t "$(window_id td-x)" @gl_hitched_by "$td_y_token"
tmux set-option -w -t "$(window_id td-x)" @gl_hitched_by_name td-y
tmux set-option -w -t "$(window_id td-y)" @gl_hitched_by "$td_x_token"
tmux set-option -w -t "$(window_id td-y)" @gl_hitched_by_name td-x
td_rc=0
td_out="$("$GANG" drop td-x 2>&1)" || td_rc=$?
equal "a forged hitch cycle still drops" 0 "$td_rc"
equal "the dropped window is gone" "" "$(window_id td-x || :)"
equal "its partner in the cycle survives" 1 "$(window_names | grep -cx td-y || :)"
equal "and is not handed to itself" "$td_x_token td-x" "$(td_hitcher td-y)"
"$GANG" drop td-y >/dev/null
for td_name in td-lead td-peer td-worker; do "$GANG" drop "$td_name" >/dev/null; done
