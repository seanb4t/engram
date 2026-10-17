// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ValidateClient reports whether c's fields are well-formed. It is pure
// (no I/O) and aggregates every failure via errors.Join, mirroring
// Validate's own aggregation shape, so a caller sees all problems at once.
//
// This is deliberately a SEPARATE function from Config.Validate, not a call
// folded into it (see ClientConfig's doc comment for why): this package's
// tests build ~33 hand-keyed Config{} literals that bypass Load entirely, so
// a client field checked inside Config.Validate would reach it as the zero
// value "" and turn every one of those previously-green tests red.
func ValidateClient(c ClientConfig) error {
	var errs []error

	// D-05: 0 is a rejected usage error, never treated as unbounded. This is
	// the one line that diverges from Embed.Timeout/Summarize.Timeout's own
	// "0 disables the timeout" convention — see ClientConfig.Timeout's doc
	// comment for why the two conventions must coexist under the same flag
	// name across this binary's commands.
	switch d, err := time.ParseDuration(c.Timeout); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Timeout, err))
	case d <= 0:
		errs = append(errs, fmt.Errorf("ENGRAM_TIMEOUT %q: must be greater than 0 — a client timeout of 0 is not treated as unbounded", c.Timeout))
	}

	if _, err := strconv.ParseBool(c.Insecure); err != nil {
		errs = append(errs, fmt.Errorf("--insecure %q: must be a boolean: %w", c.Insecure, err))
	}

	if err := ValidateOutputFormat(c.Output); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid client configuration: %w", errors.Join(errs...))
}

// ValidateOutputFormat reports whether v is a legal --output value: "json",
// "text", or "" (empty — detect from stdout). This is the single exported
// validator both the client tier (via ValidateClient above) and the
// operator tier's `--output` flag (cmd/engram/operator_output.go's
// operatorOutputFormat) call, so the accepted vocabulary and the error
// wording exist exactly once across both lanes rather than two
// hand-maintained switches that could silently drift apart.
func ValidateOutputFormat(v string) error {
	switch v {
	case "json", "text", "":
		return nil
	default:
		return fmt.Errorf(`--output %q: must be "json", "text", or empty`, v)
	}
}

// ValidateSetupAuth reports whether v is a legal `engram setup --auth`
// value: "oauth", "oauth-client", "bearer", "none", or "" (empty means "use
// the default" — the caller, cmd/engram/setup.go, defaults an empty value
// before passing it to internal/setup.Options.Auth). These four non-empty
// values are taken verbatim from the table already shipped at
// skill/engram/commands/engram-setup.md (D-03), so a later generated-
// equivalence proof compares like to like rather than reconciling two
// vocabularies. Shaped identically to ValidateOutputFormat above: a bad
// value routes to usageErrorf -> exitUsage (2), the same shape --output
// gets, and the error names every accepted mode so --help-driven
// correction needs no second lookup.
func ValidateSetupAuth(v string) error {
	switch v {
	case "oauth", "oauth-client", "bearer", "none", "":
		return nil
	default:
		return fmt.Errorf(`--auth %q: must be "oauth", "oauth-client", "bearer", "none", or empty`, v)
	}
}

// TUIThemes are the terminal console's theme names, in the order its
// picker lists them. The console defines each theme; this list is the
// contract its settings are validated against.
var TUIThemes = []string{"engram", "terminal", "catppuccin", "tokyo-night"}

// ValidateConsole checks the terminal console's own settings. It is kept
// apart from ValidateClient so a theme or mouse value meant for the
// console never fails another client command. Empty values are valid and
// mean "not set".
func ValidateConsole(c ClientConfig) error {
	var errs []error
	if c.TUITheme != "" && !slices.Contains(TUIThemes, c.TUITheme) {
		errs = append(errs, fmt.Errorf("--theme or ENGRAM_TUI_THEME %q: unknown theme; valid themes: %s",
			c.TUITheme, strings.Join(TUIThemes, ", ")))
	}
	if c.TUIMouse != "" {
		if _, err := strconv.ParseBool(c.TUIMouse); err != nil {
			errs = append(errs, fmt.Errorf("--mouse or ENGRAM_TUI_MOUSE %q: must be a boolean (true or false)", c.TUIMouse))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid console configuration: %w", errors.Join(errs...))
}
