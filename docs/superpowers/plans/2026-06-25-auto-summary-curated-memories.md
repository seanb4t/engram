<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Auto-Summary for Curated Memories Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut recall-time token cost by returning a short `summary` in place of full `content`, with explicit (caller-authored) summaries and an operator-invoked cheap-model fallback.

**Architecture:** Reuse the existing `store.Memory.Summary` payload field (zero Qdrant migration) plus additive `summary_source`/`summary_model` keys. A new `internal/summarize` client mirrors `internal/embed`, hitting `/v1/chat/completions` on the same gateway. Auto-fill is an offline `engram summarize-missing` CLI sweep; recall (`search_memory`/`list_memory` + Connect) returns summaries by default with `full=true` opt-in. `update_memory` rejects an unaddressed caller-authored summary when content changes.

**Tech Stack:** Go 1.26, Qdrant (testcontainers for tests), cobra CLI, koanf config, connect-go/buf proto, MCP go-sdk, SvelteKit web UI.

**Spec:** `docs/superpowers/specs/2026-06-25-auto-summary-curated-memories-design.md` · **Bead:** engram-cly5

## Conventions every task follows

- New `internal/**` and `cmd/**` Go files MUST open with the SPDX header:

  ```go
  // SPDX-License-Identifier: Apache-2.0
  // Copyright 2026 Sean Brandt
  ```

- Store integration tests use `testStore(t)` (testcontainers Qdrant; auto-skips when no Docker / `ENGRAM_QDRANT_TEST_ADDR`). Pure-logic tests need no Qdrant.
- VCS is jj. Commit per `references/vcs-preamble.md`: `jj commit -m "type(scope): msg"` with the AI-authorship byline. Never push.
- Quality gate before each commit: `go test ./...` (or the targeted package), `task lint` when touching many files.

---

### Task 1: Data model — `summary_source` / `summary_model` provenance + payload round-trip

**Files:**

- Modify: `internal/store/store.go` (Memory struct ~46-78; `payload` ~150-189; `fromPayload` ~191-271)
- Test: `internal/store/store_test.go` (add a white-box round-trip test; package `store`)

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go` (it is `package store`, so it can call `payload`/`fromPayload` directly and convert via `qdrant.NewValueMap`):

```go
func TestPayloadRoundTripSummaryProvenance(t *testing.T) {
	m := Memory{
		ID: "11111111-1111-1111-1111-111111111111", Content: "long original content",
		Scope: "repo:x", Category: "convention", Source: "agent-inferred",
		Summary: "terse line", SummarySource: "auto", SummaryModel: "summary-cheap",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	vm := qdrant.NewValueMap(payload(m))
	got := fromPayload(m.ID, vm)
	if got.Summary != "terse line" || got.SummarySource != "auto" || got.SummaryModel != "summary-cheap" {
		t.Fatalf("summary provenance not round-tripped: %+v", got)
	}

	// A curated record with no summary must still round-trip cleanly (empty source).
	plain := Memory{ID: "22222222-2222-2222-2222-222222222222", Content: "c", Category: "gotcha"}
	g2 := fromPayload(plain.ID, qdrant.NewValueMap(payload(plain)))
	if g2.Summary != "" || g2.SummarySource != "" || g2.SummaryModel != "" {
		t.Fatalf("empty-summary record drifted: %+v", g2)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestPayloadRoundTripSummaryProvenance -v`
Expected: FAIL — `got.SummarySource` is empty because `payload` does not yet write `summary` for non-discovery records nor `summary_source`/`summary_model`, and `Memory` has no `SummarySource`/`SummaryModel` fields (compile error first).

- [ ] **Step 3: Add the provenance fields to `Memory`**

In `internal/store/store.go`, replace the trailing discovery field block (currently ending at `Summary string`):

```go
	// Discovery-only (zero-valued for the curated four categories).
	Kind      string     `json:"kind,omitempty"`      // "map" | "fact"
	Citations []Citation `json:"citations,omitempty"` // >= 1 for discoveries
	// Summary is a short recall line shown in place of Content by the recall
	// path. Authored by the caller (SummarySource "client") or filled by the
	// offline summarize-missing sweep ("auto"); "" means none.
	Summary string `json:"summary,omitempty"`
	// SummarySource is the trust signal: "client" | "auto" | "" (none).
	SummarySource string `json:"summary_source,omitempty"`
	// SummaryModel names the model that produced an "auto" summary (diagnostics);
	// empty otherwise.
	SummaryModel string `json:"summary_model,omitempty"`
```

- [ ] **Step 4: Write `summary`/`summary_source`/`summary_model` in `payload` for all records**

In `payload`, move the `summary` write OUT of the discovery branch and add the provenance keys. Replace the discovery block:

```go
	p["summary"] = m.Summary
	p["summary_source"] = m.SummarySource
	if m.SummaryModel != "" {
		p["summary_model"] = m.SummaryModel
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
```

- [ ] **Step 5: Read the provenance keys in `fromPayload`**

In `fromPayload`, just after the existing `if v, ok := p["summary"]; ok { m.Summary = v.GetStringValue() }` block, add:

```go
	if v, ok := p["summary_source"]; ok {
		m.SummarySource = v.GetStringValue()
	}
	if v, ok := p["summary_model"]; ok {
		m.SummaryModel = v.GetStringValue()
	}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestPayloadRoundTripSummaryProvenance -v`
Expected: PASS

- [ ] **Step 7: Commit**

`jj commit -m "feat(store): summary provenance fields + payload round-trip (engram-cly5)"` (with byline)

---

### Task 2: Summarizer client (`internal/summarize`)

**Files:**

- Create: `internal/summarize/summarize.go`
- Test: `internal/summarize/summarize_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/summarize/summarize_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package summarize

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummarizePostsChatCompletionAndReturnsContent(t *testing.T) {
	var gotModel, gotPath, gotSystem, gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		var req chatReq
		_ = json.Unmarshal(body, &req)
		gotModel = req.Model
		for _, m := range req.Messages {
			switch m.Role {
			case "system":
				gotSystem = m.Content
			case "user":
				gotUser = m.Content
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  do NOT remove --flag  "}}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k", "summary-cheap", 280)
	out, err := c.Summarize(context.Background(), "the full memory content here")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if out != "do NOT remove --flag" {
		t.Fatalf("summary not trimmed/returned: %q", out)
	}
	if gotPath != "/v1/chat/completions" || gotModel != "summary-cheap" {
		t.Fatalf("wrong request: path=%q model=%q", gotPath, gotModel)
	}
	if !strings.Contains(gotSystem, "Preserve") || gotUser != "the full memory content here" {
		t.Fatalf("messages malformed: system=%q user=%q", gotSystem, gotUser)
	}
}

func TestSummarizeErrorsOnEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()
	if _, err := New(srv.URL, "k", "m", 280).Summarize(context.Background(), "x"); err == nil {
		t.Fatal("want error on empty choices, got nil")
	}
}

func TestSummarizeTruncatesToMaxChars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"aaaaaaaaaaaaaaaaaaaa"}}]}`)) // 20 chars
	}))
	defer srv.Close()
	out, err := New(srv.URL, "k", "m", 8).Summarize(context.Background(), "x")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len([]rune(out)) > 8 {
		t.Fatalf("summary not truncated to 8: %q (len %d)", out, len([]rune(out)))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/summarize/ -v`
Expected: FAIL — package/`chatReq`/`New`/`Summarize` do not exist (compile error).

- [ ] **Step 3: Write the implementation**

Create `internal/summarize/summarize.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package summarize compresses a stored memory into a one-line recall summary
// via an OpenAI-compatible /v1/chat/completions endpoint (the same gateway
// engram already uses for embeddings). It is only invoked off the write path
// (the summarize-missing sweep), never during store_memory.
package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/seanb4t/engram/internal/summarize")

// Client produces summaries via an OpenAI-compatible chat-completions API.
type Client struct {
	baseURL  string
	apiKey   string
	model    string
	maxChars int
	http     *http.Client
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPTransport sets the underlying RoundTripper (e.g. otelhttp.NewTransport).
func WithHTTPTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.http.Transport = rt }
}

// New returns a summarizer for the given gateway, key, model, and character cap.
func New(baseURL, apiKey, model string, maxChars int, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, model: model, maxChars: maxChars,
		http: &http.Client{Timeout: 30 * time.Second}}
	for _, o := range opts {
		o(c)
	}
	return c
}

// systemPromptTmpl is fidelity-critical: the cheap model must keep the parts an
// agent will act on. %d is the character cap.
const systemPromptTmpl = `You compress a stored engineering memory into ONE terse line for fast recall.
Preserve VERBATIM: negations (do/don't, never, decline, avoid), imperatives, identifiers (flags, file paths, function/type names, IDs, env vars), and numbers.
Do not invent, infer, generalize, or add commentary. Compress only what is present.
Output a single line: no markdown, no surrounding quotes. Keep it under %d characters.`

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []chatMsg `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature"`
}

type chatResp struct {
	Choices []struct {
		Message chatMsg `json:"message"`
	} `json:"choices"`
}

// Summarize returns a one-line summary of content. Single line, trimmed, hard-
// capped at maxChars runes (defensive — the prompt also requests the cap).
func (c *Client) Summarize(ctx context.Context, content string) (sum string, err error) {
	ctx, span := tracer.Start(ctx, "summarize.Summarize",
		trace.WithAttributes(attribute.String("engram.summarize.model", c.model)))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	reqBody, _ := json.Marshal(chatReq{
		Model:       c.model,
		Temperature: 0,
		MaxTokens:   c.maxChars/3 + 32,
		Messages: []chatMsg{
			{Role: "system", Content: fmt.Sprintf(systemPromptTmpl, c.maxChars)},
			{Role: "user", Content: content},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat completions: status %d", resp.StatusCode)
	}
	var out chatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("chat completions: empty choices")
	}
	s := strings.TrimSpace(out.Choices[0].Message.Content)
	if s == "" {
		return "", fmt.Errorf("chat completions: empty summary")
	}
	if r := []rune(s); len(r) > c.maxChars {
		s = strings.TrimRight(string(r[:c.maxChars]), " ")
	}
	return s, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/summarize/ -v`
Expected: PASS (all three tests)

- [ ] **Step 5: Commit**

`jj commit -m "feat(summarize): OpenAI-compatible chat summarizer client (engram-cly5)"`

---

### Task 3: Config — `SummarizeConfig` + registry + validation

**Files:**

- Modify: `internal/config/config.go` (Config struct ~22-29; add sub-struct)
- Modify: `internal/config/registry.go` (registry slice ~25-46)
- Modify: `internal/config/validate.go` (Validate ~24-77)
- Test: `internal/config/config_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go` (package `config`):

```go
func TestSummarizeConfigDefaultsAndEnv(t *testing.T) {
	t.Setenv("ENGRAM_SUMMARY_MODEL", "summary-cheap")
	c, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Summarize.Model != "summary-cheap" {
		t.Errorf("Summarize.Model = %q, want summary-cheap", c.Summarize.Model)
	}
	if c.Summarize.MaxChars != "280" {
		t.Errorf("Summarize.MaxChars = %q, want default 280", c.Summarize.MaxChars)
	}
}

func TestValidateRejectsBadSummaryMaxCharsWhenEnabled(t *testing.T) {
	c := &Config{
		Qdrant:    QdrantConfig{Addr: "localhost:6334", Collection: "c"},
		Embed:     EmbedConfig{Model: "m", Dim: "1024"},
		OpenAI:    OpenAIConfig{BaseURL: "http://localhost:4000"},
		Summarize: SummarizeConfig{Model: "summary-cheap", MaxChars: "0"},
	}
	if err := c.Validate(); err == nil {
		t.Fatal("want error for ENGRAM_SUMMARY_MAX_CHARS=0 with model set, got nil")
	}
}

func TestValidateIgnoresSummaryWhenDisabled(t *testing.T) {
	c := &Config{
		Qdrant:    QdrantConfig{Addr: "localhost:6334", Collection: "c"},
		Embed:     EmbedConfig{Model: "m", Dim: "1024"},
		OpenAI:    OpenAIConfig{BaseURL: "http://localhost:4000"},
		Summarize: SummarizeConfig{Model: "", MaxChars: "garbage"},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("disabled summarize must not fail validation: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run 'TestSummarizeConfig|TestValidate.*Summary' -v`
Expected: FAIL — `Config.Summarize` / `SummarizeConfig` undefined (compile error).

- [ ] **Step 3: Add the sub-struct + Config field**

In `internal/config/config.go`, add to the `Config` struct after `Embed`:

```go
	Embed     EmbedConfig     `koanf:"embed"`
	Summarize SummarizeConfig `koanf:"summarize"`
```

And add the sub-struct near `EmbedConfig`:

```go
// SummarizeConfig selects the recall-summary model and the character cap shared
// by the summarizer and recall truncation. Empty Model disables auto-summary
// (presence-enables, like OIDC issuer); MaxChars defaults to "280".
type SummarizeConfig struct {
	Model    string `koanf:"model"`
	MaxChars string `koanf:"max_chars"`
}
```

- [ ] **Step 4: Register the env vars**

In `internal/config/registry.go`, add after the `embed.dim` entry:

```go
	{Key: "summarize.model", Env: "ENGRAM_SUMMARY_MODEL"},
	{Key: "summarize.max_chars", Env: "ENGRAM_SUMMARY_MAX_CHARS", Default: "280"},
```

(`summarize.model` has no default — empty disables the feature; `defaultsMap` skips empty defaults.)

- [ ] **Step 5: Validate max_chars only when the feature is enabled**

In `internal/config/validate.go`, add before the final `if len(errs) == 0` block:

```go
	if c.Summarize.Model != "" {
		switch n, err := strconv.ParseUint(c.Summarize.MaxChars, 10, 64); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_MAX_CHARS %q: must be a positive integer: %w", c.Summarize.MaxChars, err))
		case n == 0:
			errs = append(errs, errors.New("ENGRAM_SUMMARY_MAX_CHARS must be greater than 0"))
		}
	}
```

(`strconv` and `errors` are already imported by validate.go.)

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/config/ -run 'TestSummarizeConfig|TestValidate.*Summary' -v`
Expected: PASS

- [ ] **Step 7: Commit**

`jj commit -m "feat(config): SummarizeConfig (ENGRAM_SUMMARY_MODEL/MAX_CHARS) (engram-cly5)"`

---

### Task 4: Store — `SetSummary`, `FillSummary`, `SummarizeMissing` sweep

**Files:**

- Create: `internal/store/summarize.go`
- Test: `internal/store/summarize_test.go`

- [ ] **Step 1: Write the failing pure-logic test**

Create `internal/store/summarize_test.go` (package `store`):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
)

func TestShouldSummarize(t *testing.T) {
	cases := []struct {
		name     string
		m        Memory
		maxChars int
		want     bool
	}{
		{"long no-summary", Memory{Content: "abcdefghij"}, 4, true},
		{"already summarized", Memory{Content: "abcdefghij", Summary: "x"}, 4, false},
		{"too short", Memory{Content: "abc"}, 4, false},
		{"exactly cap", Memory{Content: "abcd"}, 4, false},
	}
	for _, tc := range cases {
		if got := shouldSummarize(tc.m, tc.maxChars); got != tc.want {
			t.Errorf("%s: shouldSummarize=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestFillSummarySkipsWhenNotEligible(t *testing.T) {
	// A short record must not call the summarizer or touch Qdrant (nil store is
	// safe precisely because shouldSummarize short-circuits first).
	called := false
	fn := func(_ context.Context, _ string) (string, error) { called = true; return "x", nil }
	var s *Store
	filled, err := s.FillSummary(context.Background(), Memory{Content: "abc"}, fn, "model", 4)
	if err != nil || filled || called {
		t.Fatalf("expected no-op: filled=%v called=%v err=%v", filled, called, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run 'TestShouldSummarize|TestFillSummarySkips' -v`
Expected: FAIL — `shouldSummarize` / `FillSummary` undefined.

- [ ] **Step 3: Write `internal/store/summarize.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/seanb4t/engram/internal/telemetry"
)

// SummarizeFunc compresses content into a one-line summary. Injected so the
// store never imports the summarizer package (matches Reindex's EmbedFunc).
type SummarizeFunc func(ctx context.Context, content string) (string, error)

// SummarizeOptions bounds a summarize-missing sweep.
type SummarizeOptions struct {
	Scope     string    // "" with AllScopes=false is a no-op guard (CLI requires one)
	AllScopes bool      // true sweeps every scope
	OlderThan time.Time // zero = no age filter; else only created_at before it
	Limit     int       // 0 = no cap on records scanned
	MaxChars  int       // eligibility threshold + summary cap
	Model     string    // stamped as summary_model on filled records
	DryRun    bool      // count eligible records without writing
}

// SummarizeResult is the operator-facing tally of a sweep.
type SummarizeResult struct {
	Scanned, Filled, Skipped, Failed int
}

// shouldSummarize is the per-record eligibility rule: no existing summary and a
// content body longer than the cap (short content is already recall-cheap).
func shouldSummarize(m Memory, maxChars int) bool {
	return m.Summary == "" && len([]rune(m.Content)) > maxChars
}

// SetSummary writes summary + provenance via a vector-preserving SetPayload
// (mirrors SetVisibility). Used by the auto path; always stamps source "auto".
func (s *Store) SetSummary(ctx context.Context, id, summary, model string) (err error) {
	ctx, span := tracer.Start(ctx, "store.SetSummary")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SetSummary", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload: qdrant.NewValueMap(map[string]any{
			"summary": summary, "summary_source": "auto", "summary_model": model,
		}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

// FillSummary summarizes one record and persists it, idempotently. Returns
// filled=false (no error) when the record is ineligible. This is the reusable
// unit shared by the sweep below and a future async-on-write queue worker.
func (s *Store) FillSummary(ctx context.Context, m Memory, summarize SummarizeFunc, model string, maxChars int) (filled bool, err error) {
	if !shouldSummarize(m, maxChars) {
		return false, nil
	}
	sum, err := summarize(ctx, m.Content)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(sum) == "" {
		return false, fmt.Errorf("summarize %s: empty summary", m.ID)
	}
	if err := s.SetSummary(ctx, m.ID, sum, model); err != nil {
		return false, err
	}
	return true, nil
}

// SummarizeMissing scrolls records (optionally scoped) and fills empty summaries
// best-effort. created_at age filtering is applied in-code (created_at is stored
// as an RFC3339 string, not a Qdrant-rangeable number). Per-record errors are
// counted, not fatal.
func (s *Store) SummarizeMissing(ctx context.Context, opts SummarizeOptions, summarize SummarizeFunc) (res SummarizeResult, err error) {
	ctx, span := tracer.Start(ctx, "store.SummarizeMissing")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SummarizeMissing", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", res.Filled))
		}
	}()

	var must []*qdrant.Condition
	if opts.Scope != "" {
		must = append(must, qdrant.NewMatch("scope", opts.Scope))
	}
	var filter *qdrant.Filter
	if len(must) > 0 {
		filter = &qdrant.Filter{Must: must}
	}

	var offset *qdrant.PointId
	for {
		pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         filter,
			Limit:          qdrant.PtrOf(uint32(256)),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(true),
		})
		if serr != nil {
			return res, serr
		}
		for _, p := range pts {
			if opts.Limit > 0 && res.Scanned >= opts.Limit {
				return res, nil
			}
			res.Scanned++
			m := fromPayload(p.Id.GetUuid(), p.Payload)
			if !opts.OlderThan.IsZero() && !m.CreatedAt.Before(opts.OlderThan) {
				res.Skipped++
				continue
			}
			if !shouldSummarize(m, opts.MaxChars) {
				res.Skipped++
				continue
			}
			if opts.DryRun {
				res.Filled++ // "would fill"
				continue
			}
			if _, ferr := s.FillSummary(ctx, m, summarize, opts.Model, opts.MaxChars); ferr != nil {
				res.Failed++
				continue
			}
			res.Filled++
		}
		if next == nil {
			return res, nil
		}
		offset = next
	}
}
```

> NOTE on `ScrollAndOffset`/`WithPayload`/`p.Payload`: confirm these match the signatures used in `internal/store/store.go`'s `Reindex` (Step 4a below). If `Reindex` uses different field names (e.g. a `Batch`-sized limit, or a different payload accessor), mirror them exactly.

- [ ] **Step 3a: Verify scroll API against `Reindex`**

Run: `mcp__probe__extract_code internal/store/store.go#Reindex` (or open `store.go` around the `ScrollAndOffset` call). Adjust the scroll block above to match the exact `ScrollPoints` fields and payload accessor `Reindex` already uses. Do not invent fields.

- [ ] **Step 4: Run pure-logic tests to verify they pass**

Run: `go test ./internal/store/ -run 'TestShouldSummarize|TestFillSummarySkips' -v`
Expected: PASS

- [ ] **Step 5: Write the integration test (testcontainers)**

Add to `internal/store/summarize_test.go`:

```go
func TestSummarizeMissingFillsEmptyOnly(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "repo:summary-it"
	long := "this is a long body well over the tiny cap used by the test harness here"

	seed := []Memory{
		{ID: "a0000000-0000-0000-0000-000000000001", Content: long, Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", CreatedAt: time.Now().UTC()},
		{ID: "a0000000-0000-0000-0000-000000000002", Content: long, Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", Summary: "already", SummarySource: "client", CreatedAt: time.Now().UTC()},
		{ID: "a0000000-0000-0000-0000-000000000003", Content: "short", Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", CreatedAt: time.Now().UTC()},
	}
	for _, m := range seed {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}

	fn := func(_ context.Context, _ string) (string, error) { return "AUTO terse", nil }
	res, err := s.SummarizeMissing(ctx, SummarizeOptions{Scope: scope, MaxChars: 8, Model: "summary-cheap"}, fn)
	if err != nil {
		t.Fatalf("SummarizeMissing: %v", err)
	}
	if res.Filled != 1 || res.Skipped != 2 {
		t.Fatalf("tally: filled=%d skipped=%d failed=%d scanned=%d", res.Filled, res.Skipped, res.Failed, res.Scanned)
	}
	got, err := s.GetReadable(ctx, seed[0].ID, Authenticated("owner-A"))
	if err != nil {
		t.Fatalf("get filled: %v", err)
	}
	if got.Summary != "AUTO terse" || got.SummarySource != "auto" || got.SummaryModel != "summary-cheap" {
		t.Fatalf("auto summary not persisted: %+v", got)
	}
}
```

> Add `"time"` to this test file's import block (the integration test uses `time.Now()`; the existing imports are only `context` + `testing`). `Authenticated(sub)`, `GetReadable`, and the `time.Now().UTC()` seed style are existing store APIs/conventions (`store_test.go`).

- [ ] **Step 6: Run the integration test**

Run: `go test ./internal/store/ -run TestSummarizeMissingFillsEmptyOnly -v`
Expected: PASS (or `SKIP` if no Docker/Qdrant — acceptable locally; CI has Qdrant).

- [ ] **Step 7: Commit**

`jj commit -m "feat(store): SetSummary + FillSummary + SummarizeMissing sweep (engram-cly5)"`

---

### Task 5: `update_memory` — explicit summary + fail-loud stale guard

**Files:**

- Create: `internal/server/summary.go` (pure helpers: `resolveSummaryUpdate`, `truncateForRecall`, `recallView`, `toRecallView` — recall helpers land here too, used by Task 7)
- Modify: `internal/store/store.go` (`Update` signature ~899-923)
- Modify: `internal/server/tools.go` (`updateArgs` ~264-269; `updateMemory` ~571-589; the `update_memory` registration ~Register)
- Test: `internal/server/summary_test.go`; update `internal/store/store_test.go` callers of `Update`

- [ ] **Step 1: Write the failing pure-helper test**

Create `internal/server/summary_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"errors"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

func sp(s string) *string { return &s }

func TestResolveSummaryUpdate(t *testing.T) {
	clientSum := store.Memory{Summary: "hand-written", SummarySource: "client"}
	autoSum := store.Memory{Summary: "machine", SummarySource: "auto"}
	none := store.Memory{}

	cases := []struct {
		name           string
		cur            store.Memory
		contentChanged bool
		arg            *string
		wantValue      string
		wantApply      bool
		wantErr        bool
	}{
		{"explicit set", clientSum, true, sp("new"), "new", true, false},
		{"explicit clear", clientSum, true, sp(""), "", true, false},
		{"unchanged preserves", clientSum, false, nil, "", false, false},
		{"none + change = noop", none, true, nil, "", false, false},
		{"auto + change = autoclear", autoSum, true, nil, "", true, false},
		{"client + change + unaddressed = reject", clientSum, true, nil, "", false, true},
	}
	for _, tc := range cases {
		v, apply, err := resolveSummaryUpdate(tc.cur, tc.contentChanged, tc.arg)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", tc.name, err, tc.wantErr)
			continue
		}
		if tc.wantErr {
			if !errors.Is(err, errStaleSummary) {
				t.Errorf("%s: want errStaleSummary, got %v", tc.name, err)
			}
			continue
		}
		if v != tc.wantValue || apply != tc.wantApply {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", tc.name, v, apply, tc.wantValue, tc.wantApply)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestResolveSummaryUpdate -v`
Expected: FAIL — `resolveSummaryUpdate` / `errStaleSummary` undefined.

- [ ] **Step 3: Write `internal/server/summary.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"errors"
	"strings"
	"time"

	"github.com/seanb4t/engram/internal/store"
)

// errStaleSummary rejects an update that would silently strand a caller-authored
// summary against changed content. Actionable: the agent must choose.
var errStaleSummary = errors.New(
	`content changed but a caller-authored summary would go stale: ` +
		`re-send the same summary to keep it, pass an updated one, or pass summary="" to clear it`)

// resolveSummaryUpdate decides what to persist for a record's summary during an
// update. arg==nil means the caller did not address the summary. apply==true
// means set the summary to value (value=="" clears).
func resolveSummaryUpdate(cur store.Memory, contentChanged bool, arg *string) (value string, apply bool, err error) {
	if arg != nil {
		return *arg, true, nil // explicit set or clear
	}
	if !contentChanged || cur.Summary == "" {
		return "", false, nil // preserve / nothing to lose
	}
	if cur.SummarySource == "auto" {
		return "", true, nil // server-derived: auto-clear, regenerated by next sweep
	}
	return "", false, errStaleSummary // caller-authored: refuse to silently strand
}

// recallView is the compact, summary-shaped recall result (Task 7).
type recallView struct {
	ID            string    `json:"id"`
	Summary       string    `json:"summary"`
	SummarySource string    `json:"summary_source,omitempty"`
	Truncated     bool      `json:"truncated,omitempty"`
	Scope         string    `json:"scope"`
	Category      string    `json:"category"`
	Tags          []string  `json:"tags,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// truncateForRecall returns a head-truncation of content (rune-safe) and whether
// it was cut. Short content is returned unchanged.
func truncateForRecall(content string, maxChars int) (string, bool) {
	r := []rune(content)
	if len(r) <= maxChars {
		return content, false
	}
	return strings.TrimRight(string(r[:maxChars]), " ") + "…", true
}

// toRecallView shapes one memory for default (summary) recall: the stored
// summary when present, else a content truncation.
func toRecallView(m store.Memory, maxChars int) recallView {
	summary, truncated := m.Summary, false
	if summary == "" {
		summary, truncated = truncateForRecall(m.Content, maxChars)
	}
	return recallView{
		ID: m.ID, Summary: summary, SummarySource: m.SummarySource, Truncated: truncated,
		Scope: m.Scope, Category: m.Category, Tags: m.Tags, CreatedAt: m.CreatedAt,
	}
}
```

- [ ] **Step 4: Run the helper test to verify it passes**

Run: `go test ./internal/server/ -run TestResolveSummaryUpdate -v`
Expected: PASS

- [ ] **Step 5: Extend `store.Update` to carry the summary**

In `internal/store/store.go`, change the `Update` signature and body. New signature:

```go
func (s *Store) Update(ctx context.Context, cur Memory, content string, shared *bool, tags *[]string, summary *string, vec []float32) (err error) {
```

After the existing `if tags != nil { cur.Tags = *tags }` block, add:

```go
	if summary != nil {
		cur.Summary = *summary
		if *summary == "" {
			cur.SummarySource, cur.SummaryModel = "", ""
		} else {
			cur.SummarySource, cur.SummaryModel = "client", ""
		}
	}
```

- [ ] **Step 6: Update every existing `Update` caller to pass the new arg**

`update_memory` is the only production caller (`internal/server/tools.go`). Any `store_test.go` test calling `Update(...)` must add `nil` for the new `summary` param before `vec`. Find them:

Run: `mcp__probe__grep "\.Update(ctx" internal/store internal/server`
For each call `s.Update(ctx, cur, content, shared, tags, vec)` → `s.Update(ctx, cur, content, shared, tags, nil, vec)`.

- [ ] **Step 7: Add `Summary` to `updateArgs` and wire the guard in the handler**

In `internal/server/tools.go`, extend `updateArgs`:

```go
type updateArgs struct {
	ID      string    `json:"id"`
	Content string    `json:"content"`
	Shared  *bool     `json:"shared,omitempty" jsonschema:"omit to keep current visibility; true=shared, false=private"`
	Tags    *[]string `json:"tags,omitempty" jsonschema:"omit to keep current tags; supply to replace the full set (empty array clears)"`
	Summary *string   `json:"summary,omitempty" jsonschema:"omit to keep current summary; supply to replace (empty string clears). If content changes and a caller-authored summary exists, you MUST address it (re-send to keep, update, or clear) or the update is rejected"`
}
```

Replace `updateMemory` (~571-589) with:

```go
func (d *deps) updateMemory(ctx context.Context, a updateArgs) error {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return err
	}
	// Ownership gate before embedding: one authoritative Get. A non-owner (or
	// missing record) gets ErrNotFound here, before the billable embed or write.
	cur, err := d.st.FetchForUpdate(ctx, a.ID, subj)
	if err != nil {
		return err
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
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return err
	}
	return d.st.Update(ctx, cur, a.Content, a.Shared, a.Tags, sumArg, vec)
}
```

- [ ] **Step 8: Update the `update_memory` tool description**

In `Register` (`tools.go`), append to the `update_memory` `Description` string:

```text
 Optionally set `summary` to replace the recall summary (empty string clears); omit to keep current. If you change content while a caller-authored summary exists, you must address the summary (re-send, update, or clear) or the update is rejected.
```

- [ ] **Step 9: Run the full server + store packages**

Run: `go test ./internal/server/ ./internal/store/ -v`
Expected: PASS (store integration tests SKIP without Qdrant).

- [ ] **Step 10: Commit**

`jj commit -m "feat(server): update_memory summary with fail-loud stale guard (engram-cly5)"`

---

### Task 6: `store_memory` — explicit caller summary

**Files:**

- Modify: `internal/server/tools.go` (`storeArgs` ~187-197; `toMemory` ~366-382; `store_memory` registration)
- Test: `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/tools_test.go` (package `server`):

```go
func TestToMemorySetsClientSummarySource(t *testing.T) {
	withSummary := storeArgs{Content: "c", Scope: "s", Source: "user-said", Category: "decision", Summary: "terse"}
	m := withSummary.toMemory("owner", "actor", time.Now())
	if m.Summary != "terse" || m.SummarySource != "client" {
		t.Fatalf("client summary not mapped: summary=%q source=%q", m.Summary, m.SummarySource)
	}
	noSummary := storeArgs{Content: "c", Scope: "s", Source: "user-said", Category: "decision"}
	m2 := noSummary.toMemory("owner", "actor", time.Now())
	if m2.Summary != "" || m2.SummarySource != "" {
		t.Fatalf("absent summary must leave source empty: %+v", m2)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestToMemorySetsClientSummarySource -v`
Expected: FAIL — `storeArgs` has no `Summary` field.

- [ ] **Step 3: Add `Summary` to `storeArgs` and `toMemory`**

In `tools.go`, add to `storeArgs`:

```go
	Summary   string   `json:"summary,omitempty" jsonschema:"optional one-line recall summary shown in place of content; preserve negations/identifiers; omit to leave empty (operator backfill or truncation fills recall)"`
```

In `toMemory`, set the fields:

```go
func (a storeArgs) toMemory(owner, actor string, createdAt time.Time) store.Memory {
	src := ""
	if a.Summary != "" {
		src = "client"
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
```

- [ ] **Step 4: Update the `store_memory` tool description**

In `Register`, append to `store_memory`'s `Description`:

```text
 Optionally pass `summary`: a one-line recall summary shown in place of content (keep negations/identifiers verbatim).
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestToMemorySetsClientSummarySource -v`
Expected: PASS

- [ ] **Step 6: Commit**

`jj commit -m "feat(server): store_memory accepts caller summary (engram-cly5)"`

---

### Task 7: Recall shaping — summary-by-default, `full` opt-in (MCP)

**Files:**

- Modify: `internal/server/tools.go` (`searchArgs`, `listArgs`; `searchMemory` ~519-532; `listMemory` ~485-495; `deps` struct ~31-40; `buildDepsFromEnv` ~138-149; `embedderFromConfig` area)
- Reuse: `internal/server/summary.go` (`toRecallView` from Task 5)
- Test: `internal/server/summary_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/summary_test.go`:

```go
func TestToRecallViewPrefersSummaryElseTruncates(t *testing.T) {
	withSum := store.Memory{ID: "1", Content: "a very long body well past the cap", Summary: "kept", SummarySource: "client", Scope: "s", Category: "decision"}
	v := toRecallView(withSum, 8)
	if v.Summary != "kept" || v.Truncated {
		t.Fatalf("summary should win untruncated: %+v", v)
	}
	noSum := store.Memory{ID: "2", Content: "abcdefghijklmnop", Scope: "s", Category: "gotcha"}
	v2 := toRecallView(noSum, 8)
	if !v2.Truncated || len([]rune(v2.Summary)) > 9 { // 8 + ellipsis
		t.Fatalf("long no-summary should truncate: %+v", v2)
	}
	short := store.Memory{ID: "3", Content: "tiny", Scope: "s", Category: "gotcha"}
	if v3 := toRecallView(short, 8); v3.Truncated || v3.Summary != "tiny" {
		t.Fatalf("short content returned as-is: %+v", v3)
	}
}

func TestShapeRecallFullVsSummary(t *testing.T) {
	ms := []store.Memory{{ID: "1", Content: "loooooong content over cap", Scope: "s", Category: "decision"}}
	full := shapeRecall(ms, true, 4)
	if _, ok := full[0].(store.Memory); !ok {
		t.Fatalf("full=true must yield store.Memory, got %T", full[0])
	}
	compact := shapeRecall(ms, false, 4)
	if _, ok := compact[0].(recallView); !ok {
		t.Fatalf("full=false must yield recallView, got %T", compact[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run 'TestToRecallView|TestShapeRecall' -v`
Expected: FAIL — `shapeRecall` undefined.

- [ ] **Step 3: Add `shapeRecall` to `internal/server/summary.go`**

```go
// shapeRecall renders memories for a recall response: full store.Memory values
// when full, else compact summary-shaped recallView values.
func shapeRecall(ms []store.Memory, full bool, maxChars int) []any {
	out := make([]any, len(ms))
	for i, m := range ms {
		if full {
			out[i] = m
		} else {
			out[i] = toRecallView(m, maxChars)
		}
	}
	return out
}
```

- [ ] **Step 4: Add `summaryMaxChars` to `deps` and populate it**

In `tools.go`, add the field to `deps`:

```go
	// summaryMaxChars is the recall truncation cap (ENGRAM_SUMMARY_MAX_CHARS).
	summaryMaxChars int
```

In `buildDepsFromEnv` (which already loads `cfg`), set it on the returned `deps` using the helper added below, e.g. `summaryMaxChars: summaryMaxChars(cfg)`. Add the helper near `embedderFromConfig`:

```go
// summaryMaxChars parses the recall cap, defaulting to 280 on empty/invalid.
func summaryMaxChars(cfg *config.Config) int {
	n, err := strconv.Atoi(cfg.Summarize.MaxChars)
	if err != nil || n <= 0 {
		return 280
	}
	return n
}
```

(Add `"strconv"` to the `tools.go` imports if not present.)

- [ ] **Step 5: Add `Full` to the recall args and reshape the handlers**

Add `Full bool` to both arg structs (locate `searchArgs` and `listArgs` in `tools.go`):

```go
	Full bool `json:"full,omitempty" jsonschema:"return full content instead of summaries (default false → compact summary view)"`
```

Change `searchMemory` to return shaped results:

```go
func (d *deps) searchMemory(ctx context.Context, a searchArgs) ([]any, error) {
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
	ms, err := d.st.Search(ctx, a.Scope, subj, vec, a.K, a.Tags)
	if err != nil {
		return nil, err
	}
	return shapeRecall(ms, a.Full, d.summaryMaxChars), nil
}
```

Change `listMemory`:

```go
func (d *deps) listMemory(ctx context.Context, a listArgs) ([]any, error) {
	if a.Limit == 0 {
		a.Limit = 20
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	ms, _, _, err := d.st.List(ctx, a.Scope, subj, store.ListOptions{Limit: a.Limit, Tags: a.Tags})
	if err != nil {
		return nil, err
	}
	return shapeRecall(ms, a.Full, d.summaryMaxChars), nil
}
```

(The `Register` wrappers already do `len(hits)` and `map[string]any{"memories": hits}` — both work unchanged with `[]any`.)

- [ ] **Step 5b: Fix existing handler tests that assume `[]store.Memory`**

`searchMemory`/`listMemory` now return `[]any`, so pre-existing tests in `internal/server/tools_test.go` that call them and range over the result accessing `.ID`/`.Scope` (e.g. `TestAnonReadIsolationHandlers`, `TestAuthedCrossActorSharedReadHandlers`, `TestSearchListMemoryTagsHandler`, and the local `ids(ms []store.Memory)` helper) will no longer compile.

Run: `mcp__probe__grep "searchMemory|listMemory" internal/server/tools_test.go`

For each caller, pass `Full: true` in the args (the full path yields `store.Memory` values) and type-assert per element so the existing authz/tag assertions stay intact:

```go
res, err := d.searchMemory(ctx, searchArgs{Query: q, Scope: s, Full: true})
// res is []any of store.Memory:
m := res[0].(store.Memory)
```

Update the `ids` helper (and similar) to accept `[]any` and assert each element to `store.Memory` (or read the `id` off a `recallView` where a test deliberately exercises the compact path). Do not weaken the isolation assertions — they test authz, not shape.

- [ ] **Step 6: Update `search_memory` / `list_memory` tool descriptions**

In `Register`, append to both `Description` strings:

```text
 Returns compact summaries by default (id, summary, summary_source, scope, category, tags, created_at); pass `full=true` for full content, or fetch one record in full via get_memory.
```

- [ ] **Step 7: Run server tests**

Run: `go test ./internal/server/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

`jj commit -m "feat(server): recall returns summaries by default, full opt-in (engram-cly5)"`

---

### Task 8: Connect/proto parity — `summary`, `summary_source`, `full`

**Files:**

- Modify: `proto/engram/v1/engram.proto` (Memory ~11-26; SearchMemoriesRequest ~54-60; ListMemoriesRequest ~39-52)
- Regenerate: `gen/` (via `task proto:gen`)
- Modify: `internal/server/connectapi.go` (`memoryToProto` ~31-39; `SearchMemories` ~85-103; `ListMemories` ~65-83; `memoriesToProto`)
- Test: `internal/server/connectapi_test.go`

- [ ] **Step 1: Edit the proto schema**

In `proto/engram/v1/engram.proto`, extend `Memory`:

```protobuf
  google.protobuf.Timestamp created_at = 14;
  string summary = 15;
  string summary_source = 16;
```

Extend the two request messages:

```protobuf
message SearchMemoriesRequest {
  string query = 1;
  string scope = 2;
  uint64 k = 3;
  repeated string tags = 4; // empty = all; non-empty = records carrying ALL listed tags (AND)
  bool full = 5;            // false (default) returns summary-shaped memories (content cleared); true returns full content
}
```

```protobuf
message ListMemoriesRequest {
  string scope = 1;
  uint64 limit = 2;
  uint64 offset = 3;
  repeated string categories = 4;
  string visibility = 5;
  repeated string tags = 6;
  bool full = 7;            // false (default) returns summary-shaped memories (content cleared); true returns full content
}
```

- [ ] **Step 2: Regenerate stubs and verify no drift escapes**

Run: `task proto:lint && task proto:gen`
Expected: regenerates `gen/go/...` and `gen/ts/...` + `ui/src/lib/gen/...`. Then `git status` (read-only) shows the regenerated files changed.

- [ ] **Step 3: Write the failing parity test**

Add to `internal/server/connectapi_test.go`:

```go
func TestMemoryToProtoMapsSummary(t *testing.T) {
	pb := memoryToProto(store.Memory{ID: "1", Summary: "terse", SummarySource: "auto"})
	if pb.Summary != "terse" || pb.SummarySource != "auto" {
		t.Fatalf("summary fields not mapped: %+v", pb)
	}
}

func TestShapeProtoMemoriesFullFlag(t *testing.T) {
	ms := []store.Memory{{ID: "1", Content: "long body over the cap here", Summary: "kept", SummarySource: "client"}}
	full := shapeProtoMemories(ms, true, 4)
	if full[0].Content == "" {
		t.Fatal("full=true must keep content")
	}
	compact := shapeProtoMemories(ms, false, 4)
	if compact[0].Content != "" || compact[0].Summary != "kept" {
		t.Fatalf("full=false must clear content and keep summary: %+v", compact[0])
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/server/ -run 'TestMemoryToProtoMapsSummary|TestShapeProtoMemoriesFullFlag' -v`
Expected: FAIL — `pb.Summary` (after regen exists) but `shapeProtoMemories` undefined.

- [ ] **Step 5: Map the fields and add proto shaping**

In `connectapi.go`, extend `memoryToProto`:

```go
func memoryToProto(m store.Memory) *engramv1.Memory {
	return &engramv1.Memory{
		Id: m.ID, Content: m.Content, Scope: m.Scope,
		Repo: m.Repo, Workspace: m.Workspace, Worktree: m.Worktree, BaseDir: m.BaseDir,
		Source: m.Source, Category: m.Category, Tags: m.Tags,
		Actor: m.Actor, Owner: m.Owner, Visibility: m.Visibility,
		CreatedAt:     timestamppb.New(m.CreatedAt),
		Summary:       m.Summary,
		SummarySource: m.SummarySource,
	}
}

// shapeProtoMemories mirrors the MCP recall contract over the Connect wire: when
// not full, clear Content and surface a summary-or-truncation so default callers
// pay summary-sized payloads. Callers opt into full content with full=true.
func shapeProtoMemories(ms []store.Memory, full bool, maxChars int) []*engramv1.Memory {
	out := make([]*engramv1.Memory, len(ms))
	for i, m := range ms {
		pb := memoryToProto(m)
		if !full {
			summary, _ := summaryOrTruncation(m, maxChars)
			pb.Content = ""
			pb.Summary = summary
		}
		out[i] = pb
	}
	return out
}
```

Add a small shared helper to `internal/server/summary.go` (so MCP and Connect agree):

```go
// summaryOrTruncation is the value the recall path shows in place of content:
// the stored summary, else a truncation. The bool reports truncation.
func summaryOrTruncation(m store.Memory, maxChars int) (string, bool) {
	if m.Summary != "" {
		return m.Summary, false
	}
	return truncateForRecall(m.Content, maxChars)
}
```

(Optionally refactor `toRecallView` to call `summaryOrTruncation` — DRY, no behavior change.)

- [ ] **Step 6: Honor `full` in the Connect handlers**

In `ListMemories`, replace the response line:

```go
	return connect.NewResponse(&engramv1.ListMemoriesResponse{
		Memories: shapeProtoMemories(ms, req.Msg.Full, a.d.summaryMaxChars), Total: total, Approximate: approximate,
	}), nil
```

In `SearchMemories`, replace the response line:

```go
	return connect.NewResponse(&engramv1.SearchMemoriesResponse{
		Memories: shapeProtoMemories(ms, req.Msg.Full, a.d.summaryMaxChars),
	}), nil
```

(`a.d` is the `*deps`, which now carries `summaryMaxChars` from Task 7.)

- [ ] **Step 7: Run tests + lint the generated tree**

Run: `go test ./internal/server/ -v && task proto:lint`
Expected: PASS. Confirm `gen/` is committed (CI `buf` job checks drift).

- [ ] **Step 8: Commit (including regenerated `gen/`)**

`jj commit -m "feat(api): Connect summary fields + full opt-in recall parity (engram-cly5)"`

---

### Task 9: CLI — `engram summarize-missing`

**Files:**

- Create: `cmd/engram/summarize.go`
- Modify: `internal/server/tools.go` (add `StoreAndSummarizerFromEnv` + `summarizerFromConfig`)
- Test: `cmd/engram/summarize_test.go`

- [ ] **Step 1: Write the failing pure-rendering test**

Create `cmd/engram/summarize_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

func TestSummarizeSummary(t *testing.T) {
	dry := summarizeSummary(store.SummarizeResult{Scanned: 10, Filled: 4, Skipped: 6}, true)
	if !strings.Contains(dry, "dry-run") || !strings.Contains(dry, "4") {
		t.Fatalf("dry-run wording: %q", dry)
	}
	live := summarizeSummary(store.SummarizeResult{Scanned: 10, Filled: 4, Skipped: 5, Failed: 1}, false)
	if strings.Contains(live, "dry-run") || !strings.Contains(live, "1 failed") {
		t.Fatalf("live wording: %q", live)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/engram/ -run TestSummarizeSummary -v`
Expected: FAIL — `summarizeSummary` undefined.

- [ ] **Step 3: Add the server builder**

In `internal/server/tools.go`, add (near `embedderFromConfig` / `summaryMaxChars`):

```go
// summarizerFromConfig builds the chat-completions summarizer from config.
func summarizerFromConfig(cfg *config.Config) *summarize.Client {
	return summarize.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Summarize.Model, summaryMaxChars(cfg),
		summarize.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)))
}

// StoreAndSummarizerFromEnv builds the store + summarizer + cap for the
// summarize-missing command. Errors when ENGRAM_SUMMARY_MODEL is unset
// (auto-summary disabled).
func StoreAndSummarizerFromEnv() (*store.Store, *summarize.Client, int, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, nil, 0, err
	}
	if cfg.Summarize.Model == "" {
		return nil, nil, 0, fmt.Errorf("ENGRAM_SUMMARY_MODEL is empty: auto-summary is disabled")
	}
	st, err := ensureStoreFromConfig(cfg)
	if err != nil {
		return nil, nil, 0, err
	}
	return st, summarizerFromConfig(cfg), summaryMaxChars(cfg), nil
}
```

(Add `"github.com/seanb4t/engram/internal/summarize"` to the `tools.go` imports. `loadAndValidate` and `ensureStoreFromConfig` already exist.)

- [ ] **Step 4: Write the command**

Create `cmd/engram/summarize.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
)

var (
	summarizeScope     string
	summarizeAllScopes bool
	summarizeOlderThan time.Duration
	summarizeLimit     int
	summarizeDryRun    bool
	summarizeTimeout   time.Duration
)

// summarizeMissingCmd fills empty recall summaries via the configured cheap
// model (ENGRAM_SUMMARY_MODEL). Operator-run, off the write path; mirrors
// reindex/prune-expired. Records that already have a summary or whose content is
// shorter than ENGRAM_SUMMARY_MAX_CHARS are skipped.
var summarizeMissingCmd = &cobra.Command{
	Use:   "summarize-missing",
	Short: "Fill empty recall summaries with the configured cheap model",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if summarizeScope == "" && !summarizeAllScopes {
			return fmt.Errorf("--scope <scope> or --all-scopes is required")
		}
		st, sm, maxChars, err := server.StoreAndSummarizerFromEnv()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if summarizeTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, summarizeTimeout)
			defer cancel()
		}
		var older time.Time
		if summarizeOlderThan > 0 {
			older = time.Now().UTC().Add(-summarizeOlderThan)
		}
		res, err := st.SummarizeMissing(ctx, store.SummarizeOptions{
			Scope:     summarizeScope,
			AllScopes: summarizeAllScopes,
			OlderThan: older,
			Limit:     summarizeLimit,
			MaxChars:  maxChars,
			Model:     summarizeModel(),
			DryRun:    summarizeDryRun,
		}, sm.Summarize)
		if err != nil {
			return err
		}
		cmd.Println(summarizeSummary(res, summarizeDryRun))
		return nil
	},
}

// summarizeModel echoes the configured model name for the result stamp. Read
// from the env directly (the builder already validated it is non-empty).
func summarizeModel() string { return os.Getenv("ENGRAM_SUMMARY_MODEL") }

// summarizeSummary renders the operator-facing one-line result. Pure (no I/O) so
// the dry-run vs live wording is unit-testable without a live gateway.
func summarizeSummary(res store.SummarizeResult, dryRun bool) string {
	if dryRun {
		return fmt.Sprintf("dry-run: %d of %d scanned record(s) would be summarized (%d skipped)",
			res.Filled, res.Scanned, res.Skipped)
	}
	return fmt.Sprintf("summarized %d of %d scanned record(s); %d skipped, %d failed",
		res.Filled, res.Scanned, res.Skipped, res.Failed)
}

func init() {
	summarizeMissingCmd.Flags().StringVar(&summarizeScope, "scope", "", "only summarize records in this scope")
	summarizeMissingCmd.Flags().BoolVar(&summarizeAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted)")
	summarizeMissingCmd.Flags().DurationVar(&summarizeOlderThan, "older-than", 0, "only records created at least this long ago (0 = any age)")
	summarizeMissingCmd.Flags().IntVar(&summarizeLimit, "limit", 0, "max records to scan (0 = no cap)")
	summarizeMissingCmd.Flags().BoolVar(&summarizeDryRun, "dry-run", false, "count eligible records without writing")
	summarizeMissingCmd.Flags().DurationVar(&summarizeTimeout, "timeout", 30*time.Minute, "max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(summarizeMissingCmd)
}
```

- [ ] **Step 5: Run test to verify it passes + build**

Run: `go test ./cmd/engram/ -run TestSummarizeSummary -v && go build ./cmd/engram`
Expected: PASS + clean build.

- [ ] **Step 6: Commit**

`jj commit -m "feat(cli): summarize-missing command for auto-summary backfill (engram-cly5)"`

---

### Task 10: Web UI — preserve console behavior under summary-by-default

**Files:**

- Modify: the SvelteKit recall call sites under `ui/src/` that call `SearchMemories` / `ListMemories` (regenerated client at `ui/src/lib/gen/`)
- (Generated client already updated by Task 8's `task proto:gen`)

- [ ] **Step 1: Locate the recall call sites**

Run: `mcp__probe__grep "listMemories|searchMemories|ListMemories|SearchMemories" ui/src`
Identify each place the console fetches memories for display (list view, search view, scope detail).

- [ ] **Step 2: Set `full: true` on console recall calls**

For each call site, add `full: true` to the request message so the operator console keeps showing full content (the console is a human-facing inspector — truncation would regress it). Example shape (match the actual client usage in the file):

```ts
// before
const res = await client.listMemories({ scope, limit });
// after — console shows full records, not recall summaries
const res = await client.listMemories({ scope, limit, full: true });
```

Apply the same `full: true` addition to the `searchMemories` call(s).

- [ ] **Step 3: Verify the UI builds**

Run: `task ui:build` (or the repo's UI build target — check `Taskfile.yaml` for `ui:*`).
Expected: clean build; the `full` field exists on the regenerated request types.

- [ ] **Step 4: Commit**

`jj commit -m "fix(ui): request full content on console recall after summary-by-default (engram-cly5)"`

---

### Task 11: Docs + skill updates

**Files:**

- Modify: `skill/engram/skills/curating-memory/SKILL.md`
- Modify: `skill/engram/hooks/session-start-memory-recall` (bootstrap recall guidance) + its test `skill/engram/hooks/tests/test_session_start_memory_recall.py` if the emitted text is asserted
- Modify: `docs-site/src/content/docs/reference/tools.md`
- Modify: `docs-site/src/content/docs/reference/memory-record.md`
- Modify: `CLAUDE.md` (Memory contract section)

- [ ] **Step 1: curating-memory skill — summary guidance**

Add a short "Summaries" subsection to `skill/engram/skills/curating-memory/SKILL.md`: when to pass `summary` on `store_memory` (one terse line preserving negations/identifiers); that recall returns summaries by default and an agent should `get_memory` (or `full=true`) before acting on caveats — especially when `summary_source=auto` (lossy); and the `update_memory` rule that changing content with a caller-authored summary present requires addressing the summary (re-send / update / clear) or the update is rejected.

- [ ] **Step 2: bootstrap recall guidance — note the new shape**

Run: `mcp__probe__grep "list_memory|merge|memories" skill/engram/hooks/session-start-memory-recall`
Update the emitted guidance to note recall now returns compact summaries by default (this is what keeps the spine bootstrap small), and that full content is one `get_memory` away. If `skill/engram/hooks/tests/test_session_start_memory_recall.py` asserts the exact emitted string, update the expected text and run:
`uv run --with pytest pytest skill/engram/hooks/tests -q`

- [ ] **Step 3: docs-site reference — tools + record**

In `docs-site/src/content/docs/reference/tools.md`: document the new `summary` arg on `store_memory` and `update_memory` (with the stale-summary rule), the `full` arg on `search_memory`/`list_memory`, and a new `summarize-missing` CLI entry. In `docs-site/src/content/docs/reference/memory-record.md`: add `summary`, `summary_source`, `summary_model` to the field tables.

- [ ] **Step 4: CLAUDE.md — memory contract**

In the "Memory contract (stable)" section of `CLAUDE.md`, add the `summary`/`summary_source` fields and one sentence that recall returns summaries by default with `full` opt-in; full content via `get_memory`.

- [ ] **Step 5: Lint docs + run hook tests**

Run: `task lint:markdown && uv run --with pytest pytest skill/engram/hooks/tests -q`
Expected: clean (note `docs-site/**` is excluded from rumdl; `skill/**` and root `*.md` are linted).

- [ ] **Step 6: Commit**

`jj commit -m "docs(summary): document summary fields, full recall, summarize-missing (engram-cly5)"`

---

### Task 12: Fidelity eval — does the cheap model preserve caveats?

**Files:**

- Create: `internal/summarize/fidelity_test.go` (gated on `ENGRAM_SUMMARY_EVAL=1` + a configured live model)
- Modify: `Taskfile.yaml` (add `eval:summary` target)

- [ ] **Step 1: Write the fidelity harness (skips unless explicitly enabled)**

Create `internal/summarize/fidelity_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package summarize

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
)

// fidelityCase pairs realistic caveat-heavy content with tokens the summary MUST
// preserve (negations, identifiers, numbers). Expand with anonymized real memories.
type fidelityCase struct {
	name        string
	content     string
	mustContain []string
}

var fidelityCases = []fidelityCase{
	{
		name:        "decline-suggestion",
		content:     "The #_top selector beats the h1 rules directly. Review bots suggest `#_top h1` which matches NOTHING and would strip the styling. DECLINE that suggestion.",
		mustContain: []string{"DECLINE", "#_top"},
	},
	{
		name:        "dep-type-flag",
		content:     "bd dep add <epic> <decision> with the default blocks type is REJECTED. Use --type related for the provenance edge, never blocks.",
		mustContain: []string{"--type related", "never", "blocks"},
	},
}

// TestSummaryFidelity runs the configured cheap model over the cases and reports
// a preservation score. Gate: set ENGRAM_SUMMARY_EVAL=1 plus ENGRAM_OPENAI_BASE_URL,
// ENGRAM_OPENAI_API_KEY, ENGRAM_SUMMARY_MODEL, ENGRAM_SUMMARY_MAX_CHARS.
func TestSummaryFidelity(t *testing.T) {
	if os.Getenv("ENGRAM_SUMMARY_EVAL") != "1" {
		t.Skip("set ENGRAM_SUMMARY_EVAL=1 (and the gateway/model env) to run the fidelity eval")
	}
	maxChars, _ := strconv.Atoi(os.Getenv("ENGRAM_SUMMARY_MAX_CHARS"))
	if maxChars <= 0 {
		maxChars = 280
	}
	c := New(os.Getenv("ENGRAM_OPENAI_BASE_URL"), os.Getenv("ENGRAM_OPENAI_API_KEY"), os.Getenv("ENGRAM_SUMMARY_MODEL"), maxChars)

	var checks, passed int
	for _, tc := range fidelityCases {
		sum, err := c.Summarize(context.Background(), tc.content)
		if err != nil {
			t.Errorf("%s: summarize error: %v", tc.name, err)
			continue
		}
		for _, tok := range tc.mustContain {
			checks++
			if strings.Contains(sum, tok) {
				passed++
			} else {
				t.Errorf("%s: summary dropped %q\n  summary: %s", tc.name, tok, sum)
			}
		}
	}
	t.Logf("fidelity: %d/%d required tokens preserved", passed, checks)
}
```

- [ ] **Step 2: Verify it skips by default**

Run: `go test ./internal/summarize/ -run TestSummaryFidelity -v`
Expected: SKIP (no `ENGRAM_SUMMARY_EVAL`).

- [ ] **Step 3: Add the task target**

In `Taskfile.yaml`, add:

```yaml
  eval:summary:
    desc: Score whether the configured summary model preserves caveats (needs a live gateway+model)
    cmds:
      - ENGRAM_SUMMARY_EVAL=1 go test ./internal/summarize/ -run TestSummaryFidelity -v
```

- [ ] **Step 4: Commit**

`jj commit -m "test(summarize): caveat-preservation fidelity eval + task target (engram-cly5)"`

---

## Final verification

- [ ] Run `task` (lint + test) and confirm green (store integration tests need Docker/Qdrant; CI provides it).
- [ ] Run `task license:check` — every new `internal/**` and `cmd/**` Go file carries the SPDX header.
- [ ] Run `task proto:lint` and confirm `gen/` has no uncommitted drift (`buf` CI job parity).
- [ ] Confirm the 7 required CI checks (test, golangci-lint, commit-lint, license headers, helm chart, actionlint, python) will pass — no job renamed, no workflow-level path filter added.

## Spec coverage map

| Spec section | Task(s) |
|---|---|
| Data model (summary + provenance, zero migration) | 1 |
| Summarizer client (`internal/summarize`) | 2 |
| Config (`SummarizeConfig`, presence-enable, validate) | 3 |
| Core fill seam (`SetSummary`/`FillSummary`/`SummarizeMissing`) | 4 |
| Explicit write path (`store_memory` summary) | 6 |
| Stale-on-edit fail-loud guard (`update_memory`) | 5 |
| Recall shaping (summary-by-default, `full` opt-in, MCP) | 7 |
| MCP tool + Connect/proto parity | 6, 7, 8 |
| Web UI update | 10 |
| Skill/plugin/docs scope | 11 |
| Validation fidelity eval | 12 |
| Offline CLI execution (`summarize-missing`) | 9 |
| Async-on-write queue (future seam only) | 4 (`FillSummary` reusable; not built) |

<!-- adr-capture: sha256=e6f40bc7ddc52b7e; session=cli; ts=2026-06-26T12:10:07Z; adrs=engram-4y7p,engram-ambu,engram-ddiw -->
