// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file is the automated harness for the red-evidence claim made by
// every phase that ships one (Phase 2: 02-02-SUMMARY.md D5, 02-03-SUMMARY.md
// D4; Phase 3: each 03-0N-SUMMARY.md's own red-evidence section): "the gate
// was proven RED against real source by applying a patch from
// red-evidence/, observing the target test FAIL, and reverting by exact
// inverse patch." That claim was previously reproducible only by a human
// running git apply / go test / git apply -R by hand — nothing detected a
// red-evidence patch that had gone stale, corrupt, or was no longer RED
// (concretely: Phase 4's CurrentVersion 0->1 blast radius, commits
// 8fb9d6d9/0fa76d62/96711281/9695616d, repaired Phase 3's TESTS but left
// four of Phase 3's red-evidence PATCHES stale over the same code, and
// nothing caught it — this harness exists to make that class of drift
// impossible to miss again). This harness converts it into a real gate.
//
// redEvidenceDirs holds ONE independent phase-directory entry per phase
// that has shipped red-evidence. For EVERY patch under EVERY directory
// (each discovered by its own glob, never a hand-listed fixture set), it
// asserts:
//  1. the patch applies cleanly at HEAD (git apply --check)
//  2. with the patch applied, its mapped target test FAILS
//  3. git apply -R restores the tree exactly (git diff --exit-code clean)
//
// It fails loudly, per directory, if that directory's glob matches zero
// patches (zero-applicability guard, matching this package's own house
// style — see schemaversion_stamp_gate_test.go) and if any patch discovered
// under that directory has no entry in that directory's own mapping — a
// directory that vanishes or gets renamed fails ITS OWN subtest, it can
// never silently contribute zero patches to the overall run.
//
// Tree safety: this harness mutates the working tree (it applies real git
// patches to real source files, potentially across more than one package —
// e.g. Phase 3's patches touch both internal/store and internal/migrate).
// It refuses to run if any file a patch touches is already dirty, and it
// ALWAYS reverts via t.Cleanup — including on a failing assertion or a
// panic — so a broken harness run never leaves store.go, additive.go, or
// any other file mutated.
//
// NOT safe to run concurrently with ITSELF. Two simultaneous runs share one
// working tree, and their apply/revert cycles interleave: the per-patch
// dirty-check catches the collision and fails the second run (as designed),
// but the interleaved reverts can still leave a source file mutated —
// observed 2026-08-16, leaving migrate_converge_test.go dirty. Ordinary
// `go test ./...` is fine (this test lives in one package and is not
// t.Parallel); the hazard is a human or agent launching a second run while
// one is already in flight. If a run dies unexpectedly, check
// `git status --porcelain` before trusting the tree.
//
// Nested `go test`: the harness shells out to `go test -run` scoped to the
// single mapped target test function name, run against ./... (not just
// this package) because a phase's target test may live in a sibling
// package. The -run regex still narrows to exactly the one named function,
// which never matches this harness's own test name
// (TestRedEvidencePatchesAreLive) or any other harness test in this file —
// so the subprocess never re-invokes this harness, no recursion guard
// beyond that scoping is needed.
//
// This is gated behind testing.Short(): it is a full mutate/build/test/
// revert cycle per patch (one per registered active-milestone patch),
// materially slower than the rest of this package's unit tests, so
// `go test -short` (the fast path) skips it while `task test` (which does
// not pass -short) still runs it in CI.
package store

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// redEvidenceDirs maps an ACTIVE-MILESTONE phase's red-evidence directory
// (relative to the module root — git's own working directory convention
// for `git apply`, NOT this package's directory) to that phase's own
// patch-filename -> target-test-function mapping. Adding a phase means
// adding one entry here; TestRedEvidencePatchesAreLive iterates every
// directory independently, so a phase's zero-applicability guard and
// set-equality mapping check never leak into another phase's.
//
// SCOPE: the OPEN milestone only, never an archived one. This mirrors
// internal/keylinks' TestActiveMilestoneKeyLinksSatisfiable and its D-04
// rationale verbatim, because the property is identical: a red-evidence
// patch asserts that mutating today's source makes a named test fail, so
// its liveness depends on the code as it stands right now. Re-applying a
// shipped milestone's patches at HEAD goes red whenever that code is
// legitimately refactored — a red that is not a defect. A gate that cries
// wolf on unrelated refactors trains people to ignore it, which recreates
// the exact silent-no-op failure this harness exists to prevent.
//
// So at /gsd-complete-milestone the phase directories are archived to
// .planning/milestones/<label>-phases/, this map empties, and the patches
// become history — inspectable there, with their retired target-test
// mappings recorded alongside them in RED-EVIDENCE.md. It refills when the
// next milestone ships its own.
//
// An empty map is therefore legitimate BETWEEN milestones and a defect
// DURING one. TestRedEvidencePatchesAreLive tells those apart rather than
// treating zero directories as vacuously green — see its empty-map guard.
var redEvidenceDirs = map[string]map[string]string{
	".planning/phases/01-test-harness-fixture-helper/red-evidence": {
		"01-01-storetest-raw-client-write.patch":       "TestQdrantClientIsHeldOnlyByStorePackage",         // reverts: D-13's per-holder write-check catching an injected storetest.Dial write
		"01-04-listscopes-full-payload-selector.patch": "TestListScopesFullPayloadsOverGRPCLimit",          // reverts: ListScopes' scope-only WithPayload selector back to full payloads (#583)
		"01-05-bare-qdrant-newclient-in-test.patch":    "TestQdrantClientConstructedOnlyByNewQdrantClient", // reverts: a bare qdrant.NewClient bypass injected into a _test.go file outside internal/store (D-11)
		"01-05-ci-qdrant-image-drift.patch":            "TestQdrantImageMatchesCIService",                  // reverts: CI's services.qdrant image drifting from storetest.QdrantImage (D-10)
	},
	".planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence": {
		"02-01-classifier-not-in-base-options.patch":             "TestResponseTooLargeClassifierSitsInsideCallerChain", // reverts: NewQdrantClient no longer installing the classifier in its base dial options (D-01)
		"02-01-classifier-requires-two-number-shape.patch":       "TestStoreListOverflowIsResponseTooLarge",             // reverts: isRecvLimitMessage additionally requiring the two-number shape, missing real traffic's single-number shape (RESEARCH.md Pitfall 1)
		"02-01-classifier-relabels-any-resource-exhausted.patch": "TestClassifyResponseTooLarge",                        // reverts: classifyResponseTooLarge dropping its isRecvLimitMessage message-shape check, relabeling every ResourceExhausted (D-02)
		"02-02-connect-arm-removed.patch":                        "TestConnectListMemoriesResponseTooLarge",             // reverts: connectError's ErrResponseTooLarge -> resource_exhausted arm (D-06)
		"02-02-mcp-mapper-unregistered.patch":                    "TestMCPListMemoryResponseTooLarge",                   // reverts: addToolMiddleware no longer registering mapResponseTooLarge (D-08)
		"02-03-exit-mapping-reverted.patch":                      "TestExitCodeBaseline",                                // reverts: exitCodeForConnectErr's CodeResourceExhausted -> exitTooLarge case (D-07)
		"02-03-operator-arm-removed.patch":                       "TestClassifyOperatorErrCodesAreDistinct",             // reverts: classifyOperatorErr's ErrResponseTooLarge arm, applying D-07's exit code to the operator tier (D-07/D-10)
		"02-03-errors-doc-drops-response-too-large.patch":        "TestErrorsDocHintCodesMatchArgErrorConstants",        // reverts: errors.md's response_too_large row of the eleven-code hint table (D-05/D-11)
	},
	".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence": {
		"03-01-content-cap-removed.patch":                 "TestMemoryWriteCapsRejectOnEveryCreateLane",         // reverts: validateStoreArgs' content cap on store/schedule/supersede, MCP and Connect (D-01)
		"03-01-tags-cap-removed.patch":                    "TestMemoryWriteCapBoundaries",                       // reverts: validateStoreArgs' checkTags call, removing the tags/tag-bytes cap on every create lane (D-10)
		"03-01-config-accepts-zero.patch":                 "TestMemoryCapsRejectZeroAndNonPositive",             // reverts: validatePositiveCap's zero/non-positive rejection arm, letting 0 configure an always-enforced cap (D-09)
		"03-02-sweep-count-not-byte-derived.patch":        "TestScrollAllPointsByteBudget",                      // reverts: sweepLimit deriving its per-RPC count from the byte budget, back to the bare spineScrollBatch count (D-02)
		"03-02-ceiling-drops-citations.patch":             "TestRecordCeilingHoldsForMaxCapRecord",              // reverts: fullRecordCeiling accounting for citations, dropping the dominant per-record term (RESEARCH.md "content alone" pitfall)
		"03-02-sweep-fallback-removed.patch":              "TestScrollAllPointsBatchOfOneFallback",              // reverts: scrollAllPoints' batch-of-1 fallback for a legacy over-cap record (D-07)
		"03-02-sweep-swallows-single-overflow.patch":      "TestScrollAllPointsSingleOversizedRecordFailsNamed", // reverts: a single-record overflow at the batch-of-1 fallback failing named rather than being silently swallowed (D-07)
		"03-03-update-content-cap-removed.patch":          "TestUpdateMemoryContentCap",                         // reverts: deps.updateMemory's content-cap check on Connect's field-mask UpdateMemory lane (D-09)
		"03-03-update-gates-on-presence-not-change.patch": "TestUpdateMemoryLegacyOversizedRecord",              // reverts: the update content-cap check's contentChanged gate, widened to presence so an unchanged legacy record is rejected (D-07/D-09)
		"03-03-record-caps-not-wired.patch":               "TestStoreFromConfigCarriesRecordCaps",               // reverts: storeFromConfig wiring store.WithRecordCaps(recordCapsFromConfig(cfg)) into the production Store (D-02/D-09)
		"03-04-ordered-page-ignores-page-budget.patch":    "TestScrollOrderedPageByteBudget",                    // reverts: scrollOrderedPage's remaining-page-budget clamp on its per-RPC count (D-02)
		"03-04-budget-cut-reported-exhausted.patch":       "TestScrollOrderedPageByteBudget",                    // reverts: a budget-cut page's Exhausted staying false, distinct from CutByBudget (REQ-list-contract-unchanged precondition)
		"03-04-ordered-page-tie-exclusion-removed.patch":  "TestScrollOrderedPageTiesAcrossRPCBoundaries",       // reverts: excludeSeen's must_not has_id exclusion of already-emitted ids at the tie boundary (D-03)
	},
	".planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence": {
		"04-04-search-fetch-drops-caller-filter.patch":       "TestSearchTwoPhaseBounded",                    // reverts: includeIDs re-wrapping the caller's own filter as a nested condition alongside the id-set inclusion (D-09)
		"04-01-hint-code-value-drift.patch":                  "TestErrorsDocHintCodesMatchArgErrorConstants", // reverts: HintOutOfRange's wire value agreeing with the published errors.md table (D-10)
		"04-01-overflow-envelope-hint-reverted.patch":        "TestResponseTooLargeEnvelopeShape",            // reverts: the shared overflow envelope rendering HintResponseTooLarge instead of a different hint (D-11)
		"04-02-cursor-reports-last-page-on-budget-cut.patch": "TestStoreListContractInvariant",               // reverts: a budget-cut cursor page's non-empty next cursor, distinct from Exhausted (D-06)
		"04-02-zero-limit-fetches-whole-scope.patch":         "TestStoreListOffsetBounded",                   // reverts: a zero List limit resolving to the numeric store.MaxRecallLimit instead of an unbounded single Scroll of the whole matched scope (D-01)
		"04-02-offset-overflow-guard-removed.patch":          "TestStoreListOffsetBounded",                   // reverts: the guard rejecting an offset+limit pair that would wrap uint64, before any RPC (D-01)
		"04-03-prefix-walk-uses-full-view.patch":             "TestStoreListDeepOffsetBounded",               // reverts: the deep-offset prefix walk's keys-only projection, back to the full view (D-07)
		"04-03-assembly-loop-stops-after-one-page.patch":     "TestListScheduledBounded",                     // reverts: collectOrderedPages' assembly loop following Next across multiple primitive pages until the requested count is reached (D-05)
		"04-04-search-fetch-always-issues-an-rpc.patch":      "TestSearchFetchSkipsEmptyBatch",               // reverts: fetchPayloadsByID's empty-batch short-circuit, so a zero-id fetch still issues a Scroll (D-09, RESEARCH Pitfall 1)
		"04-04-search-returns-reversed-rank-order.patch":     "TestSearchPreservesRankOrder",                 // reverts: Store.Search re-attaching phase one's own rank order when rebuilding the result (D-09)
		"04-05-no-summary-backfill-removed.patch":            "TestNoSummaryContentBackfill",                 // reverts: Store.List's call to the shared no-summary content backfill (Phase 3 D-04)
		"04-05-list-always-full-view.patch":                  "TestRecallViewSelection",                      // reverts: recallView's full/summary projection selection, collapsed to always full (Phase 3 D-04)
		"04-05-store-clamps-instead-of-refusing.patch":       "TestStoreRejectsOverMaximumCount",             // reverts: the store's over-maximum guard refusing rather than silently permitting an over-maximum count (D-10)
		"04-06-surface-maximum-check-removed.patch":          "TestOutOfRangeRejectedOnEveryRecallSurface",   // reverts: the typed list core's shared over-maximum check, falling through to the store's differently-shaped backstop error (D-10)
		"04-06-maximum-check-runs-after-embed.patch":         "TestOutOfRangeRejectedBeforeAnyBackend",       // reverts: the memory-search core method's over-maximum check running BEFORE the embed call, closing the denial-of-service ordering hazard (D-10)
		"04-06-full-not-threaded-to-rule-listing.patch":      "TestListRulesFullThreaded",                    // reverts: the rule listing's direct store-options literal threading a.Full through to the store (Pattern 6 step 4)
		"04-07-cli-help-drops-the-maximum.patch":             "TestRecallMaximumIsStatedNumerically",         // reverts: the CLI list --limit flag's usage string stating the numeric maximum (D-01, D-04)
	},
	".planning/phases/05-operator-sweeps-ci-backstop/red-evidence": {
		"05-01-scanspine-view-unbudgeted.patch":                 "TestScanSpineBoundedOverGRPCLimit",                // reverts: ScanSpine's scanView projection back to a zero-ceiling full-payload view (D-01, D-04)
		"05-01-citations-view-unbudgeted.patch":                 "TestEnumerateCitationsBoundedOverGRPCLimit",       // reverts: EnumerateCitations' citationsView projection back to a zero-ceiling full-payload view (D-04)
		"05-01-purge-view-unbudgeted.patch":                     "TestPreviewPurgeBoundedOverGRPCLimit",             // reverts: derivePurgeEligible's reuse of summaryView back to a zero-ceiling full-payload view (D-04)
		"05-02-revert-preview-view-unbudgeted.patch":            "TestRevertPreviewBoundedOverGRPCLimit",            // reverts: previewRevertWithSteps' schemaVersionOnlyView back to a zero-ceiling full-payload view (D-01, D-04)
		"05-03-migrate-sweep-view-unbudgeted.patch":             "TestMigrateBoundedOverGRPCLimit",                  // reverts: Store.Migrate's default sweep-mode pass view back to a zero-ceiling full-payload view (D-01)
		"05-03-migrate-pass-sentinel-escapes-as-failure.patch":  "TestMigrateBoundedOverGRPCLimit",                  // reverts: the call site's errors.Is unwrap of errMigratePassBatchComplete, so a completed pass is returned to the caller as a sweep failure (D-01)
		"05-03-revert-pass-sentinel-escapes-as-failure.patch":   "TestRevertApplyBoundedOverGRPCLimit",              // reverts: the call site's errors.Is unwrap of errRevertPassBatchComplete, so a completed pass is returned to the caller as a sweep failure (D-01)
		"05-04-summarize-limit-sentinel-escapes.patch":          "TestSummarizeMissingBoundedOverGRPCLimit",         // reverts: the call site's errors.Is unwrap of errSummarizeLimitReached, so the caller's own limit sub-case is observed as an error (D-01)
		"05-04-reindex-walks-store-collection-not-source.patch": "TestReindexSourceOverride",                        // reverts: Reindex's source walk passing s.collection instead of the effective source, so a source override silently scans nothing (D-01)
		"05-04-reindex-final-partial-page-dropped.patch":        "TestReindexBoundedOverGRPCLimit",                  // reverts: the trailing flush of the accumulator's final partial page, silently skipping a scope's last records (D-01)
		"05-05-backstop-appended-after-caller-options.patch":    "TestQdrantRecvLimitBackstopPrecedesCallerOptions", // reverts: qdrantDialOptions appending the productionRecvLimit backstop BEFORE caller options, so a caller's own receive limit still wins (D-05, D-06)
	},
	".planning/phases/06-cross-spine-partial-results/red-evidence": {
		"06-01-helper-swallows-listscopes-error.patch": "TestCrossSpineCoverageThreeStates", // reverts: searchedScopes' failure path degrading into the scopeCoverage zero value instead of {Unknown: true} — the exact fix the roadmap and 06-CONTEXT forbid by name (D-01, D-02, D-03)
	},
}

// gitModuleRoot shells out to `git rev-parse --show-toplevel` rather than
// reusing findModuleRoot's go.mod walk: every operation in this file is a
// git plumbing command (git apply, git diff, git status), and those are
// rooted at the git worktree root, which is not guaranteed to coincide with
// go.mod's directory in every layout.
func gitModuleRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse --show-toplevel: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// patchTouchedFiles parses `diff --git a/X b/Y` header lines out of a
// unified diff to report which repo-root-relative paths a patch will
// mutate. Scoped to header parsing (not a full diff parse) because that is
// all the tree-safety dirty-check needs.
func patchTouchedFiles(t *testing.T, patchPath string) []string {
	t.Helper()
	src, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("read %s: %v", patchPath, err)
	}
	re := regexp.MustCompile(`(?m)^diff --git a/(\S+) b/(\S+)$`)
	matches := re.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatalf("%s: no 'diff --git a/X b/Y' header found — not a unified diff, or a shape this parser does not recognize", patchPath)
	}
	seen := map[string]bool{}
	var files []string
	for _, m := range matches {
		for _, f := range []string{m[1], m[2]} {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	sort.Strings(files)
	return files
}

// gitStatusPorcelain reports `git status --porcelain` for exactly the given
// repo-root-relative paths (never the whole tree — the tree legitimately
// carries unrelated dirty files during this session, and this harness's
// safety contract is scoped to "the files it touches", not "the whole
// tree").
func gitStatusPorcelain(t *testing.T, root string, paths []string) string {
	t.Helper()
	args := append([]string{"status", "--porcelain", "--"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status --porcelain: %v\n%s", err, out)
	}
	return string(out)
}

// runGitApply runs `git apply <extraArgs...> <patchPath>` from root and
// fails the test loudly (not silently) on a non-zero exit, reporting
// combined output for diagnosis.
func runGitApply(t *testing.T, root, patchPath string, extraArgs ...string) (ok bool, output string) {
	t.Helper()
	args := append(append([]string{"apply"}, extraArgs...), patchPath)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return err == nil, string(out)
}

// TestRedEvidencePatchesAreLive is the automated harness. See package doc
// comment above for the full contract. It iterates every phase directory in
// redEvidenceDirs independently: each directory gets its own
// zero-applicability guard and its own set-equality mapping check, so a
// vanished/renamed directory for ONE phase fails loudly without being
// masked by another phase's patches still being present.
func TestRedEvidencePatchesAreLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping red-evidence harness under -short: it is a full mutate/build/test/revert cycle per patch")
	}

	root := gitModuleRoot(t)

	// Empty-map guard. An empty redEvidenceDirs must never be a silent
	// green: iterating zero directories would report clean while proving
	// nothing, which is precisely the vacuity this harness was built to
	// eliminate. It is legitimate only when no milestone is open.
	if len(redEvidenceDirs) == 0 {
		if active := activeMilestonePhaseDirs(t, root); len(active) > 0 {
			t.Fatalf("redEvidenceDirs is empty while %d active-milestone phase director(ies) exist (%s): "+
				"a phase that shipped red-evidence must register it here, or record explicitly that it has none. "+
				"An empty map during an open milestone is a vacuous gate, not a clean run.",
				len(active), strings.Join(active, ", "))
		}
		t.Skip("no milestone is open: a shipped milestone's red-evidence is history, not a live gate (see the SCOPE note on redEvidenceDirs)")
	}

	dirs := make([]string, 0, len(redEvidenceDirs))
	for dir := range redEvidenceDirs {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		dir := dir
		targets := redEvidenceDirs[dir]
		t.Run(dir, func(t *testing.T) {
			// Zero-applicability guard, matching this package's own house
			// style (schemaversion_stamp_gate_test.go): a glob that
			// silently matches nothing must never report clean — a
			// red-evidence directory that vanished or was renamed must
			// fail THIS directory's gate, never silently contribute zero
			// patches to the overall run.
			patches, err := filepath.Glob(filepath.Join(root, dir, "*.patch"))
			if err != nil {
				t.Fatalf("glob %s/*.patch: %v", dir, err)
			}
			if len(patches) == 0 {
				t.Fatalf("glob %s/*.patch matched zero patches — a red-evidence directory that vanished or was renamed must fail this gate, not report clean", dir)
			}
			sort.Strings(patches)

			// Mapping completeness: set equality, both directions, exactly
			// this package's established pattern for classification
			// tables.
			gotNames := map[string]bool{}
			for _, p := range patches {
				gotNames[filepath.Base(p)] = true
			}
			for name := range gotNames {
				if _, ok := targets[name]; !ok {
					t.Errorf("discovered patch %s has no entry in redEvidenceDirs[%s] — every red-evidence patch must be mapped to the target test it proves RED", name, dir)
				}
			}
			for name := range targets {
				if !gotNames[name] {
					t.Errorf("redEvidenceDirs[%s] entry %s has no matching discovered patch — stale mapping entry", dir, name)
				}
			}
			if t.Failed() {
				// Mapping is broken; running the per-patch subtests below
				// against an incomplete/stale mapping would produce
				// confusing secondary failures on top of the real one.
				return
			}

			for _, patchPath := range patches {
				patchPath := patchPath
				name := filepath.Base(patchPath)
				target := targets[name]
				t.Run(name, func(t *testing.T) {
					touched := patchTouchedFiles(t, patchPath)

					// Tree-safety: refuse to run if any file this patch
					// touches is already dirty.
					if dirty := gitStatusPorcelain(t, root, touched); dirty != "" {
						t.Fatalf("refusing to apply %s: files it touches are already dirty:\n%s", name, dirty)
					}

					// 1. The patch applies cleanly at HEAD.
					if ok, out := runGitApply(t, root, patchPath, "--check"); !ok {
						t.Fatalf("git apply --check %s failed — patch is stale or corrupt against current source:\n%s", name, out)
					}
					if ok, out := runGitApply(t, root, patchPath); !ok {
						t.Fatalf("git apply %s failed after --check succeeded (non-deterministic apply):\n%s", name, out)
					}

					// Tree safety: ALWAYS revert, even on a failing
					// assertion or a panic inside this subtest.
					reverted := false
					revert := func() {
						if reverted {
							return
						}
						reverted = true
						if ok, out := runGitApply(t, root, patchPath, "-R"); !ok {
							t.Fatalf("CRITICAL: git apply -R %s failed — working tree may still be mutated, manual `git checkout -- %s` required:\n%s", name, strings.Join(touched, " "), out)
							return
						}
						if dirty := gitStatusPorcelain(t, root, touched); dirty != "" {
							t.Fatalf("CRITICAL: tree not restored after reverting %s — git status --porcelain still shows:\n%s", name, dirty)
						}
					}
					defer revert()
					t.Cleanup(revert)

					// 2. With the patch applied, its mapped target test
					// FAILS. Scoped to ./... (not just ./internal/store/...)
					// because a phase's target test may live in a sibling
					// package (e.g. internal/migrate) — the -run regex
					// still narrows this to exactly the one named function
					// wherever it lives.
					cmd := exec.Command("go", "test", "-run", "^"+regexp.QuoteMeta(target)+"$", "-count=1", "./...")
					cmd.Dir = root
					var out bytes.Buffer
					cmd.Stdout = &out
					cmd.Stderr = &out
					runErr := cmd.Run()
					if runErr == nil {
						t.Fatalf("with %s applied, target test %s PASSED — this red-evidence patch no longer proves the gate RED (stale, or the gate regressed and no longer catches this bypass). go test output:\n%s", name, target, out.String())
					}
					var exitErr *exec.ExitError
					if !errors.As(runErr, &exitErr) {
						t.Fatalf("running target test %s with %s applied errored abnormally (not a test failure): %v\noutput:\n%s", target, name, runErr, out.String())
					}
					t.Logf("confirmed RED: %s applied -> %s failed as expected", name, target)

					// 3. git apply -R restores the tree exactly — invoked
					// here (in addition to the deferred/cleanup revert) so
					// the assertion itself runs inside this subtest's own
					// PASS/FAIL, not only as a cleanup side effect.
					revert()
				})
			}
		})
	}
}

// activeMilestonePhaseDirs returns the .planning/phases entries belonging
// to an OPEN milestone: every subdirectory except the 999.x backlog
// placeholders, which /gsd-review-backlog parks there unplanned.
//
// It exists so TestRedEvidencePatchesAreLive's empty-map guard can tell
// "no milestone is open, so there is nothing to register" apart from "a
// milestone is open and its red-evidence went unregistered". A missing
// .planning/phases fails loudly rather than reading as a closed
// milestone, so a moved or deleted tree can never turn the guard green.
//
// NOTE: internal/keylinks/gate_test.go carries the same predicate for its
// own skip guard. Both are test-only and deliberately not shared across
// the package boundary — internal/store does not otherwise depend on
// internal/keylinks, and a production export used only by tests would be
// a worse trade than twenty duplicated lines. Keep them in sync.
func activeMilestonePhaseDirs(t *testing.T, root string) []string {
	t.Helper()
	phases := filepath.Join(root, ".planning", "phases")
	entries, err := os.ReadDir(phases)
	if err != nil {
		t.Fatalf("activeMilestonePhaseDirs: read %s: %v — the phases directory moved or was deleted; "+
			"an unreadable phases directory must never be mistaken for a closed milestone", phases, err)
	}
	dirs := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "999.") {
			continue
		}
		dirs = append(dirs, e.Name())
	}
	return dirs
}
