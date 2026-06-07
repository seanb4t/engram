<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Observability: structured logging + OpenTelemetry — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the engram MCP server structured logging plus OpenTelemetry metrics and traces, exported over OTLP, instrumented at the HTTP/auth, MCP tool-call, and downstream-client (embedder + Qdrant) seams.

**Architecture:** A new `internal/telemetry` package owns logger construction and OTel SDK bootstrap behind one `Setup(ctx, cfg) → (*slog.Logger, shutdown, error)` seam that no-ops cleanly when no OTLP endpoint is configured. Instrumentation is added at three layers without touching the 11 tool handlers individually: an HTTP middleware (access log + auth-failure metric) and `otelhttp`, a single go-sdk `AddReceivingMiddleware` for per-tool spans/metrics/logs, and transport/dial-option seams on the embedder `http.Client` and Qdrant gRPC client. All emit through the global `otel.*` providers, so they degrade to no-ops when telemetry is disabled.

**Tech Stack:** Go 1.26, `log/slog`, `go.opentelemetry.io/otel` (sdk + OTLP gRPC exporters for traces/metrics/logs), `contrib/bridges/otelslog`, `contrib/instrumentation/net/http/otelhttp`, `contrib/instrumentation/google.golang.org/grpc/otelgrpc`, `modelcontextprotocol/go-sdk`, `qdrant/go-client`.

**Design spec:** `docs/superpowers/specs/2026-06-07-observability-logging-telemetry-design.md`
**Design bead:** engram-ew7

---

## File Structure

| File | Responsibility | Phase |
|------|----------------|-------|
| `internal/telemetry/config.go` (create) | `Config` struct + `ConfigFromEnv` (reads `OTEL_EXPORTER_OTLP_ENDPOINT`, `MEM_LOG_*`) | 1 |
| `internal/telemetry/logger.go` (create) | `NewLogger(cfg, lp)` → fan-out slog handler (stdout + otelslog), silent-process guard | 1 |
| `internal/telemetry/telemetry.go` (create) | `Setup(ctx, cfg)` → providers (no-op until Phase 2) + logger + shutdown | 1→2 |
| `internal/telemetry/metrics.go` (create) | tool + auth metric instruments built from a `metric.Meter` | 3 |
| `cmd/engram/serve.go` (modify) | bootstrap telemetry, wrap handler (otelhttp + access log), `http.Server` + graceful shutdown, slog | 1→3 |
| `cmd/engram/root.go` (modify) | expose `version` to telemetry resource (already a package var) | 2 |
| `internal/server/tools.go` (modify) | `Register` returns error; `buildDepsFromEnv` returns error; slog; add tool middleware; otelgrpc dial option; otelhttp embedder transport | 1→3 |
| `internal/embed/embed.go` (modify) | add `Option`/`WithHTTPTransport` seam to inject a `RoundTripper` | 3 |
| `internal/auth/auth.go` (modify) | log token-rejection reason at the verify-failure point | 3 |
| `charts/engram/values.yaml` + `templates/deployment.yaml` (modify) | OTEL_* + MEM_LOG_* env wiring | 4 |
| `go.mod` / `go.sum` (modify) | `go get` exporter/log/bridge packages (Phase 2 task 1) | 2,4 |

---

## Phase 1 — slog foundation (stdout only, no telemetry yet)

### Task 1: Telemetry config from env

**Files:**
- Create: `internal/telemetry/config.go`
- Test: `internal/telemetry/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import "testing"

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	t.Setenv("MEM_LOG_LEVEL", "debug")
	t.Setenv("MEM_LOG_FORMAT", "text")
	t.Setenv("MEM_LOG_STDOUT", "false")

	c := ConfigFromEnv("engram", "1.2.3")

	if c.ServiceName != "engram" || c.ServiceVersion != "1.2.3" {
		t.Fatalf("service identity: got %q/%q", c.ServiceName, c.ServiceVersion)
	}
	if c.OTLPEndpoint != "otel-collector:4317" {
		t.Errorf("endpoint: got %q", c.OTLPEndpoint)
	}
	if c.LogLevel != "debug" || c.LogFormat != "text" || c.LogStdout {
		t.Errorf("log cfg: got level=%q format=%q stdout=%v", c.LogLevel, c.LogFormat, c.LogStdout)
	}
}

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("MEM_LOG_LEVEL", "")
	t.Setenv("MEM_LOG_FORMAT", "")
	t.Setenv("MEM_LOG_STDOUT", "")

	c := ConfigFromEnv("engram", "dev")

	if c.OTLPEndpoint != "" {
		t.Errorf("endpoint default should be empty, got %q", c.OTLPEndpoint)
	}
	if c.LogLevel != "info" || c.LogFormat != "json" || !c.LogStdout {
		t.Errorf("defaults: got level=%q format=%q stdout=%v", c.LogLevel, c.LogFormat, c.LogStdout)
	}
	if c.Enabled() {
		t.Error("Enabled() should be false with no endpoint")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telemetry/ -run TestConfigFromEnv -v`
Expected: FAIL — `undefined: ConfigFromEnv` / package does not compile.

- [ ] **Step 3: Write minimal implementation**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package telemetry owns engram's structured logger and OpenTelemetry SDK
// bootstrap. Telemetry export is enabled only when an OTLP endpoint is set;
// otherwise providers are no-ops and logs still go to stdout.
package telemetry

import "os"

// Config controls logging and OTLP export. It is env-first, matching engram's
// no-viper convention: OTEL_* vars are also read natively by the exporters.
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string // OTEL_EXPORTER_OTLP_ENDPOINT; empty disables export
	LogLevel       string // debug|info|warn|error
	LogFormat      string // json|text
	LogStdout      bool   // also write logs to stdout
}

// Enabled reports whether OTLP export should be wired up.
func (c Config) Enabled() bool { return c.OTLPEndpoint != "" }

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ConfigFromEnv builds a Config from the environment. serviceName/serviceVersion
// are passed in (the version is ldflags-injected into main, not an env var).
func ConfigFromEnv(serviceName, serviceVersion string) Config {
	return Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		LogLevel:       envOr("MEM_LOG_LEVEL", "info"),
		LogFormat:      envOr("MEM_LOG_FORMAT", "json"),
		LogStdout:      envOr("MEM_LOG_STDOUT", "true") != "false",
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telemetry/ -run TestConfigFromEnv -v`
Expected: PASS (both `TestConfigFromEnv` and `TestConfigFromEnvDefaults`).

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(telemetry): env-first observability Config (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Logger constructor with silent-process guard

**Files:**
- Create: `internal/telemetry/logger.go`
- Test: `internal/telemetry/logger_test.go`

> **PREREQUISITE (build-order):** `logger.go` imports `otelslog`/`otel/log`,
> which are not yet in the module graph. Run the **Phase 2 Task 1 `go get`
> block now** (it is safe to run early and is idempotent — Phase 2 Task 1 then
> just re-verifies with `go mod tidy`). Do this before Step 3 or the package
> will not compile. An automated executor running phases strictly in order MUST
> still satisfy this prerequisite before Task 2.

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewLoggerWritesJSONToStdout(t *testing.T) {
	var buf bytes.Buffer
	lg := newLoggerTo(&buf, Config{LogLevel: "info", LogFormat: "json", LogStdout: true}, nil)
	lg.Info("hello", "tool", "store_memory")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("expected JSON line, got %q (%v)", buf.String(), err)
	}
	if rec["msg"] != "hello" || rec["tool"] != "store_memory" {
		t.Errorf("missing fields: %v", rec)
	}
}

func TestNewLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	lg := newLoggerTo(&buf, Config{LogLevel: "warn", LogFormat: "json", LogStdout: true}, nil)
	lg.Info("suppressed")
	lg.Warn("shown")
	if strings.Contains(buf.String(), "suppressed") {
		t.Error("info should be filtered at warn level")
	}
	if !strings.Contains(buf.String(), "shown") {
		t.Error("warn should pass")
	}
}

func TestSilentProcessGuardForcesStdout(t *testing.T) {
	// stdout disabled AND no OTLP provider => must force stdout on, not go silent.
	var buf bytes.Buffer
	cfg := Config{LogLevel: "info", LogFormat: "json", LogStdout: false} // no endpoint => not enabled
	lg := newLoggerTo(&buf, cfg, nil)
	lg.Warn("must appear")
	if !strings.Contains(buf.String(), "must appear") {
		t.Error("guard must keep stdout when no log sink would otherwise exist")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telemetry/ -run TestNewLogger -v`
Expected: FAIL — `undefined: newLoggerTo`.

- [ ] **Step 3: Write minimal implementation**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otellog "go.opentelemetry.io/otel/log"
)

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger builds the application logger. When lp is non-nil, log records are
// also bridged to OpenTelemetry via otelslog. When stdout is disabled and there
// is no OTLP log provider, stdout is forced back on so the process is never
// left with no log sink (silent-process guard).
func NewLogger(cfg Config, lp otellog.LoggerProvider) *slog.Logger {
	return newLoggerTo(os.Stdout, cfg, lp)
}

func newLoggerTo(w io.Writer, cfg Config, lp otellog.LoggerProvider) *slog.Logger {
	level := parseLevel(cfg.LogLevel)
	stdout := cfg.LogStdout
	if !stdout && lp == nil {
		// No sink would remain — force stdout on. Caller logs the degradation.
		stdout = true
	}

	var handlers []slog.Handler
	if stdout {
		opts := &slog.HandlerOptions{Level: level}
		if cfg.LogFormat == "text" {
			handlers = append(handlers, slog.NewTextHandler(w, opts))
		} else {
			handlers = append(handlers, slog.NewJSONHandler(w, opts))
		}
	}
	if lp != nil {
		handlers = append(handlers, otelslog.NewHandler("github.com/seanb4t/engram",
			otelslog.WithLoggerProvider(lp)))
	}

	if len(handlers) == 1 {
		return slog.New(handlers[0])
	}
	return slog.New(fanout(handlers))
}
```

- [ ] **Step 4: Write the fan-out handler**

Add to the same file (`internal/telemetry/logger.go`):

```go
// fanout dispatches each record to every wrapped handler. Used to write both
// stdout and the OTLP bridge from one *slog.Logger.
type fanoutHandler struct {
	handlers []slog.Handler
}

func fanout(hs []slog.Handler) slog.Handler { return &fanoutHandler{handlers: hs} }

func (f *fanoutHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (f *fanoutHandler) WithAttrs(as []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(as)
	}
	return &fanoutHandler{handlers: next}
}

func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}
```

Add `"context"` to the import block of `logger.go`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/telemetry/ -run TestNewLogger -v && go test ./internal/telemetry/ -run TestSilentProcessGuard -v`
Expected: PASS (all three logger tests). Requires the prerequisite `go get` above.

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(telemetry): slog logger with fan-out + silent-process guard (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Setup seam (no-op providers in Phase 1)

**Files:**
- Create: `internal/telemetry/telemetry.go`
- Test: `internal/telemetry/telemetry_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"testing"
)

func TestSetupDisabledReturnsLoggerAndNoopShutdown(t *testing.T) {
	cfg := Config{ServiceName: "engram", ServiceVersion: "test",
		LogLevel: "info", LogFormat: "json", LogStdout: true} // no endpoint
	lg, shutdown, err := Setup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Setup returned error when disabled: %v", err)
	}
	if lg == nil {
		t.Fatal("Setup must always return a usable logger")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown should be nil, got %v", err)
	}
	// idempotent
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("second shutdown should be nil, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telemetry/ -run TestSetupDisabled -v`
Expected: FAIL — `undefined: Setup`.

- [ ] **Step 3: Write minimal implementation (Phase 1 stub — no providers yet)**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"log/slog"
)

// ShutdownFunc flushes and closes all telemetry providers. Safe to call once;
// the no-op variant is safe to call repeatedly.
type ShutdownFunc func(context.Context) error

func noopShutdown(context.Context) error { return nil }

// Setup constructs the logger and (when enabled) the OTel providers, registers
// them as globals, and returns a shutdown that flushes them. When telemetry is
// disabled (no OTLP endpoint) it returns a stdout logger and a no-op shutdown.
//
// Phase 1: providers are not yet built; only the logger is wired. Phase 2
// replaces the body below with real provider construction behind this same
// signature, so callers never change.
func Setup(ctx context.Context, cfg Config) (*slog.Logger, ShutdownFunc, error) {
	lg := NewLogger(cfg, nil)
	if !cfg.LogStdout {
		lg.Warn("MEM_LOG_STDOUT=false but no OTLP endpoint configured; forcing stdout so logs are not silently dropped")
	}
	return lg, noopShutdown, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telemetry/ -run TestSetup -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(telemetry): Setup seam returning logger + shutdown (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Migrate `internal/server` off stdlib log

**Files:**
- Modify: `internal/server/tools.go:10` (import), `:77-106` (`buildDepsFromEnv`, `warnOwnerlessRecords`), `:365` (`search_discovery` notice), `:393-395` (`Register`)
- Test: `internal/server/tools_test.go` (add)

- [ ] **Step 1: Write the failing test**

Add to `internal/server/tools_test.go`:

```go
func TestRegisterReturnsErrorOnStoreInitFailure(t *testing.T) {
	// Point Qdrant at an address that fails fast so StoreFromEnv errors and
	// Register surfaces it instead of calling log.Fatal.
	t.Setenv("MEM_QDRANT_ADDR", "bad-host:1") // SplitHostPort ok, dial later fails
	t.Setenv("MEM_EMBED_DIM", "not-a-number") // forces StoreFromEnv error pre-dial
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	if err := Register(s); err == nil {
		t.Fatal("Register must return an error when store init fails, not exit")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestRegisterReturnsError -v`
Expected: FAIL — `Register(s)` used as value / returns nothing (compile error), or process exits via `log.Fatalf`.

- [ ] **Step 3: Change `buildDepsFromEnv` to return an error**

Replace `internal/server/tools.go:77-88` (`buildDepsFromEnv`):

```go
func buildDepsFromEnv() (*deps, error) {
	st, err := StoreFromEnv()
	if err != nil {
		return nil, err
	}
	warnOwnerlessRecords(st)
	litellmURL := EnvOr("MEM_LITELLM_URL", "http://localhost:4000")
	litellmKey := EnvOr("MEM_LITELLM_KEY", "")
	embedModel := EnvOr("MEM_EMBED_MODEL", "ollama/bge-m3")
	em := embed.New(litellmURL, litellmKey, embedModel)
	return &deps{st: st, em: em}, nil
}
```

- [ ] **Step 4: Change `Register` to return an error and consume the deps error**

Replace `internal/server/tools.go:393-395` (`Register` signature + first line):

```go
// Register wires the memory tools onto the MCP server. It returns an error if
// dependency construction (store/embedder) fails, so the caller can flush
// telemetry and exit cleanly rather than aborting via log.Fatal.
func Register(s *mcp.Server) error {
	d, err := buildDepsFromEnv()
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}
```

Add `return nil` as the final statement of `Register` (before the closing brace at the former line `:479`).

- [ ] **Step 5: Replace `log` calls with `slog` in `warnOwnerlessRecords` and the discovery notice**

In `internal/server/tools.go`, replace the body of `warnOwnerlessRecords` (`:94-106`) log calls:

```go
func warnOwnerlessRecords(st *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := st.CountOwnerless(ctx)
	if err != nil {
		slog.Warn("could not check for pre-isolation (owner-less) records", "err", err)
		return
	}
	if n > 0 {
		slog.Warn("pre-isolation records have no owner; invisible to reads and not removable by delete_all until claimed",
			"count", n, "remedy", "engram migrate-set-owner --owner <your-oidc-sub>")
	}
}
```

Replace the `log.Print` at `:365` (`search_discovery` cross-spine notice) with:

```go
		slog.Info("search_discovery: cross_spine=true; ignoring supplied scope")
```

Update the import block at `internal/server/tools.go:7-24`: remove `"log"`, add `"log/slog"`.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestRegisterReturnsError -v`
Expected: PASS.
Run: `go test ./internal/server/ -v`
Expected: PASS (existing handler/schema tests unaffected).

- [ ] **Step 7: Commit**

```bash
jj commit -m "refactor(server): structured slog + error-returning Register (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Wire logger + error-returning Register into serve.go

**Files:**
- Modify: `cmd/engram/serve.go:6-18` (imports), `:50-60` (`runServe`), `:67-84` (`withAuth` logging)

- [ ] **Step 1: Replace `runServe` to set up the logger and consume `Register`'s error**

Replace `cmd/engram/serve.go:50-60` (`runServe`):

```go
func runServe() error {
	cfg := telemetry.ConfigFromEnv("engram", version)
	logger, shutdown, err := telemetry.Setup(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("telemetry setup: %w", err)
	}
	slog.SetDefault(logger)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	srv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: version}, nil)
	if err := server.Register(srv); err != nil {
		slog.Error("server registration failed", "err", err)
		return err
	}

	var handler http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil)
	handler = withAuth(handler)

	slog.Info("engram listening", "version", version, "addr", listenAddr)
	return http.ListenAndServe(listenAddr, handler) //nolint:gosec // timeouts added in Phase 2 Task 4
}
```

- [ ] **Step 2: Replace `log` calls in `withAuth` with slog and drop `log.Fatalf`**

Replace `cmd/engram/serve.go:67-84` (`withAuth`):

```go
func withAuth(handler http.Handler) http.Handler {
	if oidcIssuer == "" {
		slog.Warn("OIDC validation DISABLED (no --oidc-issuer / MEM_OIDC_ISSUER); all requests accepted")
		return handler
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	verifier, err := auth.New(ctx, oidcIssuer, oidcAudience)
	cancel()
	if err != nil {
		slog.Error("oidc verifier init failed", "err", err, "issuer", oidcIssuer)
		os.Exit(1)
	}

	slog.Info("OIDC bearer-token validation enabled", "issuer", oidcIssuer)
	return mcpauth.RequireBearerToken(verifier.TokenVerifier(), &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: oidcResourceMetadata,
	})(handler)
}
```

- [ ] **Step 3: Fix the import block**

Update `cmd/engram/serve.go:6-18`: remove `"log"`; add `"fmt"`, `"log/slog"`, `"os"`, and `"github.com/seanb4t/engram/internal/telemetry"`. Keep `"context"`, `"net/http"`, `"time"`, the mcp/auth/cobra imports, and `"github.com/seanb4t/engram/internal/server"`.

- [ ] **Step 4: Build and run the full suite**

Run: `go build ./... && go test ./... 2>&1 | tail -20`
Expected: build succeeds; all tests pass (Qdrant integration tests may skip without a live instance — that is pre-existing behavior, not a regression).

- [ ] **Step 5: Manual smoke check (logging shape)**

Run: `MEM_LOG_FORMAT=text go run ./cmd/engram serve` then Ctrl-C.
Expected: a structured line like `level=INFO msg="engram listening" version=dev addr=:8080` (no telemetry yet). Then:
Run: `go run ./cmd/engram serve` (default json) — expect `{"time":...,"level":"INFO","msg":"engram listening",...}`.

- [ ] **Step 6: Commit**

```bash
task license:add && task fmt
jj commit -m "feat(serve): bootstrap telemetry logger + clean startup errors (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Phase 2 — Telemetry bootstrap (real OTLP providers + graceful shutdown)

### Task 1: Add OTel dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

> If you ran this `go get` block as the Phase 1 Task 2 prerequisite, it is
> already done — this task then just re-verifies (`go mod tidy` + build). The
> commands are idempotent.

- [ ] **Step 1: `go get` the exporter, SDK, log, and bridge packages**

Run:

```bash
go get \
  go.opentelemetry.io/otel \
  go.opentelemetry.io/otel/sdk \
  go.opentelemetry.io/otel/sdk/metric \
  go.opentelemetry.io/otel/sdk/log \
  go.opentelemetry.io/otel/log \
  go.opentelemetry.io/otel/log/global \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc \
  go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc \
  go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc \
  go.opentelemetry.io/contrib/bridges/otelslog \
  go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
```

- [ ] **Step 2: Tidy and verify the module graph**

Run: `go mod tidy && go build ./...`
Expected: build succeeds; `internal/telemetry/logger.go` (which imports otelslog/otellog) now compiles. The `otelhttp v0.68.0` already present is reused.

Confirm the semconv subpackage used by `providers.go` resolves (it ships inside the `go.opentelemetry.io/otel` module, so no separate `go get` is needed):

Run: `go list go.opentelemetry.io/otel/semconv/v1.26.0`
Expected: prints the package path with no error. If it errors, pick the semconv version that matches the resolved `go.opentelemetry.io/otel` (e.g. `rg semconv $(go env GOMODCACHE)/go.opentelemetry.io/...` or check the module's `semconv/` dir) and update the import in `providers.go` Step 3 accordingly.

- [ ] **Step 3: Commit**

```bash
jj commit -m "build(deps): add OTel SDK, OTLP exporters, otelslog/otelgrpc bridges (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Build the OTel providers in `Setup`

**Files:**
- Create: `internal/telemetry/providers.go`
- Modify: `internal/telemetry/telemetry.go` (`Setup` body)
- Test: `internal/telemetry/telemetry_test.go` (add an enabled-path test using a bufconn-free construction check)

- [ ] **Step 1: Write the failing test**

Add to `internal/telemetry/telemetry_test.go`:

```go
func TestSetupEnabledBuildsProvidersAndShutsDown(t *testing.T) {
	// A syntactically valid endpoint; otlptracegrpc.New does not dial eagerly,
	// so construction succeeds without a live collector.
	cfg := Config{ServiceName: "engram", ServiceVersion: "test",
		OTLPEndpoint: "localhost:4317", LogLevel: "info", LogFormat: "json", LogStdout: true}
	lg, shutdown, err := Setup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Setup(enabled) error: %v", err)
	}
	if lg == nil {
		t.Fatal("logger must be non-nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}
```

Add `"time"` to the test imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telemetry/ -run TestSetupEnabled -v`
Expected: FAIL — current `Setup` ignores the endpoint and returns a no-op (test passes only superficially) OR fails once `Setup` is changed to call undefined `buildProviders`. (Write Step 3 first, then this test exercises the real path.)

- [ ] **Step 3: Implement provider construction**

Create `internal/telemetry/providers.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// buildProviders constructs the trace, metric, and log providers wired to OTLP
// gRPC exporters, registers them as OTel globals, and returns the log provider
// (for the otelslog bridge) plus a shutdown that flushes all three.
func buildProviders(ctx context.Context, cfg Config) (otellog.LoggerProvider, ShutdownFunc, error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	))
	if err != nil {
		return nil, nil, err
	}

	traceExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	tp := trace.NewTracerProvider(trace.WithBatcher(traceExp), trace.WithResource(res))

	metricExp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint), otlpmetricgrpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	mp := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExp)), metric.WithResource(res))

	logExp, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpoint(cfg.OTLPEndpoint), otlploggrpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	lp := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(logExp)), log.WithResource(res))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	logglobal.SetLoggerProvider(lp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))

	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}
	return lp, shutdown, nil
}
```

> Note: `WithInsecure()` matches a same-cluster collector with no TLS. If the
> collector requires TLS, drop `WithInsecure()` (the SDK then uses system
> roots) — track as a follow-up if the homelab collector enforces TLS.

- [ ] **Step 4: Rewrite `Setup` to use the providers when enabled**

Replace the `Setup` body in `internal/telemetry/telemetry.go`:

```go
func Setup(ctx context.Context, cfg Config) (*slog.Logger, ShutdownFunc, error) {
	if !cfg.Enabled() {
		lg := NewLogger(cfg, nil)
		if !cfg.LogStdout {
			lg.Warn("MEM_LOG_STDOUT=false but no OTLP endpoint configured; forcing stdout so logs are not silently dropped")
		}
		return lg, noopShutdown, nil
	}
	lp, shutdown, err := buildProviders(ctx, cfg)
	if err != nil {
		// Telemetry must never be a hard startup dependency: fall back to stdout.
		lg := NewLogger(cfg, nil)
		lg.Warn("telemetry setup failed; continuing with stdout logging only", "err", err)
		return lg, noopShutdown, nil
	}
	return NewLogger(cfg, lp), shutdown, nil
}
```

Add `"context"` if not already imported (it is, from Phase 1).

- [ ] **Step 5: Run telemetry tests**

Run: `go test ./internal/telemetry/ -v`
Expected: PASS — disabled path still returns no-op; enabled path builds providers and shuts down without a live collector.

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(telemetry): OTLP trace/metric/log providers + flush on shutdown (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Graceful shutdown with `http.Server`

**Files:**
- Modify: `cmd/engram/serve.go` (`runServe` server construction + signal handling)

- [ ] **Step 1: Replace `http.ListenAndServe` with an `http.Server` + signal handling**

Replace the tail of `runServe` (the `slog.Info("engram listening"...)` line and the `return http.ListenAndServe(...)`):

```go
	httpSrv := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("engram listening", "version", version, "addr", listenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-sigCtx.Done():
		slog.Info("shutdown signal received; draining")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
```

The deferred `telemetry` `shutdown(ctx)` from Task 5 (Phase 1) still runs after this returns, flushing the batchers after the HTTP server has drained.

- [ ] **Step 2: Update imports**

Add to `cmd/engram/serve.go` imports: `"errors"`, `"os/signal"`, `"syscall"`. (`"os"`, `"context"`, `"net/http"`, `"time"`, `"log/slog"` already present from Phase 1.) Remove the `//nolint:gosec` line added in Phase 1 Task 5 (timeouts now exist).

- [ ] **Step 3: Build and smoke-test graceful shutdown**

Run: `go build ./... && go vet ./...`
Expected: clean.
Run: `go run ./cmd/engram serve &` then `kill -TERM %1`.
Expected: a `shutdown signal received; draining` log line, then clean exit (no panic, no `log.Fatal`).

- [ ] **Step 4: Commit**

```bash
jj commit -m "feat(serve): http.Server with timeouts + SIGTERM graceful shutdown (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Phase 3 — Instrumentation at the three seams

### Task 1: Tool-call metric instruments

**Files:**
- Create: `internal/telemetry/metrics.go`
- Test: `internal/telemetry/metrics_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestNewToolMetricsRecordsWithoutPanic(t *testing.T) {
	// With the global no-op MeterProvider, instruments are valid and record
	// silently — this proves the construction + record path is safe when
	// telemetry is disabled.
	m := NewToolMetrics(otel.Meter("test"))
	m.Record(context.Background(), "store_memory", "ok", 12.3)
	m.RecordAuthFailure(context.Background(), "unauthorized")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telemetry/ -run TestNewToolMetrics -v`
Expected: FAIL — `undefined: NewToolMetrics`.

- [ ] **Step 3: Write the implementation**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ToolMetrics holds the engram-specific instruments. Built from the global
// meter; records are no-ops when telemetry is disabled.
type ToolMetrics struct {
	calls        metric.Int64Counter
	duration     metric.Float64Histogram
	authFailures metric.Int64Counter
}

// NewToolMetrics constructs the instruments. Instrument creation errors (only
// possible on invalid names, which are constant here) are ignored: a nil
// instrument from the no-op provider still records safely.
func NewToolMetrics(m metric.Meter) *ToolMetrics {
	calls, _ := m.Int64Counter("engram.tool.calls")
	dur, _ := m.Float64Histogram("engram.tool.duration",
		metric.WithUnit("ms"), metric.WithDescription("tool handler latency"))
	auth, _ := m.Int64Counter("engram.auth.failures")
	return &ToolMetrics{calls: calls, duration: dur, authFailures: auth}
}

// Record logs one tool call's count and latency.
func (t *ToolMetrics) Record(ctx context.Context, tool, outcome string, ms float64) {
	attrs := metric.WithAttributes(attribute.String("tool", tool), attribute.String("outcome", outcome))
	t.calls.Add(ctx, 1, attrs)
	t.duration.Record(ctx, ms, attrs)
}

// RecordAuthFailure counts a rejected request by reason.
func (t *ToolMetrics) RecordAuthFailure(ctx context.Context, reason string) {
	t.authFailures.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telemetry/ -run TestNewToolMetrics -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(telemetry): tool + auth-failure metric instruments (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: MCP tool-call middleware (span + metric + log)

**Files:**
- Create: `internal/server/instrument.go`
- Modify: `internal/server/tools.go` (`Register` adds the middleware)
- Test: `internal/server/instrument_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestInstrumentToolsExtractsToolNameAndOutcome(t *testing.T) {
	var sawTool, sawOutcome string
	record := func(_ context.Context, tool, outcome string, _ float64) {
		sawTool, sawOutcome = tool, outcome
	}
	mw := instrumentTools(record)

	inner := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{}, errors.New("boom")
	}
	h := mw(inner)
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "store_memory"}}

	_, _ = h(context.Background(), "tools/call", req)

	if sawTool != "store_memory" {
		t.Errorf("tool name: got %q", sawTool)
	}
	if sawOutcome != "error" {
		t.Errorf("outcome: got %q want error", sawOutcome)
	}
}

func TestInstrumentToolsIgnoresNonToolMethods(t *testing.T) {
	called := false
	record := func(context.Context, string, string, float64) { called = true }
	mw := instrumentTools(record)
	h := mw(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return &mcp.ListToolsResult{}, nil
	})
	_, _ = h(context.Background(), "tools/list", &mcp.ListToolsRequest{})
	if called {
		t.Error("non-tool method should not record tool metrics")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestInstrumentTools -v`
Expected: FAIL — `undefined: instrumentTools`.

- [ ] **Step 3: Write the middleware**

Create `internal/server/instrument.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// recordFunc records one tool call's metrics; matches telemetry.ToolMetrics.Record.
type recordFunc func(ctx context.Context, tool, outcome string, ms float64)

// instrumentTools returns an mcp.Middleware that wraps tool calls with a span,
// metrics, and a structured log line. Non-tool methods pass through with only a
// debug-level trace. The tool name comes from the *mcp.CallToolRequest params;
// outcome is "error" when the handler errors or the result IsError.
func instrumentTools(record recordFunc) mcp.Middleware {
	tracer := otel.Tracer("github.com/seanb4t/engram")
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			ctr, ok := req.(*mcp.CallToolRequest)
			if !ok || ctr.Params == nil {
				return next(ctx, method, req)
			}
			tool := ctr.Params.Name

			ctx, span := tracer.Start(ctx, "tool/"+tool, oteltrace.WithSpanKind(oteltrace.SpanKindServer))
			span.SetAttributes(attribute.String("engram.tool", tool))
			start := time.Now()

			res, err := next(ctx, method, req)

			outcome := classifyOutcome(res, err)
			ms := float64(time.Since(start).Microseconds()) / 1000.0
			record(ctx, tool, outcome, ms)

			actor, owner := identityForLog(ctx)
			lg := slog.With("tool", tool, "outcome", outcome, "dur_ms", ms, "actor", actor, "owner", owner)
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				span.RecordError(err)
				lg.Error("tool call failed", "err", err)
			} else {
				lg.Info("tool call")
			}
			span.End()
			return res, err
		}
	}
}

func classifyOutcome(res mcp.Result, err error) string {
	if err != nil {
		return "error"
	}
	if ctr, ok := res.(*mcp.CallToolResult); ok && ctr.IsError {
		return "error"
	}
	return "ok"
}

// identityForLog extracts the verified actor (human-readable) and owner (sub)
// from context for log attribution. Both are "" when auth is disabled. It
// reuses the package's existing accessors rather than re-deriving from
// mcpauth, so there is a single source of truth for identity extraction.
func identityForLog(ctx context.Context) (actor, owner string) {
	actor = actorFromContext(ctx)
	owner, _ = ownerFromContext(ctx) // log path: a missing-subject error degrades to "" rather than failing the log
	return actor, owner
}
```

`actorFromContext(ctx) string` and `ownerFromContext(ctx) (string, error)` already
exist in `tools.go:303-327` in this same `package server`, so `instrument.go`
needs **no** `mcpauth` import — it calls those helpers directly.

- [ ] **Step 4: Register the middleware**

In `internal/server/tools.go` `Register`, after `d, err := buildDepsFromEnv()` error handling and before the first `mcp.AddTool`, add:

```go
	tm := telemetry.NewToolMetrics(otel.Meter("github.com/seanb4t/engram"))
	s.AddReceivingMiddleware(instrumentTools(tm.Record))
```

Add imports to `tools.go`: `"go.opentelemetry.io/otel"` and `"github.com/seanb4t/engram/internal/telemetry"`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/server/ -run TestInstrumentTools -v`
Expected: PASS.
Run: `go test ./internal/server/ -v`
Expected: PASS (schema + handler tests unaffected; the middleware only activates on `tools/call`).

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(server): per-tool span/metric/log middleware via AddReceivingMiddleware (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: HTTP access-log + auth-failure middleware + otelhttp

**Files:**
- Create: `cmd/engram/httplog.go`
- Modify: `cmd/engram/serve.go` (`runServe` handler chain)
- Test: `cmd/engram/httplog_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccessLogCapturesStatusAndCountsAuthFailures(t *testing.T) {
	var gotStatus int
	var authReason string
	mw := accessLog(
		func(_ context.Context, reason string) { authReason = reason },
		func(status int) { gotStatus = status },
	)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if gotStatus != http.StatusUnauthorized {
		t.Errorf("captured status: got %d", gotStatus)
	}
	if authReason != "unauthorized" {
		t.Errorf("auth reason: got %q", authReason)
	}
}
```

> The second `accessLog` argument (`func(int)`) is a test seam to observe the
> captured status; in production it is `nil`. The first argument is the real
> auth-failure recorder.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/engram/ -run TestAccessLog -v`
Expected: FAIL — `undefined: accessLog`.

- [ ] **Step 3: Write the middleware**

Create `cmd/engram/httplog.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// authFailureFunc counts a rejected request by reason; matches
// telemetry.ToolMetrics.RecordAuthFailure.
type authFailureFunc func(ctx context.Context, reason string)

// accessLog emits one structured log line per request and counts 401/403
// responses as auth failures. observe is a test seam (nil in production).
func accessLog(authFail authFailureFunc, observe func(int)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(sw, r)
			ms := float64(time.Since(start).Microseconds()) / 1000.0

			switch sw.status {
			case http.StatusUnauthorized:
				authFail(r.Context(), "unauthorized")
			case http.StatusForbidden:
				authFail(r.Context(), "forbidden")
			}
			if observe != nil {
				observe(sw.status)
			}

			slog.Info("http request",
				"method", r.Method, "path", r.URL.Path, "status", sw.status,
				"dur_ms", ms, "remote", r.RemoteAddr, "ua", r.UserAgent())
		})
	}
}
```

- [ ] **Step 4: Wire the chain in `runServe`**

In `cmd/engram/serve.go`, after `handler = withAuth(handler)` and before building `httpSrv`, add:

```go
	tm := telemetry.NewToolMetrics(otel.Meter("github.com/seanb4t/engram"))
	handler = accessLog(tm.RecordAuthFailure, nil)(handler)
	handler = otelhttp.NewHandler(handler, "mcp")
```

Add imports to `serve.go`: `"go.opentelemetry.io/otel"`, `"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"`, and `"github.com/seanb4t/engram/internal/telemetry"` (already imported).

- [ ] **Step 5: Run tests**

Run: `go test ./cmd/engram/ -run TestAccessLog -v && go build ./...`
Expected: PASS + clean build.

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(serve): HTTP access log + auth-failure metric + otelhttp (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Auth-failure reason logging in the verifier

**Files:**
- Modify: `internal/auth/auth.go:59-78` (`TokenVerifier`)
- Test: `internal/auth/auth_test.go` (add)

- [ ] **Step 1: Write the failing test**

Add to `internal/auth/auth_test.go`:

```go
func TestTokenVerifierLogsRejectionReason(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	// A verifier whose underlying IDTokenVerifier rejects everything.
	v := &Verifier{idv: nil} // see Step 3 note: guard nil or inject a stub
	_, err := v.TokenVerifier()(context.Background(), "garbage", nil)
	if err == nil {
		t.Fatal("expected rejection")
	}
	if !strings.Contains(buf.String(), "token rejected") {
		t.Errorf("expected a 'token rejected' log line, got %q", buf.String())
	}
}
```

> Implementation note: `Verifier.idv` is unexported and `oidc.IDTokenVerifier`
> is concrete (hard to stub). If a nil `idv` panics rather than errors, instead
> add a tiny unexported interface seam `type idVerifier interface { Verify(ctx
> context.Context, token string) (*oidc.IDToken, error) }`, store that in
> `Verifier`, and inject a stub returning an error in the test. Choose the
> seam during implementation; the assertion (a "token rejected" log on failure)
> is the requirement.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run TestTokenVerifierLogsRejectionReason -v`
Expected: FAIL — no log line emitted on rejection.

- [ ] **Step 3: Add reason logging at the failure point**

In `internal/auth/auth.go`, the verify-failure branch (`:61-66`) becomes:

```go
		idt, err := v.idv.Verify(ctx, token)
		if err != nil {
			slog.Warn("token rejected", "err", err)
			// Join keeps ErrInvalidToken in the chain (so RequireBearerToken maps
			// to 401) while preserving the underlying verification error.
			return nil, errors.Join(mcpauth.ErrInvalidToken, err)
		}
```

Add `"log/slog"` to the `internal/auth/auth.go` import block.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/auth/ -v`
Expected: PASS (the new test plus existing auth tests).

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(auth): log bearer-token rejection reason at verify failure (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Embedder transport seam + otelhttp; Qdrant otelgrpc dial option

**Files:**
- Modify: `internal/embed/embed.go:17-28` (add `Option`/`WithHTTPTransport`)
- Modify: `internal/server/tools.go:60` (Qdrant `GrpcOptions`), `:86` (embed transport)
- Test: `internal/embed/embed_test.go` (add)

- [ ] **Step 1: Write the failing test for the embedder seam**

Add to `internal/embed/embed_test.go`:

```go
func TestWithHTTPTransportIsApplied(t *testing.T) {
	marker := &markerTransport{}
	c := New("http://x", "k", "m", WithHTTPTransport(marker))
	if c.http.Transport != marker {
		t.Fatal("WithHTTPTransport did not set the client transport")
	}
}

type markerTransport struct{}

func (m *markerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(r)
}
```

Add `"net/http"` to the test imports if absent.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/embed/ -run TestWithHTTPTransport -v`
Expected: FAIL — `undefined: WithHTTPTransport` / `New` takes 3 args.

- [ ] **Step 3: Add the functional-option seam to `embed.New`**

Replace `internal/embed/embed.go:25-28` (`New`):

```go
// Option customizes a Client.
type Option func(*Client)

// WithHTTPTransport sets the underlying RoundTripper (e.g. otelhttp.NewTransport)
// so embedder HTTP calls can be traced. The 30s timeout is preserved.
func WithHTTPTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.http.Transport = rt }
}

// New returns an embedding Client for the given base URL, API key, and model.
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, model: model, http: &http.Client{Timeout: 30 * time.Second}}
	for _, o := range opts {
		o(c)
	}
	return c
}
```

- [ ] **Step 4: Run the embedder test**

Run: `go test ./internal/embed/ -run TestWithHTTPTransport -v`
Expected: PASS.

- [ ] **Step 5: Apply otelhttp transport at the embed call site**

In `internal/server/tools.go` `buildDepsFromEnv` (`:86`), replace the `em := embed.New(...)` line:

```go
	em := embed.New(litellmURL, litellmKey, embedModel,
		embed.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)))
```

Add imports to `tools.go`: `"net/http"` and `"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"`.

- [ ] **Step 6: Add the otelgrpc stats handler to the Qdrant client**

In `internal/server/tools.go` `StoreFromEnv` (`:60`), replace the `qc, err := qdrant.NewClient(...)` line:

```go
	qc, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
		GrpcOptions: []grpc.DialOption{
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		},
	})
```

Add imports to `tools.go`: `"google.golang.org/grpc"` and `"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"`.

- [ ] **Step 7: Build and run the full suite**

Run: `go build ./... && go test ./... 2>&1 | tail -20`
Expected: build succeeds; unit tests pass (Qdrant integration tests skip/pass per existing behavior).

- [ ] **Step 8: Commit**

```bash
task license:add && task fmt
jj commit -m "feat(embed,store): otelhttp embedder transport + otelgrpc Qdrant handler (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Phase 4 — Helm wiring + docs + final gates

### Task 1: Helm chart env wiring

**Files:**
- Modify: `charts/engram/values.yaml`, `charts/engram/templates/deployment.yaml`

- [ ] **Step 1: Inspect the current chart env block**

Run: `rg -n "env:|MEM_|OTEL|name:|value" charts/engram/templates/deployment.yaml | head -40`
Expected: locate the container `env:` list where `MEM_*` vars are set.

- [ ] **Step 2: Add observability values to `values.yaml`**

Add a block to `charts/engram/values.yaml` (follow the file's existing indentation/section style):

```yaml
observability:
  # OTLP collector endpoint (host:port). Empty disables telemetry export.
  otlpEndpoint: ""
  # Extra OTel resource attributes, e.g. "deployment.environment=prod".
  resourceAttributes: ""
  log:
    level: info   # debug|info|warn|error
    format: json  # json|text
    stdout: true  # also write logs to stdout
```

- [ ] **Step 3: Wire the env vars in `deployment.yaml`**

Add to the container `env:` list (match existing `name/value` style; gate the OTLP endpoint so it is only set when non-empty):

```yaml
            - name: MEM_LOG_LEVEL
              value: {{ .Values.observability.log.level | quote }}
            - name: MEM_LOG_FORMAT
              value: {{ .Values.observability.log.format | quote }}
            - name: MEM_LOG_STDOUT
              value: {{ .Values.observability.log.stdout | quote }}
            {{- if .Values.observability.otlpEndpoint }}
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: {{ .Values.observability.otlpEndpoint | quote }}
            {{- end }}
            {{- if .Values.observability.resourceAttributes }}
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: {{ .Values.observability.resourceAttributes | quote }}
            {{- end }}
```

- [ ] **Step 4: Lint/render the chart**

Run: `helm lint charts/engram && helm template charts/engram --set observability.otlpEndpoint=otel:4317 | rg -n "OTEL_EXPORTER_OTLP_ENDPOINT|MEM_LOG"`
Expected: lint passes; rendered output shows the new env vars; with the endpoint unset they are absent.

- [ ] **Step 5: Commit**

```bash
task lint
jj commit -m "feat(chart): OTEL_* and MEM_LOG_* env wiring (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Docs + final quality gates

**Files:**
- Modify: `README.md` (observability/config section), `CLAUDE.md` (Auth/Conventions note if warranted)

- [ ] **Step 1: Document the observability config surface**

Add a short "Observability" section to `README.md` listing the env vars from the spec's Configuration table (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_RESOURCE_ATTRIBUTES`, `MEM_LOG_LEVEL`, `MEM_LOG_FORMAT`, `MEM_LOG_STDOUT`), noting that telemetry is disabled when no endpoint is set and that logs always reach stdout unless explicitly disabled with a configured OTLP endpoint.

- [ ] **Step 2: Run the full gate**

Run: `task` (lint + test) then `task license:check`
Expected: all green; every new Go/Markdown file carries the SPDX header.

- [ ] **Step 3: Verify no stray `log.` / `fmt.Print` remain in server paths**

Run: `rg -n "\blog\.(Printf|Println|Fatal|Print)\b" cmd/ internal/`
Expected: no matches in `cmd/engram/serve.go`, `internal/server/tools.go`, `internal/auth/auth.go` (the migration is complete). `cmd/engram/version.go`'s `fmt.Println(version)` is intentional CLI output and may remain.

- [ ] **Step 4: Commit**

```bash
jj commit -m "docs(observability): document OTEL_*/MEM_LOG_* config surface (engram-ew7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Validation Summary

After all phases, verify against the spec:

1. **Per-request logging** — `accessLog` emits one line per HTTP request; the MCP middleware emits one per tool call with actor/owner/outcome/dur_ms. ✓ (Phase 3 Tasks 2-3)
2. **Auth-failure logging + metric** — verifier logs the reason (Phase 3 Task 4); HTTP middleware counts `engram.auth.failures{reason}` (Phase 3 Task 3). ✓
3. **Full telemetry at every seam** — HTTP (`otelhttp`), tool (`AddReceivingMiddleware` span), embedder (`otelhttp.NewTransport`), Qdrant (`otelgrpc`). ✓ (Phase 3)
4. **Metrics** — `engram.tool.calls`, `engram.tool.duration`, `engram.auth.failures`, plus otelhttp/otelgrpc client+server metrics. ✓
5. **No-op when disabled** — `Setup` returns stdout logger + no-op shutdown with no endpoint; instruments record into the no-op global providers. ✓ (Phase 1-2)
6. **Flush on SIGTERM** — graceful shutdown drains HTTP then flushes batchers. ✓ (Phase 2 Task 3)
7. **Dual-sink logs, stdout disableable, silent-process guard** — `NewLogger` fan-out + guard. ✓ (Phase 1 Task 2)
