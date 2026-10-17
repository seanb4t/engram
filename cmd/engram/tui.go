// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"connectrpc.com/connect"
	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/config"
)

// tuiCmd opens the terminal console with the client flags. A bare `engram`
// runs the same entry point with no flags, so it reads only the
// environment and the saved preferences; this command exists because
// --token-file and --insecure have no environment form (D-13, D-14).
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the read-only terminal console for browsing memories",
	Long: "Open the read-only terminal console for browsing memories. A bare engram opens the same " +
		"console with settings from the environment only. The console needs a terminal on stdin and " +
		"stdout; agents and scripts read the machine-readable catalog with engram catalog.",
	Args: noPositionalArgs,
	RunE: runConsole,
}

func init() {
	addConnectionFlags(tuiCmd)
	f := tuiCmd.Flags()
	f.String("theme", config.FlagDefault("theme"),
		"console theme: "+strings.Join(config.TUIThemes, ", ")+
			" (default: ENGRAM_TUI_THEME, then the theme chosen in the console, then engram)")
	f.Bool("mouse", false,
		"turn on mouse support: click to select, wheel to scroll "+
			"(default: ENGRAM_TUI_MOUSE, then the choice saved in the console, then off)")
	rootCmd.AddCommand(tuiCmd)
}

// stdioIsTerminal reports whether stdin and stdout are both terminals. The
// console reads key presses from stdin, so a redirected stdin would hang it
// or quit it on end of file. A test replaces this to reach the startup path
// without a pseudo-terminal.
var stdioIsTerminal = func() bool {
	return isTerminal(os.Stdin) && isTerminal(os.Stdout)
}

// runConsoleProgram runs the console until the person quits. A test
// replaces it to observe startup without taking over a terminal.
var runConsoleProgram = func(ctx context.Context, m tea.Model, opts ...tea.ProgramOption) error {
	opts = append([]tea.ProgramOption{tea.WithContext(ctx)}, opts...)
	_, err := tea.NewProgram(m, opts...).Run()
	// Ctrl-C arrives as a key press in raw mode and quits cleanly; a SIGINT
	// from outside the terminal is the same request, so it exits 0 too.
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}

// consoleColorProfile is the colour profile the program is forced to, if
// any: NO_COLOR (any non-empty value) means text attributes only.
func consoleColorProfile(getenv func(string) string) (colorprofile.Profile, bool) {
	if getenv("NO_COLOR") != "" {
		return colorprofile.Ascii, true
	}
	return 0, false
}

// runConsole is the entry point for a bare `engram` and for `engram tui`.
// Every check that can fail runs before the console takes over the screen,
// so a failure leaves the terminal as it was and exits with the same code
// and stderr message a client command gives for it.
func runConsole(cmd *cobra.Command, _ []string) error {
	if !stdioIsTerminal() {
		what := cmd.CommandPath()
		if cmd == cmd.Root() {
			what += " with no command opens the terminal console, which"
		}
		return usageErrorf("%s needs a terminal on stdin and stdout; "+
			"for the machine-readable catalog run: engram catalog (for help: engram --help)", what)
	}
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		return usageErrorf("load client configuration: %w", err)
	}
	if err := config.ValidateConsole(cfg.Client); err != nil {
		return usageErrorf("%w", err)
	}
	client, _, timeout, err := clientFromFlags(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	// The scope picker needs this anyway, it is the cheapest read, and it
	// goes through authentication: a bad credential or a dead server fails
	// here, not inside the full-screen program.
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	scopes, err := client.ListScopes(callCtx, connect.NewRequest(&engramv1.ListScopesRequest{}))
	cancel()
	if err != nil {
		return wrapRPCError(err)
	}
	settings := loadConsoleSettings(cfg.Client, os.Getenv)
	var opts []tea.ProgramOption
	if p, ok := consoleColorProfile(os.Getenv); ok {
		opts = append(opts, tea.WithColorProfile(p))
	}
	return runConsoleProgram(ctx, newConsoleModel(ctx, client, timeout, scopes.Msg, settings), opts...)
}
