<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Telemetry & Metrics At Every Seam — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Instrument the `store`, `embed`, and `auth` layers with inline OpenTelemetry spans and per-operation latency metrics, complete the OTel resource attribute set, and expose the OTel-standard sampler/interval + k8s resource knobs in the Helm chart.

**Architecture:** Each of `internal/store`, `internal/embed`, `internal/auth` gets a package-level `otel.Tracer` and starts a span per public method (attributes at creation, `RecordError`+`SetStatus` on failure, `defer span.End()`). Duration is recorded through layer-agnostic helpers in `internal/telemetry` that own the histogram instruments (single registration point, nil-safe when telemetry is off). The dependency is one-way: `store`/`embed`/`auth` → `telemetry`; `telemetry` never imports them. Resource construction in `providers.go` is rebuilt to opt into the full standard detector set. The Helm chart templates the standard `OTEL_*` env vars the SDK already honors.

**Tech Stack:** Go, `go.opentelemetry.io/otel` (trace + metric SDK v1.44.0), `sdk/trace/tracetest`, `sdk/metric` manual reader, Qdrant go-client, Helm.

**Design spec:** `docs/superpowers/specs/2026-06-11-telemetry-at-every-seam-design.md` (design bead `engram-sk8`, design-reviewer READY round 1).

---

## File Structure

| File | Responsibility | Change |
|------|----------------|--------|
| `internal/telemetry/metrics.go` | Owns all engram metric instruments | Add 3 layer histograms + `InitLayerMetrics` + `RecordStoreOp`/`RecordEmbed`/`RecordAuthVerify` |
| `internal/telemetry/metrics_test.go` | Metric helper unit tests | Create |
| `internal/telemetry/providers.go` | Provider + resource construction | Extract `buildResource`, add full detector set |
| `internal/telemetry/providers_test.go` | Resource unit test | Create |
| `cmd/engram/serve.go` | Server bootstrap | One line: `telemetry.InitLayerMetrics(...)` next to `NewToolMetrics` |
| `internal/store/store.go` | Qdrant-backed store | Package tracer + inline spans + `RecordStoreOp` in public methods |
| `internal/store/instrument_test.go` | Store span integration test | Create |
| `internal/embed/embed.go` | Embedder client | Package tracer + span in `Embed` + `RecordEmbed` |
| `internal/embed/embed_test.go` | Embed span unit test | Create (or extend) |
| `internal/auth/auth.go` | OIDC verifier | span in the `TokenVerifier` closure + `RecordAuthVerify` (reuses existing `idVerifier`) |
| `internal/auth/auth_test.go` | Auth span unit test | Create (or extend) |
| `internal/server/instrument.go` | Tool middleware | Add `engram.owner` to tool span |
| `charts/engram/values.yaml` | Chart config | Add `observability.traces.*`, `observability.metrics.exportInterval` |
| `charts/engram/templates/memory-mcp.yaml` | Deployment env | Template sampler/interval + k8s Downward-API resource attributes (replace existing `OTEL_RESOURCE_ATTRIBUTES` block) |

**Implementation order:** Task 1 (helpers) first — Tasks 2-4 depend on the `Record*` helpers existing. Tasks 5, 6, 7 are independent of each other and of 2-4.

---

### Task 1: Layer metric instruments + Record helpers

**Files:**

- Modify: `internal/telemetry/metrics.go`
- Create: `internal/telemetry/metrics_test.go`
- Modify: `cmd/engram/serve.go` (the line that builds `ToolMetrics`)

- [ ] **Step 1: Write the failing test**

Create `internal/telemetry/metrics_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// collectStoreDuration reads the engram.store.duration histogram via a manual
// reader and returns its data points.
func collectStoreDuration(t *testing.T, rdr metric.Reader) []metricdata.HistogramDataPoint[float64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := rdr.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "engram.store.duration" {
				continue
			}
			h, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("engram.store.duration is %T, want Histogram[float64]", m.Data)
			}
			return h.DataPoints
		}
	}
	return nil
}

func TestRecordStoreOpRecordsOperationAndOutcome(t *testing.T) {
	rdr := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(rdr))
	InitLayerMetrics(mp.Meter("test"))
	t.Cleanup(func() { layer = nil })

	RecordStoreOp(context.Background(), "Search", time.Now().Add(-5*time.Millisecond), nil)
	RecordStoreOp(context.Background(), "Upsert", time.Now(), errors.New("boom"))

	pts := collectStoreDuration(t, rdr)
	if len(pts) != 2 {
		t.Fatalf("got %d data points, want 2", len(pts))
	}
	got := map[string]string{}
	for _, p := range pts {
		op, _ := p.Attributes.Value("operation")
		out, _ := p.Attributes.Value("outcome")
		got[op.AsString()] = out.AsString()
	}
	if got["Search"] != "ok" {
		t.Errorf("Search outcome = %q, want ok", got["Search"])
	}
	if got["Upsert"] != "error" {
		t.Errorf("Upsert outcome = %q, want error", got["Upsert"])
	}
}

func TestRecordHelpersAreNilSafeWhenUninitialised(t *testing.T) {
	layer = nil // telemetry disabled
	// must not panic
	RecordStoreOp(context.Background(), "Search", time.Now(), nil)
	RecordEmbed(context.Background(), time.Now(), nil)
	RecordAuthVerify(context.Background(), time.Now(), errors.New("x"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telemetry/ -run TestRecordStoreOp -v`
Expected: FAIL — `undefined: InitLayerMetrics`, `undefined: layer`, `undefined: RecordStoreOp`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/telemetry/metrics.go` (add `"time"` to the import block):

```go
// layerMetrics holds the per-operation latency instruments for the store,
// embed, and auth seams. It is package state so the Record* helpers below can
// be called from internal/store|embed|auth WITHOUT those packages importing the
// meter or threading an instrument handle through every call. The dependency is
// one-way: those packages import telemetry; telemetry imports none of them.
type layerMetrics struct {
	storeDur metric.Float64Histogram
	embedDur metric.Float64Histogram
	authDur  metric.Float64Histogram
}

// layer is nil until InitLayerMetrics runs (telemetry disabled => helpers no-op).
var layer *layerMetrics

// InitLayerMetrics builds the store/embed/auth latency histograms from m and
// installs them as package state. Called once from serve.go after the meter
// provider is registered. Instrument-creation errors are ignored: a nil
// instrument from the no-op provider still records safely.
func InitLayerMetrics(m metric.Meter) {
	storeDur, _ := m.Float64Histogram("engram.store.duration",
		metric.WithUnit("ms"), metric.WithDescription("store operation latency"))
	embedDur, _ := m.Float64Histogram("engram.embed.duration",
		metric.WithUnit("ms"), metric.WithDescription("embedder call latency"))
	authDur, _ := m.Float64Histogram("engram.auth.verify.duration",
		metric.WithUnit("ms"), metric.WithDescription("token verification latency"))
	layer = &layerMetrics{storeDur: storeDur, embedDur: embedDur, authDur: authDur}
}

func outcomeOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func msSince(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}

// RecordStoreOp records one store method's latency keyed by operation+outcome.
func RecordStoreOp(ctx context.Context, op string, start time.Time, err error) {
	if layer == nil {
		return
	}
	layer.storeDur.Record(ctx, msSince(start), metric.WithAttributes(
		attribute.String("operation", op), attribute.String("outcome", outcomeOf(err))))
}

// RecordEmbed records one embedder call's latency keyed by outcome.
func RecordEmbed(ctx context.Context, start time.Time, err error) {
	if layer == nil {
		return
	}
	layer.embedDur.Record(ctx, msSince(start),
		metric.WithAttributes(attribute.String("outcome", outcomeOf(err))))
}

// RecordAuthVerify records one token verification's latency keyed by outcome.
func RecordAuthVerify(ctx context.Context, start time.Time, err error) {
	if layer == nil {
		return
	}
	layer.authDur.Record(ctx, msSince(start),
		metric.WithAttributes(attribute.String("outcome", outcomeOf(err))))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telemetry/ -run 'TestRecordStoreOp|TestRecordHelpers' -v`
Expected: PASS (both tests).

- [ ] **Step 5: Wire `InitLayerMetrics` into serve.go**

In `cmd/engram/serve.go`, find the line that builds the tool metrics (currently `tm := telemetry.NewToolMetrics(otel.Meter("github.com/seanb4t/engram"))`). Add immediately after it:

```go
	// Install the store/embed/auth latency instruments as telemetry package
	// state so the layer Record* helpers emit (no-op until this runs).
	telemetry.InitLayerMetrics(otel.Meter("github.com/seanb4t/engram"))
```

- [ ] **Step 6: Build to verify wiring**

Run: `go build ./...`
Expected: success, no errors.

- [ ] **Step 7: Run gofmt and commit**

Run: `gofmt -l internal/telemetry/metrics.go internal/telemetry/metrics_test.go cmd/engram/serve.go`
Expected: empty output (clean — see the gofmt CI trap below).

Commit using VCS-appropriate commands per `references/vcs-preamble.md`:
`feat(telemetry): layer latency instruments + Record helpers`

---

### Task 2: Store layer spans + metrics

**Files:**

- Modify: `internal/store/store.go`
- Create: `internal/store/instrument_test.go`

**Span/attribute specification** — every public method gets `ctx, span := tracer.Start(ctx, "<name>", trace.WithAttributes(<creation attrs>))`, `defer span.End()`, `defer telemetry.RecordStoreOp(ctx, "<op>", start, err)` (via a named return `err`), and on the error path `span.RecordError(err); span.SetStatus(codes.Error, err.Error())`. `engram.owner` is `subj.Owner()` where a `Subject` parameter exists.

| Method | Span name | `op` | Creation attrs | Result attrs |
|--------|-----------|------|----------------|--------------|
| `Search` | `store.Search` | `Search` | `engram.scope`, `engram.k`, `engram.owner` | `engram.result_count` |
| `SearchDiscovery` | `store.SearchDiscovery` | `SearchDiscovery` | `engram.scope`, `engram.kind`, `engram.k`, `engram.owner` | `engram.result_count` |
| `List` | `store.List` | `List` | `engram.scope`, `engram.owner` | `engram.result_count` |
| `ListScopes` | `store.ListScopes` | `ListScopes` | `engram.owner` | `engram.result_count` |
| `Upsert` | `store.Upsert` | `Upsert` | `engram.scope` (from `m.Scope`) | — |
| `Get` | `store.Get` | `Get` | — | — |
| `GetReadable` | `store.GetReadable` | `GetReadable` | `engram.owner` | — |
| `FetchForUpdate` | `store.FetchForUpdate` | `FetchForUpdate` | `engram.owner` | — |
| `Update` | `store.Update` | `Update` | — | — |
| `SetVisibility` | `store.SetVisibility` | `SetVisibility` | `engram.owner` | — |
| `OwnedOrAbsent` | `store.OwnedOrAbsent` | `OwnedOrAbsent` | `engram.owner` | — |
| `Delete` | `store.Delete` | `Delete` | `engram.owner` | — |
| `DeleteAll` | `store.DeleteAll` | `DeleteAll` | `engram.scope`, `engram.owner` | — |
| `EnsureCollection` | `store.EnsureCollection` | `EnsureCollection` | — | — |
| `MigrateSetOwner` | `store.MigrateSetOwner` | `MigrateSetOwner` | — | `engram.result_count` |
| `CountOwnerless` | `store.CountOwnerless` | `CountOwnerless` | — | `engram.result_count` |
| `CountAnonymousBucket` | `store.CountAnonymousBucket` | `CountAnonymousBucket` | — | `engram.result_count` |

> `engram.scope` is set on spans only — NOT a metric dimension (cardinality). `engram.owner` is the opaque OIDC `sub` via `subj.Owner()`; `actor`/email never appears on spans.

- [ ] **Step 1: Write the failing test**

Create `internal/store/instrument_test.go`. This is an integration test using the existing ephemeral-Qdrant harness (`testQdrantAddr` / `testStore`, see `store_test.go` TestMain):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// withSpanRecorder installs a SpanRecorder-backed global TracerProvider for the
// duration of the test and returns it. otel.Tracer delegates to the global
// provider at call time, so the package-level tracer picks this up.
func withSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

func spanByName(spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	for _, s := range spans {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func TestStoreSearchEmitsSpan(t *testing.T) {
	st := testStore(t) // skips if testQdrantAddr == "" (see store_test.go helpers)
	sr := withSpanRecorder(t)

	// testStore ensures a 3-dim collection (store_test.go: EnsureCollection(ctx, 3)).
	_, err := st.Search(context.Background(), "repo:spans", anonymous{}, make([]float32, 3), 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	sp := spanByName(sr.Ended(), "store.Search")
	if sp == nil {
		t.Fatal("no store.Search span recorded")
	}
	attrs := map[string]string{}
	for _, kv := range sp.Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	if attrs["engram.scope"] != "repo:spans" {
		t.Errorf("engram.scope = %q, want repo:spans", attrs["engram.scope"])
	}
	if _, ok := attrs["engram.result_count"]; !ok {
		t.Error("missing engram.result_count attribute")
	}
	if sp.Status().Code == codes.Error {
		t.Errorf("unexpected error status: %s", sp.Status().Description)
	}
}
```

> Confirm the exact `testStore(t)` helper name and the `anonymous{}` zero value against `internal/store/store_test.go` before running — if the helper is named differently (e.g. `newTestStore`), use that name. The `Subject` interface's anonymous case is the unexported `anonymous` struct (see `store.go:227`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestStoreSearchEmitsSpan -v`
Expected: FAIL — "no store.Search span recorded" (no span emitted yet). If Docker is unavailable the test SKIPS; in that case verify on a machine with Docker or set `MEM_QDRANT_TEST_ADDR`.

- [ ] **Step 3: Add the package tracer and imports**

At the top of `internal/store/store.go`, add to the import block:

```go
	"time"

	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
```

Add a package-level tracer near the top of the file (after the imports, before the first type):

```go
var tracer = otel.Tracer("github.com/seanb4t/engram/internal/store")
```

- [ ] **Step 4: Instrument `Search` (full example)**

Replace the body of `Search` (`store.go:259`) with:

```go
func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64) (out []Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.Search", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.Int64("engram.k", int64(k)),
		attribute.String("engram.owner", subj.Owner()),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Search", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(out)))
		}
	}()

	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: s.ownerScopeFilter(scope, subj), Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	return memoriesFromPoints(res), nil
}
```

> Pattern notes: named returns `(out []Memory, err error)` let the deferred closure read the result count and error. The `defer` ordering is `span.End()` registered first (runs last), `RecordStoreOp`+status second (runs first) — so status/attrs are set before `End`.

- [ ] **Step 5: Instrument `Upsert` (full example — no Subject, no result)**

Replace `Upsert` (`store.go:202`):

```go
func (s *Store) Upsert(ctx context.Context, m Memory, vec []float32) (err error) {
	ctx, span := tracer.Start(ctx, "store.Upsert",
		trace.WithAttributes(attribute.String("engram.scope", m.Scope)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Upsert", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	_, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(m.ID),
			Vectors: qdrant.NewVectors(vec...),
			Payload: qdrant.NewValueMap(payload(m)),
		}},
	})
	return err
}
```

> Confirm `Memory` has a `Scope` field (it does — `store.go:30`). If the field is named differently, use that name.

- [ ] **Step 6: Instrument `List` (full example — result is `(items, total, more, err)`)**

Replace `List` (`store.go:360`) preserving its existing 4-value return; wrap analogously:

```go
func (s *Store) List(ctx context.Context, scope string, subj Subject, opts ListOptions) (items []Memory, total uint64, more bool, err error) {
	ctx, span := tracer.Start(ctx, "store.List", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", subj.Owner()),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "List", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(items)))
		}
	}()

	// ... EXISTING List body, with its return statements unchanged ...
}
```

> Keep the existing `List` body verbatim inside the new wrapper — only the signature (named returns) and the prologue/deferred-closure are added. The existing `return a, b, c, nil` statements work unchanged because the names bind positionally.

- [ ] **Step 7: Instrument the remaining methods per the table**

Apply the identical pattern to every remaining method in the Task 2 table (`SearchDiscovery`, `ListScopes`, `Get`, `GetReadable`, `FetchForUpdate`, `Update`, `SetVisibility`, `OwnedOrAbsent`, `Delete`, `DeleteAll`, `EnsureCollection`, `MigrateSetOwner`, `CountOwnerless`, `CountAnonymousBucket`), using each row's span name, `op` string, creation attrs, and result attrs. For methods with no `Subject` param, omit `engram.owner`. For count/migrate methods, set `engram.result_count` to the returned `uint64` via `attribute.Int64("engram.result_count", int64(n))`. For `SearchDiscovery`, add `attribute.String("engram.kind", kind)`.

- [ ] **Step 8: Run the store span test**

Run: `go test ./internal/store/ -run TestStoreSearchEmitsSpan -v`
Expected: PASS.

- [ ] **Step 9: Run the full store package + build**

Run: `go build ./... && go test ./internal/store/ -v`
Expected: build OK; existing integration tests still PASS (spans flow through transparently).

- [ ] **Step 10: gofmt + commit**

Run: `gofmt -l internal/store/store.go internal/store/instrument_test.go`
Expected: empty.

Commit: `feat(store): inline spans + per-operation latency metrics`

---

### Task 3: Embed layer span + metrics

**Files:**

- Modify: `internal/embed/embed.go`
- Create: `internal/embed/embed_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/embed/embed_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package embed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestEmbedEmitsSpan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	c := New(srv.URL, "k", "bge-m3")
	vec, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != 3 {
		t.Fatalf("got %d dims, want 3", len(vec))
	}

	spans := sr.Ended()
	if len(spans) == 0 || spans[0].Name() != "embed.Embed" {
		t.Fatalf("want an embed.Embed span, got %v", spans)
	}
	attrs := map[string]string{}
	for _, kv := range spans[0].Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	if attrs["engram.embed.model"] != "bge-m3" {
		t.Errorf("engram.embed.model = %q, want bge-m3", attrs["engram.embed.model"])
	}
	if attrs["engram.embed.dims"] != "3" {
		t.Errorf("engram.embed.dims = %q, want 3", attrs["engram.embed.dims"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/embed/ -run TestEmbedEmitsSpan -v`
Expected: FAIL — no `embed.Embed` span.

- [ ] **Step 3: Add tracer + instrument `Embed`**

In `internal/embed/embed.go`, add to imports (note: `time` is already imported for `New`'s 30s timeout — do not duplicate it):

```go
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
```

Add the package tracer after imports:

```go
var tracer = otel.Tracer("github.com/seanb4t/engram/internal/embed")
```

Replace `Embed` (`embed.go:55`) with a wrapped version:

```go
func (c *Client) Embed(ctx context.Context, text string) (vec []float32, err error) {
	ctx, span := tracer.Start(ctx, "embed.Embed",
		trace.WithAttributes(attribute.String("engram.embed.model", c.model)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordEmbed(ctx, start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.embed.dims", len(vec)))
		}
	}()

	body, _ := json.Marshal(embedReq{Model: c.model, Input: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings: status %d", resp.StatusCode)
	}
	var out embedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("embeddings: empty data")
	}
	return out.Data[0].Embedding, nil
}
```

> Note on named returns: every `return X, errExpr` statement assigns the named returns `vec`/`err` from its expressions, regardless of any inner `:=` shadowing — so each error path returns the right error to the deferred closure, and the final `return out.Data[0].Embedding, nil` sets `err == nil` (closure records `outcome="ok"`, `dims=len(vec)`). The `req, err :=` / `resp, err :=` lines assign the function-scope `err`; the inner `if err := json...Decode(&out); err != nil { return nil, err }` uses a block-scoped `err` but its explicit `return nil, err` still propagates that error into the named return. Net: correct on every path.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/embed/ -run TestEmbedEmitsSpan -v`
Expected: PASS.

- [ ] **Step 5: Build + gofmt + commit**

Run: `go build ./... && gofmt -l internal/embed/embed.go internal/embed/embed_test.go`
Expected: build OK, gofmt empty.

Commit: `feat(embed): span + latency metric on Embed`

---

### Task 4: Auth layer span + metrics

**Files:**

- Modify: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go` (or extend an existing one)

**Why this is testable already:** the span wraps the `idv.Verify` call inside the closure returned by `TokenVerifier()`. `Verifier.idv` is ALREADY the unexported interface `idVerifier` (`auth.go:29-36`: `Verify(ctx, rawIDToken) (*oidc.IDToken, error)`), specifically so tests can inject a stub. **No struct change or new interface is needed** — the test fake just satisfies the existing `idVerifier`. (An earlier draft of this plan proposed adding `idTokenVerifier`; that was wrong — reuse `idVerifier`.)

- [ ] **Step 1: Write the failing test**

The `Verifier{idv: ...}` literal works directly — `idv` is already the `idVerifier` interface (`auth.go:36`), so `fakeIDV` (which has `Verify(ctx, string) (*oidc.IDToken, error)`) satisfies it with no production change.

Create `internal/auth/auth_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type fakeIDV struct {
	tok *oidc.IDToken
	err error
}

func (f fakeIDV) Verify(_ context.Context, _ string) (*oidc.IDToken, error) {
	return f.tok, f.err
}

func recorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

func TestTokenVerifierSpanSuccess(t *testing.T) {
	sr := recorder(t)
	v := &Verifier{idv: fakeIDV{tok: &oidc.IDToken{Subject: "user-1"}}}
	info, err := v.TokenVerifier()(context.Background(), "tok", nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if info.Extra["sub"] != "user-1" {
		t.Errorf("sub = %v, want user-1", info.Extra["sub"])
	}
	sp := sr.Ended()
	if len(sp) != 1 || sp[0].Name() != "auth.VerifyToken" {
		t.Fatalf("want auth.VerifyToken span, got %v", sp)
	}
	if got := attr(sp[0], "engram.auth.outcome"); got != "ok" {
		t.Errorf("outcome = %q, want ok", got)
	}
}

func TestTokenVerifierSpanError(t *testing.T) {
	sr := recorder(t)
	v := &Verifier{idv: fakeIDV{err: errors.New("bad token")}}
	_, err := v.TokenVerifier()(context.Background(), "tok", nil)
	if err == nil {
		t.Fatal("want error")
	}
	sp := sr.Ended()
	if len(sp) != 1 {
		t.Fatalf("want 1 span, got %d", len(sp))
	}
	if sp[0].Status().Code != codes.Error {
		t.Errorf("status = %v, want Error", sp[0].Status().Code)
	}
	if got := attr(sp[0], "engram.auth.outcome"); got != "error" {
		t.Errorf("outcome = %q, want error", got)
	}
}

func attr(s sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.Emit()
		}
	}
	return ""
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run TestTokenVerifierSpan -v`
Expected: FAIL — the test COMPILES (`fakeIDV` satisfies the existing `idVerifier`) but fails with "want auth.VerifyToken span" because no span is emitted yet.

- [ ] **Step 3: Add tracer + instrument the closure**

Add to the `auth.go` import block: `"time"`, `"github.com/seanb4t/engram/internal/telemetry"`, `"go.opentelemetry.io/otel"`, `"go.opentelemetry.io/otel/attribute"`, `"go.opentelemetry.io/otel/codes"`, `"go.opentelemetry.io/otel/trace"`.

Add the package tracer:

```go
var tracer = otel.Tracer("github.com/seanb4t/engram/internal/auth")
```

Rewrite the closure body in `TokenVerifier` (`auth.go:68`) — the span lives INSIDE the returned closure, not on the method:

```go
func (v *Verifier) TokenVerifier() mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (info *mcpauth.TokenInfo, err error) {
		ctx, span := tracer.Start(ctx, "auth.VerifyToken")
		defer span.End()
		start := time.Now()
		defer func() {
			telemetry.RecordAuthVerify(ctx, start, err)
			if err != nil {
				span.SetAttributes(attribute.String("engram.auth.outcome", "error"))
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetAttributes(attribute.String("engram.auth.outcome", "ok"))
			}
		}()

		idt, verr := v.idv.Verify(ctx, token)
		if verr != nil {
			slog.WarnContext(ctx, "token rejected", "err", verr)
			err = errors.Join(mcpauth.ErrInvalidToken, verr)
			return nil, err
		}
		var claims identityClaims
		_ = idt.Claims(&claims)
		return &mcpauth.TokenInfo{
			UserID:     identity(idt.Subject, claims.Email, claims.PreferredUsername),
			Expiration: idt.Expiry,
			Extra:      map[string]any{"sub": idt.Subject, "email": claims.Email},
		}, nil
	}
}
```

> The named returns `(info, err)` feed the deferred closure. `verr` is used locally then assigned to `err` so the error path records correctly. The success `return &mcpauth.TokenInfo{...}, nil` leaves `err == nil`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/ -run TestTokenVerifierSpan -v`
Expected: PASS (both success and error).

- [ ] **Step 5: Build + gofmt + commit**

Run: `go build ./... && gofmt -l internal/auth/auth.go internal/auth/auth_test.go`
Expected: build OK, gofmt empty.

Commit: `feat(auth): span + latency metric on token verification`

---

### Task 5: `engram.owner` on tool spans

**Files:**

- Modify: `internal/server/instrument.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/instrument_test.go` (the file already exists with the tool-instrument tests). Append:

```go
func TestInstrumentToolsSetsOwnerSpanAttribute(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	mw := instrumentTools(func(context.Context, string, string, float64) {})
	h := mw(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{}, nil
	})
	ctx := authedContext(t, "owner-xyz") // verified-subject context helper, see tools_test.go
	_, _ = h(ctx, "tools/call", &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "search_memory"}})

	sp := sr.Ended()
	if len(sp) != 1 {
		t.Fatalf("want 1 span, got %d", len(sp))
	}
	if got := spanAttr(sp[0], "engram.owner"); got != "owner-xyz" {
		t.Errorf("engram.owner = %q, want owner-xyz", got)
	}
}
```

> Reuse the `authedContext(t, sub)` helper from `internal/server/tools_test.go:196` (it injects a verified OIDC subject past the go-sdk's unexported token-context key). Add a small `spanAttr` helper if one doesn't already exist in the package's test files, and confirm the `recordFunc` signature `func(ctx, tool, outcome string, ms float64)` matches the mock arg list. Add the otel/tracetest imports to the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestInstrumentToolsSetsOwner -v`
Expected: FAIL — `engram.owner` empty (not set on span).

- [ ] **Step 3: Set the owner attribute on the tool span**

In `internal/server/instrument.go`, the tool span is created at line 38 and `engram.tool` set at line 40; `identityForLog(ctx)` is called at line 49. Hoist the identity extraction to just after the span attributes and reuse it for both the span and the existing log line:

```go
		actor, owner := identityForLog(ctx)
		if owner != "" {
			span.SetAttributes(attribute.String("engram.owner", owner))
		}
```

Then in the existing log block (line 49-50), drop the second `identityForLog` call and reuse the hoisted `actor, owner`.

> Gate on non-empty so anonymous callers don't get an empty `engram.owner`. Single-call refactor avoids calling `identityForLog` twice.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestInstrumentTools -v`
Expected: PASS (new test + existing instrument tests).

- [ ] **Step 5: gofmt + commit**

Run: `gofmt -l internal/server/instrument.go internal/server/instrument_test.go`
Expected: empty.

Commit: `feat(server): stamp engram.owner on tool spans`

---

### Task 6: Full OTel resource attributes

**Files:**

- Modify: `internal/telemetry/providers.go`
- Create: `internal/telemetry/providers_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/telemetry/providers_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"testing"
)

func TestBuildResourceHasStandardAttributes(t *testing.T) {
	res, err := buildResource(context.Background(), Config{ServiceName: "engram", ServiceVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("buildResource: %v", err)
	}
	got := map[string]string{}
	for _, kv := range res.Attributes() {
		got[string(kv.Key)] = kv.Value.Emit()
	}
	for _, key := range []string{
		"service.name", "service.version", "service.instance.id",
		"telemetry.sdk.name", "telemetry.sdk.language", "telemetry.sdk.version",
		"process.runtime.name", "os.type",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing resource attribute %q", key)
		}
	}
	if got["service.name"] != "engram" {
		t.Errorf("service.name = %q, want engram", got["service.name"])
	}
	if got["service.version"] != "1.2.3" {
		t.Errorf("service.version = %q, want 1.2.3", got["service.version"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telemetry/ -run TestBuildResource -v`
Expected: FAIL — `undefined: buildResource`.

- [ ] **Step 3: Extract `buildResource` with the full detector set**

In `internal/telemetry/providers.go`, replace the inline `resource.New(...)` block (lines 40-50) with a call to a new extracted function, and define that function:

```go
	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
```

Add the function (in the same file):

```go
// buildResource assembles the OTel resource from the full idiomatic detector
// set plus engram's service identity. resource.New is opt-in per detector and
// does NOT include resource.Default(), so every standard attribute group is
// requested explicitly. WithFromEnv is first so OTEL_RESOURCE_ATTRIBUTES /
// OTEL_SERVICE_NAME are honoured; WithAttributes is last so engram's
// service.name/version/instance.id win on conflict. WithAttributes is schemaless,
// so it never conflicts with the detectors' bundled semconv schema URL.
func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithFromEnv(),      // OTEL_RESOURCE_ATTRIBUTES + OTEL_SERVICE_NAME
		resource.WithTelemetrySDK(), // telemetry.sdk.name|language|version
		// WithProcess captures ALL of os.Args onto process.command_args, exported
		// on every signal. engram's flags are non-secret today; if a future flag
		// carries a token/key/secret path, swap this for WithProcessRuntimeName()
		// + WithProcessRuntimeVersion() (which omit command_args).
		resource.WithProcess(),   // process.pid, process.executable.*, process.runtime.*, process.owner, process.command_args
		resource.WithHost(),      // host.name
		resource.WithHostID(),    // host.id
		resource.WithOS(),        // os.type, os.description
		resource.WithContainer(), // container.id (docker/k8s cgroup)
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("service.instance.id", uuid.New().String()),
		),
	)
}
```

The imports (`resource`, `semconv`, `attribute`, `uuid`) are already present in `providers.go` — no import changes.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telemetry/ -run TestBuildResource -v`
Expected: PASS.

> If `resource.New` returns a non-nil error here (e.g. a schema-URL conflict), it surfaces as a test failure. Per ADR engram-uxh the production path already treats a resource error as a soft failure (returned up and degraded to stdout), so a conflict would be caught here, not in production.

- [ ] **Step 5: Build + full telemetry tests + gofmt + commit**

Run: `go build ./... && go test ./internal/telemetry/ -v && gofmt -l internal/telemetry/providers.go internal/telemetry/providers_test.go`
Expected: build OK, tests PASS, gofmt empty.

Commit: `feat(telemetry): full idiomatic OTel resource attribute set`

---

### Task 7: Helm chart knobs (sampler, interval, k8s resource attributes)

**Files:**

- Modify: `charts/engram/values.yaml`
- Modify: `charts/engram/templates/memory-mcp.yaml`

- [ ] **Step 1: Add values**

In `charts/engram/values.yaml`, under the `observability:` block, after `resourceAttributes: ""` (line 72) and before `log:` (line 73), add:

```yaml
  # Trace sampling. Empty => SDK default (parent-based always-on). Standard OTel
  # values: always_on | always_off | traceidratio | parentbased_traceidratio.
  # Only emitted when otlpEndpoint is set.
  traces:
    sampler: "" # OTEL_TRACES_SAMPLER
    samplerArg: "" # OTEL_TRACES_SAMPLER_ARG, e.g. "0.1" for 10% with traceidratio
  # Metric export cadence. Empty => SDK default (60000ms). Only emitted when
  # otlpEndpoint is set.
  metrics:
    exportInterval: "" # OTEL_METRIC_EXPORT_INTERVAL in milliseconds
```

- [ ] **Step 2: Add sampler/interval env templating**

In `charts/engram/templates/memory-mcp.yaml`, after the `OTEL_EXPORTER_OTLP_INSECURE` block (lines 66-68), before the resource-attributes block, add:

```yaml
            {{- if and .Values.observability.otlpEndpoint .Values.observability.traces.sampler }}
            - { name: OTEL_TRACES_SAMPLER, value: "{{ .Values.observability.traces.sampler }}" }
            {{- end }}
            {{- if and .Values.observability.otlpEndpoint .Values.observability.traces.samplerArg }}
            - { name: OTEL_TRACES_SAMPLER_ARG, value: "{{ .Values.observability.traces.samplerArg }}" }
            {{- end }}
            {{- if and .Values.observability.otlpEndpoint .Values.observability.metrics.exportInterval }}
            - { name: OTEL_METRIC_EXPORT_INTERVAL, value: "{{ .Values.observability.metrics.exportInterval }}" }
            {{- end }}
```

- [ ] **Step 3: Replace the `OTEL_RESOURCE_ATTRIBUTES` block with k8s Downward API**

REPLACE the existing block (lines 69-71):

```yaml
            {{- with .Values.observability.resourceAttributes }}
            - { name: OTEL_RESOURCE_ATTRIBUTES, value: "{{ . }}" }
            {{- end }}
```

with the Downward-API version (POD_* env MUST precede the `$(...)`-referencing `OTEL_RESOURCE_ATTRIBUTES`, since k8s expands `$(VAR)` using earlier-declared env entries):

```yaml
            # Pod identity via the Downward API, folded into OTEL_RESOURCE_ATTRIBUTES.
            # WithFromEnv() in the binary consumes these; same image works on a
            # laptop, in Docker, and in k8s (orchestrator supplies its own identity).
            - name: POD_NAME
              valueFrom: { fieldRef: { fieldPath: metadata.name } }
            - name: POD_NAMESPACE
              valueFrom: { fieldRef: { fieldPath: metadata.namespace } }
            - name: NODE_NAME
              valueFrom: { fieldRef: { fieldPath: spec.nodeName } }
            - name: POD_UID
              valueFrom: { fieldRef: { fieldPath: metadata.uid } }
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: "k8s.pod.name=$(POD_NAME),k8s.namespace.name=$(POD_NAMESPACE),k8s.node.name=$(NODE_NAME),k8s.pod.uid=$(POD_UID){{ with .Values.observability.resourceAttributes }},{{ . }}{{ end }}"
```

> CRITICAL (design Important finding): there must be exactly ONE `OTEL_RESOURCE_ATTRIBUTES` env entry. Two entries are last-wins in k8s and silently drop attributes. Deleting the old `{{- with }}` block and folding its value inline (the `{{ with … }},{{ . }}{{ end }}` suffix) satisfies this.

- [ ] **Step 4: Render and assert with helm template**

Run (default values — no otlpEndpoint, so sampler/interval absent but k8s attrs present):

```bash
helm template charts/engram | grep -A1 OTEL_RESOURCE_ATTRIBUTES
```

Expected: exactly one `OTEL_RESOURCE_ATTRIBUTES` entry whose value contains `k8s.pod.name=$(POD_NAME)`.

Run (with telemetry + sampler set):

```bash
helm template charts/engram \
  --set observability.otlpEndpoint=otel-collector:4317 \
  --set observability.traces.sampler=parentbased_traceidratio \
  --set observability.traces.samplerArg=0.1 \
  --set observability.metrics.exportInterval=30000 \
  --set observability.resourceAttributes=deployment.environment.name=prod \
  | grep -E 'OTEL_TRACES_SAMPLER|OTEL_METRIC_EXPORT_INTERVAL|OTEL_RESOURCE_ATTRIBUTES'
```

Expected: `OTEL_TRACES_SAMPLER=parentbased_traceidratio`, `OTEL_TRACES_SAMPLER_ARG=0.1`, `OTEL_METRIC_EXPORT_INTERVAL=30000`, and one `OTEL_RESOURCE_ATTRIBUTES` ending `...,deployment.environment.name=prod`.

- [ ] **Step 5: helm lint + commit**

Run: `helm lint charts/engram`
Expected: `0 chart(s) failed`.

Commit: `feat(chart): OTel sampler/interval knobs + k8s Downward-API resource attrs`

---

### Task 8: Full verification gate

**Files:** none (verification only)

- [ ] **Step 1: Full build + test**

Run: `go build ./... && go test ./...`
Expected: all packages PASS (store/server integration tests require Docker; if absent they SKIP — run on a Docker host or with `MEM_QDRANT_TEST_ADDR` set before claiming done).

- [ ] **Step 2: gofmt (CI trap — golangci does NOT catch this)**

Run: `gofmt -l $(git ls-files '*.go')`
Expected: empty output. The CI `test` job runs a standalone `gofmt -l` as its first step and fails before tests if any file is unformatted (memory: engram gofmt CI trap).

- [ ] **Step 3: Lint**

Run: `golangci-lint run ./...`
Expected: clean. (Run from the default checkout if in a secondary jj workspace — `actionlint`/`goreleaser check` need a real `.git`; pure `golangci-lint`/`go test` work in the workspace.)

- [ ] **Step 4: License headers**

Confirm every new `.go` file opens with the SPDX header (`// SPDX-License-Identifier: Apache-2.0` / `// Copyright 2026 Sean Brandt`). Run: `task license:check` (or `license-eye header check`).
Expected: pass.

- [ ] **Step 5: Final commit if anything was touched**

Commit any formatting/license fixups: `chore(telemetry): formatting + license headers`

---

## Self-Review notes (for the implementer)

- **Spec coverage:** Task 2 ↔ store spans/metrics; Task 3 ↔ embed; Task 4 ↔ auth; Task 1 ↔ domain metric instruments; Task 5 ↔ owner span identity; Task 6 ↔ full resource attributes; Task 7 ↔ chart sampler/interval + k8s resource knobs. The sampler/interval Go behavior needs NO code (SDK already reads the env vars — spec §5); only the chart exposes them.
- **slog rule:** no new bare `slog.Info` on the request path — the new auth code reuses `slog.WarnContext` (already context-aware). Store/embed add no logs (spans + metrics only), so the `*Context` rule is not at risk there.
- **Import direction:** `store`/`embed`/`auth` import `internal/telemetry`; verify `internal/telemetry` never imports them (it does not today) to keep the dependency acyclic.
- **Cardinality:** `engram.scope` is span-only, never a metric attribute. Metric attributes are bounded: `operation` (fixed method names), `outcome` (`ok`/`error`).
<!-- adr-capture: sha256=1ad7b43ad274d5e4; session=cli; ts=2026-06-11T18:27:19Z; adrs=engram-6gb,engram-wot,engram-7qd,engram-9tj -->
