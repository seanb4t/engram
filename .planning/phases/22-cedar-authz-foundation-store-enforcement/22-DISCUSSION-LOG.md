# Phase 22: Cedar Authz Foundation & Store Enforcement - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-17
**Phase:** 22-cedar-authz-foundation-store-enforcement
**Mode:** `--all --auto --chain` — all gray areas auto-selected; every question resolved to
the recommended default (grounded in `.planning/research/CEDAR.md` + locked ADRs), no user
prompts.
**Areas discussed:** PDP plumbing, decision granularity, entity schema & actions, policy
corpus & testing, policy delivery scope, deny mapping & diagnostics

---

## PDP plumbing

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit Store dependency | PDP constructed at startup, injected into Store (DEC-c0m precedent); consulted only from store | ✓ |
| Package-level global | `internal/authz` singleton initialized from go:embed at package init | |
| Handler-level consultation | Handlers ask the PDP, store just filters | |

**Choice:** Explicit dependency (recommended). Handler-level rejected outright — it would
violate DEC-cgb rather than refine it.

## Decision granularity

| Option | Description | Selected |
|--------|-------------|----------|
| Split by path shape | Buckets for bulk recall (Decide per bucket per request); per-record Authorize on id-addressed gates | ✓ |
| Buckets everywhere | Id-addressed gates also reduced to bucket checks | |
| Per-record everywhere | cedar.Authorize per candidate point | |

**Choice:** Split (CEDAR.md's explicit recommendation). Per-record-everywhere is one of the
two documented rejected approaches (O(records) on hot path, Pitfall-12 shape).

## Entity schema & actions

| Option | Description | Selected |
|--------|-------------|----------|
| Full verb set, one Principal | read/write/delete/share/schedule day one; `kind` attr; tenant/roles reserved-optional | ✓ |
| Coarse read/write only | Two actions, refine later | |
| User vs ServicePrincipal entity types | Separate entity types per caller class | |

**Choice:** Full verb set + single Principal (CEDAR.md schema sketch verbatim; honors
DEC-12c and the "no 3rd Subject variant" anti-pattern).

## Policy corpus & testing

| Option | Description | Selected |
|--------|-------------|----------|
| 4 policies + policy-text regression tests | own, shared-read, tenant-isolate (vacuous), forbid-empty-owner; CI tests evaluate the embedded corpus | ✓ |
| 3 policies only | Skip the defense-in-depth empty-owner forbid | |

**Choice:** 4 policies (ROADMAP success criterion 2 names the forbid policy explicitly;
double-blocks the #1 milestone risk).

## Policy delivery scope

| Option | Description | Selected |
|--------|-------------|----------|
| Embedded-only this phase | No ENGRAM_AUTHZ_POLICY_DIR / hot-reload | ✓ |
| Ship policy_dir override now | CEDAR.md build-order step 4 | |

**Choice:** Embedded-only — REQUIREMENTS.md's Deferred section explicitly pushes the
override path to a later milestone; requirements precedence over the research suggestion.

## Deny mapping & diagnostics

| Option | Description | Selected |
|--------|-------------|----------|
| ErrNotFound + debug-only Diagnostic | Deny ≡ missing id (DEC-xa6); Diagnostic to slog debug/span events under DEC-wot PII rules | ✓ |
| Distinct 403-shaped error | Reveal authz denial to callers | |

**Choice:** ErrNotFound (locked by DEC-xa6 and ROADMAP success criterion 4 — the alternative
was never viable; logged for completeness).

## Claude's Discretion

- Exact `internal/authz` Go API/type shapes, package file layout, test organization.
- PDP injection mechanics (functional option vs constructor param).
- How the two rejected approaches are guarded against reinvention (comments vs tests).

## Deferred Ideas

- OIDC client-credentials owner-claim source → Phase 23.
- `shared` cross-tenant visibility policy → Phase 23/#373.
- `ENGRAM_AUTHZ_POLICY_DIR` + fatal malformed-policy guard + Helm mount; hot-reload;
  tenant/group/role population — future milestones.
