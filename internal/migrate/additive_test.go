// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import (
	"maps"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestAdditiveOnlyKeySetDiff proves the additive-only invariant CheckAdditive
// enforces (D-04) with an eight-row fixture table built entirely from
// test-only steps constructed via NewStep — never migrate.Registry, which
// ships EMPTY this phase (D-06) and would make a table iterating it scan
// nothing and pass for the wrong reason (asserted explicitly below).
//
// The table asserts the key-set diff in BOTH directions: before is a
// subset of after (rows 4/7/8 prove the removal direction fires), and
// after-minus-before is SET-EQUAL to the step's declared AddsKeys — never a
// subset or a superset (rows 5 and 6 are the mirrored pair that
// distinguishes set equality from each one-way relation; D-04's second
// paragraph names this the load-bearing half). Row 8 proves the driver's
// two-independent-clones-per-application discipline is real: an ApplyFunc
// that mutates the map it was handed and returns that same map is still
// classified non-conforming.
//
// CheckAdditive compares KEY SETS ONLY. A step that overwrites an EXISTING
// key's VALUE in place — the key set itself unchanged — is invisible to
// this check (T-03-12); the restricted-writer alternative that would close
// that gap at the source (a PayloadAdder with no set/delete surface) is a
// recorded Deferred Idea, not an oversight. The consequence is contained
// downstream instead: Store.Migrate builds its Qdrant SetPayload map from
// AddedKeys(original, current) only, never from a step's full returned
// payload, so an overwritten value never reaches Qdrant — proven by
// TestMigrateWritesOnlyAddedKeys (plan 03-01, internal/store/migrate_test.go).
func TestAdditiveOnlyKeySetDiff(t *testing.T) {
	// Phase 4 registered the v0->v1 step (M13): Registry is no longer
	// empty. This table still runs against test-only fixture steps built
	// via NewStep, never Registry itself — so its content is independent
	// of Registry either way — but the shape of that independence
	// changed: it used to be "Registry is empty, so nothing to
	// accidentally scan"; now it is "the table iterates its own fixture
	// slice, never Registry's".
	if len(Registry) < 1 {
		t.Fatalf("Registry has %d step(s), want >=1: Phase 4 registered the v0->v1 step — this test's table runs against test-only fixtures, not Registry, so it is independent of Registry content", len(Registry))
	}
	if Registry[0].From() != 0 || Registry[0].To() != 1 {
		t.Fatalf("first registered step must be v0->v1, got From=%d To=%d", Registry[0].From(), Registry[0].To())
	}

	fixtures := []struct {
		name           string
		before         map[string]any
		step           Step
		wantConforming bool
		wantNamesInErr []string
	}{
		{
			name:   "conforming additive",
			before: map[string]any{"content": "x"},
			step: NewStep(0, 1, []string{"schema_version"},
				Reversible(func(p map[string]any) (map[string]any, error) {
					out := maps.Clone(p)
					delete(out, "schema_version")
					return out, nil
				}),
				func(p map[string]any) (map[string]any, error) {
					p["schema_version"] = 1
					return p, nil
				}),
			wantConforming: true,
		},
		{
			// Pins that reversibility and additive compliance are
			// ORTHOGONAL: an irreversible step is held to the same
			// key-set discipline as a reversible one, and a conforming
			// verdict here must not depend on which Reversibility
			// variant the step carries.
			name:   "conforming additive, irreversible with a stated reason",
			before: map[string]any{"content": "x"},
			step: NewStep(0, 1, []string{"schema_version"},
				Irreversible("this step's side effect on the wider record set cannot be undone by re-running the inverse of a single key stamp"),
				func(p map[string]any) (map[string]any, error) {
					p["schema_version"] = 1
					return p, nil
				}),
			wantConforming: true,
		},
		{
			// Named for the property it asserts — no ADDED keys — never
			// for a property it does not have. The sweep still advances
			// this record's schema_version to the target when it writes,
			// which is a real mutation; this row is NOT operationally a
			// no-op. Exercises the empty-set-against-empty-set edge,
			// where a naive nil-vs-empty-slice comparison breaks.
			name:   "no payload-key additions (version transition still advances)",
			before: map[string]any{"content": "x"},
			step: NewStep(0, 1, nil,
				Irreversible("pure side effect with no recorded inverse"),
				func(p map[string]any) (map[string]any, error) {
					return p, nil
				}),
			wantConforming: true,
		},
		{
			name:   "removes a key",
			before: map[string]any{"content": "x", "legacy_field": "y"},
			step: NewStep(0, 1, nil,
				Irreversible("legacy_field's value cannot be reconstructed once dropped"),
				func(p map[string]any) (map[string]any, error) {
					out := maps.Clone(p)
					delete(out, "legacy_field")
					return out, nil
				}),
			wantConforming: false,
			wantNamesInErr: []string{"legacy_field"},
		},
		{
			// D-04's load-bearing row: additive (nothing removed) and the
			// declared key IS among those added, but an undeclared key
			// rides along too. A superset check (declared ⊆ added) would
			// call this row conforming; set equality does not.
			name:   "adds an undeclared key",
			before: map[string]any{"content": "x"},
			step: NewStep(0, 1, []string{"schema_version"},
				Irreversible("undeclared_extra has no recorded inverse"),
				func(p map[string]any) (map[string]any, error) {
					p["schema_version"] = 1
					p["undeclared_extra"] = true
					return p, nil
				}),
			wantConforming: false,
			wantNamesInErr: []string{"undeclared_extra"},
		},
		{
			// Mirror of "adds an undeclared key": a subset check (added ⊆
			// declared) would call this row conforming; set equality
			// does not.
			name:   "declares a key it never adds",
			before: map[string]any{"content": "x"},
			step: NewStep(0, 1, []string{"schema_version", "never_written"},
				Irreversible("never_written is never set so there is nothing to undo"),
				func(p map[string]any) (map[string]any, error) {
					p["schema_version"] = 1
					return p, nil
				}),
			wantConforming: false,
			wantNamesInErr: []string{"never_written"},
		},
		{
			// Both directions violated at once — proves the two
			// directions are reported independently, not collapsed into
			// one generic message.
			name:   "removes one and adds an undeclared one",
			before: map[string]any{"content": "x", "legacy_field": "y"},
			step: NewStep(0, 1, nil,
				Irreversible("both the removal and the undeclared add are unrecoverable"),
				func(p map[string]any) (map[string]any, error) {
					out := maps.Clone(p)
					delete(out, "legacy_field")
					out["sneaky_key"] = true
					return out, nil
				}),
			wantConforming: false,
			wantNamesInErr: []string{"legacy_field", "sneaky_key"},
		},
		{
			// PA-5a (plan 03-01): this ApplyFunc deletes from THE MAP IT
			// WAS HANDED and returns that same map, rather than a copy.
			// If the test driver below cloned once and let before/after
			// alias one backing map, this deletion would be invisible to
			// the diff — AddedKeys and RemovedKeys would both be empty,
			// and CheckAdditive would return nil for a step that just
			// destroyed a key. This is the table-driver mirror of the
			// sweep-side sub-case in TestMigrateRefusesNonAdditiveStep;
			// it differs from row "removes a key" only in whether the
			// step copies, and that difference is the entire point.
			name:   "removes a key by mutating its input map in place",
			before: map[string]any{"content": "x", "legacy_field": "y"},
			step: NewStep(0, 1, nil,
				Irreversible("legacy_field's value is discarded in place; nothing to undo"),
				func(p map[string]any) (map[string]any, error) {
					delete(p, "legacy_field")
					return p, nil
				}),
			wantConforming: false,
			wantNamesInErr: []string{"legacy_field"},
		},
	}

	if len(fixtures) == 0 {
		t.Fatal("zero fixtures — D-05 requires a non-zero fixture count assertion")
	}

	var conformingCount, nonConformingCount int
	expectedNonConforming := make(map[string]struct{})
	for _, f := range fixtures {
		if f.wantConforming {
			conformingCount++
		} else {
			nonConformingCount++
			expectedNonConforming[f.name] = struct{}{}
		}
	}
	if conformingCount == 0 || nonConformingCount == 0 {
		t.Fatalf("fixture table must represent BOTH verdict classes: got %d conforming, %d non-conforming — a table of only-conforming rows would pass a checker that always returns nil, and a table of only-violating rows would pass one that always returns an error", conformingCount, nonConformingCount)
	}

	observedNonConforming := make(map[string]struct{})

	for _, f := range fixtures {
		f := f
		t.Run(f.name, func(t *testing.T) {
			// TWO independent clones per application, mirroring
			// Store.Migrate's own discipline (plan 03-01, PA-5a):
			// beforeRow is kept pristine and never touched by Apply; the
			// second clone is what Apply receives and, for row 8,
			// mutates in place. Passing one shared clone to both Apply
			// and CheckAdditive would alias before and after into the
			// same backing map and make row 8's deletion invisible.
			beforeRow := maps.Clone(f.before)
			after, err := f.step.Apply(maps.Clone(f.before))
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}

			gotErr := CheckAdditive(f.step, beforeRow, after)
			gotConforming := gotErr == nil

			if gotConforming != f.wantConforming {
				t.Errorf("CheckAdditive conforming = %v, want %v (err=%v)", gotConforming, f.wantConforming, gotErr)
			}

			if !f.wantConforming {
				if gotErr == nil {
					t.Fatal("expected a non-conforming verdict but CheckAdditive returned nil")
				}
				for _, name := range f.wantNamesInErr {
					if !strings.Contains(gotErr.Error(), name) {
						t.Errorf("error %q does not mention expected key %q", gotErr.Error(), name)
					}
				}
			}

			if !gotConforming {
				observedNonConforming[f.name] = struct{}{}
				// Non-conforming rows skip the idempotence assertion: a
				// step that already violates the declaration contract
				// has no defined second-application behavior, and
				// asserting one would invent a requirement D-04 does
				// not state.
				return
			}

			// Per-row step-level idempotence (PA-4), for CONFORMING rows
			// only: applying the step a second time to its own output
			// must yield a deep-equal payload. This is one of the two
			// executable halves of SC1's "idempotency" word — the other
			// is the sweep-level rerun proof in
			// TestMigrateTracerLegacyRecordEndToEnd (plan 03-01).
			// migrate.Validate's transition-uniqueness rule is a
			// structural precondition for both, not a proof of either.
			secondAfter, err := f.step.Apply(maps.Clone(after))
			if err != nil {
				t.Fatalf("second Apply: %v", err)
			}
			if !reflect.DeepEqual(secondAfter, after) {
				t.Errorf("step is not idempotent under re-application: second Apply = %v, want %v (differing keys: %v)",
					secondAfter, after, diffKeys(after, secondAfter))
			}
		})
	}

	// The strongest guard in this file: the OBSERVED non-conforming name
	// set compared to the EXPECTED one, printing both difference
	// directions on failure. Neither an always-nil checker (would lose
	// every expected name) nor an always-error checker (would gain every
	// conforming row's name) can pass this assertion, and it does not
	// depend on any row's error text.
	assertSetEqual(t, observedNonConforming, expectedNonConforming)
}

// diffKeys returns, sorted, the keys where a and b disagree — either
// present with a different value, or present in one but absent from the
// other. Used only to make an idempotence failure's diff legible.
func diffKeys(a, b map[string]any) []string {
	var out []string
	seen := make(map[string]struct{}, len(a))
	for k, v := range a {
		seen[k] = struct{}{}
		if bv, ok := b[k]; !ok || !reflect.DeepEqual(v, bv) {
			out = append(out, k)
		}
	}
	for k := range b {
		if _, ok := seen[k]; ok {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// assertSetEqual compares got (the observed non-conforming fixture names)
// against want (the expected set) as a SET, printing both difference
// directions on failure.
func assertSetEqual(t *testing.T, got, want map[string]struct{}) {
	t.Helper()
	var missing, extra []string
	for name := range want {
		if _, ok := got[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("observed non-conforming name set != expected: missing (expected but not observed) = %v; extra (observed but not expected) = %v", missing, extra)
	}
}
