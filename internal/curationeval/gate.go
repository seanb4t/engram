// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"fmt"
	"os"
	"strconv"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
	"github.com/seanb4t/engram/internal/config"
)

// evalGateEnv is the env var gating this whole package's curation-verdict
// eval. evalPairsEnv names a second, independent local JSONL pair file for
// D-01's private real-spine mode (plan 03-07 Task 3) — it carries no
// boolean semantics of its own and is returned verbatim.
const (
	evalGateEnv  = "ENGRAM_CURATION_EVAL"
	evalPairsEnv = "ENGRAM_CURATION_EVAL_PAIRS"
)

// evalGateKey and evalPairsKey are the koanf keys resolveEvalGate reads
// after the env layer.
const (
	evalGateKey  = "curation_eval"
	evalPairsKey = "curation_eval_pairs"
)

// resolveEvalGate resolves ENGRAM_CURATION_EVAL and
// ENGRAM_CURATION_EVAL_PAIRS through a package-local koanf load: a
// default-false/default-empty layer, then production's ENGRAM_ prefix and
// env layer with the same empty-value-preserves-default rule config.Load
// uses (mirrors internal/retrievaleval.resolveEvalGate exactly). This is
// test-harness configuration deliberately never registered in
// internal/config (Phase 1 D-15) and it never runs production validation —
// it must never invoke config.Load/Config.Validate, so an unrelated
// malformed ENGRAM_* var (e.g. ENGRAM_EMBED_DIM=abc) can never fail this
// package while the eval is off. A malformed gate value itself errors
// rather than reading as off, so a mistyped enable is never a silent skip.
// Any strconv.ParseBool true spelling enables the gate. The second return
// value is ENGRAM_CURATION_EVAL_PAIRS's raw value (empty when unset),
// independent of whether the gate itself is enabled.
func resolveEvalGate(environ func() []string) (bool, string, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(map[string]any{evalGateKey: "false", evalPairsKey: ""}, "."), nil); err != nil {
		return false, "", fmt.Errorf("curation eval gate defaults: %w", err)
	}

	if err := k.Load(env.Provider(".", env.Opt{
		Prefix:      config.Prefix,
		EnvironFunc: environ,
		TransformFunc: func(key, val string) (string, any) {
			switch key {
			case evalGateEnv:
				if val == "" {
					// Empty preserves the default (mirrors config.Load).
					return "", nil
				}
				return evalGateKey, val
			case evalPairsEnv:
				return evalPairsKey, val
			default:
				// Every other ENGRAM_* var is ignored: this gate maps
				// exactly these two variables, never the whole registry.
				return "", nil
			}
		},
	}), nil); err != nil {
		return false, "", fmt.Errorf("curation eval gate env: %w", err)
	}

	raw := k.String(evalGateKey)
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, "", fmt.Errorf("%s: invalid value %q: %w", evalGateEnv, raw, err)
	}
	return enabled, k.String(evalPairsKey), nil
}

// curationEvalEnabled resolves the gate from the real process environment.
func curationEvalEnabled() (bool, string, error) {
	return resolveEvalGate(os.Environ)
}
