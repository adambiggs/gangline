# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
# CONTEXT-LIGHT DEFAULTS ARE A PER-MODEL ANSWER, because this harness runs
# models whose native windows differ by five times. The same fraction does not
# mean the same thing in each: 80% of a 1000k window leaves 200k of runway for
# an agent to finish an arc and compact, while 80% of a 200k window leaves 40k
# and the red light arrives too late to act on. Smaller windows therefore get
# an earlier pair, so the absolute headroom behind red stays comparable.
#
# Fractions rather than token counts, because the window a model reports is the
# provider's to change and `collar_context` already reads the live one; a
# fraction stays correct across that change where a token pair silently stops
# fitting. Observed 2026-08-24 on this installation: claude-opus-5 reports a
# 1000k window. Haiku is the 200k class. A model matching neither is given the
# earlier pair, since firing early costs a checkpoint and firing late costs the
# arc.
#
# Omitting -m leaves the model, and so the window class, unknown; hitch already
# warns about that, and guessing a light here would be the second guess.
collar_context_lights() { # $1 model; 0 with thresholds, 1 = no default for it
  case "$1" in
    '') return 1 ;;
    *haiku*) printf '45%%,65%%\n' ;;
    *) printf '55%%,80%%\n' ;;
  esac
}
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
      # THE BEACON IS WIRED ONLY WHERE A LIGHT WILL READ IT. `statusLine`
      # replaces whatever status line the operator configured for themselves,
      # so it is not painted over a hitch that asked for no lights. `collar`
      # asks this file's own default the same question bin/gang asks it, so
      # the wiring and the armed thresholds cannot disagree about this model.
      case "${GANG_CONTEXT_LIGHTS:-off}" in
        off|'') _gl_cc_light=0 ;;
        collar)
          # AN `&&` TAIL HERE WOULD END GANG. bin/gang runs under set -e, and a
          # collar with no default for this model returns 1, so the whole list
          # would fail the hitch instead of leaving the beacon unwired.
          if collar_context_lights "${GANG_MODEL:-}" >/dev/null; then
            _gl_cc_light=1
          else
            _gl_cc_light=0
          fi ;;
        *) _gl_cc_light=1 ;;
      esac
      case "$_gl_cc_light" in
        0) _gl_cc_json="$_gl_cc_json}" ;;
        *) _gl_cc_json="$_gl_cc_json,\"statusLine\":{\"type\":\"command\",\"command\":\"\\\"$_gl_cc_esc/statusline/claude-code-context.sh\\\"\"}}" ;;
      esac
      GANG_LAUNCH="claude --settings '$_gl_cc_json'"
      GANG_RESUME_LAUNCH="claude --resume {{session_id}} --settings '$_gl_cc_json'"
      GANG_STOP_HOOK=1
      GANG_SELF_COMPACT=deferred
      unset _gl_cc_cmd _gl_cc_esc _gl_cc_json _gl_cc_light
      ;;
  esac
fi
GANG_MODEL_OPT="--model"
# Claude exposes no complete model catalog. These are the aliases its own
# --help documents; a full name can still be recognized by the native checker
# below, but it cannot be discovered here and recognition does not prove the
# current account may use it.
GANG_MODEL_ALIASES=$'fable\nopus\nsonnet'
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
  local value transcript
  value="$(printf '%s' "$2" | python3 -c '
import json, sys
value = json.load(sys.stdin).get("session_id", "")
if not isinstance(value, str) or not value:
    raise SystemExit(1)
print(value)
')" || return 1
  transcript="$(printf '%s' "$2" | python3 -c '
import json, sys
value = json.load(sys.stdin).get("transcript_path")
if value is None:
    raise SystemExit(0)
if not isinstance(value, str):
    raise SystemExit(1)
print(value, end="")
')" || return 1
  if [ -n "$transcript" ]; then
    tmux set-option -w -t "$1" @gl_session "$transcript" || return 1
  fi
  printf '%s\n' "$value"
}

claude_session_file() { # $1 = tmux target -> hook-bound transcript path
  local file
  file="$(tmux show-options -wqv -t "$1" @gl_session)" || file=""
  [ -n "$file" ] && [ -f "$file" ] || return 1
  printf '%s' "$file"
}

# ERROR EVIDENCE IS READ FROM THE TAIL ONCE. Both consumers ask about the newest
# relevant complete record, so bytes before that record cannot change either
# answer. Sharing the backward walk keeps ordinary idle notifications from
# reparsing a long-lived transcript from byte zero. A completed record that can
# still outrank the answer remains loud; a final line without its newline is an
# append in flight, not a record yet.
#
# FATAL TURN EVIDENCE IS THE NEWEST TOP-LEVEL SEMANTIC RECORD, not pane paint.
# Observed on claude-code 2.1.233: an unrecognized launch model writes a
# synthetic assistant record with isApiErrorMessage=true, error=model_not_found
# and the message checked below. On 2.1.241, an exhausted 529 retry sequence
# instead leaves a synthetic assistant record with error=server_error and
# apiErrorStatus=529, and a response stream that dies mid-turn leaves one with
# error=server_error and NO apiErrorStatus key at all — the harness prints it,
# returns to an idle composer, and nothing restarts the lane. Three specimens of
# that last class differ in their sentence and agree in their structure, and each
# is followed only by a system/turn_duration record: the turn is over.
# A following real user turn outranks any terminal record
# while recovery is running; isMeta local-command notices and tool_result-only
# user records are not turns. A later ordinary assistant record clears it. Other
# retryable API errors abstain. Missing pre-session evidence abstains; a bound
# transcript Gangline cannot interpret returns unknown instead of absence.
claude_record_read() { # $1 transcript, $2 fatal|auto|action
  python3 - "$1" "$2" <<'PY'
import datetime
import json
import re
import sys

def newest_complete_records(path):
    # Claude appends JSONL. A final line without its newline is still in flight,
    # not a malformed record; discard only that suffix. Walk backward so a
    # long-lived transcript does not make every state read scan from byte zero.
    with open(path, "rb") as transcript:
        transcript.seek(0, 2)
        end = transcript.tell()
        if end:
            transcript.seek(end - 1)
        if end and transcript.read(1) != b"\n":
            # Find the newline before the in-flight suffix without loading that
            # suffix, which itself may be much larger than one read chunk.
            probe = end
            end = 0
            while probe:
                size = min(probe, 64 * 1024)
                probe -= size
                transcript.seek(probe)
                chunk = transcript.read(size)
                split = chunk.rfind(b"\n")
                if split >= 0:
                    end = probe + split + 1
                    break
        position = end
        carry = b""
        while position:
            size = min(position, 64 * 1024)
            position -= size
            transcript.seek(position)
            data = transcript.read(size) + carry
            lines = data.split(b"\n")
            carry = lines.pop(0) if position else b""
            for raw in reversed(lines):
                if raw.strip():
                    yield raw.decode("utf-8")


def semantic(record):
    if record.get("isSidechain") is True or record.get("isMeta") is True:
        return False
    kind = record.get("type")
    if kind == "assistant":
        return True
    if kind != "user":
        return False
    message = record.get("message")
    content = message.get("content") if isinstance(message, dict) else None
    if isinstance(content, str):
        return True
    if not isinstance(content, list) or not content:
        raise ValueError("user record has no readable content")
    kinds = []
    for item in content:
        kind = item.get("type") if isinstance(item, dict) else None
        if not isinstance(kind, str) or not kind:
            raise ValueError("user content item has no readable type")
        kinds.append(kind)
    if all(kind == "tool_result" for kind in kinds):
        return False
    if any(kind == "text" for kind in kinds):
        return True
    raise ValueError("user record has an unrecognized content shape")


# HOW FAR BACK AN ANSWER IS WORTH WALKING. A transcript whose newest tool call
# is older than this is one that has taken no action for the whole of it, and
# that is the reading being asked for; naming the oldest record actually read
# answers it as a bound rather than as a scan of every byte ever written.
ACTION_BOUND = 2000


def epoch(record):
    stamp = record.get("timestamp")
    if not isinstance(stamp, str) or not stamp:
        raise ValueError("record carries no readable timestamp")
    text = stamp[:-1] + "+00:00" if stamp.endswith("Z") else stamp
    return int(datetime.datetime.fromisoformat(text).timestamp())


def tool_call(record):
    # A tool call is an assistant content block whose type is one of the API's
    # tool_use family (tool_use, server_tool_use, mcp_tool_use). Reasoning and
    # text blocks are the agent talking, which is exactly what a wedged agent
    # keeps doing.
    if record.get("type") != "assistant" or record.get("isSidechain") is True:
        return False
    message = record.get("message")
    content = message.get("content") if isinstance(message, dict) else None
    if not isinstance(content, list):
        return False
    for item in content:
        kind = item.get("type") if isinstance(item, dict) else None
        if isinstance(kind, str) and kind.endswith("tool_use"):
            return True
    return False


mode = sys.argv[2]
latest = None
scanned = 0
oldest = None
try:
    for raw in newest_complete_records(sys.argv[1]):
        record = json.loads(raw)
        if not isinstance(record, dict):
            raise ValueError("record is not an object")
        if mode == "action":
            scanned += 1
            if tool_call(record):
                print(f"at {epoch(record)}")
                raise SystemExit(0)
            oldest = record
            if scanned >= ACTION_BOUND:
                break
            continue
        if mode == "fatal":
            relevant = semantic(record)
        else:
            relevant = (
                record.get("type") == "assistant"
                and record.get("isSidechain") is not True
            )
        if relevant:
            latest = record
            break
except (OSError, UnicodeError, ValueError, json.JSONDecodeError, OverflowError):
    if mode == "fatal":
        print("bound Claude transcript is unreadable")
    if mode == "action":
        print("bound Claude transcript is unreadable")
    raise SystemExit(2)

if mode == "action":
    # THE BOUND WAS REACHED is a different answer from THERE IS NO TOOL CALL,
    # and only the second one is a claim about the whole session.
    if scanned >= ACTION_BOUND and oldest is not None:
        try:
            print(f"before {epoch(oldest)}")
        except ValueError:
            print("bound Claude transcript is unreadable")
            raise SystemExit(2)
        raise SystemExit(0)
    raise SystemExit(1)

if latest is None:
    raise SystemExit(1)

if mode == "auto":
    error = latest.get("error")
    record_id = latest.get("uuid")
    if latest.get("isApiErrorMessage") is not True or not isinstance(error, str) or not error:
        raise SystemExit(1)
    # A selected-model failure cannot be repaired by replaying the same turn.
    # The live fatal reader reports it; auto-resume must not spend its one
    # continuation or repaint the window idle first.
    if error == "model_not_found":
        raise SystemExit(1)
    if not isinstance(record_id, str) or not record_id:
        raise SystemExit(2)
    print(record_id)
    raise SystemExit(0)

if latest.get("type") != "assistant":
    raise SystemExit(1)
if latest.get("isApiErrorMessage") is not True:
    raise SystemExit(1)

if latest.get("error") == "server_error":
    # A STREAM THAT DIED CARRIES NO HTTP STATUS. The key is absent rather than
    # null, which is what separates this class from every status-bearing one,
    # and it is the structure rather than the sentence: the same record has been
    # seen saying the response stopped arriving, that the server errored
    # mid-response, and that the connection was lost.
    if "apiErrorStatus" not in latest:
        print("Claude Code ended the latest turn on a broken response stream (server_error)")
        raise SystemExit(0)
    status = latest.get("apiErrorStatus")
    if status == 529 and not isinstance(status, bool):
        print("Claude Code ended the latest turn on HTTP 529 (server_error)")
        raise SystemExit(0)
    raise SystemExit(1)

if latest.get("error") != "model_not_found":
    raise SystemExit(1)

message = latest.get("message")
content = message.get("content") if isinstance(message, dict) else None
if not isinstance(content, list):
    print("model_not_found record has no readable message content")
    raise SystemExit(2)
texts = [
    item.get("text") for item in content
    if isinstance(item, dict) and item.get("type") == "text"
    and isinstance(item.get("text"), str)
]
pattern = re.compile(
    r"^There's an issue with the selected model \(([^()\n]+)\)\. "
    r"It may not exist or you may not have access to it\. "
    r"Run /model to pick a different model\.$"
)
matches = [pattern.fullmatch(text) for text in texts]
matches = [match for match in matches if match is not None]
if len(matches) != 1:
    print("model_not_found record has an unrecognized message shape")
    raise SystemExit(2)
print(f"selected model '{matches[0].group(1)}' was rejected (model_not_found)")
PY
}

collar_bricked() { # $1 target; print cause, 0 fatal, 1 absent, 2 unknown
  local file
  file="$(claude_session_file "$1")" || return 1
  claude_record_read "$file" fatal
}

# EVIDENCE OF ACTION, WHICH IS NOT EVIDENCE OF HEALTH. This reports when the
# harness last recorded a tool call and nothing more; whether that age means an
# agent is stuck is the reader's to judge, and gang prints the age rather than a
# verdict on it.
collar_last_action() { # $1 target -> "at <epoch>" | "before <epoch>";
                       # 0 printed, 1 = no tool call in the source, 2 = unknown
  local file
  file="$(claude_session_file "$1")" || {
    printf 'no Claude transcript is bound to this window yet'
    return 2
  }
  claude_record_read "$file" action
}

collar_auto_resume_record() { # $1 target, $2 Notification kind -> failed-turn UUID
  local file
  [ "$2" = idle_prompt ] || return 1
  file="$(claude_session_file "$1")" || return 2
  claude_record_read "$file" auto
}

collar_auto_resume_prompt() { # $1 target unused, $2 UserPromptSubmit payload
  printf '%s' "$2" | python3 -c '
import json, sys
value = json.load(sys.stdin).get("prompt")
if not isinstance(value, str) or not value:
    raise SystemExit(2)
print(value, end="")
'
}
# MODEL RECOGNITION WITHOUT AN API CALL. Observed on claude-code 2.1.233:
# `claude --model ID -p ""` aborts because the print prompt is empty. Before
# that common abort, an unrecognized id emits the native warning below. Both
# paths exit nonzero in this build, so output is the evidence. A changed abort
# shape is unknown rather than recognition inferred from silence.
collar_model_check() { # $1 model; 0 recognized, 1 unrecognized, 2 unknown
  python3 - "$1" <<'PY'
import subprocess
import sys

try:
    result = subprocess.run(
        ["claude", "--model", sys.argv[1], "-p", ""],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        timeout=10,
    )
except (subprocess.TimeoutExpired, OSError, UnicodeError):
    print("native recognition check could not run")
    raise SystemExit(2)

output = result.stdout
if (
    "is not a model this version of Claude Code recognizes" in output
    or "[claude-code:unrecognized_model]" in output
):
    print("native validator rejected it as unrecognized")
    raise SystemExit(1)
if "Input must be provided either through stdin or as a prompt argument" in output:
    raise SystemExit(0)
print("native recognition check returned an unreadable result")
raise SystemExit(2)
PY
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
# A live 529 retry paints its native error before the transcript gains the
# terminal synthetic record. Once that record exists collar_bricked outranks
# this paint, so the same retained line cannot leave an exhausted retry busy.
GANG_BUSY_REGEX='^[^ ] [A-Z][a-zé]+(…|\.\.\.) *(\(|$)|Retrying in [0-9]+s|API Error: 529 Overloaded\.|▰|▱'
GANG_QUIET_AT_REST=1
# The instruction slot is declared here because THIS harness's /compact takes
# instructions for its summariser — driven on 2.1.226, where a summary told to
# keep a lane name and a build number kept both and dropped what it was told to
# omit. Codex declares a bare command until the same is driven there.
GANG_COMPACT_CMD="/compact {{instructions}}"
# A headless Claude startup is materially heavier than reading a native event
# file. Usage lights may reuse their last hook sample for this many seconds;
# explicit `gang limits` and `gang wait-limit` reads remain fresh.
GANG_USAGE_LIGHT_INTERVAL=60

# Provider-limit decisions use a headless native read, never the interactive
# /usage modal. Verified on claude-code 2.1.232: `claude -p /usage` returns plain
# text even while another Claude session is mid-turn. Each `Current ...` row
# carries percent used, a reset wall clock, and its IANA timezone.
collar_usage_limits() { # $1 = unused target; print label<TAB>used<TAB>reset<TAB>observed
  local output
  # This reader runs inside native hooks when lights or auto-resume are enabled.
  # Leave margin inside Claude's native hook bound, and use only timeout's
  # portable SECONDS COMMAND form: BusyBox implements it but not --foreground.
  # Dependency failures have their own status and operator-facing explanation.
  command -v timeout >/dev/null 2>&1 || return 64
  timeout 1 true >/dev/null 2>&1 || return 65
  output="$(timeout 50 claude -p '/usage')" || return 1
  printf '%s\n' "$output" | python3 -c '
from datetime import datetime, timedelta
import re
import sys
from zoneinfo import ZoneInfo

lines = sys.stdin.read().splitlines()
now = datetime.now().astimezone()
rows = []
pattern = re.compile(
    r"^(Current [^:]+): ([0-9]+)% used . resets "
    r"([A-Z][a-z]{2} [0-9]{1,2}, [0-9]{1,2}(?::[0-9]{2})?(?:am|pm)) "
    r"\(([^()]+)\)$"
)
for line in lines:
    match = pattern.match(line)
    if not match:
        continue
    label, used, clock, zone_name = match.groups()
    try:
        zone = ZoneInfo(zone_name)
        year = now.astimezone(zone).year
        clock_format = "%b %d, %I:%M%p %Y" if ":" in clock else "%b %d, %I%p %Y"
        reset = datetime.strptime(f"{clock} {year}", clock_format).replace(tzinfo=zone)
    except (ValueError, KeyError):
        raise SystemExit(1)
    if reset < now.astimezone(zone) - timedelta(days=1):
        reset = reset.replace(year=reset.year + 1)
    rows.append((label, int(used), int(reset.timestamp())))
if not rows:
    raise SystemExit(1)
observed = int(now.timestamp())
for label, used, reset in rows:
    print(label, used, reset, observed, sep="\t")
'
}

collar_usage_limits_error() { # $1 = collar_usage_limits exit status
  case "$1" in
    64) printf "collar 'claude-code' requires the 'timeout' command to bound its native provider-limit reader, but timeout is not installed" ;;
    65) printf "collar 'claude-code' found a 'timeout' command that cannot run the portable 'timeout seconds command' form required to bound its native provider-limit reader" ;;
    *) return 1 ;;
  esac
}
GANG_OCCUPIED_REGEX='^ +❯|Esc to'
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
# MID-TURN INPUT IS NATIVE STEERING, but attribution lands before its first
# keystroke. Driven on 2.1.232: with a turn live and the composer free, Claude
# accepts Enter as queued steering and consumes it at its next tool batch. The
# `steer` declaration makes Gangline commit the envelope to its attributed
# spool first, then claim that entry under the same pane lock before typing.
# A draft, tmux mode, or unreadable composer leaves it live for a later native
# opportunity. This is deliberately not `=1`, whose direct path types before
# the attributed spool owns anything.
GANG_MIDTURN_INPUT=steer
# Escape stops an active turn; the harness paints "esc to interrupt" while one
# is in flight.
GANG_INTERRUPT_KEY="Escape"
collar_context() { # $1 = tmux target; reads the gangline statusline beacon
  local pane m
  # A PANE THAT COULD NOT BE READ IS NOT A PANE WITH NO READOUT. The capture is
  # taken into a variable before grep sees it, because grep's verdict on empty
  # input reads exactly like its verdict on a pane carrying no readout, and the
  # refusal that reaches the operator then names the wrong fault.
  pane="$(tmux capture-pane -pJ -t "$1")" \
    || refuse "cannot read pane $1 — whether the statusline beacon is on that screen is unknown, not absent"
  m="$(printf '%s\n' "$pane" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || m=""
  # EMPTY IS THE MISS, and the miss is checked rather than inherited: without
  # pipefail a grep that found nothing hands its status to tail, the assignment
  # succeeds, and a pane with no readout prints as one with an empty reading.
  if [ -z "$m" ]; then
    # STATUS 2 IS A READABLE FRAME WITHOUT THE NATIVE READOUT. Claude can hide
    # or scroll its hitch-wired beacon during a redraw, tool output or overlay,
    # so context lights give this transient screen miss its own edge before a
    # consecutive miss becomes unavailable. `gang context` still fails loud on
    # the first miss and keeps the wiring repair for adopted windows visible.
    printf '%s\n' \
      'gang: no ctx beacon in pane — enabled lights wire it at hitch; adopted windows must wire statusline/claude-code-context.sh themselves' >&2
    return 2
  fi
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}

# A SELECTED IN-PROCESS SUBAGENT IS NOT THIS WINDOW'S AGENT INPUT. Observed on
# 2.1.229: its fullscreen view draws U+276F between a task-named opening band
# and a pure closing rule. Typing there resumes the child, which answers in its
# own transcript; it does not reach the hitched parent. The pure opening-rule
# requirement below intentionally leaves that view unreadable, so delivery can
# never route a parent-addressed envelope into a child context.
collar_input() { # $1 = tmux target; prints what a HUMAN TYPED, 1 = no box,
                 # 2 = a box whose content outgrew the pane and cannot be read,
                 # 3 = a pane that could not be read at all,
                 # 4 = a composer that is drawn but belongs to a selected
                 #     in-process subagent, so keys typed there reach the child
  local pane box rc=0
  # A PANE THAT COULD NOT BE READ IS NOT A PANE WITH NO BOX. The capture is
  # taken into a variable before awk sees it, because awk's verdict on empty
  # input reads exactly like its verdict on a pane carrying no composer.
  pane="$(tmux capture-pane -pJ -e -t "$1")" || return 3
  box="$(printf '%s\n' "$pane" | awk '
    { # A dim run ends at the next escape, whatever closes it — 0m here, but the
      gsub(/\033\[2m[^\033]*/, "")
      gsub(/\033\[[0-9;]*[A-Za-z]/, "")   # the rest of -e: attributes, zero width
      line[NR] = $0; if (NF) last = NR
      # THE AUTO-MODE ENVIRONMENT NUX OWNS INPUT even when its overlay leaves
      # the live composer painted underneath. Observed on claude-code 2.1.239:
      # it fires no PermissionRequest hook, so returning that underneath box as
      # usable lets status call the stranded agent idle and lets delivery spend
      # its safety checks on a composer that cannot receive keys.
      plain = $0
      sub(/^[[:space:]]*/, "", plain); sub(/[[:space:]]*$/, "", plain)
      if ($0 ~ /^▔+$/) auto_nux_band = NR
      if (plain == "Teach auto mode about your environment?" && auto_nux_band == NR - 1) {
        auto_nux_title = NR
      }
      if (plain == "←/→ to change usage · Enter to continue · Esc to cancel" && auto_nux_title && auto_nux_title < NR) {
        auto_nux_guide[NR] = auto_nux_title
      }
      t = $0; n = gsub(/─/, "", t)
      if (n && t == "") { prev = rule; prevw = rulew; rule = NR; rulew = n }
      # THE NAMED BAND. Once the ACTIVE conversation carries a name, the
      # harness burns that name into the rule that OPENS the composer and
      # leaves the closing rule pure, so the two rules stop matching and the
      # frame below stops being recognised at all. The band is recorded here
      # and read in END: it says which conversation owns the box under it, and
      # never on its own that the box is unusable.
      else if (n > 10) {
        band_line = $0; sub(/[[:space:]]+$/, "", band_line)
        band_name = t
        sub(/^[[:space:]]+/, "", band_name); sub(/[[:space:]]+$/, "", band_name)
        if (band_name != "" && band_line ~ /^─/ && band_line ~ /─$/) band = NR
      }
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
      # WHAT OPENED THE FRAME: ordinarily the matching pure rule above the
      # caret, and where the active conversation has a name, the band carrying
      # that name. The nearer of the two is the one this composer sits in.
      opened = 0; named = 0
      if (prev && rule && rulew == prevw) opened = prev
      if (rule && band && band < rule && band > opened) { opened = band; named = 1 }
      if (!rule || !opened) exit 1
      # The distance bound stops a stale rule far up the transcript standing in
      # for the rule that opened this box. A band earns that identity from
      # adjacency instead, and the agent switcher drawn under a named frame is
      # as long as the agent count, so the bound would only misread it.
      if (!named && last - rule > 5) exit 1
      # TEXT ALONE IS NOT THE VERDICT. The pure dialog band must touch its
      # title, and the guide must be the last nonblank row before the opening
      # composer rule. A body carrying the same prose lives after that rule,
      # so it remains readable; ordinary transcript prose without the native
      # band does too. These positions are the second question paired with the
      # exact pinned copy, not a general parser for native dialogs.
      before = opened - 1
      while (before && line[before] ~ /^[[:space:]]*$/) before--
      if (auto_nux_guide[before]) exit 1
      seen = 0; rows = 0
      for (i = opened + 1; i < rule; i++) {
        s = line[i]
        if (!seen) {
          if (s ~ /^[[:space:]]*$/) continue
          if (s !~ /^❯/) exit 1                # framed, but not the composer
          sub(/^❯/, "", s); seen = 1
        }
        box[++rows] = s
      }
      if (!seen) box[++rows] = ""
      # WHOSE COMPOSER IT IS. A titled parent session and a selected in-process
      # subagent draw the SAME named frame, and the name inside it is free
      # text, so the band cannot answer this. The footer can: the
      # permission-mode control belongs to the parent conversation and is drawn
      # under its composer in every mode (driven on claude-code 2.1.241 as
      # auto, manual and plan), while a selected child carries that child own
      # controls and no mode line at all.
      #
      # A named frame that cannot be shown to belong to the parent is refused
      # under its OWN status rather than reported as a pane with no composer:
      # driven on 2.1.241, typing into the selected child box resumed the
      # CHILD, which answered in its own transcript, so a parent-addressed
      # envelope delivered there would land in a subagent context.
      if (named) {
        mode = 0; elsewhere = 0
        for (i = rule + 1; i <= last; i++) {
          if (line[i] ~ / mode on/) mode = 1
          row = line[i]
          sub(/^❯/, "", row); sub(/^[[:space:]]+/, "", row)
          # The agent switcher marks the ACTIVE conversation with a filled ring
          # and every other one with an empty ring; the keyboard cursor is a
          # separate caret, so the row under the cursor is not the row in use.
          # main is the harness name for the root conversation.
          if (row ~ /^● /) {
            sub(/^● /, "", row); sub(/[[:space:]].*$/, "", row)
            if (row != "main") elsewhere = 1
          }
        }
        if (!mode || elsewhere) exit 4
        # AND WHERE IT IS THE PARENT, IT IS THE PARENT COMPOSER. A session that
        # has been named draws this frame around the box an agent has always
        # typed into: driven on 2.1.241, typing there reached the PARENT, which
        # answered in the parent transcript. Reporting it as no composer took
        # every titled session out of reach of delivery and left status calling
        # a healthy agent occupied by something it could not name.
      }
      for (i = 1; i <= rows; i++) print box[i]
    }')" || rc=$?
  [ "$rc" -eq 0 ] || return "$rc"
  printf '%s' "$box" | tr -d '\302\240'
}
