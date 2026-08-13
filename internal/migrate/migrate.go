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
// registered migration step. It is 0 in this phase, for three reasons:
//
//  1. "Current" means the version produced by applying every registered
//     migration step. Phase 2 (this phase) creates no registry (Phase 3
//     does) and registers no steps (Phase 4 does), so the current schema is
//     the baseline schema: v0.
//  2. The milestone already recorded (.planning/STATE.md, 2026-08-12
//     roadmap decisions; ROADMAP Phase 4 success criterion 4) that
//     backfill-short-ids becomes the v0->v1 step. Shipping CurrentVersion =
//     1 here would force one of two contradictions when Phase 4 registers
//     it: either every record already written falsely claims to have
//     passed a step that never ran, or the step must be renumbered v1->v2
//     against a recorded decision.
//  3. payload() cannot honour a v1 claim. payload() omits short_id when
//     Memory.ShortID is empty, so the codec has no way to guarantee the
//     property that defines v1. Stamping 1 from payload() would be exactly
//     the false-currency claim rejected for partial writes elsewhere in
//     this store.
//
// Raising this constant is a Phase 3/4 action taken together with
// registering the step that defines the new version — never a standalone
// bump.
const CurrentVersion Version = 0
