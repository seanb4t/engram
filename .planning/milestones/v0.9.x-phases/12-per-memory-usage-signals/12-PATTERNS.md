# Phase 12: Per-Memory Usage Signals - Pattern Map

**Mapped:** 2026-07-10
**Files analyzed:** 11 (create/modify)
**Analogs found:** 11 / 11

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/store/store.go` (`Memory` struct, `payload()`, `fromPayload()`, new `IncrementAccess`) | model + service (CRUD) | CRUD | `store.go` `SetVisibility` (~1334) + `not_before`/`not_after` field wiring (~291-296, 372-380) | exact |
| `internal/store/store.go` (`Update`) | service | CRUD | existing `Update` (~1289-1324), same function, insertion point only | exact |
| `internal/store/store.go` (`Search`/`List`/`Get` span attrs) | service | request-response (tracing) | existing `engram.result_count` `span.SetAttributes` sites (~558, 640, 766, 993, 1047) | exact |
| `internal/server/usagequeue.go` (new, or reuse `summaryqueue.go`) | service (async worker) | event-driven | `internal/server/summaryqueue.go` (full file) | exact (minus retry/backoff) |
| `internal/server/tools.go` (`getMemory` ~908, `updateMemory` ~843) | controller (MCP tool handler) | request-response | same file, same functions — insertion points only | exact |
| `internal/server/connectapi.go` (`GetMemory` ~174, `memoryToProto` ~32) | controller (Connect RPC handler) | request-response | same file, same functions — insertion points only | exact |
| `internal/server/summary.go` (`recallView` ~40, `toRecallView` ~89) | transform (view-shaping) | transform | same struct/func — add two fields | exact |
| `internal/config/registry.go` (+1 row) | config | CRUD (config load) | `{Key: "summarize.on_write", ...}` (line 40) / `{Key: "ui.enabled", ...}` (line 51) | exact |
| `internal/config/validate.go` (+parseability check) | config | validation | `ENGRAM_SUMMARY_ON_WRITE` `strconv.ParseBool` check (~109-111) | exact |
| `internal/telemetry/metrics.go` (+usage-signal counters) | utility (telemetry) | event-driven | `SummaryQueueMetrics` struct + constructor + methods (~111-158) | exact |
| `proto/engram/v1/engram.proto` (`Memory` message +2 fields) + `gen/` regen | model (wire schema) | request-response | existing `Memory` message (lines 11-32), fields 1-18 | exact |
| `internal/store/rerank_test.go` (new negative-space test) | test | transform (pure fn) | `rerank.go` `RerankHits` (~73-100) — read-only, no modification needed to prove invariance | exact |

## Pattern Assignments

### `internal/store/store.go` — `Memory` struct + `payload()` / `fromPayload()` (D-03)

**Analog:** existing `NotBefore`/`NotAfter` field wiring in the same file.

**Struct field pattern** (`store.go:114-117`, insertion point — add after `NotAfter`):
```go
NotBefore *time.Time `json:"not_before,omitempty"`
NotAfter  *time.Time `json:"not_after,omitempty"`
// [NEW] AccessCount / LastAccessedAt (D-03) go here, e.g.:
// AccessCount    uint64    `json:"access_count"`
// LastAccessedAt time.Time `json:"last_accessed_at,omitempty"`
```

**`payload()` write pattern** (`store.go:271-320`, the general shape; `not_before`/`not_after` show the "conditionally write" idiom at 291-296):
```go
p := map[string]any{
    "content": m.Content, "scope": m.Scope, /* ... */
    "created_at": m.CreatedAt.Format(time.RFC3339),
}
if m.NotBefore != nil {
    p["not_before"] = m.NotBefore.Unix()
}
if m.NotAfter != nil {
    p["not_after"] = m.NotAfter.Unix()
}
// [NEW] unconditional int write (uint64 auto-converts to Value_IntegerValue):
// p["access_count"] = m.AccessCount
// if !m.LastAccessedAt.IsZero() {
//     p["last_accessed_at"] = m.LastAccessedAt.Format(time.RFC3339)
// }
```

**`fromPayload()` read pattern** (`store.go:372-380`, `not_before`/`not_after` reading via `GetIntegerValue()`):
```go
if v, ok := p["not_before"]; ok {
    // ... sec := v.GetIntegerValue(); t := time.Unix(sec, 0); m.NotBefore = &t
}
if v, ok := p["not_after"]; ok {
    // ... same shape
}
// [NEW]
// if v, ok := p["access_count"]; ok {
//     m.AccessCount = uint64(v.GetIntegerValue())
// }
// if v, ok := p["last_accessed_at"]; ok {
//     if t, err := time.Parse(time.RFC3339, v.GetStringValue()); err == nil {
//         m.LastAccessedAt = t
//     }
// }
```
Missing key → zero value automatically (legacy records need no backfill, per D-03).

---

### `internal/store/store.go` — new `IncrementAccess(ctx, id)` (D-01, D-04, D-05)

**Analog:** `SetVisibility` (`store.go:1334-1360`) — the canonical partial-payload `SetPayload` write.

```go
// Source: internal/store/store.go:1334-1360 (SetVisibility, full function — copy the shape)
func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) (err error) {
    ctx, span := tracer.Start(ctx, "store.SetVisibility",
        trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
    defer span.End()
    start := time.Now()
    defer func() {
        telemetry.RecordStoreOp(ctx, "SetVisibility", start, err)
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        }
    }()

    if _, err := s.getWritable(ctx, id, subj); err != nil {
        return err
    }
    vis := ""
    if shared {
        vis = visibilityShared
    }
    _, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
        CollectionName: s.collection, Wait: qdrant.PtrOf(true),
        Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
        PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
    })
    return err
}
```

**Divergence for `IncrementAccess`:**
- Do **NOT** call `s.getWritable` (no re-authorization — caller already gated at the handler boundary per D-01/D-04). Instead call the internal `s.Get(ctx, id)` (or equivalent) purely to read the current `access_count` value for the RMW.
- `SetPayload` map carries two keys: `{"access_count": cur+1, "last_accessed_at": s.now().Format(time.RFC3339)}`.
- RMW is lost-update-tolerant by design (D-05) — no mutex, no retry, no optimistic-concurrency check.
- Use `s.now()` (the injectable clock field, `store.go:178-183`, `WithClock`) rather than `time.Now()` directly, matching how other store methods source time.

---

### `internal/store/store.go` — `Update` free bump (D-04)

**Analog:** existing `Update` insertion point (`store.go:~1289-1324`, illustrative excerpt from RESEARCH.md):
```go
// Source: internal/store/store.go:~1301 (existing Update, insertion point)
cur.Content = content
// [NEW] D-04: piggyback the update-path bump here, before Upsert:
cur.AccessCount++
cur.LastAccessedAt = s.now()
if shared != nil {
    // ... existing logic unchanged
}
```
Zero extra store call — rides the update's own re-embed + `Upsert`.

---

### `internal/store/store.go` — recall-span attributes (D-06)

**Analog:** existing `engram.result_count` attribute sites — `store.go:558` (`Search`), `766`/`993` (`List`, two return paths), `1047` (`Get`).

```go
// Source: internal/store/store.go:558 (Search span defer, illustrative insertion point)
} else {
    span.SetAttributes(attribute.Int("engram.result_count", len(out)))
    // [NEW] D-06:
    // ids := recallIDs(out, recallIDCap)
    // span.SetAttributes(
    //     attribute.StringSlice("engram.recall.ids", ids),
    //     attribute.Int("engram.recall.count", len(out)),
    // )
}
```
Apply the same addition at each of the four other `result_count` sites in `store.go` for `List` (two return paths) and `Get` (single-record: `ids = [id]`, `count = 1`). `attribute.StringSlice(k string, v []string) KeyValue` is the verified `go.opentelemetry.io/otel/attribute` signature (RESEARCH.md, confirmed via `go doc`).

---

### `internal/server/usagequeue.go` (new) — async get-path incrementer (D-10)

**Analog:** `internal/server/summaryqueue.go` (full file, read above) — reuse the shutdown-safety kernel:
- `ch chan string`, `mu sync.RWMutex` + `closed bool` (CR-01 guard), `wg sync.WaitGroup` (worker lifecycle), `inFlight sync.WaitGroup` (drain seam)
- `tryEnqueue(id string)` — **exact copy** of the RLock-guarded reserve-before-send pattern (`summaryqueue.go:148-181`):
```go
func (q *summaryQueue) tryEnqueue(id string) {
    if q == nil {
        return
    }
    q.mu.RLock()
    defer q.mu.RUnlock()
    if q.closed {
        if q.metrics != nil {
            q.metrics.Dropped(context.Background())
        }
        return
    }
    q.inFlight.Add(1)
    select {
    case q.ch <- id:
        if q.metrics != nil {
            q.metrics.Enqueued(context.Background())
        }
    default:
        q.inFlight.Done()
        if q.metrics != nil {
            q.metrics.Dropped(context.Background())
        }
    }
}
```
- `Start(ctx)` / `worker(ctx)` / `Shutdown(ctx)` / `Wait()` — copy verbatim shape (`summaryqueue.go:186-325`).
- **Drop, do not copy:** `backoff.Retry`, `summaryQueueMaxTries`/`summaryQueueMaxInterval`, `maxElapsed`, `newRetryBackOff`, `Retried`/`Failed` retry semantics — D-10 mandates single-attempt, no retry. `process(ctx, id)` becomes:
```go
func (q *usageQueue) process(ctx context.Context, id string) {
    defer q.itemDone()
    defer func() {
        if r := recover(); r != nil {
            // log + metrics.Failed
        }
    }()
    if err := q.increment(ctx, id); err != nil {
        // log + metrics.Failed — no retry loop
    }
}
```
Wire lifecycle into `serve.go` mirroring the Phase 11 `Register` → shutdown-func pattern (grep `summaryQueue.Start`/`Shutdown` call sites in `serve.go` for the exact wiring to mirror).

---

### `internal/server/tools.go` — `getMemory` / `updateMemory` handlers (D-01)

**Analog:** same functions, same file — insertion points only, no structural change.

```go
// Source: internal/server/tools.go getMemory (~908-922)
func (d *deps) getMemory(ctx context.Context, a idArgs) (store.Memory, error) {
    subj, err := subjectFromContext(ctx)
    if err != nil {
        return store.Memory{}, err
    }
    pid, err := d.st.ResolvePointID(ctx, a.ID)
    if err != nil {
        return store.Memory{}, err
    }
    m, err := d.st.GetReadable(ctx, pid, subj)
    if errors.Is(err, store.ErrNotFound) {
        return store.Memory{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
    }
    // [NEW] D-01/D-04: only on success, after the gate, fire-and-forget:
    // if err == nil {
    //     d.usageQueue.tryEnqueue(pid)
    // }
    return m, err
}
```
Call `tryEnqueue` **and ignore its outcome** — never check/propagate an error from it (mirrors the existing `summaryQueue.tryEnqueue(id)` call-and-ignore precedent used elsewhere in `tools.go`).

`updateMemory` (~843-870): the bump is already free inside `store.Update` (see above) — no separate `tryEnqueue` call needed here; D-04 says the update path piggybacks on the existing `Upsert`, not a second async path.

---

### `internal/server/connectapi.go` — `GetMemory` (D-01) + `memoryToProto` (D-07)

**Analog:** same file, same functions.

```go
// Source: internal/server/connectapi.go:174-201 (GetMemory, insertion point after GetReadable succeeds)
m, err := a.d.st.GetReadable(ctx, pid, subj)
if err != nil {
    // ... existing error handling unchanged
}
// [NEW] D-01: same call-and-ignore pattern as the MCP getMemory handler
// a.d.usageQueue.tryEnqueue(pid)
return connect.NewResponse(&engramv1.GetMemoryResponse{Memory: memoryToProto(m)}), nil
```

```go
// Source: internal/server/connectapi.go:32-44 (memoryToProto, full function — add 2 fields)
func memoryToProto(m store.Memory) *engramv1.Memory {
    return &engramv1.Memory{
        Id: m.ID, Content: m.Content, Scope: m.Scope,
        Repo: m.Repo, Workspace: m.Workspace, Worktree: m.Worktree, BaseDir: m.BaseDir,
        Source: m.Source, Category: m.Category, Tags: m.Tags,
        Actor: m.Actor, Owner: m.Owner, Visibility: m.Visibility,
        CreatedAt:     timestamppb.New(m.CreatedAt),
        Summary:       m.Summary,
        SummarySource: string(m.SummarySource),
        Score:         m.Score,
        ShortId:       m.ShortID,
        // [NEW] AccessCount: m.AccessCount, LastAccessedAt: timestamppb.New(m.LastAccessedAt),
    }
}
```

`SearchMemories` (~144) / `ListMemories` (~95) get **no** `tryEnqueue` call (D-02 hard invariant — never count result-set membership).

---

### `internal/server/summary.go` — `recallView` / `toRecallView` (D-07, Pitfall 2 — MANDATORY)

**Analog:** same struct/function — this is the hand-written allow-list RESEARCH flagged; adding fields to `store.Memory` alone does NOT surface them here.

```go
// Source: internal/server/summary.go:39-53 (recallView struct, full — add 2 fields)
type recallView struct {
    ID            string    `json:"id"`
    ShortID       string    `json:"short_id,omitempty"`
    Summary       string    `json:"summary"`
    SummarySource string    `json:"summary_source,omitempty"`
    Truncated     bool      `json:"truncated,omitempty"`
    Scope         string    `json:"scope"`
    Category      string    `json:"category"`
    Tags          []string  `json:"tags,omitempty"`
    CreatedAt     time.Time `json:"created_at"`
    Score         float32   `json:"score,omitempty"`
    // [NEW] AccessCount uint64 `json:"access_count"`
    // [NEW] LastAccessedAt time.Time `json:"last_accessed_at,omitempty"`
}

// Source: internal/server/summary.go:89-96 (toRecallView, full — populate 2 fields)
func toRecallView(m store.Memory, maxChars int) recallView {
    summary, truncated := summaryOrTruncation(m, maxChars)
    return recallView{
        ID: m.ID, ShortID: m.ShortID, Summary: summary, SummarySource: string(m.SummarySource), Truncated: truncated,
        Scope: m.Scope, Category: m.Category, Tags: m.Tags, CreatedAt: m.CreatedAt,
        Score: m.Score,
        // [NEW] AccessCount: m.AccessCount, LastAccessedAt: m.LastAccessedAt,
    }
}
```

`get_memory` needs **no** change here — it returns the raw `store.Memory` directly (`tools.go:1036-1037` per RESEARCH), which already carries the new fields once `Memory`/`payload`/`fromPayload` are updated.

---

### `internal/config/registry.go` + `validate.go` — `ENGRAM_USAGE_SIGNALS` (D-09)

**Analog:** `summarize.on_write` row (`registry.go:40`) and its `validate.go:109-111` parseability check.

```go
// Source: internal/config/registry.go:40 (existing row — copy shape, note different default)
{Key: "summarize.on_write", Env: "ENGRAM_SUMMARY_ON_WRITE", Default: "false"},
// [NEW]
// {Key: "usage.signals", Env: "ENGRAM_USAGE_SIGNALS", Default: "true"},
```

```go
// Source: internal/config/validate.go:108-111 (existing unconditional check — copy shape)
if _, err := strconv.ParseBool(c.Summarize.OnWrite); err != nil {
    errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_ON_WRITE %q: must be a boolean: %w", c.Summarize.OnWrite, err))
}
// [NEW]
// if _, err := strconv.ParseBool(c.Usage.Signals); err != nil {
//     errs = append(errs, fmt.Errorf("ENGRAM_USAGE_SIGNALS %q: must be a boolean: %w", c.Usage.Signals, err))
// }
```
Parsed at point-of-use in `buildDepsFromEnv` (mirror `ui.enabled`/`summarize.on_write` — never a native bool at `Load()` time).

---

### `internal/telemetry/metrics.go` — usage-signal counters

**Analog:** `SummaryQueueMetrics` (`metrics.go:111-158`, full struct + constructor + two methods shown — mirror `enqueued`/`dropped`/`failed`, drop `retried`/`fillDur` unless a latency histogram is wanted).

```go
// Source: internal/telemetry/metrics.go:117-140 (SummaryQueueMetrics struct + constructor)
type SummaryQueueMetrics struct {
    enqueued metric.Int64Counter
    dropped  metric.Int64Counter
    failed   metric.Int64Counter
    retried  metric.Int64Counter
    fillDur  metric.Float64Histogram
}

func NewSummaryQueueMetrics(m metric.Meter) *SummaryQueueMetrics {
    enqueued, _ := m.Int64Counter("engram.summary_queue.enqueued")
    dropped, _ := m.Int64Counter("engram.summary_queue.dropped")
    failed, _ := m.Int64Counter("engram.summary_queue.failed")
    retried, _ := m.Int64Counter("engram.summary_queue.retried")
    fillDur, _ := m.Float64Histogram("engram.summary_queue.fill.duration",
        metric.WithUnit("s"), metric.WithDescription("async summary fill latency"))
    return &SummaryQueueMetrics{enqueued: enqueued, dropped: dropped, failed: failed, retried: retried, fillDur: fillDur}
}

func (s *SummaryQueueMetrics) Enqueued(ctx context.Context) { s.enqueued.Add(ctx, 1) }
func (s *SummaryQueueMetrics) Dropped(ctx context.Context)  { s.dropped.Add(ctx, 1) }
```
New `UsageQueueMetrics` (or similar) follows the identical shape with instrument names `engram.usage_queue.enqueued` / `.dropped` / `.failed` — no retry counter needed (D-10: no retry).

---

### `proto/engram/v1/engram.proto` — `Memory` message +2 fields (D-07, Pitfall 3)

**Analog:** existing `Memory` message, fields 1-18.

```protobuf
// Source: proto/engram/v1/engram.proto:11-32 (Memory message, full — next fields are 19/20)
message Memory {
  string id = 1;
  string content = 2;
  // ... fields 3-17 unchanged ...
  string short_id = 18;
  // [NEW] additive, non-breaking per buf breaking:
  // uint64 access_count = 19;
  // google.protobuf.Timestamp last_accessed_at = 20;
}
```
**MUST** run `task proto:gen` and commit the resulting `gen/go/`/`gen/ts/` diff — CI's `buf` job (`.github/workflows/ci.yaml:107-123`) does `git diff --exit-code -- gen/` and fails on drift even though `buf breaking` itself passes (additive fields are non-breaking). Sequence this as a dedicated task before the `memoryToProto` edit that references `engramv1.Memory.AccessCount`.

---

### `internal/store/rerank_test.go` — negative-space invariance test (D-08)

**Analog:** `rerank.go` `RerankHits` (`~73-100`, full function, read-only — confirms it is a pure function over `Memory` fields `Score`/`ID` only, never `AccessCount`).

```go
// Source: internal/store/rerank.go:73-100 (RerankHits, full — confirms no AccessCount read today)
func RerankHits(query string, hits []Memory, k int) []Memory {
    queryTerms := tokenize(query)
    type scored struct {
        m       Memory
        overlap int
    }
    ranked := make([]scored, len(hits))
    for i, h := range hits {
        ranked[i] = scored{m: h, overlap: lexicalOverlap(queryTerms, h)}
    }
    sort.SliceStable(ranked, func(i, j int) bool {
        if ranked[i].overlap != ranked[j].overlap {
            return ranked[i].overlap > ranked[j].overlap
        }
        if ranked[i].m.Score != ranked[j].m.Score {
            return ranked[i].m.Score > ranked[j].m.Score
        }
        return ranked[i].m.ID < ranked[j].m.ID
    })
    // ...
}
```
New test: construct two `Memory` records identical in every field except `AccessCount` (one 0, one 1000000), assert `RerankHits` output order is unchanged (D-08 hard invariant). This is a pure unit test — no Qdrant/testcontainers dependency, follows existing `rerank_test.go` conventions in the same package.

## Shared Patterns

### Store-method telemetry wrapper (tracer + RecordStoreOp defer)
**Source:** `internal/store/store.go:1335-1345` (`SetVisibility`'s preamble — identical shape on every store method)
**Apply to:** `IncrementAccess` and any store-layer span-attribute edits (`Search`/`List`/`Get`)
```go
ctx, span := tracer.Start(ctx, "store.IncrementAccess",
    trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
defer span.End()
start := time.Now()
defer func() {
    telemetry.RecordStoreOp(ctx, "IncrementAccess", start, err)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
}()
```

### Bounded async worker-pool shutdown-safety kernel (CR-01)
**Source:** `internal/server/summaryqueue.go` (full file — `mu`/`closed`/`inFlight`/`tryEnqueue`/`Start`/`worker`/`Shutdown`/`Wait`)
**Apply to:** `internal/server/usagequeue.go` (new file). MUST reuse this exact RWMutex-guard-before-send pattern (D-10) — this bug class (send-on-closed-channel panic) was already found and fixed once (CR-01) in this codebase.

### koanf field registry + point-of-use bool parsing
**Source:** `internal/config/registry.go:40,51` (`summarize.on_write`, `ui.enabled` rows) + `internal/config/validate.go:108-111` (`strconv.ParseBool` check) + `buildSummaryQueue`'s `onWrite, err := strconv.ParseBool(cfg.Summarize.OnWrite)` idiom (referenced in RESEARCH.md, `tools.go` `buildDepsFromEnv`/`buildSummaryQueue` ~159-224)
**Apply to:** `ENGRAM_USAGE_SIGNALS` gate — registry row, validate.go check, and point-of-use parse in `buildDepsFromEnv` (never a native bool at `Load()` time).

### Call-and-ignore async enqueue at handler boundary
**Source:** the existing `summaryQueue.tryEnqueue(id)` call sites in `tools.go` (write-path handlers, per RESEARCH.md Open Question 4)
**Apply to:** `getMemory` (`tools.go`) and Connect `GetMemory` (`connectapi.go`) — call `usageQueue.tryEnqueue(pid)` and never check/propagate its return; D-04 requires the read path stay fully decoupled from the counter write's success/failure.

## No Analog Found

None — every file in the change set has a direct, already-shipped precedent in this codebase (RESEARCH.md's core finding: this phase is "find and copy the precedent," not "design something new").

## Metadata

**Analog search scope:** `internal/store/`, `internal/server/`, `internal/config/`, `internal/telemetry/`, `proto/engram/v1/`
**Files scanned:** `store.go`, `summaryqueue.go`, `tools.go`, `connectapi.go`, `summary.go`, `rerank.go`, `registry.go`, `validate.go`, `metrics.go`, `engram.proto`
**Pattern extraction date:** 2026-07-10
