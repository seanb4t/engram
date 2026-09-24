// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/decide/jev"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/verdict"
)

// spineConsolidateFakeStore is a recording fake satisfying
// spineConsolidateStore, letting these tests observe exactly which
// store.NearDuplicateOptions a cobra invocation produced -- and inject a
// canned result -- without dialing a live Qdrant. Also invokes
// opts.Progress (when set) with its own configured counts, so the
// stderr-wiring test can prove the RunE plumbs the callback through
// without needing a real sweep. states/statesErr/recordStatesCalls let the
// verdict-pass tests observe and control RecordStates without a live
// Qdrant.
type spineConsolidateFakeStore struct {
	called                           bool
	gotOpts                          store.NearDuplicateOptions
	pairs                            []store.DuplicatePair
	err                              error
	progressScanned, progressQueried uint64

	states             map[string]store.RecordState
	statesErr          error
	recordStatesCalls  int
	gotRecordStatesIDs []string
}

func (f *spineConsolidateFakeStore) NearDuplicates(_ context.Context, opts store.NearDuplicateOptions) ([]store.DuplicatePair, error) {
	f.called = true
	f.gotOpts = opts
	if opts.Progress != nil {
		opts.Progress(f.progressScanned, f.progressQueried)
	}
	return f.pairs, f.err
}

// RecordStates records the call and ids it was invoked with, then returns
// f.states filtered to exactly those ids (an id with no matching entry in
// f.states is simply absent from the result, mirroring the real store's
// unknown-id contract), or f.statesErr when set.
func (f *spineConsolidateFakeStore) RecordStates(_ context.Context, ids []string) (map[string]store.RecordState, error) {
	f.recordStatesCalls++
	f.gotRecordStatesIDs = ids
	if f.statesErr != nil {
		return nil, f.statesErr
	}
	out := make(map[string]store.RecordState, len(ids))
	for _, id := range ids {
		if s, ok := f.states[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

// withFakeConsolidateStore substitutes spineConsolidateStoreFromEnv with
// one that returns fake and a nil decider (the no-provider path every
// pre-verdict test exercises), restoring the real constructor via
// t.Cleanup.
func withFakeConsolidateStore(t *testing.T, fake *spineConsolidateFakeStore) {
	t.Helper()
	withFakeConsolidateStoreAndDecider(t, fake, nil, server.VerdictSettings{})
}

// withFakeConsolidateStoreAndDecider substitutes spineConsolidateStoreFromEnv
// with one that returns fake, dec and settings, restoring the real
// constructor via t.Cleanup.
func withFakeConsolidateStoreAndDecider(t *testing.T, fake *spineConsolidateFakeStore, dec decide.Decider, settings server.VerdictSettings) {
	t.Helper()
	orig := spineConsolidateStoreFromEnv
	spineConsolidateStoreFromEnv = func() (spineConsolidateStore, decide.Decider, server.VerdictSettings, error) {
		return fake, dec, settings, nil
	}
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
// both scopes: the json document carries them as distinct fields, and the
// rendered view's candidate row line -- which viewRow builds from those
// same json fields -- carries both, structurally rather than via a
// hand-written summary sentence. R1 (06-01-PLAN.md §Conversion Rules)
// moved per-pair detail out of consolidateSummary entirely, so
// consolidateSummary itself is no longer where "both scopes" is asserted.
func TestConsolidateRowNamesBothScopes(t *testing.T) {
	pairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "scope-a", BScope: "scope-b", Score: 0.9},
	}

	doc := consolidateDoc(pairs, "", true, nil, 5, 2, 2)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"a_scope":"scope-a"`) || !strings.Contains(string(b), `"b_scope":"scope-b"`) {
		t.Errorf("marshaled doc = %s, want both scope fields as distinct JSON fields", b)
	}

	var buf bytes.Buffer
	if err := renderOperatorView(&buf, consolidateSummary(pairs, "", true, nil, 5, 2, 2), doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	rendered := buf.String()
	if !strings.Contains(rendered, "scope-a") || !strings.Contains(rendered, "scope-b") {
		t.Errorf("rendered view = %q, want the candidate row to name both scopes", rendered)
	}
}

// TestConsolidateNeverLabelsPairAsDuplicateOrCluster proves the STRUCTURAL
// ranking itself never labels a pair: this test builds a doc with no
// verdict pass (no candidate's Verdict field is ever set), so no per-pair
// row (rendered view or json) may carry a duplicate/cluster/group label
// coming from that structural data alone. consolidateSummary's own "NOT
// duplicates" disclaimer is deliberately excluded from this check --
// stating the report's meaning plainly is required (see the CLI guide);
// labelling an individual CANDIDATE ROW a verdict from the structural
// fields is what is forbidden. This is orthogonal to plan 03-06's advisory
// verdict rendering (spine_review_consolidate_view.go): once the verdict
// pass runs, a candidate's OWN advisory verdict may legitimately name any
// of the five relations, including "duplicate" -- that is the verdict
// object doing its documented job (D-05), not the structural label this
// test guards against. Per-pair detail now lives only in the rendered
// view's candidate rows (R1 moved it out of consolidateSummary), so this
// checks those rows rather than summary text.
func TestConsolidateNeverLabelsPairAsDuplicateOrCluster(t *testing.T) {
	pairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.99},
	}

	doc := consolidateDoc(pairs, "s", false, nil, 5, 2, 2)
	var buf bytes.Buffer
	if err := renderOperatorView(&buf, consolidateSummary(pairs, "s", false, nil, 5, 2, 2), doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "a=") {
			continue // only candidate rows (viewRow's "a=... b=..." shape) are under test here
		}
		lower := strings.ToLower(line)
		for _, bad := range []string{"duplicate", "cluster", "group"} {
			if strings.Contains(lower, bad) {
				t.Errorf("candidate row contains %q, want no verdict/cluster label: %q", bad, line)
			}
		}
	}

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

// TestConsolidateSummaryMinScoreRendering pins the pure headline's
// min-score clause: present (naming "no min_score filter applied") only
// when minScore is nil; absent when minScore is non-nil -- the filtered
// case now states the threshold only via the min_score json key, never
// restated in the headline (R1/R2, 06-01-PLAN.md §Conversion Rules).
func TestConsolidateSummaryMinScoreRendering(t *testing.T) {
	noFilter := consolidateSummary(nil, "s", false, nil, 5, 0, 0)
	if !strings.Contains(noFilter, "no min_score filter applied") {
		t.Errorf("no-MinScore summary = %q, want it to state no min_score filter is applied", noFilter)
	}
	if strings.Contains(noFilter, "\n") {
		t.Errorf("consolidateSummary result contains a newline, want a single line: %q", noFilter)
	}

	threshold := float32(0.5)
	filtered := consolidateSummary(nil, "s", false, &threshold, 5, 0, 0)
	if strings.Contains(filtered, "min_score") {
		t.Errorf("MinScore=0.5 summary = %q, want no min_score mention in the headline (the value lives in the min_score json key)", filtered)
	}
	if strings.Contains(filtered, "\n") {
		t.Errorf("consolidateSummary result contains a newline, want a single line: %q", filtered)
	}
}

// TestConsolidateMinScoreOmitemptySymmetry proves the omitempty symmetry
// at the rendered level (D-01, 06-CONTEXT.md): a nil minScore drops the
// min_score key from BOTH the json document and the rendered view
// together -- the rendered view carries exactly one fewer top-level field
// line than the non-nil case.
func TestConsolidateMinScoreOmitemptySymmetry(t *testing.T) {
	threshold := float32(0.5)
	withFilter := consolidateDoc(nil, "s", false, &threshold, 5, 0, 0)
	withoutFilter := consolidateDoc(nil, "s", false, nil, 5, 0, 0)

	b, err := json.Marshal(withoutFilter)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(b), "min_score") {
		t.Errorf("marshaled doc = %s, want no min_score key when minScore is nil", b)
	}

	withFilterKeys, err := jsonTopLevelKeys(withFilter)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}
	withoutFilterKeys, err := jsonTopLevelKeys(withoutFilter)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}
	if got, want := len(withoutFilterKeys), len(withFilterKeys)-1; got != want {
		t.Errorf("len(jsonTopLevelKeys(nil-minScore doc)) = %d, want %d (one fewer key than the non-nil case)", got, want)
	}

	var bufWith, bufWithout bytes.Buffer
	if err := renderOperatorView(&bufWith, "headline", withFilter); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	if err := renderOperatorView(&bufWithout, "headline", withoutFilter); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	gotWith := countTopLevelFieldLines(bufWith.String())
	gotWithout := countTopLevelFieldLines(bufWithout.String())
	if gotWithout != gotWith-1 {
		t.Errorf("countTopLevelFieldLines(nil-minScore rendered) = %d, want %d (one fewer top-level field line than the non-nil case's %d)", gotWithout, gotWith-1, gotWith)
	}
}

// tracerVerdictResponse is the canned Decisions response the verdict
// tracer test's httptest handler replies with: a relation choice answer
// (duplicate, all five probabilities) and a same_subject noul answer.
const tracerVerdictResponse = `{
	"model": "typesafe/jev-1.13-20260917",
	"answers": {
		"relation": {"type": "choice", "choice": "duplicate", "probabilities": {"duplicate": 0.95, "contradicts": 0.01, "updates": 0.02, "related": 0.01, "unrelated": 0.01}},
		"same_subject": {"type": "noul", "noul": 0.97}
	}
}`

// TestSpineReviewConsolidateVerdictTracer proves the end-to-end verdict
// slice (D-04..D-06, D-09..D-11): a fake store returns one candidate pair
// whose states flow through a real jev.Client to an httptest Decisions
// endpoint and back as a nested JSON verdict.
func TestSpineReviewConsolidateVerdictTracer(t *testing.T) {
	resetClientFlags(t)

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tracerVerdictResponse))
	}))
	defer srv.Close()

	newer := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Hour)
	fake := &spineConsolidateFakeStore{
		pairs: []store.DuplicatePair{
			{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.9},
		},
		states: map[string]store.RecordState{
			// id-a is the NEWER record: PairRequest must send it as record_b
			// regardless of its A/B position in the pair.
			"id-a": {ID: "id-a", Content: "content-a", CreatedAt: newer},
			"id-b": {ID: "id-b", Content: "content-b", CreatedAt: older},
		},
	}
	settings := server.VerdictSettings{Threshold: verdict.DefaultThreshold, StateChars: verdict.DefaultStateChars}
	dec := jev.New(srv.URL+"/api", "test-key", "")
	withFakeConsolidateStoreAndDecider(t, fake, dec, settings)

	stdout, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if fake.recordStatesCalls != 1 {
		t.Errorf("RecordStates called %d times, want exactly 1", fake.recordStatesCalls)
	}

	stateMap, _ := gotBody["state"].(map[string]any)
	wantRecordB := verdict.State(fake.states["id-a"].Summary, fake.states["id-a"].Content, settings.StateChars)
	wantRecordA := verdict.State(fake.states["id-b"].Summary, fake.states["id-b"].Content, settings.StateChars)
	if got, _ := stateMap["record_b"].(string); got != wantRecordB {
		t.Errorf("state.record_b = %q, want %q (id-a is the newer record)", got, wantRecordB)
	}
	if got, _ := stateMap["record_a"].(string); got != wantRecordA {
		t.Errorf("state.record_a = %q, want %q", got, wantRecordA)
	}

	questions, _ := gotBody["questions"].(map[string]any)
	relationQ, _ := questions["relation"].(map[string]any)
	criteria, _ := relationQ["criteria"].(map[string]any)
	if len(criteria) != 5 {
		t.Errorf("relation question criteria has %d keys, want 5: %v", len(criteria), criteria)
	}
	for _, name := range verdict.Relations() {
		if _, ok := criteria[name]; !ok {
			t.Errorf("relation question criteria is missing %q: %v", name, criteria)
		}
	}
	sameSubjectQ, _ := questions["same_subject"].(map[string]any)
	if sameSubjectQ["type"] != "noul" {
		t.Errorf("same_subject question type = %v, want noul", sameSubjectQ["type"])
	}

	var doc struct {
		VerdictThreshold float64 `json:"verdict_threshold"`
		Candidates       []struct {
			Verdict map[string]any `json:"verdict"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(stdout): %v (stdout=%q)", err, stdout)
	}
	if doc.VerdictThreshold != 0.9 {
		t.Errorf("verdict_threshold = %v, want 0.9", doc.VerdictThreshold)
	}
	if len(doc.Candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(doc.Candidates))
	}
	v := doc.Candidates[0].Verdict
	wantKeys := []string{"relation", "probabilities", "same_subject", "needs_review", "model"}
	if len(v) != len(wantKeys) {
		t.Errorf("verdict has %d keys, want %d: %v", len(v), len(wantKeys), v)
	}
	for _, k := range wantKeys {
		if _, ok := v[k]; !ok {
			t.Errorf("verdict is missing key %q: %v", k, v)
		}
	}
	if v["relation"] != "duplicate" {
		t.Errorf("verdict.relation = %v, want duplicate", v["relation"])
	}
	if v["needs_review"] != false {
		t.Errorf("verdict.needs_review = %v, want false", v["needs_review"])
	}
	if v["model"] != "typesafe/jev-1.13-20260917" {
		t.Errorf("verdict.model = %v, want typesafe/jev-1.13-20260917", v["model"])
	}
}

// TestSpineReviewConsolidateNoProviderByteIdentical proves D-04: with a
// nil decider, stdout is byte-identical to json.Marshal(consolidateDoc(...))
// plus a newline, carries no verdict key, and RecordStates is never
// called.
func TestSpineReviewConsolidateNoProviderByteIdentical(t *testing.T) {
	resetClientFlags(t)
	fake := &spineConsolidateFakeStore{
		pairs:           []store.DuplicatePair{{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.9}},
		progressScanned: 5, progressQueried: 5,
	}
	withFakeConsolidateStore(t, fake)

	stdout, stderr, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}

	want, err := json.Marshal(consolidateDoc(fake.pairs, "", true, nil, spineConsolidateDefaultTopK, fake.progressScanned, fake.progressQueried))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if stdout != string(want)+"\n" {
		t.Errorf("stdout = %q, want %q (byte-identical to json.Marshal(consolidateDoc(...)) plus a newline)", stdout, string(want)+"\n")
	}
	if strings.Contains(stdout, "verdict") {
		t.Errorf("stdout contains %q, want no verdict key with no provider configured: %q", "verdict", stdout)
	}
	if strings.Contains(stderr, "verdict") {
		t.Errorf("stderr contains %q, want no verdict text with no provider configured: %q", "verdict", stderr)
	}
	if fake.recordStatesCalls != 0 {
		t.Errorf("RecordStates called %d times, want 0 (no decider configured)", fake.recordStatesCalls)
	}
}

// verdictThresholdFakeDecider is a canned decide.Decider answering every
// request with a "related" choice at probability 0.8 (all five
// probabilities present, D-05) and a same_subject noul — enough for
// TestSpineReviewConsolidateVerdictThresholdFlag to observe needs_review
// flip purely as a function of --verdict-threshold, with no live provider.
type verdictThresholdFakeDecider struct{}

func (verdictThresholdFakeDecider) Decide(_ context.Context, _ decide.Request) (decide.Response, error) {
	return verdictThresholdFakeDecider{}.response(), nil
}

func (d verdictThresholdFakeDecider) DecideMany(_ context.Context, reqs []decide.Request) []decide.Result {
	results := make([]decide.Result, len(reqs))
	for i := range reqs {
		results[i] = decide.Result{Response: d.response()}
	}
	return results
}

func (verdictThresholdFakeDecider) response() decide.Response {
	return decide.Response{
		Model: "test-model",
		Answers: map[string]decide.Answer{
			verdict.QuestionRelation: {
				Type:   decide.QuestionChoice,
				Choice: verdict.Related,
				Probabilities: map[string]float64{
					verdict.Duplicate: 0.05, verdict.Contradicts: 0.05, verdict.Updates: 0.05,
					verdict.Related: 0.8, verdict.Unrelated: 0.05,
				},
			},
			verdict.QuestionSameSubject: {Type: decide.QuestionNoul, Probability: 0.5},
		},
	}
}

// TestSpineReviewConsolidateVerdictThresholdFlag proves D-08's flag
// override end to end: a fake decider answering "related" at 0.8 flips
// needs_review as a function of --verdict-threshold alone, an invalid value
// exits exitUsage before the store/decider constructor is ever called, and
// an unrelated --verdict-threshold with no provider configured leaves the
// no-provider stdout byte-identical.
func TestSpineReviewConsolidateVerdictThresholdFlag(t *testing.T) {
	pairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.9},
	}
	states := map[string]store.RecordState{
		"id-a": {ID: "id-a", Content: "content-a"},
		"id-b": {ID: "id-b", Content: "content-b"},
	}

	cases := []struct {
		name            string
		flagValue       string
		wantThreshold   float64
		wantNeedsReview bool
	}{
		{"default 0.9: below threshold, needs_review true", "", 0.9, true},
		{"override 0.75: at or above, needs_review false", "0.75", 0.75, false},
		{"override 0.8 exactly p: needs_review false (strict less-than)", "0.8", 0.8, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			fake := &spineConsolidateFakeStore{pairs: pairs, states: states}
			withFakeConsolidateStoreAndDecider(t, fake, verdictThresholdFakeDecider{}, server.VerdictSettings{Threshold: verdict.DefaultThreshold, StateChars: verdict.DefaultStateChars})

			args := []string{"spine-review", "consolidate", "--all-scopes", "--output", "json"}
			if tc.flagValue != "" {
				args = append(args, "--verdict-threshold", tc.flagValue)
			}
			stdout, _, err := runClient(t, args...)
			if err != nil {
				t.Fatalf("runClient: %v", err)
			}

			var doc struct {
				VerdictThreshold float64 `json:"verdict_threshold"`
				Candidates       []struct {
					Verdict map[string]any `json:"verdict"`
				} `json:"candidates"`
			}
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatalf("json.Unmarshal(stdout): %v (stdout=%q)", err, stdout)
			}
			if doc.VerdictThreshold != tc.wantThreshold {
				t.Errorf("verdict_threshold = %v, want %v", doc.VerdictThreshold, tc.wantThreshold)
			}
			if len(doc.Candidates) != 1 {
				t.Fatalf("len(candidates) = %d, want 1", len(doc.Candidates))
			}
			if got := doc.Candidates[0].Verdict["needs_review"]; got != tc.wantNeedsReview {
				t.Errorf("needs_review = %v, want %v", got, tc.wantNeedsReview)
			}
		})
	}

	t.Run("invalid values exit usage before construction", func(t *testing.T) {
		for _, bad := range []string{"-0.1", "1.5", "NaN", "abc"} {
			t.Run(bad, func(t *testing.T) {
				resetClientFlags(t)
				orig := spineConsolidateStoreFromEnv
				spineConsolidateStoreFromEnv = func() (spineConsolidateStore, decide.Decider, server.VerdictSettings, error) {
					t.Error("spineConsolidateStoreFromEnv was called despite an invalid --verdict-threshold")
					return nil, nil, server.VerdictSettings{}, nil
				}
				t.Cleanup(func() { spineConsolidateStoreFromEnv = orig })

				_, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--verdict-threshold", bad)
				if err == nil {
					t.Fatalf("expected an error for --verdict-threshold %q, got nil", bad)
				}
				if got := exitCodeFromError(err); got != exitUsage {
					t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
				}
				if !strings.Contains(err.Error(), "--verdict-threshold") {
					t.Errorf("error = %v, want it to name --verdict-threshold", err)
				}
			})
		}
	})

	t.Run("no provider configured: byte-identical stdout regardless of the flag", func(t *testing.T) {
		resetClientFlags(t)
		fake := &spineConsolidateFakeStore{pairs: pairs}
		withFakeConsolidateStore(t, fake)

		stdout, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--verdict-threshold", "0.5", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v", err)
		}
		want, err := json.Marshal(consolidateDoc(pairs, "", true, nil, spineConsolidateDefaultTopK, 0, 0))
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if stdout != string(want)+"\n" {
			t.Errorf("stdout = %q, want %q (byte-identical to the no-provider run)", stdout, string(want)+"\n")
		}
		if fake.recordStatesCalls != 0 {
			t.Errorf("RecordStates called %d times, want 0 (no decider configured)", fake.recordStatesCalls)
		}
	})
}

// verdictSuccessResult builds a decide.Result whose Response carries a
// relation Choice answer at relationProb (all five D-05 probabilities
// present, the remainder split evenly across the other four) plus a
// same_subject noul — enough for the Task 2 tests below to observe
// verdict.FromResult's mapping without a live provider.
func verdictSuccessResult(relation string, relationProb float64) decide.Result {
	other := (1 - relationProb) / 4
	probs := map[string]float64{
		verdict.Duplicate: other, verdict.Contradicts: other, verdict.Updates: other,
		verdict.Related: other, verdict.Unrelated: other,
	}
	probs[relation] = relationProb
	return decide.Result{Response: decide.Response{
		Model: "test-model",
		Answers: map[string]decide.Answer{
			verdict.QuestionRelation:    {Type: decide.QuestionChoice, Choice: relation, Probabilities: probs},
			verdict.QuestionSameSubject: {Type: decide.QuestionNoul, Probability: 0.5},
		},
	}}
}

// scriptedFakeDecider returns pre-built decide.Result values, in order, from
// its single DecideMany call — it panics if the number of requests it
// receives doesn't match len(results), a test-authoring bug rather than a
// production scenario. Decide is never used by these tests.
type scriptedFakeDecider struct {
	results []decide.Result
	calls   int
}

func (d *scriptedFakeDecider) Decide(_ context.Context, _ decide.Request) (decide.Response, error) {
	panic("scriptedFakeDecider.Decide: not used by these tests")
}

func (d *scriptedFakeDecider) DecideMany(_ context.Context, reqs []decide.Request) []decide.Result {
	d.calls++
	if len(reqs) != len(d.results) {
		panic(fmt.Sprintf("scriptedFakeDecider.DecideMany: got %d reqs, want %d results", len(reqs), len(d.results)))
	}
	return d.results
}

// countingFakeDecider only records how many times Decide/DecideMany were
// called — used to prove a code path that must never reach the decider
// (--no-verdicts, a state-fetch error) genuinely doesn't.
type countingFakeDecider struct {
	decideCalls, decideManyCalls int
}

func (d *countingFakeDecider) Decide(_ context.Context, _ decide.Request) (decide.Response, error) {
	d.decideCalls++
	return decide.Response{}, nil
}

func (d *countingFakeDecider) DecideMany(_ context.Context, reqs []decide.Request) []decide.Result {
	d.decideManyCalls++
	return make([]decide.Result, len(reqs))
}

// capCountingFakeDecider records the length of reqs its single DecideMany
// call received and how many times it was called — TestSpineReviewConsolidateNoCapOnPairs
// uses it to prove D-11's no-cap contract: every buildable pair reaches the
// decider through exactly one DecideMany call, regardless of pair count.
type capCountingFakeDecider struct {
	calls      int
	lastReqLen int
}

func (d *capCountingFakeDecider) Decide(_ context.Context, _ decide.Request) (decide.Response, error) {
	panic("capCountingFakeDecider.Decide: not used")
}

func (d *capCountingFakeDecider) DecideMany(_ context.Context, reqs []decide.Request) []decide.Result {
	d.calls++
	d.lastReqLen = len(reqs)
	results := make([]decide.Result, len(reqs))
	for i := range results {
		results[i] = verdictSuccessResult(verdict.Related, 0.95)
	}
	return results
}

// blockingFakeDecider blocks every Decide call until its context is done,
// then returns ctx.Err() — DecideMany delegates to the real
// decide.DecideMany with a per-request function (the plan's own executor
// note), so the concurrency/ctx-check behavior under test is the
// production shape, not a hand-rolled substitute.
type blockingFakeDecider struct{}

func (blockingFakeDecider) Decide(ctx context.Context, _ decide.Request) (decide.Response, error) {
	<-ctx.Done()
	return decide.Response{}, ctx.Err()
}

func (d blockingFakeDecider) DecideMany(ctx context.Context, reqs []decide.Request) []decide.Result {
	return decide.DecideMany(ctx, func(c context.Context, r decide.Request) (decide.Response, error) {
		return d.Decide(c, r)
	}, reqs, 4)
}

// TestSpineReviewConsolidateNoVerdictsSuppresses proves D-04's opt-out: with
// --no-verdicts and a provider/decider configured, stdout is byte-identical
// to the no-provider run, RecordStates and decider call counts are 0, and
// stderr carries no "consolidate verdicts:" line.
func TestSpineReviewConsolidateNoVerdictsSuppresses(t *testing.T) {
	resetClientFlags(t)
	pairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.9},
	}
	states := map[string]store.RecordState{
		"id-a": {ID: "id-a", Content: "c"}, "id-b": {ID: "id-b", Content: "c"},
	}
	fake := &spineConsolidateFakeStore{pairs: pairs, states: states}
	dec := &countingFakeDecider{}
	withFakeConsolidateStoreAndDecider(t, fake, dec, server.VerdictSettings{Threshold: verdict.DefaultThreshold, StateChars: verdict.DefaultStateChars, Provider: "jev", Model: "test-model"})

	stdout, stderr, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--no-verdicts", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	want, err := json.Marshal(consolidateDoc(pairs, "", true, nil, spineConsolidateDefaultTopK, 0, 0))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if stdout != string(want)+"\n" {
		t.Errorf("stdout = %q, want %q (byte-identical to the no-provider run)", stdout, string(want)+"\n")
	}
	if fake.recordStatesCalls != 0 {
		t.Errorf("RecordStates called %d times, want 0", fake.recordStatesCalls)
	}
	if dec.decideCalls != 0 || dec.decideManyCalls != 0 {
		t.Errorf("decider called (Decide=%d, DecideMany=%d), want 0 with --no-verdicts", dec.decideCalls, dec.decideManyCalls)
	}
	if strings.Contains(stderr, "consolidate verdicts:") {
		t.Errorf("stderr contains a verdict line despite --no-verdicts: %q", stderr)
	}
}

// TestSpineReviewConsolidateFailurePolicy proves D-10's per-pair failure
// reporting, the disclosure/summary stderr lines, and no-cap (D-11) over
// four pairs: success, an auth error, a pair missing its second side's
// state, and a timeout error.
func TestSpineReviewConsolidateFailurePolicy(t *testing.T) {
	resetClientFlags(t)
	pairs := []store.DuplicatePair{
		{A: "id-a1", B: "id-b1", AShortID: "sa1", BShortID: "sb1", AScope: "s", BScope: "s", Score: 0.9},
		{A: "id-a2", B: "id-b2", AShortID: "sa2", BShortID: "sb2", AScope: "s", BScope: "s", Score: 0.9},
		{A: "id-a3", B: "id-b3", AShortID: "sa3", BShortID: "sb3", AScope: "s", BScope: "s", Score: 0.9},
		{A: "id-a4", B: "id-b4", AShortID: "sa4", BShortID: "sb4", AScope: "s", BScope: "s", Score: 0.9},
	}
	states := map[string]store.RecordState{
		"id-a1": {ID: "id-a1", Content: "c"}, "id-b1": {ID: "id-b1", Content: "c"},
		"id-a2": {ID: "id-a2", Content: "c"}, "id-b2": {ID: "id-b2", Content: "c"},
		"id-a3": {ID: "id-a3", Content: "c"}, // id-b3 deliberately absent: D-10's missing-side case
		"id-a4": {ID: "id-a4", Content: "c"}, "id-b4": {ID: "id-b4", Content: "c"},
	}
	fake := &spineConsolidateFakeStore{pairs: pairs, states: states}
	dec := &scriptedFakeDecider{results: []decide.Result{
		verdictSuccessResult(verdict.Duplicate, 0.99), // pair0: success
		{Err: &decide.Error{Kind: decide.ErrDecisionAuth}},    // pair1: auth
		{Err: &decide.Error{Kind: decide.ErrDecisionTimeout}}, // pair3: timeout (pair2 never reaches the decider)
	}}
	settings := server.VerdictSettings{Threshold: verdict.DefaultThreshold, StateChars: 1500, Provider: "jev", Model: "test-model", EndpointHost: "example.test:443"}
	withFakeConsolidateStoreAndDecider(t, fake, dec, settings)

	stdout, stderr, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if dec.calls != 1 {
		t.Errorf("DecideMany called %d times, want exactly 1", dec.calls)
	}

	var doc struct {
		Candidates []struct {
			Verdict map[string]any `json:"verdict"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(stdout): %v (stdout=%q)", err, stdout)
	}
	if len(doc.Candidates) != 4 {
		t.Fatalf("len(candidates) = %d, want 4", len(doc.Candidates))
	}
	if got := doc.Candidates[0].Verdict["relation"]; got != verdict.Duplicate {
		t.Errorf("candidates[0].verdict.relation = %v, want %q", got, verdict.Duplicate)
	}
	wantErrors := []string{"auth", "state_unavailable", "timeout"}
	for i, want := range wantErrors {
		c := doc.Candidates[i+1].Verdict
		if got := c["error"]; got != want {
			t.Errorf("candidates[%d].verdict.error = %v, want %q", i+1, got, want)
		}
		if len(c) != 1 {
			t.Errorf("candidates[%d].verdict has %d keys, want exactly 1 (error): %v", i+1, len(c), c)
		}
	}

	wantDisclosure := "consolidate verdicts: requesting advisory verdicts for 4 candidate pair(s) from decisions provider jev (model test-model) at host example.test:443"
	if !strings.Contains(stderr, wantDisclosure) {
		t.Errorf("stderr disclosure line missing or wrong: %q, want it to contain %q", stderr, wantDisclosure)
	}
	if !strings.Contains(stderr, "1500 characters") || !strings.Contains(stderr, "--no-verdicts") {
		t.Errorf("stderr disclosure line missing the state-char bound or --no-verdicts mention: %q", stderr)
	}
	wantSummary := "consolidate verdicts: requested 4, answered 1, needs_review 0, unavailable 3 (auth=1 state_unavailable=1 timeout=1)"
	if !strings.Contains(stderr, wantSummary) {
		t.Errorf("stderr summary line missing or wrong: %q, want it to contain %q", stderr, wantSummary)
	}
	if strings.Index(stderr, wantDisclosure) > strings.Index(stderr, wantSummary) {
		t.Errorf("disclosure line does not precede the summary line: stderr=%q", stderr)
	}
}

// TestSpineReviewConsolidateAllAuthFailureWarns proves D-10's loud all-auth
// signal: every request failing with the auth class prints an extra
// WARNING line naming both key env vars; a single success among auth
// failures suppresses it.
func TestSpineReviewConsolidateAllAuthFailureWarns(t *testing.T) {
	settings := server.VerdictSettings{Threshold: verdict.DefaultThreshold, StateChars: 1500, Provider: "jev", Model: "test-model"}
	pairs := []store.DuplicatePair{
		{A: "id-a1", B: "id-b1", AShortID: "sa1", BShortID: "sb1", AScope: "s", BScope: "s", Score: 0.9},
		{A: "id-a2", B: "id-b2", AShortID: "sa2", BShortID: "sb2", AScope: "s", BScope: "s", Score: 0.9},
	}
	states := map[string]store.RecordState{
		"id-a1": {ID: "id-a1", Content: "c"}, "id-b1": {ID: "id-b1", Content: "c"},
		"id-a2": {ID: "id-a2", Content: "c"}, "id-b2": {ID: "id-b2", Content: "c"},
	}

	t.Run("every request fails auth", func(t *testing.T) {
		resetClientFlags(t)
		fake := &spineConsolidateFakeStore{pairs: pairs, states: states}
		dec := &scriptedFakeDecider{results: []decide.Result{
			{Err: &decide.Error{Kind: decide.ErrDecisionAuth}},
			{Err: &decide.Error{Kind: decide.ErrDecisionAuth}},
		}}
		withFakeConsolidateStoreAndDecider(t, fake, dec, settings)

		_, stderr, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v", err)
		}
		if !strings.Contains(stderr, "WARNING:") {
			t.Fatalf("stderr missing a WARNING line: %q", stderr)
		}
		if !strings.Contains(stderr, "ENGRAM_DECISIONS_API_KEY") || !strings.Contains(stderr, "ENGRAM_OPENAI_API_KEY") {
			t.Errorf("WARNING line does not name both key env vars: %q", stderr)
		}
	})

	t.Run("one success among auth failures: no warning", func(t *testing.T) {
		resetClientFlags(t)
		fake := &spineConsolidateFakeStore{pairs: pairs, states: states}
		dec := &scriptedFakeDecider{results: []decide.Result{
			verdictSuccessResult(verdict.Related, 0.95),
			{Err: &decide.Error{Kind: decide.ErrDecisionAuth}},
		}}
		withFakeConsolidateStoreAndDecider(t, fake, dec, settings)

		_, stderr, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v", err)
		}
		if strings.Contains(stderr, "WARNING:") {
			t.Errorf("stderr contains a WARNING line despite one successful verdict: %q", stderr)
		}
	})
}

// TestSpineReviewConsolidateStateFetchErrorDegrades proves a RecordStates
// error degrades every pair to state_unavailable, never calls the decider,
// and still exits 0.
func TestSpineReviewConsolidateStateFetchErrorDegrades(t *testing.T) {
	resetClientFlags(t)
	pairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.9},
	}
	fake := &spineConsolidateFakeStore{pairs: pairs, statesErr: errors.New("qdrant unavailable")}
	dec := &countingFakeDecider{}
	withFakeConsolidateStoreAndDecider(t, fake, dec, server.VerdictSettings{Threshold: verdict.DefaultThreshold, StateChars: 1500})

	stdout, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if dec.decideManyCalls != 0 || dec.decideCalls != 0 {
		t.Errorf("decider was called (DecideMany=%d, Decide=%d), want 0 on a state-fetch error", dec.decideManyCalls, dec.decideCalls)
	}

	var doc struct {
		Candidates []struct {
			Verdict map[string]any `json:"verdict"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(stdout): %v (stdout=%q)", err, stdout)
	}
	if len(doc.Candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(doc.Candidates))
	}
	if got := doc.Candidates[0].Verdict["error"]; got != "state_unavailable" {
		t.Errorf("verdict.error = %v, want state_unavailable", got)
	}
}

// TestSpineReviewConsolidateDeadlineDuringVerdictPass proves a --timeout
// deadline reached during the verdict pass leaves every structural
// candidate present, each verdict classed timeout, and still exits 0.
func TestSpineReviewConsolidateDeadlineDuringVerdictPass(t *testing.T) {
	resetClientFlags(t)
	pairs := []store.DuplicatePair{
		{A: "id-a1", B: "id-b1", AShortID: "sa1", BShortID: "sb1", AScope: "s", BScope: "s", Score: 0.9},
		{A: "id-a2", B: "id-b2", AShortID: "sa2", BShortID: "sb2", AScope: "s", BScope: "s", Score: 0.9},
	}
	states := map[string]store.RecordState{
		"id-a1": {ID: "id-a1", Content: "c"}, "id-b1": {ID: "id-b1", Content: "c"},
		"id-a2": {ID: "id-a2", Content: "c"}, "id-b2": {ID: "id-b2", Content: "c"},
	}
	fake := &spineConsolidateFakeStore{pairs: pairs, states: states}
	withFakeConsolidateStoreAndDecider(t, fake, blockingFakeDecider{}, server.VerdictSettings{Threshold: verdict.DefaultThreshold, StateChars: 1500})

	stdout, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--timeout", "200ms", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}

	var doc struct {
		Candidates []struct {
			Verdict map[string]any `json:"verdict"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(stdout): %v (stdout=%q)", err, stdout)
	}
	if len(doc.Candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2 (structural candidates all present)", len(doc.Candidates))
	}
	for i, c := range doc.Candidates {
		if got := c.Verdict["error"]; got != "timeout" {
			t.Errorf("candidates[%d].verdict.error = %v, want timeout", i, got)
		}
	}
}

// TestSpineReviewConsolidateNoCapOnPairs proves D-11: 250 candidate pairs
// produce exactly 250 requests in a single DecideMany call — no cap.
func TestSpineReviewConsolidateNoCapOnPairs(t *testing.T) {
	resetClientFlags(t)
	const n = 250
	pairs := make([]store.DuplicatePair, n)
	states := make(map[string]store.RecordState, n*2)
	for i := 0; i < n; i++ {
		a, b := fmt.Sprintf("id-a%d", i), fmt.Sprintf("id-b%d", i)
		pairs[i] = store.DuplicatePair{
			A: a, B: b, AShortID: fmt.Sprintf("sa%d", i), BShortID: fmt.Sprintf("sb%d", i),
			AScope: "s", BScope: "s", Score: 0.9,
		}
		states[a] = store.RecordState{ID: a, Content: "c"}
		states[b] = store.RecordState{ID: b, Content: "c"}
	}
	fake := &spineConsolidateFakeStore{pairs: pairs, states: states}
	dec := &capCountingFakeDecider{}
	withFakeConsolidateStoreAndDecider(t, fake, dec, server.VerdictSettings{Threshold: verdict.DefaultThreshold, StateChars: 1500})

	if _, _, err := runClient(t, "spine-review", "consolidate", "--all-scopes", "--output", "json"); err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if dec.calls != 1 {
		t.Errorf("DecideMany called %d times, want exactly 1", dec.calls)
	}
	if dec.lastReqLen != n {
		t.Errorf("DecideMany received %d requests, want %d (no cap, D-11)", dec.lastReqLen, n)
	}
}

// TestVerdictHeadlineClause proves the pure headline addendum: it states
// verdicts are advisory and consolidate never merges or mutates, plus the
// answered/needs-review/unavailable counts; consolidateSummary's own
// output carries none of that vocabulary when no pass ran.
func TestVerdictHeadlineClause(t *testing.T) {
	stats := verdictOutcome{Requested: 4, Answered: 1, NeedsReview: 0, FailuresByClass: map[string]int{
		"auth": 1, "state_unavailable": 1, "timeout": 1,
	}}
	clause := verdictHeadlineClause(stats)
	for _, want := range []string{"advisory", "never merges or mutates", "1 answered", "0 needing review", "3 unavailable"} {
		if !strings.Contains(clause, want) {
			t.Errorf("verdictHeadlineClause(%+v) = %q, want it to contain %q", stats, clause, want)
		}
	}

	headline := consolidateSummary(nil, "s", false, nil, 5, 0, 0)
	for _, unwanted := range []string{"advisory", "never merges or mutates"} {
		if strings.Contains(headline, unwanted) {
			t.Errorf("consolidateSummary() = %q, want no verdict-pass vocabulary when no pass ran", headline)
		}
	}
}

// TestConsolidateTextViewRendersVerdict proves D-07/D-10's text form over
// three candidates: a successful unflagged verdict, a successful flagged
// verdict, and a failed one — rendered through renderOperatorView, which is
// consolidate's real text lane, via attachVerdicts' real JSON-mode
// conversion (never a hand-built verdictView literal).
func TestConsolidateTextViewRendersVerdict(t *testing.T) {
	pairs := []store.DuplicatePair{
		{A: "id-a1", B: "id-b1", AShortID: "sa1", BShortID: "sb1", AScope: "s", BScope: "s", Score: 0.9},
		{A: "id-a2", B: "id-b2", AShortID: "sa2", BShortID: "sb2", AScope: "s", BScope: "s", Score: 0.8},
		{A: "id-a3", B: "id-b3", AShortID: "sa3", BShortID: "sb3", AScope: "s", BScope: "s", Score: 0.7},
	}
	verdicts := []verdict.Verdict{
		{
			Relation:      verdict.Duplicate,
			Probabilities: verdict.Probabilities{Duplicate: 0.93, Contradicts: 0.01, Updates: 0.02, Related: 0.03, Unrelated: 0.01},
			SameSubject:   0.97,
			NeedsReview:   false,
			Model:         "typesafe/jev-1.13-20260917",
		},
		{
			Relation:      verdict.Related,
			Probabilities: verdict.Probabilities{Duplicate: 0.1, Contradicts: 0.1, Updates: 0.08, Related: 0.62, Unrelated: 0.1},
			SameSubject:   0.55,
			NeedsReview:   true,
			Model:         "typesafe/jev-1.13-20260917",
		},
		{ErrorClass: "timeout"},
	}

	doc := attachVerdicts(consolidateDoc(pairs, "s", false, nil, 5, 3, 3), verdicts, 0.9)

	var buf bytes.Buffer
	if err := renderOperatorView(&buf, "headline", doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	out := buf.String()

	var rows []string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "a=") {
			rows = append(rows, trimmed)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("got %d candidate rows, want 3: %v (full output %q)", len(rows), rows, out)
	}

	first := rows[0]
	for _, want := range []string{"verdict=duplicate p=0.93", "same_subject=0.97", "probabilities=duplicate:0.93,contradicts:", "model=typesafe/jev-1.13-20260917"} {
		if !strings.Contains(first, want) {
			t.Errorf("row[0] = %q, want it to contain %q", first, want)
		}
	}
	if strings.Contains(first, "[needs review]") {
		t.Errorf("row[0] = %q, want no [needs review] marker (unflagged)", first)
	}

	second := rows[1]
	if !strings.Contains(second, "verdict=related p=0.62 [needs review]") {
		t.Errorf("row[1] = %q, want it to contain %q", second, "verdict=related p=0.62 [needs review]")
	}

	third := rows[2]
	if !strings.Contains(third, "verdict unavailable (timeout)") {
		t.Errorf("row[2] = %q, want it to contain %q", third, "verdict unavailable (timeout)")
	}

	if !strings.Contains(out, "Verdict threshold") {
		t.Errorf("rendered output = %q, want a top-level \"Verdict threshold\" line", out)
	}
}

// TestRegisterRowFieldRendererRejectsDuplicates proves
// registerRowFieldRenderer panics on an empty key, a nil fn, or a duplicate
// registration — "verdict" is already registered by
// spine_review_consolidate_view.go's own init, so the duplicate case needs
// no setup here.
func TestRegisterRowFieldRendererRejectsDuplicates(t *testing.T) {
	assertPanics := func(t *testing.T, name string, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected a panic, got none", name)
			}
		}()
		fn()
	}

	t.Run("empty key", func(t *testing.T) {
		assertPanics(t, "empty key", func() {
			registerRowFieldRenderer("", func(json.RawMessage) (string, error) { return "", nil })
		})
	})
	t.Run("nil fn", func(t *testing.T) {
		assertPanics(t, "nil fn", func() {
			registerRowFieldRenderer("verdict-test-nil-fn", nil)
		})
	})
	t.Run("duplicate key", func(t *testing.T) {
		assertPanics(t, "duplicate key", func() {
			registerRowFieldRenderer("verdict", func(json.RawMessage) (string, error) { return "", nil })
		})
	})
}

// TestConsolidateStoreSurfaceIsReadOnly proves T-03-02: spineConsolidateStore's
// method set is exactly {NearDuplicates, RecordStates} — no mutating store
// method is reachable through this interface.
func TestConsolidateStoreSurfaceIsReadOnly(t *testing.T) {
	typ := reflect.TypeOf((*spineConsolidateStore)(nil)).Elem()
	want := map[string]bool{"NearDuplicates": true, "RecordStates": true}
	if typ.NumMethod() != len(want) {
		t.Fatalf("spineConsolidateStore has %d methods, want %d: %v", typ.NumMethod(), len(want), typ)
	}
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if !want[name] {
			t.Errorf("spineConsolidateStore has unexpected method %q, want exactly %v", name, want)
		}
	}
}
