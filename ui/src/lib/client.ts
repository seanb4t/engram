import { createClient, ConnectError, Code } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { EngramService } from './gen/engram_pb';

// Same-origin: the Connect handler is mounted at the service path on root, and
// the httpOnly session cookie is sent automatically (credentials default).
const transport = createConnectTransport({ baseUrl: '/' });

export const engram = createClient(EngramService, transport);

// mapAuthError returns the login path for an Unauthenticated ConnectError, else
// null. Callers (svelte-query onError) navigate to it via window.location.
export function mapAuthError(err: unknown): string | null {
  if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
    return '/auth/login';
  }
  return null;
}
