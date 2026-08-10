# API Coverage — Phase 03.1 (Merge Supersession)

No external API integration: this phase generalizes engram's own `supersede_memory` MCP verb from one
target to a set, entirely within `internal/store` and `internal/server`; Qdrant is the project's
existing datastore, not a new external service being integrated, and no new client, SDK, endpoint, or
third-party surface is added.
