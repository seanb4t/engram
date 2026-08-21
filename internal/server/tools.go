// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package server registers and serves the engram memory MCP tools.
package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
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
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/summarize"
	"github.com/seanb4t/engram/internal/surfaces"
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
	// maxSummaryBytes bounds storeArgs.Summary / updateArgs.Summary
	// (ENGRAM_MEMORY_MAX_SUMMARY_BYTES, D-06a/D-18): the Go-level check that
	// makes issue #360's repro attributable, checked BEFORE content presence
	// in validateStoreArgs/validateUpdateArgs. 0 disables the bound (operator
	// escape hatch, mirroring embedTimeout/summaryTimeout's "0 = infinite"
	// convention) — this is DIFFERENT from a zero-value &deps{} test literal
	// that never called buildDepsFromEnv, which tests must populate
	// explicitly to exercise the bound.
	maxSummaryBytes int
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
	warnPendingMigrations(st)
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
		maxSummaryBytes:  maxMemorySummaryBytes(cfg),
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

// maxMemorySummaryBytes parses ENGRAM_MEMORY_MAX_SUMMARY_BYTES (D-06a/D-18),
// defaulting to 512 on empty/invalid. Unlike summaryMaxChars/summaryMaxTokens,
// 0 is a legitimate PARSED value (config.Validate already guarantees a
// non-negative integer) and is honored as "bound disabled" — it is NOT
// coerced to the default, mirroring embedTimeout's/summaryTimeout's own
// "0 = escape hatch" convention. Only an unparseable value falls back.
func maxMemorySummaryBytes(cfg *config.Config) int {
	n, err := strconv.Atoi(cfg.Memory.MaxSummaryBytes)
	if err != nil || n < 0 {
		if cfg.Memory.MaxSummaryBytes != "" {
			slog.Warn("ENGRAM_MEMORY_MAX_SUMMARY_BYTES is set but unparseable or negative; using default 512",
				"value", cfg.Memory.MaxSummaryBytes)
		}
		return 512
	}
	return n
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
	opts := []embed.Option{
		embed.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
		embed.WithQueryInstruction(cfg.Embed.QueryInstruction),
		embed.WithDocumentInstruction(cfg.Embed.DocumentInstruction),
		embed.WithQueryParams(queryParams),
		embed.WithDocumentParams(documentParams),
		embed.WithTimeout(embedTimeout(cfg)),
		embed.WithEmbeddingsURL(cfg.OpenAI.EmbeddingsURL),
	}
	// D-16 (plan 04-03's option, wired here): size the success-path decode
	// bound to the configured dimension rather than copying the chat lane's
	// blind 1 MiB (summarize.go:186-188) — an embeddings response is larger
	// than a one-line summary. 64 bytes/element gives roughly 4.5x headroom
	// over the ~12-14 bytes an element costs as JSON text, floored at 64 KiB
	// so a small dim still gets a sane bound. config.Validate already
	// guarantees ENGRAM_EMBED_DIM parses to a positive integer on the normal
	// startup path, but this function is also called directly with an
	// unvalidated cfg (embed_wiring_test.go); on a parse failure this simply
	// omits the option so embed.New's own defaultMaxResponseBytes applies —
	// never a zero bound.
	if dim, derr := strconv.ParseUint(cfg.Embed.Dim, 10, 64); derr == nil && dim > 0 {
		const perElementBytes = 64
		const floorBytes = 64 * 1024
		bound := dim * perElementBytes
		if bound < floorBytes {
			bound = floorBytes
		}
		opts = append(opts, embed.WithMaxResponseBytes(int64(bound)))
	}
	return embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Embed.Model, opts...), nil
}

// summarizerFromConfig builds the chat-completions summarizer from config.
func summarizerFromConfig(cfg *config.Config) *summarize.Client {
	// cmp.Or resolved once here (D-12): ChatBaseURL/ChatAPIKey win when set,
	// otherwise the shared BaseURL/APIKey. The embedder below is deliberately
	// left untouched — it always uses BaseURL/APIKey regardless of
	// ChatBaseURL/ChatAPIKey.
	chatBaseURL := cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)
	chatAPIKey := cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)
	return summarize.New(chatBaseURL, chatAPIKey, cfg.Summarize.Model, summaryMaxChars(cfg),
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

// warnPendingMigrations is warnOwnerlessRecords' non-fatal, synchronous,
// 10-second-bounded sibling for schema migrations (REQ-migrate-never-
// automatic, REVIEWS.md C5-L5): it runs INLINE in buildDepsFromEnv, exactly
// as warnOwnerlessRecords does, so startup is DELAYED by at most 10 seconds
// and is never PREVENTED — an error, a timeout, or an unreachable Qdrant
// logs and returns. A goroutine was considered and REJECTED for two
// reasons worth recording: it would race the server's first request, and a
// merely-slow Qdrant would then emit the warning after an operator had
// already stopped reading the startup log — which is the same as not
// warning at all. REQ-migrate-never-automatic's "non-blocking" therefore
// means "never gates startup", not "asynchronous"; this comment is where a
// reader learns which. It MUST NOT invoke Store.Migrate under any
// circumstance — the ONLY automatic surface this phase allows is the
// warning itself.
func warnPendingMigrations(st *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := st.MigrateStatus(ctx)
	if err != nil {
		slog.Warn("could not check for pending schema migrations", "err", err)
		return
	}

	// H3 CORRECTED predicate: pending = Absent + sum(bucket.Count where
	// bucket.Version < CurrentVersion) — NOT sum(all buckets)+Absent, which
	// is the total collection count and would spuriously warn for every
	// non-empty all-current collection. status.Future is deliberately
	// excluded: those records are AHEAD, not behind.
	pending := status.Absent
	for _, b := range status.Buckets {
		if b.Version < int(migrate.CurrentVersion) {
			pending += b.Count
		}
	}
	if pending > 0 {
		slog.Warn("pending schema migrations exist; run: engram migrate",
			"pending", pending, "current_version", migrate.CurrentVersion)
	}

	// M4: a SEPARATE compatibility warning naming the DISTINCT future
	// versions, never just a total — an operator reading a bare total
	// cannot tell one-version-ahead drift from a wildly newer binary.
	if len(status.Future) > 0 {
		futureVersions := make([]int, len(status.Future))
		for i, b := range status.Future {
			futureVersions[i] = b.Version
		}
		slog.Warn("records at a future schema version exist — this binary may be too old for some records",
			"future_versions", futureVersions, "future_total", status.FutureTotal, "server_version", migrate.CurrentVersion)
	}
}

type storeArgs struct {
	// Content/Scope/Source/Category carry omitempty (D-06a): required-ness for
	// all four moved out of the go-sdk's inferred JSON schema and into
	// validateStoreArgs, which attributes the correct field on rejection —
	// this is issue #360's actual fix. Do NOT read the absence of "required"
	// text here as these fields becoming optional; they are exactly as
	// required as before, just enforced one layer down.
	Content string `json:"content,omitempty" jsonschema:"the memory text to persist"`
	Scope   string `json:"scope,omitempty" jsonschema:"run:tier:repo, e.g. eval-2026-05:project:selfhosted-cluster"`
	Source  string `json:"source,omitempty" jsonschema:"user-said or agent-inferred"`
	// Category's tag deliberately does NOT state the discovery-not-schedulable
	// rule: this field is promoted, via Go embedding, onto scheduleArgs and
	// supersedeArgs too, so a sentence stated here would reach
	// store_memory's and supersede_memory's OWN generated schemas — neither
	// enforces it (only parseWindow, schedule_memory's handler, does). The
	// rule composes instead onto scheduleArgs.NotBefore/NotAfter below,
	// fields exclusive to the one struct/tool that actually raises it
	// (WR-01).
	Category  string   `json:"category,omitempty" jsonschema:"decision|preference|convention|gotcha"`
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
	// Citations is promoted onto scheduleArgs and supersedeArgs via the same
	// Go field embedding as IdempotencyKey above (D-04) — one declaration
	// here, store_memory/schedule_memory/supersede_memory all inherit it; do
	// NOT declare it separately on either embedding struct. Optional on every
	// category (unlike store_discovery's required minimum): never inferred,
	// only what the caller explicitly supplies (Phase 26, D-01/D-03).
	Citations []citationArg `json:"citations,omitempty" jsonschema:"optional source anchors for this memory; never inferred, only what the caller explicitly supplies"`
}

// scheduleArgs embeds storeArgs and adds the temporal window. The anonymous
// embed flattens identically on both the json-decode and reflected-schema paths
// (Go field promotion), so the schedule_memory wire contract is byte-for-byte the
// store_memory fields plus not_before/not_after.
type scheduleArgs struct {
	storeArgs
	// NotBefore/NotAfter's tags each state both window rules (D-03): at
	// least one bound is required, and when both are set, not_before must
	// be strictly before not_after. Neither tag states the third parseWindow
	// rejection ("not_after must be in the future") — that is D-02's one
	// explicit registry exclusion (single-field, clock-dependent), pinned by
	// the doc comment at parseWindow's not-in-future check. Both tags also
	// state discovery-not-schedulable (WR-01): NotBefore/NotAfter are
	// declared directly on scheduleArgs, not promoted via embedding, so this
	// is the one place the rule can compose onto the jsonschema-tag surface
	// without also reaching store_memory's/supersede_memory's schemas (see
	// storeArgs.Category's tag comment above).
	NotBefore string `json:"not_before,omitempty" jsonschema:"RFC3339; hide from recall until this time; requires not_before and/or not_after; not_before must be strictly before not_after; discovery is not schedulable; use store_discovery"`
	NotAfter  string `json:"not_after,omitempty" jsonschema:"RFC3339; drop from recall at this time; requires not_before and/or not_after; not_before must be strictly before not_after; discovery is not schedulable; use store_discovery"`
}

// parseWindow validates and parses the schedule_memory temporal window. At least
// one bound is required; not_after must be in the future and after not_before.
// Every rejection is wrapped with the existing store.ErrInvalidArgument (review
// finding 5) so a Connect ScheduleMemory call maps to CodeInvalidArgument, not
// CodeInternal, once 17-04 wires the connectError mapper.
func parseWindow(a scheduleArgs, now time.Time) (nb, na *time.Time, err error) {
	if a.NotBefore == "" && a.NotAfter == "" {
		rule, _ := surfaces.RuleByID(surfaces.RuleScheduleWindowAtLeastOne)
		return nil, nil, conditionalErrf(classMalformed, rule)
	}
	if a.Category == "discovery" {
		rule, _ := surfaces.RuleByID(surfaces.RuleDiscoveryNotSchedulable)
		return nil, nil, conditionalErrf(classPrecondition, rule)
	}
	if a.NotBefore != "" {
		t, perr := time.Parse(time.RFC3339, a.NotBefore)
		if perr != nil {
			return nil, nil, argErrf(classMalformed, HintFormat, "not_before", "not_before must be RFC3339")
		}
		nb = &t
	}
	if a.NotAfter != "" {
		t, perr := time.Parse(time.RFC3339, a.NotAfter)
		if perr != nil {
			return nil, nil, argErrf(classMalformed, HintFormat, "not_after", "not_after must be RFC3339")
		}
		if !t.After(now) {
			// D-02's ONE explicit exclusion from the conditional-rule registry:
			// single-field and clock-dependent (compares against time.Now(),
			// not a second caller-supplied field), so it is a wall-clock state
			// constraint rather than an interface-legible one. Do NOT declare a
			// rule for this site and do NOT convert it — this is the deliberate,
			// documented carve-out internal/server/conditionalsweep_test.go
			// pins by name (conformanceExcludedSites).
			return nil, nil, argErrf(classOutOfRange, HintOrdering, "not_after", "not_after must be in the future")
		}
		na = &t
	}
	if nb != nil && na != nil && !nb.Before(*na) {
		rule, _ := surfaces.RuleByID(surfaces.RuleWindowOrdering)
		return nil, nil, conditionalErrf(classPrecondition, rule)
	}
	return nb, na, nil
}

// supersedeArgs embeds storeArgs (mirroring scheduleArgs's embedding shape,
// D-03/RESEARCH A4) so supersede_memory inherits the full store_memory field
// set (content/scope/source/category/tags/repo/workspace/worktree/base_dir/
// summary) without a hand-rolled parallel field list — the exact drift class
// persistAndEnqueue's doc comment already flags (tools.go:734-736).
//
// idempotency_key IS supported here (D-12, phase 03.1 plan 04): this closes
// Phase 25's deliberately deferred scope (WR-03/WR-04, plan T-25-10 — the
// feature was never argued against, only deferred). The promoted
// storeArgs.IdempotencyKey field is now the sole schema AND decode target:
// a caller-supplied idempotency_key is advertised on the generated schema,
// decoded, and READ by deps.supersedeMemory via
// checkIdempotentMergeReplay/resolveLostMergeRace, whose replay fingerprint
// (mergeFingerprint) covers content AND the resolved target set — so a
// same-key retry with a DIFFERENT target set is a conflict, never a silent
// different-operation success (D-13). The replay check runs BEFORE the
// already-superseded stage (D-14/PD-09) — see checkIdempotentMergeReplay's
// doc comment for the ordering requirement and PD-09's residual narrowing.
type supersedeArgs struct {
	storeArgs
	// Supersedes carries omitempty (D-06a); its non-empty-slice presence
	// check lives in deps.supersedeMemory, run before ResolvePointID. Always
	// an array on the wire (D-01/D-03, phase 03.1 promote — a one-element
	// array behaves exactly as the pre-phase single-target call). No maximum
	// length is enforced or advertised (PD-07, 03.1-00-SUMMARY.md).
	Supersedes []string `json:"supersedes,omitempty" jsonschema:"non-empty array of ids (full UUID or short_id) of the memories this new record corrects/replaces"`
}

type searchArgs struct {
	// Query carries omitempty (D-06a); its presence check runs in
	// deps.searchMemory, first, before the embed call.
	Query         string   `json:"query,omitempty"`
	Scope         string   `json:"scope,omitempty" jsonschema:"required unless cross_spine"`
	K             uint64   `json:"k,omitempty"`
	Tags          []string `json:"tags,omitempty" jsonschema:"optional; restrict to records carrying ALL listed tags"`
	Categories    []string `json:"categories,omitempty" jsonschema:"optional; restrict to records in ANY of the listed categories (OR) — unlike tags, which requires ALL"`
	Full          bool     `json:"full,omitempty" jsonschema:"return full content instead of summaries (default false → compact summary view)"`
	CreatedAfter  string   `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string   `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
	CrossSpine    bool     `json:"cross_spine,omitempty" jsonschema:"span all readable scopes (ignores scope)"`
}

type listArgs struct {
	Scope         string   `json:"scope,omitempty" jsonschema:"required unless cross_spine"`
	Limit         uint64   `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
	Tags          []string `json:"tags,omitempty" jsonschema:"optional; restrict to records carrying ALL listed tags"`
	Categories    []string `json:"categories,omitempty" jsonschema:"optional; restrict to records in ANY of the listed categories (OR) — unlike tags, which requires ALL"`
	Full          bool     `json:"full,omitempty" jsonschema:"return full content instead of summaries (default false → compact summary view)"`
	CreatedAfter  string   `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string   `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
	Cursor        string   `json:"cursor,omitempty" jsonschema:"opaque pagination cursor from a prior next_cursor; omit for the first page"`
	CrossSpine    bool     `json:"cross_spine,omitempty" jsonschema:"span all readable scopes (ignores scope)"`
}

type listScheduledArgs struct {
	// Scope carries omitempty (D-06a); its presence check runs in
	// deps.listScheduled, first.
	Scope         string `json:"scope,omitempty" jsonschema:"the scope to list scheduled/expired memories from"`
	State         string `json:"state,omitempty" jsonschema:"scheduled (default, not yet active) | expired | all"`
	Limit         uint64 `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
	CreatedAfter  string `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
}

type idArgs struct {
	// ID carries omitempty (D-06a); its presence check (requireID) runs in
	// every deps.* method that resolves it (getMemory/deleteMemory), first.
	ID string `json:"id,omitempty" jsonschema:"the memory's full UUID or its short_id"`
}

type updateArgs struct {
	// ID carries omitempty (D-06a); requireID runs in deps.updateMemory,
	// first — symmetric across both lanes (Connect always supplies an id).
	ID string `json:"id,omitempty" jsonschema:"the memory's full UUID or its short_id"`
	// Content is presence-signaled (nil = unchanged) so deps.updateMemory can
	// route a shared/summary-only change to the payload-only store method
	// (no re-embed, vector preserved) instead of unconditionally re-embedding
	// and blanking content on a nil value (landmine 2). Content now carries
	// omitempty too (D-06a): the schema no longer enforces its
	// requiredness. validateUpdateArgs re-enforces it in Go, but is called
	// ONLY from the update_memory MCP closure (tools.go, Register) — NEVER
	// from this shared deps.updateMemory — because the Connect protoconv
	// field-mask lane (protoconv.go:updateMemoryRequestToArgs) legitimately
	// supplies a nil Content for an update that omits "content". Putting the
	// check here would break that lane's field-mask semantics.
	Content *string   `json:"content,omitempty"`
	Shared  *bool     `json:"shared,omitempty" jsonschema:"omit to keep current visibility; true=shared, false=private"`
	Tags    *[]string `json:"tags,omitempty" jsonschema:"omit to keep current tags; supply to replace the full set (empty array clears)"`
	Summary *string   `json:"summary,omitempty" jsonschema:"omit to keep current summary; supply to replace (empty string clears). If content changes and a caller-authored summary exists, you MUST address it (re-send to keep, update, or clear) or the update is rejected"`
}

// scopeArgs backs delete_all — a destructive teardown (D-19/T-04-17). Scope
// carries omitempty (D-06a), which moves the ONLY guard between an omitted
// scope and that teardown out of the wire schema — deps.deleteAll's presence
// check is now the SOLE remaining guard, and it MUST run before every side
// effect in that path (see deps.deleteAll below). This is the
// highest-consequence row in the whole D-06a delta; do not relax the tag
// without deps.deleteAll's check landing in the SAME commit.
type scopeArgs struct {
	Scope string `json:"scope,omitempty"`
}

type citationArg struct {
	// Kind/Ref already carry a presence-equivalent check inside
	// validateCitations (Kind via validCitationKind's enum rejection, which
	// rejects "" as not in {file,commit,url,repo}; Ref via an explicit
	// Ref=="" check) — omitempty here is a schema-only change, no new Go
	// logic needed.
	Kind    string `json:"kind,omitempty" jsonschema:"file|commit|url|repo"`
	Ref     string `json:"ref,omitempty" jsonschema:"path, repo URL, or doc URL"`
	Locator string `json:"locator,omitempty" jsonschema:"e.g. 200-240 line range"`
	Pin     string `json:"pin,omitempty" jsonschema:"commit SHA, content-hash, @rev, or fetched-at"`
	Excerpt string `json:"excerpt,omitempty" jsonschema:"cached substance (<= ~50 lines)"`
}

type storeDiscoveryArgs struct {
	// Content/Kind/Citations/Scope carry omitempty (D-06a). All four already
	// had a Go-level presence-equivalent check in validateStoreDiscovery
	// before this plan (Content=="" reject, Kind enum reject, Scope=="" plus
	// prefix reject, Citations via validateCitations' minCount=1 length
	// check) — those checks were simply SHADOWED by the schema's own
	// required-ness; relaxing the tag makes them reachable for the first
	// time. No new validation logic needed here.
	Content   string        `json:"content,omitempty" jsonschema:"the understanding to cache (embedded + searched)"`
	Kind      string        `json:"kind,omitempty" jsonschema:"map (orientation) or fact (pinned checkable claim)"`
	Citations []citationArg `json:"citations,omitempty" jsonschema:"at least one source anchor"`
	Scope     string        `json:"scope,omitempty" jsonschema:"discovery scope, must start with discovery: (e.g. discovery:repo:<repo>)"`
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

// maxSupersedeTargetBytes bounds each INDIVIDUAL entry of supersedeArgs's
// `supersedes` (WR-01, 03.1 cycle-4 review): a real UUID or short_id never
// exceeds a small, fixed length, so 256 bytes is generous enough to cost
// nothing legitimate. This is a per-ENTRY length bound only — PD-07
// (03.1-00-SUMMARY.md) deliberately declined a cap on the NUMBER of targets
// in the set, and this constant does not revisit that ruling. Without it, an
// unresolvable entry is echoed back verbatim (comma-joined with every other
// offender) by renderTargetRejection, so an unbounded-length entry becomes
// an unbounded-length rejection — the one client-supplied string in this
// verb's blast radius that lacked the size-bound discipline every sibling
// field (IdempotencyKey, Summary, citation Excerpt, discovery Content)
// already has.
const maxSupersedeTargetBytes = 256

type searchDiscoveryArgs struct {
	// Query carries omitempty (D-06a); its presence check runs in
	// deps.searchDiscovery, first, before effectiveDiscoveryScope/embed.
	Query      string `json:"query,omitempty"`
	Scope      string `json:"scope,omitempty" jsonschema:"required unless cross_spine"`
	Kind       string `json:"kind,omitempty" jsonschema:"map|fact filter"`
	K          uint64 `json:"k,omitempty"`
	CrossSpine bool   `json:"cross_spine,omitempty" jsonschema:"span all discovery scopes (ignores scope)"`
}

// setVisibilityArgs carries omitempty on both fields (D-06a). Shared is a
// *bool (not a plain bool), the ONE type change this plan makes beyond the
// tag relaxation: a plain bool with omitempty would make `shared:false` — a
// legitimate, meaningful call — silently indistinguishable from omission,
// which is a behavior change, not a relaxation. deps.setVisibility rejects a
// nil Shared with the presence check; requireID covers ID the same way as
// idArgs/updateArgs.
type setVisibilityArgs struct {
	ID     string `json:"id,omitempty" jsonschema:"the memory's full UUID or its short_id"`
	Shared *bool  `json:"shared,omitempty" jsonschema:"true = readable by any authenticated caller; false = private"`
}

// requireID rejects an absent id/short_id with the field-attributed envelope
// (D-06a). Shared by every deps.* method that resolves idArgs.ID/
// updateArgs.ID/setVisibilityArgs.ID via ResolvePointID — that store-level
// call already rejects an empty id (store.go:1513-1516), but with a bare
// store.ErrInvalidArgument carrying no field attribution. This runs FIRST so
// the caller learns which field, not just that resolution failed. Safe to
// call from a shared MCP+Connect deps method: "id" is required on both
// lanes symmetrically (unlike updateArgs.Content, which is not).
func requireID(id string) error {
	if id == "" {
		return argErrf(classMalformed, HintRequired, "id", "id is required")
	}
	return nil
}

// validateStoreArgs enforces storeArgs' presence requirements in Go (D-06a):
// content/scope/source/category, which carry no schema-level "required"
// after this plan's omitempty relaxation — this validator IS engram's
// replacement enforcement, not a supplement to a schema check that still
// exists. Shared by store_memory/schedule_memory/supersede_memory (all three
// embed storeArgs) on both the MCP and Connect lanes: Connect's create-style
// write RPCs (StoreMemory/ScheduleMemory) have no field-mask semantics, so
// there is no asymmetric-nil case here the way there is for updateArgs.Content.
//
// The summary-length bound (maxSummaryBytes, ENGRAM_MEMORY_MAX_SUMMARY_BYTES,
// D-18) is checked FIRST, before content presence — this order is what makes
// issue #360's regression deterministic: #360's failure mode is precisely an
// oversized `summary` producing a false content-absent reading, so evaluating
// the summary constraint first means the caller is told the REAL cause even
// in the case where the underlying decode anomaly also drops `content`.
// maxSummaryBytes<=0 means the bound is disabled (D-18's "0 is honored as
// disabled" convention).
func validateStoreArgs(a storeArgs, maxSummaryBytes int) error {
	if maxSummaryBytes > 0 && len(a.Summary) > maxSummaryBytes {
		return argErrf(classOutOfRange, HintTooLong, "summary", "summary too large: %d bytes (max %d)", len(a.Summary), maxSummaryBytes)
	}
	if a.Content == "" {
		return argErrf(classMalformed, HintRequired, "content", "content is required")
	}
	if a.Scope == "" {
		return argErrf(classMalformed, HintRequired, "scope", "scope is required")
	}
	if a.Source == "" {
		return argErrf(classMalformed, HintRequired, "source", "source is required")
	}
	if a.Category == "" {
		return argErrf(classMalformed, HintRequired, "category", "category is required")
	}
	return nil
}

// validateUpdateArgs enforces updateArgs' MCP-lane requirements in Go
// (D-06a): a non-nil Content, plus the same maxSummaryBytes bound
// validateStoreArgs enforces (D-18), ordered before the content check for
// the identical #360 reason. Called ONLY from the update_memory MCP closure
// (Register, below) — NEVER from deps.updateMemory, which the Connect
// protoconv field-mask lane legitimately calls with a nil Content for an
// update that omits "content" (see updateArgs.Content's doc comment).
// requireID's ID check is NOT folded in here: unlike Content, ID is required
// symmetrically on both lanes, so it lives in deps.updateMemory instead,
// where a single check covers MCP and Connect alike.
func validateUpdateArgs(a updateArgs, maxSummaryBytes int) error {
	if maxSummaryBytes > 0 && a.Summary != nil && len(*a.Summary) > maxSummaryBytes {
		return argErrf(classOutOfRange, HintTooLong, "summary", "summary too large: %d bytes (max %d)", len(*a.Summary), maxSummaryBytes)
	}
	if a.Content == nil {
		return argErrf(classMalformed, HintRequired, "content", "content is required")
	}
	return nil
}

func validateStoreDiscovery(a storeDiscoveryArgs) error {
	if a.Content == "" {
		return argErrf(classMalformed, HintRequired, "content", "content is required")
	}
	if len(a.Content) > maxDiscoveryContentBytes {
		return argErrf(classOutOfRange, HintTooLong, "content", "content too large: %d bytes (max %d)", len(a.Content), maxDiscoveryContentBytes)
	}
	if a.Kind != "map" && a.Kind != "fact" {
		return argErrf(classMalformed, HintEnum, "kind", "kind must be \"map\" or \"fact\"")
	}
	if a.Scope == "" {
		return argErrf(classMalformed, HintRequired, "scope", "scope is required")
	}
	if !strings.HasPrefix(a.Scope, "discovery:") {
		return argErrf(classMalformed, HintPrefix, "scope", "scope must be a discovery scope (start with \"discovery:\")")
	}
	return validateCitations(a.Citations, 1)
}

// validateCitations enforces the shared citation contract (D-05): at least
// minCount entries, no more than maxDiscoveryCitations total, and per entry a
// kind accepted by validCitationKind, a non-empty ref, and an excerpt no
// larger than maxCitationExcerptBytes. store_discovery calls this with
// minCount 1 (unchanged behavior — extracted verbatim from
// validateStoreDiscovery's former inline loop); the memory write handlers
// (store_memory/schedule_memory/supersede_memory) call it with minCount 0,
// since citations are optional provenance on curated categories.
func validateCitations(cites []citationArg, minCount int) error {
	if len(cites) < minCount {
		if minCount == 1 {
			return argErrf(classMalformed, HintRequired, "citations", "at least one citation is required")
		}
		return argErrf(classMalformed, HintRequired, "citations", "at least %d citation(s) required", minCount)
	}
	if len(cites) > maxDiscoveryCitations {
		return argErrf(classOutOfRange, HintTooMany, "citations", "too many citations: %d (max %d)", len(cites), maxDiscoveryCitations)
	}
	for i, c := range cites {
		if !validCitationKind(c.Kind) {
			return argErrf(classMalformed, HintEnum, "citations[i].kind", "citation %d: kind must be one of file|commit|url|repo", i)
		}
		if c.Ref == "" {
			return argErrf(classMalformed, HintRequired, "citations[i].ref", "citation %d: ref is required (the source anchor)", i)
		}
		if len(c.Excerpt) > maxCitationExcerptBytes {
			return argErrf(classOutOfRange, HintTooLong, "citations[i].excerpt", "citation %d: excerpt too large: %d bytes (max %d)", i, len(c.Excerpt), maxCitationExcerptBytes)
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
	// Citations: nil/empty input leaves the field nil rather than an
	// allocated empty slice, keeping payload()'s len(m.Citations) > 0 gate
	// meaningful (D-01) — an allocated-but-empty slice would still satisfy
	// len()==0 there, so this guard is belt-and-suspenders, not load-bearing,
	// but it matches storeDiscovery's mapping loop shape exactly (D-05).
	var cites []store.Citation
	if len(a.Citations) > 0 {
		cites = make([]store.Citation, len(a.Citations))
		for i, cit := range a.Citations {
			cites[i] = store.Citation{Kind: cit.Kind, Ref: cit.Ref, Locator: cit.Locator, Pin: cit.Pin, Excerpt: cit.Excerpt}
		}
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
		Citations:     cites,
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
		return false, "", "", "", argErrf(classOutOfRange, HintTooLong, "idempotency_key", "idempotency_key too large: %d bytes (max %d)", len(a.IdempotencyKey), maxIdempotencyKeyBytes)
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

// checkIdempotentMergeReplay is supersede_memory's idempotency replay check
// (D-12, closing Phase 25's deliberately deferred scope WR-03/WR-04). It
// copies checkIdempotentReplay's control flow exactly — the same-key-
// different-content detection above is the mechanism being extended, not
// replaced — but compares against mergeFingerprint(a.storeArgs, targets)
// instead of contentFingerprint(a), so a same-key retry with a DIFFERENT
// target set is a conflict, never a silent different-operation success
// (D-13, T-03.1-07).
//
// Ordering (D-14). This is called from deps.supersedeMemory AFTER
// resolveAndAuthorizeSupersedeTargets (existence + ownership) and BEFORE
// validateSupersedeTargetState (already-superseded) — a structural property
// of the call sequence (two separate statements), not a position inside one
// function body a later edit can quietly move. Ordered the other way, the
// already-superseded stage would reject a retry before the key is ever
// consulted, and that failure mode is invisible in every happy-path test.
//
// PD-09's residual narrowing, stated as contract rather than only a test
// (03.1-00-SUMMARY.md, review MEDIUM): because this check runs AFTER
// existence and ownership rather than at the very top of the handler, a
// retry whose targets were deleted in the meantime, or whose ownership
// changed, does NOT replay — resolveAndAuthorizeSupersedeTargets rejects it
// as not found before this check ever runs. That is weaker than ordinary
// key-alone idempotent-replay semantics. It is a deliberate trade —
// replaying without re-checking ownership would let a key minted while the
// caller owned a record still act after they no longer do — and is
// published in reference/tools.md, not only pinned by
// TestSupersedeMemoryReplayAfterTargetDeletedRejects.
func (d *deps) checkIdempotentMergeReplay(ctx context.Context, owner string, a supersedeArgs, targets []string) (replay bool, id, shortID, pointID string, err error) {
	if a.IdempotencyKey == "" {
		return false, "", "", "", nil
	}
	if len(a.IdempotencyKey) > maxIdempotencyKeyBytes {
		return false, "", "", "", argErrf(classOutOfRange, HintTooLong, "idempotency_key", "idempotency_key too large: %d bytes (max %d)", len(a.IdempotencyKey), maxIdempotencyKeyBytes)
	}
	pointID = idempotencyPointID(owner, a.Scope, a.IdempotencyKey)
	existing, gerr := d.st.Get(ctx, pointID)
	switch {
	case errors.Is(gerr, store.ErrNotFound):
		return false, "", "", pointID, nil
	case gerr != nil:
		return false, "", "", "", gerr
	}
	if mergeFingerprint(a.storeArgs, targets) == existing.IdempotencyFingerprint {
		return true, existing.ID, existing.ShortID, "", nil
	}
	return false, "", "", "", fmt.Errorf("idempotency key %q reused with a different target set or content: %w", a.IdempotencyKey, store.ErrIdempotencyConflict)
}

// resolveLostMergeRace recovers the losing racer's answer from a store-tier
// ErrAlreadySuperseded rejection (T-03.1-26). Store.Supersede's under-lock
// already-superseded check (store.go) runs BEFORE its Upsert, so the loser
// of two simultaneous identical keyed merges never reaches Upsert and never
// reaches the keyed post-write short-id re-read persistAndEnqueue-style code
// performs after a successful write — this recovery is the ONLY path by
// which a call that never wrote can still return the winner's persisted
// answer.
//
// Deliberately narrow: it does not retry and does not write. It turns
// exactly ONE sentinel (ErrAlreadySuperseded) into exactly one replay, and
// only when the caller's OWN deterministic point id (derived from
// owner+scope+key, so cross-owner disclosure is structurally impossible)
// holds a record carrying the caller's OWN merge fingerprint. Anything wider
// would let a key convert an unrelated rejection into a fabricated success.
//
//   - pointID empty (unkeyed call): return supersedeErr unchanged — the
//     unkeyed path has no deterministic id to look up and this recovery
//     must never fire there.
//   - supersedeErr does not wrap store.ErrAlreadySuperseded: return it
//     unchanged. This recovery is scoped to exactly one rejection; every
//     other store error propagates untouched.
//   - the deterministic point is absent: return supersedeErr unchanged.
//     There is no winner to defer to — the targets were superseded by some
//     OTHER call — so the rejection stands.
//   - the point read fails (transport error): return that read error, never
//     masked as a conflict or a success.
//   - stored fingerprint equals mergeFingerprint(a.storeArgs, targets): a
//     record already sits at the caller's own deterministic id carrying the
//     caller's own fingerprint — the operation the caller asked for has
//     already happened. Return its persisted id/short id, nil error.
//   - stored fingerprint differs: a genuine conflict, not a lost race
//     (D-13) — the same key is being reused for a different operation and
//     the targets happen to already be superseded. Return the
//     idempotency-conflict sentinel — NEVER the already-superseded error and
//     never a success — because the caller's key is the stronger signal
//     here: a key reused for a different operation is a client bug the
//     caller can act on, whereas "already superseded" would send them
//     looking at the wrong thing.
func (d *deps) resolveLostMergeRace(ctx context.Context, a supersedeArgs, targets []string, pointID string, supersedeErr error) (id, shortID string, err error) {
	if pointID == "" {
		return "", "", supersedeErr
	}
	if !errors.Is(supersedeErr, store.ErrAlreadySuperseded) {
		return "", "", supersedeErr
	}
	existing, gerr := d.st.Get(ctx, pointID)
	switch {
	case errors.Is(gerr, store.ErrNotFound):
		return "", "", supersedeErr
	case gerr != nil:
		return "", "", gerr
	}
	if mergeFingerprint(a.storeArgs, targets) == existing.IdempotencyFingerprint {
		return existing.ID, existing.ShortID, nil
	}
	return "", "", fmt.Errorf("idempotency key %q reused with a different target set or content: %w", a.IdempotencyKey, store.ErrIdempotencyConflict)
}

func (d *deps) storeMemory(ctx context.Context, c caller, a storeArgs) (string, string, error) {
	if err := validateStoreArgs(a, d.maxSummaryBytes); err != nil {
		return "", "", err
	}
	if err := validateCitations(a.Citations, 0); err != nil {
		return "", "", err
	}
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
	if err := validateStoreArgs(a.storeArgs, d.maxSummaryBytes); err != nil {
		return "", "", err
	}
	// checkIdempotentReplay runs BEFORE parseWindow's future-only validation
	// (WR-02): the window is deliberately excluded from the D-07 content
	// fingerprint precisely so a retry doesn't need to resend a still-valid
	// window. If parseWindow ran first, a delayed retry of an
	// already-successful schedule_memory call with the SAME not_after value
	// could be rejected with ErrInvalidArgument (now no longer in the
	// future) before checkIdempotentReplay ever got a chance to recognize it
	// as a no-op replay. parseWindow only needs to run on the non-replay
	// (create) path below.
	if err := validateCitations(a.Citations, 0); err != nil {
		return "", "", err
	}
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
	// CrossSpine spans every scope the caller may read (D-08). Scope and
	// CrossSpine are passed through RAW from the transport; deps.listMemory is
	// what resolves them via effectiveSearchScope — the same typed-core
	// chokepoint reasoning coreSearchRequest.CrossSpine documents.
	CrossSpine bool
	// IncludeArchived is the phase 07 (D-01/D-02) orthogonal recall-gate
	// opt-in, copied straight into store.ListOptions. Connect and the CLI
	// only (D-03) — the MCP list_memory closure below never sets this, so its
	// coreListRequest{} literal leaves it at zero value and MCP recall is
	// behaviourally unchanged.
	IncludeArchived bool
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
	Categories    []string
	CreatedAfter  time.Time
	CreatedBefore time.Time
	// CrossSpine spans every scope the caller may read (D-08 widens this to
	// list_memory too, in a later plan). Scope and CrossSpine are passed
	// through RAW from the transport; deps.searchMemory is what resolves them
	// via effectiveSearchScope — the typed core is the last chokepoint before
	// the store, so a future call site cannot reach a widened filter by
	// forgetting the guard (D-07's hazard, closed here rather than in
	// internal/store).
	CrossSpine bool
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
	scope, err := effectiveSearchScope(req.Scope, req.CrossSpine)
	if err != nil {
		return coreListResult{}, err
	}
	ms, total, next, err := d.st.List(ctx, scope, c.Subj, store.ListOptions{
		Limit:           req.Limit,
		Offset:          req.Offset,
		Categories:      req.Categories,
		Visibility:      req.Visibility,
		Tags:            req.Tags,
		CreatedAfter:    req.CreatedAfter,
		CreatedBefore:   req.CreatedBefore,
		Cursor:          req.Cursor,
		CursorMode:      req.CursorMode,
		IncludeArchived: req.IncludeArchived,
	})
	if err != nil {
		return coreListResult{}, err
	}
	return coreListResult{Memories: ms, Total: total, NextToken: next}, nil
}

func (d *deps) listScheduled(ctx context.Context, c caller, a listScheduledArgs) ([]store.Memory, error) {
	// D-06a: Scope carries omitempty now; this is the sole remaining guard
	// (list_scheduled has no Connect RPC — MCP-only).
	if a.Scope == "" {
		return nil, argErrf(classMalformed, HintRequired, "scope", "scope is required")
	}
	if a.Limit == 0 {
		a.Limit = 20
	}
	after, err := parseRFC3339(a.CreatedAfter)
	if err != nil {
		return nil, argErrf(classMalformed, HintFormat, "created_after", "created_after must be RFC3339")
	}
	before, err := parseRFC3339(a.CreatedBefore)
	if err != nil {
		return nil, argErrf(classMalformed, HintFormat, "created_before", "created_before must be RFC3339")
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
		return nil, argErrf(classMalformed, HintEnum, "state", "state must be one of scheduled|expired|all")
	}
	return d.st.ListScheduled(ctx, a.Scope, c.Subj, state,
		store.ListOptions{Limit: a.Limit, CreatedAfter: after, CreatedBefore: before})
}

// searchMemory runs the shared rerank search on the transport-neutral typed
// core contract (D-07): raw []store.Memory, no MCP-shaped []any. It applies NO
// internal k default (round-4 finding-7, same discipline as listMemory's
// Limit) — store.SearchReranked rejects K==0, so each adapter (MCP closure: 8;
// Connect: 20) must apply its own default before calling here.
//
// req.Scope/req.CrossSpine are resolved via effectiveSearchScope as the FIRST
// statement, and the RESOLVED scope — never req.Scope directly — is what
// reaches d.st.SearchReranked. There is deliberately no store-level
// CrossSpine flag (D-07): the store still receives nothing but a scope
// string, so this call is not a second source of truth for the cross-spine
// decision. What it closes is the hole D-07 leaves behind — once an empty
// scope means "everything readable" at the store layer, this is the last
// chokepoint before a caller could reach that widened filter by forgetting
// the guard.
func (d *deps) searchMemory(ctx context.Context, c caller, req coreSearchRequest) ([]store.Memory, error) {
	// D-06a: searchArgs.Query carries omitempty now; this is the sole
	// remaining guard, shared by both lanes (MCP search_memory and Connect
	// SearchMemories both build coreSearchRequest before calling here).
	if req.Query == "" {
		return nil, argErrf(classMalformed, HintRequired, "query", "query is required")
	}
	scope, err := effectiveSearchScope(req.Scope, req.CrossSpine)
	if err != nil {
		return nil, err
	}
	vec, err := d.em.EmbedQuery(ctx, req.Query)
	if err != nil {
		return nil, err
	}
	return d.st.SearchReranked(ctx, scope, c.Subj, req.Query, vec, req.K, store.SearchOptions{
		Tags:          req.Tags,
		Categories:    req.Categories,
		CreatedAfter:  req.CreatedAfter,
		CreatedBefore: req.CreatedBefore,
	})
}

// effectiveDiscoveryScope resolves the scope filter for a discovery search:
// "" means span all discovery scopes (cross_spine). cross_spine ignores any
// supplied scope; otherwise a scope is mandatory.
func effectiveDiscoveryScope(a searchDiscoveryArgs) (string, error) {
	if a.CrossSpine {
		return "", nil
	}
	if a.Scope == "" {
		rule, _ := surfaces.RuleByID(surfaces.RuleScopeRequiredUnlessCrossSpine)
		return "", conditionalErrf(classMalformed, rule)
	}
	return a.Scope, nil
}

// effectiveSearchScope resolves the scope filter for search_memory (and,
// starting in a later plan, list_memory) under D-03: cross_spine==true
// ignores any supplied scope and returns "" (span every scope the caller may
// read); otherwise a non-empty scope is mandatory. This is the load-bearing
// guard D-07 leaves as the only thing standing between an accidental empty
// scope and a store whose empty-scope semantics mean "everything readable".
//
// Unlike effectiveDiscoveryScope (which takes searchDiscoveryArgs because it
// has exactly one caller), this takes the two primitive values directly: by
// the end of this phase it serves four call sites across three different
// arg/request types (search_memory args, coreSearchRequest, and their
// list_memory analogs), so a single shared arg-struct parameter would not
// fit all of them. That divergence from the mirrored analog is deliberate.
func effectiveSearchScope(scope string, crossSpine bool) (string, error) {
	if crossSpine {
		return "", nil
	}
	if scope == "" {
		rule, _ := surfaces.RuleByID(surfaces.RuleScopeRequiredUnlessCrossSpine)
		return "", conditionalErrf(classMalformed, rule)
	}
	return scope, nil
}

// EffectiveSearchScope is the exported form of effectiveSearchScope. It
// exists solely so cmd/engram's client-side scope guard can be pinned
// against this rule at compile time, per Phase 7's D-03 amendment — it
// changes no behavior and carries no other intended consumer.
func EffectiveSearchScope(scope string, crossSpine bool) (string, error) {
	return effectiveSearchScope(scope, crossSpine)
}

// searchedScopes reports the span a cross-spine search_memory/list_memory
// query covered: the scopes the caller is AUTHORIZED to read, not the scopes
// that produced hits. When crossSpine is false it returns (nil, false, nil)
// immediately and issues no query — a scope-confined call adds no round trip
// (D-13).
//
// d.st.ListScopes applies the authz predicate (ownerOrSharedCondition) ALONE
// — no recall-window, superseded-soft-hide, tag, or category conditions —
// which is exactly the predicate a cross-spine search/list runs under. That is
// why the set it returns IS the span that was searched, not a second
// approximation of it (D-12): deriving the set from the hits instead would
// report the empty set on a zero-hit search, which is precisely the ambiguity
// criterion 5 exists to remove. A scope whose only records are superseded or
// windowed-out will still appear here while contributing nothing — that is
// the intended semantics, not a bug; a test asserting this set equals "the
// scopes with results" would be pinning the wrong contract.
//
// Per-scope counts are discarded: they belong to the deferred
// REQ-cross-spine-coverage-receipt, and D-14 keeps the response flat
// precisely so nobody pre-builds the sub-message that requirement will want.
//
// An error from ListScopes fails the call rather than degrading to an empty
// list: an empty searched_scopes would read as "searched nothing", which is
// the exact ambiguity the field exists to remove.
func (d *deps) searchedScopes(ctx context.Context, c caller, crossSpine bool) ([]string, bool, error) {
	if !crossSpine {
		return nil, false, nil
	}
	counts, more, err := d.st.ListScopes(ctx, c.Subj)
	if err != nil {
		return nil, false, err
	}
	scopes := make([]string, len(counts))
	for i, sc := range counts {
		scopes[i] = sc.Scope
	}
	return scopes, more, nil
}

// recallResultMap assembles a search_memory/list_memory MCP result map: base
// (the transport-specific entries, e.g. "memories" and, for list_memory,
// "next_cursor") plus, ONLY when crossSpine is true, "searched_scopes" and
// "scopes_truncated". On a non-cross-spine call neither key is added at all
// (D-14), so an existing consumer sees a response byte-identical to today's —
// scopes_truncated is emitted even when false on a cross-spine call
// (resolving the Claude's-discretion item in 03-CONTEXT.md) so a consumer
// reading searched_scopes can read the truncation signal directly rather
// than inferring completeness from an absent key. Mutates and returns base.
func recallResultMap(base map[string]any, crossSpine bool, scopes []string, truncated bool) map[string]any {
	if crossSpine {
		base["searched_scopes"] = scopes
		base["scopes_truncated"] = truncated
	}
	return base
}

func (d *deps) searchDiscovery(ctx context.Context, c caller, a searchDiscoveryArgs) ([]store.Memory, error) {
	// D-06a: searchDiscoveryArgs.Query carries omitempty now; this is the
	// sole remaining guard, shared by both lanes (MCP search_discovery and
	// Connect SearchDiscoveries both build searchDiscoveryArgs before
	// calling here).
	if a.Query == "" {
		return nil, argErrf(classMalformed, HintRequired, "query", "query is required")
	}
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
	// D-06a: requireID here (shared core) — symmetric across both lanes,
	// unlike Content (see updateArgs.Content's doc comment for why THAT
	// check must stay out of this function).
	if err := requireID(a.ID); err != nil {
		return mutationResult{}, err
	}
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
	if err := requireID(a.ID); err != nil {
		return store.Memory{}, err
	}
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
	if err := requireID(a.ID); err != nil {
		return err
	}
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

// deleteAll clears every record the caller owns within scope — a
// destructive teardown (D-19/T-04-17). scopeArgs.Scope now carries
// omitempty in the wire schema (D-06a), so the presence check below is the
// SOLE remaining guard between an omitted scope and this call; it MUST run
// before the d.st.DeleteAll call (the only side effect in this path) and
// does — this is the entire point of D-19's "one indivisible task" rule.
// delete_all has no Connect RPC (MCP-only), so there is no second lane to
// keep in sync.
func (d *deps) deleteAll(ctx context.Context, c caller, a scopeArgs) error {
	if a.Scope == "" {
		return argErrf(classMalformed, HintRequired, "scope", "scope is required")
	}
	return d.st.DeleteAll(ctx, a.Scope, c.Subj)
}

// setVisibility shares/unshares one record by id or short id. Same no-leak
// re-wrap: the SetVisibility gate's not-found echoes only the caller's input.
func (d *deps) setVisibility(ctx context.Context, c caller, a setVisibilityArgs) (mutationResult, error) {
	if err := requireID(a.ID); err != nil {
		return mutationResult{}, err
	}
	// D-06a: Shared is now a *bool (setVisibilityArgs doc comment) so an
	// absent shared and an explicit shared:false are distinguishable; reject
	// only the former. Both lanes always supply a non-nil value — Connect's
	// visibilityToShared (protoconv.go) is unconditional — so this check is
	// safe in the shared core, unlike updateArgs.Content.
	if a.Shared == nil {
		return mutationResult{}, argErrf(classMalformed, HintRequired, "shared", "shared is required")
	}
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
	if err := d.st.SetVisibility(ctx, pid, c.Subj, *a.Shared); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mutationResult{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return mutationResult{}, err
	}
	return mutationResult{ID: rec.ID, ShortID: rec.ShortID}, nil
}

// resolvedSupersedeTarget is one supersede_memory target after Class 1/2 of
// the staged preflight below: Input is exactly what the caller sent (a full
// UUID or a short_id, verbatim), ID is its resolved canonical UUID, and Rec
// is the fetched, ownership-verified record. Every rejection message in this
// verb is built from Input; nothing downstream may build a caller-facing
// message from ID. Echoing a resolved UUID would hand back another owner's
// real id for a short_id the caller merely guessed — exactly the leak the
// 404-indistinguishability rule (INV-01) exists to prevent.
type resolvedSupersedeTarget struct {
	Input string
	ID    string
	Rec   store.Memory
}

// renderTargetRejection is the ONE production rendering helper for every
// caller-facing rejection this verb produces — every class in
// resolveAndAuthorizeSupersedeTargets/validateSupersedeTargetState below, and
// both store-tier re-wraps in supersedeMemory. Nothing else hand-assembles an
// equivalent fmt.Errorf: plan 05's documentation drift gate asserts the
// published examples are byte-identical to what the server actually renders,
// and a test that re-types the same format string drifts in parallel with
// the handler and proves nothing. This is the single definition both call.
func renderTargetRejection(sentinel error, inputs []string) error {
	return fmt.Errorf("%w: %s", sentinel, strings.Join(inputs, ", "))
}

// resolveAndAuthorizeSupersedeTargets is stage one of supersede_memory's
// split preflight (Classes 1-2 of 4). The class order is fixed and
// deliberate — nothing that can report on a specific record precedes the
// ownership stage:
//  1. set shape (empty array, blank entry) — names the argument, never a
//     value, and discloses nothing about any record;
//  2. addressability and access — resolution-not-found, resolution-ambiguous,
//     fetch-not-found and not-owned ALL collapse into the existing not-found
//     sentinel, so "not yours" and "does not exist" are one answer.
//
// Classes 3-4 (rule immutability, already-superseded) live in
// validateSupersedeTargetState below — split into a second function, not a
// hook, so plan 03.1-04's idempotency replay check can run in the gap
// between the two calls as a structural property of the call sequence
// (D-14), not a statement position a later edit can quietly move. One class
// is evaluated per response: a Connect response carries exactly one status
// code, so a homogeneous rejection is what keeps the existing
// sentinel-to-Connect-code switch (connecterror.go) intact and unmodified.
func (d *deps) resolveAndAuthorizeSupersedeTargets(ctx context.Context, c caller, inputs []string) ([]resolvedSupersedeTarget, error) {
	// Class 1 — set shape. Runs before any store call, so it discloses
	// nothing about any record — precisely why it is allowed to run first.
	// PD-07 (03.1-00-SUMMARY.md): the target-set cap was DROPPED, so this
	// class checks shape only — no branch on the NUMBER of entries, and no
	// maximum COUNT is ever advertised. The per-entry length bound below
	// (maxSupersedeTargetBytes, WR-01) is a different axis than PD-07 and
	// does not revisit it: it bounds how LONG one entry may be, never how
	// MANY entries the set may contain.
	if len(inputs) == 0 {
		return nil, argErrf(classMalformed, HintRequired, "supersedes", "supersedes is required")
	}
	for i, in := range inputs {
		if strings.TrimSpace(in) == "" {
			return nil, argErrf(classMalformed, HintRequired, "supersedes", "supersedes entries must not be blank")
		}
		if len(in) > maxSupersedeTargetBytes {
			return nil, argErrf(classOutOfRange, HintTooLong, "supersedes", "supersedes[%d] too large: %d bytes (max %d)", i, len(in), maxSupersedeTargetBytes)
		}
	}

	// Class 2 — addressability and access, as ONE stage over THREE buckets:
	// resolved, unaddressable, and (for any other resolution error) an
	// immediate transport-failure propagation.
	type indexed struct {
		Input string
		Index int
	}
	type resolvedPair struct {
		Input string
		ID    string
		Index int
	}

	resolved := make([]resolvedPair, 0, len(inputs))
	var unaddressable []indexed
	for i, in := range inputs {
		id, err := d.st.ResolvePointID(ctx, in)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrAmbiguousShortID) {
				unaddressable = append(unaddressable, indexed{Input: in, Index: i})
				continue
			}
			return nil, err // transport failure, not a rejection class
		}
		resolved = append(resolved, resolvedPair{Input: in, ID: id, Index: i})
	}

	// FetchForUpdate is called ONLY for entries that resolved to a canonical
	// id — a resolution not-found produces no id at all (store.go:1608
	// returns the sentinel with no id), so "carrying it forward into the
	// fetch" is not an operation that exists; it is impossible by
	// construction here, not merely avoided.
	targets := make([]resolvedSupersedeTarget, 0, len(resolved))
	for _, r := range resolved {
		rec, err := d.st.FetchForUpdate(ctx, r.ID, c.Subj)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				unaddressable = append(unaddressable, indexed{Input: r.Input, Index: r.Index})
				continue
			}
			return nil, err
		}
		targets = append(targets, resolvedSupersedeTarget{Input: r.Input, ID: r.ID, Rec: rec})
	}

	// This single class is what makes "not yours" and "does not exist" one
	// answer: a non-owned record fails the same FetchForUpdate gate, joins
	// the same bucket, and produces the same sentinel and the same offender
	// count as a nonexistent id.
	//
	// Ambiguity joins that bucket for the same reason and not by accident.
	// ResolvePointID is owner-agnostic and scrolls the whole collection
	// (store.go:1530/:1555), so reporting "ambiguous" as its own class would
	// tell an unauthenticated guesser that an invented handle matches
	// several records they cannot read. A caller who legitimately owns one
	// of two records sharing a handle addresses it by its full UUID instead
	// — the accepted trade recorded in PD-06 (03.1-00-SUMMARY.md).
	if len(unaddressable) > 0 {
		// The index-stable sort is load-bearing, not tidiness (cycle-3
		// review, MEDIUM). The bucket above is filled in TWO passes —
		// resolution failures in the first pass over ResolvePointID,
		// fetch/ownership failures in the second pass over the resolved
		// pairs — so without this sort the bucket renders in PASS order
		// rather than caller order: an input of [nonOwnedA, missingB] would
		// render as "B, A", contradicting the caller-order guarantee this
		// verb keeps and reference/tools.md publishes ("in the order the
		// caller supplied them"). On single-class inputs (most of this
		// plan's tests) the sort is a visible no-op — do not delete it on
		// that evidence alone; TestSupersedeMemoryMultiOffenderOrderIsCallerOrder
		// is the mixed-class case that needs it.
		slices.SortStableFunc(unaddressable, func(a, b indexed) int {
			return cmp.Compare(a.Index, b.Index)
		})
		names := make([]string, len(unaddressable))
		for i, u := range unaddressable {
			names[i] = u.Input
		}
		return nil, renderTargetRejection(store.ErrNotFound, names)
	}

	// Dedupe by resolved ID (PD-01, 03.1-00-SUMMARY.md): the dedupe pass
	// MUST run over resolved ids, never the caller's original spellings —
	// preserving the first occurrence's Input and the caller's input order
	// so a duplicate never produces two entries in a later rejection
	// message, and Store.Supersede's non-reentrant per-target lock is never
	// asked to acquire the same target twice.
	seen := make(map[string]struct{}, len(targets))
	out := make([]resolvedSupersedeTarget, 0, len(targets))
	for _, t := range targets {
		if _, ok := seen[t.ID]; ok {
			continue
		}
		seen[t.ID] = struct{}{}
		out = append(out, t)
	}
	return out, nil
}

// validateSupersedeTargetState is stage two of supersede_memory's split
// preflight (Classes 3-4 of 4), run over the resolved, owner-verified pairs
// resolveAndAuthorizeSupersedeTargets returned. Both classes operate only on
// records the caller provably owns — nothing here can be reached by a
// non-owned or nonexistent target, since those were rejected in stage one.
func (d *deps) validateSupersedeTargetState(_ context.Context, _ caller, targets []resolvedSupersedeTarget) error {
	// Class 3 — rule immutability (CR-02): list_rules relies on Store.List's
	// unconditional superseded_by gate to present the "complete rule set" —
	// superseding a rule would silently vanish it from that index without
	// going through the required delete flow. Mirrors updateMemory
	// (tools.go:1116) / setVisibility (tools.go:1233), widened to name every
	// rule offender in the set, not just the first.
	var ruleInputs []string
	for _, t := range targets {
		if t.Rec.Category == "rule" {
			ruleInputs = append(ruleInputs, t.Input)
		}
	}
	if len(ruleInputs) > 0 {
		return fmt.Errorf("%w — delete the rule instead of superseding it", renderTargetRejection(errRuleImmutable, ruleInputs))
	}

	// Class 4 — already superseded. This is a preflight, not the authority:
	// Store.Supersede re-checks under its own per-target locks, because a
	// concurrent caller can supersede a target between this read and the
	// write. That store-level re-check — not this one — is what closes the
	// race (CR-01); this stage only saves a doomed call its embed/mint spend.
	var supersededInputs []string
	for _, t := range targets {
		if t.Rec.SupersededBy != nil && *t.Rec.SupersededBy != "" {
			supersededInputs = append(supersededInputs, t.Input)
		}
	}
	if len(supersededInputs) > 0 {
		return renderTargetRejection(store.ErrAlreadySuperseded, supersededInputs)
	}
	return nil
}

// supersedeMemory corrects a memory the caller owns by merging one or more
// targets into a single new record (phase 03.1: promoted from one target to
// a set — D-01 promote semantics, a one-element array behaves exactly as the
// pre-phase single-target call). It runs the staged preflight above — the
// authorize stage (resolution, ownership, no rule/no-already-superseded is
// NOT yet checked), then the idempotency replay check (D-12/D-14, below),
// then the state stage — strictly ahead of the billable embed and the
// Qdrant-hitting MintShortID call (CR-03 cost-amplification hardening: a set
// that cannot succeed never spends), then delegates the create+back-stamp to
// Store.Supersede — which re-gates every target via getWritable/ActionWrite
// under its own per-target locks (SC3: a caller with only read/shared access
// to a target cannot supersede it; D-07: supersession only ever fires from
// this explicit call, never a similarity-threshold or write-through path;
// CR-01: the store-level re-gate is what Store.Supersede's locks make atomic
// against a concurrent racing caller). Store-tier rejections are re-wrapped
// through renderTargetRejection with the caller's ORIGINAL inputs, never a
// resolved target id — same 404-indistinguishability discipline as
// setVisibility/storeDiscovery (a non-owner cannot learn a target exists).
// The new correcting record is store_memory-shaped, so it is enqueued for
// async summary-on-write like any other store_memory write, exactly once
// regardless of target-set size.
func (d *deps) supersedeMemory(ctx context.Context, c caller, a supersedeArgs) (string, string, error) {
	if err := validateStoreArgs(a.storeArgs, d.maxSummaryBytes); err != nil {
		return "", "", err
	}
	if err := validateCitations(a.Citations, 0); err != nil {
		return "", "", err
	}

	owner := c.Subj.Owner()

	targets, err := d.resolveAndAuthorizeSupersedeTargets(ctx, c, a.Supersedes)
	if err != nil {
		return "", "", err
	}

	targetIDs := make([]string, len(targets))
	for i, t := range targets {
		targetIDs[i] = t.ID
	}

	// D-12/D-14: the idempotency replay check runs HERE, between the
	// authorize stage above and the state stage below — a structural
	// property of the call sequence (two separate statements), not a
	// position inside one function body a later edit can quietly move.
	// Ordered the other way, a retry after an unobserved success would be
	// rejected by the already-superseded stage before the key is ever
	// consulted, and that failure mode is invisible in every happy-path
	// test. See checkIdempotentMergeReplay's doc comment for PD-09's
	// residual narrowing (a retry whose targets were deleted/reowned in the
	// meantime never reaches this check — it was already rejected above).
	replay, replayID, replayShortID, pointID, err := d.checkIdempotentMergeReplay(ctx, owner, a, targetIDs)
	if err != nil {
		return "", "", err
	}
	if replay {
		return replayID, replayShortID, nil
	}

	if err := d.validateSupersedeTargetState(ctx, c, targets); err != nil {
		return "", "", err
	}

	m := a.toMemory(owner, c.Actor, d.clock())
	m.EmbedderIdentity = d.embedderIdentity
	m.Supersedes = targetIDs
	if pointID != "" {
		m.ID = pointID
		m.IdempotencyFingerprint = mergeFingerprint(a.storeArgs, targetIDs)
	}
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Supersede(ctx, m, vec, targetIDs, c.Subj); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			// The race path: the handler cannot know which target changed
			// hands between its own gate and the store's, so naming the
			// whole set is both correct and the conservative choice for
			// indistinguishability.
			inputs := make([]string, len(targets))
			for i, t := range targets {
				inputs[i] = t.Input
			}
			return "", "", renderTargetRejection(store.ErrNotFound, inputs)
		case errors.Is(err, store.ErrAlreadySuperseded):
			// THE LOSING RACER (T-03.1-26). Store.Supersede rejects an
			// already-stamped target under its own per-target locks BEFORE
			// its Upsert, so a losing simultaneous keyed merge never writes
			// and never reaches the keyed post-write re-read below.
			// resolveLostMergeRace is the only path by which such a call
			// can still return the winner's persisted answer — and only on
			// a fingerprint match; a mismatch stays a genuine conflict
			// (D-13), never converted into a success.
			if rid, rsid, rerr := d.resolveLostMergeRace(ctx, a, targetIDs, pointID, err); rerr == nil {
				return rid, rsid, nil
			} else if !errors.Is(rerr, store.ErrAlreadySuperseded) {
				return "", "", rerr
			}
			// Recover the offending canonical ids from the typed error
			// (plan 01, PD-10) and map each back to its Input through the
			// resolved pairs. Do NOT parse the error's message:
			// substring-matching resolved ids out of formatted prose is
			// fragile, and it is this mapping that keeps a resolved UUID
			// out of the response — a parsing bug here would be a
			// disclosure bug, not just a cosmetic one.
			var mte *store.MultiTargetError
			inputs := a.Supersedes // conservative fallback: the whole input set
			if errors.As(err, &mte) {
				byID := make(map[string]string, len(targets))
				for _, t := range targets {
					byID[t.ID] = t.Input
				}
				mapped := make([]string, 0, len(mte.IDs))
				for _, id := range mte.IDs {
					if in, ok := byID[id]; ok {
						mapped = append(mapped, in)
					}
				}
				if len(mapped) > 0 {
					inputs = mapped
				}
			}
			return "", "", renderTargetRejection(store.ErrAlreadySuperseded, inputs)
		default:
			return "", "", err
		}
	}
	// Re-read the point after Upsert so a concurrent DIFFERENT keyed merge
	// racing on the SAME deterministic pointID (same owner+scope+key, a
	// non-overlapping target set so no shared target lock blocks it —
	// unlike the identical-target-set race, which resolveLostMergeRace
	// covers) returns the short_id that was ACTUALLY PERSISTED, not the one
	// it discarded — mirrors persistAndEnqueue's keyed-path tail. Gated to
	// the keyed path only: m.IdempotencyFingerprint is only ever non-empty
	// when this write used a deterministic pointID; a failed re-Get here is
	// non-fatal — fall back to the locally-minted value rather than fail an
	// otherwise-successful write.
	if m.IdempotencyFingerprint != "" {
		if persisted, gerr := d.st.Get(ctx, m.ID); gerr == nil {
			m.ShortID = persisted.ShortID
		}
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

	if err := registerTools(s, d); err != nil {
		return nil, fmt.Errorf("register tools: %w", err)
	}

	// Compose both queues' shutdown into a single closure: the caller
	// (serve.go) invokes this once, strictly after httpSrv.Shutdown returns,
	// draining the summary queue then the usage queue (both nil-safe no-ops
	// when disabled).
	return func(ctx context.Context) {
		d.summaryQueue.Shutdown(ctx)
		d.usageQueue.Shutdown(ctx)
	}, nil
}

// supersedeMemoryInputSchema builds supersede_memory's advertised input
// schema: the same reflection-inferred shape jsonschema.For[T] produces for
// every other tool in this file, plus an explicit minItems constraint on
// `supersedes` that the "jsonschema" struct-tag mechanism cannot express — it
// supplies only a property description, never a numeric constraint (see
// jsonschema-go's own doc.go). A runtime-only bound the advertised schema
// does not mention is a rejection an agent can only discover by failing, so
// this is the ONE place the constraint is set: registerTools and
// TestSupersedeMemorySchemaAdvertisesTargetBounds both call this exact
// function, so the advertised schema and the value a test asserts against
// can never drift apart from each other. PD-07 (03.1-00-SUMMARY.md): the
// target-set cap was DROPPED — minItems: 1 is the ONLY bound ever
// advertised here; no maxItems is set or ever should be.
func supersedeMemoryInputSchema() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[supersedeArgs](nil)
	if err != nil {
		return nil, err
	}
	if prop, ok := schema.Properties["supersedes"]; ok {
		prop.MinItems = jsonschema.Ptr(1)
	}
	return schema, nil
}

// registerTools registers every MCP tool handler on s using the
// dependencies in d. Register calls this once buildDepsFromEnv has
// succeeded — extracting it here creates the seam registertools_test.go
// needs to enumerate the REAL registered tool set (names, Descriptions,
// wire shape) against a bare &deps{}: no live Qdrant, no live embedder, no
// ENGRAM_* environment configuration. This mirrors
// argattribution_test.go's established pattern of calling deps methods
// directly against a bare struct literal without live backend config.
//
// This started as a pure mechanical extraction from Register: no closure
// body changes, no tool name changes, no Description changes, no reordering.
// It returned nil unconditionally at extraction time (mcp.AddTool has no
// failure mode) but kept the error-returning signature so a future failure
// mode inside registration would not require changing Register's own error
// handling — the signature itself is the seam registertools_test.go pins.
// Plan 03.1-03 is that future failure mode: supersede_memory's advertised
// input schema is built once here (supersedeMemoryInputSchema, below) so it
// can carry a minItems constraint jsonschema.For's own tag mechanism cannot
// express, and that build step's error — vanishingly unlikely for a
// well-formed struct, but real — is now propagated instead of ignored.
//
// signature is the deliberate, plan-required seam (see doc comment above).
func registerTools(s *mcp.Server, d *deps) error {
	// scopeRule composes the same declared sentence effectiveSearchScope's
	// rejection and cmd/engram's --scope Usage string already reference
	// (D-03) — search_memory/list_memory/search_discovery's Descriptions
	// below compose it too, rather than each retyping their own copy, so
	// this surface can never drift from the registry independently of the
	// other two.
	scopeRule, _ := surfaces.RuleByID(surfaces.RuleScopeRequiredUnlessCrossSpine)
	// discoveryNotSchedulableRule/windowAtLeastOneRule/windowOrderingRule
	// (02-03-PLAN.md Task 1): same D-03 composition discipline as scopeRule
	// above, for the three rules parseWindow's rejections now reference via
	// conditionalErrf. discoveryNotSchedulableRule's Sentence is composed
	// ONLY into schedule_memory's Description (WR-01): the "category" field
	// is shared, via Go embedding of storeArgs, across store_memory's and
	// supersede_memory's arg structs too, but the rejection only ever fires
	// from parseWindow (schedule_memory's handler) — composing it onto the
	// other two tools' Descriptions told readers of tools that never
	// schedule anything about a rule that never fires there. See
	// storeArgs.Category's jsonschema tag comment (this rule's SurfaceFields
	// declaration) for the applicability-derivation half of this fix.
	discoveryNotSchedulableRule, _ := surfaces.RuleByID(surfaces.RuleDiscoveryNotSchedulable)
	windowAtLeastOneRule, _ := surfaces.RuleByID(surfaces.RuleScheduleWindowAtLeastOne)
	windowOrderingRule, _ := surfaces.RuleByID(surfaces.RuleWindowOrdering)

	mcp.AddTool(s, &mcp.Tool{Name: "store_memory", Description: "Persist a deliberate, well-formed memory. Do NOT store transient state, secrets, or timestamps. Optionally pass `summary`: a one-line recall summary shown in place of content (keep negations/identifiers verbatim). Optionally pass `idempotency_key` for safe retries: same key + identical content returns the original id/short_id unchanged; same key + different content is rejected; omit for a fresh record every time. The result includes the memory's id and short_id.", Annotations: annotationsFor("store_memory")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.storeMemory(ctx, c, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "schedule_memory", Description: "Persist a memory with a validity window (not_before defers recall; not_after expires it). " + windowAtLeastOneRule.Sentence + "; " + windowOrderingRule.Sentence + ". " + discoveryNotSchedulableRule.Sentence + ". Optionally pass `idempotency_key` for safe retries: same key + identical content returns the original id/short_id unchanged (the schedule window is NOT part of the replay check); same key + different content is rejected; omit for a fresh record every time. The result includes the memory's id and short_id.", Annotations: annotationsFor("schedule_memory")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scheduleArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.scheduleMemory(ctx, c, a)
			return textResult(fmt.Sprintf("scheduled %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_memory", Description: "Semantic search within a scope. " + scopeRule.Sentence + "; `cross_spine=true` spans every scope the caller can read (ignoring `scope` if supplied). Optionally pass `tags` to restrict to records carrying all listed tags (AND) before ranking. Returns compact summaries by default (id, summary, summary_source, scope, category, tags, created_at); pass `full=true` for full content, or fetch one record in full via get_memory. Each result carries a `score`: the raw Qdrant cosine similarity for this query (higher = closer), present when non-zero; unranked list_memory/get_memory results have a zero/omitted score.", Annotations: annotationsFor("search_memory")},
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
				return nil, nil, argErrf(classMalformed, HintFormat, "created_after", "created_after must be RFC3339")
			}
			before, err := parseRFC3339(a.CreatedBefore)
			if err != nil {
				return nil, nil, argErrf(classMalformed, HintFormat, "created_before", "created_before must be RFC3339")
			}
			if _, err := effectiveSearchScope(a.Scope, a.CrossSpine); err != nil {
				return nil, nil, err
			}
			if a.CrossSpine && a.Scope != "" {
				// Don't echo the caller-supplied scope value into logs (avoids
				// unbounded/sensitive scope strings reaching log aggregation) —
				// mirrors deps.searchDiscovery's identical discipline (D-02).
				slog.InfoContext(ctx, "search_memory: cross_spine=true; ignoring supplied scope")
			}
			ms, err := d.searchMemory(ctx, c, coreSearchRequest{
				Scope: a.Scope, Query: a.Query, K: k, Tags: a.Tags, Categories: a.Categories,
				CreatedAfter: after, CreatedBefore: before, CrossSpine: a.CrossSpine,
			})
			if err != nil {
				return nil, nil, err
			}
			scopes, truncated, err := d.searchedScopes(ctx, c, a.CrossSpine)
			if err != nil {
				return nil, nil, err
			}
			// MCP-specific recall shaping lives here, not in the shared core
			// (D-07): the core returns raw []store.Memory.
			hits := shapeRecall(ms, a.Full, d.summaryMaxChars)
			result := recallResultMap(map[string]any{"memories": hits}, a.CrossSpine, scopes, truncated)
			return textResult(fmt.Sprintf("%d hits", len(hits))), result, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_memory", Description: "List memories in a scope without a query. Most-recent first. " + scopeRule.Sentence + "; `cross_spine=true` spans every scope the caller can read (ignoring `scope` if supplied). Optional `created_after`/`created_before` (RFC3339) window and `cursor` for paging (use the returned next_cursor). Optional `tags` (AND). Returns {memories, next_cursor}; compact summaries by default, `full=true` for full content.", Annotations: annotationsFor("list_memory")},
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
				return nil, nil, argErrf(classMalformed, HintFormat, "created_after", "created_after must be RFC3339")
			}
			before, err := parseRFC3339(a.CreatedBefore)
			if err != nil {
				return nil, nil, argErrf(classMalformed, HintFormat, "created_before", "created_before must be RFC3339")
			}
			if _, err := effectiveSearchScope(a.Scope, a.CrossSpine); err != nil {
				return nil, nil, err
			}
			if a.CrossSpine && a.Scope != "" {
				// Don't echo the caller-supplied scope value into logs (avoids
				// unbounded/sensitive scope strings reaching log aggregation) —
				// mirrors search_memory's identical discipline (D-02).
				slog.InfoContext(ctx, "list_memory: cross_spine=true; ignoring supplied scope")
			}
			res, err := d.listMemory(ctx, c, coreListRequest{
				Scope: a.Scope, Limit: limit, Tags: a.Tags, Categories: a.Categories,
				CreatedAfter: after, CreatedBefore: before,
				Cursor: a.Cursor,
				// Preserves today's UNCONDITIONAL MCP cursor-mode pagination
				// (round-3 HIGH-2): the neutral core does NOT hardcode this, so
				// without an explicit true here the tokenless first page would
				// silently stop cursoring (offset mode leaves next_cursor empty,
				// store.go:817).
				CursorMode: true,
				CrossSpine: a.CrossSpine,
			})
			if err != nil {
				return nil, nil, err
			}
			scopes, truncated, err := d.searchedScopes(ctx, c, a.CrossSpine)
			if err != nil {
				return nil, nil, err
			}
			// MCP-specific recall shaping lives here, not in the shared core
			// (D-07): the core returns raw []store.Memory.
			mems := shapeRecall(res.Memories, a.Full, d.summaryMaxChars)
			result := recallResultMap(map[string]any{"memories": mems, "next_cursor": res.NextToken}, a.CrossSpine, scopes, truncated)
			return textResult(fmt.Sprintf("%d memories", len(mems))), result, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_scheduled", Description: "List your windowed memories the recall gate is hiding: state=scheduled (not yet active, default) | expired | all. Active memories surface via list_memory/search_memory.", Annotations: annotationsFor("list_scheduled")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listScheduledArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			mems, err := d.listScheduled(ctx, c, a)
			return textResult(fmt.Sprintf("%d scheduled", len(mems))), map[string]any{"memories": mems}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_memory", Description: "Fetch one memory by id. Unlike search_memory/list_memory, fetch-by-id is NOT recall-gated: it returns every state recall hides — scheduled (not-yet-active), expired, superseded, and archived records too. The id may be the full UUID or the short_id.", Annotations: annotationsFor("get_memory")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			m, err := d.getMemory(ctx, c, a)
			return textResult(m.Content), m, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_memory", Description: "Replace a memory's content in place (re-embeds). Optionally set `shared` to toggle visibility (true=shared, false=private); omit to keep current visibility. Optionally set `tags` to replace the full tag set (empty array clears); omit to keep current tags. Optionally set `summary` to replace the recall summary (empty string clears); omit to keep current. If you change content while a caller-authored summary exists, you must address the summary (re-send, update, or clear) or the update is rejected. The id may be the full UUID or the short_id.", Annotations: annotationsFor("update_memory")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			// D-06a: validateUpdateArgs re-enforces content-requiredness in Go
			// ONLY here (the MCP closure) — the wire contract still requires
			// it on every MCP call, but the schema no longer carries it as
			// "required" (updateArgs.Content's doc comment explains why this
			// check must NOT move into deps.updateMemory). The mutationResult
			// isn't surfaced here — MCP's wire shape hasn't changed (Connect's
			// response uses it, 17-03/17-04).
			if err := validateUpdateArgs(a, d.maxSummaryBytes); err != nil {
				return nil, nil, err
			}
			_, err = d.updateMemory(ctx, c, a)
			return textResult("updated"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_memory", Description: "Delete one memory by id. The id may be the full UUID or the short_id.", Annotations: annotationsFor("delete_memory")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			err = d.deleteMemory(ctx, c, a)
			return textResult("deleted"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "delete_all", Description: "Delete your own memories in a scope (teardown); never another caller's records.", Annotations: annotationsFor("delete_all")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scopeArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			err = d.deleteAll(ctx, c, a)
			return textResult("scope cleared"), nil, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "store_discovery", Description: "Cache agent-earned codebase understanding with citations. kind=map|fact; >=1 citation; scope discovery:repo:<repo>. The result includes the discovery's id and short_id.", Annotations: annotationsFor("store_discovery")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.storeDiscovery(ctx, c, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search_discovery", Description: "Semantic search over the discovery pool. " + scopeRule.Sentence + "; optional kind=map|fact. Results carry citations + created_at (aging signals).", Annotations: annotationsFor("search_discovery")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			hits, err := d.searchDiscovery(ctx, c, a)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"discoveries": hits}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "set_visibility", Description: "Share or unshare a memory you own. shared=true → readable by any authenticated caller (never writable by others); false → private. The id may be the full UUID or the short_id.", Annotations: annotationsFor("set_visibility")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a setVisibilityArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			_, err = d.setVisibility(ctx, c, a)
			return textResult("visibility updated"), nil, err
		})

	supersedeSchema, err := supersedeMemoryInputSchema()
	if err != nil {
		return fmt.Errorf("supersede_memory input schema: %w", err)
	}
	mcp.AddTool(s, &mcp.Tool{Name: "supersede_memory", Description: "Correct a memory you own by superseding one or more targets: stores a single new record and marks each target superseded_by the new one. Targets are soft-hidden from search_memory/list_memory but remain fetchable via get_memory — history is preserved, nothing is deleted or overwritten. An invalid target set rejects the whole call once, naming every offending target of one failure class: a target you do not own, one that does not exist, and one whose short_id is ambiguous (matches more than one record) are all the same rejection — replace an ambiguous short_id with the target's full UUID. Rejects if any target is already superseded (single live head per chain) or is a rule (delete it instead). Each target id may be the full UUID or short_id.", InputSchema: supersedeSchema, Annotations: annotationsFor("supersede_memory")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a supersedeArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.supersedeMemory(ctx, c, a)
			return textResult(fmt.Sprintf("stored %s, superseding %s", id, strings.Join(a.Supersedes, ", "))), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "store_rule", Description: "Persist a NORMATIVE rule (ground truth) for a repo/project. Call ONLY on explicit user instruction — never promote a rule unilaterally; propose it to the user instead. scope=rule:repo:<repo> or rule:project:<project>. summary is REQUIRED and is the one-line index entry (single line). Rules are always shared and user-blessed. The result includes the rule's id and short_id.", Annotations: annotationsFor("store_rule")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeRuleArgs) (*mcp.CallToolResult, any, error) {
			c, err := callerFromContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			id, sid, err := d.storeRule(ctx, c, a)
			return textResult(fmt.Sprintf("stored rule %s", id)), map[string]string{"id": id, "short_id": sid}, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_rules", Description: "List the COMPLETE rule set for one or more rule:* scopes, oldest-first. Compact index shape by default (short_id, summary, tags); full=true adds content. Optional tags filter (AND). Rules are the repo/project's normative ground truth.", Annotations: annotationsFor("list_rules")},
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
	return nil
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
