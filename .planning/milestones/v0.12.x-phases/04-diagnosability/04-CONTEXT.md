# Phase 4: Diagnosability - Context

**Gathered:** 2026-08-01
**Status:** Ready for planning

<domain>
## Phase Boundary

What the server decided, and why it rejected something, reaches whoever needs it — the operator
debugging a denial, and the agent retrying a rejected call. Four independent fixes across four
subsystems, sharing one design discipline: **bounded, structured, redaction-conscious disclosure.**

**In scope:** debug-level Cedar decision logging on both the allow and deny arms; machine-readable
field attribution on argument-validation rejections; a structured remediation hint alongside that
attribution; a bounded prefix of a provider's error body on the embeddings lane, plus a
connection-reuse drain on both provider lanes.

**Out of scope:** any change to *who* is authorized (this phase reports decisions, it does not make
them); full Cedar expression traces at any log level; changing recall or write semantics; new
provider integrations.

</domain>

<decisions>
## Implementation Decisions

### Cedar decision diagnostics

- **D-01 (the log line is emitted from internal/store, not from internal/authz):** `internal/store`
  emits it at the point it consumes a `Decision`. `internal/authz/authz.go:44-47` already designates
  this in a comment — the `diag` field "exists solely for future debug-level logging / OTel span
  attachment by internal/store" — and this phase is the future that comment anticipated. Keeps
  `internal/authz` a pure decision function with no logger dependency. `internal/authz` emits zero
  `slog` calls today; that stays true.
  — **Reversibility:** reversible.

- **D-02 (the allowlist is policy IDs, an error COUNT, and the decision/action/bucket — never the raw Diagnostic):**
  The emitted fields are the satisfied policy IDs from `Reasons`, the *count* of `Errors`, and the
  decision, action, and bucket already in hand. Cedar error **messages** are deliberately excluded:
  they can embed entity values, which is caller-adjacent data. No policy expression text, ever, at
  any level.
  — **Reversibility:** costly — a widened allowlist that ships is in operators' log pipelines.

- **D-03 (the allowlist is enforced STRUCTURALLY by a narrow accessor, not by call-site discipline):**
  `authz.Decision.diag` stays unexported. A dedicated exported accessor returns a narrow struct
  carrying exactly D-02's fields, so the only way to log diagnostics is through the allowlist and a
  future `cedar.Diagnostic` field cannot leak by someone reaching into `diag`. Chosen over exporting
  `diag` because call-site discipline is precisely the failure mode this phase exists to prevent.
  — **Reversibility:** reversible.

- **D-04 (both the allowed and the denied arm are logged, at debug):** Criterion 1 requires both.
  An operator debugging a *wrong allow* needs it at least as much as a wrong deny, and a wrong allow
  is the more dangerous bug.
  — **Reversibility:** reversible.

### Validation error attribution

- **D-05 (a structured error type carries a machine-readable field name):** Rejections carry the
  failing field as structured data, not only in prose, so the criterion-2 matrix asserts on the field
  identifier rather than on message wording. Criterion 2 says this explicitly — "proven by a matrix
  with one case per single-field-invalid input rather than by matching exact wording."
  — **Reversibility:** costly — a published error shape on both lanes.

- **D-06 (the sweep covers EVERY single-field rejection site, not a sample):** Every `validate*`
  function and every inline argument check in `internal/server/tools.go` that can reject on one field
  is enumerated and converted. A partial sweep leaves an inconsistent surface where an agent can
  branch on the field for some tools and not others, which is worse than not having it.
  — **Reversibility:** reversible.

- **D-06a (the sweep is EXTENDED above tools.go to the schema layer, so issue #360 is actually fixed — decided 2026-08-01 by Sean, after research):**
  Research established that #360's misleading `missing properties: ["content"]` is emitted by the
  go-sdk's `applySchema` (`mcp/tool.go`) BEFORE any `tools.go` code runs, because
  `storeArgs.Content/Scope/Source/Category` (`tools.go:430-433`) carry no `omitempty` and are
  therefore schema-required. D-06's original scope could not reach it, so a phase satisfying
  criterion 2 over the `tools.go` sites would have shipped with its own motivating bug still
  reproducible.
  Extended scope: drop schema-level `required` by adding `omitempty` on the affected fields, and add
  Go-level presence checks in `tools.go` so engram owns the rejection and attributes the correct
  field. Applies to every arg struct carrying schema-required fields, not only the two #360 names —
  a partial application would recreate D-06's own inconsistency objection one layer up.
  **Consequence, accepted:** the published MCP tool schema loosens. Required-ness moves out of the
  schema and into engram's validation. This ships under the same `feat!` D-11 already carries.
  **Verification obligation:** the phase must reproduce #360's exact call — a valid `content` plus an
  oversized `summary` — and assert the error now names `summary`, not `content`. Criterion 2's matrix
  alone does not prove this; it is a separate regression test naming the issue.
  — **Reversibility:** costly — a published tool schema.

- **D-07 (the matrix is table-driven with one case per single-field-invalid input):** Each case makes
  exactly one field invalid and asserts the returned field identifier equals that field. This is what
  makes it a matrix rather than a spot-check, and it is what catches an attribution that names a
  neighbouring field.
  — **Reversibility:** n/a — a verification obligation.

- **D-08 (validation message TEXT is reformatted to a field-first convention — decided 2026-08-01 by Sean, over the recommendation):**
  The recommendation was to preserve existing message text and carry the field structurally alongside,
  citing engram `e9yv53pmnv` (error text is a wire contract in this repo). Overridden: message text is
  normalized so the field leads. **Scope fence the planner must respect:** `e9yv53pmnv` concerns the
  MCP **401 auth body**, which is produced by the go-sdk's `RequireBearerToken`, NOT by
  `internal/server/tools.go`'s argument validation. This decision covers argument-validation messages
  only. The auth 401 body must remain byte-identical, and the planner must verify that separation
  before reformatting anything rather than assuming it.
  — **Reversibility:** costly — changes text callers may match on.

### Remediation hint envelope

- **D-09 (a hint is a machine-stable code plus human text, carried with the field in one envelope):**
  An agent branches on the code; a human reads the text. The hint and the D-05 field attribution
  travel together — criterion 3 says the hint sits "alongside the field attribution", so they are one
  envelope, not two mechanisms.
  — **Reversibility:** costly — a published error shape.

- **D-10 (hints are authored per validation rule at the rejection site):** The constraint is known
  where the rejection happens. Rejected a central message-to-hint lookup table because it drifts
  silently from the rules it describes, and the drift is invisible until an agent follows a stale hint.
  — **Reversibility:** reversible.

- **D-11 (Connect widens to semantically distinct standard error codes per failure class — decided 2026-08-01 by Sean, over the recommendation):**
  The recommendation was to keep the existing taxonomy and carry the hint as error-detail metadata.
  Overridden: different validation-failure classes map to different Connect error codes.
  **Technical clarification the planner must not lose:** Connect/gRPC error codes are a CLOSED enum —
  new codes cannot be defined. So this means selecting among the existing standard codes
  (`InvalidArgument`, `FailedPrecondition`, `OutOfRange`, `NotFound`, …) per class, where today
  `internal/server/connectapi.go` collapses essentially everything to `CodeInvalidArgument`
  (`:149`, `:153`, `:160`, `:173`, `:230`, `:234`, `:243`). Sean's rationale: this is a pre-1.0
  release and a code-mapping change is acceptable.
  **This is a BREAKING CHANGE and MUST ship as a `feat!` commit** (or carry a `BREAKING CHANGE:`
  footer) so it surfaces in the release notes. Release impact was checked against
  `release-please-config.json` per rule `0v4249kc9d`: with `bump-minor-pre-major: true` and
  `bump-patch-for-minor-pre-major: false` from `0.11.2`, both `feat` and `feat!` bump the MINOR to
  `0.12.0` — the `!` is a changelog signal here, not a version escalation, and it does not disturb the
  v0.12.x milestone label.
  — **Reversibility:** one-way — a released error-code mapping is a contract callers branch on.

- **D-11a (the bare-unwrapped-error defect found by research is fixed in this phase):** Research found
  that `validateStoreDiscovery`, `validateCitations`, and part of `validateStoreRule`
  (`tools.go:638-687`, `rules.go:56-67`) return bare unwrapped errors, so on the Connect lane they
  fall through `connectError`'s switch (`connecterror.go:49-85`) to `CodeInternal` — a caller's
  invalid input reported today as a server fault. This is a live, previously undocumented defect,
  squarely inside D-11's mapping work, and it is fixed here rather than filed. A test must pin each
  of the three so the wrapping cannot silently regress.
  — **Reversibility:** reversible.

- **D-12 (a hint never echoes the caller's rejected VALUE):** It names the field and states the
  constraint. Same discipline as Phase 3's D-02 no-value-echo logging: an unbounded or sensitive
  caller-supplied string must not reach an error payload or a log line.
  — **Reversibility:** reversible.

### Provider error body and connection drain

- **D-13 (reuse the chat lane's existing 4096-byte bound on the surfaced error-body prefix):**
  `internal/summarize/summarize.go:181` already reads `io.LimitReader(resp.Body, 4096)` and includes
  the trimmed body in its error. The embeddings lane copies that number rather than inventing one, so
  the two provider lanes stay consistent — the same "mirror the shipped sibling" move Phase 3 made
  with `search_discovery`.
  — **Reversibility:** reversible.

- **D-14 (the embeddings lane surfaces the body; BOTH lanes gain the drain):** Criterion 4's audit
  clause resolves asymmetrically, and the planner must not flatten it:
  - **Surfacing** is embeddings-only. `internal/embed/embed.go:248-249` returns
    `fmt.Errorf("embeddings: status %d", resp.StatusCode)` and discards the body entirely. The chat
    lane already surfaces it and needs no change on this axis.
  - **Draining** is both. Neither lane drains: `io.LimitReader(body, 4096)` leaves the remainder
    unread, so the connection cannot be reused. Add an `io.Copy(io.Discard, resp.Body)` (or
    equivalent bounded drain) after reading the prefix and before `Close` on **both** lanes.
  So "the chat/summarize lane has been audited for the same gap and fixed if it shares it" answers:
  it does not share the surfacing gap, it does share the drain gap.
  — **Reversibility:** reversible.

- **D-15 (the provider error body is surfaced verbatim within the bound, not scrubbed):** A provider's
  error body is the provider's own text, not caller data, and the request `Authorization` header is
  never echoed in a response body. Scrubbing would mangle the exact diagnostic this requirement exists
  to deliver.
  **Planner note:** confirm by reading that no code path puts caller content into the embeddings
  request in a way a provider would reflect back verbatim.
  — **Reversibility:** reversible.

- **D-16 (bound the embeddings SUCCESS-path decode too, mirroring the sibling):**
  `summarize.go:186-188` wraps its success decode in a 1 MiB `io.LimitReader` with a comment about a
  misbehaving gateway forcing an unbounded read. `embed.go:252` decodes unbounded. Same exposure, and
  the sibling already guards it. An embeddings response is larger than a one-line summary, so the
  planner should size the bound to the configured `ENGRAM_EMBED_DIM` rather than copying 1 MiB blindly.
  — **Reversibility:** reversible.

### Checkpoint resolutions (2026-08-01, Sean — plan 04-01 Task 1)

- **D-17 (the envelope grammar is a flat field-and-hint prefix, not JSON):** Errors read
  `field=<name> hint=<code>: <human text>`. Chosen over a JSON payload because these errors are
  `%w`-wrapped by callers and wrapping a JSON blob produces unparseable nesting. This string IS the
  wire format on the MCP lane — `go-sdk@v1.6.1/mcp/server.go:340-354` discards the built
  `*CallToolResult` on a non-nil error and returns only `err.Error()` as text, so there is no side
  channel to carry structure separately.
  — **Reversibility:** costly — a published error shape on both lanes.

- **D-18 (the memory summary bound is REAL but KOANF-CONFIGURABLE — decided 2026-08-01 by Sean):**
  Research established that issue #360's misattribution cannot be fixed deterministically without a
  summary length bound, because none exists today — `maxRuleSummaryBytes = 256`
  (`internal/server/rules.go:22`) bounds *rules* only. The plan proposed a hard
  `maxMemorySummaryBytes = 512` constant. Sean approved the bound **on condition that it is koanf
  configurable**, not a compile-time constant.
  So it becomes a `internal/config` registry entry — the single source of truth for `ENGRAM_` vars
  per CLAUDE.md — with a **default of 512**, following the existing entry shape
  (`internal/config/registry.go:60-66`). The rule bound stays a separate constant; this key governs
  ordinary memory summaries only.
  **Planner/executor notes:** (a) no `Legacy:` key — it is new, and retired `MEM_*` vars are a fatal
  guard (DEC-jgq/DEC-irq); (b) follow the existing "0 is honored as disabled" convention used by the
  timeout keys (`tools.go:315`, `:330`) so an operator can switch the bound off; (c) the check must
  still run **before** the content-presence check so #360's regression is deterministic rather than
  decoder-dependent — configurability does not relax that ordering; (d) a config key is a published
  operator contract, so it needs a docs-site entry in plan 04-07.
  — **Reversibility:** costly — a published config key; renaming it later breaks operator deployments
  and Helm values.

- **D-19 (`delete_all`'s relaxation ships with its mitigation as one indivisible edit):** Accepted as
  planned. `scopeArgs.Scope` (`tools.go:588`) backs `delete_all` (`tools.go:1702`), so relaxing its
  `omitempty` moves the only guard between an omitted scope and a destructive teardown out of the
  wire schema. The tag relaxation and the Go-level presence check are ONE task, the check precedes
  every side effect, and `TestDeleteAllRequiresScope` is a dedicated test rather than a matrix row.
  If the ordering cannot be guaranteed, the executor STOPS and reports rather than shipping the
  relaxation alone.
  — **Reversibility:** reversible — but the window between relaxation and check is not, which is why
  they are indivisible.

- **D-20 (the Connect code mapping stays inside the CLI-compatible trio):** Classes map only to
  `InvalidArgument` (malformed or unknown enum value), `OutOfRange` (length and numeric bounds), and
  `FailedPrecondition` (mutually-exclusive or ordering violations between individually-valid fields).
  These are exactly the three `exitCodeForConnectErr` (`cmd/engram/client_common.go:237-249`) already
  groups under one exit code, so the CLI's exit-code contract is unchanged for free and the breaking
  surface is confined to callers branching on the Connect code itself. Ships as `feat!` per D-11.
  — **Reversibility:** one-way — a released error-code mapping.

### Claude's Discretion

- Exact names for the diagnostics accessor, the structured validation-error type, and the hint code
  vocabulary.
- Which standard Connect code each validation-failure class maps to under D-11 — the planner
  proposes the mapping table; the shape of the decision is fixed, the specific assignments are not.
- Whether the hint code is a Go typed constant or a string, and how it serialises on each lane.
- The embeddings success-path decode bound under D-16.
- Whether the four fixes land as four plans or are grouped — they are independent, so wave structure
  is a planning judgment.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- `internal/summarize/summarize.go:180-182` — the already-shipped bounded-error-body shape D-13/D-14
  copy: `io.LimitReader(resp.Body, 4096)` plus the status code in the error.
- `internal/summarize/summarize.go:186-188` — the already-shipped bounded success decode D-16 mirrors,
  with a comment stating the misbehaving-gateway rationale.
- `internal/authz/authz.go:48-51` — `Decision{Allow bool; diag cedar.Diagnostic}`, with the
  unexported-diag design and the comment naming `internal/store` as the intended logging site.
- `internal/authz/authz.go:57+` — `DecideRecord` returns `Decision{Allow: bool(decision), diag: diag}`
  from `cedar.Authorize`, so `Reasons`/`Errors` are already captured and simply not read.

### Established Patterns

- Authorization is decided in `internal/authz` and enforced in `internal/store`, never in handlers
  (ADR `engram-cdr1` refining LOCKED `DEC-cgb`). This phase adds reporting at the store seam and
  changes no decision.
- No-value-echo logging: Phase 3's D-02 established that a caller-supplied string is not interpolated
  into a log line. D-12 applies the same rule to hints.
- Transport adapters own their own error mapping; `internal/server/connectapi.go` currently maps
  essentially every validation failure to `connect.CodeInvalidArgument`.
- Error text has been a wire contract at least once before (engram `e9yv53pmnv`, the MCP 401 body) —
  which is why D-08 carries an explicit scope fence.

### Integration Points

- `internal/authz/authz.go` — `Decision`, the new narrow accessor (D-03).
- `internal/store/` — the `Decision` consumption sites where D-01's log line is emitted.
- `internal/server/tools.go` — every `validate*` and inline single-field check (D-06); the hint
  authoring sites (D-10).
- `internal/server/connectapi.go` — the D-11 code-mapping widening; today's collapse to
  `CodeInvalidArgument` at `:149`, `:153`, `:160`, `:173`, `:230`, `:234`, `:243`.
- `internal/embed/embed.go:248-252` — D-14 surfacing, D-14 drain, D-16 success bound.
- `internal/summarize/summarize.go:179-183` — D-14 drain only.
- `docs-site/src/content/docs/` and the `curating-memory` skill — agent-facing surfaces for the hint
  vocabulary and the new error-code mapping, per engram convention `yaj7dqz9qq`.

</code_context>

<specifics>
## Specific Ideas

- **D-11 must ship as a `feat!` commit.** This is Sean's explicit instruction and it is the only
  breaking change in the phase. Release-please impact was verified: it does not escalate the version
  beyond the already-planned `0.12.0`.
- **D-08's scope fence is load-bearing.** Reformatting argument-validation message text is approved;
  changing the MCP 401 auth body is not. They are produced by different code (`tools.go` vs the
  go-sdk's `RequireBearerToken`). Verify the separation before editing, per engram `e9yv53pmnv`.
- Criterion 2's phrase "proven by a matrix ... rather than by matching exact wording" is a direct
  instruction about test design, not a stylistic preference — D-08's reformatting makes wording-based
  assertions even more fragile, so the matrix must assert on the structured field.
- The four fixes are genuinely independent. Nothing forces a single ordering, which makes this a good
  phase for parallel waves — unlike Phase 3, where D-18 forced strict sequencing.

</specifics>

<deferred>
## Deferred Ideas

- **Full Cedar expression traces** at any log level — explicitly out of scope; D-02's allowlist exists
  to make them unreachable.
- **A configurable error-body bound** — D-13 reuses the sibling's constant; a knob nobody tunes is not
  worth a config key.
- **Extending field attribution to the operator CLI commands** (`reindex`, `prune-expired`, …) — this
  phase covers the MCP and Connect argument surfaces. Revisit if operators ask.

</deferred>
