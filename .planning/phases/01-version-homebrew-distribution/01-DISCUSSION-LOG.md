# Phase 1: Version & Homebrew Distribution - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-23
**Phase:** 1-Version & Homebrew Distribution
**Areas discussed:** Version flag shape, Version payload contents, Cask gate strictness, Tap credential + re-ship proof

---

## Version flag shape

### Q1 — How should `engram version` expose machine-readable output?

| Option | Description | Selected |
|--------|-------------|----------|
| `--output`, text-default | Repo's `--output json\|text` vocabulary, default pinned to text unconditionally, no TTY detection | ✓ |
| `--json` bool, as roadmapped | Literal to ROADMAP and to codegraph's cask gate; a third flag vocabulary | |
| `--output`, TTY-detecting | Reuses `addOperatorOutputFlag` verbatim; breaks REQ-version-json for piped callers | |

**User's choice:** Initially answered free-text — "this seems most consistent with the rest of the cli surface" — without selecting an option. Since both `--output` variants match that instinct and they differ on the one thing that matters (the default), the question was re-asked as plain text with a fourth possibility offered: TTY-detecting *plus* amending REQ-version-json to sanction the break. User answered **1** — text default, requirement intact.
**Notes:** The ambiguity was worth resolving rather than guessing; option 3 would have silently broken the cask's own postflight, `$(engram version)` in scripts, and every non-TTY caller.

### Q2 — Does cobra's built-in `-v, --version` flag change in this phase?

| Option | Description | Selected |
|--------|-------------|----------|
| Leave it untouched | Only the subcommand gains `--output`; cobra's template unchanged | ✓ |
| Reconcile the two text outputs | `SetVersionTemplate` so both print bare `0.14.0` | |
| Route the flag through the subcommand | `-v` fully equivalent, honors `--output` | |

**User's choice:** Leave it untouched.
**Notes:** The two surfaces disagree today (`0.14.0` vs `engram version 0.14.0`) and stay that way by choice. Options 2 and 3 both change existing-caller output — the same class of break D-01 was chosen to avoid, just on a different surface.

### Q3 — What does `engram version --output bogus` do?

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse the taxonomy — exit 2 | `ValidateOutputFormat` → `usageErrorf` → `exitUsage` | ✓ |
| Accept anything, fall back to text | Never fail on the flag | |
| Reject, but exit 1 | Fail without joining the D-09 taxonomy | |

**User's choice:** Reuse the taxonomy — exit 2.
**Notes:** Accepts that `catalog.golden` grows a `version` entry via the exit-code catalog derivation. Option 2 was silently-wrong-output, the exact failure mode the phase exists to eliminate.

### Q4 — How should a non-release build derive its version?

This question was **rejected once for clarification**. The user asked: *"why not properly use semver and do last release and add -dev + git hash?"* — reframing the question away from the original "what do we do about the `dev` sentinel" toward deriving a real ordered version.

| Option | Description | Selected |
|--------|-------------|----------|
| ReadBuildInfo, patch-bumped | `runtime/debug.ReadBuildInfo()` fallback → `0.14.1-dev.2+g800a98f1`; no build-system change | ✓ |
| git describe via ldflags | Taskfile target injects `git describe`; bare `go build` still yields `dev` | |
| Both — ldflags wins, ReadBuildInfo backstops | Two code paths, `dev` unreachable | |
| Keep bare "dev" | No change (the original recommendation, superseded by the user's reframe) | |

**User's choice:** ReadBuildInfo, patch-bumped.
**Notes:** Before re-asking, one correction was raised: the obvious spelling of the idea is wrong. `git describe` yields `v0.14.0-2-g800a98f1`, which as SemVer parses as `0.14.0` with prerelease `2-g800a98f1` — and a prerelease sorts *before* its release, so a build made after v0.14.0 would compare as older. The patch-bump (`0.14.1-dev.2`) is what restores correct ordering, matching Go's own pseudo-version convention. The user's instinct also uncovered a live bug: `go install ...@v0.14.0` reports `dev` today.

### Q5 — Where does the patch-bump base come from?

Asked after the user chose "More questions" over advancing, on the grounds that this detail is what makes D-04 implementable.

| Option | Description | Selected |
|--------|-------------|----------|
| release-please-synced Go const | `const lastRelease = "0.14.0" // x-release-please-version`, 4th `extra-files` entry | ✓ |
| No base — `0.0.0-devel+hash` | Zero machinery; loses the ordering property | |
| Read the tag at build time | Always right; reintroduces the build-system dependency D-04 avoided | |

**User's choice:** release-please-synced Go const.
**Notes:** Recorded alongside it: with `bump-minor-pre-major: true` the real next release is usually 0.15.0, not 0.14.1, so the patch-bump is a correctly-ordering lower bound and not a prediction. Flagged explicitly so nobody "fixes" it later.

---

## Version payload contents

### Q1 — What fields does `engram version --output json` carry?

| Option | Description | Selected |
|--------|-------------|----------|
| version only | `{"version":"..."}`; commit and dirty already inside the string as build metadata | ✓ |
| version + commit + dirty | Promotes encoded facts to first-class fields; creates a sync invariant | |
| Full build identity | Adds `os`, `arch`, `go`; largest stability commitment | |

**User's choice:** version only.
**Notes:** D-04 had already changed this question's shape — the commit is inside the version string, so a `commit` field would be a second spelling of the same fact.

### Q2 — How does `version` render, given `renderOperator` derives text from the JSON doc?

| Option | Description | Selected |
|--------|-------------|----------|
| Own minimal render pair | Text prints bare string, json encodes doc; bounded documented divergence | ✓ |
| Reuse renderOperator, accept new text | Invariant holds binary-wide; breaks REQ-version-json | |
| Generalize renderOperator | Add a scalar text mode to the shared helper | |

**User's choice:** Own minimal render pair.
**Notes:** Surfaced during scouting, not anticipated. `renderOperatorView` always emits headline + blank line + padded label/value rows, so reuse would print a three-line block instead of `0.14.0`. Option 3 would have touched a helper every operator command and spine-review leaf depends on, to serve one caller. A test pinning text == `jq -r .version` was made a condition of the chosen option.

### Q3 — Does the version string carry a `v` prefix?

**Not asked — derived,** with the user given an explicit opportunity to object before advancing. No `v` prefix. Recorded as D-08 because the tap's existing codegraph precedent points the other way and a mismatch makes the gate compare unequal strings.

---

## Cask gate strictness

### Q1 — What does the cask's postflight assert, after stripping quarantine?

| Option | Description | Selected |
|--------|-------------|----------|
| Version equality only | Strip quarantine, run version, raise on mismatch | |
| Version + completions, no freshness | Adds completions written via `system_command`; requires `uninstall_postflight` | ✓ |
| Full codegraph parity | Adds the mtime+size freshness baseline | |

**User's choice:** Version + completions, no freshness.
**Notes:** The mechanism itself was not re-decided — prior memory already establishes that `postflight` raising is the gate and `generate_completions_from_executable` cannot be, because `write_completion` rescues failures to a warning. Option 3's freshness logic was declined as guarding files it would itself be introducing.

### Q2 — Which shells, and where?

| Option | Description | Selected |
|--------|-------------|----------|
| bash + zsh + fish | Standard Homebrew triple | ✓ |
| zsh + fish only | Skips bash; Linux users mostly run bash and Linux is in scope | |
| All four, incl. powershell | Unreachable via Homebrew | |

**User's choice:** bash + zsh + fish.

### Q3 — How far does cask verification go?

This question was **rejected once for clarification**. The original framing asked how to prove the postflight gate fails an install (local rehearsal → red-evidence / CI job / both). The user responded with a principle rather than an option: *"we need to stop trying to test/red gate things we do not own."*

The question was reformulated around an explicit ownership boundary.

| Option | Description | Selected |
|--------|-------------|----------|
| Go tests + one real install | Test the command hard; install the cask once for real, report it | |
| Also assert the generated cask's content | Snapshot-assert our own template's output | |
| Go tests only | Cask reviewed in PR, shipped; nothing else gated | ✓ |

**User's choice:** Go tests only.
**Notes:** The original framing was wrong and the user corrected it — asserting that `brew install` raises is testing Homebrew's installer, not engram. This retired the CI-job option outright and reframed what "red evidence" means for this phase. A consequence was flagged once at the time of the choice and again immediately after: ROADMAP success criteria 3 and 5 are phrased as rehearsals and cannot be met as written under this decision. That tension was deferred into the next area rather than re-litigated here.

---

## Tap credential + re-ship proof

### Q1 — What credential writes the cask to the tap?

| Option | Description | Selected |
|--------|-------------|----------|
| Extend the existing GitHub App | Reuse release-please's App; no new secret, no expiry | ✓ |
| Separate fine-grained PAT | Narrowest scope; expires at 365 days with a silent delayed failure | |
| Separate GitHub App | Narrow and non-expiring; most moving parts | |

**User's choice:** Extend the existing GitHub App.
**Notes:** Requires a manual App re-scope in GitHub's UI — a setup step outside this repo, called out so the planner does not assume it is code-reachable.

### Q2 — How is "an explicit check before any real release depends on it" satisfied?

| Option | Description | Selected |
|--------|-------------|----------|
| One-shot permissions probe | `workflow_dispatch` job asserting `.permissions.push == true`; read-only | ✓ |
| Real write probe | Write + delete a scratch file; leaves commits in the tap's history | |
| Manual confirmation only | Check the UI, record it; nothing to re-run | |

**User's choice:** One-shot permissions probe.
**Notes:** Reconciled against the prior area's principle explicitly: this verifies our own configuration, not GitHub's documented behavior, so it is not the thing D-11 rejected.

### Q3 — How is a re-ship of an older tag prevented from regressing the tap's cask?

Before framing this question, GoReleaser's documentation was checked rather than asserted from memory. Two findings changed the options: `skip_upload` is a per-cask field that accepts templates (there is no `--skip=homebrew` CLI value), and the postflight belongs in `hooks.post.install` with `hooks.post.uninstall` as the uninstall analogue. The check also surfaced the regression itself, which had not been anticipated.

| Option | Description | Selected |
|--------|-------------|----------|
| skip_upload template guard | Compute newest-tag before GoReleaser; never write the cask on a backfill | ✓ |
| Reconcile after, like `:latest` | Structural parallel to the existing Docker step; tap transiently wrong | |
| Accept it — re-ship is rare | No guard; silent regression | |

**User's choice:** skip_upload template guard.
**Notes:** This is the same regression class `release.yaml` already documents for `docker pull …:latest` — backfilling v0.11.0 after v0.11.1 left `latest` serving the older build. Prevented by construction rather than repaired after.

### Q4 — How is the criteria-3/5 tension recorded?

| Option | Description | Selected |
|--------|-------------|----------|
| Reinterpret in CONTEXT.md | Record satisfaction-by-construction in the sanctioned channel | ✓ |
| Do the two rehearsals after all | One-time manual actions, reported not gated; reverses D-11 | |
| Amend the ROADMAP criteria text | Most honest end state; barred by rule `8dfdhfs5nn` and anti-pattern 15 | |

**User's choice:** Reinterpret in CONTEXT.md.
**Notes:** Option 3 was presented with its blocker stated rather than as a clean choice — there is no gsd-tools handler that rewrites success-criteria prose, so taking it would require an upstream gap report first.

---

## Claude's Discretion

- **D-08 (no `v` prefix)** — derived rather than asked, with an explicit opportunity to object before advancing.
- **Version-derivation helper shape** — file placement, function naming, and whether `lastRelease` lives in `version.go` or its own file are left to the planner, subject to the release-please `extra-files` path staying stable.

## Deferred Ideas

- Archive naming alignment with codegraph (adding a `v` prefix so both tools in the tap match). Cosmetic; D-08 makes it unnecessary for correctness.
- A platform conditional around the macOS-only quarantine strip for Linux cask installs. Raised in the final gray-area offer, not pursued.
- Reflecting `version`'s new flag in the self-describe catalog's per-command exit-code list beyond the golden-file update D-03 implies. Raised in the final gray-area offer, not pursued.
- Shipping completions as a first-class capability with its own test coverage, distinct from the cask writing them as a second exercise of the binary.
