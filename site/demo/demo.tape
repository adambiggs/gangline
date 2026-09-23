# SPDX-License-Identifier: Apache-2.0
# Fresh Claude Code and Codex panes. record.sh owns setup and teardown.
Output site/demo/.demo-render.gif
Output site/demo/.demo-render.mp4
Set Shell bash
Set FontFamily "DejaVu Sans Mono"
Set FontSize 24
Set Width 640
Set Height 1320
Set Padding 10
Set Framerate 15
Set PlaybackSpeed 1
Set Theme { "name": "Gangline dark", "black": "#06090f", "red": "#f0a23c", "green": "#84aad6", "yellow": "#f0a23c", "blue": "#1f2860", "magenta": "#3d2444", "cyan": "#84aad6", "white": "#e9eef7", "brightBlack": "#7f8da0", "brightRed": "#f0a23c", "brightGreen": "#b2bed0", "brightYellow": "#f0a23c", "brightBlue": "#84aad6", "brightMagenta": "#b2bed0", "brightCyan": "#b2bed0", "brightWhite": "#ffffff", "background": "#06090f", "foreground": "#e9eef7", "selection": "#1f2860", "cursor": "#f0a23c" }
Set WaitTimeout 180s
Set TypingSpeed 35ms
Hide
Type `tmux -S "$GANG_TMUX_SOCKET" attach -t "$GANG_SESSION"`
Enter
Wait+Screen /Ready/
Sleep 3s
Show
Sleep 1s
Type "Ask builder to write greet.py: print Hello, team! Run it and report back."
Sleep 500ms
Enter
Wait+Screen@180s /● Demo complete:/
Sleep 6s
