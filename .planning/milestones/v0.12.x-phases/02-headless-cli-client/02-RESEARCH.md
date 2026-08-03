# Phase 2: Headless CLI Client - Research

**Researched:** 2026-07-31
**Domain:** Go CLI client over a generated ConnectRPC stub (cobra + connect-go), stdlib-only
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 (three subcommands, exactly the three the requirement names):** Ship `engram search`,
  `engram list`, and `engram store`. The Connect service already exposes `SearchMemories`,
  `ListMemories`, and `StoreMemory`, so the mapping is 1:1 with no adapter logic. `ListScopes`,
  `GetMemory`, `SearchDiscoveries` and the remaining write RPCs exist on the service but are
  deliberately NOT surfaced in this phase — adding them is additive later and each would need its
  own output shape decision.

- **D-02 (server URL via `--server` flag with an `ENGRAM_SERVER_URL` env fallback):** The flag wins
  when both are set. A URL is not a secret, so unlike the token it is allowed in `argv`. Absent
  both, the command fails with the usage exit code rather than defaulting to localhost — a silent
  localhost default is how a CI job ends up quietly querying nothing.

- **D-03 (client code lives in new `cmd/engram` client files, registered on the existing `rootCmd`):**
  Files are named `client_search.go`, `client_list.go`, `client_store.go` and a shared
  `client_common.go`. One shared `clientFromFlags` constructor builds the Connect client so the
  server URL, token resolution, TLS policy, and output-format handling exist once rather than three
  times. No new binary — `engram` stays a single binary, consistent with how `serve`, `reindex`,
  `prune-expired` and the other operator commands already hang off `rootCmd`.

- **D-04 (the client does NOT reuse `serve`'s koanf config registry):** `internal/config`'s registry
  is the server's contract — Qdrant endpoints, embedder credentials, OIDC issuers, summarizer
  models. A client that loads it would demand or accept configuration it must never need, and would
  couple the CLI to server-side config drift. The client reads only its own small set: server URL,
  token, output format, TLS policy.

- **D-05 (JSON when stdout is not a TTY, human-readable table when it is):** The default adapts to
  the caller, so an agent piping the command gets structured output with no flag, and a human at a
  terminal gets something readable. This is the behavior `REQ-cli-agent-output` asks for.

- **D-06 (`--output=json|text` forces the format regardless of TTY detection):** TTY detection is a
  heuristic and CI environments lie about it in both directions. A caller that needs a guaranteed
  shape must be able to pin it explicitly, and a test must be able to assert JSON without a pty.

- **D-07 (data to stdout, every diagnostic to stderr, on every path):** Warnings, progress, errors,
  and the TLS-insecure warning all go to stderr. stdout carries only the command's data payload, so
  `engram search ... | jq` is always valid and never contaminated by a warning line.

- **D-08 (one JSON object per invocation, mirroring the Connect response field names):** A single
  `{results: [...], ...}` document, not NDJSON. Mirroring the proto field names means an agent that
  knows the Connect API already knows the CLI's output, and it keeps the CLI from inventing a second
  vocabulary for the same data.

- **D-09 (semantic exit-code taxonomy):** `0` success · `2` usage or validation error · `3`
  authentication or authorization failure · `4` not found · `5` transport or server unavailable ·
  `1` generic/unclassified. Distinct codes let a shell caller branch without parsing stderr, which
  is the entire point of `REQ-cli-agent-output`.

- **D-10 (one shared mapper over the Connect error code, never per-command):** A single function
  reading `connect.CodeOf` translates a Connect error code to an exit code. Per-command mappings
  would drift, and the drift would be invisible until a caller's error handling silently stopped
  matching.

- **D-11 (the exit-code catalog is part of the self-describe output):** An agent discovers the codes
  from the binary itself, not from documentation it may not have. This is what makes
  `REQ-cli-self-describing` and `REQ-cli-agent-output` reinforce each other.

- **D-12 (an empty result set exits 0):** Absence is a legitimate answer, not a failure. Returning
  non-zero for "searched successfully, found nothing" would force every caller to special-case the
  most ordinary outcome.

- **D-13 (token from `ENGRAM_TOKEN`, then `--token-file`; there is NO `--token` flag):** The flag
  simply does not exist, which makes leaking a token into `argv`, `ps` output, or shell history
  structurally impossible rather than merely discouraged. Env var wins over file when both are set.

- **D-14 (TLS verification on by default; `--insecure` always warns loudly on stderr):**
  `REQ-cli-credential-safety` requires that verification cannot be disabled silently — not that it
  cannot be disabled. An operator debugging a self-signed staging cert needs the escape hatch; the
  unconditional stderr warning is what keeps it from becoming an invisible default.

- **D-15 (a bare invocation returns the full catalog as JSON on stdout, exit 0):** An agent
  discovers the command, flag and exit-code surface by running the binary with no arguments and
  parsing structured output, rather than scraping help text whose formatting is not a contract.

- **D-16 (`--help` remains ordinary human cobra output and is not replaced):** The JSON catalog is
  what a *bare* invocation yields. Replacing cobra's help would degrade the human experience to buy
  nothing — the agent path already has D-15.

### Claude's Discretion

No decisions were deferred to discretion — all 16 grey-area questions were answered explicitly.
Implementation details not covered above (exact table column layout for the TTY path, internal
function decomposition, test file organization) are at the executor's discretion, guided by the
existing `cmd/engram` conventions.

### Deferred Ideas (OUT OF SCOPE)

- Surfacing the remaining Connect RPCs as subcommands (`get`, `scopes`, `search-discoveries`, and
  the other write verbs). Additive later; each needs its own output-shape decision, and D-01 keeps
  this phase's surface to exactly what the requirements name.
- A standalone `engramctl` binary. Rejected in favor of keeping `engram` a single binary (D-03).
- Shell completion scripts and a man page. Ordinary cobra capabilities, unrelated to the
  agent-facing goal of this phase.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-cli-client-commands | An agent with only a shell can `engram search`, `engram store`, and `engram list` against a remote server given a server URL and a token. The commands speak only the generated Connect stubs and never import `internal/store`, `internal/authz`, or `internal/embed`. | Pattern 1 (client construction), Pattern 2 (field enumeration per RPC), Q6 (cobra wiring), Validation Architecture (static-import-boundary test) |
| REQ-cli-agent-output | CLI output is consumable by a non-interactive caller: structured JSON by default when not attached to a TTY, data on stdout and diagnostics on stderr, documented semantic exit codes mapped from engram's existing Connect error taxonomy, and no TTY prompt on any path. | Pattern 4 (TTY detection), Pitfall 5 + Code Examples (exit-code mapper), Pitfall 2 (stdout/stderr already clean), Don't Hand-Roll (shared format/mapper helpers) |
| REQ-cli-credential-safety | A token can be supplied without ever appearing in `argv` (env var or file), so it does not leak into `ps` output or shell history. TLS verification is on by default and cannot be disabled silently. | Q6 flag-declaration convention (D-13, no `--token` flag), Pattern 5 (TLS config), Security Domain (token-leakage and TLS-downgrade threat patterns) |
| REQ-cli-self-describing | A bare invocation returns the full command / flag / exit-code catalog as structured output, so an agent can discover the surface without parsing help text. | Pitfall 1 (rootCmd.RunE fix), Code Examples (bare-invocation catalog), Pitfall 5 mapper table (source for the exit-code catalog) |
</phase_requirements>

## Summary

This phase adds three cobra subcommands (`search`, `list`, `store`) to the existing `engram`
binary that speak only the generated `engramv1connect.EngramServiceClient` over HTTP(S). Every
mechanical question — client construction, bearer-header attachment, CSRF applicability, TTY
detection, TLS configuration, cobra wiring, exit-code taxonomy — resolves entirely on stdlib +
already-vendored `connectrpc.com/connect v1.20.0` + `github.com/spf13/cobra v1.10.2`
`[VERIFIED: go.mod:8,20]`. No new Go dependency is required or recommended.

The single most consequential finding is in `duplex_http_call.go:313-330` and `error.go:293-313`
of the vendored connect-go source: the connect-go **client** itself — not this repo's code —
already disambiguates a transport failure from a server-reported error. Any `http.Client.Do`
failure (dial refused, DNS failure, TLS handshake failure, connection reset) that is not already a
`*connect.Error` is wrapped as `connect.NewError(CodeUnavailable, err)`; a context deadline or
cancellation is wrapped as `CodeDeadlineExceeded` / `CodeCanceled`. This means `connect.CodeOf(err)`
alone is sufficient to build the D-09 exit-code mapper — no separate `errors.As` transport check is
needed, closing the exact ambiguity the phase's research questions flagged as unresolved.

The second consequential finding is structural, not mechanical: cobra's own control flow actively
fights three of the sixteen locked decisions unless `rootCmd` is changed. `rootCmd` today has no
`Run`/`RunE`, so `Runnable()` is `false`, and cobra's `execute()` returns the `flag.ErrHelp`
sentinel for a bare invocation — which `ExecuteC()` special-cases to print cobra's own help text to
stdout and exit 0, `[VERIFIED: cobra@v1.10.2/command.go:955-957,1150-1155]` directly contradicting
D-15 (bare invocation must emit the JSON catalog). Separately, `cmd/engram/root.go`'s `Execute()`
`[VERIFIED: cmd/engram/root.go:44-49]` maps every non-nil error to exit 1 unconditionally — the
single place that must change to make D-09's five-way exit-code taxonomy real. Both are addressed
below with concrete, minimal-diff mechanisms.

**Primary recommendation:** Give `rootCmd` a `RunE` that emits the self-describe catalog (fixes
D-15 by making `rootCmd` runnable, which routes around the `flag.ErrHelp`/help-and-exit-0 path);
change `cmd/engram/root.go`'s `Execute()` to consult an exported `ExitCode() int` on the returned
error (via `errors.As`) instead of hard-coding exit 1, defaulting to 1 when absent so every existing
command's behavior is byte-for-byte unchanged; build the Connect bearer credential as a
`connect.WithInterceptors` unary client interceptor that sets `Authorization` on `req.Header()`,
composed independently of TLS (which lives entirely on the `http.Client`'s `Transport`); detect TTY
via `os.Stdout.Stat().Mode()&os.ModeCharDevice != 0`, no library.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Command/flag parsing, env fallback resolution | CLI Client (local process) | — | Pure cobra/flag concern; no network or server involvement |
| Server URL + token resolution (`--server`/`ENGRAM_SERVER_URL`, `ENGRAM_TOKEN`/`--token-file`) | CLI Client (local process) | — | D-04: client owns its own tiny config surface, never `internal/config` |
| TLS policy (verify-by-default, `--insecure` opt-out) | CLI Client (local process) | API/Backend (TLS terminates there) | Client decides whether to trust the server cert; server terminates TLS (or sits behind a proxy that does) |
| Bearer credential attachment | CLI Client (local process) | API/Backend (verifies it) | Client sets the header; `internal/auth`'s composed verifier chain (already shipped, Phase 1) does the actual verification server-side |
| RPC construction/dispatch (`SearchMemories`/`ListMemories`/`StoreMemory`) | CLI Client (local process) | API/Backend (`internal/server` Connect handlers) | Client is a thin caller of the generated stub; all business logic, authz, and CSRF exemption logic lives server-side and is out of scope for this phase |
| CSRF token handling | N/A (out of scope for bearer callers) | API/Backend | `internal/server/connectcsrf.go` exempts `auth.LaneBearer` entirely — see Q3 below; the CLI must NOT implement any CSRF plumbing |
| Output formatting (JSON/table), TTY detection | CLI Client (local process) | — | D-05/D-06/D-08; pure local concern, no server involvement |
| Exit-code mapping from Connect error codes | CLI Client (local process) | API/Backend (originates the `connect.Code`) | The server decides *what* went wrong (via `connect.NewError(code, ...)`); the client's shared mapper (D-10) decides what OS exit code that becomes |
| Self-describe catalog (D-15) | CLI Client (local process) | — | Static data compiled into the binary; no server round-trip |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `connectrpc.com/connect` | v1.20.0 `[VERIFIED: go.mod:8]` | RPC transport, client construction, error codes | Already the project's sole RPC client/server library; `gen/go/engram/v1/engramv1connect` is generated against it |
| `github.com/spf13/cobra` | v1.10.2 `[VERIFIED: go.mod:20]` | Command/flag framework | Already used for every `cmd/engram/*.go` command; D-03 requires new commands to register on the existing `rootCmd` |
| stdlib `net/http`, `crypto/tls`, `os`, `encoding/json`, `text/tabwriter` | go1.26.3 toolchain `[VERIFIED: go.mod:3]` | HTTP client + TLS config, TTY detection, JSON output, human table rendering | Zero-new-dependency constraint (this milestone); every mechanical need below is covered by stdlib — see per-question findings |

### Supporting

None. No supporting library is needed beyond the Core table — this is the direct consequence of the
milestone's zero-new-dependency posture, confirmed mechanically true for every research question
below (Q1, Q4, Q5 in particular).

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os.Stdout.Stat().Mode()&os.ModeCharDevice` (stdlib) | `github.com/mattn/go-isatty` | `go-isatty` is already in the module graph as an **indirect** transitive dependency (pulled in by `testcontainers-go`'s docker client tooling) `[VERIFIED: go.mod:96]`, so it would not add a *new module*, but promoting it to a direct `require` is an avoidable dependency surface for one boolean check the stdlib already answers correctly on the project's target platforms (Linux/macOS/Windows console). Do not use it — see Q4 below for the one caveat this loses. |
| `connect.WithInterceptors` unary interceptor for the bearer header | Custom `http.RoundTripper` wrapping the base `Transport` | Both are stdlib/connect-go-native (no new dependency either way). The interceptor approach is recommended — see Q1 below for the full comparison. |
| Hand-rolled table columns via `fmt.Printf` with fixed widths | `text/tabwriter` (stdlib) | `text/tabwriter` is already in the Go stdlib, handles column alignment for variable-width content (scope names, short_ids) without manual padding math, and needs no new dependency. Use it for the TTY/table output path. |

**Installation:** None required — no new `go get`.

**Version verification:** `connectrpc.com/connect v1.20.0` and `github.com/spf13/cobra v1.10.2` are
already pinned in the live `go.mod` (`[VERIFIED: go.mod:8,20]`, read this session) — no registry
lookup needed since nothing new is being added.

## Package Legitimacy Audit

**Not applicable.** This phase adds zero new Go dependencies — every capability is covered by
stdlib plus the two already-vendored libraries in the Core Stack table above. Per
`REQUIREMENTS.md`'s "New Go dependencies" out-of-scope entry, a new dependency in this milestone
needs its own separate justification; none is warranted here.

**Packages removed due to [SLOP] verdict:** none (none proposed).
**Packages flagged as suspicious [SUS]:** none (none proposed).

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │  engram binary (single process, cmd/engram)  │
                    │                                               │
  argv/env  ──────▶ │  cobra flag/arg parse (search|list|store)    │
                    │        │                                      │
                    │        ▼                                      │
                    │  clientFromFlags (client_common.go)          │
                    │   ├─ resolve --server / ENGRAM_SERVER_URL     │
                    │   │    (missing both → exit 2, no call made)  │
                    │   ├─ resolve ENGRAM_TOKEN then --token-file   │
                    │   ├─ build *http.Client                       │
                    │   │    Transport.TLSClientConfig:             │
                    │   │    InsecureSkipVerify = --insecure        │
                    │   │    (default false; warn on stderr if true)│
                    │   └─ engramv1connect.NewEngramServiceClient(  │
                    │        httpClient, baseURL,                   │
                    │        connect.WithInterceptors(bearerAuth))  │
                    │        │                                      │
                    │        ▼                                      │
                    │  RPC call (SearchMemories/ListMemories/       │
                    │             StoreMemory)                       │
                    │        │                          │            │
                    │  success│                    error│            │
                    │        ▼                          ▼            │
                    │  render (D-05/D-06/D-08)   exit-code mapper    │
                    │   TTY? table : JSON          (D-09/D-10)       │
                    │        │                          │            │
                    └────────┼──────────────────────────┼────────────┘
                             ▼                           ▼
                    stdout (data only)          stderr (diagnostics)
                                                 + os.Exit(mapped code)
                                                          │
                                                          ▼
   ══════════════════════ network boundary (HTTP/HTTPS) ═════════════
                                                          │
                    ┌─────────────────────────────────────────────┐
                    │  engram server — Connect lane                │
                    │  (internal/server; Phase 1, already shipped) │
                    │   composed bearer verifier → LaneBearer stamp │
                    │   → CSRF interceptor exempts LaneBearer       │
                    │     entirely on write RPCs (StoreMemory)      │
                    │   → business logic (internal/store, etc.)     │
                    │        NEVER imported by the CLI client       │
                    └─────────────────────────────────────────────┘
```

### Recommended Project Structure

```
cmd/engram/
├── root.go              # EXISTING — gains rootCmd.RunE (D-15) + Execute() exit-code dispatch (D-09)
├── client_common.go      # NEW — clientFromFlags, token/server-URL resolution, TLS build,
│                          #        bearer interceptor, TTY detection, JSON/table render helpers,
│                          #        the shared connect.Code → exit-code mapper (D-10), the
│                          #        self-describe catalog data + its renderer (D-11/D-15)
├── client_common_test.go
├── client_search.go      # NEW — `engram search`, registers on rootCmd in init()
├── client_search_test.go
├── client_list.go        # NEW — `engram list`
├── client_list_test.go
├── client_store.go       # NEW — `engram store`
└── client_store_test.go
```

### Pattern 1: Connect client construction with a static bearer credential (Q1)

**What:** A `connect.UnaryInterceptorFunc` that sets the `Authorization` header on every outgoing
request, passed to `engramv1connect.NewEngramServiceClient` via `connect.WithInterceptors`.

**When to use:** Every one of the three subcommands, built once inside `clientFromFlags`.

**Why the interceptor over a `RoundTripper`:** Both are equally valid connect-go/stdlib patterns —
neither needs a new dependency. The interceptor is recommended because:
1. It operates purely at the Connect layer (`connect.AnyRequest.Header()`), so it composes with
   *any* `http.Client` passed in — including one whose `Transport` is later changed for TLS,
   proxying, or timeouts — without the interceptor needing to know about or wrap that transport.
   TLS configuration (Q5) lives entirely on `http.Client.Transport.TLSClientConfig`; the two
   concerns never need to touch the same object.
2. It mirrors this repo's own existing pattern: `internal/server/connectcsrf.go` and
   `internal/server/connectbearer.go` already build server-side Connect interceptors as
   `connect.UnaryInterceptorFunc` values `[VERIFIED: internal/server/connectcsrf.go:59-114]` — a
   client-side interceptor for the same purpose is the same idiom in the same codebase, not a new
   one.
3. `AnyRequest.Header()` returns a mutable `http.Header` `[VERIFIED: connect@v1.20.0/connect.go:241-250]`
   ("`Header() http.Header`"), so `req.Header().Set("Authorization", "Bearer "+token)` is sufficient
   — no request-body or transport-level plumbing required.

**Example (exact call shape):**
```go
// Source: connectrpc.com/connect@v1.20.0 connect.go:241-250 (AnyRequest.Header()),
// option.go:350-352 (WithInterceptors), interceptor.go:60-63 (UnaryInterceptorFunc);
// gen/go/engram/v1/engramv1connect/engram.connect.go:96 (NewEngramServiceClient signature)
func bearerInterceptor(token string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			return next(ctx, req)
		}
	}
}

httpClient := &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}, // insecure default false (D-14)
	},
}
client := engramv1connect.NewEngramServiceClient(
	httpClient, serverURL,
	connect.WithInterceptors(bearerInterceptor(token)),
)
```

**Wire format confirmation:** the server's bearer extractor requires exactly the scheme
`Bearer <token>` — case-insensitive scheme, exactly two whitespace-separated fields, non-empty
credential `[VERIFIED: internal/auth/bearer.go:75-84]` (`strings.Fields(authHeader)`, `len(fields)
!= 2`, `strings.EqualFold(fields[0], "bearer")`). `"Bearer "+token` with a single space satisfies
this exactly.

### Pattern 2: Reading the three Connect message shapes (Q2)

**What:** Field enumeration for the flags each subcommand must expose and the response shape it
must render, read directly from `proto/engram/v1/engram.proto` `[VERIFIED: proto/engram/v1/engram.proto:76-124]`.

**`SearchMemoriesRequest`** (lines 76-86) → `engram search` flags:
```
message SearchMemoriesRequest {
  string query = 1;
  string scope = 2;
  uint64 k = 3;
  repeated string tags = 4; // empty = all; non-empty = records carrying ALL listed tags (AND)
  bool full = 5;            // false (default) returns summary-shaped memories (content cleared); true returns full content
  string created_after = 6;  // RFC3339; inclusive lower bound on created_at
  string created_before = 7; // RFC3339; exclusive upper bound on created_at
  repeated string categories = 8; // empty = all categories; non-empty = records in ANY listed category (OR)
}
message SearchMemoriesResponse { repeated Memory memories = 1; }
```
`query` and `scope` are the only fields with no documented "empty means all" default in the proto
comments — everything else is optional with an explicit default noted inline. Response is a single
`repeated Memory` field named `memories`, matching D-08 ("mirror the Connect response field names").

**`ListMemoriesRequest`** (lines 55-74) → `engram list` flags: `scope`, `limit`, `offset`,
`categories` (repeated), `visibility` (`""`/`"private"`/`"shared"`), `tags` (repeated, AND
semantics), `full`, `created_after`, `created_before`, `page_token` (opaque cursor), `cursor_mode`
(bool — mutually exclusive with `offset>0` per the proto comment on line 66). Response:
`memories`, `total`, `approximate` (**deprecated**, always false per line 72 comment — do not
surface as a flag or a rendered column), `next_page_token`.

**`StoreMemoryRequest`** (lines 109-124) → `engram store` flags:
```
message StoreMemoryRequest {
  string content = 1 [(buf.validate.field).string.min_len = 1];
  string scope = 2 [(buf.validate.field).string.min_len = 1];
  string source = 3;
  string category = 4 [(buf.validate.field).string = {in: ["decision", "preference", "convention", "gotcha"]}];
  repeated string tags = 5;
  string repo = 6;
  string workspace = 7;
  string worktree = 8;
  string base_dir = 9;
  string summary = 10;
}
message StoreMemoryResponse {
  string id = 1;
  string short_id = 2;
}
```
`content` and `scope` carry a `buf.validate` `min_len = 1` constraint — a server-side rejection of
an empty value returns `CodeInvalidArgument` (D-09 exit 2), so the CLI need not duplicate that
validation client-side beyond cobra's own required-flag mechanism (belt-and-suspenders is fine but
not load-bearing). `category` is constrained to the four-value enum `["decision", "preference",
"convention", "gotcha"]` **on the wire message itself** `[VERIFIED: proto/engram/v1/engram.proto:113]`
— the CLI flag help text should surface this exact set so a caller does not have to guess and get a
CodeInvalidArgument round-trip to find out.

**Server-set fields — confirm NOT client-settable:** `Memory.actor` (field 11) and `Memory.owner`
(field 12) `[VERIFIED: proto/engram/v1/engram.proto:24-25]` appear ONLY in the `Memory` message
(the *response*/read shape) — neither field exists anywhere in `StoreMemoryRequest`
`[VERIFIED: proto/engram/v1/engram.proto:109-120, confirmed by absence]`. There is structurally no
flag the CLI could expose for `actor` or `owner` on `store` even if it wanted to; the proto itself
enforces the "server-set, never client-supplied" contract named in CLAUDE.md's memory contract.
`Memory.score` (field 17), `access_count` (field 19), `last_accessed_at` (field 20), `short_id`
(field 18), and `created_at` (field 14) are likewise response-only and must be rendered, never
accepted as store/search/list input flags.

### Pattern 3: CSRF is a no-op for this client (Q3)

**Confirmed, not assumed:** `internal/server/connectcsrf.go`'s interceptor gates ONLY the six
write-RPC procedures (line 33-40's `csrfWriteProcedures` map, which includes
`EngramServiceStoreMemoryProcedure`), and for a request whose stamped lane is `auth.LaneBearer`,
the switch at lines 79-86 takes the `case auth.LaneBearer: return next(ctx, req)` branch — full
exemption, no cookie, no `X-CSRF-Token` header, no double-submit check of any kind
`[VERIFIED: internal/server/connectcsrf.go:79-86]` (quoted: `"case auth.LaneBearer: return
next(ctx, req)"`). The code comment above it is explicit: *"A bearer caller carries no ambient
cookie by design, so CSRF does not apply to it; the exemption is total for write procedures."*

**Planner directive:** `engram store` (and any future write subcommand) sends **only** the
`Authorization: Bearer <token>` header. No CSRF cookie jar, no `X-CSRF-Token` header, no
`net/http/cookiejar` usage. A plan task that adds any CSRF plumbing to the CLI client is adding
dead code against a server path that will never execute it for a bearer caller.

### Pattern 4: TTY detection with stdlib only (Q4)

**What:** `os.Stdout.Stat()` returns an `os.FileInfo`; `Mode()&os.ModeCharDevice != 0` is true when
stdout is an interactive terminal and false when it is a pipe, redirect, or regular file
`[VERIFIED: standard library `os` package — os.Stdout is `NewFile(uintptr(syscall.Stdout), ...)`,
confirmed present in this Go toolchain, go1.26.3 per go.mod:3]`.

```go
// Source: Go stdlib os package (no new dependency)
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false // fail closed toward JSON — the safer default for an unknown environment (D-05 intent)
	}
	return info.Mode()&os.ModeCharDevice != 0
}
```

**Known caveats (flagged for the planner, not independently verified this session —
`[ASSUMED]`, training knowledge):**
1. Inside `go test`, stdout is captured/piped by the test harness, so `isTerminal(os.Stdout)`
   reports `false` during automated tests regardless of the developer's actual terminal — this is
   *desirable* for this phase (tests get deterministic JSON without needing `--output=json`), and
   matches D-06's stated purpose ("a test must be able to assert JSON without a pty").
2. On Windows, `os.ModeCharDevice` reflects the real Win32 console correctly for a native console
   host, but MSYS2/Cygwin/Git-Bash pseudo-terminals implement their TTY as an underlying pipe, so a
   human at one of those terminals could see `isTerminal` report `false` and get JSON instead of a
   table. This is exactly the situation the `go-isatty` library special-cases; the project's target
   platform is Linux/macOS servers and a macOS development machine per this repo's CLAUDE.md, so
   this is a low-priority, explicitly-accepted gap rather than a blocker — `--output=text` (D-06)
   is the escape hatch.
3. `os.Stdin`/`os.Stdout` are process-global; this check is only meaningful against the process's
   actual fd 1, not against any `io.Writer` the code might otherwise use — cobra's
   `cmd.OutOrStdout()` defaults to `os.Stdout` but tests commonly redirect it via `cmd.SetOut(buf)`.
   **The planner must decide whether the TTY check inspects `os.Stdout` directly (correct for real
   invocations) or is made an injectable dependency (needed for a unit test to simulate "TTY" without
   an actual pty)** — recommend the latter: thread a `isTTY func() bool` (or the `*os.File` to
   stat) through `clientFromFlags` so tests can force both branches deterministically, matching this
   repo's existing pattern of keeping I/O-affecting logic behind a small seam (`reindexSummary`,
   `pruneCutoff` are pure functions tested without I/O `[VERIFIED: cmd/engram/prune_test.go:15-26]`,
   `cmd/engram/reindex.go:87-99]`).

### Pattern 5: TLS configuration (Q5)

**What:** `InsecureSkipVerify` goes on the `http.Client`'s `Transport.TLSClientConfig` — nowhere
else. There is no existing TLS-configuration code anywhere in this repo to date
`[VERIFIED: grep for "InsecureSkipVerify"/"tls.Config" across the repo returned zero matches — this
is genuinely new code, not an extension of an existing pattern]`.

```go
// Source: Go stdlib crypto/tls + net/http (no new dependency)
transport := &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}, // insecure defaults false
}
if insecure {
	fmt.Fprintln(os.Stderr, "WARNING: TLS certificate verification is disabled (--insecure); "+
		"do not use against an untrusted network")
}
httpClient := &http.Client{Transport: transport}
```

**Confirms D-14 is achievable exactly as specified:** verification is on by default (zero value of
`tls.Config.InsecureSkipVerify` is `false`); the escape hatch exists only behind an explicit
`--insecure` flag that the code must pair with an unconditional stderr warning (D-07: diagnostics
always to stderr, never gated by `--output`). There is nothing else in the client design (per this
research) that could silently disable verification — the only `*http.Client` in the codebase is the
one `clientFromFlags` constructs, and D-10's single shared constructor means there is exactly one
place this could go wrong, not three.

### Pattern 6: Existing cobra conventions in `cmd/engram` (Q6)

**Registration:** every command is a package-level `var xCmd = &cobra.Command{...}`; `init()` binds
its flags and calls `rootCmd.AddCommand(xCmd)` `[VERIFIED: cmd/engram/reindex.go:101-114,
cmd/engram/prune.go:58-64]`. The new client commands follow this exactly.

**Flag declaration:** `xCmd.Flags().StringVar(&pkgVar, "name", defaultExpr, "help")` — the default
expression is frequently `os.Getenv("ENGRAM_...")` directly in the flag registration (see
`reindexTarget`'s default `os.Getenv("ENGRAM_REINDEX_TARGET")` `[VERIFIED: cmd/engram/reindex.go:102-104]`).
This is the established idiom for "flag wins, env is the fallback default" and is exactly D-02's
and D-13's required precedence — no new pattern needed, reuse this one for `--server` and (for the
file path only; **there is no `--token` flag**, D-13) `--token-file`.

**Error return path:** commands return `error` from `RunE`; they do NOT call `os.Exit` themselves
`[VERIFIED: no `os.Exit` call inside any `RunE` body across `cmd/engram/*.go` — confirmed by reading
reindex.go, prune.go in full]`. All exit-code decisions are centralized in `root.go`'s `Execute()`.
**This is the seam the plan must extend for D-09** — see Pitfall 3 below for the exact mechanism.

**`SilenceUsage`/`SilenceErrors`:** set only on `rootCmd` (`true`/`true`)
`[VERIFIED: cmd/engram/root.go:28-29]`; no subcommand overrides them. Cobra's `ExecuteC()` checks
both the found subcommand's AND the root's flag before printing usage/error itself
`[VERIFIED: cobra@v1.10.2/command.go:1130-1133,1159-1167]` — since root is `true`, the *whole
condition* is false regardless of what a subcommand sets, so cobra's own usage-to-stdout path is
already fully suppressed repo-wide. New client commands need **no changes** here to stay D-07
compliant — this is already correct and should not be touched.

**Where the exit code is controlled today:** `cmd/engram/root.go:44-49`, `Execute()`:
```go
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
```
Every non-nil error, from every command, becomes exit 1 today. This is the single change point for
D-09.

### Anti-Patterns to Avoid
- **Reimplementing `connect.CodeOf`'s job with a hand-rolled string-match on `err.Error()`:** the
  Connect wire protocol already encodes a structured code; parsing prose is exactly the "invisible
  defect" failure mode this project's `REQUIREMENTS.md` calls out repeatedly for other phases.
- **A `--token` flag "just for local testing," gated behind a build tag or hidden flag:** D-13 is
  structural, not advisory — no code path may accept a token as a CLI argument, full stop.
- **Calling `os.Exit` from inside a `RunE`:** breaks the existing testability pattern
  (`rootCmd.SetArgs(...)`; `rootCmd.Execute()`; assert on the returned `error`
  `[VERIFIED: cmd/engram/root_test.go:11-22]`) and makes the exit-code mapper untestable without a
  subprocess. Return a typed error; let `Execute()` decide the exit code.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Connect-code → OS exit code mapping | A parallel enum + manual `switch` scattered per command | One function in `client_common.go` reading `connect.CodeOf(err)`, called by all three commands and referenced by `Execute()` | D-10 explicitly requires exactly one mapper; this also makes the D-11 self-describe catalog trivially derivable from the same source of truth |
| TTY-aware output selection | A three-way `if isTTY { ... } else if forceJSON { ... } else { ... }` duplicated per command | One `resolveOutputFormat(outputFlag string, isTTY bool) format` helper called once per command before rendering | Same logic, testable in isolation with table-driven cases for `""` + tty=true/false and `"json"`/`"text"` forcing |
| Human table rendering | Manual `fmt.Printf("%-20s %-10s\n", ...)` column math | `text/tabwriter` (stdlib) | Handles variable-width scope/short_id/summary content correctly without manual width tuning |

**Key insight:** every "don't hand-roll" item above is not about avoiding a *library* (there is
none to add) — it's about avoiding *duplicated logic across three near-identical commands*. The one
shared constructor D-03 mandates (`clientFromFlags`) should be read as encompassing all of the
above three helpers, not just the Connect client construction.

## Common Pitfalls

### Pitfall 1: Bare invocation prints cobra help instead of the D-15 catalog
**What goes wrong:** `engram` with no args exits 0 having printed cobra's default help text to
stdout — not the JSON catalog D-15 requires.
**Why it happens:** `rootCmd` has no `Run`/`RunE`, so `Runnable()` returns `false`
`[VERIFIED: cobra@v1.10.2/command.go:1596-1598]`; `execute()`'s `if !c.Runnable() { return
flag.ErrHelp }` `[VERIFIED: cobra@v1.10.2/command.go:955-957]` fires; `ExecuteC()` special-cases
`errors.Is(err, flag.ErrHelp)` to call `cmd.HelpFunc()(cmd, args)` and return `nil` — exit 0, help
on stdout `[VERIFIED: cobra@v1.10.2/command.go:1150-1155]`.
**How to avoid:** give `rootCmd` a `RunE` that renders the self-describe catalog as JSON to stdout
and returns `nil`. This makes `Runnable()` true, so bare invocation now runs that `RunE` instead of
falling into the `flag.ErrHelp` branch. Order matters within `execute()`: the `--help` flag check
(`if helpVal { return flag.ErrHelp }`, `[VERIFIED: cobra@v1.10.2/command.go:934-936]`) runs
*before* the `Runnable()` check, so `engram --help` still returns `flag.ErrHelp` and gets normal
cobra help — **D-16 is preserved automatically**, no extra code needed for that half.
**Warning signs:** a plan that treats D-15 as "just print JSON when `len(os.Args)==1`" inside
`main()` before cobra even runs — fragile, bypasses cobra's arg/flag machinery entirely, and would
diverge from `--help`'s own argument handling.

### Pitfall 2: Cobra's own usage/error printing already fully suppressed — do not re-add it
**What goes wrong:** a plan "fixing" D-07 by explicitly setting `SilenceUsage`/`SilenceErrors` on
each new client command, assuming it's not already inherited.
**Why it happens:** it's a reasonable but unnecessary assumption; the actual mechanism (both root's
AND the found subcommand's flags are checked, root wins because it's `true`) is not obvious from
reading a single command file in isolation.
**How to avoid:** verified already-correct (Pattern 4 in Q6 above) — no task needed for this; flag
it in the plan as "confirmed no-op, do not add."
**Warning signs:** a redundant `SilenceUsage: true` appearing on `searchCmd`/`listCmd`/`storeCmd`
literals — harmless but a signal the assumption wasn't checked against the actual cobra source.

### Pitfall 3: `Execute()`'s blanket `os.Exit(1)` defeats the entire D-09 taxonomy
**What goes wrong:** every error from every command — including the new client commands' auth
failures, not-found results, and transport errors — exits 1, making D-09's five-way taxonomy dead
on arrival for the new commands (and note: byte-identical for every *existing* command, since none
of them return anything but a plain `error` today).
**Why it happens:** `root.go`'s `Execute()` `[VERIFIED: cmd/engram/root.go:44-49]` has exactly one
exit path.
**How to avoid:** define an exported `ExitCode() int` accessor on a small typed error (e.g. `type
cliError struct { code int; err error }` with `Unwrap`/`Error`/`ExitCode`) in `client_common.go`;
change `Execute()` to:
```go
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		code := 1
		var ec interface{ ExitCode() int }
		if errors.As(err, &ec) {
			code = ec.ExitCode()
		}
		os.Exit(code)
	}
}
```
This is additive and backward-compatible: any error that does NOT implement `ExitCode() int` (every
existing command's errors) falls through to the existing `1`, unchanged.
**Warning signs:** a plan that instead tries to make `RunE` call `os.Exit` directly inside the new
commands — breaks the tested, pure-function-return convention (Anti-Pattern above) and produces
inconsistent behavior between old and new commands' error paths.

### Pitfall 4: cobra's own flag-parse/usage errors won't carry an `ExitCode()` and will exit 1, not 2
**What goes wrong:** an unrecognized flag, a missing required flag, or a bad flag *value* (e.g.
`--limit=abc` for a `uint64` flag) is rejected by cobra's own `ParseFlags`/`ValidateRequiredFlags`
before `RunE` ever runs `[VERIFIED: cobra@v1.10.2/command.go:919-922,1007-1009]` — these errors are
plain `*strconv.NumError`/`fmt.Errorf` values with no `ExitCode()` method, so under Pitfall 3's fix
they fall through to the default `1`, not D-09's `2` (usage/validation).
**Why it happens:** D-09's taxonomy is enforced entirely by this phase's own mapper; it has no
reach into cobra's internal flag-parsing error paths.
**How to avoid:** this is a genuine open question for the plan — flagged below rather than resolved
here, because two defensible fixes exist with different blast radii: (a) wrap `rootCmd.Execute()`'s
return with a check for a flag-parse-shaped error and force exit 2 for anything that isn't already
an `ExitCode()`-carrying error but also isn't `nil` — risks over-broadly reclassifying genuine
internal-error exits from *other* (non-client) commands as "usage" errors; (b) accept that cobra's
native flag-parse errors exit 1 (a form of "generic/unclassified," which D-09 also defines) and
reserve exit 2 specifically for *this client's own* semantic validation (e.g., "both `--server` and
`ENGRAM_SERVER_URL` are unset") — narrower, safer, and defensible since D-09's own wording ("usage
or validation error") doesn't distinguish cobra-level from application-level usage errors. **Option
(b) is the safer default and is what this research recommends**, but the discrepancy should be
called out explicitly to the user during planning, not silently decided.
**Warning signs:** a UAT case asserting `engram search --limit=abc` exits 2 without the plan having
explicitly chosen and implemented option (a).

### Pitfall 5: `connect.CodeOf` DOES distinguish transport failure from server-reported "unknown" — verified, not assumed
**What goes wrong (the concern the phase brief raised):** the worry was that a transport failure
(connection refused, DNS failure) and a genuine server-side `CodeUnknown` would both surface as
`connect.CodeUnknown` from `connect.CodeOf`, making D-09's exit 5 (transport) indistinguishable from
exit 1 (generic).
**What actually happens (verified this session):** it does not conflate them. The connect-go
client's `duplexHTTPCall.makeRequest` wraps ANY `httpClient.Do` failure that is not already a
`*connect.Error` as `connect.NewError(CodeUnavailable, err)`
`[VERIFIED: connect@v1.20.0/duplex_http_call.go:313-330]` (quoted: `"if _, ok := asError(err); !ok
{ err = NewError(CodeUnavailable, err) }"`), and separately wraps a context deadline/cancellation as
`CodeDeadlineExceeded`/`CodeCanceled` via `wrapIfContextError`
`[VERIFIED: connect@v1.20.0/error.go:293-313]` (quoted: `"if errors.Is(err, context.DeadlineExceeded)
{ return NewError(CodeDeadlineExceeded, err) }"`). So `connect.CodeOf(err)` reliably returns
`CodeUnavailable` (or `CodeDeadlineExceeded`/`CodeCanceled`) for a genuine transport failure, and
only returns `CodeUnknown` when the *server* actually decided and returned that code (or for an
error that isn't a `*connect.Error` at all and isn't a recognized transport/context failure, which
`CodeOf`'s own fallback also maps to `CodeUnknown` `[VERIFIED: connect@v1.20.0/code.go:219-226]`).
**Recommended mapper table (D-10), all cross-checked against how this server actually emits codes
`[VERIFIED: internal/server/connectapi.go, connectcsrf.go, connectvalidate.go — CodeUnauthenticated
for auth failures, CodeInvalidArgument for validation, CodePermissionDenied for CSRF]`:**

| `connect.Code` | Exit (D-09) | Rationale |
|---|---|---|
| `CodeUnauthenticated` | 3 | server-emitted for every bearer/auth failure in this codebase |
| `CodePermissionDenied` | 3 | authorization failure (D-09 groups "authentication or authorization" together) |
| `CodeNotFound` | 4 | not found |
| `CodeInvalidArgument` | 2 | validation |
| `CodeFailedPrecondition`, `CodeOutOfRange` | 2 | shape-of-request problems, same bucket as validation |
| `CodeUnavailable`, `CodeDeadlineExceeded`, `CodeCanceled` | 5 | transport/server-unavailable — verified as the connect-go client's own wrapping for network-layer failures above |
| everything else (`CodeUnknown`, `CodeInternal`, `CodeAborted`, `CodeAlreadyExists`, `CodeResourceExhausted`, `CodeUnimplemented`, `CodeDataLoss`) | 1 | generic/unclassified, matching D-09's own residual bucket |
| not a `*connect.Error` at all (should not occur given the above, but defensive) | 1 | generic/unclassified |

**Warning signs:** a test asserting exit 5 by injecting a bare `errors.New("connection refused")`
directly into the mapper (bypassing the connect-go client's own wrapping) — that test would pass
against a mapper that's wrong for the real code path, because the real path never hands the mapper
a raw non-`connect.Error`. Pin the transport-failure test at the level the phase's success criteria
actually exercise: point `--server` at a closed port / unroutable address and assert the *real*
`SearchMemories` call's returned error maps to exit 5, not a synthetic error value.

## Code Examples

### The shared exit-code mapper (D-10)
```go
// Source: connectrpc.com/connect@v1.20.0 code.go (Code constants, CodeOf),
// verified server error emissions in internal/server/{connectapi,connectcsrf,connectvalidate}.go
func exitCodeForConnectErr(err error) int {
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated, connect.CodePermissionDenied:
		return 3
	case connect.CodeNotFound:
		return 4
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange:
		return 2
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeCanceled:
		return 5
	default:
		return 1
	}
}
```

### Bare-invocation self-describe catalog (D-11/D-15)
```go
// rootCmd gains a RunE; this makes it Runnable() (fixes Pitfall 1) without
// disturbing --help (Pitfall 1 explains why --help is unaffected).
rootCmd.RunE = func(cmd *cobra.Command, _ []string) error {
	return json.NewEncoder(cmd.OutOrStdout()).Encode(catalog) // catalog: static struct{Commands []...; ExitCodes map[string]int; ...}
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| N/A — no prior CLI client existed | This phase introduces the first non-server, non-MCP caller of `EngramServiceClient` | This phase | Establishes the pattern future additive subcommands (deferred: `get`, `scopes`, `search-discoveries`) will follow |

**Deprecated/outdated:** none relevant — `connect-go` v1.20.0 and cobra v1.10.2 are both the
current pinned versions in this repo; nothing here is being migrated off an old API.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | MSYS2/Cygwin/Git-Bash ptys report as non-char-device under `os.ModeCharDevice`, causing `isTerminal` to misdetect a human terminal as non-TTY on those specific Windows shells | Pattern 4 / Q4 caveats | Low — `--output=text` is the documented escape hatch (D-06); affects only a Windows dev-shell edge case, not the Linux/macOS server deployment target this repo is built for |
| A2 | `os.Stdout.Stat()` behaves identically across the target deployment platforms (Linux/macOS) for the pipe-vs-terminal distinction | Pattern 4 | Low — this is standard, long-stable Go stdlib behavior; not verified via an executed test this session, only via documented `os` package semantics |

**If this table is empty:** N/A — two low-risk items above; neither blocks planning or affects the
core mechanism (interceptor construction, TLS, exit-code mapping, CSRF exemption, cobra wiring) all
of which are `[VERIFIED]` against source read this session.

## Open Questions

1. **Cobra's own flag-parse errors and D-09's exit code 2**
   - What we know: cobra's own `ParseFlags`/`ValidateRequiredFlags` errors bypass this phase's
     custom exit-code mapper entirely (Pitfall 4) and will exit 1 under the Pitfall 3 fix unless a
     second interception point is added.
   - What's unclear: whether the phase's success criteria (or a stricter reading of D-09) require
     *every* usage-shaped error, including cobra's own, to exit 2 — or whether "generic/unclassified"
     (exit 1) is an acceptable outcome for cobra-native parse failures specifically.
   - Recommendation: default to the narrower reading (option (b) in Pitfall 4) — reserve exit 2 for
     this client's own semantic validation (missing server URL, missing required flags handled via
     the client's own check rather than cobra's `MarkFlagRequired`, invalid flag *combinations*) and
     accept exit 1 for cobra's native flag-syntax errors. Surface this explicitly to the user at
     discuss-phase or plan-review time rather than deciding it silently in a plan.

2. **`--limit`/`k`/`offset` flag types vs proto `uint64`**
   - What we know: `ListMemoriesRequest.limit`/`offset` and `SearchMemoriesRequest.k` are proto
     `uint64` `[VERIFIED: proto/engram/v1/engram.proto:57-58,79]`. Cobra/pflag has a native
     `Uint64Var` flag type.
   - What's unclear: nothing blocking — this is a straightforward `cmd.Flags().Uint64Var(...)`
     usage — flagged only so the planner doesn't default to `IntVar` and need a lossy conversion or
     a negative-value edge case (`pflag.Uint64Var` already rejects a `-1` input at parse time with
     its own error, which — per Open Question 1 above — will exit 1, not 2, unless addressed).
   - Recommendation: use `Uint64Var` directly; treat its native rejection the same as any other
     cobra-level flag-parse error per Open Question 1's resolution.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test/vet | ✓ | go1.26.5 (module requires go1.26.3) `[VERIFIED: go.mod:3, go version output]` | — |
| `golangci-lint` | `task lint` | ✓ | 2.12.2 | — |
| `task` | `task` (lint+test) | ✓ | 3.50.0 | — |
| A reachable engram server with the Connect lane mounted | end-to-end manual verification of `search`/`list`/`store` against a live server | Not probed this session — requires an explicit `connect.headless=true` mount or UI-enabled deployment per Phase 1 | — | Automated tests should use `httptest.NewServer` wrapping the real `internal/server` Connect mux in-process (matching this repo's existing `httptest` usage in `cmd/engram/csrf_test.go` `[VERIFIED: cmd/engram/csrf_test.go:1-14]`), not a live external server |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** a live server for manual E2E verification — automated tests
substitute an in-process `httptest.Server` around the real Connect handlers.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`, no assertion library (`cmd/engram` tests use plain `testing`, not testify, per `[VERIFIED: cmd/engram/prune_test.go, root_test.go]`) |
| Config file | none — `go test` needs no config |
| Quick run command | `go test ./cmd/engram/... -run TestClient -v` |
| Full suite command | `go test ./... -count=1` (per `Taskfile.yaml`'s `test:strict` target `[VERIFIED: Taskfile.yaml:49-54]`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-cli-client-commands | `search`/`list`/`store` complete against a real (in-process `httptest`) Connect server | integration | `go test ./cmd/engram/... -run TestClientSearch -v` | ❌ Wave 0 |
| REQ-cli-client-commands | No client file imports `internal/store`/`internal/authz`/`internal/embed` | static | `go list -json ./cmd/engram/... \| jq -r '.Deps[]' \| grep -E 'internal/(store\|authz\|embed)$'` — must be empty | ❌ Wave 0 (or a plan-checker step, not a Go test) |
| REQ-cli-agent-output | JSON when not a TTY, table when TTY-forced-off via injected seam | unit | `go test ./cmd/engram/... -run TestResolveOutputFormat -v` | ❌ Wave 0 |
| REQ-cli-agent-output | Data on stdout, diagnostics on stderr, exit codes distinguish auth/not-found/validation/transport | integration (uses `cmd.SetOut`/`cmd.SetErr` per existing convention) | `go test ./cmd/engram/... -run TestClient.*ExitCode -v` | ❌ Wave 0 |
| REQ-cli-agent-output | No prompt on any path | code review / static (no `bufio.Scanner(os.Stdin)` or similar in any `client_*.go`) | `grep -rn "os.Stdin" cmd/engram/client_*.go` — must be empty | ❌ Wave 0 |
| REQ-cli-credential-safety | Token never in `argv` (no `--token` flag exists) | static / code review | `go run . --help 2>&1 \| grep -i '\-\-token '` — must show only `--token-file`, never bare `--token` | ❌ Wave 0 |
| REQ-cli-credential-safety | `--insecure` always warns on stderr | unit | `go test ./cmd/engram/... -run TestInsecureWarns -v` | ❌ Wave 0 |
| REQ-cli-self-describing | Bare invocation returns full catalog as JSON, exit 0 | integration | `go test ./cmd/engram/... -run TestRootBareInvocation -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./cmd/engram/... -v` (fast — no real network, `httptest` in-process)
- **Per wave merge:** `go test ./... -count=1` (Taskfile's `test:strict`)
- **Phase gate:** Full suite green before `/gsd-verify-work`; also run `go vet ./...` (the compile
  gate per this repo's own convention — `go build` does not compile `_test.go` files and would miss
  a test-only compile error)

### Wave 0 Gaps
- [ ] `cmd/engram/client_common_test.go` — covers the exit-code mapper (D-10), TTY-resolution
      helper, and bearer interceptor construction, all as pure/injectable-seam unit tests
- [ ] `cmd/engram/client_search_test.go`, `client_list_test.go`, `client_store_test.go` — one
      `httptest.NewServer`-backed integration test per command covering success, auth failure
      (exit 3), not-found where applicable (exit 4), validation failure (exit 2), and — for at
      least one command — a real transport failure against a closed port (exit 5, per Pitfall 5's
      warning about not synthesizing this)
- [ ] `cmd/engram/root_test.go` gains a case for bare-invocation catalog output (D-15) and for
      `--help` remaining unchanged (D-16), extending the existing file rather than a new one
- [ ] No new test framework install needed — stdlib `testing` + `net/http/httptest`, both already
      used in this package

## Security Domain

`security_enforcement` is not explicitly `false` in `.planning/config.json` `[VERIFIED:
.planning/config.json — only `workflow` and `git` keys present, no `security_enforcement` key,
treated as enabled per the default rule]`, so this section is required.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Bearer token attached via `Authorization` header only; verification is entirely server-side (already shipped, Phase 1's composed `auth.ChainVerifier`) — the client's only job is correct header construction, never token validation |
| V3 Session Management | no | The CLI is stateless per-invocation; no session, no cookie, no session storage — bearer tokens are supplied fresh from `ENGRAM_TOKEN`/`--token-file` on every invocation |
| V4 Access Control | no (server-side, out of scope) | Authorization decisions are made entirely server-side; the client never evaluates or assumes any access-control outcome |
| V5 Input Validation | yes | Server-side `buf.validate` constraints (`min_len`, `in: [...]` enum) are the authoritative validation; the client's own flag-level checks (e.g. required `--content`/`--scope` on `store`) are a UX convenience, not a security boundary — the server's `CodeInvalidArgument` rejection is authoritative regardless of what the client attempts to pre-check |
| V6 Cryptography | yes | TLS verification (Q5) — `crypto/tls` stdlib only, never a hand-rolled certificate check; `InsecureSkipVerify` is the sole, explicit, always-warned opt-out (D-14) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Token leakage via `argv`/`ps`/shell history | Information Disclosure | D-13: no `--token` flag exists at all — structural, not advisory (`[VERIFIED: 02-CONTEXT.md:101-103]`) |
| Silent TLS downgrade / MITM | Tampering, Information Disclosure | D-14: verification on by default, `--insecure` requires an explicit flag and always emits an unconditional stderr warning; `InsecureSkipVerify`'s zero value is `false`, so the "do nothing" path is the secure path |
| A future contributor "helpfully" adding `--token` back, or logging the resolved token for debugging | Information Disclosure | Code-review-level control, not automatable — flag explicitly in the plan's review checklist: no `client_*.go` file may log, print, or otherwise surface the resolved token value on any path, including error messages (a token embedded verbatim in a wrapped connection error string would leak it to stderr) |
| Treating a `CodeUnknown` server response as "safe to retry blindly" | Repudiation / integrity (write path) | Out of scope for this phase by design — `REQUIREMENTS.md`'s "Out of Scope" table explicitly forbids "a CLI that retries mutating calls on ambiguous failure without an idempotency key"; `engram store` must not auto-retry on any error class, transport or otherwise |

## Sources

### Primary (HIGH confidence)
- `/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-headless-cli-client/02-CONTEXT.md` — 16 locked decisions, read in full this session
- `/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto` — read in full this session
- `/Volumes/Code/github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect/engram.connect.go` (lines 1-120) — client constructor shape
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go`, `reindex.go`, `prune.go`, `root_test.go`, `prune_test.go`, `csrf_test.go` — read in full this session
- `/Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf.go`, `internal/auth/bearer.go` — read in full this session
- `connectrpc.com/connect@v1.20.0` vendored source (`code.go`, `error.go`, `option.go`, `connect.go`, `interceptor.go`, `client.go`, `duplex_http_call.go`) — read directly from the module cache this session, pinned to the exact version in `go.mod`
- `github.com/spf13/cobra@v1.10.2` vendored source (`command.go`) — read directly from the module cache this session, pinned to the exact version in `go.mod`

### Secondary (MEDIUM confidence)
- None used — every claim that drives an implementation choice was resolved against primary source read this session.

### Tertiary (LOW confidence)
- Windows/MSYS2 TTY-detection caveat (A1 in Assumptions Log) — training knowledge, not independently verified this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, both existing libraries' exact vendored versions read directly
- Architecture: HIGH — connect-go client construction, TLS, and exit-code mapping all verified against vendored source line ranges; cobra control-flow pitfalls verified against vendored `command.go`
- Pitfalls: HIGH for Pitfalls 1, 2, 3, 5 (all verified against source); MEDIUM for Pitfall 4 (the underlying mechanism is verified, but the correct *policy* choice is a genuine open question left to the user/planner)

**Research date:** 2026-07-31
**Valid until:** 30 days (stable, pinned dependency versions; re-verify if `connectrpc.com/connect` or `spf13/cobra` are bumped by Renovate before this phase plans)
