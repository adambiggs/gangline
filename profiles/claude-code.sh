# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# Claude Code CLI. Busy markers verified live against the installed TUI — two
# forms: some phases show "esc to interrupt", but tool execution and long turns
# render only a spinner status line (glyph + gerund + ellipsis), e.g.
# "✽ Generating…" or "* Hashing… (3m 56s · ↓ 12.1k tokens)". The completed form
# ("✻ Baked for 1s") persists in scrollback and must NOT match — it reads
# " for Ns", never an ellipsis. Observed false-negative: patrol nudged a
# mid-turn manager because only "esc to interrupt" was matched.
# Launched as the operator has it configured — no permission flag here, on
# purpose. A gangline agent is unattended by construction (nobody is watching
# its pane, a dialog stops the whole team, and gang will not answer one), so a
# harness on its interactive defaults stalls a team at the first prompt. The
# setting that fixes it belongs in the operator's own settings.json, where they
# can see it and change it back: "permissions": { "defaultMode": "auto" }.
# Choosing it here would put gang's thumb on a security decision that is not
# gang's to make. Want it in the launch line anyway? Shadow this profile via
# GANG_PROFILES and patch this one line.
GANG_LAUNCH="claude"
# From `claude --help`: --model takes an alias ("sonnet", "opus") or a full
# model id ("claude-fable-5").
GANG_MODEL_OPT="--model"
# READ THIS BEFORE ADDING AN ALTERNATE. These branches do not back each other
# up. Each one covers a DIFFERENT state, so every state below is read by exactly
# one branch, and a branch that dies takes its whole state with it. The pipes
# make this look like redundancy. It is not, and one branch was dead here for a
# release without anything failing, because the ones covering the other states
# went on matching and no check ever asked which branch fired.
#
# So each branch below says what was DRIVEN, what was OBSERVED, and what was
# never driven at all. An undriven branch and a verified one are indistinguish-
# able in a file, and that is how the dead one survived a release: nothing in
# gang vet ever fires these at a pane — it checks harness versions and the
# file-format parsers — so "verified" means a human watched a pane, and only
# where it says so below. Every absence recorded here was sampled at ~4Hz, so it
# means "not painted for a quarter second", not "never painted".
#
# FIRST FORM, THE THINKING SPINNER: "✶ Zesting… (29s · ↓ 313 tokens · thought
# for 26s)". A spinner glyph, a gerund, then a parenthetical of telemetry. The
# glyph cycles (* · ✢ ✶ ✻ ✽) and the gerund is whimsical vocabulary (Zesting,
# Billowing, Blanching, Canoodling), so the SHAPE is the marker and the words
# are not — do not enumerate them. Driven and observed on clean panes.
#
# READ THIS BEFORE TRUSTING IT: it covers THINKING and nothing else. When
# claude-code begins emitting an answer it removes the spinner and paints no
# replacement, so a streaming turn is bare. Measured on a clean throwaway: of
# 220 samples with the pane demonstrably changing, 19 carried any branch of this
# regex — 8.6% — and the longest unbroken run of live-but-unmarked samples was
# 64, about 16 seconds. Streaming is most of a long turn. So busy() reading
# false does NOT mean no turn is in flight on this harness, and every caller
# that assumes it does is wrong for the length of the answer.
#
# NOTHING MARKS A STREAMING TURN, and that was measured rather than assumed. 201
# streaming frames compared line for line against an idle one: no line persists
# while streaming that is absent when idle, ANYWHERE on the pane — not merely
# outside the rows capture_status reads, so widening the scan reaches nothing.
# A streaming pane and an idle pane differ in exactly one way, that one of them
# keeps changing, and no single frame can be asked that. The same comparison
# picks the thinking spinner out immediately, so the instrument was not blind;
# and gang captures without -e, so a state marked by colour alone would be
# invisible to gang whatever this file declared. Do not close this by widening
# the branch above: a guessed alternate is a fourth single point of failure
# sharing this one variable.
#
# Latent, in this same branch, and NOT the cause of the above: the gerund shape
# accepts a SINGLE capitalised word. Every label yet observed is one word, so it
# matches everything the harness currently paints — and a two-word thinking
# label would miss entirely.
#
# SECOND FORM, API-RETRY BACKOFF: "✻ 529 Overloaded · Retrying in 2s ·
# attempt 4/10" — a turn is in flight but a digit follows the glyph, so the
# gerund shape misses it and this needs its own branch. NEVER DRIVEN: a 529 does
# not arrive on demand, and it appeared in none of the samples above. There is
# no live observation behind this branch at all. It stays because if it is right
# it is the sole cover for a whole state, and a false idle costs more than a
# dead branch.
#
# THIRD FORM, THE PROGRESS BAR: not observed in any state, and kept anyway. It
# was this file's compaction cover and is not any more — 2.1.220 paints no bar
# during a compaction (637 samples bracketing a real one: zero hits for these
# glyphs, zero for the whole of this regex) and none on an ordinary turn (0 of
# 14862 unfiltered whole-pane frames, in a capture that recorded four other
# multibyte glyphs from the same footer). So these two alternates are NOT
# defending against what put them here: a compacting pane reads idle.
#
# They stay because absence is not death, and because the two removals are not
# the same act. Dropping a DECLARATION whose absence is safe needs only that it
# cannot be shown to fire. Dropping a BUSY BRANCH needs more, because if it is
# alive in a state nobody drove — an install, a plugin setup, a download — gang
# reads idle while the harness works: sends take the wrong path, patrol nudges a
# live turn, wait returns early. The frames above cover a compaction and a
# handful of turns, and say nothing about those.
#
# Literal alternation, not [▰▱]: in cron's C locale a multibyte bracket class
# matches single BYTES, and the welcome-logo glyphs share the UTF-8 prefix.
GANG_BUSY_REGEX='^[^ ] [A-Z][a-zé]+(…|\.\.\.) *(\(|$)|Retrying in [0-9]+s|▰|▱'
# The other half is measured here too, and it is why the signal exists: during a
# live turn this runs a render loop that parks the cursor into the composer rows
# on every frame, 748 of 748, and a cursor move changes no cell. So it keeps
# writing while the screen holds still — 44 of 47 byte-identical pairs ticked in
# the blind state, and a turn blocked 60s inside a tool call with zero output
# ticked every second it had. The spinner is timer-driven, not output-driven.
# Measured quiet at rest: 30 samples at 1s with the composer empty and the agent
# finished, #{window_activity} frozen on one value, 0 ticks. That is what this
# declares. A harness that repaints at rest can fabricate this signal for as long
# as it keeps writing, so gang bounds activity-only busy globally; the declaration
# still stays unset until somebody has watched a finished agent sit still, because
# a wrong declaration costs that entire bound on every false episode.
GANG_QUIET_AT_REST=1
# Every scraped marker in this file (busy regex, ctx beacon shape, input-box
# shape) was live-verified against these harness versions. New release =
# re-verify + append (gang vet watches the pin).
GANG_VERSION_CMD="claude --version"
GANG_VERIFIED_VERSIONS="2.1.220"
# Compact command verified live: /compact is the built-in context compaction
# slash command in the installed Claude Code TUI.
GANG_COMPACT_CMD="/compact"
# GANG_COMPACTING_REGEX is deliberately unset, and that is a measurement rather
# than an omission. It was the bar glyphs and nothing else, and 2.1.220 paints no
# bar during a compaction: 637 samples across a real one on a clean pane, only 2
# of them a change, zero hits for the glyphs and zero for the whole busy regex.
# Removing it is BEHAVIOUR-NEUTRAL — a declaration that never matched bought
# nothing, so deleting it neither opens the hole below nor closes it. It stops
# hiding it. gang branches on the declaration, so an absent one is honest where a
# false one buys a fast path that does not exist.
#
# The hole, written down so nobody has to re-derive it. A compacting pane is
# guarded three ways and on this harness all three are blind at once:
# compacting() is unset, busy() is false (no bar, no spinner), and pane_stable
# PASSES because the screen does not move — 633 consecutive byte-identical
# captures, 151 seconds, one frame. That frame replayed into a pane reads "idle".
#
# What actually held patrol off is an accident worth naming. The composer stays
# drawn for the whole compaction and holds "Press up to edit queued messages", so
# input_clear is false and patrol holds its nudge and reports "input box has
# content" — the right action for the wrong reason, and only because that agent
# happened to have queued input. One that compacts with an empty box gets nudged
# mid-compaction, which is the overtaking the waiter exists to prevent.
#
# Two things a future marker hunt should not have to rediscover: the composer is
# NOT a compaction signal here, because it is drawn throughout — "detect
# compaction by the box going away" is already answered — and no bar glyph of any
# of the three sets the binary carries was painted at any point.
# Input pasted during a turn is taken, not dropped and not fed to whatever the
# turn is running: the box accepts the paste and Enter replaces it with "Press up
# to edit queued messages". Verified live, which is what lets a send reach a
# working agent instead of bouncing off it. What the flag does NOT promise is
# when: text is often handed straight to the turn already running, while a queued
# slash command waits for that turn to end — so text submitted after a command
# can still arrive before it (see resume_after_compaction).
GANG_MIDTURN_INPUT=1
# Ordinary text was observed being consumed by the RUNNING turn on 2.1.220,
# before that turn reached its boundary. That is the witness behind the parked
# availability override; codex, opencode and Pi were observed queueing instead
# and deliberately leave this unset.
#
# gang vet --probe cannot verify this declaration. The tty exposes no turn
# identity, so "this turn consumed it" and "this turn ended and the queued next
# turn began between polls" paint indistinguishable sequences.
GANG_MIDTURN_ACTS=1
# Modal chrome, not dialog sentences. Claude Code frames every modal the same
# two ways, and both were watched live: a selection cursor "❯" that is INDENTED,
# and an "Esc to <verb>" footer. Indentation is the whole tell — the composer's
# own "❯" sits at column zero (profile_input below keys on exactly that), so a
# "❯" with space in front of it is a menu row, which means a menu owns the
# screen. The footer covers the modals that carry no menu at all.
#
# Watched live, in the state described, with the composer gone in every one:
# the shell approval (" ❯ 1. Yes" · "Esc to cancel · Tab to amend"), the plan
# approval ("   ❯ 1. Yes, and use auto mode", NO Esc footer), the first-run
# trust modal (" ❯ 1. Yes, I trust this folder" · "Esc to cancel"), /model,
# /theme and /config (indented "❯" plus a footer), and /status (footer only —
# an info screen with no menu). Neither alternate covers all seven; the union
# does, which is why both are here.
#
# Enumerating the sentences is what this replaces, and the plan approval is the
# receipt: it asks "Would you like to proceed?", so the old
# "Do you want to proceed\?" never matched it and a pane stopped dead on a
# permission dialog reported idle.
GANG_GATED_REGEX='^ +❯|Esc to'

profile_context() { # $1 = tmux target; reads the gangline statusline beacon
  # Claude Code shows no context numbers natively — the shipped statusline
  # (statusline/claude-code-context.sh) paints "ctx <used>k/<win>k <pct>%"
  # into the pane from the statusline payload's own context_window figures.
  local m
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || die "no ctx beacon in pane — wire statusline/claude-code-context.sh into settings.json statusLine (see README)"
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}

profile_vet() { # setup gate: is anything actually painting the beacon read above?
  # profile_context dies when the beacon is absent, and that die lands on whoever
  # runs `gang context` — one agent at a time, long after `gang up`, for what is a
  # single host-level fault. Whether it is wired is a static configuration fact,
  # so vet can answer it before a team exists: on the run where every version row
  # reads OK and the operator is asking why the roster context column is all
  # dashes, this is the row that says why.
  #
  # Scope is stated in the finding rather than assumed away. Settings merge across
  # four scopes with managed highest and user LOWEST, and the two in between are
  # per-project (docs.claude.com, "Settings precedence"). vet cannot reach those:
  # `gang hitch -d` takes any directory, so the project an agent will run in is
  # not known here. Reading both host-level scopes is what keeps an OK honest;
  # naming what was not read is what keeps a DRIFT honest.
  #
  # Gated on the binary, not on the settings file, and the difference is the whole
  # target: a fresh install with claude present and nothing configured is exactly
  # the case worth catching, while a host that runs codex alone must never be
  # nagged about a harness it does not have.
  command -v claude >/dev/null \
    || { echo "claude is not installed — nothing to gate"; return 0; }
  local beacon out status detail cmd src tok
  beacon="$ROOT/statusline/claude-code-context.sh"
  # Lowest precedence first, so a later file wins the same way the harness
  # resolves them. CLAUDE_CONFIG_DIR relocates the user scope: in the installed
  # 2.1.220 binary the user settings path is `Kx.join(y3r("userSettings"),
  # bIh(t))` where `bIh` returns "settings.json" and `y3r` resolves that scope to
  # the config dir, which `function ERl(){return process.env.CLAUDE_CONFIG_DIR}`
  # selects. Reading $HOME/.claude on a host that sets it would report a false
  # finding against a file the harness never opens.
  #
  # Protocol out of python, tab separated: <status> <detail> <cmd> <src> <tok>.
  # On `bad`, detail is the whole finding and the rest are empty.
  out="$(python3 - "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/settings.json" \
                   /etc/claude-code/managed-settings.json \
                   "/Library/Application Support/ClaudeCode/managed-settings.json" <<'PY'
import json, os, shlex, sys

read, cmd, src = [], "", ""
for p in sys.argv[1:]:
    try:
        with open(p) as fh:
            d = json.load(fh)
    except FileNotFoundError:
        continue
    except (OSError, ValueError) as e:
        # Existing-but-unreadable is a third answer, never "not configured": the
        # file may hold the wiring, so the honest report is that vet could not
        # tell, and the caller fails loud rather than guessing either way.
        print("bad\t%s does not parse (%s), so whether the beacon is wired here "
              "is undetermined\t\t\t" % (p, e))
        sys.exit(0)
    if not isinstance(d, dict):
        print("bad\t%s holds a JSON %s at its top level, not an object, so "
              "whether the beacon is wired here is undetermined\t\t\t"
              % (p, type(d).__name__))
        sys.exit(0)
    read.append(p)
    sl = d.get("statusLine")
    if isinstance(sl, dict) and sl.get("command"):
        cmd, src = sl["command"], p

# The setting is a shell command string, so the beacon can sit behind an
# interpreter or carry arguments. Split it the way a shell would and match the
# script by NAME: an operator who installed gangline elsewhere, or who copied the
# script, is still wired — and a path that no longer exists is a different
# finding from no path at all, which is why the token comes back rather than a
# yes/no. "?" is the name matching with no token the caller can test: either the
# string would not split, or the path is relative, and a relative one resolves
# against the harness's working directory rather than vet's. Both are "wired,
# and this is the wrong place to check the file" — never a finding.
tok = ""
try:
    for w in shlex.split(cmd):
        if os.path.basename(w) == "claude-code-context.sh":
            tok = w if os.path.isabs(w) else "?"
except ValueError:
    tok = "?" if "claude-code-context.sh" in cmd else ""

print("ok\t%s\t%s\t%s\t%s"
      % (", ".join(read) or "any settings file gang can read here", cmd, src, tok))
PY
)" || { echo "python3 could not read the settings files"; return 1; }
  IFS=$'\t' read -r status detail cmd src tok <<<"$out"
  [ "$status" = ok ] || { echo "$detail"; return 1; }
  if [ -z "$cmd" ]; then
    echo "no statusLine command in $detail, so nothing paints the \"ctx <used>k/<win>k <pct>%\" beacon and gang context, the roster context column and every patrol band are blind on this harness. Wire it: \"statusLine\": {\"type\": \"command\", \"command\": \"$beacon\"}. Project-scope .claude/settings.json is not read here — the directory an agent will be hitched in is not known at vet time"
    return 1
  fi
  if [ -z "$tok" ]; then
    echo "$src points statusLine at \"$cmd\", which is not gangline's beacon script. Unless that command paints a \"ctx <used>k/<win>k <pct>%\" line itself, gang has no context readout on this harness. This install's beacon is $beacon, and gang vet --probe settles it against a live pane"
    return 1
  fi
  if [ "$tok" != "?" ] && [ ! -r "$tok" ]; then
    echo "$src wires the beacon as $tok and that path is not readable, so the statusline runs nothing and the pane carries no beacon. This install's copy is $beacon"
    return 1
  fi
  echo "OK (context beacon wired in $src)"
}

profile_input() { # $1 = tmux target; prints what a HUMAN TYPED, fails if no box
  # Suggestion text is not draft text, and the difference is recoverable. Claude
  # Code renders its empty-box placeholder ('Try "how do I log an error?"') and
  # its autosuggest ghost text dim — SGR 2 — and renders typed characters with no
  # such attribute. `capture-pane -p` THROWS THAT AWAY, so ghost and draft arrive
  # as the same bytes on the grid and no reader downstream can tell them apart.
  # Measured live, both directions, one variable apart in one box: placeholder
  # "^[[2mTry ...^[[0m", typed "❯ log an error" with no SGR 2, including a draft
  # long enough to wrap. Lead read "❯ commit the docs" off its own idle pane and
  # took it for an unsent operator draft; it was ghost text, and it was dim.
  #
  # So this captures WITH attributes and drops every dim run before looking. What
  # survives is what somebody typed. A line carrying both — typed prefix plus a
  # dim completion — keeps the prefix and loses the completion, which is the case
  # the whole distinction exists for. Whether Claude Code emits that mixed line at
  # all is NOT established here; the reader handles it because getting it right
  # costs nothing, not because it was seen.
  #
  # The residual runs toward false alarm. A dim run broken across a wrapped row
  # could leave its continuation undimmed, which reads as a draft and makes gang
  # HOLD — a sweep costs nothing. The opposite error, typing over a human's real
  # draft, needs typed text to arrive dim, and typed text was measured undimmed.
  #
  # The input box is the "❯" line inside the composer's own frame: two rules of
  # "─" drawn at column zero, one directly above the box and one directly below
  # it. The frame is what identifies the box, because the prompt character alone
  # does not — Claude Code echoes every SUBMITTED message into the transcript
  # behind the same column-zero "❯". Taking the last "❯" line therefore found a
  # message sent minutes ago and reported a box that was no longer on screen.
  # Lived it: a pane stopped at a Bash approval dialog read idle, because the
  # user message above the dialog still matched — so the gate never fired, and a
  # send would have landed in the dialog and answered it with its own Enter.
  #
  # A modal takes the frame with it. The approval dialogs draw a single rule as
  # their top edge and none below; the plan approval and /theme indent theirs and
  # draw them narrower; /model, /config and /status use a different glyph
  # entirely. None of them leaves a pair, so none of them is mistaken for a box —
  # which is what lets hitch's no-input-box refusal catch a dialog before a brief
  # can answer it.
  #
  # Failing when there is no frame carries real information both ways: before the
  # TUI has painted it the harness is not yet taking input, and once it is gone
  # the screen belongs to something else — either way, hold. A human draft, a
  # dimmed ghost-text suggestion, and the "Press up to edit queued messages" hint
  # all render as text after the prompt char, and a draft can run to several rows
  # inside the frame, so every row of it is printed.
  #
  # Multibyte care, twice over. The rule test is a gsub of a literal "─" with the
  # remainder checked for emptiness — a literal matches as a byte sequence in
  # cron's C locale, where "─+" would quantify only the last byte and a bracket
  # class would match single bytes. And the idle cursor cell captures as U+00A0
  # (no-break space), whose [:space:] membership is locale-dependent, so its
  # bytes are stripped rather than trusted to a class.
  local box
  box="$(tmux capture-pane -pJ -e -t "$1" | awk '
    { # A dim run ends at the next escape, whatever closes it — 0m here, but the
      # rule holds for 22m or a colour change and does not depend on which.
      gsub(/\033\[2m[^\033]*/, "")
      gsub(/\033\[[0-9;]*[A-Za-z]/, "")   # the rest of -e: attributes, zero width
      line[NR] = $0; if (NF) last = NR
      t = $0; n = gsub(/─/, "", t)
      # All rule glyphs and nothing else. Leading spaces survive the gsub, so
      # this pins the rule to column zero too, and an indented dialog edge is
      # rejected by the same test. Width is carried along: the two rules of one
      # frame always match each other.
      if (n && t == "") { prev = rule; prevw = rulew; rule = NR; rulew = n }
    }
    END {
      if (!prev || !rule || rulew != prevw) exit 1
      # A stale rule higher up the transcript must not pair with the top edge of
      # a dialog and bracket the transcript between the two. The real frame
      # closes at the bottom of the screen, with only the ctx beacon and the
      # permission-mode hint under it.
      if (last - rule > 5) exit 1
      seen = 0
      for (i = prev + 1; i < rule; i++) {
        s = line[i]
        if (!seen) {
          if (s ~ /^[[:space:]]*$/) continue
          # Anchored regex, not substr(s, 1, 3). Counting to 3 assumes the prompt
          # glyph is three BYTES, which holds in a byte-oriented awk and fails in
          # a character-oriented one: there substr takes three CHARACTERS, so a
          # composer line comes back as "❯ c" and never equals "❯". The box is
          # then rejected on every pane, and gang reports no input box at all —
          # hold on every send, on a machine whose only difference is which awk
          # is installed. A literal in a regex matches the same way under both.
          if (s !~ /^❯/) exit 1                # framed, but not the composer
          sub(/^❯/, "", s); seen = 1
        }
        print s
      }
      if (!seen) print ""
    }')" || return 1
  printf '%s' "$box" | tr -d '\302\240'
}
