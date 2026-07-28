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
  # Never return empty. An empty -t is not "no window", it is tmux's CURRENT
  # pane — and this suite is normally run by an agent from inside a live gangline
  # session, so a missing window turns `paint` into keystrokes typed straight
  # into whichever teammate is on screen. Lived it: a failing hitch sent the
  # band-ladder beacons into the manager's input box, where they read as the
  # operator talking, and the rename below retitled the manager to "gamma".
  # A test that cannot find its own window is broken, and a broken test must not
  # be allowed to drive someone else's agent.
  printf 'BUG: no window named %s in %s — refusing to fall through to the active pane\n' \
    "$1" "$GANG_SESSION" >&2
  exit 1
}

pane_of() { tmux capture-pane -pJ -t "$(id_of "$1")"; }
has() { if pane_of "$1" | grep -qF -- "$2"; then echo yes; else echo no; fi; }
target_of() { # $1 = window name; a NON-EMPTY @id or the suite stops
  # id_of's own `exit 1` only leaves the command substitution it runs in, so a
  # caller that interpolates it directly still hands tmux an empty -t. Anything
  # that WRITES to a pane resolves through here first, in the main shell.
  local id; id="$(id_of "$1")" || exit 1
  [ -n "$id" ] || { printf 'BUG: empty target for %s\n' "$1" >&2; exit 1; }
  printf '%s' "$id"
}

paint() { # $1 = window name, $2 = beacon line the profile reads back
  local id; id="$(target_of "$1")" || exit 1
  tmux send-keys -t "$id" "printf '%s\\n' '$2'" Enter; sleep 0.4
}

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

# --- finding its own tree ------------------------------------------------------

# `readlink -f` is GNU-only; a stock macOS readlink rejects -f. gang is normally
# invoked through ~/.local/bin/gang, a symlink into the install tree, so resolving
# that link is how it finds profiles/ and roles/ at all. When -f failed the
# substitution came back empty, ROOT became the parent of the CALLER's cwd, and
# gang listed no harnesses while exiting 0 — install.sh checked only the exit
# status and printed "gang installed". A silently broken install on stock Mac.
cat > "$SHIM/readlink" <<SH
#!/usr/bin/env bash
[ "\${1:-}" = -f ] && { echo "readlink: illegal option -- f" >&2; exit 1; }
exec "$(command -v readlink)" "\$@"
SH
chmod +x "$SHIM/readlink"
ln -s "$GANG" "$SHIM/gang-via-symlink"
# From /tmp, so a ROOT derived from the caller's cwd cannot accidentally be right.
out="$(cd /tmp && PATH="$SHIM:$PATH" "$SHIM/gang-via-symlink" profiles 2>&1 | tr '\n' ' ')"
check "a BSD readlink still resolves the install tree" "yes" \
  "$(case "$out" in *bash*claude-code*pi*) echo yes ;; *) echo no ;; esac)"

# And when the tree really is absent, that is said out loud rather than reported
# as an install with zero harnesses.
cp "$GANG" "$SHIM/orphan-gang"
out="$(cd /tmp && "$SHIM/orphan-gang" profiles 2>&1)"; rc=$?
check "a gang with no tree beside it fails loudly" "1" "$rc"
check "and names what is missing" "yes" \
  "$(case "$out" in *"not a gangline tree"*) echo yes ;; *) echo no ;; esac)"

# --- lifecycle ---------------------------------------------------------------

"$GANG" hitch alpha -p bash -d /tmp >/dev/null
check "hitch registers an agent" "idle (slack tug)" "$("$GANG" status alpha)"
check "roster lists it"          "alpha bash idle" \
  "$("$GANG" roster | awk '$1=="alpha"{print $1, $2, $3}')"

# `gang up` is the first command a new install runs, and the only one that both
# hitches and briefs with no arguments at all — so it is the one whose breakage a
# stranger meets first. It ends in an exec that hands the terminal over, which is
# why it went untested: drive it through a tmux that swallows the handover and
# assert what it did before reaching it.
mkdir -p "$SHIM/attach"
cat > "$SHIM/attach/tmux" <<SH
#!/usr/bin/env bash
case "\${1:-}" in attach|switch-client) exit 0 ;; esac
exec "$(command -v tmux)" "\$@"
SH
chmod +x "$SHIM/attach/tmux"

PATH="$SHIM/attach:$PATH" "$GANG" up -p bash -d /tmp >/dev/null
check "up needs no agent name"  "idle (slack tug)" "$("$GANG" status lead)"
sleep 0.5
check "and briefs it as lead" "yes" "$(has lead roles/lead.md)"

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

# A real TUI does not echo a paste back — it COLLAPSES it into a placeholder, and
# the shape of that placeholder is the harness's own business. Claude Code writes
# "[Pasted text #1 +10 lines]", counting lines BEYOND the first, and drops the
# count entirely for a long single-line paste ("[Pasted text #2]"); Pi writes
# "[paste #N +13 lines]", counting every line. Verification that scrapes the pane
# for the literal text, or for any one harness's placeholder, quietly stops
# verifying real sends to every other harness — and because it dies BEFORE the
# submit, the message is left parked in the agent's input box, which reads to the
# operator as "nothing was sent" while the agent sits on an unsent draft.
# The box changing is the harness-independent fact underneath all of them.
cat > "$SHIM/collapsing-tui" <<'SH'
#!/usr/bin/env bash
stty -echo 2>/dev/null
n=0
printf '\033[2J\033[H❯ \n'
while IFS= read -r _; do
  n=$((n + 1))
  printf '\033[2J\033[H❯ [Pasted text #%d]\n' "$n"
done
SH
chmod +x "$SHIM/collapsing-tui"
fake_harness collapsing "$SHIM/collapsing-tui"
export GANG_PROFILES="$SHIM/custom-profiles"
"$GANG" hitch collapser -p collapsing -d /tmp >/dev/null 2>&1
"$GANG" send collapser --from tester "$(printf 'first line\nsecond line\nthird line')" \
  >/dev/null 2>&1
check "a paste the TUI collapses instead of echoing still verifies" "0" "$?"
# Guards the fixture: if this stand-in ever echoed the paste, the check above
# would pass on the literal and prove nothing about collapsed ones.
check "and the pane never showed the literal text" "no" "$(has collapser 'second line')"

# The post-Enter check used to take one unreadable box as a verdict, and there is
# a harness where that is exactly backwards: opencode's box counts as readable
# only while the hardware cursor sits in it, and the cursor steps out while the
# accepted message is repainted into the transcript. The check failed BECAUSE the
# submit it was looking for had happened, and a hitch briefing died with
# "submission unverifiable" on a message that landed — leaving the agent running
# with no role brief. Unreadable is an absence of evidence, so it buys another
# look; only a box that reads back UNCHANGED is evidence of a non-submit.
# This stand-in is blind for its two looks after Enter, the way a loaded box is.
cat > "$SHIM/custom-profiles/blinking.sh" <<SH
GANG_LAUNCH="PS1='❯ ' bash --norc"
GANG_BUSY_REGEX=""
GANG_VERIFIED_VERSIONS="any"
profile_input() {
  local n until line
  n=\$(( \$(cat "$SHIM/blink-count" 2>/dev/null || echo 0) + 1 ))
  echo "\$n" > "$SHIM/blink-count"
  until=\$(cat "$SHIM/blink-until" 2>/dev/null || echo 0)
  # 1 and 2 are the before/after reads around the paste; the blindness starts
  # at 3, the first look after Enter.
  if [ "\$n" -ge 3 ] && [ "\$n" -le "\$until" ]; then return 1; fi
  line="\$(tmux capture-pane -pJ -t "\$1" | grep '^❯' | tail -1)" || return 1
  printf '%s' "\${line#❯}"
}
SH
export GANG_PROFILES="$SHIM/custom-profiles"
"$GANG" hitch blinker -p blinking -d /tmp >/dev/null 2>&1
echo 0 > "$SHIM/blink-count"; echo 4 > "$SHIM/blink-until"
"$GANG" send blinker --from tester "BLINK_MSG" >/dev/null 2>&1
check "a box briefly unreadable after Enter still verifies" "0" "$?"
check "and the message actually landed" "yes" "$(has blinker '[gang:tester] BLINK_MSG')"

# The tolerance is bounded, not infinite: a box that never comes back still fails,
# and says which of the two things went wrong.
echo 0 > "$SHIM/blink-count"; echo 9999 > "$SHIM/blink-until"
out="$("$GANG" send blinker --from tester "NEVER" 2>&1)"; rc=$?
check "a box that never comes back still fails loudly" "1" "$rc"
check "and is not accused of holding an unsent draft" "unverifiable" \
  "$(case "$out" in *unverifiable*) echo unverifiable ;; *"never sent"*) echo wrong-verdict ;; *) echo other ;; esac)"
echo 0 > "$SHIM/blink-until"
unset GANG_PROFILES

# Enter has to be checked, not merely sent. Batch the text and the Enter into one
# send-keys and Claude Code reads the burst as a paste and the trailing newline
# as part of it: the message sits in the input box as an unsent draft that
# scrollback renders identically to a sent one. The paste verifies, the
# submission never happened, and the sender walks away believing it did.
mkdir -p "$SHIM/noenter"
cat > "$SHIM/noenter/tmux" <<SH
#!/usr/bin/env bash
if [ "\${1:-}" = send-keys ]; then
  for a in "\$@"; do [ "\$a" = Enter ] && exit 0; done
fi
exec "$(command -v tmux)" "\$@"
SH
chmod +x "$SHIM/noenter/tmux"
PATH="$SHIM/noenter:$PATH" "$GANG" send alpha --from tester "MARK_UNSENT" >/dev/null 2>&1
check "a paste that is never submitted is not reported as delivered" "1" "$?"
tmux send-keys -t "$(target_of alpha)" C-u   # the draft it correctly left behind

# --- reaching an agent that is working ---------------------------------------

# Busy was a refusal, and the refusal was the bug: a manager mid-turn was
# unreachable, --wait burned the caller's whole turn waiting on one that never
# went idle, and an agent — busy by definition while it is deciding anything —
# could not drive its own compaction. Busy does not decide whether a message can
# be delivered; gang measures that in the pane, before and after. What it decides
# is where the keystrokes LAND, and that is the harness's property to declare.
cat > "$SHIM/custom-profiles/working.sh" <<SH
GANG_LAUNCH="PS1='❯ ' bash --norc"
GANG_BUSY_REGEX="WORKING\\.\\.\\.|COMPACTING\\.\\.\\."
GANG_COMPACT_CMD="/compact"
GANG_COMPACTING_REGEX="COMPACTING\\.\\.\\."
GANG_GATED_REGEX="Do you want to proceed\\?"
GANG_MIDTURN_INPUT="\${FAKE_QUEUES:-}"
GANG_VERIFIED_VERSIONS="any"
profile_input() {
  local line
  line="\$(tmux capture-pane -pJ -t "\$1" | grep '^❯' | tail -1)" || return 1
  printf '%s' "\${line#❯}" | tr -d '\302\240'
}
SH
export GANG_PROFILES="$SHIM/custom-profiles"
"$GANG" hitch busybee -p working -d /tmp >/dev/null 2>&1
sleep 0.5
paint busybee 'WORKING...'
check "the stand-in reads as busy" "busy (tight tug)" "$("$GANG" status busybee)"

# A capture that fails is not a state. `status` names the state through a command
# substitution now, and the exit status of a substitution sitting in an argument
# list is discarded — so an unreadable pane could print a blank line and report
# success for an agent nobody was able to look at (law 8).
mkdir -p "$SHIM/noread"
cat > "$SHIM/noread/tmux" <<SH
#!/usr/bin/env bash
[ "\${1:-}" = capture-pane ] && exit 1
exec "$(command -v tmux)" "\$@"
SH
chmod +x "$SHIM/noread/tmux"
out="$(PATH="$SHIM/noread:$PATH" "$GANG" status busybee 2>&1)"; rc=$?
check "a pane gang cannot read is not a state" "1" "$rc"
check "and it says so rather than printing a blank line" "yes" \
  "$(case "$out" in *"refusing to guess"*) echo yes ;; *) echo no ;; esac)"

FAKE_QUEUES=1 "$GANG" send busybee --from tester "MARK_QUEUED" >/dev/null 2>&1
check "a harness that queues input takes mail mid-turn" "0" "$?"
sleep 0.5
check "and it really landed" "yes" "$(has busybee "[gang:tester] MARK_QUEUED")"

"$GANG" send busybee --from tester "MARK_UNQUEUED" >/dev/null 2>&1
check "one that does not is still refused" "1" "$?"

# Compacting yourself is the extreme case of a busy target: the turn in the way
# is the caller's own and it ends the moment the command returns. Self is told
# from peer by the pane id tmux exports into every pane it starts — checked
# against the live session, because that variable survives its pane.
selfpane="$(tmux list-panes -t "$(target_of busybee)" -F '#{pane_id}')"
TMUX_PANE="$selfpane" "$GANG" compact busybee --from tester >/dev/null 2>&1
check "an agent can compact itself mid-turn" "0" "$?"
"$GANG" compact busybee --from tester >/dev/null 2>&1
check "but a peer's live turn is still not cut" "1" "$?"
TMUX_PANE="%99999" "$GANG" compact busybee --from tester >/dev/null 2>&1
check "and a stale pane id is not mistaken for self" "1" "$?"

# A resume cannot ride the input queue behind its own compaction: queued text can
# be handed to the turn already running while a queued slash command waits for
# that turn to end, so the resume overtakes the compaction and is eaten by the
# very turn that was about to be compacted. It is delivered afterwards instead,
# and not until the pane has been quiet long enough that it cannot be landing in
# the gap between the turn ending and compaction starting to paint.
GANG_RESUME_TIMEOUT=60 TMUX_PANE="$selfpane" \
  "$GANG" compact busybee --from tester --resume "MARK_RESUMED" >/dev/null 2>&1
sleep 2
check "a resume waits while the agent is still busy" "no" "$(has busybee MARK_RESUMED)"
tmux send-keys -t "$(target_of busybee)" clear Enter   # compaction "finishes"
sleep 22
check "and lands once the pane settles" "yes" "$(has busybee "[gang:tester] MARK_RESUMED")"

# Waiting for quiet is the fallback, not the goal. A compaction that is visibly
# running is already past the turn that would have eaten the resume, and reads no
# input itself, so the message can go straight into the queue it drains on the way
# out. The clock is the assertion: the quiet path cannot deliver before its ten
# second floor, so anything that lands inside seven took the other branch.
paint busybee 'WORKING...'
GANG_RESUME_TIMEOUT=60 TMUX_PANE="$selfpane" \
  "$GANG" compact busybee --from tester --resume "MARK_FAST" >/dev/null 2>&1
sleep 2
check "a resume still holds while a turn that could eat it runs" "no" "$(has busybee MARK_FAST)"
tmux send-keys -t "$(target_of busybee)" clear Enter   # that turn ends...
paint busybee 'COMPACTING...'                          # ...and the compaction starts
sleep 5
check "and goes in the moment the compaction itself is running" "yes" "$(has busybee "[gang:tester] MARK_FAST")"

# --- permission gates ---------------------------------------------------------

# A harness stopped at an approval prompt is neither working nor reachable: it
# is waiting on a human, and keystrokes sent to it land IN the dialog, where
# they answer it. Every gate watched live also drops the busy hint and the
# input box, so before the gated marker such an agent read idle and the team
# stalled with nothing on any surface saying why. Gated is checked before busy
# because a gate paints mid-turn — the busy marker can still be on screen.

# patrol prints "%-16s %-18s %s", so the verdict starts at column 37
verdict() { awk -v n="$1" '$1==n { print substr($0, 37) }'; }

paint busybee 'WORKING... Do you want to proceed?'
check "a permission prompt reads as gated, even beside a busy marker" \
  "gated (hook set)" "$("$GANG" status busybee)"
check "roster shows the gate" "gated" \
  "$("$GANG" roster | awk '$1=="busybee"{print $3}')"
out="$(FAKE_QUEUES=1 "$GANG" send busybee --from tester "MARK_GATED" 2>&1)"; rc=$?
check "a send to a gated agent is refused, even where mid-turn input queues" "1" "$rc"
check "and says the prompt owns the screen" "yes" \
  "$(case "$out" in *"hook set"*) echo yes ;; *) echo no ;; esac)"
check "and nothing was typed into the dialog" "no" "$(has busybee MARK_GATED)"
out="$("$GANG" wait busybee 30 2>&1)"; rc=$?
check "wait on a gated agent fails loud, not slow" "1" "$rc"
check "naming the gate rather than timing out" "yes" \
  "$(case "$out" in *"hook set"*) echo yes ;; *) echo no ;; esac)"
out="$("$GANG" compact busybee --from tester 2>&1)"
check "compact on a gated agent names the gate, not the turn" "yes" \
  "$(case "$out" in *"hook set"*) echo yes ;; *) echo no ;; esac)"
check "patrol reports it for the operator instead of skipping it" \
  "GATED (hook set) — a permission prompt is waiting on the operator (gang attach)" \
  "$("$GANG" patrol | verdict busybee)"
tmux send-keys -t "$(target_of busybee)" clear Enter
sleep 0.5
check "an answered prompt reads idle again" "idle (slack tug)" "$("$GANG" status busybee)"
unset GANG_PROFILES

# --- addressing --------------------------------------------------------------

# tmux reads an all-digit target as a window INDEX. alpha is at index 1, so a
# name-built target sent "1" to alpha and called it delivered.
"$GANG" hitch beta -p bash -d /tmp >/dev/null
"$GANG" hitch 1    -p bash -d /tmp >/dev/null
"$GANG" send 1 --from tester "MARK_NUMERIC" >/dev/null
sleep 0.5
check "a numeric name reaches its agent"  "yes" "$(has 1 MARK_NUMERIC)"
check "and no one else"                   "no"  "$(has alpha MARK_NUMERIC)"

# A rename must not re-point an in-flight command at a different window.
# Addressed through id_of, which refuses to yield an empty target: the inline
# awk this replaced printed nothing when beta was missing, and the rename then
# landed on the active pane.
beta_id="$(target_of beta)" || exit 1
tmux rename-window -t "$beta_id" gamma
check "an agent renamed outside gang is addressable by its new name" "idle (slack tug)" \
  "$("$GANG" status gamma)"

# --- roles -----------------------------------------------------------------

check "roles are listed" "lead reviewer worker" \
  "$("$GANG" roles | tr '\n' ' ' | sed 's/ $//')"

"$GANG" hitch scout -p bash -r worker -d /tmp >/dev/null
sleep 0.5
check "a hitched agent is briefed" "yes" \
  "$(has scout '[gang:hitch] You are `scout` on a gangline team, in the worker role')"
check "the brief is pointed at, not pasted" "yes" "$(has scout "${GANG%/bin/gang}/roles/worker.md")"

"$GANG" hitch ghostrole -p bash -r nosuch -d /tmp >/dev/null 2>&1
check "an unknown role fails" "1" "$?"
check "before anything is hitched" "" "$("$GANG" roster | awk '$1=="ghostrole"{print $1}')"

# GANG_ROLES is the extension point: your directory wins over the shipped one.
mkdir -p "$SHIM/custom-roles"
printf '# Role: worker\n\nCustom.\n' > "$SHIM/custom-roles/worker.md"
GANG_ROLES="$SHIM/custom-roles" "$GANG" hitch custom -p bash -r worker -d /tmp >/dev/null
sleep 0.5
check "GANG_ROLES overrides the shipped brief" "yes" \
  "$(has custom "$SHIM/custom-roles/worker.md")"

# --- readiness ---------------------------------------------------------------

# An agent is not ready the moment its window exists. A real TUI paints nothing
# for the first seconds, and an EMPTY pane is a perfectly STABLE pane — so a
# quiet-only readiness test fires before the harness reads a byte of stdin and
# the brief pastes into a process that is not listening. This profile is blank
# for three seconds and then paints its box, the way a real one boots.
fake_harness slowboot "sleep 3; PS1='❯ ' bash --norc"

check "GANG_PROFILES adds a harness" "yes" \
  "$(GANG_PROFILES="$SHIM/custom-profiles" "$GANG" profiles | grep -qx slowboot && echo yes || echo no)"
GANG_PROFILES="$SHIM/custom-profiles" "$GANG" hitch slowpoke -p slowboot -r worker -d /tmp >/dev/null
check "a brief waits for a harness that has not painted yet" "yes" \
  "$(has slowpoke 'in the worker role')"

# patrol runs from cron, without the GANG_PROFILES that hitched this agent. It
# must still account for it: an agent missing from the roster reads as no agent.
check "an unresolvable profile is reported, not dropped" "slowpoke slowboot" \
  "$("$GANG" roster | awk '$1=="slowpoke"{print $1, $2}')"

# A first-run modal — Claude Code's "Do you trust the files in this folder?" —
# draws a "❯" of its own, indented, as a menu cursor. It is not an input box,
# and a brief pasted into a security prompt answers it. Refuse, and say so.
fake_harness modal "printf '\n ❯ 1. Yes, I trust this folder\n   2. No, exit\n'; sleep 60"
out="$(GANG_PROFILES="$SHIM/custom-profiles" GANG_BOOT_TIMEOUT=3 \
  "$GANG" hitch modalagent -p modal -r worker -d /tmp 2>&1)"
check "a brief is never pasted into a first-run dialog" "yes" \
  "$(case "$out" in *"other than its input box"*) echo yes ;; *) echo no ;; esac)"
check "and the dialog is left untouched" "no" "$(has modalagent 'gang:hitch')"

# --- context bands -------------------------------------------------------------

# The bash profile reads the same beacon shape the claude-code statusline paints,
# so one printed line exercises the whole warn path with no harness installed.
"$GANG" hitch ctxagent -p bash -d /tmp >/dev/null
paint ctxagent 'ctx 150k/200k 75%'      # crosses the 120000 band, nothing above it
check "context reads the beacon" "150k/200k (75%)" "$("$GANG" context ctxagent)"

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
probe() { "$GANG" hitch "$1" -p bash -d /tmp >/dev/null; paint "$1" "ctx $2"; }
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

# --- file-based context (codex) ----------------------------------------------

# Codex paints no readout a passive observer can reach; its profile reads the
# session rollout on disk instead. The stand-in here swaps only the launch
# command for bash — profile_context, the marker lookup, and the parser are the
# shipped codex.sh code. Every fixture record is shaped exactly as the installed
# codex (0.145.0) writes it, because that shape IS the claim under test: the
# same parser is vet's format gate.
CODEX_FIX="$SHIM/codex-home"
DAYDIR="$CODEX_FIX/sessions/2026/07/27"
mkdir -p "$DAYDIR"
cat > "$SHIM/custom-profiles/codexfile.sh" <<SH
. "${GANG%/bin/gang}/profiles/codex.sh"
GANG_LAUNCH="PS1='❯ ' bash --norc"
GANG_BUSY_REGEX=""
GANG_VERIFIED_VERSIONS="any"
profile_input() {
  local line
  line="\$(tmux capture-pane -pJ -t "\$1" | grep '^❯' | tail -1)" || return 1
  printf '%s' "\${line#❯}" | tr -d '\302\240'
}
SH
export GANG_PROFILES="$SHIM/custom-profiles"

out="$(CODEX_HOME="$CODEX_FIX" "$GANG" hitch filectx -p codexfile -d /tmp 2>&1)"
fkey="$(tmux show-options -wqv -t "$(target_of filectx)" @gl_key)"
check "a keyed profile mints a marker even with no role" "yes" \
  "$(case "$fkey" in gl-????????????????????????????????) echo yes ;; *) echo no ;; esac)"
sleep 0.5
check "the marker reaches the agent's conversation" "yes" "$(has filectx "$fkey")"
# The marker identifies the ONE transcript it was typed into. A spawner that
# printed it would record it in its own transcript too — two matches, no agent.
check "and never the spawner's stdout" "no" \
  "$(case "$out" in *gl-*) echo yes ;; *) echo no ;; esac)"

# The agent's rollout: meta, the marker as user input, then two token_counts —
# the LAST one is the current occupancy, and 150k sits past the 120k band.
{
  printf '%s\n' '{"timestamp":"2026-07-27T00:00:01.000Z","type":"session_meta","payload":{"id":"gangtest-agent","timestamp":"2026-07-27T00:00:01.000Z","cwd":"/tmp","originator":"codex-tui","cli_version":"0.145.0","source":"cli"}}'
  printf '{"timestamp":"2026-07-27T00:00:02.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"[gang:hitch] Session marker: %s — gang bookkeeping, ignore it and never repeat it anywhere."}]}}\n' "$fkey"
  printf '%s\n' '{"timestamp":"2026-07-27T00:00:03.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":15000,"cached_input_tokens":11008,"cache_write_input_tokens":0,"output_tokens":1000,"reasoning_output_tokens":400,"total_tokens":16085},"last_token_usage":{"input_tokens":16031,"cached_input_tokens":11008,"cache_write_input_tokens":0,"output_tokens":54,"reasoning_output_tokens":36,"total_tokens":16085},"model_context_window":258400},"rate_limits":{"limit_id":"codex","primary":{"used_percent":5.0,"window_minutes":10080,"resets_at":1785617494},"plan_type":"pro"}}}'
  printf '%s\n' '{"timestamp":"2026-07-27T00:00:04.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":290000,"cached_input_tokens":250000,"cache_write_input_tokens":0,"output_tokens":9000,"reasoning_output_tokens":2000,"total_tokens":299000},"last_token_usage":{"input_tokens":148000,"cached_input_tokens":140000,"cache_write_input_tokens":0,"output_tokens":2000,"reasoning_output_tokens":500,"total_tokens":150000},"model_context_window":258400},"rate_limits":{"limit_id":"codex","primary":{"used_percent":5.1,"window_minutes":10080,"resets_at":1785617494},"plan_type":"pro"}}}'
} > "$DAYDIR/rollout-2026-07-27T00-00-01-gangtest-agent.jsonl"

# A teammate that captured the agent's pane early re-records the marker inside a
# tool-output record. Different shape, different rollout, different numbers — if
# the lookup ever picks it, the readout below says 222k and the check says so.
{
  printf '{"timestamp":"2026-07-27T00:00:05.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call_x","output":"❯ [gang:hitch] Session marker: %s — gang bookkeeping, ignore it and never repeat it anywhere."}}\n' "$fkey"
  printf '%s\n' '{"timestamp":"2026-07-27T00:00:06.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":222000,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":222000},"last_token_usage":{"input_tokens":222000,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":222000},"model_context_window":258400},"rate_limits":{"limit_id":"codex","primary":{"used_percent":5.2,"window_minutes":10080,"resets_at":1785617494},"plan_type":"pro"}}}'
} > "$DAYDIR/rollout-2026-07-27T00-00-02-gangtest-capturer.jsonl"

check "context reads the last token_count of the agent's own rollout" \
  "150k/258k (58%)" "$(CODEX_HOME="$CODEX_FIX" "$GANG" context filectx)"

# The marker repeated as USER input somewhere else is unresolvable, and lookup
# must refuse rather than guess. The cache would mask the collision — drop it.
printf '{"timestamp":"2026-07-27T00:00:07.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"look at this: Session marker: %s"}]}}\n' \
  "$fkey" > "$DAYDIR/rollout-2026-07-27T00-00-03-gangtest-dup.jsonl"
tmux set-option -w -t "$(target_of filectx)" @gl_session ""
CODEX_HOME="$CODEX_FIX" "$GANG" context filectx >/dev/null 2>&1
check "a repeated marker is refused, not guessed" "1" "$?"
rm "$DAYDIR/rollout-2026-07-27T00-00-03-gangtest-dup.jsonl"

check "a codex agent joins the band ladder" "NUDGED (crossed the 120000-token band)" \
  "$(CODEX_HOME="$CODEX_FIX" "$GANG" patrol | verdict filectx)"

# Doctor runs the same parser as a format gate against the newest rollout that
# has a token_count at all, so a schema change in a codex release is caught the
# day it ships, not the day an agent's readout silently lies.
# Captured, not piped into grep -q: under this suite's pipefail, grep -q exits
# at the match, vet's NEXT row takes SIGPIPE, and the pipeline reads as a
# miss — a false negative that reproduces exactly as often as vet has a row
# to print after the matching one.
dout="$(CODEX_HOME="$CODEX_FIX" "$GANG" vet 2>/dev/null)"
check "vet gates the rollout format" "yes" \
  "$(printf '%s' "$dout" | grep -q 'file format: OK' && echo yes || echo no)"
printf '%s\n' '{"timestamp":"2026-07-27T00:00:08.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0,"total_tokens":2},"last_token_usage":{"input_tokens":1,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0,"total_tokens":2}},"rate_limits":{"limit_id":"codex","primary":{"used_percent":5.3,"window_minutes":10080,"resets_at":1785617494},"plan_type":"pro"}}}' \
  > "$DAYDIR/rollout-2026-07-27T00-00-09-gangtest-drift.jsonl"
dout="$(CODEX_HOME="$CODEX_FIX" "$GANG" doctor 2>/dev/null)"   # through the alias, deliberately
check "and fails loud when the schema drifts" "yes" \
  "$(printf '%s' "$dout" | grep -q 'file format: DRIFT' && echo yes || echo no)"
rm "$DAYDIR/rollout-2026-07-27T00-00-09-gangtest-drift.jsonl"
unset GANG_PROFILES

# --- scraped context with a joined window (opencode) --------------------------

# opencode paints used tokens and a rounded percent into its hint row but never
# the window; its profile joins the window from opencode's own models catalog,
# keyed by the painted composer badge, and cross-checks the painted percent
# against the join. The stand-in swaps launch/busy/input for bash —
# profile_context, the join, the cross-check, and profile_vet are the
# shipped opencode.sh code, and the fixture catalog entries are shaped exactly
# as the installed opencode (1.14.39) caches them. The cursor-gated
# profile_input is deliberately not driven here: it needs a real opencode
# owning a keyboard, and it is live-verified against the installed TUI.
OC_CACHE="$SHIM/oc-cache"
mkdir -p "$OC_CACHE/opencode"
oc_catalog() {
  cat > "$OC_CACHE/opencode/models.json" <<'JSON'
{"github-copilot":{"name":"GitHub Copilot","models":{"gpt-5.5":{"name":"GPT-5.5","limit":{"context":1050000,"input":922000,"output":128000}}}},
 "openai":{"name":"OpenAI","models":{"gpt-5.5":{"name":"GPT-5.5","limit":{"context":400000,"input":272000,"output":128000}}}}}
JSON
}
oc_catalog
cat > "$SHIM/custom-profiles/ocpaint.sh" <<SH
. "${GANG%/bin/gang}/profiles/opencode.sh"
GANG_LAUNCH="PS1='❯ ' bash --norc"
GANG_BUSY_REGEX=""
GANG_VERIFIED_VERSIONS="any"
profile_input() {
  local line
  line="\$(tmux capture-pane -pJ -t "\$1" | grep '^❯' | tail -1)" || return 1
  printf '%s' "\${line#❯}" | tr -d '\302\240'
}
SH
export GANG_PROFILES="$SHIM/custom-profiles"

"$GANG" hitch ocp -p ocpaint -d /tmp >/dev/null
XDG_CACHE_HOME="$OC_CACHE" "$GANG" context ocp >/dev/null 2>&1
check "context before the first turn is refused, not guessed" "1" "$?"

# One printf paints the whole screen block: badge directly above the border,
# hint row after. Painted as separate commands, the shell's command echo would
# sit between badge and border, and the badge walk would read the echo.
tmux send-keys -t "$(target_of ocp)" \
  "printf '%s\n' '┃  Build · GPT-5.5 GitHub Copilot' '╹▀▀▀▀▀▀▀▀' '  150K (14%) · \$0.42  ctrl+p commands'" Enter
sleep 0.4
check "context scrapes the hint row and joins the window from the catalog" \
  "150k/1050k (14%)" "$(XDG_CACHE_HOME="$OC_CACHE" "$GANG" context ocp)"
check "an opencode agent joins the band ladder" "NUDGED (crossed the 120000-token band)" \
  "$(XDG_CACHE_HOME="$OC_CACHE" "$GANG" patrol | verdict ocp)"

# Two catalog rows concatenating to the same "<model name> <provider name>"
# is unresolvable, and the join must refuse rather than guess between windows.
cat > "$OC_CACHE/opencode/models.json" <<'JSON'
{"github-copilot":{"name":"GitHub Copilot","models":{"gpt-5.5":{"name":"GPT-5.5","limit":{"context":1050000,"input":922000,"output":128000}}}},
 "copilot-x":{"name":"Copilot","models":{"gx":{"name":"GPT-5.5 GitHub","limit":{"context":200000,"input":136000,"output":64000}}}}}
JSON
XDG_CACHE_HOME="$OC_CACHE" "$GANG" context ocp >/dev/null 2>&1
check "an ambiguous catalog join is refused, not guessed" "1" "$?"
oc_catalog

# A painted percent the joined window cannot reproduce means the join picked
# the wrong window — model switched under the badge, or catalog drift.
tmux send-keys -t "$(target_of ocp)" \
  "printf '%s\n' '  150K (50%) · \$0.42  ctrl+p commands'" Enter
sleep 0.4
XDG_CACHE_HOME="$OC_CACHE" "$GANG" context ocp >/dev/null 2>&1
check "a percent the joined window cannot reproduce is refused" "1" "$?"

dout="$(XDG_CACHE_HOME="$OC_CACHE" "$GANG" vet 2>/dev/null)"
check "vet gates the catalog format" "yes" \
  "$(printf '%s' "$dout" | grep -q 'file format: OK (catalog join candidates' && echo yes || echo no)"
printf '%s\n' '{"github-copilot":{"name":"GitHub Copilot","models":{"gpt-5.5":{"name":"GPT-5.5"}}}}' \
  > "$OC_CACHE/opencode/models.json"
dout="$(XDG_CACHE_HOME="$OC_CACHE" "$GANG" vet 2>/dev/null)"
check "and fails loud when the catalog drifts" "yes" \
  "$(printf '%s' "$dout" | grep -q 'file format: DRIFT — models catalog holds no named model' && echo yes || echo no)"
oc_catalog
unset GANG_PROFILES

# --- colour ------------------------------------------------------------------

# Every check in this file reads gang through a pipe, where colour is off — so a
# change that emitted no colour anywhere would pass all of them. tmux is the pty:
# a pane is a terminal, and `capture-pane -e` hands back the escape sequences it
# received. Running gang inside alpha's own pane is the only way to see what an
# operator sees.
in_pane() { # $1 = agent whose pane to borrow, $2... = command; -> its raw output
  # Polled, not slept: a roster of this many agents reads a session file per
  # keyed profile, and a fixed sleep caught a screen that was still empty — which
  # a check for "is there colour here" reads as "no", passing for a real failure.
  local id t=0; id="$(target_of "$1")" || exit 1; shift
  tmux send-keys -t "$id" "clear; $* ; echo IN_PANE_DONE" Enter
  while [ "$t" -lt 60 ]; do
    # Anchored, so the shell's echo of the command line is not the marker.
    tmux capture-pane -p -t "$id" | grep -q '^IN_PANE_DONE' && break
    sleep 1; t=$((t + 1))
  done
  # -J rejoins rows the pane wrapped. A roster row naming a profile gang cannot
  # resolve runs past 80 columns, and a wrapped row read as two lines compares
  # against nothing.
  tmux capture-pane -peJ -t "$id"
}
ESC=$'\033'
out="$(in_pane alpha "GANG_SESSION=$GANG_SESSION env -u NO_COLOR $GANG roster")"
check "a roster on a terminal is coloured" "yes" \
  "$(case "$out" in *"${ESC}[32m"*) echo yes ;; *) echo no ;; esac)"
check "NO_COLOR turns it off on a terminal too" "no" \
  "$(case "$(in_pane alpha "GANG_SESSION=$GANG_SESSION NO_COLOR=1 $GANG roster")" in
       *"${ESC}["*) echo yes ;; *) echo no ;; esac)"

# Colour changes the bytes, never the layout: printf pads by byte count, so
# colouring a column before padding it eats nine characters of the field and the
# roster stops lining up. Rendered on a terminal, the row is the piped row
# character for character, or the padding went in the wrong order.
#
# The LAST row, not a named one. A pane shows its last 24 lines by default and
# this roster is longer than that, so the row to compare has to be one that
# cannot have scrolled off the top — which is exactly what a bigger terminal
# hides. Locally these panes inherit an attached client's height and everything
# fits; CI has no client, and the first agent was off screen.
row="$("$GANG" roster | tail -1)"
check "and it does not move a column" "$row" \
  "$(printf '%s' "$out" | sed "s/$ESC\[[0-9;]*m//g" \
     | awk -v n="${row%% *}" '$1==n{sub(/ +$/, ""); print}')"

# --- refusals ----------------------------------------------------------------

"$GANG" send alpha "no identity here" >/dev/null 2>&1
check "send without --from is refused" "1" "$?"
"$GANG" send ghost --from tester hi >/dev/null 2>&1
check "send to an unknown agent fails" "1" "$?"
"$GANG" hitch zed -p >/dev/null 2>&1
check "a flag missing its value fails cleanly" "1" "$?"
check "and hitches nothing" "" "$("$GANG" roster | awk '$1=="zed"{print $1}')"
"$GANG" --help >/dev/null
check "--help exits clean" "0" "$?"

# --- verbs -------------------------------------------------------------------

# hitch and vet carry aliases (spawn, doctor) so old hands and old scripts keep
# working; kill carries none — no dogs are killed here, and a verb that is gone
# must say so rather than half-work. (doctor's parity ran above, at the
# schema-drift gate.)
"$GANG" spawn aliased -p bash -d /tmp >/dev/null
check "spawn still answers as an alias for hitch" "idle (slack tug)" \
  "$("$GANG" status aliased)"
out="$("$GANG" kill aliased 2>&1)"; rc=$?
check "kill is no longer a verb" "1" "$rc"
check "and says so by name" "yes" \
  "$(case "$out" in *"unknown command 'kill'"*) echo yes ;; *) echo no ;; esac)"
check "and the dog it aimed at is untouched" "aliased" \
  "$("$GANG" roster | awk '$1=="aliased"{print $1}')"
"$GANG" drop aliased >/dev/null

# --- teardown ----------------------------------------------------------------

"$GANG" drop gamma >/dev/null
check "drop removes the agent" "" "$("$GANG" roster | awk '$1=="gamma"{print $1}')"
"$GANG" down >/dev/null
check "down ends the session" "no team (session '$GANG_SESSION' not running)" \
  "$("$GANG" roster)"

printf '\n%s\n' "$([ "$fails" -eq 0 ] && echo "all checks passed" || echo "$fails check(s) failed")"
[ "$fails" -eq 0 ]
