#!/usr/bin/env bash
# The second instrument behind tmux-floor.yml, beside probe-paste-property.sh.
#
# Executes every tmux call bin/gang and the shipped profiles actually make, at
# whatever tmux version this container has. The inventory in issue #31 dates each
# of those calls in tmux's own source; this runs them, because a getopt string
# says a flag is accepted and says nothing about the call succeeding.
#
# Prints one line per dependency and a machine-readable pass/fail count. It never
# judges the floor — that is the caller's job. Two of these checks are less
# obvious than they look and are the reason the file exists rather than a
# `gang roles` smoke test: display-message is read with NO CLIENT ATTACHED,
# because gang is run from a plain shell as often as from inside a pane, and
# window_activity is checked for its UNITS rather than its presence, because gang
# compares it against `date +%s`.
set -u

export DEBIAN_FRONTEND=noninteractive
apt-get -qq update >/dev/null 2>&1 || true
apt-get -qq install -y tmux >/dev/null 2>&1 || true
command -v tmux >/dev/null 2>&1 || { echo 'tmux=ABSENT'; exit 0; }
ver="$(tmux -V)"; echo "tmux=${ver#tmux }"

SOCK=vocab
S=gangvocab
T() { tmux -L "$SOCK" "$@"; }
trap 'T kill-server >/dev/null 2>&1; rm -f "${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)/$SOCK"' EXIT

pass=0 fail=0
ck() { # $1 = label, $2 = expected-nonempty|exact:<value>, rest = command
  local label="$1" want="$2"; shift 2
  local out rc
  out="$("$@" 2>&1)"; rc=$?
  if [ "$rc" -ne 0 ]; then
    printf 'FAIL %-46s rc=%s %s\n' "$label" "$rc" "${out%%$'\n'*}"; fail=$((fail+1)); return
  fi
  case "$want" in
    nonempty) [ -n "$out" ] || { printf 'FAIL %-46s empty output\n' "$label"; fail=$((fail+1)); return; } ;;
    exact:*)  [ "$out" = "${want#exact:}" ] || { printf 'FAIL %-46s got %s\n' "$label" "$out"; fail=$((fail+1)); return; } ;;
  esac
  printf 'OK   %-46s %s\n' "$label" "${out%%$'\n'*}"; pass=$((pass+1))
}

# --- session and window creation, printing an id back through a format --------
WID="$(T new-session -d -P -F '#{window_id}' -s "$S" -n first -c /tmp "sh -c 'echo VOCAB_MARK; sleep 600'" 2>&1)"
case "$WID" in
  @*) printf 'OK   %-46s %s\n' "new-session -d -P -F '#{window_id}' -s -n -c" "$WID"; pass=$((pass+1)) ;;
  *)  printf 'FAIL %-46s %s\n' "new-session -d -P -F '#{window_id}' -s -n -c" "$WID"; fail=$((fail+1))
      echo "summary=CTD nothing else can run without a session"; exit 0 ;;
esac

W2="$(T new-window -d -P -F '#{window_id}' -t "=$S" -n second -c /tmp "sh -c 'echo VOCAB_MARK; sleep 600'" 2>&1)"
case "$W2" in
  @*) printf 'OK   %-46s %s\n' "new-window -d -P -F -t '=session' -n -c" "$W2"; pass=$((pass+1)) ;;
  *)  printf 'FAIL %-46s %s\n' "new-window -d -P -F -t '=session' -n -c" "$W2"; fail=$((fail+1)) ;;
esac

# --- the = exact-match target prefix -----------------------------------------
ck "has-session -t '=session' (rc only)"   ''       T has-session -t "=$S"
ck "list-windows -t '=session' -F #{window_id}" nonempty T list-windows -t "=$S" -F '#{window_id}'
ck "list-windows -t '=session' -F #W"     nonempty T list-windows -t "=$S" -F '#W'
ck "list-panes -s -t '=session' -F ids"   nonempty T list-panes -s -t "=$S" -F '#{pane_id} #{window_id}'

# --- window user options, by window id --------------------------------------
ck "set-option -w -t @id @user value"     ''       T set-option -w -t "$WID" @gl_probe hello
ck "show-options -wqv -t @id @user"       exact:hello T show-options -wqv -t "$WID" @gl_probe
ck "set-option -uw -t @id @user"          ''       T set-option -uw -t "$WID" @gl_probe
unset_out="$(T show-options -wqv -t "$WID" @gl_probe 2>&1)"
if [ -z "$unset_out" ]; then
  printf 'OK   %-46s reads empty after unset\n' "show-options -wqv after -uw"; pass=$((pass+1))
else
  printf 'FAIL %-46s %s\n' "show-options -wqv after -uw" "$unset_out"; fail=$((fail+1))
fi
ck "set-option -w automatic-rename off"   ''       T set-option -w -t "$WID" automatic-rename off
ck "set-option -w allow-rename off"       ''       T set-option -w -t "$WID" allow-rename off

# --- reads with NO client attached (gang runs from a plain shell too) --------
ck "display-message -p -t @id '#W'"                nonempty T display-message -p -t "$WID" '#W'
ck "display-message -p -t @id '#{window_activity}'" nonempty T display-message -p -t "$WID" '#{window_activity}'
ck "display-message -p -t @id '#{cursor_y}'"       nonempty T display-message -p -t "$WID" '#{cursor_y}'

# window_activity must be epoch seconds gang can compare against date +%s
act="$(T display-message -p -t "$WID" '#{window_activity}' 2>/dev/null)"
now="$(date +%s)"
case "$act" in
  ''|*[!0-9]*) printf 'FAIL %-46s not an integer: %s\n' "window_activity is epoch seconds" "$act"; fail=$((fail+1)) ;;
  *) if [ "$act" -le "$now" ] && [ "$act" -gt $((now - 86400)) ]; then
       printf 'OK   %-46s %s vs now %s\n' "window_activity is epoch seconds" "$act" "$now"; pass=$((pass+1))
     else
       printf 'FAIL %-46s %s vs now %s\n' "window_activity is epoch seconds" "$act" "$now"; fail=$((fail+1))
     fi ;;
esac

# --- captures ---------------------------------------------------------------
sleep 1   # let the pane paint before asking what is on it
capck() { # $1 = label, rest = capture command; the pane printed VOCAB_MARK
  local label="$1"; shift
  local out; out="$("$@" 2>&1)"
  if printf '%s\n' "$out" | grep -qF VOCAB_MARK; then
    printf 'OK   %-46s sees the pane text\n' "$label"; pass=$((pass+1))
  else
    printf 'FAIL %-46s no VOCAB_MARK in capture\n' "$label"; fail=$((fail+1))
  fi
}
capck "capture-pane -p -t @id"     T capture-pane -p -t "$WID"
capck "capture-pane -pJ -t @id"    T capture-pane -pJ -t "$WID"
capck "capture-pane -pJ -e -t @id" T capture-pane -pJ -e -t "$WID"

# --- buffer from stdin, then a bracketed paste ------------------------------
if printf 'probe\n' | T load-buffer -b vb - 2>/dev/null; then
  printf 'OK   %-46s\n' "load-buffer -b name - (stdin)"; pass=$((pass+1))
else
  printf 'FAIL %-46s\n' "load-buffer -b name - (stdin)"; fail=$((fail+1))
fi
ck "paste-buffer -p -d -b name -t @id" '' T paste-buffer -p -d -b vb -t "$WID"

# --- keys -------------------------------------------------------------------
ck "send-keys -t @id C-u"  '' T send-keys -t "$WID" C-u
ck "send-keys -t @id Enter" '' T send-keys -t "$WID" Enter
ck "send-keys -t @id literal-text" '' T send-keys -t "$WID" "some text"

# --- teardown verbs gang uses ----------------------------------------------
ck "kill-window -t @id"          '' T kill-window -t "$W2"
ck "kill-session -t '=session'"  '' T kill-session -t "=$S"

printf 'summary=%s ok, %s fail\n' "$pass" "$fail"
printf 'vocab_pass=%s\nvocab_fail=%s\n' "$pass" "$fail"
