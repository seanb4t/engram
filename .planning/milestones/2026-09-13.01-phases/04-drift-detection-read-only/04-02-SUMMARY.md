---
phase: 04-drift-detection-read-only
plan: 02
subsystem: setup
tags: [drift-detection, observation, claude-code, codex, redaction, mcp-registration]

# Dependency graph
requires:
  - phase: 02-custom-auth-headers
    provides: header vocabulary (HeaderSpec{Name, EnvVar}), Codex's ErrHeaderUnsupported decline
provides:
  - "Verbatim D-05 observation record of the literal-header-value echo shape for both `claude mcp get` and `codex mcp get --json`, replacing Assumption A1/A3 with observed reality"
affects: [04-01, 04-04, 04-05]

# Actuals (#2632)
actuals:
  tokens: 2774
  tasks: 2
  commits: 1

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created:
    - .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
  modified: []

key-decisions:
  - "The maintainer ran the D-05 protocol on macOS/fish (bash's $? mapped to fish's $status; unset mapped to set -e) — recorded as an environment note, not a protocol deviation."
  - "Claude Code's Headers: block echoes the literal value verbatim with no additional escaping or masking on read-back, even though the ADD verb's own confirmation output shows [REDACTED] — confirming the incident this phase closes (gotcha ryr82bf2s2) is a read-path gap, not a write-path one."
  - "Codex's literal header rode under transport.http_headers as an object-of-strings, confirming Assumption A3; env_http_headers was never exercised since http_headers accepted the literal on the first attempt (no rejection occurred)."
  - "Codex 0.154.0's mcp get --json key set is byte-identical to the 04-RESEARCH.md-documented 0.153.4 shape — no new key needs to be added to the totality-parse struct for D-11."

patterns-established: []

requirements-completed: [REQ-drift-redaction, REQ-drift-observed-registration]

coverage:
  - id: D1
    description: "04-OBSERVATIONS.md records the verbatim literal-header echo shape for claude mcp get and codex mcp get --json, dated, versioned, with exit codes and a What this pins fact list"
    requirement: REQ-drift-observed-registration
    verification:
      - kind: other
        ref: "Task 2 automated verify (rg/test chain over 04-OBSERVATIONS.md: dummy marker present, no stray sk- token, both command lines present, dated Observed line, What this pins heading, >=3 exit code lines, single-file commit)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The throwaway probe-literal-04 entries were fully removed from Claude Code and the isolated Codex CODEX_HOME, with the maintainer's real engram registration and real ~/.codex/config.toml never touched"
    verification: []
    human_judgment: true
    rationale: "Rule m45p2b4bp7 forbids an automated test from invoking a real claude/codex binary or touching the operator's $HOME; only the human's own confirmation (captured in the resume signal and mirrored in the Protocol/Removal sections of 04-OBSERVATIONS.md) attests removal — not machine-checkable from this repo."

duration: 6min
completed: 2026-09-15
status: complete
---

# Phase 4 Plan 2: D-05 Literal Header Echo Observation Summary

**Recorded the maintainer's verbatim capture of `claude mcp get` and `codex mcp get --json` echoing a literal (non-`${VAR}`) custom header value, closing the last unobserved shape gap before plan 04-05's scanner fixtures are written.**

## Performance

- **Duration:** 6 min (Task 2 only; Task 1 was a human-run checkpoint resolved out-of-band)
- **Started:** 2026-09-15T23:15:00Z
- **Completed:** 2026-09-15T23:21:31Z
- **Tasks:** 2 (1 checkpoint, resolved with full captures; 1 auto)
- **Files modified:** 1 created

## Accomplishments
- Task 1 (`checkpoint:human-verify`, `gate="blocking-human"`): the maintainer ran the D-05 manual observation protocol on their own machine (claude 2.1.273, codex-cli 0.154.0, macOS/fish shell) and pasted full verbatim captures — resolved, not skipped.
- Task 2: wrote `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` from those captures verbatim (no reconstruction, no cleanup), covering: Environment, Protocol, Claude Code literal value, Claude Code bare-reference control, Codex bearer-only control, Codex literal header (hand-edited), and a `## What this pins` fact list for plan 04-05.
- Confirmed Codex's `http_headers` renders as an object-of-strings (Assumption A3 verified) and that the 0.154.0 key set matches the 04-RESEARCH.md-documented 0.153.4 shape exactly — no unrecognized keys.
- Confirmed Claude Code echoes the literal header value in cleartext on `mcp get`, in the same `Headers:` block position as the bare-reference shape (Assumption A1 verified for the label-set framing), while the `ADD` verb's own confirmation output masks it as `[REDACTED]` — a cosmetic-only mask that provides no read-back protection.
- Recorded two provenance caveats honestly rather than silently smoothing them over: the `/tmp/04-observe-claude.txt` redirect was later re-run post-removal (overwriting the file), so the literal capture's authoritative source is the terminal `cat` output, not a second read of the file; and the hand-edited `config.toml` was never pasted back, so its acceptance is evidenced only by the post-edit `--json` capture, not a file diff.

## Task Commits

1. **Task 1: OBSERVE — D-05 protocol run by the maintainer** - checkpoint, no commit (human action)
2. **Task 2: Record the observation as 04-OBSERVATIONS.md** - `3cafb90c` (docs)

**Plan metadata:** (this commit)

## Files Created/Modified
- `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` - Dated D-05 observation record: environment, exact protocol commands, verbatim literal/bare-reference/bearer-only/literal-hand-edited captures with exit codes, and the `## What this pins` facts for plan 04-05's scanners.

## Decisions Made
- Ledger correction (execution-time, not a plan deviation): the per-plan commit ledger at `.git/gsd-plan-head-before-04-02` was found pre-existing and stale (dated Sep 10, pointing at a commit ~135 commits behind the actual plan start) — almost certainly a leftover from an earlier milestone that also numbered a phase/plan `04-02` in this shared `.git` directory. Corrected it to the actual pre-plan HEAD (`7257a020`, the `04-03` completion commit, confirmed as the direct parent of this plan's only commit) before computing `actuals.commits`. This is an execution-time correctness fix to the measurement instrument, not a plan-scope deviation — no plan file or code changed as a result.
- See `key-decisions` in frontmatter for the observation's substantive findings.

## Deviations from Plan

None - plan executed exactly as written. (The ledger correction above is an execution-tooling correctness fix, not a deviation from the plan's `<action>`/`<verify>`/`<acceptance_criteria>`.)

## Issues Encountered
None. Task 2's automated verify and all five acceptance criteria passed on the first attempt.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 04-05's Claude Code and Codex literal-echo scanner fixtures can now be built by quoting `04-OBSERVATIONS.md` and citing it in a header comment (D-08) — the soft gate (D-06) is cleared.
- REQ-drift-redaction and REQ-drift-observed-registration remain open in REQUIREMENTS.md: this plan INFORMS both (supplies the observed shape) but their code-level satisfaction lives in plans 04-01 (already complete), 04-04, and 04-05 (not yet run). `requirements.ready-ids` confirmed 0/2 ready to mark at this point in the phase — correct, since 04-04/04-05 still declare them and haven't produced summaries.
- No blockers for the remaining phase 4 plans.

---
*Phase: 04-drift-detection-read-only*
*Completed: 2026-09-15*

## Self-Check: PASSED

- `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` — FOUND on disk.
- `.planning/phases/04-drift-detection-read-only/04-02-SUMMARY.md` — FOUND on disk (this file).
- Commit `3cafb90` (`docs(04): record literal header echo observation for drift fixtures (D-05, D-08)`) — FOUND in `git log --oneline --all`.
- Commit `fc967cf` (`docs(04-02): complete D-05 literal header echo observation plan`) — FOUND in `git log --oneline --all`.
- Task 2's automated `<verify>` chain re-run: all links passed (dummy marker present, zero stray `sk-` tokens, both read-verb command lines present, dated `**Observed:**` line present, `## What this pins` heading present, 5 `exit code: N` lines ≥3, single-file commit for the record).
- All five `<acceptance_criteria>` re-checked and passing (7 `## ` headings, 38 `probe-literal-04` lines, zero `mcp (get|add|remove) engram` matches, `Headers:`/`http_headers` both present, first line is the title, commit subject and single-file `--stat` match exactly).
