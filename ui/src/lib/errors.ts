import { writable } from 'svelte/store';
import { ConnectError } from '@connectrpc/connect';
import { mapAuthError } from './client';

// errorBanner holds the most recent non-auth query error message for a global
// toast/banner. Auth (Unauthenticated) errors are handled by a redirect in the
// layout's QueryCache and never surfaced here.
export const errorBanner = writable<string | null>(null);

export function describeError(err: unknown): string {
  if (err instanceof ConnectError) return err.message;
  if (err instanceof Error) return err.message;
  return String(err);
}

// logError writes the console diagnostic without raising the global error
// banner -- the log-without-banner half of reportError, used for queries
// marked `meta: { silent: true }` (e.g. the migration banner's own lookup).
export function logError(err: unknown): void {
  console.error('[engram] query error:', err);
}

export function reportError(err: unknown): void {
  const msg = describeError(err);
  // Always log; non-auth errors were previously dropped silently.
  logError(err);
  errorBanner.set(msg);
}

export function clearError(): void {
  errorBanner.set(null);
}

// handleQueryError is the whole QueryCache.onError routing: auth redirect
// first (an unauthenticated console redirects regardless of which query
// noticed, including a silent one), then the silent-query opt-out, then the
// ordinary log-and-banner path. `+layout.svelte` wires this by direct
// reference (`onError: handleQueryError`) so production and every test lane
// share exactly one routing implementation.
export function handleQueryError(err: unknown, query?: { meta?: Record<string, unknown> }): void {
  const target = mapAuthError(err);
  if (target) {
    window.location.assign(target);
    return;
  }
  if (query?.meta?.silent) {
    logError(err);
    return;
  }
  reportError(err);
}
