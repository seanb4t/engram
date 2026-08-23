---
phase: 01-gate-ci-integrity
plan: 03
kind: one-time-reassessment
milestone_scoped: v0.13.x Phase 1 (interface-enforceability) + Phase 2 (interface-discoverability)
requirement: REQ-keylink-past-gates-reassessed
produced_by: internal/keylinks/sweep_test.go#TestReassessV013Phase12
resolved_at_commit: e3b38bb5c86bae050162f8935180e1c746334c88
created: 2026-08-13
---

# v0.13.x Phase 1-2 Key-Link Reassessment

## Why this document exists

`gsd-tools verify key-links` hands a plan's `pattern:` field to JavaScript's `new RegExp` without
ever unescaping it (`verify.cjs:1049-1117`). A pattern authored with a doubled escape character
(`\\.`) therefore reached the regex engine meaning "a literal backslash, then any character" —
valid regex, silently unmatchable. v0.13.x Phases 1-2 shipped and passed verification while a
majority of their key-link gates were checking nothing (#479). `REQ-keylink-past-gates-reassessed`
requires those gates re-resolved against HEAD, once, with the result recorded — not repaired
(D-14; "reassessed" is not "repaired"). This document is that record.

## Resolution rule (D-11)

A key-link is **pinned** when its corrected, escape-free pattern (`internal/keylinks` plan 01-02's
rewrite) resolves against its `from` file's content at HEAD, or — failing that — against its `to`
file's content at HEAD by the same from-then-to fallback the consuming tool (`verify.cjs`) applies.
Resolution ran against commit `e3b38bb5c86bae050162f8935180e1c746334c88` (this phase's own Task 1
commit, immediately after `sweep_test.go` landed and before this table was written).

## Verdict table

Every row below is one v0.13.x Phase 1-2 key-link, transcribed verbatim from
`TestReassessV013Phase12`'s `-v` output at the commit above. 30 links found, matching the 30
recorded at planning time (`01-03-PLAN.md`'s estimate) exactly — no reconciliation needed here,
unlike plan 01-02's 39-vs-38 discrepancy.

| Plan file:line | from | to | pattern | verdict | reason |
|---|---|---|---|---|---|
| `01-interface-enforceability/01-01-PLAN.md:41` | `cmd/engram/exitcode_baseline_test.go` | `cmd/engram/root.go` | `exitCodeFromError[(]` | pinned | matched its from file |
| `01-interface-enforceability/01-01-PLAN.md:45` | `cmd/engram/exitcode_baseline_test.go` | `cmd/engram/clienttest_test.go` | `runClient[(]t` | pinned | matched its from file |
| `01-interface-enforceability/01-02-PLAN.md:49` | `cmd/engram/client_list.go` | `cmd/engram/root.go` | `ValidateFlagGroups[(][)]` | pinned | matched its from file |
| `01-interface-enforceability/01-02-PLAN.md:53` | `cmd/engram/root.go` | `cmd/engram/client_common.go` | `usageErrorf[(]` | pinned | matched its from file |
| `01-interface-enforceability/01-03-PLAN.md:51` | `internal/config/registry.go` | `internal/config/config.go` | `koanf:"client"` | pinned-via-target | matched its to file by the from-file fallback; from file did not match |
| `01-interface-enforceability/01-03-PLAN.md:55` | `internal/config/config.go` | `internal/config/client_validate.go` | `ValidateClient[(]` | pinned-via-target | matched its to file by the from-file fallback; from file did not match |
| `01-interface-enforceability/01-04-PLAN.md:49` | `cmd/engram/client_search.go` | `cmd/engram/root.go` | `MarkFlagsMutuallyExclusive[(]"scope", "cross-spine"[)]` | pinned | matched its from file |
| `01-interface-enforceability/01-04-PLAN.md:53` | `cmd/engram/migrate.go` | `internal/store/store.go` | `store[.]ValidateOwnerRemap[(]` | pinned | matched its from file |
| `01-interface-enforceability/01-05-PLAN.md:47` | `cmd/engram/operror.go` | `internal/store/store.go` | `store[.]Err(NotFound\|InvalidArgument\|AmbiguousShortID)` | pinned | matched its from file |
| `01-interface-enforceability/01-05-PLAN.md:51` | `cmd/engram/reindex.go` | `cmd/engram/operror.go` | `classifyOperatorErr[(]` | pinned | matched its from file |
| `01-interface-enforceability/01-06-PLAN.md:40` | `cmd/engram/serve.go` | `cmd/engram/client_common.go` | `usageErrorf[(]` | pinned | matched its from file |
| `01-interface-enforceability/01-06-PLAN.md:44` | `cmd/engram/migrate.go` | `cmd/engram/operror.go` | `classifyOperatorErr[(]` | pinned | matched its from file |
| `01-interface-enforceability/01-07-PLAN.md:44` | `cmd/engram/client_common.go` | `internal/config/config.go` | `config[.]Load[(]cmd[.]Flags[(][)][)]` | pinned | matched its from file |
| `01-interface-enforceability/01-07-PLAN.md:48` | `cmd/engram/client_common.go` | `internal/config/client_validate.go` | `config[.]ValidateClient[(]` | pinned | matched its from file |
| `01-interface-enforceability/01-07-PLAN.md:52` | `cmd/engram/catalog.go` | `cmd/engram/client_common.go` | `Code: exitTimeout` | pinned | matched its from file |
| `01-interface-enforceability/01-08-PLAN.md:43` | `cmd/engram/client_search.go` | `cmd/engram/client_common.go` | `context[.]WithTimeout[(]` | pinned | matched its from file |
| `01-interface-enforceability/01-08-PLAN.md:47` | `cmd/engram/timeout_test.go` | `cmd/engram/client_common.go` | `exitTimeout` | pinned | matched its from file |
| `01-interface-enforceability/01-09-PLAN.md:45` | `cmd/engram/docsync_test.go` | `docs-site/src/content/docs/guides/upgrade.md` | `upgrade[.]md` | pinned | matched its from file |
| `01-interface-enforceability/01-09-PLAN.md:49` | `docs-site/src/content/docs/guides/cli.md` | `cmd/engram/catalog.go` | `Exit codes` | pinned | matched its from file |
| `02-interface-discoverability/02-01-PLAN.md:67` | `cmd/engram/client_search.go` | `internal/surfaces/rules.go` | `surfaces[.]RuleByID` | pinned | matched its from file |
| `02-interface-discoverability/02-01-PLAN.md:71` | `internal/server/tools.go` | `internal/server/conditionalerr.go` | `conditionalErrf[(]` | pinned | matched its from file |
| `02-interface-discoverability/02-01-PLAN.md:75` | `internal/surfacesgen/main.go` | `internal/surfaces/rules.go` | `surfaces[.]Rules[(][)]` | pinned | matched its from file |
| `02-interface-discoverability/02-02-PLAN.md:54` | `internal/server/tools.go` | `internal/server/registertools_test.go` | `mcp[.]NewInMemoryTransports` | pinned-via-target | matched its to file by the from-file fallback; from file did not match |
| `02-interface-discoverability/02-02-PLAN.md:58` | `internal/surfaces/conformance_test.go` | `internal/surfaces/rules.go` | `surfaces[.]Rules[(][)]\|Rules[(][)]` | pinned | matched its from file |
| `02-interface-discoverability/02-03-PLAN.md:60` | `internal/server/connectapi.go` | `internal/surfaces/rules.go` | `conditionalErrf[(]` | pinned | matched its from file |
| `02-interface-discoverability/02-03-PLAN.md:64` | `cmd/engram/client_list.go` | `internal/surfaces/rules.go` | `surfaces[.]RuleByID` | pinned | matched its from file |
| `02-interface-discoverability/02-04-PLAN.md:48` | `internal/server/tools.go` | `internal/surfaces/toolclass.go` | `surfaces[.]ClassForTool` | **unpinned** | routed through a wrapper — `internal/server/tools.go`'s `mcp.AddTool` calls `annotationsFor(name)` (`internal/server/toolannotations.go:24`), and `annotationsFor` is what calls `surfaces.ClassForTool`, not the `from` file directly. `tools.go` itself has never contained the literal call site the pattern names (confirmed: `git log --all -S"ClassForTool" -- internal/server/tools.go` returns no commits). The linked behavior is real and correctly wired; only the pattern's from-file assumption is stale. |
| `02-interface-discoverability/02-04-PLAN.md:52` | `internal/server/toolannotations_test.go` | `internal/server/registertools_test.go` | `registeredTools[(]` | pinned | matched its from file |
| `02-interface-discoverability/02-05-PLAN.md:56` | `cmd/engram/catalog.go` | `internal/surfaces/toolclass.go` | `surfaces[.]ClassForCommand` | pinned | matched its from file |
| `02-interface-discoverability/02-05-PLAN.md:60` | `Taskfile.yaml` | `cmd/engram/testdata/` | `-update` | pinned | matched its from file |

## Rollup

| Verdict | Count |
|---|---|
| pinned | 26 |
| pinned-via-target | 3 |
| unpinned | 1 |
| unreadable | 0 |
| invalid | 0 |
| **total** | **30** |

## Reproducing this table

```
go test ./internal/keylinks/... -run 'TestReassessV013Phase12|TestReassessmentTableIsComplete' -v
```

`TestReassessV013Phase12` logs one line per link (file:line, from, to, pattern, verdict, reason)
plus a rollup line. `TestReassessmentTableIsComplete` mechanically asserts that every parsed link
produced exactly one verdict from the closed set `{pinned, pinned-via-target, unpinned, unreadable,
invalid}` — its own red direction was proven by a deliberate skip before this table was written
(verbatim output in `01-03-SUMMARY.md`).

## What happens to the one unpinned gate (D-14)

`02-interface-discoverability/02-04-PLAN.md:48`'s `surfaces[.]ClassForTool` key-link is recorded
unpinned and is **not repaired in this phase**. `REQ-keylink-past-gates-reassessed` asks for
reassessment, not repair — writing a regression test or moving the call site is unscoped work
discovered mid-phase, and D-14 says that decision is made deliberately and later, not absorbed
silently now. The underlying behavior this gate was meant to protect (every MCP tool's
`Annotations` sourced from `surfaces.ClassForTool` via the shared blast-radius table) is real and
correctly wired — confirmed above by inspection, not merely assumed — so no functional defect was
found, only a stale key-link pin. If repinning ever becomes worthwhile, that is its own scoped
work.

## Upstream unescaping gap disposition (D-01)

The root cause of #479 lives in gsd-core, not this repo: `parseMustHavesBlock` strips a single
pair of leading/trailing quote characters from a `pattern:` scalar and never backslash-unescapes
it, so a doubled-escape string (`\\.`) reaches `new RegExp` intact and compiles to "a literal
backslash, then any character" — valid regex, silently unmatchable. D-01 fixes this repo-locally
(plan 01-02's rewrite plus this phase's `internal/keylinks` guard) and deliberately does **not**
patch gsd-core. D-01 explicitly left open whether the gap itself gets *reported* upstream (a
GitHub issue against the tool) or *spine-tracked* (a durable engram memory in this repo), naming
it Sean's call rather than the executor's, per the live precedent in memory `cvvrwjbsnz` (a prior
GSD-core bug spine-tracked rather than filed, by explicit decision).

**Decision, made by Sean at this phase's Task 3 checkpoint: `spine-track`.** The gap is recorded
as durable project memory in this repo's engram spine. No GitHub issue is filed against the
gsd-core tool's repository, and no other outward-facing action is taken.

**Consequence, stated plainly and not softened:** because the gap stays unreported upstream, every
other consumer of gsd-core's `verify key-links` keeps hitting the identical silent no-op this
phase's own guard exists to catch — `parseMustHavesBlock` is unpatched, so the defect is not fixed
at its source, only worked around here. This repo's `internal/keylinks` guard is therefore
**permanent, not transitional**: there is no upstream fix in flight that would ever make it
redundant, because none was requested. Choosing `spine-track` over `file-upstream` or `both` is a
choice to keep the record local and skip external coordination, at the cost of leaving the actual
defect open everywhere else the tool is used.

**Where the memory itself is written:** this document records the decision and its consequence;
it does not create the spine record. Per the division of labor at this checkpoint, the orchestrator
writes the actual `store_memory` (or `supersede_memory`, if a prior related record exists) call at
phase close — search-before-store and supersede-on-contradiction are the orchestrator's
responsibility, not this plan's Task 3.

## Placement note (deliberate, not an oversight)

Decision D-13 in `01-CONTEXT.md` originally named this phase's own `VERIFICATION.md` as the home
for this table. `VERIFICATION.md` is written and re-parsed by GSD tooling, and this repo's own
planning-artifacts rule (`8dfdhfs5nn`: never invent structure inside a tool-owned generated file)
forbids hand-adding a new section to a document a tool both writes and re-reads — that exact class
of edit has already produced silent misclassification elsewhere in this repo's planning history.
This table therefore lands in this standalone, phase-owned document instead, which no tool
generates or re-parses; `01-VERIFICATION.md` cites it by path rather than embedding it. D-13's
substance is honored unchanged: the verdicts live in this phase's own record, not in
`.planning/WINDOWS.md` (which would file permanent open debt against shipped work nobody plans to
revisit) and not as a burst of one GitHub issue per unpinned gate (a single unpinned gate would not
have warranted that anyway). Only the file changed, and the reason is on the record here per rule
`8dfdhfs5nn`'s own instruction to report a tool-shape gap rather than invent around it locally.
