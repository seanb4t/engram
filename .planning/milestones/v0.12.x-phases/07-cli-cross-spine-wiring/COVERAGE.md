No external API integration: this phase wires engram's own CLI to engram's own first-party Connect API — both live in this repo, no third-party service, SDK, or vendor surface is involved.

## Why the detector fired

The `api-coverage` detector matched `connect` + `api` on the CONTEXT.md phrase "landed `cross_spine`
plus `searched_scopes` / `scopes_truncated` on the Connect API". That is `EngramService`, defined in
`proto/engram/v1/engram.proto` and served by `internal/server/connectapi.go` — first-party, in-tree,
and already fully implemented by v0.12.x Phase 3.

The full-coverage-by-default checkpoint exists to stop "we integrated the API" from silently meaning
"we integrated whatever the first use case exercised" against a **vendor** surface whose un-built
capabilities become invisible holes. That failure mode does not apply here: there is no vendor, and
the capability being reached (`cross_spine` on `SearchMemories` / `ListMemories`) is a single already-
shipped, already-tested field pair that this phase exists specifically to finish exposing.

Enumerating a matrix row per `EngramService` RPC would be fabrication — this phase deliberately touches
exactly two request literals and adds zero server capability. The scope fence in `07-CONTEXT.md`
(`<domain>`, as amended) is the real coverage decision, and it is already explicit and reasoned.
