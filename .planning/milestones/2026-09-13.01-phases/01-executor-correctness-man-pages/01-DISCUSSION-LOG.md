# Phase 1: Executor Correctness & Man Pages - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-13
**Phase:** 1-executor-correctness-man-pages
**Areas discussed:** Man-page determinism & header, Man-page command coverage, Cask install/uninstall shape, Timeout row wording

---

## Man-page determinism & header

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed zero date in code | Header.Date set to a constant; no env dependency | ✓ |
| Honor SOURCE_DATE_EPOCH, else zero | Reproducible-build convention with constant fallback | |
| Leave Date nil (cobra default) | time.Now() month — stable only within a calendar month | |

**User's choice:** Fixed zero date in code

| Option | Description | Selected |
|--------|-------------|----------|
| Source = `engram <version>`, Manual = `Engram Manual` | ldflags version; byte-identical per build | ✓ |
| Source = `engram`, Manual = `Engram Manual` | Build-independent, no version on footer | |
| Leave both unset | Source becomes cobra's autogen string | |

**User's choice:** Source = `engram <version>`, Manual = `Engram Manual`

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — set DisableAutoGenTag on root | Drops the HISTORY footer; propagates to children | ✓ |
| No — keep cobra's footer | Deterministic once Date is pinned, cobra-branded line stays | |

**User's choice:** Yes — set on root

| Option | Description | Selected |
|--------|-------------|----------|
| Generate twice, compare + assert header / no HISTORY | No goldens to regenerate | ✓ |
| Twice-compare + golden of root page | Adds `testdata/engram.1.golden` | |
| Golden every page | Full snapshot, regen on every help edit | |

**User's choice:** Generate twice, compare + header asserts
**Notes:** None.

---

## Man-page command coverage

| Option | Description | Selected |
|--------|-------------|----------|
| Cobra default — visible tree incl. `completion` | Zero filtering; hidden `man` and deprecated aliases excluded automatically | ✓ |
| Match the classified catalog — exclude `completion` | Mirror `commandWalkSkip`; extra code | |
| You decide | Planner picks | |

**User's choice:** Cobra default incl. `completion`

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — derive expected set from the tree | Walk rootCmd with IsAvailableCommand; assert `engram-man.1` + deprecated pages absent | ✓ |
| Yes — hardcode sentinel names | A few presence/absence checks | |
| No — twice-compare is enough | Determinism test only | |

**User's choice:** Derive expected set from the tree
**Notes:** None.

---

## Cask install/uninstall shape

| Option | Description | Selected |
|--------|-------------|----------|
| Generate straight into man1 | `mkdir_p` + `system_command binary, args: ["man", man1]` | ✓ |
| Generate into tmpdir, then copy | Isolates partial failure; more Ruby | |

**User's choice:** Generate straight into man1

| Option | Description | Selected |
|--------|-------------|----------|
| Glob `engram.1` + `engram-*.1`, rm_f each | File-scoped, no manifest | ✓ |
| Install-time manifest; read + rm_f at uninstall | Literal path list; 4th artifact | |
| You decide | Planner picks | |

**User's choice:** Glob + rm_f each

| Option | Description | Selected |
|--------|-------------|----------|
| After completions; pin version→completion→man + glob string; add `manpage:` to forbidden list | Appended 4th binary exercise | ✓ |
| Between version gate and completions | Same pinning, different link order | |
| You decide | Planner picks | |

**User's choice:** After completions
**Notes:** None.

---

## Timeout row wording

| Option | Description | Selected |
|--------|-------------|----------|
| Wrap at runSeam: `timed out after 20s: context deadline exceeded` | runSeam owns execTimeout; errors.Is preserved | ✓ |
| Bare ctx.Err(): `context deadline exceeded` | Three-line diff only | |
| Wrap inside osRun | osRun cannot name the duration | |

**User's choice:** Wrap at runSeam

| Option | Description | Selected |
|--------|-------------|----------|
| Same seam-error path, bare `context canceled` | `ctx.Err() != nil` covers both sentinels | ✓ |
| Deadline only; Canceled keeps ExitError handling | Narrow check | |

**User's choice:** Same seam-error path

| Option | Description | Selected |
|--------|-------------|----------|
| Zero RunResult{} | Matches documented contract | ✓ |
| Carry partial stdout/stderr | Would need a doc amendment | |

**User's choice:** Zero RunResult{}
**Notes:** None.

---

## Claude's Discretion

- osRun test subprocess strategy (helper re-exec or POSIX `sleep`; never a third-party CLI or the operator's `$HOME`).
- `engram man` arg validation, Short/Long text, file placement, pinned-date constant location.
- Plan/wave ordering of the two independent fixes.

## Deferred Ideas

None — documenting `man engram` in `guides/install.md` is Phase 5's docs close-out.
