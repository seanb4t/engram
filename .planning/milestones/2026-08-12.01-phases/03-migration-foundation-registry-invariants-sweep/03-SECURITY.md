---
phase: 03
slug: migration-foundation-registry-invariants-sweep
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-14
---

# Phase 03 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Register origin: **authored at plan time** — all five `03-0N-PLAN.md` files carry a
`<threat_model>` block. The auditor verified that each declared mitigation exists in the
implementation; it did not scan for new threats, and found no new attack surface on
inspection.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| operator tier → Qdrant | `Store.Migrate` sweeps and rewrites stored records outside any recall path; no Subject, no authz predicate | whole payloads of every record in the collection, all owners |
| `internal/store` → `internal/migrate` | one-way import into a stdlib-only leaf; steps are pure map→map functions with no I/O and no client handle | a decoded `map[string]any` copy of one record's payload |
| registered step → stored payload | an `ApplyFunc` proposes payload changes; only declared added keys are ever written back | added keys plus `schema_version` |
| recall tier → `schema_version` | recall (`Search`/`List`/`SearchDiscovery`/`ListScheduled`) must never gate on `schema_version` | none, by construction — this boundary exists to stay empty |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 | Tampering | sweep write path | high | mitigate | `CheckAdditive` runs before any write (`migrate.go:230-236`), fail-closed on the whole call; sole write verb is a per-point `SetPayload` built from `AddedKeys` (`migrate.go:78-84,240-271`) | closed |
| T-03-02 | Denial of Service | `Store.Migrate` loop | medium | mitigate | Non-shrinking-backlog guard from a fresh exact `Count`, never a write signal (`migrate.go:156-178`); exercised live by fault-inject scenario 2 and converge subtest 3 | closed |
| T-03-03 | Denial of Service | `payloadToMap` | low | **accept** | `payload()` (`store.go:600-650`) emits only flat scalars plus one-level `tags`/`supersedes` slices — no nested path into `payloadToMap` exists today | closed |
| T-03-04 | Information Disclosure | `backlogFilter` reaching recall/authz | high | mitigate | `backlogFilter` is operator-tier only (`migratebacklog.go:45-49`); `Store.Migrate`'s `Count`/`ScrollAndOffset` mechanically classified into `operatorMigrationEmitters` by an AST derivation that does not depend on reachability (`schemaversion_recallgate_test.go:544`), paired with wire-level `TestSchemaVersionNeverGatesRecall` | closed |
| T-03-05 | Elevation of Privilege | `Reversibility` interface | medium | mitigate | Unexported `isReversibility()` seal (`step.go:20-38`); `TestReversibilityIsSealedToThisPackage` (`step_test.go:146-252`) combines a reflection method-count check, an AST scan for exported carriers, and a live out-of-package build probe | closed |
| T-03-06 | Repudiation | partial sweep audit trail | low | **accept** | `MigrateResult` documents `Migrated`/`Failed` as telemetry-only; `Backlog`, a fresh re-derivation, is authoritative (`migrate.go:39-53`) | closed |
| T-03-07 | Tampering | explicit `nil` reversibility | high | mitigate | `NewStep` panic check (`step.go:131-133`); `TestNewStepPanicsOnNilReversibility` (`step_test.go:34-41`), proven load-bearing by RED cycle 1 | closed |
| T-03-08 | Tampering | a gate that silently checks nothing | high | mitigate | Non-zero-coverage guards present and executed **before** the scan/loop they guard in every gate: `leafpurity_test.go:72-74`, `step_test.go:166-167,192-197`, `registry_test.go:109-110,203-204,252-254,293-294`, `additive_test.go:184-185` | closed |
| T-03-09 | Tampering | leaf-package purity | medium | mitigate | `TestMigratePackageIsStdlibOnlyLeaf` (source scan, the shipped guarantee); `go list -deps` cross-check re-confirmed live at 0 non-stdlib in `03-VERIFICATION.md`. The out-of-band grep was itself repaired this phase — see Audit Notes | closed |
| T-03-10 | Denial of Service | build-probe temp dir | low | mitigate | Created under `internal/migrate/`, removed by `t.Cleanup` (`step_test.go:223-231`); `03-REVIEW.md` IN-02 records the abnormal-termination residual as accepted | closed |
| T-03-11 | Tampering | declared `addsKeys` drift | high | mitigate | True set-equality on the added-key set, proven by mirrored fixture rows ("adds an undeclared key" / "declares a key it never adds") distinguishing it from subset and superset (`additive_test.go`) | closed |
| T-03-12 | Tampering | in-place value overwrite of an existing key | medium | mitigate | Explicitly **outside** what a key-set diff can see; contained downstream by write shaping — the `SetPayload` map is built from `AddedKeys(original, current)` only, never `current` wholesale (`migrate.go:240-258`). Blindness documented at `additive.go:53-60`; proven by `TestMigrateWritesOnlyAddedKeys` | closed |
| T-03-13 | Tampering | records left unmigrated while the sweep reports success | high | mitigate | `migrate_faultinject_test.go` subtests 1 (self-heals in a later pass) and 2 (persistent failure terminates; resume converges) | closed |
| T-03-14 | Tampering | acting on a `SetPayload` error that misreports what landed (qdrant/qdrant#9371) | high | mitigate | Fault-inject subtest 3 asserts `Migrated==0`/`Failed==4` (wrong signals) against `Backlog==0` and per-record marker presence (right collection state) | closed |
| T-03-15 | Tampering | test seam leaking into production `Store` state | medium | mitigate | `git diff --exit-code -- internal/store/store.go` clean — production `Store` byte-identical; zero FUNCTIONAL references to the rejected hook (`rg -o 's\.setPayloadKeys\|setPayloadKeys\s*[:=]'` → 0), against 16 in `store_test.go` and 1 in `store.go`, proving the check can match. See Audit Notes | closed |
| T-03-16 | Repudiation | fault injector that injects nothing and passes | high | mitigate | Every subtest asserts `inj.seen() != 0` (fatal) and `inj.injected()` equals the armed count (`migrate_faultinject_test.go:379-384,427-429,554-559`) | closed |
| T-03-17 | Tampering | re-processing an already-current record | high | mitigate | Converge subtest 1 asserts both at the wire (`writeIDs` exclusion) and in the collection (`converge_marker` absent, `schema_version` unchanged) (`migrate_converge_test.go:203-231`) | closed |
| T-03-18 | Tampering | proving the property via raw payload injection instead of the production write path | high | mitigate | `injectRawPayload` count 0 in the file; the mid-sweep write goes through `writerStore.Upsert` (`migrate_converge_test.go:134`) | closed |
| T-03-19 | Repudiation | flaky sleep-timed race | medium | mitigate | No `time.Sleep` in the file; the trigger is `midSweepInterceptor`'s `onScroll` callback, deterministic on the sweep's own second `ScrollPoints` | closed |
| T-03-20 | Tampering | `ApplyFunc` mutating its input map, aliasing `before`/`after` | high | mitigate | Two independent `maps.Clone` per step (`migrate.go:213-225`); `additive_test.go` row "removes a key by mutating its input map in place" | closed |
| T-03-21 | Repudiation | build probe reporting an environment problem as proof of the seal | medium | mitigate | Three discriminated `t.Fatalf` messages — toolchain-unavailable, unexpected-successful-build, wrong-reason failure requiring the marker method in the output (`step_test.go:211-246`) | closed |
| T-03-22 | Tampering | Phase 4 relocating `Registry` into a builder function, voiding D-03 | medium | mitigate | `TestRegistryIsAPackageLevelVarWithPhase4Marker` (`registry_test.go:242-317`) asserts file-scope `var`, a non-empty `ValueSpec.Values` composite literal, and the `// PHASE4:`/D-03 marker; independently confirmed by live mutation testing in `03-REVIEW.md` §1 | closed |
| T-03-23 | Tampering (process) | wave-2 RED patches colliding in one git index | low | mitigate | Not code-auditable. Recorded evidence: 13 distinct red-evidence patches across the five plans with no cross-contamination, clean working tree, coherent non-overlapping task commits per SUMMARY | closed |
| T-03-24 | Repudiation | resume assertion made against an interceptor wired to a different client | high | mitigate | `resumeInj` is a separate injector on a separate client; `resumeInj.seen() > 0` asserted fatal before set-equality against `wantOutstanding` (`migrate_faultinject_test.go:464-495`) | closed |
| T-03-25 | Repudiation | deriving record identity from fixture insertion order | medium | mitigate | `succeededID := recordedIDs[0]` from the injector's captured wire traffic, with a non-empty check before indexing (`migrate_faultinject_test.go:431-441`) | closed |
| T-03-26 | Repudiation | SC5 recorded as fully discharged when only partially proven | high | mitigate | `03-05-SUMMARY.md:37,213` reports SC5 as **partially proven**, names the causal-half deferral to Phase 4 and the `// PHASE4:` marker, and does not claim SC5 green | closed |
| T-03-27 | Repudiation | `t.Fatalf` from an uncontrolled goroutine silently failing to fail | medium | mitigate | Hook uses `h.recordErr` into a mutex-guarded slice, drained after `Migrate` returns; no `t.Fatal`/`t.Error` call sites inside the hook body (`migrate_converge_test.go:128-177,368-376`) | closed |
| T-03-28 | Repudiation | "hook fired exactly once" inferred rather than observed | medium | mitigate | Integer `fires` counter incremented inside `sync.Once.Do`, asserted `== 1`; `triggerMatches` asserted `>= 1` (`migrate_converge_test.go:278-283,325,346-350`) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-03-01 | T-03-03 | `payloadToMap` recursion depth is bounded by what `Store.Upsert` writes through `payload()` — flat scalars plus a single-level tag list. No untrusted party can author an arbitrarily nested payload today. Revisit if a future phase admits nested payload authoring. | plan-time disposition (03-01), verified by audit | 2026-08-14 |
| AR-03-02 | T-03-06 | Per-record audit logging of a partially-applied sweep is out of this phase's scope. `MigrateResult`'s counters are telemetry; the authoritative statement is always a fresh re-derivation (`Backlog`), and the collection's own `schema_version` values are self-describing. | plan-time disposition (03-01), verified by audit | 2026-08-14 |

---

## Audit Notes

Two threats closed only after a **defective verification command** was repaired. In both cases the
security property was intact and the gate that was supposed to demonstrate it was measuring a proxy.
Recorded here because the pattern recurred three times in this phase and is the milestone's
signature defect (durable records `x6v6qxqd6f`, `rnacmkj0xg`, `k000pn14qp`).

**T-03-15** — the plan's criterion was `rg -c 'setPayloadKeys' internal/store/migrate_faultinject_test.go`
prints `0`. It prints `1`: a doc comment at line 86 explaining why an independent gRPC interceptor was
chosen *over* the rejected hook. A bare identifier grep cannot distinguish a functional use from prose
naming the rejected alternative, so the only way to satisfy it as written is to **delete the comment**
— making the code less informative to pass a check about a property it does not measure. The criterion
now asserts the property directly: production `Store` byte-identical (`git diff --exit-code`), and zero
functional references (`rg -o 's\.setPayloadKeys|setPayloadKeys\s*[:=]' | wc -l` → 0, against 16 in
`store_test.go` and 1 in `store.go`, proving the pattern can match).

**T-03-09** — the leaf-purity cross-check `go list -deps ./internal/migrate | rg -c '^[^/]+\.[^/]+/'`
was a **constant**: `go list -deps` always includes the listed package itself, and this module's path
(`github.com/…`) matches the non-stdlib pattern, so it returned `1` for any input. Its companion
anti-vacuity guard (`| wc -l` > 0) checked that the *producer emitted rows*, not that the *filter could
ever match*, so both halves passed unconditionally. Repaired in commit `e8bf14f2`; the shipped
guarantee was always the source-scanning `TestMigratePackageIsStdlibOnlyLeaf`.

Neither repair changed implementation code. Both are recorded so Phase 4 does not inherit a gate that
cannot fail.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-14 | 28 | 28 | 0 | gsd-security-auditor (ASVS L1, block_on: high) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-14
