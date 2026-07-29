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
| **harness** | what a dog wears; the line clips to it, never surgery inside the dog | the CLI agent (Claude Code, Codex, opencode, Pi); gang clips on at the tty |
| **dog** | pulls | an agent — a named tmux window |
| **hitch** | clip a dog into the gangline, harness by harness | `gang hitch` — start an agent as a named window (alias: `spawn`) |
| **vet check** | at every checkpoint a veterinarian examines each dog before it runs on | `gang vet` — the strategy-rot check: each profile's pinned versions against the installed harness, and with `--probe`, its markers against a live pane (alias: `doctor`) |
| **lead** | runs front, picks the line between the musher's commands | the lead role: holds the plan, splits the work, steers |
| **swing** | runs just behind lead, swings the team true through the turns | the reviewer role: catches the drift before it becomes the direction |
| **team dog** | the engine — the pulling power in the middle of the line | the worker role: does the work, reports back |
| **tight tug / slack tug** | a pulling dog keeps its tugline taut; a coasting one lets it sag | busy / idle |
| **hook set** | the snow hook dug in; the team stands anchored until the musher pulls it | gated — a modal owns the input box; Gangline sends are refused rather than pasted into a dialog where a keystroke would answer it |
| **line out** | the leader holds the gangline taut while the team hooks up | a harness whose input box is up and taking keystrokes — ready to be briefed |
| **checkpoint** | where teams rest, resupply, and leave dogs with handlers | the clean seam an arc ends on — tests green, a commit made, a question answered; where compaction and releases happen |
| **dropped** | left at a checkpoint with the handlers — deliberate and routine | a released window (`gang drop`); finishing an arc and being released is health, not failure |
| **dog in the basket** | a dog riding in the sled bag instead of pulling | a finished agent left running instead of released — the anti-pattern *dropped* exists to prevent |
| **tangle** | lines crossed; the whole team stands while the musher walks up to clear it | two agents in one file — everyone's work stops for the untangling |
| **on by** | pass the distraction without breaking stride | a side-issue you notice is a line in your report, not a detour |
| **intelligent disobedience** | a good lead refuses the command onto thin ice; it can stop or steer the team, never choose the destination | say so once — short, specific — when an instruction looks like it breaks something the musher cannot see; their word is then final, and your own judgment is never authorization |

If the metaphor stops carrying weight, delete this file and rename nothing:
every verb was chosen to read literally without it — hitch is attach, drop is
release, vet is examine — and every rule already does. The generic aliases
(`spawn`, `doctor`) answer regardless.

(And since you asked: real mushers say *hike*, almost never *mush*. Of the 1925
serum run's lead dogs, Togo ran the long, dangerous leg while Balto got the
statue — credit your working dogs.)
