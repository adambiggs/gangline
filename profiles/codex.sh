# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# OpenAI Codex CLI. Every marker here was watched live against the installed
# TUI, in the state it describes, before it was written down.
# The update check is turned off at launch, not in anyone's config file. Left
# on, a fresh Codex opens on "› 1. Update now / 2. Skip" whenever a release is
# out, and a spawned agent sits on that prompt instead of reading its brief —
# gang refuses the paste (correctly: it is a menu, not a composer) and the agent
# never starts. -c overrides one key for this process only, so an operator's own
# codex still tells them about updates.
GANG_LAUNCH="codex -c check_for_update_on_startup=false"
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
# dialog: the interrupt hint is gone while the prompt is up, so such an agent
# reads idle. That is the honest answer to "is a turn in flight" — none is —
# and the guard that matters is the input box below, which finds no composer
# and refuses the paste rather than answering somebody's security prompt.
GANG_BUSY_REGEX="esc to interrupt"
# Verified from the live slash menu: "/compact  summarize conversation to
# prevent hitting the context limit". A finished compaction leaves "Context
# compacted" in the transcript, where it stays — a marker for humans reading
# scrollback, never a state to scrape.
GANG_COMPACT_CMD="/compact"
# Codex takes input during a turn, and says so itself: paste while it is
# working and the hint row switches to "tab to queue message". Watched end to
# end — the paste lands in the composer, Enter moves it into the transcript,
# the running turn is not disturbed, and the message is answered when that turn
# finishes.
GANG_MIDTURN_INPUT=1
# GANG_COMPACTING_REGEX is deliberately unset, and this one is not a gap in the
# observation — it is what the observation found. Codex draws compaction as an
# ordinary "Working (… esc to interrupt)" turn, with nothing on screen to tell
# it apart from any other turn. There is no marker to declare, so a --resume
# waits for the pane to go quiet instead of queueing behind the compaction:
# slower by a few seconds, and correct without scraping anything.
# Every scraped marker in this file was live-verified against these harness
# versions. New release = re-verify + append (gang doctor watches the pin).
GANG_VERSION_CMD="codex --version"
GANG_VERIFIED_VERSIONS="0.144.5"

# profile_context is deliberately absent, so `gang context` on a Codex agent
# fails loudly and patrol reports it as not patrolled rather than guessing.
# Codex does render a readout — "99% context left" — but only in the composer's
# hint row, and only while the composer holds text: idle with an empty composer,
# and idle with text, both show the model-and-directory row instead. Reading it
# would mean typing into the agent to make the number appear, which is the one
# thing a passive observer must never do. The full figure lives in /status
# ("Context window: 99% left (14.7K used / 258K)"), and submitting a slash
# command into somebody's session to measure it is the same trespass. If a
# later build paints the percentage where it can simply be read, this is four
# lines of awk and Codex joins the band ladder.

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
