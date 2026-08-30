# Gangline demo: talk to a real Claude Code lead and Codex worker, then watch them.

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
Type "unset TMUX TMUX_PANE; PS1='$ '; cd /tmp/gangline-demo-run; clear"
Enter
Show

# Native harnesses become ordinary named tmux windows.
Type "gang hitch lead -c claude-code -d /tmp -m sonnet -e high"
Enter
Wait+Screen@120s /delivered startup contract to lead/
Sleep 2s
Type "gang hitch worker -c codex -d /tmp -m gpt-5.6-terra -e high"
Enter
Wait+Screen@120s /delivered startup contract to worker/
Sleep 2s

# Attach is the view: these are the harnesses themselves, in named tmux windows.
Type "clear"
Enter
Type "gang attach"
Sleep 1s
Hide
Enter
Sleep 1s
Ctrl+b
Type "w"
Sleep 1s
Show
Sleep 5s
Hide
Escape
Ctrl+b
Type "d"
Sleep 2s
Show

# Talk opens a real editor; closing it delivers through Gangline's verified path.
Type "clear"
Enter
Type "gang talk lead"
Enter
Wait+Screen@30s /gang-talk\./
Type "i"
Type@45ms "Ask worker to read /tmp/gangline-demo-run/TASK.md."
Enter
Type@45ms "Verify the result, then finish with: TEAM READY"
Sleep 2s
Escape
Type ":wq"
Enter
Wait+Screen@60s /delivered to lead as/
Sleep 3s

# Let the native team work; neither inference nor polling is part of the cut.
Hide
Type `while [ ! -f answer.txt ]; do sleep 1; done; echo ARTIFACT-"READY"`
Enter
Wait+Screen@300s /ARTIFACT-READY/
Type `while ! gang capture lead 20 2>/dev/null | grep -q 'TEAM READY'; do sleep 1; done; echo LEAD-"READY"`
Enter
Wait+Screen@300s /LEAD-READY/
Type "clear"
Enter
Sleep 500ms
Show
Type "cat answer.txt"
Enter
Wait+Screen@30s /substrate/
Sleep 5s
