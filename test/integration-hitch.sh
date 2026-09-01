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
#
# A SHIM THAT SHADOWS THE SUITE'S CLOCK OWES THE SAME LEDGER. This one is on
# PATH ahead of it, so without the recording line every budget spent under a
# boot barrier would be unmeasurable — the exact silence test/integration.sh
# describes where it writes that ledger.
# A BARRIER THAT NEEDS THE TMUX SERVER CANNOT REPORT ON THE TMUX SERVER, and
# this one waits on a fixture running inside that very server. It used to block
# on `tmux wait-for`, a client call with no timeout, so a server that stopped
# answering could only make the run quiet — never red. Observed 2026-08-24 on
# the sibling barrier in test/integration-substrate.sh: the waiter and the
# fixture's signaller both sat alive and blocked, and the run held the gate lock
# for hours. The byte now travels down a pipe this run owns, so neither side is
# a tmux client. The shim cannot assert, so the caller reads the marker; what it
# guarantees is that a stalled fixture stalls on its own pipe rather than on a
# shared server.
prompt_boot_barrier() { # $1 = bin dir, $2 = seen marker, $3 = fixture pipe path
  mkdir -p "$1"
  rm -f "$3"
  mkfifo "$3"
  cat > "$1/sleep" <<SH
#!/bin/sh
[ -z "\${GANG_TEST_CLOCK_LEDGER:-}" ] || printf '%s\n' "\$1" >> "\$GANG_TEST_CLOCK_LEDGER"
if [ ! -e '$2' ]; then
  : > '$2'
  head -c 1 < '$3' > /dev/null
fi
exit 0
SH
  chmod +x "$1/sleep"
}
# THE BUDGET IS NOT WHAT THE TWO ANSWERED-PROMPT FIXTURES MEASURE. Observations
# are how the gate wait is counted, and the suite's sleep returns immediately,
# so the default budget would be spent here in the time it takes to read a pane
# sixty times — racing the operator's answer below, with load deciding the
# verdict instead of behaviour. Both raise it out of the way, per invocation so
# nothing later inherits it, and the budget gets a fixture of its own.
cat > "$RUN_ROOT/collars/dialog-observe-boot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_LAUNCH="env DIALOG_VARIANT=known DIALOG_KEY_LOG='$RUN_ROOT/dialog-observe-boot.keys' DIALOG_READY='$RUN_ROOT/dialog-observe-boot-ready' '$RUN_ROOT/dialog-fixture.py'"
SH
: > "$RUN_ROOT/dialog-observe-boot.keys"
prompt_boot_barrier "$RUN_ROOT/observe-boot-bin" \
  "$RUN_ROOT/observe-boot-seen" "$RUN_ROOT/dialog-observe-boot-ready"
observe_boot_pipe="$RUN_ROOT/dialog-observe-boot.out"
mkfifo "$observe_boot_pipe"
exec 8<>"$observe_boot_pipe"
PATH="$RUN_ROOT/observe-boot-bin:$PATH" GANG_GATE_LOOKS=1000000 \
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
GANG_LAUNCH="env DIALOG_VARIANT=trust DIALOG_KEY_LOG='$RUN_ROOT/dialog-trust-boot.keys' DIALOG_READY='$RUN_ROOT/dialog-trust-boot-ready' '$RUN_ROOT/dialog-fixture.py'"
SH
: > "$RUN_ROOT/dialog-trust-boot.keys"
prompt_boot_barrier "$RUN_ROOT/trust-boot-bin" \
  "$RUN_ROOT/trust-boot-seen" "$RUN_ROOT/dialog-trust-boot-ready"
trust_boot_pipe="$RUN_ROOT/dialog-trust-boot.out"
mkfifo "$trust_boot_pipe"
exec 8<>"$trust_boot_pipe"
PATH="$RUN_ROOT/trust-boot-bin:$PATH" GANG_GATE_LOOKS=1000000 \
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

# A GATE NOBODY ANSWERS. Hitch parks the contract and keeps observing, and the
# budget is what stops that observation from holding the caller's terminal for
# the rest of the run — the caller is often another agent, which cannot answer a
# native prompt at all. Nothing is answered here, so the fixture's key log is
# also the proof that the exit was a give-up rather than a delivery.
cat > "$RUN_ROOT/collars/dialog-gate-budget.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_LAUNCH="env DIALOG_VARIANT=known DIALOG_KEY_LOG='$RUN_ROOT/dialog-gate-budget.keys' DIALOG_READY='$RUN_ROOT/dialog-gate-budget-ready' '$RUN_ROOT/dialog-fixture.py'"
SH
: > "$RUN_ROOT/dialog-gate-budget.keys"
prompt_boot_barrier "$RUN_ROOT/gate-budget-bin" \
  "$RUN_ROOT/gate-budget-seen" "$RUN_ROOT/dialog-gate-budget-ready"
# THREE LOOKS RATHER THAN ONE, SO THE PACE BETWEEN THEM IS MEASURABLE. The
# give-up is asserted exactly as before and at the same budget boundary; what
# a budget of one could not show is the second between observations, because
# one look gives up before ever reaching it. Under the counted clock the two
# waits an exhausted three-look budget owes are a readable artifact, and the
# spin this loop would become without them — sixty pane reads as fast as the
# box allows — turns this check red instead of finishing green and instantly.
gate_budget_ledger="$(clock_ledger gate-budget)"
gate_budget_rc=0
gate_budget_out="$(PATH="$RUN_ROOT/gate-budget-bin:$PATH" GANG_GATE_LOOKS=3 \
  GANG_TEST_CLOCK_LEDGER="$gate_budget_ledger" \
  "$GANG" hitch gate-budget -c dialog-gate-budget -d /tmp 2>&1)" || gate_budget_rc=$?
equal "an unanswered first-run prompt ends the hitch on its own status" \
  4 "$gate_budget_rc"
equal "the gate budget is spent one second per unanswered observation" \
  2 "$(clock_naps "$gate_budget_ledger" 1)"
contains "and the refusal says the contract was not delivered" \
  "$gate_budget_out" "startup contract was NOT delivered"
contains "and names the recovery that works from there" \
  "$gate_budget_out" "gang drop gate-budget"
equal "and still nothing answered the prompt" "" \
  "$(<"$RUN_ROOT/dialog-gate-budget.keys")"
equal "the attributed contract is still parked where roster shows it" "1" \
  "$("$GANG" roster --porcelain 2>/dev/null | awk -F '\t' '$1 == "gate-budget" { print $4 }')"
# ITS OWN ARCHIVE ROOT. This is the one fixture that deliberately leaves a
# message parked, so its drop is the one that writes a teardown archive — and
# the spool part counts the directories under the shared root exactly.
GANG_ARCHIVE_DIR="$RUN_ROOT/gate-budget-archive" "$GANG" drop gate-budget >/dev/null

# FOUR BARRIERS THAT ALL LEANED ON THE TMUX SERVER. This case holds the pane's
# shell and gang's own observer against each other while a first-run modal is on
# screen, and it learned all four of those events through `tmux wait-for`:
# client calls into the very server whose responsiveness decides whether the
# modal can be read at all. Observed 2026-08-24 on this fixture, the observed
# and painted ends sat alive and blocked together while the run held the gate
# lock behind them; `tmux wait-for` has no timeout, so that could only ever be
# quiet, never red. Each event now travels down a pipe this run owns, so no side
# of any of them is a tmux client.
#
# The two ends this shell writes are held open read-write below, so a peer that
# died still leaves the write returning rather than making this shell the next
# thing that hangs. The end this shell reads is deliberately not held open: a
# writer that opened and closed must arrive here as end-of-file, which is what
# the asserted byte is for.
modal_observed="$RUN_ROOT/boot-modal-observed"
modal_painted="$RUN_ROOT/boot-modal-painted"
modal_clear="$RUN_ROOT/boot-modal-clear"
modal_probe_release="$RUN_ROOT/boot-modal-probe-release"
mkfifo "$modal_observed" "$modal_painted" "$modal_clear" "$modal_probe_release"
cat > "$RUN_ROOT/collars/boot-modal.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'printf \"FIRST_RUN_MODAL\\n\"; printf x > \"$modal_painted\"; head -c 1 < \"$modal_clear\" > /dev/null; PS1=\"❯ \" exec bash --norc' fixture"
GANG_OCCUPIED_REGEX='FIRST_RUN_MODAL'
_gl_modal_real="\$(declare -f collar_input)"
eval "modal_real_input \${_gl_modal_real#collar_input}"
collar_input() {
  if [ -e "$RUN_ROOT/boot-modal-blocked" ]; then
    if [ -e "$RUN_ROOT/boot-modal-seen" ]; then
      printf x > "$modal_observed"
      head -c 1 < "$modal_probe_release" > /dev/null
    else
      head -c 1 < "$modal_painted" > /dev/null
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
# OPENED AFTER THE LAUNCH so the observer under test does not inherit either
# end and the only holders are this shell and the peer each one is for.
exec 6<>"$modal_clear"
exec 7<>"$modal_probe_release"
# A WRITER THAT OPENED AND CLOSED WITHOUT WRITING is not an observer that
# reached a blocked modal, and under set -e an empty read would end the run
# instead of naming what happened.
modal_signal=""
modal_read_rc=0
IFS= read -r -n 1 modal_signal < "$modal_observed" || modal_read_rc=$?
equal "the boot observer announces the modal it is blocked on" \
  "0 x" "$modal_read_rc $modal_signal"
contains "hitch reports a first-run modal before it is cleared" \
  "$(<"$modal_output")" "answer it with 'gang attach'"
rm -f -- "$RUN_ROOT/boot-modal-blocked"
printf x >&6
printf x >&7
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
exec 6>&- 7>&-
"$GANG" drop boot-modal >/dev/null

# A FIRST-RUN GATE THAT OUTLIVES HITCH'S BOOT BUDGET has no turn and therefore
# no Stop. The old assertion required hitch to fail and prescribe a second
# manual `gang send`; that behavior was wrong because it dropped a fully formed
# startup envelope. Hitch now parks it, remains the foreground owner, and keeps
# observing the tty. This collar deliberately has no hook, proving that a Codex
# operator choosing "continue without hooks" cannot invalidate the promise.
# THE WAITER RUNS INSIDE A PANE OF THE SERVER IT IS CALLING, which is the one
# thing every barrier that has actually wedged here has in common: a fixture
# shell in a pane, blocking on a `tmux wait-for` against the very server that
# runs that pane. Driven 2026-08-24, this channel was signalled by a test shell
# that had already moved on while this waiter sat blocked for minutes, twice, on
# two different trees. A pipe this run owns takes the server out of it.
startup_gate_clear="$RUN_ROOT/startup-gate-clear"
mkfifo "$startup_gate_clear"
cat > "$RUN_ROOT/startup-gate.sh" <<SH
#!/bin/sh
printf 'FIRST_RUN_GATE\n'
head -c 1 < '$startup_gate_clear' > /dev/null
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
# HELD OPEN FIRST, so a fixture that died leaves this write returning rather
# than making the suite the next thing that hangs.
exec 6<>"$startup_gate_clear"
printf x >&6
exec 6>&-
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
# The same pane-side waiter, and the same pipe. This fixture blocks in a pane
# of the server it calls exactly as the one above does.
startup_second_clear="$RUN_ROOT/startup-second-clear"
mkfifo "$startup_second_clear"
rm -f "$RUN_ROOT/startup-second-ready"
mkfifo "$RUN_ROOT/startup-second-ready"
cat > "$RUN_ROOT/startup-second.sh" <<SH
#!/bin/sh
printf 'FIRST_RUN_GATE\n'
head -c 1 < '$startup_second_clear' > /dev/null
exec env DIALOG_VARIANT=trust \
  DIALOG_KEY_LOG='$RUN_ROOT/startup-second.keys' \
  DIALOG_READY='$RUN_ROOT/startup-second-ready' \
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
exec 6<>"$startup_second_clear"
printf x >&6
exec 6>&-
# The same run-owned pipe, and the byte is asserted rather than assumed: a
# writer that opened and closed without writing is not a fixture that reached
# its signal, and under set -e an empty read would end the run instead of
# naming what happened.
startup_second_signal=""
startup_second_read_rc=0
IFS= read -r -n 1 startup_second_signal < "$RUN_ROOT/startup-second-ready" \
  || startup_second_read_rc=$?
equal "the second startup fixture announced its own readiness" \
  "0 x" "$startup_second_read_rc $startup_second_signal"
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
# THE THIRD PANE-SIDE WAITER, AND THE ONE THAT PROVED THE CEILING WORKS. On
# 2026-08-24 this channel was signalled by the shell below and its waiter — a
# fixture inside a pane — never woke, so the composer never appeared, the window
# was never renamed, and the delivery barrier downstream of it expired. The
# ceiling turned four silent hours into a named refusal in 215 seconds; this
# turns the cause into a pipe.
startup_up_clear="$RUN_ROOT/startup-up-clear"
mkfifo "$startup_up_clear"
startup_up_attach_outcome="test-startup-up-attach-outcome-$$"
startup_up_delivered="test-startup-up-delivered-$$"
# THE DRIVER'S EXIT IS READ OFF A CHANNEL, NOT WAITED FOR ON ITS PID. Both the
# old tmux channel and the PID were released by the same subshell after `script`
# returned, so a client that attached and never gave the pty back stranded the
# mandatory suite with no verdict at all. The driver reports its own status into
# this pipe instead, and the read below carries an outer ceiling: completion
# arrives as bytes, expiration arrives as the absence of them, and the two are
# different named outcomes rather than the same silence.
#
# NO DELAY IS EVIDENCE HERE. The pass is the status that has already arrived, so
# a healthy run spends nothing; the ceiling is spent only by a defect, and what
# it buys is a failure that names its cause instead of a suite that never ends.
#
#   detach to driver exit, this run   the instrument line printed below
#   test ceiling (this read)          30s
#   push CI ceiling for the whole     15 minutes (.github/workflows/shell.yml)
#   suite, which this must not reach
#
# The first row is measured on every run rather than written down once here. A
# margin recorded in a comment stops being true without saying so, and the
# number that decides whether this ceiling is roomy is the one this host just
# produced — so the run prints it and a reader compares the two columns.
startup_up_ceiling=30
startup_up_exit_pipe="$RUN_ROOT/startup-up.exit"
mkfifo "$startup_up_exit_pipe"
exec 9<>"$startup_up_exit_pipe"
cat > "$RUN_ROOT/startup-up.sh" <<SH
#!/bin/sh
printf 'FIRST_RUN_GATE\n'
head -c 1 < '$startup_up_clear' > /dev/null
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
# a missing client cannot strand the suite in an unbounded wait. The status
# reaches the pipe before the barrier is released, so a barrier released by the
# driver's own exit is always released over a status already readable.
(
  startup_up_rc=0
  TERM=xterm PATH="${PATH#"$RUN_ROOT/bin:"}" script -qec \
    "stty rows 60 cols 200; $GANG up startup-up -c startup-up -d /tmp -r startup-up" /dev/null \
    > "$RUN_ROOT/startup-up.out" 2>&1 || startup_up_rc=$?
  printf '%s\n' "$startup_up_rc" > "$RUN_ROOT/startup-up.status"
  printf 'exit %s\n' "$startup_up_rc" >&9
  tmux wait-for -S "$startup_up_attach_outcome"
  exit "$startup_up_rc"
) &
startup_up_process=$!
wait "$startup_up_attached_waiter"
tmux set-hook -gu client-attached
if [ "$(tmux show-options -gqv @test_startup_up_attached)" != yes ]; then
  exec 9>&-
  kill "$startup_up_process" 2>/dev/null || true
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
exec 6<>"$startup_up_clear"
printf x >&6
exec 6>&-
wait "$startup_up_delivered_waiter"
tmux set-hook -gu after-rename-window
# source-guard: producer@8e4fa6251628: the busy-glyph hook fires only after the hookless up path verifies and retires its nonce-addressed startup entry
contains "gang up delivers the parked contract after its attached prompt clears" \
  "$(pane startup-up)" "You are startup-up in Gangline"
excludes "gang up retires the verified startup spool entry" \
  "$("$GANG" status startup-up)" "spooled:"
# `script` may close its synthetic client on stdin EOF after the delivery; an
# already-detached client and one detached here are the same settled state.
#
# A CLEAN EXIT IS NOW AN ASSERTION RATHER THAN A PRECONDITION. The old shape had
# no pass arm, so the one healthy outcome was neither counted nor visible, and
# the unhealthy one was a suite that never returned. The bounded read gives both
# a name: 'exit 0' is what a driver that gave the pty back reports, and anything
# else — a nonzero status, or the ceiling above elapsing with the driver still
# holding the terminal — is the actual value of the same counted check.
#
# The PID is deliberately not waited on afterwards. The status has already been
# read, so a wait could only add back the unbounded stall this replaced; the
# remaining work in that subshell is one tmux signal and an exit.
tmux detach-client -s "=$GANG_SESSION" 2>/dev/null || true
startup_up_outcome=""
startup_up_spent="$SECONDS"
read -r -t "$startup_up_ceiling" startup_up_outcome <&9 \
  || startup_up_outcome="reported no exit within ${startup_up_ceiling}s and still holds the terminal"
startup_up_spent=$((SECONDS - startup_up_spent))
exec 9>&-
printf 'instrument startup-up-driver-exit=%ss ceiling=%ss\n' \
  "$startup_up_spent" "$startup_up_ceiling"
equal "gang up's synthetic attached client exits cleanly after detachment" \
  "exit 0" "$startup_up_outcome"
if [ "$startup_up_outcome" != "exit 0" ]; then
  printf '       driver output: %s\n' "$(<"$RUN_ROOT/startup-up.out")"
  kill "$startup_up_process" 2>/dev/null || true
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
# THE READINESS BUDGET IS SECONDS, AND THIS IS WHERE THEY ARE COUNTED. The
# screen never resolves, so the wait runs to exhaustion and owes one wait per
# second of budget — three here rather than one, so the count is a pace and not
# a single tick. The refusal below is unchanged and fires at the same
# exhaustion; what the ledger adds is that the budget was consumed to get there
# rather than spun through, which under a clock that only returns instantly no
# assertion in this suite could previously tell apart.
startup_unknown_ledger="$(clock_ledger startup-unknown)"
if startup_unknown_out="$(GANG_BOOT_TIMEOUT=3 \
    GANG_TEST_CLOCK_LEDGER="$startup_unknown_ledger" "$GANG" hitch startup-unknown \
    -c startup-unknown -d /tmp 2>&1)"; then
  fail "an unknown stable boot screen is not accepted as a startup gate" \
    "$startup_unknown_out"
else
  pass "an unknown stable boot screen is not accepted as a startup gate"
fi
equal "the readiness budget is spent one second per unresolved observation" \
  3 "$(clock_naps "$startup_unknown_ledger" 1)"
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
# HITCHED AFTER IT, so it sits later in the window list. Roster used to end at
# the first agent it could not observe, and every agent after that one was
# missing from a listing that is the documented check before a teardown — a
# smaller team and a truncated one read the same from outside.
"$HITCH" zz-after-broken -c bash -d /tmp >/dev/null
if broken_roster="$("$GANG" roster 2>&1)"; then
  fail "roster fails when an agent row cannot be observed" "roster exited successfully"
else
  broken_roster_rc=$?
  equal "roster propagates the observation failure" "1" "$broken_roster_rc"
fi
contains "roster names the collar whose observation failed" \
  "$broken_roster" "broken-observer.sh"
contains "the unobservable agent still gets a row" \
  "$broken_roster" "broken-observer"
contains "and that row says which unknown this is" \
  "$broken_roster" "state-unreadable"
contains "and every agent after it is still listed" \
  "$broken_roster" "zz-after-broken"
broken_porcelain="$("$GANG" roster --porcelain 2>/dev/null)" || :
equal "the porcelain row for an unreadable state is unknown" "unknown" \
  "$(printf '%s\n' "$broken_porcelain" | awk -F '\t' '$1 == "broken-observer" { print $3 }')"
equal "and the porcelain listing reaches the agents after it too" "bash" \
  "$(printf '%s\n' "$broken_porcelain" | awk -F '\t' '$1 == "zz-after-broken" { print $2 }')"
"$GANG" drop zz-after-broken >/dev/null
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
bad_file_windows_before="$(window_names)"
if bad_file_out="$(env -u GANG_BOOT_TIMEOUT GANG_CONFIG_DIR="$CONFIG_CASES/bad-file-value" \
  "$GANG" hitch config-bad-file -c bash -d /tmp 2>&1)"; then
  fail "a bad configured value is blamed on its file line" \
    "hitch unexpectedly succeeded"
else
  contains "a bad configured value is blamed on its file line" \
    "$bad_file_out" "from $CONFIG_CASES/bad-file-value/config line 1"
fi
equal "a bad configured startup budget opens no window" \
  "$bad_file_windows_before" "$(window_names)"
bad_env_windows_before="$(window_names)"
if bad_env_out="$(GANG_BOOT_TIMEOUT=abc GANG_CONFIG_DIR="$CONFIG_CASES/bad-env-value" \
  "$GANG" hitch config-bad-env -c bash -d /tmp 2>&1)"; then
  fail "a bad environment value is blamed on the environment" \
    "hitch unexpectedly succeeded"
else
  contains "a bad environment value is blamed on the environment" \
    "$bad_env_out" "from the environment"
fi
equal "a bad environment startup budget opens no window" \
  "$bad_env_windows_before" "$(window_names)"

bad_gate_windows_before="$(window_names)"
refuses "a bad gate-observation budget is refused" \
  "GANG_GATE_LOOKS must be a whole number of observations" \
  env GANG_GATE_LOOKS=many "$GANG" hitch config-bad-gate -c bash -d /tmp
equal "a bad gate-observation budget opens no window" \
  "$bad_gate_windows_before" "$(window_names)"

# THE SPOOL ROOT IS A HITCH PRECONDITION, not a post-launch discovery. Every
# window receives a spool identity before hitch can succeed, and the configured
# root can be rejected without opening that window. Before this preflight the
# same refusal left a live, registered agent behind with no spool identity.
bad_lock_root="$RUN_ROOT/hitch-lock-not-a-directory"
: > "$bad_lock_root"
bad_lock_windows_before="$(window_names)"
refuses "a lock root that cannot hold a spool is refused" \
  "exists and is not a directory" \
  env GANG_LOCK_DIR="$bad_lock_root" \
    "$GANG" hitch config-bad-lock-root -c bash -d /tmp
equal "an unusable spool root opens no window" \
  "$bad_lock_windows_before" "$(window_names)"
# Keep the mutant that opens the window from contaminating later checks.
bad_lock_leak="$(window_id config-bad-lock-root)" || bad_lock_leak=""
[ -z "$bad_lock_leak" ] || tmux kill-window -t "$bad_lock_leak"

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
  fail "the trailing-newline witness leaves the startup outcome unknown" \
    "hitch unexpectedly succeeded"
else
  case "$trailing_out" in
    *"submission outcome unknown"*)
      pass "the trailing-newline witness leaves the startup outcome unknown" ;;
    *) fail "the trailing-newline witness leaves the startup outcome unknown" "$trailing_out" ;;
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
if [[ "$curfew_pair" =~ ^v2:[0-9]+:[0-9]+:[0-9]+:[0-9]+$ ]]; then
  pass "the curfew stores one wall label and one monotonic duration"
else
  fail "the curfew stores one wall label and one monotonic duration" \
    "got [$curfew_pair]"
fi

# A WALL-CLOCK STEP CANNOT SPEND A MONOTONIC TEAM BUDGET. The wall target is
# retained only as the local-time label; the declaration and deadline readings
# decide the yellow edge in the shared elapsed domain.
curfew_clock_shim="$RUN_ROOT/curfew-clock"
curfew_date_bin="$RUN_ROOT/curfew-date-bin"
mkdir -p "$curfew_date_bin"
cat > "$curfew_clock_shim" <<'SH'
#!/bin/sh
now="${GANG_TEST_CLOCK_NOW_NS:?}"
case "${1:-}" in
  now) [ "$#" -eq 1 ] || exit 2; printf '%s\n' "$now" ;;
  elapsed)
    [ "$#" -eq 3 ] || exit 2
    [ "$now" -ge "$2" ] || exit 2
    [ $(( now - $2 )) -ge "$3" ]
    ;;
  *) exit 2 ;;
esac
SH
chmod +x "$curfew_clock_shim"
{
  printf '#!/bin/sh\n'
  printf 'REAL=%q\n' "$(command -v date)"
  cat <<'SH'
if [ "${1:-}" = +%s ]; then
  printf '%s\n' 9999999999
  exit 0
fi
exec "$REAL" "$@"
SH
} > "$curfew_date_bin/date"
chmod +x "$curfew_date_bin/date"
curfew_wall_declared=1800000000
curfew_wall_at=$((curfew_wall_declared + 100))
curfew_mono_declared=40000000000
curfew_mono_at=$((curfew_mono_declared + 100000000000))
tmux set-option -t "=$GANG_SESSION:" @gl_curfew \
  "v2:$curfew_wall_at:$curfew_wall_declared:$curfew_mono_at:$curfew_mono_declared"
curfew_step_light="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  PATH="$curfew_date_bin:$PATH" GANG_TEST_CLOCK="$curfew_clock_shim" \
  GANG_TEST_CLOCK_NOW_NS=100000000000 \
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" \
  "$GANG" hook)"
contains "a wall-clock step cannot spend the monotonic curfew" \
  "$curfew_step_light" "Yellow time light"

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
# source-guard: whole-surface@b66eb60af741: the claim is the SHAPE of the attribution rather than which body carries it — every producer of this string is a send gang could not observe a window for, which is exactly what is asserted
contains "the delivered message is attributed" "$alpha_pane" "[gang:self-declared:tester#"

"$HITCH" inside-target -c bash -d /tmp >/dev/null
# shellcheck disable=SC2154  # set in test/integration-substrate.sh
printf 'MARK_INSIDE_SENDER' | TMUX_PANE="$alpha_tmux_pane" \
  "$GANG" send --to inside-target --stdin >/dev/null
contains "a sender in a glyphed window is attributed by its bare name" \
  "$(pane inside-target)" "[gang:alpha#"
"$GANG" drop inside-target >/dev/null

# A CALLER WITH NO PANE IDENTITY CAN NAME ITSELF ANYTHING. A harness's sandboxed
# command surface reaches the socket and the executable with the tmux
# environment stripped, so `self_window` finds nothing, `--from` becomes
# mandatory, and the name it supplies used to reach the receiver in the same
# shape as one Gangline had watched a pane produce. Nothing here proves who is
# calling — authentication is banned in this repo and the claim still stands as
# claimed — but the envelope now says which of the two it is carrying.
#
# The claimed name is `alpha`, a live agent whose OBSERVED envelope the
# assertion directly above this one pins, so the two forms are compared against
# each other rather than against a description of one. The target is fresh, so
# nothing else on its screen could satisfy either assertion.
"$HITCH" declared-target -c bash -d /tmp >/dev/null
declared_out="$(printf 'MARK_DECLARED_SENDER' | env -u TMUX -u TMUX_PANE \
  "$GANG" send --to declared-target --from alpha --stdin)"
declared_pane="$(pane declared-target)"
# source-guard: producer@0173a5fd7f83: the send three lines above is the sole producer of this literal; no other sender, spool entry or fixture writes it
contains "a sandboxed caller's message still arrives" \
  "$declared_pane" "MARK_DECLARED_SENDER"
# source-guard: producer@0f340f61a849: declared-target was hitched four lines above and this send is the only delivery ever made into it, so its pane carries one envelope
contains "and its opening tag marks the sender as self-declared" \
  "$declared_pane" "[gang:self-declared:alpha#"
# source-guard: producer@c0c4b6d50709: the same single delivery into a freshly hitched window; the closing tag is the other end of that one envelope
contains "as does its closing tag, so either end settles it" \
  "$declared_pane" "[/gang:self-declared:alpha#"
excludes "and it cannot be read as the identity gang observes in alpha's window" \
  "$declared_pane" "[gang:alpha#"
contains "the sender is told which identity went on the wire" \
  "$declared_out" "[gang:self-declared:alpha]"
"$GANG" drop declared-target >/dev/null

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
  "$("$GANG" status alpha | sed -n '1p')"
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
# WHERE THE HARNESS IS CHOSEN, and the two halves fail differently. `gang drop`
# naming either afterwards is the report arriving after the choice it should
# have informed — and one warning covering both said something false about each.
# This collar inherits a resume launch and declares no witness, so it is the
# launch-only half: the relaunch works, on an id nothing here will ever learn.
contains "a collar that relaunches onto an id it never learns warns about the id" \
  "$(<"$RUN_ROOT/choiceboth.err")" "witnesses no harness session identity"
excludes "and not about a resume launch it does declare" \
  "$(<"$RUN_ROOT/choiceboth.err")" "declares no resume launch"
"$GANG" drop choiceboth >/dev/null
"$GANG" drop choicemodel >/dev/null
cat > "$RUN_ROOT/collars/resumable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_RESUME_LAUNCH="PS1='❯ ' bash --norc {{session_id}}"
collar_session_id() { printf 'fixture-session-id'; }
SH
"$HITCH" resumable-agent -c resumable -d /tmp >/dev/null
excludes "and a collar declaring both halves draws no warning at all" \
  "$(<"$RUN_ROOT/hitch-stderr")" "declares no resume launch"
excludes "in either of its wordings" \
  "$(<"$RUN_ROOT/hitch-stderr")" "witnesses no harness session identity"
"$GANG" drop resumable-agent >/dev/null
# THE HALF THAT WITNESSES AND CANNOT RELAUNCH. Only the hook path stamps
# @gl_session_id, and it stamps through a collar_session_id — so a collar with
# the witness and no launch DOES get an id, and drop used to quote it as a
# relaunch command that hitch then refuses: the operator's one recorded way back
# printed at the moment the agent ends, and not runnable. Stamped directly here
# because what is under test is what drop does with a stamp, not how one arrives.
cat > "$RUN_ROOT/collars/witness-only.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_session_id() { printf 'witness-only-id'; }
SH
"$GANG" hitch witness-only -c witness-only -d /tmp \
  >/dev/null 2> "$RUN_ROOT/witness-only.err"
contains "a collar that witnesses an id it cannot relaunch onto still warns" \
  "$(<"$RUN_ROOT/witness-only.err")" "declares no resume launch"
excludes "without claiming gang will never learn that id" \
  "$(<"$RUN_ROOT/witness-only.err")" "will never learn"
tmux set-option -w -t "$(window_id witness-only)" @gl_session_id witness-only-id
witness_only_drop="$("$GANG" drop witness-only)"
contains "and drop prints the id it really has" \
  "$witness_only_drop" "session id: witness-only-id"
excludes "without quoting a relaunch command hitch refuses" \
  "$witness_only_drop" "--resume witness-only-id"
contains "saying instead what that id is" \
  "$witness_only_drop" "not a way back into it"
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

# WHICH CONTEXT LIGHTS AN AGENT GETS IS A PER-MODEL QUESTION THE COLLAR ANSWERS.
# One team setting sized in tokens cannot fit a team whose harnesses report
# windows differing several-fold: a red threshold that leaves useful headroom in
# the larger window cannot fire in the smaller one at all. So the built-in
# setting is `collar`, the collar answers for the hitched model, and an explicit
# spec — team-wide or on one agent — overrides that answer.
lights_stamp() { # $1 agent -> the thresholds hitch registered on its window
  tmux show-options -wqv -t "$(window_id "$1")" @gl_context_lights 2>/dev/null || :
}
cat > "$RUN_ROOT/collars/model-lights.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/catalog-model.sh"
collar_context_lights() {
  case "\$1" in
    exact) printf '40%%,70%%\n' ;;
    neighbor) printf '250000,400000\n' ;;
    *) return 1 ;;
  esac
}
SH
"$HITCH" lightsdefault -c model-lights -d /tmp -m exact >/dev/null
equal "with nothing configured a collar's own default arms the lights" \
  "40%,70%" "$(lights_stamp lightsdefault)"
"$HITCH" lightsother -c model-lights -d /tmp -m neighbor >/dev/null
equal "and a second model on the same collar gets its own thresholds" \
  "250000,400000" "$(lights_stamp lightsother)"
"$HITCH" lightsnone -c catalog-model -d /tmp -m exact >/dev/null
equal "a collar shipping no default leaves the lights off, as before defaults" \
  "" "$(lights_stamp lightsnone)"
# AND IT IS OFF BECAUSE THIS LAUNCH SAID SO, not because nothing was written.
# `--resume` respawns a registered window and window options outlive a respawn,
# so a stamp carried over from an earlier launch would keep lights armed over a
# launch that wired no native source for them to read.
equal "an off hitch still writes the stamp, so no earlier one can survive it" \
  1 "$(tmux show-options -w -t "$(window_id lightsnone)" \
    | grep -c '^@gl_context_lights')"
GANG_CONTEXT_LIGHTS=90000,120000 "$HITCH" lightsteam -c model-lights \
  -d /tmp -m exact >/dev/null
equal "a team-wide spec overrides the collar's default" \
  "90000,120000" "$(lights_stamp lightsteam)"
GANG_CONTEXT_LIGHTS=90000,120000 "$HITCH" lightsagent -c model-lights \
  -d /tmp -m exact --lights 30%,60% >/dev/null
equal "and one agent's --lights overrides the team's spec" \
  "30%,60%" "$(lights_stamp lightsagent)"
GANG_CONTEXT_LIGHTS=90000,120000 "$HITCH" lightsoff -c model-lights \
  -d /tmp -m exact -l off >/dev/null
equal "--lights off takes one agent out of a team that lights the rest" \
  "" "$(lights_stamp lightsoff)"
"$HITCH" lightsask -c model-lights -d /tmp -m exact -l collar >/dev/null
equal "--lights collar asks the collar explicitly" \
  "40%,70%" "$(lights_stamp lightsask)"
for lights_agent in lightsdefault lightsother lightsnone lightsteam \
  lightsagent lightsoff lightsask; do
  "$GANG" drop "$lights_agent" >/dev/null
done

# A SPEC IS REFUSED UNDER THE NAME OF WHOEVER WROTE IT. The same grammar now
# arrives from three places, and a collar's broken default reported as the
# operator's GANG_CONTEXT_LIGHTS sends them to edit a setting they never wrote.
cat > "$RUN_ROOT/collars/bad-lights.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/catalog-model.sh"
collar_context_lights() { printf 'oops\n'; }
SH
refuses "a collar's malformed default is refused under the collar's name" \
  "collar 'bad-lights' context-light default for model 'exact'" \
  "$GANG" hitch lightsbad -c bad-lights -d /tmp -m exact
equal "and a refused collar default leaves no window behind" "" \
  "$(window_id lightsbad)"
refuses "a malformed --lights is refused under the flag's name" \
  "hitch --lights must increase from yellow to red" \
  "$GANG" hitch lightsflagbad -c model-lights -d /tmp -m exact -l 90,10
equal "and a refused --lights leaves no window behind" "" \
  "$(window_id lightsflagbad)"

# A DEFAULT NEVER ARMS A LIGHT ITS OWN COLLAR CANNOT READ. Nothing asked for
# this light, so a hook reporting the reading it could not take on every turn
# of every agent would be noise the operator never chose. An explicitly
# configured threshold still arms and still reports, because that was an ask.
cat > "$RUN_ROOT/collars/blind-lights.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/model-lights.sh"
unset -f collar_context
SH
"$HITCH" lightsblind -c blind-lights -d /tmp -m exact >/dev/null
equal "a collar with no native context source gets no defaulted lights" \
  "" "$(lights_stamp lightsblind)"
GANG_CONTEXT_LIGHTS=40%,70% "$HITCH" lightsblindset -c blind-lights \
  -d /tmp -m exact >/dev/null
equal "but an explicit spec still arms them there" \
  "40%,70%" "$(lights_stamp lightsblindset)"
"$GANG" drop lightsblind >/dev/null
"$GANG" drop lightsblindset >/dev/null

# THE SHIPPED COLLARS ANSWER, AND THEIR ANSWERS DEPEND ON THE MODEL. This is the
# mixed-window team the absolute-threshold setting could not serve: one unset
# team config, working lights on every agent, sized for the window each one has.
lights_shipped() { # $1 collar file, $2 model -> its default, or "none"
  ROOT="$ROOT" bash -c \
    '. "$1"; if out="$(collar_context_lights "$2")"; then printf "%s" "$out"; else printf none; fi' \
    fixture "$1" "$2"
}
equal "claude-code prices its 1M-window models by cost, not remaining room" \
  "20%,40%" "$(lights_shipped "$ROOT/collars/claude-code.sh" claude-opus-5)"
equal "and sizes its 200k class for the runway behind red" \
  "45%,65%" "$(lights_shipped "$ROOT/collars/claude-code.sh" claude-haiku-4-5-20251001)"
equal "no model means no window class, so claude-code offers no default" \
  "none" "$(lights_shipped "$ROOT/collars/claude-code.sh" "")"
equal "codex holds its lights until the window is nearly spent" \
  "75%,90%" "$(lights_shipped "$ROOT/collars/codex.sh" gpt-5-codex)"
equal "and codex offers none either" \
  "none" "$(lights_shipped "$ROOT/collars/codex.sh" "")"

# Each hitched harness in its own killable cgroup. systemd-oomd kills the
# descendant LEAF cgroup holding the most swap, and a tmux server inherits the
# cgroup of whatever started it, so without this every agent shares one leaf and
# one kill takes the team. The scope is proven by invocation and not only by
# composition: the stub records the argv it was actually called with, and the
# wrapped agent still boots and still takes a message.
mkdir -p "$RUN_ROOT/scope-bin"
cat > "$RUN_ROOT/scope-bin/systemd-run" <<SH
#!/bin/sh
printf '%s\n' "\$*" > '$RUN_ROOT/scope.argv'
while [ "\${1:-}" != env ]; do
  [ \$# -gt 0 ] || exit 97
  shift
done
exec "\$@"
SH
cat > "$RUN_ROOT/scope-bin/systemctl" <<'SH'
#!/bin/sh
[ "${1:-}" != --user ] || shift
case "${1:-}" in
  is-active) exit 3 ;;
  *) exit 0 ;;
esac
SH
chmod +x "$RUN_ROOT/scope-bin/systemd-run" "$RUN_ROOT/scope-bin/systemctl"
if scope_path="$(tmux show-environment -g PATH 2>/dev/null)"; then
  scope_path="${scope_path#PATH=}"
else
  scope_path="$PATH"
fi
tmux set-environment -g PATH "$RUN_ROOT/scope-bin:$scope_path"
PATH="$RUN_ROOT/scope-bin:$PATH" GANG_SCOPE=on \
  "$HITCH" scoped -c bash -d /tmp >/dev/null
tmux set-environment -g PATH "$scope_path"
scope_line="$(tmux display-message -p -t "$(window_id scoped)" '#{pane_start_command}')"
contains "a scoped hitch launches inside its own transient user scope" \
  "$scope_line" "systemd-run --user --scope"
contains "and the scope is named after the session and the agent" \
  "$scope_line" "--unit='gangline-$GANG_SESSION-scoped.scope'"
contains "memory accounting is stated, not inherited" \
  "$scope_line" "-p MemoryAccounting=yes"
contains "the scope wraps the launch rather than replacing it" \
  "$scope_line" "PS1='❯ ' bash --norc"
contains "and systemd-run was invoked with the unit, not merely handed it" \
  "$(<"$RUN_ROOT/scope.argv")" "--unit=gangline-$GANG_SESSION-scoped.scope"
printf 'MARK_SCOPED' | "$GANG" send --to scoped --from tester --stdin >/dev/null
# source-guard: producer@8daa1cf9ed62: the verified send just above is the only producer of MARK_SCOPED — no fixture, collar or other sender writes that literal, and the scoped window was hitched empty
contains "a scoped agent is an ordinary agent" "$(pane scoped)" "MARK_SCOPED"
"$GANG" drop scoped >/dev/null

# A PID-isolated sandbox can reach the host system bus while its direct user
# bus is unusable. The local-host machine transport must carry both the manager
# preflight and the stale-unit check, while the launch itself remains the same
# host-side systemd-run command in tmux.
mkdir -p "$RUN_ROOT/scope-host-bin"
cp "$RUN_ROOT/scope-bin/systemd-run" "$RUN_ROOT/scope-host-bin/systemd-run"
cat > "$RUN_ROOT/scope-host-bin/systemctl" <<SH
#!/bin/sh
printf '%s\n' "\$*" >> '$RUN_ROOT/scope-host.argv'
[ "\${1:-}" != --user ] || shift
case "\${1:-}" in
  --machine="$(id -un)@.host") shift ;;
  *) exit 1 ;;
esac
case "\${1:-}" in
  is-active) exit 3 ;;
  *) exit 0 ;;
esac
SH
chmod +x "$RUN_ROOT/scope-host-bin/systemd-run" "$RUN_ROOT/scope-host-bin/systemctl"
tmux set-environment -g PATH "$RUN_ROOT/scope-host-bin:$scope_path"
PATH="$RUN_ROOT/scope-host-bin:$PATH" GANG_SCOPE=on \
  "$HITCH" scopedhost -c bash -d /tmp >/dev/null
tmux set-environment -g PATH "$scope_path"
contains "an unreachable direct user bus retries through the local host" \
  "$(<"$RUN_ROOT/scope-host.argv")" "--machine=$(id -un)@.host show"
contains "the host transport also checks whether the stable unit name is free" \
  "$(<"$RUN_ROOT/scope-host.argv")" "--machine=$(id -un)@.host is-active"
contains "the host-transport preflight still launches the scoped harness" \
  "$(tmux display-message -p -t "$(window_id scopedhost)" '#{pane_start_command}')" \
  "systemd-run --user --scope"
"$GANG" drop scopedhost >/dev/null

# An unusable scope is refused at hitch rather than discovered as a window that
# died at launch, and a value that is neither on nor off never reads as off.
mkdir -p "$RUN_ROOT/scope-nomgr-bin"
cp "$RUN_ROOT/scope-bin/systemd-run" "$RUN_ROOT/scope-nomgr-bin/systemd-run"
cat > "$RUN_ROOT/scope-nomgr-bin/systemctl" <<'SH'
#!/bin/sh
exit 1
SH
chmod +x "$RUN_ROOT/scope-nomgr-bin/systemctl"
mkdir -p "$RUN_ROOT/scope-taken-bin"
cp "$RUN_ROOT/scope-bin/systemd-run" "$RUN_ROOT/scope-taken-bin/systemd-run"
cat > "$RUN_ROOT/scope-taken-bin/systemctl" <<'SH'
#!/bin/sh
exit 0
SH
chmod +x "$RUN_ROOT/scope-taken-bin/systemctl"
refuses "a scope with no user manager to create it is refused at hitch" \
  "cannot give 'scopeless' its own cgroup" \
  env PATH="$RUN_ROOT/scope-nomgr-bin:$PATH" GANG_SCOPE=on \
    "$GANG" hitch scopeless -c bash -d /tmp
equal "a refused scope leaves no window behind" "" "$(window_id scopeless)"
refuses "a scope setting that is neither on nor off is refused" \
  "GANG_SCOPE must be on or off" \
  env GANG_SCOPE=yes "$GANG" config
refuses "a scope name still held by an earlier agent is refused, not respawned" \
  "is still running, so something from an earlier 'scopeheld' outlived its window" \
  env PATH="$RUN_ROOT/scope-taken-bin:$PATH" GANG_SCOPE=on \
    "$GANG" hitch scopeheld -c bash -d /tmp
equal "a refused scope name leaves no window behind" "" "$(window_id scopeheld)"

# THE SERVER THAT HOLDS THE TEAM IS THE ONE PROCESS WHOSE DEATH ENDS ALL OF IT.
# Every agent gets a unit; the server used to inherit whatever cgroup ran
# `gang up`, so a whole team could vanish and leave nothing but a login scope
# deactivating. Only the `tmux new-session` that FORKS a server may be wrapped —
# against a server that is already up it forks nothing, and the scope would hold
# only the client that exits a moment later.
#
# ITS OWN LOCK AND ARCHIVE ROOTS. A second tmux server is a second register of
# published spool identities, and the spool root is keyed by GANG_LOCK_DIR
# rather than by socket — so a session opened on one server while the other
# holds the live windows would read every one of those spools as an orphan.
# One server per lock root is the assumption spool_mint already makes; this
# world keeps it rather than testing what happens when it is broken.
mkdir -p "$RUN_ROOT/scope-server-bin"
cat > "$RUN_ROOT/scope-server-bin/systemd-run" <<SH
#!/bin/sh
printf '%s\n' "\$*" >> '$RUN_ROOT/scope-server.argv'
while [ "\${1:-}" != MemoryAccounting=yes ]; do
  [ \$# -gt 0 ] || exit 97
  shift
done
shift
exec "\$@"
SH
cp "$RUN_ROOT/scope-bin/systemctl" "$RUN_ROOT/scope-server-bin/systemctl"
chmod +x "$RUN_ROOT/scope-server-bin/systemd-run" "$RUN_ROOT/scope-server-bin/systemctl"
scope_server_session="srvteam-$$"
scope_server_root="$RUN_ROOT/scope-server-run"
mkdir -p "$scope_server_root"
scope_server_socket="$scope_server_root/tmux-$(id -u)/default"
rm -f "$RUN_ROOT/scope-server.argv"
scope_server_rc=0
env PATH="$RUN_ROOT/scope-server-bin:$PATH" GANG_SCOPE=on \
    TMUX_TMPDIR="$scope_server_root" GANG_SESSION="$scope_server_session" \
    GANG_LOCK_DIR="$scope_server_root/locks" \
    GANG_ARCHIVE_DIR="$scope_server_root/archive" \
    "$GANG" hitch srvlead -c bash -d /tmp >/dev/null 2>&1 || scope_server_rc=$?
equal "a hitch that forks its own tmux server completes" "0" "$scope_server_rc"
scope_server_argv="$(<"$RUN_ROOT/scope-server.argv")"
contains "the tmux server is started inside a unit named after its session" \
  "$scope_server_argv" "--unit=gangline-$scope_server_session.scope"
contains "and that unit wraps the new-session that forks it" \
  "$scope_server_argv" "tmux new-session"
contains "memory accounting is stated for the server too" \
  "$scope_server_argv" "--unit=gangline-$scope_server_session.scope -p MemoryAccounting=yes"
contains "the agent inside it still gets a unit of its own" \
  "$scope_server_argv" "--unit=gangline-$scope_server_session-srvlead.scope"
env TMUX_TMPDIR="$scope_server_root" GANG_SESSION="$scope_server_session" \
    GANG_LOCK_DIR="$scope_server_root/locks" \
    GANG_ARCHIVE_DIR="$scope_server_root/archive" \
    "$GANG" down "$scope_server_session" >/dev/null 2>&1 || true
tmux -S "$scope_server_socket" kill-server 2>/dev/null || true

# AND A SERVER GANG DID NOT START IS SAID OUT LOUD. Silently claiming a unit
# that would hold nothing is the accounting lie this whole change exists to end,
# so the hitch says the server stays outside it and runs anyway.
rm -f "$RUN_ROOT/scope-server.argv"
scope_joined_session="jointeam-$$"
scope_joined_out="$(env PATH="$RUN_ROOT/scope-server-bin:$PATH" GANG_SCOPE=on \
    GANG_SESSION="$scope_joined_session" \
    "$GANG" hitch joinlead -c bash -d /tmp 2>&1)" || true
contains "a server gang did not start is reported as outside the accounting" \
  "$scope_joined_out" "already running, so gangline does not own a unit for it"
excludes "and no unit is claimed for a server that forks nothing" \
  "$(cat "$RUN_ROOT/scope-server.argv" 2>/dev/null || printf '')" \
  "--unit=gangline-$scope_joined_session.scope"
env GANG_SESSION="$scope_joined_session" "$GANG" down "$scope_joined_session" \
  >/dev/null 2>&1 || true

# A LAUNCH THAT DIES IS NOT A SLOW BOOT, and both shapes of it are real. A
# launch that dies before its window is registered used to surface as a raw
# tmux `no such window` naming nothing gang had tried to run; one that dies
# while gang waits for its input box used to spend the whole boot budget and
# then be reported as an agent that is up but showing something else — a
# live-agent recovery offered for a process that is not running.
cat > "$RUN_ROOT/collars/deadlaunch.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'exit 9' deadlaunch-argv-marker"
SH
if diesatonce_out="$("$GANG" hitch diesatonce -c deadlaunch -d /tmp 2>&1)"; then
  fail "a launch that dies at once is refused" \
    "hitch unexpectedly succeeded: [$diesatonce_out]"
else
  contains "a launch that dies at once is reported as a death" \
    "$diesatonce_out" "'diesatonce' died at launch"
  contains "and the refusal names the command that died" \
    "$diesatonce_out" "deadlaunch-argv-marker"
  excludes "rather than a raw tmux window error naming nothing" \
    "$diesatonce_out" "no such window"
fi
equal "a launch that died leaves no window behind" "" "$(window_id diesatonce)"

# A DEATH UNDER THE COLLAR'S READ IS STAGED, NOT RACED. The window
# registrations have all succeeded by the time the boot wait asks the collar
# what the pane is showing, so that read is the one place left where a capture
# can find the window already gone — and tmux answers a capture on a window
# that went with `can't find window`, on its own stderr, naming an id the
# operator never saw. The death is armed inside the collar's own read, held by
# the pane's fifo until its process is gone, so this arrives every run rather
# than in the few milliseconds a timed launch would have to hit. Both halves of
# the refusal are read: gang's death note has to be there, and tmux's raw
# answer must not have arrived ahead of it.
cat > "$RUN_ROOT/collars/deadcapture.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'exec 9<>$RUN_ROOT/deadcapture.fifo; PS1=\"❯ \" exec bash --norc' deadcapture-argv-marker"
eval "\$(declare -f collar_input | sed '1s/collar_input/deadcapture_shipped_input/')"
collar_input() {
  if [ -e "$RUN_ROOT/deadcapture-arm" ]; then
    rm -f "$RUN_ROOT/deadcapture-arm"
    exec 9<"$RUN_ROOT/deadcapture.fifo"
    tmux send-keys -l -t "\$1" 'exit 9'
    tmux send-keys -t "\$1" Enter
    cat <&9 >/dev/null
    exec 9<&-
    tmux run-shell true >/dev/null
  fi
  deadcapture_shipped_input "\$1"
}
SH
rm -f "$RUN_ROOT/deadcapture.fifo"
mkfifo "$RUN_ROOT/deadcapture.fifo"
: > "$RUN_ROOT/deadcapture-arm"
if deadcapture_out="$("$GANG" hitch deadcapture -c deadcapture -d /tmp 2>&1)"; then
  fail "a launch that dies under the collar's read is refused" \
    "hitch unexpectedly succeeded: [$deadcapture_out]"
else
  contains "a death under the collar's read is reported as a death" \
    "$deadcapture_out" "died at launch"
  contains "and that refusal names the command that died" \
    "$deadcapture_out" "deadcapture-argv-marker"
  excludes "rather than tmux's own answer arriving ahead of it" \
    "$deadcapture_out" "can't find window"
  excludes "in either of the wordings tmux gives that absence" \
    "$deadcapture_out" "no such window"
fi
equal "a death under the collar's read leaves no window behind" \
  "" "$(window_id deadcapture)"

# THE BOOT LOOP'S ONE HOOK IS THE COLLAR'S OWN READ, called once per pass in
# gang's process, so a death staged there lands INSIDE the wait with no clock
# involved and no pane polled. The window is told to hold its corpse and the
# pane's shell is typed an exit; the read that follows is held by the pane's own
# fifo until its last descriptor closes, and a tmux round trip with a child of
# its own drains the corpse the server had not reaped. See the barrier note
# below the deadgone case for why tmux's pane-died hook cannot hold this.
cat > "$RUN_ROOT/collars/deadboot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'exec 9<>$RUN_ROOT/deadboot.fifo; PS1=\"❯ \" exec bash --norc' deadboot-argv-marker"
collar_input() {
  printf 'read\n' >> "$RUN_ROOT/deadboot-reads"
  if [ -e "$RUN_ROOT/deadboot-arm" ]; then
    rm -f "$RUN_ROOT/deadboot-arm"
    tmux set-option -w -t "\$1" remain-on-exit on
    exec 9<"$RUN_ROOT/deadboot.fifo"
    tmux send-keys -l -t "\$1" 'trap "echo DEADBOOT-DYING-WORDS" EXIT; exit 9'
    tmux send-keys -t "\$1" Enter
    cat <&9 >/dev/null
    exec 9<&-
    tmux run-shell true >/dev/null
  fi
  return 1
}
SH
: > "$RUN_ROOT/deadboot-arm"
rm -f "$RUN_ROOT/deadboot-reads" "$RUN_ROOT/deadboot.fifo"
mkfifo "$RUN_ROOT/deadboot.fifo"
if deadboot_out="$("$GANG" hitch deadboot -c deadboot -d /tmp 2>&1)"; then
  fail "a launch that dies inside the boot wait is refused" \
    "hitch unexpectedly succeeded: [$deadboot_out]"
else
  contains "a launch that dies inside the boot wait is reported as a death" \
    "$deadboot_out" "'deadboot' died at launch"
  contains "and the status its pane died with is named" \
    "$deadboot_out" "(status 9)"
  contains "and the last thing it said is carried out with it" \
    "$deadboot_out" "DEADBOOT-DYING-WORDS"
  contains "and so is the command that died" \
    "$deadboot_out" "deadboot-argv-marker"
  excludes "not as an agent that is up and showing something else" \
    "$deadboot_out" "is up but is showing"
fi
# THE WAIT ENDED AT THE DEATH, NOT AT THE BUDGET. One read armed the death and
# the pass after it found the pane dead, so a second read would mean gang had
# gone back to asking what a dead pane was showing.
equal "the boot wait ends at the death rather than at the budget" \
  1 "$(wc -l < "$RUN_ROOT/deadboot-reads" | tr -d ' ')"
refuses "a name whose window holds a dead pane is refused as one, with repairs" \
  "already exists and nothing is running in its window" \
  "$GANG" hitch deadboot -c deadboot -d /tmp
"$GANG" drop deadboot >/dev/null
equal "clearing the dead window frees the name" "" "$(window_id deadboot)"

# The same death where nothing holds the corpse: tmux destroys a window whose
# pane exits, so the reading gang has to interpret is an absence.
cat > "$RUN_ROOT/collars/deadboot-gone.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' deadgone-argv-marker"
collar_input() {
  if [ -e "$RUN_ROOT/deadgone-arm" ]; then
    rm -f "$RUN_ROOT/deadgone-arm"
    tmux kill-window -t "\$1"
  fi
  return 1
}
SH
: > "$RUN_ROOT/deadgone-arm"
if deadgone_out="$("$GANG" hitch deadgone -c deadboot-gone -d /tmp 2>&1)"; then
  fail "a boot whose window goes away is refused" \
    "hitch unexpectedly succeeded: [$deadgone_out]"
else
  contains "a window that goes away during the boot wait is read as a death" \
    "$deadgone_out" "'deadgone' died at launch: its window is already gone"
  contains "and that refusal names the command too" \
    "$deadgone_out" "deadgone-argv-marker"
  excludes "rather than an unreadable pane" "$deadgone_out" "cannot read pane"
fi
equal "and no window is left behind by it" "" "$(window_id deadgone)"

# THE pane-died HOOK IS NOT A BARRIER, and every death below is ordered without
# it. tmux settles a pane's death from two independent events: the pty reaching
# EOF, which is what makes #{pane_dead} read 1, and the reap of the child, which
# is what fills in #{pane_dead_status} and draws the held corpse's banner. The
# hook fires only from the second, and only while the first has already landed —
# so when the EOF is processed before the reap the hook is never dispatched at
# all, neither then nor when the reap arrives afterwards. `tmux wait-for` has no
# bound, so a fixture waiting on that hook does not go red, it stops the suite.
# Measured on tmux 3.2a: a pane reading dead with an empty status, its child
# still an unreaped zombie, and its channel never signalled again.
#
# A pane's own file descriptors carry the fact instead. The pane holds a fifo
# open read-write, so its own open cannot block, and the fixture's read-only
# open returns once that descriptor exists. Reading the fifo to EOF afterwards
# ends exactly when the pane's last descriptor closes, so the process is gone
# rather than merely typed at. `tmux run-shell` then costs the server a round
# trip and a child of its own, which drains any pane it had not reaped, and the
# settled fact is asserted immediately rather than waited on.
#
# WHERE THE FIFO IS OPENED IS THE WHOLE GUARANTEE, so it is opened in the pane's
# own launch command wherever this fixture writes one — the deadboot collar's
# GANG_LAUNCH and the split below. A launch command runs before the shell reads
# anything, so the descriptor exists as soon as tmux has spawned the pane and no
# typed line has to be consumed for the fixture's open to return. A pane that is
# alive but never reads its input cannot park the fixture there. What the form
# still rests on is that the pane outlives the fixture's open, which is the
# fixture's own premise everywhere below: each of these panes is a shell that
# stays up until it is typed an exit. A launch that died before its first
# descriptor existed would park, and the deliberate alternative — holding a
# writer open here so the read could never block — was refused, because it
# converts that park into a red every time a pane opens its fifo a moment later
# than the fixture reaches the read, and a mandatory gate that reddens at random
# is worse than one that stops on a broken environment.
#
# The one pane whose launch this fixture does not write is the hitched agent's,
# and its fifo is opened by a typed line below. That line is consumed on the
# same guarantee the exit typed after it already rests on: hitch has verified
# the agent's input box before either is sent. This is the suite's ordinary
# standard for a typed barrier and not a new exposure — but it is the one place
# here where a shell that stopped reading would park rather than fail, so do not
# copy this form to a pane whose readiness nothing has established.
pane_holds_fifo() { # $1 = pane id, $2 = fifo the pane's shell must hold open
  tmux send-keys -l -t "$1" "exec 9<>$2"
  tmux send-keys -t "$1" Enter
}

# A WINDOW IS NOT ITS ACTIVE PANE. An operator who splits an agent's window and
# leaves a held corpse in front still has a live shell in it, and the refusal
# for a window where nothing runs names `gang drop`, which kills the whole
# window — so reading only the active pane turns a wrong answer into a
# destructive instruction. Both directions are driven here: the corpse in front
# of a live pane, and then the same window with nothing left running in it.
"$HITCH" splitcorpse -c bash -d /tmp >/dev/null
splitcorpse_id="$(window_id splitcorpse)"
tmux set-option -w -t "$splitcorpse_id" remain-on-exit on
splitcorpse_front="$(tmux display-message -p -t "$splitcorpse_id" '#{pane_id}')"
mkfifo "$RUN_ROOT/splitcorpse-front.fifo" "$RUN_ROOT/splitcorpse-rest.fifo"
splitcorpse_live="$(tmux split-window -d -P -F '#{pane_id}' -t "$splitcorpse_id" \
  "exec 9<>$RUN_ROOT/splitcorpse-rest.fifo; PS1='❯ ' exec bash --norc")"
pane_holds_fifo "$splitcorpse_front" "$RUN_ROOT/splitcorpse-front.fifo"
exec 3<"$RUN_ROOT/splitcorpse-front.fifo"
exec 4<"$RUN_ROOT/splitcorpse-rest.fifo"
tmux send-keys -l -t "$splitcorpse_front" 'exit 9'
tmux send-keys -t "$splitcorpse_front" Enter
cat <&3 >/dev/null
exec 3<&-
tmux run-shell true >/dev/null
equal "the corpse in front is a settled death before the window is read" \
  1 "$(tmux display-message -p -t "$splitcorpse_front" '#{pane_dead}')"
equal "and the pane behind it is still running" \
  0 "$(tmux display-message -p -t "$splitcorpse_live" '#{pane_dead}')"
splitcorpse_out="$("$GANG" hitch splitcorpse -c bash -d /tmp 2>&1)" || :
excludes "a corpse in front of a live pane is not read as an empty window" \
  "$splitcorpse_out" "nothing is running in its window"
contains "so the refusal is the ordinary one, which destroys nothing" \
  "$splitcorpse_out" "already exists"
tmux select-pane -t "$splitcorpse_live"
equal "a quiet live pane beside a held corpse keeps its ordinary state" \
  "~idle~" \
  "$("$GANG" status splitcorpse | sed -n '1p')"
tmux send-keys -l -t "$splitcorpse_live" 'exit 7'
tmux send-keys -t "$splitcorpse_live" Enter
cat <&4 >/dev/null
exec 4<&-
tmux run-shell true >/dev/null
equal "and every pane in the window is a settled death before it is read" \
  "" "$(tmux list-panes -t "$splitcorpse_id" -F '#{pane_dead}' | grep -vx 1)"
equal "status reports that nothing is running in a held corpse window" \
  "!dead! (nothing is running in this window)" \
  "$("$GANG" status splitcorpse | sed -n '1p')"
equal "porcelain roster gives a dead window its own stable word" dead \
  "$("$GANG" roster --porcelain \
    | awk -F '\t' '$1 == "splitcorpse" { print $3 }')"
refuses "and once every pane has exited the same window reads as empty" \
  "already exists and nothing is running in its window" \
  "$GANG" hitch splitcorpse -c bash -d /tmp
"$GANG" drop splitcorpse >/dev/null
equal "clearing the split window frees the name" "" "$(window_id splitcorpse)"
