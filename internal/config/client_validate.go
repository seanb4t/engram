// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"errors"
	"fmt"
	"strconv"
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
