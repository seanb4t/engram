// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package server registers and serves the engram memory MCP tools.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qdrant/go-client/qdrant"
	otelgrpc "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/embed"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/telemetry"
)

type deps struct {
	st *store.Store
	em interface {
		Embed(context.Context, string) ([]float32, error)
	}
}

// StoreFromEnv builds a Qdrant-backed Store from the ENGRAM_QDRANT_* / ENGRAM_EMBED_DIM
// environment and ensures the collection exists. Shared by the server bootstrap
// and the migrate-set-owner / prune-expired commands.
func StoreFromEnv() (*store.Store, error) {
	st, embedDim, err := StoreFromEnvNoEnsure()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := st.EnsureCollection(ctx, embedDim); err != nil {
		return nil, fmt.Errorf("EnsureCollection: %w", err)
	}
	return st, nil
}

// StoreFromEnvNoEnsure builds the Store from the ENGRAM_QDRANT_* / ENGRAM_EMBED_DIM
// environment but does NOT create the collection, and returns the configured embed
// dimension. reindex uses this so it can require the source collection to already
// exist rather than silently creating an empty one at the new dimension.
func StoreFromEnvNoEnsure() (*store.Store, uint64, error) {
	cfg, err := config.Load(nil)
	if err != nil {
		return nil, 0, fmt.Errorf("load config: %w", err)
	}
	// Fail fast and uniformly on malformed data-plane config before building any
	// client. This single call site covers every store-building command: serve
	// (via buildDepsFromEnv), reindex, migrate, and prune.
	if err := cfg.Validate(); err != nil {
		return nil, 0, err
	}
	embedDim, err := strconv.ParseUint(cfg.Embed.Dim, 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ENGRAM_EMBED_DIM %q: %w", cfg.Embed.Dim, err)
	}
	host, portStr, err := net.SplitHostPort(cfg.Qdrant.Addr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ENGRAM_QDRANT_ADDR %q: %w", cfg.Qdrant.Addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid port in ENGRAM_QDRANT_ADDR %q: %w", cfg.Qdrant.Addr, err)
	}
	qc, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
		GrpcOptions: []grpc.DialOption{
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		},
	})
	if err != nil {
		return nil, 0, fmt.Errorf("qdrant client: %w", err)
	}
	return store.New(qc, cfg.Qdrant.Collection), embedDim, nil
}

// buildDepsFromEnv wires up the store and embedder from the environment. The
// store/Qdrant vars (ENGRAM_QDRANT_ADDR, ENGRAM_QDRANT_COLLECTION, ENGRAM_EMBED_DIM) are
// read by StoreFromEnv; the embedder vars are read by EmbedderFromEnv.
func buildDepsFromEnv() (*deps, error) {
	st, err := StoreFromEnv()
	if err != nil {
		return nil, err
	}
	warnOwnerlessRecords(st)
	return &deps{st: st, em: EmbedderFromEnv()}, nil
}

// EmbedderFromEnv builds the OpenAI-compatible embedder from the
// ENGRAM_OPENAI_BASE_URL, ENGRAM_OPENAI_API_KEY, and ENGRAM_EMBED_MODEL
// environment. Exported so admin commands (e.g. reindex) re-embed with the same
// configured embedder the server bootstrap uses.
func EmbedderFromEnv() *embed.Client {
	cfg, err := config.Load(nil)
	if err != nil {
		// Defaults always load cleanly; a Load error here means a malformed
		// koanf layer, which is a programming error, not operator input.
		panic(fmt.Sprintf("config load: %v", err))
	}
	return embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Embed.Model,
		embed.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)))
}

// warnOwnerlessRecords loudly warns at startup when pre-isolation (owner-less)
// records exist: they are invisible to every owner-scoped read and cannot be
// cleared by delete_all until claimed via `engram migrate-set-owner --owner <sub>`.
// A count error is itself logged (best-effort; never blocks startup).
func warnOwnerlessRecords(st *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := st.CountOwnerless(ctx)
	if err != nil {
		slog.Warn("could not check for pre-isolation (owner-less) records", "err", err)
	}
	if err == nil && n > 0 {
		slog.Warn("pre-isolation records have no owner — invisible to reads and not removable by delete_all until migrate-set-owner runs",
			"count", n)
	}

	// Anonymous bucket (explicit owner==""): readable by any anonymous caller.
	// Surfaces a deployment that previously ran auth-disabled before any
	// network read surface is exposed.
	an, aErr := st.CountAnonymousBucket(ctx)
	if aErr != nil {
		slog.Warn("could not check the anonymous (owner=='') bucket", "err", aErr)
		return
	}
	if an > 0 {
		slog.Warn("anonymous-bucket records exist (owner==\"\"): readable by any unauthenticated caller; they predate an OIDC-enabled deployment", "count", an)
	}
}

type storeArgs struct {
	Content   string   `json:"content" jsonschema:"the memory text to persist"`
	Scope     string   `json:"scope" jsonschema:"run:tier:repo, e.g. eval-2026-05:project:selfhosted-cluster"`
	Source    string   `json:"source" jsonschema:"user-said or agent-inferred"`
	Category  string   `json:"category" jsonschema:"decision|preference|convention|gotcha"`
	Tags      []string `json:"tags,omitempty"`
	Repo      string   `json:"repo,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	Worktree  string   `json:"worktree_path,omitempty"`
	BaseDir   string   `json:"base_dir,omitempty"`
}

type scheduleArgs struct {
	Content   string   `json:"content" jsonschema:"the memory text to persist"`
	Scope     string   `json:"scope" jsonschema:"run:tier:repo, e.g. eval-2026-05:project:selfhosted-cluster"`
	Source    string   `json:"source" jsonschema:"user-said or agent-inferred"`
	Category  string   `json:"category" jsonschema:"decision|preference|convention|gotcha"`
	Tags      []string `json:"tags,omitempty"`
	Repo      string   `json:"repo,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	Worktree  string   `json:"worktree_path,omitempty"`
	BaseDir   string   `json:"base_dir,omitempty"`
	NotBefore string   `json:"not_before,omitempty" jsonschema:"RFC3339; hide from recall until this time"`
	NotAfter  string   `json:"not_after,omitempty" jsonschema:"RFC3339; drop from recall at this time"`
}

// parseWindow validates and parses the schedule_memory temporal window. At least
// one bound is required; not_after must be in the future and after not_before.
func parseWindow(a scheduleArgs, now time.Time) (nb, na *time.Time, err error) {
	if a.NotBefore == "" && a.NotAfter == "" {
		return nil, nil, fmt.Errorf("schedule_memory requires not_before and/or not_after (use store_memory for unscheduled records)")
	}
	if a.Category == "discovery" {
		return nil, nil, fmt.Errorf("discovery is not schedulable; use store_discovery")
	}
	if a.NotBefore != "" {
		t, perr := time.Parse(time.RFC3339, a.NotBefore)
		if perr != nil {
			return nil, nil, fmt.Errorf("not_before: %w", perr)
		}
		nb = &t
	}
	if a.NotAfter != "" {
		t, perr := time.Parse(time.RFC3339, a.NotAfter)
		if perr != nil {
			return nil, nil, fmt.Errorf("not_after: %w", perr)
		}
		if !t.After(now) {
			return nil, nil, fmt.Errorf("not_after %s is not in the future", a.NotAfter)
		}
		na = &t
	}
	if nb != nil && na != nil && !nb.Before(*na) {
		return nil, nil, fmt.Errorf("not_before must be strictly before not_after")
	}
	return nb, na, nil
}

type searchArgs struct {
	Query string `json:"query"`
	Scope string `json:"scope"`
	K     uint64 `json:"k,omitempty"`
}

type listArgs struct {
	Scope string `json:"scope" jsonschema:"the scope to list memories from"`
	Limit uint64 `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
}

type listScheduledArgs struct {
	Scope string `json:"scope" jsonschema:"the scope to list scheduled/expired memories from"`
	State string `json:"state,omitempty" jsonschema:"scheduled (default, not yet active) | expired | all"`
	Limit uint64 `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
}

type idArgs struct {
	ID string `json:"id"`
}

type updateArgs struct {
	ID      string    `json:"id"`
	Content string    `json:"content"`
	Shared  *bool     `json:"shared,omitempty" jsonschema:"omit to keep current visibility; true=shared, false=private"`
	Tags    *[]string `json:"tags,omitempty" jsonschema:"omit to keep current tags; supply to replace the full set (empty array clears)"`
}

type scopeArgs struct {
	Scope string `json:"scope"`
}

type citationArg struct {
	Kind    string `json:"kind" jsonschema:"file|commit|url|repo"`
	Ref     string `json:"ref" jsonschema:"path, repo URL, or doc URL"`
	Locator string `json:"locator,omitempty" jsonschema:"e.g. 200-240 line range"`
	Pin     string `json:"pin,omitempty" jsonschema:"commit SHA, content-hash, @rev, or fetched-at"`
	Excerpt string `json:"excerpt,omitempty" jsonschema:"cached substance (<= ~50 lines)"`
}

type storeDiscoveryArgs struct {
	Content   string        `json:"content" jsonschema:"the understanding to cache (embedded + searched)"`
	Kind      string        `json:"kind" jsonschema:"map (orientation) or fact (pinned checkable claim)"`
	Citations []citationArg `json:"citations" jsonschema:"at least one source anchor"`
	Scope     string        `json:"scope" jsonschema:"discovery scope, must start with discovery: (e.g. discovery:repo:<repo>)"`
	Tags      []string      `json:"tags,omitempty"`
	Summary   string        `json:"summary,omitempty"`
	ID        string        `json:"id,omitempty" jsonschema:"omit to create; supply to replace in place"`
}

// store_discovery size bounds (resource-exhaustion guards). Generous enough not
// to reject legitimate excerpts (the skill's soft cap is ~50 lines) while
// rejecting abusive payloads outright.
const (
	maxDiscoveryContentBytes = 64 * 1024 // the understanding text (sent to the embedder)
	maxCitationExcerptBytes  = 16 * 1024 // cached substance per citation
	maxDiscoveryCitations    = 50
)

type searchDiscoveryArgs struct {
	Query      string `json:"query"`
	Scope      string `json:"scope,omitempty" jsonschema:"required unless cross_spine"`
	Kind       string `json:"kind,omitempty" jsonschema:"map|fact filter"`
	K          uint64 `json:"k,omitempty"`
	CrossSpine bool   `json:"cross_spine,omitempty" jsonschema:"span all discovery scopes (ignores scope)"`
}

type setVisibilityArgs struct {
	ID     string `json:"id"`
	Shared bool   `json:"shared" jsonschema:"true = readable by any authenticated caller; false = private"`
}

func validateStoreDiscovery(a storeDiscoveryArgs) error {
	if a.Content == "" {
		return fmt.Errorf("content is required")
	}
	if len(a.Content) > maxDiscoveryContentBytes {
		return fmt.Errorf("content too large: %d bytes (max %d)", len(a.Content), maxDiscoveryContentBytes)
	}
	if a.Kind != "map" && a.Kind != "fact" {
		return fmt.Errorf("kind must be \"map\" or \"fact\", got %q", a.Kind)
	}
	if a.Scope == "" {
		return fmt.Errorf("scope is required")
	}
	if !strings.HasPrefix(a.Scope, "discovery:") {
		return fmt.Errorf("scope must be a discovery scope (start with \"discovery:\"), got %q", a.Scope)
	}
	if len(a.Citations) == 0 {
		return fmt.Errorf("at least one citation is required")
	}
	if len(a.Citations) > maxDiscoveryCitations {
		return fmt.Errorf("too many citations: %d (max %d)", len(a.Citations), maxDiscoveryCitations)
	}
	for i, c := range a.Citations {
		if !validCitationKind(c.Kind) {
			return fmt.Errorf("citation %d: kind must be one of file|commit|url|repo, got %q", i, c.Kind)
		}
		if c.Ref == "" {
			return fmt.Errorf("citation %d: ref is required (the source anchor)", i)
		}
		if len(c.Excerpt) > maxCitationExcerptBytes {
			return fmt.Errorf("citation %d: excerpt too large: %d bytes (max %d)", i, len(c.Excerpt), maxCitationExcerptBytes)
		}
	}
	return nil
}

// validCitationKind reports whether k is one of the four citation kinds the
// store_discovery contract accepts. Mirrors the citationArg.Kind jsonschema enum.
func validCitationKind(k string) bool {
	switch k {
	case "file", "commit", "url", "repo":
		return true
	}
	return false
}

func (d *deps) storeMemory(ctx context.Context, a storeArgs) (string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", err
	}
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return "", err
	}
	m := store.Memory{
		ID:        uuid.NewString(),
		Content:   a.Content,
		Scope:     a.Scope,
		Repo:      a.Repo,
		Workspace: a.Workspace,
		Worktree:  a.Worktree,
		BaseDir:   a.BaseDir,
		Source:    a.Source,
		Category:  a.Category,
		Tags:      a.Tags,
		Actor:     actorFromContext(ctx),
		Owner:     subj.Owner(),
		CreatedAt: time.Now().UTC(),
	}
	return m.ID, d.st.Upsert(ctx, m, vec)
}

func (d *deps) scheduleMemory(ctx context.Context, a scheduleArgs) (string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	nb, na, err := parseWindow(a, now)
	if err != nil {
		return "", err
	}
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return "", err
	}
	m := store.Memory{
		ID:        uuid.NewString(),
		Content:   a.Content,
		Scope:     a.Scope,
		Repo:      a.Repo,
		Workspace: a.Workspace,
		Worktree:  a.Worktree,
		BaseDir:   a.BaseDir,
		Source:    a.Source,
		Category:  a.Category,
		Tags:      a.Tags,
		Actor:     actorFromContext(ctx),
		Owner:     subj.Owner(),
		CreatedAt: now,
		NotBefore: nb,
		NotAfter:  na,
	}
	return m.ID, d.st.Upsert(ctx, m, vec)
}

func (d *deps) storeDiscovery(ctx context.Context, a storeDiscoveryArgs) (string, error) {
	if err := validateStoreDiscovery(a); err != nil {
		return "", err
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", err
	}
	if a.ID != "" {
		if err := d.st.OwnedOrAbsent(ctx, a.ID, subj); err != nil {
			return "", err
		}
	}
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return "", err
	}
	cites := make([]store.Citation, len(a.Citations))
	for i, c := range a.Citations {
		cites[i] = store.Citation{Kind: c.Kind, Ref: c.Ref, Locator: c.Locator, Pin: c.Pin, Excerpt: c.Excerpt}
	}
	id := a.ID
	if id == "" {
		id = uuid.NewString()
	}
	m := store.Memory{
		ID:        id,
		Content:   a.Content,
		Scope:     a.Scope,
		Source:    "agent-inferred",
		Category:  "discovery",
		Kind:      a.Kind,
		Citations: cites,
		Summary:   a.Summary,
		Tags:      a.Tags,
		Actor:     actorFromContext(ctx),
		Owner:     subj.Owner(),
		CreatedAt: time.Now().UTC(),
	}
	return m.ID, d.st.Upsert(ctx, m, vec)
}

// actorFromContext returns the verified caller identity injected by the
// RequireBearerToken middleware, or "" when auth is disabled (no token in ctx).
// The value is never client-supplied — it comes from the validated OIDC token.
func actorFromContext(ctx context.Context) string {
	if ti := mcpauth.TokenInfoFromContext(ctx); ti != nil {
		return ti.UserID
	}
	return ""
}

// subjectFromContext delegates to SubjectFromTokenInfo. See that function for the
// fail-closed rationale and nil-token semantics.
func subjectFromContext(ctx context.Context) (store.Subject, error) {
	return SubjectFromTokenInfo(mcpauth.TokenInfoFromContext(ctx))
}

func (d *deps) listMemory(ctx context.Context, a listArgs) ([]store.Memory, error) {
	if a.Limit == 0 {
		a.Limit = 20
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	ms, _, _, err := d.st.List(ctx, a.Scope, subj, store.ListOptions{Limit: a.Limit})
	return ms, err
}

func (d *deps) listScheduled(ctx context.Context, a listScheduledArgs) ([]store.Memory, error) {
	if a.Limit == 0 {
		a.Limit = 20
	}
	var state store.ScheduledState
	switch a.State {
	case "", "scheduled":
		state = store.ScheduledPending
	case "expired":
		state = store.ScheduledExpired
	case "all":
		state = store.ScheduledAll
	default:
		return nil, fmt.Errorf("state must be one of scheduled|expired|all, got %q", a.State)
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return d.st.ListScheduled(ctx, a.Scope, subj, state, store.ListOptions{Limit: a.Limit})
}

func (d *deps) searchMemory(ctx context.Context, a searchArgs) ([]store.Memory, error) {
	if a.K == 0 {
		a.K = 8
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	vec, err := d.em.Embed(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	return d.st.Search(ctx, a.Scope, subj, vec, a.K)
}

// effectiveDiscoveryScope resolves the scope filter for a discovery search:
// "" means span all discovery scopes (cross_spine). cross_spine ignores any
// supplied scope; otherwise a scope is mandatory.
func effectiveDiscoveryScope(a searchDiscoveryArgs) (string, error) {
	if a.CrossSpine {
		return "", nil
	}
	if a.Scope == "" {
		return "", fmt.Errorf("scope is required unless cross_spine is true")
	}
	return a.Scope, nil
}

func (d *deps) searchDiscovery(ctx context.Context, a searchDiscoveryArgs) ([]store.Memory, error) {
	scope, err := effectiveDiscoveryScope(a)
	if err != nil {
		return nil, err
	}
	if a.CrossSpine && a.Scope != "" {
		// Don't echo the caller-supplied scope value into logs (avoids
		// unbounded/sensitive scope strings reaching log aggregation).
		slog.InfoContext(ctx, "search_discovery: cross_spine=true; ignoring supplied scope")
	}
	if a.K == 0 {
		a.K = 8
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	vec, err := d.em.Embed(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	return d.st.SearchDiscovery(ctx, scope, a.Kind, subj, vec, a.K)
}

func (d *deps) updateMemory(ctx context.Context, a updateArgs) error {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return err
	}
	// Ownership gate before embedding: a single authoritative Get. A non-owner
	// (or missing record) gets ErrNotFound here, so we never reach the billable
	// embed call or a write. The fetched record is handed straight to Update, so
	// the update path makes one Qdrant round-trip for ownership, not two.
	cur, err := d.st.FetchForUpdate(ctx, a.ID, subj)
	if err != nil {
		return err
	}
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return err
	}
	return d.st.Update(ctx, cur, a.Content, a.Shared, a.Tags, vec)
}

// Register wires the memory tools onto the MCP server. It accepts a pre-built
// *telemetry.ToolMetrics (constructed once in runServe and reused for both tool
// instrumentation and auth-failure recording) so there is a single instrument
// instance rather than two disjoint ones. Returns an error if dependency
// construction (store/embedder) fails, so the caller can flush telemetry and
// exit cleanly rather than aborting via log.Fatal.
func Register(s *mcp.Server, mux *http.ServeMux, tm *telemetry.ToolMetrics, resolve connectResolver) error {
	d, err := buildDepsFromEnv()
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}
	if err := d.mountConnect(mux, resolve); err != nil {
		return fmt.Errorf("mount connect: %w", err)
	}

	s.AddReceivingMiddleware(instrumentTools(tm.Record))

	mcp.AddTool(s, &mcp.Tool{Name: "store_memory", Description: "Persist a deliberate, well-formed memory. Do NOT store transient state, secrets, or timestamps."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeArgs) (*mcp.CallToolResult, any, error) {
			id, err := d.storeMemory(ctx, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "schedule_memory", Description: "Persist a memory with a validity window (not_before defers recall; not_after expires it). At least one bound (RFC3339) is required; use store_memory for unscheduled records."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scheduleArgs) (*mcp.CallToolResult, any, error) {
			id, err := d.scheduleMemory(ctx, a)
			return textResult(fmt.Sprintf("scheduled %s", id)), map[string]string{"id": id}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_memory", Description: "Semantic search within a scope."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchArgs) (*mcp.CallToolResult, any, error) {
			hits, err := d.searchMemory(ctx, a)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"memories": hits}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_memory", Description: "List recent memories in a scope without a query (session bootstrap). Most-recent first."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listArgs) (*mcp.CallToolResult, any, error) {
			mems, err := d.listMemory(ctx, a)
			return textResult(fmt.Sprintf("%d memories", len(mems))), map[string]any{"memories": mems}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_scheduled", Description: "List your windowed memories the recall gate is hiding: state=scheduled (not yet active, default) | expired | all. Active memories surface via list_memory/search_memory."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listScheduledArgs) (*mcp.CallToolResult, any, error) {
			mems, err := d.listScheduled(ctx, a)
			return textResult(fmt.Sprintf("%d scheduled", len(mems))), map[string]any{"memories": mems}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_memory", Description: "Fetch one memory by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			subj, err := subjectFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			m, err := d.st.GetReadable(ctx, a.ID, subj)
			return textResult(m.Content), m, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_memory", Description: "Replace a memory's content in place (re-embeds). Optionally set `shared` to toggle visibility (true=shared, false=private); omit to keep current visibility. Optionally set `tags` to replace the full tag set (empty array clears); omit to keep current tags."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, any, error) {
			err := d.updateMemory(ctx, a)
			return textResult("updated"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_memory", Description: "Delete one memory by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			subj, err := subjectFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			err = d.st.Delete(ctx, a.ID, subj)
			return textResult("deleted"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_all", Description: "Delete your own memories in a scope (teardown); never another caller's records."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scopeArgs) (*mcp.CallToolResult, any, error) {
			subj, err := subjectFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			err = d.st.DeleteAll(ctx, a.Scope, subj)
			return textResult("scope cleared"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "store_discovery", Description: "Cache agent-earned codebase understanding with citations. kind=map|fact; >=1 citation; scope discovery:repo:<repo>."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			id, err := d.storeDiscovery(ctx, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_discovery", Description: "Semantic search over the discovery pool. scope required unless cross_spine=true; optional kind=map|fact. Results carry citations + created_at (aging signals)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			hits, err := d.searchDiscovery(ctx, a)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"discoveries": hits}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "set_visibility", Description: "Share or unshare a memory you own. shared=true → readable by any authenticated caller (never writable by others); false → private."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a setVisibilityArgs) (*mcp.CallToolResult, any, error) {
			subj, err := subjectFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			err = d.st.SetVisibility(ctx, a.ID, subj, a.Shared)
			return textResult("visibility updated"), nil, err
		})
	return nil
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
