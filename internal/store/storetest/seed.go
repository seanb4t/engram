// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storetest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
)

// Shape names an oversized-fixture layout. The zero value is deliberately
// invalid, so a caller must choose one explicitly.
type Shape int

const (
	// ManySmall sizes the fixture so ONE page within internal/store's
	// exported store.MaxRecallLimit (1000) overflows the named limit — many
	// small records.
	ManySmall Shape = iota + 1
	// FewLarge sizes the fixture as #583's original shape — a few large
	// records, each roughly limit/32.
	FewLarge
)

// String returns "many-small", "few-large", or "Shape(<n>)" for any other
// value (including the invalid zero value).
func (s Shape) String() string {
	switch s {
	case ManySmall:
		return "many-small"
	case FewLarge:
		return "few-large"
	default:
		return fmt.Sprintf("Shape(%d)", int(s))
	}
}

// ManySmallRecords is the fixed record count for the ManySmall shape. It
// mirrors internal/store's exported store.MaxRecallLimit so ONE List page of
// a ManySmall fixture overflows the named limit (D-05); pinned against it by
// plan 01-04's TestManySmallShapeFitsOneListPage.
const ManySmallRecords = 1000

// fewLargeDivisor sizes the FewLarge shape's per-record byte count as
// limit/fewLargeDivisor — the #583 shape (roughly 128 KiB at 4 MiB).
const fewLargeDivisor = 32

// Spec parametrizes SeedOversized.
type Spec struct {
	// Limit is the caller-named receive limit under test — pass RecvLimit
	// (D-04).
	Limit int
	// Shape selects the fixture's record-count/record-size layout.
	Shape Shape
	// Vector is stored on every record and must match the collection's
	// configured dimension.
	Vector []float32
	// Template is copied for every record, letting a caller set Category,
	// Tags, scheduling windows, Scope, or Owner. The seeder always
	// overwrites ID, Content, and CreatedAt. An empty Template Scope or
	// Owner gets a fresh generated value; an empty Actor defaults to the
	// resolved Owner.
	Template store.Memory
}

// Fixture reports what SeedOversized actually wrote.
type Fixture struct {
	Shape Shape
	// Scope and Owner are the resolved values every seeded record shares.
	Scope, Owner string
	// IDs lists the seeded record ids in write order.
	IDs []string
	// RecordBytes is the per-record content byte count; TotalBytes is the
	// logical content bytes actually written (len(IDs) * RecordBytes).
	RecordBytes, TotalBytes int
}

// layout derives a (records, recordBytes) pair for shape that logically
// exceeds limit by at least 1.25x (D-05), using ceiling division only, and
// validates the result via checkOversized before returning it.
func layout(limit int, shape Shape) (records, recordBytes int, err error) {
	if limit <= 0 {
		return 0, 0, fmt.Errorf("storetest: limit must be a positive byte count naming the receive limit under test, got %d", limit)
	}
	// target is limit plus ceil(limit/4) — at least 1.25x the limit.
	target := limit + (limit+3)/4
	switch shape {
	case FewLarge:
		recordBytes = limit / fewLargeDivisor
		if recordBytes < 1 {
			return 0, 0, fmt.Errorf("storetest: limit %d is too small for shape %s (limit/%d rounds to a record size below 1 byte)", limit, shape, fewLargeDivisor)
		}
		records = ceilDiv(target, recordBytes)
	case ManySmall:
		records = ManySmallRecords
		recordBytes = ceilDiv(target, records)
	default:
		return 0, 0, fmt.Errorf("storetest: unknown shape %s", shape)
	}
	if cErr := checkOversized(limit, records, recordBytes); cErr != nil {
		return 0, 0, cErr
	}
	return records, recordBytes, nil
}

// ceilDiv returns ceil(a/b) for positive a and b using integer arithmetic
// only.
func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

// checkOversized reports whether (records, recordBytes) actually exceeds
// limit by accumulation: it rejects a non-positive limit, fewer than one
// record, a record smaller than one byte, a single record at or over the
// limit (oversized fixtures must overflow by accumulation, never by one
// oversized record — the deferred single-large-record content-cap
// question), and a logical total that does not STRICTLY exceed the limit
// (equal is not oversized).
func checkOversized(limit, records, recordBytes int) error {
	switch {
	case limit <= 0:
		return fmt.Errorf("storetest: limit must be a positive byte count naming the receive limit under test, got %d", limit)
	case records < 1:
		return fmt.Errorf("storetest: records must be at least 1, got %d", records)
	case recordBytes < 1:
		return fmt.Errorf("storetest: recordBytes must be at least 1, got %d", recordBytes)
	case recordBytes >= limit:
		return fmt.Errorf("storetest: a single record of %d bytes is at or over the named limit %d bytes — oversized fixtures must overflow by accumulation, not by one oversized record", recordBytes, limit)
	case records*recordBytes <= limit:
		return fmt.Errorf("storetest: logical total %d bytes (%d records x %d bytes/record) does not exceed the named limit %d bytes", records*recordBytes, records, recordBytes, limit)
	default:
		return nil
	}
}

// validateSpec rejects a nil store or an empty Vector before delegating to
// layout for spec's Limit/Shape.
func validateSpec(st *store.Store, spec Spec) (records, recordBytes int, err error) {
	if st == nil {
		return 0, 0, fmt.Errorf("storetest: SeedOversized requires a non-nil *store.Store")
	}
	if len(spec.Vector) == 0 {
		return 0, 0, fmt.Errorf("storetest: SeedOversized requires a non-empty Spec.Vector")
	}
	return layout(spec.Limit, spec.Shape)
}

// fixtureIdentity mints a fresh scope/owner pair for shape, so two
// SeedOversized calls in the same collection never merge unless the
// caller's Template deliberately supplies one.
func fixtureIdentity(shape Shape) (scope, owner string) {
	scope = fmt.Sprintf("storetest-oversized:project:%s-%s", shape, uuid.NewString())
	owner = "storetest-owner-" + uuid.NewString()
	return scope, owner
}

// createdAtFor returns base plus i seconds — the per-record CreatedAt
// SeedOversized assigns, strictly increasing and second-distinct in write
// order.
func createdAtFor(base time.Time, i int) time.Time {
	return base.Add(time.Duration(i) * time.Second)
}

// SeedOversized seeds, through public Store.Upsert only, a fixture whose
// logical content bytes strictly exceed spec.Limit — in the shape spec.Shape
// names. It validates spec before writing anything, holds the phase's
// single -short skip via the testing package (every oversized test inherits
// it, D-12),
// registers cleanup via Store.DeleteAll BEFORE its first write so a partial
// seed is reclaimed, writes every record sequentially (deterministic write
// order and CreatedAt), and self-asserts what it actually wrote before
// returning the Fixture. It never reaches for a raw *qdrant.Client (D-08).
func SeedOversized(t testing.TB, st *store.Store, spec Spec) Fixture {
	t.Helper()

	records, recordBytes, err := validateSpec(st, spec)
	if err != nil {
		t.Fatalf("SeedOversized: %v", err)
	}

	if testing.Short() {
		t.Skipf("SeedOversized writes an oversized %s fixture (%d records x %d bytes/record, over the named %d-byte limit); skipped in -short", spec.Shape, records, recordBytes, spec.Limit)
	}

	scope, owner := spec.Template.Scope, spec.Template.Owner
	if scope == "" || owner == "" {
		fiScope, fiOwner := fixtureIdentity(spec.Shape)
		if scope == "" {
			scope = fiScope
		}
		if owner == "" {
			owner = fiOwner
		}
	}

	// Registered BEFORE the first write, so a Fatalf mid-loop still reclaims
	// whatever was already seeded.
	t.Cleanup(func() {
		if delErr := st.DeleteAll(context.Background(), scope, store.Authenticated(owner)); delErr != nil {
			t.Errorf("SeedOversized cleanup: DeleteAll(%q): %v", scope, delErr)
		}
	})

	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Duration(records) * time.Second)
	content := strings.Repeat("x", recordBytes)

	start := time.Now()
	ids := make([]string, 0, records)
	totalBytes := 0
	for i := 0; i < records; i++ {
		m := spec.Template
		m.ID = uuid.NewString()
		m.Content = content
		m.Scope = scope
		m.Owner = owner
		if m.Actor == "" {
			m.Actor = owner
		}
		m.CreatedAt = createdAtFor(base, i)
		if upErr := st.Upsert(ctx, m, spec.Vector); upErr != nil {
			t.Fatalf("SeedOversized: upsert %s record %d: %v", spec.Shape, i, upErr)
		}
		ids = append(ids, m.ID)
		totalBytes += len(m.Content)
	}
	elapsed := time.Since(start)

	if cErr := checkOversized(spec.Limit, len(ids), recordBytes); cErr != nil {
		t.Fatalf("SeedOversized: %v", cErr)
	}
	if totalBytes != len(ids)*recordBytes {
		t.Fatalf("SeedOversized: logical total %d bytes != len(ids)*recordBytes (%d*%d=%d)", totalBytes, len(ids), recordBytes, len(ids)*recordBytes)
	}

	t.Logf("storetest: seeded %s fixture: %d records, %d bytes/record, %d total bytes, limit %d bytes, in %s", spec.Shape, len(ids), recordBytes, totalBytes, spec.Limit, elapsed)

	return Fixture{
		Shape:       spec.Shape,
		Scope:       scope,
		Owner:       owner,
		IDs:         ids,
		RecordBytes: recordBytes,
		TotalBytes:  totalBytes,
	}
}
