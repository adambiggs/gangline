# Changelog

## [2.3.0](https://github.com/adambiggs/gangline/compare/gangline-v2.2.0...gangline-v2.3.0) (2026-08-25)


### Features

* **at:** deliver a message when the clock passes ([dce8ea1](https://github.com/adambiggs/gangline/commit/dce8ea197908413c6293267381c829a5e0719d30)), closes [#112](https://github.com/adambiggs/gangline/issues/112)
* **cli:** add agent rename command ([7d0edf1](https://github.com/adambiggs/gangline/commit/7d0edf1a16018e30e3ae0d96457a0425cdb7cb8b))
* **codex:** refuse a launch that would stop on the hooks-review menu ([804ce8b](https://github.com/adambiggs/gangline/commit/804ce8b36a180ff1fd77903064415d66861320a5)), closes [#142](https://github.com/adambiggs/gangline/issues/142)
* **collars:** attribute telemetry per agent ([4123e24](https://github.com/adambiggs/gangline/commit/4123e2457ce57acc32fa2a75533a1492cac638e7))
* **collars:** mark which collars an agent can be resumed onto ([3b25493](https://github.com/adambiggs/gangline/commit/3b25493445e01a623b4177d573319bc94fc19fd3))
* **hitch:** guard an agent from ending its own team's tmux server ([bc94ce3](https://github.com/adambiggs/gangline/commit/bc94ce3fb5185b770b1242fbc3668585c307089a))
* **hitch:** warn when live agents were hitched from another gang path ([5d7152a](https://github.com/adambiggs/gangline/commit/5d7152afe3e4c18d927c4d7491afde0e71d97d50))
* **lights:** default Codex thresholds by window ([f5ab80d](https://github.com/adambiggs/gangline/commit/f5ab80d3f25bede8228f9d0ce460f9891ad510d0))
* **lights:** default context lights to the collar's per-model thresholds ([b6b58a8](https://github.com/adambiggs/gangline/commit/b6b58a8b6d48fe0247429554de7f62fb69a96605))
* **opencode:** stamp a resumable session identity ([b64af2d](https://github.com/adambiggs/gangline/commit/b64af2d9a857536abdab37af360597b06bd25a14)), closes [#144](https://github.com/adambiggs/gangline/issues/144)
* **send:** mark a sender Gangline did not observe as self-declared ([9e8f6a5](https://github.com/adambiggs/gangline/commit/9e8f6a58f133822680289baa18c97d1b23d67e4d)), closes [#131](https://github.com/adambiggs/gangline/issues/131)
* **status:** report when an agent last ran a tool ([cadc46f](https://github.com/adambiggs/gangline/commit/cadc46ffa9e81760e73930147bf4c1a66d5bf607))
* **teams:** record and enumerate the socket each team runs on ([df7b9eb](https://github.com/adambiggs/gangline/commit/df7b9ebd000ce35487c5390c66e34349ea2a88b6))
* **up:** start the tmux server inside a gangline-owned unit ([6483004](https://github.com/adambiggs/gangline/commit/6483004e0ea2f8bb850ad8eea1d6a1a628e6fbd3))


### Bug Fixes

* **adopt:** refuse windows with no running pane ([a7e6977](https://github.com/adambiggs/gangline/commit/a7e6977f81fa78b2fee13e2180794bfedc354cfd))
* **adopt:** separate registration from observation ([2c757fe](https://github.com/adambiggs/gangline/commit/2c757fe6569b4367b1a03bcc635208d6b22c0aab))
* **adopt:** validate before registering the window ([8b4c62d](https://github.com/adambiggs/gangline/commit/8b4c62d4af75aa1a4e9bbc9c47d3dfa5e5ccc690))
* **adopt:** validate spool identity before registration ([521b659](https://github.com/adambiggs/gangline/commit/521b65936502c37c3e680736e79a1ee38441fdd4))
* **at:** answer a bare invocation with the synopsis ([062d0a8](https://github.com/adambiggs/gangline/commit/062d0a84e4407485b2905f50931c2069141a9618)), closes [#112](https://github.com/adambiggs/gangline/issues/112)
* **auto-resume:** disclose discarded wake turn ([876b721](https://github.com/adambiggs/gangline/commit/876b721fc3aad81047fb82a6c73e5cd3670cf17c))
* **claim:** surface event claim failures ([984e60c](https://github.com/adambiggs/gangline/commit/984e60c2d1c61d151bdaade959e311460fccf191))
* **claude-code:** give a selected subagent composer its own verdict ([ef3dc3d](https://github.com/adambiggs/gangline/commit/ef3dc3d9a9d4f8a0ca58ada774e26b65148e69f0)), closes [#129](https://github.com/adambiggs/gangline/issues/129)
* **claude-code:** read a titled session composer as the agent's own ([b057646](https://github.com/adambiggs/gangline/commit/b0576468d87e2c6dcb3dc042bba277793c95a09b)), closes [#152](https://github.com/adambiggs/gangline/issues/152)
* **claude-code:** recognise a dialog by its chrome and name it where delivery fails ([ff5fd71](https://github.com/adambiggs/gangline/commit/ff5fd719d3efbe7e43dfb7e239023cda1f8fcca2)), closes [#143](https://github.com/adambiggs/gangline/issues/143)
* **claude-code:** stop measuring the band an overlay is recognised by ([ec70e92](https://github.com/adambiggs/gangline/commit/ec70e9294a5597dfc0ba4d16a2baba26215841f5)), closes [#143](https://github.com/adambiggs/gangline/issues/143)
* **codex:** raise context-light defaults to 75%,90% ([d87b444](https://github.com/adambiggs/gangline/commit/d87b4441cab0262b13b314090688b80a143e5278))
* **collar:** a context pane that could not be read is not one with no readout ([05fb62c](https://github.com/adambiggs/gangline/commit/05fb62ca6230af26c49cc0b0ca8fabeb6f2556e4))
* **collar:** classify auto-mode setup as occupied ([d0741c8](https://github.com/adambiggs/gangline/commit/d0741c8e21a2872154fb0ed1075f180a861e486a))
* **collar:** clear every optional collar function when a collar loads ([1912fcc](https://github.com/adambiggs/gangline/commit/1912fccaf7656190f46e70d8db68d123b25eb157))
* **collar:** clear the overlay reader with the other optional collar functions ([3d2e8bc](https://github.com/adambiggs/gangline/commit/3d2e8bcb4d60bb437ac6de0fb95ab15177941e82))
* **collar:** import the Codex config reader where it is used ([9d508e3](https://github.com/adambiggs/gangline/commit/9d508e3177e77c2e827b2c0e6eddbf3847404ce2))
* **collar:** resolve configured model for effort validation ([3b68fa8](https://github.com/adambiggs/gangline/commit/3b68fa8ef93133fc8a5eaa63288239a2dd4e60a1))
* **collars:** tell an explicit-id resume from no resume at all ([fb07b6b](https://github.com/adambiggs/gangline/commit/fb07b6bfe47c030bf25ba2c91e76a74367c2b555))
* **collar:** surface a Claude turn killed mid-stream ([24a815c](https://github.com/adambiggs/gangline/commit/24a815c473dc04aa204b56c7e413b67a19ebff1b))
* **collar:** surface terminal Claude 529 failures ([3eacb49](https://github.com/adambiggs/gangline/commit/3eacb494a2165a416f2f6069daad3522060aa7ea))
* **compact:** serialize deferred dispatch ([299cf58](https://github.com/adambiggs/gangline/commit/299cf588f81f865b0af6067b2afbe2d27ad68275))
* **context:** distinguish transient beacon misses ([4c10b52](https://github.com/adambiggs/gangline/commit/4c10b52e42ce2ee86d2c0251350d48f1c35d66ba))
* **down:** preflight every spool before teardown ([9195d88](https://github.com/adambiggs/gangline/commit/9195d88b395e726e6fa68b33476a232205c6089f))
* **drop:** a stamped session id is not always a way back ([9249c21](https://github.com/adambiggs/gangline/commit/9249c213cfe58aeb38a03e5a7be4199f590ae4ed))
* **flush:** recover a parked message against the body, not a second rendering ([8b82041](https://github.com/adambiggs/gangline/commit/8b82041da32a16cae1801b3b428cc7233fb40439)), closes [#124](https://github.com/adambiggs/gangline/issues/124)
* **hitch:** bound the wait on a first-run prompt nobody answers ([b44386e](https://github.com/adambiggs/gangline/commit/b44386e4192e790373a81f78f252465a139edf0c))
* **hitch:** the two halves of resuming fail differently, and are warned so ([4a255b0](https://github.com/adambiggs/gangline/commit/4a255b00ee6790676e2a8ed80ae8856d8df7855d))
* **hitch:** validate spool root before launch ([3015c80](https://github.com/adambiggs/gangline/commit/3015c802d630b0176d6b9eab43f6dd85952d5914))
* **hitch:** validate startup budgets before launch ([43c796c](https://github.com/adambiggs/gangline/commit/43c796c21142527bd1be24c819e625e8b3273119))
* **hooks:** refuse literal blank-line escapes ([1b3ebb2](https://github.com/adambiggs/gangline/commit/1b3ebb2677ca66ffec329455d4911ef18d5511b5))
* **install:** require tmux 3.2 ([91bd626](https://github.com/adambiggs/gangline/commit/91bd62605d36b0c4d60c5c3b0db63211baa08cab))
* **opencode:** read the composer's own prompt as an empty box ([e5d7799](https://github.com/adambiggs/gangline/commit/e5d7799175814bf714155817b764746aa412d82b))
* **roster:** an agent gang could not read does not end the listing ([c4d3722](https://github.com/adambiggs/gangline/commit/c4d37223fb7a609c443a37ff8b7c5d4578e99550))
* **scope:** bridge sandboxed hitches to user manager ([66b8c0a](https://github.com/adambiggs/gangline/commit/66b8c0a86c866012d01727db5d3e1569798493d0))
* **send:** name the native screen a paste did not reach ([16a445c](https://github.com/adambiggs/gangline/commit/16a445c00b3c41b4e77e4a0dc1d0e0a1a46f94d9))
* **send:** report a typed-but-unconfirmed delivery as its own verdict ([1885a07](https://github.com/adambiggs/gangline/commit/1885a075df701265faa21554bfbfc299cdd65679))
* **send:** settled is two readings that were taken, not two that match ([4fdc81a](https://github.com/adambiggs/gangline/commit/4fdc81aed355c0a6086f3268ebe214f4fb1a4807))
* **send:** state the evidence a failed paste already holds ([90dbb3e](https://github.com/adambiggs/gangline/commit/90dbb3e7228b4f1799d0698f073c121187892163))
* **spool:** account for spools whose window is gone ([3f8e942](https://github.com/adambiggs/gangline/commit/3f8e9420a39bfd129a26c9deb439b3b5d310ae78))
* **spool:** name a staged body whose sender never committed it ([5d3db70](https://github.com/adambiggs/gangline/commit/5d3db70d9fcca963b126761f4ff7aaf22b52edce))
* **spool:** preflight teardown archive eligibility ([c7b02a4](https://github.com/adambiggs/gangline/commit/c7b02a42326db7a9d3cb88af6a8fc61ea994f7d9))
* **turn:** offer a drain at an expired turn bracket ([ec61e71](https://github.com/adambiggs/gangline/commit/ec61e71bcc4c175122c47d19021e071ae55e07ac)), closes [#123](https://github.com/adambiggs/gangline/issues/123)
* **wait:** replace GNU timeout with portable deadline ([3d92ecd](https://github.com/adambiggs/gangline/commit/3d92ecd037c3e5a373495f053e3fa9bbee5f1340))


### Performance Improvements

* **collar:** bound Claude auto-resume transcript reads ([d4ed320](https://github.com/adambiggs/gangline/commit/d4ed3209d726664e2cad6068b00b646da380a032))
* **resolve:** sanitize an agent name only where it is printed ([30adfe1](https://github.com/adambiggs/gangline/commit/30adfe14e126bc0de91fc170b098524783b6c734))
* **test:** restore the mandatory gate runtime ceiling ([74c2129](https://github.com/adambiggs/gangline/commit/74c2129a3304cbf399b7c8d1e1905bf9a389ea0d))

## [2.2.0](https://github.com/adambiggs/gangline/compare/gangline-v2.1.0...gangline-v2.2.0) (2026-08-18)


### Features

* **hitch:** give each agent its own oomd-killable cgroup ([461ba29](https://github.com/adambiggs/gangline/commit/461ba29aa46f509c38caaac5163bd70af81c97d9))
* **usage:** arm a provider-reset wake at a declared threshold ([868c147](https://github.com/adambiggs/gangline/commit/868c14763bf248a543a9963292df66d97ba06d9d))
* **usage:** recover one dead Claude stream ([829d48b](https://github.com/adambiggs/gangline/commit/829d48b5e2befa6d2e1afaf197f707d9c20e0e89))
* validate launch models and expose bricked turns ([1e5f6a7](https://github.com/adambiggs/gangline/commit/1e5f6a77ae3b26e0c85feac34ca10cb238fffd26))


### Bug Fixes

* **collar:** read a pane that could not be read as unreadable, not as no box ([dd02da0](https://github.com/adambiggs/gangline/commit/dd02da0baef7543139060d00e63c90179373a3bd))
* **compact:** claim the self-compaction request before touching its continuation ([c1933bb](https://github.com/adambiggs/gangline/commit/c1933bb31365d01e4add77b280f7fe8df7751078)), closes [#127](https://github.com/adambiggs/gangline/issues/127)
* **diagnostics:** replace every byte a terminal obeys, not every control character ([ba7281d](https://github.com/adambiggs/gangline/commit/ba7281dc5f5132d3415acde55a5ad0945d128059))
* **diagnostics:** sanitize what a refusal prints, once, where they all print ([fddf915](https://github.com/adambiggs/gangline/commit/fddf9150696d5390d83db567cc69a911351be805)), closes [#136](https://github.com/adambiggs/gangline/issues/136)
* **flush:** refuse against a running turn instead of diagnosing a stuck queue ([8016bf2](https://github.com/adambiggs/gangline/commit/8016bf2f989efb270c6b7eb70e1168ffe0d08c48)), closes [#122](https://github.com/adambiggs/gangline/issues/122)
* **hitch:** keep tmux's own refusal out of a dead launch's diagnosis ([4372988](https://github.com/adambiggs/gangline/commit/4372988cb16b988018349d3ac59296c273d456a8))
* **hitch:** name the dead window a hitch is refusing over ([b883253](https://github.com/adambiggs/gangline/commit/b8832536f95c2625abd8f9a189189e6ac8163304))
* **hitch:** read a pane that cannot be read as unknown, not as empty ([f76adf1](https://github.com/adambiggs/gangline/commit/f76adf19f4eed0188c61873b9cad61913fbfba27))
* **hitch:** read every pane before calling a window empty ([053e6a1](https://github.com/adambiggs/gangline/commit/053e6a18ee35ef4dbe14055c97e78f4803822a26))
* **hitch:** refuse a scope name an earlier agent still holds ([c349984](https://github.com/adambiggs/gangline/commit/c349984ba13b9918f3bd2ae7ad6223ef151902b8))
* **hitch:** report a launch that died instead of waiting out its boot ([7f9c2ed](https://github.com/adambiggs/gangline/commit/7f9c2ed9770133d5e3aee30fa82d0989dbf3181b))
* **hitch:** silence a failed read where death explains it, not everywhere ([2a46450](https://github.com/adambiggs/gangline/commit/2a46450fda82c3bfc2af9c50ede729d57ee07765))
* **install:** stop git explaining detached HEAD after the release clone ([22a32ab](https://github.com/adambiggs/gangline/commit/22a32ab44680fe260ae21d448635a292045515f1))
* **send:** a box that did not hold still is not a settled one ([ef86841](https://github.com/adambiggs/gangline/commit/ef86841e64b323fad366c9d3bebbb5448c853a75))
* **send:** a composer gang could not read is not a settled one ([8d8f611](https://github.com/adambiggs/gangline/commit/8d8f611c9fb6aad1f66001036b5f1df88c8e5950))
* **send:** commit a superseding message before retiring what it replaces ([3fde04b](https://github.com/adambiggs/gangline/commit/3fde04bb800e0f85bfa417cba5f33463e7038928)), closes [#126](https://github.com/adambiggs/gangline/issues/126)
* **send:** make a supersession retire and land together or not at all ([a1362dc](https://github.com/adambiggs/gangline/commit/a1362dc04264ec45711b60a73e200880aa6bae3d))
* **send:** name --stdin when a body is passed as an argument ([c764e38](https://github.com/adambiggs/gangline/commit/c764e3869a7a00994cdfa0183bbbc915a85547eb)), closes [#117](https://github.com/adambiggs/gangline/issues/117)
* **shell:** restore portable macOS checks ([cbcbbae](https://github.com/adambiggs/gangline/commit/cbcbbae3352b7458530f35b907c838511bb2d4c1))
* **spool:** archive every child, and refuse to delete what teardown cannot ([16a98e5](https://github.com/adambiggs/gangline/commit/16a98e57d37b0cba3fcd2fd5849c24e38f62153a)), closes [#141](https://github.com/adambiggs/gangline/issues/141)
* **spool:** give every archived child a name of its own ([0116188](https://github.com/adambiggs/gangline/commit/0116188d54a1cd60dbcbb57939803f57dc6fd693))
* **spool:** mint a spool identity past the ones already on disk ([d080b91](https://github.com/adambiggs/gangline/commit/d080b912d51313f296684d723d356f5cf13d802f)), closes [#140](https://github.com/adambiggs/gangline/issues/140)
* **spool:** report an unproducible spool stamp instead of dying silently ([03ebb0a](https://github.com/adambiggs/gangline/commit/03ebb0a4edbbd26ca8e2e5604d13876b458b1215))
* **spool:** reserve a spool identity where it is published ([3da8c10](https://github.com/adambiggs/gangline/commit/3da8c106409d262eec02df32c9d3c480321ac64d))
* **state:** check both decay witnesses instead of assembling them in a printf ([cb0afa9](https://github.com/adambiggs/gangline/commit/cb0afa9fb8e370ae2aaecd11d33f8cc009ee11a6))
* **state:** classify a composer reading in one place, and keep unknown unknown ([202872c](https://github.com/adambiggs/gangline/commit/202872c3fde16080df58f920efdab2f8beacd524))
* **teardown:** part with an id a relaunch can use, from drop and from down ([7997ae9](https://github.com/adambiggs/gangline/commit/7997ae9cd845df1a43468415954b34d940954063)), closes [#113](https://github.com/adambiggs/gangline/issues/113)
* **test:** open a death fifo from the pane's launch, not a typed line ([c0c0556](https://github.com/adambiggs/gangline/commit/c0c055687968358fa377d6ee50c6d160a3dea1aa))
* **test:** order pane deaths behind the pane's own descriptors ([82159c1](https://github.com/adambiggs/gangline/commit/82159c16da1e4dcf4f6677db0479cffc5de11ef6))
* **usage:** preserve operator wake decisions ([5349312](https://github.com/adambiggs/gangline/commit/534931272137aed314e65d0f03f2b2db159c0b30))
* **usage:** recover dead provider wakes ([47b195e](https://github.com/adambiggs/gangline/commit/47b195e83f23d7920582a4b4fa3effabf9ff5920))

## [2.1.0](https://github.com/adambiggs/gangline/compare/gangline-v2.0.0...gangline-v2.1.0) (2026-08-14)


### Features

* install tagged releases and streamline verification ([818215a](https://github.com/adambiggs/gangline/commit/818215ab2186912833026c379e7a471326c1394e))


### Bug Fixes

* **drop:** sanitize missing target errors ([fc4df09](https://github.com/adambiggs/gangline/commit/fc4df098e2b1fe972721aa87f7b019a9240d4bfc))
* **identity:** sanitize resolver diagnostics ([453fa27](https://github.com/adambiggs/gangline/commit/453fa27fb454c6b5063b4a7a52b0b8db768dbfb1))
* **release:** protect upgrades and publication ([e12d0b8](https://github.com/adambiggs/gangline/commit/e12d0b87e63baefda089205dc15bd7c0d6874d05))


### Performance Improvements

* **hooks:** meet the fast push boundary ([2c0109b](https://github.com/adambiggs/gangline/commit/2c0109bf3b44155595f0f13a0c2be3d1783c5f69))


### Miscellaneous Chores

* release 2.1.0 ([bc99ed0](https://github.com/adambiggs/gangline/commit/bc99ed09c6e006cb615379d0c7d7b9ae95d15a85))

## [2.0.0](https://github.com/adambiggs/gangline/compare/gangline-v1.0.0...gangline-v2.0.0) (2026-08-14)


### ⚠ BREAKING CHANGES

* **hooks:** x` alone at the end of a message is parsed
* **help:** `gang` with no arguments exits 0 and prints a getting-started page on stdout instead of printing the command inventory on stderr and exiting 1. Scripts that read the inventory from a bare invocation must call `gang help` or `gang --help`.
* **gang:** gang usage is removed; use gang limits. Collars declaring GANG_DIALOGS, GANG_DIALOG_LINES_<id>, GANG_DIALOG_HITCH_DIR_TRUST, GANG_USAGE_CMD, GANG_USAGE_CONFIRM_KEY, GANG_USAGE_RENDER or GANG_USAGE_DISMISS_KEY keep working, but those declarations are no longer read. A first-run prompt that used to be answered automatically now waits for the operator: attach and answer it, and hitch delivers the parked contract.
* **gang:** every pre-rename name listed above is removed. Rename GANG_PROFILE to GANG_COLLAR and GANG_PROFILES to GANG_COLLARS in configuration and environment, GANG_TEST_PROFILES to GANG_TEST_COLLARS, profile_input, profile_context and profile_session_id to collar_input, collar_context and collar_session_id in custom collars, -p/--profile to -c/--collar, gang profiles to gang collars, gang cutoff to gang curfew, and gang spawn to gang hitch. Drop send --spool, which has been the default since 1.0. A team running across the upgrade must be re-hitched rather than migrated in place.
* **hooks:** `
* **config:** remove GANG_ACTIVITY_LIMIT and GANG_CLEAR_PRESSES from Gangline config files; they remain environment-only implementation seams.
* **occupancy:** remove GANG_OCCUPIED_LIMIT from Gangline config files; timed occupancy decay no longer exists.
* tools/pii-scan is gone and the pii-scan CI job with it. A clone that invoked either has no in-repo replacement; use the operator's installed Snubline scanner. Repository push and pull-request CI no longer scan for PII shapes at all.

### Features

* **compaction:** let a harness declare its compaction instead of gang guessing ([ca9f731](https://github.com/adambiggs/gangline/commit/ca9f73122d404315e11e67f8c8d6a0ae7fe1b434))
* **compaction:** tell the summariser what a lane needs to keep ([9346f7c](https://github.com/adambiggs/gangline/commit/9346f7cfa81aff5424bd45e98e7228e46e867bfd))
* **compact:** land every compaction on a turn, not an empty composer ([f13e296](https://github.com/adambiggs/gangline/commit/f13e2964d6b4a3195b68547e61b0f10d451c9e6b))
* **contract:** make brevity and direct peer traffic contract terms ([11b77d6](https://github.com/adambiggs/gangline/commit/11b77d6b178fd3f9b5905b966ac50d3ee46c6c78))
* **contract:** put the contract in the system prompt where a collar has one ([eac1447](https://github.com/adambiggs/gangline/commit/eac1447d7b1d99ef4b4878c33728b22f491c9ad0))
* **contract:** send agents to a contract file instead of pasting it ([8863837](https://github.com/adambiggs/gangline/commit/8863837ae7209530bb60fa2ea49134e68a11a184))
* **gang:** add native turn barriers ([d6343fa](https://github.com/adambiggs/gangline/commit/d6343fab63d2bff05b9a5b811efb2da2fbe6fd39))
* **gang:** add provider usage limits and reset waits ([42f0add](https://github.com/adambiggs/gangline/commit/42f0add450dd60a1bd4261573ae99c45e32ac253))
* **gang:** default mail, flush, interrupt and usage to the calling window ([cadb911](https://github.com/adambiggs/gangline/commit/cadb91190e11ff15351ba0ce263b203b367c1224))
* **gang:** delete known-dialog auto-answering and the composer usage page ([6683d92](https://github.com/adambiggs/gangline/commit/6683d923b1513b7eb9acd0ac51dd90d4c60b8b4e))
* **gang:** explain collar state matches ([f7d2158](https://github.com/adambiggs/gangline/commit/f7d2158450785b80ee388209f36b0ed2d994b815))
* **gang:** remove the 1.x published-name compatibility layer ([9df2872](https://github.com/adambiggs/gangline/commit/9df28721c3cf9297d0630791f871cb54d6b005d7))
* **help:** answer bare gang with a getting-started page for agents ([49ae989](https://github.com/adambiggs/gangline/commit/49ae989537883ceab64418fa7c3ffc560678e99f))
* **interrupt:** carry a reason past the queue ([fe651f5](https://github.com/adambiggs/gangline/commit/fe651f547a0a4548c7046feced613dd0d04eeb12))
* **mail:** let an agent's own read consume its queue ([953c8e7](https://github.com/adambiggs/gangline/commit/953c8e70c264317378fd714feb867fbc853dc201))
* **mail:** read an agent's waiting queue without touching it ([909e592](https://github.com/adambiggs/gangline/commit/909e59210eeee36dfb7feb09bbc625dc8ee486f2))
* **roles:** attach role briefs at hitch ([ba9ce76](https://github.com/adambiggs/gangline/commit/ba9ce768fd7209c82850803d78fe3565bedd48cd))
* **roles:** declare Claude role prompt option ([87e3851](https://github.com/adambiggs/gangline/commit/87e38517fac03271e0e8403aed004960feb11f1c))
* **roles:** ship lead role brief ([98db5b2](https://github.com/adambiggs/gangline/commit/98db5b28febf8dfacd65dfb9e9d1a65c9b44c961))
* **roster:** add porcelain output ([d514924](https://github.com/adambiggs/gangline/commit/d514924d592eecffa5cc1634720eec6eb8a93d51))
* **roster:** report how long the oldest message has waited ([35d6e84](https://github.com/adambiggs/gangline/commit/35d6e8422ba4d03bb7e8a1f7eae28a2aca86581e))
* **send:** steer a busy claude-code agent through its own queue ([cb58413](https://github.com/adambiggs/gangline/commit/cb584134026c9bf67b166b91596180222bd5b547))
* **spool:** archive pending messages instead of unlinking them ([9d73d6e](https://github.com/adambiggs/gangline/commit/9d73d6eca02dfdb8b58fc978fae5e02775abf325))
* **spool:** drain a whole queue as one chronological bundle ([f228ad0](https://github.com/adambiggs/gangline/commit/f228ad0f2a6f8799b0a345d732a19e91294701b1))
* **team:** align arc ownership and review ([ae300a2](https://github.com/adambiggs/gangline/commit/ae300a245582b7bb6c1b2fa579d2c6f02062aaa5))
* **up:** attach the lead role by default ([1d39d02](https://github.com/adambiggs/gangline/commit/1d39d023bc7500a6ac96ebe64320f759db8142d9))


### Bug Fixes

* **cli:** reject stray stable command arguments ([1b19cf2](https://github.com/adambiggs/gangline/commit/1b19cf232d11410a2561ac1e187d43db4b7c36c1))
* **codex:** stamp sessions from hook payloads ([57a6606](https://github.com/adambiggs/gangline/commit/57a6606a82d9672323d6bb7be5338878b06dee30))
* **collars:** decline Codex hooks for a quote-bearing root ([464ade3](https://github.com/adambiggs/gangline/commit/464ade3128f1611234d9f6b30d1948b59fa39fce))
* **commits:** refuse indeterminate push ranges ([4633459](https://github.com/adambiggs/gangline/commit/4633459f4f21d9cfbac208fc96af4d625d99ebea))
* **compact:** keep a deferred self-compaction across a refused boundary ([9295f8a](https://github.com/adambiggs/gangline/commit/9295f8aed33a1cf08c159e2721a1a42a9655fe90))
* **compact:** wire the Codex bracket and settle one a refusal left open ([bb89f3c](https://github.com/adambiggs/gangline/commit/bb89f3c6862003a82e7be5e737ff3235ac2bd54d))
* **config:** keep implementation seams out of file config ([92673a6](https://github.com/adambiggs/gangline/commit/92673a671de584f38cb50834e44a291b4fb7ae62))
* **curfew:** name missing python dependency ([9290feb](https://github.com/adambiggs/gangline/commit/9290feb09a00274d9329f3200e5d80d11d07a716))
* **delivery:** drain attributed steering mid-turn ([013ea06](https://github.com/adambiggs/gangline/commit/013ea062de72a7cab047f484f0ba6237a0e35643))
* **delivery:** expose unreadable spool drains ([8a06f84](https://github.com/adambiggs/gangline/commit/8a06f84e881db5c5229ba350db2ba69f091c38da))
* **delivery:** keep continuation verification narrow ([8988a97](https://github.com/adambiggs/gangline/commit/8988a9760f14fc3d2d838bbb6d6d398b6419ff89))
* **delivery:** preserve gated and busy handoffs ([58801cd](https://github.com/adambiggs/gangline/commit/58801cd3ec3c6b4da07d2102bc2b0321e95da84f))
* **delivery:** retain native steering recovery ([bed0eb5](https://github.com/adambiggs/gangline/commit/bed0eb56dae018aef815c33c9d270cc40aba0e13))
* **down:** refuse a teardown run from inside the session ([dda0bd6](https://github.com/adambiggs/gangline/commit/dda0bd66e0d34a321dc0218050a3b02ea921c2c9))
* **down:** require the session name a teardown ends ([791d28f](https://github.com/adambiggs/gangline/commit/791d28faf364ea210a9498790c53042cd1fb1796))
* **flush:** distinguish a drained queue from a message never parked ([cc06378](https://github.com/adambiggs/gangline/commit/cc06378a69dd412749b1635003572a8b4a47b875))
* **gang:** resolve an agent by its registration, not its window title ([6f4c5b2](https://github.com/adambiggs/gangline/commit/6f4c5b2af5910d90a7513f447a393846483bb748))
* **gang:** retire the manual park recovery and trim failure-time prose ([d82b29f](https://github.com/adambiggs/gangline/commit/d82b29f1fe27acf0c399547e43fc6267a06337e1))
* **gang:** sanitize a registration before it reaches the terminal ([d2df7c5](https://github.com/adambiggs/gangline/commit/d2df7c50bbb42b40503de61b4328d99efb8fc77d))
* **gate:** close the ownership claims an adversarial review broke ([f7c41b3](https://github.com/adambiggs/gangline/commit/f7c41b32e9788053435b91adb600bc96eaf87975))
* **gate:** judge a link by where it points and read the file before blocking ([2f727ad](https://github.com/adambiggs/gangline/commit/2f727ad6462a52971f42d21d1b67fa4acf38da8f))
* **gate:** name a destination inside the tree instead of blaming the tree ([478b8df](https://github.com/adambiggs/gangline/commit/478b8df67c21338b918897f3fc680ea3a11db16b))
* **gate:** pin the configuration once instead of closing doors one at a time ([551db3e](https://github.com/adambiggs/gangline/commit/551db3e76cf57f1e91db4324954014bd10abe8e2))
* **gate:** pin what counts as the tree, and refuse a copy that failed ([7b17a19](https://github.com/adambiggs/gangline/commit/7b17a19a290154bc34dd5d24bf56aeb82a793b32))
* **gate:** read the index safely, name the subtree, and own the destination ([8d3731b](https://github.com/adambiggs/gangline/commit/8d3731b1a9a7e80976402f0ec37eb58fc302ab7d))
* **githooks:** print the global gate verdict last ([006b5d3](https://github.com/adambiggs/gangline/commit/006b5d3d212cc0772870d9e0c0e179f2065d52e6))
* **hitch:** clear known boot dialogs ([7f713ad](https://github.com/adambiggs/gangline/commit/7f713add3894438fdf8f824ca6cddaf187b36d93))
* **hook:** name an argument the event is not ([979e3cc](https://github.com/adambiggs/gangline/commit/979e3cc71817cb6568e8e70c0abec4024ad9c70a))
* **hook:** report a native event shape gang cannot interpret ([6d1c233](https://github.com/adambiggs/gangline/commit/6d1c233c7b88a7f4f13ab81e1ea1524919a4191b))
* **hooks:** a footer is the last block, not any line after a blank one ([e607653](https://github.com/adambiggs/gangline/commit/e6076533da5100002f05d7c4a5a91ebb6294826a))
* **hooks:** enforce the BREAKING CHANGE footer a breaking subject promises ([e050651](https://github.com/adambiggs/gangline/commit/e0506518dca79b31ca1c81f0fb29486484c3c8ea))
* **hooks:** judge pushed commits by the pushed gate and refuse a failed traversal ([f75d496](https://github.com/adambiggs/gangline/commit/f75d49652e299f8214930178c322a624f136ceb8))
* **hooks:** let the host-global gate report progress while it runs ([3dfb705](https://github.com/adambiggs/gangline/commit/3dfb705b27f3ade79fa1fc37bc1db77a1a8719d5))
* **hooks:** require the breaking footer to be a footer ([bd98d28](https://github.com/adambiggs/gangline/commit/bd98d28033799c2e16ada770e9d39cf4bd3b7ed1))
* **hooks:** silence collar identity diagnostics ([4d42d68](https://github.com/adambiggs/gangline/commit/4d42d68f515d311200dc2f78daea5670eea5cbf1))
* **installer:** replace the gang link instead of writing through it ([cdcdfdd](https://github.com/adambiggs/gangline/commit/cdcdfdd6bb13fcd12c9f743346eb370ded0b7446))
* **interrupt:** name a bad option before resolving self ([650cf8b](https://github.com/adambiggs/gangline/commit/650cf8beba57c03b60c209ff1bab44996a7a4e79))
* **interrupt:** stop this window's own turn with a reason ([915ab81](https://github.com/adambiggs/gangline/commit/915ab81f0cf564b6d7e484ca45126eb694d1618b))
* **messaging:** close adversarial observability gaps ([bc6b9df](https://github.com/adambiggs/gangline/commit/bc6b9df04d7d6406e8bb5722186a070aacc7a372))
* **messaging:** preserve atomic mail ownership ([da071a0](https://github.com/adambiggs/gangline/commit/da071a0f77a5734fa8f4a7b5821b149b5f708468))
* **messaging:** preserve observable delivery truth ([e3ec847](https://github.com/adambiggs/gangline/commit/e3ec8474405e0e8d6d13dd2f96ae935a3c6f0c28))
* **messaging:** tell a clipped composer from an absent one ([ed58bcd](https://github.com/adambiggs/gangline/commit/ed58bcdab6169184176943a48b99f1391957a756))
* **occupancy:** retire permission evidence only on observation ([34fcdbf](https://github.com/adambiggs/gangline/commit/34fcdbfab05c0639bfc6a4aa7b86598c4fe35ff8))
* **parser:** refuse arity no command consumes ([f943786](https://github.com/adambiggs/gangline/commit/f94378680060f5f65a162feb6cfb658cbd3a0dfa))
* **pii-scan:** read diff framing instead of the +++ prefix ([43ef5ba](https://github.com/adambiggs/gangline/commit/43ef5ba8aa318e53f0288ed1ac7e6681c12266b3))
* **prose:** remove the unearned byte ceiling ([249f0dd](https://github.com/adambiggs/gangline/commit/249f0dd422ea0eb8284ececab81ed8183bd92d44))
* **roles:** close review coverage gaps ([c5f01e6](https://github.com/adambiggs/gangline/commit/c5f01e6f4606d71641236dc486fc5e3c3934b380))
* **roster:** parse the padded stamp in base 10 ([c579505](https://github.com/adambiggs/gangline/commit/c579505727e09997d24e37182576af719d13afa7))
* **send:** report what is true of the queued message before why it waited ([9f3a9a6](https://github.com/adambiggs/gangline/commit/9f3a9a66f55157871a1b4feb7d122439f9702cfd))
* **stall:** claim the debounce before deciding on it ([d2ae34a](https://github.com/adambiggs/gangline/commit/d2ae34aab78440421bc49155f3e3f3ce781075e7))
* **startup:** refuse without a UTF-8 locale ([392d693](https://github.com/adambiggs/gangline/commit/392d69391c5d8e7aa2a78507dcad45cd9f89d619))
* **submit:** count the unreadable-box bound only where the read failed ([1606f40](https://github.com/adambiggs/gangline/commit/1606f40d2d356f46a1e534a45fce34cc5a500011))
* **test:** keep the operator's /etc/bash.bashrc off every fixture Enter path ([f71a415](https://github.com/adambiggs/gangline/commit/f71a4150405179a1170fd53e8b05dcd4ec4ec481))
* **test:** make the hermetic-shell guard test a word, and prove it ([11e6a81](https://github.com/adambiggs/gangline/commit/11e6a81bd958a83f2ab6062740b4153379724a0a))
* **test:** report only what the unverified-submission path observed ([910e602](https://github.com/adambiggs/gangline/commit/910e60255005d324187a6acfa10055db7bea0876))
* **test:** stop printing a cause the run never measured ([1e5cb8d](https://github.com/adambiggs/gangline/commit/1e5cb8d551021f403b1cabef6945eaae3fcfd6ea))
* **usage:** wait for command consumption ([dc16e0a](https://github.com/adambiggs/gangline/commit/dc16e0a9c168820a23001972830a00969e9aa5b1))
* **wait-limit:** declare the wake before arming the timer that keeps it ([37d956d](https://github.com/adambiggs/gangline/commit/37d956dd1c66c737341c9bb28c40b68de680fd61))
* **wait-limit:** fire against the timer that armed the declaration ([b785b6d](https://github.com/adambiggs/gangline/commit/b785b6d16efde3297e357c037a5a8c5db1ade0c9))
* **wait-limit:** read the unit's state, arm before declaring, disclose a refused callback ([d3eebef](https://github.com/adambiggs/gangline/commit/d3eebef635b7b3668953b5bcd7aae03259c0088b))
* **wait:** bound and retire native barriers safely ([579c584](https://github.com/adambiggs/gangline/commit/579c584a6878c1ca99287c871b1afd4e1e361178))
* **wait:** make native barriers race-safe ([bdccfa6](https://github.com/adambiggs/gangline/commit/bdccfa6b9c73e87af97cfb4fe1d4d8af3c37bfde))


### Performance Improvements

* **lint:** lint one file per shellcheck process ([5720934](https://github.com/adambiggs/gangline/commit/5720934488b12f31d88f055855665be8bf40bd16))


### Miscellaneous Chores

* release 2.0.0 ([5eed914](https://github.com/adambiggs/gangline/commit/5eed9142e8c3f3dc8da23c6412199833cfb88346))
* **release:** cut this range as 1.1.0, not 2.0.0 ([009274f](https://github.com/adambiggs/gangline/commit/009274fedd53d4aa360db396e0db648428ee433d))


### Code Refactoring

* remove PII scanning; Snubline owns the gate ([fee26a8](https://github.com/adambiggs/gangline/commit/fee26a88b2686255f2c76a32e03e4caa91372da6))

## [1.0.0](https://github.com/adambiggs/gangline/compare/gangline-v0.8.0...gangline-v1.0.0) (2026-08-08)


### ⚠ BREAKING CHANGES

* update -p to -c, GANG_PROFILE to GANG_COLLAR, GANG_PROFILES to GANG_COLLARS, gang profiles to gang collars, gang cutoff to gang curfew, and profile_* contract functions to collar_*. Old spellings remain accepted through 1.x and are removed in 2.0.

### Features

* rename profile to collar and cutoff to curfew ([9b5e96b](https://github.com/adambiggs/gangline/commit/9b5e96baf01952903718754d050fcf8c5bfa573d))
* **status:** add line-state glyph vocabulary ([9aef751](https://github.com/adambiggs/gangline/commit/9aef751b4638d655e06b92b97d1474528176e15d))
* **tmux:** show last-witnessed state in window names ([45483c8](https://github.com/adambiggs/gangline/commit/45483c88d0d40e6b9bca8a7ba4d9a7c93a898f5f))

## [0.8.0](https://github.com/adambiggs/gangline/compare/gangline-v0.7.0...gangline-v0.8.0) (2026-08-08)


### Features

* **identity:** bind panes to native sessions ([4b35387](https://github.com/adambiggs/gangline/commit/4b35387b200fcde1ce94f6d164e209237d8b97eb))


### Bug Fixes

* **identity:** close review round one gaps ([243cb6e](https://github.com/adambiggs/gangline/commit/243cb6e64bd19a5905610152ee84093533e8c525))
* **lock:** support the macOS Bash 3.2 cell ([7603984](https://github.com/adambiggs/gangline/commit/7603984e05c9ccfb194ca7ed5bb9b26ea4b5ccaa))
* **security:** close repository gate bypasses ([db3cff7](https://github.com/adambiggs/gangline/commit/db3cff7e4f903782d67a1e18dcf7ba20fb6f2f92))

## [0.7.0](https://github.com/adambiggs/gangline/compare/gangline-v0.6.0...gangline-v0.7.0) (2026-08-08)


### ⚠ BREAKING CHANGES

* **send:** a refused send to a drainable target now exits 0 as parked; pass --live-only for the old refusal behavior. --spool is deprecated and accepted as an announced no-op.

### Features

* **config:** load strict user configuration ([89cc406](https://github.com/adambiggs/gangline/commit/89cc4069de7f7e2ebcaa71a10635b696425263ab))
* **config:** report effective configuration ([d895228](https://github.com/adambiggs/gangline/commit/d89522886c83faa0e59cb9b0a477140b85490539))
* **gang:** default a bare command to the window it runs in ([9b21da0](https://github.com/adambiggs/gangline/commit/9b21da0f68b4b61b3b3e7a5c6c464d72e501874a))
* **gang:** expose the context computation as an on-demand query ([02aa06d](https://github.com/adambiggs/gangline/commit/02aa06de10abd15ec33069c4353116a7a07dbaab))
* **gang:** make help legible at phone-SSH widths ([a78eba0](https://github.com/adambiggs/gangline/commit/a78eba0129999a310b9950a4897f353ca176aabf))
* **gang:** report a harness's own usage page without attaching ([80b6dea](https://github.com/adambiggs/gangline/commit/80b6dea5ba7fe5f40233faa6293112afbc19be6d))
* **hitch:** add marathon startup rule ([132bfd6](https://github.com/adambiggs/gangline/commit/132bfd60f3732ff72be169143bb8c1c7e6212c98))
* **hitch:** inject operator doctrine ([9dee928](https://github.com/adambiggs/gangline/commit/9dee928cacd6c29e192f702a52fb05f0af28b547))
* **hitch:** require deliberate model choices ([ffc85b7](https://github.com/adambiggs/gangline/commit/ffc85b7da0c0c9add4f4d5ec6c69f69d925b77bc))
* **hitch:** state the complement of envelope attribution ([8cc9aef](https://github.com/adambiggs/gangline/commit/8cc9aef1e641e0e91da25da58b0ca40e26a2b8ec))
* **hook:** forward native awaiting-input events as stall notes ([583a68b](https://github.com/adambiggs/gangline/commit/583a68b0c9bd9cfdcd0996f07d78bd44e023be88))
* merge lead ergonomics ([96982ca](https://github.com/adambiggs/gangline/commit/96982ca67afd6869e6115a3635b26124db47f9ac))
* **profiles:** wire the claude-code Notification hook ([4bdd2d6](https://github.com/adambiggs/gangline/commit/4bdd2d6502fec024f17e8a4fa2189ed50e33b961))
* **send:** park a refused message by default ([38ecdc7](https://github.com/adambiggs/gangline/commit/38ecdc71fa6c8d55e023d6c3f207aad8b413f54c))


### Bug Fixes

* **claude-code:** defer hooked self-compaction ([00407f8](https://github.com/adambiggs/gangline/commit/00407f805d569a073a16bf61e2a4179f940f7b63))
* **githooks:** delegate to the host-global gate ([ce2e587](https://github.com/adambiggs/gangline/commit/ce2e58708a0fafc09cdf550ec97e676ddb7d6b3f))
* **githooks:** lint the pushed commit in a worktree, not a git-less export ([6e6c4c9](https://github.com/adambiggs/gangline/commit/6e6c4c925e3de397df2915d4331398ba3fe1bdff))
* **hitch:** preserve startup delivery refusals ([3e33fbe](https://github.com/adambiggs/gangline/commit/3e33fbee6e571dead64040aeb70e06d8b515807e))
* **hook:** preserve universal turn boundaries ([985cfcd](https://github.com/adambiggs/gangline/commit/985cfcd928b1ba3b881fbd7a0b2b582a40103477))

## [0.6.0](https://github.com/adambiggs/gangline/compare/gangline-v0.5.0...gangline-v0.6.0) (2026-08-07)


### ⚠ BREAKING CHANGES

* **cli:** gang wait is removed. End the native turn and let peer messages restart it, or inspect once with gang status or gang capture.

### Features

* **composer:** read the input box through the profile's styled reading ([169290b](https://github.com/adambiggs/gangline/commit/169290b22cc2b17316c4d3023fcdc74872ccadd6))
* **delivery:** a box-not-empty refusal names what gang read ([8fce815](https://github.com/adambiggs/gangline/commit/8fce8156f0f328d7b09646ddbfbdd986b5ef0567))
* **flush:** recover a parked queue as a verified operation ([e2c84dd](https://github.com/adambiggs/gangline/commit/e2c84dd278ee9d2b276a57ea8507da8bd30a0cc9))
* **hitch:** continue through first-run prompts ([ce5c0db](https://github.com/adambiggs/gangline/commit/ce5c0dbae0453fe176316ebb7c0a8df659fbce2b))
* **hitch:** launch an agent at the reasoning effort its profile spells ([3b83e48](https://github.com/adambiggs/gangline/commit/3b83e48a861abbfa9b72cc568a14f05906676353))
* **interrupt:** send the profile's stop key and close the turn it ended ([7e70282](https://github.com/adambiggs/gangline/commit/7e70282d65f21e162d5a39d34e2a4ececbc85c27))
* **profiles:** declare native reasoning effort for claude-code and codex ([cf28842](https://github.com/adambiggs/gangline/commit/cf288421490ef49b10fe968738b572781579d695))
* **send:** --spool parks a refused delivery for the target's own Stop to drain ([62f9c7f](https://github.com/adambiggs/gangline/commit/62f9c7f53eaf2f7b7371fa6062811cfd981ec9d6))
* **status:** witness executable binary skew ([d00d135](https://github.com/adambiggs/gangline/commit/d00d1358a6dedcdeef6625f6d0ccaa2101f9b6ca))


### Bug Fixes

* **busy:** a decay must witness that nothing moved while it decided ([cf74e73](https://github.com/adambiggs/gangline/commit/cf74e7357e06aa4505dac012bace582c9d4e8dac))
* **busy:** an abandoned turn decays to idle instead of standing expired ([abce280](https://github.com/adambiggs/gangline/commit/abce2802be5d147d148b679f5e3b644c039457b5))
* **busy:** frozen busy paint over an expired bracket is not a live turn ([571fd61](https://github.com/adambiggs/gangline/commit/571fd61d4541f2f96b828e5015f5e388abea98dd))
* **busy:** readers never write the turn bracket; churn control senses its leg ([cbd6d2c](https://github.com/adambiggs/gangline/commit/cbd6d2c8866947b948812c4fb8e85dfbda1677d1))
* **busy:** the quiet reading belongs inside the interval that guards it ([a959dd6](https://github.com/adambiggs/gangline/commit/a959dd6d98206bb523b1729102bb9d5e2f4fe29c))
* **claude-code:** refuse queued mid-turn sends ([5300b08](https://github.com/adambiggs/gangline/commit/5300b089fc6227d40cf5e61a8a64080539045555))
* **claude-code:** refuse stranded mid-turn sends ([cc8a9a1](https://github.com/adambiggs/gangline/commit/cc8a9a18f60b24f599eed04c44bf19e17697ae3b))
* **context:** keep native warm-up silent ([0425608](https://github.com/adambiggs/gangline/commit/04256086ce801aaf6d260e168da44eb42bbce6cd))
* **delivery:** a box gang cannot match is unattributed, not a human draft ([ec20540](https://github.com/adambiggs/gangline/commit/ec20540755b92418a31fdd2e73f974e958c7a913))
* **delivery:** a box that emptied is not a box that cannot be read ([e324b13](https://github.com/adambiggs/gangline/commit/e324b13f8719e7e0b2fe2de6c6746a086b121feb))
* **flush:** compare the recalled body byte for byte, normalizing nothing ([44c538a](https://github.com/adambiggs/gangline/commit/44c538a93456bdfd41e2a82d34da4d9951838b39))
* **hitch:** keep new agents idle ([65ec90a](https://github.com/adambiggs/gangline/commit/65ec90a8121b1d1e21af41730c240c583987705d))
* **hitch:** refuse an effort checker that fails, whatever it printed ([042ffe0](https://github.com/adambiggs/gangline/commit/042ffe0751d0508e60283387cd0facbd63549d9d))
* **profiles:** claude-code declares its parked-queue evidence ([5452a72](https://github.com/adambiggs/gangline/commit/5452a723d493c84cc8dff415c2b8bab2a8a51015))
* **profiles:** effort discovery honors producer status, strict list shape, bounded time ([10e58a4](https://github.com/adambiggs/gangline/commit/10e58a49fd7e96c7f45b4796ee586d7d647164eb))
* **roster:** propagate observation failures ([de07f8c](https://github.com/adambiggs/gangline/commit/de07f8cfbc43fe8f1aed608420d17274abb5c22b))
* **roster:** require a visible idle composer ([b2112c6](https://github.com/adambiggs/gangline/commit/b2112c6f7cdf0c41fa58590a23db95804e9dd4fa))
* **send:** a submission the harness parks in its queue is not a delivery ([b0bda8b](https://github.com/adambiggs/gangline/commit/b0bda8bdaef35ce0b53043d0e21c483351954793))
* **send:** a verified delivery retires the prior staged record ([6f39b75](https://github.com/adambiggs/gangline/commit/6f39b7585e9d6f32ad148f40fcee202155065504))
* **send:** an expired busy witness does not veto a provably empty box ([55c175e](https://github.com/adambiggs/gangline/commit/55c175ea1d12dd2e6d71b9159320db5e834139aa))
* **send:** an unreadable queue-evidence reread fails closed ([7cbc14c](https://github.com/adambiggs/gangline/commit/7cbc14cdbc8edc286c2adf5ac4417361cd41a45c))
* **send:** close the round-1 review findings on spool, flush and interrupt ([7819590](https://github.com/adambiggs/gangline/commit/7819590fa31dac106e91e0b09faadd3b6f18e10c))
* **send:** delivery never writes turn state; busy worlds exercise the pty leg ([fff3e06](https://github.com/adambiggs/gangline/commit/fff3e0673fcd49e0b26690bf35326c7266e7302d))
* **send:** name the hard-stuck queue escalation in parked-input guidance ([051a26d](https://github.com/adambiggs/gangline/commit/051a26d1557f801ee4a99948f5f79bdf422337f5))
* **send:** observed movement is not the absence the fall-through is for ([5661aae](https://github.com/adambiggs/gangline/commit/5661aaee34f5f9667d953a28a30613f1535285de))
* **status:** a staged record the empty box refutes is not reported ([0475037](https://github.com/adambiggs/gangline/commit/047503742ca79a402ab334a38c2f5795cf24a265))
* **status:** name the directory a held message is readable in ([9ffddd9](https://github.com/adambiggs/gangline/commit/9ffddd93670805fe3c78fd521843294e9f96bf35))
* **status:** report undelivered input for what is known, guard clearing semantics ([09697b3](https://github.com/adambiggs/gangline/commit/09697b3246ef4b849b5b3eb01cd990fc4237065c))


### Performance Improvements

* **codex:** read newest context record first ([0bc14a8](https://github.com/adambiggs/gangline/commit/0bc14a844230fd61ceb060ca5315a39ba83b38b9))
* **roster:** remove pane churn sampling ([5569bff](https://github.com/adambiggs/gangline/commit/5569bff77eaa55cb82c6457e37a4a3e979c2afe4))


### Reverts

* restore Claude mid-turn sends ([5814651](https://github.com/adambiggs/gangline/commit/58146519b50141251b063a3ba821a663d94b94b0))


### Code Refactoring

* **cli:** remove agent polling wait ([ae9ba17](https://github.com/adambiggs/gangline/commit/ae9ba17cf19db9e58c379161df44a2e388e3e3b9))

## [0.5.0](https://github.com/adambiggs/gangline/compare/gangline-v0.4.0...gangline-v0.5.0) (2026-08-05)


### ⚠ BREAKING CHANGES

* gang wait no longer marks callers as parked or exposes waiting-on state through status and roster. Profiles no longer declare GANG_MIDTURN_ACTS.
* gang hitch no longer accepts -r or --role, and GANG_ROLE and GANG_ROLES are no longer read. Goals and roles remain ordinary native-harness prose.
* gang send now takes its target through --to. Calls inside a Gangline window derive their sender and reject --from; outside callers must provide --from.
* reduce Gangline to substrate primitives

### Features

* **claude-code:** the context beacon rides the launch line, quoted for the shell that reads it ([6cf0139](https://github.com/adambiggs/gangline/commit/6cf01393259e75622ca319fbca6ba548f3083a40))
* **context:** tell an agent what it is carrying, not what is left ([724458b](https://github.com/adambiggs/gangline/commit/724458b8c003d3455c013653d704ab1f3ab89427))
* **context:** the compaction bracket, an event tier over the mark ([3698e20](https://github.com/adambiggs/gangline/commit/3698e20ef40965d2a1d54b9625f97d87ea95c169))
* **context:** the context fact, an owned tier over the beacon scrape ([6f8753c](https://github.com/adambiggs/gangline/commit/6f8753c4449ad05579bbd727c2122880d671b6ff))
* **context:** write the band nudges as instructions, not prose ([e5e40ef](https://github.com/adambiggs/gangline/commit/e5e40efc9e2708f817b591f0dddcdf03390e2b66))
* **cron:** derive the patrol entry from the install that will run it ([3455b88](https://github.com/adambiggs/gangline/commit/3455b88ae667f9b1d8b3d15797524ce99e647219))
* **cutoff:** a hitch can declare the team's budget on the way in ([7ea3764](https://github.com/adambiggs/gangline/commit/7ea37643d582933a59ff9919560263aa8033fbc0))
* **cutoff:** restore thin team time lights ([e1f8f07](https://github.com/adambiggs/gangline/commit/e1f8f07dfc87bf2143d60d3871bbd0ccafc3dfcc))
* **cutoff:** the team's wall-clock budget enters as a declaration, not a measurement ([cbfa3d0](https://github.com/adambiggs/gangline/commit/cbfa3d07a5f75738020fff5b1ac339f6688048b1))
* derive senders inside Gangline ([9e1740e](https://github.com/adambiggs/gangline/commit/9e1740e3e3774b8fa8664c14f4114af807b95b2c))
* **hooks:** run the quick CI gates before the push, not after it ([c19e25b](https://github.com/adambiggs/gangline/commit/c19e25bdf8cfb20b65b2daa9ecd1d0248ecb9196))
* **hook:** the budget speaks in the turn, both notes in one reply ([674ed2f](https://github.com/adambiggs/gangline/commit/674ed2fb6bd172786492fecc9937ffdd03b79b70))
* **hook:** turn brackets, and a busy() that prefers them to the pane ([019bc06](https://github.com/adambiggs/gangline/commit/019bc060db61bc7670cee522dfd38709042bcb6b))
* **install:** refresh an existing patrol entry when the install updates ([6c174e8](https://github.com/adambiggs/gangline/commit/6c174e8b272ed65676d3c25334023e157c591643))
* **occupancy:** the occupied raise, an event tier over both scrapes ([d131ab9](https://github.com/adambiggs/gangline/commit/d131ab9ca9ea1980975ebca613a6645205085ce5))
* **patrol:** an explicit ladder for the budget, absolute rungs refused ([535524e](https://github.com/adambiggs/gangline/commit/535524eb91eb8e92fbb9d933a8453d36e00398b8))
* **patrol:** record a sweep in gang, not in a crontab pipeline ([9ffa6e2](https://github.com/adambiggs/gangline/commit/9ffa6e29ddd160facd1c08ece65083a6bd82ef7a))
* **patrol:** the band ladder speaks on the time axis ([087db27](https://github.com/adambiggs/gangline/commit/087db27bd66dfe729dbdcd2a735f6039ea8ec447))
* **patrol:** the team's budget is reported on every sweep ([9bbee3d](https://github.com/adambiggs/gangline/commit/9bbee3d0f03830c8064d024ac24f5eb5d1787412))
* **pii-scan:** scan commits for PII, gate pushes on it ([567a700](https://github.com/adambiggs/gangline/commit/567a700a90f12fbd768296e4ef63b9ed3c0bd0dc))
* **probe:** vet --probe learns the fact pipeline ([53ed9ee](https://github.com/adambiggs/gangline/commit/53ed9ee5bcc31a8a170e40da3b4134a9c032bc29))
* **profile:** wire claude-code's turn hooks at hitch, in exec form ([87743a4](https://github.com/adambiggs/gangline/commit/87743a49645b3a158d855d07ac7812d7e7302be5))
* **send:** hold a send behind a hand at the keyboard instead of handing it back ([bf76648](https://github.com/adambiggs/gangline/commit/bf76648acfcff21f9ac01ef791d7b0b06eeacd52))
* **vet:** --file-issue reaches the tier-conflict class ([bda6c24](https://github.com/adambiggs/gangline/commit/bda6c24b9b25454c8ac773a50f1f44fe658d1e16))
* **vet:** hold each agent's two witnesses against each other ([2ea1982](https://github.com/adambiggs/gangline/commit/2ea198284b59c52b842d8d7a099c1e8425a2ba26))
* **vet:** python3 is re-asserted where an operator looks, by being run ([4e614d7](https://github.com/adambiggs/gangline/commit/4e614d7f1020871e2dca44a2434e8b0717b96e82))
* **vet:** report a patrol entry that stopped matching this install ([e029a74](https://github.com/adambiggs/gangline/commit/e029a74ebf3e913d2c98a670f35388440fd1645c))


### Bug Fixes

* bind harness commands to their exact team ([88adef6](https://github.com/adambiggs/gangline/commit/88adef6c84804ffc11e1b459e6ed1129e939f8f0))
* **codex:** tell Codex's placeholder from a draft somebody typed ([97c0450](https://github.com/adambiggs/gangline/commit/97c04503acdea5f8fee0c006c545f9458b204c77)), closes [#53](https://github.com/adambiggs/gangline/issues/53)
* **context:** quote both names in compact's inline-resume refusal ([d26286f](https://github.com/adambiggs/gangline/commit/d26286f8584553a02cc31dbf9e04450bd66eb815))
* **context:** quote the agent name in the suggested compaction command ([351efc4](https://github.com/adambiggs/gangline/commit/351efc4c61e1f3b4bcb48fc749f2f37fa9d231d7))
* **context:** steer the band note to the handoff already being kept ([497048d](https://github.com/adambiggs/gangline/commit/497048dcc2e7a557ef4faa2a2046cb8b6fe7ef69))
* **context:** stop reading a zero token count as usage dropping ([ec6159d](https://github.com/adambiggs/gangline/commit/ec6159df08f41a2a50a1fd6ee3838853f8af4ba0))
* **cutoff:** use portable local times ([f175940](https://github.com/adambiggs/gangline/commit/f175940513faadde366051067fcd87a69d176e77))
* **delivery:** create lock ownership atomically ([4c508f3](https://github.com/adambiggs/gangline/commit/4c508f32c4e56055c5b0ecf1dd6cbe073df6e2bc))
* **delivery:** name runnable refusal paths ([070393b](https://github.com/adambiggs/gangline/commit/070393b91bf0c0e51ea6e113d13fcff0428f96b7))
* **hitch:** stop soliciting a startup reply ([bcf8930](https://github.com/adambiggs/gangline/commit/bcf8930da1fc75b48fc0d4f2f713aee0725faa03))
* **installer:** execute Python dependency check ([c0c3dd5](https://github.com/adambiggs/gangline/commit/c0c3dd52c2bf31023c2a4868ad923ace11ca82e6))
* keep disabled context lights invisible ([85477aa](https://github.com/adambiggs/gangline/commit/85477aa4a3304f2838c9ff743868faa94621daf4))
* **lint:** guard on shellcheck running, not its name resolving ([5b10b83](https://github.com/adambiggs/gangline/commit/5b10b83ea312e71eaddaa609326d4c9d2c056c65))
* **patrol:** let an unproved compaction expire instead of silencing an agent ([f208506](https://github.com/adambiggs/gangline/commit/f208506e7e35d6ace3531476c0e2c457253c9c66))
* **patrol:** nudge the agent that is working, not only the one that is idle ([aa47fab](https://github.com/adambiggs/gangline/commit/aa47fab0875a5e2429d63afed4683c6f891f6ecf))
* **patrol:** rebuild an unreadable band memory instead of going silent ([7ea66b6](https://github.com/adambiggs/gangline/commit/7ea66b6ad6b497bfe09334137aa473b91ac5ce73))
* **patrol:** rebuild the state gang wrote instead of refusing over it ([64d85c4](https://github.com/adambiggs/gangline/commit/64d85c42331286f6a3d69e59dbe4a68cee007eb4))
* **pii-scan:** pin and assert bash 3.2 on the macOS CI cell ([fc70490](https://github.com/adambiggs/gangline/commit/fc70490e01c13f9060511b382c9d49cbf9788ace))
* **probe:** a row that names a busy marker names the file, not the pattern ([29c48df](https://github.com/adambiggs/gangline/commit/29c48df69d6e3eb7dc7d16691ed5e44a0ef786c5))
* **profiles:** an indented marker is a menu cursor, not a prompt ([5731f40](https://github.com/adambiggs/gangline/commit/5731f4097f83c58b4abeb7fe04c0c62c93bb4bae))
* **profiles:** restore native Codex hooks ([70dd4e2](https://github.com/adambiggs/gangline/commit/70dd4e294f9fb2f6414c3cff937a6620bc10c400))
* **profiles:** the box is the last non-empty row ([f764110](https://github.com/adambiggs/gangline/commit/f764110f2b0cedb0efae3a92a0a50314b0e394b6))
* **profiles:** the marker picks the box's row, not emptiness ([6b02018](https://github.com/adambiggs/gangline/commit/6b020183b744f87837520f6e4c5f6bf60aa1a0a7))
* scan only prospective PII additions ([a4e52aa](https://github.com/adambiggs/gangline/commit/a4e52aa5923772e0d8b1c5289147d83b062300d9))
* **send:** a clear nobody made cannot erase the record of a paste ([ae41693](https://github.com/adambiggs/gangline/commit/ae41693704cd6de1bd19eff9fe789c5f94e9855a))
* **send:** an unverified paste is recorded, not only reported ([d81f02f](https://github.com/adambiggs/gangline/commit/d81f02f545b7f68aa007fbfc0fce0365a81fa053))
* **send:** refuse a delivery into a box that still holds a draft ([e0d5d31](https://github.com/adambiggs/gangline/commit/e0d5d319e56eb3129d882fad58b14511d556f991))
* **test:** clear the two shellcheck warnings that turned CI red ([91291b6](https://github.com/adambiggs/gangline/commit/91291b62b8e7fac5141e21c0887d26617027b547))
* **test:** the retryable fixture counts its looks instead of trusting $RANDOM ([b6cf1ef](https://github.com/adambiggs/gangline/commit/b6cf1ef56ce497df925d6c15302eaf8700db795d))
* **turn-bracket:** a closed bracket stops outranking a pane still being written to ([0562d8a](https://github.com/adambiggs/gangline/commit/0562d8afad8182e11d28a9a194ba8c5638a6846e))
* **vet:** a compaction in flight explains the paint it is blamed for ([9ee60e3](https://github.com/adambiggs/gangline/commit/9ee60e3cfa34fbfed01740a30876a6eea5117b8a))
* **vet:** a failed gh issue create can no longer report a filing ([3e056fe](https://github.com/adambiggs/gangline/commit/3e056fe2277a30326b8575023eec0257a32880b7))
* **vet:** the tier row names the file that declares a marker, not the marker ([6a08458](https://github.com/adambiggs/gangline/commit/6a08458972c4090932e1785b50877c43da0f4d06))


### Code Refactoring

* make wait observational ([8fb85b5](https://github.com/adambiggs/gangline/commit/8fb85b5db270dacdf3742052f2e7b3399ffefc13))
* reduce Gangline to substrate primitives ([a1f6286](https://github.com/adambiggs/gangline/commit/a1f6286c847d2377f9ef7a3253b6c9c6dc5de23f))
* remove role-brief plumbing ([fb0e80f](https://github.com/adambiggs/gangline/commit/fb0e80ff3eceefb56312239b2a3a9983c751ae1c))

## [0.4.0](https://github.com/adambiggs/gangline/compare/gangline-v0.3.0...gangline-v0.4.0) (2026-07-31)


### ⚠ BREAKING CHANGES

* profiles declare GANG_OCCUPIED_REGEX. A profile still setting GANG_GATED_REGEX fails to load, naming the file and the replacement.
* `gang context-report` is gone, along with GANG_CONTEXT_LOG and GANG_CONTEXT_LOG_MAX_BYTES. Nothing writes a persistent dataset any more.

### Features

* **hitch:** --resume, so a dead tmux server costs work rather than memory ([b101b3f](https://github.com/adambiggs/gangline/commit/b101b3f58f0fa3855b7dd9fd96a600f26d85044d)), closes [#44](https://github.com/adambiggs/gangline/issues/44)
* remove context-report and the measurement apparatus behind it ([1782882](https://github.com/adambiggs/gangline/commit/178288236c46a97a258bca33a7cc241328f71934))
* rename the occupancy declaration and refuse the retired name ([a53952d](https://github.com/adambiggs/gangline/commit/a53952d15877820f2096756d586cface88724759)), closes [#47](https://github.com/adambiggs/gangline/issues/47)
* **vet:** probe declared mid-turn actions ([b1f2203](https://github.com/adambiggs/gangline/commit/b1f2203f6b62643549049cb76fc6510b3c93b5c9))


### Bug Fixes

* **deliver:** stop typing where nothing proved it was safe to type ([473fe89](https://github.com/adambiggs/gangline/commit/473fe89360ec67898a307cead75883944dc525dd)), closes [#37](https://github.com/adambiggs/gangline/issues/37) [#40](https://github.com/adambiggs/gangline/issues/40) [#46](https://github.com/adambiggs/gangline/issues/46)
* **demo:** ask for the team to stay, so the closing frame has one ([2d6c431](https://github.com/adambiggs/gangline/commit/2d6c43189b8554b0c798fc1dbcec5393da4832cd))
* **demo:** gate the closing shot on idle, not on nothing being busy ([d40b00b](https://github.com/adambiggs/gangline/commit/d40b00bde1d0df8a16a4ab869854b4ec7ffe0e0a)), closes [#49](https://github.com/adambiggs/gangline/issues/49)
* **env:** refuse a numeric bound that is not a number, at every read site ([cc3c8a5](https://github.com/adambiggs/gangline/commit/cc3c8a56948612f26ff11772f21fd28a13fb37c3)), closes [#41](https://github.com/adambiggs/gangline/issues/41)
* **patrol:** repeat final-band nudges ([3b534c4](https://github.com/adambiggs/gangline/commit/3b534c42206de160b9b4413d78bce6070eb84962))
* **send:** keep the trailing newlines stdin_body pays a sentinel to preserve ([d524107](https://github.com/adambiggs/gangline/commit/d52410722fc80eb5cabd5eb0b945b22ef4572f82)), closes [#42](https://github.com/adambiggs/gangline/issues/42)
* **test:** void a run whose own code moved under it ([a09b833](https://github.com/adambiggs/gangline/commit/a09b8333e5fd2ebf6dc1d97d76a9cdbdabd99a7e)), closes [#48](https://github.com/adambiggs/gangline/issues/48)

## [0.3.0](https://github.com/adambiggs/gangline/compare/gangline-v0.2.0...gangline-v0.3.0) (2026-07-31)


### ⚠ BREAKING CHANGES

* **state:** the `gang status` primary state prefix changes from `gated` to `occupied`. Scripts matching the state prefix as documented in docs/reference.md must move with it. `gated` is not accepted as an alias.

### Features

* **context:** escalate what each band asks for, not how loudly ([ade63ff](https://github.com/adambiggs/gangline/commit/ade63ff2d88435b3d24598e1676baed371072582))
* **down:** say what a teardown is about to destroy ([e6c5854](https://github.com/adambiggs/gangline/commit/e6c58541bcfb9bf4ef010ce9cfc5af1bef672f1e))
* **send:** surface the traffic an occupied agent is refusing ([4ff88cf](https://github.com/adambiggs/gangline/commit/4ff88cf8318855efebc9f425dfbae1054639c0e2))
* **state:** publish occupancy, not authority ([c16228e](https://github.com/adambiggs/gangline/commit/c16228e15bef1ff8a9da535f12cc57f37176f44a))
* **vet:** gate the claude-code context beacon on its configuration ([fddc688](https://github.com/adambiggs/gangline/commit/fddc68800d035efcd0e556e0e88c5b8b66df3e39))
* **vet:** say which phase a probe is waiting in ([e2fb4d6](https://github.com/adambiggs/gangline/commit/e2fb4d63c03cf261853ef86aec094f6a82672b3c))


### Bug Fixes

* **ci:** assert the macOS cell really parses with bash 3.2 ([da7f526](https://github.com/adambiggs/gangline/commit/da7f5262fbc96bc2c12d7a8a1ddbca60d9e0b015))
* **docs:** say what wait promises, and point install at a command not a heading ([fb89a59](https://github.com/adambiggs/gangline/commit/fb89a59640190d4a2ff4d4f986303b50a26d6fbd)), closes [#45](https://github.com/adambiggs/gangline/issues/45)
* **patrol:** stop spending an expired compaction grace as permission to inject ([ff26a34](https://github.com/adambiggs/gangline/commit/ff26a34e3b97a6ca5ea4257898377b7d0bb862f0))
* **send:** neutralise the shape of a tag, not one spelling of it ([dda33d5](https://github.com/adambiggs/gangline/commit/dda33d52439b0a60caa43460c5d5c2d0af2d92f8)), closes [#38](https://github.com/adambiggs/gangline/issues/38)

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
