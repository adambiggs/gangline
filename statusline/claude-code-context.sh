#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Claude Code statusline: render the gangline context beacon, and write the fact
# behind it.
#
# The statusline payload natively carries context_window numbers, so this is a
# pure transform — no transcript IO, fast enough for the in-flight-cancel rule.
# Two surfaces come out of one set of numbers:
#
#   THE BEACON, printed. The line a human reads, and the pane surface the
#   claude-code profile scrapes, the same way the pi profile reads Pi's native
#   "NN%/NNNk" readout. Keep the format stable: "ctx <used>k/<win>k <pct>%".
#
#   THE FACT, written to the window's @gl_ctx option. The context predicate's
#   owned tier (the evidence precedence rule), which gang prefers over scraping the beacon back off
#   the screen it just painted it onto. bin/gang's context-fact section is the
#   reader and sets out the record's shape; this is its only writer, and the two
#   are pinned against each other in the suite.
#
# THE PAINT COMES FIRST AND THE WRITE CANNOT COST IT. A statusline that dies
# blinds the human as well as the machine, so the beacon is printed before the
# write is attempted, no error from the write is fatal, and no tmux — invoked
# outside a pane, or with the server unreachable — is a silent paint-only run
# rather than a failure. That is law 7 wearing this surface's coat.
#
# Wire via settings.json:
#   "statusLine": {"type": "command", "command": "<gangline>/statusline/claude-code-context.sh"}
rc=0
out="$(SL_PAYLOAD="$(cat)" python3 - <<'PYEOF'
import json
import os
import sys
import time

try:
    p = json.loads(os.environ.get("SL_PAYLOAD", ""))
except json.JSONDecodeError:
    print("ctx -")
    sys.exit(0)

cw = p.get("context_window") or {}
usage = cw.get("current_usage") or {}
window = cw.get("context_window_size") or 0
if not usage or not window:
    print("ctx -")  # no API call yet (or just compacted); beacon returns next update
    sys.exit(0)

# Prompt-side tokens = what occupies the window going into the next call.
tokens = (
    usage.get("input_tokens", 0)
    + usage.get("cache_creation_input_tokens", 0)
    + usage.get("cache_read_input_tokens", 0)
)
print(f"ctx {tokens // 1000}k/{window // 1000}k {100 * tokens // window}%")
# The fact, on a second line the caller strips off before printing. Exact counts
# rather than the beacon's thousands, because nothing has to survive a screen to
# get here. A payload with no prompt-side tokens in it yet writes no second line
# at all — gang's reader refuses a zero, and having nothing to say and writing
# nothing are one code path rather than two.
if tokens > 0:
    print(f"ctx {tokens} {window} {int(time.time())}")
PYEOF
)" || rc=$?
# Not swallowed. A broken transform has already said so on stderr, and printing a
# plausible beacon over the top of it would report a status this script does not
# have (law 8).
[ "$rc" -eq 0 ] || exit "$rc"

printf '%s\n' "${out%%$'\n'*}"

case "$out" in
  *$'\n'*) ;;
  *) exit 0 ;;                        # beacon only: no figures to write
esac
[ -n "${TMUX_PANE:-}" ] || exit 0      # not in a pane: paint-only, silently
tmux set-option -w -t "$TMUX_PANE" @gl_ctx "${out#*$'\n'}" 2>/dev/null || true
