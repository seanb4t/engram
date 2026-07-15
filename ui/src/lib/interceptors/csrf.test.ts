import { describe, it, expect } from 'vitest';
import { createClient, createRouterTransport } from '@connectrpc/connect';
import { EngramService } from '../gen/engram_pb';
import { attachCsrf } from './csrf';

// The node test tier runs under `environment: 'node'` (no DOM), so `document`
// does not exist as a global. Stub a minimal cookie-jar object per test —
// attachCsrf only ever reads/re-reads `document.cookie` as a plain string.
function stubDocumentCookie(cookie: string): void {
  Object.defineProperty(globalThis, 'document', {
    value: { cookie },
    writable: true,
    configurable: true
  });
}

function buildClient() {
  const seenHeaders: Array<string | null> = [];
  const transport = createRouterTransport(
    ({ service }) => {
      service(EngramService, {
        storeMemory: (_req, ctx) => {
          seenHeaders.push(ctx.requestHeader.get('X-CSRF-Token'));
          return { id: 'new-id', shortId: 'short' };
        }
      });
    },
    { transport: { interceptors: [attachCsrf] } }
  );
  return { client: createClient(EngramService, transport), seenHeaders };
}

describe('attachCsrf', () => {
  it('sets X-CSRF-Token to the engram_csrf cookie value when present', async () => {
    stubDocumentCookie('other=ignored; engram_csrf=abc123; another=value');
    const { client, seenHeaders } = buildClient();
    await client.storeMemory({ content: 'hello' });
    expect(seenHeaders).toEqual(['abc123']);
  });

  it('does not set X-CSRF-Token when no engram_csrf cookie is present', async () => {
    stubDocumentCookie('other=ignored; another=value');
    const { client, seenHeaders } = buildClient();
    await client.storeMemory({ content: 'hello' });
    expect(seenHeaders).toEqual([null]);
  });

  it('re-reads the cookie per request rather than caching it', async () => {
    stubDocumentCookie('engram_csrf=first-value');
    const { client, seenHeaders } = buildClient();
    await client.storeMemory({ content: 'hello' });

    stubDocumentCookie('engram_csrf=second-value');
    await client.storeMemory({ content: 'hello again' });

    expect(seenHeaders).toEqual(['first-value', 'second-value']);
  });
});
