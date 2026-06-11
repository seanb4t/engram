<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { ConnectError, Code } from '@connectrpc/connect';
  let { memory, loading, error }: { memory: Memory | undefined; loading: boolean; error: unknown } = $props();
  // Distinguish "record not found" from a genuine load failure so the message
  // isn't misleading (af5.11).
  const notFound = $derived(error instanceof ConnectError && error.code === Code.NotFound);
</script>

<div class="p-3" style="width:300px">
  {#if loading}
    <div class="eg-muted">loading…</div>
  {:else if notFound}
    <div class="eg-muted">record not found</div>
  {:else if error}
    <div class="eg-error">failed to load record — retry</div>
  {:else if !memory}
    <div class="eg-muted">select a record</div>
  {:else}
    <div style="color:var(--cat-{memory.category});font-weight:700">{memory.category}</div>
    <div class="my-2">{memory.content}</div>
    <div class="eg-label">Metadata</div>
    <div style="display:grid;grid-template-columns:auto 1fr;gap:2px 10px;color:var(--muted)">
      <span>scope</span><span style="color:var(--foreground)">{memory.scope}</span>
      <span>source</span><span style="color:var(--foreground)">{memory.source}</span>
      <span>actor</span><span style="color:var(--foreground)">{memory.actor}</span>
      <span>created</span><span style="color:var(--foreground)">{memory.createdAt ? timestampDate(memory.createdAt).toISOString().slice(0, 10) : ''}</span>
      <span>visibility</span><span style="color:var(--accent)">{memory.visibility}</span>
    </div>
    <div class="mt-2">{memory.tags.map((t) => '#' + t).join(' ')}</div>
  {/if}
</div>
