<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Contributing to engram

Full contributor documentation lives on the docs site: codebase architecture,
conventions, and code style at
**<https://engram.seanb4t.dev/contributing/architecture/>**, and the
release process at
**<https://engram.seanb4t.dev/contributing/releasing/>**.

Architecture Decision Records are under [`docs/adr/`](./docs/adr/).

## Issue-first workflow

Every change starts as an issue, and every PR links an **approved** one.

1. **Open an issue** using the right template — Bug report, Feature request,
   Enhancement, or Chore. Bug/chore land in `needs-triage`; feature/enhancement
   land in `needs-review`.
2. **Wait for approval.** A maintainer moves the issue to `confirmed-bug`,
   `approved-feature`, or `approved-enhancement`. That label is the green light
   to implement.
3. **Open a typed PR** against the matching template — `?template=fix.md`,
   `?template=feature.md`, or `?template=enhancement.md` — and link the issue
   with `Fixes #NNN` (bug) or `Closes #NNN` (feature/enhancement).

A PR with no linked issue, or whose issue lacks the matching approval label, is a
**gate violation** and may be closed until an issue is approved first. Keep one
concern per PR, and give it a Conventional Commit title (`type(scope): …`) — the
title is validated in CI.

## Testing

`task` runs lint + tests. `task test:go` is the Go suite; `task test:e2e` runs
only the binary-level tier described below.

### Tiers

| Tier | Where | What it covers |
|------|-------|----------------|
| Unit | alongside the code | Pure logic — validation, URL joins, filter-condition builders |
| Integration | `internal/store`, `internal/server` | Handlers and the store against a **real Qdrant** (testcontainers) |
| Binary (e2e) | `internal/e2e` | Builds the real binary, runs `engram serve` as a subprocess, drives it over the real MCP transport |

The binary tier exists for exactly one seam: the wiring **between** the other
tiers. Integration tests call `buildDepsFromEnv`, the tool handlers, and the
store directly — so a break in cobra/env → `runServe` → mux → MCP transport →
tool registration leaves the whole suite green while the shipped binary is
broken. Keep that tier small; assert tool *semantics* in `internal/server`,
where it is faster and more precise.

### Two traps worth knowing

**1. `ok` does not mean the tests ran.** Qdrant-backed tests `t.Skip` when no
Qdrant is reachable, and `go test` still prints `ok`. Verifying "feature X is
covered by TestY, and TestY passes" against a skipping run proves nothing.

```sh
ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1   # fail-closed; CI sets this
```

To confirm positively, add `-v` and count: `--- SKIP` should be zero.

**2. One shared Qdrant breaks isolation and produces a *false* failure.**
Pointing `ENGRAM_QDRANT_TEST_ADDR` at a single long-lived instance across
multiple packages accumulates records in a shared collection, and
`TestBackfillShortIDs` then fails with:

```text
short ids not globally unique: 313 distinct of 301
```

The tell is that **distinct exceeds total** — impossible for a real uniqueness
violation, which would show *fewer* distinct than total. It means the scan sees
records the test never inserted. Prefer the default testcontainers path
(ephemeral Qdrant per package); use `ENGRAM_QDRANT_TEST_ADDR` only as a
single-package fast path against a throwaway instance.

### Writing binary-tier tests

Build the subprocess environment with `childEnv`, never `os.Environ()`. A
developer shell commonly exports real `ENGRAM_*` values (`ENGRAM_EMBED_DIM`,
`ENGRAM_OPENAI_BASE_URL`, `ENGRAM_OPENAI_API_KEY` pointing at a live gateway).
Inheriting them makes the test assert against the machine rather than the code —
and can send a real API key to whatever endpoint the test configures. Use the
in-process `stubEmbedder` helper instead of a live provider.

## License

By contributing you agree your contributions are licensed under the
[Apache License 2.0](./LICENSE).
