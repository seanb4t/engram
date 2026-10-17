// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// consoleStart records what reached runConsoleProgram: the model, and the
// stderr the caller had already seen when the program would have taken
// over the screen.
type consoleStart struct {
	started bool
	model   tea.Model
	stderr  string
}

// forceTerminal makes the console's terminal check pass and replaces the
// program run with a recorder, so no test ever takes over a real terminal.
func forceTerminal(t *testing.T) *consoleStart {
	t.Helper()
	rec := &consoleStart{}
	origTerm, origRun := stdioIsTerminal, runConsoleProgram
	stdioIsTerminal = func() bool { return true }
	runConsoleProgram = func(_ context.Context, m tea.Model, _ ...tea.ProgramOption) error {
		rec.started, rec.model = true, m
		if b, ok := rootCmd.ErrOrStderr().(*bytes.Buffer); ok {
			rec.stderr = b.String()
		}
		return nil
	}
	t.Cleanup(func() { stdioIsTerminal, runConsoleProgram = origTerm, origRun })
	return rec
}

func scopesStub() *stubEngramService {
	return &stubEngramService{
		listScopesFn: func(context.Context, *engramv1.ListScopesRequest) (*engramv1.ListScopesResponse, error) {
			return &engramv1.ListScopesResponse{Scopes: []*engramv1.ScopeCount{{Scope: "repo:example/one", Count: 3}}}, nil
		},
	}
}

// TestRootBareInvocationNeedsTerminal: without a terminal, a bare engram
// opens nothing and prints one line pointing at engram catalog (the line
// Execute writes to stderr is err's text), with exit 2.
func TestRootBareInvocationNeedsTerminal(t *testing.T) {
	resetClientFlags(t)
	stdout, stderr, err := runClient(t, noArgs...)
	if stdout != "" || stderr != "" {
		t.Errorf("stdout = %q, stderr = %q, want both empty before Execute prints the error", stdout, stderr)
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Fatalf("exitCodeFromError(err) = %d, want %d (err=%v)", got, exitUsage, err)
	}
	if msg := err.Error(); !strings.Contains(msg, "engram catalog") || strings.Contains(msg, "\n") {
		t.Errorf("error = %q, want one line naming engram catalog", msg)
	}
}

// TestTUINeedsTerminal: engram tui says that it needs a terminal and names
// engram catalog.
func TestTUINeedsTerminal(t *testing.T) {
	resetClientFlags(t)
	stdout, _, err := runClient(t, "tui")
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Fatalf("exitCodeFromError(err) = %d, want %d (err=%v)", got, exitUsage, err)
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "engram tui needs a terminal") || !strings.Contains(msg, "engram catalog") || strings.Contains(msg, "\n") {
		t.Errorf("error = %q, want one line saying engram tui needs a terminal and naming engram catalog", msg)
	}
}

func TestTUIRejectsPositionalArgument(t *testing.T) {
	resetClientFlags(t)
	rec := forceTerminal(t)
	stdout, _, err := runClient(t, "tui", "extra")
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (err=%v)", got, exitUsage, err)
	}
	if rec.started {
		t.Error("the console started despite a positional argument")
	}
}

// TestTUIStartupFailures: every startup failure exits before the console
// takes over the screen, with the exit code a client command gives.
func TestTUIStartupFailures(t *testing.T) {
	unauth := &stubEngramService{
		listScopesFn: func(context.Context, *engramv1.ListScopesRequest) (*engramv1.ListScopesResponse, error) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("bad token"))
		},
	}
	cases := []struct {
		name     string
		args     func(t *testing.T) []string
		want     int
		contains []string
	}{
		{
			name:     "no server",
			args:     func(*testing.T) []string { return []string{"tui"} },
			want:     exitUsage,
			contains: []string{"--server", "ENGRAM_SERVER_URL"},
		},
		{
			name: "credential rejected",
			args: func(t *testing.T) []string { return []string{"tui", "--server", startStubServer(t, unauth)} },
			want: exitAuth,
		},
		{
			name: "server unreachable",
			args: func(t *testing.T) []string { return []string{"tui", "--server", closedPortURL(t)} },
			want: exitUnavailable,
		},
		{
			name: "deadline passed",
			args: func(t *testing.T) []string {
				return []string{"tui", "--server", startHungServer(t), "--timeout", "200ms"}
			},
			want: exitTimeout,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("ENGRAM_SERVER_URL", "")
			resetClientFlags(t)
			rec := forceTerminal(t)
			_, _, err := runClient(t, c.args(t)...)
			if got := exitCodeFromError(err); got != c.want {
				t.Fatalf("exitCodeFromError(err) = %d, want %d (err=%v)", got, c.want, err)
			}
			for _, s := range c.contains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("error = %q, want it to name %s", err.Error(), s)
				}
			}
			if rec.started {
				t.Error("the console started despite a startup failure")
			}
		})
	}
}

// TestTUIInsecureWarnsBeforeStart: the TLS warning is on stderr before the
// console would take over the screen.
func TestTUIInsecureWarnsBeforeStart(t *testing.T) {
	resetClientFlags(t)
	rec := forceTerminal(t)
	_, _, err := runClient(t, "tui", "--server", startStubServer(t, scopesStub()), "--insecure")
	if err != nil {
		t.Fatalf("engram tui: %v", err)
	}
	if !rec.started {
		t.Fatal("the console did not start")
	}
	if !strings.Contains(rec.stderr, "TLS certificate verification is disabled") {
		t.Errorf("stderr at start = %q, want the TLS warning", rec.stderr)
	}
}

// TestBareEngramOpensConsole: a bare engram on a terminal reads the server
// from the environment and opens the console with the scopes it read.
func TestBareEngramOpensConsole(t *testing.T) {
	resetClientFlags(t)
	rec := forceTerminal(t)
	t.Setenv("ENGRAM_SERVER_URL", startStubServer(t, scopesStub()))
	if _, _, err := runClient(t, noArgs...); err != nil {
		t.Fatalf("bare engram: %v", err)
	}
	m, ok := rec.model.(*consoleModel)
	if !ok || len(m.scopes) != 1 || m.scopes[0].GetScope() != "repo:example/one" {
		t.Errorf("console model = %#v, want one scope from ListScopes", rec.model)
	}
}

// TestConsoleProgramQuitExitsZero: a program run that the model quits
// returns nil, so the process exits 0.
func TestConsoleProgramQuitExitsZero(t *testing.T) {
	m := newConsoleModel(context.Background(), nil, time.Second, &engramv1.ListScopesResponse{})
	err := runConsoleProgram(context.Background(), quitOnlyModel{m},
		tea.WithInput(strings.NewReader("q")), tea.WithOutput(io.Discard), tea.WithWindowSize(80, 24))
	if err != nil {
		t.Errorf("runConsoleProgram = %v, want nil", err)
	}
}

// quitOnlyModel wraps the console model with an Init that loads nothing, so
// the program run needs no server.
type quitOnlyModel struct{ *consoleModel }

func (quitOnlyModel) Init() tea.Cmd { return nil }
