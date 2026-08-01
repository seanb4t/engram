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
| `--dry-run` | `false` | Scan and count only — no target is created and nothing is written. Combined with `--resume` against a target that already exists, the count splits into would-re-embed and would-skip instead of one flat total (see [Repairing a pre-patch resume](#repairing-a-pre-patch-resume)). |
| `--resume` | `false` | Skip target points whose content, tags, and embedder identity all still match the source, so an interrupted run restarts cheaply (see below). |
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

Used alone, `--dry-run` reports a flat scan count, unchanged from prior
releases:

```
dry-run: 1245 record(s) would be re-embedded into "memory_v2" at dim 768
```

Combined with `--resume` against a target that already exists, `--dry-run`
sizes the repair instead — a would-re-embed/would-skip split:

```
dry-run --resume: 5 would be re-embedded, 1235 would be skipped (unchanged), 5 skipped (no content), 1245 scanned, into "memory_v2" at dim 768
```

Against a target that does not yet exist, `--dry-run --resume` reports every
scanned record as would-re-embed and creates nothing.

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

`--resume` makes that restart cheap. Before embedding each record, engram
checks the target for that id and **skips it only when three things all still
match**: the content, the tag set (compared without regard to order), and the
embedder identity used to write the target record. Any one of the three
differing re-embeds the record; all three matching skips it (reported as
`unchanged` in the counts):

```sh
engram reindex --target memory_v2 --resume
```

No extra bookkeeping is stored — the comparison reads straight off the target
payload — and the copied payload stays byte-for-byte identical.

One residual is deliberate: the embedded text folds tags in slice order (the
text handed to the embedder ends with `"\n\ntags: a, b"`), so a record whose
tags were only reordered embeds to different text yet is intentionally
treated as unchanged and skipped. Normalizing tag order at write time would
remove this residual, but it touches every write path and every stored
record and is out of scope here.

:::caution[Verify before you cut over]
`reindex` never mutates the source collection, so a bad run costs nothing but
time — but the cutover (`ENGRAM_QDRANT_COLLECTION` + restart) is the point of no
return for live traffic. Confirm the target's record count and spot-check recall
against it before switching.
:::

## Repairing a pre-patch resume

**What went wrong.** A `--resume` run on a version before this fix compared
only content, so a record whose tags were edited while its content stayed the
same was reported unchanged and kept the vector it had before the tag edit.
Nothing errored; the counts looked healthy.

**The mechanism.** `reindex` re-scrolls the source collection fresh on every
invocation and compares the source's current content and tags against what
the target holds. Vector and payload are written together by one upsert built
from that one source read, so a target record can never hold a vector from
one revision and a payload from another. The target does not self-correct and
has no memory of what it should look like; the source is authoritative and
re-read every time. That is why re-running the patched `--resume` heals the
affected records, and it is exactly why the limit below exists.

**The procedure.** Size it first — the would-re-embed count is the blast
radius:

```sh
engram reindex --target memory_v2 --resume --dry-run
```

Then run the same command without `--dry-run`:

```sh
engram reindex --target memory_v2 --resume
```

There is no separate repair command and none is needed: the patched resume
is the repair path.

**The limit.** If the source collection was deleted after a previous cutover,
the correct tags are gone — they live only in the source. Re-embedding from
the live target payload would produce a vector consistent with the *stale*
tags: silently wrong while appearing healed. There is deliberately no command
for this case. The recovery is to re-derive the affected records from
wherever they were originally authored, or to accept the stale vectors. An
operator who has deleted their source collection and runs `--resume` gets
silent skips, not a heal.

## See also

- [Configuration](/guides/configure/) — the `ENGRAM_EMBED_*`, `ENGRAM_OPENAI_*`,
  and `ENGRAM_QDRANT_COLLECTION` variables this flow depends on.
- [Upgrade Guide](/guides/upgrade/) — version-specific behavioral changes,
  including back-filling summaries after a migration.
