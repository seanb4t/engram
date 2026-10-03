# Proposal

## Why

`search_memory` is the slow path in production: p50 1.03 s, p95 2.9 s, max 3.9 s (#698). The tail comes from Qdrant vector queries that wait on disk. The raw vectors are memory-mapped on Longhorn storage. Under node memory pressure the kernel evicts those pages, and the next query re-reads them one major page fault at a time (~1,300 faults per cold query, ~2 ms each). Qdrant 1.19 cannot pin dense vectors in RAM, but it can pin an int8 quantized copy. In a local prototype at production shape, that cut cold-query faults 4–7x.

Startup also has a latent failure that this change touches directly (#683). One 15 s deadline covers connecting to Qdrant, creating the collection, and building every payload index. The HTTP listener opens only after all of that. So on a collection large enough that index builds outlast the deadline, or the chart's liveness probe (~40–55 s), the server crash-loops. Adding a quantization step to the same path makes this the right moment to give schema setup its own budget.

## What Changes

- engram reconciles the memory collection's vector quantization against a new setting, `ENGRAM_QDRANT_QUANTIZATION`:
  - `int8` (default): int8 scalar quantization, quantile 0.99, quantized vectors pinned in RAM, rescoring left at Qdrant's default. Applied to new collections at creation and to existing ones with an idempotent update.
  - `off`: actively removes quantization from an existing collection.
  - `unmanaged`: engram never reads or changes the collection's quantization config.
- Only `engram serve` reconciles quantization. CLI verbs that open the store treat it as `unmanaged`, so an operator's shell environment cannot flip the server's setting. `engram reindex` applies the configured mode when it creates its target collection.
- Startup splits into two budgets. Connecting and creating the collection keep the existing 15 s. Schema convergence (the quantization reconcile and payload index builds) gets its own deadline, `ENGRAM_QDRANT_SCHEMA_TIMEOUT`. All of it still completes before the server starts listening.
- Each payload index build is logged with its duration.
- The Helm chart exposes `memory.qdrant.quantization` and `memory.qdrant.schemaTimeoutSeconds`, and adds a `startupProbe` whose window the template computes from the schema timeout.
- On the first boot after upgrade, Qdrant re-optimizes the collection to build the quantized copy. This happens in the background (under a second at production scale) and needs no migration.

## Capabilities

### New Capabilities

- `collection-provisioning`: how engram creates and reconciles its Qdrant collections at startup and for reindex targets. Covers vector parameters, payload indexes, vector quantization, which entry points may reconcile which settings, and the time budgets that bound startup.

### Modified Capabilities

None. No specs exist yet. This is the project's first OpenSpec capability.

## Impact

- **Code:** `internal/store` (`EnsureCollection` / `ensureCollection` / `ensureIndexes`, reindex target creation), `internal/config` (two new keys in the registry, validation), `internal/server` (`ensureStoreFromConfig` deadlines; `StoreFromEnv` passes `unmanaged`), `cmd/engram/serve.go`.
- **Chart:** `charts/engram` values, `_helpers.tpl` env wiring, `memory-mcp.yaml` startupProbe.
- **Docs:** `guides/configure.md` (two new variables), `guides/upgrade.md` (first-boot re-optimization), `guides/deploy.md` (startupProbe).
- **Memory:** about 3 KB of Qdrant heap per vector (~12 MB at today's 4,200 records).
- **Recall quality:** with rescoring on, the first pass is approximate but final scores are exact. `task eval:retrieval` must show no recall@k / MRR regression before shipping.
- **Dependencies:** none new. `github.com/qdrant/go-client` v1.19.3 already has the quantization API.
- **Out of scope:** page faults from the appendable segment and the payload and ID files, cluster memory QoS (selfhosted-cluster #2609), and the second search per `search_memory` call (#684).
