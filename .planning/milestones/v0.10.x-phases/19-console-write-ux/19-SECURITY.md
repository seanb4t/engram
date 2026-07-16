---
phase: 19
slug: console-write-ux
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
block_on: high
created: 2026-07-15
---

# Phase 19 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Verified retroactively against implemented code (not documentation/intent) per
`/gsd-secure-phase`. Threat register sourced from the `<threat_model>` blocks
authored at plan time in `19-01-PLAN.md` .. `19-06-PLAN.md`. No new threats
were scanned for — this audit verifies each declared mitigation exists in the
cited implementation files.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| canonical `gen/ts` → vendored `ui/gen` | A stale/hand-edited vendored copy can silently desync the console client from the wire contract | Generated RPC/message TS bindings |
| build tooling → committed artifact | Generated code (gen client, embedded SPA) is committed and CI-checked; a bypass ships an untested/stale client | Build output |
| browser JS → same-origin Connect API | Untrusted client attaches `X-CSRF-Token`; server (`connectcsrf.go`) is the authoritative verifier | CSRF token, session cookie |
| `engram_csrf` cookie (non-HttpOnly) → JS | Cookie is deliberately JS-readable so the SPA can echo it (double-submit design) | CSRF token value |
| operator intent → destructive action | Delete and private→shared are irreversible/high-consequence; the UI must require explicit acknowledgement | Delete/share confirmation |
| optimistic client cache → displayed state | Cache is mutated before the server confirms; a failed write must not leave stale state | Cached memory/discovery records |
| client mutation input → server write RPC | Client field checks are UX-only; server protovalidate is the authoritative validator | Write RPC request payloads |
| in-flight form `$state` → re-auth event | A hard-fail must not drop the operator's typed input | Form field values, `sessionStorage` resume envelope |
| operator visibility choice → shared write | private→shared is one-way; the form must gate it behind acknowledgement | Visibility intent |
| console UI → server authz | The console can only invoke write RPCs; all authz/rule-immutability enforced server-side (Phase 17) | RPC calls |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-19-01 | Tampering | `ui/src/lib/gen/` (hand-edited/stale/uncompilable vendored client) | medium | mitigate | Structure-preserving `cp -R` re-vendor via `task proto:gen` + CI `git diff --exit-code -- ui/src/lib/gen/` drift guard + `pnpm check` compile gate | closed |
| T-19-02 | Tampering | Missing `--destructive` token / `destructive` Button variant hides or blocks a destructive affordance | low | mitigate | `--destructive`/`--destructive-foreground` tokens (per-theme) + real `destructive` variant in `button.svelte` | closed |
| T-19-11 | Tampering | `retryOnce` double-submitting a non-idempotent create (StoreMemory/StoreDiscovery) | **high** | mitigate | Retry only fires on Unauthenticated/PermissionDenied, both rejected pre-handler by session Resolve + CSRF verify; `connecterror.go` never maps a business error to those two codes; exactly one retry | closed |
| T-19-12 | Information Disclosure | `engram_csrf` cookie readable by page JS (XSS could exfiltrate it) | medium | accept | Non-HttpOnly required by double-submit design; per-request, never cached; XSS-hardening out of scope (server-owned) | closed (accepted risk logged) |
| T-19-13 | Spoofing | Missing/forged CSRF header on a write | low | mitigate | `attachCsrf` echoes the server-minted cookie as `X-CSRF-Token`; server (`connectcsrf.go`) independently re-derives and verifies, binds to `Subject.Owner()` | closed |
| T-19-31 | Tampering | Accidental private→shared (one-way disclosure) | medium | mitigate | `ShareWarningInline` requires explicit "Share anyway" ack with accurate copy before the `shared` intent is set | closed |
| T-19-32 | Tampering | Accidental/double-fired delete | medium | mitigate | `DeleteConfirmDialog` host-authoritative closure + pending state disabling Delete/Cancel while `onconfirm` in flight | closed |
| T-19-33 | Elevation of Privilege | Rule write surface appearing in the console | low | mitigate | Mechanical `memory.category === 'rule'` suppression in `MemoryRow` and `MemoryDetail` | closed |
| T-19-34 | Repudiation | Invalid nested-interactive DOM (button-in-button) | low | mitigate | `MemoryRow` root restructured to a non-button `<div>`; kebab dropdown trigger is a sibling, not nested | closed |
| T-19-41 | Tampering | Optimistic cache showing a write that never landed | medium | mitigate | `onMutate` `getQueriesData` snapshot + `onError` `setQueriesData` rollback across all matching keys; create is invalidate-only | closed |
| T-19-42 | Repudiation | `onError` swallowing the auth error, hiding terminal failure from the form | medium | mitigate | Mutation `onError` performs rollback + toast but does not consume the error; caller's own `onError` still fires | closed |
| T-19-43 | Information Disclosure | Create-as-shared/schedule-as-shared silently landing PRIVATE | medium | mitigate | Explicit Store*/Schedule*→SetVisibility(SHARED) composite for both create and schedule paths; discriminated result never falsely reports shared | closed |
| T-19-44 | Elevation of Privilege | Client-side validation mistaken for a security boundary | low | mitigate | Source comments state client checks are UX-only; server protovalidate is authoritative | closed |
| T-19-45 | Tampering | Composite partial-success duplicating a create (secondary SetVisibility auth failure rethrown into whole-create resubmit) | **high** | mitigate | `shareIfRequested`/`createDiscoveryComposite` catch every secondary SetVisibility failure (incl. Unauthenticated/PermissionDenied) and return `created_private`, never rethrow | closed |
| T-19-51 | Information Disclosure | Accidental private→shared, accidental unshare in edit, or create landing private while believed shared | medium | mitigate | Embedded `ShareWarningInline` gate; edit-mode already-shared record is READ-ONLY (`isEditSharedReadOnly`), `shared` never enters dirty mask as `false` | closed |
| T-19-52 | Denial of Service (UX) / data loss | Losing typed input on session expiry or full re-auth redirect; resume-envelope race | medium | mitigate | Two-tier D-09: in-SPA sheet stays open + `$state` preserved; single-owner versioned/TTL `resume.ts` envelope, form persists only, route peeks/consumes after ack | closed |
| T-19-53 | Information Disclosure | Resume-envelope data persisting/tampering/open-redirect-shaped `returnPath` | low | mitigate | Version+TTL+shape-validated envelope; `normalizeReturnPath` (no `/ui/ui/`); `isAllowedDestination` allowlist (`/observe`,`/search`,`/discovery`); sole route-owned consume/delete | closed |
| T-19-61 | Tampering | Accidental delete via row/detail action | medium | mitigate | Delete callback routes through `DeleteConfirmDialog` before any delete hook fires | closed |
| T-19-62 | Information Disclosure | Accidental private→shared via row/detail share action | medium | mitigate | Share callback routes through `ShareWarningInline` acknowledgement before set-visibility fires | closed |
| T-19-63 | Elevation of Privilege | A rule write entry point slipping into the console | low | mitigate | `WriteSurfaces`/routes render no rule affordance; discovery route passes no `onedit` (D-04) | closed |
| T-19-64 | Tampering | Stale optimistic state after a failed write (incl. foreign shared record) | medium | mitigate | Plan-04 hooks roll back on error; routes add no bypassing refetch; foreign-record rejection surfaces as rollback + `write failed` | closed |
| T-19-65 | Tampering | Shipped binary silently not containing the write UX (stale embedded SPA) | **high** | mitigate | `task ui:build` rebuild committed under `internal/webauth/static/` (clean `git status`); CI `ui-drift` job independently rebuilds + `git diff --exit-code` | closed |
| T-19-66 | Tampering | Row/search edit prefilling summary-shaped (content-cleared) data → Save overwrites body with empty content | **high** | mitigate | `WriteSurfaces.openEdit(id)` refetches FULL record via `GetMemory` (`queryClient.fetchQuery(['getMemory', id])`) before opening the sheet; never prefills from a list/search row | closed |
| T-19-67 | Information Disclosure | Re-share warning shown for already-shared record; share intent lost across re-auth; resume-envelope race/tamper | medium | mitigate | `requestShare(memory, kind)` reads visibility from the passed `Memory` and no-ops when shared; single-owner `resume.ts` envelope (see T-19-52/53) | closed |
| T-19-68 | Denial of Service (UX) | Inline delete/share terminal auth failure dead-ends at generic toast with no re-auth path, target silently cleared/hidden | medium | mitigate | `WriteSurfaces` awaits settlement; host-authoritative `DeleteConfirmDialog`/`ShareWarningInline` closure; terminal auth RETAINS target + shows re-auth CTA; no auto-replay | closed |
| T-19-SC | Tampering | npm/pip/cargo installs (declared identically in all 6 plan registers: 19-01..19-06) | low | accept | No new packages installed during Phase 19 (verified: `git log` on `ui/package.json` across the phase's commit range shows only `chore(deps)` version-bump commits, no new dependency additions; `dompurify`/`marked` predate this phase, added in PR #221) | closed (accepted risk logged) |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` (block_on) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

**High-severity threats (the blocking gate at `block_on: high`): T-19-11, T-19-45, T-19-65, T-19-66 — all verified CLOSED with direct code evidence (see Audit Trail below). `threats_open: 0`.**

---

## Evidence Detail (selected — full trace)

- **T-19-11 / T-19-45 (double-create safety):**
  - `ui/src/lib/interceptors/retryOnce.ts:9,32-39` — retry set is exactly `{Code.Unauthenticated, Code.PermissionDenied}`, one retry.
  - `internal/server/connectauth.go:18-28` — subject resolution failure → `CodeUnauthenticated`, pre-handler.
  - `internal/server/connectcsrf.go:58-91` — CSRF interceptor → `CodePermissionDenied` on any mismatch, pre-handler (runs before validate/handler).
  - `internal/server/connecterror.go:45-68` — the *only* production error mapper; never emits `CodeUnauthenticated`/`CodePermissionDenied` from a business error (only NotFound/InvalidArgument/FailedPrecondition/Canceled/DeadlineExceeded/Internal) — confirms those two codes can only originate pre-handler, so a retry cannot re-fire a mutation.
  - `ui/src/lib/mutations/memory.ts:125-137` (`shareIfRequested`) and `ui/src/lib/mutations/discovery.ts:75-87` (`createDiscoveryComposite`) — secondary `SetVisibility` failure is caught in a bare `try/catch`, returns `{status:'created_private', id}`, never rethrown. Both `createMemoryComposite`/`scheduleMemoryComposite` (memory.ts:139-150) route through the same `shareIfRequested`.
  - `ui/src/lib/components/MemoryFormSheet.svelte:172-195` (`handleWriteSuccess`/`handleWriteError`) — all three composite statuses (`created`/`created_shared`/`created_private`) resolve via `handleWriteSuccess`; only a *rejected* primary-call promise reaches the D-09 hard-auth resubmit branch.
- **T-19-65 (shipped binary):** `git status --porcelain -- internal/webauth/static/` clean; `git log` shows `2143d4a7 chore(19-06): rebuild embedded SPA with the write UX (task ui:build)` and a later `398f9b70 chore(19): rebuild embedded SPA after code-review fixes`; `.github/workflows/ci.yaml:143-164` (`ui-drift` job) independently rebuilds and asserts `git diff --exit-code`.
- **T-19-66 (full-content edit prefill):** `ui/src/lib/components/WriteSurfaces.svelte:96-111` (`openEdit`) — `await queryClient.fetchQuery({queryKey:['getMemory', id], queryFn: () => engram.getMemory({id})})` runs and resolves *before* `sheetOpen = true`; on fetch error it toasts and never opens a blank/stale edit.
- **Interceptor order (`SC2`/`SC3` transport prerequisite):** `ui/src/lib/client.ts:21-26` — `writeTransport` interceptors literally `[retryOnce, attachCsrf]`; `attachCsrf` (`csrf.ts:14-21`) re-reads `document.cookie` fresh on every `next(req)` call, so a retry re-enters it with the current cookie.

## Cross-referenced (independently verified, not re-derived)

`.planning/phases/19-console-write-ux/19-REVIEW.md` (iteration 3, `status: clean`, 0 findings) independently re-verified 5 invariants against live source, all consistent with this audit's findings: interceptor order, no-duplicate-create composite, host-authoritative `DeleteConfirmDialog`, single-owner resume envelope, edit-visibility never emits `shared:false`. One accepted wontfix noted there (WR-03: inline delete/share re-auth lands on `/ui/` rather than the originating route — a deliberate round-5 scope decision, not a threat-register item; consistent with T-19-68's documented in-SPA-only retention scope).

The `MemoryDetail.svelte` `{@html}` sink (`ui/src/lib/markdown.ts`) is fed through `marked` → `DOMPurify.sanitize` with a tight tag/attribute allowlist and a safe-scheme (`https?`/`mailto`) href hook — confirmed present, though it is pre-existing infrastructure from PR #221, not a Phase 19 threat-register entry, so it is not scored as a phase-19 threat.

---

## Unregistered Flags (SUMMARY.md `## Threat Flags`)

None. No `## Threat Flags` section exists in any of `19-01-SUMMARY.md` .. `19-06-SUMMARY.md` (checked directly — no new attack surface was flagged by the executor during implementation beyond the plan-time register).

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-19-01 | T-19-12 | `engram_csrf` cookie is deliberately non-HttpOnly (required by the double-submit CSRF design so the SPA can echo it as `X-CSRF-Token`); an XSS on the console would be able to read it, but XSS-hardening is server-owned and out of this phase's scope. Mitigating factors: token is bound to `Owner` only (`internal/webauth/csrf.go:39,58`), short-lived relative to session, and never cached client-side beyond a single request (`csrf.ts` re-reads per call). | Phase 19 plan (19-02-PLAN.md, disposition: accept) | 2026-07-15 |
| AR-19-02 | T-19-SC | No new npm/pip/cargo packages were installed during Phase 19 (verified via `ui/package.json` git history across the phase's commit range — all `chore(deps)` entries are pre-existing-dependency version bumps, no additions). Declared identically across all 6 plan threat registers. | Phase 19 plans (19-01..19-06-PLAN.md, disposition: accept) | 2026-07-15 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-15 | 26 | 26 | 0 | gsd-security-auditor (retroactive verification, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-15
