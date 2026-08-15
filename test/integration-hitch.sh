# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Hitch: first-run prompts, native event shapes gang cannot interpret, refusal contracts, the team curfew, addressing, and model and effort selection.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# Hitch names a first-run prompt and parks on positive evidence, and answers
# nothing. Two event barriers make that exact: a sleep shim holds gang's first
# observation until the fixture has painted, so the verdict is about a menu that
# is on screen rather than about a pane that was still starting; and the output
# pipe holds the test until the accepted line is written, so only the manual
# keys that follow it can reach the UI.
prompt_boot_barrier() { # $1 = bin dir, $2 = seen marker, $3 = fixture channel
  mkdir -p "$1"
  cat > "$1/sleep" <<SH
#!/bin/sh
if [ ! -e '$2' ]; then
  : > '$2'
  tmux wait-for '$3'
fi
exit 0
SH
  chmod +x "$1/sleep"
}
cat > "$RUN_ROOT/collars/dialog-observe-boot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_LAUNCH="env DIALOG_VARIANT=known DIALOG_KEY_LOG='$RUN_ROOT/dialog-observe-boot.keys' DIALOG_READY='dialog-observe-boot-ready-$$' '$RUN_ROOT/dialog-fixture.py'"
SH
: > "$RUN_ROOT/dialog-observe-boot.keys"
prompt_boot_barrier "$RUN_ROOT/observe-boot-bin" \
  "$RUN_ROOT/observe-boot-seen" "dialog-observe-boot-ready-$$"
observe_boot_pipe="$RUN_ROOT/dialog-observe-boot.out"
mkfifo "$observe_boot_pipe"
exec 8<>"$observe_boot_pipe"
PATH="$RUN_ROOT/observe-boot-bin:$PATH" \
  "$GANG" hitch dialog-observe-boot -c dialog-observe-boot -d /tmp >&8 2>&1 &
observe_boot_pid=$!
observe_boot_out=""
while IFS= read -r observe_boot_line <&8; do
  observe_boot_out="${observe_boot_out}${observe_boot_out:+$'\n'}$observe_boot_line"
  case "$observe_boot_line" in
    *"nothing further is needed from you"*) break ;;
  esac
done
pass "a first-run prompt is accepted immediately on positive evidence"
contains "hitch names the prompt it is waiting on" \
  "$observe_boot_out" \
  "is waiting on a first-run prompt"
contains "and points at the one way to answer it" \
  "$observe_boot_out" "gang attach"
equal "hitch sends no key to the prompt" "" \
  "$(<"$RUN_ROOT/dialog-observe-boot.keys")"
observe_boot_id="$(window_id dialog-observe-boot)"
tmux send-keys -t "$observe_boot_id" Down Enter
if wait "$observe_boot_pid"; then
  pass "hitch continues after the operator answers the prompt"
else
  fail "hitch continues after the operator answers the prompt" \
    "$observe_boot_out"
fi
exec 8>&-
equal "only the operator's manual answer reaches the prompt" \
  $'Down\nEnter' "$(<"$RUN_ROOT/dialog-observe-boot.keys")"
# source-guard: producer@be4e8d85b13f: the successful hitch above is the sole producer of this nonce-addressed startup body, and the key log independently proves the fixture's composer was restored by the operator's own answer
contains "the post-prompt startup contract is delivered" \
  "$(pane dialog-observe-boot)" "You are dialog-observe-boot in Gangline"
"$GANG" drop dialog-observe-boot >/dev/null

# The shipped Codex directory-trust prompt was the one screen hitch -d used to
# answer on the operator's behalf. It is now an ordinary unanswered prompt: no
# key, a parked contract, and a recovery that does not prescribe a drop.
cat > "$RUN_ROOT/collars/dialog-trust-boot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_LAUNCH="env DIALOG_VARIANT=trust DIALOG_KEY_LOG='$RUN_ROOT/dialog-trust-boot.keys' DIALOG_READY='dialog-trust-boot-ready-$$' '$RUN_ROOT/dialog-fixture.py'"
SH
: > "$RUN_ROOT/dialog-trust-boot.keys"
prompt_boot_barrier "$RUN_ROOT/trust-boot-bin" \
  "$RUN_ROOT/trust-boot-seen" "dialog-trust-boot-ready-$$"
trust_boot_pipe="$RUN_ROOT/dialog-trust-boot.out"
mkfifo "$trust_boot_pipe"
exec 8<>"$trust_boot_pipe"
PATH="$RUN_ROOT/trust-boot-bin:$PATH" \
  "$GANG" hitch trust-boot -c dialog-trust-boot -d /tmp >&8 2>&1 &
trust_boot_pid=$!
trust_boot_out=""
while IFS= read -r trust_boot_line <&8; do
  trust_boot_out="${trust_boot_out}${trust_boot_out:+$'\n'}$trust_boot_line"
  case "$trust_boot_line" in
    *"nothing further is needed from you"*) break ;;
  esac
done
equal "directory trust receives no key from hitch -d" "" \
  "$(<"$RUN_ROOT/dialog-trust-boot.keys")"
contains "the parked contract names the manual recovery" \
  "$trust_boot_out" "gang attach"
excludes "and the recovery does not prescribe an unconditional drop" \
  "$trust_boot_out" "then 'gang drop"
tmux send-keys -t "$(window_id trust-boot)" Enter
if wait "$trust_boot_pid"; then
  pass "hitch delivers once the operator answers directory trust"
else
  fail "hitch delivers once the operator answers directory trust" \
    "$trust_boot_out"
fi
exec 8>&-
equal "only the operator's Enter reached the trust prompt" "Enter" \
  "$(<"$RUN_ROOT/dialog-trust-boot.keys")"
# source-guard: producer@e417b69a0f67: the waited-on hitch is the sole producer of this body, and the trust prompt's key log shows only the operator's Enter before it
contains "the post-prompt hitch delivers its startup contract" \
  "$(pane trust-boot)" "You are trust-boot in Gangline"
submitted "the post-prompt startup contract was submitted" trust-boot
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
# Positive prompt evidence now commits the contract immediately, so the old
# "warning retracted" assertion described a pre-commit wait that no longer
# exists. Acceptance and the later verified drain are the two truthful states.
contains "the resumed hitch reports verified startup delivery" \
  "$(<"$modal_output")" "delivered queued startup contract to boot-modal"
contains "the resumed hitch reports accepted-before-delivered state" \
  "$(<"$modal_output")" "queued startup contract for boot-modal — accepted, not yet in the session"
submitted "the resumed startup contract was submitted" boot-modal
"$GANG" drop boot-modal >/dev/null

# A FIRST-RUN GATE THAT OUTLIVES HITCH'S BOOT BUDGET has no turn and therefore
# no Stop. The old assertion required hitch to fail and prescribe a second
# manual `gang send`; that behavior was wrong because it dropped a fully formed
# startup envelope. Hitch now parks it, remains the foreground owner, and keeps
# observing the tty. This collar deliberately has no hook, proving that a Codex
# operator choosing "continue without hooks" cannot invalidate the promise.
startup_gate_clear="test-startup-gate-clear-$$"
cat > "$RUN_ROOT/startup-gate.sh" <<SH
#!/bin/sh
printf 'FIRST_RUN_GATE\n'
tmux wait-for "$startup_gate_clear"
PS1='❯ ' exec bash --norc
SH
chmod +x "$RUN_ROOT/startup-gate.sh"
cat > "$RUN_ROOT/collars/startup-gated.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="'$RUN_ROOT/startup-gate.sh'"
GANG_OCCUPIED_REGEX='FIRST_RUN_GATE'
SH
startup_gate_pipe="$RUN_ROOT/startup-gate.out"
mkfifo "$startup_gate_pipe"
exec 9<>"$startup_gate_pipe"
"$GANG" hitch startup-gated \
  -c startup-gated -d /tmp >&9 2>&1 &
startup_gate_hitch=$!
startup_gate_out=""
while IFS= read -r startup_gate_line <&9; do
  startup_gate_out="${startup_gate_out}${startup_gate_out:+$'\n'}$startup_gate_line"
  case "$startup_gate_line" in
    *"nothing further is needed from you"*) break ;;
  esac
done
pass "a hookless first-run gate parks its startup contract"
contains "hitch reports the startup contract as accepted but not delivered" \
  "$startup_gate_out" "queued startup contract for startup-gated — accepted, not yet in the session"
contains "the only requested operator action is answering the native prompt" \
  "$startup_gate_out" "answer the observed native startup prompt with 'gang attach'"
contains "hitch says no second delivery action is needed" \
  "$startup_gate_out" "nothing further is needed from you"
contains "hitch owns delivery even when no native hook will run" \
  "$startup_gate_out" "this hitch will deliver the contract when the composer appears"
excludes "hitch no longer asks the operator to send the startup contract" \
  "$startup_gate_out" "gang send --to startup-gated"
if kill -0 "$startup_gate_hitch" 2>/dev/null; then
  pass "hitch remains foreground while the operator owns the startup gate"
else
  fail "hitch remains foreground while the operator owns the startup gate" \
    "hitch exited before the operator answered"
fi
contains "the undelivered startup contract is visible in the ordinary spool" \
  "$("$GANG" status startup-gated)" "spooled: 1"
excludes "nothing was typed through the startup gate" \
  "$(pane startup-gated)" "You are startup-gated in Gangline"
tmux wait-for -S "$startup_gate_clear"
while IFS= read -r startup_gate_line <&9; do
  startup_gate_out="${startup_gate_out}${startup_gate_out:+$'\n'}$startup_gate_line"
  case "$startup_gate_line" in
    *"delivered queued startup contract to startup-gated"*) break ;;
  esac
done
if wait "$startup_gate_hitch"; then
  pass "the foreground hitch completes after the operator clears the gate"
else
  fail "the foreground hitch completes after the operator clears the gate" \
    "$startup_gate_out"
fi
exec 9>&-
# source-guard: producer@cda0e8616113: the hookless fixture above exposes its composer only after the manual gate clears, and the nonce-addressed spool is the sole producer of this startup body
contains "foreground hitch delivers the parked contract after the composer appears" \
  "$(pane startup-gated)" "You are startup-gated in Gangline"
excludes "the verified foreground drain retires the startup spool entry" \
  "$("$GANG" status startup-gated)" "spooled:"
submitted "the foreground-delivered startup contract was submitted" startup-gated
"$GANG" drop startup-gated >/dev/null

# A SECOND NATIVE PROMPT MAY FOLLOW THE FIRST GATE. The foreground observer
# stays quiet after its one operator notice and keeps observing, so the contract
# committed before the first gate still lands after the operator answers the
# second. This models the Codex ordering where hook review precedes directory
# trust; gang answers neither.
startup_second_clear="test-startup-second-clear-$$"
cat > "$RUN_ROOT/startup-second.sh" <<SH
#!/bin/sh
printf 'FIRST_RUN_GATE\n'
tmux wait-for "$startup_second_clear"
exec env DIALOG_VARIANT=trust \
  DIALOG_KEY_LOG='$RUN_ROOT/startup-second.keys' \
  DIALOG_READY='test-startup-second-ready-$$' \
  '$RUN_ROOT/dialog-fixture.py'
SH
chmod +x "$RUN_ROOT/startup-second.sh"
cat > "$RUN_ROOT/collars/startup-second.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="'$RUN_ROOT/startup-second.sh'"
GANG_OCCUPIED_REGEX='FIRST_RUN_GATE|^› [0-9]+\. '
SH
: > "$RUN_ROOT/startup-second.keys"
startup_second_pipe="$RUN_ROOT/startup-second.out"
mkfifo "$startup_second_pipe"
exec 8<>"$startup_second_pipe"
"$GANG" hitch startup-second -c startup-second -d /tmp >&8 2>&1 &
startup_second_hitch=$!
startup_second_out=""
while IFS= read -r startup_second_line <&8; do
  startup_second_out="${startup_second_out}${startup_second_out:+$'\n'}$startup_second_line"
  case "$startup_second_line" in
    *"nothing further is needed from you"*) break ;;
  esac
done
contains "the first gate commits the startup contract before its successor" \
  "$startup_second_out" "queued startup contract for startup-second"
tmux wait-for -S "$startup_second_clear"
tmux wait-for "test-startup-second-ready-$$"
equal "the second prompt receives no key from the observer" "" \
  "$(<"$RUN_ROOT/startup-second.keys")"
tmux send-keys -t "$(window_id startup-second)" Enter
if wait "$startup_second_hitch"; then
  pass "the foreground observer continues through a second prompt"
else
  fail "the foreground observer continues through a second prompt" \
    "$startup_second_out"
fi
exec 8>&-
equal "the second prompt receives only the operator's answer" \
  "Enter" "$(<"$RUN_ROOT/startup-second.keys")"
# source-guard: producer@097c1dc1b2d2: the only startup-second body is the nonce-bound startup entry committed before the gate cleared, and the answered-key log independently proves the successor prompt completed before this read
contains "the startup contract follows the second prompt into the session" \
  "$(pane startup-second)" "You are startup-second in Gangline"
excludes "the second-prompt drain retires the startup entry" \
  "$("$GANG" status startup-second)" "spooled:"
submitted "the post-second-prompt startup contract was submitted" startup-second
"$GANG" drop startup-second >/dev/null

# `gang up` OWNS BOTH SIDES OF THE SAME FIRST-RUN GATE: its tmux client must
# expose the prompt while the original hitch invocation keeps the contract
# observer alive. Event barriers witness the client attach and the second shell
# prompt (the one after the startup envelope was submitted); no Stop hook helps.
startup_up_clear="test-startup-up-clear-$$"
startup_up_attach_outcome="test-startup-up-attach-outcome-$$"
startup_up_process_done="test-startup-up-process-done-$$"
startup_up_delivered="test-startup-up-delivered-$$"
cat > "$RUN_ROOT/startup-up.sh" <<SH
#!/bin/sh
printf 'FIRST_RUN_GATE\n'
tmux wait-for "$startup_up_clear"
PS1='❯ ' exec bash --norc
SH
chmod +x "$RUN_ROOT/startup-up.sh"
cat > "$RUN_ROOT/collars/startup-up.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="'$RUN_ROOT/startup-up.sh'"
GANG_OCCUPIED_REGEX='FIRST_RUN_GATE'
SH
mkdir -p "$GANG_CONFIG_DIR/roles"
printf '%s\n' 'STARTUP_UP_ROLE' > "$GANG_CONFIG_DIR/roles/startup-up.md"
tmux set-hook -g client-attached \
  "set-option -g @test_startup_up_attached yes; wait-for -S $startup_up_attach_outcome"
tmux wait-for "$startup_up_attach_outcome" &
startup_up_attached_waiter=$!
tmux wait-for "$startup_up_process_done" &
startup_up_process_waiter=$!
# An attached client adds a real terminal-render path that the suite's global
# compressed sleep is not calibrated for. Use the production verification
# clock and a roomy but ordinary client viewport; the suite header records why
# a 0.05s readback under asynchronous render is unknown rather than failure.
# `script` supplies the pty but inherits TERM. A hosted non-tty shell advertises
# no usable terminal, so name the synthetic terminal this fixture means to
# provide instead of asking tmux to infer one from its absent parent terminal.
#
# The driver exit and the attach hook signal the same outcome barrier. The hook
# records positive attachment first; an attach refusal records its status and
# output before releasing the barrier. Either outcome is immediate evidence, so
# a missing client cannot strand the suite in an unbounded wait.
(
  startup_up_rc=0
  TERM=xterm PATH="${PATH#"$RUN_ROOT/bin:"}" script -qec \
    "stty rows 60 cols 200; $GANG up startup-up -c startup-up -d /tmp -r startup-up" /dev/null \
    > "$RUN_ROOT/startup-up.out" 2>&1 || startup_up_rc=$?
  printf '%s\n' "$startup_up_rc" > "$RUN_ROOT/startup-up.status"
  tmux wait-for -S "$startup_up_attach_outcome"
  tmux wait-for -S "$startup_up_process_done"
  exit "$startup_up_rc"
) &
startup_up_process=$!
wait "$startup_up_attached_waiter"
tmux set-hook -gu client-attached
if [ "$(tmux show-options -gqv @test_startup_up_attached)" != yes ]; then
  wait "$startup_up_process_waiter"
  wait "$startup_up_process" 2>/dev/null || true
  fail "gang up exposes a positively observed first-run gate in its tmux client" \
    "synthetic client exited with status $(<"$RUN_ROOT/startup-up.status") before tmux observed client-attached: $(<"$RUN_ROOT/startup-up.out")"
  exit 1
fi
tmux set-option -gu @test_startup_up_attached
pass "gang up exposes a positively observed first-run gate in its tmux client"
contains "gang up parks its contract before exposing the gate" \
  "$("$GANG" status startup-up)" "spooled: 1"
excludes "gang up types nothing through the native gate" \
  "$(pane startup-up)" "You are startup-up in Gangline"
tmux set-hook -g after-rename-window \
  "if-shell -F '#{==:#{window_name},-startup-up-}' 'wait-for -S $startup_up_delivered' ''"
tmux wait-for "$startup_up_delivered" &
startup_up_delivered_waiter=$!
tmux wait-for -S "$startup_up_clear"
wait "$startup_up_delivered_waiter"
tmux set-hook -gu after-rename-window
# source-guard: producer@8e4fa6251628: the busy-glyph hook fires only after the hookless up path verifies and retires its nonce-addressed startup entry
contains "gang up delivers the parked contract after its attached prompt clears" \
  "$(pane startup-up)" "You are startup-up in Gangline"
excludes "gang up retires the verified startup spool entry" \
  "$("$GANG" status startup-up)" "spooled:"
# `script` may close its synthetic client on stdin EOF after the delivery; an
# already-detached client and one detached here are the same settled state. Its
# driver records status and signals completion only after `script` returns, so
# this barrier establishes process exit before the PID wait merely reaps it.
tmux detach-client -s "=$GANG_SESSION" 2>/dev/null || true
wait "$startup_up_process_waiter"
startup_up_rc=0
wait "$startup_up_process" || startup_up_rc=$?
if [ "$startup_up_rc" -ne 0 ]; then
  fail "gang up's synthetic attached client exits cleanly after detachment" \
    "driver exited with status $(<"$RUN_ROOT/startup-up.status"): $(<"$RUN_ROOT/startup-up.out")"
  exit 1
fi
"$GANG" drop startup-up >/dev/null

# A HEADLESS `up` MUST NOT ENTER THE LONG FOLLOW LOOP after its attach client
# has already failed. The tmux shim couples that failure to clearing the gate,
# so the old wait-until-composer shape would still terminate instead of hanging
# the suite; only the immediate, explicit attach refusal satisfies the check.
startup_up_headless_clear="test-startup-up-headless-clear-$$"
cat > "$RUN_ROOT/startup-up-headless.sh" <<SH
#!/bin/sh
printf 'FIRST_RUN_GATE\n'
tmux wait-for "$startup_up_headless_clear"
PS1='❯ ' exec bash --norc
SH
chmod +x "$RUN_ROOT/startup-up-headless.sh"
cat > "$RUN_ROOT/collars/startup-up-headless.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="'$RUN_ROOT/startup-up-headless.sh'"
GANG_OCCUPIED_REGEX='FIRST_RUN_GATE'
SH
mkdir -p "$RUN_ROOT/headless-up-bin"
real_tmux="$(command -v tmux)"
cat > "$RUN_ROOT/headless-up-bin/tmux" <<SH
#!/bin/sh
if [ "\${1:-}" = attach ]; then
  "$real_tmux" wait-for -S "$startup_up_headless_clear"
  exit 17
fi
exec "$real_tmux" "\$@"
SH
chmod +x "$RUN_ROOT/headless-up-bin/tmux"
if startup_up_headless_out="$(PATH="$RUN_ROOT/headless-up-bin:$PATH" \
    "$GANG" up startup-up-headless -c startup-up-headless -d /tmp \
    -r startup-up </dev/null 2>&1)"; then
  fail "gang up refuses when its startup-prompt client cannot attach" \
    "$startup_up_headless_out"
else
  pass "gang up refuses when its startup-prompt client cannot attach"
fi
contains "the failed attach is named before any long composer wait" \
  "$startup_up_headless_out" \
  "tmux could not attach a client to the startup prompt (status 17)"
contains "the headless refusal leaves the attributed contract inspectable" \
  "$("$GANG" status startup-up-headless)" "spooled: 1"
# Keep this intentionally undelivered startup entry out of the suite's shared
# teardown-archive accounting; the exact disposable window still exercises the
# ordinary archive path, just under its fixture-owned root.
GANG_ARCHIVE_DIR="$RUN_ROOT/headless-up-archive" \
  "$GANG" drop startup-up-headless >/dev/null

# A STABLE SCREEN IS NOT EVIDENCE OF A STARTUP PROMPT. A provider-error pane has
# no composer and no collar-owned occupied marker, so hitch must retain its
# fail-loud recovery instead of waiting forever on a screen it cannot name.
startup_unknown_hold="test-startup-unknown-hold-$$"
cat > "$RUN_ROOT/collars/startup-unknown.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'printf \"PROVIDER_ERROR\\n\"; tmux wait-for \"$startup_unknown_hold\"' fixture"
SH
if startup_unknown_out="$(GANG_BOOT_TIMEOUT=1 "$GANG" hitch startup-unknown \
    -c startup-unknown -d /tmp 2>&1)"; then
  fail "an unknown stable boot screen is not accepted as a startup gate" \
    "$startup_unknown_out"
else
  pass "an unknown stable boot screen is not accepted as a startup gate"
fi
contains "the unknown screen retains the fail-loud recovery" \
  "$startup_unknown_out" "showing something other than its input box"
excludes "the unknown screen is never reported as accepted" \
  "$startup_unknown_out" "queued startup contract"
excludes "the unknown screen receives no parked contract" \
  "$("$GANG" status startup-unknown)" "spooled:"
"$GANG" drop startup-unknown >/dev/null

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
# taller still. Size only this fixture lane; the prompt fixtures above retain
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

# 2.0 removed the pre-rename command name and the in-place option migration.
refuses "the removed cutoff command name is unknown" \
  "unknown command 'cutoff'" "$GANG" cutoff 90m
equal "and the refused alias declared no curfew" "no curfew declared" \
  "$("$GANG" curfew)"

legacy_curfew="$(( $(date +%s) + 600 )) $(( $(date +%s) - 60 ))"
tmux set-option -t "=$GANG_SESSION:" @gl_cutoff "$legacy_curfew"
tmux set-option -u -t "=$GANG_SESSION:" @gl_curfew
equal "a pre-rename team option is residue, not a declaration" \
  "no curfew declared" "$("$GANG" curfew)"
equal "and it is left where it was, unmigrated" "$legacy_curfew" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_cutoff)"
equal "and nothing was written into the current option" "" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_curfew)"
tmux set-option -u -t "=$GANG_SESSION:" @gl_cutoff

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
cat > "$RUN_ROOT/collars/unchecked-model.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_MODEL_OPT='--model'
SH
if unchecked_model_out="$("$GANG" hitch unchecked-model -c unchecked-model \
    -d /tmp -m exact 2>&1)"; then
  fail "a model option with no collar validation cannot open a window" \
    "hitch accepted an unchecked model"
else
  pass "a model option with no collar validation cannot open a window"
fi
contains "the unchecked-model refusal names both supported validation contracts" \
  "$unchecked_model_out" "neither collar_models nor collar_model_check"
equal "an unverifiable model leaves no window behind" "" \
  "$(window_id unchecked-model)"

cat > "$RUN_ROOT/collars/catalog-model.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/unchecked-model.sh"
collar_models() { printf 'exact\nneighbor\n'; }
SH
if catalog_bad_out="$("$GANG" hitch catalog-bad -c catalog-model \
    -d /tmp -m invented 2>&1)"; then
  fail "a model absent from a complete native catalog is refused" \
    "hitch accepted an absent model"
else
  pass "a model absent from a complete native catalog is refused"
fi
contains "the absent-model refusal points to deterministic discovery" \
  "$catalog_bad_out" "gang models -c catalog-model"
equal "a model refused by the catalog leaves no window behind" "" \
  "$(window_id catalog-bad)"
"$HITCH" catalog-ok -c catalog-model -d /tmp -m exact >/dev/null
contains "an exact catalog model reaches the native launch option" \
  "$(tmux display-message -p -t "$(window_id catalog-ok)" '#{pane_start_command}')" \
  "--model exact"
"$GANG" drop catalog-ok >/dev/null

cat > "$RUN_ROOT/collars/checked-model.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/unchecked-model.sh"
collar_model_check() {
  case "\$1" in
    exact) return 0 ;;
    invented) printf 'fixture rejected it'; return 1 ;;
    *) printf 'fixture could not read recognition'; return 2 ;;
  esac
}
SH
refuses "a native recognition checker can reject a model without enumerating" \
  "fixture rejected it" "$GANG" hitch checked-bad -c checked-model \
  -d /tmp -m invented
equal "a recognition rejection leaves no window behind" "" \
  "$(window_id checked-bad)"
refuses "an unreadable recognition result stays unknown" \
  "could not determine whether model 'unreadable' is recognized" \
  "$GANG" hitch checked-unknown -c checked-model -d /tmp -m unreadable
equal "an unknown recognition result leaves no window behind" "" \
  "$(window_id checked-unknown)"
"$HITCH" checked-ok -c checked-model -d /tmp -m exact >/dev/null
"$GANG" drop checked-ok >/dev/null

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
