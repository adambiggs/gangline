# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# The event stream is a direct durable proof: every required kind is appended
# and filtered back through its reader. No sleeps or eventual-state probes are
# involved.

events_path="$RUN_ROOT/events.jsonl"
event_kinds=(
  context.read context.band-changed compaction.native-witnessed
  compaction.self-requested compaction.dispatched compaction.self-failed
  compaction.self-completed compaction.completed delivery.queued delivery.sending
  delivery.verified delivery.held delivery.interrupted delivery.archived
  state.classified tick.passed tick.failed tick.deadline-killed
  agent.hitched agent.dropped
)

emit_event_fixture() { # $1 path
  local path="$1" kind
  for kind in "${event_kinds[@]}"; do
    "$ROOT/libexec/gang-events" append --events "$path" --team proof \
      --team-created 1 --agent alpha --kind "$kind" --facts '{"fixture":true}' \
      || return 1
  done
  for kind in "${event_kinds[@]}"; do
    "$ROOT/libexec/gang-events" read --events "$path" --team proof --agent alpha \
      --kind "$kind" | grep -F '"kind": "' >/dev/null || return 1
  done
}

if emit_event_fixture "$events_path"; then
  pass "event proof emits and reads every required kind"
else
  fail "event proof emits and reads every required kind" "one emitted kind was not readable"
fi
event_rows="$($ROOT/libexec/gang-events read --events "$events_path" --team proof --agent alpha | wc -l | tr -d ' ')"
equal "event proof retains exactly one line per required kind" "${#event_kinds[@]}" "$event_rows"
event_since="$($ROOT/libexec/gang-events read --events "$events_path" --team proof --kind delivery.verified | sed -n '1p')"
contains "event reader filters by kind" "$event_since" 'delivery.verified'
# A separate team checks lifecycle appends through the production command and
# keeps them readable after teardown.
event_team="gang-events-proof-$$"
event_data="$RUN_ROOT/event-proof-data"
event_gang() { env GANG_SESSION="$event_team" XDG_DATA_HOME="$event_data" "$GANG" "$@"; }
# A harness PATH can accumulate package and plugin directories until tmux
# refuses a run-shell command that embeds it. Event delivery still crosses
# run-shell with a PATH of its own; inflate the caller PATH past that boundary
# while requiring every row to be recorded normally.
event_long_path="$PATH"
for ((event_path_copy = 0; event_path_copy < 40; event_path_copy++)); do
  event_long_path="$RUN_ROOT/event-path-component:$event_long_path"
done
event_hitch_out="$(PATH="$event_long_path" event_gang hitch event-proof -c bash -d "$RUN_ROOT" 2>&1)" || {
  fail "event proof private team hitches" "$event_hitch_out"
}
excludes "a long caller PATH does not overflow the event worker command" \
  "$event_hitch_out" "command too long"
event_hitch_log="$(event_gang log event-proof --kind agent.hitched 2>&1)" || {
  fail "event proof reads a live lifecycle event" "$event_hitch_log"
}
contains "event proof crosses the host boundary for hitch evidence" \
  "$event_hitch_log" '"kind": "agent.hitched"'
event_drop_out="$(PATH="$event_long_path" event_gang drop event-proof 2>&1)" || {
  fail "event proof private team drops" "$event_drop_out"
}
excludes "the long ambient PATH does not overflow the usage worker command" \
  "$event_drop_out" "usage event not written: tmux could not run"
event_drop_log="$(event_gang log event-proof --kind agent.dropped 2>&1)" || {
  fail "event proof reads teardown evidence" "$event_drop_log"
}
contains "event log remains readable after team teardown" \
  "$event_drop_log" '"kind": "agent.dropped"'

unset -f event_gang
