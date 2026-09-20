#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Record the public demo against a separate, disposable Gangline team.
set -euo pipefail

repo=$(cd "$(dirname "$0")/../.." && pwd)
demo_root=/tmp/gangline-demo-run
demo_home=/tmp/gangline-demo-home
demo_session=gangline-demo
proof_session=gangline-demo-proof
vhs_tmp=$(mktemp -d /tmp/gangline-vhs.XXXXXX)
demo_tmux_root=$(mktemp -d /tmp/gangline-demo-tmux.XXXXXX)
operator_home=${HOME:?HOME is required to seed the disposable demo home}
operator_config=${GANG_CONFIG_DIR:-${XDG_CONFIG_HOME:-$operator_home/.config}/gangline}
# Keep the recorded shell's HOME neutral while Codex reads only its disposable
# authentication profile.
export CODEX_HOME="$demo_home/.codex"
demo_config=$vhs_tmp/gangline-config
# Diagnostics have to outlive the recording. The team's sockets, its panes and
# the demo root all belong to this run and are gone the moment it ends, so a
# failed take can only be read afterwards from a directory that is not part of
# it. Resolve it before the private overrides below move XDG_STATE_HOME.
demo_diag=${XDG_STATE_HOME:-$HOME/.local/state}/gangline-demo/$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$demo_diag"

# A recorder may itself run inside an agent window. Make every bare tmux and
# gang invocation resolve through the disposable server, and keep a second
# checkout's guard shim out of the PATH inherited by the recorded agents.
unset TMUX TMUX_PANE CLAUDE_CODE_CHILD_SESSION GANG_LOCK_DIR
export TMUX_TMPDIR="$demo_tmux_root"
export GANG_LOCK_DIR="$demo_tmux_root/locks"
# hitch writes the socket it actually reached the team on under this root, asked
# of the server rather than assembled from a label. Teardown reads it there
# instead of guessing a path, and its absence is how this script knows no team
# was ever created.
team_socket_record="$GANG_LOCK_DIR/teams/$demo_session"
# The tape's readiness probes run from here, not from the demo root, where the
# recorded agents would see a file that is not theirs.
export GANG_DEMO_DIAG="$demo_diag"
export GANG_ARCHIVE_DIR="$demo_tmux_root/archive"
export XDG_STATE_HOME="$demo_tmux_root/state"
# A generated follow-up occupies Claude Code's composer after the lead ends its
# turn, so Gangline correctly parks the worker report instead of overwriting it.
export CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false
# Each recorded harness boots alongside the other one and the browser and
# encoder this script starts, and under that load a harness can take longer to
# reach a composer than the default foreground bound allows. A startup contract
# that misses the bound is queued rather than typed, and nothing drains that
# queue before the agent's first turn exists, so the recorded team would never
# receive its brief.
export GANG_BOOT_TIMEOUT=120
# The recorder tty may have no usable systemd user bus. The exact private
# server and session teardown below own this short-lived team's containment.
export GANG_SCOPE=off
clean_path=""
old_ifs=$IFS
IFS=:
for path_entry in $PATH; do
  case "$path_entry" in
    */libexec/gang-tmux-guard|*/.asdf/shims) continue ;;
  esac
  clean_path="${clean_path:+$clean_path:}$path_entry"
done
IFS=$old_ifs
PATH="$repo/bin:$clean_path"
export PATH

[ "$demo_root" = /tmp/gangline-demo-run ] || {
  echo "refusing unexpected demo root: $demo_root" >&2
  exit 1
}
[ "$demo_home" = /tmp/gangline-demo-home ] || {
  echo "refusing unexpected demo home: $demo_home" >&2
  exit 1
}
[ "$demo_session" = gangline-demo ] || {
  echo "refusing unexpected demo session: $demo_session" >&2
  exit 1
}
[ "$proof_session" = gangline-demo-proof ] || {
  echo "refusing unexpected proof session: $proof_session" >&2
  exit 1
}

for tool in vhs ffmpeg ttyd chromium tmux gang claude codex git python3; do
  command -v "$tool" >/dev/null || {
    echo "missing demo dependency: $tool" >&2
    exit 1
  }
done

# A recording that fails leaves the only account of why inside the panes that are
# about to be deleted. Keep them first, and keep the scrollback: the interesting
# line is rarely the last one.
capture_demo_panes() {
  local socket pane window
  if [ ! -f "$team_socket_record" ]; then
    echo "no demo team socket recorded; no panes to keep" >&2
    return 0
  fi
  IFS= read -r socket < "$team_socket_record"
  if ! tmux -S "$socket" list-panes -a -F '#{window_name} #{pane_id}' \
    > "$demo_diag/panes.txt"; then
    echo "could not list the demo team's panes on $socket" >&2
    return 0
  fi
  while read -r window pane; do
    tmux -S "$socket" capture-pane -p -S -2000 -t "$pane" \
      > "$demo_diag/pane-$window.txt" ||
      echo "could not capture demo pane $window ($pane)" >&2
  done < "$demo_diag/panes.txt"
  echo "kept the demo team's panes under $demo_diag" >&2
}

# Gangline ends its own team: this archives each window's pending spool and
# reports what it could not remove, which a kill-session cannot. Deleting the
# socket root without ending the session leaves a server nothing can reach any
# more, still holding live agents.
end_demo_team() {
  if [ ! -f "$team_socket_record" ]; then
    echo "no demo team was created; nothing to end" >&2
    return 0
  fi
  GANG_SESSION="$demo_session" gang down "$demo_session"
}

cleanup() {
  local status=$?
  [ "$status" -eq 0 ] || capture_demo_panes
  local ended=0
  end_demo_team || ended=$?
  rm -rf -- "$demo_root"
  rm -rf -- "$demo_home"
  rm -rf -- "$vhs_tmp"
  rm -rf -- "$demo_tmux_root"
  [ "$ended" -eq 0 ] || {
    echo "the recorded demo team did not end cleanly (gang down exit $ended);" \
      "a server may still hold live agents — find it with" \
      "\"pgrep -af -- '-s $demo_session'\" and check its /proc/PID/cwd" >&2
    [ "$status" -ne 0 ] || exit 1
  }
}
trap cleanup EXIT

rm -rf -- "$demo_root"
rm -rf -- "$demo_home"
mkdir -p "$demo_root" "$demo_home"
mkdir -p "$demo_home/.codex"
cp "${HOME:?HOME is required}/.codex/auth.json" "$demo_home/.codex/auth.json"
cat > "$demo_home/.codex/config.toml" <<'TOML'
[projects."/tmp/gangline-demo-run"]
trust_level = "trusted"
TOML
cp -R "$operator_home/.claude" "$demo_home/.claude"
git -C "$demo_root" init -q
cat > "$demo_root/TASK.md" <<'TASK'
Build a restrained, runnable terminal finale at /tmp/gangline-demo-run/finale.py.

This is the closing shot of a public demo, and its held final frame becomes the
still poster. It has to look like something a serious engineering team would
leave on screen: quiet, typographic, legible on a phone.

Requirements:
- Python 3 standard library only; no downloads or generated data files.
- Animate for about three seconds, then hold a completely static final frame.
  Nothing blinks, pulses, scrolls, or moves once the reveal is done, and the
  cursor is not visible in it.
- The final frame is centered typography on the bare terminal background: a
  large block-lettered "GANGLINE" wordmark at least six rows tall, a thin
  horizontal rule under it, and the exact subtitle
  "CLAUDE + CODEX — CONNECTED BY GANGLINE" below that. Nothing else.
- The wordmark is drawn as solid type: the strokes of each letter are filled with
  one glyph, and everything that is not a stroke is a space. Do not fill the
  counters, the gaps between letters or the area around them with dots, periods,
  hyphens, dimmed characters, shading or any other placeholder — a lattice of
  filler reads as a debug grid, not as a wordmark, and it is the one thing this
  frame cannot look like.
- The rule is a single unbroken horizontal line: one repeated line-drawing or
  underline character with no gaps. A run of hyphens, dashes, underscores with
  spaces, or equals signs is a dashed line and is wrong.
- Use exactly two colors: 24-bit foreground #f09aac for the wordmark, and
  24-bit foreground #8b949e for the rule and the subtitle. No third color, no
  rainbow or per-character gradient, no background fills, no confetti, no
  stars, no sparkles, no emoji, no box-drawing frame.
- The animation reveals that same composition rather than playing a different
  scene: draw the rule, bring the wordmark in, type the subtitle on, stop.
- Hide the cursor during animation and restore it even on interruption.
- Adapt to the current terminal dimensions and remain legible without color.
- Accept --hold SECONDS to keep the completed final frame displayed before
  returning; default to one second and reject invalid values cleanly.
- Support --check: render no animation, validate the important invariants, print
  exactly "show ready", and exit zero. Its invariants must prove that the final
  frame carries the wordmark and the exact subtitle, that the frame's 24-bit
  color escapes name only those two colors — a third is a failure — that every
  cell of the wordmark rows is either the stroke glyph or a space, and that the
  rule row holds one repeated character and no spaces.

Make it executable. Prove it with py_compile and --check, then run the animation
once. Do not merely describe code: create and test the real artifact. Do not
delete files or cache directories. Leave the completed artifact in place; the
recorder will show the result, so do not send a follow-up message when finished.
TASK

# Keep the disposable demo's Gangline settings separate from the recorder.
mkdir -p "$demo_config/collars"
[ ! -f "$operator_config/config" ] || cp "$operator_config/config" "$demo_config/config"
[ ! -f "$operator_config/CONTRACT.md" ] || cp -L "$operator_config/CONTRACT.md" "$demo_config/CONTRACT.md"
cat > "$demo_config/collars/codex.sh" <<COLLAR
# shellcheck shell=bash
. "$repo/collars/codex.sh"
GANG_LAUNCH="codex -c check_for_update_on_startup=false"
GANG_RESUME_LAUNCH="codex resume {{session_id}} -c check_for_update_on_startup=false -c 'tui.resume_cwd=\\\"current\\\"'"
GANG_STOP_HOOK=""
GANG_SELF_COMPACT=""
GANG_SELF_COMPACT_WITNESS=""
COLLAR

# Claude Code asks whether a directory is trusted the first time it opens one,
# and an unanswered dialog stalls the whole recording. The demo root is built
# from scratch under /tmp on every run, and the host empties /tmp at boot, so any
# stored answer for it is pruned and the dialog returns on a cold host. Record
# the answer for the one fixed path the guard above pins, before anything
# launches an agent in it.
claude_config="${CLAUDE_CONFIG_DIR:-$operator_home}/.claude.json"
[ -f "$claude_config" ] || {
  echo "claude configuration not found: $claude_config" >&2
  exit 1
}
cp "$claude_config" "$demo_home/.claude.json"
python3 - "$demo_home/.claude.json" "$demo_root" <<'SEED'
import json
import os
import sys
import tempfile

config_path, project = sys.argv[1], sys.argv[2]
with open(config_path, encoding="utf-8") as handle:
    config = json.load(handle)
entry = config.setdefault("projects", {}).setdefault(project, {})
if entry.get("hasTrustDialogAccepted") is True:
    print("demo root already trusted: %s" % project)
    sys.exit(0)
entry["hasTrustDialogAccepted"] = True
directory = os.path.dirname(config_path) or "."
mode = os.stat(config_path).st_mode & 0o7777
staged_fd, staged = tempfile.mkstemp(dir=directory, prefix=".claude.json.demo.")
try:
    with os.fdopen(staged_fd, "w", encoding="utf-8") as handle:
        json.dump(config, handle, indent=2)
    os.chmod(staged, mode)
    os.replace(staged, config_path)
except BaseException:
    os.unlink(staged)
    raise
print("trusted demo root: %s" % project)
SEED

# The tape needs a durable stage marker before filming the next step. Ask
# Gangline what the agents are doing instead of requiring their prose to contain
# a particular phrase.
cat > "$demo_diag/probe.sh" <<'PROBE'
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Wait for one recorded agent to reach a state, then print a marker the tape
# waits on. Usage: probe.sh <agent> idle|busy|briefed|reported [seconds]
#
# briefed and artifact stay true once they are true, which is why the tape waits
# on one of them before it reads a momentary idle or busy: a probe for the moment
# a turn starts can arrive after it and then spend its whole budget on a team
# that has already moved on.
set -euo pipefail

agent=${1:?probe: agent name required}
condition=${2:?probe: condition required}
limit=${3:-280}
diag=$(cd -- "$(dirname -- "$0")" && pwd)
log=$diag/probe-$agent.log

read_status() {
  if ! NO_COLOR=1 gang status "$agent" > "$diag/status-$agent.txt" 2>> "$log"; then
    printf 'PROBE-ERROR %s %s: gang status failed; see %s\n' \
      "$agent" "$condition" "$log"
    exit 1
  fi
}

satisfied() {
  case $condition in
    idle | busy)
      read_status
      if grep -q '~idle~' "$diag/status-$agent.txt"; then
        [ "$condition" = idle ]
      else
        [ "$condition" = busy ]
      fi
      ;;
    briefed)
      # The lead's envelope on the worker's screen is the arrival of the brief
      # itself, not a guess from how busy the pane looks.
      if ! gang capture "$agent" 80 > "$diag/capture-$agent.txt" 2>> "$log"; then
        printf 'PROBE-ERROR %s %s: gang capture failed; see %s\n' \
          "$agent" "$condition" "$log"
        exit 1
      fi
      grep -q '\[gang:lead#' "$diag/capture-$agent.txt"
      ;;
    artifact)
      test -x /tmp/gangline-demo-run/finale.py \
        && python3 /tmp/gangline-demo-run/finale.py --check \
          | grep -qx 'show ready'
      ;;
    reported)
      if ! gang capture "$agent" 80 > "$diag/capture-$agent.txt" 2>> "$log"; then
        printf 'PROBE-ERROR %s %s: gang capture failed; see %s\n' \
          "$agent" "$condition" "$log"
        exit 1
      fi
      grep -q '\[gang:worker#' "$diag/capture-$agent.txt"
      ;;
    *)
      printf 'PROBE-ERROR %s %s: unknown condition\n' "$agent" "$condition"
      exit 1
      ;;
  esac
}

waited=0
while [ "$waited" -lt "$limit" ]; do
  if satisfied; then
    printf '%s %s %s satisfied after %ss\n' \
      "$(date -u +%H:%M:%SZ)" "$agent" "$condition" "$waited" >> "$log"
    printf 'PROBE-READY %s %s\n' "$agent" "$condition"
    exit 0
  fi
  sleep 1
  waited=$((waited + 1))
done
printf '%s %s %s gave up after %ss\n' \
  "$(date -u +%H:%M:%SZ)" "$agent" "$condition" "$limit" >> "$log"
# Say so on screen. An unmet condition that prints nothing is indistinguishable
# from a team that never got there, and the tape's own timeout reports neither.
printf 'PROBE-TIMEOUT %s %s: not reached in %ss; last reading under %s\n' \
  "$agent" "$condition" "$limit" "$diag"
exit 1
PROBE
chmod +x "$demo_diag/probe.sh"

# Establish and prove the private socket before either native agent launches.
# Ending its only proof session lets tmux exit; hitch then creates the recorded
# team on the same isolated socket root without an unregistered window in it.
tmux new-session -d -s "$proof_session" -n socket-proof 'tail -f /dev/null'
[ "$(tmux list-sessions -F '#S')" = "$proof_session" ] || {
  echo "private demo server contains an unexpected session" >&2
  tmux list-sessions >&2
  exit 1
}
tmux kill-session -t "=$proof_session"

cd "$repo"
render_gif=site/demo/.demo-render.gif
render_mp4=site/demo/.demo-render.mp4
GANG_CONFIG_DIR="$demo_config" \
  GANG_COLLARS="$demo_config/collars" \
  HOME="$demo_home" XDG_CONFIG_HOME="$vhs_tmp/chromium-config" TMPDIR="$vhs_tmp" \
  vhs site/demo/demo.tape
[ -s "$render_gif" ] && [ -s "$render_mp4" ] || {
  echo "recorder produced an empty media file" >&2
  exit 1
}
mv "$render_gif" site/demo.gif
mv "$render_mp4" site/demo.mp4
ffmpeg -v error -y -sseof -3 -i site/demo.mp4 -frames:v 1 \
  site/demo-poster.jpg

echo "recorded site/demo.gif, site/demo.mp4, and site/demo-poster.jpg"
