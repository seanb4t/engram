<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Centralized config validation (`Config.Validate`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a single pure `Config.Validate()` that asserts engram's data-plane config is well-formed, run uniformly at startup with aggregated errors, so malformed config fails loudly and early instead of late, opaquely, or never.

**Architecture:** A new `Validate` method on `internal/config.Config` checks the Qdrant + embedder fields with `errors.Join` aggregation. It is wired into `server.StoreFromEnvNoEnsure` — the single choke point every store-building command (`serve` via `buildDepsFromEnv`, `reindex`, `migrate`, `prune`) already funnels through — so one call site covers all entrypoints without adding another `config.Load`. `serve` keeps a separate one-line `listen_addr` guard. `config.Load` stays pure assembly (no validation inside it), preserving the invariant that justifies `EmbedderFromEnv`'s panic-on-`Load`-error.

**Tech Stack:** Go, `errors.Join`, `net.SplitHostPort`, `net/url`, `strconv`. Spec: `docs/superpowers/specs/2026-06-14-config-validation-design.md`.

**Design bead:** engram-syt

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/config/validate.go` (create) | `Config.Validate()` — pure data-plane well-formedness, aggregated errors |
| `internal/config/validate_test.go` (create) | Table-driven unit tests for every rule + aggregation + happy path |
| `internal/server/tools.go` (modify) | Call `cfg.Validate()` in `StoreFromEnvNoEnsure` after `config.Load(nil)` |
| `internal/server/tools_test.go` (modify) | Wiring test: malformed env → `StoreFromEnvNoEnsure` returns the validation error before any Qdrant call |
| `cmd/engram/serve.go` (modify) | Serve-local `listen_addr` non-empty guard in `runServe` |

---

### Task 1: `Config.Validate()`

**Files:**

- Create: `internal/config/validate.go`
- Test: `internal/config/validate_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/validate_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"
)

// validConfig returns a Config whose data-plane fields all pass Validate.
// Tests mutate one field to exercise a single rule.
func validConfig() *Config {
	return &Config{
		Qdrant: QdrantConfig{Addr: "localhost:6334", Collection: "mem_eval"},
		Embed:  EmbedConfig{Model: "ollama/bge-m3", Dim: "1024"},
		OpenAI: OpenAIConfig{BaseURL: "http://localhost:4000"},
	}
}

func TestValidateHappyPath(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate(valid) = %v, want nil", err)
	}
}

func TestValidateFieldRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string // substring expected in the error
	}{
		{"qdrant addr empty", func(c *Config) { c.Qdrant.Addr = "" }, "ENGRAM_QDRANT_ADDR"},
		{"qdrant addr no port", func(c *Config) { c.Qdrant.Addr = "localhost" }, "ENGRAM_QDRANT_ADDR"},
		{"qdrant addr non-numeric port", func(c *Config) { c.Qdrant.Addr = "localhost:nope" }, "ENGRAM_QDRANT_ADDR"},
		{"qdrant addr port out of range", func(c *Config) { c.Qdrant.Addr = "localhost:70000" }, "out of range"},
		{"qdrant collection empty", func(c *Config) { c.Qdrant.Collection = "" }, "ENGRAM_QDRANT_COLLECTION"},
		{"embed model empty", func(c *Config) { c.Embed.Model = "" }, "ENGRAM_EMBED_MODEL"},
		{"embed dim empty", func(c *Config) { c.Embed.Dim = "" }, "ENGRAM_EMBED_DIM"},
		{"embed dim non-numeric", func(c *Config) { c.Embed.Dim = "abc" }, "ENGRAM_EMBED_DIM"},
		{"embed dim zero", func(c *Config) { c.Embed.Dim = "0" }, "ENGRAM_EMBED_DIM"},
		{"openai base_url empty", func(c *Config) { c.OpenAI.BaseURL = "" }, "ENGRAM_OPENAI_BASE_URL"},
		{"openai base_url bad scheme", func(c *Config) { c.OpenAI.BaseURL = "ftp://x" }, "scheme must be http"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateAggregatesAllFailures(t *testing.T) {
	c := validConfig()
	c.Qdrant.Addr = ""
	c.OpenAI.BaseURL = "ftp://x"
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want aggregated error")
	}
	for _, want := range []string{"ENGRAM_QDRANT_ADDR", "ENGRAM_OPENAI_BASE_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregated error %q missing %q (should report ALL failures)", err, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestValidate -v`
Expected: FAIL — `c.Validate undefined (type *Config has no field or method Validate)` (compile error).

- [ ] **Step 3: Write minimal implementation**

Create `internal/config/validate.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// Validate reports whether c's data-plane fields are well-formed. It is pure
// (no I/O) and aggregates every failure via errors.Join, so a caller sees all
// problems at once. Each error names the ENGRAM_* env var, not the koanf key.
//
// Scope is the fields every command's store/embedder path consumes (Qdrant,
// embedder). Optional fields (ENGRAM_OPENAI_API_KEY), fields validated elsewhere
// (OIDC/UI creds via resolveUIConfig), and the serve-only listen address are
// intentionally NOT checked here. Validation lives outside config.Load on
// purpose: Load stays pure assembly so a Load error remains a programming error
// (a malformed koanf layer), never operator input.
func (c *Config) Validate() error {
	var errs []error

	switch host, portStr, err := net.SplitHostPort(c.Qdrant.Addr); {
	case c.Qdrant.Addr == "":
		errs = append(errs, errors.New("ENGRAM_QDRANT_ADDR is empty: must be host:port"))
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_QDRANT_ADDR %q: must be host:port: %w", c.Qdrant.Addr, err))
	default:
		_ = host
		port, perr := strconv.Atoi(portStr)
		switch {
		case perr != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_QDRANT_ADDR %q: port must be numeric: %w", c.Qdrant.Addr, perr))
		case port < 1 || port > 65535:
			errs = append(errs, fmt.Errorf("ENGRAM_QDRANT_ADDR %q: port %d out of range 1-65535", c.Qdrant.Addr, port))
		}
	}

	if c.Qdrant.Collection == "" {
		errs = append(errs, errors.New("ENGRAM_QDRANT_COLLECTION is empty"))
	}

	if c.Embed.Model == "" {
		errs = append(errs, errors.New("ENGRAM_EMBED_MODEL is empty"))
	}

	switch dim, err := strconv.ParseUint(c.Embed.Dim, 10, 64); {
	case c.Embed.Dim == "":
		errs = append(errs, errors.New("ENGRAM_EMBED_DIM is empty: must be a positive integer"))
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_DIM %q: must be a positive integer: %w", c.Embed.Dim, err))
	case dim == 0:
		errs = append(errs, errors.New("ENGRAM_EMBED_DIM must be greater than 0"))
	}

	switch u, err := url.Parse(c.OpenAI.BaseURL); {
	case c.OpenAI.BaseURL == "":
		errs = append(errs, errors.New("ENGRAM_OPENAI_BASE_URL is empty: must be an http(s) URL"))
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_BASE_URL %q: must be a valid URL: %w", c.OpenAI.BaseURL, err))
	case u.Scheme != "http" && u.Scheme != "https":
		errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_BASE_URL %q: scheme must be http or https", c.OpenAI.BaseURL))
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
}
```

> Note on the `switch` form: the init statement (`net.SplitHostPort`, `url.Parse`)
> always runs, but the empty-string `case` is listed first and matched first, so
> the parse result on an empty input is simply ignored — no nil-deref, and empty
> gets its own clear message ahead of the parse-error message.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestValidate -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Run the whole package + lint**

Run: `go test ./internal/config/ && golangci-lint run ./internal/config/...`
Expected: `ok` and `0 issues`.

- [ ] **Step 6: Commit**

```
jj commit -m "feat(config): add Config.Validate for data-plane well-formedness (engram-syt)"
```

---

### Task 2: Wire `Validate` into the store path (covers serve/reindex/migrate/prune)

**Files:**

- Modify: `internal/server/tools.go:58-62` (`StoreFromEnvNoEnsure`, right after `config.Load(nil)`)
- Test: `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/tools_test.go`:

```go
func TestStoreFromEnvNoEnsureValidatesConfig(t *testing.T) {
	// A malformed (non-empty) data-plane value reaches Config.Validate via
	// StoreFromEnvNoEnsure's config.Load(nil). Validation runs BEFORE any Qdrant
	// client construction, so this returns fast and needs no live Qdrant.
	t.Setenv("ENGRAM_OPENAI_BASE_URL", "ftp://nope")
	_, _, err := StoreFromEnvNoEnsure()
	if err == nil {
		t.Fatal("StoreFromEnvNoEnsure with bad ENGRAM_OPENAI_BASE_URL = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "invalid configuration") ||
		!strings.Contains(err.Error(), "ENGRAM_OPENAI_BASE_URL") {
		t.Errorf("error %q, want aggregated validation error naming ENGRAM_OPENAI_BASE_URL", err)
	}
}
```

If `strings` is not already imported in `tools_test.go`, add it to the import block.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestStoreFromEnvNoEnsureValidatesConfig -v`
Expected: FAIL — `err == nil` (today `ftp://nope` is accepted; the embedder only concatenates the URL, so nothing rejects it at startup). It may instead fail later trying to reach Qdrant; either way the assertion `want validation error` is unmet.

- [ ] **Step 3: Write minimal implementation**

In `internal/server/tools.go`, in `StoreFromEnvNoEnsure`, insert the `Validate` call immediately after the `config.Load(nil)` error check (currently line 62, before the `ParseUint`):

```go
func StoreFromEnvNoEnsure() (*store.Store, uint64, error) {
	cfg, err := config.Load(nil)
	if err != nil {
		return nil, 0, fmt.Errorf("load config: %w", err)
	}
	// Fail fast and uniformly on malformed data-plane config before building any
	// client. This single call site covers every store-building command: serve
	// (via buildDepsFromEnv), reindex, migrate, and prune.
	if err := cfg.Validate(); err != nil {
		return nil, 0, err
	}
	embedDim, err := strconv.ParseUint(cfg.Embed.Dim, 10, 64)
	// ... unchanged below ...
```

Leave the existing `ParseUint` / `SplitHostPort` / `Atoi` parses in place — they
parse-for-use (build the client's host/port and the embed dim). `Validate` just
fails earlier with the aggregated, uniform message.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestStoreFromEnvNoEnsureValidatesConfig -v`
Expected: PASS.

- [ ] **Step 5: Run the package + lint**

Run: `go test ./internal/server/ && golangci-lint run ./internal/server/...`
Expected: `ok` (integration tests needing Qdrant auto-skip) and `0 issues`.

- [ ] **Step 6: Commit**

```
jj commit -m "feat(server): validate config in StoreFromEnvNoEnsure startup path (engram-syt)"
```

---

### Task 3: Serve-local `listen_addr` guard

**Files:**

- Modify: `cmd/engram/serve.go:64-67` (`runServe`, right after `config.Load(cmd.Flags())`)

**TDD exception (config/startup glue):** `runServe` binds a real socket and is not
unit-testable without a refactor disproportionate to a one-line non-empty guard.
Per the spec (D4) this check is deliberately serve-local (admin commands bind no
listener). Verify manually instead of with a unit test — see Step 2.

- [ ] **Step 1: Add the guard**

In `cmd/engram/serve.go`, in `runServe`, immediately after the `config.Load` block:

```go
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	// Serve-local guard: an empty listen address makes http.Server bind ":http"
	// (port 80) silently. ENGRAM_LISTEN_ADDR defaults to :8080; only an explicit
	// --listen-addr "" can empty it. Data-plane fields are validated in the
	// store path (StoreFromEnvNoEnsure → Config.Validate).
	if cfg.Server.ListenAddr == "" {
		return fmt.Errorf("ENGRAM_LISTEN_ADDR (or --listen-addr) is empty: a listen address is required")
	}
```

- [ ] **Step 2: Verify manually**

Build and run with an explicitly-empty listen addr; confirm it fails fast and does
NOT bind a socket:

Run: `go build -o /tmp/engram ./cmd/engram && /tmp/engram serve --listen-addr ""`
Expected: exits non-zero printing `ENGRAM_LISTEN_ADDR (or --listen-addr) is empty: a listen address is required`; no "engram listening" log.

- [ ] **Step 3: Run build + lint**

Run: `go build ./... && golangci-lint run ./cmd/engram/...`
Expected: clean build, `0 issues`.

- [ ] **Step 4: Commit**

```
jj commit -m "feat(serve): fail fast on empty listen address (engram-syt)"
```

---

## Final verification

- [ ] **Full gates green** — Run: `go build ./... && go test ./... && golangci-lint run && task license:check`
  Expected: build clean; all tests pass (integration auto-skips without Qdrant); `0 issues`; license headers valid (the two new files carry the SPDX header).
