# Changelog

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
