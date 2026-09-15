---
phase: "4"
slug: "drift-detection-read-only"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-15"
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no third-party test framework anywhere in this repo) |
| **Config file** | none — `go test` is invoked directly, per `Taskfile.yaml`'s `test:go` task |
| **Quick run command** | `go test ./internal/setup/... ./cmd/engram/... -count=1` |
| **Full suite command** | `task test` (lint + `go test ./...`) |
| **Estimated runtime** | ~60 seconds (quick) / ~180 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/setup/... ./cmd/engram/... -count=1`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite must be green; additionally confirm `04-OBSERVATIONS.md` exists, is dated, and every new scanner fixture cites it (D-08)
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| {N}-01-01 | 01 | 1 | REQ-{XX} | T-{N}-01 / — | {expected secure behavior or "N/A"} | unit | `{command}` | ✅ / ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*(Seeded by plan-phase; gsd-planner fills one row per task from the plans it authors.)*

---

## Wave 0 Requirements

- [ ] `internal/setup/drift_test.go` (or equivalent) — shared `Facet`/comparison pure-function tests (REQ-drift-three-way, REQ-drift-facet-naming)
- [ ] `internal/setup/claudecode_test.go` extension — Claude Code scan fixtures; the literal-echo case is gated on `04-OBSERVATIONS.md` (D-06)
- [ ] `internal/setup/codex_test.go` extension — Codex scan fixtures, same gate
- [ ] `internal/setup/plan_test.go`-adjacent — `TestRedactionUnconditional` (REQ-drift-redaction), the read-path mirror of `TestNoSecretInArgs`
- [ ] `cmd/engram/setup_test.go` / `operator_view_setup_test.go` extension — facet rendering, `setupApplySummary` preserved-count, `--output json` leak test
- [ ] `cmd/engram/migrate_docs_test.go`-style docs gate for `guides/agent-setup.md`'s new `preserved` row and opencode not-compared statement
- [ ] `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` — NOT a Go test; the D-05/D-08 human-checkpoint deliverable that scanner fixtures depend on

*RED-evidence approach: `OutcomePreserved` and the drift-comparison functions do not exist yet, so the first test referencing them fails to COMPILE — a valid, strong RED in Go. Each subsequent test in the same file must be run individually with `-run <name> -v` to confirm its own RED before the GREEN commit (gotcha `bsbsvn4hbc`: a `-run` pattern matching nothing false-greens).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Literal (non-reference) header value echo shape from `claude mcp get` and `codex mcp get --json` | REQ-drift-redaction | Rule `m45p2b4bp7` — no test may invoke a real third-party CLI or touch the operator's `$HOME`; the observation is the D-05 protocol the maintainer runs once | Follow the protocol drafted in 04-RESEARCH.md: register a throwaway entry NOT named `engram` carrying a literal header value, run the read verb, capture verbatim into `04-OBSERVATIONS.md` with CLI versions, remove the entry. Fixtures cite the record. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
