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
# the session, and a worker hitched that way answered gang roster and reported
# home through gang send. So the sandbox stays on and network access comes on —
# the operator sets both. Denied, the agent is reachable but mute: briefs arrive
# (gang writes from outside the sandbox) and nothing it runs itself gets back.
GANG_LAUNCH="codex -c check_for_update_on_startup=false"
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
# answers "no turn in flight". That state belongs to GANG_GATED_REGEX below.
GANG_BUSY_REGEX="esc to interrupt"
# Two dialogs, each watched live. The command approval (first alternate),
# through a full ask→deny→erase cycle: the question line holds still while the
# command, the reason, and the numbered menu vary around it. The first-run
# trust prompt (second alternate), through both answers in a scratch dir:
# declining exits the process, accepting repaints the composer with the wording
# gone — so a match means a dialog owns the screen right now, never scrollback.
# Its "›" menu cursor sits at column zero, the composer's own column, in the
# "› N. " row shape profile_input refuses below. The busy hint and the composer
# are gone while either dialog is up, so an unmarked gate read idle and a paste
# died on the missing composer with nothing saying why. A dialog worded
# differently falls back to that old behavior.
GANG_GATED_REGEX='Would you like to run the following command\?|Do you trust the contents of this directory\?'
# Verified from the live slash menu: "/compact  summarize conversation to
# prevent hitting the context limit". A finished compaction leaves "Context
# compacted" in the transcript, where it stays — a marker for humans reading
# scrollback, never a state to scrape.
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
# it apart from any other turn. There is no marker to declare, so a --resume
# waits for the pane to go quiet instead of queueing behind the compaction:
# slower by a few seconds, and correct without scraping anything.
# Every scraped marker in this file was live-verified against these harness
# versions. New release = re-verify + append (gang vet watches the pin).
GANG_VERSION_CMD="codex --version"
GANG_VERIFIED_VERSIONS="0.144.5 0.145.0"
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
  local line
  line="$(tmux capture-pane -pJ -t "$1" | grep '^›' | tail -1)" || return 1
  case "$line" in '› '[0-9]*'. '*) return 1 ;; esac
  # An empty composer is never empty: Codex paints rotating ghost text into it
  # ("Implement {feature}"), which reads as a draft. That is right for anything
  # asking "would a paste land on top of something" — better to hold than to
  # type over a human — and it is why patrol would never nudge a Codex agent
  # even once it can read one's context.
  printf '%s' "${line#›}"
}
