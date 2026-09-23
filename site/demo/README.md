# Recording the demo

`record.sh` starts a disposable Claude Code and Codex team on its own tmux
socket. `demo.tape` records
the native panes while the guide delegates a Python task, the builder writes
and runs it, and the reply arrives. The terminal remains visible throughout
that exchange. The tape sets the canvas, font, frame rate, typing speed and
playback speed. Vertical panes keep the terminal text legible on phones.

Run from a trusted linked worktree with `gang`, `tmux`, both authenticated
harnesses, VHS, Chromium, ttyd and FFmpeg on PATH:

```sh
DEMO_STATE="$HOME/.local/state/gangline-demo-$(date +%s)" site/demo/record.sh
```

The state path must be new. Preserve and remove a previous `greet.py` before
recording again. The script keeps the event log and native pane scrollback in
the state directory and drops only its own team. It does not replace the
installed Gangline binary or change saved harness settings. The demo launches
Codex with full sandbox access so Gangline can
observe the peer harness process when delivering its reply. Run it only in
the trusted demo worktree. A permission prompt needs the operator; a failed
take is not a recording.

The tape starts after both harnesses launch. Its keyboard request is ordinary
operator input. All teammate envelopes come from `gang send` in registered
agent panes; never add a sender name with `--from`.

Review the captured event log and scrollback before publishing. Confirm both
message deliveries and the program output. `verify.py` requires both registered senders and native delivery receipts
before the script installs the assets and writes `site/demo.txt` from the
recorded messages. Inspect the entire video for private material, cropped text,
permission prompts and missing work. The native terminal appearance is kept
in both site themes. GitHub Pages publishes the generated video, GIF and poster
with cache-versioned URLs.

Use `ffprobe -show_entries format=duration site/demo.mp4` to measure playback
length. Use FFmpeg's `framemd5` output to count distinct decoded frames. Review
live browser playback at desktop and phone widths after deployment, and keep
browser screenshots in a local directory.
