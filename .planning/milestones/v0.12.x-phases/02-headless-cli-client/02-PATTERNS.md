# Phase 2: Headless CLI Client - Pattern Map

**Mapped:** 2026-07-31
**Files analyzed:** 8 (4 new command files + 4 paired tests; `root.go` modification for D-15)
**Analogs found:** 8 / 8 (all partial — no prior Connect *client* exists in this repo; flagged below)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `cmd/engram/client_common.go` | utility (client constructor + shared mapper) | request-response | `cmd/engram/reindex.go` (flag/env wiring) + `internal/server/tools.go:142,159` (`StoreFromEnv`/`StoreAndEmbedderFromEnvNoEnsure` — "single constructor, env-first" shape) | role-match, no exact analog (net-new: Connect client construction) |
| `cmd/engram/client_search.go` | command (cobra) | request-response | `cmd/engram/prune.go` (single-RPC-shaped command, simplest existing command) | role-match |
| `cmd/engram/client_list.go` | command (cobra) | request-response | `cmd/engram/prune.go` | role-match |
| `cmd/engram/client_store.go` | command (cobra) | request-response | `cmd/engram/reindex.go` (required-flag validation pattern) | role-match |
| `cmd/engram/client_common_test.go` | test | transform | `cmd/engram/reindex_test.go` (pure-function unit test, no I/O) | role-match |
| `cmd/engram/client_search_test.go` | test | request-response | `cmd/engram/root_test.go` (drives `rootCmd.Execute()` directly, asserts on error/output) | role-match, needs new capture helper (see below) |
| `cmd/engram/client_list_test.go` | test | request-response | `cmd/engram/root_test.go` | role-match, needs new capture helper |
| `cmd/engram/client_store_test.go` | test | request-response | `cmd/engram/prune_test.go` (tests a pure helper extracted from `RunE`) | role-match |
| `cmd/engram/root.go` (modify: bare-invocation self-describe) | command (cobra root) | request-response | itself, `Execute()` at `cmd/engram/root.go:44-49` | exact (same file) |

## Pattern Assignments

### `cmd/engram/client_common.go` (utility, request-response)

**Analog:** composite — flag/env idiom from `cmd/engram/reindex.go`, single-constructor idiom from `internal/server/tools.go`. No file in this repo builds a Connect *client*; every existing use of `gen/go/engram/v1/engramv1connect` is server-side (`NewEngramServiceClient` is only ever called to build a *handler*, not a client — confirmed via `mcp__probe`/grep across `internal/server`). This piece is genuinely net-new; do not force an analogy beyond flag/env conventions.

**SPDX header** (every Go file, verbatim, `cmd/engram/root.go:1-2`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

**Env-first flag pattern** (copy this shape, `cmd/engram/reindex.go:102-104`):
```go
reindexCmd.Flags().StringVar(&reindexTarget, "target",
    os.Getenv("ENGRAM_REINDEX_TARGET"),
    "target collection to create and populate (required)")
```
Apply identically for `--server` (default `os.Getenv("ENGRAM_SERVER_URL")`, D-02). **Do NOT** apply this shape to the token — D-13 mandates no `--token` flag exists at all; token resolution is `ENGRAM_TOKEN` env then `--token-file` path, read directly in `clientFromFlags`, never bound via `Flags().StringVar`.

**Required-value validation pattern** (`cmd/engram/reindex.go:42-44`):
```go
if reindexTarget == "" {
    return fmt.Errorf("--target (new collection name) is required")
}
```
Use this shape for "no `--server`/`ENGRAM_SERVER_URL` given" — but per D-09 this must map to exit code `2` (usage/validation), not the bare `os.Exit(1)` that `Execute()` in `root.go:44-49` currently applies uniformly to every `RunE` error. **This is the one existing behavior that must change** — see Error/Exit-code section below.

**Single env-first constructor pattern** (`internal/server/tools.go:142`, `:159` — read via targeted grep, function bodies not needed beyond signature/shape since this phase does not reuse them, only their *shape*):
```go
func StoreFromEnv() (*store.Store, error)
func StoreAndEmbedderFromEnvNoEnsure() (*store.Store, uint64, *embed.Client, string, error)
```
`clientFromFlags` should follow this same "one function, env+flags in, ready-to-use client out, single error return" shape — but building an `engramv1connect.EngramServiceClient`, not a `*store.Store`. Signature: `clientFromFlags(cmd *cobra.Command) (engramv1connect.EngramServiceClient, error)` (or equivalent taking the resolved `--server`/`--token-file`/`--insecure`/`--output` flag values).

**Connect client construction** (net-new — no repo analog; from `gen/go/engram/v1/engramv1connect/*.go:96`):
```go
func NewEngramServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) EngramServiceClient
```
The `httpClient` arg is where TLS policy (D-14: verify by default, `--insecure` disables + stderr warning) is threaded — via a `*http.Client{Transport: &http.Transport{TLSClientConfig: ...}}`. No existing file in the repo constructs a custom `http.Transport`; this is net-new, build it directly against `net/http` and `crypto/tls` per D-14, not from an analog.

**TTY detection** (net-new — `golang.org/x/term` is already in `go.mod:146` but only as an **indirect** dependency; promote it to direct with `go mod tidy` once `client_common.go` imports `term.IsTerminal`). No existing file uses it — confirmed no `isatty`/`term.IsTerminal`/`os.Stdout.Fd()` usage anywhere in the repo today.

**Shared Connect-error → exit-code mapper** (net-new, D-10): build one function `exitCodeFor(err error) int` using `connect.CodeOf(err)`, switching on `connect.Code{Unauthenticated,PermissionDenied}` → 3, `connect.CodeNotFound` → 4, `connect.Code{Unavailable,DeadlineExceeded,Unknown}` (transport class) → 5, `connect.CodeInvalidArgument` → 2, default → 1. No repo analog exists for Connect-error classification (`internal/server` only ever *returns* Connect errors, never classifies one from the client side) — this table is derived straight from D-09, not copied from code.

---

### `cmd/engram/client_search.go`, `client_list.go` (command, request-response)

**Analog:** `cmd/engram/prune.go:26-49` — closest existing shape: single external call, minimal flags, one summary line out.

**Cobra command declaration + RunE + init/registration pattern** (`cmd/engram/prune.go:26-49`, `:58-64`):
```go
var pruneExpiredCmd = &cobra.Command{
    Use:   "prune-expired",
    Short: "Delete memories whose validity window (not_after) has lapsed",
    RunE: func(cmd *cobra.Command, _ []string) error {
        st, err := server.StoreFromEnv()
        if err != nil {
            return err
        }
        ...
        cmd.Printf("pruned ~%d expired record(s) ...\n", n, before.Format(time.RFC3339))
        return nil
    },
}

func init() {
    pruneExpiredCmd.Flags().DurationVar(&pruneOlderThan, "older-than", 0, "...")
    rootCmd.AddCommand(pruneExpiredCmd)
}
```
Copy exactly: `Use`/`Short`, `RunE` returning `error` (never `Run`, so cobra can propagate the error for exit-code mapping), flags bound in `init()`, `rootCmd.AddCommand(...)` in the same `init()`. Data payload goes via `cmd.Println`/`cmd.Printf` (writes to the command's configured stdout, not raw `fmt.Println` — important because tests redirect `cmd.SetOut`).

**stdout/stderr discipline** (`cmd/engram/reindex.go:65-66, 74-77` — best existing model, explicitly commented):
```go
// Per-batch progress goes to stderr so it never pollutes the single
// parseable summary line on stdout (engram-xddn).
...
Progress: func(r store.ReindexResult) {
    cmd.PrintErrf("reindex progress: scanned %d, upserted %d...\n", ...)
},
```
This is the one place in the repo that already separates data (stdout) from diagnostics (stderr) deliberately — copy this discipline for D-07: `cmd.Println`/`cmd.OutOrStdout()` for the JSON/table payload, `cmd.PrintErrln`/`cmd.ErrOrStderr()` for every warning (including the D-14 `--insecure` warning).

**`--output` flag** (D-06): follow the `reindex.go` `StringVar`-with-default flag-binding shape (`reindex.go:102-104` above), default `""` meaning "TTY-detect" (D-05), validated in `RunE` against `{"json","text"}` (invalid value → exit code 2 per D-09, using the same `fmt.Errorf` validation shape as `reindex.go:42-44`).

**Empty-result handling** (D-12): no repo analog needed — this is a "do nothing special" case; just ensure `RunE` returns `nil` (exit 0) when `SearchMemories`/`ListMemories` returns zero rows, mirroring `prune.go`'s pattern of returning `nil` after printing a summary regardless of count (`prune.go:44-47` prints and returns nil unconditionally, count included).

---

### `cmd/engram/client_store.go` (command, request-response)

**Analog:** `cmd/engram/reindex.go:41-44` — required-flag validation before doing any network/IO work.

Copy the up-front validation pattern (target/content required before constructing the client), and the `os.Interrupt`/`SIGTERM` cancellation idiom from `reindex.go:57-58` / `prune.go:34-35` if `store` needs to be interruptible (likely not needed — a single unary RPC — but if a `--timeout` flag is added for symmetry with `reindex`/`prune`, copy `reindex.go:59-63`):
```go
ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
defer stop()
if timeout > 0 {
    var cancel context.CancelFunc
    ctx, cancel = context.WithTimeout(ctx, timeout)
    defer cancel()
}
```

---

### `cmd/engram/root.go` (modify — D-15 bare-invocation self-describe)

**Analog:** itself. Current `rootCmd` (`root.go:24-33`) and `Execute()` (`root.go:44-49`).

**What exists today and must change:**
```go
// root.go:44-49
func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }
}
```
Every command failure today exits `1`. D-09 requires 0/1/2/3/4/5 depending on error class. **This function is the single choke point that controls the process exit code today** — it is the only `os.Exit` call in `cmd/engram` (confirmed: `reindex.go`, `prune.go` etc. only ever `return err` from `RunE`, never call `os.Exit` themselves). To satisfy D-09 without breaking existing operator commands (`serve`, `reindex`, `prune-expired`, `summarize`, `backfill`, `migrate` — all fine with exit 1 on error), `Execute()` needs a way to ask "does this error carry a specific exit code" — e.g. a small `exitCoder interface { ExitCode() int }` that `client_common.go`'s mapper wraps its errors in, and `Execute()` type-asserts:
```go
func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        code := 1
        if ec, ok := err.(interface{ ExitCode() int }); ok {
            code = ec.ExitCode()
        }
        os.Exit(code)
    }
}
```
This is the minimal change that preserves every existing command's behavior (plain errors still exit 1) while letting the three new client commands opt in to the 0-5 taxonomy. This is a recommendation, not an existing pattern — flag it to the planner as a required `root.go`/`main.go` touch, not something the client files can achieve alone.

**`SilenceUsage`/`SilenceErrors`** (`root.go:24-33`, comment at `root.go:20-23`): already set at the root level, so this already satisfies "cobra must not print usage to stdout on error" (D-07) for every subcommand including the new ones — no per-command change needed, this inherits automatically. Confirmed the mechanism: `SilenceErrors` suppresses cobra's own `Error: ...` print, and `Execute()`'s own `fmt.Fprintln(os.Stderr, ...)` supplies it instead (`root.go:20-23` comment explains this exact tradeoff) — the new client commands get this for free.

**Bare-invocation self-describe (D-15):** no existing analog — `rootCmd` today has no `RunE`/`Run` at all (falls through to cobra's default help-on-no-args). Add a `RunE` to `rootCmd` itself (or a `PersistentPreRunE` check on `len(os.Args) == 1`) that, when invoked with zero args, prints the JSON catalog to stdout and returns before cobra's default help kicks in. `--help` must remain untouched (D-16) — do not hook `HelpFunc`, only handle the true bare-invocation case in `RunE`.

---

## Shared Patterns

### SPDX header
**Source:** every file in `cmd/engram/`, e.g. `cmd/engram/root.go:1-2`
**Apply to:** all 4 new command files + 4 test files
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

### cobra command registration
**Source:** `cmd/engram/prune.go:58-64`, `cmd/engram/reindex.go:101-114`
**Apply to:** `client_search.go`, `client_list.go`, `client_store.go`
```go
func init() {
    xCmd.Flags().StringVar(&xFlag, "flag-name", defaultFromEnv, "usage")
    rootCmd.AddCommand(xCmd)
}
```

### stdout (data) vs stderr (diagnostics) split
**Source:** `cmd/engram/reindex.go:65-66, 74-77`
**Apply to:** all three client commands, plus the `--insecure` warning in `client_common.go`

### Legacy-env rejection gate
**Source:** `cmd/engram/root.go:30-32`, `internal/config.CheckLegacy`
**Note:** this runs in `PersistentPreRunE` on `rootCmd` and therefore fires for the new client commands too. Per D-04 the client does not import `internal/config`'s registry for its *own* settings, but it cannot opt out of this pre-existing `PersistentPreRunE` short of restructuring `rootCmd` — confirm with the planner whether `CheckLegacy` matters for client-only invocations (it inspects `os.Environ()` broadly, not registry-scoped, so it should be harmless/no-op for a client-only environment, but it is inherited whether wanted or not).

## No Analog Found

| File/Concern | Role | Data Flow | Reason |
|---|---|---|---|
| Connect client construction (`NewEngramServiceClient` called as a *client*, not building a handler) | utility | request-response | Repo has only ever been a Connect **server**; every existing `engramv1connect` usage is server-side handler wiring in `internal/server` |
| TLS transport / `--insecure` toggle | utility | request-response | No existing file builds a custom `http.Transport`/`tls.Config`; must be written from D-14 directly against `crypto/tls`/`net/http` |
| TTY detection (`term.IsTerminal`) | utility | transform | `golang.org/x/term` is only an indirect dependency today (`go.mod:146`); no file uses it — will need `go mod tidy` to promote to direct once imported |
| Connect-error → exit-code classification | utility | transform | No repo code classifies a `connect.Code` from the client side; derive directly from D-09/D-10, no code to copy |
| Per-error-class exit code propagation through `Execute()` | control flow | request-response | `root.go`'s `Execute()` (`root.go:44-49`) is hard-coded to `os.Exit(1)`; needs the `ExitCode() int` interface addition described above — this is a required modification, not a pattern to copy |
| Bare-invocation JSON self-describe catalog | command | request-response | `rootCmd` has no `RunE` today; D-15 behavior is entirely new |

## Metadata

**Analog search scope:** `cmd/engram/*.go` (all 22 files enumerated), `internal/server/tools.go` (constructor shape only), `gen/go/engram/v1/engramv1connect/*.go` (client constructor signature), `go.mod` (dependency check for `golang.org/x/term`, `connectrpc.com/connect`)
**Files scanned:** 8 read in full (`root.go`, `main.go`, `reindex.go`, `reindex_test.go`, `prune.go`, `prune_test.go`, `root_test.go`, `02-CONTEXT.md`) + targeted greps across the rest of `cmd/engram` and `internal/server`
**Pattern extraction date:** 2026-07-31
