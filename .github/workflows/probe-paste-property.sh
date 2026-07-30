#!/usr/bin/env bash
# The instrument behind tmux-floor.yml. It lives here, beside its only caller,
# so that it is a real file the shell cell can lint rather than a heredoc inside
# YAML that nothing checks. GitHub reads only *.yml in this directory, so a
# script here is inert to Actions.
#
# Runs INSIDE a container whose tmux version is the only thing under test.
#
# It OBSERVES and never judges: every line it prints is a fact, or the token that
# says a fact could not be established. The caller decides what passes, because
# an observer allowed to say "ok" is an observer that can say "ok" for having
# done nothing.
set -u

export DEBIAN_FRONTEND=noninteractive
apt-get -qq update >/dev/null 2>&1 || true
apt-get -qq install -y tmux >/dev/null 2>&1 || true
command -v tmux >/dev/null 2>&1 || { echo 'tmux=ABSENT'; exit 0; }

ver="$(tmux -V)"
echo "tmux=${ver#tmux }"
echo "bash=${BASH_VERSION}"

SOCK=paste-property
T() { tmux -L "$SOCK" "$@"; }
cleanup() {
  T kill-server >/dev/null 2>&1
  # kill-server leaves the socket file behind. The name is one this script set,
  # never one a caller handed it.
  rm -f "${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)/$SOCK"
}
trap cleanup EXIT

PAYLOAD=/tmp/paste-payload
printf 'echo MARK_A\necho MARK_B\necho MARK_C\n' > "$PAYLOAD"

# An anchor session that outlives every case. Each case's pane exits when its
# case ends, and a tmux server with no sessions left exits too — so without this
# the server comes and goes underneath the cases, and starting the next one
# races the last one's teardown ("lost server"). `exit-empty` would say this
# directly and does not exist before tmux 2.7, which is inside the range this
# cell is here to test.
T new-session -d -s anchor -x 80 -y 24 "sleep 3600"

# ---------------------------------------------------------------------------
# Observation 1 — did tmux transmit the bracket bytes?
#
# The receiver is two lines of sh, and that is the point: it requests DECSET
# 2004 with its own printf, so no shell's readline version can influence the
# result. tmux sends brackets only to a pane whose application asked for them,
# so a receiver that never asks makes every tmux look broken. That is exactly
# the confound that made the first run of this measurement conclude the opposite
# (issue #25) — the request below IS the fix, not scaffolding. Deleting it does
# not simplify this file, it silently restores the bug.
# ---------------------------------------------------------------------------
cat > /tmp/paste-recv.sh <<'RECV'
printf '\033[?2004h'
# Printed AFTER the request and read off the pane before anything is pasted.
# tmux consumes a pane's output in order, so this glyph on screen is proof the
# mode request was already processed — which a sleep can only assume.
printf 'RECV_READY\n'
exec cat > /tmp/paste-raw
RECV

brackets() { # $1 = extra paste-buffer flags, $2 = session name -> yes | no | UNDELIVERED | UNKNOWN
  # One server for the whole probe, a fresh SESSION per case. Killing the server
  # between cases raced with the next new-session and cost a "lost server" on
  # roughly one run in three — a flake that surfaced as could-not-determine, so
  # it was loud rather than wrong, but it still decided nothing.
  rm -f /tmp/paste-raw
  T new-session -d -s "$2" -x 80 -y 24 "sh /tmp/paste-recv.sh"
  local up=no
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12; do
    if T capture-pane -p -t "$2" 2>/dev/null | grep -qF RECV_READY; then up=yes; break; fi
    sleep 0.3
  done
  # Never attempted is not the same claim as attempted and unbracketed.
  [ "$up" = yes ] || { echo UNKNOWN; return; }
  T load-buffer -b b "$PAYLOAD"
  # shellcheck disable=SC2086  # $1 is a deliberate empty-or-flag word list
  T paste-buffer $1 -b b -t "$2"
  sleep 1
  # cat block-buffers its stdout when that stdout is a file, so nothing reaches
  # the file until EOF. Reading before this would report "no brackets" for a
  # paste that arrived perfectly — a false negative pointing the safe way.
  T send-keys -t "$2" C-d
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    [ -s /tmp/paste-raw ] && break
    sleep 0.3
  done
  # Payload absent means the receiver never got it, which is not the same claim
  # as "got it unbracketed".
  if [ ! -f /tmp/paste-raw ] || ! grep -qF MARK_A /tmp/paste-raw; then
    echo UNDELIVERED
    return
  fi
  if grep -qF "$(printf '\033[200~')" /tmp/paste-raw; then echo yes; else echo no; fi
}

echo "brackets_with_p=$(brackets -p bracket_p)"
echo "brackets_without_p=$(brackets '' bracket_np)"

# ---------------------------------------------------------------------------
# Observation 2 — did the payload EXECUTE at a real readline receiver?
#
# Corroboration by a different mechanism: brackets on the wire are what tmux
# does, not-executing is what a user gets. Two instruments that disagree mean
# the run decided nothing, and the caller enforces that.
#
# readline defaults enable-bracketed-paste to on only from bash 5.1, so the
# receiver's setting is forced and then reported rather than assumed.
# ---------------------------------------------------------------------------
printf 'set enable-bracketed-paste on\n' > /tmp/paste-inputrc
control=unknown
if INPUTRC=/tmp/paste-inputrc bash --norc -i -c 'bind -v' </dev/null 2>/dev/null \
   | grep -qF 'set enable-bracketed-paste on'; then
  control=on
elif INPUTRC=/tmp/paste-inputrc bash --norc -i -c 'bind -v' </dev/null 2>/dev/null \
     | grep -qF 'enable-bracketed-paste'; then
  control=off
fi
echo "readline_control=$control"

executed() { # $1 = extra paste-buffer flags, $2 = session name -> 0 | 1 | UNKNOWN
  T new-session -d -s "$2" -x 80 -y 24 "INPUTRC=/tmp/paste-inputrc PS1='> ' bash --norc"
  local up=no
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12; do
    if T capture-pane -p -t "$2" 2>/dev/null | grep -q '>'; then up=yes; break; fi
    sleep 0.3
  done
  [ "$up" = yes ] || { echo UNKNOWN; return; }
  T load-buffer -b b "$PAYLOAD"
  # shellcheck disable=SC2086
  T paste-buffer $1 -b b -t "$2"
  sleep 2
  local pane
  pane="$(T capture-pane -p -t "$2")"
  printf '%s\n' "$pane" | grep -qF MARK_A || { echo UNKNOWN; return; }
  # An executed line prints its output on a line of its own, with no prompt.
  if printf '%s\n' "$pane" | grep -qx 'MARK_A'; then echo 1; else echo 0; fi
}

echo "executed_with_p=$(executed -p readline_p)"
echo "executed_without_p=$(executed '' readline_np)"
