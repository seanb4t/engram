// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// spineConsolidateFakeStore is a recording fake satisfying
// spineConsolidateStore, letting these tests observe exactly which
// store.NearDuplicateOptions a cobra invocation produced -- and inject a
// canned result -- without dialing a live Qdrant. Also invokes
// opts.Progress (when set) with its own configured counts, so the
// stderr-wiring test can prove the RunE plumbs the callback through
// without needing a real sweep.
type spineConsolidateFakeStore struct {
	called                           bool
	gotOpts                          store.NearDuplicateOptions
	pairs                            []store.DuplicatePair
	err                              error
	progressScanned, progressQueried uint64
}

func (f *spineConsolidateFakeStore) NearDuplicates(_ context.Context, opts store.NearDuplicateOptions) ([]store.DuplicatePair, error) {
	f.called = true
	f.gotOpts = opts
	if opts.Progress != nil {
		opts.Progress(f.progressScanned, f.progressQueried)
	}
	return f.pairs, f.err
}

// withFakeConsolidateStore substitutes spineConsolidateStoreFromEnv with
// one that returns fake, restoring the real constructor via t.Cleanup.
func withFakeConsolidateStore(t *testing.T, fake *spineConsolidateFakeStore) {
	t.Helper()
	orig := spineConsolidateStoreFromEnv
	spineConsolidateStoreFromEnv = func() (spineConsolidateStore, error) { return fake, nil }
	t.Cleanup(func() { spineConsolidateStoreFromEnv = orig })
}

// TestSpineReviewConsolidateRejectsInvalidOutput mirrors
// TestSpineReviewScanRejectsInvalidOutput: an illegal --output value exits
// exitUsage before any store is ever dialed (no fake needed).
func TestSpineReviewConsolidateRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewConsolidateAllScopesMapsToOptions proves --all-scopes
// maps to NearDuplicateOptions.AllScopes: true with Scope left "" --
// NEVER encoded as a non-empty Scope string standing in for "all scopes".
func TestSpineReviewConsolidateAllScopesMapsToOptions(t *testing.T) {
	resetClientFlags(t)
	fake := &spineConsolidateFakeStore{}
	withFakeConsolidateStore(t, fake)

	if _, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json"); err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if !fake.called {
		t.Fatal("NearDuplicates was never called")
	}
	if !fake.gotOpts.AllScopes {
		t.Error("gotOpts.AllScopes = false, want true")
	}
	if fake.gotOpts.Scope != "" {
		t.Errorf("gotOpts.Scope = %q, want \"\" -- --all-scopes must never be represented as a non-empty Scope", fake.gotOpts.Scope)
	}
}

// TestSpineReviewConsolidateScopeAndAllScopesRejected proves --scope and
// --all-scopes together are rejected via cobra's own
// MarkFlagsMutuallyExclusive validation, exiting exitUsage BEFORE the
// store is ever dialed -- the fake's called flag stays false.
func TestSpineReviewConsolidateScopeAndAllScopesRejected(t *testing.T) {
	resetClientFlags(t)
	fake := &spineConsolidateFakeStore{}
	withFakeConsolidateStore(t, fake)

	_, _, err := runClient(t, "spine-review", "consolidate", "--scope", "x", "--all-scopes")
	if err == nil {
		t.Fatal("expected an error for --scope and --all-scopes together, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
	if fake.called {
		t.Error("NearDuplicates was called despite the mutually-exclusive flag violation")
	}
}

// TestSpineReviewConsolidateMinScoreDefValueIsEmptyString reads --min-score's
// DefValue from the LIVE cobra tree: the no-filter default must be
// observable as the empty string, not merely documented in prose.
func TestSpineReviewConsolidateMinScoreDefValueIsEmptyString(t *testing.T) {
	f := spineReviewConsolidateCmd.Flags().Lookup("min-score")
	if f == nil {
		t.Fatal("no --min-score flag registered on spine-review consolidate")
	}
	if f.DefValue != "" {
		t.Errorf("--min-score DefValue = %q, want \"\" (absent means no filter)", f.DefValue)
	}
}

// TestSpineReviewConsolidateMinScoreOptionNilWhenAbsent proves the parsed
// option is nil when --min-score is absent and non-nil when supplied --
// the no-filter state is observable through the actual flag-to-Options
// mapping, not just parseMinScore in isolation.
func TestSpineReviewConsolidateMinScoreOptionNilWhenAbsent(t *testing.T) {
	resetClientFlags(t)
	fake := &spineConsolidateFakeStore{}
	withFakeConsolidateStore(t, fake)

	if _, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json"); err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if fake.gotOpts.MinScore != nil {
		t.Errorf("gotOpts.MinScore = %v, want nil when --min-score is absent", fake.gotOpts.MinScore)
	}

	resetClientFlags(t)
	if _, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--min-score", "0.75", "--output", "json"); err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if fake.gotOpts.MinScore == nil {
		t.Fatal("gotOpts.MinScore = nil, want a pointer when --min-score is supplied")
	}
	if *fake.gotOpts.MinScore != 0.75 {
		t.Errorf("*gotOpts.MinScore = %v, want 0.75", *fake.gotOpts.MinScore)
	}
}

// TestParseMinScore covers parseMinScore in isolation: empty means nil, a
// valid number parses, and an invalid value returns a usage error.
func TestParseMinScore(t *testing.T) {
	got, err := parseMinScore("")
	if err != nil {
		t.Fatalf(`parseMinScore(""): %v`, err)
	}
	if got != nil {
		t.Errorf(`parseMinScore("") = %v, want nil`, got)
	}

	got, err = parseMinScore("0.5")
	if err != nil {
		t.Fatalf(`parseMinScore("0.5"): %v`, err)
	}
	if got == nil || *got != 0.5 {
		t.Errorf(`parseMinScore("0.5") = %v, want a pointer to 0.5`, got)
	}

	_, err = parseMinScore("not-a-number")
	if err == nil {
		t.Fatal(`parseMinScore("not-a-number") = nil error, want a usage error`)
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(parseMinScore error) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewConsolidateTopKDefaultAndOverride proves --top-k flows
// through to NearDuplicateOptions.TopK, both at its default and when
// overridden.
func TestSpineReviewConsolidateTopKDefaultAndOverride(t *testing.T) {
	resetClientFlags(t)
	fake := &spineConsolidateFakeStore{}
	withFakeConsolidateStore(t, fake)

	if _, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json"); err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if fake.gotOpts.TopK != spineConsolidateDefaultTopK {
		t.Errorf("default gotOpts.TopK = %d, want %d", fake.gotOpts.TopK, spineConsolidateDefaultTopK)
	}

	resetClientFlags(t)
	if _, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--top-k", "10", "--output", "json"); err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if fake.gotOpts.TopK != 10 {
		t.Errorf("gotOpts.TopK = %d, want 10", fake.gotOpts.TopK)
	}
}

// TestSpineReviewConsolidateMinScoreAboveEveryPairYieldsZeroCandidates
// proves a zero-candidate report (as the store returns after filtering
// every seeded pair below --min-score) renders as an empty JSON array and
// exits 0.
func TestSpineReviewConsolidateMinScoreAboveEveryPairYieldsZeroCandidates(t *testing.T) {
	resetClientFlags(t)
	fake := &spineConsolidateFakeStore{}
	withFakeConsolidateStore(t, fake)

	stdout, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--min-score", "0.99", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if fake.gotOpts.MinScore == nil || *fake.gotOpts.MinScore != 0.99 {
		t.Errorf("gotOpts.MinScore = %v, want a pointer to 0.99", fake.gotOpts.MinScore)
	}
	if !strings.Contains(stdout, `"candidates":[]`) {
		t.Errorf("stdout = %q, want a zero-candidate report", stdout)
	}
}

// TestSpineReviewConsolidateProgressGoesToStderr captures stdout and
// stderr separately: progress lines must land on stderr, and stdout must
// hold exactly one JSON document.
func TestSpineReviewConsolidateProgressGoesToStderr(t *testing.T) {
	resetClientFlags(t)
	fake := &spineConsolidateFakeStore{
		pairs:           []store.DuplicatePair{{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.9}},
		progressScanned: 2, progressQueried: 2,
	}
	withFakeConsolidateStore(t, fake)

	stdout, stderr, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if !strings.Contains(stderr, "consolidate progress") {
		t.Errorf("stderr = %q, want a progress line", stderr)
	}
	if strings.Contains(stdout, "progress") {
		t.Errorf("stdout = %q, want no progress text on stdout", stdout)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not exactly one JSON document: %v (stdout=%q)", err, stdout)
	}
}

// TestConsolidateDocEmptyCandidatesMarshalsEmptyArray proves
// consolidateDoc's Candidates slice is non-nil even for a zero-pair
// result: JSON mode must emit "[]", never "null".
func TestConsolidateDocEmptyCandidatesMarshalsEmptyArray(t *testing.T) {
	doc := consolidateDoc(nil, "s", false, nil, 5, 0, 0)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"candidates":[]`) {
		t.Errorf("marshaled doc = %s, want a literal \"candidates\":[] (never null)", b)
	}
}

// TestConsolidateRowNamesBothScopes proves a cross-scope pair's row names
// both scopes in text mode, and carries both as distinct JSON fields.
func TestConsolidateRowNamesBothScopes(t *testing.T) {
	pairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "scope-a", BScope: "scope-b", Score: 0.9},
	}

	text := consolidateSummary(pairs, "", true, nil, 5, 2, 2)
	if !strings.Contains(text, "scope-a") || !strings.Contains(text, "scope-b") {
		t.Errorf("text = %q, want both scopes named", text)
	}

	doc := consolidateDoc(pairs, "", true, nil, 5, 2, 2)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"a_scope":"scope-a"`) || !strings.Contains(string(b), `"b_scope":"scope-b"`) {
		t.Errorf("marshaled doc = %s, want both scope fields as distinct JSON fields", b)
	}
}

// TestConsolidateNeverLabelsPairAsDuplicateOrCluster proves no per-pair
// row (text or JSON) carries a duplicate/cluster/group verdict label. The
// text header's own "NOT duplicates" disclaimer is deliberately excluded
// from this check -- stating the report's meaning plainly is required
// (see the CLI guide); labelling an individual CANDIDATE ROW a verdict is
// what is forbidden.
func TestConsolidateNeverLabelsPairAsDuplicateOrCluster(t *testing.T) {
	pairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.99},
	}

	text := consolidateSummary(pairs, "s", false, nil, 5, 2, 2)
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "score=") {
			continue // only candidate rows are under test here
		}
		lower := strings.ToLower(line)
		for _, bad := range []string{"duplicate", "cluster", "group"} {
			if strings.Contains(lower, bad) {
				t.Errorf("candidate row contains %q, want no verdict/cluster label: %q", bad, line)
			}
		}
	}

	doc := consolidateDoc(pairs, "s", false, nil, 5, 2, 2)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	lowerJSON := strings.ToLower(string(b))
	for _, bad := range []string{"duplicate", "cluster", "\"group\""} {
		if strings.Contains(lowerJSON, bad) {
			t.Errorf("marshaled doc contains %q, want no verdict/cluster label: %s", bad, b)
		}
	}
}

// TestConsolidateSummaryMinScoreRendering pins the pure formatter's
// min-score line for both the no-filter and filtered states.
func TestConsolidateSummaryMinScoreRendering(t *testing.T) {
	noFilter := consolidateSummary(nil, "s", false, nil, 5, 0, 0)
	if !strings.Contains(noFilter, "no filter applied") {
		t.Errorf("no-MinScore summary = %q, want it to state no filter is applied", noFilter)
	}

	threshold := float32(0.5)
	filtered := consolidateSummary(nil, "s", false, &threshold, 5, 0, 0)
	if !strings.Contains(filtered, "min_score=0.5") {
		t.Errorf("MinScore=0.5 summary = %q, want it to state the threshold", filtered)
	}
}
