# Gangline demo: a real Claude Code lead and Codex worker build the finale.

Output site/demo/.demo-render.gif
Output site/demo/.demo-render.mp4

Set Shell bash
Set FontSize 24
Set Width 1280
Set Height 720
Set Padding 18
Set Theme { "name": "Gangline dark", "black": "#06090f", "red": "#f0a23c", "green": "#84aad6", "yellow": "#f0a23c", "blue": "#1f2860", "magenta": "#3d2444", "cyan": "#84aad6", "white": "#e9eef7", "brightBlack": "#7f8da0", "brightRed": "#f0a23c", "brightGreen": "#b2bed0", "brightYellow": "#f0a23c", "brightBlue": "#84aad6", "brightMagenta": "#b2bed0", "brightCyan": "#b2bed0", "brightWhite": "#ffffff", "background": "#06090f", "foreground": "#e9eef7", "selection": "#1f2860", "cursor": "#f0a23c" }
Set WaitTimeout 300s
Set TypingSpeed 32ms

Env GANG_SESSION "gangline-demo"
Env GANG_COLLAR "claude-code"
Env VISUAL "vim -f -i NONE"
Env HOME "/tmp/gangline-demo-home"

Hide
Type "unset TMUX TMUX_PANE NO_COLOR; PS1='demo$ '; cd /tmp/gangline-demo-run; clear"
Enter
Show

# Native harnesses become ordinary named tmux windows.
Type "gang hitch lead -c claude-code -d . -m sonnet -e high"
Enter
Wait+Screen@240s /delivered startup contract to lead/
Sleep 2s
Type "gang hitch worker -c codex -d . -m gpt-5.6-terra -e high"
Enter
Wait+Screen@240s /delivered startup contract to worker/
Sleep 2s

# The operator's brief reaches Claude Code, then its role delegates the artifact
# to Codex through Gangline's verified path.
Type "clear"
Enter
Type@20ms "printf '%s\\n' 'Delegate TASK.md to the Codex worker with gang send. Have it build and test the artifact, then leave the finished result ready for the recording.' | gang send lead --from operator"
Enter
Wait+Screen@60s /delivered to lead as/
Sleep 1s

# Once delegation lands, attach to the actual harness panes. The work starts on
# screen; the long inference stretch is cut rather than played in real time.
# This probe records Gangline's attributed delivery before the closing view.
Hide
Type "$GANG_DEMO_DIAG/probe.sh worker briefed 300"
Enter
Wait+Screen@360s /PROBE-READY worker briefed/
Sleep 5s
Type "clear"
Enter
Show
Type "gang attach"
Enter
Sleep 1s
Sleep 3s
Hide
Ctrl+b
Type "n"
Sleep 1s
Show
Sleep 3s
Hide
Ctrl+b
Type "d"
Sleep 1s

# The work is complete, so show the two native panes side by side. Gangline's
# registration remains intact while they work; this is the held closing view.
Type "lead_pane=$(tmux list-panes -a -F '#{window_name} #{pane_id}' | awk '$1 ~ /lead/ {print $2; exit}'); worker_pane=$(tmux list-panes -a -F '#{window_name} #{pane_id}' | awk '$1 ~ /worker/ {print $2; exit}'); tmux set -g status-style 'bg=#06090f,fg=#b2bed0'; tmux set -g pane-border-style 'fg=#1f2860'; tmux set -g pane-active-border-style 'fg=#f0a23c'; tmux set -g pane-border-status top; tmux set -g pane-border-format '#{pane_title}'; tmux join-pane -h -s $worker_pane -t $lead_pane; tmux select-layout -t $lead_pane even-horizontal; tmux select-pane -t $lead_pane -T 'Claude Code — lead'; tmux select-pane -t $worker_pane -T 'Codex — worker'"
Enter
Sleep 6s
