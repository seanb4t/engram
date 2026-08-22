// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package migrate holds the record schema-version type and the
// current-version constant. It is deliberately stdlib-only, with zero
// imports: the dependency direction is internal/store imports
// internal/migrate, never the reverse (D-04, REQ-migration-step-registry),
// so this leaf package can be referenced by store, proto-mapping, and CLI
// code without any cycle risk. Phase 3 grows the migration-step registry
// into this same package.
package migrate

// Version is a record's schema-version discriminator. It is a named type
// (not a bare int) so the monotonic stamping comparison in
// internal/store's payload() is type-checked, and a bare int cannot be
// passed where a version belongs. The zero value IS v0 IS absent: a record
// with no stored schema_version key decodes to Version(0), needs no
// backfill, and no pointer indirection is required to distinguish "unset"
// from "v0" — they are defined to be the same state (D-09).
type Version int

// CurrentVersion is the schema version produced by applying every
// registered migration step. It is 1 as of Phase 4, for three reasons:
//
//  1. "Current" means the version produced by applying every registered
//     migration step. Phase 4 registered the v0->v1 backfill-short-ids
//     step in migrate.Registry, so the current schema is v1.
//  2. The milestone recorded (.planning/STATE.md, 2026-08-12 roadmap
//     decisions; ROADMAP Phase 4 success criterion 4) that
//     backfill-short-ids becomes the v0->v1 step. The constant and the
//     step landed in the SAME change (this is that change), so neither of
//     the two contradictions a standalone bump would have forced — a
//     falsely-claimed pass, or a renumbered step against a recorded
//     decision — arises.
//  3. payload() cannot, by itself, honour a v1 claim: it unconditionally
//     stamps int(max(CurrentVersion, m.SchemaVersion)) (store.go:646)
//     while omitting short_id when Memory.ShortID is empty (store.go:
//     658-659), so the CODEC ALONE cannot guarantee the property that
//     defines v1. What makes the v1 stamp honest is WHERE the guarantee
//     actually lives: the server layer mints a short_id before every
//     write, at internal/server/tools.go:1144 (persistAndEnqueue), :1287,
//     and :2097 — production paths mint first, so a stamped v1 record
//     really does carry a short_id. The residual: a DIRECT Store.Upsert
//     with an empty ShortID — a test, or a future non-server caller —
//     stamps a v1 claim the record does not satisfy, and because that
//     record is already at-target no sweep will revisit it. This is a
//     documented robustness boundary, not a live defect; recording it here
//     is what keeps the false-currency class visible.
//
// Raising this constant is a Phase 3/4 action taken together with
// registering the step that defines the new version — never a standalone
// bump. This is that change: migrate.Registry's v0->v1 step and this
// constant's 0->1 raise landed together.
const CurrentVersion Version = 1
