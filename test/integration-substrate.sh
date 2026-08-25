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
  '7bcb73df5cc4fc1836ff62846464f5d5b530ccaa074b84ad22951c6f6775c7fd'

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

# 2.0 retired the composer-driven usage page. The verb refuses and names the
# structured command that replaced it, rather than failing as an unknown word.
refuses "the retired usage verb names its replacement" \
  "'gang limits' reads the same quota" "$GANG" usage alpha
excludes "the retired usage verb types nothing into its former target" \
  "$(pane alpha)" "/usage"

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
# A COMMAND THAT TAKES ONE AGENT NAME DEFAULTS TO THE CALLER. Bare flush and
# interrupt inside an agent window therefore resolve to that agent rather than
# printing a synopsis, and each then refuses on something the fixture's own
# collar does not declare — which is the evidence that a target was resolved at
# all. drop, down, adopt, hitch and send's recipient stay without a self default
# on purpose and remain in the loop above.
for self_bare in flush:GANG_QUEUED_REGEX interrupt:GANG_INTERRUPT_KEY; do
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

# ADOPTION NAMES A HARNESS ALREADY RUNNING IN THE WINDOW. A held corpse still
# has a window name, and before this precondition adopt registered that empty
# window, minted its spool, and reported it as an agent. The pane's own fifo is
# the death barrier: EOF settles process exit before the immediate tmux read.
mkfifo "$RUN_ROOT/adopt-dead.fifo"
adopt_dead_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n adopt-dead \
  "exec 9<>$RUN_ROOT/adopt-dead.fifo; PS1='❯ ' exec bash --norc")"
adopt_dead_pane="$(tmux list-panes -t "$adopt_dead_id" -F '#{pane_id}')"
exec 3<"$RUN_ROOT/adopt-dead.fifo"
tmux set-option -w -t "$adopt_dead_id" remain-on-exit on
tmux send-keys -l -t "$adopt_dead_pane" 'exit 23'
tmux send-keys -t "$adopt_dead_pane" Enter
cat <&3 >/dev/null
exec 3<&-
tmux run-shell true >/dev/null
equal "the refused-adopt fixture has no running pane" 1 \
  "$(tmux display-message -p -t "$adopt_dead_id" '#{pane_dead}')"
adopt_dead_before="$(tmux show-options -wv -t "$adopt_dead_id")"
refuses "adopt refuses a window where no harness is running" \
  "nothing is running in window 'adopt-dead'" \
  "$GANG" adopt adopt-dead -c bash
equal "refusing a dead window leaves every window option unchanged" \
  "$adopt_dead_before" \
  "$(tmux show-options -wv -t "$adopt_dead_id")"
tmux kill-window -t "$adopt_dead_id"

# A REFUSAL MUST PRECEDE ADOPTION'S FIRST MUTATION. These settings are stamped
# only after the window becomes an agent, but their parsers can reject the
# operator's configuration. Rejecting one after registration says adoption
# failed while leaving a named agent and a fresh spool identity behind.
adopt_invalid_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n adopt-invalid "PS1='❯ ' bash --norc")"
equal "the refused-adopt fixture is a live unregistered window" \
  "$adopt_invalid_id|0" \
  "$(tmux display-message -p -t "$adopt_invalid_id" '#{window_id}|#{pane_dead}')"
adopt_invalid_before="$(tmux show-options -wv -t "$adopt_invalid_id")"
refuses "adopt validates provider-usage lights before changing the window" \
  "GANG_USAGE_LIGHTS must increase from yellow to red" \
  env GANG_USAGE_LIGHTS=95%,90% "$GANG" adopt adopt-invalid -c bash
equal "a refused provider-usage setting leaves every window option unchanged" \
  "$adopt_invalid_before" \
  "$(tmux show-options -wv -t "$adopt_invalid_id")"
refuses "adopt validates automatic resume before changing the window" \
  "GANG_AUTO_RESUME must be off or one percentage" \
  env GANG_AUTO_RESUME=90 "$GANG" adopt adopt-invalid -c bash
equal "a refused automatic-resume setting leaves every window option unchanged" \
  "$adopt_invalid_before" \
  "$(tmux show-options -wv -t "$adopt_invalid_id")"
tmux kill-window -t "$adopt_invalid_id"

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

# Adoption establishes identity; it is not a state query. A collar whose pane
# reader refuses must still be adoptable, with unknown as the honest initial
# glyph, while the explicit status command keeps failing loudly on that reader.
cat > "$RUN_ROOT/collars/adopt-unreadable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_input() {
  : > "$RUN_ROOT/adopt-state-reader-called"
  return 3
}
SH
adopt_unreadable_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n adopt-unreadable "PS1='❯ ' bash --norc")"
adopt_unreadable_rc=0
"$GANG" adopt adopt-unreadable -c adopt-unreadable >/dev/null 2>&1 \
  || adopt_unreadable_rc=$?
equal "adoption does not fail on an unrelated state observation" \
  0 "$adopt_unreadable_rc"
equal "the adopted identity is complete despite its unreadable state" \
  "adopt-unreadable|adopt-unreadable" \
  "$(tmux show-options -wqv -t "$adopt_unreadable_id" @gl_agent)|$(tmux show-options -wqv -t "$adopt_unreadable_id" @gl_collar)"
equal "adoption does not call the collar's state reader" absent \
  "$([ ! -e "$RUN_ROOT/adopt-state-reader-called" ] && printf absent || printf called)"
equal "an adopted window with no state observation starts unknown" \
  "?adopt-unreadable?" \
  "$(tmux display-message -p -t "$adopt_unreadable_id" '#{window_name}')"
refuses "an explicit state observation still fails loudly" \
  "cannot read the composer" "$GANG" status adopt-unreadable
"$GANG" drop adopt-unreadable >/dev/null

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

# A CONTRADICTED STAMP IS NOT SOMETHING TO QUOTE. Sends from such a pane
# already fail closed; a parting line offering --resume <that id> makes the
# same claim in the place an operator acts on it.
"$HITCH" contradicted -c identity -d /tmp >/dev/null
contradicted_id="$(window_id contradicted)"
tmux set-option -w -t "$contradicted_id" @gl_session_id stamped-identity-1
tmux set-option -w -t "$contradicted_id" @gl_session_mismatch \
  "stamped 'stamped-identity-1' != live 'live-identity-2'"
contradicted_out="$("$GANG" drop contradicted)"
contains "a contradicted stamp parts as the mismatch it is" \
  "$contradicted_out" "MISMATCH (stamped 'stamped-identity-1' != live 'live-identity-2')"
excludes "not as a relaunch into a conversation it is not running" \
  "$contradicted_out" "--resume stamped-identity-1"

"$HITCH" survivor -c identity -d /tmp >/dev/null
survivor_id="$(window_id survivor)"
survivor_pane="$(tmux list-panes -t "$survivor_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"Stop","session_id":"surviving-native-id"}' \
  | TMUX_PANE="$survivor_pane" "$GANG" hook
refuses "explicit resume refuses a different session over a stamped window" \
  "stamped for harness session 'surviving-native-id', not requested session 'other-native-id'" \
  "$GANG" hitch survivor -c identity -d /tmp --resume other-native-id
tmux set-option -w -t "$survivor_id" remain-on-exit on
# A BARRIER THAT NEEDS THE TMUX SERVER CANNOT REPORT ON THE TMUX SERVER. What
# this waits for is a pane's shell reaching its exit, and it used to learn that
# through `tmux wait-for -S` in that shell's EXIT trap: a client call into the
# very server whose responsiveness is not the thing under test. Observed
# 2026-08-24, that call sat for twenty-five minutes against a server busy
# elsewhere, and because the gate serialises on one lock it held every other run
# behind it. `tmux wait-for` has no timeout, so the run could not go red on it —
# only quiet, which is the worse of the two.
#
# The signal now goes down a pipe this run owns. The trap writes one byte from
# the pane's own shell, and the read below returns on that byte, so neither side
# of the barrier is a tmux client and a busy server cannot swallow it. It is
# still an event and not a clock: the read blocks on the byte exactly as the
# channel blocked on the signal, and no wall time is spent either way.
survivor_dead="$RUN_ROOT/survivor-dead"
mkfifo "$survivor_dead"
printf -v survivor_exit 'trap %q EXIT; exit' "printf x > $survivor_dead"
tmux send-keys -l -t "$survivor_id" "$survivor_exit"
tmux send-keys -t "$survivor_id" Enter
# A WRITER THAT OPENED AND CLOSED WITHOUT WRITING IS NOT A SHELL THAT REACHED
# ITS TRAP, and under set -e an empty read would end the run rather than say so.
survivor_signal=""
survivor_read_rc=0
IFS= read -r -n 1 survivor_signal < "$survivor_dead" || survivor_read_rc=$?
equal "the surviving window announced its own exit before it was read back" \
  "0 x" "$survivor_read_rc $survivor_signal"
"$HITCH" survivor -c identity -d /tmp --resume >/dev/null
contains "bare resume reads the stamp from a surviving dead window" \
  "$(tmux display-message -p -t "$survivor_id" '#{pane_start_command}')" \
  "resume-surviving-native-id"
"$GANG" drop survivor >/dev/null

# A BARRIER NOBODY SIGNALS MUST GO RED, NOT QUIET. Every barrier above learns
# its event from `tmux wait-for`, a client call with no timeout: a signal that
# is sent and never arrives used to park the whole run, print nothing, and hold
# the host's heavy test lock until somebody read a process list. Converting the
# call sites one at a time fixes the ones converted; bounding the wait fixes the
# ones nobody has looked at yet, which on the two occasions this happened is
# where it happened. test/integration.sh puts that ceiling on a tmux shim in the
# bin directory this run already owns and already leads PATH with.
#
# THIS IS THE ONE CHECK HERE WHOSE SUBJECT IS A TIMEOUT, so its clock is scaled
# rather than stopped: stopping it would assert that an expiry which never
# happens reports correctly. THE MARGIN, MEASURED 2026-08-24 on a loaded box:
#
#   signalled barrier, waiter already blocked   ~10ms
#   this fixture's ceiling                      1s
#   the suite's ceiling                         120s
#
# A hundredfold over the transport it is bounding, and a hundredth of the budget
# a real barrier gets, so this spends a second to prove the second is enough.
barrier_probe_ledger="$RUN_ROOT/barrier-probe-ledger"
barrier_probe_channel="test-never-signalled-$$"
barrier_probe_rc=0
# ITS OWN LEDGER. The shim writes an expiry where the run's summary reads it and
# refuses to end green; a probe that deliberately expires must not leave that
# record behind, or this check would make its own run red.
barrier_probe_out="$(GANG_TEST_WAIT_CEILING=1 \
  GANG_TEST_WAIT_LEDGER="$barrier_probe_ledger" \
  tmux wait-for "$barrier_probe_channel" 2>&1)" || barrier_probe_rc=$?
equal "a barrier nothing signals is cut off instead of parking the run" \
  "111" "$barrier_probe_rc"
contains "and the cut-off says the barrier was never signalled" \
  "$barrier_probe_out" "BARRIER NEVER SIGNALLED"
contains "and names the channel that was waited on" \
  "$barrier_probe_out" "$barrier_probe_channel"
contains "and writes that channel where a dying pane cannot take it" \
  "$(<"$barrier_probe_ledger")" "$barrier_probe_channel"
equal "the run's own barrier ledger is untouched by the probe" "" \
  "$(cat "$RUN_ROOT/wedged-barriers" 2>/dev/null || true)"
# AND THE CEILING DOES NOT BOUND AN ANSWERED BARRIER INTO A FAILURE. A signal
# is latched, so this pair is the ordinary path every other barrier here takes,
# run through the same shim.
tmux wait-for -S "$barrier_probe_channel-answered"
barrier_answered_rc=0
tmux wait-for "$barrier_probe_channel-answered" || barrier_answered_rc=$?
equal "a barrier that is answered still returns clean through the ceiling" \
  "0" "$barrier_answered_rc"

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
# characters — so the title was never a way to paint somebody's terminal. Some
# supported tmux versions return a user option raw, so reading identity from
# @gl_agent can open that door. Names go out sanitized; matching keeps the bytes.
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

# tmux versions differ at the boundary above: 3.2 returns the raw user-option
# byte while 3.4 serializes it as the four visible characters `\033`. In the
# latter world drop does not resolve the raw lookup name, which exposed a second
# route to the terminal: its missing-agent error formatted the unsanitized
# argument before cmd_drop had made its safe display copy. This independent
# absent-name case fixes the error path in place instead of relying on either
# tmux representation of a forged registration.
missing_ctl_name="$(printf 'missing\033[31mrogue')"
missing_ctl_drop="$("$GANG" drop "$missing_ctl_name" 2>&1)" || true
equal "a missing-agent drop refusal makes its control byte visible, never active" \
  "gang: no agent 'missing?[31mrogue'" "$missing_ctl_drop"

# win_id owns every resolver's contradicted-title and ambiguous-name errors.
# Sanitizing only inside drop leaves the registration it reports able to paint
# through every caller. This forged identity is independently witnessed by its
# tmux option; the requested title itself contains no hostile byte.
ctl_claimed_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n victim "PS1='❯ ' bash --norc")"
tmux set-option -w -t "$ctl_claimed_id" @gl_agent "$ctl_name"
tmux set-option -w -t "$ctl_claimed_id" @gl_collar bash
ctl_claimed_out="$("$GANG" drop victim 2>&1)" || true
contains "a forged registration produces the contradicted-title refusal" \
  "$ctl_claimed_out" "registered to Gangline agent"
equal "the resolver never paints a forged registration through that refusal" \
  "clean" "$(ctl_escapes "$ctl_claimed_out")"
tmux kill-window -t "$ctl_claimed_id" 2>/dev/null || true

# WHICH WORLD THIS RUN IS STANDING IN, ASKED RATHER THAN ASSUMED. The two
# refusals win_id owns can only be made to carry a hostile byte where a user
# option round-trips one, and supported tmux versions disagree about that. The
# probe is the single definition of the question; both this suite and the tmux
# cell in .github/workflows/shell.yml read it rather than restating it, and it
# ends loudly on a substrate that answers neither way.
ctl_option_bytes="$("$ROOT/test/tmux-option-bytes.sh")"
printf 'instrument tmux=%s user-option-control-bytes=%s\n' \
  "$(tmux -V)" "$ctl_option_bytes"

if [ "$ctl_option_bytes" = raw ]; then
  # THE OTHER RESOLVER DIAGNOSTIC. Status 3 above says a window belongs to
  # somebody else; status 2 says two of them answer to the same name, and it
  # formats the requested identity for exactly the same terminal. Both windows
  # register the control-bearing name, so the resolver matches raw bytes twice
  # and the ambiguity is real rather than a title coincidence.
  ctl_twin_a="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
    -n ctl-twin-a "PS1='❯ ' bash --norc")"
  ctl_twin_b="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
    -n ctl-twin-b "PS1='❯ ' bash --norc")"
  for ctl_twin in "$ctl_twin_a" "$ctl_twin_b"; do
    tmux set-option -w -t "$ctl_twin" @gl_agent "$ctl_name"
    tmux set-option -w -t "$ctl_twin" @gl_collar bash
  done
  ctl_ambiguous_out="$("$GANG" drop "$ctl_name" 2>&1)" || true
  contains "two windows registered to one control-bearing name are ambiguous" \
    "$ctl_ambiguous_out" "is ambiguous in session"
  equal "the resolver never paints an ambiguous identity through that refusal" \
    "clean" "$(ctl_escapes "$ctl_ambiguous_out")"
  contains "the ambiguous refusal still names the rename that resolves it" \
    "$ctl_ambiguous_out" "Rename one (tmux rename-window)"
  tmux kill-window -t "$ctl_twin_a" 2>/dev/null || true
  tmux kill-window -t "$ctl_twin_b" 2>/dev/null || true
else
  unknown "win_id's resolver diagnostics sanitize a raw control-bearing identity" \
    "this tmux serializes a user option's control bytes into visible text, so no registration on this host can hold the byte those diagnostics exist to disarm — the contradicted-title checks above passed over an identity that never carried one, and the ambiguous branch cannot be reached with one at all. The tmux cell in .github/workflows/shell.yml is where both are exercised."
fi

# A synchronous tty fixture paints the two menus Gangline used to answer and
# records every key it receives. 2.0 answers neither: both are ordinary
# occupancy, and the key log is the witness that no keystroke was sent.
cat > "$RUN_ROOT/dialog-fixture.py" <<'PY'
#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
import os
import sys
import threading
import tty

variant = os.environ.get("DIALOG_VARIANT", "known")
log_path = os.environ["DIALOG_KEY_LOG"]
ready = os.environ["DIALOG_READY"]
answered = os.environ.get("DIALOG_ANSWERED") or ""
# A CAPTURE IS PAINTED BYTE FOR BYTE. The external-import prompt is a recorded
# frame from the harness itself, and the point of rendering it is that the
# shipped collar's regex meets the bytes the harness produced, not a
# reconstruction of them.
capture = os.environ.get("DIALOG_CAPTURE") or ""
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

def paint_capture():
    with open(capture, encoding="utf-8") as stream:
        rows = stream.read().splitlines()
    sys.stdout.write("\x1b[2J\x1b[H" + "\r\n".join(rows) + "\r\n")
    sys.stdout.flush()

def paint():
    # CRLF, EXPLICITLY. tty.setraw below turns off ONLCR, so a bare "\n" leaves
    # the cursor where it was and every row after the first starts indented by
    # the length of the one before it. A menu row that should begin at column
    # zero would then begin wherever the previous line ended, and a
    # line-anchored occupancy regex would match or miss by terminal width.
    rows = ["  " + line for line in body] + [""]
    rows += [f"{'›' if index == 0 else ' '} {index + 1}. {label}"
             for index, label in enumerate(labels)]
    rows += ["", "  " + footer]
    sys.stdout.write("\x1b[2J\x1b[H" + "\r\n".join(rows) + "\r\n")
    sys.stdout.flush()

def record(key):
    with open(log_path, "a", encoding="utf-8") as stream:
        stream.write(key + "\n")

# A BARRIER THAT NEEDS THE TMUX SERVER CANNOT REPORT ON THE TMUX SERVER. These
# two signals used to be `tmux wait-for -S`: a client call, from a pane inside
# the very server whose responsiveness is not what any assertion here is about.
# Driven 2026-08-24 on two independent runs, that call and the waiter opposite
# it both sat alive and blocked — the fixture HAD reached its signal, and the
# transport carrying it never completed. `tmux wait-for` has no timeout, so a
# run could not go red on it, only quiet, and one such run held the gate lock
# for close to four hours. The byte now goes down a pipe the run owns, so
# neither side of the barrier is a tmux client.
# WRITTEN FROM A THREAD, because the mechanism it replaces did not block. A
# `tmux wait-for -S` returns whether or not anyone is waiting, so the fixture
# went straight on to reading keystrokes; opening a pipe for write blocks until
# a reader arrives, which would hold this fixture out of its input loop for as
# long as the test took to reach the read. The thread keeps the old timing —
# the fixture proceeds immediately — while the reader still blocks on a real
# byte, so the barrier stays an event rather than becoming a clock.
def signal(path):
    def write():
        with open(path, "w", encoding="utf-8") as stream:
            stream.write("x")
    threading.Thread(target=write, daemon=True).start()

# THE MENU ONLY MOVES FOR AN ANSWER, and every byte that arrives is recorded
# first, so a key Gangline should not have sent cannot be hidden by the screen
# reacting to it: the assertions read the log as well as the pane. Enter is the
# operator answering; the menu then gives way to a composer, which is what makes
# the manual-clearance path deliverable.
composer = False
draft = bytearray()
tty.setraw(sys.stdin.fileno())
if capture:
    paint_capture()
else:
    paint()
signal(ready)
while True:
    char = os.read(sys.stdin.fileno(), 1)
    if composer:
        if char in (b"\r", b"\n"):
            sys.stdout.write("\r\n" + draft.decode("utf-8") + "\r\n\u276f ")
            sys.stdout.flush()
            draft.clear()
        else:
            draft.extend(char)
            os.write(sys.stdout.fileno(), char)
        continue
    if char == b"\x1b":
        tail = os.read(sys.stdin.fileno(), 2)
        record({b"[B": "Down", b"[A": "Up"}.get(tail, "Escape " + repr(tail)))
    elif char in (b"\r", b"\n"):
        record("Enter")
        composer = True
        sys.stdout.write("\x1b[2J\x1b[H\u276f ")
        sys.stdout.flush()
        if answered:
            signal(answered)
    else:
        record(repr(char))
PY
chmod +x "$RUN_ROOT/dialog-fixture.py"
# THE COLLAR DECLARES THE RECORDS THAT USED TO BE ANSWERED. A fixture collar
# with no GANG_DIALOGS makes every "no key reached it" assertion below true of
# a core that still had the matcher, because there would have been nothing for
# it to match: the guard could not fail. These are the shipped Codex collar's
# records verbatim, against the menus this fixture paints, with the keys core
# used to press and the hitch-time directory-trust exception that let the
# second one name trust at all. 2.0 reads none of it.
cat > "$RUN_ROOT/collars/dialog.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' dialog"
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter
directory-trust-prompt|^› [0-9]+\. |Yes, continue||Enter'
GANG_DIALOG_HITCH_DIR_TRUST=directory-trust-prompt
GANG_DIALOG_LINES_safety_buffering_prompt='Our systems are thinking a bit more about this request before responding.
Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.
Retry with a faster model
Dismiss and keep waiting
Learn more
No action is required. Codex will keep waiting, and this menu will close when the response is ready.'
GANG_DIALOG_LINES_directory_trust_prompt='Do you trust the contents of this directory? Working with untrusted contents comes with higher risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec policies to load.
Yes, continue
No, quit
Press enter to continue'
SH
# An answerable record whose fingerprint carries authority language was refused
# at load, and the refusal was the deleted machinery's central safety property.
# There is nothing left to refuse it: the record loads and is never read. This
# is Claude's external-import prompt, whose own collar never declared it for
# that reason, and the capture below is the frame the harness itself printed.
cat > "$RUN_ROOT/collars/dialog-claude.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/claude-code.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' dialog-claude"
GANG_RESUME_LAUNCH=""
GANG_STOP_HOOK=""
GANG_SELF_COMPACT=""
GANG_DIALOGS='external-import-trust|^❯ [0-9]+\. |Yes, allow external imports||Enter'
GANG_DIALOG_LINES_external_import_trust='Important: Only use Claude Code with files you trust. Accessing untrusted files may pose security risks https://code.claude.com/docs/en/security
Yes, allow external imports
No, disable external imports
Enter to confirm · Esc to cancel'
SH

# THE SAME BARRIER RULE THE SURVIVOR FIXTURE ALREADY FOLLOWS. Both of this
# fixture's signals travel down pipes this run owns rather than through the
# tmux server, for the reason recorded beside `signal` above. The read blocks
# on the byte exactly as the channel blocked on the signal, so this stays an
# event and spends no wall time.
dialog_channel() { # $1 agent, $2 ready|answered -> that run-owned pipe's path
  printf '%s/dialog-%s-%s' "$RUN_ROOT" "$2" "$1"
}

# A WRITER THAT OPENED AND CLOSED WITHOUT WRITING IS NOT A FIXTURE THAT REACHED
# ITS SIGNAL, and under set -e an empty read would end the run rather than name
# what happened.
# READ WHERE THE COUNTERS LIVE. Two of the three callers of dialog_start
# capture its stdout, and an assertion made inside a command substitution
# increments its counters in a subshell that exits — the check would run, pass
# or fail, and be recorded nowhere. So the barrier is awaited by the caller.
dialog_await() { # $1 agent, $2 ready|answered; assert the byte it was sent
  local channel byte="" rc=0
  channel="$(dialog_channel "$1" "$2")"
  IFS= read -r -n 1 byte < "$channel" || rc=$?
  equal "the $2 signal of $1 arrived from the fixture itself" "0 x" "$rc $byte"
}

dialog_start() { # $1 agent, $2 variant, $3 collar, $4 capture file
  local name="$1" variant="$2" id command
  "$HITCH" "$name" -c "${3:-dialog}" -d /tmp >/dev/null
  id="$(window_id "$name")"
  : > "$RUN_ROOT/$name.keys"
  rm -f "$(dialog_channel "$name" ready)" "$(dialog_channel "$name" answered)"
  mkfifo "$(dialog_channel "$name" ready)" "$(dialog_channel "$name" answered)"
  printf -v command 'DIALOG_VARIANT=%q DIALOG_KEY_LOG=%q DIALOG_READY=%q DIALOG_ANSWERED=%q DIALOG_CAPTURE=%q %q' \
    "$variant" "$RUN_ROOT/$name.keys" "$(dialog_channel "$name" ready)" \
    "$(dialog_channel "$name" answered)" "${4:-}" \
    "$RUN_ROOT/dialog-fixture.py"
  tmux send-keys -l -t "$id" "$command"
  tmux send-keys -t "$id" Enter
}

# The Codex wait screen and the directory-trust prompt are the two records the
# shipped collar used to answer, and hitch -d used to answer the second one on
# the operator's behalf. Both are now refused like any other occupied screen.
for dialog_case in known trust; do
  dialog_start "dialog-$dialog_case" "$dialog_case" dialog
  dialog_await "dialog-$dialog_case" ready
  dialog_before="$(pane "dialog-$dialog_case")"
  equal "a $dialog_case menu is occupancy of unknown authority" \
    "!occupied! (authority unknown)" \
    "$("$GANG" status "dialog-$dialog_case" | sed -n '1p')"
  dialog_live_rc=0
  dialog_live_out="$(printf 'DIALOG_BODY_REACHED' | "$GANG" send \
    --to "dialog-$dialog_case" --from tester --live-only --stdin 2>&1)" \
    || dialog_live_rc=$?
  if [ "$dialog_live_rc" -eq 0 ]; then
    fail "a live-only send to the $dialog_case menu refuses" \
      "send unexpectedly succeeded"
  else
    contains "a live-only send to the $dialog_case menu refuses" \
      "$dialog_live_out" "is occupied (authority unknown)"
  fi
  printf 'DIALOG_BODY_PARKED' | "$GANG" send --to "dialog-$dialog_case" \
    --from tester --stdin > "$RUN_ROOT/dialog-$dialog_case.park" 2>&1 || true
  contains "the default send path refuses it on the same occupancy" \
    "$(<"$RUN_ROOT/dialog-$dialog_case.park")" "is occupied (authority unknown)"
  equal "no key reached the $dialog_case menu through any of it" "" \
    "$(<"$RUN_ROOT/dialog-$dialog_case.keys")"
  # source-guard: whole-surface@386affb9ca58: the claim is that NOTHING wrote to this pane, so every visible byte is the evidence and any producer would falsify it
  equal "and the $dialog_case menu is byte-for-byte where it was" \
    "$dialog_before" "$(pane "dialog-$dialog_case")"
  excludes "and no part of either body reached the screen" \
    "$(pane "dialog-$dialog_case")" "DIALOG_BODY"
  # AN EMPTY LOG IS A CLAIM ABOUT THE INSTRUMENT FIRST. These are the two keys
  # the deleted registry would have pressed at this menu, sent by hand: the log
  # records them, so the emptiness asserted above is the absence of a keystroke
  # rather than a fixture that stopped listening.
  tmux send-keys -t "$(window_id "dialog-$dialog_case")" Down Enter
  dialog_await "dialog-$dialog_case" answered
  equal "the key log records the answer the operator sends by hand" \
    $'Down\nEnter' "$(<"$RUN_ROOT/dialog-$dialog_case.keys")"
  "$GANG" drop "dialog-$dialog_case" >/dev/null
done

# The shipped Claude collar's occupancy regex is bound to a frame the harness
# printed, through the collar itself: the fixture overrides only the launch, so
# GANG_OCCUPIED_REGEX is the one collars/claude-code.sh ships.
dialog_external_hitch=0
dialog_external_load="$(dialog_start dialog-external external-import \
  dialog-claude "$ROOT/test/fixtures/claude-external-import.txt" 2>&1)" \
  || dialog_external_hitch=$?
if [ "$dialog_external_hitch" -eq 0 ]; then
  pass "a collar declaring an authority-shaped legacy record loads"
  # Awaited only where the launch succeeded: a fixture that never started would
  # otherwise be waited on for a byte nothing is going to write.
  dialog_await dialog-external ready
else
  fail "a collar declaring an authority-shaped legacy record loads" \
    "$dialog_external_load"
fi
# source-guard: whole-surface@9e916c8be644: the only thing that ever writes to this pane is the fixture painting the restored capture, so any producer of this line on it is that capture reaching the screen — which is the whole claim
contains "the captured harness frame is what is on screen" \
  "$(pane dialog-external)" "Yes, allow external imports"
equal "and the shipped Claude occupancy regex reads it as occupancy" \
  "!occupied! (authority unknown)" \
  "$("$GANG" status dialog-external | sed -n '1p')"
dialog_external_rc=0
dialog_external_out="$(printf 'DIALOG_BODY_EXTERNAL' | "$GANG" send \
  --to dialog-external --from tester --live-only --stdin 2>&1)" \
  || dialog_external_rc=$?
if [ "$dialog_external_rc" -eq 0 ]; then
  fail "the external-import prompt refuses delivery like any other occupancy" \
    "send unexpectedly succeeded"
else
  contains "the external-import prompt refuses delivery like any other occupancy" \
    "$dialog_external_out" "is occupied (authority unknown)"
fi
equal "and the record Gangline no longer reads answered nothing" "" \
  "$(<"$RUN_ROOT/dialog-external.keys")"
"$GANG" drop dialog-external >/dev/null

# CLAUDE 2.1.239 CAN PAINT ITS AUTO-MODE ENVIRONMENT NUX OVER A LIVE COMPOSER.
# The hookless dialog owns the keyboard, but the two ordinary composer rules
# remain parseable below it. Start with the right-trimmed frame captured from
# the live harness, then expose the underneath composer the defect requires.
cp "$ROOT/test/fixtures/claude-auto-mode-environment.txt" \
  "$RUN_ROOT/claude-auto-nux-overlay.txt"
auto_nux_rule="$(printf '─%.0s' $(seq 100))"
printf '%s\n%s\n%s\n%s\n%s\n' \
  "$auto_nux_rule" '❯' "$auto_nux_rule" '  ctx 1k/200k 1%' \
  '  bypass permissions on' >> "$RUN_ROOT/claude-auto-nux-overlay.txt"
dialog_start dialog-auto-nux external-import dialog \
  "$RUN_ROOT/claude-auto-nux-overlay.txt"
dialog_await dialog-auto-nux ready
# Hitch used the ordinary framed Bash fixture so startup readiness was already
# proven before the screen changed. Observation now uses the shipped Claude
# reader, without relaunching or changing a byte of the pane under test.
tmux set-option -w -t "$(window_id dialog-auto-nux)" @gl_collar dialog-claude
# A POSITIVE SCREEN WITNESS before the state assertion: dialog_start returns
# only after the fixture has painted and signalled its native-ready barrier.
# source-guard: producer@ac863bc3ccf6: dialog_start waits on the fixture signal sent only after this capture has been painted from the nominated file
contains "the auto-mode NUX is painted over the fixture composer" \
  "$(pane dialog-auto-nux)" "Teach auto mode about your environment?"
equal "the hookless NUX over a composer is occupied rather than idle" \
  "!occupied! (authority unknown)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status dialog-auto-nux | sed -n '1p')"
equal "roster does not advertise the stranded slot as free" "occupied" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster --porcelain \
    | awk -F '\t' '$1 == "dialog-auto-nux" { print $3 }')"
auto_nux_send_rc=0
auto_nux_send_out="$(printf 'AUTO_NUX_BODY' | "$GANG" send \
  --to dialog-auto-nux --from tester --live-only --stdin 2>&1)" \
  || auto_nux_send_rc=$?
equal "the live dialog refuses before any paste" "refused" \
  "$([ "$auto_nux_send_rc" -ne 0 ] && printf refused || printf sent)"
contains "the pre-paste refusal names occupancy" \
  "$auto_nux_send_out" "is occupied (authority unknown)"
equal "state observation sends no key to the native NUX" "" \
  "$(<"$RUN_ROOT/dialog-auto-nux.keys")"
# Without the occupied screen rule, the same hookless dialog still must not
# fall through to idle: visible busy paint is the remaining fail-closed tier.
cat > "$RUN_ROOT/collars/dialog-claude-busy.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-claude.sh"
GANG_OCCUPIED_REGEX=""
GANG_BUSY_REGEX='Claude Code reads this project'
SH
tmux set-option -w -t "$(window_id dialog-auto-nux)" @gl_collar dialog-claude-busy
equal "a dialog that also paints busy remains busy rather than idle" "-busy-" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status dialog-auto-nux | sed -n '1p')"
equal "busy-dialog observation sends no key either" "" \
  "$(<"$RUN_ROOT/dialog-auto-nux.keys")"
"$GANG" drop dialog-auto-nux >/dev/null
