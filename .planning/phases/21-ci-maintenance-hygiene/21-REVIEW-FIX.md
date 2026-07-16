---
phase: 21-ci-maintenance-hygiene
source: 21-REVIEW.md
fixed: 2026-07-16
mode: --fix --auto --all
iterations: 2
findings_in: 2
fixed_count: 2
skipped_count: 0
status: resolved
---

# Phase 21 — Code Review Fix

Auto-fix pass over `21-REVIEW.md` (`--fix --auto --all`: Critical + Warning + Info in scope).
Both findings fixed and verified live against the real toolchain; re-review converged clean at
iteration 2. No new findings introduced.

## Findings resolved

### WR-01 (warning) — `gh api` failure silently swallowed in `get-user-id`
**File:** `.github/workflows/ci.yaml` (the `ui-drift` self-heal `get-user-id` step)
**Outcome:** FIXED — commit `fix(ci): fail get-user-id step on gh api error instead of emitting empty id (WR-01)`.

The step was `run: echo "user-id=$(gh api ...)" >> "$GITHUB_OUTPUT"`. Bash's `set -e` (the default
for `run:` steps) does **not** abort on a command substitution's failure when the `$()` sits inside
another command's argument — `echo` exits 0 regardless, emitting an empty `user-id`. A transient
GitHub API blip during a legitimate Renovate self-heal would then feed the downstream self-heal step
a malformed committer email (`+slug[bot]@users.noreply.github.com`) and push a badly-attributed
commit, instead of failing cleanly.

Fix: assign on its own line so `-e` aborts the step on a non-zero `gh api` exit
(`user_id=$(...)` is a simple-command assignment, which `-e` **does** catch):

```yaml
run: |
  user_id=$(gh api "/users/${APP_SLUG}[bot]" --jq .id)
  echo "user-id=$user_id" >> "$GITHUB_OUTPUT"
```

Degradation is now correct: a failed `get-user-id` skips the self-heal step (its `if:` carries an
implicit `success()`), so **no malformed commit is pushed** — the job goes red and is retryable,
which is strictly better than a bad-attribution commit. Verified: `task lint:actions` (actionlint)
exits 0.

### IN-01 (info) — stale call-site count in `queue_export_test.go` doc comment
**File:** `internal/server/queue_export_test.go`
**Outcome:** FIXED — commit `docs(server): drop drift-prone call-site count from queue_export_test comment (IN-01)`.

The comment claimed "every one of its **10** call sites … is a `_test.go` file"; the real count is
17 and will keep drifting as tests are added. The load-bearing claim (no production caller; all sites
are `_test.go`) holds and was independently verified. Fix: drop the specific number — "every one of
its call sites in the repo is a `_test.go` file" — a count-independent statement (consistent with
this phase's own decision not to hardcode drift-prone figures, cf. the rumdl count in 21-01).
Verified: `go build ./...`, `go vet`, `gofmt -l internal/server/` all clean.

## Post-fix gate status
- `task lint:actions` (actionlint): PASS
- `go build ./...`: PASS
- `go vet ./internal/server/`: PASS
- `gofmt -l internal/server/`: clean (CI precheck satisfied)

## Not touched (out of scope, correctly)
- Pre-existing `Taskfile.yaml` yamlfmt failure — tracked as #370, unrelated to this phase.
- Pre-existing gopls `strp→new` / `reflect.TypeOf→TypeFor` suggestions in `tools_test.go` (lines
  2031–2300) — not introduced by this phase, not flagged by golangci-lint (the real gate).
- The locked design decisions (App-token over `GITHUB_TOKEN`, no `contents: write` job permission,
  Client-ID from `secrets.`, `renovate.json` untouched) — researched, decided, verified; the review
  found no defect in their implementation.
