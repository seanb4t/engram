// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package retrievaleval measures retrieval quality end-to-end. It seeds a
// labeled query -> expected-record dataset (including the permanent GitHub
// #261 regression fixture) through the exact production doc-embed sequence
// (store.EmbedText -> embed.Client.Embed -> store.Store.Upsert), searches it
// through the production query path (embed.Client.EmbedQuery ->
// store.Store.Search), and reports recall@k / MRR plus the #261 baseline rank
// and raw-score gap (diagnostic).
//
// The whole package is gated behind ENGRAM_RETRIEVAL_EVAL=1 (see the
// `task eval:retrieval` Taskfile target). TestMain short-circuits on that gate
// as its first statement, before any testcontainer/Docker startup, so the
// required `go test ./...` CI job pays zero additional Docker/Qdrant cost from
// this package when the gate is unset.
package retrievaleval
