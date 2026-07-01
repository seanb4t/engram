<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Asymmetric / cloud embedder param passthrough — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let engram embed queries and documents asymmetrically for cloud/gateway embedders (via a provider-agnostic request-body param) and for self-hosted both-side-prefix models (via a document-side text instruction), without changing default behavior.

**Architecture:** Two orthogonal, opt-in mechanisms on `embed.Client`, composing with the query text-instruction shipped in PR #262. (1) `queryParams`/`documentParams` maps merged into the `/v1/embeddings` request body (`EmbedQuery` merges query params, `Embed` merges document params); a two-path body build keeps the empty/default case byte-for-byte unchanged. (2) `documentInstruction`, the document-side mirror of the query instruction, applied by `Embed` at store + reindex. Query-side knobs are hot; document-side knobs (and #262's tags-in-vector) require a reindex.

**Tech Stack:** Go, koanf config (`internal/config`), OpenAI-compatible embeddings client (`internal/embed`), Helm (`charts/engram`), Starlight docs (`docs-site`).

**Spec:** `docs/superpowers/specs/2026-07-01-asymmetric-cloud-embedder-params-design.md`

**Design bead:** `engram-0qed` · **Umbrella feature:** `engram-wd89.1`

---

## Files touched

- `internal/config/embedparams.go` (create) — `ParseEmbedParams` JSON-object parse + reserved-key guard.
- `internal/config/embedparams_test.go` (create) — its tests.
- `internal/config/config.go` (modify) — `EmbedConfig` gains `QueryParams`, `DocumentParams`, `DocumentInstruction` (all `string`).
- `internal/config/registry.go` (modify) — three new registry keys.
- `internal/config/validate.go` (modify) — validate the two `*_PARAMS` via `ParseEmbedParams`.
- `internal/config/validate_test.go` (modify) — validation cases for the params.
- `internal/embed/embed.go` (modify) — client fields, options, two-path body build, `kind` telemetry, document instruction.
- `internal/embed/embed_test.go` (modify) — params + document-instruction tests.
- `internal/server/tools.go` (modify) — `embedderFromConfig` passes the three new options.
- `charts/engram/values.yaml` (modify) — `memory.embed.{queryParams,documentParams,documentInstruction}`.
- `charts/engram/templates/memory-mcp.yaml` (modify) — emit the three env vars when non-empty.
- `docs-site/src/content/docs/guides/embedding-instructions.md` (modify) — cloud `*_PARAMS` values, E5/nomic `document_instruction`, hot-vs-reindex callout.

---

### Task 1: Config — `ParseEmbedParams` helper

The single source of truth for parsing an `ENGRAM_EMBED_*_PARAMS` value into a body-param map. Empty is a valid no-op; non-empty must be a JSON **object**; the reserved keys `model`/`input` are rejected. Used by both `Validate()` (Task 2) and `embedderFromConfig` (Task 5).

**Files:**

- Create: `internal/config/embedparams.go`
- Test: `internal/config/embedparams_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/embedparams_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import "testing"

func TestParseEmbedParams(t *testing.T) {
	t.Run("empty is a valid no-op", func(t *testing.T) {
		m, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", "")
		if err != nil {
			t.Fatalf("empty: unexpected error %v", err)
		}
		if m != nil {
			t.Fatalf("empty: got %v, want nil map", m)
		}
	})

	t.Run("valid object parses", func(t *testing.T) {
		m, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", `{"input_type":"search_query"}`)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if m["input_type"] != "search_query" {
			t.Errorf("got %v, want input_type=search_query", m)
		}
	})

	// null, arrays, and scalars are valid JSON but not objects — all rejected.
	for _, bad := range []string{`not json`, `null`, `[1,2]`, `"str"`, `42`, `true`} {
		t.Run("rejects non-object: "+bad, func(t *testing.T) {
			if _, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", bad); err == nil {
				t.Errorf("%s: expected error, got nil", bad)
			}
		})
	}

	for _, k := range []string{"model", "input"} {
		t.Run("rejects reserved key "+k, func(t *testing.T) {
			if _, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", `{"`+k+`":"x"}`); err == nil {
				t.Errorf("reserved key %s: expected error, got nil", k)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestParseEmbedParams`
Expected: FAIL — build error `undefined: ParseEmbedParams`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/config/embedparams.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"encoding/json"
	"fmt"
)

// ParseEmbedParams parses an ENGRAM_EMBED_*_PARAMS value (name is the env var,
// used in errors) into a request-body param map. An empty string is a valid
// no-op and returns (nil, nil). A non-empty value must be a JSON object; the
// reserved keys "model" and "input" are rejected because the embedder sets them
// authoritatively and operator params must not override them. `null`, arrays,
// and scalars are valid JSON but not objects and are rejected.
func ParseEmbedParams(name, s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("%s: must be a JSON object: %w", name, err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: must be a JSON object, got %T", name, v)
	}
	for _, k := range []string{"model", "input"} {
		if _, exists := m[k]; exists {
			return nil, fmt.Errorf("%s: must not contain reserved key %q", name, k)
		}
	}
	return m, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestParseEmbedParams`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(config): ParseEmbedParams JSON-object parse + reserved-key guard (engram-0qed)"`

---

### Task 2: Config — `EmbedConfig` fields, registry keys, and validation

**Files:**

- Modify: `internal/config/config.go` (EmbedConfig struct)
- Modify: `internal/config/registry.go` (after the `embed.query_instruction` entry)
- Modify: `internal/config/validate.go` (inside `Validate`, after the `ENGRAM_OPENAI_BASE_URL` switch, before the summarize block)
- Test: `internal/config/validate_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/validate_test.go`:

```go
func TestValidateEmbedParams(t *testing.T) {
	// Empty (the default) is valid — validConfig leaves these unset.
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("empty embed params: Validate() = %v, want nil", err)
	}

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"query_params not an object", func(c *Config) { c.Embed.QueryParams = `"nope"` }, "ENGRAM_EMBED_QUERY_PARAMS"},
		{"query_params invalid json", func(c *Config) { c.Embed.QueryParams = `{bad` }, "ENGRAM_EMBED_QUERY_PARAMS"},
		{"query_params reserved key", func(c *Config) { c.Embed.QueryParams = `{"model":"x"}` }, "reserved key"},
		{"document_params not an object", func(c *Config) { c.Embed.DocumentParams = `[1]` }, "ENGRAM_EMBED_DOCUMENT_PARAMS"},
		{"document_params valid is ok", func(c *Config) { c.Embed.DocumentParams = `{"input_type":"search_document"}` }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(c)
			err := c.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() error = %q, want substring %q", err, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestValidateEmbedParams`
Expected: FAIL — build error `c.Embed.QueryParams undefined` (struct field missing).

- [ ] **Step 3a: Add the struct fields**

In `internal/config/config.go`, replace the `EmbedConfig` struct (currently `Model`, `Dim`, `QueryInstruction`) — add three fields after `QueryInstruction`:

```go
	// QueryParams / DocumentParams are JSON objects (as raw strings) merged into
	// the /v1/embeddings request body for query vs document embeds respectively —
	// e.g. {"input_type":"search_query"}. Empty = none. Parsed/validated in
	// Config.Validate() and again in the embedder builder (cf. Dim), never
	// coerced by the koanf unmarshal.
	QueryParams    string `koanf:"query_params"`
	DocumentParams string `koanf:"document_params"`
	// DocumentInstruction is the document-side mirror of QueryInstruction: a text
	// prefix/template applied to documents at store + reindex (empty = raw).
	DocumentInstruction string `koanf:"document_instruction"`
```

- [ ] **Step 3b: Add the registry keys**

In `internal/config/registry.go`, add after the `embed.query_instruction` line:

```go
	{Key: "embed.query_params", Env: "ENGRAM_EMBED_QUERY_PARAMS"},
	{Key: "embed.document_params", Env: "ENGRAM_EMBED_DOCUMENT_PARAMS"},
	{Key: "embed.document_instruction", Env: "ENGRAM_EMBED_DOCUMENT_INSTRUCTION"},
```

- [ ] **Step 3c: Add the validation**

In `internal/config/validate.go`, inside `Validate`, immediately after the `ENGRAM_OPENAI_BASE_URL` switch closes (before the `if c.Summarize.Model != ""` block), add:

```go
	// Query/document params: empty is a valid no-op (self-gated); a non-empty
	// value must be a JSON object with no reserved keys. Only well-formedness is
	// checked here — Load stays assembly-only (ADR engram-wtw).
	if _, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", c.Embed.QueryParams); err != nil {
		errs = append(errs, err)
	}
	if _, err := ParseEmbedParams("ENGRAM_EMBED_DOCUMENT_PARAMS", c.Embed.DocumentParams); err != nil {
		errs = append(errs, err)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/`
Expected: PASS (TestValidateEmbedParams and all existing config tests).

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(config): embed query/document params + document instruction keys (engram-0qed)"`

---

### Task 3: `embed.Client` — request-body param passthrough (two-path build)

Add `queryParams`/`documentParams` to the client, thread them into the request body via a two-path build (empty → the existing struct marshal, exact prior bytes; non-empty → a merged map with `model`/`input` applied last), and stamp an `engram.embed.kind` span attribute.

**Files:**

- Modify: `internal/embed/embed.go`
- Test: `internal/embed/embed_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/embed/embed_test.go` (a helper that captures the whole decoded body, plus the behavior test):

```go
// captureBody records the full decoded request body of each embeddings request.
func captureBody(t *testing.T, got *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		*got = body
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1}}},
		})
	}))
}

func TestEmbedParamsMergedIntoBody(t *testing.T) {
	t.Run("query params merged by EmbedQuery", func(t *testing.T) {
		var got map[string]any
		srv := captureBody(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithQueryParams(map[string]any{"input_type": "search_query"}))
		if _, err := c.EmbedQuery(context.Background(), "hello"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		if got["input_type"] != "search_query" || got["model"] != "m" || got["input"] != "hello" {
			t.Errorf("body = %v; want input_type=search_query, model=m, input=hello", got)
		}
	})

	t.Run("document params merged by Embed", func(t *testing.T) {
		var got map[string]any
		srv := captureBody(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithDocumentParams(map[string]any{"input_type": "search_document"}))
		if _, err := c.Embed(context.Background(), "doc"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if got["input_type"] != "search_document" || got["input"] != "doc" {
			t.Errorf("body = %v; want input_type=search_document, input=doc", got)
		}
	})

	t.Run("reserved keys cannot be clobbered by params", func(t *testing.T) {
		var got map[string]any
		srv := captureBody(t, &got)
		defer srv.Close()
		// Even a caller that bypasses config validation cannot override model/input.
		c := New(srv.URL, "k", "m", WithQueryParams(map[string]any{"model": "evil", "input": "evil"}))
		if _, err := c.EmbedQuery(context.Background(), "real"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		if got["model"] != "m" || got["input"] != "real" {
			t.Errorf("body = %v; model/input must be authoritative", got)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/embed/ -run TestEmbedParamsMergedIntoBody`
Expected: FAIL — build error `undefined: WithQueryParams` / `WithDocumentParams`.

- [ ] **Step 3a: Add client fields**

In `internal/embed/embed.go`, in the `Client` struct add after `queryInstruction string`:

```go
	documentInstruction string
	queryParams         map[string]any
	documentParams      map[string]any
```

- [ ] **Step 3b: Add the options**

After `WithQueryInstruction` in `internal/embed/embed.go`, add:

```go
// WithQueryParams sets request-body params merged into query embeds (EmbedQuery),
// e.g. {"input_type":"search_query"} for OpenRouter/Cohere. Reserved keys
// "model"/"input" in the map are ignored (applied last, always authoritative).
func WithQueryParams(params map[string]any) Option {
	return func(c *Client) { c.queryParams = params }
}

// WithDocumentParams sets request-body params merged into document embeds (Embed),
// e.g. {"input_type":"search_document"}.
func WithDocumentParams(params map[string]any) Option {
	return func(c *Client) { c.documentParams = params }
}
```

- [ ] **Step 3c: Thread params + kind through the call path**

In `internal/embed/embed.go`:

Change `Embed` to pass its params and kind:

```go
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, text, c.documentParams, "document")
}
```

Change the last line of `EmbedQuery` from `return c.embed(ctx, text)` to:

```go
	return c.embed(ctx, text, c.queryParams, "query")
```

Change the `embed` method signature and body build. Replace the signature line and the `body, _ := json.Marshal(...)` line:

```go
func (c *Client) embed(ctx context.Context, text string, params map[string]any, kind string) (vec []float32, err error) {
	ctx, span := tracer.Start(ctx, "embed.Embed", trace.WithAttributes(
		attribute.String("engram.embed.model", c.model),
		attribute.String("engram.embed.kind", kind),
	))
```

and, replacing the single `body, _ := json.Marshal(embedReq{Model: c.model, Input: text})` line:

```go
	// Empty params → marshal the struct (exact prior wire bytes; default path).
	// Non-empty → merge params first, then set model/input last so they are
	// always authoritative. Go sorts map keys on marshal; that is JSON-
	// semantically identical, so callers compare decoded objects, not raw bytes.
	var body []byte
	if len(params) == 0 {
		body, _ = json.Marshal(embedReq{Model: c.model, Input: text})
	} else {
		m := make(map[string]any, len(params)+2)
		for k, v := range params {
			m[k] = v
		}
		m["model"] = c.model
		m["input"] = text
		body, _ = json.Marshal(m)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/embed/`
Expected: PASS (new test + all existing embed tests, including the #262 `TestEmbedQueryInstruction` and `captureInput` tests).

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(embed): request-body param passthrough for query/document embeds (engram-0qed)"`

---

### Task 4: `embed.Client` — document-side instruction

The document-side mirror of the query instruction. `Embed` wraps the document text: empty → raw; contains `{document}` → literal template; else → prefix.

**Files:**

- Modify: `internal/embed/embed.go`
- Test: `internal/embed/embed_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/embed/embed_test.go`:

```go
func TestEmbedDocumentInstruction(t *testing.T) {
	t.Run("placeholder substituted", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithDocumentInstruction("search_document: {document}"))
		if _, err := c.Embed(context.Background(), "the fox"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if got != "search_document: the fox" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("no placeholder is a prefix", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithDocumentInstruction("passage: "))
		if _, err := c.Embed(context.Background(), "the fox"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if got != "passage: the fox" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("empty leaves document raw and queries are never wrapped by it", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithDocumentInstruction("passage: "))
		if _, err := c.EmbedQuery(context.Background(), "a query"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		if got != "a query" {
			t.Errorf("document instruction must not affect queries; got %q", got)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/embed/ -run TestEmbedDocumentInstruction`
Expected: FAIL — build error `undefined: WithDocumentInstruction`.

- [ ] **Step 3a: Add the option + placeholder const**

In `internal/embed/embed.go`, near `queryPlaceholder`, add:

```go
// documentPlaceholder, when present in the document instruction, is replaced by
// the raw document text (e.g. "search_document: {document}"); otherwise the
// instruction is prepended as a prefix.
const documentPlaceholder = "{document}"

// WithDocumentInstruction sets the document-side text applied by Embed: empty =
// raw; contains "{document}" = literal template; otherwise prepended as a prefix
// (e.g. "passage: "). For both-side-prefix models like E5 / nomic. Changing it
// alters stored document vectors, so it requires a reindex to take effect.
func WithDocumentInstruction(instruction string) Option {
	return func(c *Client) { c.documentInstruction = instruction }
}
```

- [ ] **Step 3b: Wrap the document text in `Embed`**

In `internal/embed/embed.go`, replace the body of `Embed`:

```go
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	switch {
	case c.documentInstruction == "":
		// raw
	case strings.Contains(c.documentInstruction, documentPlaceholder):
		text = strings.ReplaceAll(c.documentInstruction, documentPlaceholder, text)
	default:
		text = c.documentInstruction + text
	}
	return c.embed(ctx, text, c.documentParams, "document")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/embed/`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(embed): document-side instruction for both-side-prefix models (engram-0qed)"`

---

### Task 5: Wire `embedderFromConfig`

Parse the config param strings and pass all three new options. Parsing cannot fail here because `Config.Validate()` (via `loadAndValidate`) already ran — the same validated-upstream pattern `storeFromConfig` uses for `Dim`.

**Files:**

- Modify: `internal/server/tools.go` (`embedderFromConfig`, currently at ~line 202)

- [ ] **Step 1: Write the failing test**

Add to `internal/server/tools_test.go`:

```go
func TestEmbedderFromConfigPassesParamsAndInstructions(t *testing.T) {
	cfg := &config.Config{
		OpenAI: config.OpenAIConfig{BaseURL: "http://x", APIKey: "k"},
		Embed: config.EmbedConfig{
			Model:               "m",
			QueryParams:         `{"input_type":"search_query"}`,
			DocumentParams:      `{"input_type":"search_document"}`,
			DocumentInstruction: "passage: ",
		},
	}
	// Must construct without panic and yield a usable client (smoke: nil-safe).
	if em := embedderFromConfig(cfg); em == nil {
		t.Fatal("embedderFromConfig returned nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestEmbedderFromConfigPassesParamsAndInstructions`
Expected: FAIL to compile only if fields are missing; otherwise PASS trivially. If it already passes (fields exist from Task 2, options from Tasks 3-4), that is expected — this is a smoke guard. Proceed to Step 3 to add the wiring that makes the params actually take effect.

- [ ] **Step 3: Add the wiring**

In `internal/server/tools.go`, replace `embedderFromConfig`:

```go
func embedderFromConfig(cfg *config.Config) *embed.Client {
	// Validated upstream in loadAndValidate → Config.Validate(); errors are
	// unreachable here (same pattern as storeFromConfig for Dim).
	queryParams, _ := config.ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", cfg.Embed.QueryParams)
	documentParams, _ := config.ParseEmbedParams("ENGRAM_EMBED_DOCUMENT_PARAMS", cfg.Embed.DocumentParams)
	return embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Embed.Model,
		embed.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
		embed.WithQueryInstruction(cfg.Embed.QueryInstruction),
		embed.WithDocumentInstruction(cfg.Embed.DocumentInstruction),
		embed.WithQueryParams(queryParams),
		embed.WithDocumentParams(documentParams))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestEmbedderFromConfigPassesParamsAndInstructions` then `go build ./...`
Expected: PASS; build clean.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(server): wire embed query/document params + document instruction from config (engram-0qed)"`

---

### Task 6: Helm chart

**Files:**

- Modify: `charts/engram/values.yaml` (the `memory.embed` block)
- Modify: `charts/engram/templates/memory-mcp.yaml` (after the `ENGRAM_EMBED_DIM` env line, near the `queryInstruction` guard)

- [ ] **Step 1: Add the values**

In `charts/engram/values.yaml`, under `memory.embed` (after the existing `queryInstruction` key), add:

```yaml
    # JSON strings merged into the /v1/embeddings request body for query vs
    # document embeds (cloud models: input_type / task_type). Empty = none.
    # e.g. queryParams: '{"input_type":"search_query"}'
    queryParams: ""
    documentParams: ""
    # Document-side text prefix for both-side-prefix models (E5/nomic), e.g.
    # "passage: " or "search_document: {document}". Empty = raw. Changing it
    # requires a reindex.
    documentInstruction: ""
```

- [ ] **Step 2: Emit the env vars**

In `charts/engram/templates/memory-mcp.yaml`, after the existing `ENGRAM_EMBED_QUERY_INSTRUCTION` `with` block (added in #262), add:

```yaml
            {{- with .Values.memory.embed.queryParams }}
            - { name: ENGRAM_EMBED_QUERY_PARAMS, value: {{ . | quote }} }
            {{- end }}
            {{- with .Values.memory.embed.documentParams }}
            - { name: ENGRAM_EMBED_DOCUMENT_PARAMS, value: {{ . | quote }} }
            {{- end }}
            {{- with .Values.memory.embed.documentInstruction }}
            - { name: ENGRAM_EMBED_DOCUMENT_INSTRUCTION, value: {{ . | quote }} }
            {{- end }}
```

- [ ] **Step 3: Verify template + lint**

Run: `helm template charts/engram --set memory.embed.queryParams='{"input_type":"search_query"}' | grep -A1 ENGRAM_EMBED_QUERY_PARAMS`
Expected: the env var renders with the JSON string value.

Run: `helm lint charts/engram`
Expected: `0 chart(s) failed`.

- [ ] **Step 4: Commit**

Run: `jj commit -m "feat(chart): surface embed query/document params + document instruction (engram-0qed)"`

---

### Task 7: Documentation — per-model guide

Update the guide so the cloud rows carry concrete `*_PARAMS` values, E5/nomic carry a `document_instruction`, and the hot-vs-reindex boundary is explicit.

**Files:**

- Modify: `docs-site/src/content/docs/guides/embedding-instructions.md`

- [ ] **Step 1: Replace the cloud "leave empty / set at gateway" section**

In `docs-site/src/content/docs/guides/embedding-instructions.md`, in the "Cloud models" section, replace the guidance so each provider row shows the params to set. Use this table (query side is hot; document side needs a reindex):

```markdown
| Provider / model | `ENGRAM_EMBED_QUERY_PARAMS` / `ENGRAM_EMBED_DOCUMENT_PARAMS` |
| --- | --- |
| OpenRouter, Cohere embed v3, Voyage | `{"input_type":"search_query"}` / `{"input_type":"search_document"}` |
| Jina embeddings v3 | `{"task":"retrieval.query"}` / `{"task":"retrieval.passage"}` |
| Google Gemini / Vertex | `{"task_type":"RETRIEVAL_QUERY"}` / `{"task_type":"RETRIEVAL_DOCUMENT"}` |

The gateway forwards these fields to the provider (OpenRouter accepts `input_type`
natively; LiteLLM maps provider params per model). The **document** side changes
stored vectors, so set it before indexing or run a reindex.
```

- [ ] **Step 2: Give E5/nomic a `document_instruction` row**

In the "both-side prefix" section, replace the "leave empty" note with:

```markdown
| Model | `ENGRAM_EMBED_QUERY_INSTRUCTION` (hot) | `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` (needs reindex) |
| --- | --- | --- |
| intfloat/e5-\* | `query: {query}` | `passage: {document}` |
| nomic-embed-text | `search_query: {query}` | `search_document: {document}` |

Both sides are required for these models; the document side changes stored
vectors, so set it before indexing or `engram reindex` afterward.
```

- [ ] **Step 3: Add the hot-vs-reindex callout**

Add near the top of the guide, after the intro:

```markdown
## Hot vs reindex-gated

- **Hot (no reindex):** `ENGRAM_EMBED_QUERY_INSTRUCTION`, `ENGRAM_EMBED_QUERY_PARAMS` — they change only the query vector at search time.
- **Reindex-gated:** `ENGRAM_EMBED_DOCUMENT_INSTRUCTION`, `ENGRAM_EMBED_DOCUMENT_PARAMS` (and tags-in-vector) — they change the stored document vector, so existing records need `engram reindex` + a collection cutover (see [Reindex](/guides/reindex/)).
```

- [ ] **Step 4: Commit**

Run: `jj commit -m "docs(guide): cloud embed params + E5/nomic document instruction + hot-vs-reindex (engram-0qed)"`

---

### Task 8: Full verification gate

**Files:** none (verification only)

- [ ] **Step 1: Run the full gate**

Run: `task` (lint + test) — or, if slower gates are unavailable, `go test ./... && go tool golangci-lint run ./... && task license:check`
Expected: all green — `0 issues` from golangci-lint, all packages `ok`, license `invalid: 0`.

- [ ] **Step 2: Confirm default wire shape is unchanged**

Run: `go test ./internal/embed/ -run 'TestEmbedReturnsVector|TestEmbedQueryInstruction' -v`
Expected: PASS — the empty-params default path still marshals `{model, input}` via the struct (no behavior change for existing deployments).

- [ ] **Step 3: Commit any final formatting**

Run: `task fmt` then `jj commit -m "chore: fmt (engram-0qed)"` (skip the commit if `task fmt` made no changes).

---

## Notes for the implementer

- **Reindex adoption:** document-side knobs take effect on existing records only after `engram reindex --target <new>` + repointing `ENGRAM_QDRANT_COLLECTION`. No code handles this automatically — it is an operator action (documented in Task 7).
- **Backward compatibility:** every new knob defaults empty; an unconfigured deployment sends the exact prior request body (Task 3 struct fast-path, verified in Task 8 Step 2).
- **golangci-lint:** the `queryParams, _ := config.ParseEmbedParams(...)` blank-error assignment in Task 5 is an intentional, validated-upstream ignore; `errcheck` accepts explicit `_`.
- **Reindex coverage (no new reindex test):** the spec's Testing section mentions a reindex-level test, but none is added here by design. `store.Reindex` calls the injected `EmbedFunc` — already proven by the existing `TestReindexRoundtrip` — and the real CLI passes `em.Embed` (`cmd/engram/reindex.go:77`), whose document-instruction + document-params behavior is unit-tested in Tasks 3–4. The document-side reindex path is therefore covered by composition; a dedicated integration test would be redundant.
- **Telemetry scope:** Task 3 adds only the `engram.embed.kind` string attribute. The spec's per-knob boolean "applied" flags are marked optional there and are omitted to keep the span minimal; add them later if production traces need the finer signal.
<!-- adr-capture: sha256=e8428c0bd363d227; session=cli; ts=2026-07-01T23:24:14Z; adrs=engram-zyhq -->
