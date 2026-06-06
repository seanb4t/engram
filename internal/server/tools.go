// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package server registers and serves the engram memory MCP tools.
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qdrant/go-client/qdrant"

	"github.com/seanb4t/engram/internal/embed"
	"github.com/seanb4t/engram/internal/store"
)

// EnvOr returns the environment variable k, or def when k is unset/empty.
func EnvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

type deps struct {
	st *store.Store
	em interface {
		Embed(context.Context, string) ([]float32, error)
	}
}

// StoreFromEnv builds a Qdrant-backed Store from the MEM_QDRANT_* / MEM_EMBED_DIM
// environment and ensures the collection exists. Shared by the server bootstrap
// and the migrate-set-owner command.
func StoreFromEnv() (*store.Store, error) {
	qdrantAddr := EnvOr("MEM_QDRANT_ADDR", "localhost:6334")
	collection := EnvOr("MEM_QDRANT_COLLECTION", "mem_eval")
	embedDimStr := EnvOr("MEM_EMBED_DIM", "1024")
	embedDim, err := strconv.ParseUint(embedDimStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid MEM_EMBED_DIM %q: %w", embedDimStr, err)
	}
	host, portStr, err := net.SplitHostPort(qdrantAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid MEM_QDRANT_ADDR %q: %w", qdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port in MEM_QDRANT_ADDR %q: %w", qdrantAddr, err)
	}
	qc, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		return nil, fmt.Errorf("qdrant client: %w", err)
	}
	st := store.New(qc, collection)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := st.EnsureCollection(ctx, embedDim); err != nil {
		return nil, fmt.Errorf("EnsureCollection: %w", err)
	}
	return st, nil
}

// buildDepsFromEnv reads environment variables and wires up the store and embedder.
// Reads: MEM_QDRANT_ADDR (host:port gRPC 6334), MEM_QDRANT_COLLECTION (default "mem_eval"),
// MEM_LITELLM_URL, MEM_LITELLM_KEY, MEM_EMBED_MODEL (default "ollama/bge-m3"),
// MEM_EMBED_DIM (default 1024).
func buildDepsFromEnv() *deps {
	st, err := StoreFromEnv()
	if err != nil {
		log.Fatalf("%v", err)
	}
	litellmURL := EnvOr("MEM_LITELLM_URL", "http://localhost:4000")
	litellmKey := EnvOr("MEM_LITELLM_KEY", "")
	embedModel := EnvOr("MEM_EMBED_MODEL", "ollama/bge-m3")
	em := embed.New(litellmURL, litellmKey, embedModel)
	return &deps{st: st, em: em}
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

type searchArgs struct {
	Query string `json:"query"`
	Scope string `json:"scope"`
	K     uint64 `json:"k,omitempty"`
}

type listArgs struct {
	Scope string `json:"scope" jsonschema:"the scope to list memories from"`
	Limit uint64 `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
}

type idArgs struct {
	ID string `json:"id"`
}

type updateArgs struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Shared  *bool  `json:"shared,omitempty" jsonschema:"omit to keep current visibility; true=shared, false=private"`
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
	Citations []citationArg `json:"citations" jsonschema:">= 1 source anchor"`
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
		Owner:     ownerFromContext(ctx),
		CreatedAt: time.Now().UTC(),
	}
	return m.ID, d.st.Upsert(ctx, m, vec)
}

func (d *deps) storeDiscovery(ctx context.Context, a storeDiscoveryArgs) (string, error) {
	if err := validateStoreDiscovery(a); err != nil {
		return "", err
	}
	sub := ownerFromContext(ctx)
	if a.ID != "" {
		if err := d.st.OwnedOrAbsent(ctx, a.ID, sub); err != nil {
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
		Owner:     sub,
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

// ownerFromContext returns the stable OIDC subject (the authorization key)
// injected by the RequireBearerToken middleware, or "" when auth is disabled.
// Never client-supplied — it is the validated token's `sub`, which
// auth.TokenVerifier places in TokenInfo.Extra["sub"].
func ownerFromContext(ctx context.Context) string {
	if ti := mcpauth.TokenInfoFromContext(ctx); ti != nil {
		if sub, ok := ti.Extra["sub"].(string); ok {
			return sub
		}
	}
	return ""
}

func (d *deps) searchMemory(ctx context.Context, a searchArgs) ([]store.Memory, error) {
	if a.K == 0 {
		a.K = 8
	}
	vec, err := d.em.Embed(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	return d.st.Search(ctx, a.Scope, ownerFromContext(ctx), vec, a.K)
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
		log.Print("search_discovery: cross_spine=true; ignoring supplied scope")
	}
	if a.K == 0 {
		a.K = 8
	}
	vec, err := d.em.Embed(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	return d.st.SearchDiscovery(ctx, scope, a.Kind, ownerFromContext(ctx), vec, a.K)
}

func (d *deps) updateMemory(ctx context.Context, a updateArgs) error {
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return err
	}
	return d.st.Update(ctx, a.ID, ownerFromContext(ctx), a.Content, a.Shared, vec)
}

// Register wires the memory tools onto the MCP server.
func Register(s *mcp.Server) {
	d := buildDepsFromEnv()

	mcp.AddTool(s, &mcp.Tool{Name: "store_memory", Description: "Persist a deliberate, well-formed memory. Do NOT store transient state, secrets, or timestamps."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeArgs) (*mcp.CallToolResult, any, error) {
			id, err := d.storeMemory(ctx, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_memory", Description: "Semantic search within a scope."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchArgs) (*mcp.CallToolResult, any, error) {
			hits, err := d.searchMemory(ctx, a)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"memories": hits}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_memory", Description: "List recent memories in a scope without a query (session bootstrap). Most-recent first."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listArgs) (*mcp.CallToolResult, any, error) {
			if a.Limit == 0 {
				a.Limit = 20
			}
			mems, err := d.st.List(ctx, a.Scope, ownerFromContext(ctx), a.Limit)
			return textResult(fmt.Sprintf("%d memories", len(mems))), map[string]any{"memories": mems}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_memory", Description: "Fetch one memory by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			m, err := d.st.GetReadable(ctx, a.ID, ownerFromContext(ctx))
			return textResult(m.Content), m, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_memory", Description: "Replace a memory's content in place (re-embeds)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, any, error) {
			err := d.updateMemory(ctx, a)
			return textResult("updated"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_memory", Description: "Delete one memory by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			err := d.st.Delete(ctx, a.ID, ownerFromContext(ctx))
			return textResult("deleted"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_all", Description: "Delete your own memories in a scope (teardown); never another caller's records."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scopeArgs) (*mcp.CallToolResult, any, error) {
			err := d.st.DeleteAll(ctx, a.Scope, ownerFromContext(ctx))
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
			err := d.st.SetVisibility(ctx, a.ID, ownerFromContext(ctx), a.Shared)
			return textResult("visibility updated"), nil, err
		})
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
