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
	// Milestone 2026-09-13.01, Phase 01 (01-executor-correctness-man-pages):
	// two independent regressions this phase's SUMMARYs claim TDD RED
	// against, pinned as live evidence rather than narrated history.
	//
	// osRun/runSeam (01-01, environment.go/apply.go): GitHub #560's
	// misclassification — a context-killed child satisfies
	// errors.As(runErr, &exitErr) exactly like any other abnormal exit, so
	// dropping the ctx.Err() check-before-unwrap silently turns a
	// deadline-killed/canceled subprocess back into a clean nonzero exit
	// (ExitCode: -1, nil error) instead of the "never got an answer" error
	// Environment.Run's contract promises. The second patch pins the
	// operator-facing wording runSeam wraps that error in
	// ("timed out after 20s: …") — losing it still returns an error, but
	// silently regresses D-11's legibility contract for the row an
	// operator actually reads.
	//
	// man.go (01-02): the two invariants D-01/D-03/D-04's byte-stability
	// and cobra/doc's own help-grafting side effect depend on. Un-pinning
	// manDate from the Unix epoch reintroduces the exact machine/date
	// dependence D-01 exists to eliminate (README's "Jan 1970" promise).
	// Dropping the snapshotCommandTree/pruneToSnapshot restore lets
	// cobra/doc's genMan graft a "help" child onto every subgroup it
	// renders (migrate, spine-review, completion) — a child a real
	// `engram <group> --help` never lists, and the regression writeManPages'
	// own doc comment names as the reason this restore exists.
	".planning/phases/01-executor-correctness-man-pages/red-evidence": {
		"01-01-osrun-ctx-err-first.patch":     "TestOsRunReportsContextDeadlineExceeded",
		"01-01-runseam-timeout-wording.patch": "TestDriftReportedLegibly",
		"01-02-man-header-pinned.patch":       "TestManPagesByteStable",
		"01-02-man-tree-restore.patch":        "TestManGenerationLeavesCommandTreeUnchanged",
	},
	// Milestone 2026-09-13.01, Phase 02 (02-custom-auth-headers): five
	// independent regressions this phase's SUMMARYs claim RED against
	// (02-02-SUMMARY.md D5, 02-03-SUMMARY.md D4), pinned as live evidence
	// rather than narrated history.
	//
	// claudeCodeHeaderArgs / openCodeHeaderArgs (02-01, 02-02): each
	// runtime authors its OWN header-flag dialect for the "--header"
	// argument that rides alongside `mcp add` (claude-code's
	// "NAME: ${ENVVAR}" colon-space HTTP-header-string form vs opencode's
	// "NAME={env:ENVVAR}" KEY=VALUE form — 02-RESEARCH.md's live-verified
	// finding that treating them as interchangeable silently sends
	// opencode the wrong dialect, the exact regression 02-02 exists to
	// avoid). The claude-code patch drops rendering entirely (proving the
	// extra-header args are wired in at all); the opencode patch swaps in
	// claude-code's colon-space form (proving the dialect stays
	// opencode's own, never borrowed).
	//
	// codexRuntime.Plan's up-front header guard (02-01): codex's `mcp add`
	// exposes no custom-header flag at all, so a caller-supplied header
	// must be DECLINED via ErrHeaderUnsupported before any other work —
	// removing the guard lets Plan silently accept and drop the header,
	// which is worse than an error (D-09/D-10's whole reason for a
	// distinct sentinel from ErrAuthModeUnsupported).
	//
	// genericHeaders.MarshalJSON (02-02): D-08's ordering contract
	// (Authorization first, then extras sorted case-insensitively) is the
	// entire reason this named map type exists instead of a plain
	// map[string]string, which encoding/json would marshal in raw byte
	// order — falling back to that plain marshaling reproduces exactly
	// the ordering bug this type was introduced to fix.
	//
	// setupParseHeaders' Authorization collision guard (02-03): --header
	// Authorization=... must be rejected as a CLI usage error naming
	// --auth bearer, per spec's fixed rejection order (rule 1 of 4) —
	// dropping it lets a caller silently author a header that COLLIDES
	// with whatever --auth bearer already produces, with no error at the
	// boundary that owns validation.
	".planning/phases/02-custom-auth-headers/red-evidence": {
		"02-01-claudecode-header-args.patch":  "TestClaudeCodeHeaders",
		"02-01-codex-header-decline.patch":    "TestCodexDeclinesHeaders",
		"02-02-opencode-header-dialect.patch": "TestOpenCodeHeaders",
		"02-02-generic-header-order.patch":    "TestGenericHeaders",
		"02-03-header-validation.patch":       "TestSetupHeaderRejectsAuthorizationCollision",
	},
	// Milestone 2026-09-13.01, Phase 03 (03-plugin-first-delivery): five
	// independent regressions this phase's SUMMARYs claim RED against
	// (each 03-0N-SUMMARY.md's own red-evidence section), pinned as live
	// evidence rather than narrated history.
	//
	// classifyPluginVersion's never-downgrade arm (03-01, plugin.go): D-01's
	// whole point is that a plugin newer than the resolved binary is
	// reported PluginCurrent with an explanatory note and is NEVER treated
	// as an update target — flipping the comparator's greater-than arm to
	// PluginOutdated silently reintroduces exactly the downgrade-on-newer
	// bug D-01 exists to forbid.
	//
	// claude-code's PluginActions default arm (03-01, claudecode.go): a
	// PluginCurrent (or PluginUnavailable) classification must author ZERO
	// actions — REQ-plugin-three-way-state's whole reason a "current" state
	// exists separately from "outdated" is to make idempotent re-runs a
	// true no-op; authoring an update action here would churn a working
	// install on every --apply.
	//
	// codex's PluginActions outdated arm (03-01, codex.go): D-02's
	// remove-then-add sequencing exists because codex's own `plugin add`
	// has no update semantics over an existing entry — collapsing the
	// two-action sequence to a bare add silently drops the remove step
	// this file's own doc comment names as the reason D-02 requires it.
	//
	// DetectPresence's per-entry symlink classification (03-02,
	// presence.go): REQ-plugin-skips-skills-copy's report-only contract
	// depends on correctly distinguishing a maintainer's own symlinked
	// skill entry from an ordinary directory — checking the wrong file-mode
	// bit (ModeDir instead of ModeSymlink) misclassifies both directions at
	// once, exactly the report-only contract's failure mode.
	//
	// setupRuntimeRowFromResult's plugin/native routing (03-03, setup.go):
	// D-07's routing predicate is p.Delivered() — a plugin-capable,
	// currently-delivered runtime must author ZERO native skills writes,
	// ever, in the same run. Inverting that one condition silently routes
	// every plugin-delivered runtime through the native skills.Install
	// path, reproducing REQ-plugin-skips-skills-copy's exact double-write
	// defect this phase exists to prevent.
	".planning/phases/03-plugin-first-delivery/red-evidence": {
		"03-01-plugin-version-compare.patch": "TestPluginVersionCompare",
		"03-01-plugin-state-machine.patch":   "TestPluginPlan",
		"03-01-codex-remove-then-add.patch":  "TestPluginPlan",
		"03-02-detect-presence.patch":        "TestDetectPresence",
		"03-03-plugin-skips-native.patch":    "TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites",
	},
	// Milestone 2026-09-13.01, Phase 04 (04-drift-detection-read-only): nine
	// independent regressions this phase's own red-evidence claims pin as
	// live, spanning D-04 (OutcomePreserved is a non-failed attempt, never
	// laundered into failure), D-03 (Result.Registered is always REBUILT
	// from the parsed-and-redacted Observation, never a raw probe capture),
	// D-11 (a runtime's Observe is a TOTAL parse — unaccounted content is
	// always reported, never silently tolerated), D-12 (skills/plugin
	// facets and their outcomes ride on every rendered row, never dropped
	// silently), and REQ-drift-redaction (an observed secret or third-party
	// literal must never reach a rendered field, unconditionally).
	//
	// exit.go/aggregate.go (04-01): Classify's non-failed-attempt case and
	// AggregateOutcome's precedenceOrder both name OutcomePreserved
	// explicitly (D-04) — dropping it from either falls a preserved
	// registration through to the failure arm/default in one of the two
	// classifiers, silently reporting "setup performing correctly by
	// declining to destroy something it cannot re-create" as a failure.
	//
	// apply.go (04-01): the !mutate branch's res.Registered MUST be
	// rebuilt from the parsed-and-redacted Observation (renderObservation),
	// never the raw probe1 capture (D-03) — reverting to the old raw
	// capture reopens exactly the secret-leak path D-03 was authored to
	// close, caught here by TestRedactionUnconditional's codex-observed-
	// literal fixture.
	//
	// codex.go Observe (04-01, 04-05): two independent totality signals
	// this file's own doc comment names — the key-set diff (c) and the
	// DisallowUnknownFields totality gate (d) — must both keep reporting
	// content D-11 has no vocabulary for; the header mapping additionally
	// turns an observed "http_headers" key engram never planned into an
	// unplanned/preserved facet rather than silent drop. Removing the
	// key-diff's append lets an unrecognized top-level/transport key go
	// unreported (the totality gate's generic fallback label replaces the
	// specific field name, breaking the exhaustive per-field assertions);
	// removing the http_headers-to-observedHeaders mapping makes an
	// observed literal header vanish from Observation.Headers entirely
	// instead of surfacing as HeaderUnplanned.
	//
	// claudecode.go Observe (04-05): the "Status:" line must classify as
	// chrome (Pitfall 4: live connection state must never affect
	// classification) — reclassifying it as unrecognized content churns
	// Unrecognized on every dial-state fluctuation, which
	// status-failed-dial-is-chrome pins against. Separately, an observed
	// custom-header VALUE must never survive past the joinHeaders
	// comparison into any retained field (D-02/D-03) — appending it to
	// Unrecognized alongside its redacted ObservedHeader entry reopens the
	// same secret-leak class D-03 exists to close, this time via
	// claude-code's own header block rather than codex's JSON parse.
	".planning/phases/04-drift-detection-read-only/red-evidence": {
		"04-01-preserved-not-in-classify.patch":      "TestClassifyExhaustiveOutcomeCombinations",
		"04-01-precedence-slot.patch":                "TestAggregateOutcomeExhaustive",
		"04-01-registered-raw-capture.patch":         "TestRedactionUnconditional",
		"04-01-codex-tolerant-decode.patch":          "TestObserveCodexRegistration",
		"04-04-facets-not-copied.patch":              "TestSetupPreviewJSONCarriesDriftFacets",
		"04-04-apply-summary-preserved.patch":        "TestSetupApplySummaryCountsPreserved",
		"04-05-claudecode-status-facet.patch":        "TestObserveClaudeCodeRegistration",
		"04-05-claudecode-value-retained.patch":      "TestRedactionUnconditional",
		"04-05-codex-literal-header-tolerated.patch": "TestObserveCodexRegistration",
	},
	// Milestone 2026-09-13.01, Phase 05 (05-apply-time-preserve-gate-documentation):
	// ten independent regressions this phase's own red-evidence claims pin as
	// live, spanning D-01 (already-correct and preserved issue ZERO
	// registration-write actions under --apply, checked BEFORE Plan.Actions'
	// loop runs even once — Claude Code's tolerant `mcp remove` included),
	// D-02 (a post-write Registered is always REBUILT through Observe ->
	// renderObservation, never the raw probe2 capture), D-03/D-04 (the OAuth
	// re-login RewriteConsequence fires by observed SHAPE alone — no
	// Authorization/bearer header at all — never unconditionally), D-05
	// (each runtime authors its OWN ManualRemediation sentence for a
	// preserved row; the shared executor only relays it, never composes or
	// substitutes another runtime's), D-07 (plugin-first delivery is
	// documented alongside the apply gate and the preserved cross-link),
	// REQ-apply-preserve-gate (the operator-facing guarantee that --apply
	// compares before writing), REQ-apply-rewrite-consequence (the OAuth
	// re-login note's Pitfall-4 shape gating), and REQ-docs-setup-v2 (the
	// install/plugin/agent-setup guides document the shipped v2 setup
	// contract, including the man page and preserved-row remediation).
	//
	// apply.go (05-01): the compared-outcome short-circuit for
	// OutcomeAlreadyCorrect/OutcomePreserved must return BEFORE
	// plan.Actions' loop runs even once — moving the check to fire only
	// after the first iteration lets Claude Code's tolerant `mcp remove`
	// (plan.Actions[0] for every claude-code auth mode) actually run against
	// a preserved or already-correct registration, reopening the exact
	// destructive-window incident class D-01 exists to close. Two
	// independent patches pin this: one moves the check inside the loop
	// (preserved), one drops OutcomeAlreadyCorrect from the case entirely
	// (already-correct). A third patch pins D-02 by reverting the post-write
	// Registered rebuild to the raw, unredacted probe2 capture — the exact
	// secret-leak class Phase 4's D-03 closed, reopened here on the
	// apply-time re-read path specifically.
	//
	// claudecode.go Observe (05-01): the OAuth re-login RewriteConsequence
	// must fire by SHAPE alone (no observed Authorization/bearer header) —
	// setting it unconditionally regresses Pitfall 4: a bearer- or
	// foreign-Authorization-shaped registration has no OAuth session to
	// lose, so the note becomes actively misleading on every such row.
	//
	// apply.go/codex.go (05-01): D-05's ManualRemediation is AUTHORED-HERE
	// per runtime — hard-coding claude-code's remediation sentence into the
	// shared renderClassification (and blanking codex's own constant)
	// silently swaps codex's correct manual step for claude-code's,
	// misdirecting an operator clearing a preserved codex registration.
	//
	// setup.go (05-02): deleting the apply-gate sentences from
	// setupLongDescription regresses REQ-apply-preserve-gate's own
	// documentation success criterion — the CLI's --help text must state
	// the same before-writing/no-registration-command/mcp-remove/runtime's-
	// own-tool/log-in-again guarantee TestSetupHelpStatesApplyGate pins.
	//
	// agent-setup.md (05-02): two independent regressions in the drift
	// guide's results table — reinserting the stale "does not guarantee
	// that no write commands ran" sentence onto the already-correct row (a
	// claim the apply-time preserve gate makes false, REQ-apply-preserve-
	// gate), and dropping the preserved row's claude-code manual-remediation
	// clause (`claude mcp remove engram --scope user`, D-05) while leaving
	// codex's own remediation intact.
	//
	// install.md (05-03) and plugin.md (05-03): REQ-docs-setup-v2's
	// cask-contents and plugin-first documentation — deleting the `man
	// engram-setup` sentence from the install guide, and deleting the
	// preserved-row cross-link line from the plugin guide (D-07: a
	// preserved registration's remediation is documented once, pointed at
	// from the plugin guide, never restated or silently dropped).
	".planning/phases/05-apply-time-preserve-gate-documentation/red-evidence": {
		"05-01-preserved-flag-inside-loop.patch":       "TestApplyPreservedNeverRunsClaudeCodeRemove",
		"05-01-already-correct-still-writes.patch":     "TestApplyAlreadyCorrectIssuesZeroWrites",
		"05-01-registered-raw-probe2.patch":            "TestApplyWroteRegisteredIsRedacted",
		"05-01-oauth-note-on-bearer.patch":             "TestOAuthReLoginConsequence",
		"05-01-remediation-composed-in-executor.patch": "TestPreviewClassifiesRegistration",
		"05-02-help-drops-apply-gate.patch":            "TestSetupHelpStatesApplyGate",
		"05-02-guide-stale-guarantee.patch":            "TestAgentSetupGuideDocumentsDrift",
		"05-02-guide-preserved-no-remediation.patch":   "TestAgentSetupGuideDocumentsDrift",
		"05-03-install-drops-man-page.patch":           "TestInstallGuideDocumentsSetupV2",
		"05-03-plugin-drops-preserved-crosslink.patch": "TestPluginGuideDocumentsPluginFirst",
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
