// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storetest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
)

func TestLayout(t *testing.T) {
	type want struct {
		records, recordBytes int
	}
	cases := []struct {
		name    string
		limit   int
		shape   Shape
		want    want
		wantErr bool
	}{
		{name: "few-large at 4MiB", limit: 4 << 20, shape: FewLarge, want: want{40, 131072}},
		{name: "many-small at 4MiB", limit: 4 << 20, shape: ManySmall, want: want{1000, 5243}},
		{name: "few-large at 64MiB", limit: 64 << 20, shape: FewLarge, want: want{40, 2097152}},
		{name: "many-small at 64MiB", limit: 64 << 20, shape: ManySmall, want: want{1000, 83887}},
		{name: "few-large at 32", limit: 32, shape: FewLarge, want: want{40, 1}},
		{name: "many-small at 1000", limit: 1000, shape: ManySmall, want: want{1000, 2}},
		{name: "many-small at 2", limit: 2, shape: ManySmall, want: want{1000, 1}},
		{name: "few-large below divisor", limit: 31, shape: FewLarge, wantErr: true},
		{name: "many-small limit 1", limit: 1, shape: ManySmall, wantErr: true},
		{name: "zero limit", limit: 0, shape: FewLarge, wantErr: true},
		{name: "negative limit", limit: -1, shape: ManySmall, wantErr: true},
		{name: "zero shape", limit: 4 << 20, shape: Shape(0), wantErr: true},
		{name: "unknown shape 99", limit: 4 << 20, shape: Shape(99), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records, recordBytes, err := layout(tc.limit, tc.shape)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("layout(%d, %s) = (%d, %d, nil), want an error", tc.limit, tc.shape, records, recordBytes)
				}
				return
			}
			if err != nil {
				t.Fatalf("layout(%d, %s): unexpected error: %v", tc.limit, tc.shape, err)
			}
			if records != tc.want.records || recordBytes != tc.want.recordBytes {
				t.Errorf("layout(%d, %s) = (%d, %d), want (%d, %d)", tc.limit, tc.shape, records, recordBytes, tc.want.records, tc.want.recordBytes)
			}
			total := records * recordBytes
			target := tc.limit + (tc.limit+3)/4
			if total < target {
				t.Errorf("total %d < target %d (limit + ceil(limit/4), at least 1.25x the limit)", total, target)
			}
			if total <= tc.limit {
				t.Errorf("total %d is not strictly greater than limit %d", total, tc.limit)
			}
			if recordBytes >= tc.limit {
				t.Errorf("recordBytes %d is not below limit %d", recordBytes, tc.limit)
			}
		})
	}
}

func TestCheckOversized(t *testing.T) {
	cases := []struct {
		name                        string
		limit, records, recordBytes int
		wantErr                     bool
	}{
		{name: "valid", limit: 100, records: 101, recordBytes: 1, wantErr: false},
		{name: "equal is not oversized", limit: 100, records: 100, recordBytes: 1, wantErr: true},
		{name: "exactly the limit", limit: 4 << 20, records: 32, recordBytes: 131072, wantErr: true},
		{name: "single record at the limit", limit: 100, records: 1, recordBytes: 100, wantErr: true},
		{name: "single record over the limit", limit: 100, records: 1, recordBytes: 101, wantErr: true},
		{name: "zero records", limit: 100, records: 0, recordBytes: 50, wantErr: true},
		{name: "zero recordBytes", limit: 100, records: 5, recordBytes: 0, wantErr: true},
		{name: "zero limit", limit: 0, records: 10, recordBytes: 10, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkOversized(tc.limit, tc.records, tc.recordBytes)
			if tc.wantErr && err == nil {
				t.Fatalf("checkOversized(%d, %d, %d) = nil, want an error", tc.limit, tc.records, tc.recordBytes)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("checkOversized(%d, %d, %d): unexpected error: %v", tc.limit, tc.records, tc.recordBytes, err)
			}
		})
	}
}

func TestValidateSpec(t *testing.T) {
	validVector := []float32{0.1, 0.2, 0.3}

	t.Run("nil store", func(t *testing.T) {
		_, _, err := validateSpec(nil, Spec{Limit: RecvLimit, Shape: FewLarge, Vector: validVector})
		if err == nil {
			t.Fatal("validateSpec(nil, ...) = nil error, want an error")
		}
	})

	t.Run("empty vector", func(t *testing.T) {
		st := newTestStore(t, nil, testCollection("validate_empty_vector"))
		_, _, err := validateSpec(st, Spec{Limit: RecvLimit, Shape: FewLarge, Vector: nil})
		if err == nil {
			t.Fatal("validateSpec with an empty Vector = nil error, want an error")
		}
	})

	invalidLayouts := []struct {
		name  string
		limit int
		shape Shape
	}{
		{name: "few-large below divisor", limit: 31, shape: FewLarge},
		{name: "many-small limit 1", limit: 1, shape: ManySmall},
		{name: "zero limit", limit: 0, shape: FewLarge},
		{name: "negative limit", limit: -1, shape: ManySmall},
		{name: "zero shape", limit: RecvLimit, shape: Shape(0)},
		{name: "unknown shape 99", limit: RecvLimit, shape: Shape(99)},
	}
	for _, tc := range invalidLayouts {
		t.Run(tc.name, func(t *testing.T) {
			st := newTestStore(t, nil, testCollection("validate_"+strings.ReplaceAll(tc.name, " ", "_")))
			_, _, err := validateSpec(st, Spec{Limit: tc.limit, Shape: tc.shape, Vector: validVector})
			if err == nil {
				t.Fatalf("validateSpec with invalid layout (limit=%d, shape=%s) = nil error, want an error", tc.limit, tc.shape)
			}
		})
	}

	t.Run("valid few-large at RecvLimit", func(t *testing.T) {
		st := newTestStore(t, nil, testCollection("validate"))
		records, recordBytes, err := validateSpec(st, Spec{Limit: RecvLimit, Shape: FewLarge, Vector: validVector})
		if err != nil {
			t.Fatalf("validateSpec: unexpected error: %v", err)
		}
		if records != 40 || recordBytes != 131072 {
			t.Errorf("validateSpec = (%d, %d, nil), want (40, 131072, nil)", records, recordBytes)
		}
	})
}

func TestFixtureIdentityIsUnique(t *testing.T) {
	scope1, owner1 := fixtureIdentity(FewLarge)
	scope2, owner2 := fixtureIdentity(FewLarge)

	if scope1 == scope2 {
		t.Errorf("fixtureIdentity(FewLarge) returned the same scope twice: %q", scope1)
	}
	if owner1 == owner2 {
		t.Errorf("fixtureIdentity(FewLarge) returned the same owner twice: %q", owner1)
	}

	const wantScopePrefix = "storetest-oversized:project:few-large-"
	if !strings.HasPrefix(scope1, wantScopePrefix) {
		t.Errorf("scope1 = %q, want prefix %q", scope1, wantScopePrefix)
	}
	if !strings.HasPrefix(scope2, wantScopePrefix) {
		t.Errorf("scope2 = %q, want prefix %q", scope2, wantScopePrefix)
	}
	const wantOwnerPrefix = "storetest-owner-"
	if !strings.HasPrefix(owner1, wantOwnerPrefix) {
		t.Errorf("owner1 = %q, want prefix %q", owner1, wantOwnerPrefix)
	}
	if !strings.HasPrefix(owner2, wantOwnerPrefix) {
		t.Errorf("owner2 = %q, want prefix %q", owner2, wantOwnerPrefix)
	}
}

func TestCreatedAtStrictlyIncreasing(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	seen := map[string]bool{}
	var prev time.Time
	for i := 0; i < 10; i++ {
		got := createdAtFor(base, i)
		if i > 0 && !got.Equal(prev.Add(time.Second)) {
			t.Errorf("createdAtFor(base, %d) = %v, want exactly one second after createdAtFor(base, %d) = %v", i, got, i-1, prev)
		}
		formatted := got.Format(time.RFC3339)
		if seen[formatted] {
			t.Errorf("createdAtFor(base, %d) formatted %q is a duplicate", i, formatted)
		}
		seen[formatted] = true
		prev = got
	}
	if len(seen) != 10 {
		t.Errorf("got %d distinct formatted timestamps, want 10", len(seen))
	}
}

// TestSeedOversizedShapes seeds one subtest per shape, in the order
// FewLarge, ManySmall, against real Qdrant. No subtest opts into parallel
// execution (D-12: at most one fixture live per package).
func TestSeedOversizedShapes(t *testing.T) {
	shapes := []Shape{FewLarge, ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := Dial(t, RecvLimit)
			ctx := context.Background()
			name := testCollection("seed_" + shape.String() + "_" + uuid.NewString())
			st := newTestStore(t, c, name)
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("EnsureCollection: %v", err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("DeleteCollection(%q): %v", name, err)
				}
			})

			fx := SeedOversized(t, st, Spec{Limit: RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			if fx.TotalBytes != len(fx.IDs)*fx.RecordBytes {
				t.Errorf("TotalBytes = %d, want len(IDs)*RecordBytes = %d", fx.TotalBytes, len(fx.IDs)*fx.RecordBytes)
			}
			if fx.TotalBytes <= RecvLimit {
				t.Errorf("TotalBytes = %d, want greater than RecvLimit %d", fx.TotalBytes, RecvLimit)
			}
			if fx.RecordBytes >= RecvLimit {
				t.Errorf("RecordBytes = %d, want below RecvLimit %d", fx.RecordBytes, RecvLimit)
			}
			wantLen := 40
			if shape == ManySmall {
				wantLen = 1000
			}
			if len(fx.IDs) != wantLen {
				t.Fatalf("len(IDs) = %d, want %d", len(fx.IDs), wantLen)
			}

			scopes, _, err := st.ListScopes(ctx, store.Authenticated(fx.Owner))
			if err != nil {
				t.Fatalf("ListScopes: %v", err)
			}
			var count uint64
			for _, sc := range scopes {
				if sc.Scope == fx.Scope {
					count = sc.Count
				}
			}
			if count != uint64(len(fx.IDs)) {
				t.Errorf("ListScopes count for scope %q = %d, want %d", fx.Scope, count, len(fx.IDs))
			}

			m0, err := st.Get(ctx, fx.IDs[0])
			if err != nil {
				t.Fatalf("Get(IDs[0]): %v", err)
			}
			m1, err := st.Get(ctx, fx.IDs[1])
			if err != nil {
				t.Fatalf("Get(IDs[1]): %v", err)
			}
			mLast, err := st.Get(ctx, fx.IDs[len(fx.IDs)-1])
			if err != nil {
				t.Fatalf("Get(IDs[last]): %v", err)
			}
			if !m1.CreatedAt.After(m0.CreatedAt) {
				t.Errorf("m1.CreatedAt (%v) is not after m0.CreatedAt (%v)", m1.CreatedAt, m0.CreatedAt)
			}
			if !mLast.CreatedAt.After(m1.CreatedAt) {
				t.Errorf("mLast.CreatedAt (%v) is not after m1.CreatedAt (%v)", mLast.CreatedAt, m1.CreatedAt)
			}
			if !m1.CreatedAt.Equal(m0.CreatedAt.Add(time.Second)) {
				t.Errorf("m1.CreatedAt (%v) is not exactly one second after m0.CreatedAt (%v)", m1.CreatedAt, m0.CreatedAt)
			}
		})
	}
}
