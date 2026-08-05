# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# OpenAI Codex CLI. Every marker here was watched live against the installed
# TUI, in the state it describes, before it was written down.
# The update check is turned off at launch, not in anyone's config file. Left
# on, a fresh Codex opens on "› 1. Update now / 2. Skip" whenever a release is
# out, and a hitched agent sits on that prompt instead of reading its brief —
# gang refuses the paste (correctly: it is a menu, not a composer) and the agent
# never starts. -c overrides one key for this process only, so an operator's own
# codex still tells them about updates.
# No sandbox or approval flag here: those are the operator's settings to choose,
# in their own config.toml. What they need to know is particular to Codex, and
# the README says it — its sandbox gates connect() on the network toggle without
# discriminating by address family, so a Codex agent with network access denied
# cannot open a UNIX socket either, and the tmux socket IS gangline's transport.
# Watched live, same socket and command: under workspace-write `tmux -S <sock>
# ls` gets EPERM; with sandbox_workspace_write.network_access = true it lists
# the session, and a worker hitched that way answers gang roster and gang status
# from its own pane. So the sandbox stays on and network access comes on — the
# operator sets both. Denied, the agent is reachable but mute: briefs arrive
# (gang writes from outside the sandbox) and nothing it runs itself gets back.
#
# Network access is necessary and not sufficient for gang send. Delivery also
# takes a per-pane lock under ${XDG_RUNTIME_DIR:-/tmp}/gangline-<uid>, and the
# sandbox leaves $XDG_RUNTIME_DIR outside its writable roots — mkdir there fails
# "Read-only file system" — so an agent holding the socket open can read the team
# and still not answer it. Point the lock at a directory the sandbox does leave
# writable and the same send succeeds; that is the whole of what stands between
# the two. Nothing in this file can do it: the lock is one every delivering
# process has to agree on, so where it lives is gang's to decide, not a
# profile's to relocate.
#
# Watch for this when re-verifying, because the same command gives opposite
# answers on two machines: where nothing sets XDG_RUNTIME_DIR the lock lands in
# /tmp, which the sandbox does leave writable, and send works with network access
# alone. Verify under systemd, which sets it, or the failure will not reproduce.
#
# What this launch line will NOT grow, however convenient it looks: anything that
# skips approval prompts or drops the sandbox — -c approval_policy=never,
# -c sandbox_mode=danger-full-access, --dangerously-bypass-approvals-and-sandbox.
# The pull toward it is real, because a Codex agent sitting behind a permission
# prompt reads `gated`, refuses every send, and no teammate can clear it. That is
# a thing to tell the operator, not a thing to fix here. How much authority a
# harness gets is the decision of the person at the keyboard, made in their own
# ~/.codex/config.toml; shipping it as a default hands it to every future
# installer without asking. See CONTRIBUTING, "Before adding code".
#
# OWNED-EVENT WRITERS, WITHOUT AN OWNERSHIP DECLARATION YET. Codex's official
# Hooks documentation says `-c` accepts a dotted key with a TOML value, and that
# matching hooks from separate configuration sources all run: this launch layer
# is additive to an operator's ~/.codex/hooks.json, never a rewrite of it. The
# nested value below was accepted by `codex doctor`; its malformed-shape control
# was refused.
#
# Doctor is NOT an event-name allowlist: a structurally valid invented event was
# accepted as `config loaded`. The event names therefore stay literal from the
# official table, in this one list, and must agree with cmd_hook's cases. Generating
# or spelling a second copy would restore the silent typo route the check cannot
# see.
#
# ONE COMMAND STRING FOR EVERY EVENT AND SEAT. Trust is attached to the hook
# definition, so putting a pane id in here would make every hitch a new trust
# decision. The absolute path is fixed when the profile is loaded; cmd_hook binds
# the firing process to its seat from inherited $TMUX_PANE instead. That inheritance
# was observed in Codex exec and interactive use, but it is ordinary process
# ancestry rather than a documented Codex contract.
#
# PostToolUse is wired only after the exact bytes cmd_hook emits were driven
# through a real Codex event: the tool result reached the raw rollout unchanged,
# and no decision-shaped key appeared in the reply. Its additionalContext is NOT
# a no-op — Codex records it in the model's context as a developer-role message.
# That is the intended route for an in-turn band nudge, and any future Codex
# hook_notes work must preserve it deliberately rather than discovering it anew.
#
# Real Codex turn bytes reached the shipped cmd_hook unchanged, python3 resolved
# and ran from the inherited PATH, and the resulting closed @gl_turn bracket was
# read back from the seat. The compaction pair and PermissionRequest have not been
# driven live: their ingestion is inferred from that shared payload path and the
# same documented event-name field, not claimed as observed. Phase one's absent
# declaration keeps that distinction honest if any of them never arrives.
#
# The wired set is stdout-safe as presently mapped: Stop, both compaction events,
# and PermissionRequest are silent; UserPromptSubmit has no Codex decision-channel
# vocabulary; PostToolUse's current reply was observed not to block or alter its
# tool result. PermissionRequest's reply object carries both a decision and
# additional context, so its silence is load-bearing.
#
# GANG_PROBE_FACTS stays unset. The readers already consume any records these
# writers produce, while an untrusted hook leaves no record and preserves today's
# scrape tier. Declaring ownership waits until bin/gang can make a declared fact
# that never arrives as loud as one that arrived and expired.
_gl_codex_hook="[{ hooks = [{ type = \"command\", command = \"\\\"$ROOT/bin/gang\\\" hook\" }] }]"
_gl_codex_hook_flags=""
for _gl_codex_event in UserPromptSubmit PostToolUse Stop PreCompact PostCompact PermissionRequest; do
  _gl_codex_hook_flags+=" -c 'hooks.$_gl_codex_event=$_gl_codex_hook'"
done
GANG_LAUNCH="codex -c check_for_update_on_startup=false$_gl_codex_hook_flags"
# Resuming after a tmux server death (ADR-0007). A full launch line rather than a
# flag, because codex spells resume as a SUBCOMMAND and no appended option could
# express it. Directory-scoped by default: `codex resume --all` is documented as
# "disables cwd filtering", which is what establishes that the default filters.
GANG_RESUME_LAUNCH="codex resume --last -c check_for_update_on_startup=false$_gl_codex_hook_flags"
unset _gl_codex_hook _gl_codex_hook_flags _gl_codex_event
# From `codex --help`: -m/--model, a bare model id ("gpt-5.6-sol").
GANG_MODEL_OPT="-m"
# One busy form that covers more than it looks like it does. A running turn
# paints "• Working (12s • esc to interrupt)"; the leading glyph pulses between
# • and ◦, the timer counts, and the tail grows segments when there is
# background work ("· 1 background terminal running · /ps to view · /stop to
# close"). The interrupt hint is the part that holds still. Compaction paints
# this same line and nothing else — Codex runs /compact as an ordinary turn —
# so a compacting agent reads busy, which is exactly what patrol needs and what
# a naive "is it done yet" marker would get wrong.
#
# What this deliberately does NOT match is a Codex waiting on an approval
# dialog: the interrupt hint is gone while the prompt is up, so busy honestly
# answers "no turn in flight". That state belongs to GANG_OCCUPIED_REGEX below.
#
# DO NOT SWEEP THIS STRING ACROSS PROFILES. Claude Code's TUI paints no
# interrupt hint at all, so the two profiles disagreeing about the same literal
# is the correct state and not drift to reconcile — delete it here to match and
# Codex loses busy detection entirely, in every state, because this is its only
# busy marker. Measured on clean panes that had never been told the string: 863
# frames of 12536 here, 0 of 9467 there. Grepping a harness binary settles
# neither direction: this string occurs ZERO times in the installed codex
# binary while painting on codex's own screen. Only a pane answers it.
GANG_BUSY_REGEX="esc to interrupt"
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
# Modal chrome, not dialog sentences. Codex frames every modal the same way: it
# draws the selected row with a "›" at column zero — the composer's own column —
# and numbers it. That "› N. " shape is the framing device, and it is already
# the shape profile_input refuses below, so the two halves of the gated check
# agree by construction instead of by coincidence.
#
# Watched live, in the state described, composer gone in every one: the command
# approval through a full ask→answer→erase cycle ("› 1. Yes, proceed (y)"), the
# /permissions modal ("› 1. Ask for approval (current)"), the /model picker
# ("› 1. gpt-5.6-sol (current)"), and the first-run trust prompt in a scratch dir
# ("› 1. Yes, continue") — declining exits the process, accepting repaints the
# composer with the wording gone, so a match means a dialog owns the screen right
# now, never scrollback.
#
# What deliberately does NOT match: the completion popups. Typing "/" or "@"
# opens a list UNDER a composer that still has the keyboard, and its rows carry
# no "› " cursor — the composer keeps it. Watched live; a send there lands in the
# composer, which is why those must read idle and do. /status is the same story
# from the other side: it renders into the transcript with the composer live
# below it, so it is not a gate and is not matched.
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
# Verified from the live slash menu: "/compact  summarize conversation to
# prevent hitting the context limit". A finished compaction leaves "Context
# compacted" in the transcript, where it stays — a marker for humans reading
# scrollback, never a state to scrape.
# The command is idle-only. Watched live on 0.145.0 across three self-issued
# attempts: Codex accepted the Enter, cleared the composer, and answered each one
# with "'/compact' is disabled while a task is in progress." So a Codex agent can
# never compact itself through gang: the tool call that types into its own TUI is
# itself a task in progress.
#
# A peer can, and patrol can nudge one into asking. Both of those rest entirely
# on profile_input below telling Codex's ghost text apart from a real draft:
# gang will not deliver a slash command into a box it cannot prove is empty, so
# a reader that cannot see that difference closes every compaction path this
# harness has at once, silently, while each individual refusal looks correct.
GANG_COMPACT_CMD="/compact"
# Codex takes input during a turn, and says so itself twice over: the hint row
# switches to "tab to queue message" while it is working, and the startup tip
# reads "Press Tab to queue a message when a task is running; otherwise it sends
# immediately". Watched end to end anyway — the paste lands in the composer,
# Enter moves it into the transcript under "Messages to be submitted after next
# tool call", the running turn is not disturbed, and the message is answered
# when that turn finishes.
GANG_MIDTURN_INPUT=1
# GANG_COMPACTING_REGEX is deliberately unset, and this one is not a gap in the
# observation — it is what the observation found. Codex draws compaction as an
# ordinary "Working (… esc to interrupt)" turn, with nothing on screen to tell
# it apart from any other turn. There is no marker to declare, so a resume
# waits for the pane to go quiet instead of queueing behind the compaction:
# slower by a few seconds, and correct without scraping anything.
# Every scraped marker in this file was live-verified against these harness
# versions. New release = re-verify + append (gang vet watches the pin).
GANG_VERSION_CMD="codex --version"
GANG_VERIFIED_VERSIONS="0.144.5 0.145.0 0.146.0"
# Codex paints no context readout a passive observer can reach — the composer
# hint row needs text typed to appear, and /status is a submitted command; both
# mean typing into somebody's session. So this profile reads the figure where
# Codex itself keeps it: the session rollout under
# ${CODEX_HOME:-~/.codex}/sessions/YYYY/MM/DD/rollout-*.jsonl, appended every
# turn with a token_count event. gang cannot know which rollout is which agent's
# from the outside, so it asks for a marker: GANG_SESSION_KEY=1 makes hitch mint
# a token, plant it in the agent's first message (where the rollout records it
# verbatim), and keep it in the window's @gl_key — exactly as long-lived as the
# agent, like every other per-agent state.
GANG_SESSION_KEY=1

codex_sessions_dir() { printf '%s/sessions' "${CODEX_HOME:-$HOME/.codex}"; }

codex_session_for() { # $1 = marker -> the one rollout that recorded it as user input
  local dir hits
  dir="$(codex_sessions_dir)"
  [ -d "$dir" ] || die "no codex sessions tree at $dir"
  hits="$(grep -rlF -- "$1" "$dir" 2>/dev/null)" \
    || die "marker not in any rollout under $dir — the hitch message may not be flushed yet"
  # grep finds every FILE carrying the marker; only the agent's own rollout
  # carries it as a user-role message. A teammate that captured the agent's pane
  # early can re-record the marker inside a tool-output record — a different
  # shape, filtered here. Two user-role hits is a repeated marker, and lookup
  # refuses to guess between them (law 8).
  printf '%s\n' "$hits" | python3 -c '
import json, sys
marker = sys.argv[1]
mine = []
for path in (l.strip() for l in sys.stdin if l.strip()):
    with open(path, encoding="utf-8", errors="replace") as f:
        for line in f:
            if marker not in line:
                continue
            try:
                rec = json.loads(line)
            except ValueError:
                continue
            p = rec.get("payload") or {}
            if p.get("type") == "message" and p.get("role") == "user":
                mine.append(path)
                break
if len(mine) != 1:
    n = len(mine)
    print(f"marker matches {n} rollouts as user input — "
          + ("not flushed yet, or the marker line never landed" if n == 0
             else "the marker was repeated; re-hitch the agent: " + " ".join(mine)),
          file=sys.stderr)
    sys.exit(1)
print(mine[0])
' "$1" || die "cannot bind this agent to a codex rollout"
}

codex_context_read() { # $1 = rollout path; prints "<used>k/<win>k (<pct>%)"
  # Occupancy is last_token_usage.total_tokens against model_context_window,
  # verified against codex-rs rust-v0.145.0 protocol.rs: TokenUsage::
  # tokens_in_context_window() returns `self.total_tokens`, and the TUI applies
  # it to the LAST turn's usage — the cumulative total_token_usage sums every
  # turn and overruns the window within minutes. Codex's own "% left" also
  # subtracts BASELINE_TOKENS (12000) from both sides; this readout stays raw
  # occupancy because the band ladder is absolute tokens, so the percent here
  # reads a few points higher than Codex's. Any missing field is format drift
  # and dies loudly — gang vet runs this same parser as its format gate.
  python3 -c '
import json, sys
path = sys.argv[1]
info = None
with open(path, encoding="utf-8", errors="replace") as f:
    for line in f:
        if "\"token_count\"" not in line:
            continue
        try:
            rec = json.loads(line)
        except ValueError:
            continue
        p = rec.get("payload") or {}
        if p.get("type") == "token_count" and p.get("info"):
            info = p["info"]
if info is None:
    print("no token_count event yet — codex reports usage after its first turn",
          file=sys.stderr)
    sys.exit(1)
try:
    used = info["last_token_usage"]["total_tokens"]
    win = info["model_context_window"]
    pct = round(100 * used / win)
except (KeyError, TypeError, ZeroDivisionError) as e:
    print(f"token_count schema drifted ({e!r} in {path}) — "
          "re-verify against the installed codex and update profiles/codex.sh",
          file=sys.stderr)
    sys.exit(1)
print(f"{round(used / 1000)}k/{round(win / 1000)}k ({pct}%)")
' "$1" || die "unreadable codex context in $1"
}

profile_context() { # $1 = tmux target; file-based — reads the rollout, never the pane
  local key file
  key="$(tmux show-options -wqv -t "$1" @gl_key)"
  [ -n "$key" ] \
    || die "window has no @gl_key — codex context needs the hitch-time session marker; adopted windows have none (re-hitch via gang hitch)"
  # The rollout path is derived once and cached on the window; the cache dies
  # with the window like the key that built it. Re-derived if the file vanishes.
  file="$(tmux show-options -wqv -t "$1" @gl_session)"
  if [ -z "$file" ] || [ ! -f "$file" ]; then
    file="$(codex_session_for "$key")"
    tmux set-option -w -t "$1" @gl_session "$file"
  fi
  codex_context_read "$file"
}

profile_vet() { # format gate: the parser above against the newest rollout on disk
  # Newest-first because drift ships with a codex release: old rollouts keep the
  # old shape forever and would vouch for a parser the current build has already
  # broken. A rollout with no token_count yet proves nothing either way — skip
  # it, it is just a session that has not finished a turn.
  local dir f
  dir="$(codex_sessions_dir)"
  while read -r f; do
    grep -q '"token_count"' "$f" 2>/dev/null || continue
    codex_context_read "$f" >/dev/null || return 1
    echo "OK (token_count parses: $(basename "$f"))"
    return 0
  done < <(find "$dir" -name 'rollout-*.jsonl' 2>/dev/null | sort -r | head -20)
  echo "no rollout with a token_count under $dir — nothing to gate"
  return 0
}

profile_input() { # $1 = tmux target; prints the composer, fails if there is none
  # Codex marks the selected row of a dialog with the same "›", in the same
  # column, as the composer: the trust prompt at startup ("› 1. Yes, continue")
  # and every command approval ("› 1. Yes, proceed (y)"). Claude Code indents
  # its menu cursor, so matching column zero is enough there; here it is not,
  # and the difference decides whether a brief gets pasted into a security
  # prompt and answered by a stray Enter.
  #
  # Both dialogs number their rows and the composer's opening characters are
  # whatever the operator typed, so a leading "N. " is the tell. A draft that
  # genuinely begins "1. " is refused rather than delivered — the wrong answer
  # in the safe direction, and a caller that sees the refusal can look.
  # Suggestion text is not draft text, and the difference is recoverable. Codex
  # paints rotating ghost text into an empty composer ("Implement {feature}")
  # dim — SGR 2 — and renders typed characters with no such attribute.
  # `capture-pane -p` THROWS THAT AWAY, so the offer and a real draft arrive as
  # the same bytes and no reader downstream can tell them apart. Measured live on
  # 0.145.0, both directions one variable apart in one box: the offer captures as
  # "\033[2mImplement {feature}\033[0m", a typed draft as "a real human draft"
  # carrying no attribute at all.
  #
  # What it cost while that attribute went unread, because it is the whole of
  # issue #53: the composer never read empty, so input_clear never succeeded.
  # patrol held every band nudge on "input box has content", `gang compact`
  # refused to deliver /compact into a box it could not prove was empty, and a
  # Codex agent therefore had NO compaction path through gang at all — not by
  # itself, not by a peer, not by patrol. An agent that is never nudged and can
  # never be compacted runs to the end of its window and loses the thread, which
  # is the exact failure roles/_common.md instructs every agent to prevent.
  #
  # So this captures WITH attributes and drops every dim run before looking.
  # What survives is what somebody typed. A line carrying both — typed prefix
  # plus a dim completion — keeps the prefix and loses the completion, which is
  # the case the whole distinction exists for.
  #
  # The residual runs toward false alarm, deliberately. A dim run broken across a
  # wrapped row could leave its continuation undimmed, which reads as a draft and
  # makes gang HOLD — a sweep costs nothing. The opposite error, typing over a
  # human's real draft, needs typed text to arrive dim, and typed text was
  # measured undimmed.
  #
  # The transcript echoes submitted messages behind the same "›", and those
  # rows carry SGR 1;2 rather than 2, so they survive the dim strip and are
  # matched here as well. Taking the LAST such row is what picks the composer
  # out of them: it is the bottom-most thing on screen.
  local line
  line="$(tmux capture-pane -pJ -e -t "$1" | awk '
    { # A dim run ends at the next escape, whatever closes it — 0m here, but the
      # rule holds for 22m or a colour change and does not depend on which.
      gsub(/\033\[2m[^\033]*/, "")
      gsub(/\033\[[0-9;]*[A-Za-z]/, "")   # the rest of -e: attributes, zero width
      # Anchored regex against a literal, never substr: a byte-oriented awk and a
      # character-oriented one disagree about how many units "›" is, and this
      # repo runs mawk locally and gawk in CI.
      if ($0 ~ /^›/) last = $0
    }
    END { if (!length(last)) exit 1; print last }')" || return 1
  case "$line" in '› '[0-9]*'. '*) return 1 ;; esac
  printf '%s' "${line#›}"
}
