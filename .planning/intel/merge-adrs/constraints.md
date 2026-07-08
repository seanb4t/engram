# Constraints — merge-adrs companion set

All 31 ingest docs are ADRs (decisions), not SPECs. No dedicated constraint documents
were ingested. The entries below capture normative technical constraints *implied* by
the locked decisions that downstream planning must honor. Each traces to its source ADR.

---

- **Rule summaries** must be single-line, ≤256 bytes, non-empty; malformed summaries are
  rejected, never normalized.
  - source: docs/adr/engram-m4s8-reject-malformed-rule-summaries-newline-oversize-cleared-nev.md

- **update_memory** must atomically reject a content change that leaves an existing
  client-authored summary unaddressed; an existing auto summary is auto-cleared.
  - source: docs/adr/engram-ddiw-reject-update-memory-content-change-unaddressed-client-summa.md

- **Memory summaries** are never generated on the write path; only at write time by the
  submitter or by the offline `engram summarize-missing` sweep.
  - source: docs/adr/engram-4y7p-explicit-first-memory-summary-offline-operator-auto-fill.md

- **Config.Validate** must check only the five universal data-plane fields (qdrant.addr,
  qdrant.collection, embed.model, embed.dim, openai.base_url); `server.listen_addr` is
  guarded serve-locally so admin commands do not trip on serve-only config.
  - source: docs/adr/engram-d24-validate-data-plane-fields-only-listen-addr-is-serve-local-g.md

- **config.Load** must remain assembly-only (no operator-value validation); validation lives
  in a separate pure `Config.Validate()`.
  - source: docs/adr/engram-wtw-keep-config-load-assembly-only-validate-via-separate-config.md

- **Span attributes** must carry only `engram.owner` (opaque OIDC sub); `actor`, email, and
  username are PII and must not reach trace backends (confined to structured log lines).
  - source: docs/adr/engram-wot-spans-carry-engram-owner-only-exclude-actor-and-email-as-pii.md

- **Sampler & metric export interval** configured only via OTel-standard env vars
  (`OTEL_TRACES_SAMPLER`, `OTEL_METRIC_EXPORT_INTERVAL`); no `MEM_*` equivalents.
  - source: docs/adr/engram-7qd-reuse-otel-standard-env-vars-sampler-and-export-interval-add.md

- **k8s resource attributes** injected via Helm Downward API into `OTEL_RESOURCE_ATTRIBUTES`;
  binary carries no k8s SDK detector.
  - source: docs/adr/engram-9tj-inject-k8s-resource-attributes-via-chart-downward-api-not-go.md

- **Session cookie** is httpOnly, SameSite, AES-GCM encrypted, no server-side store; v1 read
  lane seals only `{sub, expiry}` (no OIDC access/refresh token client-side).
  - source: docs/adr/engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md
  - source: docs/adr/engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md

- **User memory content rendering** must pass through marked → DOMPurify (tight allowlist +
  link-hardening) as the sole `{@html}` entry point (XSS boundary).
  - source: docs/adr/engram-3nas-render-user-memory-content-via-marked-dompurify-allowlist.md

- **SPA static handler** must serve `index.html` (200) for extensionless `/ui/*` routes, real
  files when present, 404 otherwise.
  - source: docs/adr/engram-vxk-spa-fallback-static-handler-serve-index-html-client-routes.md

- **Store public signatures** must remain stable; clock injection via `WithClock` functional
  option (unexported `now` field defaulting to `time.Now`).
  - source: docs/adr/engram-c0m-inject-store-clock-via-withclock-option-keep-public-signatur.md

- **Expired records** are soft-hidden at recall, never auto-destroyed; storage reclaimed only
  via explicit `engram prune-expired`.
  - source: docs/adr/engram-ufz-soft-hide-expired-records-at-recall-opt-prune-expired-storag.md

- **engram wordmark** ships as inlined outlined SVG paths — no webfont fetch / FOUT / network
  dependency.
  - source: docs/adr/engram-no3-ship-engram-wordmark-as-outlined-svg-paths-not-webfont.md

- **engram plugin** ships no bundled `.mcp.json`; MCP-server registration is done only via
  `/engram-setup` (`claude mcp add`, user scope).
  - source: docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md
