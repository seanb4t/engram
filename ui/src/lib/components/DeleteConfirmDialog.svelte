<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';

  // Host-authoritative closure (Codex round-5 HIGH): `open` is driven entirely
  // by the host from its `deleteTarget`. This component never sets `open =
  // false` itself on Delete/confirm — it closes only via Cancel/dismiss
  // (which calls `oncancel`, letting the host clear its target) or when the
  // host clears `deleteTarget` on success. This mirrors ShareWarningInline,
  // which is likewise host-controlled.
  let {
    open = $bindable(false),
    kind,
    onconfirm,
    oncancel,
    authFailure = false,
    onreauth
  }: {
    open?: boolean;
    kind: 'memory' | 'discovery';
    onconfirm: () => Promise<void>;
    oncancel: () => void;
    authFailure?: boolean;
    onreauth?: () => void;
  } = $props();

  const copy = {
    memory: {
      title: 'Delete this memory?',
      body: "this can't be undone. the record and its content are removed permanently."
    },
    discovery: {
      title: 'Delete this discovery?',
      body: "this can't be undone. the map/fact and its citations are removed permanently."
    }
  } as const;

  // Pending state while the awaitable onconfirm is in flight — disables
  // Delete/Cancel and suppresses Escape/overlay dismissal so the mutation
  // can't be double-fired (T-19-32).
  let pending = $state(false);

  async function handleDelete() {
    if (pending) return;
    pending = true;
    try {
      await onconfirm();
    } finally {
      pending = false;
    }
  }

  // Fires only when bits-ui itself closes the dialog (Cancel button, Escape,
  // overlay click) — never on a host-driven `open = false` assignment, since
  // that path doesn't run through bits-ui's internal open-state setter.
  function handleOpenChange(next: boolean) {
    if (!next) oncancel();
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content
    showCloseButton={!pending}
    escapeKeydownBehavior={pending ? 'ignore' : 'close'}
    interactOutsideBehavior={pending ? 'ignore' : 'close'}
  >
    <Dialog.Header>
      <Dialog.Title>{copy[kind].title}</Dialog.Title>
      <Dialog.Description>{copy[kind].body}</Dialog.Description>
    </Dialog.Header>
    {#if authFailure}
      <div role="alert" class="flex flex-col gap-2 text-cat-gotcha text-[12px]">
        <span>write failed — session expired. re-authenticate to continue.</span>
        <Button variant="outline" size="sm" class="self-start" onclick={() => onreauth?.()}>Re-authenticate</Button>
      </div>
    {/if}
    <Dialog.Footer>
      <Dialog.Close>
        {#snippet child({ props })}
          <Button variant="outline" {...props} disabled={pending}>Cancel</Button>
        {/snippet}
      </Dialog.Close>
      <Button variant="destructive" disabled={pending} onclick={handleDelete}>Delete</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
