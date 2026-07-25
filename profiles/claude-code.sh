# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gl load_profile via source
# Claude Code CLI. Busy marker verified against the installed TUI: a running turn
# shows "esc to interrupt" in the status area; idle shows the input box without it.
GL_LAUNCH="claude"
GL_BUSY_REGEX="esc to interrupt"
