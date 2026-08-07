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
# parenthesized comma-list on the --effort option row.
#
# The producer is run under the parser's control so its exit status is honored:
# help text from a claude that failed is not evidence, however plausible it
# reads. The list is anchored to the option's own description block — cut at
# the next option row — so a parenthetical belonging to another flag can never
# be adopted, and only the comma-separated level-list form counts, so prose
# parentheses like "(experimental)" cannot be mistaken for a vocabulary.
# Exactly one such list may appear; ambiguity, wrapping the parser cannot
# finish, a failed or wedged producer (bounded by the timeout), and a
# duplicated level all produce NOTHING, which bin/gang refuses as a broken
# declaration rather than a bad value. The final || true keeps empty output as
# that channel.
GANG_EFFORT_OPT="--effort="
GANG_EFFORT_CMD="python3 -c '
import re, subprocess

try:
    result = subprocess.run(
        [\"claude\", \"--help\"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        timeout=10,
    )
except (subprocess.TimeoutExpired, OSError):
    raise SystemExit(1)
if result.returncode:
    raise SystemExit(result.returncode)

text = result.stdout
if text.count(\"--effort <level>\") != 1:
    raise SystemExit(1)
after = text.split(\"--effort <level>\", 1)[1]
block = re.split(r\"\\n[ \\t]*-\", after, maxsplit=1)[0]
lists = re.findall(
    r\"\\(([a-z0-9-]+(?:[ \\t\\n]*,[ \\t\\n]*[a-z0-9-]+)+)\\)\", block
)
if len(lists) != 1:
    raise SystemExit(1)
levels = [part.strip() for part in lists[0].split(\",\")]
if len(levels) != len(set(levels)):
    raise SystemExit(1)
print(*levels, sep=\"\\n\")
' || true"
GANG_BUSY_REGEX='^[^ ] [A-Z][a-zé]+(…|\.\.\.) *(\(|$)|Retrying in [0-9]+s|▰|▱'
GANG_QUIET_AT_REST=1
GANG_COMPACT_CMD="/compact"
GANG_OCCUPIED_REGEX='^ +❯|Esc to'
# PARKED INPUT IS NOT A SUBMISSION. The queue strand (observed on 2.1.223)
# renders a queued body in the transcript styled exactly like a submitted
# prompt and empties the composer; the state is visible only in the composer
# itself, which reads "❯ Press up to edit queued messages" (nbsp after the
# glyph, stripped by profile_input like every nbsp). bin/gang matches this
# against the box reading before pasting and after Enter, and treats a hit as
# failed delivery — recoverable, because the hint also names the keystroke
# below: Up loads the parked body, Enter submits it; a plain Enter does not
# flush it.
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*$'
GANG_QUEUE_RECALL_KEY="Up"
# Escape stops an active turn; the harness paints "esc to interrupt" while one
# is in flight.
GANG_INTERRUPT_KEY="Escape"
# The launch above composes a native Stop hook into --settings, so this harness
# announces its own turn boundaries to gang — which is what a spool needs to
# drain.
GANG_STOP_HOOK=1


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
