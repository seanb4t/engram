# Phase 1: Gate & CI Integrity - Research

**Researched:** 2026-08-13
**Domain:** Go stdlib regex/testing (key-link guard) + GitHub Actions CI (Qdrant testcontainer stability)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Key-Link Fix Locus**
- **D-01:** Fix is repo-local only. `gsd-core`'s `parseMustHavesBlock` is NOT patched. Report the
  unescaping gap upstream per rule `8dfdhfs5nn`; filing vs. spine-tracking (precedent `cvvrwjbsnz`)
  is Sean's call, not the executor's.
- **D-02:** Normalized patterns use the escape-free character-class form — `[.]`, `[(]`, `[)]` —
  never `\.` or `\\.`. This is the form v0.13.x Phase 3 already adopted in `ca8d337c`.
- **D-03:** The guard covers BOTH silent-no-op shapes: (a) `\\` escaping, and (b) a valid, correctly
  escaped, but unsatisfiable pattern (#479's second finding — `addApplyFlag[(]` pinned on a file
  whose leaf routes through `registerDestructive`, so the symbol never appears in `from` at all).
- **D-04:** The two halves of the guard have DIFFERENT scopes. Escaping check: time-invariant, runs
  over EVERY plan in `.planning/` including archived milestones. Satisfiability check: depends on
  code as it stands now, runs ONLY against the active milestone (`.planning/phases/**`).

**Guard Shape & Fail-First Proof**
- **D-05:** Guard lives in a new stdlib-only leaf package, `internal/keylinks` (name indicative),
  exercised by a Go test inside `go test ./...`. Mirrors `internal/surfaces` / `internal/openaiurl`.
  No new toolchain, no new CI job.
- **D-06:** Fail-first proven by a committed good/bad fixture pair in testdata: known-good
  `key_links` block asserts GREEN, known-corrupted asserts RED.
- **D-07:** On failure, guard reports EVERY offender in one run: `file:line`, which shape failed,
  and the corrected character-class form. Not fail-fast-on-first.
- **D-08:** Patterns restricted to the RE2 ∩ JavaScript common subset — guard rejects
  backreferences, lookaround, AND named groups, not merely malformed patterns.

**Reassessment Scope & Archived Plans**
- **D-09:** ALL 38 offending patterns normalized repo-wide, not only the 25 in v0.13.x Phases 1–2.
- **D-10:** Archived, shipped `PLAN.md` files edited in place, in ONE commit, rationale in the
  commit message, no inline annotation in the documents.
- **D-11:** "Genuinely pinned" = the corrected pattern resolves against its `from` file at HEAD.
  Match → pinned. No match → recorded unpinned with the reason.
- **D-12:** The reassessment is a one-time sweep, distinct from the recurring guard.
- **D-13:** Verdicts land as a table in this phase's own `VERIFICATION.md`.
- **D-14:** A gate found unpinned is recorded only — NOT repaired in this phase.

**Qdrant CI Mitigation**
- **D-15:** CI runs one shared Qdrant, sets `ENGRAM_QDRANT_TEST_ADDR`. `TestMain` already honors
  this as its fast path. Serialization (`-p 1`) and a larger runner both rejected.
- **D-16:** Collections namespaced by a per-package constant prefix (e.g. `store_mem_eval_test` /
  `server_mem_eval_test`). Per-test unique names rejected as too wide a diff.
  `internal/store/reindex_test.go`'s `src`/`tgt` pairs need the same prefix treatment.
- **D-17:** ALL FOUR Qdrant-backed packages move onto the shared instance: `internal/store`,
  `internal/server`, `internal/e2e`, `internal/retrievaleval`.
- **D-18:** Per-package testcontainer boot path STAYS as fallback (env var → testcontainer → skip).
- **D-19:** Container exit reason captured by a CI post-step on failure (`if: failure()`) dumping
  container state, exit code, logs, and `dmesg` OOM evidence. In-Go capture rejected as primary.
- **D-20:** Fix proven by asserting the MECHANISM (one container; all four packages resolve to the
  same address; collection-name prefixes provably disjoint), not repeated-green-runs.

### Claude's Discretion
No area was answered "you decide" — every question resolved to an explicit choice. Open to
planning judgment: the exact package name for the guard leaf (`internal/keylinks` is indicative),
the precise prefix strings in D-16, and the specific shape of the CI service container definition
vs. a plain boot step.

### Deferred Ideas (OUT OF SCOPE)
- Fixing `parseMustHavesBlock` in gsd-core — reported upstream per D-01, not fixed here.
- Repinning v0.13.x gates found unpinned — recorded (D-14), repaired in none.
- Per-test collection isolation (`t.Name()` / random suffixes) — revisit only if intra-package
  parallelism is ever wanted.
- Pinned-commit resolution for archived key-links (resolving at ship commit, not HEAD).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-keylink-pattern-matchable | `pattern:` field actually compiles AND matches; `\\`-escaping eliminated repo-wide; guard detects reintroduction | Verified defect mechanism in `verify.cjs`/`frontmatter.cjs`; empirically verified which shapes Go's `regexp` package rejects for free vs. requires explicit checks (see Code Examples) |
| REQ-keylink-past-gates-reassessed | v0.13.x Phases 1–2 key-links re-resolved against the tool; genuinely pinned or recorded unpinned | D-11's HEAD-resolution definition; sweep tooling reuses the same matcher as the guard (D-12) |
| REQ-ci-qdrant-container-stability | `go test ./...` no longer fails from testcontainer death; exit reason captured on failure | Verified `TestMain` precedence chain across all 4 packages; verified exact collection-name collisions; verified GitHub Actions `services:` mechanics and Qdrant health-check endpoints |
</phase_requirements>

## Summary

This phase has almost no external-library research surface — it is entirely a fix against this
repo's own Go stdlib test infrastructure and CI YAML, plus a new stdlib-only Go package. The
CONTEXT.md decisions already resolve every architectural question; what remained to research was
**empirical verification of exact tool behavior** so the plan can specify implementation precisely
rather than approximately.

Three things were directly verified this session, not assumed: (1) `verify.cjs`'s `new
RegExp(link['pattern'])` call at `verify.cjs:1100` receives the pattern string completely
unprocessed by `frontmatter.cjs`'s `parseMustHavesBlock` (it only strips leading/trailing quote
characters, never backslash-unescapes) — confirming the defect mechanism exactly as CONTEXT.md
describes it; (2) which JS-only regex constructs Go's RE2-based `regexp` package rejects for free
at compile time versus which require explicit guard logic — this was probed empirically with a
throwaway Go program in this session, not recalled from training data, and the results are
load-bearing for how the `internal/keylinks` guard must be built (see Code Examples); (3) the exact
set of hardcoded, colliding Qdrant collection names across all four affected test packages, plus
confirmation that `internal/e2e` already uses a collision-safe per-instance naming scheme and needs
no D-16 remediation, narrowing the real touch surface to `internal/store` and `internal/server`.

**Primary recommendation:** Build `internal/keylinks` as two independent stdlib-only checkers
(escaping-shape string scan + satisfiability-shape regex-and-grep) sharing one pattern-validation
core built on Go's `regexp` + `re.SubexpNames()`; for CI, add a `services:` Qdrant container to the
`test` job (port 6334 mapped, `--health-cmd` against `/readyz` on 6333) and rename only two
packages' hardcoded collection strings (`internal/store`, `internal/server`) — `internal/e2e` and
`internal/retrievaleval` need no code change, only inclusion in the shared-address wiring.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Key-link pattern escaping/satisfiability guard | Go stdlib leaf package (`internal/keylinks`) | `go test ./...` (CI) | Mirrors `internal/surfaces`'s established pattern: a conformance gate that is itself ordinary Go code, imported by nothing but exercised by a test, so it rides the existing CI gate with zero new infrastructure |
| One-time v0.13.x Phase 1–2 key-link reassessment | Ad hoc sweep script/test reusing `internal/keylinks`'s matcher | This phase's `VERIFICATION.md` (output) | Not a persistent capability — a one-time audit whose *tooling* lives in the leaf package but whose *output* is a document, not code |
| Archived `.planning/**` pattern rewrites | `.planning/**` markdown (data, not code) | — | Mechanical text edits; no runtime component |
| Qdrant CI stability | GitHub Actions workflow (`ci.yaml`) | `internal/{store,server,e2e,retrievaleval}` test setup (Go) | The resource-pressure fix is a CI topology change (one container vs. four); the collision fix is Go test code (collection-name constants) — two tiers, one mitigation |
| Container-death diagnosability | GitHub Actions post-step (`if: failure()`) | — | Must run outside the Go process since the process itself may already be gone when the container dies (D-19's rejection of in-Go `Logs()`/`State()` as primary) |

## Standard Stack

### Core
No new external dependencies. This phase uses only:

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Go `regexp` / `regexp/syntax` | stdlib (go 1.26.3, per `go.mod:3`) | Pattern compilation + satisfiability checking in `internal/keylinks` | RE2 engine — guaranteed linear-time, immune to catastrophic backtracking (ReDoS), unlike JS's backtracking engine. `[VERIFIED: empirical probe, this session]` |
| Go `testing` (stdlib) | go 1.26.3 | Guard's own test, fixture-pair fail-first proof | Every leaf package in this repo (`internal/surfaces`, `internal/openaiurl`) uses stdlib-only `testing`, no `testify`, confirmed by reading `internal/surfaces/conformance_test.go` and `internal/openaiurl/openaiurl_test.go` this session `[VERIFIED: internal/surfaces/conformance_test.go, internal/openaiurl/openaiurl_test.go]` |
| GitHub Actions `services:` | n/a (workflow syntax) | Shared Qdrant container for the `test` job | Native GHA mechanism for exactly this use case (one container the whole job can reach); no third-party action needed `[CITED: docs.github.com/en/actions — About service containers]` |

### Supporting

| Component | Purpose | When to Use |
|-----------|---------|-------------|
| `qdrant/qdrant:v1.18.2` (same tag already pinned) | The shared CI Qdrant instance | Reuse `internal/store/store_test.go:33`'s `qdrantImageTag` constant value for the `services:` image so the shared container and the per-package testcontainer fallback stay version-locked `[VERIFIED: internal/store/store_test.go:27-33]` |
| `testcontainers-go/modules/qdrant` v0.43.0 (already a dependency) | Per-package fallback boot path (D-18) | Unchanged — this phase does not touch this path, only adds a faster CI-preferred path in front of it |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| GHA native `services:` container | A manual `docker run -d` step + `docker exec` health poll | `services:` is declarative, gets automatic teardown, and (with `--health-cmd`) gets automatic job-start gating for free; a manual step needs hand-rolled readiness polling — CONTEXT.md's discretion note explicitly leaves this choice open, but `services:` is the standard-library equivalent for CI |
| `re.SubexpNames()` for named-group rejection | Walking the `regexp/syntax.Regexp` AST for `OpCapture` nodes with non-empty `Name` | `SubexpNames()` is public API, one line, and was empirically confirmed this session to catch BOTH Go's `(?P<name>...)` and the JS-style `(?<name>...)` syntax (Go 1.26 accepts both at compile time) — no need for the lower-level `regexp/syntax` package |

**Installation:** none — no new `go.mod` entries.

**Version verification:** `go.mod:3` pins `go 1.26.3`; this session's `go version` reports
`go1.26.5 darwin/arm64` locally, consistent within the module's toolchain floor. No package
registry lookup applies (stdlib only).

## Package Legitimacy Audit

Not applicable — this phase installs no external packages (stdlib + already-vendored
`testcontainers-go`/`qdrant/go-client`, both pre-existing `go.mod` dependencies untouched by this
phase).

## Architecture Patterns

### System Architecture Diagram

```
 .planning/**/*.md (plan files, incl. archived milestones)
        │
        │  read at CI time (go test ./...)
        ▼
 ┌─────────────────────────────┐
 │ internal/keylinks (new)     │
 │                              │
 │  parseKeyLinks(file) ──────┐│
 │        │                   ││
 │        ▼                   ││
 │  ┌─────────────┐  ┌────────▼┴──────┐
 │  │ escaping    │  │ satisfiability │
 │  │ check       │  │ check          │
 │  │ (ALL plans, │  │ (active        │
 │  │ D-04)       │  │ milestone only,│
 │  │             │  │ D-04)          │
 │  └──────┬──────┘  └────────┬───────┘
 │         │                  │
 │         ▼                  ▼
 │   RE2-compile + no-backslash   RE2-compile +
 │   + no-named-group (D-08)      regex.Find against
 │                                 `from:` file content
 └────────────┬─────────────────────────┬──────┘
              │  offenders (file:line,   │
              │  shape, corrected form)  │
              ▼                          │
         t.Errorf (D-07: all in one run) │
              │                          │
              ▼                          ▼
        go test ./... FAILS      (reused by one-time
        (blocks CI merge)         v0.13.x sweep, D-12,
                                   output -> VERIFICATION.md
                                   table, D-13)


 CI test job (.github/workflows/ci.yaml)
        │
        ▼
 ┌───────────────────────────────────────────┐
 │ services:                                  │
 │   qdrant: image qdrant/qdrant:v1.18.2      │
 │     ports: 6334:6334 (+6333 for healthcheck)│
 │     options: --health-cmd curl /readyz     │
 └───────────────┬─────────────────────────────┘
                 │  ENGRAM_QDRANT_TEST_ADDR=localhost:6334
                 ▼
 ┌─────────────────────────────────────────────────────────┐
 │ go test ./...  (all packages run concurrently)            │
 │                                                             │
 │  internal/store    TestMain → env var fast path → dial ───┤
 │  internal/server    TestMain → env var fast path → dial ───┤──▶ ONE shared
 │  internal/e2e        TestMain → env var fast path → dial ───┤    Qdrant
 │  internal/retrievaleval TestMain → env var fast path → dial─┘    instance
 │                                                             │
 │  collections namespaced per package:                       │
 │   store_mem_eval_test / server_mem_eval_test / e2e_<port>  │
 │   (already unique) / retrievaleval_<uuid> (already unique) │
 └───────────────┬─────────────────────────────────────────────┘
                 │ on failure only
                 ▼
        if: failure() post-step
        dumps container state, exit code, logs, dmesg (D-19)
```

### Recommended Project Structure
```
internal/keylinks/
├── keylinks.go          # ParsePlanKeyLinks, CheckEscaping, CheckSatisfiability
├── keylinks_test.go      # the guard's own test — walks .planning/**, asserts zero offenders
└── testdata/
    ├── good_key_links.md   # known-good fixture (D-06)
    └── bad_key_links.md    # known-corrupted fixture — proves fail-first (D-06)
```

### Pattern 1: Compile-time RE2 rejection you get for free
**What:** Go's `regexp.Compile` (RE2) rejects lookahead, lookbehind, and backreferences at parse
time with a distinct, matchable error string — no custom detection code needed for these three
JS-only constructs.
**When to use:** In the guard's per-pattern validation step, treat any `regexp.Compile` error as an
offender in its own right (a plan author wrote a JS-only construct); do not special-case these
three, since the stdlib already rejects them.
**Example:**
```go
// Source: empirically verified this session via a throwaway `go run` probe
// against go1.26.5 — see Common Pitfalls for why this cannot be assumed from
// training data alone.
_, err := regexp.Compile(`(?=foo)`)
// err: "error parsing regexp: invalid or unsupported Perl syntax: `(?=`"

_, err = regexp.Compile(`(?!foo)`)
// err: "error parsing regexp: invalid or unsupported Perl syntax: `(?!`"

_, err = regexp.Compile(`(?<=foo)`)
// err: "error parsing regexp: invalid named capture: `(?<=foo)`"

_, err = regexp.Compile(`(foo)\1`)
// err: "error parsing regexp: invalid escape sequence: `\1`"
```

### Pattern 2: What compile-time rejection does NOT catch — needs explicit checks
**What:** Two of D-08's required rejections are NOT free. `\\.` (the escaping-defect shape itself)
compiles successfully under RE2 with the SAME wrong semantics as under JS ("literal backslash, then
any char") — so RE2 compiling without error is not evidence the pattern is escape-clean. Named
groups, in BOTH Go's `(?P<name>...)` syntax and the JS `(?<name>...)` syntax, also compile
successfully under Go 1.26 (this repo's toolchain) — Go added `(?<name>...)` as an accepted alias.
**When to use:** Every pattern, after successful compile.
**Example:**
```go
// Source: empirically verified this session — go1.26.5 darwin/arm64.
re, _ := regexp.Compile(`\\.`)          // compiles; means "\ then any char", NOT ".":
                                          // the exact silent-no-op shape D-02/D-03 exist to catch
re2, _ := regexp.Compile(`(?P<name>foo)bar`)
fmt.Println(re2.SubexpNames())          // [ name] — non-empty entry at index 1 = named group
re3, _ := regexp.Compile(`(?<name>foo)bar`) // JS syntax — Go 1.26 ALSO accepts this
fmt.Println(re3.SubexpNames())          // [ name] — same detection catches both syntaxes

// Escaping check (D-02): flat string scan, not a regex-of-a-regex.
if strings.Contains(rawPatternString, `\`) {
    // reject — D-02's normalized form never needs a backslash
}

// Named-group check (D-08): one API call catches both Go and JS named-group
// syntax without distinguishing which was written.
for _, name := range compiled.SubexpNames() {
    if name != "" {
        // reject — named groups banned by D-08
    }
}
```

### Pattern 3: Satisfiability check (D-03/D-11) — resolve against the `from:` file at HEAD
**What:** A pattern that compiles and is escape-clean can still be a no-op if it can never match its
`from:` file's current content (#479's second finding: a symbol pinned via `pattern:` that was
routed through a different function by a later refactor).
**When to use:** For the recurring guard (active milestone only, D-04) and the one-time v0.13.x
sweep (D-11/D-12) — same underlying check, different file sets.
**Example:**
```go
// Source: mirrors verify.cjs's own matching logic (verify.cjs:1098-1119) so
// the Go-side check and the tool's actual JS-side behavior test the same
// question, just in the RE2 engine instead of JS's backtracking engine —
// valid because D-08 already restricts patterns to the RE2 ∩ JS common
// subset, so a match/no-match verdict means the same thing in both engines.
compiled := regexp.MustCompile(pattern) // already passed D-08's gate
fromContent, err := os.ReadFile(fromPath)
if err != nil || !compiled.Match(fromContent) {
    // check `to:` file too before declaring unsatisfiable — verify.cjs
    // falls back to the target file (verify.cjs:1106-1110)
}
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Detecting JS-only regex syntax (lookaround, backreferences) | A custom regex-of-regexes or hand-written character scanner for `(?=`, `(?!`, `(?<=`, `\1`-style backreferences | `regexp.Compile`'s own error, checked for these classes | RE2 already refuses to compile these constructs with a stable, greppable error string — verified this session; hand-rolling a detector duplicates logic the stdlib gives for free and risks missing a JS regex feature the stdlib author already enumerated |
| Detecting named capture groups | Parsing the pattern string for `(?P<` / `(?<` substrings | `regexp.Regexp.SubexpNames()` after compile | One API call handles both Go's and JS's named-group syntax uniformly (empirically confirmed both compile under this repo's Go 1.26 toolchain) — a string-substring approach would need to separately special-case both syntaxes and could false-positive inside a character class like `[(?<]` |
| Waiting for a service container to be ready in CI | A hand-rolled `sleep`/retry loop before the `go test` step | GHA `services:`'s built-in `options: --health-cmd` polling | The runner already blocks job steps until the declared health check passes; a manual sleep either races (too short) or wastes CI minutes (too long, or fixed regardless of actual startup time) |
| Capturing why a container died mid-test | Custom Go code polling container status via the Docker SDK from inside the test binary | A `docker inspect`/`docker logs`/`dmesg` CI post-step (`if: failure()`) | D-19's own reasoning: when the container dies, the Go process that would poll it is often unreachable too — the CI runner itself, which is still alive, is the only reliable place to gather this evidence |

**Key insight:** Every piece of "custom tooling" this phase might be tempted to hand-roll (regex
dialect detection, container health polling, death diagnostics) already has a stdlib or
platform-native answer that was empirically or documentationally confirmed this session. The
temptation to write bespoke detection logic for the RE2/JS gap in particular should be resisted —
`regexp.Compile` + `SubexpNames()` is the complete, minimal surface needed.

## Common Pitfalls

### Pitfall 1: Assuming RE2 compile failure implies the escaping defect
**What goes wrong:** A plan writer (or a future contributor to the guard) assumes "if
`regexp.Compile` succeeds, the pattern is fine" — but `\\.` compiles successfully under RE2 with the
exact wrong semantics the guard exists to catch.
**Why it happens:** It's intuitive to conflate "compiles" with "correct" for a regex-validation
tool, and most OTHER JS-only-syntax defects (lookaround, backreferences) DO fail to compile under
RE2 — creating a false pattern of "compile failure = defect" that breaks for this specific shape.
**How to avoid:** The escaping check (D-02/D-04) must be an explicit string-level check (`\` present
anywhere in the raw pattern string), run unconditionally, never gated on compile success/failure.
**Warning signs:** A guard implementation that only checks `err := regexp.Compile(pattern); err !=
nil` and treats `err == nil` as "pattern is fine" — this is Pitfall 1 exactly, and it is the same
failure mode #479 diagnoses in `verify.cjs` itself, just relocated to the guard's own Go code.

### Pitfall 2: Confusing "no `ListCollections`-then-delete-all" with "no name collision risk"
**What goes wrong:** Assuming a shared Qdrant instance is automatically safe because tests only ever
target named collections (verified this session: zero `ListCollections` calls anywhere in
`internal/` — `[VERIFIED: repo-wide grep, no matches]`) — but that only protects against
*accidental drop-everything* teardown bugs, not against two packages writing to the SAME collection
name simultaneously and corrupting each other's fixtures mid-run.
**Why it happens:** "Safe from accidental deletion" and "safe from concurrent writes to the same
name" are different properties; D-16's prefix requirement addresses the second, and it is easy to
read the `ListCollections` absence as covering both.
**How to avoid:** Confirm — not assume — that every hardcoded collection-name string in
`internal/store/store_test.go` and `internal/server/tools_test.go` is disjoint after prefixing.
This session's full inventory (below) is exhaustive for these two files; re-run the same grep after
editing to confirm no new collision was introduced.
**Warning signs:** A CI run where two packages' tests both create a collection with the SAME final
name (e.g., both prefix to `mem_eval_test` because the prefix was applied inconsistently) — this
would silently reintroduce cross-package interference under the shared instance, the opposite of
what D-16 is meant to prevent.

### Pitfall 3: `internal/e2e` and `internal/retrievaleval` do NOT need D-16 code changes
**What goes wrong:** Applying the same "add a package prefix constant" remediation to all four
packages named in D-17, when only two of them (`internal/store`, `internal/server`) actually have
the collision.
**Why it happens:** D-16's prose names `internal/store`/`internal/server`'s hardcoded strings as the
concrete example, and D-17 separately says "all four move onto the shared instance" — it's easy to
conflate "moves onto shared instance" (all four, true) with "needs a new prefix constant" (only two,
per this session's verification).
**How to avoid:** `internal/e2e/harness_test.go:257` already generates
`"e2e_" + strconv.FormatInt(int64(port), 10)` — unique per test-server instance, already collision
safe. `internal/retrievaleval/retrieval_eval_test.go:299` already generates
`"retrievaleval_" + uuid.NewString()` — also already unique. Both packages need ONLY the CI-side
`ENGRAM_QDRANT_TEST_ADDR` wiring (which they already read); zero Go-side collection-name edits.
**Warning signs:** A plan task that touches `internal/e2e` or `internal/retrievaleval` Go source for
collection naming — that is unscoped work relative to what this session verified is actually
colliding.

### Pitfall 4: GHA service containers don't wait for readiness without an explicit health check
**What goes wrong:** Declaring a `services: qdrant: image: ...` block with no `options:
--health-cmd` and assuming the runner waits for Qdrant to actually be accepting connections before
starting the `test` step — it does not; GHA only gates on the Docker image's OWN `HEALTHCHECK`
instruction if the image ships one, or an explicit `--health-cmd` in `options:`.
**Why it happens:** The GHA docs' basic examples (redis, postgres) tend to show canonical images
that DO ship a `HEALTHCHECK`; Qdrant's own image is not confirmed to ship one in this session's
research, so omitting `--health-cmd` risks a race where `go test` starts dialing before Qdrant's
gRPC/REST listeners are up — reintroducing exactly the "connection refused" cascade this phase
exists to eliminate, just from a different cause (early dial instead of concurrent OOM).
**How to avoid:** Set `options: --health-cmd "curl -f http://localhost:6333/readyz" --health-interval 5s --health-timeout 3s --health-retries 10`
in the `services:` block. This repo's own Helm chart already uses `/readyz` on the REST port for
Qdrant's Kubernetes readiness probe — `[VERIFIED: charts/engram/templates/qdrant.yaml:67-68]`
(`readinessProbe: httpGet: { path: /readyz, port: rest }`), giving in-repo precedent for which
endpoint to poll.
**Warning signs:** CI failures specifically at test START (immediate connection-refused on the
first Qdrant-touching test) rather than mid-run, which points at a startup race rather than the
resource-pressure death this phase is otherwise fixing.

## Code Examples

### Full pattern-validation function shape (composes Patterns 1–2 above)
```go
// Source: composed from this session's empirical verification; mirrors
// verify.cjs's own try/catch-around-new RegExp structure (verify.cjs:1098-1119)
// so a Go-side PASS is real evidence about the JS-side tool's behavior.
package keylinks

import (
	"fmt"
	"regexp"
	"strings"
)

type patternIssue struct {
	Shape string // "escaping" | "unsupported-syntax" | "named-group" | "compile-error"
	Fix   string // corrected character-class form, when mechanically derivable
}

func validatePattern(raw string) (*regexp.Regexp, *patternIssue) {
	if strings.Contains(raw, `\`) {
		return nil, &patternIssue{Shape: "escaping", Fix: suggestCharClassForm(raw)}
	}
	re, err := regexp.Compile(raw)
	if err != nil {
		// Catches lookaround/backreferences for free (Pattern 1).
		return nil, &patternIssue{Shape: "compile-error", Fix: fmt.Sprintf("invalid: %v", err)}
	}
	for _, name := range re.SubexpNames() {
		if name != "" {
			return nil, &patternIssue{Shape: "named-group"}
		}
	}
	return re, nil
}
```

### GitHub Actions `services:` block for shared CI Qdrant
```yaml
# Source: mechanics confirmed via docs.github.com "About service containers";
# port numbers confirmed via qdrant.tech/documentation/quickstart; health
# endpoint confirmed via this repo's own charts/engram/templates/qdrant.yaml:67-68.
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      qdrant:
        image: qdrant/qdrant:v1.18.2 # same tag as store_test.go:33's qdrantImageTag
        ports:
          - 6334:6334 # gRPC — what the Go client dials (qdrant.Config{Port: ...})
          - 6333:6333 # REST — needed only for the health-cmd probe below
        options: >-
          --health-cmd "curl -f http://localhost:6333/readyz"
          --health-interval 5s
          --health-timeout 3s
          --health-retries 10
    env:
      ENGRAM_QDRANT_TEST_ADDR: localhost:6334
      ENGRAM_REQUIRE_QDRANT: "1"
    steps:
      # ...existing checkout/setup-go/go test steps, unchanged...
      - name: dump container diagnostics on failure
        if: failure()
        run: |
          docker ps -a --filter "label=com.github.actions.local-checkout" || true
          docker logs "$(docker ps -aq --filter ancestor=qdrant/qdrant:v1.18.2)" 2>&1 | tail -200 || true
          dmesg 2>/dev/null | grep -i -E 'oom|killed' || echo "no OOM evidence in dmesg"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Four independent `qdrant/qdrant:v1.18.2` testcontainers, one per package, booted concurrently by `go test ./...` | One shared `services:` Qdrant container for the whole CI `test` job | This phase | Removes 2-vCPU runner resource pressure at its source (D-15); per-package testcontainer boot path stays as local-dev fallback (D-18), unaffected |
| Key-link `pattern:` fields hand-written with `\\`-escaped forms (`\\.`, `\\(`) | Escape-free character-class forms (`[.]`, `[(]`, `[)]`) — already the convention v0.13.x Phase 3 adopted in `ca8d337c` | v0.13.x Phase 3 (partial); this phase (repo-wide, D-09) | Patterns become actually matchable by both the RE2-based guard and the JS-based `verify.cjs` consumer, closing #479 |

**Deprecated/outdated:**
- `\\`-escaped `pattern:` fields — banned outright by D-02's flat no-backslash rule; not merely
  discouraged.

## Runtime State Inventory

Not applicable — this phase touches no databases, external service configuration, OS-registered
state, secrets, or build artifacts referencing a renamed string. It edits `.planning/**` markdown
(data, not a live system), adds one new Go package, edits four Go test files, and edits one CI
workflow file. No rename/rebrand/migration trigger applies (see execution flow Step 2.5's trigger
condition — this phase is neither a rename nor a migration).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Qdrant's Docker image does not ship its own `HEALTHCHECK` instruction (motivating the explicit `--health-cmd` in the Code Examples section) | Common Pitfalls #4, Code Examples | Low — if the image DOES ship a health check, the explicit `--health-cmd` is redundant but harmless; the plan should not assume this is definitely absent without a quick `docker inspect qdrant/qdrant:v1.18.2 --format '{{.Config.Healthcheck}}'` check during execution |
| A2 | GHA's default `--health-retries`/`--health-interval` defaults (30s/30s per general GHA docs) would be adequate without the tighter values shown in Code Examples | Code Examples | Low — using the tighter values shown (5s interval, 10 retries = up to 50s wait) is strictly safer than defaults for a container that needs to be ready before the FIRST test dials; no functional risk either way, only CI wall-clock time |

**If this table is empty:** N/A — two low-risk items above.

## Open Questions

1. **Which specific 38 files/line-ranges carry the offending `pattern:` fields?**
   - What we know: repo-wide grep this session found 52 lines matching `pattern:.*\\` across
     `.planning/**/*.md` (a superset — some are likely single-backslash valid-escape forms like
     `\.` rather than the double-backslash `\\.` defect shape; CONTEXT.md's count of exactly 38 is
     more precise than this session's coarse grep).
   - What's unclear: the exact enumerated list of 38 offenders and which 25 fall inside v0.13.x
     Phases 1–2 specifically — CONTEXT.md asserts these counts but this session did not re-derive
     them from a matcher equivalent to the guard being built (doing so requires the guard's own
     validated-pattern logic, which is Wave 0 work, not research).
   - Recommendation: Wave 0 of the plan should run the guard's escaping check in report-only mode
     (or a throwaway script using the same `validatePattern` logic from Code Examples) against
     `.planning/**` FIRST, before any rewrite work, to get the authoritative offender list and
     confirm/refute the 38/25 counts before D-09/D-10's rewrite work begins.

2. **Exact final prefix strings for D-16.**
   - What we know: CONTEXT.md gives `store_mem_eval_test` / `server_mem_eval_test` as illustrative
     examples ("e.g.") and explicitly leaves the precise strings to planning discretion.
   - What's unclear: whether `reindex_test.go`'s existing `reindex_`-prefixed names (`reindex_src`,
     `reindex_srcov_real`, etc. — full inventory below) should be double-prefixed
     (`store_reindex_src`) or left as-is if their `reindex_` prefix is already sufficiently unique
     against `internal/server`'s collection space.
   - Recommendation: Use `store_` / `server_` as the package-level prefix uniformly, prepended to
     EVERY hardcoded name in that package (including the already-somewhat-prefixed
     `reindex_*` names) — consistency here is cheap and removes any need to reason about whether
     `reindex_src` could theoretically collide with a hypothetical future `internal/server` name.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Go toolchain | `internal/keylinks` guard, all Go test changes | Yes | go1.26.5 darwin/arm64 (local); `go.mod` pins `go 1.26.3` | — |
| Docker (local dev) | Per-package testcontainer fallback (D-18, unchanged) | Not verified this session (no `docker info` probe was warranted — CI, not local dev, is this phase's target) | — | `ENGRAM_QDRANT_TEST_ADDR` unset + no Docker → tests skip (existing, unchanged behavior) |
| GitHub Actions `ubuntu-latest` runner | Shared Qdrant `services:` container | Yes (existing CI target, per `ci.yaml:26`) | Docker pre-installed on `ubuntu-latest`, confirmed by the existing per-package testcontainer path already working today | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — Docker-absent local dev already degrades gracefully
via the existing skip path, unchanged by this phase.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no third-party framework anywhere in `internal/`, confirmed: zero `stretchr/testify` imports repo-wide `[VERIFIED: repo-wide grep, 0 matches]`) |
| Config file | none — `go test` needs no config file |
| Quick run command | `go test ./internal/keylinks/... -v` (new package, once created) |
| Full suite command | `go test ./...` (already the CI gate, `ci.yaml:40`); `task test:strict` for the fail-closed local variant (`Taskfile.yaml:49-54`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|--------------------|--------------|
| REQ-keylink-pattern-matchable | Guard fails on a reintroduced `\\`-escaped or unsatisfiable pattern; passes on the corrected character-class form | unit (fixture pair, D-06) | `go test ./internal/keylinks/... -run TestKeyLinkFixtures -v` | ❌ Wave 0 (new package) |
| REQ-keylink-pattern-matchable | Zero offending patterns remain in `.planning/**` after D-09's rewrite | unit (repo-wide scan) | `go test ./internal/keylinks/... -run TestNoOffendingPatterns -v` | ❌ Wave 0 |
| REQ-keylink-past-gates-reassessed | Every v0.13.x Phase 1–2 key-link resolved to pinned/unpinned with reason | manual-only (produces `VERIFICATION.md` table, D-13) — the sweep itself can reuse `internal/keylinks`'s matcher as a throwaway `go run` invocation, but the DELIVERABLE is a document, not a passing test | n/a — verified by reading the resulting table, not by an automated assertion | n/a |
| REQ-ci-qdrant-container-stability | Exactly one Qdrant container in the CI run; all four packages resolve to the same address; collection prefixes provably disjoint (D-20) | integration/smoke (CI-only — needs the live GHA `services:` container, cannot run identically locally without a manual `ENGRAM_QDRANT_TEST_ADDR` override) | Locally: `ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1` against a manually-started single Qdrant, to rehearse the CI topology before pushing | ✅ existing `TestMain` chains already support this — no new test file needed, only the collection-name constant edits |
| REQ-ci-qdrant-container-stability | Container exit reason captured in failure output when the container dies | n/a — this is CI workflow behavior (D-19's `if: failure()` step), not something `go test` itself asserts | n/a — verified by forcing a failure (e.g., temporarily set an absurdly low `--memory` limit on the `services:` container) and reading the post-step's output in a scratch CI run, per the verification-before-completion discipline | n/a |

### Sampling Rate
- **Per task commit:** `go test ./internal/keylinks/...` (guard's own package — fast, no Qdrant
  needed since D-04's escaping check is pure string/regex logic and D-04's satisfiability check
  only reads `.planning/**` + source files, no live services)
- **Per wave merge:** `go test ./...` (full suite — exercises the guard as part of the whole build,
  and for Wave(s) touching CI, requires an actual push to a PR branch since `services:` containers
  only exist in the real GHA environment, not `act` or local `go test`)
- **Phase gate:** Full suite green in a REAL GitHub Actions run (not just local `go test ./...`)
  before `/gsd-verify-work` — D-20 explicitly rejects "green streak" as sufficient evidence, so the
  phase gate must include reading the CI run's own diagnostics for the mechanism assertions (one
  container, shared address, disjoint prefixes), not just a pass/fail signal.

### Wave 0 Gaps
- [ ] `internal/keylinks/keylinks.go` + `keylinks_test.go` — the guard itself; no existing file to
  extend, this is greenfield within the phase.
- [ ] `internal/keylinks/testdata/good_key_links.md` + `bad_key_links.md` — the D-06 fixture pair.
- [ ] A throwaway or committed sweep script/test for the v0.13.x Phase 1–2 reassessment (D-11/D-12)
  — CONTEXT.md does not specify whether this is a permanent test file or a one-off `go run`
  invocation whose OUTPUT (the `VERIFICATION.md` table) is the only persisted artifact; planning
  discretion per the Claude's Discretion note.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|-------------------|
| V2 Authentication | No | This phase touches no auth surface — CI tooling and a repo-internal Go package |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A |
| V5 Input Validation | Partial — `pattern:` strings are project-authored (trusted `.planning/**` content, not attacker input), but the guard's own regex-compilation logic is effectively an input-validation gate | D-08's restriction to the RE2 ∩ JS common subset, enforced via `regexp.Compile` + `SubexpNames()` (see Code Examples) |
| V6 Cryptography | No | N/A |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| ReDoS (catastrophic regex backtracking) if `pattern:` content were ever attacker-influenced | Denial of Service | Not a live risk today — `pattern:` values are authored by repo maintainers in `.planning/**`, not user input — but RE2's design (used by both the guard AND, transitively, by restricting to the RE2 ∩ JS subset) is inherently immune: RE2 guarantees linear-time matching with no catastrophic-backtracking pathological cases, by construction `[CITED: RE2 project docs — "a regular expression engine that runs in time linear in the size of the input"]`. Worth noting as a side benefit of D-08's restriction, not a new mitigation this phase must add. |
| CI post-step (D-19) shell script processing container names/logs | Tampering (script injection via crafted log content) | Container name/logs are read via `docker logs`/`dmesg`, not interpolated into a shell command from untrusted PR content — mirror this repo's own existing pattern in `ci.yaml`'s `ui-drift` job, which explicitly passes `${{ }}` expression values through `env:` rather than inline shell interpolation specifically to avoid this class of injection `[VERIFIED: .github/workflows/ci.yaml:252-260, comment explicitly names the hazard]` |

## Sources

### Primary (HIGH confidence)
- `internal/store/store_test.go` (this repo) — `TestMain` precedence chain, `requireQdrant`,
  `terminateQdrant`, `qdrantImageTag`, `testStore`'s `mem_eval_test` collection name — read in full
  this session, lines 1–260.
- `internal/server/tools_test.go` (this repo) — collection-name collision confirmed at line 321
  (`mem_eval_test`) and line 5286 (`mem_load_once_test`) via targeted grep + line read.
- `internal/e2e/harness_test.go:245-264` (this repo) — `startServer`'s already-unique
  `"e2e_" + strconv.FormatInt(int64(port), 10)` collection naming — read in full.
- `internal/retrievaleval/retrieval_eval_test.go:299` (this repo) — already-unique
  `"retrievaleval_"+uuid.NewString()` naming — confirmed via targeted grep.
- `internal/store/reindex_test.go` (this repo) — full inventory of `src`/`tgt`-family collection
  names (`reindex_src`, `reindex_tgt`, `reindex_srcov_real`, `reindex_srcov_env`,
  `reindex_srcov_tgt`, `reindex_prog_src`, `reindex_prog_tgt`) — confirmed via targeted grep.
- `internal/config/registry.go:26-29` (this repo) — default `qdrant.addr` (`localhost:6334`) and
  `qdrant.collection` (`mem_eval`) — read directly.
- `charts/engram/templates/qdrant.yaml:63-68` (this repo) — existing `/livez`/`/readyz` Kubernetes
  probe precedent for the CI health-check endpoint choice — confirmed via grep + context.
- `$HOME/.claude/gsd-core/bin/lib/verify.cjs:1039-1129` — `cmdVerifyKeyLinks`'s `new
  RegExp(link['pattern'])` call and its try/catch structure — read in full this session.
- `$HOME/.claude/gsd-core/bin/lib/frontmatter.cjs:500-590` — `parseMustHavesBlock`'s YAML-ish
  line-based parser, confirmed it only strips leading/trailing quote characters
  (`.replace(/^["']|["']$/g, '')`) and never processes backslash escape sequences — read in full
  this session, confirming the defect mechanism exactly.
- Empirical Go probe (this session, `go run` against go1.26.5 darwin/arm64) — confirmed which
  JS-only regex constructs `regexp.Compile` rejects for free (lookahead, negative lookahead,
  lookbehind, backreferences) versus accepts silently (the `\\.` escaping-defect shape; both
  `(?P<name>...)` and `(?<name>...)` named-group syntaxes) — this is the single most
  plan-load-bearing finding in this research and could not have been sourced from training data
  alone with confidence, since Go's acceptance of the JS `(?<name>...)` syntax specifically is a
  relatively recent stdlib addition.
- `internal/surfaces/conformance_test.go`, `internal/openaiurl/openaiurl_test.go` (this repo) —
  confirmed stdlib-only `testing` idiom (no `testify`), table-driven `t.Run` subtests — read
  directly this session.

### Secondary (MEDIUM confidence)
- docs.github.com "About service containers" / "Creating Redis service containers" — GHA
  `services:` port-mapping and `--health-cmd`/`--health-interval`/`--health-timeout`/
  `--health-retries` syntax, via WebSearch this session.
- qdrant.tech/documentation/quickstart, api.qdrant.tech "Kubernetes readiness probe" — Qdrant's
  default ports (6333 REST, 6334 gRPC) and `/healthz`/`/livez`/`/readyz` endpoint semantics, via
  WebSearch this session — independently corroborated by this repo's own Helm chart choice of
  `/readyz` for readiness, raising this to effectively HIGH confidence.

### Tertiary (LOW confidence)
- RE2 project's "linear time" design claim — long-standing, well-known property of the RE2 engine
  (used by Go's `regexp` package), cited from general knowledge of the project's stated design goal
  rather than re-fetched from RE2's own docs this session; low practical risk since it is not
  load-bearing for any plan task (D-08's restriction to the RE2 ∩ JS subset stands on its own
  functional-correctness merits regardless of the ReDoS side-benefit).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all behavior claims for existing stdlib/tooling
  either read from source this session or empirically probed.
- Architecture: HIGH — CONTEXT.md's 20 locked decisions already fully specify the architecture;
  this research verified the concrete file-level facts (exact collection names, exact line numbers,
  exact regex-compile behavior) needed to execute against those decisions precisely.
- Pitfalls: HIGH for Pitfalls 1–3 (all empirically or textually verified this session); MEDIUM for
  Pitfall 4 (Qdrant's own `HEALTHCHECK` presence was not directly probed — see Assumption A1).

**Research date:** 2026-08-13
**Valid until:** Effectively indefinite for the Go-stdlib-behavior claims (RE2 semantics are
extremely stable across Go versions); ~90 days for the GHA `services:`/health-check syntax claims
(GitHub Actions syntax is stable but not immutable).
