#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Drives bin/gang against a real tmux server with the bash profile. No mocks:
# the principals under test are tmux and the script itself, and every case here
# is a bug that shipped once.
#
#   test/integration.sh
set -uo pipefail   # deliberately not -e: a failed assertion reports, not aborts

GANG="$(cd "$(dirname "$(readlink -f "$0")")/.." && pwd)/bin/gang"
export GANG_SESSION="gangtest-$$"
trap 'tmux kill-session -t "$GANG_SESSION" 2>/dev/null' EXIT

SHIM="$(mktemp -d)"
trap 'tmux kill-session -t "$GANG_SESSION" 2>/dev/null; rm -rf "$SHIM"' EXIT
# A tmux that silently swallows paste-buffer and passes everything else through.
# Delivery failing loudly is the one claim worth a fault injector: the failure
# it guards against is by nature silent.
cat > "$SHIM/tmux" <<SH
#!/usr/bin/env bash
[ "\${1:-}" = paste-buffer ] && exit 0
exec "$(command -v tmux)" "\$@"
SH
chmod +x "$SHIM/tmux"

fails=0
check() { # $1 = what, $2 = expected, $3 = actual
  if [ "$2" = "$3" ]; then
    printf 'ok   %s\n' "$1"
  else
    printf 'FAIL %s\n       expected: [%s]\n       actual:   [%s]\n' "$1" "$2" "$3"
    fails=$((fails + 1))
  fi
}

id_of() { # $1 = window NAME -> @id, so the test can address a window named "1"
  local id name
  while read -r id name; do
    [ "$name" = "$1" ] && { printf '%s' "$id"; return 0; }
  done < <(tmux list-windows -t "$GANG_SESSION" -F '#{window_id} #W')
  return 1
}

pane_of() { tmux capture-pane -pJ -t "$(id_of "$1")"; }
has() { if pane_of "$1" | grep -qF -- "$2"; then echo yes; else echo no; fi; }
paint() { # $1 = window name, $2 = beacon line the profile reads back
  tmux send-keys -t "$(id_of "$1")" "printf '%s\\n' '$2'" Enter; sleep 0.4
}

# --- lifecycle ---------------------------------------------------------------

"$GANG" spawn alpha -p bash -d /tmp >/dev/null
check "spawn registers an agent" "idle" "$("$GANG" status alpha)"
check "roster lists it"          "alpha bash idle" \
  "$("$GANG" roster | awk '$1=="alpha"{print $1, $2, $3}')"

# --- delivery ----------------------------------------------------------------

"$GANG" send alpha --from tester "MARK_ONE" >/dev/null
sleep 0.5
check "send lands in the pane" "yes" "$(has alpha "[gang:tester] MARK_ONE")"

# Verification counts evidence before and after the paste. If it just asked
# "is this text anywhere on screen", the second of two identical sends would
# verify against the first one's echo and submit nothing.
"$GANG" send alpha --from tester "MARK_TWICE" >/dev/null; sleep 0.5
c1="$(pane_of alpha | grep -cF -- MARK_TWICE)"
"$GANG" send alpha --from tester "MARK_TWICE" >/dev/null; sleep 0.5
c2="$(pane_of alpha | grep -cF -- MARK_TWICE)"
check "a repeat of an identical message still lands" "grew" \
  "$([ "$c2" -gt "$c1" ] && echo grew || echo stalled)"

# ...and the other half of that: with the same text already on screen from the
# send above, a paste that never lands must NOT verify against the old echo.
PATH="$SHIM:$PATH" "$GANG" send alpha --from tester "MARK_TWICE" >/dev/null 2>&1
check "a paste that never lands is not reported as delivered" "1" "$?"

# --- addressing --------------------------------------------------------------

# tmux reads an all-digit target as a window INDEX. alpha is at index 1, so a
# name-built target sent "1" to alpha and called it delivered.
"$GANG" spawn beta -p bash -d /tmp >/dev/null
"$GANG" spawn 1    -p bash -d /tmp >/dev/null
"$GANG" send 1 --from tester "MARK_NUMERIC" >/dev/null
sleep 0.5
check "a numeric name reaches its agent"  "yes" "$(has 1 MARK_NUMERIC)"
check "and no one else"                   "no"  "$(has alpha MARK_NUMERIC)"

# A rename must not re-point an in-flight command at a different window.
tmux rename-window -t "$(tmux list-windows -t "$GANG_SESSION" -F '#{window_id} #W' |
  awk '$2=="beta"{print $1}')" gamma
check "an agent renamed outside gang is addressable by its new name" "idle" \
  "$("$GANG" status gamma)"

# --- roles -----------------------------------------------------------------

check "roles are listed" "manager reviewer worker" \
  "$("$GANG" roles | tr '\n' ' ' | sed 's/ $//')"

"$GANG" spawn scout -p bash -r worker -d /tmp >/dev/null
sleep 0.5
check "a spawned agent is briefed" "yes" \
  "$(has scout '[gang:spawn] You are `scout` on a gangline team, in the worker role')"
check "the brief is pointed at, not pasted" "yes" "$(has scout "${GANG%/bin/gang}/roles/worker.md")"

"$GANG" spawn ghostrole -p bash -r nosuch -d /tmp >/dev/null 2>&1
check "an unknown role fails" "1" "$?"
check "before anything is spawned" "" "$("$GANG" roster | awk '$1=="ghostrole"{print $1}')"

# GANG_ROLES is the extension point: your directory wins over the shipped one.
mkdir -p "$SHIM/custom-roles"
printf '# Role: worker\n\nCustom.\n' > "$SHIM/custom-roles/worker.md"
GANG_ROLES="$SHIM/custom-roles" "$GANG" spawn custom -p bash -r worker -d /tmp >/dev/null
sleep 0.5
check "GANG_ROLES overrides the shipped brief" "yes" \
  "$(has custom "$SHIM/custom-roles/worker.md")"

# --- readiness ---------------------------------------------------------------

# An agent is not ready the moment its window exists. A real TUI paints nothing
# for the first seconds, and an EMPTY pane is a perfectly STABLE pane — so a
# quiet-only readiness test fires before the harness reads a byte of stdin and
# the brief pastes into a process that is not listening. This profile is blank
# for three seconds and then paints its box, the way a real one boots.
mkdir -p "$SHIM/custom-profiles"
fake_harness() { # $1 = profile name, $2 = launch command; input box shaped like a TUI's
  cat > "$SHIM/custom-profiles/$1.sh" <<SH
GANG_LAUNCH="$2"
GANG_BUSY_REGEX=""
GANG_VERIFIED_VERSIONS="any"
profile_input() {
  local line
  line="\$(tmux capture-pane -pJ -t "\$1" | grep '^❯' | tail -1)" || return 1
  printf '%s' "\${line#❯}" | tr -d '\302\240'
}
SH
}
fake_harness slowboot "sleep 3; PS1='❯ ' bash --norc"

check "GANG_PROFILES adds a harness" "yes" \
  "$(GANG_PROFILES="$SHIM/custom-profiles" "$GANG" profiles | grep -qx slowboot && echo yes || echo no)"
GANG_PROFILES="$SHIM/custom-profiles" "$GANG" spawn slowpoke -p slowboot -r worker -d /tmp >/dev/null
check "a brief waits for a harness that has not painted yet" "yes" \
  "$(has slowpoke 'in the worker role')"

# patrol runs from cron, without the GANG_PROFILES that spawned this agent. It
# must still account for it: an agent missing from the roster reads as no agent.
check "an unresolvable profile is reported, not dropped" "slowpoke slowboot" \
  "$("$GANG" roster | awk '$1=="slowpoke"{print $1, $2}')"

# A first-run modal — Claude Code's "Do you trust the files in this folder?" —
# draws a "❯" of its own, indented, as a menu cursor. It is not an input box,
# and a brief pasted into a security prompt answers it. Refuse, and say so.
fake_harness modal "printf '\n ❯ 1. Yes, I trust this folder\n   2. No, exit\n'; sleep 60"
out="$(GANG_PROFILES="$SHIM/custom-profiles" GANG_BOOT_TIMEOUT=3 \
  "$GANG" spawn modalagent -p modal -r worker -d /tmp 2>&1)"
check "a brief is never pasted into a first-run dialog" "yes" \
  "$(case "$out" in *"other than its input box"*) echo yes ;; *) echo no ;; esac)"
check "and the dialog is left untouched" "no" "$(has modalagent 'gang:spawn')"

# --- context bands -------------------------------------------------------------

# The bash profile reads the same beacon shape the claude-code statusline paints,
# so one printed line exercises the whole warn path with no harness installed.
"$GANG" spawn ctxagent -p bash -d /tmp >/dev/null
paint ctxagent 'ctx 150k/200k 75%'      # crosses the 120000 band, nothing above it
check "context reads the beacon" "150k/200k (75%)" "$("$GANG" context ctxagent)"

# patrol prints "%-16s %-18s %s", so the verdict starts at column 37
verdict() { awk -v n="$1" '$1==n { print substr($0, 37) }'; }

check "patrol nudges past a band" "NUDGED (crossed the 120000-token band)" \
  "$("$GANG" patrol | verdict ctxagent)"
check "the nudge reaches the pane" "yes" "$(has ctxagent '[gang:patrol] [context-usage]')"
check "a second sweep holds its peace" "steady (band 1)" \
  "$("$GANG" patrol | verdict ctxagent)"

# The in-turn leg shares that band memory, so it must not re-warn what patrol
# already warned about — one note per band, not one per leg.
p="$(tmux list-panes -t "$(id_of ctxagent)" -F '#{pane_id}')"
hook() { printf '{"hook_event_name":"PostToolUse"}' | TMUX_PANE="$p" "$GANG" context-hook; }
check "the hook is quiet on a band patrol already warned about" "" "$(hook)"
tmux set-option -w -t "$p" @gl_band 0
check "the hook warns on a fresh band" "yes" \
  "$(case "$(hook)" in *additionalContext*'120000-token band'*) echo yes ;; *) echo no ;; esac)"
check "and advances the shared band memory" "1" "$(tmux show-options -wqv -t "$p" @gl_band)"

# --- the band ladder ---------------------------------------------------------

# Every default rung is absolute, so each must add exactly one band at any window
# size. The proportional rung this replaced was 90%, which on a 200k window WAS
# 180000 — two rungs firing at once, band 1 jumping straight to 3 — and on a 1M
# window sat at 900k, leaving 250k..900k without a single further signal.
probe() { "$GANG" spawn "$1" -p bash -d /tmp >/dev/null; paint "$1" "ctx $2"; }
probe rung200a '119k/200k 59%'    # under the first rung
probe rung200b '120k/200k 60%'
probe rung200c '180k/200k 90%'    # 90% of this window: must not double-count
probe rung1ma  '249k/1000k 24%'
probe rung1mb  '250k/1000k 25%'
probe rung1mc  '350k/1000k 35%'   # the region a proportional ladder left silent
"$GANG" patrol >/dev/null         # first sweep nudges and records the band
settled="$("$GANG" patrol)"       # second reports it
band_of() { printf '%s\n' "$settled" | awk -v n="$1" '$1==n { print $NF }' | tr -d ')'; }

check "no rung crossed is band 0"        "0" "$(band_of rung200a)"
check "one rung is one band"             "1" "$(band_of rung200b)"
check "90% of a 200k window is not a rung of its own" "2" "$(band_of rung200c)"
check "a 1M agent below 250k sits lower" "2" "$(band_of rung1ma)"
check "and steps at 250k"                "3" "$(band_of rung1mb)"
check "and again at 350k"                "4" "$(band_of rung1mc)"

# --- refusals ----------------------------------------------------------------

"$GANG" send alpha "no identity here" >/dev/null 2>&1
check "send without --from is refused" "1" "$?"
"$GANG" send ghost --from tester hi >/dev/null 2>&1
check "send to an unknown agent fails" "1" "$?"
"$GANG" spawn zed -p >/dev/null 2>&1
check "a flag missing its value fails cleanly" "1" "$?"
check "and spawns nothing" "" "$("$GANG" roster | awk '$1=="zed"{print $1}')"
"$GANG" --help >/dev/null
check "--help exits clean" "0" "$?"

# --- teardown ----------------------------------------------------------------

"$GANG" kill gamma >/dev/null
check "kill removes the agent" "" "$("$GANG" roster | awk '$1=="gamma"{print $1}')"
"$GANG" down >/dev/null
check "down kills the session" "no team (session '$GANG_SESSION' not running)" \
  "$("$GANG" roster)"

printf '\n%s\n' "$([ "$fails" -eq 0 ] && echo "all checks passed" || echo "$fails check(s) failed")"
[ "$fails" -eq 0 ]
