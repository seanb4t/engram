# Design

## Context

See proposal.md for motivation and specs/collection-provisioning/spec.md for the behavior contract.

Current state:

- `Store.EnsureCollection(ctx, dim)` calls `ensureCollection(ctx, name, dim)`. That runs `CollectionExists`, then `CreateCollection` with only vector size and distance, then `ensureIndexes`. `ensureIndexes` calls `CreateFieldIndex` 11 times in sequence with `Wait: true` and treats AlreadyExists as success.
- `ensureCollection` has three callers: startup (`server.ensureStoreFromConfig`), the reindex target (`store.go` reindex path), and the retrieval eval (`retrievaleval`, via `EnsureCollection`).
- `ensureStoreFromConfig` wraps the whole of `EnsureCollection` in one 15 s context. It serves both `engram serve` (through `buildDepsFromEnv` and `StoreAndDeciderFromEnv`) and about 15 CLI verbs (through `StoreFromEnv`).
- `serve.go` calls `ListenAndServe` only after `server.Register` returns, so the port stays closed until provisioning finishes. The chart's liveness and readiness probes are `tcpSocket` with no startup probe. Liveness starts after 10 s, runs every 15 s, and fails after 3 misses.
- Measured on local Qdrant v1.19.1 at production shape (engram record `k7cv7yxj9h`): an update that sets quantization returns in about 5 ms, even mid-optimization. Re-optimization runs in the background (yellow, then green in about 0.5–0.8 s). An identical re-update triggers nothing. `Disabled` removes quantization the same way.

## Goals / Non-Goals

**Goals:**

- One place decides a collection's quantization: the store's provisioning path. Every caller states its intent explicitly and none infers it.
- Startup failures name the step and budget that failed.
- The chart's startup window and the server's schema deadline come from one value.

**Non-Goals:**

- No change to query construction: no `SearchParams`, oversampling, or rescore overrides.
- No background or deferred index builds. #683's options A and C were rejected in favor of keeping "provisioned before listening".
- No quantization for named or sparse vectors. engram uses one unnamed dense vector.
- No change to how the vector size of an existing collection is handled.

## Decisions

### D1. Split provisioning into two store calls

A new `CreateCollectionIfAbsent(ctx, dim)` does exists-then-create and nothing else. A new `EnsureSchema(ctx)` runs the quantization reconcile, then `ensureIndexes`. `EnsureCollection(ctx, dim)` keeps its name and becomes the helper that runs both under one context, so its ~60 existing call sites (tests, the eval, e2e) are unchanged. `ensureStoreFromConfig` calls the two steps with their own contexts: 15 s for connecting and `CreateCollectionIfAbsent`, then `ENGRAM_QDRANT_SCHEMA_TIMEOUT` for `EnsureSchema`. Errors are wrapped with the step and budget, for example `schema provisioning (ENGRAM_QDRANT_SCHEMA_TIMEOUT=2m0s): ensure index "tags": context deadline exceeded`.

- *Alternative:* keep one call and pass two contexts. Rejected because it hides two budgets behind one method, and callers such as the eval and tests want the composed behavior anyway, which `EnsureCollection` keeps giving them.
- *Why order quantization before indexes:* the reconcile is cheap and fails loudly on a bad config, such as Qdrant rejecting a setting. Index builds are the slow part, so a config error should not wait behind them.

### D2. Quantization mode is a store option, set only by the server

`store.WithQuantization(mode)` sets the mode, where mode is `int8`, `off`, or `unmanaged`. The zero value is `unmanaged`. `buildDepsFromEnv` (the `serve` path) passes the configured mode. `StoreFromEnv` and the other CLI constructors pass nothing, so CLI verbs never change quantization (spec: "Only the server reconciles quantization").

- *Alternative:* a per-call parameter on `EnsureSchema`. Rejected because the store also needs the mode at create time inside `CreateCollectionIfAbsent`. One option, set once, keeps the two calls consistent.
- `StoreAndDeciderFromEnv` is also `serve`-adjacent wiring. Its callers must be checked when implementing, so that only `engram serve` sets the mode.

### D3. Create-time quantization versus reconcile

When `CreateCollectionIfAbsent` creates a collection and the mode is `int8`, the `CreateCollection` request carries the quantization config. This avoids a create-then-update round trip. In `off` or `unmanaged` mode the create carries none. `EnsureSchema` then reconciles:

| Mode | Current config | Action |
|---|---|---|
| `int8` | equal to target | none |
| `int8` | absent or different | `UpdateCollection` with the int8 diff |
| `off` | absent | none |
| `off` | present | `UpdateCollection` with the `Disabled` diff |
| `unmanaged` | any | none: no quantization update is ever sent |

The target is `ScalarQuantization{Type: Int8, Quantile: 0.99, Memory: Pinned}`. Equality compares only those three fields, read from `GetCollectionInfo().Config.QuantizationConfig`. The server may echo the deprecated `always_ram` field, so a whole-message `proto.Equal` would report false drift and re-send the update on every boot.

When a different scalar config or another quantization type (binary, product, turbo) is found under `int8` mode, engram overwrites it. `int8` means engram owns the setting; `unmanaged` is the opt-out for hand tuning.

### D4. Reindex target

`ReindexOptions` gains the configured mode, which the `reindex` command fills from config. When the target does not exist, it is created with that mode (D3, create-time path). An existing target is never reconciled. The store used by the reindex CLI itself stays `unmanaged` (D2).

### D5. Config surface

- `qdrant.quantization`: env `ENGRAM_QDRANT_QUANTIZATION`, default `int8`, validated against the three values in `Config.Validate`.
- `qdrant.schema_timeout`: env `ENGRAM_QDRANT_SCHEMA_TIMEOUT`, default `2m`. It uses the same Go-duration parsing and "positive" validation as `embed.timeout`.
- Both are added to `internal/config/registry.go`, the registry the docs-site configure reference derives from.

### D6. Chart

- Values `memory.qdrant.quantization: int8` and `memory.qdrant.schemaTimeoutSeconds: 120`. The value is an integer of seconds because Helm cannot do arithmetic on Go duration strings.
- `_helpers.tpl` emits `ENGRAM_QDRANT_QUANTIZATION` and `ENGRAM_QDRANT_SCHEMA_TIMEOUT: "<n>s"`.
- `memory-mcp.yaml` adds `startupProbe: tcpSocket http, periodSeconds: 5, failureThreshold: ceil((15 + n + 15) / 5)`. The extra 15 s covers process start and config load. Kubernetes holds liveness and readiness until the startup probe succeeds, so their current settings stay as they are.

### D7. Logging

`ensureIndexes` reads the collection's payload schema once, still calls `CreateFieldIndex` for every index as before, and logs each one with `field`, `built` and `duration`: at Info when the field was absent beforehand (a real build), at Debug when it already existed. Logging every index at Info was rejected because CLI verbs run on Go's default slog handler, which prints Info to stderr, so every `engram prune-expired` would print 11 lines. A quantization update logs the mode and the previous config at Info; a no-op logs at Debug.

### D8. Testing

These tests cover our own logic against the pinned Qdrant testcontainer, under rules `m45p2b4bp7` and `3p0zsqrhmb`:

- `int8` creates the collection quantized.
- `int8` updates an existing unquantized collection.
- `int8` replaces a different scalar config.
- A second `EnsureSchema` sends no update. This uses an interceptor or a counting client seam, not a sleep.
- `off` removes quantization.
- `unmanaged` leaves a hand-set config unchanged.
- The CLI constructor's store does not touch quantization.
- A reindex fresh target is created quantized, and an existing target is left alone.
- Config validation rejects bad values for both keys.
- The schema deadline error names the step. A fault-injecting interceptor drives this, the same pattern as `migrate_faultinject_test.go`.
- A chart render test (helm template) checks the env values and the startup probe arithmetic.

There is no test of page faults or Qdrant's re-optimization timing, since that is Qdrant's behavior.

## Risks / Trade-offs

- **The retrieval eval may never exercise quantized segments.** Qdrant builds the quantized copy only for optimized (indexed) segments, and the issue's own prototype found the appendable segment unquantized. The eval's fixture collections are small enough to sit entirely in an appendable segment, so an `int8`-versus-`off` comparison could be vacuous. → The eval's quantized run must force optimization of its collections (a low `indexing_threshold` on the eval collection, then wait for status green) and confirm quantization took effect before querying. A top-k overlap check against a restored snapshot of production is the stronger pre-ship signal, and is recommended as a manual step.
- **Recall loss from the approximate first pass.** Rescoring keeps the final scores exact, but candidate selection uses int8 distances, so a near-boundary true neighbour can drop out of the top k. → This is the pre-ship eval gate (proposal Impact). If it regresses, the fallback is setting oversampling at query time, which would be a follow-up change, not part of this one.
- **Two servers with different modes would flip-flop the collection.** → An operator error, outside the single-replica chart's normal state. The quantization update log line makes it visible.
- **A long `schemaTimeoutSeconds` delays crash-loop detection.** The startup probe waits the whole window before declaring failure. → The default of 120 s is generous against today's sub-second builds while bounding the delay.
- **About 3 KB of Qdrant heap per record.** → About 12 MB today. Covered by Qdrant's 2Gi limit in chart values.
- **Old binary after rollback.** A pre-change engram does not read quantization, so the collection stays quantized. → Harmless: queries work the same. To remove it, run the new binary with `off`, or remove it by hand.

## Migration Plan

1. Release as usual (release-please). No data migration and no `engram migrate` step.
2. On the first `serve` boot with the default `int8`, the collection gains quantization. Qdrant re-optimizes in the background in under a second at production scale, and queries keep working throughout.
3. After deploy, check the Qdrant slow-request log for `query` entries, `store.Search` p95 (target under 200 ms), and major page faults per hour (if selfhosted-cluster #2516 is in place).
4. Rollback: set `memory.qdrant.quantization: off` and restart, which removes quantization. Or roll back the image and leave quantization in place, which is harmless.

## Open Questions

- The startup-probe margin (15 s for process start and config load) is an estimate. Confirm it against a cold pod start on the cluster; it changes only a chart constant.
