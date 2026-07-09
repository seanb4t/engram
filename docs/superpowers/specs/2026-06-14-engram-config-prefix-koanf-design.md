<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Config prefix `ENGRAM_`, provider-neutral embedder, and a koanf-backed `internal/config`

- **Design bead:** engram-xv9
- **Status:** design
- **Author:** Sean Brandt
- **Date:** 2026-06-14

## Problem

Two related problems in engram's configuration surface:

1. **The embedder vars leak an internal implementation choice.** `MEM_LITELLM_URL`
   and `MEM_LITELLM_KEY` name *LiteLLM* — the proxy a particular homelab happens to
   front its embeddings with. The server only ever needed an **OpenAI-compatible
   `/v1/embeddings` endpoint**; LiteLLM is one of many backends that speak that
   protocol (Ollama, vLLM, TEI, OpenAI itself). The name confuses readers of both
   the code and the docs, and ties a generic project to one operator's topology.

2. **The whole config surface is prefixed `MEM_`.** The project is named *engram*.
   `MEM_*` is a vestige of an earlier name and is inconsistent with the binary,
   the Helm chart (`charts/engram`), the skill (`skill/engram`), and the brand.
   There are 23 such variables (20 server-config + 3 command-local).

A third, structural problem surfaces once the rename is on the table: **config
reads are scattered.** Env vars are read ad hoc via `server.EnvOr` (used as cobra
flag *defaults*, evaluated at registration time), via getenv closures in
`cmd/engram/serve.go`/`uiconfig.go`, and directly in `internal/telemetry/config.go`.
A blind rename would have to touch every one of those sites, and the next rename
would too. There is no single place that owns "what are engram's config keys."

## Goals

- Rename every `MEM_*` env var to `ENGRAM_*`.
- Rename the embedder connection vars to provider-neutral, protocol-named keys:
  `ENGRAM_OPENAI_BASE_URL` / `ENGRAM_OPENAI_API_KEY`.
- Remove LiteLLM-specific language from code comments and docs.
- Introduce a typed `internal/config` package backed by **koanf** that owns all
  keys, defaults, the env→config mapping, and flag-override precedence — so this
  rename and future ones are a one-line registry edit.
- Make the hard break **loud, not silent**: a startup guard that fails fast when a
  retired `MEM_*` var is still set, naming its `ENGRAM_*` replacement.

## Non-goals

- **No dual-read / no `MEM_*` compatibility shim.** This is a hard break shipped
  pre-1.0 as `feat!:`. (Decision below.)
- **No file-based config** (no YAML/TOML config file). koanf is adopted only for its
  env + flag providers and typed unmarshal; a file provider may be added later but
  is out of scope here.
- **No CLI flag renames.** No flag carries the `MEM_` prefix (`--oidc-issuer`,
  `--listen-addr`, `--mcp-path`, …); flag names are unchanged, so flag-based
  deployments are unaffected. Only env-based deployments migrate.
- **No change to the memory data model, scopes, auth model, or tool contract.**

## Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Hard break**, no dual-read. | Pre-1.0 (v0.7.x); release-please treats `feat!:` as a minor bump pre-1.0, so the break is cheap. A dual-read shim would keep the leak/confusion alive indefinitely. |
| D2 | Embedder connection vars → `ENGRAM_OPENAI_BASE_URL` / `ENGRAM_OPENAI_API_KEY`. | Mirrors the OpenAI SDK's own canonical env vars (`OPENAI_BASE_URL` / `OPENAI_API_KEY`); "OpenAI" names the **wire protocol**, not an impl. Verified against `/openai/openai-python`: the SDK documents `base_url` as the mechanism for OpenAI-compatible APIs. |
| D3 | Embedder *model/dimension* vars keep the `EMBED_` stem: `ENGRAM_EMBED_MODEL` / `ENGRAM_EMBED_DIM`. | Two clean families: `ENGRAM_OPENAI_*` is the protocol connection; `ENGRAM_EMBED_*` is engram's embedding choice. The OpenAI SDK has no env var for model, so grouping model under `OPENAI_` would invent a non-standard name. |
| D4 | Introduce `internal/config` backed by **koanf v2**. | CLAUDE.md already mandates "cobra; no viper — env-first with flag overrides." koanf is the lightweight, modular, no-viper realization of exactly that layered model. |
| D5 | A single **field registry** in `internal/config` is the sole source of truth. | The env transform, defaults, and the legacy guard all derive from it; renames become one-line edits and the three can never drift apart. |
| D6 | **Legacy-env guard is fatal**, at root `PersistentPreRunE`. | Converts the hard break's silent footgun (ignored var → silent fallback to a default → wrong/garbage embeddings) into a loud, actionable startup failure. Applies to every subcommand because `reindex`/`migrate-set-owner`/`prune-expired` read the same embedder/Qdrant env. |
| D7 | koanf keys are **nested** (`openai.base_url`, `qdrant.addr`, `ui.issuer`). | Maps to sub-structs on `Config`, keeps consumers narrow (`cfg.UI`, `cfg.OpenAI`). Nesting comes from the registry's explicit key paths, not a blind `_`→`.` transform (which would over-split `BASE_URL` → `base.url`). |

## Architecture

### The `internal/config` package

```text
internal/config/
  config.go      // Config struct (+ sub-structs), Load(), Unmarshal
  registry.go    // the field registry: the single source of truth
  legacy.go      // CheckLegacy(): fatal guard derived from the registry
  config_test.go
```

**`Config`** — typed, nested:

```go
type Config struct {
    Server ServerConfig // ListenAddr, MCPPath
    Qdrant QdrantConfig // Addr, Collection
    Embed  EmbedConfig  // Model, Dim
    OpenAI OpenAIConfig // BaseURL, APIKey
    OIDC   OIDCConfig   // Issuer, Audience, ClientID, ClientSecret, ResourceMetadata
    UI     UIConfig     // Enabled, Issuer, RedirectURL, CookieKey
    Log    LogConfig    // Level, Format, Stdout
}
```

**The field registry** — one entry per config key; everything derives from it:

```go
type field struct {
    Key     string // koanf key path, e.g. "openai.base_url"
    Env     string // current env var,  e.g. "ENGRAM_OPENAI_BASE_URL"
    Legacy  string // retired env var,  e.g. "MEM_LITELLM_URL" ("" if brand-new)
    Default string // default value, "" if none
}

var registry = []field{
    {"server.listen_addr",    "ENGRAM_LISTEN_ADDR",          "MEM_LISTEN_ADDR",          ":8080"},
    {"server.mcp_path",       "ENGRAM_MCP_PATH",             "MEM_MCP_PATH",             ""},
    {"qdrant.addr",           "ENGRAM_QDRANT_ADDR",          "MEM_QDRANT_ADDR",          "localhost:6334"},
    {"qdrant.collection",     "ENGRAM_QDRANT_COLLECTION",    "MEM_QDRANT_COLLECTION",    "mem_eval"},
    {"embed.model",           "ENGRAM_EMBED_MODEL",          "MEM_EMBED_MODEL",          "ollama/bge-m3"},
    {"embed.dim",             "ENGRAM_EMBED_DIM",            "MEM_EMBED_DIM",            "1024"},
    {"openai.base_url",       "ENGRAM_OPENAI_BASE_URL",      "MEM_LITELLM_URL",          "http://localhost:4000"},
    {"openai.api_key",        "ENGRAM_OPENAI_API_KEY",       "MEM_LITELLM_KEY",          ""},
    {"oidc.issuer",           "ENGRAM_OIDC_ISSUER",          "MEM_OIDC_ISSUER",          ""},
    {"oidc.audience",         "ENGRAM_OIDC_AUDIENCE",        "MEM_OIDC_AUDIENCE",        ""},
    {"oidc.client_id",        "ENGRAM_OIDC_CLIENT_ID",       "MEM_OIDC_CLIENT_ID",       ""},
    {"oidc.client_secret",    "ENGRAM_OIDC_CLIENT_SECRET",   "MEM_OIDC_CLIENT_SECRET",   ""},
    {"oidc.resource_metadata","ENGRAM_OIDC_RESOURCE_METADATA","MEM_OIDC_RESOURCE_METADATA",""},
    {"ui.enabled",            "ENGRAM_UI_ENABLED",           "MEM_UI_ENABLED",           ""},
    {"ui.issuer",             "ENGRAM_UI_ISSUER",            "MEM_UI_ISSUER",            ""},
    {"ui.redirect_url",       "ENGRAM_UI_REDIRECT_URL",      "MEM_UI_REDIRECT_URL",      ""},
    {"ui.cookie_key",         "ENGRAM_UI_COOKIE_KEY",        "MEM_UI_COOKIE_KEY",        ""},
    {"log.level",             "ENGRAM_LOG_LEVEL",            "MEM_LOG_LEVEL",            "info"},
    {"log.format",            "ENGRAM_LOG_FORMAT",           "MEM_LOG_FORMAT",           "json"},
    {"log.stdout",            "ENGRAM_LOG_STDOUT",           "MEM_LOG_STDOUT",           "true"},
}
```

> The admin-command vars `MEM_MIGRATE_OWNER` → `ENGRAM_MIGRATE_OWNER` and
> `MEM_REINDEX_TARGET` → `ENGRAM_REINDEX_TARGET`, and the test-only
> `MEM_QDRANT_TEST_ADDR` → `ENGRAM_QDRANT_TEST_ADDR`, are part of the rename and
> the legacy guard but are command-local (read in `migrate.go`/`reindex.go`/tests),
> not part of the server `Config` struct. The plan decides whether to fold them
> into the registry or keep them as command-scoped reads; either way their legacy
> names are registered with the guard.

### Load precedence

```go
// Load builds Config from defaults, the ENGRAM_ env layer, and (optionally) a
// cobra/pflag flagset whose explicitly-set flags override env.
func Load(flags *pflag.FlagSet) (*Config, error) {
    k := koanf.New(".")
    // 1. defaults (registry) — lowest precedence
    k.Load(confmap.Provider(defaultsFromRegistry(), "."), nil)
    // 2. ENGRAM_ env layer — overrides defaults
    k.Load(env.Provider(".", env.Opt{Prefix: "ENGRAM_", TransformFunc: registryEnvTransform}), nil)
    // 3. flags — override env, but ONLY when explicitly set on the command line
    if flags != nil {
        k.Load(posflag.Provider(flags, ".", k), nil)
    }
    var c Config
    return &c, k.Unmarshal("", &c)
}
```

`registryEnvTransform` looks each `ENGRAM_*` var up in the registry and returns its
`Key` path (explicit mapping, not a blind `_`→`.` rewrite). Passing the koanf
instance `k` to `posflag.Provider` is load-bearing: it makes posflag override a key
only when the flag was *changed*, so an unset flag's default does not clobber an
env-provided value. (This is the runtime-resolution property the UI-issuer fix in
PR #105 had to hand-roll; koanf gives it for free.)

### Consumer migration

- `server.EnvOr` is **retired**. `StoreFromEnv` / `StoreFromEnvNoEnsure` /
  `EmbedderFromEnv` take a `*config.Config` (or the relevant sub-struct) instead of
  reading env. The store stays embedder-agnostic; `EmbedderFromEnv` becomes
  `EmbedderFromConfig(cfg.OpenAI, cfg.Embed)` building `embed.New(cfg.OpenAI.BaseURL,
  cfg.OpenAI.APIKey, cfg.Embed.Model, …)` — `embed.New`'s signature is **already**
  provider-neutral (`baseURL, apiKey, model`), so only the call site and var names
  change.
- `cmd/engram/serve.go` registers flags with plain literal defaults (no
  `EnvOr(...)` default), calls `config.Load(cmd.Flags())` in `RunE`, and threads
  the typed `*Config` into `server.Register` / `resolveUIConfig`.
- `resolveUIConfig` (the tested seam, `cmd/engram/uiconfig.go`) keeps its resolution
  logic (`ui.issuer` falls back to `oidc.issuer`) but reads from `cfg.UI`/`cfg.OIDC`
  instead of a getenv closure. Its test suite's cases (default fallback, override,
  alone, error) are preserved, re-expressed against `Config`.
- `internal/telemetry/config.go` reads its `ENGRAM_*` keys via the same loader (or
  is passed the relevant sub-struct).

### Legacy-env guard

```go
// CheckLegacy returns an error naming every retired MEM_* var still present in the
// environment, mapped to its ENGRAM_* replacement. Called from root PersistentPreRunE.
func CheckLegacy(environ []string) error
```

Derived from `registry` (the `Legacy` column) plus the command-local extras. On any
hit it returns a multi-line error:

```text
Error: retired environment variables are set and no longer read:
  MEM_LITELLM_URL  → ENGRAM_OPENAI_BASE_URL
  MEM_QDRANT_ADDR  → ENGRAM_QDRANT_ADDR
Rename them (see the v0.x migration notes) and retry.
```

`root.Execute()` already prints `Error: <msg>` to stderr and exits non-zero (the
SilenceErrors fix from the reindex work), so returning this from `PersistentPreRunE`
surfaces cleanly. The whole guard is deleted at 1.0 by removing the `Legacy` column.

## Data flow

```text
                ┌─ defaults (registry) ─┐
ENGRAM_* env ───┤  env.Provider         ├─ koanf ─ Unmarshal ─→ *config.Config
--flags ────────┘  posflag.Provider     ┘                          │
                                                                    ├─→ server.Register(cfg)
os.Environ ─→ config.CheckLegacy ─(fatal on MEM_* hit)              ├─→ resolveUIConfig(cfg)
   (PersistentPreRunE, before Load)                                 ├─→ telemetry(cfg.Log/…)
                                                                    └─→ EmbedderFromConfig(cfg.OpenAI, cfg.Embed)
```

## Error handling

- **Retired var present:** fatal at `PersistentPreRunE` (D6) — never a silent fallback.
- **Malformed value** (e.g. `ENGRAM_EMBED_DIM` not an int, `ENGRAM_QDRANT_ADDR` not
  host:port): `Load`/consumer returns a wrapped error naming the **new** key; same
  fail-fast behavior as today's `strconv.ParseUint`/`net.SplitHostPort` errors,
  re-pointed at the new names.
- **koanf load error:** wrapped and returned from `Load`; no partial/zero-value Config
  is used.
- The `ENGRAM_UI_ENABLED` tri-state ("" headless-if-creds | "true" | "false") is
  preserved as a string in `UIConfig.Enabled` and interpreted exactly as today; koanf
  does not coerce it (it stays a registry string default of "").

## Testing

- **`config_test.go`** (new): defaults-only Load; env overrides default; flag overrides
  env; unset flag does **not** clobber env (the posflag-instance property); nested-key
  unmarshal into sub-structs; malformed `embed.dim` errors; `CheckLegacy` returns the
  full mapping when multiple `MEM_*` are set and nil when none are.
- **`uiconfig_test.go`**: re-expressed against `Config` — keep all four existing cases
  (default fallback, `ui.issuer` override, `ui.issuer` alone, no-issuer error).
- **`tools_test.go` / `store_test.go` / `reindex_test.go` / `telemetry/config_test.go`**:
  `t.Setenv("MEM_…")` → `t.Setenv("ENGRAM_…")`; assertions on error strings re-pointed
  at new key names.
- **`mcproute_test.go` / `uiconfig_test.go`**: env-name updates.
- **Guard integration:** a test that sets a `MEM_*` var and asserts the command exits
  non-zero with the mapping line.

## Mechanical surfaces (no logic change)

- **Helm:** `charts/engram/values.yaml` + `templates/memory-mcp.yaml` — env var *names*
  only; the `memory.oidc.*` / `memory.ui.*` value keys are unchanged, so values.yaml
  churn is minimal. Follows the existing gating idioms (the `MEM_UI_ENABLED` tri-state
  `ne (… | toString) ""` gate, secret-backed `if .name` gates). `helm lint` + `helm
  template` gate it.
- **docs-site:** `guides/configure.md`, `guides/quickstart.md`, `guides/deploy.md`,
  `reference/auth.md`, `contributing/architecture.md` — rename all vars; the Embedder
  table's "OpenAI-compatible embeddings endpoint (LiteLLM or OpenAI)" wording drops the
  LiteLLM call-out → "OpenAI-compatible embeddings endpoint (point it at any backend
  that speaks the OpenAI `/v1/embeddings` API)".
- **skill:** `skill/engram/commands/engram-setup.md`; `skill/engram/hooks/tests/
  test_no_residual_memory_oauth.py`.
- **code comments:** de-LiteLLM `internal/embed/embed.go`, `internal/auth/auth.go`,
  `cmd/engram/serve.go` (→ "an OpenAI-compatible embeddings endpoint / gateway").
- **CLAUDE.md:** the "CLI: cobra; no viper" line gains koanf: "config is loaded by
  `internal/config` (koanf): env-first via the `ENGRAM_` prefix, with `--flag`
  overrides; no viper." Update the `tools.go (buildDepsFromEnv)` references and the
  `MEM_*` mentions in the Memory-contract / Auth sections.

## Release / migration

- One PR, `feat!:` with a `BREAKING CHANGE:` footer carrying the full `old → new`
  mapping table. release-please cuts a pre-1.0 minor and writes the CHANGELOG entry.
- The legacy guard is the operator's safety net: a partially-migrated deploy fails
  fast with the exact renames it still needs.
- Helm chart is a breaking change for chart users too; release-please bumps the chart
  version/appVersion as usual.

## ADR candidates (for `capture-adrs`)

1. **Config prefix is `ENGRAM_`, loaded via koanf in `internal/config`** (supersedes
   the implicit "scattered `MEM_*` getenv" status quo; records the no-viper rationale).
2. **The embedder is named by protocol (`OPENAI_*`), not by implementation.**
3. **Breaking config renames ship with a fatal legacy-env guard** (a reusable policy:
   hard breaks must be loud, never silent fallbacks).

## Open questions for the plan

- Fold the command-local vars (`MIGRATE_OWNER`, `REINDEX_TARGET`, `QDRANT_TEST_ADDR`)
  into the registry, or keep them as command-scoped reads with only their legacy names
  registered for the guard?
- Sequencing: `internal/config` + registry + tests first (lands green, unused), then
  migrate consumers, then mechanical sweep, then wire the guard — vs. one big-bang PR.
  (Recommended: staged within one PR, consumer-by-consumer, so each commit builds.)
<!-- adr-capture: sha256=7be96d255d62a478; session=cli; ts=2026-06-14T11:21:57Z; adrs=engram-jgq,engram-378,engram-irq -->
