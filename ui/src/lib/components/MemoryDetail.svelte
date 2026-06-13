<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { ScrollArea } from '$lib/components/ui/scroll-area';
  import { toast } from 'svelte-sonner';
  import { relativeTime, fullTimestamp } from '$lib/time';
  import CopyIcon from '@lucide/svelte/icons/copy';
  let { memory, loading, error }: { memory: Memory | undefined; loading: boolean; error: unknown } = $props();
  const notFound = $derived(error instanceof ConnectError && error.code === Code.NotFound);
  const created = $derived(memory?.createdAt ? timestampDate(memory.createdAt) : undefined);
  async function copy() { if (memory) { await navigator.clipboard.writeText(memory.content); toast.success('copied'); } }
</script>

<div class="w-[360px] shrink-0 border-l border-border flex flex-col min-h-0">
  {#if loading}
    <div class="p-3 text-muted-foreground">loading…</div>
  {:else if notFound}
    <div class="p-3 text-muted-foreground">record not found</div>
  {:else if error}
    <div class="p-3 text-cat-gotcha">failed to load record</div>
  {:else if !memory}
    <div class="p-3 text-muted-foreground">select a record</div>
  {:else}
    <div class="flex items-center gap-2 p-3 border-b border-border">
      <Badge variant="outline" class="text-[10px] uppercase" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
      {#if created}<span class="text-[11px] text-muted-foreground" title={fullTimestamp(created)}>{relativeTime(created)}</span>{/if}
      <Button variant="outline" size="sm" class="ml-auto" aria-label="copy content" onclick={copy}><CopyIcon data-icon="inline-start" /> copy</Button>
    </div>
    <div class="p-3 border-b border-border flex flex-col gap-2">
      <div class="text-[11.5px] font-mono truncate" title={memory.scope}>{memory.scope}</div>
      <div class="flex gap-1.5 flex-wrap text-[10.5px]">
        <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">by</span> {memory.actor}</span>
        <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">src</span> {memory.source}</span>
        <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">vis</span> {memory.visibility}</span>
      </div>
    </div>
    <ScrollArea class="flex-1 min-h-0"><div class="p-3 text-[13px] leading-relaxed whitespace-pre-wrap">{memory.content}</div></ScrollArea>
    <div class="p-3 border-t border-border flex gap-1.5 flex-wrap">
      {#each memory.tags as t (t)}<span class="px-1.5 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
    </div>
  {/if}
</div>
