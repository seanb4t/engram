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
	// st is the narrow memStore interface (not the concrete *store.Store) so a
	// fake can substitute for it in tests without a live Qdrant (D-10
	// prerequisite). buildDepsFromEnv still constructs and assigns a concrete
	// *store.Store here — a pure interface carve with zero behavior change.
	st memStore
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
	// embedderIdentity is the computed embedder-config-identity stamp
	// (config.EmbedderIdentity), computed ONCE at deps construction
	// (buildDepsFromEnv) and stamped on every document-embed write site
	// before Upsert/Update (Phase 13 D-05). Empty is acceptable for a bare
	// &deps{} test literal — the field is payload-only (store.Memory.
	// EmbedderIdentity is json:"-"), so an empty value there just persists
	// as an empty stamp, never surfacing on any wire path.
	embedderIdentity string
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
// configured embed dimension), the matching embedder, and the computed
// embedder-config-identity (config.EmbedderIdentity) from a SINGLE config
// load. reindex uses it so the source collection must already exist (no-ensure)
// rather than being silently created at the new dimension, config is parsed
// exactly once — the engram-635 single-load invariant, applied to the reindex
// path the same way buildDepsFromEnv applies it to serve — and reindex has a
// path to the identity string to stamp onto reindexed records (Phase 13 SC3,
// ReindexOptions.Identity).
func StoreAndEmbedderFromEnvNoEnsure() (*store.Store, uint64, *embed.Client, string, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, 0, nil, "", err
	}
	st, dim, err := storeFromConfig(cfg)
	if err != nil {
		return nil, 0, nil, "", err
	}
	em, err := embedderFromConfig(cfg)
	if err != nil {
		return nil, 0, nil, "", err
	}
	identity, err := config.EmbedderIdentity(cfg)
	if err != nil {
		return nil, 0, nil, "", fmt.Errorf("embedder identity: %w", err)
	}
	return st, dim, em, identity, nil
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
	identity, err := config.EmbedderIdentity(cfg)
	if err != nil {
		return nil, fmt.Errorf("embedder identity: %w", err)
	}
	return &deps{
		st:               st,
		em:               em,
		summaryMaxChars:  summaryMaxChars(cfg),
		summaryQueue:     buildSummaryQueue(cfg, st, sqm),
		usageQueue:       buildUsageQueue(cfg, st, uqm),
		embedderIdentity: identity,
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

// embedTimeout parses the per-request embed HTTP client timeout, defaulting to
// 30s on empty/invalid. 0 is honored (disables the timeout); negatives fall
// back to the default. Mirrors summaryTimeout.
func embedTimeout(cfg *config.Config) time.Duration {
	d, err := time.ParseDuration(cfg.Embed.Timeout)
	if err != nil || d < 0 {
		if cfg.Embed.Timeout != "" {
			slog.Warn("ENGRAM_EMBED_TIMEOUT is set but unparseable or negative; using default 30s",
				"value", cfg.Embed.Timeout)
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
		embed.WithDocumentParams(documentParams),
		embed.WithTimeout(embedTimeout(cfg)),
		embed.WithEmbeddingsURL(cfg.OpenAI.EmbeddingsURL)), nil
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
	// IdempotencyKey is promoted onto scheduleArgs via Go field embedding
	// (both store_memory and schedule_memory gain it from this single
	// declaration, D-13) — do NOT declare it separately on scheduleArgs.
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional owner-scoped replay-safety key: a repeat call with the same key and identical content returns the original record unchanged (no duplicate, no side-effects); the same key with different content is rejected; omit for a fresh record every time"`
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
// Every rejection is wrapped with the existing store.ErrInvalidArgument (review
// finding 5) so a Connect ScheduleMemory call maps to CodeInvalidArgument, not
// CodeInternal, once 17-04 wires the connectError mapper.
func parseWindow(a scheduleArgs, now time.Time) (nb, na *time.Time, err error) {
	if a.NotBefore == "" && a.NotAfter == "" {
		return nil, nil, fmt.Errorf("schedule_memory requires not_before and/or not_after (use store_memory for unscheduled records): %w", store.ErrInvalidArgument)
	}
	if a.Category == "discovery" {
		return nil, nil, fmt.Errorf("discovery is not schedulable; use store_discovery: %w", store.ErrInvalidArgument)
	}
	if a.NotBefore != "" {
		t, perr := time.Parse(time.RFC3339, a.NotBefore)
		if perr != nil {
			return nil, nil, fmt.Errorf("not_before: %w: %w", perr, store.ErrInvalidArgument)
		}
		nb = &t
	}
	if a.NotAfter != "" {
		t, perr := time.Parse(time.RFC3339, a.NotAfter)
		if perr != nil {
			return nil, nil, fmt.Errorf("not_after: %w: %w", perr, store.ErrInvalidArgument)
		}
		if !t.After(now) {
			return nil, nil, fmt.Errorf("not_after %s is not in the future: %w", a.NotAfter, store.ErrInvalidArgument)
		}
		na = &t
	}
	if nb != nil && na != nil && !nb.Before(*na) {
		return nil, nil, fmt.Errorf("not_before must be strictly before not_after: %w", store.ErrInvalidArgument)
	}
	return nb, na, nil
}

// supersedeArgs embeds storeArgs (mirroring scheduleArgs's embedding shape,
// D-03/RESEARCH A4) so supersede_memory inherits the full store_memory field
// set (content/scope/source/category/tags/repo/workspace/worktree/base_dir/
// summary) without a hand-rolled parallel field list — the exact drift class
// persistAndEnqueue's doc comment already flags (tools.go:734-736).
//
// idempotency_key is intentionally NOT supported on supersede_memory this
// phase (WR-03/WR-04, plan T-25-10 deferred scope): deps.supersedeMemory
// never calls checkIdempotentReplay or stamps IdempotencyFingerprint, so any
// idempotency_key a caller sends is silently IGNORED — a normal supersede
// happens, no replay, no error (see the defensive clear in supersedeMemory
// below). IdempotencyKey below (a depth-0 `json:"-"` field) DOES remove
// idempotency_key from the advertised JSON schema: jsonschema-go's
// reflect.VisibleFields-based inference applies Go's normal
// shallowest-depth-wins shadowing rule, so this field wins over storeArgs'
// depth-1 promoted one there (pinned by
// TestSupersedeMemorySchemaExcludesIdempotencyKey).
//
// It does NOT, however, remove idempotency_key from the wire DECODE: a
// `json:"-"` field has no JSON name, so it never enters encoding/json's
// same-name shadowing contest — it just excuses itself, leaving the
// promoted storeArgs.IdempotencyKey (json:"idempotency_key,omitempty") as
// the sole decode target for that key. A caller that (incorrectly, since it
// isn't advertised) sends idempotency_key on supersede_memory therefore
// STILL populates a.storeArgs.IdempotencyKey on the wire (pinned by
// TestSupersedeArgsDecodePopulatesPromotedIdempotencyKey) — the defensive
// clear at the top of supersedeMemory is what actually makes it inert, not
// this shadow.
type supersedeArgs struct {
	storeArgs
	Supersedes string `json:"supersedes" jsonschema:"id (full UUID or short_id) of the memory this new record corrects/replaces"`
	// IdempotencyKey shadows storeArgs.IdempotencyKey for schema purposes
	// only — see the type doc comment above. Never read.
	IdempotencyKey string `json:"-"`
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
	ID string `json:"id" jsonschema:"the memory's full UUID or its short_id"`
	// Content is presence-signaled (nil = unchanged) so deps.updateMemory can
	// route a shared/summary-only change to the payload-only store method
	// (no re-embed, vector preserved) instead of unconditionally re-embedding
	// and blanking content on a nil value (landmine 2). The MCP tool still
	// requires content on every call (schema unchanged: no omitempty), so a
	// non-nil pointer is always populated on that lane, preserving MCP's
	// existing full-replace behavior byte-for-byte; the Connect protoconv
	// layer (17-03/17-04) is what actually supplies nil, for a field-mask
	// update that omits "content".
	Content *string   `json:"content"`
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

// maxIdempotencyKeyBytes bounds storeArgs.IdempotencyKey (IN-01): it is a
// short opaque client-generated retry token, not free-form content, so a
// modest cap is defense-in-depth against oversized payloads — consistent
// with the size-bound discipline the store_discovery fields above already
// establish for other client-supplied strings in this file.
const maxIdempotencyKeyBytes = 512

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

// checkIdempotentReplay resolves the D-08 check-before-embed branch for a
// keyed store_memory/schedule_memory call — the same "resolve ID -> Get
// existing -> decide -> THEN embed" shape storeDiscovery already uses, minus
// its OwnedOrAbsent gate (D-09: owner is baked into the point-ID hash, so a
// raw Get can only ever resolve to the caller's own record). Absent a key,
// this is a no-op: the keyless path is untouched and toMemory keeps minting a
// fresh uuid.NewString() (SC5). With a key it does a single point Get at the
// deterministic ID — never a search/scroll (D-05) — and returns one of three
// outcomes:
//   - absent (store.ErrNotFound): replay=false, pointID set — fall through to
//     create AT this resolved pointID; the caller MUST thread it into m.ID
//     rather than recompute it independently (RESEARCH Pattern 2 anti-pattern).
//   - fingerprint match: replay=true, id/shortID are the ORIGINAL record's —
//     the caller returns immediately, before Embed/persistAndEnqueue (SC1
//     zero side-effects: no re-embed, no new short_id, no summary re-enqueue).
//   - fingerprint mismatch: replay=false, err wraps store.ErrIdempotencyConflict
//     — surfaced BEFORE Embed (SC2), never a silent overwrite, never a 404.
//
// IN-01: the point ID is derived from (owner, scope, key) alone — there is no
// tool discriminator, so store_memory and schedule_memory SHARE the same
// idempotency-key namespace by design. A store_memory call with a given key
// followed by a schedule_memory call reusing that same scope+key+content is a
// cross-tool replay: it returns the ORIGINAL (unscheduled) record, with no
// window ever applied, indistinguishable from a genuinely scheduled write.
// This is intentional (see the D-07 fingerprint excluding the schedule
// window, RESEARCH Open Question 1) and MUST NOT be changed unilaterally —
// altering the point-ID hash input is a locked design decision (D-07/D-08)
// that would silently un-dedup every previously keyed record on its next
// replay. See TestCheckIdempotentReplayCrossToolNamespaceShared for the
// pinned current behavior.
func (d *deps) checkIdempotentReplay(ctx context.Context, owner string, a storeArgs) (replay bool, id, shortID, pointID string, err error) {
	if a.IdempotencyKey == "" {
		return false, "", "", "", nil
	}
	if len(a.IdempotencyKey) > maxIdempotencyKeyBytes {
		return false, "", "", "", fmt.Errorf("idempotency_key too large: %d bytes (max %d): %w", len(a.IdempotencyKey), maxIdempotencyKeyBytes, store.ErrInvalidArgument)
	}
	pointID = idempotencyPointID(owner, a.Scope, a.IdempotencyKey)
	existing, gerr := d.st.Get(ctx, pointID)
	switch {
	case errors.Is(gerr, store.ErrNotFound):
		return false, "", "", pointID, nil
	case gerr != nil:
		return false, "", "", "", gerr
	}
	if contentFingerprint(a) == existing.IdempotencyFingerprint {
		return true, existing.ID, existing.ShortID, "", nil
	}
	return false, "", "", "", fmt.Errorf("idempotency key %q reused with different content: %w", a.IdempotencyKey, store.ErrIdempotencyConflict)
}

func (d *deps) storeMemory(ctx context.Context, c caller, a storeArgs) (string, string, error) {
	owner := c.Subj.Owner()
	replay, id, shortID, pointID, err := d.checkIdempotentReplay(ctx, owner, a)
	if err != nil {
		return "", "", err
	}
	if replay {
		return id, shortID, nil
	}
	m := a.toMemory(owner, c.Actor, d.clock())
	m.EmbedderIdentity = d.embedderIdentity
	if pointID != "" {
		m.ID = pointID
		m.IdempotencyFingerprint = contentFingerprint(a)
	}
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	return d.persistAndEnqueue(ctx, m, vec)
}

// persistAndEnqueue is the shared MintShortID -> Upsert -> tryEnqueue tail
// shared by storeMemory and scheduleMemory (IN-01/D-05): the two handlers
// duplicated this sequence verbatim, which let the write-path invariant
// silently diverge. Callers must have already built m and vec (embed done,
// owner/actor stamped, any schedule window applied) before calling this.
// Enqueue only happens after a confirmed-successful Upsert; never
// blocks/errors the write path even when the queue is disabled or full
// (SC#1, SC#2). storeDiscovery and storeRule deliberately do NOT call this —
// discoveries and rules own their own summaries (see storeDiscovery's
// doc-comment) — do not fold them in.
func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error) {
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	// Re-read the point after Upsert so a concurrent keyed racer that lost the
	// last-write-wins race (same deterministic pointID, independently minted
	// short_id) returns the short_id that was ACTUALLY PERSISTED, not the one
	// it discarded (CR-01). Upsert replaces the whole payload, so whichever
	// racer wrote last owns the point's short_id going forward; a failed
	// re-Get here is non-fatal — fall back to the locally-minted value rather
	// than fail an otherwise-successful write.
	//
	// Gated to the keyed path only (WR-01): m.IdempotencyFingerprint is only
	// ever non-empty when this write used a deterministic pointID (set in
	// storeMemory/scheduleMemory right before calling persistAndEnqueue). On
	// a keyless write, m.ID is a fresh uuid.NewString() that no concurrent
	// request can ever target, so the race this re-Get resolves is
	// structurally impossible there — skip the extra Qdrant round trip on
	// the overwhelmingly common (keyless) case.
	if m.IdempotencyFingerprint != "" {
		if persisted, gerr := d.st.Get(ctx, m.ID); gerr == nil {
			m.ShortID = persisted.ShortID
		}
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

func (d *deps) scheduleMemory(ctx context.Context, c caller, a scheduleArgs) (string, string, error) {
	// checkIdempotentReplay runs BEFORE parseWindow's future-only validation
	// (WR-02): the window is deliberately excluded from the D-07 content
	// fingerprint precisely so a retry doesn't need to resend a still-valid
	// window. If parseWindow ran first, a delayed retry of an
	// already-successful schedule_memory call with the SAME not_after value
	// could be rejected with ErrInvalidArgument (now no longer in the
	// future) before checkIdempotentReplay ever got a chance to recognize it
	// as a no-op replay. parseWindow only needs to run on the non-replay
	// (create) path below.
	owner := c.Subj.Owner()
	replay, id, shortID, pointID, err := d.checkIdempotentReplay(ctx, owner, a.storeArgs)
	if err != nil {
		return "", "", err
	}
	if replay {
		// The schedule window is excluded from the D-07 content fingerprint
		// (RESEARCH Open Question 1, deliberately resolved): a replay with a
		// CHANGED not_before/not_after still returns the original record with
		// its ORIGINAL window unchanged.
		return id, shortID, nil
	}
	now := d.clock()
	nb, na, err := parseWindow(a, now)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(owner, c.Actor, now)
	m.NotBefore = nb
	m.NotAfter = na
	m.EmbedderIdentity = d.embedderIdentity
	if pointID != "" {
		m.ID = pointID
		m.IdempotencyFingerprint = contentFingerprint(a.storeArgs)
	}
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	return d.persistAndEnqueue(ctx, m, vec)
}

// storeDiscovery persists a client-authored discovery. It deliberately never
// enqueues for async summary fill: discoveries own their own summaries (D-06
// negative space) — see TestDiscoveryAndRuleNeverEnqueue.
func (d *deps) storeDiscovery(ctx context.Context, c caller, a storeDiscoveryArgs) (string, string, error) {
	if err := validateStoreDiscovery(a); err != nil {
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
		if err := d.st.OwnedOrAbsent(ctx, pointID, c.Subj); err != nil {
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
	for i, cit := range a.Citations {
		cites[i] = store.Citation{Kind: cit.Kind, Ref: cit.Ref, Locator: cit.Locator, Pin: cit.Pin, Excerpt: cit.Excerpt}
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
		ID:               id,
		ShortID:          shortID,
		Content:          a.Content,
		Scope:            a.Scope,
		Source:           "agent-inferred",
		Category:         "discovery",
		Kind:             a.Kind,
		Citations:        cites,
		Summary:          a.Summary,
		Tags:             a.Tags,
		Actor:            c.Actor,
		Owner:            c.Subj.Owner(),
		CreatedAt:        d.clock(),
		EmbedderIdentity: d.embedderIdentity,
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

// coreListRequest is the transport-neutral list request: a SUPERSET of both the
// MCP list_memory args and the Connect ListMemories fields (D-07). It carries
// every Connect field (Offset, Categories, Visibility, exact Total via the
// result) so the Connect read rewire (17-04) drops nothing. CreatedAfter/
// CreatedBefore are time.Time (round-4 MED-6): each transport parses its own
// wire form (RFC3339 string for MCP, proto Timestamp for Connect) at its own
// boundary BEFORE building this request, so a parse failure is owned by the
// transport and never misclassified by connectError as CodeInternal. Limit==0
// carries NO "default to 20" meaning here (round-4 finding-7): each adapter
// applies its own default (MCP 20, Connect leaves 0 = "all", store.go:873-874)
// before calling deps.listMemory. CursorMode is carried from the request, not
// hardcoded (round-3 HIGH-2) — the MCP list_memory closure sets it true to
// preserve today's unconditional MCP cursor-mode pagination.
type coreListRequest struct {
	Scope         string
	Limit         uint64
	Offset        uint64
	Categories    []string
	Visibility    string
	Tags          []string
	CreatedAfter  time.Time
	CreatedBefore time.Time
	Cursor        string
	CursorMode    bool
}

// coreListResult is the typed list result: raw []store.Memory (no []any, no
// MCP/Connect-specific shaping — each transport shapes its own presentation),
// the exact matched Total (store.go's server-side Count, not a page-size
// approximation), and NextToken (empty in offset mode; populated only by a
// full first page in cursor mode — store.go:817/:865).
type coreListResult struct {
	Memories  []store.Memory
	Total     uint64
	NextToken string
}

// coreSearchRequest is the transport-neutral search request: a SUPERSET
// carrying every field either lane needs. K carries NO internal default
// (round-4 finding-7 discipline, same as the list Limit): each adapter applies
// its own default (MCP 8, Connect 20) before calling deps.searchMemory —
// store.SearchReranked rejects K==0 with ErrInvalidArgument. CreatedAfter/
// CreatedBefore are time.Time for the same reason as coreListRequest.
type coreSearchRequest struct {
	Scope         string
	Query         string
	K             uint64
	Tags          []string
	CreatedAfter  time.Time
	CreatedBefore time.Time
}

// listMemory returns a page of the caller's readable records in scope on the
// transport-neutral typed core contract (D-07): every Connect list field
// (offset/categories/visibility/exact total/cursor/cursor_mode) survives the
// shared path, and the result is raw []store.Memory — no MCP-shaped []any.
// deps.listMemory applies NO Limit/CursorMode default; each transport adapter
// applies its own (MCP closure: limit=20, CursorMode=true; Connect: limit=0
// means "all", CursorMode carried from the request) before calling here
// (round-3 HIGH-2, round-4 finding-7).
func (d *deps) listMemory(ctx context.Context, c caller, req coreListRequest) (coreListResult, error) {
	ms, total, next, err := d.st.List(ctx, req.Scope, c.Subj, store.ListOptions{
		Limit:         req.Limit,
		Offset:        req.Offset,
		Categories:    req.Categories,
		Visibility:    req.Visibility,
		Tags:          req.Tags,
		CreatedAfter:  req.CreatedAfter,
		CreatedBefore: req.CreatedBefore,
		Cursor:        req.Cursor,
		CursorMode:    req.CursorMode,
	})
	if err != nil {
		return coreListResult{}, err
	}
	return coreListResult{Memories: ms, Total: total, NextToken: next}, nil
}

func (d *deps) listScheduled(ctx context.Context, c caller, a listScheduledArgs) ([]store.Memory, error) {
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
	return d.st.ListScheduled(ctx, a.Scope, c.Subj, state,
		store.ListOptions{Limit: a.Limit, CreatedAfter: after, CreatedBefore: before})
}

// searchMemory runs the shared rerank search on the transport-neutral typed
// core contract (D-07): raw []store.Memory, no MCP-shaped []any. It applies NO
// internal k default (round-4 finding-7, same discipline as listMemory's
// Limit) — store.SearchReranked rejects K==0, so each adapter (MCP closure: 8;
// Connect: 20) must apply its own default before calling here.
func (d *deps) searchMemory(ctx context.Context, c caller, req coreSearchRequest) ([]store.Memory, error) {
	vec, err := d.em.EmbedQuery(ctx, req.Query)
	if err != nil {
		return nil, err
	}
	return d.st.SearchReranked(ctx, req.Scope, c.Subj, req.Query, vec, req.K, req.Tags, req.CreatedAfter, req.CreatedBefore)
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

func (d *deps) searchDiscovery(ctx context.Context, c caller, a searchDiscoveryArgs) ([]store.Memory, error) {
	scope, err := effectiveDiscoveryScope(a)
	if err != nil {
		return nil, err
	}
	if a.CrossSpine && a.Scope != "" {
		// Don't echo the caller-supplied scope value into logs (avoids
		// unbounded/sensitive scope strings reaching log aggregation).
		slog.InfoContext(ctx, "search_discovery: cross_spine=true; ignoring supplied scope")
	}
	// Retained MCP-lane default (round-2 finding 7): the Connect
	// SearchDiscoveries adapter (17-04) pre-applies k=20 before calling this
	// method, so this internal default governs ONLY the MCP lane. Do NOT
	// remove — deleting it would regress the Connect discovery-search default
	// from 20 to 8.
	if a.K == 0 {
		a.K = 8
	}
	vec, err := d.em.EmbedQuery(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	return d.st.SearchDiscovery(ctx, scope, a.Kind, c.Subj, vec, a.K)
}

// updateMemory applies a partial update to one record by id or short id.
// a.Content is presence-signaled (nil = unchanged): when neither content nor
// tags change, the update routes to the payload-only store method (no
// re-embed, vector preserved); when either changes, it re-embeds and routes
// through the vector-upsert path (store.Update) — the embedded document is a
// function of both content and tags (landmine 2).
func (d *deps) updateMemory(ctx context.Context, c caller, a updateArgs) (mutationResult, error) {
	// Resolve id or short id to the point UUID (owner-agnostic; the gate governs).
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return mutationResult{}, err
	}
	// Ownership gate before embedding: one authoritative Get. A non-owner (or
	// missing record) gets ErrNotFound here, before the billable embed or write.
	// Re-wrap not-found with the caller's ORIGINAL input so a resolved short id
	// never leaks another owner's real UUID.
	cur, err := d.st.FetchForUpdate(ctx, pid, c.Subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mutationResult{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return mutationResult{}, err
	}
	// Rule guard (cur.Category is known for free from the fetch above): a rule's
	// summary is its index line — it must stay a non-empty single line; and a
	// rule is always shared — reject an un-share here too, mirroring
	// set_visibility, so update_memory cannot be used to bypass that gate. Both
	// checks run before the embed/write.
	if cur.Category == "rule" {
		if a.Summary != nil {
			if err := validateRuleSummary(*a.Summary); err != nil {
				return mutationResult{}, err
			}
		}
		if a.Shared != nil && !*a.Shared {
			return mutationResult{}, fmt.Errorf("%w — delete the rule instead of making it private", errRuleImmutable)
		}
	}
	contentChanged := a.Content != nil && *a.Content != cur.Content
	// Resolve the summary BEFORE embedding so a stale-summary rejection costs no
	// embed call. The owner gate has already run, so a rejected caller never
	// reaches here and never learns whether a summary exists.
	value, apply, err := resolveSummaryUpdate(cur, contentChanged, a.Summary)
	if err != nil {
		return mutationResult{}, err
	}
	var sumArg *string
	if apply {
		sumArg = &value
	}
	if a.Content == nil && a.Tags == nil {
		// Neither content nor tags changed — only shared/summary may have.
		// Route to the payload-only method: no re-embed, existing vector
		// preserved (landmine 2 part b; review HIGH).
		if err := d.st.UpdatePayload(ctx, cur, a.Shared, sumArg); err != nil {
			return mutationResult{}, err
		}
		return mutationResult{ID: cur.ID, ShortID: cur.ShortID}, nil
	}
	// Content and/or tags changed: the embedded document changes. Re-embed with
	// the content and tag set that will persist — the replacement when
	// supplied, else the current value. This re-embeds even on a tags-only
	// change, since tags are part of EmbedText.
	contentToStore := cur.Content
	if a.Content != nil {
		contentToStore = *a.Content
	}
	tags := cur.Tags
	if a.Tags != nil {
		tags = *a.Tags
	}
	vec, err := d.em.Embed(ctx, store.EmbedText(contentToStore, tags))
	if err != nil {
		return mutationResult{}, err
	}
	// Re-stamp on every re-embed: Store.Update re-Upserts cur through
	// payload(), so this persists the CURRENT identity even if the embedder
	// config changed since the record was first written (D-05).
	cur.EmbedderIdentity = d.embedderIdentity
	if err := d.st.Update(ctx, cur, contentToStore, a.Shared, a.Tags, sumArg, vec); err != nil {
		return mutationResult{}, err
	}
	return mutationResult{ID: cur.ID, ShortID: cur.ShortID}, nil
}

// getMemory fetches one record by id or short id. Resolution is owner-agnostic;
// the GetReadable gate governs visibility. A not-found from the gate is re-wrapped
// with the caller's ORIGINAL input so a resolved short id never leaks another
// owner's real UUID.
func (d *deps) getMemory(ctx context.Context, c caller, a idArgs) (store.Memory, error) {
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return store.Memory{}, err
	}
	m, err := d.st.GetReadable(ctx, pid, c.Subj)
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
func (d *deps) deleteMemory(ctx context.Context, c caller, a idArgs) error {
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return err
	}
	if err := d.st.Delete(ctx, pid, c.Subj); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return err
	}
	return nil
}

// setVisibility shares/unshares one record by id or short id. Same no-leak
// re-wrap: the SetVisibility gate's not-found echoes only the caller's input.
func (d *deps) setVisibility(ctx context.Context, c caller, a setVisibilityArgs) (mutationResult, error) {
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return mutationResult{}, err
	}
	// Rules are always shared: reject any visibility change on a rule. Read the
	// record to learn its category (ResolvePointID returns only the UUID). Run
	// this BEFORE the write-ownership gate so the actionable "always shared"
	// message wins over an owner-only ErrNotFound (spec implementation-order note;
	// rules are unconditionally readable, so this is not a leak).
	rec, err := d.st.GetReadable(ctx, pid, c.Subj)
	if err != nil {
		// Re-wrap not-found with the caller's ORIGINAL input: pid is the resolved
		// UUID (possibly another owner's, resolved from their short id), and
		// GetReadable embeds it in ErrNotFound — echoing pid would leak the real
		// UUID (404-indistinguishability). Mirrors the SetVisibility gate below.
		if errors.Is(err, store.ErrNotFound) {
			return mutationResult{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return mutationResult{}, err
	}
	if rec.Category == "rule" {
		return mutationResult{}, fmt.Errorf("%w — delete the rule instead of changing its visibility", errRuleImmutable)
	}
	if err := d.st.SetVisibility(ctx, pid, c.Subj, a.Shared); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mutationResult{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return mutationResult{}, err
	}
	return mutationResult{ID: rec.ID, ShortID: rec.ShortID}, nil
}

// supersedeMemory corrects a memory the caller owns: it resolves the target,
// gates ownership (and rejects a rule target) BEFORE embedding, embeds the
// correcting content, and delegates the create+back-stamp to Store.Supersede
// — which re-gates the target via getWritable/ActionWrite under its own
// per-target lock (SC3: a caller with only read/shared access to the target
// cannot supersede it; D-07: supersession only ever fires from this explicit
// call, never a similarity-threshold or write-through path; CR-01: the
// store-level re-gate is what Store.Supersede's lock makes atomic against a
// concurrent racing caller). On store.ErrNotFound the error is re-wrapped
// with the caller's ORIGINAL a.Supersedes input, never the resolved target
// id — same 404-indistinguishability discipline as setVisibility/
// storeDiscovery (a non-owner cannot learn a target exists). The new
// correcting record is store_memory-shaped, so it is enqueued for async
// summary-on-write like any other store_memory write.
func (d *deps) supersedeMemory(ctx context.Context, c caller, a supersedeArgs) (string, string, error) {
	// WR-04 defense-in-depth: a.storeArgs.IdempotencyKey can be populated on
	// the wire despite supersedeArgs' json:"-" shadow field (see the
	// supersedeArgs doc comment — the shadow only removes the field from the
	// advertised schema, not the decode). Clearing it here — before it is
	// ever read — guarantees a caller-supplied idempotency_key is silently
	// ignored (no replay, no error) rather than being read by some future
	// refactor that reuses storeArgs' idempotency helpers.
	a.storeArgs.IdempotencyKey = ""
	targetID, err := d.st.ResolvePointID(ctx, a.Supersedes)
	if err != nil {
		return "", "", err
	}
	// Ownership gate BEFORE the billable embed and the Qdrant-hitting
	// MintShortID call (CR-03 cost-amplification hardening — mirrors
	// updateMemory/storeDiscovery's ordering; TestUpdateMemoryEmbedNotCalledForNonOwner's
	// pattern). FetchForUpdate is the same getWritable/ActionWrite gate
	// Store.Supersede re-runs (under its per-target lock) below; a non-owner
	// or nonexistent target is rejected here, before any spend.
	targetRec, err := d.st.FetchForUpdate(ctx, targetID, c.Subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", "", fmt.Errorf("%w: %s", store.ErrNotFound, a.Supersedes)
		}
		return "", "", err
	}
	// Rule guard (CR-02): list_rules relies on Store.List's unconditional
	// superseded_by gate to present the "complete rule set" — superseding a
	// rule would silently vanish it from that index without going through
	// the required delete flow. Mirrors updateMemory (tools.go:1116) /
	// setVisibility (tools.go:1233).
	if targetRec.Category == "rule" {
		return "", "", fmt.Errorf("%w — delete the rule instead of superseding it", errRuleImmutable)
	}
	owner := c.Subj.Owner()
	m := a.toMemory(owner, c.Actor, d.clock())
	m.EmbedderIdentity = d.embedderIdentity
	m.Supersedes = &targetID
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Supersede(ctx, m, vec, targetID, c.Subj); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Re-wrap with the caller's ORIGINAL input: targetID is the
			// resolved UUID (possibly another owner's, resolved from their
			// short id), and Supersede embeds it in ErrNotFound — echoing
			// targetID would leak the real UUID (404-indistinguishability).
			// Mirrors setVisibility/storeDiscovery.
			return "", "", fmt.Errorf("%w: %s", store.ErrNotFound, a.Supersedes)
		}
		return "", "", err
	}
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
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
func Register(s *mcp.Server, mux *http.ServeMux, tm *telemetry.ToolMetrics, sqm *telemetry.SummaryQueueMetrics, uqm *telemetry.UsageQueueMetrics, resolve connectResolver, csrfVerify func(owner, token string) bool, reseal resealFunc) (shutdown func(context.Context), err error) {
	d, err := buildDepsFromEnv(sqm, uqm)
	if err != nil {
		return nil, fmt.Errorf("build deps: %w", err)
	}
	if err := d.mountConnect(mux, resolve, csrfVerify, reseal); err != nil {
		return nil, fmt.Errorf("mount connect: %w", err)
	}

	s.AddReceivingMiddleware(instrumentTools(tm.Record))

	mcp.AddTool(s, &mcp.Tool{Name: "store_memory", Description: "Persist a deliberate, well-formed memory. Do NOT store transient state, secrets, or timestamps. Optionally pass `summary`: a one-line recall summary shown in place of content (keep negations/identifiers verbatim). Optionally pass `idempotency_key` for safe retries: same key + identical content returns the original id/short_id unchanged; same key + different content is rejected; omit for a fresh record every time. The result includes the memory's id and short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.storeMemory(ctx, c, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "schedule_memory", Description: "Persist a memory with a validity window (not_before defers recall; not_after expires it). At least one bound (RFC3339) is required; use store_memory for unscheduled records. Optionally pass `idempotency_key` for safe retries: same key + identical content returns the original id/short_id unchanged (the schedule window is NOT part of the replay check); same key + different content is rejected; omit for a fresh record every time. The result includes the memory's id and short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scheduleArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.scheduleMemory(ctx, c, a)
			return textResult(fmt.Sprintf("scheduled %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_memory", Description: "Semantic search within a scope. Optionally pass `tags` to restrict to records carrying all listed tags (AND) before ranking. Returns compact summaries by default (id, summary, summary_source, scope, category, tags, created_at); pass `full=true` for full content, or fetch one record in full via get_memory. Each result carries a `score`: the raw Qdrant cosine similarity for this query (higher = closer), present when non-zero; unranked list_memory/get_memory results have a zero/omitted score."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			// MCP lane default (round-4 finding-7 discipline): the core applies
			// no internal k default, so this closure supplies MCP's 8 before
			// calling deps.searchMemory (Connect supplies 20 in 17-04).
			k := a.K
			if k == 0 {
				k = 8
			}
			after, err := parseRFC3339(a.CreatedAfter)
			if err != nil {
				return nil, nil, fmt.Errorf("created_after: %w", err)
			}
			before, err := parseRFC3339(a.CreatedBefore)
			if err != nil {
				return nil, nil, fmt.Errorf("created_before: %w", err)
			}
			ms, err := d.searchMemory(ctx, c, coreSearchRequest{
				Scope: a.Scope, Query: a.Query, K: k, Tags: a.Tags,
				CreatedAfter: after, CreatedBefore: before,
			})
			if err != nil {
				return nil, nil, err
			}
			// MCP-specific recall shaping lives here, not in the shared core
			// (D-07): the core returns raw []store.Memory.
			hits := shapeRecall(ms, a.Full, d.summaryMaxChars)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"memories": hits}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_memory", Description: "List memories in a scope without a query. Most-recent first. Optional `created_after`/`created_before` (RFC3339) window and `cursor` for paging (use the returned next_cursor). Optional `tags` (AND). Returns {memories, next_cursor}; compact summaries by default, `full=true` for full content."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			// MCP lane default (round-4 finding-7 discipline): the core applies
			// no internal Limit default (Connect leaves 0 = "all").
			limit := a.Limit
			if limit == 0 {
				limit = 20
			}
			after, err := parseRFC3339(a.CreatedAfter)
			if err != nil {
				return nil, nil, fmt.Errorf("created_after: %w", err)
			}
			before, err := parseRFC3339(a.CreatedBefore)
			if err != nil {
				return nil, nil, fmt.Errorf("created_before: %w", err)
			}
			res, err := d.listMemory(ctx, c, coreListRequest{
				Scope: a.Scope, Limit: limit, Tags: a.Tags,
				CreatedAfter: after, CreatedBefore: before,
				Cursor: a.Cursor,
				// Preserves today's UNCONDITIONAL MCP cursor-mode pagination
				// (round-3 HIGH-2): the neutral core does NOT hardcode this, so
				// without an explicit true here the tokenless first page would
				// silently stop cursoring (offset mode leaves next_cursor empty,
				// store.go:817).
				CursorMode: true,
			})
			if err != nil {
				return nil, nil, err
			}
			// MCP-specific recall shaping lives here, not in the shared core
			// (D-07): the core returns raw []store.Memory.
			mems := shapeRecall(res.Memories, a.Full, d.summaryMaxChars)
			return textResult(fmt.Sprintf("%d memories", len(mems))), map[string]any{"memories": mems, "next_cursor": res.NextToken}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_scheduled", Description: "List your windowed memories the recall gate is hiding: state=scheduled (not yet active, default) | expired | all. Active memories surface via list_memory/search_memory."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listScheduledArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			mems, err := d.listScheduled(ctx, c, a)
			return textResult(fmt.Sprintf("%d scheduled", len(mems))), map[string]any{"memories": mems}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_memory", Description: "Fetch one memory by id. Unlike search_memory/list_memory, fetch-by-id is NOT recall-gated: it returns scheduled (not-yet-active) and expired records too. The id may be the full UUID or the short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			m, err := d.getMemory(ctx, c, a)
			return textResult(m.Content), m, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_memory", Description: "Replace a memory's content in place (re-embeds). Optionally set `shared` to toggle visibility (true=shared, false=private); omit to keep current visibility. Optionally set `tags` to replace the full tag set (empty array clears); omit to keep current tags. Optionally set `summary` to replace the recall summary (empty string clears); omit to keep current. If you change content while a caller-authored summary exists, you must address the summary (re-send, update, or clear) or the update is rejected. The id may be the full UUID or the short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			// MCP unconditionally replaces content (the wire contract requires
			// it on every call); the mutationResult isn't surfaced here — MCP's
			// wire shape hasn't changed (Connect's response uses it, 17-03/17-04).
			_, err = d.updateMemory(ctx, c, a)
			return textResult("updated"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_memory", Description: "Delete one memory by id. The id may be the full UUID or the short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			err = d.deleteMemory(ctx, c, a)
			return textResult("deleted"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_all", Description: "Delete your own memories in a scope (teardown); never another caller's records."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scopeArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			err = d.st.DeleteAll(ctx, a.Scope, c.Subj)
			return textResult("scope cleared"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "store_discovery", Description: "Cache agent-earned codebase understanding with citations. kind=map|fact; >=1 citation; scope discovery:repo:<repo>. The result includes the discovery's id and short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.storeDiscovery(ctx, c, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_discovery", Description: "Semantic search over the discovery pool. scope required unless cross_spine=true; optional kind=map|fact. Results carry citations + created_at (aging signals)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			hits, err := d.searchDiscovery(ctx, c, a)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"discoveries": hits}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "set_visibility", Description: "Share or unshare a memory you own. shared=true → readable by any authenticated caller (never writable by others); false → private. The id may be the full UUID or the short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a setVisibilityArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			_, err = d.setVisibility(ctx, c, a)
			return textResult("visibility updated"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "supersede_memory", Description: "Correct a memory you own by superseding it: stores a new record and marks the target superseded_by the new one. The target is soft-hidden from search_memory/list_memory but remains fetchable via get_memory — history is preserved, nothing is deleted or overwritten. Rejects if the target is already superseded (single live head per chain). The target id may be the full UUID or short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a supersedeArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.supersedeMemory(ctx, c, a)
			return textResult(fmt.Sprintf("stored %s, superseding %s", id, a.Supersedes)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "store_rule", Description: "Persist a NORMATIVE rule (ground truth) for a repo/project. Call ONLY on explicit user instruction — never promote a rule unilaterally; propose it to the user instead. scope=rule:repo:<repo> or rule:project:<project>. summary is REQUIRED and is the one-line index entry (single line). Rules are always shared and user-blessed. The result includes the rule's id and short_id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeRuleArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.storeRule(ctx, c, a)
			return textResult(fmt.Sprintf("stored rule %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_rules", Description: "List the COMPLETE rule set for one or more rule:* scopes, oldest-first. Compact index shape by default (short_id, summary, tags); full=true adds content. Optional tags filter (AND). Rules are the repo/project's normative ground truth."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listRulesArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			rules, advisory, err := d.listRules(ctx, c, a)
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
