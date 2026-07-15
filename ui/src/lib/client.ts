import { createClient, ConnectError, Code } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { EngramService } from './gen/engram_pb';
import { attachCsrf } from './interceptors/csrf';
import { retryOnce } from './interceptors/retryOnce';

// Same-origin: the Connect handler is mounted at the service path on root.
// Because requests go to the same origin, the browser's fetch API sends the
// httpOnly session cookie automatically (fetch's `credentials` defaults to
// "same-origin"). This is a fetch behavior, not a createConnectTransport option.
const transport = createConnectTransport({ baseUrl: '/' });

export const engram = createClient(EngramService, transport);

// Write-only transport: interceptor order is [retryOnce, attachCsrf] —
// retryOnce OUTER, attachCsrf INNER. On a retry, retryOnce re-enters
// next(req), which re-enters attachCsrf, re-reading document.cookie fresh
// (Pitfall 3). Reversing the order would freeze the header from the first
// attempt and defeat the retry's whole purpose. The read client (`engram`)
// above is untouched — reads never carry these interceptors.
const writeTransport = createConnectTransport({
  baseUrl: '/',
  interceptors: [retryOnce, attachCsrf]
});

export const engramWrite = createClient(EngramService, writeTransport);

// mapAuthError returns the login path for an Unauthenticated ConnectError, else
// null. Callers (svelte-query onError) navigate to it via window.location.
export function mapAuthError(err: unknown): string | null {
  if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
    return '/auth/login';
  }
  return null;
}
