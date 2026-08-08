# Changelog

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
