---
phase: 02-custom-auth-headers
verified: 2026-09-14T00:00:00Z
status: passed
score: 12/12 must-haves verified
covered_files:
  - ".planning/phases/02-custom-auth-headers/02-01-PLAN.md"
  - ".planning/phases/02-custom-auth-headers/02-01-SUMMARY.md"
  - ".planning/phases/02-custom-auth-headers/02-02-PLAN.md"
  - ".planning/phases/02-custom-auth-headers/02-02-SUMMARY.md"
  - ".planning/phases/02-custom-auth-headers/02-03-PLAN.md"
  - ".planning/phases/02-custom-auth-headers/02-03-SUMMARY.md"
  - ".planning/phases/02-custom-auth-headers/02-04-PLAN.md"
  - ".planning/phases/02-custom-auth-headers/02-04-SUMMARY.md"
  - ".planning/phases/02-custom-auth-headers/02-CONTEXT.md"
  - ".planning/phases/02-custom-auth-headers/02-REVIEW-FIX.md"
  - ".planning/phases/02-custom-auth-headers/02-REVIEW.md"
  - ".planning/phases/02-custom-auth-headers/02-VALIDATION.md"
  - "cmd/engram/clienttest_test.go"
  - "cmd/engram/destructive_test.go"
  - "cmd/engram/golden_test.go"
  - "cmd/engram/operator_view_setup_test.go"
  - "cmd/engram/setup.go"
  - "cmd/engram/setup_delegation_test.go"
  - "cmd/engram/setup_test.go"
  - "cmd/engram/testdata/catalog.golden"
  - "cmd/engram/testdata/help.golden"
  - "docs-site/src/content/docs/guides/agent-setup.md"
  - "internal/setup/claudecode.go"
  - "internal/setup/claudecode_test.go"
  - "internal/setup/codex.go"
  - "internal/setup/codex_test.go"
  - "internal/setup/generic.go"
  - "internal/setup/generic_test.go"
  - "internal/setup/opencode.go"
  - "internal/setup/opencode_test.go"
  - "internal/setup/plan_test.go"
  - "internal/setup/runtime.go"
  - "internal/setupgen/setupgen.go"
  - "internal/setupgen/setupgen_test.go"
  - "skill/engram/commands/engram-setup.md"
covered_digest: "v1:sha256:8eaba5bb88d06332becd3605f3b55cdc33da9aec1b85a729c82bf374fef82e5a"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Confirm neither the hand-authored /engram-setup prose (skill/engram/commands/engram-setup.md) nor guides/agent-setup.md suggests, documents, or generates a Codex header workaround (a TOML http_headers edit, a config-override flag, etc.) inside engram's own writer or generated tables — only a plain pointer that Codex has its own configuration the operator edits themselves."
    expected: "Both files describe Codex's decline (`failed` row naming `--bearer-token-env-var`) and, at most, a one-line pointer that Codex documents its own per-server header configuration which the operator can use directly — never a snippet, command, or generated table cell that has engram itself writing or suggesting a specific Codex config edit."
    why_human: "This is a judgment-tier prohibition (02-04-PLAN.md must_haves.prohibitions, verification: judgment) — not a grep-provable structural check. My own read of both files found them compliant (see Prohibitions section below), but per the verification protocol a judgment-tier prohibition must never be silently absorbed into a passed verdict; it is surfaced here for explicit human sign-off rather than blocking the run."
---

# Phase 2: Custom Auth Headers Verification Report

**Phase Goal:** A user can name the auth header a registration uses (for example a gateway's own `x-gateway-api-key`) alongside its env-var-reference value, for every runtime that can express it — Claude Code, opencode, and `generic` — each rendered in that runtime's own CLI syntax and authored per-runtime file, never through a shared cross-runtime formatter. Codex declines a non-`Authorization` header name explicitly, the same way `oauth-client` is already declined for opencode, and never gains a hand-written `[mcp_servers.engram.http_headers]` TOML edit. Every existing `--auth oauth|oauth-client|bearer|none` mode keeps its shipped argv, help text, and generated prose unchanged when no header name is given.

**Verified:** 2026-09-14
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

All verification below was run live against working-tree HEAD (`3cf70558`, branch `feat/2026-09-13.01`) — no claims were taken from SUMMARY.md narrative without independent reproduction (test execution, `rg` greps against actual source, and the built binary's preview output).

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A user can register with Claude Code naming a custom header + env-var reference, rendered `"NAME: ${ENVVAR}"` (SC1) | ✓ VERIFIED | `go test ./internal/setup/ -run TestClaudeCodeHeaders -v` PASS; live binary: `/tmp/engram-verify setup --auth oauth --header x-gateway-api-key=GATEWAY_KEY --runtime claude-code --output json` → `command` contains `--header 'x-gateway-api-key: ${GATEWAY_KEY}'` |
| 2 | A user can register with opencode naming a custom header, rendered `"NAME={env:ENVVAR}"` (SC1) | ✓ VERIFIED | `TestOpenCodeHeaders` PASS; live binary run → `command` contains `--header 'x-gateway-api-key={env:GATEWAY_KEY}'` |
| 3 | A user can register with `generic` naming a custom header, carried as `"NAME": "${ENVVAR}"` in the existing `headers` JSON object | ✓ VERIFIED | `TestGenericHeaders` PASS; live binary run → `config` JSON contains `"headers":{"Authorization":"Bearer ${ENGRAM_TOKEN}","x-gateway-api-key":"${GATEWAY_KEY}"}` |
| 4 | Each runtime authors its own dialect string; no shared cross-runtime formatter exists | ✓ VERIFIED | `claudeCodeHeaderArgs` lives only in `claudecode.go`, `openCodeHeaderArgs` only in `opencode.go`, `genericHeaders.MarshalJSON` only in `generic.go`; `sortedHeaders` (the one shared helper, in `runtime.go`) only orders — `rg -n 'ToLower\|Compare' internal/setup/runtime.go` shows no dialect string literal there |
| 5 | No literal secret VALUE ever appears on argv, in written config, in preview text, `--output json`, or in logs — only an env-var reference (SC2) | ✓ VERIFIED | `TestNoSecretInArgs` — 32/32 subtests PASS (4 runtimes × 4 auth modes × {no header, header}) with a second sentinel value proven absent from every `Args` element and `Config`; `rg -n Getenv internal/setup/runtime.go internal/setup/claudecode.go internal/setup/codex.go internal/setup/opencode.go internal/setup/generic.go` shows only the pre-existing `XDG_CONFIG_HOME` read in `opencode.go` — zero header-variable environment reads anywhere in the package; `cmd/engram/setup.go`'s only new `os.Getenv` is `ENGRAM_HEADERS` (a list of NAMEs) |
| 6 | Deterministic ordering: auth header first, then extras sorted case-insensitively, byte-identical regardless of input order (D-08) | ✓ VERIFIED | `TestSortedHeadersTotalOrder`, `TestSetupHeaderOrderIndependent` PASS; live binary: `--header CF-Access-Client-Id=CF_ID --header x-gateway-api-key=GATEWAY_KEY` vs the reversed flag order produced byte-identical `command`/`headers` fields |
| 7 | Every `--auth oauth\|oauth-client\|bearer\|none` mode with no header name produces argv, help text, and generated `/engram-setup` prose identical to what shipped in `2026-08-23.01` (SC3) | ✓ VERIFIED | `help.golden`'s `Accepted --auth modes` block diffed byte-for-byte identical against both `git show ceabd8e9:cmd/engram/testdata/help.golden` (last pre-phase commit) and `git show v0.16.1:cmd/engram/testdata/help.golden` (shipped release tag) — zero diff both ways; `TestSetupGeneratedInvocations` PASS for all 4 shipped cases (`oauth`, `oauth-client`, `bearer`, `none`) plus the new `bearer+header` case |
| 8 | Naming a header other than `Authorization` for Codex produces a `failed` row whose reason names the capability gap; setup never writes `[mcp_servers.engram.http_headers]` (SC4) | ✓ VERIFIED | `TestCodexDeclinesHeaders`, `TestSetupHeaderCodexDeclined` PASS; live binary: codex row → `"outcome":"failed"`, `"reason":"codex: custom header(s) x-gateway-api-key: codex mcp add exposes only --bearer-token-env-var (no custom header flag); drop --header or exclude codex via --runtime: setup: custom header is not supported by this runtime..."`, no `command` field; `rg -n 'http_headers\|env_http_headers\|"-c"' internal/setup/codex.go` prints nothing |
| 9 | `--help`, `guides/agent-setup.md`, and the regenerated `/engram-setup` prose show the gateway header shape with its env-reference form, including the Codex limitation (SC5) | ✓ VERIFIED | Live binary `--help` output contains the `Additional headers` paragraph, the `--header NAME=ENVVAR` flag line, and the fifth example `--header x-gateway-api-key=GATEWAY_KEY`; `docs-site/.../agent-setup.md` has a `### Gateway headers` section with the per-runtime rendering table and the Codex limitation; `skill/engram/commands/engram-setup.md` has a `bearer+header` row in both generated tables plus hand-authored prose naming the Codex limitation |
| 10 | The CLI-boundary rejects an `Authorization` collision, a malformed NAME, a malformed/literal-looking ENVVAR, and a duplicate NAME, never echoing a pasted secret | ✓ VERIFIED | `TestSetupHeaderRejectsAuthorizationCollision`, `...MalformedName`, `...MalformedEnvVar`, `...DuplicateName` all PASS with `exitUsage` and zero side effects |
| 11 | `go.mod`/`go.sum` are byte-unchanged; `internal/setup` stays a stdlib-only leaf | ✓ VERIFIED | `git diff --exit-code v0.16.1 HEAD -- go.mod go.sum` exits 0 (no output) |
| 12 | The shipped-bundle privacy guard passes with the vendor-neutral rename (`x-gateway-api-key`/`GATEWAY_KEY`) applied consistently | ✓ VERIFIED | `uv run --with pytest pytest skill/engram/hooks/tests -q` → 33 passed; `rg -i litellm skill/engram/` prints only the guard file's own comment line |

**Score:** 12/12 truths verified (0 present-but-behavior-unverified)

### Prohibitions (`must_haves.prohibitions` across 02-01..02-04)

| # | Statement (abbreviated) | Tier | Disposition |
|---|--------------------------|------|-------------|
| 1 | Never dereference/resolve/print/log a header env var's VALUE | test | ✓ Enforced — `TestNoSecretInArgs` (32 subtests), zero `Getenv` of a header var (grep-confirmed) |
| 2 | Never silently downgrade/drop/coerce a header codex can't express | test | ✓ Enforced — `TestCodexDeclinesHeaders`, `TestSetupHeaderCodexDeclined` |
| 3 | Never give Codex a header via its config file or config-override flag | test | ✓ Enforced — grep confirms `codex.go` contains no `http_headers`/`env_http_headers`/`"-c"`; no TOML write path added |
| 4 | Never introduce a shared cross-runtime header formatter | test | ✓ Enforced — dialect strings confirmed present only in each runtime's own file (see Truth 4) |
| 5 | Never let a header VALUE reach generic's Config or opencode's argv | test | ✓ Enforced — same evidence as #1, extended to opencode/generic |
| 6 | Never echo a rejected `--header`'s right-hand side in a usage error/stderr | test | ✓ Enforced — `TestSetupParseHeaders`, `TestSetupHeaderRejectsMalformedEnvVar` assert the sentinel RHS absent from both `err.Error()` and captured stderr |
| 7 | Never read a header variable's value anywhere in the CLI (only `ENGRAM_HEADERS`, a list of NAMEs) | test | ✓ Enforced — grep confirms `cmd/engram/setup.go`'s only new `os.Getenv` calls are `ENGRAM_RUNTIME`/`ENGRAM_HEADERS` |
| 8 | Never turn a rejected header into a per-runtime failed row or silent no-op (must be an up-front usage error, zero effects) | test | ✓ Enforced — the four `TestSetupHeaderRejects*` tests assert `exitUsage` and `effects == 0` |
| 9 | Never document/suggest/generate a Codex header workaround inside engram's own writer or generated tables | **judgment** | LLM-judge: compliant on direct read (see below) — **flagged for human sign-off**, not silently passed |
| 10 | Never place a literal secret value in any doc/prose/fixture/generated table | test | ✓ Enforced — `rg -n 'sk-[A-Za-z0-9]\|Bearer [A-Za-z0-9]{8}' docs-site/.../agent-setup.md skill/engram/commands/engram-setup.md` prints nothing; all examples use `GATEWAY_KEY`/`${GATEWAY_KEY}` env references only |

**Prohibition #9 detail (judgment tier, 02-04-PLAN.md):** I read both hand-authored prose locations directly.
- `docs-site/src/content/docs/guides/agent-setup.md:141-146`: "Codex has no custom-header flag... so a `--header` run reports a `failed` row for `codex`... setup never writes Codex's configuration for you. Codex documents its own per-server header configuration in its config file — configure it there yourself if you need it." — this points the *operator* at Codex's own docs; it contains no TOML snippet, no flag suggestion, and no generated engram invocation for Codex headers.
- `skill/engram/commands/engram-setup.md:41-46`: "Codex cannot express an extra header... its delegation row reports `failed` naming the header — either drop `--header` or exclude codex with `--runtime`; **never hand-edit Codex's configuration to add one**." — this explicitly instructs the agent *against* a workaround.

Both read as compliant with the prohibition. Per the verification protocol, a judgment-tier prohibition is never silently absorbed into a `passed` verdict regardless of how confident the automated read is — it is recorded here as a non-authoritative LLM-judge verdict and surfaced as a human-verification item (frontmatter `human_verification`), which is why overall status is `human_needed` rather than `passed` even though every other check is green.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/setup/runtime.go` | `HeaderSpec`, `Options.Headers`, `ErrHeaderUnsupported`, `sortedHeaders` | ✓ VERIFIED | All four present at documented line numbers; doc comments state the never-dereferenced contract |
| `internal/setup/claudecode.go` | `claudeCodeHeaderArgs`, wired into all 3 case arms | ✓ VERIFIED | `rg -c -F 'claudeCodeHeaderArgs(opts.Headers)'` = 3 |
| `internal/setup/codex.go` | header-decline guard before `env.HomeDir()` | ✓ VERIFIED | `if len(opts.Headers) > 0 {` at line 88, before `home, err := env.HomeDir()` |
| `internal/setup/opencode.go` | `openCodeHeaderArgs`, wired into both case arms | ✓ VERIFIED | `rg -c -F 'openCodeHeaderArgs(opts.Headers)'` = 2 |
| `internal/setup/generic.go` | `genericHeaders` named map + `MarshalJSON` | ✓ VERIFIED | present at line 87; live binary confirms Authorization-first ordering |
| `cmd/engram/setup.go` | `--header` flag, `setupHeaderEnvDefault`, `setupParseHeaders`, `setupHeadersSummary`, help paragraph | ✓ VERIFIED | all four functions present; `--help` output confirms paragraph + example live |
| `internal/setupgen/setupgen.go` | `Case.Label`, fifth `bearer+header` case | ✓ VERIFIED | `Case{Label: "bearer+header", ...}` present; `go run ./internal/surfacesgen --check-setup` exits 0 (no drift) |
| `skill/engram/commands/engram-setup.md` | regenerated tables + hand-authored prose | ✓ VERIFIED | `bearer+header` row present in both generated tables; prose present |
| `docs-site/.../agent-setup.md` | `### Gateway headers` subsection | ✓ VERIFIED | present with rendering table and Codex limitation |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `claudecode.go` case arms | `runtime.go` `sortedHeaders`/dialect precedent | `append(...literal Args..., claudeCodeHeaderArgs(opts.Headers)...)` | ✓ WIRED |
| `codex.go` guard | `runtime.go` `ErrHeaderUnsupported` | `%w`-wrapped exactly as `ErrAuthModeUnsupported` is | ✓ WIRED |
| `apply.go` | `codex.go`/any runtime's `Plan()` error | `Outcome: OutcomeFailed, Reason: err.Error()` — untouched, free mechanism | ✓ WIRED (line 236 confirmed) |
| `cmd/engram/setup.go` `setupResolve` | `internal/setup.Options.Headers` | `setup.Options{..., Headers: headers}` (line 509) | ✓ WIRED |
| `cmd/engram/clienttest_test.go` `resetClientFlags` | `setupHeaders` package var | `setupHeaders = nil` (line 190) | ✓ WIRED |
| `cmd/engram/golden_test.go` `envDerivedFlagDefaults` | `setup.go`'s `--header` flag | `"setup": {"runtime": true, "header": true}` | ✓ WIRED |
| `internal/setupgen` `Cases()` | `cmd/engram/setup_delegation_test.go` `TestSetupGeneratedInvocations` | `for _, c := range setupgen.Cases()` iterates the 5th case through the real CLI | ✓ WIRED (subtest `bearer+header` passes both lanes) |

### Data-Flow Trace (Level 4)

The "data" here is the header spec flowing from CLI input to rendered output, traced end-to-end through the built binary (`go build -o /tmp/engram-verify ./cmd/engram`) in preview mode against a fake `$HOME`, never `--apply`:

| Stage | Source | Destination | Verified |
|-------|--------|-------------|----------|
| CLI flag → `HeaderSpec` | `--header x-gateway-api-key=GATEWAY_KEY` | `setupParseHeaders` → `setup.Options.Headers` | ✓ live run, `TestSetupParseHeaders` |
| `Options.Headers` → claude-code argv | `sortedHeaders` → `claudeCodeHeaderArgs` | `command` field: `--header 'x-gateway-api-key: ${GATEWAY_KEY}'` | ✓ live run confirmed exact string |
| `Options.Headers` → opencode argv | `sortedHeaders` → `openCodeHeaderArgs` | `command` field: `--header 'x-gateway-api-key={env:GATEWAY_KEY}'` | ✓ live run confirmed exact string |
| `Options.Headers` → generic Config | `sortedHeaders` → `genericHeaders.MarshalJSON` | `config` JSON `"headers":{"Authorization":...,"x-gateway-api-key":"${GATEWAY_KEY}"}` | ✓ live run confirmed exact JSON |
| `Options.Headers` → codex | header guard (before `HomeDir`) | `outcome: failed`, exact reason string, no `command` | ✓ live run confirmed |
| `Options.Headers` → row facet | `setupHeadersSummary` | `"headers":"x-gateway-api-key=GATEWAY_KEY"` (and multi-header sorted form) | ✓ live run confirmed, order-independent |

No hardcoded/static fallback found anywhere in this chain — every rendered value traces to the real `Options.Headers` slice through the real per-runtime `Plan()` implementation.

### Behavioral Spot-Checks / Direct Execution

Beyond running the pinned test list, the built binary was exercised directly in preview mode (no `--apply`, no real runtime CLI invoked, fake `$HOME`):

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| claude-code header rendering | `setup --auth oauth --header x-gateway-api-key=GATEWAY_KEY --runtime claude-code --output json` | `--header 'x-gateway-api-key: ${GATEWAY_KEY}'` in `command` | ✓ PASS |
| opencode header rendering | same, `--runtime opencode` | `--header 'x-gateway-api-key={env:GATEWAY_KEY}'` | ✓ PASS |
| generic header rendering | same, `--runtime generic`, `--auth bearer` | `headers` JSON object, Authorization first | ✓ PASS |
| codex decline | same, `--runtime codex` | `failed` row, exact decline reason, no `command` | ✓ PASS |
| order independence | two headers, flag order vs. reversed flag order | byte-identical `command`/`headers` both times | ✓ PASS |
| `--help` documents the flag | `setup --help` | `Additional headers` paragraph + fifth example present | ✓ PASS |

### Probe Execution

Not applicable — this phase is not a migration/tooling phase with `scripts/*/tests/probe-*.sh` conventions; no probes declared in PLAN/SUMMARY. `go run ./internal/surfacesgen --check-setup` (the phase's own drift-detection "probe" for generated docs) was run and exits 0.

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|--------------|--------|----------|
| REQ-header-name-parameter | 02-01, 02-02, 02-03, 02-04 | Name the header for Claude Code/opencode/generic, per-runtime syntax, no shared formatter | ✓ SATISFIED | Truths 1-4, all header-rendering tests PASS |
| REQ-header-value-env-ref-only | 02-01, 02-02, 02-03 | Value is always an env-var reference, never a literal | ✓ SATISFIED | Truth 5, Prohibitions 1/5/7/10 |
| REQ-header-bearer-unchanged | 02-01, 02-02, 02-03, 02-04 | Zero-header paths byte-identical to shipped `2026-08-23.01` | ✓ SATISFIED | Truth 7 (diffed against v0.16.1 and pre-phase commit) |
| REQ-header-codex-declined | 02-01, 02-03, 02-04 | Codex declines with a named-gap `failed` row, never hand-writes TOML | ✓ SATISFIED | Truth 8, Prohibitions 2/3 |
| REQ-header-documented | 02-03, 02-04 | `--help`, `agent-setup.md`, `/engram-setup` prose document the shape | ✓ SATISFIED | Truth 9 |

All 5 REQ IDs declared across the phase's plans are present in `.planning/REQUIREMENTS.md`, mapped to `Phase 2`, and marked `Complete`. No orphaned requirements found (`grep -E "Phase 2" .planning/REQUIREMENTS.md` shows exactly these 5 IDs, all accounted for above).

Note: `.planning/REQUIREMENTS.md` line 37 still reads "LiteLLM's `x-litellm-api-key`" — this is superseded example text from before the 02-04 checkpoint's vendor-neutral rename (`x-gateway-api-key`/`GATEWAY_KEY`). Per the task's explicit instruction, this is known superseded history in a planning artifact, not a code gap — the actual shipped code, tests, and docs all use the vendor-neutral identifier consistently (confirmed above). Not flagged as a gap.

### Anti-Patterns Found

None. Scanned all phase-modified non-test and test files for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`-style debt markers, `not implemented`/`coming soon`/`will be here` stub prose, and empty-return stubs. The only "placeholder" hits are the pre-existing, correctly-used `bearerProvenance` path-provenance-placeholder terminology (a design term describing the `<from PATH>` display form, unrelated to incompleteness) — not a debt marker.

One unrelated, pre-existing `go vet` finding was observed (`cmd/engram/operator_view_test.go:441`, a deliberate duplicate JSON tag under `//nolint:govet`, introduced 2026-08-22, not touched by this phase) — noted for completeness, not a phase-02 gap.

### Repo-Wide Gate (run once, this session)

- `go build ./...` — exit 0
- `go test ./... -count=1` — all 20+ packages `ok`, 0 `FAIL` lines (internal/store alone took ~49s; full run ~90s)
- `go test ./internal/setup/ -count=1 -shuffle=on` — `ok`
- `go test ./internal/setupgen/ -count=1` — `ok`
- `go run ./internal/surfacesgen --check-setup` — exit 0
- `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1` — `ok` (27.5s)
- `go test ./internal/keylinks/ -count=1` — `ok`
- `git diff --exit-code v0.16.1 HEAD -- go.mod go.sum` — exit 0 (no new deps)
- `uv run --with pytest pytest skill/engram/hooks/tests -q` — 33 passed
- `gofmt -l internal/setup/ cmd/engram/ internal/setupgen/` — no output (clean)

### Human Verification Required

1. **Codex-workaround prohibition (judgment tier)** — see `human_verification` frontmatter and Prohibitions §9 above. My own read found both `skill/engram/commands/engram-setup.md` and `docs-site/.../agent-setup.md` compliant (they point the operator at Codex's own config, one file explicitly says "never hand-edit Codex's configuration to add one"), but this is a judgment-tier prohibition that the verification protocol requires surfacing for explicit human sign-off rather than silently folding into a passed verdict.

### Gaps Summary

No gaps. All 12 observable truths derived from the 5 ROADMAP success criteria plus the phase's `must_haves.truths` across all four plans are verified with reproduced evidence (test execution + live binary preview runs + source greps), not SUMMARY.md narrative. All 5 REQ IDs are satisfied and correctly mapped. All 10 `must_haves.prohibitions` across the four plans are enforced; 9 of 10 are test-tier with passing named tests, and the 1 judgment-tier prohibition reads as compliant on direct inspection but is routed to human verification per protocol rather than silently passed — this is the sole reason overall status is `human_needed` rather than `passed`.

---

*Verified: 2026-09-14*
*Verifier: Claude (gsd-verifier)*

## Re-fingerprint 2026-09-15 (orchestrator)

Phase 3 (Plugin-First Delivery) additively edited 10 files in this phase's `covered_files`
(`internal/setup/{claudecode,codex}.go`, `cmd/engram/{setup.go,setup_test.go,setup_delegation_test.go,operator_view_setup_test.go,testdata/help.golden}`,
`internal/setupgen/{setupgen.go,setupgen_test.go}`, `skill/engram/commands/engram-setup.md`), which
correctly flipped the covered digest and this report to `stale`.

This phase's CONCLUSION is unchanged and was re-proven at Phase 3's HEAD before re-fingerprinting —
every Phase 2 test passes: `TestClaudeCodeHeaders`, `TestOpenCodeHeaders`, `TestGenericHeaders`,
`TestCodexDeclinesHeaders`, `TestNoSecretInArgs`, `TestSortedHeadersTotalOrder` (`internal/setup`);
the nine `TestSetupParseHeaders`/`TestSetupHeader*` tests (`cmd/engram`, `-count=1`); and the full
`internal/setupgen` suite. The digest is re-pinned to the current bytes so the staleness signal stays
meaningful for the NEXT unrelated change rather than staying permanently tripped.

