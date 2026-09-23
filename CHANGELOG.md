# Changelog

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
