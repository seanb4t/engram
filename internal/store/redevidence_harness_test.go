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
// revert cycle per patch (22 patches today, across two phase directories),
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

// redEvidenceDirs maps each phase's red-evidence directory (relative to the
// module root — git's own working directory convention for `git apply`,
// NOT this package's directory) to that phase's own patch-filename ->
// target-test-function mapping. Adding a phase means adding one entry here;
// TestRedEvidencePatchesAreLive iterates every directory independently, so
// a phase's zero-applicability guard and set-equality mapping check never
// leak into another phase's.
var redEvidenceDirs = map[string]map[string]string{
	".planning/phases/02-record-schema-versioning-foundation/red-evidence": {
		"02-02-red-1-bypass.patch":                       "TestEveryPointWriteRoutesThroughPayload",
		"02-02-red-2-stale-classification.patch":         "TestEveryPointWriteRoutesThroughPayload",
		"02-02-red-3-cross-package-client.patch":         "TestQdrantClientIsHeldOnlyByStorePackage",
		"02-03-red-1-toplevel.patch":                     "TestSchemaVersionNeverGatesRecall",
		"02-03-red-2-nested.patch":                       "TestSchemaVersionNeverGatesRecall",
		"02-03-red-3-unclassified-scroll.patch":          "TestRecallEmissionSetIsCompleteAndClassified",
		"02-03-red-4-unclassified-scrollandoffset.patch": "TestRecallEmissionSetIsCompleteAndClassified",
		"02-03-red-5-linkage.patch":                      "TestRecallEmissionSetIsCompleteAndClassified",
		"02-review-wr01-monotonic-max.patch":             "TestPayloadRoundTripsSchemaVersion",
	},
	".planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence": {
		"03-01-red-1-range-only-filter.patch":      "TestBacklogFilterMatchesAbsentAndBelowTarget",
		"03-01-red-2-declared-drift-written.patch": "TestMigrateRefusesNonAdditiveStep",
		"03-02-red-1-nil-rev-accepted.patch":       "TestNewStepPanicsOnNilReversibility",
		"03-02-red-2-contiguity-dropped.patch":     "TestValidateRejectsOrderingAndUniquenessViolations",
		"03-02-red-3-nonstdlib-import.patch":       "TestMigratePackageIsStdlibOnlyLeaf",
		"03-02-red-4-zero-files-scanned.patch":     "TestMigratePackageIsStdlibOnlyLeaf",
		"03-03-red-1-superset-not-equality.patch":  "TestAdditiveOnlyKeySetDiff",
		"03-03-red-2-removal-check-dropped.patch":  "TestAdditiveOnlyKeySetDiff",
		"03-03-red-3-zero-fixtures.patch":          "TestAdditiveOnlyKeySetDiff",
		// 03-04-red-1-persisted-cursor.patch is deliberately absent. It was
		// retired 2026-08-16, not lost: it never drove any test RED, including
		// on the day it was authored — 03-04-SUMMARY.md's key-decisions record
		// "Cycle 1 (persisted cursor) reddened ZERO of the three subtests
		// against the plan's own committed fixtures". The mutation it expresses
		// (persist the scroll cursor across sweep passes) only misbehaves when a
		// below-target record whose id sorts BEFORE an already-advanced cursor
		// arrives mid-sweep, and no test constructs that ordering. Recover it
		// from git history if that test is ever written; see issue #501.
		"03-04-red-2-trust-error-signal.patch":     "TestMigratePartialFailureResume",
		"03-05-red-1-lte-includes-current.patch":   "TestBacklogFilterMatchesAbsentAndBelowTarget",
		"03-05-red-2-midsweep-write-skipped.patch": "TestMigrateConvergesWithoutLock",
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
