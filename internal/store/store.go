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

func (s *Store) scopeFilter(scope string) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)}}
}

// ownerOrSharedCondition matches records the caller may READ: owned by sub OR
// marked shared. Sharing grants read, never write.
func ownerOrSharedCondition(sub string) *qdrant.Condition {
	return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
		qdrant.NewMatch("owner", sub),
		qdrant.NewMatch("visibility", "shared"),
	}})
}

// ownerScopeFilter restricts to a scope AND the caller's readable set.
func (s *Store) ownerScopeFilter(scope, sub string) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOrSharedCondition(sub),
	}}
}

// Search returns the k nearest readable memories to vec within scope (records
// the caller owns, plus shared records).
func (s *Store) Search(ctx context.Context, scope, sub string, vec []float32, k uint64) ([]Memory, error) {
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: s.ownerScopeFilter(scope, sub), Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(true),
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
// matches both map and fact. sub restricts results to the caller's own records
// plus any shared records (ownerOrSharedCondition), even when scope is empty —
// this is the cross_spine = my+shared semantic. Builds a compound exact-match
// filter from the same NewMatch primitive scopeFilter uses — no prefix matching.
func (s *Store) SearchDiscovery(ctx context.Context, scope, kind, sub string, vec []float32, k uint64) ([]Memory, error) {
	must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	if kind != "" {
		must = append(must, qdrant.NewMatch("kind", kind))
	}
	must = append(must, ownerOrSharedCondition(sub))
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
func (s *Store) List(ctx context.Context, scope, sub string, limit uint64) ([]Memory, error) {
	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         s.ownerScopeFilter(scope, sub),
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

// GetReadable returns the record only if the caller may READ it (owns it or it
// is shared); otherwise ErrNotFound, so ownership never leaks across actors.
func (s *Store) GetReadable(ctx context.Context, id, sub string) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	if m.Owner != sub && m.Visibility != "shared" {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}

// getWritable returns the record only if the caller OWNS it (shared does NOT
// grant write); otherwise ErrNotFound. The mutate primitive.
func (s *Store) getWritable(ctx context.Context, id, sub string) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	if m.Owner != sub {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}

// Delete removes the memory with the given id, only if owned by sub.
func (s *Store) Delete(ctx context.Context, id, sub string) error {
	if _, err := s.getWritable(ctx, id, sub); err != nil {
		return err
	}
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	})
	return err
}

// DeleteAll removes every memory in scope.
func (s *Store) DeleteAll(ctx context.Context, scope string) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(s.scopeFilter(scope)),
	})
	return err
}
