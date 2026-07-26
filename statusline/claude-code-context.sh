#!/usr/bin/env bash
# Claude Code statusline: render the gangline context beacon.
#
# The statusline payload natively carries context_window numbers, so this is a
# pure transform — no transcript IO, fast enough for the in-flight-cancel rule.
# The beacon is the machine-readable surface `gang context` / `gang roster`
# scrape from the pane (claude-code profile), the same way the pi profile reads
# Pi's native "NN%/NNNk" readout. Keep the format stable: "ctx <used>k/<win>k <pct>%".
#
# Wire via settings.json:
#   "statusLine": {"type": "command", "command": "<gangline>/statusline/claude-code-context.sh"}
SL_PAYLOAD="$(cat)" python3 - <<'PYEOF'
import json
import os
import sys

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
PYEOF
