// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"fmt"
	"os"
	"strconv"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
	"github.com/seanb4t/engram/internal/config"
)

// evalGateEnv is the env var gating this whole package's retrieval eval.
const evalGateEnv = "ENGRAM_RETRIEVAL_EVAL"

// evalGateKey is the koanf key resolveEvalGate reads after the env layer.
const evalGateKey = "retrieval_eval"

// resolveEvalGate resolves ENGRAM_RETRIEVAL_EVAL through a package-local
// koanf load: a default-false layer, then production's ENGRAM_ prefix and
// env layer with the same empty-value-preserves-default rule config.Load
// uses. This is a deliberate departure from internal/config's field
// registry: the registry excludes test-only vars (registry.go's header
// comment, TestCheckLegacyIgnoresTestOnlyVar), and this gate exists only for
// this package's own tests (2026-09-22.01 Phase 1 D-15, revised after
// research — RESEARCH.md's registry recommendation is superseded by the
// user-confirmed CONTEXT.md decision). It must never invoke the production
// config validation step, so an unrelated malformed ENGRAM_* var (e.g.
// ENGRAM_EMBED_DIM=abc) can never fail this package while the eval is off
// (T-01-01). A malformed gate
// value itself errors rather than reading as off — the same rule as
// storetest.RequireQdrant — so a mistyped enable is never a silent skip.
// Any strconv.ParseBool true spelling enables it, a compatible
// generalization of the historical bare "1".
func resolveEvalGate(environ func() []string) (bool, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(map[string]any{evalGateKey: "false"}, "."), nil); err != nil {
		return false, fmt.Errorf("retrieval eval gate defaults: %w", err)
	}

	if err := k.Load(env.Provider(".", env.Opt{
		Prefix:      config.Prefix,
		EnvironFunc: environ,
		TransformFunc: func(key, val string) (string, any) {
			if key != evalGateEnv || val == "" {
				// Empty preserves the default (mirrors config.Load), and
				// every other ENGRAM_* var is ignored: this gate maps
				// exactly one variable, never the whole registry.
				return "", nil
			}
			return evalGateKey, val
		},
	}), nil); err != nil {
		return false, fmt.Errorf("retrieval eval gate env: %w", err)
	}

	raw := k.String(evalGateKey)
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: invalid value %q: %w", evalGateEnv, raw, err)
	}
	return enabled, nil
}

// retrievalEvalEnabled resolves the gate from the real process environment.
func retrievalEvalEnabled() (bool, error) {
	return resolveEvalGate(os.Environ)
}
