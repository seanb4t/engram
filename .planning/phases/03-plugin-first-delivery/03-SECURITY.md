---
phase: "03"
slug: "plugin-first-delivery"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-17"
---

# Phase 03 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| vendor CLI stdout → `ParsePluginList` / `ParseMarketplaceList` | `plugin list --json` / `marketplace list` output is untrusted third-party data: parsed by exact field/name match, never interpolated into argv, never executed. | Plugin/marketplace listing JSON (untrusted) |
| `internal/setup` → vendor plugin write verbs | The lane execs authored argv (fixed literals only) through `runSeam`; a marketplace-declared install command runs code neither engram nor the operator wrote — scoped to engram's own marketplace. | Fixed argv; marketplace source identity |
| `internal/skills.DetectPresence` → operator filesystem | Reads (`Lstat`, `ReadFile`) an absolute destination authored by a runtime's own `Plan()`; never writes. | Native skills path, `AGENTS.md` (read-only) |
| `PluginResult` → `setupRuntimeRow` → terminal / `--output json` | Bounded vendor stderr and a marketplace source line become row values; rendered only through `sanitizeViewValue`'s scalar branch. | Result rows (data only) |
| plugin facet → skills routing | One boolean (`Delivered()`) decides whether the native writer runs; a wrong value duplicates or omits skills — never deletes. | Delivery boolean |
| `skill/engram/**` → every plugin installer | Shipped bundle is public content; any private host or vendor id leaks to every installer. | Manifests, commands, skills, hooks |
| `release-please-config.json` → both manifests | One config rewrites two `version` fields. | Version/identity fields |
| `setupgen.Render` → `/engram-setup` prose → agent | Generated tables tell an agent what `--apply` runs; a stale or re-typed argv breaks the AUTHORED-HERE invariant. | Command tables |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 | Elevation of Privilege | `PluginActions` authoring `marketplace add` for a source engram does not own, or re-pointing an operator's `engram` marketplace | high | mitigate | Only source literals are `seanb4t/engram` (`internal/setup/claudecode.go:226`) and `https://github.com/seanb4t/engram` (`codex.go:170`), each a fixed `Action` var; `marketplace add` is authored only when the probe found no marketplace named `engram` (D-04/D-05); probe failure yields `PluginUnavailable`, never a write on unknown state; `TestPluginPlan` (`plugin_test.go`) pins the forks | closed |
| T-03-02 | Tampering | crafted/corrupted `plugin list --json` entry steering the state machine | medium | mitigate | Exact `id == "engram@engram"` / `name && marketplaceName == "engram"` matching; worst lie outcome is `current` (no action) or `absent` (reinstall) — never destructive; nothing parsed reaches argv; `TestPluginPlan` seeds decoy entries | closed |
| T-03-06 | Denial of Service | hung vendor CLI blocking `engram setup` | low | mitigate | Every probe/action runs through `runSeam` (`plugin.go`, `apply.go`) under `execTimeout`; deadline reported as `timed out after 20s`; a probe deadline degrades to the native path (D-10/D-12) | closed |
| T-03-07 | Repudiation | `current`-vs-`wrote` misclassification hiding what `--apply` did | low | mitigate | `Command` is `Plan.Display()` over the exact actions run; `AggregateOutcome` (`cmd/engram/setup.go`) reports `wrote` only after every action exited 0, `already-correct` only for zero actions, `failed` with the failing argv | closed |
| T-03-08 | Tampering | test suite invoking a real `claude`/`codex` plugin verb or the operator's `$HOME` | medium | mitigate | `plugin_test.go` has zero `OSEnvironment`; `fakeEnvWithRun` (81 uses) / `fakePluginRun` (scripted by `filepath.Base(path)` + args) / `fakeSkillsEnvWithEntries` drive every plugin test; recorded argv asserted exactly | closed |
| T-03-03 | Tampering | setup deleting/rewriting a native skills path or the operator's `AGENTS.md` while switching to plugin delivery | high | mitigate | `internal/skills/presence.go` has zero `WriteFile`/`Remove`/`MkdirAll`/`Create` identifiers (only `Lstat` ×3, `ReadFile` ×2); `setupReportNativeSkills` never calls `skills.Install`; `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites` asserts zero writes and an unchanged seeded `AGENTS.md`; removal deferred to a future explicit verb (D-08) | closed |
| T-03-09 | Information Disclosure | presence report leaking `AGENTS.md` content | low | mitigate | Only the boolean `IndexBlock` is derived from the index read (`presence.go`); content never leaves the function | closed |
| T-03-10 | Tampering | test suite touching the operator's real home | medium | mitigate | Real-FS subtest confined to `t.TempDir()`; other subtests use a map-backed fake; `TestSkillsPackageImportsAreGated` (`importgate_test.go`) keeps the import allowlist unchanged | closed |
| T-03-04 | Repudiation | failed plugin install laundered into a successful row, or an unavailable probe failing a healthy registration | high | mitigate | `setupApplyPluginFacet` folds `failed` through `AggregateOutcome` (row `failed`, reason `plugin: …`) and folds nothing for `unavailable`; `TestSetupApplyJSONEmitsPluginFacet` (exitPartial) and `TestSetupPluginUnavailableFallsBackToNative` (exit 0, `wrote`) pin both directions | closed |
| T-03-11 | Information Disclosure | third-party stdout/stderr or marketplace source string forging report structure | low | mitigate | Every facet value is a flat string through `sanitizeViewValue` (C0/DEL stripped) and `boundCapture` (4096-byte bound); `TestOperatorViewFixturesHaveNoUnsanitizedNesting` over the plugin fixtures | closed |
| T-03-12 | Tampering | test binary's own `dev` version driving assertions | low | mitigate | `setupVersion` seam + `withFakeSetupVersion(t, "0.16.1")` (10 uses in `setup_test.go`); production keeps `resolvedVersion` | closed |
| T-03-05 | Information Disclosure | vendor-specific or private-deployment substring shipped in `.codex-plugin/plugin.json` or regenerated prose | high | mitigate | Description byte-copied from the neutral Claude manifest; `skill/engram/hooks/tests/test_no_residual_memory_oauth.py` scans every file under `skill/engram/`; `TestPluginManifestIdentityMatches` (`internal/setupgen/manifest_test.go`) re-checks banned substrings non-literally | closed |
| T-03-13 | Tampering | the two manifests' identity fields diverging across a release | medium | mitigate | `TestPluginManifestIdentityMatches` asserts byte-equality of `name`/`version`/`description` and the exact `extra-files` entry (`release-please-config.json:27` names `skill/engram/.codex-plugin/plugin.json`) | closed |
| T-03-14 | Tampering | `/engram-setup` table carrying an argv that differs from what `--apply` runs | medium | mitigate | Table rendered from `PluginActions` through `Plan.Display()` — never a literal in setupgen; `surfacesgen --check-setup` is the CI drift gate; `TestRenderPluginTable` pins rows against real actions | closed |
| T-03-15 | Elevation of Privilege | `hooks` or `mcpServers` key in the codex manifest causing duplicate-hooks load or a bundled server | low | mitigate | Manifest carries exactly four keys (`$schema`, `name`, `version`, `description` — verified); zero `hooks`/`mcpServers` hits; asserted by `TestPluginManifestIdentityMatches` | closed |
| T-03-SC | Tampering | npm/pip/cargo installs | low | accept | Not applicable — zero new packages; stdlib-only leaf preserved; pytest runs via the repo's existing `uv run --with pytest`; `go.mod`/`go.sum` byte-unchanged — see AR-03-01 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-03-01 | T-03-SC | No package-manager install task in any of the four plans; `internal/setup` remains stdlib-only; the skills import allowlist is unchanged; `go.mod`/`go.sum` byte-unchanged. Supply-chain control is not applicable. | Plan author (03-01..04 threat models) | 2026-09-17 |
| AR-03-02 | T-03-01 (residual, by design) | A marketplace already named `engram` — whatever it points at — is used as-is and never re-pointed (D-05). An operator who deliberately configured a foreign `engram` marketplace has that choice honored; engram reports the observed source rather than overriding it. | Sean (decision dmgmtc0q9h, 2026-09-14) | 2026-09-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-17 | 16 | 16 | 0 | /gsd-secure-phase (orchestrator, L1 grep-depth short-circuit — register authored at plan time, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-17
