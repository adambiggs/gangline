# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# The event stream is a direct durable proof: every required kind is appended,
# filtered back through its reader, and a deliberate omitted kind makes the
# fixture red. No sleeps or eventual-state probes are involved.

events_path="$RUN_ROOT/events.jsonl"
event_kinds=(
  context.read context.band-changed compaction.native-witnessed
  compaction.self-requested compaction.dispatched compaction.self-failed
  compaction.self-completed compaction.completed delivery.queued delivery.sending
  delivery.verified delivery.held delivery.interrupted delivery.archived
  state.classified tick.passed tick.failed alert.raised alert.cleared
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
