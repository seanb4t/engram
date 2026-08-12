# Phase 1: Interface Enforceability - Pattern Map

**Mapped:** 2026-08-03
**Files analyzed:** ~14 (modified only — this phase adds no new files per RESEARCH.md's
"Recommended Project Structure")
**Analogs found:** 14 / 14 (every touched file has a same-repo precedent; this is a
refactor-in-place phase, not a greenfield one)

## File Classification

| File to Modify | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/config/registry.go` (add `client.*` fields) | config | CRUD (declare) | `embed.timeout`/`summarize.timeout` rows in the same file | exact — same file, same shape |
| `internal/config/config.go` (add `ClientConfig` struct) | config | CRUD | `EmbedConfig`/`SummarizeConfig` structs, same file | exact |
| `internal/config/client_validate.go` (NEW — separate from `Validate()`) | config | transform (pure) | `internal/config/validate.go`'s `Validate()` duration-field block | role-match, deliberately NOT the same function (Pitfall 7) |
| `internal/config/client_validate_test.go` (NEW) | test | — | `internal/config/validate_test.go`'s `validConfig()` + `TestValidateFieldRules` table | exact structural analog, new file to avoid the 33-literal tax |
| `cmd/engram/client_common.go` (retire `resolveServerURL`/token/insecure/output resolvers; add `exitTimeout`; add koanf-backed client config load) | controller/config bridge | request-response | itself (before/after diff) | exact |
| `cmd/engram/root.go` (extend `PersistentPreRunE`; add `SetFlagErrorFunc`) | controller | request-response | itself (before/after diff) | exact |
| `cmd/engram/client_list.go` (`MarkFlagsMutuallyExclusive` paging trio + scope/cross-spine) | controller (cobra command) | request-response | itself, plus `client_search.go`'s existing scope/cross-spine flag block | exact |
| `cmd/engram/client_search.go` (`MarkFlagsMutuallyExclusive` scope/cross-spine) | controller | request-response | `client_list.go`'s identical flag pair | exact |
| `cmd/engram/migrate.go` (`MarkFlagsMutuallyExclusive` + `MarkFlagsOneRequired` for `--from`/`--from-missing`/`--from-anon`; strip `selected != 1` from `buildRemapSource`) | controller | request-response | itself; `buildRemapSource` is the pure-validator precedent | exact |
| `cmd/engram/reindex.go`, `prune.go`, `summarize.go`, `backfill.go`, `serve.go` (classify returned errors into 2/3/4/5) | controller (operator command) | batch / request-response | `internal/server/connecterror.go`'s `connectError` (shape to mirror) | role-match (different vocabulary: store/gRPC not Connect) |
| `cmd/engram/catalog.go` (add `exitTimeout` entry, reword D-17 note) | config/doc | transform | itself (`doc.ExitCodes` literal, `doc.Notes`) | exact |
| `cmd/engram/exitcode_baseline_test.go` (NEW — D-09 before-table) | test | batch/table-driven | `cmd/engram/root_test.go`'s `TestExitCodeFromError` + `catalog_test.go`'s `TestCatalogExitCodesMatchMapper` + `clienttest_test.go`'s `runClient` harness | role-match — deliberately NOT `assertExitCode` |
| `docs-site/.../guides/cli.md`, `guides/upgrade.md` | docs | — | existing exit-code table / existing `## v0.12.0` upgrade section | exact |

## Pattern Assignments

### `internal/config/registry.go` + `config.go` + `validate.go` (config, CRUD) — D-04 client fields

**Analog:** `embed.timeout` / `summarize.timeout` — the full declare→bind→validate chain,
verbatim from this repo, read this session.

**1. Registry entry** (`internal/config/registry.go:36,44`):
```go
{Key: "embed.timeout", Env: "ENGRAM_EMBED_TIMEOUT", Default: "30s"},
...
{Key: "summarize.timeout", Env: "ENGRAM_SUMMARY_TIMEOUT", Default: "30s"},
```
Mirror for the 5 new client fields — note only `--server`/`--token-file`/`--output`/`--insecure`/
`--timeout` need a `Flag` value (client fields ARE cobra-overridable, unlike most server fields);
follow the `Flag:` pattern already used by `server.listen_addr`/`oidc.issuer` rows
(`registry.go:26,53-55`):
```go
{Key: "server.listen_addr", Env: "ENGRAM_LISTEN_ADDR", Legacy: "MEM_LISTEN_ADDR", Flag: "listen-addr", Default: ":8080"},
{Key: "oidc.issuer", Env: "ENGRAM_OIDC_ISSUER", Legacy: "MEM_OIDC_ISSUER", Flag: "oidc-issuer"},
```
Recommended new rows (client fields have no `Legacy` — brand-new, D-04 is additive):
```go
{Key: "client.server_url", Env: "ENGRAM_SERVER_URL", Flag: "server"},
{Key: "client.token_file", Env: "" /* deliberately none — credential must never reach argv/env registry the same way, see resolveToken's existing doc comment */, Flag: "token-file"},
{Key: "client.output", Env: "", Flag: "output"},
{Key: "client.insecure", Env: "", Flag: "insecure", Default: "false"},
{Key: "client.timeout", Env: "ENGRAM_TIMEOUT", Flag: "timeout", Default: "30s"},
```
Note: `ENGRAM_TOKEN` (the credential) is deliberately handled by `resolveToken`'s existing
env/file logic — D-13 in `client_common.go`'s doc comments says a credential must never be able
to reach argv. Do not add a `client.token` koanf field; keep `resolveToken`'s current mechanism
and only route `--token-file`'s *path* through koanf.

**2. Struct field** (`internal/config/config.go:72-76`, the `EmbedConfig.Timeout` field with its
doc-comment convention to copy):
```go
// Timeout is the per-request embed HTTP client timeout (ENGRAM_EMBED_TIMEOUT,
// default "30s"); "0" disables it (no timeout). Validated unconditionally in
// Config.Validate — the embedder is always active, unlike Summarize.Timeout
// which is gated on Summarize.Model.
Timeout string `koanf:"timeout"`
```
New `ClientConfig` struct (new type, same file, same doc-comment convention) — but see the
`SummarizeConfig` doc comment at `config.go:105-107` for the precedent on documenting a
**different** zero-semantics for a same-named flag across commands — apply the same discipline
for D-05's "0 is rejected, not unbounded" vs the operator `--timeout`'s "0 is unbounded":
```go
// ClientConfig holds every client-side flag/setting (D-04), resolved via the
// same koanf registry as server config but validated SEPARATELY (see
// client_validate.go) — Config.Validate()'s own doc comment already
// establishes this pattern for OIDC/UI fields.
type ClientConfig struct {
	ServerURL string `koanf:"server_url"`
	TokenFile string `koanf:"token_file"`
	Output    string `koanf:"output"`
	Insecure  string `koanf:"insecure"` // parsed with strconv.ParseBool, mirrors Summarize.OnWrite
	// Timeout: default "30s"; unlike EmbedConfig.Timeout / SummarizeConfig.Timeout
	// where "0" means unbounded, D-05 makes "0" a REJECTED usage error here —
	// document this divergence inline since it is the same flag concept with
	// opposite zero-semantics from the operator commands' --timeout (Pitfall 6).
	Timeout string `koanf:"timeout"`
}
```

**3. Validation rule** (`internal/config/validate.go:62-70`, the `Embed.Timeout` block —
COPY THE SHAPE, not the target function; D-05's "0 rejected" needs `d <= 0` not `d < 0`):
```go
// embed.timeout runs UNCONDITIONALLY (unlike summarize.timeout, which is
// gated on Summarize.Model) — the embedder is always active, there is no
// disabled state. 0 = no timeout (infinite), the explicit D-08 escape hatch.
switch d, err := time.ParseDuration(c.Embed.Timeout); {
case err != nil:
	errs = append(errs, fmt.Errorf("ENGRAM_EMBED_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Embed.Timeout, err))
case d < 0:
	errs = append(errs, fmt.Errorf("ENGRAM_EMBED_TIMEOUT %q: must not be negative", c.Embed.Timeout))
}
```
D-04/D-05 equivalent (in the NEW `client_validate.go`, NOT `Config.Validate()` — see Pitfall 7):
```go
switch d, err := time.ParseDuration(c.Timeout); {
case err != nil:
	errs = append(errs, fmt.Errorf("ENGRAM_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Timeout, err))
case d <= 0: // D-05: 0 is REJECTED here, unlike Embed/Summarize's "0 = unbounded"
	errs = append(errs, fmt.Errorf("ENGRAM_TIMEOUT %q: must be positive (0 is not unbounded for the client timeout)", c.Timeout))
}
```

**4. Full `Config{}` test literal** — the 33-literal tax, concretely
(`internal/config/validate_test.go:13-23`, `validConfig()`, quoted in full):
```go
func validConfig() *Config {
	return &Config{
		Qdrant:    QdrantConfig{Addr: "localhost:6334", Collection: "mem_eval"},
		Embed:     EmbedConfig{Model: "ollama/bge-m3", Dim: "1024", Timeout: "30s"},
		Memory:    MemoryConfig{MaxSummaryBytes: "512"},
		OpenAI:    OpenAIConfig{BaseURL: "http://localhost:4000"},
		Summarize: SummarizeConfig{OnWrite: "false", Workers: "2", QueueSize: "256"},
		Usage:     UsageConfig{Signals: "true"},
		Connect:   ConnectConfig{Headless: "false"},
	}
}
```
This is `Config`'s *only* full-literal helper, reused across `TestValidateHappyPath` and every row
of `TestValidateFieldRules`'s mutate-one-field table. It does NOT set `ClientConfig` today — that
proves the "separate validation function" approach in Pitfall 7 is correct: as long as
`client.*` fields are validated by a function OTHER than `Config.Validate()`, this literal (and
all other pre-existing `Config{}` literals across the package's test files) never needs to change.
If a planner instead folds client validation into `Config.Validate()`, this exact literal — and
an estimated 32 siblings — break on the new field's zero-value `""`, which is precisely the
`s780vae1vr` tax CONTEXT.md/RESEARCH.md warn about. Build the client-config test file's own
`validClientConfig()`-style helper instead, scoped to the new file only.

---

### `cmd/engram/client_common.go` (controller/config bridge) — retiring the hand-rolled resolvers

**Analog:** itself — this is a before/after diff, not a cross-file pattern borrow.

**Current shape to retire** (`client_common.go:46-89`, quoted in full — the flag → `os.Getenv` →
`usageErrorf` pattern D-04 replaces):
```go
func resolveServerURL(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if v := os.Getenv("ENGRAM_SERVER_URL"); v != "" {
		return v, nil
	}
	return "", usageErrorf("--server or ENGRAM_SERVER_URL is required")
}

func resolveToken(tokenFilePath string) (string, error) {
	if v := os.Getenv("ENGRAM_TOKEN"); v != "" {
		return v, nil
	}
	if tokenFilePath == "" {
		return "", nil
	}
	b, err := os.ReadFile(tokenFilePath)
	if err != nil {
		return "", usageErrorf("reading --token-file: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}
```
`resolveOutputFormat` (`client_common.go:174-188`) and `--insecure`'s direct `clientInsecure bool`
package var (`client_common.go:30`, bound at `addClientFlags`, `client_common.go:40-41`) are the
other two resolution sites named in CONTEXT.md's canonical_refs — same retirement shape: flag var
read directly, no koanf.

**What replaces it:** the koanf `ClientConfig` (above) loaded once via `config.Load()` +
`config.Bind(cmd)`-equivalent (mirror however `root.go`/`serve.go` currently loads `Config` for
the server side — grep `config.Load(` in `serve.go` for that exact call shape before inventing a
second one). `resolveToken`'s file-read-and-trim logic (lines 71-89) is NOT retired outright — only
its *path resolution* (`tokenFilePath` itself) moves to koanf; the read-and-trim body stays, since
it is genuinely client-local I/O, not a config value.

**Exit-code constants to extend** (`client_common.go:194-201`, quoted in full — D-06 adds
`exitTimeout = 6` here):
```go
const (
	exitOK          = 0 // success
	exitGeneric     = 1 // generic/unclassified
	exitUsage       = 2 // usage or validation error
	exitAuth        = 3 // authentication or authorization failure
	exitNotFound    = 4 // not found
	exitUnavailable = 5 // transport or server unavailable
)
```

**`cliError`/`usageErrorf` — the typed-error carrier every classification path (D-02/D-03) reuses**
(`client_common.go:203-218`, quoted in full):
```go
type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }
func (e *cliError) ExitCode() int { return e.code }

func usageErrorf(format string, a ...any) error {
	return &cliError{code: exitUsage, err: fmt.Errorf(format, a...)}
}
```
Operator commands' D-03 classification should construct `&cliError{code: exitNotFound, err: ...}`
etc. directly, or add sibling constructors (`notFoundErrorf`, `unavailableErrorf`) following
`usageErrorf`'s exact shape — do not invent a second error-carrier type.

---

### D-09 before-table (test) — the analog AND the anti-analog

**Positive analogs (build on these):**

`TestExitCodeFromError` (`cmd/engram/root_test.go:29-41`, quoted in full — proves
`exitCodeFromError` is already exported for `errors.As`-based testing and is the exact function
`Execute()` calls):
```go
func TestExitCodeFromError(t *testing.T) {
	if got := exitCodeFromError(nil); got != 0 {
		t.Errorf("exitCodeFromError(nil) = %d, want 0", got)
	}
	if got := exitCodeFromError(errors.New("plain")); got != 1 {
		t.Errorf("exitCodeFromError(plain error) = %d, want 1", got)
	}
	if got := exitCodeFromError(&cliError{code: 4, err: errors.New("not found")}); got != 4 {
		t.Errorf("exitCodeFromError(*cliError code=4) = %d, want 4", got)
	}
	wrapped := fmt.Errorf("wrapped: %w", &cliError{code: 4, err: errors.New("not found")})
	if got := exitCodeFromError(wrapped); got != 4 {
		t.Errorf("exitCodeFromError(fmt.Errorf-wrapped *cliError) = %d, want 4 (errors.As walk)", got)
	}
}
```

`TestCatalogExitCodesMatchMapper` (`cmd/engram/catalog_test.go:304-322`, quoted in full — the
set-equality discipline the before/after table should copy per memory `nczgrtfec2`: assert
distinct-where-changed, identical-where-not, never a loose "codes are as expected"):
```go
func TestCatalogExitCodesMatchMapper(t *testing.T) {
	doc := decodeCatalog(t)
	catalogCodes := make(map[int]bool)
	for _, ec := range doc.ExitCodes {
		catalogCodes[ec.Code] = true
	}
	mapperCodes := map[int]bool{exitOK: true}
	for i := 1; i <= 16; i++ {
		mapperCodes[exitCodeForConnectErr(connect.NewError(connect.Code(i), errors.New("boom")))] = true
	}
	mapperCodes[exitCodeForConnectErr(errors.New("not a connect error"))] = true
	if !reflect.DeepEqual(catalogCodes, mapperCodes) {
		t.Errorf("catalog exit codes = {%s}, mapper-producible exit codes = {%s}",
			sortedIntKeys(catalogCodes), sortedIntKeys(mapperCodes))
	}
}
```
This test needs NO edit for D-06 (self-deriving) — but see `TestCatalogListsEveryExitCode`
(catalog_test.go:218-242, hard-codes `len != 6` / `code > 5`) which WILL need its literals bumped
to 7/0-6 in the same commit (RESEARCH.md Pitfall 3, already fully quoted there — not re-quoted
here).

**The explicit anti-analog — do NOT copy this one:**

`assertExitCode` (`cmd/engram/client_search_test.go:425-437`, quoted in full):
```go
func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatalf("error %v does not carry ExitCode()", err)   // <-- FAILS on a bare error
	}
	if got := ec.ExitCode(); got != want {
		t.Errorf("ExitCode() = %d, want %d", got, want)
	}
}
```
`t.Fatal`s the moment an error lacks `ExitCode()` — but D-09's before-table exists specifically to
pin rows where the CURRENT code returns an untyped error (flag-group errors, `config.CheckLegacy`,
`buildRemapSource`'s `fmt.Errorf`). Using `assertExitCode` on those rows aborts the test instead of
recording "falls through to 1." Build the before-table directly on `exitCodeFromError(err)`
(handles both typed and untyped uniformly) plus the `runClient(t, args...)` harness
(`cmd/engram/clienttest_test.go:142-155` — not re-quoted here, returns cobra's raw `err` from
`rootCmd.Execute()`).

---

### Cobra flag-group construction — the 3 D-07 claim sites, current shape

**1. `client_list.go:91-108`** (`init()`, quoted in full — paging trio currently enforced
NOWHERE, and scope/cross-spine only enforced via the shared Go-level guard, not cobra):
```go
func init() {
	addClientFlags(listCmd)
	listCmd.Flags().StringVar(&listScope, "scope", "", "...")
	listCmd.Flags().BoolVar(&listCrossSpine, "cross-spine", false, "...")
	listCmd.Flags().Uint64Var(&listLimit, "limit", 0, "max results (0 = server default)")
	listCmd.Flags().Uint64Var(&listOffset, "offset", 0, "offset-for-UI paging; mutually exclusive with --cursor-mode")
	listCmd.Flags().StringSliceVar(&listCategories, "categories", nil, "...")
	listCmd.Flags().StringVar(&listVisibility, "visibility", "", "...")
	listCmd.Flags().StringSliceVar(&listTags, "tags", nil, "...")
	listCmd.Flags().BoolVar(&listFull, "full", false, "...")
	listCmd.Flags().StringVar(&listCreatedAfter, "created-after", "", "...")
	listCmd.Flags().StringVar(&listCreatedBefore, "created-before", "", "...")
	listCmd.Flags().StringVar(&listPageToken, "page-token", "", "opaque cursor...; when set, cursor paging (ignores --offset)")
	listCmd.Flags().BoolVar(&listCursorMode, "cursor-mode", false, "opt into cursor paging...; mutually exclusive with a non-zero --offset")
	rootCmd.AddCommand(listCmd)
}
```
D-07/D-08 add (append inside the same `init()`, before `rootCmd.AddCommand(listCmd)`):
```go
listCmd.MarkFlagsMutuallyExclusive("scope", "cross-spine")
listCmd.MarkFlagsMutuallyExclusive("offset", "cursor-mode", "page-token")
```

**2. `client_search.go:78-91`** (`init()`, quoted in full — same scope/cross-spine pair, no
paging trio since search has no offset/cursor flags):
```go
func init() {
	addClientFlags(searchCmd)
	searchCmd.Flags().StringVar(&searchQuery, "query", "", "search query (required)")
	searchCmd.Flags().StringVar(&searchScope, "scope", "", "...")
	searchCmd.Flags().BoolVar(&searchCrossSpine, "cross-spine", false, "...")
	searchCmd.Flags().Uint64Var(&searchK, "k", 0, "...")
	searchCmd.Flags().StringSliceVar(&searchTags, "tags", nil, "...")
	searchCmd.Flags().BoolVar(&searchFull, "full", false, "...")
	searchCmd.Flags().StringVar(&searchCreatedAfter, "created-after", "", "...")
	searchCmd.Flags().StringVar(&searchCreatedBefore, "created-before", "", "...")
	searchCmd.Flags().StringSliceVar(&searchCategories, "categories", nil, "...")
	rootCmd.AddCommand(searchCmd)
}
```
D-07 add: `searchCmd.MarkFlagsMutuallyExclusive("scope", "cross-spine")`.

**3. `migrate.go:141-150`** (`init()` for `migrateRemapOwnerCmd`, quoted in full):
```go
migrateRemapOwnerCmd.Flags().StringVar(&remapFrom, "from", "", "...")
migrateRemapOwnerCmd.Flags().BoolVar(&remapMissing, "from-missing", false, "...")
migrateRemapOwnerCmd.Flags().BoolVar(&remapAnon, "from-anon", false, "...")
migrateRemapOwnerCmd.Flags().StringVar(&remapTo, "to", "", "...")
migrateRemapOwnerCmd.Flags().BoolVar(&remapDryRun, "dry-run", false, "...")
migrateRemapOwnerCmd.Flags().DurationVar(&remapTimeout, "timeout", 5*time.Minute, "max wall-clock (0 disables); also cancellable via Ctrl-C")
rootCmd.AddCommand(migrateRemapOwnerCmd)
```
D-07 add (before `rootCmd.AddCommand`):
```go
migrateRemapOwnerCmd.MarkFlagsMutuallyExclusive("from", "from-missing", "from-anon")
migrateRemapOwnerCmd.MarkFlagsOneRequired("from", "from-missing", "from-anon")
```

**`buildRemapSource`'s CURRENT counting logic to strip** (`migrate.go`, quoted in full — the
`selected != 1` block D-07 removes; `store.ValidateOwnerRemap` call stays):
```go
func buildRemapSource(from string, missing, anon bool, to string) (store.OwnerRemapSource, error) {
	selected := 0
	if missing {
		selected++
	}
	if anon {
		selected++
	}
	if from != "" {
		selected++
	}
	if selected != 1 {
		return nil, fmt.Errorf("exactly one source required: --from <value> | --from-missing | --from-anon")
	}
	var src store.OwnerRemapSource
	switch {
	case missing:
		src = store.RemapMissing()
	case anon:
		src = store.RemapAnon()
	default:
		src = store.RemapFrom(from)
	}
	if err := store.ValidateOwnerRemap(src, to); err != nil {
		return nil, err
	}
	return src, nil
}
```
After D-07, the `selected := 0 ... if selected != 1 { return ... }` block (8 lines) is deleted;
everything else — the `switch`/`store.ValidateOwnerRemap` call — is unchanged, preserving the
"pure, unit-testable validator before I/O" property RESEARCH.md's Established Patterns section
names as load-bearing.

**`rootCmd.PersistentPreRunE` — the single interception point** (`root.go:45-47`, quoted in full,
current state):
```go
PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
	return config.CheckLegacy(os.Environ())
},
```
D-02/D-07 recommended shape (RESEARCH.md Pattern 1, already fully worked out there — reproduced
here as the concrete edit target):
```go
PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
	if err := config.CheckLegacy(os.Environ()); err != nil {
		return usageErrorf("%s", err)
	}
	if err := cmd.ValidateFlagGroups(); err != nil {
		return usageErrorf("%s", err)
	}
	return nil
},
```

---

### D-03 operator command error paths — Connect-reachable vs raw store/Qdrant errors

None of the six operator commands dial through `internal/server`'s Connect handlers — they call
`server.StoreFromEnv()` / `server.StoreAndEmbedderFromEnvNoEnsure()` /
`server.StoreAndSummarizerFromEnv()` directly and then a `store.*` method, so `exitCodeForConnectErr`
does **not** reach any of these paths (RESEARCH.md's Pattern 3/Architectural Responsibility Map
already establish this; the classification below is the concrete per-command site inventory).

**Shape to mirror:** `internal/server/connecterror.go`'s `connectError` (RESEARCH.md quotes this
in full already — not re-quoted here) is the sentinel-switch shape; D-03's CLI-side equivalent
targets `exitUsage`/`exitNotFound`/`exitUnavailable` instead of Connect codes.

| Command | Site | Current return | Classification |
|---|---|---|---|
| `reindex.go:43` | `reindexTarget == ""` guard | `fmt.Errorf("--target ... is required")` | `exitUsage` (2) — bad flag value |
| `reindex.go:52` | `server.StoreAndEmbedderFromEnvNoEnsure()` | raw `err` | `exitUsage` (2) if config-load/validate; classify by unwrapping `Config.Validate()`'s aggregated error vs a Qdrant dial failure |
| `reindex.go:80` | `st.Reindex(ctx, ...)` | raw `err` | store/Qdrant error — needs explicit classification (not Connect-reachable); `store.ErrNotFound`-style sentinel → `exitNotFound` (4), a Qdrant gRPC `Unavailable` → `exitUnavailable` (5) |
| `prune.go:32` | `server.StoreFromEnv()` | raw `err` | `exitUsage` (2), same config-load reasoning |
| `prune.go:44` | `st.PruneExpired(ctx, before)` | raw `err` | store/Qdrant — `exitUnavailable` (5) unless a specific sentinel applies |
| `summarize.go:38` | `--scope`/`--all-scopes` guard | `fmt.Errorf(...)` | `exitUsage` (2) |
| `summarize.go:42` | `server.StoreAndSummarizerFromEnv()` | raw `err` | `exitUsage` (2), config-load |
| `summarize.go:65` | the sweep call | raw `err` | store/Qdrant/OpenAI-chat backend — `exitUnavailable` (5) for backend unreachable |
| `backfill.go:29` | `server.StoreFromEnv()` | raw `err` | `exitUsage` (2) |
| `backfill.go:43` | backfill call | raw `err` | store/Qdrant — classify per sentinel |
| `serve.go:72,79,85,176,180,184,190,308,330` | config/telemetry/cookie/csrf/OIDC-discovery/owner-claim/headless guards | `fmt.Errorf(...)` | `exitUsage` (2) — all are pre-flight config validation, per Open Question 2's recommendation |
| `serve.go:126,135,139,149,155,159,172,216,231,275` | store/listener construction | raw `err` | mixed — classify individually; `ListenAndServe()`'s own OS-level failure (address in use) is explicitly recommended to stay on the `exitGeneric`(1) backstop per RESEARCH.md Open Question 2, NOT force-mapped |
| `migrate.go` (`migrateRemapOwnerCmd`) | `buildRemapSource` (after D-07 strips selected-count) | `store.ValidateOwnerRemap`'s bare errors | `exitUsage` (2) — quoted in full in RESEARCH.md Code Examples, not re-quoted here |
| `migrate.go` | `server.StoreFromEnv()` / `st.RemapOwner(ctx, ...)` | raw `err` | same config-load vs store/Qdrant split as above |

**`store` sentinel vocabulary available for the switch** (`internal/store/store.go:66,73,78,84,92,99`
— already fully quoted in RESEARCH.md Pattern 3, reproduced here only as a pointer since the
planner will copy that exact block): `store.ErrNotFound`, `store.ErrInvalidArgument`,
`store.ErrAmbiguousShortID`, `store.ErrShortIDExhausted`, `store.ErrIdempotencyConflict`,
`store.ErrAlreadySuperseded`.

**`buildRemapSource` remains the pure-validator precedent to preserve** — RESEARCH.md's Established
Patterns section names this explicitly ("Pure, unit-testable validators before I/O... so both fail
fast before opening a Qdrant connection"); D-03's per-command classification should NOT collapse
this pattern into the config-load call, i.e. keep flag-shape validation (exit 2, no I/O) textually
separate from the `server.StoreFromEnv()`/backend-call classification (exit 2/4/5, has I/O), the
same separation `reindex.go:43` (pure guard) vs `reindex.go:52` (I/O-bearing) already demonstrates
today even before this phase's classification lands.

## Shared Patterns

### Typed exit-code carrier
**Source:** `cmd/engram/client_common.go:203-218` (`cliError`/`usageErrorf`)
**Apply to:** every classification site above (D-02/D-03/D-06/D-07) — construct `&cliError{code:
exitX, err: ...}` or a sibling `xErrorf` helper; never a second error-carrier type.

### Pure validator before I/O
**Source:** `cmd/engram/migrate.go`'s `buildRemapSource` / `internal/store/store.go`'s
`ValidateOwnerRemap`
**Apply to:** any new client-side or operator-side flag-shape check — validate before dialing
Qdrant/OpenAI, keep the function free of `server.StoreFromEnv()`/I/O calls so it stays
unit-testable without a live backend.

### Config registry declare→bind→validate chain
**Source:** `internal/config/registry.go` + `config.go` + `validate.go`'s `Embed.Timeout`/
`Summarize.Timeout` (fully excerpted above)
**Apply to:** every new `client.*` koanf field (D-04) — registry row → struct field with
doc-comment convention → validation in a SEPARATE function (`client_validate.go`, not
`Config.Validate()`) → a new, dedicated test file's own `validConfig()`-style helper.

### Set-equality test discipline (not loose "codes are as expected")
**Source:** `cmd/engram/catalog_test.go:304-322` (`TestCatalogExitCodesMatchMapper`)
**Apply to:** D-09's before-table — assert exit codes are distinct where CONTEXT.md claims a row
changes and identical where it claims a row does not, per memory `nczgrtfec2`.

## No Analog Found

None — every file this phase touches has an in-repo precedent (this is a refactor/unification
phase across already-established `cmd/engram/` and `internal/config/` conventions, not new
capability requiring a fresh pattern).

## Metadata

**Analog search scope:** `cmd/engram/*.go` and `*_test.go`, `internal/config/*.go` and
`*_test.go`, `internal/store/store.go`, `internal/server/connecterror.go`
**Files scanned:** ~20 (root.go, client_common.go, client_list.go, client_search.go, migrate.go,
reindex.go, prune.go, summarize.go, backfill.go, serve.go, catalog.go, root_test.go,
catalog_test.go, client_search_test.go, config.go, registry.go, validate.go, validate_test.go,
store.go, connecterror.go)
**Pattern extraction date:** 2026-08-03
