import { describe, it, expect } from 'vitest';
import { mapAuthError, engram, engramWrite } from './client';
import { createClient, ConnectError, Code, createRouterTransport } from '@connectrpc/connect';
import { EngramService } from './gen/engram_pb';
import { retryOnce } from './interceptors/retryOnce';
import { attachCsrf } from './interceptors/csrf';

describe('mapAuthError', () => {
  it('returns a login redirect target for Unauthenticated', () => {
    const err = new ConnectError('nope', Code.Unauthenticated);
    expect(mapAuthError(err)).toBe('/auth/login');
  });
  it('returns null for other errors', () => {
    expect(mapAuthError(new ConnectError('boom', Code.Internal))).toBeNull();
  });
});

describe('engramWrite', () => {
  it('is exported and is a distinct client from engram', () => {
    expect(engram).toBeTruthy();
    expect(engramWrite).toBeTruthy();
    expect(engramWrite).not.toBe(engram);
  });
});

// Codex round-4 LOW: the per-interceptor unit tests (csrf.test.ts,
// retryOnce.test.ts) never exercise the seam where retryOnce re-enters
// attachCsrf on retry. This composed test proves the [retryOnce, attachCsrf]
// order (the exact literal array client.ts uses for writeTransport) rebuilds
// the X-CSRF-Token header from a REFRESHED cookie on the retry attempt, not a
// stale value captured before the first attempt.
describe('auth-race retry (composed [retryOnce, attachCsrf])', () => {
  it('re-enters attachCsrf on retry and reads the cookie value refreshed mid-flight', async () => {
    Object.defineProperty(globalThis, 'document', {
      value: { cookie: 'engram_csrf=stale-value' },
      writable: true,
      configurable: true
    });

    let invocations = 0;
    const seenHeaders: Array<string | null> = [];
    const transport = createRouterTransport(
      ({ service }) => {
        service(EngramService, {
          storeMemory: (_req, ctx) => {
            invocations += 1;
            seenHeaders.push(ctx.requestHeader.get('X-CSRF-Token'));
            if (invocations === 1) {
              // Simulate a concurrent reseal refreshing the CSRF cookie
              // between the first (failed) attempt and the retry.
              (globalThis as { document: { cookie: string } }).document.cookie =
                'engram_csrf=refreshed-value';
              throw new ConnectError('csrf/session race', Code.PermissionDenied);
            }
            return { id: 'new-id', shortId: 'short' };
          }
        });
      },
      { transport: { interceptors: [retryOnce, attachCsrf] } }
    );
    const client = createClient(EngramService, transport);

    const res = await client.storeMemory({ content: 'hello' });

    expect(res.id).toBe('new-id');
    expect(invocations).toBe(2);
    expect(seenHeaders).toEqual(['stale-value', 'refreshed-value']);
  });
});
