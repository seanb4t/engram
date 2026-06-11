import { describe, it, expect } from 'vitest';
import { mapAuthError } from './client';
import { ConnectError, Code } from '@connectrpc/connect';

describe('mapAuthError', () => {
  it('returns a login redirect target for Unauthenticated', () => {
    const err = new ConnectError('nope', Code.Unauthenticated);
    expect(mapAuthError(err)).toBe('/auth/login');
  });
  it('returns null for other errors', () => {
    expect(mapAuthError(new ConnectError('boom', Code.Internal))).toBeNull();
  });
});
