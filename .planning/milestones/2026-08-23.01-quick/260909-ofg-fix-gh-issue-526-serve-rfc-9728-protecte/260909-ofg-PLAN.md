---
phase: quick-260909-ofg
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/config/registry.go
  - internal/config/config.go
  - internal/config/mcp_resource_test.go
  - cmd/engram/wellknown.go
  - cmd/engram/wellknown_test.go
  - cmd/engram/mcproute.go
  - cmd/engram/mcproute_test.go
  - cmd/engram/serve.go
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/deploy.md
  - docs-site/src/content/docs/reference/auth.md
  - charts/engram/values.yaml
  - charts/engram/templates/_helpers.tpl
  - Taskfile.yaml
autonomous: true
requirements: [GH-526]

estimate:
  tokens: 72000
  raw_tokens: 45000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "An unauthenticated GET of /.well-known/oauth-protected-resource returns 200 with Content-Type application/json and an RFC 9728 document."
    - "An unauthenticated GET of the RFC 9728 §3.1 path-suffix form (/.well-known/oauth-protected-resource + the resolved ENGRAM_MCP_PATH, e.g. /.well-known/oauth-protected-resource/mcp) returns the same document."
    - "authorization_servers[0] equals ENGRAM_OIDC_ISSUER when an issuer is configured, and the key is absent entirely when no issuer is configured."
    - "resource equals ENGRAM_MCP_RESOURCE_URL verbatim when that is set; when it is unset, resource is derived from the request origin and the resolved MCP path."
    - "When ENGRAM_MCP_RESOURCE_URL is set, no request header can influence the served resource value."
    - "Setting ENGRAM_MCP_PATH to /.well-known or any path under it fails startup with a clear reserved-root error instead of shadowing the metadata routes."
    - "With ENGRAM_MCP_PATH=/ (legacy root catch-all), a POST to /.well-known/oauth-protected-resource still reaches the MCP transport — the escape hatch is not narrowed."
    - "With ENGRAM_MCP_RESOURCE_URL set and ENGRAM_OIDC_RESOURCE_METADATA unset, the 401 WWW-Authenticate challenge points at a URL this server actually serves."
    - "Every other path keeps today's behavior: /.well-known/anything-else is 404, POST /.well-known/oauth-protected-resource is 404 in non-escape-hatch mode."
  artifacts:
    - cmd/engram/wellknown.go
    - cmd/engram/wellknown_test.go
    - internal/config/mcp_resource_test.go
  key_links:
    - "runServe (cmd/engram/serve.go) mounts the metadata routes on the same mux as mountMCPRoutes, outside withAuth — no bearer gate reaches them."
    - "reservedMCPRoots (cmd/engram/mcproute.go) must contain /.well-known or resolveMCPPath will let ENGRAM_MCP_PATH collide with the new mounts."
    - "resolveResourceMetadataURL feeds withAuth's resourceMetadataURL argument, tying the 401 challenge to a path this server serves."
---

<objective>
Serve the RFC 9728 protected-resource metadata document from engram itself, at both
`/.well-known/oauth-protected-resource` and the §3.1 path-suffix form, unauthenticated, built from
config engram already holds — closing GitHub issue #526.

Purpose: engram already emits an RFC 9728 challenge on 401 (`WWW-Authenticate: Bearer
resource_metadata="<url>"`) but never serves the document that URL points at, so something in front
of it has to synthesise one. LiteLLM in `oauth_passthrough` mode deliberately does not synthesise —
it fetches the upstream's own document and 502s when there is none — so no standard OAuth MCP
client can discover how to authenticate to engram behind it. RFC 9728 places this document on the
resource server, not the gateway.

Output: a new `cmd/engram/wellknown.go` handler + mount, one new env-only config key
(`ENGRAM_MCP_RESOURCE_URL`), `/.well-known` added to `reservedMCPRoots`, table-driven tests, and
docs + Helm chart wiring for the new key.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@CLAUDE.md
@.planning/STATE.md

@cmd/engram/mcproute.go
@cmd/engram/mcproute_test.go
@internal/config/registry.go
</context>

<decisions>

These two questions were left open by issue #526. Both are decided here. The executor implements
them as written and does not revisit them.

**D-01 — `scopes_supported` stays a hard-coded constant `["offline_access"]`. NOT configurable.**
Rationale: `offline_access` is exactly what the retired agentgateway-synthesised document
advertised (`argocd/app-configs/agentgateway/mcp-engram.yaml`, `scopesSupported`), so a constant
reproduces the shipped behaviour byte-for-byte with zero migration. There is no second known
consumer wanting a different value. A constant can be widened into a config key later without
breaking anyone; a config key is a permanent `ENGRAM_` surface that can never be withdrawn. Same
reasoning applies to `bearer_methods_supported`, which is the constant `["header"]`.

**D-02 — `ENGRAM_OIDC_RESOURCE_METADATA` DOES gain a served-path default, but only when
`ENGRAM_MCP_RESOURCE_URL` is set.** Rationale: the acceptance criterion is that the challenge URL
must stop pointing at nothing. Deriving it needs a public origin, and the only honest source of one
at server-construction time is `ENGRAM_MCP_RESOURCE_URL` — `withAuth` receives a single static
string when the middleware is built, so it cannot do the per-request `X-Forwarded-*` derivation the
handler does. Synthesising an absolute URL from the listen address instead would produce something
like `http://:8080/...`, which is worse than emitting nothing. So: an explicitly set
`ENGRAM_OIDC_RESOURCE_METADATA` always wins; if it is unset and `ENGRAM_MCP_RESOURCE_URL` is set,
the challenge URL is derived from that origin plus the served path; if both are unset, behaviour is
exactly what ships today (no `resource_metadata` in the challenge).

**D-03 — `ENGRAM_MCP_RESOURCE_URL` is env-only, with no `--mcp-resource-url` flag.** Rationale: it
is a deployment-topology value (the public URL a gateway exposes), set once in a Helm values file,
never typed at a prompt. Keeping it flag-free leaves `cmd/engram/testdata/help.golden` and
`catalog.golden` untouched. The `--oidc-resource-metadata` flag's usage string is likewise left
unchanged, for the same golden-stability reason and because D-02 makes it a pure default-fill that
never changes what an explicitly-passed flag does.

**D-04 — `ENGRAM_MCP_RESOURCE_URL` is validated in `cmd/engram`, not in `Config.Validate`.**
Rationale: `Config.Validate`'s own doc comment scopes it to "the fields every command's
store/embedder path consumes" and explicitly excludes serve-only fields such as the listen address.
`ENGRAM_MCP_RESOURCE_URL` is serve-only, so it is shape-checked at its single use site in
`runServe` and surfaced as a `usageErrorf` — the same treatment `resolveMCPPath`'s error already
gets.

</decisions>

<verified_orientation>

Facts below were confirmed by reading the live tree and by running a throwaway `net/http` probe
during planning. Do not re-derive them; do re-read the files you edit.

- `cmd/engram/mcproute.go` — `defaultMCPPath = "/mcp"`, `reservedMCPRoots = []string{"/ui",
  "/auth", "/engram.v1.EngramService"}`, `resolveMCPPath`, `mountMCPRoutes`, `rootHandler`.
  `resolveMCPPath` rejects a non-absolute path, a trailing slash (except the bare `/`), and any
  reserved root or sub-path of one.
- `cmd/engram/serve.go` — `mux := http.NewServeMux()` at line ~122; `resolvedMCPPath, err :=
  resolveMCPPath(cfg.Server.MCPPath)` at ~260; `handler = withAuth(handler, chain,
  cfg.OIDC.ResourceMetadata)` at ~268; `mountMCPRoutes(mux, handler, uiCfg.Enabled,
  resolvedMCPPath)` at ~274. `runServe` calls `config.Load(cmd.Flags())` at line 69 and does not
  call `Config.Validate()`.
- `runServe` already registers method-scoped patterns on this mux (`mux.HandleFunc("GET
  /auth/login", ...)`), so the `"GET <path>"` pattern form is established precedent here.
- `internal/config/config.go` — `ServerConfig{ListenAddr, MCPPath}` (koanf keys `listen_addr`,
  `mcp_path`); `OIDCConfig.ResourceMetadata` (koanf key `resource_metadata`).
- `internal/config/registry.go` — the `field{Key, Env, Legacy, Flag, Default}` registry. A
  brand-new key carries an empty `Legacy` (nothing retired to guard against); an env-only key
  carries an empty `Flag`.
- `internal/auth/bearer401_test.go` pins the go-sdk's own 401 body and `WWW-Authenticate` header
  byte-for-byte, driving `mcpauth.RequireBearerToken` directly with a hard-coded
  `resourceMetadataURL` constant. It does not call engram's `withAuth`, so D-02's default-fill
  cannot reach it. Leave that file alone.
- **Probe result (run this session, `net/http` + `httptest`, Go 1.26.3):** with `GET
  /.well-known/oauth-protected-resource` and a `/` catch-all both registered, `GET` on that path
  reaches the metadata handler and `POST` on that same path falls through to the `/` catch-all
  (no 405). With `/` owned by `rootHandler` instead, `POST` on that path is a 404 and
  `GET /.well-known/nope` is a 404. This is what makes the `"GET <path>"` pattern form safe: it
  preserves the `ENGRAM_MCP_PATH=/` legacy catch-all exactly.
- Docs pages that document comparable config: `docs-site/src/content/docs/guides/configure.md`
  (line 13 `ENGRAM_MCP_PATH` row; line ~141 `ENGRAM_OIDC_RESOURCE_METADATA` row),
  `docs-site/src/content/docs/reference/auth.md` (line ~29 serve-flag table),
  `docs-site/src/content/docs/guides/deploy.md` (line ~27 Helm values table).
- Helm: `charts/engram/values.yaml` has `memory.mcpPath` at line 20 and `memory.oidc.*` at ~127;
  `charts/engram/templates/_helpers.tpl` renders `ENGRAM_MCP_PATH` and the `ENGRAM_OIDC_*` vars
  inside the `engram.containerEnv` define. There is NO generic `extraEnv` escape hatch, so a new
  key is unreachable from the chart until it is wired. `Taskfile.yaml` `chart:validate` pins a
  sha256 of the `engram.containerEnv` block (`EXPECTED_CHECKSUM`, line ~201) and has been
  deliberately re-pinned four times before.
- Repo rule `m45p2b4bp7`: never assert third-party behaviour we do not own. No test in this plan
  may gate LiteLLM's or agentgateway's documented relay behaviour — tests verify engram's handler
  and engram's config only.

</verified_orientation>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: Serve the protected-resource document end-to-end</name>
  <files>internal/config/registry.go, internal/config/config.go, cmd/engram/wellknown.go, cmd/engram/wellknown_test.go, cmd/engram/serve.go</files>
  <behavior>
    Write these tests in cmd/engram/wellknown_test.go FIRST and watch them fail before implementing.
    All three are table-driven in the style of TestResolveMCPPath / TestMountMCPRoutes.

    TestProtectedResourceDocument — drives the handler through httptest and asserts the whole
    response, not fragments:
    - issuer configured, resource URL configured: status 200; Content-Type exactly application/json;
      unmarshalled body has resource equal to the configured URL, authorization_servers equal to a
      one-element slice holding the issuer, bearer_methods_supported equal to a one-element slice
      holding the literal header value, scopes_supported equal to a one-element slice holding the
      offline-access scope.
    - issuer empty (auth disabled): the authorization_servers key is ABSENT from the raw JSON
      object, not present-and-empty. Assert against a map[string]any unmarshal so absence is
      distinguishable from an empty array.
    - both well-known paths (host-only form and path-suffix form) produce byte-identical bodies for
      the same request origin.

    TestProtectedResourceResourceURLDerivation — table over {configured, mcpPath, host, xfProto,
    xfHost, tls} to the expected resource string:
    - configured value present and X-Forwarded-Host set to an attacker host: result is the
      configured value, unchanged. This is the security-critical row.
    - configured empty, plain Host, no forwarded headers, mcpPath /mcp: scheme http, the Host value,
      path /mcp appended.
    - configured empty, request over TLS: scheme https.
    - X-Forwarded-Proto exactly the https literal: scheme https. X-Forwarded-Proto set to a bogus
      value: falls back to the TLS-derived scheme, never echoed.
    - X-Forwarded-Host a bare host, and a bare host:port: honored.
    - X-Forwarded-Host containing a comma-separated list, a slash, an at sign, whitespace, or empty:
      each rejected, result falls back to the request Host.
    - mcpPath is the bare root: no path component is appended to the derived origin.

    TestProtectedResourceCacheControl — derived (no configured URL) responses carry a no-store
    Cache-Control value; configured responses carry no Cache-Control header at all.
  </behavior>
  <action>
    Add the config key. In internal/config/registry.go add one field row with Key "server.mcp_resource_url", Env "ENGRAM_MCP_RESOURCE_URL", empty Legacy (brand new, nothing retired), empty Flag (per D-03), empty Default — place it immediately after the existing server.mcp_path row. In internal/config/config.go add MCPResourceURL string with koanf tag mcp_resource_url to ServerConfig, and extend that struct's doc comment to name it as the public URL a client reaches the MCP endpoint on. Do not touch internal/config/validate.go (per D-04).

    Create cmd/engram/wellknown.go with the Apache-2.0 SPDX header (run task license:add if unsure; task license:check must stay green). Give it:

    A wellKnownPRMBase constant holding the RFC 9728 host-only path, and package-level constants or vars for the two fixed document values decided in D-01 — the single bearer method and the single supported scope. Do not read either from config.

    A protectedResourceMetadata struct with json tags resource, authorization_servers (with omitempty so an unconfigured issuer omits the key entirely rather than emitting an empty array), bearer_methods_supported, and scopes_supported.

    A protectedResourcePaths(mcpPath string) helper returning the host-only path and the RFC 9728 §3.1 path-suffix path. The suffix form is the host-only path concatenated with mcpPath. Return an empty suffix when mcpPath is the bare root, because concatenating it would produce a trailing-slash ServeMux subtree pattern and because the host-only form already IS the correct document location for a root-mounted resource.

    A validForwardedHost(v string) bool helper. Accept only a bare authority: non-empty, no comma (a comma means a multi-proxy append list whose leading element is the most client-controlled; reject the whole value and fall back to the trusted Host rather than trusting any element of it), no whitespace, no slash, no at sign, no colon-slash-slash, length at most 255, and url.Parse of a protocol-relative form of the value must round-trip its Host back unchanged. Document the fall-back-to-Host consequence in the comment.

    A resourceURLFor(configured, mcpPath string, r *http.Request) string. When configured is non-empty return it verbatim and consult NO request header — this is the T-526-01 mitigation and the test above pins it. Otherwise derive: scheme is the X-Forwarded-Proto value only when it equals exactly the http or https literal, else https when r.TLS is non-nil, else http; host is the X-Forwarded-Host value only when validForwardedHost accepts it, else r.Host; the path component is mcpPath unless mcpPath is the bare root, in which case none.

    A resolveMCPResourceURL(raw string) (string, error) shape check (D-04): trim; empty is a valid no-op returning empty with no error; otherwise url.Parse must succeed, the scheme must be http or https, the host must be non-empty, and any query or fragment is an error since neither is meaningful in a resource identifier. Mirror the ENGRAM_OPENAI_EMBEDDINGS_URL message idiom already in internal/config/validate.go: name the env var, quote the offending value, state the constraint.

    A protectedResourceHandler(configuredResourceURL, issuer, mcpPath string) http.Handler. It sets Content-Type to application/json; when configuredResourceURL is empty it also sets Cache-Control to no-store, so a shared cache in front of engram can never retain a document whose resource was derived from a spoofable header (T-526-01); when configuredResourceURL is non-empty it sets no Cache-Control at all. It builds the struct — resource from resourceURLFor, authorization_servers as a one-element slice when issuer is non-empty and nil otherwise, the two D-01 constants — and encodes it. There is no bearer gate, no session lookup, and no store or embedder call anywhere in this handler.

    A mountWellKnownRoutes(mux *http.ServeMux, h http.Handler, mcpPath string) that registers h under the method-scoped GET pattern for the host-only path, and under the same method-scoped form for the suffix path when protectedResourcePaths returned one. Comment WHY the pattern is method-scoped: a bare path pattern would steal POST from the ENGRAM_MCP_PATH=/ legacy root catch-all, whereas the GET-scoped form lets a POST fall through to it (verified behaviour, see the probe note in verified_orientation).

    Wire it in cmd/engram/serve.go. Immediately after the existing resolveMCPPath block, call resolveMCPResourceURL on cfg.Server.MCPResourceURL; on error slog.Error it and return usageErrorf wrapping it, exactly like the adjacent invalid-MCP-path branch (bucket 1, a malformed configured value). Then, just before the existing mountMCPRoutes call, call mountWellKnownRoutes with a handler built from the resolved resource URL, cfg.OIDC.Issuer, and resolvedMCPPath. Mount it directly on mux, NOT through withAuth, accessLog, or otelhttp — matching how the /auth and /ui mounts already sit bare on this mux. Note in a comment that the whole-mux CrossOriginProtection wrapper still covers it and passes safe-method GET untouched. Extend the existing MCP-transport-mounted slog.Info, or add one beside it, reporting whether the resource URL is configured or derived so an operator can see which mode is live.
  </action>
  <verify>
    <automated>cd /Volumes/Code/orca/engram/fix-gh-issue-526 &amp;&amp; go build ./... &amp;&amp; go test ./cmd/engram -run 'TestProtectedResourceDocument|TestProtectedResourceResourceURLDerivation|TestProtectedResourceCacheControl' -count=1 -v 2>&amp;1 | rg -o '^--- PASS: (TestProtectedResourceDocument|TestProtectedResourceResourceURLDerivation|TestProtectedResourceCacheControl)\b' | sort -u | wc -l | rg -q '^ *3$' &amp;&amp; echo TASK1_GATE_OK</automated>
  </verify>
  <done>
    The three named tests each exist and each PASS (the gate counts three distinct top-level PASS
    lines, so a `-run` pattern that silently matches nothing cannot report green — the trap recorded
    in STATE.md as `bsbsvn4hbc`). `go build ./...` is clean. A GET of either well-known path returns
    200 JSON; with a configured resource URL, no request header can alter the served `resource`.
  </done>
  <reversibility rating="costly">
    `ENGRAM_MCP_RESOURCE_URL` is a permanent `ENGRAM_` surface once released — withdrawable only
    through the soft-deprecation path the `migrate-set-owner` alias established. The handler and
    routes themselves are freely reversible.
  </reversibility>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Reserve /.well-known, and default the 401 challenge to a path we serve</name>
  <files>cmd/engram/mcproute.go, cmd/engram/mcproute_test.go, cmd/engram/wellknown.go, cmd/engram/wellknown_test.go, cmd/engram/serve.go, internal/config/mcp_resource_test.go</files>
  <behavior>
    New cases appended to the existing TestResolveMCPPath table in cmd/engram/mcproute_test.go:
    - the bare well-known root: an error.
    - the host-only protected-resource path: an error (it is under the reserved root).
    - a lookalike that merely shares a prefix but is not the reserved root nor a sub-path of it:
      allowed, mirroring the existing non-reserved-lookalike row for /uimcp.

    New TestMountWellKnownRoutes in cmd/engram/wellknown_test.go, mux-level, mirroring
    TestMountMCPRoutes' shape — build a real http.ServeMux, mount an MCP stub handler via
    mountMCPRoutes and the metadata handler via mountWellKnownRoutes, then drive it:
    - non-escape-hatch mode (mcpPath /mcp): GET of each well-known path is 200 and reaches the
      metadata handler; POST of the host-only path is 404; GET of an unrelated path under the
      well-known root is 404; GET and POST of /mcp still reach the MCP stub.
    - escape-hatch mode (mcpPath the bare root): GET of the host-only path reaches the metadata
      handler; POST of that SAME path reaches the MCP stub, proving the legacy root catch-all was
      not narrowed; only the host-only path is registered, since the suffix form is skipped.

    New TestResolveResourceMetadataURL in cmd/engram/wellknown_test.go, table over {configured,
    mcpResourceURL, mcpPath} to the expected challenge URL:
    - configured non-empty: returned verbatim, even when mcpResourceURL is also set.
    - both empty: empty result — today's shipped behaviour, unchanged.
    - configured empty, mcpResourceURL set, mcpPath /mcp: the mcpResourceURL origin plus the §3.1
      path-suffix form. Assert the full string.
    - configured empty, mcpResourceURL set, mcpPath the bare root: the origin plus the host-only
      form, with no trailing slash.
    - mcpResourceURL carrying its own path, query, or trailing slash: only its scheme and host are
      used; the result never doubles a path segment.

    New internal/config/mcp_resource_test.go, in the style of internal/config/connect_test.go:
    - the registry contains exactly one row for the new key, its Legacy field is empty (brand new),
      and its Flag field is empty (per D-03).
    - setting the env var and calling Load lands the value on cfg.Server.MCPResourceURL.
    - unset env leaves that field empty.
  </behavior>
  <action>
    In cmd/engram/mcproute.go add the bare well-known root to reservedMCPRoots and extend that var's doc comment: this root is now owned by the protected-resource metadata mounts in wellknown.go, and letting ENGRAM_MCP_PATH land on or under it would either panic http.ServeMux on a conflicting registration or let the MCP transport shadow the metadata document (threat T-526-03). Keep the existing "keep in sync with the explicit mounts in runServe" instruction accurate by naming mountWellKnownRoutes alongside it. resolveMCPPath itself needs no change — its existing loop already rejects a reserved root and any sub-path of one.

    In cmd/engram/wellknown.go add resolveResourceMetadataURL(configured, mcpResourceURL, mcpPath string) string implementing D-02. Explicitly configured wins and is returned untouched. Empty mcpResourceURL yields empty, preserving exactly what ships today. Otherwise parse mcpResourceURL, keep only its scheme and host, and append whichever of the two protectedResourcePaths forms applies for mcpPath — the suffix form for a non-root MCP path, the host-only form for the root. A parse failure yields empty rather than a malformed challenge URL; note in the comment that resolveMCPResourceURL has already rejected malformed input at startup, so this arm is a belt-and-braces fallback and not a live path. Document the D-02 rationale on this function, including why the per-request derivation resourceURLFor does is unavailable here (withAuth takes one static string at construction time).

    In cmd/engram/serve.go change the withAuth call to pass the result of resolveResourceMetadataURL over cfg.OIDC.ResourceMetadata, the already-shape-checked resource URL from Task 1, and resolvedMCPPath — replacing the bare cfg.OIDC.ResourceMetadata argument. Do not change withAuth's own signature or body. Leave internal/auth/bearer401_test.go untouched: it drives the go-sdk middleware directly with its own constant and does not reach engram's withAuth.
  </action>
  <verify>
    <automated>cd /Volumes/Code/orca/engram/fix-gh-issue-526 &amp;&amp; go test ./cmd/engram ./internal/config ./internal/auth -run 'TestResolveMCPPath|TestMountWellKnownRoutes|TestResolveResourceMetadataURL|TestMCPResourceURL|TestMCP401BodyByteIdentical' -count=1 -v 2>&amp;1 | rg -o '^--- PASS: (TestResolveMCPPath|TestMountWellKnownRoutes|TestResolveResourceMetadataURL|TestMCPResourceURL|TestMCP401BodyByteIdentical)\b' | sort -u | wc -l | rg -q '^ *5$' &amp;&amp; echo TASK2_GATE_OK</automated>
  </verify>
  <done>
    All five named tests exist and PASS as five distinct top-level PASS lines, including the
    untouched `TestMCP401BodyByteIdentical`, proving D-02 did not disturb the pinned 401 challenge.
    `ENGRAM_MCP_PATH` can no longer be set to the well-known root or a path under it.
  </done>
</task>

<task type="auto">
  <name>Task 3: Document the new key and wire it through the Helm chart</name>
  <files>docs-site/src/content/docs/guides/configure.md, docs-site/src/content/docs/reference/auth.md, docs-site/src/content/docs/guides/deploy.md, charts/engram/values.yaml, charts/engram/templates/_helpers.tpl, Taskfile.yaml</files>
  <precondition>`helm` is on PATH — `task chart:validate` shells out to `helm lint` and `helm template`, and this task's gate runs it.</precondition>
  <action>
    In docs-site/src/content/docs/guides/configure.md add a row for the new env var to the same table that carries the ENGRAM_MCP_PATH row, formatted identically (env var, flag column showing that there is none per D-03, default empty, description). The description states: the public URL clients reach the MCP endpoint on; when set it becomes the resource field of the served protected-resource document verbatim and no request header can override it; when unset the value is derived per request from the forwarded/Host headers and the MCP path. In the OIDC section that carries the ENGRAM_OIDC_RESOURCE_METADATA row, amend that row's description to record D-02: when it is left empty and the new key is set, the challenge URL now defaults to the path engram itself serves.

    In docs-site/src/content/docs/reference/auth.md add a short subsection under the enabling-authentication area covering the protected-resource metadata document: both paths it is served at, that it is unauthenticated by design and returns application/json, the four fields and where each comes from (naming the two D-01 constants as constants), that authorization_servers is omitted entirely when no issuer is configured, and the derived-versus-configured resource rule including the no-store caching consequence. Add the new env var to that page's serve-flag/env table, marking it env-only. Keep the page's existing heading depth and table formatting.

    In docs-site/src/content/docs/guides/deploy.md add a `memory.mcpResourceUrl` row to the values-to-env mapping table, beside the existing memory.mcpPath row.

    In charts/engram/values.yaml add the key mcpResourceUrl under memory, defaulting to the empty string, immediately after mcpPath, with a comment naming the env var it maps to and stating that empty means engram derives the value per request. The key name is fixed at memory.mcpResourceUrl — this task's gate sets it by that exact path. In charts/engram/templates/_helpers.tpl add a with-guarded row inside the engram.containerEnv define rendering the env var from that value, placed immediately after the existing ENGRAM_MCP_PATH row and matching its exact one-line style so the default render still omits the var entirely.

    Re-pin the chart drift checksum. Run task chart:validate; it fails and prints the actual sha256. Confirm first that the failure is only the checksum (the CronJob and secretKeyRef assertions above it must still pass) and that a default helm template render omits the new var. Then replace EXPECTED_CHECKSUM in Taskfile.yaml with the printed value and append one sentence to the re-pin comment block already there, in the same form as the four prior entries: dated today, naming this quick task and the guarded row that was added. Re-run task chart:validate until it prints OK.

    Do not add an entry to docs-site guides/upgrade.md: nothing here is breaking and no operator action is required on upgrade — two new routes appear, and the D-02 default only engages for a deployment that opts in by setting the new key. Do not add an SPDX header to any docs-site or .planning file.
  </action>
  <verify>
    <automated>cd /Volumes/Code/orca/engram/fix-gh-issue-526 &amp;&amp; test "$(rg -l 'ENGRAM_MCP_RESOURCE_URL' docs-site/src/content/docs/guides/configure.md docs-site/src/content/docs/reference/auth.md docs-site/src/content/docs/guides/deploy.md charts/engram/templates/_helpers.tpl | wc -l | tr -d ' ')" = 4 &amp;&amp; task chart:validate &amp;&amp; ! helm template charts/engram | rg -q 'ENGRAM_MCP_RESOURCE_URL' &amp;&amp; helm template charts/engram --set memory.mcpResourceUrl=https://example.test/mcp | rg -q 'ENGRAM_MCP_RESOURCE_URL' &amp;&amp; task license:check &amp;&amp; task lint &amp;&amp; echo TASK3_GATE_OK</automated>
  </verify>
  <done>
    All four files name the new env var; `task chart:validate` prints OK against a freshly re-pinned
    checksum; a default `helm template` render still omits the var (it appears only when the value
    is set); `task license:check` and `task lint` are clean.
  </done>
</task>

</tasks>

<threat_model>

ASVS level 1, blocking threshold: high.

## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Internet/gateway → `GET /.well-known/oauth-protected-resource*` | Unauthenticated by design (RFC 9728 requires public reachability). No bearer gate, no session, no store access. |
| Reverse proxy → engram | `X-Forwarded-Proto` / `X-Forwarded-Host` / `Host` cross here. Engram cannot distinguish a header set by a trusted proxy from one an untrusted client sent through it. |
| Operator config → route table | `ENGRAM_MCP_PATH` selects where the MCP transport mounts on the same mux as the new metadata routes. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-526-01 | Spoofing / Tampering | `resourceURLFor` (`cmd/engram/wellknown.go`) — derived `resource` field | medium | mitigate | Three layers, all test-pinned. (1) A configured `ENGRAM_MCP_RESOURCE_URL` short-circuits header consultation entirely — the configured-wins-over-spoofed-header row in `TestProtectedResourceResourceURLDerivation` is the pin. (2) `X-Forwarded-Proto` is honored only when it equals exactly `http` or `https`; any other value falls back to TLS-derived scheme and is never echoed into the document. `X-Forwarded-Host` is honored only when `validForwardedHost` accepts a bare authority — comma-lists, slashes, at-signs, whitespace, and over-length values all fall back to the trusted `Host`. (3) Derived responses carry `Cache-Control: no-store`, so no shared cache in front of engram retains a spoofed document for a later victim. Residual risk after mitigation: a client that fetches through an attacker-supplied `X-Forwarded-Host` in a deployment with no configured resource URL sees an attacker origin in `resource` — accepted at L1, because that client already controls its own request and gains nothing it could not self-assert. |
| T-526-02 | Information disclosure | protected-resource document body | low | accept | The document is required to be publicly reachable by RFC 9728, and it discloses only `ENGRAM_OIDC_ISSUER`, the MCP path, and two constants. The issuer is already disclosed by the existing 401 `WWW-Authenticate` challenge and by every OAuth client flow. The handler emits no client ID, no client secret, no audience, no service-auth config, and no internal hostname beyond the request's own origin. `authorization_servers` is omitted entirely when no issuer is configured, so an auth-disabled deployment does not advertise a nonexistent authorization server. |
| T-526-03 | Tampering / Elevation of privilege | `reservedMCPRoots` + `resolveMCPPath` (`cmd/engram/mcproute.go`) | high | mitigate | Without reservation, `ENGRAM_MCP_PATH=/.well-known/oauth-protected-resource` would either panic `http.ServeMux` on a conflicting registration at startup or let the bearer-gated MCP transport shadow the document clients must read unauthenticated to discover how to authenticate. Task 2 adds `/.well-known` to `reservedMCPRoots`; `resolveMCPPath`'s existing loop then rejects the root and every sub-path of it at startup with a clear operator-facing error. Pinned by three new rows in `TestResolveMCPPath`, including a non-reserved-lookalike row proving the check is not an over-broad prefix match. Meets the `high` blocking threshold. |
| T-526-04 | Denial of service | unauthenticated metadata handler | low | accept | The handler performs no I/O, no store or embedder call, no crypto, and no unbounded allocation — it encodes a fixed, small struct. It is strictly cheaper per request than the unauthenticated 401 path engram already exposes on the same listener, so it widens no DoS surface. Rate limiting stays a gateway concern, unchanged by this plan. |
| T-526-05 | Repudiation | route mounting | low | accept | The metadata routes are deliberately mounted bare on the mux, outside `accessLog` and `otelhttp`, matching the existing `/auth/*` and `/ui/` mounts. The request carries no identity to log and no authorization decision to attribute, so there is nothing to repudiate. A startup `slog.Info` records whether the resource URL is configured or derived, which is the operator-visible fact that matters. |
| T-526-SC | Tampering | npm/pip/cargo/go dependencies | high | accept | No package-manager install task exists in this plan. The implementation uses only `encoding/json`, `net/http`, and `net/url` from the standard library, plus the already-vendored `internal/config`. `go.mod` is not modified, so the package-legitimacy gate has no packages to audit and no `[ASSUMED]`/`[SUS]` checkpoint is required. |

</threat_model>

<verification>

- `task` (lint + test) is clean from the worktree root.
- `task license:check` is clean — the two new Go files carry the Apache-2.0 SPDX header; no
  docs-site or `.planning` file gained one.
- `task chart:validate` prints OK; a default `helm template charts/engram` render omits
  `ENGRAM_MCP_RESOURCE_URL`, and a render with the value set emits it.
- `cmd/engram/testdata/help.golden` and `catalog.golden` are unchanged in `git diff` (D-03 — no new
  flag, no usage-string edit).
- `internal/auth/bearer401_test.go` is unchanged in `git diff`.
- No test in this plan asserts LiteLLM or agentgateway behaviour (repo rule `m45p2b4bp7`).

</verification>

<success_criteria>

- Both `/.well-known/oauth-protected-resource` and `/.well-known/oauth-protected-resource<mcp-path>`
  return 200 with `Content-Type: application/json` and the four-field RFC 9728 document,
  unauthenticated. Every other path keeps today's behavior.
- `authorization_servers[0]` equals `ENGRAM_OIDC_ISSUER`; the key is absent when no issuer is set.
- `resource` equals `ENGRAM_MCP_RESOURCE_URL` verbatim when configured, and is header-derived only
  when it is not — with the spoofing mitigations of T-526-01 in place and test-pinned.
- `ENGRAM_OIDC_RESOURCE_METADATA` defaults to the served path when unset and
  `ENGRAM_MCP_RESOURCE_URL` is set (D-02); explicit config still wins; both-unset is unchanged.
- `ENGRAM_MCP_PATH` cannot collide with or shadow `/.well-known` (T-526-03).
- `ENGRAM_MCP_PATH=/` still routes `POST /.well-known/oauth-protected-resource` to the MCP
  transport — the legacy escape hatch is not narrowed.
- `scopes_supported` is the constant decided in D-01, matching the retired gateway-synthesised
  document.

</success_criteria>

<output>
Create `.planning/quick/260909-ofg-fix-gh-issue-526-serve-rfc-9728-protecte/260909-ofg-SUMMARY.md` when done.

Commit in coherent groups with Conventional Commits and an explicit pathspec (rule `n6m4as49mr` —
this is a shared working directory, never `git commit -a`). Suggested split: one `feat(serve):`
commit per task. Branch is `orca-seanb4t/fix-gh-issue-526`; `main` is protected, so land via PR.
Reference `Closes #526` in the PR body.
</output>
