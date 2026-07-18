// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
	"time"
)

// TestServicePrincipalIsolation proves tenancy isolation (#373 / SC4 / D-07)
// for service principals against the store filters Phase 22 already wired —
// ZERO new store code. A client-credentials/static-token-resolved owner (a
// namespacedOwner-encoded string, e.g. "9:client_id:6:svc-aa") is isolated to
// its own owner bucket for private records exactly like any other
// authenticated owner: it never collides with a human owner or the anonymous
// bucket, recalls empty (not an error) when it owns nothing, and the
// isolation outcome is independent of insertion order.
func TestServicePrincipalIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:service-principal"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	const (
		// serviceA/serviceB mirror the namespacedOwner("claim", "value")
		// injective length-prefix encoding (internal/auth/auth.go) that both
		// the OIDC client-credentials lane and the static-token lane produce.
		// The store treats the owner as an opaque authz key, so these literal
		// strings exercise the real shape without importing internal/auth.
		serviceA = "9:client_id:6:svc-aa"     // client-credentials-resolved tenant A
		serviceB = "12:static_token:6:svc-bb" // static-token-resolved tenant B
		serviceC = "9:client_id:6:svc-cc"     // owns nothing (empty-input case)
		human    = "person@example.com"
	)

	// Adjacency (SC4): a service owner string never equals a human owner nor
	// the empty anonymous bucket — structurally distinct by the length-prefix
	// namespacedOwner shape.
	for _, owner := range []string{serviceA, serviceB, serviceC} {
		if owner == human {
			t.Fatalf("service owner %q collides with human owner", owner)
		}
		if owner == "" {
			t.Fatalf("service owner must never be empty (anonymous-bucket collision)")
		}
	}

	mk := func(id, owner, vis string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("cccccccc-0000-0000-0000-000000000001", serviceA, "")       // A private
	mk("cccccccc-0000-0000-0000-000000000002", serviceB, "")       // B private
	mk("cccccccc-0000-0000-0000-000000000003", human, "")          // human private
	mk("cccccccc-0000-0000-0000-000000000004", serviceB, "shared") // B shared

	// A sees only its own private record + B's shared record (2), never B's
	// or the human's private records.
	hits, err := s.Search(ctx, scope, Authenticated(serviceA), []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("Search(serviceA): got %d want 2", len(hits))
	}
	for _, h := range hits {
		if h.Owner != serviceA && h.Visibility != "shared" {
			t.Errorf("leaked another owner's private record: id=%s owner=%s vis=%s", h.ID, h.Owner, h.Visibility)
		}
	}

	// List honors the same filter.
	lst, _, _, err := s.List(ctx, scope, Authenticated(serviceA), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 2 {
		t.Errorf("List(serviceA): got %d want 2", len(lst))
	}

	// Empty-input: a service principal that owns nothing recalls zero hits,
	// not an error. Uses a DEDICATED scope with no `shared` records at all —
	// the main scope above has a serviceB `shared` record which ANY
	// authenticated caller (including serviceC) is entitled to see under
	// D-15's global shared-read grant, so it would falsely pollute a
	// zero-hits assertion here (that visibility is proven separately by
	// TestSharedCrossTenantReadIntended).
	emptyScope := "iso-test:project:service-principal-empty"
	defer func() { cleanupErr(t, "DeleteAllRaw "+emptyScope, s.DeleteAllRaw(ctx, emptyScope)) }()
	mkEmpty := func(id, owner, vis string) {
		m := Memory{ID: id, Content: "x", Scope: emptyScope, Owner: owner, Visibility: vis,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mkEmpty("ffffffff-0000-0000-0000-000000000001", serviceA, "") // A private, no shared records in this scope
	mkEmpty("ffffffff-0000-0000-0000-000000000002", human, "")    // human private

	emptyHits, err := s.Search(ctx, emptyScope, Authenticated(serviceC), []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("search(serviceC): %v", err)
	}
	if len(emptyHits) != 0 {
		t.Fatalf("Search(serviceC): got %d want 0 (owns nothing)", len(emptyHits))
	}
	emptyList, _, _, err := s.List(ctx, emptyScope, Authenticated(serviceC), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list(serviceC): %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("List(serviceC): got %d want 0 (owns nothing)", len(emptyList))
	}

	// Ordering independence: re-seed a fresh scope with insertion order
	// reversed relative to above (serviceB's shared record first, serviceA's
	// private record last) and re-assert the same isolation outcome.
	scope2 := "iso-test:project:service-principal-reorder"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope2, s.DeleteAllRaw(ctx, scope2)) }()
	mk2 := func(id, owner, vis string) {
		m := Memory{ID: id, Content: "x", Scope: scope2, Owner: owner, Visibility: vis,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk2("dddddddd-0000-0000-0000-000000000001", serviceB, "shared") // B shared, seeded FIRST this time
	mk2("dddddddd-0000-0000-0000-000000000002", human, "")          // human private
	mk2("dddddddd-0000-0000-0000-000000000003", serviceB, "")       // B private
	mk2("dddddddd-0000-0000-0000-000000000004", serviceA, "")       // A private, seeded LAST

	reorderHits, err := s.Search(ctx, scope2, Authenticated(serviceA), []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("search(reorder): %v", err)
	}
	if len(reorderHits) != 2 {
		t.Fatalf("Search(reorder serviceA): got %d want 2", len(reorderHits))
	}
	for _, h := range reorderHits {
		if h.Owner != serviceA && h.Visibility != "shared" {
			t.Errorf("leaked another owner's private record under reordered insertion: id=%s owner=%s vis=%s", h.ID, h.Owner, h.Visibility)
		}
	}
}
