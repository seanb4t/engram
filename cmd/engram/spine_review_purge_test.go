// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/surfaces"
)

// purgeFilterScopeRuleSentence reads RulePurgeFilterRequiresScope's declared
// Sentence from the registry -- used only to assert a rejection message
// CONTAINS it, never to duplicate the text.
func purgeFilterScopeRuleSentence(t *testing.T) string {
	t.Helper()
	rule, ok := surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)
	if !ok {
		t.Fatal("surfaces.RulePurgeFilterRequiresScope is not registered")
	}
	return rule.Sentence
}

// TestParsePurgeClasses validates --class against the enumerated
// vocabulary: legal values pass through unchanged, an illegal value is
// rejected as a usage error naming the legal set.
func TestParsePurgeClasses(t *testing.T) {
	got, err := parsePurgeClasses([]string{"superseded", "archived"})
	if err != nil {
		t.Fatalf("parsePurgeClasses: %v", err)
	}
	want := []store.PurgeClass{store.PurgeClassSuperseded, store.PurgeClassArchived}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePurgeClasses = %v, want %v", got, want)
	}

	if _, err := parsePurgeClasses([]string{"bogus"}); err == nil {
		t.Fatal("parsePurgeClasses([\"bogus\"]) = nil error, want a usage error")
	} else if exitCodeFromError(err) != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", exitCodeFromError(err), exitUsage)
	}
}

// TestRequirePurgeFilterScope pins D-10's harder filter-path gate: a
// structural-class-only run (even with --older-than as that class's window
// override) needs no --scope; the free-form filter path does.
func TestRequirePurgeFilterScope(t *testing.T) {
	cases := []struct {
		name    string
		opts    store.PurgeOptions
		wantErr bool
	}{
		{"no classes, no filter, no scope", store.PurgeOptions{}, false},
		{"class only, no scope", store.PurgeOptions{Classes: []store.PurgeClass{store.PurgeClassArchived}}, false},
		{"class plus older-than override, no scope", store.PurgeOptions{Classes: []store.PurgeClass{store.PurgeClassArchived}, OlderThan: time.Hour}, false},
		{"category with no scope", store.PurgeOptions{Category: "decision"}, true},
		{"tags with no scope", store.PurgeOptions{Tags: []string{"x"}}, true},
		{"older-than alone (no class), no scope", store.PurgeOptions{OlderThan: time.Hour}, true},
		{"category with scope", store.PurgeOptions{Category: "decision", Scope: "s"}, false},
		{"category with all-scopes", store.PurgeOptions{Category: "decision", AllScopes: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requirePurgeFilterScope(tc.opts)
			if tc.wantErr && err == nil {
				t.Fatal("requirePurgeFilterScope: want an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("requirePurgeFilterScope: want nil, got %v", err)
			}
			if tc.wantErr {
				if !strings.Contains(err.Error(), purgeFilterScopeRuleSentence(t)) {
					t.Errorf("error = %q, want it to contain the registered rule Sentence", err.Error())
				}
				if exitCodeFromError(err) != exitUsage {
					t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", exitCodeFromError(err), exitUsage)
				}
			}
		})
	}
}

// TestSpineReviewPurgeNoTransportFlag asserts NEITHER a "manifest" NOR a
// "token" flag is reachable -- local or inherited -- so an operator cannot
// pass one and believe it was honoured. The manifest is in-process only
// (03-07-PLAN.md's settled transport decision).
func TestSpineReviewPurgeNoTransportFlag(t *testing.T) {
	for _, name := range []string{"manifest", "token"} {
		if f := spineReviewPurgeCmd.Flags().Lookup(name); f != nil {
			t.Errorf("spineReviewPurgeCmd.Flags().Lookup(%q) = %v, want nil", name, f)
		}
		if f := spineReviewPurgeCmd.InheritedFlags().Lookup(name); f != nil {
			t.Errorf("spineReviewPurgeCmd.InheritedFlags().Lookup(%q) = %v, want nil", name, f)
		}
	}
}

// TestSpineReviewPurgeOwnFlagSet asserts the leaf's own registered flag set
// equals the same nine-name literal destructive_test.go's
// mutatingFlagCases table uses (renamed from destructiveFlagCases in 04-04
// Task 2), read via ownFlagNames -- one shared helper, never two
// independently typed lists that could silently diverge.
func TestSpineReviewPurgeOwnFlagSet(t *testing.T) {
	want := []string{"all-scopes", "apply", "category", "class", "older-than", "output", "scope", "tags", "timeout"}
	sort.Strings(want)
	got := ownFlagNames(spineReviewPurgeCmd)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spineReviewPurgeCmd own flags = %v, want %v", got, want)
	}
}

// TestSpineReviewPurgeSameRunNoticePublished asserts BOTH the preview
// document and the command's Long help state the same-run limitation and
// the intersection's concurrent-writer scoping, matched on the SAME
// package-level constants the CLI guide's prose mirrors by hand. The notice
// moved from the preview SENTENCE into purgeReportDoc's own fields when
// purgePreviewSummary was trimmed to a headline producer (06-01-PLAN.md R1);
// asserting it against the document is the stronger claim -- the notice is
// a document key, not text-only prose, so it also reaches the json lane.
func TestSpineReviewPurgeSameRunNoticePublished(t *testing.T) {
	doc := purgePreviewDoc(nil, store.PurgeOptions{})
	if doc.SameRunLimitation != purgeSameRunLimitationNotice {
		t.Errorf("purgePreviewDoc(...).SameRunLimitation = %q, want %q", doc.SameRunLimitation, purgeSameRunLimitationNotice)
	}
	if doc.IntersectionScope != purgeIntersectionScopingNotice {
		t.Errorf("purgePreviewDoc(...).IntersectionScope = %q, want %q", doc.IntersectionScope, purgeIntersectionScopingNotice)
	}
	for _, notice := range []string{purgeSameRunLimitationNotice, purgeIntersectionScopingNotice} {
		if !strings.Contains(spineReviewPurgeCmd.Long, notice) {
			t.Errorf("spineReviewPurgeCmd.Long = %q, want it to contain %q", spineReviewPurgeCmd.Long, notice)
		}
	}
}

// TestSpineReviewPurgeRejectsInvalidOutput mirrors every other spine-review
// leaf's --output validation test.
func TestSpineReviewPurgeRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, spineReviewPurgeCmd)
	_, _, err := runClient(t, "spine-review", "purge", "--class", "expired", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewPurgeFilterPathRequiresScope proves the rejection fires
// BEFORE any store dial (a category filter with no --scope must fail even
// with no Qdrant reachable).
func TestSpineReviewPurgeFilterPathRequiresScope(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, spineReviewPurgeCmd)
	_, _, err := runClient(t, "spine-review", "purge", "--category", "decision")
	if err == nil {
		t.Fatal("expected an error for --category with no --scope, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
	if !strings.Contains(err.Error(), purgeFilterScopeRuleSentence(t)) {
		t.Errorf("error = %q, want it to contain the registered rule Sentence", err.Error())
	}
}

// TestSpineReviewPurgeClassOnlyDoesNotRequireScope proves a class-only
// invocation passes the pre-flight scope gate (the subsequent store dial
// then fails against an unreachable Qdrant, which is a DIFFERENT,
// unavailable-class error -- proving the scope gate itself did not fire).
func TestSpineReviewPurgeClassOnlyDoesNotRequireScope(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, spineReviewPurgeCmd)
	t.Setenv("ENGRAM_QDRANT_ADDR", "127.0.0.1:1")
	t.Setenv("ENGRAM_EMBED_DIM", "3")
	_, _, err := runClient(t, "spine-review", "purge", "--class", "expired", "--timeout", "1s")
	if err == nil {
		t.Fatal("expected a transport error (no Qdrant reachable), got nil")
	}
	if got := exitCodeFromError(err); got == exitUsage {
		t.Errorf("exitCodeFromError(err) = %d (exitUsage), want a transport/unavailable code -- "+
			"a class-only run must not trip the filter-path scope gate", got)
	}
}

// TestSpineReviewPurgeClassTagsDoNotLatchAcrossRows is the CR-01-shaped
// regression case: two rows supplying DIFFERENT --class/--tags values must
// each see only their own -- proving the two repeatable stringSlice flags
// do not latch across table rows.
//
// This is a MUTATION CHECK (inject-and-revert), not RED-first: this task
// adds the nil-list entries and this regression case together, so the
// latching failure state never arises naturally in task order. See the
// plan SUMMARY for the observed injected-defect failure line.
func TestSpineReviewPurgeClassTagsDoNotLatchAcrossRows(t *testing.T) {
	runRow := func(t *testing.T, class, tag string) {
		t.Helper()
		resetClientFlags(t)
		resetCommandFlagState(t, spineReviewPurgeCmd)
		args := []string{"spine-review", "purge", "--class", class, "--tags", tag, "--output", "yaml"}
		if _, _, err := runClient(t, args...); err == nil {
			t.Fatal("expected an error (--output yaml is always invalid), got nil")
		}
	}
	t.Run("row-a", func(t *testing.T) { runRow(t, "superseded", "row-a-tag") })
	t.Run("row-b", func(t *testing.T) {
		runRow(t, "expired", "row-b-tag")
		if len(spinePurgeClass) != 1 || spinePurgeClass[0] != "expired" {
			t.Errorf("spinePurgeClass = %v after row 2, want [\"expired\"] only -- row 1's --class value leaked into row 2", spinePurgeClass)
		}
		if len(spinePurgeTags) != 1 || spinePurgeTags[0] != "row-b-tag" {
			t.Errorf("spinePurgeTags = %v after row 2, want [\"row-b-tag\"] only -- row 1's --tags value leaked into row 2", spinePurgeTags)
		}
	})
}

// TestPurgeReportDocFieldsNeverNull proves purgePreviewDoc/purgeAppliedDoc
// keep every id-list field non-nil (marshals "[]", never "null") and carry
// the same three count fields plus three id lists in both modes. It also
// proves the R2 gap-closure field's mode split: rerun is populated on the
// preview document only, and absent (empty, so omitempty drops the key)
// from the applied document -- an applied run has nothing left to re-run.
func TestPurgeReportDocFieldsNeverNull(t *testing.T) {
	opts := store.PurgeOptions{Classes: []store.PurgeClass{store.PurgeClassExpired}, Scope: "s"}

	preview := purgePreviewDoc(nil, opts)
	if preview.Applied {
		t.Error("purgePreviewDoc.Applied = true, want false")
	}
	if preview.Deleted == nil || preview.Spared == nil || preview.Appeared == nil || preview.Eligible == nil {
		t.Errorf("purgePreviewDoc has a nil id-list field: %+v", preview)
	}
	if preview.Rerun == "" {
		t.Error("purgePreviewDoc.Rerun is empty, want the re-run command")
	}
	if want := purgeRerunCommand(opts); preview.Rerun != want {
		t.Errorf("purgePreviewDoc.Rerun = %q, want %q", preview.Rerun, want)
	}

	applied := purgeAppliedDoc(nil, store.PurgeResult{}, opts)
	if !applied.Applied {
		t.Error("purgeAppliedDoc.Applied = false, want true")
	}
	if applied.Deleted == nil || applied.Spared == nil || applied.Appeared == nil || applied.Eligible == nil {
		t.Errorf("purgeAppliedDoc has a nil id-list field: %+v", applied)
	}
	if applied.Rerun != "" {
		t.Errorf("purgeAppliedDoc.Rerun = %q, want empty (an applied run has nothing left to re-run)", applied.Rerun)
	}
}

// TestPurgeAppliedSummaryNamesAppearedExplicitly proves the headline names
// the appeared count with its own explicit "not purged" wording, never
// merged silently into the deleted count (T-03-22's mitigation) -- this
// explanatory nuance is exactly what D-04 says a field name cannot carry,
// so it stays in the hand-written headline even after R1's per-row trim.
func TestPurgeAppliedSummaryNamesAppearedExplicitly(t *testing.T) {
	res := store.PurgeResult{Deleted: []string{"d1"}, Spared: []string{"s1"}, Appeared: []string{"a1"}}
	got := purgeAppliedSummary(res, store.PurgeOptions{})
	for _, want := range []string{"1 deleted", "1 spared", "1 appeared", "NOT purged; re-run to include"} {
		if !strings.Contains(got, want) {
			t.Errorf("purgeAppliedSummary(...) = %q, want it to contain %q", got, want)
		}
	}
}

// TestPurgeAppliedViewRendersAppearedRow proves the per-id "not purged"
// wording -- deleted from purgeAppliedSummary in this task -- is rendered
// by the operator view from purgeAppliedDoc's appeared field.
func TestPurgeAppliedViewRendersAppearedRow(t *testing.T) {
	res := store.PurgeResult{Deleted: []string{"d1"}, Spared: []string{"s1"}, Appeared: []string{"a1"}}
	opts := store.PurgeOptions{}
	doc := purgeAppliedDoc(nil, res, opts)

	var buf bytes.Buffer
	if err := renderOperatorView(&buf, purgeAppliedSummary(res, opts), doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"d1", "s1", "a1"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered view = %q, want it to contain %q", out, want)
		}
	}
}

// TestShellQuoteRoundTripsThroughARealShell proves shellQuote's escaping is
// not merely plausible-looking: piping the quoted output through an actual
// `sh -c` invocation recovers the exact original string, for a value
// containing a space and a value containing a single quote (WR-03,
// 06-REVIEW.md) -- the two shapes purgeRerunCommand's own doc comment
// names as the ones a copy-paste would otherwise silently mis-split.
func TestShellQuoteRoundTripsThroughARealShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh on PATH")
	}
	cases := []string{
		"plain",
		"has a space",
		"has'a'quote",
		"O'Brien's scope",
		"",
		"line one\nline two",
	}
	for _, want := range cases {
		t.Run(want, func(t *testing.T) {
			quoted := shellQuote(want)
			out, err := exec.Command("sh", "-c", "printf '%s' "+quoted).Output()
			if err != nil {
				t.Fatalf("sh -c: %v", err)
			}
			if got := string(out); got != want {
				t.Errorf("shellQuote(%q) = %q; sh round-trip = %q, want %q", want, quoted, got, want)
			}
		})
	}
}

// TestPurgeRerunCommandQuotesFreeFormValues proves purgeRerunCommand
// shell-quotes --scope, --category, and --tags (the free-form values an
// operator can set on the write path) so a value containing a space or a
// single quote does not silently split into extra/different arguments if
// the rendered command is copy-pasted into a shell (WR-03, 06-REVIEW.md).
// Verified by actually splitting the rendered command through a real `sh`
// (via `set --`) rather than by string-matching the quoting scheme.
func TestPurgeRerunCommandQuotesFreeFormValues(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh on PATH")
	}
	opts := store.PurgeOptions{
		Classes:  []store.PurgeClass{store.PurgeClassExpired},
		Scope:    "has a space",
		Category: "o'brien",
		Tags:     []string{"tag one", "tag'two"},
	}
	got := purgeRerunCommand(opts)

	// `set -- $got; for a in "$@"; do printf '%s\n' "$a"; done` re-parses
	// the rendered command line through sh's own word-splitting/quote
	// rules and prints one line per resulting argument -- the ground truth
	// for "what would this copy-paste actually run as".
	script := "set -- " + got + "; for a in \"$@\"; do printf '%s\\n' \"$a\"; done"
	out, err := exec.Command("sh", "-c", script).Output()
	if err != nil {
		t.Fatalf("sh -c: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(out), "\n"), "\n")

	want := []string{
		"engram", "spine-review", "purge", "--apply",
		"--class", "expired",
		"--scope", "has a space",
		"--category", "o'brien",
		"--tags", "tag one",
		"--tags", "tag'two",
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("rendered command %q re-parsed by sh as %v, want %v (a free-form value with a space or quote must survive a copy-paste unchanged)", got, args, want)
	}
}
