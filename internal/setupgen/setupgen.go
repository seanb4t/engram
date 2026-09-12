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

// Case shares synthetic auth inputs between rendering and CLI conformance tests.
type Case struct {
	Options        setup.Options
	DelegationArgs []string // Full preview argv, including engram; never --apply.
}

// Cases returns fresh values in the published auth-choice order. CLI flag names
// belong to engram's Cobra command, not Plan; conformance tests validate them.
func Cases() []Case {
	cases := make([]Case, 0, 4)
	for _, auth := range []string{"oauth", "oauth-client", "bearer", "none"} {
		opts := setup.Options{URL: "https://engram.example.com/mcp", Auth: auth}
		args := []string{"engram", "setup", "--url", opts.URL, "--auth", opts.Auth}
		if auth == "oauth-client" {
			opts.ClientID = "example-client-id"
			args = append(args, "--client-id", opts.ClientID)
		}
		cases = append(cases, Case{Options: opts, DelegationArgs: args})
	}
	return cases
}

// Render produces both tables atomically in memory. Only a synthetic HomeDir is
// available: unexpected environment reads or process calls fail generation.
func Render(planFn PlanFunc) (string, error) {
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
			return "", fmt.Errorf("setupgen: %s Plan: %w", c.Options.Auth, err)
		}
		var adds []setup.Action
		for _, action := range plan.Actions {
			if len(action.Args) >= 3 && action.Args[0] == "claude" && action.Args[1] == "mcp" && action.Args[2] == "add" {
				adds = append(adds, action)
			}
		}
		if len(adds) != 1 {
			return "", fmt.Errorf("setupgen: %s: expected exactly one claude mcp add action, got %d", c.Options.Auth, len(adds))
		}
		fmt.Fprintf(&fallback, "| `%s` | %s |\n", c.Options.Auth, commandCell(adds[0].Command()))
		fmt.Fprintf(&delegation, "| `%s` | %s |\n", c.Options.Auth, commandCell((setup.Action{Args: c.DelegationArgs}).Command()))
	}
	return delegation.String() + "\n" + fallback.String(), nil
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
	return check(path, setup.ClaudeCode.Plan)
}

func check(path string, planFn PlanFunc) error {
	if err := compare(path, planFn); err != nil {
		return fmt.Errorf("setupgen: check %s: %w; run task surfaces:gen after repairing any invalid anchors", path, err)
	}
	return nil
}

func compare(path string, planFn PlanFunc) error {
	want, err := Render(planFn)
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
	return write(path, setup.ClaudeCode.Plan)
}

func write(path string, planFn PlanFunc) error {
	body, err := Render(planFn)
	if err != nil {
		return err
	}
	if _, _, err := readRegion(path); err != nil {
		return fmt.Errorf("setupgen: write %s: %w", path, err)
	}
	return surfaces.WriteRegion(path, RegionID, body)
}
