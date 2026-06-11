import { writable } from 'svelte/store';
import { ConnectError } from '@connectrpc/connect';

// errorBanner holds the most recent non-auth query error message for a global
// toast/banner. Auth (Unauthenticated) errors are handled by a redirect in the
// layout's QueryCache and never surfaced here.
export const errorBanner = writable<string | null>(null);

export function describeError(err: unknown): string {
  if (err instanceof ConnectError) return err.message;
  if (err instanceof Error) return err.message;
  return String(err);
}

export function reportError(err: unknown): void {
  const msg = describeError(err);
  // Always log; non-auth errors were previously dropped silently.
  console.error('[engram] query error:', err);
  errorBanner.set(msg);
}

export function clearError(): void {
  errorBanner.set(null);
}
