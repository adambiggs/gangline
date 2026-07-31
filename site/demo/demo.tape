# gangline demo — a Claude Code lead, a Codex worker, an opencode reviewer.
#
# Everything on screen is real: real harnesses on real subscriptions, real
# messages, real roster state. Hide/Show only skips the inference waits —
# hidden poll loops gate each cut on actual gang state, nothing is staged.
#
# Pacing is a feature, not padding. Each pane arrives holding a screenful of
# agent output, and a cut that lands before it can be read shows that an agent
# was there without showing what it did. Every dwell below is set to the slowest
# thing on that screen, not the fastest.
#
# Each gate prints its marker only when the state it is waiting for actually
# arrives, so vhs's own Wait timeout is the single deadline. A loop that gave up
# and printed the marker anyway once cost a whole take: the lead had named its
# worker "builder", every `^worker` gate fell through, and the tape carried on
# selecting windows that were not there. Failing the recording is cheaper than
# auditing a take that looks nearly right.
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

# ── Scene 1: gang up — the documented first command, verbatim ────────────────
# The README's quickstart opens with this line and nothing else, so the demo opens
# with it too: no arguments, and it ends with a briefed lead you are attached to.
Type "cd ~/hello && gang up"
Sleep 600ms
Enter
Hide
# The attach and the detach both happen behind this cut, and they have to. Claude
# Code's boot box names the account it is signed in as, and it re-fetches that
# profile from the server on the way up — so scrubbing the seeded config does not
# hold, and `gang up` attaches the instant the brief verifies. What the cut resumes
# on is the shell gang up printed to, with the whole exchange still above the
# prompt. The lead's own pane gets its scene once its output has pushed that box
# off the screen, which scene 4 waits for explicitly.
Wait+Screen@120s /hitched lead/
Wait+Screen@300s /briefed lead as lead/
Sleep 3s
Ctrl+B
Sleep 400ms
Type "d"
Sleep 1.5s
Show
Sleep 6.5s

# ── Scene 2: task the lead — one attributed message, and it runs the arc ────
Type "clear"
Enter
Sleep 300ms
# The names are pinned, not left to the lead: the gates below select windows by
# name, and a lead free to call its worker whatever it likes will.
Type@20ms `gang send lead --from adam --stdin <<'EOF'`
Sleep 300ms
Enter
Type@20ms `hitch a codex agent named worker (role worker, dir ~/hello): build hello.py that prints Hello, world! and run it to prove it. once worker reports done, hitch an opencode agent named reviewer (role reviewer, dir ~/hello) to check the work and report to you. when the review is clean, summarize in your pane and go idle - i'm watching from outside tmux.`
Enter
Type `EOF`
Sleep 300ms
Enter
Hide
Wait+Screen@120s /delivered to lead as/
Show
Sleep 4.5s

# ── Scene 3: the lead has hired a worker ────────────────────────────────────
Hide
Type `while :; do gang roster 2>/dev/null | grep -q '^worker' && { clear; echo S3-"OK"; break; }; sleep 2; done`
Enter
Wait+Screen@600s /S3-OK/
Type "clear"
Enter
Sleep 300ms
Show
Type "gang roster"
Sleep 500ms
Enter
Wait+Screen@30s /worker/
Sleep 6.5s

# ── Scene 4: attach — the lead orchestrating, live ───────────────────────────
Hide
# Nothing is attached to until the account name is off the lead's screen. By now
# its own transcript has always scrolled the boot box away, but "always" is not a
# guarantee, and the one frame this protects against is the one that would matter.
Type `while :; do gang capture lead 40 | grep -q Organization || { echo S4-"OK"; break; }; sleep 2; done`
Enter
Wait+Screen@300s /S4-OK/
Type "tmux select-window -t gangline:lead"
Enter
Sleep 500ms
Type "clear"
Enter
Type "gang attach"
Enter
Sleep 2s
Show
Sleep 10s

# ── Scene 5: the worker built it and proved it ───────────────────────────────
Hide
Ctrl+B
Sleep 300ms
Type "d"
Sleep 1s
# Idle alone is not done: a freshly booted pane reads idle too. The file the
# worker was asked for is the fact, and the pane behind it will show the proof.
Type `while :; do [ -f ~/hello/hello.py ] && gang roster 2>/dev/null | grep '^worker' | grep -q idle && { echo S5-"OK"; break; }; sleep 2; done`
Enter
Wait+Screen@600s /S5-OK/
Type "tmux select-window -t gangline:worker"
Enter
Sleep 500ms
Type "gang attach"
Enter
Sleep 2s
Show
Sleep 10s

# ── Scene 6: the reviewer, hired only once there was work to review ─────────
Hide
Ctrl+B
Sleep 300ms
Type "d"
Sleep 1s
# Two gates, strict then lenient. The reviewer existing is the fact this scene
# needs and is waited on forever; catching it mid-review is only the nicer frame,
# so that one gives up after a minute rather than failing a whole take over it.
Type `while :; do gang roster 2>/dev/null | grep -q '^reviewer' && break; sleep 2; done; for i in $(seq 1 30); do gang roster | grep '^reviewer' | grep -q busy && break; sleep 2; done; echo S6-"OK"`
Enter
Wait+Screen@600s /S6-OK/
Type "tmux select-window -t gangline:reviewer"
Enter
Sleep 500ms
Type "gang attach"
Enter
Sleep 2s
Show
Sleep 10s

# ── Scene 7: arc complete — everyone idle, the proof runs ────────────────────
# The gate asks every agent to BE idle, not merely to not be busy. Those are
# different questions and gang has five state words, not two: parked, occupied
# and expired all pass a not-busy test with nothing having finished. Asking the
# weaker one ends the take on the lead still parked in `gang wait reviewer`
# while the reviewer beside it already reads idle — a closing frame that looks
# like a deadlock, two seconds before the wait would have returned, because idle
# has to hold for two consecutive polls before `gang wait` believes it.
#
# Colour is off down a pipe (gang gates it on -t 1), so the state really is the
# third field and does not have escapes wrapped around it.
Hide
Ctrl+B
Sleep 300ms
Type "d"
Sleep 1s
Type `while :; do gang roster 2>/dev/null | awk 'NF { n++; if ($3 == "idle") i++ } END { exit !(n && n == i) }' && { clear; echo S7-"OK"; break; }; sleep 2; done`
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
Sleep 6.5s
Type "python3 hello.py"
Sleep 500ms
Enter
Wait+Screen@15s /Hello, world!/
Sleep 5s
