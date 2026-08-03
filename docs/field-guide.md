# The Musher's Field Guide

Gangline's vocabulary is mushing. Every term below names something real in the
system, defined once, here. The metaphor names roles, status, and verbs — never
rules.
Rules live where rules live (`CONSTITUTION.md`, the role briefs), stated
literally; if a term here ever seems to disagree with one, the rule wins.

| Term | On the trail | On the team |
|------|--------------|-------------|
| **gangline** | the one line hitching many dogs, each in its own harness, to one sled and one musher | the tmux session hitching every agent, each in its own harness, to one operator |
| **musher** | drives the sled, picks the destination, gives the commands | the operator — the human the whole line runs back to and whose direction is final |
| **harness** | what a dog wears; the line clips to it, never surgery inside the dog | the CLI agent (Claude Code, Codex, opencode, Pi); Gangline clips on at the tty |
| **dog** | pulls | an agent — a named tmux window |
| **hitch** | clip a dog into the gangline, harness by harness | `gang hitch` — start an agent as a named window (alias: `spawn`) |
| **vet check** | at every checkpoint a veterinarian examines each dog before it runs on | `gang vet` — the strategy-rot check: each profile's version declarations against the installed harness (an unpinned profile fails), and with `--probe`, its markers against a live pane (alias: `doctor`) |
| **lead** | runs front, picks the line between the musher's commands | the lead role: holds the plan, splits the work, steers |
| **swing** | runs just behind lead, swings the team true through the turns | the reviewer role: catches the drift before it becomes the direction |
| **team dog** | the engine — the pulling power in the middle of the line | the worker role: does the work, reports back |
| **tight tug / slack tug** | a pulling dog keeps its tugline taut; a coasting one lets it sag | busy / idle |
| **hook set** | the snow hook dug in and the team standing anchored — whether the musher set it or it caught on its own | `occupied` — a harness-owned UI has the input box, so Gangline refuses sends rather than paste into a dialog where a keystroke would answer it. Gangline establishes that the box is taken, not who can free it, which is why the state qualifies itself `(authority unknown)` |
| **line out** | the leader holds the gangline taut while the team hooks up | a harness whose input box is up and taking keystrokes — ready to be briefed |
| **trail markers** | lath stakes and reflectors planted along the route before anyone runs it, read by every team going past and written by none of them; they keep a musher on route at the hour when nothing of the route itself is visible | the team's shared brief — where the work is going, who owns which files, the bounds that bind every dog. One dog owns it and the rest read it; ask its owner to change it rather than editing it. Point at a marker rather than copying it into your own notes: a copy goes stale in the file that copied it, silently, and travels further for carrying someone's name |
| **checkpoint** | where teams rest, resupply, and leave dogs with handlers | the clean seam an arc ends on — tests green, a commit made, a question answered; where compaction and releases happen |
| **cutoff** | the time a checkpoint closes — reach it late and you are withdrawn from the trail, however well the team is still running | the declared wall-clock instant (`gang cutoff`, `gang hitch --cutoff`) that closes the team's day — the *budget* is the span up to it, and that is the span time bands ring fractions of; the reserve at the end is room to make one last *checkpoint* before the cutoff arrives |
| **dropped** | left at a checkpoint with the handlers — deliberate and routine | a released window (`gang drop`); finishing an arc and being released is health, not failure |
| **dog in the basket** | a dog riding in the sled bag instead of pulling | a finished agent left running instead of released — the anti-pattern *dropped* exists to prevent |
| **tangle** | lines crossed; the whole team stands while the musher walks up to clear it | two agents in one file — everyone's work stops for the untangling |
| **on by** | pass the distraction without breaking stride | a side-issue you notice is a line in your report, not a detour |
| **intelligent disobedience** | a good lead refuses the command onto thin ice; it can stop or steer the team, never choose the destination | say so once — short, specific — when an instruction looks like it breaks something the musher cannot see; their word is then final, and your own judgment is never authorization |

If the metaphor stops carrying weight, delete this file and rename nothing:
every verb was chosen to read literally without it — hitch is attach, drop is
release, vet is examine — and every rule already does. The generic aliases
(`spawn`, `doctor`) answer regardless.

One word travels the other direction. **Dogfooding** is software's term, not
the trail's — run the thing you are building — and everywhere else it is a
borrowed idiom. This repo is where it lands literally: gangline is developed by
a gangline team, dogs hitched to the very line they are building. `gang` on a
contributor's `PATH` is a symlink into the working tree, so a saved edit is
under the team before it is committed — `CONTRIBUTING.md` explains why that is
the right behaviour and what it demands of checkpoints. And the pun is
load-bearing: a class of defect has a live team as its precondition, and no
fixture summons one on schedule. The catch recorded in commit 6a08458 fired
only because a real agent happened to be compacting while the suite ran. The
team is the test rig.

(And since you asked: real mushers say *hike*, almost never *mush*. Of the 1925
serum run's lead dogs, Togo ran the long, dangerous leg while Balto got the
statue — credit your working dogs.)
