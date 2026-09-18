# Red-Evidence Manifest — milestone 2026-09-13.01

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
authored at (milestone squash `0aaba2c4`, released as v0.17.0).

## 01-executor-correctness-man-pages

4 patches.

| Patch | Target test proven RED |
|---|---|
| `01-01-osrun-ctx-err-first.patch` | `TestOsRunReportsContextDeadlineExceeded` |
| `01-01-runseam-timeout-wording.patch` | `TestDriftReportedLegibly` |
| `01-02-man-header-pinned.patch` | `TestManPagesByteStable` |
| `01-02-man-tree-restore.patch` | `TestManGenerationLeavesCommandTreeUnchanged` |

## 02-custom-auth-headers

5 patches.

| Patch | Target test proven RED |
|---|---|
| `02-01-claudecode-header-args.patch` | `TestClaudeCodeHeaders` |
| `02-01-codex-header-decline.patch` | `TestCodexDeclinesHeaders` |
| `02-02-opencode-header-dialect.patch` | `TestOpenCodeHeaders` |
| `02-02-generic-header-order.patch` | `TestGenericHeaders` |
| `02-03-header-validation.patch` | `TestSetupHeaderRejectsAuthorizationCollision` |

## 03-plugin-first-delivery

5 patches.

| Patch | Target test proven RED |
|---|---|
| `03-01-plugin-version-compare.patch` | `TestPluginVersionCompare` |
| `03-01-plugin-state-machine.patch` | `TestPluginPlan` |
| `03-01-codex-remove-then-add.patch` | `TestPluginPlan` |
| `03-02-detect-presence.patch` | `TestDetectPresence` |
| `03-03-plugin-skips-native.patch` | `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites` |

## 04-drift-detection-read-only

9 patches.

| Patch | Target test proven RED |
|---|---|
| `04-01-preserved-not-in-classify.patch` | `TestClassifyExhaustiveOutcomeCombinations` |
| `04-01-precedence-slot.patch` | `TestAggregateOutcomeExhaustive` |
| `04-01-registered-raw-capture.patch` | `TestRedactionUnconditional` |
| `04-01-codex-tolerant-decode.patch` | `TestObserveCodexRegistration` |
| `04-04-facets-not-copied.patch` | `TestSetupPreviewJSONCarriesDriftFacets` |
| `04-04-apply-summary-preserved.patch` | `TestSetupApplySummaryCountsPreserved` |
| `04-05-claudecode-status-facet.patch` | `TestObserveClaudeCodeRegistration` |
| `04-05-claudecode-value-retained.patch` | `TestRedactionUnconditional` |
| `04-05-codex-literal-header-tolerated.patch` | `TestObserveCodexRegistration` |

## 05-apply-time-preserve-gate-documentation

10 patches.

| Patch | Target test proven RED |
|---|---|
| `05-01-preserved-flag-inside-loop.patch` | `TestApplyPreservedNeverRunsClaudeCodeRemove` |
| `05-01-already-correct-still-writes.patch` | `TestApplyAlreadyCorrectIssuesZeroWrites` |
| `05-01-registered-raw-probe2.patch` | `TestApplyWroteRegisteredIsRedacted` |
| `05-01-oauth-note-on-bearer.patch` | `TestOAuthReLoginConsequence` |
| `05-01-remediation-composed-in-executor.patch` | `TestPreviewClassifiesRegistration` |
| `05-02-help-drops-apply-gate.patch` | `TestSetupHelpStatesApplyGate` |
| `05-02-guide-stale-guarantee.patch` | `TestAgentSetupGuideDocumentsDrift` |
| `05-02-guide-preserved-no-remediation.patch` | `TestAgentSetupGuideDocumentsDrift` |
| `05-03-install-drops-man-page.patch` | `TestInstallGuideDocumentsSetupV2` |
| `05-03-plugin-drops-preserved-crosslink.patch` | `TestPluginGuideDocumentsPluginFirst` |
