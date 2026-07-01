---
title: Reindex (embedder migration)
description: Migrate memories to an embedder with a different vector dimension by re-embedding into a fresh Qdrant collection with engram reindex — including --source, --resume, and the cutover flow.
---

Qdrant fixes a collection's vector size at creation, and vectors from different
embedding models are not comparable. So switching to an embedder with a
**different output dimension** (`ENGRAM_EMBED_DIM`) — or a different model at the
same dimension — cannot happen in place. `engram reindex` re-embeds every stored
memory into a **new** collection at the currently-configured embedder's
dimension, leaving the old collection untouched so you can verify before cutting
over.

If you are keeping the same embedder, you never need this command.

## The migration flow

1. **Point engram at the new embedder.** Set the embedder variables (see
   [Configuration](/guides/configure/)) to the new model/endpoint and its
   dimension:

   ```sh
   export ENGRAM_EMBED_MODEL=ollama/new-model
   export ENGRAM_EMBED_DIM=768                 # the new model's output dimension
   export ENGRAM_OPENAI_BASE_URL=http://localhost:4000
   ```

2. **Dry-run to sanity-check the count.** This scans the source and reports the
   number of records found — an upper bound on what a real run re-embeds, since
   records with no content are skipped rather than embedded — without creating
   the target or writing anything:

   ```sh
   engram reindex --target memory_v2 --dry-run
   ```

3. **Run the reindex.** This creates the target collection at the new dimension
   and populates it. The source is never modified:

   ```sh
   engram reindex --target memory_v2
   ```

4. **Verify the target,** then **cut over** by pointing
   `ENGRAM_QDRANT_COLLECTION` at the new collection and restarting the server:

   ```sh
   export ENGRAM_QDRANT_COLLECTION=memory_v2
   # restart `engram serve`
   ```

The final summary line names the exact cutover command for the target you used.

## Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--target` | `ENGRAM_REINDEX_TARGET` | **Required.** New collection to create and populate. Must differ from the source. |
| `--source` | `ENGRAM_QDRANT_COLLECTION` | Collection to read from. Set it to reindex an arbitrary collection without changing env. |
| `--dry-run` | `false` | Scan and count only — no target is created and nothing is written. |
| `--resume` | `false` | Skip target points that already hold identical content, so an interrupted run restarts cheaply (see below). |
| `--timeout` | `30m` | Wall-clock bound for the whole run; `0` means no deadline. `Ctrl-C` / `SIGTERM` aborts either way. |

The two source-related defaults resolve differently: `--target` reads the
`ENGRAM_REINDEX_TARGET` env var as its flag default (so `--help` shows that
value), while `--source` defaults to empty at the flag and the server resolves an
empty value to `ENGRAM_QDRANT_COLLECTION`.

## Output

The single machine-parseable **summary** goes to **stdout**:

```
re-embedded 1240/1245 record(s) into "memory_v2" at dim 768 (5 skipped, no content; 0 unchanged); source left untouched — verify, then set ENGRAM_QDRANT_COLLECTION=memory_v2 and restart to cut over
```

Per-batch **progress** goes to **stderr**, so it never pollutes that summary
line:

```
reindex progress: scanned 256, upserted 256, skipped 0, unchanged 0
```

A record carrying no content is counted as `skipped` (nothing to embed) rather
than written as a meaningless vector.

## Resuming an interrupted run

Reindex is not transactional: an embed or upsert error part-way through leaves
the target partially populated. Because upserts are keyed by point id, simply
re-running the same command is safe and idempotent — but by default it
re-embeds every record again.

`--resume` makes that restart cheap. Before embedding each record, engram checks
the target for that id and **skips any record whose content is unchanged**
(reported as `unchanged` in the counts), re-embedding only what is new or
changed:

```sh
engram reindex --target memory_v2 --resume
```

The skip test is content equality — equal content re-embeds to an equal vector
under the same embedder — so no extra bookkeeping is stored and the copied
payload stays byte-for-byte identical.

:::caution[Verify before you cut over]
`reindex` never mutates the source collection, so a bad run costs nothing but
time — but the cutover (`ENGRAM_QDRANT_COLLECTION` + restart) is the point of no
return for live traffic. Confirm the target's record count and spot-check recall
against it before switching.
:::

## See also

- [Configuration](/guides/configure/) — the `ENGRAM_EMBED_*`, `ENGRAM_OPENAI_*`,
  and `ENGRAM_QDRANT_COLLECTION` variables this flow depends on.
- [Upgrade Guide](/guides/upgrade/) — version-specific behavioral changes,
  including back-filling summaries after a migration.
