#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Focused acceptance instruments for shipped role briefs. test/integration.sh
# invokes the complete set; ROLE_ACS selects calibrated subsets for mutants.
set -euo pipefail

unset TMUX TMUX_PANE

TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-role-test.XXXXXX")"
TMUX_SOCKET="$TEST_ROOT/tmux-$(id -u)/default"
SCRIPT_ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GANG="${GANG_UNDER_TEST:-$SCRIPT_ROOT/bin/gang}"
PRODUCT_ROOT="$(cd -P "$(dirname "$GANG")/.." && pwd)"

cleanup() {
  tmux -S "$TMUX_SOCKET" kill-server 2>/dev/null || true
  rm -rf -- "$TEST_ROOT"
}
trap cleanup EXIT HUP INT TERM

mkdir -p "$TEST_ROOT/bin" "$TEST_ROOT/config" "$TEST_ROOT/collars"
cat > "$TEST_ROOT/bin/sleep" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 0
SH
chmod +x "$TEST_ROOT/bin/sleep"
export PATH="$TEST_ROOT/bin:$PATH"
export TMUX_TMPDIR="$TEST_ROOT"
export GANG_CONFIG_DIR="$TEST_ROOT/config"
export GANG_SESSION="role-tests-$$"
export GANG_TEST_COLLARS=1
export GANG_CHURN_WAIT=0
export GANG_LOCK_DIR="$TEST_ROOT/locks"
export GANG_COLLARS="$TEST_ROOT/collars"

# The focused message fixtures intentionally carry the shipped lead brief. Read
# the Bash stand-in's newest prompt from its complete history so verified
# delivery does not depend on whether that long draft still fits the grid.
cat > "$TEST_ROOT/collars/bash.sh" <<SH
# shellcheck shell=bash
. "$PRODUCT_ROOT/collars/bash.sh"
collar_input() {
  local line
  line="\$(tmux capture-pane -pJ -S - -t "\$1" |
    awk '{ i = index(\$0, "❯")
           if (i > 0 && (i == 1 || substr(\$0, 1, i - 1) ~ /[^ \\t]/)) line = \$0 }
         END { print line }')" || return 1
  case "\$line" in *❯*) ;; *) return 1 ;; esac
  printf '%s' "\${line#*❯}" | tr -d '\302\240'
}
SH

checks=0
fails=0
pass() { checks=$((checks + 1)); printf 'ok   %s\n' "$1"; }
fail() { checks=$((checks + 1)); fails=$((fails + 1)); printf 'FAIL %s\n       %s\n' "$1" "$2"; }
contains() { case "$2" in *"$3"*) pass "$1" ;; *) fail "$1" "missing [$3]" ;; esac; }
excludes() { case "$2" in *"$3"*) fail "$1" "unexpected [$3]" ;; *) pass "$1" ;; esac; }
equal() { [ "$2" = "$3" ] && pass "$1" || fail "$1" "expected [$2], got [$3]"; }

window_id() {
  local id name bare first last
  while read -r id name; do
    bare="$name"
    if [ "${#bare}" -ge 3 ]; then
      first="${bare:0:1}" last="${bare: -1}"
      case "$first" in -|'~'|'!'|'?') [ "$first" = "$last" ] && bare="${bare:1:${#bare}-2}" ;; esac
    fi
    [ "$bare" = "$1" ] && { printf '%s' "$id"; return; }
  done < <(tmux list-windows -t "=$GANG_SESSION" -F '#{window_id} #{window_name}')
  return 1
}
window_names() { tmux list-windows -t "=$GANG_SESSION" -F '#W' 2>/dev/null || true; }
pane_all() { tmux capture-pane -pJ -S - -t "$(window_id "$1")"; }
drop_agent() { "$GANG" drop "$1" >/dev/null; }
selected() { [ -z "${ROLE_ACS:-}" ] || case " $ROLE_ACS " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }
run_ac() { if selected "$1"; then "$1"; fi; }

tmux new-session -d -s role-grid-fixture -n grid "PS1='❯ ' bash --norc"
tmux set-option -g default-size 200x100
tmux resize-window -t '=role-grid-fixture:grid' -x 200 -y 100

ac1() {
  GANG_CONFIG_DIR="$TEST_ROOT/ac1-config" "$GANG" hitch role-ac1 -c bash -d /tmp --role lead >/dev/null 2>&1 || true
  contains "AC1 message-level pane contract carries the role body" "$(pane_all role-ac1)" "Your product is a team that finished the"
  drop_agent role-ac1
}

ac2() {
  local out rc=0
  out="$(GANG_CONFIG_DIR="$TEST_ROOT/ac2-config" "$GANG" hitch role-ac2 -c bash -d /tmp --role no-such-role 2>&1)" || rc=$?
  [ "$rc" -ne 0 ] && pass "AC2 unknown role refuses" || fail "AC2 unknown role refuses" "$out"
  contains "AC2 refusal names the miss" "$out" "no role 'no-such-role'"
  contains "AC2 refusal advertises usable roles" "$out" "lead"
  excludes "AC2 refusal opens no window" "$(window_names)" "role-ac2"
}

ac3() {
  local config="$TEST_ROOT/ac3-config" out rc=0
  mkdir -p "$config/roles/shape.md"
  out="$(GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac3 -c bash -d /tmp --role shape 2>&1)" || rc=$?
  [ "$rc" -ne 0 ] && pass "AC3 role directory refuses" || fail "AC3 role directory refuses" "$out"
  contains "AC3 refusal names the path" "$out" "$config/roles/shape.md"
  contains "AC3 refusal names the shape" "$out" "not a readable regular file"
  excludes "AC3 shape is not reported absent" "$out" "no role 'shape'"
  excludes "AC3 refusal opens no window" "$(window_names)" "role-ac3"
}

ac4() {
  local config="$TEST_ROOT/ac4-config" body
  mkdir -p "$config/roles"
  printf 'MARK_ROLE_ORDER\n' > "$config/roles/ordered.md"
  printf 'MARK_DOCTRINE_ORDER\n' > "$config/DOCTRINE.md"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac4 -c bash -d /tmp --role ordered >/dev/null
  body="$(pane_all role-ac4)"
  if printf '%s' "$body" | python3 -c 'import sys; s=sys.stdin.read(); p=[s.find(x) for x in ["You are role-ac4 in Gangline", "MARK_ROLE_ORDER", "Operator doctrine (", "MARK_DOCTRINE_ORDER"]]; raise SystemExit(0 if min(p)>=0 and p==sorted(p) else 1)'; then
    pass "AC4 role precedes doctrine"
  else
    fail "AC4 role precedes doctrine" "$body"
  fi
  contains "AC4 states message-level precedence" "$body" "where present, outranks it"
  drop_agent role-ac4
}

ac5() {
  local config="$TEST_ROOT/ac5-config" body
  mkdir -p "$config/roles"
  printf 'MARK_OPERATOR_LEAD\n' > "$config/roles/lead.md"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac5 -c bash -d /tmp --role lead >/dev/null
  body="$(pane_all role-ac5)"
  contains "AC5 operator role wins" "$body" "MARK_OPERATOR_LEAD"
  excludes "AC5 roles are not concatenated" "$body" "Your product is a team that finished the"
  drop_agent role-ac5
}

ac6() {
  local config="$TEST_ROOT/ac6-config" kind expected out rc name
  mkdir -p "$config/roles"
  printf 'before\000after' > "$config/roles/nul.md"
  printf '\377' > "$config/roles/invalid-utf8.md"
  awk 'BEGIN { for (i=0; i<8193; i++) printf "x" }' > "$config/roles/ceiling.md"
  printf 'before\rafter' > "$config/roles/control.md"
  for kind in nul invalid-utf8 ceiling control; do
    case "$kind" in nul) expected="contains a NUL byte" ;; invalid-utf8) expected="not valid UTF-8" ;; ceiling) expected="exceeds the 8192-byte category-error ceiling" ;; control) expected="control characters other than tab and newline" ;; esac
    name="role-ac6-$kind" rc=0
    out="$(GANG_CONFIG_DIR="$config" "$GANG" hitch "$name" -c bash -d /tmp --role "$kind" 2>&1)" || rc=$?
    [ "$rc" -ne 0 ] && pass "AC6 $kind refuses" || fail "AC6 $kind refuses" "$out"
    contains "AC6 $kind names its defect" "$out" "$expected"
    excludes "AC6 $kind opens no window" "$(window_names)" "$name"
  done
}

ac7() {
  local config="$TEST_ROOT/ac7-config" out rc=0
  mkdir -p "$config/roles"
  : > "$config/roles/empty.md"
  printf 'MARK_NONEMPTY_ROLE\n' > "$config/roles/nonempty.md"
  out="$(GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac7-empty -c bash -d /tmp --role empty 2>&1)" || rc=$?
  [ "$rc" -ne 0 ] && pass "AC7 empty role refuses" || fail "AC7 empty role refuses" "$out"
  contains "AC7 empty refusal names vacuity" "$out" "is empty"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac7-ok -c bash -d /tmp --role nonempty >/dev/null
  contains "AC7 absent doctrine remains valid" "$(pane_all role-ac7-ok)" "MARK_NONEMPTY_ROLE"
  drop_agent role-ac7-ok
}

cat > "$TEST_ROOT/argv-witness.py" <<'PY'
#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
import os, sys
prefix = sys.argv[1]
for index, value in enumerate(sys.argv[2:]):
    with open(f"{prefix}.{index}.bin", "wb") as stream:
        stream.write(value.encode("utf-8", "surrogateescape"))
os.environ["PS1"] = "❯ "
os.execvp("bash", ["bash", "--norc"])
PY
chmod +x "$TEST_ROOT/argv-witness.py"

make_argv_collar() {
  local name="$1" prefix="$2"
  cat > "$TEST_ROOT/collars/$name.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$PRODUCT_ROOT/collars/bash.sh"
GANG_LAUNCH="python3 '$TEST_ROOT/argv-witness.py' '$prefix' fresh"
GANG_RESUME_LAUNCH="python3 '$TEST_ROOT/argv-witness.py' '$prefix' resumed {{session_id}}"
GANG_ROLE_PROMPT_OPT="--append-system-prompt"
SH
}

ac8() {
  local config="$TEST_ROOT/ac8-config" receipt="$TEST_ROOT/ac8.receipt" loaded="$TEST_ROOT/ac8.loaded" seen="$TEST_ROOT/ac8.seen" event=role-ac8-received
  mkdir -p "$config/roles"
  printf 'ROLE_TAB\tVALUE\n\n' > "$config/roles/trailing.md"
  cat > "$TEST_ROOT/ac8-receiver.py" <<'PY'
#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
import pathlib, re, subprocess, sys
receipt, event = sys.argv[1:]
body = b""
while True:
    line = sys.stdin.buffer.readline()
    if not line:
        raise SystemExit(0)
    body += line
    if re.search(rb"\[/gang:hitch#[0-9a-f]+\]\n$", body):
        pathlib.Path(receipt).write_bytes(body[:-1])
        subprocess.run(["tmux", "wait-for", "-S", event], check=True)
        body = b""
PY
  chmod +x "$TEST_ROOT/ac8-receiver.py"
  cat > "$TEST_ROOT/collars/ac8.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
GANG_LAUNCH="python3 '$TEST_ROOT/ac8-receiver.py' '$receipt' '$event'"
GANG_BUSY_REGEX=""
collar_input() {
  if [ ! -e '$loaded' ]; then printf ''; return; fi
  if [ ! -e '$seen' ]; then : > '$seen'; printf draft; return; fi
  printf ''
}
SH
  # The wrapper marks that composition ended; it does not record bytes. The
  # launched receiver above is the byte witness and writes only what crossed
  # the pty and reached the process.
  cat > "$TEST_ROOT/bin/tmux" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
if [ "\${1:-}" = load-buffer ]; then
  : > '$loaded'
fi
exec /usr/bin/tmux "\$@"
SH
  chmod +x "$TEST_ROOT/bin/tmux"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac8 -c ac8 -d /tmp --role trailing >/dev/null
  tmux wait-for "$event"
  if python3 - "$receipt" "$config/roles/trailing.md" <<'PY'
import pathlib, re, sys
wire = pathlib.Path(sys.argv[1]).read_bytes()
body = pathlib.Path(sys.argv[2]).read_bytes()
match = re.search(rb"Role brief \([^\n]+\):\n\n(.*?)\n\nEnd this turn\.", wire, re.S)
raise SystemExit(0 if match and match.group(1) == body else 1)
PY
  then pass "AC8 message role body arrives byte-exact"
  else fail "AC8 message role body arrives byte-exact" "receipt mismatch"
  fi
  drop_agent role-ac8
  rm -f "$TEST_ROOT/bin/tmux"
}

ac9() {
  local config="$TEST_ROOT/ac9-config" prefix="$TEST_ROOT/ac9-argv" value pane
  mkdir -p "$config/roles"
  printf 'MARK_SYSTEM_BODY\n' > "$config/roles/system.md"
  make_argv_collar argv9 "$prefix"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac9 -c argv9 -d /tmp --role system >/dev/null
  value="$(<"$prefix.2.bin")" pane="$(pane_all role-ac9)"
  equal "AC9 argv has the role option" "--append-system-prompt" "$(<"$prefix.1.bin")"
  contains "AC9 argv has the preamble" "$value" "You are running inside a Gangline team"
  contains "AC9 argv has the body" "$value" "MARK_SYSTEM_BODY"
  contains "AC9 composer has the pointer" "$pane" "Its brief was attached to this harness"
  excludes "AC9 composer does not duplicate the body" "$pane" "MARK_SYSTEM_BODY"
  drop_agent role-ac9
}

ac10() {
  local config="$TEST_ROOT/ac10-config" prefix="$TEST_ROOT/ac10-argv" path arg found=0
  mkdir -p "$config/roles"
  printf 'MARK_BY_VALUE\n' > "$config/roles/value.md"
  path="$config/roles/value.md"
  make_argv_collar argv10 "$prefix"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac10 -c argv10 -d /tmp --role value >/dev/null
  for arg in "$prefix".*.bin; do [ "$(<"$arg")" = "$path" ] && found=1; done
  equal "AC10 no argv item is the role path" 0 "$found"
  contains "AC10 argv carries role bytes" "$(<"$prefix.2.bin")" "MARK_BY_VALUE"
  drop_agent role-ac10
}

ac11() {
  local config="$TEST_ROOT/ac11-config" prefix="$TEST_ROOT/ac11-argv" value
  mkdir -p "$config/roles"
  printf 'OPERATOR_BODY_WITHOUT_PREAMBLE\n' > "$config/roles/operator.md"
  make_argv_collar argv11 "$prefix"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac11 -c argv11 -d /tmp --role operator >/dev/null
  value="$(<"$prefix.2.bin")"
  if printf '%s' "$value" | python3 -c 'import sys; s=sys.stdin.read(); raise SystemExit(0 if s.index("Nothing below overrides it.") < s.index("OPERATOR_BODY_WITHOUT_PREAMBLE") else 1)'; then
    pass "AC11 Gangline preamble precedes operator body"
  else
    fail "AC11 Gangline preamble precedes operator body" "$value"
  fi
  drop_agent role-ac11
}

ac12() {
  local config="$TEST_ROOT/ac12-config" prefix="$TEST_ROOT/ac12-argv" expected="$TEST_ROOT/ac12-expected"
  mkdir -p "$config/roles"
  printf "quote ' here\n\044dollar\n" > "$config/roles/shell.md"
  make_argv_collar argv12 "$prefix"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac12 -c argv12 -d /tmp --role shell >/dev/null
  {
    printf '%s\n' \
      'You are running inside a Gangline team. The role brief below was attached' \
      'by Gangline when this session launched. Operator doctrine, where the' \
      'operator has any, reaches you as part of your first message; where that' \
      'doctrine and this brief disagree, the doctrine governs and this brief' \
      'yields to it. Nothing below overrides it.'
    printf '\n--- role brief: shell (%s) ---\n' "$config/roles/shell.md"
    printf "quote ' here\n\044dollar\n"
  } > "$expected"
  cmp -s "$expected" "$prefix.2.bin" \
    && pass "AC12 quoted role value reaches argv as one exact argument" \
    || fail "AC12 quoted role value reaches argv as one exact argument" "byte mismatch"
  equal "AC12 option precedes the one value" "--append-system-prompt" "$(<"$prefix.1.bin")"
  drop_agent role-ac12
}

ac13() {
  local prefix="$TEST_ROOT/ac13-argv"
  make_argv_collar argv13 "$prefix"
  GANG_CONFIG_DIR="$TEST_ROOT/ac13-config" "$GANG" hitch role-ac13 -c argv13 -d /tmp >/dev/null
  equal "AC13 no-role argv has only the base item" 1 "$(find "$TEST_ROOT" -maxdepth 1 -name 'ac13-argv.*.bin' | wc -l | tr -d ' ')"
  equal "AC13 base argv item is intact" fresh "$(<"$prefix.0.bin")"
  drop_agent role-ac13
}

ac14() {
  local root="$TEST_ROOT/ac14" variant socket session=role-state-fixture map id
  mkdir -p "$root/collars" "$root/config" "$root/bin"
  cat > "$root/bin/od" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf ' 01 02 03 04\n'
SH
  chmod +x "$root/bin/od"
  cat > "$root/collars/state.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$PRODUCT_ROOT/collars/bash.sh"
GANG_LAUNCH="python3 '$TEST_ROOT/argv-witness.py' '$root/state-argv' state"
GANG_ROLE_PROMPT_OPT="--append-system-prompt"
GANG_SESSION_KEY=1
SH
  for variant in plain role; do
    mkdir -p "$root/$variant"
    socket="$root/$variant/tmux-$(id -u)/default"
    if [ "$variant" = role ]; then
      TMUX_TMPDIR="$root/$variant" GANG_SESSION="$session" GANG_CONFIG_DIR="$root/config" \
        GANG_COLLARS="$root/collars" GANG_LOCK_DIR="$root/$variant/locks" PATH="$root/bin:$PATH" \
        "$GANG" hitch same-agent -c state -d /tmp --role lead >/dev/null
    else
      TMUX_TMPDIR="$root/$variant" GANG_SESSION="$session" GANG_CONFIG_DIR="$root/config" \
        GANG_COLLARS="$root/collars" GANG_LOCK_DIR="$root/$variant/locks" PATH="$root/bin:$PATH" \
        "$GANG" hitch same-agent -c state -d /tmp >/dev/null
    fi
    id="$(TMUX_TMPDIR="$root/$variant" tmux list-windows -t "=$session" -F '#{window_id}')"
    map="$root/$variant.map"
    TMUX_TMPDIR="$root/$variant" tmux show-options -w -t "$id" > "$map"
    TMUX_TMPDIR="$root/$variant" tmux show-options -t "$id" >> "$map"
    tmux -S "$socket" kill-server
    rm -f -- "$socket"
  done
  cmp -s "$root/plain.map" "$root/role.map" \
    && pass "AC14 role leaves window and session mappings byte-identical" \
    || fail "AC14 role leaves window and session mappings byte-identical" "$(diff -u "$root/plain.map" "$root/role.map" || true)"
}

ac15() {
  GANG_CONFIG_DIR="$TEST_ROOT/ac15-config" "$GANG" hitch lead -c bash -d /tmp >/dev/null
  excludes "AC15 agent name does not infer a role" "$(pane_all lead)" "Your product is a team that finished the"
  excludes "AC15 role-less contract has no role section" "$(pane_all lead)" "Your role in this team"
  drop_agent lead
}

ac16() {
  local config="$TEST_ROOT/ac16-config" name body role_flag doctrine_flag
  mkdir -p "$config/roles"
  printf 'SMALL_ROLE\n' > "$config/roles/small.md"
  for doctrine_flag in absent present; do
    if [ "$doctrine_flag" = present ]; then printf 'SMALL_DOCTRINE\n' > "$config/DOCTRINE.md"; else rm -f "$config/DOCTRINE.md"; fi
    for role_flag in absent present; do
      name="role-ac16-$doctrine_flag-$role_flag"
      if [ "$role_flag" = present ]; then
        GANG_CONFIG_DIR="$config" "$GANG" hitch "$name" -c bash -d /tmp --role small >/dev/null
      else
        GANG_CONFIG_DIR="$config" "$GANG" hitch "$name" -c bash -d /tmp >/dev/null
      fi
      body="$(pane_all "$name")"
      contains "AC16 delegation sentence: $doctrine_flag doctrine, $role_flag role" "$body" \
        "When you delegate a piece of work, spend your own context choosing, briefing, and judging it rather than producing it"
      drop_agent "$name"
    done
  done
}

ac17() {
  local tree="$TEST_ROOT/ac17-tree" config="$TEST_ROOT/ac17-config" out refusal advertised rc=0
  mkdir -p "$tree/bin" "$tree/collars" "$tree/roles" "$config/roles"
  cp "$PRODUCT_ROOT/bin/gang" "$tree/bin/gang"
  cp "$PRODUCT_ROOT/collars/"*.sh "$tree/collars/"
  cp "$PRODUCT_ROOT/roles/"*.md "$tree/roles/"
  printf 'SHIPPED_SHADOW\n' > "$tree/roles/shadow.md"
  printf 'OPERATOR_LEAD\n' > "$config/roles/lead.md"
  awk 'BEGIN { for (i=0; i<8193; i++) printf "x" }' > "$config/roles/shadow.md"
  printf 'INVALID_NAME\n' > "$config/roles/bad:name.md"
  out="$(GANG_CONFIG_DIR="$config" "$tree/bin/gang" roles)"
  equal "AC17 operator override appears once" 1 "$(printf '%s\n' "$out" | awk -F '\t' '$1=="lead" {n++} END {print n+0}')"
  contains "AC17 operator origin is reported" "$out" "$config/roles/lead.md"
  contains "AC17 invalid shadow winner is reported" "$out" $'shadow\t'
  contains "AC17 shadow defect is reported" "$out" "exceeds the 8192-byte category-error ceiling"
  contains "AC17 invalid basename is not hidden" "$out" "bad:name"
  contains "AC17 basename status names the law" "$out" "role name must use only"
  refusal="$(GANG_CONFIG_DIR="$config" GANG_TEST_COLLARS=1 "$tree/bin/gang" hitch role-ac17 -c bash -d /tmp --role shadow 2>&1)" || rc=$?
  [ "$rc" -ne 0 ] && pass "AC17 unusable override refuses" || fail "AC17 unusable override refuses" "$refusal"
  advertised="$(GANG_CONFIG_DIR="$config" GANG_TEST_COLLARS=1 "$tree/bin/gang" hitch role-ac17-miss -c bash -d /tmp --role absent 2>&1 || true)"
  advertised="${advertised##*usable roles: }"
  excludes "AC17 invalid winner is not advertised" "$advertised" "shadow"
  excludes "AC17 invalid basename is not advertised" "$advertised" "bad:name"
}

ac18() {
  local config="$TEST_ROOT/hostile-A"$'\t'"B"$'\n'"C"$'\033'"D" out report msg system prefix="$TEST_ROOT/ac18-argv"
  mkdir -p "$config/roles"
  printf 'INVALID_HOSTILE\n' > "$config/roles/bad"$'\t'"name"$'\n'"part"$'\033'"tail.md"
  printf 'SAFE_BODY\n' > "$config/roles/safe.md"
  out="$(GANG_CONFIG_DIR="$config" "$GANG" roles)"
  equal "AC18 listing keeps one row per discovered role" 3 "$(printf '%s\n' "$out" | wc -l | tr -d ' ')"
  equal "AC18 rows keep three fields" 3 "$(printf '%s\n' "$out" | awk -F '\t' 'NF==3 {n++} END {print n+0}')"
  excludes "AC18 listing removes hostile tab sequence" "$out" $'hostile-A\tB'
  excludes "AC18 listing removes hostile newline sequence" "$out" $'B\nC'
  excludes "AC18 listing removes hostile escape sequence" "$out" $'C\033D'
  report="$(GANG_CONFIG_DIR="$config" "$GANG" config)"
  contains "AC18 config sanitizes roles path" "$report" "hostile-A B?C?D/roles"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac18-msg -c bash -d /tmp --role safe >/dev/null
  msg="$(pane_all role-ac18-msg)"
  contains "AC18 message attribution is sanitized" "$msg" "hostile-A B?C?D/roles/safe.md"
  drop_agent role-ac18-msg
  make_argv_collar argv18 "$prefix"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac18-system -c argv18 -d /tmp --role safe >/dev/null
  system="$(<"$prefix.2.bin")"
  contains "AC18 system attribution is sanitized" "$system" "hostile-A B?C?D/roles/safe.md"
  excludes "AC18 system attribution removes escape" "$system" $'C\033D'
  drop_agent role-ac18-system
}

ac19() {
  local present="$TEST_ROOT/ac19-present" absent="$TEST_ROOT/ac19-absent"
  mkdir -p "$present/roles" "$absent"
  contains "AC19 config reports present roles slot" "$(GANG_CONFIG_DIR="$present" "$GANG" config)" $'roles\t'"$present/roles"$'\tpresent'
  contains "AC19 config reports absent roles slot" "$(GANG_CONFIG_DIR="$absent" "$GANG" config)" $'roles\t'"$absent/roles"$'\tabsent'
}

ac20() {
  local config="$TEST_ROOT/ac20-config" bad="$TEST_ROOT/ac20-bad" body out rc=0
  mkdir -p "$config"
  printf 'DOCTRINE_REGRESSION_LOCK\n' > "$config/DOCTRINE.md"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac20 -c bash -d /tmp >/dev/null
  body="$(pane_all role-ac20)"
  contains "AC20 doctrine still reaches role-less hitch" "$body" "DOCTRINE_REGRESSION_LOCK"
  excludes "AC20 role-less doctrine has no role text" "$body" "Your role in this team"
  drop_agent role-ac20
  mkdir -p "$bad"
  printf 'before\rafter' > "$bad/DOCTRINE.md"
  out="$(GANG_CONFIG_DIR="$bad" "$GANG" hitch role-ac20-bad -c bash -d /tmp 2>&1)" || rc=$?
  [ "$rc" -ne 0 ] && pass "AC20 shared reader retains doctrine control validation" \
    || fail "AC20 shared reader retains doctrine control validation" "$out"
  contains "AC20 doctrine refusal keeps its defect" "$out" "control characters other than tab and newline"
}

ac21() {
  local config="$TEST_ROOT/ac21-config" prefix="$TEST_ROOT/ac21-argv"
  mkdir -p "$config/roles"
  printf 'RESUME_ROLE_BODY\n' > "$config/roles/resume.md"
  make_argv_collar argv21 "$prefix"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac21 -c argv21 -d /tmp --resume fixture-session --role resume >/dev/null
  equal "AC21 resume launch form ran" resumed "$(<"$prefix.0.bin")"
  equal "AC21 resume identity is retained" fixture-session "$(<"$prefix.1.bin")"
  equal "AC21 resume form has role option" "--append-system-prompt" "$(<"$prefix.2.bin")"
  contains "AC21 resume form has role bytes" "$(<"$prefix.3.bin")" "RESUME_ROLE_BODY"
  drop_agent role-ac21
}

ac23() {
  local config="$TEST_ROOT/ac23-config" prefix="$TEST_ROOT/ac23-argv" hex
  mkdir -p "$config/roles"
  printf 'body\n\n' > "$config/roles/trailing.md"
  make_argv_collar argv23 "$prefix"
  GANG_CONFIG_DIR="$config" "$GANG" hitch role-ac23 -c argv23 -d /tmp --role trailing >/dev/null
  hex="$(od -An -tx1 -v "$prefix.2.bin" | tr -d ' \n')"
  case "$hex" in *0a0a) pass "AC23 role prompt preserves two trailing newlines" ;; *) fail "AC23 role prompt preserves two trailing newlines" "$hex" ;; esac
  drop_agent role-ac23
}

for ac_name in ac1 ac2 ac3 ac4 ac5 ac6 ac7 ac8 ac9 ac10 ac11 ac12 ac13 ac14 ac15 ac16 ac17 ac18 ac19 ac20 ac21 ac23; do
  run_ac "$ac_name"
done

printf '%s role-brief checks\n' "$checks"
if [ "$fails" -ne 0 ]; then
  printf '%s role-brief checks failed\n' "$fails" >&2
  exit 1
fi
