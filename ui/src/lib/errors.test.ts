import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Code, ConnectError } from '@connectrpc/connect';
import { get } from 'svelte/store';

// mapAuthError is mocked so the auth-redirect case is deterministic and no
// Connect transport is constructed during this node test. The mock exports
// engram/engramWrite alongside it because errors.ts's import graph resolves
// to the same './client' module specifier.
const { mapAuthErrorMock } = vi.hoisted(() => ({ mapAuthErrorMock: vi.fn() }));
vi.mock('./client', () => ({
  mapAuthError: mapAuthErrorMock,
  engram: {},
  engramWrite: {}
}));

import { describeError, logError, reportError, handleQueryError, errorBanner, clearError } from './errors';

describe('describeError', () => {
  it('returns the ConnectError message', () => {
    expect(describeError(new ConnectError('boom', Code.Internal))).toContain('boom');
  });
  it('returns a plain Error message', () => {
    expect(describeError(new Error('nope'))).toBe('nope');
  });
  it('stringifies unknown values', () => {
    expect(describeError('weird')).toBe('weird');
  });
});

describe('logError / reportError / handleQueryError', () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    mapAuthErrorMock.mockReset();
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    clearError();
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
  });

  it('logError logs and leaves errorBanner null', () => {
    logError(new Error('boom'));
    expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
    expect(get(errorBanner)).toBeNull();
  });

  it('reportError logs AND sets errorBanner', () => {
    reportError(new Error('boom'));
    expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
    expect(get(errorBanner)).toBe('boom');
  });

  it('handleQueryError redirects on an Unauthenticated error before anything else, logging nothing and leaving errorBanner null', () => {
    const assignSpy = vi.fn();
    vi.stubGlobal('window', { location: { assign: assignSpy } });
    mapAuthErrorMock.mockReturnValue('/auth/login');
    handleQueryError(new ConnectError('session expired', Code.Unauthenticated));
    expect(assignSpy).toHaveBeenCalledWith('/auth/login');
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(get(errorBanner)).toBeNull();
    vi.unstubAllGlobals();
  });

  it('handleQueryError redirects on an Unauthenticated error even when the query is marked silent -- auth wins first', () => {
    const assignSpy = vi.fn();
    vi.stubGlobal('window', { location: { assign: assignSpy } });
    mapAuthErrorMock.mockReturnValue('/auth/login');
    handleQueryError(new ConnectError('session expired', Code.Unauthenticated), { meta: { silent: true } });
    expect(assignSpy).toHaveBeenCalledWith('/auth/login');
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(get(errorBanner)).toBeNull();
    vi.unstubAllGlobals();
  });

  it('handleQueryError(err, { meta: { silent: true } }) for a non-auth error logs exactly once and leaves errorBanner null', () => {
    mapAuthErrorMock.mockReturnValue(null);
    handleQueryError(new Error('boom'), { meta: { silent: true } });
    expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
    expect(get(errorBanner)).toBeNull();
  });

  it('handleQueryError(err, {}) for a non-auth error sets errorBanner (ordinary query behaviour unchanged)', () => {
    mapAuthErrorMock.mockReturnValue(null);
    handleQueryError(new Error('boom'), {});
    expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
    expect(get(errorBanner)).toBe('boom');
  });

  it('handleQueryError(err) for a non-auth error sets errorBanner (ordinary query behaviour unchanged)', () => {
    mapAuthErrorMock.mockReturnValue(null);
    handleQueryError(new Error('boom'));
    expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
    expect(get(errorBanner)).toBe('boom');
  });
});
