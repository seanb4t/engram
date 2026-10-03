# Tasks

## 1. Config surface

- [ ] 1.1 Add `qdrant.quantization` (`ENGRAM_QDRANT_QUANTIZATION`, default `int8`) and `qdrant.schema_timeout` (`ENGRAM_QDRANT_SCHEMA_TIMEOUT`, default `2m`) to `internal/config/registry.go` and `QdrantConfig`. Verify with a config test that the defaults resolve when unset.
- [ ] 1.2 Validate both keys in `Config.Validate`: the mode must be one of `int8`/`off`/`unmanaged`, and the timeout must be a positive Go duration. Each error names its env var. Verify with table tests covering an invalid mode, zero, negative, and an unparseable timeout.
- [ ] 1.3 Document both variables in `docs-site` `guides/configure.md`, including what each mode does and that only `engram serve` applies the mode. Verify that `task lint` is clean and the docs-site config-reference check (if it derives from the registry) passes.

## 2. Store: split provisioning and add the quantization option

- [ ] 2.1 Add the `store.WithQuantization(mode)` option, with `unmanaged` as the zero value. Split `EnsureCollection` (exists, then create) from a new `EnsureSchema` (quantization reconcile, then `ensureIndexes`), and add a small helper that runs both for callers that want the composed behavior. Verify that existing `internal/store` tests pass unchanged with the helper swapped in where they call `EnsureCollection`.
- [ ] 2.2 Create-time quantization: in `int8` mode, `CreateCollection` carries `ScalarQuantization{Int8, 0.99, Pinned}`; `off` and `unmanaged` carry none. Verify with testcontainer tests: an `int8` create reads back that config, and an `off` create reads back none.
- [ ] 2.3 Reconcile in `EnsureSchema` per design D3: compare only type, quantile and memory from `GetCollectionInfo`, update when they differ, send the `Disabled` diff under `off`, and never read under `unmanaged`. Verify with testcontainer tests for: `int8` on an unquantized collection, `int8` replacing quantile 0.95, `off` removing quantization, `unmanaged` leaving a hand-set config unchanged, and a second `EnsureSchema` sending no `UpdateCollection` (counted by a gRPC interceptor).
- [ ] 2.4 Log one Info entry per payload index with `field` and `duration`, and log quantization updates with the mode and the before and after config (Debug for a no-op). Verify with a test that captures slog output and asserts one index entry per field.
- [ ] 2.5 Confirm that stored records are untouched by a quantization change. Verify with a testcontainer test: seed records, switch `off` to `int8` to `off`, then `Get` every record and compare content and payload byte-for-byte.

## 3. Reindex target

- [ ] 3.1 Add the configured mode to `ReindexOptions` and fill it from config in the `reindex` command. Create a missing target with that mode, and never reconcile an existing target. Verify with testcontainer tests: a fresh target under `int8` is quantized, and a pre-existing unquantized target stays unquantized after a resumed reindex.
- [ ] 3.2 Note in `guides/reindex.md` that the target is created with the configured quantization mode. Verify that `task lint` is clean.

## 4. Server startup budgets

- [ ] 4.1 Split `ensureStoreFromConfig` into two contexts: 15 s for connecting and `EnsureCollection`, then `qdrant.schema_timeout` for `EnsureSchema`. Wrap errors with the step and budget. Verify with a fault-injecting interceptor test (same pattern as `migrate_faultinject_test.go`): a stalled `CreateFieldIndex` past the schema timeout yields an error naming schema provisioning and `ENGRAM_QDRANT_SCHEMA_TIMEOUT`, and a stall shorter than the timeout but longer than 15 s succeeds.
- [ ] 4.2 Pass `store.WithQuantization(cfg.Qdrant.Quantization)` only on the `engram serve` construction path. `StoreFromEnv`, `StoreAndDeciderFromEnv` (unless serve-only), `migrateFamilyStoreFromEnv` and `spineConsolidateStoreFromEnv` leave it unset. Verify with a test: the server constructor's store applies `int8`, and a CLI constructor's store leaves an `off`-mode collection unquantized with `ENGRAM_QDRANT_QUANTIZATION` unset.
- [ ] 4.3 Verify that a quantization update rejected by Qdrant fails `engram serve` before `ListenAndServe`. Use an interceptor that returns an error for `UpdateCollection`, and check that `server.Register` returns an error naming the quantization step and that the classified exit is non-zero.

## 5. Helm chart

- [ ] 5.1 Add values `memory.qdrant.quantization: int8` and `memory.qdrant.schemaTimeoutSeconds: 120` with comments in the values file's existing style. Emit `ENGRAM_QDRANT_QUANTIZATION` and `ENGRAM_QDRANT_SCHEMA_TIMEOUT: "<n>s"` from `engram.containerEnv`. Verify that `task chart:lint` passes, including the `engram.containerEnv` drift pin.
- [ ] 5.2 Add a `tcpSocket` `startupProbe` to `memory-mcp.yaml` with `periodSeconds: 5` and `failureThreshold` computed as `ceil((15 + n + 15) / 5)`. Verify with new `chart:validate` assertions: the default render contains the probe and `ENGRAM_QDRANT_QUANTIZATION=int8`, and `--set memory.qdrant.schemaTimeoutSeconds=600` renders `ENGRAM_QDRANT_SCHEMA_TIMEOUT` `600s` with `failureThreshold * periodSeconds >= 615`.
- [ ] 5.3 Document the startup probe and the two values in `guides/deploy.md`. Verify that `task lint` is clean.

## 6. Recall-quality gate (before shipping)

- [ ] 6.1 Make `task eval:retrieval` able to run with quantization on and off, for example by honouring `ENGRAM_QDRANT_QUANTIZATION` in `retrievaleval`'s store construction. In the quantized run, force optimization of each eval collection (low `indexing_threshold`, wait for status green) so quantized segments actually exist. Verify by logging, per eval collection, that its info reports the int8 config and status green before any query runs.
- [ ] 6.2 Run `task eval:retrieval` with `int8` and with `off` against a live gateway, and record recall@k and MRR for both in the PR description. Verify that `int8` shows no regression versus `off`. If it does, stop and raise it before merging.

## 7. Upgrade notes and integration

- [ ] 7.1 Add a `guides/upgrade.md` entry: the first `serve` boot quantizes the collection (background re-optimization, under a second at production scale, no migration step), memory cost is about 3 KB per record, the rollback path is `memory.qdrant.quantization: off` or the old image, and the new startup probe exists. Verify that `task lint` is clean.
- [ ] 7.2 Run the full quality gate, `task` (lint and test), and confirm that `license:check` passes with no headers needed under `openspec/`. Verify that both exit 0.
- [ ] 7.3 After deploy (tracked in the PR, not blocking merge), check the Qdrant slow-request log for `query` entries, `store.Search` p95 (target under 200 ms), and major page faults per hour if selfhosted-cluster #2516 is available. Close #698 and #683 with the observed numbers.
