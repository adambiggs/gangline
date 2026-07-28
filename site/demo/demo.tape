# gangline demo — a Claude Code lead, a Codex worker, an opencode reviewer.
#
# Everything on screen is real: real harnesses on real subscriptions, real
# messages, real roster state. Hide/Show only skips the inference waits —
# hidden poll loops gate each cut on actual gang state, nothing is staged.
#
# Run via record.sh, which stages the disposable demo HOME this tape assumes.

Output demo.webm
Output demo.mp4

Set Shell bash
Set FontSize 20
Set Width 1280
Set Height 720
Set Padding 16
Set Theme "GitHub Dark"
Set WaitTimeout 600s
Set TypingSpeed 40ms

Env HOME "/home/demo"
Env TMUX_TMPDIR "/home/demo/.tmux"
Env PATH "/home/demo/bin:/home/demo/gangline/bin:/usr/local/bin:/usr/bin:/bin"

# Quiet setup: plain prompt, no history expansion (the send text has a bang).
Hide
Type "set +H; unset TMUX XDG_CONFIG_HOME XDG_DATA_HOME; PS1='$ '; cd ~; clear"
Enter
Sleep 1s
Show

# ── Scene 1: spawn a lead ────────────────────────────────────────────────────
Type "gang spawn lead -r lead -d ~/hello"
Sleep 500ms
Enter
Wait+Screen@120s /spawned lead/
Hide
Wait+Screen@300s /briefed lead as lead/
Show
Sleep 3s

# ── Scene 2: one attributed message; the lead runs the whole arc from it ─────
Type@20ms `gang send lead --from adam "team up: spawn a codex worker and an opencode reviewer (roles worker and reviewer, dir ~/hello). worker builds hello.py that prints Hello, world! and runs it to prove it. then the reviewer checks the work and reports to you. when the review is clean, summarize in your pane and go idle - i'm watching from outside tmux."`
Sleep 500ms
Enter
Hide
Wait+Screen@120s /delivered to lead as/
Show
Sleep 3.5s

# ── Scene 3: the lead has hired its own team; look at the roster ─────────────
Hide
Type `for i in $(seq 1 240); do gang roster 2>/dev/null | grep -q '^reviewer' && break; sleep 2; done; clear; echo S3-"OK"`
Enter
Wait+Screen@600s /S3-OK/
Type "clear"
Enter
Sleep 300ms
Show
Type "gang roster"
Sleep 500ms
Enter
Wait+Screen@30s /reviewer/
Sleep 4.5s

# ── Scene 4: attach — the lead orchestrating, live ───────────────────────────
Hide
Type "tmux select-window -t gangline:lead"
Enter
Sleep 500ms
Type "clear"
Enter
Type "gang attach"
Enter
Sleep 2s
Show
Sleep 5s

# ── Scene 5: the worker built it and proved it ───────────────────────────────
Hide
Ctrl+B
Sleep 300ms
Type "d"
Sleep 1s
Type `for i in $(seq 1 240); do gang roster 2>/dev/null | grep '^worker' | grep -q idle && break; sleep 2; done; echo S5-"OK"`
Enter
Wait+Screen@600s /S5-OK/
Type "tmux select-window -t gangline:worker"
Enter
Sleep 500ms
Type "gang attach"
Enter
Sleep 2s
Show
Sleep 6s

# ── Scene 6: the reviewer at work ────────────────────────────────────────────
Hide
Ctrl+B
Sleep 300ms
Type "d"
Sleep 1s
Type `for i in $(seq 1 90); do gang roster 2>/dev/null | grep '^reviewer' | grep -q busy && break; sleep 2; done; echo S6-"OK"`
Enter
Wait+Screen@300s /S6-OK/
Type "tmux select-window -t gangline:reviewer"
Enter
Sleep 500ms
Type "gang attach"
Enter
Sleep 2s
Show
Sleep 6s

# ── Scene 7: arc complete — everyone idle, the proof runs ────────────────────
Hide
Ctrl+B
Sleep 300ms
Type "d"
Sleep 1s
Type `for i in $(seq 1 300); do gang roster 2>/dev/null | grep -q busy || break; sleep 2; done; clear; echo S7-"OK"`
Enter
Wait+Screen@900s /S7-OK/
Type "clear"
Enter
Sleep 300ms
Show
Type "gang roster"
Sleep 500ms
Enter
Wait+Screen@30s /reviewer/
Sleep 4s
Type "python3 hello/hello.py"
Sleep 500ms
Enter
Wait+Screen@15s /Hello, world!/
Sleep 4.5s
