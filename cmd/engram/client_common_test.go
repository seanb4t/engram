// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/server"
)

// TestExitCodeForConnectErrTable is the D-10 mapper table test: every
// connect.Code from 1 through 16, plus an error that is not a
// *connect.Error at all.
func TestExitCodeForConnectErrTable(t *testing.T) {
	cases := []struct {
		code connect.Code
		want int
	}{
		{connect.CodeCanceled, exitUnavailable},
		{connect.CodeUnknown, exitGeneric},
		{connect.CodeInvalidArgument, exitUsage},
		{connect.CodeDeadlineExceeded, exitTimeout},
		{connect.CodeNotFound, exitNotFound},
		{connect.CodeAlreadyExists, exitGeneric},
		{connect.CodePermissionDenied, exitAuth},
		{connect.CodeResourceExhausted, exitGeneric},
		{connect.CodeFailedPrecondition, exitUsage},
		{connect.CodeAborted, exitGeneric},
		{connect.CodeOutOfRange, exitUsage},
		{connect.CodeUnimplemented, exitGeneric},
		{connect.CodeInternal, exitGeneric},
		{connect.CodeUnavailable, exitUnavailable},
		{connect.CodeDataLoss, exitGeneric},
		{connect.CodeUnauthenticated, exitAuth},
	}
	if len(cases) != 16 {
		t.Fatalf("test table has %d entries, want 16 (one per connect.Code)", len(cases))
	}
	for _, c := range cases {
		t.Run(c.code.String(), func(t *testing.T) {
			err := connect.NewError(c.code, errors.New("boom"))
			if got := exitCodeForConnectErr(err); got != c.want {
				t.Errorf("exitCodeForConnectErr(%v) = %d, want %d", c.code, got, c.want)
			}
		})
	}
	t.Run("not a connect.Error", func(t *testing.T) {
		if got := exitCodeForConnectErr(errors.New("plain")); got != exitGeneric {
			t.Errorf("exitCodeForConnectErr(plain error) = %d, want %d", got, exitGeneric)
		}
	})
}

// TestExitCodeTimeoutDistinctFromUnavailable is the D-06 gate: a deadline
// exceeded RPC and a dial failure must produce DIFFERENT exit codes,
// asserted by explicit inequality — not merely each being "not the
// default" (memory 667p88n2be: a test asserting only "not CodeInternal"
// still passed on a switch-arm ordering bug that silently collapsed every
// error class back to one code). It also pins CodeCanceled's placement:
// caller-initiated cancellation stays on exitUnavailable, not exitTimeout.
func TestExitCodeTimeoutDistinctFromUnavailable(t *testing.T) {
	if exitTimeout == exitUnavailable {
		t.Fatalf("exitTimeout (%d) == exitUnavailable (%d), want distinct", exitTimeout, exitUnavailable)
	}

	deadline := exitCodeForConnectErr(connect.NewError(connect.CodeDeadlineExceeded, errors.New("boom")))
	if deadline != exitTimeout {
		t.Errorf("exitCodeForConnectErr(CodeDeadlineExceeded) = %d, want %d (exitTimeout)", deadline, exitTimeout)
	}

	unavailable := exitCodeForConnectErr(connect.NewError(connect.CodeUnavailable, errors.New("boom")))
	if unavailable != exitUnavailable {
		t.Errorf("exitCodeForConnectErr(CodeUnavailable) = %d, want %d (exitUnavailable)", unavailable, exitUnavailable)
	}

	canceled := exitCodeForConnectErr(connect.NewError(connect.CodeCanceled, errors.New("boom")))
	if canceled != exitUnavailable {
		t.Errorf("exitCodeForConnectErr(CodeCanceled) = %d, want %d (exitUnavailable) — a caller-initiated "+
			"cancellation is not a server that failed to answer", canceled, exitUnavailable)
	}

	if deadline == unavailable {
		t.Fatalf("exitCodeForConnectErr(CodeDeadlineExceeded) == exitCodeForConnectErr(CodeUnavailable) (%d), want distinct", deadline)
	}

	// The full set of codes producible across every connect.Code plus a
	// non-connect error plus exitOK must equal exactly {0,1,2,3,4,5,6}.
	got := map[int]bool{exitOK: true}
	for i := 1; i <= 16; i++ {
		got[exitCodeForConnectErr(connect.NewError(connect.Code(i), errors.New("boom")))] = true
	}
	got[exitCodeForConnectErr(errors.New("not a connect error"))] = true
	want := map[int]bool{
		exitOK: true, exitGeneric: true, exitUsage: true, exitAuth: true,
		exitNotFound: true, exitUnavailable: true, exitTimeout: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("codes producible by exitCodeForConnectErr = %v, want %v", got, want)
	}
}

// TestResolveOutputFormat is the D-05/D-06 table test over the six
// TTY/flag combinations plus an invalid --output value.
func TestResolveOutputFormat(t *testing.T) {
	cases := []struct {
		name    string
		flagVal string
		isTTY   bool
		want    outputFormat
	}{
		{"empty tty=true", "", true, formatText},
		{"empty tty=false", "", false, formatJSON},
		{"json tty=true", "json", true, formatJSON},
		{"json tty=false", "json", false, formatJSON},
		{"text tty=true", "text", true, formatText},
		{"text tty=false", "text", false, formatText},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveOutputFormat(c.flagVal, c.isTTY)
			if err != nil {
				t.Fatalf("resolveOutputFormat(%q, %v) returned error: %v", c.flagVal, c.isTTY, err)
			}
			if got != c.want {
				t.Errorf("resolveOutputFormat(%q, %v) = %v, want %v", c.flagVal, c.isTTY, got, c.want)
			}
		})
	}
	t.Run("invalid value", func(t *testing.T) {
		_, err := resolveOutputFormat("yaml", false)
		if err == nil {
			t.Fatal("expected an error for an invalid --output value")
		}
		var ec interface{ ExitCode() int }
		if !errors.As(err, &ec) {
			t.Fatalf("error %v does not carry ExitCode()", err)
		}
		if ec.ExitCode() != exitUsage {
			t.Errorf("ExitCode() = %d, want %d", ec.ExitCode(), exitUsage)
		}
	})
}

// TestValidateScopeCrossSpineParity is the D-03 anti-drift gate: it pins
// requireScopeUnlessCrossSpine against internal/server.EffectiveSearchScope
// across the three rows that function still governs after D-07 (scope
// empty/non-empty x cross-spine off/on, MINUS the scope-set-with-cross-spine
// row, which cobra's declared flag group now rejects before this function
// ever runs — see the end-to-end subtest below).
//
// This does NOT assert blanket equality — that would be wrong by
// construction. D-04 makes the client deliberately STRICTER than the
// server on the "scope set, cross-spine on" row: the server accepts that
// combination and silently discards the scope (Phase 3 D-02), while the
// client rejects it up front so an explicitly-typed filter is never
// silently ignored. The load-bearing assertion below is therefore
// one-directional: the client must never ACCEPT an input the server would
// REJECT. Do not relax this row into agreement; it is the intended
// divergence, not a bug.
//
// Importing internal/server here is legal precisely because
// TestClientFilesImportBoundary's file walk skips any file whose name ends
// in "_test.go" (see the "scanned" loop above) — only production
// client_*.go files are denylisted from internal/server.
func TestValidateScopeCrossSpineParity(t *testing.T) {
	cases := []struct {
		name       string
		scope      string
		crossSpine bool
		clientErr  bool
		serverErr  bool
	}{
		{"empty scope, cross-spine off", "", false, true, true},
		{"empty scope, cross-spine on", "", true, false, false},
		{"scope set, cross-spine off", "repo:x", false, false, false},
	}
	if len(cases) != 3 {
		t.Fatalf("test table has %d entries, want 3 (the rows requireScopeUnlessCrossSpine still governs)", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clientErr := requireScopeUnlessCrossSpine(c.scope, c.crossSpine) != nil
			if clientErr != c.clientErr {
				t.Errorf("requireScopeUnlessCrossSpine(%q, %v) error presence = %v, want %v", c.scope, c.crossSpine, clientErr, c.clientErr)
			}
			_, serverRawErr := server.EffectiveSearchScope(c.scope, c.crossSpine)
			serverErr := serverRawErr != nil
			if serverErr != c.serverErr {
				t.Errorf("EffectiveSearchScope(%q, %v) error presence = %v, want %v", c.scope, c.crossSpine, serverErr, c.serverErr)
			}
			// Load-bearing invariant: the client must never accept an input
			// the server would reject (the client may only be stricter,
			// never looser).
			if !clientErr && serverErr {
				t.Errorf("client accepted (%q, %v) but server would reject it — client guard has drifted looser than EffectiveSearchScope", c.scope, c.crossSpine)
			}
		})
	}

	// The D-04 divergence row — scope set together with cross-spine — moved
	// from requireScopeUnlessCrossSpine to a declared cobra flag group on
	// searchCmd and listCmd (D-07). Assert it end-to-end, on both commands:
	// the CLI rejects with exitUsage even though
	// internal/server.EffectiveSearchScope accepts the identical combination
	// (and silently discards the scope). Do not "fix" this into agreement —
	// it is the intended divergence pinned above, now enforced one layer
	// earlier than the Go function.
	t.Run("scope set, cross-spine on (D-04: client is stricter here) — enforced by cobra flag group", func(t *testing.T) {
		if _, serverRawErr := server.EffectiveSearchScope("repo:x", true); serverRawErr != nil {
			t.Fatalf("server.EffectiveSearchScope(%q, true) = %v, want nil — the server accepts this combination; the client's stricter rejection is what this test pins", "repo:x", serverRawErr)
		}
		for _, cmdName := range []string{"search", "list"} {
			t.Run(cmdName, func(t *testing.T) {
				resetClientFlags(t)
				cmd := searchCmd
				args := []string{"search", "--server", deadServer, "--scope", "repo:x", "--cross-spine", "--query", "q"}
				if cmdName == "list" {
					cmd = listCmd
					args = []string{"list", "--server", deadServer, "--scope", "repo:x", "--cross-spine"}
				}
				resetCommandFlagState(t, cmd)

				_, _, err := runClient(t, args...)
				if got := exitCodeFromError(err); got != exitUsage {
					t.Errorf("%s --scope repo:x --cross-spine: exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", cmdName, got, exitUsage, err)
				}
			})
		}
	})
}

// scopeCrossSpineFlagCommand is one row of TestScopeCrossSpineFlagsNameEachOther's
// table: a command that carries both --scope and --cross-spine flags.
var scopeCrossSpineFlagCommands = []*cobra.Command{
	searchCmd,
	listCmd,
}

// TestScopeCrossSpineFlagsNameEachOther is the D-00 mechanical check: on
// every command carrying both flags, each flag's Usage string names the
// other by its literal double-dash spelling, so the relationship is
// discoverable from either entry point without running the command.
func TestScopeCrossSpineFlagsNameEachOther(t *testing.T) {
	if len(scopeCrossSpineFlagCommands) != 2 {
		t.Fatalf("scopeCrossSpineFlagCommands has %d entries, want 2 (searchCmd, listCmd)", len(scopeCrossSpineFlagCommands))
	}
	for _, cmd := range scopeCrossSpineFlagCommands {
		t.Run(cmd.Name(), func(t *testing.T) {
			scopeFlag := cmd.Flags().Lookup("scope")
			if scopeFlag == nil || scopeFlag.Usage == "" {
				t.Fatalf("%s: --scope flag missing or has empty Usage", cmd.Name())
			}
			crossSpineFlag := cmd.Flags().Lookup("cross-spine")
			if crossSpineFlag == nil || crossSpineFlag.Usage == "" {
				t.Fatalf("%s: --cross-spine flag missing or has empty Usage", cmd.Name())
			}
			if !strings.Contains(scopeFlag.Usage, "--cross-spine") {
				t.Errorf("%s: --scope Usage %q does not name --cross-spine", cmd.Name(), scopeFlag.Usage)
			}
			if !strings.Contains(crossSpineFlag.Usage, "--scope") {
				t.Errorf("%s: --cross-spine Usage %q does not name --scope", cmd.Name(), crossSpineFlag.Usage)
			}
		})
	}
}

// TestIsTerminalOnNonTTY confirms isTerminal is false for a pipe and for a
// regular file — the only two non-pty cases a test can exercise. The
// positive branch (a real pty) cannot be exercised without one, which is
// why resolveOutputFormat takes the boolean as a parameter and is tested
// directly for isTTY=true above; this gap is recorded here, not hidden.
func TestIsTerminalOnNonTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(r) {
		t.Error("isTerminal(pipe read end) = true, want false")
	}

	f, err := os.CreateTemp(t.TempDir(), "isterm")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("isTerminal(regular file) = true, want false")
	}
}

// allowedClientImports is the exhaustive set of non-standard-library
// imports a cmd/engram/client_*.go production file may use. Adding an
// entry here is a deliberate architectural decision, not a mechanical
// fix — and adding a repo-internal path to it will fail
// TestClientFilesImportBoundary's second clause regardless.
var allowedClientImports = map[string]bool{
	"connectrpc.com/connect":                                     true,
	"github.com/spf13/cobra":                                     true,
	"google.golang.org/protobuf/encoding/protojson":              true,
	"google.golang.org/protobuf/proto":                           true,
	"github.com/seanb4t/engram/gen/go/engram/v1":                 true,
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect": true,
}

// TestClientFilesImportBoundary is the REQ-cli-client-commands / D-04 gate.
// A package-wide `go list -deps ./cmd/engram/...` gate is NOT usable here
// and would be a false RED: reindex.go and prune.go sit in the same
// package and legitimately import internal/store, so the boundary must be
// per-file.
//
// This gate constrains the client command IMPLEMENTATIONS, not the linked
// binary, which also contains the operator commands.
func TestClientFilesImportBoundary(t *testing.T) {
	files, err := filepath.Glob("client_*.go")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no client_*.go files found — gate cannot exercise anything")
	}

	fset := token.NewFileSet()
	seen := map[string]bool{}
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		scanned++
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", f, err)
		}
		for _, imp := range af.Imports {
			seen[strings.Trim(imp.Path.Value, `"`)] = true
		}
	}
	if scanned == 0 {
		t.Fatal("no non-test client_*.go production files found — gate cannot exercise anything")
	}

	// Clause 1: every non-standard-library import is in the allowlist. An
	// import is treated as standard library when its first path segment
	// contains no dot — this admits any stdlib package the implementation
	// legitimately needs while still catching every module.
	for path := range seen {
		firstSeg := path
		if i := strings.Index(path, "/"); i >= 0 {
			firstSeg = path[:i]
		}
		if !strings.Contains(firstSeg, ".") {
			continue // stdlib
		}
		if !allowedClientImports[path] {
			t.Errorf("REQ-cli-client-commands/D-04: import %q in a client_*.go production file is not in allowedClientImports", path)
		}
	}

	// Clause 2: no member of allowedClientImports is itself a repo-internal
	// path — this closes the "just widen the allowlist" escape hatch.
	for allowed := range allowedClientImports {
		if strings.HasPrefix(allowed, "github.com/seanb4t/engram/internal/") {
			t.Errorf("REQ-cli-client-commands/D-04: allowedClientImports contains a repo-internal path %q — widening the allowlist to admit an internal package is itself the violation this gate exists to catch", allowed)
		}
	}

	// Clause 3: the explicit denylist appears in no client file.
	denylist := []string{
		"github.com/seanb4t/engram/internal/store",
		"github.com/seanb4t/engram/internal/authz",
		"github.com/seanb4t/engram/internal/embed",
		"github.com/seanb4t/engram/internal/server",
		"github.com/seanb4t/engram/internal/config",
	}
	for _, deny := range denylist {
		if seen[deny] {
			t.Errorf("REQ-cli-client-commands/D-04: client_*.go imports %q, which no client implementation file may import", deny)
		}
	}
}

// TestNoTokenFlagAnywhere is the D-13 gate: no command in the binary — root
// or any subcommand — declares a flag named "token". search's --token-file
// is the positive control that the walk actually reaches the client
// commands: without it, this test would pass against a tree walk that
// visits nothing.
func TestNoTokenFlagAnywhere(t *testing.T) {
	var foundTokenFile bool
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		checkFlags := func(f *pflag.Flag) {
			if f.Name == "token" {
				t.Errorf("command %q declares a --token flag", cmd.CommandPath())
			}
			if f.Name == "token-file" && cmd.Name() == "search" {
				foundTokenFile = true
			}
		}
		cmd.Flags().VisitAll(checkFlags)
		cmd.PersistentFlags().VisitAll(checkFlags)
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	if !foundTokenFile {
		t.Fatal("positive control failed: search does not declare --token-file — the tree walk is not reaching client commands")
	}
}

// TestClientCommandsAcceptNoPositionalArgs is the D-13 structural gate:
// nothing typed after the verb is consumed as data.
func TestClientCommandsAcceptNoPositionalArgs(t *testing.T) {
	resetClientFlags(t)
	if searchCmd.Args == nil {
		t.Fatal("searchCmd.Args is nil — no positional-argument policy is enforced")
	}
	svc := &stubEngramService{}
	url := startStubServer(t, svc)
	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x", "some-positional")
	if err == nil {
		t.Error("expected an error for a positional argument, got nil")
	}
}

// TestInsecureWarnsOnStderrAndStdoutStaysJSON is the D-14/D-07 gate: the
// entire stdout buffer must parse as one JSON object even with --insecure
// set. A strings.Contains check on stdout would pass even with a warning
// line prepended — this test unmarshals the whole buffer.
func TestInsecureWarnsOnStderrAndStdoutStaysJSON(t *testing.T) {
	resetClientFlags(t)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, stderr, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x", "--insecure", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr == "" {
		t.Fatal("stderr is empty, want an --insecure warning")
	}
	if !strings.Contains(strings.ToLower(stderr), "certificate") {
		t.Errorf("stderr = %q, want it to mention certificate verification", stderr)
	}
	var js json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &js); err != nil {
		t.Fatalf("stdout did not unmarshal in its entirety as one JSON object: %v\nstdout=%q", err, stdout)
	}
	if strings.Contains(stdout, "WARNING") || strings.Contains(strings.ToLower(stdout), "certificate") {
		t.Errorf("stdout = %q, want the warning text to appear nowhere in stdout", stdout)
	}
}

// TestInsecureIsNotSetByEnvironment is the D-14 gate: verification cannot
// be disabled by anything but an explicit --insecure flag on the command
// line.
func TestInsecureIsNotSetByEnvironment(t *testing.T) {
	resetClientFlags(t)
	t.Setenv("ENGRAM_INSECURE", "true")
	t.Setenv("ENGRAM_TLS_INSECURE", "true")
	t.Setenv("ENGRAM_SKIP_VERIFY", "true")

	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	_, stderr, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clientInsecure {
		t.Error("clientInsecure = true; want false — no environment variable may disable TLS verification")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty (no --insecure warning without the flag)", stderr)
	}
}

// TestTLSVerificationOnByDefault reads the constructed *http.Client rather
// than trusting the flag default (D-14).
func TestTLSVerificationOnByDefault(t *testing.T) {
	secure := newHTTPClient(false)
	transport, ok := secure.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("newHTTPClient(false).Transport is %T, want *http.Transport", secure.Transport)
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("newHTTPClient(false).Transport.TLSClientConfig.InsecureSkipVerify = true, want false")
	}

	insecure := newHTTPClient(true)
	transport2, ok := insecure.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("newHTTPClient(true).Transport is %T, want *http.Transport", insecure.Transport)
	}
	if !transport2.TLSClientConfig.InsecureSkipVerify {
		t.Error("newHTTPClient(true).Transport.TLSClientConfig.InsecureSkipVerify = false, want true")
	}
}

// assertNoSentinel is shared by TestTokenNeverAppearsInOutput's subtests.
func assertNoSentinel(t *testing.T, sentinel, stdout, stderr string, err error) {
	t.Helper()
	if strings.Contains(stdout, sentinel) {
		t.Errorf("stdout contains the token sentinel: %q", stdout)
	}
	if strings.Contains(stderr, sentinel) {
		t.Errorf("stderr contains the token sentinel: %q", stderr)
	}
	if err != nil && strings.Contains(err.Error(), sentinel) {
		t.Errorf("returned error contains the token sentinel: %v", err)
	}
}

// TestTokenNeverAppearsInOutput proves the resolved credential never
// appears in stdout, stderr, or the returned error's Error() string, on the
// success, auth-failure, and transport-failure paths — the last being the
// most likely accidental vehicle, since a wrapped transport error can
// carry the request URL.
func TestTokenNeverAppearsInOutput(t *testing.T) {
	const sentinel = "sentinel-do-not-leak-9f3ac1"

	t.Run("success path", func(t *testing.T) {
		resetClientFlags(t)
		t.Setenv("ENGRAM_TOKEN", sentinel)
		svc := &stubEngramService{
			searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
				return &engramv1.SearchMemoriesResponse{}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, stderr, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x")
		assertNoSentinel(t, sentinel, stdout, stderr, err)
	})

	t.Run("auth failure path", func(t *testing.T) {
		resetClientFlags(t)
		t.Setenv("ENGRAM_TOKEN", sentinel)
		svc := &stubEngramService{
			searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("nope"))
			},
		}
		url := startStubServer(t, svc)
		stdout, stderr, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x")
		assertNoSentinel(t, sentinel, stdout, stderr, err)
	})

	t.Run("transport failure path", func(t *testing.T) {
		resetClientFlags(t)
		t.Setenv("ENGRAM_TOKEN", sentinel)
		stdout, stderr, err := runClient(t, "search", "--server", "http://127.0.0.1:1", "--query", "q", "--scope", "repo:x")
		assertNoSentinel(t, sentinel, stdout, stderr, err)
	})
}

// TestTokenFileTrailingNewlineTrimmed proves resolveToken trims the
// trailing newline a shell redirect writes by default — an untrimmed
// value would become a malformed multi-field credential the server's
// extractor rejects.
func TestTokenFileTrailingNewlineTrimmed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("abc123\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("ENGRAM_TOKEN", "")
	got, err := resolveToken(path)
	if err != nil {
		t.Fatalf("resolveToken: %v", err)
	}
	if got != "abc123" {
		t.Errorf("resolveToken(file with trailing newline) = %q, want %q", got, "abc123")
	}
}

// TestTokenFileUnreadableIsUsageError pins REVIEW.md WR-01: an unreadable
// --token-file is the caller naming a bad path, so it must exit 2 (the
// client's own semantic validation, per D-17) and not 1. Exiting 1 would be
// indistinguishable from a server-side failure, leaving a script unable to
// tell "fix your path" from "retry later".
//
// The positive control matters here: without asserting the error is non-nil
// FIRST, an errors.As against a nil error would simply report false and the
// test would read as if it had checked the code.
func TestTokenFileUnreadableIsUsageError(t *testing.T) {
	t.Setenv("ENGRAM_TOKEN", "")
	missing := filepath.Join(t.TempDir(), "no-such-token-file")

	_, err := resolveToken(missing)
	if err == nil {
		t.Fatal("expected an error for an unreadable --token-file")
	}
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatalf("error %v does not carry ExitCode(); an unreadable --token-file must be a usage error", err)
	}
	if ec.ExitCode() != exitUsage {
		t.Errorf("ExitCode() = %d, want %d (exitUsage)", ec.ExitCode(), exitUsage)
	}
}

// TestNoClientPathReadsStandardInput is the REQ-cli-agent-output gate: no
// client_*.go production file may reference os.Stdin. A cron loop blocked
// on a hidden prompt is the failure mode the requirement exists to
// prevent, and a code-review note does not survive a future contributor.
func TestNoClientPathReadsStandardInput(t *testing.T) {
	files, err := filepath.Glob("client_*.go")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		scanned++
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", f, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if ident.Name == "os" && sel.Sel.Name == "Stdin" {
				t.Errorf("%s references os.Stdin — no client code path may read standard input (REQ-cli-agent-output)", f)
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("no non-test client_*.go production files found — gate cannot exercise anything")
	}
}

// TestRootSilencesUsageAndErrors pins the D-07 no-op: rootCmd already
// silences cobra's own usage/error printing, and searchCmd must not set
// either flag redundantly — a future edit to root.go cannot silently start
// printing usage text to stdout on error without this test failing.
func TestRootSilencesUsageAndErrors(t *testing.T) {
	if !rootCmd.SilenceUsage {
		t.Error("rootCmd.SilenceUsage = false, want true")
	}
	if !rootCmd.SilenceErrors {
		t.Error("rootCmd.SilenceErrors = false, want true")
	}
	if searchCmd.SilenceUsage {
		t.Error("searchCmd.SilenceUsage = true, want false — this is inherited from rootCmd, not set redundantly")
	}
	if searchCmd.SilenceErrors {
		t.Error("searchCmd.SilenceErrors = true, want false — this is inherited from rootCmd, not set redundantly")
	}
}
