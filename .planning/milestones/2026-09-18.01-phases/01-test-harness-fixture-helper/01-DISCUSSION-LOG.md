# Phase 1: Test Harness & Fixture Helper - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-18
**Phase:** 01-test-harness-fixture-helper
**Areas discussed:** Constructor home & shape, Named receive limit, Fixture helper API, Convergence scope & CI

---

## Framing (user clarification before Area 1)

The first round of constructor questions was rejected for clarification. **User:** "I want
idiomatic recommendations. Do not avoid / downplay options that are correct aside from introducing
new things. We aren't afraid of work, we're afraid of going against the grain, reinventing wheels,
and impacting future maintenance." Questions were re-framed on idiom/maintenance; stored as engram
memory `1w3h5sy56m`.

## Constructor home & shape

| Option | Description | Selected |
|--------|-------------|----------|
| Export from internal/store | Constructor in the package that owns the Qdrant client; tools.go and tests call it | ✓ |
| Dedicated internal/qdrantclient | Single-responsibility dial package; third client-holder allowlist entry | |
| Test-only mirror | Copy prod options in test code — parity by convention, drifts | |

| Option | Description | Selected |
|--------|-------------|----------|
| Pass-through ...grpc.DialOption | Upstream vocabulary, same shape as qdrant.Config.GrpcOptions; test dial requires a limit | ✓ |
| Named option WithMaxRecvBytes(n) | Matches store.Option style but wraps grpc's own option | |
| Required positional param | Forces production to name a limit in Phase 1 | |

| Option | Description | Selected |
|--------|-------------|----------|
| No change until Phase 5 | Backstop lands after regression tests prove the fix | ✓ |
| Pin 4 MiB explicitly now | Equal to grpc default; pre-empts Phase 5's one-place decision | |

**User's choice:** all recommended.

## Named receive limit

| Option | Description | Selected |
|--------|-------------|----------|
| 4 MiB | Production's effective ceiling and #583's value; Phase 3 budget stays a simple constant | ✓ |
| Smaller, e.g. 1 MiB | Smaller fixtures; forces Phase 3 budget to derive from client limit now | |
| Each test picks its own | Flexible, no convention | |

| Option | Description | Selected |
|--------|-------------|----------|
| Derive from limit + margin | Helper computes n/size (≥1.25×), self-asserts logical bytes > limit | ✓ |
| Fixed constants per shape | Hard-coded n × bytes, hand-edited on limit change | |

## Fixture helper API

Surfaced constraint: with the constructor in `store`, a harness that dials through it imports
`store`, so `store`'s in-package tests cannot import it (cycle). Go's sanctioned fix is the
external test package; repo precedent `spine_forgery_test.go`.

| Option | Description | Selected |
|--------|-------------|----------|
| internal/store/storetest + store_test | httptest idiom; oversized regression tests are black-box package store_test | ✓ |
| Leaf pkg for constructor | Move NewQdrantClient below store to break the cycle (reverses Area 1) | |
| Harness with write callback | Sizing-only harness; callers rebuild Memory; dial still cycles | |

Locked without asking (only correct choice): seeding through public `Store.Upsert`; cleanup through
public `Store.DeleteAll` (not the test-only `DeleteAllRaw`); `storetest` joins the client-holder
allowlist.

## Convergence scope & CI

| Option | Description | Selected |
|--------|-------------|----------|
| Full: clients + TestMain + tag | 11 client sites + 4 TestMain / 3 requireQdrant / 4 image-tag copies into storetest | ✓ |
| Clients only | Literal REQ; duplication remains | |

| Option | Description | Selected |
|--------|-------------|----------|
| AST gate | qdrant.NewClient only inside store.NewQdrantClient, scanning _test.go too | ✓ |
| Convention + review | How #583's precedent drifted into #585 | |

| Option | Description | Selected |
|--------|-------------|----------|
| Seeder skips in -short; no t.Parallel | One skip site; ≈ one fixture per package at peak | ✓ |
| Per-test skip, parallel allowed | Flexible; raises CI pressure risk | |

## Claude's Discretion

- storetest API names/signatures and seeder return shape
- Exact margin and per-shape record sizes within D-05
- e2e / retrievaleval adaptation to storetest.Main
- Phase 1 red-evidence patches and their redEvidenceDirs registration

## Deferred Ideas

- Single-record-larger-than-limit fixture shape → Phase 3 (content cap)
- Byte budget derived from client limit → Phase 3
- REQ-ci-store-green closing evidence / #497 → Phase 5
