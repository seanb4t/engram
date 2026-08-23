# Red-Evidence Manifest — milestone 2026-08-12.01

Retired target-test mappings for the red-evidence patches archived under this
directory. Each patch is a reversible mutation that was proven to make its named
test FAIL; `internal/store`'s `TestRedEvidencePatchesAreLive` applied them on every
run while this milestone was open.

They are **no longer executed**. The harness is scoped to the open milestone only
(see the SCOPE note on `redEvidenceDirs`), mirroring `internal/keylinks`'
`TestActiveMilestoneKeyLinksSatisfiable` and its D-04 rationale: a patch asserts
that mutating *today's* source makes a test fail, so re-applying a shipped
milestone's patches at HEAD reds on any legitimate later refactor — a red that is
not a defect. The mapping is recorded here so the archive stays self-describing
and a historical patch can still be re-run by hand against the commit it was
authored at.

## 02-record-schema-versioning-foundation

9 patches.

| Patch | Target test proven RED |
|---|---|
| `02-02-red-1-bypass.patch` | `TestEveryPointWriteRoutesThroughPayload` |
| `02-02-red-2-stale-classification.patch` | `TestEveryPointWriteRoutesThroughPayload` |
| `02-02-red-3-cross-package-client.patch` | `TestQdrantClientIsHeldOnlyByStorePackage` |
| `02-03-red-1-toplevel.patch` | `TestSchemaVersionNeverGatesRecall` |
| `02-03-red-2-nested.patch` | `TestSchemaVersionNeverGatesRecall` |
| `02-03-red-3-unclassified-scroll.patch` | `TestRecallEmissionSetIsCompleteAndClassified` |
| `02-03-red-4-unclassified-scrollandoffset.patch` | `TestRecallEmissionSetIsCompleteAndClassified` |
| `02-03-red-5-linkage.patch` | `TestRecallEmissionSetIsCompleteAndClassified` |
| `02-review-wr01-monotonic-max.patch` | `TestPayloadRoundTripsSchemaVersion` |

## 03-migration-foundation-registry-invariants-sweep

12 patches.

| Patch | Target test proven RED |
|---|---|
| `03-01-red-1-range-only-filter.patch` | `TestBacklogFilterMatchesAbsentAndBelowTarget` |
| `03-01-red-2-declared-drift-written.patch` | `TestMigrateRefusesNonAdditiveStep` |
| `03-02-red-1-nil-rev-accepted.patch` | `TestNewStepPanicsOnNilReversibility` |
| `03-02-red-2-contiguity-dropped.patch` | `TestValidateRejectsOrderingAndUniquenessViolations` |
| `03-02-red-3-nonstdlib-import.patch` | `TestMigratePackageIsStdlibOnlyLeaf` |
| `03-02-red-4-zero-files-scanned.patch` | `TestMigratePackageIsStdlibOnlyLeaf` |
| `03-03-red-1-superset-not-equality.patch` | `TestAdditiveOnlyKeySetDiff` |
| `03-03-red-2-removal-check-dropped.patch` | `TestAdditiveOnlyKeySetDiff` |
| `03-03-red-3-zero-fixtures.patch` | `TestAdditiveOnlyKeySetDiff` |
| `03-04-red-2-trust-error-signal.patch` | `TestMigratePartialFailureResume` |
| `03-05-red-1-lte-includes-current.patch` | `TestBacklogFilterMatchesAbsentAndBelowTarget` |
| `03-05-red-2-midsweep-write-skipped.patch` | `TestMigrateConvergesWithoutLock` |

## 06-typed-operator-renderer

3 patches.

| Patch | Target test proven RED |
|---|---|
| `06-red-1-empty-field-line-suppressed.patch` | `TestOperatorViewIdentityAcrossEveryOperatorCommand` |
| `06-red-2-text-lane-only-field.patch` | `TestOperatorViewIdentityAcrossEveryOperatorCommand` |
| `06-red-3-operator-command-dropped.patch` | `TestOperatorViewFixturesCoverEveryOperatorCommand` |

**Total:** 24 patches across 3 phases.
