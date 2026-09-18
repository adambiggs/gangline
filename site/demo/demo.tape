# Gangline demo: a real Claude Code lead and Codex worker build the finale.

Output site/demo.gif
Output site/demo.mp4

Set Shell bash
Set FontSize 18
Set Width 1280
Set Height 720
Set Padding 18
Set Theme "GitHub Dark"
Set WaitTimeout 300s
Set TypingSpeed 32ms

Env GANG_SESSION "gangline-demo"
Env GANG_COLLAR "claude-code"
Env VISUAL "vim -f -i NONE"

Hide
Type "unset TMUX TMUX_PANE NO_COLOR; PS1='$ '; cd /tmp/gangline-demo-run; clear"
Enter
Show

# Native harnesses become ordinary named tmux windows.
Type "gang hitch lead -c claude-code -d . -m sonnet -e high -r demo-lead"
Enter
Wait+Screen@240s /delivered startup contract to lead/
Sleep 2s
Type "gang hitch worker -c codex -d . -m gpt-5.6-terra -e high"
Enter
Wait+Screen@240s /delivered startup contract to worker/
Sleep 2s

# Talk opens a real editor; closing it delivers through Gangline's verified path.
Type "clear"
Enter
Type "gang talk lead"
Enter
Wait+Screen@30s /gang-talk\./
Type "i"
Type@28ms "Build the animated terminal show in TASK.md with the Codex worker."
Enter
Type@28ms "Delegate TASK.md via gang send --to worker --stdin; do not spawn a subagent."
Enter
Type@28ms "When worker reports completion, summarize the result and end your turn."
Sleep 1s
Escape
Type ":wq"
Enter
Wait+Screen@60s /delivered to lead as/
Sleep 2s

# Once delegation lands, attach to the actual harness panes. The work starts on
# screen; the long inference stretch is cut rather than played in real time.
# The probes ask Gangline what the team is doing rather than reading the agents'
# prose: the reply the worker owes the lead is the brief arriving, and the
# attributed envelope in the lead's pane is the report coming back.
Hide
Type "$GANG_DEMO_DIAG/probe.sh worker briefed 300 && $GANG_DEMO_DIAG/probe.sh worker busy 60"
Enter
Wait+Screen@420s /PROBE-READY worker busy/
Type "clear"
Enter
Show
Type "gang attach"
Enter
Sleep 1s
Sleep 4s
Hide
Ctrl+b
Type "n"
Sleep 1s
Show
Sleep 5s
Hide
Ctrl+b
Type "d"
Sleep 1s

# Time-lapse through implementation and the lead's handling of the attributed
# completion report. Returning idle closes the lead's later turn.
Type "$GANG_DEMO_DIAG/probe.sh lead reported 400 && $GANG_DEMO_DIAG/probe.sh lead idle 60"
Enter
Wait+Screen@480s /PROBE-READY lead idle/
Type "clear"
Enter
Show
Type "gang capture lead 20"
Enter
Sleep 4s

# End on the artifact itself: the team's animated work owns the final screen.
Type "clear"
Enter
Type "python3 finale.py --hold 8"
Enter
Wait+Screen@30s /CONNECTED BY GANGLINE/
Sleep 6s
