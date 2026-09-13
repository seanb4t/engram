# Requirements: engram — Milestone `2026-09-13.01` Setup v2

**Defined:** 2026-09-13
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its
context, and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as
the store grows.

**Milestone goal:** `engram setup` is safe to re-run against a real machine — it delivers skills,
hooks, and `/engram-setup` through the runtime's own plugin system where one exists, can express
any working auth shape, and never replaces a registration it did not write.

> **Research basis.** `.planning/research/SUMMARY.md` (2026-09-13). Two corrections it surfaced
> are already applied here: shell completions shipped in `2026-08-23.01` Phase 1 (only man pages
> are new work), and `codex mcp add` (codex-cli 0.154.0, verified live) has no generic header flag
> — a custom header name is *declined* for Codex, never coerced onto `--bearer-token-env-var` and
> never written into `config.toml` by hand. One live-verified cross-cutting fact: `claude mcp get`
> and `codex mcp get --json` echo header **values** in cleartext for registrations engram did not
> write, so every probe-derived surface redacts values unconditionally.

## v1 Requirements

### Executor Correctness

- [x] **REQ-osrun-deadline-error**: When a runtime subprocess is killed because its context deadline expired, `osRun` reports the deadline error (`ctx.Err()`) rather than a clean nonzero exit with a nil error, so the executor's timeout path engages and the result row names the timeout. GitHub #560 (carried W01).

### Man Pages

- [x] **REQ-manpages-generated**: The released binary can generate its own man pages from the live cobra command tree (`engram man <dir>`, hidden like `completion`), one page per command, with deterministic output (no auto-generated timestamp) so re-generation is byte-stable. Zero new Go dependencies — `cobra/doc` is promoted from indirect to direct only.
- [x] **REQ-manpages-cask-installed**: The Homebrew cask installs the generated man pages on `post_install` and removes exactly those paths on `post_uninstall`, symmetric with the shipped completions hooks; the version gate still runs first, and `generate_completions_from_executable` stays absent. Ordering and absence are pinned by `releaseconfig_test.go` like the completions step already is. Shell completions themselves require no work — they shipped in `2026-08-23.01`.

### Custom Auth Headers

- [ ] **REQ-header-name-parameter**: A user can name the auth header (for example `x-litellm-api-key`) in addition to the env-var reference carrying its value, so a gateway registration is expressible for Claude Code, opencode, and `generic`. Each runtime renders the header in its own CLI syntax (`"Name: value"` for `claude mcp add --header`, `Name=value` for `opencode mcp add --header`), authored per runtime — never through a shared cross-runtime formatter.
- [ ] **REQ-header-value-env-ref-only**: A header value is always an env-var *reference* in the runtime's own reference syntax (`${VAR}`, `{env:VAR}`); no literal secret ever appears on argv, in a written config, in preview text, in `--output json`, or in logs. The existing bearer provenance path is generalized, not duplicated.
- [ ] **REQ-header-bearer-unchanged**: `--auth oauth | oauth-client | bearer | none` keep their shipped behavior, argv, help text, and generated `/engram-setup` prose when no header name is given; the new capability is additive.
- [ ] **REQ-header-codex-declined**: For Codex, a header name other than `Authorization` yields a `failed` row whose reason names the capability gap (`codex mcp add` exposes only `--bearer-token-env-var`), exactly as `oauth-client` is already declined for opencode. Setup never silently downgrades the name and never writes `[mcp_servers.engram.http_headers]` by hand.
- [ ] **REQ-header-documented**: `engram setup --help`, `guides/agent-setup.md`, and the regenerated `/engram-setup` prose show the gateway header shape (LiteLLM's `x-litellm-api-key`) with the env-reference form, including the Codex limitation.

### Drift Detection & Reconcile

- [ ] **REQ-drift-observed-registration**: Preview reads the runtime's existing engram registration through the runtime's own read verb and normalizes it to the same shape the Plan authors — URL, auth mode, header names — using Codex's `mcp get --json` as structured input and a bounded text scan for Claude Code; opencode's table output is explicitly not parsed, and its comparison is documented as coarse. No third-party config file is read.
- [ ] **REQ-drift-three-way**: A registration is classified as one of exactly three states — identical (`already-correct`), differs-but-reproducible (`would-write`), or differs-and-not-reproducible (`preserved`) — and the last two are never collapsed. `already-correct` is a real comparison with unit coverage for all three states per runtime, not a read-probe heuristic.
- [ ] **REQ-drift-preserved-outcome**: `preserved` is a first-class outcome in text and JSON output, with a reason naming what setup cannot reproduce (e.g. a header it does not author), reflected consistently in aggregation and exit codes, and documented in `guides/agent-setup.md`'s results table. Codex's semantics are whole-entry (preserve the runtime or overwrite it — no partial merge) and the docs say so.
- [ ] **REQ-drift-facet-naming**: A `would-write` row for an existing registration names which facet(s) differ — URL, auth mode, header name, or value reference — rather than a bare "differs".
- [ ] **REQ-drift-redaction**: Header values obtained from any runtime read-probe are redacted unconditionally before comparison storage, rendering, JSON output, or logging — setup never tries to tell a safe-looking reference from a literal secret. Verified with a fixture whose probe output carries a literal value.
- [ ] **REQ-apply-preserve-gate**: `--apply` consults the same classification before writing and performs zero write actions for a `preserved` registration — including never running Claude Code's `mcp remove` step — while still applying skills/plugin actions for that runtime. Proven by a fixture test that runs `--apply` (not only preview) against a pre-seeded unreproducible registration and asserts no registration write was issued.
- [ ] **REQ-apply-rewrite-consequence**: When a reproducible difference on Claude Code requires remove-then-add of an existing registration, preview and apply state that an OAuth-authenticated registration will need to log in again before the rewrite runs.

### Plugin-First Delivery

- [ ] **REQ-plugin-capability-detection**: Setup detects whether a present Claude Code or Codex binary actually exposes a working `plugin` CLI, separately from binary-on-PATH detection; a runtime without it falls back to the native skills copy with a reported reason, never a failed runtime row.
- [ ] **REQ-plugin-install-or-update**: Under `--apply`, for a plugin-capable Claude Code or Codex, setup adds engram's own marketplace when absent, installs the engram plugin when absent, updates it when outdated, and does nothing when already current. Preview shows the exact plugin-CLI argv; `--apply` is the consent gate — no additional flag. Only engram's own marketplace source is ever added.
- [ ] **REQ-plugin-three-way-state**: Plugin state is reported as one of absent / installed-but-outdated / installed-and-current, comparing the runtime's `plugin list` output against the version the binary itself carries, since neither vendor CLI reports "up to date" vs "updated" itself.
- [ ] **REQ-plugin-skips-skills-copy**: Plugin delivery and the native skills copy are mutually exclusive per runtime per run — a plugin-delivered runtime gets no files under its user-scope skills directory and no `AGENTS.md` index block, so `curating-memory` never appears twice. Setup never removes a skills path without first checking for a managed symlink.
- [ ] **REQ-plugin-facet-reported**: Plugin delivery is its own result facet (text and JSON) alongside registration and skills, so a `wrote` registration next to a `failed` plugin install stays visible, and the exit taxonomy accounts for it.
- [ ] **REQ-codex-plugin-manifest**: `skill/engram/.codex-plugin/plugin.json` exists so Codex's plugin loader accepts the engram plugin; its version is release-please-synced like `.claude-plugin/plugin.json`, and a drift gate keeps the two manifests' identity fields equal.
- [ ] **REQ-plugin-setupgen-regenerated**: `/engram-setup`'s generated prose reflects plugin actions and the new outcomes in the same change that introduces them; the existing `setupgen` CI drift gate stays green.

### Documentation

- [ ] **REQ-docs-setup-v2**: `guides/install.md`, `guides/agent-setup.md`, and `guides/plugin.md` describe the shipped behavior — plugin-first delivery per runtime, the header shape, the `preserved` outcome and apply gate, and man pages — with a post-release live observation recorded before the requirement is checked off (the `2026-08-23.01` D-10 pattern).

## v2 Requirements

Deferred. Tracked, not in this roadmap.

- **REQ-register-cursor**: Cursor support — still the one target needing a config-file writer (`~/.cursor/mcp.json`, merge-never-replace). Carried from `2026-08-23.01`.
- **REQ-drift-opencode-structured**: Parse opencode's registration for full-fidelity comparison. Its `mcp list` prints a box-drawing table this repo already declined to parse; revisit if opencode gains a `--json` read verb.
- **REQ-plugin-opencode**: Plugin delivery for opencode, if opencode ships a plugin CLI. Native skills copy remains its path.
- **REQ-setup-core-maintenance**: The `2026-08-23.01` maintenance observations (duplicate failed-count calculation, validation ordering before the missing-URL check, the "verbatim" capture comment, the un-captured "emits no warning" human check).

## Out of Scope

| Feature | Reason |
|---------|--------|
| Parsing or writing TOML / JSONC to give Codex a custom header | `codex mcp add` cannot express it; hand-appending `[mcp_servers.engram.http_headers]` without a parser breaks on a duplicate table header, and a parser breaks the zero-new-dependency and shell-out-only constraints. Decline explicitly instead. |
| Merging a preserved header into a live write ("auto-reconcile") | Reconciliation here means *preserve*, never rewrite a registration engram did not author. Merging would reopen the exact overwrite incident this milestone fixes. |
| A separate plugin-install consent flag | Decided at scoping: `--apply` is the consent gate; preview already shows the exact plugin-CLI argv and the marketplace is engram's own. |
| Adding or trusting a foreign marketplace | Only engram's own marketplace source is ever registered. |
| Literal secret values as flags or config | A secret is only ever an env-var reference; this constraint is inherited, not relaxed. |
| Shell completions | Shipped in `2026-08-23.01` Phase 1 (cobra `completion` + cask hooks, pinned by `releaseconfig_test.go`). Not re-scoped. |
| Verifying against the operator's real `$HOME` or a real third-party CLI from tests | Rule `m45p2b4bp7` and gotcha `ryr82bf2s2`: fake seams only. A plan that documents `--apply` as a verification step is an attractive nuisance. |
| Cursor, team/org config, User Rules | Deferred with `REQ-register-cursor`. |

## Traceability

Which phases cover which requirements. Filled during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-osrun-deadline-error | Phase 1 | Complete |
| REQ-manpages-generated | Phase 1 | Complete |
| REQ-manpages-cask-installed | Phase 1 | Complete |
| REQ-header-name-parameter | Phase 2 | Pending |
| REQ-header-value-env-ref-only | Phase 2 | Pending |
| REQ-header-bearer-unchanged | Phase 2 | Pending |
| REQ-header-codex-declined | Phase 2 | Pending |
| REQ-header-documented | Phase 2 | Pending |
| REQ-drift-observed-registration | Phase 4 | Pending |
| REQ-drift-three-way | Phase 4 | Pending |
| REQ-drift-preserved-outcome | Phase 4 | Pending |
| REQ-drift-facet-naming | Phase 4 | Pending |
| REQ-drift-redaction | Phase 4 | Pending |
| REQ-apply-preserve-gate | Phase 5 | Pending |
| REQ-apply-rewrite-consequence | Phase 5 | Pending |
| REQ-plugin-capability-detection | Phase 3 | Pending |
| REQ-plugin-install-or-update | Phase 3 | Pending |
| REQ-plugin-three-way-state | Phase 3 | Pending |
| REQ-plugin-skips-skills-copy | Phase 3 | Pending |
| REQ-plugin-facet-reported | Phase 3 | Pending |
| REQ-codex-plugin-manifest | Phase 3 | Pending |
| REQ-plugin-setupgen-regenerated | Phase 3 | Pending |
| REQ-docs-setup-v2 | Phase 5 | Pending |

**Coverage:**

- v1 requirements: 23 total
- Mapped to phases: 23
- Unmapped: 0 ✓

---

*Requirements defined: 2026-09-13*
*Last updated: 2026-09-13 after roadmap creation*
