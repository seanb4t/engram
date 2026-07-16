# Deferred Items — Phase 21

Out-of-scope discoveries logged per the executor's scope-boundary rule (issues
pre-existing and unrelated to the current task's file changes are logged, not
fixed).

## 21-02: `task lint:yaml` (yamlfmt) fails on pre-existing `Taskfile.yaml` formatting

**Found during:** 21-02 final verification (`task` full default gate).

**Issue:** `yamlfmt -lint .` reports `Taskfile.yaml` as not matching its
formatting rules. This is unrelated to any of 21-02's file changes
(`internal/server/{summaryqueue,usagequeue,tools,tools_test,queue_export_test}.go`)
— `Taskfile.yaml` is untouched by this plan. Confirmed pre-existing: byte-identical
to the `Taskfile.yaml` at commit `bb3edccb` (the last commit before 21-02 started,
end of plan 21-01).

**Impact:** `task` (the full default lint+test gate) exits non-zero via
`lint:yaml`, even though `task lint:go`, `task license:check`, `gofmt -l .`,
and `task test:go` are all clean. CI runs `yamlfmt` too (per `Taskfile.yaml`
`lint:yaml`), so CI is presumably already red on `main` for this reason,
independent of Phase 21.

**Not fixed here:** out of 21-02's scope (Taskfile.yaml formatting, not
`internal/server/` or the Phase-11 review residuals). Needs its own issue —
either fix `Taskfile.yaml`'s formatting to satisfy `yamlfmt`, or adjust
`.yamlfmt` config if the current formatting is intentional.
