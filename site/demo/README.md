# Demo source

The player is an edited replay of real Claude Code and Codex session text,
reflowed to fit the site's narrow content column and phones. It shows the
handoff, the work, and the reply in sequence. It is not a screen recording;
the player and caption disclose the reflowed excerpts.

`session.json` contains the displayed excerpts. `receipts.json` contains the
corresponding recorded Gangline send and native delivery events. The renderer
refuses an envelope without a registered sender and matching successful
delivery, or if its displayed message differs from the recorded message.

To replace the session, run a throwaway team on a private tmux socket as
specified in `AGENTS.md`. Give its guide and builder a small real task, capture
their native panes, and keep the matching events from the team's log. Never
invent an envelope or remove its sender qualification. Copy complete messages
and native tool output into the source files; trim between excerpts, not within
a message. Drop the throwaway team after preserving the evidence.

Install Playwright in a scratch directory outside the repository, and use the
command at the top of `render.mjs` to render. It needs Chromium and FFmpeg. The
renderer checks text bounds before writing the themed videos, GIFs, posters,
and plain-text transcript. Review each frame at the actual player size.

GitHub Pages publishes the generated assets through `.github/workflows/pages.yml`.
The workflow adds a content hash to the video, poster, and player-script URLs.
