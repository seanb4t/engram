// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"cmp"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/decide/jev"
)

// deciderFromConfig builds the typed-decision Decider from an already-loaded
// config. An empty provider returns (nil, nil): it constructs nothing and
// touches no network (D-01). Provider "jev" builds the Jev backend with the
// key-only fallback resolved here at the wiring seam — cfg.Decisions.APIKey
// wins when set, otherwise cfg.OpenAI.APIKey (D-03), mirroring
// summarizerFromConfig's ChatAPIKey precedent. The base URL does NOT get this
// treatment: it fails Config.Validate when empty and provider=jev instead of
// falling back here. Any other provider value is a configuration error.
func deciderFromConfig(cfg *config.Config) (decide.Decider, error) {
	switch cfg.Decisions.Provider {
	case "":
		return nil, nil
	case "jev":
		apiKey := cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)
		return jev.New(cfg.Decisions.BaseURL, apiKey, cfg.Decisions.Model,
			jev.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
		), nil
	default:
		return nil, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: unknown provider (want \"\" or \"jev\")", cfg.Decisions.Provider)
	}
}
