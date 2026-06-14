<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# ENGRAM_ config prefix, provider-neutral embedder, koanf `internal/config` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename every `MEM_*` env var to `ENGRAM_*`, rename the embedder connection vars to provider-neutral `ENGRAM_OPENAI_BASE_URL` / `ENGRAM_OPENAI_API_KEY`, behind a new koanf-backed `internal/config` package with a single field registry and a fatal startup guard that catches retired `MEM_*` vars.

**Architecture:** A new `internal/config` package owns all config keys in one `registry` slice. `config.Load(flags)` layers koanf providers — registry defaults, then the `ENGRAM_`-prefixed env layer, then a changed-CLI-flags overlay (env-first, flags override) — and unmarshals into a typed nested `*Config`. Env-only consumers (store, embedder, telemetry) call `config.Load(nil)`; `serve` calls `config.Load(cmd.Flags())`. A registry-derived `CheckLegacy` runs at root `PersistentPreRunE` and fails fast when any retired `MEM_*` var is still set, naming its replacement. The break is hard (no dual-read) and ships pre-1.0 as `feat!`.

**Tech Stack:** Go 1.26, cobra, [koanf v2](https://github.com/knadh/koanf) (`providers/env/v2`, `providers/confmap`), spf13/pflag (already transitive via cobra), Helm, Astro/Starlight docs-site.

**Design bead:** engram-xv9 · **Spec:** `docs/superpowers/specs/2026-06-14-engram-config-prefix-koanf-design.md`

---

## Canonical rename mapping (all 23 vars)

The single source of truth for every task. Only the first two are *not* a mechanical `MEM_` → `ENGRAM_` swap.

| Old | New |
|-----|-----|
| `MEM_LITELLM_URL` | `ENGRAM_OPENAI_BASE_URL` |
| `MEM_LITELLM_KEY` | `ENGRAM_OPENAI_API_KEY` |
| `MEM_LISTEN_ADDR` | `ENGRAM_LISTEN_ADDR` |
| `MEM_MCP_PATH` | `ENGRAM_MCP_PATH` |
| `MEM_QDRANT_ADDR` | `ENGRAM_QDRANT_ADDR` |
| `MEM_QDRANT_COLLECTION` | `ENGRAM_QDRANT_COLLECTION` |
| `MEM_QDRANT_TEST_ADDR` | `ENGRAM_QDRANT_TEST_ADDR` |
| `MEM_EMBED_MODEL` | `ENGRAM_EMBED_MODEL` |
| `MEM_EMBED_DIM` | `ENGRAM_EMBED_DIM` |
| `MEM_OIDC_ISSUER` | `ENGRAM_OIDC_ISSUER` |
| `MEM_OIDC_AUDIENCE` | `ENGRAM_OIDC_AUDIENCE` |
| `MEM_OIDC_CLIENT_ID` | `ENGRAM_OIDC_CLIENT_ID` |
| `MEM_OIDC_CLIENT_SECRET` | `ENGRAM_OIDC_CLIENT_SECRET` |
| `MEM_OIDC_RESOURCE_METADATA` | `ENGRAM_OIDC_RESOURCE_METADATA` |
| `MEM_UI_ENABLED` | `ENGRAM_UI_ENABLED` |
| `MEM_UI_ISSUER` | `ENGRAM_UI_ISSUER` |
| `MEM_UI_REDIRECT_URL` | `ENGRAM_UI_REDIRECT_URL` |
| `MEM_UI_COOKIE_KEY` | `ENGRAM_UI_COOKIE_KEY` |
| `MEM_LOG_LEVEL` | `ENGRAM_LOG_LEVEL` |
| `MEM_LOG_FORMAT` | `ENGRAM_LOG_FORMAT` |
| `MEM_LOG_STDOUT` | `ENGRAM_LOG_STDOUT` |
| `MEM_MIGRATE_OWNER` | `ENGRAM_MIGRATE_OWNER` |
| `MEM_REINDEX_TARGET` | `ENGRAM_REINDEX_TARGET` |

---

## File Structure

**New:**

- `internal/config/config.go` — `Config` + sub-structs, `Load(flags)`, `FlagDefault(key)`.
- `internal/config/registry.go` — the `field` registry (single source of truth) + derived maps.
- `internal/config/legacy.go` — `CheckLegacy(environ)`.
- `internal/config/config_test.go` — Load precedence + unmarshal tests.
- `internal/config/legacy_test.go` — guard tests.

**Modified (Go):**

- `internal/server/tools.go` — `StoreFromEnvNoEnsure` / `EmbedderFromEnv` read `*config.Config`; retire `EnvOr`.
- `internal/telemetry/config.go` — `ConfigFromEnv` reads log fields via `config.Load(nil)`; drop local `envOr`.
- `cmd/engram/uiconfig.go` — `resolveUIConfig` takes a resolved struct, not a getenv closure; `ENGRAM_` names.
- `cmd/engram/serve.go` — flags registered without `EnvOr` defaults; `runServe` loads `config`; `withAuth` takes OIDC config.
- `cmd/engram/reindex.go`, `cmd/engram/migrate.go` — command-local flag defaults read `ENGRAM_*` directly.
- `cmd/engram/root.go` — `rootCmd.PersistentPreRunE` calls `config.CheckLegacy`.

**Modified (tests):** `internal/server/tools_test.go`, `internal/store/store_test.go`, `internal/store/reindex_test.go`, `internal/telemetry/config_test.go`, `internal/telemetry/providers_test.go`, `cmd/engram/uiconfig_test.go`, `cmd/engram/mcproute_test.go`.

**Modified (non-Go):** `charts/engram/values.yaml`, `charts/engram/templates/memory-mcp.yaml`, `docs-site/src/content/docs/guides/{configure,quickstart,deploy}.md`, `docs-site/src/content/docs/reference/auth.md`, `docs-site/src/content/docs/contributing/architecture.md`, `skill/engram/commands/engram-setup.md`, `skill/engram/hooks/tests/test_no_residual_memory_oauth.py`, `CLAUDE.md`, code comments in `internal/embed/embed.go` + `internal/auth/auth.go` + `cmd/engram/serve.go`.

---

## Task 1: `internal/config` core (registry + Load)

**Files:**

- Create: `internal/config/registry.go`, `internal/config/config.go`, `internal/config/config_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add koanf dependencies**

Run:

```bash
go get github.com/knadh/koanf/v2@latest
go get github.com/knadh/koanf/providers/env/v2@latest
go get github.com/knadh/koanf/providers/confmap@latest
go mod tidy
```

Expected: `go.mod` gains `github.com/knadh/koanf/v2`, `.../providers/env/v2`, `.../providers/confmap` (and transitive `go-viper/mapstructure`). `spf13/pflag` is already present (via cobra).

- [ ] **Step 2: Write the registry**

Create `internal/config/registry.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

// Prefix is the env-var namespace for all engram configuration.
const Prefix = "ENGRAM_"

// field is one configuration key. The registry of fields is the single source
// of truth: the env-var transform, the defaults layer, the flag overlay, and
// the legacy-env guard are all derived from it. Renaming a var is a one-line
// edit here.
type field struct {
	Key     string // koanf key path, e.g. "openai.base_url"
	Env     string // current env var, e.g. "ENGRAM_OPENAI_BASE_URL"
	Legacy  string // retired env var, e.g. "MEM_LITELLM_URL" ("" if brand-new)
	Flag    string // cobra flag name that overrides it ("" if env-only)
	Default string // default value ("" if none)
}

// registry holds every server-config key. Command-local vars (migrate/reindex
// targets, the test-only Qdrant addr) are NOT here — they are read directly by
// their command, but their legacy names are registered for the guard in
// legacy.go.
var registry = []field{
	{"server.listen_addr", "ENGRAM_LISTEN_ADDR", "MEM_LISTEN_ADDR", "listen-addr", ":8080"},
	{"server.mcp_path", "ENGRAM_MCP_PATH", "MEM_MCP_PATH", "mcp-path", ""},
	{"qdrant.addr", "ENGRAM_QDRANT_ADDR", "MEM_QDRANT_ADDR", "", "localhost:6334"},
	{"qdrant.collection", "ENGRAM_QDRANT_COLLECTION", "MEM_QDRANT_COLLECTION", "", "mem_eval"},
	{"embed.model", "ENGRAM_EMBED_MODEL", "MEM_EMBED_MODEL", "", "ollama/bge-m3"},
	{"embed.dim", "ENGRAM_EMBED_DIM", "MEM_EMBED_DIM", "", "1024"},
	{"openai.base_url", "ENGRAM_OPENAI_BASE_URL", "MEM_LITELLM_URL", "", "http://localhost:4000"},
	{"openai.api_key", "ENGRAM_OPENAI_API_KEY", "MEM_LITELLM_KEY", "", ""},
	{"oidc.issuer", "ENGRAM_OIDC_ISSUER", "MEM_OIDC_ISSUER", "oidc-issuer", ""},
	{"oidc.audience", "ENGRAM_OIDC_AUDIENCE", "MEM_OIDC_AUDIENCE", "oidc-audience", ""},
	{"oidc.client_id", "ENGRAM_OIDC_CLIENT_ID", "MEM_OIDC_CLIENT_ID", "oidc-client-id", ""},
	{"oidc.client_secret", "ENGRAM_OIDC_CLIENT_SECRET", "MEM_OIDC_CLIENT_SECRET", "oidc-client-secret", ""},
	{"oidc.resource_metadata", "ENGRAM_OIDC_RESOURCE_METADATA", "MEM_OIDC_RESOURCE_METADATA", "oidc-resource-metadata", ""},
	{"ui.enabled", "ENGRAM_UI_ENABLED", "MEM_UI_ENABLED", "ui-enabled", ""},
	{"ui.issuer", "ENGRAM_UI_ISSUER", "MEM_UI_ISSUER", "ui-issuer", ""},
	{"ui.redirect_url", "ENGRAM_UI_REDIRECT_URL", "MEM_UI_REDIRECT_URL", "ui-redirect-url", ""},
	{"ui.cookie_key", "ENGRAM_UI_COOKIE_KEY", "MEM_UI_COOKIE_KEY", "ui-cookie-key", ""},
	{"log.level", "ENGRAM_LOG_LEVEL", "MEM_LOG_LEVEL", "", "info"},
	{"log.format", "ENGRAM_LOG_FORMAT", "MEM_LOG_FORMAT", "", "json"},
	{"log.stdout", "ENGRAM_LOG_STDOUT", "MEM_LOG_STDOUT", "", "true"},
}

// envToKey maps each ENGRAM_* env var to its koanf key.
var envToKey = func() map[string]string {
	m := make(map[string]string, len(registry))
	for _, f := range registry {
		m[f.Env] = f.Key
	}
	return m
}()

// defaultsMap is the registry's defaults as a koanf confmap (empty defaults omitted).
func defaultsMap() map[string]any {
	m := make(map[string]any, len(registry))
	for _, f := range registry {
		if f.Default != "" {
			m[f.Key] = f.Default
		}
	}
	return m
}

// flagToKey maps a cobra flag name to its koanf key (only fields that have a flag).
var flagToKey = func() map[string]string {
	m := make(map[string]string)
	for _, f := range registry {
		if f.Flag != "" {
			m[f.Flag] = f.Key
		}
	}
	return m
}()

// FlagDefault returns the registry default for the field bound to flag name, so
// cobra flag registration shows accurate --help defaults without duplicating
// literals. Returns "" when the flag is unknown or its field has no default.
func FlagDefault(flagName string) string {
	key, ok := flagToKey[flagName]
	if !ok {
		return ""
	}
	for _, f := range registry {
		if f.Key == key {
			return f.Default
		}
	}
	return ""
}
```

- [ ] **Step 3: Write the failing test for Load precedence**

Create `internal/config/config_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"testing"

	flag "github.com/spf13/pflag"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OpenAI.BaseURL != "http://localhost:4000" {
		t.Errorf("BaseURL default = %q, want http://localhost:4000", cfg.OpenAI.BaseURL)
	}
	if cfg.Embed.Model != "ollama/bge-m3" {
		t.Errorf("Embed.Model default = %q", cfg.Embed.Model)
	}
	if cfg.Server.ListenAddr != ":8080" {
		t.Errorf("ListenAddr default = %q", cfg.Server.ListenAddr)
	}
}

func TestLoadEnvOverridesDefault(t *testing.T) {
	t.Setenv("ENGRAM_OPENAI_BASE_URL", "http://embed.internal:9000")
	t.Setenv("ENGRAM_QDRANT_COLLECTION", "prod")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OpenAI.BaseURL != "http://embed.internal:9000" {
		t.Errorf("BaseURL = %q, want env value", cfg.OpenAI.BaseURL)
	}
	if cfg.Qdrant.Collection != "prod" {
		t.Errorf("Collection = %q, want prod", cfg.Qdrant.Collection)
	}
}

func TestLoadChangedFlagOverridesEnv(t *testing.T) {
	t.Setenv("ENGRAM_OIDC_ISSUER", "https://env-issuer")
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	f.String("oidc-issuer", "", "")
	if err := f.Parse([]string{"--oidc-issuer", "https://flag-issuer"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDC.Issuer != "https://flag-issuer" {
		t.Errorf("Issuer = %q, want flag to override env", cfg.OIDC.Issuer)
	}
}

func TestLoadUnsetFlagDoesNotClobberEnv(t *testing.T) {
	t.Setenv("ENGRAM_OIDC_ISSUER", "https://env-issuer")
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	f.String("oidc-issuer", "", "")
	if err := f.Parse([]string{}); err != nil { // flag NOT set
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDC.Issuer != "https://env-issuer" {
		t.Errorf("Issuer = %q, want env value preserved (unset flag must not clobber)", cfg.OIDC.Issuer)
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `go test ./internal/config/ -run TestLoad -v`
Expected: FAIL — `undefined: Load` (config.go not written yet).

- [ ] **Step 5: Write `config.go`**

Create `internal/config/config.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package config loads engram's configuration. It is env-first (the ENGRAM_
// prefix) with CLI-flag overrides, realized as koanf layers: registry defaults,
// then the ENGRAM_ env layer, then a changed-flags overlay. No viper, no config
// file. The field registry (registry.go) is the single source of truth.
package config

import (
	"fmt"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
	flag "github.com/spf13/pflag"
)

// Config is engram's fully-resolved configuration. Values are kept as strings
// where the consumer already validates them (e.g. embed.dim is parsed by the
// store with a fail-fast error), so unmarshal never silently coerces.
type Config struct {
	Server ServerConfig `koanf:"server"`
	Qdrant QdrantConfig `koanf:"qdrant"`
	Embed  EmbedConfig  `koanf:"embed"`
	OpenAI OpenAIConfig `koanf:"openai"`
	OIDC   OIDCConfig   `koanf:"oidc"`
	UI     UIConfig     `koanf:"ui"`
	Log    LogConfig    `koanf:"log"`
}

type ServerConfig struct {
	ListenAddr string `koanf:"listen_addr"`
	MCPPath    string `koanf:"mcp_path"`
}

type QdrantConfig struct {
	Addr       string `koanf:"addr"`
	Collection string `koanf:"collection"`
}

type EmbedConfig struct {
	Model string `koanf:"model"`
	Dim   string `koanf:"dim"`
}

type OpenAIConfig struct {
	BaseURL string `koanf:"base_url"`
	APIKey  string `koanf:"api_key"`
}

type OIDCConfig struct {
	Issuer           string `koanf:"issuer"`
	Audience         string `koanf:"audience"`
	ClientID         string `koanf:"client_id"`
	ClientSecret     string `koanf:"client_secret"`
	ResourceMetadata string `koanf:"resource_metadata"`
}

type UIConfig struct {
	Enabled     string `koanf:"enabled"`
	Issuer      string `koanf:"issuer"`
	RedirectURL string `koanf:"redirect_url"`
	CookieKey   string `koanf:"cookie_key"`
}

type LogConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
	Stdout string `koanf:"stdout"`
}

// Load builds Config from registry defaults, the ENGRAM_ env layer, and — when
// flags is non-nil — an overlay of CLI flags that were explicitly set (env-first,
// changed flags override). Pass nil for env-only consumers (store, embedder,
// telemetry); pass cmd.Flags() from serve.
func Load(flags *flag.FlagSet) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaultsMap(), "."), nil); err != nil {
		return nil, fmt.Errorf("config defaults: %w", err)
	}

	// First arg is the koanf key-path DELIMITER (not the prefix). It is unused
	// here because TransformFunc returns already-dotted registry keys.
	if err := k.Load(env.Provider(".", env.Opt{
		Prefix: Prefix,
		TransformFunc: func(key, val string) (string, any) {
			if mapped, ok := envToKey[key]; ok {
				return mapped, val
			}
			return "", nil // ignore unknown ENGRAM_* vars
		},
	}), nil); err != nil {
		return nil, fmt.Errorf("config env: %w", err)
	}

	if flags != nil {
		overlay := map[string]any{}
		for name, key := range flagToKey {
			if flags.Changed(name) {
				v, err := flags.GetString(name)
				if err != nil {
					return nil, fmt.Errorf("config flag %q: %w", name, err)
				}
				overlay[key] = v
			}
		}
		if len(overlay) > 0 {
			if err := k.Load(confmap.Provider(overlay, "."), nil); err != nil {
				return nil, fmt.Errorf("config flags: %w", err)
			}
		}
	}

	var c Config
	if err := k.Unmarshal("", &c); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}
	return &c, nil
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/config/ -run TestLoad -v`
Expected: PASS (all four cases).

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(config): koanf-backed internal/config with field registry

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: Legacy-env guard

**Files:**

- Create: `internal/config/legacy.go`, `internal/config/legacy_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/legacy_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"
)

func TestCheckLegacyNoneSet(t *testing.T) {
	if err := CheckLegacy([]string{"PATH=/bin", "ENGRAM_OPENAI_BASE_URL=http://x"}); err != nil {
		t.Errorf("CheckLegacy with no MEM_* set = %v, want nil", err)
	}
}

func TestCheckLegacyReportsMapping(t *testing.T) {
	err := CheckLegacy([]string{
		"MEM_LITELLM_URL=http://old:4000",
		"MEM_QDRANT_ADDR=q:6334",
		"PATH=/bin",
	})
	if err == nil {
		t.Fatal("CheckLegacy with MEM_* set = nil, want error")
	}
	msg := err.Error()
	for _, want := range []string{
		"MEM_LITELLM_URL", "ENGRAM_OPENAI_BASE_URL",
		"MEM_QDRANT_ADDR", "ENGRAM_QDRANT_ADDR",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

func TestCheckLegacyCommandLocalVar(t *testing.T) {
	// Command-local legacy vars (not in the Config registry) are still caught.
	err := CheckLegacy([]string{"MEM_REINDEX_TARGET=foo"})
	if err == nil || !strings.Contains(err.Error(), "ENGRAM_REINDEX_TARGET") {
		t.Errorf("CheckLegacy(MEM_REINDEX_TARGET) = %v, want mapping to ENGRAM_REINDEX_TARGET", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/config/ -run TestCheckLegacy -v`
Expected: FAIL — `undefined: CheckLegacy`.

- [ ] **Step 3: Write `legacy.go`**

Create `internal/config/legacy.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"fmt"
	"sort"
	"strings"
)

// legacyMap is every retired MEM_* var → its ENGRAM_* replacement. It is the
// registry's Legacy column plus the command-local vars that are read directly
// by their command (and so are not in the Config registry). Delete this whole
// file at 1.0.
var legacyMap = func() map[string]string {
	m := map[string]string{
		// command-local (read in reindex.go / migrate.go / tests)
		"MEM_REINDEX_TARGET":   "ENGRAM_REINDEX_TARGET",
		"MEM_MIGRATE_OWNER":    "ENGRAM_MIGRATE_OWNER",
		"MEM_QDRANT_TEST_ADDR": "ENGRAM_QDRANT_TEST_ADDR",
	}
	for _, f := range registry {
		if f.Legacy != "" {
			m[f.Legacy] = f.Env
		}
	}
	return m
}()

// CheckLegacy returns an error naming every retired MEM_* variable present in
// environ (the os.Environ() slice form, "KEY=VALUE"), mapped to its ENGRAM_*
// replacement. Returns nil when none are set. Called from root PersistentPreRunE
// so a half-migrated deployment fails fast instead of silently falling back to a
// default.
func CheckLegacy(environ []string) error {
	var hits []string
	for _, kv := range environ {
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if repl, ok := legacyMap[name]; ok {
			hits = append(hits, fmt.Sprintf("  %s → %s", name, repl))
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Strings(hits)
	return fmt.Errorf("retired environment variables are set and no longer read:\n%s\nRename them (see the v0.x migration notes) and retry",
		strings.Join(hits, "\n"))
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/config/ -run TestCheckLegacy -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(config): fatal legacy-env guard for retired MEM_* vars

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: Migrate the embedder + store (server) to config

**Files:**

- Modify: `internal/server/tools.go:31-118`
- Test: `internal/server/tools_test.go`

- [ ] **Step 1: Update the failing test first (rename env + assert new behavior)**

In `internal/server/tools_test.go`, apply the rename mapping to every `t.Setenv`/`os.Setenv` and assertion:

- `MEM_QDRANT_TEST_ADDR` → `ENGRAM_QDRANT_TEST_ADDR`
- `MEM_QDRANT_ADDR` → `ENGRAM_QDRANT_ADDR`
- `MEM_EMBED_DIM` → `ENGRAM_EMBED_DIM`

Run: `rg -n "MEM_" internal/server/tools_test.go` and replace each occurrence per the mapping table. If the test sets `MEM_QDRANT_ADDR` to drive `StoreFromEnvNoEnsure`, it now sets `ENGRAM_QDRANT_ADDR`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/server/ -run TestStoreFromEnv -v` (or the relevant test names found in step 1)
Expected: FAIL — the test sets `ENGRAM_*` but `tools.go` still reads `MEM_*`, so defaults are used and assertions break.

- [ ] **Step 3: Rewrite the config-reading funcs in `tools.go`**

Replace `EnvOr` (lines 31-37), `StoreFromEnvNoEnsure` (62-94), and `EmbedderFromEnv` (108-118) so they consume `config.Load(nil)`. Add the import `"github.com/seanb4t/engram/internal/config"`. Delete `EnvOr` entirely and the now-unused `os`/`strconv` imports if no longer referenced (keep `strconv` — still used for dim/port parsing).

```go
// StoreFromEnvNoEnsure builds the Store from the ENGRAM_QDRANT_* / ENGRAM_EMBED_DIM
// environment but does NOT create the collection, and returns the configured embed
// dimension. reindex uses this so it can require the source collection to already
// exist rather than silently creating an empty one at the new dimension.
func StoreFromEnvNoEnsure() (*store.Store, uint64, error) {
	cfg, err := config.Load(nil)
	if err != nil {
		return nil, 0, err
	}
	embedDim, err := strconv.ParseUint(cfg.Embed.Dim, 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ENGRAM_EMBED_DIM %q: %w", cfg.Embed.Dim, err)
	}
	host, portStr, err := net.SplitHostPort(cfg.Qdrant.Addr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ENGRAM_QDRANT_ADDR %q: %w", cfg.Qdrant.Addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid port in ENGRAM_QDRANT_ADDR %q: %w", cfg.Qdrant.Addr, err)
	}
	qc, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
		GrpcOptions: []grpc.DialOption{
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		},
	})
	if err != nil {
		return nil, 0, fmt.Errorf("qdrant client: %w", err)
	}
	return store.New(qc, cfg.Qdrant.Collection), embedDim, nil
}

// EmbedderFromEnv builds the OpenAI-compatible embedder from the
// ENGRAM_OPENAI_BASE_URL, ENGRAM_OPENAI_API_KEY, and ENGRAM_EMBED_MODEL
// environment. Exported so admin commands (e.g. reindex) re-embed with the same
// configured embedder the server bootstrap uses.
func EmbedderFromEnv() *embed.Client {
	cfg, err := config.Load(nil)
	if err != nil {
		// Defaults always load cleanly; a Load error here means a malformed
		// koanf layer, which is a programming error, not operator input.
		panic(fmt.Sprintf("config load: %v", err))
	}
	return embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Embed.Model,
		embed.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)))
}
```

> Note: `EmbedderFromEnv` returns `*embed.Client` (no error) to preserve its
> signature and its caller in `reindex.go`. `config.Load(nil)` only errors on a
> malformed koanf layer (impossible with the static registry), so a panic is the
> correct fail-stop. If the reviewer prefers an error return, that is a
> follow-up; the signature is preserved here to keep this task contained.

Delete the `EnvOr` function. Update the `StoreFromEnv` doc comment's `MEM_QDRANT_* / MEM_EMBED_DIM` reference to `ENGRAM_QDRANT_* / ENGRAM_EMBED_DIM`. Update the `buildDepsFromEnv` doc comment likewise.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/server/ -v`
Expected: PASS. Then `go build ./...` — Expected: SUCCESS (reindex.go/migrate.go still call `server.EnvOr` for their *own* flag defaults; those are migrated in Task 7. `EnvOr` is removed here, so `go build` will FAIL until Task 7).

> **Sequencing note:** Removing `EnvOr` breaks `serve.go`, `reindex.go`, `migrate.go` which still reference it. To keep every commit building, **do Step 3 of Tasks 6 and 7 in the same working session before building/committing**, OR temporarily keep `EnvOr` and delete it in Task 7. Chosen approach: **keep `EnvOr` for now** (do not delete in this task); only swap the *bodies* of `StoreFromEnvNoEnsure`/`EmbedderFromEnv`. Delete `EnvOr` in Task 7 once its last caller is gone.

Revised Step 3 ending: **do not delete `EnvOr`**; leave it in place. Re-run:

Run: `go build ./... && go test ./internal/server/ -v`
Expected: SUCCESS + PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "refactor(server): read embedder/store config via internal/config; rename to ENGRAM_OPENAI_*/ENGRAM_QDRANT_*

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Migrate telemetry log vars to config

**Files:**

- Modify: `internal/telemetry/config.go:25-43`
- Test: `internal/telemetry/config_test.go`, `internal/telemetry/providers_test.go`

- [ ] **Step 1: Rename env in the tests**

In `internal/telemetry/config_test.go` and `providers_test.go`, replace `MEM_LOG_LEVEL` → `ENGRAM_LOG_LEVEL`, `MEM_LOG_FORMAT` → `ENGRAM_LOG_FORMAT`, `MEM_LOG_STDOUT` → `ENGRAM_LOG_STDOUT` in every `t.Setenv`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/telemetry/ -v`
Expected: FAIL — tests set `ENGRAM_LOG_*` but `config.go` reads `MEM_LOG_*`.

- [ ] **Step 3: Rewrite `ConfigFromEnv`**

In `internal/telemetry/config.go`, delete the local `envOr` and read log fields from `config.Load(nil)`. Keep `OTEL_EXPORTER_OTLP_ENDPOINT` as a native read (it is not an `ENGRAM_` var).

```go
import (
	"os"

	"github.com/seanb4t/engram/internal/config"
)

// ConfigFromEnv builds a Config from the environment. serviceName/serviceVersion
// are passed in (the version is ldflags-injected into main, not an env var). Log
// fields come from internal/config (ENGRAM_LOG_*); the OTLP endpoint is read
// natively (OTEL_* vars are consumed directly by the exporters).
func ConfigFromEnv(serviceName, serviceVersion string) Config {
	c, err := config.Load(nil)
	if err != nil {
		panic("telemetry config load: " + err.Error()) // static registry: cannot fail
	}
	return Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		LogLevel:       c.Log.Level,
		LogFormat:      c.Log.Format,
		LogStdout:      c.Log.Stdout != "false",
	}
}
```

Update the package-doc / `Config` comment that says "OTEL_* vars are also read natively" to also mention `ENGRAM_LOG_*` via `internal/config`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/telemetry/ -v && go build ./...`
Expected: PASS + SUCCESS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "refactor(telemetry): read ENGRAM_LOG_* via internal/config

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: Refactor `resolveUIConfig` to a struct input + ENGRAM_ names

**Files:**

- Modify: `cmd/engram/uiconfig.go`
- Test: `cmd/engram/uiconfig_test.go`

- [ ] **Step 1: Rewrite the test against the new signature**

The new `resolveUIConfig` takes a `uiRaw` struct (built from `*config.Config` by the caller) instead of a `getenv` closure. Rewrite `cmd/engram/uiconfig_test.go` cases to construct `uiRaw` directly. Keep all four behaviors: default issuer fallback, `ui.issuer` override, `ui.issuer` alone, no-issuer error, plus the enabled/disabled/`"false"`/`"true"`-missing-creds cases. Example replacements:

```go
func TestResolveUIConfigDefaultsIssuerToOIDC(t *testing.T) {
	got, err := resolveUIConfig(uiRaw{
		Enabled: "true", OIDCIssuer: "https://oidc", ClientID: "c", ClientSecret: "s",
		RedirectURL: "https://cb", CookieKey: "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatalf("resolveUIConfig: %v", err)
	}
	if !got.Enabled || got.Issuer != "https://oidc" {
		t.Errorf("got %+v, want enabled with issuer defaulted to OIDC issuer", got)
	}
}

func TestResolveUIConfigUIIssuerOverrides(t *testing.T) {
	got, err := resolveUIConfig(uiRaw{
		UIIssuer: "https://ui", OIDCIssuer: "https://oidc", ClientID: "c", ClientSecret: "s",
		RedirectURL: "https://cb", CookieKey: "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatalf("resolveUIConfig: %v", err)
	}
	if got.Issuer != "https://ui" {
		t.Errorf("Issuer = %q, want ui issuer to win", got.Issuer)
	}
}

func TestResolveUIConfigEnabledTrueMissingCreds(t *testing.T) {
	_, err := resolveUIConfig(uiRaw{Enabled: "true"})
	if err == nil {
		t.Fatal("want error for ENGRAM_UI_ENABLED=true with missing creds")
	}
}

func TestResolveUIConfigNoIssuerError(t *testing.T) {
	_, err := resolveUIConfig(uiRaw{
		ClientID: "c", ClientSecret: "s", RedirectURL: "https://cb",
		CookieKey: "0123456789abcdef0123456789abcdef",
	})
	if err == nil {
		t.Fatal("want error: enabled-by-creds but no issuer")
	}
}

func TestResolveUIConfigFalseIsHardOff(t *testing.T) {
	got, err := resolveUIConfig(uiRaw{
		Enabled: "false", OIDCIssuer: "https://oidc", ClientID: "c", ClientSecret: "s",
		RedirectURL: "https://cb", CookieKey: "0123456789abcdef0123456789abcdef",
	})
	if err != nil || got.Enabled {
		t.Errorf("got %+v err %v, want disabled", got, err)
	}
}
```

(Apply the cookie-key decode test rename `MEM_UI_COOKIE_KEY` → `ENGRAM_UI_COOKIE_KEY` in any error-message assertion.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/engram/ -run TestResolveUIConfig -v`
Expected: FAIL — `uiRaw` undefined and signature mismatch.

- [ ] **Step 3: Rewrite `resolveUIConfig`**

In `cmd/engram/uiconfig.go`, replace the getenv-closure signature with a `uiRaw` value. `requiredUICreds` becomes a presence check over struct fields; missing-creds reporting uses the `ENGRAM_*` names.

```go
// uiRaw is the unresolved web-UI input, built from *config.Config by the caller.
// ClientID/ClientSecret come from the OIDC family (ENGRAM_OIDC_CLIENT_ID/SECRET);
// OIDCIssuer is the MCP-lane issuer used as the UI-issuer fallback.
type uiRaw struct {
	Enabled      string // ENGRAM_UI_ENABLED
	UIIssuer     string // ENGRAM_UI_ISSUER
	OIDCIssuer   string // ENGRAM_OIDC_ISSUER
	ClientID     string // ENGRAM_OIDC_CLIENT_ID
	ClientSecret string // ENGRAM_OIDC_CLIENT_SECRET
	RedirectURL  string // ENGRAM_UI_REDIRECT_URL
	CookieKey    string // ENGRAM_UI_COOKIE_KEY
}

// resolveUIConfig implements the spec's activation tiebreaker (unchanged logic):
//   - Enabled=="false" (any case) is a hard off-switch.
//   - Otherwise the UI is on iff all four required creds are present.
//   - Enabled=="true" with missing creds is a fail-fast startup error.
//   - UIIssuer falls back to OIDCIssuer; enabled with neither is an error.
func resolveUIConfig(r uiRaw) (UIConfig, error) {
	if strings.EqualFold(r.Enabled, "false") {
		return UIConfig{Enabled: false}, nil
	}
	type cred struct {
		env, val string
	}
	creds := []cred{
		{"ENGRAM_OIDC_CLIENT_ID", r.ClientID},
		{"ENGRAM_OIDC_CLIENT_SECRET", r.ClientSecret},
		{"ENGRAM_UI_REDIRECT_URL", r.RedirectURL},
		{"ENGRAM_UI_COOKIE_KEY", r.CookieKey},
	}
	var missing []string
	for _, c := range creds {
		if c.val == "" {
			missing = append(missing, c.env)
		}
	}
	allCreds := len(missing) == 0

	if strings.EqualFold(r.Enabled, "true") && !allCreds {
		return UIConfig{}, fmt.Errorf("ENGRAM_UI_ENABLED=true but missing required creds: %v", missing)
	}
	if !allCreds {
		return UIConfig{Enabled: false}, nil
	}
	issuer := r.UIIssuer
	if issuer == "" {
		issuer = r.OIDCIssuer
	}
	if issuer == "" {
		return UIConfig{}, fmt.Errorf("web UI enabled but no OIDC issuer: set ENGRAM_UI_ISSUER or ENGRAM_OIDC_ISSUER")
	}
	return UIConfig{
		Enabled:      true,
		Issuer:       issuer,
		ClientID:     r.ClientID,
		ClientSecret: r.ClientSecret,
		RedirectURL:  r.RedirectURL,
		CookieKey:    r.CookieKey,
	}, nil
}
```

Delete the now-unused `requiredUICreds` var. Update `decodeCookieKey`'s error string `MEM_UI_COOKIE_KEY` → `ENGRAM_UI_COOKIE_KEY`.

- [ ] **Step 4: Bridge the serve.go call site (keep the build green)**

`serve.go` still has the package vars (`uiEnabled`, `uiIssuer`, `oidcIssuer`, `uiClientID`, `uiClientSecret`, `uiRedirectURL`, `uiCookieKey`) at this point — they are removed in Task 6. Replace the `resolveUIConfig(func(k string) string {...})` closure (serve.go lines 107-126) with a `uiRaw` built from those still-existing vars, so this commit builds. Task 6 rewrites this again to read from `cfg`.

```go
uiCfg, err := resolveUIConfig(uiRaw{
	Enabled:      uiEnabled,
	UIIssuer:     uiIssuer,
	OIDCIssuer:   oidcIssuer,
	ClientID:     uiClientID,
	ClientSecret: uiClientSecret,
	RedirectURL:  uiRedirectURL,
	CookieKey:    uiCookieKey,
})
```

- [ ] **Step 5: Run to verify build + tests pass**

Run: `go build ./... && go test ./cmd/engram/ -run 'TestResolveUIConfig|TestDecodeCookieKey' -v`
Expected: build SUCCESS; tests PASS.

- [ ] **Step 6: Commit**

```bash
jj commit -m "refactor(serve): resolveUIConfig takes a resolved struct; ENGRAM_ UI/OIDC names

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Migrate `serve.go` to config

**Files:**

- Modify: `cmd/engram/serve.go`

- [ ] **Step 1: Replace flag registration (`init`)**

Remove the package-var block (lines 31-45) and register flags without `EnvOr` defaults, using `config.FlagDefault` for accurate `--help`. Add import `"github.com/seanb4t/engram/internal/config"`.

```go
func init() {
	f := serveCmd.Flags()
	f.String("listen-addr", config.FlagDefault("listen-addr"), "listen address")
	f.String("oidc-issuer", config.FlagDefault("oidc-issuer"),
		"OIDC issuer URL; setting it enables bearer-token enforcement")
	f.String("oidc-audience", config.FlagDefault("oidc-audience"), "expected OIDC audience (optional)")
	f.String("oidc-resource-metadata", config.FlagDefault("oidc-resource-metadata"),
		"WWW-Authenticate resource metadata URL (optional)")
	f.String("ui-enabled", config.FlagDefault("ui-enabled"),
		"enable the web UI + login lane (empty=imply from creds; 'false'=hard off)")
	f.String("ui-issuer", config.FlagDefault("ui-issuer"),
		"OIDC issuer for the web-UI login lane (empty=default to --oidc-issuer)")
	f.String("oidc-client-id", config.FlagDefault("oidc-client-id"), "OIDC confidential-client ID for the web login")
	f.String("oidc-client-secret", config.FlagDefault("oidc-client-secret"), "OIDC client secret for the web login")
	f.String("ui-redirect-url", config.FlagDefault("ui-redirect-url"), "OIDC auth-code callback URL")
	f.String("ui-cookie-key", config.FlagDefault("ui-cookie-key"), "32-byte AES-GCM key sealing the session cookie")
	f.String("mcp-path", config.FlagDefault("mcp-path"),
		"path for the MCP transport (empty=/mcp; '/'=legacy root catch-all)")
}
```

Update the `init` doc comment: "Flag defaults come from the ENGRAM_* env vars (set by the Helm chart) via internal/config, so env drives config and explicitly-set flags override — cobra+koanf, no viper."

- [ ] **Step 2: Thread config through `runServe`**

`runServe` now takes the cobra command so it can load flags. Change `serveCmd.RunE` to `RunE: func(cmd *cobra.Command, _ []string) error { return runServe(cmd) }` and the signature to `func runServe(cmd *cobra.Command) error`. At the top of `runServe`:

```go
cfg, err := config.Load(cmd.Flags())
if err != nil {
	return fmt.Errorf("load config: %w", err)
}
```

Replace the `uiRaw{...}`-from-package-vars call introduced in Task 5 with one built from `cfg`:

```go
uiCfg, err := resolveUIConfig(uiRaw{
	Enabled:      cfg.UI.Enabled,
	UIIssuer:     cfg.UI.Issuer,
	OIDCIssuer:   cfg.OIDC.Issuer,
	ClientID:     cfg.OIDC.ClientID,
	ClientSecret: cfg.OIDC.ClientSecret,
	RedirectURL:  cfg.UI.RedirectURL,
	CookieKey:    cfg.UI.CookieKey,
})
if err != nil {
	slog.Error("web UI config invalid", "err", err)
	return err
}
```

Replace remaining package-var references:
- `resolveMCPPath(mcpPath)` → `resolveMCPPath(cfg.Server.MCPPath)`
- `withAuth(handler)` → `withAuth(handler, cfg.OIDC)` (see Step 3)
- `httpSrv.Addr: listenAddr` → `cfg.Server.ListenAddr`
- the `slog.Info("engram listening", ..., "addr", listenAddr)` → `cfg.Server.ListenAddr`

- [ ] **Step 3: Update `withAuth` to take OIDC config**

```go
// withAuth wraps the MCP handler with OIDC bearer-token validation when an issuer
// is configured. The upstream embedding/auth gateway forwards the caller's token
// untouched; engram verifies it and exposes the caller identity to tool handlers.
// No issuer → validation disabled (all requests accepted), logged loudly.
func withAuth(handler http.Handler, oidc config.OIDCConfig) (http.Handler, error) {
	if oidc.Issuer == "" {
		slog.Warn("OIDC validation DISABLED (no --oidc-issuer / ENGRAM_OIDC_ISSUER); all requests accepted")
		return handler, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	verifier, err := auth.New(ctx, oidc.Issuer, oidc.Audience)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("oidc verifier init: %w", err)
	}
	slog.Info("OIDC bearer-token validation enabled", "issuer", oidc.Issuer)
	return mcpauth.RequireBearerToken(verifier.TokenVerifier(), &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: oidc.ResourceMetadata,
	})(handler), nil
}
```

Update the `slog.Error("oidc verifier init failed", ..., "issuer", oidcIssuer)` line in `runServe` → `cfg.OIDC.Issuer`. De-LiteLLM the `withAuth` comment (done above).

- [ ] **Step 4: Run to verify build + tests pass**

Run: `go build ./... && go test ./cmd/engram/ -v`
Expected: build SUCCESS (serve.go no longer references removed vars; `EnvOr` still exists, called only by reindex.go/migrate.go now). Tests PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "refactor(serve): load config via internal/config; flags override ENGRAM_ env

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 7: Migrate command-local flags + retire `EnvOr`

**Files:**

- Modify: `cmd/engram/reindex.go:51,83-85`, `cmd/engram/migrate.go:59-61`, `internal/server/tools.go` (delete `EnvOr`)

- [ ] **Step 1: reindex.go — rename flag default + comments**

In `reindex.go` `init`, replace:

```go
reindexCmd.Flags().StringVar(&reindexTarget, "target",
	os.Getenv("ENGRAM_REINDEX_TARGET"),
	"target collection to create and populate (required)")
```

Add `"os"` to imports if not present (it is — used for signal). Update the command doc comment block (lines 26-35): `MEM_EMBED_DIM` → `ENGRAM_EMBED_DIM`, `MEM_QDRANT_COLLECTION` → `ENGRAM_QDRANT_COLLECTION`, `MEM_EMBED_MODEL` → `ENGRAM_EMBED_MODEL`, `MEM_LITELLM_URL` → `ENGRAM_OPENAI_BASE_URL`. Update the final `cmd.Printf` success string `set MEM_QDRANT_COLLECTION=%s` → `set ENGRAM_QDRANT_COLLECTION=%s`.

- [ ] **Step 2: migrate.go — rename flag default + error string**

```go
migrateSetOwnerCmd.Flags().StringVar(&migrateOwner, "owner",
	os.Getenv("ENGRAM_MIGRATE_OWNER"),
	"OIDC sub to stamp onto owner-less records (required, non-empty)")
```

Update the `--owner (or MEM_MIGRATE_OWNER) is required` error → `--owner (or ENGRAM_MIGRATE_OWNER) is required`. Add `"os"` to imports if missing (it is present).

- [ ] **Step 3: Delete `EnvOr` from `tools.go`**

Remove the `EnvOr` function (originally lines 31-37) and drop the `os` import from `tools.go` if it is now unused (check with `rg "\bos\." internal/server/tools.go`).

- [ ] **Step 4: Run to verify build + full test**

Run: `rg -n "EnvOr|server\.EnvOr" --type go` — Expected: no matches.
Run: `go build ./... && go test ./... 2>&1 | tail -20`
Expected: build SUCCESS; all Go tests PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "refactor(cmd): ENGRAM_ command-local env vars; retire server.EnvOr

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 8: Wire the legacy guard into root

**Files:**

- Modify: `cmd/engram/root.go`
- Test: `cmd/engram/root_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `cmd/engram/root_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"
)

func TestRootRejectsLegacyEnv(t *testing.T) {
	t.Setenv("MEM_LITELLM_URL", "http://old:4000")
	rootCmd.SetArgs([]string{"version"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected PersistentPreRunE to reject MEM_LITELLM_URL")
	}
	if !strings.Contains(err.Error(), "ENGRAM_OPENAI_BASE_URL") {
		t.Errorf("error %q should map the retired var to its replacement", err.Error())
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/engram/ -run TestRootRejectsLegacyEnv -v`
Expected: FAIL — no guard yet, `version` succeeds.

- [ ] **Step 3: Add `PersistentPreRunE`**

In `cmd/engram/root.go`, add the guard to `rootCmd` (add imports `"os"` and `"github.com/seanb4t/engram/internal/config"`):

```go
var rootCmd = &cobra.Command{
	Use:           "engram",
	Short:         "Self-hosted, correctable, OAuth-secured memory for coding agents",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		return config.CheckLegacy(os.Environ())
	},
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./cmd/engram/ -run TestRootRejectsLegacyEnv -v`
Expected: PASS.

> **Test-isolation note:** other `cmd/engram` tests must not leak `MEM_*` into the
> environment (they were renamed to `ENGRAM_*` in Tasks 3-6, so this holds). Run
> the full package to confirm: `go test ./cmd/engram/ -v`.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(cmd)!: fail fast on retired MEM_* env vars at startup

BREAKING CHANGE: MEM_* environment variables are no longer read. Rename to
ENGRAM_* (and MEM_LITELLM_URL/KEY to ENGRAM_OPENAI_BASE_URL/API_KEY). The server
and admin commands now exit non-zero at startup if any retired MEM_* var is set.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 9: Helm chart rename

**Files:**

- Modify: `charts/engram/templates/memory-mcp.yaml`, `charts/engram/values.yaml`

- [ ] **Step 1: Read the current values structure**

Run: `rg -n "litellm|embed:|qdrant:|listenAddr|observability|ui:|oidc:" charts/engram/values.yaml`
Identify the `memory.litellm.url` key and the API-key secret block.

- [ ] **Step 2: Rename env var names in the template**

In `charts/engram/templates/memory-mcp.yaml`, apply the mapping table to every `name: MEM_*`. Specifically (per current line numbers):
- `MEM_LISTEN_ADDR` → `ENGRAM_LISTEN_ADDR`
- `MEM_QDRANT_ADDR` → `ENGRAM_QDRANT_ADDR`
- `MEM_QDRANT_COLLECTION` → `ENGRAM_QDRANT_COLLECTION`
- `MEM_LITELLM_URL` → `ENGRAM_OPENAI_BASE_URL` (and its value `{{ .Values.memory.litellm.url }}` → `{{ .Values.memory.openai.baseURL }}`)
- `MEM_EMBED_MODEL` → `ENGRAM_EMBED_MODEL`; `MEM_EMBED_DIM` → `ENGRAM_EMBED_DIM`
- `MEM_LITELLM_KEY` → `ENGRAM_OPENAI_API_KEY` (secret-backed; keep the `if .name` gate)
- `MEM_MCP_PATH`, `MEM_OIDC_*`, `MEM_UI_*`, `MEM_LOG_*` → `ENGRAM_*`
- Update the comment "server defaults the UI login issuer to MEM_OIDC_ISSUER" → `ENGRAM_OIDC_ISSUER`.

- [ ] **Step 3: Rename the values.yaml key**

In `charts/engram/values.yaml`, rename `memory.litellm: { url: ... }` → `memory.openai: { baseURL: ... }`, and rename the API-key secret block accordingly (e.g. `memory.litellm.keySecret` → `memory.openai.apiKeySecret`) to match the template's `valueFrom.secretKeyRef`. Keep all other `memory.*` / `observability.*` value keys unchanged (only env var NAMES changed for those).

- [ ] **Step 4: Verify the chart lints and renders**

Run:

```bash
helm lint charts/engram
helm template charts/engram | rg -n "ENGRAM_|MEM_"
```

Expected: `helm lint` passes; `helm template` shows only `ENGRAM_*` env names and **zero** `MEM_*`.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(chart)!: ENGRAM_ env vars; memory.openai.baseURL replaces memory.litellm.url

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 10: docs-site rename + de-LiteLLM wording

**Files:**

- Modify: `docs-site/src/content/docs/guides/configure.md`, `guides/quickstart.md`, `guides/deploy.md`, `reference/auth.md`, `contributing/architecture.md`

- [ ] **Step 1: Apply the rename mapping to all five docs files**

For each file, replace every `MEM_*` per the mapping table. In `configure.md` the Embedder table rows for `MEM_LITELLM_URL`/`MEM_LITELLM_KEY` become `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`.

- [ ] **Step 2: De-LiteLLM the Embedder table description**

In `configure.md`, change the description cell "OpenAI-compatible embeddings endpoint (LiteLLM or OpenAI)" to "OpenAI-compatible embeddings endpoint — point it at any backend that speaks the OpenAI `/v1/embeddings` API (e.g. Ollama, vLLM, TEI, LiteLLM, OpenAI)". Update the "Source: internal/server/tools.go (buildDepsFromEnv)" caption if the function reference changed (it did not rename, but it now delegates to `EmbedderFromEnv` → `internal/config`; update the caption to "internal/config (registry) + internal/server/tools.go (EmbedderFromEnv)").

- [ ] **Step 3: Verify no residual MEM_ / LiteLLM in docs-site**

Run: `rg -n "MEM_|LiteLLM|litellm" docs-site/src/content/docs/`
Expected: no matches (LiteLLM may remain only inside the neutral examples list in configure.md — confirm it reads as one example among several, not as the named requirement).

- [ ] **Step 4: Commit**

```bash
jj commit -m "docs(site): ENGRAM_ env vars; de-LiteLLM the embedder reference

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 11: skill, python test, code comments, CLAUDE.md

**Files:**

- Modify: `skill/engram/commands/engram-setup.md`, `skill/engram/hooks/tests/test_no_residual_memory_oauth.py`, `internal/embed/embed.go:4-5`, `internal/auth/auth.go:7`, `CLAUDE.md`

- [ ] **Step 1: Rename in the skill + python test**

Apply the mapping table to `skill/engram/commands/engram-setup.md` and the `MEM_*` reference in `skill/engram/hooks/tests/test_no_residual_memory_oauth.py`.

- [ ] **Step 2: De-LiteLLM code comments**

- `internal/embed/embed.go` line 4-5: change "an OpenAI-compatible /v1/embeddings endpoint (e.g. LiteLLM fronting Ollama bge-m3)" → "an OpenAI-compatible /v1/embeddings endpoint (e.g. Ollama, vLLM, or a LiteLLM gateway)".
- `internal/auth/auth.go` line 7: change "IdP + LiteLLM gateway" → "IdP + OIDC-aware embedding gateway" (or simply "IdP").

- [ ] **Step 3: Update CLAUDE.md**

In `/Volumes/Code/github.com/seanb4t/engram/CLAUDE.md`:
- The Conventions bullet "CLI: cobra; no viper — config is env-first (`MEM_*`) with flag overrides." → "CLI: cobra; config is loaded by `internal/config` (koanf): env-first via the `ENGRAM_` prefix with `--flag` overrides; no viper."
- Replace `MEM_*` mentions in the Memory-contract and Auth sections (`MEM_OIDC_ISSUER` → `ENGRAM_OIDC_ISSUER`) and the layout table if it cites env vars.
- Add `internal/config/` to the layout table: "koanf config loader + field registry (single source of truth for ENGRAM_ vars)".

- [ ] **Step 4: Verify residual sweep (whole repo, excluding history)**

Run:

```bash
rg -n "MEM_[A-Z_]+|MEM_LITELLM|\blitellm\b|LiteLLM" \
  -g '!docs/superpowers/specs/**' -g '!docs/superpowers/plans/**' \
  -g '!docs/adr/**' -g '!CHANGELOG.md' --hidden -g '!.git' -g '!.jj'
```

Expected: matches only where intended — the neutral "LiteLLM" example in `configure.md` and `embed.go`, and any `internal/config/{registry,legacy}.go` lines that intentionally carry the `MEM_*` legacy mapping. **Zero** `MEM_*` reads anywhere else.

- [ ] **Step 5: Commit**

```bash
jj commit -m "docs(skill,comments): ENGRAM_ vars; koanf config note in CLAUDE.md; de-LiteLLM comments

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 12: Full verification gate

**Files:** none (verification only)

- [ ] **Step 1: Run the project quality gate**

Run: `task` (= lint + test)
Expected: golangci-lint clean, all Go tests PASS.

- [ ] **Step 2: License headers**

Run: `task license:check`
Expected: 0 invalid (new `internal/config/*.go` files carry the SPDX header).

- [ ] **Step 3: Format**

Run: `task fmt && jj diff --git --stat`
Expected: no unexpected reformatting; if `task fmt` changed files, review and fold into the relevant commit.

- [ ] **Step 4: Helm + actionlint (CI parity)**

Run: `helm lint charts/engram`
Expected: pass.

- [ ] **Step 5: Final residual assertion**

Run:

```bash
rg -c "MEM_" --hidden -g '!.git' -g '!.jj' -g '!docs/superpowers/**' -g '!docs/adr/**' -g '!CHANGELOG.md' \
  | rg -v "internal/config/(registry|legacy).go|internal/config/legacy_test.go|cmd/engram/root_test.go"
```

Expected: no output (the only files mentioning `MEM_` are the intentional legacy-guard map + its tests).

- [ ] **Step 6: Smoke-test the guard end to end**

Run:

```bash
go build -o /tmp/engram ./cmd/engram
MEM_LITELLM_URL=http://x /tmp/engram version; echo "exit: $?"
ENGRAM_OPENAI_BASE_URL=http://x /tmp/engram version; echo "exit: $?"
```

Expected: first invocation prints the `MEM_LITELLM_URL → ENGRAM_OPENAI_BASE_URL` mapping error and `exit: 1`; second prints the version and `exit: 0`.

- [ ] **Step 7: Commit any formatting/lint fixups (if Step 3 produced changes)**

```bash
jj commit -m "chore: gofmt/lint fixups for ENGRAM_ config migration

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Notes for the PR

- This is **one PR**, titled `feat!: rename MEM_* config to ENGRAM_* and de-LiteLLM the embedder`. The `BREAKING CHANGE:` footer (already on the Task 8 commit) must carry the full mapping table so release-please surfaces it in the CHANGELOG.
- The seven CI required checks (test, golangci-lint, commit-lint, license headers, helm chart, actionlint, python) must all pass — Task 12 covers their local equivalents.
- After merge, the operator's own homelab deploy must update its env (or the guard will halt startup with the exact renames needed).
<!-- adr-capture: sha256=ac9e12746265c955; session=cli; ts=2026-06-14T11:21:57Z; adrs=engram-jgq,engram-378,engram-irq -->
