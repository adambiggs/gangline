# Gangline demo: two real Codex sessions in a disposable tmux team.

Output site/demo.gif
Output site/demo.mp4

Set Shell bash
Set FontSize 18
Set Width 1280
Set Height 720
Set Padding 18
Set Theme "GitHub Dark"
Set WaitTimeout 300s
Set TypingSpeed 28ms

Env GANG_SESSION "gangline-demo"
Env GANG_PROFILE "codex"

Hide
Type "unset TMUX TMUX_PANE; PS1='$ '; cd /tmp/gangline-demo-run; clear"
Enter
Show

# Native harnesses become ordinary named tmux windows.
Type "gang hitch lead -p codex -d /tmp"
Enter
Wait+Screen@120s /delivered startup contract to lead/
Sleep 2s
Type "gang hitch worker -p codex -d /tmp"
Enter
Wait+Screen@120s /delivered startup contract to worker/
Sleep 2s
Type "gang roster"
Enter
Wait+Screen@30s /worker/
Sleep 4s

# The operator supplies a goal, not a coordination schema.
Type "clear"
Enter
Type@12ms `printf '%s\n' 'Use gang send --to worker --stdin to ask worker to read /tmp/gangline-demo-run/TASK.md, write /tmp/gangline-demo-run/answer.txt containing exactly substrate, and prove it with wc -c and od only; do not print file-owner metadata. You own only delegation and verification: do not edit the file yourself. Verify the worker result and finish with the exact words TEAM READY.' | gang send --to lead --from operator --stdin`
Enter
Wait+Screen@60s /delivered to lead as/
Sleep 4s

# Cut inference time, but gate the cut on the requested artifact.
Hide
Type `while [ ! -f answer.txt ]; do sleep 1; done; echo ARTIFACT-"READY"`
Enter
Wait+Screen@300s /ARTIFACT-READY/
Type "clear"
Enter
Sleep 500ms
Show
Type "gang capture worker 8"
Enter
Wait+Screen@30s /substrate/
Sleep 8s

# The lead receives the worker's attributed report and verifies the result.
Hide
Type `while ! gang capture lead 40 2>/dev/null | grep -q 'TEAM READY'; do sleep 1; done; echo LEAD-"READY"`
Enter
Wait+Screen@300s /LEAD-READY/
Type "clear"
Enter
Sleep 500ms
Show
Type "gang capture lead 40"
Enter
Wait+Screen@30s /TEAM READY/
Sleep 8s

# Gangline still owns only the transparent substrate and current state.
Type "clear"
Enter
Type "gang roster"
Enter
Wait+Screen@30s /worker/
Sleep 5s
Type "cat answer.txt"
Enter
Wait+Screen@30s /substrate/
Sleep 5s
