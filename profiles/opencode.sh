# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# opencode TUI. Every marker here was watched live against the installed TUI,
# in the state it describes, before it was written down.
# Autoupdate is turned off at launch, not in anyone's config file. Left on,
# opencode updates ITSELF — silently for patch releases (verified in the
# installed source: the updater returns early only on autoupdate=false config
# or this variable) — and a fleet pinned by GANG_VERIFIED_VERSIONS must not
# change version mid-arc. Env for this process only; the operator's own
# opencode keeps updating itself.
# No --auto here, on purpose: an agent nobody is watching cannot answer a dialog
# and gang will not answer it either, but clearing permissions is the operator's
# decision to make, not this file's. Vanilla opencode asks nothing anyway — its
# gates exist only where the operator's own opencode.json says "ask", so the
# flag matters exactly to the operators who configured one, and they are the
# ones who should choose it. Shadow via GANG_PROFILES to add it.
# The role directories are pre-authorized at launch, because gang creates the
# need for them itself. A briefed agent is POINTED at its role file by path, and
# those files live in gangline's own tree — outside the directory opencode was
# started in, which is the only place opencode reads without asking. So
# `gang hitch <name> -p opencode -r <role>` produced an agent stopped on
# "Access external directory ~/…/gangline/roles" for the very file it had just
# been told to read: a brief that was delivered and could not be acted on.
# Reproduced here on a fresh trust store, and every opencode hitch with a role
# hits it.
#
# This is the narrowest thing that clears it: two allow patterns, for the role
# directories gang will actually search and nothing else. It is not a permissions
# posture — it grants read access to the briefs gang wrote and stops there, so
# the "no --auto here" reasoning above still holds in full.
#
# OPENCODE_CONFIG_CONTENT is MERGED with the operator's own config rather than
# replacing it. Verified against the installed build with `opencode debug
# config`: a config carrying "permission": {"bash": "ask"} keeps it and gains
# external_directory alongside, and the resolved config grows by exactly that one
# key. Set for this process only, like the autoupdate switch — the operator's own
# opencode is untouched.
#
# Both patterns, because only "/*" is what opencode itself names in the dialog
# while "/**" is what its own docs use for a subtree. A directory whose path
# contains a single quote is skipped rather than escaped: the launch line is
# handed to sh, and a broken one would stop the harness from starting at all,
# which is far worse than one permission prompt.
GANG_LAUNCH="OPENCODE_DISABLE_AUTOUPDATE=1 opencode"
_gl_oc_dirs=""
for _gl_oc_d in ${GANG_ROLES:+"$GANG_ROLES"} ${ROOT:+"$ROOT/roles"}; do
  [ -d "$_gl_oc_d" ] || continue
  case "$_gl_oc_d" in *\'*) continue ;; esac
  _gl_oc_dirs="$_gl_oc_dirs\"$_gl_oc_d/*\":\"allow\",\"$_gl_oc_d/**\":\"allow\","
done
[ -z "$_gl_oc_dirs" ] || GANG_LAUNCH="OPENCODE_DISABLE_AUTOUPDATE=1 OPENCODE_CONFIG_CONTENT='{\"permission\":{\"external_directory\":{${_gl_oc_dirs%,}}}}' opencode"
unset _gl_oc_dirs _gl_oc_d
# From `opencode --help`: -m/--model, spelled provider/model
# ("google/gemini-3.1-pro-preview").
# NO GANG_RESUME_LAUNCH, deliberately (ADR-0007). opencode has --continue, but its
# sessions are not scoped to a directory: `opencode session list` returns a
# byte-identical list from two different directories, measured here. So --continue
# resumes the most recent session anywhere on the machine, and two opencode agents
# rebuilt after a server death would both land in the same conversation — neither
# of them the one asked for. `gang hitch --resume` refuses this profile by name,
# which is the honest answer; a declaration nobody could scope is the unsafe one.
GANG_MODEL_OPT="-m"
# A running turn paints "⬝⬝⬝⬝⬝⬝⬝⬝  esc interrupt" into the hint row — the
# dotted spinner fills and drains, the interrupt hint holds still. Compaction
# paints this same line and nothing else (watched: /compact ran as an ordinary
# busy turn), so a compacting agent reads busy, which is what patrol needs.
# Absent while idle and while the command palette is up.
GANG_BUSY_REGEX="esc interrupt"
# The working half is NOT measured here — only on claude-code. It costs nothing
# either way: if this harness writes nothing while it works, the arm never fires
# and churn answers as it always did. It is declared because the rest half, which
# is the one with a dangerous direction, IS measured.
# Measured quiet at rest: 30 samples at 1s with the composer empty and the agent
# finished, #{window_activity} frozen on one value, 0 ticks. That is what this
# declares. A harness that repaints at rest can fabricate this signal for as long
# as it keeps writing, so gang bounds activity-only busy globally; the declaration
# still stays unset until somebody has watched a finished agent sit still, because
# a wrong declaration costs that entire bound on every false episode.
GANG_QUIET_AT_REST=1
# From the live slash menu: "/compact  Compact session".
GANG_COMPACT_CMD="/compact"
# Watched end to end: text typed during a turn lands in the composer, Enter
# moves it into the transcript flagged "QUEUED", and it is submitted and
# answered the moment the running turn finishes.
GANG_MIDTURN_INPUT=1
# Two markers, because opencode frames its two kinds of modal differently.
#
# First alternate — the dialog header. Every overlay opencode floats over the
# composer titles itself on one row and right-aligns a lone "esc" on that same
# row, and nothing else on screen ends that way. Watched live, composer keyboard
# gone in every one (the cursor moves into the dialog's own search field, which
# is what profile_input below keys on): the command palette ("Commands … esc"),
# the model picker ("Select model … esc"), the session switcher ("Sessions …
# esc") and the theme picker ("Themes … esc"). The busy hint row ends in
# "interrupt", never "esc", so a turn in flight cannot match it.
#
# Second alternate — the permission dialog, which carries no such header. It is
# config-enabled: vanilla opencode allows every tool until a permission block in
# opencode.json says "ask". Watched live through a full ask→answer→erase cycle:
# the header glyph and words hold still while the tool detail varies, and the
# busy hint row is replaced by the dialog's own key hints. Erased the moment it
# is answered — a match means the dialog owns the screen right now. A literal
# multibyte STRING is safe in cron's C locale (it matches as a byte sequence);
# only bracket classes break there.
#
# Reachability caveat, and it is gang's to fix rather than this file's: opencode
# anchors a dialog around the composer and grows it UPWARDS, so on a full-screen
# picker the header row sits further from the bottom of the pane than the region
# gang scans for state (GANG_STATUS_ROWS, 20 rows). Measured at 70x34: header on
# row 11, scan starts at row 14. The header is still the right marker — the scan
# has to be able to see it.
GANG_OCCUPIED_REGEX='△ Permission required| {2,}esc *$'
# GANG_COMPACTING_REGEX is deliberately unset — the observation found nothing
# to scrape: compaction is drawn as an ordinary working turn with no marker of
# its own, so a resume waits for the pane to go quiet instead of queueing.
# Every scraped marker in this file was live-verified against these harness
# versions. New release = re-verify + append (gang vet watches the pin).
GANG_VERSION_CMD="opencode --version"
GANG_VERIFIED_VERSIONS="1.14.39 1.18.7 1.18.8"

opencode_models_json() { printf '%s/opencode/models.json' "${XDG_CACHE_HOME:-$HOME/.cache}"; }

profile_context() { # $1 = tmux target; hint row carries used+percent, catalog carries the window
  # The hint row paints "9.4K (1%)" — plain "517" under a thousand, watched
  # right after a live /compact — but never the window. The window lives where
  # opencode itself reads it: the models.dev catalog cache, keyed by
  # provider+model. The pane keys the join: the composer badge paints
  # "<agent> · <model name> <provider name>", and the catalog supplies the
  # candidate name pairs for an exact match — per-pane correct beside agents
  # on other models, and it follows a mid-session model switch. The painted
  # percent then cross-checks the join: round(used/window*100) must reproduce
  # it (the same formula the TUI runs, verified in the installed source),
  # within the rounding the compact notation cost the used figure. A miss is
  # a wrong window, not tolerable drift — refuse loudly.
  local cap row badge names
  cap="$(tmux capture-pane -pJ -t "$1")" || die "cannot read pane $1"
  row="$(printf '%s\n' "$cap" | grep -Eo '[0-9]+(\.[0-9]+)?[KM]? \([0-9]+%\)' | tail -1)" \
    || die "no context readout in the hint row — opencode paints it after the agent's first turn, and a narrow pane clips it mid-token (a clipped readout loses its closing paren, so it can never half-match)"
  badge="$(printf '%s\n' "$cap" | awk '/^[[:space:]]*╹▀/ { b = p } { p = $0 } END { print b }')"
  # Wide panes float a right-hand info column onto the badge row itself; the
  # column gap is a run of spaces, and model/provider names only ever use
  # single ones — cut at the first double space.
  case "$badge" in
    *' · '*) names="${badge##*' · '}"; names="${names%%  *}"
             names="${names%"${names##*[![:space:]]}"}" ;;
    *) die "no agent·model badge above the composer border — opencode may still be booting" ;;
  esac
  python3 - "$(opencode_models_json)" "$names" "${row%% *}" "${row##*\(}" <<'PY' || die "cannot bind pane $1 to a context window"
import json, math, sys
path, names, used_s, pct_s = sys.argv[1:5]
try:
    cat = json.load(open(path, encoding="utf-8"))
except (OSError, ValueError) as e:
    print(f"models catalog unreadable at {path} ({e}) — opencode writes it at startup",
          file=sys.stderr)
    sys.exit(1)
unit = {"K": 1000, "M": 1000000}.get(used_s[-1], 1)
num = used_s.rstrip("KM")
used = round(float(num) * unit)
step = unit / (10 ** (len(num.split(".")[1]) if "." in num else 0))
pct = int(pct_s.rstrip("%)"))
wins = set()
for pid, p in cat.items():
    if not isinstance(p, dict):
        continue
    pname = p.get("name") or pid
    for mid, m in (p.get("models") or {}).items():
        if f"{m.get('name') or mid} {pname}" == names:
            ctx = (m.get("limit") or {}).get("context")
            if isinstance(ctx, int) and ctx > 0:
                wins.add(ctx)
if len(wins) != 1:
    n = len(wins)
    print(f"badge '{names}' matches {n} catalog windows — "
          + ("catalog or badge drifted; re-verify against the installed opencode"
             if n == 0 else "ambiguous join; refusing to guess"),
          file=sys.stderr)
    sys.exit(1)
win = wins.pop()
tol = math.ceil(step / 2 / win * 100) + 1
if abs(round(used / win * 100) - pct) > tol:
    print(f"painted {pct}% does not reproduce from {used}/{win} — wrong window joined "
          "(model switched under the badge, or catalog drift); re-verify", file=sys.stderr)
    sys.exit(1)
print(f"{round(used / 1000)}k/{round(win / 1000)}k ({pct}%)")
PY
}

profile_input() { # $1 = tmux target; prints the composer, fails if it has no keyboard
  # The composer is the ┃-bordered block sitting directly on the ╹▀▀▀ bottom
  # border: its last row is the agent·model badge, the rows above are content.
  # Painted is not enough here. User messages in the transcript draw the same
  # ┃ edge, completion dropdowns box their rows with ┃ on BOTH sides, and a
  # modal leaves the composer fully painted while stealing its keyboard — a
  # paste would land in the modal's search field. The hardware cursor settles
  # it: opencode parks it inside the composer content rows exactly when the
  # composer owns the keyboard (watched: palette open → cursor in the palette
  # search field; closed → cursor right after "┃  "). Cursor outside the
  # content rows = nothing to paste into, whatever is painted. Dialog shapes
  # are deliberately not enumerated — any focus thief moves the cursor.
  local cap cur
  cap="$(tmux capture-pane -pJ -t "$1")" || return 1
  cur="$(tmux display-message -p -t "$1" '#{cursor_y}')" || return 1
  printf '%s\n' "$cap" | awk -v cur="$cur" '
    { line[NR] = $0 }
    /^[[:space:]]*╹▀/ { border = NR }
    END {
      if (!border) exit 1
      top = border - 1
      while (top > 1 && line[top - 1] ~ /^[[:space:]]*┃/) top--
      # line[border-1] is the badge row; content is top .. border-2
      if (top > border - 2) exit 1
      if (cur + 1 < top || cur + 1 > border - 2) exit 1
      for (i = top; i <= border - 2; i++) {
        s = line[i]
        if (s ~ /┃[[:space:]]*$/) continue   # dropdown row, boxed both sides
        sub(/^[[:space:]]*┃ ?/, "", s)
        print s
      }
    }'
}

profile_vet() { # format gate: the catalog side of the join, against the live cache
  # The pane side — hint row, badge — cannot be gated without an agent on
  # screen; what rots silently on disk is the catalog. Names or limit shapes
  # shifting under an opencode release would kill every join at once, so parse
  # the same file profile_context joins against. No cache = opencode has never
  # run here (CI) — nothing to gate.
  local f
  f="$(opencode_models_json)"
  [ -f "$f" ] || { echo "no models catalog at $f — nothing to gate"; return 0; }
  python3 - "$f" <<'PY'
import json, sys
try:
    cat = json.load(open(sys.argv[1], encoding="utf-8"))
except (OSError, ValueError) as e:
    print(f"models catalog unreadable ({e}) — re-verify against the installed opencode",
          file=sys.stderr)
    sys.exit(1)
n = sum(1 for p in cat.values() if isinstance(p, dict) and p.get("name")
        for m in (p.get("models") or {}).values()
        if m.get("name") and isinstance((m.get("limit") or {}).get("context"), int))
if not n:
    print("models catalog holds no named model with limit.context — the badge join is dead; "
          "re-verify against the installed opencode", file=sys.stderr)
    sys.exit(1)
print(f"OK (catalog join candidates: {n})")
PY
}
