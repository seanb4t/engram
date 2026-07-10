// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package server registers and serves the engram memory MCP tools.
package server

import (
	"context"
	"errors"
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
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/embed"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/summarize"
	"github.com/seanb4t/engram/internal/telemetry"
)

type deps struct {
	st *store.Store
	em interface {
		// Embed embeds a document (raw). EmbedQuery embeds a search query,
		// optionally with an instruction prefix (asymmetric embedding).
		Embed(context.Context, string) ([]float32, error)
		EmbedQuery(context.Context, string) ([]float32, error)
	}
	// now is the handler time source for schedule_memory window validation. Nil
	// means wall-clock time.Now (see clock); tests inject a fixed clock to pin
	// "now" deterministically, mirroring store.WithClock.
	now func() time.Time
	// summaryMaxChars is the recall truncation cap (ENGRAM_SUMMARY_MAX_CHARS).
	summaryMaxChars int
	// summaryQueue is the async-on-write summary worker pool. nil (disabled)
	// unless ENGRAM_SUMMARY_MODEL is set AND ENGRAM_SUMMARY_ON_WRITE parses
	// true (D-01 AND-gate, decided in buildDepsFromEnv). Its methods are
	// nil-safe no-ops, so call sites never branch on whether it's enabled.
	summaryQueue *summaryQueue
	// usageQueue is the async get-path access-count incrementer (Phase 12,
	// D-01/D-10). nil (disabled) unless ENGRAM_USAGE_SIGNALS parses true
	// (default true; decided in buildUsageQueue). The D-06 OTLP recall-span
	// analytics are independent of this flag. Its methods are nil-safe
	// no-ops, so getMemory/Connect GetMemory never branch on whether it's
	// enabled.
	usageQueue *usageQueue
}

// configLoad is the indirection seam for loading koanf config from the process
// environment. buildDepsFromEnv must perform exactly one load per startup; tests
// override this to count loads.
var configLoad = config.Load

// loadAndValidate loads the data-plane config and fails fast on malformed values
// before any client is built. It is the one place load + Validate live, so every
// store-building path funnels through it: serve (buildDepsFromEnv), reindex
// (StoreAndEmbedderFromEnvNoEnsure), and migrate / prune (StoreFromEnv).
func loadAndValidate() (*config.Config, error) {
	cfg, err := configLoad(nil)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// storeFromConfig builds the Qdrant-backed Store (without ensuring the collection)
// from an already-loaded config and returns the configured embed dimension.
func storeFromConfig(cfg *config.Config) (*store.Store, uint64, error) {
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

// ensureStoreFromConfig builds the Store from an already-loaded config and ensures
// its collection exists at the configured embed dimension.
func ensureStoreFromConfig(cfg *config.Config) (*store.Store, error) {
	st, embedDim, err := storeFromConfig(cfg)
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

// StoreFromEnv builds a Qdrant-backed Store from the ENGRAM_QDRANT_* / ENGRAM_EMBED_DIM
// environment and ensures the collection exists. Used by the migrate-remap-owner /
// prune-expired commands; the server bootstrap builds its store through
// buildDepsFromEnv (sharing ensureStoreFromConfig) so config is loaded only once.
func StoreFromEnv() (*store.Store, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, err
	}
	return ensureStoreFromConfig(cfg)
}

// StoreAndEmbedderFromEnvNoEnsure builds the no-ensure source Store (with its
// configured embed dimension) and the matching embedder from a SINGLE config
// load. reindex uses it so the source collection must already exist (no-ensure)
// rather than being silently created at the new dimension, and so config is
// parsed exactly once — the engram-635 single-load invariant, applied to the
// reindex path the same way buildDepsFromEnv applies it to serve.
func StoreAndEmbedderFromEnvNoEnsure() (*store.Store, uint64, *embed.Client, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, 0, nil, err
	}
	st, dim, err := storeFromConfig(cfg)
	if err != nil {
		return nil, 0, nil, err
	}
	em, err := embedderFromConfig(cfg)
	if err != nil {
		return nil, 0, nil, err
	}
	return st, dim, em, nil
}

// buildDepsFromEnv wires up the store and embedder from the environment with a
// single config load: it loads + validates once, then builds both the
// collection-ensured store and the embedder from that one *config.Config. sqm
// is the pre-built static SummaryQueueMetrics (from serve.go, threaded through
// Register); it is used only when the D-01 AND-gate enables the async summary
// worker (buildSummaryQueue), and may be nil in tests exercising the disabled
// path. uqm is the pre-built static UsageQueueMetrics (also from serve.go),
// used only when ENGRAM_USAGE_SIGNALS enables the async usage-signal worker
// (buildUsageQueue); may also be nil in tests exercising the disabled path.
func buildDepsFromEnv(sqm *telemetry.SummaryQueueMetrics, uqm *telemetry.UsageQueueMetrics) (*deps, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, err
	}
	st, err := ensureStoreFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	warnOwnerlessRecords(st)
	em, err := embedderFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &deps{
		st:              st,
		em:              em,
		summaryMaxChars: summaryMaxChars(cfg),
		summaryQueue:    buildSummaryQueue(cfg, st, sqm),
		usageQueue:      buildUsageQueue(cfg, st, uqm),
	}, nil
}

// buildSummaryQueue constructs and starts the async-on-write summary worker
// pool when the D-01 two-switch AND-gate is satisfied: ENGRAM_SUMMARY_MODEL is
// non-empty AND ENGRAM_SUMMARY_ON_WRITE parses true. This is where the AND-gate
// actually lives — Config.Validate() checks the three fields unconditionally
// but does not decide runtime enablement. Either switch off returns nil
// (disabled; every summaryQueue method becomes a nil-safe no-op, so call
// sites never branch). Reuses summarizerFromConfig so the async worker shares
// the identical summarizer construction as the summarize-missing sweep — no
// parallel summarizer. Registers the D-09 queue-depth gauge immediately after
// the queue is constructed and started: this is the only place q.depth() has
// a live queue to close over (Codex finding #1; serve.go only builds the
// static SummaryQueueMetrics instruments and never touches the queue).
func buildSummaryQueue(cfg *config.Config, st *store.Store, sqm *telemetry.SummaryQueueMetrics) *summaryQueue {
	if cfg.Summarize.Model == "" {
		return nil
	}
	onWrite, err := strconv.ParseBool(cfg.Summarize.OnWrite)
	if err != nil || !onWrite {
		return nil
	}
	workers, err := strconv.Atoi(cfg.Summarize.Workers)
	if err != nil || workers <= 0 {
		if cfg.Summarize.Workers != "" {
			slog.Warn("ENGRAM_SUMMARY_WORKERS is set but unparseable or non-positive; using default 2",
				"value", cfg.Summarize.Workers)
		}
		workers = 2
	}
	queueSize, err := strconv.Atoi(cfg.Summarize.QueueSize)
	if err != nil || queueSize <= 0 {
		if cfg.Summarize.QueueSize != "" {
			slog.Warn("ENGRAM_SUMMARY_QUEUE_SIZE is set but unparseable or non-positive; using default 256",
				"value", cfg.Summarize.QueueSize)
		}
		queueSize = 256
	}
	summarizer := summarizerFromConfig(cfg)
	fill := storeFill(st, summarizer.Summarize, cfg.Summarize.Model, summaryMaxChars(cfg))
	q := newSummaryQueue(workers, queueSize, summaryTimeout(cfg), sqm, fill)
	q.Start(context.Background())
	telemetry.RegisterSummaryQueueDepth(otel.Meter("github.com/seanb4t/engram"), q.depth)
	slog.Info("async-on-write summaries enabled", "workers", workers, "queue_size", queueSize)
	return q
}

// buildUsageQueue constructs and starts the async get-path access-count
// incrementer (Phase 12, D-01/D-04/D-10) when ENGRAM_USAGE_SIGNALS parses
// true (default "true" — on by default, D-09). False/unparseable returns nil
// (disabled; every usageQueue method becomes a nil-safe no-op, so the
// getMemory/Connect GetMemory call sites never branch). The D-06 OTLP
// recall-span analytics are independent of this flag. Unlike
// buildSummaryQueue there are no dedicated env-configurable worker/queue-size
// knobs this phase — a small fixed pool is sufficient for a lightweight,
// single-attempt, drop-on-full best-effort counter bump.
func buildUsageQueue(cfg *config.Config, st *store.Store, uqm *telemetry.UsageQueueMetrics) *usageQueue {
	signals, err := strconv.ParseBool(cfg.Usage.Signals)
	if err != nil || !signals {
		return nil
	}
	const workers = 2
	const queueSize = 256
	fill := func(ctx context.Context, id string) error {
		return st.IncrementAccess(ctx, id)
	}
	q := newUsageQueue(workers, queueSize, uqm, fill)
	q.Start(context.Background())
	slog.Info("usage signals enabled", "workers", workers, "queue_size", queueSize)
	return q
}

// summaryMaxChars parses the recall cap, defaulting to 280 on empty/invalid.
func summaryMaxChars(cfg *config.Config) int {
	n, err := strconv.Atoi(cfg.Summarize.MaxChars)
	if err != nil || n <= 0 {
		if cfg.Summarize.MaxChars != "" {
			slog.Warn("ENGRAM_SUMMARY_MAX_CHARS is set but unparseable or non-positive; using default 280",
				"value", cfg.Summarize.MaxChars)
		}
		return 280
	}
	return n
}

// summaryMaxTokens parses the generation ceiling, defaulting to 1024 on
// empty/invalid. 0 is honored (omits the cap; gateway default); negatives fall
// back to the default.
func summaryMaxTokens(cfg *config.Config) int {
	n, err := strconv.Atoi(cfg.Summarize.MaxTokens)
	if err != nil || n < 0 {
		if cfg.Summarize.MaxTokens != "" {
			slog.Warn("ENGRAM_SUMMARY_MAX_TOKENS is set but unparseable or negative; using default 1024",
				"value", cfg.Summarize.MaxTokens)
		}
		return 1024
	}
	return n
}

// summaryTimeout parses the per-request HTTP timeout, defaulting to 30s on
// empty/invalid. 0 is honored (disables the timeout); negatives fall back to
// the default.
func summaryTimeout(cfg *config.Config) time.Duration {
	d, err := time.ParseDuration(cfg.Summarize.Timeout)
	if err != nil || d < 0 {
		if cfg.Summarize.Timeout != "" {
			slog.Warn("ENGRAM_SUMMARY_TIMEOUT is set but unparseable or negative; using default 30s",
				"value", cfg.Summarize.Timeout)
		}
		return 30 * time.Second
	}
	return d
}

// embedderFromConfig builds the OpenAI-compatible embedder from an already-loaded
// config.
func embedderFromConfig(cfg *config.Config) (*embed.Client, error) {
	// ParseEmbedParams errors are surfaced here (not discarded) so a malformed
	// ENGRAM_EMBED_QUERY_PARAMS / ENGRAM_EMBED_DOCUMENT_PARAMS value fails
	// startup instead of silently disabling the params.
	queryParams, err := config.ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", cfg.Embed.QueryParams)
	if err != nil {
		return nil, err
	}
	documentParams, err := config.ParseEmbedParams("ENGRAM_EMBED_DOCUMENT_PARAMS", cfg.Embed.DocumentParams)
	if err != nil {
		return nil, err
	}
	return embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Embed.Model,
		embed.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
		embed.WithQueryInstruction(cfg.Embed.QueryInstruction),
		embed.WithDocumentInstruction(cfg.Embed.DocumentInstruction),
		embed.WithQueryParams(queryParams),
		embed.WithDocumentParams(documentParams)), nil
}

// summarizerFromConfig builds the chat-completions summarizer from config.
func summarizerFromConfig(cfg *config.Config) *summarize.Client {
	return summarize.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Summarize.Model, summaryMaxChars(cfg),
		summarize.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
		summarize.WithMaxTokens(summaryMaxTokens(cfg)),
		summarize.WithTimeout(summaryTimeout(cfg)))
}

// StoreAndSummarizerFromEnv builds the store + summarizer + resolved model name
// + cap for the summarize-missing command. Errors when ENGRAM_SUMMARY_MODEL is
// unset (auto-summary disabled). The model is returned (not re-read from the env
// by the caller) so the value stamped on records as summary_model is exactly the
// one the summarizer uses, regardless of which config layer supplied it.
func StoreAndSummarizerFromEnv() (*store.Store, *summarize.Client, string, int, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, nil, "", 0, err
	}
	if cfg.Summarize.Model == "" {
		return nil, nil, "", 0, fmt.Errorf("ENGRAM_SUMMARY_MODEL is empty: auto-summary is disabled")
	}
	st, err := ensureStoreFromConfig(cfg)
	if err != nil {
		return nil, nil, "", 0, err
	}
	return st, summarizerFromConfig(cfg), cfg.Summarize.Model, summaryMaxChars(cfg), nil
}

// warnOwnerlessRecords loudly warns at startup when pre-isolation (owner-less)
// records exist: they are invisible to every owner-scoped read and cannot be
// cleared by delete_all until claimed via `engram migrate-remap-owner --from-missing --to <owner>`.
// A count error is itself logged (best-effort; never blocks startup).
func warnOwnerlessRecords(st *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := st.CountOwnerless(ctx)
	if err != nil {
		slog.Warn("could not check for pre-isolation (owner-less) records", "err", err)
	}
	if err == nil && n > 0 {
		slog.Warn("pre-isolation records have no owner — invisible to reads and not removable by delete_all until you run: engram migrate-remap-owner --from-missing --to <owner>",
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
	Summary   string   `json:"summary,omitempty" jsonschema:"optional one-line recall summary shown in place of content; preserve negations/identifiers; omit to leave empty (operator backfill or truncation fills recall)"`
}

// scheduleArgs embeds storeArgs and adds the temporal window. The anonymous
// embed flattens identically on both the json-decode and reflected-schema paths
// (Go field promotion), so the schedule_memory wire contract is byte-for-byte the
// store_memory fields plus not_before/not_after.
type scheduleArgs struct {
	storeArgs
	NotBefore string `json:"not_before,omitempty" jsonschema:"RFC3339; hide from recall until this time"`
	NotAfter  string `json:"not_after,omitempty" jsonschema:"RFC3339; drop from recall at this time"`
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
	Query         string   `json:"query"`
	Scope         string   `json:"scope"`
	K             uint64   `json:"k,omitempty"`
	Tags          []string `json:"tags,omitempty" jsonschema:"optional; restrict to records carrying ALL listed tags"`
	Full          bool     `json:"full,omitempty" jsonschema:"return full content instead of summaries (default false → compact summary view)"`
	CreatedAfter  string   `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string   `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
}

type listArgs struct {
	Scope         string   `json:"scope" jsonschema:"the scope to list memories from"`
	Limit         uint64   `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
	Tags          []string `json:"tags,omitempty" jsonschema:"optional; restrict to records carrying ALL listed tags"`
	Full          bool     `json:"full,omitempty" jsonschema:"return full content instead of summaries (default false → compact summary view)"`
	CreatedAfter  string   `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string   `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
	Cursor        string   `json:"cursor,omitempty" jsonschema:"opaque pagination cursor from a prior next_cursor; omit for the first page"`
}

type listScheduledArgs struct {
	Scope         string `json:"scope" jsonschema:"the scope to list scheduled/expired memories from"`
	State         string `json:"state,omitempty" jsonschema:"scheduled (default, not yet active) | expired | all"`
	Limit         uint64 `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
	CreatedAfter  string `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
}

type idArgs struct {
	ID string `json:"id" jsonschema:"the memory's full UUID or its short_id"`
}

type updateArgs struct {
	ID      string    `json:"id" jsonschema:"the memory's full UUID or its short_id"`
	Content string    `json:"content"`
	Shared  *bool     `json:"shared,omitempty" jsonschema:"omit to keep current visibility; true=shared, false=private"`
	Tags    *[]string `json:"tags,omitempty" jsonschema:"omit to keep current tags; supply to replace the full set (empty array clears)"`
	Summary *string   `json:"summary,omitempty" jsonschema:"omit to keep current summary; supply to replace (empty string clears). If content changes and a caller-authored summary exists, you MUST address it (re-send to keep, update, or clear) or the update is rejected"`
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
	ID        string        `json:"id,omitempty" jsonschema:"omit to create; supply the full UUID or short_id to replace in place"`
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
	ID     string `json:"id" jsonschema:"the memory's full UUID or its short_id"`
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

// toMemory builds the common store.Memory from the shared store fields. The
// caller supplies the server-set identity (owner/actor) and creation instant;
// for scheduled records the caller then sets NotBefore/NotAfter on the returned
// value. Both store_memory and schedule_memory funnel through here so their
// record shape stays aligned.
func (a storeArgs) toMemory(owner, actor string, createdAt time.Time) store.Memory {
	src := store.SummarySourceNone
	if a.Summary != "" {
		src = store.SummarySourceClient
	}
	return store.Memory{
		ID:            uuid.NewString(),
		Content:       a.Content,
		Scope:         a.Scope,
		Repo:          a.Repo,
		Workspace:     a.Workspace,
		Worktree:      a.Worktree,
		BaseDir:       a.BaseDir,
		Source:        a.Source,
		Category:      a.Category,
		Tags:          a.Tags,
		Summary:       a.Summary,
		SummarySource: src,
		Actor:         actor,
		Owner:         owner,
		CreatedAt:     createdAt,
	}
}

func (d *deps) storeMemory(ctx context.Context, a storeArgs) (string, string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(subj.Owner(), actorFromContext(ctx), d.clock())
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	// Enqueue only after a confirmed-successful Upsert; never blocks/errors
	// the write path even when the queue is disabled or full (SC#1, SC#2).
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}

// clock returns the handler's current time in UTC, defaulting to wall-clock
// time.Now when deps.now is unset so bare &deps{} literals and buildDepsFromEnv
// keep working without wiring a clock.
func (d *deps) clock() time.Time {
	if d.now != nil {
		return d.now().UTC()
	}
	return time.Now().UTC()
}

func (d *deps) scheduleMemory(ctx context.Context, a scheduleArgs) (string, string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	now := d.clock()
	nb, na, err := parseWindow(a, now)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(subj.Owner(), actorFromContext(ctx), now)
	m.NotBefore = nb
	m.NotAfter = na
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	// Enqueue only after a confirmed-successful Upsert; never blocks/errors
	// the write path even when the queue is disabled or full (SC#1, SC#2).
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}

// storeDiscovery persists a client-authored discovery. It deliberately never
// enqueues for async summary fill: discoveries own their own summaries (D-06
// negative space) — see TestDiscoveryAndRuleNeverEnqueue.
func (d *deps) storeDiscovery(ctx context.Context, a storeDiscoveryArgs) (string, string, error) {
	if err := validateStoreDiscovery(a); err != nil {
		return "", "", err
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}

	pointID := ""        // resolved UUID for replace; "" for a fresh create
	carriedShortID := "" // existing handle to preserve across replace
	if a.ID != "" {
		resolved, rerr := d.st.ResolvePointID(ctx, a.ID)
		if rerr != nil {
			// ResolvePointID's errors already echo the caller's own input, so
			// they propagate as-is. Its fast path accepts any well-formed UUID
			// without an existence check — ErrNotFound here always means a
			// failed short-id lookup: a fresh client-supplied UUID seeds a new
			// point via that fast path plus OwnedOrAbsent's absent permission;
			// a nonexistent short id cannot seed one.
			return "", "", rerr
		}
		pointID = resolved
		if err := d.st.OwnedOrAbsent(ctx, pointID, subj); err != nil {
			// Re-wrap not-found with the caller's ORIGINAL input: pointID may
			// be another owner's record resolved from their short id, and
			// echoing the resolved UUID would leak existence and identity.
			if errors.Is(err, store.ErrNotFound) {
				return "", "", fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
			}
			return "", "", err
		}
		if existing, gerr := d.st.Get(ctx, pointID); gerr == nil {
			carriedShortID = existing.ShortID
		} else if !errors.Is(gerr, store.ErrNotFound) {
			return "", "", gerr
		}
	}

	vec, err := d.em.Embed(ctx, store.EmbedText(a.Content, a.Tags))
	if err != nil {
		return "", "", err
	}
	cites := make([]store.Citation, len(a.Citations))
	for i, c := range a.Citations {
		cites[i] = store.Citation{Kind: c.Kind, Ref: c.Ref, Locator: c.Locator, Pin: c.Pin, Excerpt: c.Excerpt}
	}

	id := pointID
	if id == "" {
		id = uuid.NewString()
	}
	shortID := carriedShortID
	if shortID == "" {
		if shortID, err = d.st.MintShortID(ctx, nil); err != nil {
			return "", "", err
		}
	}
	m := store.Memory{
		ID:        id,
		ShortID:   shortID,
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
		CreatedAt: d.clock(),
	}
	return m.ID, m.ShortID, d.st.Upsert(ctx, m, vec)
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

func (d *deps) listMemory(ctx context.Context, a listArgs) ([]any, string, error) {
	if a.Limit == 0 {
		a.Limit = 20
	}
	after, err := parseRFC3339(a.CreatedAfter)
	if err != nil {
		return nil, "", fmt.Errorf("created_after: %w", err)
	}
	before, err := parseRFC3339(a.CreatedBefore)
	if err != nil {
		return nil, "", fmt.Errorf("created_before: %w", err)
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, "", err
	}
	ms, _, next, err := d.st.List(ctx, a.Scope, subj, store.ListOptions{
		Limit:         a.Limit,
		Tags:          a.Tags,
		CreatedAfter:  after,
		CreatedBefore: before,
		Cursor:        a.Cursor,
		CursorMode:    true,
	})
	if err != nil {
		return nil, "", err
	}
	return shapeRecall(ms, a.Full, d.summaryMaxChars), next, nil
}

func (d *deps) listScheduled(ctx context.Context, a listScheduledArgs) ([]store.Memory, error) {
	if a.Limit == 0 {
		a.Limit = 20
	}
	after, err := parseRFC3339(a.CreatedAfter)
	if err != nil {
		return nil, fmt.Errorf("created_after: %w", err)
	}
	before, err := parseRFC3339(a.CreatedBefore)
	if err != nil {
		return nil, fmt.Errorf("created_before: %w", err)
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
	return d.st.ListScheduled(ctx, a.Scope, subj, state,
		store.ListOptions{Limit: a.Limit, CreatedAfter: after, CreatedBefore: before})
}

func (d *deps) searchMemory(ctx context.Context, a searchArgs) ([]any, error) {
	if a.K == 0 {
		a.K = 8
	}
	after, err := parseRFC3339(a.CreatedAfter)
	if err != nil {
		return nil, fmt.Errorf("created_after: %w", err)
	}
	before, err := parseRFC3339(a.CreatedBefore)
	if err != nil {
		return nil, fmt.Errorf("created_before: %w", err)
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	vec, err := d.em.EmbedQuery(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	ms, err := d.st.SearchReranked(ctx, a.Scope, subj, a.Query, vec, a.K, a.Tags, after, before)
	if err != nil {
		return nil, err
	}
	return shapeRecall(ms, a.Full, d.summaryMaxChars), nil
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
	vec, err := d.em.EmbedQuery(ctx, a.Query)
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
	// Resolve id or short id to the point UUID (owner-agnostic; the gate governs).
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return err
	}
	// Ownership gate before embedding: one authoritative Get. A non-owner (or
	// missing record) gets ErrNotFound here, before the billable embed or write.
	// Re-wrap not-found with the caller's ORIGINAL input so a resolved short id
	// never leaks another owner's real UUID.
	cur, err := d.st.FetchForUpdate(ctx, pid, subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return err
	}
	// Rule guard (cur.Category is known for free from the fetch above): a rule's
	// summary is its index line — it must stay a non-empty single line; and a
	// rule is always shared — reject an un-share here too, mirroring
	// set_visibility, so update_memory cannot be used to bypass that gate. Both
	// checks run before the embed/write.
	if cur.Category == "rule" {
		if a.Summary != nil {
			if err := validateRuleSummary(*a.Summary); err != nil {
				return err
			}
		}
		if a.Shared != nil && !*a.Shared {
			return fmt.Errorf("rules are always shared — delete the rule instead of making it private")
		}
	}
	// Resolve the summary BEFORE embedding so a stale-summary rejection costs no
	// embed call. The owner gate has already run, so a rejected caller never
	// reaches here and never learns whether a summary exists.
	value, apply, err := resolveSummaryUpdate(cur, a.Content != cur.Content, a.Summary)
	if err != nil {
		return err
	}
	var sumArg *string
	if apply {
		sumArg = &value
	}
	// Tags are part of the embedded document (EmbedText), so re-embed with the
	// tag set that will persist: the replacement when supplied, else the current
	// tags. This re-embeds even on a tags-only change.
	tags := cur.Tags
	if a.Tags != nil {
		tags = *a.Tags
	}
	vec, err := d.em.Embed(ctx, store.EmbedText(a.Content, tags))
	if err != nil {
		return err
	}
	return d.st.Update(ctx, cur, a.Content, a.Shared, a.Tags, sumArg, vec)
}

// getMemory fetches one record by id or short id. Resolution is owner-agnostic;
// the GetReadable gate governs visibility. A not-found from the gate is re-wrapped
// with the caller's ORIGINAL input so a resolved short id never leaks another
// owner's real UUID.
func (d *deps) getMemory(ctx context.Context, a idArgs) (store.Memory, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return store.Memory{}, err
	}
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return store.Memory{}, err
	}
	m, err := d.st.GetReadable(ctx, pid, subj)
	if errors.Is(err, store.ErrNotFound) {
		return store.Memory{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
	}
	if err == nil {
		// D-01: count only on a successful fetch-by-id; call-and-ignore — the
		// read's latency/success is never coupled to the counter write.
		d.usageQueue.tryEnqueue(pid)
	}
	return m, err
}

// deleteMemory deletes one record by id or short id. Same no-leak re-wrap as
// getMemory: the Delete gate's not-found echoes only the caller's input.
func (d *deps) deleteMemory(ctx context.Context, a idArgs) error {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return err
	}
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return err
	}
	if err := d.st.Delete(ctx, pid, subj); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return err
	}
	return nil
}

// setVisibility shares/unshares one record by id or short id. Same no-leak
// re-wrap: the SetVisibility gate's not-found echoes only the caller's input.
func (d *deps) setVisibility(ctx context.Context, a setVisibilityArgs) error {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return err
	}
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return err
	}
	// Rules are always shared: reject any visibility change on a rule. Read the
	// record to learn its category (ResolvePointID returns only the UUID). Run
	// this BEFORE the write-ownership gate so the actionable "always shared"
	// message wins over an owner-only ErrNotFound (spec implementation-order note;
	// rules are unconditionally readable, so this is not a leak).
	rec, err := d.st.GetReadable(ctx, pid, subj)
	if err != nil {
		// Re-wrap not-found with the caller's ORIGINAL input: pid is the resolved
		// UUID (possibly another owner's, resolved from their short id), and
		// GetReadable embeds it in ErrNotFound — echoing pid would leak the real
		// UUID (404-indistinguishability). Mirrors the SetVisibility gate below.
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return err
	}
	if rec.Category == "rule" {
		return fmt.Errorf("rules are always shared — delete the rule instead of changing its visibility")
	}
	if err := d.st.SetVisibility(ctx, pid, subj, a.Shared); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return err
	}
	return nil
}

// Register wires the memory tools onto the MCP server. It accepts a pre-built
// *telemetry.ToolMetrics (constructed once in runServe and reused for both tool
// instrumentation and auth-failure recording) so there is a single instrument
// instance rather than two disjoint ones, and pre-built
// *telemetry.SummaryQueueMetrics / *telemetry.UsageQueueMetrics (also built
// once in runServe) threaded into buildDepsFromEnv so each async queue's
// static instruments share the same meter. Returns a shutdown closure the
// caller invokes on shutdown to drain BOTH the summary queue and the usage
// queue — a no-op for either queue that is disabled (nil) — and an error if
// dependency construction (store/embedder) fails, so the caller can flush
// telemetry and exit cleanly rather than aborting via log.Fatal.
func Register(s *mcp.Server, mux *http.ServeMux, tm *telemetry.ToolMetrics, sqm *telemetry.SummaryQueueMetrics, uqm *telemetry.UsageQueueMetrics, resolve connectResolver) (shutdown func(context.Context), err error) {
	d, err := buildDepsFromEnv(sqm, uqm)
	if err != nil {
		return nil, fmt.Errorf("build deps: %w", err)
	}
	if err := d.mountConnect(mux, resolve); err != nil {
		return nil, fmt.Errorf("mount connect: %w", err)
	}

	s.AddReceivingMiddleware(instrumentTools(tm.Record))

	mcp.AddTool(s, &mcp.Tool{Name: "store_memory", Description: "Persist a deliberate, well-formed memory. Do NOT store transient state, secrets, or timestamps. Optionally pass `summary`: a one-line recall summary shown in place of content (keep negations/identifiers verbatim). The result includes the memory's id and short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeArgs) (*mcp.CallToolResult, any, error) {
			id, sid, err := d.storeMemory(ctx, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "schedule_memory", Description: "Persist a memory with a validity window (not_before defers recall; not_after expires it). At least one bound (RFC3339) is required; use store_memory for unscheduled records. The result includes the memory's id and short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scheduleArgs) (*mcp.CallToolResult, any, error) {
			id, sid, err := d.scheduleMemory(ctx, a)
			return textResult(fmt.Sprintf("scheduled %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_memory", Description: "Semantic search within a scope. Optionally pass `tags` to restrict to records carrying all listed tags (AND) before ranking. Returns compact summaries by default (id, summary, summary_source, scope, category, tags, created_at); pass `full=true` for full content, or fetch one record in full via get_memory. Each result carries a `score`: the raw Qdrant cosine similarity for this query (higher = closer), present when non-zero; unranked list_memory/get_memory results have a zero/omitted score."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchArgs) (*mcp.CallToolResult, any, error) {
			hits, err := d.searchMemory(ctx, a)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"memories": hits}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_memory", Description: "List memories in a scope without a query. Most-recent first. Optional `created_after`/`created_before` (RFC3339) window and `cursor` for paging (use the returned next_cursor). Optional `tags` (AND). Returns {memories, next_cursor}; compact summaries by default, `full=true` for full content."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listArgs) (*mcp.CallToolResult, any, error) {
			mems, next, err := d.listMemory(ctx, a)
			return textResult(fmt.Sprintf("%d memories", len(mems))), map[string]any{"memories": mems, "next_cursor": next}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_scheduled", Description: "List your windowed memories the recall gate is hiding: state=scheduled (not yet active, default) | expired | all. Active memories surface via list_memory/search_memory."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listScheduledArgs) (*mcp.CallToolResult, any, error) {
			mems, err := d.listScheduled(ctx, a)
			return textResult(fmt.Sprintf("%d scheduled", len(mems))), map[string]any{"memories": mems}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_memory", Description: "Fetch one memory by id. Unlike search_memory/list_memory, fetch-by-id is NOT recall-gated: it returns scheduled (not-yet-active) and expired records too. The id may be the full UUID or the short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			m, err := d.getMemory(ctx, a)
			return textResult(m.Content), m, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_memory", Description: "Replace a memory's content in place (re-embeds). Optionally set `shared` to toggle visibility (true=shared, false=private); omit to keep current visibility. Optionally set `tags` to replace the full tag set (empty array clears); omit to keep current tags. Optionally set `summary` to replace the recall summary (empty string clears); omit to keep current. If you change content while a caller-authored summary exists, you must address the summary (re-send, update, or clear) or the update is rejected. The id may be the full UUID or the short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, any, error) {
			err := d.updateMemory(ctx, a)
			return textResult("updated"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_memory", Description: "Delete one memory by id. The id may be the full UUID or the short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			err := d.deleteMemory(ctx, a)
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

	mcp.AddTool(s, &mcp.Tool{Name: "store_discovery", Description: "Cache agent-earned codebase understanding with citations. kind=map|fact; >=1 citation; scope discovery:repo:<repo>. The result includes the discovery's id and short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			id, sid, err := d.storeDiscovery(ctx, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_discovery", Description: "Semantic search over the discovery pool. scope required unless cross_spine=true; optional kind=map|fact. Results carry citations + created_at (aging signals)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			hits, err := d.searchDiscovery(ctx, a)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"discoveries": hits}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "set_visibility", Description: "Share or unshare a memory you own. shared=true → readable by any authenticated caller (never writable by others); false → private. The id may be the full UUID or the short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a setVisibilityArgs) (*mcp.CallToolResult, any, error) {
			err := d.setVisibility(ctx, a)
			return textResult("visibility updated"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "store_rule", Description: "Persist a NORMATIVE rule (ground truth) for a repo/project. Call ONLY on explicit user instruction — never promote a rule unilaterally; propose it to the user instead. scope=rule:repo:<repo> or rule:project:<project>. summary is REQUIRED and is the one-line index entry (single line). Rules are always shared and user-blessed. The result includes the rule's id and short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeRuleArgs) (*mcp.CallToolResult, any, error) {
			id, sid, err := d.storeRule(ctx, a)
			return textResult(fmt.Sprintf("stored rule %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_rules", Description: "List the COMPLETE rule set for one or more rule:* scopes, oldest-first. Compact index shape by default (short_id, summary, tags); full=true adds content. Optional tags filter (AND). Rules are the repo/project's normative ground truth."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listRulesArgs) (*mcp.CallToolResult, any, error) {
			rules, advisory, err := d.listRules(ctx, a)
			msg := fmt.Sprintf("%d rules", len(rules))
			if advisory != "" {
				msg += " (" + advisory + ")"
			}
			return textResult(msg), map[string]any{"rules": rules}, err
		})
	// Compose both queues' shutdown into a single closure: the caller
	// (serve.go) invokes this once, strictly after httpSrv.Shutdown returns,
	// draining the summary queue then the usage queue (both nil-safe no-ops
	// when disabled).
	return func(ctx context.Context) {
		d.summaryQueue.Shutdown(ctx)
		d.usageQueue.Shutdown(ctx)
	}, nil
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
