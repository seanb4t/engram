# Phase 1: Executor Correctness & Man Pages - Pattern Map

**Mapped:** 2026-09-13
**Files analyzed:** 8
**Analogs found:** 8 / 8

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|-----------------|---------------|
| `internal/setup/environment.go` (modify `osRun`) | utility (subprocess-exec seam) | request-response | itself (in-place edit) — same file, current code is the analog | exact |
| `internal/setup/environment_test.go` (new) | test | request-response | `internal/store/store_test.go:280-303` (`os.Args[0]` re-exec idiom) | role-match (test idiom, not same package) |
| `internal/setup/apply.go` (modify `runSeam`) | utility (seam wrapper) | request-response | itself (in-place edit) — `describeSeamError` (`apply.go:153-155`) is the sibling pattern for wrapping | exact |
| `internal/setup/apply_test.go` (extend) | test | request-response | `internal/setup/apply_test.go:193-208` (`probe-seam-error-under-apply-fails` subtest, same file) | exact |
| `cmd/engram/man.go` (new hidden command) | route/controller (cobra command) | file-I/O | `cmd/engram/backfill.go` (hidden/aliased command registration shape) + `cmd/engram/buildversion.go` (self-contained utility logic file, license header) | role-match |
| `cmd/engram/man_test.go` (new) | test | file-I/O | `cmd/engram/cmdwalk.go` + `cmd/engram/surfaces_test.go` (walking `rootCmd`, `commandWalkSkip` — but must NOT reuse `nonHiddenCommands()`, see Shared Patterns) | role-match |
| `cmd/engram/releaseconfig_test.go` (extend `TestReleaseConfigCaskInstallGate`) | test | transform (static string assertions) | itself (in-place edit, same file, `:14-29` `stripComment`/`nonCommentLines`, `:133-165` the gate) | exact |
| `.goreleaser.yaml` (extend cask hooks) | config | event-driven (install/uninstall hook) | itself (in-place edit) — the existing completions loop (`:184-200`) is the shape to mirror | exact |

## Pattern Assignments

### `internal/setup/environment.go` (utility, request-response)

**Analog:** itself — `internal/setup/environment.go:1-104` (current state, read in full)

**SPDX header** (lines 1-2, copy verbatim into every new Go file in scope):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

**Current `osRun`** (lines 84-104):
```go
func osRun(ctx context.Context, path string, args []string) (RunResult, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result := RunResult{Stdout: stdout.String(), Stderr: stderr.String()}

	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return result, nil
	case errors.As(runErr, &exitErr):
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	default:
		return result, runErr
	}
}
```

**D-10 fix — add one case FIRST in the switch, no new imports** (`context` and `errors` are already imported):
```go
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		return RunResult{}, ctx.Err()
	case runErr == nil:
		return result, nil
	case errors.As(runErr, &exitErr):
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	default:
		return result, runErr
	}
```

**Comment-style pattern to mirror:** every exported/unexported function above has a doc comment explaining WHY, citing decision IDs and other files (see `Environment.Run`'s doc lines 34-49). Update `osRun`'s own doc comment (lines 76-83) to state the new `ctx.Err()`-first contract instead of leaving it stale — this repo's convention is that doc comments are load-bearing, not decorative.

---

### `internal/setup/environment_test.go` (new, test, request-response)

**Analog:** `internal/store/store_test.go:270-303` (`TestDialTestClientFailsWhenRequiredAndUnavailable`) — the ONLY existing `os.Args[0]` re-exec-as-subprocess precedent in this repo.

**Re-exec idiom to copy** (adapt: engram's variant doesn't need a subprocess/CombinedOutput check of another test — it re-execs itself as the CHILD PROCESS `osRun` runs, per RESEARCH.md's recommended shape):
```go
// Source: internal/store/store_test.go:280-286 (verified pattern to mirror)
func TestDialTestClientFailsWhenRequiredAndUnavailable(t *testing.T) {
	if os.Getenv("ENGRAM_STORE_TEST_DIAL_FAIL_HELPER") == "1" {
		testQdrantAddr = ""
		dialTestClient(t)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestDialTestClientFailsWhenRequiredAndUnavailable", "-test.v")
	cmd.Env = append(os.Environ(), "ENGRAM_STORE_TEST_DIAL_FAIL_HELPER=1", "ENGRAM_REQUIRE_QDRANT=1")
	...
}
```

**Adapted shape for `osRun`'s deadline test** (per RESEARCH.md's own recommendation — a helper test gated on env var, invoked as `osRun`'s child via `os.Args[0]`, not via `exec.Command` + `CombinedOutput` since here `osRun` itself is under test):
```go
func TestOsRunReportsContextDeadlineExceeded(t *testing.T) {
	if os.Getenv("ENGRAM_SETUP_TEST_OSRUN_SLEEP_HELPER") == "1" {
		time.Sleep(2 * time.Second)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	res, err := osRun(ctx, os.Args[0], []string{
		"-test.run=TestOsRunReportsContextDeadlineExceeded", "-test.v",
	})
	// env var must be injected via exec.CommandContext's own Env — osRun's
	// signature takes only (ctx, path, args), so set it via os.Setenv +
	// t.Setenv is wrong (osRun doesn't fork with a customized env); instead
	// spawn the helper via osRun itself and have the helper read the env
	// var from THIS process's inherited environment — t.Setenv before
	// calling osRun is sufficient since exec.CommandContext inherits os.Environ().
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("osRun err = %v, want context.DeadlineExceeded", err)
	}
	if res != (RunResult{}) {
		t.Fatalf("osRun RunResult = %+v, want zero value alongside a ctx error (D-12)", res)
	}
}
```
Note: `t.Setenv("ENGRAM_SETUP_TEST_OSRUN_SLEEP_HELPER", "1")` before the `osRun` call — no `cmd.Env` override needed since `osRun`/`exec.CommandContext` already inherits `os.Environ()`. Never touches `$HOME` or a third-party CLI (rule `ryr82bf2s2`/`m45p2b4bp7`).

---

### `internal/setup/apply.go` (utility, request-response)

**Analog:** itself — `internal/setup/apply.go:1-30,115-160` (current state, read in full)

**SPDX header + imports** (lines 1-11):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)
```
D-11 adds `"errors"` to this block (alphabetical: after none, before `"fmt"`).

**Current `runSeam`** (lines 122-129):
```go
func runSeam(ctx context.Context, env Environment, path string, args []string) (RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	return env.Run(ctx, path, args)
}
```

**D-11 fix:**
```go
func runSeam(ctx context.Context, env Environment, path string, args []string) (RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	rr, err := env.Run(ctx, path, args)
	if err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s: %w", execTimeout, err)
	}
	return rr, err
}
```

**`describeSeamError`** (unchanged, lines 153-155 — confirms no further change needed since `%v` on a `%w`-wrapped error prints the full chain):
```go
func describeSeamError(name, cmdDisplay string, err error) string {
	return fmt.Sprintf("%s: %s: %v", name, cmdDisplay, err)
}
```

---

### `internal/setup/apply_test.go` (extend, test, request-response)

**Analog:** itself — the existing `probe-seam-error-under-apply-fails` subtest (lines ~193-208).

**Pattern to copy** (fake `Environment.Run` scripted to return the target error, NOT a real 20s wait — see RESEARCH.md Pitfall 3):
```go
t.Run("probe-seam-error-under-apply-fails", func(t *testing.T) {
	rt := fakeRuntime{name: "faketool", plan: Plan{
		Runtime: "faketool",
		Actions: []Action{{Args: []string{"faketool", "add"}}},
		Probe:   []string{"faketool", "get"},
	}}
	var calls []runCall
	env := fakeEnvWithRun(scriptedRun(&calls,
		scriptedResult{Err: errors.New("exec: start failure")},
	), "faketool")

	res := Apply(context.Background(), env, rt, Options{})
	if res.Outcome != OutcomeFailed {
		t.Fatalf("Apply outcome = %q, want %q", res.Outcome, OutcomeFailed)
	}
})
```

**New sibling subtest for D-11** (same shape, `scriptedResult{Err: context.DeadlineExceeded}` instead, asserting `Result`'s reason/notes contain `"timed out after 20s"`):
```go
t.Run("probe-seam-deadline-exceeded-wraps-timeout-wording", func(t *testing.T) {
	rt := fakeRuntime{name: "faketool", plan: Plan{
		Runtime: "faketool",
		Actions: []Action{{Args: []string{"faketool", "add"}}},
		Probe:   []string{"faketool", "get"},
	}}
	var calls []runCall
	env := fakeEnvWithRun(scriptedRun(&calls,
		scriptedResult{Err: context.DeadlineExceeded},
	), "faketool")

	res := Apply(context.Background(), env, rt, Options{})
	if res.Outcome != OutcomeFailed {
		t.Fatalf("Apply outcome = %q, want %q", res.Outcome, OutcomeFailed)
	}
	if !strings.Contains(res.Reason, "timed out after 20s: context deadline exceeded") {
		t.Errorf("Reason = %q, want it to contain the timeout wording", res.Reason)
	}
})
```
Confirm `fakeEnvWithRun`/`scriptedRun`/`scriptedResult`/`runCall` are the actual existing test helper names in this file before use — read the file's helper section to match field names exactly (this excerpt reconstructs the shape from the visible subtest; verify exact struct fields at implementation time).

---

### `cmd/engram/man.go` (new, controller/route, file-I/O)

**Analog 1 (registration shape):** `cmd/engram/backfill.go` (hidden/deprecated-adjacent command, `init()`-time `rootCmd.AddCommand` before other registration calls).

**Analog 2 (SPDX header + self-contained logic file style):** `cmd/engram/buildversion.go:1-13` (header + doc-comment-heavy unexported helpers).

**Analog 3 (version string reuse for D-02's `Source`):** `cmd/engram/root.go:19,26-28`:
```go
var version = "dev"
...
var rootCmd = &cobra.Command{
	Use:           "engram",
	...
	Version:       version,
	...
}
```

**Recommended shape** (per RESEARCH.md's own recommendation, consistent with `backfill.go`'s `init()`-registration convention):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// manDate is pinned to the Unix epoch in UTC (D-01) so every generated
// page's .TH date is byte-stable across machines and releases; .UTC() is
// required — the bare local-zone form would render "Dec 1969" on
// negative-UTC-offset machines (RESEARCH.md's Timezone nuance).
var manDate = time.Unix(0, 0).UTC()

var manCmd = &cobra.Command{
	Use:    "man <dir>",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return doc.GenManTree(rootCmd, &doc.GenManHeader{
			Date:    &manDate,
			Source:  "engram " + version,
			Manual:  "Engram Manual",
		}, args[0])
	},
}

func init() {
	rootCmd.DisableAutoGenTag = true // D-03
	rootCmd.AddCommand(manCmd)
}
```
Note: setting `rootCmd.DisableAutoGenTag = true` belongs in `man.go`'s own `init()` since it is scoped to this feature — do not add it to `root.go` unless the planner prefers colocating all `rootCmd` field-sets there; either is consistent with existing convention (`backfill.go`'s `init()` also mutates package-level `rootCmd`/command state it doesn't declare).

---

### `cmd/engram/man_test.go` (new, test, file-I/O)

**Analog 1 (walking rootCmd + skip predicate — but see Pitfall below):** `cmd/engram/cmdwalk.go:13-40` (`commandWalkSkip`, `commandKey`) and `cmd/engram/surfaces_test.go:97-133` (`nonHiddenCommands`, `keySet` idiom).

**CRITICAL — do not reuse `nonHiddenCommands()` for the expected-set test.** `commandWalkSkip` (`cmdwalk.go:22`) explicitly excludes `cmd.Name() == "completion"`, but D-05 requires `completion` subtree pages to exist. The man-page test must walk `rootCmd.Commands()` recursively using cobra's own `IsAvailableCommand()` / `IsAdditionalHelpTopicCommand()` predicates directly — the same predicate `GenManTree` itself uses internally (`cobra@v1.10.2/doc/man_docs.go:289-291`, `command.go:1607-1620`), never `commandWalkSkip`.

**`commandKey`-style helper to model the new walk on** (`cmd/engram/cmdwalk.go:27-40`, same idea, different predicate):
```go
func commandKey(cmd *cobra.Command) string {
	path := cmd.CommandPath()
	prefix := cmd.Root().Name() + " "
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	return path
}
```

**Byte-stability test shape** (per RESEARCH.md's verified live double-run):
```go
func TestManPagesByteStable(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	header := &doc.GenManHeader{Date: &manDate, Source: "engram " + version, Manual: "Engram Manual"}
	if err := doc.GenManTree(rootCmd, header, dir1); err != nil {
		t.Fatal(err)
	}
	if err := doc.GenManTree(rootCmd, header, dir2); err != nil {
		t.Fatal(err)
	}
	// walk dir1, compare byte-for-byte against the same filename in dir2
	// assert .TH line contains the pinned date/Source, and no HISTORY footer
}
```

**Expected-set test shape** (D-06 — walk `rootCmd` with cobra's own predicate, NOT `commandWalkSkip`):
```go
func TestManPagesMatchAvailableCommands(t *testing.T) {
	dir := t.TempDir()
	header := &doc.GenManHeader{Date: &manDate, Source: "engram " + version, Manual: "Engram Manual"}
	if err := doc.GenManTree(rootCmd, header, dir); err != nil {
		t.Fatal(err)
	}
	var want []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if !c.IsAvailableCommand() && c != rootCmd {
			return
		}
		want = append(want, strings.ReplaceAll(c.CommandPath(), " ", "-")+".1")
		for _, ch := range c.Commands() {
			if ch.IsAvailableCommand() || ch.HasAvailableSubCommands() {
				walk(ch)
			}
		}
	}
	walk(rootCmd)
	// assert dir's file list == want, and explicitly assert absence of
	// engram-man.1, engram-backfill-short-ids.1, engram-migrate-set-owner.1
}
```

---

### `cmd/engram/releaseconfig_test.go` (extend `TestReleaseConfigCaskInstallGate`)

**Analog:** itself — lines 1-30 (`stripComment`, `nonCommentLines`) and 133-165 (`TestReleaseConfigCaskInstallGate`, current state read in full).

**Current chain to extend** (verified current lines ~150-165):
```go
checkOrdering(t, file, lines, lineNos, "if OS.mac?", `system_command "/usr/bin/xattr"`)
checkOrdering(t, file, lines, lineNos, `system_command "/usr/bin/xattr"`, `"version", "--output", "json"`)
checkOrdering(t, file, lines, lineNos, `"version", "--output", "json"`, `args: ["completion"`)

forbidden := []string{"generate_completions_from_executable", "brews:", "rm_rf"}
for _, f := range forbidden {
	if n := countMatches(lines, f); n != 0 {
		t.Errorf("%s: expected 0 non-comment occurrences of %q, found %d", file, f, n)
	}
}

completionPaths := []string{
	"etc/bash_completion.d/engram",
	"share/zsh/site-functions/_engram",
	"share/fish/vendor_completions.d/engram.fish",
}
for _, p := range completionPaths {
	if n := countMatches(lines, p); n != 2 {
		t.Errorf("%s: expected exactly 2 non-comment occurrences of %q (install + uninstall), found %d", file, p, n)
	}
}
```

**D-09 extension** (append one ordering link, one count assertion, one glob-string count assertion, widen `forbidden`):
```go
checkOrdering(t, file, lines, lineNos, `args: ["completion"`, `args: ["man"`)

if n := countMatches(lines, `args: ["man"`); n != 1 {
	t.Errorf("%s: expected exactly 1 non-comment occurrence of `args: [\"man\"`, found %d", file, n)
}

const manpageGlob = `Dir.glob("#{HOMEBREW_PREFIX}/share/man/man1/engram{,-*}.1")`
if n := countMatches(lines, manpageGlob); n != 1 {
	t.Errorf("%s: expected exactly 1 non-comment occurrence of the uninstall man-page glob, found %d", file, n)
}

forbidden := []string{"generate_completions_from_executable", "brews:", "rm_rf", "manpage:"}
```
`stripComment`'s `#{` lookahead (lines 14-29, unchanged) already handles Ruby interpolation in the new lines — no test-harness change needed, only new assertions (Pitfall 2).

---

### `.goreleaser.yaml` (config, event-driven cask hook)

**Analog:** itself — the existing completions loop (`post.install`, current lines ~184-200) and the three `rm_f` lines (`post.uninstall`, current lines ~201-210).

**Shape to mirror for `post_install`** (append after the completions `.each` block, before the heredoc closes):
```ruby
man1_dir = "#{HOMEBREW_PREFIX}/share/man/man1"
FileUtils.mkdir_p(man1_dir)
system_command binary, args: ["man", man1_dir]
```

**Shape to mirror for `post_uninstall`** (append after the three completion `rm_f` lines):
```ruby
Dir.glob("#{HOMEBREW_PREFIX}/share/man/man1/engram{,-*}.1").each { |f| FileUtils.rm_f f }
```

**Rule to follow (D-09):** comments near these lines must not spell `manpage:` or name any Homebrew helper method (`generate_completions_from_executable`-style), because `forbidden` is a literal occurrence count over the whole file including comments — mirror the existing completions section's comment style (explains WHY, never names the rejected mechanism except as prose describing "the declarative field" without the literal string).

## Shared Patterns

### SPDX header (every new Go file)
**Source:** `internal/setup/environment.go:1-2`, `cmd/engram/releaseconfig_test.go:1-2`, `cmd/engram/backfill.go:1-2` — universal, byte-identical across the repo.
**Apply to:** `cmd/engram/man.go`, `cmd/engram/man_test.go`, `internal/setup/environment_test.go`.
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```
Do NOT add this header to `.planning/**` files (would break GSD's frontmatter parsing per repo CLAUDE.md) — not applicable here since all new files in this phase are `.go`/`.yaml`.

### Seam-first `ctx.Err()` classification
**Source:** `internal/setup/environment.go:84-104` (current), D-10's fix.
**Apply to:** `osRun` only — this is the single production `Environment.Run` implementation; no other seam in the repo needs this treatment per RESEARCH.md.

### Fake-`Environment`-injection testing (never touch real subprocess/`$HOME` for D-11's wrap logic)
**Source:** `internal/setup/apply_test.go`'s existing `probe-seam-error-under-apply-fails` subtest.
**Apply to:** `apply_test.go`'s new DeadlineExceeded subtest — inject the error via the fake `Environment.Run`, never wait out the real 20s `execTimeout` (Pitfall 3).

### `os.Args[0]` re-exec for a real (but harmless) child process
**Source:** `internal/store/store_test.go:270-303`.
**Apply to:** `internal/setup/environment_test.go`'s new `osRun` deadline test — the only place in this phase that needs a genuinely running child process outliving a context deadline without invoking a third-party CLI (rule `m45p2b4bp7`).

### Literal-occurrence-count static gates over `.goreleaser.yaml`
**Source:** `cmd/engram/releaseconfig_test.go`'s `stripComment`/`nonCommentLines`/`countMatches`/`checkOrdering` (lines 1-30, ~133-165).
**Apply to:** `releaseconfig_test.go`'s new man-page assertions AND `.goreleaser.yaml`'s new hook lines — the two must be co-designed: whatever literal Ruby string is chosen for the man1 path/glob must appear in both files verbatim (durable-memory rule `6dt7nh9tse`).

### cobra's own `IsAvailableCommand()` walk (never `commandWalkSkip`)
**Source:** `cobra@v1.10.2/doc/man_docs.go:289-291`, `cobra@v1.10.2/command.go:1607-1620` (vendored, not in this repo's tree but authoritative for `GenManTree`'s own filtering).
**Apply to:** `cmd/engram/man_test.go`'s D-06 expected-set test — must NOT import or call `nonHiddenCommands()`/`commandWalkSkip` (`cmd/engram/cmdwalk.go:22`, `surfaces_test.go:107-109`), which deliberately excludes the `completion` subtree and would silently disagree with D-05 (Pitfall 1).

## No Analog Found

None — every file in scope has at least a role-match analog; the two "new file, no direct precedent in this repo" cases (`man.go`'s `doc.GenManTree` wrapping, `environment_test.go`'s deadline-specific re-exec helper) both have a documented, verified upstream-library or in-repo-idiom source cited above and in RESEARCH.md's Code Examples section.

## Metadata

**Analog search scope:** `internal/setup/*.go`, `cmd/engram/*.go`, `internal/store/store_test.go`, `.goreleaser.yaml`, vendored `cobra@v1.10.2/doc/*.go` (read-only reference, not part of this repo's tracked tree).
**Files scanned:** 12 (read in full or targeted ranges) — `internal/setup/environment.go`, `internal/setup/apply.go`, `internal/setup/apply_test.go`, `cmd/engram/root.go`, `cmd/engram/buildversion.go`, `cmd/engram/backfill.go`, `cmd/engram/migrate.go`, `cmd/engram/cmdwalk.go`, `cmd/engram/surfaces_test.go`, `cmd/engram/releaseconfig_test.go`, `internal/store/store_test.go`, `.goreleaser.yaml`.
**Pattern extraction date:** 2026-09-13
**Tracked-source gate:** all analog paths above are ordinary tracked source files under `internal/`, `cmd/engram/`, and repo root — none are `.gsd/capabilities/` mirrors; verified via direct `Read`/`Bash` access to the working tree (no gitignored mirror paths were considered as analogs).
