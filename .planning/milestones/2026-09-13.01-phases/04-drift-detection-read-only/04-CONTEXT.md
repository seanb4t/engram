# Phase 4: Drift Detection (Read-Only) - Context

**Gathered:** 2026-09-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Preview learns to read a runtime's *existing* engram registration through the runtime's own
read verb, normalize it to the same shape the Plan authors (URL, auth mode, header set), and
classify it as exactly one of three states — `already-correct` / `would-write` / `preserved` —
naming which facet(s) differ when it does, with every header value obtained from a probe
redacted unconditionally before it is stored, rendered, output, or logged. This phase is
**read-only**: no write path changes, no `--apply` branching on the classification (that is
Phase 5), no third-party config file is ever read, and opencode's registration is explicitly
not parsed. It replaces today's write-then-byte-compare heuristic for `already-correct` with a
real pre-write comparison and closes the cleartext leak in `Result.Registered`.

</domain>

<decisions>
## Implementation Decisions

### Carried forward (decided in earlier phases — do not re-ask)
- Plugin-first delivery; **`--apply` is the only consent gate**; 3-way state + facet naming in
  scope; Cursor deferred (`qy29m0j3d2`, 03-CONTEXT.md).
- Header vocabulary is fixed by Phase 2: a header is `HeaderSpec{Name, EnvVar}`; engram renders
  a **bare** env reference in each runtime's own syntax (`${VAR}` on Claude Code, `{env:VAR}` on
  opencode) and never a scheme; the auth-mode header renders first, then extra headers sorted by
  name case-insensitively (02-CONTEXT.md D-04, D-08 — D-08 was made explicitly so this phase can
  compare header *sets* without a spurious ordering facet).
- Codex declines `--header` entirely via `ErrHeaderUnsupported` (02-CONTEXT.md D-09/D-10), so a
  Codex registration engram authored never carries an extra header.
- Table-printing CLI output is parsed as a **coarse name match only, never as structure**
  (03-CONTEXT.md D-11's posture for `plugin marketplace list`, mirroring opencode's `mcp list`).
- The `AUTHORED-HERE` invariant (`internal/setup/plan.go` package doc): every runtime's probe and
  argv live in that runtime's own file; the shared executor stays content-blind and no code path
  outside a runtime's own file special-cases it by name.

### The `preserved` predicate
- **D-01:** A registration is `preserved` **iff the observed entry carries a facet the current
  `Options` do not account for** — an extra header engram was not asked to write, an unknown
  field, a transport engram does not set. If every observed facet maps to something setup itself
  authors from the current `Options`, the difference is reproducible → `would-write`; if nothing
  differs → `already-correct`. Both Claude Code and Codex have whole-entry semantics (remove-then-
  add / overwrite, no partial merge), so the predicate is literally "would my write destroy
  something I cannot re-create". Explicit operator intent on argv does **not** override this: a
  differing `--url` is `would-write`, but an unaccounted-for header on the same entry still makes
  the row `preserved`.
- **D-02:** Header values are **compared raw, in memory, for equality only**; only the redacted
  form is ever carried into the comparison result, rendered, emitted as JSON, or logged.
  Redaction stays **unconditional** — every observed value is redacted regardless of its shape, so
  setup never branches on reference-vs-literal (REQ-drift-redaction). This keeps SC4's
  `header-value-ref` facet detectable (a changed env var reference is reported) while satisfying
  the requirement as written ("before comparison *storage*, rendering, JSON output, or logging").
  Only the **observed** side needs redaction: engram's planned value is a bare `${VAR}` name it
  authored from argv (02-CONTEXT.md D-03 forbids literals on argv), so the planned side renders in
  full — e.g. `x-litellm-api-key: observed <redacted>, would write ${LITELLM_KEY}`.
- **D-03:** `Result.Registered` is **rebuilt from parsed-and-redacted fields**; raw probe text is
  never retained past the parse. Redaction by construction — a value that was never copied out of
  the subprocess buffer cannot leak — rather than a free-text redaction filter that must match
  every runtime's output format. — **Reversibility:** costly — `--output json`'s `registered`
  field is part of the published json contract (PROJECT.md: "json is the contract"); it stays a
  plain scalar string (the `sanitizeViewValue` scalar-only rule) but its *content* changes from a
  bounded raw capture to a normalized rendering, and unrecognized-but-harmless probe chrome stops
  being shown.
- **D-04:** `preserved` is a **non-failed attempt** in `Classify` (`internal/setup/exit.go`):
  alongside `already-correct` / `would-write` / `wrote` it contributes to `hasNonFailedAttempt`,
  so an all-preserved run exits **0**; it yields 8 only beside a genuine `failed` row. Preserving an
  operator's registration is setup performing correctly. `Classify`'s `default` branch treats any
  unplaced Outcome as a failure, so this placement must be made explicitly. — **Reversibility:**
  costly — the 0/8/9 exit taxonomy is documented in `guides/agent-setup.md` and consumed by
  scripts; moving `preserved` later changes CI behavior for every caller.

### The live-verify prerequisite
- **D-05:** The ROADMAP's blocking prerequisite — what each runtime's read verb prints for a header
  whose value is **not** a bare `${VAR}`/`{env:VAR}` reference — is satisfied by a **manual
  observation protocol the maintainer runs once**: register a throwaway MCP entry (not named
  `engram`) carrying a literal header value on Claude Code and Codex, run the read verb, capture
  the verbatim output, remove the entry. Fixtures are then built from observed reality. **No test
  ever invokes a real third-party CLI** (rule `m45p2b4bp7`; the repo's standing discipline that
  verification never touches the operator's `$HOME`). — **Reversibility:** reversible.
- **D-06:** The observation is a **soft gate**: only each runtime's own scanner and its fixtures
  wait on it. Everything shape-independent is built in parallel — the new Outcome value and its
  `Classify` placement, the normalized-registration type, the typed `Facet` vocabulary, the D-01
  predicate, redaction-into-result, the `Registered` rebuild, text/JSON rendering, and the
  `guides/agent-setup.md` results-table row.
- **D-07:** The protocol's scope is the **literal-value echo only**, on Claude Code and Codex. The
  bare-reference echo shape was already observed in the research session; the absent/not-found read
  is already exercised by today's probe handling (`apply.go` renders a nonzero probe exit like a
  zero exit); OAuth-authenticated read-back is Phase 5's concern (REQ-apply-rewrite-consequence).
- **D-08:** The observation is recorded as a dated **`04-OBSERVATIONS.md`** in this phase
  directory — verbatim captured output, the exact commands run, and the CLI versions observed —
  and **every fixture cites it** in a header comment (the `2026-08-23.01` D-10 post-release-
  observation pattern). The verifier checks fixtures against that record.

### Unparseable / coarse rows
- **D-09:** **Ambiguity resolves to `would-write` — never to `already-correct`, never to
  `preserved`.** This extends `apply.go`'s existing "ambiguity resolves to wrote, never to
  already-correct" invariant. `preserved` is always a claim about something **positively observed
  and identified**, so a preserved row can always name what it is preserving. An unreadable
  registration (probe seam error, nonzero probe exit, output the scanner cannot frame) is
  `would-write`, exactly as today. — **Reversibility:** costly — Phase 5's apply-time gate consults
  this classification; flipping ambiguity to `preserved` later would stop opencode registering onto
  any machine with an existing entry.
- **D-10:** **opencode authors no drift classification.** Every present opencode row is
  `would-write`, as today; `guides/agent-setup.md` states plainly that opencode is not compared
  because its `mcp list` prints a table engram declines to parse (`REQ-drift-opencode-structured`
  stays out of scope until opencode ships a `--json` read verb). SC2's "unit coverage of all three
  states per runtime" applies to the **parsed** runtimes — Claude Code and Codex — and opencode's
  exemption is written into the plan/verification artifacts so the verifier does not read it as a
  gap.
- **D-11:** Claude Code's bounded text scan is a **total parse**: every line in the facet-bearing
  region must map to a known facet or known non-facet chrome. Anything left over is, by
  definition, something the `Options` do not account for → `preserved`, with the unrecognized
  content named (redacted) as the reason. This is D-01 enforced structurally: a Claude Code release
  that adds a field makes setup cautious, not blind. Cosmetic output changes may cause spurious
  `preserved` rows until the scanner learns the new shape — accepted. The same totality applies to
  Codex's `mcp get --json`: an unknown key is an unaccounted-for facet.
- **D-12:** The differing facet is a **closed typed `Facet` enum** — `url`, `auth-mode`,
  `header-name`, `header-value-ref`, plus an `unrecognized-content` value for D-11 — with **every**
  differing facet reported in a fixed stable order, rendered from the typed set in both text and
  JSON. This follows the typed-cause-never-message-text discipline (`plan.go` Result doc;
  `cmd/engram/operror.go`'s `classifyOperatorErr`), keeps SC4 unit-testable without string
  matching, and lets `--output json` consumers branch on facets rather than parse prose.

### Claude's Discretion
- Where the comparison lives: a per-runtime `scan`/`observe` function authored in each runtime's
  own file (`claudecode.go`, `codex.go`; `opencode.go` authors none) returning one shared
  normalized-registration value, with the D-01 predicate and facet diff as a pure function in a
  new `internal/setup/drift.go` — vs. widening the `Runtime` interface. Keep the executor
  content-blind either way; the leaf-purity gate (`leafpurity_test.go`) and the AUTHORED-HERE
  invariant both apply.
- The Codex JSON totality mechanism (`json.Decoder.DisallowUnknownFields` on a struct mirroring the
  observed `mcp get --json` shape is the natural stdlib fit — confirm against `04-OBSERVATIONS.md`).
- What a `preserved` row renders for its planned argv — whether `Command` still shows what setup
  *would have* written beside the reason it will not, so the operator can compare by eye.
- The redacted placeholder's exact text (`<redacted>` or similar) and how `Reason`/`Notes`/a new
  facet field carry the typed facet set as flat scalars only
  (`TestOperatorViewFixturesHaveNoUnsanitizedNesting`).
- How Phase 3's plugin facet sits beside a `preserved` registration on the same row — the plugin
  install proceeds independently of registration drift (03-CONTEXT.md D-12 posture).
- Exact wording of the `guides/agent-setup.md` results-table row for `preserved`, including the
  Codex whole-entry sentence REQ-drift-preserved-outcome requires.
- Handling the Claude Code probe's network sensitivity (`claude mcp get` dials the registered URL
  for a user-scope entry — `claudecode.go` Plan doc): a connection-status line must be recognized
  chrome under D-11, or an unreachable server would produce a spurious `preserved`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements
- `.planning/ROADMAP.md` §"Phase 4: Drift Detection (Read-Only)" — goal, the blocking live-verify
  prerequisite, five success criteria
- `.planning/REQUIREMENTS.md` — `REQ-drift-observed-registration`, `REQ-drift-three-way`,
  `REQ-drift-preserved-outcome`, `REQ-drift-facet-naming`, `REQ-drift-redaction`; and the
  out-of-scope `REQ-drift-opencode-structured`
- `.planning/PROJECT.md` — as-built `engram setup` baseline (Outcome taxonomy, exit codes, "json
  is the contract", zero-new-Go-deps constraint)

### Prior-phase decisions this phase builds on
- `.planning/phases/02-custom-auth-headers/02-CONTEXT.md` — header vocabulary (D-01–D-08), Codex
  decline (D-09/D-10); D-08's stable header ordering exists for this phase's set comparison
- `.planning/phases/03-plugin-first-delivery/03-CONTEXT.md` — probe posture (D-10/D-11), plugin
  facet never fails a row (D-12), "where the plugin probe results live for Phase 4's drift
  comparison to reuse" (Claude's Discretion)

### Code contracts
- `internal/setup/plan.go` — `Outcome` enum (five string-backed values; zero value is a programming
  error), `Plan{Runtime, Actions, Probe, Config, Skills}`, `Result` (scalar-only rendered fields;
  `Registered` is the field D-03 rebuilds), the AUTHORED-HERE invariant
- `internal/setup/apply.go` — `execute()`'s nine-step sequence; the preview branch that today sets
  `Result.Registered = displayCapture(raw)` and refuses to classify; the D-08 byte-compare
  `already-correct` heuristic this phase replaces; `displayCapture`/`boundCapture`
- `internal/setup/exit.go` — `Classify`'s exhaustive switch (`default` → failure) where
  `OutcomePreserved` must be placed per D-04
- `internal/setup/runtime.go` — `Options{URL, Auth, ClientID, TokenFile, Headers []HeaderSpec}`,
  `HeaderSpec{Name, EnvVar}`, the `Runtime` interface, `ErrAuthModeUnsupported`/`ErrHeaderUnsupported`
- `internal/setup/claudecode.go` — `Probe: claude mcp get engram` (text; **dials the registered
  URL**), header rendering as `"NAME: ${VAR}"`
- `internal/setup/codex.go` — `Probe: codex mcp get engram --json` (structured), `--bearer-token-env-var`
- `internal/setup/opencode.go` — `mcp list` probe, `--header NAME={env:VAR}`; authors no scanner (D-10)
- `internal/setup/leafpurity_test.go` — the leaf-purity gate any new file must respect
- `cmd/engram/operator_view.go` — `sanitizeViewValue` scalar-only branch;
  `TestOperatorViewFixturesHaveNoUnsanitizedNesting`

### Documentation surface
- `docs-site/src/content/docs/guides/agent-setup.md` §"Read results and repeat safely" (results
  table, lines ~178–186) — gains the `preserved` row and the opencode not-compared statement;
  the exit-code text D-04 keeps stable

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Plan.Probe` per runtime is already authored and already executed once in preview
  (`execute()` step 4); this phase parses its output instead of dumping it.
- `Options.Headers []HeaderSpec` with Phase 2's deterministic ordering is the planned-side header
  set; the observed side normalizes to the same `{Name, EnvVar-or-redacted}` shape.
- `Classify` (`exit.go`) is a pure function with an exhaustive `exit_test.go` combination table —
  adding `OutcomePreserved` extends that table.
- `displayCapture`/`boundCapture`/`quoteWord` (`apply.go`, `quote.go`) remain the rendering path for
  whatever scalar D-03 produces.
- `describeSeamError`/`describeFailure` show the fixed-composition style a preserved `Reason`
  should follow (runtime name, then typed facts, never stderr string-matching).

### Established Patterns
- **Ambiguity resolves away from convergence**: `execute()` never claims `already-correct` from a
  single read; D-09 extends the same invariant to `preserved`.
- **Typed cause, never message text**: outcomes and reasons are composed from typed facts; D-12's
  `Facet` enum follows this.
- **Scalar-only rendered fields**: every rendered `Result` field is a plain string so
  `sanitizeViewValue` cannot be bypassed — the facet set and redacted registration must fold into
  flat scalars, not a nested struct or slice with a json tag.
- **Nothing in `internal/setup` reads a header env var**: `os.Getenv` of a header variable is
  forbidden (02-CONTEXT.md); the observed value from a probe is the only place a literal can appear,
  hence D-02/D-03.
- **Fixtures, not live CLIs**: `apply_test.go` drives `execute()` through a fake `Environment`
  with scripted `RunResult`s; the three-state-per-runtime coverage (SC2) and the literal-value
  redaction proof (SC5) are fixture tests of that shape, built from `04-OBSERVATIONS.md`.

### Integration Points
- `execute()`'s `!mutate` branch (`apply.go`): replace `res.Registered = displayCapture(raw)` with
  scan → normalize → compare → classify → redact → render. The `mutate` branch is **untouched** in
  this phase (Phase 5 consumes the classification there).
- `Outcome` (`plan.go`) gains `OutcomePreserved`; `Classify` (`exit.go`) places it; `cmd/engram/
  setup.go`'s row renderer and `guides/agent-setup.md` learn the value.
- Per-runtime scanners live in `claudecode.go` / `codex.go`; `opencode.go` and `generic.go` author
  none (generic has no probe and no actions — `execute()` step 2a returns before any probe).
- Phase 3's plugin facet is computed from the same `Plan()` call and is independent of
  registration drift — a `preserved` registration row still carries its plugin/skills facets.

</code_context>

<specifics>
## Specific Ideas

- The incident this closes is gotcha `ryr82bf2s2` (2026-09-10): `engram setup --apply` overwrote
  the maintainer's hand-configured gateway registration. The acceptance shape the user cares about
  is: an existing Claude Code entry carrying `x-litellm-api-key: <literal>` that setup was not asked
  to write reads as `preserved` naming that header, with the literal never appearing anywhere.
- Facet message shape the user endorsed: `x-litellm-api-key: observed <redacted>, would write
  ${LITELLM_KEY}` — observed side redacted, planned side in full.
- The manual protocol should register a throwaway entry **not named `engram`** so the maintainer's
  real registration is never touched.

</specifics>

<deferred>
## Deferred Ideas

- Full-fidelity opencode comparison — `REQ-drift-opencode-structured`, revisit if opencode gains a
  `--json` read verb.
- Capturing bare-reference, absent-read, and OAuth-authenticated read-back shapes in the same
  observation sitting was offered and declined; OAuth read-back belongs to Phase 5
  (REQ-apply-rewrite-consequence) if it needs a fresh observation.
- A durable engram gotcha recording the observed echo behavior was offered and declined in favor of
  the phase artifact alone (CLI-version-specific observations go stale in the spine).

</deferred>

---

*Phase: 04-drift-detection-read-only*
*Context gathered: 2026-09-15*
