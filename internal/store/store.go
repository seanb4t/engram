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
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracer is the package-level OTel tracer for store-layer spans. It delegates to
// the global TracerProvider at call time.
var tracer = otel.Tracer("github.com/seanb4t/engram/internal/store")

// ownerOf returns the span-safe opaque owner for subj. A nil Subject (the
// discarded-extraction-error fail-closed case) has no owner, so it reports "" —
// never panicking on the interface method call.
func ownerOf(subj Subject) string {
	if subj == nil {
		return ""
	}
	return subj.Owner()
}

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
	// NotBefore gates deferred reveal: the record is hidden from recall until
	// now >= NotBefore. nil = always active (no lower gate).
	NotBefore *time.Time `json:"not_before,omitempty"`
	// NotAfter gates expiry: the record drops out of recall once now >= NotAfter.
	// nil = never expires.
	NotAfter *time.Time `json:"not_after,omitempty"`
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
	now        func() time.Time
}

// Option configures a Store at construction.
type Option func(*Store)

// WithClock overrides the time source the recall window gate reads. Defaults to
// time.Now. Tests inject a fixed clock to exercise active/scheduled/expired
// boundaries deterministically.
func WithClock(fn func() time.Time) Option {
	return func(s *Store) { s.now = fn }
}

// New returns a Store backed by the given Qdrant client and collection.
func New(c *qdrant.Client, collection string, opts ...Option) *Store {
	s := &Store{client: c, collection: collection, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// EnsureCollection is idempotent: creates the collection at the given vector size if absent.
func (s *Store) EnsureCollection(ctx context.Context, dim uint64) (err error) {
	ctx, span := tracer.Start(ctx, "store.EnsureCollection")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "EnsureCollection", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	return s.ensureCollection(ctx, s.collection, dim)
}

// ensureCollection idempotently creates a named collection at the given vector
// size (distance Cosine) if absent. Factored out of EnsureCollection so reindex
// can provision a *target* collection distinct from s.collection.
func (s *Store) ensureCollection(ctx context.Context, name string, dim uint64) error {
	exists, err := s.client.CollectionExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
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
	if m.NotBefore != nil {
		p["not_before"] = m.NotBefore.Unix()
	}
	if m.NotAfter != nil {
		p["not_after"] = m.NotAfter.Unix()
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
	if v, ok := p["not_before"]; ok {
		t := time.Unix(v.GetIntegerValue(), 0).UTC()
		m.NotBefore = &t
	}
	if v, ok := p["not_after"]; ok {
		t := time.Unix(v.GetIntegerValue(), 0).UTC()
		m.NotAfter = &t
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
func (s *Store) Upsert(ctx context.Context, m Memory, vec []float32) (err error) {
	ctx, span := tracer.Start(ctx, "store.Upsert",
		trace.WithAttributes(attribute.String("engram.scope", m.Scope)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Upsert", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	_, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
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

// ownerOnlyCondition restricts to records the caller OWNS — no shared-read grant.
// It backs management views (ListScheduled) where a `shared` record belonging to
// another actor must stay invisible: a shared+scheduled memory is hidden from
// everyone but its owner until it becomes active (then normal recall surfaces it).
// Fail-closed for nil/unknown Subjects, exactly like ownerOrSharedCondition.
func ownerOnlyCondition(subj Subject) *qdrant.Condition {
	switch s := subj.(type) {
	case authenticated:
		return qdrant.NewMatch("owner", s.sub)
	case anonymous:
		return qdrant.NewMatch("owner", "")
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

// tagMatchConditions returns one exact-match condition per requested tag, with
// AND semantics: appended to a filter's Must, they require a record to carry
// every listed tag. Qdrant matches a scalar value against a list-valued payload
// field by membership, so NewMatch("tags", t) means "the tags list contains t".
// Empty/nil tags yields no conditions — a passthrough, never a contradiction.
// Empty-string elements are skipped: a stored tag is never empty, so matching on
// "" is meaningless and its filter behavior is implementation-defined; dropping
// it makes [""] a passthrough and ["go", ""] equivalent to ["go"].
func tagMatchConditions(tags []string) []*qdrant.Condition {
	conds := make([]*qdrant.Condition, 0, len(tags))
	for _, t := range tags {
		if t == "" {
			continue
		}
		conds = append(conds, qdrant.NewMatch("tags", t))
	}
	return conds
}

// activeWindowConditions gates recall to records whose validity window is open
// at now: (not_before absent OR <= now) AND (not_after absent OR > now). Stored
// window keys are epoch-second integers; the Range bound is *float64 (Qdrant's
// Range field type). Records with no window match via NewIsEmpty — unchanged
// behavior for every pre-feature record. not_after is exclusive (expires AT it).
func activeWindowConditions(now time.Time) []*qdrant.Condition {
	sec := float64(now.Unix())
	// Separate *float64 allocations per bound: proto message field pointers are
	// independently owned, so the two Range structs must not alias one pointer.
	return []*qdrant.Condition{
		qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewRange("not_before", &qdrant.Range{Lte: qdrant.PtrOf(sec)}),
			qdrant.NewIsEmpty("not_before"),
		}}),
		qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewRange("not_after", &qdrant.Range{Gt: qdrant.PtrOf(sec)}),
			qdrant.NewIsEmpty("not_after"),
		}}),
	}
}

// Search returns the k nearest readable memories to vec within scope.
// Authenticated callers see their own records plus shared records; anonymous
// callers see only the ownerless bucket.
func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64, tags []string) (out []Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.Search", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.Int64("engram.k", int64(k)),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Search", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(out)))
		}
	}()

	f := s.ownerScopeFilter(scope, subj)
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
	f.Must = append(f.Must, tagMatchConditions(tags)...)
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: f, Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(true),
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
func (s *Store) SearchDiscovery(ctx context.Context, scope, kind string, subj Subject, vec []float32, k uint64) (out []Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.SearchDiscovery", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.kind", kind),
		attribute.Int64("engram.k", int64(k)),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SearchDiscovery", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(out)))
		}
	}()

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

// ListOptions parameterizes List: page window (Limit/Offset) and the server-side
// category/visibility filters the operator console applies. Zero value = first
// page, no filters.
type ListOptions struct {
	Limit      uint64
	Offset     uint64
	Categories []string // empty = all
	Visibility string   // "" = all | "private" | "shared"
	Tags       []string // empty = all; non-empty = records carrying ALL listed tags. Honored by List (and Search via its tags param); ListScheduled ignores it.
}

// listFilter builds the Qdrant filter for List: scope + per-actor authz (outer
// Must constraint) AND optional category/visibility request filters. The authz
// condition stays the outer Must, so no filter combination can reach another
// actor's records.
//
// Visibility semantics:
//   - "" (empty): no visibility filter — return all readable records.
//   - "shared": match records with stored visibility=="shared".
//   - "private": match records whose stored visibility is "" (the canonical
//     private representation — the store only ever writes "" or "shared"). This
//     is expressed as MustNot(visibility=="shared") so that an empty-string match
//     in Qdrant is reliable across payload-key-absent and empty-value cases.
func listFilter(scope string, subj Subject, opts ListOptions) *qdrant.Filter {
	must := []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOrSharedCondition(subj),
	}
	if len(opts.Categories) > 0 {
		should := make([]*qdrant.Condition, 0, len(opts.Categories))
		for _, c := range opts.Categories {
			should = append(should, qdrant.NewMatch("category", c))
		}
		must = append(must, qdrant.NewFilterAsCondition(&qdrant.Filter{Should: should}))
	}
	must = append(must, tagMatchConditions(opts.Tags)...)
	switch opts.Visibility {
	case visibilityShared:
		must = append(must, qdrant.NewMatch("visibility", visibilityShared))
	case "private":
		// Private records are stored with visibility=="" (empty string). Use
		// MustNot(visibility=="shared") rather than matching empty directly, because
		// Qdrant's NewMatch on an empty string may not reliably match absent or
		// empty-value keys across all payload states.
		return &qdrant.Filter{
			Must:    must,
			MustNot: []*qdrant.Condition{qdrant.NewMatch("visibility", visibilityShared)},
		}
	}
	return &qdrant.Filter{Must: must}
}

// List returns a CreatedAt-desc page of the caller's readable records in scope,
// the pre-page total (matched within scanCap), and an approximate flag (true
// when the match count hit scanCap). When Offset >= total, the page is empty
// (clamped, never a slice panic) and total is still the real matched count.
func (s *Store) List(ctx context.Context, scope string, subj Subject, opts ListOptions) (items []Memory, total uint64, more bool, err error) {
	ctx, span := tracer.Start(ctx, "store.List", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "List", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(items)))
		}
	}()

	const scanCap = 1000
	f := listFilter(scope, subj, opts)
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, 0, false, err
	}
	all := make([]Memory, 0, len(pts))
	for _, p := range pts {
		all = append(all, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	total = uint64(len(all))
	approximate := len(all) == scanCap
	if opts.Offset >= total {
		return []Memory{}, total, approximate, nil
	}
	end := opts.Offset + opts.Limit
	if opts.Limit == 0 || end > total {
		end = total
	}
	return all[opts.Offset:end], total, approximate, nil
}

// ScheduledState selects which hidden-by-the-recall-gate records ListScheduled
// returns. Active (currently-valid) windowed records are never returned here —
// they surface through normal Search/List.
type ScheduledState string

// ScheduledPending, ScheduledExpired, and ScheduledAll filter which
// hidden-by-the-recall-gate records ListScheduled returns.
const (
	ScheduledPending ScheduledState = "scheduled" // now < not_before (not yet active)
	ScheduledExpired ScheduledState = "expired"   // now >= not_after (already lapsed)
	ScheduledAll     ScheduledState = "all"       // union of pending and expired
)

// scheduledStateCondition returns the inverse-window clause for a state. now is
// the comparison instant; its epoch seconds become the *float64 Qdrant Range bound.
func scheduledStateCondition(state ScheduledState, now time.Time) *qdrant.Condition {
	sec := float64(now.Unix())
	// Separate *float64 allocations per bound: proto message field pointers are
	// independently owned and must not alias.
	pending := qdrant.NewRange("not_before", &qdrant.Range{Gt: qdrant.PtrOf(sec)})
	expired := qdrant.NewRange("not_after", &qdrant.Range{Lte: qdrant.PtrOf(sec)})
	switch state {
	case ScheduledExpired:
		return expired
	case ScheduledAll:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{pending, expired}})
	default: // ScheduledPending
		return pending
	}
}

// ListScheduled returns the caller's OWN windowed records that the recall gate is
// hiding, for management (review/reschedule/delete). It applies the INVERSE
// temporal clause and an owner-only authz envelope (ownerOnlyCondition, NOT the
// shared-read grant): a `shared` scheduled/expired record belonging to another
// actor stays invisible here until it becomes active, preserving the deferred-
// reveal guarantee. It does not reuse List (whose gate would exclude exactly
// these records). CreatedAt-desc, bounded by the same scanCap as List. Only
// opts.Limit is honored; opts.Offset is ignored — paginates by Limit alone.
func (s *Store) ListScheduled(ctx context.Context, scope string, subj Subject, state ScheduledState, opts ListOptions) (items []Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.ListScheduled", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
		attribute.String("engram.scheduled_state", string(state)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ListScheduled", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(items)))
		}
	}()

	const scanCap = 1000
	f := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOnlyCondition(subj),
		scheduledStateCondition(state, s.now()),
	}}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: f,
		Limit: qdrant.PtrOf(uint32(scanCap)), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	all := make([]Memory, 0, len(pts))
	for _, p := range pts {
		all = append(all, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	if opts.Limit > 0 && uint64(len(all)) > opts.Limit {
		all = all[:opts.Limit]
	}
	return all, nil
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
func (s *Store) ListScopes(ctx context.Context, subj Subject) (out []ScopeCount, more bool, err error) {
	ctx, span := tracer.Start(ctx, "store.ListScopes",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ListScopes", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(out)))
		}
	}()

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
	out = make([]ScopeCount, 0, len(counts))
	for sc, n := range counts {
		out = append(out, ScopeCount{Scope: sc, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Scope < out[j].Scope })
	return out, len(pts) == scanCap, nil
}

// Get returns the memory with the given id.
func (s *Store) Get(ctx context.Context, id string) (m Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.Get")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Get", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

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
func (s *Store) GetReadable(ctx context.Context, id string, subj Subject) (out Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.GetReadable",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "GetReadable", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

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
func (s *Store) OwnedOrAbsent(ctx context.Context, id string, subj Subject) (err error) {
	ctx, span := tracer.Start(ctx, "store.OwnedOrAbsent",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "OwnedOrAbsent", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

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
func (s *Store) FetchForUpdate(ctx context.Context, id string, subj Subject) (out Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.FetchForUpdate",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "FetchForUpdate", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	return s.getWritable(ctx, id, subj)
}

// Update applies a content change (re-embedded via vec) to a record previously
// fetched and ownership-verified via FetchForUpdate. It does NOT re-fetch: cur
// is authoritative, so the update path gates ownership exactly once. When shared
// is non-nil it also sets visibility (true → "shared", false → ""); nil leaves
// visibility unchanged so a content edit never silently unshares. tags follows
// the same presence-signal contract: non-nil replaces the full set (an empty
// slice clears), nil leaves the existing tags untouched.
func (s *Store) Update(ctx context.Context, cur Memory, content string, shared *bool, tags *[]string, vec []float32) (err error) {
	ctx, span := tracer.Start(ctx, "store.Update")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Update", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	cur.Content = content
	if shared != nil {
		if *shared {
			cur.Visibility = visibilityShared
		} else {
			cur.Visibility = ""
		}
	}
	if tags != nil {
		cur.Tags = *tags
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
func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) (err error) {
	ctx, span := tracer.Start(ctx, "store.SetVisibility",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SetVisibility", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if _, err := s.getWritable(ctx, id, subj); err != nil {
		return err
	}
	vis := ""
	if shared {
		vis = visibilityShared
	}
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

// Delete removes the memory with the given id, only if owned by subj.
func (s *Store) Delete(ctx context.Context, id string, subj Subject) (err error) {
	ctx, span := tracer.Start(ctx, "store.Delete",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Delete", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if _, err := s.getWritable(ctx, id, subj); err != nil {
		return err
	}
	_, err = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	})
	return err
}

// DeleteAll removes the subject's OWN records in scope (never another owner's,
// and never another owner's shared records). A nil/unknown Subject is rejected
// without deleting anything — fail closed.
func (s *Store) DeleteAll(ctx context.Context, scope string, subj Subject) (err error) {
	ctx, span := tracer.Start(ctx, "store.DeleteAll", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "DeleteAll", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

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
	_, err = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(filter),
	})
	return err
}

// PruneExpired deletes every record whose not_after is strictly before the given
// instant — an operator/admin sweep run from the CLI across the WHOLE collection
// (no subject authz; it is not on behalf of a caller). Records without a
// not_after key are never matched. Returns a BEST-EFFORT deleted count: it is the
// Count taken just before the filter-Delete (Qdrant's delete response carries no
// count), so concurrent writes between the two RPCs can make the reported number
// drift from the exact number removed. The delete filter itself is exact; only
// the reported tally is approximate. Treat it as a sweep summary, not an audit.
func (s *Store) PruneExpired(ctx context.Context, before time.Time) (deleted uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.PruneExpired",
		trace.WithAttributes(attribute.Int64("engram.before", before.Unix())))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "PruneExpired", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	f := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewRange("not_after", &qdrant.Range{Lt: qdrant.PtrOf(float64(before.Unix()))}),
	}}
	n, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: f, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	if _, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(f),
	}); err != nil {
		return 0, err
	}
	return n, nil
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
func (s *Store) CountOwnerless(ctx context.Context) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.CountOwnerless")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "CountOwnerless", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

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
func (s *Store) CountAnonymousBucket(ctx context.Context) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.CountAnonymousBucket")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "CountAnonymousBucket", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

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
func (s *Store) MigrateSetOwner(ctx context.Context, owner string) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.MigrateSetOwner")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "MigrateSetOwner", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

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

// EmbedFunc re-embeds a memory's content into a vector. Reindex takes it as a
// callback so the store stays embedder-agnostic (every other write path already
// receives a precomputed vector); the caller supplies the currently-configured
// embedder.
type EmbedFunc func(ctx context.Context, content string) ([]float32, error)

// ReindexOptions parameterizes Reindex. Target and Dim are required; Target must
// differ from the source collection. Batch is the scroll page size (0 → a sane
// default). DryRun scans and counts without creating the target or writing.
type ReindexOptions struct {
	Target string
	Dim    uint64
	Batch  uint32
	DryRun bool
}

// ReindexResult reports what Reindex did: points scanned from the source,
// points re-embedded and upserted into the target (0 on a dry run), and points
// skipped because they carried no content to embed (Scanned == Upserted +
// Skipped on a successful non-dry run).
type ReindexResult struct {
	Scanned  uint64
	Upserted uint64
	Skipped  uint64
}

// reindexBatch is the default scroll page size when ReindexOptions.Batch is 0.
const reindexBatch = 256

// Reindex re-embeds every point in the source collection (s.collection) into a
// new Target collection, enabling a migration to an embedder with a different
// output dimension (Qdrant vector size is immutable, so a new collection is the
// only path). It scrolls the source for (id, payload), re-embeds the payload's
// content with embed, and upserts (same id, new vector, payload preserved
// VERBATIM) into the target. The source is never mutated, so the operator can
// verify the target before cutting ENGRAM_QDRANT_COLLECTION over.
//
// The payload is carried as the raw Qdrant map rather than round-tripped through
// Memory: that preserves keys the Memory model does not know (forward/backward
// schema drift) and, critically, does NOT synthesize an owner key on a
// pre-isolation record — a Memory round-trip would write owner=="" and silently
// drop it into the anonymous bucket.
//
// Fail-closed: an embed error aborts immediately and is returned wrapped; no
// zero/garbage vector is ever written. A point carrying no content is skipped
// (counted in ReindexResult.Skipped) rather than embedded as an empty string.
// The operation is bounded and cancellable via ctx.
//
// No rollback: Reindex is NOT transactional. A scroll, embed, or upsert error
// part-way through leaves the target partially populated (ReindexResult reports
// how many landed). Because upsert is keyed by point id, re-running Reindex with
// the same target is idempotent and safe — it overwrites and completes the set.
func (s *Store) Reindex(ctx context.Context, opts ReindexOptions, embed EmbedFunc) (res ReindexResult, err error) {
	ctx, span := tracer.Start(ctx, "store.Reindex",
		trace.WithAttributes(attribute.String("engram.target", opts.Target)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Reindex", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(
				attribute.Int64("engram.scanned", int64(res.Scanned)),
				attribute.Int64("engram.upserted", int64(res.Upserted)),
				attribute.Int64("engram.skipped", int64(res.Skipped)),
			)
		}
	}()

	if opts.Target == "" {
		return res, fmt.Errorf("reindex: target collection is required")
	}
	if opts.Target == s.collection {
		return res, fmt.Errorf("reindex: target collection %q must differ from source", opts.Target)
	}
	if opts.Dim == 0 {
		return res, fmt.Errorf("reindex: target dimension must be > 0")
	}
	if embed == nil {
		return res, fmt.Errorf("reindex: embed function is required")
	}
	batch := opts.Batch
	if batch == 0 {
		batch = reindexBatch
	}

	// Require the source to already exist. Without this, a typo'd source name
	// (or a not-yet-created collection) would scroll zero points and report a
	// misleading success — especially since the caller's StoreFromEnv may have
	// just created an empty source at the wrong dimension.
	srcExists, err := s.client.CollectionExists(ctx, s.collection)
	if err != nil {
		return res, fmt.Errorf("reindex: check source %q: %w", s.collection, err)
	}
	if !srcExists {
		return res, fmt.Errorf("reindex: source collection %q does not exist", s.collection)
	}

	if !opts.DryRun {
		if err = s.ensureCollection(ctx, opts.Target, opts.Dim); err != nil {
			return res, fmt.Errorf("reindex: ensure target %q: %w", opts.Target, err)
		}
	}

	var offset *qdrant.PointId
	for {
		var pts []*qdrant.RetrievedPoint
		var next *qdrant.PointId
		pts, next, err = s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Limit:          qdrant.PtrOf(batch),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(true),
			WithVectors:    qdrant.NewWithVectors(false),
		})
		if err != nil {
			return res, fmt.Errorf("reindex: scroll source: %w", err)
		}
		for _, p := range pts {
			res.Scanned++
			if opts.DryRun {
				continue
			}
			content := p.Payload["content"].GetStringValue()
			if content == "" {
				// Nothing to embed — skip rather than write a meaningless vector
				// for an empty string. Surfaced via ReindexResult.Skipped.
				res.Skipped++
				continue
			}
			var vec []float32
			vec, err = embed(ctx, content)
			if err != nil {
				return res, fmt.Errorf("reindex: embed point %s: %w", p.Id.GetUuid(), err)
			}
			if _, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
				CollectionName: opts.Target,
				Wait:           qdrant.PtrOf(true),
				Points: []*qdrant.PointStruct{{
					Id:      p.Id,
					Vectors: qdrant.NewVectors(vec...),
					Payload: p.Payload,
				}},
			}); err != nil {
				return res, fmt.Errorf("reindex: upsert point %s into %q: %w", p.Id.GetUuid(), opts.Target, err)
			}
			res.Upserted++
		}
		if next == nil {
			break
		}
		offset = next
	}
	return res, nil
}
