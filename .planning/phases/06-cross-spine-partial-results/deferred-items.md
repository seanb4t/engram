## Deferred Items

- `internal/store`'s `TestRedEvidencePatchesAreLive` hit Go's default 601s per-package test
  timeout twice during plan 06-01's `task` gate runs, on a tree containing zero changes to
  `internal/store`.
  status: open
  **What:** `TestRedEvidencePatchesAreLive` (`internal/store/redevidence_harness_test.go`)
  sequentially applies all 54 currently-registered red-evidence patches, shelling out a real
  `go test -run <target>` subprocess per patch to prove RED, then reverting via `git apply -R`.
  On this run's machine, that sequential subprocess-per-patch architecture — compounded by heavy,
  concurrent, *unrelated* `go test`/`golangci-lint` load from other active sessions on the same
  shared host (confirmed via `ps aux`: a different repo's `go test -count=1 ./...`, and later a
  `go test -p 4 -race -covermode=atomic ./...` run) — pushed total wall-clock past Go's 601s
  default per-package timeout twice in a row, at 601.305s and 601.117s respectively. Both times,
  the timeout fired mid-patch (the process is killed via `SIGQUIT`+exit before `t.Cleanup` can run
  the `git apply -R` revert), leaving exactly one already-applied red-evidence patch un-reverted on
  disk: first `internal/server/tools.go` (a `checkTags` call removed) + `internal/store/boundedread.go`
  (`sweepLimit`'s byte-derived branch removed), second `internal/store/store.go` (`Reindex`'s
  trailing-page flush removed). Both times the stray diff was hand-verified against `git diff` and
  restored with `git checkout -- <file>` before continuing — no red-evidence patch registration,
  target test, or unrelated source file was modified.
  **Evidence this is environmental, not a 06-01 regression:**
  - Zero files under `internal/store` are in 06-01's `files_modified` or actual diff
    (`git diff --name-only 5b75fa01..HEAD -- internal/store` is empty).
  - Every 06-01-scoped check passed cleanly, twice: `go build ./...`, `go vet ./internal/server/...`,
    all `internal/server` package tests (`go test -short ./internal/server/... -count=1` — 26.7s /
    36.1s across the two full-gate attempts), the four new `TestCrossSpineCoverageUnknown*` tests,
    `TestCrossSpineCoverageThreeStates`, the descriptor gate
    (`TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs`), `task proto:gen` +
    `git diff --exit-code -- proto gen ui/src/lib/gen`, and all seven pre-existing cross-spine
    regression tests under `ENGRAM_REQUIRE_QDRANT=1` (7/7 PASS, 0 SKIP).
  - `internal/store/storetest` (a sibling package in the same module) passed on both attempts
    (5.9s / 4.5s) — the Docker/Qdrant testcontainer path itself is healthy.
  - No `--- FAIL:` line for any named test ever appeared in either `task` run's output — the
    failure is exclusively `FAIL github.com/seanb4t/engram/internal/store 601.3xxs`, i.e. the
    package-level timeout, never a test assertion.
  **Confirmed via a third, diagnostic run:** `go test ./internal/store/... -count=1 -timeout 20m`
  (bypassing `task`'s unconfigured default) PASSED cleanly at 685.150s (`internal/store`) / 4.418s
  (`internal/store/storetest`), with zero `--- FAIL:` lines and a clean `git status` afterward —
  conclusive proof the package is correct and the harness merely needs more wall-clock time than
  Go's 601s default under this run's machine load, not that anything is broken.
  **Recommendation:** either give `internal/store`'s test binary an explicit `-timeout` well above
  601s in `Taskfile.yaml`'s `test:go` (or a per-package override for this file), or reduce
  `TestRedEvidencePatchesAreLive`'s per-patch cost (e.g. a single `go test -c`-compiled binary
  reused across `-run` invocations instead of `go build`+`go test` per patch). Out of scope for
  plan 06-01 — no `internal/store` file is in this plan's files_modified, and the fix belongs to
  whichever phase or maintenance pass owns `internal/store`'s test-harness performance.

- Same characteristic recurred during plan 06-03's phase close, now at 58 registered patches.
  status: open
  **What:** Bare `task` (no `-timeout` override) hit the 601s per-package default and killed
  `internal/store`'s test binary mid-patch at 630.559s — again leaving one already-applied
  red-evidence patch un-reverted on disk (`03-03-update-content-cap-removed.patch` over
  `internal/server/tools.go`, a `checkContentBytes` call removed). Hand-verified against `git diff`
  and restored with `git checkout -- internal/server/tools.go` before any commit; no red-evidence
  patch registration, target test, or unrelated source file was modified. A follow-up diagnostic at
  `-timeout 20m` also timed out (1209.860s, closer to the limit than 06-01's 685s run — the harness
  is getting slower as patches accumulate); a second diagnostic at `-timeout 60m` avoided the
  timeout but hit a transient Docker/testcontainer `connection refused` across nearly every test in
  the package — every test failed identically at container-connect, not at an assertion, which is
  the signature of environment flakiness rather than a code defect (no leftover patch this time).
  This plan's own `<verify>` steps for Tasks 1 and 2 (explicit `-timeout 180m`, the plan-specified
  generous timeout) already provide clean, valid, authoritative proof: 54/54 and then 58/58
  confirmed RED, `ok`, clean tree. No timeout was raised in `Taskfile.yaml` or CI.
  **Recommendation unchanged from 06-01**, now with more urgency: the harness's total wall-clock
  keeps growing with each phase's patches and is now within ~1.5x of Go's own default timeout even
  under a dedicated, otherwise-idle invocation. Whichever phase or maintenance pass owns
  `internal/store`'s test-harness performance should treat this as escalating, not merely open.
