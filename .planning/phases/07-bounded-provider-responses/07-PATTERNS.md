# Phase 7: Bounded Provider Responses - Pattern Map

**Mapped:** 2026-09-21
**Files analyzed:** 12 (4 existing files modified + 1 new package + docs + config)
**Analogs found:** 12 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/httpdrain/httpdrain.go` (new) | utility | streaming | `internal/testhttp/reuse.go` | role-match (shared internal helper package, non-`_test.go`, importable by both clients) |
| `internal/httpdrain/httpdrain_test.go` (new) | test | streaming | `internal/embed/embed_test.go` (`TestEmbedNon2xxDrainsForReuse`) | role-match |
| `internal/embed/embed.go` (modify: drains at :295, :309; `Option`/`New` at :73-149; `WithTimeout` :108) | service (HTTP client) | request-response | itself (existing) — sibling `internal/summarize/summarize.go` for the cross-client convention check | exact |
| `internal/summarize/summarize.go` (modify: drains at :182, :191; `Option`/`New` at :56-92; `WithTimeout` :76) | service (HTTP client) | request-response | `internal/embed/embed.go` (already the documented "direct twin") | exact |
| `internal/config/registry.go` (modify: +6 `field` entries) | config | CRUD (config registry) | itself — `memory.max_content_bytes` / `memory.max_tags` / `memory.max_tag_bytes` entries (lines 41-57) are the exact shape for "brand-new key, no Legacy, comment explains 0-handling" | exact |
| `internal/config/config.go` (modify: `EmbedConfig` :64, `SummarizeConfig` :137, new fields) | model (config struct) | CRUD | itself — `MemoryConfig` struct (lines 89-119) is the doc-comment shape for a brand-new always-vs-sometimes-enforced bound | exact |
| `internal/config/validate.go` (modify: +6 validation blocks) | middleware (validation) | request-response | itself — `embed.timeout` block (:65-73, duration+non-negative) and `validatePositiveCap`/`ENGRAM_SUMMARY_WORKERS` block (:240-245, positive-integer) are the two shapes needed | exact |
| `internal/server/tools.go` (modify: new `*Timeout`-style helpers near :451-479, wiring at :495-538) | controller (wiring/DI) | request-response | itself — `embedTimeout`/`summaryTimeout` (:451-479) is the exact helper shape to replicate 4-6 times | exact |
| `internal/embed/embed_test.go` (modify: add truncation assertion; add D-11 regression tests) | test | request-response | itself — `TestEmbedNon2xxIncludesStatusAndBody` (:428-448) and `TestEmbedNon2xxDrainsForReuse` (:450-485) | exact |
| `internal/summarize/summarize_test.go` (modify: add truncation assertion; add D-11 regression tests; name `maxErrorBodyBytes` const) | test | request-response | `internal/embed/embed_test.go` (already the named-const sibling) | exact |
| `internal/store/redevidence_harness_test.go` (modify: +1 `redEvidenceDirs` map entry) | test (harness registry) | batch | itself — Phase 6 entry (lines 170-175) is the most-recent, smallest-shape template | exact |
| `docs-site/src/content/docs/guides/configure.md` (modify: +6 knob rows) | config (docs) | CRUD | itself — "Embedder" table (:28-38, has Flag column, env-only rows use `—`) and "Async-on-write summaries" table (:132-138, `ENGRAM_SUMMARY_WORKERS`/`QUEUE_SIZE` rows) | exact |
| `docs-site/src/content/docs/guides/upgrade.md` (modify: +1 `## Unreleased` numbered subsection for D-07) | config (docs) | CRUD | itself — subsection `### 17.` (most recent, lines 448-467) for the numbering/shape convention; `### 6.` (lines 161-172) for a zero-semantics-breaking-change of the identical shape | exact |

## Pattern Assignments

### `internal/httpdrain/httpdrain.go` (new file — utility, streaming)

**Analog:** `internal/testhttp/reuse.go` (same "new shared `internal/` package, not `_test.go`, because Go cannot share a `_test.go` across package boundaries" justification named explicitly in D-02).

**Package-boundary justification pattern** (`internal/testhttp/reuse.go` lines 4-12):
```go
// Package testhttp provides connection-reuse test instrumentation shared by
// internal/embed and internal/summarize. It is a normal (non-_test.go) file
// in an internal package rather than a _test.go helper because Go cannot
// share a _test.go across package boundaries, and both provider clients'
// test packages need the same tracker.
//
// It imports no test framework and exposes only counters and accessors, so
// nothing test-only is pulled into a production import graph even though the
// package is importable from non-test code.
package testhttp
```
For `httpdrain`, the doc comment should mirror this shape but state the inverse fact: it IS pulled into the production import graph (both `embed.go` and `summarize.go` import it at runtime, not just from tests), so its own doc comment should say plainly "imported by both provider clients' production code, not test-only" rather than reuse testhttp's "nothing test-only is pulled in" framing verbatim.

**SPDX header** — every new `.go` file gets the standard two-line header (verified present on all read analogs):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

**Core pattern — D-01's verified mechanism**, to be implemented essentially verbatim from the CONTEXT.md decision (no separate analog exists in the repo — this is genuinely new code, not adapted from an existing drain, since the four call sites this phase touches are themselves the ones being extracted into this shared helper):
```go
func Drain(body io.ReadCloser, maxBytes int64, maxTime time.Duration) {
    t := time.AfterFunc(maxTime, func() { _ = body.Close() })
    defer t.Stop()
    _, _ = io.Copy(io.Discard, io.LimitReader(body, maxBytes))
}
```
(Exact signature/name is the planner's/executor's call — CONTEXT.md pins the mechanism, not the API shape. D-05's "0 skips the drain entirely" must be handled by the CALLER before invoking this helper, or inside it as an early return — either is consistent with D-06, which is about `Option`/`New` defaulting, not this helper's internals.)

**What each of the four call sites currently look like** (the exact code this phase replaces), from `internal/embed/embed.go` lines 293-296 and 306-309:
```go
errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
_, _ = io.Copy(io.Discard, resp.Body) // drain remainder so the connection is reusable (D-14)
```
```go
if err := json.NewDecoder(io.LimitReader(resp.Body, c.maxResponseBytes)).Decode(&out); err != nil {
    return nil, err
}
_, _ = io.Copy(io.Discard, resp.Body) // drain remainder so the connection is reusable (D-14)
```
The `_, _ = io.Copy(io.Discard, resp.Body)` line at both sites (embed.go :295, :309; summarize.go :182, :191 — bare `resp.Body`, unbounded by either bytes or time) is exactly what `httpdrain`'s helper replaces at all four sites.

---

### `internal/embed/embed.go` and `internal/summarize/summarize.go` (modify — service, request-response)

**Analog:** each other — CONTEXT.md and the source comments already establish these as "direct twins" (embed.go:36 explicitly says "Copied verbatim from internal/summarize/summarize.go:181 … so the two provider lanes stay consistent").

**Existing `Option`/defaults convention this phase DELIBERATELY diverges from (D-06)** — `internal/embed/embed.go` lines 120-131 and 145-147:
```go
// WithMaxResponseBytes bounds the success-path response decode (D-16). A
// non-positive n leaves defaultMaxResponseBytes in place — this mirrors the
// existing option shapes (e.g. WithMaxTokens in internal/summarize), where an
// out-of-range override is silently ignored rather than producing an
// unbounded read.
func WithMaxResponseBytes(n int64) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxResponseBytes = n
		}
	}
}
```
```go
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, model: model, http: &http.Client{Timeout: defaultEmbedTimeout}}
	for _, o := range opts {
		o(c)
	}
	...
	if c.maxResponseBytes <= 0 {
		c.maxResponseBytes = defaultMaxResponseBytes
	}
	return c
}
```
**D-06's required divergence:** move the default INTO the struct literal in `New`, before options run, e.g.:
```go
c := &Client{..., http: &http.Client{Timeout: defaultEmbedTimeout},
	drainBytes: defaultDrainBytes, drainTimeout: defaultDrainTimeout}
for _, o := range opts {
	o(c)
}
// no post-loop "if <= 0, apply default" clamp for drainBytes/drainTimeout —
// an explicit WithDrainBytes(0)/WithDrainTimeout(0) must be honored as 0
// (D-05/D-06), unlike maxResponseBytes above.
```
CONTEXT.md D-06 requires this divergence be **commented at both the option sites and at `New`** so it reads as intentional. The `WithMaxResponseBytes` doc comment above is the exact prose template to contrast against — copy its "mirrors X" framing but invert the conclusion and name the divergence explicitly (e.g. "UNLIKE WithMaxResponseBytes above, 0 is honored, not swallowed — see D-05/D-06").

**`WithTimeout`'s current doc + implementation (D-07's target)** — `internal/embed/embed.go` lines 103-110:
```go
// WithTimeout sets the per-request HTTP client timeout. d <= 0 disables it
// (Go's http.Client treats a zero Timeout as no bound), the explicit D-08
// operator escape hatch for very slow local models. Composes with
// WithHTTPTransport regardless of option order — both mutate the shared
// c.http, and this is the last field either touches.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}
```
and `internal/summarize/summarize.go` lines 75-78 (identical shape, shorter comment):
```go
// WithTimeout sets the per-request HTTP client timeout. d <= 0 disables it.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}
```
D-07/D-09 require: (1) rewriting both doc comments — "d <= 0 disables it" becomes "resolves to a ceiling" — since `EmbedConfig.Timeout`'s and `SummarizeConfig.Timeout`'s doc comments in `internal/config/config.go` make the same promise and both need the same correction (see below); (2) applying the ceiling clamp in `New`, AFTER all options have run (not inside `WithTimeout`), e.g. appended after the options loop in both `New` functions:
```go
for _, o := range opts {
	o(c)
}
if c.http.Timeout <= 0 || c.http.Timeout > c.maxTimeout {
	c.http.Timeout = c.maxTimeout // D-07/D-09 ceiling, applied last so option order is preserved
}
```

**The bare-`4096`-vs-named-const gap (D-10 blocking sub-task)** — `internal/embed/embed.go` line 37 already names it:
```go
const maxErrorBodyBytes = 4096
```
`internal/summarize/summarize.go` line 181 still has the bare literal:
```go
body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
```
Add `const maxErrorBodyBytes = 4096` to `summarize.go` (mirroring embed.go's const + its doc-comment cross-reference convention) before writing the truncation assertion, per D-10.

**Drain call sites to replace** (exact current lines, both files):
- `internal/embed/embed.go` line 295 (non-2xx) and line 309 (success)
- `internal/summarize/summarize.go` line 182 (non-200) and line 191 (success)

All four currently read `_, _ = io.Copy(io.Discard, resp.Body) // drain remainder so the connection is reusable (D-14)` — replace with a call into `httpdrain`, keeping a comment that references D-01/D-02 (this phase) rather than D-14 (the prior phase that added the bare drain).

---

### `internal/config/registry.go` (modify — config, CRUD)

**Analog:** the `memory.max_content_bytes` / `memory.max_tags` / `memory.max_tag_bytes` entries, lines 44-57 — the exact "brand-new key, always-enforced, 0 rejected" shape D-05/D-08 need, immediately adjacent to the "0 disables" shape (`memory.max_summary_bytes`, lines 41-44) for contrast:

```go
// memory.max_content_bytes (D-01/D-09): a brand-new key, no Legacy value —
// this bound did not exist before this phase. UNLIKE memory.max_summary_bytes,
// it is ALWAYS enforced: Config.Validate rejects "0" and non-positive values
// rather than honoring them as "disabled", because plan 03-02 derives the
// read-side per-record ceiling from this cap and a disabled cap would
// silently remove that provable bound.
{Key: "memory.max_content_bytes", Env: "ENGRAM_MEMORY_MAX_CONTENT_BYTES", Default: "65536"},
```
vs the "0 disables" shape immediately above it (lines 41-44):
```go
// memory.max_summary_bytes (D-06a/D-18): a brand-new key, no Legacy value —
// the bound did not exist before this phase, so there is nothing retired to
// guard against.
{Key: "memory.max_summary_bytes", Env: "ENGRAM_MEMORY_MAX_SUMMARY_BYTES", Default: "512"},
```

**Six new entries needed, split across the two conventions:**
- `embed.drain_bytes` / `embed.drain_timeout` / `summarize.drain_bytes` / `summarize.drain_timeout` — D-05's "0 skips the drain, negative rejected" is its OWN third shape, not identical to either existing memory.* convention: it is closest to `memory.max_summary_bytes`'s "0 is a valid, meaningful value" framing, but the negative-is-rejected half borrows `validatePositiveCap`'s rejection wording style. **No existing registry comment combines "0 is honored" with "negative is rejected" in one field** — this is newly composed, not copied whole. Cite both `memory.max_summary_bytes` (for the "0 is a deliberately supported escape hatch" framing) and `embed.timeout`'s validate.go block (for the "d < 0 rejected" half) as the two half-analogs.
- `embed.max_timeout` / `summarize.max_timeout` — D-08's "non-positive rejected outright" is the SAME shape as `memory.max_content_bytes` above (always-enforced, 0 rejected) — copy that comment shape directly, substituting the D-08/D-05 decision IDs and the "this is the request-timeout ceiling, not a byte cap" framing.

Field literal shape to copy verbatim (`Key`/`Env`/`Default`, no `Legacy`, no `Flag` — matching D-04's "no Legacy … no Flag" framing, which itself echoes the `server.mcp_resource_url` entry's comment at lines 28-31):
```go
// server.mcp_resource_url (D-03, GH-526): a brand-new key, no Legacy value
// (nothing retired to guard against) and no Flag (env-only — a deployment-
// topology value, never typed at a prompt).
{Key: "server.mcp_resource_url", Env: "ENGRAM_MCP_RESOURCE_URL"},
```

---

### `internal/config/config.go` (modify — model, CRUD)

**Analog:** `MemoryConfig` struct, lines 89-119 (the doc-comment shape for a struct that mixes an always-vs-sometimes-enforced bound), and the `EmbedConfig.Timeout` / `SummarizeConfig.Timeout` field comments that D-07 requires to be corrected:

`EmbedConfig.Timeout` doc comment, lines 82-86 (MUST change per D-07):
```go
// Timeout is the per-request embed HTTP client timeout (ENGRAM_EMBED_TIMEOUT,
// default "30s"); "0" disables it (no timeout). Validated unconditionally in
// Config.Validate — the embedder is always active, unlike Summarize.Timeout
// which is gated on Summarize.Model.
Timeout string `koanf:"timeout"`
```
`SummarizeConfig` doc comment's Timeout sentence, lines 131-133 (MUST change per D-07):
```go
// ... Timeout is the per-request HTTP
// client timeout (default "30s"); "0" disables it. Neither is the same as the
// summarize-missing command's --timeout, which bounds the whole sweep.
```
Both need the "0 disables it (no timeout)" clause replaced with "0 resolves to the MaxTimeout ceiling (default 10m), not to unbounded — see D-07" and both need a new `MaxTimeout string` field added to their structs, following the existing per-field doc-comment density (every field in this file gets 2-6 lines of doc comment naming the decision ID and cross-referencing the sibling).

---

### `internal/config/validate.go` (modify — middleware, request-response)

**Analog, duration shape:** `embed.timeout`, lines 65-73:
```go
// embed.timeout runs UNCONDITIONALLY (unlike summarize.timeout, which is
// gated on Summarize.Model) — the embedder is always active, there is no
// disabled state. 0 = no timeout (infinite), the explicit D-08 escape hatch.
switch d, err := time.ParseDuration(c.Embed.Timeout); {
case err != nil:
	errs = append(errs, fmt.Errorf("ENGRAM_EMBED_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Embed.Timeout, err))
case d < 0:
	errs = append(errs, fmt.Errorf("ENGRAM_EMBED_TIMEOUT %q: must not be negative", c.Embed.Timeout))
}
```
This block's own doc comment ("0 = no timeout (infinite), the explicit D-08 escape hatch") is now WRONG under this phase's D-07/D-08 and must be corrected in place — this phase's D-08 reuses the "D-08" decision ID differently (Phase 7's D-08 is the max_timeout ceiling knob, not the same D-08 this comment cites from whatever prior phase authored it). The planner/executor must verify which phase's D-08 this comment references and correct or disambiguate it, since Phase 7 introduces its OWN D-08.

**Analog, positive-integer shape:** `ENGRAM_SUMMARY_WORKERS`, lines 240-245, and the shared helper it and `validatePositiveCap` both use:
```go
switch n, err := strconv.ParseUint(c.Summarize.Workers, 10, 64); {
case err != nil:
	errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_WORKERS %q: must be a positive integer: %w", c.Summarize.Workers, err))
case n == 0:
	errs = append(errs, errors.New("ENGRAM_SUMMARY_WORKERS must be greater than 0"))
}
```
and the named-parser alternative already in this file for exactly this "always enforced, 0 rejected" shape (lines 260-281, `validatePositiveCap` + `ParsePositiveIntCap`) — this is the PREFERRED analog for `embed.max_timeout`/`summarize.max_timeout` (D-08) over the inline `switch` shape above, since this file already centralizes that exact pattern into one named helper rather than re-inlining it a third time. `embed.max_timeout`/`summarize.max_timeout` are durations, not integers, though — so the new validation is a **composition** of the duration-parse shape (`time.ParseDuration`) with the "non-positive rejected" conclusion (`d <= 0` fails, unlike `embed.timeout`'s `d < 0` which allows exactly 0):
```go
switch d, err := time.ParseDuration(c.Embed.MaxTimeout); {
case err != nil:
	errs = append(errs, fmt.Errorf("ENGRAM_EMBED_MAX_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Embed.MaxTimeout, err))
case d <= 0:
	errs = append(errs, fmt.Errorf("ENGRAM_EMBED_MAX_TIMEOUT %q: must be a positive duration", c.Embed.MaxTimeout))
}
```
**No existing validate.go site currently combines `time.ParseDuration` with a `<= 0` (rather than `< 0`) rejection** — this exact composition is new to this phase, assembled from the two named ingredients above, not copied wholesale from one site.

For `embed.drain_bytes`/`embed.drain_timeout`/`summarize.drain_bytes`/`summarize.drain_timeout` (D-05: "0 skips the drain, negative rejected") — `drain_bytes` is an integer (compose `strconv.ParseUint`'s shape, lines 56-63's `embed.dim` block, with a `d < 0`-style non-negative-but-zero-allowed conclusion — actually `strconv.ParseUint` cannot return negative, so the negative-rejection must be done via `strconv.Atoi`/`ParseNonNegativeIntCap` from `internal/config/validate.go` lines 301-316, which is EXACTLY the "zero valid, negative rejected" integer parser already exported for this reuse):
```go
// ParseNonNegativeIntCap parses value as a non-negative integer ...
// Zero is a valid result — callers that treat zero as an escape hatch
// (ENGRAM_MEMORY_MAX_SUMMARY_BYTES's "0 disables", D-18) decide that
// themselves; this function only bounds the parse.
func ParseNonNegativeIntCap(value string) (int, error) { ... }
```
and its call site, lines 82-84:
```go
if _, err := ParseNonNegativeIntCap(c.Memory.MaxSummaryBytes); err != nil {
	errs = append(errs, fmt.Errorf("ENGRAM_MEMORY_MAX_SUMMARY_BYTES %q: %w", c.Memory.MaxSummaryBytes, err))
}
```
This is the DIRECT, exact-shape analog for `embed.drain_bytes`/`summarize.drain_bytes` — reuse `ParseNonNegativeIntCap` verbatim, just retargeting the env-var name in the wrapping `fmt.Errorf`. For `embed.drain_timeout`/`summarize.drain_timeout` (a duration, not an integer), compose `time.ParseDuration` with a `d < 0` (not `<= 0`) rejection — identical shape to `embed.timeout`'s existing block (lines 68-73) verbatim, since D-05's "0 = skip the drain" is the exact same "0 is a valid escape hatch, negative is not" semantics `embed.timeout` already has.

**Summary of the four validate.go compositions needed:**
| Field | Shape to copy | Zero handling |
|---|---|---|
| `embed.drain_bytes` / `summarize.drain_bytes` | `ParseNonNegativeIntCap` (lines 301-316, called at 82-84) — verbatim reuse | 0 valid |
| `embed.drain_timeout` / `summarize.drain_timeout` | `embed.timeout`'s duration block (lines 68-73) — verbatim shape, `d < 0` rejects | 0 valid |
| `embed.max_timeout` / `summarize.max_timeout` | New composition: duration parse + `d <= 0` rejects (no existing site does exactly this) | 0 rejected |

---

### `internal/server/tools.go` (modify — controller/wiring, request-response)

**Analog:** `embedTimeout` (lines 469-479) and `summaryTimeout` (lines 454-464) — the exact "parse, warn-on-unparseable-or-negative, fall back to a hardcoded default" shape to replicate 4-6 times for the new drain/max-timeout knobs:

```go
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
```
```go
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
```
Note: since `Config.Validate` now rejects negative/malformed values for the new knobs at startup (per D-05/D-08), the `err != nil || d < 0` fallback branch in these new helpers becomes defense-in-depth for callers that bypass `Validate` (this file's own doc comments elsewhere note test call sites construct `Config{}` directly) — keep the same warn-and-fallback shape regardless, for consistency with the two existing helpers.

**Wiring call sites** — `embedderFromConfig` (lines 483-525) and `summarizerFromConfig` (lines 527-539) are where the new `Option`s get appended, mirroring:
```go
opts := []embed.Option{
	...
	embed.WithTimeout(embedTimeout(cfg)),
	embed.WithEmbeddingsURL(cfg.OpenAI.EmbeddingsURL),
}
```
and
```go
return summarize.New(chatBaseURL, chatAPIKey, cfg.Summarize.Model, summaryMaxChars(cfg),
	summarize.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
	summarize.WithMaxTokens(summaryMaxTokens(cfg)),
	summarize.WithTimeout(summaryTimeout(cfg)))
```
New `embed.WithDrainBytes(...)`, `embed.WithDrainTimeout(...)`, `embed.WithMaxTimeout(...)` (and the `summarize.` twins) get appended to these same option lists, sourced from new `embedDrainBytes`/`embedDrainTimeout`/`embedMaxTimeout`/`summaryDrainBytes`/`summaryDrainTimeout`/`summaryMaxTimeout` helpers built on the `embedTimeout`/`summaryTimeout` template above (byte-valued ones use `strconv.Atoi`/`config.ParseNonNegativeIntCap` instead of `time.ParseDuration`, mirroring `summaryMaxTokens`'s integer-parsing shape at lines 440-449 rather than the duration shape).

---

### `internal/embed/embed_test.go` and `internal/summarize/summarize_test.go` (modify — test, request-response)

**Analog for D-10's truncation assertion:** `TestEmbedNon2xxIncludesStatusAndBody`, lines 428-448 — add a companion assertion (either extend this test or add a new one) that the surfaced error text is bounded at `maxErrorBodyBytes`, using the same "deliberately oversized body" setup `TestEmbedNon2xxDrainsForReuse` already uses (lines 456-461):
```go
bigBody := strings.Repeat("x", maxErrorBodyBytes*2)
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, bigBody)
}))
```
then assert `len(err.Error())` (or the extracted body portion) is bounded near `maxErrorBodyBytes`, not `len(bigBody)`.

**Analog for D-11's two regression shapes**, both drawn from `TestEmbedNon2xxDrainsForReuse` (lines 450-485) — this is the ONLY existing test in the repo that wires `internal/testhttp.ReuseTracker` correctly, and its own doc comment states the load-bearing wiring rule explicitly (lines 24-30 of `internal/testhttp/reuse.go`):
```go
// It only observes requests whose context was produced by (*ReuseTracker).
// Context. Both internal/embed and internal/summarize build their requests
// with http.NewRequestWithContext, so wrapping the ctx passed into
// Embed/EmbedQuery/Summarize is what makes the tracker see anything —
// attaching it anywhere else (e.g. only on the *http.Client) silently
// observes nothing, and a reuse assertion built that way passes vacuously
// regardless of whether the code under test actually drains its responses.
```
and the exact call-site wiring, `embed_test.go` lines 468-484:
```go
tracker := &testhttp.ReuseTracker{}
// Both calls go through the same Client (and thus the same underlying
// http.Client/Transport connection pool) — httptest's default client
// would not share a pool across independently-constructed clients.
c := New(srv.URL, "k", "m")
ctx := tracker.Context(context.Background())

if _, err := c.Embed(ctx, "x"); err == nil {
	t.Fatal("want error on 503, got nil")
}
if _, err := c.Embed(ctx, "y"); err == nil {
	t.Fatal("want error on 503, got nil")
}

if tracker.Reused() < 1 {
	t.Fatalf("want at least one reused connection, got Reused()=%d Total()=%d", tracker.Reused(), tracker.Total())
}
```
D-11 shape 1 (large-but-fast, with a reused control) is a direct extension of this exact pattern: a body over `drainBytes` served instantly should assert NOT reused (`tracker.Reused()` for that call path stays low / a fresh connection opens), paired with an under-`drainBytes` body that IS reused (control) — build BOTH cases in the SAME test using `tracker.Total()`/`tracker.Reused()` deltas across three sequential calls (oversized, undersized, undersized-again) rather than inventing a second tracker.

D-11 shape 2 (slow-trickle under `WithTimeout(0)`) has **no existing analog anywhere in the repo** — confirmed by `rg -n "Flusher|trickle" --type go` returning zero hits repo-wide. This is genuinely new test infrastructure: an `httptest.NewServer` handler that type-asserts its `http.ResponseWriter` to `http.Flusher`, writes in small chunks with `time.Sleep` between them, and stops after a BOUNDED total (CONTEXT.md D-11: "~3s worth", never open-ended, so a regression degrades to a clean assertion failure rather than a `go test` timeout). Given `internal/testhttp` already exists as the shared cross-client test-instrumentation package (and is not test-only itself, so it CAN host a helper used from both `_test.go` files), it is the natural home for a new `TrickleServer`/`SlowServer` helper — the planner should route this new helper through `internal/testhttp`, not duplicate it into both `embed_test.go` and `summarize_test.go`, for the same D-02 "one shared helper, not a duplicate that can drift" reasoning already applied to `ReuseTracker`.

**Named-const gap (D-10 blocking sub-task):** `internal/summarize/summarize_test.go` line 164 currently hardcodes `4096*2` where `embed_test.go` line 461 already references the named `maxErrorBodyBytes*2`:
```go
// embed_test.go (already correct):
bigBody := strings.Repeat("x", maxErrorBodyBytes*2)
// summarize_test.go (needs the const added to summarize.go first, per D-10, then this line updated to match):
bigBody := strings.Repeat("x", 4096*2)
```

---

### `internal/store/redevidence_harness_test.go` (modify — test/harness registry, batch)

**Analog:** the Phase 6 entry, lines 170-175 — the most recent, and structurally the simplest (single-line comments, no multi-clause decision citations), making it the cleanest template:
```go
".planning/phases/06-cross-spine-partial-results/red-evidence": {
	"06-01-helper-swallows-listscopes-error.patch":     "TestCrossSpineCoverageThreeStates",          // reverts: searchedScopes' failure path degrading into the scopeCoverage zero value instead of {Unknown: true} — the exact fix the roadmap and 06-CONTEXT forbid by name (D-01, D-02, D-03)
	"06-01-connect-search-discards-hits.patch":         "TestCrossSpineCoverageUnknownConnectSearch", // reverts: Connect SearchMemories aborting with a mapped error on coverage-unknown, discarding already-computed hits again (D-01)
	"06-01-mcp-list-discards-hits.patch":               "TestCrossSpineCoverageUnknownMCPList",       // reverts: the MCP list_memory closure aborting with an error on coverage-unknown, returning an error result instead of the memories it already has (D-01)
	"06-01-empty-scopes-substituted-for-absence.patch": "TestCrossSpineCoverageUnknownMCPSearch",     // reverts: recallResultMap adding searched_scopes (an empty slice) on the coverage-unknown path, the "searched nothing" reading D-03 exists to prevent
	"06-02-footer-drops-unknown-form.patch":            "TestClientListCoverageUnknownFooter",        // reverts: renderCoverageFooter's unknown branch, falling through to the count-bearing form and printing a count of zero (D-05)
},
```
New entry per D-12:
```go
".planning/phases/07-bounded-provider-responses/red-evidence": {
	"07-0N-<description>.patch": "TestName", // reverts: <what regresses> (D-XX)
	...
},
```
Map key is the phase-relative directory path (module-root-relative, per D-12's note that the harness applies patches from module root regardless of which package the patch targets — so `internal/embed`, `internal/summarize`, `internal/config`, `internal/httpdrain` patches all register under this ONE Phase 7 key, no per-package harness change needed). Each patch filename should follow the `NN-MM-short-description.patch` convention visible across every existing entry (phase-plan number, then a short kebab-case description of what the patch regresses), targeting whichever new test proves that RED direction.

---

### `docs-site/src/content/docs/guides/configure.md` (modify — docs)

**Analog for the 4 drain knobs:** "Embedder" section table, lines 30-36 (has a `Flag` column, all rows use `—` since env-only):
```markdown
| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_OPENAI_BASE_URL` | — | `http://localhost:4000` | OpenAI-compatible embeddings endpoint — ... |
```
and the "Async-on-write summaries" table, lines 132-136 (identical column shape, and the closest sibling content-wise — worker-pool tuning knobs, same section family as the new drain knobs):
```markdown
| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_SUMMARY_ON_WRITE` | — | `false` | Enables the async-on-write summary worker pool (requires `ENGRAM_SUMMARY_MODEL` also set) |
| `ENGRAM_SUMMARY_WORKERS` | — | `2` | Worker goroutine pool size draining the enqueue channel |
| `ENGRAM_SUMMARY_QUEUE_SIZE` | — | `256` | Bounded enqueue channel capacity |
```
**Load-bearing gap the planner must know:** `ENGRAM_EMBED_TIMEOUT` and `ENGRAM_SUMMARY_TIMEOUT` are NOT currently documented anywhere in `configure.md` (confirmed: `rg -n "TIMEOUT" docs-site/src/content/docs/guides/configure.md` matches nothing). D-13 only commits to documenting the SIX new knobs (`embed.drain_bytes`, `embed.drain_timeout`, `summarize.drain_bytes`, `summarize.drain_timeout`, `embed.max_timeout`, `summarize.max_timeout`) — it does not commit to retroactively documenting the pre-existing `ENGRAM_EMBED_TIMEOUT`/`ENGRAM_SUMMARY_TIMEOUT` rows, so the planner should add the six new rows into the "Embedder" and "Auto-summary" sections without assuming a `TIMEOUT` row already exists there to anchor next to.

Each section's `Source:` trailer line (e.g. line 38: `Source: internal/config (registry) + internal/server/tools.go (embedderFromConfig).`) should be updated to mention the new helpers once they exist.

---

### `docs-site/src/content/docs/guides/upgrade.md` (modify — docs)

**Analog:** subsection `### 17.`, lines 448-467 (most recent numbered entry — establishes the current highest number to increment from) and subsection `### 6.`, lines 161-172 (the closest PRIOR precedent for "an existing `0`-means-unbounded escape hatch is removed", exactly D-07's shape):
```markdown
### 6. `migrate-remap-owner --timeout 0` / `migrate-set-owner --timeout 0` no longer means unbounded

Before this release, these two commands' pre-existing `--timeout` flag
treated `0` as "disable the deadline." That is now a rejected usage error
(exit `2`), reconciled onto the same rule the new client `--timeout` uses
(#5 above) — the binary no longer ships a `--timeout` whose zero-semantics
depend on which command you happen to be scripting, for these two commands.

**Who should act:** any operator who scripts `migrate-remap-owner --timeout
0` or `migrate-set-owner --timeout 0` expecting an unbounded run. Remove
the flag (both commands' own default, `5m`, still applies) or supply a
large explicit duration (`--timeout 24h`) instead.
```
D-07's new subsection is `### 18.` (next after 17), following this exact shape: state the prior behavior, the new behavior (ceiling, not unbounded — reference `ENGRAM_EMBED_MAX_TIMEOUT`/`ENGRAM_SUMMARY_MAX_TIMEOUT` default `10m`), and a **Who should act** paragraph naming anyone who relied on `WithTimeout(0)` / `ENGRAM_EMBED_TIMEOUT=0` / `ENGRAM_SUMMARY_TIMEOUT=0` meaning "forever." Also add a row to the "Do you need to act?" table near the top (lines 27-38) pointing at `§18`, matching the existing row shape:
```markdown
| rely on `ENGRAM_EMBED_TIMEOUT=0` / `ENGRAM_SUMMARY_TIMEOUT=0` meaning no timeout at all | §18 |
```

Per CLAUDE.md and the repo's own frontmatter rule (restated in CONTEXT.md's "Carried forward" list): **no SPDX header** on this file or `configure.md` — both start with `---` YAML frontmatter.

---

## Shared Patterns

### Config registry entry (brand-new key, no Legacy, no Flag)
**Source:** `internal/config/registry.go` lines 28-31 (`server.mcp_resource_url`), lines 41-57 (the three `memory.*` entries)
**Apply to:** all six new registry entries in this phase (`embed.drain_bytes`, `embed.drain_timeout`, `embed.max_timeout`, `summarize.drain_bytes`, `summarize.drain_timeout`, `summarize.max_timeout`)
```go
// <key> (D-XX): a brand-new key, no Legacy value (nothing retired to guard
// against) and no Flag (a provider-tuning value, never typed at a prompt).
{Key: "<key>", Env: "ENGRAM_<ENV>", Default: "<default>"},
```

### Validation error envelope
**Source:** `internal/config/validate.go`, pervasive (e.g. lines 68-73, 240-245, 276-281)
**Apply to:** all six new `Config.Validate` blocks
```go
errs = append(errs, fmt.Errorf("ENGRAM_<VAR> %q: <what's wrong>: %w", c.<Field>, err))
```
Every rejection names the `ENGRAM_` env var (never the koanf key), per `Validate`'s own doc comment (config.go line 18: "Each error names the ENGRAM_* env var, not the koanf key").

### `*Timeout`/`*Bytes` config-parsing helper in `internal/server/tools.go`
**Source:** `embedTimeout` (lines 469-479), `summaryTimeout` (lines 454-464), `summaryMaxTokens` (lines 440-449, integer variant)
**Apply to:** the 4-6 new helpers wiring the drain/max-timeout knobs into `embed.Option`/`summarize.Option` construction
```go
func <name>(cfg *config.Config) <time.Duration|int64> {
	v, err := <time.ParseDuration|strconv.Atoi>(cfg.<Section>.<Field>)
	if err != nil || v < 0 {
		if cfg.<Section>.<Field> != "" {
			slog.Warn("ENGRAM_<VAR> is set but unparseable or negative; using default <default>",
				"value", cfg.<Section>.<Field>)
		}
		return <default>
	}
	return v
}
```

### Connection-reuse test wiring
**Source:** `internal/testhttp/reuse.go` (`ReuseTracker`), consumed by `TestEmbedNon2xxDrainsForReuse` (embed_test.go :450-485) and `TestSummarizeNon200DrainsForReuse` (summarize_test.go :152-187)
**Apply to:** `internal/httpdrain/httpdrain_test.go`, the D-11 shape-1 (large-but-fast) regression tests in both `embed_test.go` and `summarize_test.go`
**Load-bearing rule (must be restated wherever this is used):** the tracker only observes requests whose CONTEXT it wrapped via `tracker.Context(...)` — attaching it any other way (e.g. only via `WithHTTPTransport`) observes nothing and the assertion passes vacuously regardless of correctness.

### "Direct twins" cross-reference comment
**Source:** `internal/embed/embed.go` line 36 ("Copied verbatim from internal/summarize/summarize.go:181 (D-13) rather than invented, so the two provider lanes stay consistent"); `internal/testhttp/reuse.go` lines 26-27 ("Both internal/embed and internal/summarize build their requests with http.NewRequestWithContext")
**Apply to:** every drain-bound/timeout-ceiling addition to `embed.go`/`summarize.go` — each new const/field/doc-comment pair should cross-reference its sibling file+line, matching the existing discipline in both files.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| Slow-trickle `httptest` helper (D-11 shape 2, to live in `internal/testhttp` or `internal/httpdrain`) | test utility | streaming | No test anywhere in the repo builds a deliberately slow/chunked `http.Flusher`-based server — confirmed via `rg -n "Flusher\|trickle" --type go` (zero hits). This is genuinely new test infrastructure; build it following `internal/testhttp/reuse.go`'s "shared internal package, not `_test.go`, because Go cannot cross package boundaries" precedent, not by copying an existing trickle server. |
| `internal/httpdrain/httpdrain.go`'s core `Drain` function body | utility | streaming | The `time.AfterFunc` + `defer t.Stop()` + `io.LimitReader` mechanism is pinned by D-01 directly from verified Go `net/http` source, not from an existing repo call site — the four call sites this phase touches are themselves the (soon-to-be-replaced) unbounded drains, so there is no bounded-drain analog to copy from inside this repo. |

## Metadata

**Analog search scope:** `internal/config/`, `internal/embed/`, `internal/summarize/`, `internal/server/tools.go`, `internal/testhttp/`, `internal/store/redevidence_harness_test.go`, `docs-site/src/content/docs/guides/{configure,upgrade}.md`
**Files scanned:** 12 (all named directly in 07-CONTEXT.md's Code Context section; no additional Glob/Grep sweep was needed since CONTEXT.md already enumerates every touched file and most line numbers)
**Pattern extraction date:** 2026-09-21
**Tracked-source gate:** all 12 analog paths verified via `git ls-files` — all tracked, none are gitignored mirrors.
