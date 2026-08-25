---
phase: 01-version-homebrew-distribution
audited: 2026-08-25
verdict: SECURED
asvs_level: 1
block_on: high
threats_open: 0
threats_total: 16
threats_unique: 14
register_authored_at_plan_time: true
rewritten_rows: [T-01-08, T-01-12]
---

# Phase 01 — Security Audit

**Verdict: SECURED.** 16/16 threat rows closed (14 unique IDs; `T-01-SC` repeats once per
plan). No open threat at or above the `high` block threshold.

Register origin: authored at plan time — all three PLAN.md files carry a `<threat_model>`
block. The audit verified that each stated mitigation exists in the implementation; it did
not scan for new threats generally, except where the design changed after planning (below).

## The design changed after the register was written

The credential design was changed on the maintainer's explicit instruction *after* all three
plans executed and after phase verification passed (commit `f67622eb`; SUMMARY corrected in
`dcafc5c1`). The original design widened the existing release App with
`repositories: engram,homebrew-tap`, so one credential served two repositories and two
purposes. That was rejected on the principle that an App exists to do one job.

Two register rows described the old arrangement and were re-derived against the code as it
now stands rather than accepted as written.

### T-01-08 — Elevation of Privilege, HIGH — CLOSED (rewritten)

The stale mitigation read: *"`repositories: engram,homebrew-tap` is a closed allowlist naming
exactly the two repositories the phase needs; the `owner:` input, which would grant every
repository the App is installed on, is explicitly not used."*

That text no longer describes the code, and the new code **does** pass `owner:` — the very
input the original row warned about. This was scrutinized specifically because it was
introduced late, by the orchestrator, and had not been reviewed.

Settled by reading the pinned action's own compiled source rather than its README prose —
`actions/create-github-app-token` at `bcd2ba49218906704ab6c1aa796996da409d3eb1`,
`dist/main.cjs:23222-23261`, `resolveInstallationTarget`:

| Inputs | Dispatch | Resulting scope |
|---|---|---|
| `owner` set, `repositories` **empty** | `type: "owner"` → `getTokenFromOwner` | every repo the App is installed on — **the widening case** |
| `owner` **and** `repositories` set | `normalizeRepositoryTarget` → `type: "repository"` → `getTokenFromRepository` | exactly the listed repositories |

With `repositories:` present, `owner:` only selects which installation account to query; it
adds no scope. The action's own documented example uses this combined form.

Current mitigation, verified in code:

- `release.yaml:32-42` — the release App's mint carries **no** `repositories:` and **no**
  `owner:` input, so it defaults to *this repository only*. It cannot reach the tap under any
  circumstance.
- `release.yaml:49-55` — a dedicated tap-publisher App mint, `owner: seanb4t` +
  `repositories: homebrew-tap`.
- `verify-tap-credential.yaml:19-30` — the probe mints the same tap App, so it proves the
  credential the cask push will actually hold.
- `.goreleaser.yaml` — `homebrew_casks[].repository.token` consumes `HOMEBREW_TAP_TOKEN`,
  overriding `GITHUB_TOKEN` for that publisher only.
- `rg 'engram,homebrew-tap' .github/ .goreleaser.yaml` → **zero matches**. No credential spans
  both repositories.

This is a stronger posture than the design the row was originally written against: the
release credential cannot write the tap, and the tap credential cannot cut a release.

### T-01-12 — Repudiation, low — accept (rationale rewritten)

The stale rationale read: *"the tap write is performed by the same App identity that already
cuts this repository's tags and GitHub Releases, so the actor is attributable through the
App's own audit trail."* That is now factually false — there are two single-purpose Apps.

Disposition remains `accept`, on a stronger basis: the tap-publisher App is single-purpose and
only ever writes to `homebrew-tap`, so its audit trail (App-slug-attributed commits,
`contents: write` scoped to one repository) is narrower and easier to attribute than the
shared-App design it replaced.

## Threat register

| ID | Category | Severity | Disposition | Status | Evidence |
|---|---|---|---|---|---|
| T-01-01 | Tampering | high | mitigate | CLOSED | `.goreleaser.yaml`: `if OS.mac?` (143) < `xattr` strip (144) < version-json assertion (157); exactly one `xattr` call. Ordering verified by statement position, not by the comment describing it. |
| T-01-02 | Spoofing | medium | accept | CLOSED | `checksum: name_template: checksums.txt` present; signing/notarizing out of scope at milestone level. |
| T-01-03 | Tampering | low | mitigate | CLOSED | `cmd/engram/version.go:68` routes `--output` through `config.ValidateOutputFormat`, rejecting via `usageErrorf` → `exitUsage`. |
| T-01-04 | DoS | low | accept | CLOSED | Both binary invocations local, bounded, under Homebrew's installer. |
| T-01-05 | Tampering | medium | mitigate | CLOSED | `buildversion_test.go:169` `TestLastReleaseMatchesManifest`; `release-please-config.json:27` extra-files entry present. No fallback defaulting on either side. |
| T-01-06 | Spoofing | low | accept | CLOSED | `resolvedVersion()` step 1 returns the ldflags value unchanged on release builds; never an authz input. |
| T-01-07 | Info Disclosure | low | accept | CLOSED | `serve.go:87,235,300` call `resolvedVersion()`; public-repo commit hash + `.dirty` carry no secret. |
| **T-01-08** | Elevation of Privilege | **high** | mitigate | **CLOSED** | Rewritten — see above. Verified against the pinned action's compiled source. |
| T-01-09 | Tampering | high | mitigate | CLOSED | `release.yaml`: checkout (98) < upload guard (152) < goreleaser (182); both `SKIP_HOMEBREW_UPLOAD` branches present exactly once. Guard computes before GoReleaser runs. |
| T-01-10 | Info Disclosure | medium | mitigate | CLOSED | `HOMEBREW_TAP_TOKEN`/`GITHUB_TOKEN` via `env:` (`release.yaml:187,191`); `GH_TOKEN` via `env:` (`verify-tap-credential.yaml:32-33`). Never a command-line argument. |
| T-01-11 | Tampering | medium | mitigate | CLOSED | `verify-tap-credential.yaml:36` is a bare `gh api … --jq '.permissions.push'` GET; no write verbs, no cleanup path. Baseline SHA `969aef42…` recorded for the no-write comparison. |
| **T-01-12** | Repudiation | low | accept | **CLOSED** | Rationale rewritten — see above. |
| T-01-13 | DoS | low | accept | CLOSED | `release.yaml:159-162` exits 1 with `::error::` only when no `v*` tag resolves; no silent default to upload. |
| T-01-SC ×3 | Tampering | low | accept | CLOSED | No package-manager install task in any plan; stdlib only. `actions/create-github-app-token` pinned by SHA in both mint steps. |

## Unregistered flags

None. No `## Threat Flags` section exists in any SUMMARY.md.

The credential redesign introduced new *mechanism* (a second App, `HOMEBREW_TAP_TOKEN`, the
`repository.token` override) but no new *threat category* — it is the same shape already
covered by T-01-08/T-01-10/T-01-11, a token crossing a repository boundary. Coverage was
re-verified against the new mechanism rather than assumed.

## Residual, non-blocking

The original design's grant of `seanb4t/homebrew-tap` to the release App was reported revoked
by the maintainer but **cannot be verified by any CLI** — `gh api /user/installations` returns
403 for a PAT. Tracked on issue #514, item 3.

This does not reopen T-01-08. The release App's mint requests no `repositories:` or `owner:`
input at all, so even if the installation-level grant were still present, that code path
cannot produce a token scoped beyond `engram`. The mitigation holds from configuration rather
than from account state, which is the durable form. The revocation is defense-in-depth
cleanup, not a live code gap.

## Also confirmed

`generate_completions_from_executable` — flagged in `01-VERIFICATION.md` as leaking into a
`.goreleaser.yaml` comment and defeating the plan's zero-occurrence acceptance criterion — was
fixed in `49e64913` and is now absent from the file.
