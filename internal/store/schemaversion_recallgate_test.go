// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves ROADMAP success criterion 4: schema_version never appears
// in any Qdrant recall or authz filter condition transmitted by Search,
// SearchReranked, SearchDiscovery, List, ListScheduled or ListScopes.
//
// THE AUTHORITATIVE PROOF is TestSchemaVersionNeverGatesRecall (Task 3): a
// gRPC unary interceptor captures the *qdrant.Filter objects the six
// caller-facing recall entry points actually TRANSMIT to a real Qdrant, and
// a recursive walker (Task 1, walkFilterKeys — this task) proves
// schema_version is absent from every one of them. This is evidence, not
// inference: the interceptor observes the wire, it does not reconstruct a
// filter by re-calling ownerScopeFilter/listFilter and re-appending
// conditions.
//
// TestRecallEmissionSetIsCompleteAndClassified (Task 2, added next in this
// same file) is a SECONDARY, static layer: a go/ast derivation of every
// place internal/store transmits a Query/QueryBatch/Scroll/ScrollAndOffset/
// Count call, closed over a same-package call graph from the six recall
// entry points, with every emission site landing in exactly one of three
// explicitly justified categories. Its job is to catch TOMORROW'S new write
// path — not today's, which Task 3 already proves directly — and it will be
// stated at exactly the strength go/ast establishes, no further:
//
//  1. Function values, interface dispatch and reflection are not followed
//     by the call graph. The three-way classification IS the backstop for
//     this limit.
//  2. The method vocabulary is a MAINTAINED LIST, and the classification
//     CANNOT backstop it: an emission behind an unenumerated method name
//     produces no subject at all.
//  3. No type identity: this biases both the emission scan and the call
//     graph toward OVER-approximation, the correct bias for a gate.
//
// This file reuses (does not re-implement) plan 02-02's scanQdrantCalls,
// scanPackageDirForCalls and receiverText from schemaversion_stamp_gate_test.go
// — the whole point of that plan pinning them to a caller-supplied method
// set.
package store

import (
	"slices"
	"sort"
	"testing"

	"github.com/qdrant/go-client/qdrant"
)

// ============================================================================
// Task 1: the recursive filter-key walker and its positive/negative controls
// ============================================================================

// walkFilterKeys returns, as a sorted slice, every payload field key f
// references — recursing through Must, Should AND MustNot (a MustNot
// reference is just as capable of narrowing recall as a Must one) and
// through every one of the pinned go-client@v1.18.3 client's SEVEN
// Condition oneof variants, read from the module cache rather than guessed:
// Field, IsEmpty, IsNull, Nested (its own key, AND its wrapped sub-filter,
// if any), Filter (a wrapped sub-filter — the recursion), HasId and
// HasVector (both carry no payload key and are handled explicitly so that
// "handled" is a checked property, not an unwritten assumption).
//
// All seven are named here by design: an eighth variant added by a future
// client upgrade is a hole, so walkCondition fails the test loudly via
// t.Fatalf rather than silently falling through an unnamed default.
//
// f == nil is a documented no-op: it returns an empty slice without
// panicking.
//
// The returned slice is sorted so every assertion built on it is
// order-independent — reordering the f.Must appends in Search (or any
// other builder) must not be able to change any verdict this file reaches.
func walkFilterKeys(t *testing.T, f *qdrant.Filter) []string {
	t.Helper()
	keys := map[string]bool{}
	walkFilter(t, f, keys)
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// walkFilter recurses into f's three condition slices. Load-bearing, not
// defensive: categoryMatchCondition wraps its OR group via
// qdrant.NewFilterAsCondition, so a walker that only scanned the top level
// would miss any key buried in that group and the whole gate would pass
// vacuously.
func walkFilter(t *testing.T, f *qdrant.Filter, keys map[string]bool) {
	t.Helper()
	if f == nil {
		return
	}
	walkConditions(t, f.GetMust(), keys)
	walkConditions(t, f.GetShould(), keys)
	walkConditions(t, f.GetMustNot(), keys)
}

func walkConditions(t *testing.T, conds []*qdrant.Condition, keys map[string]bool) {
	t.Helper()
	for _, c := range conds {
		walkCondition(t, c, keys)
	}
}

// walkCondition handles all seven Condition oneof variants of
// go-client@v1.18.3 (qdrant/qdrant_common.pb.go:292-306) by concrete
// wrapper type, matched with an EXHAUSTIVE type switch — never a default
// that silently ignores an unrecognized shape. A nil oneof (a legitimately
// possible zero-value Condition) contributes nothing. Any wrapper type this
// switch does not recognize is a client-version mismatch and fails the
// test loudly, naming the unrecognized Go type.
func walkCondition(t *testing.T, c *qdrant.Condition, keys map[string]bool) {
	t.Helper()
	if c == nil {
		return
	}
	switch v := c.GetConditionOneOf().(type) {
	case *qdrant.Condition_Field:
		keys[v.Field.GetKey()] = true
	case *qdrant.Condition_IsEmpty:
		keys[v.IsEmpty.GetKey()] = true
	case *qdrant.Condition_IsNull:
		keys[v.IsNull.GetKey()] = true
	case *qdrant.Condition_HasId:
		// HasId carries no payload key — handled explicitly, contributes
		// nothing. Pinned by TestFilterWalkerSeesEveryPosition's
		// has-id/has-vector subtest.
	case *qdrant.Condition_HasVector:
		// HasVector carries no payload key — same as HasId, above.
	case *qdrant.Condition_Nested:
		keys[v.Nested.GetKey()] = true
		walkFilter(t, v.Nested.GetFilter(), keys)
	case *qdrant.Condition_Filter:
		walkFilter(t, v.Filter, keys)
	case nil:
		// An empty oneof — legitimately possible for a zero-value
		// Condition. Contributes nothing.
	default:
		t.Fatalf("walkCondition: unhandled Condition oneof variant %T — the pinned go-client@v1.18.3 exposes exactly seven (Field, IsEmpty, HasId, Filter, IsNull, Nested, HasVector; qdrant/qdrant_common.pb.go:292-306); an eighth means the module version moved and this walker must be updated before this gate can be trusted", v)
	}
}

// assertKeysEqual compares got (already sorted by walkFilterKeys) against
// want as a SET, never as a bare non-zero-length check — an "at least one
// key" assertion would pass a walker that returns the wrong keys entirely.
func assertKeysEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)
	if !slices.Equal(got, wantSorted) {
		t.Errorf("walked key set = %v, want %v", got, wantSorted)
	}
}

// TestFilterWalkerSeesEveryPosition proves walkFilterKeys sees every
// position a schema_version condition could hide in — both directions: ten
// positive/structural subtests plus two negative controls (adjacency and a
// clean filter), so a walker that returned an empty set for every input
// could not pass this file.
func TestFilterWalkerSeesEveryPosition(t *testing.T) {
	// A nil filter must not panic. Checked once here, outside any subtest,
	// so it does not perturb the ten-subtest count this task's <verify>
	// command greps for.
	if got := walkFilterKeys(t, nil); len(got) != 0 {
		t.Fatalf("walkFilterKeys(nil) = %v, want an empty slice (and must not panic)", got)
	}

	t.Run("top-level Must", func(t *testing.T) {
		f := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch(schemaVersionKey, "irrelevant-value")}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("MustNot", func(t *testing.T) {
		f := &qdrant.Filter{MustNot: []*qdrant.Condition{qdrant.NewMatch(schemaVersionKey, "irrelevant-value")}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("is-empty", func(t *testing.T) {
		// The exact condition shape the inverted-cardinality trap would
		// introduce: absence is the rare state for superseded_by/archived_at
		// but the MAJORITY state for schema_version at adoption.
		f := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty(schemaVersionKey)}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("is-null", func(t *testing.T) {
		f := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsNull(schemaVersionKey)}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("nested object condition", func(t *testing.T) {
		// qdrant.NewNestedCondition sets Nested.Key itself AND wraps the
		// given condition inside Nested.Filter.Must — both halves are
		// asserted in this one subtest, per the plan's instruction.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewNestedCondition("metadata", qdrant.NewMatch(schemaVersionKey, "irrelevant-value")),
		}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{"metadata", schemaVersionKey})
	})

	t.Run("has-id and has-vector carry no payload key", func(t *testing.T) {
		// Pins the two non-key-bearing variants as a CHECKED property: the
		// walked key set is empty and the walker does not panic. A walker
		// that panicked or mis-keyed either variant fails here.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewHasID(qdrant.NewID("00000000-0000-0000-0000-000000000000")),
			qdrant.NewHasVector("dense"),
		}}
		got := walkFilterKeys(t, f)
		if len(got) != 0 {
			t.Errorf("walked key set = %v, want empty — HasId/HasVector carry no payload key", got)
		}
	})

	t.Run("nested Should group", func(t *testing.T) {
		// THE LOAD-BEARING SUBTEST: this is exactly categoryMatchCondition's
		// shape (qdrant.NewFilterAsCondition(&qdrant.Filter{Should: ...})).
		// A non-recursive walker — one that only scanned the top-level
		// Must/Should/MustNot without following Condition_Filter — would
		// report this filter clean while a schema_version condition sits
		// inside the OR group, and the whole gate would pass vacuously.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
				qdrant.NewMatch(schemaVersionKey, "irrelevant-value"),
				qdrant.NewMatch("other_key", "x"),
			}}),
		}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey, "other_key"})
	})

	t.Run("doubly nested", func(t *testing.T) {
		// Proves the recursion is UNBOUNDED, not merely one level deep.
		inner := qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewMatch(schemaVersionKey, "irrelevant-value"),
		}})
		outer := qdrant.NewFilterAsCondition(&qdrant.Filter{Must: []*qdrant.Condition{inner}})
		f := &qdrant.Filter{Must: []*qdrant.Condition{outer}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("adjacency negative control", func(t *testing.T) {
		// Matching is exact string equality: a key that merely contains or
		// is contained by the target must not register as a hit.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatch("scope", "x"),
			qdrant.NewMatch("owner", "y"),
			qdrant.NewMatch("schema", "z"),
			qdrant.NewMatch("schema_version_legacy", "w"),
		}}
		got := walkFilterKeys(t, f)
		if slices.Contains(got, schemaVersionKey) {
			t.Errorf("walked key set %v contains %q — matching must be exact string equality, not substring/prefix/suffix", got, schemaVersionKey)
		}
		assertKeysEqual(t, got, []string{"scope", "owner", "schema", "schema_version_legacy"})
	})

	t.Run("clean filter negative control", func(t *testing.T) {
		// A realistic recall-shaped filter with no version reference. A
		// walker that returned an EMPTY set for every input would pass a
		// naive absence check trivially — asserting non-emptiness here is
		// what stops that.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatch("scope", "x"),
			qdrant.NewMatch("owner", "y"),
			qdrant.NewIsEmpty("superseded_by"),
			qdrant.NewIsEmpty("archived_at"),
		}}
		got := walkFilterKeys(t, f)
		if len(got) == 0 {
			t.Fatal("walked key set is empty for a realistic recall filter — a walker that always returns empty would pass a naive absence check")
		}
		if slices.Contains(got, schemaVersionKey) {
			t.Errorf("walked key set %v unexpectedly contains %q", got, schemaVersionKey)
		}
	})
}
