# Phase 20: Correctness & Polish - Pattern Map

**Mapped:** 2026-07-15
**Files analyzed:** 11 (3 new, 8 modified)
**Analogs found:** 11 / 11

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `charts/engram/templates/_helpers.tpl` | config (Helm named template) | transform | `charts/engram/templates/memory-mcp.yaml` (env block L30-163) | exact (factor-out source) |
| `charts/engram/templates/summarize-cronjob.yaml` (NEW) | config (Helm CronJob) | batch | `charts/engram/templates/memory-mcp.yaml` (Deployment) | role-match |
| `charts/engram/values.yaml` | config | — | existing `memory.summarize:` block (L95-99) | exact (sibling block) |
| `proto/engram/v1/engram.proto` (`Memory` message) | model (wire schema) | transform | same message, `last_accessed_at = 20` field + existing `Citation` message (L122) | exact |
| `internal/server/connectapi.go` (`memoryToProto` + new `citationsToProto`) | transform / mapper | transform | same file's `memoriesToProto`; sibling inverse-direction helpers `citationToArg`/`citationsToArgs` in `protoconv.go` | exact (naming symmetry) |
| `internal/store/store.go` (`MintShortID`) | service (store layer) | CRUD (retry-until-unique) | same function's existing unbounded loop; sibling sentinel `ErrAmbiguousShortID` (L56, use at L1226) | exact |
| `internal/store/store_test.go` (`TestMintShortIDExhaustsAfterCap`, new) | test | CRUD | `TestMintShortIDRetriesOnCollision` (L2741-2761) | exact |
| `internal/embed/embed.go` (`embedReq`/`embed()`) | service (HTTP client) | request-response | same function's two-path body build (L227-242) | exact (self-refactor) |
| `internal/config/config.go` / `internal/config/embedparams.go` (`ParseEmbedParams`) | utility (validation) | transform | same function's inline `[]string{"model","input"}` literal (L29) | exact (self-refactor) |
| `internal/server/tools.go` (`storeDiscoveryArgs.ID` jsonschema) | model (MCP tool schema) | request-response | already correct — verification only | n/a |
| `internal/server/tools_test.go` (jsonschema-string assertion, new) | test | request-response | no existing schema-string-pinning test found; pattern from table-driven Go tests elsewhere in the file | role-match |

## Pattern Assignments

### `charts/engram/templates/_helpers.tpl` (NEW) — config, transform

**Analog:** `charts/engram/templates/memory-mcp.yaml` (current inline `env:` block, lines 30-163)

No `_helpers.tpl` exists yet in this chart (only `expose.yaml`, `memory-mcp.yaml`, `qdrant.yaml`). Create the named template wrapping the **exact, byte-identical** current env list:

```yaml
{{- define "engram.containerEnv" -}}
- { name: ENGRAM_LISTEN_ADDR, value: "{{ .Values.memory.listenAddr }}" }
- { name: ENGRAM_QDRANT_ADDR, value: "qdrant.{{ .Release.Namespace }}.svc.cluster.local:6334" }
- { name: ENGRAM_QDRANT_COLLECTION, value: "{{ .Values.memory.qdrant.collection }}" }
- { name: ENGRAM_OPENAI_BASE_URL, value: "{{ .Values.memory.openai.baseURL }}" }
- { name: ENGRAM_EMBED_MODEL, value: "{{ .Values.memory.embed.model }}" }
- { name: ENGRAM_EMBED_DIM, value: "{{ .Values.memory.embed.dim }}" }
{{- with .Values.memory.embed.queryInstruction }}
- { name: ENGRAM_EMBED_QUERY_INSTRUCTION, value: {{ . | quote }} }
{{- end }}
{{- with .Values.memory.embed.queryParams }}
- { name: ENGRAM_EMBED_QUERY_PARAMS, value: {{ . | quote }} }
{{- end }}
{{- with .Values.memory.embed.documentParams }}
- { name: ENGRAM_EMBED_DOCUMENT_PARAMS, value: {{ . | quote }} }
{{- end }}
{{- with .Values.memory.embed.documentInstruction }}
- { name: ENGRAM_EMBED_DOCUMENT_INSTRUCTION, value: {{ . | quote }} }
{{- end }}
{{- if .Values.memory.openai.apiKeySecret.name }}
- name: ENGRAM_OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: "{{ .Values.memory.openai.apiKeySecret.name }}"
      key: "{{ .Values.memory.openai.apiKeySecret.key }}"
{{- end }}
{{- /* ... every remaining var verbatim through the OTEL_RESOURCE_ATTRIBUTES
       Downward-API block and the conditional SSL_CERT_FILE var ... */}}
{{- if .Values.memory.openai.caBundle.configMapName }}
- { name: SSL_CERT_FILE, value: "/etc/ssl/ca-bundle/{{ .Values.memory.openai.caBundle.key }}" }
{{- end }}
{{- end -}}
```

Then `memory-mcp.yaml`'s `env:` key becomes:
```yaml
          env: {{- include "engram.containerEnv" . | nindent 12 }}
```

**Critical constraint (D-09 + Pitfall in RESEARCH.md Pattern 4):** copy every line L31-163 of `memory-mcp.yaml` **verbatim** into the `define` block — do not touch a single line's content/formatting; only change the indentation context (Deployment nindent is 12, CronJob will be 16). Verify with `diff <(helm template ... --show-only templates/memory-mcp.yaml)` before/after — must be a no-op.

---

### `charts/engram/templates/summarize-cronjob.yaml` (NEW) — config, batch

**Analog:** `charts/engram/templates/memory-mcp.yaml` Deployment (image/imagePullSecrets/caBundle volume conventions), `cmd/engram/summarize.go:37-39` (arg contract).

```yaml
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
              args: ["summarize-missing", "--all-scopes"]
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

`imagePullSecrets`/`caBundle` volume conventions copied verbatim from `memory-mcp.yaml` L21-23, 159-169, 179-184. **Copy `imagePullPolicy`/`image` construction exactly** — same `.Values.image.*` keys, no new values added.

**Reminder D-04 pitfall (`cmd/engram/summarize.go:37-39`):** `summarize-missing` requires exactly one of `--scope`/`--all-scopes`; the CronJob must hardcode `--all-scopes` — do not add a `--scope` values.yaml knob this phase.

---

### `charts/engram/values.yaml` — config

**Analog:** existing `memory.summarize:` block (lines 95-99):
```yaml
  summarize:
    model: "" # ENGRAM_SUMMARY_MODEL, e.g. "gpt-4o-mini"; empty disables auto-summary
    maxChars: 280 # ENGRAM_SUMMARY_MAX_CHARS
    maxTokens: 1024 # ENGRAM_SUMMARY_MAX_TOKENS
    timeout: 30s # ENGRAM_SUMMARY_TIMEOUT
```
Add a nested `cronjob:` block in the same commented-inline style (D-07/D-08 knobs):
```yaml
  summarize:
    # ...existing keys unchanged...
    cronjob:
      enabled: false # D-07: opt-in; only meaningful when summarize.model is set
      schedule: "0 3 * * *" # daily
      successfulJobsHistoryLimit: 3
      failedJobsHistoryLimit: 1
```

---

### `proto/engram/v1/engram.proto` (`Memory` message) — model, transform

**Analog:** the message's own existing field-numbering convention; `Citation` message being reused verbatim.

Current tail (verified live, `last_accessed_at = 20` is the highest field):
```protobuf
message Memory {
  // ... fields 1-20 unchanged ...
  google.protobuf.Timestamp last_accessed_at = 20;
}
```
Append strictly after field 20, touching no other line:
```protobuf
  // Discovery-only fields; empty on plain memories (never set by
  // store_memory/schedule_memory, only by store_discovery).
  string kind = 21;
  repeated Citation citations = 22;
```
`Citation` message already exists at `engram.proto:122` (used by `StoreDiscoveryRequest`) — no new message type. Run `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'` before pushing (mirrors CI `buf` job); `git diff` on this file should show exactly a 2-3 line addition (Pitfall 2 in RESEARCH.md).

---

### `internal/server/connectapi.go` (`memoryToProto` + new `citationsToProto`) — transform, transform

**Analog:** same file, `memoryToProto`/`memoriesToProto` (lines 32-60, read above); inverse-direction sibling `citationToArg`/`citationsToArgs` in `protoconv.go` (different file — write path, do NOT put the new helper there).

Current `memoryToProto` (verbatim, lines 32-52):
```go
func memoryToProto(m store.Memory) *engramv1.Memory {
	// LastAccessedAt is nil for never-accessed records; leave the proto field
	// unset rather than emitting a year-1 (0001-01-01) Timestamp.
	var lastAccessed *timestamppb.Timestamp
	if m.LastAccessedAt != nil {
		lastAccessed = timestamppb.New(*m.LastAccessedAt)
	}
	return &engramv1.Memory{
		Id: m.ID, Content: m.Content, Scope: m.Scope,
		Repo: m.Repo, Workspace: m.Workspace, Worktree: m.Worktree, BaseDir: m.BaseDir,
		Source: m.Source, Category: m.Category, Tags: m.Tags,
		Actor: m.Actor, Owner: m.Owner, Visibility: m.Visibility,
		CreatedAt:      timestamppb.New(m.CreatedAt),
		Summary:        m.Summary,
		SummarySource:  string(m.SummarySource),
		Score:          m.Score,
		ShortId:        m.ShortID,
		AccessCount:    m.AccessCount,
		LastAccessedAt: lastAccessed,
	}
}
```
Extend the struct literal with two new lines and add the sibling helper right above/below it, naming-symmetric with `memoriesToProto`:
```go
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

// inside memoryToProto's return literal, add:
		Kind:      m.Kind,
		Citations: citationsToProto(m.Citations),
```
No conditional guard on `m.Category == "discovery"` — `Kind`/`Citations` are Go zero-value (`""`/`nil`) on every non-discovery write path already; copy unconditionally (Anti-pattern in RESEARCH.md).

---

### `internal/store/store.go` (`MintShortID`) — service, CRUD

**Analog:** same function's current unbounded loop (verbatim, lines 1763-1807, read above); sentinel-error idiom from `ErrAmbiguousShortID` (line 56, used at line 1226):
```go
// existing sibling sentinel, line 56 + its call site line 1226
var ErrAmbiguousShortID = errors.New("ambiguous short id")
// ...
return "", fmt.Errorf("%w: %s", ErrAmbiguousShortID, canonical)
```
New sentinel, same idiom, placed beside it:
```go
var ErrShortIDExhausted = errors.New("short id mint exhausted")
```
Bounded-loop rewrite (D-04 cap=16, D-05 seen-dup must not consume budget, D-06 rides existing telemetry deferred-func for free — that wrapper at lines 1767-1772 needs zero changes):
```go
const maxMintAttempts = 16

func (s *Store) MintShortID(ctx context.Context, seen map[string]struct{}) (id string, err error) {
	ctx, span := tracer.Start(ctx, "store.MintShortID")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "MintShortID", start, err)
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
				attempts-- // D-05: seen-map hits don't consume the real-collision-check budget
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
**Pitfall to avoid:** do not increment a counter at the loop head before the `seen`-map dup check — that would violate D-05 and make batch callers (`BackfillShortIDs`) exhaust prematurely.

---

### `internal/store/store_test.go` (`TestMintShortIDExhaustsAfterCap`, NEW) — test, CRUD

**Analog:** `TestMintShortIDRetriesOnCollision` (lines 2741-2761, read above):
```go
func TestMintShortIDRetriesOnCollision(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000031", ShortID: "collidecol", Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	st.mintCandidate = func() (string, error) {
		calls++
		if calls == 1 {
			return "collidecol", nil // taken → must retry
		}
		return "freshfresh", nil
	}
	got, err := st.MintShortID(ctx, nil)
	if err != nil || got != "freshfresh" || calls != 2 {
		t.Fatalf("got %q err %v calls %d (want freshfresh / 2)", got, err, calls)
	}
}
```
New test mirrors this shape but makes `mintCandidate` return a distinct always-colliding value every call (so every attempt reaches the real `Count` check), then asserts `errors.Is(err, ErrShortIDExhausted)` and `calls == 16`:
```go
func TestMintShortIDExhaustsAfterCap(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000032", ShortID: "alwaystaken", Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	st.mintCandidate = func() (string, error) {
		calls++
		return "alwaystaken", nil // every candidate collides — forces exhaustion
	}
	_, err := st.MintShortID(ctx, nil)
	if !errors.Is(err, ErrShortIDExhausted) {
		t.Fatalf("err = %v, want ErrShortIDExhausted", err)
	}
	if calls != 16 {
		t.Fatalf("calls = %d, want 16", calls)
	}
}
```
Also add a `seen`-map variant per D-05 test-map requirement: pre-populate `seen` with a few dup candidates, assert exhaustion still happens at exactly 16 real `Count()` calls (not 16 total loop iterations).

---

### `internal/embed/embed.go` (`embedReq` / `embed()`) — service, request-response

**Analog:** same function's current two-path body build (verbatim, lines 227-242, read above):
```go
	// Empty params → marshal the struct (exact prior wire bytes; default path).
	// Non-empty → merge params first, then set model/input last so they are
	// always authoritative. Go sorts map keys on marshal; that is JSON-
	// semantically identical, so callers compare decoded objects, not raw bytes.
	var body []byte
	if len(params) == 0 {
		body, _ = json.Marshal(embedReq{Model: c.model, Input: text})
	} else {
		m := make(map[string]any, len(params)+2)
		for k, v := range params {
			m[k] = v
		}
		m["model"] = c.model
		m["input"] = text
		body, _ = json.Marshal(m)
	}
```
Collapse to a single map-based path (#302), and export the reserved-key list (#304) right beside `embedReq`:
```go
// ReservedParamKeys are the request-body keys the embedder sets authoritatively;
// operator-supplied params (ENGRAM_EMBED_*_PARAMS) must never override them.
// Exported so internal/config.ParseEmbedParams shares this exact list (#304).
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
`embedReq` struct (lines 136-139) can stay (harmless, may still be referenced by tests) or be removed if unused after the collapse — check `embed_test.go` for `embedReq{...}` construction before deleting. **Do not add a raw-byte-comparison test** — existing tests decode into `map[string]any` first (the code comment at line 230 already documents key-order is JSON-semantically fine); this is the established contract, not new.

---

### `internal/config/config.go` / `internal/config/embedparams.go` (`ParseEmbedParams`) — utility, transform

**Analog:** same function's current inline literal (verbatim, lines 17-35, read above):
```go
func ParseEmbedParams(name, s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("%s: must be a JSON object: %w", name, err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: must be a JSON object, got %T", name, v)
	}
	for _, k := range []string{"model", "input"} {
		if _, exists := m[k]; exists {
			return nil, fmt.Errorf("%s: must not contain reserved key %q", name, k)
		}
	}
	return m, nil
}
```
Replace the inline literal with a reference to the shared exported slice (#304), adding the import edge `internal/config` → `internal/embed` (confirmed no cycle — `internal/config` currently imports no `internal/*` packages; `internal/embed` currently imports only `internal/telemetry`):
```go
import "github.com/seanb4t/engram/internal/embed"
// ...
	for _, k := range embed.ReservedParamKeys {
		if _, exists := m[k]; exists {
			return nil, fmt.Errorf("%s: must not contain reserved key %q", name, k)
		}
	}
```
File is `internal/config/embedparams.go`, not `config.go` — `ParseEmbedParams` lives in the sibling file, not the `Config` struct file read above.

---

### `internal/server/tools.go` (`storeDiscoveryArgs.ID`) — verification only, no diff expected

**Confirmed live at HEAD** (lines 543-551):
```go
type storeDiscoveryArgs struct {
	Content   string        `json:"content" jsonschema:"the understanding to cache (embedded + searched)"`
	Kind      string        `json:"kind" jsonschema:"map (orientation) or fact (pinned checkable claim)"`
	Citations []citationArg `json:"citations" jsonschema:"at least one source anchor"`
	Scope     string        `json:"scope" jsonschema:"discovery scope, must start with discovery: (e.g. discovery:repo:<repo>)"`
	Tags      []string      `json:"tags,omitempty"`
	Summary   string        `json:"summary,omitempty"`
	ID        string        `json:"id,omitempty" jsonschema:"omit to create; supply the full UUID or short_id to replace in place"`
}
```
The `.ID` field's jsonschema tag **already contains** `"...supply the full UUID or short_id to replace in place"` — #303 is already resolved (confirmed via `git log -S`, commit `92a6f610`, PR #288, 2026-07-06, predates this phase). **No production code change** — only add the pinning test below.

---

### `internal/server/tools_test.go` (jsonschema-string assertion, NEW) — test, request-response

**Analog:** no existing test pins a raw jsonschema tag string in this file; follow the file's general table-driven-test convention (simple `t.Run`/direct assertion is sufficient for a single literal-string pin — no table needed for one field):
```go
func TestStoreDiscoveryArgsIDSchemaAdvertisesShortID(t *testing.T) {
	f, ok := reflect.TypeOf(storeDiscoveryArgs{}).FieldByName("ID")
	if !ok {
		t.Fatal("storeDiscoveryArgs has no ID field")
	}
	tag := f.Tag.Get("jsonschema")
	if !strings.Contains(tag, "short_id") {
		t.Fatalf("storeDiscoveryArgs.ID jsonschema tag = %q, want it to mention short_id", tag)
	}
}
```
This is a regression pin only — per RESEARCH.md Pitfall 1, do NOT modify `tools.go` line 550 as part of this task.

## Shared Patterns

### Sentinel error + errors.Is
**Source:** `internal/store/store.go:56` (`ErrAmbiguousShortID`) + its call site at `internal/store/store.go:1226`
**Apply to:** `internal/store/store.go`'s new `ErrShortIDExhausted` (#308)
```go
var ErrAmbiguousShortID = errors.New("ambiguous short id")
// ...
return "", fmt.Errorf("%w: %s", ErrAmbiguousShortID, canonical)
```
`errorlint` in CI lint enforces `%w` (never `%v`/string-concat) — both existing and new sentinel wraps must use `%w`.

### Helm named-template factoring
**Source:** none yet exists in this chart — new pattern, but the source content to factor out is `charts/engram/templates/memory-mcp.yaml` lines 30-163 (verbatim).
**Apply to:** `_helpers.tpl` (new) consumed by both `memory-mcp.yaml` and `summarize-cronjob.yaml` (new).

### Store→proto conversion location discipline
**Source:** `internal/server/connectapi.go` header/file-boundary convention vs. `internal/server/protoconv.go` (that file's own header comment: "every write RPC proto request -> internal *Args mapping and every write-result -> proto response mapping lives here").
**Apply to:** the new `citationsToProto` helper — belongs in `connectapi.go` (read path: store→proto) beside `memoryToProto`/`memoriesToProto`, NOT in `protoconv.go` (write path: proto→args, where the inverse `citationToArg`/`citationsToArgs` already live).

### Additive-only proto (Phase-15 discipline)
**Source:** `proto/engram/v1/engram.proto` `Memory` message's own existing field-numbering (fields 1-20 declared in strict numeric order).
**Apply to:** the new `kind = 21` / `citations = 22` fields — append only, touch no existing line; CI enforces via `go tool buf breaking` + the `buf` job's `git diff --exit-code -- gen/` drift check.

## No Analog Found

None — all 11 files/changes have a concrete analog or are verification-only with no diff expected.

## Metadata

**Analog search scope:** `internal/server/`, `internal/store/`, `internal/embed/`, `internal/config/`, `charts/engram/templates/`, `charts/engram/values.yaml`, `proto/engram/v1/engram.proto`
**Files scanned:** `connectapi.go`, `protoconv.go` (header only), `store.go` (L1750-1810, L56, L1226), `store_test.go` (L2741-2780), `embed.go` (L120-250), `config.go` (L1-40), `embedparams.go` (full), `tools.go` (L543-582), `memory-mcp.yaml` (full), `values.yaml` (L95-110), `engram.proto` (per RESEARCH.md line citations)
**Pattern extraction date:** 2026-07-15

---
*Phase: 20-correctness-polish*
</content>
