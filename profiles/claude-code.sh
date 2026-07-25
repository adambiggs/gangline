# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# Claude Code CLI. Busy marker verified against the installed TUI: a running turn
# shows "esc to interrupt" in the status area; idle shows the input box without it.
GANG_LAUNCH="claude"
GANG_BUSY_REGEX="esc to interrupt"
