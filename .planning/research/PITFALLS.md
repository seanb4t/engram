# Pitfalls Research

**Domain:** Adding a second (bearer) auth credential, a headless-mountable API surface, a
first-party CLI client, cross-scope search, and authz/error diagnostics to an existing,
shipped, multi-tenant, authorization-enforcing Go server (engram) whose authz chokepoint is
the storage layer and whose Connect transport already has a working CSRF defense for a
cookie-authenticated browser lane.
**Researched:** 2026-07-29
**Confidence:** HIGH (grounded directly in the current `internal/server`, `internal/auth`,
`internal/webauth`, and `internal/store` source — not generic advice)

## Critical Pitfalls

### Pitfall 1: CSRF exemption inferred from a request-controlled signal instead of resolver provenance

**What goes wrong:**
`newConnectCSRFInterceptor` (`internal/server/connectcsrf.go`) today has exactly one identity
source: `subjectFromConnectContext`, populated by a resolver that is *always* the cookie lane
(`webauth.Resolver.Resolve`). Every write request that reaches the interceptor is, by
construction, cookie-authenticated, so `verify(subj.Owner(), cookieValue)` is a safe check.
The moment a second resolver path exists (bearer), the interceptor's implicit assumption —
"if I have a Subject, it came from a cookie" — becomes false, but nothing in the code
currently *says* that assumption out loud. The natural, wrong fix is to special-case the
missing pieces of the cookie flow: skip CSRF "if there's no `engram_csrf` cookie" or "if
there's no `X-CSRF-Token` header" or "if the caller sent `Authorization: Bearer ...`". All
three of those are **request-controlled**: a cookie-authenticated attacker (the exact actor
CSRF exists to stop — a browser with a live session making a cross-origin POST) can simply
omit the CSRF header, or omit the cookie, or add a garbage `Authorization` header, and now
qualifies for the exemption meant only for a genuinely bearer-authenticated caller. This is a
full CSRF bypass on the six write RPCs, exploitable by any page the victim's browser loads
while the session cookie is live — silently, because the request still authenticates fine
(cookie → Subject) and the exemption fires before the token/cookie mismatch would ever be
checked.

**Why it happens:**
The interceptor's existing code has no explicit "how was this Subject resolved" field to
branch on — only the *result* (a `store.Subject`), never the *mechanism*. When bearer support
lands, whoever wires the exemption reaches for the nearest available signal, which is always
something the caller sent on the wire (a header's presence/absence, an Authorization prefix),
because that's what's visible at the point they're editing. This is exactly the "keyed on
request-controlled input" trap named in PROJECT.md's milestone #1 risk, and it is structurally
the same class of bug as v0.11.x's service-principal `owner==""` risk: a security invariant
that lives at a seam between two independently-evolving lanes, where the "obvious" signal to
key on is attacker-controlled.

**How to avoid:**
Provenance must be decided **inside the resolver**, at the moment the credential is actually
verified, and carried forward as an explicit, non-inferable value — not reconstructed
downstream from what the request happens to contain. Concretely: extend the value stashed
under `connectSubjectKey` (or add a sibling context key) with an explicit lane tag (e.g. an
enum `laneCookie`/`laneBearer`) set by the *resolver that succeeded*, never derived from
header presence at the CSRF interceptor. `newConnectCSRFInterceptor` then branches on that
tag, not on `req.Header().Get(...)` or `dummy.Cookie(...)` returning an error. Write the
provenance-carrying field first, as its own small, reviewable change, before wiring any bearer
resolver to it — this mirrors the v0.11.x precedent of proving the risk closed as the phase's
**first** test, before any other write-lane code depends on the new field.

**Warning signs:**
- The CSRF exemption condition (in code review) reads as `if header == "" { skip }`,
  `if err != nil { skip }` (cookie lookup failing), or checks `req.Header().Get("Authorization")`
  anywhere inside `newConnectCSRFInterceptor` — any of these is the bug, even if the
  accompanying comment sounds reasonable.
- The bearer resolver and the cookie resolver both populate the *same* `TokenInfo.Extra` shape
  with no additional field distinguishing them — downstream code has no way to tell them apart
  even if it wanted to.
- Grep for `csrf` finds no reference to a lane/provenance type at all — only Subject/Owner.

**Phase to address:**
Headless client lane (Connect bearer identity) — this MUST be the first slice of that phase,
proven with a negative test before the bearer resolver is wired to anything else. Suggested
test: `TestCSRFNotExemptedByMissingHeaderOrCookie` — construct a request resolved via the
**cookie** lane (provenance = cookie) that omits `X-CSRF-Token`, omits the `engram_csrf`
cookie, and/or carries a bogus `Authorization: Bearer garbage` header; assert
`CodePermissionDenied` in every case. Pair it with
`TestCSRFExemptedOnlyByBearerProvenance` — a request resolved via the **bearer** lane with no
cookie/CSRF header present at all must pass, but a request that fails bearer verification and
falls through to no-identity must be `CodeUnauthenticated`, never silently retried against the
cookie lane.

---

### Pitfall 2: The combined resolver silently reclassifies a failed bearer attempt as "try cookie instead"

**What goes wrong:**
Mounting Connect headless-and-with-UI at once means the new resolver must decide, per request,
which lane to try. The natural implementation is "if `Authorization` header present, try
bearer; else try cookie" — which is exactly the request-controlled branching Pitfall 1 warns
against, just moved one layer down (into resolver construction rather than the CSRF
interceptor) and now controlling *which identity* the caller gets, not just the CSRF
exemption. Two dangerous shapes fall out of this:
1. A request carrying **both** a stale/expired `Authorization` header (e.g. a CI job with a
   cached bad token) **and** a live session cookie: if bearer verification failing silently
   falls through to trying the cookie, the caller gets a valid identity from a lane it didn't
   intend to use, and any lane-specific behavior (CSRF exemption, rate limits, audit
   attribution) now applies to the wrong lane's rules for a caller who thought they
   authenticated as a service principal.
2. A request carrying a garbage/replayed `Authorization` header and no cookie: if the failure
   is swallowed and treated as "no bearer, must be a cookie-shaped request", it produces a
   confusing `CodeUnauthenticated: no session cookie` instead of the more correct "bearer
   token invalid" — masking the real failure and making the client's own retry logic guess
   wrong about which credential to fix.

**Why it happens:**
Both lanes ultimately produce the same `*mcpauth.TokenInfo` shape, so it's tempting to write
the resolver as `if bearer, try it; on any error, fall through to cookie` — treating the two
lanes as interchangeable fallback options the way `auth.ChainVerifier`'s OIDC-human →
OIDC-service fallback is written (`verifyOIDCBranch` in `internal/auth/chain.go` explicitly
tries human then service). That fallback pattern is correct *within* one mechanism family
(both are OIDC bearer tokens); it is **not** correct *across* mechanism families (bearer vs.
cookie) because those have different trust boundaries — a bearer credential is
attacker-suppliable in a way a sealed, server-signed cookie is not, and CSRF-relevant code
downstream needs to know which one actually won.

**How to avoid:**
The resolver should determine intent **structurally and unambiguously**, not by
try-then-fallback: if the request carries a well-formed `Authorization: Bearer <token>`
header, that is the caller's declared lane — verify it and return success or a hard
`CodeUnauthenticated` bearer-specific failure; never consult the cookie in that path. Only a
request with **no** `Authorization` header at all falls to the cookie resolver. This mirrors
`auth.ChainVerifier`'s own `discriminate()` function, which routes by *shape*, deny-by-default
on the unrecognized branch, and never "tries everything." Reuse that discriminator concept at
the Connect resolver boundary instead of reinventing a looser one.

**Warning signs:** A resolver function with an `if err != nil { return cookieResolve(...) }`
line after a bearer verification attempt. A test suite with no case for "valid cookie present
+ invalid bearer header present" (this exact combination is what an attacker would send).

**Phase to address:**
Headless client lane, same slice as Pitfall 1. Negative test:
`TestBearerFailureNeverFallsThroughToCookie` — request has a valid session cookie AND an
invalid `Authorization` header; assert the response is the bearer-specific
`CodeUnauthenticated`, not a resolved cookie identity.

---

### Pitfall 3: Connect's bearer path bypasses `mcpauth.RequireBearerToken`, silently dropping the Authorization-header parse and the `Expiration` check

**What goes wrong:**
On the MCP lane, `cmd/engram/serve.go`'s `withAuth` wraps the handler with
`mcpauth.RequireBearerToken(chain, ...)`. That wrapper's internal `verify()` (in the go-sdk,
`auth/auth.go`) does two things beyond calling the `TokenVerifier`: it parses
`Authorization: Bearer <token>` out of the header itself, and — separately from whatever the
verifier returned — it hard-rejects a zero or past `TokenInfo.Expiration`
(`"token missing expiration"` / `"token expired"`). This is exactly the mechanism the
milestone-context note about `RequireBearerToken` hard-rejecting a zero `Expiration` is
describing, and it is why `StaticTokenVerifier` sets a 100-year `staticTokenExpirationHorizon`
— that horizon exists **only** to satisfy this specific downstream check.

Connect does not go through `RequireBearerToken` at all — `newConnectSubjectInterceptor` calls
`resolve(ctx, req)` directly and stores whatever `*mcpauth.TokenInfo` comes back, with **no**
expiration check anywhere in that path (confirmed by reading the go-sdk's `auth/auth.go`
directly: the `Expiration` check lives inside the unexported `verify()` helper that only
`RequireBearerToken` calls). If the new Connect bearer resolver is implemented as "call
`auth.ChainVerifier`'s returned function directly," it gets the header-parsing and the
`Expiration` enforcement **from neither lane** — `ChainVerifier` itself does not check
`Expiration`; only `RequireBearerToken`'s `verify()` does, and Connect never calls that. The
practical effect: a `TokenInfo.Expiration` value on the Connect lane becomes decorative. For
OIDC-derived tokens this is partially masked because the underlying OIDC verifier itself
checks the JWT `exp` claim during verification — but for the static-token lane, whose
`TokenInfo.Expiration` is a fabricated 100-year sentinel specifically because *something else*
was expected to reject it, nothing on the Connect lane would ever reject an expired static
token, because nothing on the Connect lane currently understands "expired" at all beyond what
the verifier itself does.

**Why it happens:** The two transports (`net/http` MCP handler wrapped by `RequireBearerToken`,
and the Connect interceptor chain calling `resolve` directly) were built independently and
never shared a bearer-extraction/expiration-check helper, because until now only one of them
(MCP) accepted bearer tokens at all. "Reusing the shipped `auth.ChainVerifier`" (as scoped in
PROJECT.md) is correct at the verifier-composition layer but easy to conflate with "reusing
the shipped bearer-token *handling*," which lives one layer up, in `RequireBearerToken`, and is
MCP-transport-specific (an `http.Handler` wrapper, not something `connect.UnaryInterceptorFunc`
can drop in directly since Connect's request shape differs).

**How to avoid:** Extract the `Authorization: Bearer` header parse and the `Expiration`
zero/past check into a small transport-agnostic helper (or literally reuse a factored-out
version of the go-sdk's `verify()` logic) that both the MCP path (via `RequireBearerToken`,
unchanged) and the new Connect bearer resolver call. Do not let the Connect resolver treat
"got a `*TokenInfo` back with no error" as "authenticated" — it must independently confirm
`Expiration` is non-zero and in the future, exactly as `RequireBearerToken` does today for MCP.

**Warning signs:** The new Connect bearer resolver's happy path is `ti, err := chain(ctx, tok,
req); if err != nil { return nil, err }; return ti, nil` with no reference to
`ti.Expiration` anywhere. A static token whose `staticTokenExpirationHorizon` comment
("`mcpauth.RequireBearerToken` hard-rejects...") is now misleading on the Connect lane because
nothing on that lane does the rejecting.

**Phase to address:** Headless client lane. This is **invisible to a green test suite** unless
a test specifically constructs a `TokenInfo` with a **past** `Expiration` and confirms the
Connect bearer resolver rejects it — a test that only exercises the happy path (valid,
non-expired token) never exercises this gap, and `go vet`/lint have no way to flag "a field
exists but nothing reads it." Negative test:
`TestConnectBearerResolverRejectsExpiredTokenInfo` — feed the resolver a `TokenVerifier` stub
that returns a `TokenInfo{Expiration: time.Now().Add(-time.Hour)}` with `err == nil`; assert
the Connect request is rejected, not merely "the underlying verifier would have caught it" —
this must hold even for a verifier (like a hypothetical future or misconfigured static-token
map) that returns a stale-but-nil-error `TokenInfo`.

---

### Pitfall 4: Two independently-constructed `ChainVerifier`s drift when only one call site is updated

**What goes wrong:** `withAuth` in `cmd/engram/serve.go` is today the *single* call site that
builds `humanVerifier`/`serviceVerifier`/`staticVerifier` from config and composes them via
`auth.ChainVerifier`. Wiring bearer auth onto Connect requires that same composed verifier (or
an equivalent) at the Connect mount call site. If the Connect wiring reconstructs its own
three verifiers from `cfg.OIDC`/`cfg.ServiceAuth` independently, rather than reusing the chain
`withAuth` already built, the two chains can silently diverge the next time either one is
edited — e.g. a future lane added only to `withAuth`, or a bugfix to owner-claim parsing
applied to only one construction. Both compile, both vet clean, both pass their own
lane-scoped tests (each has its own mocked chain), and the drift is invisible until a caller
observes different acceptance behavior between the MCP and Connect lanes for the same bearer
token — a lane-confusion bug of a different flavor than Pitfall 1/2, but from the same root
cause (duplicated logic at a security seam).

**Why it happens:** `withAuth`'s three-verifier construction is presently private to
`cmd/engram/serve.go` and returns an already-wrapped `http.Handler`, not the composed
`mcpauth.TokenVerifier` itself — there's no existing seam to "just reuse" it from
`mountConnect`'s call site without a refactor. The path of least resistance for whoever wires
Connect bearer support is to copy the three `if svcAuth.X != ""` blocks rather than extract and
share them.

**How to avoid:** Refactor `withAuth` to expose the composed `mcpauth.TokenVerifier` (or the
raw lane constructors) as a separate function callable from both `withAuth` (MCP) and the new
Connect-mount wiring — one build site, two consumers. This is the same "one call site" pattern
the milestone already values (`withAuth` is explicitly documented as "the ONE call site that
changes for the service-auth chain").

**Warning signs:** `grep -n "auth.NewService\|auth.NewStaticTokenVerifier\|auth.New("
cmd/engram/*.go internal/server/*.go` returns matches in more than one file/function.

**Phase to address:** Headless client lane. Verification: a single `TestAuthChainSharedBetweenLanes`
(or a code-structure assertion, mirroring the `TestWriteParity` AST-delegation precedent from
v0.11.x) confirming both mount sites call the same constructor function, not parallel
copies.

---

### Pitfall 5: Headless mount silently flips a deployment's exposure on upgrade

**What goes wrong:** `mountConnect` (`internal/server/connectapi.go:362-365`) today returns
immediately when `resolve == nil`, and `resolve` is only non-nil when the UI is enabled
(cookie resolver wired). Every deployment that runs with the UI disabled today has **no**
Connect surface at all — not gated-and-empty, genuinely absent from the mux. The moment
headless mounting exists, that invariant inverts: a deployment upgrading past this milestone
could go from "no Connect endpoint reachable" to "Connect endpoint reachable with bearer auth"
without any deliberate operator action, if the new flag defaults on, or if the flag's absence
is treated as "mount using whatever auth is available" rather than "don't mount." This is a
network-surface exposure regression on upgrade — the exact failure mode PROJECT.md's posture
note calls out ("a deployment with no Connect surface today could gain one").

A second, subtler version: even with the new flag correctly defaulting off, if the *condition*
for mounting becomes `resolve != nil || headlessEnabled` where `headlessEnabled` derives its
default from something that's already true in most deployments (e.g. "true whenever any
service-auth lane is configured," since v0.11.x shipped service auth already), operators who
configured static tokens or client-credentials **for the MCP lane only** get Connect mounted
as a side effect, without ever opting into a Connect-specific exposure decision.

**Why it happens:** The cleanest code change is often "loosen the early-return condition,"
which is easy to write as an OR that silently broadens exposure rather than a genuinely
separate, independently-defaulted-off gate.

**How to avoid:** The new headless-mount config key must be its own explicit
`ENGRAM_`-prefixed field (per the koanf field-registry discipline, DEC-jgq) with a
default-off zero value, checked as its own independent condition — never derived from,
or OR'd with, the existing UI-enabled/service-auth-configured flags. `mountConnect` should
mount when `(UI enabled AND cookie resolver present) OR (headless flag explicitly true AND
bearer chain present)` — two clearly separate booleans, not one loosened one. Document the
default in the Helm chart's `values.yaml` with an explicit comment that it is opt-in, and add
a config-loader test asserting the zero-value/unset case leaves Connect unmounted exactly as
it is pre-milestone.

**Warning signs:** The diff to `mountConnect`'s guard condition is a one-line change to the
existing `if resolve == nil` check rather than an additional, separately-named condition. The
Helm chart's new value has no explicit `false`/empty default committed alongside it. No test
asserts "UI disabled, headless flag unset → Connect not mounted" (the pre-milestone baseline
behavior) still holds.

**Phase to address:** Headless client lane. Negative test: `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag`
— construct config with UI disabled and the new headless flag left at its zero value; assert
`mountConnect` still returns without registering any handler, byte-for-byte the current R1
behavior.

---

### Pitfall 6: Docs, Helm chart, and field registry drift out of sync with the new headless/bearer surface

**What goes wrong:** A new `ENGRAM_` config key, a new CLI subcommand set, and a newly
reachable network endpoint each have their own documentation surface (`docs-site/`, Helm
`values.yaml` comments, `internal/config`'s field registry, and — per this exact milestone's
own #356 item — the committed `gen/` drift-check). Shipping the server-side capability without
updating all of them produces a deployment that *can* run headless but whose Helm chart has no
documented value for it, whose docs-site has no guide describing the bearer-token flow (in the
same style as the existing `guides/reindex` / `guides/embedding-models.md` precedent), and
whose operators have no way to discover the feature except by reading source. This isn't a
correctness bug, but it directly produces the "silently gains an exposed endpoint" complaint
from a different angle: an operator who *does* want headless mode has no documented, supported
way to turn it on safely (CSRF/token lifecycle implications included), so they either don't
adopt it or misconfigure it.

**Why it happens:** Config/docs/chart updates are easy to defer to "polish" and this milestone
already has an explicit "completion tail" bucket (#356 codegen drift) suggesting the project
has a known pattern of shipping code before docs/chart catch up.

**How to avoid:** Treat the Helm `values.yaml` recipe + docs-site guide + `internal/config`
field-registry entry as part of the same unit of work as the headless-mount code itself, not a
follow-up — mirroring how Phase 14 paired every embedder model option with a `values.yaml`
recipe and a docs-site guide in the same phase.

**Warning signs:** `task chart:push`/Helm lint passes but `values.yaml` has no comment block
for the new key. `rg` for the new `ENGRAM_` env var name finds it in `internal/config` and
tests but nowhere under `docs-site/`.

**Phase to address:** Headless client lane (documentation slice, same phase, not deferred).

---

### Pitfall 7: A first-party CLI leaks its bearer token via argv, shell history, or child-process environment

**What goes wrong:** A CLI invoked as `engram store --token=eyJ...` (or similar) puts the
token in `ps auxww` output, shell history files, and any process-list-scraping monitoring —
visible to every other user on a shared CI runner or box, and to any tool that logs the
command line it ran (a very common CI failure mode: the exact invocation, secrets included,
ends up in build logs). A token passed via an env var is safer from `ps`, but is not free
either: env vars are inherited by every child process the CLI spawns (unlikely here, but
relevant if the CLI ever shells out) and, more importantly, are commonly captured whole by
crash-reporting/telemetry libraries that dump `os.Environ()` on panic — a bearer token in
`ENGRAM_TOKEN` ending up verbatim in a Sentry/crash-report payload is a real, easy-to-miss
leak. A CLI built for "autonomous agents and CI" is specifically the profile most likely to be
invoked non-interactively with credentials baked into scripts, making this more likely than
for a human-operated CLI.

**Why it happens:** `--token` flags are the fastest thing to implement and test; env-var
support is added as "the secure alternative" without actually auditing what else in the binary
reads/logs/dumps the full environment.

**How to avoid:** Prefer reading the token from a file path (`ENGRAM_TOKEN_FILE` or
`--token-file`, following the same `ENGRAM_` prefix convention as the server), never accept it
as a bare CLI flag value (accept `--token-file` only, or read stdin), and if an env var is
supported, audit that no panic/crash-report/telemetry path in the CLI dumps `os.Environ()` or
logs its own invocation args verbatim. Never `slog` the resolved token or the raw
`Authorization` header value anywhere in the CLI (mirrors the server's own
no-token-in-logs discipline already established for `StaticTokenVerifier`).

**Warning signs:** `--help` output or README examples show `engram store --token=...` as the
documented usage. Any `recover()`/panic handler in the CLI includes `%+v` of a config struct
that embeds the raw token, or logs `os.Environ()`.

**Phase to address:** Headless client lane (CLI subcommand slice).

---

### Pitfall 8: CLI TLS-verification default and error/exit-code design make automation unsafe or unusable

**What goes wrong:** Two independent traps for a CLI meant to run unattended in CI/agents:
(1) a convenience `--insecure`/`InsecureSkipVerify` flag that's easy to leave on by default
during development and never removed before shipping — silently accepted by an agent script
that never notices TLS verification was off; (2) collapsing every failure (auth rejected,
network unreachable, server 500, CSRF/authz denied, malformed input) to the same exit code 1
— which is exactly what `connectError`'s Connect-code taxonomy (`CodeUnauthenticated`,
`CodePermissionDenied`, `CodeNotFound`, `CodeInvalidArgument`, `CodeFailedPrecondition`,
`CodeInternal`, plus the `context.Canceled`/`DeadlineExceeded` arms) already exists to prevent
server-side. A CLI that throws that taxonomy away at the last mile forces every calling script
to string-match stderr to decide "should I retry" vs "should I alert a human" vs "is my token
just expired" — brittle by construction and exactly the kind of thing this project's own
CLAUDE.md warns against pinning to strings.

**Why it happens:** TLS verification bypass flags get added for local/dev testing against a
self-signed cert and never gated behind an explicit, loud, non-default opt-in. Exit codes get
treated as an afterthought because the interactive human use case ("read the error message")
works fine with a single generic failure code.

**How to avoid:** No TLS bypass by default; if a bypass flag exists at all, require both an
explicit flag AND a matching env var (defense against a script accidentally inheriting a flag
from a copied command line) and print a loud warning to stderr every time it's used. Map the
Connect code taxonomy to a small, stable, documented set of CLI exit codes (e.g. 2 =
auth/credential failure, 3 = permission/CSRF denied, 4 = not found, 5 = invalid
argument/validation, 1 = everything else/internal) so CI and agent callers can branch on
`$?` without parsing text.

**Warning signs:** The CLI's only documented exit codes are "0 = success, 1 = failure." A
`--insecure` flag exists with no corresponding test asserting it is off by default.

**Phase to address:** Headless client lane (CLI subcommand slice).

---

### Pitfall 9: Client/server version skew silently misbehaves instead of failing loud

**What goes wrong:** The CLI talks to the typed Connect stubs generated from `proto/`. This
project's own `buf breaking` discipline keeps the wire contract additive-only, so an *older*
CLI against a *newer* server is generally safe (new fields just don't populate). The dangerous
direction is a *newer* CLI against an *older* server: a CLI built after a field is added (e.g.
a future `cross_spine` on `SearchMemoriesRequest`, per #344) sent to a server that predates
that field either silently no-ops the new behavior (the exact "carried caveat" already
documented for v0.11.x: "the deployed engram server predates this merge, so `supersede_memory`,
citations, and the `categories` filter are not callable until the next release") or, worse,
the CLI interprets an old server's response shape as if the new field were honored, presenting
results the user believes are cross-spine-scoped when they are not — a silent correctness
regression in exactly the search-scope dimension Pitfall 10 below is about.

**Why it happens:** Additive-proto discipline protects the *wire format* from breaking, but
says nothing about whether the *client's assumptions* about server behavior hold — that's a
semantic, not a schema-level, compatibility question, and nothing currently checks it
automatically.

**How to avoid:** Have the CLI report the server's build/version (a lightweight version RPC or
reuse of an existing metadata surface) and, for any CLI flag that depends on a
recently-added field, either detect the server doesn't support it and fail loud
("cross_spine requires engram server ≥ v0.12.0") or clearly label output as
possibly-unscoped when the server is too old to guarantee the flag was honored. At minimum,
document the version-skew caveat in the CLI's own docs guide the way the v0.11.x carried
caveat was documented in PROJECT.md.

**Warning signs:** No `engram --version`-equivalent server round-trip exists; the CLI has no
test simulating "new client flag against a server response that omits the corresponding
field."

**Phase to address:** Headless client lane (CLI subcommand slice); cross-reference into
Cross-spine memory recall phase for the `cross_spine`-specific instance.

---

### Pitfall 10: `scope` becoming optional widens the authz filter, not just the search filter

**What goes wrong:** `search_memory`'s `cross_spine` (mirroring `SearchDiscoveryArgs.CrossSpine`)
makes `scope` optional. The existing `SearchDiscoveries` Connect handler already has a
precedent for this exact shape: `CrossSpine: req.Msg.Scope == ""` — an *empty* scope is
deliberately overloaded to mean "search everywhere," and `deps.searchDiscovery`'s
`effectiveDiscoveryScope` explicitly rejects an empty scope **unless** `CrossSpine == true`.
The danger for `search_memory` is that "everywhere" for a discovery scope (all
`discovery:*` scopes) is a fundamentally different authorization surface than "everywhere" for
a regular memory scope: memories are not confined to a `memory:*`-style namespace the way
discoveries are confined to `discovery:*` — a memory record's `scope` is an arbitrary
repo/workspace/worktree string. If cross-spine memory search is implemented by simply *not
appending a scope filter* to the Qdrant query when `cross_spine=true`, the remaining filter is
whatever the **owner/authz** `Must` clause already provides — which is correct *only if* that
owner clause is unconditionally applied ahead of, and independently of, the scope filter, the
same way DEC-cgb requires. The actual risk is subtler than "forgot to filter by owner": it's
that the *existing* single-scope code path may have historically leaned on `scope` being
required as an implicit second narrowing signal in some code path (e.g., a helper that only
gets called with a non-empty scope today and was never audited for empty-scope behavior),
and making `scope` optional exercises that helper on an input it was never designed for.

**Why it happens:** The `SearchDiscoveries` precedent makes "optional scope = cross_spine
flag" look like a solved, safe pattern to copy — but discovery and memory categories don't
share exactly the same authz-filter code path (discoveries live in `discovery:*` scopes by
convention; regular memories don't have an equivalent namespace convention), so copying the
*shape* of the fix without re-verifying the *filter composition* underneath it is the trap.
Additionally, "scope optional" is easy to implement as "if scope=='' skip the scope clause,"
which is correct only if the authz clause was already unconditionally composed — a fact that
must be verified by reading `Store.Search`'s filter-building code, not assumed by analogy.

**How to avoid:** Before touching `search_memory`'s handler, read `Store.Search`'s Qdrant
filter construction end to end and confirm the authz `Must` clause (owner/visibility bucket
decision) is composed **unconditionally**, with the scope condition as a genuinely separate,
optional `Must` entry — never a single combined condition where omitting scope could
accidentally also omit part of the authz gate. Add the cross-spine flag as an explicit,
named parameter through the same `store.SearchOptions` struct discipline already established
in v0.11.x (D-09: "Two adjacent `[]string` params transpose silently"; the same "explicit
struct field over positional/implicit" lesson applies to a boolean flag as much as to a slice
param). Write the negative test *before* the implementation.

**Warning signs:** The diff to `Store.Search` shows the scope condition and the owner/authz
condition built by a single function that takes `scope string` and only adds *one* combined
filter clause for "isolation" — i.e., scope and authz aren't visibly two separate `Must`
entries in the Qdrant filter you can point to independently.

**Phase to address:** Cross-spine memory recall. Negative test:
`TestCrossSpineSearchNeverBypassesOwnerFilter` — two different authenticated owners each with
records in overlapping scope names; owner A searches with `cross_spine=true` and an empty
scope; assert owner B's private records never appear in A's results, using real Qdrant
(testcontainers, matching the project's existing `-race` vs. real-Qdrant precedent for
authz-adjacent features) rather than a mock that could paper over a real filter-composition
bug.

---

### Pitfall 11: `authz.Decision.diag` wiring leaks PII, aids policy probing, or is only exercised on the deny path

**What goes wrong:** `authz.Decision` (`internal/authz/authz.go`) carries an unexported
`diag cedar.Diagnostic` computed on **every** `DecideBucket`/`DecideRecord` call — allow and
deny alike — but currently has zero readers. Wiring it to debug logging has three distinct,
independent failure modes:
1. **PII/identity leakage.** DEC-wot already locked "spans carry `engram.owner` (opaque
   `sub`) only; exclude actor/email as PII" specifically because raw claim values are
   sensitive. A `cedar.Diagnostic` dump can easily include the full Cedar entity/request,
   which — depending on how the PDP's Principal/Memory schema populates its attributes —
   may embed the same owner/actor strings DEC-wot deliberately keeps out of telemetry. Wiring
   `diag` to logging without re-applying that same redaction discipline reopens a decision
   the project already made once.
2. **Policy-probing detail.** A verbose diagnostic (which policy IDs matched/didn't match,
   why) is exactly the kind of oracle an attacker probing for authz bypasses wants — Cedar's
   own diagnostic is designed for *operator* debugging, not for being echoed anywhere a
   caller (even indirectly, via a shared log aggregator with broader read access than the
   engram operator) might see it. It must land in `debug`-level structured logging /
   span events only — never in any client-facing error (this dovetails with `connectError`'s
   existing discipline of a generic `CodeInternal` message with no store/embed internals
   leaked to the client).
3. **Only-correct-on-deny diagnostics.** It's tempting to wire logging as
   `if !decision.Allow { log(decision.diag) }` — cheap, and deny is the "interesting" case.
   But `diag` is computed unconditionally per the doc comment ("computed on every decision"),
   and an **allow**-path diagnostic is just as valuable for debugging "why did this succeed
   when I expected a deny" (e.g. investigating an over-permissive policy, or confirming a
   fix). A deny-only wiring silently produces a diagnostic feature that only ever shows half
   the picture, and worse, nobody notices because the deny path still "looks complete" — it's
   invisible until someone specifically needs to debug an unexpected *allow*.

**Why it happens:** Deny-path logging is the obvious, minimal-effort interpretation of
"diagnostics for what the server rejects," and PII-safety is easy to forget because
`cedar.Diagnostic` is an opaque third-party type whose exact field shape isn't obvious without
reading the cedar-go source — nobody currently in this codebase has needed to look inside it.

**How to avoid:** Log `diag` on both allow and deny at `debug` level (never `info`/`warn`, to
bound volume — see Performance Traps below), gated the same way other debug-only spans/logs in
this codebase already are, and explicitly enumerate which `cedar.Diagnostic` fields are safe
to log (policy IDs, allow/deny reason codes) versus which carry request/entity attribute
values that must be redacted or omitted, re-applying DEC-wot's owner-only-no-actor-no-email
rule to whatever attribute set the diagnostic exposes. Add a code-review checklist item (or a
test asserting a fixed set of safe field names) rather than trusting ad hoc judgment at the
call site.

**Warning signs:** The wiring is a single `if !decision.Allow` branch. A `slog.Debug` call
passes `decision` (or an unredacted `diag`) as a single `%+v`-style value rather than
extracting named, reviewed fields.

**Phase to address:** Authz diagnosability.

---

### Pitfall 12: "Improving" a validation error message masks a different real failure

**What goes wrong:** #360 names a live instance of this exact class: `store_memory` reports
`missing properties: ["content"]` when the actual fault is an over-long `summary`. This kind
of bug is almost always a symptom of *validation logic that isn't independently testable per
field* — e.g., a JSON-schema `oneOf`/`allOf` construction where an over-long `summary`
disqualifies the whole schema branch that would otherwise have matched, and the validator's
generic "which required property is missing" fallback reports whichever property that
disqualified branch happened to require, not the field that actually failed. Simply changing
the string ("content is required" → something else) without fixing the underlying branch-
misattribution risks two new failure modes: (a) the new message is *also* wrong for a
different malformed-input shape nobody tested (the fix "cures" the one reported case and
introduces a fresh mismatch for another combination of bad fields), or (b) the message is
correct in isolation but a test written to pin the new string text is now a brittle assertion
that will break the next time wording is touched for an unrelated reason, without ever
verifying the message is *accurate* for the actual failing field.

**Why it happens:** Error-message bugs get fixed by testing the one reported repro case (an
over-long summary produces the wrong message) and asserting the new string appears — passing
that one test looks like "fixed," but if the fix is a string substitution rather than a
validation-attribution fix (report the field whose own constraint actually failed, not a
downstream schema artifact), the underlying misattribution mechanism is untouched and will
resurface on the next multi-field-invalid input.

**How to avoid:** Root-cause the *validation branch selection*, not the string: find where the
schema/validator decides "which required property to name" and confirm it can name any field
whose own independent constraint fails (content empty, summary too long, category invalid,
etc.), not just a hardcoded "content" fallback. Test message *accuracy*, not exact text:
assert the error mentions the actually-invalid field name (e.g. `strings.Contains(err.Error(),
"summary")` and NOT `strings.Contains(err.Error(), "content")`) for a case where only summary
is invalid — this pins correctness without pinning brittle full-string wording, and will still
catch a regression where the misattribution comes back under a *differently worded* message.
Also add a matrix test: for each single-field-invalid case (bad content, bad summary, bad
category, bad scope, ...) assert the reported field matches the actually-broken one — this is
the only way to catch the "fixed the reported case, broke a different one" failure mode.

**Warning signs:** The fix diff is a one-line string change with no change to the validation
control flow. The only new/changed test asserts the exact previously-reported string
(`"summary too long"` for example) rather than a matrix of single-field-invalid cases.

**Phase to address:** Failure legibility. This class of bug **survives a green test suite** by
default — the original bug shipped despite full lint/vet/test coverage precisely because
nothing exercised "summary too long AND content present" as a distinct case from "content
missing"; a fix that only adds back a test for the one reported repro leaves every *other*
multi-field-invalid combination equally unverified.

---

### Pitfall 13: `reindex --resume`'s equality key needs the target's *tags*, not just its content — and the target lookup doesn't fetch them yet

**What goes wrong:** `Store.Reindex`'s resume-skip check (`internal/store/store.go`, around
line 2667) currently compares only `ti.content == content` (plus the identity stamp) to decide
whether a point can be skipped as unchanged. `reindexTarget` (the per-id resume-lookup shape)
has exactly two fields, `content` and `identity` — **no `tags` field at all** — and
`reindexTargetContents` only ever reads `p.Payload["content"]` and the identity key out of the
target collection's stored payload; it never reads `p.Payload["tags"]`. A tag-only edit
(content unchanged, tags changed) is therefore currently invisible to the resume check: the
source content matches the target's stored content, so the point is skipped as `Unchanged`,
even though its vector was originally embedded via `EmbedText(m.Content, m.Tags)` — folding
tags into the embedding — so a tag change *should* trigger a re-embed but silently doesn't.

Fixing this is **not** a one-line change to the comparison (`ti.content == content &&
ti.tags == tags`) — `reindexTarget` must first grow a `tags` field, and
`reindexTargetContents` must be extended to actually read `p.Payload["tags"]` from the
**target** collection's stored payload (not just the source point, which already has `m.Tags`
available via `fromPayload`). Get this ordering wrong — comparing against a always-zero-value
`ti.tags` because the target lookup was never extended — and the equality check either (a)
always treats tags as matching (a `nil`/empty `ti.tags` structurally-equal to some other
default), silently reintroducing the exact bug being fixed, just with different code, or (b)
always treats tags as mismatching (comparing an unpopulated `nil` slice against a real slice
from the source), which defeats resume's entire purpose by re-embedding every already-current
record on every resumed run — a correctness bug in the *opposite* direction, invisible because
it "just" costs extra embedder calls and looks like resume "still working" (nothing errors;
everything just silently gets slower and more expensive, potentially by a lot, on every
resumed run).

Also: tags order matters for slice equality — `m.Tags` (source) and the target's stored tags
must be compared with the same order-independent discipline `contentFingerprint` already uses
for tags (`slices.Clone` + `slices.Sort` before comparing/hashing) — a naive `slices.Equal`
without sorting first would treat `["a","b"]` and `["b","a"]` as different, again re-embedding
unnecessarily (or, if the *source* embed step sorts tags before embedding but the *comparison*
doesn't, potentially masking a real change the other direction).

**Where partial-progress correctness bugs hide:** a reindex run interrupted midway through a
large collection, then resumed, has *already* processed and skipped some prefix of records
under the old (content-only) equality key before this fix lands. If the fix changes the
equality key going forward but doesn't account for records a prior *unpatched* resume run
already (incorrectly) marked "unchanged" and skipped, those specific records stay stale
indefinitely — a resumed run only re-evaluates points it scans in the current pass; it does
not retroactively re-check points a previous run's `Unchanged` counter already walked past
correctly-for-content-but-not-for-tags. This is a genuine "looks done but isn't" trap:
`ReindexResult.Unchanged` going up looks like success, not a sign that some subset of those
"unchanged" records actually needs re-embedding.

**Why it happens:** The existing code comment even states the (currently false, post-fix-needed)
assumption directly: *"equal content (and, from the same source payload, equal tags) re-embeds
to an equal vector"* — content-implies-tags was true only because, historically, nothing could
change tags without also changing content through the same write path. That assumption breaks
the moment tags can be edited independently of content (which they already can, via
`update_memory`'s tag-replace semantics) — the reindex resume code just never caught up to that
fact.

**How to avoid:** (1) Add `tags []string` to `reindexTarget`; (2) extend
`reindexTargetContents` to read `p.Payload["tags"]` from the target collection alongside
content/identity; (3) compare tags with the same sort-before-compare discipline as
`contentFingerprint`; (4) update the stale doc comment on `reindexTargetContents`/the resume-skip
block since it currently asserts the false content-implies-tags equivalence; (5) explicitly
decide and document what happens to records a prior *unpatched* resume run already skipped
incorrectly — likely: document that a full non-resume reindex (or a repair sweep) is needed
once, post-fix, to catch any records whose tags diverged from what a stale resume run
believed, since the fix only changes behavior for records evaluated *after* it ships.

**Warning signs:** `reindexTarget`'s struct literal in code review still has exactly two
fields. The PR diff touches only the `if ti.content == content && ...` comparison line and
nothing in `reindexTargetContents`. The doc comment above the resume-skip block still says
"equal content ... equal tags" as an assumption rather than an explicit check.

**Phase to address:** Reindex resume correctness. Negative test:
`TestReindexResumeSkipsOnContentMatchTagsDiffer` — seed source and target with identical
content but different tag sets (including a same-elements-different-order case); run
`Reindex(Resume: true)`; assert the point is **re-embedded** (shows up in `Upserted`, not
`Unchanged`) and that the resulting target vector reflects the new tags (e.g. via a
differ-style assertion comparable to the existing `TestRetrievalEval_AsymmetryDiffer`
precedent, not just "some vector changed"). Add a second case,
`TestReindexResumeSkipsWhenContentAndTagsBothMatch` (order-independent), as the paired
positive control so the fix doesn't overshoot into "always re-embeds."

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|------------------|
| Building a second, Connect-local `ChainVerifier` instead of sharing `withAuth`'s | Faster to land the Connect bearer slice without refactoring `serve.go` | Silent drift between MCP and Connect bearer acceptance the next time either is edited (Pitfall 4) | Never — refactor `withAuth` to expose the shared constructor first |
| Inferring CSRF-exemption from header/cookie presence instead of resolver provenance | No new context-key plumbing needed | Full CSRF bypass on all six write RPCs (Pitfall 1) | Never |
| Pinning a validation-error fix to the one reported repro string | Closes the ticket fast | Leaves the misattribution mechanism live for every other multi-field-invalid combination (Pitfall 12) | Never for a security- or correctness-relevant validator; acceptable only for genuinely cosmetic wording with no attribution logic underneath |
| Shipping headless-mount as a loosened `mountConnect` guard rather than a new independent flag | Smaller diff | Silent exposure flip on upgrade for existing deployments (Pitfall 5) | Never |
| Deferring Helm/docs updates to a follow-up PR | Ships the code sooner | Feature is unsafe-by-default to adopt because nobody can find the documented, supported way to configure it (Pitfall 6) | Only for genuinely internal/dev-only flags never exposed in `values.yaml` |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|--------------------|
| `mcpauth.RequireBearerToken` (go-sdk) vs. Connect interceptor chain | Assuming Connect's bearer resolver inherits header-parsing + `Expiration` enforcement "for free" because both call the same `auth.ChainVerifier` | Explicitly re-implement/reuse `RequireBearerToken`'s `verify()`-equivalent logic (header parse + `Expiration` check) at the Connect resolver, since Connect never passes through the go-sdk's HTTP middleware (Pitfall 3) |
| Cookie resolver (`webauth.Resolver`) + new bearer resolver, combined at one Connect mount | Try-bearer-then-fallback-to-cookie, treating the two lanes as interchangeable the way `verifyOIDCBranch` treats human/service OIDC | Route by structural presence of `Authorization` header, deny-by-default, never fall through across mechanism families (Pitfall 2) |
| `newConnectResealInterceptor` (unconditional, not procedure-gated) meeting a bearer-authenticated request | Assuming it needs bearer-awareness added | It is already lane-safe by construction — `Reseal` no-ops when the request carries no session cookie — but any future change to `Reseal` must preserve that it triggers only off an actual request-supplied cookie, never off the resolved `TokenInfo`/Subject, or it risks minting a session cookie for a bearer-only caller |
| `SearchDiscoveries`'s `Scope == ""` → `CrossSpine` precedent, copied to `search_memory` | Assuming the discovery-scope authz-filter shape transfers unchanged to the memory-scope authz-filter shape | Re-verify `Store.Search`'s filter composition keeps the authz `Must` clause unconditional and independent from the (now optional) scope clause before reusing the pattern (Pitfall 10) |
| `store.SearchOptions` positional-vs-struct discipline (D-09 precedent) | Adding `cross_spine` as a new bare boolean parameter threaded positionally alongside existing filter args | Add it as a named `SearchOptions` field, continuing the struct-over-positional-params discipline already adopted for `Tags`/`Categories` |
| `authz.Decision.diag` (cedar-go `cedar.Diagnostic`) → `slog` | Passing the whole `Decision`/`diag` value to a logger in one `%+v` | Extract and redact named fields per DEC-wot's existing owner-only-no-actor-no-email discipline before logging (Pitfall 11) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Logging `authz.Decision.diag` at a non-debug level, or on every allow decision in a hot recall path | OTLP/log volume spikes disproportionately to traffic; log-storage cost jumps right after this milestone ships | Gate at `debug` level only, matching existing OTel sampler/debug conventions (DEC-7qd); consider sampling if `DecideBucket` is called O(buckets) per recall (already the case per ADR `engram-cdr1`) | As soon as debug logging is left on in a production-traffic environment, or if diag logging is accidentally wired to `info` |
| Cross-spine memory search with an unbounded owner/visibility bucket enumeration | `search_memory(cross_spine=true)` latency degrades non-linearly as a caller's authorized scope/bucket count grows | Confirm cross-spine still resolves to an O(buckets) `DecideBucket` call, per the existing ADR `engram-cdr1` pattern, not a fallback to O(records) scanning when scope is empty | A caller with many distinct scopes under one owner, once cross-spine search is available broadly |
| Reindex resume's per-page target lookup (`reindexTargetContents`) growing per-point payload size once `tags` is added | Reindex throughput regresses measurably on large collections after the tags fix lands, since each page's target `Get` now also transfers tag payloads | Keep the lookup O(pages) as already documented ("One Get per page keeps the lookup O(pages), not O(points)") — adding a field to the fetched payload doesn't change that big-O, but validate it in a benchmark on a realistic-size fixture before/after the fix | Very large tag sets per record, or very large collections, though this is a minor/likely-acceptable regression relative to correctness |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| CSRF exemption keyed on request-controlled input (header/cookie presence) | Full CSRF bypass on all six write RPCs for cookie-authenticated victims | Key exemption on resolver-set provenance only (Pitfall 1) |
| Bearer verification failure silently falling through to cookie resolution | Confused-deputy: caller gets identity/privileges from the wrong lane | Discriminate by credential shape, deny-by-default, no cross-lane fallback (Pitfall 2) |
| Connect bearer path skipping `Expiration` enforcement | An expired (or, for static tokens, a token an operator believes was rotated out) bearer stays valid indefinitely on the Connect lane specifically | Explicit `Expiration` check at the Connect resolver, independent of the MCP-only `RequireBearerToken` wrapper (Pitfall 3) |
| Headless mount defaulting on, or derived from an unrelated already-true condition | Deployment upgrade silently gains a reachable, bearer-authenticated network surface it never opted into | Independent, explicitly-defaulted-off `ENGRAM_` flag; regression test pinning the pre-milestone "not mounted" baseline (Pitfall 5) |
| CLI token via bare `--token` flag or unaudited env var propagation | Token visible in `ps`, shell history, CI logs, or crash-report environment dumps | File-based or stdin credential input; audit crash/telemetry paths for `os.Environ()` dumps (Pitfall 7) |
| `authz.Decision.diag` logged unredacted | Policy-probing oracle for an attacker; potential PII leak reopening DEC-wot | Redact to a reviewed, named field allowlist; log both allow and deny paths at debug level (Pitfall 11) |
| Cross-spine search filter composed as a single combined scope+authz condition | A refactor that "removes" the scope clause for `cross_spine=true` could accidentally weaken the authz clause bundled with it | Keep authz and scope as two independently-composed, always-present-except-scope Qdrant `Must` entries (Pitfall 10) |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| CLI collapses every failure to exit code 1 with free-text stderr | CI/agent callers can't distinguish "retry me" from "fix your config" from "this is a real bug," so scripts either retry blindly or alert on everything | Map the Connect code taxonomy to a small stable exit-code contract (Pitfall 8) |
| CLI silently no-ops a new flag against an old server | Caller believes a query was cross-spine-scoped (or otherwise using new semantics) when the server ignored the flag entirely | Detect and fail loud, or clearly label output as version-uncertain (Pitfall 9) |
| Validation error names the wrong field | Caller "fixes" the field the error named, resubmits, and gets the same rejection for a different reason — a confusing, multi-round-trip debugging loop | Attribute errors to the actually-failing field, verified per-field, not per reported repro (Pitfall 12) |
| Reindex `--resume` reports `Unchanged` for records that are actually stale (pre-tags-fix) | Operator trusts a completed resume run believing the collection is fully current; tag-based recall precision silently degrades for those records with no error surfaced anywhere | Document + provide a one-time repair path after the tags fix ships (Pitfall 13) |

## "Looks Done But Isn't" Checklist

- [ ] **Bearer identity on Connect:** Often missing an explicit provenance field distinguishing
      it from the cookie lane — verify the CSRF interceptor branches on a resolver-set tag, not
      on header/cookie presence (grep `newConnectCSRFInterceptor` for any `req.Header()` or
      `.Cookie(` reference beyond the existing cookie-lane verify call).
- [ ] **Connect bearer resolver:** Often missing the `Expiration` check `RequireBearerToken`
      performs for MCP — verify with a stale-`TokenInfo` unit test, not just a happy-path one.
- [ ] **Headless mount flag:** Often missing an explicit off-by-default `ENGRAM_` config test —
      verify a config-loader test asserts the zero-value case leaves `mountConnect` mounting
      nothing.
- [ ] **`cross_spine` on `search_memory`:** Often missing a cross-owner isolation test that
      actually exercises the empty-scope path against real Qdrant, not a mock — verify a
      two-owner `-race`-safe testcontainers test exists (mirroring the idempotency/supersession
      precedent).
- [ ] **`authz.Decision.diag` logging:** Often missing field-level redaction — verify the log
      call passes named, reviewed fields, not the whole `Decision`/`diag` value.
- [ ] **Validation error message fix (#360):** Often only tests the one reported repro string —
      verify a matrix of single-field-invalid cases each reports the correct field.
- [ ] **`reindex --resume` tags fix (#345):** Often only changes the comparison line — verify
      `reindexTarget`/`reindexTargetContents` were actually extended to fetch the target's
      stored `tags`, and that tag comparison is order-independent.
- [ ] **CLI credential handling:** Often accepts a bare `--token` flag "for convenience" —
      verify only file/stdin-based credential input is documented/supported.
- [ ] **Docs/chart/config parity:** Often shipped one phase behind the code — verify the new
      `ENGRAM_` key has a `values.yaml` comment and a docs-site guide in the same phase.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|------------------|
| CSRF exemption keyed on request-controlled input (shipped, then caught) | LOW if caught before release (revert the exemption condition, add the negative test); HIGH if it reached a real deployment (must treat as an active vulnerability — rotate `ENGRAM_UI_COOKIE_KEY` per the existing `engram-slr8` kill-switch precedent to invalidate all live sessions, then patch) | Add provenance field; add `TestCSRFNotExemptedByMissingHeaderOrCookie`; if shipped, follow incident process — rotate cookie key, patch, backport |
| Connect bearer skipping `Expiration` enforcement | LOW — add the check and the negative test; no data-layer damage, only an authentication-lifetime gap | Add explicit `Expiration` check at the resolver; add `TestConnectBearerResolverRejectsExpiredTokenInfo` |
| Headless mount accidentally defaulted on | LOW if caught pre-release (flip default, add the regression test); MEDIUM if a real deployment upgraded and unknowingly exposed Connect (must audit access logs for the exposure window, then patch + document in release notes) | Add the independent flag with an off default; audit `newConnectAccessLogInterceptor` logs for the affected window if already shipped |
| `reindex --resume` tags-staleness (already-run resumes under the old key) | MEDIUM — no data loss, but requires a one-time non-resume (or repair-sweep) reindex to correct any records a prior unpatched resume incorrectly skipped | Ship the fix; run/communicate a one-time full reindex (or a targeted repair pass keyed on tag-payload presence) as a documented follow-up, mirroring the `engram reindex` operator-guide precedent |
| `authz.Decision.diag` leaking unredacted fields into logs | LOW-MEDIUM depending on log retention/access scope — treat as a PII-handling incident if any actor/email-equivalent value reached a log sink broader than the operator | Add field allowlist/redaction; purge or restrict access to the affected log window per the org's log-retention policy |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase (theme) | Verification |
|---------|---------------------------|----------------|
| 1. CSRF exemption keyed on request-controlled input | Headless client lane — FIRST slice, before any other bearer-dependent code | `TestCSRFNotExemptedByMissingHeaderOrCookie` + `TestCSRFExemptedOnlyByBearerProvenance` |
| 2. Resolver silently falls through cookie↔bearer | Headless client lane | `TestBearerFailureNeverFallsThroughToCookie` |
| 3. Connect bearer skips header-parse/`Expiration` enforcement | Headless client lane | `TestConnectBearerResolverRejectsExpiredTokenInfo` |
| 4. Duplicated `ChainVerifier` construction drifts | Headless client lane | Structural/AST test asserting one shared constructor, or a config-change regression test exercised against both mount sites |
| 5. Headless mount defaults on / flips exposure on upgrade | Headless client lane | `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag` |
| 6. Docs/chart/config drift | Headless client lane (docs slice, same phase) | Chart-lint + docs-site guide present in the same PR as the code |
| 7. CLI credential leakage (argv/env/crash reports) | Headless client lane (CLI slice) | Manual review + no `--token` flag accepting a bare value; crash-handler audit |
| 8. CLI TLS default / exit-code collapse | Headless client lane (CLI slice) | Exit-code contract test per Connect-code class; TLS-bypass-off-by-default test |
| 9. Client/server version skew | Headless client lane (CLI slice); cross-check in Cross-spine memory recall | Version-mismatch simulation test against an old-shaped response |
| 10. `cross_spine` widening the authz filter | Cross-spine memory recall | `TestCrossSpineSearchNeverBypassesOwnerFilter` (real Qdrant, two owners) |
| 11. `authz.Decision.diag` PII/probing/deny-only logging | Authz diagnosability | Field-allowlist test; assert both allow and deny paths log |
| 12. Validation error message masks a different failure | Failure legibility | Single-field-invalid matrix test asserting per-field message accuracy |
| 13. Reindex resume equality key missing tags | Reindex resume correctness | `TestReindexResumeSkipsOnContentMatchTagsDiffer` + `TestReindexResumeSkipsWhenContentAndTagsBothMatch` |

## Sources

- `internal/server/connectcsrf.go`, `internal/server/connectapi.go`, `internal/server/connectauth.go`,
  `internal/server/connectreseal.go`, `internal/server/identity.go`, `internal/server/connecterror.go`,
  `internal/server/idempotency.go`, `internal/server/tools.go` (read directly, current `main`)
- `internal/auth/chain.go`, `internal/auth/static_token.go`, `internal/webauth/resolver.go`,
  `internal/webauth/reseal.go` (read directly, current `main`)
- `internal/store/store.go` (Reindex/resume path, lines ~2620-2758, read directly)
- `internal/authz/authz.go` (`Decision`/`diag`, read directly)
- `cmd/engram/serve.go` (`withAuth`, `mountMCPRoutes`, read directly)
- `github.com/modelcontextprotocol/go-sdk@v1.6.1` `auth/auth.go` (`RequireBearerToken`'s
  `verify()` — bearer header parse + `Expiration` enforcement — read directly from the module
  cache to confirm it is NOT invoked anywhere in the Connect interceptor chain)
- `.planning/PROJECT.md` — v0.12.x milestone scope, milestone #1 risk note, posture note,
  Decisions section (DEC-cgb, DEC-xa6, DEC-wot, DEC-jgq, ADR `engram-cdr1`, ADR `engram-slr8`),
  and the v0.11.x retrospective entries this milestone explicitly builds on (map-orientation
  inversion, fingerprint-field-list omission, fail-closed-as-first-test precedent)

---
*Pitfalls research for: adding headless bearer auth, a headless-mountable Connect API, a
first-party CLI client, cross-scope search, and authz/error diagnostics to engram (v0.12.x)*
*Researched: 2026-07-29*
