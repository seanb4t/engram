# Changelog

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
