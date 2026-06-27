<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { ScrollArea } from '$lib/components/ui/scroll-area';
  import * as Tabs from '$lib/components/ui/tabs';
  import { toast } from 'svelte-sonner';
  import { relativeTime, fullTimestamp } from '$lib/time';
  import { renderMarkdown } from '$lib/markdown';
  import CopyIcon from '@lucide/svelte/icons/copy';
  let { memory, loading, error }: { memory: Memory | undefined; loading: boolean; error: unknown } = $props();
  const notFound = $derived(error instanceof ConnectError && error.code === Code.NotFound);
  const created = $derived(memory?.createdAt ? timestampDate(memory.createdAt) : undefined);
  const hasSummary = $derived(!!memory?.summary?.trim());
  const defaultTab = $derived(hasSummary ? 'summary' : 'content');
  const bodyHtml = $derived(memory ? renderMarkdown(memory.content) : '');
  async function copy() {
    if (!memory) return;
    try {
      await navigator.clipboard.writeText(memory.content);
      toast.success('copied');
    } catch {
      // clipboard write can reject (denied permission, insecure context, lost focus);
      // surface it so the button never appears to silently do nothing.
      toast.error('copy failed');
    }
  }
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
      <span class="cat-dot" style="background:var(--cat-{memory.category})"></span>
      <Badge variant="outline" class="text-[10px] uppercase" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
      {#if created}<span class="text-[11px] text-muted-foreground" title={fullTimestamp(created)}>{relativeTime(created)}</span>{/if}
      <Button variant="outline" size="sm" class="ml-auto" aria-label="copy content" onclick={copy}><CopyIcon data-icon="inline-start" /> copy</Button>
    </div>
    <Tabs.Root value={defaultTab} class="flex-1 flex flex-col min-h-0">
      <Tabs.List class="mx-3 mt-2">
        <Tabs.Trigger value="summary">Summary</Tabs.Trigger>
        <Tabs.Trigger value="content">Content</Tabs.Trigger>
        <Tabs.Trigger value="meta">Meta</Tabs.Trigger>
      </Tabs.List>

      <Tabs.Content value="summary" class="p-3 min-h-0">
        {#if hasSummary}
          <div class="flex items-center justify-between mb-2">
            <span class="text-[9.5px] uppercase tracking-wide text-muted-foreground font-semibold">Summary</span>
            {#if memory.summarySource === 'auto'}
              <span class="inline-flex items-center gap-1 text-[10px] text-primary border border-primary/45 rounded-full px-2 py-0.5 bg-primary/10">✦ auto</span>
            {:else if memory.summarySource === 'client'}
              <span class="inline-flex items-center text-[10px] text-muted-foreground border border-border rounded-full px-2 py-0.5">authored</span>
            {/if}
          </div>
          <div class="text-[13.5px] leading-relaxed">{memory.summary}</div>
        {:else}
          <div class="text-[12px] text-muted-foreground">No summary — see Content.</div>
        {/if}
      </Tabs.Content>

      <Tabs.Content value="content" class="min-h-0 flex flex-col">
        <ScrollArea class="flex-1 min-h-0"><div class="markdown-body p-3 text-[13px] leading-relaxed">{@html bodyHtml}</div></ScrollArea>
      </Tabs.Content>

      <Tabs.Content value="meta" class="p-3 flex flex-col gap-2 min-h-0">
        <div class="text-[11.5px] font-mono break-all" title={memory.scope}>{memory.scope}</div>
        <div class="flex gap-1.5 flex-wrap text-[10.5px]">
          <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">by</span> {memory.actor}</span>
          <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">src</span> {memory.source}</span>
          <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">vis</span> {memory.visibility}</span>
        </div>
        <div class="flex gap-1.5 flex-wrap">
          {#each memory.tags as t (t)}<span class="px-1.5 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
        </div>
      </Tabs.Content>
    </Tabs.Root>
  {/if}
</div>

<style>
  /* {@html} output can't take Tailwind utilities, so style the rendered
     markdown via :global on the wrapper. Mirrors the visual-companion mockup. */
  .markdown-body :global(h1),
  .markdown-body :global(h2),
  .markdown-body :global(h3),
  .markdown-body :global(h4) { font-weight: 650; margin: 0.9em 0 0.4em; }
  .markdown-body :global(h3) { font-size: 13px; }
  .markdown-body :global(p) { margin: 0 0 0.7em; }
  .markdown-body :global(ul),
  .markdown-body :global(ol) { margin: 0 0 0.7em; padding-left: 1.3em; }
  .markdown-body :global(li) { margin: 0.2em 0; }
  .markdown-body :global(strong) { font-weight: 650; color: var(--foreground); }
  .markdown-body :global(code) { font-family: ui-monospace, Menlo, monospace; font-size: 11.5px; background: var(--accent); border-radius: 4px; padding: 1px 5px; }
  .markdown-body :global(pre) { background: var(--code-bg); border: 1px solid var(--border); border-radius: 8px; padding: 10px 11px; overflow: auto; margin: 0 0 0.7em; }
  .markdown-body :global(pre code) { background: none; padding: 0; font-size: 11.5px; line-height: 1.5; }
  .markdown-body :global(a) { color: var(--primary); text-decoration: underline; text-underline-offset: 2px; }
  .markdown-body :global(blockquote) { border-left: 3px solid var(--border); margin: 0 0 0.7em; padding-left: 0.8em; color: var(--muted-foreground); }
</style>
