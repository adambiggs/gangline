#!/usr/bin/env bash
# Record site/demo/demo.tape against a separate demo HOME at /home/demo.
#
# The demo runs three real harnesses (Claude Code lead, Codex worker, opencode
# reviewer) on the invoking user's subscription auth, under a HOME of its own so
# no real path, hostname, or config lands on screen. That is a cosmetic
# boundary, not a security one: these agents run as the invoking user, with that
# user's files and network, and a separate HOME changes neither. Record on a
# machine where that is fine. This script stages the demo HOME idempotently,
# resets per-take state, runs vhs, and moves the rendered videos into site/.
# Auth files are copied byte-for-byte and never printed; wipe them with
# --wipe-auth when you are done recording.
#
# Usage:
#   site/demo/record.sh              stage + record
#   site/demo/record.sh --wipe-auth  remove copied credentials from /home/demo
set -euo pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
DEMO=/home/demo
REAL_HOME="$HOME"

wipe_auth() {
  rm -f "$DEMO/.claude/.credentials.json" \
        "$DEMO/.claude.json" \
        "$DEMO/.codex/auth.json" \
        "$DEMO/.local/share/opencode/auth.json"
  echo "wiped copied credentials from $DEMO (re-staged on next run)"
}

if [ "${1:-}" = "--wipe-auth" ]; then
  wipe_auth
  exit 0
fi

for tool in vhs ffmpeg tmux git rsync claude codex opencode node python3; do
  command -v "$tool" >/dev/null || { echo "missing: $tool" >&2; exit 1; }
done
if [ ! -d "$DEMO" ]; then
  echo "create the demo HOME first (owned by you):" >&2
  echo "  sudo mkdir -m 755 $DEMO && sudo chown $(id -u):$(id -g) $DEMO" >&2
  exit 1
fi

# ── Layout ───────────────────────────────────────────────────────────────────
mkdir -p "$DEMO/bin" "$DEMO/.tmux" "$DEMO/.claude" "$DEMO/.codex" \
         "$DEMO/.config/opencode" "$DEMO/.local/share/opencode" "$DEMO/hello"

# Frozen copy of Gangline itself, so repo churn never changes a take mid-series.
rsync -a --delete "$REPO/bin" "$REPO/profiles" "$REPO/roles" "$REPO/statusline" \
  "$DEMO/gangline/"

# The tape's PATH is only $DEMO/bin + $DEMO/gangline/bin + system dirs; this
# symlink farm is how version-managed tools stay reachable without their
# real (user-specific) install paths ever appearing in the recorded env.
for tool in claude codex opencode node python3 git tmux; do
  ln -sf "$(command -v "$tool")" "$DEMO/bin/$tool"
done

cat > "$DEMO/.tmux.conf" <<'EOF'
# Demo cosmetics: no hostname/clock in the status bar, quiet colors.
set -g status-right ""
set -g status-style "bg=colour235,fg=colour245"
set -g window-status-current-style "fg=colour231,bold"
set -g status-left-length 20
EOF

cat > "$DEMO/.gitconfig" <<'EOF'
[user]
	name = demo
	email = demo@localhost
[init]
	defaultBranch = main
EOF

# A .git makes the harnesses treat ~/hello as a project root; it stays
# commitless — the demo shows the work, not the checkpoint.
[ -d "$DEMO/hello/.git" ] || git -C "$DEMO/hello" init -q

cat > "$DEMO/.claude/settings.json" <<'EOF'
{
  "permissions": {
    "allow": [
      "Bash",
      "Read",
      "Glob",
      "Grep",
      "Write",
      "Edit"
    ]
  },
  "statusLine": {
    "type": "command",
    "command": "/home/demo/gangline/statusline/claude-code-context.sh"
  },
  "theme": "dark"
}
EOF

cat > "$DEMO/.codex/config.toml" <<'EOF'
model = "gpt-5.6-sol"
model_reasoning_effort = "medium"
approval_policy = "never"
# The setup the README recommends for a Codex worker, and the demo is the place
# to show it: the sandbox stays on, and network access buys back the tmux socket
# a Codex agent needs to answer at all (README: "A sandboxed Codex agent needs
# network access to answer"). Without it the lead falls back to gang capture
# polling, which puts a caveat on screen instead of the thing the demo is for.
sandbox_mode = "workspace-write"

[sandbox_workspace_write]
network_access = true

[projects."/home/demo/hello"]
trust_level = "trusted"
EOF

cat > "$DEMO/.config/opencode/opencode.json" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "model": "github-copilot/gemini-3.1-pro-preview",
  "permission": {
    "external_directory": {
      "/home/demo/gangline/**": "allow"
    }
  }
}
EOF

# ── Auth (copied, never printed) ─────────────────────────────────────────────
copy_auth() {
  [ -f "$1" ] || { echo "missing auth source: $1" >&2; exit 1; }
  install -m 600 "$1" "$2"
}
copy_auth "$REAL_HOME/.claude/.credentials.json" "$DEMO/.claude/.credentials.json"
copy_auth "$REAL_HOME/.codex/auth.json"          "$DEMO/.codex/auth.json"
copy_auth "$REAL_HOME/.local/share/opencode/auth.json" \
          "$DEMO/.local/share/opencode/auth.json"

# Claude Code keeps its account identity and folder trust in ~/.claude.json; seed
# just those so the demo boots straight past login and the trust dialog. Rewritten
# every run rather than written once, because what lands in here ends up on screen
# — a fix to it has to reach the next take instead of sitting behind a file that
# already exists.
#
# The account identity is deliberately NOT scrubbed here. Claude Code paints the
# signed-in organization in its boot box — for a personal account that string is
# literally "<your email>'s Organization" — and it re-fetches that profile from the
# server on the way up, so a neutralised emailAddress in this file is overwritten
# seconds later. Tried it, watched it come back. The boot box is kept off camera by
# the tape instead (demo.tape, scenes 1 and 4).
#
# The release note and the announcement impressions ARE carried over: a fresh HOME
# has seen neither, and a boot box padded out with promos is a taller header for
# the lead's own output to push off screen before its scene can start. Impressions
# are copied by id and pushed past any plausible cap rather than invented, so this
# suppresses exactly the announcements the operator has already worked through.
python3 - "$REAL_HOME/.claude.json" "$DEMO/.claude.json" \
         "$(claude --version | awk '{print $1}')" <<'EOF'
import json, sys
real = json.load(open(sys.argv[1]))
seed = {
    "oauthAccount": real["oauthAccount"],
    "hasCompletedOnboarding": True,
    "lastReleaseNotesSeen": sys.argv[3],
    "announcementImpressions": {k: 99 for k in real.get("announcementImpressions", {})},
    "projects": {"/home/demo/hello": {"hasTrustDialogAccepted": True}},
}
with open(sys.argv[2], "w") as f:
    json.dump(seed, f, indent=1)
EOF
chmod 600 "$DEMO/.claude.json"

# ── Per-take reset ───────────────────────────────────────────────────────────
# `env -u TMUX` is load-bearing: a tmux client inside a tmux pane takes its
# socket from $TMUX and ignores TMUX_TMPDIR, so without this the kill-server
# lands on the invoking user's own server.
env -u TMUX TMUX_TMPDIR="$DEMO/.tmux" tmux kill-server 2>/dev/null || true
rm -f "$DEMO/hello/hello.py" "$DEMO/demo.webm" "$DEMO/demo.mp4" "$DEMO/demo.gif"

# Agent state is per-take state. A lead that learned something in an earlier
# take writes it to project memory, then recites it in the next one as settled
# fact — including things a later config change has already made false. Clearing
# transcripts and memory is what makes a take reproducible instead of
# path-dependent. Credentials and the seeded .claude.json survive deliberately;
# only the agents' accumulated beliefs go.
rm -rf "$DEMO/.claude/projects" "$DEMO/.claude/history.jsonl" \
       "$DEMO/.codex/sessions" "$DEMO/.codex/history.jsonl" \
       "$DEMO/.local/share/opencode/storage"

# ── Record ───────────────────────────────────────────────────────────────────
# The tape's Env lines build the clean recorded environment; vhs otherwise runs
# in the invoking env so it can find its browser. What is subtracted below is
# everything that names the environment DOING the recording, because the tape
# runs a real gang and a real harness that will read it.
#
# The rule, so the next variable gets handled without another lost take: strip
# anything identifying this session, this pane, or the harness invoking this
# script. TMUX and TMUX_PANE are separate facts and both are load-bearing —
# subtracting one is not subtracting the other:
#
#   TMUX       a tmux client inside a pane takes its socket from $TMUX and
#              ignores TMUX_TMPDIR, so a gang inside the tape would reach the
#              invoking user's server instead of the demo one.
#   TMUX_PANE  gang's self_window() resolves the CALLER's identity from it, and
#              tmux exports it into everything a pane starts. Measured: recording
#              from a Gangline lead's own pane leaked TMUX_PANE into the tape, so
#              `gang send lead --from adam` was refused as impersonation —
#              correctly, since gang saw the caller as `lead` — and the take died
#              at scene 2. Recording from inside a pane is the normal case: a
#              Gangline lead is exactly who records this.
#   XDG_RUNTIME_DIR
#              lock_pane() defaults its lock directory to
#              $XDG_RUNTIME_DIR/gangline-$(id -u). A login session sets that to
#              /run/user/<uid>, which is a real host path — and one the demo's
#              Codex worker cannot write to, because it runs workspace-write
#              sandboxed. Measured: the worker died mid-task on screen and had to
#              re-send with GANG_LOCK_DIR set by hand. Unset, gang lands on
#              /tmp/gangline-<uid>, which every harness here can reach. The
#              underlying default is issue #19; this fence is correct regardless,
#              because /run/user/<uid> is exactly the kind of real host path this
#              script exists to keep off camera.
#
# The CLAUDE* set is stripped on principle rather than on measurement. The tape's
# lead IS Claude Code, and a nested harness inheriting its parent's session and
# child-session ids is not the clean environment this script exists to build.
cd "$DEMO"
env -u TMUX -u TMUX_PANE -u XDG_RUNTIME_DIR \
    -u CLAUDECODE -u CLAUDE_CODE_ENTRYPOINT -u CLAUDE_CODE_EXECPATH \
    -u CLAUDE_CODE_SESSION_ID -u CLAUDE_CODE_CHILD_SESSION -u CLAUDE_PID \
    vhs "$REPO/site/demo/demo.tape"

mv "$DEMO/demo.webm" "$REPO/site/demo.webm"
mv "$DEMO/demo.mp4"  "$REPO/site/demo.mp4"

# The README cannot play a video, so it gets a GIF — derived from the MP4 rather
# than rendered by vhs, so it is the same take frame for frame and a re-record
# cannot leave the two showing different runs. The palette is built once across
# the whole file (stats_mode=diff) and applied with a bayer dither, which is what
# keeps a minute of 720p terminal near the MP4's size instead of many times it.
PAL=$(mktemp --suffix=.png)
trap 'rm -f "$PAL"' EXIT
GIF_VF="fps=10,scale=1280:-1:flags=lanczos"
ffmpeg -v error -y -i "$REPO/site/demo.mp4" \
    -vf "$GIF_VF,palettegen=max_colors=64:stats_mode=diff" "$PAL"
ffmpeg -v error -y -i "$REPO/site/demo.mp4" -i "$PAL" \
    -lavfi "${GIF_VF}[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=5:diff_mode=rectangle" \
    "$REPO/site/demo.gif"

ls -l "$REPO/site/demo.webm" "$REPO/site/demo.mp4" "$REPO/site/demo.gif"
echo
echo "before committing: frame-audit the video (ffmpeg -i site/demo.mp4 -vf fps=1 f%03d.png)"
echo "and check every frame for anything that should not be on screen."
echo "done recording? wipe credentials: site/demo/record.sh --wipe-auth"
