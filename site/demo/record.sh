#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Record the public demo against a separate, disposable Gangline team.
set -euo pipefail

repo=$(cd "$(dirname "$0")/../.." && pwd)
demo_root=/tmp/gangline-demo-run
demo_session=gangline-demo
proof_session=gangline-demo-proof
vhs_tmp=$(mktemp -d /tmp/gangline-vhs.XXXXXX)
demo_tmux_root=$(mktemp -d /tmp/gangline-demo-tmux.XXXXXX)
operator_config=${GANG_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/gangline}
demo_config=$vhs_tmp/gangline-config

# A recorder may itself run inside an agent window. Make every bare tmux and
# gang invocation resolve through the disposable server, and keep a second
# checkout's guard shim out of the PATH inherited by the recorded agents.
unset TMUX TMUX_PANE
export TMUX_TMPDIR="$demo_tmux_root"
export GANG_LOCK_DIR="$demo_tmux_root/locks"
export GANG_ARCHIVE_DIR="$demo_tmux_root/archive"
export XDG_STATE_HOME="$demo_tmux_root/state"
# A generated follow-up occupies Claude Code's composer after the lead ends its
# turn, so Gangline correctly parks the worker report instead of overwriting it.
export CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false
# The recorder tty may have no usable systemd user bus. The exact private
# server and session teardown below own this short-lived team's containment.
export GANG_SCOPE=off
clean_path=""
old_ifs=$IFS
IFS=:
for path_entry in $PATH; do
  case "$path_entry" in
    */libexec/gang-tmux-guard) continue ;;
  esac
  clean_path="${clean_path:+$clean_path:}$path_entry"
done
IFS=$old_ifs
PATH="$clean_path"
export PATH

[ "$demo_root" = /tmp/gangline-demo-run ] || {
  echo "refusing unexpected demo root: $demo_root" >&2
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

for tool in vhs ffmpeg ttyd chromium tmux gang claude codex git; do
  command -v "$tool" >/dev/null || {
    echo "missing demo dependency: $tool" >&2
    exit 1
  }
done

cleanup() {
  tmux -S "$demo_tmux_root/tmux-$(id -u)/default" \
    has-session -t "=$demo_session" 2>/dev/null &&
    tmux -S "$demo_tmux_root/tmux-$(id -u)/default" \
      kill-session -t "=$demo_session" || true
  rm -rf -- "$demo_root"
  rm -rf -- "$vhs_tmp"
  rm -rf -- "$demo_tmux_root"
}
trap cleanup EXIT

rm -rf -- "$demo_root"
mkdir -p "$demo_root"
git -C "$demo_root" init -q
cat > "$demo_root/TASK.md" <<'TASK'
Build a polished, runnable terminal show at /tmp/gangline-demo-run/hello.py.

Requirements:
- Python 3 standard library only; no downloads or generated data files.
- Animate for about three seconds, then leave a bold final frame on screen.
- Use ANSI motion and vivid 24-bit color to fill a 1280x720 terminal with a
  large, centered rainbow banner reading "HELLO, TEAM!", stars/confetti, and
  the exact subtitle "CLAUDE + CODEX — CONNECTED BY GANGLINE". The final frame
  must visibly use at least six saturated hues across the banner (red, orange,
  yellow, green, cyan, blue, magenta), not grayscale or dim theme colors.
- Hide the cursor during animation and restore it even on interruption.
- Adapt to the current terminal dimensions and remain legible without color.
- Accept --hold SECONDS to keep the completed final frame displayed before
  returning; default to one second and reject invalid values cleanly.
- Support --check: render no animation, validate the important invariants, print
  exactly "show ready", and exit zero. Its invariants must prove that the final
  color frame contains multiple explicit 24-bit foreground-color escape codes.

Make it executable. Prove it with py_compile and --check, run the animation once,
and report the proof and design choices to the lead. Do not merely describe code:
create and test the real artifact.
TASK

# The external operator shell is necessarily self-declared. Give the demo lead
# verified launch-time context that this specific brief is expected, while
# preserving the operator's ordinary doctrine and settings in the private copy.
mkdir -p "$demo_config/roles"
[ ! -f "$operator_config/config" ] || cp "$operator_config/config" "$demo_config/config"
[ ! -f "$operator_config/DOCTRINE.md" ] || cp -L "$operator_config/DOCTRINE.md" "$demo_config/DOCTRINE.md"
[ ! -f "$operator_config/CONTRACT.md" ] || cp -L "$operator_config/CONTRACT.md" "$demo_config/CONTRACT.md"
cat > "$demo_config/roles/demo-lead.md" <<'ROLE'
The operator will send this demo's task brief through gang talk from the external
shell after hitch. Its self-declared:operator attribution is expected in this
single-tenant recording and carries the task you should act on.

Delegate TASK.md as one whole arc to the existing named worker with gang send.
Do not spawn a native subagent. Stay idle while the worker owns the arc. When its
attributed completion report starts your next turn, summarize the result and end.
ROLE

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
GANG_CONFIG_DIR="$demo_config" \
  XDG_CONFIG_HOME="$vhs_tmp/chromium-config" TMPDIR="$vhs_tmp" \
  vhs site/demo/demo.tape
ffmpeg -v error -y -sseof -3 -i site/demo.mp4 -frames:v 1 \
  site/demo-poster.jpg

echo "recorded site/demo.gif, site/demo.mp4, and site/demo-poster.jpg"
