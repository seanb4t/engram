// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import (
	"fmt"
	"maps"
)

// v1FillShortID is the v0->v1 step's ApplyMinterFunc: it ensures payload
// carries a non-empty "short_id" key. If short_id is already present and
// non-empty — the state a prior standalone Store.BackfillShortIDs run
// leaves behind — payload is returned unchanged and mint is NEVER called:
// an existing short_id is preserved verbatim, never re-minted, because
// minted short_ids may already be cited by get_memory/supersede_memory
// (D-03/REVIEWS.md H1). The CheckAdditive pre-existing-key carve-out is
// what makes this legal: short_id is declared in AddsKeys, and a
// pre-existing short_id satisfies that declaration without this step
// having added anything.
//
// Otherwise payload is cloned (never mutated in place — the two-clone
// discipline the sweep applies per step means this function must not rely
// on being handed an already-isolated map), short_id is set from mint(),
// and the clone is returned. A mint error is wrapped and returned; the
// caller (Store.Migrate) is responsible for failure-counting that record
// rather than aborting the whole sweep.
func v1FillShortID(payload map[string]any, mint func() (string, error)) (map[string]any, error) {
	if sid, ok := payload["short_id"]; ok {
		if s, ok := sid.(string); ok && s != "" {
			return payload, nil
		}
	}
	out := maps.Clone(payload)
	sid, err := mint()
	if err != nil {
		return nil, fmt.Errorf("migrate: v1FillShortID: mint short_id: %w", err)
	}
	out["short_id"] = sid
	return out, nil
}
