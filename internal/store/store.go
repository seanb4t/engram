// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package store persists and queries memories as vectors in a Qdrant collection.
package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// ErrNotFound is returned when an id is absent OR not visible to the caller —
// the two are indistinguishable by design, so ownership never leaks across actors.
var ErrNotFound = errors.New("not found")

// visibilityShared is the Visibility sentinel for a record readable by any
// authenticated caller. Sharing grants read, never write. Defined once so a typo
// in an authorization path is a compile error rather than a silent gate bypass.
const visibilityShared = "shared"

// Memory is the unit of storage. Fields map 1:1 to Qdrant payload keys.
type Memory struct {
	ID        string   `json:"id"`
	Content   string   `json:"content"`
	Scope     string   `json:"scope"` // run:tier:repo, e.g. eval-2026-05:project:selfhosted-cluster
	Repo      string   `json:"repo"`
	Workspace string   `json:"workspace"`
	Worktree  string   `json:"worktree_path"`
	BaseDir   string   `json:"base_dir"`
	Source    string   `json:"source"` // user-said | agent-inferred
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	// Actor is the verified caller identity (email/username/subject) taken from
	// the validated OIDC token — never client-supplied. Empty when auth is off.
	Actor string `json:"actor"`
	// Owner is the stable OIDC subject (`sub`) of the caller — the authorization
	// key. Server-set from the validated token, never client-supplied. Empty when
	// auth is disabled (the single anonymous bucket).
	Owner string `json:"owner"`
	// Visibility gates cross-actor reads: "" (private, default) or "shared"
	// (readable by any authenticated caller). Writes always require ownership.
	Visibility string    `json:"visibility,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	// Discovery-only (zero-valued for the curated four categories).
	Kind      string     `json:"kind,omitempty"`      // "map" | "fact"
	Citations []Citation `json:"citations,omitempty"` // >= 1 for discoveries
	Summary   string     `json:"summary,omitempty"`
}

// Citation anchors a discovery to a source so it can be verified and aged.
type Citation struct {
	Kind    string `json:"kind"`              // file | commit | url | repo
	Ref     string `json:"ref"`               // path / repo URL / doc URL
	Locator string `json:"locator,omitempty"` // e.g. "200-240" line range
	Pin     string `json:"pin,omitempty"`     // aging anchor captured at store time
	Excerpt string `json:"excerpt,omitempty"` // cached substance
}

// Store persists and queries memories in a Qdrant collection.
type Store struct {
	client     *qdrant.Client
	collection string
}

// New returns a Store backed by the given Qdrant client and collection.
func New(c *qdrant.Client, collection string) *Store {
	return &Store{client: c, collection: collection}
}

// EnsureCollection is idempotent: creates the collection at the given vector size if absent.
func (s *Store) EnsureCollection(ctx context.Context, dim uint64) error {
	exists, err := s.client.CollectionExists(ctx, s.collection)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: s.collection,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size: dim, Distance: qdrant.Distance_Cosine,
		}),
	})
}

func payload(m Memory) map[string]any {
	tags := make([]any, len(m.Tags))
	for i, t := range m.Tags {
		tags[i] = t
	}
	p := map[string]any{
		"content":       m.Content,
		"scope":         m.Scope,
		"repo":          m.Repo,
		"workspace":     m.Workspace,
		"worktree_path": m.Worktree,
		"base_dir":      m.BaseDir,
		"source":        m.Source,
		"category":      m.Category,
		"tags":          tags,
		"actor":         m.Actor,
		"owner":         m.Owner,
		"visibility":    m.Visibility,
		"created_at":    m.CreatedAt.Format(time.RFC3339),
	}
	if m.Category == "discovery" {
		p["kind"] = m.Kind
		p["summary"] = m.Summary
		cites := make([]any, len(m.Citations))
		for i, c := range m.Citations {
			cites[i] = map[string]any{
				"kind": c.Kind, "ref": c.Ref, "locator": c.Locator,
				"pin": c.Pin, "excerpt": c.Excerpt,
			}
		}
		p["citations"] = cites
	}
	return p
}

func fromPayload(id string, p map[string]*qdrant.Value) Memory {
	m := Memory{ID: id}
	if v, ok := p["content"]; ok {
		m.Content = v.GetStringValue()
	}
	if v, ok := p["scope"]; ok {
		m.Scope = v.GetStringValue()
	}
	if v, ok := p["repo"]; ok {
		m.Repo = v.GetStringValue()
	}
	if v, ok := p["workspace"]; ok {
		m.Workspace = v.GetStringValue()
	}
	if v, ok := p["worktree_path"]; ok {
		m.Worktree = v.GetStringValue()
	}
	if v, ok := p["base_dir"]; ok {
		m.BaseDir = v.GetStringValue()
	}
	if v, ok := p["source"]; ok {
		m.Source = v.GetStringValue()
	}
	if v, ok := p["category"]; ok {
		m.Category = v.GetStringValue()
	}
	if v, ok := p["actor"]; ok {
		m.Actor = v.GetStringValue()
	}
	if v, ok := p["owner"]; ok {
		m.Owner = v.GetStringValue()
	}
	if v, ok := p["visibility"]; ok {
		m.Visibility = v.GetStringValue()
	}
	if v, ok := p["tags"]; ok {
		if lv := v.GetListValue(); lv != nil {
			for _, item := range lv.GetValues() {
				m.Tags = append(m.Tags, item.GetStringValue())
			}
		}
	}
	if v, ok := p["created_at"]; ok {
		if t, err := time.Parse(time.RFC3339, v.GetStringValue()); err == nil {
			m.CreatedAt = t
		}
	}
	if v, ok := p["kind"]; ok {
		m.Kind = v.GetStringValue()
	}
	if v, ok := p["summary"]; ok {
		m.Summary = v.GetStringValue()
	}
	if v, ok := p["citations"]; ok {
		if lv := v.GetListValue(); lv != nil {
			for _, item := range lv.GetValues() {
				sv := item.GetStructValue()
				if sv == nil {
					continue // skip malformed (non-struct) list items
				}
				f := sv.GetFields()
				m.Citations = append(m.Citations, Citation{
					Kind:    f["kind"].GetStringValue(),
					Ref:     f["ref"].GetStringValue(),
					Locator: f["locator"].GetStringValue(),
					Pin:     f["pin"].GetStringValue(),
					Excerpt: f["excerpt"].GetStringValue(),
				})
			}
		}
	}
	return m
}

// Upsert inserts or replaces a memory (same ID replaces in place).
func (s *Store) Upsert(ctx context.Context, m Memory, vec []float32) error {
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(m.ID),
			Vectors: qdrant.NewVectors(vec...),
			Payload: qdrant.NewValueMap(payload(m)),
		}},
	})
	return err
}

// ownerOrSharedCondition matches records the subject may READ.
//
// Authenticated: owner==sub OR visibility=="shared".
// Anonymous: owner=="" ONLY — shared records require an authenticated subject;
// the anonymous bucket is not a back-door to all shared records.
// nil/unknown (a discarded extraction error): matchNothing — fail closed.
func ownerOrSharedCondition(subj Subject) *qdrant.Condition {
	switch s := subj.(type) {
	case authenticated:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewMatch("owner", s.sub),
			qdrant.NewMatch("visibility", visibilityShared),
		}})
	case anonymous:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatch("owner", ""),
		}})
	default:
		return matchNothing()
	}
}

// matchNothing returns a condition no record can satisfy (owner==x AND owner!=x).
// It backs the fail-closed default arm of read-filter switches when the Subject
// is nil/unknown — a query then returns zero rows rather than over-returning.
func matchNothing() *qdrant.Condition {
	const x = "\x00engram-no-such-owner"
	return qdrant.NewFilterAsCondition(&qdrant.Filter{
		Must:    []*qdrant.Condition{qdrant.NewMatch("owner", x)},
		MustNot: []*qdrant.Condition{qdrant.NewMatch("owner", x)},
	})
}

// ownerScopeFilter restricts to a scope AND the caller's readable set (see
// ownerOrSharedCondition for anonymous vs authenticated semantics).
func (s *Store) ownerScopeFilter(scope string, subj Subject) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOrSharedCondition(subj),
	}}
}

// Search returns the k nearest readable memories to vec within scope.
// Authenticated callers see their own records plus shared records; anonymous
// callers see only the ownerless bucket.
func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64) ([]Memory, error) {
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: s.ownerScopeFilter(scope, subj), Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	return memoriesFromPoints(res), nil
}

// memoriesFromPoints decodes Qdrant scored points into Memory records.
func memoriesFromPoints(res []*qdrant.ScoredPoint) []Memory {
	out := make([]Memory, 0, len(res))
	for _, p := range res {
		out = append(out, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	return out
}

// SearchDiscovery runs a top-k vector search constrained to discovery records.
// Empty scope spans all discovery scopes (the cross_spine case); empty kind
// matches both map and fact. subj restricts results via ownerOrSharedCondition:
// authenticated callers see own + shared records (cross_spine = my+shared);
// anonymous callers see only ownerless records — shared requires an
// authenticated subject. Builds a compound exact-match filter — no prefix
// matching.
func (s *Store) SearchDiscovery(ctx context.Context, scope, kind string, subj Subject, vec []float32, k uint64) ([]Memory, error) {
	must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	if kind != "" {
		must = append(must, qdrant.NewMatch("kind", kind))
	}
	must = append(must, ownerOrSharedCondition(subj))
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: &qdrant.Filter{Must: must}, Limit: qdrant.PtrOf(k),
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	return memoriesFromPoints(res), nil
}

// List returns memories in a scope without a query vector (for session-start
// bootstrap), most-recent first. Scrolls up to scanCap points in the scope and
// sorts by CreatedAt in-process to avoid requiring a Qdrant payload index.
// Read set follows ownerOrSharedCondition: anonymous callers see only the
// ownerless bucket; authenticated callers see own + shared records.
func (s *Store) List(ctx context.Context, scope string, subj Subject, limit uint64) ([]Memory, error) {
	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         s.ownerScopeFilter(scope, subj),
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Memory, 0, len(pts))
	for _, p := range pts {
		out = append(out, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && uint64(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ScopeCount is a scope plus the number of records in it the caller can read.
type ScopeCount struct {
	Scope string
	Count uint64
}

// ListScopes enumerates the caller's readable scopes with per-scope counts.
// Qdrant has no GROUP BY, so it scrolls the readable set (owner OR shared, across
// ALL scopes — ownerOrSharedCondition, not ownerScopeFilter which pins a scope)
// bounded by scanCap and aggregates in-process. The second return is true when
// the scan hit scanCap, meaning the counts are a bounded sample, not exact.
func (s *Store) ListScopes(ctx context.Context, subj Subject) ([]ScopeCount, bool, error) {
	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{ownerOrSharedCondition(subj)}},
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, false, err
	}
	counts := map[string]uint64{}
	for _, p := range pts {
		counts[fromPayload(p.Id.GetUuid(), p.Payload).Scope]++
	}
	out := make([]ScopeCount, 0, len(counts))
	for sc, n := range counts {
		out = append(out, ScopeCount{Scope: sc, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Scope < out[j].Scope })
	return out, len(pts) == scanCap, nil
}

// Get returns the memory with the given id.
func (s *Store) Get(ctx context.Context, id string) (Memory, error) {
	pts, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return Memory{}, err
	}
	if len(pts) == 0 {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return fromPayload(id, pts[0].Payload), nil
}

// GetReadable returns the record only if the caller may READ it; otherwise
// ErrNotFound, so ownership never leaks across actors.
//
// Authenticated callers: readable if owner==sub OR visibility=="shared".
// Anonymous callers: readable only if owner=="" (ownerless bucket).
// The "shared" grant requires an authenticated subject — anonymous callers
// cannot read shared records. nil/unknown Subject → fail closed (ErrNotFound).
func (s *Store) GetReadable(ctx context.Context, id string, subj Subject) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	switch sj := subj.(type) {
	case authenticated:
		if m.Owner != sj.sub && m.Visibility != visibilityShared {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	case anonymous:
		if m.Owner != "" {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	default:
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
}

// getWritable returns the record only if the caller OWNS it (shared does NOT
// grant write); otherwise ErrNotFound. The mutate primitive.
//
// Owner-only: anonymous requires owner=="", authenticated requires owner==sub.
// shared visibility is irrelevant to the write gate — shared grants read, not
// write. Any owner-stamped record (owner!="") is invisible to anonymous mutation,
// preserving fail-closed write isolation even in mixed-auth deployments.
// Per-actor isolation requires authentication (see the package isolation contract
// and README).
func (s *Store) getWritable(ctx context.Context, id string, subj Subject) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	switch sj := subj.(type) {
	case authenticated:
		if m.Owner != sj.sub {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	case anonymous:
		if m.Owner != "" {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	default:
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
}

// OwnedOrAbsent permits a client-supplied-id write: nil if the id is absent (new
// record) or already owned by the subject (replace in place); ErrNotFound if it
// exists and is owned by a different actor or the subject is anonymous but the
// record has an owner (refuse cross-owner overwrite). Transport errors surface
// unchanged.
func (s *Store) OwnedOrAbsent(ctx context.Context, id string, subj Subject) error {
	m, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	switch sj := subj.(type) {
	case authenticated:
		if m.Owner != sj.sub {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return nil
	case anonymous:
		if m.Owner != "" {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
}

// FetchForUpdate returns the record iff it exists and is owned by the subject
// (the same gate as the internal write path); otherwise ErrNotFound. The update
// handler calls this once as the authoritative ownership gate BEFORE embedding,
// then hands the returned record to Update — so the update path performs a
// single Qdrant Get instead of two. The returned record carries current
// visibility, so a content-only Update (shared==nil) preserves it.
//
// Anonymous-bucket semantics preserved: Anonymous() matches owner=="" exactly as
// getWritable does, so ownerless records remain mutually writable when auth is
// disabled.
func (s *Store) FetchForUpdate(ctx context.Context, id string, subj Subject) (Memory, error) {
	return s.getWritable(ctx, id, subj)
}

// Update applies a content change (re-embedded via vec) to a record previously
// fetched and ownership-verified via FetchForUpdate. It does NOT re-fetch: cur
// is authoritative, so the update path gates ownership exactly once. When shared
// is non-nil it also sets visibility (true → "shared", false → ""); nil leaves
// visibility unchanged so a content edit never silently unshares.
func (s *Store) Update(ctx context.Context, cur Memory, content string, shared *bool, vec []float32) error {
	cur.Content = content
	if shared != nil {
		if *shared {
			cur.Visibility = visibilityShared
		} else {
			cur.Visibility = ""
		}
	}
	return s.Upsert(ctx, cur, vec)
}

// SetVisibility flips a record's shared flag without re-embedding (uses
// SetPayload, preserving the vector), only if owned by subj.
//
// TOCTOU note: if the record is deleted between the getWritable ownership gate
// and the SetPayload call, Qdrant's point-ID-selector SetPayload returns a
// NotFound gRPC error (verified against v1.18.2). That error propagates
// unchanged, so SetVisibility is fail-closed with respect to concurrent
// deletion — no additional re-fetch is required.
func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) error {
	if _, err := s.getWritable(ctx, id, subj); err != nil {
		return err
	}
	vis := ""
	if shared {
		vis = visibilityShared
	}
	_, err := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

// Delete removes the memory with the given id, only if owned by subj.
func (s *Store) Delete(ctx context.Context, id string, subj Subject) error {
	if _, err := s.getWritable(ctx, id, subj); err != nil {
		return err
	}
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	})
	return err
}

// DeleteAll removes the subject's OWN records in scope (never another owner's,
// and never another owner's shared records). A nil/unknown Subject is rejected
// without deleting anything — fail closed.
func (s *Store) DeleteAll(ctx context.Context, scope string, subj Subject) error {
	var owner string
	switch sj := subj.(type) {
	case authenticated:
		owner = sj.sub
	case anonymous:
		owner = ""
	default:
		return fmt.Errorf("%w: nil subject", ErrNotFound)
	}
	filter := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		qdrant.NewMatch("owner", owner),
	}}
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(filter),
	})
	return err
}

// ownerlessFilter matches pre-isolation records — those written before the owner
// key existed. NewIsEmpty matches a missing, null, or empty-array "owner" payload
// but NOT an explicit empty string, so auth-disabled records (which carry an
// explicit owner=="") are intentionally excluded.
func ownerlessFilter() *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty("owner")}}
}

// CountOwnerless returns the number of pre-isolation (owner-less) records. These
// are invisible to every owner-scoped read until migrate-set-owner stamps them;
// the server bootstrap uses this to warn the operator. See ownerlessFilter.
func (s *Store) CountOwnerless(ctx context.Context) (uint64, error) {
	return s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: ownerlessFilter(), Exact: qdrant.PtrOf(true),
	})
}

// CountAnonymousBucket returns the number of records in the auth-disabled
// anonymous bucket (an explicit owner==""). Distinct from CountOwnerless, which
// matches pre-isolation records with NO owner key (NewIsEmpty). The server
// bootstrap warns when this is non-empty: those records are readable by any
// anonymous caller, so an operator who once ran auth-disabled should know they
// exist before enabling a network surface.
func (s *Store) CountAnonymousBucket(ctx context.Context) (uint64, error) {
	return s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", "")}},
		Exact:          qdrant.PtrOf(true),
	})
}

// MigrateSetOwner backfills owner onto every pre-isolation record (one that lacks
// an owner key). Idempotent: records that already carry an owner — including the
// auth-disabled owner=="" bucket — are not matched (see ownerlessFilter).
//
// Returns the number of owner-less records counted immediately before the stamp.
// The count and the SetPayload are two separate operations, not one atomic
// transaction, and Qdrant's SetPayload reports no affected-point count to
// reconcile against — so under concurrent writes the returned figure can drift
// from the rows actually stamped (a record added after the count is still
// stamped by the filter but not counted; one deleted in the window is counted
// but not stamped). This is acceptable for the intended use: a one-time, offline
// admin backfill on a single-user deployment with no concurrent writers. Run it
// that way and the count is exact.
func (s *Store) MigrateSetOwner(ctx context.Context, owner string) (uint64, error) {
	if owner == "" {
		return 0, fmt.Errorf("owner must be non-empty")
	}
	missing := ownerlessFilter()
	// Snapshot count taken just before the stamp; see the non-atomicity caveat in
	// the doc comment. Exact:true so the offline single-user count is precise.
	cnt, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: missing, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if cnt == 0 {
		return 0, nil
	}
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"owner": owner}),
		PointsSelector: qdrant.NewPointsSelectorFilter(missing),
	})
	if err != nil {
		return 0, err
	}
	return cnt, nil
}
