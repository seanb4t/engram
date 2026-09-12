# Phase 4: Skills Distribution - Pattern Map

**Mapped:** 2026-09-09
**Files analyzed:** 12 (new/modified)
**Analogs found:** 12 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/skills/embed.go` | config (embed directive) | file-I/O (compile-time) | `internal/webauth/static.go` (lines 1-20) | exact |
| `internal/skills/data/**` (vendored copy, git-tracked) | config/data | file-I/O | `internal/webauth/static/**` (vendored via `ui:build`) | exact |
| `internal/skills/drift_test.go` | test | file-I/O / set-equality | `internal/webauth/static_test.go:38` (the `all:` gotcha test) | exact |
| `internal/skills/inventory.go` | service (structural discovery) | transform (embed FS → skill list) | `cmdwalk.go:118`'s `operatorCommands()` structural-predicate idiom (cited in CONTEXT.md; not yet read this pass) + `fs.WalkDir` stdlib | role-match |
| `internal/skills/environment.go` | utility (filesystem seam) | request-response (func-field injection) | `internal/setup/environment.go` (lines 20-46) | exact |
| `internal/skills/agentsmd.go` | service (anchored splice) | transform / file-I/O | `internal/surfaces/anchor.go` (`scanAnchors` detection reusable; `WriteRegion` NOT reusable) | role-match (detection half exact, write half is an anti-pattern) |
| `internal/skills/install.go` | service (write orchestrator) | CRUD (idempotent write) | `internal/setup/apply.go` (D-08 byte-compare convergence pattern — not yet read this pass, cited in CONTEXT.md) + `internal/migrate/registry.go`'s `errors.Join` (lines 38-92) | role-match |
| `internal/skills/leafpurity_test.go` (optional, discretion) | test | static analysis | `internal/setup/leafpurity_test.go` (full file) | exact |
| `internal/setup/plan.go` (`SkillTarget` addition) | model (declarative struct) | CRUD (declared, not written) | `internal/setup/plan.go`'s own `Plan`/`Action`/`Outcome` (lines 1-160, same file) | exact |
| `internal/setup/{claudecode,codex,opencode,generic}.go` (each authors `SkillTarget`) | config (per-runtime authoring) | CRUD | `internal/setup/codex.go` (full file — `Plan()`'s per-runtime authored-here shape) | exact |
| `cmd/engram/setup.go` (composition: `SkillTarget` → `skills.Install`, new row fields) | controller (CLI command wiring) | request-response | `cmd/engram/setup.go`'s own `setupBuildRows`/`setupRuntimeRowFromResult` (lines 108-135, 264-283, same file) | exact |
| `Taskfile.yaml` (`skills:vendor` target) | config (build tooling) | batch (file copy) | `Taskfile.yaml` `ui:build` (lines 25-33) | exact |

## Pattern Assignments

### `internal/skills/embed.go` (config, file-I/O)

**Analog:** `internal/webauth/static.go`

**Full embed pattern** (verbatim, lines 1-20):
```go
package webauth

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// all: is required — a bare `//go:embed static` excludes files and directories
// whose names begin with "_" or ".", which would drop the SvelteKit build output
// under static/_app/ and leave the SPA unable to mount (GH #106).
//
//go:embed all:static
var staticFS embed.FS
```

**Apply to `internal/skills`:**
```go
//go:embed all:data
var skillsFS embed.FS
```
Same `all:` prefix, same rationale comment style (D-04 explicitly requires this — see
`internal/webauth/static_test.go:38` for the regression this prefix prevents). Use `fs.Sub` if a
package-relative view is convenient (see `static.go`'s `fs.Sub(staticFS, "static")` at line ~29),
but `internal/skills` likely wants to walk `"data"` directly via `fs.WalkDir` rather than `Sub`,
since D-04 wants structural discovery, not HTTP serving.

---

### `internal/skills/data/**` (vendored copy)

**Analog:** `internal/webauth/static/**` (the committed, git-tracked SvelteKit build output)

No code excerpt — the pattern is structural: a plain committed subtree under the package
directory, populated by a Taskfile target, never hand-edited. Same disposition applies here per
D-01/D-08: the vendored copy is byte-identical to `skill/engram/skills/`, never diverges.

---

### `internal/skills/drift_test.go` (test, set-equality)

**Analog:** `internal/webauth/static_test.go:38` — read the file directly since it is short; it is
the test proving the `all:` prefix isn't accidentally dropped. Structurally:

- Walk the embedded FS, collect the full set of relative paths.
- Walk `../../skill/engram/skills/` on disk, collect the full set of relative paths.
- **Set-equality first** (not containment — see D-02's explicit "never containment" invariant and
  memories `bqhy5v5hq9`/`8583e0yqa1`), then **byte-equality per path**.
- Fail loudly and name every offending path (added/removed/differing), not just the first.

This is a **new** test shape in this repo (no existing test does set-equality between an embed.FS
and its on-disk vendor source) — `static_test.go` proves the underlying `all:` mechanics work, but
does not itself assert the vendored copy matches a second, separate source tree. Build this test
fresh, borrowing only the walk-and-compare shape.

---

### `internal/skills/inventory.go` (service, structural discovery)

**Analog:** stdlib `fs.WalkDir` (pattern given in 04-RESEARCH.md's Code Examples section,
verified against `pkg.go.dev/io/fs`), combined with this repo's **structural-predicate-over-
enumeration** idiom.

```go
// Pattern (from RESEARCH.md, combines confirmed stdlib APIs — not verbatim repo code)
func discoverSkills(fsys embed.FS) ([]string, error) {
	var names []string
	err := fs.WalkDir(fsys, "data", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != "data" && filepath.Dir(path) == "data" {
			names = append(names, filepath.Base(path))
		}
		return nil
	})
	return names, err
}
```

**Invariant to carry over:** never hardcode "five" anywhere in this file or its tests (D-04). Grep
this repo's `cmdwalk.go:118` `operatorCommands()` for the established phrasing of this idiom before
writing the doc comment.

---

### `internal/skills/environment.go` (utility, filesystem seam)

**Analog:** `internal/setup/environment.go` (verbatim, lines 20-46)

```go
type Environment struct {
	LookPath func(file string) (string, error)
	Getenv   func(key string) string
	HomeDir  func() (string, error)
	Run      func(ctx context.Context, path string, args []string) (RunResult, error)
}

var OSEnvironment = Environment{
	LookPath: exec.LookPath,
	Getenv:   os.Getenv,
	HomeDir:  os.UserHomeDir,
	Run:      osRun,
}
```

**Apply to `internal/skills`:** a **narrower** struct-of-func-fields — never an interface, per this
confirmed precedent. `internal/skills` needs `HomeDir` (D-10's every-destination-derivation seam),
plus `ReadFile`, `WriteFile`, `MkdirAll`, `Stat` (D-08's byte-compare, D-16's read-whole-then-write).
No `LookPath`/`Run`/`Getenv` needed — this package never execs a subprocess. Real implementation
(`OSEnvironment`-equivalent) wires `os.ReadFile`, `os.WriteFile`, `os.MkdirAll`, `os.Stat`,
`os.UserHomeDir` directly, mirroring the doc-comment style: cite `cliNow`
(`cmd/engram/destructive.go`) and `citationFileReader` (`cmd/engram/spine_review_verify.go`) as the
struct-of-func-fields precedent family, exactly as `environment.go`'s own doc comment does.

---

### `internal/skills/agentsmd.go` (service, anchored splice)

**Analog:** `internal/surfaces/anchor.go` — **detection reusable, write NOT reusable.**

**Reusable: the anchor literal + scan state machine shape** (verbatim, lines 20-24 and the
`scanAnchors` function, lines 60-110):
```go
htmlStart = "<!-- engram:rule:start " + ruleID + " -->"
htmlEnd = "<!-- engram:rule:end " + ruleID + " -->"
```
Adapt to a **fixed, parameterless** marker pair for the single whole-file skills block (no
per-rule-ID parameterization needed — there is exactly one skills block per AGENTS.md):
`<!-- engram:skills:start -->` / `<!-- engram:skills:end -->`, matching the existing
`engram:<domain>:start`/`:end` naming convention.

`scanAnchors`'s `sawStart`/`sawEnd`/`pending *anchorPair` state machine (lines 60-110) is the shape
to imitate for D-15's absent/well-formed/malformed trichotomy: it already distinguishes "neither
seen" (absent) from "seen but ill-paired" (malformed), and already rejects three distinct malformed
shapes with a specific error each (second start before matching end; end with no open start;
unterminated start at EOF). **Gap to close:** it reports these as file-path + rule-ID text, not
byte/line offsets — D-15 requires the offending marker's offset in `Reason`, so capture
`startLineIdx`/`endLineIdx` (already tracked internally, lines 45-50, just not surfaced) into the
returned error.

**NOT reusable — `WriteRegion`'s two behaviors that are the opposite of what D-15/D-16 need**
(verbatim, lines 152-176 and 195-224):
```go
// WriteRegion rewrites the body inside EVERY one of rule ruleID's anchor
// pairs in path ... (multi-pair-tolerant — the OPPOSITE of D-15, which
// requires 2+ pairs to hard-fail)
func WriteRegion(path, ruleID, body string) error { ... }

// writeFileAtomic writes content to path via a same-directory temp file and
// os.Rename ... (atomic-rename — the OPPOSITE of D-16, which requires
// in-place os.WriteFile so a symlinked AGENTS.md is written THROUGH, never
// replaced)
func writeFileAtomic(path, content string) error { ... }
```
**Do not import `internal/surfaces` for the write path.** Reimplement a ~50-line, single-fixed-pair
detection scan locally in `internal/skills` (per RESEARCH.md Open Question 3's recommendation),
and write via a single `os.WriteFile` call over the whole spliced content — never
`os.CreateTemp`+`os.Rename`.

---

### `internal/skills/install.go` (service, write orchestrator)

**Analog (failure accumulation):** `internal/migrate/registry.go`'s `Validate` (verbatim, line 92):
```go
return errors.Join(errs...)
```
Used here to accumulate every violation across a full pass rather than stopping at the first
(verbatim pattern at lines 60-90: build up `var errs []error`, `errs = append(errs, fmt.Errorf(...))`
per independent check, return `errors.Join(errs...)` once). Apply directly to D-07: registration and
skills attempted independently per runtime, every skill attempted, each failure accumulated into
the row's `Reason`.

**Analog (declarative-then-execute split):** `internal/setup/plan.go`'s `Plan`/`Action`/`Outcome`
doc comments (same file, lines 1-160) — read in full for the "explicit value, never absence or
zero value" idiom that `SkillTarget`'s `Kind` discriminator must follow, and for `Action.Tolerant`
as the precedent for "authored explicitly, never inferred from position" (memory `w21by88q6e`).

**Byte-compare convergence (D-08):** not directly excerpted this pass — `internal/setup/apply.go`
is cited in CONTEXT.md as the D-08 precedent for Phase 3's own already-correct/wrote convergence
check; read it before implementing `Install`'s per-file byte-compare so the two convergence checks
(registration vs. skills) use a structurally similar decision shape.

---

### `internal/setup/leafpurity_test.go` → new `internal/skills/leafpurity_test.go` (discretion)

**Analog:** `internal/setup/leafpurity_test.go` (full file, verbatim reusable almost as-is)

Key excerpt (lines 82-90):
```go
func TestSetupPackageIsStdlibOnlyLeaf(t *testing.T) {
	files := nonTestGoFiles(t, ".")
	if len(files) == 0 {
		t.Fatal("scanned zero non-test .go files in internal/setup — a scan matching nothing is vacuously green, which is a defect in the scan, not evidence the package is pure")
	}
	mod := modulePath(t)
	...
```
If `internal/skills` gets its own leaf-purity gate (Claude's Discretion per CONTEXT.md — `embed` and
`io/fs` are stdlib, so it CAN be a leaf), copy this file's `findGoMod`/`modulePath`/`nonTestGoFiles`
helpers verbatim and retarget the package-scan. Note: `internal/skills` will need `os`/`path/filepath`
for its filesystem seam, which is stdlib-fine — the same-module-import check (the second half of
this test) is the one that matters, since `internal/skills` must never import `internal/setup`
(D-05's declare/install split is one-directional: `cmd/engram` composes both, but neither internal
package imports the other).

---

### `internal/setup/plan.go` (`SkillTarget` addition)

**Analog:** the same file's existing `Outcome`/`Action`/`Plan` doc-comment style (lines 25-160)

Key excerpt — the "explicit value, never absence" doc-comment idiom to match (lines 27-30):
```go
// Outcome classifies the result of planning (or, once Apply lands,
// actually performing) one runtime's registration. It is a five-value
// enum, string-backed for readable JSON/text rendering. OutcomeNotPresent
// is a REAL, explicit value — never modeled as an absence or the zero
// value (D-07): ...
```
`SkillTarget` should follow this same discipline: an explicit `Kind` discriminator (including an
explicit "no skills" case for `generic`, per D-11), never two optional string fields whose presence
is inferred. Add `SkillTarget` as a new field on `Plan` (same shape-growing precedent as `Config`,
lines ~130-138 of the same file), authored per-runtime in each runtime's own file — never a
runtime-agnostic default (the package doc comment's AUTHORED-HERE invariant, restated at the top of
`plan.go`, lines 12-16).

---

### `internal/setup/{claudecode,codex,opencode,generic}.go` (`SkillTarget` authoring)

**Analog:** `internal/setup/codex.go` (full file, verbatim)

```go
// codexRuntime implements Runtime for Codex, authoring the live-verified
// `codex mcp add <NAME> --url <URL>` invocation surface ...
type codexRuntime struct{}

var Codex Runtime = codexRuntime{}

func (codexRuntime) Name() string { return "codex" }

func (codexRuntime) Detect(env Environment) bool {
	_, err := env.LookPath("codex")
	return err == nil
}

func (codexRuntime) Plan(_ Environment, opts Options) (Plan, error) {
	probe := []string{"codex", "mcp", "get", "engram", "--json"}
	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{ ... }},
			Probe: probe,
		}, nil
	...
	}
}
```

Each of `claudecode.go`/`codex.go`/`opencode.go`/`generic.go` adds its own `SkillTarget` value
inside its existing `Plan()` method, in the same file, following this exact "authored here, no
special-casing by name" shape. Per D-14/RESEARCH.md's Codex-destination open question, `codex.go`'s
`SkillTarget` should target `$HOME/.agents/skills` (per this session's locked decision — see
`04-CONTEXT.md`'s "Decisions locked THIS SESSION" note), NOT `$CODEX_HOME/skills`. `generic.go`'s
`SkillTarget` is D-11's "no skills" `Kind` case carrying a destination-agnostic payload instead of a
filesystem destination.

---

### `cmd/engram/setup.go` (composition: `SkillTarget` → `skills.Install`, new row fields)

**Analog:** the same file's existing `setupBuildRows`/`setupRuntimeRowFromResult` (lines 108-135,
264-283)

```go
// Source: cmd/engram/setup.go:126-135 (verbatim)
func setupBuildRows(ctx context.Context, env setup.Environment, runtimes []setup.Runtime, opts setup.Options) []setupRuntimeRow {
	rows := make([]setupRuntimeRow, 0, len(runtimes))
	for _, rt := range runtimes {
		rows = append(rows, setupRuntimeRowFromResult(setup.Preview(ctx, env, rt, opts)))
	}
	return rows
}
```

This is the **composition point** D-05 requires: `cmd/engram` is the only package permitted to
import both `internal/setup` and `internal/skills`. Extend this loop (or add a parallel step
alongside it) to also call `skills.Install(skillsEnv, rt.Plan(...).SkillTarget)` per runtime, and
fold its result into the SAME row via D-06's aggregation precedence
(`failed > wrote > already-correct > would-write > not-present`) rather than adding a second row.

**Row struct extension pattern** — `setupRuntimeRow`'s doc comment (lines 64-77) already states the
governing constraint for every new field this phase adds:
```go
// setupRuntimeRow is one runtime's row in setupReportDoc.Runtimes,
// rendered through renderOperator with zero bespoke rendering code
// (D-15): viewRow already renders each element as a dense
// "key=value ..." line, and sanitizeViewValue already strips control
// characters from any string field here ...
```
New fields (`registration=`, `skills=`, destination, `digest=`, byte count) are plain `string`
fields with `json:"...,omitempty"` tags on the existing `setup.Result`-shaped row struct — never a
nested object/array (that bypasses `sanitizeViewValue`'s scalar-only branch, per the same doc
comment's warning about `Config` two paragraphs later, and `TestOperatorViewFixturesHaveNoUnsanitizedNesting`).

**Rendering (no new code needed):** `viewRow`/`sanitizeViewValue` (`cmd/engram/operator_view.go`
lines 161-234) already render any new `key=value` field with zero changes — confirmed by reading
both functions this session. `sanitizeViewValue` strips only C0/DEL control characters (memory
`wvpxqrd5m0`) — every shell metacharacter passes through untouched, so a digest (hex string) and a
byte count (decimal string) are safe by construction, but a raw skill `name`/`summary` value
rendered into a row (if ever done) would not be shell-metacharacter-safe.

---

### `Taskfile.yaml` (`skills:vendor` target)

**Analog:** `Taskfile.yaml` `ui:build` (verbatim, lines 25-33)

```yaml
ui:build:
  desc: Build the SvelteKit SPA and vendor it into internal/webauth/static
  dir: ui
  cmds:
    - pnpm install --frozen-lockfile
    - pnpm build
    - rm -rf ../internal/webauth/static
    - mkdir -p ../internal/webauth/static
    - cp -R build/. ../internal/webauth/static/
```

**Apply to skills (simpler — no build step, per D-01/RESEARCH.md):**
```yaml
skills:vendor:
  desc: Vendor skill/engram/skills into internal/skills/data
  cmds:
    - rm -rf internal/skills/data
    - mkdir -p internal/skills/data
    - cp -R skill/engram/skills/. internal/skills/data/
```
Whether this joins `task default` is the planner's call per CONTEXT.md; D-02's Go-test verifier
already runs inside `task test` regardless of whether the vendor step itself is wired into a
default task chain.

## Shared Patterns

### Struct-of-func-fields filesystem/exec seam (never an interface)
**Source:** `internal/setup/environment.go` (lines 20-70)
**Apply to:** `internal/skills/environment.go` (new, narrower version: `HomeDir`, `ReadFile`,
`WriteFile`, `MkdirAll`, `Stat`)
```go
type Environment struct {
	LookPath func(file string) (string, error)
	Getenv   func(key string) string
	HomeDir  func() (string, error)
	Run      func(ctx context.Context, path string, args []string) (RunResult, error)
}
var OSEnvironment = Environment{ LookPath: exec.LookPath, Getenv: os.Getenv, HomeDir: os.UserHomeDir, Run: osRun }
```
This is the repo-standard injectable seam (also modeled on `cliNow` in `cmd/engram/destructive.go`
and `citationFileReader` in `cmd/engram/spine_review_verify.go`) — repo rule `m45p2b4bp7` (never
gate on third-party behavior) is precisely why every I/O boundary in this milestone is expressed
this way rather than hit directly in test.

### Independent-failure accumulation via `errors.Join`
**Source:** `internal/migrate/registry.go` (`Validate`, lines 60-92)
**Apply to:** `internal/skills/install.go` (D-07: registration and skills attempted
independently per runtime; every skill attempted; each failure accumulated into `Reason`)
```go
var errs []error
// ... errs = append(errs, fmt.Errorf(...)) per independent check ...
return errors.Join(errs...)
```

### Vendor-then-`//go:embed all:` (with the drift-preventing rationale comment)
**Source:** `internal/webauth/static.go` (lines 1-20), `Taskfile.yaml` `ui:build` (lines 25-33)
**Apply to:** `internal/skills/embed.go`, `Taskfile.yaml` `skills:vendor`
```go
//go:embed all:data
var skillsFS embed.FS
```
The `all:` prefix is load-bearing (D-04) — carry forward the same explanatory comment style citing
the concrete regression it prevents (`static_test.go:38`, GH #106 in the analog; this phase's own
drift test is the equivalent guard).

### Dense `key=value` row rendering, zero new rendering code
**Source:** `cmd/engram/operator_view.go` (`viewRow` lines 161-181, `sanitizeViewValue` lines
223-234), `cmd/engram/setup.go` (`setupRuntimeRow` doc comment, lines 64-77)
**Apply to:** every new field D-03/D-06/D-11 add — plain string fields with `json:",omitempty"`
tags on the row struct, never a nested object/array in the text-row path (D-03 explicitly routes
full skill bytes to the `--output json` lane instead, exactly to avoid this trap).

### Explicit value, never absence or zero value
**Source:** `internal/setup/plan.go` (`Outcome` doc comment, lines 25-30)
**Apply to:** `SkillTarget`'s `Kind` discriminator (including an explicit "no skills" case for
`generic`, D-11) and any new enum this phase introduces.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/skills/drift_test.go`'s set-equality-then-byte-equality assertion shape | test | file-I/O comparison | No existing test in this repo compares an `embed.FS` against a second, separate on-disk source tree for full set+byte equality — `static_test.go` proves `all:` mechanics work but does not cross-check against a second tree. Build fresh from RESEARCH.md's Code Examples + the walk-and-compare description above; this is the single highest-value new test per RESEARCH.md's Wave 0 Gaps. |
| Frontmatter (YAML) parsing at runtime for the AGENTS.md index line | utility (parser) | transform | No YAML-parsing code exists anywhere in this codebase reachable from the shipped binary today (`go mod why` confirms both `gopkg.in/yaml.v3` and `go.yaml.in/yaml/v3` are build-tool-only via `buf`/`cobra/doc`). This is an open planner/checkpoint decision per RESEARCH.md Open Question 2 (real YAML parser promoted to direct dependency, vs. a minimal hand-rolled `---`-fenced line scanner) — no in-repo analog to copy either way. |

## Metadata

**Analog search scope:** `internal/webauth/`, `internal/surfaces/`, `internal/setup/`,
`internal/migrate/`, `internal/authz/`, `cmd/engram/setup.go`, `cmd/engram/operator_view.go`,
`Taskfile.yaml`
**Files scanned:** 9 read in full this session (`static.go`, `anchor.go`, `environment.go`,
`plan.go`, `leafpurity_test.go`, `codex.go`, `registry.go` (partial), `setup.go` (partial),
`operator_view.go` (partial)) + `Taskfile.yaml` excerpt via RESEARCH.md's prior verified reads
**Pattern extraction date:** 2026-09-09
