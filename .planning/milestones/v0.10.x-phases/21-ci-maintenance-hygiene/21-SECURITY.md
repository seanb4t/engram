---
phase: 21
slug: ci-maintenance-hygiene
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-16
register_authored_at_plan_time: true
---

# Phase 21 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> State B run (no prior SECURITY.md; register built from the three PLAN.md `<threat_model>`
> blocks). ASVS L1, block-on `high`. All three plans authored their threat model at plan
> time (`register_authored_at_plan_time: true`), both HIGH threats are mitigated, and
> `threats_open: 0` — the L1 short-circuit applied (no auditor spawn; grep-depth
> mitigation verification performed inline against the shipped code, not SUMMARY claims).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| CI runner ↔ GitHub API (Plan 21-03) | The `ui-drift` job mints a GitHub App installation token and pushes to a PR branch | App private key (secret), short-lived installation token, git push over HTTPS |
| Renovate bot PR ↔ elevated-credential CI step (Plan 21-03) | Untrusted/attacker-influenceable PR context (`head_ref`, `actor`) gates access to the token-mint step | branch name, actor login, `head.repo.full_name` |
| Write handler ↔ async summary queue (Plan 21-02) | `persistAndEnqueue` sits downstream of owner/actor authz stamping; hands record ids to a background pool | memory id (post-authz), no credentials |
| Local lint gate ↔ shipped Markdown (Plan 21-01) | `.rumdl.toml` exclude scope determines what prose is still linted | none (config only) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-21-01-01 | Tampering | `.rumdl.toml` exclude array | low | mitigate | Plain one-line `.planning` exclude (not a glob); Task-1 scope probe proved a repo-root `.md` violation is still reported (`Found 4 issues in 1 file`, exit 1) — verified live by the verifier. | closed |
| T-21-01-02 | Repudiation | ROADMAP/REQUIREMENTS acceptance list | low | mitigate | SC2/SC3 + REQUIREMENTS.md corrected (commit `5a4bd691`); IN-01 relabeled to the real dedup, stale count dropped; confirmed via negative grep. | closed |
| T-21-01-03 | Information Disclosure | `.planning/` contents | low | accept | Excluding `.planning/` from lint does not change its visibility — already committed and public. See Accepted Risks. | closed |
| T-21-02-01 | Tampering | `persistAndEnqueue` (`tools.go:670`) | medium | mitigate | `TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure` (`tools_test.go:846`) asserts zero enqueues on a failed Upsert for both handlers; passed BEFORE and after the refactor (characterization test). | closed |
| T-21-02-02 | Denial of Service | `persistAndEnqueue` / `tryEnqueue` | medium | mitigate | Non-blocking `tryEnqueue` semantics preserved verbatim; `TestStoreMemoryReturnsWhenSummarizerHangs` (`tools_test.go:750`) present and passing. | closed |
| T-21-02-03 | Denial of Service | `Wait()` on both queues | low | mitigate | `Wait()` moved to `queue_export_test.go`; absent from every file `go build` compiles (`go build ./cmd/engram` produces a `Wait()`-free binary) — no production caller can be written. | closed |
| T-21-02-04 | Elevation of Privilege | write-path authz (owner/actor stamping) | low | accept | Extracted region is entirely downstream of authz stamping; no authz logic moved. `TestStoreMemoryStampsOwnerHandler` unaffected. See Accepted Risks. | closed |
| T-21-02-05 | Information Disclosure | test env handling (IN-02) | low | mitigate | `TestBuildDepsFromEnvLoadsConfigOnce` clears `ENGRAM_SUMMARY_MODEL`/`ENGRAM_SUMMARY_ON_WRITE` via `t.Setenv` (`tools_test.go:1709-1710`) — no ambient env can point a leaked worker at a real summarizer. | closed |
| T-21-03-01 | Elevation of Privilege | App-token mint step guard | **high** | mitigate | Three-signal guard on the mint step (`ci.yaml:195-200`); the `github.event.pull_request.head.repo.full_name == github.repository` conjunct (not spoofable by a fork PR author) is load-bearing. Fork PRs never reach the mint. | closed |
| T-21-03-02 | Elevation of Privilege | GitHub App installation permissions | **high** | mitigate | Two layers: (a) App provisioned `Contents: Read & write` only (human-enforced at the Task-3 checkpoint; installed on `seanb4t/engram` only); (b) mint step self-scopes the token via `permission-contents: 'write'` (`ci.yaml:205`) — even an over-provisioned App yields a narrow token. `RELEASE_APP` deliberately not reused. | closed |
| T-21-03-03 | Spoofing | branch-name / actor guard | medium | mitigate | Actor conjunct `github.actor == 'fzymgc-renovate[bot]'` (`ci.yaml:199`) — the repo's live self-hosted bot identity, not the default Mend name. A human `renovate/experiment` branch fails the actor check. | closed |
| T-21-03-04 | Information Disclosure | App key / minted token in logs | medium | mitigate | Key stored as a GitHub secret; action masks the token by default and auto-revokes it in its `post` step; token passed via `env: GH_TOKEN` + `gh auth setup-git` (`ci.yaml:212/218/229`), never echoed or embedded in a remote URL. | closed |
| T-21-03-05 | Tampering | self-heal push target (merge ref vs branch tip) | medium | mitigate | Merge-ref correction: shallow-clones the true PR head branch and commits there (`ci.yaml:230-245`) rather than pushing the `refs/pull/N/merge` checkout — no silent `main`-into-branch merge. | closed |
| T-21-03-06 | Tampering | `main` branch | low | mitigate | `github.event_name == 'pull_request'` conjunct (`ci.yaml:197`) — the `push`-to-`main` trigger has no PR context, so `main` never self-heals; `protect-main` ruleset stands (App is not a bypass actor). | closed |
| T-21-03-07 | Tampering | `ui/` dependency tree → committed SPA bundle | medium | accept | Rebuild step already runs unconditionally on every PR and is unchanged; self-heal commits only what that already-reviewed build produces; `pnpm install --frozen-lockfile` pins the resolved tree. No new attack surface. See Accepted Risks. | closed |
| T-21-03-08 | Repudiation | self-heal commit attribution | low | mitigate | Committer identity derived at runtime from the action's `app-slug` output (`ci.yaml:206-214/241-242`); no fabricated identity hardcoded. | closed |
| T-21-03-09 | Denial of Service | Renovate PR auto-merge | medium | mitigate | App installation token is used for the push (`ci.yaml:193`), which DOES trigger `pull_request: synchronize` so required checks rerun on the new SHA — avoiding the `GITHUB_TOKEN` "Expected"-forever wedge. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

**17 threats total · 17 closed · 0 open.** Both HIGH threats (T-21-03-01, T-21-03-02) mitigated. No critical threats. No supply-chain rows: no plan installs any npm/Go/pip/cargo package or adds a dependency.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-21-01 | T-21-01-03 | Excluding `.planning/` from the lint gate does not change its visibility — it is already committed and public. Lint scope has no bearing on disclosure. | Sean (plan-authored, low severity) | 2026-07-16 |
| AR-21-02 | T-21-02-04 | The `persistAndEnqueue` extraction is entirely downstream of owner/actor authz stamping (`m` is fully built before it); no authz logic is moved, read, or re-derived. | Sean (plan-authored, low severity) | 2026-07-16 |
| AR-21-03 | T-21-03-07 | The `ui/` build/rebuild already runs unconditionally on every PR and is unchanged by this plan; the self-heal step commits only that already-reviewed build output, and `--frozen-lockfile` pins the exact tree. No new attack surface. | Sean (plan-authored, medium severity — bounded to existing surface) | 2026-07-16 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-16 | 17 | 17 | 0 | /gsd-secure-phase (State B, ASVS L1 short-circuit — register_authored_at_plan_time:true, threats_open:0, no auditor spawn; grep-depth mitigation verification against shipped code) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log (AR-21-01/02/03)
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-16
