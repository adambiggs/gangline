# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Real tmux substrate: lifecycle, observation, verified attributed delivery, custom-collar precedence and migration, and native session identity.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# Real tmux substrate: lifecycle, observation, verified attributed delivery and
# exact-name addressing. Gangline's command returns only after the state checked
# below has been established.
printf 'unset BASHPID\n' > "$RUN_ROOT/no-bashpid"
BASH_ENV="$RUN_ROOT/no-bashpid" \
  "$HITCH" alpha -c bash -d /tmp >/dev/null
excludes "a collar with no launch choices emits no impossible-choice warning" \
  "$(<"$RUN_ROOT/hitch-stderr")" "hitching 'alpha' without"
alpha_id="$(window_id alpha)"
# Keep the Bash stand-in's composer immediately observable for ordinary
# doctrine-sized contracts while leaving the explicit pane-overflow fixture far
# beyond the grid below.
tmux resize-window -t "$alpha_id" -x 80 -y 30
equal "a hitched first turn writes the raw busy window glyph" \
  "-alpha-" "$(tmux display-message -p -t "$alpha_id" '#{window_name}')"
contains "Bash 3.2 can lock and deliver the startup contract" \
  "$(pane alpha)" "You are alpha in Gangline"
contains "hitch creates an observable idle agent" "$($GANG status alpha)" "~idle~"
equal "an idle observation writes the raw slack window glyph" \
  "~alpha~" "$(tmux display-message -p -t "$alpha_id" '#{window_name}')"
contains "roster lists the hitched collar" "$($GANG roster)" "alpha"
contains "roster is an immediate snapshot" \
  "$(GANG_CHURN_WAIT=not-a-duration $GANG roster)" "alpha"

alpha_tmux_pane="$(tmux list-panes -t "$alpha_id" -F '#{pane_id}')"
contains "whoami prints the collar field under its 1.0 name" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" whoami)" "collar: bash"
contains "bare status targets the calling agent window" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" status)" "~idle~"
contains "bare capture targets the calling agent window" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" capture)" \
  "You are alpha in Gangline"
equal "bare composer targets the calling agent window" "" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" composer)"

# 2.0 removed the pre-rename spellings. The alias no longer resolves to
# -c/--collar, so it is refused as an unknown argument before a window exists.
refuses "the removed hitch collar flag is an unknown argument" \
  "hitch: unknown argument '-p'" \
  "$GANG" hitch legacy-flag -p bash -d /tmp
excludes "the refused legacy flag opens no window" "$(window_names)" "legacy-flag"
refuses "and its long spelling is unknown too" \
  "hitch: unknown argument '--profile'" \
  "$GANG" hitch legacy-flag --profile bash -d /tmp
excludes "the refused long spelling opens no window" "$(window_names)" "legacy-flag"

# Calibrate custom-directory precedence against a state with no shadow file,
# then make the custom bash collar emit a marker the shipped fixture cannot.
shadow_dir="$RUN_ROOT/shadow-collars"
mkdir -p "$shadow_dir"
excludes "the shipped bash collar cannot emit the shadow marker" \
  "$(GANG_COLLARS="$shadow_dir" "$GANG" composer alpha)" "CUSTOM_COLLAR_MARKER"
cat > "$shadow_dir/bash.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
_shadow_real="\$(declare -f collar_input)"
eval "shadow_real_input \${_shadow_real#collar_input}"
collar_input() {
  [ ! -e "$RUN_ROOT/shadow-marker-on" ] || { printf 'CUSTOM_COLLAR_MARKER'; return; }
  shadow_real_input "\$1"
}
SH
GANG_COLLARS="$shadow_dir" "$HITCH" shadowed -c bash -d /tmp >/dev/null
: > "$RUN_ROOT/shadow-marker-on"
equal "a custom collar directory shadows the shipped collar" \
  "CUSTOM_COLLAR_MARKER" \
  "$(GANG_COLLARS="$shadow_dir" "$GANG" composer shadowed)"
"$GANG" drop shadowed >/dev/null

# 2.0 removed the 0.x contract-function spelling. A collar that still declares
# profile_input is no longer forwarded into collar_input: it is a collar with no
# input reader, so the reading falls back to the whole pane instead of that
# collar's function. The current spelling calibrates the reader first, so the
# negative assertion below cannot pass because the instrument reads nothing.
cat > "$RUN_ROOT/collars/legacy-contract.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
profile_input() { printf 'MARK_LEGACY_FORWARD'; }
unset -f collar_input
SH
cat > "$RUN_ROOT/collars/current-contract.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_input() { printf 'MARK_LEGACY_FORWARD'; }
SH
tmux new-window -d -t "=$GANG_SESSION" -n current-contract "PS1='❯ ' bash --norc"
"$GANG" adopt current-contract -c current-contract >/dev/null
equal "the reader instrument is calibrated on the current spelling" \
  "MARK_LEGACY_FORWARD" "$("$GANG" composer current-contract 2>&1)"
"$GANG" drop current-contract >/dev/null
tmux new-window -d -t "=$GANG_SESSION" -n legacy-contract "PS1='❯ ' bash --norc"
"$GANG" adopt legacy-contract -c legacy-contract > /dev/null \
  2> "$RUN_ROOT/legacy-contract-adopt.err"
equal "adopting a collar with only the removed spelling announces nothing" "" \
  "$(<"$RUN_ROOT/legacy-contract-adopt.err")"
excludes "the removed contract spelling is not forwarded into collar_input" \
  "$("$GANG" composer legacy-contract 2>&1)" "MARK_LEGACY_FORWARD"
"$GANG" drop legacy-contract >/dev/null

# A running pre-rename window is no longer migrated in place: @gl_profile is
# residue Gangline does not read, so a window carrying only that option is not
# a registered agent and is refused rather than silently healed.
"$HITCH" legacy-option -c bash -d /tmp >/dev/null
legacy_option_id="$(window_id legacy-option)"
legacy_option_pane="$(tmux list-panes -t "$legacy_option_id" -F '#{pane_id}')"
tmux set-option -w -t "$legacy_option_id" @gl_profile bash
tmux set-option -uw -t "$legacy_option_id" @gl_collar
refuses "a window carrying only the removed option is not an agent" \
  "is not a gang agent" "$GANG" status legacy-option
equal "and the removed option is left exactly where it was, unmigrated" "bash" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_profile)"
equal "and nothing was written into the current option" "" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_collar)"
printf '%s\n' '{"hook_event_name":"UserPromptSubmit"}' \
  | TMUX_PANE="$legacy_option_pane" "$GANG" hook \
    > "$RUN_ROOT/legacy-option-hook.out" 2> "$RUN_ROOT/legacy-option-hook.err"
equal "a hook over the removed option is byte-silent on stderr" "" \
  "$(<"$RUN_ROOT/legacy-option-hook.err")"
equal "and the hook does not migrate it either" "" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_collar)"
tmux set-option -w -t "$legacy_option_id" @gl_collar bash
tmux set-option -uw -t "$legacy_option_id" @gl_profile

# The fixture clock is an instrument, so this run says exactly which bytes it
# ran under rather than leaving the reader to trust the name.
sleep_stub_sha="$(shasum -a 256 "$RUN_ROOT/bin/sleep" | awk '{print $1}')"
printf 'instrument sleep=%s sha256=%s spec-sha256=%s\n' \
  "$RUN_ROOT/bin/sleep" "$sleep_stub_sha" \
  '52a740a3954c18b82f2b4461d92a38dfb17d8e1655d4e5796c4d0e07f97ac995'

GANG_CONTEXT_LIGHTS=off "$HITCH" ctx-agent -c ctx-known -d /tmp >/dev/null
ctx_agent_id="$(window_id ctx-agent)"
ctx_agent_pane="$(tmux list-panes -t "$ctx_agent_id" -F '#{pane_id}')"
equal "named context prints the collar reading byte-for-byte" \
  "42k/200k (21%)" "$("$GANG" context ctx-agent)"
equal "bare context targets the calling agent window" \
  "42k/200k (21%)" "$(TMUX_PANE="$ctx_agent_pane" "$GANG" context)"
equal "context answers while context lights are off" \
  "42k/200k (21%)" "$(GANG_CONTEXT_LIGHTS=off "$GANG" context ctx-agent)"
"$HITCH" ctx-failing -c ctx-fail -d /tmp >/dev/null
excludes "roster carries no context reading column" \
  "$("$GANG" roster)" "42k/200k"

ctx_fail_stdout="$RUN_ROOT/context-fail.stdout"
ctx_fail_stderr="$RUN_ROOT/context-fail.stderr"
if "$GANG" context ctx-failing >"$ctx_fail_stdout" 2>"$ctx_fail_stderr"; then
  fail "a collar context failure stays non-zero" "context unexpectedly succeeded"
else
  pass "a collar context failure stays non-zero"
fi
equal "a collar context failure fabricates no reading" "" "$(<"$ctx_fail_stdout")"
contains "a collar context failure keeps its own diagnostic" \
  "$(<"$ctx_fail_stderr")" "fixture context unavailable"

"$HITCH" ctx-missing -c ctx-none -d /tmp >/dev/null
refuses "a missing collar_context names the collar" \
  "collar 'ctx-none' declares no collar_context" \
  "$GANG" context ctx-missing

GANG_CONTEXT_LIGHTS=off "$HITCH" usage-inline -c usage-inline -d /tmp >/dev/null
usage_inline_id="$(window_id usage-inline)"
tmux resize-window -t "$usage_inline_id" -x 80 -y 12
usage_marker_ready="test-usage-marker-ready-$$"
printf -v usage_marker_cmd 'printf "INLINE_OLD_MARKER\\n"; tmux wait-for -S %q' \
  "$usage_marker_ready"
tmux send-keys -l -t "$usage_inline_id" "$usage_marker_cmd"
tmux send-keys -t "$usage_inline_id" Enter
tmux wait-for "$usage_marker_ready"
expected_usage="$(awk 'BEGIN { for (i=1; i<=30; i++) printf "USAGE_%02d\n", i }')"
usage_inline_out="$("$GANG" usage usage-inline)"
equal "inline usage returns every line taller than the pane" \
  "$expected_usage" "$(printf '%s\n' "$usage_inline_out" | sed 's/[[:space:]]*$//')"
excludes "inline usage excludes the pre-existing transcript" \
  "$usage_inline_out" "INLINE_OLD_MARKER"
equal "inline usage restores an empty composer" "" \
  "$("$GANG" composer usage-inline)"

"$HITCH" usage-modal -c usage-modal -d /tmp >/dev/null
usage_modal_out="$("$GANG" usage usage-modal)"
equal "modal usage returns the visible page raw" \
  $'MODAL_ONE\nMODAL_TWO' \
  "$(printf '%s\n' "$usage_modal_out" | sed 's/[[:space:]]*$//')"
equal "modal usage dismisses back to an empty composer" "" \
  "$("$GANG" composer usage-modal)"

"$HITCH" usage-confirm -c usage-confirm -d /tmp >/dev/null
usage_confirm_out="$("$GANG" usage usage-confirm)"
equal "usage presses the collar's confirmation key before capture" \
  "CONFIRMED_USAGE" \
  "$(printf '%s\n' "$usage_confirm_out" | sed 's/[[:space:]]*$//')"
equal "confirmed modal usage restores an empty composer" "" \
  "$("$GANG" composer usage-confirm)"

"$HITCH" usage-stuck -c usage-stuck -d /tmp >/dev/null
usage_stuck_stdout="$RUN_ROOT/usage-stuck.stdout"
usage_stuck_stderr="$RUN_ROOT/usage-stuck.stderr"
if "$GANG" usage usage-stuck >"$usage_stuck_stdout" 2>"$usage_stuck_stderr"; then
  fail "usage refuses when its dismissal does not restore the composer" \
    "usage unexpectedly succeeded"
else
  pass "usage refuses when its dismissal does not restore the composer"
fi
contains "failed usage restoration still prints the captured content" \
  "$(<"$usage_stuck_stdout")" "MODAL_STUCK"
contains "failed usage restoration names the key and gang attach" \
  "$(<"$usage_stuck_stderr")" "after C-g"
contains "failed usage restoration points at gang attach" \
  "$(<"$usage_stuck_stderr")" "gang attach"

usage_before_refusals="$(pane usage-inline)"
tmux set-option -w -t "$usage_inline_id" @gl_turn "open $(date +%s)"
refuses "usage refuses a busy target" "is mid-turn" \
  "$GANG" usage usage-inline
tmux set-option -w -t "$usage_inline_id" @gl_turn broken
refuses "usage refuses a could-not-determine target" \
  "cannot determine whether 'usage-inline' is mid-turn" \
  "$GANG" usage usage-inline
tmux set-option -uw -t "$usage_inline_id" @gl_turn
equal "usage readiness refusals type nothing" \
  "$usage_before_refusals" "$(pane usage-inline)"

"$HITCH" usage-occupied -c usage-occupied -d /tmp >/dev/null
usage_occupied_id="$(window_id usage-occupied)"
usage_occupied_ready="test-usage-occupied-ready-$$"
printf -v usage_occupied_cmd 'printf OCCUPIED_USAGE; tmux wait-for -S %q; IFS= read -r _' \
  "$usage_occupied_ready"
tmux send-keys -l -t "$usage_occupied_id" "$usage_occupied_cmd"
tmux send-keys -t "$usage_occupied_id" Enter
tmux wait-for "$usage_occupied_ready"
usage_occupied_before="$(pane usage-occupied)"
refuses "usage refuses an occupied target" "occupied (authority unknown)" \
  "$GANG" usage usage-occupied
equal "an occupied usage refusal types nothing" \
  "$usage_occupied_before" "$(pane usage-occupied)"

refuses "a collar with no GANG_USAGE_CMD refuses usage" \
  "declares no GANG_USAGE_CMD" "$GANG" usage alpha

# A SCREEN THAT DIFFERS BECAUSE GANG JUST TYPED INTO IT IS NOT A USAGE SCREEN.
# Until this fixture existed, gang pressed Enter and compared the very next
# capture, so whichever of two states that round trip caught decided which
# refusal came out: the shell finishing first gave "never changed after clear",
# gang looking first gave a rollover complaint about a scrollback that had not
# rolled over. Measured at two wrong refusals in twenty-five under sustained
# load, none in twenty quiet. Here the command never completes, so the losing
# state is permanent and the verdict is the same every time.
"$HITCH" usage-hold -c usage-hold -d /tmp >/dev/null
usage_hold_id="$(window_id usage-hold)"
usage_hold_ready="test-usage-hold-ready-$$"
usage_hold_marker="READY_USAGE_HOLD_$$"
printf -v usage_hold_cmd 'PROMPT_COMMAND=; PS1=%q""%q; clear' \
  "READY_USAGE_" "HOLD_$$❯ "
printf -v usage_hold_pipe \
  'needle=%q; event=%q; seen=; while IFS= read -r -n 1 char; do seen="${seen}${char}"; case "$seen" in *"$needle") tmux wait-for -S "$event"; exit 0;; esac; if [ "${#seen}" -gt 256 ]; then seen="${seen: -256}"; fi; done' \
  "$usage_hold_marker❯ " "$usage_hold_ready"
printf -v usage_hold_pipe_shell '%q' "$usage_hold_pipe"
tmux pipe-pane -O -t "$usage_hold_id" "bash -c $usage_hold_pipe_shell"
tmux send-keys -l -t "$usage_hold_id" "$usage_hold_cmd"
tmux send-keys -t "$usage_hold_id" Enter
tmux wait-for "$usage_hold_ready"
tmux pipe-pane -t "$usage_hold_id"
usage_hold_stdout="$RUN_ROOT/usage-hold.stdout"
usage_hold_stderr="$RUN_ROOT/usage-hold.stderr"
if "$GANG" usage usage-hold \
    >"$usage_hold_stdout" 2>"$usage_hold_stderr"; then
  fail "usage refuses a command the harness has not taken" \
    "usage unexpectedly succeeded"
else
  pass "usage refuses a command the harness has not taken"
fi
contains "an unconsumed command is named as an unverified submission" \
  "$(<"$usage_hold_stderr")" "submit NOT verified"
excludes "and is not reported as a scrollback that rolled over" \
  "$(<"$usage_hold_stderr")" "rolled over"
equal "an unconsumed command yields no usage content" "" \
  "$(<"$usage_hold_stdout")"
"$GANG" drop usage-hold >/dev/null

"$HITCH" usage-nochange -c usage-nochange -d /tmp >/dev/null
usage_nochange_id="$(window_id usage-nochange)"
usage_nochange_ready="test-usage-nochange-ready-$$"
usage_nochange_marker="READY_USAGE_NOCHANGE_$$"
printf -v usage_nochange_cmd 'PROMPT_COMMAND=; PS1=%q""%q; clear' \
  "READY_USAGE_" "NOCHANGE_$$❯ "
printf -v usage_nochange_pipe \
  'needle=%q; event=%q; seen=; while IFS= read -r -n 1 char; do seen="${seen}${char}"; case "$seen" in *"$needle") tmux wait-for -S "$event"; exit 0;; esac; if [ "${#seen}" -gt 256 ]; then seen="${seen: -256}"; fi; done' \
  "$usage_nochange_marker❯ " "$usage_nochange_ready"
printf -v usage_nochange_pipe_shell '%q' "$usage_nochange_pipe"
tmux pipe-pane -O -t "$usage_nochange_id" \
  "bash -c $usage_nochange_pipe_shell"
tmux send-keys -l -t "$usage_nochange_id" "$usage_nochange_cmd"
tmux send-keys -t "$usage_nochange_id" Enter
tmux wait-for "$usage_nochange_ready"
tmux pipe-pane -t "$usage_nochange_id"
usage_nochange_stdout="$RUN_ROOT/usage-nochange.stdout"
usage_nochange_stderr="$RUN_ROOT/usage-nochange.stderr"
if "$GANG" usage usage-nochange \
    >"$usage_nochange_stdout" 2>"$usage_nochange_stderr"; then
  fail "usage refuses when the native screen never changes" \
    "usage unexpectedly succeeded"
else
  pass "usage refuses when the native screen never changes"
fi
contains "an unchanged usage screen names the command" \
  "$(<"$usage_nochange_stderr")" "after clear"
equal "an unchanged usage screen prints no content" "" \
  "$(<"$usage_nochange_stdout")"
equal "an unchanged usage screen leaves the composer empty" "" \
  "$("$GANG" composer usage-nochange)"

tmux set-option -g history-limit 5
"$HITCH" usage-rollover -c usage-inline -d /tmp >/dev/null
tmux set-option -g history-limit 2000
usage_rollover_id="$(window_id usage-rollover)"
tmux resize-window -t "$usage_rollover_id" -x 80 -y 12
usage_rollover_stdout="$RUN_ROOT/usage-rollover.stdout"
usage_rollover_stderr="$RUN_ROOT/usage-rollover.stderr"
if "$GANG" usage usage-rollover \
    >"$usage_rollover_stdout" 2>"$usage_rollover_stderr"; then
  fail "usage refuses a rolled-over inline scrollback" \
    "usage unexpectedly succeeded"
else
  pass "usage refuses a rolled-over inline scrollback"
fi
contains "a rolled-over usage read names the lost origin" \
  "$(<"$usage_rollover_stderr")" "scrollback of 'usage-rollover' rolled over"
equal "a rolled-over usage read prints no content" "" \
  "$(<"$usage_rollover_stdout")"

"$HITCH" usage-unknown -c usage-unknown -d /tmp >/dev/null
refuses "usage refuses an unknown render declaration" \
  "unknown GANG_USAGE_RENDER 'unknown'" "$GANG" usage usage-unknown

alpha_before_bare_help="$(pane alpha)"
alpha_composer_before_bare_help="$($GANG composer alpha)"
for incoherent_bare in hitch adopt send drop; do
  if incoherent_output="$(TMUX_PANE="$alpha_tmux_pane" "$GANG" "$incoherent_bare" 2>&1)"; then
    fail "bare gang $incoherent_bare refuses inside an agent" \
      "command unexpectedly succeeded: [$incoherent_output]"
  else
    contains "bare gang $incoherent_bare prints help inside an agent" \
      "$incoherent_output" "gang $incoherent_bare"
  fi
done
# A COMMAND THAT TAKES ONE AGENT NAME DEFAULTS TO THE CALLER. Bare flush,
# interrupt and usage inside an agent window therefore resolve to that agent
# rather than printing a synopsis, and each then refuses on something the
# fixture's own collar does not declare — which is the evidence that a target
# was resolved at all. drop, down, adopt, hitch and send's recipient stay
# without a self default on purpose and remain in the loop above.
for self_bare in flush:GANG_QUEUED_REGEX interrupt:GANG_INTERRUPT_KEY usage:GANG_USAGE_CMD; do
  self_bare_cmd="${self_bare%%:*}"
  self_bare_decl="${self_bare#*:}"
  if self_bare_out="$(TMUX_PANE="$alpha_tmux_pane" "$GANG" "$self_bare_cmd" 2>&1)"; then
    fail "bare gang $self_bare_cmd targets the calling agent" \
      "command unexpectedly succeeded: [$self_bare_out]"
  else
    contains "bare gang $self_bare_cmd targets the calling agent" \
      "$self_bare_out" "declares no $self_bare_decl"
    excludes "bare gang $self_bare_cmd prints no synopsis inside an agent" \
      "$self_bare_out" "gang $self_bare_cmd"
  fi
done
equal "incoherent bare commands leave the calling pane untouched" \
  "$alpha_before_bare_help" "$(pane alpha)"
equal "incoherent bare commands leave the composer untouched" \
  "$alpha_composer_before_bare_help" "$($GANG composer alpha)"
"$GANG" drop ctx-agent >/dev/null
"$GANG" drop ctx-failing >/dev/null
"$GANG" drop ctx-missing >/dev/null
"$GANG" drop usage-inline >/dev/null
"$GANG" drop usage-modal >/dev/null
"$GANG" drop usage-confirm >/dev/null
"$GANG" drop usage-stuck >/dev/null
"$GANG" drop usage-occupied >/dev/null
"$GANG" drop usage-nochange >/dev/null
"$GANG" drop usage-rollover >/dev/null
"$GANG" drop usage-unknown >/dev/null

outside_status="$(env -u TMUX_PANE "$GANG" status 2>&1 || true)"
contains "bare status outside an agent prints its synopsis" \
  "$outside_status" "gang status"
contains "bare status outside an agent explains why self is unavailable" \
  "$outside_status" "this shell is not a Gangline agent window"

for meaningful_command in roster collars config curfew notify; do
  if meaningful_output="$($GANG "$meaningful_command" 2>&1)"; then
    excludes "bare gang $meaningful_command keeps its ordinary meaning" \
      "$meaningful_output" "gang — drive native CLI agents in tmux"
  else
    fail "bare gang $meaningful_command keeps its ordinary meaning" \
      "$meaningful_output"
  fi
done
binary_stamp="$(tmux show-options -wqv -t "$alpha_id" @gl_binary_id)"
if [[ "$binary_stamp" =~ ^cksum:[0-9]+:[0-9]+$ ]]; then
  pass "hitch stamps the documented binary identity"
else
  fail "hitch stamps the documented binary identity" "got [$binary_stamp]"
fi
excludes "status is quiet when the stamped binary is current" \
  "$("$GANG" status alpha)" "binary-skew"
excludes "roster is quiet when the stamped binary is current" \
  "$("$GANG" roster)" "binary-skew"
tmux set-option -w -t "$alpha_id" @gl_binary_id cksum:1:2
contains "status warns when the running window has another binary identity" \
  "$("$GANG" status alpha)" "binary-skew (cksum:1:2 != current $binary_stamp)"
contains "roster warns when the running window has another binary identity" \
  "$("$GANG" roster)" "binary-skew (cksum:1:2 != current $binary_stamp)"
tmux set-option -w -t "$alpha_id" @gl_binary_id "$binary_stamp"

installed_root="$RUN_ROOT/installed"
mkdir -p "$installed_root/bin"
cp -R "$ROOT/collars" "$installed_root/collars"
cp "$GANG" "$installed_root/bin/gang"
installed_gang="$installed_root/bin/gang"
excludes "byte-identical Gangline copies compare as current" \
  "$("$installed_gang" status alpha)" "binary-skew"
printf '\n# fixture changes the executable bytes\n' >> "$installed_gang"
changed_stamp="$(cksum "$installed_gang" | awk '{ print "cksum:" $1 ":" $2 }')"
contains "an uncommitted executable change produces binary skew" \
  "$("$installed_gang" status alpha)" \
  "binary-skew ($binary_stamp != current $changed_stamp)"

dirty_root="$RUN_ROOT/dirty-checkout"
mkdir -p "$dirty_root/bin"
# gang_root resolves the executing checkout physically. Keep this fixture's
# expected path in the same identity domain when TMPDIR traverses a symlink
# (macOS exposes /var/folders through /private/var/folders).
dirty_root="$(cd -P "$dirty_root" && pwd)"
cp -R "$ROOT/collars" "$dirty_root/collars"
cp "$GANG" "$dirty_root/bin/gang"
git -C "$dirty_root" init -q
git -C "$dirty_root" add -- bin/gang collars
git -C "$dirty_root" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: clean executable witness'
dirty_head="$(git -C "$dirty_root" rev-parse HEAD)"
excludes "a clean checkout executable emits no dirty-execution warning" \
  "$("$dirty_root/bin/gang" collars 2>&1)" "executing dirty"
printf '\n# named dirty-execution mutant\n' >> "$dirty_root/bin/gang"
dirty_warning="$("$dirty_root/bin/gang" collars 2>&1)"
contains "a dirty live executable warns on ordinary command dispatch" \
  "$dirty_warning" "WARNING: executing dirty $dirty_root/bin/gang"
contains "the dirty warning names the HEAD its bytes diverged from" \
  "$dirty_warning" "$dirty_head"
git -C "$dirty_root" checkout -q -- bin/gang
excludes "restoring the executable to HEAD removes the warning" \
  "$("$dirty_root/bin/gang" collars 2>&1)" "executing dirty"

tmux set-option -uw -t "$alpha_id" @gl_binary_id
contains "status warns when a pre-witness window is unstamped" \
  "$("$GANG" status alpha)" "binary-skew (window unstamped; current $binary_stamp)"
contains "roster warns when a pre-witness window is unstamped" \
  "$("$GANG" roster)" "binary-skew (window unstamped; current $binary_stamp)"
tmux set-option -w -t "$alpha_id" @gl_binary_id "$binary_stamp"

mkdir -p "$RUN_ROOT/no-identity"
cat > "$RUN_ROOT/no-identity/git" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 127
SH
chmod +x "$RUN_ROOT/no-identity/git"
contains "a broken git command cannot abort roster" \
  "$(PATH="$RUN_ROOT/no-identity:$PATH" "$GANG" roster)" "alpha"
cat > "$RUN_ROOT/no-identity/cksum" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 127
SH
chmod +x "$RUN_ROOT/no-identity/cksum"
unavailable_status="$(PATH="$RUN_ROOT/no-identity:$PATH" "$GANG" status alpha)"
contains "an unavailable witness is explicit without aborting status" \
  "$unavailable_status" \
  "binary-identity unavailable (window $binary_stamp; current unavailable)"
contains "an unavailable witness does not abort roster" \
  "$(PATH="$RUN_ROOT/no-identity:$PATH" "$GANG" roster)" \
  "binary-identity unavailable (window $binary_stamp; current unavailable)"

adopted_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n adopted "PS1='❯ ' bash --norc")"
"$GANG" adopt adopted -c bash >/dev/null
equal "adopt stamps the current binary identity" "$binary_stamp" \
  "$(tmux show-options -wqv -t "$adopted_id" @gl_binary_id)"
"$GANG" drop adopted >/dev/null

# THE OPERATOR-FACING HALF of the clipped-composer reading. A box that outgrew
# its pane is drawn and is taking input; what it lacks is room to render. Naming
# that a harness which never drew a box sends the reader after the wrong thing,
# and it is the reading that let a stranded paste look like a dead harness.
clipped_agent_rule="$(printf '─%.0s' $(seq 40))"
{ printf 'transcript %s\n' 1 2 3 4 5 6
  printf '%s\n' "$clipped_agent_rule"
  printf '%s\n' '❯ a pasted body that outgrew'
  printf '%s\n' '  the rows this pane had for'
  printf '%s' '  it, so the box never closes'
} > "$RUN_ROOT/clipped-agent-frame"
tmux new-window -d -t "=$GANG_SESSION" -n clipped-agent \
  "cat '$RUN_ROOT/clipped-agent-frame'; cat" >/dev/null
"$GANG" adopt clipped-agent -c claude-code >/dev/null
clipped_agent_read="$("$GANG" composer clipped-agent 2>&1 || true)"
contains "gang composer names a box its pane was too short to show" \
  "$clipped_agent_read" "taller than its pane"
excludes "gang composer does not blame the harness for a box it did draw" \
  "$clipped_agent_read" "has not drawn its input box"
"$GANG" drop clipped-agent >/dev/null

adopt_alias_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n adopt-alias "PS1='❯ ' bash --norc")"
refuses "the removed adopt collar flag is an unknown argument" \
  "adopt: unknown argument '-p'" \
  "$GANG" adopt adopt-alias -p bash
equal "the refused adopt leaves the window unregistered" "" \
  "$(tmux show-options -wqv -t "$adopt_alias_id" @gl_agent)"
tmux kill-window -t "$adopt_alias_id"

# composer reads the box through the collar's styled reading, not the raw
# pane; a freshly hitched agent's box is definitively empty
equal "composer prints nothing for an empty box" \
  "" "$("$GANG" composer alpha)"

mkdir -p "$RUN_ROOT/collars"
export GANG_COLLARS="$RUN_ROOT/collars"

# Native session identity is a window fact, and every renewal consumes that
# exact fact rather than directory recency. This fixture exposes the common
# hook payload shape without launching a real harness.
cat > "$RUN_ROOT/collars/identity.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fresh-identity"
GANG_RESUME_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' resume-{{session_id}}"
collar_session_id() {
  [ ! -e "$RUN_ROOT/identity-stderr-on" ] || printf 'COLLAR-IDENTITY-STDERR\n' >&2
  printf '%s' "\$2" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session_id"])'
}
SH
"$HITCH" identity -c identity -d /tmp >/dev/null
identity_id="$(window_id identity)"
identity_pane="$(tmux list-panes -t "$identity_id" -F '#{pane_id}')"
: > "$RUN_ROOT/identity-stderr-on"
printf '%s' '{"hook_event_name":"Stop","session_id":"native-identity-123"}' \
  | TMUX_PANE="$identity_pane" "$GANG" hook \
    2> "$RUN_ROOT/identity-hook.err"
rm -f -- "$RUN_ROOT/identity-stderr-on"
equal "a hook suppresses collar identity diagnostics" "" \
  "$(<"$RUN_ROOT/identity-hook.err")"
equal "the first native hook stamps its exact harness session id" \
  "native-identity-123" \
  "$(tmux show-options -wqv -t "$identity_id" @gl_session_id)"
equal "hitch records the window's Gangline agent identity" "identity" \
  "$(tmux show-options -wqv -t "$identity_id" @gl_agent)"

# Codex consumes that same native payload directly. This registered window has
# the aborted-hitch/adopt shape: no startup nonce and no rollout binding. The
# first hook must stamp both facts without a sessions-tree search.
codex_payload_file="$RUN_ROOT/codex-payload-session.jsonl"
cat > "$codex_payload_file" <<'JSONL'
{"type":"session_meta","payload":{"id":"codex-payload-123"}}
{"payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":60000},"model_context_window":300000}}}
JSONL
codex_payload_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n codex-payload "PS1='❯ ' bash --norc")"
tmux set-option -w -t "$codex_payload_id" @gl_agent codex-payload
tmux set-option -w -t "$codex_payload_id" @gl_collar codex
codex_payload_pane="$(tmux list-panes -t "$codex_payload_id" -F '#{pane_id}')"
equal "the codex payload fixture begins with no native session binding" "|" \
  "$(tmux show-options -wqv -t "$codex_payload_id" @gl_session_id)|$(tmux show-options -wqv -t "$codex_payload_id" @gl_session)"
printf '%s' \
  "{\"hook_event_name\":\"Stop\",\"session_id\":\"codex-payload-123\",\"transcript_path\":\"$codex_payload_file\"}" \
  | TMUX_PANE="$codex_payload_pane" "$GANG" hook
equal "a first Codex hook stamps an agent that has no startup nonce" \
  "codex-payload-123" \
  "$(tmux show-options -wqv -t "$codex_payload_id" @gl_session_id)"
equal "the Codex hook binds its native transcript path for context" \
  "$codex_payload_file" \
  "$(tmux show-options -wqv -t "$codex_payload_id" @gl_session)"
equal "Codex context uses the hook-bound transcript without a nonce" \
  "60k/300k (20%)" "$($GANG context codex-payload)"
"$GANG" drop codex-payload >/dev/null

# Calibrate the parser's failure direction: a native-looking payload with the
# identity field absent must remain UNSTAMPED rather than borrowing any other
# hook or cwd fact.
codex_missing_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n codex-missing-id "PS1='❯ ' bash --norc")"
tmux set-option -w -t "$codex_missing_id" @gl_agent codex-missing-id
tmux set-option -w -t "$codex_missing_id" @gl_collar codex
codex_missing_pane="$(tmux list-panes -t "$codex_missing_id" -F '#{pane_id}')"
printf '%s' \
  "{\"hook_event_name\":\"Stop\",\"transcript_path\":\"$codex_payload_file\"}" \
  | TMUX_PANE="$codex_missing_pane" "$GANG" hook
equal "a Codex payload missing session_id calibrates to UNSTAMPED" "" \
  "$(tmux show-options -wqv -t "$codex_missing_id" @gl_session_id)"
equal "a rejected Codex payload does not half-bind its transcript" "" \
  "$(tmux show-options -wqv -t "$codex_missing_id" @gl_session)"
"$GANG" drop codex-missing-id >/dev/null

identity_report="$(TMUX_PANE="$identity_pane" "$GANG" whoami)"
contains "whoami names the agent" "$identity_report" "agent: identity"
contains "whoami names the pane" "$identity_report" "pane: $identity_pane"
contains "whoami names the collar" "$identity_report" "collar: identity"
contains "whoami names the stamped native session" "$identity_report" \
  "harness session id: native-identity-123"
contains "whoami names the latest live payload id" "$identity_report" \
  "live payload id: native-identity-123"
contains "whoami names the team" "$identity_report" "team session: $GANG_SESSION"

self_before="$(pane identity)"
refuses "a self-send is refused under the intent it violates" \
  "a message that returns to its author is never intended" \
  bash -c 'printf SELF_RETURN | TMUX_PANE="$1" "$2" send --to identity --stdin' \
  fixture "$identity_pane" "$GANG"
equal "a refused self-send types nothing" "$self_before" "$(pane identity)"

tmux set-option -w -t "$identity_id" @gl_session_id bogus-native-id
printf '%s' '{"hook_event_name":"PostToolUse","session_id":"native-identity-123"}' \
  | TMUX_PANE="$identity_pane" "$GANG" hook
contains "status exposes a native session identity mismatch" \
  "$("$GANG" status identity)" \
  "stamped 'bogus-native-id' != live 'native-identity-123'"
contains "roster exposes a native session identity mismatch" \
  "$("$GANG" roster)" "identity-mismatch="
mismatch_before="$(pane alpha)"
refuses "a mismatched pane cannot send under the window's authority" \
  "identity mismatch" bash -c \
  'printf MISMATCH_SPOKE | TMUX_PANE="$1" "$2" send --to alpha --stdin' \
  fixture "$identity_pane" "$GANG"
equal "the mismatch refusal types nothing into its target" \
  "$mismatch_before" "$(pane alpha)"
tmux set-option -w -t "$identity_id" @gl_session_id native-identity-123
printf '%s' '{"hook_event_name":"Stop","session_id":"native-identity-123"}' \
  | TMUX_PANE="$identity_pane" "$GANG" hook
excludes "a matching next hook repairs the visible mismatch" \
  "$("$GANG" roster)" "identity-mismatch="

drop_identity_out="$("$GANG" drop identity)"
contains "drop prints the parting native session id before destroying its window" \
  "$drop_identity_out" "session id: native-identity-123"
contains "drop prints the exact explicit-id relaunch line" "$drop_identity_out" \
  "gang hitch identity --resume native-identity-123"
"$GANG" hitch identity -c identity -d "$RUN_ROOT" \
  --resume native-identity-123 >/dev/null
contains "explicit resume substitutes the quoted identity from any cwd" \
  "$(tmux display-message -p -t "$(window_id identity)" '#{pane_start_command}')" \
  "resume-native-identity-123"
"$GANG" drop identity >/dev/null
refuses "bare resume without a surviving stamped window refuses loudly" \
  "gang hitch identity --resume <session-id>" \
  "$GANG" hitch identity -c identity -d /tmp --resume
equal "a refused stamp-less resume launches nothing" "" "$(window_id identity)"

"$HITCH" survivor -c identity -d /tmp >/dev/null
survivor_id="$(window_id survivor)"
survivor_pane="$(tmux list-panes -t "$survivor_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"Stop","session_id":"surviving-native-id"}' \
  | TMUX_PANE="$survivor_pane" "$GANG" hook
refuses "explicit resume refuses a different session over a stamped window" \
  "stamped for harness session 'surviving-native-id', not requested session 'other-native-id'" \
  "$GANG" hitch survivor -c identity -d /tmp --resume other-native-id
tmux set-option -w -t "$survivor_id" remain-on-exit on
survivor_dead="test-survivor-dead-$$"
printf -v survivor_exit 'trap %q EXIT; exit' "tmux wait-for -S $survivor_dead"
tmux send-keys -l -t "$survivor_id" "$survivor_exit"
tmux send-keys -t "$survivor_id" Enter
tmux wait-for "$survivor_dead"
"$HITCH" survivor -c identity -d /tmp --resume >/dev/null
contains "bare resume reads the stamp from a surviving dead window" \
  "$(tmux display-message -p -t "$survivor_id" '#{pane_start_command}')" \
  "resume-surviving-native-id"
"$GANG" drop survivor >/dev/null

unadopted_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n unadopted "PS1='❯ ' bash --norc")"
unadopted_pane="$(tmux list-panes -t "$unadopted_id" -F '#{pane_id}')"
unadopted_target_before="$(pane alpha)"
refuses "an unadopted pane cannot send under its bare window name" \
  "no registered Gangline agent/collar identity" bash -c \
  'printf IDENTITY_ABSENT | TMUX_PANE="$1" "$2" send --to alpha --stdin' \
  fixture "$unadopted_pane" "$GANG"
equal "identity absence types nothing into the claimed target" \
  "$unadopted_target_before" "$(pane alpha)"
tmux kill-window -t "$unadopted_id"

residue_recovery_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n residue-recovery "PS1='❯ ' bash --norc")"
residue_recovery_pane="$(tmux list-panes -t "$residue_recovery_id" -F '#{pane_id}')"
tmux set-option -w -t "$residue_recovery_id" @gl_collar identity
residue_target_before="$(pane residue-recovery)"
if residue_target_out="$(printf RESIDUE_TARGET_BODY | "$GANG" send \
    --to residue-recovery --from tester --live-only --stdin 2>&1)"; then
  fail "collar residue without registration is not a send target" \
    "send unexpectedly succeeded"
else
  contains "the unregistered target refusal names adoption as the repair" \
    "$residue_target_out" "not a registered Gangline agent"
fi
# source-guard: whole-surface@f8a9746a76b8: a refused target must leave every visible pane byte unchanged regardless of producer
equal "the unregistered target receives no message bytes" \
  "$residue_target_before" "$(pane residue-recovery)"
refuses "collar residue without @gl_agent cannot send under a bare window name" \
  "no registered Gangline agent/collar identity" bash -c \
  'printf RESIDUE_BEFORE_ADOPT | TMUX_PANE="$1" "$2" send --to alpha --stdin' \
  fixture "$residue_recovery_pane" "$GANG"
if residue_adopt_out="$("$GANG" adopt residue-recovery -c identity 2>&1)"; then
  pass "the deliberate adopt remedy claims a collar-residue window"
else
  fail "the deliberate adopt remedy claims a collar-residue window" \
    "$residue_adopt_out"
fi
equal "adopt stamps the claimed residue window with its named identity" \
  "residue-recovery" \
  "$(tmux show-options -wqv -t "$residue_recovery_id" @gl_agent)"
if printf 'RESIDUE_AFTER_ADOPT' | TMUX_PANE="$residue_recovery_pane" \
    "$GANG" send --to alpha --stdin >/dev/null 2>&1; then
  pass "the named adopt remedy resolves the send refusal"
else
  fail "the named adopt remedy resolves the send refusal" \
    "send still refused after deliberate adoption"
fi
contains "the recovered pane's attributed message reaches its peer" \
  "$(pane alpha)" "RESIDUE_AFTER_ADOPT"
excludes "the pre-adoption refused body never reached the peer" \
  "$(pane alpha)" "RESIDUE_BEFORE_ADOPT"
"$GANG" drop residue-recovery >/dev/null

residue_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n displaced "PS1='❯ ' bash --norc")"
tmux set-option -w -t "$residue_id" @gl_agent lead
refuses "hitch refuses a target window registered to another identity" \
  "registered to Gangline agent 'lead'" \
  "$GANG" hitch displaced -c identity -d /tmp
refuses "adopt refuses a window registered to another Gangline identity" \
  "registered to Gangline agent 'lead'" "$GANG" adopt displaced -c bash
contains "the mismatch refusal names the requested window identity too" \
  "$("$GANG" adopt displaced -c bash 2>&1 || true)" "displaced"
tmux kill-window -t "$residue_id"

# IDENTITY IS THE REGISTRATION, NOT THE TITLE. tmux lets anyone rename a window,
# and gang paints its own state glyph into that title out of @gl_agent, so a
# resolver that matches on #W can be aimed at another lane's harness by a rename
# alone — and would paste, submit, VERIFY and report success against it. The
# registration is what hitch and adopt wrote; the title is decoration.
"$HITCH" registered-name -c bash -d /tmp >/dev/null
registered_name_id="$(window_id registered-name)"
tmux rename-window -t "$registered_name_id" borrowed-title
refuses "a window title cannot address the agent registered under another name" \
  "registered to Gangline agent 'registered-name'" \
  bash -c 'printf TITLE_SPOOF | "$1" send --to borrowed-title --from tester --stdin' \
  fixture "$GANG"
excludes "the title-addressed message never reached the registered harness" \
  "$(tmux capture-pane -pJ -t "$registered_name_id")" "TITLE_SPOOF"
if printf 'REGISTERED_REACHES' |
    "$GANG" send --to registered-name --from tester --stdin >/dev/null 2>&1; then
  pass "the registered identity still resolves through a changed window title"
else
  fail "the registered identity still resolves through a changed window title" \
    "send refused the agent under its own registered name"
fi
contains "and that message reaches the registered harness" \
  "$(tmux capture-pane -pJ -t "$registered_name_id")" "REGISTERED_REACHES"
equal "the agent's own name is its registration, whatever the title says" \
  "registered-name" \
  "$(TMUX_PANE="$(tmux list-panes -t "$registered_name_id" -F '#{pane_id}')" \
    "$GANG" whoami | sed -n 's/^agent: //p')"
# Teardown, not an assertion: a resolver that cannot find this window by its
# registration is exactly the defect above, and the fixture still has to go.
"$GANG" drop registered-name >/dev/null 2>&1 \
  || tmux kill-window -t "$registered_name_id"

# A REGISTRATION IS RAW BYTES, AND IT IS NOW THE TRUSTED IDENTITY. tmux escapes
# control characters out of a window TITLE — #W reads \033 back as six literal
# characters — so the title was never a way to paint somebody's terminal. An
# option value is returned exactly as written, so reading identity from
# @gl_agent opened that door. Names go out sanitized; matching keeps the bytes.
ctl_name="$(printf 'ctl\033[31mrogue')"
ctl_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n ctl-rogue "PS1='❯ ' bash --norc")"
ctl_pane="$(tmux list-panes -t "$ctl_id" -F '#{pane_id}')"
tmux set-option -w -t "$ctl_id" @gl_agent "$ctl_name"
tmux set-option -w -t "$ctl_id" @gl_collar bash
ctl_escapes() { case "$1" in *$'\033'*) printf raw ;; *) printf clean ;; esac; }
ctl_whoami="$(TMUX_PANE="$ctl_pane" "$GANG" whoami 2>&1)" || true
ctl_mail="$(TMUX_PANE="$ctl_pane" "$GANG" mail 2>&1)" || true
ctl_drop="$("$GANG" drop "$ctl_name" 2>&1)" || true
equal "a control-bearing registration never reaches the terminal raw" \
  "clean clean clean" \
  "$(ctl_escapes "$ctl_whoami") $(ctl_escapes "$ctl_mail") $(ctl_escapes "$ctl_drop")"
tmux kill-window -t "$ctl_id" 2>/dev/null || true

# A synchronous tty fixture paints the captured Codex menu and records every
# key Gangline sends. Each mutant changes one load-bearing observation.
cat > "$RUN_ROOT/dialog-fixture.py" <<'PY'
#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
import os
import subprocess
import sys
import tty

variant = os.environ.get("DIALOG_VARIANT", "known")
log_path = os.environ["DIALOG_KEY_LOG"]
ready = os.environ["DIALOG_READY"]
selected = 0
composer = False
draft = bytearray()
answered = 0
safe_index = 1
selected_glyph = "›"
external_frame = None
body = [
    "Our systems are thinking a bit more about this request before responding.",
    "Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.",
]
labels = ["Retry with a faster model", "Dismiss and keep waiting", "Learn more"]
footer = "No action is required. Codex will keep waiting, and this menu will close when the response is ready."
if variant == "trust":
    body = [
        "> You are in /tmp/fixture-cwd",
        "Do you trust the contents of this directory? Working with untrusted contents comes with higher risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec policies to load.",
    ]
    labels = ["Yes, continue", "No, quit"]
    footer = "Press enter to continue"
    safe_index = 0
if variant == "external-import":
    with open(os.environ["DIALOG_CAPTURE"], encoding="utf-8") as stream:
        external_frame = stream.read().splitlines()
    labels = ["Yes, allow external imports", "No, disable external imports"]
    safe_index = 0
    selected_glyph = "❯"
if variant == "changed-byte":
    body[0] = body[0].replace("systems", "system")
if variant == "reordered":
    body[0], body[1] = body[1], body[0]
if variant == "authority":
    body = ["Permission required before this command can run.", "Do you want to allow full access?"]
    labels = ["Allow", "Deny", "Learn more"]
    footer = "Choose whether to approve this permission."
if variant == "extra-option":
    labels.insert(2, "Open diagnostics")

def paint():
    sys.stdout.write("\x1b[2J\x1b[H")
    if external_frame is not None:
        for line in external_frame:
            print(line)
        sys.stdout.flush()
        return
    for line in body:
        print("  " + line)
    print()
    for index, label in enumerate(labels):
        glyph = selected_glyph if index == selected or (variant == "two-glyph" and index == 1) else " "
        print(f"{glyph} {index + 1}. {label}")
    print()
    print("  " + footer)
    sys.stdout.flush()

def record(key):
    with open(log_path, "a", encoding="utf-8") as stream:
        stream.write(key + "\n")

tty.setraw(sys.stdin.fileno())
paint()
subprocess.run(["tmux", "wait-for", "-S", ready], check=True)
while True:
    char = os.read(sys.stdin.fileno(), 1)
    if composer:
        if char in (b"\r", b"\n"):
            sys.stdout.write("\r\n" + draft.decode("utf-8") + "\r\n❯ ")
            sys.stdout.flush()
            draft.clear()
        else:
            draft.extend(char)
            os.write(sys.stdout.fileno(), char)
        continue
    if char == b"\x1b":
        tail = os.read(sys.stdin.fileno(), 2)
        if tail == b"[B":
            record("Down")
            if variant != "wrong-move":
                selected = min(selected + 1, len(labels) - 1)
            paint()
        elif tail == b"[A":
            record("Up")
            if variant != "wrong-move":
                selected = max(selected - 1, 0)
            paint()
    elif char in (b"\r", b"\n"):
        record("Enter")
        if selected == safe_index and variant != "confirm-stuck":
            answered += 1
            if variant == "recurring" and answered < 5:
                recur_fifo = os.environ.get("DIALOG_RECUR_FIFO")
                sys.stdout.write("\x1b[2J\x1b[H❯ ")
                sys.stdout.flush()
                if recur_fifo:
                    with open(recur_fifo, "rb", buffering=0) as stream:
                        stream.read(1)
                else:
                    subprocess.run(
                        ["tmux", "wait-for", os.environ["DIALOG_RECUR_SIGNAL"]],
                        check=True,
                    )
                selected = 0
                paint()
                recur_ack = os.environ.get("DIALOG_RECUR_ACK")
                if recur_ack:
                    with open(recur_ack, "wb", buffering=0) as stream:
                        stream.write(b"x")
            else:
                composer = True
                sys.stdout.write("\x1b[2J\x1b[H❯ ")
                sys.stdout.flush()
        else:
            paint()
PY
chmod +x "$RUN_ROOT/dialog-fixture.py"
dialog_recurring_signal="dialog-recurring-next-$$"

cat > "$RUN_ROOT/collars/dialog.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' dialog"
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter'
GANG_DIALOG_LINES_safety_buffering_prompt='Our systems are thinking a bit more about this request before responding.
Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.
Retry with a faster model
Dismiss and keep waiting
Learn more
No action is required. Codex will keep waiting, and this menu will close when the response is ready.'
SH
cat > "$RUN_ROOT/collars/dialog-wrong.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Up|Enter'
SH
cat > "$RUN_ROOT/collars/dialog-recurring.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_LAUNCH="env DIALOG_VARIANT=recurring DIALOG_KEY_LOG='$RUN_ROOT/dialog-recurring.keys' DIALOG_READY='dialog-recurring-ready-$$' DIALOG_RECUR_SIGNAL='$dialog_recurring_signal' '$RUN_ROOT/dialog-fixture.py'"
SH
cat > "$RUN_ROOT/collars/dialog-observe.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_DIALOGS='operator-choice|^› [0-9]+\. |||'
GANG_DIALOG_LINES_operator_choice="\$GANG_DIALOG_LINES_safety_buffering_prompt"
SH
observe_boot_seen="dialog-observe-boot-seen-$$"
observe_boot_release="dialog-observe-boot-release-$$"
cat > "$RUN_ROOT/collars/dialog-observe-boot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
unset -f collar_input
GANG_LAUNCH="env DIALOG_VARIANT=known DIALOG_KEY_LOG='$RUN_ROOT/dialog-observe-boot.keys' DIALOG_READY='dialog-observe-boot-ready-$$' '$RUN_ROOT/dialog-fixture.py'"
SH
cat > "$RUN_ROOT/collars/dialog-leading-space.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
GANG_DIALOGS='dead-record|^ +› [0-9]+\. |||'
GANG_DIALOG_LINES_dead_record="\$GANG_DIALOG_LINES_operator_choice"
SH
cat > "$RUN_ROOT/collars/dialog-single-space.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
GANG_DIALOGS='dead-record|^ › [0-9]+\. |||'
GANG_DIALOG_LINES_dead_record="\$GANG_DIALOG_LINES_operator_choice"
SH
cat > "$RUN_ROOT/collars/dialog-class-space.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
GANG_DIALOGS='dead-record|^[[:space:]]› [0-9]+\. |||'
GANG_DIALOG_LINES_dead_record="\$GANG_DIALOG_LINES_operator_choice"
SH
cat > "$RUN_ROOT/collars/dialog-optional-space.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
GANG_DIALOGS='optional-space|^ *› [0-9]+\. |||'
GANG_DIALOG_LINES_optional_space="\$GANG_DIALOG_LINES_operator_choice"
SH
# shellcheck disable=SC2154  # set in test/integration-cli.sh
printf -v claude_external_record_q '%q' "$claude_external_record"
# shellcheck disable=SC2154  # set in test/integration-cli.sh
printf -v claude_external_lines_q '%q' "$claude_external_lines"
cat > "$RUN_ROOT/collars/dialog-claude-external.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_DIALOGS=$claude_external_record_q
GANG_DIALOG_LINES_external_import_trust=$claude_external_lines_q
SH
mkdir -p "$RUN_ROOT/dialog-observe-bin"
cat > "$RUN_ROOT/dialog-observe-bin/sleep" <<SH
#!/bin/sh
if [ ! -e "$RUN_ROOT/dialog-observe-sleep-seen" ]; then
  : > "$RUN_ROOT/dialog-observe-sleep-seen"
  printf '%s' "\$1" > "$RUN_ROOT/dialog-observe-sleep-argument"
  tmux wait-for -S "$observe_boot_seen"
  tmux wait-for "$observe_boot_release"
fi
exit 0
SH
chmod +x "$RUN_ROOT/dialog-observe-bin/sleep"

for dead_marker_case in leading-space single-space class-space; do
  if dead_marker_out="$("$GANG" hitch "dialog-$dead_marker_case" \
      -c "dialog-$dead_marker_case" -d /tmp 2>&1)"; then
    fail "$dead_marker_case dialog marker dead after normalization is refused" \
      "hitch unexpectedly succeeded"
  else
    contains "$dead_marker_case refusal names normalization" \
      "$dead_marker_out" "requires leading whitespace that dialog normalization removes"
  fi
  equal "$dead_marker_case dead marker opens no window" "" \
    "$(window_id "dialog-$dead_marker_case")"
done
if "$HITCH" dialog-optional-space -c dialog-optional-space -d /tmp \
    >/dev/null; then
  pass "a marker allowing zero leading spaces remains loadable"
else
  fail "a marker allowing zero leading spaces remains loadable" \
    "hitch refused a marker that matches normalized input"
fi
"$GANG" drop dialog-optional-space >/dev/null

dialog_start() { # $1 agent, $2 variant, $3 collar
  local name="$1" variant="$2" collar="$3" id command
  "$HITCH" "$name" -c "$collar" -d /tmp >/dev/null
  id="$(window_id "$name")"
  case "$name" in dialog-known) tmux resize-window -t "$id" -x 48 -y 24 ;; esac
  : > "$RUN_ROOT/$name.keys"
  printf -v command 'DIALOG_VARIANT=%q DIALOG_KEY_LOG=%q DIALOG_READY=%q DIALOG_CAPTURE=%q %q' \
    "$variant" "$RUN_ROOT/$name.keys" "dialog-ready-$name-$$" \
    "$ROOT/test/fixtures/claude-external-import.txt" "$RUN_ROOT/dialog-fixture.py"
  tmux send-keys -l -t "$id" "$command"
  tmux send-keys -t "$id" Enter
  tmux wait-for "dialog-ready-$name-$$"
}

dialog_start dialog-known known dialog
dialog_known_err="$RUN_ROOT/dialog-known.err"
if printf 'DIALOG_BODY_REACHED' | "$GANG" send --to dialog-known --from tester \
    --live-only --stdin >/dev/null 2>"$dialog_known_err"; then
  pass "a fully fingerprinted known dialog is auto-answered"
else
  fail "a fully fingerprinted known dialog is auto-answered" \
    "$(<"$dialog_known_err")"
fi
contains "the full-block match survives a narrow soft-wrapped pane and sends" \
  "$(pane dialog-known)" "DIALOG_BODY_REACHED"
contains "known-dialog triage names the exact registry id" \
  "$(<"$dialog_known_err")" "safety-buffering-prompt"
equal "known-dialog triage moves once and confirms only after verification" \
  $'Down\nEnter' "$(<"$RUN_ROOT/dialog-known.keys")"
"$GANG" drop dialog-known >/dev/null

for mutant in extra-option reordered changed-byte two-glyph authority; do
  dialog_start "dialog-$mutant" "$mutant" dialog
  mutant_before="$(pane "dialog-$mutant")"
  mutant_keys="$(<"$RUN_ROOT/dialog-$mutant.keys")"
  if printf "MUTANT_$mutant" | "$GANG" send --to "dialog-$mutant" \
      --from tester --live-only --stdin >/dev/null 2>&1; then
    fail "$mutant known-dialog mutant is refused" "send unexpectedly succeeded"
  else
    pass "$mutant known-dialog mutant is refused"
  fi
  equal "$mutant mutant leaves the dialog byte-for-byte on screen" \
    "$mutant_before" "$(pane "dialog-$mutant")"
  equal "$mutant mutant receives no auto-answer key" "$mutant_keys" \
    "$(<"$RUN_ROOT/dialog-$mutant.keys")"
  "$GANG" drop "dialog-$mutant" >/dev/null
done

dialog_start dialog-wrong wrong-move dialog-wrong
if wrong_out="$(printf WRONG_SELECTION | "$GANG" send --to dialog-wrong \
    --from tester --live-only --stdin 2>&1)"; then
  fail "confirm never fires while the glyph is on a non-safe row" \
    "send unexpectedly succeeded"
else
  contains "the wrong-selection refusal names the dialog" \
    "$wrong_out" "safety-buffering-prompt"
  contains "the wrong-selection refusal points at direct inspection" \
    "$wrong_out" "gang attach"
fi
equal "the wrong-selection mutant records its move but no confirm" \
  "Up" "$(<"$RUN_ROOT/dialog-wrong.keys")"
"$GANG" drop dialog-wrong >/dev/null

dialog_start dialog-confirm-stuck confirm-stuck dialog
if stuck_out="$(printf STUCK_CONFIRM | "$GANG" send --to dialog-confirm-stuck \
    --from tester --live-only --stdin 2>&1)"; then
  fail "a confirm key that does not clear the dialog fails loud" \
    "send unexpectedly succeeded"
else
  contains "the uncleared-dialog refusal names the registry id" \
    "$stuck_out" "safety-buffering-prompt"
  contains "the uncleared-dialog refusal points at gang attach" \
    "$stuck_out" "gang attach"
fi
equal "the stuck-confirm fixture received only the verified move and confirm" \
  $'Down\nEnter' "$(<"$RUN_ROOT/dialog-confirm-stuck.keys")"
"$GANG" drop dialog-confirm-stuck >/dev/null

dialog_start dialog-status known dialog
status_dialog_before="$(pane dialog-status)"
contains "status names a known transient without answering it" \
  "$("$GANG" status dialog-status)" \
  "!occupied! (known transient: safety-buffering-prompt)"
equal "status is read-only even for a recognized dialog" \
  "$status_dialog_before" "$(pane dialog-status)"
equal "status presses no dialog key" "" "$(<"$RUN_ROOT/dialog-status.keys")"
"$GANG" drop dialog-status >/dev/null

dialog_start dialog-observe known dialog-observe
observe_before="$(pane dialog-observe)"
contains "status names a recognized operator dialog without calling it transient" \
  "$("$GANG" status dialog-observe)" \
  "!occupied! (known operator dialog: operator-choice)"
if observe_send="$(printf OBSERVE_ONLY_BODY | "$GANG" send \
    --to dialog-observe --from tester --live-only --stdin 2>&1)"; then
  fail "an observe-only dialog refuses delivery" "send unexpectedly succeeded"
else
  contains "the refusal names the operator decision Gangline will not make" \
    "$observe_send" "known operator dialog 'operator-choice'"
fi
equal "observe-only recognition sends no key" "" \
  "$(<"$RUN_ROOT/dialog-observe.keys")"
# source-guard: whole-surface@f2d2c71674d2: no visible byte may change because observe-only recognition is forbidden to send any key
equal "observe-only recognition leaves the operator dialog unchanged" \
  "$observe_before" "$(pane dialog-observe)"
"$GANG" drop dialog-observe >/dev/null
