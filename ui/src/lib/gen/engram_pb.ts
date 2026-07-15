// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Hand-authored re-export barrel — NOT generated. Keeps client.ts's
// `./gen/engram_pb` import stable while the canonical generated output lives
// at ./engram/v1/engram_pb (structure-preserving vendored copy of
// gen/ts/engram/v1/engram_pb.ts via `task proto:gen`). Never overwritten by
// the re-vendor's `cp -R` (which only touches the engram/ and buf/
// subtrees) and never hand-edited to match generated content.
export * from './engram/v1/engram_pb';
