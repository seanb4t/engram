// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
)

// TestSchemaVersionForwardBackwardCompat proves ROADMAP success criterion 5
// against real Qdrant: a binary reads a record whose schema_version is
// NEWER than its own migrate.CurrentVersion without rejecting, hiding, or
// downgrading it, and mirrors the same proof for the older-than-binary
// direction. Every injected/expected version is derived from
// migrate.CurrentVersion — a test-only override of the constant was
// considered and rejected as primary proof, because rollback safety
// actually depends on DECODING an unfamiliar payload, not on the stamping
// plumbing that produced it. All injection goes through the raw Qdrant
// client (SetPayload/DeletePayload), bypassing payload() entirely.
//
// The row set is derived from migrate.CurrentVersion, not hardcoded: two
// rows ("absent", "newer") while the constant is 0, a third ("older-
// explicit") once it is raised. See compatRow and the row-construction
// block below.
func TestSchemaVersionForwardBackwardCompat(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("schemaversion_compat")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	// The derived expected row count: 2 while CurrentVersion is 0 (this
	// phase), 3 once a later phase raises it. This is computed
	// independently of the rows slice built below so the count assertion
	// after the loop is a real cross-check, not a tautology.
	expectedRowCount := 2
	if migrate.CurrentVersion > 0 {
		expectedRowCount = 3
	}

	// The genuine older-than-binary state at CurrentVersion == 0 is the
	// key-absent legacy record — CurrentVersion - 1 is not representable
	// (see this plan's "The older-than direction at CurrentVersion = 0").
	// "absent" always runs and is the sole older-than-binary proof until
	// the constant is raised.
	rows := []compatRow{
		{
			name:              "absent",
			omitVersionKey:    true,
			postUpdateVersion: migrate.CurrentVersion,
			coversOlderThan:   true,
		},
	}
	// "older-explicit" activates itself the moment a later phase raises
	// migrate.CurrentVersion above 0 — no code here needs to change for
	// that to happen; only the constant's value does.
	if migrate.CurrentVersion > 0 {
		rows = append(rows, compatRow{
			name:              "older-explicit",
			schemaVersion:     migrate.CurrentVersion - 1,
			postUpdateVersion: migrate.CurrentVersion,
		})
	}
	rows = append(rows, compatRow{
		name:              "newer",
		schemaVersion:     migrate.CurrentVersion + 1,
		injectUnknownKeys: true,
		postUpdateVersion: migrate.CurrentVersion + 1,
	})

	if len(rows) != expectedRowCount {
		t.Fatalf("row-set derivation is inconsistent: built %d rows %v, want %d (derived from migrate.CurrentVersion=%d)",
			len(rows), rowNames(rows), expectedRowCount, migrate.CurrentVersion)
	}

	var (
		executedNames    = make([]string, 0, len(rows))
		olderThanCovered bool
	)
	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			runCompatRow(ctx, t, s, row)
		})
		executedNames = append(executedNames, row.name)
		// The older-than-binary coverage claim is a CHECKED value, not a
		// comment: while CurrentVersion is 0, "absent" running (with its
		// coversOlderThan flag) is what satisfies it; once the constant is
		// raised, only "older-explicit" actually running satisfies it —
		// so this assertion fails on its own the day someone raises
		// migrate.CurrentVersion without wiring the third row's execution.
		if migrate.CurrentVersion == 0 && row.name == "absent" && row.coversOlderThan {
			olderThanCovered = true
		}
		if migrate.CurrentVersion > 0 && row.name == "older-explicit" {
			olderThanCovered = true
		}
	}

	if len(executedNames) != expectedRowCount {
		t.Fatalf("executed %d compatibility rows %v, want %d — a row silently vanished or an extra one ran",
			len(executedNames), executedNames, expectedRowCount)
	}
	if !olderThanCovered {
		t.Fatalf("older-than-binary direction coverage claim unmet: no executed row asserts it (migrate.CurrentVersion=%d, executed=%v)",
			migrate.CurrentVersion, executedNames)
	}
	t.Logf("executed compatibility rows: %v (expected %d, derived from migrate.CurrentVersion=%d)",
		executedNames, expectedRowCount, migrate.CurrentVersion)
}

// compatRow is one row of the forward/backward compatibility table. Both
// records in a row (see runCompatRow) are seeded identically per this row's
// shape, so a single injected version and unknown-key set proves one claim
// across both the active and the pending-windowed population.
type compatRow struct {
	name string
	// schemaVersion is the schema_version raw-injected onto both records.
	// Ignored when omitVersionKey is true.
	schemaVersion migrate.Version
	// omitVersionKey selects the "absent" row's shape: the schema_version
	// key is raw-DELETED from both points rather than set, producing the
	// genuine key-absent legacy-record state Store.Upsert can never
	// construct on its own (payload() always writes the key).
	omitVersionKey bool
	// injectUnknownKeys is true only for the "newer" row: it also injects
	// two payload keys this binary has no Memory field for, of two
	// different value types, proving fromPayload's tolerant decode without
	// corrupting neighbouring known fields.
	injectUnknownKeys bool
	// postUpdateVersion is the SchemaVersion expected on the active record
	// after a normal Store.Update, per D-05's monotonic-maximum rule.
	postUpdateVersion migrate.Version
	// coversOlderThan marks a row as (potentially) satisfying the
	// older-than-binary coverage claim. Only "absent" sets it; whether it
	// actually counts depends on migrate.CurrentVersion (see the coverage
	// check in the caller).
	coversOlderThan bool
}

func rowNames(rows []compatRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.name
	}
	return out
}

// compatOwner is the shared owner/subject for every record this test seeds.
// Every row's two records are owned by the same subject so the same
// Authenticated subject drives every recall path across every row.
const compatOwner = "sub-schemaversion-compat"

// compatRecordKind discriminates the two records every row seeds.
type compatRecordKind int

const (
	compatActive compatRecordKind = iota
	compatWindowed
)

func (k compatRecordKind) String() string {
	if k == compatWindowed {
		return "windowed"
	}
	return "active"
}

// compatRecordID returns a deterministic, valid-UUID point id for a given
// row name and record kind. The "cf0" prefix is unique to this file within
// internal/store's test suite; collisions are additionally impossible
// because this test owns a dedicated collection (schemaversion_compat),
// never a shared one.
func compatRecordID(rowName string, kind compatRecordKind) string {
	return fmt.Sprintf("cf000000-0000-0000-0000-%s%05d", compatRowHexTag(rowName), int(kind))
}

// compatRowHexTag maps a row name to a fixed 7-hex-digit tag so
// compatRecordID produces a stable, readable, collision-free id per row.
func compatRowHexTag(rowName string) string {
	switch rowName {
	case "absent":
		return "1000000"
	case "older-explicit":
		return "2000000"
	case "newer":
		return "3000000"
	default:
		// Unreached by any row this test constructs; a distinct tag keeps
		// a future row from silently colliding with an existing one.
		return "9000000"
	}
}

// schemaVersionCompatMatrix is the shared per-path applicability matrix
// every row's recall assertions are driven from (Task 2 reuses it
// verbatim): the temporal reachability of a record depends on its WINDOW,
// not its VERSION, so this table is the same for every row and only the
// version expectations differ per row.
//
// The five recall paths do NOT share recall semantics:
//   - Search/SearchReranked/List apply activeWindowConditions (store.go);
//     the pending "windowed" record fails that gate.
//   - SearchDiscovery applies NO temporal gate at all (store.go:1181-1200),
//     so the pending record is visible there while hidden from
//     Search/List — not a contradiction, the direct consequence of that
//     path's filter shape.
//   - ListScheduled applies the INVERSE window clause (pending or
//     expired): the active record, having no window, never matches it.
var schemaVersionCompatMatrix = []struct {
	record compatRecordKind
	path   string
	want   bool
	reason string
}{
	{compatActive, "Search", true, "active record has no temporal window (or a currently-valid one): activeWindowConditions passes"},
	{compatActive, "SearchReranked", true, "wraps Search; same active-window gate, same result"},
	{compatActive, "SearchDiscovery", true, "category=discovery; SearchDiscovery applies no temporal gate at all"},
	{compatActive, "List", true, "active record has no temporal window (or a currently-valid one): activeWindowConditions passes"},
	{compatActive, "ListScheduled", false, "ListScheduled selects only pending/expired records; the active record is neither"},

	{compatWindowed, "Search", false, "pending (not_before in the future): excluded by activeWindowConditions"},
	{compatWindowed, "SearchReranked", false, "wraps Search; same active-window gate excludes the pending record"},
	{compatWindowed, "SearchDiscovery", true, "SearchDiscovery applies no temporal gate at all (store.go:1181-1200)"},
	{compatWindowed, "List", false, "pending (not_before in the future): excluded by activeWindowConditions"},
	{compatWindowed, "ListScheduled", true, "pending (now < not_before): selected by scheduledStateCondition"},
}

// compatRecallLimit is the k/Limit every recall call in this test uses. Each
// row's scope holds at most two records, so any limit at or above 2 returns
// every readable record — membership is asserted by id, never by count.
const compatRecallLimit = 10

// runCompatRow seeds a row's two records (active, windowed), raw-injects
// the row's version shape onto both identically, and asserts every claim
// this plan makes: decode success, known-field survival, per-path recall
// membership, and (for "newer") the accepted D-06 unknown-key loss on a
// subsequent Store.Update.
func runCompatRow(ctx context.Context, t *testing.T, s *Store, row compatRow) {
	t.Helper()
	subj := Authenticated(compatOwner)
	scope := fmt.Sprintf("schemaversion-compat:project:%s", row.name)

	activeID := compatRecordID(row.name, compatActive)
	windowedID := compatRecordID(row.name, compatWindowed)
	activeVec := []float32{0.11, 0.22, 0.33}
	windowedVec := []float32{0.44, 0.55, 0.66}

	activeMem := Memory{
		ID: activeID, Content: "active discovery record for row " + row.name,
		Scope: scope, Owner: compatOwner, Category: "discovery", Kind: "fact",
		CreatedAt: time.Now().UTC(),
	}
	future := time.Now().UTC().Add(time.Hour)
	windowedMem := Memory{
		ID: windowedID, Content: "windowed discovery record for row " + row.name,
		Scope: scope, Owner: compatOwner, Category: "discovery", Kind: "fact",
		CreatedAt: time.Now().UTC(), NotBefore: &future,
	}

	// Seed through the normal Store.Upsert path — both records get a valid
	// vector and every field the recall paths need. payload() also
	// unconditionally stamps schema_version here; the raw injection below
	// is what OVERRIDES that stamp to construct this row's shape.
	if err := s.Upsert(ctx, activeMem, activeVec); err != nil {
		t.Fatalf("seed active: %v", err)
	}
	if err := s.Upsert(ctx, windowedMem, windowedVec); err != nil {
		t.Fatalf("seed windowed: %v", err)
	}

	// unknownKV is empty for every row except "newer": two payload keys of
	// two different value types with no corresponding Memory field, so the
	// decode is exercised against more than one shape.
	unknownKV := map[string]any{}
	if row.injectUnknownKeys {
		unknownKV["future_capability_enabled"] = true
		unknownKV["future_capability_note"] = "a payload key this binary has no struct field for"
	}

	// Raw-inject onto BOTH points identically — this identity is what
	// makes the two populations one version claim rather than two.
	for _, id := range []string{activeID, windowedID} {
		if row.omitVersionKey {
			// The "absent" row's shape can never come from Store.Upsert
			// (payload() always writes schema_version): raw-DELETE the key
			// so the point genuinely has none — the state of every record
			// written before this milestone.
			deleteRawPayloadKeys(ctx, t, s, id, []string{schemaVersionKey})
			continue
		}
		kv := map[string]any{schemaVersionKey: int(row.schemaVersion)}
		for k, v := range unknownKV {
			kv[k] = v
		}
		injectRawPayload(ctx, t, s, id, qdrant.NewValueMap(kv))
	}

	// 1. Decodes without error, and 2. known fields survive alongside any
	// unknown keys — asserted via Store.Get, never a panic.
	expectedVersion := row.schemaVersion
	if row.omitVersionKey {
		// The absent-key decode contract (D-09) is fixed at the Go zero
		// value regardless of migrate.CurrentVersion's value — this is
		// NOT a version derived from the constant, it is the guarantee
		// that "unset" and "v0" are the same state.
		expectedVersion = migrate.Version(0)
	}

	gotActive, err := s.Get(ctx, activeID)
	if err != nil {
		t.Fatalf("Get active: %v", err)
	}
	if gotActive.SchemaVersion != expectedVersion {
		t.Fatalf("active SchemaVersion = %d, want %d", gotActive.SchemaVersion, expectedVersion)
	}
	assertKnownFieldsIntact(t, "active", gotActive, activeMem)

	gotWindowed, err := s.Get(ctx, windowedID)
	if err != nil {
		t.Fatalf("Get windowed: %v", err)
	}
	if gotWindowed.SchemaVersion != expectedVersion {
		t.Fatalf("windowed SchemaVersion = %d, want %d", gotWindowed.SchemaVersion, expectedVersion)
	}
	assertKnownFieldsIntact(t, "windowed", gotWindowed, windowedMem)

	if row.omitVersionKey {
		// The absent-key claim is proven on the RAW stored payload, not
		// merely on the decoded zero value — a decoded zero could also
		// mean "the key was present and literally 0".
		for _, id := range []string{activeID, windowedID} {
			raw := rawPayload(ctx, t, s, id)
			if _, ok := raw[schemaVersionKey]; ok {
				t.Fatalf("raw payload for %s still carries %q after DeletePayload; want the key genuinely absent", id, schemaVersionKey)
			}
		}
	}

	// 3./4. Recallable per the applicability matrix, and not hidden for
	// the wrong reason — driven from the shared matrix, never re-derived
	// per row (Task 2 reuses this verbatim).
	assertApplicabilityMatrix(ctx, t, s, subj, scope, activeID, windowedID, activeVec)

	// 5. Not downgraded by a later write; the accepted D-06 unknown-key
	// loss is asserted rather than assumed. 6. "Never rejected" is proven
	// by every call above returning a nil error and a correctly typed
	// result — no error-text search for a version-mismatch phrase is
	// performed anywhere in this file. This diverges deliberately from
	// internal/webauth's sessionPayloadVersion, which hard-rejects a
	// mismatched session payload version outright
	// (internal/webauth/resolver.go, internal/webauth/reseal.go) —
	// REQ-schema-version-wire-visible chose the opposite behaviour for
	// records on purpose.
	curActive, err := s.FetchForUpdate(ctx, activeID, subj)
	if err != nil {
		t.Fatalf("FetchForUpdate active: %v", err)
	}
	if err := s.Update(ctx, curActive, activeMem.Content+" (updated)", nil, nil, nil, activeVec); err != nil {
		t.Fatalf("Update active: %v", err)
	}
	updatedActive, err := s.Get(ctx, activeID)
	if err != nil {
		t.Fatalf("Get active after Update: %v", err)
	}
	// Stamp preservation only — the compatibility claim after Update is
	// scoped to the version, never to unknown-key survival.
	if updatedActive.SchemaVersion != row.postUpdateVersion {
		t.Fatalf("post-Update active SchemaVersion = %d, want %d (D-05's monotonic maximum, observed on the exact rollback path)",
			updatedActive.SchemaVersion, row.postUpdateVersion)
	}
	if updatedActive.Content != activeMem.Content+" (updated)" {
		t.Errorf("post-Update active Content = %q, want the edit to have landed", updatedActive.Content)
	}

	if row.injectUnknownKeys {
		// D-06's accepted limitation, CHECKED rather than glossed:
		// fromPayload never decoded the future-only keys into the Go
		// struct, and payload() never re-emits what it never decoded, so
		// Store.Update's whole-payload rewrite drops them — while the
		// higher stamp above survives. Acceptable because migration steps
		// are additive-only, what is lost is re-derivable, and the
		// recovery is re-running the migration sweep — which does not
		// ship until Phase 3/4.
		rawAfterUpdate := rawPayload(ctx, t, s, activeID)
		for k := range unknownKV {
			if _, ok := rawAfterUpdate[k]; ok {
				t.Errorf("raw payload after Update still carries future-only key %q; D-06's accepted unknown-key loss did not occur", k)
			}
		}
	} else {
		// This row injects no unknown keys, so it carries no
		// unknown-key-loss assertion — deliberate, not an oversight.
		_ = unknownKV
	}
}

// assertKnownFieldsIntact asserts the decoded record's known fields survive
// alongside whatever version/unknown-key shape was raw-injected — the
// unknown key must be ignored, never allowed to corrupt a neighbour.
func assertKnownFieldsIntact(t *testing.T, label string, got, want Memory) {
	t.Helper()
	if got.Content != want.Content {
		t.Errorf("%s Content = %q, want %q", label, got.Content, want.Content)
	}
	if got.Scope != want.Scope {
		t.Errorf("%s Scope = %q, want %q", label, got.Scope, want.Scope)
	}
	if got.Category != want.Category {
		t.Errorf("%s Category = %q, want %q", label, got.Category, want.Category)
	}
	if got.Owner != want.Owner {
		t.Errorf("%s Owner = %q, want %q", label, got.Owner, want.Owner)
	}
}

// assertApplicabilityMatrix drives schemaVersionCompatMatrix against one
// row's two seeded records: it runs each of the five recall paths exactly
// once, asserts membership-by-id per the matrix's boolean+reason, and
// asserts the executed entry count equals the matrix's enumerated entry
// count so a (record, path) pair can never silently vanish.
func assertApplicabilityMatrix(ctx context.Context, t *testing.T, s *Store, subj Subject, scope, activeID, windowedID string, queryVec []float32) {
	t.Helper()

	searchHits, err := s.Search(ctx, scope, subj, queryVec, compatRecallLimit, SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	rerankedHits, err := s.SearchReranked(ctx, scope, subj, "discovery record", queryVec, compatRecallLimit, SearchOptions{})
	if err != nil {
		t.Fatalf("SearchReranked: %v", err)
	}
	discoveryHits, err := s.SearchDiscovery(ctx, scope, "", subj, queryVec, compatRecallLimit)
	if err != nil {
		t.Fatalf("SearchDiscovery: %v", err)
	}
	listItems, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: compatRecallLimit})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	scheduledItems, err := s.ListScheduled(ctx, scope, subj, ScheduledAll, ListOptions{Limit: compatRecallLimit})
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}

	membership := map[string]map[compatRecordKind]bool{
		"Search":          idMembership(searchHits, activeID, windowedID),
		"SearchReranked":  idMembership(rerankedHits, activeID, windowedID),
		"SearchDiscovery": idMembership(discoveryHits, activeID, windowedID),
		"List":            idMembership(listItems, activeID, windowedID),
		"ListScheduled":   idMembership(scheduledItems, activeID, windowedID),
	}

	executed := 0
	for _, entry := range schemaVersionCompatMatrix {
		got := membership[entry.path][entry.record]
		executed++
		if got != entry.want {
			t.Errorf("%s membership in %s = %v, want %v (%s)", entry.record, entry.path, got, entry.want, entry.reason)
		}
	}
	if executed != len(schemaVersionCompatMatrix) {
		t.Fatalf("executed %d applicability-matrix entries, want %d (a (record, path) pair silently vanished)",
			executed, len(schemaVersionCompatMatrix))
	}
}

// idMembership reports, for a []Memory result set, whether activeID and
// windowedID are present by id — never by count, and never by position.
func idMembership(ms []Memory, activeID, windowedID string) map[compatRecordKind]bool {
	present := make(map[string]bool, len(ms))
	for _, m := range ms {
		present[m.ID] = true
	}
	return map[compatRecordKind]bool{
		compatActive:   present[activeID],
		compatWindowed: present[windowedID],
	}
}

// injectRawPayload writes kv directly onto the point at id via the raw
// Qdrant client's SetPayload, deliberately bypassing payload(). This is
// what makes this test's evidence about DECODING an unfamiliar payload
// rather than about the stamping plumbing — a test-only override of the
// version constant was considered and rejected as primary proof for
// exactly that reason.
func injectRawPayload(ctx context.Context, t *testing.T, s *Store, id string, kv map[string]*qdrant.Value) {
	t.Helper()
	if _, err := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        kv,
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	}); err != nil {
		t.Fatalf("injectRawPayload(%s): %v", id, err)
	}
}

// deleteRawPayloadKeys removes keys directly from the point at id via the
// raw Qdrant client's DeletePayload — the only way to construct the
// genuinely key-absent legacy-record shape, since Store.Upsert's payload()
// unconditionally writes schema_version.
func deleteRawPayloadKeys(ctx context.Context, t *testing.T, s *Store, id string, keys []string) {
	t.Helper()
	if _, err := s.client.DeletePayload(ctx, &qdrant.DeletePayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Keys:           keys,
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	}); err != nil {
		t.Fatalf("deleteRawPayloadKeys(%s): %v", id, err)
	}
}

// rawPayload reads a point's payload directly via the raw Qdrant client —
// used to assert the RAW stored shape (key presence/absence, unknown-key
// survival) rather than the decoded Memory, which fromPayload's guarded
// reads would otherwise mask.
func rawPayload(ctx context.Context, t *testing.T, s *Store, id string) map[string]*qdrant.Value {
	t.Helper()
	pts, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		t.Fatalf("rawPayload Get(%s): %v", id, err)
	}
	if len(pts) == 0 {
		t.Fatalf("rawPayload Get(%s): point not found", id)
	}
	return pts[0].Payload
}
