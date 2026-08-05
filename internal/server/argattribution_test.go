// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
)

// TestValidationErrorAttributionMatrix is criterion 2's central verification
// artifact (D-07): one case per single-field-invalid input, covering both
// 04-01's five validateStoreDiscovery rejections and this plan's sweep of
// tools.go (RESEARCH rows 6-21, 32-34, plus three parseWindow sub-checks the
// row inventory did not separately enumerate — see the 04-04-SUMMARY.md
// "RESEARCH inventory gap" note). Every assertion is on the FIELD IDENTIFIER
// (argFieldsOf, compared for full-set EQUALITY, never membership) and the
// HINT CODE — never on message wording, exactly as criterion 2 instructs.
//
// Coverage note on RESEARCH rows 18-21 (the four inline created_after/
// created_before parses in the search_memory/list_memory MCP closures,
// tools.go:1568-1574,1615-1621): these are byte-identical in field, hint,
// and detail text to the listScheduled window-parse rows tested below
// (rows 13/14) — same argErrf(classMalformed, HintFormat, <field>, "<field>
// must be RFC3339") call shape, confirmed by git diff review. They are NOT
// independently driven through a full MCP client/server round trip here:
// they live inside mcp.AddTool closures reachable only via Register(), which
// builds its deps from environment-configured Qdrant/embedder state
// (buildDepsFromEnv), not from an injectable test double — standing up that
// path would add transport-level test infrastructure disproportionate to
// confirming a rejection shape already covered byte-for-byte by rows 13/14.
// The whole-file value-echo gate (`got %q` zero-count, T-04-02) and `go vet`
// still apply to those four sites unconditionally. This matrix already
// clears the >=21 floor without them (23 rows below).
func TestValidationErrorAttributionMatrix(t *testing.T) {
	const sc = "discovery:repo:X"
	cite := []citationArg{{Kind: "file", Ref: "f"}}

	type wantEnvelope struct {
		fields []string
		hint   HintCode
	}

	// --- 04-01's five validateStoreDiscovery rows ---
	discoveryCases := []struct {
		name string
		a    storeDiscoveryArgs
		want wantEnvelope
	}{
		{"discovery_empty_content", storeDiscoveryArgs{Content: "", Kind: "fact", Scope: sc, Citations: cite}, wantEnvelope{[]string{"content"}, HintRequired}},
		{"discovery_content_too_large", storeDiscoveryArgs{Content: strings.Repeat("a", maxDiscoveryContentBytes+1), Kind: "fact", Scope: sc, Citations: cite}, wantEnvelope{[]string{"content"}, HintTooLong}},
		{"discovery_bad_kind", storeDiscoveryArgs{Content: "x", Kind: "blob", Scope: sc, Citations: cite}, wantEnvelope{[]string{"kind"}, HintEnum}},
		{"discovery_empty_scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "", Citations: cite}, wantEnvelope{[]string{"scope"}, HintRequired}},
		{"discovery_non_discovery_scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "repo:X", Citations: cite}, wantEnvelope{[]string{"scope"}, HintPrefix}},
	}
	for _, tc := range discoveryCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoreDiscovery(tc.a)
			assertEnvelope(t, err, tc.want.fields, tc.want.hint)
		})
	}

	// --- validateCitations, rows 6-11 ---
	citationCases := []struct {
		name     string
		cites    []citationArg
		minCount int
		want     wantEnvelope
	}{
		{"citations_below_min_one", nil, 1, wantEnvelope{[]string{"citations"}, HintRequired}},
		{"citations_below_min_multi", nil, 2, wantEnvelope{[]string{"citations"}, HintRequired}},
		{"citations_too_many", make([]citationArg, maxDiscoveryCitations+1), 0, wantEnvelope{[]string{"citations"}, HintTooMany}},
		{"citations_bad_kind", []citationArg{{Kind: "bogus", Ref: "r"}}, 0, wantEnvelope{[]string{"citations[i].kind"}, HintEnum}},
		{"citations_empty_ref", []citationArg{{Kind: "file", Ref: ""}}, 0, wantEnvelope{[]string{"citations[i].ref"}, HintRequired}},
		{"citations_excerpt_too_large", []citationArg{{Kind: "file", Ref: "r", Excerpt: strings.Repeat("a", maxCitationExcerptBytes+1)}}, 0, wantEnvelope{[]string{"citations[i].excerpt"}, HintTooLong}},
	}
	for _, tc := range citationCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// citations_too_many needs every element individually VALID (kind
			// file, non-empty ref, small excerpt) so the count check is what
			// fires, not a per-element check on element 0.
			cites := tc.cites
			if tc.name == "citations_too_many" {
				for i := range cites {
					cites[i] = citationArg{Kind: "file", Ref: "r"}
				}
			}
			err := validateCitations(cites, tc.minCount)
			assertEnvelope(t, err, tc.want.fields, tc.want.hint)
		})
	}

	// --- checkIdempotentReplay, row 12 ---
	t.Run("idempotency_key_too_large", func(t *testing.T) {
		d := &deps{}
		_, _, _, _, err := d.checkIdempotentReplay(context.Background(), "owner", storeArgs{
			Scope:          "tool:project:x",
			IdempotencyKey: strings.Repeat("a", maxIdempotencyKeyBytes+1),
		})
		assertEnvelope(t, err, []string{"idempotency_key"}, HintTooLong)
	})

	// --- listScheduled, rows 13-15 ---
	listScheduledCases := []struct {
		name string
		a    listScheduledArgs
		want wantEnvelope
	}{
		{"list_scheduled_bad_created_after", listScheduledArgs{Scope: "tool:project:x", CreatedAfter: "not-a-date"}, wantEnvelope{[]string{"created_after"}, HintFormat}},
		{"list_scheduled_bad_created_before", listScheduledArgs{Scope: "tool:project:x", CreatedBefore: "not-a-date"}, wantEnvelope{[]string{"created_before"}, HintFormat}},
		{"list_scheduled_bad_state", listScheduledArgs{Scope: "tool:project:x", State: "bogus"}, wantEnvelope{[]string{"state"}, HintEnum}},
	}
	for _, tc := range listScheduledCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d := &deps{}
			_, err := d.listScheduled(context.Background(), caller{}, tc.a)
			assertEnvelope(t, err, tc.want.fields, tc.want.hint)
		})
	}

	// --- effectiveDiscoveryScope, row 16 ---
	t.Run("discovery_scope_conditional_required", func(t *testing.T) {
		_, err := effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: false, Scope: ""})
		assertEnvelope(t, err, []string{"scope"}, HintConditionalRequired)
	})

	// --- effectiveSearchScope, row 17 ---
	// Field attribution widens from ["scope"] to ["scope", "cross_spine"]
	// here (and only here — effectiveDiscoveryScope above is untouched):
	// this site now rejects via conditionalErrf against the declared
	// scope-required-unless-cross-spine rule (02-01-PLAN.md Task 1), whose
	// registry entry correctly names both fields the constraint relates.
	t.Run("search_scope_conditional_required", func(t *testing.T) {
		_, err := effectiveSearchScope("", false)
		assertEnvelope(t, err, []string{"scope", "cross_spine"}, HintConditionalRequired)
	})

	// --- parseWindow, rows 32/33/34 plus the three sub-checks RESEARCH's
	// inventory did not separately number (not_before format, not_after
	// format, not_after-in-the-past) ---
	fixedNow := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	future := "2030-01-01T00:00:00Z"
	pastButValid := "2019-01-01T00:00:00Z"
	parseWindowCases := []struct {
		name string
		a    scheduleArgs
		want wantEnvelope
	}{
		{
			"window_both_bounds_absent",
			scheduleArgs{},
			wantEnvelope{[]string{"not_before", "not_after"}, HintRequired},
		},
		{
			"window_discovery_not_schedulable",
			scheduleArgs{storeArgs: storeArgs{Category: "discovery"}, NotBefore: future},
			wantEnvelope{[]string{"category"}, HintNotApplicable},
		},
		{
			"window_not_before_malformed",
			scheduleArgs{NotBefore: "SENTINEL-BADTIME-not-rfc3339"},
			wantEnvelope{[]string{"not_before"}, HintFormat},
		},
		{
			"window_not_after_malformed",
			scheduleArgs{NotAfter: "SENTINEL-BADTIME-not-rfc3339"},
			wantEnvelope{[]string{"not_after"}, HintFormat},
		},
		{
			"window_not_after_not_in_future",
			scheduleArgs{NotAfter: pastButValid},
			wantEnvelope{[]string{"not_after"}, HintOrdering},
		},
		{
			"window_ordering_violation",
			scheduleArgs{NotBefore: "2025-01-02T00:00:00Z", NotAfter: "2025-01-01T00:00:00Z"},
			wantEnvelope{[]string{"not_before", "not_after"}, HintOrdering},
		},
	}
	for _, tc := range parseWindowCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseWindow(tc.a, fixedNow)
			assertEnvelope(t, err, tc.want.fields, tc.want.hint)
		})
	}
}

// assertEnvelope asserts err is non-nil FIRST (guarding the vacuous-row
// failure mode: a row whose input was actually accepted must fail loudly,
// not pass silently), then asserts argFieldsOf(err) equals wantFields as a
// full-SET equality (never membership — membership would let an attribution
// naming a neighbouring field slip through, per D-07) and argHintOf(err)
// equals wantHint exactly.
func assertEnvelope(t *testing.T, err error, wantFields []string, wantHint HintCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a non-nil error for this deliberately-invalid input, got nil")
	}
	gotFields := argFieldsOf(err)
	if !reflect.DeepEqual(gotFields, wantFields) {
		t.Errorf("argFieldsOf(err) = %v, want %v (err: %v)", gotFields, wantFields, err)
	}
	gotHint := argHintOf(err)
	if gotHint != wantHint {
		t.Errorf("argHintOf(err) = %v, want %v (err: %v)", gotHint, wantHint, err)
	}
}

// TestHintNeverEchoesValue is the D-12 NEGATIVE gate: for a representative
// set of converted rejections, drive them with a distinctive sentinel value
// and assert err.Error() does NOT contain that marker. A positive test that
// the right constraint is stated cannot prove the caller's value is absent —
// only an explicit absence assertion can.
//
// RED transcript (recorded here per the task's instruction, captured this
// session): with Task 2 already committed, the citation-kind check's
// argErrf call in tools.go was temporarily reverted to append the
// pre-conversion `, got %q", i, c.Kind` value-echo tail, and
// `go test ./internal/server/... -run 'TestHintNeverEchoesValue$/citation_kind_no_echo' -v -count=1`
// was re-run. It failed:
//
//	=== RUN   TestHintNeverEchoesValue
//	=== RUN   TestHintNeverEchoesValue/citation_kind_no_echo
//	    argattribution_test.go:246: err.Error() = "field=citations[i].kind hint=enum: citation 0: kind must be one of file|commit|url|repo, got \"SENTINEL-9f2xQ\"" contains forbidden marker "SENTINEL-9f2xQ"
//	--- FAIL: TestHintNeverEchoesValue (0.00s)
//	    --- FAIL: TestHintNeverEchoesValue/citation_kind_no_echo (0.00s)
//	FAIL
//
// The revert was then undone (`git diff --exit-code -- internal/server/tools.go`
// confirmed zero net change), and the subtest passes again below. This
// confirms the gate fires on the exact defect class it exists to prevent.
func TestHintNeverEchoesValue(t *testing.T) {
	const marker = "SENTINEL-9f2xQ"

	assertNoEcho := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("expected a non-nil error, got nil")
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("err.Error() = %q contains forbidden marker %q", err.Error(), marker)
		}
	}

	t.Run("citation_kind_no_echo", func(t *testing.T) {
		err := validateCitations([]citationArg{{Kind: marker, Ref: "r"}}, 0)
		assertNoEcho(t, err)
	})

	t.Run("list_scheduled_state_no_echo", func(t *testing.T) {
		d := &deps{}
		_, err := d.listScheduled(context.Background(), caller{}, listScheduledArgs{Scope: "tool:project:x", State: marker})
		assertNoEcho(t, err)
	})

	t.Run("list_scheduled_created_after_no_echo", func(t *testing.T) {
		d := &deps{}
		_, err := d.listScheduled(context.Background(), caller{}, listScheduledArgs{Scope: "tool:project:x", CreatedAfter: marker})
		assertNoEcho(t, err)
	})

	t.Run("parse_window_not_before_no_echo", func(t *testing.T) {
		_, _, err := parseWindow(scheduleArgs{NotBefore: marker}, time.Now())
		assertNoEcho(t, err)
	})

	t.Run("idempotency_key_no_echo", func(t *testing.T) {
		d := &deps{}
		_, _, _, _, err := d.checkIdempotentReplay(context.Background(), "owner", storeArgs{
			Scope:          "tool:project:x",
			IdempotencyKey: marker + strings.Repeat("a", maxIdempotencyKeyBytes),
		})
		assertNoEcho(t, err)
	})

	t.Run("discovery_kind_no_echo", func(t *testing.T) {
		err := validateStoreDiscovery(storeDiscoveryArgs{
			Content: "x", Kind: marker, Scope: "discovery:repo:X",
			Citations: []citationArg{{Kind: "file", Ref: "f"}},
		})
		assertNoEcho(t, err)
	})

	t.Run("discovery_scope_prefix_no_echo", func(t *testing.T) {
		err := validateStoreDiscovery(storeDiscoveryArgs{
			Content: "x", Kind: "fact", Scope: marker,
			Citations: []citationArg{{Kind: "file", Ref: "f"}},
		})
		assertNoEcho(t, err)
	})
}

// TestCitationValidationIsNotCodeInternal pins the D-11a closure for
// validateCitations: each of its six rejections must NOT map to
// connect.CodeInternal on the Connect lane, the second of D-11a's three
// bare-unwrapped-error defects (validateStoreDiscovery was closed in 04-01;
// validateStoreRule is 04-05's).
func TestCitationValidationIsNotCodeInternal(t *testing.T) {
	valid := citationArg{Kind: "file", Ref: "r"}
	overCount := make([]citationArg, maxDiscoveryCitations+1)
	for i := range overCount {
		overCount[i] = valid
	}

	cases := []struct {
		name     string
		cites    []citationArg
		minCount int
	}{
		{"below_min_one", nil, 1},
		{"below_min_multi", nil, 2},
		{"too_many", overCount, 0},
		{"bad_kind", []citationArg{{Kind: "bogus", Ref: "r"}}, 0},
		{"empty_ref", []citationArg{{Kind: "file", Ref: ""}}, 0},
		{"excerpt_too_large", []citationArg{{Kind: "file", Ref: "r", Excerpt: strings.Repeat("a", maxCitationExcerptBytes+1)}}, 0},
	}
	ctx := context.Background()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateCitations(tc.cites, tc.minCount)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			got := connect.CodeOf(connectError(ctx, err))
			if got == connect.CodeInternal {
				t.Fatalf("connectError classified a validation rejection as CodeInternal (D-11a): %v", err)
			}
		})
	}
}
