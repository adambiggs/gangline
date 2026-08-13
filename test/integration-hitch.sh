# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Hitch: boot dialogs, native event shapes gang cannot interpret, refusal contracts, the team curfew, addressing, and model and effort selection.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# Bind the shipped Claude declaration to the matcher that consumes it. The
# fixture renders the stable suffix captured from Claude; the collar under
# test contributes its own marker and block, rather than restating either.
dialog_start dialog-claude-external external-import dialog-claude-external
claude_external_before="$(pane dialog-claude-external)"
contains "the shipped Claude record recognizes its rendered dialog" \
  "$("$GANG" status dialog-claude-external)" \
  "known operator dialog: external-import-trust"
if claude_external_send="$(printf SHIPPED_EXTERNAL_BODY | "$GANG" send \
    --to dialog-claude-external --from tester --live-only --stdin 2>&1)"; then
  fail "the shipped Claude operator dialog refuses delivery" \
    "send unexpectedly succeeded"
else
  contains "the shipped refusal names its exact dialog" \
    "$claude_external_send" "known operator dialog 'external-import-trust'"
fi
equal "the shipped Claude record sends no key" "" \
  "$(<"$RUN_ROOT/dialog-claude-external.keys")"
# source-guard: whole-surface@c171fc82a7df: an observe-only shipped record is forbidden to change any visible pane byte, whatever produced it
equal "the shipped Claude operator dialog remains byte-exact" \
  "$claude_external_before" "$(pane dialog-claude-external)"
"$GANG" drop dialog-claude-external >/dev/null

# Hitch names an observe-only prompt while preserving the original boot budget.
# The sleep shim is an event barrier: the notice is already written when it
# signals, then the fixture receives only the two manual operator keys below.
: > "$RUN_ROOT/dialog-observe-boot.keys"
PATH="$RUN_ROOT/dialog-observe-bin:$PATH" GANG_BOOT_TIMEOUT=3 \
  "$GANG" hitch dialog-observe-boot -c dialog-observe-boot -d /tmp \
  >"$RUN_ROOT/dialog-observe-boot.out" 2>&1 &
observe_boot_pid=$!
# shellcheck disable=SC2154  # set in test/integration-substrate.sh
tmux wait-for "$observe_boot_seen"
equal "an observe-only branch waits before testing generic readiness" \
  "1" "$(<"$RUN_ROOT/dialog-observe-sleep-argument")"
contains "hitch names the operator dialog it is waiting on" \
  "$(<"$RUN_ROOT/dialog-observe-boot.out")" \
  "known operator dialog 'operator-choice'"
equal "hitch sends no key to the operator dialog" "" \
  "$(<"$RUN_ROOT/dialog-observe-boot.keys")"
observe_boot_id="$(window_id dialog-observe-boot)"
tmux send-keys -t "$observe_boot_id" Down Enter
# shellcheck disable=SC2154  # set in test/integration-substrate.sh
tmux wait-for -S "$observe_boot_release"
if wait "$observe_boot_pid"; then
  pass "hitch continues after the operator answers the recognized dialog"
else
  fail "hitch continues after the operator answers the recognized dialog" \
    "$(<"$RUN_ROOT/dialog-observe-boot.out")"
fi
equal "only the operator's manual answer reaches the recognized dialog" \
  $'Down\nEnter' "$(<"$RUN_ROOT/dialog-observe-boot.keys")"
# source-guard: producer@b9a4f7b286cf: the successful hitch above is the sole producer of this nonce-addressed startup body after the fixture restores its composer
contains "the post-dialog startup contract is delivered" \
  "$(pane dialog-observe-boot)" "You are dialog-observe-boot in Gangline"
"$GANG" drop dialog-observe-boot >/dev/null

cat > "$RUN_ROOT/collars/dialog-ambiguous.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter
same-bytes|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter'
GANG_DIALOG_LINES_same_bytes="\$GANG_DIALOG_LINES_safety_buffering_prompt"
SH
dialog_start dialog-ambiguous known dialog-ambiguous
if ambiguous_out="$("$GANG" status dialog-ambiguous 2>&1)"; then
  fail "two matching dialog records are refused as ambiguity" \
    "status unexpectedly succeeded"
else
  contains "ambiguity names the first matching id" \
    "$ambiguous_out" "safety-buffering-prompt"
  contains "ambiguity names the second matching id" "$ambiguous_out" "same-bytes"
fi
equal "an ambiguous registry presses no key" "" \
  "$(<"$RUN_ROOT/dialog-ambiguous.keys")"
"$GANG" drop dialog-ambiguous >/dev/null

cat > "$RUN_ROOT/collars/dialog-danger.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='permission-surface|^› [0-9]+\. |Allow||Enter'
GANG_DIALOG_LINES_permission_surface='Do you want to allow this tool to run?
Allow
Deny'
SH
refuses "load_collar mechanically rejects an authority-shaped dialog registry" \
  "forbidden authority word" "$GANG" hitch dialog-danger -c dialog-danger -d /tmp
equal "a refused authority registry opens no window" "" "$(window_id dialog-danger)"

cat > "$RUN_ROOT/collars/dialog-trust-unscoped.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='directory-trust-prompt|^› [0-9]+\. |Yes, continue||Enter'
GANG_DIALOG_LINES_directory_trust_prompt='Do you trust this directory?
Yes, continue
No, quit'
SH
refuses "directory trust remains forbidden without the hitch-directory declaration" \
  "forbidden authority word" \
  "$GANG" hitch dialog-trust-unscoped -c dialog-trust-unscoped -d /tmp
equal "an unscoped trust registry opens no window" "" \
  "$(window_id dialog-trust-unscoped)"

authority_probe_cases='approval|Approval required to continue
access|Grant access to your files
authorize|Authorize this device
admin|Grant admin rights to the agent
denial|Denial of safeguards is disabled
ask-again|Yes, and do not ask again
bypass|Bypass all safety checks
write-access|Enable write access to disk'
while IFS='|' read -r authority_slug authority_line; do
  [ -n "$authority_slug" ] || continue
  cat > "$RUN_ROOT/collars/dialog-authority-$authority_slug.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='authority-$authority_slug|^› [0-9]+\. |No||Enter'
GANG_DIALOG_LINES_authority_${authority_slug//-/_}='$authority_line
Yes
No'
SH
  refuses "authority registry rejects $authority_slug language" \
    "forbidden authority word" \
    "$GANG" hitch "authority-$authority_slug" \
      -c "dialog-authority-$authority_slug" -d /tmp
  equal "the $authority_slug authority refusal opens no window" "" \
    "$(window_id "authority-$authority_slug")"
done <<<"$authority_probe_cases"

cat > "$RUN_ROOT/collars/dialog-four-fields.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='broken|^› [0-9]+\. |Safe|Enter'
SH
cat > "$RUN_ROOT/collars/dialog-bad-id.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='Bad_ID|^› [0-9]+\. |Safe||Enter'
GANG_DIALOG_LINES_Bad_ID='Safe'
SH
cat > "$RUN_ROOT/collars/dialog-safe-absent.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='safe-absent|^› [0-9]+\. |Missing||Enter'
GANG_DIALOG_LINES_safe_absent='Present'
SH
cat > "$RUN_ROOT/collars/dialog-block-missing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='block-missing|^› [0-9]+\. |Safe||Enter'
SH
refuses "a four-field dialog record is refused at collar load" \
  "without exactly five fields" \
  "$GANG" hitch dialog-four-fields -c dialog-four-fields -d /tmp
refuses "a dialog id outside the slug alphabet is refused at collar load" \
  "invalid dialog id" "$GANG" hitch dialog-bad-id -c dialog-bad-id -d /tmp
refuses "a safe label absent from its declared block is refused at collar load" \
  "is not one of" \
  "$GANG" hitch dialog-safe-absent -c dialog-safe-absent -d /tmp
refuses "an unset dialog block is refused at collar load" \
  "has no non-empty GANG_DIALOG_LINES_block_missing" \
  "$GANG" hitch dialog-block-missing -c dialog-block-missing -d /tmp

: > "$RUN_ROOT/dialog-recurring.keys"
mkdir -p "$RUN_ROOT/recurring-bin"
cat > "$RUN_ROOT/recurring-bin/sleep" <<'SH'
#!/bin/sh
[ "${1:-}" != 1 ] || tmux wait-for -S "$DIALOG_RECUR_SIGNAL"
exit 0
SH
chmod +x "$RUN_ROOT/recurring-bin/sleep"
# shellcheck disable=SC2154  # set in test/integration-substrate.sh
if recurring_out="$(DIALOG_RECUR_SIGNAL="$dialog_recurring_signal" \
    PATH="$RUN_ROOT/recurring-bin:$PATH" GANG_BOOT_TIMEOUT=2 \
    "$GANG" hitch dialog-recurring \
    -c dialog-recurring -d /tmp 2>&1)"; then
  fail "a recurring known transient consumes the hitch boot budget" \
    "hitch unexpectedly answered every recurrence and succeeded"
else
  contains "recurring known-transient exhaustion fails loud" \
    "$recurring_out" "startup message was not delivered"
fi
case "$(<"$RUN_ROOT/dialog-recurring.keys")" in
  $'Down\nEnter'|$'Down\nEnter\nDown\nEnter')
    pass "recurring dialogs cannot answer beyond the hitch boot budget" ;;
  *) fail "recurring dialogs cannot answer beyond the hitch boot budget" \
       "unexpected key sequence [$(<"$RUN_ROOT/dialog-recurring.keys")]" ;;
esac
"$GANG" drop dialog-recurring >/dev/null

# Calibrate the boot-dialog instrument first: one fingerprint byte is wrong,
# so the same trust-shaped screen must receive no key and reproduce the settled
# non-composer hitch failure. The successful twin then exercises the shipped
# empty-move shape and startup delivery, not merely dialog recognition.
cat > "$RUN_ROOT/collars/dialog-trust-boot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="env DIALOG_VARIANT=trust DIALOG_KEY_LOG='$RUN_ROOT/dialog-trust-boot.keys' DIALOG_READY='dialog-trust-boot-ready-$$' '$RUN_ROOT/dialog-fixture.py'"
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
GANG_DIALOGS='directory-trust-prompt|^› [0-9]+\. |Yes, continue||Enter'
GANG_DIALOG_HITCH_DIR_TRUST=directory-trust-prompt
GANG_DIALOG_LINES_directory_trust_prompt='Do you trust the contents of this directory? Working with untrusted contents comes with higher risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec policies to load.
Yes, continue
No, quit
Press enter to continue'
SH
cat > "$RUN_ROOT/collars/dialog-trust-boot-miss.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-trust-boot.sh"
GANG_LAUNCH="env DIALOG_VARIANT=trust DIALOG_KEY_LOG='$RUN_ROOT/dialog-trust-boot-miss.keys' DIALOG_READY='dialog-trust-boot-miss-ready-$$' '$RUN_ROOT/dialog-fixture.py'"
GANG_DIALOG_LINES_directory_trust_prompt='Do you trust the content of this directory? Working with untrusted contents comes with higher risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec policies to load.
Yes, continue
No, quit
Press enter to continue'
SH
: > "$RUN_ROOT/dialog-trust-boot-miss.keys"
if trust_miss_out="$(GANG_BOOT_TIMEOUT=1 "$GANG" hitch trust-boot-miss \
    -c dialog-trust-boot-miss -d /tmp 2>&1)"; then
  fail "a mutated trust fingerprint calibrates the boot-dialog guard red" \
    "hitch unexpectedly succeeded"
else
  pass "a mutated trust fingerprint calibrates the boot-dialog guard red"
fi
equal "the missed boot dialog receives no auto-answer key" "" \
  "$(<"$RUN_ROOT/dialog-trust-boot-miss.keys")"
contains "unknown-dialog recovery delivers the contract after manual clearance" \
  "$trust_miss_out" "gang send --to trust-boot-miss"
excludes "unknown-dialog recovery does not prescribe an unconditional drop" \
  "$trust_miss_out" "then 'gang drop"
"$GANG" drop trust-boot-miss >/dev/null

: > "$RUN_ROOT/dialog-trust-boot.keys"
if GANG_BOOT_TIMEOUT=3 "$GANG" hitch trust-boot -c dialog-trust-boot -d /tmp \
    >"$RUN_ROOT/dialog-trust-boot.out" 2>&1; then
  pass "hitch answers the known directory-trust dialog and continues"
else
  fail "hitch answers the known directory-trust dialog and continues" \
    "$(<"$RUN_ROOT/dialog-trust-boot.out")"
fi
equal "the preselected trust row needs only its empty-move confirmation" \
  "Enter" "$(<"$RUN_ROOT/dialog-trust-boot.keys")"
contains "the post-dialog hitch delivers its startup contract" \
  "$(pane trust-boot)" "You are trust-boot in Gangline"
contains "the post-dialog hitch reports verified startup delivery" \
  "$(<"$RUN_ROOT/dialog-trust-boot.out")" \
  "delivered startup contract to trust-boot"
submitted "the post-dialog startup contract was submitted" trust-boot
"$GANG" drop trust-boot >/dev/null

modal_observed="test-boot-modal-observed-$$"
modal_painted="test-boot-modal-painted-$$"
modal_clear="test-boot-modal-clear-$$"
modal_probe_release="test-boot-modal-probe-release-$$"
cat > "$RUN_ROOT/collars/boot-modal.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'printf \"FIRST_RUN_MODAL\\n\"; tmux wait-for -S \"$modal_painted\"; tmux wait-for \"$modal_clear\"; PS1=\"❯ \" exec bash --norc' fixture"
GANG_OCCUPIED_REGEX='FIRST_RUN_MODAL'
_gl_modal_real="\$(declare -f collar_input)"
eval "modal_real_input \${_gl_modal_real#collar_input}"
collar_input() {
  if [ -e "$RUN_ROOT/boot-modal-blocked" ]; then
    if [ -e "$RUN_ROOT/boot-modal-seen" ]; then
      tmux wait-for -S "$modal_observed"
      tmux wait-for "$modal_probe_release"
    else
      tmux wait-for "$modal_painted"
      : > "$RUN_ROOT/boot-modal-seen"
    fi
    return 1
  fi
  modal_real_input "\$1"
}
SH
touch "$RUN_ROOT/boot-modal-blocked"
modal_output="$RUN_ROOT/boot-modal.out"
GANG_BOOT_TIMEOUT=5 "$GANG" hitch boot-modal -c boot-modal -d /tmp \
  >"$modal_output" 2>&1 &
modal_hitch_pid=$!
tmux wait-for "$modal_observed"
contains "hitch reports a first-run modal before it is cleared" \
  "$(<"$modal_output")" "answer it with 'gang attach'"
rm -f -- "$RUN_ROOT/boot-modal-blocked"
tmux wait-for -S "$modal_clear"
tmux wait-for -S "$modal_probe_release"
if wait "$modal_hitch_pid"; then
  pass "the same hitch completes after the operator clears the modal"
else
  fail "the same hitch completes after the operator clears the modal" \
    "$(<"$modal_output")"
fi
contains "the resumed hitch delivers its startup contract" \
  "$(pane boot-modal)" "You are boot-modal in Gangline"
contains "the resumed hitch reports verified startup delivery" \
  "$(<"$modal_output")" "delivered startup contract to boot-modal"
contains "the resumed hitch retracts the first-run warning" \
  "$(<"$modal_output")" "now has an input box; hitch is continuing"
submitted "the resumed startup contract was submitted" boot-modal
"$GANG" drop boot-modal >/dev/null

late_observed="test-late-composer-observed-$$"
late_launch="test-late-composer-launch-$$"
late_probe_release="test-late-composer-probe-release-$$"
cat > "$RUN_ROOT/collars/late-composer.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'tmux wait-for \"$late_launch\"; PS1=\"❯ \" exec bash --norc' fixture"
_gl_late_real="\$(declare -f collar_input)"
eval "late_real_input \${_gl_late_real#collar_input}"
collar_input() {
  if [ -e "$RUN_ROOT/late-composer-blocked" ]; then
    if [ -e "$RUN_ROOT/late-composer-seen" ]; then
      tmux wait-for -S "$late_observed"
      tmux wait-for "$late_probe_release"
    else
      : > "$RUN_ROOT/late-composer-seen"
    fi
    return 1
  fi
  late_real_input "\$1"
}
SH
touch "$RUN_ROOT/late-composer-blocked"
late_output="$RUN_ROOT/late-composer.out"
GANG_BOOT_TIMEOUT=5 "$GANG" hitch late-composer -c late-composer -d /tmp \
  >"$late_output" 2>&1 &
late_hitch_pid=$!
tmux wait-for "$late_observed"
excludes "a blank slow boot is not reported as a first-run modal" \
  "$(<"$late_output")" "answer it with 'gang attach'"
rm -f -- "$RUN_ROOT/late-composer-blocked"
tmux wait-for -S "$late_launch"
tmux wait-for -S "$late_probe_release"
if wait "$late_hitch_pid"; then
  pass "a delayed composer completes its original hitch"
else
  fail "a delayed composer completes its original hitch" "$(<"$late_output")"
fi
excludes "a completed delayed composer leaves no first-run warning" \
  "$(<"$late_output")" "answer it with 'gang attach'"
"$GANG" drop late-composer >/dev/null

cat > "$RUN_ROOT/collars/broken-observer.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_BUSY_REGEX='['
SH
"$HITCH" broken-observer -c broken-observer -d /tmp >/dev/null
if broken_roster="$("$GANG" roster 2>&1)"; then
  fail "roster fails when an agent row cannot be observed" "roster exited successfully"
else
  broken_roster_rc=$?
  equal "roster propagates the observation failure" "1" "$broken_roster_rc"
fi
contains "roster names the collar whose observation failed" \
  "$broken_roster" "broken-observer.sh"
"$GANG" drop broken-observer >/dev/null
contains "startup is one useful contract, not a bookkeeping turn" \
  "$(pane alpha)" "You are alpha in Gangline"
contains "startup ends instead of polling for work" \
  "$(pane alpha)" "End this turn."
# The operative prose is pointed at rather than pasted, so the pane is held to
# naming the file and the sentences are proven where they now live. What each
# line requires is unchanged; only the surface carrying it moved. Newlines fold
# to spaces because the file wraps its sentences and a composer never did.
contract_prose() { tr '\n' ' ' < "$ROOT/CONTRACT.md" | tr -s ' '; }
contains "startup names the contract file it points at" \
  "$(pane alpha)" "CONTRACT.md"
contains "startup orders that contract read before anything else" \
  "$(pane alpha)" "before anything else"
contains "startup gives an unreadable contract a loud stop rather than a guess" \
  "$(pane alpha)" "say so and stop rather than improvising the contract"
contains "the contract says every agent belongs to an addressable team" \
  "$(contract_prose)" \
  "You are one agent on a Gangline team. Run \`gang send --to NAME --stdin\` to address any teammate by name."
contains "the contract makes teammate reachability the shared-state test" \
  "$(contract_prose)" \
  "Put unfinished work and supporting detail in files that teammates can read without you."
contains "the contract lets a result owner hitch help" \
  "$(contract_prose)" "You may hitch teammates to help."
contains "the contract requires one completed-result report" \
  "$(contract_prose)" "Send the lead one report when the result is complete."
excludes "the startup contract no longer spends a line on compaction" \
  "$(pane alpha)" "compact with"
contains "the contract carries the operator-authorized marathon rule" \
  "$(contract_prose)" \
  "When a decision is irreversible or doctrine does not cover it, record the question for the operator. Stop only the affected work and continue everything else."
contains "the contract states the complement of envelope attribution" \
  "$(contract_prose)" \
  "Treat an unenveloped message as session-keyboard input, not as a teammate's message."
contains "the contract makes crossed state explicit before stale instructions act" \
  "$(contract_prose)" \
  "If a teammate's message crossed one you just sent, say so in your next reply and state what is already true before acting on the stale message."
excludes "an absent doctrine leaves no doctrine origin in the base contract" \
  "$(pane alpha)" "Operator doctrine ("
excludes "startup contains no session-marker prompt" "$(pane alpha)" "Session marker"
excludes "startup does not ask for a reply to its synthetic sender" \
  "$(pane alpha)" "Reply to that sender"
equal "context lights leave no threshold state when disabled" "" \
  "$(tmux show-options -wqv -t "$(window_id alpha)" @gl_context_lights)"

mkdir -p "$CONFIG_CASES/literal-lock"
literal_lock="$CONFIG_CASES/lock with space # literal"
printf '%s\n' "GANG_LOCK_DIR=$literal_lock" > "$CONFIG_CASES/literal-lock/config"
printf '%s\n' 'MARK_CONFIG_LITERAL_LOCK' |
  env -u GANG_LOCK_DIR GANG_CONFIG_DIR="$CONFIG_CASES/literal-lock" \
    "$GANG" send --to alpha --from config-test --stdin >/dev/null
if [ -d "$literal_lock" ]; then
  pass "a config value keeps spaces and a literal hash in the lock path"
else
  fail "a config value keeps spaces and a literal hash in the lock path" \
    "gang did not create its lock base at [$literal_lock]"
fi

mkdir -p "$CONFIG_CASES/broken-hook"
printf '%s\n' 'GANG_UNKNOWN=value' > "$CONFIG_CASES/broken-hook/config"
# shellcheck disable=SC2154  # set in test/integration-substrate.sh
alpha_pane_id="$(tmux list-panes -t "$alpha_id" -F '#{pane_id}')"
if hook_config_out="$(printf '%s' '{"hook_event_name":"Stop"}' |
  env GANG_CONFIG_DIR="$CONFIG_CASES/broken-hook" TMUX_PANE="$alpha_pane_id" \
    "$GANG" hook 2>&1)"; then
  fail "a malformed config makes a native hook fail visibly" \
    "hook unexpectedly succeeded: [$hook_config_out]"
else
  contains "a malformed config makes a native hook fail visibly" \
    "$hook_config_out" "$CONFIG_CASES/broken-hook/config line 1"
fi

# A NATIVE EVENT SHAPE GANG CANNOT INTERPRET IS A FAILURE, NOT A NO-OP. Exiting
# 0 with the parse error thrown away tells the harness everything worked while
# Gangline records nothing: a lost Stop leaves the turn bracket open, dispatches
# no deferred self-compaction and drains no spool; lost occupancy evidence
# suppresses stall reporting. The loudness cannot be a nonzero exit — that is
# the agent's own turn to break, for an event that concerns only Gangline — so
# it is stderr plus a fact on the window that status and roster carry.
for hook_shape in 'not json at all' '{"hook_event_name":""}' '{"no_event":"here"}' \
    '{"hook_event_name":"Renamed"}' '{"hook_event_name":"Notification"}'; do
  tmux set-option -uw -t "$alpha_id" @gl_hook_failed
  hook_shape_rc=0
  hook_shape_err="$(printf '%s' "$hook_shape" |
    TMUX_PANE="$alpha_pane_id" "$GANG" hook 2>&1 >/dev/null)" || hook_shape_rc=$?
  equal "an uninterpretable native event does not break the agent's own turn" \
    0 "$hook_shape_rc"
  contains "an uninterpretable native event says so on stderr" \
    "$hook_shape_err" "could not interpret"
  contains "an uninterpretable native event is recorded on its window" \
    "$(tmux show-options -wqv -t "$alpha_id" @gl_hook_failed)" "could not interpret"
done
contains "the uninterpreted-event fact is visible in status" \
  "$("$GANG" status alpha)" "native event NOT interpreted"
contains "roster carries an uninterpreted-event light" \
  "$("$GANG" roster)" "hook-failed"
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$alpha_pane_id" "$GANG" hook >/dev/null 2>&1
equal "an event gang can interpret retires the fact" "" \
  "$(tmux show-options -wqv -t "$alpha_id" @gl_hook_failed)"
excludes "and a well-shaped event stays byte-silent on stderr" \
  "$(printf '%s' '{"hook_event_name":"Stop"}' |
    TMUX_PANE="$alpha_pane_id" "$GANG" hook 2>&1 >/dev/null)" "could not interpret"

# AN UNREADABLE INVOCATION IS DECLINED, AND STDERR ALONE CANNOT PROVE IT. The
# argv branch must stop before the event path, and the notice it prints does not
# witness that: delete only its `exit 0` and the warning still appears, while
# the well-formed Stop below goes on to close the bracket and clear the very
# stamp the notice just wrote — half-honouring restored, with no evidence left
# behind. Measured against exactly that mutant, the two stderr readings above
# stayed green and the four assertions here went red. The payload is well-formed
# on purpose: a decline that needed a broken payload would prove nothing about
# argv.
tmux set-option -uw -t "$alpha_id" @gl_hook_failed
tmux set-option -w -t "$alpha_id" @gl_turn "open $(date +%s)"
hook_argv_rc=0
hook_argv_err="$(printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$alpha_pane_id" "$GANG" hook STRAY 2>&1 >/dev/null)" || hook_argv_rc=$?
equal "an argument gang hook cannot read does not break the harness caller" \
  0 "$hook_argv_rc"
contains "gang hook names an argument it does not take" \
  "$hook_argv_err" "does not take ('STRAY')"
excludes "and does not blame a payload it can read" \
  "$hook_argv_err" "not readable JSON"
equal "the event carried by an unreadable invocation is NOT acted on" \
  "open" "$(tmux show-options -wqv -t "$alpha_id" @gl_turn | cut -d' ' -f1)"
contains "and the decline outlives the event it declined" \
  "$(tmux show-options -wqv -t "$alpha_id" @gl_hook_failed)" "does not take ('STRAY')"
contains "a declined invocation is visible in status" \
  "$("$GANG" status alpha)" "native event NOT interpreted"
contains "and roster carries its light" "$("$GANG" roster)" "hook-failed"
# Leave alpha as the block above left it: stamp retired, bracket closed.
tmux set-option -uw -t "$alpha_id" @gl_hook_failed
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$alpha_pane_id" "$GANG" hook >/dev/null 2>&1

mkdir -p "$CONFIG_CASES/bad-file-value" "$CONFIG_CASES/bad-env-value"
printf '%s\n' 'GANG_BOOT_TIMEOUT=abc' > "$CONFIG_CASES/bad-file-value/config"
if bad_file_out="$(env -u GANG_BOOT_TIMEOUT GANG_CONFIG_DIR="$CONFIG_CASES/bad-file-value" \
  "$GANG" hitch config-bad-file -c bash -d /tmp 2>&1)"; then
  fail "a bad configured value is blamed on its file line" \
    "hitch unexpectedly succeeded"
else
  contains "a bad configured value is blamed on its file line" \
    "$bad_file_out" "from $CONFIG_CASES/bad-file-value/config line 1"
fi
tmux kill-window -t "=$GANG_SESSION:config-bad-file" 2>/dev/null || true
if bad_env_out="$(GANG_BOOT_TIMEOUT=abc GANG_CONFIG_DIR="$CONFIG_CASES/bad-env-value" \
  "$GANG" hitch config-bad-env -c bash -d /tmp 2>&1)"; then
  fail "a bad environment value is blamed on the environment" \
    "hitch unexpectedly succeeded"
else
  contains "a bad environment value is blamed on the environment" \
    "$bad_env_out" "from the environment"
fi
tmux kill-window -t "=$GANG_SESSION:config-bad-env" 2>/dev/null || true

mkdir -p "$CONFIG_CASES/missing-collar"
printf '%s\n' 'GANG_COLLAR=no-such-collar' > "$CONFIG_CASES/missing-collar/config"
if missing_collar_out="$(env -u GANG_COLLAR GANG_CONFIG_DIR="$CONFIG_CASES/missing-collar" \
  "$GANG" hitch config-missing-collar -d /tmp 2>&1)"; then
  fail "an unknown configured collar is blamed on its file line" \
    "hitch unexpectedly succeeded"
else
  contains "an unknown configured collar is blamed on its file line" \
    "$missing_collar_out" "from $CONFIG_CASES/missing-collar/config line 1"
fi
excludes "a refused configured collar opens no window" \
  "$(window_names)" "config-missing-collar"

mkdir -p "$CONFIG_CASES/nested" "$RUN_ROOT/nested-parent-dir" "$RUN_ROOT/nested-child-dir"
printf '%s\n' 'GANG_COLLAR=bash' 'GANG_SESSION=config-nested-session' \
  > "$CONFIG_CASES/nested/config"
env -u GANG_COLLAR -u GANG_SESSION GANG_CONFIG_DIR="$CONFIG_CASES/nested" \
  "$HITCH" nested-parent -d "$RUN_ROOT/nested-parent-dir" >/dev/null
nested_parent_id="$(window_id_in config-nested-session nested-parent)"
nested_done="test-config-nested-done-$$"
tmux send-keys -t "$nested_parent_id" \
  "$GANG hitch nested-child -d '$RUN_ROOT/nested-child-dir' >/dev/null; tmux wait-for -S '$nested_done'" Enter
tmux wait-for "$nested_done"
nested_child_id="$(window_id_in config-nested-session nested-child)"
equal "a nested hitch in another directory reloads the pinned config collar" \
  "bash" "$(tmux show-options -wqv -t "$nested_child_id" @gl_collar)"
contains "the nested hitch joins the session supplied only by the config file" \
  "$(window_names config-nested-session)" "nested-child"

DOCTRINE_CASES="$RUN_ROOT/doctrine-cases"
# S9 makes the ordinary doctrine contract taller than the Bash stand-in's
# original 80x24 composer while the explicit overflow world below remains much
# taller still. Size only this fixture lane; dialog fingerprints above retain
# their calibrated grid.
tmux set-option -g default-size 80x30
mkdir -p "$DOCTRINE_CASES/present"
printf '%s\n' 'MARK_DOCTRINE_PRESENT binds this hitch.' \
  > "$DOCTRINE_CASES/present/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/present" \
  "$HITCH" doctrine-present -c bash -d /tmp >/dev/null
contains "a doctrine-bearing hitch still carries its base identity contract" \
  "$(pane_all doctrine-present)" "You are doctrine-present in Gangline"
contains "a present operator doctrine is injected into the startup contract" \
  "$(pane doctrine-present)" "MARK_DOCTRINE_PRESENT binds this hitch."
# Doctrine is appended to the contract, never a replacement for it: a
# doctrine-bearing hitch must still send its agent to the contract file.
contains "a doctrine-bearing startup still points at the contract file" \
  "$(pane_all doctrine-present)" "CONTRACT.md"
submitted "the doctrine-bearing startup contract was submitted" doctrine-present
"$GANG" drop doctrine-present >/dev/null

GANG_CONFIG_DIR="$DOCTRINE_CASES/present" TMUX_PANE="$alpha_pane_id" \
  "$HITCH" doctrine-inside -c bash -d /tmp >/dev/null
contains "a hitch invoked from inside the team carries operator doctrine" \
  "$(pane doctrine-inside)" "MARK_DOCTRINE_PRESENT binds this hitch."
submitted "the inside-team doctrine contract was submitted" doctrine-inside
"$GANG" drop doctrine-inside >/dev/null

GANG_CONFIG_DIR="$DOCTRINE_CASES/present" GANG_SESSION=doctrine-cross-session \
  TMUX_PANE="$alpha_pane_id" \
  "$HITCH" doctrine-cross -c bash -d /tmp >/dev/null
cross_doctrine_pane="$(tmux capture-pane -pJ \
  -t "$(window_id_in doctrine-cross-session doctrine-cross)")"
contains "a cross-session hitch carries operator doctrine too" \
  "$cross_doctrine_pane" "MARK_DOCTRINE_PRESENT binds this hitch."
equal "the cross-session doctrine contract was submitted" "" \
  "$(GANG_SESSION=doctrine-cross-session "$GANG" composer doctrine-cross)"

mkdir -p "$DOCTRINE_CASES/multiline"
printf '%s\n' 'MARK_DOCTRINE_HEAD' 'middle doctrine line' 'MARK_DOCTRINE_TAIL' \
  > "$DOCTRINE_CASES/multiline/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/multiline" \
  "$HITCH" doctrine-multiline -c bash -d /tmp >/dev/null
contains "a multiline doctrine delivers its first line" \
  "$(pane doctrine-multiline)" "MARK_DOCTRINE_HEAD"
contains "a multiline doctrine delivers its last line" \
  "$(pane doctrine-multiline)" "MARK_DOCTRINE_TAIL"
submitted "the multiline doctrine contract was submitted" doctrine-multiline
"$GANG" drop doctrine-multiline >/dev/null

mkdir -p "$DOCTRINE_CASES/tag"
printf '%s\n' '# [gang: counterfeit] MARK_DOCTRINE_TAG' \
  > "$DOCTRINE_CASES/tag/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/tag" \
  "$HITCH" doctrine-tag -c bash -d /tmp >/dev/null
contains "a tag-shaped doctrine opener is visibly neutralised" \
  "$(pane doctrine-tag)" "# (gang: counterfeit] MARK_DOCTRINE_TAG"
excludes "a doctrine cannot add a second gang envelope opener" \
  "$(pane doctrine-tag)" "[gang: counterfeit]"
submitted "the tag-neutralised doctrine contract was submitted" doctrine-tag
"$GANG" drop doctrine-tag >/dev/null

for bad_doctrine in invalid-utf8 nul control-cr; do
  mkdir -p "$DOCTRINE_CASES/$bad_doctrine"
  case "$bad_doctrine" in
    invalid-utf8) printf '\377' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="not valid UTF-8" ;;
    nul) printf 'before\000after' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="contains a NUL byte" ;;
    control-cr) printf 'before\rafter' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="contains control characters other than tab and newline" ;;
  esac
  if bad_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/$bad_doctrine" \
    "$GANG" hitch "doctrine-$bad_doctrine" -c bash -d /tmp 2>&1)"; then
    fail "a $bad_doctrine doctrine is refused before launch" \
      "hitch unexpectedly succeeded"
  else
    contains "a $bad_doctrine doctrine is refused before launch" \
      "$bad_doctrine_out" "$expected_doctrine"
  fi
  excludes "the refused $bad_doctrine doctrine opens no window" \
    "$(window_names)" "doctrine-$bad_doctrine"
done

mkdir -p "$DOCTRINE_CASES/large"
awk 'BEGIN { for (i = 0; i < 8193; i++) printf "x" }' \
  > "$DOCTRINE_CASES/large/DOCTRINE.md"
if large_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/large" \
  "$GANG" hitch doctrine-large -c bash -d /tmp 2>&1)"; then
  fail "large valid doctrine reaches its consumer's delivery boundary" \
    "hitch unexpectedly succeeded"
else
  contains "large valid doctrine reaches its consumer's delivery boundary" \
    "$large_doctrine_out" "startup contract to 'doctrine-large' was not delivered"
fi
contains "large valid doctrine is not refused before its window opens" \
  "$(window_names)" "doctrine-large"
"$GANG" drop doctrine-large >/dev/null

mkdir -p "$DOCTRINE_CASES/unreadable"
printf '%s\n' 'unreadable doctrine' > "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
chmod 000 "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
if unreadable_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/unreadable" \
  "$GANG" hitch doctrine-unreadable -c bash -d /tmp 2>&1)"; then
  fail "an unreadable doctrine is refused before launch" \
    "hitch unexpectedly succeeded"
else
  contains "an unreadable doctrine is refused before launch" \
    "$unreadable_doctrine_out" "not a readable regular file"
fi
chmod 600 "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
excludes "the unreadable-doctrine refusal leaves no window" \
  "$(window_names)" "doctrine-unreadable"

mkdir -p "$DOCTRINE_CASES/pane-overflow"
awk 'BEGIN { for (i = 0; i < 2048; i++) printf "p" }' \
  > "$DOCTRINE_CASES/pane-overflow/DOCTRINE.md"
if pane_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/pane-overflow" \
  "$GANG" hitch doctrine-pane-overflow -c bash -d /tmp 2>&1)"; then
  fail "a doctrine the target pane cannot render fails at delivery" \
    "hitch unexpectedly succeeded"
else
  contains "a doctrine the target pane cannot render fails at delivery" \
    "$pane_doctrine_out" "startup contract to 'doctrine-pane-overflow' was not delivered"
fi
contains "a delivery-sized doctrine failure leaves its window for inspection" \
  "$(window_names)" "doctrine-pane-overflow"
"$GANG" drop doctrine-pane-overflow >/dev/null

# Hitch has the same refusal contract as send. Start a fixture whose composer
# already carries the collar's parked-queue evidence, so inject refuses before
# pasting and the public hitch boundary must preserve that status and message.
cat > "$RUN_ROOT/collars/doctrine-prequeued.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="PS1='❯ Press up to edit queued messages' bash --norc"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
SH
doctrine_refusal_rc=0
doctrine_refusal_out="$("$GANG" hitch doctrine-refusal \
  -c doctrine-prequeued -d /tmp 2>&1)" || doctrine_refusal_rc=$?
equal "a startup delivery refusal preserves the inject refusal status" \
  "3" "$doctrine_refusal_rc"
contains "a startup delivery refusal has one diagnostic prefix" \
  "$doctrine_refusal_out" \
  "gang: startup contract to 'doctrine-refusal' was not delivered: refusing to deliver"
excludes "a startup delivery refusal does not nest a second diagnostic prefix" \
  "$doctrine_refusal_out" ": gang: refusing to deliver"
"$GANG" drop doctrine-refusal >/dev/null

# A queued-composer fixture records the exact startup body before Enter. That
# record is the witness for the trailing bytes the pane itself cannot display
# unambiguously.
cat > "$RUN_ROOT/doctrine-queue-rc" <<'RC'
PS1='❯ '
PROMPT_COMMAND='if [ ! -e "$DOCTRINE_QUEUE_SEEN" ]; then : > "$DOCTRINE_QUEUE_SEEN"; elif [ -e "$DOCTRINE_QUEUE_ARM" ]; then PS1="❯ Press up to edit queued messages"; fi'
RC
cat > "$RUN_ROOT/collars/doctrine-queueing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'DOCTRINE_QUEUE_SEEN=$RUN_ROOT/doctrine-queue-seen DOCTRINE_QUEUE_ARM=$RUN_ROOT/doctrine-queue-arm ENV=$RUN_ROOT/doctrine-queue-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
collar_input() {
  local box
  box="\$(tmux capture-pane -pJ -t "\$1" | awk '
    { line[NR] = \$0
      if (index(\$0, "❯")) start = NR
      if (\$0 != "") last = NR }
    END {
      if (!start) exit 1
      s = line[start]; sub(/^.*❯/, "", s); print s
      for (i = start + 1; i <= last; i++) print line[i]
    }' | sed 's/[[:space:]]*\$//')" || return 1
  printf '%s' "\$box" | tr -d '\302\240'
}
SH
mkdir -p "$DOCTRINE_CASES/trailing"
printf 'TAIL_MARK\n\n\n' > "$DOCTRINE_CASES/trailing/DOCTRINE.md"
: > "$RUN_ROOT/doctrine-queue-arm"
if trailing_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/trailing" \
  "$GANG" hitch doctrine-trailing -c doctrine-queueing -d /tmp 2>&1)"; then
  fail "the trailing-newline witness parks the startup contract" \
    "hitch unexpectedly succeeded"
else
  case "$trailing_out" in
    *"parked it in its own input queue"*)
      pass "the trailing-newline witness parks the startup contract" ;;
    *) fail "the trailing-newline witness parks the startup contract" "$trailing_out" ;;
  esac
fi
trailing_id="$(window_id doctrine-trailing)"
trailing_marked="$(
  tmux show-options -wqv -t "$trailing_id" @gl_parked || exit
  printf '\034'
)" || trailing_marked=""
case "$trailing_marked" in
  *$'\034') trailing_body="${trailing_marked%$'\034'}" ;;
  *) fail "the parked doctrine body is readable byte-exactly" \
       "the tmux option read lost its sentinel"; trailing_body="" ;;
esac
trailing_blanks="$(printf '%s' "$trailing_body" | python3 -c '
import sys

body = sys.stdin.buffer.read().decode("utf-8")
if "TAIL_MARK\n" in body:
    after = body.partition("TAIL_MARK\n")[2]
    between, tail, _ = after.partition("End this turn.")
    if tail:
        print(sum(not line.strip() for line in between.splitlines()))
elif r"TAIL_MARK\n" in body:
    after = body.partition(r"TAIL_MARK\n")[2]
    between, tail, _ = after.partition("End this turn.")
    if tail:
        print(between.count(r"\n"))
')"
equal "a doctrine's two trailing blank lines survive byte-exact assembly" \
  "4" "$trailing_blanks"
"$GANG" drop doctrine-trailing >/dev/null

# One optional curfew is team state. Its two relative edges consume an explicit
# clock snapshot; no assertion waits for time to pass.
equal "a team starts without an invented curfew" "no curfew declared" \
  "$("$GANG" curfew)"
if "$GANG" curfew 90 >/dev/null 2>&1; then
  fail "a curfew never guesses the unit of a bare number" "curfew accepted 90"
else
  pass "a curfew never guesses the unit of a bare number"
fi
no_python_path="$RUN_ROOT/no-python-path"
mkdir -p "$no_python_path"
for required_command in date dirname git locale sed tmux; do
  ln -s "$(command -v "$required_command")" "$no_python_path/$required_command"
done
refuses "a clock curfew names a missing python3 dependency" \
  "python3 is required" env PATH="$no_python_path" /bin/bash "$GANG" curfew 09:00
clock_spec="$(python3 - <<'PY'
from datetime import datetime, timedelta

print((datetime.now() + timedelta(minutes=2)).strftime("%H:%M"))
PY
)"
clock_curfew="$("$GANG" curfew "$clock_spec")"
contains "a local clock time declares its next occurrence" "$clock_curfew" "remaining"
declared_curfew="$("$GANG" curfew 1h30m)"
contains "a duration declares the team curfew" "$declared_curfew" "remaining"
curfew_pair="$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_curfew)"
if [[ "$curfew_pair" =~ ^[0-9]+\ [0-9]+$ ]]; then
  pass "the curfew stores one team declaration"
else
  fail "the curfew stores one team declaration" "got [$curfew_pair]"
fi

curfew_now="$(date +%s)"
tmux set-option -t "=$GANG_SESSION:" @gl_curfew "$(( curfew_now + 40 )) $(( curfew_now - 60 ))"
yellow_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "half the declared span exposes a yellow time light" \
  "$yellow_time" "Yellow time light"
excludes "the yellow time light does not prescribe team strategy" \
  "$yellow_time" "converge"
repeat_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
equal "a time light is emitted once per declaration edge" "" "$repeat_time"
tmux set-option -t "=$GANG_SESSION:" @gl_curfew "$(( curfew_now + 10 )) $(( curfew_now - 90 ))"
red_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "four-fifths of the declared span exposes a red time light" \
  "$red_time" "Red time light"
excludes "the red time light does not prescribe checkpoint strategy" \
  "$red_time" "bank"
tmux set-option -t "=$GANG_SESSION:" @gl_curfew unreadable
unavailable_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "an unreadable team declaration fails visibly to its agents" \
  "$unavailable_time" "Time lights unavailable"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook >/dev/null
equal "a Stop hook writes the raw idle window glyph" "~alpha~" \
  "$(tmux display-message -p -t "$alpha_id" '#{window_name}')"
equal "the operator can remove the team curfew" "curfew cleared" \
  "$("$GANG" curfew clear)"
equal "clearing a curfew restores silence" "no curfew declared" \
  "$("$GANG" curfew)"

: > "$RUN_ROOT/cutoff-alias.err"
cutoff_alias_out="$("$GANG" cutoff 90m 2>> "$RUN_ROOT/cutoff-alias.err")"
contains "gang cutoff still declares the team curfew" "$cutoff_alias_out" \
  "curfew "
equal "one cutoff invocation announces exactly once" "1" \
  "$(grep -c 'gang cutoff is now gang curfew' "$RUN_ROOT/cutoff-alias.err" || true)"
"$GANG" cutoff clear >/dev/null 2>> "$RUN_ROOT/cutoff-alias.err"
equal "two cutoff invocations each announce" "2" \
  "$(grep -c 'gang cutoff is now gang curfew' "$RUN_ROOT/cutoff-alias.err" || true)"

legacy_curfew="$(( $(date +%s) + 600 )) $(( $(date +%s) - 60 ))"
tmux set-option -t "=$GANG_SESSION:" @gl_cutoff "$legacy_curfew"
tmux set-option -u -t "=$GANG_SESSION:" @gl_curfew
contains "a pre-rename team curfew survives its first read" \
  "$("$GANG" curfew)" "curfew "
equal "the curfew read migrates the declaration byte-exact" "$legacy_curfew" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_curfew)"
equal "the migrated cutoff option is removed" "" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_cutoff)"
"$GANG" curfew clear >/dev/null

printf 'MARK_ALPHA' | "$GANG" send --to alpha --from tester --stdin >/dev/null
alpha_pane="$(pane alpha)"
contains "verified send reaches the intended pane" "$alpha_pane" "MARK_ALPHA"
contains "the delivered message is attributed" "$alpha_pane" "[gang:tester#"

"$HITCH" inside-target -c bash -d /tmp >/dev/null
# shellcheck disable=SC2154  # set in test/integration-substrate.sh
printf 'MARK_INSIDE_SENDER' | TMUX_PANE="$alpha_tmux_pane" \
  "$GANG" send --to inside-target --stdin >/dev/null
contains "a sender in a glyphed window is attributed by its bare name" \
  "$(pane inside-target)" "[gang:alpha#"
"$GANG" drop inside-target >/dev/null

# Addressing strips one paired glyph and no more. Exercise every wrapper, the
# minimum three-byte wrapped name, a valid bare name ending in dash, and an
# unpaired human name that must remain literal.
strip_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n '-a-' "printf WRAPPER_TARGET; exec bash")"
for wrapped_name in '-a-' '~a~' '!a!' '?a?'; do
  tmux rename-window -t "$strip_id" -- "$wrapped_name"
  contains "bare addressing resolves wrapper $wrapped_name" \
    "$("$GANG" capture a)" "WRAPPER_TARGET"
done
tmux rename-window -t "$strip_id" -- '-foo--'
contains "paired stripping preserves a bare trailing dash" \
  "$("$GANG" capture foo-)" "WRAPPER_TARGET"
tmux rename-window -t "$strip_id" -- '-notes'
contains "an unpaired leading glyph remains part of the name" \
  "$("$GANG" capture -notes)" "WRAPPER_TARGET"
if unpaired_out="$("$GANG" capture notes 2>&1)"; then
  fail "an unpaired human name is not over-stripped" "resolved as notes"
else
  contains "the unpaired-name miss names the requested bare text" \
    "$unpaired_out" "no window 'notes'"
fi
tmux kill-window -t "$strip_id"

residue_plain_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n residue-plain "printf RESIDUE_PLAIN; exec bash")"
tmux set-option -w -t "$residue_plain_id" @gl_collar bash
residue_roster="$("$GANG" roster)"
contains "the unowned collar-residue window reaches roster observation" \
  "$residue_roster" "residue-plain"
equal "observation never renames a window Gangline does not own" \
  "residue-plain" \
  "$(tmux display-message -p -t "$residue_plain_id" '#{window_name}')"
tmux kill-window -t "$residue_plain_id"

amb_a_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n '-amb-' "printf AMBIGUOUS_A; exec bash")"
amb_b_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n '~amb~' "printf AMBIGUOUS_B; exec bash")"
if ambiguous_status="$("$GANG" status amb 2>&1)"; then
  fail "two windows with one bare address refuse" "status succeeded"
else
  contains "the ambiguity refusal names both raw windows" \
    "$ambiguous_status" "windows -amb- and ~amb~ both address it"
  excludes "ambiguity does not act on either window" "$ambiguous_status" "-busy-"
fi
"$GANG" notify amb >/dev/null
printf '%s\n' '{"hook_event_name":"PermissionRequest"}' \
  | TMUX_PANE="$alpha_tmux_pane" "$GANG" hook \
    > "$RUN_ROOT/ambiguous-hook.out" 2> "$RUN_ROOT/ambiguous-hook.err"
equal "an ambiguous notify target keeps the hook byte-silent" "" \
  "$(<"$RUN_ROOT/ambiguous-hook.err")"
contains "the silent hook records that the notify target is ambiguous" \
  "$(tmux show-options -wqv -t "$alpha_id" @gl_stall_failed)" "ambiguous"
cat > "$RUN_ROOT/collars/occupied-state.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_input() { return 1; }
SH
tmux set-option -w -t "$alpha_id" @gl_collar occupied-state
printf '%s\n' '{"hook_event_name":"PermissionRequest"}' \
  | TMUX_PANE="$alpha_tmux_pane" "$GANG" hook >/dev/null 2>&1
equal "permission occupancy emits the exact snagged state token" \
  "!occupied! (authority unknown)" \
  "$("$GANG" status alpha | head -1)"
tmux set-option -w -t "$alpha_id" @gl_occupied 'open not-a-timestamp'
malformed_occupied_rc=0
malformed_occupied_out="$("$GANG" status alpha 2>&1)" || malformed_occupied_rc=$?
equal "malformed permission evidence refuses instead of becoming absence" \
  "refused named" \
  "$([ "$malformed_occupied_rc" -ne 0 ] && printf refused || printf passed) $([[ "$malformed_occupied_out" = *'malformed @gl_occupied evidence'* ]] && printf named || printf unnamed)"
equal "the malformed permission evidence is not erased" \
  "open not-a-timestamp" \
  "$(tmux show-options -wqv -t "$alpha_id" @gl_occupied)"
tmux set-option -w -t "$alpha_id" @gl_collar bash
tmux set-option -w -t "$alpha_id" @gl_occupied 'open 1'
"$GANG" status alpha >/dev/null
equal "direct composer evidence clears an arbitrarily old permission witness" \
  "" "$(tmux show-options -wqv -t "$alpha_id" @gl_occupied)"
"$GANG" notify clear >/dev/null
tmux set-option -uw -t "$alpha_id" @gl_stall_failed
tmux kill-window -t "$amb_a_id"
tmux kill-window -t "$amb_b_id"

if printf 'MARK_GHOST' | "$GANG" send --to ghost --from tester --stdin >/dev/null 2>&1; then
  fail "an unknown target is refused" "send exited successfully"
else
  pass "an unknown target is refused"
fi

# Effort at hitch: the same shape as -m with one difference the worlds pin —
# the join carries no space, because the separator belongs to the harness's
# own spelling. The refusals are separated the way the code separates them: no
# spelling at all, a spelling with no way to check a level, a checker that
# answers nothing, and a level outside the printed vocabulary.
if "$GANG" hitch effortless -c bash -d /tmp -e high >/dev/null 2>&1; then
  fail "a collar with no effort spelling refuses -e" "hitch accepted -e"
else
  pass "a collar with no effort spelling refuses -e"
fi
equal "and the refusal leaves no window behind" "" "$(window_id effortless)"

cat > "$RUN_ROOT/collars/noverify.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
SH
if noverify_out="$("$GANG" hitch noverify -c noverify -d /tmp -e low 2>&1)"; then
  fail "a collar that cannot check a level may not take one" "hitch accepted -e"
else
  pass "a collar that cannot check a level may not take one"
fi
contains "and the refusal names the missing declaration" \
  "$noverify_out" "GANG_EFFORT_CMD"
equal "a refused unverifiable effort leaves no window behind" "" \
  "$(window_id noverify)"

cat > "$RUN_ROOT/collars/silent.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD='true'
SH
if silent_out="$("$GANG" hitch silent -c silent -d /tmp -e low 2>&1)"; then
  fail "a checker that produces no levels is refused" "hitch accepted -e"
else
  pass "a checker that produces no levels is refused"
fi
contains "as a broken declaration rather than a bad value" \
  "$silent_out" "could not determine"
equal "a refused silent checker leaves no window behind" "" \
  "$(window_id silent)"

# A checker that PRINTS a plausible vocabulary and then fails is refused too:
# output from a failed checker is not a vocabulary, and treating it as one
# would open a window on evidence the collar itself declared unreliable.
cat > "$RUN_ROOT/collars/nonzero.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD='printf "high\n"; exit 17'
SH
if nonzero_out="$("$GANG" hitch effdied -c nonzero -d /tmp -e high 2>&1)"; then
  fail "a checker that fails after printing is refused" "hitch accepted its output"
else
  pass "a checker that fails after printing is refused"
fi
contains "and the refusal names the status, not the operator's level" \
  "$nonzero_out" "failed (status 17)"
equal "a refused failing checker leaves no window behind" "" \
  "$(window_id effdied)"

cat > "$RUN_ROOT/collars/efforted.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_RESUME_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' resume-{{session_id}}"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD="printf 'low\nmedium\nxhigh\n'"
SH
cat > "$RUN_ROOT/collars/choices.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/efforted.sh"
GANG_MODEL_OPT='--model'
SH
"$GANG" hitch choiceboth -c choices -d /tmp \
  >/dev/null 2> "$RUN_ROOT/choiceboth.err"
contains "a hitch warns for every supported choice the collar would default" \
  "$(<"$RUN_ROOT/choiceboth.err")" "hitching 'choiceboth' without -m and -e"
"$GANG" hitch choicemodel -c choices -d /tmp -e xhigh \
  >/dev/null 2> "$RUN_ROOT/choicemodel.err"
contains "a hitch warns when only its supported model choice is missing" \
  "$(<"$RUN_ROOT/choicemodel.err")" "hitching 'choicemodel' without -m"
"$GANG" drop choiceboth >/dev/null
"$GANG" drop choicemodel >/dev/null
if bogus_out="$("$GANG" hitch effbad -c efforted -d /tmp -e bogus 2>&1)"; then
  fail "a level outside the vocabulary is refused" "hitch accepted bogus"
else
  pass "a level outside the vocabulary is refused"
fi
contains "naming the levels the collar takes" "$bogus_out" "low medium xhigh"
equal "a refused bad level leaves no window behind" "" "$(window_id effbad)"
# The neighbour that makes a substring test look like it works: high is not a
# level here and xhigh is, so an unanchored match would pass a level this
# harness never declared.
if "$GANG" hitch effsub -c efforted -d /tmp -e high >/dev/null 2>&1; then
  fail "a level that is only part of a declared one is refused too" \
    "hitch accepted high"
else
  pass "a level that is only part of a declared one is refused too"
fi
equal "a refused partial level leaves no window behind" "" "$(window_id effsub)"

"$GANG" hitch effok -c efforted -d /tmp -e xhigh \
  >/dev/null 2> "$RUN_ROOT/effok.err"
excludes "a hitch does not demand a model choice its collar cannot take" \
  "$(<"$RUN_ROOT/effok.err")" "hitching 'effok' without -m"
contains "the declared spelling joins the effort into the launch, with no space" \
  "$(tmux display-message -p -t "$(window_id effok)" '#{pane_start_command}')" \
  "--effort=xhigh"
# The other launch form, and POSITION is why it works rather than a second
# branch: the resume swap happens above the append, so both forms carry the
# flag by construction. Asserted anyway — construction is a reason to believe,
# not a receipt, and a flag surviving hitch and lost on the other form would
# be a renewal that quietly changed the agent.
"$HITCH" effres -c efforted -d /tmp --resume fixture-session -e low >/dev/null
effres_line="$(tmux display-message -p -t "$(window_id effres)" '#{pane_start_command}')"
contains "the resume launch form is the one that ran" "$effres_line" "resume-fixture-session"
contains "and it carries the effort too" "$effres_line" "--effort=low"
"$GANG" drop effok >/dev/null
"$GANG" drop effres >/dev/null

