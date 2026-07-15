<script lang="ts">
  import { Button } from '$lib/components/ui/button';

  // Host-controlled: the HOST renders this component while its `shareTarget`
  // is set, and removes it on success (host-authoritative closure, symmetric
  // with DeleteConfirmDialog). This component never manages its own
  // visibility.
  let {
    onconfirm,
    oncancel,
    authFailure = false,
    onreauth
  }: {
    onconfirm: () => Promise<void>;
    oncancel: () => void;
    authFailure?: boolean;
    onreauth?: () => void;
  } = $props();

  // Pending state while the awaitable onconfirm is in flight — disables
  // Share anyway/Cancel so the share write can't be double-fired.
  let pending = $state(false);

  async function handleShare() {
    if (pending) return;
    pending = true;
    try {
      await onconfirm();
    } finally {
      pending = false;
    }
  }
</script>

<div role="alert" class="flex flex-col gap-2 p-3 text-cat-gotcha bg-card border border-cat-gotcha rounded text-[12px]">
  <span>sharing makes this readable by every authenticated caller. you can stop sharing later, but you can't retract what's already been read.</span>
  {#if authFailure}
    <div class="flex flex-col gap-2">
      <span>write failed — session expired. re-authenticate to continue.</span>
      <Button variant="outline" size="sm" class="self-start" onclick={() => onreauth?.()}>Re-authenticate</Button>
    </div>
  {/if}
  <div class="flex gap-2 justify-end">
    <Button variant="outline" size="sm" disabled={pending} onclick={() => oncancel()}>Cancel</Button>
    <Button variant="destructive" size="sm" disabled={pending} onclick={handleShare}>Share anyway</Button>
  </div>
</div>
