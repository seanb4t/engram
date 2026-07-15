import { describe, it, expect } from 'vitest';
import { createClient, ConnectError, Code, createRouterTransport } from '@connectrpc/connect';
import { EngramService } from '../gen/engram_pb';
import { retryOnce } from './retryOnce';

function buildClient(handler: (invocation: number) => { id: string; shortId: string } | never) {
  let invocations = 0;
  const transport = createRouterTransport(
    ({ service }) => {
      service(EngramService, {
        storeMemory: () => {
          invocations += 1;
          return handler(invocations);
        }
      });
    },
    { transport: { interceptors: [retryOnce] } }
  );
  return { client: createClient(EngramService, transport), invocationCount: () => invocations };
}

describe('auth-race retry', () => {
  it('retries once on Unauthenticated and succeeds, invoking the handler exactly twice', async () => {
    const { client, invocationCount } = buildClient((n) => {
      if (n === 1) throw new ConnectError('session cookie race', Code.Unauthenticated);
      return { id: 'new-id', shortId: 'short' };
    });
    const res = await client.storeMemory({ content: 'hello' });
    expect(res.id).toBe('new-id');
    expect(invocationCount()).toBe(2);
  });

  it('retries once on PermissionDenied and succeeds, invoking the handler exactly twice', async () => {
    const { client, invocationCount } = buildClient((n) => {
      if (n === 1) throw new ConnectError('csrf race', Code.PermissionDenied);
      return { id: 'new-id', shortId: 'short' };
    });
    const res = await client.storeMemory({ content: 'hello' });
    expect(res.id).toBe('new-id');
    expect(invocationCount()).toBe(2);
  });

  it('rethrows the second failure unchanged when both attempts fail with the same code, invoking exactly twice', async () => {
    const { client, invocationCount } = buildClient(() => {
      throw new ConnectError('hard-expired session', Code.Unauthenticated);
    });
    await expect(client.storeMemory({ content: 'hello' })).rejects.toMatchObject({
      code: Code.Unauthenticated,
      rawMessage: 'hard-expired session'
    });
    expect(invocationCount()).toBe(2);
  });

  it('does not retry a non-auth failure code, invoking the handler exactly once', async () => {
    const { client, invocationCount } = buildClient(() => {
      throw new ConnectError('bad content', Code.InvalidArgument);
    });
    await expect(client.storeMemory({ content: '' })).rejects.toMatchObject({
      code: Code.InvalidArgument
    });
    expect(invocationCount()).toBe(1);
  });
});
