// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package setupgen renders setup instructions from runtime-authored Plans.
package setupgen

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/seanb4t/engram/internal/setup"
	"github.com/seanb4t/engram/internal/surfaces"
)

// RegionID identifies the generated command tables in the slash command.
const RegionID = "setup-commands"

// Path is the canonical target, relative to the repository root.
const Path = "skill/engram/commands/engram-setup.md"

// PlanFunc is the runtime authoring seam; rendering never executes its actions.
type PlanFunc func(setup.Environment, setup.Options) (setup.Plan, error)

// PluginActionsFunc is the plugin-lane authoring seam: rendering calls it
// to obtain the exact write actions a runtime would issue for an observed
// plugin state and marketplace presence, but — mirroring PlanFunc's own
// generation-only contract — never executes any of them.
type PluginActionsFunc func(state setup.PluginState, marketplacePresent bool) []setup.Action

// claudeCodePluginActions type-asserts setup.ClaudeCode as
// setup.PluginRuntime and returns its PluginActions method: the plugin
// table's argv source is ALWAYS the real, exported PluginRuntime
// implementation (internal/setup/claudecode.go), never a re-typed literal
// in this package. A failed assertion means setup.ClaudeCode stopped
// implementing PluginRuntime — a programming error caught at generation
// time, never a runtime path, hence the panic rather than an error return.
func claudeCodePluginActions() PluginActionsFunc {
	pr, ok := setup.ClaudeCode.(setup.PluginRuntime)
	if !ok {
		panic("setupgen: setup.ClaudeCode does not implement setup.PluginRuntime")
	}
	return pr.PluginActions
}

// Case shares synthetic auth inputs between rendering and CLI conformance tests.
//
// Label is the Mode-column label both generated tables use. The four
// shipped auth-only cases use their auth mode verbatim (so their rows stay
// byte-identical), while a case that adds a header is labeled
// "<auth>+header" — distinct from any bare auth-mode label so the shipped
// rows are never mistaken for a header-bearing one.
type Case struct {
	Label          string
	Options        setup.Options
	DelegationArgs []string // Full preview argv, including engram; never --apply.
}

// Cases returns fresh values in the published auth-choice order, followed
// by the gateway-header case. CLI flag names belong to engram's Cobra
// command, not Plan; conformance tests validate them.
func Cases() []Case {
	cases := make([]Case, 0, 5)
	for _, auth := range []string{"oauth", "oauth-client", "bearer", "none"} {
		opts := setup.Options{URL: "https://engram.example.com/mcp", Auth: auth}
		args := []string{"engram", "setup", "--url", opts.URL, "--auth", opts.Auth}
		if auth == "oauth-client" {
			opts.ClientID = "example-client-id"
			args = append(args, "--client-id", opts.ClientID)
		}
		cases = append(cases, Case{Label: auth, Options: opts, DelegationArgs: args})
	}
	// The canonical gateway example (02-CONTEXT.md): a bearer server behind
	// an API gateway that also wants x-gateway-api-key. Chosen because it
	// shows the auth header AND the extra header on one line, in D-08 order
	// (auth-mode header first, extra headers after).
	headerOpts := setup.Options{
		URL:     "https://engram.example.com/mcp",
		Auth:    "bearer",
		Headers: []setup.HeaderSpec{{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"}},
	}
	headerArgs := []string{"engram", "setup", "--url", headerOpts.URL, "--auth", headerOpts.Auth,
		"--header", "x-gateway-api-key=GATEWAY_KEY"}
	cases = append(cases, Case{Label: "bearer+header", Options: headerOpts, DelegationArgs: headerArgs})
	return cases
}

// Render produces both tables atomically in memory. Only a synthetic HomeDir is
// available: unexpected environment reads or process calls fail generation.
// Each row's Mode cell carries the authoring Case's Label rather than its
// raw Options.Auth, so a case that adds an extra header renders under its
// own "<auth>+header" label without disturbing the four shipped auth-only
// rows. Extra headers are additional --header pairs on the SAME `claude mcp
// add` action (D-04/D-08) — never a second action — which is what keeps
// the exactly-one-add filter below true for the header case too.
func Render(planFn PlanFunc, pluginFn PluginActionsFunc) (string, error) {
	if planFn == nil {
		return "", fmt.Errorf("setupgen: nil Plan function")
	}
	var fallback, delegation strings.Builder
	fallback.WriteString("### Claude Code fallback registration\n\n| Mode | Command |\n| --- | --- |\n")
	delegation.WriteString("### Delegation preview\n\n| Mode | Command |\n| --- | --- |\n")
	for _, c := range Cases() {
		var accessErr error
		unexpected := func(operation string) error {
			accessErr = fmt.Errorf("setupgen: unexpected environment access: %s", operation)
			return accessErr
		}
		env := setup.Environment{
			HomeDir:  func() (string, error) { return "/synthetic/engram-setup-home", nil },
			LookPath: func(string) (string, error) { return "", unexpected("LookPath") },
			Getenv:   func(string) string { _ = unexpected("Getenv"); return "" },
			Run: func(context.Context, string, []string) (setup.RunResult, error) {
				return setup.RunResult{}, unexpected("Run")
			},
		}
		plan, err := planFn(env, c.Options)
		if accessErr != nil {
			return "", accessErr
		}
		if err != nil {
			return "", fmt.Errorf("setupgen: %s Plan: %w", c.Label, err)
		}
		var adds []setup.Action
		for _, action := range plan.Actions {
			if len(action.Args) >= 3 && action.Args[0] == "claude" && action.Args[1] == "mcp" && action.Args[2] == "add" {
				adds = append(adds, action)
			}
		}
		if len(adds) != 1 {
			return "", fmt.Errorf("setupgen: %s: expected exactly one claude mcp add action, got %d", c.Label, len(adds))
		}
		fmt.Fprintf(&fallback, "| `%s` | %s |\n", c.Label, commandCell(adds[0].Command()))
		fmt.Fprintf(&delegation, "| `%s` | %s |\n", c.Label, commandCell((setup.Action{Args: c.DelegationArgs}).Command()))
	}
	body := delegation.String() + "\n" + fallback.String()
	if pluginFn == nil {
		return body, nil
	}
	pluginTable, err := renderPluginTable(pluginFn)
	if err != nil {
		return "", err
	}
	return body + "\n" + pluginTable, nil
}

// pluginTableRow describes one fixed row of the plugin-delivery table:
// the observed (state, marketplacePresent) pair pluginFn is called with,
// and whether that state is expected to author at least one action.
type pluginTableRow struct {
	label              string
	state              setup.PluginState
	marketplacePresent bool
	wantActions        bool
}

// pluginTableRows is the FIXED, ORDERED set of states the plugin-delivery
// table renders — never derived from a live probe (rendering only ever
// sees synthetic Options, never a real Environment). PluginUnavailable is
// deliberately absent: its argv is always empty (no action authored),
// giving the reader nothing beyond what the `current` row already shows.
var pluginTableRows = []pluginTableRow{
	{"`absent` (marketplace absent)", setup.PluginAbsent, false, true},
	{"`absent` (marketplace present)", setup.PluginAbsent, true, true},
	{"`outdated`", setup.PluginOutdated, true, true},
	{"`current`", setup.PluginCurrent, true, false},
}

// renderPluginTable renders the third table — "### Claude Code plugin
// delivery (--apply)" — showing exactly what --apply runs for a
// plugin-capable Claude Code, by observed plugin state (D-01/D-02/D-04/
// D-06 argv, authored in claudecode.go, never re-typed here). It exists
// so /engram-setup's generated tables are never silently incomplete
// relative to what --apply actually does (03-RESEARCH.md Pitfall 7).
// Codex's own `codex plugin` argv are deliberately never rendered here:
// this package renders claude-code's Plan only (see Check/Write); the
// hand-authored prose around the anchored region points readers at the
// live preview for codex's equivalents. The renderer validates its own
// invariants rather than trusting pluginFn: a row whose wantActions is
// true but authored zero actions (or vice versa), or any authored action
// whose argv does not begin "claude plugin", fails generation outright —
// the renderer knows the shape it renders.
func renderPluginTable(pluginFn PluginActionsFunc) (string, error) {
	var b strings.Builder
	b.WriteString("### Claude Code plugin delivery (--apply)\n\n| Plugin state | Command |\n| --- | --- |\n")
	for _, row := range pluginTableRows {
		actions := pluginFn(row.state, row.marketplacePresent)
		n := len(actions)
		switch {
		case row.wantActions && n == 0:
			return "", fmt.Errorf("setupgen: plugin %s: expected %s, got %d action(s)", row.label, "actions", n)
		case !row.wantActions && n != 0:
			return "", fmt.Errorf("setupgen: plugin %s: expected %s, got %d action(s)", row.label, "no action", n)
		}
		for _, action := range actions {
			if len(action.Args) < 2 || action.Args[0] != "claude" || action.Args[1] != "plugin" {
				return "", fmt.Errorf("setupgen: plugin %s: action %q is not a claude plugin invocation", row.label, action.Command())
			}
		}
		cell := "(no action)"
		if n > 0 {
			cell = commandCell(setup.Plan{Actions: actions}.Display())
		}
		fmt.Fprintf(&b, "| %s | %s |\n", row.label, cell)
	}
	return b.String(), nil
}

// commandCell preserves shell quoting while escaping Markdown table syntax.
func commandCell(command string) string {
	fence := "`"
	for strings.Contains(command, fence) {
		fence += "`"
	}
	command = strings.ReplaceAll(command, "|", `\|`)
	if len(fence) > 1 {
		return fence + " " + command + " " + fence
	}
	return fence + command + fence
}

// readRegion requires exactly one canonical Markdown pair. The shared surfaces
// reader deliberately supports repeated regions for other consumers; setup has
// only one. Return raw region bytes too, because ReadRegion normalizes CRLF.
func readRegion(path string) (body, raw string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	text := string(data)
	startToken := "engram:rule:start " + RegionID
	endToken := "engram:rule:end " + RegionID
	if strings.Count(text, startToken) != 1 || strings.Count(text, endToken) != 1 {
		return "", "", fmt.Errorf("expected exactly one setup start and end anchor")
	}
	start := "<!-- " + startToken + " -->"
	end := "<!-- " + endToken + " -->"
	i, j := strings.Index(text, start), strings.Index(text, end)
	if i < 0 || j < i+len(start) {
		return "", "", fmt.Errorf("missing, malformed, or reversed setup anchors")
	}
	body, found, err := surfaces.ReadRegion(path, RegionID)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", fmt.Errorf("setup region not found")
	}
	return body, text[i+len(start) : j], nil
}

// Check compares the existing region to the real Claude Plan without writing.
func Check(path string) error {
	return check(path, setup.ClaudeCode.Plan, claudeCodePluginActions())
}

func check(path string, planFn PlanFunc, pluginFn PluginActionsFunc) error {
	if err := compare(path, planFn, pluginFn); err != nil {
		return fmt.Errorf("setupgen: check %s: %w; run task surfaces:gen after repairing any invalid anchors", path, err)
	}
	return nil
}

func compare(path string, planFn PlanFunc, pluginFn PluginActionsFunc) error {
	want, err := Render(planFn, pluginFn)
	if err != nil {
		return err
	}
	body, raw, err := readRegion(path)
	if err != nil {
		return err
	}
	// WriteRegion adds a line boundary on either side of the rendered body.
	// Keep the renderer's trailing newline: it separates the table and anchor.
	if body != want || raw != "\n"+want+"\n" {
		return fmt.Errorf("generated setup region has drifted")
	}
	return nil
}

// Write replaces only the existing anchored region using the real Claude Plan.
func Write(path string) error {
	return write(path, setup.ClaudeCode.Plan, claudeCodePluginActions())
}

func write(path string, planFn PlanFunc, pluginFn PluginActionsFunc) error {
	body, err := Render(planFn, pluginFn)
	if err != nil {
		return err
	}
	if _, _, err := readRegion(path); err != nil {
		return fmt.Errorf("setupgen: write %s: %w", path, err)
	}
	return surfaces.WriteRegion(path, RegionID, body)
}
