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
      _gl_cc_json="$_gl_cc_json,\"PreCompact\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"PostCompact\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"PermissionRequest\":[{\"hooks\":[$_gl_cc_cmd]}]}"
      _gl_cc_json="$_gl_cc_json,\"statusLine\":{\"type\":\"command\",\"command\":\"\\\"$_gl_cc_esc/statusline/claude-code-context.sh\\\"\"}}"
      GANG_LAUNCH="claude --settings '$_gl_cc_json'"
      GANG_RESUME_LAUNCH="claude --continue --settings '$_gl_cc_json'"
      unset _gl_cc_cmd _gl_cc_esc _gl_cc_json
      ;;
  esac
fi
GANG_MODEL_OPT="--model"
GANG_BUSY_REGEX='^[^ ] [A-Z][a-zé]+(…|\.\.\.) *(\(|$)|Retrying in [0-9]+s|▰|▱'
GANG_QUIET_AT_REST=1
GANG_COMPACT_CMD="/compact"
GANG_MIDTURN_INPUT=1
GANG_MIDTURN_ACTS=1
GANG_OCCUPIED_REGEX='^ +❯|Esc to'


profile_context() { # $1 = tmux target; reads the gangline statusline beacon
  local m
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || die "no ctx beacon in pane — hitched windows wire it at launch; adopted windows must wire statusline/claude-code-context.sh themselves"
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
