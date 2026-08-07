# User configuration, operator doctrine, and hitch discipline — implementation spec

Four coupled changes: a user configuration file behind the `GANG_*` environment
surface, an operator doctrine slot appended to the startup contract, a
hitch-discipline line in that contract, and deferred self-compaction for the
claude-code profile.

Every claim about current behaviour below was read out of the tree at
`da6541a` and is cited by owning symbol with the line quoted. Where a claim is
about an external tool it was measured against that tool, not recalled.

## 1. User configuration file

### 1.1 The question the operator asked

*"There must be a lib for this right?"* — surveyed below, and the honest answer
is that the library saves less than it costs here.

**(a) `git config -f ~/.config/gangline/config`, reusing git's INI parser.**

Measured against `git version 2.34.1`:

| probe | result |
|---|---|
| `git config -f c.ini --list` outside any repository | works, prints `section.key=value` |
| key case | folded to lower case on output; values keep their case |
| key `boot_timeout` | `fatal: bad config line 2` — **underscore is not a legal key character** |
| duplicated key, `--get` | returns the **last** value, exit 0, no warning |
| value `/tmp/a b # c` | returns `/tmp/a b` — **`#` silently truncates** an unquoted value |
| valueless key `profiles` | `--list` prints the key with no `=`; `--get` prints empty, exit 0 |
| missing file | `fatal: unable to read config file`, exit 128 |
| malformed file | `fatal: bad config line 1 in file bad.ini`, exit 128 |
| `[include] path = other.ini` under `-f` | the include was **not** expanded; only `include.path` was listed |
| user's global gitconfig | does not leak into a `-f` read |

Three findings decide it:

1. **`git` is not a runtime dependency of `bin/gang` today.** `grep -n '\bgit\b'
   bin/gang` returns nothing; the only git in the tree is in `install.sh`
   (`need git`, `git clone`). Adopting git's parser makes every `gang status`,
   every `gang send`, and every `gang hook` — which runs on every
   `UserPromptSubmit` and `PostToolUse` of every agent — depend on a binary
   Gangline does not otherwise need. `bin/gang` does hard-require `python3`, but
   it requires it **at the point of use**, inside `cmd_hitch`: `command -v
   python3 >/dev/null 2>&1 || die "python3 is required for native hook payloads
   and context lights"`. A config read has no such point of use; it is on every
   path.
2. **Key names cannot mirror the variable names.** `_` is refused, so
   `GANG_BOOT_TIMEOUT` must become `gang.boot-timeout` and the implementation
   must carry a hand-maintained name-mapping table. That table is the code the
   free parser was supposed to save.
3. **The parse still has to be policed.** Silent last-wins on a duplicate and
   silent truncation at an unquoted `#` are exactly the silent fallbacks
   `CONSTITUTION.md` law 8 forbids, so the implementation must enumerate
   `--list`, count duplicates, and reject unknown keys itself. That is the same
   allowlist loop option (b) needs. git's parser hands back a parse that must
   then be audited; it does not hand back a decision.

The `[include]` question is closed independently of git's version: an unknown
key is fatal under §1.5, and `include.path` is an unknown key.

**(b) A strictly parsed `KEY=VALUE` file — parse, never source.** No new
dependency. The file uses the **exact variable names** the operator already
types on a command line, so `GANG_BOOT_TIMEOUT=45 gang hitch …` and the config
line `GANG_BOOT_TIMEOUT=45` are the same string; nothing to learn, nothing to
map. The parser is small because the format is deliberately trivial — no
sections, no types, no arrays, no includes, no escapes. The 200-line parsers in
the wild exist for the features Gangline does not want.

**(c) A real parser dependency (`yq`, a TOML reader).** A new runtime binary on
every invocation, absent from base systems, for a file with a dozen flat scalar
keys. Fails law 5 and the zero-new-runtime-deps rule outright. Rejected.

**Recommendation: (b).** Sourcing is not among the options at all: a sourced
file executes arbitrary code on every `gang` invocation, including `gang hook`
inside every agent's turn, from a file an operator may have synced from another
machine.

### 1.2 Location

```
${XDG_CONFIG_HOME:-$HOME/.config}/gangline/config
```

The same directory the doctrine slot uses, so an operator has one Gangline
directory. Absent file means no config layer, silently — absence is the default
state, not a failure. There is no `GANG_CONFIG` override: `XDG_CONFIG_HOME` is
the standard mechanism and is what the test suite uses to isolate itself.

### 1.3 Precedence

Environment variable > config file > built-in default. Environment stays
authoritative so CI, one-off invocations, and the `env` prefix Gangline itself
writes into launch lines all keep working unchanged.

"Set in the environment" means **set**, not non-empty: `GANG_PROFILES=` from the
environment is an operator deliberately clearing it and outranks the file.

### 1.4 Settable keys

Exactly these, with the defaults they have today:

| key | default | where the default lives now |
|---|---|---|
| `GANG_PROFILE` | `claude-code` | `cmd_hitch`: `prof="${GANG_PROFILE:-claude-code}"` |
| `GANG_SESSION` | `gangline` | `SESSION="${GANG_SESSION:-$DEFAULT_SESSION}"` |
| `GANG_PROFILES` | unset | `profile_file`, `all_profiles` |
| `GANG_LOCK_DIR` | `/tmp/gangline-$(id -u)` | `lock_pane`, `spool_root_of` |
| `GANG_CONTEXT_LIGHTS` | `off` | `GANG_CONTEXT_LIGHTS="${GANG_CONTEXT_LIGHTS:-off}"` |
| `GANG_BOOT_TIMEOUT` | `30` | three `boot="${GANG_BOOT_TIMEOUT:-30}"` sites |
| `GANG_CHURN_WAIT` | `0.5` | top-of-file default block |
| `GANG_ACTIVITY_WINDOW` | `5` | top-of-file default block |
| `GANG_ACTIVITY_LIMIT` | `300` | top-of-file default block |
| `GANG_TURN_LIMIT` | `300` | top-of-file default block |
| `GANG_OCCUPIED_LIMIT` | `900` | top-of-file default block |
| `GANG_CLEAR_PRESSES` | `40` | `presses="${GANG_CLEAR_PRESSES:-40}"` |

**Refused with their own message, not as unknown keys:**

- **Profile-contract declarations** — `GANG_LAUNCH`, `GANG_RESUME_LAUNCH`,
  `GANG_MODEL_OPT`, `GANG_EFFORT_OPT`, `GANG_EFFORT_CMD`, `GANG_BUSY_REGEX`,
  `GANG_OCCUPIED_REGEX`, `GANG_QUEUED_REGEX`, `GANG_QUEUE_RECALL_KEY`,
  `GANG_INTERRUPT_KEY`, `GANG_STOP_HOOK`, `GANG_QUIET_AT_REST`,
  `GANG_MIDTURN_INPUT`, `GANG_COMPACT_CMD`, `GANG_SELF_COMPACT`,
  `GANG_SESSION_KEY`. `load_profile` clears each of these to `""` and then
  sources the profile, so a config value would be discarded without a trace.
  The refusal names where the value belongs: *"`GANG_BUSY_REGEX` is a profile
  declaration, not operator configuration — `load_profile` clears it before
  sourcing the profile, so a value here would be discarded. Put it in a custom
  profile and point `GANG_PROFILES` at its directory."*
- **`GANG_TEST_PROFILES`** — a per-invocation switch for Gangline's own suite.
  Persisting it would make the `bash` stand-in permanently hitchable, which
  `cmd_hitch` refuses on purpose (*"'bash' is a stand-in for gangline's own
  suite, not a harness to run an agent on"*). Refused as *"a per-invocation
  switch for gangline's own suite, not operator configuration"*.
- **`GANG_MODEL`, `GANG_PROFILE_FILE`** — internal. `GANG_MODEL` is set for the
  lifetime of one `sh -c` in `effort_levels`; `GANG_PROFILE_FILE` is stamped by
  `load_profile`. Refused as internal.

Anything else is an unknown key.

### 1.5 File grammar and failure semantics

Read line by line. Never sourced, never `eval`'d as text, no `$` expansion, no
quote removal, no escape processing.

- A line that is empty or all blanks is skipped.
- A line whose first non-blank character is `#` is a comment. **`#` starts a
  comment only at the start of a line**; inside a value it is a literal.
- Otherwise the line must be `NAME=VALUE`:
  - leading blanks on the line are stripped;
  - `NAME` is everything up to the first `=`, and must contain no blanks —
    `GANG_SESSION = x` is refused, because this file's syntax is the shell
    assignment syntax the operator already uses, and accepting a second dialect
    is how a format grows;
  - `VALUE` is everything after the first `=`, taken literally, with **trailing
    blanks stripped**. No Gangline value has a meaningful trailing space, and an
    invisible one is the misconfiguration fail-loud cannot catch: it would
    surface as *"unknown profile 'codex '"*, blaming the wrong thing.
- An **empty value** (`GANG_PROFILES=`) is refused: *"a key with no value states
  nothing — delete the line to take the default."* Allowing it would let
  `gang config` report a config layer whose effective value came from a `:-`
  default anyway.
- A **duplicated key** is refused, naming both line numbers. Last-wins is a
  silent fallback.
- An **unknown key** is refused, naming the file, the line number, the key, and
  the full settable set.
- A file that exists but is not a regular file, or cannot be read, is fatal.

**Unknown keys are fatal, everywhere, including `gang hook`.** The cost is
real: a config carrying a key from a newer `gang` makes an older `gang`
unusable until the file is edited, and a malformed config surfaces as a hook
error on every turn of every agent. That is the correct direction. The
alternative — warn and continue — means an operator who typed
`GANG_BOOT_TIMOUT=600` believes a bound is set that is not, and finds out when
a hitch times out at 30 seconds with nothing on screen connecting the two. The
precedent in `bin/gang` is unambiguous: an unknown `GANG_SELF_COMPACT` value
dies (*"profile '$1' sets unknown GANG_SELF_COMPACT"*), an unknown
`GANG_STOP_HOOK` value dies, a key name outside the tmux key alphabet dies, an
unknown `hitch` argument dies. A hook that quietly ran on defaults while `gang
status` refused would be the split-brain law 8 exists to forbid. The error
carries the file, the line, and the fix, so it is one edit.

### 1.6 Implementation

Ordering in `bin/gang`: `config_load` is called **after** `fail`/`die` are
defined and **before** the first use of any settable variable. Concretely,
`DEFAULT_SESSION` / `SESSION=` and the top-of-file default block currently
sitting above and below `gang_root` both move below the `config_load` call. No
read site changes: every consumer already reads through `${X:-default}` or
through the default block, and `config_load` only assigns the variable.

```sh
GANG_CONFIG_KEYS='GANG_PROFILE GANG_SESSION GANG_PROFILES GANG_LOCK_DIR
GANG_CONTEXT_LIGHTS GANG_BOOT_TIMEOUT GANG_CHURN_WAIT GANG_ACTIVITY_WINDOW
GANG_ACTIVITY_LIMIT GANG_TURN_LIMIT GANG_OCCUPIED_LIMIT GANG_CLEAR_PRESSES'
```

`CONFIG_ORIGINS` accumulates one record per key, tab-separated, newline-joined:
`NAME<TAB>env`, `NAME<TAB>env<TAB><lineno>` (environment set and the file also
named it), or `NAME<TAB>config<TAB><lineno>`. A key with no record came from its
built-in default. bash-3.2 has no associative arrays; this string plus a
`config_origin NAME` reader is the substitute, and the whole set is a dozen
entries.

Two idioms are load-bearing and both avoid indirect expansion, so they hold on
bash 3.2 (macOS `/bin/bash`) without relying on `${!name+word}`:

```sh
eval "envset=\${$k+set}"    # was this key set in the environment at all
eval "$k=\$value"           # assign; the VALUE is a quoted parameter reference,
                            # never interpolated text, so no content can inject
```

`$k` is drawn only from `GANG_CONFIG_KEYS`, a literal in this script. **The
assignment must never be written `eval "$k='$value'"`** — that form is
injectable by a value containing a quote, and it is the single line in this
change a reviewer should check first.

Values are assigned as ordinary shell variables and **not exported**. Exporting
would change the environment of every command `gang` runs — the harness launch
line, the profile's `GANG_EFFORT_CMD` producer — giving the config layer reach
the environment layer only has when an operator asked for it, and making
"environment beats config" untestable from inside a child process.

Reading the file must tolerate a missing final newline:

```sh
while IFS= read -r line || [ -n "$line" ]; do …; done < "$file"
```

### 1.7 Propagation to hitched agents

`cmd_hitch` builds `launch_env` and prefixes the launch line with it:

```sh
launch_env="GANG_SESSION=$(shell_quote "$SESSION")"
[ -z "${GANG_PROFILES:-}" ] || launch_env="$launch_env GANG_PROFILES=$(shell_quote "$GANG_PROFILES")"
[ -z "${GANG_LOCK_DIR:-}" ] || launch_env="$launch_env GANG_LOCK_DIR=$(shell_quote "$GANG_LOCK_DIR")"
```

This is unchanged and needs no edit: `SESSION` is already the resolved value,
and the two guards are shell-variable tests that see a config-assigned value the
same as an environment one. The effect is deliberate and should be documented —
the **resolved** values are pinned into the child as environment, whatever layer
produced them, so the child's top layer agrees with the parent's answer. That
matters because `tmux new-window` runs its command in the tmux **server's**
environment, so a child's `HOME`/`XDG_CONFIG_HOME` are not guaranteed to be the
caller's, and re-resolving from a file could disagree. `docs/reference.md`
already requires that *"every process addressing one team must agree on
`GANG_SESSION`, `GANG_PROFILES`, and `GANG_LOCK_DIR`"*; pinning is how that stays
true across the config layer.

No other key is propagated. Adding propagation for `GANG_BOOT_TIMEOUT` and the
evidence bounds has no consumer today (law 5).

### 1.8 Blaming the right layer

With three layers, *"GANG_BOOT_TIMEOUT must be a whole number of seconds, got
'abc'"* no longer says where to go and fix it. A helper appends the origin:

```sh
config_origin_note GANG_BOOT_TIMEOUT
#  -> " (from ~/.config/gangline/config line 4)"
#  -> " (from the environment)"
#  -> ""   when the value is the built-in default
```

Applied at exactly these sites, and no others:

- `require_whole` and `require_interval` — both already take *"the name to
  blame"* as `$1`, so the note is one append in each and covers
  `GANG_BOOT_TIMEOUT`, `GANG_CHURN_WAIT`, `GANG_ACTIVITY_WINDOW`,
  `GANG_ACTIVITY_LIMIT`, `GANG_TURN_LIMIT`, `GANG_OCCUPIED_LIMIT`, and
  `GANG_CLEAR_PRESSES` at every call site.
- The two `context_lights_parse` refusals.
- `cmd_hitch`'s unknown-profile path, **only when `-p` was not given** — when
  `-p` was given the value came from the command line and the note would lie.
  `load_profile` itself is called with `-p` values, adopted-profile names, and
  roster rows, so the note belongs in `cmd_hitch` beside `prof="${GANG_PROFILE:-…}"`,
  not inside `load_profile`.

Paths are printed with `$HOME` collapsed to `~`.

### 1.9 `gang config`

It earns its place, and the argument is that it stops a silent fallback rather
than that it is convenient. Three layers with no introspection means an operator
seeing Gangline address the wrong session cannot tell whether it came from the
environment, the file, or a default — and the single most common failure of a
new config file is *"my config file isn't working"*, whose answer is almost
always an environment variable outranking it. This is also the only place the
doctrine slot's state is discoverable before a hitch delivers it.

`gang config` takes no arguments, needs no tmux server, and prints one line per
settable key plus the doctrine line:

```
GANG_PROFILE=codex                      environment (overriding config line 2)
GANG_SESSION=arc-cfg                    ~/.config/gangline/config line 3
GANG_PROFILES=                          default
GANG_BOOT_TIMEOUT=30                    default
…
doctrine  ~/.config/gangline/DOCTRINE.md  present
```

`doctrine … absent` when the file is not there. Values print raw; this is a
human-readable report in the shape `gang profiles` and `gang roster` already
use. Wire it into `usage()` and the dispatcher (`config) cmd_config "$@" ;;`),
and replace the `Environment:` block in `usage()` with a pointer to `gang
config` and `docs/reference.md` — that block already omits half the variables.

If one item in this spec has to be cut for scope, this is it; everything else
is load-bearing.

## 2. Operator doctrine injection

### 2.1 The slot

```
${XDG_CONFIG_HOME:-$HOME/.config}/gangline/DOCTRINE.md
```

Core ships no doctrine and no example doctrine. Nothing in the repository
creates this file. Its deletion path (law 6) is that the operator deletes the
file; Gangline never writes it, and a hitched agent's copy dies with the window.

### 2.2 Who gets it: the operator's own hitches, not the lead's

Doctrine binds delegation behaviour, so it belongs to the agent the operator
delegates *to*, not to the agents that agent delegates to in turn. But
`bin/gang` has no concept of a lead: `cmd_up` merely defaults the name to
`lead`, and `hitch lead` is a name like any other. Branching core on the literal
string `lead` would invent a role, and `AGENTS.md` is explicit that Gangline
*"does not coordinate work or supervise agents"*.

The distinction is already in the code, under a different name. `send_sender`
separates a call made from inside the team from one made from the operator's own
shell:

```sh
if self="$(self_window)"; then      # inside the team: identity is read off the window
…
else                                # outside: the operator's shell
```

`self_window` returns non-zero unless `$TMUX_PANE` is set **and** that pane is
listed in `$SESSION`. So:

> **A hitch made from outside the team carries the operator's doctrine. A hitch
> made from inside it does not.**

This is mechanism-level, harness-neutral, invents no role, and needs no flag.
The operator's own hitches — `gang up`, `gang hitch lead`, a second lead for a
parallel arc — carry doctrine. A lead hitching a teammate does not pass the
operator's delegation doctrine down; the lead writes that teammate's charter
itself. `gang adopt` never carries doctrine, because *"adoption does not inject
startup text"* at all and that stays true.

The accepted edge: a lead hitching a sub-lead gets no doctrine, and must carry
it forward in prose. The conservative direction — fewer injections of a document
the recipient may not need — is the right one to be wrong in.

The helper is one line and is named for what it means:

```sh
hitch_from_operator() { ! self_window >/dev/null 2>&1; }
```

### 2.3 Guards before the window opens

All three checks run **before** `tmux new-session`/`new-window`, so a refusal
leaves no window behind.

1. **Not a regular file, or unreadable** → fatal, naming the path. Absence is
   silent; presence Gangline cannot read is not.
2. **Size ceiling: 32768 bytes**, measured with `wc -c`. The guard exists to
   catch a category error — the slot pointed at a document tree, a log, or a
   binary — not to tune paste performance. Prose a person wrote to bind an
   agent's behaviour does not reach 32 KiB; a mistake reaches it immediately.
   Wrong in the small direction it refuses a legitimate doctrine loudly, with
   the ceiling in the message and a one-line fix; wrong in the large direction
   it lets an unbounded paste into a composer whose delivery verification reads
   a fixed pane region, where the failure is an unverified startup contract and
   a window to drop.
3. **Text only.** NUL bytes are detected by comparing `wc -c < "$file"` against
   `LC_ALL=C tr -d '\000' < "$file" | wc -c` — command substitution drops NULs,
   so the read would silently lose bytes. Remaining control characters are
   caught with the idiom `bin/gang` already uses for `$ROOT` in
   `profiles/claude-code.sh` (`case "$ROOT" in *[[:cntrl:]]*`), applied after
   deleting tab and newline:

   ```sh
   probe="$(printf '%s' "$doctrine" | tr -d '\11\12')"
   case "$probe" in *[[:cntrl:]]*) die … ;; esac
   ```

   `utf8_locale` runs at the top of `bin/gang`, so `[[:cntrl:]]` under a UTF-8
   locale does not match UTF-8 continuation bytes; the operator's own doctrine
   contains em dashes and must pass. Carriage returns are refused — a CRLF
   doctrine would paste stray CRs into a composer.

### 2.4 Escaping and quoting: there is no new hazard, and here is why

The delivery path is already binary-safe and already carries multi-line bodies.

- **No shell interpolation anywhere on the path.** `inject` writes the body with
  `printf '%s' "$2" | tmux load-buffer -b "$buf" -` and pastes with
  `tmux paste-buffer -p -d -b "$buf" -t "$1"`. Nothing is eval'd, nothing
  reaches a command line. Read the doctrine with `doctrine="$(cat "$file")"` —
  command substitution never interprets content — and never with `.` or `eval`.
- **Tag-shaped text is already neutralised.** `envelope` rewrites bracketed
  `gang:` openers over the *whole* body before wrapping it, so a doctrine
  containing `[gang: …]` cannot forge an envelope:
  `sed -E 's,(\[|［|〔|【)([[:space:]]*/?[[:space:]]*)([Gg][Aa][Nn][Gg])([[:space:]]*:),(\2\3\4,g'`.
  The doctrine must therefore be concatenated into the brief **before**
  `envelope` is called, not appended after — assembling the brief first is what
  puts it under that guard.
- **Multi-line is a proven path, not a new one.** `test/integration.sh` sends a
  two-line body through the same `inject` and asserts the recorded composer
  contains the tail line: `contains "and every line of it is recorded, not just
  the first" … "MARK_MULTI_TAIL"`. The startup contract has been single-line
  only by accident of its content, not by any property of the delivery.

### 2.5 The assembled contract

`startup_brief` gains the doctrine and the §3 line, and keeps `End this turn.`
**last** — it is the terminator, and anything after it reads as further
instruction:

```
You are <name> in Gangline. Peer messages name their sender in the gang
envelope. To message a peer: gang send --to NAME --stdin. At natural
checkpoints compact with: gang compact <name>. <hitch-discipline line, §3>

Operator doctrine (~/.config/gangline/DOCTRINE.md):

<doctrine, verbatim>

End this turn.
```

The origin line is named so the agent can tell operator doctrine from Gangline
mechanism and can quote its source. With no doctrine file, the contract is
today's text plus the §3 line, with no separator and no blank lines.

`startup_brief` takes the agent name today; it gains a second argument for the
doctrine body (empty when there is none) so the caller decides, keeping
`hitch_from_operator` out of the string builder.

Existing assertions survive unedited: `contains … "$(pane alpha)" "You are alpha
in Gangline"` and `contains … "$(pane alpha)" "End this turn."` both still hold.

## 3. Hitch discipline in the startup contract

One line, in **every** hitch's contract, not only the operator's. It costs one
sentence, it is true for anyone who runs `gang hitch`, and scoping it would
deny it to exactly the sub-lead case §2.2 already leaves without doctrine. The
split is principled: **Gangline's own line is universal; the operator's
doctrine is scoped to the operator's own hitches.**

> When you hitch a teammate, choose its model and reasoning effort deliberately
> and pass them as `-m` and `-e`; where the profile declares no such option gang
> refuses the flag, so that choice is the profile's rather than a default's.
> Never let an unexamined default stand in for a decision.

It enforces that a choice is made, never which choice — no model names, no
effort levels, no tiering policy. That belongs in DOCTRINE.md, and the
operator's file carries it.

This belongs in core because it is the prose form of a refusal core already
implements. `cmd_hitch` refuses an effort it cannot validate, and says why:

> *"An unchecked effort is not refused by the harness either — it runs at a
> level nobody chose — so it is refused here"*

with the reasoning recorded above it: *"the operator gets a live agent at an
effort nobody chose, or one that dies as soon as it is briefed."* Gangline
already spends code to stop an unchosen effort reaching a window. Telling the
agent that does the hitching to choose is the same decision, one layer up, for
free.

## 4. `GANG_SELF_COMPACT=deferred` for claude-code

### 4.1 The mechanism, read out of the source

`cmd_compact` dispatches on two facts — whether the caller is the target, and
whether the profile defers:

```sh
if [ "$mine" -eq 1 ] && [ "$GANG_SELF_COMPACT" = deferred ]; then
  self_compact_request "$AGENT_ID" >/dev/null
  echo "self-compaction scheduled for the end of this turn"
  return 0
fi
if [ "$mine" -ne 1 ]; then
  busy "$AGENT_ID" || busy_rc=$?
  …
fi
inject "$AGENT_ID" "$GANG_COMPACT_CMD" head
```

`profiles/codex.sh` declares `GANG_SELF_COMPACT=deferred`;
`profiles/claude-code.sh` declares nothing, and `load_profile` clears the
variable to `""` before sourcing. So on claude-code, an agent compacting itself
takes `mine=1` with no deferral: the first branch is skipped, **the busy check
is skipped too** — it is guarded by `[ "$mine" -ne 1 ]` — and `inject` types
`/compact` into the agent's own composer while its own turn is running, which is
the only time an agent can call this.

claude-code parks input arriving mid-turn. The profile records the rendering:

> *"The queue strand (observed on 2.1.223) renders a queued body in the
> transcript styled exactly like a submitted prompt and empties the composer;
> the state is visible only in the composer itself"*

`inject`'s post-Enter verification catches the park and fails loudly, but the
`/compact` is now in the harness's queue, and `inject`'s **preflight** then
refuses every subsequent delivery to that window:

> *"the harness of $tname has parked earlier input in its own queue — its Enter
> is queueing rather than submitting, so nothing gang sends can enter the
> session until the queue is recovered"*

That is the trap: a lead that self-compacts becomes unreachable, and the
recorded recovery on 2026-08-07 was a drop and re-hitch.

### 4.2 The fix, and its preconditions

One line in `profiles/claude-code.sh`, with a comment in the profile's voice
recording why. Every precondition the deferred path needs is already satisfied:

| precondition | status |
|---|---|
| `GANG_COMPACT_CMD` declared | `GANG_COMPACT_CMD="/compact"` |
| `GANG_STOP_HOOK=1` | declared |
| a native Stop hook actually installed | the launch line composes `"Stop":[{"hooks":[…gang hook]}]` into `--settings`, **independently of `GANG_CONTEXT_LIGHTS`** — the lights branch only appends `statusLine` |
| Stop reaches the dispatcher | `cmd_hook`: `Stop) … self_compact_dispatch "$TMUX_PANE" "$prof"` |
| the value is accepted | `load_profile` accepts `''\|deferred` and dies on anything else |

After the fix: `gang compact <self>` records `@gl_self_compact_requested`, types
nothing, and returns. The agent's Stop hook fires,
`self_compact_dispatch` backgrounds `self_compact_after_stop`, which re-loads
the profile, waits for the composer with `wait_ready`, and injects `/compact`
into a box that is now free. `status` and `roster` already report pending and
failed self-compaction. Ordering against the spool drain is unchanged from
codex: `self_compact_dispatch` is backgrounded before `spool_stop`, and both
paths serialise on the pane lock inside `inject`.

**This fix has not landed and it is not wrong.** It is the smallest change in
this spec and the one with a recorded live failure behind it.

### 4.3 One pre-existing condition, noted and out of scope

`profiles/claude-code.sh` declares `GANG_STOP_HOOK=1` unconditionally, but the
hook JSON is only composed when `$ROOT` is set, executable, and free of quotes,
backslashes, and control characters — otherwise `GANG_LAUNCH` stays bare
`claude` with no hooks at all. In that case the declaration is a claim the
launch line does not honour, which already affects `--spool` and would now also
affect deferred self-compaction. It predates this work and is not fixed here.
Flagging it rather than silently widening scope.

## 5. Test plan

All behavioural. No test asserts that a file exists or greps a config value; each
asserts the artifact the behaviour produced — the value a command resolved, the
text on a pane, the option a command stamped, the message a refusal printed.

### 5.1 Suite isolation — mandatory, and a prerequisite for everything below

`test/integration.sh` must export a private config root beside its existing
`GANG_SESSION` / `GANG_LOCK_DIR` exports:

```sh
export XDG_CONFIG_HOME="$RUN_ROOT/xdg"
```

Without it every fixture hitch reads the developer's real
`~/.config/gangline/config` and injects their real `DOCTRINE.md`, and
`.githooks/pre-push` — which runs the suite from a `git archive` tree but not in
a clean environment — inherits the same leak. This is the first change the
implementer should make.

### 5.2 Configuration file (`test/integration.sh`, new section)

1. **The file supplies a value.** With no `GANG_SESSION` in the environment
   (`env -u GANG_SESSION`) and a config naming a session that exists on the
   private tmux server, `gang roster` lists that session's window. The artifact
   is the roster row, not the resolution.
2. **Environment outranks the file.** Environment and file name different
   existing sessions; `gang roster` lists the environment one's window.
3. **The file is never executed.** The config contains
   `GANG_SESSION=$(touch "$RUN_ROOT/executed")`. Assert the marker file does not
   exist **and** that `gang config` reports `GANG_SESSION` with that literal
   text as its value. The positive half is what keeps the negative half from
   passing on a fixture that did nothing — the rule in `CONTRIBUTING.md` that a
   negative assertion must not pass because its fixture produced no value.
4. **An unknown key refuses.** Non-zero exit, and the message names the file,
   the line number, and the key.
5. **A profile declaration refuses with its own message.** `GANG_BUSY_REGEX=x`
   in the file; the message names it as a profile declaration and points at
   `GANG_PROFILES`. Distinct from the unknown-key message, asserted by content.
6. **`GANG_TEST_PROFILES` refuses with its own message.**
7. **A duplicated key refuses**, naming both line numbers.
8. **An empty value refuses.**
9. **A value keeps `#`, spaces, and quotes literally.** `GANG_LOCK_DIR` set to a
   path containing a space and a `#`; assert `gang send` creates its lock under
   exactly that directory. This is where option (a)'s comment-truncation would
   have shown up, so the guard is worth having whichever parser lands.
10. **A trailing blank is stripped**, proven by a profile name with a trailing
    space in the file resolving to a working profile rather than an unknown one.
11. **A bad value blames the file.** `GANG_BOOT_TIMEOUT=abc` in the config; the
    refusal names the config file and its line number. Then the same value in
    the environment; the refusal says the environment. Two assertions, because a
    note that is always the same string is not attribution.
12. **`gang config` attributes each layer**: one key from the environment
    (marked as overriding the file's line), one from the file, one absent from
    both marked `default`.
13. **The resolved session is pinned into the launch line.** Hitch the `bash`
    fixture with `GANG_SESSION` coming from the file only, then read the child's
    environment out of its own pane and assert it carries the resolved session.

### 5.3 Doctrine (`test/integration.sh`, new section)

1. **Present and injected.** A doctrine fixture containing a marker sentence;
   hitch from outside the team; the pane carries both `You are <name> in
   Gangline` and the marker. Both halves asserted, so a pane that rendered
   nothing cannot pass.
2. **Absent.** No doctrine file; the pane carries the base contract and not the
   `Operator doctrine` origin line.
3. **A hitch from inside the team carries no doctrine.** Run `gang hitch` with
   `TMUX_PANE` set to an existing agent's pane — the idiom
   `test/integration.sh` already uses for in-team calls — and assert the new
   pane has the base contract and not the marker.
4. **Multi-line doctrine lands whole.** A doctrine whose first and last lines
   both carry distinct markers; assert **both** appear in the pane. A guard that
   reads only the first line is satisfied by a truncated paste.
5. **Tag-shaped doctrine is neutralised.** Doctrine containing `[gang:lead#x]`;
   assert the pane shows the neutralised `(gang:` form and not a second
   `[gang:` opener.
6. **Over-ceiling doctrine refuses before the window opens.** Exit non-zero, the
   message names the file and the ceiling, and **no window with that name
   exists** — the artifact that separates "refused" from "refused after
   launching".
7. **Non-text doctrine refuses**, one case with a NUL byte and one with a bare
   carriage return, each before the window opens.
8. **Unreadable doctrine refuses**; absent doctrine does not.
9. **`gang config` reports the doctrine present and absent.**

### 5.4 Hitch discipline

The hitched pane's contract contains the line, on both an operator hitch and an
in-team hitch, and `End this turn.` is still present — the existing assertion at
that site is not edited.

### 5.5 Deferred self-compaction

The claude-code declaration cannot be proven end to end in the mandatory suite:
that needs a real `claude` turn, and `CONTRIBUTING.md` puts real harness turns in
operator smoke tests, not the suite. Stating that plainly rather than dressing a
source read as behaviour:

1. **Mechanism guard (mandatory, behavioural).** Extend the existing deferred
   self-compaction fixture. Today it proves the deferred path dispatches at
   Stop. Add the half the defect lives in: with the fixture's pane **painting a
   busy marker**, `gang compact <self>` leaves the composer empty and stamps
   `@gl_self_compact_requested`. Then the same call on a fixture profile with
   no `GANG_SELF_COMPACT` types into the composer mid-turn. Two directions, so
   the guard is not green on both.
2. **Declaration parity (mandatory).** `test/integration.sh` already sources
   `profiles/codex.sh` in a subshell and asserts `GANG_SELF_COMPACT` is
   `deferred`. Extend that existing check to claude-code. It is a declaration
   check, not a behavioural one, and it is included only because it matches an
   assertion already in the suite — the behavioural weight is carried by (1).
3. **Operator smoke test (not mandatory, documented in the spec's landing
   notes).** In a separately named disposable session, hitch a real claude-code
   agent, have it run `gang compact <self>` mid-turn, and observe that its
   composer stays empty, that `gang status` reports the pending self-compaction,
   and that `/compact` is submitted after the turn ends. Never against the live
   `gangline` session or the development agent.

### 5.6 Lint

`test/lint.sh` needs no new rule. It globs `bin/gang` and `profiles/*.sh`
already, and every new fixture lives under `test/`, which it also globs. New
shell files carry an SPDX identifier.

## 6. Documentation at landing

- `docs/reference.md` — the `Environment` section becomes `Configuration`: the
  precedence rule, the file path and grammar, the settable set, what is refused
  and why, and `gang config` under *Discovery and hooks*. The startup-contract
  description under `gang hitch` gains the doctrine slot and its operator-hitch
  scoping. Keep the existing table; add the layer, do not restate the variables
  twice.
- `docs/DECISIONS.md` — three terse entries, in the house voice:
  - *Configuration is parsed, never sourced* — the file mirrors the environment
    names, the environment stays authoritative, an unknown key is fatal.
  - *Doctrine is the operator's, and rides the operator's own hitches* — core
    ships the slot and no content; inside-the-team hitches do not carry it.
  - *A hitch states its model and its effort* — the contract requires the
    choice, never the choice's content.
- `README.md` — unchanged. This adds no product claim.
- `CHANGELOG.md` — untouched; Release Please owns it.

Commits are Conventional and atomic — one logical change each: suite isolation,
the config loader, `gang config`, the doctrine slot, the discipline line, the
claude-code declaration, docs.
