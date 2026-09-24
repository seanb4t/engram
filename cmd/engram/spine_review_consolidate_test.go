// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"encoding/json"
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

// TestConsolidateNeverLabelsPairAsDuplicateOrCluster proves no per-pair
// row (rendered view or json) carries a duplicate/cluster/group verdict
// label. consolidateSummary's own "NOT duplicates" disclaimer is
// deliberately excluded from this check -- stating the report's meaning
// plainly is required (see the CLI guide); labelling an individual
// CANDIDATE ROW a verdict is what is forbidden. Per-pair detail now lives
// only in the rendered view's candidate rows (R1 moved it out of
// consolidateSummary), so this checks those rows rather than summary
// text.
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
