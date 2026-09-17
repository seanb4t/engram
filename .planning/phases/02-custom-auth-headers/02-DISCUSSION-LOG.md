# Phase 2: Custom Auth Headers - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-13
**Phase:** 2-custom-auth-headers
**Areas discussed:** CLI shape, Header value form, Multiplicity & env var naming, Codex boundary

---

## CLI shape

**User clarification before the first option set was accepted:** "We need to be careful with header,
this isn't preclusive of any other header — it's an additional/separate header with a potentially
sensitive value." The initial question (which framed the header as renaming `Authorization` on the
bearer mode) was withdrawn and reformulated.

| Option | Description | Selected |
|--------|-------------|----------|
| `--header NAME=ENVVAR`, repeatable, valid with every `--auth` mode | Adds a header alongside whatever auth renders; `Authorization` rejected | ✓ |
| `--header NAME=ENVVAR`, single-use only | Simpler drift model; one extra header | |
| `--header 'NAME: ${ENVVAR}'` raw pass-through | Claude Code syntax re-rendered per runtime | |

**User's choice:** repeatable `--header NAME=ENVVAR` orthogonal to `--auth`

| Option | Description | Selected |
|--------|-------------|----------|
| Reject any case-variant of `Authorization` | One owner per header; usage error points at `--auth bearer` | ✓ |
| Allow it and override the auth header | Power-user escape hatch | |
| Allow only when `--auth none` | No collision possible | |

**User's choice:** Reject any case-variant

| Option | Description | Selected |
|--------|-------------|----------|
| Strict: RFC 7230 token + POSIX env identifier | Duplicates and literal-looking values rejected | ✓ |
| Lenient: split on `=` | Relies on runtime CLI | |

**User's choice:** Strict
**Notes:** Withdrawn first question recorded above; research's "bearer generalized" framing superseded.

---

## Header value form

| Option | Description | Selected |
|--------|-------------|----------|
| Bare reference `${VAR}` / `{env:VAR}` | Scheme lives in the env var's value | ✓ |
| Always prefix `Bearer ` | Mirrors bearer rendering | |
| Optional scheme in the flag | `NAME=[SCHEME ]ENVVAR` | |

**User's choice:** Bare reference

| Option | Description | Selected |
|--------|-------------|----------|
| Existing `headers` map with `${ENVVAR}` + prose note | Reuses `generic.go` Headers | ✓ |
| Separate `env_headers` map | Codex-style name→env map | |

**User's choice:** `headers` map + note
**Notes:** None.

---

## Multiplicity & env var naming

| Option | Description | Selected |
|--------|-------------|----------|
| Flag only in this phase | No ENGRAM_ counterpart | |
| Add `ENGRAM_HEADERS` (comma-separated NAME=ENVVAR) | Consistent with url/auth env defaults | ✓ |

**User's choice:** Add `ENGRAM_HEADERS`

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror `--runtime`: StringSliceVar + `os.Getenv`, no registry row | Existing precedent, documented in registry.go | ✓ |
| Registry row + slice-aware overlay | Wider blast radius | |

**User's choice:** Mirror `--runtime`

| Option | Description | Selected |
|--------|-------------|----------|
| Sort by name after the auth header | Deterministic | ✓ |
| Preserve user order | Faithful but drift-prone | |

**User's choice:** Sort by name after the auth header
**Notes:** Each header names its own env var; `ENGRAM_TOKEN` stays bearer-only.

---

## Codex boundary

| Option | Description | Selected |
|--------|-------------|----------|
| Codex row `failed` with capability-gap reason; others proceed | Mirrors oauth-client on opencode | ✓ |
| Usage error at the CLI when codex is targeted | Blocks other runtimes on detection-driven targeting | |
| Silently skip codex | Rejected by REQ-header-codex-declined | |

**User's choice:** Row `failed`, others proceed

| Option | Description | Selected |
|--------|-------------|----------|
| New `ErrHeaderUnsupported` sentinel | Headers orthogonal to auth mode | ✓ |
| Reuse `ErrAuthModeUnsupported` | One sentinel | |

**User's choice:** New `ErrHeaderUnsupported`
**Notes:** None.

---

## Claude's Discretion

- `Options.Headers` field shape; validation location; `--output json` field shape; setupgen 5th Case
  and prose wording; docs placement; how opencode `--header` repeatability is live-verified without
  touching the operator's config.

## Deferred Ideas

- `ENGRAM_HEADERS` as a first-class registry row (slice-aware overlay).
- Pointing the Codex failed-row reason at the manual `env_http_headers` TOML workaround.
