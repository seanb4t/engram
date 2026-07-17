# Changelog

## [0.8.8](https://github.com/seanb4t/engram/compare/v0.8.7...v0.8.8) (2026-07-17)


### Bug Fixes

* **ci:** run renovate ui/ postUpgradeTasks via bash -c (shell-free) ([#301](https://github.com/seanb4t/engram/issues/301)) ([#392](https://github.com/seanb4t/engram/issues/392)) ([5218eb6](https://github.com/seanb4t/engram/commit/5218eb6af16b5c25542b79b9619f51aa5ff74a24))
* **deps:** update all non-major dependencies ([#380](https://github.com/seanb4t/engram/issues/380)) ([4533872](https://github.com/seanb4t/engram/commit/45338720d291fdb8ecbe188cf8171aa16b841f62))
* **deps:** update all non-major dependencies ([#382](https://github.com/seanb4t/engram/issues/382)) ([d91024c](https://github.com/seanb4t/engram/commit/d91024c79bc29d67192e0bbbba87a4f16ff9e7cd))

## [0.8.7](https://github.com/seanb4t/engram/compare/v0.8.6...v0.8.7) (2026-07-16)


### Bug Fixes

* CI & maintenance hygiene — renovate self-heal, phase-11 residuals, rumdl exclude (phase 21) ([#371](https://github.com/seanb4t/engram/issues/371)) ([4c94033](https://github.com/seanb4t/engram/commit/4c9403396957b09b4c5d4007e4bec562375f10b8))

## [0.8.6](https://github.com/seanb4t/engram/compare/v0.8.5...v0.8.6) (2026-07-16)


### Features

* **console:** add memory & discovery write UX (create/edit/delete/re-share/schedule) ([#367](https://github.com/seanb4t/engram/issues/367)) ([72a8011](https://github.com/seanb4t/engram/commit/72a80119945e5a41b397ac18c0fea3dac772984f))
* Phase 20 — correctness & polish (discovery proto fidelity, MintShortID cap, embed cleanup, summarize CronJob) ([#368](https://github.com/seanb4t/engram/issues/368)) ([4a5def8](https://github.com/seanb4t/engram/commit/4a5def82e9e4b844e901a16a9a4ed9bae5a313ab))
* **server:** wire Connect write handlers with MCP authz parity (full CRUD + schedule) ([#322](https://github.com/seanb4t/engram/issues/322)) ([#363](https://github.com/seanb4t/engram/issues/363)) ([a95d3b3](https://github.com/seanb4t/engram/commit/a95d3b3e5f25e9006f7426d6f69cb8ed69e4c704))
* **webauth:** stateless sliding-expiry session rotation ([#323](https://github.com/seanb4t/engram/issues/323)) ([#365](https://github.com/seanb4t/engram/issues/365)) ([8ba5e42](https://github.com/seanb4t/engram/commit/8ba5e4290883a9e3557dbba99a8bca230292b275))

## [0.8.5](https://github.com/seanb4t/engram/compare/v0.8.4...v0.8.5) (2026-07-12)


### Features

* **api:** additive Connect write-RPC wire contract + protovalidate interceptor ([#322](https://github.com/seanb4t/engram/issues/322)) ([#359](https://github.com/seanb4t/engram/issues/359)) ([7d17c53](https://github.com/seanb4t/engram/commit/7d17c5346e4c70c287ebd289e5e0165f2ee4c264))
* **embed:** phase 13 — embedder reliability foundation (timeout, base-URL join, config-identity) ([#348](https://github.com/seanb4t/engram/issues/348)) ([3acd662](https://github.com/seanb4t/engram/commit/3acd662db49d34ad50a7f4fdf794f89ebdd8b4bd))
* **server:** write-lane CSRF defense (CrossOriginProtection + double-submit interceptor) ([#322](https://github.com/seanb4t/engram/issues/322)) ([#361](https://github.com/seanb4t/engram/issues/361)) ([fed9050](https://github.com/seanb4t/engram/commit/fed9050c9c6e1819e267b6a3cf3b313493563e46))
* v0.9.x recall-quality milestone (eval harness, async summaries, usage signals) ([#336](https://github.com/seanb4t/engram/issues/336)) ([658795e](https://github.com/seanb4t/engram/commit/658795e97852448bc792a20c2f1a020d59743000))


### Bug Fixes

* **lint:** exclude node_modules from yamlfmt (follow-up to [#314](https://github.com/seanb4t/engram/issues/314)) ([#329](https://github.com/seanb4t/engram/issues/329)) ([a52162a](https://github.com/seanb4t/engram/commit/a52162a2152f75b26b317f828744ef7f13aa0437))
* **lint:** green up local rumdl + yamlfmt gate ([#314](https://github.com/seanb4t/engram/issues/314)) ([#325](https://github.com/seanb4t/engram/issues/325)) ([fc045d7](https://github.com/seanb4t/engram/commit/fc045d714ed97d3b32996b83e7c63aca8f424975))
* **ui:** clear svelte-check TS2590 in vendored shadcn primitives ([#311](https://github.com/seanb4t/engram/issues/311)) ([#326](https://github.com/seanb4t/engram/issues/326)) ([68db077](https://github.com/seanb4t/engram/commit/68db0777e09639f2fcb1c902f0c73cc51537bf63))

## [0.8.4](https://github.com/seanb4t/engram/compare/v0.8.3...v0.8.4) (2026-07-09)


### Features

* add rule memory kind (store_rule/list_rules) ([#290](https://github.com/seanb4t/engram/issues/290)) ([306467d](https://github.com/seanb4t/engram/commit/306467d3a2be6b79c9b9fedc59f9ef657d820c74))
* short_id handles for memory records (engram-c0yl) ([#288](https://github.com/seanb4t/engram/issues/288)) ([92a6f61](https://github.com/seanb4t/engram/commit/92a6f610e9592014df09470f864af81a75c030ca))


### Bug Fixes

* **deps:** update module google.golang.org/grpc to v1.82.0 ([#274](https://github.com/seanb4t/engram/issues/274)) ([59c21a3](https://github.com/seanb4t/engram/commit/59c21a3890d7de5feeaf37f0a5cb1c84f71029f2))

## [0.8.3](https://github.com/seanb4t/engram/compare/v0.8.2...v0.8.3) (2026-07-02)


### Features

* asymmetric/cloud embedder param passthrough (engram-0qed) ([#264](https://github.com/seanb4t/engram/issues/264)) ([7f376db](https://github.com/seanb4t/engram/commit/7f376db412232922bb8fcb1902b36c9bac654481))
* **embed:** asymmetric query instruction + tags-in-vector + expose score (engram-wd89) ([#262](https://github.com/seanb4t/engram/issues/262)) ([08a0b97](https://github.com/seanb4t/engram/commit/08a0b9793150c78f1c166278cd9c44c058544f8c))
* **reindex:** --source, per-batch progress, and resumable --resume (engram-orve/xddn/irhg) ([#258](https://github.com/seanb4t/engram/issues/258)) ([bb57383](https://github.com/seanb4t/engram/commit/bb573831a0b93570a2aa95152655e9d5b534fe05))

## [0.8.2](https://github.com/seanb4t/engram/compare/v0.8.1...v0.8.2) (2026-06-30)


### Features

* **connect:** cursor_mode opt-in for ListMemories first-page bootstrap (engram-3hp9) ([#255](https://github.com/seanb4t/engram/issues/255)) ([42ef01c](https://github.com/seanb4t/engram/commit/42ef01c1cf6caedf7d5071a04bb9c5f972824237))

## [0.8.1](https://github.com/seanb4t/engram/compare/v0.8.0...v0.8.1) (2026-06-30)


### Bug Fixes

* **deps:** update module github.com/qdrant/go-client to v1.18.3 ([#251](https://github.com/seanb4t/engram/issues/251)) ([bc924c6](https://github.com/seanb4t/engram/commit/bc924c6d220b4d96feaf3733c0bd49719913072a))

## [0.8.0](https://github.com/seanb4t/engram/compare/v0.7.17...v0.8.0) (2026-06-29)


### ⚠ BREAKING CHANGES

* derive record owner from configurable OIDC claim + migrate-remap-owner (engram-8bsz) ([#248](https://github.com/seanb4t/engram/issues/248))

### Features

* derive record owner from configurable OIDC claim + migrate-remap-owner (engram-8bsz) ([#248](https://github.com/seanb4t/engram/issues/248)) ([d6ea507](https://github.com/seanb4t/engram/commit/d6ea507df6bbd3dd88948526df2239dc5032316f))

## [0.7.17](https://github.com/seanb4t/engram/compare/v0.7.16...v0.7.17) (2026-06-28)


### Bug Fixes

* **renovate:** block cookie major bumps (v2 breaks SvelteKit 2.68.0 build) ([#241](https://github.com/seanb4t/engram/issues/241)) ([2d59af0](https://github.com/seanb4t/engram/commit/2d59af0dc62c021266629a4c9120288fa6295787))
* **ui:** cap app shell to viewport so detail pane stays fixed (engram-ecdj) ([#243](https://github.com/seanb4t/engram/issues/243)) ([6ea5867](https://github.com/seanb4t/engram/commit/6ea5867765f79d855b2ca03278fad64e90b2b3d2))

## [0.7.16](https://github.com/seanb4t/engram/compare/v0.7.15...v0.7.16) (2026-06-28)


### Features

* **engram-skill:** prefer engram over beads for durable memory (engram-g9rj) ([#237](https://github.com/seanb4t/engram/issues/237)) ([9aafa74](https://github.com/seanb4t/engram/commit/9aafa7418059d9dfec13d00e3aab41325dffb4a2))

## [0.7.15](https://github.com/seanb4t/engram/compare/v0.7.14...v0.7.15) (2026-06-28)


### Features

* **summarize:** harden summarize-missing egress (engram-uhwd) ([#231](https://github.com/seanb4t/engram/issues/231)) ([95207f8](https://github.com/seanb4t/engram/commit/95207f871a680af982a448e606dcbc1f365214d4))

## [0.7.14](https://github.com/seanb4t/engram/compare/v0.7.13...v0.7.14) (2026-06-27)


### Features

* **ui:** memory display + auto-summary UX (epic engram-gyo7) ([#221](https://github.com/seanb4t/engram/issues/221)) ([49309b2](https://github.com/seanb4t/engram/commit/49309b2eaf67c20e0f870a8b57464eb81db1644a))

## [0.7.13](https://github.com/seanb4t/engram/compare/v0.7.12...v0.7.13) (2026-06-27)


### Bug Fixes

* **summarize:** configurable max_tokens + HTTP timeout for reasoning models ([#218](https://github.com/seanb4t/engram/issues/218)) ([c45f0a2](https://github.com/seanb4t/engram/commit/c45f0a2563d171feba8fd44d5c56b4f503838556))

## [0.7.12](https://github.com/seanb4t/engram/compare/v0.7.11...v0.7.12) (2026-06-26)


### Features

* **chart:** surface auto-summary config (ENGRAM_SUMMARY_MODEL/_MAX_CHARS) ([#215](https://github.com/seanb4t/engram/issues/215)) ([c641523](https://github.com/seanb4t/engram/commit/c64152305c9d3af3999020890a896c8a0257b423))

## [0.7.11](https://github.com/seanb4t/engram/compare/v0.7.10...v0.7.11) (2026-06-26)


### Bug Fixes

* **deps:** update dependency @astrojs/starlight to ^0.41.0 ([#212](https://github.com/seanb4t/engram/issues/212)) ([4afb026](https://github.com/seanb4t/engram/commit/4afb026727f14867202a9121b715d4dc00279faa))
* **ui:** rebuild vendored SPA after svelte 5.56.4 bump ([#214](https://github.com/seanb4t/engram/issues/214)) ([efe5447](https://github.com/seanb4t/engram/commit/efe5447aaf9b660249ce4e8e7c62b526cbbcabcc))

## [0.7.10](https://github.com/seanb4t/engram/compare/v0.7.9...v0.7.10) (2026-06-26)


### Features

* **summary:** auto-summary for curated memories (engram-cly5) ([#207](https://github.com/seanb4t/engram/issues/207)) ([d84ec95](https://github.com/seanb4t/engram/commit/d84ec95633c737718444785b970731699eb1d045))

## [0.7.9](https://github.com/seanb4t/engram/compare/v0.7.8...v0.7.9) (2026-06-24)


### Features

* **telemetry:** finer low-end histogram buckets for engram.*.duration (engram-e1kh) ([#193](https://github.com/seanb4t/engram/issues/193)) ([560e5d1](https://github.com/seanb4t/engram/commit/560e5d19a6ea4e69ae682f943320cca715de18c2))


### Bug Fixes

* **deps:** update module github.com/coreos/go-oidc/v3 to v3.19.0 ([#176](https://github.com/seanb4t/engram/issues/176)) ([7718af6](https://github.com/seanb4t/engram/commit/7718af684428ba35a01e17dbaec9e1d98965cfc5))
* **deps:** update module github.com/testcontainers/testcontainers-go/modules/qdrant to v0.43.0 ([#181](https://github.com/seanb4t/engram/issues/181)) ([0e4e905](https://github.com/seanb4t/engram/commit/0e4e90562045ff2a661065c99aa2e41ea8e1745a))
* gate Helm OpenAI-key secret, bump cookie to 0.7.2, clear docs-site lint ([#171](https://github.com/seanb4t/engram/issues/171)) ([ac47513](https://github.com/seanb4t/engram/commit/ac475137f12169590deb7fd2285daf639999d2d4))

## [0.7.8](https://github.com/seanb4t/engram/compare/v0.7.7...v0.7.8) (2026-06-20)


### Features

* **api:** tag filter parity on Connect EngramService ([#166](https://github.com/seanb4t/engram/issues/166)) ([6e6d626](https://github.com/seanb4t/engram/commit/6e6d626004fa4383e663f259bfadb07302696a1b))
* **memory:** tag-filtered recall on search_memory and list_memory ([#164](https://github.com/seanb4t/engram/issues/164)) ([475884b](https://github.com/seanb4t/engram/commit/475884b186beb20e47305b074be791856ba98c69))

## [0.7.7](https://github.com/seanb4t/engram/compare/v0.7.6...v0.7.7) (2026-06-15)


### Features

* **memory:** update_memory can replace a record's tag set ([#147](https://github.com/seanb4t/engram/issues/147)) ([f145275](https://github.com/seanb4t/engram/commit/f1452758cd9ec8878cf12722901fac08f97a8f9f))

## [0.7.6](https://github.com/seanb4t/engram/compare/v0.7.5...v0.7.6) (2026-06-15)


### Features

* **config:** centralized Config.Validate for data-plane well-formedness ([#143](https://github.com/seanb4t/engram/issues/143)) ([eed8963](https://github.com/seanb4t/engram/commit/eed8963558777ecad112d5e6a6e392de2ffbf67a))


### Bug Fixes

* **config:** drop test-only MEM_QDRANT_TEST_ADDR from legacy runtime guard ([#142](https://github.com/seanb4t/engram/issues/142)) ([4622ac7](https://github.com/seanb4t/engram/commit/4622ac7aeccf1c065003dc2b32b5a88090f40749))
* **engram-skill:** route explicit time-bound 'remember' requests via an engram-vs-beads gate ([#135](https://github.com/seanb4t/engram/issues/135)) ([d7f1d72](https://github.com/seanb4t/engram/commit/d7f1d72314d1b111065134df6e31837805c581fa))

## [0.7.5](https://github.com/seanb4t/engram/compare/v0.7.4...v0.7.5) (2026-06-14)


### Features

* **docs-site:** landing-page redesign — sidebar, hero, cards, footer (engram-45i) ([#133](https://github.com/seanb4t/engram/issues/133)) ([4dbc5ff](https://github.com/seanb4t/engram/commit/4dbc5ff365543a37c48ce9587cf5e185a28af759))

## [0.7.4](https://github.com/seanb4t/engram/compare/v0.7.3...v0.7.4) (2026-06-13)


### Features

* engram brand system — console identity + docs-site unification ([#127](https://github.com/seanb4t/engram/issues/127)) ([9b55779](https://github.com/seanb4t/engram/commit/9b55779c0936d4dba5731d1abafdf5da01e4839a))


### Bug Fixes

* **brand:** --cat-discovery token, dark-scheme favicon tuning, light/dark docs logo, accent dedupe ([#129](https://github.com/seanb4t/engram/issues/129)) ([7e61303](https://github.com/seanb4t/engram/commit/7e61303c2e2233ea6a36f99f6869d8581328769c))

## [0.7.3](https://github.com/seanb4t/engram/compare/v0.7.2...v0.7.3) (2026-06-13)


### Features

* **cmd:** add 'engram reindex' for embedder/model migration ([#117](https://github.com/seanb4t/engram/issues/117)) ([#123](https://github.com/seanb4t/engram/issues/123)) ([987045c](https://github.com/seanb4t/engram/commit/987045c42aaeb4ef69abb1a2792f10c8801bc6e7))

## [0.7.2](https://github.com/seanb4t/engram/compare/v0.7.1...v0.7.2) (2026-06-13)


### Features

* scheduled/future memories (temporal validity window) ([#119](https://github.com/seanb4t/engram/issues/119)) ([74b1247](https://github.com/seanb4t/engram/commit/74b1247872d55a33ce1235d82237f66b552b1120))


### Bug Fixes

* **ui:** distinct 'select a scope' empty state + console test coverage (engram-2b0) ([#120](https://github.com/seanb4t/engram/issues/120)) ([8443cb8](https://github.com/seanb4t/engram/commit/8443cb82fc7cedd888cd198bbf08fcc822542b39))

## [0.7.1](https://github.com/seanb4t/engram/compare/v0.7.0...v0.7.1) (2026-06-13)


### Features

* **ui:** operator console redesign — shadcn-forward shell, components, routes (engram-kco) ([#113](https://github.com/seanb4t/engram/issues/113)) ([b9353df](https://github.com/seanb4t/engram/commit/b9353df82b96ddf12740e93aa0a8702ee9a4e963))

## [0.7.0](https://github.com/seanb4t/engram/compare/v0.6.1...v0.7.0) (2026-06-12)


### ⚠ BREAKING CHANGES

* **serve:** configurable MCP transport path (MEM_MCP_PATH); console at root when UI enabled ([#108](https://github.com/seanb4t/engram/issues/108)) (#111)

### Features

* **serve:** configurable MCP transport path (MEM_MCP_PATH); console at root when UI enabled ([#108](https://github.com/seanb4t/engram/issues/108)) ([#111](https://github.com/seanb4t/engram/issues/111)) ([a4e4c62](https://github.com/seanb4t/engram/commit/a4e4c62072a48ca9b50482b58906ae48d68e5362))


### Bug Fixes

* **webauth:** embed SvelteKit _app assets via //go:embed all: ([#106](https://github.com/seanb4t/engram/issues/106)) ([#109](https://github.com/seanb4t/engram/issues/109)) ([aa663e2](https://github.com/seanb4t/engram/commit/aa663e2714680087e21e289e4f8d5c783d524404))

## [0.6.1](https://github.com/seanb4t/engram/compare/v0.6.0...v0.6.1) (2026-06-12)


### Features

* **auth:** decouple web-UI OIDC issuer from MCP bearer issuer (MEM_UI_ISSUER) ([#105](https://github.com/seanb4t/engram/issues/105)) ([e8f7a4b](https://github.com/seanb4t/engram/commit/e8f7a4b743bb7651b99c3fe852940dd24b754be1))


### Bug Fixes

* **telemetry:** tolerate partial OTel resource detection on distroless ([#103](https://github.com/seanb4t/engram/issues/103)) ([8cec041](https://github.com/seanb4t/engram/commit/8cec04136ad1e861b0c52fbdaf269a1da39a6f5e))

## [0.6.0](https://github.com/seanb4t/engram/compare/v0.5.2...v0.6.0) (2026-06-12)


### ⚠ BREAKING CHANGES

* cookie/OIDC auth lane for the Connect web UI (R1–R4) ([#67](https://github.com/seanb4t/engram/issues/67))

### Features

* **chart:** expose web-UI lane env vars (MEM_UI_*/MEM_OIDC_CLIENT_*) ([#91](https://github.com/seanb4t/engram/issues/91)) ([16253f5](https://github.com/seanb4t/engram/commit/16253f5585ec5d71669c5f3c24e3f96054d00cd0))
* **chart:** optional Ingress / Gateway API HTTPRoute to expose the console ([#99](https://github.com/seanb4t/engram/issues/99)) ([42178a9](https://github.com/seanb4t/engram/commit/42178a9f592c4c8123869977e0027d6c88084540))
* cookie/OIDC auth lane for the Connect web UI (R1–R4) ([#67](https://github.com/seanb4t/engram/issues/67)) ([5ee3982](https://github.com/seanb4t/engram/commit/5ee39827eafb53f78b164596e5c9e3e3ce55c02f))
* engram operator-console SPA (v1 observe) ([#79](https://github.com/seanb4t/engram/issues/79)) ([54ede5e](https://github.com/seanb4t/engram/commit/54ede5ec183be026281c2116e7fdde56a72bb3d6))
* engram web UI v1 backend API foundation (EngramService Connect) ([#62](https://github.com/seanb4t/engram/issues/62)) ([97b22ff](https://github.com/seanb4t/engram/commit/97b22ff663d893ccaa11af267cb89cb0044c8a9f))
* **telemetry:** instrumentation depth at every seam ([#88](https://github.com/seanb4t/engram/issues/88)) ([09659de](https://github.com/seanb4t/engram/commit/09659de2c5e9aaf60506c77317c18b9537a25b28))


### Bug Fixes

* **deps:** migrate @tanstack/svelte-query to v6 (runes API) ([#98](https://github.com/seanb4t/engram/issues/98)) ([68dbe59](https://github.com/seanb4t/engram/commit/68dbe59402b774b1d54b321e6548c52d00e4fe9a))
* **webauth:** nil-dependency guards in NewHandler/NewResolver ([#92](https://github.com/seanb4t/engram/issues/92)) ([d96477a](https://github.com/seanb4t/engram/commit/d96477a6718bd091b76c9e82b66686f855ad39f7))

## [0.5.2](https://github.com/seanb4t/engram/compare/v0.5.1...v0.5.2) (2026-06-09)


### Features

* **migrate:** harden migrate-set-owner (timeout/cancel) + isolation test hardening ([#58](https://github.com/seanb4t/engram/issues/58)) ([8710b70](https://github.com/seanb4t/engram/commit/8710b70cd359fc1e875b72ab39aa244b0421302e))
* **telemetry:** include trace_id/span_id in stdout logs ([#52](https://github.com/seanb4t/engram/issues/52)) ([0fefc14](https://github.com/seanb4t/engram/commit/0fefc1467df1bfb705fd1accc4cbeb11b3d70ab5))


### Bug Fixes

* **authz:** fail-closed hardening for the anonymous bucket (epic engram-dg5) ([#54](https://github.com/seanb4t/engram/issues/54)) ([fb44bf0](https://github.com/seanb4t/engram/commit/fb44bf028aa60e757cc0b1305b17dea4b70676c9))

## [0.5.1](https://github.com/seanb4t/engram/compare/v0.5.0...v0.5.1) (2026-06-07)


### Features

* **chart:** internal-CA trust for embedder + secret-backed OTLP headers ([#48](https://github.com/seanb4t/engram/issues/48)) ([e93d3d0](https://github.com/seanb4t/engram/commit/e93d3d06d53a3db6d1d57ae748f5247af4e82b24))

## [0.5.0](https://github.com/seanb4t/engram/compare/v0.4.3...v0.5.0) (2026-06-07)


### Features

* **observability:** structured slog logging + OpenTelemetry (engram-ew7) ([#44](https://github.com/seanb4t/engram/issues/44)) ([0b074e3](https://github.com/seanb4t/engram/commit/0b074e387baa5026817d610b2aa885ae76e7a7d1))


### Bug Fixes

* **deps:** update opentelemetry-go-contrib monorepo to v0.69.0 ([#46](https://github.com/seanb4t/engram/issues/46)) ([3cae175](https://github.com/seanb4t/engram/commit/3cae175851087c671b37a34120661e73aa637b17))


### Miscellaneous Chores

* release 0.5.0 ([#47](https://github.com/seanb4t/engram/issues/47)) ([35ab973](https://github.com/seanb4t/engram/commit/35ab973984e04a11d2201639e9fd00829330a2eb))

## [0.4.3](https://github.com/seanb4t/engram/compare/v0.4.2...v0.4.3) (2026-06-07)


### Bug Fixes

* **server:** store_discovery jsonschema tag crashes server at startup ([#41](https://github.com/seanb4t/engram/issues/41)) ([234df4c](https://github.com/seanb4t/engram/commit/234df4cabd590351edcf9bbdb32a38c346a0208e))

## [0.4.2](https://github.com/seanb4t/engram/compare/v0.4.1...v0.4.2) (2026-06-06)


### Features

* per-actor memory isolation (authn → authz) (engram-99z) ([#38](https://github.com/seanb4t/engram/issues/38)) ([bfa6470](https://github.com/seanb4t/engram/commit/bfa6470e144878bdf57c95334cae85c6b35c8736))

## [0.4.1](https://github.com/seanb4t/engram/compare/v0.4.0...v0.4.1) (2026-06-06)

### Miscellaneous Chores

* release v0.4.1 ([#34](https://github.com/seanb4t/engram/issues/34)) ([692c941](https://github.com/seanb4t/engram/commit/692c941dca5fbd800bb9883e328aeeac7b58f557))
