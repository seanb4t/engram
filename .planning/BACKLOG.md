# engram — Backlog

**Purpose:** a reconciled index of open, un-milestoned work so `/gsd-new-milestone` and
`/gsd-review-backlog` can see promotion candidates in one place.

> **GitHub Issues is the source of truth for status.** This file is an index, not a tracker.
> It carries no checkboxes by design — the two prior revisions did, and both drifted badly
> (see the 2026-07-29 review below). Regenerate it from `gh issue list` rather than editing
> status by hand.

**Origin:** engram moved off the beads issue tracker on 2026-07-08. The full historical export
(1,037 issues incl. 1,017 closed + the migrated `bd remember` memories) is archived at
`.planning/archive/beads-export-engram-2026-07-08.jsonl`. The `from-beads` label marks the
migrated items; as of 2026-07-29 none remain open.

---

## Promotion candidates — 18 open issues (2026-07-29)

No active milestone. v0.11.x closed and archived 2026-07-27; `REQUIREMENTS.md` is absent.
**Nothing can be promoted until `/gsd-new-milestone` defines a target.** These are the
candidates it should consider.

### Feature ideas (2)

- [#343](https://github.com/seanb4t/engram/issues/343) — cli: headless client transport (search/store/list) for agents without MCP access
- [#344](https://github.com/seanb4t/engram/issues/344) — tools: `cross_spine` support on `search_memory` (parity with `search_discovery`)

### Partially shipped — remainder open (2)

- [#350](https://github.com/seanb4t/engram/issues/350) — distinct base URLs for embedder vs chat. **Base URL split shipped** in Phase 26 (`ENGRAM_OPENAI_CHAT_BASE_URL`); the *distinct API key* half was not. Documented as a known constraint.
- [#394](https://github.com/seanb4t/engram/issues/394) — Cedar PDP follow-ups. **WR-01 (DeleteAll via PDP) and WR-02 (deny-mapping test) both shipped** in Phase 22; only IN-01 (wire `Decision.diag` to debug logging) remains, deliberately deferred until operator diagnostics land.

### Phase review follow-ups (8)

- [#345](https://github.com/seanb4t/engram/issues/345) `bug` — reindex `--resume` skips tag-only changes → stale vector (Phase 13 WR-01)
- [#346](https://github.com/seanb4t/engram/issues/346) — base-URL join edge cases (Phase 13 WR-02). **Note:** this is a *deliberate* non-fix — Phase 13 left query/fragment joins non-canonicalizing as operator-error scope, and Phase 26's `TestJoin` pins that behavior. Consider closing with that rationale rather than promoting.
- [#347](https://github.com/seanb4t/engram/issues/347) — embed client discards provider error body on non-2xx (Phase 13 WR-03)
- [#353](https://github.com/seanb4t/engram/issues/353) `bug` — eval differ: `reflect.DeepEqual` on `[]float32` can false-PASS on float jitter (Phase 14 WR-01)
- [#354](https://github.com/seanb4t/engram/issues/354) `bug` — eval differ skip guard reads `os.Getenv`, not resolved koanf config (Phase 14 WR-02)
- [#355](https://github.com/seanb4t/engram/issues/355) — Phase 14 docs/comment nits (IN-01/IN-02)
- [#357](https://github.com/seanb4t/engram/issues/357) — Phase 15: strengthen write-RPC validation test coverage (WR-01/WR-02/IN-01/IN-02)
- [#358](https://github.com/seanb4t/engram/issues/358) — Phase 15: de-duplicate + harden the idempotency-level ban gate (WR-03)

### Bugs / correctness (2)

- [#360](https://github.com/seanb4t/engram/issues/360) `bug` — over-long `summary` reports the misleading `missing properties: ["content"]`
- [#370](https://github.com/seanb4t/engram/issues/370) `bug` — `task lint:yaml` fails on `Taskfile.yaml` while CI is green. **Note:** `yamlfmt -lint Taskfile.yaml` passes as of 2026-07-29 — likely resolved, verify on the reporter's setup before closing.

### Infrastructure / tooling (3)

- [#356](https://github.com/seanb4t/engram/issues/356) — sync committed UI TS types with proto; extend CI drift check beyond `gen/`
- [#366](https://github.com/seanb4t/engram/issues/366) — full-stack e2e harness for console write flows (compose + mock OIDC + Playwright). Explicitly deferred in v0.10.x.
- [#393](https://github.com/seanb4t/engram/issues/393) — `ui/` postUpgradeTask build OOMs the shared Renovate pod; cap build heap so it fails loud

### Open questions (1)

- [#351](https://github.com/seanb4t/engram/issues/351) — investigate why the skills aren't pushing for rule capture

_(#155 Dependency Dashboard is a Renovate bot artifact — not tracked here.)_

---

## Review log

### 2026-07-29 (`/gsd-review-backlog`)

Reviewed after v0.11.x shipped and archived. **This file was rewritten** — the prior revision's
checkboxes contradicted GitHub on 14 issues, despite its own header warning that status tracked
here would drift.

**Promoted: 0.** There is no active milestone to promote into — v0.11.x closed 2026-07-27 and no
successor is scoped. Promotion has to wait for `/gsd-new-milestone`.

**Closed — 9 `from-beads` items, as stale:** #306, #309, #310, #312, #313, #315, #316, #318, #319.
These went through **two** promotion cycles without being worked: marked `[x] → v0.9.x` on
2026-07-09, caught as falsely-marked and re-promoted to `v0.10.x` on 2026-07-11, then missed by
both v0.10.x and v0.11.x. Two milestones, two explicit promotions, zero delivery — a signal about
real priority rather than an oversight. All were reviewer polish suggestions (helper extraction,
naming, redundant reads, test qualification), none correctness-affecting. Reopenable if the
relevant code is touched.

**Closed earlier the same day via `/gsd-inbox`** — 6 issues delivered by v0.11.x: #340
(idempotency), #341 (citations), #342 (supersession), #362 (service token auth), #373 (tenancy
isolation), #374 (category filter).

**Corrected drift:** #311 and #314 were listed as open maintenance but are closed; #340/#341/#342
were listed as v0.11.x candidates but had shipped; #345/#346/#347 and the nine from-beads items
were listed as "Promoted → v0.10.x" but never landed.

Result: 34 open issues → 19.

### 2026-07-11 (`/gsd-review-backlog`)

Reviewed after Phase 13 (Embedder Reliability Foundation) merged (PR #348).

- **Closed — delivered by Phase 13:** #333 (`ENGRAM_EMBED_TIMEOUT`), #332 (shape-aware base-URL join). #332's operator-error edge cases live on in #346.
- **Promoted → v0.10.x (12):** #345, #346, #347 plus the nine from-beads leftovers. *(Retrospect: none of the twelve landed in v0.10.x. The nine were closed as stale on 2026-07-29; #345/#346/#347 remain open.)*
- **Deferred → v0.11.x candidates (5):** #340, #341, #342, #343, #344. *(Retrospect: #340/#341/#342 shipped in v0.11.x; #343/#344 remain open.)*
- **Removed:** none.

### 2026-07-09 (`/gsd-review-backlog`)

Reviewed after v0.8.4 shipped (milestone v0.8.x complete).

- **Promoted → `v0.9.x` (21):** clusters B (embedder/ops), C (short_id polish), D (design/quality), plus #322 (Connect write-lane RPCs + CSRF) and #323 (session rotation).
- **Maintenance, outside any milestone:** #314, #311, #301. *(All three since closed.)*
- **Removed:** none.
- *(Retrospect: this review's `[x]` marks were unreliable — the 2026-07-11 review found nine items marked promoted that had never been worked.)*
