// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package store persists and queries memories as vectors in a Qdrant collection.
package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/shortid"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// ErrInvalidArgument tags errors caused by a malformed caller request (a bad
// cursor, mutually exclusive paging options) as opposed to a Qdrant/infra
// failure. The Connect layer maps it to CodeInvalidArgument and everything else
// to CodeInternal, so a Count/Scroll outage is no longer mislabeled as a client
// error.
var ErrInvalidArgument = errors.New("invalid argument")

// ErrAmbiguousShortID means a short id matched more than one record — an
// invariant violation (MintShortID enforces global uniqueness), surfaced rather
// than silently resolving to an arbitrary point.
var ErrAmbiguousShortID = errors.New("ambiguous short id")

// visibilityShared is the Visibility sentinel for a record readable by any
// authenticated caller. Sharing grants read, never write. Defined once so a typo
// in an authorization path is a compile error rather than a silent gate bypass.
const visibilityShared = "shared"

// SummarySource records the provenance of a record's summary. It is persisted
// to Qdrant (payload/fromPayload) and crosses the Connect proto and MCP recall
// boundaries as a bare string, so its values are contract-locked — use the
// constants below. The named type is a convention aid, not enforcement: Go's
// untyped string constants still assign to it, so a stray "Client" would
// compile, persist, and silently miss the stale-summary guard's
// == SummarySourceClient check. Prefer the constants.
type SummarySource string

const (
	// SummarySourceNone marks a record with no summary provenance. It is the
	// zero value so an unconfigured Memory reads as none without assignment.
	SummarySourceNone SummarySource = ""
	// SummarySourceClient marks a caller-authored summary (set by store_memory
	// and schedule_memory via toMemory, and re-stamped by update_memory) — the
	// trust signal the stale-summary guard protects.
	SummarySourceClient SummarySource = "client"
	// SummarySourceAuto marks a sweep-generated summary (regenerable,
	// auto-clears on content change).
	SummarySourceAuto SummarySource = "auto"
)

// Memory is the unit of storage. Fields map 1:1 to Qdrant payload keys.
type Memory struct {
	ID string `json:"id"`
	// ShortID is a short, case-insensitive Crockford base32 handle (see
	// internal/shortid) minted alongside ID and usable anywhere an id is
	// accepted. Stable: never rotated once assigned. Empty only for
	// pre-backfill legacy records.
	ShortID   string   `json:"short_id,omitempty"`
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
	// Owner is the caller's configured owner-claim value (default email) — the
	// authorization key. Server-set, never client-supplied.
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
	// Summary is a short recall line shown in place of Content by the recall
	// path. Authored by the caller (SummarySource "client") or filled by the
	// offline summarize-missing sweep ("auto"); "" means none.
	Summary string `json:"summary,omitempty"`
	// SummarySource records summary provenance: client | auto | none. See the
	// named type and prefer its constants.
	SummarySource SummarySource `json:"summary_source,omitempty"`
	// SummaryModel names the model that produced an "auto" summary (diagnostics);
	// empty otherwise.
	SummaryModel string `json:"summary_model,omitempty"`
	// SummaryEgressAt is the k1oe.2 durable audit stamp: when this record's
	// content was egressed to the summarizer model (auto path only). Store-only;
	// not on the Connect wire. Zero if never egressed or the summary was
	// client-authored/cleared.
	SummaryEgressAt time.Time `json:"summary_egress_at"`
	// Score is the Qdrant similarity score of this record for the query that
	// returned it (higher = closer). Set only on Search results; zero on
	// list/get. Lets callers see how close a near-miss ranked (GH#261).
	Score float32 `json:"score,omitempty"`
}

// EmbedText builds the text sent to the embedder for a record. Tags are folded
// into the embedded document so curated keywords contribute to vector recall
// (they are otherwise only a hard AND pre-filter). Documents are embedded via
// this helper at store, update, and reindex time; the query side is embedded
// separately (optionally with an instruction prefix) so re-embedding an existing
// corpus with `engram reindex` applies the same composition uniformly.
func EmbedText(content string, tags []string) string {
	if len(tags) == 0 {
		return content
	}
	return content + "\n\ntags: " + strings.Join(tags, ", ")
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

	// mintCandidate generates a short_id candidate; nil defaults to shortid.New.
	// Overridable in tests to force MintShortID's collision-retry branch.
	mintCandidate func() (string, error)
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
// size (distance Cosine) if absent, then ensures the recall payload indexes on
// every call. Factored out of EnsureCollection so reindex can provision a
// *target* collection distinct from s.collection.
func (s *Store) ensureCollection(ctx context.Context, name string, dim uint64) error {
	exists, err := s.client.CollectionExists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: name,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size: dim, Distance: qdrant.Distance_Cosine,
			}),
		}); err != nil {
			return err
		}
	}
	// Indexes are ensured on every boot (idempotently) so existing collections
	// gain them without a data migration: Qdrant backfills the index over the
	// already-stored RFC3339 created_at strings and keyword payloads.
	return s.ensureIndexes(ctx, name)
}

// ensureIndexes idempotently creates the recall payload indexes. owner is a
// tenant-optimized keyword (authz key), scope a keyword, created_at a datetime
// (enables server-side DatetimeRange + order_by). Re-creating an existing index
// is idempotent in Qdrant; the AlreadyExists check defends against versions/races
// that return it instead of succeeding silently.
func (s *Store) ensureIndexes(ctx context.Context, name string) error {
	type idx struct {
		field  string
		typ    qdrant.FieldType
		params *qdrant.PayloadIndexParams
	}
	idxs := []idx{
		{"owner", qdrant.FieldType_FieldTypeKeyword,
			qdrant.NewPayloadIndexParamsKeyword(&qdrant.KeywordIndexParams{IsTenant: qdrant.PtrOf(true)})},
		{"scope", qdrant.FieldType_FieldTypeKeyword, nil},
		{"created_at", qdrant.FieldType_FieldTypeDatetime, nil},
		{"short_id", qdrant.FieldType_FieldTypeKeyword, nil},
	}
	for _, ix := range idxs {
		req := &qdrant.CreateFieldIndexCollection{
			CollectionName:   name,
			FieldName:        ix.field,
			FieldType:        qdrant.PtrOf(ix.typ),
			FieldIndexParams: ix.params,
			Wait:             qdrant.PtrOf(true),
		}
		if _, err := s.client.CreateFieldIndex(ctx, req); err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == grpccodes.AlreadyExists {
				continue
			}
			return fmt.Errorf("ensure index %q: %w", ix.field, err)
		}
	}
	return nil
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
	p["summary"] = m.Summary
	p["summary_source"] = string(m.SummarySource)
	if m.SummaryModel != "" {
		p["summary_model"] = m.SummaryModel
	}
	if !m.SummaryEgressAt.IsZero() {
		p["summary_egress_at"] = m.SummaryEgressAt.Format(time.RFC3339)
	}
	if m.ShortID != "" {
		p["short_id"] = m.ShortID
	}
	if m.Category == "discovery" {
		p["kind"] = m.Kind
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
	if v, ok := p["short_id"]; ok {
		m.ShortID = v.GetStringValue()
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
	if v, ok := p["summary_source"]; ok {
		m.SummarySource = SummarySource(v.GetStringValue())
	}
	if v, ok := p["summary_model"]; ok {
		m.SummaryModel = v.GetStringValue()
	}
	if v, ok := p["summary_egress_at"]; ok {
		if t, err := time.Parse(time.RFC3339, v.GetStringValue()); err == nil {
			m.SummaryEgressAt = t
		}
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
func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64, tags []string, after, before time.Time) (out []Memory, err error) {
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
	if c := createdRangeCondition(after, before); c != nil {
		f.Must = append(f.Must, c)
	}
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
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		m.Score = p.Score
		out = append(out, m)
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
	// Half-open creation-time window. Zero value = unbounded on that side.
	// CreatedAfter is inclusive (gte); CreatedBefore is exclusive (lt).
	CreatedAfter  time.Time
	CreatedBefore time.Time
	// Cursor carries an opaque keyset token to resume from; "" starts at the
	// first page. Cursor mode is selected by Cursor != "" || CursorMode (a non-empty
	// Cursor alone is enough, even with CursorMode false). Mutually exclusive with
	// Offset > 0.
	Cursor     string
	CursorMode bool // true = boundary-id-set cursor paging (MCP default); false = offset paging (UI)
	// Ascending flips the offset-mode created_at ordering from the default
	// descending (recency-first recall) to ascending (oldest-first). Honored
	// only on the offset/all path used by list_rules; cursor mode is unaffected
	// (rules do not paginate). Zero value preserves the existing desc behavior.
	Ascending bool
}

// createdRangeCondition builds a half-open [after, before) DatetimeRange on the
// created_at datetime index: after→Gte (inclusive), before→Lt (exclusive). A
// zero bound is omitted. Returns nil when both bounds are zero (no filter).
func createdRangeCondition(after, before time.Time) *qdrant.Condition {
	if after.IsZero() && before.IsZero() {
		return nil
	}
	dr := &qdrant.DatetimeRange{}
	if !after.IsZero() {
		dr.Gte = timestamppb.New(after.UTC())
	}
	if !before.IsZero() {
		dr.Lt = timestamppb.New(before.UTC())
	}
	return qdrant.NewDatetimeRange("created_at", dr)
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

// List returns a CreatedAt-ordered page of the caller's readable records in scope
// — descending by default, ascending when ListOptions.Ascending is set (offset/all
// mode only) — the exact matched total (server-side Count), and a nextCursor (empty
// in offset mode; populated only by cursor-mode paging). Ordering and the total are
// computed server-side. When Offset >= total, the page is empty (clamped, never a
// slice panic) and total is still the real matched count.
func (s *Store) List(ctx context.Context, scope string, subj Subject, opts ListOptions) (items []Memory, total uint64, nextCursor string, err error) {
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

	if (opts.Cursor != "" || opts.CursorMode) && opts.Offset > 0 {
		return nil, 0, "", fmt.Errorf("list: cursor mode and offset are mutually exclusive: %w", ErrInvalidArgument)
	}
	if opts.Ascending && (opts.Cursor != "" || opts.CursorMode) {
		return nil, 0, "", fmt.Errorf("list: ascending ordering is honored only in offset/all mode, not cursor mode: %w", ErrInvalidArgument)
	}

	f := listFilter(scope, subj, opts)
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
	if c := createdRangeCondition(opts.CreatedAfter, opts.CreatedBefore); c != nil {
		f.Must = append(f.Must, c)
	}

	// Exact total over the filtered set (replaces the scanCap approximation).
	total, err = s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: f, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return nil, 0, "", err
	}

	if opts.Cursor != "" || (opts.Offset == 0 && opts.Limit > 0 && opts.CursorMode) {
		items, nextCursor, err = s.listByCursor(ctx, f, opts)
		return items, total, nextCursor, err
	}

	// Offset mode: Qdrant has no numeric OFFSET, so scroll offset+limit ordered
	// records and return the trailing limit.
	fetch := opts.Offset + opts.Limit
	if opts.Limit == 0 {
		fetch = total // limit 0 = "all" (preserves prior behavior)
	}
	if fetch == 0 {
		// Reached only when Limit==0 ("all") and the filtered set is empty
		// (total==0): Qdrant's Scroll rejects Limit=0 ("must be 1 or larger")
		// and there is nothing to fetch — short-circuit to an empty page.
		return []Memory{}, total, "", nil
	}
	dir := qdrant.Direction_Desc
	if opts.Ascending {
		dir = qdrant.Direction_Asc
	}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(fetch)),
		OrderBy:        &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(dir)},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, 0, "", err
	}
	all := make([]Memory, 0, len(pts))
	for _, p := range pts {
		all = append(all, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	if opts.Offset >= uint64(len(all)) {
		return []Memory{}, total, "", nil
	}
	return all[opts.Offset:], total, "", nil
}

// maxListLimit caps a single cursor page (and bounds a decoded cursor's seen
// set), so neither a large Limit nor a crafted cursor can drive an unbounded
// Scroll over-fetch.
const maxListLimit = 1000

// listByCursor implements boundary-id-set keyset paging over the already-built
// filter f. opts.Cursor may be empty (first page); a non-empty cursor resumes at
// its created_at boundary, dropping ids already emitted at that exact timestamp.
func (s *Store) listByCursor(ctx context.Context, f *qdrant.Filter, opts ListOptions) ([]Memory, string, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	var startFrom *qdrant.StartFrom
	seen := map[string]bool{}
	var boundary string
	if opts.Cursor != "" {
		c, err := decodeCursor(opts.Cursor)
		if err != nil {
			return nil, "", fmt.Errorf("list cursor: %w: %w", err, ErrInvalidArgument)
		}
		if len(c.Seen) > maxListLimit {
			return nil, "", fmt.Errorf("list cursor: seen set too large: %w", ErrInvalidArgument)
		}
		boundary = c.C
		startFrom = qdrant.NewStartFromDatetime(c.C)
		for _, id := range c.Seen {
			seen[id] = true
		}
	}

	// Over-fetch limit + len(seen) + 1: len(seen) covers the boundary ids dropped
	// at resume, and the +1 guarantees forward progress (a full page yields a
	// usable next cursor rather than silently terminating).
	fetch := limit + uint64(len(seen)) + 1
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(fetch)),
		OrderBy: &qdrant.OrderBy{
			Key:       "created_at",
			Direction: qdrant.PtrOf(qdrant.Direction_Desc),
			StartFrom: startFrom,
		},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, "", err
	}

	out := make([]Memory, 0, limit)
	for _, p := range pts {
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		ts := m.CreatedAt.UTC().Format(time.RFC3339)
		if ts == boundary && seen[m.ID] {
			continue // already emitted at this exact timestamp
		}
		out = append(out, m)
		if uint64(len(out)) == limit {
			break
		}
	}

	if uint64(len(out)) < limit {
		return out, "", nil // exhausted: no next page
	}

	// Build next cursor from the last emitted record: c = its created_at, seen =
	// every emitted id sharing that timestamp (so the next page drops them).
	last := out[len(out)-1]
	nextC := last.CreatedAt.UTC().Format(time.RFC3339)
	nextSeen := make([]string, 0, 4)
	// Carry forward prior seen ids if the boundary did not advance. These are
	// disjoint from the emitted ids appended below: any emitted record at the
	// boundary timestamp passed the seen[m.ID] drop, so it was never in seen.
	if nextC == boundary {
		for id := range seen {
			nextSeen = append(nextSeen, id)
		}
	}
	// Emitted records are distinct point ids, so appending those at nextC adds no
	// duplicate — the next seen set is duplicate-free by construction.
	for _, m := range out {
		if m.CreatedAt.UTC().Format(time.RFC3339) == nextC {
			nextSeen = append(nextSeen, m.ID)
		}
	}
	return out, encodeCursor(listCursor{C: nextC, Seen: nextSeen}), nil
}

// ScheduledState selects which hidden-by-the-recall-gate records ListScheduled
// returns. Active (currently-valid) windowed records are never returned here —
// they surface through normal Search/List.
type ScheduledState string

// The recognized ScheduledState values; each inline comment gives the recall-gate
// predicate it selects.
const (
	ScheduledPending ScheduledState = "scheduled" // now < not_before (not yet active)
	ScheduledExpired ScheduledState = "expired"   // now >= not_after (already lapsed)
	ScheduledAll     ScheduledState = "all"       // union of pending and expired
)

// valid reports whether the state is one of the three recognized values. The
// store guards on this so a caller that bypasses the handler's validation cannot
// silently fall through scheduledStateCondition's default to ScheduledPending.
func (st ScheduledState) valid() bool {
	switch st {
	case ScheduledPending, ScheduledExpired, ScheduledAll:
		return true
	}
	return false
}

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
// these records). Server-side order_by created_at desc bounded directly by
// opts.Limit (default 20); the created_at window (opts.CreatedAfter/Before) is
// applied as a DatetimeRange. opts.Offset is ignored — paginates by Limit alone.
func (s *Store) ListScheduled(ctx context.Context, scope string, subj Subject, state ScheduledState, opts ListOptions) (items []Memory, err error) {
	if !state.valid() {
		return nil, fmt.Errorf("invalid scheduled state %q (want scheduled|expired|all)", state)
	}
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

	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	f := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOnlyCondition(subj),
		scheduledStateCondition(state, s.now()),
	}}
	if c := createdRangeCondition(opts.CreatedAfter, opts.CreatedBefore); c != nil {
		f.Must = append(f.Must, c)
	}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: f,
		Limit:       qdrant.PtrOf(uint32(limit)),
		OrderBy:     &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(qdrant.Direction_Desc)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	items = make([]Memory, 0, len(pts))
	for _, p := range pts {
		items = append(items, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	return items, nil
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

// ResolvePointID maps a caller-supplied identifier — a full UUID (any form
// uuid.Parse accepts) or a short id — to the canonical Qdrant point UUID. It is
// owner-agnostic and applies NO authz: the caller's downstream ownership gate
// (GetReadable / OwnedOrAbsent / getWritable) still governs access. Trims before
// the UUID check because uuid.Parse is length-strict and rejects whitespace.
func (s *Store) ResolvePointID(ctx context.Context, idOrShort string) (id string, err error) {
	ctx, span := tracer.Start(ctx, "store.ResolvePointID")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ResolvePointID", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	t := strings.TrimSpace(idOrShort)
	if t == "" {
		return "", fmt.Errorf("%w: empty id", ErrInvalidArgument)
	}
	if u, perr := uuid.Parse(t); perr == nil {
		return u.String(), nil // canonicalize URN / braced / raw-hex forms to hyphenated
	}
	canonical := shortid.Canonical(t)
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("short_id", canonical)}},
		Limit:          qdrant.PtrOf(uint32(2)),
		WithPayload:    qdrant.NewWithPayload(false),
	})
	if err != nil {
		return "", err
	}
	switch len(pts) {
	case 0:
		return "", fmt.Errorf("%w: %s", ErrNotFound, idOrShort)
	case 1:
		return pts[0].Id.GetUuid(), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrAmbiguousShortID, canonical)
	}
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
// slice clears), nil leaves the existing tags untouched. summary follows the
// same presence-signal contract: non-nil replaces the summary (empty string
// clears), nil leaves the existing summary untouched.
func (s *Store) Update(ctx context.Context, cur Memory, content string, shared *bool, tags *[]string, summary *string, vec []float32) (err error) {
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
	if summary != nil {
		cur.Summary = *summary
		if *summary == "" {
			cur.SummarySource, cur.SummaryModel = SummarySourceNone, ""
		} else {
			cur.SummarySource, cur.SummaryModel = SummarySourceClient, ""
		}
		// A client-authored or cleared summary is no longer auto-egressed
		// provenance, so the egress stamp is invalidated.
		cur.SummaryEgressAt = time.Time{}
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
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(deleted)))
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
	deleted = n
	return deleted, nil
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

// MintShortID returns a short id not currently present on any record, retrying
// on the (astronomically unlikely at 50 bits) global collision. When seen is
// non-nil, ids it returns are recorded there and candidates already in it are
// skipped — a defensive same-run guard for batch callers (backfill), so that
// intra-run uniqueness never depends on the backend's read-after-write
// visibility between a mint and its subsequent write. The global Count is
// authoritative.
func (s *Store) MintShortID(ctx context.Context, seen map[string]struct{}) (id string, err error) {
	ctx, span := tracer.Start(ctx, "store.MintShortID")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "MintShortID", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	gen := s.mintCandidate
	if gen == nil {
		gen = shortid.New
	}
	for {
		cand, genErr := gen()
		if genErr != nil {
			err = genErr
			return "", err
		}
		if seen != nil {
			if _, dup := seen[cand]; dup {
				continue
			}
		}
		n, countErr := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection,
			Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("short_id", cand)}},
			Exact:          qdrant.PtrOf(true),
		})
		if countErr != nil {
			err = countErr
			return "", err
		}
		if n == 0 {
			if seen != nil {
				seen[cand] = struct{}{}
			}
			id = cand
			return id, nil
		}
	}
}

// missingShortIDFilter matches records with no short_id key (pre-backfill legacy
// rows). NewIsEmpty matches missing/null/empty — NOT a non-empty value — so
// already-backfilled records are excluded (idempotent). Mirrors ownerlessFilter.
func missingShortIDFilter() *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty("short_id")}}
}

// BackfillShortIDs assigns a globally-unique short id to every record lacking
// one, writing it with a payload-only SetPayload (no re-embed; vectors and the
// absent-owner-key invariant are preserved). Writing the filtered field while
// paging is safe because ScrollAndOffset's next_page_offset is a point-ID
// forward watermark (a continuation point-id, not an ordinal skip-count):
// stamping already-visited points shrinks the NewIsEmpty("short_id") matched
// set but can never skip a not-yet-visited point. The offset=next loop matches
// SummarizeMissing mechanically. dryRun counts without writing.
func (s *Store) BackfillShortIDs(ctx context.Context, dryRun bool) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.BackfillShortIDs")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "BackfillShortIDs", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	if dryRun {
		n, err = s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection, Filter: missingShortIDFilter(), Exact: qdrant.PtrOf(true),
		})
		return n, err
	}

	seen := map[string]struct{}{}
	var offset *qdrant.PointId
	for {
		pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         missingShortIDFilter(),
			Limit:          qdrant.PtrOf(uint32(reindexBatch)),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(false),
		})
		if serr != nil {
			err = serr
			return n, err
		}
		for _, p := range pts {
			sid, merr := s.MintShortID(ctx, seen)
			if merr != nil {
				err = merr
				return n, err
			}
			if _, perr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
				CollectionName: s.collection, Wait: qdrant.PtrOf(true),
				Payload:        qdrant.NewValueMap(map[string]any{"short_id": sid}),
				PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{p.Id}),
			}); perr != nil {
				err = perr
				return n, err
			}
			n++
		}
		if next == nil {
			return n, nil
		}
		offset = next
	}
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

// OwnerRemapSource selects which records RemapOwner re-stamps. It is a sealed
// sum, like Subject: exactly RemapMissing() (owner-less, pre-isolation records
// via IsEmpty), RemapAnon() (the explicit anonymous bucket, owner==""), or
// RemapFrom(sub) (a specific current owner value, a sub or an email). Missing
// and Anon are distinct because IsEmpty("owner") and Match("owner","") target
// different record sets. Sealing (the unexported marker method) makes "which
// source" a compile-time-exhaustive choice instead of a runtime field-count
// check. Like Subject's enforcement gates, dispatch happens via a single
// exhaustive type switch in RemapOwner rather than per-variant interface
// methods, so an unhandled/nil source is nil-safe by construction — a type
// switch's default arm matches a nil interface value without panicking.
// OwnerRemapSource has no Subject-style accessor method (Subject's Owner())
// because nothing outside RemapOwner's own switch needs a value out of the
// source; ValidateOwnerRemap gets what it needs (remapFrom.from) the same way.
type OwnerRemapSource interface {
	isOwnerRemapSource()
}

type remapMissing struct{}

func (remapMissing) isOwnerRemapSource() {}

type remapAnon struct{}

func (remapAnon) isOwnerRemapSource() {}

type remapFrom struct{ from string }

func (remapFrom) isOwnerRemapSource() {}

// RemapMissing selects owner-less (pre-isolation) records.
func RemapMissing() OwnerRemapSource { return remapMissing{} }

// RemapAnon selects the explicit anonymous bucket (owner=="").
func RemapAnon() OwnerRemapSource { return remapAnon{} }

// RemapFrom selects records currently stamped with the given owner value (a
// sub or an email). It panics on an empty value, consistent with
// Authenticated's empty-owner panic: an empty From must never silently
// collapse into a different source.
func RemapFrom(from string) OwnerRemapSource {
	if from == "" {
		panic("store.RemapFrom: from value must be non-empty")
	}
	return remapFrom{from: from}
}

// ValidateOwnerRemap reports whether src and to are well-formed for a remap:
// src must be non-nil, to must be non-empty, and (for a RemapFrom source)
// remapping a value onto itself is a no-op the caller almost certainly didn't
// intend. It is a free function, not an OwnerRemapSource method, because `to`
// is not a property of the source — shared by RemapOwner and the CLI's
// buildRemapSource so this check lives in one place.
func ValidateOwnerRemap(src OwnerRemapSource, to string) error {
	if src == nil {
		return errors.New("owner remap source is required")
	}
	if to == "" {
		return errors.New("to must be non-empty")
	}
	if f, ok := src.(remapFrom); ok && f.from == to {
		return fmt.Errorf("from and to are identical (%q)", to)
	}
	return nil
}

// RemapOwner re-stamps owner=<selected source> -> owner=to across the WHOLE
// collection (operator sweep; no subject authz, like PruneExpired). dryRun
// returns the matched count without writing. Validation runs before any Qdrant
// call. Non-transactional: the reported count is a best-effort snapshot taken
// just before the filtered SetPayload (which is itself exact). Idempotent.
func (s *Store) RemapOwner(ctx context.Context, src OwnerRemapSource, to string, dryRun bool) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.RemapOwner")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "RemapOwner", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	if err = ValidateOwnerRemap(src, to); err != nil {
		return 0, err
	}
	var filter *qdrant.Filter
	switch sj := src.(type) {
	case remapMissing:
		filter = ownerlessFilter()
	case remapAnon:
		filter = &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", "")}}
	case remapFrom:
		filter = &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", sj.from)}}
	default:
		// ValidateOwnerRemap already rejected nil above, so reaching here
		// means a non-nil src of a sealed variant this switch doesn't
		// recognize — a future OwnerRemapSource variant added without a
		// corresponding case, not a caller-input problem.
		return 0, fmt.Errorf("owner remap: unrecognized source type %T", sj)
	}

	cnt, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: filter, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if cnt == 0 || dryRun {
		return cnt, nil
	}
	if _, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"owner": to}),
		PointsSelector: qdrant.NewPointsSelectorFilter(filter),
	}); err != nil {
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
	// Source overrides the collection Reindex reads from. Empty (the default)
	// means the store's configured collection (s.collection, i.e.
	// ENGRAM_QDRANT_COLLECTION) — so operators get the env default for free and
	// only set Source to reindex an arbitrary collection without mutating env.
	Source string
	Dim    uint64
	Batch  uint32
	DryRun bool
	// Progress, when non-nil, is called once after each scanned batch with the
	// running ReindexResult totals, so a long reindex can surface incremental
	// feedback. It is a callback (not store-side I/O) to keep the store
	// embedder- and output-agnostic — the caller owns rendering, matching how
	// EmbedFunc keeps embedding out of the store.
	Progress func(ReindexResult)
	// Resume makes an interrupted reindex cheap to restart: before embedding a
	// source point, the target is checked for that id, and a point already
	// present with IDENTICAL content is skipped (counted Unchanged) instead of
	// re-embedded. The skip predicate is content equality — equal content yields
	// an equal embedding under a fixed embedder, which is the whole point of
	// resuming — so no separate hash is persisted and the target payload stays
	// verbatim. Ignored on a dry run (nothing is written). When false, every
	// point is re-embedded (the prior idempotent-overwrite behavior).
	Resume bool
}

// Validate checks the options that depend only on the options themselves,
// including the target-differs-from-source guard for an EXPLICIT Source (both
// fields are then in opts). The default-source case (Source=="" → s.collection)
// and the non-nil-embed precondition are checked by Reindex, which alone has the
// store's collection and the embed callback.
func (o ReindexOptions) Validate() error {
	if o.Target == "" {
		return errors.New("reindex: target collection is required")
	}
	if o.Dim == 0 {
		return errors.New("reindex: target dimension must be > 0")
	}
	if o.Source != "" && o.Target == o.Source {
		return errors.New("reindex: target collection must differ from source")
	}
	return nil
}

// ReindexResult reports what Reindex did: points scanned from the source,
// points re-embedded and upserted into the target (0 on a dry run), points
// skipped because they carried no content to embed, and (resume only) points
// left unchanged because the target already held them with identical content.
// On a successful non-dry run, Scanned == Upserted + Skipped + Unchanged.
type ReindexResult struct {
	Scanned   uint64
	Upserted  uint64
	Skipped   uint64
	Unchanged uint64
}

// reindexBatch is the default scroll page size when ReindexOptions.Batch is 0.
const reindexBatch = 256

// Reindex re-embeds every point in the source collection (opts.Source, or
// s.collection when unset) into a
// new Target collection, enabling a migration to an embedder with a different
// output dimension (Qdrant vector size is immutable, so a new collection is the
// only path). It scrolls the source for (id, payload), re-embeds the payload's
// content with embed, and upserts (same id, new vector, payload preserved
// VERBATIM) into the target. The source is never mutated, so the operator can
// verify the target before cutting ENGRAM_QDRANT_COLLECTION over.
//
// The payload is carried as the raw Qdrant map rather than round-tripped through
// Memory: that preserves keys the Memory model does not know (forward/backward
// schema drift) and, critically, keeps a pre-isolation record's owner key ABSENT.
// An absent owner key and an explicit owner=="" are distinct states: an absent
// key marks a needs-backfill record invisible to every read until
// `engram migrate-set-owner` sets it, whereas owner=="" is the anonymous bucket.
// A Memory round-trip would synthesize an explicit owner=="" and silently
// relocate the record into that bucket.
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
// opts.Resume makes that re-run cheap: points already present in the target with
// identical content are skipped (counted Unchanged) instead of re-embedded.
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
				attribute.Int64("engram.unchanged", int64(res.Unchanged)),
			)
		}
	}()

	if err = opts.Validate(); err != nil {
		return res, err
	}
	// The read source is the override when set, else the store's configured
	// collection (engram-orve). All source reads and the target-differs guard
	// below key off this effective source, not s.collection directly.
	source := opts.Source
	if source == "" {
		source = s.collection
	}
	// Record the effective source (may differ from the store's collection under
	// --source) so a reindex trace names both endpoints.
	span.SetAttributes(attribute.String("engram.source", source))
	if opts.Target == source {
		return res, fmt.Errorf("reindex: target collection %q must differ from source", opts.Target)
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
	srcExists, err := s.client.CollectionExists(ctx, source)
	if err != nil {
		return res, fmt.Errorf("reindex: check source %q: %w", source, err)
	}
	if !srcExists {
		return res, fmt.Errorf("reindex: source collection %q does not exist", source)
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
			CollectionName: source,
			Limit:          qdrant.PtrOf(batch),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(true),
			WithVectors:    qdrant.NewWithVectors(false),
		})
		if err != nil {
			return res, fmt.Errorf("reindex: scroll source: %w", err)
		}
		// A dry run only needs the count, so tally the whole page and skip the
		// per-point embed/upsert path entirely.
		if opts.DryRun {
			res.Scanned += uint64(len(pts))
		} else {
			// Resume: fetch this batch's ids from the target once so a point already
			// embedded with identical content can be skipped (engram-irhg). One Get
			// per page keeps the lookup O(pages), not O(points).
			var targetContent map[string]string
			if opts.Resume {
				targetContent, err = s.reindexTargetContents(ctx, opts.Target, pts)
				if err != nil {
					return res, fmt.Errorf("reindex: resume lookup in %q: %w", opts.Target, err)
				}
			}
			for _, p := range pts {
				res.Scanned++
				m := fromPayload(p.Id.GetUuid(), p.Payload)
				content := m.Content
				if content == "" {
					// Nothing to embed — skip rather than write a meaningless vector
					// for an empty string. Surfaced via ReindexResult.Skipped.
					res.Skipped++
					continue
				}
				if tc, ok := targetContent[p.Id.GetUuid()]; ok && tc == content {
					// Target already holds this id with identical content — equal
					// content (and, from the same source payload, equal tags) re-embeds
					// to an equal vector, so skip the embed+upsert.
					res.Unchanged++
					continue
				}
				var vec []float32
				// Embed content + tags (EmbedText) so a re-embed folds curated tags
				// into the vector exactly as the store path does.
				vec, err = embed(ctx, EmbedText(m.Content, m.Tags))
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
		}
		// Surface running totals after each scanned page (engram-xddn).
		if opts.Progress != nil {
			opts.Progress(res)
		}
		if next == nil {
			break
		}
		offset = next
	}
	return res, nil
}

// reindexTargetContents fetches the content payload of the given source points'
// ids from the target collection, returning id→content only for ids that already
// exist there. It backs Reindex's resume skip: a fresh or partially-populated
// target simply yields fewer (or no) entries, so a first run skips nothing.
func (s *Store) reindexTargetContents(ctx context.Context, target string, pts []*qdrant.RetrievedPoint) (map[string]string, error) {
	if len(pts) == 0 {
		return nil, nil
	}
	ids := make([]*qdrant.PointId, 0, len(pts))
	for _, p := range pts {
		ids = append(ids, p.Id)
	}
	got, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: target,
		Ids:            ids,
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(got))
	for _, p := range got {
		out[p.Id.GetUuid()] = p.Payload["content"].GetStringValue()
	}
	return out, nil
}
