# Changelog

## [1.14.1](https://github.com/adambiggs/gangline/compare/gangline-v1.14.0...gangline-v1.14.1) (2026-10-03)


### Bug Fixes

* **capture:** name the agent when its record has no pane ([aa332a1](https://github.com/adambiggs/gangline/commit/aa332a18a492c68b6b845a49c34e17b402e43fe3))
* **down:** record who ran a down before any agent is dropped ([8d833a8](https://github.com/adambiggs/gangline/commit/8d833a8c71ff603f307a38578fe00fe2ecc344bb))
* **hitch:** record a booting agent as the hitcher of agents it starts ([434be17](https://github.com/adambiggs/gangline/commit/434be17c4c0b99e2705df96d1504211d32408aea))
* **hitch:** tell the hitcher of every failure, boot deadline included ([ac88bf6](https://github.com/adambiggs/gangline/commit/ac88bf686584b1d0515e721e2ddd07f2d73f1f56))
* **hitch:** tell the hitcher when its agent fails ([abf5435](https://github.com/adambiggs/gangline/commit/abf54355550666a26d6b0e4535c35f076adf5a9a))
* **team:** grant lead authority only to the lead the operator started ([a692496](https://github.com/adambiggs/gangline/commit/a69249672866ac4cb4a6ec99a1014bebe72b86a2))
* **team:** refuse ending teammates from an agent pane that did not start them ([fb3849f](https://github.com/adambiggs/gangline/commit/fb3849fc9c2bf1017062528311b260f3cfb57bab))

## [1.14.0](https://github.com/adambiggs/gangline/compare/gangline-v1.13.1...gangline-v1.14.0) (2026-10-03)


### Features

* **upgrade:** confirm before installing, with --yes to skip ([8f43b76](https://github.com/adambiggs/gangline/commit/8f43b76bc1d2d5584327ef81156ba6f7ef2c7fa5))

## [1.13.1](https://github.com/adambiggs/gangline/compare/gangline-v1.13.0...gangline-v1.13.1) (2026-10-03)


### Bug Fixes

* **activity:** log blocked and wedged readings without screen text ([ddfac99](https://github.com/adambiggs/gangline/commit/ddfac99ae958dc830d03fc9fc750c180fa605572))
* **compact:** name a capacity wait while the failed turn's screen reads busy ([4c548f9](https://github.com/adambiggs/gangline/commit/4c548f992da0aa393fcb87225e81e1f7b895bd56))
* **compact:** start a queued compaction after a non-capacity turn failure ([29864e3](https://github.com/adambiggs/gangline/commit/29864e342f856422b1e7121b064cd8d4c63f4fcc))
* **delivery:** tell the sender when an unverified message is later delivered ([0ab91a0](https://github.com/adambiggs/gangline/commit/0ab91a0d16fb7176ad3ed5450b282c435c5b1bf1))
* **native:** clear an unattributed turn failure when a queued turn finishes ([f14ec36](https://github.com/adambiggs/gangline/commit/f14ec36baa6ab5c056d777fa90bc80b7c3e55fd4))
* **native:** record a queued turn's failure against that turn ([905515f](https://github.com/adambiggs/gangline/commit/905515f01b9f62727b014a121beec75c320db52a))
* **snooze:** fail a wake pulled out of the native queue ([e5f5144](https://github.com/adambiggs/gangline/commit/e5f5144cf668a5cc00d2139db358b38beaef4051))
* **tmux:** reap a pane exit when the server lost its child signal ([b48ba09](https://github.com/adambiggs/gangline/commit/b48ba09107c63ec1ce112e5f9a4b59b9b8f36833))
* **tmux:** treat a macOS signal refused to an exiting process as gone ([611c77c](https://github.com/adambiggs/gangline/commit/611c77cf13158d60cf39c2326d14a9ac14e1c34c))
* **watchdog:** bound cleanup's wait for the scheduler lock ([19b4197](https://github.com/adambiggs/gangline/commit/19b4197796791f075cee5251cbd573e186d96238))

## [1.13.0](https://github.com/adambiggs/gangline/compare/gangline-v1.12.5...gangline-v1.13.0) (2026-10-02)


### Features

* **log:** record what each activity reading was derived from ([1471987](https://github.com/adambiggs/gangline/commit/1471987cf26863ecc4f123bd1b7313672e8c686d))


### Bug Fixes

* **compact:** log the move to unverified when a compaction passes its deadline ([659f151](https://github.com/adambiggs/gangline/commit/659f151716f57fcedb7bf3eb3047f5264c7a2a72))
* **compact:** name the agent and its mode in the busy retry ([2a0c74e](https://github.com/adambiggs/gangline/commit/2a0c74ee55ee1cb49e243bfa6deac164873383b9))
* **delivery:** keep a drain or drop going when the sender notice fails ([97d7aee](https://github.com/adambiggs/gangline/commit/97d7aee1f6cf1b5a3e9f9ea1382648bb770d482b))
* **drop:** finish when an unheld pane closes before its identity read ([6ab460b](https://github.com/adambiggs/gangline/commit/6ab460bc17f8a2d49841ac453d7da00dcc0e1838))
* **native:** clear a turn failure when a queued turn finishes after it ([5f6c4d7](https://github.com/adambiggs/gangline/commit/5f6c4d74096fce8e4fb96d8d5c5b6c1bb816e443))
* **snooze:** judge a queued wake by the turn that ran it ([a4fe3e2](https://github.com/adambiggs/gangline/commit/a4fe3e26e8dd96d88f9692aa0ab3b2f799c0250d))
* **tmux:** retry a macOS process-exit wait a signal interrupted ([af8b1f3](https://github.com/adambiggs/gangline/commit/af8b1f39d430ba2cfb238f6e3f99d689cef44d41))
* **watchdog:** wait for the scheduler lock when the last agent leaves ([ac333e5](https://github.com/adambiggs/gangline/commit/ac333e55ce987fc8c5232e8f47f16a6141e102a2))

## [1.12.5](https://github.com/adambiggs/gangline/compare/gangline-v1.12.4...gangline-v1.12.5) (2026-10-02)


### Bug Fixes

* **activity:** keep a turn open while Claude Code holds a queued prompt ([f1f83e0](https://github.com/adambiggs/gangline/commit/f1f83e0a2d419eac1c7e34adf9c7535c53825de3))
* **cli:** resolve --version before help and usage errors read the command ([7b7553f](https://github.com/adambiggs/gangline/commit/7b7553f6cc67a4d5103303702e590cbc186f991f))
* **compact:** refuse a locked agent with the retry ([db0ec90](https://github.com/adambiggs/gangline/commit/db0ec90085fa03ba5213aaaf52709a904d9a4006))
* **compact:** refuse while an unverified compaction still holds its note ([4e66276](https://github.com/adambiggs/gangline/commit/4e66276c07ed75e28e9abf50e47b3815357c97cb))
* **delivery:** tell the sender when a held message is not delivered ([72b5521](https://github.com/adambiggs/gangline/commit/72b55219fc19820ed37fcba55d23dc95bb2a9c01))
* **snooze:** judge a wake queued behind a running turn by its own turn ([e871140](https://github.com/adambiggs/gangline/commit/e8711408ebeb905ad66ce694c0bf0c1f440da87d))

## [1.12.4](https://github.com/adambiggs/gangline/compare/gangline-v1.12.3...gangline-v1.12.4) (2026-10-02)


### Bug Fixes

* **blocked:** bound a Claude Code view whose only rule is dim ([a1ccbf2](https://github.com/adambiggs/gangline/commit/a1ccbf23d716ebac416e1b9e5ef2e57aa4a6bca7))
* **blocked:** detect a Claude Code question dialog ([9aa0bad](https://github.com/adambiggs/gangline/commit/9aa0bad283e05357747dc4a80ca5aa97f9159ef1))
* **blocked:** let a Claude Code composer holding the cursor own input ([5c1baa4](https://github.com/adambiggs/gangline/commit/5c1baa4a79ccd930be9101e8279982807b8ac988))
* **blocked:** read a Codex prompt only below the conversation ([7c87dcc](https://github.com/adambiggs/gangline/commit/7c87dccb58ec365ca2e886afc03dd709404c5eb5))
* **drop:** finish teardown when the native exits before its identity is read ([acfa429](https://github.com/adambiggs/gangline/commit/acfa42907972e06569077b62b9f1d95625685076))
* **startup:** detect the Codex hooks review list ([302817c](https://github.com/adambiggs/gangline/commit/302817c582be7f4818aa261c53e676d4705b36fa))
* **startup:** detect the Codex per-event hooks review ([f80f568](https://github.com/adambiggs/gangline/commit/f80f568229ca2104fa452e39f440eb5d6b63b5c5))
* **tmux:** read a pane root that exits during the macOS table read as gone ([0b8c2d5](https://github.com/adambiggs/gangline/commit/0b8c2d5e4186f6c173a755cffd53d1e5775bf85c))
* **tmux:** report a pane process that vanished between reads as its exit ([251f745](https://github.com/adambiggs/gangline/commit/251f745991d68964611dacb211e774ce3c6c7e46))
* **tmux:** report a reaped process as gone when opening its handle ([93628a0](https://github.com/adambiggs/gangline/commit/93628a0829e51e1c88df681a95a2223bb3df16b5))
* **tmux:** report a tmux server that exits mid-witness by its own answer ([9726923](https://github.com/adambiggs/gangline/commit/972692355438a1ee9d098808c75a44c692f26b2c))
* **tmux:** skip a recorded process reaped before it is pinned ([74f0aab](https://github.com/adambiggs/gangline/commit/74f0aab09a8431f21b5e977631b773a1782732e1))
* **tmux:** take the recorded root when a pane process exits during acquisition ([3107921](https://github.com/adambiggs/gangline/commit/31079210fd9afe1dacd0d63f84dd604243d4f7fb))

## [1.12.3](https://github.com/adambiggs/gangline/compare/gangline-v1.12.2...gangline-v1.12.3) (2026-10-02)


### Bug Fixes

* **blocked:** read a native prompt only from the live input surface ([baefa3b](https://github.com/adambiggs/gangline/commit/baefa3b0b8e3074e66bd873fe1812d6e3fd214db))
* **cli:** name a route when an agent name is not registered ([a6a0e58](https://github.com/adambiggs/gangline/commit/a6a0e58ecf54d25199ba73f9b11f90121e259b9b))
* **cli:** resolve --version as the version command in help ([c5f6067](https://github.com/adambiggs/gangline/commit/c5f6067b3c3cfb81882b9328cb022827d83728a8))
* **compact:** fail a compaction whose queued resume note is blocked ([f38a97a](https://github.com/adambiggs/gangline/commit/f38a97a58d63f09c409ba6db5bb7dbd252b29438))
* **compact:** refuse a compaction while the last resume note is queued ([892c0ce](https://github.com/adambiggs/gangline/commit/892c0ce252f951a8ea1a9ca068c50207fc6e803f))
* **snooze:** name a route when the caller cannot snooze ([dd8bc3e](https://github.com/adambiggs/gangline/commit/dd8bc3e5838f78a568e540ff4d59a2dc3df4bd6f))
* **snooze:** refuse a paste placeholder in a note for any harness ([7157215](https://github.com/adambiggs/gangline/commit/7157215923cac1728a1c38ae372a1bd8b9a3df53))
* **startup:** read a trust prompt only from the live input surface ([771cf10](https://github.com/adambiggs/gangline/commit/771cf10f0c2ec7fab81479643e1c569bbe41047b))

## [1.12.2](https://github.com/adambiggs/gangline/compare/gangline-v1.12.1...gangline-v1.12.2) (2026-10-02)


### Bug Fixes

* **cli:** name the next step in identity refusals ([7d7447b](https://github.com/adambiggs/gangline/commit/7d7447beac0a549db5622a4caf58b3944706d9a1))
* **cli:** refuse extra arguments to gang version as unexpected ([13279fa](https://github.com/adambiggs/gangline/commit/13279fa45ef365d8a639f3be12d884f6e5b4e383))
* **cli:** word collars, roles and config argument errors like other commands ([a64ad1c](https://github.com/adambiggs/gangline/commit/a64ad1c8a2b60e6cd8fe6a06fd8b8bc638bd7661))
* **roster:** fail an unlisted agent whose recorded process has exited ([50c1511](https://github.com/adambiggs/gangline/commit/50c151125eafe3cb49b9aa82f62ce4ba8f4da9f5))
* **roster:** keep the record when tmux lists no team session ([01ef169](https://github.com/adambiggs/gangline/commit/01ef1691c9109369a0874f43c7c42cd370b35aeb))
* **tick:** record a blocked startup's prompt once ([a15ee89](https://github.com/adambiggs/gangline/commit/a15ee899012ca81460404fc64eba6a34d69899a1))
* **tmux:** read a registered pane and its session from one listing ([5b68164](https://github.com/adambiggs/gangline/commit/5b68164ee0dfad0df65465f9157e045967b34f3f))
* **tmux:** take the recorded root when a pane closes during a drop ([06cba3c](https://github.com/adambiggs/gangline/commit/06cba3c5949fb9d727c04f8533f199b3c6572259))

## [1.12.1](https://github.com/adambiggs/gangline/compare/gangline-v1.12.0...gangline-v1.12.1) (2026-10-02)


### Bug Fixes

* **activity:** read a witnessed turn as busy until its finish boundary ([4829dc7](https://github.com/adambiggs/gangline/commit/4829dc72c2ee432637cf71c241bc877cc53fe2ff))
* **collar:** match a Claude compaction spinner past its first minute ([def4080](https://github.com/adambiggs/gangline/commit/def408037ab2578a329094fa25a6e58497ed0038))
* **compact:** capture under the operation context during the start window ([9c8398c](https://github.com/adambiggs/gangline/commit/9c8398c1a78aa8aa48d443094e6d090438efcae7))
* **compact:** fail a compaction abandoned before its submit key as not run ([640bbde](https://github.com/adambiggs/gangline/commit/640bbdee6a34993f1e4bea2743134644d94b7eec))
* **compact:** fail a compaction with no start shown as possibly run ([e811ed1](https://github.com/adambiggs/gangline/commit/e811ed1025e6a9d47c79dc185da5e98ff95d0410))
* **compact:** hold input while the pane shows a compaction running ([0d7bb07](https://github.com/adambiggs/gangline/commit/0d7bb07e08505fe9cbc1a83c5908e40425ec2af7))
* **compact:** tell the requester when a queued compaction fails ([41cb8d8](https://github.com/adambiggs/gangline/commit/41cb8d823de9bc9f87259ce403897cebc86d1093))
* **compact:** withhold the resume note until a compaction shows on screen ([6815485](https://github.com/adambiggs/gangline/commit/681548591e56e6304478f047184115306154271a))
* **send:** hold input until a queued resume note is admitted ([02ed8c9](https://github.com/adambiggs/gangline/commit/02ed8c9ff3f5307825c8389f66ac06f69538c77b))
* **send:** match Claude Code's own paste placeholder forms ([9d836a2](https://github.com/adambiggs/gangline/commit/9d836a241302d7e9b9fc264624c928fd4b473f74))
* **send:** refuse a paste placeholder token for Claude Code recipients ([7211e13](https://github.com/adambiggs/gangline/commit/7211e1360ea6758d3c780260d6f37f670a60eb4e))

## [1.12.0](https://github.com/adambiggs/gangline/compare/gangline-v1.11.0...gangline-v1.12.0) (2026-10-02)


### Features

* **compact:** point the default resume note at queued messages ([b6fc6b0](https://github.com/adambiggs/gangline/commit/b6fc6b09f05c24a833dd6a9ae7a34114a98c5a58))


### Bug Fixes

* **cli:** name the next step in inactive and busy refusals ([792269b](https://github.com/adambiggs/gangline/commit/792269baddbfa3a36eb878fee4e29da8fea4e777))
* **cli:** state the accepted form in option and value errors ([bff35f2](https://github.com/adambiggs/gangline/commit/bff35f2c290bc0af431c573b4409e359c37d6aba))
* **drop:** do not warn of skipped cleanup for a native that exited at boot ([eaba1ce](https://github.com/adambiggs/gangline/commit/eaba1ceca46b52bab63495d9142eb672e9c13bf0))
* **drop:** refuse an unregistered agent with exit status 3 ([843ec13](https://github.com/adambiggs/gangline/commit/843ec13d3805635c3da959366bc3b9f0e10808fb))
* **help:** keep every option meaning in the Options column ([8e20ffb](https://github.com/adambiggs/gangline/commit/8e20ffbb547ac36704cdff9b761c9ee1c70094ea))
* **help:** show collar check help for help collar check ([21dfd1f](https://github.com/adambiggs/gangline/commit/21dfd1f11cd6f501b2665aff3fb46cf2a935145a))
* **help:** state option defaults ([d208a39](https://github.com/adambiggs/gangline/commit/d208a39682bb7850fb3191ad9ac457ed0fc11375))
* **hitch:** name why a pane's hold failed ([3cd33d8](https://github.com/adambiggs/gangline/commit/3cd33d8cd03f6dbad861c90241c41998d084d3d7))
* **roster:** keep a row's reason within the terminal's row ([6d39ec2](https://github.com/adambiggs/gangline/commit/6d39ec23a2b58c912ef61330da241fd86583f785))
* **roster:** show the saved record of an agent whose state is locked ([b2496f1](https://github.com/adambiggs/gangline/commit/b2496f1860bf1077b8df86686c925b19681f8614))
* **tick:** release the pane hold a killed hitch left on a ready agent ([c55c295](https://github.com/adambiggs/gangline/commit/c55c295c82f1812d3b8d5138fa3050239e35d967))
* **tmux:** read an exited tmux server as its panes closed ([d5687cf](https://github.com/adambiggs/gangline/commit/d5687cf03b679d56ca6ac5938375d3290e2e8023))
* **tmux:** reap before reading a dead pane's exit status ([9c2ba04](https://github.com/adambiggs/gangline/commit/9c2ba043e8c36ec9f528c9f316cd0e00e0a2d704))

## [1.11.0](https://github.com/adambiggs/gangline/compare/gangline-v1.10.3...gangline-v1.11.0) (2026-10-02)


### Features

* **snooze:** show the lead every wake and log each wake stage ([9ceebeb](https://github.com/adambiggs/gangline/commit/9ceebeb2377fbdf259cbe51bfa2a158615590ea5))

## [1.10.3](https://github.com/adambiggs/gangline/compare/gangline-v1.10.2...gangline-v1.10.3) (2026-10-02)


### Bug Fixes

* **compact:** submit /compact only when the composer shows it ([6d48410](https://github.com/adambiggs/gangline/commit/6d484106d9af44606799381d1206ce44e24f38dc)), closes [#84](https://github.com/adambiggs/gangline/issues/84)
* **harness:** verify a Claude paste that quotes its own wrapper tag ([684b30f](https://github.com/adambiggs/gangline/commit/684b30f7bcfb480cda6e0217c9b3a11755079729)), closes [#84](https://github.com/adambiggs/gangline/issues/84)

## [1.10.2](https://github.com/adambiggs/gangline/compare/gangline-v1.10.1...gangline-v1.10.2) (2026-10-02)


### Bug Fixes

* **cli:** wrap help and usage text to 78 columns ([609a180](https://github.com/adambiggs/gangline/commit/609a1806daca41a3d7a071a99c5b9e9a2cb675e7))
* **drop:** warn about skipped cleanup only when there was something to clean ([0439faf](https://github.com/adambiggs/gangline/commit/0439faf7f9728c8d556ff6e85a8f1bbeb1250a3f))
* **hitch:** keep a native exit at a blocked startup and release at readiness ([e1700bc](https://github.com/adambiggs/gangline/commit/e1700bcc8fe6ba0e0da182652721d13a7f556013))
* **hitch:** report an interrupted hitch on one line ([ba25ddf](https://github.com/adambiggs/gangline/commit/ba25ddfa073644157af1bc9a0e0cc6662f5466e2))
* **roster:** show why an agent is failed, blocked, wedged, or unknown ([2c75052](https://github.com/adambiggs/gangline/commit/2c750529362d5acd9657db7914acd4e78e3c0588))
* **site:** restore the home page's left column and lead with the CLI ([a5884b9](https://github.com/adambiggs/gangline/commit/a5884b968f3b4c003b1d3fe0f177f778cc464abc))
* **tick:** fail a blocked startup whose pane id names another pane ([9cc400a](https://github.com/adambiggs/gangline/commit/9cc400a164fdf6256dca48884ab27cc1d2f4abc8))
* **tick:** fail a booting agent whose pane id names another pane at its deadline ([9755276](https://github.com/adambiggs/gangline/commit/9755276f1d83404fae7f009447989be119ca8195))
* **tick:** fail and forget a registered pane closed outside gang ([b99034f](https://github.com/adambiggs/gangline/commit/b99034fe46ab465f91ba1e180b045e88d44e4be1))
* **tick:** forget a registered pane only when its server shows it closed ([b532931](https://github.com/adambiggs/gangline/commit/b532931359fbd39bc8b8ef46e88f17d2f0dc0d69))
* **tick:** release a boot hold only on the registered pane ([59174fa](https://github.com/adambiggs/gangline/commit/59174fa2eeacb9d888d7bed52887d97fae1aa446))
* **tmux:** hold a launched pane from inside it before the native starts ([e1b1b1c](https://github.com/adambiggs/gangline/commit/e1b1b1c76d4a9547b536a0b54587adfeca9cc16a))
* **tmux:** match a registered pane in a window shared by several sessions ([fabc4e7](https://github.com/adambiggs/gangline/commit/fabc4e73f137c197bdc65e843bd15e51f1b9a51d))
* **tmux:** run a held native command through the user's default shell ([5d76a2c](https://github.com/adambiggs/gangline/commit/5d76a2c8df9bbeb8e978a8e5867f77857f4806ef))
* **tmux:** title an agent's window only through its pane registration ([770875b](https://github.com/adambiggs/gangline/commit/770875b62ff0b0bf20d2c7333321eff83a1a41d0))

## [1.10.1](https://github.com/adambiggs/gangline/compare/gangline-v1.10.0...gangline-v1.10.1) (2026-10-01)


### Bug Fixes

* **hitch:** leave the team usable after an interrupted hitch ([585e269](https://github.com/adambiggs/gangline/commit/585e2699c5b51d7aba4211089aee3e54d63e5bdc))
* **hitch:** report a native CLI's boot exit instead of losing it with the pane ([163ef5f](https://github.com/adambiggs/gangline/commit/163ef5fb0fc94324cf9e2779ef240d9eda1695b9))

## [1.10.0](https://github.com/adambiggs/gangline/compare/gangline-v1.9.2...gangline-v1.10.0) (2026-10-01)


### Features

* **cli:** print roster, status, and context as JSON ([0343ce1](https://github.com/adambiggs/gangline/commit/0343ce1f0cd52ba1e573cb4ee754d9bfbebd5388))
* **cli:** select the team with --team on every team command ([843646d](https://github.com/adambiggs/gangline/commit/843646d059f5b3ea583d155c0175a5af5aa8e2ab))
* **queue:** show why each pending message waits ([4247a89](https://github.com/adambiggs/gangline/commit/4247a89b85f9a802f3fce6a6d2843a307e56b40d))


### Bug Fixes

* **cli:** parse flags the same way in every command ([542b5a4](https://github.com/adambiggs/gangline/commit/542b5a4e7b9bd48be3cab3919cfa19378e29ba3d))
* **collar:** report unrun probes as unknown and surface cleanup failures ([fc24063](https://github.com/adambiggs/gangline/commit/fc240637d882a58a003f842fe2f1ff448899035f))
* **collar:** type probe input through guarded pane input ([53b3af9](https://github.com/adambiggs/gangline/commit/53b3af9f99fcf85b259fc3bcc54420532c699dad))
* **compact:** classify the native screen before compaction recovery ([4aec988](https://github.com/adambiggs/gangline/commit/4aec9882267a83b49d3e3a8ac4dbe282a41a785b))
* **compact:** hold compaction while startup input is unverified ([11e67df](https://github.com/adambiggs/gangline/commit/11e67dfa36386f0eed03f856eaf3b8db3916fa63))
* **compaction:** keep a retained failure receipt when withholding a resume ([7edafca](https://github.com/adambiggs/gangline/commit/7edafca2748e64923fe935c007f0e99f60d28695))
* **context:** keep retained receipts when discarding stale context notes ([2f85d0b](https://github.com/adambiggs/gangline/commit/2f85d0b31fcf5c43102eb626e036a43368190aa5))
* **hitch:** leave model ids to the native CLI and show catalog diagnostics ([a879f9e](https://github.com/adambiggs/gangline/commit/a879f9ec38a39026138e5a3f69962cf7b72920c5))
* **hitch:** refuse a resume only when its transcript names another session ([666e6ec](https://github.com/adambiggs/gangline/commit/666e6ec5f087ac32f963a749900e111c58ee4451))
* **hooks:** refuse the push when pre-push is interrupted ([935e566](https://github.com/adambiggs/gangline/commit/935e566e70a75075bb35fd5858d9f71add06e439))
* **hooks:** report pre-push worktree cleanup failures ([8de0f68](https://github.com/adambiggs/gangline/commit/8de0f68b0203377f12b5be04132cb57be0a2dac2))
* **hooks:** stop advising --no-verify when pre-push refuses ([6035e7f](https://github.com/adambiggs/gangline/commit/6035e7f16d5aab5869adb91ce2c96b0ef3edcac2))
* **snooze:** withdraw a due wake still queued for native input ([e01186f](https://github.com/adambiggs/gangline/commit/e01186f83124cd26d6d5f0d44c784a6c1423cf20))
* **statusline:** install into the Claude config directory Claude Code reads ([ff48774](https://github.com/adambiggs/gangline/commit/ff487740d82dfe2ed38763a37b3c83bbb2bb739e))
* **tick:** record tick failures in the team log ([f49ae70](https://github.com/adambiggs/gangline/commit/f49ae705d7ad5c1b3964130a3a961a800f7a4c25))
* **usage:** publish each recipient's notices and wakes independently ([d5a1183](https://github.com/adambiggs/gangline/commit/d5a1183c4bc0ef993b32e51048c1ab0219f1313e))
* **usage:** withdraw usage warnings once their window resets ([e14b2e5](https://github.com/adambiggs/gangline/commit/e14b2e5dc6e7bead221568a423525ce822b02d87))
* **watchdog:** mark an outage when an elapsed timer's tick cannot re-arm it ([b972f88](https://github.com/adambiggs/gangline/commit/b972f88f6b41a4aeea74ed50572a3412ca5f9da4))
* **watchdog:** mark an outage when replacing the recorded timer fails ([42d0ba2](https://github.com/adambiggs/gangline/commit/42d0ba22f4c71685e7fbd55c8b34c21ecbd66bff))
* **watchdog:** surface an unavailable scheduler and clear health markers on recovery ([00de05a](https://github.com/adambiggs/gangline/commit/00de05a2b2cbb6f97548a976a9b97353281e73d9))

## [1.9.2](https://github.com/adambiggs/gangline/compare/gangline-v1.9.1...gangline-v1.9.2) (2026-10-01)


### Bug Fixes

* **tmux:** dismiss output viewers before guarded input ([a3e7be8](https://github.com/adambiggs/gangline/commit/a3e7be8f40a92154492dd595a414f765675981b3))

## [1.9.1](https://github.com/adambiggs/gangline/compare/gangline-v1.9.0...gangline-v1.9.1) (2026-10-01)


### Bug Fixes

* **compaction:** reconcile completion before draining notices ([058ce97](https://github.com/adambiggs/gangline/commit/058ce97db7d32cace2a1645e570dc4722effa196)), closes [#68](https://github.com/adambiggs/gangline/issues/68) [#65](https://github.com/adambiggs/gangline/issues/65)
* **context:** cancel stale band notices after compaction ([f156d10](https://github.com/adambiggs/gangline/commit/f156d105441ca6d37e6d53c1c3de716709014888))
* **messages:** shorten Gangline system sender tags ([85d6071](https://github.com/adambiggs/gangline/commit/85d60713e3c81d8d70d0006964637f0f6ba09fe7))
* **startup:** retain proof for safe recovery before submission ([d26c096](https://github.com/adambiggs/gangline/commit/d26c09633fd9b9d85e1b7bc2c64d94f22c916e28)), closes [#66](https://github.com/adambiggs/gangline/issues/66)
* **tmux:** leave copy mode before guarded input ([6aed2d9](https://github.com/adambiggs/gangline/commit/6aed2d9349a0ad713e7061381802637c3511e8a3))

## [1.9.0](https://github.com/adambiggs/gangline/compare/gangline-v1.8.1...gangline-v1.9.0) (2026-10-01)


### Features

* auto-park agents on provider caps and keep usage notices factual ([a124fca](https://github.com/adambiggs/gangline/commit/a124fca90823222f8a225750581129ffdf945cfa))


### Bug Fixes

* **tmux:** stream long guarded sends through source-file ([1c1d760](https://github.com/adambiggs/gangline/commit/1c1d76059fd634cb77a578dc1f4cbc2f177688a1))

## [1.8.1](https://github.com/adambiggs/gangline/compare/gangline-v1.8.0...gangline-v1.8.1) (2026-09-30)


### Bug Fixes

* **collars:** name bundled Claude collar after its executable ([40acc33](https://github.com/adambiggs/gangline/commit/40acc330c3fa866232a39e1ccfdd9850693fafc2)), closes [#72](https://github.com/adambiggs/gangline/issues/72)

## [1.8.0](https://github.com/adambiggs/gangline/compare/gangline-v1.7.0...gangline-v1.8.0) (2026-09-30)


### Features

* confirm whole-team teardown interactively ([deccf5b](https://github.com/adambiggs/gangline/commit/deccf5bc9eda7067b31048cbb4a240e46daf56c0))

## [1.7.0](https://github.com/adambiggs/gangline/compare/gangline-v1.6.0...gangline-v1.7.0) (2026-09-30)


### Features

* **site:** add sharing cards to home and docs ([c258019](https://github.com/adambiggs/gangline/commit/c258019ccf48e228b37f5898fae7f2ecfde82516))
* **site:** measure visits and named links without tracking cookies ([bcd2304](https://github.com/adambiggs/gangline/commit/bcd2304852c92422801acb762aa2df21e2b1a0dc))
* warn leads at native usage limits and schedule agent wakes ([89631b2](https://github.com/adambiggs/gangline/commit/89631b269bce7a1b60c707edf4ee8e110b1b619b))


### Bug Fixes

* accept claude as an alias for the Claude Code collar ([c41262d](https://github.com/adambiggs/gangline/commit/c41262d990a69b98614c59a98570d72236fed640))
* bind sandbox commands to registered panes ([584e69a](https://github.com/adambiggs/gangline/commit/584e69ade5cce64ed56b9f8ba60a50f973282560))
* grant linked worktree gitdir to opted-in Codex profile ([16eab9a](https://github.com/adambiggs/gangline/commit/16eab9a5557649b04eb304655bc026d15b0082ee))
* keep uncertain usage input inspectable until confirmed ([6ba327f](https://github.com/adambiggs/gangline/commit/6ba327f7c65c2c9d7d244e7d9014584f9f59f8b9))
* record demo with sandboxed permission profile ([6113e31](https://github.com/adambiggs/gangline/commit/6113e317cdbb107e6b356f0780f7683529587d0a))
* recover unresolved usage wakes after failure ([ac8a17d](https://github.com/adambiggs/gangline/commit/ac8a17d970a68b3f5662abe0735f44182bfc715e))
* refuse adoption of an already registered pane ([be9aa9d](https://github.com/adambiggs/gangline/commit/be9aa9df4d0ee9416023494989ab8aa16aa43c28))
* refuse Codex bypass alias with permission profile ([1a4fbf6](https://github.com/adambiggs/gangline/commit/1a4fbf642cdd5a72c33a8dee05c10a9d67c02b4f))
* **site:** preserve spaces after setup links ([07c9149](https://github.com/adambiggs/gangline/commit/07c91496cf3e4449f1536075370174d433ff86ee))
* **tmux:** preserve multiline text in guarded sends ([a4ae55a](https://github.com/adambiggs/gangline/commit/a4ae55aaeb30557915a1287c62f0684a875b8d76))
* **tmux:** verify Darwin caller ancestry with native parent identity ([b6f1ccf](https://github.com/adambiggs/gangline/commit/b6f1ccfc807fe5a345368b5400eb5f2f869c0b28))


### Performance Improvements

* **hooks:** keep release and acceptance checks in the full gate ([e24afaf](https://github.com/adambiggs/gangline/commit/e24afaf58fc16679d66b400f02c096a51340f507))

## [1.6.0](https://github.com/adambiggs/gangline/compare/gangline-v1.5.0...gangline-v1.6.0) (2026-09-27)


### Features

* **site:** add the GitHub social preview card ([2bbb131](https://github.com/adambiggs/gangline/commit/2bbb131587d738b12936ed05c52d4f22162217dd))
* **site:** draw the social card's field at twice site scale ([d8c65d4](https://github.com/adambiggs/gangline/commit/d8c65d4604d9bf49151eb478f621e8ac8e94623f))
* **site:** hold the social card's field at a chosen moment ([9174715](https://github.com/adambiggs/gangline/commit/917471559ddcd21cd5507c87c6eb9ffb31edd81e))
* **site:** load the shared living field bundle ([a183fff](https://github.com/adambiggs/gangline/commit/a183fff8d62284397d08a2e2dfa7bec4667d84bd))
* **site:** pare the social card to wordmark, tagline and demo ([fed16f6](https://github.com/adambiggs/gangline/commit/fed16f6a32098ad083382e77443aaf2b25c628e9))


### Bug Fixes

* **demo:** preserve native harness colours in the recording ([ebb6036](https://github.com/adambiggs/gangline/commit/ebb6036ce5f5909d48882ce36b552a8d8aae294a))
* **help:** distinguish agent names from team sessions ([5ca18c6](https://github.com/adambiggs/gangline/commit/5ca18c6549a188988fdbf518e7d8b3c94e57f6c7))
* **hitch:** defer startup through folder trust prompts ([b8be646](https://github.com/adambiggs/gangline/commit/b8be6463024f4c0801c8608d89d86bf75a3ed36a))
* **hitch:** preserve startup stabilization past boot deadline ([93893e2](https://github.com/adambiggs/gangline/commit/93893e27c2f48b56d34bcb1036d26b0bff4a74ad))
* **hitch:** recognize permission choices during startup ([2cb6acc](https://github.com/adambiggs/gangline/commit/2cb6acc7dce0f26ff6908e150aefec8c88702f17))
* **send:** use registered pane identity and tmux foreground ([bfbbf98](https://github.com/adambiggs/gangline/commit/bfbbf9819b4fb7b0c27ee3e97a6ef13c6579de00))
* **site:** render the social card from a still field frame ([9226bbf](https://github.com/adambiggs/gangline/commit/9226bbf8bda368f8f565f887bdb3084edbd92553))
* **site:** render the social card's field at the card's true size ([c2ae4bb](https://github.com/adambiggs/gangline/commit/c2ae4bbc9807b0f5a39871b5c4407e2ac4cf1bae))
* **store:** wake file watchers when audit logs append ([d5a2585](https://github.com/adambiggs/gangline/commit/d5a2585062a0de9de31cb604a3fe9249fd24ebb5))
* update the vendored field with fixed glyph spacing ([76c3ea8](https://github.com/adambiggs/gangline/commit/76c3ea84e14cd7e6ff21135af582a6a5ba7a2959))
* update vendored field for continuous quality changes ([aae8b7e](https://github.com/adambiggs/gangline/commit/aae8b7e6d90d0192fc67240bbabab9113d6ccbd6))

## [1.5.0](https://github.com/adambiggs/gangline/compare/gangline-v1.4.0...gangline-v1.5.0) (2026-09-26)


### Features

* **collars:** support partial overlays and configurable context notices ([5e24a72](https://github.com/adambiggs/gangline/commit/5e24a72bdbe5ef638b5c92f7ebc35bf4edfc8036))


### Bug Fixes

* **cli:** parse options before validating command operands ([23916b0](https://github.com/adambiggs/gangline/commit/23916b07ce1f03a2db762858e1f3948bc309b000))
* **cli:** retain specific flag errors in command diagnostics ([3bfeed7](https://github.com/adambiggs/gangline/commit/3bfeed7ba7618bbec056888add7098138936058d))
* **cli:** stop help interception after operands and terminators ([f09d323](https://github.com/adambiggs/gangline/commit/f09d3234e4d654a4e576055e123eb728bd77140b))
* **compact:** accept wrapped composer read-back as the submitted text ([db202c8](https://github.com/adambiggs/gangline/commit/db202c81f9d4dacb393c5b70df195688c241d340))
* **config:** show environment sources for environment-only settings ([c782462](https://github.com/adambiggs/gangline/commit/c7824621b471163473655fc9036217037d900cf4))
* **help:** keep the command inventory concise and show flag syntax once ([ea96ecb](https://github.com/adambiggs/gangline/commit/ea96ecbb846de5fd06338687be57382f13f17cf9))
* **hitch:** report missing harness before creating a pane ([5135e8a](https://github.com/adambiggs/gangline/commit/5135e8a5ef8916433491d0ac3cb029802a99333d))
* **install:** skip Claude settings without its CLI ([59eb60f](https://github.com/adambiggs/gangline/commit/59eb60febd1603db363dc9aa9ece7e719e6a2a38))
* **schedule:** accept exact deadlines and Go durations ([73155d6](https://github.com/adambiggs/gangline/commit/73155d6d4b6edbdb9f6b1b638708776d63470a71))
* **send:** report retained message when draining fails ([e31b81b](https://github.com/adambiggs/gangline/commit/e31b81b702a77b564f466142302320b471b031fb))

## [1.4.0](https://github.com/adambiggs/gangline/compare/gangline-v1.3.0...gangline-v1.4.0) (2026-09-25)


### Features

* **distribution:** install verified release binaries ([acf80c0](https://github.com/adambiggs/gangline/commit/acf80c0b2fbf1b52c8281a27a42fe10d16f622bf))


### Bug Fixes

* **compact:** submit resume before completion to preserve input order ([925f47d](https://github.com/adambiggs/gangline/commit/925f47d3505d1c86de7620026c1d6caddc61f049))

## [1.3.0](https://github.com/adambiggs/gangline/compare/gangline-v1.2.1...gangline-v1.3.0) (2026-09-25)


### Features

* **send:** accept a positional message body for stable commands ([396eeb7](https://github.com/adambiggs/gangline/commit/396eeb72b4d2bdfc5e9932d8c22964c1106d334b))


### Bug Fixes

* attribute startup context separately from assignments ([b074ab7](https://github.com/adambiggs/gangline/commit/b074ab700a028177bd3a90dacafaeb7fe33a0240))
* **hitch:** preserve Codex startup through permission menus ([a93a331](https://github.com/adambiggs/gangline/commit/a93a331ae179f5235182474277b2a7aa347a3a33))
* keep spinner progress from reporting idle ([5f684cc](https://github.com/adambiggs/gangline/commit/5f684cc47fc60ff76803166eec8a788868f513df))
* read macOS process lineage without ps ([01b56eb](https://github.com/adambiggs/gangline/commit/01b56eb2af5eb8fb2ab2a5bc2a40c00ee47b9cd9))
* recover retained startup after collapsed paste ([109fc95](https://github.com/adambiggs/gangline/commit/109fc9549e2672c96f3cdf44ef0f751628dad492))
* retain process identity across exec before pinning ([e872561](https://github.com/adambiggs/gangline/commit/e872561a934fed42391f034588bf37a1a49f00ac))
* supersede stale names when starting a stopped team ([a1999c0](https://github.com/adambiggs/gangline/commit/a1999c0b23a3f76c83ca6846813e1c25e3c67bd7)), closes [#60](https://github.com/adambiggs/gangline/issues/60)

## [1.2.1](https://github.com/adambiggs/gangline/compare/gangline-v1.2.0...gangline-v1.2.1) (2026-09-24)


### Bug Fixes

* **cli:** generate complete flag help from parser definitions ([ab140c6](https://github.com/adambiggs/gangline/commit/ab140c6361c6234647e5a2c8e593398a02dc147b))
* **cli:** show flag aliases on one help row ([85ba771](https://github.com/adambiggs/gangline/commit/85ba771cfe970bb79ae7805b5dfb6345dff3497a))
* explain recovery when stopped team retains lead claim ([7552c09](https://github.com/adambiggs/gangline/commit/7552c09db2a8d14c84ec0300e116cc8f60afb455))
* ignore Claude composer suggestion at empty input cursor ([12d06f3](https://github.com/adambiggs/gangline/commit/12d06f34ca0326a4fb5538ed281b4d39114c9446))
* recognize wrapped Codex queue preview ([8ae8925](https://github.com/adambiggs/gangline/commit/8ae89258f2fcb9106860e858f56923ce8edc9d5a))
* show failed native turns without attributing stale hooks ([600fd10](https://github.com/adambiggs/gangline/commit/600fd103e874cc0e9d90067254142078f7f47271))
* treat vanished process stats as exited during teardown ([a9f95cf](https://github.com/adambiggs/gangline/commit/a9f95cfc26c1fcfffc8a3888101771eddd14980b))

## [1.2.0](https://github.com/adambiggs/gangline/compare/gangline-v1.1.0...gangline-v1.2.0) (2026-09-24)


### Features

* keep idle macOS teams moving with a launchd watchdog ([71a7ccd](https://github.com/adambiggs/gangline/commit/71a7ccd7aabdc4bd7cc5c8c0f6c7e618ba3cdf61))
* keep idle teams moving with a rearming watchdog ([aa3731c](https://github.com/adambiggs/gangline/commit/aa3731ce85f2c019dfd0fb6f4c789f22b4a9820c))


### Bug Fixes

* explain models flag and native hook input ([70b7d90](https://github.com/adambiggs/gangline/commit/70b7d906cea22e9873f0521038e872d4ab684aaa)), closes [#50](https://github.com/adambiggs/gangline/issues/50)
* identify context-band notices with concise system tags ([1b2abb6](https://github.com/adambiggs/gangline/commit/1b2abb61e6c5baf198b1faf22b45ddb2161fd38b))
* load and clean up macOS watchdog in user domain ([28d4cbb](https://github.com/adambiggs/gangline/commit/28d4cbbbd2586c0fef06ca2495ad5e2c6bec1426))
* recognize Claude tool activity in busy screens ([0d5ad11](https://github.com/adambiggs/gangline/commit/0d5ad1160165be20b7a7d01b70a531cf55355b39)), closes [#48](https://github.com/adambiggs/gangline/issues/48)
* refuse hitches beside unregistered team panes ([bd8d1fb](https://github.com/adambiggs/gangline/commit/bd8d1fb3db584c113eb193f09332cc337f79d58a)), closes [#47](https://github.com/adambiggs/gangline/issues/47)
* report reconciled send receipts before returning ([783c596](https://github.com/adambiggs/gangline/commit/783c596a39db0425d62834eba277f8448af31182))
* tolerate missing composer frames after pasting input ([82e19c2](https://github.com/adambiggs/gangline/commit/82e19c27c57cef92d5e77158d0a9747b809bf09f))
* use persisted short tokens for message envelopes ([2ff22b7](https://github.com/adambiggs/gangline/commit/2ff22b7c32845d2fd61d0407e2fb96089e636d58))

## [1.1.0](https://github.com/adambiggs/gangline/compare/gangline-v1.0.0...gangline-v1.1.0) (2026-09-24)


### Features

* **limits:** query account usage without a running team ([aa274d3](https://github.com/adambiggs/gangline/commit/aa274d3e9cf0d9990df9ba683fdde6bcc0de8fe9))


### Bug Fixes

* **compact:** preserve the author of custom resume notes ([2d6c3e0](https://github.com/adambiggs/gangline/commit/2d6c3e0b551c8d374d70f52944bc2bfbee66867b))
* **compact:** state when the resume note arrives, and deliver it first ([33fa60d](https://github.com/adambiggs/gangline/commit/33fa60de84f1ad2d60c65802395bc8e6e83d5168)), closes [#38](https://github.com/adambiggs/gangline/issues/38)
* **context:** band notes tell the agent to compact itself ([0b99ec7](https://github.com/adambiggs/gangline/commit/0b99ec76b2fccd2b42fe9919e1fe52d0277d81ba)), closes [#38](https://github.com/adambiggs/gangline/issues/38)
* **context:** shorten notice IDs without reordering crossings ([77bb97d](https://github.com/adambiggs/gangline/commit/77bb97dfc1664983d42ea4f0a7db60712baf63f0))
* **contract:** name the commands an agent uses ([b61c1e8](https://github.com/adambiggs/gangline/commit/b61c1e8a8fe04710d49a0c776b40ba98b0e3497b)), closes [#38](https://github.com/adambiggs/gangline/issues/38)
* **hitch:** reject mismatched native resume identities before launch ([16df117](https://github.com/adambiggs/gangline/commit/16df117d6fd999a219765a9b7b4054c0718434c5))
* send Gangline's own messages under a gangline sender ([aebfb8a](https://github.com/adambiggs/gangline/commit/aebfb8a6c0d78e4f386ad441c7f5ac8a5773d9b6)), closes [#38](https://github.com/adambiggs/gangline/issues/38)
* **site:** keep playback controls below the demo text ([66312d9](https://github.com/adambiggs/gangline/commit/66312d9a0bc78273823a971a1d264354000415a0))
* **site:** make the native team demo readable on phones ([d41368c](https://github.com/adambiggs/gangline/commit/d41368c973bd9b4c64fea7b6170cd43fc3d3c4e9))
* **site:** record the demo as a landscape terminal ([76eb867](https://github.com/adambiggs/gangline/commit/76eb8677f076b0b73a73489f8b06b17001cf116c))
* **site:** restore the live terminal demo recording ([3c5fec6](https://github.com/adambiggs/gangline/commit/3c5fec644d552eb494a729bf498719b404290ba6))
* **store:** retry interrupted directory watch calls ([7884f66](https://github.com/adambiggs/gangline/commit/7884f660afde833a341319ee0d9e8f9ce5645df7))
* **tmux:** skip every non-CSI escape in a captured pane ([7a6969a](https://github.com/adambiggs/gangline/commit/7a6969aea958b6f33aced8633b7bfa6866ea2f4b))

## [1.0.0](https://github.com/adambiggs/gangline/compare/gangline-v0.9.0...gangline-v1.0.0) (2026-09-23)


### ⚠ BREAKING CHANGES

* Gangline no longer ships the v0.9 shell runtime, shell collars, or shell test surface.

### Features

* **capture:** render parsed pane screens ([f11a2c9](https://github.com/adambiggs/gangline/commit/f11a2c91cd9177ab66f704a792dc89da5c1d1711))
* **cli:** add offline replay entry point ([80f9385](https://github.com/adambiggs/gangline/commit/80f93855e9ff152c49b49be2066153cc79131ca5))
* **cli:** attach through the tmux backend ([b71487a](https://github.com/adambiggs/gangline/commit/b71487a2fb8daa61c9f1adc44fd452ffcb5f4ba8))
* **cli:** drive durable team lifecycle ([e09ce6f](https://github.com/adambiggs/gangline/commit/e09ce6f8b33977fbc69ba9767160f678f20e061a))
* **cli:** establish the binary command surface ([a6d78e6](https://github.com/adambiggs/gangline/commit/a6d78e692fd76b5fdf134bf26e61bc93611dfd95))
* **cli:** parse durable schedules without shell helpers ([90b0fe9](https://github.com/adambiggs/gangline/commit/90b0fe98dbe736da589b3e6accb8fcbda23ffb0f))
* complete Go runtime cutover ([773d135](https://github.com/adambiggs/gangline/commit/773d1351c9415b346874d400de3db5a69280deed))
* **config:** load strict operator settings ([732367a](https://github.com/adambiggs/gangline/commit/732367ac1864a5513924564aef888405dd6e0673))
* **core:** model complete team lifecycle ([adda0cf](https://github.com/adambiggs/gangline/commit/adda0cf7447f930afeed0de8e6191eaa86ef52fc))
* **docs:** add the shared line field background ([55ac381](https://github.com/adambiggs/gangline/commit/55ac3813e454d2c155ddfa281f456ac41cff360c))
* establish Go core and package boundaries ([5d74035](https://github.com/adambiggs/gangline/commit/5d74035e5f38a3bacfed110b8d16eee33c831664))
* **harness:** define complete collar contracts ([f6ce4b5](https://github.com/adambiggs/gangline/commit/f6ce4b5b1ea9b62c5ad975439dfa49ed6bcc5777))
* **harness:** implement collar primitives and probes ([ff38999](https://github.com/adambiggs/gangline/commit/ff3899994fd02f5fa922929fd66f5d401bab8723))
* **hitch:** apply operator launch arguments by collar ([fba3546](https://github.com/adambiggs/gangline/commit/fba3546d7870b4c4621ce6a2e0f49d9aeaf30273))
* hold delivery during approval prompts ([1a9db3e](https://github.com/adambiggs/gangline/commit/1a9db3e8c8fea19f35e757c6283bc55bd3cd6b8d))
* **models:** discover native harness choices ([f7323de](https://github.com/adambiggs/gangline/commit/f7323defef88aeb9e8e1f264c2afe2684dd05248))
* **observe:** record normalized native lifecycle and context evidence ([1087039](https://github.com/adambiggs/gangline/commit/1087039d6feec7252e180a82d9eb7525f06a46f5)), closes [#21](https://github.com/adambiggs/gangline/issues/21) [#22](https://github.com/adambiggs/gangline/issues/22)
* **send:** render attributed envelopes ([a086efa](https://github.com/adambiggs/gangline/commit/a086efa491a6a5ebb62c30da8e2348a6f2dbeb91))
* **store:** persist replayable team events ([92f7c21](https://github.com/adambiggs/gangline/commit/92f7c21fe6d81fcdd76f8c70a338ed6ba67fa15b))
* **substrate:** add tmux backend ([26e963c](https://github.com/adambiggs/gangline/commit/26e963cb13c7b6989e81caffa8cbbab37767dd02))
* **substrate:** expose tmux session lifecycle ([7faae26](https://github.com/adambiggs/gangline/commit/7faae2603d6437091fe61aa06bf1fbf25c0af12d))
* **teams:** list recorded team state ([d343d28](https://github.com/adambiggs/gangline/commit/d343d28a3d1739a603a2085e657f1c7a9c2ad4ce))
* wait on event log boundaries ([af2d685](https://github.com/adambiggs/gangline/commit/af2d6851d5eb4a2b4a40bd1e4c6d9194d9b60ac4))


### Bug Fixes

* **cli:** keep observation gates exhaustive ([7ac60f5](https://github.com/adambiggs/gangline/commit/7ac60f50d9f7814729e821017446cee486d9761d))
* **cli:** make recovered effects idempotent ([1fb5d67](https://github.com/adambiggs/gangline/commit/1fb5d6786d2f309feafe6920883730152e471e24))
* **cli:** reject ignored recovery arguments ([5fc031d](https://github.com/adambiggs/gangline/commit/5fc031d254df0294d7d7331a52f46863156ff305))
* **collar:** bound native compatibility probes ([c3861dd](https://github.com/adambiggs/gangline/commit/c3861dd00e61545eb6c0ffdb9b91d89d344df4d2))
* **collar:** verify installed harness startup and hooks ([7b503c8](https://github.com/adambiggs/gangline/commit/7b503c89793bd107ad2dc32e9f3193bfc3068179))
* compact displayed context token counts ([cef21e1](https://github.com/adambiggs/gangline/commit/cef21e1344a67a2b8cb86d016429ca54615d703c))
* **context:** restore band notes through normal delivery ([c314ec0](https://github.com/adambiggs/gangline/commit/c314ec0aa1ea503fadf6656a583a0b8953145ab4))
* **core:** retain work until drop succeeds ([59cf781](https://github.com/adambiggs/gangline/commit/59cf781545c7661c792af68fb898a3f7114f2015))
* deliver startup prose once with observed attribution ([d78fe0a](https://github.com/adambiggs/gangline/commit/d78fe0a79c94b51e0814f854e8206ab64d0e4730))
* **delivery:** accept bare Claude paste witnesses ([659ad77](https://github.com/adambiggs/gangline/commit/659ad77e9c89ae77adacaa181178d3db6854633f))
* **delivery:** bracket multiline native input ([e738f93](https://github.com/adambiggs/gangline/commit/e738f935e22bb3a11c4890ea85d001a0b5edcbad))
* **delivery:** preserve native receipts and recover stalled turns ([c59b6e6](https://github.com/adambiggs/gangline/commit/c59b6e6c88f024575354eeab8e9e7dd7e10fa8f0)), closes [#18](https://github.com/adambiggs/gangline/issues/18) [#20](https://github.com/adambiggs/gangline/issues/20) [#23](https://github.com/adambiggs/gangline/issues/23)
* **delivery:** release deferred sends at native boundaries ([566199b](https://github.com/adambiggs/gangline/commit/566199be4f9a2f3832132da4b186e1af8cee876c)), closes [#10](https://github.com/adambiggs/gangline/issues/10) [#11](https://github.com/adambiggs/gangline/issues/11) [#12](https://github.com/adambiggs/gangline/issues/12) [#13](https://github.com/adambiggs/gangline/issues/13) [#15](https://github.com/adambiggs/gangline/issues/15)
* **delivery:** retain live sends until acceptance or drop ([b6d695b](https://github.com/adambiggs/gangline/commit/b6d695b10b7e2fc1ed5925242e4512f7826d6b3b)), closes [#15](https://github.com/adambiggs/gangline/issues/15)
* **delivery:** retry durable queues at safe composers ([6f84a7a](https://github.com/adambiggs/gangline/commit/6f84a7aaca025374c826114c9afffe0b0156cb92))
* **delivery:** settle native composers before submit ([e37fd6f](https://github.com/adambiggs/gangline/commit/e37fd6f7274b9123c450973bb4476f2515889251))
* **delivery:** submit steering input during native turns ([ae23277](https://github.com/adambiggs/gangline/commit/ae23277e2783b1a3fecbde4906e665c7f033eb3f)), closes [#19](https://github.com/adambiggs/gangline/issues/19)
* **delivery:** verify normalized Claude paste witnesses ([7c0916a](https://github.com/adambiggs/gangline/commit/7c0916a8df69b1fcf2aec0ef0cd88af142036295))
* **delivery:** wait for native composer paint ([476475e](https://github.com/adambiggs/gangline/commit/476475e5730227ce87924370b2c7ff27eaa55eea))
* distinguish native queue acceptance and report drop cleanup ([489be2f](https://github.com/adambiggs/gangline/commit/489be2f7f6280203f2608d81940f4a403c8259df))
* distinguish unsubmitted composer input from native work ([57958b5](https://github.com/adambiggs/gangline/commit/57958b5c1f2cc8ec64cd89c074d210a72874d1fd))
* emit Claude context notes at the early checkpoint ([b6935e1](https://github.com/adambiggs/gangline/commit/b6935e1a16260b244d057e8ff2ea74fded4da744))
* **hooks:** delegate pre-push to the global hook ([6e8a5b0](https://github.com/adambiggs/gangline/commit/6e8a5b05decfc75300aed56a4d317f0201a6cc69))
* **hooks:** serialize native observations and record failures ([d7dc863](https://github.com/adambiggs/gangline/commit/d7dc863d1229854239732cf7504cbf7f1d995884))
* **install:** build from the release module root ([def3b43](https://github.com/adambiggs/gangline/commit/def3b43904a8c80c675591e9f7e2463fa10b8f2f))
* **installer:** install published release layouts ([2c165dd](https://github.com/adambiggs/gangline/commit/2c165ddc1b757a36bf1208dac6a75474b52d6eb3))
* **lifecycle:** preserve agent generations and state marks ([3fc401d](https://github.com/adambiggs/gangline/commit/3fc401d0e008f75b585577ad44cfdbd68310be57))
* observe current activity and preserve queued compaction ([c254651](https://github.com/adambiggs/gangline/commit/c254651278bdc0a69a6c813e84f3334097d4bc4e))
* reap detached pane descendants ([6ce01fd](https://github.com/adambiggs/gangline/commit/6ce01fd272b9c4c4eb90df5bdc1e38626841f4bf))
* recognize expanded Claude input before submitting ([f616e85](https://github.com/adambiggs/gangline/commit/f616e855429a77cc534f3e0412725494e2a9b211))
* record vanished panes ([ab96d4a](https://github.com/adambiggs/gangline/commit/ab96d4a2f2351333edeea2e4698319efcd1e09d7))
* recover original startup input and reconcile late submit witnesses ([52e5a8a](https://github.com/adambiggs/gangline/commit/52e5a8ab53f0db690144967263e8e48b5a5201dc))
* report collar probe launch failures ([3b72e78](https://github.com/adambiggs/gangline/commit/3b72e789c9fabb4b85b665d5434676864b5dba85))
* report native retry menus as input blockers ([53f7d5d](https://github.com/adambiggs/gangline/commit/53f7d5d1efb0eb43c482e303e31fbc9eb3de992d))
* report probe failures as unknown activity ([bc2690b](https://github.com/adambiggs/gangline/commit/bc2690bc35c86ded6401472812e786b7be5762a3))
* require native compaction completion before resuming ([ae6b20e](https://github.com/adambiggs/gangline/commit/ae6b20e2a6ab2fed2247c75e5c834490c52b607c))
* restore published installs and portable release checks ([5363e44](https://github.com/adambiggs/gangline/commit/5363e4459ac5839b229b6b34efe94f0f16c9d2a4))
* retry startup delivery and resolve capture targets ([0af5766](https://github.com/adambiggs/gangline/commit/0af57663963b2f76bd024df04f189a52d7f63c49)), closes [#7](https://github.com/adambiggs/gangline/issues/7) [#8](https://github.com/adambiggs/gangline/issues/8)
* **send:** reject unsafe message bytes ([7b7fbe0](https://github.com/adambiggs/gangline/commit/7b7fbe03e292ad1b1d139bf7f50445b824ca579b))
* **startup:** register the lead after native readiness ([1325a6c](https://github.com/adambiggs/gangline/commit/1325a6ca1a326e771c7d221d91b150c7b3b8f110))
* **store:** replay event state across binary upgrades ([4551501](https://github.com/adambiggs/gangline/commit/4551501bf34fe55a57a1229fabe0043ecb3ee433)), closes [#14](https://github.com/adambiggs/gangline/issues/14)
* verify harness foreground process before input ([a2d3b74](https://github.com/adambiggs/gangline/commit/a2d3b744be99da3d0e384fc3fdc0dba7ef740d0a))

## 0.9.0 (2026-09-19)

- Start, observe, and connect CLI-agent teams in tmux with attributed, verified message delivery.
