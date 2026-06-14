<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-jgq; do not edit manually; use `/adr update engram-jgq` -->

# Unify config under ENGRAM_ prefix via koanf internal/config

**Date:** 2026-06-14
**Status:** Accepted
**Decision:** engram-jgq
**Deciders:** Sean Brandt

## Context

Engram's 23 env vars carried a `MEM_*` prefix — a vestige of a prior name — inconsistent with the binary, Helm chart, skill, and brand. Config reads were scattered across `server.EnvOr`, getenv closures in `cmd/engram`, and `internal/telemetry/config.go`, with no single registry, making renames expensive and drift-prone. The project mandates cobra with **no viper**; koanf is the lightweight, modular realization of the required env-first + flag-override layered model.

## Decision

Introduce `internal/config` (koanf v2) with a single field registry as the sole owner of all `ENGRAM_*` keys, their defaults, the env→koanf-path transform, and the legacy-env guard. Config is loaded as koanf layers — registry defaults, then the `ENGRAM_` env layer, then a changed-CLI-flags overlay — and unmarshalled into a typed nested `*Config`. Env-only consumers call `config.Load(nil)`; `serve` passes `cmd.Flags()`. Retire `server.EnvOr`.

## Rationale

- koanf is the lightweight, modular realization of the project's existing env-first + flag-override requirement without viper.
- A single field registry makes renames a one-line edit; the env transform, defaults, and legacy guard all derive from it and cannot drift.
- An explicit env→key-path mapping (not a blind `_`→`.` rewrite) prevents over-splitting (e.g. `BASE_URL` → `base.url`).
- A changed-flag-only overlay gives flags-override-env precedence while never letting an unset flag clobber an env value — the property the PR #105 UI-issuer fix had to hand-roll.

## Alternatives Considered

- **Keep scattered EnvOr / getenv reads, rename in place** — minimal diff, no new dependency, but every future rename touches every call site and the key catalogue has no single owner; transform/defaults/guard drift is unchecked. Rejected.
- **Adopt viper** — widely known, built-in env+flag+file, but explicitly prohibited by CLAUDE.md's cobra/no-viper convention. Rejected.

## Consequences

- **Positive:** all config keys owned in one place; future renames are one-line registry edits; consumers receive a typed nested `*Config` with no ambient getenv calls; changed-flag-only override is correct by construction.
- **Negative:** new koanf v2 dependency; all existing `EnvOr`/getenv call sites must migrate to consume `*config.Config`.
- **Neutral:** CLI flag names are unchanged (only env-based deployments migrate); a file-based config provider is out of scope but possible later.
