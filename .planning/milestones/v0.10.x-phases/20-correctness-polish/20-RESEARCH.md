# Phase 20: Correctness & Polish - Research

**Researched:** 2026-07-15
**Domain:** Internal Go/proto/Helm hardening — no new capabilities, six independent bugfix-shaped changes
**Confidence:** HIGH

## Summary

This phase closes six independent correctness gaps found during v0.9.x code review. Every gap
was verified directly against the live codebase (not inferred from CONTEXT.md's file:line
hints) — all anchors CONTEXT.md cites are accurate as of HEAD (`24bb8461`), with one important
scope correction: **D-03's open question resolves to "already fixed"** — `summary` is not
dropped by `SearchDiscoveries`; only `kind`/`citations` are genuinely missing. **The #303
open question also resolves to "already fixed"** — `storeDiscoveryArgs.ID`'s jsonschema tag has
carried `"...or short_id..."` wording since the original short_id PR (#288, commit `92a6f610`,
2026-07-06), predating this phase. Both findings **narrow scope**, not expand it.

The other four gaps (#308 MintShortID cap, #304 embed reserved-key sharing, #302 embed
body-build collapse, #269 summarize CronJob) are exactly as CONTEXT.md describes: real,
unaddressed, and mechanically bounded. No new Go dependencies, no new Helm subcharts, no new
proto messages — this is entirely internal refactor + one additive proto extension + one new
Helm template. Zero external packages are introduced; the Package Legitimacy Gate is N/A this
phase (see that section below).

**Primary recommendation:** Follow D-10's 4-plan split, but rescope Plan A and Plan C precisely
per the findings in this document. Plan A becomes "proto Memory.kind/citations extension
(#307) + verify-and-close #303 (no code change expected)." All four plans are file-disjoint
enough to execute in parallel (one shared touch point: `internal/embed/embed.go` is touched by
both #304 and #302, hence CONTEXT.md's Plan B grouping — keep that).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Discovery wire fidelity (#307) | API / Backend (proto + Connect handler) | — | `Memory` proto message + `memoryToProto` conversion; console rendering explicitly deferred (D-02) |
| Short-id collision-retry cap (#308) | Database / Storage (store layer) | API / Backend (error bubbles to tool handlers) | `MintShortID` owns the Qdrant-checked retry loop; callers just propagate the error |
| Embed param-key sharing (#304) | API / Backend (`internal/embed`, `internal/config`) | — | Both are backend config/wire-contract packages; no browser/CDN involvement |
| Embed body-build collapse (#302) | API / Backend (`internal/embed`) | — | Pure internal refactor of the outbound embeddings HTTP request builder |
| Discovery short_id schema (#303) | API / Backend (MCP tool jsonschema) | — | Already resolved; verification-only, no tier change |
| Summarize CronJob (#269) | CDN / Static... N/A — **Infra/Ops tier** (Helm chart) | — | Not a request-serving tier; it's a scheduled batch job definition reusing the Deployment's env/image plumbing |

No capability in this phase touches the Browser/Client tier — confirms D-02's explicit scoping
(console rendering deferred).

## Package Legitimacy Audit

**Not applicable this phase.** No new Go modules, npm packages, or Helm subcharts are
introduced. All six fixes use only:
- Go stdlib (`errors`, `fmt`, `encoding/json`)
- Already-vendored deps already imported by the touched packages (`connectrpc.com/connect`,
  `google.golang.org/protobuf`, existing `internal/telemetry`, `internal/shortid`)
- Kubernetes `batch/v1` CronJob — a core Kubernetes API kind, not a package/dependency.

No `go.mod`/`package.json`/`Chart.yaml` `dependencies:` changes are expected. If a plan turns
out to need a new import, re-run the Package Legitimacy Gate for that import before adding it.

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────────────┐
                         │   proto/engram/v1/engram.proto       │
                         │   Memory { ...; +kind=21; +citations=22 }│
                         └───────────────┬───────────────────────┘
                                         │ task proto:gen (buf generate)
                                         ▼
              ┌──────────────────────────────────────────────────┐
              │ gen/go (Go structs)         gen/ts (TS types)     │
              └───────────┬──────────────────────────┬───────────┘
                          │                          │ cp -R (task proto:gen, already automated)
                          ▼                          ▼
     internal/server/connectapi.go          ui/src/lib/gen/ (vendored copy)
     memoryToProto(m store.Memory)                   │
       + m.Kind, citationsToProto(m.Citations)        │ task ui:build (SPA rebuild,
                          │                            │  CI ui-drift job re-checks)
                          ▼                            ▼
     SearchDiscoveries RPC response ──────► Connect wire ──────► console gen client
     (kind/citations now populated;                              (types exist; NOT rendered
      summary already worked pre-phase)                           this phase — D-02)

     ────────────────────────────────────────────────────────────

     store_memory / schedule_memory / store_discovery / store_rule
                          │
                          ▼
          internal/store/store.go: MintShortID(ctx, seen)
                for attempts < 16 {           <- NEW bounded loop (was `for {}`)
                    cand := gen()
                    if seen[cand] { continue }  <- does NOT consume attempt budget (D-05)
                    n := client.Count(...)      <- ONE real Qdrant check = 1 attempt
                    if n == 0 { return cand }
                }
                return "", ErrShortIDExhausted  <- NEW sentinel, errors.Is-checkable
                          │
                          ▼
          bubbles as normal write failure through store_memory/store_discovery/store_rule
          (telemetry.RecordStoreOp + span.RecordError already wrap this path — D-06, free)

     ────────────────────────────────────────────────────────────

     internal/config/embedparams.go: ParseEmbedParams
          reserved keys ["model","input"] ──┐
                                             │ share via embed.ReservedParamKeys (NEW)
     internal/embed/embed.go: embed()  ◄─────┘
          BEFORE: struct-marshal (empty params) OR map-merge (non-empty params) — 2 paths
          AFTER:  always map-merge; reserved keys read from the shared list — 1 path

     ────────────────────────────────────────────────────────────

     charts/engram/templates/_helpers.tpl (NEW)
          {{ define "engram.containerEnv" }} ... shared env block (verbatim from
          memory-mcp.yaml's current inline env list) ... {{ end }}
                │                              │
                ▼                              ▼
     memory-mcp.yaml Deployment      NEW summarize-cronjob.yaml (batch/v1 CronJob)
     (env: {{ include "engram.containerEnv" . }})   (disabled by default; D-07/D-08 knobs;
                                                      runs `engram summarize-missing --all-scopes`)
```

### Recommended Project Structure (files touched, no new dirs)
```
proto/engram/v1/engram.proto          # +2 additive fields on Memory
internal/server/connectapi.go         # memoryToProto extended + new citationsToProto helper
internal/store/store.go               # MintShortID bounded loop + ErrShortIDExhausted
internal/store/store_test.go          # new exhaustion test (mirrors existing collision test)
internal/embed/embed.go               # ReservedParamKeys export + single-path embed()
internal/embed/embed_test.go          # existing tests should still pass unmodified (decode-based)
internal/config/embedparams.go        # ParseEmbedParams reads embed.ReservedParamKeys
internal/server/tools.go              # storeDiscoveryArgs.ID — verify only, likely NO diff
charts/engram/templates/_helpers.tpl  # NEW — shared containerEnv named template
charts/engram/templates/memory-mcp.yaml   # env: block replaced with {{ include }}
charts/engram/templates/summarize-cronjob.yaml  # NEW batch/v1 CronJob
charts/engram/values.yaml             # new memory.summarize.cronjob block
gen/go/, gen/ts/, ui/src/lib/gen/     # regenerated by `task proto:gen` (already re-vendors TS)
internal/webauth/static/              # rebuilt by `task ui:build` if SPA output changes at all
```

### Pattern 1: Additive-only proto extension (Phase-15 discipline, reused verbatim)
**What:** New fields always get the next unused field number; never renumber/retype existing
fields. `Memory`'s highest current field is `last_accessed_at = 20`; the next free numbers are
**21 and 22** (verified — no other in-flight phase claims them as of HEAD).
**When to use:** Any wire-visible Memory extension.
**Example:**
```protobuf
// Source: proto/engram/v1/engram.proto (Memory message, live at HEAD)
message Memory {
  // ... fields 1-20 unchanged ...
  google.protobuf.Timestamp last_accessed_at = 20;
  // NEW — discovery-only fields; empty on plain memories (never set by
  // store_memory/schedule_memory, only by store_discovery). Reuses the
  // existing Citation message (engram.proto:122), no new message type.
  string kind = 21;
  repeated Citation citations = 22;
}
```
CI enforces this two ways: `go tool buf breaking --against main` (rejects renumber/retype) and
the `buf` job's `git diff --exit-code -- gen/` drift check (rejects hand-edited/stale gen/).

### Pattern 2: store→proto conversion lives beside its sibling, not in protoconv.go
**What:** `internal/server/connectapi.go`'s `memoryToProto`/`memoriesToProto` (read-path,
store→proto) is a **different file** from `internal/server/protoconv.go` (write-path,
proto-request→args and write-result→proto — see its own header comment: "every write RPC proto
request -> internal *Args mapping and every write-result -> proto response mapping lives here").
`protoconv.go` already has the *inverse* direction for citations —
`citationToArg`/`citationsToArgs` (engramv1.Citation → citationArg, used by StoreDiscovery
write RPC). #307 needs the missing *forward* direction (store.Citation → engramv1.Citation) for
the read path — this belongs in `connectapi.go` beside `memoryToProto`, named
`citationsToProto` for naming symmetry with the existing `memoriesToProto`.
**Example:**
```go
// Source: internal/server/connectapi.go (extend the existing memoryToProto, HEAD lines 32-52)
func citationsToProto(cs []store.Citation) []*engramv1.Citation {
	if len(cs) == 0 {
		return nil
	}
	out := make([]*engramv1.Citation, len(cs))
	for i, c := range cs {
		out[i] = &engramv1.Citation{Kind: c.Kind, Ref: c.Ref, Locator: c.Locator, Pin: c.Pin, Excerpt: c.Excerpt}
	}
	return out
}

func memoryToProto(m store.Memory) *engramv1.Memory {
	// ... existing fields unchanged ...
	return &engramv1.Memory{
		// ... existing field assignments ...
		Kind:      m.Kind,
		Citations: citationsToProto(m.Citations),
	}
}
```
`store.Citation`'s fields (`Kind`, `Ref`, `Locator`, `Pin`, `Excerpt` —
`internal/store/store.go:184-190`) map 1:1 to the `Citation` proto message
(`engram.proto:122-128`) — no field-name or type reconciliation needed.

### Pattern 3: sentinel error + errors.Is (repo-idiomatic, reused verbatim)
**What:** Package-level `var Err... = errors.New("...")`, wrapped with `fmt.Errorf("%w: ...", Err..., detail)` at the return site, compared via `errors.Is` by callers. `errorlint` enforces `%w` (not `%v`/string concat) in CI lint.
**When to use:** `MintShortID`'s exhaustion path (#308).
**Example:**
```go
// Source: internal/store/store.go (existing sibling sentinel, line 56 + its one call site line 1226)
var ErrAmbiguousShortID = errors.New("ambiguous short id")
// ...
return "", fmt.Errorf("%w: %s", ErrAmbiguousShortID, canonical)

// NEW sentinel for #308, same idiom, placed beside the other short-id sentinels:
var ErrShortIDExhausted = errors.New("short id mint exhausted")
// ...
return "", fmt.Errorf("%w: after %d attempts", ErrShortIDExhausted, maxMintAttempts)
```

### Pattern 4: Helm named-template factoring for shared container env
**What:** `_helpers.tpl` does not exist yet in `charts/engram/templates/` (only `expose.yaml`,
`memory-mcp.yaml`, `qdrant.yaml` exist). Create it with one `{{ define "engram.containerEnv" }}`
block containing the **exact** current inline `env:` list from `memory-mcp.yaml` (lines 30-163
at HEAD — every var from `ENGRAM_LISTEN_ADDR` through the conditional `SSL_CERT_FILE` block,
including the Downward-API `POD_*`/`OTEL_RESOURCE_ATTRIBUTES` vars). The Deployment then becomes
`env: {{- include "engram.containerEnv" . | nindent 12 }}` — **`helm template` output for the
Deployment must be byte-identical before/after** (verify with a diff, not just visual review).
**When to use:** #269's CronJob needs the identical env (image, Qdrant, OpenAI/embed, summarize
config, CA bundle) without duplicating 130+ lines of template.
**Example (CronJob skeleton — new file):**
```yaml
# Source: pattern derived from charts/engram/templates/memory-mcp.yaml (HEAD) + kubernetes.io CronJob v1 spec
{{- if .Values.memory.summarize.cronjob.enabled }}
apiVersion: batch/v1
kind: CronJob
metadata:
  name: memory-mcp-summarize
spec:
  schedule: {{ .Values.memory.summarize.cronjob.schedule | quote }}
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: {{ .Values.memory.summarize.cronjob.successfulJobsHistoryLimit }}
  failedJobsHistoryLimit: {{ .Values.memory.summarize.cronjob.failedJobsHistoryLimit }}
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          {{- with .Values.imagePullSecrets }}
          imagePullSecrets: {{ toYaml . | nindent 12 }}
          {{- end }}
          containers:
            - name: summarize-missing
              image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
              imagePullPolicy: {{ .Values.image.pullPolicy }}
              args: ["summarize-missing", "--all-scopes", "--timeout={{ .Values.memory.summarize.cronjob.timeout | default "25m" }}"]
              env: {{- include "engram.containerEnv" . | nindent 16 }}
              {{- if .Values.memory.openai.caBundle.configMapName }}
              volumeMounts:
                - { name: ca-bundle, mountPath: /etc/ssl/ca-bundle, readOnly: true }
              {{- end }}
          {{- if .Values.memory.openai.caBundle.configMapName }}
          volumes:
            - name: ca-bundle
              configMap: { name: "{{ .Values.memory.openai.caBundle.configMapName }}" }
          {{- end }}
{{- end }}
```
Note: `summarize-missing` requires `--scope <s>` OR `--all-scopes` (cobra `RunE` returns an
error otherwise — `cmd/engram/summarize.go:37-39`) — the CronJob must pass `--all-scopes` (a
sweep-everything default is the only sane unattended-cron behavior).

### Anti-Patterns to Avoid
- **Re-deriving the reserved-key list in a third place:** Only `internal/embed` should own
  the literal `["model", "input"]`; `internal/config` imports it. Do not duplicate the literal
  list a third time in a test helper — reference the exported symbol there too.
- **Comparing raw embed request bytes:** No existing test in `internal/embed/embed_test.go`
  compares raw JSON bytes — all decode into `map[string]any` first (`captureBody` pattern,
  confirmed in `TestEmbedParamsMergedIntoBody`). Collapsing to the map-based path changes wire
  key order (struct-declared `model,input` → map-marshal alphabetical `input,model`) — this is
  JSON-semantically identical and already anticipated by the existing code comment at
  `embed.go:230` ("Go sorts map keys on marshal; that is JSON-semantically identical, so callers
  compare decoded objects, not raw bytes."). Do not add a new byte-comparison test that would
  make this collapse look "breaking" — it isn't, by the codebase's own established contract.
- **Hand-editing `gen/` or `ui/src/lib/gen/`:** Both are generated; edit only
  `proto/engram/v1/engram.proto` and run `task proto:gen` (already re-vendors the TS tree per
  the Phase-19 pattern — no separate manual copy step needed).
- **Conditionally setting Kind/Citations only "for discovery records" in `memoryToProto`:**
  Unnecessary — `store.Memory.Kind`/`.Citations` are only ever populated by `storeDiscovery`
  (`internal/server/tools.go:769`, `:749`); on every other write path they are the Go zero value
  (`""`, `nil`). Just copy them through unconditionally; no `if m.Category == "discovery"` guard
  needed.
- **Skipping `task ui:build` because "no UI code changed":** The `ui-drift` CI job rebuilds the
  SPA from source on every run regardless of what changed and diffs the embedded
  `internal/webauth/static/`. If `ui/src/lib/gen/engram_pb.ts`'s regenerated type shape changes
  build output at all, CI goes red without a local `task ui:build` + commit. Run it defensively
  even though D-02 means no *behavioral* SPA change is expected.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bounded retry loop | A generic retry/backoff library | A plain `for i := 0; i < maxMintAttempts; i++` loop | This is a uniqueness-check loop against Qdrant `Count`, not a network-flakiness retry — no backoff/jitter semantics needed; a bounded counter is the entire fix (D-04's 16 cap). |
| Shared constant list | A new micro-package for "shared embed constants" | Export `ReservedParamKeys` directly from `internal/embed` (already has zero internal deps; `internal/config` gains a clean one-way import) | Issue #304 itself proposed this exact shape; no cyclic-import risk (`internal/embed` currently imports only `internal/telemetry`, stdlib, otel). |
| CronJob env plumbing | A separate `ConfigMap`/`Secret` sync mechanism between Deployment and CronJob | Helm named template (`_helpers.tpl`) `include`d by both | Single source of truth at render time; zero runtime coupling, zero drift risk — exactly what #269 was filed to fix (real production drift incidents cited in the issue: image-tag drift #1459, collection drift #1546). |

**Key insight:** Every one of these six fixes is intentionally small — the "don't hand-roll"
risk here is scope creep (e.g., adding retry/backoff to MintShortID, or building a general
CronJob templating helper beyond this one job). Stay mechanical.

## Common Pitfalls

### Pitfall 1: Treating #303 and D-03(#307/summary) as code changes when they are verification-only
**What goes wrong:** A plan writes a diff to `storeDiscoveryArgs.ID`'s jsonschema tag or to
`memoryToProto`'s `Summary:` line, producing a no-op or (worse) accidentally reverting the
already-correct wording.
**Why it happens:** CONTEXT.md flagged both as open questions precisely because the original
GitHub issues describe a state that predates the current HEAD; the code was already fixed by
the time this phase started planning.
**How to avoid:** Plan A's #303 task should be "read tools.go:550, confirm it already contains
`short_id`, add/keep a regression test asserting the jsonschema string, comment+close GitHub
#303 citing commit `92a6f610`." No production code diff expected. Likewise `summary` in
`memoryToProto` needs zero changes — only `kind`/`citations` are new lines.
**Warning signs:** A `git diff` on `internal/server/tools.go` for the #303 plan that touches
line 550 without a preceding failing test proving the current text was wrong.

### Pitfall 2: buf-breaking false-positive from field reordering in generated code
**What goes wrong:** Editing the `.proto` file by hand can accidentally reorder or touch
existing field declarations while adding the new ones, tripping `buf breaking --against main`.
**Why it happens:** Fields 1-20 in `Memory` are declared in numeric order in the source; it's
easy to paste the new fields in the wrong spot or touch a comment on an adjacent existing field.
**How to avoid:** Append `kind = 21` and `citations = 22` strictly after `last_accessed_at = 20`,
touching no other line in the message. Run `go tool buf breaking --against
'https://github.com/seanb4t/engram.git#branch=main'` locally before pushing (mirrors the CI
`buf` job step).
**Warning signs:** `git diff proto/engram/v1/engram.proto` showing more than a 2-line addition.

### Pitfall 3: CronJob env drift reintroduced by a values.yaml default mismatch
**What goes wrong:** The CronJob's `args` hardcode `--all-scopes`, but if a future values.yaml
knob for `--scope` is added without updating both the Deployment's mental model and docs, the
sweep silently narrows.
**Why it happens:** `summarize-missing` requires exactly one of `--scope`/`--all-scopes`
(`cmd/engram/summarize.go:37-39`); an unattended CronJob with no `--scope` default MUST pass
`--all-scopes` or every invocation errors out.
**How to avoid:** Hardcode `--all-scopes` in the CronJob template for D-07/D-08's stated
"scheduled sweep" use case; do not expose a `--scope` values.yaml knob this phase (out of scope
per the CONTEXT deferred-ideas "no `--older-than`-style bound... unless operational need
arises").
**Warning signs:** `helm template --set memory.summarize.cronjob.enabled=true charts/engram`
rendering a Job with no `--all-scopes`/`--scope` arg.

### Pitfall 4: MintShortID's `seen`-map dedup accidentally counted against the cap
**What goes wrong:** A naive bounded-loop rewrite increments the attempt counter on every loop
iteration (including `seen`-map hits), silently violating D-05 and making batch-mint callers
(`BackfillShortIDs`, which passes a growing `seen` map) far more likely to hit exhaustion simply
because they've minted many ids in the same run.
**Why it happens:** The most natural refactor (`for i := 0; i < max; i++`) puts the increment at
the loop head, before the `seen`-map `continue` check.
**How to avoid:** Increment the counter only after the Qdrant `Count` call succeeds/fails —
i.e., only a real collision-check attempt counts. The existing `seen[cand]` dup-continue at
`store.go:1786` must `continue` **without** touching the counter.
**Warning signs:** `TestMintShortIDRetriesOnCollision` (existing, line 2741) or a new
batch-mint test starts flaking/exhausting after adding the cap.

## Code Examples

### Bounded MintShortID (full function shape)
```go
// Source: internal/store/store.go MintShortID (HEAD lines 1763-1807), rewritten for D-04/D-05/D-06
const maxMintAttempts = 16 // D-04: extra headroom over the ~8 that is already astronomically safe in 32^10

var ErrShortIDExhausted = errors.New("short id mint exhausted")

func (s *Store) MintShortID(ctx context.Context, seen map[string]struct{}) (id string, err error) {
	ctx, span := tracer.Start(ctx, "store.MintShortID")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "MintShortID", start, err) // D-06: exhaustion rides this existing wrapper for free
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	gen := s.mintCandidate
	if gen == nil {
		gen = shortid.New
	}
	for attempts := 0; attempts < maxMintAttempts; attempts++ {
		cand, genErr := gen()
		if genErr != nil {
			err = genErr
			return "", err
		}
		if seen != nil {
			if _, dup := seen[cand]; dup {
				attempts-- // D-05: seen-map hits do not consume the real-collision-check budget
				continue
			}
		}
		n, countErr := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection,
			Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("short_id", cand)}},
			Exact:          qdrant.PtrOf(true),
		})
		if countErr != nil {
			err = countErr
			return "", err
		}
		if n == 0 {
			if seen != nil {
				seen[cand] = struct{}{}
			}
			id = cand
			return id, nil
		}
	}
	err = fmt.Errorf("%w: after %d attempts", ErrShortIDExhausted, maxMintAttempts)
	return "", err
}
```
Note: `attempts--` inside a `for i := 0; i < max; i++` loop is a valid but slightly unusual
idiom — an equally clean alternative is a separate `checked := 0` counter incremented only on
the real Count() call, checked at the top of an infinite `for` loop (`if checked >= max { return
exhaustion }`). Either satisfies D-04/D-05; pick whichever the planner finds more readable.

### Single-path embed() body build
```go
// Source: internal/embed/embed.go embed() (HEAD lines 227-242), collapsed per #302
// ReservedParamKeys are the request-body keys the embedder sets authoritatively;
// operator-supplied params (ENGRAM_EMBED_*_PARAMS) must never override them.
// Exported so internal/config.ParseEmbedParams shares this exact list (#304) —
// see internal/config/embedparams.go.
var ReservedParamKeys = []string{"model", "input"}

// in embed():
m := make(map[string]any, len(params)+2)
for k, v := range params {
	m[k] = v
}
m["model"] = c.model
m["input"] = text
body, _ := json.Marshal(m)
```
```go
// Source: internal/config/embedparams.go ParseEmbedParams (HEAD lines 17-35), updated for #304
for _, k := range embed.ReservedParamKeys {
	if _, exists := m[k]; exists {
		return nil, fmt.Errorf("%s: must not contain reserved key %q", name, k)
	}
}
```
Import check performed: `internal/config` currently imports no `internal/*` packages;
`internal/embed` currently imports only `internal/telemetry`. `internal/config` → `internal/embed`
is a new one-way edge with no cycle risk (confirmed via `go build ./...`, clean at HEAD).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `MintShortID` unbounded `for {}` | Bounded 16-attempt loop + `ErrShortIDExhausted` | This phase | A pathological Qdrant-state bug now surfaces as a normal write error instead of hanging the request indefinitely |
| `embed()` two body-build paths | Single map-based path | This phase | Removes a code-path fork that only existed to preserve exact prior wire-byte order — a property no test or consumer actually depends on |
| `summarize-missing` run manually / via hand-rolled out-of-chart CronJob (per issue #269, the `fzymgc-house` cluster's Source-B CronJob) | In-chart `batch/v1` CronJob, opt-in, sharing Deployment env via `_helpers.tpl` | This phase | Eliminates the two drift incidents cited in #269 (image-tag drift, collection-name drift) by construction |

**Deprecated/outdated:**
- Hand-rolled out-of-chart summarize CronJobs (as run today on `fzymgc-house`) — superseded by
  the in-chart `cronjob.enabled: true` knob once this phase ships; the docs-site upgrade guide
  should note the migration path (out of scope to write this phase unless trivial).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `--all-scopes` (not a per-namespace `--scope`) is the correct CronJob default sweep mode | Architecture Patterns / Pattern 4, Pitfall 3 | If an operator wants a scoped-only sweep, the CronJob would need a values.yaml `scope` override added later — low risk, additive |
| A2 | The Helm values key path should be `memory.summarize.cronjob.*` (not `memory.summarize.cron.*` as GitHub issue #269's suggested shape used) | Code Examples / Pattern 4 | Purely a naming choice (Claude's Discretion per CONTEXT.md); either is internally consistent, no functional risk |
| A3 | `citationsToProto` is the right name/location (connectapi.go, not protoconv.go) for the new store→proto Citation mapper | Architecture Patterns / Pattern 2 | Cosmetic only — plan-checker or reviewer may prefer a different name/location; no behavior risk |

**If this table is empty:** N/A — see entries above; none of these are compliance/security/
retention claims, all are naming/default-value judgment calls explicitly delegated to Claude's
Discretion by CONTEXT.md.

## Open Questions

None outstanding — both open research questions posed by CONTEXT.md (D-03 summary field status;
#303 residual gap) are resolved above with direct code citations. No further research needed
before planning.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `go tool buf` | proto:gen/proto:lint (#307) | ✓ (declared in go.mod `tool` directives) | repo-pinned | — |
| `pnpm` / Node | ui:build re-vendor + SPA rebuild (#307) | ✓ (CI uses pnpm@11 / node 26; assume present in exec environment) | pnpm 11 | If unavailable locally, defer SPA rebuild verification to CI's `ui-drift` job |
| `helm` | chart:lint / template rendering (#269) | ✓ (CI job `chart` uses azure/setup-helm v5) | — | — |
| Qdrant (live, for store tests) | `TestMintShortIDRetriesOnCollision`-style tests (#308) | Assumed via existing testcontainers-Qdrant suite already in CI | — | — |

No missing dependencies with no fallback identified.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + testcontainers-Qdrant for live-store tests; `helm lint`/`helm template` for chart validation |
| Config file | none — `go test ./...` via Taskfile `test:go`; chart validation via Taskfile `chart:lint` |
| Quick run command | `go test ./internal/store/... ./internal/embed/... ./internal/config/... ./internal/server/... -run <Test>` |
| Full suite command | `task` (lint + test) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-discovery-proto-fidelity | `SearchDiscoveries` response carries `kind`+`citations` (summary already worked) | unit | `go test ./internal/server/... -run TestMemoryToProto -v` (extend existing or new test asserting `Kind`/`Citations` round-trip) | ❌ Wave 0 (new test asserting the extended fields) |
| REQ-discovery-proto-fidelity | proto/gen drift stays clean | ci-gate | `go tool buf lint && go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main' && go tool buf generate && git diff --exit-code -- gen/` | ✅ existing CI `buf` job |
| REQ-shortid-mint-cap | `MintShortID` returns `ErrShortIDExhausted` after 16 forced-collision attempts | unit | `go test ./internal/store/... -run TestMintShortIDExhaust -v` (new, mirrors existing `TestMintShortIDRetriesOnCollision` at store_test.go:2741) | ❌ Wave 0 |
| REQ-shortid-mint-cap | seen-map dup does not consume attempt budget | unit | same new test, assert with a `seen` map pre-populated with N dup candidates that exhaustion still happens at exactly 16 real Count() calls | ❌ Wave 0 |
| REQ-embed-param-key-sharing | `config.ParseEmbedParams` rejects reserved keys sourced from `embed.ReservedParamKeys` | unit | `go test ./internal/config/... -run TestEmbedParams -v` (existing `embedparams_test.go`, extend assertion to reference the shared symbol) | ✅ existing file, extend |
| REQ-embed-body-build-collapse | single-path `embed()` still produces correct decoded body for empty AND non-empty params | unit | `go test ./internal/embed/... -run TestEmbedParamsMergedIntoBody -v` (existing, decode-based — should pass unmodified) | ✅ existing |
| REQ-discovery-shortid-schema | `storeDiscoveryArgs.ID` jsonschema tag contains `short_id` | unit | `go test ./internal/server/... -run TestStoreDiscoveryArgsSchema -v` (new — asserts the literal tag string so a future regression is caught) | ❌ Wave 0 (currently unasserted — the field is correct but nothing pins it) |
| REQ-summarize-cronjob | `helm template` renders a valid `batch/v1` CronJob when enabled, and the Deployment's env is unchanged | integration/render | `helm template charts/engram --set memory.summarize.cronjob.enabled=true \| grep -A5 'kind: CronJob'` + `diff <(helm template charts/engram --show-only templates/memory-mcp.yaml) <(git stash; helm template charts/engram --show-only templates/memory-mcp.yaml; git stash pop)` | ❌ Wave 0 (no existing chart test harness — see gap below) |

### Sampling Rate
- **Per task commit:** the scoped `go test ./internal/<pkg>/... -run <Test>` for the package touched by that task
- **Per wave merge:** `task` (full lint + test) plus `helm lint charts/engram && helm template charts/engram` (both default-disabled and `--set memory.summarize.cronjob.enabled=true` renders)
- **Phase gate:** Full suite green before `/gsd-verify-work`, plus a manual `git diff` inspection confirming the Deployment's rendered env is byte-identical pre/post the `_helpers.tpl` factor-out

### Wave 0 Gaps
- [ ] `internal/server/connectapi_test.go` (or wherever `memoryToProto` is currently tested) — add a case asserting `Kind`/`Citations` round-trip through `memoryToProto`/`memoriesToProto`
- [ ] `internal/store/store_test.go` — add `TestMintShortIDExhaustsAfterCap` (forced-collision `mintCandidate`/mocked `Count` always non-zero, or the existing test-double pattern extended to force every attempt to "collide")
- [ ] `internal/server/tools_test.go` — add a small assertion pinning the `storeDiscoveryArgs.ID` jsonschema string (currently unasserted — nothing would catch a regression that silently dropped the `short_id` wording again, which is precisely how #303 was filed)
- [ ] No chart test harness exists (`charts/engram/` has no `tests/` dir). This phase should add a
      lightweight validation, NOT a full `helm unittest` framework — e.g. a Taskfile target or a
      short bash script that (a) renders with `cronjob.enabled=false` (default) and asserts no
      `kind: CronJob` in output, (b) renders with `cronjob.enabled=true` and asserts `kind:
      CronJob` + the expected `schedule`/`concurrencyPolicy` values are present, (c) diffs the
      Deployment's rendered env before/after the `_helpers.tpl` refactor is a no-op. Keep this
      proportional — a full `helm-unittest` plugin dependency is likely overkill for one chart
      validation; a bash+grep/diff script in `chart:lint` is sufficient and adds no new tooling
      dependency.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | This phase touches no auth path |
| V3 Session Management | no | No session changes |
| V4 Access Control | no | `memoryToProto`'s new fields ride the exact same authz-gated `SearchDiscoveries` handler already in production — no new read surface, just more fields on an already-authorized response. No new write surface. |
| V5 Input Validation | no (marginal) | The CronJob's `--all-scopes`/`--timeout` args are Helm-values-controlled (operator-trusted), not user input. `ParseEmbedParams`' reserved-key validation is unchanged in strictness, only its key-list source moves. |
| V6 Cryptography | no | No crypto touched |

**No new attack surface this phase.** #307 exposes two additional read-only fields
(`kind`/`citations`) on an RPC (`SearchDiscoveries`) whose authorization gate (per-actor +
`shared`-record read rules) is entirely upstream of `memoryToProto` and unchanged. #269's
CronJob runs the same binary, same image, same env, as a scheduled batch job with no exposed
network port (no `Service`/`Ingress` for the CronJob) — no new externally-reachable surface.

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Discovery record fields leaking across owner boundary via the new proto fields | Information Disclosure | Not a new risk — `kind`/`citations` ride the same per-record authz-filtered `Memory` conversion every other field already uses; no separate authz check is introduced or needed |
| CronJob running with overly-broad env/secrets it doesn't need | Elevation of Privilege (blast radius) | Accepted trade-off: D-09 explicitly reuses the full Deployment env (including OIDC/UI secrets the summarize command never reads) to guarantee zero drift, at the cost of the CronJob pod having secrets it doesn't use. This mirrors the Deployment's own existing secret-mount posture (same `caBundle`/API-key secrets); no new secret is introduced. If tightened later, that's a separate hardening phase, not this one. |

## Sources

### Primary (HIGH confidence)
- Live codebase at HEAD (`24bb8461`) — every file:line claim in this document was verified via
  direct `Read`/`rg` against the actual files, not inferred from CONTEXT.md.
- `git log -L` / `git log -p -S` on `internal/server/tools.go` and `internal/embed/embed.go` —
  confirmed exact provenance of the "short_id" jsonschema wording (commit `92a6f610`, PR #288).
- `gh issue view` for #307, #308, #304, #302, #269, #303 — original issue bodies (not just
  REQUIREMENTS.md's one-line summaries) read directly from GitHub, confirming CONTEXT.md's
  framing and surfacing the #302 wire-byte-order nuance and the #269 real-world drift incidents.
- `.github/workflows/ci.yaml` — confirmed exact CI gate commands (`buf` job's breaking-change +
  drift checks; `ui-drift` job's unconditional SPA rebuild-and-diff; `chart` job's `helm lint`
  only, no `helm template` diff gate today).
- `Taskfile.yaml` — confirmed `proto:gen` already performs the TS re-vendor copy (no separate
  manual step needed per the Phase-19 `19-01-PLAN.md` pattern, cross-checked directly).

### Secondary (MEDIUM confidence)
- None — all claims in this phase were directly verifiable against the local repo and GitHub
  issues; no external web research was needed (this is an internal-codebase hardening phase with
  zero new third-party dependencies or APIs).

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: N/A — no new libraries; HIGH confidence code stays within existing idioms
  (sentinel errors, `errors.Is`, Go stdlib `encoding/json`, existing Helm chart structure)
- Architecture: HIGH — every pattern cited was read directly from the live source, not assumed
- Pitfalls: HIGH — each pitfall is backed by a specific line-level code observation (existing
  test seams, existing code comments anticipating the exact concern, issue-body nuances)

**Research date:** 2026-07-15
**Valid until:** Until this phase's code lands (internal-only research; not time-sensitive to
external API/library churn — the only "staleness" risk is another concurrent phase claiming
proto fields 21/22 first, which the `buf breaking` CI gate would catch immediately)
