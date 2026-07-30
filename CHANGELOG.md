# Changelog

## [0.2.0](https://github.com/adambiggs/gangline/compare/gangline-v0.1.0...gangline-v0.2.0) (2026-07-30)


### ⚠ BREAKING CHANGES

* **messaging:** gang send now requires --stdin for message bodies, and gang compact resumes now use --resume-stdin; positional message and --resume arguments are refused.

### Features

* **context:** derive the band ladder between absolute bounds ([09ff167](https://github.com/adambiggs/gangline/commit/09ff167f5089ba47080686e16b78d5fd75691bbd))
* **context:** make compliance evidence decidable ([afac6c2](https://github.com/adambiggs/gangline/commit/afac6c2c401f1d59fafb5cf01b039a448c6cf48c))
* **context:** record compliance evidence without inference ([02a898d](https://github.com/adambiggs/gangline/commit/02a898daef612d926f473a6a78bfb467750463da))
* **messaging:** read delivered prose from stdin ([73193db](https://github.com/adambiggs/gangline/commit/73193db8502d4637b8612c435578fde1fb541d35))


### Bug Fixes

* **context:** judge final-band exposure per note, not per drop row ([be8854f](https://github.com/adambiggs/gangline/commit/be8854f8f3116da7fd04edf2c9b4d39624c3dac9))
* **context:** make the warning ladder proportional so one default fits a mixed team ([c3ca002](https://github.com/adambiggs/gangline/commit/c3ca002431358ba41c7fb026cfd7a102a6c0f8b8))
* **context:** restore the absolute band ladder, and record the decision ([f033a53](https://github.com/adambiggs/gangline/commit/f033a534ecd43199ed42c3c2c910c6da2d3a7da4))
* distinguish uncertain agent states ([b151496](https://github.com/adambiggs/gangline/commit/b15149603b3a091f75d2bb64b846d02955250568)), closes [#16](https://github.com/adambiggs/gangline/issues/16) [#17](https://github.com/adambiggs/gangline/issues/17) [#22](https://github.com/adambiggs/gangline/issues/22)
* **install:** lower the tmux floor to 2.6, bounded by evidence ([8f292ad](https://github.com/adambiggs/gangline/commit/8f292adbe27d1987b2168c274f9deac8cccbf4e6))
* **messaging:** keep wait advice on stdin ([f3e8786](https://github.com/adambiggs/gangline/commit/f3e878688139a7cb0b29840982c8265c865978d8))
* parse churn sampling on bash 3.2 ([daa179c](https://github.com/adambiggs/gangline/commit/daa179c68a9f8a093263b3fa5d77a2b791bd8022))
* reserve substrate sender names ([43b5cb7](https://github.com/adambiggs/gangline/commit/43b5cb7f3bc1fbd194c050eac8860b23b893eec7))
* **send:** put the delivery lock somewhere every process can agree on ([bf511f7](https://github.com/adambiggs/gangline/commit/bf511f7d6a5044ad41c32b4be8e77c463c8855a7))
* **vet:** tear the probe down on every exit path, not only the clean one ([ab4bc3b](https://github.com/adambiggs/gangline/commit/ab4bc3b5c1ce92e2f3fde2aea11cfc19f48161f6))

## [0.1.0](https://github.com/adambiggs/gangline/compare/gangline-v0.0.1...gangline-v0.1.0) (2026-07-29)


### ⚠ BREAKING CHANGES

* install.sh fails without python3 instead of warning.
* **send:** the wire format is an envelope, not a `[gang:<sender>]` prefix. Anything matching on the old prefix — custom role briefs, scripts reading panes — needs updating. An agent inside the session can no longer send under another agent's name.
* hitch, drop, and vet — the metaphor verbs go primary
* the executable is now `gang`, environment variables are GANG_* (was GL_*), and the injected identity prefix is `[gang:<sender>]` (was `[gl:<sender>]`). Existing profiles, cron lines, and agent prompts referencing the old names must be updated.

### Features

* add a Codex profile ([48a2a74](https://github.com/adambiggs/gangline/commit/48a2a746b386edf267c277f2223dd1d387daa009))
* **adopt:** register existing tmux windows as agents ([984ae10](https://github.com/adambiggs/gangline/commit/984ae102bb2a6f4d9371ac9ad68bbaec695234ee))
* colour the console output ([50380a1](https://github.com/adambiggs/gangline/commit/50380a16db519bc648340d824a6a1ce449e43c70))
* **compact:** --resume queues a message that fires when compaction ends ([8c26389](https://github.com/adambiggs/gangline/commit/8c263898e8b0f663ee745c0aafcea21d04a1f164))
* **context:** gang context, roster column, and context-hook ([8fd11ba](https://github.com/adambiggs/gangline/commit/8fd11bab55b793fbafd752cd6abc28fa14622e71))
* **demo:** a reproducible recorder for the landing-page demo ([f5830f7](https://github.com/adambiggs/gangline/commit/f5830f7be58b4468cf63008a852ac56a77c6e40c))
* **down:** tear the whole team down ([6099e2f](https://github.com/adambiggs/gangline/commit/6099e2f865409203417b719bdfc75e79775ac6b1))
* file-based context for Codex — spawn-minted session markers, rollout parser, doctor format gate ([d588e46](https://github.com/adambiggs/gangline/commit/d588e46f36c78e70ee274bd65caea37d8052c17b))
* gated state — permission prompts read loud instead of idle ([1daacc6](https://github.com/adambiggs/gangline/commit/1daacc6f8186fe2ed68535cb01e825b4e641ae46))
* hand a resume to the compaction instead of waiting for quiet ([8702371](https://github.com/adambiggs/gangline/commit/870237199e5ef7963fdb7e35122fa34f6e7c6eb9))
* **hitch:** -m launches an agent on a specific model ([11a6b94](https://github.com/adambiggs/gangline/commit/11a6b940da565e40381292a17e4752208bb18f20))
* hitch, drop, and vet — the metaphor verbs go primary ([1f8e401](https://github.com/adambiggs/gangline/commit/1f8e401268ecfa64ac5bf7c7d1d2b80cc23d752a))
* hybrid metaphor surface — lead role, bracketed status words, harness order ([a719c3d](https://github.com/adambiggs/gangline/commit/a719c3d05422cfa8a35dfab4430aec846606f7bb))
* **install:** install with one command ([0a69a6a](https://github.com/adambiggs/gangline/commit/0a69a6a22fafccfb1e323d34cb4aaa83925044ec))
* opencode is a gangline harness ([331cdee](https://github.com/adambiggs/gangline/commit/331cdee2f0860980bb1f404cbe4d9a78c93b2414))
* patrol input-box guard and doctor strategy-rot detection ([8e4616b](https://github.com/adambiggs/gangline/commit/8e4616bad1232d086b478ca65acc140c5fd99a8e))
* **patrol:** harness-agnostic band-warning sweep ([9a598aa](https://github.com/adambiggs/gangline/commit/9a598aacaec641a2c9f4b93f168dfdf9f373e78e))
* **patrol:** hold nudges on a compaction gang issued itself ([51ef184](https://github.com/adambiggs/gangline/commit/51ef18478544a2cf4f1a88a8e05eba28710c1dbd))
* **patrol:** stash-wrap injection over claude code drafts ([8949e45](https://github.com/adambiggs/gangline/commit/8949e450594a5378b311d54dec7c5143876f0902))
* pi profile, compact primitive, shared verified-injection path ([e4eea1f](https://github.com/adambiggs/gangline/commit/e4eea1f66442d1708f7f3fa6fa4c011a63f031da))
* **pi:** verify live input and compaction states ([bf888ac](https://github.com/adambiggs/gangline/commit/bf888accb0255e0bcd521c4fd967d5a650d02b97))
* **profiles:** gate on modal chrome, not on a list of dialog sentences ([f1f9516](https://github.com/adambiggs/gangline/commit/f1f9516505b8a79ed90a9d0cd9e4ba85855d4033))
* **profiles:** resolve harnesses from GANG_PROFILES first ([4a705a9](https://github.com/adambiggs/gangline/commit/4a705a94a4943d637885efef9684c0dabfdb7e0d))
* **roles:** ship role briefs and hand one to an agent at spawn ([f424ee8](https://github.com/adambiggs/gangline/commit/f424ee8d301a3a71ca2c14bc25fb74816b653db9))
* **site:** publish gangline.ai ([087eede](https://github.com/adambiggs/gangline/commit/087eede253dc330a4989215646c0eea821d199cf))
* **site:** retake the demo — gang up, room to read, hiring in order ([0b69867](https://github.com/adambiggs/gangline/commit/0b698673ed656dea08ee99c37cb04085e8976405))
* **site:** retake the demo on the current gang ([31aad6f](https://github.com/adambiggs/gangline/commit/31aad6fcbeb6a3db65efd853690d6cd2a208add5))
* **site:** retake the demo with both agents reporting back ([9383c9e](https://github.com/adambiggs/gangline/commit/9383c9ec3ac392bf01484e1811fb43327154863b))
* **site:** show the demo on the landing page ([ccd95dd](https://github.com/adambiggs/gangline/commit/ccd95ddfaaaedcc9d2c8b3f798ba93413cb67a4b))
* tmux-substrate core, constitution, ADR-0001, claude-code + bash profiles ([ec0b324](https://github.com/adambiggs/gangline/commit/ec0b32475bf12ebc0aed4f2399e4f11cfa0de353))
* **up:** fresh-session bootstrap (spawn + attach) ([cb07034](https://github.com/adambiggs/gangline/commit/cb070349cfa5eb42c044732beef9cd605a60fe9e))
* **up:** launch with no arguments, and brief agents that are still booting ([37f11f0](https://github.com/adambiggs/gangline/commit/37f11f0b332eb1eb9c6bf51f6c274a9b441af585))
* **vet:** drive the harness instead of comparing version strings ([a6e7389](https://github.com/adambiggs/gangline/commit/a6e7389056628adeeccf5e1917a053ebb5b0ebe1))


### Bug Fixes

* address a session by its whole name, and take a window's id from its birth ([2ae5efe](https://github.com/adambiggs/gangline/commit/2ae5efe7ceb2f2e8feaccf41eeeb07b3451c8a33))
* **ci:** give the macOS cell a timeout without handing it a GNU userland ([88fe6cc](https://github.com/adambiggs/gangline/commit/88fe6cccb68963299333db2f009bac1ec67db847))
* **ci:** make the Versions step prove what it claims, and fail on an unknown runner ([c021ce8](https://github.com/adambiggs/gangline/commit/c021ce84ee3886d3998cfa7e695b27059ca93424))
* **ci:** re-check the PR title when the title is what changed ([da6c50e](https://github.com/adambiggs/gangline/commit/da6c50eec8cccce0626e2203c093a7ed3fe592ea))
* **claude-code:** drop a busy alternate the TUI stopped painting ([d9bbc64](https://github.com/adambiggs/gangline/commit/d9bbc64a816268d1fdc23d95af80da8cd2d5f863))
* **claude-code:** stop declaring a compaction marker that never fires ([cd9b0cc](https://github.com/adambiggs/gangline/commit/cd9b0ccecf36d57e407f4b0f40f8aadbd64056f7))
* **codex:** network access is necessary and not sufficient for gang send ([6a0d36e](https://github.com/adambiggs/gangline/commit/6a0d36e8deabc69424075171e1ae798be1ee8eaa))
* **compaction:** define the grace once and say what moving it costs at both ends ([68703d0](https://github.com/adambiggs/gangline/commit/68703d01f0ef970f1bdf82d531fde62883155c31))
* **context:** make every default band rung absolute ([713f932](https://github.com/adambiggs/gangline/commit/713f9326bb5717eeabe5c0c82deeabdb3c952fb4))
* **delivery:** stop leaving an undelivered paste in somebody's input box ([5c9a00f](https://github.com/adambiggs/gangline/commit/5c9a00f56ea6055d6c718cfb106b38fd0131043b))
* **demo:** agent state is per-take state ([d2e525a](https://github.com/adambiggs/gangline/commit/d2e525a2a54b88e19fd43b355a2cef07ff1394f7))
* **demo:** stop a take inheriting the identity of whoever records it ([511c8ad](https://github.com/adambiggs/gangline/commit/511c8ad43c9cc2dc6fb63008f34c1348c0850562))
* **diagnostics:** stop asserting causes gang never checked ([594e248](https://github.com/adambiggs/gangline/commit/594e24842910b8154e59653d90ad706f618f6140))
* doctor runs the file-format gate even when the version probe fails ([a4a24ad](https://github.com/adambiggs/gangline/commit/a4a24adb02075922630ee7645c6986294faf7331))
* **doctor:** dedup rot issues via the list endpoint, not search ([69616a8](https://github.com/adambiggs/gangline/commit/69616a83e035c4ee89580f0f6c1dc0c43fa7bd14))
* fail loud when addressing resolves to nothing ([4dd7c18](https://github.com/adambiggs/gangline/commit/4dd7c18c6712add909bcbd02478ab5366119a64d))
* **gang:** close the three bin/gang source issues ([#9](https://github.com/adambiggs/gangline/issues/9), [#10](https://github.com/adambiggs/gangline/issues/10), [#5](https://github.com/adambiggs/gangline/issues/5)) ([0d588eb](https://github.com/adambiggs/gangline/commit/0d588eb44e77881f673ef6cdf825d36df37408a8))
* **hitch:** a brief that lands on a gate must not exit 0 ([c7d0def](https://github.com/adambiggs/gangline/commit/c7d0def64084313da8d9802c7007b8bf97ee68dd))
* **hitch:** check readiness whether or not there is a brief to deliver ([a20eba2](https://github.com/adambiggs/gangline/commit/a20eba262dc1f045a3efd6e76e331c8c8e879554))
* **hook:** stop rejecting conforming commits with long bodies ([2615b0b](https://github.com/adambiggs/gangline/commit/2615b0b375bff47a9ff4cac82096e45a5849785c))
* **hook:** tell an unreadable message file apart from an all-comment one ([7649e1e](https://github.com/adambiggs/gangline/commit/7649e1ec0a14cb4ae26650d71b12c8d4ee7fa06a))
* **input:** find the composer by an anchored glyph, not by a byte count ([9417312](https://github.com/adambiggs/gangline/commit/9417312eb48fff4c3ca30b86548243d561a1c847))
* **input:** tell what a person typed from what the harness suggested ([b7d05b7](https://github.com/adambiggs/gangline/commit/b7d05b77c87c37c76c677540d9e168635fd37565))
* let an agent be reached, and compact itself, while it is working ([510b07f](https://github.com/adambiggs/gangline/commit/510b07f73a0c016522c6739b6eac406209f01fd9))
* **lint:** clear the eight shellcheck findings this branch introduced ([c533a52](https://github.com/adambiggs/gangline/commit/c533a526e11fb96cbb9599022e34957e9dd51c22))
* make addressing, delivery, and sweeps fail loud instead of wrong ([5e2c650](https://github.com/adambiggs/gangline/commit/5e2c65071fc65bfde85c0c1273025d58c36dd6f4))
* never spend "could not determine" as "determined false" ([6c2ceb9](https://github.com/adambiggs/gangline/commit/6c2ceb94d452c4d29705b7f9e35f72f222d133ee))
* **opencode:** clear the role directory gang itself points the agent at ([73eaf3e](https://github.com/adambiggs/gangline/commit/73eaf3e8fe8033f490f9da31647cb97af6d5b345))
* **patrol:** gate injection on pane stability and detect compaction as busy ([be98f94](https://github.com/adambiggs/gangline/commit/be98f94387523bfa423e710c20024435fb050189))
* **pi:** match the picker cursor by regex, not by a four-byte substr ([077cdeb](https://github.com/adambiggs/gangline/commit/077cdebbb9332407168f613f8447e990a95675f9))
* **profiles:** declare claude code's compact command ([c21d272](https://github.com/adambiggs/gangline/commit/c21d272c3e014b41c94442e6bc8bbffa7e3eb3dd))
* **profiles:** stop offering the test stand-in as a harness ([b4ecd42](https://github.com/adambiggs/gangline/commit/b4ecd426f3c41b1fb70da2102a71b81cd726ed6e))
* **resume:** wait for proof a compaction happened, not for a quiet screen ([59c7c13](https://github.com/adambiggs/gangline/commit/59c7c13e1407612b4b605e8e7ec62e393057eb28))
* scrape in a UTF-8 locale, whatever the caller's is ([73f9670](https://github.com/adambiggs/gangline/commit/73f9670bb53108a4736202451fd2c407cf801bcb))
* **send:** accept pi's multi-line paste placeholder as delivery evidence ([54ced7f](https://github.com/adambiggs/gangline/commit/54ced7f19b889e9a42a6140baefffd3fcd11caa1))
* **send:** an unreadable input box after Enter is not a failed submit ([7561805](https://github.com/adambiggs/gangline/commit/7561805c29826faf2dff22abed29408ae66c7ae9))
* **send:** check the gate again before pressing Enter ([43dd5f9](https://github.com/adambiggs/gangline/commit/43dd5f939124ac083771fc650908e1405266f04e))
* **send:** envelope every message, and sign it with the window it came from ([537da22](https://github.com/adambiggs/gangline/commit/537da224c5d6a9ea5553d309a0e6bbe118874591))
* **send:** one pane, one writer ([4a04c9a](https://github.com/adambiggs/gangline/commit/4a04c9ad1c9451b055283a5478e858b7fca574a7))
* **send:** verify delivery against the input box, not a paste placeholder ([b96a929](https://github.com/adambiggs/gangline/commit/b96a929c603a2d06042e5324c14bff1be5167067))
* **send:** verify delivery through TUI paste-collapse placeholders ([b70806a](https://github.com/adambiggs/gangline/commit/b70806a6ba3361647652072d8d94f6a25251c269))
* **spawn:** brief an agent only once its input box is painted ([7cfaebf](https://github.com/adambiggs/gangline/commit/7cfaebfeba0feaa709e313cac72bc78872c28e16))
* **state:** a gate is a dialog that owns the screen, not its words on it ([0203a06](https://github.com/adambiggs/gangline/commit/0203a06c5df98c67bf0e9df6ebd49077f6c6befb))
* **state:** a server never started is not a server that cannot be reached ([a58a28d](https://github.com/adambiggs/gangline/commit/a58a28d5dddac62f262187fbe465e61cec4c8324))
* **state:** ask whether the harness WROTE, not only what the pane shows ([db7a48f](https://github.com/adambiggs/gangline/commit/db7a48f89a6d8d40cd889f7215adcac3dd717e7a))
* **state:** read a declared gate marker from the whole pane ([37c2834](https://github.com/adambiggs/gangline/commit/37c28342beb058680fb644b23f98d61266b98781))
* **state:** read a live turn from churn, not from a marker that is not painted ([517df5b](https://github.com/adambiggs/gangline/commit/517df5b4048760266f444ef4931cc8da23498efe))
* **state:** resolve an undeterminable pane to gated, never idle ([d455e0a](https://github.com/adambiggs/gangline/commit/d455e0ad47d46a4e5233b2d682370a8ae540db1e))
* **state:** stop reporting an agent busy for gang's own waiting call ([8e6c170](https://github.com/adambiggs/gangline/commit/8e6c170ed9814544aefc8c79083a1d93505aa1ca))
* **state:** take the marker match off a pipe, where a lost race read busy as idle ([a6edf22](https://github.com/adambiggs/gangline/commit/a6edf222ace5804821dfbc62e39463f389b5c257))
* **status:** debounce idle detection to two consecutive polls ([fcbc4dd](https://github.com/adambiggs/gangline/commit/fcbc4dd5420340bd7947b8b8e80187285a09fd89))
* stop building gang's own lists through a fork that can fail ([92f5f81](https://github.com/adambiggs/gangline/commit/92f5f8153658032411c34892877e9174c934580a))
* stop reporting success over an install that cannot work ([4f758ec](https://github.com/adambiggs/gangline/commit/4f758ec3761a5e3b52382e5446e53895815361e5))
* surface three failures that were invisible ([8d8c1a8](https://github.com/adambiggs/gangline/commit/8d8c1a85b53ef2a285634723eadb7ad937a1af39))
* **test:** bound the probe in shell, because the bound it had was decoration ([f7e8452](https://github.com/adambiggs/gangline/commit/f7e8452e1f31e012651dcdfa5b4111c2293bbb9a))
* **test:** keep the word shellcheck off the start of a comment line ([6f9713b](https://github.com/adambiggs/gangline/commit/6f9713bf4c07f011cf95ede9798218e651cff0fc))
* **test:** move every case out of a command substitution, so the suite parses on 3.2 ([8b04781](https://github.com/adambiggs/gangline/commit/8b047819b8e30b160cbb24703591af09db62f7a3)), closes [#6](https://github.com/adambiggs/gangline/issues/6)
* **test:** refuse to address the active pane when a window is missing ([e4d18ff](https://github.com/adambiggs/gangline/commit/e4d18ff537c3859a6fd4a6cd32fe2c1dd742b543))
* **test:** scope the probe-socket check to this run, not to the whole directory ([6655e21](https://github.com/adambiggs/gangline/commit/6655e2198ea833d57f6aae1f2dccf4b680e1550d))
* **vet:** capture the issue list instead of piping it into grep -q ([6ca3cf3](https://github.com/adambiggs/gangline/commit/6ca3cf302f442dccd10607d7b413cc888230072c))
* **vet:** read the box between the paste and the Enter, as cmd_send does ([ee67c12](https://github.com/adambiggs/gangline/commit/ee67c12a78ef36dc920578b54a3c45a50fa7a636))
* **vet:** walk every installed profile, including the ones nobody ships ([b1847fe](https://github.com/adambiggs/gangline/commit/b1847fe2ded76090e28e1424ba4cb224cd05b334))
* wait for a change you caused, not for a condition already true ([3196753](https://github.com/adambiggs/gangline/commit/31967530590ebc027483f252e1f510d3f709cbf2))


### Performance Improvements

* **state:** resolve churn for the whole team in one wait ([6272f0e](https://github.com/adambiggs/gangline/commit/6272f0e78898810ebd175b9a1d43717abe59725e))


### Code Refactoring

* rename executable and protocol from gl to gang ([c50fafe](https://github.com/adambiggs/gangline/commit/c50fafe0ba8110dcdc2d21b7b83dac81fdb1f159))


### Build System

* require python3 ([813891a](https://github.com/adambiggs/gangline/commit/813891a54aec78b02390ae02424badd8092ebaaf))
