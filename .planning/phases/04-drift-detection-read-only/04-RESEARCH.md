# Phase 4: Drift Detection (Read-Only) - Research

**Researched:** 2026-09-15
**Domain:** Go stdlib CLI-output parsing and comparison (`internal/setup`), extending an existing shell-out executor — no new external libraries, no third-party config-file parsing.
**Confidence:** MEDIUM-HIGH — the executor, aggregation, and per-runtime `Plan()`/`Probe` architecture is fully read and cited from source; the one HIGH-confidence gap (the literal-header-value echo shape) is an explicit, already-scheduled human checkpoint (D-05), not a research gap this document can close.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Carried forward (decided in earlier phases — do not re-ask):**
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

**The `preserved` predicate:**
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

**The live-verify prerequisite:**
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

**Unparseable / coarse rows:**
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

### Deferred Ideas (OUT OF SCOPE)
- Full-fidelity opencode comparison — `REQ-drift-opencode-structured`, revisit if opencode gains a
  `--json` read verb.
- Capturing bare-reference, absent-read, and OAuth-authenticated read-back shapes in the same
  observation sitting was offered and declined; OAuth read-back belongs to Phase 5
  (REQ-apply-rewrite-consequence) if it needs a fresh observation.
- A durable engram gotcha recording the observed echo behavior was offered and declined in favor of
  the phase artifact alone (CLI-version-specific observations go stale in the spine).
- `--apply`'s write-time gate (REQ-apply-preserve-gate, REQ-apply-rewrite-consequence) — Phase 5.
- `guides/install.md`/`guides/plugin.md` full documentation pass (REQ-docs-setup-v2) — Phase 5.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-drift-observed-registration | Preview reads the runtime's existing registration through its own read verb, normalizes to the shape `Plan` authors (URL, auth mode, header names); Codex's `mcp get --json` as structured input, a bounded text scan for Claude Code; opencode not parsed; no third-party config file read | Architecture Patterns (per-runtime scan functions), Code Examples (verbatim probe shapes), Common Pitfalls 1/2 |
| REQ-drift-three-way | Exactly one of `already-correct` / `would-write` / `preserved`, never collapsed; real comparison with unit coverage of all three states per runtime | Architecture Patterns (Preview's classification model changes), Validation Architecture |
| REQ-drift-preserved-outcome | `preserved` is first-class in text/JSON, reason names what cannot be reproduced, consistent in aggregation/exit codes, documented with Codex's whole-entry semantics | Code Examples (`Classify`/`AggregateOutcome` extension sites), Validation Architecture |
| REQ-drift-facet-naming | A `would-write` row names which facet(s) differ (URL, auth mode, header name, value reference) | Code Examples (typed `Facet` enum), Don't Hand-Roll |
| REQ-drift-redaction | Header values from any probe redacted unconditionally before storage/render/JSON/log; proven by a literal-value fixture | Common Pitfalls (secret-leak vector), Validation Architecture, Security Domain |
</phase_requirements>

## Summary

This phase replaces `internal/setup`'s **write-then-byte-compare** `already-correct` heuristic
(`apply.go`'s `execute()`, the mutate-branch's two-probe-read comparison) with a real **single-read
comparison against `Plan`'s own authored intent**, but **only in the preview (`!mutate`) branch**.
Today, `apply.go` states explicitly and repeatedly that a single probe read has "no honest basis
for claiming already-correct" — Preview *always* returns `OutcomeWouldWrite`, regardless of what
the probe shows. Phase 4 overturns that rule for Preview specifically: the probe output is now
**parsed and compared against what `Options` would author**, so a single read can honestly answer
"is this already right," "would this change something," or "does this carry something I cannot
reproduce." The `mutate` (Apply) branch's own two-read byte-compare for `wrote` vs `already-correct`
is explicitly **untouched this phase** — the write path itself doesn't consult drift yet
(that's Phase 5, `REQ-apply-preserve-gate`).

The architecture has a strong, already-shipped precedent to imitate almost verbatim: Phase 3's
`PluginRuntime` optional interface (`internal/setup/plugin.go`) is structurally the same shape this
phase needs — an interface only `claude-code` and `codex` implement, parsed output folded into a
typed state, composed as a facet in `cmd/engram`, with the shared executor staying content-blind.
Nothing about `execute()`'s core loop, `Environment`, or `RunResult` needs to change; the work is
new **parsing and comparison logic**, authored per-runtime (AUTHORED-HERE), landing in a new shared
pure-function file (`drift.go`, mirroring `aggregate.go`'s single-declared-precedence-table
pattern) plus one new `Outcome` value threaded through `plan.go`/`exit.go`/`aggregate.go`/
`cmd/engram/setup.go` — four sites, all already known from the Phase 3 precedent of adding a facet.

The one finding this research cannot close is the phase's own stated blocking prerequisite: **no
verbatim capture of a literal (non-reference) header value being echoed back by either `claude mcp
get` or `codex mcp get --json` exists anywhere in this repository's committed research.** The prior
session redacted the literal value *before it left the session* (`STACK.md`'s own Sources section:
"values redacted before leaving this session") — it recorded the *fact* that both CLIs echo
cleartext, never the *exact byte shape* of that echo. This is precisely why 04-CONTEXT.md's D-05
schedules a fresh, dated `04-OBSERVATIONS.md` capture as a human checkpoint before any redaction
code is trusted — this document drafts the exact protocol commands so the plan can carry them
verbatim as a `checkpoint:human-verify` task.

**Primary recommendation:** Model the new capability as an optional `DriftRuntime`-shaped interface
(or, per Claude's Discretion, a bare per-runtime `scan` function plus one shared `drift.go` pure
comparison) implemented only by `claude-code` and `codex` — exactly the `PluginRuntime` precedent —
and change **only** `execute()`'s `!mutate` branch to consult it; leave `Apply()`'s mutate branch,
`Environment`, `RunResult`, and every write `Action` untouched.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Probe execution (unchanged) | `internal/setup` (`apply.go` `execute()`) | — | Already the sole process boundary; this phase adds no new subprocess calls |
| Per-runtime probe-output parsing (new) | `internal/setup` (`claudecode.go`, `codex.go`) | — | AUTHORED-HERE invariant: only the runtime's own file may know its own output shape |
| Redaction of observed values (new) | `internal/setup` (parse/normalize boundary) | — | D-03: redaction happens at extraction, so a raw value is never copied into any struct that could later render |
| Three-way classification + facet diff (new) | `internal/setup` (`drift.go`, pure function) | — | Centrally auditable single predicate (D-01), mirrors `aggregate.go`'s single `precedenceOrder` |
| `Outcome`/`Classify`/`AggregateOutcome` vocabulary widening (new) | `internal/setup` (`plan.go`, `exit.go`, `aggregate.go`) | — | Five-value enum becomes six; three exhaustive-switch sites already exist and are the known extension points |
| Row composition / facet rendering (new field) | `cmd/engram` (`setup.go`) | — | `setupRuntimeRowFromResult`/`setupApplySummary` are the existing facet-composition layer (Phase 3's plugin facet precedent) |
| Docs (results table + Codex whole-entry sentence) | Documentation (`docs-site`) | — | `guides/agent-setup.md`'s existing five-row Outcome table gains a sixth row |
| Literal-value echo ground truth (blocking, human) | Human checkpoint (`04-OBSERVATIONS.md`) | `internal/setup` scanners/fixtures | D-05/D-06: soft-gates only the scanners and their fixtures, not the shape-independent plumbing |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` | Go stdlib (module Go 1.26.3) | Decode `codex mcp get --json`'s structured output with `json.Decoder.DisallowUnknownFields` for D-11's totality parse | Already imported in `codex.go`/`claudecode.go` for the plugin-list JSON shapes (Phase 3 precedent); zero new dependency |
| `strings` | Go stdlib | Line-scan Claude Code's fixed-label text block (`Scope:`/`Status:`/`Type:`/`URL:`/`Headers:`) | Already the mechanism `claudecode.go`'s `ParseMarketplaceList` uses for a different coarse text parse; matches the repo's established "small, bounded scan, never a general parser" discipline |
| `sort` (via existing `sortedHeaders`) | Go stdlib, already used | Deterministic header-set comparison | `runtime.go`'s `sortedHeaders` already gives a total order for both the planned and observed header sets — reuse, do not reimplement |

No external packages are installed or proposed this phase — every capability is new **data and
logic** inside `internal/setup`'s existing stdlib-only leaf package, enforced mechanically by
`TestSetupPackageIsStdlibOnlyLeaf` (`internal/setup/leafpurity_test.go:88-134`), which fails the
build if any non-stdlib or same-module import appears in a non-test `.go` file under
`internal/setup/`.

**Installation:** none — no `go.mod` change, no `go get`.

**Version verification:** N/A — no package versions to pin. The two third-party CLI surfaces this
phase parses (`claude mcp get`, `codex mcp get --json`) are pinned by the *content* of
`04-OBSERVATIONS.md` (dated, versioned per D-08), not by a `go.mod` entry.

## Package Legitimacy Audit

> Not applicable — this phase installs zero external packages. `internal/setup`'s stdlib-only
> constraint is enforced mechanically by `TestSetupPackageIsStdlibOnlyLeaf`
> (`internal/setup/leafpurity_test.go:88-134`), which parses every non-test `.go` file's imports
> and fails on any import whose first path segment contains a `.` (non-stdlib) or whose path is
> prefixed by this module's own path (same-module import, violating the leaf-package direction).

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
                 engram setup  (preview, no --apply)
                        │
                        ▼
        setupPlanDoc → setupBuildRows → setup.Preview(rt, opts)
                        │
                        ▼
        ┌───────────────────────────────────────────────┐
        │ execute(ctx, env, rt, opts, mutate=false)      │  internal/setup/apply.go
        │  1. rt.Detect → not-present short-circuit      │
        │  2. rt.Plan(env, opts) → authored Actions/Probe│  ◄── the "expected" side:
        │  3. resolve binary (LookPath)                  │      opts.URL / opts.Auth /
        │  4. runSeam(Plan.Probe) → raw probe1 (RunResult)│      sortedHeaders(opts.Headers)
        │  5. [NEW] if Probe wired AND runtime implements│
        │     the drift-capable interface:               │
        │       observed, ok := rt.Scan(probe1.Stdout+   │  ◄── claudecode.go / codex.go
        │                              probe1.Stderr)     │      "AUTHORED HERE" parsers
        │       if ok:                                    │      (per-runtime file, D-11
        │         outcome, facets := drift.Compare(       │       total parse)
        │             observed, opts, plan)                │
        │       else: OutcomeWouldWrite (D-09 ambiguity)   │
        │     else (opencode/generic): OutcomeWouldWrite   │
        │         (D-10, unchanged today's behavior)        │
        │  6. Result.Registered = REBUILT from the parsed  │  ◄── D-03: never raw bytes past
        │     + REDACTED observed value, never raw probe1  │      the parse step
        └───────────────────────────────────────────────┘
                        │
                        ▼
        cmd/engram/setup.go: setupRuntimeRowFromResult
          composes registration facet + plugin facet (Phase 3)
          + skills facet via AggregateOutcome, adds the new
          typed-facet-diff field(s) as flat scalars
                        │
                        ▼
              renderOperator (text / --output json)
```

The `mutate=true` (Apply) branch of `execute()` is **not shown as changed** — it keeps its existing
two-read byte-compare unconditionally (steps 6-9 of `apply.go`'s own doc comment,
`internal/setup/apply.go:213-260`) and does not consult `drift.Compare` at all this phase. Phase 5
is where the write path first reads this phase's classification.

### Recommended Project Structure
```
internal/setup/
├── plan.go          # Outcome gains OutcomePreserved (one new const + doc-comment update)
├── exit.go           # Classify's switch gains a case placing OutcomePreserved with the
│                      #   already-correct/would-write/wrote non-failed group (D-04)
├── aggregate.go       # precedenceOrder gains OutcomePreserved at an explicit, reasoned position
│                      #   (see Open Questions — CONTEXT.md does not pin this)
├── drift.go           # NEW: Facet enum, ObservedRegistration-shaped type, the single D-01
│                      #   predicate + facet-diff pure function — mirrors aggregate.go's
│                      #   single-declared-table discipline
├── claudecode.go      # gains a scan/parse function for `claude mcp get` output (AUTHORED HERE)
├── codex.go           # gains a scan/parse function for `codex mcp get --json` (AUTHORED HERE)
├── opencode.go         # UNCHANGED — authors no scanner (D-10)
├── apply.go            # execute()'s !mutate branch rewritten to consult the new capability;
│                        #   mutate branch untouched this phase
└── *_test.go            # new fixture tests per REQ, following the scriptedRun/fakeEnv harness

cmd/engram/
├── setup.go            # setupRuntimeRowFromResult composes the new facet-diff field(s);
│                        #   setupApplySummary's tally gains a preserved count
└── operator_view_setup_test.go  # fixture rows extended with a preserved example (flat scalars)

docs-site/src/content/docs/guides/
└── agent-setup.md      # results table gains a `preserved` row + Codex whole-entry sentence

.planning/phases/04-drift-detection-read-only/
└── 04-OBSERVATIONS.md  # NEW (D-08): dated, verbatim literal-echo capture + CLI versions;
                          #   every new fixture cites it in a header comment
```

### Pattern 1: Optional per-runtime capability interface (the `PluginRuntime` precedent)

**What:** An exported interface only some `Runtime` implementations satisfy, type-asserted exactly
once at the capability's entry point — never a by-name branch.
**When to use:** Exactly this phase's situation: a capability (parsing observed registration state)
that only `claude-code` and `codex` can honestly support, while `opencode`/`generic` cannot (D-10).
**Example — the exact shape already shipped for the plugin facet, worth imitating closely:**
```go
// Source: internal/setup/plugin.go:75-93 (read this session, verbatim)
type PluginRuntime interface {
	PluginProbes() (list, marketplace []string)
	ParsePluginList(stdout string) (version string, installed bool, err error)
	ParseMarketplaceList(stdout string) (present bool, source string)
	PluginActions(state PluginState, marketplacePresent bool) []Action
}
```
`ClaudeCode` and `Codex` implement `PluginRuntime`; `OpenCode` and `Generic` do not — the caller
(`internal/setup/plugin.go`'s `executePlugin`) type-asserts once and falls back to
`PluginUnavailable` when the assertion fails, never producing a failed row for that alone. A new
`DriftRuntime`-shaped interface (or, per Claude's Discretion, a plainer pair of functions —
`Scan(probeOutput string) (ObservedRegistration, bool)` — is equally consistent with this
precedent and slightly cheaper) should follow the identical shape: implemented by `claudecode.go`
and `codex.go` only, asserted once inside `execute()`'s `!mutate` branch, falling back to today's
`OutcomeWouldWrite` (D-09) when the assertion fails or the scan itself reports it could not parse.

### Pattern 2: Single-declared precedence/vocabulary table (the `aggregate.go` precedent)

**What:** A widened enum's ordering/classification rule lives in exactly ONE place, referenced by
every consumer, so the doc comment and the code cannot drift from each other.
**When to use:** Both `Classify`'s switch (`exit.go`) and `AggregateOutcome`'s fold
(`aggregate.go`) need to learn `OutcomePreserved`; both already model this discipline for the
existing five values.
**Example:**
```go
// Source: internal/setup/aggregate.go:8-16 (read this session, verbatim)
var precedenceOrder = []Outcome{
	OutcomeFailed,
	OutcomeWrote,
	OutcomeAlreadyCorrect,
	OutcomeWouldWrite,
	OutcomeNotPresent,
}
```
`isRecognizedOutcome` iterates this SAME slice, so adding `OutcomePreserved` here is the one edit
that keeps `AggregateOutcome`'s "unrecognized outcome in either argument = failed" invariant intact
for the new value automatically. The exact insertion POINT (`aggregate.go`'s own list; `Classify`
only needs a `case OutcomePreserved:` added to the `hasNonFailedAttempt` group, per D-04 — it does
not need a precedence position since `Classify` has no ordering, only a two-way partition) is an
open question this research flags rather than resolves (see Open Questions).

### Pattern 3: Redaction by construction, never a redaction filter (D-03)

**What:** A raw probe capture is parsed into typed fields at the SAME step it is redacted; nothing
downstream (`Result.Registered`, the facet diff, `--output json`) ever holds a byte range that was
copied unmodified from subprocess output.
**When to use:** Every header value extracted from ANY runtime's probe output, unconditionally
(D-02) — never conditioned on whether the value "looks like" a `${VAR}` reference.
**Example (the shape to follow, adapting `displayCapture`'s existing bound-then-render idiom):**
```go
// Illustrative shape — not yet implemented; follows apply.go's existing
// displayCapture(boundCapture(s)) two-step discipline (internal/setup/apply.go:96-99,
// read this session, verbatim) but redacts instead of merely bounding.
type ObservedRegistration struct {
	Parseable bool              // false => D-09 ambiguity: caller falls back to OutcomeWouldWrite
	URL       string            // rendered in full — a URL is not a secret
	AuthMode  string             // e.g. "bearer" — a mode name, not a secret
	Headers   []ObservedHeader   // Name in full; Value ALWAYS "<redacted>" — never the parsed value
}
type ObservedHeader struct {
	Name  string
	Value string // constant redacted placeholder; the raw parsed value is discarded at this line
}
```
The raw parsed value must never be assigned to a struct field that survives past the comparison —
compare it in a local variable, in the SAME function, and let it go out of scope. This is stricter
than `displayCapture`'s existing bound-and-quote discipline (which preserves bounded content for
display); here the content itself must never reach a renderable field.

### Anti-Patterns to Avoid
- **A shared cross-runtime output parser:** exactly the anti-pattern `claudeCodeHeaderArgs`/
  `openCodeHeaderArgs`'s own doc comments warn against on the write side (the opencode
  colon-space regression) — a shared parser for two structurally different output formats (one
  JSON, one fixed-label text) invites the same class of silent format-mismatch bug. Keep each
  scan function in its own runtime's file.
- **Regex-based literal-vs-reference detection:** `STACK.md`'s own "What NOT to Use" table already
  rejects this explicitly — "never try to distinguish 'safe-looking' from 'unsafe-looking' values."
  D-02 codifies this as unconditional redaction.
- **Parsing `opencode mcp list`'s table:** already explicitly rejected twice (`opencode.go`'s own
  doc comment, and `PITFALLS.md`'s Pitfall 7) — do not "improve" opencode's coverage in this phase.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Reading Codex's underlying registration state | A TOML parser for `~/.codex/config.toml` | `codex mcp get <name> --json`, decoded with `encoding/json` | Already decided out of scope repo-wide (REQUIREMENTS.md's Out of Scope table); `codex mcp add` already owns that file, and engram never reads it directly (standing constraint, PROJECT.md) |
| Reading Claude Code's underlying registration state | A parser for `~/.claude.json` | `claude mcp get <name>`, line-scanned for its documented fixed labels | Same standing constraint; the CLI's own read verb is the only sanctioned surface |
| Distinguishing a literal secret from a `${VAR}` reference | A regex/heuristic classifier | Unconditional redaction (D-02) | A literal secret and a reference are visually indistinguishable in the general case (`STACK.md`); the cost of guessing wrong is a leaked credential |
| Comparing opencode's registration structurally | A box-drawing-table parser for `opencode mcp list` | Coarse `would-write` for every present opencode row (D-10) | opencode's own doc comment already rejects this; no `--json`, no `get`, dials every registered server on every call — noise, not signal |
| Detecting CLI flag/output-shape drift ahead of time | A `--help` scrape or version-floor gate | Post-hoc reporting only (D-11's total-parse: an unrecognized field becomes `preserved`, never a crash) | `apply.go`'s own doc comment already states why: a `--help` scrape is scraping a format that drifts as easily as the flag surface it claims to protect, and a version floor gates the wrong proxy |

**Key insight:** every "don't hand-roll" item in this table is not a generic library-avoidance
concern — it is a repo-wide, already-documented, already-decided constraint (either in
`REQUIREMENTS.md`'s Out of Scope table or in a sibling file's own doc comment) that this phase's
own scope explicitly reaffirms rather than reopens.

## Runtime State Inventory

> Not applicable — this is not a rename/refactor/migration phase. No stored data, service config,
> OS-registered state, secrets, or build artifacts are renamed or relocated by this phase.

## Common Pitfalls

### Pitfall 1: Assuming Preview's "single read has no honest basis for already-correct" rule still holds
**What goes wrong:** `apply.go`'s own doc comment for `execute()` states, twice, that a single probe
read in Preview can never honestly produce anything but `OutcomeWouldWrite` — "D-08's byte-compare
needs a WRITE between two reads." A plan or implementation that treats this as still-true and tries
to bolt drift detection onto the `mutate` (Apply) branch instead of `!mutate` (Preview) would ship
a feature that only activates on `--apply`, directly contradicting SC1 ("Preview reads...") and the
whole phase's premise (preview-only, no `--apply` branching this phase).
**Why it happens:** The comment is correct for the OLD comparison basis (two-reads-for-convergence)
and reads as a load-bearing invariant of the package, not scoped to that one comparison method.
**How to avoid:** The new comparison basis is fundamentally different — comparing ONE observed read
against the ALREADY-KNOWN expected value (`Options`/`Plan`), never against a second read. A single
read is dishonest evidence for "did this change between two points in time" but perfectly honest
evidence for "does this match what I already know I would write." State this distinction explicitly
in the rewritten doc comment so a future reader does not "fix" it back.
**Warning signs:** A plan that only touches `apply.go`'s `mutate == true` branch, or that adds
`OutcomePreserved` without touching the `!mutate` block at all.

### Pitfall 2: Trusting the literal-value echo shape without the D-05 observation
**What goes wrong:** Building the redaction path (D-02/D-03) and its fixture tests against an
ASSUMED echo shape (e.g. "probably the same `Headers:\n  Name: <value>` block, just with the raw
secret instead of `${VAR}`") risks a fixture that never matches real CLI output, silently
undermining the exact leak this requirement exists to close (REQ-drift-redaction).
**Why it happens:** The bare-`${VAR}` echo case WAS live-verified this milestone (03-RESEARCH.md,
`claude mcp get` and the codex JSON `bearer_token_env_var` field both echo only the reference name)
— it is easy to assume the literal case degrades the same way without re-checking.
**How to avoid:** Treat D-05's manual protocol as a hard blocker for the scanner/fixture work
specifically (D-06's soft gate), not the whole phase. Draft the exact commands now (see Code
Examples, "D-05 Manual Observation Protocol") so the checkpoint task is concrete and immediately
actionable rather than open-ended.
**Warning signs:** A `04-OBSERVATIONS.md` that restates the ALREADY-KNOWN bare-reference finding
instead of the NEW literal-value finding, or fixture comments citing `03-RESEARCH.md` instead of
`04-OBSERVATIONS.md` for the literal-echo shape.

### Pitfall 3: A shared struct/formatter across Claude Code's text scan and Codex's JSON decode
**What goes wrong:** Reaching for one `parseRegistration(runtimeName, output string)` dispatcher
function reproduces the exact "shared cross-runtime formatter" anti-pattern this codebase has
already been burned by twice on the WRITE side (`opencode.go`'s colon-space regression,
`claudecode.go`'s own header-rendering doc comment naming it explicitly).
**Why it happens:** Both scans ultimately produce the SAME `ObservedRegistration`-shaped output, so
factoring the shared RESULT type looks like it implies a shared PARSING function too.
**How to avoid:** Share the output TYPE (`ObservedRegistration`/`Facet`), never the parsing logic.
Each runtime's file owns its own scan function end-to-end, exactly as `ParsePluginList` (JSON) and
`ParseMarketplaceList` (coarse text) already coexist in the SAME `PluginRuntime` interface without
sharing an implementation.
**Warning signs:** A new file (e.g. `internal/setup/scan.go`) containing per-runtime `if runtime ==
"claude-code"` branches — a by-name special case the package's own doc comment forbids.

### Pitfall 4: Claude Code's network-dial connection-status line breaking D-11's total parse
**What goes wrong:** `claude mcp get` dials the registered URL for a `--scope user` entry and
prints a `Status: ✘ Failed to connect` / connected line (live-observed, `03-RESEARCH.md`). Under
D-11's total-parse rule ("every line must map to a known facet or known non-facet chrome"), if this
status line is not explicitly recognized as chrome, EVERY Claude Code registration whose target
server is momentarily unreachable becomes spuriously `preserved` (an "unaccounted-for" line) rather
than correctly classified.
**Why it happens:** The status line's exact text varies with the live connection outcome (success/
failure/timeout), unlike the other fixed labels (`Scope:`, `Type:`, `URL:`, `Headers:`) whose
label text is constant even though their values vary.
**How to avoid:** Explicitly special-case the `Status:` line's PREFIX (never its full text) as
recognized chrome regardless of what follows it, mirroring how the shared executor already treats
a nonzero probe exit as informational rather than diagnostic (D-11 of `apply.go`, "reports rather
than diagnoses").
**Warning signs:** A `preserved` row appearing intermittently for a Claude Code registration whose
`Options` genuinely match — correlated with target-server reachability, not with any real config
change. `03-CONTEXT.md`'s own "Claude's Discretion" list already names this exact risk.

### Pitfall 5: Forgetting Codex's whole-entry semantics when composing the `preserved` reason
**What goes wrong:** REQ-drift-preserved-outcome explicitly requires the reason to name Codex's
"preserve-or-overwrite, no partial merge" semantics. A reason string that only names the differing
facet (e.g. "unrecognized header x-foo") without stating that `--apply` cannot partially update
just that one field understates the operator-facing risk `codex mcp add`'s silent whole-entry
overwrite actually carries (`PITFALLS.md`'s Pitfall 10, already documented in `codex.go`'s own doc
comment: "Codex is the only registered runtime whose `mcp add` genuinely overwrites an existing
entry silently").
**Why it happens:** The facet-naming mechanism (D-12) and the whole-entry-semantics documentation
requirement (SC3) are two different sentences in the same requirement, easy to satisfy one and
miss the other.
**How to avoid:** Treat the Codex whole-entry sentence as a FIXED, composed-once string (mirroring
`describeFailure`'s fixed-order composition discipline), appended to every Codex `preserved`
row's reason — not left to `guides/agent-setup.md` prose alone, since SC3 requires it "reflected
consistently in aggregation and exit codes" too.

## Code Examples

### Verbatim: Claude Code's fixed-label text-block shape (the D-11 total-parse target)

The label SET (never a full worked example with all labels populated) is documented, HIGH
confidence:

```
Source: .planning/research/STACK.md:44 (read this session, verbatim) — codified this milestone
against a real registration.
"claude mcp get" has no `--json` ... its human output is a small, fixed set of labelled lines
(`Scope:`, `Status:`, `Type:`, `URL:`, `Headers:` followed by indented `key: value` lines)
```

The Headers block's shape for a BARE-REFERENCE header value (the one case already captured
verbatim, from a live end-to-end reproduction):

```
Source: .planning/milestones/2026-08-23.01-phases/03-runtime-registration/03-RESEARCH.md:428-431
(read this session, verbatim) — live against claude 2.1.265.
  Headers:
    Authorization: Bearer ${ENGRAM_TEST_TOKEN}
```

**No verbatim capture of the LITERAL-value echo case exists in this repository.** The prior
session redacted it before recording (`STACK.md`'s own Sources section: `claude mcp get engram`
"(values redacted before leaving this session)"). This is exactly the gap D-05's manual protocol
(below) must close before any fixture or comparison code that touches this shape is trusted.

### Verbatim: Codex's `mcp get --json` structured shape

```json
// Source: .planning/milestones/2026-08-23.01-phases/03-runtime-registration/03-RESEARCH.md:460-468
// (read this session, verbatim) — live against codex-cli 0.153.4, an isolated CODEX_HOME.
{
  "name": "probe", "enabled": true, "disabled_reason": null,
  "transport": {
    "type": "streamable_http", "url": "http://x",
    "bearer_token_env_var": "ENGRAM_TOKEN",
    "http_headers": null, "env_http_headers": null, "http_headers_helper": null
  },
  "enabled_tools": null, "disabled_tools": null,
  "startup_timeout_sec": null, "tool_timeout_sec": null
}
```

`http_headers`/`env_http_headers` were `null` in this capture because the registration used only
`--bearer-token-env-var` (no `--header`, which `codex mcp add` doesn't expose at all — D-09/D-10 of
02-CONTEXT.md). Whether a registration carrying a literal or referenced custom header populates
`http_headers` (a map) or `env_http_headers`, and in what shape, is UNVERIFIED — this is IN SCOPE
for D-05's manual protocol, since the operator's real gateway registration (the incident this
phase closes, gotcha `ryr82bf2s2`) almost certainly used one of these two fields. `D-11`'s totality
parse (`json.Decoder.DisallowUnknownFields` against a struct mirroring this exact shape) means an
unrecognized key at ANY level — including inside a not-yet-observed `http_headers` shape — safely
becomes `preserved` rather than silently dropped, which is the structural safety net for this gap.

### D-05 Manual Observation Protocol (draft — carry into the plan as a `checkpoint:human-verify` task)

Read-only verbs only reach the automated test suite; the REGISTRATION step below is a one-time,
human-run, non-automated action against the maintainer's own machine, matching D-05's own framing
("a manual observation protocol the maintainer runs once"). Both throwaway names below are
deliberately NOT `engram`, so the maintainer's real registration is never touched (04-CONTEXT.md's
own "Specifics" section).

**Claude Code:**
```bash
# 1. Register a throwaway entry carrying a LITERAL (non-reference) header value.
#    Point the URL at an address that will never actually answer, to keep the
#    live network dial (claudecode.go's own documented behavior) harmless.
claude mcp add --transport http probe-literal-04 http://127.0.0.1:1/mcp \
  --scope user --header 'x-litellm-api-key: sk-DO-NOT-COMMIT-literal-test-abc123'

# 2. Run the read verb and capture stdout+stderr VERBATIM.
claude mcp get probe-literal-04 > /tmp/04-observe-claude.txt 2>&1
cat /tmp/04-observe-claude.txt

# 3. Remove the throwaway entry immediately.
claude mcp remove probe-literal-04 --scope user
```

**Codex:** `codex mcp add` has no header flag at all (D-09/D-10), so a literal header value can
only reach a registration by hand-editing the underlying `config.toml` — exactly the shape an
operator's real gateway tooling, or a hand-edit, would produce (the actual incident this phase
closes). Use an **isolated `CODEX_HOME`** so the maintainer's real Codex config is never touched at
all (the same isolation `03-RESEARCH.md` already used live):
```bash
export CODEX_HOME=/tmp/engram-04-observe
mkdir -p "$CODEX_HOME"
codex mcp add probe-literal-04 --url http://127.0.0.1:1/mcp --bearer-token-env-var DUMMY_04

# Hand-edit $CODEX_HOME/config.toml: under [mcp_servers.probe-literal-04.transport],
# add a literal (non-reference) header value, e.g.:
#   http_headers = { "x-litellm-api-key" = "sk-DO-NOT-COMMIT-literal-test-abc123" }

codex mcp get probe-literal-04 --json > /tmp/04-observe-codex.json
cat /tmp/04-observe-codex.json

rm -rf "$CODEX_HOME"
```

**Recording:** capture BOTH files' content verbatim into `04-OBSERVATIONS.md`, with the literal
test value clearly marked as a dummy (never a real credential), the exact commands run, and
`claude --version`/`codex --version` output. Every fixture the scanners' tests build must cite this
file in a header comment (D-08).

### Illustrative: the totality-parse Go shape for Codex's JSON (pattern, not yet implemented)

```go
// Pattern to follow — internal/setup/codex.go already has the sibling precedent
// (codexPluginListDoc, read this session verbatim):
//   type codexPluginListDoc struct {
//       Installed *[]codexPluginListEntry `json:"installed"`
//   }
// D-11's totality parse for the REGISTRATION shape additionally needs
// DisallowUnknownFields so an unrecognized top-level or transport-nested key
// is a decode ERROR (mapped to "unrecognized-content", i.e. preserved) rather
// than silently ignored — encoding/json's default (silently drop unknown
// keys) is the WRONG default for this specific use, unlike the plugin-list
// parse above, which intentionally tolerates extra fields.
dec := json.NewDecoder(strings.NewReader(probeOutput))
dec.DisallowUnknownFields()
var doc codexRegistrationDoc
if err := dec.Decode(&doc); err != nil {
    // D-11: any unrecognized field -> preserved, never a crash, never dropped.
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Preview always classifies `OutcomeWouldWrite`; a single read is treated as too weak to trust | Preview classifies `already-correct` / `would-write` / `preserved` from ONE read, compared against the already-known `Options`/`Plan`, not against a second read | This phase | `apply.go`'s own doc comment (steps 1-9) must be rewritten for the `!mutate` branch; the `mutate` branch's two-read byte-compare (D-08) is UNCHANGED |
| `Result.Registered` is a bounded, quoted VERBATIM capture of raw probe stdout+stderr (`displayCapture(raw)`) | `Result.Registered` is REBUILT from parsed-and-redacted fields; raw text never survives past the parse | This phase (D-03) | `--output json`'s `registered` field's CONTENT changes shape even though its TYPE (plain string) does not; unrecognized-but-harmless probe chrome stops being shown verbatim |
| Five-value `Outcome` enum; `Classify`/`AggregateOutcome` exhaustive over exactly those five | Six-value enum (`OutcomePreserved` added) | This phase | Three exhaustive-switch sites (`exit.go`, `aggregate.go`'s `precedenceOrder`, `aggregate_test.go`'s `allOutcomes`) all need the new value, or their own exhaustiveness gates fail loudly (by design) |

**Deprecated/outdated:** the "ambiguity resolves to wrote, never already-correct" framing
(`apply.go`'s D-08 comment) stays TRUE for the `mutate` branch but is now only HALF the story —
Preview's new D-09 companion rule ("ambiguity resolves to would-write, never already-correct, never
preserved") governs the `!mutate` branch instead. A future reader must not conflate the two.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The literal-header-value echo shape (both Claude Code and Codex) will resemble the already-observed bare-reference shape closely enough that the SAME field-extraction code (just without the `${...}` unwrap) works — i.e. the literal value appears in the SAME position/field as the reference does today | Common Pitfalls 2, Code Examples | Medium — if the literal case instead triggers a DIFFERENT output shape (e.g. Codex's `http_headers` map appears only for literal values, never for env-var-backed ones, or Claude Code prints an additional warning line for a non-`${VAR}` value), the scanner needs a structurally different branch, not just a redaction step. This is exactly why D-05/D-06 gate the scanners specifically. |
| A2 | `OutcomePreserved`'s exact position in `aggregate.go`'s `precedenceOrder` slice does not affect correctness for THIS phase, because no plan/mutate-path code yet folds a `preserved` registration facet against a plugin/skills facet in a way where ordering changes the visible result | Architecture Patterns (Pattern 2), Open Questions | Low-Medium — `ARCHITECTURE.md`'s own research (a DIFFERENT document, predating 04-CONTEXT.md's locked decisions) recommended inserting it between `OutcomeAlreadyCorrect` and `OutcomeWouldWrite`; 04-CONTEXT.md itself never pins a precedence POSITION, only `Classify`'s non-failed-attempt bucket (D-04). If a plugin-install failure and a preserved registration occur on the same row, the chosen position decides which outcome string the row's aggregated `Outcome` field shows. |
| A3 | Codex's JSON `http_headers` (a map) and `env_http_headers` (also a map, by naming convention) are the two fields a custom-header registration populates — inferred from field naming, not yet observed with either populated | Code Examples (Codex JSON shape) | Medium — if the real shape differs (e.g. a single unified `headers` field, or a list-of-objects rather than a map), the Codex scan's struct definition needs correction; D-11's `DisallowUnknownFields` safety net means a wrong guess here fails LOUD (decode error → preserved) rather than silently misparsing, which bounds the blast radius |

**None of these are compliance, retention, or security-standard claims** — all are third-party CLI
output-shape observations pending the phase's own scheduled human checkpoint (D-05).

## Open Questions

1. **Where does `OutcomePreserved` sit in `aggregate.go`'s `precedenceOrder`?**
   - What we know: D-04 places it in `Classify`'s `hasNonFailedAttempt` bucket (a two-way
     partition, order-independent). A DIFFERENT, earlier research document (`ARCHITECTURE.md`,
     which predates 04-CONTEXT.md's locked decisions and was not itself ratified by the user)
     recommended inserting it between `OutcomeAlreadyCorrect` and `OutcomeWouldWrite` in the FOLD
     precedence `AggregateOutcome` uses.
   - What's unclear: 04-CONTEXT.md never revisits or locks this specific ordering question; it is
     not listed under "Claude's Discretion" either, so it is a genuine gap between the two
     documents.
   - Recommendation: the planner should treat `ARCHITECTURE.md`'s suggested position (between
     already-correct and would-write — "a preserved hand-edit means 'correctly wrote nothing,'
     closer in spirit to already-correct than to a bare preview") as a reasonable default, but
     confirm explicitly with a checkpoint or in-plan note, since it decides the visible aggregated
     `Outcome` for a row where a `preserved` registration coincides with a `would-write` plugin
     facet or vice versa.

2. **Does `--output json`'s new facet field carry the typed `Facet` set as a joined string, or as
   several boolean-shaped scalar fields (one per enum value)?**
   - What we know: D-12 requires the differing facet(s) reported "in a fixed stable order,
     rendered from the typed set in both text and JSON," and `TestOperatorViewFixturesHaveNoUnsanitizedNesting`
     forbids anything but a flat scalar (no slice, no nested object) on `setupRuntimeRow`.
   - What's unclear: whether this phase reuses the EXISTING joined-string idiom
     (`SkillsDigest`/`Headers`' "comma-joined, sorted" pattern already on `setupRuntimeRow`) or
     introduces something new.
   - Recommendation: reuse the joined-string idiom — it is already proven compatible with the
     sanitization gate and needs no new renderer code, matching this phase's stated preference for
     additive, not novel, mechanism.

3. **Is the "expected" side of the comparison built from `Options` directly, or by re-parsing
   `Plan.Display()`/`Action.Args`?**
   - What we know: the AUTHORED-HERE invariant (`plan.go`) already forbids re-deriving argv
     anywhere outside a runtime's own `Plan()`. `Options.Headers` (with `sortedHeaders` for
     ordering) is the canonical, already-validated source `Plan()` itself was built from.
   - What's unclear: 04-CONTEXT.md's D-02 example ("would write `${LITELLM_KEY}`") implies the
     planned side is RENDERED (in a runtime's own `${VAR}`/`{env:VAR}` syntax), which argues for
     reusing each runtime's existing header-arg-rendering helper (`claudeCodeHeaderArgs`/
     `openCodeHeaderArgs`) rather than comparing raw `Options` values structurally.
   - Recommendation: compare against `Options` fields structurally (URL string equality, auth-mode
     equality, header NAME-set equality) for the CLASSIFICATION decision, but render the "would
     write" side of a facet message through the SAME per-runtime rendering helper `Plan()` already
     calls, so the rendered planned-value text can never drift from what `--apply` would actually
     write.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `claude` CLI | D-05's manual observation protocol (human checkpoint, not automated tests) | ✓ (on the researching machine, per `STACK.md`) | 2.1.270 (research session); re-verify at execution time | The checkpoint task documents the exact commands; if unavailable on the executing machine, the checkpoint is deferred to whichever machine the maintainer runs it on — it is explicitly a human, not automated, step |
| `codex-cli` | Same | ✓ (on the researching machine) | 0.154.0 (research session); re-verify at execution time | Same |
| `opencode` CLI | Not required this phase — opencode authors no scanner (D-10) | N/A | N/A | N/A |
| Automated tests (all REQ-drift-* fixtures) | `internal/setup`, `cmd/engram` test suites | ✓ | Go stdlib `testing`, already in use | None needed — every fixture test uses the existing `scriptedRun`/`fakeEnv` harness and NEVER invokes a real third-party CLI (rule `m45p2b4bp7`) |

**Missing dependencies with no fallback:** none — the one live-CLI dependency (`claude`/`codex`
binaries) gates only the human D-05 checkpoint, never the automated build or test suite.

**Missing dependencies with fallback:** none beyond the above.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no third-party test framework anywhere in this repo) |
| Config file | none — `go test` is invoked directly, per `Taskfile.yaml`'s `test:go` task |
| Quick run command | `go test ./internal/setup/... ./cmd/engram/... -count=1` |
| Full suite command | `task test` (lint + `go test ./...`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-drift-observed-registration | Claude Code scan extracts URL/auth-mode/header-names from a scripted D-11-shaped text block; Codex scan decodes the JSON shape via `DisallowUnknownFields` | unit | `go test ./internal/setup/ -run TestScanClaudeCodeRegistration -v -count=1` / `-run TestScanCodexRegistration` | ❌ Wave 0 — new test file, e.g. `internal/setup/drift_test.go` or per-runtime `claudecode_test.go`/`codex_test.go` extension |
| REQ-drift-three-way | All three outcomes reachable per parsed runtime (claude-code, codex); opencode NEVER reaches `already-correct`/`preserved` | unit | `go test ./internal/setup/ -run TestDriftThreeWayStates -v -count=1` | ❌ Wave 0 |
| REQ-drift-preserved-outcome | `Classify`/`AggregateOutcome`/`setupApplySummary` all correctly handle `OutcomePreserved`; docs gate on `guides/agent-setup.md`'s results table | unit + docs gate | `go test ./internal/setup/ -run TestClassify -v -count=1`; `go test ./internal/setup/ -run TestAggregateOutcome -v -count=1`; `go test ./cmd/engram/ -run TestAgentSetupGuideDocumentsPreserved -v -count=1` | ❌ Wave 0 — extends existing `exit_test.go`/`aggregate_test.go` exhaustive tables; new docs-gate test mirrors `cmd/engram/migrate_docs_test.go`'s zero-occurrence-plus-positive-control shape |
| REQ-drift-facet-naming | Each `Facet` enum value is independently reachable and rendered in a fixed order, both Go-value and `--output json` | unit | `go test ./internal/setup/ -run TestFacetDiffOrdering -v -count=1`; `go test ./cmd/engram/ -run TestSetupRuntimeRowFacetJSON -v -count=1` | ❌ Wave 0 |
| REQ-drift-redaction | A scripted probe stdout carrying a literal secret string never appears in `Result.Registered`, any Reason/Notes text, or marshaled `--output json` bytes | unit (negative-space) | `go test ./internal/setup/ -run TestRedactionUnconditional -v -count=1`; `go test ./cmd/engram/ -run TestSetupJSONNeverLeaksProbeLiteral -v -count=1` | ❌ Wave 0 — mirrors `internal/setup/plan_test.go:112` `TestNoSecretInArgs`'s shape but on the READ path instead of the write path |

### Sampling Rate
- **Per task commit:** `go test ./internal/setup/... ./cmd/engram/... -count=1`
- **Per wave merge:** `task test` (full lint + test suite, including `TestSetupPackageIsStdlibOnlyLeaf`)
- **Phase gate:** Full suite green before `/gsd-verify-work`; additionally confirm
  `04-OBSERVATIONS.md` exists, is dated, and every new scanner fixture cites it (D-08) — this is a
  documentation/provenance check, not a `go test` assertion, and belongs in the phase's own
  verification checklist.

### Wave 0 Gaps
- [ ] `internal/setup/drift_test.go` (or equivalent) — the shared `Facet`/comparison pure-function
  tests (REQ-drift-three-way, REQ-drift-facet-naming)
- [ ] `internal/setup/claudecode_test.go` extension — Claude Code scan fixtures, gated on
  `04-OBSERVATIONS.md` for the literal-echo case specifically (D-06)
- [ ] `internal/setup/codex_test.go` extension — Codex scan fixtures, same gate
- [ ] `internal/setup/plan_test.go`-adjacent — `TestRedactionUnconditional` (REQ-drift-redaction)
- [ ] `cmd/engram/setup_test.go`/`operator_view_setup_test.go` extension — facet rendering,
  `setupApplySummary` preserved-count, `--output json` leak test
- [ ] `cmd/engram/migrate_docs_test.go`-style docs gate for `guides/agent-setup.md`'s new row
- [ ] `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` — NOT a Go test; a human
  checkpoint deliverable (D-05/D-08), tracked as its own Wave 0 gap because scanner fixtures depend
  on it existing before they can be written honestly

*(RED-evidence approach: since `OutcomePreserved` and the drift-comparison functions do not exist
yet, the FIRST test written against them fails to COMPILE — a valid, strong RED signal in Go,
stronger than a runtime assertion failure, since it proves the test references real, not-yet-
authored symbols rather than a stub. Each subsequent test in the same file should be run
individually with `-run <name> -v` to confirm its own specific RED before the corresponding GREEN
commit, per this repo's own `bsbsvn4hbc` gotcha about `-run` patterns matching nothing and
false-greening.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | This phase reads registration METADATA (URL/auth-mode/header names), never authenticates anything itself |
| V3 Session Management | no | N/A — local CLI tool, no sessions |
| V4 Access Control | no | N/A — no new access-control surface |
| V5 Input Validation | yes | Third-party CLI probe output is UNTRUSTED DATA, never parsed as structure/commands — D-11's total-parse treats an unrecognized field as a safe fallback (`preserved`), never a crash or silent misparse; `encoding/json`'s `DisallowUnknownFields` is the stdlib control, never a hand-rolled parser |
| V6 Cryptography | no | No cryptographic operation in this phase — redaction (D-02/D-03) is data-handling discipline, not cryptography |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| A literal secret value, echoed by a runtime's own read verb for a registration engram did not write, reaches `Result.Registered`, `--output json`, a log line, or (via `internal/setupgen`) generated/committed markdown | Information Disclosure | Redaction BY CONSTRUCTION (D-02/D-03): the raw value is never assigned to any struct field that survives past the parse/compare step; proven by `TestRedactionUnconditional`'s negative-space fixture (REQ-drift-redaction) |
| A CLI-surface release (claude/codex) adds a new field to its probe output, silently changing what "the whole registration matches" means | Tampering (of the comparison's own correctness, not of engram's data) | D-11's total parse: an unrecognized field becomes `preserved` (cautious), never silently ignored (which would risk a false `already-correct`) — this is a structural, not a version-pinned, mitigation |
| A `preserved` row's reason text, or the facet-diff field, accidentally carries a raw probe fragment (e.g. via a naive `fmt.Sprintf("%v", observed)` debug-style composition) | Information Disclosure | Typed-cause-never-message-text discipline already established for `Reason` (`describeFailure`/`describeSeamError`) extends to the facet diff — compose from the typed `Facet` enum and REDACTED fields only, never from a struct's default `%v`/`%+v` formatting |
| Claude Code's probe dials the registered URL as a side effect of a read-only PREVIEW | Information Disclosure (of the operator's reachability to the configured endpoint) | Already accepted (T-03-10, `03-SECURITY.md`) prior to this phase; unchanged — this phase does not add a new dial, it only parses the existing probe's output |

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/setup/plan.go`, `apply.go`, `exit.go`, `runtime.go`, `aggregate.go`, `plugin.go`,
  `claudecode.go`, `codex.go`, `opencode.go`, `generic.go`, `quote.go`, `environment.go`,
  `leafpurity_test.go` — full read, this session, exact line ranges cited inline above
- `internal/setup/apply_test.go`, `exit_test.go`, `detect_test.go`, `plan_test.go` (fixture
  harness — `fakeEnv`/`fakeEnvWithRun`/`scriptedRun`/`scriptedResult`) — read this session
- `cmd/engram/setup.go` (row composition, `setupRuntimeRowFromResult`, `setupApplySummary`,
  `setupBuildRows`, `setupResolve`) — read this session
- `cmd/engram/operator_view.go`, `operator_output_test.go` (`sanitizeViewValue`,
  `TestOperatorViewFixturesHaveNoUnsanitizedNesting`) — grepped and read this session
- `docs-site/src/content/docs/guides/agent-setup.md` §"Read results and repeat safely" — read this
  session, the exact five-row table this phase extends
- `.planning/phases/04-drift-detection-read-only/04-CONTEXT.md` — read this session (the locked
  decisions D-01 through D-12 this document restates)
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md`, `.planning/ROADMAP.md` §"Phase 4" — read this
  session

### Secondary (MEDIUM confidence — prior-session live-verified findings, read this session)
- `.planning/research/STACK.md` — the label-set claim for `claude mcp get`'s text shape, the
  Codex JSON field list, and the explicit "values redacted before leaving this session" gap
- `.planning/research/ARCHITECTURE.md` §4 "Drift detection + reconcile hand-edits" — the
  `ObservedRegistration`/`compareRegistration` design sketch and its `OutcomePreserved`
  precedence-position recommendation (NOTE: predates 04-CONTEXT.md's locked decisions; not itself
  a locked decision — see Open Questions #1)
- `.planning/research/PITFALLS.md` — Pitfalls 4, 7, 10 (secret-leak-vector generalization risk,
  opencode's coarse-comparison rationale, Codex's whole-entry-no-partial-merge limitation)
- `.planning/milestones/2026-08-23.01-phases/03-runtime-registration/03-RESEARCH.md` — the two
  verbatim code-example blocks (Claude Code Headers block, Codex JSON shape) cited above

### Tertiary (LOW confidence — flagged, not yet independently re-verified this session)
- The exact shape Codex's JSON populates for a CUSTOM (non-bearer) header (`http_headers`/
  `env_http_headers`) — inferred from field naming only; ASSUMED (A3)
- Whether the literal-value echo shape structurally resembles the bare-reference shape already
  observed — ASSUMED (A1), explicitly gated behind D-05's human checkpoint

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, mechanically enforced by an existing test
- Architecture: HIGH — every extension point (`Outcome`, `Classify`, `AggregateOutcome`,
  `setupRuntimeRow`) is an already-shipped pattern (the Phase 3 plugin-facet precedent) applied
  to a new capability, not a novel mechanism
- Pitfalls: MEDIUM-HIGH — five pitfalls identified, four from direct code/doc-comment reading this
  session, one (Pitfall 2) is the phase's own named blocking prerequisite
- Redaction correctness (the literal-echo shape specifically): LOW until D-05's `04-OBSERVATIONS.md`
  lands — by design, this is the phase's own stated soft gate (D-06), not a research failure

**Research date:** 2026-09-15
**Valid until:** the CLI surfaces this phase depends on (`claude mcp get`, `codex mcp get --json`)
are actively released tools; re-verify version numbers and the D-05 literal-echo shape at
implementation time if more than ~30 days have elapsed, per this repo's own established discipline
for third-party CLI-surface research (`03-RESEARCH.md`'s own re-verification pattern).
