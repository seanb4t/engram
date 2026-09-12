# Phase 3: Runtime Registration - Context

**Gathered:** 2026-09-08
**Status:** Ready for planning

<domain>
## Phase Boundary

`engram setup --apply` actually registers the engram MCP server with Claude Code, Codex, and
opencode by executing each runtime's own CLI, and emits a portable configuration for any MCP
client engram does not natively support — across every auth mode engram deploys behind. No
runtime's config file is ever read, parsed, or written; the only thing engram touches is each
runtime's own `mcp add`/`mcp get` surface.

This phase is where the `engram → third-party runtime CLI` trust boundary — recorded but
deliberately **not crossed** in Phase 2 (`02-SECURITY.md`) — is crossed for the first time.

**In scope:** `Apply()` as a shared package-level executor over a `Plan`'s argv; the
`Environment.Run` seam; the `Args`/`Command` split and its quoting; the read → write → read
convergence probe and `Plan.Probe`; the bearer env-var-reference form for claude-code and
opencode; post-hoc CLI-drift classification; the child-process timeout/stdio policy; the
`generic` opt-in pseudo-runtime and its portable JSON config; the report fields those add
(`binary`, `registered`, `token_file`, `config`); re-dispositioning T-02-04/R-02-02 and
T-02-05/R-02-01 (issue #523).

**Out of scope:** skills distribution and any growth of `Plan.Actions` beyond registration
(Phase 4); the `/engram-setup` delegation gate and `internal/setupgen` (Phase 5); install
documentation (Phase 6); Cursor (deferred to v2 at scoping, `REQUIREMENTS.md` line 67); any
change to the five-value `Outcome` vocabulary or to `Classify` (both pinned in Phase 2).

**Moved here from Phase 2 by its own D-09:** success criterion 5 (a second `--apply` converges
and reports `already correct` distinctly from `wrote it`) and REQ-setup-idempotent. Phase 2
stubbed `Apply()`, so neither was reachable there.

</domain>

<decisions>
## Implementation Decisions

### Execution form — the argv/display split

- **D-01:** `Action` gains **`Args []string`, which is the authored source of truth**.
  `Command` becomes a **derived** rendering of `Args` for display only — never independently
  authored, so it cannot contradict what `Apply()` runs. `exec` receives an argument list; there
  is no shell to inject into and nothing is ever re-parsed.
  This is the execution half of the remedy `02-SECURITY.md`'s Notes for Phase 3 prescribes:
  *"use `exec.Command`'s argument-list form rather than shell-string interpolation… a stronger
  control than the `shellQuote` helper proposed in `02-REVIEW.md`."*
  It **honors** `plan.go:11-16`'s "consume these strings, never re-derive them" in substance:
  the invocation is still authored once, in `Plan()`, in the runtime's own file. What changes is
  its *type*, not its authorship. A later phase still consumes; it does not re-derive.
  Whether `Command` is a struct field recomputed at construction or a `Display()` method is the
  planner's, subject to the invariant that it is a pure function of `Args`.
  Churn: all seven `Command:` constructions in `internal/setup/{claudecode,codex,opencode}.go`,
  plus the pinned strings in `plan_test.go` and `cmd/engram/setup_test.go`.
  — **Reversibility:** costly — undoing it means re-authoring seven invocation sites and
  reopening T-02-05, which this phase closes.

- **D-02:** The derived display string uses **minimal POSIX quoting**: a word renders bare when
  every rune is in `[A-Za-z0-9_@%+=:,./-]`, and is otherwise single-quoted with `'\''` escaping.
  Ordinary URLs therefore render exactly as they do today, so the preview stays readable and
  pinned-string churn stays near-zero — while a `--url` or path carrying `; | $ ` &` or a space
  gets quoted.
  This is the **display half** of #523, and it is not optional: `02-SECURITY.md`'s R-02-01 states
  *"Both are needed, and they cover different paths — argv form for what `Apply()` runs, quoting
  for what the preview displays and a human may paste. Fixing only the execution path leaves the
  copy-paste vector open."* The named consumers of that text are the coding agents in
  `--runtime`, which act on it programmatically.
  Rejected: **quote-everything** (noisy preview, churns every golden, and Phase 6's docs-site
  examples inherit the noise). Rejected: **no quoting** (closes only the execution path, so #523
  could not be closed by this phase).

- **D-03:** `Apply()` is **one shared, package-level executor** over a `Plan`'s `Args`. The
  `Runtime` interface stays at `Name`/`Detect`/`Plan`; `Environment` gains a `Run`-style seam so
  tests drive a fake process boundary and never invoke a real third-party binary (rule
  `m45p2b4bp7`).
  All three runtimes are structurally identical shell-outs, so this needs zero per-runtime
  execution code and **structurally enforces** `runtime.go`'s existing constraint that no code
  path outside a runtime's own file may special-case it by name. Adding a fourth runtime is then
  writing `Plan()` and nothing else.
  Rejected: `Apply` **on the interface** (three near-identical implementations to keep in sync).
  Rejected: a shared executor **plus an optional per-runtime override** (two ways to answer one
  question; the escape hatch gets used before it is earned).
  `internal/setup` must remain a stdlib-only leaf — `leafpurity_test.go` enforces it; `os/exec`
  is stdlib, so the seam is admissible.

- **D-04:** `Apply()` execs the **`LookPath`-resolved absolute path**, closing the TOCTOU window
  between `Detect()` and `Apply()` — what was reported present is what runs.
  `Args[0]` nevertheless **stays the bare name** (`claude`, `codex`, `opencode`): `Plan()` has no
  `LookPath` result and should not acquire one, `Command` stays a pure function of `Args` per
  D-01, and the previewed string stays pasteable and machine-independent (which Phase 6's
  docs-site examples depend on). `Apply()` substitutes the resolved path at exec time and
  **records it on the `Result` as its own field**, rendered on the row.
  These are two different facts — what you would type, and what actually ran — and both are
  reported rather than conflated. Reporting the resolved binary is also what makes a
  `PATH`-spoofing incident leave a trace in the report (see Security below).

### Auth modes under `--apply`

- **D-05:** Under `--auth bearer`, engram writes a **variable reference, never a value**.
  `codex` already does this natively via `--bearer-token-env-var ENGRAM_TOKEN`; claude-code and
  opencode get the equivalent env-var reference embedded in their `--header` value, so **argv
  carries a variable NAME and never a secret**, and the runtime resolves the credential itself at
  connect time. All three runtimes then support bearer.
  This satisfies REQ-register-auth-modes' *"A secret is never placed on a command line where the
  shell or process table would capture it"* on the execution path, not merely the preview path.
  Rejected: **refusing the pair** for claude-code/opencode (compliant, but bearer is the likeliest
  self-hosted mode, and it only relocates the exposure into the operator's own shell history).
  Rejected: **reading `--token-file` and interpolating the credential** — it would put the secret
  in `ps`, reopen T-02-01 at severity `high`, and break `leafpurity_test.go`, which currently
  passes precisely because `internal/setup` performs zero file reads.
  **Research precondition:** the exact expansion syntax per runtime (`${VAR}` / `{env:VAR}` /
  something else) is **third-party and unverified**. If a runtime does not expand it, engram
  writes a registration that fails at first use rather than at registration time. The researcher
  must establish the real syntax for claude-code and opencode before planning; do not take the
  illustrative forms in this document as verified.
  — **Reversibility:** costly — the reference is written into someone else's config file, so
  changing the form later requires every operator to re-run `engram setup --apply`.

- **D-06:** `--token-file` **survives, scoped to the generic portable config only**. It has no
  execution meaning for a native runtime: the child process (`<cli> mcp add`) only *writes
  config* — it never uses the credential, which the runtime needs later at connect time from its
  own environment. A path engram never reads and never passes has nothing to do there.
  Consequence, stated so it is not rediscovered: Phase 2's D-16 provenance form
  (`Bearer <from /path/to/token>`) **no longer describes what `--apply` writes** for a native
  runtime, and the preview must render the reference form that will actually be written. Preview
  and apply must show the same string.

- **D-07:** When `--token-file` is supplied and a native runtime is targeted, **each affected
  row carries an explicit marker** (e.g. `token_file=ignored`) rather than being silently inert.
  This fits D-15's one-typed-document contract exactly — `viewRow` renders it as one more
  `key=value` with no bespoke output — and the operator learns per runtime precisely where the
  flag did and did not apply.
  A flag that is honored or inert depending on `--runtime`, with no signal either way, is the
  fails-by-absence shape memory `zcev96ng18` records elsewhere in this binary. The marker is
  what converts it into a legible one.

### Convergence and idempotence

- **D-08:** `Apply()` observes existing state by **read → write → read, byte-comparing the two
  reads**. Identical → `already-correct`; different → `wrote`.
  This depends only on the read verb being **deterministic**, never on its format — nothing is
  parsed, so a cosmetic change to a runtime's output cannot break engram, and a changed `--url`
  is detected by construction. It satisfies REQ-setup-idempotent and success criterion 5 without
  reading any config file and without acquiring a third-party *output-format* dependency (which
  the requirements never authorized and which memory `r7n0nejp9f` shows drifts on a scale of
  days).
  **Invariant: ambiguity resolves to `wrote`, never to `already-correct`.** A nondeterministic
  read (timestamps, unstable ordering) must degrade to over-reporting change, never to falsely
  claiming convergence — falsely reporting `already-correct` for an entry whose URL has changed
  would mean `--apply` silently stops updating, the vacuous-gate family memories `srbmss1c5z` and
  `8583e0yqa1` record.
  Accepted consequence: the write always runs (already sanctioned — D-13 of Phase 2 classifies
  `setup` `Destructive: true` precisely because `mcp add` overwrites an existing entry), and each
  runtime costs three shell-outs instead of one.
  Rejected: **parsing the read verb's output** (third-party format dependency; would let preview
  classify `already-correct`, which is the only thing this loses). Rejected: **presence-only via
  exit status** (a changed `--url` reports `already-correct` and is never applied). Rejected: an
  **engram-owned marker file** (converges with a record of reality rather than reality; lies the
  moment anyone edits the config by hand).
  — **Reversibility:** costly — `already-correct` versus `wrote` is the observable contract
  success criterion 5 and REQ-setup-idempotent are written against.

- **D-09:** The per-runtime read verb is authored as **`Plan.Probe []string`**, in the same
  `Plan()` call that authors the write argv. The `Runtime` interface stays at three methods, every
  runtime-specific string stays in that runtime's own file, D-01/D-09-of-Phase-2's
  authored-in-`Plan()` invariant extends to the read verb, and the shared executor sequences
  probe → actions → probe with no per-runtime knowledge.
  Rejected: **`Actions` gaining a `Mutates` flag** (each runtime would hand-author the same
  three-step sequence). Rejected: **a fourth `Probe()` interface method** (splits a runtime's argv
  authoring across two methods for nothing the field does not already give).

- **D-10:** **Preview also runs the probe** and reports currently-registered state as an
  informational row field. The classified `Outcome` **stays `would-write`** — byte-compare needs
  a write between the two reads, so preview cannot honestly classify `already-correct` — but the
  operator sees present state next to intended state and can tell that a `--url` is about to
  change before anything is written.
  This **reverses Phase 2's D-12 posture** that a preview shells out to nothing, and that reversal
  must be stated rather than absorbed:
  - `02-SECURITY.md`'s **T-02-08** ("a preview cannot produce a nonzero exit code") rested partly
    on `setupPreview` having exactly one terminal statement and starting no process. The exit-code
    property must be re-argued and re-pinned on its own terms; **D-08 of Phase 2 still holds** —
    a preview exits nonzero only for usage or config errors, and a probe that fails must not
    change that.
  - The timeout policy D-12 of Phase 2 declined to invent is now mandatory (see D-12 below), and
    it applies to **preview as well as apply**.
  — **Reversibility:** costly — once scripts read the informational field, removing it is a
  breaking change to the report's published shape.

### Failure legibility and process policy

- **D-11:** CLI-surface drift is detected and reported **post hoc only** — there is no
  pre-flight probe of a runtime's help output and no version-floor check. A nonzero exit from
  either the probe or the write becomes an `OutcomeFailed` row whose `Reason` names **the
  runtime, the exact argv engram issued, and the runtime's captured stderr verbatim**.
  This satisfies REQ-register-cli-surface-drift-legible as written: its named failure mode is
  *"rather than silently writing nothing"* — silence, not lack of prediction — and a report
  carrying what engram tried and what the runtime said back is neither silent nor vague. Engram
  **reports rather than diagnoses**: it does not itself distinguish "unknown flag" from "network
  error", and does not need to.
  Rejected: a **pre-flight `mcp add --help` scrape** — it asserts a third-party surface, which
  rule `m45p2b4bp7` forbids, and matching tokens in help text is scraping a format that drifts.
  Rejected: a **version floor** — memory `r7n0nejp9f` measured the flag surface *holding* across
  two codex minor bumps in six days, so a version gate false-fails on drift that does not matter
  and cannot catch a flag removed inside an allowed range. It gates the proxy, not the property.

- **D-12:** The child-process timeout is a **fixed internal constant, not a flag**. Each `exec`
  is bounded by a `context.WithTimeout` at package scope; no `--timeout` flag, no registry row,
  no golden churn.
  **This is a deliberate, reasoned divergence from a repo idiom, recorded so a reviewer does not
  file it as an oversight.** `setupCmd` is currently the only destructive command without a
  `--timeout` — `migrate`, `prune-expired`, `reindex`, `spine-review purge`,
  `backfill-short-ids` and `migrate-remap-owner` all carry a `DurationVar` publishing the same
  *"max wall-clock (…); also cancellable via Ctrl-C"* sentence
  (`destructive_test.go:419-433`). The idiomatic move was named during discussion and
  deliberately not taken: `setup` already carries six flags, and these are fast local CLIs rather
  than store sweeps, so the tunability the siblings need is not yet earned here.
  Promoting it to a flag later is purely additive — see Deferred Ideas.
  — **Reversibility:** reversible — adding the flag later is additive and breaks no caller.

- **D-13:** Every child process gets **stdout and stderr captured, and stdin explicitly closed**
  (`/dev/null`). Capture is already forced by D-08's byte-compare and is what supplies D-11's
  `Reason`. Closing stdin means a runtime that decides to prompt gets EOF and **fails in
  milliseconds with a legible error** instead of hanging until the timeout constant expires.
  This also keeps D-15's one-serialization-plus-a-view invariant intact: nothing a child prints
  ever reaches the terminal except through the report. Letting the write **inherit** stdio was
  rejected because interleaved child output corrupts the `--output json` lane outright and leaves
  `Reason` empty, which D-11 depends on. Capturing while **inheriting stdin** was rejected as
  strictly worse than either alternative — the prompt's text lands in the captured buffer, so the
  operator faces a silent, unexplained hang.

### The generic portable config

- **D-14:** `generic` is a **registered pseudo-runtime that is opt-in only**: it lives in
  `Runtimes` so `--runtime generic` works and it gets an ordinary row, but its `Detect()` reports
  not-present unless it was explicitly named — so a bare `engram setup` still reports only what is
  actually on the machine, and the summary's "N/M present" count is not inflated by a runtime that
  is not a thing on this machine.
  The one cost is a deliberate stretch of `Detect()`'s meaning, which must be documented in
  `generic`'s own file rather than special-cased anywhere else — `runtime.go`'s
  no-special-casing-by-name constraint applies to it exactly as to the other three.

- **D-15:** The portable configuration is **minified single-line JSON in an ordinary row field**.
  JSON is whitespace-insensitive, so a minified blob pastes into a client's config and behaves
  identically to a pretty one; `viewRow`'s dense `key=value` shape survives untouched; and
  `--output json` nests it properly as an object for machine consumers.
  This preserves D-15 of Phase 2 (one serialization plus a view) with **zero divergence** — which
  matters because that decision already recorded that a second divergence was "not worth taking
  twice". Rejected: a **verbatim multi-line block** appended to the text lane (readable, but the
  second divergence). Rejected: **writing it to a path** behind a new flag (a real filesystem
  write from a command whose preview contract is "changes nothing", so the config would be
  unobtainable from a preview at all).

- **D-16:** `generic` reports **`would-write` in both lanes**, and `Classify` (`exit.go`) is
  **untouched**. There is no state transition for `--apply` to make — the config *is* the
  deliverable — and `would-write` already counts as a non-failed attempt, so no pinned pure
  function or its exhaustive combination table
  (`exit_test.go`'s `TestClassifyExhaustiveOutcomeCombinations`) moves.
  Consequence, stated plainly: `--runtime generic,claude-code --apply` with a failing claude-code
  exits **8 (partial), not 9** — defensible, because the operator did receive the portable
  config. `generic`'s own file must say why its outcome never changes under `--apply`.
  Rejected: `already-correct` under `--apply` — `plan.go` documents that value as *"the existing
  registration already matches what `Plan` would produce"*, a claim about observed state that
  `generic` never observes; it would be the one place the vocabulary means something else.
  Rejected: making `generic` **contribute to neither class** — more precise about what the exit
  code means, but it reopens `Classify` and its pinned exhaustive table for a marginal gain.

### Security — threats this phase must re-open or add

Not decisions, but not the planner's to discover either. `/gsd-secure-phase` and the phase's own
threat register must address each:

- **T-02-05 / R-02-01 → issue #523.** Explicitly *"a precondition of Phase 3"*. D-01 (argv form)
  and D-02 (display quoting) are the two controls; **both** are required, and closing only one
  leaves the other path open.
- **T-02-04 / R-02-02.** `exec.LookPath` spoofing "stops being theoretical the moment Phase 3
  invokes the resolved binary". D-04 chose to exec the resolved path and to report it, so the
  disposition must be rewritten for a boundary engram now **crosses**, not merely observes.
- **NEW — third-party stdout becomes report content.** D-10 and D-11 render a runtime's own
  stdout/stderr into `registered=` and `reason=` fields. This is a new untrusted-input path into
  the operator's terminal and into `--output json`. `sanitizeViewValue` (`operator_view.go:223`)
  strips **only** `r < 0x20 || r == 0x7f` — C0 controls and DEL; every shell metacharacter is
  printable and passes through untouched (memory `wvpxqrd5m0`). Do not read T-02-13 as covering
  this: it is the same adjacent-but-orthogonal trap that register already documents. Decide
  deliberately whether captured output is truncated, bounded, or otherwise constrained before it
  is rendered.
- **NEW — the `engram → third-party runtime CLI` trust boundary is crossed.** `02-SECURITY.md`
  recorded it at row 31 as "NOT crossed this phase… so Phase 3 inherits the boundary rather than
  rediscovering it." It is now live.

### Claude's Discretion

- Whether `Command` is a recomputed struct field or a `Display()` method; where the quoter lives;
  the exact safe-set constant — subject to D-01's purity invariant and D-02's policy.
- The `Environment.Run` seam's exact signature (return shape for stdout/stderr/exit code), and
  whether `Apply` returns `(Result, error)` or accumulates — subject to D-03's shared-executor
  shape and to `internal/migrate`'s `errors.Join`-style accumulation precedent.
- The concrete timeout value in D-12, and whether the constant is per-`exec` or per-runtime.
- Field names for the new `Result`/row keys (`binary`, `registered`, `token_file`, `config`) and
  their `json` tags, subject to D-04, D-07, D-10 and D-15 each being reported.
- How `generic` names itself in `--help` and its `Detect()` documentation, subject to D-14.
- `--help` prose changes for `--token-file`'s narrowed meaning (D-06), subject to
  REQ-setup-correct-by-reading.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition and requirements
- `.planning/ROADMAP.md` § "Phase 3: Runtime Registration" — goal, 7 requirements, 5 success
  criteria. Criterion 5 arrived here from Phase 2 via its D-09.
- `.planning/REQUIREMENTS.md` lines 39–44 — REQ-register-claude-code, REQ-register-codex,
  REQ-register-opencode, REQ-register-generic-mcp, REQ-register-auth-modes,
  REQ-register-cli-surface-drift-legible.
- `.planning/REQUIREMENTS.md` line 32 — REQ-setup-idempotent, moved to this phase.
- `.planning/REQUIREMENTS.md` line 67 — REQ-register-cursor, deferred to v2 at scoping.

### Prior-phase context this phase builds directly on
- `.planning/phases/02-setup-command-core/02-CONTEXT.md` — **read in full.** D-02 (`--url`
  verbatim), D-03 (the four auth modes), D-05/D-16 (token-file path, provenance rendering —
  both narrowed by D-06 here), D-06 (the 8/9 exit taxonomy), D-07 (`not-present` is never a
  failure), D-09 (invocation strings authored in `Plan()`, Apply stubbed, criterion 2 moved
  here), D-12 (detection is `LookPath` only, and preview shells out to nothing — **reversed by
  D-10 here**), D-13 (`Destructive: true`), D-14 (`registerDestructive`), D-15 (one
  serialization plus a view).
- `.planning/phases/02-setup-command-core/02-SECURITY.md` — **read in full, especially
  § "Notes for Phase 3" (lines 75–83)**, the Accepted Risks Log R-02-01 and R-02-02, and the
  trust-boundary table row 31. This file names this phase's security work directly.
- `.planning/phases/02-setup-command-core/02-VERIFICATION.md`,
  `.planning/phases/02-setup-command-core/02-REVIEW.md` — what shipped and what the review
  found, including the `shellQuote` proposal `02-SECURITY.md` judges weaker than argv form.
- `.planning/phases/01-version-homebrew-distribution/01-CONTEXT.md` — D-11's ownership boundary
  ("we do not test or red-gate what we do not own").

### The code this phase changes
- `internal/setup/plan.go` — `Outcome` (five values, pinned), `Action`, `Plan`, `Result`,
  `bearerProvenance`, and the package doc comment committing Apply to consuming `Plan()`'s
  strings. D-01/D-04/D-09/D-15 all land here.
- `internal/setup/runtime.go` — the `Runtime` interface and its no-special-casing-by-name
  constraint, `Runtimes`, `Names`, `Select`, `ErrAuthModeUnsupported`, `ErrApplyNotImplemented`
  (retired this phase).
- `internal/setup/environment.go` — the injectable seam D-03 extends with `Run`.
- `internal/setup/{claudecode,codex,opencode}.go` — the seven `Command:` constructions D-01
  converts to `Args`, and the bearer forms D-05 rewrites. Note codex already carries the correct
  shape via `--bearer-token-env-var`.
- `internal/setup/exit.go` — `Classify` and `ExitClass`. **Untouched by D-16**; read it to
  confirm that, not to change it.
- `internal/setup/leafpurity_test.go` — the stdlib-only-leaf gate and the zero-file-reads
  property that keeps T-02-01 closed. D-03 and D-05 both must not break it.
- `cmd/engram/setup.go` — `setupBuildRows`, `setupPreview`, `setupApplyRun`,
  `setupApplyStubReason` (deleted this phase), `setupRuntimeRow`, `setupReportDoc`,
  `setupPreviewSummary`, `setupApplySummary`, `setupLongDescription`, and the `init()` flag
  block.
- `cmd/engram/destructive_test.go:433` — `setupCmd`'s expected flag set. D-12 deliberately
  leaves this row unchanged; every sibling row on lines 419–430 carries `"timeout"`.
- `cmd/engram/operator_view.go:161` (`viewRow`), `:223` (`sanitizeViewValue`) — the dense
  row rendering D-15 relies on and the sanitizer whose scope the new Security note bounds.
- `cmd/engram/testdata/catalog.golden`, `cmd/engram/testdata/help.golden` — pinned; move if
  help text changes.

### Milestone research (read critically)
- `.planning/research/SUMMARY.md` § "Post-Synthesis Live Verification" — the live-probed
  `mcp add` surfaces for all three runtimes, the pinned versions (codex-cli 0.148.0, opencode
  1.18.15 at that time), and consequence 5, which argues for failing legibly on an unexpected
  flag surface. **Re-probe before planning** — memory `r7n0nejp9f` measured the observed
  versions already drifted to codex 0.150.1 / opencode 1.18.20 within six days, with the flag
  surface holding.
- `.planning/research/ARCHITECTURE.md` line 183 — per-runtime independent Apply and
  `errors.Join`-style accumulation.
- `.planning/research/PITFALLS.md` — Pitfall 5 (a new CLI re-introducing the hand-editing
  problem the prose path solved), Pitfall 8 (per-runtime config paths, moot by D-12 of Phase 2).

### The prose path this phase must stay equivalent to
- `skill/engram/commands/engram-setup.md` lines 17–27 (URL determination and the `/mcp` mount),
  29–45 (the four auth modes and the `claude mcp add` invocation table), 46–52 (the
  never-put-the-secret-on-the-command-line notes D-05 now implements on the execution path).
- `docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md`.

### Codebase maps and repo rules
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/STRUCTURE.md`,
  `.planning/codebase/TESTING.md`.
- Repo rule `m45p2b4bp7` — never write a test or gate that asserts third-party behavior we do
  not own. D-11 and D-03 are both shaped by it.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `registerDestructive` / `addApplyFlag` / `applyRequested` (`cmd/engram/destructive.go`) — the
  preview/apply gate is already wired for `setup`; this phase fills in the apply closure rather
  than building a path.
- `renderOperator` / `renderOperatorView` / `viewRow` (`cmd/engram/operator_output.go`,
  `operator_view.go`) — every new field (D-04, D-07, D-10, D-15) is one more `key=value` with no
  new rendering code.
- `setup.Classify` + `setupExitCode` + `nonConnectProducedCodes` — the 0/8/9 taxonomy is built
  and pinned. This phase is the first to make `exitPartial = 8` **reachable in production**:
  `setupApplyRun`'s doc comment records that it currently has no live producer because every
  attempted runtime fails the stub. That gap closes here, and `catalog_test.go`'s allowlist
  comment should be revisited accordingly.
- `internal/migrate/registry.go`'s `Validate` — the `errors.Join`-style accumulation idiom for
  D-03's independent per-runtime outcomes.
- `cliNow` (`destructive.go:25`) and `citationFileReader` (`spine_review_verify.go`) — the
  package-var injectable-seam class `Environment.Run` joins.

### Established Patterns
- **Typed cause, never message text** — `classifyOperatorErr` (`cmd/engram/operror.go:69`) is a
  switch of `errors.Is`/`errors.As` arms with a true passthrough default. D-11's failure
  classification must follow that shape: stderr goes into `Reason` as *data*, and must never be
  string-matched to decide an outcome.
- **Structural predicates over enumerations, which fail by absence** — `operatorCommands()`
  (`cmdwalk.go:118`) and memory `zcev96ng18`. D-07 exists because a silently-inert flag is the
  same failure shape.
- **Golden-file pinning** — `testdata/catalog.golden` and `testdata/help.golden` move whenever
  help text does.
- **Preview must render before any error return** — T-02-06 and `setup.go:316`. Every new
  failure path in this phase inherits that: a nonzero exit must never erase the per-runtime
  record.

### Integration Points
- `internal/setup` gains its first `os/exec` execution path (only `exec.LookPath` today), under
  the `leafpurity_test.go` stdlib-only constraint.
- `cmd/engram/setup.go`'s `setupApplyRun` replaces the D-09 stub; `ErrApplyNotImplemented` and
  `setupApplyStubReason` are deleted, and `setupApplySummary`'s "lands in a later phase" wording
  becomes untrue and must change.
- `setupPreview` gains a process-execution path (D-10), which is a behavioral change to a
  function T-02-08 currently reasons about structurally.

</code_context>

<specifics>
## Specific Ideas

- **`codex` is the reference shape for D-05.** Its `--bearer-token-env-var ENGRAM_TOKEN` already
  does exactly what claude-code and opencode must be made to do: put a variable *name* on the
  command line and let the runtime resolve the credential itself. Author the other two to match
  it, not the other way round.
- **`Command` must never be authored, only derived.** If a code review can find a place where a
  display string is written by hand, D-01 has been implemented wrongly regardless of whether the
  output looks right.
- **Standing user principle, restated from Phase 1 D-11, Phase 2, and rule `m45p2b4bp7`:** do not
  test or red-gate third-party behavior. Verify engram's own argv authoring, quoting, sequencing,
  classification, report shape and exit codes. Do not assert that `claude mcp add` behaves a
  particular way — drive the `Environment.Run` seam with a fake.
- The illustrative env-var expansion forms in D-05 (`${ENGRAM_TOKEN}`, `{env:ENGRAM_TOKEN}`) are
  **placeholders pending research**, not verified syntax. Treat them as a shape, not a spec.

</specifics>

<deferred>
## Deferred Ideas

- **Promote D-12's timeout constant to a `--timeout` flag.** Purely additive, and it would close
  `setup`'s standing inconsistency with the six sibling destructive commands at
  `destructive_test.go:419-433`. Revisit on evidence — an operator or CI job actually hitting
  the constant — rather than by anticipation.
- **Let preview classify `already-correct` by parsing the read verb's output.** Rejected under
  D-08 because it buys a third-party output-format dependency; the only thing byte-compare
  cannot do is classify convergence *before* writing. Revisit only if Phase 6's install docs
  show operators need a true dry-run convergence check.
- **A verbatim, pretty multi-line block for the generic config in the text lane.** Rejected
  under D-15 as a second divergence from one-serialization-plus-a-view. Same disposition Phase 2
  gave the same idea; revisit only if the minified form proves unusable in Phase 6's
  documentation.
- **Cursor as a fourth runtime.** Deferred to v2 at scoping (REQ-register-cursor). The research
  established it needs a plain-JSON file writer with merge-never-replace semantics, unlike the
  three shell-out runtimes.
- **An `orphaned-config` detection state** (config dir present, binary absent). Carried forward
  unchanged from Phase 2's deferred list; D-12 of Phase 2 still makes it out of scope.
- **A pre-flight flag-surface probe or version floor.** Rejected under D-11 as gating the proxy
  rather than the property. If post-hoc stderr proves genuinely unreadable to operators in
  practice, revisit with real failure reports rather than by anticipation.

</deferred>

---

*Phase: 3-Runtime Registration*
*Context gathered: 2026-09-08*
