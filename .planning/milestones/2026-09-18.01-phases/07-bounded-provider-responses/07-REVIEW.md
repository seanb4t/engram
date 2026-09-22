---
phase: 07-bounded-provider-responses
reviewed: 2026-09-21T00:00:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - internal/httpdrain/httpdrain.go
  - internal/httpdrain/httpdrain_test.go
  - internal/embed/embed.go
  - internal/embed/embed_test.go
  - internal/summarize/summarize.go
  - internal/summarize/summarize_test.go
  - internal/testhttp/trickle.go
  - internal/testhttp/reuse.go
  - internal/config/config.go
  - internal/config/registry.go
  - internal/config/validate.go
  - internal/config/config_test.go
  - internal/config/validate_test.go
  - internal/config/providerbounds_test.go
  - internal/server/tools.go
  - internal/server/providerbounds_test.go
  - internal/store/redevidence_harness_test.go
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/upgrade.md
findings:
  critical: 1
  warning: 2
  info: 1
  total: 4
status: issues_found
---

# Phase 07: Code Review Report

**Reviewed:** 2026-09-21
**Depth:** standard
**Files Reviewed:** 19 (`internal/embed/embed_test.go` and `internal/summarize/summarize_test.go` reviewed for the phase's new/changed tests, not their full pre-existing suites)
**Status:** issues_found

## Summary

The core mechanism (`internal/httpdrain.Drain`: `time.AfterFunc` closing the body,
`io.LimitReader` bounding bytes, no goroutine outliving the call) is correctly
implemented and matches the pinned `net/http` mechanism from D-01. Zero-value
handling for the four drain knobs traces correctly end to end — env var →
registry default → `Config` struct field → `ParseNonNegativeIntCap` →
`tools.go` helper → `WithDrainBytes(0)`/`WithDrainTimeout(0)` → `Drain` — at
every hop `"0"` survives as `0`, never silently replaced by a default. The
embed/summarize "direct twin" invariant holds: the two clients are
structurally identical in every way I diffed them, including in the one bug
found below (both carry it identically). The byte-axis regression tests
correctly work around `net/http`'s own 256 KiB post-close safety-net drain by
serving an oversized body with an explicit `Content-Length` header, so they
prove `httpdrain`'s own bound rather than passing for the wrong reason.

One finding is a genuine BLOCKER: the request-timeout ceiling clamp in both
`embed.Client.New` and `summarize.Client.New` clamps an explicit **positive**
`WithTimeout` value down to the ceiling when it exceeds it — directly
contradicting the phase's own locked decision D-07 ("An explicit positive d is
honored uncapped, however large... Rejected: clamping every value"), and
self-contradicting `WithTimeout`'s own doc comment in both files plus
`configure.md` and `upgrade.md`, all of which promise "honored UNCAPPED,
however large." The regression test suite encodes the *implemented* (wrong)
behavior as the expected one, so this will not surface in CI.

## Critical Issues

### CR-01: The timeout ceiling clamps explicit positive values, contradicting locked decision D-07

**File:** `internal/embed/embed.go:252-254`, `internal/summarize/summarize.go:206-208`
**Issue:**

```go
// D-07/D-09: applied here, after the options loop and never inside
// WithTimeout itself, so last-writer-wins option ordering between
// WithTimeout and WithMaxTimeout is preserved regardless of which was
// supplied first. A non-positive or over-ceiling timeout resolves to the
// ceiling; an explicit positive timeout at or below it is honored
// exactly as given.
if c.http.Timeout <= 0 || c.http.Timeout > c.maxTimeout {
    c.http.Timeout = c.maxTimeout
}
```

`07-CONTEXT.md`'s D-07 (a locked decision) is unambiguous:

> An explicit positive `d` is honored **uncapped**, however large: the
> operator named a number, so respect it. **Rejected: clamping every value**,
> which would override a deliberately-chosen longer duration.

The shipped code does exactly the rejected thing: `c.http.Timeout > c.maxTimeout`
clamps *any* positive value above the ceiling, not only non-positive ones. This
is proven by the phase's own test, which asserts the wrong behavior as correct:

```go
{
    name: "positive duration above the default ceiling is clamped",
    opts: []Option{WithTimeout(20 * time.Minute)},
    want: defaultMaxTimeout,
},
```
(`internal/embed/embed_test.go` `TestEmbedTimeoutCeiling`; identical subtest in
`internal/summarize/summarize_test.go` `TestSummarizeTimeoutCeiling`.)

The same code's own doc comment contradicts itself in the same file:
`WithTimeout`'s doc comment (embed.go:138-142, summarize.go:120-124) states
*"An explicit positive d is honored UNCAPPED, however large — the operator
named a number, so it is respected"* — directly above the clamp that caps it.
`docs-site/src/content/docs/guides/configure.md` and `upgrade.md` both repeat
the same "uncapped, however large" promise to operators.

**Real-world impact:** the exact scenario `07-CONTEXT.md`'s own `<specifics>`
section names — *"WithTimeout(0) keeps its purpose (no request deadline for a
slow self-hosted gateway) right up to the ceiling"* — is broken for an operator
who instead sets an explicit long duration (e.g. `ENGRAM_EMBED_TIMEOUT=30m` for
a slow local model) without separately raising `ENGRAM_EMBED_MAX_TIMEOUT`
above its 10m default. Their explicitly-configured value is silently
downgraded to 10m, with no startup validation error (`Config.Validate` does
not cross-check `Timeout` against `MaxTimeout`) and no runtime log line.

**Fix:** drop the `|| c.http.Timeout > c.maxTimeout` clause so only a
non-positive value resolves to the ceiling, matching D-07 and every doc
comment:

```go
if c.http.Timeout <= 0 {
    c.http.Timeout = c.maxTimeout
}
```

This requires also correcting `TestEmbedTimeoutCeiling`'s and
`TestSummarizeTimeoutCeiling`'s "positive duration above the default ceiling"
subtests (currently asserting `want: defaultMaxTimeout`; they should assert
the explicit duration is honored uncapped instead), and the two red-evidence
patches/target tests that currently exercise this clamp
(`07-03-timeout-zero-means-unbounded.patch` targets `TestSummarizeTimeoutCeiling`,
which would need its own "above ceiling honored uncapped" case reverted by a
parallel embed-side patch to keep both axes of D-07 covered).

## Warnings

### WR-01: `httpdrain.Drain`'s doc comment doesn't state that the body is left open on the ordinary completion path

**File:** `internal/httpdrain/httpdrain.go:21-53`
**Issue:** `Drain` only calls `body.Close()` on two paths: the non-positive-bound
skip (line 47) and the time-bound timer firing (line 50). On the ordinary
path — the byte bound is reached, or the body reaches EOF before either bound
fires — `Drain` returns *without ever closing `body`*. `httpdrain_test.go`'s
`TestDrainStopsAtByteBound` explicitly pins this: `if body.isClosed() { t.Fatal(...) }`.

This is safe today only because both of the package's two current callers
(`internal/embed/embed.go:388`, `internal/summarize/summarize.go:297`) already
carry their own `defer func() { _ = resp.Body.Close() }()` immediately after
`c.http.Do(req)` succeeds — a precondition that lives entirely in the callers,
not documented on `Drain` itself. `Drain`'s doc comment describes the
close-on-timeout and close-on-skip paths in detail but never states "the
caller remains responsible for closing `body` when neither bound triggers
Drain's own Close" — an easy trap for a future third caller (this is an
exported function in a package whose own doc comment says it exists precisely
so a "single shared mechanism cannot drift" across future callers) who assumes
a function named `Drain` always disposes of the body.

**Fix:** add one sentence to `Drain`'s doc comment, e.g.:

```go
// Drain does not itself guarantee body is closed when neither bound is
// exceeded — the byte-bound and natural-EOF paths return without an
// explicit Close. Callers must retain their own body.Close() (typically via
// defer immediately after a successful response), exactly as
// internal/embed and internal/summarize already do.
```

### WR-02: `Config.Validate` never cross-checks a per-request `Timeout` against its own `MaxTimeout`

**File:** `internal/config/validate.go:65-110`, `:250-280`
**Issue:** `ENGRAM_EMBED_TIMEOUT`/`ENGRAM_SUMMARY_TIMEOUT` and
`ENGRAM_EMBED_MAX_TIMEOUT`/`ENGRAM_SUMMARY_MAX_TIMEOUT` are validated entirely
independently — nothing rejects (or even warns on) a startup configuration
where `Timeout > MaxTimeout`. Independent of CR-01 (which currently makes this
combination silently downgrade the request timeout), even after CR-01 is
fixed to honor positive values uncapped, an operator configuring
`ENGRAM_EMBED_TIMEOUT=30m` with the default `ENGRAM_EMBED_MAX_TIMEOUT=10m`
still ends up with a `MaxTimeout` that can never be reached by any `d<=0`
resolution on that lane, which is confusing but harmless; today, before the
CR-01 fix, that same combination is where the bug bites hardest and is
invisible at startup.
**Fix:** not a blocker, but consider a startup warning (not a validation
error — both remain independently valid values) when `Timeout > MaxTimeout`
on either lane, so an operator sees the interaction rather than discovering it
via unexpectedly early request timeouts.

## Info

### IN-01: `httpdrain.Drain`'s "no goroutine outlives this call" claim is not exactly true on the timer-fired path

**File:** `internal/httpdrain/httpdrain.go:29-32`
**Issue:** The doc comment states *"The timer is always stopped before Drain
returns, so no timer and no goroutine outlives this call: there is no
background goroutine here at all, only a stdlib timer bound to the call's own
lifetime."* This is true for the timer itself, but `time.AfterFunc`'s
callback runs in its own goroutine once fired; if the timer fires at (or just
before) the moment `io.Copy` finishes normally, `t.Stop()` returns `false`
(per `time.Timer.Stop`'s documented semantics for an already-fired timer) and
does **not** wait for that goroutine to finish running `body.Close()`. In
practice this goroutine's body is a single `Close()` call and exits within
nanoseconds, so this has no observable impact — but the doc comment's literal
claim ("no goroutine outlives this call") is not strictly accurate for that
narrow race window.
**Fix:** none required functionally; consider softening the doc comment to
"no goroutine outlives this call in any way that matters" or similar, since
the current wording invites a reader to assume a stronger guarantee than Go's
`time.AfterFunc`/`Timer.Stop` actually provide.

---

_Reviewed: 2026-09-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
