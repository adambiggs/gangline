# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# The event stream is a direct durable proof: every required kind is appended,
# filtered back through its reader, and a deliberate omitted kind makes the
# fixture red. No sleeps or eventual-state probes are involved.

events_path="$RUN_ROOT/events.jsonl"
event_kinds=(
  context.read context.band-changed compaction.native-witnessed
  compaction.self-requested compaction.dispatched compaction.self-failed compaction.self-retried
  compaction.self-completed compaction.completed delivery.queued delivery.sending
  delivery.verified delivery.held delivery.interrupted delivery.archived
  state.classified tick.passed tick.held tick.failed tick.deadline-killed alert.raised alert.cleared
  agent.hitched agent.dropped
)

emit_event_fixture() { # $1 path; GANG_TEST_EVENT_MUTANT may omit one exact kind
  local path="$1" kind
  for kind in "${event_kinds[@]}"; do
    [ "${GANG_TEST_EVENT_MUTANT:-}" != "$kind" ] || continue
    "$ROOT/libexec/gang-events" append --events "$path" --team proof \
      --team-created 1 --agent alpha --kind "$kind" --facts '{"fixture":true}' \
      || return 1
  done
  for kind in "${event_kinds[@]}"; do
    [ "${GANG_TEST_EVENT_MUTANT:-}" != "$kind" ] || continue
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
mutant_path="$RUN_ROOT/events-mutant.jsonl"
if GANG_TEST_EVENT_MUTANT=delivery.verified emit_event_fixture "$mutant_path"; then
  fail "event proof mutant is red" "a missing delivery.verified event passed"
else
  pass "event proof mutant is red"
fi

# The helper fixture checks the format and filter in isolation. This separate
# private team drives the production staging, host-side run-shell append, and
# `gang log` reader for every kind; its lifecycle edge remains readable after
# teardown. The deliberate logger mutation removes one production append, so
# the proof is red if an expected event stops reaching the reader.
event_team="gang-events-proof-$$"
event_data="$RUN_ROOT/event-proof-data"
event_gang() { env GANG_SESSION="$event_team" XDG_DATA_HOME="$event_data" "$GANG" "$@"; }
event_hitch_out="$(event_gang hitch event-proof -c bash -d "$RUN_ROOT" 2>&1)" || {
  fail "event proof private team hitches" "$event_hitch_out"
}
event_hitch_log="$(event_gang log event-proof --kind agent.hitched 2>&1)" || {
  fail "event proof reads a live lifecycle event" "$event_hitch_log"
}
# source-guard: whole-surface@8791bccc2376: gang log is the reader under test, and only the production hitch above can append this team-scoped lifecycle row
contains "event proof crosses the host boundary for hitch evidence" \
  "$event_hitch_log" '"kind": "agent.hitched"'
event_proof_out="$(event_gang __event-proof 2>&1)" || {
  fail "event proof emits every production event kind" "$event_proof_out"
}
for event_kind in "${event_kinds[@]}"; do
  case "$event_kind" in
    tick.*|alert.*) event_kind_log="$(event_gang log --kind "$event_kind" 2>&1)" ;;
    *) event_kind_log="$(event_gang log event-proof --kind "$event_kind" 2>&1)" ;;
  esac
  # source-guard: whole-surface@82834deea4df: the production proof driver and its reader filter are the complete evidence for this exact requested kind
  if [[ "$event_kind_log" == *"\"kind\": \"$event_kind\""* ]]; then
    pass "production event proof reads $event_kind through gang log"
  else
    fail "production event proof reads $event_kind through gang log" "$event_kind_log"
  fi
done
event_drop_out="$(event_gang drop event-proof 2>&1)" || {
  fail "event proof private team drops" "$event_drop_out"
}
event_drop_log="$(event_gang log event-proof --kind agent.dropped 2>&1)" || {
  fail "event proof reads teardown evidence" "$event_drop_log"
}
# source-guard: whole-surface@8026fdc628c1: this post-teardown command is the reader under test, and the immediately preceding drop is its only row producer
contains "event log remains readable after team teardown" \
  "$event_drop_log" '"kind": "agent.dropped"'

event_mutant_team="gang-events-mutant-$$"
event_mutant_data="$RUN_ROOT/event-mutant-data"
event_mutant_gang() { env GANG_SESSION="$event_mutant_team" XDG_DATA_HOME="$event_mutant_data" \
  GANG_TEST_EVENT_MUTANT=delivery.verified "$GANG" "$@"; }
event_mutant_hitch="$(event_mutant_gang hitch event-mutant -c bash -d "$RUN_ROOT" 2>&1)" || {
  fail "event proof mutant team hitches" "$event_mutant_hitch"
}
event_mutant_proof="$(event_mutant_gang __event-proof 2>&1)" || {
  fail "event proof mutant emits its remaining kinds" "$event_mutant_proof"
}
event_mutant_log="$(env GANG_SESSION="$event_mutant_team" XDG_DATA_HOME="$event_mutant_data" \
  "$GANG" log event-mutant --kind delivery.verified 2>&1)" || {
  fail "event proof reads its mutant team" "$event_mutant_log"
}
# source-guard: whole-surface@d71f76500ad4: the deliberately omitted proof event and the complete filtered reader output establish that the mutation is observable
if [[ "$event_mutant_log" == *'"kind": "delivery.verified"'* ]]; then
  fail "production event mutation is red" "a mutated delivery instrumentation still reached gang log"
else
  pass "production event mutation is red"
fi
event_mutant_drop="$(event_mutant_gang drop event-mutant 2>&1)" || {
  fail "event proof mutant team drops" "$event_mutant_drop"
}
unset -f event_gang event_mutant_gang
