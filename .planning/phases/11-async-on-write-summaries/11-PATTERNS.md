# Phase 11: Async-on-Write Summaries - Pattern Map

**Mapped:** 2026-07-09
**Files analyzed:** 7 (2 new, 5 modified)
**Analogs found:** 7 / 7 (all in-repo; no external-pattern fallback needed)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/server/summaryqueue.go` (NEW) | service (worker pool) | event-driven | none in-repo — net-new pattern (see below) | no analog |
| `internal/server/summaryqueue_test.go` (NEW) | test | event-driven | `internal/store/summarize_test.go` (SummarizeFunc fakes) | role-match |
| `internal/server/tools.go` (`deps` struct + `storeMemory`/`scheduleMemory` tails) | controller / handler | request-response → enqueue | itself (existing struct/handlers being extended) | exact |
| `internal/telemetry/metrics.go` | service (instrumentation) | event-driven (async gauge) + request-response (counters/histogram) | `ToolMetrics`/`NewToolMetrics` in same file | exact |
| `internal/config/registry.go` + `internal/config/config.go` (`SummarizeConfig`) | config | CRUD (config load) | `summarize.model`/`SummarizeConfig` fields, same file | exact |
| `internal/config/validate.go` (`Validate()`) | config / validation | request-response (fail-fast) | `c.Summarize.Model != ""` gated block, same file | exact |
| `cmd/engram/serve.go` (shutdown block) | lifecycle wiring | event-driven (signal → drain) | itself (existing `sigCtx`/`httpSrv.Shutdown` block) | exact |

## Pattern Assignments

### `internal/server/summaryqueue.go` (NEW — service, event-driven)

**No direct analog exists in this repo.** Grep confirms exactly one `go func()` site in production code (`cmd/engram/serve.go:195`, the HTTP listen goroutine) and zero `sync.WaitGroup` usage outside tests. This is a **net-new concurrency pattern** for the codebase — the planner should treat the worker-pool/retry/drain shape as new plumbing, not a copy of an existing local idiom. Use RESEARCH.md's Pattern 1/2/3 code (bounded non-blocking enqueue, `backoff.Retry`-wrapped worker loop, close-after-`httpSrv.Shutdown` drain) as the primary source — it was already verified against the current source this session.

**Constructor/injection idiom to copy** — mirrors how `deps` and `StoreAndSummarizerFromEnv` build the summarizer today (`internal/server/tools.go:232-257`, current line numbers verified this session — shifted by ~+1 line from RESEARCH.md's citations but same shape):

```go
// internal/server/tools.go:232-237 — summarizerFromConfig (the constructor the
// worker pool must reuse, not reimplement):
func summarizerFromConfig(cfg *config.Config) *summarize.Client {
	return summarize.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Summarize.Model, summaryMaxChars(cfg),
		summarize.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
		summarize.WithMaxTokens(summaryMaxTokens(cfg)),
		summarize.WithTimeout(summaryTimeout(cfg)))
}

// internal/server/tools.go:239-257 — StoreAndSummarizerFromEnv (the CLI's
// builder; the worker pool's construction in buildDepsFromEnv should follow
// this exact same shape — cfg → ensureStoreFromConfig → summarizerFromConfig):
func StoreAndSummarizerFromEnv() (*store.Store, *summarize.Client, string, int, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, nil, "", 0, err
	}
	if cfg.Summarize.Model == "" {
		return nil, nil, "", 0, fmt.Errorf("ENGRAM_SUMMARY_MODEL is empty: auto-summary is disabled")
	}
	st, err := ensureStoreFromConfig(cfg)
	if err != nil {
		return nil, nil, "", 0, err
	}
	return st, summarizerFromConfig(cfg), cfg.Summarize.Model, summaryMaxChars(cfg), nil
}
```

**FillSummary signature the worker calls** (`internal/store/summarize.go:75`, `internal/store/summarize.go:21-23` for the injected-func seam — verified current):

```go
// internal/store/summarize.go:21-23
type SummarizeFunc func(ctx context.Context, content string) (string, error)

// internal/store/summarize.go:75
func (s *Store) FillSummary(ctx context.Context, m Memory, summarize SummarizeFunc, model string, maxChars int) (filled bool, err error) {
```
Idempotent (`shouldSummarize`, `summarize.go:43`), vector-preserving (`SetSummary` → `SetPayload`, `summarize.go:61-68`). The worker is a thin caller — do not reimplement eligibility or persistence.

**Enqueue/pool/drain code shapes:** use RESEARCH.md verbatim — Pattern 1 (`tryEnqueue`, non-blocking `select`/`default`), Pattern 2 (`worker`, `backoff.Retry` with `WithMaxTries`/`WithNotify`), Pattern 3 corrected version (drain ordering: `httpSrv.Shutdown(ctx)` completes first, **then** `close(q.ch)`, **then** `wg.Wait()` — never close the channel while a handler could still be mid-flight).

---

### `internal/server/tools.go` — `deps` struct + `storeMemory`/`scheduleMemory` tails (controller, request-response → enqueue)

**Current `deps` struct** (`internal/server/tools.go:33-47`, verified this session):

```go
type deps struct {
	st *store.Store
	em interface {
		Embed(context.Context, string) ([]float32, error)
		EmbedQuery(context.Context, string) ([]float32, error)
	}
	now func() time.Time
	summaryMaxChars int
}
```
New field goes here, e.g. `summaryQueue *summaryQueue` (nil-safe per RESEARCH.md Pattern 1 — nil means disabled, no branching needed at call sites beyond the nil check inside `tryEnqueue`).

**`buildDepsFromEnv`** (`internal/server/tools.go:149-164`, sole non-test construction site for `deps`):

```go
func buildDepsFromEnv() (*deps, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, err
	}
	st, err := ensureStoreFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	warnOwnerlessRecords(st)
	em, err := embedderFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &deps{st: st, em: em, summaryMaxChars: summaryMaxChars(cfg)}, nil
}
```
This is where the queue is constructed (only when `cfg.SummaryOnWrite && cfg.Summarize.Model != ""`) and started.

**`Register()`** (`internal/server/tools.go:914-923`, exactly one caller repo-wide: `cmd/engram/serve.go:145` — confirmed via grep, safe to add a second return value):

```go
func Register(s *mcp.Server, mux *http.ServeMux, tm *telemetry.ToolMetrics, resolve connectResolver) error {
	d, err := buildDepsFromEnv()
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}
	if err := d.mountConnect(mux, resolve); err != nil {
		return fmt.Errorf("mount connect: %w", err)
	}
	s.AddReceivingMiddleware(instrumentTools(tm.Record))
	mcp.AddTool(s, &mcp.Tool{Name: "store_memory", ...}, ...)
```
Planner: change signature to return a drain/shutdown func (e.g. `func Register(...) (shutdown func(context.Context), err error)`), matching RESEARCH.md's Pitfall 5 recommendation.

**`storeMemory` tail** (`internal/server/tools.go:502-516`, current — single-line return needs splitting so enqueue only fires post-success):

```go
func (d *deps) storeMemory(ctx context.Context, a storeArgs) (string, string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(subj.Owner(), actorFromContext(ctx), d.clock())
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	return m.ID, m.ShortID, d.st.Upsert(ctx, m, vec)
}
```
→ split the last line into `if err := d.st.Upsert(ctx, m, vec); err != nil { return "", "", err }` then `d.summaryQueue.tryEnqueue(m.ID)` then `return m.ID, m.ShortID, nil` — exact shape given in RESEARCH.md's "Non-blocking enqueue call site" code example.

**`scheduleMemory` tail** (`internal/server/tools.go:528-549`) — identical shape/fix, same single-line-Upsert-return pattern to split.

**Discovery/rule exclusion (D-06):** `storeDiscovery` begins at `internal/server/tools.go:551` — do **not** add an enqueue call there or in the rule handler; this is the explicit negative-space instruction for D-06 (verify by grep-checking no `tryEnqueue` call lands in `storeDiscovery`/`storeRule`).

---

### `internal/telemetry/metrics.go` (service/instrumentation)

**Analog:** `ToolMetrics`/`NewToolMetrics`, same file (`internal/telemetry/metrics.go:16-31`, full file read this session — 110 lines, no re-read needed):

```go
type ToolMetrics struct {
	calls        metric.Int64Counter
	duration     metric.Float64Histogram
	authFailures metric.Int64Counter
}

func NewToolMetrics(m metric.Meter) *ToolMetrics {
	calls, _ := m.Int64Counter("engram.tool.calls")
	dur, _ := m.Float64Histogram("engram.tool.duration",
		metric.WithUnit("ms"), metric.WithDescription("tool handler latency"))
	auth, _ := m.Int64Counter("engram.auth.failures")
	return &ToolMetrics{calls: calls, duration: dur, authFailures: auth}
}

func (t *ToolMetrics) Record(ctx context.Context, tool, outcome string, ms float64) {
	attrs := metric.WithAttributes(attribute.String("tool", tool), attribute.String("outcome", outcome))
	t.calls.Add(ctx, 1, attrs)
	t.duration.Record(ctx, ms, attrs)
}
```

Copy this exact idiom for a new `SummaryQueueMetrics` type: instrument-creation errors ignored (`_`), constructed once from the shared meter, `Add`/`Record` helper methods that take `ctx` + attribute values. `RecordStoreOp`/`RecordEmbed` (`metrics.go:85-100`) show the package-state (`layer *layerMetrics`, nil-check-then-record) alternative if the planner prefers package-level helpers over a struct passed into `internal/server` — either is consistent with existing style (RESEARCH.md Open Question #2 flags this as a style choice, not a functional one).

For the new **queue-depth gauge** (`Int64ObservableGauge` + `WithInt64Callback`) — no existing analog in this repo (all current instruments are synchronous `Int64Counter`/`Float64Histogram`); use RESEARCH.md's verified code example:
```go
depthGauge, _ := m.Int64ObservableGauge("engram.summary_queue.depth",
	metric.WithUnit("{item}"),
	metric.WithDescription("current summary-on-write queue depth"),
	metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
		o.Observe(int64(len(q.ch)))
		return nil
	}),
)
```

**Where instruments are wired in:** `InitLayerMetrics` (`metrics.go:63-71`) is called once from `serve.go` after the meter provider is registered — the new `SummaryQueueMetrics` constructor should be called from the same `serve.go` startup sequence, alongside `tm := telemetry.NewToolMetrics(...)` (see `serve.go` around the `mcp.NewServer`/`Register` call at line 144-145).

---

### `internal/config/registry.go` + `internal/config/config.go` (`SummarizeConfig`)

**Existing `ENGRAM_SUMMARY_MODEL` declaration** (`internal/config/registry.go:36-39`, verified current — no `Default` on `model`, i.e. presence-enables; `max_chars`/`max_tokens`/`timeout` have string defaults):

```go
{Key: "summarize.model", Env: "ENGRAM_SUMMARY_MODEL"},
{Key: "summarize.max_chars", Env: "ENGRAM_SUMMARY_MAX_CHARS", Default: "280"},
{Key: "summarize.max_tokens", Env: "ENGRAM_SUMMARY_MAX_TOKENS", Default: "1024"},
{Key: "summarize.timeout", Env: "ENGRAM_SUMMARY_TIMEOUT", Default: "30s"},
```
New entries for `ENGRAM_SUMMARY_ON_WRITE` / `ENGRAM_SUMMARY_WORKERS` / `ENGRAM_SUMMARY_QUEUE_SIZE` land immediately after `summarize.timeout`, same `[]field{...}` block, same shape.

**`field` struct** (`registry.go:13-19`) — all values are `string` (koanf-string-first, parsed downstream): `Key`, `Env`, `Legacy`, `Flag`, `Default` all `string`. **Bool fields are represented as strings too** — the closest analog for a bool-typed env var is `ui.enabled` (`registry.go:48`, `config.go:112` — `Enabled string` koanf field, consumed as `cfg.UI.Enabled` string and only turned into a bool at the point of use, e.g. `webauth`/UI branching in `serve.go:101,120`). Follow this exact pattern for `ENGRAM_SUMMARY_ON_WRITE`: declare it as `string` in `SummarizeConfig`, default `"false"`, parse with `strconv.ParseBool` at the point of use (buildDepsFromEnv / Validate), not as a native Go `bool` field — this matches the project's "everything koanf-string, typed parsing at the edge" convention (int fields like `embed.dim` are also declared `string` and parsed with `strconv`).

**`SummarizeConfig` struct** (`internal/config/config.go:82-87`):

```go
type SummarizeConfig struct {
	Model     string `koanf:"model"`
	MaxChars  string `koanf:"max_chars"`
	MaxTokens string `koanf:"max_tokens"`
	Timeout   string `koanf:"timeout"`
}
```
New fields: `OnWrite string \`koanf:"on_write"\``, `Workers string \`koanf:"workers"\``, `QueueSize string \`koanf:"queue_size"\``.

**`Validate()` gated-validation pattern** (`internal/config/validate.go:84-94`, the `if c.Summarize.Model != ""` block — the exact idiom the new `ENGRAM_SUMMARY_WORKERS`/`_QUEUE_SIZE` positive-integer checks should nest inside, alongside a new bool-parse check for `ON_WRITE`):

```go
if c.Summarize.Model != "" {
	switch n, err := strconv.ParseUint(c.Summarize.MaxChars, 10, 64); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_MAX_CHARS %q: must be a positive integer: %w", c.Summarize.MaxChars, err))
	case n == 0:
		errs = append(errs, errors.New("ENGRAM_SUMMARY_MAX_CHARS must be greater than 0"))
	}

	// max_tokens is a non-negative ceiling; 0 omits the cap (gateway default).
	if _, err := strconv.ParseUint(c.Summarize.MaxTokens, 10, 64); err != nil {
		errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_MAX_TOKENS %q: must be a non-negative integer: %w", c.Summarize.MaxTokens, err))
	}
	// ... (timeout parse follows same shape)
}
```
D-01's "both model set AND on_write=true" gate is a natural extension of this same `if c.Summarize.Model != ""` block — validate `OnWrite`'s bool-parseability and `Workers`/`QueueSize`'s positive-integer-ness unconditionally (they're harmless if unused), but the *runtime* enable decision (`buildDepsFromEnv`) is where the AND-gate actually lives, mirroring how `oidc.issuer` presence-enables auth today.

---

### `cmd/engram/serve.go` — shutdown block (lifecycle wiring)

**Current shutdown sequence** (`cmd/engram/serve.go:191-210`, verified current — exact ordering the drain must respect):

```go
sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

serveErr := make(chan error, 1)
go func() {
	slog.Info("engram listening", "version", version, "addr", cfg.Server.ListenAddr)
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
**Critical ordering constraint (RESEARCH.md Pitfall 1, confirmed against this exact block):** the worker-pool drain (`summaryQueue.Shutdown(shutdownCtx)` or equivalent, returned from `Register()`) must run **after** `httpSrv.Shutdown(shutdownCtx)` completes, sharing the remainder of the same 15s `shutdownCtx` budget — e.g.:
```go
case <-sigCtx.Done():
	slog.Info("shutdown signal received; draining")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err := httpSrv.Shutdown(shutdownCtx)
	drainSummaryQueue(shutdownCtx) // NEW — only after Shutdown returns; never parallel
	return err
```
Never parallelize these two steps — closing the enqueue channel while a handler is still mid-flight (pre-`httpSrv.Shutdown` return) is a "send on closed channel" panic risk.

**Register() call site** (`serve.go:144-148`) — the sole place the drain handle is captured:
```go
srv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: version}, nil)
if err := server.Register(srv, mux, tm, connectResolve); err != nil {
	slog.Error("server registration failed", "err", err)
	return err
}
```
→ becomes `drainSummaries, err := server.Register(...)` with `drainSummaries` invoked from the shutdown branch above.

---

## Shared Patterns

### Config: string-typed fields, edge-parsed
**Source:** `internal/config/config.go` (all config structs), `internal/config/registry.go`
**Apply to:** all three new `ENGRAM_SUMMARY_*` vars — declare as `string` koanf fields with string `Default`s in the registry; parse to `bool`/`int` only at the point of use (`buildDepsFromEnv`) and validate parseability in `Config.Validate()`. Do not introduce a native `bool`/`int` koanf field — no precedent for it anywhere in this config package.

### Presence-enables-a-feature gating
**Source:** `internal/config/validate.go:84` (`if c.Summarize.Model != ""`), `oidc.issuer` (`registry.go:42`, no default)
**Apply to:** D-01's two-switch gate (`Summarize.Model != "" && OnWrite == true`) — model presence continues to gate the CLI/summarizer construction; `OnWrite` is the additional AND-condition specific to the async worker, checked in `buildDepsFromEnv` before constructing the queue.

### Injected-function seam (store stays summarizer-ignorant)
**Source:** `internal/store/summarize.go:21-23` (`SummarizeFunc`), consumed by `FillSummary`/`SummarizeMissing`
**Apply to:** the worker pool must call `store.FillSummary(ctx, m, summarizeFunc, model, maxChars)` through the same injected-closure boundary — never have `internal/store` import `internal/summarize` directly, and the worker pool (living in `internal/server`, which already imports both) is the correct layer to bridge them, exactly as `summarizerFromConfig`/`StoreAndSummarizerFromEnv` already do for the CLI.

### OTel instrument construction: ignore-error-on-create, nil-safe record
**Source:** `internal/telemetry/metrics.go:25-31` (`NewToolMetrics`), `:63-71` (`InitLayerMetrics`)
**Apply to:** all new summary-queue instruments (gauge, histogram, 4 counters) — construct once from the shared meter at `serve.go` startup, ignore the `_` error return (invalid names are impossible here, they're constants), and let a nil instrument from a no-op provider record safely without extra nil checks in hot paths.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/server/summaryqueue.go` (worker pool type itself: pool struct, `worker()` goroutine loop, `backoff.Retry` wrapping, `Shutdown()` drain) | service | event-driven | No prior goroutine-pool / `sync.WaitGroup` / retry-library usage anywhere in `internal/` or `cmd/` (confirmed via repo-wide grep this session — the only production `go func()` is the single HTTP-listen goroutine in `serve.go:195`). This is genuinely net-new plumbing; RESEARCH.md's Pattern 1/2/3 code (verified against the real `cenkalti/backoff/v5` API and this repo's real shutdown sequence) is the primary source, not an in-repo analog. |

## Metadata

**Analog search scope:** `internal/server/`, `internal/telemetry/`, `internal/config/`, `internal/store/`, `cmd/engram/`
**Files scanned:** `tools.go`, `metrics.go`, `registry.go`, `config.go`, `validate.go`, `summarize.go`, `serve.go` (all read/grepped this session; no re-reads of overlapping ranges)
**Pattern extraction date:** 2026-07-09
