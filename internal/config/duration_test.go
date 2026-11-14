// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"
)

func TestValidatePositiveDurationText(t *testing.T) {
	cases := []struct {
		env, value, want string
	}{
		{"ENGRAM_QDRANT_SCHEMA_TIMEOUT", "0s", `ENGRAM_QDRANT_SCHEMA_TIMEOUT "0s": must be a positive duration`},
		{"ENGRAM_QDRANT_SCHEMA_TIMEOUT", "120", `ENGRAM_QDRANT_SCHEMA_TIMEOUT "120": must be a Go duration (e.g. 30s, 2m): time: missing unit in duration "120"`},
		{"ENGRAM_QDRANT_STARTUP_TIMEOUT", "0", `ENGRAM_QDRANT_STARTUP_TIMEOUT "0": must be a positive duration`},
		{"ENGRAM_QDRANT_STARTUP_TIMEOUT", "abc", `ENGRAM_QDRANT_STARTUP_TIMEOUT "abc": must be a Go duration (e.g. 30s, 2m): time: invalid duration "abc"`},
	}
	for _, tc := range cases {
		err := validatePositiveDuration(tc.value, tc.env)
		if err == nil || err.Error() != tc.want {
			t.Errorf("validatePositiveDuration(%q, %s) = %v, want %q", tc.value, tc.env, err, tc.want)
		}
	}
}

func TestValidateStartupTimeoutAccepted(t *testing.T) {
	for _, v := range []string{"1ns", "1s", "60s"} {
		c := validConfig()
		c.Qdrant.StartupTimeout = v
		if err := c.Validate(); err != nil {
			t.Errorf("Validate with ENGRAM_QDRANT_STARTUP_TIMEOUT=%q = %v, want nil", v, err)
		}
	}
}

func TestPropertyValidatePositiveDuration(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := time.Duration(rapid.Int64().Draw(t, "d"))
		err := validatePositiveDuration(d.String(), "ENGRAM_QDRANT_STARTUP_TIMEOUT")
		if (err != nil) != (d <= 0) {
			t.Fatalf("validatePositiveDuration(%q) = %v, want error exactly when d <= 0", d.String(), err)
		}
		if err != nil && !strings.Contains(err.Error(), "ENGRAM_QDRANT_STARTUP_TIMEOUT") {
			t.Fatalf("error %q does not name the variable", err)
		}
	})
}
