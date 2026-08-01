---
phase: 04-diagnosability
plan: 07
subsystem: docs
tags: [docs-site, mcp, connect-rpc, error-handling, skill, agent-guidance]

requires:
  - phase: 04-diagnosability
    provides: "04-01's argError envelope and ten-code HintCode vocabulary (internal/server/argerror.go); 04-02's authz.DecisionLog field allowlist and internal/store's decideBucket/decideRecord debug log lines; 04-05's connectError class-to-code wiring; 04-06's D-18 amendment (koanf-configurable ENGRAM_MEMORY_MAX_SUMMARY_BYTES) and the D-06a schema-required relocation"
provides:
  - "docs-site/src/content/docs/reference/errors.md — the new error-envelope reference: grammar, all ten hint codes checked off against argerror.go, the class-to-Connect-code table, the no-value-echo guarantee"
  - "curating-memory SKILL.md's '## Reading a rejection' section — the agent-facing retry-pattern guidance closing REQ-error-hint-envelope"
  - "guides/configure.md's '## Memory' section (ENGRAM_MEMORY_MAX_SUMMARY_BYTES) and its authorization-diagnostics operator note under Logging — closing REQ-authz-decision-diagnostics"
  - "guides/upgrade.md's v0.12.0 entry — the three wire-visible changes (message reformat, schema loosening, Connect code widening) with the 401-body scope fence and the exit-code-unchanged claim"
affects: []

actuals:
  tokens: 5020
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Code-vs-doc extraction gate re-pointed at the internal/config registry entry's Default value (D-18 amendment) instead of a compile-time constant that no longer exists in tools.go — same discipline, different extraction source."

key-files:
  created:
    - docs-site/src/content/docs/reference/errors.md
  modified:
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/configure.md
    - docs-site/src/content/docs/guides/upgrade.md
    - skill/engram/skills/curating-memory/SKILL.md
    - CLAUDE.md

key-decisions:
  - "D-18 amendment applied as instructed: the plan's frozen verify gate `rg -o 'maxMemorySummaryBytes\\s*=\\s*[0-9]+' internal/server/tools.go` no longer matches anything (maxMemorySummaryBytes is now a parser FUNCTION, not a constant assignment — confirmed empty match, expected). Re-pointed the code-vs-doc cross-check at internal/config/registry.go's `memory.max_summary_bytes` entry's `Default: \"512\"` value instead, extracted via `rg -o 'memory\\.max_summary_bytes.*Default: \"([0-9]+)\"' -r '$1'`, and verified that value (512) appears in both SKILL.md and CLAUDE.md."
  - "The memory-summary bound is documented as a full docs-site 'Memory' config section (not folded into 'Auto-summary', which is a different mechanism — server-generated vs caller-authored summaries) per D-18's 'a config key is a published operator contract' instruction."
  - "supersede_memory's summary bound is NOT given its own table row in tools.md — its docs already state it 'behaves exactly as in store_memory' for the field, so the bound note added to store_memory's row propagates by reference rather than by duplicated text."

patterns-established: []

requirements-completed: [REQ-authz-decision-diagnostics, REQ-error-hint-envelope]

coverage:
  - id: D1
    description: "The error-envelope reference page documents the grammar (single-field and relational examples), all ten hint codes checked off one by one against argerror.go, and the class-to-Connect-code table"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: static
        ref: "rg -c '^\\| `[a-z_]+` \\|' docs-site/src/content/docs/reference/errors.md == 10 (floor was >=10)"
        status: pass
      - kind: static
        ref: "rg -q 'field=' and rg -q 'hint=' docs-site/src/content/docs/reference/errors.md"
        status: pass
    human_judgment: false
  - id: D2
    description: "curating-memory SKILL.md tells an agent to branch on field+hint (not wording), gives concrete retry patterns for too_long/required/mutually_exclusive+ordering, points to the errors reference instead of duplicating it, and states the summary bound with the value cross-checked against the config registry's default"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: static
        ref: "rg -q 'hint' skill/engram/skills/curating-memory/SKILL.md; value cross-check (512) present in both SKILL.md and CLAUDE.md"
        status: pass
    human_judgment: false
  - id: D3
    description: "guides/configure.md documents the debug-level authz decision log's actual field names (allow, action, bucket, policy_ids, policy_error_count), what it does NOT carry (Cedar trace, error message text, owner/scope), and the per-request volume bound"
    requirement: "REQ-authz-decision-diagnostics"
    verification:
      - kind: static
        ref: "rg -q 'policy' docs-site/src/content/docs/guides/configure.md; field names transcribed from internal/store/store.go's decideBucket/decideRecord slog.DebugContext calls"
        status: pass
    human_judgment: false
  - id: D4
    description: "guides/upgrade.md's v0.12.0 entry names all three wire-visible changes, explicitly scopes the MCP 401 auth body OUT (pinned by TestMCP401BodyByteIdentical), and states the CLI exit-code table is unchanged, backed by the shipped TestExitCodeForConnectErrTable"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: static
        ref: "rg -q 'field=' and rg -q 'OutOfRange|FailedPrecondition' docs-site/src/content/docs/guides/upgrade.md"
        status: pass
      - kind: unit
        ref: "go test ./cmd/engram/... -run 'TestExitCodeForConnectErrTable$' -v -count=1 -- PASS"
        status: pass
    human_judgment: false

duration: ~10min
completed: 2026-08-01
status: complete
---

# Phase 4 Plan 07: Error-Envelope Reference, Agent/Operator Guidance, and the Upgrade Note Summary

**One new docs-site reference page (`errors.md`) makes the field+hint envelope, all ten hint codes, and the Connect class-to-code mapping discoverable; the `curating-memory` skill and `CLAUDE.md` teach an agent to branch on `field=`/`hint=` instead of message wording; `guides/configure.md` gains the `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` config entry and the authz decision-log operator note; and `guides/upgrade.md` names all three wire-visible breaking changes with an explicit "CLI exit codes did not change" reassurance — closing `REQ-error-hint-envelope` and `REQ-authz-decision-diagnostics`, the last two open requirements in the phase.**

## Performance

- **Duration:** ~10 min
- **Tasks:** 3
- **Files modified:** 6 (1 created, 5 modified)
- **Commits:** 3

## D-18 Amendment — Applied As Instructed

Two consequences, both handled:

1. **Docs coverage.** `guides/configure.md` gained a new `## Memory` section documenting
   `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` (default `512`, `0` disables) as an operator-facing config
   key, placed between `## Embedder` and `## Auto-summary` (matching `internal/config.Config`'s
   struct field order: Server, Qdrant, Embed, **Memory**, Summarize, ...). Cross-referenced from
   `curating-memory` SKILL.md and `CLAUDE.md`.
2. **The frozen `maxMemorySummaryBytes` gate.** The plan's own verify line
   (`rg -o 'maxMemorySummaryBytes\s*=\s*[0-9]+' internal/server/tools.go`) was run and confirmed
   to match **nothing** — `maxMemorySummaryBytes` is now a parser *function*
   (`func maxMemorySummaryBytes(cfg *config.Config) int`), not a `name = value` constant, exactly
   as the amendment predicted. Per the amendment's explicit instruction, the cross-check was
   re-pointed at `internal/config/registry.go`'s `memory.max_summary_bytes` registry entry instead:

   ```
   V=$(rg -o 'memory\.max_summary_bytes.*Default: "([0-9]+)"' internal/config/registry.go -r '$1')
   # V=512
   rg -q "$V" skill/engram/skills/curating-memory/SKILL.md   # OK
   rg -q "$V" CLAUDE.md                                       # OK
   ```

   Same discipline (compare code to doc by extraction, never by eye), different extraction
   source — no eyeballed number anywhere in this plan's docs.

## Accomplishments

### Task 1 — `docs-site/src/content/docs/reference/errors.md` (new)

- The envelope grammar `field=<name> hint=<code>: <human text>`, with one single-field example
  (`store_memory`'s oversized `summary`) and one relational example (`list_memory`'s
  `cursor_mode`/`offset` conflict).
- **All ten hint codes**, transcribed one by one from `internal/server/argerror.go`'s `HintCode`
  constants (not from any plan SUMMARY): `required`, `conditional_required`, `too_long`,
  `too_many`, `enum`, `format`, `prefix`, `ordering`, `mutually_exclusive`, `not_applicable` — a
  table row per code, each with a "what to do" remediation, not just a glossary entry.
- The class-to-Connect-code table (`classMalformed`→`CodeInvalidArgument`,
  `classOutOfRange`→`CodeOutOfRange`, `classPrecondition`→`CodeFailedPrecondition`), stating all
  three share the CLI's `exitUsage` (exit `2`).
- The D-12 no-value-echo guarantee as a caller-facing safety statement (safe to log verbatim), and
  the explicit carve-out that the go-sdk's own schema-level and 401 rejections are OUTSIDE this
  grammar.
- Cross-links to `reference/tools.md` and `guides/cli.md#exit-codes`.

### Task 2 — Agent and operator guidance

- **`curating-memory` SKILL.md** gained a `## Reading a rejection` section (placed after
  `## Cross-spine recall`, before `## Discipline`): branch on `field`/`hint`, not wording; three
  concrete retry patterns (`too_long` on `summary` → shorten, don't resend the whole record;
  `required` → the field was absent, not malformed; `mutually_exclusive`/`ordering` → two fields
  named, the pair is wrong together). Points to `/reference/errors/` rather than duplicating the
  table.
- **The existing `## Summaries` section** (not a new section — matching the plan's explicit
  instruction to add "one line") gained a sentence stating the 512-byte default bound and the
  `field=summary hint=too_long` rejection shape.
- **`reference/tools.md`** gained the summary-bound note on `store_memory`'s and
  `schedule_memory`'s `summary` rows (both identical text, updated via `replace_all`) and
  `update_memory`'s `summary` row; `supersede_memory` inherits the note by reference since its
  docs already state it "behaves exactly as in `store_memory`" for shared fields. Added one
  sentence pointing to `/reference/errors/` near the tool-summary table.
- **`guides/configure.md`** gained the `## Memory` section (above) and, under the existing
  `## Logging` section, the authz decision-diagnostics note: the exact emitted field names
  (`allow`, `action`, `bucket` (bucket-arm only), `policy_ids`, `policy_error_count`), what it does
  NOT carry (Cedar expression trace, policy error message text, owner/scope values), and the
  volume bound (at most two lines per bulk recall — one per bucket probed — and one per
  id-addressed op).
- **`CLAUDE.md`**'s Memory contract gained exactly one sentence (with subordinate clauses) naming
  the envelope grammar and the summary bound, citing `docs-site reference/errors.md`.

### Task 3 — Upgrade note + phase-close gates

- **`guides/upgrade.md`** gained a `## v0.12.0 — Field-attributed, hint-carrying argument
  rejections` entry covering all three wire-visible changes: (1) message-text reformat, with an
  explicit scope fence naming `TestMCP401BodyByteIdentical` and stating the 401 body is
  unchanged; (2) the loosened tool schema (required-ness moved into engram, same rejections, right
  field now — closing #360) plus the new summary-bound rejection; (3) the Connect code widening,
  with the "CLI needs no change" claim backed by `TestExitCodeForConnectErrTable`.
- **The exit-code table verdict:** confirmed unchanged, not edited. `TestExitCodeForConnectErrTable`
  (the shipped, unmodified table test) maps `CodeInvalidArgument`, `CodeFailedPrecondition`, and
  `CodeOutOfRange` all to `exitUsage` — read directly from `cmd/engram/client_common_test.go`
  before writing the note, and re-run green as a Task 3 gate. No edit to `guides/cli.md` was made.

## The Sentence On CLI Exit Codes (quoted verbatim, per the requested check)

From `guides/upgrade.md`'s section 3:

> **The `engram` CLI needs no change.** All three codes already shared the CLI's `exitUsage`
> exit code (`2`) before this release, and still do — verified against
> `exitCodeForConnectErr`'s own unmodified test table (`TestExitCodeForConnectErrTable`).

Verified accurate: `cmd/engram/client_common.go`'s `exitCodeForConnectErr` groups
`connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange` under one
`case` returning `exitUsage` — unchanged before and after this phase (04-01's `argError.ConnectCode()`
comment independently confirms the same trio). `TestExitCodeForConnectErrTable` (16 cases, one per
`connect.Code`) passed on the final tree.

## Task Commits

1. **Task 1: the error-envelope and hint-code reference** — `78e89b17` (docs)
2. **Task 2: rejection envelope for agents and decision log for operators** — `addd259e` (docs)
3. **Task 3: upgrade note for the reformatted messages, loosened schema, and widened codes** — `b6d46b95` (docs)

## Hint-Code Check-Off (D1, transcribed from `internal/server/argerror.go`)

| Constant | String value | In `errors.md`? |
|---|---|---|
| `HintRequired` | `required` | yes |
| `HintConditionalRequired` | `conditional_required` | yes |
| `HintTooLong` | `too_long` | yes |
| `HintTooMany` | `too_many` | yes |
| `HintEnum` | `enum` | yes |
| `HintFormat` | `format` | yes |
| `HintPrefix` | `prefix` | yes |
| `HintOrdering` | `ordering` | yes |
| `HintMutuallyExclusive` | `mutually_exclusive` | yes |
| `HintNotApplicable` | `not_applicable` | yes |

10/10 — exact match, no code in the file that isn't in the doc and no doc row without a
corresponding constant.

## Exit-Code-Table Verdict (Task 3)

**Confirmed unchanged; no edit made.** `guides/cli.md`'s exit-code table already states that
codes `2` covers "usage or validation error engram's own commands detect" without enumerating
individual Connect codes, so nothing in its prose needed to change. The underlying mapper
(`exitCodeForConnectErr`) was read directly and its own test (`TestExitCodeForConnectErrTable`)
re-run green — this is the evidence backing the upgrade note's "no CLI change needed" claim, not
an assumption.

## Phase-Close Gate Results (Task 3, run on the final tree)

| Gate | Result |
|---|---|
| `task lint:markdown` (rumdl, all Task-2/3 gates) | clean, 125 files |
| `task license:check` (all Task-1/2/3 gates) | 0 invalid, 241 valid, 877 ignored (docs-site correctly ignored) |
| `go test ./cmd/engram/... -run 'TestExitCodeForConnectErrTable$' -v` | `--- PASS: TestExitCodeForConnectErrTable` |
| `task` (lint + full repo test suite, Go + Python) | all green — 15 Go packages `ok`, 33 Python tests passed |
| `go vet ./...` | clean |
| `task chart:validate` | OK (checksum-pinned `engram.containerEnv` untouched — this plan added no Helm-surfaced env var; `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` was already wired in 04-06) |
| `task proto:lint` | clean |
| `task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/` | zero diff |
| `task ui:build && git diff --exit-code -- internal/webauth/static/` | zero diff |
| `git diff --exit-code -- go.mod go.sum` | zero diff |

Every phase-close gate is green. This plan touched no Go code, so `task` was expected to be a
zero-behavior-change confirmation rather than a discovery pass — it was.

## Requirements Marked Complete

`requirements.mark-complete REQ-authz-decision-diagnostics REQ-error-hint-envelope` was run after
confirming both are genuinely satisfied:

- **`REQ-authz-decision-diagnostics`**: the code (D-01 through D-04, plan 04-02) was already
  shipped; this plan closes the "operator can see why" half by documenting the exact log field
  names, the volume bound, and what the line does NOT carry — the missing piece per convention
  `yaj7dqz9qq`.
- **`REQ-error-hint-envelope`**: the envelope (D-05/D-09/D-11, plans 04-01/04-04/04-05/04-06) was
  already shipped and complete on the code side per 04-06-SUMMARY.md's own flagged note ("very
  likely a plan-frontmatter omission, not a deliberate scope exclusion"). This plan's own
  frontmatter declares `REQ-error-hint-envelope` explicitly, and its own objective states the
  requirement is "deliberately still Pending... and THIS PLAN is what completes it" — so marking
  it here is the authored plan's own explicit instruction, not an inference across plan
  boundaries.

`REQ-validation-error-attribution` and `REQ-embed-provider-error-body` were already `[x]` from
plan 04-06 — unchanged by this plan.

`.planning/REQUIREMENTS.md` diff: both checkbox rows and both traceability-table rows flipped from
`Pending` to `Complete`/`[x]`; `write_set_complete: true` from `requirements.mark-complete`'s own
report.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 — discretion, matches the plan's own explicit amendment] `## Summaries are bounded` section removed, folded into the existing `## Summaries` section**
- **Found during:** Task 2, first draft of the SKILL.md edit
- **Issue:** Initial draft added a new top-level `## Summaries are bounded` section. The plan's
  explicit instruction was "Add **one line** to the existing `## Summaries` section" — a new
  section is not one line added to an existing section.
- **Fix:** Removed the new section; added one sentence to the existing `## Summaries` section
  instead, stating the bound and the rejection shape.
- **Files modified:** `skill/engram/skills/curating-memory/SKILL.md` (self-corrected before
  commit — no separate commit).
- **Verification:** `rg -n '^## '` confirms `## Summaries` still appears exactly once; the plan's
  acceptance criterion (`SKILL.md`'s summary bound value equals the shipped default, proven by a
  gate) still passes.

---

**Total deviations:** 1 (self-corrected during drafting, before any commit — not a post-hoc fix)
**Impact on plan:** None. No architectural change; corrected to match the plan's literal
instruction before the file was ever staged.

## Issues Encountered

None. All three tasks' `<verify>` gates passed on first attempt after the one self-corrected
drafting mistake above (caught before commit, not after).

## Next Phase Readiness

- Phase 4 (Diagnosability) is now complete: all four requirements (`REQ-validation-error-attribution`,
  `REQ-error-hint-envelope`, `REQ-authz-decision-diagnostics`, `REQ-embed-provider-error-body`) are
  marked `[x]` in `.planning/REQUIREMENTS.md`.
- Every new contract this phase shipped — the envelope grammar, the ten-code hint vocabulary, the
  Connect class-to-code mapping, the koanf-configurable memory-summary bound, and the debug-level
  authz decision log — is now documented on the surface its consumer actually reads (agent skill,
  docs-site reference/guide pages, `CLAUDE.md`'s contract index).
- No blockers. `task` (lint + full suite, all languages), `go vet`, `license:check`,
  `chart:validate`, `proto:lint`, `proto:gen`/`ui:build` zero-drift, and `go.mod`/`go.sum` zero-diff
  are all green on the final tree — this was the last plan in the phase.

---
*Phase: 04-diagnosability*
*Completed: 2026-08-01*

## Self-Check: PASSED

All created/modified files confirmed present on disk
(`docs-site/src/content/docs/reference/errors.md`,
`docs-site/src/content/docs/reference/tools.md`,
`docs-site/src/content/docs/guides/configure.md`,
`docs-site/src/content/docs/guides/upgrade.md`,
`skill/engram/skills/curating-memory/SKILL.md`, `CLAUDE.md`, this SUMMARY.md).
All three task commits (`78e89b17`, `addd259e`, `b6d46b95`) confirmed present in
`git log --oneline --all`.
