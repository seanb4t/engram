# Phase 1: Version & Homebrew Distribution - Pattern Map

**Mapped:** 2026-08-23
**Files analyzed:** 8
**Analogs found:** 8 / 8 (all matched; 2 build/CI files are pattern-followed but deliberately not test-analogized per D-11/CLAUDE.md)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `cmd/engram/version.go` | CLI command (local, no dial) | request-response (in-process) | `cmd/engram/operator_output.go` + `cmd/engram/client_common.go` (composite) | role-match (deliberately diverges — see Divergence Rationale) |
| `cmd/engram/version_test.go` (new) | test | request-response | `cmd/engram/exitcode_baseline_test.go` (table shape) + `internal/config/client_validate_test.go`-style unit test (pure-function seam) | role-match |
| `cmd/engram/testdata/catalog.golden` | config (generated) | transform | itself (existing `version` entry, `flags: []`) | exact (regenerate via `task surfaces:gen`) |
| `cmd/engram/testdata/help.golden` | config (generated) | transform | itself (existing `version` block, lines 459-469) | exact (regenerate via `task surfaces:gen`) |
| `.goreleaser.yaml` (`homebrew_casks:` block) | config | batch/publish | `Casks/codegraph.rb` in `seanb4t/homebrew-tap` (cross-repo) + this file's own `dockers:`/`archives:` blocks for local YAML conventions | role-match |
| `.github/workflows/release.yaml` (newest-tag-before-goreleaser step + `verify-tap-credential` job) | CI workflow | event-driven | this file's own "Reconcile :latest after a re-ship" step (logic to lift) + its own App-token mint step (lines 32-36) | exact (same file, adjacent steps) |
| `release-please-config.json` (4th `extra-files` entry) | config | transform | itself (existing 3 `extra-files` entries) | exact |
| `internal/telemetry/config.go` (consumer, unchanged but referenced) | config | — | n/a — read-only integration point, no pattern needed | n/a |

## Pattern Assignments

### `cmd/engram/version.go` (CLI command, request-response)

**Current state** (full file, 16 lines):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the engram version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println(version)
	},
}
```

**Analog 1 — flag registration pattern to NOT copy verbatim:** `cmd/engram/client_common.go:51-53`
```go
f.String("output", config.FlagDefault("output"),
    `output format: "json" or "text" (default: detect from stdout)`)
```
`config.FlagDefault("output")` resolves to `""` (registry.go:94 has no `Default` row for `client.output`).
D-01 requires `version` to hardcode `"text"` instead — see Pitfall 1 below.

**Analog 2 — validator call + usage-error routing:** `cmd/engram/operator_output.go:44-48`
```go
func operatorOutputFormat(cmd *cobra.Command, v string) (outputFormat, error) {
	if err := config.ValidateOutputFormat(v); err != nil {
		return formatJSON, usageErrorf("%w", err)
	}
	return outputFormatFromConfig(v, isTTYWriter(cmd.OutOrStdout())), nil
}
```
Copy the `config.ValidateOutputFormat` → `usageErrorf` shape (D-03) exactly. Do **not** copy the
`outputFormatFromConfig`/TTY-detection call that follows it — `version` never calls
`outputFormatFromConfig` at all (D-01 forbids TTY detection).

**Validator being reused (unchanged):** `internal/config/client_validate.go:58-65`
```go
func ValidateOutputFormat(v string) error {
	switch v {
	case "json", "text", "":
		return nil
	default:
		return fmt.Errorf(`--output %q: must be "json", "text", or empty`, v)
	}
}
```

**Analog 3 — the render pair `version` must NOT call, cited for contrast:** `cmd/engram/operator_output.go:83-89`
```go
func renderOperator(cmd *cobra.Command, format outputFormat, headline string, doc any) error {
	if format == formatText {
		return renderOperatorView(cmd.OutOrStdout(), headline, doc)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	return enc.Encode(doc)
}
```
`renderOperatorView` always emits headline + blank line + padded rows — cannot produce a bare
scalar. D-07 requires `version` to write its own two-line pair instead (json.Encoder for the
json lane, a bare `fmt.Fprintln` for text), pinned by a test asserting the two stay equal.

**`version` var and ldflags contract (unchanged, consumed by the dev-build helper):** `cmd/engram/root.go:16-19`
```go
// version is the engram build version, injected at release time via
// -ldflags "-X main.version=X.Y.Z" (goreleaser). Defaults to "dev" for
// local/source builds.
var version = "dev"
```

---

### `cmd/engram/version_test.go` (new — unit test)

**Analog 1 — table-driven exit-code row shape:** `cmd/engram/exitcode_baseline_test.go:236-243`
```go
{
    name:    "root/legacy-env",
    args:    []string{"version"},
    env:     map[string]string{"MEM_QDRANT_ADDR": "old-host:1234"},
    before:  exitGeneric,
    after:   exitUsage,
    changes: true,
    landed:  true,
},
```
D-03 requires a new row here for `args: []string{"version", "--output", "bogus"}`, `introduced: true`,
`after: exitUsage` — see the `introduced` field's doc comment at `exitcode_baseline_test.go` lines
~43-47 (rows for capabilities that don't exist yet at baseline commit).

**Analog 2 — testable-seam pattern to mirror for the dev-build derivation helper:** `outputFormatFromConfig`
(`cmd/engram/client_common.go:198-208`) takes `isTTY bool` as a parameter specifically so a table
test can force both branches without a real TTY. RESEARCH.md's Validation Architecture section
names this explicitly as the pattern `resolvedVersion()`'s core logic should mirror: structure it as
a pure function `deriveDevVersion(lastRelease, revision string, dirty bool) string` (name at
planner's discretion) wrapped by a thin `resolvedVersion()` that calls `debug.ReadBuildInfo()` —
the pure function is what `version_test.go` exercises directly, never `ReadBuildInfo()` itself
(a `go test` binary has its own embedded build info, not an injectable fixture).

---

### `cmd/engram/testdata/{catalog,help}.golden` (generated config)

**Pattern:** these are tool-generated artifacts — do not hand-edit. Run `task surfaces:gen` after
`version.go`'s `--output` flag lands; review the diff to confirm it touches only `version`'s section.

**Existing `catalog.golden` entry to be extended** (lines 1131-1140):
```json
{
  "name": "version",
  "summary": "Print the engram version",
  "flags": [],
  "blast_radius": {
    "read_only": true,
    "destructive": false,
    "idempotent": true
  }
}
```
`ReadOnly`/`Idempotent` classification already exists and needs no change (per RESEARCH.md); only
`flags: []` grows an `output` entry.

**Existing `help.golden` block to be extended** (lines 459-469):
```
## engram version

```
Print the engram version

Usage:
  engram version [flags]

Flags:
  -h, --help   help for version
```
```
Gains an `--output` line inside the `Flags:` block. Line 29's `-v, --version   version for engram`
(cobra's built-in flag) is explicitly **unchanged** per D-02.

---

### `.goreleaser.yaml` (`homebrew_casks:` block)

**Analog — this repo's own YAML shape conventions** (existing `archives:`/`checksum:`, lines 35-42):
```yaml
archives:
  - id: default
    formats:
      - tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE
      - README.md

checksum:
  name_template: "checksums.txt"
```
Note: no `v` prefix in `name_template` — this is the existing precedent D-08 pins the cask against
(`engram_{Version}_{Os}_{Arch}`, no `v`).

**Cross-repo analog — `Casks/codegraph.rb` in `seanb4t/homebrew-tap`** (structure only; content
diverges per CONTEXT.md's "Specific Ideas" — no `man` subcommand, no `v`-prefixed archive names, no
quarantine strip in the precedent). The `hooks.post.install` / `hooks.post.uninstall` shape and the
`system_command binary, args: [...]` invocation-then-JSON-parse idiom is the reusable part; see
RESEARCH.md's Pattern 2/Pattern 3 for the full composed skeleton (GoReleaser docs + codegraph.rb
merged, adjusted for `#{staged_path}` vs `HOMEBREW_PREFIX/bin` ordering — Pitfall 2 — and the
`OS.mac?` guard — Pitfall 7).

**Skip-upload templating precedent:** no in-repo analog (new field); use RESEARCH.md's verified
`skip_upload: "{{ .Env.SKIP_HOMEBREW_UPLOAD }}"` form, fed by the workflow step below.

---

### `.github/workflows/release.yaml` (newest-tag-before-goreleaser step + credential-verify job)

**Analog 1 — the exact logic to lift and move earlier (currently runs AFTER goreleaser):**
"Reconcile :latest after a re-ship" step (lines ~148-166):
```yaml
- if: ${{ steps.target.outputs.ship == 'true' && github.event_name == 'workflow_dispatch' }}
  name: Reconcile :latest after a re-ship
  env:
    SHIPPED_TAG: ${{ steps.target.outputs.tag }}
  run: |
    set -euo pipefail
    newest=$(git tag -l 'v*' --sort=-v:refname | head -1)
    echo "shipped=${SHIPPED_TAG} newest=${newest}"
    if [ -z "$newest" ]; then
      echo "::error::no v* tags found — cannot verify :latest"
      exit 1
    fi
    if [ "$newest" = "$SHIPPED_TAG" ]; then
      echo ":latest correctly points at the newest tag — nothing to do."
      exit 0
    fi
    ...
```
D-14 reuses the `git tag -l 'v*' --sort=-v:refname | head -1` newest-tag computation, but as a NEW
step running BEFORE the `goreleaser-action` invocation (line ~131), exporting
`SKIP_HOMEBREW_UPLOAD=true|false` via `$GITHUB_ENV` for `.goreleaser.yaml`'s `skip_upload` template
to consume (Pitfall 6 — must be set before the GoReleaser step runs, GoReleaser cannot compute it
itself).

**Analog 2 — the App-token mint step to duplicate for the new `workflow_dispatch`-only job (D-13):**
lines 32-37:
```yaml
- uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3
  id: app-token
  with:
    app-id: ${{ secrets.RELEASE_APP }}
    private-key: ${{ secrets.RELEASE_APP_PRIVATE_KEY }}
```
D-12/D-13 extend this with a `repositories: engram,homebrew-tap` input — **both** repos named, not
just the tap (Pitfall 5: an unset default already narrows to "current repo only", and a
`repositories:` input listing only `homebrew-tap` would break the SAME job's release-please write
to `engram` itself). The `env:`-only token consumption pattern (never a CLI arg) is already
established at lines 41 and 137 — reuse it for the new `gh api` call.

**Analog 3 — GH_TOKEN-via-env pattern for `gh` CLI calls:**
```yaml
- name: Log in to GHCR (image)
  env:
    GHCR_USER: ${{ github.actor }}
    GHCR_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GHCR_USER" --password-stdin
```
Mirror this `env:`-block-then-`run:`-shell-script shape for D-13's `gh api
repos/seanb4t/homebrew-tap --jq .permissions.push` assertion (full skeleton already composed in
RESEARCH.md's Code Examples section, GH_TOKEN sourced from `steps.app-token.outputs.token`).

---

### `release-please-config.json` (4th `extra-files` entry)

**Analog — the 3 existing entries to match verbatim in shape:**
```json
"extra-files": [
  {
    "type": "yaml",
    "path": "charts/engram/Chart.yaml",
    "jsonpath": "$.version"
  },
  {
    "type": "yaml",
    "path": "charts/engram/Chart.yaml",
    "jsonpath": "$.appVersion"
  },
  {
    "type": "json",
    "path": "skill/engram/.claude-plugin/plugin.json",
    "jsonpath": "$.version"
  }
]
```
D-05 adds a 4th entry with `"type": "generic"` (not `yaml`/`json` — the Go const is neither),
`"path"` pointing at wherever the planner places `const lastRelease = "0.14.0" //
x-release-please-version` (root.go or a new file per Claude's Discretion). `always-update: true` is
already set at the package level (line 4) and needs no change.

---

## Shared Patterns

### Output-format validation (json/text vocabulary)
**Source:** `internal/config/client_validate.go:58-65` (`ValidateOutputFormat`)
**Apply to:** `version.go` only in this phase — the single exported validator every `--output` site
in the binary already shares (client tier via `ValidateClient`, operator tier via
`operatorOutputFormat`). `version` becomes the third call site, reusing the function but NOT the
TTY-detecting resolution that follows it elsewhere.

### Exit-code taxonomy (usage errors)
**Source:** `cmd/engram/client_common.go:215-236` (`exitUsage = 2` and friends) + `usageErrorf`
**Apply to:** `version.go`'s `--output bogus` path, `version_test.go`'s new baseline row. No new
exit code is introduced; `version` joins the existing `exitUsage` derivation.

### GitHub App token minting, env-only consumption
**Source:** `.github/workflows/release.yaml:32-37`, `:41`, `:137`
**Apply to:** D-13's new `verify-tap-credential` job — same action pin, same `env:`-block
consumption discipline (never a CLI arg), extended `repositories:` scope.

### "Compute before, template after" for GoReleaser-consumed decisions
**Source:** `.github/workflows/release.yaml`'s existing `target` resolver step (lines 44-72) — the
established idiom in this file of computing a decision into `$GITHUB_OUTPUT`/`$GITHUB_ENV` in one
step, then branching every later step's `if:`/`env:` off it, rather than re-deriving the same logic
in multiple places.
**Apply to:** D-14's newest-tag-before-goreleaser step, feeding `SKIP_HOMEBREW_UPLOAD` into
`.goreleaser.yaml`'s `skip_upload` template.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| Dev-build version derivation helper (new function, file placement at planner's discretion per Claude's Discretion) | utility | transform | Genuinely new capability — no existing code in this repo reads `debug.ReadBuildInfo()`. RESEARCH.md's Code Examples section (`resolvedVersion()` skeleton) is the reference implementation to adapt, not a codebase analog. The `.N` placeholder in `0.14.1-dev.N+g<hash>` is an explicitly unresolved Open Question (RESEARCH.md) the planner must decide (recommended: fixed `.0`). |
| `homebrew_casks: hooks.post.install`'s quarantine-strip step (`xattr -dr com.apple.quarantine`) | config (Ruby, generated) | file-I/O | `Casks/codegraph.rb` has NO quarantine strip at all (flagged explicitly in orienting notes as a gap, not an analog). Use GoReleaser's own documented `OS.mac?` + `#{staged_path}` example (RESEARCH.md Pattern 2) instead. |
| `verify-tap-credential` `workflow_dispatch` job as a whole | CI workflow (new job) | request-response | No existing job in this repo does a read-only cross-repo permission probe; composed fresh from the App-token mint pattern (Shared Patterns above) + a `gh api --jq` call. |

## Divergence Rationale (why `version.go` does not fully match any single analog)

Per CONTEXT.md's "Emergent pattern" note: `version` is this binary's first genuinely local command
(no server, no Qdrant, no network) and cannot fully inherit any of the three existing
flag-registration/render idioms:

1. `addClientFlags`/`addOperatorOutputFlag`'s TTY-detecting default — would break the unchanged-
   text-for-existing-callers guarantee (D-01).
2. `renderOperator`'s text-derived-from-JSON rendering — cannot emit a bare scalar (D-07).
3. Nothing to diverge from on the exit-code path — `version` fully inherits `exitUsage`/
   `usageErrorf`/`ValidateOutputFormat` unchanged (D-03).

Treat `version.go` as a fourth, minimal, hand-rolled registration site — documented as such in its
own comments, not as an oversight or inconsistency to "fix" toward one of the other two patterns.

## Metadata

**Analog search scope:** `cmd/engram/` (all `*.go` and `testdata/*.golden`), `.goreleaser.yaml`,
`.github/workflows/release.yaml`, `release-please-config.json`, `internal/config/client_validate.go`,
cross-repo `seanb4t/homebrew-tap:Casks/codegraph.rb` (read via `gh api` during research, not
re-fetched here — see RESEARCH.md Sources)
**Files scanned:** 8 in-repo + 1 cross-repo (via RESEARCH.md's prior fetch)
**Pattern extraction date:** 2026-08-23
