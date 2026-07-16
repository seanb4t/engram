# Phase 21: CI / Maintenance Hygiene - Pattern Map

**Mapped:** 2026-07-15
**Files analyzed:** 6 modified (no new files, no new packages)
**Analogs found:** 6 / 6 — all in-repo, no external patterns needed

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `.rumdl.toml` | config | transform (lint exclude list) | itself — neighboring `exclude` entries (`.beads`, `docs-site`) | exact |
| `.github/workflows/ci.yaml` (`ui-drift` job) | CI workflow step | event-driven (PR push, self-heal commit) | `.github/workflows/release.yaml` (`release` job's App-token mint step) | exact |
| `internal/server/summaryqueue.go` | service (in-process worker pool) | event-driven / queue | itself — `usageQueue`'s mirrored kernel confirms the target shape | exact (self + sibling) |
| `internal/server/usagequeue.go` | service (in-process worker pool) | event-driven / queue | `summaryqueue.go` — verbatim mirror of the same kernel | exact |
| `internal/server/tools.go` (`storeMemory`/`scheduleMemory`) | controller/handler | CRUD (write path) | itself — the two duplicated call sites are each other's analog | exact |
| `internal/server/tools_test.go` (`TestBuildDepsFromEnvLoadsConfigOnce`) | test | request-response (unit) | `internal/config/config_test.go`'s `TestLoadDefaults` | exact |

## Pattern Assignments

### `.rumdl.toml` (config)

**Analog:** itself, lines 17-29 (the `exclude` array)

```toml
exclude = [
  ".git",
  ".worktrees",
  ".beads", # beads-managed (README.md, generated config)
  ".agents", # agent-tool-managed skill mirrors
  ".claude",
  ".codex",
  "dist",
  "node_modules",
  "vendor",
  "CHANGELOG.md", # generated + owned by release-please (re-dirtied each release; MD012)
  "docs-site", # Astro/Starlight site — MDX + generated output, not plain prose
]
```

**Convention to copy exactly (D-09):**
- Plain directory name, no glob (`.planning`, not `.planning/**`) — matches every existing entry; no `**` form appears anywhere in the array.
- Trailing `# why` comment on the same line, present on most entries (`.beads`, `.agents`, `CHANGELOG.md`, `docs-site`) — match that density/style, e.g.:
  ```toml
  ".planning", # GSD planning artifacts: agent-generated, not shipped prose
  ```
- Insertion point: append at the end of the array (after `"docs-site"`), consistent with how each new entry has been appended chronologically rather than alphabetized.
- `respect-gitignore = true` (line 31) stays untouched — `.planning/` is committed, not gitignored, which is exactly why the `exclude` array (not `.gitignore`) is the correct lever (confirmed live: `rumdl check .` → 100% of issues under `.planning/`).

---

### `.github/workflows/ci.yaml` — `ui-drift` job self-heal step

**Analog:** `.github/workflows/release.yaml` lines 1-27 (App-token mint + usage) and `ci.yaml` lines 210-217 (`commit-lint`'s job-level `permissions:` — precedent to NOT copy the escalation pattern from, per research).

**release.yaml App-token pattern (verbatim, lines 1-27):**
```yaml
name: release

on:
  push:
    branches: [main]

# The release-please GitHub App token (minted below) performs every release-PR /
# tag / GitHub Release write and is the named bypass actor on the protect-main
# ruleset. The default GITHUB_TOKEN only needs packages:write to push the image +
# OCI chart to GHCR.
permissions:
  contents: read

jobs:
  release:
    name: release-please + image + OCI chart
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write # push image + chart to GHCR via GITHUB_TOKEN
    steps:
      - uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3
        id: app-token
        with:
          app-id: ${{ secrets.RELEASE_APP }}
          private-key: ${{ secrets.RELEASE_APP_PRIVATE_KEY }}
      - uses: googleapis/release-please-action@45996ed1f6d02564a971a2fa1b5860e934307cf7 # v5.0.0
        id: release
        with:
          token: ${{ steps.app-token.outputs.token }}
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
```

**Key facts the planner must copy, not reinvent:**
1. **Pinned SHA + version comment**: `actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3` — same pin research recommends reusing verbatim for the new `ui-drift` step (already vetted, already in production in this repo).
2. **Secret/var names already claimed by release.yaml**: `secrets.RELEASE_APP` (App ID, via legacy `app-id:` input) and `secrets.RELEASE_APP_PRIVATE_KEY`. **Do not reuse these** — the new App must be purpose-scoped (`Contents: Read & write` only) and use its own secret names (research suggests `vars.CI_BOT_APP_CLIENT_ID` / `secrets.CI_BOT_APP_PRIVATE_KEY`, using the current `client-id:` input rather than the legacy `app-id:` release.yaml uses — style choice per research, either works). A human must create/install this App and add the secrets before the guarded push path is live — this is a `checkpoint:human-verify`, not something the plan can complete unattended.
3. **No job-level `permissions: contents: write` needed.** release.yaml's `release` job declares only `contents: read, packages: write` (no `contents: write`) yet successfully pushes tags/releases via the App token — the App's own installation permissions grant write, independent of the job's `permissions:` block. **Do not add `contents: write` to `ui-drift`'s job-level `permissions:`** — leave the repo-wide `permissions: contents: read` (ci.yaml lines 9-10) completely untouched. This is Pitfall 2 from RESEARCH.md — do not fall into it.
4. release.yaml has **no explicit `git config user.name/user.email` step** in the excerpt above (release-please-action handles its own git identity internally) — so there is no verbatim git-identity step to copy from release.yaml itself; the researcher's sketch (RESEARCH.md "Code Examples" section) is the reference for that piece: `git config user.name "<app-slug>[bot]"` / `git config user.email "<APP_ID>+<app-slug>[bot]@users.noreply.github.com>"`, matching the observed convention (`fzymgc-renovate[bot] <293849087+fzymgc-renovate[bot]@users.noreply.github.com>`).

**`ci.yaml`'s existing `ui-drift` job to extend (verbatim, lines 155-177):**
```yaml
  ui-drift:
    name: ui vendored-asset drift
    runs-on: ubuntu-latest
    if: ${{ !startsWith(github.head_ref, 'release-please--') }}
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7
      # `uses:` steps ignore working-directory, so point action-setup at the
      # ui packageManager field (pnpm@11) explicitly — the repo root has no
      # package.json for it to read.
      - uses: pnpm/action-setup@0ebf47130e4866e96fce0953f49152a61190b271 # v6
        with:
          package_json_file: ui/package.json
      - uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6
        with:
          node-version: '26'
          cache: pnpm
          cache-dependency-path: ui/pnpm-lock.yaml
      - run: |
          cd ui && pnpm install --frozen-lockfile && pnpm build
          rm -rf ../internal/webauth/static && mkdir -p ../internal/webauth/static
          cp -R build/. ../internal/webauth/static/
      - run: git diff --exit-code internal/webauth/static/ || { echo "::error::vendored SPA is stale — run 'task ui:build'
          and commit"; exit 1; }
```
The last `run:` step (fail-with-guidance) is the one to gate/replace with a drift-detection + guarded-branch step, per RESEARCH.md's full sketch — this excerpt is the exact current state the plan diffs against.

**`commit-lint`'s job-level `permissions:` precedent (verbatim, lines 210-217) — cited for contrast, NOT to copy the escalation shape:**
```yaml
  commit-lint:
    name: commit-lint
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    permissions:
      pull-requests: read # action-semantic-pull-request reads PR metadata via GITHUB_TOKEN
    steps:
      - uses: amannn/action-semantic-pull-request@48f256284bd46cdaab1048c3721360e808335d50 # v6.1.1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```
This is the in-repo precedent for a job-level `permissions:` block granting `GITHUB_TOKEN` a capability for a specific action — but per RESEARCH.md, this shape does **not** transfer to `ui-drift`'s push step, because that step must not use `GITHUB_TOKEN` at all (it needs a re-triggering identity). Cited here only so the planner recognizes the pattern and correctly does NOT apply it.

**Guard expression (already fully specified by research, copy verbatim):**
```yaml
if: >-
  github.event_name == 'pull_request' &&
  startsWith(github.head_ref, 'renovate/') &&
  github.actor == 'fzymgc-renovate[bot]' &&
  github.event.pull_request.head.repo.full_name == github.repository
```

---

### `internal/server/summaryqueue.go` / `usagequeue.go` — WR-03 `Wait()` relocation

**Analog:** the two files mirror each other's kernel; extracting both shows the symmetry the planner must preserve.

**Current production `Wait()` — to be DELETED from both non-test files:**

`summaryqueue.go` (verbatim):
```go
// Wait blocks until every enqueued (non-dropped) id currently in flight has
// reached a terminal fill outcome (success, exhausted retries, or a
// recovered panic) — the deterministic drain seam tests use instead of
// time.Sleep polling. Nil-safe no-op on a disabled queue.
func (q *summaryQueue) Wait() {
	if q == nil {
		return
	}
	q.inFlight.Wait()
}
```

`usagequeue.go` (verbatim — near-identical, confirming D-00c's "verbatim mirror" claim):
```go
// Wait blocks until every enqueued (non-dropped) id currently in flight has
// reached a terminal fill outcome (success, error, or a recovered panic) —
// the deterministic drain seam tests use instead of time.Sleep polling.
// Nil-safe no-op on a disabled queue.
func (q *usageQueue) Wait() {
	if q == nil {
		return
	}
	q.inFlight.Wait()
}
```

**Relocation target:** move each verbatim into a `_test.go` file in the same package (`package server`) — RESEARCH.md confirms **no `export_test.go` convention currently exists** in `internal/server/` (test files are `<subject>_test.go`, all `package server`, in-package not `package server_test`). Two viable placements, Claude's call per D-04:
- Inline into `summaryqueue_test.go` / `usagequeue_test.go` directly (lowest-friction, no new file), or
- A single new shared file, e.g. `internal/server/queue_export_test.go`, holding both methods together (keeps the "these are test-only escape hatches" grouping explicit).

Either way: **no build tag needed** — the `_test.go` suffix alone is compiler-enforced exclusion from `go build`.

**Doc-comment density/style to match (D-08/CR-01 ID-citing convention) — representative sample from `summaryqueue.go`:**
```go
// tryEnqueue is the write-path call site's entry point: nil-safe (a disabled
// queue is a no-op), non-blocking (a full queue drops and counts instead of
// stalling the caller — SC#2), and never returns an error since the caller
// must never be slowed down by this best-effort path.
func (q *summaryQueue) tryEnqueue(id string) {
	if q == nil {
		return
	}
	// RLock serializes only against Shutdown's close (concurrent enqueues still
	// proceed in parallel); it guarantees the closed-check and the send below
	// cannot straddle Shutdown's close(ch), so a late enqueue drops instead of
	// panicking on send-to-closed (CR-01).
	q.mu.RLock()
	defer q.mu.RUnlock()
	...
```
Note the citation style: `SC#2`, `CR-01` — finding/decision IDs embedded directly in prose comments, not just in commit messages. The relocated `Wait()` comment should keep its existing ID-free style (it doesn't cite one), but any NEW comment explaining *why* it moved should cite `WR-03` explicitly, matching this repo's convention of naming the finding in the comment, e.g.: `// Wait is test-only (WR-03): moved out of production reach because every...`.

**SPDX header (both files, identical, confirm on any new file too):**
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server
```

---

### `internal/server/tools.go` — IN-01 `persistAndEnqueue` extraction

**Analog:** the two duplicated call sites are each other's pattern; `storeDiscovery`'s deliberate non-enqueue is the negative-space contrast.

**`storeMemory` call site (verbatim, lines 650-667):**
```go
func (d *deps) storeMemory(ctx context.Context, c caller, a storeArgs) (string, string, error) {
	m := a.toMemory(c.Subj.Owner(), c.Actor, d.clock())
	m.EmbedderIdentity = d.embedderIdentity
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	// Enqueue only after a confirmed-successful Upsert; never blocks/errors
	// the write path even when the queue is disabled or full (SC#1, SC#2).
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}
```

**`scheduleMemory` call site (verbatim, lines 679-703) — the duplicated block is identical from `MintShortID` onward:**
```go
func (d *deps) scheduleMemory(ctx context.Context, c caller, a scheduleArgs) (string, string, error) {
	now := d.clock()
	nb, na, err := parseWindow(a, now)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(c.Subj.Owner(), c.Actor, now)
	m.NotBefore = nb
	m.NotAfter = na
	m.EmbedderIdentity = d.embedderIdentity
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	// Enqueue only after a confirmed-successful Upsert; never blocks/errors
	// the write path even when the queue is disabled or full (SC#1, SC#2).
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}
```

**Duplicated region to extract (identical in both, D-05 target):**
```go
if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
    return "", "", err
}
if err := d.st.Upsert(ctx, m, vec); err != nil {
    return "", "", err
}
// Enqueue only after a confirmed-successful Upsert; never blocks/errors
// the write path even when the queue is disabled or full (SC#1, SC#2).
d.summaryQueue.tryEnqueue(m.ID)
return m.ID, m.ShortID, nil
```
Suggested extraction (D-05, signature is Claude's call but this shape fits with no adaptation, per RESEARCH.md):
```go
func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error) {
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}
```
Both `m` and `vec` are already fully built by the time each caller reaches this point — no adaptation needed at either call site, just replace the inline block with `return d.persistAndEnqueue(ctx, m, vec)`.

**Negative-space contrast — `storeDiscovery` does NOT enqueue, and its doc-comment explains why (verbatim, lines 705-712):**
```go
// storeDiscovery persists a client-authored discovery. It deliberately never
// enqueues for async summary fill: discoveries own their own summaries (D-06
// negative space) — see TestDiscoveryAndRuleNeverEnqueue.
func (d *deps) storeDiscovery(ctx context.Context, c caller, a storeDiscoveryArgs) (string, string, error) {
	if err := validateStoreDiscovery(a); err != nil {
		return "", "", err
	}
	...
```
Confirmed via `rg "tryEnqueue|summaryQueue" internal/server/rules.go` → no matches: `storeRule` (`internal/server/rules.go:95`) is equally excluded. **Do not fold either into the new `persistAndEnqueue` helper** — only `storeMemory` and `scheduleMemory` call it.

---

### `internal/server/tools_test.go` — IN-02 test hermeticity

**Analog:** `internal/config/config_test.go`'s `TestLoadDefaults` — the env-clearing pattern to copy.

**Analog pattern (verbatim, `config_test.go:13-20`):**
```go
func TestLoadDefaults(t *testing.T) {
	// Isolate from ambient ENGRAM_* in the dev/CI shell. Empty values preserve
	// the registry default (the documented empty-env invariant), so this both
	// clears inherited overrides and keeps the assertions deterministic.
	t.Setenv("ENGRAM_OPENAI_BASE_URL", "")
	t.Setenv("ENGRAM_EMBED_MODEL", "")
	t.Setenv("ENGRAM_LISTEN_ADDR", "")
	cfg, err := Load(nil)
	...
```

**Target — current `TestBuildDepsFromEnvLoadsConfigOnce` (verbatim, `tools_test.go:1624-1640`):**
```go
func TestBuildDepsFromEnvLoadsConfigOnce(t *testing.T) {
	if testQdrantAddr == "" {
		failOrSkipNoQdrant(t)
	}
	// buildDepsFromEnv reads the data-plane config from the process env; point it
	// at the test Qdrant with a dedicated collection so EnsureCollection succeeds.
	t.Setenv("ENGRAM_QDRANT_ADDR", testQdrantAddr)
	t.Setenv("ENGRAM_QDRANT_COLLECTION", "mem_load_once_test")
	t.Setenv("ENGRAM_EMBED_DIM", "3")

	loads := 0
	orig := configLoad
	configLoad = func(flags *flag.FlagSet) (*config.Config, error) {
		loads++
		return orig(flags)
	}
	t.Cleanup(func() { configLoad = orig })

	d, err := buildDepsFromEnv(nil, nil)
	...
```
**D-06 fix (verbatim per CONTEXT.md):** add two more `t.Setenv` calls alongside the existing three, matching the config_test.go style (empty-string clears, doesn't unset):
```go
t.Setenv("ENGRAM_SUMMARY_MODEL", "")
t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")
```
Insert these next to the existing `t.Setenv("ENGRAM_QDRANT_ADDR", ...)` block (same comment block, extended) so an ambient dev/CI env can never start a real summary queue from this test (currently leaks 2 worker goroutines for the test binary's lifetime per RESEARCH.md).

## Shared Patterns

### SPDX header (all Go files)
**Source:** `internal/server/summaryqueue.go:1-2`, `internal/server/usagequeue.go:1-2` (identical across the package)
**Apply to:** any new Go file this phase creates (e.g. a `queue_export_test.go` if that placement is chosen)
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```
`task license:check` enforces this; `task license:add` applies it automatically if missed.

### Least-privilege GitHub App tokens for bot pushes
**Source:** `.github/workflows/release.yaml:1-27`
**Apply to:** `.github/workflows/ci.yaml`'s `ui-drift` self-heal step — mint via `actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3`, keep job-level `permissions:` unescalated (App token supplies write independently), use fresh purpose-scoped secret names (do not reuse `RELEASE_APP`/`RELEASE_APP_PRIVATE_KEY`).

### Nil-safe, ID-citing doc comments on queue types
**Source:** `internal/server/summaryqueue.go`, `internal/server/usagequeue.go`
**Apply to:** any relocated/new method on either queue type — comments should be dense, explain the *why* (not just what), and cite finding/decision IDs (`WR-03`, `CR-01`, `SC#1`/`SC#2`, `D-0x`) inline where the code embodies a specific reviewed decision.

## No Analog Found

None — all 6 files in scope have a strong, concrete in-repo analog (several are their own analog via the file's existing neighboring code).

## Answers to Orchestrator's Specific Questions

1. **Does release.yaml's App-token pattern transfer cleanly to ci.yaml?** Yes, structurally — the mint step (`actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3`) copies verbatim. What does NOT transfer is the specific secrets: `RELEASE_APP`/`RELEASE_APP_PRIVATE_KEY` belong to the release App and must not be silently reused — the new App needs its own name/secrets and a `checkpoint:human-verify` for provisioning (per RESEARCH.md Open Question 2).
2. **`export_test.go` convention:** none exists in `internal/server/`. All test files are `<subject>_test.go`, `package server` (in-package, not `package server_test`) — confirmed via `ls internal/server/*_test.go` in RESEARCH.md. An in-package `_test.go` file can freely define a method called from other in-package `_test.go` files; this is standard Go and requires no build tag. The planner should either inline the relocated `Wait()` into `summaryqueue_test.go`/`usagequeue_test.go`, or introduce one new `queue_export_test.go` — either establishes the convention going forward since none currently exists.
3. **Doc-comment convention:** dense, decision/finding-ID-citing (`CR-01`, `SC#1`, `SC#2`, `WR-03`, `D-0x`) — see `tryEnqueue`'s comment above for a representative sample. New comments explaining the `Wait()` relocation should cite `WR-03` explicitly to match this style.
4. **SPDX headers:** confirmed present verbatim on every Go file sampled (`summaryqueue.go`, `usagequeue.go`); exact two-line text captured above under Shared Patterns. Any new file must carry it or `task license:check` fails.

## Metadata

**Analog search scope:** `.rumdl.toml`, `.github/workflows/{ci,release}.yaml`, `internal/server/{summaryqueue,usagequeue,tools,tools_test}.go`, `internal/config/config_test.go`, `internal/server/rules.go` (negative-space confirmation)
**Files scanned:** 9
**Pattern extraction date:** 2026-07-15
</content>
