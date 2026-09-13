# Phase 2: Setup Command Core - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-29
**Phase:** 2-Setup Command Core
**Areas discussed:** Flag surface & auth vocabulary, Partial-failure exit codes, Runtime selection & detection reporting, Preview shape & blast-radius classification

---

## Flag surface & auth vocabulary

### How does a caller supply the engram server URL?

| Option | Description | Selected |
|--------|-------------|----------|
| `--url` flag | Short, unambiguous, zero collision with `cmdwalk.go:118`'s `server` predicate. Third spelling of "where engram lives" in this binary. | ✓ |
| `--mcp-url` flag | Names precisely what it is; makes the `/mcp` requirement legible in `--help`. Longer. | |
| Positional `engram setup <url>` | Mirrors `/engram-setup`'s `argument-hint`, near-literal pass-through for Phase 5. No operator command takes a positional URL today. | |
| `--url` AND optional positional | Flag canonical, positional as delegation sugar. Two ways to say one thing plus a precedence rule. | |

**User's choice:** `--url` flag

### What does that URL mean?

| Option | Description | Selected |
|--------|-------------|----------|
| Verbatim MCP endpoint | Written as-is. Correct for gateway routes and `ENGRAM_MCP_PATH=/` deployments that no suffix rule could derive. A bare base URL silently 404s. | ✓ |
| Base URL, setup appends `/mcp` | Friendlier common case; structurally wrong for the two documented non-default deployments. | |
| Verbatim + warning on suffix-less URL | Heuristic that can cry wolf on a legitimate root-mounted deployment. | |

**User's choice:** Verbatim MCP endpoint

### How is the auth mode selected?

| Option | Description | Selected |
|--------|-------------|----------|
| One `--auth` enum flag | `oauth\|oauth-client\|bearer\|none`, validated like `--output`. Matches the four rows already in the shipped prose table. | ✓ |
| `--auth` enum + inferred default | Probes the URL for a 401. Network call inside a preview; unreachable server turns preview into failure. | |
| Separate per-mode flags | Needs `MarkFlagsMutuallyExclusive`, whose plain `fmt.Errorf` bypasses `cliError`/`ExitCode()` — the exact defect v0.13.x unified #453 with #467 to close. | |

**User's choice:** One `--auth` enum flag

### Do setup's flags get `internal/config/registry.go` rows?

| Option | Description | Selected |
|--------|-------------|----------|
| URL reuses `ENGRAM_SERVER_URL`; rest flag-only | Originally offered as recommended, on the grounds that `--auth`/`--runtime` are per-invocation intent (citing `client.output`'s Env-less row). | |
| Full registry enrollment | `--url`, `--auth`, `--runtime` all get `ENGRAM_` rows. Maximally scriptable. | ✓ |
| Flag-only, no registry rows | Self-describing call sites; loses the `(default: ENGRAM_SERVER_URL)` help suffix. | |

**User's choice:** Full registry enrollment — *"2 is idiomatic here, and correct for koanf - correct? why make things break the principle of least surprise?"*

**Notes:** The user's challenge was correct and the recommendation was withdrawn. Measured rather
than assumed: 45 of 48 registry entries carry an `Env` row, so env-first-with-flag-override is the
pattern and omission is the exception. Each of the three exceptions (`client.token_file`,
`client.output`, `client.insecure`) carries a written justification for why that specific key is
not deployment state — none of which extends to `--url` or `--auth`. The original recommendation
borrowed `client.output`'s rationale and over-generalized it.

Follow-on boundary added and confirmed with the user: **`--apply` gets no registry row**, on the
`client.insecure` precedent — an exported env var that silently converts a preview into a mutation
is the same class of harm. Verified no existing mutating operator command has an `apply` row.

---

## Partial-failure exit codes

### How does `setup --apply` signal partial success?

| Option | Description | Selected |
|--------|-------------|----------|
| New `exitPartial = 8` | Dedicated code with a `nonConnectProducedCodes` entry — the shape `exitFindings` already uses. Requires a five-file one-commit change. | ✓ |
| Reuse `exitFindings = 7` | No new code, but 7's published meaning is bound to an explicit opt-in flag another command owns. | |
| Exit 0, partial-ness only in the report | Contradicts REQ-setup-partial-failure-legible outright. | |

**User's choice:** New `exitPartial = 8`

### What does total failure exit with?

| Option | Description | Selected |
|--------|-------------|----------|
| New `exitSetupFailed = 9` | Three-way becomes structurally unambiguous: 0 / 8 / 9. Two new codes in one phase. | ✓ |
| Reuse existing typed codes per-cause | Fits the typed-cause idiom, but "everything failed" is then not a single testable code and mixed causes need a tiebreak. | |
| `exitPartial = 8` + `exitGeneric = 1` | Reverses D-02's published redefinition of 1 as an unreachable-by-design backstop. | |

**User's choice:** New `exitSetupFailed = 9`

### Is an absent runtime a failure?

| Option | Description | Selected |
|--------|-------------|----------|
| Expected outcome, never a failure | `not-present` becomes a first-class `Outcome` value; a Claude-Code-only machine exits 0. | ✓ |
| Failure only when explicitly named | Same machine state yields different codes depending on phrasing. | |
| Always a failure | Makes exit 0 nearly unreachable on a real machine, draining the three-way signal. | |

**User's choice:** Expected outcome, never a failure

### Can a preview run exit nonzero?

| Option | Description | Selected |
|--------|-------------|----------|
| Only for usage/config errors | Exit 2 for bad `--auth`/`--url`, 0 otherwise. Safe to run unconditionally in a script's inspection step. | ✓ |
| Mirrors what `--apply` would return | Breaks the "read-only commands exit 0" expectation `migrate` and `prune-expired` both set. | |
| Nonzero on any blocking condition | "Blocking" becomes a judgment call, and flag-surface drift is only detectable in Phase 3. | |

**User's choice:** Only for usage/config errors

---

## Runtime selection & detection reporting

### Where does Phase 2 stop and Phase 3 begin?

| Option | Description | Selected |
|--------|-------------|----------|
| `Detect` + `Plan` here, `Apply` in Phase 3 | Satisfies success criterion 1 literally; the invocation strings are authored here and merely executed in Phase 3. | ✓ |
| Interface + types only, no real runtimes | The research's own build-order item 2. Leaves criteria 1–4 unsatisfiable and the roadmap's goal statement untrue at phase close. | |
| `Detect` + `Plan` for Claude Code only | Validates the abstraction against the single runtime we already know best — the weakest test of an interface whose job is to not assume a write mechanism. | |

**User's choice:** `Detect` + `Plan` here, `Apply` in Phase 3

**Notes:** This deliberately overrides `.planning/research/ARCHITECTURE.md:191`'s build-order item 2.
Recorded in CONTEXT.md D-09 with an explicit instruction that Phase 3 must not re-derive the
invocation strings.

### What does a bare `engram setup` target?

| Option | Description | Selected |
|--------|-------------|----------|
| Every detected runtime | Matches the milestone's own pitch; preview-by-default carries the safety weight. | ✓ |
| Every supported runtime, present or not | Stable diffable output shape, but "target" and "report on" become two concepts for `--help` to teach. | |
| Nothing; `--runtime` required | Contradicts REQ-setup-detects-runtimes and is more typing than the prose path it replaces. | |

**User's choice:** Every detected runtime

### `--runtime` unknown/absent handling?

| Option | Description | Selected |
|--------|-------------|----------|
| Unknown = exit 2; absent = reported row | Two genuinely different kinds of wrong: one about the invocation, one about the machine. | ✓ |
| Unknown = exit 2; absent = exit 9 | Reverses the area-2 decision for one invocation shape. | |
| Both are exit 2 | Conflates a typo with a fact about the machine; the remedies share nothing. | |

**User's choice:** Unknown = exit 2; absent = reported row

**Notes:** `StringSliceVar` was settled by precedent rather than asked — it is the binary's
established multi-value idiom (`--tags`, `--categories`, `--id`, `--class`) and accepts both
repeated and comma-separated forms for free.

### What is the detection signal?

| Option | Description | Selected |
|--------|-------------|----------|
| Binary on PATH only | Satisfies the stale-config-dir requirement by construction. Documented false negative for non-PATH installs — the safe direction. | ✓ |
| PATH primary, config dir as corroboration | Adds an `orphaned-config` state and per-runtime path knowledge acquired for a diagnostic. | |
| PATH + version capture | Groundwork for Phase 3 drift detection, but makes a read-only preview shell out to three third-party binaries and needs a timeout policy. | |

**User's choice:** Binary on PATH only

---

## Preview shape & blast-radius classification

### What `Class` does setup's `toolclass.go` row carry?

| Option | Description | Selected |
|--------|-------------|----------|
| `ReadOnly:false, Destructive:true, Idempotent:true` | Follows the table's stated conservative rule — re-running overwrites an existing `engram` entry. | ✓ |
| `ReadOnly:false, Destructive:false, Idempotent:true` | Argues setup only ever writes its own entry; strains the "some valid invocation" wording. | |
| `ReadOnly:false, Destructive:true, Idempotent:false` | Contradicts REQ-setup-idempotent, which this phase owns. | |

**User's choice:** `ReadOnly:false, Destructive:true, Idempotent:true`

### `registerDestructive` or a bespoke preview/apply path?

| Option | Description | Selected |
|--------|-------------|----------|
| Through `registerDestructive` | `!ReadOnly` admission gate admits it; `--apply`'s usage string composed from the registry rule, satisfying the conformance gate by construction. | ✓ |
| Own preview/apply path | Avoids bending a store-shaped abstraction, at the cost of a hand-copied usage string — the vacuous-drift shape this repo has scar tissue about. | |

**User's choice:** Through `registerDestructive`

### How is the per-runtime report rendered?

| Option | Description | Selected |
|--------|-------------|----------|
| Through `renderOperator`, runtimes as an array | Preserves one-serialization-plus-a-view; `viewRow` renders dense `key=value` lines. The exact command reads densely inside a row. | ✓ |
| `renderOperator` summary + verbatim command block | Better readability at the cost of a second Phase-1-D-07-style divergence needing its own pinning test. | |
| Bespoke renderer | Two independent encodings with nothing forcing them to agree. | |

**User's choice:** Through `renderOperator`, runtimes as an array

**Notes:** The accepted consequence — the exact invocation renders as a dense field value rather
than a copy-pasteable block — was surfaced to the user before the area closed and recorded in
CONTEXT.md D-15 so a reviewer does not file it as a defect.

### How does preview show the exact command under `--auth bearer`?

| Option | Description | Selected |
|--------|-------------|----------|
| `--token-file`, preview shows the path | Mirrors `client.token_file` exactly. Renders `Bearer <from /path/to/token>` — exact in every part that is not the secret. | ✓ |
| `--token-file`, preview redacts to `Bearer ***` | Loses provenance; a wrong-file mistake stays invisible until after `--apply`. | |
| `ENGRAM_TOKEN` env only, no flag | The one auth mode with no visible knob; weakens REQ-setup-correct-by-reading. | |

**User's choice:** `--token-file`, preview shows the path

---

## Claude's Discretion

- The precise Go shapes of `Plan`, `Action`, `Outcome`, `Result`, and `Environment` — field names,
  enum representation, file placement within `internal/setup` — subject to the two constraints
  named in CONTEXT.md D-07 and D-09.
- Whether the two new exit-code consts live in `client_common.go`'s block or a `setup`-owned file.
- `--help` prose wording, subject to success criterion 5.
- `StringSliceVar` for `--runtime` was decided by precedent rather than asked.

## Deferred Ideas

- Warning on a suffix-less `--url` (rejected under D-02; revisit with evidence after Phase 6).
- `<runtime> --version` capture during detection (rejected under D-12; genuine groundwork for
  Phase 3's REQ-register-cli-surface-drift-legible).
- An `orphaned-config` detection state (rejected under D-12).
- A separate copy-pasteable verbatim command block in the text lane (rejected under D-15).
- Positional or config-file input for `--url`/`--auth`/`--runtime` (not needed for
  REQ-setup-non-interactive).

**Scope creep redirected:** none — discussion stayed within the phase boundary throughout.
