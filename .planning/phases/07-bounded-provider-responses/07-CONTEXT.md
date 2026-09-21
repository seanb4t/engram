# Phase 7: Bounded Provider Responses - Context

**Gathered:** 2026-09-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Bound the embed and summarize clients' post-response body drain by **bytes and by time**, so no
provider response can stall a goroutine indefinitely — and close the last path by which
`WithTimeout(0)` leaves a request truly unbounded. Covers REQ-provider-drain-bounded (GitHub #457)
and REQ-provider-error-body-closed (GitHub #347).

Exactly four drain sites exist repo-wide, verified by `rg 'io\.Copy\(io\.Discard' --type go`
excluding tests, and **all four are in scope**:

| File | Line | Path |
|------|------|------|
| `internal/embed/embed.go` | 295 | non-2xx, after the bounded error read |
| `internal/embed/embed.go` | 309 | success, after the bounded decode |
| `internal/summarize/summarize.go` | 182 | non-200, after the bounded error read |
| `internal/summarize/summarize.go` | 191 | success, after the bounded decode |

Fully independent of the Qdrant read-path work in Phases 1–5: no shared code with `internal/store`,
no shared code with Phase 6's `internal/server` changes. Zero risk to and from the rest of this
milestone.

Out of scope: the Qdrant read path; retry/backoff for a connection abandoned mid-drain; changing
`maxErrorBodyBytes` (4096) or `defaultMaxResponseBytes` (1 MiB); any other HTTP client in the repo
(there are no other `io.Copy(io.Discard, …)` drains — see Deferred for the OIDC/JWKS lane).

</domain>

<decisions>
## Implementation Decisions

### Carried forward

- **D-00 [informational] (user preference `1w3h5sy56m`, rule `xvqj44e5mk`):** choose by idiom and
  long-term maintenance, never by effort.
- `internal/config/registry.go` is the **single source of truth** for every `ENGRAM_` var
  (CLAUDE.md). A new knob is one registry entry + one `Config` struct field + validation in
  `Config.Validate` + a docs-site row — never a value read directly from the environment.
- A rejected value names the failing field in one envelope (`ENGRAM_X %q: must be …`), matching
  `internal/config/validate.go`'s existing shape.
- Red-evidence discipline from Phases 1–6: every RED direction gets one `git diff -U3` patch,
  proven apply → RED → revert by `TestRedEvidencePatchesAreLive`.
- No SPDX header on any file whose first line must be `---` frontmatter (`docs-site/**`,
  `.planning/**`).

### The bounded drain (REQ-provider-drain-bounded, #457)

- **D-01 — mechanism: a timer closes the body.** The drain arms
  `t := time.AfterFunc(maxTime, func() { _ = body.Close() })`, `defer t.Stop()`, and copies through
  `io.LimitReader(body, maxBytes)` into `io.Discard`. Closing the body unblocks the in-flight
  `Read`, so a trickle body is abandoned after exactly `maxTime`. **No goroutine outlives the
  call.** The caller's existing `defer func() { _ = resp.Body.Close() }()` becomes a harmless second
  Close (net/http returns nil).
  Rejected: a goroutine + `select` (the goroutine is unblocked only because the *caller's* deferred
  Close happens to run next — an invisible coupling that breaks silently if a future call site
  forgets the defer, and `time.After` leaks its timer); `context.AfterFunc` (same Close-unblocks-Read
  mechanism reached through two more moving parts, and it forces a `ctx` parameter onto a helper
  that needs none).
  — **Reversibility:** two-way.
- **D-02 — one shared helper in a new `internal/httpdrain` package**, imported by both clients.
  Same move the repo already made with `internal/testhttp` when embed and summarize needed shared
  instrumentation. Rejected: an unexported twin in each package — a real duplicate that can drift,
  across two packages whose comments already describe each other as "direct twins".
- **D-03 — all four sites, not just the two error paths #457 names.** The success-path drains have
  the identical shape and the identical exposure; fixing only the error path would leave a hole
  whose existence is harder to notice because it is on the happy path.

### Bounds and configuration

- **D-04 — four per-client knobs, mirroring the existing `ENGRAM_EMBED_TIMEOUT` /
  `ENGRAM_SUMMARY_TIMEOUT` pairing:**

  | Key | Env | Default |
  |-----|-----|---------|
  | `embed.drain_bytes` | `ENGRAM_EMBED_DRAIN_BYTES` | `262144` (256 KiB) |
  | `embed.drain_timeout` | `ENGRAM_EMBED_DRAIN_TIMEOUT` | `2s` |
  | `summarize.drain_bytes` | `ENGRAM_SUMMARY_DRAIN_BYTES` | `262144` (256 KiB) |
  | `summarize.drain_timeout` | `ENGRAM_SUMMARY_DRAIN_TIMEOUT` | `2s` |

  All brand-new keys: no `Legacy` value (nothing retired to guard against) and no `Flag` (a
  provider-tuning value, never typed at a prompt). Rejected: two shared `ENGRAM_PROVIDER_DRAIN_*`
  knobs — the per-client shape matches the existing timeout pair exactly.
- **D-05 — `0` skips the drain entirely; negative is rejected.** `0` on *either* knob means close
  the body immediately and let the connection go rather than be reused. A negative value fails
  `Config.Validate` with a named field. There is deliberately **no way to express "unbounded"** —
  that is the hazard this phase exists to remove. `0` stays a useful, safe setting for an operator
  who would rather burn TCP handshakes than ever stall.
- **D-06 — defaults are set in the struct literal in `New`, BEFORE options are applied**, so an
  explicit `WithDrainBytes(0)` is honored as `0` rather than swallowed. This is a **deliberate
  divergence** from the sibling convention at `embed.go:145` (`WithMaxResponseBytes`: "a
  non-positive n leaves the default in place"), forced by D-05. It must be commented at both
  option sites and at the `New` site so the divergence reads as intentional, not as a bug.

### The request-timeout ceiling (#457's paired question)

- **D-07 — `WithTimeout(d)` with `d <= 0` now resolves to a ceiling, not to "no timeout".** An
  explicit positive `d` is honored **uncapped**, however large: the operator named a number, so
  respect it. Rejected: clamping every value, which would override a deliberately-chosen longer
  duration.
  **This is a BREAKING documented-behavior change** — `EmbedConfig.Timeout`'s doc comment and
  `SummarizeConfig.Timeout`'s both currently promise `"0" disables it (no timeout)`. Both comments
  change, and `guides/upgrade.md` gets a note: anyone who relied on `0` sets an explicit duration
  instead.
  — **Reversibility:** one-way — a published behavior change to a documented escape hatch.
- **D-08 — the ceiling is itself configurable:** `embed.max_timeout` / `ENGRAM_EMBED_MAX_TIMEOUT`
  and `summarize.max_timeout` / `ENGRAM_SUMMARY_MAX_TIMEOUT`, both defaulting to `10m`. A
  non-positive value is **rejected** by `Config.Validate` (consistent with D-05's "no way to
  express unbounded"). Six new registry entries this phase.
  **Known limitation, recorded rather than papered over:** the recursion is real — a large enough
  value (`999h`) is effectively unbounded. This knob is a speed bump that forces an operator to
  write a number they can see, not a hard guarantee. Raised at decision time and accepted.
- **D-09 — the clamp is applied in `New`, after all options have run**, not inside `WithTimeout`,
  so option ordering (last writer wins) is preserved. It lives in the **client**, not in
  `tools.go`'s `embedTimeout`/`summaryTimeout` helpers, so every caller — including tests and any
  future embedder of these packages — gets the ceiling rather than only the server wiring path.

### The already-shipped bounded error read (REQ-provider-error-body-closed, #347)

- **D-10 — add the missing truncation assertion, then close #347.** The existing
  `TestEmbedNon2xxIncludesStatusAndBody` / `TestSummarizeNon200IncludesStatusAndBody` assert the
  provider snippet **appears**; nothing asserts it is **truncated** at the bound. The existing
  `…DrainsForReuse` tests feed a 2×-oversized body but assert reuse, not boundedness. Add one
  assertion per client pinning that an oversized error body is cut at the bound, then close #347
  citing the snippet test, the drain test and the new truncation assertion.
  Blocking sub-task: `summarize.go:181` uses a bare `4096` literal where embed has a named
  `maxErrorBodyBytes` const — name it in `summarize` too, so the assertion has something to
  reference instead of re-hardcoding the number in a test.

### Proof obligations

- **D-11 — two regression SHAPES per client, per the roadmap's explicit requirement:**
  1. **Large-but-fast.** A body larger than `drainBytes` delivered instantly: assert the call
     returns AND the connection is **not** reused (the byte bound stopped mid-body). Paired with a
     body smaller than `drainBytes` that **is** reused — without the control, a client that never
     drained anything would look green.
  2. **Slow-trickle under `WithTimeout(0)`.** Drain timeout set small (e.g. 50ms); assert elapsed
     time is far below what the trickle would cost.
  **The trickle server must write a BOUNDED total and finish** (e.g. ~3s worth), never trickle
  open-ended. An open-ended trickle turns the red-evidence RED into a `go test` timeout instead of
  a clean assertion failure, which is a materially worse proof and a slow one.
- **D-12 — Phase 7 registers its own `red-evidence/` directory** in `redEvidenceDirs`
  (`internal/store/redevidence_harness_test.go`). The harness lives in `internal/store` but applies
  patches module-root-relative, so patches targeting `internal/embed`, `internal/summarize`,
  `internal/config` and `internal/httpdrain` need a registry entry only — no harness change. The
  harness is at 58 patches; each Phase 7 patch adds to the same full-suite run.
- **D-13 — documentation obligations:** `guides/configure.md` gains the six knobs;
  `guides/upgrade.md`'s `## Unreleased` gains **one** numbered subsection for D-07's breaking
  change. Both `EmbedConfig.Timeout` and `SummarizeConfig.Timeout` doc comments are corrected.

</decisions>

<code_context>
## Code Context

- `internal/embed/embed.go` — `Client`, `Option`, `New` (defaults at :145), `WithTimeout` (:108),
  `WithMaxResponseBytes` (:125), `maxErrorBodyBytes` (:37), drains at :295 and :309.
- `internal/summarize/summarize.go` — `Client`, `Option`, `New` (:81), `WithTimeout` (:76),
  `defaultTimeout` (:39), the bare `4096` at :181, drains at :182 and :191.
- `internal/config/registry.go` — the six new `field` entries; `internal/config/config.go` —
  `EmbedConfig` (:64) and `SummarizeConfig` (:137) struct fields; `internal/config/validate.go` —
  the duration/positive-integer validation shapes already used by `ENGRAM_EMBED_TIMEOUT` (:65-70)
  and `ENGRAM_SUMMARY_WORKERS` (:242-244).
- `internal/server/tools.go` — client construction at :501 (`embed.WithTimeout`), :522
  (`embed.WithMaxResponseBytes`), :538 (`summarize.WithTimeout`); the `embedTimeout` (:469) and
  `summaryTimeout` (:454) helpers where the new drain options get wired.
- `internal/testhttp/reuse.go` — `ReuseTracker`, already shared by both clients' tests; the
  large-but-fast shape's reuse assertions build on it.
- Both `embed_test.go` and `summarize_test.go` are **internal** test packages (`package embed`,
  `package summarize`), so unexported fields and consts are directly reachable from tests.
- `docs-site/src/content/docs/guides/configure.md` and `guides/upgrade.md`.

</code_context>

<specifics>
## Specific Ideas

- The drain's only job is connection reuse. Abandoning it costs one TCP handshake; blocking costs a
  wedged goroutine forever. Giving up is the correct outcome, not a failure.
- `io.LimitReader` alone does **not** close this hole — it bounds bytes, not time.
- `WithTimeout(0)` keeps its purpose (no *request* deadline for a slow self-hosted gateway) right up
  to the ceiling; it just no longer means "forever".

</specifics>

<deferred>
## Deferred Ideas

- Bounding the OIDC/JWKS discovery and key-fetch HTTP lane (`internal/auth`) the same way — no
  `io.Copy(io.Discard, …)` drain exists there today, so it is not this phase's four-site scope, but
  it is the same class of exposure.
- Retry/backoff when a connection is abandoned mid-drain. Today the next request simply opens a
  fresh connection, which is correct and cheap.
- GitHub #596 (reindex's per-page `Get` unbounded by bytes) — filed during Phase 5, unrelated here.

</deferred>

---

*Phase: 07-bounded-provider-responses*
*Context gathered: 2026-09-21*
