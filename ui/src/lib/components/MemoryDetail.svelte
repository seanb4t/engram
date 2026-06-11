<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  let { memory, loading, error }: { memory: Memory | undefined; loading: boolean; error: unknown } = $props();
</script>

<div class="p-3" style="width:300px">
  {#if loading}
    <div style="color:var(--muted)">loading…</div>
  {:else if error}
    <div style="color:var(--cat-gotcha)">failed to load</div>
  {:else if !memory}
    <div style="color:var(--muted)">select a record</div>
  {:else}
    <div style="color:var(--cat-{memory.category});font-weight:700">{memory.category}</div>
    <div class="my-2">{memory.content}</div>
    <div style="color:var(--muted);text-transform:uppercase;font-size:10px">Metadata</div>
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
