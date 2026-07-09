<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Centralized config validation (`Config.Validate`)

- **Design bead:** engram-syt
- **Status:** design
- **Author:** Sean Brandt
- **Date:** 2026-06-14

## Problem

PR #139 review finding `engram-ii6.3`/`ii6.6` asked engram to "fail loudly when
required config fields resolve empty." Grounding the request against current
`main` (post-#139) showed the literal framing is already largely covered, which
reshaped the work:

1. **The named fields cannot resolve empty today.** In `internal/config/registry.go`,
   `qdrant.addr` (default `localhost:6334`) and `openai.base_url`
   (default `http://localhost:4000`) both carry a non-empty `Default` and have
   **no `Flag`**. `config.Load`'s `TransformFunc` makes an empty env var *preserve
   the default*, and with no flag there is no way to force-empty them. The exact
   failure mode the finding targets is unreachable.

2. **`qdrant.addr` and `embed.dim` are already fail-fast.** `StoreFromEnvNoEnsure`
   runs `net.SplitHostPort(cfg.Qdrant.Addr)` + `strconv.Atoi(port)` and
   `strconv.ParseUint(cfg.Embed.Dim, …)` at startup; an empty or malformed value
   already errors loudly (`invalid ENGRAM_QDRANT_ADDR …`). UI/OIDC credential
   emptiness is already validated by `resolveUIConfig`.

What is *actually* missing is not "empty checks" but a **single, authoritative
statement of what well-formed config means**, run uniformly at startup. Today the
well-formedness rules are scattered: addr-parsing lives in `StoreFromEnvNoEnsure`,
dim-parsing beside it, MCP-path rules in `resolveMCPPath`, UI-cred rules in
`resolveUIConfig`. There is no one place a reader (or a new field) can consult to
answer "what makes this config valid?", and error messages are inconsistent in
shape and timing (some at store-build, some mid-request).

## Goals

- Add a single pure `Config.Validate() error` in `internal/config` that asserts
  the well-formedness of the **data-plane** fields every command depends on.
- Make validation failures **loud, early, and aggregated**: every problem reported
  at once, at command startup, before any store/embedder construction or network
  bind.
- Run validation at **every entrypoint** (`serve`, `reindex`, `migrate`,
  `prune`), so admin commands get the same uniform early error as the server.
- Give well-formedness one home, so adding a config field means adding one rule in
  one obvious place.

## Non-goals

- **No new validation *inside* `config.Load`.** `Load` stays pure *assembly*.
  Folding validation into `Load` would break the invariant that `Load` only fails
  on a malformed koanf layer (a programming error) — the invariant that justifies
  `EmbedderFromEnv` panicking on `Load` error (see `engram-edv`, closed wontfix).
  Operator config typos must surface as ordinary returned errors, never panics.
- **No fix to the existing double-`config.Load` in `serve`.** `serve` loads config
  once with flags and again via `StoreFromEnv`'s `Load(nil)`. That is a separate
  tracked follow-up; this work places `Validate` without entangling it and does
  not refactor the double-load.
- **No removal of the in-place parses** in `StoreFromEnvNoEnsure`. Those parse
  values *for use* (they build the Qdrant client's host/port). `Validate` runs
  earlier and reports uniformly; the shallow duplication (validate-for-report vs
  parse-for-use) is acceptable.
- **No validation of optional or already-validated fields.** `openai.api_key`
  (optional — local Ollama needs none), `oidc.*`/`ui.*` (optional or validated by
  `resolveUIConfig`/auth), and `log.*` (handled by telemetry) are out of scope.
- **No strict/prod mode** that requires connection fields be explicitly set
  (rejecting the dev-friendly localhost defaults). Explicitly out of scope; the
  localhost defaults stay.

## Decisions

### D1 — `Validate` is a separate explicit call, not part of `Load`

`config.Load` remains assembly-only. Each command calls `cfg.Validate()` itself
after the config is available. Rationale: preserves `Load`'s "only fails on
malformed koanf" invariant and keeps `EmbedderFromEnv`'s panic-on-`Load`-error
correct (per the `engram-edv` decision). Cost: each entrypoint must remember the
call — accepted, and covered by tests.

### D2 — `Validate` covers the data-plane field set only

| Field | Rule |
|---|---|
| `qdrant.addr` | non-empty; parses as `host:port`; port ∈ 1–65535 (the range bound is a `Validate`-only tightening — the consumer's `Atoi` does not range-check) |
| `qdrant.collection` | non-empty |
| `embed.model` | non-empty |
| `embed.dim` | parses as `uint`; value > 0 |
| `openai.base_url` | non-empty; parses as URL with `http`/`https` scheme |

These are exactly the fields `StoreFromEnv`/`EmbedderFromEnv` consume, so they are
universal to every command. Serve-only and conditional fields are deliberately
excluded so admin commands (which load the same `Config`) never trip on config
they do not use.

### D3 — Aggregated errors via `errors.Join`

`Validate` accumulates all field failures and returns them joined, so an operator
sees every problem in one run rather than fixing-and-rerunning. Each wrapped error
names the `ENGRAM_*` env var (not the koanf key) so the message is actionable for
env-based deployments. Returns `nil` when all rules pass.

### D4 — `listen_addr` is a serve-local guard

`server.listen_addr` is serve-specific (admin commands bind no listener) and is
the one data-ish field that *can* be force-emptied (it has a `--listen-addr`
flag, and `--listen-addr ""` would make `http.Server` bind `:http` silently). It
is therefore validated by a one-line non-empty guard in `runServe`, not by the
shared `Validate`.

## Design

### Surface

```go
// Validate reports whether c's data-plane fields are well-formed. It is pure
// (no I/O) and aggregates every failure via errors.Join, so a caller sees all
// problems at once. Errors name the ENGRAM_* env var, not the koanf key.
func (c *Config) Validate() error
```

### Behavior

- Runs each D2 rule, collecting errors into a slice; returns `errors.Join(errs...)`
  (which is `nil` when empty).
- For `qdrant.addr` and `embed.dim`, `Validate` reuses the *same* primitives the
  consumer runs (`net.SplitHostPort`/`strconv.Atoi`, `strconv.ParseUint`), so
  "valid per `Validate`" and "parses in `StoreFromEnv`" cannot drift for those.
- For `openai.base_url` (`net/url.Parse`, http/https scheme) and the
  `qdrant.addr` port-range bound (1–65535), `Validate` is intentionally
  *stricter* than any consumer: the embedder only concatenates `base_url +
  "/v1/embeddings"` (no parse), and `StoreFromEnv`'s `Atoi` has no range check.
  These are validation-only rules that turn a late/opaque failure (a bad URL
  surfacing on the first embed call; an out-of-range port) into an early, named
  startup error.
- No logging, no `os.Exit` — pure value → error. Callers decide how to surface it.

### Call sites

Each command, immediately after it has a resolved `*Config`, calls `Validate` and
returns its error (cobra prints it and exits non-zero):

- `serve` (`runServe`): after `config.Load(cmd.Flags())`, before building
  telemetry/store/listener; plus the D4 `listen_addr` guard.
- `reindex`, `migrate`, `prune`: each loads config (or reuses the load it already
  performs) and calls `Validate` before constructing the store.

The planning phase resolves the precise mechanics for the admin commands. The
material constraint: `reindex`/`migrate`/`prune` live in `cmd/engram` but reach
config only *through* `server.StoreFromEnv`/`StoreFromEnvNoEnsure`, which embed
`config.Load(nil)` internally. The two realistic approaches, each with a different
package-boundary cost:

1. **Explicit at `RunE`:** each admin command calls `config.Load(nil)` +
   `cfg.Validate()` at its `RunE` before constructing the store. Visible and
   uniform, but adds another `config.Load` alongside the one already inside
   `StoreFromEnv` (compounding the double-load wart).
2. **Validated store helper in `server`:** `StoreFromEnv` (or a sibling) calls
   `cfg.Validate()` on the config it already loads, so every store-building path
   is covered by one call site. Avoids a new `Load`, but moves the validation
   trigger out of the command and into the `server` package.

The plan picks one; this spec does not, and does not fix the double-load.

### Error shape (illustrative)

```text
invalid configuration:
  ENGRAM_QDRANT_ADDR "": must be host:port
  ENGRAM_OPENAI_BASE_URL "ftp://x": must be an http(s) URL
```

## Testing

- **`Validate` is pure → table-driven unit tests** in `internal/config`: one case
  per rule (valid; empty; malformed) plus a multi-failure case asserting
  aggregation reports *all* problems and the happy path returns `nil`. Assert each
  error names its `ENGRAM_*` var.
- **Call-site coverage:** a test per entrypoint (or a shared helper) asserting an
  invalid `Config` causes the command to fail fast with the aggregated error
  before any store/network construction.
- Existing `StoreFromEnvNoEnsure` parse tests are unchanged (the in-place parses
  stay).

## Out of scope / follow-ups

- The serve double-`config.Load` refactor (separately tracked).
- A strict/prod mode requiring connection fields be explicitly set.
- File-based config validation (no config file exists).
<!-- adr-capture: sha256=0e0f9a1786afed2f; session=cli; ts=2026-06-15T00:01:25Z; adrs=engram-wtw,engram-d24 -->
