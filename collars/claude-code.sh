# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
GANG_LAUNCH="claude"
GANG_RESUME_LAUNCH="claude --resume {{session_id}}"
if [ -n "${ROOT:-}" ] && [ -x "$ROOT/bin/gang" ]; then
  case "$ROOT" in
    *[\'\"\\]*|*[[:cntrl:]]*) ;;
    *)
      _gl_cc_cmd="{\"type\":\"command\",\"command\":\"$ROOT/bin/gang\",\"args\":[\"hook\"]}"
      _gl_cc_esc="${ROOT//\$/\\\\\$}"; _gl_cc_esc="${_gl_cc_esc//\`/\\\\\`}"
      _gl_cc_json="{\"hooks\":{\"UserPromptSubmit\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"PostToolUse\":[{\"matcher\":\"*\",\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"Stop\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"PermissionRequest\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"Notification\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"PreCompact\":[{\"hooks\":[$_gl_cc_cmd]}]"
      _gl_cc_json="$_gl_cc_json,\"PostCompact\":[{\"hooks\":[$_gl_cc_cmd]}]}"
      case "${GANG_CONTEXT_LIGHTS:-off}" in
        off|'') _gl_cc_json="$_gl_cc_json}" ;;
        *) _gl_cc_json="$_gl_cc_json,\"statusLine\":{\"type\":\"command\",\"command\":\"\\\"$_gl_cc_esc/statusline/claude-code-context.sh\\\"\"}}" ;;
      esac
      GANG_LAUNCH="claude --settings '$_gl_cc_json'"
      GANG_RESUME_LAUNCH="claude --resume {{session_id}} --settings '$_gl_cc_json'"
      GANG_STOP_HOOK=1
      GANG_SELF_COMPACT=deferred
      unset _gl_cc_cmd _gl_cc_esc _gl_cc_json
      ;;
  esac
fi
GANG_MODEL_OPT="--model"
GANG_ROLE_PROMPT_OPT="--append-system-prompt"
GANG_HARNESS_PROMPT="Claude Code's task list is scoped to this harness session. Agents in other Gangline windows cannot read it, so do not cite its task IDs to them."
# AWAITING INPUT IS THE HARNESS'S OWN WORD. Observed on claude-code 2.1.224:
# the Notification hook matches on notification_type, whose complete value set
# is permission_prompt, idle_prompt, auth_success, elicitation_dialog,
# elicitation_complete, elicitation_response, agent_needs_input,
# agent_completed. These four are the ones that mean a person is being waited
# on; the other four report something that finished. A value not in this list
# is not a stall — a renamed one stops raising notes rather than raising wrong
# ones, and re-verifying this list is what a version bump costs.
#
# That cost is smaller than it reads, because this harness enumerates its own
# hook vocabulary: /hooks opens a read-only menu listing every event with a
# one-line description, and it needs no model turn. Use it before assuming an
# event does or does not exist.
GANG_STALL_TYPES="permission_prompt idle_prompt elicitation_dialog agent_needs_input"
# COMPACTION IS DECLARED, NOT INFERRED (driven on 2.1.226). A compacting
# harness queues what it is sent, and @gl_turn stays CLOSED for the whole of
# it, so gang used to read a compacting agent as idle and deliver into it. The
# native pair above says so directly: PreCompact opens the bracket, PostCompact
# closes it and drains. PreCompact's payload names its own cause in a `trigger`
# field.
#
# THREE ORIGINS RAISE THE PAIR, each watched rather than assumed: an operator
# typing /compact, gang submitting /compact to another agent, and deferred
# self-compaction where the agent runs bare `gang compact` on itself. All three
# read trigger="manual".
#
# NATIVE AUTO-COMPACTION IS A NAMED UNKNOWN — not induced, rather than not yet
# checked. What was tried, so nobody repeats it: CLAUDE_CODE_AUTO_COMPACT_WINDOW
# does not gate it ON 2.1.226 (confirmed in the process environment at 20000
# with the agent running at 165% of that, no compaction) — a statement about
# the build it was driven on and no other, which matters because anything
# pinning an older claude has not been tested and must not inherit this;
# and pre-consuming the window with a
# large attachment cannot induce it, because compaction is CONVERSATION-scoped
# — the harness says so itself, "the rest is system prompt, tool definitions,
# and attachment content" — so an agent will sit at 98% of its window
# indefinitely when the compactable part is small. Proximity to the limit is
# not the trigger; conversation size is. The only route left is a real
# conversation of roughly two hundred thousand tokens, and tool results cap at
# a few thousand each, which is why that is slow rather than merely untried.
#
# The cheap question to answer FIRST, if a real session ever auto-compacts: is
# the TURN bracket already open across it? Auto-compaction is decided when the
# harness is about to send a request, which is inside a turn — if that holds,
# the turn witness already covers this case and the unknown above is a note
# rather than a gap.

collar_session_id() { # $1 = tmux target, $2 = native hook payload
  printf '%s' "$2" | python3 -c '
import json, sys
value = json.load(sys.stdin).get("session_id", "")
if not isinstance(value, str) or not value:
    raise SystemExit(1)
print(value)
'
}
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
# The instruction slot is declared here because THIS harness's /compact takes
# instructions for its summariser — driven on 2.1.226, where a summary told to
# keep a lane name and a build number kept both and dropped what it was told to
# omit. Codex declares a bare command until the same is driven there.
GANG_COMPACT_CMD="/compact {{instructions}}"
# Verified on claude-code 2.1.226: /usage opens a full-screen tabbed modal with
# no composer, and Escape restores an empty composer. The page scrolls; gang
# returns the visible screen and does not drive the scrollbar.
GANG_USAGE_CMD="/usage"
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="Escape"
GANG_OCCUPIED_REGEX='^ +❯|Esc to'
# OBSERVE, NEVER ANSWER. Driven on claude-code 2.1.227 with external imports
# unapproved for this project. The imported path and the prompt's first lines
# vary with the working tree, so the stable suffix begins at the warning; -J
# joins its visual wraps. Empty safe/move/confirm fields make this record
# read-only in core. The dialog asks an operator security question and no key
# Gangline can send is an answer it is authorized to choose.
GANG_DIALOGS='external-import-trust|^❯ [0-9]+\. |||'
GANG_DIALOG_LINES_external_import_trust='Important: Only use Claude Code with files you trust. Accessing untrusted files may pose security risks https://code.claude.com/docs/en/security
Yes, allow external imports
No, disable external imports
Enter to confirm · Esc to cancel'
# BEFORE A SESSION EXISTS. Enumerated on claude-code 2.1.226 against a cold
# CLAUDE_CONFIG_DIR: a splash-only frame, the theme picker, the login-method
# picker, "Opening browser to sign in…", and the authorize URL above "Paste
# code here if prompted >". None of them draws a composer — collar_input needs
# a run framed by two ─ rules of equal width, and none of these is framed at
# all — so gang cannot mistake an auth field for an input box. Driven through
# the real commands, not read off the regexes: the window reads !occupied!
# (authority unknown), composer and /usage refuse by name, send spools instead
# of typing, and hitch declines to deliver the startup contract.
#
# Of the five, the two pickers put a "❯" row on screen, so hitch also names
# them as a first-run prompt to answer with gang attach. The other three match
# nothing here and fail with hitch's generic wording. That silence is the
# floor, not a defect: naming them would take a per-build fingerprint, refused
# for the reason recorded in collars/codex.sh. Note the code prompt is ASCII
# ">" (0x3E) rather than the composer's U+276F — the same pre-session alphabet
# split codex shows.
#
# Anything past the login picker needs real credentials, so whatever a first
# run draws AFTER signing in is UNKNOWN here rather than assumed absent.
# PARKED INPUT IS NOT A SUBMISSION. The queue strand (observed on 2.1.223)
# renders a queued body in the transcript styled exactly like a submitted
# prompt and empties the composer; the state is visible only in the composer
# itself, which reads "❯ Press up to edit queued messages" (nbsp after the
# glyph, stripped by collar_input like every nbsp). bin/gang matches this
# against the box reading before pasting and after Enter, and treats a hit as
# failed delivery — recoverable, because the hint also names the keystroke
# below: Up loads the parked body, Enter submits it; a plain Enter does not
# flush it.
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*$'
GANG_QUEUE_RECALL_KEY="Up"
# Escape stops an active turn; the harness paints "esc to interrupt" while one
# is in flight.
GANG_INTERRUPT_KEY="Escape"
collar_context() { # $1 = tmux target; reads the gangline statusline beacon
  local m
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || die "no ctx beacon in pane — enabled lights wire it at hitch; adopted windows must wire statusline/claude-code-context.sh themselves"
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}

collar_input() { # $1 = tmux target; prints what a HUMAN TYPED, 1 = no box,
                 # 2 = a box whose content outgrew the pane and cannot be read
  local box rc=0
  box="$(tmux capture-pane -pJ -e -t "$1" | awk '
    { # A dim run ends at the next escape, whatever closes it — 0m here, but the
      gsub(/\033\[2m[^\033]*/, "")
      gsub(/\033\[[0-9;]*[A-Za-z]/, "")   # the rest of -e: attributes, zero width
      line[NR] = $0; if (NF) last = NR
      t = $0; n = gsub(/─/, "", t)
      if (n && t == "") { prev = rule; prevw = rulew; rule = NR; rulew = n }
    }
    END {
      # A box the pane could not fit keeps the rule that opened it and loses the
      # one that closes it, so the last rule on screen has the caret under it
      # instead of the status lines. Its tail is below the fold and unreadable;
      # say which unreadable this is rather than report a drawn box as absent.
      for (i = rule + 1; rule && i <= last; i++) {
        if (line[i] ~ /^[[:space:]]*$/) continue
        if (line[i] ~ /^❯/) exit 2
        break
      }
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
    }')" || rc=$?
  [ "$rc" -eq 0 ] || return "$rc"
  printf '%s' "$box" | tr -d '\302\240'
}
