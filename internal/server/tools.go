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

// buildDepsFromEnv reads environment variables and wires up the store and embedder.
// Reads: MEM_QDRANT_ADDR (host:port gRPC 6334), MEM_QDRANT_COLLECTION (default "mem_eval"),
// MEM_LITELLM_URL, MEM_LITELLM_KEY, MEM_EMBED_MODEL (default "ollama/bge-m3"),
// MEM_EMBED_DIM (default 1024).
func buildDepsFromEnv() *deps {
	qdrantAddr := EnvOr("MEM_QDRANT_ADDR", "localhost:6334")
	collection := EnvOr("MEM_QDRANT_COLLECTION", "mem_eval")
	litellmURL := EnvOr("MEM_LITELLM_URL", "http://localhost:4000")
	litellmKey := EnvOr("MEM_LITELLM_KEY", "")
	embedModel := EnvOr("MEM_EMBED_MODEL", "ollama/bge-m3")
	embedDimStr := EnvOr("MEM_EMBED_DIM", "1024")

	embedDim, err := strconv.ParseUint(embedDimStr, 10, 64)
	if err != nil {
		log.Fatalf("invalid MEM_EMBED_DIM %q: %v", embedDimStr, err)
	}

	host, portStr, err := net.SplitHostPort(qdrantAddr)
	if err != nil {
		log.Fatalf("invalid MEM_QDRANT_ADDR %q: %v", qdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("invalid port in MEM_QDRANT_ADDR %q: %v", qdrantAddr, err)
	}

	qc, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		log.Fatalf("qdrant client: %v", err)
	}

	st := store.New(qc, collection)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err = st.EnsureCollection(ctx, embedDim)
	cancel()
	if err != nil {
		log.Fatalf("EnsureCollection: %v", err)
	}

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
}

type scopeArgs struct {
	Scope string `json:"scope"`
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

func (d *deps) searchMemory(ctx context.Context, a searchArgs) ([]store.Memory, error) {
	if a.K == 0 {
		a.K = 8
	}
	vec, err := d.em.Embed(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	return d.st.Search(ctx, a.Scope, vec, a.K)
}

func (d *deps) updateMemory(ctx context.Context, a updateArgs) error {
	cur, err := d.st.Get(ctx, a.ID)
	if err != nil {
		return err
	}
	cur.Content = a.Content
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return err
	}
	return d.st.Upsert(ctx, cur, vec) // same ID → in-place replace + re-embed
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
			mems, err := d.st.List(ctx, a.Scope, a.Limit)
			return textResult(fmt.Sprintf("%d memories", len(mems))), map[string]any{"memories": mems}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_memory", Description: "Fetch one memory by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			m, err := d.st.Get(ctx, a.ID)
			return textResult(m.Content), m, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_memory", Description: "Replace a memory's content in place (re-embeds)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, any, error) {
			err := d.updateMemory(ctx, a)
			return textResult("updated"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_memory", Description: "Delete one memory by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			err := d.st.Delete(ctx, a.ID)
			return textResult("deleted"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_all", Description: "Delete every memory in a scope (teardown)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scopeArgs) (*mcp.CallToolResult, any, error) {
			err := d.st.DeleteAll(ctx, a.Scope)
			return textResult("scope cleared"), nil, err
		})
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
