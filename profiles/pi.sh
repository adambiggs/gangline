# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gl load_profile via source
# Pi coding agent. Busy marker observed live against the installed TUI: a running
# turn shows a braille spinner + "Working..."; the spinner char cycles, so match
# the stable token. Compact command from dist/core/slash-commands.js: "compact".
GL_LAUNCH="pi"
GL_BUSY_REGEX="Working\\.\\.\\."
GL_COMPACT_CMD="/compact"
