# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
GANG_LAUNCH="claude"
GANG_RESUME_LAUNCH="claude --continue"
if [ -n "${ROOT:-}" ] && [ -x "$ROOT/bin/gang" ]; then
  case "$ROOT" in
    *[\'\"\\]*|*[[:cntrl:]]*) ;;
    *)
      _gl_cc_cmd="{\"type\":\"command\",\"command\":\"$ROOT/bin/gang\",\"args\":[\"hook\"]}"
      _gl_cc_esc="${ROOT//\$/\\\\\$}"; _gl_cc_esc="${_gl_cc_esc//\`/\\\\\`}"
      _gl_cc_json="{\"hooks\":{\"UserPromptSubmit\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"PostToolUse\":[{\"matcher\":\"*\",\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"Stop\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"PermissionRequest\":[{\"hooks\":[$_gl_cc_cmd]}]}"
      case "${GANG_CONTEXT_LIGHTS:-off}" in
        off|'') _gl_cc_json="$_gl_cc_json}" ;;
        *) _gl_cc_json="$_gl_cc_json,\"statusLine\":{\"type\":\"command\",\"command\":\"\\\"$_gl_cc_esc/statusline/claude-code-context.sh\\\"\"}}" ;;
      esac
      GANG_LAUNCH="claude --settings '$_gl_cc_json'"
      GANG_RESUME_LAUNCH="claude --continue --settings '$_gl_cc_json'"
      unset _gl_cc_cmd _gl_cc_esc _gl_cc_json
      ;;
  esac
fi
GANG_MODEL_OPT="--model"
# REASONING EFFORT. The option includes its separator because bin/gang joins it
# to the level with no space; the joined --effort=<level> form is accepted by
# the observed harness. An unknown level is NOT an error there — claude warns,
# exits 0 and runs at its default — so hitch must refuse it, and the vocabulary
# comes from the harness's own --help, which prints the levels in a
# parenthesized list after the flag. Wrapping may split that list across lines;
# the parser reads through it. Any help shape it cannot finish produces
# NOTHING, which bin/gang refuses as a broken declaration rather than a bad
# value, and the final || true keeps empty output as the one failure channel.
GANG_EFFORT_OPT="--effort="
GANG_EFFORT_CMD="claude --help 2>/dev/null | python3 -c '
import re, sys

text = sys.stdin.read()
if text.count(\"--effort <level>\") != 1:
    raise SystemExit(1)
after = text.split(\"--effort <level>\", 1)[1]
match = re.match(r\"[^()]*\\(([^()]+)\\)\", after)
if match is None:
    raise SystemExit(1)
levels = [part.strip() for part in match.group(1).split(\",\")]
if not levels or any(
    not level or any(char.isspace() for char in level) for level in levels
):
    raise SystemExit(1)
if len(levels) != len(set(levels)):
    raise SystemExit(1)
print(*levels, sep=\"\\n\")
' || true"
GANG_BUSY_REGEX='^[^ ] [A-Z][a-zé]+(…|\.\.\.) *(\(|$)|Retrying in [0-9]+s|▰|▱'
GANG_QUIET_AT_REST=1
GANG_COMPACT_CMD="/compact"
GANG_OCCUPIED_REGEX='^ +❯|Esc to'


profile_context() { # $1 = tmux target; reads the gangline statusline beacon
  local m
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || die "no ctx beacon in pane — enabled lights wire it at hitch; adopted windows must wire statusline/claude-code-context.sh themselves"
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}

profile_input() { # $1 = tmux target; prints what a HUMAN TYPED, fails if no box
  local box
  box="$(tmux capture-pane -pJ -e -t "$1" | awk '
    { # A dim run ends at the next escape, whatever closes it — 0m here, but the
      gsub(/\033\[2m[^\033]*/, "")
      gsub(/\033\[[0-9;]*[A-Za-z]/, "")   # the rest of -e: attributes, zero width
      line[NR] = $0; if (NF) last = NR
      t = $0; n = gsub(/─/, "", t)
      if (n && t == "") { prev = rule; prevw = rulew; rule = NR; rulew = n }
    }
    END {
      if (!prev || !rule || rulew != prevw) exit 1
      if (last - rule > 5) exit 1
      seen = 0
      for (i = prev + 1; i < rule; i++) {
        s = line[i]
        if (!seen) {
          if (s ~ /^[[:space:]]*$/) continue
          if (s !~ /^❯/) exit 1                # framed, but not the composer
          sub(/^❯/, "", s); seen = 1
        }
        print s
      }
      if (!seen) print ""
    }')" || return 1
  printf '%s' "$box" | tr -d '\302\240'
}
