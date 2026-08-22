// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

// Decoder is an OPTIONAL, SEPARATE interface a future migration step may
// satisfy to attach per-version decoding logic (D-11). Step does NOT
// implement Decoder today, and is not required to: nothing in this phase
// needs to decode anything. A future phase that DOES need decoding embeds
// Step in its own type, adds a DecodeAt method, and reaches it at the point
// of use with a type assertion — `if d, ok := any(x).(Decoder); ok`. This
// changes no existing call site and no existing signature.
//
// D-11 explicitly rejects the alternative: threading a nil-able Decoder
// parameter through NewStep now. That would make every call site carry a
// parameter that means nothing until a customer for it exists — a step
// with no decoding need would pass nil forever, and NewStep's signature
// would grow a parameter whose only current value is documentation.
//
// The payload shape is map[string]any, not a richer record type, because
// this package is a stdlib-only leaf (SC1) and cannot know internal/store's
// Memory type without breaking that leaf property. A future phase that
// needs a richer shape defines its own interface rather than widening this
// one — see TestMigratePackageIsStdlibOnlyLeaf.
type Decoder interface {
	// DecodeAt decodes raw — a payload map for a record observed at
	// version v — into whatever shape the implementing step needs.
	DecodeAt(v Version, raw map[string]any) (map[string]any, error)
}
